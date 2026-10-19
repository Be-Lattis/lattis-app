package archive

import (
	"strings"

	"github.com/ncruces/go-sqlite3"
)

// Préfixes des corbeilles d'annulation (colonnes, lignes, tableaux, tris,
// conversions, imports, requêtes SQL : voir internal/canvas, Step.Purge).
var trashPrefixes = []string{"_lattis_trash", "_lattis_sqltrash_"}

// scrub retire de l'instantané à enregistrer l'historique d'annulation
// (_lattis_journal) et les corbeilles : le fichier .lattis ne garde jamais
// ce qui a été supprimé ou modifié. La base de travail les garde, Ctrl+Z
// marche jusqu'à la fermeture. secure_delete efface le contenu des pages
// libérées ; la base n'est compactée (VACUUM) que si elles pèsent lourd.
// extra : fonctions du moteur, pour les index et contraintes qui s'en
// servent au VACUUM.
func scrub(snap string, extra []func(*sqliteConn) error) error {
	c, err := sqlite3.Open(snap)
	if err != nil {
		return err
	}
	defer c.Close()
	for _, fn := range extra {
		if err := fn(c); err != nil {
			return err
		}
	}
	if err := c.Exec(`PRAGMA secure_delete = ON`); err != nil {
		return err
	}
	var trash []string
	has := map[string]bool{}
	st, _, err := c.Prepare(`SELECT name FROM sqlite_schema WHERE type = 'table'`)
	if err != nil {
		return err
	}
	for st.Step() {
		n := st.ColumnText(0)
		has[n] = true
		for _, p := range trashPrefixes {
			if strings.HasPrefix(n, p) {
				trash = append(trash, n)
				break
			}
		}
	}
	if err := st.Close(); err != nil {
		return err
	}
	if !has["_lattis_journal"] && len(trash) == 0 {
		return nil
	}
	if err := c.Exec(`BEGIN IMMEDIATE`); err != nil {
		return err
	}
	stmts := []string{}
	if has["_lattis_journal"] {
		stmts = append(stmts, `DELETE FROM _lattis_journal`)
	}
	if has["_lattis_outbox"] {
		// Étapes envoyées gardées pour annuler : sans objet sans historique.
		stmts = append(stmts, `UPDATE _lattis_outbox SET seq = NULL WHERE seq IS NOT NULL`)
	}
	for _, n := range trash {
		stmts = append(stmts, `DROP TABLE "`+strings.ReplaceAll(n, `"`, `""`)+`"`)
	}
	for _, q := range stmts {
		if err := c.Exec(q); err != nil {
			c.Exec(`ROLLBACK`)
			return err
		}
	}
	if err := c.Exec(`COMMIT`); err != nil {
		return err
	}
	var free, pages int64
	if st, _, err := c.Prepare(`SELECT (SELECT freelist_count FROM pragma_freelist_count), (SELECT page_count FROM pragma_page_count)`); err == nil {
		if st.Step() {
			free, pages = st.ColumnInt64(0), st.ColumnInt64(1)
		}
		st.Close()
	}
	if free > 0 && free*4 >= pages {
		return c.Exec(`VACUUM`)
	}
	return nil
}
