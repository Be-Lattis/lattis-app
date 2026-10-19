package bridge

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/protocol"
)

// OpenFileEvent demande à l'interface d'ouvrir un classeur désigné par le
// système (double-clic sur un fichier .lattis alors que l'application tourne
// déjà, sous macOS).
const OpenFileEvent = "doc:open-file"

// Extension est l'extension des classeurs Lattis, associée à l'application
// par les installeurs (MSIX, NSIS, .app, .desktop).
const Extension = ".lattis"

// IsWorkbook dit si path désigne un classeur Lattis (d'après son extension).
func IsWorkbook(path string) bool {
	return strings.EqualFold(filepath.Ext(path), Extension)
}

// LaunchArg reconnaît un argument de ligne de commande qui désigne un
// classeur : un chemin en .lattis, ou un fichier qui existe ; ok faux pour
// une option ou un autre mot (preset de --demo…). Le chemin rendu est
// absolu, "" si le .lattis n'existe pas.
func LaunchArg(a string) (path string, ok bool) {
	if a == "" || strings.HasPrefix(a, "-") {
		return "", false
	}
	st, err := os.Stat(a)
	switch {
	case err == nil && st.Mode().IsRegular():
	case IsWorkbook(a):
		return "", true
	default:
		return "", false
	}
	if abs, err := filepath.Abs(a); err == nil {
		a = abs
	}
	return a, true
}

// launchState est le classeur à ouvrir au lancement : passé en argument
// (Windows, Linux : association de fichiers) ou annoncé par le système
// (macOS : événement « ouvrir le fichier »).
type launchState struct {
	mu    sync.Mutex
	path  string
	asked bool // l'interface a déjà lu le classeur de lancement
	// link : lien lattis:// du lancement ; linkAsked : l'interface l'a lu.
	link      string
	linkAsked bool
}

// RequestOpen demande l'ouverture d'un classeur. Avant que l'interface ait
// lu le classeur de lancement, il le devient ; ensuite, l'interface est
// prévenue par OpenFileEvent.
func (s *DocumentService) RequestOpen(path string) {
	if path == "" {
		return
	}
	s.launch.mu.Lock()
	if !s.launch.asked {
		s.launch.path = path
		s.launch.mu.Unlock()
		return
	}
	s.launch.mu.Unlock()
	s.sys().Emit(OpenFileEvent, path)
}

// LaunchFile donne, une seule fois, le classeur à ouvrir au lancement, ou "".
func (s *DocumentService) LaunchFile() string {
	s.launch.mu.Lock()
	defer s.launch.mu.Unlock()
	s.launch.asked = true
	p := s.launch.path
	s.launch.path = ""
	return p
}

// OpenLinkEvent demande à l'interface d'ouvrir un lien lattis:// reçu
// pendant que l'application tourne (instance déjà ouverte, macOS).
const OpenLinkEvent = "doc:open-link"

// IsDesktopLink dit si un argument est un lien lattis:// (à ouvrir avec
// RequestLink, pas un fichier).
func IsDesktopLink(a string) bool {
	return strings.HasPrefix(strings.ToLower(a), protocol.DesktopScheme+":")
}

// FileURI ramène un argument file:///… (gestionnaires de fichiers Linux,
// %U du .desktop) à un chemin ; tout autre argument est rendu tel quel.
func FileURI(a string) string {
	if !strings.HasPrefix(strings.ToLower(a), "file://") {
		return a
	}
	u, err := url.Parse(a)
	if err != nil || (u.Host != "" && u.Host != "localhost") {
		return a
	}
	p := u.Path
	if len(p) > 2 && p[0] == '/' && p[2] == ':' { // file:///C:/… sous Windows
		p = p[1:]
	}
	return filepath.FromSlash(p)
}

// RequestLink demande l'ouverture d'un lien lattis:// : au lancement, il est
// gardé pour l'interface (LaunchLink) ; ensuite, elle est prévenue par
// OpenLinkEvent.
func (s *DocumentService) RequestLink(link string) {
	if link == "" {
		return
	}
	s.launch.mu.Lock()
	if !s.launch.linkAsked {
		s.launch.link = link
		s.launch.mu.Unlock()
		return
	}
	s.launch.mu.Unlock()
	s.sys().Emit(OpenLinkEvent, link)
}

// LaunchLink donne, une seule fois, le lien lattis:// du lancement, ou "".
func (s *DocumentService) LaunchLink() string {
	s.launch.mu.Lock()
	defer s.launch.mu.Unlock()
	s.launch.linkAsked = true
	l := s.launch.link
	s.launch.link = ""
	return l
}

// LinkTarget est un classeur désigné par un lien lattis:// : son serveur
// (adresse normalisée) et son identifiant, et si Lattis connaît déjà ce
// serveur (sinon l'interface demande confirmation avant de s'y connecter).
type LinkTarget struct {
	Server   string `json:"server"`
	Workbook string `json:"workbook"`
	Known    bool   `json:"known"`
}

// ParseLink lit un lien lattis://open?server=…&workbook=… (aucun autre
// paramètre n'est lu).
func (s *DocumentService) ParseLink(link string) (LinkTarget, error) {
	iss, id, err := protocol.ParseDesktopLink(link)
	if err != nil {
		return LinkTarget{}, err
	}
	server, err := auth.NormalizeURL(iss)
	if err != nil {
		return LinkTarget{}, fmt.Errorf("%w : %v", protocol.ErrDesktopLink, err)
	}
	t := LinkTarget{Server: server, Workbook: id}
	for _, sv := range s.LattisState().Servers {
		if sv.URL == server {
			t.Known = true
		}
	}
	return t, nil
}
