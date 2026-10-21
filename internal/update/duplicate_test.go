package update

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/msg"
)

const (
	userDir    = `C:\Users\essai-lattis\AppData\Local\Programs\Lattis`
	machineDir = `C:\Program Files\Lattis\Lattis`
)

func TestEntryFor(t *testing.T) {
	e := entryFor("", `"C:\Program Files\Lattis\Lattis\uninstall.exe"`, " 0.3.3-beta ")
	if e.Dir != machineDir || e.Version != "0.3.3-beta" || e.Uninstaller != machineDir+`\uninstall.exe` {
		t.Errorf("sans InstallLocation : %+v", e)
	}
	e = entryFor(userDir+`\`, "", "0.3.5-beta")
	if e.Dir != userDir+`\` || e.Uninstaller != userDir+`\uninstall.exe` {
		t.Errorf("sans UninstallString : %+v", e)
	}
	if e := entryFor("", "", "0.3.5-beta"); e != (Entry{}) {
		t.Errorf("clé vide : %+v", e)
	}
	if got := uninstallerPath(`C:\Lattis\uninstall.exe /S`); got != `C:\Lattis\uninstall.exe` {
		t.Errorf("sans guillemets : %q", got)
	}
}

func TestFindDuplicate(t *testing.T) {
	user := Entry{Dir: userDir, Version: "0.3.4-beta", Uninstaller: userDir + `\uninstall.exe`}
	machine := Entry{Dir: machineDir, Version: "0.3.3-beta", Uninstaller: machineDir + `\uninstall.exe`}
	cases := []struct {
		name          string
		exe           string
		user, machine Entry
		want          Kind // "" : pas de doublon
	}{
		{"une seule (utilisateur)", userDir + `\lattis.exe`, user, Entry{}, ""},
		{"une seule (tous)", machineDir + `\lattis.exe`, Entry{}, machine, ""},
		{"même dossier inscrit deux fois", userDir + `\lattis.exe`, user, Entry{Dir: userDir + `\`}, ""},
		{"lancée de l'utilisateur", userDir + `\lattis.exe`, user, machine, KindMachine},
		{"casse et barre finale", `c:\users\essai-lattis\appdata\local\programs\lattis\lattis.exe`, Entry{Dir: userDir + `\`, Version: "0.3.4-beta"}, machine, KindMachine},
		{"lancée de tous", machineDir + `\lattis.exe`, user, machine, KindUser},
		{"portable : la plus ancienne (tous)", `D:\usb\lattis.exe`, user, machine, KindMachine},
		{"portable : la plus ancienne (utilisateur)", `D:\usb\lattis.exe`, Entry{Dir: userDir, Version: "0.3.0-beta"}, machine, KindUser},
		{"portable : versions inconnues, celle pour tous", `D:\usb\lattis.exe`, Entry{Dir: userDir}, Entry{Dir: machineDir}, KindMachine},
	}
	for _, c := range cases {
		d := findDuplicate(c.exe, c.user, c.machine)
		if c.want == "" {
			if d != nil {
				t.Errorf("%s : doublon inattendu %+v", c.name, d)
			}
			continue
		}
		if d == nil || d.Scope != c.want {
			t.Errorf("%s : %+v, attendu %s", c.name, d, c.want)
			continue
		}
		if d.Elevation != (c.want == KindMachine) {
			t.Errorf("%s : élévation %v", c.name, d.Elevation)
		}
	}
	// L'autre : son dossier, sa version, son désinstalleur.
	d := findDuplicate(userDir+`\lattis.exe`, user, machine)
	if d.Dir != machineDir || d.Version != "0.3.3-beta" || d.uninstaller != machineDir+`\uninstall.exe` {
		t.Errorf("autre installation : %+v", d)
	}
}

func TestRemovalFor(t *testing.T) {
	present := func(string) bool { return true }
	absent := func(string) bool { return false }
	sys := `C:\Windows\System32`
	machine := Duplicate{Scope: KindMachine, Dir: machineDir, Version: "0.3.3-beta", uninstaller: machineDir + `\uninstall.exe`}
	if r := removalFor(machine, present, sys); r != (removal{File: machineDir + `\uninstall.exe`, Args: "/S /KEEPDATA", Elevate: true}) {
		t.Errorf("pour tous : %+v", r)
	}
	r := removalFor(machine, absent, sys+`\`)
	want := removal{File: sys + `\reg.exe`, Args: `delete "HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\LattisLattis" /f /reg:64`, Elevate: true}
	if r != want {
		t.Errorf("pour tous, clé orpheline : %+v", r)
	}
	user := Duplicate{Scope: KindUser, Dir: userDir, Version: "0.3.6-beta", uninstaller: userDir + `\uninstall.exe`}
	if r := removalFor(user, present, sys); r != (removal{File: userDir + `\uninstall.exe`, Args: "/S /KEEPDATA"}) {
		t.Errorf("pour l'utilisateur : %+v", r)
	}
	for _, v := range []string{"0.3.5-beta", "0.3.0-beta", ""} {
		old := user
		old.Version = v
		if r := removalFor(old, present, sys); r != (removal{OwnUser: true}) {
			t.Errorf("pour l'utilisateur, désinstalleur %q sans /KEEPDATA : %+v", v, r)
		}
	}
	if r := removalFor(user, absent, sys); r != (removal{DeleteUserKey: true}) {
		t.Errorf("pour l'utilisateur, clé orpheline : %+v", r)
	}
}

func TestOwnUserFiles(t *testing.T) {
	got := ownUserFiles(userDir + `\`)
	if len(got) != 4 || got[0] != userDir+`\lattis.exe` || got[3] != userDir+`\uninstall.exe` {
		t.Errorf("fichiers : %v", got)
	}
}

func TestLnkMentions(t *testing.T) {
	utf16 := func(s string) []byte {
		var b []byte
		for _, r := range s {
			b = append(b, byte(r), byte(r>>8))
		}
		return b
	}
	ansi := append([]byte("L\x00\x00\x00junk"), []byte(`C:\Users\essai-lattis\AppData\Local\Programs\Lattis\lattis.exe`+"\x00")...)
	if !lnkMentions(ansi, userDir) {
		t.Error("cible ANSI non vue")
	}
	wide := append([]byte{0x4c, 0, 0, 0}, utf16(`C:\USERS\essai-lattis\AppData\Local\Programs\Lattis\lattis.exe`)...)
	if !lnkMentions(wide, userDir+`\`) {
		t.Error("cible UTF-16 non vue")
	}
	if lnkMentions(ansi, machineDir) || lnkMentions(ansi, "") {
		t.Error("autre dossier vu")
	}
	// Un dossier dont le nom prolonge celui-ci n'est pas celui-ci.
	if lnkMentions([]byte(`C:\Users\essai-lattis\AppData\Local\Programs\Lattis2\lattis.exe`), userDir) {
		t.Error("préfixe pris pour le dossier")
	}
}

// fakeInstalls : un faux Registre, modifiable par le faux retrait.
type fakeInstalls struct {
	mu            sync.Mutex
	user, machine Entry
}

func (f *fakeInstalls) read() (Entry, Entry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.user, f.machine
}

func (f *fakeInstalls) drop(scope Kind) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if scope == KindUser {
		f.user = Entry{}
	} else {
		f.machine = Entry{}
	}
}

func dupUpdater(reg *fakeInstalls, remove func(Duplicate) error) (*Updater, chan Status) {
	ch := make(chan Status, 32)
	u := &Updater{
		Current:  "0.3.6-beta",
		Kind:     KindUser,
		Client:   &Client{},
		Exe:      userDir + `\lattis.exe`,
		Installs: reg.read,
		Remove:   remove,
		OnChange: func(s Status) { ch <- s },
	}
	return u, ch
}

func waitStatus(t *testing.T, ch chan Status, ok func(Status) bool) Status {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case s := <-ch:
			if ok(s) {
				return s
			}
		case <-timeout:
			t.Fatal("état attendu jamais vu")
		}
	}
}

func shortPoll(t *testing.T, timeout time.Duration) {
	poll, to := dupPoll, dupTimeout
	dupPoll, dupTimeout = 5*time.Millisecond, timeout
	t.Cleanup(func() { dupPoll, dupTimeout = poll, to })
}

// Deux installations : l'avis au lancement, puis le retrait de l'autre
// (celle pour tous), suivi jusqu'à sa disparition du Registre.
func TestRemoveDuplicate(t *testing.T) {
	shortPoll(t, 5*time.Second)
	reg := &fakeInstalls{
		user:    Entry{Dir: userDir, Version: "0.3.6-beta", Uninstaller: userDir + `\uninstall.exe`},
		machine: Entry{Dir: machineDir, Version: "0.3.3-beta", Uninstaller: machineDir + `\uninstall.exe`},
	}
	var got []Duplicate
	u, ch := dupUpdater(reg, func(d Duplicate) error {
		got = append(got, d)
		go func() {
			time.Sleep(20 * time.Millisecond)
			reg.drop(d.Scope)
		}()
		return nil
	})
	after := make(chan struct{}, 1)
	u.AfterRemove = func() { after <- struct{}{} }
	u.Init()
	s := u.Status(msg.FR)
	if s.Duplicate == nil || s.Duplicate.Scope != KindMachine || s.Duplicate.Dir != machineDir || s.Duplicate.Version != "0.3.3-beta" || !s.Duplicate.Elevation {
		t.Fatalf("avis : %+v", s.Duplicate)
	}
	if err := u.RemoveDuplicate(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].uninstaller != machineDir+`\uninstall.exe` {
		t.Errorf("retrait lancé : %+v", got)
	}
	waitStatus(t, ch, func(s Status) bool { return s.Duplicate != nil && s.Duplicate.Removing })
	waitStatus(t, ch, func(s Status) bool { return s.Duplicate == nil })
	select {
	case <-after:
	case <-time.After(time.Second):
		t.Error("AfterRemove non appelé")
	}
	// Plus rien à retirer : sans effet.
	if err := u.RemoveDuplicate(); err != nil || len(got) != 1 {
		t.Errorf("second retrait : %v, %d", err, len(got))
	}
}

// Invite UAC refusée : l'avis reste, avec la raison, et peut être relancé.
func TestRemoveDuplicateRefused(t *testing.T) {
	reg := &fakeInstalls{user: Entry{Dir: userDir}, machine: Entry{Dir: machineDir}}
	refused := msg.New("update.duplicate.refused", nil)
	u, _ := dupUpdater(reg, func(Duplicate) error { return refused })
	u.Init()
	if err := u.RemoveDuplicate(); !errors.Is(err, refused) {
		t.Errorf("erreur : %v", err)
	}
	s := u.Status(msg.FR)
	if s.Duplicate == nil || s.Duplicate.Removing || s.Duplicate.Error != msg.Format(msg.FR, "update.duplicate.refused", nil) {
		t.Errorf("après refus : %+v", s.Duplicate)
	}
	if en := u.Status(msg.EN); en.Duplicate.Error != msg.Format(msg.EN, "update.duplicate.refused", nil) {
		t.Errorf("en anglais : %q", en.Duplicate.Error)
	}
}

// Le désinstalleur n'aboutit pas : après l'attente, l'avis le dit.
func TestRemoveDuplicateTimeout(t *testing.T) {
	shortPoll(t, 30*time.Millisecond)
	reg := &fakeInstalls{user: Entry{Dir: userDir}, machine: Entry{Dir: machineDir}}
	u, ch := dupUpdater(reg, func(Duplicate) error { return nil })
	u.Init()
	if err := u.RemoveDuplicate(); err != nil {
		t.Fatal(err)
	}
	s := waitStatus(t, ch, func(s Status) bool { return s.Duplicate != nil && s.Duplicate.Error != "" })
	if s.Duplicate.Removing || s.Duplicate.Error != msg.Format(msg.Current(), "update.duplicate.notRemoved", nil) {
		t.Errorf("après l'attente : %+v", s.Duplicate)
	}
}

// Version de développement, une seule installation, ou pas de Registre :
// pas d'avis.
func TestNoDuplicate(t *testing.T) {
	reg := &fakeInstalls{user: Entry{Dir: userDir}, machine: Entry{Dir: machineDir}}
	u, _ := dupUpdater(reg, nil)
	u.Current = "dev"
	u.Init()
	if s := u.Status(msg.FR); s.Duplicate != nil {
		t.Errorf("version dev : %+v", s.Duplicate)
	}
	u, _ = dupUpdater(&fakeInstalls{user: Entry{Dir: userDir}}, nil)
	u.Init()
	if s := u.Status(msg.FR); s.Duplicate != nil {
		t.Errorf("une seule : %+v", s.Duplicate)
	}
	u = &Updater{Current: "0.3.6-beta", Client: &Client{}}
	u.Init()
	if s := u.Status(msg.FR); s.Duplicate != nil {
		t.Errorf("sans Registre : %+v", s.Duplicate)
	}
}
