package api

import (
	"strings"
	"testing"
)

// cells.goal_seek : la valeur trouvée est posée (keep par défaut), ou
// seulement rendue avec keep = false.
func TestGoalSeekCommand(t *testing.T) {
	h := newHost(t)
	call(t, h, "cells.set", map[string]any{"row": int64(1), "col": int64(1), "value": int64(4)})
	call(t, h, "cells.set", map[string]any{"row": int64(1), "col": int64(3), "value": "=A1*3+2"})
	r := call(t, h, "cells.goal_seek", map[string]any{"target": "C1", "value": float64(20), "changing": "A1", "keep": false}).(map[string]any)
	if r["found"] != true || r["value"] != float64(6) || r["result"] != "20" {
		t.Fatalf("recherche : %s", asJSON(r))
	}
	if got := asJSON(call(t, h, "cells.get", map[string]any{"row": int64(1), "col": int64(1)})); !strings.Contains(got, `"raw":"4"`) {
		t.Fatalf("keep = false a changé A1 : %s", got)
	}
	call(t, h, "cells.goal_seek", map[string]any{"target": "C1", "value": float64(20), "changing": "A1"})
	if got := asJSON(call(t, h, "cells.get", map[string]any{"row": int64(1), "col": int64(1)})); !strings.Contains(got, `"raw":"6"`) {
		t.Fatalf("valeur posée : %s", got)
	}
}
