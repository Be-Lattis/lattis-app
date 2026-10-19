package grid

import (
	"fmt"
	"testing"
)

func TestWithoutTechnical(t *testing.T) {
	r := QueryResult{Columns: []string{"id", "_pos", "Nom", "_x"}, Rows: [][]any{{1, "a", "Lyon", 2}}}
	got := r.WithoutTechnical()
	if fmt.Sprint(got.Columns, got.Rows) != "[id Nom] [[1 Lyon]]" {
		t.Fatalf("%v %v", got.Columns, got.Rows)
	}
}
