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
// sheet = -1 : le verrou du contenu de toutes les feuilles posé avec lui ;
// sheet = -2 : celui de la structure de toutes les feuilles, qui les traite
// comme si chacune avait Protéger la structure ; ces deux verrous n'ont pas
// d'empreinte propre : c'est le mot de passe du classeur qui les ôte),
// hors du journal : la poser ou l'ôter ne s'annule pas (comme Excel). Pour
// qu'une annulation ou un rétablissement d'une action antérieure ne la
// contourne pas, le bus de commandes compare une empreinte du contenu des
// feuilles protégées (et de la liste des onglets si le classeur l'est)
// avant et après : si elle change, l'annulation est refusée.
//
// La structure d'une feuille peut aussi être protégée (Révision › Protéger
// la structure, table _lattis_format_protection, nommée ainsi au format 35
// pour une première version qui ne verrouillait que la mise en forme) :
// tout est refusé (guard) sauf la saisie dans les tableaux, gardée par
// guardData ou guardCells : valeurs des cellules de données (saisie,
// collage de valeurs, Suppr), lignes ajoutées sous un tableau ou insérées,
// lignes supprimées, tri et filtre, saisie par les scripts (Editor). Sont
// refusés les cellules libres, la mise en forme, les colonnes et en-têtes,
// les tableaux eux-mêmes, les objets, les noms, les règles, la validation,
// les lignes et colonnes de la feuille, la mise en page et l'onglet (nom,
// couleur, place, suppression, onglet à choix). Un collage copié dans
// Lattis colle les valeurs seules (Transfer). Contre l'annulation,
// l'empreinte de la feuille hors données des tableaux (structurePrint) ne
// doit pas changer, pour les seules commandes antérieures à la protection :
// celles faites depuis ont été gardées par elle (voir historyGuard).
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

// ErrStructureProtected refuse une modification d'une feuille dont la
// structure est protégée, autre que la saisie dans ses tableaux ; Book :
// par la protection du classeur (structure de toutes les feuilles).
type ErrStructureProtected struct {
	Sheet string
	Book  bool
	// Shared : refus du partage (classeur collaboratif ouvert par un
	// éditeur ou un lecteur : LayoutLocker).
	Shared error
}

// Unwrap rend le refus du partage (errors.Is).
func (e ErrStructureProtected) Unwrap() error { return e.Shared }

func (e ErrStructureProtected) Error() string {
	if e.Shared != nil {
		return e.Shared.Error()
	}
	if e.Book {
		return fmt.Sprintf("La structure des feuilles du classeur est protégée : seule la saisie dans les tableaux est permise sur « %s » (Révision › Ôter la protection du classeur)", e.Sheet)
	}
	return fmt.Sprintf("La structure de la feuille « %s » est protégée : seule la saisie dans les tableaux est permise (Révision › Ôter la protection de la structure)", e.Sheet)
}

// ErrPassword signale un mot de passe de protection incorrect.
var ErrPassword = errors.New("mot de passe incorrect")

// workbookScope désigne la structure du classeur dans _lattis_protection.
const workbookScope = 0

// sheetsScope désigne, dans _lattis_protection, le verrou du contenu de
// toutes les feuilles posé avec la protection du classeur.
const sheetsScope = -1

// structScope désigne, dans _lattis_protection, le verrou de la structure
// de toutes les feuilles posé avec la protection du classeur (seule la
// saisie dans les tableaux reste permise) ; sans changement de format.
const structScope = -2

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
// protégé avec le contenu de toutes ses feuilles, ou si la structure de la
// feuille l'est. La seule protection de la structure du classeur laisse le
// contenu modifiable, comme Excel.
func (t *tx) guard(sheet int64) error {
	if err := t.guardData(sheet); err != nil {
		return err
	}
	return t.guardStructure(sheet)
}

// guardStructure refuse l'action si la structure de la feuille est protégée.
func (t *tx) guardStructure(sheet int64) error {
	e, err := t.structureLock(sheet)
	if err == nil && e != nil {
		return *e
	}
	return err
}

// structureLock rend le refus d'une feuille dont la structure est protégée,
// par elle-même ou par le classeur (nil : elle ne l'est pas).
func (t *tx) structureLock(sheet int64) (*ErrStructureProtected, error) {
	if err := t.layoutLock(); err != nil {
		var name string
		_ = t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&name)
		return &ErrStructureProtected{Sheet: name, Book: true, Shared: err}, nil
	}
	var name string
	var own, book bool
	err := t.row(`SELECT s.name, EXISTS (SELECT 1 FROM `+structTable+` WHERE sheet = s.id),
		EXISTS (SELECT 1 FROM _lattis_protection WHERE sheet = ?) FROM _lattis_sheets s WHERE s.id = ?`, structScope, sheet).Scan(&name, &own, &book)
	if errors.Is(err, sql.ErrNoRows) || err == nil && !own && !book {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ErrStructureProtected{Sheet: name, Book: !own}, nil
}

// structureLocked dit si la structure de la feuille est protégée.
func (t *tx) structureLocked(sheet int64) (bool, error) {
	e, err := t.structureLock(sheet)
	return e != nil, err
}

// guardCells garde la saisie dans la zone z : sous la protection de la
// structure, seulement dans les données d'un tableau (below : ou juste
// dessous, ce qui lui ajoute des lignes). La transaction ne change alors
// plus la forme des tableaux (dataOnly).
func (t *tx) guardCells(sheet int64, z Rect, below bool) error {
	if err := t.guardData(sheet); err != nil {
		return err
	}
	lock, err := t.structureLock(sheet)
	if err != nil || lock == nil {
		return err
	}
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	for _, tb := range tables {
		if z.C0 >= tb.Col && z.C1 <= tb.Right() && z.R0 > tb.Row && (z.R1 <= tb.Bottom() || below && z.R0 <= tb.Bottom()+1) {
			t.dataOnly = true
			return nil
		}
	}
	return *lock
}

// guardData refuse l'action si la feuille est protégée (ou le contenu de
// toutes les feuilles) : la saisie dans les tableaux, permise sous la
// protection de la structure, n'est gardée que par elle.
func (t *tx) guardData(sheet int64) error {
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

// guarded exécute fn si la feuille n'est pas protégée.
func (t *tx) guarded(sheet int64, fn func() error) error {
	if err := t.guard(sheet); err != nil {
		return err
	}
	return fn()
}

// guardWorkbook refuse l'action si le classeur est protégé.
func (t *tx) guardWorkbook() error {
	if err := t.layoutLock(); err != nil {
		return err
	}
	var k int
	if err := t.row(`SELECT count(*) FROM _lattis_protection WHERE sheet = ?`, workbookScope).Scan(&k); err != nil {
		return err
	}
	if k > 0 {
		return ErrProtected{}
	}
	return nil
}

// LayoutLocker est implémenté par le partage (Sharing) d'un classeur
// collaboratif : seuls ses propriétaires et concepteurs en changent les
// cellules libres et la mise en page (décision 0018). Pour les autres, tout
// se passe comme sous la protection de la structure de toutes les feuilles
// (saisie dans les tableaux seulement, selon leurs droits).
type LayoutLocker interface {
	// LayoutLock : nil si la mise en page se modifie, sinon le refus.
	LayoutLock() error
}

// layoutLock rend le refus du partage (nil : permis).
func (t *tx) layoutLock() error {
	if t.m == nil || t.m.sharing == nil {
		return nil
	}
	if l, ok := t.m.sharing.(LayoutLocker); ok {
		return l.LayoutLock()
	}
	return nil
}

// ErrScriptsShared : scripts d'un classeur collaboratif modifiés par qui
// n'en est ni propriétaire ni concepteur (ils se partagent avec tous les
// membres ; audit du mode en ligne, constat 20).
var ErrScriptsShared = errors.New("Seuls les propriétaires et concepteurs modifient les scripts de ce classeur partagé")

// scriptsLock refuse de changer les scripts d'un classeur collaboratif à
// qui n'en est ni propriétaire ni concepteur (même règle que la mise en
// page, LayoutLocker).
func (t *tx) scriptsLock() error {
	if t.layoutLock() != nil {
		return ErrScriptsShared
	}
	return nil
}

// guardTab refuse de changer l'onglet d'une feuille (nom, place, couleur,
// masquage, suppression) si le classeur ou la structure de la feuille est
// protégé.
func (t *tx) guardTab(sheet int64) error {
	if err := t.guardWorkbook(); err != nil {
		return err
	}
	return t.guardStructure(sheet)
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

// dataTable lit un tableau dont on change les lignes (saisie, ajout ou
// suppression de lignes) : permis sous la protection de la structure.
func (t *tx) dataTable(name string) (Table, error) {
	tb, err := t.sortTable(name)
	if err == nil {
		err = t.lockTable(tb.ID)
	}
	return tb, err
}

// sortTable lit un tableau à trier (présentation locale, comme viewTable) :
// permis sous la protection de la structure.
func (t *tx) sortTable(name string) (Table, error) {
	tb, err := t.tableByName(name)
	if err == nil {
		err = t.guardData(tb.Sheet)
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
	return m.setProtection(ctx, protTable, sheet, true, pw, lockTabs)
}

// UnprotectSheet ôte la protection d'une feuille ; pw doit être le mot de
// passe donné en la protégeant, s'il y en avait un.
func (m *Model) UnprotectSheet(ctx context.Context, sheet int64, pw string) error {
	return m.setProtection(ctx, protTable, sheet, false, pw, lockTabs)
}

// ProtectStructure protège la structure d'une feuille (pw vide : sans mot
// de passe) : seule la saisie dans ses tableaux reste permise.
func (m *Model) ProtectStructure(ctx context.Context, sheet int64, pw string) error {
	return m.setProtection(ctx, structTable, sheet, true, pw, lockTabs)
}

// UnprotectStructure ôte la protection de la structure d'une feuille.
func (m *Model) UnprotectStructure(ctx context.Context, sheet int64, pw string) error {
	return m.setProtection(ctx, structTable, sheet, false, pw, lockTabs)
}

// ProtectWorkbook protège la structure du classeur (onglets) ; sheets
// verrouille aussi le contenu de toutes les feuilles, structure leur
// structure (saisie dans les tableaux seulement), jusqu'à ce que la
// protection du classeur soit ôtée.
func (m *Model) ProtectWorkbook(ctx context.Context, pw string, sheets, structure bool) error {
	lock := lockTabs
	switch {
	case sheets:
		lock = lockSheets
	case structure:
		lock = lockStructures
	}
	return m.setProtection(ctx, protTable, workbookScope, true, pw, lock)
}

// Ce que verrouille la protection du classeur, en plus des onglets.
const (
	lockTabs       = iota // onglets seulement
	lockSheets            // contenu de toutes les feuilles (sheet = -1)
	lockStructures        // structure de toutes les feuilles (sheet = -2)
)

// UnprotectWorkbook ôte la protection de la structure du classeur, et le
// verrou du contenu des feuilles posé avec elle.
func (m *Model) UnprotectWorkbook(ctx context.Context, pw string) error {
	return m.setProtection(ctx, protTable, workbookScope, false, pw, lockTabs)
}

// Tables des protections : feuilles et classeur, structure des feuilles
// (nom du format 35, quand elle ne gardait que la mise en forme).
const (
	protTable   = `_lattis_protection`
	structTable = `_lattis_format_protection`
)

// setProtection écrit hors du journal (rien à annuler) dans table. lock
// (classeur seulement) pose aussi le verrou du contenu ou de la structure
// de toutes les feuilles.
func (m *Model) setProtection(ctx context.Context, table string, scope int64, on bool, pw string, lock int) error {
	h, err := hashPassword(pw)
	if err != nil {
		return err
	}
	var top int64 // dernière commande du journal (protection de la structure)
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
		case table == structTable:
			what = fmt.Sprintf("La structure de la feuille « %s »", name)
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
			if table == structTable || lock == lockStructures {
				if err := sqlTx.QueryRowContext(ctx, `SELECT coalesce(max(seq), 0) FROM _lattis_journal`).Scan(&top); err != nil {
					return err
				}
			}
			if scope == workbookScope && lock != lockTabs {
				s := int64(sheetsScope)
				if lock == lockStructures {
					s = structScope
				}
				_, err := sqlTx.ExecContext(ctx, `INSERT OR REPLACE INTO _lattis_protection (sheet, hash) VALUES (?, '')`, s)
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
			_, err = sqlTx.ExecContext(ctx, `DELETE FROM _lattis_protection WHERE sheet IN (?, ?, ?)`, workbookScope, sheetsScope, structScope)
		} else {
			_, err = sqlTx.ExecContext(ctx, `DELETE FROM `+table+` WHERE sheet = ?`, scope)
		}
		return err
	})
	if err == nil {
		m.protMu.Lock()
		m.protGen++
		switch {
		case table == structTable && on:
			m.structSince[scope] = top
		case table == structTable:
			delete(m.structSince, scope)
		case scope == workbookScope && on && lock == lockStructures:
			m.structSince[structScope] = top
		case scope == workbookScope && !on:
			delete(m.structSince, structScope)
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
	// Structure de toutes les feuilles verrouillée avec celle du classeur.
	WorkbookStructure bool `json:"workbookStructure"`
	// Feuilles protégées : identifiant → protégée avec un mot de passe.
	Sheets map[int64]bool `json:"sheets"`
	// Feuilles dont la structure est protégée : identifiant → avec un mot de
	// passe.
	Structures map[int64]bool `json:"structures"`
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
	p := Protection{Sheets: map[int64]bool{}, Structures: map[int64]bool{}}
	rs, err := t.query(`SELECT 0, sheet, hash <> '' FROM _lattis_protection UNION ALL SELECT 1, sheet, hash <> '' FROM ` + structTable)
	if err != nil {
		return p, err
	}
	defer rs.Close()
	for rs.Next() {
		var structure bool
		var id int64
		var pw bool
		if err := rs.Scan(&structure, &id, &pw); err != nil {
			return p, err
		}
		switch {
		case structure:
			p.Structures[id] = pw
		case id == workbookScope:
			p.Workbook, p.WorkbookPassword = true, pw
		case id == sheetsScope:
			p.WorkbookSheets = true
		case id == structScope:
			p.WorkbookStructure = true
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
// changer ; celle d'une feuille à structure protégée (hors données des
// tableaux), seulement si la commande est antérieure à sa protection.
func (m *Model) historyGuard(ctx context.Context, sqlTx *sql.Tx, seq int64) (func() error, error) {
	t := &tx{Tx: &command.Tx{Tx: sqlTx, Ctx: ctx}, m: m}
	before, names, err := t.prints()
	if err != nil {
		return nil, err
	}
	structs, err := m.structureGuarded(t, seq)
	if err != nil {
		return nil, err
	}
	sbefore := map[int64]string{}
	if len(structs) > 0 {
		metas, err := t.metaTables()
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		for _, id := range structs {
			if _, err := t.sheetPrint(h, id, metas, false); err != nil {
				return nil, err
			}
			sbefore[id] = string(h.Sum(nil))
			h.Reset()
		}
	}
	if len(before) == 0 && len(sbefore) == 0 {
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
		if len(sbefore) == 0 {
			return nil
		}
		metas, err := t.metaTables()
		if err != nil {
			return err
		}
		h := sha256.New()
		for id, want := range sbefore {
			name, err := t.sheetPrint(h, id, metas, false)
			if err != nil {
				return err
			}
			if string(h.Sum(nil)) != want {
				if e, err := t.structureLock(id); err == nil && e != nil {
					return *e // par la feuille ou par le classeur
				}
				return ErrStructureProtected{Sheet: name}
			}
			h.Reset()
		}
		return nil
	}, nil
}

// structureGuarded rend les feuilles à structure protégée depuis une
// commande postérieure ou égale à seq : rejouer seq peut contourner leur
// protection. Une commande faite depuis la protection a été gardée par
// elle (guard) : son annulation ne rend que ce qu'elle a changé.
func (m *Model) structureGuarded(t *tx, seq int64) ([]int64, error) {
	// Feuilles protégées elles-mêmes (own), et toutes si le classeur
	// verrouille leur structure.
	p, err := t.protection()
	if err != nil {
		return nil, err
	}
	q := `SELECT sheet FROM ` + structTable + ` ORDER BY sheet`
	if p.WorkbookStructure {
		q = `SELECT id FROM _lattis_sheets ORDER BY id`
	}
	rs, err := t.query(q)
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
	sinceOf := func(id int64) int64 {
		if since, ok := m.structSince[id]; ok {
			return since
		}
		return m.structBase // protégée à l'ouverture
	}
	for _, id := range ids {
		since := int64(-1)
		if _, own := p.Structures[id]; own {
			since = sinceOf(id)
		}
		if p.WorkbookStructure {
			since = max(since, sinceOf(structScope))
		}
		if seq <= since {
			out = append(out, id)
		}
	}
	return out, nil
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
	metas, err := t.metaTables()
	if err != nil {
		return nil, nil, err
	}
	for id := range locked {
		if names[id], err = t.sheetPrint(h, id, metas, true); err != nil {
			return nil, nil, err
		}
		out[id] = sum()
	}
	return out, names, nil
}

// metaTable : table de métadonnées repérée par feuille (bySheet) ou par
// tableau, et ses colonnes (hors filtre).
type metaTable struct {
	name, cols string
	bySheet    bool
}

// metaTables liste les tables de métadonnées du contenu des feuilles.
func (t *tx) metaTables() ([]metaTable, error) {
	var metas []metaTable
	rs, err := t.query(`SELECT name FROM main.sqlite_schema WHERE type = 'table' AND name LIKE '\_lattis\_%' ESCAPE '\' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var tnames []string
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			rs.Close()
			return nil, err
		}
		tnames = append(tnames, n)
	}
	rs.Close()
	for _, n := range tnames {
		// L'état de la synchronisation (identifiants partagés des lignes,
		// droits, conflits) n'est pas du contenu : il change hors du journal.
		if n == protTable || n == structTable || n == "_lattis_journal" || n == "_lattis_sheets" || strings.HasPrefix(n, sqlTrash) || syncState[n] {
			continue
		}
		crs, err := t.query(`SELECT name FROM pragma_table_info(?)`, n)
		if err != nil {
			return nil, err
		}
		var cols []string
		sheet, tbl := false, false
		for crs.Next() {
			var c string
			if err := crs.Scan(&c); err != nil {
				crs.Close()
				return nil, err
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
	return metas, nil
}

// sheetPrint ajoute à h l'empreinte d'une feuille et rend son nom : ses
// métadonnées et, avec data, les données de ses tableaux ; sans data
// (structure protégée), son onglet (nom, couleur, masquage) mais ni les
// données des tableaux ni leurs saisies hors type.
func (t *tx) sheetPrint(h hash.Hash, id int64, metas []metaTable, data bool) (string, error) {
	var exists int
	var name string
	if err := t.row(`SELECT count(*), coalesce(max(name), '') FROM _lattis_sheets WHERE id = ?`, id).Scan(&exists, &name); err != nil {
		return "", err
	}
	fmt.Fprintf(h, "exists:%d\x1e", exists)
	if !data {
		if err := t.hashRows(h, `SELECT name, color, hidden FROM _lattis_sheets WHERE id = ?`, id); err != nil {
			return "", err
		}
	}
	for _, mt := range metas {
		if !data && mt.name == "_lattis_invalid" {
			continue
		}
		where := `sheet = ?`
		if !mt.bySheet {
			where = `tbl IN (SELECT id FROM _lattis_tables WHERE sheet = ?)`
		}
		fmt.Fprintf(h, "%s\x1d", mt.name)
		if err := t.hashRows(h, `SELECT `+mt.cols+` FROM `+mt.name+` WHERE `+where+` ORDER BY `+mt.cols, id); err != nil {
			return "", err
		}
	}
	if !data {
		return name, nil
	}
	tables, err := t.query(`SELECT id FROM _lattis_tables WHERE sheet = ? ORDER BY id`, id)
	if err != nil {
		return "", err
	}
	var ids []int64
	for tables.Next() {
		var tid int64
		if err := tables.Scan(&tid); err != nil {
			tables.Close()
			return "", err
		}
		ids = append(ids, tid)
	}
	tables.Close()
	for _, tid := range ids {
		fmt.Fprintf(h, "%s\x1d", tableSQL(tid))
		if err := t.hashRows(h, `SELECT * FROM `+tableSQL(tid)+` ORDER BY id`); err != nil {
			return "", err
		}
	}
	return name, nil
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
