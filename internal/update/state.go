package update

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// State : mémoire de la mise à jour, dans un petit fichier du dossier de
// configuration de l'utilisateur (update.json), hors de config.json.
type State struct {
	// LastVersion : version de l'application au lancement précédent ; une
	// version différente au lancement suivant veut dire qu'elle a été mise
	// à jour.
	LastVersion string `json:"lastVersion,omitempty"`
	// LastCheck : dernière recherche sur GitHub (au plus deux par jour).
	LastCheck time.Time `json:"lastCheck,omitempty"`
	// Latest : la version plus récente trouvée alors (tag) et sa page.
	Latest     string `json:"latest,omitempty"`
	LatestPage string `json:"latestPage,omitempty"`
	// Pending : mise à jour téléchargée et vérifiée, à installer à la
	// fermeture de l'application.
	Pending *Pending `json:"pending,omitempty"`
	// ApplyError : l'installation lancée à la fermeture précédente n'a pas
	// pu partir (droits d'administration refusés…).
	ApplyError string `json:"applyError,omitempty"`
}

// Pending est une mise à jour prête.
type Pending struct {
	Version string `json:"version"` // sans « v »
	Page    string `json:"page,omitempty"`
	Kind    Kind   `json:"kind"`
	// File : installeur téléchargé, ou nouvel exécutable (portable : à côté
	// de l'ancien, « lattis.exe.new »).
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	// Dir : dossier d'installation (installeurs : /D=).
	Dir string `json:"dir,omitempty"`
	// Attempts : les essais d'installation de cette version, dans l'ordre.
	Attempts []Attempt `json:"attempts,omitempty"`
}

// Issue d'un essai d'installation.
const (
	// OutcomeLaunched : installeur lancé (silencieux) à la fermeture, ou
	// exécutable portable remplacé ; jugé au lancement suivant.
	OutcomeLaunched = "launched"
	// OutcomeManual : installeur lancé avec son interface (« Installer
	// manuellement ») ; jugé au lancement suivant.
	OutcomeManual = "manual"
	// OutcomeFailed : l'installation n'a pas pu partir, ou l'application
	// relancée a toujours l'ancienne version.
	OutcomeFailed = "failed"
)

// MaxAutoAttempts : après autant d'échecs, la mise à jour n'est plus
// installée d'elle-même à la fermeture (reste « Installer manuellement »).
const MaxAutoAttempts = 2

// Attempt : un essai d'installation de la mise à jour en attente.
type Attempt struct {
	At      time.Time `json:"at"`
	Outcome string    `json:"outcome"`
	// Error : l'installation n'a pas pu partir (dans la langue d'alors).
	Error string `json:"error,omitempty"`
	// Result, Code, InstDir : la trace laissée par l'installeur
	// (install-result.txt : ok, failed, locked, elevation ; « none » s'il
	// n'en a laissé aucune), lue au lancement suivant.
	Result  string `json:"result,omitempty"`
	Code    string `json:"code,omitempty"`
	InstDir string `json:"instDir,omitempty"`
}

// failures : nombre d'essais en échec.
func (p *Pending) failures() int {
	n := 0
	if p != nil {
		for _, a := range p.Attempts {
			if a.Outcome == OutcomeFailed {
				n++
			}
		}
	}
	return n
}

// lastFailure : le dernier essai en échec (nil : aucun).
func (p *Pending) lastFailure() *Attempt {
	if p == nil {
		return nil
	}
	for i := len(p.Attempts) - 1; i >= 0; i-- {
		if p.Attempts[i].Outcome == OutcomeFailed {
			return &p.Attempts[i]
		}
	}
	return nil
}

// LoadState lit l'état ; un fichier absent ou illisible donne un état vide.
func LoadState(path string) State {
	var st State
	if path == "" {
		return st
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	if json.Unmarshal(b, &st) != nil {
		return State{}
	}
	return st
}

// SaveState écrit l'état (fichier temporaire renommé).
func SaveState(path string, st State) error {
	if path == "" {
		return errors.New("update : pas de fichier d'état")
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".update-*.json")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(append(b, '\n'))
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

// Launched note le lancement de la version current et dit si c'est le
// premier lancement après un changement de version (previous : la version
// d'avant). Le tout premier lancement (aucune version notée) n'en est pas
// un. Une mise à jour en attente déjà installée (ou plus ancienne que la
// version courante) est oubliée ; ApplyError ne vaut que pour une mise à
// jour encore en attente.
func (st *State) Launched(current string) (previous string, updated bool) {
	previous = st.LastVersion
	updated = previous != "" && previous != current
	st.LastVersion = current
	cur, okCur := ParseVersion(current)
	if p := st.Pending; p != nil {
		pv, ok := ParseVersion(p.Version)
		if !ok || !okCur || !Newer(pv, cur) {
			st.Pending = nil
		}
	}
	if st.Pending == nil {
		st.ApplyError = ""
	}
	if st.Latest != "" {
		if lv, ok := ParseVersion(st.Latest); !ok || !okCur || !Newer(lv, cur) {
			st.Latest, st.LatestPage = "", ""
		}
	}
	return previous, updated
}

// stale : le fichier n'existe plus ou n'a plus l'empreinte vérifiée.
func (p *Pending) stale() bool {
	if p == nil || p.File == "" {
		return true
	}
	if _, err := os.Stat(p.File); errors.Is(err, fs.ErrNotExist) {
		return true
	}
	sum, err := FileSHA256(p.File)
	return err != nil || sum != p.SHA256
}
