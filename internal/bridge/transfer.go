package bridge

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/lalamefine/lattis/internal/transfer"
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
		p, err := s.sys().OpenFile(FileDialog{
			Title: "Importer des données",
			Filters: []FileFilter{
				{"CSV ou classeur Excel", "*.csv;*.tsv;*.txt;*.xlsx;*.xlsm"},
				{"CSV", "*.csv;*.tsv;*.txt"},
				{"Classeur Excel", "*.xlsx;*.xlsm"},
			},
		})
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
	return s.sys().SaveFile(FileDialog{
		Title:     "Exporter le tableau " + table,
		Filename:  table + ".csv",
		Directory: docDir(d.Path()),
		Filters:   []FileFilter{{"CSV", "*.csv"}},
		Ext:       ".csv",
	}, func(p string) error {
		return transfer.ExportCSVFile(ctx, d.Model, table, p, transfer.ExcelFR)
	})
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
	var n int
	p, err := s.sys().SaveFile(FileDialog{
		Title:     "Exporter le classeur au format Excel",
		Filename:  name,
		Directory: docDir(d.Path()),
		Filters:   []FileFilter{{"Classeur Excel", "*.xlsx"}},
		Ext:       ".xlsx",
	}, func(p string) (err error) {
		n, err = transfer.ExportXLSXFile(ctx, d.Model, p)
		return err
	})
	return XLSXExport{Path: p, Clipped: n}, err
}

// docDir est le dossier du classeur enregistré, "" sinon.
func docDir(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Dir(path)
}
