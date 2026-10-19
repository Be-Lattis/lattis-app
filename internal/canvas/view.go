package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
)

// SheetInfo décrit un onglet.
type SheetInfo struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	NoGrid bool   `json:"noGrid"` // quadrillage masqué
	// Headings : lettres de colonnes et numéros de lignes toujours affichés
	// (sinon seulement pendant la saisie d'une formule).
	Headings bool `json:"headings"`
	// Protected : feuille protégée (Révision › Protéger la feuille) ;
	// Password : il faut un mot de passe pour ôter la protection.
	Protected bool `json:"protected"`
	Password  bool `json:"password"`
	// Color : couleur de l'onglet (« #rrggbb », vide : aucune) ; Hidden :
	// feuille masquée (absente de la barre d'onglets).
	Color  string `json:"color"`
	Hidden bool   `json:"hidden"`
	// FreezeRows, FreezeCols : volets figés (lignes et colonnes qui restent
	// affichées pendant le défilement), 0 sans volet figé.
	FreezeRows int `json:"freezeRows"`
	FreezeCols int `json:"freezeCols"`
	// Zoom : zoom de la feuille en pour cent (100 par défaut).
	Zoom int `json:"zoom"`
	// Kind : "html" pour un onglet page HTML, "doc" pour un onglet Document (voir pages.go), vide pour une
	// feuille.
	Kind string `json:"kind"`
	// Choice : onglet à choix (« Ventes · 2025 ▾ », choice.go), absent sinon.
	Choice *SheetChoice `json:"choice,omitempty"`
}

// ColumnInfo décrit une colonne de tableau pour l'interface.
type ColumnInfo struct {
	ID     int64     `json:"id"`
	Errors int       `json:"errors"` // saisies hors type
	Name   string    `json:"name"`
	SQL    string    `json:"sql"`
	Type   grid.Type `json:"type"`
	Fixed  bool      `json:"fixed"`
	// Colonne calculée : formule telle que l'utilisateur la lit (français).
	Formula string `json:"formula"`
	// Colonne calculée : type numérique deviné du résultat (« decimal » :
	// affichée comme une colonne Décimal), vide sinon.
	Result grid.Type `json:"result"`
	// Mise en forme de toute la colonne et de son en-tête (JSON, voir Style).
	Style       string `json:"style"`
	HeaderStyle string `json:"headerStyle"`
	// Mise en forme conditionnelle (JSON : liste de Rule).
	Rules string `json:"rules"`
	// Colonne saisie à côté d'un résultat chargé (hand.go) : modifiable,
	// gardée à l'actualisation ; en-tête vert, les colonnes chargées en bleu.
	Hand bool `json:"hand,omitempty"`
}

// TableInfo décrit un tableau posé sur une feuille.
type TableInfo struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	SQL   string `json:"sql"`
	Row   int    `json:"row"` // ancre : ligne d'en-tête
	Col   int    `json:"col"`
	Rows  int    `json:"rows"`  // lignes de données affichées (filtre)
	Total int    `json:"total"` // lignes de données, masquées comprises
	// Filtre (JSON : identifiant de colonne → ColumnFilter), vide sans filtre.
	Filter string `json:"filter"`
	// Source SQL des lignes affichées (la table, ou une requête filtrée).
	Source  string       `json:"source"`
	Columns []ColumnInfo `json:"columns"`
	// En-têtes sur deux lignes : le groupe de chaque nom « Groupe.Sous »
	// s'affiche dans une bande, ligne Row-1 (canvas/twolevel.go).
	TwoLevel bool `json:"twoLevel"`
	// Rang de la colonne clé (identifiant des lignes), -1 sans clé.
	Key int `json:"key"`
	// Sans clé : la première colonne semble identifier chaque ligne (entier
	// unique et croissant) ; l'interface propose d'en faire la clé.
	KeyHint bool `json:"keyHint"`
	// Rang de la colonne numérotée d'office (clé d'entiers, ou première
	// colonne en suite régulière : une ligne ajoutée reçoit le numéro
	// suivant, autokey.go), -1 sinon.
	AutoNumber int `json:"autoNumber"`
	// Requête d'import qui charge le tableau (source externe ou autre
	// tableau) : identifiant et nom, 0 et vide sinon.
	QueryID   int64  `json:"queryId"`
	QueryName string `json:"queryName"`
	// Colonnes saisies (hand.go) : clé de jointure (nom d'une colonne
	// chargée, « # » : rang des lignes, vide : pas encore choisie) et nombre
	// de lignes disparues dont des valeurs saisies sont mises de côté.
	HandKey  string `json:"handKey,omitempty"`
	HandKept int    `json:"handKept,omitempty"`
	// Classeur partagé : "published" (tableau publié sur le serveur) ou
	// "private" (resté local) ; vide pour un classeur local.
	Shared string `json:"shared,omitempty"`
	// ReadOnly : pourquoi le tableau publié ne se modifie pas (« Classeur
	// publié en lecture seule », « Hors ligne, lecture seule »), vide sinon.
	ReadOnly string `json:"readOnly,omitempty"`
	// Classeur collaboratif (§8.1) : niveau de l'utilisateur sur le tableau
	// publié (reader, participant, editor, none ; vide sinon) et lecture
	// filtrée par ses droits (« Vue filtrée par vos droits »).
	Access   string `json:"access,omitempty"`
	Filtered bool   `json:"filtered,omitempty"`
	// Follows : le tableau suit le choix de l'onglet (il a la colonne de
	// catégorie, rang ChoiceCol) ; Rows ne compte alors que les lignes de la
	// valeur choisie, les autres gardent leur place (lignes à remplir).
	Follows   bool `json:"follows,omitempty"`
	ChoiceCol int  `json:"choiceCol,omitempty"`
}

// SharingAccess est la partie facultative de Sharing qui donne les droits
// d'un classeur collaboratif (workbook.Link).
type SharingAccess interface {
	TableAccess(tbl int64) (level string, filtered bool)
}

// CellInfo est une cellule libre.
type CellInfo struct {
	Row int    `json:"row"`
	Col int    `json:"col"`
	Raw string `json:"raw"`
	// Cellule avec formule : valeur calculée, affichée à la place de Raw.
	Value string `json:"value"`
	Error bool   `json:"error"`
	// Tableau dynamique (FILTRE, TRIER…) : valeurs déversées depuis la
	// cellule, vers le bas et la droite (la première est Value).
	Spill [][]string `json:"spill,omitempty"`
}

// NameInfo est une cellule nommée de la feuille.
type NameInfo struct {
	Name string `json:"name"`
	Row  int    `json:"row"`
	Col  int    `json:"col"`
}

// Layout est la mise en page d'une feuille : ses tableaux et ses cellules
// libres. Les valeurs des tableaux se lisent ensuite par fenêtres.
type Layout struct {
	Sheets  []SheetInfo `json:"sheets"`
	Sheet   int64       `json:"sheet"`
	Tables  []TableInfo `json:"tables"`
	Cells   []CellInfo  `json:"cells"`
	Widths  []ColWidth  `json:"widths"`  // largeurs choisies par l'utilisateur
	Heights []RowHeight `json:"heights"` // hauteurs choisies par l'utilisateur
	Names   []NameInfo  `json:"names"`   // cellules nommées de la feuille
	Styles  []CellStyle `json:"styles"`  // mise en forme hors tableaux
	// Mise en forme des lignes et colonnes entières de la feuille.
	RowStyles []LineStyle `json:"rowStyles"`
	ColStyles []LineStyle `json:"colStyles"`
	// Mise en forme conditionnelle des lignes et colonnes entières.
	RowRules []LineRules `json:"rowRules"`
	ColRules []LineRules `json:"colRules"`
	// Règles « formule » vérifiées par les cellules libres (voir ruleMarks).
	RuleMarks []RuleMark   `json:"ruleMarks"`
	Pivots    []PivotInfo  `json:"pivots"`  // tableaux croisés, calculés
	Charts    []ChartInfo  `json:"charts"`  // graphiques, avec leurs données
	Buttons   []ButtonInfo `json:"buttons"` // boutons d'action
	// Ranges : zones « plage » (tableaux convertis), en rectangles.
	Ranges []Rect `json:"ranges"`
	// Structure du classeur protégée (onglets), avec ou sans mot de passe.
	WorkbookProtected bool `json:"workbookProtected"`
	WorkbookPassword  bool `json:"workbookPassword"`
	// … avec le contenu de toutes les feuilles verrouillé.
	WorkbookSheets bool `json:"workbookSheets"`
	// Mise en page de la feuille (onglet Mise en page).
	PageSetup PageSetup `json:"pageSetup"`
}

// CellStyle est la mise en forme d'une cellule hors tableau.
type CellStyle struct {
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Style string `json:"style"`
}

// ColWidth est la largeur choisie pour une colonne de la feuille.
// RowHeight est la hauteur choisie pour une ligne de la feuille.
type RowHeight struct {
	Row    int `json:"row"`
	Height int `json:"height"`
}

type ColWidth struct {
	Col   int `json:"col"`
	Width int `json:"width"`
}

// Layout lit la mise en page d'une feuille.
func (m *Model) Layout(ctx context.Context, sheet int64) (Layout, error) {
	l := Layout{Sheet: sheet, Tables: []TableInfo{}, Cells: []CellInfo{}, Widths: []ColWidth{}, Heights: []RowHeight{}, Names: []NameInfo{}, Styles: []CellStyle{}}
	err := m.read(ctx, func(t *tx) error {
		rs, err := t.query(`SELECT id, name, gridlines = 0, headings <> 0, color, hidden <> 0, freeze_rows, freeze_cols, zoom,
			coalesce((SELECT p.kind FROM _lattis_pages p WHERE p.sheet = _lattis_sheets.id), '')
			FROM _lattis_sheets ORDER BY pos, id`)
		if err != nil {
			return err
		}
		for rs.Next() {
			var s SheetInfo
			if err := rs.Scan(&s.ID, &s.Name, &s.NoGrid, &s.Headings, &s.Color, &s.Hidden, &s.FreezeRows, &s.FreezeCols, &s.Zoom, &s.Kind); err != nil {
				rs.Close()
				return err
			}
			l.Sheets = append(l.Sheets, s)
		}
		rs.Close()
		for i := range l.Sheets {
			if l.Sheets[i].Choice, err = t.sheetChoice(l.Sheets[i].ID); err != nil {
				return err
			}
		}
		prot, err := t.protection()
		if err != nil {
			return err
		}
		l.WorkbookProtected, l.WorkbookPassword, l.WorkbookSheets = prot.Workbook, prot.WorkbookPassword, prot.WorkbookSheets
		if l.PageSetup, err = t.pageSetup(sheet); err != nil {
			return err
		}
		for i, s := range l.Sheets {
			l.Sheets[i].Password, l.Sheets[i].Protected = prot.Sheets[s.ID]
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		for _, tb := range tables {
			ti := TableInfo{ID: tb.ID, Name: tb.Name, SQL: tb.sql(), Row: tb.Row, Col: tb.Col, Rows: tb.Shown, Total: tb.Rows, Source: tb.view(), Key: -1, TwoLevel: tb.TwoLevel}
			if s := t.m.sharing; s != nil {
				ti.Shared = s.TableShare(tb.ID)
				if err := s.TableLock(tb.ID); err != nil {
					ti.ReadOnly = err.Error()
				}
				if a, ok := s.(SharingAccess); ok {
					ti.Access, ti.Filtered = a.TableAccess(tb.ID)
				}
			}
			if err := t.row(`SELECT id, name FROM _lattis_queries WHERE target = ? ORDER BY id LIMIT 1`, tb.ID).Scan(&ti.QueryID, &ti.QueryName); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			for i, c := range tb.Cols {
				if c.ID == tb.Key {
					ti.Key = i
				}
				if tb.choice != nil && c.ID == tb.choice.Col.ID {
					ti.Follows, ti.ChoiceCol = true, i
				}
			}
			if ti.KeyHint, err = t.keyHint(tb); err != nil {
				return err
			}
			if ti.AutoNumber, err = t.autoNumbered(tb); err != nil {
				return err
			}
			if len(tb.Filter) > 0 {
				b, err := json.Marshal(tb.Filter)
				if err != nil {
					return err
				}
				ti.Filter = string(b)
			}
			errs := map[int64]int{}
			rs, err := t.query(`SELECT col, count(*) FROM _lattis_invalid WHERE tbl = ? GROUP BY col`, tb.ID)
			if err != nil {
				return err
			}
			for rs.Next() {
				var col int64
				var n int
				if err := rs.Scan(&col, &n); err != nil {
					rs.Close()
					return err
				}
				errs[col] = n
			}
			rs.Close()
			colStyles := map[[2]int64]string{}
			rs, err = t.query(`SELECT rid, col, style FROM _lattis_styles WHERE tbl = ? AND rid <= 0`, tb.ID)
			if err != nil {
				return err
			}
			for rs.Next() {
				var rid, col int64
				var style string
				if err := rs.Scan(&rid, &col, &style); err != nil {
					rs.Close()
					return err
				}
				colStyles[[2]int64{rid, col}] = style
			}
			rs.Close()
			for _, c := range tb.Cols {
				rules, err := t.columnRules(tb.ID, c.ID)
				if err != nil {
					return err
				}
				ci := ColumnInfo{ID: c.ID, Errors: errs[c.ID], Name: c.Name, SQL: colSQL(c.ID), Type: c.Type, Fixed: c.Fixed, Formula: displayFormula(c.Formula),
					Style: colStyles[[2]int64{0, c.ID}], HeaderStyle: colStyles[[2]int64{-1, c.ID}], Rules: rules}
				if c.Formula != "" {
					ci.Result = t.computedResult(tb, c)
				}
				ti.Columns = append(ti.Columns, ci)
			}
			if ti.QueryID != 0 {
				if err := t.handLayout(tb, ti.QueryID, &ti); err != nil {
					return err
				}
			}
			l.Tables = append(l.Tables, ti)
		}
		rs, err = t.query(`SELECT col, width FROM _lattis_col_widths WHERE sheet = ? ORDER BY col`, sheet)
		if err != nil {
			return err
		}
		for rs.Next() {
			var w ColWidth
			if err := rs.Scan(&w.Col, &w.Width); err != nil {
				rs.Close()
				return err
			}
			l.Widths = append(l.Widths, w)
		}
		rs.Close()
		rs, err = t.query(`SELECT row, height FROM _lattis_row_heights WHERE sheet = ? ORDER BY row`, sheet)
		if err != nil {
			return err
		}
		for rs.Next() {
			var h RowHeight
			if err := rs.Scan(&h.Row, &h.Height); err != nil {
				rs.Close()
				return err
			}
			l.Heights = append(l.Heights, h)
		}
		rs.Close()
		rs, err = t.query(`SELECT name, row, col FROM _lattis_names WHERE sheet = ? ORDER BY name`, sheet)
		if err != nil {
			return err
		}
		for rs.Next() {
			var n NameInfo
			if err := rs.Scan(&n.Name, &n.Row, &n.Col); err != nil {
				rs.Close()
				return err
			}
			l.Names = append(l.Names, n)
		}
		rs.Close()
		if l.RowStyles, l.ColStyles, err = t.lineStyles(sheet); err != nil {
			return err
		}
		if l.RowRules, l.ColRules, err = t.lineRulesOf(sheet); err != nil {
			return err
		}
		rs, err = t.query(`SELECT row, col, style FROM _lattis_cell_styles WHERE sheet = ?`, sheet)
		if err != nil {
			return err
		}
		for rs.Next() {
			var c CellStyle
			if err := rs.Scan(&c.Row, &c.Col, &c.Style); err != nil {
				rs.Close()
				return err
			}
			l.Styles = append(l.Styles, c)
		}
		rs.Close()
		cells, err := t.cells(sheet)
		if err != nil {
			return err
		}
		for _, c := range cells {
			ci := CellInfo{Row: c.Row, Col: c.Col, Raw: c.Raw}
			if isFormula(c.Raw) {
				if rows, ok, e := t.spillFree(sheet, c.Raw); ok {
					switch {
					case e.IsErr():
						ci.Value, ci.Error = e.String(), true
					case spillBlocked(c.Row, c.Col, rows, cells, tables):
						ci.Value, ci.Error = ErrSpillText, true
					case len(rows) == 0:
						ci.Value, ci.Error = "#CALC!", true
					default:
						ci.Value, ci.Spill = rows[0][0], rows
					}
				} else {
					v := t.evalFree(sheet, c.Raw)
					ci.Value, ci.Error = v.String(), v.IsErr()
				}
			}
			l.Cells = append(l.Cells, ci)
		}
		if l.RuleMarks, err = t.ruleMarks(sheet, l.Styles, l.RowRules, l.ColRules, l.Cells, l.Tables); err != nil {
			return err
		}
		if l.Pivots, err = t.pivots(sheet); err != nil {
			return err
		}
		if l.Charts, err = t.charts(sheet); err != nil {
			return err
		}
		if l.Buttons, err = t.buttons(sheet); err != nil {
			return err
		}
		marks, err := t.rangeMarks(sheet)
		l.Ranges = rangeRects(marks)
		return err
	})
	return l, err
}

// TableNames décrit un tableau pour l'aide à la saisie des formules.
type TableNames struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	// Kind : « table », ou « pivot » pour un tableau croisé (vue SQL).
	Kind string `json:"kind"`
	// SQL : nom interne (t_1) d'un tableau, nom du TCD sinon ; ColumnSQL
	// de même pour les colonnes ; Types et Rows pour l'onglet SQL.
	SQL       string   `json:"sql"`
	ColumnSQL []string `json:"columnSql"`
	Types     []string `json:"types"`
	Rows      int      `json:"rows"`
	// Sheet : feuille où il est posé.
	Sheet int64 `json:"sheet"`
	// Key : rang de la colonne clé d'un tableau, -1 si aucune.
	Key int `json:"key"`
	// ColsField : champ en colonnes d'un TCD ; ses colonnes, à partir de la
	// ColsFrom-ième (comptée depuis 1), en portent les valeurs.
	ColsField string `json:"colsField,omitempty"`
	ColsFrom  int    `json:"colsFrom,omitempty"`
}

// AllTables liste les tableaux de toutes les feuilles et leurs colonnes.
func (m *Model) AllTables(ctx context.Context) ([]TableNames, error) {
	var out []TableNames
	err := m.read(ctx, func(t *tx) error {
		rs, err := t.query(`SELECT id FROM _lattis_tables ORDER BY name`)
		if err != nil {
			return err
		}
		var ids []int64
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return err
			}
			ids = append(ids, id)
		}
		rs.Close()
		for _, id := range ids {
			tb, err := t.table(id)
			if err != nil {
				return err
			}
			tn := TableNames{Name: tb.Name, Kind: "table", SQL: tb.sql(), Rows: tb.Rows, Sheet: tb.Sheet, Key: -1}
			for i, c := range tb.Cols {
				if c.ID == tb.Key {
					tn.Key = i
				}
				tn.Columns = append(tn.Columns, c.Name)
				tn.ColumnSQL = append(tn.ColumnSQL, colSQL(c.ID))
				typ := string(c.Type)
				if c.Formula != "" {
					typ = "formula"
				}
				tn.Types = append(tn.Types, typ)
			}
			out = append(out, tn)
		}
		ps, err := t.allPivots()
		if err != nil {
			return err
		}
		for _, p := range ps {
			_, cols, nrows, err := t.pivotQueryHand(p.ID, p.Sheet, p.Cfg)
			if err != nil {
				continue
			}
			tn := TableNames{Name: p.Cfg.Name, Kind: "pivot", Sheet: p.Sheet, Key: -1, SQL: `"` + p.Cfg.Name + `"`, Rows: nrows}
			if cfg := p.Cfg.normalized(); cfg.Cols != "" {
				tn.ColsField, tn.ColsFrom = cfg.Cols, min(len(cfg.RowFields), len(cols))+1
			}
			for i, c := range cols {
				tn.Columns = append(tn.Columns, c)
				tn.ColumnSQL = append(tn.ColumnSQL, `"`+strings.ReplaceAll(c, `"`, `""`)+`"`)
				typ := "text"
				if i >= len(p.Cfg.RowFields) {
					typ = "decimal"
				}
				tn.Types = append(tn.Types, typ)
			}
			out = append(out, tn)
		}
		return nil
	})
	return out, err
}
