// Package helpdoc cherche dans l'aide de l'application (pages Markdown du
// volet d'aide), découpée en sections par titre : la même aide pour
// l'utilisateur, l'assistant, les scripts et les clients MCP. Chaque langue
// a ses pages (pages/<langue>/*.md, le français à la racine) ; une page
// absente d'une langue est prise en français.
package helpdoc

import (
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/lalamefine/lattis/frontend/src/lib/help/pages"
	"github.com/lalamefine/lattis/internal/msg"
)

// Section est une partie d'une page d'aide (sous un titre ## ou ###).
type Section struct {
	Page  string `json:"page"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// Page est une page d'aide entière.
type Page struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text,omitempty"`
}

// help : les pages d'une langue et leurs sections.
type help struct {
	pages    []Page
	sections []Section
}

var (
	mu    sync.Mutex
	langs = map[msg.Lang]*help{}
)

// load donne l'aide de la langue l : ses pages, complétées des pages
// françaises qui lui manquent (langue inconnue : le français).
func load(l msg.Lang) *help {
	if _, ok := msg.Parse(string(l)); !ok {
		l = msg.FR
	}
	mu.Lock()
	defer mu.Unlock()
	if h := langs[l]; h != nil {
		return h
	}
	h := &help{}
	have := map[string]bool{}
	read := func(pattern string) {
		files, _ := fs.Glob(pages.FS, pattern)
		for _, f := range files {
			id := strings.TrimSuffix(path.Base(f), ".md")
			if have[id] {
				continue
			}
			if b, err := pages.FS.ReadFile(f); err == nil {
				have[id] = true
				h.add(id, string(b))
			}
		}
	}
	if l != msg.FR {
		read(string(l) + "/*.md")
	}
	read("*.md")
	slices.SortStableFunc(h.pages, func(a, b Page) int { return strings.Compare(a.ID, b.ID) })
	langs[l] = h
	return h
}

// add découpe une page en sections.
func (h *help) add(id, text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	p := Page{ID: id, Title: id, Text: text}
	cur := Section{Page: id}
	var body strings.Builder
	flush := func() {
		if t := strings.TrimSpace(body.String()); t != "" || cur.Title != "" {
			cur.Text = t
			h.sections = append(h.sections, cur)
		}
		body.Reset()
	}
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "# "):
			p.Title = strings.TrimSpace(line[2:])
			cur.Title = p.Title
		case strings.HasPrefix(line, "## "), strings.HasPrefix(line, "### "):
			flush()
			cur = Section{Page: id, Title: strings.TrimSpace(strings.TrimLeft(line, "# "))}
		default:
			body.WriteString(line + "\n")
		}
	}
	flush()
	h.pages = append(h.pages, p)
}

// Pages liste les pages françaises (sans leur texte).
func Pages() []Page { return PagesIn(msg.FR) }

// PagesIn liste les pages de la langue l (sans leur texte).
func PagesIn(l msg.Lang) []Page {
	h := load(l)
	out := make([]Page, len(h.pages))
	for i, p := range h.pages {
		out[i] = Page{ID: p.ID, Title: p.Title}
	}
	return out
}

// Get rend une page française entière ; faux si elle n'existe pas.
func Get(id string) (Page, bool) { return GetIn(msg.FR, id) }

// GetIn rend une page entière dans la langue l (en français si elle n'y est
// pas traduite) ; faux si elle n'existe pas.
func GetIn(l msg.Lang, id string) (Page, bool) {
	for _, p := range load(l).pages {
		if p.ID == id {
			return p, true
		}
	}
	return Page{}, false
}

// accents : lettres accentuées (français, allemand, espagnol) et leur forme
// simple.
var accents = strings.NewReplacer("à", "a", "â", "a", "ä", "a", "á", "a", "ç", "c", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i", "í", "i", "ô", "o", "ö", "o", "ó", "o", "ù", "u", "û", "u", "ü", "u", "ú", "u", "ÿ", "y", "ñ", "n",
	"ß", "ss", "œ", "oe", "æ", "ae", "’", "'")

// fold met en minuscules et retire les accents.
func fold(s string) string { return accents.Replace(strings.ToLower(s)) }

// Search cherche dans l'aide française (voir SearchIn).
func Search(query string, max int) []Section { return SearchIn(msg.FR, query, max) }

// SearchIn rend au plus max sections de l'aide de la langue l qui
// contiennent les mots de query, les plus pertinentes d'abord (mots dans le
// titre comptés triple) ; rien dans cette langue : l'aide française.
func SearchIn(l msg.Lang, query string, max int) []Section {
	words := strings.FieldsFunc(fold(query), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '_' })
	if len(words) == 0 {
		return nil
	}
	out := search(load(l), words, max)
	if len(out) == 0 && l != msg.FR {
		out = search(load(msg.FR), words, max)
	}
	return out
}

func search(h *help, words []string, max int) []Section {
	type hit struct {
		s     Section
		score int
	}
	var hits []hit
	for _, s := range h.sections {
		title, text := fold(s.Title), fold(s.Text)
		score, all := 0, true
		for _, w := range words {
			n := strings.Count(text, w) + 3*strings.Count(title, w)
			if n == 0 {
				all = false
			}
			score += n
		}
		if score == 0 {
			continue
		}
		if all {
			score += 100
		}
		hits = append(hits, hit{s, score})
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return b.score - a.score })
	out := []Section{}
	for _, h := range hits {
		if len(out) == max {
			break
		}
		if r := []rune(h.s.Text); len(r) > 1500 {
			h.s.Text = string(r[:1500]) + "…"
		}
		out = append(out, h.s)
	}
	return out
}
