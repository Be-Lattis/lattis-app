package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// RegistrySource lit une clé du Registre. Chaque valeur porte un chemin
// pointé (« connections.erp.config.host ») ; une sous-clé ajoute son nom au
// chemin. REG_SZ est lu comme une valeur d'environnement (JSON si c'en est),
// REG_DWORD et REG_QWORD comme des nombres (0/1 pour les booléens),
// REG_MULTI_SZ comme une liste de textes.
type RegistrySource struct {
	Root  registry.Key
	Path  string
	Label string
	R     Rank
	Trust bool
}

func (s *RegistrySource) Name() string  { return s.Label + `\` + s.Path }
func (s *RegistrySource) Rank() Rank    { return s.R }
func (s *RegistrySource) Trusted() bool { return s.Trust }

func (s *RegistrySource) Load(context.Context) (Tree, error) {
	k, err := registry.OpenKey(s.Root, s.Path, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if errors.Is(err, registry.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer k.Close()
	t := Tree{}
	if err := readRegistryKey(k, "", t); err != nil {
		return nil, err
	}
	return t, nil
}

// configFileValue : valeur spéciale qui désigne un fichier JSON à lire au
// rang de la clé.
const configFileValue = "ConfigFile"

func readRegistryKey(k registry.Key, prefix string, t Tree) error {
	names, err := k.ReadValueNames(0)
	if err != nil {
		return err
	}
	for _, n := range names {
		if n == "" || (prefix == "" && strings.EqualFold(n, configFileValue)) {
			continue
		}
		v, err := registryValue(k, n)
		if err != nil {
			return fmt.Errorf("%s : %w", n, err)
		}
		if v != nil {
			setPath(t, joinPath(prefix, n), v)
		}
	}
	subs, err := k.ReadSubKeyNames(0)
	if err != nil {
		return err
	}
	for _, sub := range subs {
		sk, err := registry.OpenKey(k, sub, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			return err
		}
		err = readRegistryKey(sk, joinPath(prefix, sub), t)
		sk.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func registryValue(k registry.Key, name string) (any, error) {
	_, typ, err := k.GetValue(name, nil)
	if err != nil {
		return nil, err
	}
	switch typ {
	case registry.SZ, registry.EXPAND_SZ:
		s, _, err := k.GetStringValue(name)
		if err != nil {
			return nil, err
		}
		if typ == registry.EXPAND_SZ {
			if x, err := registry.ExpandString(s); err == nil {
				s = x
			}
		}
		return parseScalar(s), nil
	case registry.DWORD, registry.QWORD:
		n, _, err := k.GetIntegerValue(name)
		if err != nil {
			return nil, err
		}
		return float64(n), nil
	case registry.MULTI_SZ:
		l, _, err := k.GetStringsValue(name)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(l))
		for i, s := range l {
			out[i] = s
		}
		return out, nil
	}
	return nil, nil
}

// registryConfigFile donne le fichier désigné par la valeur ConfigFile.
func registryConfigFile(root registry.Key, path string) string {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	s, typ, err := k.GetStringValue(configFileValue)
	if err != nil {
		return ""
	}
	if typ == registry.EXPAND_SZ {
		if x, err := registry.ExpandString(s); err == nil {
			s = x
		}
	}
	return strings.TrimSpace(s)
}

func registryLayer(root registry.Key, label, path string, r Rank, trust bool) []Source {
	var out []Source
	if f := registryConfigFile(root, path); f != "" {
		out = append(out, &FileSource{Path: f, R: r, AdminFile: trust})
	}
	return append(out, &RegistrySource{Root: root, Path: path, Label: label, R: r, Trust: trust})
}

// registrySources : HKLM puis HKCU (le second l'emporte) au rang Registre.
func registrySources() []Source {
	out := registryLayer(registry.LOCAL_MACHINE, "HKLM", `SOFTWARE\Lattis`, RankRegistry, true)
	return append(out, registryLayer(registry.CURRENT_USER, "HKCU", `SOFTWARE\Lattis`, RankRegistry, false)...)
}

// registryPolicySources : stratégie posée par GPO ou Intune. Jamais dans
// HKCU, modifiable par l'utilisateur.
func registryPolicySources() []Source {
	return registryLayer(registry.LOCAL_MACHINE, "HKLM", `SOFTWARE\Policies\Lattis`, RankPolicy, true)
}
