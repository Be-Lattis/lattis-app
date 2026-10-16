package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
)

// Classeurs récents (menu Fichier › Récents) : une liste de chemins, du plus
// récent au plus ancien, gardée avec les autres données de l'application.

const maxRecent = 10

// recentFile est le fichier de la liste ; surchargeable pour les tests.
var recentFile = func() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Slitable", "recent.json"), nil
}

func readRecent() []string {
	p, err := recentFile()
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var list []string
	if json.Unmarshal(b, &list) != nil {
		return nil
	}
	return list
}

// addRecent place path en tête de la liste (erreurs ignorées : la liste
// n'est qu'une commodité).
func addRecent(path string) {
	if path == "" {
		return
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	list := slices.DeleteFunc(readRecent(), func(p string) bool { return p == path })
	list = append([]string{path}, list...)
	if len(list) > maxRecent {
		list = list[:maxRecent]
	}
	p, err := recentFile()
	if err != nil {
		return
	}
	b, _ := json.Marshal(list)
	if os.MkdirAll(filepath.Dir(p), 0o755) == nil {
		os.WriteFile(p, b, 0o644)
	}
}

// Recent liste les classeurs ouverts ou enregistrés récemment qui existent
// encore, du plus récent au plus ancien.
func (s *DocumentService) Recent(ctx context.Context) []string {
	list := []string{}
	for _, p := range readRecent() {
		if _, err := os.Stat(p); err == nil {
			list = append(list, p)
		}
	}
	return list
}

// CloseDocument ferme le classeur ouvert (les modifications non enregistrées sont
// perdues : l'interface demande d'abord).
func (s *DocumentService) CloseDocument(ctx context.Context) error {
	s.replace(nil)
	return nil
}
