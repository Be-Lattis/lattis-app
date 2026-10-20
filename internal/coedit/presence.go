package coedit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
)

// --- Dossier, sessions, présences --------------------------------------------

// DirOf donne le dossier du travail à plusieurs d'un classeur :
// « ~$Ventes.lattis.d » à côté de « Ventes.lattis ».
func DirOf(path string) string {
	return filepath.Join(filepath.Dir(path), "~$"+filepath.Base(path)+".d")
}

const (
	presenceExt = ".presence"
	journalExt  = ".jsonl"
)

// États d'une présence.
const (
	StateJoining = "joining" // demande à rejoindre, attend les autres
	StateReady   = "ready"   // relève ses transactions dans son journal
)

// Presence est le fichier de présence d'une session, réécrit à chaque
// battement.
type Presence struct {
	V       int       `json:"v"`
	Session string    `json:"session"`
	User    string    `json:"user"`
	Login   string    `json:"login"`
	Machine string    `json:"machine"`
	PID     int       `json:"pid"`
	Since   time.Time `json:"since"`
	// Beat : compteur de battements (la vivacité se juge sur son
	// changement, à l'horloge de l'observateur).
	Beat  int64  `json:"beat"`
	State string `json:"state"`
	// Journal : segment en cours ; Seq : sa dernière ligne écrite.
	Journal string `json:"journal,omitempty"`
	Seq     int64  `json:"seq,omitempty"`
	// Applied : positions appliquées de chaque journal (accusés de
	// réception, pour le compactage).
	Applied map[string]int64 `json:"applied,omitempty"`
	// Welcome : sessions qui demandent à rejoindre et que celle-ci a
	// accueillies (plus rien d'elle hors journal : le fichier enregistré et
	// les journaux suffisent pour la rattraper).
	Welcome []string `json:"welcome,omitempty"`
}

// Name donne le nom affiché d'une présence : « Alice (PC-COMPTA) ».
func (p Presence) Name() string {
	who := p.User
	if who == "" {
		who = p.Login
	}
	if who == "" {
		who = "un autre utilisateur"
	}
	return who
}

// newSession donne l'identifiant d'une nouvelle session :
// <poste>-<identifiant>-<pid>-<4 caractères aléatoires>.
func newSession(me archive.Holder) string {
	var b [2]byte
	rand.Read(b[:])
	return clean(me.Machine) + "-" + clean(me.Login) + "-" + strconv.Itoa(me.PID) + "-" + hex.EncodeToString(b[:])
}

// clean garde des lettres, chiffres, « . » et « _ » (nom de fichier sûr
// sur tous les systèmes, sans « - », séparateur des parties).
func clean(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "x"
	}
	return b.String()
}

// segmentName : journal <session>-<n>.
func segmentName(sid string, n int) string { return sid + "-" + strconv.Itoa(n) }

// sessionOf donne la session d'un journal (le nom sans son numéro de
// segment).
func sessionOf(journal string) string {
	if i := strings.LastIndexByte(journal, '-'); i > 0 {
		return journal[:i]
	}
	return journal
}

// writePresence remplace le fichier de présence (fichier temporaire puis
// renommage : un lecteur ne voit jamais un fichier à moitié écrit).
func writePresence(dir string, p Presence) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	archive.Hide(dir)
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	name := filepath.Join(dir, p.Session+presenceExt)
	tmp := name + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	var rerr error
	for i := range 5 { // Windows : un lecteur peut tenir le fichier un instant
		if rerr = os.Rename(tmp, name); rerr == nil {
			return nil
		}
		time.Sleep(time.Duration(i+1) * 20 * time.Millisecond)
	}
	os.Remove(tmp)
	return rerr
}

func removePresence(dir, sid string) {
	os.Remove(filepath.Join(dir, sid+presenceExt))
}

// --- Vivacité -----------------------------------------------------------------

// seen suit une présence observée.
type seen struct {
	p Presence
	// at : dernier changement du compteur de battements vu (horloge
	// locale).
	at time.Time
}

// roster tient les présences observées dans le dossier.
type roster struct {
	self  string
	stale time.Duration
	m     map[string]*seen
}

func newRoster(self string, stale time.Duration) *roster {
	return &roster{self: self, stale: stale, m: map[string]*seen{}}
}

// scan relit les présences du dossier (entries : sa liste) ; rend les
// présences vivantes (hors soi) triées par session.
func (r *roster) scan(dir string, entries []os.DirEntry, now time.Time) []Presence {
	present := map[string]bool{}
	me := archive.Me()
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, presenceExt) {
			continue
		}
		sid := strings.TrimSuffix(n, presenceExt)
		if sid == r.self {
			continue
		}
		present[sid] = true
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			continue // en cours de remplacement : la dernière lecture vaut
		}
		var p Presence
		if json.Unmarshal(b, &p) != nil || p.Session != sid {
			continue
		}
		s := r.m[sid]
		switch {
		case s == nil:
			s = &seen{p: p, at: now}
			// Première vue : l'heure du fichier dit si elle bat encore (à
			// l'horloge du NAS ou de son poste ; au pire, elle passe pour
			// vivante un intervalle de trop, ou morte jusqu'au battement
			// suivant).
			if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) > r.stale {
				s.at = now.Add(-r.stale)
			}
			r.m[sid] = s
		case p.Beat != s.p.Beat || p.State != s.p.State:
			s.at = now
		}
		s.p = p
		if strings.EqualFold(p.Machine, me.Machine) && p.PID != me.PID && !archive.ProcessAlive(p.PID) {
			s.at = time.Time{} // processus disparu sur ce poste
		}
	}
	for sid := range r.m {
		if !present[sid] {
			delete(r.m, sid) // partie proprement
		}
	}
	return r.live(now)
}

// live rend les présences vivantes (hors soi).
func (r *roster) live(now time.Time) []Presence {
	var out []Presence
	for _, s := range r.m {
		if now.Sub(s.at) < r.stale {
			out = append(out, s.p)
		}
	}
	slices.SortFunc(out, func(a, b Presence) int { return strings.Compare(a.Session, b.Session) })
	return out
}

// AwayKeep : durée pendant laquelle une session qui ne bat plus (poste
// coupé du dossier partagé : Wi-Fi, veille) est encore attendue. Ses
// présence et segments ne sont pas retirés, et les segments qu'elle n'a pas
// appliqués ne sont pas compactés : revenue, elle reçoit ce que les autres
// ont fait pendant sa coupure (sinon elle le manquait pour toujours, sans
// message). Un processus disparu sur ce poste n'est pas attendu.
var AwayKeep = 24 * time.Hour

// away rend les présences des sessions qui ne battent plus depuis moins de
// AwayKeep (hors soi) : encore attendues.
func (r *roster) away(now time.Time) []Presence {
	var out []Presence
	for _, s := range r.m {
		if d := now.Sub(s.at); d >= r.stale && d < AwayKeep && !s.at.IsZero() {
			out = append(out, s.p)
		}
	}
	return out
}

// staleSessions rend les sessions observées qui ne battent plus et ne sont
// plus attendues (AwayKeep dépassé, ou processus disparu sur ce poste).
func (r *roster) staleSessions(now time.Time) []string {
	var out []string
	for sid, s := range r.m {
		if d := now.Sub(s.at); d >= r.stale && (d >= AwayKeep || s.at.IsZero()) {
			out = append(out, sid)
		}
	}
	return out
}

// --- Rejoindre -------------------------------------------------------------------

// Ticket est la demande acceptée d'une session qui rejoint : elle ouvre
// alors le fichier et démarre (Start) avec la même présence.
type Ticket struct {
	path string
	dir  string
	p    Presence
}

// NoAnswerError : une session ouverte n'a pas accueilli la demande à temps
// (version de Lattis sans travail à plusieurs, réglage désactivé, poste
// injoignable).
type NoAnswerError struct {
	Who string
}

func (e *NoAnswerError) Error() string {
	return fmt.Sprintf("%s n'a pas répondu à la demande de travail à plusieurs (version de Lattis plus ancienne, travail à plusieurs désactivé sur son poste, ou poste injoignable)", e.Who)
}

// Join demande à rejoindre le travail à plusieurs sur le classeur path :
// présence « joining », puis attente que chaque session vivante l'ait
// accueillie (Presence.Welcome), dont celle qui tient le verrou ~$. Sans
// réponse au bout de t.JoinWait : *NoAnswerError, la présence est retirée.
func Join(ctx context.Context, path string, t Timing) (*Ticket, error) {
	t = t.withDefaults()
	me := archive.Me()
	now := time.Now()
	p := Presence{V: 1, Session: newSession(me), User: me.User, Login: me.Login, Machine: me.Machine, PID: me.PID, Since: now.UTC().Truncate(time.Second), State: StateJoining}
	dir := DirOf(path)
	if err := writePresence(dir, p); err != nil {
		return nil, fmt.Errorf("travail à plusieurs impossible : le dossier de « %s » n'accepte pas l'écriture (%w)", filepath.Base(path), err)
	}
	r := newRoster(p.Session, t.Stale)
	deadline := now.Add(t.JoinWait)
	lastBeat := now
	fail := func(err error) (*Ticket, error) {
		removePresence(dir, p.Session)
		removeIfEmpty(dir)
		return nil, err
	}
	for {
		now = time.Now()
		entries, _ := os.ReadDir(dir)
		live := r.scan(dir, entries, now)
		holder := archive.HeldBy(path)
		ok := true
		who := ""
		for _, q := range live {
			if q.State == StateJoining {
				continue // une autre demande en même temps
			}
			if q.State != StateReady || !slices.Contains(q.Welcome, p.Session) {
				ok, who = false, q.Name()
			}
		}
		if holder != nil && !holderLive(*holder, live) {
			ok = false
			if who == "" {
				who = holder.User
			}
		}
		if ok {
			return &Ticket{path: path, dir: dir, p: p}, nil
		}
		if now.After(deadline) {
			if who == "" {
				who = "Le poste qui a ouvert le classeur"
			}
			return fail(&NoAnswerError{Who: who})
		}
		if now.Sub(lastBeat) >= t.Beat {
			p.Beat++
			writePresence(dir, p)
			lastBeat = now
		}
		select {
		case <-ctx.Done():
			return fail(ctx.Err())
		case <-time.After(t.Poll / 2):
		}
	}
}

// holderLive : le détenteur du verrou ~$ est une session vivante prête.
func holderLive(h archive.Holder, live []Presence) bool {
	for _, q := range live {
		if q.State == StateReady && strings.EqualFold(q.Machine, h.Machine) && q.PID == h.PID {
			return true
		}
	}
	return false
}

// Cancel abandonne une demande acceptée (ouverture du fichier impossible).
func (t *Ticket) Cancel() {
	if t == nil {
		return
	}
	removePresence(t.dir, t.p.Session)
	removeIfEmpty(t.dir)
}

// removeIfEmpty supprime le dossier s'il ne contient plus rien.
func removeIfEmpty(dir string) {
	if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
		os.Remove(dir)
	}
}

// Others dit si d'autres sessions vivantes travaillent sur le classeur
// path (présences du dossier), et lesquelles : à l'ouverture, la question
// « Travailler à plusieurs » se pose même sans verrou ~$.
func Others(path string, t Timing) []Presence {
	t = t.withDefaults()
	dir := DirOf(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	r := newRoster("", t.Stale)
	return r.scan(dir, entries, time.Now())
}

var errClosed = errors.New("travail à plusieurs arrêté")
