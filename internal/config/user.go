package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lalamefine/lattis/internal/msg"
)

// Réglages modifiables depuis l'interface (Fichier › Paramètres) : écrits
// dans le config.json de l'utilisateur, la couche User. Une stratégie, une
// variable LATTIS_*, une option --set ou le Registre, plus prioritaires,
// l'emportent toujours sur eux.

// UserEditable liste les clés que l'interface peut écrire.
var UserEditable = []string{"features.sharedWorkbooks", "features.coEditing", "ai.enabled", "ai.dataSharing", "ai.provider", "ai.model", "ai.keepConversation", "ai.localUrl", "mcp.confirmWrites",
	"limits.undoDepth", "limits.scriptSeconds", "limits.sqlReadSeconds", "limits.sqlWriteSeconds",
	"ui.theme", "ui.themes", "ui.language", "updates.check"}

// UserVisible liste les clés que l'interface lit sans les écrire.
var UserVisible = []string{"ui.builtinThemes"}

// Editable dit si l'interface peut écrire cette clé. ui.themes s'écrit
// thème par thème (ui.themes.<id>, null pour le retirer) : l'interface ne
// recopie jamais les thèmes de l'organisation dans le fichier de
// l'utilisateur.
func Editable(path string) bool {
	if themePath(path) {
		return true
	}
	if strings.EqualFold(path, "ui.themes") {
		return false
	}
	for _, p := range UserEditable {
		if strings.EqualFold(p, path) {
			return true
		}
	}
	return false
}

// UserSetting est l'état d'un réglage de l'interface.
type UserSetting struct {
	Path  string `json:"path"`
	Value any    `json:"value"`
	// Locked : imposé par une stratégie de l'organisation.
	Locked bool `json:"locked,omitempty"`
	// Fixed : fixé par une couche plus prioritaire que l'utilisateur
	// (Registre, variable LATTIS_*, --set) ; le changer ici est sans effet.
	Fixed bool `json:"fixed,omitempty"`
	// Source : couche qui donne la valeur.
	Source string `json:"source,omitempty"`
	// Managed (ui.themes) : identifiants venus d'ailleurs que du fichier de
	// l'utilisateur (organisation…), qu'il ne peut pas retirer.
	Managed []string `json:"managed,omitempty"`
	// ReadOnly : réglage lu par l'interface, qu'elle n'écrit pas.
	ReadOnly bool `json:"readOnly,omitempty"`
}

// UserSettings donne l'état des réglages de l'interface dans c.
func UserSettings(c *Config) []UserSetting {
	out := make([]UserSetting, 0, len(UserEditable)+len(UserVisible))
	for _, p := range append(slices.Clone(UserEditable), UserVisible...) {
		v, _ := c.Lookup(p)
		s := UserSetting{Path: p, Value: v, Locked: c.Locked(p), ReadOnly: slices.Contains(UserVisible, p)}
		if o, ok := c.Origin(p); ok {
			s.Source = o.Source
			s.Fixed = !s.Locked && o.Rank > RankUser
		}
		if p == "ui.themes" {
			s.Managed = themeOrigins(c)
			s.Locked = false // un thème imposé n'empêche pas d'ajouter les siens
		}
		out = append(out, s)
	}
	return out
}

// active est le Loader de l'application (Init).
var active atomic.Pointer[Loader]

var userMu sync.Mutex

func (l *Loader) userFile() string {
	if l.UserFile != "" {
		return l.UserFile
	}
	return userFile()
}

// SetUser écrit path = v dans le config.json de l'utilisateur (créé au
// besoin, les autres clés gardées), puis recharge la configuration, qui
// devient la configuration courante. Refusé pour une clé hors de
// UserEditable, une valeur invalide ou une clé imposée par une stratégie.
func (l *Loader) SetUser(ctx context.Context, path string, v any) (*Config, error) {
	if !Editable(path) {
		return nil, msg.New("config.notEditable", msg.Args{"path": path})
	}
	if cv, ok := coerce(path, v); ok {
		v = cv
	}
	if m, ok := v.(map[string]any); ok {
		v = Tree(m) // objet venu du pont (thème)
	}
	remove := v == nil && themePath(path)
	if check := validatorFor(path); check != nil && !remove {
		if err := check(v); err != nil {
			return nil, fmt.Errorf("%s : %w", path, err)
		}
	}
	userMu.Lock()
	defer userMu.Unlock()
	if l.Load(ctx).Locked(path) {
		return nil, msg.New("config.locked", msg.Args{"path": path})
	}
	t, err := l.readUser()
	if err != nil {
		return nil, err
	}
	if remove {
		// Retiré du fichier (null y masquerait un thème de l'organisation).
		if th, ok := lookup(t, "ui.themes"); ok {
			if m, ok := th.(Tree); ok {
				if k, ok := findKey(m, path[len(themesPrefix):]); ok {
					delete(m, k)
				}
			}
		}
	} else {
		setPath(t, path, v)
	}
	if err := l.writeUser(t); err != nil {
		return nil, err
	}
	c := l.Load(ctx)
	SetCurrent(c)
	return c, nil
}

// SetUserValue écrit un réglage de l'utilisateur avec le Loader de
// l'application (un Loader sans option avant Init).
func SetUserValue(ctx context.Context, path string, v any) (*Config, error) {
	return activeLoader().SetUser(ctx, path, v)
}

func activeLoader() *Loader {
	if l := active.Load(); l != nil {
		return l
	}
	return &Loader{}
}

// ResetUser remet les réglages paths à leur valeur par défaut (Fichier ›
// Paramètres › Réinitialiser) : ils sont retirés du config.json de
// l'utilisateur, ses autres clés (serveurs, thèmes créés…) gardées. Un
// réglage imposé par une stratégie n'est pas touché. Refusé pour une clé
// hors de UserEditable ; ui.themes (thèmes créés par l'utilisateur) n'est
// jamais remis à zéro ici.
func (l *Loader) ResetUser(ctx context.Context, paths []string) (*Config, error) {
	for _, p := range paths {
		if !Editable(p) || themePath(p) {
			return nil, msg.New("config.notEditable", msg.Args{"path": p})
		}
	}
	userMu.Lock()
	defer userMu.Unlock()
	cur := l.Load(ctx)
	if !l.Memory {
		file := l.userFile()
		if file == "" {
			return nil, msg.New("config.userDirNotFound", nil)
		}
		if _, err := os.Stat(file); errors.Is(err, fs.ErrNotExist) {
			// Rien d'écrit : déjà les valeurs par défaut.
			SetCurrent(cur)
			return cur, nil
		}
	}
	t, err := l.readUser()
	if err != nil {
		return nil, err
	}
	changed := false
	for _, p := range paths {
		if !cur.Locked(p) && deletePath(t, p) {
			changed = true
		}
	}
	if changed {
		if err := l.writeUser(t); err != nil {
			return nil, err
		}
	}
	c := l.Load(ctx)
	SetCurrent(c)
	return c, nil
}

// ResetUserValues remet des réglages de l'utilisateur à leur valeur par
// défaut avec le Loader de l'application.
func ResetUserValues(ctx context.Context, paths []string) (*Config, error) {
	return activeLoader().ResetUser(ctx, paths)
}

// deletePath retire la clé d'un chemin pointé (sans tenir compte de la
// casse) et les objets devenus vides qui la contenaient ; faux si elle
// n'y était pas.
func deletePath(t Tree, path string) bool {
	seg, rest, nested := strings.Cut(path, ".")
	k, ok := findKey(t, seg)
	if !ok {
		return false
	}
	if !nested {
		delete(t, k)
		return true
	}
	sub, ok := t[k].(Tree)
	if !ok || !deletePath(sub, rest) {
		return false
	}
	if len(sub) == 0 {
		delete(t, k)
	}
	return true
}

// readUser lit les réglages de l'utilisateur : son config.json (absent :
// aucun), ou la mémoire (Loader.Memory).
func (l *Loader) readUser() (Tree, error) {
	if l.Memory {
		return l.memUser(), nil
	}
	file := l.userFile()
	if file == "" {
		return nil, msg.New("config.userDirNotFound", nil)
	}
	b, err := os.ReadFile(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Tree{}, nil
	case err != nil:
		return nil, err
	}
	t, err := parseTree(b)
	if err != nil {
		// Ne pas écraser un fichier écrit à la main et cassé.
		return nil, fmt.Errorf("%s : %w", file, err)
	}
	return t, nil
}

// writeUser écrit les réglages de l'utilisateur (config.json, ou la
// mémoire).
func (l *Loader) writeUser(t Tree) error {
	if l.Memory {
		l.memMu.Lock()
		l.mem = deepCopy(t)
		l.memMu.Unlock()
		return nil
	}
	out, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(l.userFile(), append(out, '\n'))
}

// memUser : copie des réglages gardés en mémoire (Loader.Memory).
func (l *Loader) memUser() Tree {
	l.memMu.Lock()
	defer l.memMu.Unlock()
	if l.mem == nil {
		return Tree{}
	}
	return deepCopy(l.mem)
}

// writeFileAtomic écrit un fichier par un fichier temporaire renommé : une
// coupure ne laisse jamais un config.json à moitié écrit.
func writeFileAtomic(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}
