package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ncruces/go-sqlite3"
)

// Une panique de SQLite pendant une écriture (mémoire épuisée…) devient une
// erreur : l'écriture est annulée et la base reste utilisable.
func TestWritePanic(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	err = s.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE t (x)`); err != nil {
			return err
		}
		panic("sqlite3: out of memory")
	})
	if !errors.Is(err, ErrEngine) {
		t.Fatalf("erreur attendue, %v", err)
	}
	if err := s.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE t (x)`)
		return err
	}); err != nil {
		t.Fatalf("base inutilisable après la panique : %v", err)
	}
}

// TakeTouched rend les tableaux modifiés depuis l'appel précédent : une
// écriture sur t_2 laisse t_1 valable ; une écriture sur les métadonnées des
// colonnes concerne tous les tableaux.
func TestTouched(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	exec := func(q string) {
		t.Helper()
		if err := s.Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec(q); return err }); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE t_1 (x)`)
	exec(`CREATE TABLE t_2 (x)`)
	exec(`CREATE TABLE _lattis_columns (x)`)
	s.TakeTouched()
	exec(`INSERT INTO t_2 VALUES (1)`)
	exec(`UPDATE t_1 SET x = 2`)
	content, structure, all := s.TakeTouched()
	if all || !structure["t_2"] || structure["t_1"] || !content["t_1"] {
		t.Fatalf("après t_2 et t_1 : %v %v %v", content, structure, all)
	}
	if content, structure, all = s.TakeTouched(); all || len(content)+len(structure) != 0 {
		t.Fatalf("remise à zéro : %v %v %v", content, structure, all)
	}
	exec(`INSERT INTO _lattis_columns VALUES (1)`)
	if _, _, all = s.TakeTouched(); !all {
		t.Fatal("métadonnées des colonnes : tous les tableaux concernés")
	}
	// Le choix d'un onglet filtre ses tableaux : tous concernés ; un autre
	// réglage de la feuille (zoom) non.
	exec(`CREATE TABLE _lattis_sheets (id, zoom, choice)`)
	s.TakeTouched()
	exec(`UPDATE _lattis_sheets SET zoom = 120 WHERE id = 1`)
	if _, _, all = s.TakeTouched(); all {
		t.Fatal("le zoom ne concerne aucun tableau")
	}
	v := s.Version("t_1")
	exec(`UPDATE _lattis_sheets SET choice = '{}' WHERE id = 1`)
	if _, _, all = s.TakeTouched(); !all || s.Version("t_1") == v {
		t.Fatal("choix de l'onglet : tous les tableaux concernés")
	}
}

// Versions par tableau : une saisie ne change pas celle de ses lignes ; une
// écriture ailleurs ne change rien.
func TestVersion(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	exec := func(q string) {
		t.Helper()
		if err := s.Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec(q); return err }); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE t_1 (x)`)
	exec(`CREATE TABLE t_2 (x)`)
	all, rows := s.Version("t_1"), s.Version(RowsOf("t_1"))
	exec(`INSERT INTO t_2 VALUES (1)`)
	if s.Version("t_1") != all || s.Version(RowsOf("t_1")) != rows {
		t.Fatal("une écriture dans t_2 change la version de t_1")
	}
	exec(`UPDATE t_1 SET x = 2`)
	if s.Version("t_1") == all || s.Version(RowsOf("t_1")) != rows {
		t.Fatal("une mise à jour de valeurs : version des valeurs seulement")
	}
	exec(`INSERT INTO t_1 VALUES (3)`)
	if s.Version(RowsOf("t_1")) == rows {
		t.Fatal("un ajout de ligne change la version des lignes")
	}
	if s.Quiet() < 0 {
		t.Fatal("aucune écriture ne tourne")
	}
}

// Mémoire de SQLite bornée par classeur : 64 Mo de cache pour l'écriture,
// 32 Mo par connexion de lecture, et 6 connexions de lecture au plus
// (audit 6, M9).
func TestCacheBudget(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var read, write int
	if err := s.Read().QueryRowContext(ctx, `PRAGMA cache_size`).Scan(&read); err != nil {
		t.Fatal(err)
	}
	if err := s.Write(ctx, func(tx *sql.Tx) error { return tx.QueryRow(`PRAGMA cache_size`).Scan(&write) }); err != nil {
		t.Fatal(err)
	}
	if read != -readCacheKiB || write != -writeCacheKiB {
		t.Fatalf("cache : lecture %d, écriture %d", read, write)
	}
	if n := s.Read().Stats().MaxOpenConnections; n < minReadConns || n > maxReadConns {
		t.Fatalf("%d connexions de lecture", n)
	}
	if total := readCacheKiB*maxReadConns + writeCacheKiB; total > 256<<10 {
		t.Fatalf("plafond de %d Kio", total)
	}
}

// Base venue d'un fichier reçu (audit 6, M2) : une vue du fichier ne peut
// appeler qu'une fonction sans effet de bord, le schéma ne se réécrit pas
// (mode défensif) et cell_size_check est actif, en lecture comme en écriture.
func TestHardened(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "recu.db")
	fns := func(c *sqlite3.Conn) error {
		if err := c.CreateFunction("sure", 1, sqlite3.DETERMINISTIC|sqlite3.INNOCUOUS, func(ctx sqlite3.Context, a ...sqlite3.Value) { ctx.ResultInt(1) }); err != nil {
			return err
		}
		return c.CreateFunction("risquee", 1, sqlite3.DETERMINISTIC, func(ctx sqlite3.Context, a ...sqlite3.Value) { ctx.ResultInt(2) })
	}
	s, err := Open(path, fns)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE t (x); INSERT INTO t VALUES (1); CREATE VIEW ok AS SELECT sure(x) AS v FROM t; CREATE VIEW piege AS SELECT risquee(x) AS v FROM t`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var v int
	if err := s.Read().QueryRowContext(ctx, `SELECT v FROM ok`).Scan(&v); err != nil || v != 1 {
		t.Fatalf("fonction INNOCUOUS dans une vue : %d, %v", v, err)
	}
	if err := s.Read().QueryRowContext(ctx, `SELECT v FROM piege`).Scan(&v); err == nil {
		t.Fatal("fonction non marquée INNOCUOUS appelée depuis une vue du fichier")
	}
	if err := s.Read().QueryRowContext(ctx, `SELECT risquee(x) FROM t`).Scan(&v); err != nil || v != 2 {
		t.Fatalf("appel direct : %d, %v", v, err)
	}
	err = s.Exec(ctx, func(db *sql.DB) error {
		if _, err := db.ExecContext(ctx, `PRAGMA writable_schema=ON`); err != nil {
			return err
		}
		_, err := db.ExecContext(ctx, `UPDATE sqlite_schema SET sql = 'CREATE TABLE t (y)' WHERE name = 't'`)
		return err
	})
	if err == nil {
		t.Fatal("schéma réécrit malgré le mode défensif")
	}
	for _, db := range []*sql.DB{s.Read(), s.write} {
		var trusted, cells int
		if err := db.QueryRowContext(ctx, `SELECT (SELECT trusted_schema FROM pragma_trusted_schema), (SELECT cell_size_check FROM pragma_cell_size_check)`).Scan(&trusted, &cells); err != nil {
			t.Fatal(err)
		}
		if trusted != 0 || cells != 1 {
			t.Fatalf("trusted_schema=%d, cell_size_check=%d", trusted, cells)
		}
	}
}

// Commandes de l'utilisateur validées en synchronous=FULL, les autres
// écritures en NORMAL (audit 6, M3).
func TestWriteDurable(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	level := func(write func(context.Context, func(*sql.Tx) error) error) int {
		var n int
		if err := write(ctx, func(tx *sql.Tx) error { return tx.QueryRow(`PRAGMA synchronous`).Scan(&n) }); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := level(s.Write); n != 1 {
		t.Fatalf("écriture ordinaire : synchronous=%d", n)
	}
	if n := level(s.WriteDurable); n != 2 {
		t.Fatalf("commande de l'utilisateur : synchronous=%d", n)
	}
	if n := level(s.Write); n != 1 {
		t.Fatalf("après une commande : synchronous=%d", n)
	}
}
