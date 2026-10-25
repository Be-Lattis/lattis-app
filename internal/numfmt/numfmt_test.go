package numfmt

import (
	"encoding/json"
	"os"
	"testing"
)

// testCase est un cas de testdata/cases.json, partagé avec l'interface
// (frontend/src/lib/grid/numcode.test.ts) : les deux moteurs rendent le
// même texte.
type testCase struct {
	Code      string   `json:"code"`
	Value     *float64 `json:"value"`
	TextValue *string  `json:"text_value"`
	Lang      string   `json:"lang"`
	Text      string   `json:"text"`
	Color     string   `json:"color"`
	OK        *bool    `json:"ok"`
}

func TestCases(t *testing.T) {
	raw, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []testCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		f := Parse(c.Code)
		loc := ForLanguage(c.Lang)
		var r Result
		if c.TextValue != nil {
			r = f.Text(*c.TextValue)
		} else {
			r = f.Number(*c.Value, loc)
		}
		if c.OK != nil && !*c.OK {
			if r.OK {
				t.Errorf("%q : %v accepté (%q)", c.Code, *c.Value, r.Text)
			}
			continue
		}
		if r.Text != c.Text || r.Color != c.Color || !r.OK {
			t.Errorf("%q (%s) : %q %q, attendu %q %q", c.Code, c.Lang, r.Text, r.Color, c.Text, c.Color)
		}
	}
}

func TestPlain(t *testing.T) {
	if got := Plain(" " + Fill + " 1 234,50 € "); got != "  1 234,50 € " {
		t.Errorf("Plain : %q", got)
	}
	if got := Plain("a" + Fill + "-b"); got != "a-b" {
		t.Errorf("Plain : %q", got)
	}
}

func TestToExcel(t *testing.T) {
	for in, want := range map[string]string{
		`# ##0,00 "kg"`:                 `#,##0.00 "kg"`,
		`#,##0.00`:                      `#,##0.00`,
		`0,00E+00`:                      `0.00E+00`,
		`# ?/?`:                         `# ?/?`,
		`# ##0;[Rouge]-# ##0`:           `#,##0;[Red]-#,##0`,
		`jj/mm/aaaa hh:mm`:              `dd/mm/yyyy hh:mm`,
		`jjjj j mmmm aaaa`:              `dddd d mmmm yyyy`,
		`hh:mm:ss,00`:                   `hh:mm:ss.00`,
		`Standard" kg"`:                 `General" kg"`,
		`0,0 %`:                         `0.0 %`,
		`[>=1000,5][Bleu]0;[Couleur3]0`: `[>=1000.5][Blue]0;[Color3]0`,
	} {
		if got := ToExcel(in); got != want {
			t.Errorf("ToExcel(%q) = %q, attendu %q", in, got, want)
		}
	}
}

func TestKinds(t *testing.T) {
	if !Parse("jj/mm/aaaa").IsDate() || !Parse("[h]:mm").IsDate() || Parse("0,00").IsDate() || Parse("Standard").IsDate() {
		t.Error("IsDate")
	}
	if !Parse("@").IsText() || Parse("0;@").IsText() {
		t.Error("IsText")
	}
}
