package msg

import (
	"errors"
	"fmt"
	"testing"
)

func TestParseResolve(t *testing.T) {
	for tag, want := range map[string]Lang{"fr": FR, "fr-FR": FR, "en-US": EN, "de_DE.UTF-8": DE, "es-419": ES, "zh-Hans-CN": ZH, "ZH_cn": ZH} {
		if l, ok := Parse(tag); !ok || l != want {
			t.Errorf("Parse(%q) = %q, %v", tag, l, ok)
		}
	}
	for _, tag := range []string{"", "auto", "it-IT", "C"} {
		if _, ok := Parse(tag); ok {
			t.Errorf("Parse(%q) accepté", tag)
		}
	}
	for _, c := range []struct {
		setting, system string
		want            Lang
	}{
		{"de", "fr-FR", DE},
		{"auto", "es-ES", ES},
		{"", "zh-CN", ZH},
		{"auto", "it-IT", Fallback},
		{"auto", "", Fallback},
	} {
		if l := Resolve(c.setting, c.system); l != c.want {
			t.Errorf("Resolve(%q, %q) = %q, attendu %q", c.setting, c.system, l, c.want)
		}
	}
}

func TestFormat(t *testing.T) {
	fr["test.plural"] = "{n, plural, =0 {aucune ligne} one {# ligne} other {# lignes}} dans « {table} »"
	catalogs[EN]["test.plural"] = "{n, plural, =0 {no rows} one {# row} other {# rows}} in “{table}”"
	catalogs[ZH]["test.plural"] = "{n, plural, other {# 行}}"
	t.Cleanup(func() {
		delete(fr, "test.plural")
		delete(catalogs[EN], "test.plural")
		delete(catalogs[ZH], "test.plural")
	})
	for _, c := range []struct {
		l    Lang
		n    any
		want string
	}{
		{FR, 0, "aucune ligne dans « T »"},
		{FR, 1, "1 ligne dans « T »"},
		{FR, 1.5, "1,5 ligne dans « T »"},
		{FR, int64(12), "12 lignes dans « T »"},
		{EN, 0, "no rows in “T”"},
		{EN, 1, "1 row in “T”"},
		{EN, 2, "2 rows in “T”"},
		{ZH, 1, "1 行"},
		// Pas de texte allemand : le français.
		{DE, 3, "3 lignes dans « T »"},
	} {
		if got := Format(c.l, "test.plural", Args{"n": c.n, "table": "T"}); got != c.want {
			t.Errorf("%s %v : %q, attendu %q", c.l, c.n, got, c.want)
		}
	}
	// Paramètre absent : laissé tel quel ; identifiant inconnu : lui-même.
	if got := Format(FR, "table.nameTaken", nil); got != "tableau : « {name} » existe déjà" {
		t.Errorf("sans paramètre : %q", got)
	}
	if got := Format(EN, "inconnu.cle", nil); got != "inconnu.cle" {
		t.Errorf("inconnu : %q", got)
	}
}

func TestError(t *testing.T) {
	kind := errors.New("tableau")
	e := Wrap(kind, "table.nameTaken", Args{"name": "Ventes"})
	if e.Error() != "tableau : « Ventes » existe déjà" {
		t.Fatalf("Error() = %q", e.Error())
	}
	if !errors.Is(e, kind) || !errors.Is(e, New("table.nameTaken", nil)) || errors.Is(e, New("sheet.nameTaken", nil)) {
		t.Fatal("errors.Is")
	}
	catalogs[EN]["table.nameTaken"] = "table: “{name}” already exists"
	t.Cleanup(func() { delete(catalogs[EN], "table.nameTaken") })
	if got := Text(EN, e); got != "table: “Ventes” already exists" {
		t.Errorf("Text = %q", got)
	}
	// Enveloppée : seul le texte du message change.
	w := fmt.Errorf("import : %w", e)
	if got := Text(EN, w); got != "import : table: “Ventes” already exists" {
		t.Errorf("Text enveloppée = %q", got)
	}
	if got := Text(EN, errors.New("brut")); got != "brut" {
		t.Errorf("erreur ordinaire = %q", got)
	}
	if Text(FR, w) != w.Error() {
		t.Error("français : le texte d'origine")
	}
}

func TestCurrent(t *testing.T) {
	defer SetCurrent(Current())
	SetCurrent("xx")
	SetCurrent(ES)
	if Current() != ES {
		t.Fatalf("Current = %q", Current())
	}
}

// Les textes du catalogue français ont des accolades équilibrées.
func TestCatalogue(t *testing.T) {
	for id, s := range fr {
		depth := 0
		for _, c := range s {
			switch c {
			case '{':
				depth++
			case '}':
				depth--
			}
			if depth < 0 {
				break
			}
		}
		if depth != 0 {
			t.Errorf("%s : accolades déséquilibrées", id)
		}
	}
}
