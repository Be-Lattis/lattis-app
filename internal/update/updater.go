package update

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/msg"
)

// Phase : où en est la mise à jour, pour la barre d'état.
type Phase string

const (
	PhaseIdle        Phase = ""            // rien à signaler
	PhaseAvailable   Phase = "available"   // version plus récente, installable par l'application
	PhaseManual      Phase = "manual"      // version plus récente, à télécharger depuis sa page
	PhaseDownloading Phase = "downloading" // téléchargement et vérification
	PhaseReady       Phase = "ready"       // prête : redémarrer pour l'appliquer
	PhaseFailed      Phase = "failed"      // téléchargement ou vérification en échec
)

// Status est l'état montré par la barre d'état.
type Status struct {
	Phase Phase `json:"phase"`
	// Version : la version proposée (sans « v ») ; Page : sa page GitHub.
	Version string `json:"version,omitempty"`
	Page    string `json:"page,omitempty"`
	// Elevation : l'installation demandera les droits d'administration
	// (installation par machine, invite UAC à la fermeture).
	Elevation bool `json:"elevation,omitempty"`
	// Error : la raison d'un échec, dans la langue de la session.
	Error string `json:"error,omitempty"`
	// Updated : premier lancement après une mise à jour, la version
	// installée et la page de ses notes de version.
	Updated     string `json:"updated,omitempty"`
	UpdatedPage string `json:"updatedPage,omitempty"`

	// Paramètres › Mises à jour : la version installée, la dernière
	// recherche (RFC 3339, vide : jamais) et son échec éventuel (dans la
	// langue de la session), une recherche en cours, et pourquoi la
	// recherche est impossible (Unavailable, vide sinon).
	Current     string      `json:"current"`
	LastCheck   string      `json:"lastCheck,omitempty"`
	LastError   string      `json:"lastError,omitempty"`
	Checking    bool        `json:"checking,omitempty"`
	Unavailable Unavailable `json:"unavailable,omitempty"`
}

// Unavailable : pourquoi la recherche des nouvelles versions est impossible.
type Unavailable string

const (
	UnavailableDev    Unavailable = "dev"    // compilation de développement (version « dev »)
	UnavailableMSIX   Unavailable = "msix"   // paquet MSIX, mis à jour par Windows
	UnavailablePolicy Unavailable = "policy" // coupée par la stratégie de l'organisation
	UnavailableOff    Unavailable = "off"    // désactivée dans les paramètres
)

// Délais de la recherche : la première peu après le démarrage (jamais
// pendant qu'il se fait), puis au plus toutes les CheckEvery.
const (
	FirstCheckDelay = 30 * time.Second
	CheckEvery      = 12 * time.Hour
	checkTimeout    = 15 * time.Second
	// restartWindow : « Redémarrer » relance l'application si la fermeture
	// suit dans ce délai (l'utilisateur a pu l'annuler, puis fermer plus
	// tard sans vouloir de relance).
	restartWindow = 2 * time.Minute
)

// Updater mène la mise à jour de l'application de bureau.
type Updater struct {
	Current   string  // version de l'application (bridge.Version)
	Client    *Client // requêtes vers GitHub
	StateFile string  // update.json
	Dir       string  // dossier des installeurs téléchargés
	Exe       string  // exécutable en cours
	Kind      Kind
	// Enabled : la recherche est permise (updates.check), lu à chaque fois.
	Enabled func() bool
	// Locked : updates.check est imposé par la stratégie (nil : jamais).
	Locked func() bool
	// Verify : contrôle de la signature du fichier téléchargé (Authenticode
	// quand l'application est elle-même signée), nil : aucun.
	Verify func(path string) error
	// OnChange : l'état a changé (évènement vers l'interface).
	OnChange func(Status)
	Now      func() time.Time

	mu        sync.Mutex
	st        State
	phase     Phase
	err       error
	restartAt time.Time
	updated   string
	init      bool
	// Dernière recherche en échec (en mémoire seulement : une recherche
	// manquée ne repousse pas la suivante) et recherche en cours.
	checkErr error
	failedAt time.Time
	checking bool
	checkMu  sync.Mutex    // une recherche à la fois
	loaded   chan struct{} // Loaded : l'interface est chargée
	loadOnce sync.Once
}

// New prépare la mise à jour de l'exécutable en cours : genre
// d'installation, fichier d'état et dossier de téléchargement dans les
// dossiers de l'utilisateur.
func New(current string) *Updater {
	exe, _ := os.Executable()
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	u := &Updater{
		Current: current,
		Client:  &Client{HTTP: &http.Client{Timeout: 10 * time.Minute}, Version: current},
		Exe:     exe,
		Kind:    DetectKind(exe),
	}
	if dir, err := os.UserConfigDir(); err == nil {
		u.StateFile = filepath.Join(dir, "Lattis", "update.json")
	}
	if dir, err := os.UserCacheDir(); err == nil {
		u.Dir = filepath.Join(dir, "Lattis", "updates")
	}
	if SelfSigned(exe) {
		u.Verify = VerifySignature
	}
	return u
}

func (u *Updater) now() time.Time {
	if u.Now != nil {
		return u.Now()
	}
	return time.Now()
}

func (u *Updater) enabled() bool { return u.Enabled == nil || u.Enabled() }

// Checkable : une version publiée (pas « dev »), hors paquet MSIX (mis à
// jour par Windows).
func (u *Updater) Checkable() bool {
	_, ok := ParseVersion(u.Current)
	return ok && u.Kind != KindMSIX
}

// unavailable : pourquoi la recherche est impossible (vide : possible).
func (u *Updater) unavailable() Unavailable {
	if _, ok := ParseVersion(u.Current); !ok {
		return UnavailableDev
	}
	if u.Kind == KindMSIX {
		return UnavailableMSIX
	}
	if !u.enabled() {
		if u.Locked != nil && u.Locked() {
			return UnavailablePolicy
		}
		return UnavailableOff
	}
	return ""
}

// Init lit l'état, note le lancement (premier lancement après une mise à
// jour ?) et range les restes d'une mise à jour passée. À appeler une fois,
// au démarrage.
func (u *Updater) Init() {
	u.mu.Lock()
	if u.init {
		u.mu.Unlock()
		return
	}
	u.init = true
	u.st = LoadState(u.StateFile)
	prev, updated := u.st.Launched(u.Current)
	if updated {
		if pv, ok := ParseVersion(prev); ok {
			if cv, ok := ParseVersion(u.Current); ok && Newer(cv, pv) {
				u.updated = u.Current
			}
		}
	}
	if p := u.st.Pending; p != nil && p.stale() {
		u.st.Pending, u.st.ApplyError = nil, ""
	}
	if u.st.Pending != nil {
		u.phase = PhaseReady
		if u.st.ApplyError != "" {
			u.err = errors.New(u.st.ApplyError)
		}
	} else if u.st.Latest != "" {
		u.phase = u.offerPhase()
	}
	keep := ""
	if u.st.Pending != nil {
		keep = u.st.Pending.File
	}
	st := u.st
	u.mu.Unlock()
	cleanup(u.Exe, u.Dir, keep)
	if u.StateFile != "" {
		_ = SaveState(u.StateFile, st)
	}
}

// offerPhase : ce qu'on propose pour une version plus récente.
func (u *Updater) offerPhase() Phase {
	if u.Kind.Automatic() {
		return PhaseAvailable
	}
	return PhaseManual
}

// Status rend l'état, ses textes dans la langue l.
func (u *Updater) Status(l msg.Lang) Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.status(l)
}

func (u *Updater) status(l msg.Lang) Status {
	s := Status{}
	if u.updated != "" {
		s.Updated, s.UpdatedPage = u.updated, ReleasePage(u.Client.Repo, u.updated)
	}
	phase := u.phase
	// Recherche coupée (stratégie, réglage) : plus de proposition ; une
	// mise à jour déjà téléchargée reste à appliquer.
	if !u.enabled() && phase != PhaseReady {
		phase = PhaseIdle
	}
	s.Phase = phase
	switch phase {
	case PhaseIdle:
	case PhaseReady:
		if p := u.st.Pending; p != nil {
			s.Version, s.Page = p.Version, p.Page
			s.Elevation = p.Kind.NeedsElevation()
		}
	default:
		if v, ok := ParseVersion(u.st.Latest); ok {
			s.Version = v.String()
		}
		s.Page = u.st.LatestPage
		s.Elevation = u.Kind.NeedsElevation() && phase != PhaseManual
	}
	if u.err != nil && (phase == PhaseFailed || phase == PhaseReady) {
		s.Error = msg.Text(l, u.err)
	}
	s.Current = u.Current
	last := u.st.LastCheck
	if u.checkErr != nil && u.failedAt.After(last) {
		last = u.failedAt
		s.LastError = msg.Text(l, u.checkErr)
	}
	if !last.IsZero() {
		s.LastCheck = last.UTC().Format(time.RFC3339)
	}
	s.Checking = u.checking
	s.Unavailable = u.unavailable()
	return s
}

func (u *Updater) changed() {
	if u.OnChange != nil {
		u.OnChange(u.Status(msg.Current()))
	}
}

func (u *Updater) save() {
	if u.StateFile != "" {
		_ = SaveState(u.StateFile, u.st)
	}
}

// Loaded signale que l'interface a fini de charger : la première
// recherche part tout de suite (demande de Louis), sans attendre
// FirstCheckDelay ni les CheckEvery écoulés depuis la précédente.
func (u *Updater) Loaded() {
	u.loadOnce.Do(func() { close(u.loadedCh()) })
}

func (u *Updater) loadedCh() chan struct{} {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.loaded == nil {
		u.loaded = make(chan struct{})
	}
	return u.loaded
}

// Run cherche les nouvelles versions en arrière-plan jusqu'à la fin de ctx :
// la première fois dès que l'interface est chargée (Loaded), au plus tard
// après first, puis toutes les heures si la dernière recherche date de plus
// de CheckEvery. Rien pour une version « dev » ni sous MSIX.
func (u *Updater) Run(ctx context.Context, first time.Duration) {
	u.Init()
	u.changed()
	if !u.Checkable() {
		return
	}
	wait := first
	loaded := u.loadedCh()
	for start := true; ; start = false {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		case <-loaded:
		}
		loaded = nil // une seule fois
		wait = time.Hour
		if u.enabled() && (start || u.due()) {
			c, cancel := context.WithTimeout(ctx, checkTimeout)
			_ = u.Check(c) // hors ligne, proxy : silencieux, on réessaiera
			cancel()
		}
		u.changed()
	}
}

func (u *Updater) due() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.now().Sub(u.st.LastCheck) >= CheckEvery
}

// CheckNow : « Rechercher maintenant » (Paramètres › Mises à jour) : la
// recherche part tout de suite, sans tenir compte des CheckEvery écoulés
// depuis la précédente ; rien si elle est impossible (Unavailable). Rend
// l'état ensuite, textes dans la langue l ; un échec y est (LastError).
func (u *Updater) CheckNow(ctx context.Context, l msg.Lang) Status {
	u.mu.Lock()
	ok := u.unavailable() == ""
	u.mu.Unlock()
	if ok {
		c, cancel := context.WithTimeout(ctx, checkTimeout)
		_ = u.Check(c)
		cancel()
	}
	return u.Status(l)
}

// Check cherche sur GitHub une version plus récente que la version
// courante (même canal : pré-versions comprises pour une bêta). Un échec
// est gardé pour Paramètres › Mises à jour, dans la langue de la session.
func (u *Updater) Check(ctx context.Context) error {
	cur, ok := ParseVersion(u.Current)
	if !ok {
		return nil
	}
	u.checkMu.Lock()
	defer u.checkMu.Unlock()
	u.mu.Lock()
	u.checking = true
	u.mu.Unlock()
	u.changed()
	list, err := u.Client.Releases(ctx)
	if err != nil {
		err = checkError(ctx, err)
		u.mu.Lock()
		u.checking = false
		u.checkErr, u.failedAt = err, u.now().UTC()
		u.mu.Unlock()
		u.changed()
		return err
	}
	best := Newest(list, cur)
	u.mu.Lock()
	u.checking = false
	u.checkErr, u.failedAt = nil, time.Time{}
	u.st.LastCheck = u.now().UTC()
	if best == nil {
		u.st.Latest, u.st.LatestPage = "", ""
		if u.phase != PhaseReady && u.phase != PhaseDownloading {
			u.phase = PhaseIdle
		}
	} else {
		changed := u.st.Latest != best.Tag
		u.st.Latest, u.st.LatestPage = best.Tag, best.URL
		switch {
		case u.phase == PhaseDownloading:
		case u.phase == PhaseReady && !changed:
		case u.phase == PhaseReady:
			// Plus récente que celle déjà téléchargée : on la propose.
			u.st.Pending, u.err = nil, nil
			u.phase = u.offerPhase()
		default:
			u.phase, u.err = u.offerPhase(), nil
		}
	}
	u.save()
	u.mu.Unlock()
	u.changed()
	return nil
}

// checkError : l'échec d'une recherche, dit simplement (hors ligne, délai
// dépassé, proxy) ; les erreurs déjà traduites (limite de GitHub, réponse
// inattendue…) restent telles quelles.
func checkError(ctx context.Context, err error) error {
	var m msg.Messager
	if errors.As(err, &m) {
		return err
	}
	var ne net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), ctx.Err() == context.DeadlineExceeded,
		errors.As(err, &ne) && ne.Timeout():
		return msg.Wrap(err, "update.timeout", nil)
	case errors.Is(err, context.Canceled):
		return err
	}
	return msg.Wrap(err, "update.unreachable", nil)
}

// Prepare télécharge et vérifie la version proposée (clic dans la barre
// d'état) ; elle est ensuite installée à la fermeture de l'application.
func (u *Updater) Prepare(ctx context.Context) error {
	u.mu.Lock()
	tag := u.st.Latest
	if u.phase == PhaseDownloading || u.phase == PhaseReady || tag == "" || !u.Kind.Automatic() {
		u.mu.Unlock()
		return nil
	}
	u.phase, u.err = PhaseDownloading, nil
	u.mu.Unlock()
	u.changed()

	p, manual, err := u.download(ctx, tag)
	u.mu.Lock()
	switch {
	case manual:
		u.phase = PhaseManual
	case err != nil:
		u.phase, u.err = PhaseFailed, err
	default:
		u.st.Pending, u.st.ApplyError = p, ""
		u.phase = PhaseReady
		u.save()
	}
	u.mu.Unlock()
	u.changed()
	return err
}

// download : téléchargement et vérifications ; manual si l'application ne
// peut pas s'installer elle-même (dossier de l'exécutable portable en
// lecture seule).
func (u *Updater) download(ctx context.Context, tag string) (*Pending, bool, error) {
	name, _ := AssetFor(u.Kind)
	exeDir := filepath.Dir(u.Exe)
	var dest string
	if u.Kind == KindPortable {
		if !writable(exeDir) {
			return nil, true, nil
		}
		dest = u.Exe + ".new"
	} else {
		if u.Dir == "" {
			return nil, false, msg.New("update.noFolder", nil)
		}
		dest = filepath.Join(u.Dir, tag, name)
	}
	rel, err := u.Client.ReleaseByTag(ctx, tag)
	if err != nil {
		return nil, false, err
	}
	v, ok := rel.Version()
	cur, _ := ParseVersion(u.Current)
	if !ok || !Newer(v, cur) {
		// Jamais de retour en arrière, même si GitHub le proposait.
		return nil, false, msg.New("update.notNewer", msg.Args{"version": rel.Tag})
	}
	asset, ok := rel.Asset(name)
	if !ok {
		return nil, false, msg.New("update.noAsset", msg.Args{"name": name, "version": v.String()})
	}
	want, err := u.Client.Expected(ctx, rel, name)
	if err != nil {
		return nil, false, err
	}
	if want.Size == 0 && asset.Size > 0 {
		want.Size = asset.Size
	}
	if err := u.Client.Download(ctx, asset.URL, dest, want); err != nil {
		return nil, false, err
	}
	if u.Verify != nil {
		if err := u.Verify(dest); err != nil {
			os.Remove(dest)
			return nil, false, msg.New("update.notSigned", msg.Args{"name": name})
		}
	}
	return &Pending{Version: v.String(), Page: rel.URL, Kind: u.Kind, File: dest, SHA256: want.SHA256, Dir: exeDir}, false, nil
}

// RequestRestart : l'utilisateur a demandé à redémarrer pour appliquer la
// mise à jour ; l'application se relancera si elle se ferme bientôt.
func (u *Updater) RequestRestart() {
	u.mu.Lock()
	u.restartAt = u.now()
	u.mu.Unlock()
}

// DismissUpdated : l'avis « Mise à jour effectuée » a été lu ou fermé.
func (u *Updater) DismissUpdated() {
	u.mu.Lock()
	u.updated = ""
	u.mu.Unlock()
	u.changed()
}

// ApplyOnExit installe la mise à jour prête, à la fermeture de
// l'application : lance l'installeur (silencieux ; par machine, avec
// l'invite UAC) ou remplace l'exécutable portable, puis relance
// l'application si l'utilisateur a demandé à redémarrer. Un échec est
// gardé pour être dit au lancement suivant.
func (u *Updater) ApplyOnExit() error {
	u.mu.Lock()
	p := u.st.Pending
	relaunch := !u.restartAt.IsZero() && u.now().Sub(u.restartAt) < restartWindow
	u.mu.Unlock()
	if p == nil {
		return nil
	}
	if p.stale() {
		u.mu.Lock()
		u.st.Pending = nil
		u.save()
		u.mu.Unlock()
		return msg.New("update.fileChanged", nil)
	}
	err := apply(*p, u.Exe, relaunch)
	u.mu.Lock()
	if err != nil {
		u.st.ApplyError = msg.Text(msg.Current(), err)
	} else {
		u.st.ApplyError = ""
	}
	u.save()
	u.mu.Unlock()
	return err
}

// replaceExe met next à la place de exe, en cours d'exécution : Windows
// permet de renommer un exécutable chargé (« .old », supprimé au lancement
// suivant), jamais de le réécrire.
func replaceExe(exe, next string) error {
	old := exe + ".old"
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(next, exe); err != nil {
		_ = os.Rename(old, exe)
		return err
	}
	return nil
}

// writable dit si on peut écrire dans un dossier (essai d'un fichier).
func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".lattis-write-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

// cleanup range les restes d'une mise à jour : l'ancien exécutable
// portable (« .old ») et les installeurs déjà appliqués (sauf keep).
func cleanup(exe, dir, keep string) {
	if exe != "" {
		os.Remove(exe + ".old")
		if keep != exe+".new" {
			os.Remove(exe + ".new")
		}
	}
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if keep != "" && filepath.Dir(keep) == p {
			continue
		}
		os.RemoveAll(p)
	}
}
