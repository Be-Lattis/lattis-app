package api

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/script"
	"github.com/lalamefine/lattis/internal/transfer"
)

// QueryLimit borne les lignes renvoyées par db.query (serveur MCP).
const QueryLimit = 10_000

// readSQL exécute une requête de lecture avec les noms affichés (db.query,
// favorites.run) : pool de lecture en query_only, sous l'autorisateur des
// requêtes de l'utilisateur (ni écriture, ni ATTACH, ni PRAGMA qui
// modifie), dans le délai limits.sqlReadSeconds (grid.QueryArgs).
func (c *Call) readSQL(q string, params []any) (any, error) {
	d, err := c.Doc()
	if err != nil {
		return nil, err
	}
	if q, err = d.ReadCTE(c, q); err != nil {
		return nil, err
	}
	if q, err = script.Rewrite(c, d.DB().Read(), q); err != nil {
		return nil, err
	}
	res, err := grid.QueryArgs(c, d.DB().Read(), q, QueryLimit, params...)
	if err != nil {
		return nil, fmt.Errorf("%s", script.SQLError(err))
	}
	return res, nil
}

// DocState décrit le classeur ouvert.
type DocState struct {
	Open  bool   `json:"open"`
	Path  string `json:"path"`
	Title string `json:"title"`
	Dirty bool   `json:"dirty"`
	Undo  string `json:"undo"` // action que Annuler défera
	Redo  string `json:"redo"`
}

func (c *Call) state() (DocState, error) {
	d, err := c.Host.Doc()
	if err != nil {
		return DocState{}, nil
	}
	st := DocState{Open: true, Path: d.Path(), Title: msg.Format(c.lang(), "doc.untitled", nil)}
	if st.Path != "" {
		st.Title = strings.TrimSuffix(filepath.Base(st.Path), filepath.Ext(st.Path))
	}
	// Classeur ouvert depuis un serveur : son cache s'appelle
	// <identifiant>.lattis ; le titre est celui du serveur.
	if d.Share != nil {
		if info := d.Share.Info(); info != nil && info.Title != "" && strings.EqualFold(st.Title, info.WorkbookID) {
			st.Title = info.Title
		}
	}
	if st.Dirty, err = d.Dirty(c); err != nil {
		return st, err
	}
	h, err := d.Bus.State(c)
	st.Undo, st.Redo = h.Undo, h.Redo
	return st, err
}

func init() {
	register(
		// --- Classeur (MCP) -----------------------------------------------------
		&Command{
			Name:    "doc.state",
			Summary: "Décrit le classeur ouvert : fichier, modifications non enregistrées, actions à annuler et à rétablir.",
			Returns: "{open, path, title, dirty, undo, redo}",
			Run:     func(c *Call) (any, error) { return c.state() },
		},
		&Command{
			Name: "doc.undo", Summary: "Annule la dernière action (Ctrl+Z).", NoLua: true,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				if _, err := d.Bus.Undo(c); err != nil {
					return nil, err
				}
				c.Host.Changed(c)
				return c.state()
			},
		},
		&Command{
			Name: "doc.redo", Summary: "Rétablit la dernière action annulée (Ctrl+Y).", NoLua: true,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				if _, err := d.Bus.Redo(c); err != nil {
					return nil, err
				}
				c.Host.Changed(c)
				return c.state()
			},
		},
		&Command{
			Name: "doc.new", Summary: "Crée un classeur vide (le classeur ouvert est fermé ; ses modifications non enregistrées restent récupérables).", NoLua: true,
			Run: func(c *Call) (any, error) {
				if err := c.Host.NewDoc(c); err != nil {
					return nil, err
				}
				return c.state()
			},
		},
		&Command{
			Name: "doc.open", Summary: "Ouvre un classeur .lattis.", NoLua: true,
			Params: []Param{{Name: "path", Type: String, Desc: "chemin du fichier .lattis"}},
			Run: func(c *Call) (any, error) {
				p, _ := c.Str("path")
				if err := c.Host.OpenDoc(c, p); err != nil {
					return nil, err
				}
				return c.state()
			},
		},
		&Command{
			Name: "doc.save", Summary: "Enregistre le classeur (path : enregistrer sous ce fichier).", NoLua: true,
			Params: []Param{{Name: "path", Type: String, Desc: "chemin du fichier .lattis (absent : le fichier courant)", Optional: true}},
			Run: func(c *Call) (any, error) {
				p, _ := c.Str("path")
				if p != "" && !strings.HasSuffix(strings.ToLower(p), ".lattis") {
					p += ".lattis"
				}
				if err := c.Host.SaveDoc(c, p); err != nil {
					return nil, err
				}
				return c.state()
			},
		},
		&Command{
			Name:    "doc.theme",
			Summary: "Thème de couleur du classeur (Affichage › Thème) : couleur des tableaux ordinaires (table) et des tableaux d'une source externe (source). nil : thème par défaut des réglages.",
			Returns: "{id, name, table, source} ou nil",
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				return d.Theme(c)
			},
		},
		&Command{
			Name: "doc.set_theme", Label: "Thème", Write: true,
			Summary: "Change le thème de couleur du classeur : deux couleurs de base #rrggbb, les nuances (en-tête, texte, bordure) s'en déduisent. Sans couleurs : le thème par défaut des réglages.",
			Params: []Param{
				{Name: "table", Type: String, Desc: "couleur des tableaux ordinaires, #rrggbb", Optional: true},
				{Name: "source", Type: String, Desc: "couleur des tableaux d'une source externe (requête), #rrggbb", Optional: true},
				{Name: "name", Type: String, Desc: "nom du thème", Optional: true},
			},
			Example: `doc.set_theme("#2563eb", "#ea580c", "Bleu et orange")`,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				if !c.Has("table") && !c.Has("source") {
					return nil, d.SetTheme(c, nil)
				}
				table, _ := c.Str("table")
				source, _ := c.Str("source")
				name, _ := c.Str("name")
				return nil, d.SetTheme(c, &canvas.Theme{Name: name, Table: table, Source: source})
			},
		},
		&Command{
			Name: "doc.import", Label: "Importer", Summary: "Importe un fichier CSV ou un classeur Excel (.xlsx) dans de nouvelles feuilles.", NoLua: true, Write: true,
			Returns: "{sheets, tables, warnings}",
			Params:  []Param{{Name: "path", Type: String, Desc: "chemin du fichier .csv, .tsv, .txt ou .xlsx"}},
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				p, _ := c.Str("path")
				return transfer.ImportFile(c, d.Model, p)
			},
		},
		&Command{
			Name: "doc.export_csv", Summary: "Exporte un tableau en CSV (séparateur ;, virgule décimale, UTF-8 : ce qu'Excel en français ouvre directement).", NoLua: true,
			Params: []Param{pTable, {Name: "path", Type: String, Desc: "chemin du fichier .csv à écrire"}},
			Run: func(c *Call) (any, error) {
				t, err := c.table("table")
				if err != nil {
					return nil, err
				}
				p, _ := c.Str("path")
				d, _ := c.Doc()
				return p, transfer.ExportCSVFile(c, d.Model, t.Name, p, transfer.ExcelFR)
			},
		},
		&Command{
			Name: "doc.export_xlsx", Summary: "Exporte tout le classeur au format Excel .xlsx : tableaux Excel, formules traduites quand Excel a un équivalent (sinon leur valeur), mise en forme.", NoLua: true,
			Returns: "{path, clipped, report} (clipped : cellules au-delà des limites d'Excel, non exportées ; report : ce qui n'est pas exporté tel quel, voir Details)",
			Details: exportReportDetails,
			Params:  []Param{{Name: "path", Type: String, Desc: "chemin du fichier .xlsx à écrire"}},
			Run: func(c *Call) (any, error) {
				return exportBook(c, ".xlsx", transfer.ExportXLSXFile)
			},
		},
		&Command{
			Name: "doc.export_ods", Summary: "Exporte tout le classeur au format OpenDocument .ods (LibreOffice) : formules traduites quand c'est possible (sinon leur valeur), mise en forme, tableaux en plages nommées.", NoLua: true,
			Returns: "{path, clipped, report}",
			Details: exportReportDetails,
			Params:  []Param{{Name: "path", Type: String, Desc: "chemin du fichier .ods à écrire"}},
			Run: func(c *Call) (any, error) {
				return exportBook(c, ".ods", transfer.ExportODSFile)
			},
		},
		&Command{
			Name: "doc.recalc", Label: "Recalculer", Summary: "Renouvelle AUJOURDHUI, MAINTENANT et ALEA (F9).", Write: true,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				d.Recalc()
				return nil, nil
			},
		},
		&Command{
			Name:    "formulas.functions",
			Summary: "Liste les fonctions des formules (noms français), avec leur syntaxe et leur description.",
			Run:     func(c *Call) (any, error) { return formula.Functions(formula.FR), nil },
		},

		// --- SQL (MCP ; les scripts ont db.query et db.execute) -----------------
		&Command{
			Name:    "db.query",
			Summary: "Exécute une requête SQL de lecture (SQLite) avec les noms affichés des tableaux et des colonnes ; renvoie au plus 10 000 lignes.",
			Details: "Un nom avec espace, accent ou majuscule se met entre guillemets doubles (\"Montant HT\"). Chaque tableau a une colonne cachée _id, la clé de la ligne ; les tableaux croisés s'interrogent comme des tableaux. Les ? prennent les valeurs de params.",
			Returns: "{columns, rows, truncated}",
			Params:  []Param{{Name: "sql", Type: String, Desc: "requête SELECT ou WITH"}, {Name: "params", Type: Array, Desc: "valeurs des ?", Optional: true}},
			NoLua:   true,
			Run: func(c *Call) (any, error) {
				q, _ := c.Str("sql")
				var params []any
				if err := c.Decode("params", &params); err != nil {
					return nil, err
				}
				return c.readSQL(q, params)
			},
		},
		&Command{
			Name:    "db.execute",
			Label:   "Requête SQL",
			Summary: "Exécute une requête d'écriture (INSERT, UPDATE, DELETE) avec les noms affichés ; annulable, elle renvoie le nombre de lignes touchées.",
			Details: "Les changements de structure (CREATE, ALTER, DROP) sont refusés : utilisez les commandes tables.* et columns.*.",
			Params:  []Param{{Name: "sql", Type: String, Desc: "requête INSERT, UPDATE ou DELETE"}, {Name: "params", Type: Array, Desc: "valeurs des ?", Optional: true}},
			Write:   true,
			NoLua:   true,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				q, _ := c.Str("sql")
				var params []any
				if err := c.Decode("params", &params); err != nil {
					return nil, err
				}
				return d.ExecSQL(c, q, params...)
			},
		},

		// --- Bibliothèque de scripts ---------------------------------------------
		&Command{
			Name:    "scripts.list",
			Summary: "Liste les scripts enregistrés dans le classeur, avec leur source.",
			Returns: "{ {id, name, source}, ... }",
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				return d.Scripts(c)
			},
		},
		&Command{
			Name:    "scripts.save",
			Label:   "Enregistrer le script",
			Summary: "Enregistre un script dans la bibliothèque du classeur (le remplace s'il existe déjà sous ce nom).",
			Params:  []Param{{Name: "name", Type: String, Desc: "nom du script"}, {Name: "source", Type: String, Desc: "code Lua"}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				d, err := c.Doc()
				if err != nil {
					return nil, err
				}
				name, _ := c.Str("name")
				src, _ := c.Str("source")
				id, err := scriptID(c, name)
				if err != nil {
					return nil, err
				}
				return d.SaveScript(c, id, name, src)
			},
		},
		&Command{
			Name:    "scripts.delete",
			Label:   "Supprimer le script",
			Summary: "Supprime un script de la bibliothèque.",
			Params:  []Param{{Name: "name", Type: String, Desc: "nom du script"}},
			Write:   true,
			Run: func(c *Call) (any, error) {
				name, _ := c.Str("name")
				id, err := scriptID(c, name)
				if err != nil {
					return nil, err
				}
				if id == 0 {
					return nil, msg.New("api.scriptNotFound", msg.Args{"script": name})
				}
				d, _ := c.Doc()
				return nil, d.DeleteScript(c, id)
			},
		},
		&Command{
			Name:    "scripts.check",
			Summary: "Vérifie la syntaxe d'un code Lua sans l'exécuter ; renvoie \"\" s'il est correct, sinon l'erreur.",
			Params:  []Param{{Name: "source", Type: String, Desc: "code Lua"}},
			Run: func(c *Call) (any, error) {
				src, _ := c.Str("source")
				if err := script.Check(src); err != nil {
					return err.Error(), nil
				}
				return "", nil
			},
		},
		&Command{
			Name:    "scripts.run",
			Summary: "Exécute un script Lua : un script enregistré (name) ou du code (source), et éventuellement l'une de ses fonctions. Tout ce qu'il modifie forme une seule étape d'annulation.",
			Details: "Le script a accès à db.* et à toutes les commandes (sheets, tables, ui…) ; print écrit dans output. Durée maximale : 30 secondes, hors boîtes de dialogue.",
			Returns: "{output, writes, durationMs, error}",
			Params: []Param{
				{Name: "name", Type: String, Desc: "nom d'un script enregistré", Optional: true},
				{Name: "source", Type: String, Desc: "code Lua à exécuter (à la place de name)", Optional: true},
				{Name: "function", Type: String, Desc: "fonction globale à appeler après le chargement", Optional: true},
			},
			NoLua: true,
			Run: func(c *Call) (any, error) {
				name, _ := c.Str("name")
				src, _ := c.Str("source")
				fn, _ := c.Str("function")
				if (name == "") == (src == "") {
					return nil, msg.New("api.nameOrSource", nil)
				}
				if name != "" {
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
						if s.Name == name {
							src, found = s.Source, true
						}
					}
					if !found {
						return nil, msg.New("api.scriptNotFound", msg.Args{"script": name})
					}
				}
				return RunScriptFrom(c, c.Host, name, src, fn)
			},
		},
	)
}

func scriptID(c *Call, name string) (int64, error) {
	d, err := c.Doc()
	if err != nil {
		return 0, err
	}
	list, err := d.Scripts(c)
	if err != nil {
		return 0, err
	}
	for _, s := range list {
		if s.Name == name {
			return s.ID, nil
		}
	}
	return 0, nil
}

// exportBook écrit tout le classeur (doc.export_xlsx, doc.export_ods).
// exportReportDetails décrit le compte rendu d'un export .xlsx ou .ods.
const exportReportDetails = "report : {sheets, scripts, clipped} ; scripts : scripts du classeur, non exportés. Chaque feuille de sheets : name, kind (\"html\" ou \"doc\" : onglet page ou document, exporté vide), tables (tableaux exportés), translated (formules exportées comme formules), formulas et columns (formules de cellules et colonnes calculées exportées par leur valeur), functions (fonctions sans équivalent qui en sont la cause, noms français), pivots (tableaux croisés exportés en valeurs), charts (graphiques), rules (règles de mise en forme conditionnelle), notes, lists (validations par liste), buttons (boutons et contrôles) : non exportés ; queries (tableaux chargés par une requête : valeurs sans la requête), twoLevel (en-têtes sur deux lignes, exportés sur une), protected (protection non exportée), clipped."

func exportBook(c *Call, ext string, export func(context.Context, *canvas.Model, string) (transfer.ExportReport, error)) (any, error) {
	d, err := c.Doc()
	if err != nil {
		return nil, err
	}
	p, _ := c.Str("path")
	if strings.TrimSpace(p) == "" {
		return nil, msg.New("api.pathExpected", msg.Args{"ext": ext})
	}
	if !strings.EqualFold(filepath.Ext(p), ext) {
		p += ext
	}
	rep, err := export(c, d.Model, p)
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": p, "clipped": rep.Clipped, "report": rep}, nil
}

var _ canvas.ScriptInfo
