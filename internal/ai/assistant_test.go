package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/command"
)

// apiError répond une erreur de l'API Anthropic.
func apiError(w http.ResponseWriter, status int, typ, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	b, _ := json.Marshal(map[string]any{"type": "error", "error": map[string]any{"type": typ, "message": msg}})
	w.Write(b)
}

func TestAnthropicErrors(t *testing.T) {
	var answer func(w http.ResponseWriter)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		answer(w)
	}))
	defer srv.Close()
	p := Lookup("anthropic")
	req := Request{Model: "claude-sonnet-5-5", Messages: []Message{{Role: User, Content: []Block{{Type: TextBlock, Text: "Bonjour"}}}}}
	cred := Credentials{Key: "sk-test", Endpoint: srv.URL}
	cases := []struct {
		status    int
		typ, msg  string
		want, not string
	}{
		{400, "invalid_request_error", "Your credit balance is too low to access the Anthropic API. Please go to Plans & Billing to upgrade or purchase credits.", "Crédits épuisés", ""},
		{400, "invalid_request_error", "prompt is too long: 1200000 tokens > 1000000 maximum", "Nouvelle conversation", ""},
		{401, "authentication_error", "invalid x-api-key", "Connexion", "Compte"},
		{429, "rate_limit_error", "slow down", "limite d'utilisation", ""},
	}
	for _, c := range cases {
		answer = func(w http.ResponseWriter) { apiError(w, c.status, c.typ, c.msg) }
		_, err := p.Turn(context.Background(), cred, req, func(Event) {})
		if err == nil || !strings.Contains(err.Error(), c.want) || (c.not != "" && strings.Contains(err.Error(), c.not)) {
			t.Errorf("%d %q : %v", c.status, c.msg, err)
		}
	}
	if !strings.Contains(p.Info().KeyURL, "platform.claude.com/settings/keys") || p.Info().DefaultModel != "claude-sonnet-5-5" || !p.Info().NeedsKey || len(p.Info().Models) != 3 {
		t.Fatalf("%+v", p.Info())
	}
}

func TestAnthropicCheckKey(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if r.Header.Get("X-Api-Key") != "sk-ant-bon" {
			apiError(w, 401, "authentication_error", "invalid x-api-key")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"data":[{"id":"claude-sonnet-5-5","type":"model","display_name":"Claude Sonnet 5.5","created_at":"2026-09-01T00:00:00Z"}],"has_more":false,"first_id":"claude-sonnet-5-5","last_id":"claude-sonnet-5-5"}`)
	}))
	kc := Lookup("anthropic").(KeyChecker)
	if err := kc.CheckKey(context.Background(), Credentials{Key: "sk-ant-bon", Endpoint: srv.URL}); err != nil || path != "/v1/models" {
		t.Fatalf("%v %s", err, path)
	}
	err := kc.CheckKey(context.Background(), Credentials{Key: "sk-ant-faux", Endpoint: srv.URL})
	if err == nil || errors.Is(err, ErrUnreachable) || !strings.Contains(err.Error(), "clé d'API refusée") {
		t.Fatalf("clé refusée : %v", err)
	}
	srv.Close()
	if err := kc.CheckKey(context.Background(), Credentials{Key: "sk-ant-bon", Endpoint: srv.URL}); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("injoignable : %v", err)
	}
}

func TestCost(t *testing.T) {
	u := Usage{InputTokens: 1_000_000 + 2_000_000 + 400_000, CacheReadTokens: 2_000_000, CacheWriteTokens: 400_000, OutputTokens: 100_000}
	// Sonnet 5.5 : 2 $ + 2 × 0,20 $ + 0,4 × 2,50 $ + 0,1 × 10 $.
	if c := Cost("claude-sonnet-5-5", u); math.Abs(c-4.4) > 1e-9 {
		t.Fatalf("Sonnet : %v", c)
	}
	if c := Cost("claude-opus-5-5", Usage{InputTokens: 1_000_000, OutputTokens: 1_000_000}); math.Abs(c-24) > 1e-9 {
		t.Fatalf("Opus : %v", c)
	}
	// Haiku : palier au-delà de 100 000 jetons de requête.
	if c := Cost("claude-haiku-5-5", Usage{InputTokens: 50_000}); math.Abs(c-0.005) > 1e-12 {
		t.Fatalf("Haiku : %v", c)
	}
	if c := Cost("claude-haiku-5-5", Usage{InputTokens: 200_000}); math.Abs(c-0.1) > 1e-12 {
		t.Fatalf("Haiku long : %v", c)
	}
	if Cost("modele-inconnu", u) != 0 {
		t.Fatal("tarif inventé")
	}
}

func TestSessionUsageAndToolEvents(t *testing.T) {
	h := newHost(t)
	f := &fakeProvider{replies: []Reply{
		{Stop: StopToolUse, Usage: Usage{InputTokens: 1000, OutputTokens: 100}, Message: Message{Role: Assistant, Content: []Block{
			{Type: ToolUseBlock, ID: "t1", Name: "columns_insert", Input: json.RawMessage(`{"table":"Ventes","name":"TVA"}`)},
			{Type: ToolUseBlock, ID: "t2", Name: "tables_list", Input: json.RawMessage(`{}`)},
			{Type: ToolUseBlock, ID: "t3", Name: "sheets_rename", Input: json.RawMessage(`{"sheet":"Feuil1","name":"Analyse"}`)},
		}}},
		{Stop: StopEnd, Usage: Usage{InputTokens: 2000, OutputTokens: 50}, Message: Message{Role: Assistant, Content: []Block{{Type: TextBlock, Text: "Fait."}}}},
	}}
	var s Session
	var events []Event
	turn := Turn{Provider: f, Model: "claude-sonnet-5-5", Tools: Tools(SchemaOnly), Exec: Exec(h, SchemaOnly), Label: Label, Write: IsWrite, Target: Target}
	ctx := command.WithGroup(context.Background(), "Assistant IA")
	if err := s.Send(ctx, turn, "Ajoute TVA", func(e Event) { events = append(events, e) }); err != nil {
		t.Fatal(err)
	}
	var usages int
	done := map[string]Event{}
	for _, e := range events {
		switch e.Kind {
		case EventUsage:
			usages++
			if e.Usage == nil || e.Usage.Cost <= 0 {
				t.Errorf("usage sans coût : %+v", e.Usage)
			}
		case EventToolDone:
			done[e.ID] = e
		}
	}
	if usages != 2 {
		t.Fatalf("%d évènements usage", usages)
	}
	if e := done["t1"]; !e.Write || e.Target != "Ventes" || e.Error {
		t.Errorf("t1 : %+v", e)
	}
	if e := done["t2"]; e.Write || e.Target != "" {
		t.Errorf("t2 : %+v", e)
	}
	if e := done["t3"]; !e.Write || e.Target != "Analyse" {
		t.Errorf("t3 : %+v", e)
	}
	u := s.Usage()
	if u.InputTokens != 3000 || u.OutputTokens != 150 || math.Abs(u.Cost-Cost("claude-sonnet-5-5", Usage{InputTokens: 3000, OutputTokens: 150})) > 1e-12 {
		t.Fatalf("cumul : %+v", u)
	}
	// Étapes du journal relevées pour l'annulation.
	if seqs := command.GroupSeqs(ctx); len(seqs) != 1 {
		t.Fatalf("étapes : %v", seqs)
	}
	// Snapshot : forme neutre ; Restore remplace et remet le cumul.
	hist, u2 := s.Snapshot()
	if len(hist) != 4 || u2 != u {
		t.Fatalf("snapshot : %d messages, %+v", len(hist), u2)
	}
	var r Session
	r.Restore(hist, u2)
	if r.Len() != 4 || r.Usage() != u {
		t.Fatal("restauration")
	}
	if err := r.Reset(); err != nil || r.Len() != 0 || r.Usage() != (Usage{}) {
		t.Fatal("remise à zéro")
	}
}

func TestSessionUnsentMergedRollback(t *testing.T) {
	f := &fakeProvider{
		replies: []Reply{{Stop: StopToolUse, Message: Message{Role: Assistant, Content: []Block{{Type: ToolUseBlock, ID: "t1", Name: "x"}}}}, {}},
		errs:    []error{nil, errors.New("panne")},
	}
	ctx, cancel := context.WithCancel(context.Background())
	var s Session
	turn := Turn{Provider: f, Exec: func(context.Context, string, json.RawMessage) (string, bool) {
		cancel()
		return "fait", false
	}}
	s.Send(ctx, turn, "un", func(Event) {})
	// Le message suivant rejoint le résultat d'outil, puis échoue : il est
	// retiré sans toucher au résultat.
	err := s.Send(context.Background(), turn, "deux", func(Event) {})
	if !Unsent(err) || err.Error() != "panne" {
		t.Fatalf("err %v", err)
	}
	hist, _ := s.Snapshot()
	if len(hist) != 3 || len(hist[2].Content) != 1 || hist[2].Content[0].Type != ToolResultBlock {
		t.Fatalf("conversation : %+v", hist)
	}
}

func TestShorten(t *testing.T) {
	big := strings.Repeat("x", 40_000)
	var s Session
	for i := range 5 {
		id := string(rune('a' + i))
		s.history = append(s.history,
			Message{Role: Assistant, Provider: "anthropic", Native: "natif", Content: []Block{{Type: ToolUseBlock, ID: id, Name: "tables_list"}}},
			Message{Role: User, Content: []Block{{Type: ToolResultBlock, ToolUseID: id, Text: big}}})
	}
	sent := s.history[1].Content // déjà rendu (requête en cours)
	if s.shorten(1_000_000) {
		t.Fatal("raccourci sous le seuil")
	}
	if !s.shorten(MaxHistoryChars) {
		t.Fatal("pas raccourci")
	}
	total := 0
	for _, m := range s.history {
		total += size(m)
	}
	if total > MaxHistoryChars*2/3 {
		t.Fatalf("encore %d caractères", total)
	}
	if s.history[1].Content[0].Text != OldResult || s.history[1].Content[0].ToolUseID != "a" {
		t.Fatalf("résultat ancien : %+v", s.history[1].Content[0])
	}
	if sent[0].Text != big {
		t.Fatal("copie déjà rendue modifiée")
	}
	// Les derniers messages restent entiers ; la forme native disparaît à
	// partir du premier message modifié, sauf pour le dernier message de
	// l'assistant (M7).
	if s.history[9].Content[0].Text != big {
		t.Fatal("dernier résultat retiré")
	}
	if s.history[0].Native == nil || s.history[2].Native != nil || s.history[6].Native != nil || s.history[8].Native == nil {
		t.Fatal("forme native")
	}

	// Entrées volumineuses des anciens appels raccourcies aussi (M7).
	var p Session
	huge := json.RawMessage(`{"values":"` + strings.Repeat("y", 60_000) + `"}`)
	for i := range 4 {
		id := string(rune('a' + i))
		p.history = append(p.history,
			Message{Role: Assistant, Content: []Block{{Type: ToolUseBlock, ID: id, Name: "cells_paste", Input: huge}}},
			Message{Role: User, Content: []Block{{Type: ToolResultBlock, ToolUseID: id, Text: "ok"}}})
	}
	if !p.shorten(MaxHistoryChars) || string(p.history[0].Content[0].Input) != oldInput || string(p.history[6].Content[0].Input) != string(huge) {
		t.Fatalf("entrées : %.40s… / %.40s…", p.history[0].Content[0].Input, p.history[6].Content[0].Input)
	}
}

func TestCompose(t *testing.T) {
	got := Compose("Analyse ce tableau", View{Sheet: "Ventes", Table: "Commandes", Selection: "Commandes[Montant][3:10]"}, []string{"L'utilisateur a annulé les modifications de votre réponse précédente."})
	want := "[Remarque : L'utilisateur a annulé les modifications de votre réponse précédente.]\n[Contexte : feuille active « Ventes », tableau « Commandes », sélection Commandes[Montant][3:10]]\n\nAnalyse ce tableau"
	if got != want {
		t.Fatalf("%q", got)
	}
	if Compose("Bonjour", View{}, nil) != "Bonjour" {
		t.Fatal("sans contexte")
	}
	if !strings.Contains(System(SchemaOnly), "[Contexte :") || !strings.Contains(System(SchemaOnly), "SOMME.SI(Commandes[IdClient]; [IdClient]; Commandes[Montant HT])") {
		t.Fatal("consigne")
	}
}

// addFormula ajoute au tableau une colonne calculée par formula.
func addFormula(t *testing.T, h *host, table, name, formula string) {
	t.Helper()
	ctx := context.Background()
	if _, err := api.Invoke(ctx, h, "columns.insert", map[string]any{"table": table, "name": name}); err != nil {
		t.Fatal(err)
	}
	info, err := api.Invoke(ctx, h, "tables.get", map[string]any{"table": table})
	if err != nil {
		t.Fatal(err)
	}
	var tb struct {
		Row, Col int
		Columns  []any
	}
	b, _ := json.Marshal(info)
	json.Unmarshal(b, &tb)
	if _, err := api.Invoke(ctx, h, "cells.set", map[string]any{"row": tb.Row + 1, "col": tb.Col + len(tb.Columns) - 1, "value": formula}); err != nil {
		t.Fatal(err)
	}
}

func TestFormulaErrors(t *testing.T) {
	h := newHost(t)
	ctx := context.Background()
	// Clients (Client, Région), clé sur Client, à côté de Ventes.
	rows := [][]any{{"Client", "Région"}}
	for i := range 6 { // C6 manque : #N/A dans REFERENCE
		rows = append(rows, []any{"C" + string(rune('0'+i)), "R" + string(rune('0'+i%2))})
	}
	if _, err := api.Invoke(ctx, h, "cells.paste", map[string]any{"row": 1, "col": 6, "values": rows}); err != nil {
		t.Fatal(err)
	}
	var clients string
	res, _ := api.Invoke(ctx, h, "tables.list", nil)
	var list []struct{ Name string }
	b, _ := json.Marshal(res)
	json.Unmarshal(b, &list)
	for _, tb := range list {
		if tb.Name != "Ventes" {
			clients = tb.Name
		}
	}
	if _, err := api.Invoke(ctx, h, "tables.rename", map[string]any{"table": clients, "name": "Clients"}); err != nil {
		t.Fatal(err)
	}
	if _, err := api.Invoke(ctx, h, "tables.set_key", map[string]any{"table": "Clients", "column": "Client"}); err != nil {
		t.Fatal(err)
	}
	// Exemples de la consigne : colonne calculée, REFERENCE et SOMME.SI
	// vers un autre tableau.
	addFormula(t, h, "Ventes", "Inverse", "=1/[Montant]")
	addFormula(t, h, "Ventes", "Région", "=REFERENCE(Clients[Région]; [Client])")
	addFormula(t, h, "Clients", "Total", "=SOMME.SI(Ventes[Client]; [Client]; Ventes[Montant])")
	addFormula(t, h, "Ventes", "Gros", "=[Montant]>100")

	exec := execRaw(h, SchemaOnly)
	out, isErr := exec(ctx, errorsTool, json.RawMessage(`{"table":"Ventes"}`))
	if isErr {
		t.Fatal(out)
	}
	var r struct {
		Columns []struct {
			Name, Formula, Result string
			Errors                map[string]int64
		}
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err, out)
	}
	got := map[string]int{}
	for i, c := range r.Columns {
		got[c.Name] = i
	}
	if len(r.Columns) != 3 {
		t.Fatalf("colonnes : %s", out)
	}
	inv, reg, big := r.Columns[got["Inverse"]], r.Columns[got["Région"]], r.Columns[got["Gros"]]
	// Montant vaut 0 à la première ligne ; C6 revient 4 fois sur 30 lignes.
	if inv.Errors["#DIV/0!"] != 1 || len(inv.Errors) != 1 || inv.Result != "nombre" {
		t.Errorf("Inverse : %+v", inv)
	}
	if reg.Errors["#N/A"] != 4 || reg.Result != "texte" {
		t.Errorf("Région : %+v", reg)
	}
	if len(big.Errors) != 0 || big.Result != "booléen" {
		t.Errorf("Gros : %+v", big)
	}
	if strings.Contains(out, "R0") || strings.Contains(out, "0,1") {
		t.Fatalf("valeur dans le résultat : %s", out)
	}
	out, isErr = exec(ctx, errorsTool, json.RawMessage(`{"table":"Clients"}`))
	if isErr || !strings.Contains(out, `"result":"nombre"`) || !strings.Contains(out, `"errors":{}`) {
		t.Fatalf("Clients : %s", out)
	}
	// Le profil donne aussi le type du résultat.
	out, isErr = exec(ctx, profileTool, json.RawMessage(`{"table":"Ventes"}`))
	if isErr || !strings.Contains(out, `"name":"Région","type":`) || !strings.Contains(out, `"result":"texte"`) {
		t.Fatalf("profil : %s", out)
	}
	if Label(errorsTool, json.RawMessage(`{"table":"Ventes"}`)) != "Vérifier les formules « Ventes »" {
		t.Fatal("libellé")
	}
}
