// Package pages embarque les pages de l'aide de l'interface (Markdown), pour
// que l'API (help.search : MCP, Lua, assistant) cherche dans la même aide que
// le volet de l'application. Le français est à la racine, les autres
// langues dans leur dossier (en, de, es, zh).
package pages

import "embed"

//go:embed *.md en/*.md de/*.md es/*.md zh/*.md
var FS embed.FS
