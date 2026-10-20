package msg

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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
	setEN(t, "table.nameTaken", "table: “{name}” already exists")
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

// typed : une erreur typée qui porte son message (Messager).
type typed struct{ name string }

func (e typed) Error() string { return e.Msg().String() }
func (e typed) Msg() Message  { return M("table.nameTaken", Args{"name": e.name}) }

// Paramètres qui sont eux-mêmes des messages ou des erreurs : écrits dans
// la langue demandée ; erreurs typées traduites comme un *Error.
func TestMessageArgs(t *testing.T) {
	setEN(t, "table.nameTaken", "table: “{name}” already exists")
	catalogs[EN]["test.wrap"] = "import failed: {err} ({what})"
	fr["test.wrap"] = "import impossible : {err} ({what})"
	fr["test.what"] = "le tableau"
	catalogs[EN]["test.what"] = "the table"
	t.Cleanup(func() {
		delete(catalogs[EN], "test.wrap")
		delete(fr, "test.wrap")
		delete(fr, "test.what")
		delete(catalogs[EN], "test.what")
	})
	inner := typed{"Ventes"}
	e := Wrap(inner, "test.wrap", Args{"err": inner, "what": M("test.what", nil)})
	if e.Error() != "import impossible : tableau : « Ventes » existe déjà (le tableau)" {
		t.Fatalf("Error() = %q", e.Error())
	}
	if got := e.Text(EN); got != "import failed: table: “Ventes” already exists (the table)" {
		t.Errorf("Text = %q", got)
	}
	if got := Text(EN, fmt.Errorf("lecture : %w", inner)); got != "lecture : table: “Ventes” already exists" {
		t.Errorf("Text d'une erreur typée enveloppée = %q", got)
	}
	in := e.Args.In(EN)
	if in["err"] != "table: “Ventes” already exists" || in["what"] != "the table" {
		t.Errorf("Args.In = %v", in)
	}
	if M("test.what", nil).String() != "le tableau" {
		t.Error("Message.String : le français")
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

// Chaque identifiant écrit en toutes lettres dans le moteur
// (msg.New("table.nameTaken", …), msg.Wrap(…, "…"), msg.M, say, Format)
// a son texte français.
func TestIDsInCatalogue(t *testing.T) {
	call := regexp.MustCompile(`(?:msg\.(?:New|M)\(|msg\.Wrap\([^"\n]*?,\s*|msg\.Format\([^"\n]*?,\s*|\bsay\()"([A-Za-z0-9_.]+)"\s*[,)]`)
	root := filepath.Join("..", "..")
	n := 0
	for _, dir := range []string{"internal", "engine", "cmd"} {
		filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			for _, m := range call.FindAllStringSubmatch(string(b), -1) {
				n++
				if !Has(ID(m[1])) {
					t.Errorf("%s : %s absent du catalogue français", p, m[1])
				}
			}
			return nil
		})
	}
	if n < 100 {
		t.Fatalf("%d identifiants trouvés seulement", n)
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

// setEN remplace le temps du test le texte anglais de id (puis le rend).
func setEN(t *testing.T, id ID, s string) {
	old, had := catalogs[EN][id]
	catalogs[EN][id] = s
	t.Cleanup(func() {
		if had {
			catalogs[EN][id] = old
		} else {
			delete(catalogs[EN], id)
		}
	})
}

var paramName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// params : les paramètres de s, tels que render les lit ({nom}, et
// « nom (plural) » pour {nom, plural, …}, avec ceux de ses branches) ; les
// accolades qui ne nomment pas un paramètre ({ nom = valeur }, JSON) sont
// du texte. Faux si une accolade n'est pas fermée ou qu'un pluriel n'a pas
// de branche other.
func params(s string, out map[string]bool) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '}' {
			return false
		}
		if s[i] != '{' {
			continue
		}
		j := closing(s, i)
		if j < 0 {
			return false
		}
		name, rest, comma := strings.Cut(s[i+1:j], ",")
		name = strings.TrimSpace(name)
		if paramName.MatchString(name) {
			kind, branches, _ := strings.Cut(rest, ",")
			switch {
			case !comma:
				out[name] = true
			case strings.TrimSpace(kind) == "plural":
				out[name+" (plural)"] = true
				cases := parseBranches(branches)
				if _, ok := cases["other"]; !ok {
					return false
				}
				for _, text := range cases {
					if !params(text, out) {
						return false
					}
				}
			}
		}
		i = j
	}
	return true
}

// Chaque catalogue traduit (non vide) a exactement les identifiants du
// français, et pour chacun les mêmes paramètres et pluriels.
func TestCatalogueParity(t *testing.T) {
	if len(en) == 0 {
		t.Fatal("catalogue anglais vide")
	}
	for _, l := range Langs {
		cat := catalogs[l]
		if l == FR || len(cat) == 0 {
			continue
		}
		for id := range fr {
			if _, ok := cat[id]; !ok {
				t.Errorf("%s : %s manque", l, id)
			}
		}
		for id, s := range cat {
			ref, ok := fr[id]
			if !ok {
				t.Errorf("%s : %s absent du catalogue français", l, id)
				continue
			}
			if strings.TrimSpace(s) == "" && strings.TrimSpace(ref) != "" {
				t.Errorf("%s : %s vide", l, id)
			}
			want, got := map[string]bool{}, map[string]bool{}
			if !params(ref, want) {
				t.Errorf("fr : %s mal formé", id)
			}
			if !params(s, got) {
				t.Errorf("%s : %s mal formé (accolades, branche other)", l, id)
			}
			if fmt.Sprint(keys(want)) != fmt.Sprint(keys(got)) {
				t.Errorf("%s : %s : paramètres %v, attendu %v", l, id, keys(got), keys(want))
			}
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
