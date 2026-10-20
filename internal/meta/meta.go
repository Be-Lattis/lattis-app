// Package meta définit le schéma des tables de métadonnées _lattis_* et
// ses migrations.
//
// La version du format est le user_version de SQLite : chaque migration fait
// passer la base d'une version à la suivante, dans une transaction. Une base
// d'une version plus récente que celle du programme est refusée.
//
// Les tables utilisateur s'appellent t_<id> et leurs colonnes c_<id> : un nom
// affiché peut changer, contenir des accents, des espaces ou un mot réservé
// sans toucher au schéma SQL.
//
// Le format est décrit pour les outils tiers dans docs/format-lattis.md : à
// tenir à jour avec chaque migration.
package meta

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/store"
)

// migrations[i] fait passer la base de la version i à la version i+1.
var migrations = []string{
	// 0 → 1 : feuilles, tableaux, colonnes, cellules libres, saisies
	// invalides, journal des commandes.
	`
CREATE TABLE _lattis_sheets (
	id   INTEGER PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	pos  INTEGER NOT NULL          -- rang de l'onglet
) STRICT;
CREATE TABLE _lattis_tables (
	id    INTEGER PRIMARY KEY,
	sheet INTEGER NOT NULL REFERENCES _lattis_sheets(id),
	name  TEXT NOT NULL UNIQUE,
	row   INTEGER NOT NULL,        -- ancre : ligne d'en-tête
	col   INTEGER NOT NULL         -- ancre : première colonne
) STRICT;
CREATE TABLE _lattis_columns (
	tbl   INTEGER NOT NULL REFERENCES _lattis_tables(id),
	id    INTEGER NOT NULL,
	name  TEXT NOT NULL,
	type  TEXT NOT NULL,
	fixed INTEGER NOT NULL DEFAULT 0, -- 0 : type inféré, élargi automatiquement
	pos   INTEGER NOT NULL,           -- rang d'affichage, 0..n-1
	PRIMARY KEY (tbl, id)
) STRICT;
CREATE TABLE _lattis_cells (
	sheet INTEGER NOT NULL REFERENCES _lattis_sheets(id),
	row   INTEGER NOT NULL,
	col   INTEGER NOT NULL,
	raw   TEXT NOT NULL,
	PRIMARY KEY (sheet, row, col)
) STRICT;
CREATE TABLE _lattis_invalid (
	tbl INTEGER NOT NULL,
	rid INTEGER NOT NULL,
	col INTEGER NOT NULL,
	raw TEXT NOT NULL,
	PRIMARY KEY (tbl, rid, col)
) STRICT;
INSERT INTO _lattis_sheets (id, name, pos) VALUES (1, 'Feuille1', 0);
` + command.Schema,
	// 1 → 2 : largeur des colonnes de chaque feuille (comme Excel, une largeur
	// par colonne de la feuille, quel que soit le tableau qui s'y trouve).
	`
CREATE TABLE _lattis_col_widths (
	sheet INTEGER NOT NULL REFERENCES _lattis_sheets(id),
	col   INTEGER NOT NULL,
	width INTEGER NOT NULL,
	PRIMARY KEY (sheet, col)
) STRICT;
`,
	// 2 → 3 : colonnes calculées (formule de la colonne, en anglais canonique)
	// et colonne clé de chaque tableau (pour REFERENCE).
	`
ALTER TABLE _lattis_columns ADD COLUMN formula TEXT;
ALTER TABLE _lattis_tables ADD COLUMN key_col INTEGER;
`,
	// 3 → 4 : cellules nommées (pas d'adresse A1 : une cellule libre utilisée
	// dans une formule porte un nom, comme les noms définis d'Excel).
	`
CREATE TABLE _lattis_names (
	name  TEXT NOT NULL PRIMARY KEY COLLATE NOCASE,
	sheet INTEGER NOT NULL REFERENCES _lattis_sheets(id),
	row   INTEGER NOT NULL,
	col   INTEGER NOT NULL
) STRICT;
CREATE UNIQUE INDEX _lattis_names_cell ON _lattis_names(sheet, row, col);
`,
	// 4 → 5 : bibliothèque de scripts Lua du classeur (onglet Script).
	`
CREATE TABLE _lattis_scripts (
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
CREATE TABLE _lattis_styles (
	tbl   INTEGER NOT NULL,
	rid   INTEGER NOT NULL,        -- 0 : toute la colonne
	col   INTEGER NOT NULL,
	style TEXT NOT NULL,
	PRIMARY KEY (tbl, rid, col)
) STRICT;
CREATE TABLE _lattis_cell_styles (
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
ALTER TABLE _lattis_columns ADD COLUMN rules TEXT;
`,
	// 7 → 8 : filtre automatique d'un tableau (JSON : identifiant de colonne →
	// valeurs gardées, erreurs seulement).
	`
ALTER TABLE _lattis_tables ADD COLUMN filter TEXT;
`,
	// 8 → 9 : tableaux croisés dynamiques (configuration JSON), posés sur une
	// feuille et recalculés à chaque lecture.
	`
CREATE TABLE _lattis_pivots (
	id     INTEGER PRIMARY KEY,
	sheet  INTEGER NOT NULL REFERENCES _lattis_sheets(id),
	row    INTEGER NOT NULL,
	col    INTEGER NOT NULL,
	config TEXT NOT NULL
) STRICT;
`,
	// 9 → 10 : graphiques (configuration JSON), posés sur une feuille.
	`
CREATE TABLE _lattis_charts (
	id     INTEGER PRIMARY KEY,
	sheet  INTEGER NOT NULL REFERENCES _lattis_sheets(id),
	row    INTEGER NOT NULL,
	col    INTEGER NOT NULL,
	config TEXT NOT NULL
) STRICT;
`,
	// 10 → 11 : hauteur des lignes de chaque feuille (comme les largeurs).
	`
CREATE TABLE _lattis_row_heights (
	sheet  INTEGER NOT NULL REFERENCES _lattis_sheets(id),
	row    INTEGER NOT NULL,
	height INTEGER NOT NULL,
	PRIMARY KEY (sheet, row)
) STRICT;
`,
	// 11 → 12 : quadrillage affiché ou masqué, feuille par feuille.
	`
ALTER TABLE _lattis_sheets ADD COLUMN gridlines INTEGER NOT NULL DEFAULT 1;
`,
	// 12 → 13 : boutons d'action (configuration JSON : libellé, script,
	// fonction, taille), posés sur une feuille comme les graphiques.
	`
CREATE TABLE _lattis_buttons (
	id     INTEGER PRIMARY KEY,
	sheet  INTEGER NOT NULL REFERENCES _lattis_sheets(id),
	row    INTEGER NOT NULL,
	col    INTEGER NOT NULL,
	config TEXT NOT NULL
) STRICT;
`,
	// 13 → 14 : en-têtes A1 (lettres et numéros) toujours affichés.
	`
ALTER TABLE _lattis_sheets ADD COLUMN headings INTEGER NOT NULL DEFAULT 0;
`,
	// 14 → 15 : mise en forme des lignes et colonnes entières de la feuille
	// (JSON, voir canvas.Style) : elle vaut pour toutes leurs cellules,
	// présentes et à venir ; une cellule l'emporte sur sa ligne, une ligne
	// sur sa colonne.
	`
CREATE TABLE _lattis_row_styles (
	sheet INTEGER NOT NULL,
	row   INTEGER NOT NULL,
	style TEXT NOT NULL,
	PRIMARY KEY (sheet, row)
) STRICT;
CREATE TABLE _lattis_col_styles (
	sheet INTEGER NOT NULL,
	col   INTEGER NOT NULL,
	style TEXT NOT NULL,
	PRIMARY KEY (sheet, col)
) STRICT;
`,
	// 15 → 16 : mise en forme conditionnelle des lignes et colonnes entières.
	`
CREATE TABLE _lattis_row_rules (
	sheet INTEGER NOT NULL,
	row   INTEGER NOT NULL,
	rules TEXT NOT NULL,
	PRIMARY KEY (sheet, row)
) STRICT;
CREATE TABLE _lattis_col_rules (
	sheet INTEGER NOT NULL,
	col   INTEGER NOT NULL,
	rules TEXT NOT NULL,
	PRIMARY KEY (sheet, col)
) STRICT;
`,
	// 16 → 17 : requêtes favorites de l'onglet SQL (nom affiché et texte
	// SQL tel que saisi, avec les noms affichés), dans l'ordre d'ajout.
	`
CREATE TABLE _lattis_favorites (
	id   INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	sql  TEXT NOT NULL
) STRICT;
`,
	// 17 → 18 : protection des feuilles et de la structure du classeur
	// (sheet = 0). hash : vide sans mot de passe, sinon empreinte salée
	// PBKDF2-SHA256 (jamais le mot de passe en clair, voir canvas/protect.go).
	// Hors du journal : protéger ou ôter la protection ne s'annule pas.
	`
CREATE TABLE _lattis_protection (
	sheet INTEGER NOT NULL PRIMARY KEY,
	hash  TEXT NOT NULL
) STRICT;
`,
	// 18 → 19 : sources externes et requêtes d'import (équivalent de Power
	// Query, voir docs/decisions/0003). config d'une connexion : JSON sans
	// mot de passe (rangé dans le trousseau du système). source et steps d'une
	// requête : JSON (internal/query) ; target : tableau chargé (pas « tbl » :
	// la protection des feuilles empreinte les tables qui ont une colonne tbl).
	`
CREATE TABLE _lattis_connections (
	id     INTEGER PRIMARY KEY,
	name   TEXT NOT NULL UNIQUE COLLATE NOCASE,
	kind   TEXT NOT NULL,
	config TEXT NOT NULL
) STRICT;
CREATE TABLE _lattis_queries (
	id        INTEGER PRIMARY KEY,
	name      TEXT NOT NULL UNIQUE COLLATE NOCASE,
	source    TEXT NOT NULL,
	steps     TEXT NOT NULL,
	target    INTEGER,
	refreshed TEXT
) STRICT;
`,
	// 19 → 20 : couleur d'onglet (« #rrggbb », vide : aucune) et feuilles
	// masquées (Masquer / Afficher…, au moins une feuille reste visible).
	`
ALTER TABLE _lattis_sheets ADD COLUMN color TEXT NOT NULL DEFAULT '';
ALTER TABLE _lattis_sheets ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0;
`,
	// 20 → 21 : volets figés (Affichage › Figer les volets) : nombre de
	// lignes et de colonnes qui restent affichées pendant le défilement.
	`
ALTER TABLE _lattis_sheets ADD COLUMN freeze_rows INTEGER NOT NULL DEFAULT 0;
ALTER TABLE _lattis_sheets ADD COLUMN freeze_cols INTEGER NOT NULL DEFAULT 0;
`,
	// 21 → 22 : zoom de la feuille (Affichage › Zoom), en pour cent (10 à
	// 400). Réglage d'affichage, hors du journal d'annulation.
	`
ALTER TABLE _lattis_sheets ADD COLUMN zoom INTEGER NOT NULL DEFAULT 100;
`,
	// 22 → 23 : onglets « page HTML » (une ligne par feuille de ce type :
	// HTML, CSS et JavaScript affichés dans un cadre isolé) et hôtes que les
	// scripts du classeur peuvent joindre en HTTP sans redemander (hors du
	// journal : une autorisation ne s'annule pas).
	`
CREATE TABLE _lattis_pages (
	sheet  INTEGER NOT NULL PRIMARY KEY REFERENCES _lattis_sheets(id),
	source TEXT NOT NULL
) STRICT;
CREATE TABLE _lattis_http_hosts (
	host TEXT NOT NULL PRIMARY KEY COLLATE NOCASE
) STRICT;
`,
	// 23 → 24 : onglets « Document » (texte riche, avec graphiques et
	// boutons) : des pages d'un autre genre.
	`
ALTER TABLE _lattis_pages ADD COLUMN kind TEXT NOT NULL DEFAULT 'html';
`,
	// 24 → 25 : zones « plage » (un tableau converti en plage, comme dans
	// Excel) : leurs cellules libres ne refont pas de tableau d'elles-mêmes
	// et ne rejoignent pas un tableau voisin.
	`
CREATE TABLE _lattis_range_cells (
	sheet INTEGER NOT NULL,
	row   INTEGER NOT NULL,
	col   INTEGER NOT NULL,
	PRIMARY KEY (sheet, row, col)
) STRICT;
`,
	// 25 → 26 : « Ne plus proposer » la première colonne d'un tableau comme
	// clé (voir canvas/keyhint.go).
	`
ALTER TABLE _lattis_tables ADD COLUMN no_key_hint INTEGER NOT NULL DEFAULT 0;
`,
	// 26 → 27 : informations du classeur ; « id » est son identifiant
	// aléatoire, qui le désigne sur ce poste pour l'accord d'utiliser un mot
	// de passe enregistré (query.Secrets).
	`
CREATE TABLE _lattis_info (
	key   TEXT NOT NULL PRIMARY KEY,
	value TEXT NOT NULL
) STRICT;
INSERT INTO _lattis_info (key, value) VALUES ('id', lower(hex(randomblob(16))));
`,
	// 27 → 28 : classeurs partagés sur un Serveur Lattis (journal logique,
	// voir docs/collaboration/journal-logique.md). Rien ne change pour un
	// tableau non publié : ces tables restent vides.
	//
	// _lattis_remote_tables : tableau local ↔ table du serveur ; published
	// fait relever ses opérations logiques (internal/remote/oplog). columns :
	// JSON des colonnes ↔ colonnes du serveur et de leurs types figés ;
	// since : xid8 (en texte) de la dernière lecture des changements.
	//
	// _lattis_row_ids : identifiant partagé (lattis_id, UUIDv7) et auteur
	// de chaque ligne d'un tableau publié, par sa clé locale (rid = id de
	// t_<tbl>). Une table à part plutôt que deux colonnes de t_<id> : les
	// reconstructions de tableaux (type, colonnes, conversions, copies), le
	// SQL des utilisateurs (SELECT *) et les exports ne voient que les
	// colonnes c_<id>, et un classeur non partagé n'est pas touché.
	//
	// _lattis_outbox : file des étapes à envoyer, dans l'ordre de id. seq :
	// commande du journal (NULL quand elle n'y est plus) ; kind : do, undo,
	// redo ; batch : parties d'une même transaction du serveur (id de la
	// première) ; ops : JSON des command.Op.
	//
	// _lattis_conflicts : conflits en attente de décision (cellule, valeur
	// locale, valeur du serveur, auteur, date).
	//
	// _lattis_capabilities : niveau effectif par tableau (tbl = 0 : le
	// classeur, avec le rôle), filtres et restrictions connus (JSON).
	`
CREATE TABLE _lattis_remote_tables (
	tbl         INTEGER PRIMARY KEY,
	published   INTEGER NOT NULL DEFAULT 0,
	remote_name TEXT,
	remote_id   TEXT,
	columns     TEXT,
	since       TEXT
) STRICT;
CREATE TABLE _lattis_row_ids (
	tbl           INTEGER NOT NULL,
	rid           INTEGER NOT NULL,
	lattis_id     TEXT NOT NULL UNIQUE,
	lattis_author TEXT,
	PRIMARY KEY (tbl, rid)
) STRICT, WITHOUT ROWID;
CREATE TABLE _lattis_outbox (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	seq     INTEGER,
	kind    TEXT NOT NULL CHECK (kind IN ('do', 'undo', 'redo')),
	batch   INTEGER NOT NULL,
	ops     TEXT NOT NULL,
	state   TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'sent', 'conflict', 'rejected')),
	error   TEXT,
	created TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;
CREATE INDEX _lattis_outbox_seq ON _lattis_outbox(seq);
CREATE INDEX _lattis_outbox_state ON _lattis_outbox(state, id);
CREATE TABLE _lattis_conflicts (
	id      INTEGER PRIMARY KEY,
	outbox  INTEGER,
	tbl     INTEGER NOT NULL,
	row_id  TEXT NOT NULL,
	col     INTEGER NOT NULL,
	local   TEXT,
	server  TEXT,
	author  TEXT,
	at      TEXT
) STRICT;
CREATE TABLE _lattis_capabilities (
	tbl          INTEGER PRIMARY KEY,
	level        TEXT NOT NULL,
	role         TEXT,
	filters      TEXT,
	restrictions TEXT
) STRICT;
`,
	// 28 → 29 : tableaux liés à une donnée du Data Lake (requête sur une vue
	// donnees.* d'un Serveur Lattis, internal/bridge/data_sync.go), tenus à
	// jour par relevés incrémentaux hors du journal d'annulation.
	//
	// _lattis_data_rows : identifiant de chaque ligne dans la donnée
	// (lattis_id du tableau source) par sa clé locale (rid = id de t_<tbl>) ;
	// à part de _lattis_row_ids, propre aux tableaux publiés (une même ligne
	// peut être dans un tableau publié et dans le tableau lié qui le lit).
	//
	// _lattis_data_sync : donnée et point de reprise (xid8 en texte) du
	// dernier relevé de chaque tableau lié.
	`
CREATE TABLE _lattis_data_rows (
	tbl       INTEGER NOT NULL,
	rid       INTEGER NOT NULL,
	lattis_id TEXT NOT NULL,
	PRIMARY KEY (tbl, rid),
	UNIQUE (tbl, lattis_id)
) STRICT, WITHOUT ROWID;
CREATE TABLE _lattis_data_sync (
	tbl     INTEGER PRIMARY KEY,
	dataset TEXT NOT NULL,
	since   TEXT NOT NULL
) STRICT;
`,
	// 29 → 30 : en-têtes sur deux lignes (canvas/twolevel.go) : le nom
	// « Groupe.Sous » d'une colonne s'affiche en deux parties, le groupe dans
	// une bande au-dessus de l'en-tête.
	`
ALTER TABLE _lattis_tables ADD COLUMN two_level INTEGER NOT NULL DEFAULT 0;
`,
	// 30 → 31 : mise en page de chaque feuille (onglet Mise en page : papier,
	// orientation, marges, échelle, zone d'impression, titres, sauts de page,
	// en-tête et pied de page), en JSON (canvas.PageSetup) ; pas de ligne :
	// réglages par défaut.
	`
CREATE TABLE _lattis_page_setup (
	sheet INTEGER NOT NULL PRIMARY KEY REFERENCES _lattis_sheets(id),
	setup TEXT NOT NULL
) STRICT;
`,
	// 31 → 32 : colonnes saisies à côté d'un résultat chargé (tableau d'une
	// requête d'import ou de l'onglet SQL, canvas/hand.go, décision 0008).
	// owner désigne le résultat : « query:<id> » (« pivot:<id> » réservé
	// aux tableaux croisés).
	//
	// _lattis_hand : clé de jointure, nom d'une colonne chargée (« # » : rang
	// de la ligne ; pas de ligne : pas encore choisie).
	// _lattis_hand_cols : colonnes saisies (h, numéro stable) et leur
	// identifiant dans le tableau chargé actuel.
	// _lattis_hand_kept : valeurs saisies sur des lignes disparues à
	// l'actualisation, par valeur de clé, rattachées si la clé revient
	// (invalid : texte hors type, comme _lattis_invalid).
	`
CREATE TABLE _lattis_hand (
	owner TEXT NOT NULL PRIMARY KEY,
	key   TEXT NOT NULL
) STRICT;
CREATE TABLE _lattis_hand_cols (
	owner TEXT NOT NULL,
	h     INTEGER NOT NULL,
	col   INTEGER NOT NULL,
	PRIMARY KEY (owner, h)
) STRICT;
CREATE TABLE _lattis_hand_kept (
	owner   TEXT NOT NULL,
	k       TEXT NOT NULL,
	h       INTEGER NOT NULL,
	v       ANY,
	invalid INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (owner, k, h)
) STRICT;
`,
	// 32 → 33 : onglet à choix (canvas/choice.go, conception
	// regrouper-onglets-semblables §6.3) : catégorie de la feuille (colonne
	// source, nom de repli) et dernière valeur choisie, en JSON ; NULL : pas
	// de choix. Classeur partagé : la catégorie est publiée, la valeur
	// choisie reste sur le poste (remote/workbook/choice.go).
	`
ALTER TABLE _lattis_sheets ADD COLUMN choice TEXT;
`,
	// 33 → 34 : feuilles à plusieurs tableaux regroupées (canvas/choicecells.go,
	// conception regrouper-onglets-semblables §6.5, lot 4) : un tableau peut ne
	// pas suivre l'onglet à choix malgré sa colonne de catégorie (follow = 0 ;
	// NULL : il suit s'il a la colonne) ; cellules par valeur de la catégorie
	// (« Objectif : 5 000 en 2024, 6 000 en 2025 »), la cellule libre montrant
	// celle de la valeur choisie (value « * » : « (Toutes) »).
	`
ALTER TABLE _lattis_tables ADD COLUMN follow INTEGER;
CREATE TABLE _lattis_choice_cells (
	sheet INTEGER NOT NULL REFERENCES _lattis_sheets(id),
	row   INTEGER NOT NULL,
	col   INTEGER NOT NULL,
	value TEXT NOT NULL,
	raw   TEXT NOT NULL,
	PRIMARY KEY (sheet, row, col, value)
) STRICT;
`,
	// 34 → 35 : protection de la mise en forme d'une feuille (Révision ›
	// Protéger la mise en forme, canvas/protect.go) : styles, largeurs,
	// hauteurs, fusions et règles verrouillés, la saisie reste permise.
	// hash comme _lattis_protection ; hors du journal. Elle garde depuis la
	// structure de la feuille (Protéger la structure : saisie dans les
	// tableaux seulement), sans changement de schéma.
	`
CREATE TABLE _lattis_format_protection (
	sheet INTEGER NOT NULL PRIMARY KEY,
	hash  TEXT NOT NULL
) STRICT;
`,
}

// Version est la version du format que ce programme écrit.
var Version = len(migrations)

// ErrTooNew signale une base écrite par une version plus récente.
type ErrTooNew struct{ Found int }

func (e ErrTooNew) Error() string { return e.Msg().String() }

// Msg : le message de l'erreur (msg.Messager).
func (e ErrTooNew) Msg() msg.Message {
	return msg.M("archive.tooNew", msg.Args{"format": e.Found, "max": Version})
}

// ErrForeign signale une base SQLite qui n'est pas un classeur Lattis
// (version 0, mais déjà des tables).
var ErrForeign = msg.New("archive.foreignDatabase", nil)

// MigrateTo met une base neuve au format v (classeurs d'un format ancien,
// pour les tests de migration).
func MigrateTo(ctx context.Context, db *store.Store, v int) error {
	return db.Write(ctx, func(tx *sql.Tx) error {
		for i := range v {
			if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, v))
		return err
	})
}

// Migrate met la base à la version courante.
func Migrate(ctx context.Context, db *store.Store) error {
	_, err := MigrateFrom(ctx, db)
	return err
}

// MigrateFrom met la base à la version courante et donne sa version
// d'origine (0 : base neuve). Toutes les étapes passent dans une seule
// transaction : un échec laisse la base telle qu'elle était.
func MigrateFrom(ctx context.Context, db *store.Store) (from int, err error) {
	r, err := MigrateOpen(ctx, db)
	return r.From, err
}

// Opened est le résultat de MigrateOpen.
type Opened struct {
	// From : version d'origine (0 : base neuve).
	From int
	// Dropped : déclencheurs inconnus supprimés (voir MigrateOpen).
	Dropped []string
}

// MigrateOpen prépare une base à l'ouverture : supprime les déclencheurs
// (TRIGGER) du schéma principal, puis la met à la version courante, dans
// une seule transaction (un échec laisse la base telle qu'elle était).
//
// Lattis ne crée aucun déclencheur persistant : les siens sont temporaires,
// sur la connexion (canvas/sqlexec.go, remote/oplog) ; l'utilisateur ne
// peut pas en créer (autorisateur, store/authorizer.go). Un déclencheur
// rangé dans le fichier y a donc été écrit hors de Lattis (classeur piégé)
// et s'exécuterait avec les écritures du moteur, migrations comprises : il
// est supprimé avant toute autre écriture. Les vues et les index inconnus,
// qui ne font rien d'eux-mêmes, restent.
func MigrateOpen(ctx context.Context, db *store.Store) (r Opened, err error) {
	err = db.Write(ctx, func(tx *sql.Tx) error {
		r = Opened{}
		var v int
		if err := tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v); err != nil {
			return err
		}
		r.From = v
		if v > Version {
			return ErrTooNew{v}
		}
		fresh := v == 0
		if fresh {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master`).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return ErrForeign
			}
		}
		dropped, err := dropTriggers(ctx, tx)
		if err != nil {
			return err
		}
		r.Dropped = dropped
		for ; v < Version; v++ {
			if _, err := tx.ExecContext(ctx, migrations[v]); err != nil {
				return fmt.Errorf("migration du format %d au format %d : %w", v, v+1, err)
			}
		}
		// Nouveau classeur : feuille sans quadrillage, comme le veut Louis (les
		// documents existants gardent le leur).
		if fresh {
			if _, err := tx.ExecContext(ctx, `UPDATE _lattis_sheets SET gridlines = 0`); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, Version))
		return err
	})
	return r, err
}

// dropTriggers supprime les déclencheurs du schéma principal et donne leurs
// noms (voir MigrateOpen).
func dropTriggers(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rs, err := tx.QueryContext(ctx, `SELECT name FROM main.sqlite_master WHERE type = 'trigger' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			rs.Close()
			return nil, err
		}
		names = append(names, n)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	for _, n := range names {
		if _, err := tx.ExecContext(ctx, `DROP TRIGGER IF EXISTS main."`+strings.ReplaceAll(n, `"`, `""`)+`"`); err != nil {
			return nil, fmt.Errorf("suppression du déclencheur « %s » : %w", n, err)
		}
	}
	return names, nil
}
