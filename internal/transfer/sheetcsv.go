package transfer

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"

	"github.com/lalamefine/lattis/internal/canvas"
)

// ExportSheetCSV écrit une feuille entière en CSV pour Excel en français
// (séparateur ;, UTF-8 avec BOM) : de A1 à la dernière cellule utilisée,
// tableaux, tableaux croisés et cellules libres à leur place, valeurs telles
// que la grille les affiche ; un texte qui commence comme une formule est
// neutralisé (Neutralize).
func ExportSheetCSV(ctx context.Context, m *canvas.Model, sheet int64, w io.Writer) error {
	l, err := m.Layout(ctx, sheet)
	if err != nil {
		return err
	}
	var info canvas.SheetInfo
	for _, s := range l.Sheets {
		if s.ID == sheet {
			info = s
		}
	}
	if info.ID == 0 {
		return fmt.Errorf("feuille %d introuvable", sheet)
	}
	sh, err := collectSheet(ctx, m, info)
	if err != nil {
		return err
	}
	rows, cols := 0, 0
	for k, x := range sh.cells {
		if x.kind != 0 {
			rows, cols = max(rows, k[0]+1), max(cols, k[1]+1)
		}
	}
	bw := bufio.NewWriterSize(w, 64<<10)
	bw.WriteString("Feff")
	cw := csv.NewWriter(bw)
	cw.UseCRLF = true
	cw.Comma = ';'
	rec := make([]string, cols)
	for r := 0; r < rows; r++ {
		for c := range rec {
			rec[c] = ""
			if x, ok := sh.cells[[2]int{r, c}]; ok && x.kind != 0 {
				rec[c] = printText(x)
				if x.kind == 's' {
					rec[c] = Neutralize(rec[c])
				}
			}
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	return bw.Flush()
}

// ExportSheetCSVFile écrit la feuille en CSV dans path (fichier temporaire
// renommé à la fin).
func ExportSheetCSVFile(ctx context.Context, m *canvas.Model, sheet int64, path string) error {
	return writeAtomic(path, func(w io.Writer) error { return ExportSheetCSV(ctx, m, sheet, w) })
}

// MaxReadCells borne une lecture de plage (SheetValues).
const MaxReadCells = 100_000

// SheetValues rend les valeurs affichées du rectangle (r0, c0)–(r1, c1)
// d'une feuille, ligne par ligne : tableaux, tableaux croisés, déversements
// et cellules libres confondus ("" : cellule vide).
func SheetValues(ctx context.Context, m *canvas.Model, sheet int64, r0, c0, r1, c1 int) ([][]string, error) {
	if r1 < r0 || c1 < c0 {
		return nil, fmt.Errorf("plage vide")
	}
	if n := (r1 - r0 + 1) * (c1 - c0 + 1); n > MaxReadCells {
		return nil, fmt.Errorf("plage trop grande : %d cellules, %d au plus (lisez les tableaux avec tables.rows ou db.query)", n, MaxReadCells)
	}
	l, err := m.Layout(ctx, sheet)
	if err != nil {
		return nil, err
	}
	var info canvas.SheetInfo
	for _, s := range l.Sheets {
		if s.ID == sheet {
			info = s
		}
	}
	if info.ID == 0 {
		return nil, fmt.Errorf("feuille %d introuvable", sheet)
	}
	sh, err := collectSheet(ctx, m, info)
	if err != nil {
		return nil, err
	}
	out := make([][]string, r1-r0+1)
	for i := range out {
		out[i] = make([]string, c1-c0+1)
		for j := range out[i] {
			if x, ok := sh.cells[[2]int{r0 + i, c0 + j}]; ok && x.kind != 0 {
				out[i][j] = printText(x)
			}
		}
	}
	return out, nil
}
