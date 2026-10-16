package mcp

import (
	"os"
	"strings"
	"testing"
)

// docs/mcp.md cite chaque outil.
func TestDocListsTools(t *testing.T) {
	b, err := os.ReadFile("../../docs/mcp.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range Tools() {
		if !strings.Contains(string(b), "`"+tool.Name+"`") {
			t.Errorf("docs/mcp.md ne cite pas %s", tool.Name)
		}
	}
}
