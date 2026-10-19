package api

import (
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Commandes des évènements du classeur (events.go).

// WorkbookListenerView est un écouteur d'un évènement du classeur.
type WorkbookListenerView struct {
	Event    string `json:"event"`
	Script   string `json:"script"`
	Function string `json:"function"`
}

func workbookEventsHelp() string {
	var ls []string
	for _, e := range WorkbookEvents {
		ls = append(ls, `"`+e.Name+`" (`+e.Desc+`)`)
	}
	return "Évènements : " + strings.Join(ls, " ; ") + ". " +
		"Chaque table porte aussi event (le nom de l'évènement) ; lignes et colonnes se comptent à partir de 1, comme ui.selection. " +
		"value et old (valeur affichée après et avant) ne sont donnés que pour une seule cellule saisie. " +
		"Seules les actions de l'utilisateur déclenchent ces évènements : ce qu'un script (écouteur compris) modifie, sélectionne ou affiche n'en déclenche aucun, si bien qu'un écouteur ne se relance jamais lui-même. " +
		"Un écouteur est une fonction globale d'un script enregistré de la bibliothèque, appelée avec la table de l'évènement (aussi dans la variable globale event) ; il est enregistré dans le classeur et survit au script. " +
		"Les écouteurs d'un évènement s'exécutent dans l'ordre où ils ont été abonnés ; tout ce qu'ils modifient s'annule d'un coup (Ctrl+Z). " +
		"Un écouteur en erreur est signalé une fois dans la barre d'état et la console, sans bloquer l'interface. " +
		"before_save et before_close ont 5 secondes au plus (hors boîtes de dialogue) ; une erreur ne bloque ni l'enregistrement ni la fermeture."
}

func init() {
	register(
		&Command{
			Name:    "events.on",
			Label:   "Écouter un évènement du classeur",
			Summary: "Abonne une fonction d'un script à un évènement du classeur (ouverture, enregistrement, fermeture, changement d'onglet, de sélection, saisie…) ; elle sera appelée à chaque évènement avec une table qui le décrit.",
			Details: workbookEventsHelp() + " handler est la fonction elle-même ou son nom ; script, par défaut le script en cours (il doit être enregistré dans la bibliothèque). Abonner deux fois la même fonction ne change rien.",
			Params: []Param{
				{Name: "event", Type: String, Desc: `"open", "before_save", "save", "before_close", "sheet", "selection", "change", "sheet_add", "sheet_delete" ou "recalc"`},
				{Name: "handler", Type: String, Desc: "fonction globale du script (ou son nom)"},
				{Name: "script", Type: String, Desc: "script de la fonction (absent : le script en cours)", Optional: true},
			},
			Write:   true,
			Returns: "true si l'écouteur est nouveau",
			Example: `scripts.save("Évènements", [==[
function saisie(e)
  -- e.sheet, e.address, e.value, e.old : la cellule saisie
  if e.value ~= nil then
    print(e.address .. " : " .. tostring(e.old) .. " → " .. tostring(e.value))
  end
end
function avant_fermeture(e)
  return ui.confirm("Fermer le classeur ?") -- false : la fermeture est annulée
end
]==])
events.on("change", "saisie", "Évènements")
events.on("before_close", "avant_fermeture", "Évènements")`,
			Run: func(c *Call) (any, error) {
				l := canvas.Listener{}
				l.Event, _ = c.Str("event")
				l.Function, _ = c.Str("handler")
				l.Script, _ = c.Str("script")
				l.Event = strings.ToLower(strings.TrimSpace(l.Event))
				l.Function = strings.TrimSpace(l.Function)
				if err := CheckWorkbookEvent(l.Event); err != nil {
					return nil, err
				}
				if l.Function == "" {
					return nil, fmt.Errorf("handler : précisez la fonction")
				}
				if l.Script == "" {
					l.Script = currentScript(c)
				}
				if l.Script == "" {
					return nil, fmt.Errorf("script : précisez le script de la fonction %s", l.Function)
				}
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				list, err := d.Scripts(c)
				if err != nil {
					return nil, err
				}
				found := false
				for _, s := range list {
					if strings.EqualFold(s.Name, l.Script) {
						l.Script, found = s.Name, true
					}
				}
				if !found {
					return nil, fmt.Errorf("script « %s » introuvable : enregistrez-le dans la bibliothèque (onglet SCRIPT)", l.Script)
				}
				ls, err := d.WorkbookListeners(c)
				if err != nil {
					return nil, err
				}
				for _, x := range ls {
					if x.Event == l.Event && strings.EqualFold(x.Script, l.Script) && x.Function == l.Function {
						return false, nil
					}
				}
				return true, d.SetWorkbookListeners(c, append(ls, l))
			},
		},
		&Command{
			Name:    "events.off",
			Label:   "Ne plus écouter un évènement du classeur",
			Summary: "Désabonne des écouteurs des évènements du classeur et renvoie leur nombre : tous, ou ceux d'un évènement, d'une fonction, d'un script.",
			Params: []Param{
				{Name: "event", Type: String, Desc: "évènement (absent : tous)", Optional: true},
				{Name: "handler", Type: String, Desc: "fonction (ou son nom ; absente : toutes)", Optional: true},
				{Name: "script", Type: String, Desc: "script (absent : tous)", Optional: true},
			},
			Write:   true,
			Returns: "nombre d'écouteurs retirés",
			Example: `events.off("selection") -- plus rien n'écoute la sélection`,
			Run: func(c *Call) (any, error) {
				event, _ := c.Str("event")
				fn, _ := c.Str("handler")
				sc, _ := c.Str("script")
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				ls, err := d.WorkbookListeners(c)
				if err != nil {
					return nil, err
				}
				var keep []canvas.Listener
				n := 0
				for _, l := range ls {
					if (event == "" || strings.EqualFold(l.Event, strings.TrimSpace(event))) && (fn == "" || l.Function == fn) && (sc == "" || strings.EqualFold(l.Script, sc)) {
						n++
						continue
					}
					keep = append(keep, l)
				}
				if n == 0 {
					return 0, nil
				}
				return n, d.SetWorkbookListeners(c, keep)
			},
		},
		&Command{
			Name:    "events.list",
			Summary: "Liste les écouteurs des évènements du classeur, dans l'ordre où ils s'exécutent.",
			Details: workbookEventsHelp(),
			Returns: "{ {event, script, function}, ... }",
			Params:  []Param{{Name: "event", Type: String, Desc: "évènement (absent : tous)", Optional: true}},
			Example: `for _, l in ipairs(events.list()) do
  print(l.event, l.script, l["function"])
end`,
			Run: func(c *Call) (any, error) {
				event, _ := c.Str("event")
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				ls, err := d.WorkbookListeners(c)
				if err != nil {
					return nil, err
				}
				out := []WorkbookListenerView{}
				for _, l := range ls {
					if event == "" || strings.EqualFold(l.Event, strings.TrimSpace(event)) {
						out = append(out, WorkbookListenerView{Event: l.Event, Script: l.Script, Function: l.Function})
					}
				}
				return out, nil
			},
		},
		&Command{
			Name:    "events.enable",
			Summary: "Active (true) ou coupe (false) les évènements du classeur pour la session, comme Application.EnableEvents d'Excel ; renvoie l'état précédent.",
			Details: "Coupés, aucun écouteur des évènements du classeur ne s'exécute ; les évènements des contrôles (controls.on) ne sont pas concernés. Ouvrir un classeur les réactive.",
			Params:  []Param{{Name: "on", Type: Boolean, Desc: "true : activer ; false : couper"}},
			Returns: "true si les évènements étaient activés",
			Example: `local avant = events.enable(false)
-- … travail sans déclencher d'écouteur …
events.enable(avant)`,
			Run: func(c *Call) (any, error) {
				on, err := c.Bool("on")
				if err != nil {
					return nil, err
				}
				return SetEventsEnabled(on), nil
			},
		},
		&Command{
			Name:    "events.enabled",
			Summary: "Dit si les évènements du classeur sont activés (events.enable).",
			Returns: "booléen",
			Run:     func(c *Call) (any, error) { return EventsEnabled(), nil },
		},
	)
}
