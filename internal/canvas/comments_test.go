package canvas

import (
	"context"
	"errors"
	"testing"
)

// threadAt rend le fil de la cellule (row, col) de la feuille 1 (nil : aucun).
func threadIn(t *testing.T, d *fixture, row, col int) *Thread {
	t.Helper()
	th, err := d.Threads(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := range th {
		if th[i].Row == row && th[i].Col == col {
			return &th[i]
		}
	}
	return nil
}

func TestCommentsFreeCell(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	alice := Who{Name: "Alice", ID: "local:alice"}
	bob := Who{Name: "Bob", ID: "local:bob"}
	before := dump(t, d)
	first, err := d.AddComment(ctx, 1, 4, 2, "  À vérifier @Bob\r\n", alice)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.AddComment(ctx, 1, 4, 2, "C'est fait", bob); err != nil {
		t.Fatal(err)
	}
	th := threadIn(t, d, 4, 2)
	if th == nil || th.ID != first || len(th.Comments) != 2 || th.Comments[0].Text != "À vérifier @Bob" || th.Comments[1].Author != "Bob" || th.SheetName != "Feuille1" {
		t.Fatalf("fil : %+v", th)
	}
	// Vide, et pas le sien.
	if _, err := d.AddComment(ctx, 1, 4, 2, " \n", alice); !errors.Is(err, errCommentEmpty) {
		t.Fatalf("commentaire vide : %v", err)
	}
	if err := d.EditComment(ctx, th.Comments[1].ID, "Non", alice); !errors.Is(err, errCommentNotYours) {
		t.Fatalf("modifier le message d'un autre : %v", err)
	}
	if err := d.EditComment(ctx, th.Comments[1].ID, "Fait hier", bob); err != nil {
		t.Fatal(err)
	}
	if err := d.ResolveThread(ctx, first, true); err != nil {
		t.Fatal(err)
	}
	if th = threadIn(t, d, 4, 2); !th.Resolved || th.Comments[1].Text != "Fait hier" || th.Comments[1].Edited == "" {
		t.Fatalf("fil résolu, réponse modifiée : %+v", th)
	}
	// Insérer deux lignes au-dessus : le fil descend ; une colonne avant : il glisse.
	if err := d.InsertSheetRows(ctx, 1, 0, 2); err != nil {
		t.Fatal(err)
	}
	if err := d.InsertSheetColumns(ctx, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if threadIn(t, d, 6, 3) == nil {
		t.Fatal("le fil n'a pas suivi l'insertion")
	}
	// Déplacer la cellule (couper-coller) : le fil suit.
	if err := d.MoveCells(ctx, 1, 6, 3, 6, 3, 2, 1); err != nil {
		t.Fatal(err)
	}
	if threadIn(t, d, 8, 4) == nil || threadIn(t, d, 6, 3) != nil {
		t.Fatal("le fil n'a pas suivi le déplacement")
	}
	// Supprimer la ligne : le fil part, messages compris.
	if err := d.DeleteSheetRows(ctx, 1, 8, 8); err != nil {
		t.Fatal(err)
	}
	var n int
	d.db.Read().QueryRow(`SELECT count(*) FROM _lattis_comments`).Scan(&n)
	if threadIn(t, d, 8, 4) != nil || n != 0 {
		t.Fatalf("fil gardé après la suppression de sa ligne (%d messages)", n)
	}
	check(t, d, "commentaires")
	// Chaque action est une étape : tout s'annule.
	for range 8 {
		if _, err := d.bus.Undo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestCommentsTableRow(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pop"}, {"Lyon", "520"}, {"Paris", "2100"}, {"Nice", "340"}}); err != nil {
		t.Fatal(err)
	}
	me := Who{Name: "Moi", ID: "local:moi"}
	// Paris, colonne Pop (ligne 2) ; et l'en-tête Ville.
	id, err := d.AddComment(ctx, 1, 2, 1, "Chiffre 2021", me)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.AddComment(ctx, 1, 0, 0, "Nom officiel ?", me); err != nil {
		t.Fatal(err)
	}
	var paris int64
	d.db.Read().QueryRow(`SELECT id FROM t_1 WHERE c_1 = 'Paris'`).Scan(&paris)
	var rid int64
	d.db.Read().QueryRow(`SELECT rid FROM _lattis_comment_rows WHERE thread = ?`, id).Scan(&rid)
	if rid != paris {
		t.Fatalf("ancré à la ligne %d, Paris est %d", rid, paris)
	}
	// Tri décroissant par Pop : Paris passe en tête, son fil la suit.
	if err := d.SortTable(ctx, "Feuille1", 1, true); err != nil {
		t.Fatal(err)
	}
	if th := threadIn(t, d, 1, 1); th == nil || th.ID != id || th.Table != "Feuille1" {
		t.Fatalf("fil après le tri : %+v", th)
	}
	if threadIn(t, d, 0, 0) == nil {
		t.Fatal("fil de l'en-tête perdu")
	}
	// Filtre qui masque Paris : son fil n'est plus montré.
	if err := d.SetFilter(ctx, "Feuille1", 0, &ColumnFilter{Values: []string{"Lyon", "Nice"}}); err != nil {
		t.Fatal(err)
	}
	th, _ := d.Threads(ctx, 1)
	for _, x := range th {
		if x.ID == id {
			t.Fatalf("fil d'une ligne masquée montré : %+v", x)
		}
	}
	if err := d.SetFilter(ctx, "Feuille1", 0, nil); err != nil {
		t.Fatal(err)
	}
	// Supprimer le fil par sa cellule, puis l'annuler.
	if err := d.DeleteThread(ctx, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if threadIn(t, d, 1, 1) != nil {
		t.Fatal("fil supprimé encore là")
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if th := threadIn(t, d, 1, 1); th == nil || len(th.Comments) != 1 {
		t.Fatalf("fil rendu par l'annulation : %+v", th)
	}
	// Supprimer le premier message emporte le fil.
	if err := d.DeleteComment(ctx, id, me); err != nil {
		t.Fatal(err)
	}
	if threadIn(t, d, 1, 1) != nil {
		t.Fatal("le premier message supprimé n'a pas emporté le fil")
	}
	check(t, d, "commentaires d'un tableau")
}

// Une feuille supprimée emporte ses fils (une feuille recréée sous le même
// identifiant ne les retrouve pas) ; l'annulation les rend.
func TestCommentsSheetDeleted(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	sheet, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.AddComment(ctx, sheet, 1, 1, "Bonjour", Who{Name: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteSheet(ctx, sheet); err != nil {
		t.Fatal(err)
	}
	all, err := d.Threads(ctx, 0)
	if err != nil || len(all) != 0 {
		t.Fatalf("fils après la suppression de la feuille : %v %+v", err, all)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if all, _ = d.Threads(ctx, 0); len(all) != 1 || all[0].Sheet != sheet {
		t.Fatalf("fil rendu avec la feuille : %+v", all)
	}
}
