package api

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/msg"
)

var update = flag.Bool("update", false, "réécrire la référence générée")

const refFile = "../../frontend/src/lib/script/api.gen.ts"

func TestReferenceUpToDate(t *testing.T) {
	want := LuaReference()
	if *update {
		if err := os.WriteFile(refFile, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(refFile)
	if err != nil {
		t.Fatal(err)
	}
	// Extraction Windows (autocrlf) : les fins de ligne ne comptent pas.
	if strings.ReplaceAll(string(got), "\r\n", "\n") != want {
		t.Fatal("api.gen.ts n'est plus à jour : go test ./internal/api -run TestReference -update")
	}
	for _, l := range msg.Langs {
		if l == msg.FR {
			continue
		}
		file := "../../frontend/src/lib/script/ref/" + string(l) + ".api.json"
		want := LuaReferenceText(l)
		if *update {
			if err := os.WriteFile(file, []byte(want), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.ReplaceAll(string(got), "\r\n", "\n") != want {
			t.Fatalf("%s n'est plus à jour : go test ./internal/api -run TestReference -update", file)
		}
	}
}

// TestReferenceTranslated : chaque commande des scripts a ses textes dans
// chaque langue (résumé, précisions et forme du résultat quand le français
// en a), chaque description de paramètre sa traduction ; pas de texte pour
// une commande inconnue.
func TestReferenceTranslated(t *testing.T) {
	for _, l := range msg.Langs {
		if l == msg.FR {
			continue
		}
		d := apiDocs[l]
		if d == nil {
			t.Errorf("%s : pas de textes des commandes", l)
			continue
		}
		p := d.phrases
		if p.params == "" || p.colon == "" || p.sep == "" || p.undo == "" || p.stop == "" || strings.Count(p.named, "%s") != 2 {
			t.Errorf("%s : morceaux de phrase incomplets (%+v)", l, p)
		}
		used := map[string]bool{}
		for _, c := range Commands() {
			if c.NoLua {
				continue
			}
			used[c.Name] = true
			x, ok := d.cmds[c.Name]
			switch {
			case !ok:
				t.Errorf("%s : %s sans texte", l, c.Name)
				continue
			case x.S == "" || !strings.HasSuffix(x.S, ".") && !strings.HasSuffix(x.S, "。"):
				t.Errorf("%s %s : résumé manquant ou sans point final (%q)", l, c.Name, x.S)
			case (x.D == "") != (c.Details == ""):
				t.Errorf("%s %s : précisions présentes d'un côté seulement", l, c.Name)
			case (x.R == "") != (c.Returns == ""):
				t.Errorf("%s %s : forme du résultat présente d'un côté seulement", l, c.Name)
			}
			for _, pa := range c.Params {
				if d.params[pa.Desc] == "" {
					t.Errorf("%s %s : paramètre %s non traduit (%q)", l, c.Name, pa.Name, pa.Desc)
				}
			}
		}
		for name := range d.cmds {
			if !used[name] {
				t.Errorf("%s : texte de %s, commande inconnue ou absente des scripts", l, name)
			}
		}
	}
}

func TestCommandsDocumented(t *testing.T) {
	for _, c := range Commands() {
		if c.Summary == "" || !strings.HasSuffix(c.Summary, ".") {
			t.Errorf("%s : résumé manquant ou sans point final", c.Name)
		}
		if c.Write && c.Label == "" {
			t.Errorf("%s : modifie le classeur sans nom d'étape d'annulation", c.Name)
		}
		for _, p := range c.Params {
			if p.Desc == "" || p.Type == "" {
				t.Errorf("%s : paramètre %s non décrit", c.Name, p.Name)
			}
		}
	}
}
