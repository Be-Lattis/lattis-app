// Package doc assemble un document Lattis ouvert : l'archive et son
// répertoire de travail (internal/archive), la base migrée (internal/meta),
// le bus de commandes (internal/command) et le modèle des feuilles
// (internal/canvas).
package doc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/store"
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
	d := &Document{Model: canvas.New(a.DB, bus), Bus: bus, Share: share, Oplog: rec, arc: a, id: id}
	d.Model.SetSharing(share)
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
func New(ctx context.Context) (*Document, error) {
	a, err := archive.New(canvas.Register)
	if err != nil {
		return nil, err
	}
	d, err := wrap(ctx, a, false)
	if err != nil {
		return nil, err
	}
	return d, nil // vide : rien à perdre tant qu'on n'a rien saisi
}

// Open ouvre un classeur. Si une session précédente de ce fichier a été
// interrompue, l'erreur est un *archive.RecoverableError : proposer Recover
// ou archive.Discard. Un fichier illisible donne un *archive.OpenError (s'il
// est endommagé : proposer Salvage). Le fichier n'est jamais modifié.
func Open(ctx context.Context, path string) (*Document, error) {
	a, err := archive.Open(path, canvas.Register)
	if err != nil {
		return nil, err
	}
	d, err := wrap(ctx, a, false)
	if err != nil {
		var tooNew meta.ErrTooNew
		switch {
		case errors.As(err, &tooNew):
			return nil, &archive.OpenError{Path: path, Reason: err.Error(), Err: archive.ErrFormat}
		case errors.Is(err, meta.ErrForeign):
			return nil, &archive.OpenError{Path: path, Reason: err.Error(), Err: archive.ErrFormat}
		}
		return nil, archive.Damaged(path, err)
	}
	d.claimLegacy(ctx)
	return d, nil
}

// Recover rouvre le répertoire de travail d'une session interrompue, avec
// toutes ses modifications validées et son historique d'annulation. En cas
// d'échec, la session reste proposée.
func Recover(ctx context.Context, o archive.Orphan) (*Document, error) {
	a, err := archive.Recover(o, canvas.Register)
	if err != nil {
		return nil, err
	}
	d, err := wrap(ctx, a, true)
	if err != nil {
		return nil, fmt.Errorf("récupération impossible : %w", err)
	}
	d.saved = -1 // les modifications récupérées ne sont pas enregistrées
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
func (d *Document) Save(ctx context.Context, path string) error {
	if path == "" && d.arc.Path == "" {
		return errors.New("choisir un fichier pour enregistrer")
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
	if (path == "" || path == d.arc.Path) && d.arc.Path != "" && top == d.saved && prot == d.savedProt && share == d.savedShare.Load() {
		return nil
	}
	// Premier enregistrement d'un classeur migré par-dessus son fichier : la
	// version d'origine est d'abord copiée à côté (une version plus ancienne
	// de Lattis ne pourra plus lire le fichier réécrit).
	if d.backup != "" && (path == "" || samePath(path, d.arc.Path)) {
		if err := backupFile(d.arc.Path, d.backup); err != nil {
			return fmt.Errorf("copie de sauvegarde de l'ancien format impossible, rien n'a été enregistré : %w", err)
		}
		d.backup = ""
	}
	if err := d.arc.Save(ctx, path); err != nil {
		return err
	}
	d.saved, d.savedProt = top, prot
	d.savedShare.Store(share)
	d.backup = "" // enregistré ailleurs : le fichier d'origine reste tel quel
	return nil
}

func samePath(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	return strings.EqualFold(a, b)
}

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
	return d.Share.Reload(ctx)
}

// TakeOpenNotice rend (une fois) le message à afficher à l'ouverture :
// déclencheurs inconnus supprimés du classeur (meta.MigrateOpen), vide
// sinon.
func (d *Document) TakeOpenNotice() string {
	n := len(d.dropped)
	if n == 0 {
		return ""
	}
	names := strings.Join(d.dropped, ", ")
	d.dropped = nil
	if n == 1 {
		return fmt.Sprintf("1 déclencheur inconnu supprimé du classeur (sécurité) : %s. Enregistrez le classeur pour le retirer aussi du fichier.", names)
	}
	return fmt.Sprintf("%d déclencheurs inconnus supprimés du classeur (sécurité) : %s. Enregistrez le classeur pour les retirer aussi du fichier.", n, names)
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
	}
	return err
}

// Close ferme le classeur (les modifications non enregistrées sont perdues).
func (d *Document) Close() error { return d.arc.Close() }
