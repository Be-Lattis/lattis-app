package transfer

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
)

// ImportFile importe un fichier d'après son extension : .csv, .tsv, .txt
// .xlsx (et .xlsm, dont les macros sont ignorées) ou .ods.
func ImportFile(ctx context.Context, m *canvas.Model, path string) (canvas.ImportResult, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv", ".tsv", ".txt":
		return ImportCSV(ctx, m, path, CSVOptions{})
	case ".xlsx", ".xlsm":
		return ImportXLSX(ctx, m, path)
	case ".ods":
		return ImportODS(ctx, m, path)
	case ".xls":
		return canvas.ImportResult{}, msg.New("transfer.xlsNotRead", msg.Args{"file": filepath.Base(path)})
	}
	return canvas.ImportResult{}, msg.New("transfer.unknownFormat", msg.Args{"file": filepath.Base(path)})
}
