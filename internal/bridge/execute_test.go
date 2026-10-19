package bridge

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Onglet SQL : une requête longue s'arrête au bouton Arrêter ou au délai
// des écritures, sans rien modifier (audit 6, I10).
func TestSQLStopAndTimeout(t *testing.T) {
	ctx := context.Background()
	s := &DocumentService{}
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.CloseDocument(ctx) })
	if _, err := s.Call(ctx, "cells.paste", map[string]any{"sheet": "Feuille1", "row": 1, "col": 1, "values": []any{[]any{"Nom", "N"}, []any{"a", 1}, []any{"b", 2}}}); err != nil {
		t.Fatal(err)
	}
	_, err := s.Call(ctx, "tables.list", nil)
	if err != nil {
		t.Fatal(err)
	}
	slow := `UPDATE "Feuille1" SET "N" = (WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c WHERE x < 1000000000) SELECT count(*) FROM c)`
	sum := func() string {
		r, err := s.Call(ctx, "db.query", map[string]any{"sql": `SELECT sum("N") FROM "Feuille1"`})
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprint(r)
	}
	before := sum()

	// Arrêter.
	time.AfterFunc(200*time.Millisecond, s.StopSQL)
	start := time.Now()
	_, err = s.Execute(ctx, slow)
	if err == nil || !strings.Contains(err.Error(), "arrêtée par l'utilisateur") || time.Since(start) > 5*time.Second {
		t.Fatalf("Arrêter : %v en %v", err, time.Since(start))
	}
	// Lecture aussi.
	time.AfterFunc(200*time.Millisecond, s.StopSQL)
	if _, err := s.Query(ctx, `WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x + 1 FROM c WHERE x < 1000000000) SELECT count(*) FROM c`); err == nil || !strings.Contains(err.Error(), "arrêtée par l'utilisateur") {
		t.Fatalf("Arrêter (lecture) : %v", err)
	}

	// Délai des écritures.
	old := SQLWriteTimeout
	SQLWriteTimeout = 300 * time.Millisecond
	defer func() { SQLWriteTimeout = old }()
	start = time.Now()
	_, err = s.Execute(ctx, slow)
	if err == nil || !strings.Contains(err.Error(), "durée maximale") || time.Since(start) > 5*time.Second {
		t.Fatalf("délai : %v en %v", err, time.Since(start))
	}
	if got := sum(); got != before {
		t.Fatalf("données modifiées : %s, avant %s", got, before)
	}
	// Une requête rapide passe toujours.
	if r, err := s.Execute(ctx, `UPDATE "Feuille1" SET "N" = "N" + 1`); err != nil || r.Changed != 2 {
		t.Fatalf("écriture rapide : %+v %v", r, err)
	}
}
