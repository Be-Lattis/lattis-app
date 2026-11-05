package bridge

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// derivedShareServers : faux serveur partagé par deux postes (auteur sur
// auteur.example.test, lecteur sur lattis.example.test), cache et récents
// dans des dossiers de test.
func derivedShareServers(t *testing.T) *workbooktest.Fake {
	t.Helper()
	fake := workbooktest.New()
	oldServer, oldDir, oldRetry, oldRecent := shareServer, shareCacheDir, shareRetry, recentFile
	cache := t.TempDir()
	shareCacheDir = func() (string, error) { return cache, nil }
	shareRetry = 20 * time.Millisecond
	recent := filepath.Join(t.TempDir(), "recent.json")
	recentFile = func() (string, error) { return recent, nil }
	t.Cleanup(func() { shareServer, shareCacheDir, shareRetry, recentFile = oldServer, oldDir, oldRetry, oldRecent })
	shareServer = func(ctx context.Context, server string) (workbook.Server, auth.User, string, error) {
		u := "bob"
		if strings.Contains(server, "auteur.example.test") {
			u = "alice"
		}
		return fake.As(u), auth.User{PGRole: u}, "Serveur de test", nil
	}
	return fake
}

// derivedQueryOf : requête qui charge le tableau name du classeur ouvert
// ("" : aucune, tableau saisi).
func derivedQueryOf(t *testing.T, s *DocumentService, name string) string {
	t.Helper()
	d, err := s.current()
	mustShare(t, err)
	var src string
	err = d.DB().Read().QueryRow(`SELECT coalesce((SELECT q.source FROM _lattis_queries q WHERE q.target = t.id), '') FROM _lattis_tables t WHERE t.name = ?`, name).Scan(&src)
	mustShare(t, err)
	return src
}

// derivedRows lit la première colonne du tableau name, triée.
func derivedRows(t *testing.T, s *DocumentService, name string) []string {
	t.Helper()
	d, err := s.current()
	mustShare(t, err)
	var id int64
	mustShare(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = ?`, name).Scan(&id))
	rs, err := d.DB().Read().Query(fmt.Sprintf(`SELECT CAST(c_1 AS TEXT) FROM t_%d ORDER BY 1`, id))
	mustShare(t, err)
	defer rs.Close()
	var out []string
	for rs.Next() {
		var v string
		mustShare(t, rs.Scan(&v))
		out = append(out, v)
	}
	return out
}

// Un tableau calculé par une requête SQL sur le classeur (Référencer depuis
// l'onglet SQL) arrive chez les autres comme un tableau dynamique : sa
// requête publiée, sans ses lignes, recalculé sur chaque poste, et il suit
// les changements de sa source. Un classeur déjà publié avec ses lignes
// figées le devient à la republication.
func TestShareSQLDerivedTableStaysDynamic(t *testing.T) {
	ctx := context.Background()
	fake := derivedShareServers(t)

	a, _ := shareService(t)
	_, err := a.New(ctx)
	mustShare(t, err)
	_, err = a.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Café", "4"}, {"Thé", "2"}, {"Chocolat", "3"}})
	mustShare(t, err)
	ref, err := a.ReferenceQuery(ctx, `SELECT c_1 AS Produit, c_2 AS Prix FROM t_1 WHERE c_2 > 2`, "Chers", nil, nil)
	mustShare(t, err)
	if len(ref.Loads) != 1 {
		t.Fatalf("référencer : %+v", ref)
	}
	if got := derivedRows(t, a, "Chers"); !slices.Equal(got, []string{"Café", "Chocolat"}) {
		t.Fatalf("auteur : %v", got)
	}
	check, err := a.ShareCheck(ctx)
	mustShare(t, err)
	var source, derived int64
	for _, tb := range check.Tables {
		switch {
		case tb.Derived && tb.Query == "Chers":
			derived = tb.ID
		case !tb.Derived && tb.Query == "":
			source = tb.ID
		}
	}
	if source == 0 || derived == 0 {
		t.Fatalf("vérification : %+v", check.Tables)
	}

	// Classeur publié avant la correction : la boîte proposait « stocké sur
	// le serveur » par défaut, le tableau partait avec ses lignes et sans sa
	// requête (tableau figé chez les autres).
	_, err = a.SharePublish(ctx, ShareRequest{Server: "https://auteur.example.test", Title: "Prix", Mode: workbook.Frozen, Tables: []int64{source, derived}})
	mustShare(t, err)
	list, err := fake.As("alice").Workbooks(ctx)
	mustShare(t, err)
	id := list[0].ID
	mustShare(t, fake.Grant(id, "bob", "reader"))
	b, hb := shareService(t)
	_, err = b.ShareOpen(ctx, "https://lattis.example.test", id, "Prix")
	mustShare(t, err)
	if q := derivedQueryOf(t, b, "Chers"); q != "" {
		t.Fatalf("stocké sur le serveur : requête %s", q)
	}
	waitShare(t, hb, "lecteur en ligne", func(s workbook.State) bool { return s.Online })

	// Republication par l'auteur avec le choix par défaut de la boîte
	// (frontend/src/lib/remote/share.ts, derivedLocal) : recalculé sur
	// chaque poste. Le serveur ne garde que la source.
	_, err = a.ShareRepublish(ctx, []int64{source}, []int64{derived})
	mustShare(t, err)
	if _, _, _, tables := fake.Book(id); len(tables) != 1 {
		t.Fatalf("tableaux du serveur : %v", tables)
	}
	waitShare(t, hb, "nouvelle version", func(s workbook.State) bool { return s.NewVersion == 2 })
	_, err = b.ShareReload(ctx)
	mustShare(t, err)
	if q := derivedQueryOf(t, b, "Chers"); !strings.Contains(q, `"sql"`) || !strings.Contains(q, "t_1") {
		t.Fatalf("lecteur : requête %q", q)
	}
	if got := derivedRows(t, b, "Chers"); !slices.Equal(got, []string{"Café", "Chocolat"}) {
		t.Fatalf("lecteur, recalculé : %v", got)
	}
	var shared string
	for sheet := int64(1); sheet <= 3; sheet++ {
		l, err := b.Layout(ctx, sheet)
		if err != nil {
			continue
		}
		for _, tb := range l.Tables {
			if tb.Name == "Chers" {
				shared = tb.Shared
			}
		}
	}
	if shared != "local" {
		t.Fatalf("lecteur : tableau %q", shared)
	}

	// La source change chez l'auteur : la nouvelle version la porte, le
	// lecteur recalcule le tableau chez lui.
	_, err = a.SetCell(ctx, 1, 2, 1, "5") // Thé : 2 → 5
	mustShare(t, err)
	_, err = a.ShareRepublish(ctx, nil, nil)
	mustShare(t, err)
	waitShare(t, hb, "troisième version", func(s workbook.State) bool { return s.NewVersion == 3 })
	_, err = b.ShareReload(ctx)
	mustShare(t, err)
	if got := derivedRows(t, b, "Chers"); !slices.Equal(got, []string{"Café", "Chocolat", "Thé"}) {
		t.Fatalf("lecteur, après changement de la source : %v", got)
	}

	// Un poste neuf (session web neuve : cache vide) l'ouvre aussi en
	// dynamique.
	_ = b.CloseDocument(ctx)
	shareCacheDir = func() (string, error) { return filepath.Join(t.TempDir(), "neuf"), nil }
	c, _ := shareService(t)
	_, err = c.ShareOpen(ctx, "https://lattis.example.test", id, "Prix")
	mustShare(t, err)
	if q := derivedQueryOf(t, c, "Chers"); q == "" {
		t.Fatal("poste neuf : tableau figé")
	}
	if got := derivedRows(t, c, "Chers"); !slices.Equal(got, []string{"Café", "Chocolat", "Thé"}) {
		t.Fatalf("poste neuf : %v", got)
	}
	if _, err := c.RefreshQuery(ctx, queryID(t, c, "Chers")); err != nil {
		t.Fatalf("actualisation chez le lecteur : %v", err)
	}
}

func queryID(t *testing.T, s *DocumentService, name string) int64 {
	t.Helper()
	d, err := s.current()
	mustShare(t, err)
	var id int64
	mustShare(t, d.DB().Read().QueryRow(`SELECT id FROM _lattis_queries WHERE name = ?`, name).Scan(&id))
	return id
}
