// Package msg : messages du moteur montrés à l'utilisateur, dans la langue
// de sa session (docs/i18n.md). Un message a un identifiant (ID) et des
// paramètres nommés ; son texte vient d'un catalogue par langue, le français
// servant de référence et de repli. Une erreur à montrer est un *Error : son
// Error() est en français (journaux et tests lisibles), le pont la traduit
// en la rendant à l'interface (bridge.MarshalError, transport web).
//
// Syntaxe des textes, la même que t() côté interface
// (frontend/src/lib/i18n) : {nom} est remplacé par le paramètre nom ;
// {n, plural, =0 {aucune ligne} one {# ligne} other {# lignes}} choisit
// selon le nombre n (# : ce nombre). Pas d'échappement par apostrophe.
package msg

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
)

// Lang : langue de l'interface (code ISO 639-1).
type Lang string

const (
	FR Lang = "fr"
	EN Lang = "en"
	DE Lang = "de"
	ES Lang = "es"
	ZH Lang = "zh" // chinois simplifié
)

// Langs : les langues proposées, le français (référence) en premier.
var Langs = []Lang{FR, EN, DE, ES, ZH}

// Auto : réglage ui.language qui suit la langue du système (ou du
// navigateur en web).
const Auto = "auto"

// Fallback : langue d'un système dont la langue n'est pas proposée.
const Fallback = EN

// Parse lit une langue dans un code de langue (« fr », « en-US »,
// « zh-Hans-CN », « de_DE.UTF-8 ») ; faux si elle n'est pas proposée.
func Parse(tag string) (Lang, bool) {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(tag, "-_.@"); i >= 0 {
		tag = tag[:i]
	}
	for _, l := range Langs {
		if string(l) == tag {
			return l, true
		}
	}
	return "", false
}

// Resolve donne la langue de la session : le réglage (ui.language) s'il
// nomme une langue proposée, sinon (auto, vide) celle du système (system,
// code de langue), sinon Fallback.
func Resolve(setting, system string) Lang {
	if l, ok := Parse(setting); ok {
		return l
	}
	if l, ok := Parse(system); ok {
		return l
	}
	return Fallback
}

var current atomic.Value // Lang

// Current : langue de la session du bureau (une seule fenêtre) ; le moteur
// web garde la sienne par session (bridge.DocumentService). Français tant
// que SetCurrent n'a pas été appelé.
func Current() Lang {
	if l, ok := current.Load().(Lang); ok {
		return l
	}
	return FR
}

// SetCurrent fixe la langue de la session du bureau.
func SetCurrent(l Lang) {
	if _, ok := Parse(string(l)); ok {
		current.Store(l)
	}
}

// ID : identifiant d'un message, pointé par domaine (« table.nameTaken »).
type ID string

// Args : paramètres nommés d'un message.
type Args map[string]any

// catalogs : textes par langue ; le français et l'anglais sont complets, les
// autres se remplissent au fil des traductions (une clé absente rend le
// français ; TestCatalogueParity vérifie les identifiants et paramètres).
var catalogs = map[Lang]map[ID]string{FR: fr, EN: en, DE: de, ES: es, ZH: zh}

// Has dit si le catalogue français connaît id.
func Has(id ID) bool {
	_, ok := fr[id]
	return ok
}

// Format rend le message id dans la langue l : son texte, à défaut le
// français, à défaut l'identifiant lui-même.
func Format(l Lang, id ID, args Args) string {
	s, ok := catalogs[l][id]
	if !ok || s == "" {
		if s, ok = fr[id]; !ok {
			return string(id)
		}
		l = FR
	}
	return render(l, s, args, "")
}

// Error : erreur montrée à l'utilisateur. Err, facultative, est l'erreur
// qu'elle précise (errors.Is(err, canvas.ErrTable) reste vrai).
type Error struct {
	ID   ID
	Args Args
	Err  error
}

// New crée une erreur à montrer.
func New(id ID, args Args) *Error { return &Error{ID: id, Args: args} }

// Wrap crée une erreur à montrer qui précise kind (sentinelle).
func Wrap(kind error, id ID, args Args) *Error { return &Error{ID: id, Args: args, Err: kind} }

// Error : le texte français.
func (e *Error) Error() string { return Format(FR, e.ID, e.Args) }

// Text : le texte dans la langue l.
func (e *Error) Text(l Lang) string { return Format(l, e.ID, e.Args) }

func (e *Error) Unwrap() error { return e.Err }

// Msg : le message de l'erreur.
func (e *Error) Msg() Message { return Message{ID: e.ID, Args: e.Args} }

// Messager : une erreur qui porte un message (*Error, ou une erreur typée
// dont Error() est Msg().String()) ; Text et le pont la traduisent.
type Messager interface {
	error
	Msg() Message
}

// Is : même message (deux *Error de même identifiant).
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.ID == e.ID
}

// Message : un message montré à l'utilisateur qui n'est pas une erreur
// (note de la barre d'état, morceau d'un autre message). String() est le
// texte français.
type Message struct {
	ID   ID
	Args Args
}

// M crée un message.
func M(id ID, args Args) Message { return Message{ID: id, Args: args} }

// Text : le texte dans la langue l.
func (m Message) Text(l Lang) string { return Format(l, m.ID, m.Args) }

// String : le texte français.
func (m Message) String() string { return Format(FR, m.ID, m.Args) }

// In rend les paramètres dans la langue l, prêts pour JSON : les messages,
// les erreurs et les valeurs qui s'écrivent (fmt.Stringer) deviennent leur
// texte, le reste ne change pas.
func (a Args) In(l Lang) Args {
	if a == nil {
		return nil
	}
	out := make(Args, len(a))
	for k, v := range a {
		switch v.(type) {
		case texter, error, fmt.Stringer:
			out[k] = value(l, v)
		default:
			out[k] = v
		}
	}
	return out
}

// Text rend err dans la langue l : traduite si c'est un Messager (*Error,
// erreur typée), ou si elle en enveloppe un (fmt.Errorf("…: %w", e) : seul
// le texte de e change) ; err.Error() sinon.
func Text(l Lang, err error) string {
	if err == nil {
		return ""
	}
	var m Messager
	if !errors.As(err, &m) || l == FR {
		return err.Error()
	}
	s, fr, tr := err.Error(), m.Error(), m.Msg().Text(l)
	if s == fr {
		return tr
	}
	return strings.Replace(s, fr, tr, 1)
}

// render remplace les {paramètres} et choisit les pluriels de s ; hash est
// le nombre que vaut « # » dans une branche de pluriel.
func render(l Lang, s string, args Args, hash string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		if c == '#' && hash != "" {
			b.WriteString(hash)
			i++
			continue
		}
		if c != '{' {
			b.WriteByte(c)
			i++
			continue
		}
		j := closing(s, i)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(placeholder(l, s[i+1:j], args, s[i:j+1]))
		i = j + 1
	}
	return b.String()
}

// closing : position de l'accolade qui ferme celle de s[i] (-1 sans).
func closing(s string, i int) int {
	depth := 0
	for k := i; k < len(s); k++ {
		switch s[k] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return k
			}
		}
	}
	return -1
}

// placeholder rend {inner} : un paramètre, ou un pluriel ; raw (le texte
// tel quel) pour un paramètre absent.
func placeholder(l Lang, inner string, args Args, raw string) string {
	name, rest, plural := strings.Cut(inner, ",")
	name = strings.TrimSpace(name)
	v, ok := args[name]
	if !plural {
		if !ok {
			return raw
		}
		return value(l, v)
	}
	kind, branches, _ := strings.Cut(rest, ",")
	if strings.TrimSpace(kind) != "plural" {
		return raw
	}
	n, isNum := number(v)
	if !ok || !isNum {
		n = 0
	}
	cases := parseBranches(branches)
	text, found := cases["="+strconv.FormatFloat(n, 'f', -1, 64)]
	if !found {
		text, found = cases[Category(l, n)]
	}
	if !found {
		text = cases["other"]
	}
	return render(l, text, args, value(l, n))
}

// parseBranches lit « one {…} other {…} » en sélecteur → texte.
func parseBranches(s string) map[string]string {
	out := map[string]string{}
	for i := 0; i < len(s); {
		open := strings.IndexByte(s[i:], '{')
		if open < 0 {
			break
		}
		sel := strings.TrimSpace(s[i : i+open])
		j := closing(s, i+open)
		if j < 0 {
			break
		}
		out[sel] = s[i+open+1 : j]
		i = j + 1
	}
	return out
}

// Category : catégorie de pluriel CLDR de n dans la langue l (one, other) :
// de 0 à 2 exclu au singulier en français, 1 seul en anglais, allemand et
// espagnol, pas de pluriel en chinois (comme Intl.PluralRules).
func Category(l Lang, n float64) string {
	switch l {
	case ZH:
		return "other"
	case FR:
		if n >= 0 && n < 2 { // CLDR : partie entière 0 ou 1 (1,5 ligne)
			return "one"
		}
		return "other"
	}
	if n == 1 {
		return "one"
	}
	return "other"
}

func number(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint64:
		return float64(x), true
	case float64:
		return x, true
	case float32:
		return float64(x), true
	}
	return 0, false
}

// texter : une valeur qui s'écrit dans une langue (*Error, Message).
type texter interface{ Text(Lang) string }

// value écrit un paramètre : un message (*Error, Message) ou une erreur qui
// en enveloppe un dans la langue l, un nombre avec la décimale de la langue
// (sans séparateur de milliers), le reste tel que fmt l'écrit.
func value(l Lang, v any) string {
	switch x := v.(type) {
	case texter:
		return x.Text(l)
	case error:
		return Text(l, x)
	}
	n, ok := number(v)
	if !ok {
		return fmt.Sprint(v)
	}
	s := strconv.FormatFloat(n, 'f', -1, 64)
	if l == FR || l == DE || l == ES {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}
