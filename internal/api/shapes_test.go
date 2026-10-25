package api

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"
)

func TestObjectsLua(t *testing.T) {
	h := newHost(t)
	run(t, h, `
local f = objects.create(2, 3, {kind = "roundRect", text = "Objectif", bold = true, name = "Cadre"})
local a = objects.create(6, 3, {kind = "arrow", width = 120, height = 0, line = "#c00000", lineWidth = 3})
local l = objects.list()
assert(#l == 2 and l[1].id == f and l[1].kind == "roundRect" and l[1].row == 2 and l[1].col == 3 and l[1].text == "Objectif", l[1].kind)
assert(l[2].kind == "arrow" and l[2].height == 0)
objects.delete("Cadre")
assert(#objects.list() == 1)
local ok = pcall(objects.create, 1, 1, {kind = "star"})
assert(not ok)
`)
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewGray(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}
	id := call(t, h, "objects.create", map[string]any{"row": 1, "col": 1, "config": map[string]any{"kind": "image", "data": base64.StdEncoding.EncodeToString(b.Bytes())}})
	if id == nil {
		t.Fatal("image non posée")
	}
	if got := asJSON(call(t, h, "objects.list", nil)); !bytes.Contains([]byte(got), []byte(`"kind":"image"`)) {
		t.Fatalf("objets %s", got)
	}
}
