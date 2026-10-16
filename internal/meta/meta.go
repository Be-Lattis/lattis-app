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
	// 2 → 3 : colonnes calculées (formule de la colonne, en anglais canonique)
	// et colonne clé de chaque tableau (pour REFERENCE).
	`
ALTER TABLE _slitable_columns ADD COLUMN formula TEXT;
ALTER TABLE _slitable_tables ADD COLUMN key_col INTEGER;
`,
	// 3 → 4 : cellules nommées (pas d'adresse A1 : une cellule libre utilisée
	// dans une formule porte un nom, comme les noms définis d'Excel).
	`
CREATE TABLE _slitable_names (
	name  TEXT NOT NULL PRIMARY KEY COLLATE NOCASE,
	sheet INTEGER NOT NULL REFERENCES _slitable_sheets(id),
	row   INTEGER NOT NULL,
	col   INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX _slitable_names_cell ON _slitable_names(sheet, row, col);
`,
	// 4 → 5 : bibliothèque de scripts Lua du classeur (onglet SCRIPT).
	`
CREATE TABLE _slitable_scripts (
	id     INTEGER PRIMARY KEY,
	name   TEXT NOT NULL UNIQUE COLLATE NOCASE,
	source TEXT NOT NULL
) STRICT;
`,
	// 5 → 6 : mise en forme (JSON : gras, italique, couleurs, alignement,
	// format de nombre). Une cellule de tableau est désignée par sa ligne
	// (clé technique) et sa colonne, pour suivre les tris et insertions ;
	// une colonne entière a son propre style ; une cellule libre sa position.
	`
CREATE TABLE _slitable_styles (
	tbl   INTEGER NOT NULL,
	rid   INTEGER NOT NULL,        -- 0 : toute la colonne
	col   INTEGER NOT NULL,
	style TEXT NOT NULL,
	PRIMARY KEY (tbl, rid, col)
) STRICT;
CREATE TABLE _slitable_cell_styles (
	sheet INTEGER NOT NULL,
	row   INTEGER NOT NULL,
	col   INTEGER NOT NULL,
	style TEXT NOT NULL,
	PRIMARY KEY (sheet, row, col)
) STRICT;
`,
	// 6 → 7 : mise en forme conditionnelle d'une colonne (règles JSON,
	// évaluées sur les lignes affichées).
	`
ALTER TABLE _slitable_columns ADD COLUMN rules TEXT;
`,
	// 7 → 8 : filtre automatique d'un tableau (JSON : identifiant de colonne →
	// valeurs gardées, erreurs seulement).
	`
ALTER TABLE _slitable_tables ADD COLUMN filter TEXT;
`,
	// 8 → 9 : tableaux croisés dynamiques (configuration JSON), posés sur une
	// feuille et recalculés à chaque lecture.
	`
CREATE TABLE _slitable_pivots (
	id     INTEGER PRIMARY KEY,
	sheet  INTEGER NOT NULL REFERENCES _slitable_sheets(id),
	row    INTEGER NOT NULL,
	col    INTEGER NOT NULL,
	config TEXT NOT NULL
) STRICT;
`,
	// 9 → 10 : graphiques (configuration JSON), posés sur une feuille.
	`
CREATE TABLE _slitable_charts (
	id     INTEGER PRIMARY KEY,
	sheet  INTEGER NOT NULL REFERENCES _slitable_sheets(id),
	row    INTEGER NOT NULL,
	col    INTEGER NOT NULL,
	config TEXT NOT NULL
) STRICT;
`,
	// 10 → 11 : hauteur des lignes de chaque feuille (comme les largeurs).
	`
CREATE TABLE _slitable_row_heights (
	sheet  INTEGER NOT NULL REFERENCES _slitable_sheets(id),
	row    INTEGER NOT NULL,
	height INTEGER NOT NULL,
	PRIMARY KEY (sheet, row)
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
