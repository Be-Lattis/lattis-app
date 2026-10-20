package archive

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/zstd"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/ncruces/go-sqlite3"
)

// Report résume une récupération de données (Salvage).
type Report struct {
	Tables  int      `json:"tables"`  // tables recopiées, entières ou en partie
	Rows    int64    `json:"rows"`    // lignes recopiées
	Damaged []string `json:"damaged"` // tables incomplètes ou illisibles
}

// Salvage tire d'un classeur endommagé ses données lisibles. La base est
// extraite comme elle peut (une archive tronquée comprise : la base y est
// décompressée jusqu'à la coupure), puis recopiée ligne à ligne, table par table,
// dans une base neuve au schéma courant (schema la crée), en sautant ce qui
// est illisible. L'historique d'annulation n'est pas repris. Le résultat est
// un nouveau document, jamais enregistré : le fichier d'origine n'est pas
// touché.
func Salvage(path string, schema func(db string) error, extra ...func(*sqliteConn) error) (*Doc, Report, error) {
	dir, err := workDir("")
	if err != nil {
		return nil, Report{}, err
	}
	lock, err := lockDir(dir)
	if err != nil {
		return nil, Report{}, err
	}
	fail := func(err error) (*Doc, Report, error) {
		lock.Close()
		os.RemoveAll(dir)
		return nil, Report{}, err
	}
	src := filepath.Join(dir, "endommagé.sqlite")
	if err := rawDB(path, src); err != nil {
		return fail(err)
	}
	dst := filepath.Join(dir, dbName)
	if err := schema(dst); err != nil {
		return fail(err)
	}
	rep, err := salvageDB(src, dst)
	if err != nil {
		return fail(err)
	}
	os.Remove(src)
	os.Remove(src + "-journal")
	os.Remove(src + "-wal")
	os.Remove(src + "-shm")
	d, err := openDir(dir, "", lock, extra)
	if err != nil {
		os.RemoveAll(dir)
		return nil, Report{}, err
	}
	return d, rep, nil
}

var errNothing = msg.New("archive.nothingReadable", nil)

// rawDB copie la base de l'archive dans dst, aussi loin qu'elle se lit.
func rawDB(path, dst string) error {
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	n, err := copyRawDB(path, w)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n == 0 {
		return errNothing
	}
	// Au-delà de la taille annoncée par l'en-tête : la fin de l'archive
	// (descripteur, répertoire central), pas la base.
	f, err := os.Open(dst)
	if err != nil {
		return err
	}
	h := make([]byte, 100)
	k, _ := io.ReadFull(f, h)
	f.Close()
	pageSize, pages, ok := dbHeader(h[:k])
	if !ok {
		return errNothing
	}
	if want := pageSize * pages; want > 0 && n > want {
		return os.Truncate(dst, want)
	}
	return nil
}

// copyRawDB écrit dans w les octets de la base : par l'archive si elle se
// lit (en gardant ce qui précède une erreur de lecture), sinon en cherchant
// l'en-tête local de la base dans le fichier et en prenant ce qui suit,
// décompressé jusqu'à la première erreur (Zstandard, Deflate) ou tel quel
// (Store).
func copyRawDB(path string, w io.Writer) (int64, error) {
	if zr, err := OpenZip(path); err == nil {
		defer zr.Close()
		for _, f := range zr.File {
			if f.Name != dbName || bomb(f) != "" {
				continue
			}
			r, err := f.Open()
			if err != nil {
				break
			}
			n, _ := io.Copy(w, r) // erreur de somme de contrôle : garder ce qui est lu
			r.Close()
			return n, nil
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	// L'en-tête local de la base suit le manifeste, en tête d'archive.
	head := make([]byte, 1<<20)
	k, _ := io.ReadFull(f, head)
	head = head[:k]
	for off := 0; ; {
		i := bytes.Index(head[off:], []byte("PK\x03\x04"))
		if i < 0 {
			return 0, errNothing
		}
		off += i
		if off+30 > len(head) {
			return 0, errNothing
		}
		h := head[off:]
		method := binary.LittleEndian.Uint16(h[8:])
		nameLen := int(binary.LittleEndian.Uint16(h[26:]))
		extraLen := int(binary.LittleEndian.Uint16(h[28:]))
		known := method == zip.Store || method == zip.Deflate || method == Zstd
		if off+30+nameLen > len(head) || string(h[30:30+nameLen]) != dbName || !known {
			off += 4
			continue
		}
		start := int64(off + 30 + nameLen + extraLen)
		if _, err := f.Seek(start, io.SeekStart); err != nil {
			return 0, err
		}
		return copyRawData(w, f, method, start)
	}
}

// copyRawData décompresse les données d'une entrée sans en connaître la
// fin : ce qui suit le flux (descripteur, répertoire central) ou une
// coupure l'arrête, et ce qui a été lu est gardé. Comme à l'extraction, la
// sortie est bornée à maxRatio fois les octets compressés.
func copyRawData(w io.Writer, f *os.File, method uint16, start int64) (int64, error) {
	var r io.Reader
	switch method {
	case zip.Store:
		return io.Copy(w, f)
	case zip.Deflate:
		fr := flate.NewReader(f)
		defer fr.Close()
		r = fr
	default:
		// Un seul fil : tout bloc lu avant l'erreur est rendu.
		d, err := zstd.NewReader(f, zstd.WithDecoderConcurrency(1))
		if err != nil {
			return 0, err
		}
		defer d.Close()
		r = d
	}
	limit := int64(ratioFloor)
	if st, err := f.Stat(); err == nil {
		limit = max(limit, (st.Size()-start)*int64(ratioLimit(method)))
	}
	n, _ := io.Copy(w, io.LimitReader(r, limit)) // erreur : garder ce qui est lu
	return n, nil
}

// salvageDB recopie dans dst (base au schéma courant) ce qui se lit de src :
// pour chaque table, les lignes dans l'ordre des rowid jusqu'à la première
// page illisible, puis depuis la fin à rebours jusqu'à la même. Une table
// de src absente de dst y est créée ; une table présente des deux côtés ne
// reçoit que les colonnes communes (classeur d'un format plus ancien). Les
// lignes de dst (feuille par défaut, identifiant) sont remplacées par celles
// de src de même clé. Deux connexions : une erreur de lecture de src ne
// touche pas la transaction d'écriture de dst.
func salvageDB(src, dst string) (Report, error) {
	var rep Report
	r, err := sqlite3.Open(src)
	if err != nil {
		return rep, errNothing
	}
	defer r.Close()
	// writable_schema : lire une base plus courte que ne l'annonce son
	// en-tête (tronquée) au lieu de la refuser ; trusted_schema désactivé :
	// son schéma n'appelle aucune fonction à effet de bord.
	if err := r.Exec(`PRAGMA trusted_schema=OFF; PRAGMA writable_schema=ON`); err != nil {
		return rep, err
	}
	w, err := sqlite3.Open(dst)
	if err != nil {
		return rep, err
	}
	defer w.Close()
	if err := w.Exec(`PRAGMA journal_mode=MEMORY; PRAGMA synchronous=OFF`); err != nil {
		return rep, err
	}
	type object struct{ typ, name, sql string }
	var objects []object
	st, _, err := r.Prepare(`SELECT type, name, coalesce(sql, '') FROM sqlite_master ORDER BY rowid`)
	if err != nil {
		return rep, errNothing
	}
	for st.Step() {
		objects = append(objects, object{st.ColumnText(0), st.ColumnText(1), st.ColumnText(2)})
	}
	st.Close()
	if len(objects) == 0 {
		return rep, errNothing
	}
	if err := w.Exec(`BEGIN`); err != nil {
		return rep, err
	}
	for _, o := range objects {
		if o.typ != "table" || strings.HasPrefix(o.name, "sqlite_") || o.name == "_lattis_journal" {
			continue
		}
		cols, err := columns(w, o.name)
		if err != nil || len(cols) == 0 {
			if o.sql == "" || w.Exec(o.sql) != nil {
				rep.Damaged = append(rep.Damaged, o.name)
				continue
			}
			cols, _ = columns(w, o.name)
		}
		have, err := columns(r, o.name)
		if err != nil {
			rep.Damaged = append(rep.Damaged, o.name)
			continue
		}
		common := []string{}
		for _, col := range have {
			for _, k := range cols {
				if strings.EqualFold(col, k) {
					common = append(common, col)
					break
				}
			}
		}
		n, ok := copyRows(r, w, o.name, common)
		rep.Tables++
		rep.Rows += n
		if !ok {
			rep.Damaged = append(rep.Damaged, o.name)
		}
	}
	// Index, déclencheurs et vues des tables créées ici (ceux du schéma
	// courant existent déjà) ; un index unique que les lignes récupérées
	// contredisent est abandonné.
	for _, o := range objects {
		if o.typ != "table" && o.sql != "" && !exists(w, o.name) {
			w.Exec(o.sql)
		}
	}
	if _, err := columns(w, "sqlite_sequence"); err == nil {
		copyRows(r, w, "sqlite_sequence", []string{"name", "seq"})
	}
	return rep, w.Exec(`COMMIT`)
}

func quote(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

// columns donne les colonnes d'une table (aucune : table absente).
func columns(c *sqlite3.Conn, table string) ([]string, error) {
	st, _, err := c.Prepare(`SELECT name FROM pragma_table_info(?)`)
	if err != nil {
		return nil, err
	}
	defer st.Close()
	st.BindText(1, table)
	var out []string
	for st.Step() {
		out = append(out, st.ColumnText(0))
	}
	return out, st.Err()
}

func exists(c *sqlite3.Conn, name string) bool {
	st, _, err := c.Prepare(`SELECT 1 FROM sqlite_master WHERE name = ?`)
	if err != nil {
		return false
	}
	defer st.Close()
	st.BindText(1, name)
	return st.Step()
}

// bind recopie la colonne col de st dans le paramètre i de ins (les deux
// connexions ne partagent pas leurs valeurs).
func bind(ins *sqlite3.Stmt, i int, st *sqlite3.Stmt, col int) {
	switch st.ColumnType(col) {
	case sqlite3.INTEGER:
		ins.BindInt64(i, st.ColumnInt64(col))
	case sqlite3.FLOAT:
		ins.BindFloat(i, st.ColumnFloat(col))
	case sqlite3.TEXT:
		ins.BindText(i, st.ColumnText(col))
	case sqlite3.BLOB:
		ins.BindBlob(i, st.ColumnBlob(col, nil))
	default:
		ins.BindNull(i)
	}
}

// copyRows recopie de r à w les lignes lisibles d'une table ; ok : toutes
// l'ont été.
func copyRows(r, w *sqlite3.Conn, table string, cols []string) (n int64, ok bool) {
	if len(cols) == 0 {
		return 0, true
	}
	q := make([]string, len(cols))
	marks := make([]string, len(cols))
	for i, col := range cols {
		q[i] = quote(col)
		marks[i] = "?"
	}
	list := strings.Join(q, ", ")
	ins, _, err := w.Prepare(fmt.Sprintf(`INSERT OR REPLACE INTO %s (%s) VALUES (%s)`, quote(table), list, strings.Join(marks, ", ")))
	if err != nil {
		return 0, false
	}
	defer ins.Close()
	put := func(st *sqlite3.Stmt, first int) {
		for i := range cols {
			bind(ins, i+1, st, first+i)
		}
		if ins.Exec() == nil {
			n++
		}
		ins.ClearBindings()
	}
	// pass lit dans un sens à partir de from (exclu), sans atteindre stop.
	// Renvoie le dernier rowid lu et si la lecture est allée au bout.
	pass := func(sel string, from, stop int64, desc bool) (last int64, done bool) {
		st, _, err := r.Prepare(sel)
		if err != nil {
			return from, false
		}
		defer st.Close()
		st.BindInt64(1, from)
		last = from
		for st.Step() {
			id := st.ColumnInt64(0)
			if (desc && id <= stop) || (!desc && id >= stop) {
				return last, true
			}
			put(st, 1)
			last = id
		}
		return last, st.Err() == nil
	}
	from := fmt.Sprintf(`SELECT rowid, %s FROM %s`, list, quote(table))
	probe, _, err := r.Prepare(from + ` LIMIT 0`)
	if err != nil {
		// Table sans rowid : un balayage simple, jusqu'à la première erreur.
		st, _, err := r.Prepare(fmt.Sprintf(`SELECT %s FROM %s`, list, quote(table)))
		if err != nil {
			return 0, false
		}
		defer st.Close()
		for st.Step() {
			put(st, 0)
		}
		return n, st.Err() == nil
	}
	probe.Close()
	last, done := pass(from+` WHERE rowid > ? ORDER BY rowid`, -1<<63, 1<<63-1, false)
	if done {
		return n, true
	}
	pass(from+` WHERE rowid < ? ORDER BY rowid DESC`, 1<<63-1, last, true)
	return n, false
}
