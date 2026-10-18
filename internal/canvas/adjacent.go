package canvas

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/grid"
)

// Tableaux jointifs.
//
// Un tableau garde toujours au moins une cellule vide à sa gauche, à sa
// droite et sous lui. Quand une action (saisie, collage, déplacement,
// suppression ou insertion de lignes et de colonnes, conversion…) colle
// quelque chose contre lui, cela le rejoint à la fin de l'action, dans la
// même étape d'annulation :
//
//   - un tableau juste à sa droite (colonnes adjacentes, lignes qui se
//     recoupent) : les deux fusionnent (mergeRight), lignes alignées par rang ;
//     le plus gros garde son nom (à égalité, celui de gauche) ;
//   - une cellule libre juste à droite ou à gauche (lignes du tableau) :
//     elle devient une nouvelle colonne (sur la ligne d'en-tête : son nom) ;
//   - une cellule libre juste dessous (colonnes du tableau) : une nouvelle
//     ligne.
//
// Exactement comme si elle avait été tapée là (input). Au-dessus du tableau,
// une cellule libre reste libre (un titre) ; deux tableaux l'un sur l'autre
// ne fusionnent pas (le dessous s'écarte, voir ensureGapBelow).

// layoutRe reconnaît une instruction qui peut changer la disposition d'une
// feuille : ancres et colonnes des tableaux, cellules libres, lignes ajoutées
// ou retirées d'un tableau.
var layoutRe = regexp.MustCompile(`(?is)^\s*(INSERT\s+INTO|DELETE\s+FROM|ALTER\s+TABLE)\s+t_\d+\b|_lattis_(tables|cells|columns)\b`)

func (t *tx) markLayout(do []stmt) {
	if t.layout {
		return
	}
	for _, s := range do {
		if layoutRe.MatchString(s.Q) {
			t.layout = true
			return
		}
	}
}

// Apply et ApplyStep relèvent au passage les changements de disposition.
func (t *tx) Apply(do, undo []stmt) error {
	t.markLayout(do)
	return t.Tx.Apply(do, undo)
}

func (t *tx) ApplyStep(s command.Step) error {
	t.markLayout(s.Do)
	return t.Tx.ApplyStep(s)
}

// run exécute une action puis rattache ce qu'elle a rendu jointif.
func (m *Model) run(c *command.Tx, fn func(*tx) error) (*tx, error) {
	t := &tx{Tx: c, m: m}
	if err := fn(t); err != nil {
		return t, err
	}
	return t, t.joinAdjacent()
}

// note retient un message pour la barre d'état (voir TakeNotice).
func (m *Model) note(notes []string) {
	if len(notes) == 0 {
		return
	}
	m.noticeMu.Lock()
	m.notice = strings.Join(notes, " ; ")
	m.noticeMu.Unlock()
}

// TakeNotice rend (et oublie) le message de la dernière action, s'il y en a
// un : tableaux fusionnés, cellules rattachées à un tableau.
func (m *Model) TakeNotice() string {
	m.noticeMu.Lock()
	defer m.noticeMu.Unlock()
	s := m.notice
	m.notice = ""
	return s
}

// joinAdjacent rattache, sur chaque feuille non protégée, ce qui touche un
// tableau ; seulement si l'action a changé la disposition.
func (t *tx) joinAdjacent() error {
	if !t.layout || t.joining || t.ro {
		return nil
	}
	t.joining = true
	defer func() { t.joining = false }()
	rs, err := t.query(`SELECT id FROM _lattis_sheets ORDER BY id`)
	if err != nil {
		return err
	}
	var sheets []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return err
		}
		sheets = append(sheets, id)
	}
	rs.Close()
	for _, sh := range sheets {
		if t.guard(sh) != nil {
			continue
		}
		if err := t.joinSheet(sh); err != nil {
			return err
		}
	}
	return nil
}

func (t *tx) joinSheet(sheet int64) error {
	type joined struct {
		first string // première cellule rattachée
		n     int
	}
	absorbed := map[string]*joined{} // tableau → cellules rattachées
	var order []string
	defer func() {
		for _, name := range order {
			j := absorbed[name]
			what := "« " + clip(j.first) + " » rattaché"
			if j.n > 1 {
				what = fmt.Sprintf("%d cellules (« %s »…) rattachées", j.n, clip(j.first))
			}
			t.notes = append(t.notes, fmt.Sprintf("%s au tableau « %s » (Ctrl+Z pour annuler)", what, name))
		}
	}()
	for range 10000 {
		done, err := t.joinStep(sheet, func(tb Table, raw string, n int) {
			j := absorbed[tb.Name]
			if j == nil {
				j = &joined{first: raw}
				absorbed[tb.Name] = j
				order = append(order, tb.Name)
			}
			j.n += n
		})
		if err != nil || done {
			return err
		}
	}
	return fmt.Errorf("%w : impossible de rattacher les cellules voisines des tableaux", ErrTable)
}

// joinStep rattache une seule chose collée à un tableau de la feuille ; done
// quand il n'y a plus rien.
func (t *tx) joinStep(sheet int64, absorbed func(tb Table, first string, n int)) (done bool, err error) {
	tables, err := t.tables(sheet)
	if err != nil || len(tables) == 0 {
		return true, err
	}
	fixed, err := t.fixedTables(tables)
	if err != nil {
		return false, err
	}
	// Deux tableaux côte à côte : fusion.
	for _, a := range tables {
		for _, b := range tables {
			if a.ID == b.ID || fixed[a.ID] || fixed[b.ID] || a.Right()+1 != b.Col || a.Row > b.Bottom() || b.Row > a.Bottom() {
				continue
			}
			return false, t.joinTables(a, b)
		}
	}
	cells, err := t.cells(sheet)
	if err != nil {
		return false, err
	}
	for _, tb := range tables {
		if fixed[tb.ID] {
			continue
		}
		c, ok := touching(tb, cells)
		if !ok {
			continue
		}
		if err := t.setFreeCell(sheet, c.Row, c.Col, ""); err != nil {
			return false, err
		}
		var ev Event
		if err := t.input(sheet, c.Row, c.Col, c.Raw, &ev); err != nil {
			return false, err
		}
		if cur, err := t.table(tb.ID); err == nil {
			tb = cur
		}
		// La saisie a pu emmener d'autres cellules de la même colonne ou ligne.
		after, err := t.cells(sheet)
		if err != nil {
			return false, err
		}
		absorbed(tb, c.Raw, max(1, len(cells)-len(after)))
		return false, nil
	}
	return true, nil
}

func clip(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 20 {
		return string(r[:19]) + "…"
	}
	return s
}

// touching donne une cellule libre collée au tableau (à droite ou à gauche
// sur ses lignes, dessous sur ses colonnes) : celle de la ligne d'en-tête
// d'abord, pour qu'elle nomme la nouvelle colonne.
func touching(tb Table, cells []cell) (cell, bool) {
	var best cell
	found := false
	for _, c := range cells {
		side := (c.Col == tb.Right()+1 || c.Col == tb.Col-1) && c.Row >= tb.Row && c.Row <= tb.Bottom()
		below := c.Row == tb.Bottom()+1 && c.Col >= tb.Col && c.Col <= tb.Right()
		if !side && !below || strings.TrimSpace(c.Raw) == "" {
			continue
		}
		if !found || c.Row < best.Row || c.Row == best.Row && c.Col < best.Col {
			best, found = c, true
		}
	}
	return best, found
}

// fixedTables : tableaux qui ne grandissent ni ne fusionnent d'eux-mêmes
// (filtrés, ou résultats d'une requête d'import, que l'actualisation
// remplace).
func (t *tx) fixedTables(tables []Table) (map[int64]bool, error) {
	out := map[int64]bool{}
	for _, tb := range tables {
		if tb.Filtered() {
			out[tb.ID] = true
		}
	}
	rs, err := t.query(`SELECT target FROM _lattis_queries WHERE target IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rs.Err()
}

// joinTables fusionne b, collé à droite de a : le plus gros des deux garde
// son nom (à égalité, a).
func (t *tx) joinTables(a, b Table) error {
	bigger := b.Rows*len(b.Cols) > a.Rows*len(a.Cols)
	merged, err := t.mergeRight(a, b)
	if err != nil {
		return err
	}
	if bigger {
		q := `UPDATE _lattis_tables SET name = ? WHERE id = ?`
		if err := t.apply([]stmt{st(q, b.Name, merged.ID)}, []stmt{st(q, merged.Name, merged.ID)}); err != nil {
			return err
		}
	}
	t.notes = append(t.notes, fmt.Sprintf("Tableaux « %s » et « %s » fusionnés (Ctrl+Z pour annuler)", a.Name, b.Name))
	return nil
}

// growSide : une saisie juste à gauche (left) ou juste à droite d'un
// tableau, sur ses lignes, y ajoute une colonne (sur la ligne d'en-tête :
// son nom, sinon « Colonne n »). À gauche, le tableau s'ancre une colonne
// plus à gauche et ses autres colonnes ne bougent pas ; à droite, ce qui
// suit s'écarte. Les autres cellules libres de cette colonne, sur les
// lignes du tableau, le rejoignent aussi.
func (t *tx) growSide(tb Table, row, col int, raw string, ev *Event, left bool) error {
	if tb.Filtered() && row != tb.Row {
		return ErrFiltered{tb.Name}
	}
	taken, err := t.takeFree(tb.Sheet, func(c cell) bool {
		return c.Col == col && c.Row >= tb.Row && c.Row <= tb.Bottom() && c.Row != row
	})
	if err != nil {
		return err
	}
	name := fmt.Sprintf("Colonne %d", len(tb.Cols)+1)
	if row == tb.Row {
		name = strings.TrimSpace(raw)
	}
	for _, c := range taken {
		if c.Row == tb.Row && strings.TrimSpace(c.Raw) != "" {
			name = strings.TrimSpace(c.Raw)
		}
	}
	name = freeColumnName(tb, name)
	i := len(tb.Cols)
	if left {
		var id int64
		if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_columns WHERE tbl = ?`, tb.ID).Scan(&id); err != nil {
			return err
		}
		if err := t.addColumn(tb, id, name, grid.Text, 0); err != nil {
			return err
		}
		if err := t.moveTable(tb, 0, -1); err != nil {
			return err
		}
		i = 0
	} else if _, err := t.newColumn(tb, i, name); err != nil {
		return err
	}
	if tb, err = t.table(tb.ID); err != nil {
		return err
	}
	for _, c := range taken {
		if c.Row == tb.Row {
			continue
		}
		if err := t.setCell(tb, c.Row-tb.Row-1, i, c.Raw, ev); err != nil {
			return err
		}
		if tb, err = t.table(tb.ID); err != nil {
			return err
		}
	}
	if row == tb.Row {
		return nil
	}
	return t.setCell(tb, row-tb.Row-1, i, raw, ev)
}

// takeFree retire de la feuille les cellules libres retenues par keep et
// les rend (pour les saisir dans un tableau qui s'agrandit).
func (t *tx) takeFree(sheet int64, keep func(cell) bool) ([]cell, error) {
	cells, err := t.cells(sheet)
	if err != nil {
		return nil, err
	}
	var out []cell
	for _, c := range cells {
		if !keep(c) || strings.TrimSpace(c.Raw) == "" {
			continue
		}
		if err := t.setFreeCell(sheet, c.Row, c.Col, ""); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// inAny indique si la cellule (row, col) est dans un des tableaux.
func inAny(tables []Table, row, col int) bool {
	for _, tb := range tables {
		if row >= tb.Row && row <= tb.Bottom() && col >= tb.Col && col <= tb.Right() {
			return true
		}
	}
	return false
}

// freeColumnName rend name, suivi d'un numéro s'il est déjà pris dans tb.
func freeColumnName(tb Table, name string) string {
	taken := map[string]bool{}
	for _, c := range tb.Cols {
		taken[strings.ToLower(c.Name)] = true
	}
	out := name
	for n := 2; taken[strings.ToLower(out)]; n++ {
		out = fmt.Sprintf("%s %d", name, n)
	}
	return out
}
