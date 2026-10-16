package transfer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lalamefine/slitable/internal/canvas"
)

// ImportFile importe un fichier d'après son extension : .csv, .tsv, .txt
// ou .xlsx (et .xlsm, dont les macros sont ignorées).
func ImportFile(ctx context.Context, m *canvas.Model, path string) (canvas.ImportResult, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv", ".tsv", ".txt":
		return ImportCSV(ctx, m, path, CSVOptions{})
	case ".xlsx", ".xlsm":
		return ImportXLSX(ctx, m, path)
	case ".xls":
		return canvas.ImportResult{}, fmt.Errorf("%s : l'ancien format .xls n'est pas lu ; enregistrez-le en .xlsx depuis Excel", filepath.Base(path))
	}
	return canvas.ImportResult{}, fmt.Errorf("%s : format non reconnu (CSV ou .xlsx attendu)", filepath.Base(path))
}
