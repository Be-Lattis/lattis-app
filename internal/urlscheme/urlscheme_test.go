package urlscheme

import (
	"errors"
	"slices"
	"testing"
)

const (
	exe       = `C:\Users\l\AppData\Local\Temp\go-build1\b001\exe\lattis.exe`
	installed = `C:\Users\l\AppData\Local\Programs\Lattis\lattis.exe`
	gone      = `C:\Users\l\AppData\Local\Temp\go-build0\b001\exe\lattis.exe`
)

func existsIn(paths ...string) func(string) bool {
	return func(p string) bool { return slices.Contains(paths, p) }
}

func TestDecide(t *testing.T) {
	all := existsIn(exe, installed)
	cases := []struct {
		name   string
		in     Inputs
		action Action
		target string
	}{
		{"absente", Inputs{Exe: exe}, Register, exe},
		{"absente en dev", Inputs{Exe: exe, Dev: true}, Register, exe},
		{"installée l'emporte", Inputs{Command: Command(installed), Exe: exe}, Keep, installed},
		{"dev reprend l'installée", Inputs{Command: Command(installed), Exe: exe, Dev: true}, Register, exe},
		{"exécutable disparu", Inputs{Command: Command(gone), Exe: exe}, Register, exe},
		{"exécutable disparu en dev", Inputs{Command: Command(gone), Exe: exe, Dev: true}, Register, exe},
		{"déjà celui-ci", Inputs{Command: Command(exe), Exe: exe}, Keep, exe},
		{"déjà celui-ci en dev", Inputs{Command: Command(exe), Exe: exe, Dev: true}, Keep, exe},
		{"casse différente", Inputs{Command: `"C:\USERS\L\APPDATA\LOCAL\PROGRAMS\LATTIS\LATTIS.EXE" "%1"`, Exe: installed}, Keep, `C:\USERS\L\APPDATA\LOCAL\PROGRAMS\LATTIS\LATTIS.EXE`},
		{"commande sans %1", Inputs{Command: `"` + exe + `"`, Exe: exe}, Register, exe},
		{"paquet MSIX", Inputs{Exe: exe, Packaged: true}, Skip, ""},
		{"désactivé", Inputs{Exe: exe, Disabled: true, Dev: true}, Skip, ""},
		{"exécutable inconnu", Inputs{}, Skip, ""},
	}
	for _, c := range cases {
		d := Decide(c.in, all)
		if d.Action != c.action || d.Target != c.target {
			t.Errorf("%s : %v %q (%s), attendu %v %q", c.name, d.Action, d.Target, d.Reason, c.action, c.target)
		}
	}
}

func TestCommandExe(t *testing.T) {
	cases := map[string]string{
		``: ``,
		`"C:\Program Files\Lattis\lattis.exe" "%1"`: `C:\Program Files\Lattis\lattis.exe`,
		`C:\Program Files\Lattis\lattis.exe %1`:     `C:\Program Files\Lattis\lattis.exe`,
		`C:\Lattis\lattis.EXE "%1"`:                 `C:\Lattis\lattis.EXE`,
		`"C:\Lattis\lattis.exe`:                     `C:\Lattis\lattis.exe`,
		`lattis %1`:                                 `lattis`,
	}
	for in, want := range cases {
		if got := CommandExe(in); got != want {
			t.Errorf("CommandExe(%q) = %q, attendu %q", in, got, want)
		}
	}
}

type fakeRegistry struct {
	cmd     string
	readErr error
	wrote   []string
	failW   bool
}

func (f *fakeRegistry) Command() (string, error) { return f.cmd, f.readErr }
func (f *fakeRegistry) Register(exe string) error {
	if f.failW {
		return errors.New("accès refusé")
	}
	f.wrote = append(f.wrote, exe)
	f.cmd = Command(exe)
	return nil
}

func TestApply(t *testing.T) {
	all := existsIn(exe, installed)

	r := &fakeRegistry{}
	if d, err := Apply(r, Inputs{Exe: exe}, all); err != nil || d.Action != Register || r.cmd != `"`+exe+`" "%1"` {
		t.Fatalf("clé absente : %+v %v %q", d, err, r.cmd)
	}
	// Second lancement : rien à écrire.
	if d, _ := Apply(r, Inputs{Exe: exe}, all); d.Action != Keep || len(r.wrote) != 1 {
		t.Fatalf("second lancement : %+v, écritures %v", d, r.wrote)
	}

	r = &fakeRegistry{cmd: Command(installed)}
	if d, _ := Apply(r, Inputs{Exe: exe}, all); d.Action != Keep || len(r.wrote) != 0 {
		t.Fatalf("version installée : %+v, écritures %v", d, r.wrote)
	}

	r = &fakeRegistry{readErr: errors.New("illisible")}
	if d, _ := Apply(r, Inputs{Exe: exe}, all); d.Action != Register || len(r.wrote) != 1 {
		t.Fatalf("clé illisible : %+v", d)
	}

	r = &fakeRegistry{readErr: errors.New("ne doit pas être lue")}
	if d, err := Apply(r, Inputs{Exe: exe, Packaged: true}, all); err != nil || d.Action != Skip || len(r.wrote) != 0 {
		t.Fatalf("MSIX : %+v %v", d, err)
	}

	r = &fakeRegistry{failW: true}
	if _, err := Apply(r, Inputs{Exe: exe}, all); err == nil {
		t.Fatal("écriture refusée : erreur attendue")
	}
}

func TestStripFlag(t *testing.T) {
	args, ok := StripFlag([]string{"--demo", "med", "--no-register-protocol", "-v"})
	if !ok || !slices.Equal(args, []string{"--demo", "med", "-v"}) {
		t.Fatalf("%v %v", args, ok)
	}
	if args, ok := StripFlag([]string{"-v"}); ok || len(args) != 1 {
		t.Fatalf("%v %v", args, ok)
	}
}
