package bridge

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/transfer"
)

// Fichier .xlsx ouvert dans Lattis (Fichier › Ouvrir, « Ouvrir avec »,
// glisser-déposer) : le classeur garde le fichier et son format. Au premier
// Enregistrer de la session, l'interface demande s'il faut garder le format
// .xlsx ou convertir en .lattis (réglage files.xlsxSave : ask, keep,
// convert) ; garder le .xlsx dit d'abord ce que ce format ne tient pas
// (compte rendu de l'export, transfer.XLSXLosses).

// XLSXSaveSetting : réglage de l'enregistrement d'un fichier .xlsx ouvert.
const XLSXSaveSetting = "files.xlsxSave"

// XLSXQuestion est la question posée à l'enregistrement d'un classeur
// adossé à un .xlsx : Ask, garder le .xlsx ou convertir (sinon garder est
// déjà choisi, il reste à confirmer les pertes) ; Report : ce que le .xlsx
// ne garde pas tel quel.
type XLSXQuestion struct {
	File   string                `json:"file"`
	Ask    bool                  `json:"ask"`
	Report transfer.ExportReport `json:"report"`
}

// xlsxState : classeur pour lequel « garder le .xlsx » est confirmé dans
// cette session (plus de question jusqu'à sa fermeture).
type xlsxState struct {
	mu   sync.Mutex
	kept *doc.Document
}

func (s *DocumentService) xlsxKept(d *doc.Document) bool {
	s.xlsx.mu.Lock()
	defer s.xlsx.mu.Unlock()
	return s.xlsx.kept == d
}

func (s *DocumentService) keepXLSX(d *doc.Document) {
	s.xlsx.mu.Lock()
	s.xlsx.kept = d
	s.xlsx.mu.Unlock()
}

// Openable dit si path est un fichier que Lattis ouvre comme classeur :
// .lattis, ou .xlsx (gardé dans son format).
func Openable(path string) bool { return IsWorkbook(path) || archive.Foreign(path) }

// openDoc ouvre un classeur .lattis, ou un fichier .xlsx qui garde son format.
func (s *DocumentService) openDoc(ctx context.Context, path string, mode doc.OpenMode) (*doc.Document, error) {
	if archive.Foreign(path) {
		if mode == doc.OpenCoEdit || mode == doc.OpenAnyway {
			mode = doc.OpenNormal
		}
		return doc.OpenXLSX(ctx, path, mode, s.sessionLang())
	}
	return doc.OpenAs(ctx, path, mode)
}

// xlsxSaveMode lit files.xlsxSave (ask par défaut).
func xlsxSaveMode() string {
	switch v := config.Current().String(XLSXSaveSetting, "ask"); v {
	case "keep", "convert":
		return v
	}
	return "ask"
}

// saveXLSXDoc : Enregistrer sur un classeur adossé à un .xlsx. Rend la
// question à poser (st.XLSX), ou enregistre selon le réglage.
func (s *DocumentService) saveXLSXDoc(ctx context.Context, d *doc.Document) (DocState, error) {
	if s.xlsxKept(d) {
		return s.writeXLSX(ctx, d)
	}
	mode := xlsxSaveMode()
	if mode == "convert" {
		return s.convertXLSX(ctx, d)
	}
	rep, err := transfer.XLSXLosses(ctx, d.Model)
	if err != nil {
		return DocState{}, err
	}
	if mode == "keep" && !rep.Lossy() {
		return s.writeXLSX(ctx, d)
	}
	st, err := s.State(ctx)
	st.XLSX = &XLSXQuestion{File: filepath.Base(d.Path()), Ask: mode == "ask", Report: rep}
	return st, err
}

// SaveXLSX répond à DocState.XLSX : keep enregistre dans le fichier .xlsx
// (sans plus demander jusqu'à la fermeture du classeur), sinon le classeur
// est converti en .lattis (Enregistrer sous, à côté du fichier).
func (s *DocumentService) SaveXLSX(ctx context.Context, keep bool) (DocState, error) {
	d, err := s.current()
	if err != nil {
		return DocState{}, err
	}
	if !d.XLSX() {
		return s.Save(ctx, false)
	}
	if !keep {
		return s.convertXLSX(ctx, d)
	}
	s.keepXLSX(d)
	return s.writeXLSX(ctx, d)
}

// writeXLSX enregistre dans le fichier .xlsx du classeur.
func (s *DocumentService) writeXLSX(ctx context.Context, d *doc.Document) (DocState, error) {
	if err := saveDoc(ctx, d, ""); err != nil {
		return s.saveFailed(ctx, d, err)
	}
	return s.saved(ctx, d)
}

// convertXLSX enregistre le classeur en .lattis, à côté du fichier .xlsx :
// il devient un classeur Lattis ordinaire.
func (s *DocumentService) convertXLSX(ctx context.Context, d *doc.Document) (DocState, error) {
	return s.saveAs(ctx, d, docBaseName(d)+".lattis")
}

// saveAsPath : chemin choisi dans Enregistrer sous, avec son extension
// (.lattis si l'utilisateur n'a mis ni .lattis ni .xlsx).
func saveAsPath(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".lattis", ".xlsx":
		return p
	}
	return withExt(p, ".lattis")
}
