package archive

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/lalamefine/lattis/internal/msg"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// --- Verrou à côté du fichier (~$nom.lattis) --------------------------------
//
// Comme Excel : un petit fichier caché à côté du classeur dit qui l'a ouvert
// (utilisateur, poste, heure), pour prévenir un autre poste qui l'ouvre depuis
// un dossier partagé. Il est rafraîchi pendant que le classeur est ouvert et
// supprimé à la fermeture ; un verrou abandonné (processus disparu sur ce
// poste, ou pas rafraîchi depuis lockStale) est ignoré.

const peerPrefix = "~$"

var (
	// lockRefresh : intervalle de rafraîchissement du verrou.
	lockRefresh = 15 * time.Minute
	// lockStale : âge au-delà duquel un verrou non rafraîchi est abandonné.
	lockStale = 12 * time.Hour
)

// Holder dit qui tient un classeur ouvert.
type Holder struct {
	User    string    `json:"user"`  // nom affiché (ou identifiant)
	Login   string    `json:"login"` // identifiant de session
	Machine string    `json:"machine"`
	PID     int       `json:"pid"`
	Since   time.Time `json:"since"`
}

// Self dit si le verrou vient de cet utilisateur.
func (h Holder) self() bool {
	me := whoAmI()
	return strings.EqualFold(h.Login, me.Login) && strings.EqualFold(h.Machine, me.Machine)
}

// String décrit le détenteur pour l'utilisateur.
func (h Holder) String() string { return h.Msg().String() }

// Msg : qui tient le verrou, comme message (« Jean sur le poste PC-1,
// depuis le 02/01/2026 à 15:04 »).
func (h Holder) Msg() msg.Message {
	var who any = h.User
	if h.User == "" {
		who = msg.M("archive.holder.someone", nil)
	}
	if h.self() {
		who = msg.M("archive.holder.selfWindow", nil)
	} else if strings.EqualFold(h.Login, whoAmI().Login) {
		who = msg.M("archive.holder.self", nil)
	}
	args := msg.Args{"who": who}
	id := "archive.holder"
	if h.Machine != "" && !h.self() {
		id += "Machine"
		args["machine"] = h.Machine
	}
	if !h.Since.IsZero() {
		id += "Since"
		args["date"] = h.Since.Local().Format("02/01/2006")
		args["time"] = h.Since.Local().Format("15:04")
	}
	return msg.M(msg.ID(id), args)
}

var whoAmI = sync.OnceValue(func() Holder {
	h := Holder{PID: os.Getpid()}
	if u, err := user.Current(); err == nil {
		h.Login = u.Username
		h.User = u.Name
		if h.User == "" {
			h.User = u.Username
		}
		// DOMAINE\nom : seul le nom parle à l'utilisateur.
		if i := strings.LastIndexAny(h.User, `\/`); i >= 0 && h.User == h.Login {
			h.User = h.User[i+1:]
		}
	}
	h.Machine, _ = os.Hostname()
	return h
})

// Me décrit l'utilisateur et le poste de ce processus (comme dans le
// verrou ~$).
func Me() Holder { return whoAmI() }

// ProcessAlive dit si le processus pid de ce poste existe encore.
func ProcessAlive(pid int) bool { return processAlive(pid) }

// Hide cache un fichier ou un dossier (Windows ; sans effet ailleurs).
func Hide(path string) { hide(path) }

// LockPath donne le fichier de verrou d'un classeur.
func LockPath(path string) string {
	return filepath.Join(filepath.Dir(path), peerPrefix+filepath.Base(path))
}

// LockedError : le classeur est ouvert ailleurs (verrou ~$ vivant).
type LockedError struct {
	Path   string
	Holder Holder
}

func (e *LockedError) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e *LockedError) Msg() msg.Message {
	return msg.M("archive.locked", msg.Args{"file": filepath.Base(e.Path), "holder": e.Holder.Msg()})
}

// LockedBy rend le détenteur du verrou vivant de path, nil s'il n'y en a pas
// (ou s'il est abandonné, ou illisible).
func LockedBy(path string) *Holder {
	h, ok := readPeer(LockPath(path))
	if !ok || !alive(h, LockPath(path)) {
		return nil
	}
	return &h
}

// HeldBy rend le détenteur du verrou vivant de path comme LockedBy, ce
// processus compris (travail à plusieurs : la session qui le tient doit
// accueillir celle qui rejoint, même dans le même processus).
func HeldBy(path string) *Holder {
	lp := LockPath(path)
	h, ok := readPeer(lp)
	if !ok {
		return nil
	}
	if h.PID == os.Getpid() && strings.EqualFold(h.Machine, whoAmI().Machine) {
		if st, err := os.Stat(lp); err != nil || time.Since(st.ModTime()) > lockStale {
			return nil
		}
		return &h
	}
	if !alive(h, lp) {
		return nil
	}
	return &h
}

func readPeer(lp string) (Holder, bool) {
	b, err := os.ReadFile(lp)
	if err != nil {
		return Holder{}, false
	}
	var h Holder
	if json.Unmarshal(b, &h) != nil {
		return Holder{}, false
	}
	return h, true
}

// alive : verrou rafraîchi depuis moins de lockStale et, sur ce poste,
// processus encore là. Un verrou de ce processus-ci ne compte pas : le
// verrou du répertoire de travail empêche déjà d'ouvrir deux fois le même
// fichier (ErrAlreadyOpen), et le numéro a pu resservir.
func alive(h Holder, lp string) bool {
	st, err := os.Stat(lp)
	if err != nil || time.Since(st.ModTime()) > lockStale {
		return false
	}
	if strings.EqualFold(h.Machine, whoAmI().Machine) {
		return h.PID > 0 && h.PID != os.Getpid() && processAlive(h.PID)
	}
	return true
}

// peerLock est le verrou posé par ce document.
type peerLock struct {
	path string
	data []byte
	stop chan struct{}
	done chan struct{}
}

// takePeer pose le verrou de path. Un verrou vivant d'un autre donne un
// *LockedError ; un verrou abandonné est remplacé.
func takePeer(path string) (*peerLock, error) {
	lp := LockPath(path)
	me := whoAmI()
	me.Since = time.Now().UTC().Truncate(time.Second)
	data, _ := json.Marshal(me)
	for range 2 {
		f, err := os.OpenFile(lp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			if h, ok := readPeer(lp); ok && alive(h, lp) {
				return nil, &LockedError{Path: path, Holder: h}
			}
			os.Remove(lp) // abandonné ou illisible
			continue
		}
		if err != nil {
			return nil, err
		}
		_, err = f.Write(data)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(lp)
			return nil, err
		}
		hide(lp)
		l := &peerLock{path: lp, data: data, stop: make(chan struct{}), done: make(chan struct{})}
		go l.refresh()
		return l, nil
	}
	return nil, msg.New("archive.lockFailed", msg.Args{"lock": lp})
}

// refresh garde le verrou à jour tant que le document est ouvert (recréé
// s'il a disparu).
func (l *peerLock) refresh() {
	defer close(l.done)
	t := time.NewTicker(lockRefresh)
	defer t.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-t.C:
			now := time.Now()
			if err := os.Chtimes(l.path, now, now); errors.Is(err, os.ErrNotExist) {
				if os.WriteFile(l.path, l.data, 0o644) == nil {
					hide(l.path)
				}
			}
		}
	}
}

// release retire le verrou, s'il est toujours le nôtre.
func (l *peerLock) release() {
	if l == nil {
		return
	}
	close(l.stop)
	<-l.done
	if b, err := os.ReadFile(l.path); err == nil && bytes.Equal(b, l.data) {
		os.Remove(l.path)
	}
}

// --- Modification du fichier par ailleurs ------------------------------------

// Stamp identifie le contenu d'un fichier .lattis sur disque : taille, heure
// et empreinte (manifeste et répertoire du ZIP, dont le CRC de la base :
// rapide, même sur un gros classeur).
type Stamp struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	Sum     string    `json:"sum"`
}

const stampName = "disk.json" // dans le répertoire de travail

func stampOf(path string) (Stamp, error) {
	st, err := os.Stat(path)
	if err != nil {
		return Stamp{}, err
	}
	return Stamp{Size: st.Size(), ModTime: st.ModTime(), Sum: zipSum(path)}, nil
}

// zipSum : empreinte du manifeste et des entrées du ZIP (vide si illisible).
func zipSum(path string) string {
	zr, err := OpenZip(path)
	if err != nil {
		return ""
	}
	defer zr.Close()
	h := sha256.New()
	for _, f := range zr.File {
		io.WriteString(h, f.Name)
		binary.Write(h, binary.LittleEndian, [3]uint64{uint64(f.CRC32), f.UncompressedSize64, f.CompressedSize64})
		if f.Name == manifestName && f.UncompressedSize64 < 1<<20 {
			if r, err := f.Open(); err == nil {
				io.Copy(h, r)
				r.Close()
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// same : le fichier n'a pas changé depuis s (heure et taille identiques, ou
// même contenu : outil de synchronisation qui retouche l'heure).
func (s Stamp) same(path string) (bool, error) {
	st, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil // disparu : l'enregistrer le recrée, rien à écraser
	}
	if err != nil {
		return false, err
	}
	if st.Size() == s.Size && st.ModTime().Equal(s.ModTime) {
		return true, nil
	}
	sum := zipSum(path)
	return sum != "" && sum == s.Sum, nil
}

func writeStamp(dir string, s Stamp) {
	if b, err := json.Marshal(s); err == nil {
		os.WriteFile(filepath.Join(dir, stampName), b, 0o644)
	}
}

func readStamp(dir string) *Stamp {
	b, err := os.ReadFile(filepath.Join(dir, stampName))
	if err != nil {
		return nil
	}
	var s Stamp
	if json.Unmarshal(b, &s) != nil {
		return nil
	}
	return &s
}

// ChangedError : le fichier a été modifié sur disque (autre poste, autre
// programme) depuis son ouverture ou son dernier enregistrement.
type ChangedError struct{ Path string }

func (e *ChangedError) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e *ChangedError) Msg() msg.Message {
	return msg.M("archive.changed", msg.Args{"file": filepath.Base(e.Path)})
}

// ErrReadOnly : classeur ouvert en lecture seule, à enregistrer sous un
// autre nom.
var ErrReadOnly = msg.New("archive.readOnly", nil)
