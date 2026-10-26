package bridge

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
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
				{"CSV ou classeur", "*.csv;*.tsv;*.txt;*.xlsx;*.xlsm;*.ods"},
				{"CSV", "*.csv;*.tsv;*.txt"},
				{"Classeur Excel", "*.xlsx;*.xlsm"},
				{"Classeur OpenDocument", "*.ods"},
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
	if err := s.sessionPath(path); err != nil {
		return ImportResult{}, err
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
		Directory: docFolder(d),
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
		Directory: docFolder(d),
		Filters:   []FileFilter{{"CSV", "*.csv"}},
		Ext:       ".csv",
	}, func(p string) error {
		return transfer.ExportSheetCSVFile(ctx, d.Model, sheet, p)
	})
}

// PrintPlan découpe une feuille en pages selon sa mise en page (Fichier ›
// Imprimer / Exporter en PDF) ; sizes : tailles de la grille affichée.
// L'interface écrit le contenu des pages (frontend/src/lib/print).
func (s *DocumentService) PrintPlan(ctx context.Context, sheet int64, sizes transfer.PrintSizes) (transfer.PrintPlan, error) {
	d, err := s.current()
	if err != nil {
		return transfer.PrintPlan{}, err
	}
	return transfer.SheetPrintPlan(ctx, d.Model, sheet, sizes, printInfo(d))
}

// PageBreaks donne les sauts de page d'une feuille (traits pointillés de la
// grille, onglet Mise en page) ; sizes : tailles de la grille affichée.
func (s *DocumentService) PageBreaks(ctx context.Context, sheet int64, sizes transfer.PrintSizes) (transfer.PageBreaks, error) {
	d, err := s.current()
	if err != nil {
		return transfer.PageBreaks{}, err
	}
	return transfer.SheetPageBreaks(ctx, d.Model, sheet, sizes)
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

// SQLExport est le résultat d'un export SQL : le fichier écrit (vide si
// l'utilisateur a annulé) et ce que contient le script.
type SQLExport struct {
	Path   string             `json:"path"`
	Report transfer.SQLReport `json:"report"`
}

// ExportSQL écrit un script SQL (CREATE TABLE, INSERT par lots) des
// tableaux du classeur, d'une feuille (o.Sheet) ou d'un tableau (o.Table),
// dans le dialecte o.Dialect ; avancement dans la barre d'état.
func (s *DocumentService) ExportSQL(ctx context.Context, o transfer.SQLOptions) (_ SQLExport, err error) {
	d, err := s.current()
	if err != nil {
		return SQLExport{}, err
	}
	name := docBaseName(d)
	o.Source = name
	if d.Path() != "" && remoteDoc(d) == nil {
		o.Source = filepath.Base(d.Path())
	}
	if o.Table != "" {
		name = o.Table
	}
	var rep transfer.SQLReport
	p, err := s.sys().SaveFile(FileDialog{
		Title:     "Exporter en SQL",
		Filename:  name + ".sql",
		Directory: docFolder(d),
		Filters:   []FileFilter{{"Script SQL", "*.sql"}},
		Ext:       ".sql",
	}, func(p string) (err error) {
		wctx, t := s.track(ctx, true)
		defer s.untrack(t, &err)
		rep, err = transfer.ExportSQLFile(wctx, d.Model, p, o)
		return err
	})
	return SQLExport{Path: p, Report: rep}, err
}

func (s *DocumentService) exportBook(ctx context.Context, ext, title string, filter FileFilter, export func(context.Context, *canvas.Model, string) (transfer.ExportReport, error)) (XLSXExport, error) {
	d, err := s.current()
	if err != nil {
		return XLSXExport{}, err
	}
	// Classeur du serveur : son titre, pas le nom de son cache.
	name := docBaseName(d) + ext
	var rep transfer.ExportReport
	p, err := s.sys().SaveFile(FileDialog{
		Title:     title,
		Filename:  name,
		Directory: docFolder(d),
		Filters:   []FileFilter{filter},
		Ext:       ext,
	}, func(p string) (err error) {
		rep, err = export(ctx, d.Model, p)
		return err
	})
	return XLSXExport{Path: p, Clipped: rep.Clipped, Report: rep}, err
}

// printInfo : fichier cité par l'en-tête et le pied de page (&[Fichier],
// &[Chemin]) ; pour un classeur du serveur, son titre et l'adresse du
// serveur, pas le cache local.
func printInfo(d *doc.Document) transfer.PrintInfo {
	if r := remoteDoc(d); r != nil {
		return transfer.PrintInfo{File: r.Title, Dir: strings.TrimRight(r.Server, "/") + "/"}
	}
	return transfer.PrintInfo{Path: d.Path()}
}

// docDir est le dossier du classeur enregistré, "" sinon.
func docDir(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Dir(path)
}
