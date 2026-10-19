package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/protocol"
)

// Classeurs récents (menu Fichier › Ouvrir un fichier récent, accueil) :
// du plus récent au plus ancien, gardés avec les autres données de
// l'application. Un fichier local est noté par son chemin (une chaîne dans
// recent.json) ; un classeur ouvert depuis un Serveur Lattis par son
// serveur, son identifiant et son titre (un objet) : il se rouvre depuis le
// serveur, son cache local n'est jamais montré.

const maxRecent = 10

// RecentFile est un classeur récent.
type RecentFile struct {
	// Path : fichier local ; pour un classeur du serveur, son cache (jamais
	// affiché).
	Path string `json:"path"`
	// Classeur du serveur (WorkbookID non vide) : titre, serveur, adresse
	// <serveur>/w/<id>.
	Title      string `json:"title,omitempty"`
	Server     string `json:"server,omitempty"`
	ServerName string `json:"serverName,omitempty"`
	WorkbookID string `json:"workbookId,omitempty"`
	Link       string `json:"link,omitempty"`
}

// remote : classeur du serveur.
func (r RecentFile) remote() bool { return r.WorkbookID != "" }

// same : même classeur (même serveur et identifiant, ou même fichier).
func (r RecentFile) same(o RecentFile) bool {
	if r.remote() && o.remote() {
		return r.Server == o.Server && r.WorkbookID == o.WorkbookID
	}
	return r.Path != "" && r.Path == o.Path
}

// recentFile est le fichier de la liste ; surchargeable pour les tests.
var recentFile = func() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Lattis", "recent.json"), nil
}

// readRecent lit la liste : chaînes (fichiers locaux, et toute la liste des
// versions précédentes) ou objets (classeurs du serveur).
func readRecent() []RecentFile {
	p, err := recentFile()
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var raw []json.RawMessage
	if json.Unmarshal(b, &raw) != nil {
		return nil
	}
	var list []RecentFile
	for _, m := range raw {
		var path string
		if json.Unmarshal(m, &path) == nil {
			list = append(list, RecentFile{Path: path})
			continue
		}
		var r RecentFile
		if json.Unmarshal(m, &r) == nil && (r.Path != "" || r.remote()) {
			list = append(list, r)
		}
	}
	return list
}

func writeRecent(list []RecentFile) {
	p, err := recentFile()
	if err != nil {
		return
	}
	out := make([]any, 0, len(list))
	for _, r := range list {
		if r.remote() {
			r.Link = "" // recalculé à la lecture
			out = append(out, r)
		} else {
			out = append(out, r.Path)
		}
	}
	b, _ := json.Marshal(out)
	if os.MkdirAll(filepath.Dir(p), 0o755) == nil {
		os.WriteFile(p, b, 0o644)
	}
}

// pushRecent place r en tête de la liste (erreurs ignorées : la liste
// n'est qu'une commodité).
func pushRecent(r RecentFile) {
	if r.Path != "" {
		if abs, err := filepath.Abs(r.Path); err == nil {
			r.Path = abs
		}
	}
	list := slices.DeleteFunc(readRecent(), r.same)
	list = append([]RecentFile{r}, list...)
	if len(list) > maxRecent {
		list = list[:maxRecent]
	}
	writeRecent(list)
}

// addRecent place le fichier local path en tête de la liste.
func addRecent(path string) {
	if path != "" {
		pushRecent(RecentFile{Path: path})
	}
}

// addRecentDoc place le classeur d en tête de la liste : un classeur du
// serveur par son serveur et son titre, un fichier local par son chemin.
func addRecentDoc(d *doc.Document) {
	if r := remoteDoc(d); r != nil {
		pushRecent(RecentFile{Path: d.Path(), Title: r.Title, Server: r.Server, ServerName: r.ServerName, WorkbookID: r.WorkbookID})
		return
	}
	addRecent(d.Path())
}

// Recent liste les classeurs ouverts ou enregistrés récemment, du plus
// récent au plus ancien : les fichiers locaux qui existent encore, les
// classeurs du serveur (rouverts depuis le serveur, ShareOpen). Un cache
// noté par son chemin (versions précédentes) devient son classeur du
// serveur.
func (s *DocumentService) Recent(ctx context.Context) []RecentFile {
	list := []RecentFile{}
	for _, r := range readRecent() {
		if !r.remote() {
			if _, err := os.Stat(r.Path); err != nil {
				continue
			}
			if inShareCache(r.Path) {
				m, err := archive.ReadManifest(r.Path)
				if err != nil || m.Remote == nil {
					continue
				}
				r.Server, r.WorkbookID = m.Remote.Server, m.Remote.WorkbookID
			} else if m, err := archive.ReadManifest(r.Path); err == nil && m.Remote != nil && m.Remote.Mode == workbook.Collaborative {
				// Fichier de l'auteur d'un classeur partagé en direct : une seule
				// entrée, le classeur du serveur (son titre : le nom du fichier).
				r.Title = strings.TrimSuffix(filepath.Base(r.Path), filepath.Ext(r.Path))
				r.Server, r.WorkbookID, r.Path = m.Remote.Server, m.Remote.WorkbookID, ""
			}
		}
		if r.remote() {
			if _, err := os.Stat(r.Path); r.Path != "" && err != nil {
				r.Path = "" // cache disparu : rouvert depuis le serveur seulement
			}
			if r.Title == "" {
				r.Title = remoteTitle
			}
			r.Link = protocol.WorkbookLink(r.Server, r.WorkbookID)
		}
		if !slices.ContainsFunc(list, r.same) {
			list = append(list, r)
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
