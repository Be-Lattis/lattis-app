package numfmt

import "testing"

func TestFromExcel(t *testing.T) {
	for in, want := range map[string]string{
		`#,##0.00 "kg"`:               `# ##0,00 "kg"`,
		`0.00E+00`:                    `0,00E+00`,
		`# ?/?`:                       `# ?/?`,
		`#,##0;[Red]-#,##0`:           `# ##0;[Rouge]-# ##0`,
		`dd/mm/yyyy hh:mm`:            `jj/mm/aaaa hh:mm`,
		`dddd d mmmm yyyy`:            `jjjj j mmmm aaaa`,
		`hh:mm:ss.00`:                 `hh:mm:ss,00`,
		`General" kg"`:                `Standard" kg"`,
		`0.0 %`:                       `0,0 %`,
		`[>=1000.5][Blue]0;[Color3]0`: `[>=1000,5][Bleu]0;[Couleur3]0`,
		`_-* #,##0.00\ "€"_-;\-* #,##0.00\ "€"_-`: `_-* # ##0,00\ "€"_-;\-* # ##0,00\ "€"_-`,
	} {
		got := FromExcel(in)
		if got != want {
			t.Errorf("FromExcel(%q) = %q, attendu %q", in, got, want)
		}
		if back := ToExcel(got); back != in {
			t.Errorf("ToExcel(FromExcel(%q)) = %q", in, back)
		}
	}
}
