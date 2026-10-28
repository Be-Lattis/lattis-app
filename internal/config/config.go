// Package config résout la configuration de l'application en cascade :
// chaque clé prend la valeur de la couche la plus prioritaire qui la définit.
//
// De la plus prioritaire à la moins prioritaire :
//
//	Policy    stratégie de l'organisation (HKLM\SOFTWARE\Policies\Lattis,
//	          /etc/lattis/policy.json…) : les clés posées là sont verrouillées
//	Flag      --set chemin=valeur
//	Env       variables LATTIS_*
//	Registry  HKCU\SOFTWARE\Lattis puis HKLM\SOFTWARE\Lattis (Windows)
//	User      config.json de l'utilisateur (UserConfigDir/Lattis)
//	Machine   config.json de la machine (ProgramData, /etc/lattis…)
//	Remote    configuration distante (configUrl), mise en cache
//	Default   valeurs compilées
//
// Les objets sont fusionnés clé par clé (une couche peut ne surcharger qu'une
// feuille) ; null retire une clé posée plus bas. Chaque feuille garde sa
// provenance, pour « lattis config » et pour griser dans l'interface ce qui
// est imposé. Une valeur invalide est ignorée pour cette clé seulement : la
// couche suivante la fournit, et un avertissement le signale. La conception
// complète est dans la note conception/configuration-entreprise.md du projet.
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Rank est le rang d'une couche ; plus il est grand, plus elle est prioritaire.
type Rank int

const (
	RankDefault Rank = iota
	RankRemote
	RankMachine
	RankUser
	RankRegistry
	RankEnv
	RankFlag
	RankPolicy
)

func (r Rank) String() string {
	switch r {
	case RankDefault:
		return "défaut"
	case RankRemote:
		return "distante"
	case RankMachine:
		return "machine"
	case RankUser:
		return "utilisateur"
	case RankRegistry:
		return "registre"
	case RankEnv:
		return "environnement"
	case RankFlag:
		return "ligne de commande"
	case RankPolicy:
		return "stratégie"
	}
	return fmt.Sprintf("rang %d", int(r))
}

// Tree est un arbre JSON décodé (objets map[string]any).
type Tree = map[string]any

// Source est une couche de configuration.
type Source interface {
	// Name désigne la couche pour l'utilisateur : chemin du fichier, clé de
	// Registre, « variables LATTIS_* »…
	Name() string
	Rank() Rank
	// Trusted : seul un administrateur peut écrire cette couche. Les clés
	// sensibles (configUrl, remote.*) ne sont lues que dans ces couches.
	Trusted() bool
	// Load lit la couche ; (nil, nil) si elle n'existe pas.
	Load(ctx context.Context) (Tree, error)
}

// Origin dit d'où vient une valeur.
type Origin struct {
	Source string `json:"source"`
	Rank   Rank   `json:"rank"`
	// Trusted : la couche n'est modifiable que par un administrateur.
	Trusted bool `json:"trusted,omitempty"`
	// Locked : imposée par une stratégie, l'utilisateur ne peut pas la changer.
	Locked bool `json:"locked,omitempty"`
}

// Warning signale une couche illisible ou une valeur écartée. Lattis
// démarre quand même.
type Warning struct {
	Source string `json:"source"`
	Path   string `json:"path,omitempty"`
	Msg    string `json:"message"`
}

func (w Warning) String() string {
	if w.Path != "" {
		return fmt.Sprintf("%s : %s : %s", w.Source, w.Path, w.Msg)
	}
	return fmt.Sprintf("%s : %s", w.Source, w.Msg)
}

// Config est la configuration résolue. Elle ne change pas après Resolve.
type Config struct {
	tree Tree
	// origins : provenance de chaque feuille, par chemin en minuscules.
	origins map[string]Origin
	// paths : chemin avec la casse retenue, par chemin en minuscules.
	paths    map[string]string
	Warnings []Warning
	// Remote : configuration distante utilisée.
	Remote RemoteStatus
}

// Options règle la résolution.
type Options struct {
	// PolicyOverride : la stratégie passe sous l'environnement et la ligne de
	// commande (tests, dépannage). Seule une stratégie qui pose
	// allowPolicyOverride peut l'autoriser.
	PolicyOverride bool
}

// Resolve lit les couches et les fusionne. Les sources de même rang sont
// appliquées dans l'ordre donné : la dernière l'emporte.
func Resolve(ctx context.Context, opts Options, sources ...Source) *Config {
	type layer struct {
		src  Source
		tree Tree
	}
	c := &Config{tree: Tree{}, origins: map[string]Origin{}, paths: map[string]string{}}
	var layers []layer
	for _, s := range sources {
		t, err := s.Load(ctx)
		if err != nil {
			c.Warnings = append(c.Warnings, Warning{Source: s.Name(), Msg: err.Error()})
			continue
		}
		if t != nil {
			layers = append(layers, layer{src: s, tree: t})
		}
	}
	// Clés réservées aux administrateurs posées par une couche de confiance :
	// une couche ordinaire ne peut alors plus les changer. Sans elles (poste
	// sans configuration d'entreprise), l'utilisateur peut les poser.
	adminSet := map[string]bool{}
	for _, l := range layers {
		if !l.src.Trusted() {
			continue
		}
		walk(l.tree, "", func(_ Tree, _ string, path string, v any) bool {
			if adminOnly(path) {
				if _, sub := v.(Tree); !sub {
					adminSet[strings.ToLower(path)] = true
				}
			}
			return true
		})
	}
	policyAllowsOverride := false
	for i := range layers {
		l := &layers[i]
		l.tree = c.validate(l.src, l.tree, adminSet)
		if l.src.Rank() == RankPolicy {
			if v, ok := lookup(l.tree, "allowPolicyOverride"); ok && v == true {
				policyAllowsOverride = true
			}
		}
	}
	override := opts.PolicyOverride && policyAllowsOverride
	order := func(l layer) float64 {
		if l.src.Rank() == RankPolicy && override {
			// Juste au-dessus du Registre, sous l'environnement.
			return float64(RankRegistry) + 0.5
		}
		return float64(l.src.Rank())
	}
	sort.SliceStable(layers, func(i, j int) bool { return order(layers[i]) < order(layers[j]) })
	for _, l := range layers {
		o := Origin{Source: l.src.Name(), Rank: l.src.Rank(), Trusted: l.src.Trusted(),
			Locked: l.src.Rank() == RankPolicy && !override}
		c.merge(c.tree, "", l.tree, o)
	}
	return c
}

// validate écarte d'une couche les valeurs invalides, les secrets et les
// clés réservées aux couches de confiance quand l'une d'elles les pose.
func (c *Config) validate(s Source, t Tree, adminSet map[string]bool) Tree {
	walk(t, "", func(parent Tree, key, path string, v any) bool {
		warn := func(msg string) {
			c.Warnings = append(c.Warnings, Warning{Source: s.Name(), Path: path, Msg: msg})
			delete(parent, key)
		}
		if isSecretKey(path) {
			warn("les secrets ne vont jamais dans la configuration (trousseau du système)")
			return false
		}
		if !s.Trusted() && adminOnly(path) && adminSetCovers(adminSet, path) {
			warn("clé imposée par la configuration de l'administrateur, ignorée ici")
			return false
		}
		if strings.EqualFold(path, "allowPolicyOverride") && s.Rank() != RankPolicy {
			warn("lue seulement dans la stratégie")
			return false
		}
		if v == nil {
			return false
		}
		if cv, ok := coerce(path, v); ok {
			parent[key] = cv
			v = cv
		}
		if check := validatorFor(path); check != nil {
			if err := check(v); err != nil {
				warn(err.Error())
				return false
			}
		}
		if !strings.Contains(path, ".") && !knownTopLevel(path) {
			c.Warnings = append(c.Warnings, Warning{Source: s.Name(), Path: path, Msg: "clé inconnue (gardée)"})
		}
		return true
	})
	return t
}

// adminSetCovers : path, une de ses parties ou un de ses parents est posé
// par une couche de confiance.
func adminSetCovers(adminSet map[string]bool, path string) bool {
	lp := strings.ToLower(path)
	for k := range adminSet {
		if k == lp || strings.HasPrefix(k, lp+".") || strings.HasPrefix(lp, k+".") {
			return true
		}
	}
	return false
}

// walk parcourt un arbre ; fn renvoie faux pour ne pas descendre dans v.
// fn peut retirer la clé de parent.
func walk(t Tree, prefix string, fn func(parent Tree, key, path string, v any) bool) {
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := t[k]
		p := joinPath(prefix, k)
		if !fn(t, k, p, v) {
			continue
		}
		if sub, ok := v.(Tree); ok {
			walk(sub, p, fn)
		}
	}
}

func joinPath(prefix, k string) string {
	if prefix == "" {
		return k
	}
	return prefix + "." + k
}

// findKey cherche une clé sans tenir compte de la casse.
func findKey(t Tree, k string) (string, bool) {
	if _, ok := t[k]; ok {
		return k, true
	}
	for x := range t {
		if strings.EqualFold(x, k) {
			return x, true
		}
	}
	return "", false
}

// merge fusionne src dans dst (objets clé par clé, le reste remplacé).
func (c *Config) merge(dst Tree, prefix string, src Tree, o Origin) {
	keys := make([]string, 0, len(src))
	for k := range src {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := src[k]
		name := k
		if existing, ok := findKey(dst, k); ok {
			name = existing
		}
		p := joinPath(prefix, name)
		if sub, ok := v.(Tree); ok {
			if cur, ok := dst[name].(Tree); ok {
				c.merge(cur, p, sub, o)
				continue
			}
			c.forget(p)
			nt := Tree{}
			dst[name] = nt
			c.merge(nt, p, sub, o)
			continue
		}
		c.forget(p)
		if v == nil {
			delete(dst, name)
			continue
		}
		dst[name] = v
		lp := strings.ToLower(p)
		c.origins[lp] = o
		c.paths[lp] = p
	}
}

// forget oublie la provenance d'un chemin et de ce qu'il contient.
func (c *Config) forget(p string) {
	lp := strings.ToLower(p)
	for k := range c.origins {
		if k == lp || strings.HasPrefix(k, lp+".") {
			delete(c.origins, k)
			delete(c.paths, k)
		}
	}
}

func lookup(t Tree, path string) (any, bool) {
	var cur any = t
	for _, seg := range strings.Split(path, ".") {
		m, ok := cur.(Tree)
		if !ok {
			return nil, false
		}
		k, ok := findKey(m, seg)
		if !ok {
			return nil, false
		}
		cur = m[k]
	}
	return cur, true
}

// Lookup donne la valeur d'un chemin pointé (« connections.erp.config.host »),
// sans tenir compte de la casse.
func (c *Config) Lookup(path string) (any, bool) {
	return lookup(c.tree, path)
}

// Origin donne la provenance d'une feuille.
func (c *Config) Origin(path string) (Origin, bool) {
	o, ok := c.origins[strings.ToLower(path)]
	return o, ok
}

// Imposed dit si une clé, ou l'une de ses parties, est posée par une couche
// plus prioritaire que l'utilisateur (stratégie, --set, LATTIS_*, Registre) :
// Fichier › Paramètres ne peut pas la changer.
func (c *Config) Imposed(path string) bool {
	lp := strings.ToLower(path)
	for k, o := range c.origins {
		if (o.Locked || o.Rank > RankUser) && (k == lp || strings.HasPrefix(k, lp+".") || strings.HasPrefix(lp, k+".")) {
			return true
		}
	}
	return false
}

// Locked dit si une clé, ou l'une de ses parties, est imposée par une
// stratégie.
func (c *Config) Locked(path string) bool {
	lp := strings.ToLower(path)
	for k, o := range c.origins {
		if o.Locked && (k == lp || strings.HasPrefix(k, lp+".") || strings.HasPrefix(lp, k+".")) {
			return true
		}
	}
	return false
}

// String donne la valeur texte d'un chemin, def si absente ou d'un autre type.
func (c *Config) String(path, def string) string {
	if v, ok := c.Lookup(path); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

// Bool donne la valeur booléenne d'un chemin, def si absente.
func (c *Config) Bool(path string, def bool) bool {
	if v, ok := c.Lookup(path); ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

// Decode décode un chemin (ou tout, si path est vide) dans out, comme du JSON.
func (c *Config) Decode(path string, out any) error {
	var v any = c.tree
	if path != "" {
		var ok bool
		if v, ok = c.Lookup(path); !ok {
			return nil
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// Settings donne la configuration typée.
func (c *Config) Settings() (Settings, error) {
	var s Settings
	err := c.Decode("", &s)
	return s, err
}

// JSON donne la configuration effective, secrets masqués.
func (c *Config) JSON() ([]byte, error) {
	return json.MarshalIndent(mask(c.tree, ""), "", "  ")
}

// Entry est une feuille de la configuration effective.
type Entry struct {
	Path  string `json:"path"`
	Value any    `json:"value"`
	Origin
}

// Explain liste chaque feuille avec sa provenance, secrets masqués.
func (c *Config) Explain() []Entry {
	var out []Entry
	for lp, o := range c.origins {
		p := c.paths[lp]
		v, _ := c.Lookup(p)
		if isMaskedKey(p) {
			v = "••••"
		}
		out = append(out, Entry{Path: p, Value: v, Origin: o})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Path) < strings.ToLower(out[j].Path) })
	return out
}

func mask(v any, path string) any {
	switch x := v.(type) {
	case Tree:
		out := Tree{}
		for k, sub := range x {
			out[k] = mask(sub, joinPath(path, k))
		}
		return out
	}
	if isMaskedKey(path) {
		return "••••"
	}
	return v
}
