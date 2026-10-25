package archive

import (
	"bytes"
	"io"
	"os"
	"strings"

	"github.com/ncruces/go-sqlite3"
)

// Préfixes des corbeilles d'annulation (colonnes, lignes, tableaux, tris,
// conversions, imports, requêtes SQL : voir internal/canvas, Step.Purge).
var trashPrefixes = []string{"_lattis_trash", "_lattis_sqltrash_"}

// scrub retire de l'instantané à enregistrer l'historique d'annulation
// (_lattis_journal), les corbeilles et les images qu'aucun objet ne cite
// plus : le fichier .lattis ne garde jamais
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
	images := has["_lattis_images"] && has["_lattis_objects"]
	if !has["_lattis_journal"] && len(trash) == 0 && !images {
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
	if images {
		// Images qu'aucun objet ne cite plus (supprimé, insertion annulée) :
		// gardées dans la base de travail pour Ctrl+Z, pas dans le fichier.
		stmts = append(stmts, `DELETE FROM _lattis_images WHERE hash NOT IN (SELECT json_extract(config, '$.image') FROM _lattis_objects WHERE json_extract(config, '$.image') IS NOT NULL)`)
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
	// Des pages encore en place peuvent garder, dans leur espace libre, la
	// trace de corbeilles créées puis retirées pendant la session (avant
	// secure_delete) : le schéma qui les nommait. Dans ce cas seulement, la
	// base est compactée, ce qui réécrit chaque page.
	if len(trash) > 0 {
		left, err := mentions(snap, trashPrefixes)
		if err != nil {
			return err
		}
		if left {
			return c.Exec(`VACUUM`)
		}
	}
	return nil
}

// mentions dit si le fichier contient encore l'un des textes (lecture par
// blocs, avec un recouvrement pour un texte à cheval sur deux blocs).
func mentions(path string, texts []string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	long := 0
	for _, t := range texts {
		long = max(long, len(t))
	}
	buf := make([]byte, 1<<20+long)
	keep := 0
	for {
		n, err := io.ReadFull(f, buf[keep:])
		chunk := buf[:keep+n]
		for _, t := range texts {
			if bytes.Contains(chunk, []byte(t)) {
				return true, nil
			}
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		keep = min(long, len(chunk))
		copy(buf, chunk[len(chunk)-keep:])
	}
}
