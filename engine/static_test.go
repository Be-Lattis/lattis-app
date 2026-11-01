package engine

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

// Frontend servi au navigateur : compressé quand il l'accepte, et gardé par
// lui pour les fichiers dont le nom change avec le contenu.
func TestEngineStaticCompressedAndCached(t *testing.T) {
	js := strings.Repeat("const a = 1;\n", 500)
	_, srv := newEngine(t, Options{Static: fstest.MapFS{
		"index.html":               {Data: []byte("<!doctype html>")},
		"_app/immutable/chunk.js":  {Data: []byte(js)},
		"_app/immutable/small.js":  {Data: []byte("1")},
		"_app/version.json":        {Data: []byte(`{"version":"1"}`)},
	}})
	get := func(path, enc string) *http.Response {
		req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept-Encoding", enc)
		res, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { res.Body.Close() })
		return res
	}

	res := get("/_app/immutable/chunk.js", "gzip, deflate")
	if res.Header.Get("Content-Encoding") != "gzip" || !strings.Contains(res.Header.Get("Cache-Control"), "immutable") || !strings.Contains(res.Header.Get("Content-Type"), "javascript") {
		t.Fatalf("en-têtes : %v", res.Header)
	}
	z, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(z)
	if err != nil || string(b) != js {
		t.Fatalf("contenu décompressé : %d octets, %v", len(b), err)
	}

	res = get("/_app/immutable/chunk.js", "identity")
	b, _ = io.ReadAll(res.Body)
	if res.Header.Get("Content-Encoding") != "" || string(b) != js {
		t.Fatalf("sans gzip : %v, %d octets", res.Header, len(b))
	}
	if res = get("/_app/immutable/small.js", "gzip"); res.Header.Get("Content-Encoding") != "" {
		t.Fatalf("petit fichier compressé : %v", res.Header)
	}
	if res = get("/_app/version.json", "gzip"); strings.Contains(res.Header.Get("Cache-Control"), "immutable") {
		t.Fatalf("fichier sans empreinte gardé : %v", res.Header)
	}
}
