package ai

import (
	"encoding/json"
	"strings"
)

// Longueur de la conversation : tout l'historique, résultats d'outils
// compris, repart au modèle à chaque tour. Au-delà de MaxHistoryChars, les
// résultats d'outils les plus anciens sont remplacés par OldResult, jusqu'à
// revenir sous les deux tiers du seuil (pour ne pas recommencer à chaque
// tour, ce qui viderait aussi le cache du fournisseur). Les appels et leurs
// résultats restent appariés : la conversation reste valide.

// MaxHistoryChars : seuil de longueur de l'historique, en caractères
// (environ 40 000 jetons).
const MaxHistoryChars = 150_000

// OldResult remplace le contenu d'un résultat d'outil ancien.
const OldResult = "[résultat ancien retiré]"

// keepRecent : messages de la fin jamais raccourcis (la réponse en cours).
const keepRecent = 2

// Entrée d'un ancien appel d'outil plus longue que oldInputMax : remplacée
// par oldInput.
const (
	oldInputMax = 2000
	oldInput    = `{"_":"entrée ancienne retirée"}`
)

// size estime la longueur d'un message (forme neutre).
func size(m Message) int {
	n := 0
	for _, b := range m.Content {
		n += len(b.Text) + len(b.Input) + len(b.Name)
	}
	return n
}

// shorten raccourcit l'historique s'il dépasse max ; à appeler sous s.mu.
// Un message modifié perd sa forme propre au fournisseur, et tous les
// messages qui le suivent aussi : un fournisseur qui lie ses blocs de
// réflexion au début de la conversation (Claude) refuserait de les relire
// après une modification de ce début. Ils repartent dans la forme neutre.
func (s *Session) shorten(max int) bool {
	total := 0
	for _, m := range s.history {
		total += size(m)
	}
	if total <= max {
		return false
	}
	target := max * 2 / 3
	first := -1
	for i := 0; i < len(s.history)-keepRecent && total > target; i++ {
		m := &s.history[i]
		if m.Role != User {
			// Entrées volumineuses des anciens appels (cells_paste…).
			changed := false
			for j := range m.Content {
				b := &m.Content[j]
				if b.Type != ToolUseBlock || len(b.Input) <= oldInputMax {
					continue
				}
				if !changed {
					m.Content = append([]Block(nil), m.Content...)
					b = &m.Content[j]
					changed = true
				}
				total -= len(b.Input) - len(oldInput)
				b.Input = json.RawMessage(oldInput)
			}
			if changed && first < 0 {
				first = i
			}
			continue
		}
		changed := false
		for j := range m.Content {
			b := &m.Content[j]
			if b.Type != ToolResultBlock || b.Text == OldResult || len(b.Text) <= len(OldResult) {
				continue
			}
			if !changed {
				// Copie : les messages déjà rendus (Snapshot, requête en
				// cours) gardent leur contenu.
				m.Content = append([]Block(nil), m.Content...)
				b = &m.Content[j]
				changed = true
			}
			total -= len(b.Text) - len(OldResult)
			b.Text = OldResult
		}
		if changed && first < 0 {
			first = i
		}
	}
	if first < 0 {
		return false
	}
	// Le dernier message de l'assistant garde sa forme native : au milieu
	// d'une boucle d'outils, Claude exige ses blocs de réflexion.
	last := -1
	for i := len(s.history) - 1; i >= 0; i-- {
		if s.history[i].Role == Assistant {
			last = i
			break
		}
	}
	for i := first; i < len(s.history); i++ {
		if i != last {
			s.history[i].Provider, s.history[i].Native = "", nil
		}
	}
	return true
}

// View est ce que l'utilisateur regarde quand il écrit.
type View struct {
	Sheet     string // feuille active
	Table     string // tableau sous le curseur
	Selection string // sélection lisible (« Commandes[Montant][3:10] », « B2:D5 »)
	Today     string // date du jour (AAAA-MM-JJ) ; "" : non donnée
}

// Compose écrit le message envoyé au modèle : remarques (annulations…),
// puis le contexte de l'interface entre crochets, puis le texte tapé. La
// consigne (System) explique ces deux lignes au modèle ; l'interface
// n'affiche que le texte tapé.
func Compose(text string, v View, notes []string) string {
	var b strings.Builder
	for _, n := range notes {
		b.WriteString("[Remarque : " + n + "]\n")
	}
	var ctx []string
	if v.Sheet != "" {
		ctx = append(ctx, "feuille active « "+oneLine(v.Sheet)+" »")
	}
	if v.Table != "" {
		ctx = append(ctx, "tableau « "+oneLine(v.Table)+" »")
	}
	if v.Selection != "" {
		ctx = append(ctx, "sélection "+oneLine(v.Selection))
	}
	if v.Today != "" {
		ctx = append(ctx, "aujourd'hui "+oneLine(v.Today))
	}
	if len(ctx) > 0 {
		b.WriteString("[Contexte : " + strings.Join(ctx, ", ") + "]\n")
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString(text)
	return b.String()
}
