package bridge

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/coedit"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/collab"
)

// Travail à plusieurs sur un classeur d'un dossier partagé (NAS,
// décision 0011) : chaque classeur ouvert depuis son fichier regarde le
// dossier caché « ~$nom.lattis.d » ; un autre poste qui l'ouvre propose
// « Travailler à plusieurs » à la place de « Ouvrir quand même ».

// CoEditEvent est émis quand l'état du travail à plusieurs change
// (présences, conflits, dossier injoignable) : l'interface relit
// CoEditState.
const CoEditEvent = "coedit:changed"

// coeditTiming : délais du travail à plusieurs (les essais les raccourcissent).
var coeditTiming coedit.Timing

// coeditState suit la session de travail à plusieurs du classeur ouvert.
type coeditState struct {
	mu     sync.Mutex
	eng    *coedit.Engine
	doc    *doc.Document
	cancel context.CancelFunc
	// ticket : demande acceptée, pour le classeur en cours d'ouverture
	// (OpenTogether).
	ticket *coedit.Ticket
}

// coeditable : le classeur se prête au travail à plusieurs (ouvert depuis
// son fichier, modifiable, pas partagé sur un Serveur Lattis).
func coeditable(d *doc.Document) bool {
	return d != nil && d.Path() != "" && !d.XLSX() && !d.ReadOnly() && d.Share.Info() == nil && config.CoEditing(config.Current())
}

// startCoEdit démarre la session du classeur d (après une ouverture) :
// avec la demande acceptée par OpenTogether, ou seule, à l'écoute.
func (s *DocumentService) startCoEdit(d *doc.Document) {
	s.coedit.mu.Lock()
	tk := s.coedit.ticket
	s.coedit.ticket = nil
	s.coedit.mu.Unlock()
	if !coeditable(d) {
		tk.Cancel()
		return
	}
	opt := coedit.Options{Timing: coeditTiming, OnEvent: func(ev coedit.Event) { s.coeditEvent(d, ev) }}
	var eng *coedit.Engine
	var err error
	if tk != nil {
		eng, err = coedit.Start(context.Background(), d, tk, opt)
		if err != nil {
			tk.Cancel()
			d.Notify(s.say("coedit.startFailed", msg.Args{"err": err}))
		}
	} else {
		eng, err = coedit.New(d, opt)
	}
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.coedit.mu.Lock()
	s.coedit.eng, s.coedit.doc, s.coedit.cancel = eng, d, cancel
	s.coedit.mu.Unlock()
	go func() {
		defer recoverPanic("travail à plusieurs")
		eng.Run(ctx)
	}()
}

// stopCoEdit arrête la session du classeur qui se ferme : la file part,
// la présence est retirée.
func (s *DocumentService) stopCoEdit() {
	s.coedit.mu.Lock()
	eng, cancel := s.coedit.eng, s.coedit.cancel
	s.coedit.eng, s.coedit.doc, s.coedit.cancel = nil, nil, nil
	s.coedit.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if eng != nil {
		eng.Close()
	}
}

// coeditEngine rend la session du classeur ouvert (nil : aucune).
func (s *DocumentService) coeditEngine() *coedit.Engine {
	cur, err := s.current()
	s.coedit.mu.Lock()
	defer s.coedit.mu.Unlock()
	if err != nil || cur != s.coedit.doc {
		return nil
	}
	return s.coedit.eng
}

// coeditEvent relaie un tour de la session : messages, grille à relire,
// indicateur.
func (s *DocumentService) coeditEvent(d *doc.Document, ev coedit.Event) {
	if cur, err := s.current(); err != nil || cur != d {
		return
	}
	if len(ev.Notices) > 0 {
		d.Notify(msg.Join(s.sessionLang(), ev.Notices))
	}
	if ev.Data || len(ev.Notices) > 0 {
		s.changed(context.Background())
	}
	if ev.State {
		if eng := s.coeditEngine(); eng != nil {
			s.sys().Emit(CoEditEvent, eng.State())
		}
	}
}

// CoEditState décrit le travail à plusieurs du classeur ouvert (Active
// faux : seul, ou classeur qui ne s'y prête pas).
func (s *DocumentService) CoEditState() coedit.State {
	if eng := s.coeditEngine(); eng != nil {
		return eng.State()
	}
	return coedit.State{Peers: []coedit.Peer{}}
}

// kickCoEdit fait partir aussitôt la modification locale.
func (s *DocumentService) kickCoEdit() {
	if eng := s.coeditEngine(); eng != nil {
		eng.Kick()
	}
}

// openQuestion : à l'ouverture, le classeur est ouvert ailleurs (verrou
// ~$, ou sessions du travail à plusieurs) ; nil sinon. coEdit : « Travailler
// à plusieurs » est proposé.
func (s *DocumentService) openQuestion(path string) (q *FileQuestion, coEdit bool) {
	on := config.CoEditing(config.Current())
	if on {
		if m, err := archive.ReadManifest(path); err == nil && m.Remote != nil {
			on = false // classeur partagé sur un Serveur Lattis
		}
	}
	if h := archive.LockedBy(path); h != nil {
		e := archive.LockedError{Path: path, Holder: *h}
		return &FileQuestion{Path: path, Message: msg.Text(s.sessionLang(), &e)}, on
	}
	if !on {
		return nil, false
	}
	if others := coedit.Others(path, coeditTiming); len(others) > 0 {
		var names []string
		for _, p := range others {
			names = append(names, s.say("coedit.personOnMachine", msg.Args{"name": p.Who(), "machine": p.Machine}))
		}
		return &FileQuestion{Path: path, Message: s.say("coedit.openedBy", msg.Args{"file": filepath.Base(path), "names": strings.Join(names, ", ")})}, true
	}
	return nil, false
}

// OpenTogether ouvre un classeur déjà ouvert sur d'autres postes pour y
// travailler à plusieurs : la demande attend que chaque session l'accueille
// (20 s au plus). Sans réponse, la question d'avant revient
// (OpenResult.Locked, sans « Travailler à plusieurs »).
func (s *DocumentService) OpenTogether(ctx context.Context, path string) (OpenResult, error) {
	if err := s.sessionPath(path); err != nil {
		return OpenResult{}, err
	}
	tk, err := coedit.Join(ctx, path, coeditTiming)
	var none *coedit.NoAnswerError
	if errors.As(err, &none) {
		return OpenResult{Locked: &FileQuestion{Path: path, Message: msg.Text(s.sessionLang(), err)}}, nil
	}
	if err != nil {
		return OpenResult{}, err
	}
	s.coedit.mu.Lock()
	s.coedit.ticket = tk
	s.coedit.mu.Unlock()
	r, err := s.open(ctx, path, doc.OpenCoEdit)
	s.coedit.mu.Lock()
	left := s.coedit.ticket // pas consommé : ouverture échouée ou détournée
	s.coedit.ticket = nil
	s.coedit.mu.Unlock()
	left.Cancel()
	return r, err
}

// coeditConflicts et coeditResolve servent le volet Conflits quand le
// classeur n'est pas collaboratif sur un Serveur Lattis.
func (s *DocumentService) coeditConflicts(ctx context.Context) ([]collab.Conflict, bool, error) {
	eng := s.coeditEngine()
	if eng == nil {
		return nil, false, nil
	}
	cs, err := eng.Conflicts(ctx)
	if cs == nil {
		cs = []collab.Conflict{}
	}
	return cs, true, err
}
