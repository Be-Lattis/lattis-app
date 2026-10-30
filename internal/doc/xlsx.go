package doc

import (
	"context"
	"database/sql"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/trace"
	"github.com/lalamefine/lattis/internal/transfer"
)

// Classeur adossé à un fichier .xlsx : ouvert depuis le fichier (import dans
// une copie de travail), il s'enregistre dans le même fichier par l'export
// .xlsx (archive.Doc.Export), tant que l'utilisateur ne le convertit pas en
// .lattis (Enregistrer sous).

// XLSX dit si le classeur est adossé à un fichier .xlsx.
func (d *Document) XLSX() bool { return archive.Foreign(d.arc.Path) }

// exportHook branche l'export .xlsx sur l'enregistrement (fichier .xlsx
// ouvert, ou Enregistrer sous en .xlsx).
func (d *Document) exportHook() {
	d.arc.Export = func(ctx context.Context, w io.Writer) error {
		_, err := transfer.ExportXLSX(ctx, d.Model, w)
		return err
	}
}

// OpenXLSX ouvre un fichier .xlsx en gardant son format : son contenu est
// importé dans une copie de travail (sans étape d'annulation), dans la
// langue lang (noms et messages). Une session précédente interrompue donne
// un *archive.RecoverableError, comme Open ; le fichier n'est jamais modifié.
func OpenXLSX(ctx context.Context, path string, mode OpenMode, lang msg.Lang) (_ *Document, err error) {
	defer trace.Since(1, trace.Doc, time.Now(), &err, "ouverture de %s (.xlsx)", path)
	a, err := archive.OpenForeign(path, canvas.Register)
	if err != nil {
		return nil, err
	}
	d, err := wrap(ctx, a, false)
	if err != nil {
		return nil, err
	}
	d.SetLanguage(lang)
	res, err := transfer.OpenXLSX(ctx, d.Model, path)
	if err == nil {
		err = d.forgetHistory(ctx)
	}
	if err == nil {
		d.saved, err = d.top(ctx)
	}
	if err != nil {
		d.Close()
		return nil, err
	}
	if n := len(res.Warnings); n > 0 {
		d.notes = append(d.notes, msg.M("doc.xlsxOpenWarnings", msg.Args{"n": n, "file": filepath.Base(path), "first": res.Warnings[0]}))
	}
	if err := a.MarkPristine(ctx); err != nil {
		d.Close()
		return nil, err
	}
	if mode == OpenReadOnly {
		a.ReadOnly = true
	}
	return d, nil
}

// forgetHistory oublie l'historique d'annulation (l'import du fichier
// ouvert n'est pas une action à annuler) et ses corbeilles.
func (d *Document) forgetHistory(ctx context.Context) error {
	return d.arc.DB.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM _lattis_journal`); err != nil {
			return err
		}
		rs, err := tx.QueryContext(ctx, `SELECT name FROM sqlite_schema WHERE type = 'table' AND (name LIKE '\_lattis\_trash%' ESCAPE '\' OR name LIKE '\_lattis\_sqltrash\_%' ESCAPE '\')`)
		if err != nil {
			return err
		}
		var trash []string
		for rs.Next() {
			var n string
			if err := rs.Scan(&n); err != nil {
				rs.Close()
				return err
			}
			trash = append(trash, n)
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return err
		}
		for _, n := range trash {
			if _, err := tx.ExecContext(ctx, `DROP TABLE "`+strings.ReplaceAll(n, `"`, `""`)+`"`); err != nil {
				return err
			}
		}
		return nil
	})
}
