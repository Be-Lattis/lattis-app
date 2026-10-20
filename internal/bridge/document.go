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
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
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

	// Classeur à ouvrir au lancement (association de fichiers, launch.go).
	launch launchState
}

var (
	errNoDoc  = msg.New("doc.none", nil)
	errNoDemo = errors.New("aucune démonstration demandée au lancement (--demo)")
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
func (s *DocumentService) Orphans() ([]archive.Orphan, error) { return archive.Orphans() }

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
	d, err := doc.New(ctx)
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
			Title:   "Ouvrir un classeur",
			Filters: []FileFilter{{"Classeur Lattis", "*.lattis"}},
		})
		if err != nil {
			return OpenResult{}, err
		}
		if p == "" {
			return OpenResult{Cancelled: true}, nil
		}
		path = p
	}
	if r := sharedFile(path); r != nil {
		return OpenResult{Shared: r}, nil
	}
	// Ouvert sur un autre poste (verrou ~$ à côté du fichier, ou sessions
	// du travail à plusieurs) : l'interface propose la lecture seule, et de
	// travailler à plusieurs (ou, sans, de l'ouvrir quand même).
	if q, together := openQuestion(path); q != nil {
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
	if q, together := openQuestion(path); q != nil {
		return OpenResult{Locked: q, CoEdit: together}, nil
	}
	return s.open(ctx, path, doc.OpenNormal)
}

// OpenLocked ouvre un classeur verrouillé par un autre poste : en lecture
// seule (Enregistrer sous seulement), ou quand même (sans prendre son
// verrou ; l'enregistrement vérifie encore que le fichier n'a pas changé).
func (s *DocumentService) OpenLocked(ctx context.Context, path string, readOnly bool) (OpenResult, error) {
	mode := doc.OpenAnyway
	if readOnly {
		mode = doc.OpenReadOnly
	}
	return s.open(ctx, path, mode)
}

func (s *DocumentService) open(ctx context.Context, path string, mode doc.OpenMode) (OpenResult, error) {
	d, err := doc.OpenAs(ctx, path, mode)
	var rec *archive.RecoverableError
	if errors.As(err, &rec) {
		return OpenResult{Recoverable: &rec.Orphan}, nil
	}
	var bad *archive.OpenError
	if errors.As(err, &bad) && bad.Salvage {
		return OpenResult{Damaged: &DamagedFile{Path: path, Message: err.Error()}}, nil
	}
	if err != nil {
		return OpenResult{}, err
	}
	s.replace(d)
	addRecentDoc(d)
	st, err := s.State(ctx)
	st.Notice = d.TakeOpenNotice()
	return OpenResult{State: st}, err
}

// Salvage récupère les données lisibles d'un classeur endommagé dans un
// nouveau classeur sans titre (le fichier endommagé n'est pas modifié).
func (s *DocumentService) Salvage(ctx context.Context, path string) (SalvageResult, error) {
	d, rep, err := doc.Salvage(ctx, path)
	if err != nil {
		return SalvageResult{}, fmt.Errorf("récupération impossible : %w", err)
	}
	s.replace(d)
	msg := fmt.Sprintf("Données récupérées : %d ligne(s) dans %d table(s)", rep.Rows, rep.Tables)
	if len(rep.Damaged) > 0 {
		msg += fmt.Sprintf(", %d table(s) incomplète(s)", len(rep.Damaged))
	}
	msg += ". Enregistrer sous un nouveau nom, puis vérifier le contenu."
	st, err := s.State(ctx)
	st.Notice = d.TakeOpenNotice()
	return SalvageResult{State: st, Report: msg}, err
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
			st.Notice = d.TakeOpenNotice()
			return st, err
		}
	}
	return DocState{}, errors.New("session introuvable")
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
		return DocState{}, fmt.Errorf("conversation de l'assistant non enregistrée : %w", err)
	}
	if s.listening(api.EventBeforeSave) {
		r := s.fireSync(api.EventBeforeSave, map[string]any{"path": d.Path(), "as": as || d.Path() == ""})
		if r.Cancelled {
			st, err := s.State(ctx)
			st.Notice = "Enregistrement annulé par un script (évènement before_save)"
			return st, err
		}
	}
	switch {
	case d.ReadOnly():
		// Ouvert en lecture seule : une copie, sous un autre nom.
		return s.saveAs(ctx, d, docCopyName(d))
	case as || d.Path() == "":
		// Classeur du serveur : son titre, pas le nom de son cache.
		return s.saveAs(ctx, d, docBaseName(d)+".lattis")
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
	p, err := s.sys().SaveFile(FileDialog{
		Title:     "Enregistrer le classeur",
		Filename:  name,
		Directory: docFolder(d),
		Filters:   []FileFilter{{"Classeur Lattis", "*.lattis"}},
		Ext:       ".lattis",
	}, func(p string) error {
		werr = saveDoc(ctx, d, p)
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
	st.Conflict = &FileQuestion{Path: d.Path(), Message: changed.Error()}
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
		st.Notice = remoteSavedNotice(st.Remote)
	}
	// Fichier de l'auteur d'un classeur partagé en direct : enregistré sur
	// le poste, et synchronisé avec le serveur comme un classeur ouvert
	// depuis le serveur ; le message dit les deux.
	if st.Remote == nil && st.Notice == "" && !st.Dirty && d.Share != nil {
		if r := d.Share.State(); r.Shared && r.Mode == workbook.Collaborative && r.Gone == "" {
			where := serverLabel(r.Server, r.ServerName)
			if r.Online {
				st.Notice = fmt.Sprintf("Enregistré : %s ; vos modifications partent aussi vers %s", st.Path, where)
			} else {
				st.Notice = fmt.Sprintf("Enregistré : %s ; hors ligne, vos modifications partiront vers %s à la reconnexion", st.Path, where)
			}
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
	st := DocState{Open: true, Path: d.Path(), Title: "Classeur sans titre"}
	if st.Path != "" {
		st.Title = strings.TrimSuffix(filepath.Base(st.Path), filepath.Ext(st.Path))
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
func (s *DocumentService) Layout(ctx context.Context, sheet int64) (canvas.Layout, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Layout{}, err
	}
	return d.Layout(ctx, sheet)
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
		l, err := d.Layout(ctx, sheet)
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
func (s *DocumentService) Window(ctx context.Context, sheet, table int64, first, count int) (grid.Window, error) {
	t, err := s.gridTable(ctx, sheet, table)
	if err != nil {
		return grid.Window{}, err
	}
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

// RangeStats résume des cellules d'un tableau pour la barre d'état
// (Moyenne, Nb, Somme), en SQL : lignes first à first+count-1, colonnes cols.
func (s *DocumentService) RangeStats(ctx context.Context, sheet, table int64, first, count int, cols []int) (grid.Stats, error) {
	t, err := s.gridTable(ctx, sheet, table)
	if err != nil {
		return grid.Stats{}, err
	}
	return t.Stats(ctx, first, count, cols)
}

// RuleStats donne les statistiques de colonnes d'un tableau pour la mise
// en forme conditionnelle (10 premiers, moyenne, doublons, barres de
// données, nuances, icônes), calculées en SQL et gardées en cache.
func (s *DocumentService) RuleStats(ctx context.Context, sheet, table int64, cols []int, req grid.RuleStatsReq) (grid.RuleStats, error) {
	t, err := s.gridTable(ctx, sheet, table)
	if err != nil {
		return grid.RuleStats{}, err
	}
	return t.RuleStats(ctx, cols, req)
}

// SetCell traite une saisie validée dans une cellule de la feuille.
func (s *DocumentService) SetCell(ctx context.Context, sheet int64, row, col int, raw string) (canvas.Event, error) {
	return s.SetCellKeyed(ctx, sheet, row, col, raw, "")
}

// SetCellKeyed est SetCell avec la réponse à la question de la clé (refus
// « clé à choisir ») : "index" (numéros de ligne) ou "column" (la colonne
// cherchée devient la clé), dans la même étape d'annulation que la saisie.
func (s *DocumentService) SetCellKeyed(ctx context.Context, sheet int64, row, col int, raw, key string) (canvas.Event, error) {
	ctx = canvas.WithKeyChoice(ctx, canvas.KeyChoice(key))
	d, err := s.current()
	if err != nil {
		return canvas.Event{}, err
	}
	w := s.watchChange(sheet, row, col, row, col, "edit")
	ev, err := d.SetCell(ctx, sheet, row, col, raw)
	if err != nil {
		return ev, err
	}
	_, err = s.changed(ctx)
	s.fireChange(w)
	return ev, err
}

// Undo et Redo annulent et rétablissent.
func (s *DocumentService) Undo(ctx context.Context) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	if _, err := d.Bus.Undo(ctx); err != nil {
		return DocState{}, err
	}
	// Classeur collaboratif : cellules modifiées par un collègue depuis,
	// laissées à sa valeur (§7).
	d.Notify(collab.UndoNotice(d.Oplog, false))
	return s.changed(ctx)
}

func (s *DocumentService) Redo(ctx context.Context) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	if _, err := d.Bus.Redo(ctx); err != nil {
		return DocState{}, err
	}
	d.Notify(collab.UndoNotice(d.Oplog, true))
	return s.changed(ctx)
}

// InsertRow, DeleteRow, InsertColumn et DeleteColumn agissent sur un tableau.
// MoveCells déplace les cellules libres d'une plage de (dr, dc).
func (s *DocumentService) MoveCells(ctx context.Context, sheet int64, r0, c0, r1, c1, dr, dc int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.MoveCells(ctx, sheet, r0, c0, r1, c1, dr, dc) })
}

// InsertSheetRows insère n lignes entières avant la ligne at de la feuille.
func (s *DocumentService) InsertSheetRows(ctx context.Context, sheet int64, at, n int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.InsertSheetRows(ctx, sheet, at, n) })
}

// DeleteSheetRows supprime les lignes entières r0 à r1 de la feuille.
func (s *DocumentService) DeleteSheetRows(ctx context.Context, sheet int64, r0, r1 int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteSheetRows(ctx, sheet, r0, r1) })
}

// InsertSheetColumns insère n colonnes entières avant la colonne at de la feuille.
func (s *DocumentService) InsertSheetColumns(ctx context.Context, sheet int64, at, n int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.InsertSheetColumns(ctx, sheet, at, n) })
}

// DeleteSheetColumns supprime les colonnes entières c0 à c1 de la feuille.
func (s *DocumentService) DeleteSheetColumns(ctx context.Context, sheet int64, c0, c1 int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteSheetColumns(ctx, sheet, c0, c1) })
}

func (s *DocumentService) InsertRow(ctx context.Context, table string, n int) (DocState, error) {
	w := s.watchTableRow("", 0, "insert_row")
	st, err := s.act(ctx, func(d *doc.Document) error { return d.InsertRow(ctx, table, n) })
	if err == nil {
		s.fireTableRow(w, table, n)
	}
	return st, err
}

func (s *DocumentService) DeleteRow(ctx context.Context, table string, n int) (DocState, error) {
	w := s.watchTableRow(table, n, "delete_row")
	st, err := s.act(ctx, func(d *doc.Document) error { return d.DeleteRow(ctx, table, n) })
	if err == nil {
		s.fireTableRow(w, table, n)
	}
	return st, err
}

func (s *DocumentService) InsertColumn(ctx context.Context, table string, i int, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.InsertColumn(ctx, table, i, name) })
}

func (s *DocumentService) DeleteColumn(ctx context.Context, table string, i int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteColumn(ctx, table, i) })
}

func (s *DocumentService) act(ctx context.Context, fn func(*doc.Document) error) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	if err := fn(d); err != nil {
		return DocState{}, err
	}
	return s.changed(ctx)
}

// SetActiveSheet indique la feuille affichée : les adresses Cell_A1 sans
// feuille de l'onglet SQL s'y lisent.
func (s *DocumentService) SetActiveSheet(sheet int64) {
	s.mu.Lock()
	s.sheet = sheet
	s.mu.Unlock()
}

func (s *DocumentService) activeSheet() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sheet == 0 {
		return 1
	}
	return s.sheet
}

// Query exécute une requête SQL en lecture seule sur le classeur (onglet SQL).
func (s *DocumentService) Query(ctx context.Context, q string) (grid.QueryResult, error) {
	d, err := s.current()
	if err != nil {
		return grid.QueryResult{}, err
	}
	if q, err = d.CellSQL(ctx, q, s.activeSheet()); err != nil {
		return grid.QueryResult{}, err
	}
	// Les tableaux croisés cités s'interrogent comme des vues.
	if q, err = d.ReadCTE(ctx, q); err != nil {
		return grid.QueryResult{}, err
	}
	qctx, done := s.sqlContext(ctx, 0) // délai de grid.Query
	defer done()
	res, err := grid.Query(qctx, d.DB().Read(), q, QueryLimit)
	res = res.WithoutTechnical() // _pos… : jamais dans l'aperçu
	return res, sqlErr(qctx, err)
}

// QuerySheet décrit un résultat SQL posé sur une nouvelle feuille.
type QuerySheet struct {
	Sheet int64    `json:"sheet"`
	Table string   `json:"table"`
	State DocState `json:"state"`
}

// QueryToSheet exécute une requête de lecture (onglet SQL) et pose tout son
// résultat dans un nouveau tableau, sur une nouvelle feuille nommée name
// (vide : « Requête1 »…) ; headers : en-têtes affichés. Annulable.
func (s *DocumentService) QueryToSheet(ctx context.Context, q, name string, headers []string) (QuerySheet, error) {
	d, err := s.current()
	if err != nil {
		return QuerySheet{}, err
	}
	if q, err = d.CellSQL(ctx, q, s.activeSheet()); err != nil {
		return QuerySheet{}, err
	}
	if q, err = d.ReadCTE(ctx, q); err != nil {
		return QuerySheet{}, err
	}
	qctx, done := s.sqlContext(ctx, sqlWriteTimeout())
	res, err := d.QueryToSheet(qctx, q, name, headers)
	err = sqlErr(qctx, err)
	done()
	if err != nil {
		return QuerySheet{}, err
	}
	out := QuerySheet{}
	if len(res.Sheets) > 0 {
		out.Sheet = res.Sheets[0]
	}
	if len(res.Tables) > 0 {
		out.Table = res.Tables[0]
	}
	out.State, err = s.changed(ctx)
	return out, err
}

// Paste colle un bloc (presse-papiers d'Excel) à partir d'une cellule.
func (s *DocumentService) Paste(ctx context.Context, sheet int64, row, col int, block [][]string) (canvas.Event, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Event{}, err
	}
	width := 1
	for _, r := range block {
		width = max(width, len(r))
	}
	w := s.watchChange(sheet, row, col, row+max(len(block), 1)-1, col+width-1, "paste")
	ev, err := d.Paste(ctx, sheet, row, col, block)
	if err != nil {
		return ev, err
	}
	_, err = s.changed(ctx)
	s.fireChange(w)
	return ev, err
}

// AddSheet ajoute un onglet à la fin et renvoie son identifiant.
func (s *DocumentService) AddSheet(ctx context.Context) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.AddSheet(ctx)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	s.fireSheet(api.EventSheetAdd, id)
	return id, err
}

// InsertSheet ajoute un onglet avant l'onglet before et renvoie son identifiant.
func (s *DocumentService) InsertSheet(ctx context.Context, before int64) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.InsertSheet(ctx, before)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	s.fireSheet(api.EventSheetAdd, id)
	return id, err
}

// MoveSheet place un onglet au rang to (0 : premier).
func (s *DocumentService) MoveSheet(ctx context.Context, sheet int64, to int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.MoveSheet(ctx, sheet, to) })
}

// RenameSheet et DeleteSheet agissent sur un onglet.
func (s *DocumentService) RenameSheet(ctx context.Context, sheet int64, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RenameSheetLinked(ctx, sheet, name) })
}

func (s *DocumentService) DeleteSheet(ctx context.Context, sheet int64) (DocState, error) {
	var data map[string]any
	if s.listening(api.EventSheetDelete) {
		data = api.SheetEventData(host{s}, sheet)
	}
	st, err := s.act(ctx, func(d *doc.Document) error { return d.DeleteSheet(ctx, sheet) })
	if err == nil && data != nil {
		s.fireAsync(api.EventSheetDelete, data)
	}
	return st, err
}

// CopySheet copie un onglet (tableaux, cellules, mise en forme, objets) et
// place la copie au rang to ; renvoie l'identifiant de la copie.
func (s *DocumentService) CopySheet(ctx context.Context, sheet int64, to int) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.CopySheet(ctx, sheet, to)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	s.fireSheet(api.EventSheetAdd, id)
	return id, err
}

// SetSheetColor fixe la couleur d'un onglet (« #rrggbb », vide : aucune).
func (s *DocumentService) SetSheetColor(ctx context.Context, sheet int64, color string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSheetColor(ctx, sheet, color) })
}

// SetSheetsColor colore les onglets d'une sélection de feuilles, en une étape
// d'annulation.
func (s *DocumentService) SetSheetsColor(ctx context.Context, sheets []int64, color string) (DocState, error) {
	gctx := command.WithGroup(ctx, "Couleur d'onglet")
	return s.act(ctx, func(d *doc.Document) error {
		for _, id := range sheets {
			if err := d.SetSheetColor(gctx, id, color); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetSheetsHidden masque ou affiche une sélection de feuilles, en une étape
// d'annulation.
func (s *DocumentService) SetSheetsHidden(ctx context.Context, sheets []int64, hidden bool) (DocState, error) {
	label := "Masquer les feuilles"
	if !hidden {
		label = "Afficher les feuilles"
	}
	gctx := command.WithGroup(ctx, label)
	return s.act(ctx, func(d *doc.Document) error {
		for _, id := range sheets {
			if err := d.SetSheetHidden(gctx, id, hidden); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetSheetHidden masque ou affiche un onglet.
func (s *DocumentService) SetSheetHidden(ctx context.Context, sheet int64, hidden bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSheetHidden(ctx, sheet, hidden) })
}

// ConvertPreview compte les valeurs qui ne se convertiraient pas au type to.
func (s *DocumentService) ConvertPreview(ctx context.Context, table string, i int, to grid.Type) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	return d.ConvertPreview(ctx, table, i, to)
}

// TypeMix donne le type majoritaire d'une colonne passée en texte par
// élargissement, et le nombre de valeurs qui n'en sont pas.
func (s *DocumentService) TypeMix(ctx context.Context, table string, i int) (canvas.Mix, error) {
	return remember(ctx, s, "mix", table, i, func(d *doc.Document) (canvas.Mix, error) { return d.TypeMix(ctx, table, i) })
}

// SetType fixe le type d'une colonne (valeurs non convertibles marquées en erreur).
func (s *DocumentService) SetType(ctx context.Context, table string, i int, to grid.Type) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetType(ctx, table, i, to) })
}

// SetColumnWidth fixe la largeur d'une colonne de la feuille.
func (s *DocumentService) SetColumnWidth(ctx context.Context, sheet int64, col, width int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetColumnWidth(ctx, sheet, col, width) })
}

// FormulaHelp sert l'aide à la saisie des formules : fonctions (en
// français) et tableaux du classeur avec leurs colonnes.
type FormulaHelp struct {
	Functions []formula.FunctionInfo `json:"functions"`
	Tables    []canvas.TableNames    `json:"tables"`
}

// FormulaHelp renvoie le catalogue de l'aide à la saisie.
func (s *DocumentService) FormulaHelp(ctx context.Context) (FormulaHelp, error) {
	h := FormulaHelp{Functions: formula.Functions(formula.FR), Tables: []canvas.TableNames{}}
	d, err := s.current()
	if err != nil {
		return h, nil
	}
	h.Tables, err = d.AllTables(ctx)
	return h, err
}

// SetName nomme une cellule libre (vide : retire le nom).
func (s *DocumentService) SetName(ctx context.Context, sheet int64, row, col int, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetName(ctx, sheet, row, col, name) })
}

// SortTable trie un tableau selon la colonne i (desc : décroissant).
func (s *DocumentService) SortTable(ctx context.Context, table string, i int, desc bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SortTable(ctx, table, i, desc) })
}

// SortTableCustom trie un tableau selon la colonne i dans un ordre
// personnalisé, enregistré pour la colonne.
func (s *DocumentService) SortTableCustom(ctx context.Context, table string, i int, order canvas.CustomOrder) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SortTableCustom(ctx, table, i, order) })
}

// ColumnOrderInfo est l'ordre personnalisé enregistré d'une colonne (Order
// nil : aucun) ; Applied : c'est le dernier tri du tableau.
type ColumnOrderInfo struct {
	Order   *canvas.CustomOrder `json:"order"`
	Applied bool                `json:"applied"`
}

// ColumnOrder rend l'ordre personnalisé enregistré de la colonne i.
func (s *DocumentService) ColumnOrder(ctx context.Context, table string, i int) (ColumnOrderInfo, error) {
	d, err := s.current()
	if err != nil {
		return ColumnOrderInfo{}, err
	}
	o, ok, applied, err := d.ColumnOrder(ctx, table, i)
	if err != nil || !ok {
		return ColumnOrderInfo{}, err
	}
	return ColumnOrderInfo{Order: &o, Applied: applied}, nil
}

// CustomLists rend les listes personnalisées (prédéfinies puis du classeur).
func (s *DocumentService) CustomLists(ctx context.Context) ([]canvas.CustomList, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.CustomLists(ctx)
}

// SetTheme change le thème de couleur du classeur (nil : thème par défaut
// des réglages), annulable.
func (s *DocumentService) SetTheme(ctx context.Context, th *canvas.Theme) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetTheme(ctx, th) })
}

// SetCustomLists remplace les listes personnalisées du classeur.
func (s *DocumentService) SetCustomLists(ctx context.Context, lists []canvas.CustomList) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetCustomLists(ctx, lists) })
}

// ApplyStyle met en forme un rectangle de la feuille (patch : propriétés à
// poser, une valeur vide les retire).
func (s *DocumentService) ApplyStyle(ctx context.Context, sheet int64, r0, c0, r1, c1 int, patch map[string]any) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ApplyStyle(ctx, sheet, r0, c0, r1, c1, patch) })
}

// ApplyLineStyle met en forme des lignes (rows) ou des colonnes entières
// [i0, i1] de la feuille, comme Excel.
func (s *DocumentService) ApplyLineStyle(ctx context.Context, sheet int64, rows bool, i0, i1 int, patch map[string]any) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ApplyLineStyle(ctx, sheet, rows, i0, i1, patch) })
}

// SetNote pose la note d'une cellule (texte vide : la retire).
func (s *DocumentService) SetNote(ctx context.Context, sheet int64, row, col int, text string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetNote(ctx, sheet, row, col, text) })
}

// MergeCells fusionne et centre des cellules libres ; renvoie le nombre de
// valeurs effacées (seule celle en haut à gauche est gardée).
func (s *DocumentService) MergeCells(ctx context.Context, sheet int64, r0, c0, r1, c1 int) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	n, err := d.MergeCells(ctx, sheet, r0, c0, r1, c1)
	if err == nil {
		s.changed(ctx)
	}
	return n, err
}

// UnmergeCells annule les fusions qui touchent un rectangle.
func (s *DocumentService) UnmergeCells(ctx context.Context, sheet int64, r0, c0, r1, c1 int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.UnmergeCells(ctx, sheet, r0, c0, r1, c1) })
}

// SetLinesHidden masque ou affiche des lignes (rows) ou des colonnes [i0, i1].
func (s *DocumentService) SetLinesHidden(ctx context.Context, sheet int64, rows bool, i0, i1 int, hide bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetLinesHidden(ctx, sheet, rows, i0, i1, hide) })
}

// SetFreeze fige les volets de la feuille (0, 0 : les libère).
func (s *DocumentService) SetFreeze(ctx context.Context, sheet int64, rows, cols int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetFreeze(ctx, sheet, rows, cols) })
}

// --- Onglet à choix (canvas/choice.go) ---------------------------------------

// SheetChoiceValues donne les valeurs proposées par la flèche d'un onglet à
// choix (lignes de chacune ; 0 : valeur ajoutée sans ligne encore).
func (s *DocumentService) SheetChoiceValues(ctx context.Context, sheet int64) ([]canvas.FilterValue, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.SheetChoiceValues(ctx, sheet)
}

// SheetChoiceCandidates donne les colonnes des tableaux de la feuille que
// « Choisir par… » propose (peu de valeurs différentes).
func (s *DocumentService) SheetChoiceCandidates(ctx context.Context, sheet int64) ([]canvas.ChoiceCandidate, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.SheetChoiceCandidates(ctx, sheet)
}

// ChooseSheetBy fait de la feuille un onglet à choix par une colonne d'un de
// ses tableaux (value vide : toutes les valeurs) ; annulable.
func (s *DocumentService) ChooseSheetBy(ctx context.Context, sheet int64, table, column, value string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSheetChoice(ctx, sheet, table, column, value) })
}

// ChooseSheetByFormula fait de la feuille un onglet à choix par une formule
// sur les colonnes des lignes (« Choisir par › Une formule… ») ; annulable.
func (s *DocumentService) ChooseSheetByFormula(ctx context.Context, sheet int64, name, formula, value string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSheetChoiceFormula(ctx, sheet, name, formula, value) })
}

// PreviewChoiceFormula donne l'aperçu d'une catégorie calculée : premières
// valeurs, tableaux qui suivraient, ou l'erreur de la formule.
func (s *DocumentService) PreviewChoiceFormula(ctx context.Context, sheet int64, formula string) (canvas.ChoicePreview, error) {
	d, err := s.current()
	if err != nil {
		return canvas.ChoicePreview{}, err
	}
	return d.PreviewChoiceFormula(ctx, sheet, formula)
}

// ChooseSheetValue règle l'onglet à choix sur une valeur (all : toutes) ;
// de la navigation, hors du journal d'annulation.
func (s *DocumentService) ChooseSheetValue(ctx context.Context, sheet int64, value string, all bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ChooseSheetValue(ctx, sheet, value, all) })
}

// NewSheetChoiceValue ajoute une valeur à l'onglet à choix et l'affiche
// (« Nouvelle année… ») ; annulable.
func (s *DocumentService) NewSheetChoiceValue(ctx context.Context, sheet int64, value string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.NewSheetChoiceValue(ctx, sheet, value) })
}

// ClearSheetChoice retire l'onglet à choix de la feuille ; annulable.
func (s *DocumentService) ClearSheetChoice(ctx context.Context, sheet int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ClearSheetChoice(ctx, sheet) })
}

// SetZoom fixe le zoom de la feuille, en pour cent (10 à 400) ; hors du
// journal d'annulation.
func (s *DocumentService) SetZoom(ctx context.Context, sheet int64, pct int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetZoom(ctx, sheet, pct) })
}

// Find cherche un texte dans une feuille (cellules dans l'ordre de lecture),
// limité à la zone in quand elle est donnée (la sélection) ; nil : toute la
// feuille.
func (s *DocumentService) Find(ctx context.Context, sheet int64, text string, in *canvas.Rect) ([]canvas.Match, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.FindIn(ctx, sheet, text, in)
}

// Replace remplace un texte dans la zone in (la sélection, ou la cellule
// trouvée), ou dans toute la feuille si elle est nil, et renvoie le nombre
// de cellules modifiées.
func (s *DocumentService) Replace(ctx context.Context, sheet int64, text, by string, in *canvas.Rect) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	n, err := d.ReplaceIn(ctx, sheet, text, by, in)
	if err == nil && n > 0 {
		s.changed(ctx)
	}
	return n, err
}

// SetRules remplace les règles de mise en forme conditionnelle d'une colonne.
func (s *DocumentService) SetRules(ctx context.Context, table string, i int, rules []canvas.Rule) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetRules(ctx, table, i, rules) })
}

// SetFilter remplace le filtre d'une colonne (nil : retire le filtre).
func (s *DocumentService) SetFilter(ctx context.Context, table string, i int, f *canvas.ColumnFilter) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetFilter(ctx, table, i, f) })
}

// ClearFilters retire tous les filtres d'un tableau.
func (s *DocumentService) ClearFilters(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ClearFilters(ctx, table) })
}

// FilterValues donne les valeurs distinctes d'une colonne pour le filtre.
func (s *DocumentService) FilterValues(ctx context.Context, table string, i int) ([]canvas.FilterValue, error) {
	return remember(ctx, s, "values", table, i, func(d *doc.Document) ([]canvas.FilterValue, error) { return d.FilterValues(ctx, table, i) })
}

// RenameTable renomme un tableau.
func (s *DocumentService) RenameTable(ctx context.Context, old, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RenameTable(ctx, old, name) })
}

// DeleteTables supprime des tableaux entiers.
func (s *DocumentService) DeleteTables(ctx context.Context, names []string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteTables(ctx, names) })
}

// ConvertToRange convertit un tableau en plage de cellules libres.
func (s *DocumentService) ConvertToRange(ctx context.Context, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ConvertToRange(ctx, name) })
}

// ColumnsToValues convertit des colonnes calculées en colonnes de valeurs.
func (s *DocumentService) ColumnsToValues(ctx context.Context, table string, cols []int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ColumnsToValues(ctx, table, cols) })
}

// RangeToValues convertit en valeurs les colonnes calculées et les cellules
// libres avec formule de la plage.
func (s *DocumentService) RangeToValues(ctx context.Context, sheet int64, r0, c0, r1, c1 int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RangeToValues(ctx, sheet, r0, c0, r1, c1) })
}

// FormatAsTable refait un tableau de la plage qui contient la cellule.
func (s *DocumentService) FormatAsTable(ctx context.Context, sheet int64, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.FormatAsTable(ctx, sheet, row, col)
		return err
	})
}

// ConvertSpill remplace le résultat déversé de la formule de la cellule
// par un tableau : lié (recalculé quand ses sources changent) ou de
// valeurs, en une seule étape d'annulation.
func (s *DocumentService) ConvertSpill(ctx context.Context, sheet int64, row, col int, linked bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.ConvertSpill(ctx, sheet, row, col, linked)
		return err
	})
}

// MoveTable déplace un tableau (fusion s'il est posé côte à côte d'un autre).
func (s *DocumentService) MoveTable(ctx context.Context, name string, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.MoveTable(ctx, name, row, col) })
}

// ExtractColumns sort des colonnes d'un tableau vers un nouveau tableau.
func (s *DocumentService) ExtractColumns(ctx context.Context, name string, i0, i1, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ExtractColumns(ctx, name, i0, i1, row, col) })
}

// Recalc renouvelle AUJOURDHUI, MAINTENANT et ALEA (F9).
func (s *DocumentService) Recalc(ctx context.Context) (DocState, error) {
	st, err := s.act(ctx, func(d *doc.Document) error {
		d.Recalc()
		return nil
	})
	if err == nil && s.listening(api.EventRecalc) {
		s.fireAsync(api.EventRecalc, map[string]any{})
	}
	return st, err
}

// RemoveDuplicates supprime les lignes en double d'un tableau ; renvoie leur nombre.
func (s *DocumentService) RemoveDuplicates(ctx context.Context, table string, cols []int) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	n, err := d.RemoveDuplicates(ctx, table, cols)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return n, err
}

// TextToColumnsPreview donne l'aperçu de « Convertir » (texte en colonnes)
// sur les lignes r0 à r1 de la colonne col.
func (s *DocumentService) TextToColumnsPreview(ctx context.Context, sheet int64, r0, r1, col int, o canvas.TextColumnsOptions) (canvas.TextColumnsPreview, error) {
	d, err := s.current()
	if err != nil {
		return canvas.TextColumnsPreview{}, err
	}
	return d.TextToColumnsPreview(ctx, sheet, r0, r1, col, o)
}

// TextToColumns convertit le texte des lignes r0 à r1 de la colonne col en
// colonnes ; renvoie le nombre de colonnes importées.
func (s *DocumentService) TextToColumns(ctx context.Context, sheet int64, r0, r1, col int, o canvas.TextColumnsOptions) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	n, err := d.TextToColumns(ctx, sheet, r0, r1, col, o)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return n, err
}

// CreatePivot crée un tableau croisé sur une nouvelle feuille ; renvoie la feuille.
func (s *DocumentService) CreatePivot(ctx context.Context, cfg canvas.PivotConfig) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	sheet, err := d.CreatePivot(ctx, cfg)
	if err != nil {
		return 0, err
	}
	_, err = s.changed(ctx)
	return sheet, err
}

// SetPivot change la configuration d'un tableau croisé.
func (s *DocumentService) SetPivot(ctx context.Context, id int64, cfg canvas.PivotConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetPivot(ctx, id, cfg) })
}

// SetPivotCalcField ajoute (old vide), modifie ou retire (formula vide) un
// champ calculé d'un tableau croisé (« Champ calculé… ») ; annulable.
func (s *DocumentService) SetPivotCalcField(ctx context.Context, id int64, old, name, formula string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetPivotCalcField(ctx, id, old, name, formula) })
}

// PreviewPivotFormula donne l'aperçu d'un champ calculé sur le tableau
// source (premières valeurs, ou l'erreur de la formule).
func (s *DocumentService) PreviewPivotFormula(ctx context.Context, source, formula string) (canvas.ChoicePreview, error) {
	d, err := s.current()
	if err != nil {
		return canvas.ChoicePreview{}, err
	}
	return d.PreviewPivotFormula(ctx, source, formula)
}

// DeletePivot supprime un tableau croisé.
func (s *DocumentService) DeletePivot(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeletePivot(ctx, id) })
}

// CreateChart pose un graphique sur une feuille.
func (s *DocumentService) CreateChart(ctx context.Context, sheet int64, row, col int, cfg canvas.ChartConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.CreateChart(ctx, sheet, row, col, cfg)
		return err
	})
}

// SetChart change la configuration d'un graphique.
func (s *DocumentService) SetChart(ctx context.Context, id int64, cfg canvas.ChartConfig) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetChart(ctx, id, cfg) })
}

// MoveChart déplace un graphique.
func (s *DocumentService) MoveChart(ctx context.Context, id int64, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.MoveChart(ctx, id, row, col) })
}

// DeleteChart supprime un graphique.
func (s *DocumentService) DeleteChart(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteChart(ctx, id) })
}

// KeyCheck dit si une colonne peut devenir la clé du tableau (vide : oui).
func (s *DocumentService) KeyCheck(ctx context.Context, table string, i int) (string, error) {
	return remember(ctx, s, "key", table, i, func(d *doc.Document) (string, error) { return d.KeyCheck(ctx, table, i) })
}

// SetKey fait d'une colonne la clé du tableau.
func (s *DocumentService) SetKey(ctx context.Context, table string, i int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetKey(ctx, table, i) })
}

// ClearKey retire la clé d'un tableau.
func (s *DocumentService) ClearKey(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ClearKey(ctx, table) })
}

// SetHandKey change la clé de jointure des colonnes saisies à côté d'un
// tableau chargé (column : colonne chargée ; "#" : rang des lignes). Les
// valeurs mises de côté sont oubliées (annulable).
func (s *DocumentService) SetHandKey(ctx context.Context, table, column string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.SetHandKey(ctx, table, column)
		return err
	})
}

// ForgetHandKept oublie les valeurs saisies mises de côté (lignes disparues
// du résultat) d'un tableau chargé.
func (s *DocumentService) ForgetHandKept(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error {
		_, err := d.ForgetHandKept(ctx, table)
		return err
	})
}

// DismissKeyHint ne propose plus la première colonne du tableau comme clé.
func (s *DocumentService) DismissKeyHint(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DismissKeyHint(ctx, table) })
}

// DuplicatesPreview compte les lignes en double selon des colonnes.
func (s *DocumentService) DuplicatesPreview(ctx context.Context, table string, cols []int) (int, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	return d.DuplicatesPreview(ctx, table, cols)
}

// FieldValues donne les valeurs distinctes d'une colonne désignée par son nom.
func (s *DocumentService) FieldValues(ctx context.Context, table, field string) ([]canvas.FilterValue, error) {
	d, err := s.current()
	if err != nil {
		return nil, err
	}
	return d.FieldValues(ctx, table, field)
}

// ValidationList lit la liste d'une validation tirée d'une plage (src :
// Tableau[Colonne], Cell[A1:A10], Feuille2!Cell[A1:A10] ou une cellule
// nommée), pour une cellule de la feuille sheet : valeurs distinctes dans
// l'ordre de la source, ou l'erreur qui dit pourquoi elle ne se lit pas.
func (s *DocumentService) ValidationList(ctx context.Context, sheet int64, src string) (canvas.ValidationList, error) {
	d, err := s.current()
	if err != nil {
		return canvas.ValidationList{}, err
	}
	return d.ValidationList(ctx, sheet, src)
}

// PromoteHeader prend la première ligne d'un tableau comme en-têtes.
func (s *DocumentService) PromoteHeader(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.PromoteHeader(ctx, table) })
}

// HeaderToTitle sort l'en-tête d'un tableau (c'est un titre) : la 1re ligne
// de données devient l'en-tête.
func (s *DocumentService) HeaderToTitle(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.HeaderToTitle(ctx, table) })
}

// TitleToHeader : « Utiliser comme en-têtes » ; la ligne juste au-dessus du
// tableau donne les noms des colonnes, l'ancien en-tête devient la 1re ligne.
func (s *DocumentService) TitleToHeader(ctx context.Context, table string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.TitleToHeader(ctx, table) })
}

// SetTwoLevel affiche (ou non) les en-têtes d'un tableau sur deux lignes
// (groupe « Groupe.Sous » dans une bande au-dessus de l'en-tête).
func (s *DocumentService) SetTwoLevel(ctx context.Context, table string, on bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetTwoLevel(ctx, table, on) })
}

// RowToNote sort la dernière ligne d'un tableau : elle reste dessous, hors
// du tableau (note ou total).
func (s *DocumentService) RowToNote(ctx context.Context, table string, n int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RowToNote(ctx, table, n) })
}

// JoinTable ajoute au tableau voisin une cellule libre gardée hors du
// tableau (note, total, remarque).
func (s *DocumentService) JoinTable(ctx context.Context, sheet int64, row, col int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.JoinTable(ctx, sheet, row, col) })
}

// PlaceChart déplace et redimensionne un graphique (ancrage libre).
func (s *DocumentService) PlaceChart(ctx context.Context, id int64, row, col, offX, offY, width, height int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.PlaceChart(ctx, id, row, col, offX, offY, width, height) })
}

// Dependencies donne les antécédents et les dépendants d'une cellule.
func (s *DocumentService) Dependencies(ctx context.Context, sheet int64, row, col int) (canvas.Dependencies, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Dependencies{}, err
	}
	return d.Dependencies(ctx, sheet, row, col)
}

// SetHeadings affiche toujours, ou seulement pendant la saisie d'une
// formule, les lettres de colonnes et numéros de lignes de la feuille.
func (s *DocumentService) SetHeadings(ctx context.Context, sheet int64, show bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetHeadings(ctx, sheet, show) })
}

// SetSheetRules applique des changements de mise en forme conditionnelle à
// une feuille (colonnes de tableaux, lignes ou colonnes entières, cellules
// libres) en une seule étape d'annulation.
func (s *DocumentService) SetSheetRules(ctx context.Context, sheet int64, ch canvas.RuleChanges) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetSheetRules(ctx, sheet, ch) })
}

// SetLineRules fixe la mise en forme conditionnelle de lignes (rows) ou de
// colonnes entières i0 à i1 de la feuille (aucune règle : la retire).
func (s *DocumentService) SetLineRules(ctx context.Context, sheet int64, rows bool, i0, i1 int, rules []canvas.Rule) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetLineRules(ctx, sheet, rows, i0, i1, rules) })
}

// SetGridlines affiche ou masque le quadrillage de la feuille.
func (s *DocumentService) SetGridlines(ctx context.Context, sheet int64, show bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetGridlines(ctx, sheet, show) })
}

// SetPageSetup remplace la mise en page d'une feuille (onglet Mise en
// page) ; label nomme l'étape d'annulation (vide : « Mise en page »).
func (s *DocumentService) SetPageSetup(ctx context.Context, sheet int64, p canvas.PageSetup, label string) (DocState, error) {
	if label == "" {
		label = "Mise en page"
	}
	return s.act(ctx, func(d *doc.Document) error { return d.SetPageSetupAs(ctx, sheet, p, label) })
}

// SetRowHeight fixe la hauteur d'une ligne (0 : hauteur par défaut).
func (s *DocumentService) SetRowHeight(ctx context.Context, sheet int64, row, height int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SetRowHeight(ctx, sheet, row, height) })
}

// Favorites liste les requêtes SQL favorites du classeur.
func (s *DocumentService) Favorites(ctx context.Context) ([]canvas.Favorite, error) {
	d, err := s.current()
	if err != nil {
		return []canvas.Favorite{}, nil
	}
	return d.Favorites(ctx)
}

// AddFavorite ajoute une requête SQL aux favoris (nom vide : sa première
// ligne) et renvoie son identifiant.
func (s *DocumentService) AddFavorite(ctx context.Context, name, sql string) (int64, error) {
	d, err := s.current()
	if err != nil {
		return 0, err
	}
	id, err := d.AddFavorite(ctx, name, sql)
	if err == nil {
		s.changed(ctx)
	}
	return id, err
}

// RenameFavorite renomme une requête favorite.
func (s *DocumentService) RenameFavorite(ctx context.Context, id int64, name string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.RenameFavorite(ctx, id, name) })
}

// DeleteFavorite retire une requête des favoris.
func (s *DocumentService) DeleteFavorite(ctx context.Context, id int64) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteFavorite(ctx, id) })
}

// ProtectSheet protège une feuille (password vide : sans mot de passe) ;
// hors de l'historique d'annulation, comme Excel.
func (s *DocumentService) ProtectSheet(ctx context.Context, sheet int64, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ProtectSheet(ctx, sheet, password) })
}

// UnprotectSheet ôte la protection d'une feuille.
func (s *DocumentService) UnprotectSheet(ctx context.Context, sheet int64, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.UnprotectSheet(ctx, sheet, password) })
}

// ProtectStructure protège la structure d'une feuille (seule la saisie
// dans ses tableaux reste permise) ; hors de l'historique d'annulation.
func (s *DocumentService) ProtectStructure(ctx context.Context, sheet int64, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ProtectStructure(ctx, sheet, password) })
}

// UnprotectStructure ôte la protection de la structure d'une feuille.
func (s *DocumentService) UnprotectStructure(ctx context.Context, sheet int64, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.UnprotectStructure(ctx, sheet, password) })
}

// ProtectWorkbook protège la structure du classeur (onglets) ; sheets
// verrouille aussi le contenu de toutes les feuilles.
func (s *DocumentService) ProtectWorkbook(ctx context.Context, password string, sheets, structure bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.ProtectWorkbook(ctx, password, sheets, structure) })
}

// UnprotectWorkbook ôte la protection de la structure du classeur.
func (s *DocumentService) UnprotectWorkbook(ctx context.Context, password string) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.UnprotectWorkbook(ctx, password) })
}

// ClearCells vide entièrement les cellules libres d'une plage (valeurs,
// mise en forme, fusions, notes, validations).
func (s *DocumentService) ClearCells(ctx context.Context, sheet int64, r0, c0, r1, c1 int) (DocState, error) {
	w := s.watchChange(sheet, r0, c0, r1, c1, "clear")
	st, err := s.act(ctx, func(d *doc.Document) error { return d.ClearCells(ctx, sheet, r0, c0, r1, c1) })
	if err == nil {
		s.fireChange(w)
	}
	return st, err
}

// ClearRange efface une zone en une seule étape d'annulation (tableaux
// entièrement compris supprimés ; all : mise en forme des cellules libres
// comprise, comme Supprimer).
func (s *DocumentService) ClearRange(ctx context.Context, sheet int64, r0, c0, r1, c1 int, all bool) (DocState, error) {
	w := s.watchChange(sheet, r0, c0, r1, c1, "clear")
	st, err := s.act(ctx, func(d *doc.Document) error { return d.ClearRange(ctx, sheet, r0, c0, r1, c1, all) })
	if err == nil {
		s.fireChange(w)
	}
	return st, err
}

// Transfer copie ou déplace (move) une zone d'une feuille vers une autre
// zone, d'une feuille à l'autre : copier-coller et couper-coller internes,
// glisser de la bordure de la sélection (voir canvas.Model.Transfer).
func (s *DocumentService) Transfer(ctx context.Context, from int64, src canvas.Rect, to int64, dst canvas.Rect, move bool, values [][]string) (canvas.Event, error) {
	d, err := s.current()
	if err != nil {
		return canvas.Event{}, err
	}
	source := "copy"
	if move {
		source = "move"
	}
	w := s.watchChange(to, dst.R0, dst.C0, dst.R1, dst.C1, source)
	ev, err := d.Transfer(ctx, from, src, to, dst, move, values)
	if err != nil {
		return ev, err
	}
	_, err = s.changed(ctx)
	s.fireChange(w)
	return ev, err
}

// PinNote épingle la note d'une cellule (toujours affichée) ou la remet en infobulle.
func (s *DocumentService) PinNote(ctx context.Context, sheet int64, row, col int, pinned bool) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.PinNote(ctx, sheet, row, col, pinned) })
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

// DeleteSheets supprime plusieurs onglets (sélection de plusieurs onglets)
// en une seule étape d'annulation ; une feuille au moins doit rester.
func (s *DocumentService) DeleteSheets(ctx context.Context, sheets []int64) (DocState, error) {
	var events []map[string]any
	if s.listening(api.EventSheetDelete) {
		for _, id := range sheets {
			events = append(events, api.SheetEventData(host{s}, id))
		}
	}
	gctx := command.WithGroup(ctx, "Supprimer les feuilles")
	st, err := s.act(ctx, func(d *doc.Document) error {
		for _, id := range sheets {
			if err := d.DeleteSheet(gctx, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		for _, e := range events {
			s.fireAsync(api.EventSheetDelete, e)
		}
	}
	return st, err
}

// OrderSheets range les onglets dans l'ordre order (toutes les feuilles,
// masquées comprises), en une seule étape d'annulation : glisser plusieurs
// onglets sélectionnés.
func (s *DocumentService) OrderSheets(ctx context.Context, order []int64) (DocState, error) {
	gctx := command.WithGroup(ctx, "Déplacer les feuilles")
	return s.act(ctx, func(d *doc.Document) error {
		for i, id := range order {
			if err := d.MoveSheet(gctx, id, i); err != nil {
				return err
			}
		}
		return nil
	})
}

// SuppressColumns : Suppr sur les colonnes entières i à j d'un tableau, en
// une étape (vides au milieu : le tableau se coupe en deux).
func (s *DocumentService) SuppressColumns(ctx context.Context, table string, i, j int) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.SuppressColumns(ctx, table, i, j) })
}

// DeleteColumnSpans supprime des colonnes de plusieurs tableaux (colonnes
// entières de la feuille qui les traversent), en une étape.
func (s *DocumentService) DeleteColumnSpans(ctx context.Context, spans []canvas.ColumnSpan) (DocState, error) {
	return s.act(ctx, func(d *doc.Document) error { return d.DeleteColumnSpans(ctx, spans) })
}
