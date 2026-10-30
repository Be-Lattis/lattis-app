package transfer

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Compte rendu d'un export .xlsx ou .ods : ce qui n'a pas été repris tel
// quel, feuille par feuille, pour le dire à l'utilisateur (Excel et
// LibreOffice n'ont pas d'équivalent, ou l'export ne l'écrit pas encore).

// ExportReport est le compte rendu d'un export.
type ExportReport struct {
	Sheets []SheetReport `json:"sheets"`
	// Scripts : scripts du classeur, non exportés.
	Scripts int `json:"scripts"`
	// Clipped : cellules au-delà des limites d'Excel, non exportées.
	Clipped int `json:"clipped"`
	// Favorites : requêtes SQL favorites ; Scenarios : scénarios des
	// simulations (Et si…) ; non exportés.
	Favorites int `json:"favorites"`
	Scenarios int `json:"scenarios"`
}

// Lossy dit si le compte rendu signale quelque chose que le fichier ne
// garde pas tel quel (formules en valeurs, scripts…).
func (r ExportReport) Lossy() bool {
	if r.Scripts > 0 || r.Clipped > 0 || r.Favorites > 0 || r.Scenarios > 0 {
		return true
	}
	for _, s := range r.Sheets {
		if s.Kind != "" || s.Protected || s.Formulas+s.Columns+s.Pivots+s.Charts+s.Rules+s.Notes+s.Lists+s.Buttons+s.Queries+s.TwoLevel+s.Clipped > 0 {
			return true
		}
	}
	return false
}

// XLSXLosses dit, sans rien écrire, ce qu'un enregistrement au format .xlsx
// ne garderait pas tel quel (fichier .xlsx ouvert dans Lattis : question
// avant de l'enregistrer dans son format). Les cellules au-delà des limites
// d'Excel des tableaux ne sont comptées qu'à l'écriture.
func XLSXLosses(ctx context.Context, m *canvas.Model) (ExportReport, error) {
	sheets, _, err := exportBook(ctx, m, toXLSX)
	if err != nil {
		return ExportReport{}, err
	}
	return report(ctx, m, sheets), nil
}

// SheetReport est le compte rendu d'une feuille.
type SheetReport struct {
	Name string `json:"name"`
	// Kind : "html" (page) ou "doc" (document) : onglet exporté vide.
	Kind   string `json:"kind,omitempty"`
	Tables int    `json:"tables"` // tableaux exportés
	// Translated : formules exportées comme formules (cellules et colonnes
	// calculées).
	Translated int `json:"translated"`
	// Formulas : formules de cellules exportées par leur valeur ; Columns :
	// colonnes calculées exportées par leurs valeurs ; Functions : les
	// fonctions en cause (noms français).
	Formulas  int      `json:"formulas"`
	Columns   int      `json:"columns"`
	Functions []string `json:"functions,omitempty"`
	Pivots    int      `json:"pivots"`  // tableaux croisés, exportés en valeurs
	Charts    int      `json:"charts"`  // graphiques, non exportés
	Rules     int      `json:"rules"`   // règles de mise en forme conditionnelle, non exportées
	Notes     int      `json:"notes"`   // notes de cellules, non exportées
	Lists     int      `json:"lists"`   // validations (listes de valeurs), non exportées
	Buttons   int      `json:"buttons"` // boutons et contrôles, non exportés
	// Queries : tableaux chargés par une requête, exportés en valeurs (sans
	// la requête).
	Queries int `json:"queries"`
	// TwoLevel : tableaux aux en-têtes sur deux lignes, exportés sur une
	// (« Groupe.Sous »).
	TwoLevel  int  `json:"twoLevel"`
	Protected bool `json:"protected"` // protection de la feuille, non exportée
	Clipped   int  `json:"clipped"`
}

// addFns ajoute des fonctions en cause, sans doublon, dans l'ordre alphabétique.
func (r *SheetReport) addFns(fns []string) {
	for _, f := range fns {
		if i, found := slices.BinarySearch(r.Functions, f); !found {
			r.Functions = slices.Insert(r.Functions, i, f)
		}
	}
}

// formula compte une formule (cellule ou colonne calculée) selon sa traduction.
func (r *SheetReport) formula(ok, column bool, why []string) {
	switch {
	case ok:
		r.Translated++
	case column:
		r.Columns++
	default:
		r.Formulas++
	}
	r.addFns(why)
}

// style compte les propriétés d'une couche de style non exportées : note,
// validation, règles conditionnelles (identifiants dans rules).
func (r *SheetReport) style(st canvas.Style, rules map[string]bool) {
	if s, _ := st["nt"].(string); s != "" {
		r.Notes++
	}
	if st["dv"] != nil {
		r.Lists++
	}
	if cf, ok := st["cf"].([]any); ok {
		for _, x := range cf {
			id := ""
			if m, ok := x.(map[string]any); ok {
				id, _ = m["id"].(string)
			}
			if id == "" {
				b, _ := json.Marshal(x)
				id = string(b)
			}
			rules[id] = true
		}
	}
}

// ruleList ajoute les règles d'une liste JSON (colonne, ligne ou colonne
// de la feuille) : une règle posée à plusieurs endroits compte une fois.
func ruleList(js string, rules map[string]bool) {
	var rs []canvas.Rule
	if json.Unmarshal([]byte(js), &rs) != nil {
		return
	}
	for _, x := range rs {
		id := x.ID
		if id == "" {
			b, _ := json.Marshal(x)
			id = string(b)
		}
		rules[id] = true
	}
}

// report assemble le compte rendu après l'écriture (cellules coupées).
func report(ctx context.Context, m *canvas.Model, sheets []xsheet) ExportReport {
	rep := ExportReport{Sheets: make([]SheetReport, len(sheets))}
	for i, sh := range sheets {
		r := sh.rep
		r.Clipped = sh.clipped
		rep.Clipped += sh.clipped
		rep.Sheets[i] = r
	}
	if ss, err := m.Scripts(ctx); err == nil {
		rep.Scripts = len(ss)
	}
	if fs, err := m.Favorites(ctx); err == nil {
		rep.Favorites = len(fs)
	}
	for _, sh := range sheets {
		if sc, err := m.Scenarios(ctx, sh.info.ID); err == nil {
			rep.Scenarios += len(sc)
		}
	}
	return rep
}
