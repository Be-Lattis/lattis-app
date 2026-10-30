package bridge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/work"
	"github.com/lalamefine/lattis/protocol"
)

// ChangedEvent est émis après chaque modification du document : l'interface
// relit la mise en page et vide son cache de lignes.
const ChangedEvent = "doc:changed"

// DocState décrit le classeur ouvert pour l'interface (titre, menus).
type DocState struct {
	Open  bool   `json:"open"`
	Path  string `json:"path"`
	Title string `json:"title"`
	Dirty bool   `json:"dirty"`
	Undo  string `json:"undo"` // libellé de l'action à annuler
	Redo  string `json:"redo"`
	// Notice : message de la dernière action pour la barre d'état (tableaux
	// fusionnés, cellules rattachées à un tableau), vide sinon.
	Notice string `json:"notice,omitempty"`
	// ReadOnly : classeur verrouillé par un autre poste, ouvert en lecture
	// seule (Enregistrer devient Enregistrer sous).
	ReadOnly bool `json:"readOnly,omitempty"`
	// Conflict : rien n'a été enregistré, le fichier a été modifié par
	// ailleurs depuis son ouverture ; l'interface demande quoi faire
	// (ResolveSave).
	Conflict *FileQuestion `json:"conflict,omitempty"`
	// Remote : classeur ouvert depuis un Serveur Lattis (son fichier n'est
	// qu'un cache local, jamais montré) : titre sur le serveur, serveur,
	// adresse <serveur>/w/<id>, en ligne ou non ; nil pour un fichier local.
	Remote *workbook.State `json:"remote,omitempty"`
	// Theme : thème de couleur du classeur (Affichage › Thème) ; nil : thème
	// par défaut des réglages (ui.theme).
	Theme *canvas.Theme `json:"theme,omitempty"`
	// Format : "xlsx" pour un fichier .xlsx ouvert qui garde son format
	// (xlsxdoc.go), vide pour un classeur .lattis.
	Format string `json:"format,omitempty"`
	// XLSX : rien n'a été enregistré, l'interface demande s'il faut garder
	// le format .xlsx ou convertir en .lattis (SaveXLSX).
	XLSX *XLSXQuestion `json:"xlsx,omitempty"`
}

// OpenResult est le résultat d'une ouverture : soit le classeur est ouvert,
// soit une session précédente interrompue est proposée en récupération.
type OpenResult struct {
	State       DocState        `json:"state"`
	Recoverable *archive.Orphan `json:"recoverable,omitempty"`
	// Damaged : fichier endommagé dont on peut tenter de récupérer les
	// données (Salvage) ; les autres fichiers illisibles sont une erreur.
	Damaged *DamagedFile `json:"damaged,omitempty"`
	// Locked : classeur ouvert sur un autre poste (verrou ~$) ; l'interface
	// propose de l'ouvrir en lecture seule ou quand même (OpenLocked).
	Locked *FileQuestion `json:"locked,omitempty"`
	// CoEdit : avec Locked, « Travailler à plusieurs » (OpenTogether)
	// remplace « Ouvrir quand même » (travail à plusieurs sur un dossier
	// partagé, features.coEditing).
	CoEdit bool `json:"coEdit,omitempty"`
	// Shared : le fichier est celui de l'auteur d'un classeur partagé en direct ;
	// l'interface l'ouvre depuis le serveur (en ligne, synchronisé) plutôt
	// que le fichier (OpenLocalCopy l'ouvre tel quel, serveur injoignable).
	Shared    *RecentFile `json:"shared,omitempty"`
	Cancelled bool        `json:"cancelled"`
}

// DamagedFile décrit un classeur endommagé.
type DamagedFile struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// FileQuestion : question à poser sur un fichier (verrouillé par un autre
// poste, modifié par ailleurs).
type FileQuestion struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// SalvageResult est le résultat d'une récupération de données.
type SalvageResult struct {
	State  DocState `json:"state"`
	Report string   `json:"report"` // résumé pour l'utilisateur
}

// DocumentService expose un classeur à l'interface (phase 2).
type DocumentService struct {
	hostRef // hôte : bureau (Wails) ou web (platform.go)
	mu      sync.Mutex
	doc     *doc.Document
	tables  map[int64]*grid.Table // tableaux chargés pour la lecture par fenêtres
	// memo garde les résultats coûteux du menu ▾ d'une colonne (valeurs du
	// filtre, contrôle de clé, mélange de types), jusqu'à la prochaine
	// modification de leur tableau.
	memo     map[memoKey]memoVal
	sheet    int64 // feuille affichée : Cell_A1 de l'onglet SQL s'y lit
	ui       uiState
	mcp      mcpState
	demo     *demoRun                // classeur de démonstration demandé au lancement
	building atomic.Pointer[demoRun] // démonstration en cours de fabrication (DemoProgress)
	pq       pqState                 // base de travail des requêtes d'import (query.go)
	ev       eventLoop               // évènements du classeur (events.go)
	closeReq closeRequest            // fermeture de la fenêtre demandée (closewin.go)
	ai       aiState                 // conversation de l'onglet IA (ai.go)
	share    shareState              // classeur partagé : suivi du serveur (sharing.go)
	coedit   coeditState             // travail à plusieurs sur un dossier partagé (coedit.go)
	data     dataWatch               // données du Data Lake liées : suivi en direct (data.go)
	lost     string                  // file d'envoi du classeur fermé pas gardée (replace) : message pour State
	sql      sqlRuns                 // requêtes en cours de l'onglet SQL (execute.go)
	lang     langState               // langue de la session (lang.go)
	work     workState               // opérations longues suivies (work.go)
	xlsx     xlsxState               // fichier .xlsx ouvert : format gardé (xlsxdoc.go)

	// Classeur à ouvrir au lancement (association de fichiers, launch.go).
	launch launchState
}

var (
	errNoDoc  = msg.New("doc.none", nil)
	errNoDemo = msg.New("doc.noDemo", nil)
)

func (s *DocumentService) current() (*doc.Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil {
		return nil, errNoDoc
	}
	return s.doc, nil
}

// ServiceShutdown ferme le classeur. Les modifications non enregistrées
// restent dans le répertoire de travail : elles seront proposées en
// récupération au prochain lancement.
func (s *DocumentService) ServiceShutdown() error {
	s.stopMCP()
	s.stopShare()
	s.stopCoEdit()
	s.closeQuerySession()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc == nil {
		return nil
	}
	kept := keepCollabQueue(s.doc) == nil
	if dirty, _ := s.doc.Dirty(context.Background()); !kept || dirty {
		// Garder le répertoire de travail (récupération au prochain
		// lancement), mais libérer le fichier pour les autres postes.
		s.doc.Unlock()
		return nil
	}
	return s.doc.Close()
}

// Orphans liste les sessions interrompues à proposer au démarrage.
func (s *DocumentService) Orphans() ([]archive.Orphan, error) {
	// Démo publique : jetable, rien à récupérer.
	if publicDemo.Load() {
		return []archive.Orphan{}, nil
	}
	return archive.Orphans()
}

func (s *DocumentService) replace(d *doc.Document) error {
	s.mu.Lock()
	old := s.doc
	s.doc, s.tables, s.memo = d, map[int64]*grid.Table{}, nil
	s.mu.Unlock()
	s.resetEvents()
	applyLocale(d, s.sessionLang()) // locale des formules de la session
	// Classeur partagé : suivi de son serveur (republications, connexion).
	if d != nil {
		s.watchShare(d)
	} else {
		s.stopShare()
	}
	s.watchData(d)
	// Travail à plusieurs : la session du classeur quitté s'arrête avant
	// qu'il se ferme ; celle du nouveau commence.
	s.stopCoEdit()
	s.startCoEdit(d)
	var lost error
	if old != nil {
		if lost = closeKeeping(old); lost != nil {
			s.mu.Lock()
			s.lost = lost.Error()
			s.mu.Unlock()
		}
	}
	// Conversation de l'assistant : propre au classeur.
	s.aiReplaced(d)
	return lost
}

// New crée un classeur vide.
func (s *DocumentService) New(ctx context.Context) (DocState, error) {
	// Sa première feuille est nommée dans la langue de l'interface.
	d, err := doc.NewIn(ctx, s.sessionLang())
	if err != nil {
		return DocState{}, err
	}
	s.replace(d)
	return s.State(ctx)
}

// Open ouvre un classeur (path vide : boîte de dialogue).
func (s *DocumentService) Open(ctx context.Context, path string) (OpenResult, error) {
	if path == "" {
		p, err := s.sys().OpenFile(FileDialog{
			Title: "Ouvrir un classeur",
			Filters: []FileFilter{
				{"Classeur Lattis ou Excel", "*.lattis;*.xlsx"},
				{"Classeur Lattis", "*.lattis"},
				{"Classeur Excel", "*.xlsx"},
			},
		})
		if err != nil {
			return OpenResult{}, err
		}
		if p == "" {
			return OpenResult{Cancelled: true}, nil
		}
		path = p
	}
	if err := s.sessionPath(path); err != nil {
		return OpenResult{}, err
	}
	// Fichier .xlsx : ni partage, ni verrou ~$ (xlsxdoc.go).
	if archive.Foreign(path) {
		return s.open(ctx, path, doc.OpenNormal)
	}
	if r := sharedFile(path); r != nil {
		return OpenResult{Shared: r}, nil
	}
	// Ouvert sur un autre poste (verrou ~$ à côté du fichier, ou sessions
	// du travail à plusieurs) : l'interface propose la lecture seule, et de
	// travailler à plusieurs (ou, sans, de l'ouvrir quand même).
	if q, together := s.openQuestion(path); q != nil {
		return OpenResult{Locked: q, CoEdit: together}, nil
	}
	return s.open(ctx, path, doc.OpenNormal)
}

// sharedFile rend le classeur du serveur d'un fichier d'auteur partagé en
// direct (manifeste : section remote, mode collaboratif ; pas un cache), nil
// sinon. Un classeur figé garde son fichier : l'auteur y modifie et republie.
func sharedFile(path string) *RecentFile {
	if inShareCache(path) {
		return nil
	}
	m, err := archive.ReadManifest(path)
	if err != nil || m.Remote == nil || m.Remote.Mode != workbook.Collaborative {
		return nil
	}
	// Modifications faites dans ce fichier et pas encore envoyées (hors
	// ligne, puis Ctrl+S) : le fichier s'ouvre tel quel, elles partent de
	// lui-même au serveur (ouvert depuis le serveur, elles y resteraient).
	if m.Remote.Unsent > 0 {
		return nil
	}
	abs, _ := filepath.Abs(path)
	return &RecentFile{Path: abs, Title: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Server: m.Remote.Server, WorkbookID: m.Remote.WorkbookID, Link: protocol.WorkbookLink(m.Remote.Server, m.Remote.WorkbookID)}
}

// OpenLocalCopy ouvre le fichier path tel quel, même celui d'un classeur
// partagé en direct (serveur injoignable, ou choix de l'utilisateur).
func (s *DocumentService) OpenLocalCopy(ctx context.Context, path string) (OpenResult, error) {
	if err := s.sessionPath(path); err != nil {
		return OpenResult{}, err
	}
	if q, together := s.openQuestion(path); q != nil {
		return OpenResult{Locked: q, CoEdit: together}, nil
	}
	return s.open(ctx, path, doc.OpenNormal)
}

// OpenLocked ouvre un classeur verrouillé par un autre poste : en lecture
// seule (Enregistrer sous seulement), ou quand même (sans prendre son
// verrou ; l'enregistrement vérifie encore que le fichier n'a pas changé).
func (s *DocumentService) OpenLocked(ctx context.Context, path string, readOnly bool) (OpenResult, error) {
	if err := s.sessionPath(path); err != nil {
		return OpenResult{}, err
	}
	mode := doc.OpenAnyway
	if readOnly {
		mode = doc.OpenReadOnly
	}
	return s.open(ctx, path, mode)
}

func (s *DocumentService) open(ctx context.Context, path string, mode doc.OpenMode) (OpenResult, error) {
	d, err := s.openDoc(ctx, path, mode)
	var rec *archive.RecoverableError
	if errors.As(err, &rec) {
		return OpenResult{Recoverable: &rec.Orphan}, nil
	}
	var bad *archive.OpenError
	if errors.As(err, &bad) && bad.Salvage {
		return OpenResult{Damaged: &DamagedFile{Path: path, Message: msg.Text(s.sessionLang(), err)}}, nil
	}
	if err != nil {
		return OpenResult{}, err
	}
	s.replace(d)
	addRecentDoc(d)
	st, err := s.State(ctx)
	st.Notice = d.TakeOpenNoticeIn(s.sessionLang())
	return OpenResult{State: st}, err
}

// Salvage récupère les données lisibles d'un classeur endommagé dans un
// nouveau classeur sans titre (le fichier endommagé n'est pas modifié).
func (s *DocumentService) Salvage(ctx context.Context, path string) (SalvageResult, error) {
	if err := s.sessionPath(path); err != nil {
		return SalvageResult{}, err
	}
	d, rep, err := doc.Salvage(ctx, path)
	if err != nil {
		return SalvageResult{}, msg.Wrap(err, "doc.recoverFailed", msg.Args{"err": err})
	}
	s.replace(d)
	id := msg.ID("doc.salvaged")
	if len(rep.Damaged) > 0 {
		id = "doc.salvagedIncomplete"
	}
	report := s.say(id, msg.Args{"rows": rep.Rows, "tables": rep.Tables, "damaged": len(rep.Damaged)})
	st, err := s.State(ctx)
	st.Notice = d.TakeOpenNoticeIn(s.sessionLang())
	return SalvageResult{State: st, Report: report}, err
}

// Recover rouvre une session interrompue (répertoire de travail dir).
func (s *DocumentService) Recover(ctx context.Context, dir string) (DocState, error) {
	orphans, err := archive.Orphans()
	if err != nil {
		return DocState{}, err
	}
	for _, o := range orphans {
		if o.Dir == dir {
			d, err := doc.Recover(ctx, o)
			if err != nil {
				return DocState{}, err
			}
			s.replace(d)
			st, err := s.State(ctx)
			st.Notice = d.TakeOpenNoticeIn(s.sessionLang())
			return st, err
		}
	}
	return DocState{}, msg.New("session.notFound", nil)
}

// Discard abandonne une session interrompue.
func (s *DocumentService) Discard(dir string) error {
	orphans, err := archive.Orphans()
	if err != nil {
		return err
	}
	for _, o := range orphans {
		if o.Dir == dir {
			return archive.Discard(o)
		}
	}
	return nil
}

// Save enregistre ; as (ou un classeur jamais enregistré) demande le fichier.
func (s *DocumentService) Save(ctx context.Context, as bool) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	// Conversation de l'assistant gardée avec le classeur, si demandé.
	if err := s.storeConversation(ctx, d); err != nil {
		return DocState{}, msg.Wrap(err, "ai.conversationNotSaved", msg.Args{"err": err})
	}
	if s.listening(api.EventBeforeSave) {
		r := s.fireSync(api.EventBeforeSave, map[string]any{"path": d.Path(), "as": as || d.Path() == ""})
		if r.Cancelled {
			st, err := s.State(ctx)
			st.Notice = s.say("doc.saveCancelledByScript", nil)
			return st, err
		}
	}
	switch {
	case publicDemo.Load():
		// Démo publique : le classeur est téléchargé, jamais gardé.
		return s.demoSave(ctx, d)
	case d.ReadOnly():
		// Ouvert en lecture seule : une copie, sous un autre nom.
		return s.saveAs(ctx, d, docCopyName(d))
	case as && d.XLSX():
		return s.saveAs(ctx, d, docBaseName(d)+".xlsx")
	case as || d.Path() == "":
		// Classeur du serveur : son titre, pas le nom de son cache.
		return s.saveAs(ctx, d, docBaseName(d)+".lattis")
	case d.XLSX():
		// Fichier .xlsx : garder son format ou convertir (xlsxdoc.go).
		return s.saveXLSXDoc(ctx, d)
	}
	if err := saveDoc(ctx, d, ""); err != nil {
		return s.saveFailed(ctx, d, err)
	}
	return s.saved(ctx, d)
}

// SaveProgress est l'avancement d'un enregistrement : phase vide s'il n'y
// en a pas, sinon snapshot, scrub, write (done octets écrits sur total) ou
// verify.
type SaveProgress struct {
	Phase string `json:"phase"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

// SaveProgress donne l'avancement de l'enregistrement en cours ; l'interface
// l'interroge pendant Save pour un gros classeur.
func (s *DocumentService) SaveProgress() SaveProgress {
	d, err := s.current()
	if err != nil {
		return SaveProgress{}
	}
	p := d.SaveProgress()
	return SaveProgress{Phase: p.Phase, Done: p.Done, Total: p.Total}
}

// ResolveSave répond à DocState.Conflict (fichier modifié par ailleurs
// depuis son ouverture) : overwrite l'écrase, sinon le classeur est
// enregistré sous un autre nom (« Enregistrer une copie »).
func (s *DocumentService) ResolveSave(ctx context.Context, overwrite bool) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	if !overwrite {
		return s.saveAs(ctx, d, docCopyName(d))
	}
	if err := d.Overwrite(ctx); err != nil {
		return DocState{}, err
	}
	return s.saved(ctx, d)
}

// saveAs demande où enregistrer (name : nom proposé) et enregistre.
func (s *DocumentService) saveAs(ctx context.Context, d *doc.Document, name string) (DocState, error) {
	var werr error
	// Enregistrer sous : .lattis ou .xlsx (le format proposé d'abord).
	filters := []FileFilter{{"Classeur Lattis", "*.lattis"}, {"Classeur Excel", "*.xlsx"}}
	if archive.Foreign(name) {
		filters[0], filters[1] = filters[1], filters[0]
	}
	p, err := s.sys().SaveFile(FileDialog{
		Title:     "Enregistrer le classeur",
		Filename:  name,
		Directory: docFolder(d),
		Filters:   filters,
	}, func(p string) error {
		werr = saveDoc(ctx, d, saveAsPath(p))
		return werr
	})
	if werr != nil {
		return s.saveFailed(ctx, d, werr)
	}
	if err != nil || p == "" {
		return s.State(ctx)
	}
	return s.saved(ctx, d)
}

// saveFailed : un fichier modifié par ailleurs devient une question
// (DocState.Conflict), les autres échecs restent des erreurs.
func (s *DocumentService) saveFailed(ctx context.Context, d *doc.Document, err error) (DocState, error) {
	var changed *archive.ChangedError
	if !errors.As(err, &changed) {
		return DocState{}, err
	}
	st, serr := s.State(ctx)
	st.Conflict = &FileQuestion{Path: d.Path(), Message: msg.Text(s.sessionLang(), changed)}
	return st, serr
}

func (s *DocumentService) saved(ctx context.Context, d *doc.Document) (DocState, error) {
	addRecentDoc(d)
	// Sources du Data Lake envoyées à chaque enregistrement (en arrière-plan).
	s.pushOnSave(d)
	if s.listening(api.EventSave) {
		s.fireAsync(api.EventSave, map[string]any{"path": d.Path()})
	}
	st, err := s.changed(ctx)
	// Classeur du serveur : ce qui est arrivé au serveur, pas le chemin du
	// cache (l'interface montre sinon « Enregistré : <chemin> »).
	if st.Remote != nil && st.Notice == "" && !st.Dirty {
		st.Notice = remoteSavedNoticeIn(s.sessionLang(), st.Remote)
	}
	// Fichier de l'auteur d'un classeur partagé en direct : enregistré sur
	// le poste, et synchronisé avec le serveur comme un classeur ouvert
	// depuis le serveur ; le message dit les deux.
	if st.Remote == nil && st.Notice == "" && !st.Dirty && d.Share != nil {
		if r := d.Share.State(); r.Shared && r.Mode == workbook.Collaborative && r.Gone == "" {
			id := msg.ID("share.savedAlsoSent")
			if !r.Online {
				id = "share.savedOfflineLater"
			}
			st.Notice = s.say(id, msg.Args{"path": st.Path, "server": serverLabel(r.Server, r.ServerName)})
		}
	}
	return st, err
}

// docCopyName propose le nom d'une copie du classeur d.
func docCopyName(d *doc.Document) string {
	if remoteDoc(d) != nil {
		return docBaseName(d) + " (copie).lattis"
	}
	return copyName(d.Path())
}

// copyName propose le nom d'une copie : « Ventes.lattis » → « Ventes
// (copie).lattis ».
func copyName(path string) string {
	if path == "" {
		return "Classeur.lattis"
	}
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext) + " (copie)" + ext
}

// HidePaths : le moteur tourne sur un serveur (lattisd) ; DocState.Path ne
// donne que le nom du fichier.
var HidePaths bool

// State décrit le classeur ouvert.
func (s *DocumentService) State(ctx context.Context) (DocState, error) {
	d, err := s.current()
	if errors.Is(err, errNoDoc) {
		return DocState{}, nil
	}
	st := DocState{Open: true, Path: d.Path(), Title: s.say("doc.untitled", nil)}
	if st.Path != "" {
		st.Title = strings.TrimSuffix(filepath.Base(st.Path), filepath.Ext(st.Path))
		// Fichier .xlsx : le nom avec son extension (« Ventes.xlsx »).
		if d.XLSX() {
			st.Title, st.Format = filepath.Base(st.Path), "xlsx"
		}
	}
	// Classeur du serveur : son titre sur le serveur, pas le nom du cache.
	if st.Remote = remoteDoc(d); st.Remote != nil {
		st.Title = st.Remote.Title
	}
	// File d'envoi du classeur précédent qui n'a pas pu être gardée
	// (replace) : dit une fois, au classeur suivant.
	s.mu.Lock()
	st.Notice, s.lost = s.lost, ""
	s.mu.Unlock()
	// Moteur d'une session web : les chemins du serveur ne sont pas montrés.
	if HidePaths && st.Path != "" {
		st.Path = filepath.Base(st.Path)
	}
	if st.ReadOnly = d.ReadOnly(); st.ReadOnly {
		st.Title += " (lecture seule)"
	}
	if st.Dirty, err = d.Dirty(ctx); err != nil {
		return st, err
	}
	// Classeur collaboratif ouvert depuis le serveur : déjà « enregistré »
	// sur le serveur au fil de l'eau (liveCache).
	if st.Dirty && liveCache(d) {
		st.Dirty = false
	}
	if st.Theme, err = d.Theme(ctx); err != nil {
		return st, err
	}
	h, err := d.Bus.State(ctx)
	st.Undo, st.Redo = h.Undo, h.Redo
	return st, err
}

// changed invalide les tableaux chargés que les écritures ont pu toucher
// (les autres gardent leurs lignes et leurs totaux en cache) et prévient
// l'interface.
func (s *DocumentService) changed(ctx context.Context) (DocState, error) {
	var content, structure map[string]bool
	all := true
	if d, err := s.current(); err == nil {
		content, structure, all = d.DB().TakeTouched()
	}
	s.mu.Lock()
	// Tableaux de la grille : chacun sait, par les versions du Store, s'il
	// lit encore ce que contient la base (gridTable le vérifie aussi avant
	// chaque lecture, même sans appel à changed) ; on libère ici les autres.
	for id, t := range s.tables {
		if !t.Fresh() {
			delete(s.tables, id)
		}
	}
	for k, v := range s.memo {
		if all || content[v.sql] || structure[v.sql] {
			delete(s.memo, k)
		}
	}
	s.mu.Unlock()
	// Classeur collaboratif : ce qui vient d'être fait part au serveur ;
	// travail à plusieurs : dans le journal.
	s.kickShare()
	s.kickCoEdit()
	// Données du Data Lake insérées ou supprimées : suivi et « Utilisée par ».
	if d, err := s.current(); err == nil {
		s.dataChanged(d)
	}
	st, err := s.State(ctx)
	if d, derr := s.current(); derr == nil {
		st.Notice = d.TakeNotice()
	}
	if err == nil {
		s.sys().Emit(ChangedEvent, st)
	}
	return st, err
}

// Layout donne la mise en page d'une feuille (tableaux et cellules libres).
func (s *DocumentService) Layout(ctx context.Context, sheet int64) (_ canvas.Layout, err error) {
	d, err := s.current()
	if err != nil {
		return canvas.Layout{}, err
	}
	ctx, t := s.track(ctx, false)
	defer s.untrack(t, &err)
	work.Op(ctx, "layout", "")
	return d.Layout(ctx, sheet)
}

// LayoutFirst donne la mise en page d'une feuille à afficher tout de suite :
// sans ce qui demande de parcourir un grand tableau et n'est pas en cache
// (Pending) ; l'interface demande ensuite Layout, qui le calcule.
func (s *DocumentService) LayoutFirst(ctx context.Context, sheet int64) (canvas.Layout, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Layout{}, err
	}
	return d.LayoutFirst(ctx, sheet)
}

// gridTable donne le tableau prêt à lire par fenêtres (gardé en cache).
func (s *DocumentService) gridTable(ctx context.Context, sheet, table int64) (*grid.Table, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	t := s.tables[table]
	// Écriture passée depuis le chargement (quelle qu'elle soit, et même si
	// changed n'a pas été appelé : script, serveur, autre fenêtre) : relu.
	if t != nil && !t.Fresh() {
		delete(s.tables, table)
		t = nil
	}
	s.mu.Unlock()
	if t != nil {
		return t, nil
	}
	ti, ok, err := s.tableInfo(ctx, d, sheet, table)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, grid.ErrRange
	}
	if t, err = api.LoadGrid(ctx, d, ti); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.tables[table] = t
	s.mu.Unlock()
	return t, nil
}

// tableInfo décrit le tableau table tel que la mise en page de sa feuille
// le montre. La grille donne la feuille affichée ; un bloc demandé juste
// avant un changement d'onglet arrive avec la nouvelle : le tableau est
// alors cherché sur sa propre feuille.
func (s *DocumentService) tableInfo(ctx context.Context, d *doc.Document, sheet, table int64) (canvas.TableInfo, bool, error) {
	find := func(sheet int64) (canvas.TableInfo, bool, error) {
		// Les tableaux seulement : ni TCD ni graphique à calculer avant le
		// premier bloc.
		l, err := d.LayoutFirst(ctx, sheet)
		if err != nil {
			return canvas.TableInfo{}, false, err
		}
		for _, ti := range l.Tables {
			if ti.ID == table {
				return ti, true, nil
			}
		}
		return canvas.TableInfo{}, false, nil
	}
	ti, ok, err := find(sheet)
	if ok || err != nil {
		return ti, ok, err
	}
	var own int64
	switch err := d.DB().Read().QueryRowContext(ctx, `SELECT sheet FROM _lattis_tables WHERE id = ?`, table).Scan(&own); {
	case errors.Is(err, sql.ErrNoRows):
		return ti, false, nil
	case err != nil:
		return ti, false, err
	}
	if own == sheet {
		return ti, false, nil
	}
	return find(own)
}

// Window lit des lignes d'un tableau (lecture par fenêtres).
func (s *DocumentService) Window(ctx context.Context, sheet, table int64, first, count int) (_ grid.Window, err error) {
	ctx, wt := s.track(ctx, false)
	defer s.untrack(wt, &err)
	work.Op(ctx, "load", "")
	t, err := s.gridTable(ctx, sheet, table)
	if err != nil {
		return grid.Window{}, err
	}
	work.Phase(ctx, "window")
	w, err := t.Window(ctx, first, count)
	if errors.Is(err, grid.ErrRange) && first >= 0 && count > 0 {
		// Lignes au-delà de la fin du tableau chargé : la mise en page de
		// l'interface en annonce plus. Relu une fois avant de refuser.
		before := t.Info().Rows
		s.mu.Lock()
		if s.tables[table] == t {
			delete(s.tables, table)
		}
		s.mu.Unlock()
		if t, err = s.gridTable(ctx, sheet, table); err != nil {
			return grid.Window{}, err
		}
		w, err = t.Window(ctx, first, count)
		if err != nil {
			// Le détail reste dans le journal ; l'interface dit seulement
			// que le tableau sera rechargé (readfail.ts).
			info := t.Info()
			slog.Warn("lecture d'un tableau de la grille hors de ses lignes", "table", info.Name, "sql", info.SQLName,
				"feuille", sheet, "premiere", first, "nombre", count, "lignes avant", before, "lignes relues", info.Rows, "err", err)
			writeErrorLog(fmt.Sprintf("=== %s · grille, lecture du tableau « %s »\nlignes %d à %d demandées (feuille %d), %d lignes au chargement, %d après relecture : %v\nsource : %s\n",
				time.Now().Format("2006-01-02 15:04:05"), info.Name, first, first+count-1, sheet, before, info.Rows, err, info.SQLName))
		}
	}
	if err != nil {
		return w, err
	}
	// Classeur collaboratif : marques « en attente » et « conflit ».
	return w, s.syncMarks(ctx, t, table, &w)
}

// QuerySheet décrit un résultat SQL posé sur une nouvelle feuille.
type QuerySheet struct {
	Sheet int64    `json:"sheet"`
	Table string   `json:"table"`
	State DocState `json:"state"`
}

// FormulaHelp sert l'aide à la saisie des formules : fonctions (dans la
// langue de la session) et tableaux du classeur avec leurs colonnes.
type FormulaHelp struct {
	Functions []formula.FunctionInfo `json:"functions"`
	Tables    []canvas.TableNames    `json:"tables"`
}

// ColumnOrderInfo est l'ordre personnalisé enregistré d'une colonne (Order
// nil : aucun) ; Applied : c'est le dernier tri du tableau.
type ColumnOrderInfo struct {
	Order   *canvas.CustomOrder `json:"order"`
	Applied bool                `json:"applied"`
}

// StylePart est un morceau d'une mise en forme en plusieurs plages (bordures
// du ruban : contour, bord inférieur…) : un rectangle, ou des lignes
// (Line "rows", de R0 à R1) ou des colonnes (Line "cols", de C0 à C1)
// entières.
type StylePart struct {
	R0    int            `json:"r0"`
	C0    int            `json:"c0"`
	R1    int            `json:"r1"`
	C1    int            `json:"c1"`
	Line  string         `json:"line,omitempty"`
	Patch map[string]any `json:"patch"`
}

type memoKey struct {
	kind, table string
	col         int
}

type memoVal struct {
	sql string // nom SQL du tableau (t_N), pour l'invalidation
	v   any
}

// remember calcule fn une fois par tableau, colonne et genre de résultat,
// jusqu'à la prochaine modification de ce tableau (voir changed).
func remember[T any](ctx context.Context, s *DocumentService, kind, table string, i int, fn func(*doc.Document) (T, error)) (T, error) {
	var zero T
	d, err := s.current()
	if err != nil {
		return zero, err
	}
	k := memoKey{kind, table, i}
	s.mu.Lock()
	m, ok := s.memo[k]
	s.mu.Unlock()
	if ok {
		if v, ok := m.v.(T); ok {
			return v, nil
		}
	}
	v, err := fn(d)
	if err != nil {
		return v, err
	}
	var id int64
	if err := d.DB().Read().QueryRowContext(ctx, `SELECT id FROM _lattis_tables WHERE name = ?`, table).Scan(&id); err != nil {
		return v, nil // tableau introuvable : résultat non gardé
	}
	s.mu.Lock()
	if s.doc == d {
		if s.memo == nil {
			s.memo = map[memoKey]memoVal{}
		}
		s.memo[k] = memoVal{sql: fmt.Sprintf("t_%d", id), v: v}
	}
	s.mu.Unlock()
	return v, nil
}
