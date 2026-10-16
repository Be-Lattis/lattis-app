package bridge

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/lalamefine/slitable/internal/transfer"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ImportResult décrit un import : feuilles et tableaux créés, et ce qui n'a
// pas pu être repris tel quel (formules gardées en valeurs…).
type ImportResult struct {
	Cancelled bool     `json:"cancelled"`
	Sheet     int64    `json:"sheet"` // première feuille importée
	Tables    []string `json:"tables"`
	Warnings  []string `json:"warnings"`
	State     DocState `json:"state"`
}

// Import ajoute au classeur un fichier CSV ou un classeur Excel .xlsx
// (path vide : boîte de dialogue), en une seule étape d'annulation.
func (s *DocumentService) Import(ctx context.Context, path string) (ImportResult, error) {
	d, err := s.current()
	if err != nil {
		return ImportResult{}, err
	}
	if path == "" {
		p, err := application.Get().Dialog.OpenFile().
			SetTitle("Importer des données").
			AddFilter("CSV ou classeur Excel", "*.csv;*.tsv;*.txt;*.xlsx;*.xlsm").
			AddFilter("CSV", "*.csv;*.tsv;*.txt").
			AddFilter("Classeur Excel", "*.xlsx;*.xlsm").
			PromptForSingleSelection()
		if err != nil {
			return ImportResult{}, err
		}
		if p == "" {
			return ImportResult{Cancelled: true}, nil
		}
		path = p
	}
	res, err := transfer.ImportFile(ctx, d.Model, path)
	if err != nil {
		return ImportResult{}, err
	}
	out := ImportResult{Tables: res.Tables, Warnings: res.Warnings}
	if len(res.Sheets) > 0 {
		out.Sheet = res.Sheets[0]
	}
	out.State, err = s.changed(ctx)
	return out, err
}

// ExportCSV écrit un tableau en CSV (séparateur ;, virgule décimale, UTF-8
// avec BOM : ce qu'Excel en français ouvre directement). Renvoie le fichier
// écrit, vide si l'utilisateur a annulé.
func (s *DocumentService) ExportCSV(ctx context.Context, table string) (string, error) {
	d, err := s.current()
	if err != nil {
		return "", err
	}
	dlg := application.Get().Dialog.SaveFile().
		SetMessage("Exporter le tableau "+table).
		SetFilename(table+".csv").
		AddFilter("CSV", "*.csv")
	if d.Path() != "" {
		dlg = dlg.SetDirectory(filepath.Dir(d.Path()))
	}
	p, err := dlg.PromptForSingleSelection()
	if err != nil || p == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(p), ".csv") {
		p += ".csv"
	}
	return p, transfer.ExportCSVFile(ctx, d.Model, table, p, transfer.ExcelFR)
}

// PrintHTML rend la plage utilisée d'une feuille en HTML imprimable
// (Fichier › Imprimer / Exporter en PDF).
func (s *DocumentService) PrintHTML(ctx context.Context, sheet int64) (string, error) {
	d, err := s.current()
	if err != nil {
		return "", err
	}
	return transfer.PrintHTML(ctx, d.Model, sheet)
}

// XLSXExport est le résultat d'un export .xlsx : le fichier écrit (vide si
// l'utilisateur a annulé) et le nombre de cellules hors des limites d'Excel.
type XLSXExport struct {
	Path    string `json:"path"`
	Clipped int    `json:"clipped"`
}

// ExportXLSX écrit tout le classeur au format Excel .xlsx (valeurs : les
// formules sont exportées par leur résultat).
func (s *DocumentService) ExportXLSX(ctx context.Context) (XLSXExport, error) {
	d, err := s.current()
	if err != nil {
		return XLSXExport{}, err
	}
	name := "Classeur.xlsx"
	if d.Path() != "" {
		base := filepath.Base(d.Path())
		name = strings.TrimSuffix(base, filepath.Ext(base)) + ".xlsx"
	}
	dlg := application.Get().Dialog.SaveFile().
		SetMessage("Exporter le classeur au format Excel").
		SetFilename(name).
		AddFilter("Classeur Excel", "*.xlsx")
	if d.Path() != "" {
		dlg = dlg.SetDirectory(filepath.Dir(d.Path()))
	}
	p, err := dlg.PromptForSingleSelection()
	if err != nil || p == "" {
		return XLSXExport{}, err
	}
	if !strings.HasSuffix(strings.ToLower(p), ".xlsx") {
		p += ".xlsx"
	}
	n, err := transfer.ExportXLSXFile(ctx, d.Model, p)
	return XLSXExport{Path: p, Clipped: n}, err
}
