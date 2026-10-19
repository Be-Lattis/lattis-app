package canvas

// Protection des feuilles et de la structure du classeur, comme Excel
// (Révision › Protéger la feuille, Protéger le classeur).
//
// Une feuille protégée refuse toute modification de son contenu : saisie,
// collage, effacement, mise en forme, largeurs et hauteurs, lignes et
// colonnes, structure et tri des tableaux, tableaux croisés, graphiques et
// boutons, cellules nommées. Restent permis la lecture, la sélection, la
// copie, les filtres (affichage), le quadrillage et les en-têtes A1, et les
// requêtes SQL de lecture. La structure du classeur protégée refuse
// d'ajouter, d'insérer, de supprimer, de renommer ou de déplacer des
// feuilles (y compris par un import ou un tableau croisé, qui créent une
// feuille). Comme dans Excel, elle laisse modifiable le contenu des feuilles
// non protégées, sauf si l'on choisit en la posant de verrouiller aussi le
// contenu de toutes les feuilles (Révision › Protéger le classeur, case
// « Verrouiller aussi le contenu de toutes les feuilles ») : chaque feuille
// est alors traitée comme protégée, et ôter la protection du classeur ôte ce
// verrou, et seulement lui (les feuilles protégées pour elles-mêmes le
// restent).
//
// La protection vit dans _lattis_protection (sheet = 0 : le classeur ;
// sheet = -1 : le verrou du contenu de toutes les feuilles posé avec lui,
// sans empreinte propre : c'est le mot de passe du classeur qui l'ôte),
// hors du journal : la poser ou l'ôter ne s'annule pas (comme Excel). Pour
// qu'une annulation ou un rétablissement d'une action antérieure ne la
// contourne pas, le bus de commandes compare une empreinte du contenu des
// feuilles protégées (et de la liste des onglets si le classeur l'est)
// avant et après : si elle change, l'annulation est refusée.
//
// La mise en forme d'une feuille peut aussi être protégée seule (Révision ›
// Protéger la mise en forme, _lattis_format_protection) : la saisie, le
// collage, l'effacement du contenu, les lignes et colonnes, le tri, les
// filtres, les tableaux et les objets restent permis ; sont refusés les
// styles (police, couleurs, remplissage, bordures, alignement, renvoi à la
// ligne, format de nombre, décimales), le pinceau, « Effacer les formats »
// et Supprimer (qui efface aussi la mise en forme), la mise en forme
// conditionnelle, les largeurs et hauteurs, la fusion. Notes, validation et
// masquage des lignes ne sont pas de la mise en forme (cellformat.go) et
// restent permis. Un collage copié dans Lattis colle les valeurs seules
// (Transfer) ; un déplacement d'une feuille à une autre est refusé (la mise
// en forme partirait avec les cellules). La protection de la feuille, plus
// large, l'emporte. Contre l'annulation, l'empreinte de la mise en forme
// (styles sans notes ni validation, largeurs, hauteurs, règles) ne doit pas
// changer, pour les seules commandes antérieures à la protection : celles
// faites depuis ont été gardées par elle (voir historyGuard).
//
// Le mot de passe, facultatif, n'est jamais stocké : seulement une empreinte
// PBKDF2-HMAC-SHA256 salée (sel aléatoire de 16 octets).

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
)

// ErrProtected refuse une modification d'une feuille protégée (Sheet) ou du
// classeur protégé (Sheet vide) : un classeur protégé verrouille sa
// structure, et le contenu de toutes ses feuilles si on l'a choisi.
type ErrProtected struct{ Sheet string }

func (e ErrProtected) Error() string {
	if e.Sheet == "" {
		return "Le classeur est protégé : ôtez la protection (Révision › Ôter la protection du classeur) pour le modifier"
	}
	return fmt.Sprintf("La feuille « %s » est protégée : ôtez la protection (Révision › Ôter la protection de la feuille) pour la modifier", e.Sheet)
}

// ErrFormatProtected refuse un changement de mise en forme d'une feuille
// dont la mise en forme est protégée.
type ErrFormatProtected struct{ Sheet string }

func (e ErrFormatProtected) Error() string {
	return fmt.Sprintf("La mise en forme de la feuille « %s » est protégée : ôtez la protection (Révision › Ôter la protection de la mise en forme) pour la modifier", e.Sheet)
}

// ErrPassword signale un mot de passe de protection incorrect.
var ErrPassword = errors.New("mot de passe incorrect")

// workbookScope désigne la structure du classeur dans _lattis_protection.
const workbookScope = 0

// sheetsScope désigne, dans _lattis_protection, le verrou du contenu de
// toutes les feuilles posé avec la protection du classeur.
const sheetsScope = -1

const pbkdf2Iter = 120_000

// hashPassword rend l'empreinte stockée d'un mot de passe ("" sans mot de
// passe) : pbkdf2-sha256$itérations$sel$clé (base64).
func hashPassword(pw string) (string, error) {
	if pw == "" {
		return "", nil
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, pw, salt, pbkdf2Iter, 32)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", pbkdf2Iter, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// checkPassword vérifie pw contre l'empreinte stockée.
func checkPassword(stored, pw string) bool {
	if stored == "" {
		return true
	}
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err1 := enc.DecodeString(parts[2])
	want, err2 := enc.DecodeString(parts[3])
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pw, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(got, want) == 1
}

// guard refuse l'action si la feuille est protégée, ou si le classeur est
// protégé avec le contenu de toutes ses feuilles. La seule protection de la
// structure du classeur laisse le contenu modifiable, comme Excel.
func (t *tx) guard(sheet int64) error {
	var name string
	err := t.row(`SELECT s.name FROM _lattis_protection p JOIN _lattis_sheets s ON s.id = p.sheet WHERE p.sheet = ?`, sheet).Scan(&name)
	if err == nil {
		return ErrProtected{name}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var k int
	if err := t.row(`SELECT count(*) FROM _lattis_protection WHERE sheet = ?`, sheetsScope).Scan(&k); err != nil {
		return err
	}
	if k > 0 {
		return ErrProtected{}
	}
	return nil
}

// guardFormat refuse un changement de mise en forme : feuille protégée
// (guard), ou dont la mise en forme est protégée.
func (t *tx) guardFormat(sheet int64) error {
	if err := t.guard(sheet); err != nil {
		return err
	}
	var name string
	err := t.row(`SELECT s.name FROM _lattis_format_protection p JOIN _lattis_sheets s ON s.id = p.sheet WHERE p.sheet = ?`, sheet).Scan(&name)
	if err == nil {
		return ErrFormatProtected{name}
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

// notFormat : propriétés de style qui ne sont pas de la mise en forme
// (notes, masquage, validation : cellformat.go).
var notFormat = map[string]bool{"nt": true, "np": true, "hd": true, "dv": true}

// isFormat dit si un correctif de style change la mise en forme.
func isFormat(patch Style) bool {
	for k := range patch {
		if !notFormat[k] {
			return true
		}
	}
	return false
}

// guardStyle garde un correctif de style : guardFormat s'il change la mise
// en forme, guard sinon (note, validation).
func (t *tx) guardStyle(sheet int64, patch Style) error {
	if isFormat(patch) {
		return t.guardFormat(sheet)
	}
	return t.guard(sheet)
}

// guarded exécute fn si la feuille n'est pas protégée.
func (t *tx) guarded(sheet int64, fn func() error) error {
	if err := t.guard(sheet); err != nil {
		return err
	}
	return fn()
}

// guardWorkbook refuse l'action si le classeur est protégé.
func (t *tx) guardWorkbook() error {
	var k int
	if err := t.row(`SELECT count(*) FROM _lattis_protection WHERE sheet = ?`, workbookScope).Scan(&k); err != nil {
		return err
	}
	if k > 0 {
		return ErrProtected{}
	}
	return nil
}

// editTable lit un tableau à modifier : refusé si sa feuille est protégée,
// ou si c'est un tableau publié en lecture seule (classeur partagé).
func (t *tx) editTable(name string) (Table, error) {
	tb, err := t.viewTable(name)
	if err == nil {
		err = t.lockTable(tb.ID)
	}
	return tb, err
}

// lockTable refuse la modification d'un tableau publié en lecture seule.
func (t *tx) lockTable(id int64) error {
	if t.m != nil && t.m.sharing != nil {
		return t.m.sharing.TableLock(id)
	}
	return nil
}

// viewTable lit un tableau dont on change la présentation locale (tri,
// mise en forme conditionnelle, clé proposée) : refusé si sa feuille est
// protégée, permis sur un tableau publié en lecture seule (rien n'en est
// renvoyé au serveur, §5.4).
func (t *tx) viewTable(name string) (Table, error) {
	tb, err := t.tableByName(name)
	if err == nil {
		err = t.guard(tb.Sheet)
	}
	return tb, err
}

// guardObject refuse l'action si l'objet id de la table de métadonnées
// (graphique, bouton, tableau croisé) est sur une feuille protégée.
func (t *tx) guardObject(table string, id int64) error {
	var sheet int64
	err := t.row(`SELECT sheet FROM `+table+` WHERE id = ?`, id).Scan(&sheet)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // l'action signalera elle-même l'objet introuvable
	}
	if err != nil {
		return err
	}
	return t.guard(sheet)
}

// ProtectSheet protège une feuille (pw vide : sans mot de passe).
func (m *Model) ProtectSheet(ctx context.Context, sheet int64, pw string) error {
	return m.setProtection(ctx, protTable, sheet, true, pw, false)
}

// UnprotectSheet ôte la protection d'une feuille ; pw doit être le mot de
// passe donné en la protégeant, s'il y en avait un.
func (m *Model) UnprotectSheet(ctx context.Context, sheet int64, pw string) error {
	return m.setProtection(ctx, protTable, sheet, false, pw, false)
}

// ProtectFormat protège la mise en forme d'une feuille (pw vide : sans mot
// de passe) : la saisie reste permise.
func (m *Model) ProtectFormat(ctx context.Context, sheet int64, pw string) error {
	return m.setProtection(ctx, formatTable, sheet, true, pw, false)
}

// UnprotectFormat ôte la protection de la mise en forme d'une feuille.
func (m *Model) UnprotectFormat(ctx context.Context, sheet int64, pw string) error {
	return m.setProtection(ctx, formatTable, sheet, false, pw, false)
}

// ProtectWorkbook protège la structure du classeur (onglets) ; sheets
// verrouille aussi le contenu de toutes les feuilles, jusqu'à ce que la
// protection du classeur soit ôtée.
func (m *Model) ProtectWorkbook(ctx context.Context, pw string, sheets bool) error {
	return m.setProtection(ctx, protTable, workbookScope, true, pw, sheets)
}

// UnprotectWorkbook ôte la protection de la structure du classeur, et le
// verrou du contenu des feuilles posé avec elle.
func (m *Model) UnprotectWorkbook(ctx context.Context, pw string) error {
	return m.setProtection(ctx, protTable, workbookScope, false, pw, false)
}

// Tables des protections : feuilles et classeur, mise en forme des feuilles.
const (
	protTable   = `_lattis_protection`
	formatTable = `_lattis_format_protection`
)

// setProtection écrit hors du journal (rien à annuler) dans table. sheets
// (classeur seulement) pose aussi le verrou du contenu de toutes les
// feuilles.
func (m *Model) setProtection(ctx context.Context, table string, scope int64, on bool, pw string, sheets bool) error {
	h, err := hashPassword(pw)
	if err != nil {
		return err
	}
	var top int64 // dernière commande du journal (protection de la mise en forme)
	err = m.db.Write(ctx, func(sqlTx *sql.Tx) error {
		name := ""
		if scope != workbookScope {
			if err := sqlTx.QueryRowContext(ctx, `SELECT name FROM _lattis_sheets WHERE id = ?`, scope).Scan(&name); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("%w : feuille introuvable", ErrSheet)
				}
				return err
			}
		}
		var stored string
		err := sqlTx.QueryRowContext(ctx, `SELECT hash FROM `+table+` WHERE sheet = ?`, scope).Scan(&stored)
		protected := err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		what := "La structure du classeur"
		switch {
		case table == formatTable:
			what = fmt.Sprintf("La mise en forme de la feuille « %s »", name)
		case scope != workbookScope:
			what = fmt.Sprintf("La feuille « %s »", name)
		}
		if on {
			if protected {
				return fmt.Errorf("%s est déjà protégée", what)
			}
			if _, err := sqlTx.ExecContext(ctx, `INSERT INTO `+table+` (sheet, hash) VALUES (?, ?)`, scope, h); err != nil {
				return err
			}
			if table == formatTable {
				return sqlTx.QueryRowContext(ctx, `SELECT coalesce(max(seq), 0) FROM _lattis_journal`).Scan(&top)
			}
			if sheets && scope == workbookScope {
				_, err := sqlTx.ExecContext(ctx, `INSERT OR REPLACE INTO _lattis_protection (sheet, hash) VALUES (?, '')`, sheetsScope)
				return err
			}
			return nil
		}
		if !protected {
			return fmt.Errorf("%s n'est pas protégée", what)
		}
		if !checkPassword(stored, pw) {
			return ErrPassword
		}
		if table == protTable && scope == workbookScope {
			_, err = sqlTx.ExecContext(ctx, `DELETE FROM _lattis_protection WHERE sheet IN (?, ?)`, workbookScope, sheetsScope)
		} else {
			_, err = sqlTx.ExecContext(ctx, `DELETE FROM `+table+` WHERE sheet = ?`, scope)
		}
		return err
	})
	if err == nil {
		m.protMu.Lock()
		m.protGen++
		if table == formatTable {
			if on {
				m.fmtSince[scope] = top
			} else {
				delete(m.fmtSince, scope)
			}
		}
		m.protMu.Unlock()
	}
	return err
}

// ProtectionChanges compte les changements de protection faits depuis
// l'ouverture : hors du journal, ils rendent le classeur « modifié »
// (voir doc.Document.Dirty).
func (m *Model) ProtectionChanges() int64 {
	m.protMu.Lock()
	defer m.protMu.Unlock()
	return m.protGen
}

// Protection décrit ce qui est protégé dans le classeur.
type Protection struct {
	Workbook         bool `json:"workbook"`         // structure du classeur protégée
	WorkbookPassword bool `json:"workbookPassword"` // … avec un mot de passe
	// Contenu de toutes les feuilles verrouillé avec la structure du classeur.
	WorkbookSheets bool `json:"workbookSheets"`
	// Feuilles protégées : identifiant → protégée avec un mot de passe.
	Sheets map[int64]bool `json:"sheets"`
	// Feuilles dont la mise en forme est protégée : identifiant → avec un
	// mot de passe.
	Formats map[int64]bool `json:"formats"`
}

// Protection lit l'état des protections.
func (m *Model) Protection(ctx context.Context) (Protection, error) {
	var p Protection
	err := m.read(ctx, func(t *tx) (err error) {
		p, err = t.protection()
		return err
	})
	return p, err
}

func (t *tx) protection() (Protection, error) {
	p := Protection{Sheets: map[int64]bool{}, Formats: map[int64]bool{}}
	rs, err := t.query(`SELECT 0, sheet, hash <> '' FROM _lattis_protection UNION ALL SELECT 1, sheet, hash <> '' FROM _lattis_format_protection`)
	if err != nil {
		return p, err
	}
	defer rs.Close()
	for rs.Next() {
		var format bool
		var id int64
		var pw bool
		if err := rs.Scan(&format, &id, &pw); err != nil {
			return p, err
		}
		switch {
		case format:
			p.Formats[id] = pw
		case id == workbookScope:
			p.Workbook, p.WorkbookPassword = true, pw
		case id == sheetsScope:
			p.WorkbookSheets = true
		default:
			p.Sheets[id] = pw
		}
	}
	return p, rs.Err()
}

// --- Annulation : empreinte de ce qui est protégé ----------------------------

// syncState : tables de la synchronisation avec un Serveur Lattis, hors de
// l'empreinte des feuilles protégées.
var syncState = map[string]bool{
	"_lattis_remote_tables": true,
	"_lattis_row_ids":       true,
	"_lattis_outbox":        true,
	"_lattis_conflicts":     true,
	"_lattis_capabilities":  true,
	"_lattis_data_rows":     true,
	"_lattis_data_sync":     true,
}

// historyGuard encadre l'annulation ou le rétablissement de la commande seq
// (command.Bus.Guard) : l'empreinte de ce qui est protégé ne doit pas
// changer ; celle de la mise en forme protégée, seulement si la commande
// est antérieure à sa protection.
func (m *Model) historyGuard(ctx context.Context, sqlTx *sql.Tx, seq int64) (func() error, error) {
	t := &tx{Tx: &command.Tx{Tx: sqlTx, Ctx: ctx}, m: m}
	before, names, err := t.prints()
	if err != nil {
		return nil, err
	}
	fmts, err := m.formatGuarded(t, seq)
	if err != nil {
		return nil, err
	}
	fbefore := map[int64]string{}
	for _, id := range fmts {
		if fbefore[id], err = t.formatPrint(id); err != nil {
			return nil, err
		}
	}
	if len(before) == 0 && len(fbefore) == 0 {
		return func() error { return nil }, nil
	}
	return func() error {
		if len(before) > 0 {
			after, _, err := t.prints()
			if err != nil {
				return err
			}
			if before[workbookScope] != after[workbookScope] {
				return ErrProtected{}
			}
			for id, h := range before {
				if id != workbookScope && after[id] != h {
					return ErrProtected{names[id]}
				}
			}
		}
		for id, h := range fbefore {
			after, err := t.formatPrint(id)
			if err != nil {
				return err
			}
			if after != h {
				var name string
				if err := t.row(`SELECT coalesce(max(name), '') FROM _lattis_sheets WHERE id = ?`, id).Scan(&name); err != nil {
					return err
				}
				return ErrFormatProtected{name}
			}
		}
		return nil
	}, nil
}

// formatGuarded rend les feuilles à mise en forme protégée depuis une
// commande postérieure ou égale à seq : rejouer seq peut contourner leur
// protection. Une commande faite depuis la protection a été gardée par
// elle (guardFormat) : son annulation ne rend que ce qu'elle a changé.
func (m *Model) formatGuarded(t *tx, seq int64) ([]int64, error) {
	rs, err := t.query(`SELECT sheet FROM _lattis_format_protection ORDER BY sheet`)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	m.protMu.Lock()
	defer m.protMu.Unlock()
	out := ids[:0]
	for _, id := range ids {
		since, ok := m.fmtSince[id]
		if !ok {
			since = m.fmtBase // protégée à l'ouverture
		}
		if seq <= since {
			out = append(out, id)
		}
	}
	return out, nil
}

// formatPrint calcule l'empreinte de la mise en forme d'une feuille :
// styles (sans notes, masquage ni validation), largeurs, hauteurs, règles
// de mise en forme conditionnelle.
func (t *tx) formatPrint(sheet int64) (string, error) {
	h := sha256.New()
	var exists int
	if err := t.row(`SELECT count(*) FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&exists); err != nil {
		return "", err
	}
	fmt.Fprintf(h, "exists:%d\x1e", exists)
	tbls := `(SELECT id FROM _lattis_tables WHERE sheet = ?)`
	styled := []string{
		`SELECT 'c', row, col, style FROM _lattis_cell_styles WHERE sheet = ? ORDER BY row, col`,
		`SELECT 'r', row, 0, style FROM _lattis_row_styles WHERE sheet = ? ORDER BY row`,
		`SELECT 'k', col, 0, style FROM _lattis_col_styles WHERE sheet = ? ORDER BY col`,
		`SELECT 't' || tbl, rid, col, style FROM _lattis_styles WHERE tbl IN ` + tbls + ` ORDER BY tbl, rid, col`,
	}
	for _, q := range styled {
		rs, err := t.query(q, sheet)
		if err != nil {
			return "", err
		}
		for rs.Next() {
			var at string
			var a, b int64
			var s string
			if err := rs.Scan(&at, &a, &b, &s); err != nil {
				rs.Close()
				return "", err
			}
			st := parseStyle(s)
			for k := range notFormat {
				delete(st, k)
			}
			if len(st) == 0 {
				continue
			}
			enc, _ := encodeStyle(st)
			fmt.Fprintf(h, "%s:%d:%d:%s\x1e", at, a, b, enc)
		}
		err = rs.Err()
		rs.Close()
		if err != nil {
			return "", err
		}
	}
	for _, q := range []string{
		`SELECT col, width FROM _lattis_col_widths WHERE sheet = ? ORDER BY col`,
		`SELECT row, height FROM _lattis_row_heights WHERE sheet = ? ORDER BY row`,
		`SELECT row, rules FROM _lattis_row_rules WHERE sheet = ? ORDER BY row`,
		`SELECT col, rules FROM _lattis_col_rules WHERE sheet = ? ORDER BY col`,
		`SELECT tbl, id, rules FROM _lattis_columns WHERE tbl IN ` + tbls + ` AND coalesce(rules, '') <> '' ORDER BY tbl, id`,
	} {
		h.Write([]byte{0x1d})
		if err := t.hashRows(h, q, sheet); err != nil {
			return "", err
		}
	}
	return string(h.Sum(nil)), nil
}

// prints calcule l'empreinte de chaque portée protégée : la liste des onglets
// pour le classeur, le contenu pour une feuille (métadonnées repérées par
// feuille ou par tableau, données des tableaux ; le filtre, simple
// affichage, n'en fait pas partie).
func (t *tx) prints() (map[int64]string, map[int64]string, error) {
	p, err := t.protection()
	if err != nil {
		return nil, nil, err
	}
	out, names := map[int64]string{}, map[int64]string{}
	if !p.Workbook && !p.WorkbookSheets && len(p.Sheets) == 0 {
		return out, names, nil
	}
	h := sha256.New()
	sum := func() string { s := string(h.Sum(nil)); h.Reset(); return s }
	if p.Workbook {
		if err := t.hashRows(h, `SELECT id, name, pos FROM _lattis_sheets ORDER BY id`); err != nil {
			return nil, nil, err
		}
		out[workbookScope] = sum()
	}
	// Feuilles dont le contenu est verrouillé : protégées elles-mêmes, ou
	// toutes si le classeur verrouille leur contenu.
	locked := map[int64]bool{}
	for id := range p.Sheets {
		locked[id] = true
	}
	if p.WorkbookSheets {
		rs, err := t.query(`SELECT id FROM _lattis_sheets`)
		if err != nil {
			return nil, nil, err
		}
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return nil, nil, err
			}
			locked[id] = true
		}
		err = rs.Err()
		rs.Close()
		if err != nil {
			return nil, nil, err
		}
	}
	if len(locked) == 0 {
		return out, names, nil
	}
	// Tables de métadonnées et leurs colonnes (hors filtre).
	type metaTable struct {
		name, cols string
		bySheet    bool
	}
	var metas []metaTable
	rs, err := t.query(`SELECT name FROM main.sqlite_schema WHERE type = 'table' AND name LIKE '\_lattis\_%' ESCAPE '\' ORDER BY name`)
	if err != nil {
		return nil, nil, err
	}
	var tnames []string
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			rs.Close()
			return nil, nil, err
		}
		tnames = append(tnames, n)
	}
	rs.Close()
	for _, n := range tnames {
		// L'état de la synchronisation (identifiants partagés des lignes,
		// droits, conflits) n'est pas du contenu : il change hors du journal.
		if n == protTable || n == formatTable || n == "_lattis_journal" || n == "_lattis_sheets" || strings.HasPrefix(n, sqlTrash) || syncState[n] {
			continue
		}
		crs, err := t.query(`SELECT name FROM pragma_table_info(?)`, n)
		if err != nil {
			return nil, nil, err
		}
		var cols []string
		sheet, tbl := false, false
		for crs.Next() {
			var c string
			if err := crs.Scan(&c); err != nil {
				crs.Close()
				return nil, nil, err
			}
			switch c {
			case "sheet":
				sheet = true
			case "tbl":
				tbl = true
			}
			if !(n == "_lattis_tables" && c == "filter") {
				cols = append(cols, `"`+c+`"`)
			}
		}
		crs.Close()
		if sheet || tbl {
			metas = append(metas, metaTable{n, strings.Join(cols, ", "), sheet})
		}
	}
	for id := range locked {
		var exists int
		var name string
		if err := t.row(`SELECT count(*), coalesce(max(name), '') FROM _lattis_sheets WHERE id = ?`, id).Scan(&exists, &name); err != nil {
			return nil, nil, err
		}
		names[id] = name
		fmt.Fprintf(h, "exists:%d\x1e", exists)
		for _, mt := range metas {
			where := `sheet = ?`
			if !mt.bySheet {
				where = `tbl IN (SELECT id FROM _lattis_tables WHERE sheet = ?)`
			}
			fmt.Fprintf(h, "%s\x1d", mt.name)
			if err := t.hashRows(h, `SELECT `+mt.cols+` FROM `+mt.name+` WHERE `+where+` ORDER BY `+mt.cols, id); err != nil {
				return nil, nil, err
			}
		}
		tables, err := t.query(`SELECT id FROM _lattis_tables WHERE sheet = ? ORDER BY id`, id)
		if err != nil {
			return nil, nil, err
		}
		var ids []int64
		for tables.Next() {
			var tid int64
			if err := tables.Scan(&tid); err != nil {
				tables.Close()
				return nil, nil, err
			}
			ids = append(ids, tid)
		}
		tables.Close()
		for _, tid := range ids {
			fmt.Fprintf(h, "%s\x1d", tableSQL(tid))
			if err := t.hashRows(h, `SELECT * FROM `+tableSQL(tid)+` ORDER BY id`); err != nil {
				return nil, nil, err
			}
		}
		out[id] = sum()
	}
	return out, names, nil
}

// hashRows ajoute à h toutes les valeurs (et leurs types) d'une requête.
func (t *tx) hashRows(h hash.Hash, q string, args ...any) error {
	rs, err := t.query(q, args...)
	if err != nil {
		return err
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return err
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rs.Next() {
		if err := rs.Scan(ptrs...); err != nil {
			return err
		}
		for _, v := range vals {
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			fmt.Fprintf(h, "%T:%v\x1f", v, v)
		}
		h.Write([]byte{0x1e})
	}
	return rs.Err()
}
