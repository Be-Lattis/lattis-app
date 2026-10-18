package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// Réglages modifiables depuis l'interface (Fichier › Paramètres) : écrits
// dans le config.json de l'utilisateur, la couche User. Une stratégie, une
// variable LATTIS_*, une option --set ou le Registre, plus prioritaires,
// l'emportent toujours sur eux.

// UserEditable liste les clés que l'interface peut écrire.
var UserEditable = []string{"features.htmlTabs", "features.sharedWorkbooks", "ai.enabled", "ai.dataSharing", "ai.provider", "ai.model"}

// Editable dit si l'interface peut écrire cette clé.
func Editable(path string) bool {
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
}

// UserSettings donne l'état des réglages de l'interface dans c.
func UserSettings(c *Config) []UserSetting {
	out := make([]UserSetting, 0, len(UserEditable))
	for _, p := range UserEditable {
		v, _ := c.Lookup(p)
		s := UserSetting{Path: p, Value: v, Locked: c.Locked(p)}
		if o, ok := c.Origin(p); ok {
			s.Source = o.Source
			s.Fixed = !s.Locked && o.Rank > RankUser
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
		return nil, fmt.Errorf("%s : réglage non modifiable depuis l'interface", path)
	}
	if cv, ok := coerce(path, v); ok {
		v = cv
	}
	if check := validatorFor(path); check != nil {
		if err := check(v); err != nil {
			return nil, fmt.Errorf("%s : %w", path, err)
		}
	}
	userMu.Lock()
	defer userMu.Unlock()
	if l.Load(ctx).Locked(path) {
		return nil, fmt.Errorf("%s : réglage imposé par la stratégie de votre organisation", path)
	}
	file := l.userFile()
	if file == "" {
		return nil, errors.New("dossier de configuration de l'utilisateur introuvable")
	}
	t := Tree{}
	b, err := os.ReadFile(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if t, err = parseTree(b); err != nil {
			// Ne pas écraser un fichier écrit à la main et cassé.
			return nil, fmt.Errorf("%s : %w", file, err)
		}
	}
	setPath(t, path, v)
	out, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := writeFileAtomic(file, append(out, '\n')); err != nil {
		return nil, err
	}
	c := l.Load(ctx)
	SetCurrent(c)
	return c, nil
}

// SetUserValue écrit un réglage de l'utilisateur avec le Loader de
// l'application (un Loader sans option avant Init).
func SetUserValue(ctx context.Context, path string, v any) (*Config, error) {
	l := active.Load()
	if l == nil {
		l = &Loader{}
	}
	return l.SetUser(ctx, path, v)
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
