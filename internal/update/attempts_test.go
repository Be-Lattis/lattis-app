package update

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/lalamefine/lattis/internal/msg"
)

// readyUpdater : un Updater en 0.2.0-beta avec la 0.3.0-beta téléchargée
// et prête (installation pour l'utilisateur).
func readyUpdater(t *testing.T) (*Updater, *fakeGitHub) {
	t.Helper()
	gh := newFakeGitHub(t)
	gh.publish("v0.3.0-beta", true, true, map[string][]byte{AssetSetup: []byte("setup")})
	u := newTestUpdater(t, gh, "0.2.0-beta", KindUser)
	u.Init()
	ctx := context.Background()
	if err := u.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if err := u.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	if s := u.Status(msg.FR); s.Phase != PhaseReady {
		t.Fatalf("prête : %+v", s)
	}
	return u, gh
}

// relaunch : le lancement suivant, même version (l'installation n'a rien
// changé), même état et même dossier de téléchargement.
func relaunch(t *testing.T, gh *fakeGitHub, prev *Updater) *Updater {
	t.Helper()
	u := newTestUpdater(t, gh, prev.Current, prev.Kind)
	u.StateFile, u.Dir, u.Exe = prev.StateFile, prev.Dir, prev.Exe
	u.Apply, u.Launch = prev.Apply, prev.Launch
	u.Init()
	return u
}

// utf16File : le fichier de trace tel que l'écrit FileWriteUTF16LE.
func utf16File(text string) []byte {
	var b []byte
	for _, r := range utf16.Encode([]rune(text)) {
		b = append(b, byte(r), byte(r>>8))
	}
	return b
}

func TestApplyAttemptRecorded(t *testing.T) {
	u, _ := readyUpdater(t)
	var got []string
	u.Apply = func(p Pending, exe string, relaunch bool) error {
		got = append(got, p.Version)
		return nil
	}
	if err := u.ApplyOnExit(); err != nil {
		t.Fatal(err)
	}
	// Une seule fois, même appelée deux fois (fermeture puis retour de app.Run).
	if err := u.ApplyOnExit(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "0.3.0-beta" {
		t.Fatalf("installeur lancé : %v", got)
	}
	st := LoadState(u.StateFile)
	if st.Pending == nil || len(st.Pending.Attempts) != 1 {
		t.Fatalf("essai non noté : %+v", st.Pending)
	}
	a := st.Pending.Attempts[0]
	if a.Outcome != OutcomeLaunched || a.At.IsZero() || a.Error != "" {
		t.Errorf("essai : %+v", a)
	}
}

func TestApplyLaunchFailureRecorded(t *testing.T) {
	u, gh := readyUpdater(t)
	u.Apply = func(Pending, string, bool) error {
		return msg.New("update.launchFailed", msg.Args{"err": "accès refusé"})
	}
	if err := u.ApplyOnExit(); err == nil {
		t.Fatal("échec attendu")
	}
	a := LoadState(u.StateFile).Pending.Attempts[0]
	if a.Outcome != OutcomeFailed || !strings.Contains(a.Error, "accès refusé") {
		t.Errorf("essai : %+v", a)
	}
	s := relaunch(t, gh, u).Status(msg.FR)
	if s.Phase != PhaseInstallFailed || !strings.Contains(s.Error, "accès refusé") || s.Failures != 1 {
		t.Errorf("au lancement suivant : %+v", s)
	}
}

// Le cas de la 0.3.2-beta : installeur lancé, rien d'installé ; au
// lancement suivant, l'échec est dit au lieu de « Redémarrez… », avec la
// raison laissée par l'installeur ; après deux échecs, plus
// d'installation automatique.
func TestInstallFailureDetectedAtNextLaunch(t *testing.T) {
	u, gh := readyUpdater(t)
	launches := 0
	u.Apply = func(Pending, string, bool) error { launches++; return nil }
	if err := u.ApplyOnExit(); err != nil {
		t.Fatal(err)
	}
	// Aucune trace de l'installeur.
	u2 := relaunch(t, gh, u)
	s := u2.Status(msg.FR)
	if s.Phase != PhaseInstallFailed || s.Version != "0.3.0-beta" || s.Failures != 1 || s.AutoStopped || !s.Installer {
		t.Fatalf("premier échec : %+v", s)
	}
	if !strings.Contains(s.Error, "aucune trace") {
		t.Errorf("raison : %q", s.Error)
	}
	if s.Page == "" {
		t.Error("page de la version attendue")
	}
	// Deuxième essai : l'installeur dit que lattis.exe était occupé.
	u2.Apply = func(Pending, string, bool) error {
		launches++
		return os.WriteFile(filepath.Join(u2.Dir, InstallResultName), utf16File("version=0.3.0\r\nstatus=locked\r\ncode=5\r\ndir=C:\\Users\\Zoé\\Lattis\r\n"), 0o644)
	}
	if err := u2.ApplyOnExit(); err != nil {
		t.Fatal(err)
	}
	u3 := relaunch(t, gh, u2)
	s = u3.Status(msg.FR)
	if s.Phase != PhaseInstallFailed || s.Failures != 2 || !s.AutoStopped {
		t.Fatalf("deuxième échec : %+v", s)
	}
	if !strings.Contains(s.Error, "lattis.exe") {
		t.Errorf("raison : %q", s.Error)
	}
	if _, err := os.Stat(filepath.Join(u3.Dir, InstallResultName)); !os.IsNotExist(err) {
		t.Error("trace lue, à ranger")
	}
	// Plus d'installation automatique.
	if err := u3.ApplyOnExit(); err != nil {
		t.Fatal(err)
	}
	if launches != 2 {
		t.Errorf("installeur lancé %d fois, 2 attendues", launches)
	}
	// Une recherche ne fait pas oublier la mise à jour téléchargée.
	if err := u3.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s := u3.Status(msg.FR); s.Phase != PhaseInstallFailed {
		t.Errorf("après une recherche : %+v", s)
	}
	// Installée à la main ensuite : la nouvelle version oublie tout.
	u4 := newTestUpdater(t, gh, "0.3.0-beta", KindUser)
	u4.StateFile, u4.Dir = u3.StateFile, u3.Dir
	u4.Init()
	if s := u4.Status(msg.FR); s.Phase != PhaseIdle || s.Updated != "0.3.0-beta" {
		t.Errorf("après l'installation : %+v", s)
	}
}

func TestInstallManually(t *testing.T) {
	u, gh := readyUpdater(t)
	var auto, manual int
	u.Apply = func(Pending, string, bool) error { auto++; return nil }
	u.Launch = func(p Pending) error { manual++; return nil }
	if err := u.InstallManually(); err != nil {
		t.Fatal(err)
	}
	// Fermeture qui suit : pas d'installation silencieuse en plus.
	if err := u.ApplyOnExit(); err != nil {
		t.Fatal(err)
	}
	if manual != 1 || auto != 0 {
		t.Errorf("manuelle %d, automatique %d", manual, auto)
	}
	a := LoadState(u.StateFile).Pending.Attempts
	if len(a) != 1 || a[0].Outcome != OutcomeManual {
		t.Fatalf("essai : %+v", a)
	}
	// Abandonnée par l'utilisateur : toujours là, en échec.
	s := relaunch(t, gh, u).Status(msg.FR)
	if s.Phase != PhaseInstallFailed || s.Failures != 1 {
		t.Errorf("au lancement suivant : %+v", s)
	}
	// Lancement impossible : dit tout de suite.
	u5, _ := readyUpdater(t)
	u5.Launch = func(Pending) error { return errors.New("introuvable") }
	if err := u5.InstallManually(); err == nil {
		t.Fatal("échec attendu")
	}
	if s := u5.Status(msg.FR); s.Phase != PhaseInstallFailed || !strings.Contains(s.Error, "introuvable") {
		t.Errorf("échec du lancement : %+v", s)
	}
}

func TestParseInstallResult(t *testing.T) {
	r := parseInstallResult(utf16File("version=0.3.3\r\nstatus=failed\r\ncode=2\r\ndir=C:\\Program Files\\Lattis\\Lattis\r\n"))
	if r != (installResult{Version: "0.3.3", Status: "failed", Code: "2", Dir: `C:\Program Files\Lattis\Lattis`}) {
		t.Errorf("UTF-16 : %+v", r)
	}
	if r := parseInstallResult(append([]byte{0xFF, 0xFE}, utf16File("version=0.3.3\r\nstatus=ok\r\n")...)); r.Status != "ok" {
		t.Errorf("UTF-16 avec BOM : %+v", r)
	}
	if r := parseInstallResult([]byte("\xef\xbb\xbfversion=0.3.3\nstatus=ok\n")); r.Status != "ok" || r.Version != "0.3.3" {
		t.Errorf("UTF-8 : %+v", r)
	}
	if !sameRelease("0.3.3", "0.3.3-beta") || sameRelease("0.3.2", "0.3.3-beta") || sameRelease("", "0.3.3") {
		t.Error("sameRelease")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, InstallResultName)
	os.WriteFile(path, []byte("version=0.3.3\nstatus=ok\ndir=D:\\Autre\n"), 0o644)
	now := time.Now()
	if _, ok := readInstallResult(path, "0.3.4-beta", now); ok {
		t.Error("autre version : ignorée")
	}
	if _, ok := readInstallResult(path, "0.3.3-beta", now.Add(time.Hour)); ok {
		t.Error("trace plus ancienne que l'essai : ignorée")
	}
	r, ok := readInstallResult(path, "0.3.3-beta", now)
	if !ok || r.Dir != `D:\Autre` {
		t.Errorf("trace : %+v %v", r, ok)
	}
	// Les raisons, dans la langue demandée.
	for _, c := range []struct {
		a    Attempt
		want string
	}{
		{Attempt{Result: resultOK, InstDir: `D:\Autre`}, `D:\Autre`},
		{Attempt{Result: resultElevation}, "administration"},
		{Attempt{Result: resultFailed, Code: "2"}, "code 2"},
		{Attempt{Result: resultNone}, "aucune trace"},
		{Attempt{}, "toujours l'ancienne"},
		{Attempt{Error: "refusé", Result: resultNone}, "refusé"},
	} {
		if got := attemptReason(msg.FR, &c.a); !strings.Contains(got, c.want) {
			t.Errorf("%+v : %q", c.a, got)
		}
	}
}
