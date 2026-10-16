package transfer

import (
	"context"

	"github.com/lalamefine/slitable/internal/canvas"
)

// ReadXLSX lit un classeur .xlsx comme l'import (îlots, en-têtes, types)
// sans rien créer : les requêtes d'import (internal/query) y choisissent un
// tableau. Les lignes de chaque tableau (ImportTable.Rows) se relisent dans
// le fichier tant que close n'a pas été appelé.
func ReadXLSX(ctx context.Context, path string) (sheets []canvas.ImportSheet, close func() error, err error) {
	x, err := openXLSX(path)
	if err != nil {
		return nil, nil, err
	}
	sheets, _, err = x.plan(ctx, map[string]bool{})
	if err != nil {
		x.Close()
		return nil, nil, err
	}
	return sheets, x.Close, nil
}
