// Package pages embarque les pages de l'aide de l'interface (Markdown), pour
// que l'API (help.search : MCP, Lua, assistant) cherche dans la même aide que
// le volet de l'application.
package pages

import "embed"

//go:embed *.md
var FS embed.FS
