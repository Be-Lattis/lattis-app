package canvas

import (
	"context"
	"fmt"
	"strings"
)

// En-têtes tapés après les données : une ligne de noms posée juste au-dessus
// d'un tableau dont l'en-tête ressemble à des données (« Léa | 31 », ou
// « Colonne n »). La détection la garde libre quand elle n'est pas plus large
// que le tableau (titre possible, décision 0005) ; « Utiliser comme
// en-têtes » (TitleToHeader), l'inverse de « Sortir l'en-tête », l'y fait
// entrer, et la saisie le propose (HeaderOffer).

// titleAbove lit la ligne juste au-dessus de l'en-tête de tb, sur ses
// colonnes : les noms qu'elle donne ("" : cellule vide, le nom reste) et
// les cellules remplies juste à côté (hors de ses colonnes, laissées libres).
// Refus clair si elle ne peut pas devenir l'en-tête.
func (t *tx) titleAbove(tb Table) (names []string, beside [][2]int, err error) {
	if tb.TwoLevel {
		return nil, nil, fmt.Errorf("%w : « %s » a des en-têtes sur deux lignes, la ligne du dessus est sa bande de groupes (clic droit › Tableau › Retirer les groupes d'en-têtes d'abord)", ErrTable, tb.Name)
	}
	none := fmt.Errorf("%w : rien à prendre comme en-têtes au-dessus de « %s » (tapez les noms dans la ligne juste au-dessus de ses colonnes)", ErrTable, tb.Name)
	if tb.Row == 0 {
		return nil, nil, none
	}
	r := tb.Row - 1
	tables, err := t.tables(tb.Sheet)
	if err != nil {
		return nil, nil, err
	}
	for _, o := range tables {
		if o.ID != tb.ID && r >= o.Top() && r <= o.Bottom() && o.spansCols(tb.Col, tb.Right()) {
			return nil, nil, fmt.Errorf("%w : la ligne au-dessus de « %s » appartient au tableau « %s »", ErrTable, tb.Name, o.Name)
		}
	}
	pivots, err := t.pivots(tb.Sheet)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range pivots {
		if z := pivotRect(p); r >= z.R0 && r <= z.R1 && z.C0 <= tb.Right() && z.C1 >= tb.Col {
			return nil, nil, fmt.Errorf("%w : la ligne au-dessus de « %s » touche un tableau croisé", ErrTable, tb.Name)
		}
	}
	merges, err := t.merges(tb.Sheet)
	if err != nil {
		return nil, nil, err
	}
	for _, g := range merges {
		if g.overlaps(r, tb.Col, r, tb.Right()) {
			return nil, nil, fmt.Errorf("%w : la ligne au-dessus de « %s » a des cellules fusionnées (clic droit › Tableau › En-têtes sur 2 lignes pour en faire des groupes)", ErrTable, tb.Name)
		}
	}
	cells, err := t.cells(tb.Sheet)
	if err != nil {
		return nil, nil, err
	}
	marks, err := t.rangeMarks(tb.Sheet)
	if err != nil {
		return nil, nil, err
	}
	for c := tb.Col; c <= tb.Right(); c++ {
		if marks[[2]int{r, c}] {
			return nil, nil, fmt.Errorf("%w : la ligne au-dessus de « %s » fait partie d'une plage (tableau converti)", ErrTable, tb.Name)
		}
	}
	names = make([]string, len(tb.Cols))
	filled := false
	for _, c := range cells {
		v := strings.TrimSpace(c.Raw)
		if c.Row != r || v == "" {
			continue
		}
		switch {
		case c.Col == tb.Col-1 || c.Col == tb.Right()+1:
			beside = append(beside, [2]int{c.Row, c.Col})
		case c.Col >= tb.Col && c.Col <= tb.Right():
			if isFormula(v) {
				return nil, nil, fmt.Errorf("%w : « %s », au-dessus de « %s », est une formule : tapez le nom de la colonne", ErrTable, clip(v), tb.Name)
			}
			if len(v) > 1 && v[0] == '\'' {
				v = strings.TrimSpace(v[1:])
			}
			names[c.Col-tb.Col], filled = v, filled || v != ""
		}
	}
	if !filled {
		return nil, nil, none
	}
	return names, beside, nil
}

// TitleToHeader : « Utiliser comme en-têtes », l'inverse de « Sortir
// l'en-tête ». Les cellules libres juste au-dessus du tableau, sur ses
// colonnes, donnent les noms des colonnes (une cellule vide garde le nom) ;
// l'en-tête actuel devient la 1re ligne de données (sauf les noms « Colonne
// n », qui viennent de Lattis), ses valeurs typées comme une saisie (« 31 »
// reste un nombre dans une colonne de nombres). Le tableau remonte d'une
// ligne : la ligne de noms est son en-tête, le bas ne bouge pas. Les
// formules, TCD et graphiques suivent les renommages ; une seule étape.
func (m *Model) TitleToHeader(ctx context.Context, table string) error {
	return m.Do(ctx, "Utiliser comme en-têtes", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if tb.Filtered() {
			return tb.errFiltered(false)
		}
		if q, err := t.queryLoaded(tb.ID); err != nil || q {
			if err == nil {
				err = fmt.Errorf("%w : « %s » est chargé par une requête : renommez ses colonnes dans la requête (Données › Requêtes)", ErrTable, tb.Name)
			}
			return err
		}
		names, beside, err := t.titleAbove(tb)
		if err != nil {
			return err
		}
		r := tb.Row - 1
		// L'ancien en-tête : une ligne de données, sauf « Colonne n » et les
		// colonnes calculées (leur formule donne la valeur).
		old := make([]string, len(tb.Cols))
		keep := false
		for i, c := range tb.Cols {
			if c.Formula == "" && !autoColRe.MatchString(c.Name) {
				old[i], keep = c.Name, true
			}
		}
		final := make([]string, len(tb.Cols))
		for i, c := range tb.Cols {
			final[i] = c.Name
			if names[i] != "" {
				final[i] = names[i]
			}
		}
		final = uniqueColumnNames(final)
		// Les noms libérés d'abord : une colonne qui prend le nom d'une autre
		// passe par un nom provisoire (les formules suivent chaque étape).
		taken := map[string]int{}
		for i, c := range tb.Cols {
			taken[strings.ToLower(c.Name)] = i
		}
		swap := false
		for i, n := range final {
			if j, ok := taken[strings.ToLower(n)]; ok && j != i {
				swap = true
			}
		}
		if swap {
			for i, c := range tb.Cols {
				if final[i] == c.Name {
					continue
				}
				tmp := fmt.Sprintf("~%d~%s", i+1, c.Name)
				if err := t.rename(tb, c, tmp); err != nil {
					return err
				}
				tb.Cols[i].Name = tmp
			}
		}
		for i, c := range tb.Cols {
			if err := t.rename(tb, c, final[i]); err != nil {
				return err
			}
			tb.Cols[i].Name = final[i]
		}
		for i := range tb.Cols {
			if names[i] != "" {
				if err := t.setFreeCell(tb.Sheet, r, tb.Col+i, ""); err != nil {
					return err
				}
				if err := t.clearOut(tb.Sheet, r, tb.Col+i); err != nil {
					return err
				}
			}
		}
		// Ce qui est à côté de la ligne reste libre : sans la marque, la
		// cellule voisine du nouvel en-tête serait prise pour une colonne.
		for _, p := range beside {
			if err := t.markOut(tb.Sheet, p[0], p[1], ""); err != nil {
				return err
			}
		}
		if err := t.moveTable(tb, -1, 0); err != nil {
			return err
		}
		if tb, err = t.table(tb.ID); err != nil {
			return err
		}
		if keep {
			if err := t.prependRow(tb, old); err != nil {
				return err
			}
		}
		what := fmt.Sprintf("En-têtes de « %s » remplacés par la ligne du dessus", tb.Name)
		if keep {
			what += " ; l'ancien en-tête est devenu la 1re ligne"
		}
		t.notes = append(t.notes, what+" (Ctrl+Z pour annuler)")
		return nil
	})
}

// prependRow ajoute en tête du tableau une ligne de saisies (vals, "" :
// vide), typées comme une frappe : une colonne de nombres garde « 31 » en
// nombre, un texte l'élargit en texte.
func (t *tx) prependRow(tb Table, vals []string) error {
	var first string
	if tb.Rows > 0 {
		var err error
		if _, first, err = t.rowID(tb, 0); err != nil {
			return err
		}
	}
	pos, err := t.rowKey(tb, "", first)
	if err != nil {
		return err
	}
	var id int64
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM ` + tb.sql()).Scan(&id); err != nil {
		return err
	}
	if err := t.insertRow(tb, id, pos); err != nil {
		return err
	}
	var ev Event
	for i, v := range vals {
		if v == "" {
			continue
		}
		if isFormula(v) {
			v = "'" + v
		}
		if tb, err = t.setCellAt(tb, id, i, v, &ev); err != nil {
			return err
		}
	}
	return nil
}

// HeaderOffer : après une saisie en (row, col), le tableau dont la ligne
// tapée au-dessus ressemble à un en-tête (au moins deux noms, que des
// textes) alors que son en-tête actuel ressemble à des données (dataHeader) :
// la barre d'état propose « Utiliser comme en-têtes ». "" sinon ; une seule
// cellule au-dessus reste un titre, sans proposition. Rien n'est changé.
func (m *Model) HeaderOffer(ctx context.Context, sheet int64, row, col int) (string, error) {
	var name string
	err := m.read(ctx, func(t *tx) error {
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		for _, tb := range tables {
			if tb.Row != row+1 || col < tb.Col || col > tb.Right() || tb.TwoLevel || !dataHeader(tb) {
				continue
			}
			fixed, err := t.fixedTables([]Table{tb})
			if err != nil || fixed[tb.ID] != "" {
				return err
			}
			if locked, err := t.structureLocked(sheet); err != nil || locked {
				return err
			}
			if t.lockTable(tb.ID) != nil {
				return nil
			}
			names, _, err := t.titleAbove(tb)
			if err != nil {
				return nil // refus : pas de proposition
			}
			n, differ := 0, false
			for i, v := range names {
				if v == "" {
					continue
				}
				if !allText([]string{v}) {
					return nil
				}
				n++
				differ = differ || v != tb.Cols[i].Name
			}
			if n >= 2 && differ {
				name = tb.Name
			}
			return nil
		}
		return nil
	})
	return name, err
}
