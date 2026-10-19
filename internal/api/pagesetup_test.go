package api

import (
	"context"
	"strings"
	"testing"
)

func TestPageSetupCommands(t *testing.T) {
	h := newHost(t)
	v := call(t, h, "sheets.page_setup", nil).(PageSetupView)
	if v.Paper != "A4" || v.Orientation != "portrait" || v.Scale != 100 || v.Order != "down" || v.Margins.Left < 1.7 {
		t.Fatalf("par défaut : %+v", v)
	}
	run(t, h, `sheets.set_page_setup({orientation = "landscape", paper = "a3", margins = "narrow", fit_wide = 1,
  print_area = "$A$1:$F$40", title_rows = "1:2", title_cols = "A", row_breaks = {21}, order = "across",
  footer = {center = "Page &[Page] sur &[Pages]"}})`)
	got := asJSON(call(t, h, "sheets.page_setup", map[string]any{"sheet": "Feuille1"}))
	for _, want := range []string{`"paper":"A3"`, `"orientation":"landscape"`, `"left":0.635`, `"fit_wide":1`, `"print_area":"A1:F40"`, `"title_rows":"1:2"`, `"title_cols":"A:A"`, `"row_breaks":[21]`, `"order":"across"`} {
		if !strings.Contains(got, want) {
			t.Errorf("%s attendu dans %s", want, got)
		}
	}
	if v = call(t, h, "sheets.page_setup", nil).(PageSetupView); v.Footer.Center != "Page &[Page] sur &[Pages]" {
		t.Errorf("pied de page : %+v", v.Footer)
	}
	if st := call(t, h, "doc.state", nil).(DocState); st.Undo != "Script « Test »" {
		t.Fatalf("étape %q", st.Undo)
	}
	// Champs absents : inchangés ; "" efface ; reset repart de zéro.
	call(t, h, "sheets.set_page_setup", map[string]any{"setup": map[string]any{"print_area": "", "margins": map[string]any{"top": 3.0}}})
	v = call(t, h, "sheets.page_setup", nil).(PageSetupView)
	if v.PrintArea != "" || v.TitleRows != "1:2" || v.Margins.Top != 3 || v.Margins.Left != 0.635 {
		t.Fatalf("après changement partiel : %+v", v)
	}
	call(t, h, "sheets.set_page_setup", map[string]any{"setup": map[string]any{"gridlines": true}, "reset": true})
	if v = call(t, h, "sheets.page_setup", nil).(PageSetupView); v.Paper != "A4" || !v.Gridlines || v.TitleRows != "" {
		t.Fatalf("après reset : %+v", v)
	}
	for _, bad := range []map[string]any{{"orientation": "biais"}, {"paper": "A9"}, {"print_area": "zz"}, {"inconnu": 1}, {"scale": 2}} {
		if _, err := Invoke(context.Background(), h, "sheets.set_page_setup", map[string]any{"setup": bad}); err == nil {
			t.Errorf("%v accepté", bad)
		}
	}
}
