package canvas

import (
	"database/sql"
	"errors"
	"github.com/lalamefine/lattis/internal/msg"
)

// Tableau qui porte le nom de son onglet (demande de Louis) : renommer
// l'onglet depuis l'application renomme aussi le tableau de cette feuille
// dont le nom est celui de l'onglet (sans espace : « Ventes 2026 » ↔
// « Ventes2026 »), quel que soit le nombre de tableaux de la feuille, dans
// la même étape d'annulation, par les chemins ordinaires (formules, TCD,
// graphiques et adresses Feuille!… suivent ; classeur collaboratif : nom du
// tableau par la structure, nom de l'onglet par la mise en page, réservés
// aux concepteurs : un refus de l'un refuse le tout). L'inverse ne se fait
// pas : renommer le tableau ne touche jamais l'onglet. Un nom pris (ou qui
// ne peut pas être un nom de tableau) laisse le tableau tel quel, la barre
// d'état le dit. À la naissance, le premier tableau d'une feuille prend déjà
// son nom (tableName). Seul le renommage de l'application suit
// (RenameSheetLinked) ; scripts et assistant renomment l'onglet seul.
// Décision 0005.

// tableFollowsSheet renomme le tableau de la feuille nommé comme l'ancien
// nom de l'onglet (old), d'après le nouveau (name).
func (t *tx) tableFollowsSheet(sheet int64, old, name string) error {
	var cur string
	err := t.row(`SELECT name FROM _lattis_tables WHERE sheet = ? AND name = ? COLLATE NOCASE LIMIT 1`, sheet, identifier(old)).Scan(&cur)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	to := identifier(name)
	if to == cur {
		return nil
	}
	if to == "" || A1Like(to) {
		t.notes = append(t.notes, t.say("note.tableNameInvalid", msg.Args{"table": cur, "name": name}))
		return nil
	}
	var k int
	if err := t.row(`SELECT count(*) FROM _lattis_tables WHERE name = ? COLLATE NOCASE AND name <> ?`, to, cur).Scan(&k); err != nil {
		return err
	}
	if k > 0 {
		t.notes = append(t.notes, t.say("note.tableNameTaken", msg.Args{"table": cur, "name": to}))
		return nil
	}
	tb, err := t.editTable(cur)
	if err != nil {
		return err
	}
	return t.renameTable(tb, to)
}
