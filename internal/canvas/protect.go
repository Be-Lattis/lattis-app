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
// feuille).
//
// La protection vit dans _lattis_protection (sheet = 0 : le classeur),
// hors du journal : la poser ou l'ôter ne s'annule pas (comme Excel). Pour
// qu'une annulation ou un rétablissement d'une action antérieure ne la
// contourne pas, le bus de commandes compare une empreinte du contenu des
// feuilles protégées (et de la liste des onglets si le classeur l'est)
// avant et après : si elle change, l'annulation est refusée.
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
// structure et le contenu de toutes ses feuilles.
type ErrProtected struct{ Sheet string }

func (e ErrProtected) Error() string {
	if e.Sheet == "" {
		return "Le classeur est protégé : ôtez la protection (Révision › Ôter la protection du classeur) pour le modifier"
	}
	return fmt.Sprintf("La feuille « %s » est protégée : ôtez la protection (Révision › Ôter la protection) pour la modifier", e.Sheet)
}

// ErrPassword signale un mot de passe de protection incorrect.
var ErrPassword = errors.New("mot de passe incorrect")

// workbookScope désigne la structure du classeur dans _lattis_protection.
const workbookScope = 0

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

// guard refuse l'action si la feuille est protégée.
func (t *tx) guard(sheet int64) error {
	if err := t.guardWorkbook(); err != nil {
		return err
	}
	var name string
	err := t.row(`SELECT s.name FROM _lattis_protection p JOIN _lattis_sheets s ON s.id = p.sheet WHERE p.sheet = ?`, sheet).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrProtected{name}
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

// editTable lit un tableau à modifier : refusé si sa feuille est protégée.
func (t *tx) editTable(name string) (Table, error) {
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
	return m.setProtection(ctx, sheet, true, pw)
}

// UnprotectSheet ôte la protection d'une feuille ; pw doit être le mot de
// passe donné en la protégeant, s'il y en avait un.
func (m *Model) UnprotectSheet(ctx context.Context, sheet int64, pw string) error {
	return m.setProtection(ctx, sheet, false, pw)
}

// ProtectWorkbook protège la structure du classeur (onglets).
func (m *Model) ProtectWorkbook(ctx context.Context, pw string) error {
	return m.setProtection(ctx, workbookScope, true, pw)
}

// UnprotectWorkbook ôte la protection de la structure du classeur.
func (m *Model) UnprotectWorkbook(ctx context.Context, pw string) error {
	return m.setProtection(ctx, workbookScope, false, pw)
}

// setProtection écrit hors du journal (rien à annuler).
func (m *Model) setProtection(ctx context.Context, scope int64, on bool, pw string) error {
	h, err := hashPassword(pw)
	if err != nil {
		return err
	}
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
		err := sqlTx.QueryRowContext(ctx, `SELECT hash FROM _lattis_protection WHERE sheet = ?`, scope).Scan(&stored)
		protected := err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		what := "La structure du classeur"
		if scope != workbookScope {
			what = fmt.Sprintf("La feuille « %s »", name)
		}
		if on {
			if protected {
				return fmt.Errorf("%s est déjà protégée", what)
			}
			_, err := sqlTx.ExecContext(ctx, `INSERT INTO _lattis_protection (sheet, hash) VALUES (?, ?)`, scope, h)
			return err
		}
		if !protected {
			return fmt.Errorf("%s n'est pas protégée", what)
		}
		if !checkPassword(stored, pw) {
			return ErrPassword
		}
		_, err = sqlTx.ExecContext(ctx, `DELETE FROM _lattis_protection WHERE sheet = ?`, scope)
		return err
	})
	if err == nil {
		m.protMu.Lock()
		m.protGen++
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
	// Feuilles protégées : identifiant → protégée avec un mot de passe.
	Sheets map[int64]bool `json:"sheets"`
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
	p := Protection{Sheets: map[int64]bool{}}
	rs, err := t.query(`SELECT sheet, hash <> '' FROM _lattis_protection`)
	if err != nil {
		return p, err
	}
	defer rs.Close()
	for rs.Next() {
		var id int64
		var pw bool
		if err := rs.Scan(&id, &pw); err != nil {
			return p, err
		}
		if id == workbookScope {
			p.Workbook, p.WorkbookPassword = true, pw
		} else {
			p.Sheets[id] = pw
		}
	}
	return p, rs.Err()
}

// --- Annulation : empreinte de ce qui est protégé ----------------------------

// historyGuard encadre une annulation ou un rétablissement (command.Bus.Guard) :
// l'empreinte de ce qui est protégé ne doit pas changer.
func (m *Model) historyGuard(ctx context.Context, sqlTx *sql.Tx) (func() error, error) {
	t := &tx{Tx: &command.Tx{Tx: sqlTx, Ctx: ctx}, m: m}
	before, names, err := t.prints()
	if err != nil || len(before) == 0 {
		return func() error { return nil }, err
	}
	return func() error {
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
		return nil
	}, nil
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
	if !p.Workbook && len(p.Sheets) == 0 {
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
	if len(p.Sheets) == 0 {
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
		if n == "_lattis_protection" || n == "_lattis_journal" || n == "_lattis_sheets" || strings.HasPrefix(n, sqlTrash) {
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
	for id := range p.Sheets {
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
