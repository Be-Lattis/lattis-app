package bridge

import (
	"sync/atomic"
	"time"

	"github.com/lalamefine/lattis/internal/msg"
)

// Fin d'une démo publique (contrat de lattisd, docs/collaboration/version-web.md
// § 6.2) : lattisd donne au moteur l'heure de fin de la démo
// (LATTIS_DEMO_DEADLINE) et le préavis (LATTIS_DEMO_WARN_BEFORE). L'interface
// prévient avant la fin (bandeau, compte à rebours) puis, à l'heure, bloque
// le classeur ; le moteur refuse alors toute modification (DemoGuard), seuls
// la lecture et le téléchargement restent permis. lattisd arrête le moteur
// peu après.

// demoEnd : heure de fin et préavis de la démo publique ; nil : pas de fin
// annoncée (lattisd plus ancien, ou hors démo).
type demoEnd struct {
	deadline time.Time
	warn     time.Duration
}

var demoEndAt atomic.Pointer[demoEnd]

// demoNow : l'horloge (remplacée par les tests).
var demoNow = time.Now

// errDemoEnded refuse une modification après la fin de la démo.
var errDemoEnded = msg.New("demo.ended", nil)

// SetDemoDeadline fixe la fin de la démo publique et son préavis ; une heure
// nulle l'efface.
func SetDemoDeadline(deadline time.Time, warn time.Duration) {
	if deadline.IsZero() {
		demoEndAt.Store(nil)
		return
	}
	if warn < 0 {
		warn = 0
	}
	demoEndAt.Store(&demoEnd{deadline: deadline, warn: warn})
}

// DemoEnded dit si la fin de la démo publique est passée.
func DemoEnded() bool {
	e := demoEndAt.Load()
	return e != nil && !demoNow().Before(e.deadline)
}

// demoAfterEnd : méthodes encore permises après la fin de la démo (lecture
// de l'état et des données, vue, enregistrement qui télécharge, exports) ;
// toute autre est refusée (liste fermée : une méthode nouvelle est refusée
// tant qu'elle n'y est pas ajoutée).
var demoAfterEnd = map[string]bool{}

func init() {
	for _, m := range []string{
		// État, données, aide.
		"Platform", "State", "Window", "Layout", "Settings", "About", "Recent", "RangeStats", "Find",
		"ChartData", "PivotData", "QueryData", "Queries", "Connections", "ColumnOrder", "Citations",
		"ValidationList", "FilterValues", "FieldValues", "Dependencies", "FormulaHelp", "CustomLists",
		"TypeMix", "PageBreaks", "PrintHTML", "CoEditState", "ShareState", "LattisState", "AIState",
		"MCPState", "Favorites", "Scripts", "HTTPHosts", "TableSources", "WorkbookTables", "ExcelTables",
		"SaveProgress", "DemoProgress", "DemoRequested", "WorkbookReady", "FirstLaunch", "Orphans",
		"CloseRequestSeen", "BeforeClose", "ReportError", "DialogAnswer", "SetLanguage", "Query",
		// Vue : feuille affichée, sélection (le zoom, gardé dans le
		// classeur, est une modification).
		"SetActiveSheet", "SetSelection",
		// Enregistrer (télécharge le classeur) et exports.
		"Save", "Download", "ResolveSave", "ExportXLSX", "ExportODS", "ExportCSV", "ExportSheetCSV",
	} {
		demoAfterEnd["DocumentService."+m] = true
	}
	for _, m := range []string{"Window", "Query", "Status", "BenchRequested"} {
		demoAfterEnd["GridService."+m] = true
	}
}

// demoPlatform complète Platform de la fin annoncée de la démo : heures en
// millisecondes depuis 1970 (UTC), et l'heure du moteur (Now) pour que
// l'interface corrige l'écart de son horloge.
func demoPlatform(p *Platform) {
	e := demoEndAt.Load()
	if e == nil {
		return
	}
	p.DemoDeadline = e.deadline.UnixMilli()
	p.DemoWarnBefore = e.warn.Milliseconds()
	p.Now = demoNow().UnixMilli()
}

// DemoGuard refuse, après la fin de la démo publique, les appels qui
// modifieraient le classeur ; method est le nom court (« DocumentService.SetCell »).
// Sans fin annoncée ou avant l'heure : nil.
func DemoGuard(method string) error {
	if !DemoEnded() || demoAfterEnd[method] {
		return nil
	}
	return errDemoEnded
}
