// Package trust garde les accords de l'utilisateur sur ce poste, hors des
// classeurs : un classeur reçu ne peut pas s'accorder lui-même un droit en
// l'écrivant dans son fichier.
//
//   - Scripts : un script de la bibliothèque d'un classeur (bouton, contrôle,
//     évènement, onglet Document, scripts.run par nom) ne s'exécute que si
//     sa version (empreinte SHA-256 du source) a été autorisée sur ce
//     poste. Un script enregistré ou lancé depuis l'onglet SCRIPT de ce
//     poste l'est d'office : un classeur créé ici ne demande rien ; un
//     classeur reçu, ou un script modifié ailleurs, demande « Autoriser /
//     Refuser / Voir le code » à la première exécution de chaque version.
//     Un refus vaut pour la session.
//   - Pages web : un onglet page HTML d'un classeur ne s'affiche (son code
//     ne s'exécute) que si sa version (empreinte du HTML) est autorisée sur
//     ce poste. Une page créée ou modifiée ici l'est d'office ; une page
//     reçue demande « Autoriser / Ne pas exécuter / Voir le code » la
//     première fois qu'on affiche chaque version.
//   - Réseau : les hôtes que les scripts et les pages HTML d'un classeur
//     peuvent joindre, par classeur (son identifiant aléatoire,
//     doc.Document.SecretScope), comme les accords des mots de passe
//     (query.Secrets). Les sources des requêtes Power Query (API web,
//     bases de données) suivent la même règle.
//   - Fichiers : les fichiers que les requêtes d'un classeur peuvent lire,
//     par classeur.
//
// Les accords sont rangés dans le dossier de configuration de
// l'utilisateur (Lattis/accords.json). À la première ouverture de ce
// fichier, les classeurs récents (liste de l'application) sont notés
// comme « anciens » : rouverts, leurs scripts et leurs hôtes enregistrés
// dans le fichier (versions précédentes) sont repris sans question
// (ClaimLegacy). Sous go test, le magasin par défaut reste en mémoire.
package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// Store est un magasin d'accords.
type Store struct {
	mu     sync.Mutex
	path   string // vide : en mémoire seulement
	legacy func() []string
	loaded bool
	data   fileData
	// refused : empreintes des scripts refusés pendant la session.
	refused map[string]bool
}

type fileData struct {
	// Scripts : versions autorisées, par empreinte.
	Scripts map[string]ScriptGrant `json:"scripts,omitempty"`
	// Pages : versions de pages web (onglets HTML) autorisées, par empreinte.
	Pages map[string]ScriptGrant `json:"pages,omitempty"`
	// Hosts : hôtes autorisés, par classeur.
	Hosts map[string][]string `json:"hotes,omitempty"`
	// Files : fichiers que les requêtes peuvent lire, par classeur.
	Files map[string][]string `json:"fichiers,omitempty"`
	// Legacy : classeurs récents au moment de la création du fichier, dont
	// les accords enregistrés dans le classeur sont repris à la réouverture.
	Legacy []string `json:"anciens,omitempty"`
}

// ScriptGrant est l'accord donné à une version d'un script.
type ScriptGrant struct {
	Name string    `json:"nom,omitempty"`
	At   time.Time `json:"le"`
}

// New crée un magasin rangé dans path (vide : en mémoire). legacy donne,
// à la création du fichier, les classeurs à reprendre (nil : aucun).
func New(path string, legacy func() []string) *Store {
	return &Store{path: path, legacy: legacy}
}

var (
	defMu sync.Mutex
	def   = newDefault()
)

func newDefault() *Store {
	if testing.Testing() {
		return New("", nil)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return New("", nil)
	}
	return New(filepath.Join(dir, "Lattis", "accords.json"), recentWorkbooks)
}

// Default donne le magasin de l'application.
func Default() *Store {
	defMu.Lock()
	defer defMu.Unlock()
	return def
}

// Use remplace le magasin de l'application (tests : « un autre poste ») et
// renvoie la fonction qui remet le précédent.
func Use(s *Store) (restore func()) {
	defMu.Lock()
	old := def
	def = s
	defMu.Unlock()
	return func() {
		defMu.Lock()
		def = old
		defMu.Unlock()
	}
}

// recentWorkbooks lit la liste des classeurs récents de l'application
// (internal/bridge, recent.go).
func recentWorkbooks() []string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(dir, "Lattis", "recent.json"))
	if err != nil {
		return nil
	}
	var list []string
	if json.Unmarshal(b, &list) != nil {
		return nil
	}
	return list
}

// Hash donne l'empreinte d'une version de script ou de page web.
func Hash(source string) string {
	sum := sha256.Sum256([]byte(strings.ReplaceAll(source, "\r\n", "\n")))
	return hex.EncodeToString(sum[:])
}

// load lit le fichier (une fois) ; mu tenu.
func (s *Store) load() {
	if s.loaded {
		return
	}
	s.loaded = true
	s.data = fileData{Scripts: map[string]ScriptGrant{}, Pages: map[string]ScriptGrant{}, Hosts: map[string][]string{}, Files: map[string][]string{}}
	if s.path == "" {
		return
	}
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		// Premier lancement avec les accords hors des classeurs.
		if s.legacy != nil {
			for _, p := range s.legacy() {
				s.data.Legacy = append(s.data.Legacy, cleanPath(p))
			}
		}
		s.save()
		return
	}
	if err != nil {
		return
	}
	var d fileData
	if json.Unmarshal(b, &d) != nil {
		return
	}
	if d.Scripts != nil {
		s.data.Scripts = d.Scripts
	}
	if d.Pages != nil {
		s.data.Pages = d.Pages
	}
	if d.Hosts != nil {
		s.data.Hosts = d.Hosts
	}
	if d.Files != nil {
		s.data.Files = d.Files
	}
	s.data.Legacy = d.Legacy
}

// save écrit le fichier ; mu tenu.
func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// --- Scripts -----------------------------------------------------------------

// ScriptAllowed dit si la version hash d'un script est autorisée.
func (s *Store) ScriptAllowed(hash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	_, ok := s.data.Scripts[hash]
	return ok
}

// AllowScripts autorise des versions de scripts (empreinte → nom).
func (s *Store) AllowScripts(versions map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	changed := false
	for h, name := range versions {
		delete(s.refused, h)
		if _, ok := s.data.Scripts[h]; ok {
			continue
		}
		s.data.Scripts[h] = ScriptGrant{Name: name, At: time.Now().UTC().Truncate(time.Second)}
		changed = true
	}
	if !changed {
		return nil
	}
	return s.save()
}

// AllowScript autorise la version hash du script name.
func (s *Store) AllowScript(hash, name string) error {
	return s.AllowScripts(map[string]string{hash: name})
}

// RefuseScript retient, pour la session, le refus d'une version.
func (s *Store) RefuseScript(hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refused == nil {
		s.refused = map[string]bool{}
	}
	s.refused[hash] = true
}

// ScriptRefused dit si la version a été refusée pendant la session.
func (s *Store) ScriptRefused(hash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refused[hash]
}

// --- Pages web ---------------------------------------------------------------

// PageAllowed dit si la version hash d'une page web est autorisée.
func (s *Store) PageAllowed(hash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	_, ok := s.data.Pages[hash]
	return ok
}

// AllowPages autorise des versions de pages web (empreinte → nom de
// l'onglet).
func (s *Store) AllowPages(versions map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	changed := false
	for h, name := range versions {
		if _, ok := s.data.Pages[h]; ok {
			continue
		}
		s.data.Pages[h] = ScriptGrant{Name: name, At: time.Now().UTC().Truncate(time.Second)}
		changed = true
	}
	if !changed {
		return nil
	}
	return s.save()
}

// AllowPage autorise la version hash de la page name.
func (s *Store) AllowPage(hash, name string) error {
	return s.AllowPages(map[string]string{hash: name})
}

// --- Réseau ------------------------------------------------------------------

// NormalizeHost écrit un hôte comme il est rangé : en minuscules, sans
// espace autour.
func NormalizeHost(host string) string { return strings.ToLower(strings.TrimSpace(host)) }

// Hosts liste les hôtes autorisés pour le classeur, par ordre alphabétique.
func (s *Store) Hosts(workbook string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	out := slices.Clone(s.data.Hosts[workbook])
	slices.Sort(out)
	if out == nil {
		out = []string{}
	}
	return out
}

// HostAllowed dit si le classeur peut joindre host.
func (s *Store) HostAllowed(workbook, host string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return slices.Contains(s.data.Hosts[workbook], NormalizeHost(host))
}

// AllowHosts autorise des hôtes pour le classeur.
func (s *Store) AllowHosts(workbook string, hosts ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	changed := false
	for _, h := range hosts {
		h = NormalizeHost(h)
		if h == "" || slices.Contains(s.data.Hosts[workbook], h) {
			continue
		}
		s.data.Hosts[workbook] = append(s.data.Hosts[workbook], h)
		changed = true
	}
	if !changed {
		return nil
	}
	return s.save()
}

// ForgetHost retire un hôte du classeur.
func (s *Store) ForgetHost(workbook, host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	h := NormalizeHost(host)
	list := s.data.Hosts[workbook]
	if !slices.Contains(list, h) {
		return nil
	}
	list = slices.DeleteFunc(slices.Clone(list), func(x string) bool { return x == h })
	if len(list) == 0 {
		delete(s.data.Hosts, workbook)
	} else {
		s.data.Hosts[workbook] = list
	}
	return s.save()
}

// --- Fichiers des requêtes ---------------------------------------------------

// FileAllowed dit si les requêtes du classeur peuvent lire le fichier path.
func (s *Store) FileAllowed(workbook, path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	return slices.Contains(s.data.Files[workbook], cleanPath(path))
}

// AllowFiles autorise les requêtes du classeur à lire ces fichiers.
func (s *Store) AllowFiles(workbook string, paths ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	changed := false
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		p = cleanPath(p)
		if slices.Contains(s.data.Files[workbook], p) {
			continue
		}
		s.data.Files[workbook] = append(s.data.Files[workbook], p)
		changed = true
	}
	if !changed {
		return nil
	}
	return s.save()
}

// --- Reprise des classeurs existants ----------------------------------------

func cleanPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		p = a
	}
	return strings.ToLower(filepath.Clean(p))
}

// ClaimLegacy dit si path est un classeur récent au moment de la mise à
// jour (et l'oublie : la reprise ne se fait qu'une fois).
func (s *Store) ClaimLegacy(path string) bool {
	if path == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	p := cleanPath(path)
	i := slices.Index(s.data.Legacy, p)
	if i < 0 {
		return false
	}
	s.data.Legacy = slices.Delete(s.data.Legacy, i, i+1)
	s.save()
	return true
}
