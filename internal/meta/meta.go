// Package meta définit le schéma des tables de métadonnées _slitable_* et
// ses migrations.
//
// La version du format est le user_version de SQLite : chaque migration fait
// passer la base d'une version à la suivante, dans une transaction. Une base
// d'une version plus récente que celle du programme est refusée.
//
// Les tables utilisateur s'appellent t_<id> et leurs colonnes c_<id> : un nom
// affiché peut changer, contenir des accents, des espaces ou un mot réservé
// sans toucher au schéma SQL.
package meta

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lalamefine/slitable/internal/command"
	"github.com/lalamefine/slitable/internal/store"
)

// migrations[i] fait passer la base de la version i à la version i+1.
var migrations = []string{
	// 0 → 1 : feuilles, tableaux, colonnes, cellules libres, saisies
	// invalides, journal des commandes.
	`
CREATE TABLE _slitable_sheets (
	id   INTEGER PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	pos  INTEGER NOT NULL          -- rang de l'onglet
) STRICT;
CREATE TABLE _slitable_tables (
	id    INTEGER PRIMARY KEY,
	sheet INTEGER NOT NULL REFERENCES _slitable_sheets(id),
	name  TEXT NOT NULL UNIQUE,
	row   INTEGER NOT NULL,        -- ancre : ligne d'en-tête
	col   INTEGER NOT NULL         -- ancre : première colonne
) STRICT;
CREATE TABLE _slitable_columns (
	tbl   INTEGER NOT NULL REFERENCES _slitable_tables(id),
	id    INTEGER NOT NULL,
	name  TEXT NOT NULL,
	type  TEXT NOT NULL,
	fixed INTEGER NOT NULL DEFAULT 0, -- 0 : type inféré, élargi automatiquement
	pos   INTEGER NOT NULL,           -- rang d'affichage, 0..n-1
	PRIMARY KEY (tbl, id)
) STRICT;
CREATE TABLE _slitable_cells (
	sheet INTEGER NOT NULL REFERENCES _slitable_sheets(id),
	row   INTEGER NOT NULL,
	col   INTEGER NOT NULL,
	raw   TEXT NOT NULL,
	PRIMARY KEY (sheet, row, col)
) STRICT;
CREATE TABLE _slitable_invalid (
	tbl INTEGER NOT NULL,
	rid INTEGER NOT NULL,
	col INTEGER NOT NULL,
	raw TEXT NOT NULL,
	PRIMARY KEY (tbl, rid, col)
) STRICT;
INSERT INTO _slitable_sheets (id, name, pos) VALUES (1, 'Feuille 1', 0);
` + command.Schema,
	// 1 → 2 : largeur des colonnes de chaque feuille (comme Excel, une largeur
	// par colonne de la feuille, quel que soit le tableau qui s'y trouve).
	`
CREATE TABLE _slitable_col_widths (
	sheet INTEGER NOT NULL REFERENCES _slitable_sheets(id),
	col   INTEGER NOT NULL,
	width INTEGER NOT NULL,
	PRIMARY KEY (sheet, col)
) STRICT;
`,
}

// Version est la version du format que ce programme écrit.
var Version = len(migrations)

// ErrTooNew signale une base écrite par une version plus récente.
type ErrTooNew struct{ Found int }

func (e ErrTooNew) Error() string {
	return fmt.Sprintf("document au format %d, cette version de Slitable lit jusqu'au format %d", e.Found, Version)
}

// Migrate met la base à la version courante.
func Migrate(ctx context.Context, db *store.Store) error {
	return db.Write(ctx, func(tx *sql.Tx) error {
		var v int
		if err := tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v); err != nil {
			return err
		}
		if v > Version {
			return ErrTooNew{v}
		}
		for ; v < Version; v++ {
			if _, err := tx.ExecContext(ctx, migrations[v]); err != nil {
				return fmt.Errorf("migration %d → %d : %w", v, v+1, err)
			}
		}
		_, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, Version))
		return err
	})
}
