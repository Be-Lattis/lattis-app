// Package doc assemble un document Lattis ouvert : l'archive et son
// répertoire de travail (internal/archive), la base migrée (internal/meta),
// le bus de commandes (internal/command) et le modèle des feuilles
// (internal/canvas).
package doc

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/store"
	"github.com/lalamefine/lattis/internal/trace"
)

// Document est un classeur ouvert.
type Document struct {
	*canvas.Model
	Bus *command.Bus
	// Share : partage sur un Serveur Lattis (classeur local si
	// Share.Info() est nil).
	Share *workbook.Link
	// Oplog relève les opérations des tableaux publiés (file d'envoi) ;
	// TakeSkipped après une annulation dans un classeur collaboratif.
	Oplog *oplog.Recorder
	arc   *archive.Doc
	// Changements de partage (publication, rechargement : hors journal)
	// depuis l'ouverture, et au moment de l'enregistrement.
	shareGen, savedShare atomic.Int64
	saved                int64 // dernière commande appliquée au moment de l'enregistrement
	// Changements de protection (hors journal) au moment de l'enregistrement.
	savedProt int64
	// id est l'identifiant aléatoire du classeur (_lattis_info, SecretScope).
	id string
	// backup : copie de sauvegarde à faire du fichier ouvert avant de
	// l'écraser, s'il était d'un format plus ancien (vide sinon).
	backup string
	// dropped : déclencheurs inconnus supprimés à l'ouverture (sécurité),
	// signalés une fois (TakeOpenNotice).
	dropped []string
	// notes : autres messages de l'ouverture (verrou, fichier modifié par
	// ailleurs), signalés une fois.
	notes []msg.Message
	// indexer : index des tableaux partagés sur le serveur (SetIndexer).
	indexer atomic.Pointer[Indexer]
	// coedit : travail à plusieurs sur un dossier partagé (SetCoEdit).
	coedit atomic.Pointer[CoEditHooks]
}

// Indexer pose et retire sur le serveur les index d'un tableau partagé
// d'un classeur collaboratif (collab.Engine) : ils n'existent que sur le
// serveur, hors du journal d'annulation.
type Indexer interface {
	Indexes(ctx context.Context, tbl int64) ([]workbook.Index, error)
	CreateIndex(ctx context.Context, tbl int64, cols []int64, unique bool) (string, error)
	DropIndex(ctx context.Context, tbl int64, name string) error
}

// ErrNoIndexer : pas de classeur collaboratif suivi.
var ErrNoIndexer = msg.New("doc.noIndexer", nil)

// SetIndexer branche les index du serveur (nil : aucun).
func (d *Document) SetIndexer(x Indexer) {
	if x == nil {
		d.indexer.Store(nil)
		return
	}
	d.indexer.Store(&x)
}

// Indexer rend les index du serveur, ou ErrNoIndexer.
func (d *Document) Indexer() (Indexer, error) {
	if p := d.indexer.Load(); p != nil {
		return *p, nil
	}
	return nil, ErrNoIndexer
}

// wrap migre la base et assemble le document. En cas d'échec, keep garde le
// répertoire de travail (récupération : rien ne doit être perdu).
func wrap(ctx context.Context, a *archive.Doc, keep bool) (*Document, error) {
	fail := func(err error) (*Document, error) {
		if keep {
			a.Release()
		} else {
			a.Close()
		}
		return nil, err
	}
	opened, err := meta.MigrateOpen(ctx, a.DB)
	if err != nil {
		return fail(err)
	}
	bus := command.New(a.DB)
	// Tableaux publiés sur un Serveur Lattis : leurs écritures vont dans la
	// file d'envoi (rien à faire tant qu'aucun ne l'est), ou sont refusées
	// quand ils sont en lecture seule (classeur figé, hors ligne).
	share, err := workbook.Attach(ctx, a.DB)
	if err != nil {
		return fail(err)
	}
	rec := oplog.New(a.DB)
	rec.Gate = share.Gate
	bus.Recorder = rec
	a.Remote = func(ctx context.Context) (*archive.Remote, error) { return workbook.Manifest(ctx, a.DB.Read()) }
	var id string
	err = a.DB.Read().QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = 'id'`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) { // perdu (classeur récupéré) : un nouveau
		err = a.DB.Write(ctx, func(tx *sql.Tx) error {
			return tx.QueryRowContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES ('id', lower(hex(randomblob(16)))) RETURNING value`).Scan(&id)
		})
	}
	if err != nil {
		return fail(err)
	}
	// Sessions regroupées par classeur à la récupération (archive.Orphan).
	a.SetWorkbook(id)
	d := &Document{Model: canvas.New(a.DB, bus), Bus: bus, Share: share, Oplog: rec, arc: a, id: id}
	d.Model.SetSharing(share)
	// Onglets à choix : la dernière valeur choisie, sauf si elle n'a plus
	// rien à montrer (décision 12.2, canvas/choice.go). Sans incidence sur
	// le reste : un échec laisse les choix tels quels.
	_ = d.Model.SettleChoices(ctx)
	top, err := d.top(ctx)
	if err != nil {
		return fail(err)
	}
	d.saved = top
	if from := opened.From; from > 0 && from < meta.Version && a.Path != "" {
		d.backup = backupName(a.Path, from)
	}
	// Déclencheurs écrits hors de Lattis, supprimés de la copie de travail
	// (meta.MigrateOpen) : le classeur est à enregistrer pour qu'ils
	// disparaissent aussi du fichier, qui ne change pas d'ici là.
	if len(opened.Dropped) > 0 {
		d.saved = -1
		d.dropped = opened.Dropped
	}
	return d, nil
}

// backupName : « Ventes.lattis » au format 12 → « Ventes (format 12).lattis »,
// un classeur que la version de Lattis qui l'a écrit sait encore ouvrir.
func backupName(path string, format int) string {
	ext := filepath.Ext(path)
	return fmt.Sprintf("%s (format %d)%s", strings.TrimSuffix(path, ext), format, ext)
}

// New crée un classeur vide, jamais enregistré.
func New(ctx context.Context) (_ *Document, err error) {
	defer trace.Since(1, trace.Doc, time.Now(), &err, "nouveau classeur")
	a, err := archive.New(canvas.Register)
	if err != nil {
		return nil, err
	}
	d, err := wrap(ctx, a, false)
	if err != nil {
		return nil, err
	}
	// Vide : rien à perdre tant qu'on n'a rien saisi, rien à proposer en
	// récupération si le processus s'arrête d'ici là.
	if err := a.MarkPristine(ctx); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// Open ouvre un classeur. Si une session précédente de ce fichier a été
// interrompue, l'erreur est un *archive.RecoverableError : proposer Recover
// ou archive.Discard. Un fichier illisible donne un *archive.OpenError (s'il
// est endommagé : proposer Salvage). Le fichier n'est jamais modifié.
func Open(ctx context.Context, path string) (*Document, error) {
	return OpenAs(ctx, path, OpenNormal)
}

// OpenMode dit comment ouvrir un classeur verrouillé par un autre poste.
type OpenMode int

const (
	// OpenNormal pose le verrou ~$ ; déjà pris, le classeur s'ouvre en
	// lecture seule.
	OpenNormal OpenMode = iota
	// OpenReadOnly : lecture seule, sans verrou (Enregistrer sous seulement).
	OpenReadOnly
	// OpenAnyway : modifiable sans prendre le verrou d'un autre ;
	// l'enregistrement contrôle encore que le fichier n'a pas changé.
	OpenAnyway
	// OpenCoEdit : travail à plusieurs sur un dossier partagé
	// (internal/coedit) ; modifiable, le verrou ~$ est pris s'il est libre
	// (sinon une autre session le tient).
	OpenCoEdit
)

// OpenAs ouvre un classeur comme Open, selon mode.
func OpenAs(ctx context.Context, path string, mode OpenMode) (_ *Document, err error) {
	defer trace.Since(1, trace.Doc, time.Now(), &err, "ouverture de %s", path)
	a, err := archive.Open(path, canvas.Register)
	if err != nil {
		return nil, err
	}
	d, err := wrap(ctx, a, false)
	if err != nil {
		var tooNew meta.ErrTooNew
		switch {
		case errors.As(err, &tooNew):
			return nil, &archive.OpenError{Path: path, Reason: err.Error(), Why: tooNew.Msg(), Err: archive.ErrFormat}
		case errors.Is(err, meta.ErrForeign):
			return nil, &archive.OpenError{Path: path, Reason: err.Error(), Why: meta.ErrForeign.Msg(), Err: archive.ErrFormat}
		}
		return nil, archive.Damaged(path, err)
	}
	d.claimLegacy(ctx)
	// Copie de travail du fichier, sans modification : rien à proposer en
	// récupération si le processus s'arrête avant la première.
	if err := a.MarkPristine(ctx); err != nil {
		d.Close()
		return nil, err
	}
	switch mode {
	case OpenNormal:
		d.lockFile()
	case OpenReadOnly:
		a.ReadOnly = true
	case OpenCoEdit:
		d.TryLock()
	}
	return d, nil
}

// TryLock pose le verrou ~$ du fichier s'il est libre (travail à
// plusieurs : la session qui le tenait est partie) ; sans effet s'il est
// déjà posé par ce document ou tenu par un autre.
func (d *Document) TryLock() {
	if d.arc.Path == "" || d.arc.ReadOnly {
		return
	}
	if err := d.arc.Lock(); err != nil {
		var held *archive.LockedError
		if !errors.As(err, &held) {
			trace.Logf(2, trace.Doc, "verrou de %s : %v", d.arc.Path, err)
		}
	}
}

// CoEditHooks branche le travail à plusieurs sur un dossier partagé
// (internal/coedit) sur l'enregistrement.
type CoEditHooks struct {
	// Before est appelé avant l'enregistrement dans le fichier courant
	// (la file locale part dans le journal).
	Before func(ctx context.Context) error
	// Positions : positions des journaux contenues dans la base, prises
	// avec l'instantané (archive.Doc.Journal).
	Positions func(ctx context.Context, db *sql.DB) (map[string]int64, error)
	// Known : le fichier modifié par ailleurs ne contient que ce qui est
	// déjà appliqué ici (archive.Doc.Known).
	Known func(m archive.Manifest) bool
	// After est appelé après un enregistrement réussi (path : fichier
	// écrit ; same : le fichier courant).
	After func(ctx context.Context, path string, same bool)
}

// SetCoEdit branche (nil : débranche) le travail à plusieurs.
func (d *Document) SetCoEdit(h *CoEditHooks) {
	d.coedit.Store(h)
	if h == nil {
		d.arc.Journal, d.arc.Known = nil, nil
		return
	}
	d.arc.Journal, d.arc.Known = h.Positions, h.Known
}

// BaseSum donne l'empreinte du fichier tel qu'ouvert ou enregistré en
// dernier (vide : inconnue).
func (d *Document) BaseSum() string { return d.arc.BaseSum() }

// lockFile pose le verrou ~$ du fichier : pris par un autre (entre la
// question et l'ouverture), le classeur passe en lecture seule ; impossible
// à poser (dossier en lecture seule), on le signale une fois.
func (d *Document) lockFile() {
	err := d.arc.Lock()
	var held *archive.LockedError
	switch {
	case errors.As(err, &held):
		d.arc.ReadOnly = true
		d.notes = append(d.notes, msg.M("doc.openedReadOnly", msg.Args{"locked": held.Msg()}))
	case err != nil:
		trace.Logf(1, trace.Doc, "verrou de %s : %v", d.arc.Path, err)
		d.notes = append(d.notes, msg.M("doc.lockFailed", msg.Args{"file": filepath.Base(d.arc.Path)}))
	}
}

// ReadOnly dit si le classeur est ouvert en lecture seule (verrouillé par
// un autre poste) : il ne s'enregistre que sous un autre nom.
func (d *Document) ReadOnly() bool { return d.arc.ReadOnly }

// Unlock retire le verrou ~$ sans fermer le classeur (arrêt de
// l'application en gardant la session pour la récupération).
func (d *Document) Unlock() { d.arc.Unlock() }

// Recover rouvre le répertoire de travail d'une session interrompue, avec
// toutes ses modifications validées et son historique d'annulation. En cas
// d'échec, la session reste proposée.
func Recover(ctx context.Context, o archive.Orphan) (_ *Document, err error) {
	defer trace.Since(1, trace.Doc, time.Now(), &err, "récupération de la session %s (%s)", o.Dir, o.Source)
	a, err := archive.Recover(o, canvas.Register)
	if err != nil {
		return nil, err
	}
	d, err := wrap(ctx, a, true)
	if err != nil {
		return nil, msg.Wrap(err, "doc.recoverFailed", msg.Args{"err": err})
	}
	d.saved = -1 // les modifications récupérées ne sont pas enregistrées
	// Même contrôle qu'à l'ouverture : verrou d'un autre poste, fichier
	// modifié par ailleurs depuis le début de la session.
	if _, err := os.Stat(o.Source); o.Source != "" && err == nil {
		d.lockFile()
		if changed, _ := a.Changed(); changed {
			d.notes = append(d.notes, msg.M("doc.changedSinceSession", msg.Args{"file": filepath.Base(o.Source)}))
		}
	}
	return d, nil
}

// Salvage tire d'un classeur endommagé ses données lisibles, dans un
// nouveau classeur jamais enregistré (voir archive.Salvage).
func Salvage(ctx context.Context, path string) (*Document, archive.Report, error) {
	schema := func(db string) error {
		s, err := store.Open(db)
		if err != nil {
			return err
		}
		defer s.Close()
		return meta.Migrate(ctx, s)
	}
	a, rep, err := archive.Salvage(path, schema, canvas.Register)
	if err != nil {
		return nil, rep, err
	}
	d, err := wrap(ctx, a, false)
	if err != nil {
		return nil, rep, err
	}
	d.saved = -1
	return d, rep, nil
}

// DB donne la base du document.
func (d *Document) DB() *store.Store { return d.arc.DB }

// Path est le fichier .lattis (vide si jamais enregistré).
func (d *Document) Path() string { return d.arc.Path }

// SecretScope désigne le classeur pour l'accord d'utiliser les mots de
// passe enregistrés sur ce poste (query.Secrets.For) : son identifiant
// aléatoire, créé avec lui et gardé dans le fichier (un classeur reçu en a
// un autre, que l'utilisateur n'a jamais autorisé).
func (d *Document) SecretScope() string {
	return "classeur:" + d.id
}

// top donne le numéro de la dernière commande appliquée (0 si aucune).
func (d *Document) top(ctx context.Context) (int64, error) {
	var seq int64
	err := d.arc.DB.Read().QueryRowContext(ctx,
		`SELECT coalesce(max(seq), 0) FROM _lattis_journal WHERE undone = 0`).Scan(&seq)
	return seq, err
}

// Dirty indique si le classeur a changé depuis son dernier enregistrement
// (annuler jusqu'à l'état enregistré le rend de nouveau « propre »).
func (d *Document) Dirty(ctx context.Context) (bool, error) {
	top, err := d.top(ctx)
	return top != d.saved || d.ProtectionChanges() != d.savedProt || d.shareGen.Load() != d.savedShare.Load(), err
}

// Save enregistre dans path (vide : le fichier courant).
// Un fichier modifié par ailleurs depuis son ouverture n'est pas écrasé
// (*archive.ChangedError : proposer une copie ou Overwrite).
func (d *Document) Save(ctx context.Context, path string) error {
	return d.save(ctx, path, false)
}

// Overwrite enregistre dans le fichier courant même s'il a été modifié par
// ailleurs (choix de l'utilisateur).
func (d *Document) Overwrite(ctx context.Context) error {
	return d.save(ctx, "", true)
}

func (d *Document) save(ctx context.Context, path string, force bool) (err error) {
	defer trace.Since(1, trace.Doc, time.Now(), &err, "enregistrement dans %s", cmp.Or(path, d.arc.Path))
	if path == "" && d.arc.Path == "" {
		return msg.New("doc.chooseFile", nil)
	}
	top, err := d.top(ctx)
	if err != nil {
		return err
	}
	prot := d.ProtectionChanges()
	// Rien de changé depuis le dernier enregistrement dans ce fichier (le même
	// critère que la question à la fermeture) : le réécrire prendrait des
	// secondes sur un gros classeur et bloquerait les saisies.
	share := d.shareGen.Load()
	if (path == "" || samePath(path, d.arc.Path)) && d.arc.Path != "" && top == d.saved && prot == d.savedProt && share == d.savedShare.Load() {
		return nil
	}
	// Premier enregistrement d'un classeur migré par-dessus son fichier : la
	// version d'origine est d'abord copiée à côté (une version plus ancienne
	// de Lattis ne pourra plus lire le fichier réécrit).
	if d.backup != "" && (path == "" || samePath(path, d.arc.Path)) {
		if err := backupFile(d.arc.Path, d.backup); err != nil {
			return msg.Wrap(err, "doc.backupFailed", msg.Args{"err": err})
		}
		d.backup = ""
	}
	same := path == "" || d.arc.Path != "" && samePath(path, d.arc.Path)
	h := d.coedit.Load()
	if h != nil && h.Before != nil && same {
		if err := h.Before(ctx); err != nil {
			return err
		}
	}
	save := d.arc.Save
	if force {
		save = d.arc.SaveOver
	}
	if err := save(ctx, path); err != nil {
		return err
	}
	if h != nil && h.After != nil {
		h.After(ctx, d.arc.Path, same)
	}
	d.saved, d.savedProt = top, prot
	d.savedShare.Store(share)
	d.backup = "" // enregistré ailleurs : le fichier d'origine reste tel quel
	return nil
}

// samePath : même fichier (archive.SamePath : liens résolus, casse ignorée
// seulement là où le système l'ignore).
func samePath(a, b string) bool { return archive.SamePath(a, b) }

// backupFile copie src en dst, sans écraser une sauvegarde existante.
func backupFile(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if errors.Is(err, os.ErrNotExist) {
		return nil // fichier d'origine disparu : rien à garder
	}
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, dst)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// ShareChanged note un changement de partage fait hors du journal
// (publication, chargement depuis le serveur) : l'état de partage est relu
// et le classeur est à enregistrer.
func (d *Document) ShareChanged(ctx context.Context) error {
	d.shareGen.Add(1)
	// Onglets à choix reçus du serveur sans valeur choisie sur ce poste : la
	// dernière (remote/workbook/choice.go).
	_ = d.Model.SettleChoices(ctx)
	return d.Share.Reload(ctx)
}

// MarkChanged note un changement des informations du classeur fait hors
// du journal (_lattis_info : source du Data Lake liée à un tableau) : le
// classeur est à enregistrer, sans relire l'état de partage.
func (d *Document) MarkChanged() {
	d.shareGen.Add(1)
}

// TakeOpenNotice rend (une fois) le message à afficher à l'ouverture :
// déclencheurs inconnus supprimés du classeur (meta.MigrateOpen), vide
// sinon ; puis verrou d'un autre poste, fichier modifié par ailleurs.
func (d *Document) TakeOpenNotice() string { return d.TakeOpenNoticeIn(msg.FR) }

// TakeOpenNoticeIn : TakeOpenNotice dans la langue l.
func (d *Document) TakeOpenNoticeIn(l msg.Lang) string {
	notes := d.notes
	d.notes = nil
	var out []string
	if n := len(d.dropped); n > 0 {
		names := strings.Join(d.dropped, ", ")
		d.dropped = nil
		out = append(out, msg.Format(l, "doc.triggersDropped", msg.Args{"n": n, "names": names}))
	}
	for _, m := range notes {
		out = append(out, m.Text(l))
	}
	return strings.Join(out, " ")
}

// Backup donne la copie de sauvegarde que le prochain enregistrement fera
// du fichier d'origine, d'un format plus ancien (vide : aucune).
func (d *Document) Backup() string { return d.backup }

// MarkSaved considère l'état courant comme enregistré, sans écrire de
// fichier (classeur de démonstration : rien à proposer à la fermeture).
func (d *Document) MarkSaved(ctx context.Context) error {
	top, err := d.top(ctx)
	if err == nil {
		d.saved, d.savedProt = top, d.ProtectionChanges()
		d.savedShare.Store(d.shareGen.Load())
		// Rien à proposer en récupération tant qu'il n'est pas modifié.
		err = d.arc.MarkPristine(ctx)
	}
	return err
}

// SaveProgress donne l'avancement de l'enregistrement en cours (gros
// classeur : barre d'état).
func (d *Document) SaveProgress() archive.SaveProgress { return d.arc.Progress() }

// Pristine dit si le répertoire de travail est marqué « rien à récupérer »
// (aucune écriture depuis la création, l'ouverture ou l'enregistrement).
func (d *Document) Pristine() bool { return d.arc.Pristine() }

// Close ferme le classeur (les modifications non enregistrées sont perdues).
func (d *Document) Close() error {
	trace.Logf(1, trace.Doc, "fermeture de %s", cmp.Or(d.arc.Path, "Classeur sans titre"))
	return d.arc.Close()
}

// Release ferme le classeur en gardant son répertoire de travail : ses
// modifications non enregistrées sont proposées en récupération (Orphans).
func (d *Document) Release() error {
	trace.Logf(1, trace.Doc, "fermeture de %s, répertoire de travail gardé", cmp.Or(d.arc.Path, "Classeur sans titre"))
	return d.arc.Release()
}
