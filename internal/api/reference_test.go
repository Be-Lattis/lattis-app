package api

import (
	"flag"
	"os"
	"strings"
	"testing"
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
