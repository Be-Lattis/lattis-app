package transfer

import (
	"context"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Commentaires en fil (canvas/comments.go) exportés en .xlsx comme notes,
// par le même chemin que les notes (collectExtras, commentsXML) : une note
// par fil, ses messages « Auteur : texte » à la suite, « ✓ » pour un fil
// résolu ; une cellule qui a déjà une note la garde, le fil à la suite.

// addThreadNotes ajoute les fils de la feuille sh à ses notes exportées.
func addThreadNotes(ctx context.Context, m *canvas.Model, sh *xsheet) error {
	if sh.extra == nil {
		return nil
	}
	th, err := m.Threads(ctx, sh.info.ID)
	if err != nil {
		return err
	}
	for _, t := range th {
		if len(t.Comments) == 0 || t.Row >= ExcelRows || t.Col >= ExcelCols {
			continue
		}
		text := threadText(t)
		merged := false
		for i := range sh.extra.notes {
			if n := &sh.extra.notes[i]; n.r == t.Row && n.c == t.Col {
				n.text += "\n\n" + text
				merged = true
			}
		}
		if !merged {
			sh.extra.notes = append(sh.extra.notes, noteOut{r: t.Row, c: t.Col, text: text})
		}
	}
	return nil
}

// threadText écrit un fil en texte de note : « Alice : … », « Bob : … ».
func threadText(t canvas.Thread) string {
	var lines []string
	for _, c := range t.Comments {
		lines = append(lines, c.Author+" : "+c.Text)
	}
	if t.Resolved {
		lines = append(lines, "✓")
	}
	return strings.Join(lines, "\n")
}
