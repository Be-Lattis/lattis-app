package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/script"
	"github.com/lalamefine/lattis/internal/trace"
)

// Évènements du classeur, l'équivalent de Workbook_Open, Worksheet_Change,
// SelectionChange, SheetActivate, BeforeSave ou BeforeClose d'Excel.
//
// Un écouteur (events.on) est une fonction globale d'un script de la
// bibliothèque, enregistrée dans le classeur ; elle est appelée avec une
// table qui décrit l'évènement (aussi dans la variable globale event).
//
// Seules les actions de l'utilisateur dans l'interface déclenchent ces
// évènements (internal/bridge) : ce qu'un script modifie, sélectionne ou
// enregistre n'en déclenche aucun, et un écouteur ne peut donc pas se
// relancer lui-même (comme Application.EnableEvents = False pendant une
// macro Excel). Les écouteurs d'un évènement tournent dans le bac à sable
// habituel ; tout ce qu'ils modifient forme une seule étape d'annulation.
// Un écouteur en erreur est signalé une fois (barre d'état et console) puis
// ignoré, sans jamais bloquer l'interface ; avant l'enregistrement ou la
// fermeture, il ne bloque pas non plus l'action.

// Noms des évènements du classeur.
const (
	EventOpen        = "open"         // classeur ouvert (et affiché)
	EventBeforeSave  = "before_save"  // avant l'enregistrement : false l'annule
	EventSave        = "save"         // après l'enregistrement
	EventBeforeClose = "before_close" // avant la fermeture : false l'annule
	EventSheet       = "sheet"        // onglet affiché changé
	EventSelection   = "selection"    // sélection changée (après une pause)
	EventChange      = "change"       // cellules modifiées par l'utilisateur
	EventSheetAdd    = "sheet_add"    // feuille ajoutée
	EventSheetDelete = "sheet_delete" // feuille supprimée
	EventRecalc      = "recalc"       // recalcul demandé (F9)
)

// WorkbookEvents décrit les évènements du classeur, dans l'ordre de la
// documentation.
var WorkbookEvents = []struct{ Name, Desc string }{
	{EventOpen, "classeur ouvert : {path, title}"},
	{EventBeforeSave, "avant l'enregistrement (Ctrl+S, Enregistrer sous) : {path, as} ; renvoyer false l'annule"},
	{EventSave, "après l'enregistrement : {path}"},
	{EventBeforeClose, "avant la fermeture du classeur ou de la fenêtre : {path, dirty} ; renvoyer false l'annule"},
	{EventSheet, "onglet affiché changé : {sheet, sheet_id, previous, previous_id}"},
	{EventSelection, "sélection changée, une fois la souris ou le clavier au repos : {sheet, sheet_id, row, col, row2, col2, address, table, column}"},
	{EventChange, "cellules modifiées par l'utilisateur (saisie, collage, effacement, déplacement, ligne de tableau ajoutée ou supprimée) : {sheet, sheet_id, row, col, row2, col2, address, table, column, value, old, source}"},
	{EventSheetAdd, "feuille ajoutée (ou copiée) : {sheet, sheet_id}"},
	{EventSheetDelete, "feuille supprimée : {sheet, sheet_id}"},
	{EventRecalc, "formules recalculées (F9, Recalculer) : {}"},
}

// CheckWorkbookEvent vérifie un nom d'évènement du classeur.
func CheckWorkbookEvent(event string) error {
	var names []string
	for _, e := range WorkbookEvents {
		if e.Name == event {
			return nil
		}
		names = append(names, `"`+e.Name+`"`)
	}
	return msg.New("api.unknownEvent", msg.Args{"event": event, "events": strings.Join(names, ", ")})
}

// BeforeTimeout borne un écouteur before_save ou before_close (hors boîtes
// de dialogue) : l'enregistrement ou la fermeture n'attendent pas plus.
var BeforeTimeout = 5 * time.Second

// État des évènements pour l'application : un seul classeur est ouvert.
var events struct {
	mu       sync.Mutex
	disabled bool
	reported map[string]bool // erreurs déjà signalées
}

// EventsEnabled dit si les évènements du classeur sont déclenchés.
func EventsEnabled() bool {
	events.mu.Lock()
	defer events.mu.Unlock()
	return !events.disabled
}

// SetEventsEnabled active ou coupe les évènements du classeur et renvoie
// l'état précédent.
func SetEventsEnabled(on bool) bool {
	events.mu.Lock()
	defer events.mu.Unlock()
	was := !events.disabled
	events.disabled = !on
	return was
}

// ResetEvents remet les évènements à zéro pour un classeur qui s'ouvre : ils
// sont réactivés et les erreurs déjà signalées peuvent l'être de nouveau.
func ResetEvents() {
	events.mu.Lock()
	defer events.mu.Unlock()
	events.disabled = false
	events.reported = nil
}

// firstReport dit si l'erreur key n'a pas encore été signalée (et la retient).
func firstReport(key string) bool {
	events.mu.Lock()
	defer events.mu.Unlock()
	if events.reported[key] {
		return false
	}
	if events.reported == nil {
		events.reported = map[string]bool{}
	}
	events.reported[key] = true
	return true
}

// listenersOfEvent donne les écouteurs d'un évènement du classeur ; aucun
// si les évènements sont coupés.
func listenersOfEvent(ctx context.Context, d *doc.Document, event string) ([]canvas.Listener, error) {
	if !EventsEnabled() {
		return nil, nil
	}
	all, err := d.WorkbookListeners(ctx)
	if err != nil {
		return nil, err
	}
	var out []canvas.Listener
	for _, l := range all {
		if l.Event == event {
			out = append(out, l)
		}
	}
	return out, nil
}

// Listening dit si un évènement du classeur a des écouteurs (et si les
// évènements sont activés) : l'interface ne prépare la description d'un
// évènement que dans ce cas.
func Listening(host Host, event string) bool {
	d, err := host.Doc()
	if err != nil {
		return false
	}
	ls, err := listenersOfEvent(contextBG, d, event)
	return err == nil && len(ls) > 0
}

// EventResult est l'issue des écouteurs d'un évènement du classeur.
type EventResult struct {
	ScriptResult
	// Ran : nombre d'écouteurs exécutés.
	Ran int `json:"ran"`
	// Cancelled : un écouteur a renvoyé false (before_save, before_close).
	Cancelled bool `json:"cancelled"`
	// Skipped : un script tournait déjà, l'évènement n'a pas été traité.
	Skipped bool `json:"skipped"`
}

// FireEvent exécute les écouteurs d'un évènement du classeur, l'un après
// l'autre, dans une seule étape d'annulation. data décrit l'évènement ; son
// champ event est posé ici. Un écouteur en erreur arrête les suivants, et
// son erreur est signalée une fois par session (barre d'état et console).
// Un évènement qui survient pendant un script n'est pas traité (Skipped).
func FireEvent(host Host, event string, data map[string]any) (EventResult, error) {
	res := EventResult{ScriptResult: ScriptResult{Output: []string{}}}
	if err := CheckWorkbookEvent(event); err != nil {
		return res, err
	}
	d, err := host.Doc()
	if err != nil {
		return res, err
	}
	ls, err := listenersOfEvent(contextBG, d, event)
	if err != nil || len(ls) == 0 {
		return res, err
	}
	if Busy() {
		trace.Logf(1, trace.Event, "%s ignoré : un script tourne déjà", event)
		res.Skipped = true
		return res, nil
	}
	trace.Logf(1, trace.Event, "%s : %d écouteur(s)", event, len(ls))
	if trace.On(3, trace.Event) {
		trace.Logf(3, trace.Event, "%s : données %v", event, data)
	}
	list, err := d.Scripts(contextBG)
	if err != nil {
		return res, err
	}
	arg := map[string]any{}
	for k, v := range data {
		arg[k] = v
	}
	arg["event"] = event
	lim := script.Defaults()
	if event == EventBeforeSave || event == EventBeforeClose {
		lim.Timeout = BeforeTimeout
	}
	ctx, q := withQueue(command.WithGroup(context.Background(), "Évènement « "+event+" »"))
	var failed canvas.Listener
	for _, l := range ls {
		src, found := "", false
		for _, s := range list {
			if strings.EqualFold(s.Name, l.Script) {
				src, found = s.Source, true
				break
			}
		}
		if !found {
			res.Error = fmt.Sprintf("script « %s » introuvable", l.Script)
			failed = l
			break
		}
		r, ret, err := runLimited(ctx, host, l.Script, src, l.Function, arg, lim)
		if errors.Is(err, ErrBusy) {
			res.Skipped = true
			return res, nil
		}
		res.add(r)
		res.Ran++
		if err != nil {
			return res, err
		}
		if r.Error != "" {
			failed = l
			break
		}
		if b, ok := ret.(bool); ok && !b {
			res.Cancelled = true
			break
		}
	}
	if res.Error == "" {
		if err := drain(ctx, host, q, &res.ScriptResult); err != nil {
			return res, err
		}
	}
	if res.Error != "" {
		msg := fmt.Sprintf("Évènement « %s » : erreur de %s.%s : %s", event, failed.Script, failed.Function, res.Error)
		if failed.Script == "" {
			msg = fmt.Sprintf("Évènement « %s » : %s", event, res.Error)
		}
		if firstReport(event + "\x00" + failed.Script + "\x00" + failed.Function + "\x00" + res.Error) {
			host.Print(msg)
			host.Notify(msg)
		}
	}
	return res, nil
}

// --- Description des évènements ----------------------------------------------

// Address écrit une plage à partir de 0 : « B3 », ou « B3:D5 ».
func Address(r0, c0, r1, c1 int) string {
	a := formula.ColumnLetters(c0) + strconv.Itoa(r0+1)
	if r1 != r0 || c1 != c0 {
		a += ":" + formula.ColumnLetters(c1) + strconv.Itoa(r1+1)
	}
	return a
}

// sheetName donne le nom d'une feuille ("" si elle n'existe pas).
func sheetName(ctx context.Context, d *doc.Document, sheet int64) string {
	names, err := sheetNames(ctx, d)
	if err != nil {
		return ""
	}
	return names[sheet]
}

// SheetEventData décrit une feuille pour un évènement : {sheet, sheet_id}.
func SheetEventData(host Host, sheet int64) map[string]any {
	d, err := host.Doc()
	if err != nil {
		return map[string]any{"sheet_id": sheet}
	}
	return map[string]any{"sheet": sheetName(contextBG, d, sheet), "sheet_id": sheet}
}

// RangeEventData décrit une plage d'une feuille (à partir de 0) pour un
// évènement : {sheet, sheet_id, row, col, row2, col2, address, table,
// column}, positions à partir de 1 comme ui.selection ; table et column
// désignent le tableau et la colonne de la cellule en haut à gauche.
func RangeEventData(host Host, sheet int64, r0, c0, r1, c1 int) map[string]any {
	r0, r1 = min(r0, r1), max(r0, r1)
	c0, c1 = min(c0, c1), max(c0, c1)
	out := map[string]any{
		"sheet_id": sheet, "row": r0 + 1, "col": c0 + 1, "row2": r1 + 1, "col2": c1 + 1,
		"address": Address(r0, c0, r1, c1),
	}
	d, err := host.Doc()
	if err != nil {
		return out
	}
	out["sheet"] = sheetName(contextBG, d, sheet)
	if l, err := d.Layout(contextBG, sheet); err == nil {
		if t := tableAt(l, r0, c0); t != nil {
			out["table"], out["column"] = t.Name, t.Columns[c0-t.Col].Name
		}
	}
	return out
}

// TableRowEventData décrit la ligne de données n (à partir de 0) d'un
// tableau, sur toute sa largeur, avec line (à partir de 1) ; nil si le
// tableau est introuvable.
func TableRowEventData(host Host, table string, n int) map[string]any {
	c := &Call{Context: contextBG, Host: host}
	t, err := findTable(contextBG, c, table)
	if err != nil {
		return nil
	}
	row := t.Row + 1 + n
	out := RangeEventData(host, t.Sheet.ID, row, t.Col, row, t.Col+max(len(t.Columns), 1)-1)
	out["table"], out["line"] = t.Name, n+1
	delete(out, "column")
	return out
}

// CellAt lit une cellule de la feuille (à partir de 0), comme cells.get.
func CellAt(host Host, sheet int64, row, col int) (CellValue, error) {
	d, err := host.Doc()
	if err != nil {
		return CellValue{}, err
	}
	l, err := d.Layout(contextBG, sheet)
	if err != nil {
		return CellValue{}, err
	}
	return readCell(contextBG, d, l, row, col)
}
