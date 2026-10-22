package canvas

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/work"
)

// phases relève les étapes d'une opération suivie (work), sans délai ni
// espacement : chaque changement arrive, avec son heure.
type phases struct {
	mu    sync.Mutex
	t0    time.Time
	steps []work.Step
	at    []time.Duration
	on    func(work.Step, *work.Tracker) // appelé à chaque étape (essais)
	tr    *work.Tracker
}

func instant(t *testing.T) {
	d, e := work.Delay, work.Every
	work.Delay, work.Every = 0, 0
	t.Cleanup(func() { work.Delay, work.Every = d, e })
}

func (p *phases) start(ctx context.Context, cancellable bool) context.Context {
	p.t0 = time.Now()
	ctx, p.tr = work.Start(ctx, func(s work.Step) {
		p.mu.Lock()
		p.steps = append(p.steps, s)
		p.at = append(p.at, time.Since(p.t0))
		on, tr := p.on, p.tr
		p.mu.Unlock()
		if on != nil {
			on(s, tr)
		}
	}, cancellable)
	return ctx
}

// summary : durée de chaque étape (« rewrite 1234 ms »), dans l'ordre.
func (p *phases) summary(total time.Duration) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var b strings.Builder
	type span struct {
		name string
		d    time.Duration
	}
	var spans []span
	for i, s := range p.steps {
		if s.End {
			break
		}
		end := total
		if i+1 < len(p.at) {
			end = p.at[i+1]
		}
		name := s.Phase
		if name == "" {
			name = "(début)"
		}
		if n := len(spans); n > 0 && spans[n-1].name == name {
			spans[n-1].d += end - p.at[i]
			continue
		}
		spans = append(spans, span{name, end - p.at[i]})
	}
	for _, s := range spans {
		if s.d < 20*time.Millisecond {
			continue
		}
		fmt.Fprintf(&b, " %s %d ms ;", s.name, s.d.Milliseconds())
	}
	return strings.TrimSuffix(b.String(), " ;")
}

func (p *phases) op() (string, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.steps {
		if s.Op != "" {
			return s.Op, s.Name
		}
	}
	return "", ""
}

// importRows importe le tableau Commandes de n lignes × 10 colonnes.
func importRows(t *testing.T, d *fixture, n int) int64 {
	t.Helper()
	ctx := context.Background()
	villes := []string{"Lyon", "Paris", "Nice", "Brest", "Lille", "Rome", "Milan", "Berlin", "Madrid", "Porto"}
	_, err := d.Import(ctx, "Importer", []ImportSheet{{Name: "Données", Tables: []ImportTable{{
		Name:    "Commandes",
		Headers: []string{"Client", "Ville", "Pays", "Prix", "Qté", "Date", "Remise", "Code", "Note", "Poids"},
		Types:   []grid.Type{grid.Text, grid.Text, grid.Text, grid.Decimal, grid.Integer, grid.Date, grid.Decimal, grid.Integer, grid.Text, grid.Decimal},
		Count:   n,
		Rows: func(yield func([]any) error) error {
			for i := range n {
				v := i % len(villes)
				if err := yield([]any{"Client " + strconv.Itoa(i%5000), villes[v], villes[(v+3)%len(villes)], float64(i%997) + 0.5, int64(i % 13),
					fmt.Sprintf("2026-%02d-%02d", i%12+1, i%28+1), float64(i%7) / 10, int64(i), "n" + strconv.Itoa(i%31), float64(i%101) * 1.5}); err != nil {
					return err
				}
			}
			return nil
		},
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	var sheet int64
	d.read(ctx, func(t *tx) error {
		return t.row(`SELECT sheet FROM _lattis_tables WHERE name = 'Commandes'`).Scan(&sheet)
	})
	return sheet
}

// shape : colonnes et empreinte des données du tableau Commandes.
func shape(t *testing.T, d *fixture, sheet int64) string {
	t.Helper()
	ctx := context.Background()
	l, err := d.Layout(ctx, sheet)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range l.Tables[0].Columns {
		names = append(names, c.Name)
	}
	var n int
	var sum float64
	var first string
	d.read(ctx, func(tx *tx) error {
		tb, err := tx.tableByName("Commandes")
		if err != nil {
			return err
		}
		if err := tx.row(`SELECT count(*), total(`+colSQL(tb.Cols[3].ID)+`) FROM `+tb.sql()).Scan(&n, &sum); err != nil {
			return err
		}
		return tx.row(`SELECT ` + colSQL(tb.Cols[0].ID) + ` FROM ` + tb.sql() + ` ORDER BY _pos LIMIT 1`).Scan(&first)
	})
	return fmt.Sprintf("%s | %d lignes | %.1f | %s", strings.Join(names, ","), n, sum, first)
}

// TestWorkCancelRollsBack : une opération interrompue en pleine réécriture
// (bouton « Annuler » de la barre d'état) laisse le classeur tel qu'avant,
// sans étape d'annulation ; la suivante se fait normalement.
func TestWorkCancelRollsBack(t *testing.T) {
	instant(t)
	ctx := context.Background()
	d := open(t)
	sheet := importRows(t, d, 3000)
	before := shape(t, d, sheet)
	ops := []struct {
		name  string
		phase string // l'interruption tombe au début de cette étape
		run   func(ctx context.Context) error
	}{
		{"suppression de colonne", "keep", func(ctx context.Context) error { return d.DeleteColumn(ctx, "Commandes", 3) }},
		{"suppression de colonnes", "rewrite", func(ctx context.Context) error { return d.SuppressColumns(ctx, "Commandes", 2, 3) }},
		{"tri", "rank", func(ctx context.Context) error { return d.SortTable(ctx, "Commandes", 3, true) }},
		{"type", "rewrite", func(ctx context.Context) error { return d.SetType(ctx, "Commandes", 4, grid.Text) }},
		{"collage", "cells", func(ctx context.Context) error {
			_, err := d.Paste(ctx, sheet, 1, 0, [][]string{{"a"}, {"b"}, {"c"}})
			return err
		}},
		{"enregistrement", "commit", func(ctx context.Context) error { return d.InsertColumn(ctx, "Commandes", 2, "Nouvelle") }},
	}
	for _, op := range ops {
		var p phases
		p.on = func(s work.Step, tr *work.Tracker) {
			if s.Phase == op.phase {
				tr.Cancel()
			}
		}
		wctx := p.start(ctx, true)
		err := op.run(wctx)
		cancelled := p.tr.Cancelled()
		p.tr.End()
		if err == nil || !cancelled {
			t.Fatalf("%s : pas interrompue (%v), étapes %+v", op.name, err, p.steps)
		}
		if got := shape(t, d, sheet); got != before {
			t.Fatalf("%s interrompue : classeur modifié\navant : %s\naprès : %s", op.name, before, got)
		}
		if st, _ := d.bus.State(ctx); st.Undo != "" && st.Undo != "Importer" {
			t.Fatalf("%s interrompue : étape d'annulation %q", op.name, st.Undo)
		}
	}
	// Sans interruption : faite, et l'étape d'avant n'a rien laissé.
	if err := d.DeleteColumn(ctx, "Commandes", 3); err != nil {
		t.Fatal(err)
	}
	if got := shape(t, d, sheet); got == before || !strings.Contains(got, "Client,Ville,Pays,Qté") {
		t.Fatalf("suppression après les interruptions : %s", got)
	}
}

// TestWorkSteps : les opérations nomment leur action (avec la colonne
// visée) et leurs étapes ; rien n'est émis pour une opération plus courte
// que Delay.
func TestWorkSteps(t *testing.T) {
	instant(t)
	ctx := context.Background()
	d := open(t)
	importRows(t, d, 500)
	var p phases
	if err := d.DeleteColumn(p.start(ctx, true), "Commandes", 3); err != nil {
		t.Fatal(err)
	}
	p.tr.End()
	if op, name := p.op(); op != "deleteColumn" || name != "Prix" {
		t.Fatalf("action : %q %q", op, name)
	}
	seen := map[string]bool{}
	for _, s := range p.steps {
		seen[s.Phase] = true
	}
	for _, want := range []string{"keep", "rewrite", "journal", "commit"} {
		if !seen[want] {
			t.Errorf("étape %s absente : %+v", want, p.steps)
		}
	}
	if last := p.steps[len(p.steps)-1]; !last.End {
		t.Fatalf("pas de fin : %+v", last)
	}
	// Opération courte, délai ordinaire : rien n'est émis.
	work.Delay = time.Hour
	var q phases
	if err := d.InsertColumn(q.start(ctx, true), "Commandes", 0, ""); err != nil {
		t.Fatal(err)
	}
	q.tr.End()
	if len(q.steps) != 0 {
		t.Fatalf("opération courte montrée : %+v", q.steps)
	}
}

// TestMesureRetourVisuel chronomètre, étape par étape, les opérations
// lourdes d'un grand tableau de 10 colonnes (barre d'état des gros
// classeurs) :
// LATTIS_MESURE=1000000 go test ./internal/canvas -run TestMesureRetourVisuel -v -timeout 30m
func TestMesureRetourVisuel(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("LATTIS_MESURE"))
	if n == 0 {
		t.Skip("LATTIS_MESURE non défini")
	}
	instant(t)
	ctx := context.Background()
	d := open(t)
	start := time.Now()
	sheet := importRows(t, d, n)
	t.Logf("import de %d lignes × 10 colonnes : %d ms", n, time.Since(start).Milliseconds())
	mesure := func(nom string, fn func(ctx context.Context) error) {
		t.Helper()
		var p phases
		wctx := p.start(ctx, true)
		start := time.Now()
		err := fn(wctx)
		total := time.Since(start)
		p.tr.End()
		if err != nil {
			t.Fatalf("%s : %v", nom, err)
		}
		op, name := p.op()
		t.Logf("%-34s %7d ms  [%s %s]%s", nom, total.Milliseconds(), op, name, p.summary(total))
	}
	mesure("mise en page", func(ctx context.Context) error { _, err := d.Layout(ctx, sheet); return err })
	// Mise en page après chaque opération de structure : le contrôle de la
	// clé et le comptage des lignes ne sont refaits que si les lignes ou la
	// première colonne changent.
	layout := func(nom string) {
		t.Helper()
		mesure("  mise en page après "+nom, func(ctx context.Context) error { _, err := d.Layout(ctx, sheet); return err })
	}
	mesure("insérer une colonne", func(ctx context.Context) error { return d.InsertColumn(ctx, "Commandes", 3, "Nouvelle") })
	layout("l'insertion")
	mesure("supprimer une colonne vide", func(ctx context.Context) error { return d.DeleteColumn(ctx, "Commandes", 3) })
	layout("la suppression")
	mesure("supprimer la colonne Prix", func(ctx context.Context) error { return d.DeleteColumn(ctx, "Commandes", 3) })
	mesure("mise en page après", func(ctx context.Context) error { _, err := d.Layout(ctx, sheet); return err })
	mesure("annuler la suppression", func(ctx context.Context) error { _, err := d.bus.Undo(ctx); return err })
	layout("l'annulation")
	mesure("supprimer 2 colonnes (Prix, Qté)", func(ctx context.Context) error { return d.SuppressColumns(ctx, "Commandes", 3, 4) })
	layout("la suppression de 2 colonnes")
	mesure("annuler", func(ctx context.Context) error { _, err := d.bus.Undo(ctx); return err })
	layout("l'annulation")
	mesure("trier par Prix", func(ctx context.Context) error { return d.SortTable(ctx, "Commandes", 3, true) })
	layout("le tri")
	mesure("annuler le tri", func(ctx context.Context) error { _, err := d.bus.Undo(ctx); return err })
	layout("l'annulation du tri")
	mesure("type de Qté : texte", func(ctx context.Context) error { return d.SetType(ctx, "Commandes", 4, grid.Text) })
	layout("le type")
	mesure("annuler le type", func(ctx context.Context) error { _, err := d.bus.Undo(ctx); return err })
	layout("l'annulation du type")
	mesure("insérer une ligne au milieu", func(ctx context.Context) error { return d.InsertRow(ctx, "Commandes", n/2) })
	mesure("supprimer des lignes (10 lignes)", func(ctx context.Context) error { return d.DeleteSheetRows(ctx, sheet, 1000, 1009) })
	mesure("coller 1 000 × 2 dans le tableau", func(ctx context.Context) error {
		block := make([][]string, 1000)
		for i := range block {
			block[i] = []string{"x" + strconv.Itoa(i), "y"}
		}
		_, err := d.Paste(ctx, sheet, 2000, 1, block)
		return err
	})
	mesure("mise en forme de 10 000 cellules", func(ctx context.Context) error {
		return d.ApplyStyle(ctx, sheet, 1, 3, min(n, 10000), 3, Style{"bold": true})
	})
	mesure("colonne calculée [@Prix]*[@Qté]", func(ctx context.Context) error {
		_, err := d.SetCell(ctx, sheet, 0, 10, "Total")
		if err == nil {
			_, err = d.SetCell(ctx, sheet, 1, 10, "=[@Prix]*[@Qté]")
		}
		return err
	})
	mesure("mise en page avec colonne calculée", func(ctx context.Context) error { _, err := d.Layout(ctx, sheet); return err })
}
