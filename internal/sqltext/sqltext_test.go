package sqltext

import (
	"context"
	"errors"
	"testing"
)

func TestFrench(t *testing.T) {
	for in, want := range map[string]string{
		"sqlite3: SQL logic error: no such table: Inconnue":                       "Tableau inconnu : « Inconnue »",
		"sqlite3: SQL logic error: no such column: Prixx":                         "Colonne inconnue : « Prixx »",
		`sqlite3: SQL logic error: near "SELEC": syntax error`:                    "Erreur de syntaxe près de « SELEC »",
		"sqlite3: SQL logic error: ambiguous column name: Client":                 `Colonne ambiguë : « Client » est dans plusieurs tableaux de la requête ; préciser le tableau (Tableau."Client")`,
		"sqlite3: SQL logic error: incomplete input":                              "Requête incomplète : il manque la fin (parenthèse, mot-clé ou valeur)",
		"sqlite3: SQL logic error: no such function: foo":                         "Fonction inconnue : « foo »",
		"sqlite3: SQL logic error: wrong number of arguments to function count()": "Nombre d'arguments incorrect pour la fonction count()",
		"sqlite3: constraint failed: UNIQUE constraint failed: t_1.c_2":           "Valeur en double : une autre ligne a déjà cette clé (UNIQUE constraint failed: t_1.c_2)",
		"sqlite3: SQL logic error: quelque chose d'inconnu":                       "quelque chose d'inconnu",
		"déjà en français": "déjà en français",
	} {
		if got := French(errors.New(in)).Error(); got != want {
			t.Errorf("%q :\n got %q\nwant %q", in, got, want)
		}
	}
	if French(nil) != nil {
		t.Fatal("nil")
	}
	// L'erreur d'origine reste accessible.
	if err := French(context.DeadlineExceeded); !errors.Is(err, context.DeadlineExceeded) || err.Error() != "Requête interrompue : trop longue, ou arrêtée" {
		t.Fatalf("délai : %v", err)
	}
}

func TestCheckStructure(t *testing.T) {
	for _, q := range []string{"DROP TABLE Clients", "  -- x\n alter table a add column b", "/* c */ CREATE TABLE x(a)", "reindex"} {
		if !errors.Is(CheckStructure(q), ErrStructure) {
			t.Errorf("%q accepté", q)
		}
	}
	for _, q := range []string{"SELECT 1", "WITH a AS (SELECT 1) SELECT * FROM a", "UPDATE x SET a = 1", "PRAGMA table_info(x)", "SELECT 'DROP'"} {
		if err := CheckStructure(q); err != nil {
			t.Errorf("%q refusé : %v", q, err)
		}
	}
}

func TestPrependWith(t *testing.T) {
	defs := []string{"t_1 AS (SELECT 1)"}
	for in, want := range map[string]string{
		"SELECT * FROM t_1":                          "WITH t_1 AS (SELECT 1) SELECT * FROM t_1",
		" with a AS (SELECT 2) SELECT 1":             "WITH t_1 AS (SELECT 1), a AS (SELECT 2) SELECT 1",
		"WITH RECURSIVE a(n) AS (SELECT 1) SELECT 1": "WITH RECURSIVE t_1 AS (SELECT 1), a(n) AS (SELECT 1) SELECT 1",
		"UPDATE t_1 SET a = 1":                       "UPDATE t_1 SET a = 1",
	} {
		if got := PrependWith(in, defs); got != want {
			t.Errorf("%q :\n got %q\nwant %q", in, got, want)
		}
	}
	if got := PrependWith("SELECT 1", nil); got != "SELECT 1" {
		t.Fatal(got)
	}
}
