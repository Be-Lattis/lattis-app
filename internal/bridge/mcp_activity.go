package bridge

import (
	"context"
	"sort"
	"time"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/mcp"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/sharing"
)

// Retour visible des clients MCP (Claude Desktop, Claude Code…) : chaque
// appel d'outil est montré dans Lattis (évènement mcp:activity), les
// écritures d'une série d'appels forment une seule étape d'annulation
// (« Claude Desktop (MCP) »), annulable d'un clic tant qu'aucune autre
// action ne l'a suivie, et le réglage mcp.confirmWrites fait attendre
// l'accord de l'utilisateur avant chaque écriture.

// Évènements du serveur MCP.
const (
	// MCPActivityEvent : un appel d'outil s'est terminé (MCPActivity).
	MCPActivityEvent = "mcp:activity"
	// MCPConfirmEvent : un outil d'écriture attend l'accord (MCPConfirm).
	MCPConfirmEvent = "mcp:confirm"
	// MCPConfirmDoneEvent : la demande id ({id}) est close (réponse,
	// délai dépassé, client parti) : la boîte d'accord la retire.
	MCPConfirmDoneEvent = "mcp:confirm-done"
	// MCPClientEvent : un client s'est présenté ({client}, nom affiché).
	MCPClientEvent = "mcp:client"
)

// MCPActivity est un appel d'outil d'un client MCP, pour le fil d'activité.
type MCPActivity struct {
	ID     int64  `json:"id"`
	Series int    `json:"series"` // série d'appels (voir mcpSeriesGap)
	Client string `json:"client"` // nom affiché (« Claude Desktop »)
	Tool   string `json:"tool"`
	Label  string `json:"label"`
	Write  bool   `json:"write"` // l'outil peut modifier le classeur
	Error  bool   `json:"error"`
	Detail string `json:"detail"` // message d'erreur, court
	Target string `json:"target"` // feuille, tableau ou nom visé
	At     string `json:"at"`     // RFC 3339
}

// MCPConfirm est une demande d'accord avant une écriture.
type MCPConfirm struct {
	ID     int64  `json:"id"`
	Client string `json:"client"`
	Label  string `json:"label"`
	Tool   string `json:"tool"`
}

// MCPSeriesState dit si une série peut être annulée d'un clic.
type MCPSeriesState struct {
	ID     int    `json:"id"`
	Client string `json:"client"`
	Writes int    `json:"writes"` // étapes du journal encore en place
	Undo   bool   `json:"undo"`
	Reason string `json:"reason"` // pourquoi pas, sinon ""
}

// mcpSeriesGap : au-delà de ce silence, les appels d'un client ouvrent une
// nouvelle série (une autre demande faite dans Claude).
const mcpSeriesGap = 2 * time.Minute

// mcpConfirmTimeout : sans réponse de l'utilisateur, l'écriture est refusée.
var mcpConfirmTimeout = 2 * time.Minute

// mcpConnectedFor : un client vu depuis moins longtemps est « connecté ».
const mcpConnectedFor = 30 * time.Minute

// mcpConfirmWrites lit le réglage mcp.confirmWrites ; surchargeable pour
// les tests.
var mcpConfirmWrites = func() bool { return config.Current().Bool("mcp.confirmWrites", false) }

// mcpSharing lit le partage des données de l'assistant (ai.dataSharing),
// qui s'applique aussi à Claude par MCP ; surchargeable pour les tests.
var mcpSharing = func() string { return config.Current().String("ai.dataSharing", sharing.SchemaOnly) }

// Erreurs rendues au client MCP (résultat d'outil en erreur).
var (
	errMCPRefused = msg.New("mcp.refused", nil)
	errMCPTimeout = msg.New("mcp.timeout", nil)
	errMCPNoUI    = msg.New("mcp.noWindow", nil)
)

// mcpSeries est une suite d'appels d'un client sans pause de plus de
// mcpSeriesGap, sur un même classeur.
type mcpSeries struct {
	id     int
	client string
	last   time.Time
	doc    *doc.Document
	// group porte le groupe d'annulation de la série (command.WithGroup) :
	// ses écritures s'ajoutent à la même étape du journal.
	group context.Context
	// seqs : étapes du journal créées pendant la série (une seule en
	// général ; plusieurs si l'utilisateur a agi entre deux appels, ou pour
	// un script, qui a son propre groupe).
	seqs   []int64
	closed bool // annulée par MCPUndoSeries
}

// mcpWatch est l'état du suivi (dans mcpState).
type mcpWatch struct {
	nextSeries int
	nextID     int64
	current    map[string]*mcpSeries // série en cours, par client
	series     map[int]*mcpSeries
	confirms   map[int64]chan bool
	seen       string // dernier client vu (nom affiché)
	seenAt     time.Time
}

// valuesCtx garde l'annulation et l'échéance de la requête et trouve en
// plus les valeurs de extra (groupe d'annulation de la série).
type valuesCtx struct {
	context.Context
	extra context.Context
}

func (c valuesCtx) Value(k any) any {
	if v := c.Context.Value(k); v != nil {
		return v
	}
	return c.extra.Value(k)
}

func (s *DocumentService) mcpServer() *mcp.Server {
	return &mcp.Server{Host: host{s}, Name: "lattis", Version: Version, Watch: s.mcpWatchCall, Connect: s.mcpConnect, Sharing: mcpSharing}
}

// mcpConnect note le client qui vient de se présenter.
func (s *DocumentService) mcpConnect(client string) {
	name := mcp.ClientLabel(client)
	s.mcp.mu.Lock()
	s.mcp.w.seen, s.mcp.w.seenAt = name, time.Now()
	s.mcp.mu.Unlock()
	s.sys().Emit(MCPClientEvent, map[string]string{"client": name})
}

// mcpSeriesFor rend la série en cours du client, ou en ouvre une.
func (s *DocumentService) mcpSeriesFor(client string, d *doc.Document) *mcpSeries {
	w := &s.mcp.w
	now := time.Now()
	if w.current == nil {
		w.current, w.series = map[string]*mcpSeries{}, map[int]*mcpSeries{}
	}
	if ser := w.current[client]; ser != nil && !ser.closed && ser.doc == d && now.Sub(ser.last) <= mcpSeriesGap {
		ser.last = now
		return ser
	}
	w.nextSeries++
	ser := &mcpSeries{id: w.nextSeries, client: client, last: now, doc: d, group: withUndoGroup(client + " (MCP)")}
	w.current[client] = ser
	w.series[ser.id] = ser
	return ser
}

// mcpWatchCall entoure chaque appel d'outil d'un client MCP.
func (s *DocumentService) mcpWatchCall(ctx context.Context, c mcp.Call, run func(context.Context) (any, error)) (any, error) {
	client := mcp.ClientLabel(c.Client)
	d, _ := s.current()
	s.mcp.mu.Lock()
	ser := s.mcpSeriesFor(client, d)
	s.mcp.w.nextID++
	id := s.mcp.w.nextID
	s.mcp.w.seen, s.mcp.w.seenAt = client, time.Now()
	s.mcp.mu.Unlock()

	act := MCPActivity{ID: id, Series: ser.id, Client: client, Tool: c.Tool, Label: c.Label, Write: c.Write, Target: c.Target}
	var res any
	var err error
	if c.Write && mcpConfirmWrites() {
		err = s.mcpAsk(ctx, MCPConfirm{ID: id, Client: client, Label: c.Label, Tool: c.Tool})
	}
	if err == nil {
		before := journalTop(ctx, d)
		res, err = run(valuesCtx{ctx, ser.group})
		if d != nil {
			added := journalAfter(ctx, d, before)
			s.mcp.mu.Lock()
			ser.seqs = append(ser.seqs, added...)
			ser.last = time.Now()
			s.mcp.mu.Unlock()
		}
	}
	if err != nil {
		act.Error, act.Detail = true, shortError(err.Error())
	}
	act.At = time.Now().Format(time.RFC3339)
	s.sys().Emit(MCPActivityEvent, act)
	return res, err
}

func shortError(msg string) string {
	const max = 200
	r := []rune(msg)
	if len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return msg
}

// mcpAsk attend l'accord de l'utilisateur (MCPAnswer) : refus, délai
// dépassé, client parti ou fenêtre absente rendent une erreur.
func (s *DocumentService) mcpAsk(ctx context.Context, q MCPConfirm) error {
	ch := make(chan bool, 1)
	s.mcp.mu.Lock()
	if s.mcp.w.confirms == nil {
		s.mcp.w.confirms = map[int64]chan bool{}
	}
	s.mcp.w.confirms[q.ID] = ch
	s.mcp.mu.Unlock()
	defer func() {
		s.mcp.mu.Lock()
		delete(s.mcp.w.confirms, q.ID)
		s.mcp.mu.Unlock()
		s.sys().Emit(MCPConfirmDoneEvent, map[string]int64{"id": q.ID})
	}()
	if !s.sys().Emit(MCPConfirmEvent, q) {
		return errMCPNoUI
	}
	t := time.NewTimer(mcpConfirmTimeout)
	defer t.Stop()
	select {
	case ok := <-ch:
		if !ok {
			return errMCPRefused
		}
		return nil
	case <-t.C:
		return errMCPTimeout
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

// MCPAnswer répond à une demande d'accord (évènement mcp:confirm) : allow
// laisse l'outil s'exécuter. Sans effet si la demande est déjà close.
func (s *DocumentService) MCPAnswer(id int64, allow bool) {
	s.mcp.mu.Lock()
	ch := s.mcp.w.confirms[id]
	s.mcp.mu.Unlock()
	if ch != nil {
		select {
		case ch <- allow:
		default:
		}
	}
}

// journalTop est la dernière étape active du journal (0 sans classeur).
func journalTop(ctx context.Context, d *doc.Document) int64 {
	if d == nil {
		return 0
	}
	var top int64
	d.DB().Read().QueryRowContext(ctx, `SELECT coalesce(max(seq), 0) FROM _lattis_journal WHERE undone = 0`).Scan(&top)
	return top
}

// journalAfter liste les étapes actives créées après top. Une nouvelle
// étape a toujours un numéro plus grand que les étapes actives (les étapes
// annulées, effacées par toute nouvelle action, peuvent voir leur numéro
// repris : on ne compare qu'aux actives).
func journalAfter(ctx context.Context, d *doc.Document, top int64) []int64 {
	rows, err := d.DB().Read().QueryContext(ctx, `SELECT seq FROM _lattis_journal WHERE undone = 0 AND seq > ? ORDER BY seq`, top)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var n int64
		if rows.Scan(&n) == nil {
			out = append(out, n)
		}
	}
	return out
}

// seriesUndo dit combien d'étapes de la série sont encore en place et si
// elles sont les dernières du journal ("" : annulable).
func seriesUndo(ctx context.Context, ser *mcpSeries, d *doc.Document) (int, msg.Message) {
	if ser.closed {
		return 0, msg.M("mcp.undoAlreadyDone", nil)
	}
	if d == nil || ser.doc != d {
		return 0, msg.M("mcp.undoWorkbookChanged", nil)
	}
	mine := map[int64]bool{}
	for _, n := range ser.seqs {
		mine[n] = true
	}
	rows, err := d.DB().Read().QueryContext(ctx, `SELECT seq FROM _lattis_journal WHERE undone = 0 ORDER BY seq DESC`)
	if err != nil {
		return 0, msg.M("mcp.undoFailed", msg.Args{"err": err})
	}
	defer rows.Close()
	var active []int64
	for rows.Next() {
		var n int64
		if rows.Scan(&n) == nil {
			active = append(active, n)
		}
	}
	left := 0
	for _, n := range active {
		if mine[n] {
			left++
		}
	}
	if left == 0 {
		if len(ser.seqs) == 0 {
			return 0, msg.M("mcp.undoNothing", nil)
		}
		return 0, msg.M("mcp.undoAlreadyDone", nil)
	}
	for i := 0; i < left; i++ {
		if !mine[active[i]] {
			return left, msg.M("mcp.undoLater", nil)
		}
	}
	return left, msg.Message{}
}

// MCPSeries dit, pour chaque série connue, si elle peut être annulée d'un
// clic (bouton « Annuler ces modifications »), de la plus récente à la plus
// ancienne.
func (s *DocumentService) MCPSeries(ctx context.Context) []MCPSeriesState {
	d, _ := s.current()
	s.mcp.mu.Lock()
	list := make([]*mcpSeries, 0, len(s.mcp.w.series))
	for _, ser := range s.mcp.w.series {
		list = append(list, ser)
	}
	s.mcp.mu.Unlock()
	sort.Slice(list, func(i, j int) bool { return list[i].id > list[j].id })
	out := make([]MCPSeriesState, 0, len(list))
	for _, ser := range list {
		s.mcp.mu.Lock()
		n, why := seriesUndo(ctx, ser, d)
		s.mcp.mu.Unlock()
		reason := ""
		if why.ID != "" {
			reason = why.Text(s.sessionLang())
		}
		out = append(out, MCPSeriesState{ID: ser.id, Client: ser.client, Writes: n, Undo: why.ID == "", Reason: reason})
	}
	return out
}

// MCPUndoSeries annule les modifications d'une série d'appels MCP, si ce
// sont les dernières étapes du journal ; sinon une erreur dit pourquoi.
func (s *DocumentService) MCPUndoSeries(ctx context.Context, series int) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	s.mcp.mu.Lock()
	ser := s.mcp.w.series[series]
	s.mcp.mu.Unlock()
	if ser == nil {
		return DocState{}, msg.New("mcp.unknownBatch", nil)
	}
	s.mcp.mu.Lock()
	n, why := seriesUndo(ctx, ser, d)
	s.mcp.mu.Unlock()
	if why.ID != "" {
		return DocState{}, msg.New(why.ID, why.Args)
	}
	for i := 0; i < n; i++ {
		if _, err := d.Bus.Undo(ctx); err != nil {
			s.changed(ctx)
			return DocState{}, err
		}
	}
	s.mcp.mu.Lock()
	ser.closed = true
	s.mcp.mu.Unlock()
	d.Notify(collab.UndoNoticeIn(s.sessionLang(), d.Oplog, false))
	return s.changed(ctx)
}

// mcpConnected rend le client vu récemment ("" sinon).
func (s *DocumentService) mcpConnected() string {
	s.mcp.mu.Lock()
	defer s.mcp.mu.Unlock()
	if s.mcp.w.seen != "" && time.Since(s.mcp.w.seenAt) <= mcpConnectedFor {
		return s.mcp.w.seen
	}
	return ""
}

// withUndoGroup ouvre le groupe d'annulation d'une série, gardé d'un appel à
// l'autre (valuesCtx le joint au contexte de chaque requête).
func withUndoGroup(name string) context.Context {
	return command.WithGroup(context.Background(), name)
}
