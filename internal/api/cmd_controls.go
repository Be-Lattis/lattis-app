package api

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/formula"
)

// Contrôles de feuille (bouton, case à cocher, liste déroulante, zone de
// saisie, sélecteur de fichier) et leurs évènements : un écouteur
// (controls.on) est une fonction globale d'un script de la bibliothèque,
// enregistrée dans le classeur avec le contrôle ; elle est appelée à chaque
// clic ou changement de valeur avec une table {control, event, value, old}.

// ControlView est un contrôle de feuille vu d'un script.
type ControlView struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Kind        string            `json:"kind"` // button, checkbox, select, input, textarea, file
	Label       string            `json:"label"`
	Sheet       string            `json:"sheet"`
	Row         int               `json:"row"`
	Col         int               `json:"col"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
	Link        string            `json:"link,omitempty"`
	Value       any               `json:"value,omitempty"`
	Options     []string          `json:"options,omitempty"`
	OptionsFrom string            `json:"optionsFrom,omitempty"`
	Script      string            `json:"script,omitempty"`
	Function    string            `json:"function,omitempty"`
	Color       string            `json:"color,omitempty"`
	HoverColor  string            `json:"hoverColor,omitempty"`
	ActiveColor string            `json:"activeColor,omitempty"`
	TextColor   string            `json:"textColor,omitempty"`
	Hidden      bool              `json:"hidden"`
	Listeners   []canvas.Listener `json:"listeners"`
}

// ListenerView est un écouteur d'un contrôle ; Action : l'action du
// contrôle lui-même (script et fonction de sa fenêtre Modifier).
type ListenerView struct {
	Control  string `json:"control"`
	Event    string `json:"event"`
	Script   string `json:"script"`
	Function string `json:"function,omitempty"`
	Action   bool   `json:"action,omitempty"`
}

// kindName donne le genre public d'un contrôle ("button" pour un bouton).
func kindName(kind string) string {
	if kind == "" {
		return "button"
	}
	return kind
}

// kindOf lit un genre public ("button" ou "" : bouton).
func kindOf(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "button" {
		s = ""
	}
	if _, ok := canvas.ControlKinds[s]; !ok {
		return "", fmt.Errorf("type « %s » inconnu : button, checkbox, select, input, textarea ou file", s)
	}
	return s, nil
}

// ControlValue convertit la valeur affichée de la cellule liée : un
// booléen pour une case à cocher, un texte sinon.
func ControlValue(kind, shown string) any {
	if kind == "checkbox" {
		switch strings.ToLower(strings.TrimSpace(shown)) {
		case "vrai", "true", "1":
			return true
		}
		return false
	}
	return shown
}

// linkedCell donne la cellule liée d'un contrôle (ok faux : aucune), et sa
// feuille : celle du contrôle, ou celle nommée (« Feuille1!B3 », contrôle
// d'un Onglet Document).
func linkedCell(ctx context.Context, d *doc.Document, b canvas.ButtonInfo) (sheet int64, row, col int, ok bool, err error) {
	if b.Config.Link == "" {
		return 0, 0, 0, false, nil
	}
	name, cell := canvas.SplitLink(b.Config.Link)
	row, col, ok = formula.CellRef("Cell_" + cell)
	if !ok {
		return 0, 0, 0, false, nil
	}
	sheet = b.Sheet
	if name != "" {
		list, err := sheetList(ctx, d)
		if err != nil {
			return 0, 0, 0, false, err
		}
		sheet = 0
		for _, s := range list {
			if strings.EqualFold(s.Name, name) {
				sheet = s.ID
			}
		}
		if sheet == 0 {
			return 0, 0, 0, false, fmt.Errorf("cellule liée « %s » : feuille « %s » introuvable", b.Config.Link, name)
		}
	}
	return sheet, row, col, true, nil
}

// linkedValue lit la valeur affichée de la cellule liée d'un contrôle.
func linkedValue(ctx context.Context, d *doc.Document, b canvas.ButtonInfo) (string, error) {
	sheet, row, col, ok, err := linkedCell(ctx, d, b)
	if err != nil {
		return "", err
	}
	if !ok {
		// Sans cellule liée : la valeur est gardée dans le contrôle.
		return b.Config.Value, nil
	}
	l, err := d.Layout(ctx, sheet)
	if err != nil {
		return "", err
	}
	v, err := readCell(ctx, d, l, row, col)
	if err != nil {
		return "", err
	}
	return raw(v.Value), nil
}

func describeControl(ctx context.Context, d *doc.Document, b canvas.ButtonInfo, sheets map[int64]string) (ControlView, error) {
	c := b.Config
	v := ControlView{
		ID: b.ID, Name: c.Name, Kind: kindName(c.Kind), Label: c.Label, Sheet: sheets[b.Sheet],
		Row: b.Row + 1, Col: b.Col + 1, Width: c.Width, Height: c.Height,
		Link: c.Link, Options: c.Options, OptionsFrom: c.OptionsFrom, Script: c.Script, Function: c.Function,
		Color: c.Color, HoverColor: c.HoverColor, ActiveColor: c.ActiveColor, TextColor: c.TextColor,
		Hidden: c.Hidden, Listeners: c.Listeners,
	}
	if v.Width == 0 {
		v.Width = 120
	}
	if v.Height == 0 {
		v.Height = 32
	}
	if v.Listeners == nil {
		v.Listeners = []canvas.Listener{}
	}
	if c.OptionsFrom != "" {
		v.Options = b.Choices
	}
	if c.Kind != "" {
		s, err := linkedValue(ctx, d, b)
		if err != nil {
			return v, err
		}
		v.Value = ControlValue(c.Kind, s)
	}
	return v, nil
}

func sheetNames(ctx context.Context, d *doc.Document) (map[int64]string, error) {
	list, err := sheetList(ctx, d)
	if err != nil {
		return nil, err
	}
	out := map[int64]string{}
	for _, s := range list {
		out[s.ID] = s.Name
	}
	return out, nil
}

// view décrit le contrôle id.
func (c *Call) view(id int64) (ControlView, error) {
	d, _ := c.Doc()
	b, err := d.Button(c, id)
	if err != nil {
		return ControlView{}, err
	}
	names, err := sheetNames(c, d)
	if err != nil {
		return ControlView{}, err
	}
	return describeControl(c, d, b, names)
}

// control résout un contrôle : identifiant, nom, ou à défaut libellé (la
// casse compte d'abord, puis non).
func (c *Call) control(param string) (canvas.ButtonInfo, error) {
	d, err := c.Doc()
	if err != nil {
		return canvas.ButtonInfo{}, err
	}
	list, err := d.Controls(c)
	if err != nil {
		return canvas.ButtonInfo{}, err
	}
	v := c.Raw(param)
	if f, ok := number(v); ok {
		if _, isStr := v.(string); !isStr {
			for _, b := range list {
				if b.ID == int64(f) {
					return b, nil
				}
			}
			return canvas.ButtonInfo{}, fmt.Errorf("contrôle %d introuvable", int64(f))
		}
	}
	name, _ := c.Str(param)
	for _, get := range []func(canvas.ButtonInfo) string{
		func(b canvas.ButtonInfo) string { return b.Config.Name },
		func(b canvas.ButtonInfo) string { return b.Config.Label },
	} {
		for _, b := range list {
			if get(b) == name {
				return b, nil
			}
		}
		for _, b := range list {
			if strings.EqualFold(get(b), name) {
				return b, nil
			}
		}
	}
	if id, err := strconv.ParseInt(name, 10, 64); err == nil {
		for _, b := range list {
			if b.ID == id {
				return b, nil
			}
		}
	}
	return canvas.ButtonInfo{}, fmt.Errorf("contrôle « %s » introuvable", name)
}

// controlID résout un contrôle et donne son identifiant.
func (c *Call) controlID(param string) (int64, error) {
	b, err := c.control(param)
	return b.ID, err
}

// controlProps applique les propriétés d'une table à un contrôle. Elle
// renvoie la valeur à écrire dans la cellule liée (value) et la nouvelle
// position et taille (place), si elles sont données.
type controlEdit struct {
	value    any
	hasValue bool
	row, col int // à partir de 0 ; -1 : inchangé
	w, h     int // -1 : inchangé
}

var controlKeys = "name, kind, label, link, value, options, script, function, color, hoverColor, activeColor, textColor, hidden, visible, row, col, width, height"

func controlProps(cfg *canvas.ButtonConfig, props map[string]any) (controlEdit, error) {
	e := controlEdit{row: -1, col: -1, w: -1, h: -1}
	str := func(k string, v any) (string, error) {
		switch v := v.(type) {
		case nil:
			return "", nil
		case string:
			return v, nil
		case float64, int64, int:
			return raw(v), nil
		}
		return "", fmt.Errorf("%s : texte attendu", k)
	}
	integer := func(k string, v any) (int, error) {
		f, ok := number(v)
		if !ok || f != float64(int(f)) {
			return 0, fmt.Errorf("%s : nombre entier attendu", k)
		}
		return int(f), nil
	}
	boolean := func(k string, v any) (bool, error) {
		b, ok := v.(bool)
		if !ok {
			return false, fmt.Errorf("%s : booléen attendu", k)
		}
		return b, nil
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := props[k]
		var err error
		switch k {
		case "name":
			cfg.Name, err = str(k, v)
		case "kind":
			var s string
			if s, err = str(k, v); err == nil {
				cfg.Kind, err = kindOf(s)
			}
		case "label":
			cfg.Label, err = str(k, v)
		case "link":
			cfg.Link, err = str(k, v)
		case "optionsFrom":
			cfg.OptionsFrom, err = str(k, v)
		case "script":
			cfg.Script, err = str(k, v)
			if err == nil && cfg.Script == "" {
				cfg.Function = ""
			}
		case "function":
			cfg.Function, err = str(k, v)
		case "color":
			cfg.Color, err = str(k, v)
		case "hoverColor":
			cfg.HoverColor, err = str(k, v)
		case "activeColor":
			cfg.ActiveColor, err = str(k, v)
		case "textColor":
			cfg.TextColor, err = str(k, v)
		case "hidden":
			cfg.Hidden, err = boolean(k, v)
		case "visible":
			var b bool
			b, err = boolean(k, v)
			cfg.Hidden = !b
		case "options":
			list, ok := v.([]any)
			if !ok {
				return e, fmt.Errorf("options : liste de textes attendue")
			}
			cfg.Options = cfg.Options[:0:0]
			for _, o := range list {
				s, err := str(k, o)
				if err != nil {
					return e, err
				}
				cfg.Options = append(cfg.Options, s)
			}
		case "value":
			e.value, e.hasValue = v, true
		case "row", "col":
			var n int
			if n, err = integer(k, v); err == nil && n < 1 {
				err = fmt.Errorf("%s : les lignes et colonnes se comptent à partir de 1", k)
			}
			if k == "row" {
				e.row = n - 1
			} else {
				e.col = n - 1
			}
		case "width":
			e.w, err = integer(k, v)
		case "height":
			e.h, err = integer(k, v)
		default:
			return e, fmt.Errorf("propriété « %s » inconnue (%s)", k, controlKeys)
		}
		if err != nil {
			return e, err
		}
	}
	return e, nil
}

// cellInput écrit la valeur d'un contrôle comme une saisie.
func cellInput(kind string, v any) string {
	if b, ok := v.(bool); ok {
		if b {
			return "VRAI"
		}
		return "FAUX"
	}
	if kind == "checkbox" {
		if ControlValue(kind, raw(v)) == true {
			return "VRAI"
		}
		return "FAUX"
	}
	return raw(v)
}

// setControlValue écrit v dans la cellule liée du contrôle et, si la
// valeur affichée change, déclenche son évènement change.
func (c *Call) setControlValue(b canvas.ButtonInfo, v any) error {
	if b.Config.Kind == "" {
		return fmt.Errorf("un bouton n'a pas de valeur")
	}
	d, _ := c.Doc()
	old, now, err := writeControl(c, d, b, cellInput(b.Config.Kind, v))
	if err != nil || now == old {
		return err
	}
	res, err := dispatch(c, c.Host, controlEvent{id: b.ID, event: canvas.EventChange, value: now, old: old, hasOld: true})
	if err == nil && res.Error != "" {
		err = fmt.Errorf("écouteur de « %s » : %s", b.Config.Name, res.Error)
	}
	return err
}

// --- Évènements -----------------------------------------------------------------

// controlEvent est un évènement d'un contrôle ; value et old sont les
// valeurs affichées de la cellule liée.
type controlEvent struct {
	id         int64
	event      string
	value, old string
	hasOld     bool
}

// eventQueue garde les évènements provoqués par un script, traités à sa fin.
type eventQueue struct {
	mu   sync.Mutex
	list []controlEvent
}

func withQueue(ctx context.Context) (context.Context, *eventQueue) {
	q := &eventQueue{}
	return context.WithValue(ctx, queueKey, q), q
}

func (q *eventQueue) take() []controlEvent {
	q.mu.Lock()
	defer q.mu.Unlock()
	l := q.list
	q.list = nil
	return l
}

// maxRounds borne les évènements en chaîne (un écouteur qui change la
// valeur d'un contrôle, dont l'écouteur…).
const maxRounds = 16

// drain traite les évènements en attente, puis ceux qu'ils provoquent.
func drain(ctx context.Context, host Host, q *eventQueue, res *ScriptResult) error {
	for round := 0; ; round++ {
		evs := q.take()
		if len(evs) == 0 {
			return nil
		}
		if round == maxRounds {
			res.Error = "trop d'évènements en chaîne (un écouteur change-t-il la valeur de son propre contrôle ?)"
			return nil
		}
		for _, ev := range evs {
			r, err := fire(ctx, host, ev)
			res.add(r)
			if err != nil || r.Error != "" {
				return err
			}
		}
	}
}

// dispatch traite un évènement : à la fin du script en cours s'il y en a
// un, sinon tout de suite (serveur MCP), dans l'étape d'annulation de ctx.
func dispatch(ctx context.Context, host Host, ev controlEvent) (ScriptResult, error) {
	if q, ok := ctx.Value(queueKey).(*eventQueue); ok {
		q.mu.Lock()
		q.list = append(q.list, ev)
		q.mu.Unlock()
		return ScriptResult{}, nil
	}
	ctx, q := withQueue(ctx)
	res, err := fire(ctx, host, ev)
	if err != nil || res.Error != "" {
		return res, err
	}
	return res, drain(ctx, host, q, &res)
}

// handlers donne ce qu'un évènement exécute : l'action du contrôle (pour
// son évènement : click d'un bouton, change des autres), puis ses
// écouteurs, dans l'ordre.
func handlers(cfg canvas.ButtonConfig, event string) []canvas.Listener {
	var out []canvas.Listener
	if cfg.Script != "" && event == canvas.DefaultEvent(cfg.Kind) {
		out = append(out, canvas.Listener{Event: event, Script: cfg.Script, Function: cfg.Function})
	}
	for _, l := range cfg.Listeners {
		if l.Event == event {
			out = append(out, l)
		}
	}
	return out
}

// fire exécute les écouteurs d'un évènement, l'un après l'autre ; le
// premier en erreur arrête les suivants.
func fire(ctx context.Context, host Host, ev controlEvent) (ScriptResult, error) {
	res := ScriptResult{Output: []string{}}
	d, err := host.Doc()
	if err != nil {
		return res, err
	}
	b, err := d.Button(ctx, ev.id)
	if err != nil {
		return res, err
	}
	hs := handlers(b.Config, ev.event)
	if len(hs) == 0 {
		return res, nil
	}
	list, err := d.Scripts(ctx)
	if err != nil {
		return res, err
	}
	// Un bouton n'a pas de valeur : value et old restent nil.
	arg := map[string]any{"control": b.Config.Name, "event": ev.event}
	if b.Config.Kind != "" {
		arg["value"] = ControlValue(b.Config.Kind, ev.value)
		if ev.hasOld {
			arg["old"] = ControlValue(b.Config.Kind, ev.old)
		}
	}
	for _, h := range hs {
		src, found := "", false
		for _, s := range list {
			if s.Name == h.Script {
				src, found = s.Source, true
				break
			}
		}
		if !found {
			return res, fmt.Errorf("contrôle « %s » : script « %s » introuvable", b.Config.Name, h.Script)
		}
		r, err := runOne(ctx, host, h.Script, src, h.Function, arg)
		res.add(r)
		if err != nil || r.Error != "" {
			return res, err
		}
	}
	return res, nil
}

// FireControl déclenche l'évènement event du contrôle id depuis
// l'interface (clic, nouvelle valeur déjà écrite dans la cellule liée) :
// l'action du contrôle puis ses écouteurs s'exécutent, et tout ce qu'ils
// modifient forme une seule étape d'annulation.
func FireControl(host Host, id int64, event, value, old string) (ScriptResult, error) {
	d, err := host.Doc()
	if err != nil {
		return ScriptResult{}, err
	}
	b, err := d.Button(contextBG, id)
	if err != nil {
		return ScriptResult{}, err
	}
	if err := canvas.CheckEvent(b.Config.Kind, event); err != nil {
		return ScriptResult{}, err
	}
	hs := handlers(b.Config, event)
	if len(hs) == 0 {
		return ScriptResult{Output: []string{}}, nil
	}
	ctx, q := withQueue(command.WithGroup(context.Background(), scriptLabel(hs[0].Script)))
	res, err := fire(ctx, host, controlEvent{id: id, event: event, value: value, old: old, hasOld: true})
	if err != nil || res.Error != "" {
		return res, err
	}
	return res, drain(ctx, host, q, &res)
}

// writeControl donne une nouvelle valeur à un contrôle : dans sa cellule
// liée, ou dans le contrôle lui-même s'il n'en a pas ; renvoie l'ancienne et
// la nouvelle valeur affichée.
func writeControl(ctx context.Context, d *doc.Document, b canvas.ButtonInfo, raw string) (old, now string, err error) {
	if old, err = linkedValue(ctx, d, b); err != nil {
		return
	}
	sheet, row, col, ok, err := linkedCell(ctx, d, b)
	if err != nil {
		return
	}
	if ok {
		if _, err = d.SetCell(ctx, sheet, row, col, raw); err != nil {
			return
		}
	} else {
		cfg := b.Config
		cfg.Value = raw
		if err = d.SetButton(ctx, b.ID, cfg); err != nil {
			return
		}
	}
	if b, err = d.Button(ctx, b.ID); err != nil {
		return
	}
	now, err = linkedValue(ctx, d, b)
	return
}

// ControlState est un contrôle d'un Onglet Document pour l'interface : sa
// configuration et la valeur affichée de sa cellule liée.
type ControlState struct {
	Config canvas.ButtonConfig `json:"config"`
	Value  string              `json:"value"`
	// Choices : choix d'une liste alimentée par une colonne.
	Choices []string `json:"choices,omitempty"`
	Error   string   `json:"error"`
}

// ControlOf lit un contrôle et la valeur de sa cellule liée.
func ControlOf(host Host, id int64) (ControlState, error) {
	d, err := host.Doc()
	if err != nil {
		return ControlState{}, err
	}
	b, err := d.Button(contextBG, id)
	if err != nil {
		return ControlState{}, err
	}
	st := ControlState{Config: b.Config, Choices: b.Choices}
	if st.Value, err = linkedValue(contextBG, d, b); err != nil {
		st.Error = err.Error()
	}
	return st, nil
}

// SetControlValue écrit raw dans la cellule liée du contrôle id (saisie dans
// un Onglet Document : une étape d'annulation) puis, si la valeur affichée a
// changé, déclenche son action et ses écouteurs (change).
func SetControlValue(host Host, id int64, raw string) (ScriptResult, error) {
	d, err := host.Doc()
	if err != nil {
		return ScriptResult{}, err
	}
	b, err := d.Button(contextBG, id)
	if err != nil {
		return ScriptResult{}, err
	}
	old, now, err := writeControl(contextBG, d, b, raw)
	if err != nil || now == old {
		return ScriptResult{Output: []string{}}, err
	}
	return FireControl(host, id, canvas.EventChange, now, old)
}

// RunButton exécute l'action et les écouteurs du contrôle id, comme un
// clic (bouton) ou un changement de sa valeur actuelle (autres contrôles).
func RunButton(host Host, id int64) (ScriptResult, error) {
	d, err := host.Doc()
	if err != nil {
		return ScriptResult{}, err
	}
	b, err := d.Button(contextBG, id)
	if err != nil {
		return ScriptResult{}, err
	}
	v, err := linkedValue(contextBG, d, b)
	if err != nil {
		return ScriptResult{}, err
	}
	return FireControl(host, id, canvas.DefaultEvent(b.Config.Kind), v, v)
}

// listenersOf liste l'action et les écouteurs d'un contrôle.
func listenersOf(b canvas.ButtonInfo) []ListenerView {
	out := []ListenerView{}
	c := b.Config
	if c.Script != "" {
		out = append(out, ListenerView{Control: c.Name, Event: canvas.DefaultEvent(c.Kind), Script: c.Script, Function: c.Function, Action: true})
	}
	for _, l := range c.Listeners {
		out = append(out, ListenerView{Control: c.Name, Event: l.Event, Script: l.Script, Function: l.Function})
	}
	return out
}

// --- Commandes ------------------------------------------------------------------

var pControl = Param{Name: "control", Type: Ref, Desc: "contrôle (nom, identifiant ou libellé)"}

const controlsHelp = `Un contrôle a un nom unique dans le classeur (« Bouton1 », « Liste1 »…), choisi automatiquement et modifiable. kind : "button" (un clic exécute son action), "checkbox" (case à cocher, VRAI / FAUX), "select" (liste déroulante, options), "input" (texte sur une ligne), "textarea" (texte sur plusieurs lignes) ou "file" (sélecteur de fichier : la cellule reçoit le chemin du fichier choisi). Sauf le bouton, un contrôle est lié à une cellule (link, « B3 ») : sa valeur (value) est celle de cette cellule — un booléen pour une case à cocher, un texte sinon.`

const propsHelp = `Propriétés : name, label (texte du bouton ou de la case, texte indicatif d'une zone de saisie), link, value (écrite dans la cellule liée ; déclenche l'évènement change si elle change), options (liste déroulante), script et function (action du contrôle), color, hoverColor, activeColor, textColor (« #rrggbb »), hidden ou visible, row, col, width, height (pixels).`

const eventsHelp = `Évènements : "click" (bouton) et "change" (nouvelle valeur des autres contrôles, saisie dans la feuille ou controls.set). Un écouteur est une fonction globale d'un script enregistré de la bibliothèque, appelée avec la table {control, event, value, old} (aussi dans la variable globale event) ; il est enregistré dans le classeur et survit au script. L'action du contrôle (script et function) s'exécute d'abord, puis les écouteurs dans l'ordre. Les évènements provoqués par un script s'exécutent à sa fin, dans la même étape d'annulation.`

func init() {
	register(
		&Command{
			Name:    "controls.list",
			Summary: "Liste les contrôles (boutons, cases à cocher, listes, zones de saisie, sélecteurs de fichier) d'une feuille, ou de tout le classeur.",
			Details: controlsHelp,
			Returns: "{ {id, name, kind, label, sheet, row, col, width, height, link, value, options, script, function, hidden, listeners}, ... }",
			Params:  []Param{{Name: "sheet", Type: Ref, Desc: "feuille (absente : tout le classeur)", Optional: true}},
			Example: `for _, c in ipairs(controls.list()) do
  print(c.name, c.kind, c.sheet, c.value)
end`,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				var only int64
				if c.Has("sheet") {
					s, err := c.sheet("sheet")
					if err != nil {
						return nil, err
					}
					only = s.ID
				}
				list, err := d.Controls(c)
				if err != nil {
					return nil, err
				}
				names, err := sheetNames(c, d)
				if err != nil {
					return nil, err
				}
				out := []ControlView{}
				for _, b := range list {
					if only != 0 && b.Sheet != only {
						continue
					}
					v, err := describeControl(c, d, b, names)
					if err != nil {
						return nil, err
					}
					out = append(out, v)
				}
				return out, nil
			},
		},
		&Command{
			Name:    "controls.get",
			Summary: "Décrit un contrôle : genre, libellé, cellule liée et sa valeur, choix, couleurs, position, action et écouteurs.",
			Details: controlsHelp,
			Returns: "{id, name, kind, label, sheet, row, col, width, height, link, value, options, script, function, color, hoverColor, activeColor, textColor, hidden, listeners}",
			Params:  []Param{pControl},
			Example: `controls.add("checkbox", 1, 6, {name = "Urgent", label = "Urgent", link = "H1"})
local c = controls.get("Urgent")
print(c.kind, c.link, c.value)`,
			Run: func(c *Call) (any, error) {
				b, err := c.control("control")
				if err != nil {
					return nil, err
				}
				return c.view(b.ID)
			},
		},
		&Command{
			Name:    "controls.add",
			Label:   "Insérer un contrôle",
			Summary: "Pose un contrôle sur une feuille, coin haut gauche en row, col, et le renvoie décrit (son nom est choisi automatiquement s'il n'est pas donné).",
			Details: controlsHelp + " " + propsHelp + " Sans link, un contrôle autre qu'un bouton est lié à la cellule à sa droite.",
			Returns: "{id, name, kind, …} (comme controls.get)",
			Params: []Param{
				{Name: "kind", Type: String, Desc: `"button", "checkbox", "select", "input", "textarea" ou "file"`},
				pRow, pCol,
				{Name: "props", Type: Object, Desc: "propriétés : {label = …, link = \"B3\" (facultatif : sans cellule liée, la valeur est gardée dans le contrôle), options = {…} ou optionsFrom = \"Clients[Ville]\" (choix tirés d'une colonne), …}", Optional: true},
				pSheet,
			},
			Write: true,
			Example: `local l = controls.add("select", 1, 6, {options = {"Tous", "Payés", "Impayés"}, link = "H1", value = "Tous"})
print(l.name, l.value)`,
			Run: func(c *Call) (any, error) {
				s, err := c.sheet("sheet")
				if err != nil {
					return nil, err
				}
				row, err := c.Pos("row", 0)
				if err != nil {
					return nil, err
				}
				col, err := c.Pos("col", 0)
				if err != nil {
					return nil, err
				}
				k, _ := c.Str("kind")
				var cfg canvas.ButtonConfig
				if cfg.Kind, err = kindOf(k); err != nil {
					return nil, err
				}
				switch cfg.Kind {
				case "":
					cfg.Label = "Bouton"
				case "checkbox":
					cfg.Label = "Case à cocher"
				case "select":
					cfg.Options = []string{"Choix 1", "Choix 2"}
				case "file":
					cfg.Label = "Choisir un fichier…"
				}
				if cfg.Kind != "" {
					cfg.Link = formula.ColumnLetters(col+1) + strconv.Itoa(row+1)
				}
				props := map[string]any{}
				if c.Has("props") {
					if err := c.Decode("props", &props); err != nil {
						return nil, err
					}
				}
				e, err := controlProps(&cfg, props)
				if err != nil {
					return nil, err
				}
				if e.row >= 0 || e.col >= 0 {
					return nil, fmt.Errorf("props : la position se donne avec row et col")
				}
				if e.w >= 0 {
					cfg.Width = e.w
				}
				if e.h >= 0 {
					cfg.Height = e.h
				}
				d, _ := c.Doc()
				id, err := d.CreateButton(c, s.ID, row, col, cfg)
				if err != nil {
					return nil, err
				}
				if e.hasValue {
					b, err := d.Button(c, id)
					if err != nil {
						return nil, err
					}
					if err := c.setControlValue(b, e.value); err != nil {
						return nil, err
					}
				}
				return c.view(id)
			},
		},
		&Command{
			Name:    "controls.set",
			Label:   "Modifier le contrôle",
			Summary: "Change des propriétés d'un contrôle (les autres ne changent pas) : libellé, valeur, choix, couleurs, visibilité, position.",
			Details: propsHelp + " Écrire value revient à saisir la valeur dans la cellule liée.",
			Params:  []Param{pControl, {Name: "props", Type: Object, Desc: "propriétés à changer : {value = …, hidden = true, …}"}},
			Write:   true,
			Example: `controls.add("input", 1, 6, {name = "Recherche", label = "Client…"})
controls.set("Recherche", {value = "Durand", textColor = "#1f4e79"})
print(controls.get("Recherche").value)`,
			Run: func(c *Call) (any, error) {
				b, err := c.control("control")
				if err != nil {
					return nil, err
				}
				props := map[string]any{}
				if err := c.Decode("props", &props); err != nil {
					return nil, err
				}
				cfg := b.Config
				cfg.Options = append([]string(nil), b.Config.Options...)
				e, err := controlProps(&cfg, props)
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				if err := d.SetButton(c, b.ID, cfg); err != nil {
					return nil, err
				}
				if e.row >= 0 || e.col >= 0 || e.w >= 0 || e.h >= 0 {
					row, col, offX, offY, w, h := b.Row, b.Col, b.Config.OffX, b.Config.OffY, b.Config.Width, b.Config.Height
					if e.row >= 0 {
						row, offY = e.row, 0
					}
					if e.col >= 0 {
						col, offX = e.col, 0
					}
					if e.w >= 0 {
						w = e.w
					}
					if e.h >= 0 {
						h = e.h
					}
					if err := d.PlaceButton(c, b.ID, row, col, offX, offY, w, h); err != nil {
						return nil, err
					}
				}
				if e.hasValue {
					nb, err := d.Button(c, b.ID)
					if err != nil {
						return nil, err
					}
					if err := c.setControlValue(nb, e.value); err != nil {
						return nil, err
					}
				}
				return nil, nil
			},
		},
		&Command{
			Name:    "controls.delete",
			Label:   "Supprimer le contrôle",
			Summary: "Supprime un contrôle (sa cellule liée garde sa valeur).",
			Params:  []Param{pControl},
			Write:   true,
			Run: func(c *Call) (any, error) {
				b, err := c.control("control")
				if err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				return nil, d.DeleteButton(c, b.ID)
			},
		},
		&Command{
			Name:    "controls.on",
			Label:   "Écouter un contrôle",
			Summary: "Abonne une fonction d'un script à un évènement d'un contrôle (\"click\" d'un bouton, \"change\" des autres) ; elle sera appelée à chaque évènement avec {control, event, value, old}.",
			Details: eventsHelp + " handler est la fonction elle-même ou son nom ; script, par défaut le script en cours (il doit être enregistré dans la bibliothèque). Abonner deux fois la même fonction ne change rien.",
			Params: []Param{
				pControl,
				{Name: "event", Type: String, Desc: `"click" ou "change"`},
				{Name: "handler", Type: String, Desc: "fonction globale du script (ou son nom)"},
				{Name: "script", Type: String, Desc: "script de la fonction (absent : le script en cours)", Optional: true},
			},
			Write: true,
			Example: `scripts.save("Filtres", [==[
function filtrer(e)
  print(e.control .. " : " .. tostring(e.old) .. " → " .. tostring(e.value))
end
]==])
controls.add("select", 1, 6, {name = "Statut", options = {"Tous", "Payés"}, link = "H1"})
controls.on("Statut", "change", "filtrer", "Filtres")
controls.set("Statut", {value = "Payés"}) -- filtrer s'exécute à la fin du script`,
			Run: func(c *Call) (any, error) {
				b, err := c.control("control")
				if err != nil {
					return nil, err
				}
				l := canvas.Listener{}
				l.Event, _ = c.Str("event")
				l.Function, _ = c.Str("handler")
				l.Script, _ = c.Str("script")
				l.Event = strings.ToLower(strings.TrimSpace(l.Event))
				if l.Script == "" {
					l.Script = currentScript(c)
				}
				if l.Script == "" {
					return nil, fmt.Errorf("script : précisez le script de la fonction %s", l.Function)
				}
				if err := canvas.CheckEvent(b.Config.Kind, l.Event); err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				list, err := d.Scripts(c)
				if err != nil {
					return nil, err
				}
				found := false
				for _, s := range list {
					found = found || s.Name == l.Script
				}
				if !found {
					return nil, fmt.Errorf("script « %s » introuvable : enregistrez-le dans la bibliothèque (onglet SCRIPT)", l.Script)
				}
				cfg := b.Config
				for _, x := range cfg.Listeners {
					if x.Event == l.Event && x.Script == l.Script && x.Function == l.Function {
						return nil, nil
					}
				}
				cfg.Listeners = append(append([]canvas.Listener(nil), cfg.Listeners...), l)
				return nil, d.SetButton(c, b.ID, cfg)
			},
		},
		&Command{
			Name:    "controls.off",
			Label:   "Ne plus écouter un contrôle",
			Summary: "Désabonne des écouteurs d'un contrôle et renvoie leur nombre : tous, ou ceux d'un évènement, d'une fonction, d'un script.",
			Details: "L'action du contrôle (script et function, fenêtre Modifier) n'est pas un écouteur : controls.set(nom, {script = \"\"}) la retire.",
			Params: []Param{
				pControl,
				{Name: "event", Type: String, Desc: "évènement (absent : tous)", Optional: true},
				{Name: "handler", Type: String, Desc: "fonction (ou son nom ; absente : toutes)", Optional: true},
				{Name: "script", Type: String, Desc: "script (absent : tous)", Optional: true},
			},
			Write: true,
			Run: func(c *Call) (any, error) {
				b, err := c.control("control")
				if err != nil {
					return nil, err
				}
				event, _ := c.Str("event")
				fn, _ := c.Str("handler")
				sc, _ := c.Str("script")
				cfg := b.Config
				cfg.Listeners = nil
				n := 0
				for _, l := range b.Config.Listeners {
					if (event == "" || strings.EqualFold(l.Event, event)) && (fn == "" || l.Function == fn) && (sc == "" || l.Script == sc) {
						n++
						continue
					}
					cfg.Listeners = append(cfg.Listeners, l)
				}
				if n == 0 {
					return 0, nil
				}
				d, _ := c.Doc()
				return n, d.SetButton(c, b.ID, cfg)
			},
		},
		&Command{
			Name:    "controls.listeners",
			Summary: "Liste ce qu'exécutent les évènements d'un contrôle, ou de tous : son action (action = true), puis ses écouteurs, dans l'ordre.",
			Details: eventsHelp,
			Returns: "{ {control, event, script, function, action}, ... }",
			Params:  []Param{{Name: "control", Type: Ref, Desc: "contrôle (absent : tous)", Optional: true}},
			Example: `for _, l in ipairs(controls.listeners()) do
  print(l.control, l.event, l.script, l["function"])
end`,
			Run: func(c *Call) (any, error) {
				if c.Has("control") {
					b, err := c.control("control")
					if err != nil {
						return nil, err
					}
					return listenersOf(b), nil
				}
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				list, err := d.Controls(c)
				if err != nil {
					return nil, err
				}
				out := []ListenerView{}
				for _, b := range list {
					out = append(out, listenersOf(b)...)
				}
				return out, nil
			},
		},
		&Command{
			Name:    "controls.fire",
			Label:   "Déclencher un contrôle",
			Summary: "Déclenche un évènement d'un contrôle comme l'interface (clic d'un bouton, change avec la valeur actuelle des autres) : son action puis ses écouteurs s'exécutent.",
			Details: "Dans un script, l'évènement est traité à la fin du script.",
			Params:  []Param{pControl, {Name: "event", Type: String, Desc: `"click" ou "change" (absent : celui du contrôle)`, Optional: true}},
			Write:   true,
			Returns: "{output, writes, durationMs, error} (hors script)",
			Run: func(c *Call) (any, error) {
				b, err := c.control("control")
				if err != nil {
					return nil, err
				}
				event, _ := c.Str("event")
				if event == "" {
					event = canvas.DefaultEvent(b.Config.Kind)
				}
				if err := canvas.CheckEvent(b.Config.Kind, event); err != nil {
					return nil, err
				}
				d, _ := c.Doc()
				v, err := linkedValue(c, d, b)
				if err != nil {
					return nil, err
				}
				res, err := dispatch(c, c.Host, controlEvent{id: b.ID, event: event, value: v, old: v, hasOld: true})
				if err != nil {
					return nil, err
				}
				if currentScript(c) != "" {
					return nil, nil
				}
				return res, nil
			},
		},
	)
}
