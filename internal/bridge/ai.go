package bridge

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/ai"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/collab"
)

// Assistant ✦ : conversation avec un assistant qui agit sur le classeur par
// les commandes de internal/api (voir internal/ai). Opt-in (ai.enabled) ;
// fournisseur, modèle et partage des données dans la configuration de
// l'utilisateur, clés d'API dans le trousseau du système.
//
// Chaque réponse forme une étape d'annulation « Assistant IA » (plusieurs
// si l'utilisateur agit pendant la réponse) ; leurs numéros dans le journal
// (_lattis_journal) sont relevés pour annuler la réponse (AIUndoRun) et pour
// dire au modèle, au message suivant, que l'utilisateur a annulé ses
// modifications.
//
// La conversation appartient au classeur : ouvrir, créer ou fermer un
// classeur la remet à zéro, ou la restaure quand le réglage
// ai.keepConversation l'a gardée dans le fichier (_lattis_info, clé
// aiConversationKey, écrite à l'enregistrement), et émet ai:reset.

// AIEvent porte le déroulé d'une réponse (texte au fil de l'eau, outils).
const AIEvent = "ai:event"

// AIResetEvent : la conversation a changé hors de l'interface (classeur
// ouvert, nouveau, fermé) ; l'interface relit AITranscript et AIState.
const AIResetEvent = "ai:reset"

// aiUndoGroup nomme l'étape d'annulation d'une réponse.
const aiUndoGroup = "Assistant IA"

// aiConversationKey : clé de la conversation gardée dans _lattis_info.
const aiConversationKey = "ai.conversation"

// AIEventData est un évènement d'une réponse ; Run la désigne.
type AIEventData struct {
	Run int `json:"run"`
	ai.Event
}

// AIContext : ce que l'utilisateur regarde, joint à chaque message.
type AIContext struct {
	Sheet     string `json:"sheet"`     // feuille active ("" : aucune)
	Table     string `json:"table"`     // tableau sous le curseur ("" sinon)
	Selection string `json:"selection"` // sélection lisible : « Commandes[Montant][3:10] » ou « B2:D5 » ("" sinon)
}

// AIUsage : jetons et coût (dollars US) de la conversation.
type AIUsage = ai.Usage

// AILogEntry est un élément de la conversation affichée, dans l'ordre :
// message de l'utilisateur (User) ou évènement d'une réponse (Event).
// L'interface la rejoue pour réafficher la conversation.
type AILogEntry struct {
	Run   int          `json:"run"`
	User  string       `json:"user,omitempty"`
	Event *AIEventData `json:"event,omitempty"`
}

// aiRun : ce qu'une réponse a fait au classeur.
type aiRun struct {
	seqs    []int64 // étapes du journal créées par la réponse
	changes int     // appels d'outils d'écriture réussis
	// undone : annulée (AIUndoRun) ou déjà signalée au modèle comme
	// annulée par l'utilisateur.
	undone bool
}

type aiState struct {
	mu      sync.Mutex
	session ai.Session
	cancel  context.CancelFunc
	log     []AILogEntry
	runs    map[int]*aiRun
	order   []int    // réponses dans l'ordre (pour repérer les annulations)
	notes   []string // remarques à joindre au prochain message
	gen     int      // change à chaque remise à zéro (classeur remplacé)
	// structure : la structure du classeur telle que le modèle la connaît
	// (relevée à la fin de chaque réponse) ; une autre au message suivant
	// lui est jointe (ai.StructureUpdate). Vide : à joindre de nouveau.
	structure string
	// others : conversations mises de côté ; nextConv : dernier identifiant.
	others   []aiConv
	nextConv int
	// listing : fournisseurs dont la liste des modèles est en cours de
	// lecture (refreshModels).
	listing map[string]bool
}

// AIProvider est un fournisseur pour l'interface.
type AIProvider struct {
	ai.Info
	HasKey bool `json:"hasKey"`
}

// AIStatus décrit l'assistant.
type AIStatus struct {
	Enabled bool `json:"enabled"`
	// Locked : coupé ou imposé par une stratégie de l'organisation.
	Locked       bool         `json:"locked"`
	Provider     string       `json:"provider"`
	Model        string       `json:"model"`
	DataSharing  string       `json:"dataSharing"`
	Managed      bool         `json:"managed"`      // passerelle de l'entreprise (ai.endpoint)
	GatewayNoKey bool         `json:"gatewayNoKey"` // passerelle sans clé personnelle (ai.auth.mode = none)
	Providers    []AIProvider `json:"providers"`
	Busy         bool         `json:"busy"`
	Messages     int          `json:"messages"`
	// Usage : cumul des jetons et du coût de la conversation.
	Usage AIUsage `json:"usage"`
	// KeepConversation : la conversation est gardée avec le classeur
	// (réglage ai.keepConversation).
	KeepConversation bool `json:"keepConversation"`
}

// AIState décrit l'assistant : réglages, fournisseurs, clés présentes.
func (s *DocumentService) AIState() AIStatus {
	c := config.Current()
	st := AIStatus{
		Enabled:          c.Bool("ai.enabled", false),
		Locked:           c.Locked("ai.enabled"),
		Provider:         c.String("ai.provider", "anthropic"),
		Model:            c.String("ai.model", ""),
		DataSharing:      c.String("ai.dataSharing", ai.SchemaOnly),
		Managed:          c.String("ai.endpoint", "") != "",
		GatewayNoKey:     gatewayNoKey(c),
		Messages:         s.ai.session.Len(),
		Usage:            s.ai.session.Usage(),
		KeepConversation: c.Bool("ai.keepConversation", false),
	}
	for _, info := range ai.Catalog() {
		p := AIProvider{Info: info}
		if info.Available {
			_, p.HasKey = ai.DefaultKeys.Get(info.ID)
		}
		if info.ID == st.Provider && (p.HasKey || !info.NeedsKey) {
			s.refreshModels(info.ID, c.String("ai.endpoint", ""))
		}
		st.Providers = append(st.Providers, p)
		if info.ID == st.Provider && st.Model == "" {
			st.Model = info.DefaultModel
		}
	}
	s.ai.mu.Lock()
	st.Busy = s.ai.cancel != nil
	s.ai.mu.Unlock()
	return st
}

// gatewayNoKey : une passerelle d'entreprise (ai.endpoint) authentifie
// elle-même (ai.auth.mode = none) ; sinon chacun y présente sa clé
// personnelle, gardée dans le trousseau (configuration-entreprise.md).
func gatewayNoKey(c *config.Config) bool {
	return c.String("ai.endpoint", "") != "" && c.String("ai.auth.mode", "") == "none"
}

// refreshModels lit en arrière-plan la liste des modèles d'un fournisseur
// qui la publie (ai.ModelLister, mise en cache par le fournisseur) : le
// prochain AIState la montre. L'interface relit l'état après chaque envoi et
// à chaque changement de réglage.
func (s *DocumentService) refreshModels(id, endpoint string) {
	ml, ok := ai.Lookup(id).(ai.ModelLister)
	if !ok {
		return
	}
	s.ai.mu.Lock()
	if s.ai.listing[id] {
		s.ai.mu.Unlock()
		return
	}
	if s.ai.listing == nil {
		s.ai.listing = map[string]bool{}
	}
	s.ai.listing[id] = true
	s.ai.mu.Unlock()
	go func() {
		defer recoverPanic("liste des modèles")
		key, _ := ai.DefaultKeys.Get(id)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_, _ = ml.ListModels(ctx, ai.Credentials{Key: key, Endpoint: endpoint})
		s.ai.mu.Lock()
		delete(s.ai.listing, id)
		s.ai.mu.Unlock()
	}()
}

// AISetKey vérifie la clé d'API d'un fournisseur auprès de lui, puis
// l'enregistre dans le trousseau. Une clé refusée n'est pas enregistrée ;
// fournisseur injoignable : elle l'est quand même, et l'erreur le dit. Si
// le trousseau la refuse, elle sert pendant la session et l'erreur le dit.
func (s *DocumentService) AISetKey(ctx context.Context, provider, key string) (AIStatus, error) {
	if publicDemo.Load() {
		return s.AIState(), errDemo
	}
	p := ai.Lookup(provider)
	if p == nil {
		return s.AIState(), msg.New("ai.providerUnavailable", nil)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return s.AIState(), msg.New("ai.emptyKey", nil)
	}
	var warn string
	// À travers une passerelle, la clé est celle de l'organisation : la
	// passerelle n'a pas forcément la liste des modèles qui sert à vérifier.
	if kc, ok := p.(ai.KeyChecker); ok && config.Current().String("ai.endpoint", "") == "" {
		cred := ai.Credentials{Key: key, Endpoint: config.Current().String("ai.endpoint", "")}
		if err := kc.CheckKey(context.WithoutCancel(ctx), cred); err != nil {
			if !errors.Is(err, ai.ErrUnreachable) {
				return s.AIState(), err
			}
			warn = p.Info().Name
		}
	}
	if err := ai.DefaultKeys.Set(provider, key); err != nil {
		if warn != "" {
			return s.AIState(), msg.New("ai.keyUncheckedSessionOnly", msg.Args{"provider": warn, "err": err})
		}
		return s.AIState(), msg.New("ai.keySessionOnly", msg.Args{"err": err})
	}
	if warn != "" {
		return s.AIState(), msg.New("ai.keyUnchecked", msg.Args{"provider": warn})
	}
	return s.AIState(), nil
}

// AIForgetKey oublie la clé d'API d'un fournisseur.
func (s *DocumentService) AIForgetKey(provider string) (AIStatus, error) {
	err := ai.DefaultKeys.Delete(provider)
	return s.AIState(), err
}

// AISend envoie un message à l'assistant et rend la main quand sa réponse
// est finie ; le déroulé arrive par l'évènement ai:event, marqué de run
// (numéro choisi par l'interface), et se termine par un évènement done.
// view (feuille, tableau, sélection) part avec le message, devant le texte
// tapé ; l'affichage ne garde que le texte tapé. Un message que le modèle
// n'a pas reçu (échec ou arrêt au premier tour) est retiré de la
// conversation et de l'affichage : un évènement unsent remplace done, et
// l'interface le rend à la saisie.
func (s *DocumentService) AISend(ctx context.Context, run int, text string, view AIContext) error {
	return s.aiAnswer(ctx, run, &text, view)
}

// AIResume reprend la réponse run là où elle s'est arrêtée (erreur en cours
// de réponse, arrêt, nombre maximal d'étapes), sans nouveau message : ses
// modifications restent une seule réponse à annuler.
func (s *DocumentService) AIResume(ctx context.Context, run int) error {
	return s.aiAnswer(ctx, run, nil, AIContext{})
}

// aiTurn prépare un tour avec les réglages en cours ; rend aussi le partage
// des données.
func (s *DocumentService) aiTurn() (ai.Turn, string, error) {
	c := config.Current()
	if publicDemo.Load() {
		return ai.Turn{}, "", errDemo
	}
	if !c.Bool("ai.enabled", false) {
		return ai.Turn{}, "", msg.New("ai.disabled", nil)
	}
	id := c.String("ai.provider", "anthropic")
	p := ai.Lookup(id)
	if p == nil {
		return ai.Turn{}, "", msg.New("ai.providerUnavailableNamed", msg.Args{"provider": id})
	}
	info := p.Info()
	key, ok := ai.DefaultKeys.Get(id)
	if !ok && info.NeedsKey && !gatewayNoKey(c) {
		return ai.Turn{}, "", ai.ErrNoKey
	}
	model := c.String("ai.model", "")
	if model == "" {
		model = info.DefaultModel
	}
	sharing := c.String("ai.dataSharing", ai.SchemaOnly)
	h := host{s}
	// Langue de l'interface : celle des réponses et de l'écriture des
	// formules (comme dans la grille de l'utilisateur).
	lang := string(s.sessionLang())
	return ai.Turn{
		Provider: p,
		Cred:     ai.Credentials{Key: key, Endpoint: c.String("ai.endpoint", "")},
		Model:    model,
		System:   ai.SystemFor(sharing, s.htmlPages(), lang),
		Tools:    ai.ToolsWith(sharing, s.htmlPages()),
		Exec:     ai.ExecIn(h, sharing, lang),
		Label:    ai.Label,
		Write:    ai.IsWrite,
		Target:   ai.Target,
		// Tarifs publics d'Anthropic : ni pour une passerelle, ni pour un
		// revendeur qui expose les mêmes modèles (Mammouth AI).
		NoPrice: id != "anthropic" || c.String("ai.endpoint", "") != "",
	}, sharing, nil
}

// aiAnswer fait répondre l'assistant : à un nouveau message (text), ou en
// reprenant la réponse run (text nil).
func (s *DocumentService) aiAnswer(ctx context.Context, run int, text *string, view AIContext) error {
	turn, sharing, err := s.aiTurn()
	if err != nil {
		return err
	}
	resume := text == nil

	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.ai.mu.Lock()
	if s.ai.cancel != nil {
		s.ai.mu.Unlock()
		cancel()
		return ai.ErrBusy
	}
	s.ai.cancel = cancel
	gen := s.ai.gen
	s.ai.mu.Unlock()
	defer func() {
		s.ai.mu.Lock()
		if s.ai.gen == gen {
			s.ai.cancel = nil
		}
		s.ai.mu.Unlock()
		cancel()
	}()

	// Annulations faites par l'utilisateur (Ctrl+Z) depuis le dernier
	// message, et remarques en attente (« Annuler cette réponse ») : elles
	// partent avec le prochain message (pas avec une reprise).
	var notes []string
	if !resume {
		notes = s.aiUndoneNotes(ctx)
	}
	s.ai.mu.Lock()
	if !resume {
		notes = append(s.ai.notes, notes...)
		s.ai.notes = nil
	}
	if s.ai.runs == nil {
		s.ai.runs = map[int]*aiRun{}
	}
	r := s.ai.runs[run]
	if r == nil || !resume {
		r = &aiRun{}
		s.ai.runs[run] = r
		s.ai.order = append(s.ai.order, run)
	}
	if resume {
		s.aiTrimRun(run)
	} else {
		s.ai.log = append(s.ai.log, AILogEntry{Run: run, User: *text})
	}
	s.ai.mu.Unlock()

	h := host{s}
	lang := string(s.sessionLang())
	emit := func(e ai.Event) {
		if e.Kind == ai.EventToolDone && e.Write && !e.Error {
			s.ai.mu.Lock()
			r.changes++
			s.ai.mu.Unlock()
		}
		s.aiEmit(gen, run, e)
	}
	gctx := command.WithGroup(ctx, aiUndoGroup)
	// Résultats des requêtes de l'assistant (query_show) : montrés à
	// l'utilisateur dans la conversation.
	gctx = ai.WithShow(gctx, func(res ai.Result) { emit(ai.Event{Kind: ai.EventResult, Result: &res}) })
	if resume {
		err = s.ai.session.Resume(gctx, turn, emit)
	} else {
		msg := ai.Compose(*text, ai.View{Sheet: view.Sheet, Table: view.Table, Selection: view.Selection, Today: time.Now().Format("2006-01-02")}, notes)
		// Premier message : la structure du classeur, pour répondre sans
		// explorer ; ensuite, de nouveau si elle a changé hors des réponses
		// (import, Ctrl+Z, conversation reprise).
		// Conversation longue : résumée d'abord (« Session compactée »), la
		// structure repart alors entière.
		compacted := s.ai.session.NeedsCompact() && s.ai.session.Compact(gctx, turn, emit) == nil
		st := ai.StructureIn(ctx, h, sharing, lang)
		s.ai.mu.Lock()
		known := s.ai.structure
		s.ai.mu.Unlock()
		if s.ai.session.Len() == 0 || compacted {
			msg = st + msg
		} else if st != known {
			msg = ai.StructureUpdate(st) + msg
		}
		err = s.ai.session.Send(gctx, turn, msg, emit)
	}
	// La structure que le modèle connaît désormais (ses propres
	// modifications comprises).
	if !ai.Unsent(err) {
		st := ai.StructureIn(context.WithoutCancel(ctx), h, sharing, lang)
		s.ai.mu.Lock()
		if s.ai.gen == gen {
			s.ai.structure = st
		}
		s.ai.mu.Unlock()
	}

	s.ai.mu.Lock()
	r.seqs = append(r.seqs, command.GroupSeqs(gctx)...)
	// Bilan : les écritures sans effet (volets déjà figés, même valeur)
	// ne créent aucune étape du journal ; sans étape, pas de bilan.
	changes := r.changes
	if len(r.seqs) == 0 {
		changes = 0
	}
	unsent := ai.Unsent(err) && s.ai.gen == gen
	if unsent {
		// Message retiré de la conversation : retiré aussi de l'affichage,
		// et les remarques repartiront avec le suivant.
		s.ai.notes = append(notes, s.ai.notes...)
		s.aiForgetRun(run)
	}
	s.ai.mu.Unlock()
	if unsent {
		s.sys().Emit(AIEvent, AIEventData{Run: run, Event: ai.Event{Kind: ai.EventUnsent}})
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		// Pour le réaffichage seulement : l'interface reçoit l'erreur en
		// retour de l'appel.
		s.aiLog(gen, run, ai.Event{Kind: ai.EventNotice, Text: err.Error(), Error: true})
	}
	s.aiEmit(gen, run, ai.Event{Kind: ai.EventDone, Changes: changes, Undo: s.aiCanUndo(ctx, run)})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// aiTrimRun retire de la fin du journal d'affichage la fin de la réponse run
// (done, erreur, arrêt au nombre d'étapes), qu'une reprise remplace ; mu
// tenu.
func (s *DocumentService) aiTrimRun(run int) {
	for n := len(s.ai.log); n > 0; n = len(s.ai.log) {
		e := s.ai.log[n-1].Event
		if s.ai.log[n-1].Run != run || e == nil || !(e.Kind == ai.EventDone || (e.Kind == ai.EventNotice && (e.Error || e.Limit))) {
			return
		}
		s.ai.log = s.ai.log[:n-1]
	}
}

// aiForgetRun oublie une réponse que le modèle n'a pas reçue ; mu tenu.
func (s *DocumentService) aiForgetRun(run int) {
	s.ai.log = slices.DeleteFunc(s.ai.log, func(e AILogEntry) bool { return e.Run == run })
	delete(s.ai.runs, run)
	s.ai.order = slices.DeleteFunc(s.ai.order, func(r int) bool { return r == run })
}

// aiEmit envoie un évènement à l'interface et le garde dans le journal
// d'affichage, sauf si la conversation a été remplacée entre-temps.
func (s *DocumentService) aiEmit(gen, run int, e ai.Event) {
	if !s.aiLog(gen, run, e) {
		return
	}
	s.sys().Emit(AIEvent, AIEventData{Run: run, Event: e})
}

// aiLog ajoute un évènement au journal d'affichage ; les morceaux de texte
// qui se suivent sont réunis en un seul.
func (s *DocumentService) aiLog(gen, run int, e ai.Event) bool {
	s.ai.mu.Lock()
	defer s.ai.mu.Unlock()
	if s.ai.gen != gen {
		return false
	}
	if n := len(s.ai.log); e.Kind == ai.EventText && n > 0 {
		last := s.ai.log[n-1]
		if last.Run == run && last.Event != nil && last.Event.Kind == ai.EventText {
			ev := *last.Event
			ev.Text += e.Text
			s.ai.log[n-1].Event = &ev
			return true
		}
	}
	s.ai.log = append(s.ai.log, AILogEntry{Run: run, Event: &AIEventData{Run: run, Event: e}})
	return true
}

// AIStop interrompt la réponse en cours.
func (s *DocumentService) AIStop() {
	s.ai.mu.Lock()
	defer s.ai.mu.Unlock()
	if s.ai.cancel != nil {
		s.ai.cancel()
	}
}

// AIReset commence une nouvelle conversation.
func (s *DocumentService) AIReset() error {
	// Une réponse qui démarre (AISend) pose cancel avant de lire le journal
	// et la structure : refusée dès ce moment, comme AINewConversation.
	s.ai.mu.Lock()
	defer s.ai.mu.Unlock()
	if s.ai.cancel != nil {
		return ai.ErrBusy
	}
	if err := s.ai.session.Reset(); err != nil {
		return err
	}
	s.ai.log, s.ai.runs, s.ai.order, s.ai.notes, s.ai.structure = nil, nil, nil, nil, ""
	return nil
}

// AITranscript donne la conversation à réafficher (ouverture d'un
// classeur, interface rechargée), avec, pour chaque réponse, la
// possibilité de l'annuler telle qu'elle est maintenant.
func (s *DocumentService) AITranscript() []AILogEntry {
	s.ai.mu.Lock()
	log := append([]AILogEntry(nil), s.ai.log...)
	s.ai.mu.Unlock()
	out := make([]AILogEntry, len(log))
	ctx := context.Background()
	for i, e := range log {
		if e.Event != nil && e.Event.Kind == ai.EventDone {
			ev := *e.Event
			ev.Undo = s.aiCanUndo(ctx, e.Run)
			e.Event = &ev
		}
		out[i] = e
	}
	return out
}

// errAIUndoLater : la réponse n'est plus la dernière modification.
var errAIUndoLater = msg.New("ai.undoLater", nil)

// aiJournalTop rend les n dernières étapes du journal, non annulées, de la
// plus récente à la plus ancienne.
func aiJournalTop(ctx context.Context, d *doc.Document, n int) ([]int64, error) {
	rows, err := d.DB().Read().QueryContext(ctx, `SELECT seq FROM _lattis_journal WHERE undone = 0 ORDER BY seq DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var seq int64
		if err := rows.Scan(&seq); err != nil {
			return nil, err
		}
		out = append(out, seq)
	}
	return out, rows.Err()
}

// aiRunIsTop dit si les étapes de la réponse sont exactement les dernières
// du journal.
func aiRunIsTop(ctx context.Context, d *doc.Document, seqs []int64) bool {
	if len(seqs) == 0 {
		return false
	}
	top, err := aiJournalTop(ctx, d, len(seqs))
	if err != nil || len(top) != len(seqs) {
		return false
	}
	mine := map[int64]bool{}
	for _, q := range seqs {
		mine[q] = true
	}
	for _, q := range top {
		if !mine[q] {
			return false
		}
	}
	return true
}

// aiCanUndo : la réponse run a modifié le classeur et ses étapes sont les
// dernières du journal.
func (s *DocumentService) aiCanUndo(ctx context.Context, run int) bool {
	s.ai.mu.Lock()
	r := s.ai.runs[run]
	var seqs []int64
	if r != nil && !r.undone {
		seqs = append(seqs, r.seqs...)
	}
	s.ai.mu.Unlock()
	d, err := s.current()
	if err != nil || len(seqs) == 0 {
		return false
	}
	return aiRunIsTop(ctx, d, seqs)
}

// AIUndoRun annule les modifications de la réponse run si ce sont les
// dernières étapes du journal ; le modèle en est prévenu au message
// suivant.
func (s *DocumentService) AIUndoRun(ctx context.Context, run int) (DocState, error) {
	s.ai.mu.Lock()
	if s.ai.cancel != nil {
		s.ai.mu.Unlock()
		return DocState{}, ai.ErrBusy
	}
	r := s.ai.runs[run]
	if r == nil || r.undone || len(r.seqs) == 0 {
		s.ai.mu.Unlock()
		return DocState{}, msg.New("ai.nothingToUndo", nil)
	}
	seqs := append([]int64(nil), r.seqs...)
	s.ai.mu.Unlock()
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	if !aiRunIsTop(ctx, d, seqs) {
		return DocState{}, errAIUndoLater
	}
	for range seqs {
		if _, err := d.Bus.Undo(ctx); err != nil {
			s.changed(ctx)
			return DocState{}, err
		}
	}
	d.Notify(collab.UndoNoticeIn(s.sessionLang(), d.Oplog, false))
	s.ai.mu.Lock()
	r.undone = true
	s.ai.notes = append(s.ai.notes, "L'utilisateur a annulé les modifications de votre réponse précédente.")
	s.ai.mu.Unlock()
	return s.changed(ctx)
}

// aiUndoneNotes repère les réponses dont l'utilisateur a annulé les
// modifications lui-même (Ctrl+Z) : étapes marquées annulées dans le
// journal, ou disparues (une action a suivi l'annulation). Chacune n'est
// signalée qu'une fois.
func (s *DocumentService) aiUndoneNotes(ctx context.Context) []string {
	d, err := s.current()
	if err != nil {
		return nil
	}
	s.ai.mu.Lock()
	type pending struct {
		r    *aiRun
		seqs []int64
	}
	var check []pending
	for _, id := range s.ai.order {
		if r := s.ai.runs[id]; r != nil && !r.undone && len(r.seqs) > 0 {
			check = append(check, pending{r, append([]int64(nil), r.seqs...)})
		}
	}
	s.ai.mu.Unlock()
	var notes []string
	last := len(check) - 1
	for i, p := range check {
		undone := 0
		for _, seq := range p.seqs {
			var u int
			err := d.DB().Read().QueryRowContext(ctx, `SELECT undone FROM _lattis_journal WHERE seq = ?`, seq).Scan(&u)
			if errors.Is(err, sql.ErrNoRows) || (err == nil && u == 1) {
				undone++
			}
		}
		if undone == 0 {
			continue
		}
		which := "d'une de vos réponses précédentes"
		if i == last {
			which = "de votre réponse précédente"
		}
		if undone == len(p.seqs) {
			notes = append(notes, "L'utilisateur a annulé (Ctrl+Z) les modifications "+which+".")
		} else {
			notes = append(notes, "L'utilisateur a annulé (Ctrl+Z) une partie des modifications "+which+".")
		}
		s.ai.mu.Lock()
		p.r.undone = true
		s.ai.mu.Unlock()
	}
	return notes
}

// --- Conversation et classeur -------------------------------------------------

// aiSaved est la conversation gardée dans le classeur : historique dans la
// forme neutre (les messages propres au fournisseur sont perdus) et
// conversation affichée.
type aiSaved struct {
	Version int          `json:"version"`
	History []ai.Message `json:"history"`
	Log     []AILogEntry `json:"log"`
	Usage   ai.Usage     `json:"usage"`
	// Others : conversations mises de côté (ignorées des versions qui ne
	// les connaissent pas).
	Others []aiConv `json:"others,omitempty"`
}

// aiReplaced remet la conversation à zéro quand le classeur change (d nil :
// fermé), ou la restaure depuis le classeur ouvert si le réglage
// ai.keepConversation est actif ; l'interface est prévenue (ai:reset).
func (s *DocumentService) aiReplaced(d *doc.Document) {
	var saved aiSaved
	if d != nil && config.Current().Bool("ai.keepConversation", false) {
		saved, _ = loadConversation(context.Background(), d)
	}
	s.ai.mu.Lock()
	if s.ai.cancel != nil {
		s.ai.cancel()
		s.ai.cancel = nil
	}
	s.ai.gen++
	s.ai.log, s.ai.runs, s.ai.order, s.ai.notes, s.ai.structure = saved.Log, nil, nil, nil, ""
	s.ai.others = saved.Others
	s.ai.nextConv = 0
	for _, c := range saved.Others {
		s.ai.nextConv = max(s.ai.nextConv, c.ID)
	}
	s.ai.mu.Unlock()
	s.ai.session.Restore(saved.History, saved.Usage)
	s.sys().Emit(AIResetEvent, nil)
}

// loadConversation lit la conversation gardée dans le classeur.
func loadConversation(ctx context.Context, d *doc.Document) (aiSaved, error) {
	var raw string
	var out aiSaved
	err := d.DB().Read().QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, aiConversationKey).Scan(&raw)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return aiSaved{}, err
	}
	if out.Version != 1 {
		return aiSaved{}, msg.New("ai.conversationVersion", msg.Args{"version": out.Version})
	}
	return out, nil
}

// storeConversation range la conversation dans le classeur avant son
// enregistrement si le réglage ai.keepConversation est actif, et l'en
// retire sinon. Elle ne rend pas le classeur « modifié » : une conversation
// sans modification du classeur n'est gardée qu'au prochain enregistrement
// effectif.
func (s *DocumentService) storeConversation(ctx context.Context, d *doc.Document) error {
	keep := config.Current().Bool("ai.keepConversation", false)
	var value []byte
	if keep {
		hist, usage := s.ai.session.Snapshot()
		s.ai.mu.Lock()
		others := len(s.ai.others) > 0
		s.ai.mu.Unlock()
		if len(hist) > 0 || others {
			s.ai.mu.Lock()
			saved := aiSaved{Version: 1, History: hist, Log: s.ai.log, Usage: usage, Others: s.ai.others}
			var err error
			value, err = json.Marshal(saved)
			s.ai.mu.Unlock()
			if err != nil {
				return err
			}
		}
	}
	// Rien à changer : pas d'écriture (un classeur neuf reste « rien à
	// récupérer »).
	var old sql.NullString
	err := d.DB().Read().QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, aiConversationKey).Scan(&old)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if (value == nil && !old.Valid) || (old.Valid && old.String == string(value)) {
		return nil
	}
	return d.DB().Write(ctx, func(tx *sql.Tx) error {
		if value == nil {
			_, err := tx.ExecContext(ctx, `DELETE FROM _lattis_info WHERE key = ?`, aiConversationKey)
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, aiConversationKey, string(value))
		return err
	})
}

// --- Plusieurs conversations ---------------------------------------------------

// aiConv est une conversation mise de côté (Nouvelle conversation, ou une
// autre reprise) : elle se reprend telle quelle, sans ses annulations de
// réponse (seule la conversation en cours suit le journal).
type aiConv struct {
	ID      int          `json:"id"`
	Title   string       `json:"title"`
	Updated time.Time    `json:"updated"`
	History []ai.Message `json:"history"`
	Log     []AILogEntry `json:"log"`
	Usage   ai.Usage     `json:"usage"`
}

// AIConversation décrit une conversation pour la liste de l'interface.
type AIConversation struct {
	// ID : 0 pour la conversation en cours.
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Updated string `json:"updated"` // RFC 3339
	Current bool   `json:"current"`
}

// aiTitle : le premier message de l'utilisateur, abrégé.
func aiTitle(log []AILogEntry) string {
	for _, e := range log {
		if e.User != "" {
			t := strings.Join(strings.Fields(e.User), " ")
			if r := []rune(t); len(r) > 60 {
				t = string(r[:59]) + "…"
			}
			return t
		}
	}
	return "Conversation"
}

// aiShelve met la conversation en cours de côté (rien si elle est vide) et
// en commence une nouvelle ; mu tenu, aucune réponse en cours.
func (s *DocumentService) aiShelve() {
	hist, usage := s.ai.session.Snapshot()
	if len(hist) > 0 {
		s.ai.nextConv++
		s.ai.others = append(s.ai.others, aiConv{
			ID: s.ai.nextConv, Title: aiTitle(s.ai.log), Updated: time.Now(),
			History: hist, Log: s.ai.log, Usage: usage,
		})
	}
	s.ai.session.Restore(nil, ai.Usage{})
	s.ai.log, s.ai.runs, s.ai.order, s.ai.notes, s.ai.structure = nil, nil, nil, nil, ""
}

// AIConversations liste les conversations du classeur : celle en cours (si
// elle a commencé), puis les autres, de la plus récente à la plus ancienne.
func (s *DocumentService) AIConversations() []AIConversation {
	s.ai.mu.Lock()
	defer s.ai.mu.Unlock()
	out := []AIConversation{}
	if s.ai.session.Len() > 0 {
		out = append(out, AIConversation{Title: aiTitle(s.ai.log), Updated: time.Now().Format(time.RFC3339), Current: true})
	}
	for i := len(s.ai.others) - 1; i >= 0; i-- {
		c := s.ai.others[i]
		out = append(out, AIConversation{ID: c.ID, Title: c.Title, Updated: c.Updated.Format(time.RFC3339)})
	}
	return out
}

// AINewConversation met la conversation en cours de côté (elle reste dans
// la liste) et en commence une nouvelle.
func (s *DocumentService) AINewConversation() error {
	s.ai.mu.Lock()
	defer s.ai.mu.Unlock()
	if s.ai.cancel != nil {
		return ai.ErrBusy
	}
	s.aiShelve()
	return nil
}

// AIOpenConversation reprend une conversation mise de côté ; celle en cours
// est mise de côté à sa place.
func (s *DocumentService) AIOpenConversation(id int) error {
	s.ai.mu.Lock()
	defer s.ai.mu.Unlock()
	if s.ai.cancel != nil {
		return ai.ErrBusy
	}
	i := slices.IndexFunc(s.ai.others, func(c aiConv) bool { return c.ID == id })
	if i < 0 {
		return msg.New("ai.conversationNotFound", nil)
	}
	c := s.ai.others[i]
	s.ai.others = slices.Delete(s.ai.others, i, i+1)
	s.aiShelve()
	s.ai.session.Restore(c.History, c.Usage)
	s.ai.log = c.Log
	return nil
}

// AIDeleteConversation supprime une conversation (0 : celle en cours, qui
// est vidée).
func (s *DocumentService) AIDeleteConversation(id int) error {
	if id == 0 {
		return s.AIReset()
	}
	s.ai.mu.Lock()
	defer s.ai.mu.Unlock()
	s.ai.others = slices.DeleteFunc(s.ai.others, func(c aiConv) bool { return c.ID == id })
	return nil
}
