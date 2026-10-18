package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// OpenFileEvent demande à l'interface d'ouvrir un classeur désigné par le
// système (double-clic sur un fichier .lattis alors que l'application tourne
// déjà, sous macOS).
const OpenFileEvent = "doc:open-file"

func init() {
	application.RegisterEvent[string](OpenFileEvent)
}

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
	if app := application.Get(); app != nil {
		app.Event.Emit(OpenFileEvent, path)
	}
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
