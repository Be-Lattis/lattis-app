<script lang="ts">
	// Classeur façon Excel : ruban en haut, feuille (ou onglet SQL / SCRIPT) au
	// centre, onglets de feuilles et barre d'état en bas.
	//
	// Deux modes : un classeur (DocumentService : saisie libre, tableaux qui
	// naissent, annuler, enregistrer) et la démo d'1 M de lignes (GridService),
	// qui sert aussi à la mesure automatique (LATTIS_BENCH).
	import { onMount, tick, untrack } from 'svelte';
	import { noticeTtl } from '#lib/shell/notice.js';
	import { Events } from '@wailsio/runtime';
	import { DocumentService, GridService } from '#bindings/internal/bridge/index.js';
	import type { DocState, FormulaHelp } from '#bindings/internal/bridge/models.js';
	import type { Layout, RuleChanges } from '#bindings/internal/canvas/models.js';
	import type { Type } from '#bindings/internal/grid/models.js';
	import type { Orphan } from '#bindings/internal/archive/models.js';
	import Sheet, { type SheetData, type SheetSource, type SheetStatus, type ColumnFilter, type PivotConfig, type ChartConfig, type TypeMix } from '#lib/grid/Sheet.svelte';
	import Ribbon, { type DocCommand, type FileAction, type View } from '#lib/shell/Ribbon.svelte';
	import SheetTabs from '#lib/shell/SheetTabs.svelte';
	import { visibleSheet } from '#lib/shell/sheets.js';
	import { count } from '#lib/sql/result.js';
	import { printSheet } from '#lib/shell/print.js';
	import SqlView, { type Favorite } from '#lib/shell/SqlView.svelte';
	import TitleBar from '#lib/shell/TitleBar.svelte';
	import ScriptView, { type Script } from '#lib/script/ScriptView.svelte';
	import ScriptDialog, { type DialogSpec } from '#lib/script/ScriptDialog.svelte';
	import McpPanel from '#lib/shell/McpPanel.svelte';
	import HtmlPage from '#lib/page/HtmlPage.svelte';
	import DocPage from '#lib/doc/DocPage.svelte';
	import type { DocFormat } from '#lib/doc/format.js';
	import QueryHost from '#lib/query/QueryHost.svelte';
	import type { ButtonConfig } from '#lib/grid/buttons.js';
	import type { UICommand } from '#bindings/internal/bridge/models.js';
	import { typeLabel, type ColumnType } from '#lib/grid/format.js';
	import type { Fetched } from '#lib/grid/cache.js';
	import { shortcutOf } from '#lib/grid/shortcuts.js';
	import { clampZoom } from '#lib/grid/zoom.js';
	import ZoomBar from '#lib/shell/ZoomBar.svelte';
	import BusyBar from '#lib/shell/BusyBar.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { setTileFetcher } from '#lib/chart/tiles.js';

	type Mode = 'starting' | 'recover' | 'doc' | 'demo' | 'closed';

	let mode = $state<Mode>('starting');
	let bench = $state(false);
	let view = $state<View>('grid');
	let failure = $state('');
	let notice = $state('');
	// Message de la barre d'état : effacé seul au bout d'un moment (noticeTtl),
	// et à chaque changement d'onglet.
	let noticeAt = 0;
	$effect(() => {
		const m = notice;
		if (!m) return;
		noticeAt = Date.now();
		const t = setTimeout(() => {
			if (notice === m) notice = '';
		}, noticeTtl(m));
		return () => clearTimeout(t);
	});
	$effect(() => {
		void sheetId;
		void view;
		untrack(() => {
			// Un message donné juste avant d'ouvrir l'onglet (« Résultat copié
			// dans… ») reste ; les anciens partent.
			if (Date.now() - noticeAt > 1500) notice = '';
			sheet?.clearMessage();
		});
	});
	let sheetStatus = $state<SheetStatus | null>(null);
	let sheet = $state<ReturnType<typeof Sheet>>();

	// Classeur.
	let docState = $state<DocState>({ open: false, path: '', title: '', dirty: false, undo: '', redo: '' });
	let layout = $state<Layout | null>(null);
	let sheetId = $state(1);
	// Défilement horizontal de la grille, affiché dans la ligne des onglets.
	let hscroll = $state<{ content: number; view: number; left: number } | undefined>();
	// Les adresses Cell_A1 de l'onglet SQL se lisent sur la feuille affichée.
	$effect(() => {
		if (mode === 'doc') DocumentService.SetActiveSheet(sheetId).catch(() => {});
	});
	let orphans = $state<Orphan[]>([]);
	let help = $state<FormulaHelp | null>(null);
	let scripts = $state<Script[]>([]);
	// Contenu de l'onglet page HTML ou Document affiché (null : une feuille).
	let pageSource = $state<{ sheet: number; source: string } | null>(null);
	// Onglet Document affiché : son éditeur, et la mise en forme à la sélection (ruban).
	let docPage = $state<ReturnType<typeof DocPage>>();
	let docFmt = $state<DocFormat | null>(null);
	// Requêtes favorites de l'onglet SQL, enregistrées dans le classeur.
	let favorites = $state<Favorite[]>([]);
	let recent = $state<string[]>([]);
	// Fermer un classeur modifié : enregistrer d'abord ?
	let closing = $state(false);
	// Boîtes de dialogue ouvertes par un script ou un assistant (ui.dialog…), la plus ancienne d'abord.
	let dialogs = $state<DialogSpec[]>([]);
	let mcpOpen = $state(false);
	// Données › Obtenir des données : requêtes d'import (lib/query).
	let queryHost = $state<ReturnType<typeof QueryHost>>();
	// Révision › Protéger : boîte du mot de passe (protéger, ou ôter une
	// protection qui en a un).
	// Suppression d'une feuille qui contient quelque chose : confirmée avec la liste de ce qui part.
	let deleting = $state<{ id: number; name: string; items: string[] } | null>(null);
	let protecting = $state<{ what: 'sheet' | 'workbook'; id: number; name: string; off: boolean } | null>(null);
	let pw1 = $state('');
	let pw2 = $state('');
	let pwError = $state('');

	// Démo.
	let demo = $state<{ name: string; rows: number; columns: { name: string; type: ColumnType }[]; sqlName: string; sqlColumns: string[] } | null>(null);
	let demoProgress = $state('');

	const err = (e: unknown) => String(e instanceof Error ? e.message : e);

	async function reload() {
		docState = await DocumentService.State();
		layout = await DocumentService.Layout(sheetId);
		// Feuille affichée masquée ou disparue : la première feuille visible.
		const shownId = visibleSheet(layout?.sheets ?? [], sheetId);
		if (layout && shownId !== sheetId) {
			sheetId = shownId;
			layout = await DocumentService.Layout(sheetId);
		}
		// Onglet page HTML ou Document affiché : son contenu (la page se recharge s'il change).
		const shown = layout?.sheets?.find((s) => s.id === sheetId);
		if (shown?.kind === 'html' || shown?.kind === 'doc') {
			const src = await DocumentService.Page(sheetId);
			if (!pageSource || pageSource.sheet !== sheetId || pageSource.source !== src) pageSource = { sheet: sheetId, source: src };
		} else pageSource = null;
		help = await DocumentService.FormulaHelp();
		scripts = (await DocumentService.Scripts()) ?? [];
		favorites = (await DocumentService.Favorites()) ?? [];
	}

	const loadRecent = async () => (recent = (await DocumentService.Recent().catch(() => null)) ?? []);

	async function openPath(path: string) {
		const r = await DocumentService.Open(path);
		if (r.recoverable) {
			orphans = [r.recoverable];
			mode = 'recover';
		} else if (!r.cancelled) {
			view = 'grid';
			sheetId = 1;
			mode = 'doc';
			await reload();
		}
		await loadRecent();
	}

	/** Ferme le classeur ; modifié, il demande d'abord s'il faut l'enregistrer. */
	async function closeDoc(save?: boolean) {
		closing = false;
		if (mode === 'doc' && docState.dirty && save === undefined) {
			closing = true;
			return;
		}
		if (save) {
			docState = await DocumentService.Save(false);
			if (docState.dirty) return; // enregistrement abandonné
		}
		sheet?.closeOverlays();
		await DocumentService.CloseDocument();
		docState = { open: false, path: '', title: '', dirty: false, undo: '', redo: '' };
		layout = null;
		sheetStatus = null;
		view = 'grid';
		mode = 'closed';
		await loadRecent();
	}

	async function startDoc() {
		mode = 'doc';
		view = 'grid';
		sheetId = 1;
		await DocumentService.New();
		await reload();
	}

	/** Ouvre le classeur de démonstration demandé au lancement (--demo preset). */
	async function startPresetDemo(preset: string) {
		let waiting = true;
		const poll = async () => {
			if (!waiting) return;
			const p = await DocumentService.DemoProgress().catch(() => null);
			if (p && waiting)
				demoProgress =
					`Démo « ${preset} » : ${p.done.toLocaleString('fr-FR')} / ${p.total.toLocaleString('fr-FR')} lignes` +
					(p.totalCells ? ` · ${p.cells.toLocaleString('fr-FR')} / ${p.totalCells.toLocaleString('fr-FR')} cellules` : '');
			setTimeout(poll, 250);
		};
		poll();
		try {
			const r = await DocumentService.OpenDemo();
			view = 'grid';
			sheetId = r.sheet;
			mode = 'doc';
			await reload();
		} finally {
			waiting = false;
			demoProgress = '';
		}
	}

	async function startDemo() {
		mode = 'demo';
		const poll = async () => {
			const s = await GridService.Status();
			if (s.state === 'ready') {
				demo = {
					name: s.info.name,
					rows: s.info.rows,
					columns: (s.info.columns ?? []).map((c) => ({ name: c.name, type: c.type as ColumnType })),
					sqlName: s.info.sqlName,
					sqlColumns: s.info.sqlColumns ?? []
				};
			} else if (s.state === 'error') {
				failure = s.error ?? 'erreur';
			} else {
				demoProgress = s.state === 'generating' ? `Création de la démo : ${s.done.toLocaleString('fr-FR')} / ${s.total.toLocaleString('fr-FR')} lignes` : 'Ouverture de la démo…';
				setTimeout(poll, 250);
			}
		};
		poll();
	}

	// Pas de menu natif du navigateur (Retour, Actualiser, Inspecter…) : les
	// zones qui ont un menu ouvrent le leur ; les champs de saisie gardent le
	// menu du système (couper, copier, coller).
	function blockNativeMenu(e: MouseEvent) {
		const t = e.target as HTMLElement | null;
		if (t?.closest('input, textarea, [contenteditable="true"], .cm-editor')) return;
		e.preventDefault();
	}

	/** La feuille affichée a disparu (classeur ouvert par un assistant, feuille supprimée) : première feuille. */
	async function keepSheet() {
		if (layout && visibleSheet(layout.sheets ?? [], sheetId) !== sheetId) {
			sheetId = visibleSheet(layout.sheets ?? [], sheetId);
			await reload();
		}
	}

	/** Commandes de l'interface venues d'un script ou du serveur MCP (internal/bridge/host.go). */
	async function uiCommand(c: UICommand) {
		switch (c.kind) {
			case 'select': {
				const sel = c.selection;
				if (!sel || mode !== 'doc') return;
				if (view !== 'grid') {
					sheet?.closeOverlays();
					view = 'grid';
				}
				if (sel.sheet && sel.sheet !== sheetId) {
					sheet?.closeOverlays();
					sheetId = sel.sheet;
				}
				await reload();
				await tick();
				await new Promise((r) => requestAnimationFrame(r));
				sheet?.selectRange(sel.r0, sel.c0, sel.r1, sel.c1);
				break;
			}
			case 'view':
				if (c.view === 'grid' || c.view === 'sql' || c.view === 'script') {
					if (c.view !== view) sheet?.closeOverlays();
					view = c.view;
				}
				break;
			case 'notify':
				notice = c.message ?? '';
				break;
			case 'dialog':
				if (c.dialog) dialogs = [...dialogs, c.dialog as DialogSpec];
				break;
			case 'dialog-close':
				dialogs = dialogs.filter((d) => d.id !== c.id);
				break;
		}
	}

	function answerDialog(d: DialogSpec, values: Record<string, unknown> | null) {
		dialogs = dialogs.filter((x) => x.id !== d.id);
		DocumentService.DialogAnswer(d.id, values ?? {}, values !== null).catch((e) => (notice = err(e)));
		if (!dialogs.length) sheet?.focus();
	}

	// Feuille affichée, sélection et vue, lues par ui.selection (scripts, MCP).
	let lastSel = '';
	$effect(() => {
		if (mode !== 'doc') return;
		const s = sheetStatus?.sel ?? { r0: 0, c0: 0, r1: 0, c1: 0 };
		const sel = { sheet: sheetId, r0: s.r0, c0: s.c0, r1: s.r1, c1: s.c1, view };
		const k = JSON.stringify(sel);
		if (k === lastSel) return;
		lastSel = k;
		DocumentService.SetSelection(sel).catch(() => {});
	});

	// Tuiles des cartes : par le classeur, soumises à l'autorisation de l'hôte.
	setTileFetcher((z, x, y) => DocumentService.Call('charts.tile', { z, x, y }) as Promise<string>);

	onMount(() => {
		const off = Events.On('doc:changed', () => {
			if (mode === 'doc') reload().then(keepSheet);
		});
		const offUI = Events.On('ui:command', (e: { data: UICommand }) => {
			uiCommand(e.data).catch((x) => (notice = err(x)));
		});
		(async () => {
			try {
				bench = await GridService.BenchRequested();
				if (bench) return startDemo();
				loadRecent();
				const preset = await DocumentService.DemoRequested();
				if (preset) return await startPresetDemo(preset);
				orphans = (await DocumentService.Orphans()) ?? [];
				if (orphans.length > 0) mode = 'recover';
				else await startDoc();
			} catch (e) {
				failure = err(e);
			}
		})();
		const keys = (e: KeyboardEvent) => {
			if (e.defaultPrevented) return; // déjà traité (éditeur de script : Ctrl+S)
			const ctrl = e.ctrlKey || e.metaKey;
			const action: FileAction | null =
				e.key === 'F12' ? 'saveAs' : ctrl && e.key.toLowerCase() === 's' ? 'save' : ctrl && e.key.toLowerCase() === 'o' ? 'open' : ctrl && e.key.toLowerCase() === 'n' ? 'new' : ctrl && e.key.toLowerCase() === 'w' ? 'close' : ctrl && e.key.toLowerCase() === 'p' && mode === 'doc' ? 'print' : null;
			if (action) {
				e.preventDefault();
				file(action);
				return;
			}
			// Onglets : Ctrl+Pg.Suiv / Ctrl+Pg.Préc (feuille suivante, précédente), Maj+F11 (insérer une feuille).
			const sc = view === 'grid' && !protecting ? shortcutOf(e) : null;
			if (sc === 'nextSheet' || sc === 'prevSheet') {
				e.preventDefault();
				const k = tabs.findIndex((t) => t.id === sheetId) + (sc === 'nextSheet' ? 1 : -1);
				if (k >= 0 && k < tabs.length) selectSheet(tabs[k].id);
			} else if (sc === 'insertSheet' && mode === 'doc') {
				e.preventDefault();
				if (layout?.workbookProtected) notice = 'La structure du classeur est protégée : ôtez la protection pour insérer une feuille';
				else sheetAction(async () => selectSheet(await DocumentService.InsertSheet(sheetId)));
			}
		};
		window.addEventListener('keydown', keys);
		// Un texte ou un lien lâché hors d'une zone prévue ne doit jamais faire
		// quitter la page de l'application (la fenêtre afficherait une erreur).
		const noDrop = (e: DragEvent) => e.preventDefault();
		window.addEventListener('dragover', noDrop);
		window.addEventListener('drop', noDrop);
		return () => {
			off();
			offUI();
			window.removeEventListener('keydown', keys);
			window.removeEventListener('dragover', noDrop);
			window.removeEventListener('drop', noDrop);
		};
	});

	async function file(a: FileAction) {
		notice = '';
		try {
			switch (a) {
				case 'new':
					await startDoc();
					break;
				case 'open':
					await openPath('');
					break;
				case 'close':
					await closeDoc();
					break;
				case 'save':
				case 'saveAs':
					if (mode !== 'doc') return;
					docState = await DocumentService.Save(a === 'saveAs');
					if (!docState.dirty) notice = `Enregistré : ${docState.path}`;
					await loadRecent();
					break;
				case 'import': {
					if (mode !== 'doc') await startDoc();
					const r = await DocumentService.Import('');
					if (r.cancelled) break;
					if (r.sheet) sheetId = r.sheet;
					await reload();
					const tables = r.tables ?? [];
					const what = tables.length === 1 ? `Tableau « ${tables[0]} » importé` : count(tables.length, 'tableau importé', 'tableaux importés');
					const warnings = r.warnings ?? [];
					notice = warnings.length ? `${what} ; ${warnings.length} point(s) à vérifier : ${warnings.join(' · ')}` : what;
					break;
				}
				case 'export': {
					if (mode !== 'doc') return;
					const table = sheet?.activeTable() ?? layout?.tables?.[0]?.name;
					if (!table) {
						notice = 'Aucun tableau à exporter sur cette feuille';
						break;
					}
					const p = await DocumentService.ExportCSV(table);
					if (p) notice = `Tableau « ${table} » exporté : ${p}`;
					break;
				}
				case 'exportXlsx': {
					if (mode !== 'doc') return;
					const r = await DocumentService.ExportXLSX();
					if (r.path)
						notice =
							`Classeur exporté (valeurs des formules) : ${r.path}` +
							(r.clipped ? ` ; ${r.clipped.toLocaleString('fr-FR')} cellule(s) au-delà des limites d'Excel (1 048 576 lignes × 16 384 colonnes) non exportée(s)` : '');
					break;
				}
				case 'print':
				case 'pdf': {
					// Feuille active, plage utilisée ; le PDF passe par la fenêtre
					// d'impression (imprimante « Microsoft Print to PDF »).
					if (mode !== 'doc') return;
					if (activeDoc && docPage) {
						if (a === 'pdf') exportDocPdf();
						else docPage.print(`${docState.title} – ${activeInfo?.name ?? ''}`);
						break;
					}
					const name = layout?.sheets?.find((s) => s.id === sheetId)?.name ?? '';
					if (a === 'pdf') notice = 'Exporter en PDF : choisissez l’imprimante « Microsoft Print to PDF » (ou « Enregistrer au format PDF »)';
					printSheet(await DocumentService.PrintHTML(sheetId), `${docState.title} – ${name}`);
					break;
				}
				case 'demo':
					await startDemo();
					break;
				case 'mcp':
					mcpOpen = true;
					break;
			}
		} catch (e) {
			notice = err(e);
		}
		sheet?.focus();
	}

	async function recover(o: Orphan, keep: boolean) {
		try {
			if (keep) {
				await DocumentService.Recover(o.dir);
				mode = 'doc';
				await reload();
				notice = 'Modifications récupérées : pensez à enregistrer.';
			} else {
				await DocumentService.Discard(o.dir);
				orphans = orphans.filter((x) => x.dir !== o.dir);
				if (orphans.length === 0) await startDoc();
			}
		} catch (e) {
			failure = err(e);
		}
	}

	// --- Sources de la feuille ----------------------------------------------

	const docSource: SheetSource = {
		window: (t, first, count) => DocumentService.Window(sheetId, t.id, first, count) as Promise<Fetched>,
		async setCell(row, col, raw) {
			const ev = await DocumentService.SetCell(sheetId, row, col, raw);
			// Relire tout de suite (l'événement doc:changed sert aux changements venus
			// d'ailleurs, et n'existe pas en mode serveur).
			await reload();
			if (ev.keyed) return `Clé posée sur « ${ev.keyed.slice(ev.keyed.indexOf('.') + 1)} » pour REFERENCE`;
			if (ev.created) return `Tableau « ${ev.created} » créé`;
			if (ev.widened) return `Colonne ${ev.widened} passée de ${typeLabel(ev.from as ColumnType)} à ${typeLabel(ev.to as ColumnType)} (Ctrl+Z pour annuler)`;
			if (ev.invalid) return 'Valeur hors type : cellule marquée en erreur';
		},
		undo: () => DocumentService.Undo().then(reload),
		redo: () => DocumentService.Redo().then(reload),
		async paste(row, col, block) {
			const ev = await DocumentService.Paste(sheetId, row, col, block);
			await reload();
			if (ev.created && ev.headless) return `Tableau « ${ev.created} » créé sans en-têtes : clic droit › « Utiliser la 1re ligne comme en-têtes » si besoin`;
			if (ev.created) return `Tableau « ${ev.created} » créé (${count(block.length - 1)})`;
		},
		rangeStats: (t, first, count, cols) => DocumentService.RangeStats(sheetId, t.id, first, count, cols),
		moveCells: (r0, c0, r1, c1, dr, dc) => DocumentService.MoveCells(sheetId, r0, c0, r1, c1, dr, dc).then(reload),
		clearCells: (r0, c0, r1, c1) => DocumentService.ClearCells(sheetId, r0, c0, r1, c1).then(reload),
		clearRange: (r0, c0, r1, c1, all) => DocumentService.ClearRange(sheetId, r0, c0, r1, c1, all).then(reload),
		async transfer(from, src, dst, move, values) {
			const ev = await DocumentService.Transfer(from, src, sheetId, dst, move, values);
			await reload();
			if (ev.created) return `Tableau « ${ev.created} » créé`;
			if (ev.widened) return `Colonne ${ev.widened} passée de ${typeLabel(ev.from as ColumnType)} à ${typeLabel(ev.to as ColumnType)} (Ctrl+Z pour annuler)`;
		},
		sheetLines: (op, a, b) =>
			(op === 'insertRows'
				? DocumentService.InsertSheetRows(sheetId, a, b)
				: op === 'deleteRows'
					? DocumentService.DeleteSheetRows(sheetId, a, b)
					: op === 'insertCols'
						? DocumentService.InsertSheetColumns(sheetId, a, b)
						: DocumentService.DeleteSheetColumns(sheetId, a, b)
			).then(reload),
		insertRow: (t, n) => DocumentService.InsertRow(t.name, n).then(reload),
		deleteRow: (t, n) => DocumentService.DeleteRow(t.name, n).then(reload),
		insertColumn: (t, i) => DocumentService.InsertColumn(t.name, i, `Colonne ${t.columns.length + 1}`).then(reload),
		deleteColumn: (t, i) => DocumentService.DeleteColumn(t.name, i).then(reload),
		convertPreview: (t, i, to) => DocumentService.ConvertPreview(t.name, i, to as Type),
		typeMix: (t, i) => DocumentService.TypeMix(t.name, i) as Promise<TypeMix>,
		setType: (t, i, to) => DocumentService.SetType(t.name, i, to as Type).then(reload),
		setColumnWidth: (col, width) => DocumentService.SetColumnWidth(sheetId, col, width).then(reload),
		setRowHeight: (row, height) => DocumentService.SetRowHeight(sheetId, row, height).then(reload),
		setName: (row, col, name) => DocumentService.SetName(sheetId, row, col, name).then(reload),
		sortTable: (t, i, desc) => DocumentService.SortTable(t.name, i, desc).then(reload),
		refreshTable: (t) => queryHost?.refreshOne(t.query!.id) ?? Promise.resolve(),
		setRules: (t, i, rules) => DocumentService.SetRules(t.name, i, rules).then(reload),
		setLineRules: (rows, i0, i1, rules) => DocumentService.SetLineRules(sheetId, rows, i0, i1, rules).then(reload),
		setSheetRules: (ch) => DocumentService.SetSheetRules(sheetId, ch as RuleChanges).then(reload),
		ruleStats: (t, cols, req) => DocumentService.RuleStats(sheetId, t.id, cols, req),
		filterValues: async (t, i) => (await DocumentService.FilterValues(t.name, i)) ?? [],
		setFilter: (t, i, f) => DocumentService.SetFilter(t.name, i, f).then(reload),
		clearFilters: (t) => DocumentService.ClearFilters(t.name).then(reload),
		renameTable: (t, name) => DocumentService.RenameTable(t.name, name).then(reload),
		promoteHeader: (t) => DocumentService.PromoteHeader(t.name).then(reload),
		keyCheck: (t, i) => DocumentService.KeyCheck(t.name, i),
		setKey: (t, i) => DocumentService.SetKey(t.name, i).then(reload),
		clearKey: (t) => DocumentService.ClearKey(t.name).then(reload),
		recalc: () => DocumentService.Recalc().then(reload),
		dependencies: async (row, col) => {
			const d = await DocumentService.Dependencies(sheetId, row, col);
			return { subject: d.subject, precedents: d.precedents ?? [], dependents: d.dependents ?? [] };
		},
		createPivot: async (cfg) => {
			const id = await DocumentService.CreatePivot(cfg);
			await reload();
			await selectSheet(id);
		},
		setPivot: (id, cfg) => DocumentService.SetPivot(id, cfg).then(reload),
		pivotFieldValues: async (table, field) => (await DocumentService.FieldValues(table, field)) ?? [],
		createChart: (row, col, cfg) => DocumentService.CreateChart(sheetId, row, col, cfg).then(reload),
		setChart: (id, cfg) => DocumentService.SetChart(id, cfg).then(reload),
		moveChart: (id, row, col) => DocumentService.MoveChart(id, row, col).then(reload),
		placeChart: (id, row, col, offX, offY, w, h) => DocumentService.PlaceChart(id, row, col, offX, offY, w, h).then(reload),
		deleteChart: (id) => DocumentService.DeleteChart(id).then(reload),
		scripts: async () => (await DocumentService.Scripts()) ?? [],
		runButton: async (id) => {
			const r = await DocumentService.RunButton(id);
			if (r.error) return `Erreur du script : ${r.error}`;
			const out = r.output ?? [];
			return out.length ? out[out.length - 1] : '';
		},
		fireControl: async (id, event, value, old) => {
			const r = await DocumentService.FireControl(id, event, value, old);
			if (r.error) return `Erreur du script : ${r.error}`;
			const out = r.output ?? [];
			return out.length ? out[out.length - 1] : '';
		},
		pickFile: (title) => DocumentService.PickControlFile(title),
		createButton: (row, col, cfg: ButtonConfig) => DocumentService.CreateButton(sheetId, row, col, cfg).then(async (id) => (await reload(), id)),
		setButton: (id, cfg: ButtonConfig) => DocumentService.SetButton(id, cfg).then(reload),
		placeButton: (id, row, col, offX, offY, w, h) => DocumentService.PlaceButton(id, row, col, offX, offY, w, h).then(reload),
		deleteButton: (id) => DocumentService.DeleteButton(id).then(reload),
		deletePivot: (id) => DocumentService.DeletePivot(id).then(reload),
		duplicatesPreview: (t, cols) => DocumentService.DuplicatesPreview(t.name, cols),
		removeDuplicates: async (t, cols) => {
			const n = await DocumentService.RemoveDuplicates(t.name, cols);
			await reload();
			return n;
		},
		textColumnsPreview: (r0, r1, col, o) => DocumentService.TextToColumnsPreview(sheetId, r0, r1, col, o),
		textToColumns: (r0, r1, col, o) => DocumentService.TextToColumns(sheetId, r0, r1, col, o).then((n) => {
			// L'assistant se ferme tout de suite ; la feuille se recharge ensuite.
			reload().catch((e) => (notice = err(e)));
			return n;
		}),
		deleteTables: (ts) => DocumentService.DeleteTables(ts.map((t) => t.name)).then(reload),
		moveTable: (t, row, col) => DocumentService.MoveTable(t.name, row, col).then(reload),
		extractColumns: (t, i0, i1, row, col) => DocumentService.ExtractColumns(t.name, i0, i1, row, col).then(reload),
		applyStyle: (r0, c0, r1, c1, patch) => DocumentService.ApplyStyle(sheetId, r0, c0, r1, c1, patch).then(reload),
		applyLineStyle: (rows, i0, i1, patch) => DocumentService.ApplyLineStyle(sheetId, rows, i0, i1, patch).then(reload),
		setNote: (row, col, text) => DocumentService.SetNote(sheetId, row, col, text).then(reload),
		pinNote: (row, col, pinned) => DocumentService.PinNote(sheetId, row, col, pinned).then(reload),
		mergeCells: async (r0, c0, r1, c1) => {
			const n = await DocumentService.MergeCells(sheetId, r0, c0, r1, c1);
			await reload();
			return n;
		},
		unmergeCells: (r0, c0, r1, c1) => DocumentService.UnmergeCells(sheetId, r0, c0, r1, c1).then(reload),
		setLinesHidden: (rows, i0, i1, hide) => DocumentService.SetLinesHidden(sheetId, rows, i0, i1, hide).then(reload),
		setFreeze: (rows, cols) => DocumentService.SetFreeze(sheetId, rows, cols).then(reload),
		find: async (text) => (await DocumentService.Find(sheetId, text)) ?? [],
		replace: async (text, by) => {
			const n = await DocumentService.Replace(sheetId, text, by);
			await reload();
			return n;
		}
	};

	const demoSource: SheetSource = {
		window: (_t, first, count) => GridService.Window(first, count) as Promise<Fetched>,
		async setCell(_row, _col, raw, target) {
			if (target.rowId === undefined || target.column === undefined) return 'Démo : seules les cellules du tableau sont modifiables';
			// La démo ne relit pas sa mise en page : la feuille met la cellule à jour.
			return { value: await GridService.SetCell(target.rowId, target.column, raw) };
		}
	};

	const docData = $derived<SheetData | null>(
		layout
			? {
					tables: (layout.tables ?? []).map((t) => {
						const filter = t.filter ? (JSON.parse(t.filter) as Record<string, ColumnFilter>) : {};
						return {
						id: t.id,
						name: t.name,
						row: t.row,
						col: t.col,
						rows: t.rows,
						hidden: t.total - t.rows,
						key: t.key >= 0 ? t.key : undefined,
						query: t.queryId ? { id: t.queryId, name: t.queryName } : undefined,
						columns: (t.columns ?? []).map((c) => ({
							name: c.name,
							type: (c.formula ? 'formula' : c.type) as ColumnType,
							fixed: c.fixed,
							errors: c.errors,
							formula: c.formula || undefined,
							result: c.result || undefined,
							style: c.style || undefined,
							headerStyle: c.headerStyle || undefined,
							rules: c.rules || undefined,
							filter: filter[String(c.id)]
						}))
					};
					}),
					sheet: sheetId,
					cells: (layout.cells ?? []).map((c) => ({ ...c, spill: c.spill?.map((r) => r ?? []) })),
					widths: layout.widths ?? [],
					heights: layout.heights ?? [],
					names: layout.names ?? [],
					styles: layout.styles ?? [],
					rowStyles: layout.rowStyles ?? [],
					colStyles: layout.colStyles ?? [],
					rowRules: layout.rowRules ?? [],
					colRules: layout.colRules ?? [],
					ruleMarks: layout.ruleMarks ?? [],
					charts: (layout.charts ?? []).map((c) => ({
						...c,
						config: c.config as ChartConfig,
						labels: c.labels ?? [],
						series: (c.series ?? []).map((s) => ({ name: s.name, values: (s.values ?? []).map((v) => v ?? null) }))
					})),
					pivots: (layout.pivots ?? []).map((p) => ({ ...p, config: p.config as PivotConfig, cells: (p.cells ?? []).map((r) => r ?? []) })),
					buttons: (layout.buttons ?? []).map((b) => ({ ...b, config: b.config as ButtonConfig }))
				}
			: null
	);

	const demoData = $derived<SheetData | null>(demo ? { tables: [{ id: 1, name: demo.name, row: 0, col: 0, rows: demo.rows, columns: demo.columns }], cells: [] } : null);

	const objects = $derived(
		mode === 'demo' && demo
			? [{ name: demo.name, sqlName: demo.sqlName, rows: demo.rows, columns: demo.columns.map((c, i) => ({ ...c, sql: demo!.sqlColumns[i] ?? '' })) }]
			: // Tableaux et tableaux croisés (vues) de toutes les feuilles.
				(help?.tables ?? []).map((t) => ({
					name: t.name,
					sqlName: t.sql,
					rows: t.rows,
					kind: t.kind === 'pivot' ? ('pivot' as const) : ('table' as const),
					key: t.key >= 0 ? t.key : undefined,
					columns: (t.columns ?? []).map((c, i) => ({ name: c, type: (t.types?.[i] ?? 'text') as ColumnType, sql: t.columnSql?.[i] ?? c }))
				}))
	);

	const query = (sql: string) => (mode === 'demo' ? GridService.Query(sql) : DocumentService.Query(sql));
	const data = $derived(mode === 'demo' ? demoData : docData);
	const title = $derived(mode === 'demo' ? 'Démo : 1 M de lignes' : mode === 'recover' ? 'Récupération' : docState.title);
	const tabs = $derived(mode === 'demo' ? (demo ? [{ id: 1, name: demo.name }] : []) : (layout?.sheets ?? []).map((s) => ({ id: s.id, name: s.name, protected: s.protected, color: s.color, hidden: s.hidden, kind: s.kind })));

	/** « Afficher sur la feuille » : va sur la feuille de l'objet et le sélectionne. */
	async function gotoObject(name: string) {
		view = 'grid';
		const t = (help?.tables ?? []).find((x) => x.name.toLowerCase() === name.toLowerCase());
		if (t && t.sheet && t.sheet !== sheetId) {
			sheet?.closeOverlays();
			sheetId = t.sheet;
			await reload();
		}
		await tick();
		await new Promise((r) => requestAnimationFrame(r));
		sheet?.revealObject(name);
		sheet?.focus();
	}

	async function selectSheet(id: number) {
		view = 'grid';
		if (mode !== 'doc' || id === sheetId) return;
		sheet?.closeOverlays();
		sheetId = id;
		await reload();
		sheet?.focus();
	}

	// --- Protection (Révision) ----------------------------------------------

	const activeInfo = $derived(layout?.sheets?.find((s) => s.id === sheetId));
	// Onglet page HTML ou Document affiché à la place de la grille.
	const activePage = $derived(mode === 'doc' && (activeInfo?.kind === 'html' || activeInfo?.kind === 'doc') && pageSource?.sheet === sheetId ? pageSource : null);
	const activeDoc = $derived(!!activePage && activeInfo?.kind === 'doc');
	const addPage = (before = 0) => sheetAction(async () => selectSheet(await DocumentService.AddPage(before)));
	const addDoc = (before = 0) => sheetAction(async () => selectSheet(await DocumentService.AddDoc(before)));

	// Onglet Document en PDF : la fenêtre d'impression, imprimante « Microsoft Print to PDF ».
	function exportDocPdf() {
		notice = 'Exporter en PDF : choisissez l’imprimante « Microsoft Print to PDF » (ou « Enregistrer au format PDF »)';
		docPage?.print(`${docState.title} – ${activeInfo?.name ?? ''}`);
	}

	// Commandes du ruban pour l'onglet Document.
	function docCommand(c: DocCommand) {
		const d = docPage;
		if (!d) return;
		if (c.startsWith('block:')) d.setBlock(c.slice(6) as never);
		else if (c.startsWith('align:')) d.setAlign(c.slice(6) as never);
		else if (c.startsWith('chart:')) d.insertChart(c.slice(6));
		else if (c === 'button') d.insertButton();
		else if (c === 'pivot') d.insertPivot();
		else if (c === 'pdf') exportDocPdf();
		else if (c === 'input') d.insertControl('input');
		else d.command(c as never);
	}

	// Zoom de la feuille active (Affichage › Zoom, Ctrl+molette, barre d'état) :
	// appliqué tout de suite, enregistré avec la feuille un instant après (le
	// curseur et la molette en envoient beaucoup).
	// Affichage › Animations : préférence de l'utilisateur, gardée sur ce poste.
	let animate = $state(
		(() => {
			try {
				return localStorage.getItem('lattis.animations') !== 'off';
			} catch {
				return true;
			}
		})()
	);
	function setAnimate(on: boolean) {
		animate = on;
		try {
			localStorage.setItem('lattis.animations', on ? 'on' : 'off');
		} catch {
			// stockage indisponible : le choix vaut pour la session
		}
	}

	let zoomSet = $state<Record<string, number>>({});
	const docKey = $derived(`${mode}|${docState.open}|${docState.path}|${docState.title}`);
	const zoomKey = $derived(`${docKey}|${sheetId}`);
	const zoom = $derived(zoomSet[zoomKey] ?? clampZoom((activeInfo?.zoom || 100) / 100));
	$effect(() => {
		void docKey; // autre classeur : les zooms enregistrés font foi
		zoomSet = {};
	});
	let zoomTimer = 0;
	function setZoom(z: number) {
		const v = clampZoom(z);
		zoomSet = { ...zoomSet, [zoomKey]: v };
		if (mode !== 'doc') return;
		const id = sheetId;
		clearTimeout(zoomTimer);
		zoomTimer = window.setTimeout(() => {
			DocumentService.SetZoom(id, Math.round(v * 100))
				.then((st) => (docState = st))
				.catch((e) => (notice = err(e)));
		}, 400);
	}
	const bookLocked = $derived(mode === 'doc' && !!layout?.workbookProtected);
	const sheetLocked = $derived(mode === 'doc' && (!!activeInfo?.protected || bookLocked));
	const lockedMsg = $derived(
		activeInfo?.protected
			? `La feuille « ${activeInfo?.name ?? ''} » est protégée : ôtez la protection (Révision › Ôter la protection) pour la modifier`
			: bookLocked
				? 'Le classeur est protégé : ôtez la protection (Révision › Ôter la protection du classeur) pour le modifier'
				: ''
	);

	/** Protège, ou ôte la protection (mot de passe demandé s'il y en a un). */
	function toggleProtect(what: 'sheet' | 'workbook', id = sheetId) {
		if (mode !== 'doc' || !layout) return;
		const s = layout.sheets?.find((x) => x.id === id);
		const on = what === 'sheet' ? !!s?.protected : !!layout.workbookProtected;
		const hasPw = what === 'sheet' ? !!s?.password : !!layout.workbookPassword;
		const name = what === 'sheet' ? (s?.name ?? '') : docState.title;
		if (on && !hasPw) {
			applyProtect(what, id, name, true, '');
			return;
		}
		pw1 = pw2 = pwError = '';
		protecting = { what, id, name, off: on };
	}

	async function applyProtect(what: 'sheet' | 'workbook', id: number, name: string, off: boolean, pw: string) {
		try {
			if (what === 'sheet') await (off ? DocumentService.UnprotectSheet(id, pw) : DocumentService.ProtectSheet(id, pw));
			else await (off ? DocumentService.UnprotectWorkbook(pw) : DocumentService.ProtectWorkbook(pw));
			protecting = null;
			await reload();
			notice =
				what === 'sheet'
					? off
						? `Protection de la feuille « ${name} » ôtée`
						: `Feuille « ${name} » protégée`
					: off
						? 'Protection du classeur ôtée'
						: 'Structure du classeur protégée';
		} catch (e) {
			if (protecting) pwError = err(e);
			else notice = err(e);
		}
	}

	function confirmProtect() {
		const p = protecting;
		if (!p) return;
		if (!p.off && pw1 !== pw2) {
			pwError = 'Les deux mots de passe ne correspondent pas';
			return;
		}
		applyProtect(p.what, p.id, p.name, p.off, pw1);
	}

	/** Supprimer une feuille : tout son contenu part avec elle (un Ctrl+Z rend tout) ; demandé d'abord si elle n'est pas vide. */
	async function askDeleteSheet(id: number) {
		try {
			const l = await DocumentService.Layout(id);
			const name = l.sheets?.find((x) => x.id === id)?.name ?? '';
			const items: string[] = [];
			const count = (n: number, one: string, many: string, names?: string[]) => {
				if (!n) return;
				const list = names?.length ? ` (${names.slice(0, 5).join(', ')}${names.length > 5 ? '…' : ''})` : '';
				items.push((n > 1 ? `${n.toLocaleString('fr-FR')} ${many}` : `1 ${one}`) + list);
			};
			const tables = l.tables ?? [];
			count(tables.length, 'tableau', 'tableaux', tables.map((t) => `${t.name} : ${t.total.toLocaleString('fr-FR')} ligne${t.total > 1 ? 's' : ''}`));
			count(l.pivots?.length ?? 0, 'tableau croisé', 'tableaux croisés', l.pivots?.map((x) => x.config.name ?? ''));
			count(l.charts?.length ?? 0, 'graphique', 'graphiques');
			count(l.buttons?.length ?? 0, 'bouton', 'boutons');
			count(l.cells?.length ?? 0, 'cellule remplie', 'cellules remplies');
			if (!items.length) return sheetAction(() => DocumentService.DeleteSheet(id));
			deleting = { id, name, items };
		} catch (e) {
			notice = err(e);
		}
	}

	async function sheetAction(fn: () => Promise<unknown>) {
		try {
			await fn();
			await reload();
			if (visibleSheet(layout?.sheets ?? [], sheetId) !== sheetId) {
				sheetId = visibleSheet(layout?.sheets ?? [], sheetId);
				await reload();
			}
		} catch (e) {
			notice = err(e);
		}
	}
</script>

<svelte:window oncontextmenu={blockNativeMenu} />

<div class="app">
	<TitleBar
		{title}
		dirty={mode === 'doc' && docState.dirty}
		actions={[
			{ label: 'Enregistrer (Ctrl+S)', icon: 'save', run: mode === 'doc' ? () => file('save') : undefined },
			{ label: docState.undo ? `Annuler : ${docState.undo} (Ctrl+Z)` : 'Annuler (Ctrl+Z)', icon: 'undo', run: mode === 'doc' && docState.undo ? () => DocumentService.Undo().then(reload).catch((e) => (notice = err(e))) : undefined },
			{ label: docState.redo ? `Rétablir : ${docState.redo} (Ctrl+Y)` : 'Rétablir (Ctrl+Y)', icon: 'redo', run: mode === 'doc' && docState.redo ? () => DocumentService.Redo().then(reload).catch((e) => (notice = err(e))) : undefined }
		]}
	/>
	<Ribbon
		{view}
		{title}
		dirty={mode === 'doc' && docState.dirty}
		disabled={mode !== 'doc' && mode !== 'demo'}
		{recent}
		onrecent={(p) => openPath(p).catch((e) => (notice = err(e)))}
		undo={mode === 'doc' ? docState.undo : ''}
		redo={mode === 'doc' ? docState.redo : ''}
		onview={(v) => {
			if (v !== view) sheet?.closeOverlays();
			view = v;
		}}
		onfile={file}
		onundo={() => DocumentService.Undo().then(reload).catch((e) => (notice = err(e)))}
		onredo={() => DocumentService.Redo().then(reload).catch((e) => (notice = err(e)))}
		onbench={mode === 'demo' ? () => sheet?.runBench() : undefined}
		onsort={mode === 'doc' ? (desc) => sheet?.sortActive(desc) : undefined}
		oncells={mode === 'doc' ? (op) => sheet?.cellsActive(op) : undefined}
		onstyle={mode === 'doc' ? (patch) => (activeDoc ? docPage?.applyStyle(patch) : sheet?.applyStyle(patch)) : undefined}
		onfind={mode === 'doc' ? (replace) => sheet?.openFinder(replace) : undefined}
		onrules={mode === 'doc' ? (x, y) => sheet?.rulesMenu(x, y) : undefined}
		onfilter={mode === 'doc' ? (errorsOnly) => sheet?.filterActive(errorsOnly) : undefined}
		onaudit={mode === 'doc' ? (k) => sheet?.auditActive(k) : undefined}
		onpainter={mode === 'doc' ? () => sheet?.formatPainter() : undefined}
		onclip={mode === 'doc' ? (what) => (what === 'paste' ? sheet?.pasteClipboard() : sheet?.copySelection(what === 'cut')) : undefined}
		onchart={mode === 'doc' ? (kind) => sheet?.chartActive(kind) : undefined}
		onformula={mode === 'doc' ? (what) => (what === 'insert' ? sheet?.openFunctions() : what === 'sum' ? sheet?.autoSum() : sheet?.startFormula('=REFERENCE(')) : undefined}
		onpivot={mode === 'doc' ? () => sheet?.pivotActive() : undefined}
		ondedup={mode === 'doc' ? () => sheet?.dedupActive() : undefined}
		ontextcols={mode === 'doc' ? () => sheet?.textToColumnsActive() : undefined}
		onbutton={mode === 'doc' ? (kind) => sheet?.insertButton(kind) : undefined}
		onpage={mode === 'doc' ? () => addPage() : undefined}
		ondocpage={mode === 'doc' ? () => addDoc() : undefined}
		context={activePage && view === 'grid' ? (activeDoc ? 'doc' : 'html') : 'sheet'}
		ondoc={docCommand}
		docState={activeDoc && docFmt ? docFmt : undefined}
		onrecalc={mode === 'doc' ? () => DocumentService.Recalc().then(reload).catch((e) => (notice = err(e))) : undefined}
		gridlines={!layout?.sheets?.find((s) => s.id === sheetId)?.noGrid}
		ongridlines={mode === 'doc' ? (show) => DocumentService.SetGridlines(sheetId, show).then(reload).catch((e) => (notice = err(e))) : undefined}
		headings={!!layout?.sheets?.find((s) => s.id === sheetId)?.headings}
		onheadings={mode === 'doc' ? (show) => DocumentService.SetHeadings(sheetId, show).then(reload).catch((e) => (notice = err(e))) : undefined}
		sheetProtected={sheetLocked}
		sheetOwnProtected={!!activeInfo?.protected}
		workbookProtected={mode === 'doc' && !!layout?.workbookProtected}
		onprotect={mode === 'doc' ? (what) => toggleProtect(what) : undefined}
		ondata={mode === 'doc' ? (what) => queryHost?.data(what) : undefined}
		onformat={mode === 'doc' ? (tab) => sheet?.openFormat(tab) : undefined}
		onmerge={mode === 'doc' ? () => sheet?.mergeActive(true) : undefined}
		onwrap={mode === 'doc' ? () => sheet?.toggleWrap() : undefined}
		onlines={mode === 'doc' ? (rows, hide) => sheet?.hideLines(rows, hide) : undefined}
		onvalidation={mode === 'doc' ? () => sheet?.openValidation() : undefined}
		onnote={mode === 'doc' ? (what) => (what === 'edit' ? sheet?.editNote() : sheet?.deleteNote()) : undefined}
		freeze={activeInfo ? { rows: activeInfo.freezeRows, cols: activeInfo.freezeCols } : undefined}
		onfreeze={mode === 'doc' ? (what) => sheet?.freezeActive(what) : undefined}
		{zoom}
		{animate}
		onanimate={setAnimate}
		onzoom={data && view === 'grid' && !activePage ? setZoom : undefined}
		cellStyle={activeDoc ? docFmt?.style : sheetStatus?.style}
		merged={!!sheetStatus?.merged}
	/>

	<main>
		{#if failure}
			<div class="splash"><h1>Lattis</h1><p class="error">{failure}</p></div>
		{:else if mode === 'recover'}
			<div class="splash">
				<h1>Récupération</h1>
				<p>Lattis s'est fermé sans enregistrer. Ces classeurs ont des modifications récupérables :</p>
				{#each orphans as o (o.dir)}
					<div class="orphan">
						<div>
							<strong>{o.source ? o.source.split(/[\\/]/).pop() : 'Classeur sans titre'}</strong>
							<span class="muted">modifié le {new Date(o.modified).toLocaleString('fr-FR')}</span>
						</div>
						<button class="primary ico" onclick={() => recover(o, true)}><Icon name="undo" size={14} />Récupérer</button>
						<button class="ico discard" onclick={() => recover(o, false)}><Icon name="trash" size={14} />Abandonner</button>
					</div>
				{/each}
			</div>
		{:else if mode === 'closed'}
			<div class="splash">
				<h1>Lattis</h1>
				<p>Aucun classeur ouvert.</p>
				<div class="actions">
					<button class="primary ico" onclick={() => file('new')}><Icon name="new-file" size={14} />Nouveau classeur</button>
					<button class="ico" onclick={() => file('open')}><Icon name="open" size={14} />Ouvrir…</button>
				</div>
				{#if recent.length}
					<div class="recent">
						<p class="muted">Récents</p>
						{#each recent as p (p)}
							<button class="link" title={p} onclick={() => openPath(p).catch((e) => (notice = err(e)))}>{p.split(/[\\/]/).pop()}</button>
						{/each}
					</div>
				{/if}
			</div>
		{:else if data}
			{#key mode}
				<div class="pane" class:hidden={view !== 'grid' || !!activePage}>
					<Sheet
						bind:this={sheet}
						{data}
						source={mode === 'demo' ? demoSource : docSource}
						sources={(help?.tables ?? []).map((t) => ({ name: t.name, columns: t.columns ?? [], types: t.types ?? [] }))}
						catalog={mode === 'doc' && help
							? { functions: help.functions ?? [], tables: (help.tables ?? []).filter((t) => t.kind !== 'pivot'), names: (layout?.names ?? []).map((n) => n.name) }
							: undefined}
						onstatus={(s) => (sheetStatus = s)}
						onhscroll={(s) => (hscroll = s)}
						gridlines={!layout?.sheets?.find((s) => s.id === sheetId)?.noGrid}
						headings={!!layout?.sheets?.find((s) => s.id === sheetId)?.headings}
						locked={lockedMsg}
						freeze={activeInfo ? { rows: activeInfo.freezeRows, cols: activeInfo.freezeCols } : undefined}
						{zoom}
						{animate}
						onzoom={setZoom}
						autoBench={bench ? (r) => GridService.ReportBench(r) : undefined}
					/>
				</div>
			{/key}
			{#if view === 'grid' && activePage}
				{#key activePage.sheet}
					{@const pageId = activePage.sheet}
					<div class="pane">
						{#if activeDoc}
						<DocPage
							bind:this={docPage}
							name={activeInfo?.name ?? ''}
							source={activePage.source}
							save={async (src) => {
								docState = await DocumentService.SavePage(pageId, src);
								await reload();
							}}
							locked={lockedMsg}
							stamp={layout}
							{animate}
							tables={(help?.tables ?? []).map((t) => ({ name: t.name, columns: t.columns ?? [], types: t.types ?? [], kind: t.kind }))}
							{scripts}
							loadChart={(cfg) => DocumentService.ChartData(cfg as never)}
							loadPivot={(cfg) => DocumentService.PivotData(cfg as never)}
							fieldValues={async (table, field) => (await DocumentService.FieldValues(table, field)) ?? []}
							sheets={(layout?.sheets ?? []).filter((s) => !s.kind).map((s) => s.name)}
							controls={{
								load: (id) => DocumentService.DocControl(id) as never,
								create: async (cfg) => {
									const id = await DocumentService.CreateButton(pageId, 0, 0, cfg as never);
									await reload();
									return id;
								},
								update: async (id, cfg) => {
									docState = await DocumentService.SetButton(id, cfg as never);
									await reload();
								},
								setValue: async (id, raw) => {
									const r = await DocumentService.SetControlValue(id, raw);
									await reload();
									if (r.error) notice = `Erreur du script : ${r.error}`;
									else if (r.output?.length) notice = r.output[r.output.length - 1];
								},
								click: async (id) => {
									const r = await DocumentService.RunButton(id);
									await reload();
									if (r.error) notice = `Erreur du script : ${r.error}`;
									else if (r.output?.length) notice = r.output[r.output.length - 1];
								},
								pickFile: (title) => DocumentService.PickControlFile(title)
							}}
							run={async (script, fn) => {
								const r = await DocumentService.RunFunction(script, fn);
								await reload();
								if (r.error) return `Erreur du script : ${r.error}`;
								const out = r.output ?? [];
								return out.length ? out[out.length - 1] : '';
							}}
							onformat={(f) => (docFmt = f)}
							onnotice={(m) => (notice = m)}
						/>
						{:else}
						<HtmlPage
							name={activeInfo?.name ?? ''}
							source={activePage.source}
							save={async (src) => {
								docState = await DocumentService.SavePage(pageId, src);
								await reload();
							}}
							call={(name, args) => DocumentService.PageCall(name, args)}
							locked={lockedMsg}
							stamp={layout}
							onnotice={(m) => (notice = m)}
						/>
						{/if}
					</div>
				{/key}
			{/if}
			{#if view === 'sql'}
				<div class="pane"><SqlView {objects} run={query} exec={mode === 'doc' ? (q) => DocumentService.Execute(q) : undefined} ongoto={gotoObject}
						tosheet={mode === 'doc'
							? async (q, name, headers, link, columns) => {
									if (!link) {
										const r = await DocumentService.QueryToSheet(q, name, headers);
										notice = `Résultat copié dans le tableau « ${r.table} » (Ctrl+Z pour annuler)`;
										await selectSheet(r.sheet);
										return;
									}
									const r = await DocumentService.ReferenceQuery(q, name, columns, headers);
									if (r.errors?.length) throw new Error(r.errors.join('\n'));
									const l = r.loads?.[0];
									if (!l) return;
									notice = `Tableau « ${l.table} » lié à la requête : clic droit › Actualiser pour le relire`;
									await reload();
									await selectSheet(l.sheet);
								}
							: undefined}
						favorites={mode === 'doc'
							? {
									list: favorites,
									add: async (name, q) => {
										const id = await DocumentService.AddFavorite(name, q);
										await reload();
										return id;
									},
									rename: (id, name) => DocumentService.RenameFavorite(id, name).then(reload),
									remove: (id) => DocumentService.DeleteFavorite(id).then(reload)
								}
							: undefined}
					/></div>
			{:else if view === 'script'}
				<div class="pane">
					{#if mode === 'doc'}
						<ScriptView
							{scripts}
							save={async (id, name, source) => {
								const nid = await DocumentService.SaveScript(id, name, source);
								await reload();
								return nid;
							}}
							remove={(id) => DocumentService.DeleteScript(id).then(reload)}
							run={async (name, source) => {
								const r = await DocumentService.RunScript(name, source);
								if (r.writes) await reload();
								return r;
							}}
							stop={() => DocumentService.StopScript()}
							subscribe={(fn) => Events.On('script:print', (e: { data: string }) => fn(e.data))}
							network={{
								list: async () => (await DocumentService.HTTPHosts()) ?? [],
								allow: async (h) => {
									const l = (await DocumentService.AllowHTTPHost(h)) ?? [];
									docState = await DocumentService.State();
									return l;
								},
								forget: async (h) => {
									const l = (await DocumentService.ForgetHTTPHost(h)) ?? [];
									docState = await DocumentService.State();
									return l;
								}
							}}
						/>
					{:else}
						<p class="placeholder">Les scripts s'exécutent sur un classeur : Fichier › Nouveau.</p>
					{/if}
				</div>
			{/if}
		{:else}
			<div class="splash">
				<h1>Lattis</h1>
				<p>{demoProgress || 'Ouverture…'}</p>
			</div>
		{/if}
		{#if mode === 'doc'}
			<QueryHost
				bind:this={queryHost}
				onchanged={(sh) => (sh && sh !== sheetId ? selectSheet(sh) : reload()).catch((e) => (notice = err(e)))}
				onnotice={(m) => (notice = m)}
			/>
		{/if}
	</main>

	<SheetTabs
		sheets={tabs}
		active={mode === 'demo' ? 1 : sheetId}
		onselect={selectSheet}
		onadd={mode === 'doc' ? () => sheetAction(async () => selectSheet(await DocumentService.AddSheet())) : undefined}
		oninsert={mode === 'doc' ? (before) => sheetAction(async () => selectSheet(await DocumentService.InsertSheet(before))) : undefined}
		onpage={mode === 'doc' ? (before) => addPage(before) : undefined}
		ondocpage={mode === 'doc' ? (before) => addDoc(before) : undefined}
		onmove={mode === 'doc' ? (id, to) => sheetAction(() => DocumentService.MoveSheet(id, to)) : undefined}
		onrename={mode === 'doc' ? (id, name) => sheetAction(() => DocumentService.RenameSheet(id, name)) : undefined}
		ondelete={mode === 'doc' ? (id) => askDeleteSheet(id) : undefined}
		oncopy={mode === 'doc' ? (id, to) => sheetAction(async () => selectSheet(await DocumentService.CopySheet(id, to))) : undefined}
		oncolor={mode === 'doc' ? (id, color) => sheetAction(() => DocumentService.SetSheetColor(id, color)) : undefined}
		onhide={mode === 'doc' ? (id, hidden) => sheetAction(async () => {
			await DocumentService.SetSheetHidden(id, hidden);
			if (!hidden) await selectSheet(id);
		}) : undefined}
		hscroll={view === 'grid' && !activePage ? hscroll : undefined}
		onhscroll={(x, atEnd) => sheet?.scrollLeftTo(x, atEnd)}
		onhhold={(on) => sheet?.holdHScroll(on)}
		onprotect={mode === 'doc' ? (id) => toggleProtect('sheet', id) : undefined}
		locked={mode === 'doc' && !!layout?.workbookProtected}
		onrefused={(m) => (notice = m)}
	/>
	{#each dialogs.slice(0, 1) as d (d.id)}
		<ScriptDialog spec={d} onanswer={(v) => answerDialog(d, v)} />
	{/each}
	{#if mcpOpen}
		<McpPanel onclose={() => (mcpOpen = false)} />
	{/if}
	{#if protecting}
		{@const p = protecting}
		<div class="modal-back">
			<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
			<form
				class="modal protect"
				aria-label="Protection"
				onkeydown={(e) => {
					e.stopPropagation();
					if (e.key === 'Escape') protecting = null;
				}}
				onsubmit={(e) => {
					e.preventDefault();
					confirmProtect();
				}}
			>
				<p>
					{#if p.what === 'sheet'}
						{p.off ? `Ôter la protection de la feuille « ${p.name} »` : `Protéger la feuille « ${p.name} »`}
					{:else}
						{p.off ? 'Ôter la protection du classeur' : 'Protéger la structure du classeur'}
					{/if}
				</p>
				{#if !p.off}
					<p class="muted">
						{p.what === 'sheet'
							? 'Saisie, collage, mise en forme, lignes et colonnes, tri, objets et requêtes SQL d’écriture seront refusés sur cette feuille. Lecture, copie et filtres restent permis.'
							: 'Ajouter, supprimer, renommer ou déplacer des feuilles sera refusé.'}
					</p>
				{/if}
				<label>
					Mot de passe{p.off ? '' : ' (facultatif)'}
					<!-- svelte-ignore a11y_autofocus -->
					<input type="password" bind:value={pw1} autofocus autocomplete="off" />
				</label>
				{#if !p.off}
					<label>
						Confirmer le mot de passe
						<input type="password" bind:value={pw2} autocomplete="off" />
					</label>
					<p class="muted">Sans le mot de passe, la protection ne pourra plus être ôtée : notez-le.</p>
				{/if}
				{#if pwError}<p class="error">{pwError}</p>{/if}
				<div class="actions">
					<button type="submit" class="primary">OK</button>
					<button type="button" onclick={() => (protecting = null)}>Annuler</button>
				</div>
			</form>
		</div>
	{/if}
	{#if deleting}
		{@const d = deleting}
		<div class="modal-back">
			<div class="modal" role="dialog" aria-modal="true" aria-label="Supprimer la feuille">
				<p>Supprimer la feuille « {d.name} » et tout ce qu'elle contient ?</p>
				<ul class="delete-list">
					{#each d.items as it (it)}<li>{it}</li>{/each}
				</ul>
				<p class="hint">Ctrl+Z rend la feuille et tout son contenu.</p>
				<div class="actions">
					<button
						class="primary danger"
						onclick={() => {
							const id = d.id;
							deleting = null;
							sheetAction(() => DocumentService.DeleteSheet(id));
						}}>Supprimer</button
					>
					<button onclick={() => (deleting = null)}>Annuler</button>
				</div>
			</div>
		</div>
	{/if}
	{#if closing}
		<div class="modal-back">
			<div class="modal" role="dialog" aria-modal="true">
				<p>Enregistrer les modifications de « {docState.title} » avant de le fermer ?</p>
				<div class="actions">
					<button class="primary" onclick={() => closeDoc(true).catch((e) => (notice = err(e)))}>Enregistrer</button>
					<button onclick={() => closeDoc(false).catch((e) => (notice = err(e)))}>Ne pas enregistrer</button>
					<button onclick={() => (closing = false)}>Annuler</button>
				</div>
			</div>
		</div>
	{/if}
	<footer class="statusbar">
		{#if notice || sheetStatus?.message}<span class="message">{sheetStatus?.message || notice}</span>{/if}
		<span class="spacer"></span>
		{#if sheetStatus}
			{#if sheetStatus.stats}<span class="stats">{sheetStatus.stats}</span>{/if}
			{#if import.meta.env.DEV}
				<!-- Mesures de développement : absentes de la version livrée. -->
				<span>{sheetStatus.fetch}</span>
				<span>{sheetStatus.draw}</span>
				<span>{sheetStatus.fps}</span>
				<span>{sheetStatus.edit}</span>
			{/if}
		{:else}
			<span>Prêt</span>
		{/if}
		<BusyBar />
		{#if data && view === 'grid' && !activePage}<ZoomBar {zoom} onzoom={setZoom} />{/if}
	</footer>
</div>

<style>
	.delete-list {
		margin: 6px 0 4px;
		padding-left: 20px;
		font-size: 12px;
		line-height: 1.5;
	}
	.hint {
		margin: 4px 0 0;
		font-size: 12px;
		color: #5f6368;
	}
	.modal .primary.danger {
		background: #c0392b;
		border-color: #c0392b;
	}
	/* Coins des surfaces flottantes (menus, boîtes de dialogue, panneaux de filtre…) : légèrement arrondis. */
	:global(:root) {
		--float-radius: 6px;
	}
	:global(html, body) {
		margin: 0;
		height: 100%;
		background: #fff;
		overflow: hidden;
	}
	.app :global(input),
	.app :global(textarea),
	.app :global(.cm-editor),
	.app :global(pre) {
		user-select: text;
	}
	.app {
		user-select: none;
		display: flex;
		flex-direction: column;
		height: 100vh;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	main {
		flex: 1;
		min-height: 0;
		position: relative;
	}
	.pane {
		position: absolute;
		inset: 0;
	}
	.hidden {
		visibility: hidden;
	}
	.placeholder {
		padding: 24px;
		color: #4b5563;
	}
	.statusbar {
		display: flex;
		align-items: center;
		gap: 14px;
		height: 24px;
		padding: 0 10px;
		background: #217346;
		color: #e8f3ec;
		font-size: 12px;
		white-space: nowrap;
		overflow: hidden;
	}
	.message {
		background: #fff7e0;
		color: #7a4b00;
		padding: 1px 8px;
		border-radius: 3px;
	}
	.spacer {
		flex: 1;
	}
	.stats {
		color: #fff;
		font-weight: 600;
		margin-right: 12px;
	}
	.splash {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		gap: 10px;
	}
	h1 {
		font-weight: 600;
		color: #217346;
		margin: 0 0 8px;
	}
	.orphan {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 8px 14px;
		border: 1px solid #d4d7db;
		border-radius: 4px;
		min-width: 460px;
	}
	.orphan div {
		flex: 1;
		display: flex;
		flex-direction: column;
	}
	button {
		font: inherit;
		padding: 4px 12px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	button.primary {
		background: #217346;
		color: #fff;
		border-color: #217346;
		padding-left: 15px;
		padding-right: 15px;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.actions {
		display: flex;
		gap: 8px;
		justify-content: center;
	}
	/* Boutons de l'accueil : icône devant le libellé. */
	.ico {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		--icon-accent: #217346;
	}
	.ico.primary {
		--icon-accent: currentColor;
	}
	.ico.discard {
		--icon-accent: #c0392b;
	}
	.recent {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 2px;
		margin-top: 12px;
	}
	.recent .muted {
		margin: 0 0 4px;
	}
	button.link {
		border: none;
		background: none;
		color: #217346;
		padding: 2px 8px;
	}
	button.link:hover {
		text-decoration: underline;
	}
	.modal-back {
		position: fixed;
		inset: 0;
		z-index: 50;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(0, 0, 0, 0.2);
	}
	.modal {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		padding: 16px 20px;
		max-width: 440px;
	}
	.modal p {
		margin: 0 0 14px;
	}
	.modal .actions {
		justify-content: flex-end;
	}
	.modal.protect {
		min-width: 340px;
	}
	.modal.protect label {
		display: flex;
		flex-direction: column;
		gap: 4px;
		margin-bottom: 10px;
	}
	.modal.protect input {
		font: inherit;
		padding: 4px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	.modal.protect input:focus {
		outline: none;
		border-color: #217346;
	}
	.modal.protect .muted {
		margin-top: -6px;
	}
	.error {
		color: #b42318;
	}
</style>
