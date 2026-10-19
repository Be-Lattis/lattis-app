package bridge

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
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

// ExportSheetCSV écrit toute une feuille en CSV (de A1 à la dernière cellule
// utilisée, tableaux et cellules libres à leur place). Renvoie le fichier
// écrit, vide si l'utilisateur a annulé.
func (s *DocumentService) ExportSheetCSV(ctx context.Context, sheet int64, name string) (string, error) {
	d, err := s.current()
	if err != nil {
		return "", err
	}
	return s.sys().SaveFile(FileDialog{
		Title:     "Exporter la feuille " + name,
		Filename:  name + ".csv",
		Directory: docDir(d.Path()),
		Filters:   []FileFilter{{"CSV", "*.csv"}},
		Ext:       ".csv",
	}, func(p string) error {
		return transfer.ExportSheetCSVFile(ctx, d.Model, sheet, p)
	})
}

// PrintHTML rend une feuille en pages HTML imprimables, selon sa mise en
// page (Fichier › Imprimer / Exporter en PDF).
func (s *DocumentService) PrintHTML(ctx context.Context, sheet int64) (string, error) {
	d, err := s.current()
	if err != nil {
		return "", err
	}
	return transfer.PrintHTML(ctx, d.Model, sheet, transfer.PrintInfo{Path: d.Path()})
}

// PageBreaks donne les sauts de page d'une feuille (traits pointillés de la
// grille, onglet Mise en page).
func (s *DocumentService) PageBreaks(ctx context.Context, sheet int64) (transfer.PageBreaks, error) {
	d, err := s.current()
	if err != nil {
		return transfer.PageBreaks{}, err
	}
	return transfer.SheetPageBreaks(ctx, d.Model, sheet)
}

// XLSXExport est le résultat d'un export .xlsx ou .ods : le fichier écrit
// (vide si l'utilisateur a annulé), le nombre de cellules hors des limites
// d'Excel (et de LibreOffice) et le compte rendu de ce qui n'est pas exporté
// tel quel (graphiques, formules en valeurs…).
type XLSXExport struct {
	Path    string                `json:"path"`
	Clipped int                   `json:"clipped"`
	Report  transfer.ExportReport `json:"report"`
}

// ExportXLSX écrit tout le classeur au format Excel .xlsx : tableaux Excel,
// formules traduites quand Excel a un équivalent (sinon leur résultat),
// mise en forme.
func (s *DocumentService) ExportXLSX(ctx context.Context) (XLSXExport, error) {
	return s.exportBook(ctx, ".xlsx", "Exporter le classeur au format Excel", FileFilter{"Classeur Excel", "*.xlsx"}, transfer.ExportXLSXFile)
}

// ExportODS écrit tout le classeur au format OpenDocument .ods (LibreOffice) :
// formules traduites quand c'est possible, mise en forme, tableaux en plages
// nommées.
func (s *DocumentService) ExportODS(ctx context.Context) (XLSXExport, error) {
	return s.exportBook(ctx, ".ods", "Exporter le classeur au format OpenDocument", FileFilter{"Classeur OpenDocument", "*.ods"}, transfer.ExportODSFile)
}

func (s *DocumentService) exportBook(ctx context.Context, ext, title string, filter FileFilter, export func(context.Context, *canvas.Model, string) (transfer.ExportReport, error)) (XLSXExport, error) {
	d, err := s.current()
	if err != nil {
		return XLSXExport{}, err
	}
	name := "Classeur" + ext
	if d.Path() != "" {
		base := filepath.Base(d.Path())
		name = strings.TrimSuffix(base, filepath.Ext(base)) + ext
	}
	var rep transfer.ExportReport
	p, err := s.sys().SaveFile(FileDialog{
		Title:     title,
		Filename:  name,
		Directory: docDir(d.Path()),
		Filters:   []FileFilter{filter},
		Ext:       ext,
	}, func(p string) (err error) {
		rep, err = export(ctx, d.Model, p)
		return err
	})
	return XLSXExport{Path: p, Clipped: rep.Clipped, Report: rep}, err
}

// docDir est le dossier du classeur enregistré, "" sinon.
func docDir(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Dir(path)
}
