package canvas

import (
	"errors"
	"fmt"
)

// Seul tableau d'une feuille (demande de Louis) : son nom et celui de
// l'onglet se suivent. Renommer l'un renomme l'autre, dans la même étape
// d'annulation, par les chemins ordinaires (formules, TCD, graphiques et
// adresses Feuille!… suivent ; classeur collaboratif : nom du tableau par
// la structure, nom de l'onglet par la mise en page, réservés aux
// concepteurs : un refus de l'un refuse le tout). Un nom pris (ou qui ne
// peut pas être un nom de tableau) laisse l'autre tel quel, la barre
// d'état le dit. Le lien naît sans rien renommer (deux tableaux → un) et
// cesse quand un deuxième tableau arrive. À la naissance, le premier
// tableau d'une feuille prend déjà son nom (tableName). Seuls les renommages
// de l'application suivent (RenameSheetLinked, RenameTableLinked) ; scripts
// et assistant renomment l'un sans l'autre. Décision 0005.

// soleTable rend le nom du seul tableau d'une feuille (ok faux si elle en a
// zéro ou plusieurs ; tous les tableaux comptent, requêtes comprises).
func (t *tx) soleTable(sheet int64) (name string, ok bool, err error) {
	rs, err := t.query(`SELECT name FROM _lattis_tables WHERE sheet = ? LIMIT 2`, sheet)
	if err != nil {
		return "", false, err
	}
	defer rs.Close()
	n := 0
	for rs.Next() {
		if err := rs.Scan(&name); err != nil {
			return "", false, err
		}
		n++
	}
	return name, n == 1, rs.Err()
}

// tableFollowsSheet renomme le seul tableau d'une feuille d'après le nouveau
// nom de son onglet (sans espace : « Ventes 2026 » → « Ventes2026 »).
func (t *tx) tableFollowsSheet(sheet int64, sheetName string) error {
	cur, ok, err := t.soleTable(sheet)
	if err != nil || !ok {
		return err
	}
	name := identifier(sheetName)
	if name == cur {
		return nil
	}
	if name == "" || A1Like(name) {
		t.notes = append(t.notes, fmt.Sprintf("Le tableau garde son nom « %s » : « %s » ne peut pas nommer un tableau", cur, sheetName))
		return nil
	}
	var k int
	if err := t.row(`SELECT count(*) FROM _lattis_tables WHERE name = ? COLLATE NOCASE AND name <> ?`, name, cur).Scan(&k); err != nil {
		return err
	}
	if k > 0 {
		t.notes = append(t.notes, fmt.Sprintf("Le tableau garde son nom « %s » : « %s » est déjà le nom d'un autre tableau", cur, name))
		return nil
	}
	tb, err := t.editTable(cur)
	if err != nil {
		return err
	}
	return t.renameTable(tb, name)
}

// sheetFollowsTable renomme l'onglet d'une feuille d'après le nouveau nom de
// son seul tableau.
func (t *tx) sheetFollowsTable(sheet int64, name string) error {
	if _, ok, err := t.soleTable(sheet); err != nil || !ok {
		return err
	}
	var old string
	if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&old); err != nil {
		return err
	}
	if old == name || identifier(old) == name {
		return nil
	}
	// Classeur ou structure de la feuille protégés : le tableau seul est
	// renommé. Le refus d'un classeur collaboratif (non-concepteur), lui,
	// refuse le tout.
	if err := t.guardTab(sheet); err != nil {
		if errors.As(err, new(ErrProtected)) {
			t.notes = append(t.notes, fmt.Sprintf("L'onglet garde son nom « %s » : il est protégé", old))
			return nil
		}
		return err
	}
	taken, err := t.sheetNameTaken(name, sheet)
	if err != nil {
		return err
	}
	if taken {
		t.notes = append(t.notes, fmt.Sprintf("L'onglet garde son nom « %s » : « %s » est déjà le nom d'une autre feuille", old, name))
		return nil
	}
	return t.renameSheet(sheet, old, name)
}
