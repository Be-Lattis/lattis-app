package transfer

import (
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Notes d'un fichier .xlsx : les commentaires (notes) et les commentaires
// en fil de discussion deviennent la note de leur cellule, « Auteur :
// texte », une ligne par réponse. À l'export (xlsxnoteswrite.go), chaque
// note devient un commentaire.

// legacyComments lit une partie de commentaires : texte par cellule.
func (x *xlsxFile) legacyComments(p string) (map[string]string, int) {
	f := x.find(p)
	if f == nil {
		return nil, 0
	}
	var c struct {
		Authors  []string `xml:"authors>author"`
		Comments []struct {
			Ref    string `xml:"ref,attr"`
			Author int    `xml:"authorId,attr"`
			Text   struct {
				T  string `xml:"t"`
				Rs []struct {
					T string `xml:"t"`
				} `xml:"r"`
			} `xml:"text"`
		} `xml:"commentList>comment"`
	}
	if x.decode(f, &c) != nil {
		return nil, 0
	}
	out := map[string]string{}
	for _, cm := range c.Comments {
		var b strings.Builder
		b.WriteString(cm.Text.T)
		for _, r := range cm.Text.Rs {
			b.WriteString(r.T)
		}
		text := strings.TrimSpace(strings.ReplaceAll(b.String(), "\r\n", "\n"))
		author := ""
		if cm.Author >= 0 && cm.Author < len(c.Authors) {
			author = strings.TrimSpace(c.Authors[cm.Author])
		}
		if author != "" {
			if rest, ok := strings.CutPrefix(text, author+":"); ok {
				text = strings.TrimSpace(rest)
			}
			text = author + " : " + text
		}
		if text != "" {
			out[strings.ToUpper(cm.Ref)] = text
		}
	}
	return out, len(c.Comments)
}

// threadedComments lit les commentaires en fil (avec les noms des personnes).
func (x *xlsxFile) threadedComments(p string) map[string]string {
	f := x.find(p)
	if f == nil {
		return nil
	}
	var c struct {
		Items []struct {
			Ref    string `xml:"ref,attr"`
			Person string `xml:"personId,attr"`
			Text   string `xml:"text"`
		} `xml:"threadedComment"`
	}
	if x.decode(f, &c) != nil {
		return nil
	}
	people := map[string]string{}
	if _, wb, err := x.rels(""); err == nil {
		if _, byType, err := x.rels(wb["officeDocument"]); err == nil && byType["person"] != "" {
			var pl struct {
				People []struct {
					Name string `xml:"displayName,attr"`
					ID   string `xml:"id,attr"`
				} `xml:"person"`
			}
			if pf := x.find(byType["person"]); pf != nil && x.decode(pf, &pl) == nil {
				for _, p := range pl.People {
					people[p.ID] = p.Name
				}
			}
		}
	}
	out := map[string]string{}
	for _, it := range c.Items {
		ref := strings.ToUpper(it.Ref)
		line := strings.TrimSpace(it.Text)
		if n := people[it.Person]; n != "" {
			line = n + " : " + line
		}
		if out[ref] != "" {
			out[ref] += "\n" + line
		} else {
			out[ref] = line
		}
	}
	return out
}

// placeNotes pose les notes de la feuille.
func (x *xlsxFile) placeNotes(sh xlsxSheet, sb *sheetBuild) {
	if sh.file == nil {
		return
	}
	_, byType, err := x.rels(sh.file.Name)
	if err != nil {
		return
	}
	notes, n := map[string]string{}, 0
	if p := byType["comments"]; p != "" {
		notes, n = x.legacyComments(p)
	}
	if p := byType["threadedComment"]; p != "" {
		for ref, t := range x.threadedComments(p) {
			if notes == nil {
				notes = map[string]string{}
			}
			notes[ref] = t
		}
	}
	for ref, t := range notes {
		if r, c, ok := parseCellRef(ref); ok {
			sb.patch(r, c, canvas.Style{"nt": t})
		}
	}
	x.imp.notes += n
}
