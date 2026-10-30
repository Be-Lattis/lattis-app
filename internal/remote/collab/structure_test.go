package collab_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Structure des tableaux partagés (§6.4, fonctions du serveur §9.3) :
// faite sur le serveur avant la validation locale, refus du serveur
// affichés tels quels, propagation par « *structure:<id> », annulation
// par l'opération inverse.

// colsOf lit les colonnes (nom:type) d'un tableau local, dans l'ordre.
func colsOf(c *client, table string) []string {
	c.t.Helper()
	rs, err := c.d.DB().Read().Query(`SELECT k.name || ':' || k.type FROM _lattis_columns k JOIN _lattis_tables t ON t.id = k.tbl WHERE t.name = ? ORDER BY k.pos`, table)
	must(c.t, err)
	defer rs.Close()
	var out []string
	for rs.Next() {
		var s string
		must(c.t, rs.Scan(&s))
		out = append(out, s)
	}
	return out
}

// keyOf donne le nom de la colonne clé d'un tableau local ("" sans clé).
func keyOf(c *client, table string) string {
	c.t.Helper()
	var k string
	must(c.t, c.d.DB().Read().QueryRow(`SELECT coalesce((SELECT k.name FROM _lattis_columns k WHERE k.tbl = t.id AND k.id = t.key_col), '') FROM _lattis_tables t WHERE t.name = ?`, table).Scan(&k))
	return k
}

// serverCols lit les colonnes (nom:type) d'un tableau sur le serveur, et sa
// clé.
func serverCols(f *workbooktest.Fake, id, table string) ([]string, string) {
	cols, key := f.Columns(id, table)
	var out []string
	keyName := ""
	for _, c := range cols {
		out = append(out, c.Name+":"+c.Type)
		if c.ID == key {
			keyName = c.Name
		}
	}
	return out, keyName
}

func hasTable(c *client, name string) bool {
	var n int
	must(c.t, c.d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_tables WHERE name = ?`, name).Scan(&n))
	return n > 0
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(patience)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s : jamais atteint", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// remoteID donne l'identifiant sur le serveur du tableau Ventes et d'une
// de ses colonnes.
func remoteID(t *testing.T, c *client, col string) (table, column string) {
	t.Helper()
	rt, _ := remoteTable(t, c)
	for _, x := range rt.Columns {
		if x.Name == col {
			column = x.ID
		}
	}
	return rt.ID, column
}

// header renomme la colonne i de Ventes (saisie dans l'en-tête).
func (c *client) header(i int, name string) error {
	_, err := c.d.SetCell(context.Background(), 1, 0, i, name)
	return err
}

func TestStructureFollowedByColleague(t *testing.T) {
	old := collab.Debounce
	collab.Debounce = 20 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f, alice, bob, id := setup(t)
	var notices []string
	got := make(chan []string, 64)
	bob.follow(ctx, f.As("bob"), time.Hour, func(ev collab.Event) {
		if len(ev.Notices) > 0 {
			got <- ev.Notices
		}
	})
	alice.follow(ctx, f.As("alice"), time.Hour, nil)
	waitOnline(t, bob, alice)
	drain := func() {
		for {
			select {
			case n := <-got:
				notices = append(notices, n...)
			default:
				return
			}
		}
	}

	// Colonne ajoutée : faite sur le serveur (après « Produit »), reçue par
	// Bob ; une valeur saisie ensuite y passe.
	must(t, alice.d.InsertColumn(ctx, "Ventes", 1, "Remise"))
	alice.send() // écriture optimiste : faite en local, envoyée par la file
	cols, _ := serverCols(f, id, "Ventes")
	eq(t, "serveur après ajout", cols, []string{"Produit:text", "Remise:text", "Quantité:integer", "Prix:decimal"})
	waitFor(t, "colonne Remise chez Bob", func() bool {
		return strings.Join(colsOf(bob, "Ventes"), "|") == "Produit:text|Remise:text|Quantité:integer|Prix:decimal"
	})
	alice.mustSet(0, 1, "5 %")
	alice.e.Kick()
	waitFor(t, "valeur de Remise chez Bob", func() bool { return bob.col("Remise")[0] == "5 %" })

	// Colonne renommée (en-tête) : les valeurs restent.
	must(t, alice.header(3, "Tarif"))
	alice.send()
	cols, _ = serverCols(f, id, "Ventes")
	eq(t, "serveur après renommage", cols, []string{"Produit:text", "Remise:text", "Quantité:integer", "Tarif:decimal"})
	waitFor(t, "Tarif chez Bob", func() bool { return strings.Contains(strings.Join(colsOf(bob, "Ventes"), "|"), "Tarif:decimal") })
	eq(t, "Bob Tarif", bob.col("Tarif"), []string{"4.5", "2.0", "3.25"})

	// Type changé : converti sur le serveur, relu par Bob.
	must(t, alice.d.SetType(ctx, "Ventes", 2, grid.Decimal))
	alice.send()
	cols, _ = serverCols(f, id, "Ventes")
	eq(t, "serveur après conversion", cols, []string{"Produit:text", "Remise:text", "Quantité:decimal", "Tarif:decimal"})
	waitFor(t, "Quantité décimale chez Bob", func() bool { return strings.Contains(strings.Join(colsOf(bob, "Ventes"), "|"), "Quantité:decimal") })
	eq(t, "Bob Quantité", bob.col("Quantité"), []string{"3.0", "10.0", "1.0"})

	// Clé : posée sur le serveur, suivie par Bob (*structure et *meta).
	must(t, alice.d.SetKey(ctx, "Ventes", 0))
	alice.send()
	if _, key := serverCols(f, id, "Ventes"); key != "Produit" {
		t.Fatalf("clé du serveur : %q", key)
	}
	waitFor(t, "clé chez Bob", func() bool { return keyOf(bob, "Ventes") == "Produit" })

	// Colonne supprimée.
	must(t, alice.d.DeleteColumn(ctx, "Ventes", 1))
	alice.send()
	cols, _ = serverCols(f, id, "Ventes")
	eq(t, "serveur après suppression", cols, []string{"Produit:text", "Quantité:decimal", "Tarif:decimal"})
	waitFor(t, "Remise supprimée chez Bob", func() bool { return !strings.Contains(strings.Join(colsOf(bob, "Ventes"), "|"), "Remise") })

	// Tableau renommé : les écritures suivent le nouveau nom.
	must(t, alice.d.RenameTable(ctx, "Ventes", "Achats"))
	alice.send()
	if _, _, _, tables := f.Book(id); strings.Join(tables, ",") != "Achats" {
		t.Fatalf("tableaux du serveur : %v", tables)
	}
	waitFor(t, "Achats chez Bob", func() bool { return hasTable(bob, "Achats") })
	_, err := alice.d.SetCell(ctx, 1, 2, 1, "12")
	must(t, err)
	alice.e.Kick()
	waitFor(t, "valeur dans Achats chez Bob", func() bool {
		var v any
		bob.d.DB().Read().QueryRow(`SELECT c_` + strconv.FormatInt(colID(t, bob, "Achats", "Quantité"), 10) + ` FROM t_` + strconv.FormatInt(tableOf(t, bob, "Achats"), 10) + ` ORDER BY _pos LIMIT 1 OFFSET 1`).Scan(&v)
		return v == 12.0
	})

	// Tableau supprimé.
	must(t, alice.d.DeleteTables(ctx, []string{"Achats"}))
	alice.send()
	if _, _, _, tables := f.Book(id); len(tables) != 0 {
		t.Fatalf("tableaux du serveur après suppression : %v", tables)
	}
	waitFor(t, "Achats supprimé chez Bob", func() bool { return !hasTable(bob, "Achats") })
	if bob.d.Share.TableShare(tableOfAny(alice)) == "published" {
		t.Fatal("tableau toujours publié chez Alice")
	}
	// Le message de la suppression peut suivre de peu le tableau disparu
	// (événement du suivi) : attendu, pas seulement lu.
	wants := []string{"Structure du tableau « Ventes » modifiée sur le serveur", "Tableau « Achats » supprimé sur le serveur"}
	deadline := time.Now().Add(5 * time.Second)
	for {
		drain()
		all := strings.Join(notices, " ; ")
		missing := ""
		for _, want := range wants {
			if !strings.Contains(all, want) {
				missing = want
				break
			}
		}
		if missing == "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("messages de Bob : %q (attendu %q)", all, missing)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func colID(t *testing.T, c *client, table, col string) int64 {
	t.Helper()
	var id int64
	must(t, c.d.DB().Read().QueryRow(`SELECT k.id FROM _lattis_columns k JOIN _lattis_tables t ON t.id = k.tbl WHERE t.name = ? AND k.name = ?`, table, col).Scan(&id))
	return id
}

func tableOf(t *testing.T, c *client, name string) int64 {
	t.Helper()
	var id int64
	must(t, c.d.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = ?`, name).Scan(&id))
	return id
}

// tableOfAny : le tableau de la feuille 1 d'origine (premier publié).
func tableOfAny(c *client) int64 {
	var id int64
	c.d.DB().Read().QueryRow(`SELECT tbl FROM _lattis_remote_tables ORDER BY tbl LIMIT 1`).Scan(&id)
	return id
}

func TestStructureRefusedByServer(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	srv := f.As("alice")
	tid, _ := remoteID(t, alice, "Produit")
	before := colsOf(alice, "Ventes")
	// Écriture optimiste (décision 0012) : le changement est fait en local
	// tout de suite ; l'envoi le fait refuser par le serveur, il est défait
	// en local (forme et données du serveur), avec le message du serveur.
	refused := func(what string, err error, want string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s : refusé en local : %v", what, err)
		}
		if strings.Join(colsOf(alice, "Ventes"), "|") == strings.Join(before, "|") && hasTable(alice, "Ventes") {
			t.Fatalf("%s : pas appliqué en local avant l'envoi", what)
		}
		r := alice.send()
		all := strings.Join(r.Notices, " ; ")
		if r.Rejected != 1 || !strings.Contains(all, "Refusé par le serveur : ") || !strings.Contains(all, want) || !strings.Contains(all, "la modification a été annulée") {
			t.Fatalf("%s : %+v (attendu « %s »)", what, r, want)
		}
		eq(t, what+" : Alice rétablie", colsOf(alice, "Ventes"), before)
		eq(t, what+" : valeurs rétablies", alice.col("Prix"), []string{"4.5", "2.0", "3.25"})
		eq(t, what+" : Produit", alice.col("Produit"), []string{"Café", "Thé", "Chocolat"})
		if alice.states()["pending"] != 0 {
			t.Fatalf("%s : file : %v", what, alice.states())
		}
	}

	// Colonne citée par une restriction : ni renommée, ni convertie, ni
	// supprimée.
	_, err := srv.AddRestriction(ctx, id, workbook.RestrictionRule{TableID: tid, Command: "select", Predicate: `"Quantité" > 0`, Label: "En stock"})
	must(t, err)
	refused("renommage", alice.header(1, "Qté"), "la colonne « Quantité » est citée par la restriction « En stock » : modifiez d'abord ce filtre")
	refused("conversion", alice.d.SetType(ctx, "Ventes", 1, grid.Decimal), "restriction « En stock »")
	refused("suppression", alice.d.DeleteColumn(ctx, "Ventes", 1), "la colonne « Quantité » est citée par la restriction « En stock » : modifiez-le d'abord")
	eq(t, "Quantité rétablie", alice.col("Quantité"), []string{"3", "10", "1"})

	// Colonne citée par le filtre d'un rôle.
	role, err := srv.CreateRole(ctx, id, "Nord")
	must(t, err)
	must(t, srv.SetLevels(ctx, []workbook.Level{{RoleID: role, NodeKind: "table", NodeID: tid, Level: "reader", ReadFilter: `"Prix" > 1`}}))
	refused("filtre d'un rôle", alice.d.DeleteColumn(ctx, "Ventes", 2), "le filtre du rôle « Nord »")

	// Clé d'un pipeline.
	f.AddPipeline(id, tid, "Import ERP", "Produit")
	refused("clé d'un pipeline", alice.d.DeleteColumn(ctx, "Ventes", 0), "la clé du pipeline « Import ERP »")

	// Tableau qui disparaîtrait autrement que par sa suppression (conversion
	// en plage) : refusé en local, il resterait sur le serveur.
	var implicit oplog.ErrImplicitDrop
	if err := alice.d.ConvertToRange(ctx, "Ventes"); !errors.As(err, &implicit) {
		t.Fatalf("conversion en plage : %v", err)
	}

	// Tableau qui alimente le Data Lake : supprimé en local, il revient
	// (reçu du serveur, toujours partagé).
	_, err = srv.CreateDataset(ctx, workbook.DatasetSpec{WorkbookID: id, TableID: tid, Name: "Ventes publiques"})
	must(t, err)
	refused("suppression du tableau", alice.d.DeleteTables(ctx, []string{"Ventes"}), "le tableau « Ventes » alimente le Data Lake (« Ventes publiques ») : retirez d'abord ces données")
	if alice.d.Share.TableShare(tableID(t, alice.d)) != "published" {
		t.Fatal("tableau rétabli mais plus partagé")
	}

	cols, _ := serverCols(f, id, "Ventes")
	eq(t, "serveur inchangé", cols, []string{"Produit:text", "Quantité:integer", "Prix:decimal"})

	// Une colonne libre se renomme toujours.
	must(t, alice.d.InsertColumn(ctx, "Ventes", 3, "Note"))
	must(t, alice.header(3, "Commentaire"))
	alice.send()
	cols, _ = serverCols(f, id, "Ventes")
	eq(t, "serveur", cols, []string{"Produit:text", "Quantité:integer", "Prix:decimal", "Commentaire:text"})

	// Éditeur (pas concepteur) : refusé en local, avant le serveur.
	if err := bob.d.InsertColumn(ctx, "Ventes", 1, "Bob"); !errors.Is(err, workbook.ErrDesign) && !errors.Is(err, workbook.ErrLayout) {
		t.Fatalf("éditeur : %v", err)
	}
	// Hors ligne : la structure aussi attend dans la file (décision 0014 ;
	// voir TestOfflineStructure).
}

// TestStructureRefusedDropsDependents : refusé par le serveur, un
// changement de structure emporte les changements de structure suivants du
// même tableau encore en attente (ils le supposaient) ; les données
// saisies entre-temps dans les colonnes publiées partent quand même.
func TestStructureRefusedDropsDependents(t *testing.T) {
	ctx := context.Background()
	f, alice, _, id := setup(t)
	tid, _ := remoteID(t, alice, "Produit")
	_, err := f.As("alice").AddRestriction(ctx, id, workbook.RestrictionRule{TableID: tid, Command: "select", Predicate: `"Quantité" > 0`, Label: "En stock"})
	must(t, err)
	must(t, alice.header(1, "Qté"))                         // refusé par le serveur
	must(t, alice.d.InsertColumn(ctx, "Ventes", 3, "Note")) // suit le renommage refusé
	alice.mustSet(1, 2, "9")                                // Prix : colonne publiée
	eq(t, "Alice avant l'envoi", colsOf(alice, "Ventes"), []string{"Produit:text", "Qté:integer", "Prix:decimal", "Note:text"})
	r := alice.send()
	all := strings.Join(r.Notices, " ; ")
	if !strings.Contains(all, "la modification a été annulée") || !strings.Contains(all, "1 changement(s) de structure suivant(s)") {
		t.Fatalf("rapport : %+v", r)
	}
	eq(t, "Alice après le refus", colsOf(alice, "Ventes"), []string{"Produit:text", "Quantité:integer", "Prix:decimal"})
	eq(t, "Prix gardé", alice.col("Prix"), []string{"4.5", "9.0", "3.25"})
	if v, _, _, _ := f.Value(id, "Ventes", rowID(t, alice.d, 1), "Prix"); v != 9.0 {
		t.Fatalf("Prix sur le serveur : %v", v)
	}
	cols, _ := serverCols(f, id, "Ventes")
	eq(t, "serveur", cols, []string{"Produit:text", "Quantité:integer", "Prix:decimal"})
	if n := alice.states()["pending"]; n != 0 {
		t.Fatalf("file : %v", alice.states())
	}
}

func TestStructureUndo(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	reload := func() {
		t.Helper()
		_, err := bob.e.ReceiveStructure(ctx, nil, map[int64]bool{tableID(t, bob.d): true})
		must(t, err)
	}
	// server envoie la file d'Alice et compare les colonnes du serveur.
	server := func(want ...string) {
		t.Helper()
		alice.send()
		cols, _ := serverCols(f, id, "Ventes")
		eq(t, "serveur", cols, want)
	}

	// Ajout annulé : la colonne part du serveur ; rétabli : elle revient.
	must(t, alice.d.InsertColumn(ctx, "Ventes", 1, "Remise"))
	server("Produit:text", "Remise:text", "Quantité:integer", "Prix:decimal")
	_, err := alice.d.Bus.Undo(ctx)
	must(t, err)
	server("Produit:text", "Quantité:integer", "Prix:decimal")
	_, err = alice.d.Bus.Redo(ctx)
	must(t, err)
	server("Produit:text", "Remise:text", "Quantité:integer", "Prix:decimal")
	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)

	// Suppression annulée : la colonne revient avec ses valeurs (envoyées
	// ensuite par la file), chez Bob aussi.
	must(t, alice.d.DeleteColumn(ctx, "Ventes", 2))
	server("Produit:text", "Quantité:integer")
	reload()
	eq(t, "Bob sans Prix", colsOf(bob, "Ventes"), []string{"Produit:text", "Quantité:integer"})
	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)
	if r := alice.send(); r.Rejected != 0 {
		t.Fatalf("valeurs rétablies : %+v", r)
	}
	server("Produit:text", "Quantité:integer", "Prix:decimal")
	if v, _, _, _ := f.Value(id, "Ventes", rowID(t, alice.d, 0), "Prix"); v != 4.5 {
		t.Fatalf("Prix rétabli sur le serveur : %v", v)
	}
	reload()
	eq(t, "Bob Prix rétabli", bob.col("Prix"), []string{"4.5", "2.0", "3.25"})

	// Renommage et conversion annulés, avant même leur envoi : la file
	// envoie l'aller puis le retour, dans l'ordre.
	must(t, alice.header(0, "Article"))
	must(t, alice.d.SetType(ctx, "Ventes", 1, grid.Decimal))
	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)
	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)
	server("Produit:text", "Quantité:integer", "Prix:decimal")

	// Annulation refusée par le serveur : la colonne revient en local (forme
	// du serveur), l'historique du tableau est effacé.
	must(t, alice.d.InsertColumn(ctx, "Ventes", 3, "Zone"))
	alice.send()
	tid, _ := remoteID(t, alice, "Zone")
	_, err = f.As("alice").AddRestriction(ctx, id, workbook.RestrictionRule{TableID: tid, Command: "all", Predicate: `"Zone" = 'Nord'`, Label: "Zone nord"})
	must(t, err)
	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)
	eq(t, "Zone retirée en local", colsOf(alice, "Ventes"), []string{"Produit:text", "Quantité:integer", "Prix:decimal"})
	r := alice.send()
	if all := strings.Join(r.Notices, " ; "); !strings.Contains(all, "Annulation refusée par le serveur : la colonne « Zone » est citée par la restriction « Zone nord »") || !strings.Contains(all, "la modification a été annulée") {
		t.Fatalf("annulation refusée : %+v", r)
	}
	eq(t, "Zone rétablie", colsOf(alice, "Ventes"), []string{"Produit:text", "Quantité:integer", "Prix:decimal", "Zone:text"})
	if st, _ := alice.d.Bus.State(ctx); st.Undo != "" || st.Redo != "" {
		t.Fatalf("historique après refus : %+v", st)
	}
	server("Produit:text", "Quantité:integer", "Prix:decimal", "Zone:text")

	// Suppression du tableau annulée : rétabli sur ce poste seulement.
	must(t, alice.d.DeleteTables(ctx, []string{"Ventes"}))
	alice.send()
	_, err = alice.d.Bus.Undo(ctx)
	must(t, err)
	if msg := collab.UndoNotice(alice.d.Oplog, false); !strings.Contains(msg, "« Ventes » est rétabli sur ce poste seulement") {
		t.Fatalf("message : %q", msg)
	}
	if !hasTable(alice, "Ventes") || alice.d.Share.TableShare(tableID(t, alice.d)) == "published" {
		t.Fatal("tableau rétabli : absent, ou encore publié")
	}
	if _, _, _, tables := f.Book(id); len(tables) != 0 {
		t.Fatalf("serveur : %v", tables)
	}
}

// TestStructureUndoBeforeSend : un tableau supprimé puis rétabli par
// Ctrl+Z avant l'envoi part quand même du serveur (l'étape d'aller est
// envoyée) ; il reste sur ce poste seulement, et le message le dit.
func TestStructureUndoBeforeSend(t *testing.T) {
	ctx := context.Background()
	f, alice, _, id := setup(t)
	must(t, alice.d.DeleteTables(ctx, []string{"Ventes"}))
	_, err := alice.d.Bus.Undo(ctx)
	must(t, err)
	r := alice.send()
	if all := strings.Join(r.Notices, " ; "); !strings.Contains(all, "« Ventes » est rétabli sur ce poste seulement") {
		t.Fatalf("rapport : %+v", r)
	}
	if !hasTable(alice, "Ventes") || alice.d.Share.TableShare(tableID(t, alice.d)) == "published" {
		t.Fatal("tableau rétabli : absent, ou encore publié")
	}
	if _, _, _, tables := f.Book(id); len(tables) != 0 {
		t.Fatalf("serveur : %v", tables)
	}
	if alice.states()["pending"] != 0 {
		t.Fatalf("file : %v", alice.states())
	}
}

// Une modification de structure reçue efface l'historique qui touche le
// tableau : ses étapes ne s'appliqueraient plus.
func TestStructureReceivedClearsHistory(t *testing.T) {
	ctx := context.Background()
	_, alice, bob, _ := setup(t)
	bob.mustSet(0, 1, "8")
	bob.send()
	if st, _ := bob.d.Bus.State(ctx); st.Undo == "" {
		t.Fatal("pas d'historique")
	}
	// Écho de sa propre modification : rien n'est effacé.
	r, err := bob.e.ReceiveStructure(ctx, nil, map[int64]bool{tableID(t, bob.d): true})
	must(t, err)
	if st, _ := bob.d.Bus.State(ctx); st.Undo == "" || len(r.Notices) > 0 {
		t.Fatalf("relecture sans changement : %+v, %+v", st, r)
	}
	must(t, alice.header(2, "Tarif"))
	alice.send()
	r, err = bob.e.ReceiveStructure(ctx, nil, map[int64]bool{tableID(t, bob.d): true})
	must(t, err)
	if st, _ := bob.d.Bus.State(ctx); st.Undo != "" {
		t.Fatalf("historique gardé : %+v", st)
	}
	if !strings.Contains(strings.Join(r.Notices, " ; "), "historique d'annulation effacé") || !r.Changed() {
		t.Fatalf("rapport : %+v", r)
	}
	eq(t, "Bob", bob.col("Tarif"), []string{"4.5", "2.0", "3.25"})
	eq(t, "Bob Quantité", bob.col("Quantité"), []string{"8", "10", "1"})
}

func TestStructureIndexes(t *testing.T) {
	ctx := context.Background()
	_, alice, bob, _ := setup(t)
	x, err := alice.d.Indexer()
	must(t, err)
	tbl := tableID(t, alice.d)
	name, err := x.CreateIndex(ctx, tbl, []int64{colID(t, alice, "Ventes", "Produit"), colID(t, alice, "Ventes", "Prix")}, false)
	must(t, err)
	list, err := x.Indexes(ctx, tbl)
	must(t, err)
	if len(list) != 1 || list[0].Name != name || strings.Join(list[0].Columns, ",") != "Produit,Prix" {
		t.Fatalf("index : %+v", list)
	}
	if _, err := x.CreateIndex(ctx, tbl, []int64{colID(t, alice, "Ventes", "Quantité")}, true); err != nil {
		t.Fatalf("index unique : %v", err)
	}
	must(t, x.DropIndex(ctx, tbl, name))
	var re *collab.RefusedError
	if err := x.DropIndex(ctx, tbl, name); !errors.As(err, &re) || !strings.Contains(err.Error(), "introuvable sur ce tableau") {
		t.Fatalf("index retiré deux fois : %v", err)
	}
	// Pas d'index sur une colonne en double.
	must(t, alice.header(0, "Produit"))
	alice.mustSet(1, 0, "Café")
	alice.send()
	if _, err := x.CreateIndex(ctx, tbl, []int64{colID(t, alice, "Ventes", "Produit")}, true); !errors.As(err, &re) || !strings.Contains(err.Error(), "des valeurs en double") {
		t.Fatalf("index unique sur des doublons : %v", err)
	}
	bx, err := bob.d.Indexer()
	must(t, err)
	if _, err := bx.Indexes(ctx, tableID(t, bob.d)); !errors.Is(err, workbook.ErrDesign) {
		t.Fatalf("éditeur : %v", err)
	}
}
