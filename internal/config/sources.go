package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// TreeSource est une couche déjà en mémoire (valeurs par défaut,
// configuration distante).
type TreeSource struct {
	N     string
	R     Rank
	Trust bool
	T     Tree
}

func (s *TreeSource) Name() string  { return s.N }
func (s *TreeSource) Rank() Rank    { return s.R }
func (s *TreeSource) Trusted() bool { return s.Trust }
func (s *TreeSource) Load(context.Context) (Tree, error) {
	return deepCopy(s.T), nil
}

// FileSource est un fichier JSON.
type FileSource struct {
	Path string
	R    Rank
	// AdminFile : fichier déposé par un administrateur. Il n'est tenu pour
	// fiable que si un non-administrateur ne peut pas le modifier ; une
	// stratégie modifiable par tous est ignorée.
	AdminFile bool

	trusted bool
}

func (s *FileSource) Name() string  { return s.Path }
func (s *FileSource) Rank() Rank    { return s.R }
func (s *FileSource) Trusted() bool { return s.trusted }

func (s *FileSource) Load(context.Context) (Tree, error) {
	s.trusted = false
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if s.AdminFile {
		secure, err := secureFile(s.Path)
		switch {
		case err != nil:
			return nil, fmt.Errorf("droits du fichier illisibles : %w", err)
		case !secure && s.R == RankPolicy:
			return nil, errors.New("fichier modifiable par des non-administrateurs : stratégie ignorée")
		}
		s.trusted = secure
	}
	return parseTree(b)
}

func parseTree(b []byte) (Tree, error) {
	var t Tree
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("JSON invalide : %w", err)
	}
	if t == nil {
		t = Tree{}
	}
	return t, nil
}

// dirSources donne un FileSource par fichier *.json d'un dossier, par ordre
// alphabétique (fragments config.d).
func dirSources(dir string, r Rank, admin bool) []Source {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	out := make([]Source, 0, len(names))
	for _, n := range names {
		out = append(out, &FileSource{Path: dir + string(os.PathSeparator) + n, R: r, AdminFile: admin})
	}
	return out
}

// EnvSource lit les variables d'environnement :
//
//	LATTIS_CONFIG=chemin.json            un fichier entier
//	LATTIS_CONFIG_JSON='{...}'           un objet entier
//	LATTIS_CONFIG_URL=https://…          configUrl
//	LATTIS_AI_ENDPOINT=https://…         ai.endpoint
//	LATTIS_A__B__C=valeur                la clé a.b.c
//
// Les autres variables LATTIS_ (LATTIS_SQL…) ne sont pas de la
// configuration. Une valeur est lue comme du JSON quand elle en est
// (true, 12, {…}), comme du texte sinon.
type EnvSource struct {
	// Environ remplace os.Environ (tests).
	Environ func() []string
}

var envAliases = map[string]string{
	"LATTIS_CONFIG_URL":  "configUrl",
	"LATTIS_AI_ENDPOINT": "ai.endpoint",
}

func (s *EnvSource) Name() string  { return "variables LATTIS_*" }
func (s *EnvSource) Rank() Rank    { return RankEnv }
func (s *EnvSource) Trusted() bool { return false }

func (s *EnvSource) Load(context.Context) (Tree, error) {
	environ := os.Environ
	if s.Environ != nil {
		environ = s.Environ
	}
	vars := map[string]string{}
	var names []string
	for _, kv := range environ() {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(strings.ToUpper(k), "LATTIS_") {
			continue
		}
		k = strings.ToUpper(k)
		vars[k] = v
		names = append(names, k)
	}
	sort.Strings(names)
	t := Tree{}
	found := false
	if p := vars["LATTIS_CONFIG"]; p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("LATTIS_CONFIG : %w", err)
		}
		ft, err := parseTree(b)
		if err != nil {
			return nil, fmt.Errorf("LATTIS_CONFIG : %w", err)
		}
		mergeTree(t, ft)
		found = true
	}
	if j := vars["LATTIS_CONFIG_JSON"]; j != "" {
		jt, err := parseTree([]byte(j))
		if err != nil {
			return nil, fmt.Errorf("LATTIS_CONFIG_JSON : %w", err)
		}
		mergeTree(t, jt)
		found = true
	}
	for _, k := range names {
		path, ok := envAliases[k]
		if !ok {
			rest := strings.TrimPrefix(k, "LATTIS_")
			if !strings.Contains(rest, "__") {
				continue
			}
			path = strings.ToLower(strings.ReplaceAll(rest, "__", "."))
		}
		setPath(t, path, parseScalar(vars[k]))
		found = true
	}
	if !found {
		return nil, nil
	}
	return t, nil
}

// FlagSource lit les options --set chemin=valeur.
type FlagSource struct {
	Pairs []string
}

func (s *FlagSource) Name() string  { return "ligne de commande (--set)" }
func (s *FlagSource) Rank() Rank    { return RankFlag }
func (s *FlagSource) Trusted() bool { return false }

func (s *FlagSource) Load(context.Context) (Tree, error) {
	if len(s.Pairs) == 0 {
		return nil, nil
	}
	t := Tree{}
	for _, p := range s.Pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("--set %q : forme attendue chemin=valeur", p)
		}
		setPath(t, strings.TrimSpace(k), parseScalar(v))
	}
	return t, nil
}

// parseScalar lit une valeur texte : JSON si c'en est, texte sinon.
func parseScalar(s string) any {
	trim := strings.TrimSpace(s)
	if trim == "" {
		return s
	}
	switch trim[0] {
	case '{', '[', '"', 't', 'f', 'n', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		var v any
		if json.Unmarshal([]byte(trim), &v) == nil {
			if m, ok := v.(map[string]any); ok {
				return Tree(m)
			}
			return v
		}
	}
	return s
}

// setPath pose une valeur à un chemin pointé, en créant les objets
// intermédiaires (sans tenir compte de la casse des clés existantes).
func setPath(t Tree, path string, v any) {
	segs := strings.Split(path, ".")
	cur := t
	for i, seg := range segs {
		k := seg
		if existing, ok := findKey(cur, seg); ok {
			k = existing
		}
		if i == len(segs)-1 {
			if sub, ok := v.(Tree); ok {
				if old, ok := cur[k].(Tree); ok {
					mergeTree(old, sub)
					return
				}
			}
			cur[k] = v
			return
		}
		next, ok := cur[k].(Tree)
		if !ok {
			next = Tree{}
			cur[k] = next
		}
		cur = next
	}
}

// mergeTree fusionne src dans dst, sans provenance.
func mergeTree(dst, src Tree) {
	for k, v := range src {
		name := k
		if existing, ok := findKey(dst, k); ok {
			name = existing
		}
		if sub, ok := v.(Tree); ok {
			if cur, ok := dst[name].(Tree); ok {
				mergeTree(cur, sub)
				continue
			}
			dst[name] = deepCopy(sub)
			continue
		}
		dst[name] = v
	}
}

func deepCopy(t Tree) Tree {
	if t == nil {
		return nil
	}
	out := make(Tree, len(t))
	for k, v := range t {
		switch x := v.(type) {
		case Tree:
			out[k] = deepCopy(x)
		case []any:
			out[k] = append([]any(nil), x...)
		default:
			out[k] = v
		}
	}
	return out
}
