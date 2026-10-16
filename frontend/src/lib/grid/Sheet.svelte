<script lang="ts">
	// Feuille façon Excel : une grille infinie (1 048 576 lignes × 16 384
	// colonnes, comme Excel) sur laquelle sont posés des tableaux et des
	// cellules libres. Pas d'adressage A1 : les numéros de ligne et les noms
	// de colonnes ne s'affichent que pour les tableaux, et le bandeau du haut
	// reprend les noms de colonnes d'un tableau quand son en-tête sort de
	// l'écran (comme Excel). Saisir n'importe où passe par la source, qui
	// décide (cellule libre, nouveau tableau, extension, élargissement).
	// Rendu <canvas> de la zone visible, cache de blocs de lignes par tableau,
	// éditeur DOM superposé à la cellule active, barre de formule.
	import { flushSync, onMount, untrack } from 'svelte';
	import { RowCache, percentile, type Fetched } from './cache';
	import { VScroll, clamp } from './scroll';
	import { Axis } from './axis';
	import { parseTSV, toTSV } from './clipboard';
	import { formatNumber, isNumericText, painterPatch, parseStyle, resolveStyle, styledNumber, type CellStyle, type StyleLayer } from './style';
	import { dismiss } from './dismiss';
	import { explainError } from './errors';
	import { fillSeries } from './fill';
	import Select from '#lib/ui/Select.svelte';
	import Chart from '#lib/chart/Chart.svelte';
	import ChartPanel from '#lib/chart/ChartPanel.svelte';
	import PivotPanel from './PivotPanel.svelte';
	import SheetButtons from './SheetButtons.svelte';
	import type { ButtonConfig, SheetButton } from './buttons';
	import { describeRule, parseRules, ruleStyle, RULE_OPS, RULE_STYLES, type Rule, type RuleOp } from './rules';
	import Icon from '#lib/icons/Icon.svelte';
	import { drawIcon } from '#lib/icons/canvas.js';
	import { apply as applySuggestion, signature, signatureParts, suggest, type Catalog, type Completion, type FunctionDoc } from './autocomplete';
	import {
		isErrorValue,
		valueAlignment,
		alignment,
		defaultWidth,
		editValue,
		formatValue,
		typeIcon,
		typeIconName,
		typeLabel,
		type ColumnType
	} from './format';

	export interface Column {
		name: string;
		type: ColumnType;
		fixed?: boolean; // type choisi par l'utilisateur (sinon inféré)
		errors?: number; // saisies hors type
		formula?: string; // colonne calculée : formule affichée
		style?: string; // mise en forme de toute la colonne (JSON)
		headerStyle?: string;
		rules?: string; // mise en forme conditionnelle (JSON)
		filter?: ColumnFilter; // filtre de la colonne
	}

	/** Colonne passée en texte par élargissement : type de la plupart de ses valeurs. */
	export interface TypeMix {
		type: ColumnType | ''; // vide : pas de type majoritaire
		off: number; // valeurs hors de ce type
		total: number; // valeurs non vides
	}

	export interface PivotValue {
		field?: string; // vide : nombre de lignes
		agg: 'sum' | 'count' | 'countDistinct' | 'avg' | 'min' | 'max';
		show?: string; // '', pctTotal, pctRow, pctCol
	}

	export interface PivotConfig {
		name?: string; // « TCD1 » : vue SQL et source de graphique
		source: string;
		rowFields?: string[];
		cols?: string;
		values?: PivotValue[];
		filters?: { field: string; values: string[] }[];
		noGrandTotal?: boolean;
	}

	export interface SheetPivot {
		id: number;
		row: number;
		col: number;
		config: PivotConfig;
		cells: string[][]; // en-tête, groupes, total
		error: string;
	}

	export interface ChartConfig {
		source: string;
		kind: 'bar' | 'column' | 'line' | 'area' | 'pie' | 'doughnut' | 'scatter';
		x: string;
		series: string[];
		title?: string;
		width?: number;
		height?: number;
		legend?: string;
		xTitle?: string;
		yTitle?: string;
		labels?: boolean;
		noGrid?: boolean;
		stacked?: boolean;
		yMin?: number;
		yMax?: number;
		colors?: string[];
		offX?: number;
		offY?: number;
	}

	export interface SheetChart {
		id: number;
		row: number;
		col: number;
		config: ChartConfig;
		labels: string[];
		series: { name: string; values: (number | null)[] }[];
		error: string;
	}

	export interface DepTarget {
		kind: string; // column, table, cell
		table?: string;
		column?: string;
		name?: string;
		sheet: number;
		row: number;
		col: number;
	}

	export interface ColumnFilter {
		values?: string[]; // valeurs gardées (texte affiché, vide : cellules vides)
		errors?: boolean; // saisies hors type seulement
	}

	export interface SheetTable {
		id: number;
		name: string;
		row: number; // ancre : ligne d'en-tête
		col: number;
		rows: number; // lignes de données affichées
		/** Lignes masquées par le filtre (elles gardent leur place sur la feuille). */
		hidden?: number;
		/** Rang de la colonne clé (identifiant des lignes), absent sans clé. */
		key?: number;
		columns: Column[];
	}

	export interface SheetData {
		/** Feuille affichée. */
		sheet?: number;
		tables: SheetTable[];
		/** value : résultat d'une formule (sinon raw est affiché). */
		cells: { row: number; col: number; raw: string; value?: string; error?: boolean; spill?: string[][] }[];
		/** Largeurs choisies par l'utilisateur (sinon : selon le type). */
		widths?: { col: number; width: number }[];
		/** Hauteurs choisies par l'utilisateur (sinon ROW_H). */
		heights?: { row: number; height: number }[];
		/** Cellules nommées de la feuille. */
		names?: { name: string; row: number; col: number }[];
		/** Mise en forme des cellules hors tableaux (JSON). */
		styles?: { row: number; col: number; style: string }[];
		/** Mise en forme des lignes et colonnes entières de la feuille (JSON). */
		rowStyles?: { index: number; style: string }[];
		colStyles?: { index: number; style: string }[];
		/** Mise en forme conditionnelle des lignes et colonnes entières (JSON). */
		rowRules?: { index: number; rules: string }[];
		colRules?: { index: number; rules: string }[];
		/** Tableaux croisés dynamiques de la feuille, calculés. */
		pivots?: SheetPivot[];
		/** Graphiques de la feuille, avec leurs données. */
		charts?: SheetChart[];
		/** Boutons d'action de la feuille. */
		buttons?: SheetButton[] | null;
	}

	/** Cible d'une saisie, pour la source. */
	export interface Target {
		table?: SheetTable;
		index?: number; // ligne de données
		column?: number; // colonne du tableau
		rowId?: number; // clé technique de la ligne
	}

	export type LinesOp = 'insertRows' | 'deleteRows' | 'insertCols' | 'deleteCols';

	export interface SheetSource {
		window(table: SheetTable, first: number, count: number): Promise<Fetched>;
		/**
		 * Enregistre une saisie. Renvoie un message pour la barre d'état, ou la
		 * valeur stockée quand la source ne relit pas sa mise en page.
		 */
		setCell(row: number, col: number, raw: string, target: Target): Promise<string | void | { value: unknown }>;
		undo?(): Promise<unknown>;
		redo?(): Promise<unknown>;
		/** Colle un bloc (une seule étape d'annulation). */
		paste?(row: number, col: number, block: string[][]): Promise<string | void>;
		/** Lignes ou colonnes entières de la feuille : insertion (avant a, b lignes) ou suppression (a à b). */
		sheetLines?(op: LinesOp, a: number, b: number): Promise<unknown>;
		/** Mise en forme conditionnelle de lignes (rows) ou colonnes entières i0 à i1. */
		setLineRules?(rows: boolean, i0: number, i1: number, rules: Rule[]): Promise<unknown>;
		/** Déplace les cellules libres d'une plage (glisser la bordure de la sélection). */
		moveCells?(r0: number, c0: number, r1: number, c1: number, dr: number, dc: number): Promise<unknown>;
		insertRow?(table: SheetTable, n: number): Promise<unknown>;
		deleteRow?(table: SheetTable, n: number): Promise<unknown>;
		insertColumn?(table: SheetTable, i: number): Promise<unknown>;
		deleteColumn?(table: SheetTable, i: number): Promise<unknown>;
		/** Nombre de valeurs qui ne se convertiraient pas au type to. */
		convertPreview?(table: SheetTable, i: number, to: ColumnType): Promise<number>;
		setType?(table: SheetTable, i: number, to: ColumnType): Promise<unknown>;
		/** Type majoritaire d'une colonne en texte au type inféré. */
		typeMix?(table: SheetTable, i: number): Promise<TypeMix>;
		setColumnWidth?(col: number, width: number): Promise<unknown>;
		/** Hauteur d'une ligne de la feuille (0 : par défaut). */
		setRowHeight?(row: number, height: number): Promise<unknown>;
		/** Nomme une cellule libre (vide : retire le nom). */
		setName?(row: number, col: number, name: string): Promise<unknown>;
		sortTable?(table: SheetTable, i: number, desc: boolean): Promise<unknown>;
		/** Met en forme le rectangle sélectionné (une valeur vide retire la propriété). */
		applyStyle?(r0: number, c0: number, r1: number, c1: number, patch: Record<string, unknown>): Promise<unknown>;
		/** Met en forme des lignes (rows) ou des colonnes entières de la feuille, comme Excel. */
		applyLineStyle?(rows: boolean, i0: number, i1: number, patch: Record<string, unknown>): Promise<unknown>;
		find?(text: string): Promise<{ row: number; col: number }[]>;
		/** Remplace partout ; renvoie le nombre de cellules modifiées. */
		replace?(text: string, by: string): Promise<number>;
		/** Remplace les règles de mise en forme conditionnelle d'une colonne. */
		setRules?(table: SheetTable, i: number, rules: Rule[]): Promise<unknown>;
		/** Valeurs distinctes d'une colonne (filtre). */
		filterValues?(table: SheetTable, i: number): Promise<{ text: string; count: number }[]>;
		/** Filtre d'une colonne (null : retire le filtre de la colonne). */
		setFilter?(table: SheetTable, i: number, f: ColumnFilter | null): Promise<unknown>;
		clearFilters?(table: SheetTable): Promise<unknown>;
		renameTable?(table: SheetTable, name: string): Promise<unknown>;
		/** Vide si la colonne peut devenir la clé du tableau, sinon la raison. */
		keyCheck?(table: SheetTable, i: number): Promise<string>;
		setKey?(table: SheetTable, i: number): Promise<unknown>;
		clearKey?(table: SheetTable): Promise<unknown>;
		/** Renouvelle AUJOURDHUI, MAINTENANT et ALEA (F9). */
		recalc?(): Promise<unknown>;
		/** Antécédents et dépendants d'une cellule (audit de formules). */
		dependencies?(row: number, col: number): Promise<{ subject: string; precedents: DepTarget[]; dependents: DepTarget[] }>;
		/** La 1re ligne de données devient les en-têtes. */
		promoteHeader?(table: SheetTable): Promise<unknown>;
		createPivot?(cfg: PivotConfig): Promise<unknown>;
		/** Valeurs distinctes d'une colonne d'un tableau (filtre d'un TCD). */
		pivotFieldValues?(table: string, field: string): Promise<{ text: string; count: number }[]>;
		createChart?(row: number, col: number, cfg: ChartConfig): Promise<unknown>;
		setChart?(id: number, cfg: ChartConfig): Promise<unknown>;
		moveChart?(id: number, row: number, col: number): Promise<unknown>;
		/** Ancrage libre : cellule, décalage et taille en pixels. */
		placeChart?(id: number, row: number, col: number, offX: number, offY: number, width: number, height: number): Promise<unknown>;
		deleteChart?(id: number): Promise<unknown>;
		/** Boutons d'action : bibliothèque de scripts, clic, création, modification. */
		scripts?(): Promise<{ name: string; source: string }[]>;
		runButton?(id: number): Promise<unknown>;
		createButton?(row: number, col: number, cfg: ButtonConfig): Promise<unknown>;
		setButton?(id: number, cfg: ButtonConfig): Promise<unknown>;
		placeButton?(id: number, row: number, col: number, offX: number, offY: number, width: number, height: number): Promise<unknown>;
		deleteButton?(id: number): Promise<unknown>;
		setPivot?(id: number, cfg: PivotConfig): Promise<unknown>;
		deletePivot?(id: number): Promise<unknown>;
		/** Supprime les lignes en double ; renvoie leur nombre. */
		removeDuplicates?(table: SheetTable, cols: number[]): Promise<number>;
		/** Nombre de lignes que removeDuplicates retirerait. */
		duplicatesPreview?(table: SheetTable, cols: number[]): Promise<number>;
		deleteTables?(tables: SheetTable[]): Promise<unknown>;
		/** Déplace un tableau, en-tête en (row, col) ; côte à côte d'un autre : fusion. */
		moveTable?(table: SheetTable, row: number, col: number): Promise<unknown>;
		/** Sort les colonnes i0..i1 vers un nouveau tableau en (row, col). */
		extractColumns?(table: SheetTable, i0: number, i1: number, row: number, col: number): Promise<unknown>;
	}

	export interface SheetStatus {
		cell: string;
		message: string;
		fetch: string;
		draw: string;
		fps: string;
		edit: string;
		blocks: number;
		/** Agrégats de la sélection, comme la barre d'état d'Excel. */
		stats: string;
		/** Sélection (à partir de 0, r0 ≤ r1, c0 ≤ c1). */
		sel: { r0: number; c0: number; r1: number; c1: number };
	}

	interface Props {
		data: SheetData;
		source: SheetSource;
		/** Fonctions et tableaux du classeur, pour l'aide à la saisie des formules. */
		catalog?: Catalog;
		/** Tableaux et tableaux croisés de toutes les feuilles (sources de graphiques et de TCD). */
		sources?: { name: string; columns: string[]; types: string[] }[];
		onstatus?: (s: SheetStatus) => void;
		/** Lance la mesure dès l'ouverture et transmet le résultat (mode SLITABLE_BENCH). */
		autoBench?: (result: string) => void;
		/** Quadrillage de la feuille (masqué : seules les lignes des tableaux restent). */
		gridlines?: boolean;
		/** Lettres de colonnes et numéros de lignes toujours affichés (sinon : pendant la saisie d'une formule). */
		headings?: boolean;
		/** Défilement horizontal (largeur du contenu, largeur visible, position) : la barre est dans la ligne des onglets, comme Excel. */
		onhscroll?: (s: { content: number; view: number; left: number }) => void;
	}

	let { data, source, catalog, sources = [], onstatus, autoBench, gridlines = true, headings = false, onhscroll }: Props = $props();

	export const SHEET_ROWS = 1_048_576;
	export const SHEET_COLS = 16_384;
	const ROW_H = 22;
	const DEF_W = 88;
	const TOP_H = 22;
	const MIN_MARGIN = 52;
	const FONT = '13px "Segoe UI", system-ui, -apple-system, sans-serif';
	const FONT_BOLD = '600 13px "Segoe UI", system-ui, -apple-system, sans-serif';
	const FONT_SMALL = '11px "Segoe UI", system-ui, -apple-system, sans-serif';

	const C = {
		bg: '#ffffff',
		grid: '#e2e4e7',
		strip: '#f5f6f7',
		stripLine: '#d4d7db',
		stripText: '#5f6368',
		head: '#e3efe7',
		headText: '#14532d',
		icon: '#6b8f78',
		border: '#8fbc9f',
		gray: '#f1f2f4',
		text: '#1f2328',
		placeholder: '#eef0f3',
		select: 'rgba(33, 115, 70, 0.10)',
		active: '#217346',
		activeStrip: '#d3e7dc',
		error: '#c0392b',
		spill: '#5b9bd5',
		found: 'rgba(250, 204, 21, 0.30)'
	};

	// --- Tableaux posés sur la feuille ------------------------------------

	interface Entry {
		t: SheetTable;
		cache: RowCache;
		last: number; // dernière ligne de la feuille occupée
		lastCol: number;
	}

	let entries: Entry[] = [];
	let cells = new Map<string, string>();
	let names = new Map<string, string>(); // position → nom
	// Mise en forme, en couches (voir style.ts) : colonnes et lignes entières
	// de la feuille, colonnes et en-têtes des tableaux, cellules.
	let freeStyles = new Map<string, StyleLayer>(); // position → style (hors tableaux : cellules libres, TCD)
	let sheetRowStyles = new Map<number, StyleLayer>();
	let sheetColStyles = new Map<number, StyleLayer>();
	// Mise en forme conditionnelle des lignes et colonnes entières.
	let sheetRowRules = new Map<number, Rule[]>();
	let sheetColRules = new Map<number, Rule[]>();

	/** Style des règles de la ligne ou de la colonne entière d'une cellule (colonne d'abord, comme pour la mise en forme). */
	function lineRule(r: number, c: number, v: unknown, text: string, bad = false): CellStyle | undefined {
		if (!sheetRowRules.size && !sheetColRules.size) return undefined;
		return resolveStyle(ruleStyle(sheetColRules.get(c), v, text, bad), ruleStyle(sheetRowRules.get(r), v, text, bad));
	}

	/** Valeur d'un texte affiché pour les règles : nombre si c'en est un. */
	function ruleValue(text: string): unknown {
		if (!isNumeric(text)) return text;
		const n = Number(text.replace(/[\s\u00a0\u202f%]/g, '').replace(',', '.'));
		return Number.isFinite(n) ? n : text;
	}
	// Styles de colonne et d'en-tête, par tableau puis colonne.
	let colStyles = new Map<number, (StyleLayer | undefined)[]>();
	let headStyles = new Map<number, (StyleLayer | undefined)[]>();
	let colRules = new Map<number, (Rule[] | undefined)[]>();
	let shown = new Map<string, { text: string; error: boolean }>(); // formules des cellules libres
	// Tableaux dynamiques déversés depuis une cellule libre (FILTRE, TRIER…).
	let spills: { row: number; col: number; rows: string[][] }[] = [];
	let pivots = $state.raw<SheetPivot[]>([]);
	let charts = $state.raw<SheetChart[]>([]);
	let buttons = $state.raw<SheetButton[]>([]);
	let buttonsLayer = $state<ReturnType<typeof SheetButtons>>();
	// Décalage de la vue, pour placer les graphiques au-dessus de la grille.
	let viewPos = $state({ x: 0, y: 0 });
	let colAxis = $state.raw(new Axis(SHEET_COLS, DEF_W));
	const key = (r: number, c: number) => `${r},${c}`;

	function makeEntry(t: SheetTable): Entry {
		const e: Entry = {
			t,
			last: t.row + t.rows,
			lastCol: t.col + t.columns.length - 1,
			cache: new RowCache(
				(first, count) => source.window(e.t, first, count),
				t.rows,
				(c, v) => formatValue(e.t.columns[c]?.type ?? 'text', v),
				() => {
					schedule();
					refreshFx();
				}
			)
		};
		return e;
	}

	/** Recalcule la géométrie quand la mise en page change (après une saisie). */
	function applyData(d: SheetData) {
		const old = new Map(entries.map((e) => [e.t.id, e]));
		entries = d.tables.map((t) => {
			const e = old.get(t.id);
			if (!e) return makeEntry(t);
			e.t = t;
			e.last = t.row + t.rows;
			e.lastCol = t.col + t.columns.length - 1;
			e.cache.invalidate(t.rows);
			return e;
		});
		cells = new Map(d.cells.map((c) => [key(c.row, c.col), c.raw]));
		mixes.clear();
		tipCell = '';
		pivots = d.pivots ?? [];
		if (openPivotOnLoad && pivots.length) {
			openPivotOnLoad = false;
			const p = pivots[pivots.length - 1];
			panel = { kind: 'pivot', id: p.id };
			active = { row: p.row, col: p.col };
			anchor = { ...active };
		}
		charts = d.charts ?? [];
		buttons = d.buttons ?? [];
		spills = d.cells.filter((c) => c.spill?.length).map((c) => ({ row: c.row, col: c.col, rows: c.spill! }));
		shown = new Map(d.cells.filter((c) => c.raw.startsWith('=')).map((c) => [key(c.row, c.col), { text: c.value ?? '', error: !!c.error }]));
		userWidths = new Map((d.widths ?? []).map((w) => [w.col, w.width]));
		userHeights = new Map((d.heights ?? []).map((h) => [h.row, h.height]));
		rebuildRows();
		names = new Map((d.names ?? []).map((n) => [key(n.row, n.col), n.name]));
		freeStyles = new Map((d.styles ?? []).map((s) => [key(s.row, s.col), parseStyle(s.style) ?? {}]));
		sheetRowStyles = new Map((d.rowStyles ?? []).map((s) => [s.index, parseStyle(s.style) ?? {}]));
		sheetColStyles = new Map((d.colStyles ?? []).map((s) => [s.index, parseStyle(s.style) ?? {}]));
		sheetRowRules = new Map((d.rowRules ?? []).map((s) => [s.index, parseRules(s.rules) ?? []]));
		sheetColRules = new Map((d.colRules ?? []).map((s) => [s.index, parseRules(s.rules) ?? []]));
		colStyles = new Map(d.tables.map((t) => [t.id, t.columns.map((c) => parseStyle(c.style))]));
		headStyles = new Map(d.tables.map((t) => [t.id, t.columns.map((c) => parseStyle(c.headerStyle))]));
		colRules = new Map(d.tables.map((t) => [t.id, t.columns.map((c) => parseRules(c.rules))]));
		rebuildAxis();
		updateExtent();
		refreshFx();
		schedule();
	}

	// Largeurs : choisies par l'utilisateur, sinon selon le type de la colonne
	// du premier tableau qui l'occupe, sinon la largeur par défaut.
	let userWidths = new Map<number, number>();
	const MAX_FIT = 400;
	let measurer: CanvasRenderingContext2D | null = null;
	/** Largeur du texte dans la police de la grille (normale ou grasse). */
	function textWidth(s: string, bold = false): number {
		if (!s) return 0;
		measurer ??= document.createElement('canvas').getContext('2d');
		if (!measurer) return 0;
		measurer.font = bold ? FONT_BOLD : FONT;
		return measurer.measureText(s).width;
	}
	function rebuildAxis(override?: { col: number; width: number }) {
		const widths = new Map<number, number>();
		for (const e of entries) {
			e.t.columns.forEach((c, i) => {
				if (!widths.has(e.t.col + i)) widths.set(e.t.col + i, defaultWidth(c.type));
			});
		}
		// Tableaux croisés : largeur d'après le texte affiché (« Somme de Commande »).
		for (const p of pivots) {
			if (p.error) continue;
			p.cells[0]?.forEach((_, j) => {
				const c = p.col + j;
				if (widths.has(c)) return;
				const w = Math.max(...p.cells.map((r, i) => textWidth(r[j] ?? '', i === 0 || i === p.cells.length - 1)));
				widths.set(c, clamp(Math.ceil(w) + 14, DEF_W, MAX_FIT));
			});
		}
		for (const [c, w] of userWidths) widths.set(c, w);
		if (override) widths.set(override.col, override.width);
		colAxis = new Axis(SHEET_COLS, DEF_W, widths);
	}

	/** Étendue affichée d'un tableau : une ligne ou une colonne de plus quand la
	 * cellule active est juste dessous ou juste à droite (la saisie l'étendra),
	 * jusqu'au tableau voisin qu'elle rejoindrait (fusion). */
	function grown(e: Entry): { last: number; lastCol: number } {
		const { row, col } = active;
		let last = e.last;
		let lastCol = e.lastCol;
		if (row === e.last + 1 && col >= e.t.col && col <= e.lastCol && !e.t.hidden) last++;
		if (col === e.lastCol + 1 && row >= e.t.row && row <= e.last) {
			lastCol++;
			const next = entries.find((o) => o.t.row === e.t.row && o.t.col === col + 1);
			if (next) {
				lastCol = next.lastCol;
				last = Math.max(last, next.last);
			}
		}
		return { last, lastCol };
	}

	const inTable = (e: Entry, r: number, c: number) => r >= e.t.row && r <= e.last && c >= e.t.col && c <= e.lastCol;
	const tableAt = (r: number, c: number) => entries.find((e) => inTable(e, r, c));

	// La zone de défilement suit le contenu, comme Excel : la partie utilisée
	// de la feuille plus un écran, et elle grandit à mesure qu'on défile vers
	// le bas ou la droite (au lieu de barres faites pour 1 M de lignes vides).
	// Hauteurs des lignes : par défaut ROW_H, sinon celles choisies (marge de gauche).
	let rowAxis = $state.raw(new Axis(SHEET_ROWS, ROW_H));
	let userHeights = new Map<number, number>();
	const rh = (r: number) => rowAxis.size(r);
	const rowTop = (r: number) => rowAxis.offset(r);
	const rspan = (r0: number, r1: number) => rowAxis.offset(r1 + 1) - rowAxis.offset(r0);
	function rebuildRows(override?: { row: number; height: number }) {
		const hs = new Map(userHeights);
		if (override) hs.set(override.row, override.height);
		rowAxis = new Axis(SHEET_ROWS, ROW_H, hs);
	}

	const vs = new VScroll(100 * ROW_H, 0);
	let spacer: HTMLDivElement;
	let extentCols = 26;
	// Largeur de la gouttière : élargie d'après le plus grand numéro de ligne
	// visible, comme Excel (« 1000000 » ne doit pas être coupé).
	let MARGIN = $state(MIN_MARGIN);

	// Pendant qu'on tire une barre de défilement, la zone ne rétrécit pas (la
	// barre ne « glisse » plus sous la souris) ; arrivée au bout, elle
	// s'agrandit d'un écran, comme Excel.
	let holdRows = 0;
	let holdCols = 0;

	/** Barre horizontale (ligne des onglets) tirée : figer la largeur, l'agrandir au bout. */
	export function holdHScroll(on: boolean) {
		holdCols = on ? extentCols : 0;
		updateExtent();
	}

	function growAtEnd(rows: boolean) {
		if (rows) {
			const lastVisible = rowAxis.indexAt(vs.top + vs.view);
			holdRows = Math.min(SHEET_ROWS, Math.max(holdRows, lastVisible + Math.max(10, Math.floor(vs.view / ROW_H))));
		} else {
			const cw = Math.max(0, viewW - MARGIN);
			holdCols = Math.min(SHEET_COLS, Math.max(holdCols, colAxis.indexAt(scroller.scrollLeft + cw) + Math.max(3, Math.floor(cw / DEF_W))));
		}
	}

	function updateExtent() {
		if (!scroller) return;
		let usedRow = Math.max(active.row, anchor.row);
		let usedCol = Math.max(active.col, anchor.col);
		for (const e of entries) {
			usedRow = Math.max(usedRow, e.t.row + e.t.rows + (e.t.hidden ?? 0) + 1);
			usedCol = Math.max(usedCol, e.lastCol + 1);
		}
		for (const k of cells.keys()) {
			const [r, c] = k.split(',').map(Number);
			usedRow = Math.max(usedRow, r);
			usedCol = Math.max(usedCol, c);
		}
		const lastVisible = rowAxis.indexAt(vs.top + vs.view);
		// Comme Excel : la barre s'arrête à la zone utilisée (ou à l'écran affiché) ;
		// molette, flèches et sélection vont au-delà et l'agrandissent.
		const rows = Math.min(SHEET_ROWS, Math.max(usedRow + 1, lastVisible + 1, holdRows));
		const cw = Math.max(0, viewW - MARGIN);
		const lastVisibleCol = colAxis.indexAt(scroller.scrollLeft + cw);
		extentCols = Math.min(SHEET_COLS, Math.max(usedCol + 1, lastVisibleCol + 1, holdCols));
		vs.content = rowTop(rows);
		vs.to(vs.top);
		onhscroll?.({ content: MARGIN + colAxis.offset(extentCols), view: scroller.clientWidth, left: scroller.scrollLeft });
		if (spacer) {
			spacer.style.height = `${TOP_H + vs.spacer}px`;
			spacer.style.width = `${MARGIN + colAxis.offset(extentCols)}px`;
		}
	}
	let viewW = $state(0);
	let viewH = $state(0);

	let wrap: HTMLDivElement;
	let scroller: HTMLDivElement;
	let canvas: HTMLCanvasElement;
	let input = $state<HTMLInputElement>();
	let ctx: CanvasRenderingContext2D;
	let mounted = false;

	// Sélection : cellule active + ancre de la plage (Maj + flèches / glisser).
	let active = $state.raw({ row: 0, col: 0 });
	let anchor = { row: 0, col: 0 };
	let dragging = false;
	// Déplacement par la bordure de la sélection (comme Excel).
	let moving = $state<{
		sel: { r0: number; r1: number; c0: number; c1: number };
		from: { row: number; col: number };
		to?: { row: number; col: number };
	} | null>(null);

	// Édition (dans la cellule ou dans la barre de formule).
	let editing = $state(false);
	let editByTyping = false;
	let editValueText = $state('');
	let fxText = $state('');
	let fxInput = $state<HTMLInputElement>();

	// Aide à la saisie des formules (éditeur de cellule ou barre de formule).
	let completion = $state<(Completion & { sel: number; where: 'cell' | 'fx' }) | null>(null);
	let sig = $state<{ fn: FunctionDoc; index: number; where: 'cell' | 'fx' } | null>(null);
	let suggestPos = $state({ x: 0, y: 0 });

	function onFormulaInput(el: HTMLInputElement, where: 'cell' | 'fx') {
		if (where === 'cell') {
			fxText = el.value;
			// Une formule se lit de gauche à droite, quelle que soit la colonne.
			if (el.value.startsWith('=')) el.style.textAlign = 'left';
		}
		if (!catalog) return;
		suggestPos = { x: MARGIN + colAxis.offset(active.col) - scroller.scrollLeft, y: TOP_H + rowTop(active.row + 1) - vs.top + 2 };
		const caret = el.selectionStart ?? el.value.length;
		const e = tableAt(active.row, active.col);
		const c = suggest(el.value, caret, catalog, e?.t.name);
		completion = c ? { ...c, sel: 0, where } : null;
		const s = signature(el.value, caret, catalog);
		sig = s ? { ...s, where } : null;
	}

	/** Le curseur bouge sans saisie (flèches, clic) : l'argument en gras suit. */
	function moveSig(e: Event, where: 'cell' | 'fx') {
		if (!catalog || (e instanceof KeyboardEvent && !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key))) return;
		const el = e.currentTarget as HTMLInputElement;
		const s = signature(el.value, el.selectionStart ?? el.value.length, catalog);
		sig = s ? { ...s, where } : null;
	}

	/** Gère les touches de la liste de suggestions ; vrai si la touche est consommée. */
	function completionKey(e: KeyboardEvent, el: HTMLInputElement, set: (v: string) => void): boolean {
		if (!completion) return false;
		const c = completion;
		switch (e.key) {
			case 'ArrowDown':
				completion = { ...c, sel: (c.sel + 1) % c.items.length };
				break;
			case 'ArrowUp':
				completion = { ...c, sel: (c.sel + c.items.length - 1) % c.items.length };
				break;
			case 'Tab':
			case 'Enter': {
				const caret = el.selectionStart ?? el.value.length;
				const r = applySuggestion(el.value, caret, c.from, c.items[c.sel]);
				set(r.text);
				completion = null;
				requestAnimationFrame(() => {
					el.setSelectionRange(r.caret, r.caret);
					onFormulaInput(el, c.where);
				});
				break;
			}
			case 'Escape':
				completion = null;
				break;
			default:
				return false;
		}
		e.preventDefault();
		e.stopPropagation();
		return true;
	}
	let nameBox = $state('');
	// Cellule d'un tableau : nom du tableau (modifiable) et colonne[ligne].
	let tableBox = $state<{ name: string; col: string } | null>(null);
	let fxEditing = $state(false);

	let message = '';
	let bench = $state('');

	// Mesures.
	const drawTimes: number[] = [];
	const frameStamps: number[] = [];
	let lastEditMs = NaN;
	let missingFrames = 0;

	$effect(() => {
		const d = data;
		untrack(() => applyData(d));
	});

	// Mode A1 : lettres et numéros dans les bandeaux pendant la saisie d'une
	// formule (ou toujours, Affichage › En-têtes A1) ; un clic sur une
	// cellule insère alors sa référence.
	// Barre de formule synchrone avec la saisie dans la cellule (et la cellule
	// avec la barre), frappe par frappe, comme Excel.
	$effect(() => {
		if (editing) fxText = editValueText;
	});
	$effect(() => {
		void fxText;
		if (fxEditing) untrack(() => schedule());
	});

	const formulaMode = $derived((editing && editValueText.trimStart().startsWith('=')) || (fxEditing && fxText.trimStart().startsWith('=')));
	const a1 = $derived(headings || formulaMode);

	$effect(() => {
		void gridlines;
		void a1;
		untrack(() => schedule());
	});

	let frame = 0;
	function schedule() {
		if (mounted && !frame) frame = requestAnimationFrame(draw);
	}

	function visibleRows() {
		const first = rowAxis.indexAt(vs.top);
		const last = Math.min(SHEET_ROWS - 1, rowAxis.indexAt(vs.top + vs.view - 1));
		return { first, last };
	}

	/** Lignes de données d'un tableau visibles (indices de données), ou null. */
	function visibleData(e: Entry, first: number, last: number) {
		const a = Math.max(first, e.t.row + 1);
		const b = Math.min(last, e.last);
		return a <= b ? { first: a - e.t.row - 1, last: b - e.t.row - 1 } : null;
	}

	const isNumeric = isNumericText;

	/**
	 * Style effectif d'une cellule de la feuille, quelle qu'elle soit (cellule
	 * de tableau, en-tête, cellule libre, de tableau croisé ou vide) : colonne
	 * puis ligne de la feuille, colonne du tableau, cellule. Sans la mise en
	 * forme conditionnelle.
	 */
	function styleAt(r: number, c: number, e = tableAt(r, c)): CellStyle | undefined {
		const col = sheetColStyles.get(c);
		const row = sheetRowStyles.get(r);
		if (!e) return resolveStyle(col, row, freeStyles.get(key(r, c)));
		const i = c - e.t.col;
		if (r === e.t.row) return resolveStyle(col, row, headStyles.get(e.t.id)?.[i]);
		return resolveStyle(col, row, colStyles.get(e.t.id)?.[i], parseStyle(e.cache.row(r - e.t.row - 1)?.sty?.get(i)));
	}

	function draw() {
		frame = 0;
		pendingBorders = [];
		if (viewPos.x !== scroller.scrollLeft || viewPos.y !== vs.top) viewPos = { x: scroller.scrollLeft, y: vs.top };
		const t0 = performance.now();
		const w = viewW;
		const h = viewH;
		const dpr = window.devicePixelRatio || 1;
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		ctx.fillStyle = C.bg;
		ctx.fillRect(0, 0, w, h);

		const left = scroller.scrollLeft;
		const { first: r0, last: r1 } = visibleRows();
		ctx.font = FONT_SMALL;
		const margin = Math.max(MIN_MARGIN, Math.ceil(ctx.measureText(String(r1)).width) + 14);
		if (margin !== MARGIN) {
			MARGIN = margin;
			updateExtent();
		}
		const c0 = colAxis.indexAt(left);
		const c1 = colAxis.indexAt(left + w - MARGIN);
		const cx = (c: number) => MARGIN + colAxis.offset(c) - left;
		const ry = (r: number) => TOP_H + rowTop(r) - vs.top;
		const xEnd = Math.min(w, cx(c1 + 1));
		const spanW = (a: number, b: number) => colAxis.offset(b + 1) - colAxis.offset(a);

		const visible = entries.filter((e) => e.last + 1 >= r0 && e.t.row <= r1 && e.lastCol + 1 >= c0 && e.t.col <= c1);
		let incomplete = false;
		for (const e of visible) {
			const d = visibleData(e, r0, r1);
			if (d) {
				e.cache.want(d.first, d.last);
				if (!e.cache.has(d.first, d.last)) incomplete = true;
			}
		}
		if (incomplete) missingFrames++;

		// En-têtes (agrandis à l'écran quand la cellule active les prolonge).
		for (const e of visible) {
			const g = grown(e);
			ctx.fillStyle = C.head;
			ctx.fillRect(cx(e.t.col), ry(e.t.row), spanW(e.t.col, g.lastCol), rh(e.t.row));
		}

		// Plage sélectionnée.
		const s0 = Math.min(active.row, anchor.row);
		const s1 = Math.max(active.row, anchor.row);
		const k0 = Math.min(active.col, anchor.col);
		const k1 = Math.max(active.col, anchor.col);
		if (s0 !== s1 || k0 !== k1) {
			ctx.fillStyle = C.select;
			ctx.fillRect(cx(k0), ry(s0), spanW(k0, k1), rspan(s0, s1));
		}

		// Recherche : toutes les occurrences visibles, en léger (l'active reste encadrée).
		if (finder && matches.length && matchText === finder.text) {
			ctx.fillStyle = C.found;
			for (const m of matches)
				if (m.row >= r0 && m.row <= r1 && m.col >= c0 && m.col <= c1) ctx.fillRect(cx(m.col), ry(m.row), colAxis.size(m.col), rh(m.row));
		}

		// Lignes masquées par un filtre : une note sous le tableau.
		for (const e of visible) {
			const n = e.t.hidden ?? 0;
			const r = e.last + 1;
			if (!n || r < r0 || r > r1) continue;
			ctx.font = 'italic ' + FONT_SMALL;
			ctx.fillStyle = C.icon;
			ctx.textAlign = 'left';
			ctx.textBaseline = 'middle';
			ctx.fillText(`⧩ ${n} ligne${n > 1 ? 's' : ''} masquée${n > 1 ? 's' : ''} par le filtre`, cx(e.t.col) + 6, ry(r) + rh(r) / 2);
		}

		// Contenu des tableaux, colonne par colonne (une découpe par colonne).
		ctx.textBaseline = 'middle';
		for (const e of visible) {
			const tr0 = Math.max(r0, e.t.row);
			const tr1 = Math.min(r1, e.last);
			for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol) && tr0 <= tr1; c++) {
				const i = c - e.t.col;
				const col = e.t.columns[i];
				const x = cx(c);
				const cw = colAxis.size(c);
				ctx.save();
				ctx.beginPath();
				ctx.rect(x, TOP_H, cw, h - TOP_H);
				ctx.clip();
				for (let r = tr0; r <= tr1; r++) {
					const y = ry(r);
					if (r === e.t.row) {
						const hs = styleAt(r, c, e);
						if (hs) borders(hs, x, y, cw, rh(r));
						if (hs?.bg) {
							ctx.fillStyle = hs.bg;
							ctx.fillRect(x, y, cw, rh(r));
						}
						ctx.font = FONT_SMALL;
						ctx.fillStyle = C.icon;
						ctx.textAlign = 'right';
						// Icône du type : SVG pour date et booléen, texte pour les autres.
						const svg = typeIconName(col.type);
						const ICON = 13;
						const icon = svg ? ' ▾' : (col.filter ? '⧩ ' : '') + typeIcon(col.type) + ' ▾';
						let iconW = ctx.measureText(icon).width;
						ctx.fillText(icon, x + cw - 6, y + rh(r) / 2);
						if (svg) {
							drawIcon(ctx, svg, x + cw - 6 - iconW - ICON, y + (rh(r) - ICON) / 2, ICON, C.icon, C.active, 1.3);
							iconW += ICON;
							if (col.filter) {
								ctx.fillText('⧩ ', x + cw - 6 - iconW, y + rh(r) / 2);
								iconW += ctx.measureText('⧩ ').width;
							}
						}
						if (e.t.key === i) {
							ctx.fillText('🔑 ', x + cw - 6 - iconW, y + rh(r) / 2);
							iconW += ctx.measureText('🔑 ').width;
						}
						if (col.errors) {
							ctx.fillStyle = C.error;
							ctx.fillText('⚠ ' + col.errors, x + cw - 6 - iconW - ctx.measureText(' ').width, y + rh(r) / 2);
							ctx.fillStyle = C.icon;
						}
						ctx.save();
						ctx.beginPath();
						ctx.rect(x, y, cw - iconW - 10, rh(r));
						ctx.clip();
						ctx.font = (hs?.i ? 'italic ' : '') + FONT_BOLD;
						ctx.fillStyle = hs?.fg ?? C.headText;
						ctx.textAlign = 'left';
						const label = ellipsize(col.name, cw - iconW - 16);
						ctx.fillText(label, x + 6, y + rh(r) / 2);
						if (hs?.u) ctx.fillRect(x + 6, y + rh(r) / 2 + 7, ctx.measureText(label).width, 1);
						ctx.restore();
						continue;
					}
					const row = e.cache.row(r - e.t.row - 1);
					if (!row) {
						ctx.fillStyle = C.placeholder;
						ctx.fillRect(x + 6, y + 7, cw * 0.6, rh(r) - 14);
						continue;
					}
					const v = row.cells[i];
					const bad = row.bad?.has(i) || (col.type === 'formula' && isErrorValue(v));
					const cs = resolveStyle(styleAt(r, c, e), lineRule(r, c, v, row.text[i] ?? '', !!bad), ruleStyle(colRules.get(e.t.id)?.[i], v, row.text[i] ?? '', !!bad));
					if (cs?.bg) {
						ctx.fillStyle = cs.bg;
						ctx.fillRect(x, y, cw, rh(r));
					}
					if (cs) borders(cs, x, y, cw, rh(r));
					let s = row.text[i];
					if (!s) continue;
					if ((cs?.nf || cs?.dp !== undefined) && !row.bad?.has(i))
						s = typeof row.cells[i] === 'number' ? (formatNumber(cs!.nf, row.cells[i] as number, cs!.dp) ?? s) : styledNumber(s, cs);
					if (row.bad?.has(i)) {
						// Saisie hors type : valeur brute en rouge, coin rouge comme le
						// triangle d'Excel.
						ctx.font = FONT;
						ctx.textAlign = 'left';
						ctx.fillStyle = C.error;
						ctx.fillText(s, x + 6, y + rh(r) / 2);
						ctx.beginPath();
						ctx.moveTo(x + cw - 7, y + 1);
						ctx.lineTo(x + cw - 1, y + 1);
						ctx.lineTo(x + cw - 1, y + 7);
						ctx.closePath();
						ctx.fill();
						continue;
					}
					const align = cs?.al ?? (col.type === 'formula' ? valueAlignment(v) : alignment(col.type));
					drawText(s, x, y, cw, align, cs, col.type === 'formula' && isErrorValue(v) ? C.error : C.text);
					if (col.type === 'formula' && isErrorValue(v)) errorCorner(x, y);
				}
				ctx.restore();
			}
		}

		// Mise en forme hors tableaux (cellules libres, tableaux croisés, cellules
		// vides des lignes et colonnes mises en forme) : remplissage et bordures.
		ctx.font = FONT;
		ctx.fillStyle = C.text;
		const paint = (r: number, c: number) => {
			if (tableAt(r, c)) return;
			const fs = resolveStyle(styleAt(r, c, undefined), freeRule(r, c));
			if (!fs) return;
			if (fs.bg) {
				ctx.fillStyle = fs.bg;
				ctx.fillRect(cx(c), ry(r), colAxis.size(c), rh(r));
			}
			borders(fs, cx(c), ry(r), colAxis.size(c), rh(r));
		};
		if (sheetRowStyles.size || sheetColStyles.size || sheetRowRules.size || sheetColRules.size) {
			for (let r = r0; r <= r1; r++) for (let c = c0; c <= c1; c++) paint(r, c);
		} else
			for (const k of freeStyles.keys()) {
				const [r, c] = k.split(',').map(Number);
				if (r >= r0 && r <= r1 && c >= c0 && c <= c1) paint(r, c);
			}
		// Valeurs déversées : comme des cellules, dans un cadre bleu.
		for (const sp of spills) {
			const sr1 = sp.row + sp.rows.length - 1;
			const sc1 = sp.col + sp.rows[0].length - 1;
			if (sr1 < r0 || sp.row > r1 || sc1 < c0 || sp.col > c1) continue;
			for (let i = Math.max(0, r0 - sp.row); i < sp.rows.length && sp.row + i <= r1; i++)
				sp.rows[i].forEach((v, j) => {
					if ((i === 0 && j === 0) || !v) return;
					const c = sp.col + j;
					const num = isNumeric(v);
					const st = styleAt(sp.row + i, c, undefined);
					drawText(styledNumber(v, st), cx(c), ry(sp.row + i), colAxis.size(c), st?.al ?? (num ? 'right' : 'left'), st, isErrorValue(v) ? C.error : C.text);
				});
			ctx.save();
			ctx.strokeStyle = C.spill;
			ctx.lineWidth = 1;
			ctx.strokeRect(Math.round(cx(sp.col)) + 0.5, Math.round(ry(sp.row)) + 0.5, spanW(sp.col, sc1) - 1, rspan(sp.row, sp.row + sp.rows.length - 1) - 1);
			ctx.restore();
		}
		// Tableaux croisés : en-tête et total en gras, dans un cadre.
		for (const p of pivots) {
			if (p.error) {
				drawText('Tableau croisé : ' + p.error, cx(p.col), ry(p.row), colAxis.size(p.col), 'left', undefined, C.error, w);
				continue;
			}
			const pr1 = p.row + p.cells.length - 1;
			const pc1 = p.col + p.cells[0].length - 1;
			if (pr1 < r0 || p.row > r1 || pc1 < c0 || p.col > c1) continue;
			// En-tête et total sur fond gris, sauf remplissage choisi.
			ctx.fillStyle = C.head;
			for (const r of [p.row, pr1])
				for (let c = p.col; c <= pc1; c++) if (!styleAt(r, c, undefined)?.bg) ctx.fillRect(cx(c), ry(r), colAxis.size(c), rh(r));
			for (let i = Math.max(0, r0 - p.row); i < p.cells.length && p.row + i <= r1; i++)
				p.cells[i].forEach((v, j) => {
					if (!v) return;
					const c = p.col + j;
					const bold = i === 0 || i === p.cells.length - 1;
					// Mise en forme posée sur les cellules du TCD, leurs lignes ou colonnes (format de nombre, décimales, police…).
					const st = resolveStyle(styleAt(p.row + i, c, undefined), lineRule(p.row + i, c, ruleValue(v), v));
					const style = bold ? { ...st, b: st?.b ?? true } : st;
					drawText(styledNumber(v, st), cx(c), ry(p.row + i), colAxis.size(c), st?.al ?? (j > 0 && i > 0 ? 'right' : 'left'), style, C.text);
				});
			ctx.save();
			ctx.strokeStyle = C.border;
			ctx.lineWidth = 1;
			ctx.strokeRect(Math.round(cx(p.col)) + 0.5, Math.round(ry(p.row)) + 0.5, spanW(p.col, pc1) - 1, rspan(p.row, pr1) - 1);
			ctx.restore();
		}
		for (const [k, rawText] of cells) {
			const [r, c] = k.split(',').map(Number);
			if (r < r0 || r > r1 || c < c0 || c > c1) continue;
			const fstyle = resolveStyle(styleAt(r, c, undefined), freeRule(r, c));
			const x = cx(c);
			const cw = colAxis.size(c);
			const f = shown.get(k);
			if (f?.error) errorCorner(x, ry(r));
			let raw = f ? f.text : rawText;
			const num = isNumeric(raw);
			if (num) raw = styledNumber(raw, fstyle);
			if (fstyle) {
				drawText(raw, x, ry(r), cw, fstyle.al ?? (num ? 'right' : 'left'), fstyle, f?.error ? C.error : C.text, num ? cw : w);
				continue;
			}
			ctx.fillStyle = f?.error ? C.error : C.text;
			ctx.save();
			ctx.beginPath();
			ctx.rect(Math.max(x, MARGIN), TOP_H, num ? cw : w, h - TOP_H);
			ctx.clip();
			ctx.textAlign = num ? 'right' : 'left';
			ctx.fillText(raw, num ? x + cw - 6 : x + 6, ry(r) + rh(r) / 2);
			ctx.restore();
		}

		// Quadrillage de toute la feuille ; masqué, il ne reste qu'à
		// l'intérieur des tableaux.
		ctx.save();
		if (!gridlines) {
			ctx.beginPath();
			for (const e of visible) {
				const g = grown(e);
				ctx.rect(cx(e.t.col), ry(e.t.row), spanW(e.t.col, g.lastCol), rspan(e.t.row, g.last));
			}
			ctx.clip();
		}
		ctx.beginPath();
		ctx.strokeStyle = C.grid;
		ctx.lineWidth = 1;
		for (let r = r0; r <= r1 + 1; r++) {
			const y = Math.round(ry(r)) - 0.5;
			ctx.moveTo(MARGIN, y);
			ctx.lineTo(xEnd, y);
		}
		for (let c = c0; c <= c1 + 1; c++) {
			const x = Math.round(cx(c)) - 0.5;
			ctx.moveTo(x, TOP_H);
			ctx.lineTo(x, h);
		}
		ctx.stroke();
		ctx.restore();
		strokeBorders();

		ctx.save();
		ctx.beginPath();
		ctx.rect(MARGIN, TOP_H, w - MARGIN, h - TOP_H);
		ctx.clip();
		// Bordure discrète de chaque tableau, trait sous l'en-tête.
		ctx.strokeStyle = C.border;
		ctx.lineWidth = 1;
		for (const e of visible) {
			const g = grown(e);
			const x = Math.round(cx(e.t.col)) - 0.5;
			const y = Math.round(ry(e.t.row)) - 0.5;
			ctx.strokeStyle = g.last !== e.last || g.lastCol !== e.lastCol ? C.active : C.border;
			ctx.strokeRect(x, y, spanW(e.t.col, g.lastCol), rspan(e.t.row, g.last));
			ctx.beginPath();
			ctx.moveTo(x, y + rh(e.t.row));
			ctx.lineTo(x + spanW(e.t.col, g.lastCol), y + rh(e.t.row));
			ctx.stroke();
		}
		// Zone copiée ou coupée : cadre pointillé, comme Excel.
		if (clipRange) {
			const k = clipRange;
			ctx.save();
			ctx.setLineDash([4, 3]);
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 1.5;
			ctx.strokeRect(cx(k.c0) + 1, ry(k.r0) + 1, spanW(k.c0, k.c1) - 2, rspan(k.r0, k.r1) - 2);
			ctx.restore();
		}
				// Déplacement en cours : contour de la destination.
		if (moving?.to) {
			const { r0, c0, r1, c1 } = moving.sel;
			const dr = moving.to.row - moving.from.row;
			const dc = moving.to.col - moving.from.col;
			ctx.save();
			ctx.setLineDash([5, 3]);
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 2;
			ctx.strokeRect(cx(c0 + dc) + 1, ry(r0 + dr) + 1, spanW(c0 + dc, c1 + dc) - 2, rspan(r0 + dr, r1 + dr) - 2);
			ctx.restore();
		}
		// Cellule active.
		if (active.row >= r0 - 1 && active.row <= r1 + 1) {
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 2;
			ctx.strokeRect(cx(active.col) + 0.5, ry(active.row) + 0.5, colAxis.size(active.col) - 1, rh(active.row) - 1);
		}
		// Flèches d'audit : bleues vers la cellule (antécédents), rouges depuis
		// elle (dépendants), comme Excel.
		if (arrows) {
			const center = (r: number, c: number) => ({ x: cx(c) + colAxis.size(c) / 2, y: ry(r) + rh(r) / 2 });
			const me = center(arrows.row, arrows.col);
			const draw = (from: { x: number; y: number }, to: { x: number; y: number }, color: string) => {
				ctx.save();
				ctx.strokeStyle = color;
				ctx.fillStyle = color;
				ctx.lineWidth = 1.5;
				ctx.beginPath();
				ctx.arc(from.x, from.y, 3, 0, Math.PI * 2);
				ctx.fill();
				ctx.beginPath();
				ctx.moveTo(from.x, from.y);
				ctx.lineTo(to.x, to.y);
				ctx.stroke();
				const a = Math.atan2(to.y - from.y, to.x - from.x);
				ctx.beginPath();
				ctx.moveTo(to.x, to.y);
				ctx.lineTo(to.x - 9 * Math.cos(a - 0.4), to.y - 9 * Math.sin(a - 0.4));
				ctx.lineTo(to.x - 9 * Math.cos(a + 0.4), to.y - 9 * Math.sin(a + 0.4));
				ctx.closePath();
				ctx.fill();
				ctx.restore();
			};
			for (const p of arrows.pre) if (p.sheet === sheetIdOf()) draw(center(p.row, p.col), me, '#2b6cb0');
			for (const d of arrows.dep) if (d.sheet === sheetIdOf()) draw(me, center(d.row, d.col), '#c0392b');
		}
		// Saisie dans la barre de formule : la cellule active l'affiche en direct.
		if (fxEditing && !editing) {
			const x = cx(active.col);
			const y = ry(active.row);
			const cw = colAxis.size(active.col);
			ctx.fillStyle = C.bg;
			ctx.fillRect(x + 1, y + 1, cw - 2, rh(active.row) - 2);
			drawText(fxText, x, y, cw, 'left', undefined, C.text, w);
		}
		// Poignée de recopie (coin bas droit de la sélection) et aperçu.
		if (!editing && selChart === null) {
			const h = fillHandle();
			ctx.fillStyle = C.active;
			ctx.fillRect(h.x - 3, h.y - 3, 6, 6);
			ctx.fillStyle = C.bg;
			ctx.fillRect(h.x - 3, h.y + 3, 6, 1);
			ctx.fillRect(h.x + 3, h.y - 3, 1, 7);
		}
		if (filling?.to) {
			const t = filling.to;
			ctx.save();
			ctx.setLineDash([3, 2]);
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 1.5;
			ctx.strokeRect(cx(t.c0) + 1, ry(t.r0) + 1, spanW(t.c0, t.c1) - 2, rspan(t.r0, t.r1) - 2);
			ctx.restore();
		}
		ctx.restore();

		// Bandeau du haut : vide (pas de lettres de colonnes), sauf quand
		// l'en-tête d'un tableau visible est sorti de l'écran.
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, 0, w, TOP_H);
		// Colonnes de la sélection : même surbrillance que les lignes dans la marge.
		ctx.fillStyle = C.activeStrip;
		for (let c = Math.max(c0, k0); c <= Math.min(c1, k1); c++) {
			ctx.fillRect(Math.max(cx(c), MARGIN), 0, cx(c + 1) - Math.max(cx(c), MARGIN), TOP_H);
		}
		if (a1) {
			ctx.font = FONT_SMALL;
			ctx.textAlign = 'center';
			for (let c = c0; c <= c1; c++) {
				const x = cx(c);
				if (x + colAxis.size(c) <= MARGIN) continue;
				ctx.fillStyle = c >= k0 && c <= k1 ? C.headText : C.stripText;
				ctx.fillText(addrA1(0, c).slice(0, -1), x + colAxis.size(c) / 2, TOP_H / 2);
			}
		}
		for (const e of a1 ? [] : visible) {
			if (e.t.row >= r0 || e.last < r0) continue;
			for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol); c++) {
				const x = cx(c);
				ctx.save();
				ctx.beginPath();
				ctx.rect(Math.max(x, MARGIN), 0, colAxis.size(c), TOP_H);
				ctx.clip();
				ctx.fillStyle = c >= k0 && c <= k1 ? C.activeStrip : C.head;
				ctx.fillRect(x, 0, colAxis.size(c), TOP_H);
				ctx.font = FONT_BOLD;
				ctx.fillStyle = C.headText;
				ctx.textAlign = 'left';
				ctx.fillText(e.t.columns[c - e.t.col].name, x + 6, TOP_H / 2);
				ctx.restore();
			}
		}
		// Marge de gauche : petits traits aux séparations de lignes (redimensionnables).
		ctx.beginPath();
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		for (let r = r0; r <= r1; r++) {
			const y = Math.round(ry(r + 1)) - 0.5;
			if (y <= TOP_H) continue;
			ctx.moveTo(MARGIN * 0.55, y);
			ctx.lineTo(MARGIN, y);
		}
		ctx.stroke();
		// Petits traits aux séparations de colonnes : on voit où redimensionner.
		ctx.beginPath();
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		for (let c = c0; c <= c1; c++) {
			const x = Math.round(cx(c + 1)) - 0.5;
			if (x <= MARGIN) continue;
			ctx.moveTo(x, TOP_H * 0.35);
			ctx.lineTo(x, TOP_H);
		}
		ctx.stroke();
		for (let c = Math.max(c0, k0); c <= Math.min(c1, k1); c++) {
			ctx.fillStyle = C.active;
			ctx.fillRect(cx(c), TOP_H - 2, colAxis.size(c), 2);
		}

		// Marge de gauche : numéros 1..n des lignes de données des tableaux.
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, TOP_H, MARGIN, h - TOP_H);
		ctx.font = FONT_SMALL;
		ctx.textAlign = 'right';
		// Numéros : ceux du tableau de la sélection (ou de celui qu'elle
		// prolonge) ; ceux des autres tableaux, plus discrets.
		const cur = tableAt(active.row, active.col) ?? entries.find((e) => grown(e).last !== e.last);
		const curLast = cur ? grown(cur).last : -1;
		for (let r = r0; r <= r1; r++) {
			const y = ry(r);
			if (r >= s0 && r <= s1) {
				ctx.fillStyle = C.activeStrip;
				ctx.fillRect(0, y, MARGIN, rh(r));
				ctx.fillStyle = C.active;
				ctx.fillRect(MARGIN - 2, y, 2, rh(r));
			}
			if (a1) {
				ctx.fillStyle = r >= s0 && r <= s1 ? C.headText : C.stripText;
				ctx.fillText(String(r + 1), MARGIN - 8, y + rh(r) / 2);
				continue;
			}
			const mine = !!cur && r > cur.t.row && r <= curLast;
			const e = mine ? cur : visible.find((e) => r > e.t.row && r <= e.last);
			if (e) {
				ctx.fillStyle = mine || !cur ? C.stripText : C.stripLine;
				// Avec une clé, ses valeurs identifient les lignes dans la marge (choix de Louis).
				let label = String(r - e.t.row);
				if (e.t.key !== undefined && r <= e.last) {
					const k = e.cache.row(r - e.t.row - 1)?.text[e.t.key];
					if (k !== undefined) label = k;
				}
				ctx.save();
				ctx.beginPath();
				ctx.rect(0, y, MARGIN - 4, rh(r));
				ctx.clip();
				ctx.fillText(label, MARGIN - 8, y + rh(r) / 2);
				ctx.restore();
			}
		}
		ctx.beginPath();
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		ctx.moveTo(0, TOP_H - 0.5);
		ctx.lineTo(w, TOP_H - 0.5);
		ctx.moveTo(MARGIN - 0.5, 0);
		ctx.lineTo(MARGIN - 0.5, h);
		ctx.stroke();

		if (editing) placeEditor();

		const t1 = performance.now();
		pushStat(drawTimes, t1 - t0);
		frameStamps.push(t1);
		while (frameStamps.length && frameStamps[0] < t1 - 1000) frameStamps.shift();
	}

	/** Texte d'une cellule avec sa mise en forme (gras, italique, souligné, couleur). */
	/** Texte raccourci avec « … » pour tenir dans width (police courante). */
	function ellipsize(text: string, width: number): string {
		if (ctx.measureText(text).width <= width) return text;
		let lo = 0;
		let hi = text.length;
		while (lo < hi) {
			const mid = (lo + hi + 1) >> 1;
			if (ctx.measureText(text.slice(0, mid) + '…').width <= width) lo = mid;
			else hi = mid - 1;
		}
		return lo > 0 ? text.slice(0, lo) + '…' : '…';
	}

	// Bordures des cellules : relevées pendant le dessin des cellules, tracées
	// après le quadrillage, sur ses lignes (deux cellules voisines partagent
	// le même trait), pour qu'il ne les recouvre pas.
	let pendingBorders: number[] = [];

	/** Bordures d'une cellule (style bt, bb, bl, br). */
	function borders(st: CellStyle, x: number, y: number, w: number, h: number) {
		if (!st.bt && !st.bb && !st.bl && !st.br) return;
		const X0 = Math.round(x) - 0.5;
		const X1 = Math.round(x + w) - 0.5;
		const Y0 = Math.round(y) - 0.5;
		const Y1 = Math.round(y + h) - 0.5;
		if (st.bt) pendingBorders.push(X0 - 0.5, Y0, X1 + 0.5, Y0);
		if (st.bb) pendingBorders.push(X0 - 0.5, Y1, X1 + 0.5, Y1);
		if (st.bl) pendingBorders.push(X0, Y0 - 0.5, X0, Y1 + 0.5);
		if (st.br) pendingBorders.push(X1, Y0 - 0.5, X1, Y1 + 0.5);
	}

	function strokeBorders() {
		if (!pendingBorders.length) return;
		ctx.save();
		ctx.beginPath();
		ctx.rect(MARGIN, TOP_H, viewW - MARGIN, viewH - TOP_H);
		ctx.clip();
		ctx.strokeStyle = C.text;
		ctx.lineWidth = 1;
		ctx.beginPath();
		const b = pendingBorders;
		for (let i = 0; i < b.length; i += 4) {
			ctx.moveTo(b[i], b[i + 1]);
			ctx.lineTo(b[i + 2], b[i + 3]);
		}
		ctx.stroke();
		ctx.restore();
		pendingBorders = [];
	}

	/** Petit triangle d'angle d'une cellule en erreur (le détail est dans la barre d'état). */
	function errorCorner(x: number, y: number) {
		ctx.fillStyle = C.error;
		ctx.beginPath();
		ctx.moveTo(x + 1, y + 1);
		ctx.lineTo(x + 7, y + 1);
		ctx.lineTo(x + 1, y + 7);
		ctx.closePath();
		ctx.fill();
	}

	function drawText(s: string, x: number, y: number, cw: number, align: 'left' | 'center' | 'right', st: CellStyle | undefined, color: string, clipW = cw) {
		ctx.save();
		ctx.beginPath();
		ctx.rect(Math.max(x, MARGIN), TOP_H, clipW, viewH - TOP_H);
		ctx.clip();
		ctx.font = (st?.i ? 'italic ' : '') + (st?.b ? FONT_BOLD : FONT);
		ctx.textAlign = align;
		ctx.textBaseline = 'middle';
		ctx.fillStyle = st?.fg ?? color;
		const tx = align === 'left' ? x + 6 : align === 'right' ? x + cw - 6 : x + cw / 2;
		const h = rh(rowAxis.indexAt(y - TOP_H + vs.top + 1));
		ctx.fillText(s, tx, y + h / 2);
		if (st?.u) {
			const w = ctx.measureText(s).width;
			const lx = align === 'left' ? tx : align === 'right' ? tx - w : tx - w / 2;
			ctx.fillRect(lx, y + h / 2 + 7, w, 1);
		}
		ctx.restore();
	}

	/**
	 * Style de la sélection (pour basculer gras, italique…, et le pinceau) :
	 * celui de la ligne ou colonne entière sélectionnée, sinon celui de la
	 * cellule active.
	 */
	function activeStyle(): CellStyle {
		const w = wholeKind();
		if (w === 'rows') return resolveStyle(sheetRowStyles.get(active.row)) ?? {};
		if (w === 'cols') return resolveStyle(sheetColStyles.get(active.col)) ?? {};
		return styleAt(active.row, active.col) ?? {};
	}

	/** Met en forme la sélection ; une bascule (b, i, u) suit la cellule active. */
	export function applyStyle(patch: Record<string, unknown>) {
		if (!source.applyStyle) return;
		const p = { ...patch };
		for (const k of ['b', 'i', 'u'] as const) if (p[k] === 'toggle') p[k] = !activeStyle()[k];
		if (p.dp === 'more' || p.dp === 'less') {
			// Comme Excel : part des décimales affichées dans la cellule active.
			const cur = activeStyle().dp ?? decimalsOf(cellInfo(active.row, active.col).value);
			p.dp = Math.max(0, Math.min(10, cur + (p.dp === 'more' ? 1 : -1)));
		}
		const { r0, r1, c0, c1 } = selection();
		const w = wholeKind();
		if (w && source.applyLineStyle) {
			// Lignes ou colonnes entières : la mise en forme vaut pour toutes
			// leurs cellules, présentes et à venir (comme Excel).
			const line = source.applyLineStyle;
			const rows = w === 'rows';
			const [i0, i1] = rows ? [r0, r1] : [c0, c1];
			if (p.border === 'outline') {
				menuAction(async () => {
					await line(rows, i0, i0, rows ? { bt: true } : { bl: true });
					await line(rows, i1, i1, rows ? { bb: true } : { br: true });
				});
				return;
			}
			menuAction(() => line(rows, i0, i1, borderPatch(p)));
			return;
		}
		const apply = source.applyStyle;
		if (p.border === 'outline') {
			// Contour : chaque bord de la plage reçoit son trait.
			menuAction(async () => {
				await apply(r0, c0, r0, c1, { bt: true });
				await apply(r1, c0, r1, c1, { bb: true });
				await apply(r0, c0, r1, c0, { bl: true });
				await apply(r0, c1, r1, c1, { br: true });
			});
			return;
		}
		menuAction(() => apply(r0, c0, r1, c1, borderPatch(p)));
	}

	/** Traduit border: 'all' | 'none' en propriétés de bordure. */
	function borderPatch(p: Record<string, unknown>) {
		if (p.border === 'all') {
			delete p.border;
			Object.assign(p, { bt: true, bb: true, bl: true, br: true });
		} else if (p.border === 'none') {
			delete p.border;
			Object.assign(p, { bt: null, bb: null, bl: null, br: null });
		}
		return p;
	}

	function decimalsOf(text: string): number {
		const m = /[,.](\d+)/.exec(text.replace(/\s/g, ''));
		return m ? m[1].length : 0;
	}

	// Reproduire la mise en forme : la prochaine sélection reçoit le style copié.
	let painter = $state<Record<string, unknown> | null>(null);
	export function formatPainter() {
		painter = painterPatch(activeStyle());
		say('Reproduire la mise en forme : sélectionnez les cellules à mettre en forme (Échap pour annuler)');
	}
	function applyPainter() {
		if (!painter) return;
		const p = painter;
		painter = null;
		applyStyle(p);
	}

	function pushStat(a: number[], v: number) {
		a.push(v);
		if (a.length > 300) a.shift();
	}

	const ms = (v: number) => (Number.isFinite(v) ? v.toFixed(v < 10 ? 1 : 0) + ' ms' : '–');

	/** Ce que contient une cellule, pour l'édition et la barre de formule. */
	function cellInfo(r: number, c: number) {
		const e = tableAt(r, c);
		if (!e) return { e, kind: 'free' as const, value: cells.get(key(r, c)) ?? '' };
		const i = c - e.t.col;
		if (r === e.t.row) return { e, i, kind: 'header' as const, value: e.t.columns[i].name };
		const n = r - e.t.row - 1;
		const row = e.cache.row(n);
		return {
			e,
			i,
			n,
			row,
			kind: 'data' as const,
			// Une saisie hors type se corrige à partir de sa valeur brute.
			// Colonne calculée : la barre de formule montre la formule, comme Excel.
			value: e.t.columns[i].formula ?? (row ? (row.bad?.get(i) ?? editValue(e.t.columns[i].type, row.cells[i])) : '')
		};
	}

	/** Désignation de la cellule active (pas d'adresse A1). */
	/** Adresse d'une cellule libre, telle qu'elle s'écrit dans une formule ou en SQL : Cell[A1]. */
	function cellAddr(r: number, c: number): string {
		return `Cell[${addrA1(r, c)}]`;
	}

	/** Lettre(s) de colonne et numéro de ligne : A1. */
	function addrA1(r: number, c: number): string {
		let s = '';
		for (let n = c + 1; n > 0; n = Math.floor((n - 1) / 26)) s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
		return s + (r + 1);
	}

	function cellName(r: number, c: number): string {
		const named = names.get(key(r, c));
		if (named) return named;
		const e = tableAt(r, c);
		// Cellule libre : lettre et numéro seulement (A1) ; Cell_A1 dans une formule.
		if (!e) return addrA1(r, c);
		const k = e.t.columns[c - e.t.col];
		const col = `${e.t.name}[${k.name}]`;
		if (r === e.t.row) {
			const errs = k.errors ? ` · ${k.errors} erreur${k.errors > 1 ? 's' : ''}` : '';
			return `${col} · ${typeLabel(k.type)} (${k.fixed ? 'fixé' : 'inféré'})${errs}`;
		}
		const bad = e.cache.row(r - e.t.row - 1)?.bad?.get(c - e.t.col);
		return `${col} · ${r - e.t.row}` + (bad !== undefined ? ` · « ${bad} » n'est pas du type ${typeLabel(k.type)}` : '');
	}

	function refreshStatus() {
		const stats = entries[0]?.cache.stats ?? { roundTrip: [], server: [] };
		onstatus?.({
			// Une cellule libre : son adresse est déjà dans la zone de nom.
			cell: tableAt(active.row, active.col) || names.has(key(active.row, active.col)) ? cellName(active.row, active.col) : '',
			message,
			fetch: `bloc p50 ${ms(percentile(stats.roundTrip, 0.5))}`,
			draw: `rendu ${ms(percentile(drawTimes, 0.5))}`,
			fps: `${frameStamps.length} img/s`,
			edit: `saisie ${ms(lastEditMs)}`,
			blocks: entries.reduce((n, e) => n + e.cache.size, 0),
			stats: selectionStats(),
			sel: selection()
		});
	}

	const statFmt = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 10 });

	/** « Moyenne · Nb · Somme » de la sélection (cellules chargées), si plusieurs cellules. */
	function selectionStats(): string {
		const r0 = Math.min(active.row, anchor.row);
		const r1 = Math.max(active.row, anchor.row);
		const c0 = Math.min(active.col, anchor.col);
		const c1 = Math.max(active.col, anchor.col);
		const n = (r1 - r0 + 1) * (c1 - c0 + 1);
		if (n < 2) return '';
		if (n > 50_000) return 'Sélection trop grande pour les totaux';
		let count = 0;
		let nums = 0;
		let sum = 0;
		let missing = false;
		for (let r = r0; r <= r1; r++) {
			for (let c = c0; c <= c1; c++) {
				const e = tableAt(r, c);
				let v: unknown;
				if (e) {
					if (r === e.t.row) {
						count++;
						continue;
					}
					const row = e.cache.row(r - e.t.row - 1);
					if (!row) {
						missing = true;
						continue;
					}
					v = row.bad?.has(c - e.t.col) ? row.bad.get(c - e.t.col) : row.cells[c - e.t.col];
				} else {
					const f = shown.get(key(r, c));
					v = f ? f.text : cells.get(key(r, c));
					if (typeof v === 'string' && isNumeric(v)) v = Number(v.replace(/[\s\u00a0\u202f]/g, '').replace(',', '.'));
				}
				if (v === null || v === undefined || v === '') continue;
				count++;
				if (typeof v === 'number' && Number.isFinite(v)) {
					nums++;
					sum += v;
				}
			}
		}
		if (count === 0) return '';
		const parts = [];
		if (nums) parts.push(`Moyenne : ${statFmt.format(sum / nums)}`);
		parts.push(`Nb : ${count.toLocaleString('fr-FR')}`);
		if (nums) parts.push(`Somme : ${statFmt.format(sum)}`);
		return parts.join('   ') + (missing ? ' (lignes en chargement)' : '');
	}

	// --- Filtre -----------------------------------------------------------------

	let filterDlg = $state<{
		e: Entry;
		i: number;
		x: number;
		y: number;
		values: { text: string; count: number; on: boolean }[];
		search: string;
		errors: boolean;
		error: string;
	} | null>(null);

	/** Position d'un panneau ancré sous l'en-tête de la colonne i du tableau e,
	 * gardé dans la grille (width : largeur du panneau). */
	function anchorUnder(e: Entry, i: number, width: number) {
		const x = MARGIN + colAxis.offset(e.t.col + i) - scroller.scrollLeft;
		return {
			x: Math.max(4, Math.min(x, scroller.clientWidth - width - 4)),
			y: clamp(TOP_H + rowTop(e.t.row + 1) - vs.top, 4, Math.max(4, scroller.clientHeight - 200))
		};
	}

	/** Met le focus sur le premier champ d'un panneau à son ouverture. */
	function focusFirst(node: HTMLElement) {
		requestAnimationFrame(() => (node.querySelector<HTMLElement>('.add input') ?? node.querySelector<HTMLElement>('.add button'))?.focus());
	}

	async function openFilter(e: Entry, i: number) {
		menu = null;
		if (!source.filterValues) return;
		const f = e.t.columns[i]?.filter;
		try {
			const vals = await source.filterValues(e.t, i);
			const keep = f?.values ? new Set(f.values) : null;
			filterDlg = {
				e,
				i,
				...anchorUnder(e, i, 310),
				values: vals.map((v) => ({ ...v, on: !keep || keep.has(v.text) })),
				search: '',
				errors: !!f?.errors,
				error: ''
			};
		} catch (err) {
			say(String(err instanceof Error ? err.message : err));
		}
	}

	/** Filtre de la colonne de la cellule active (ruban Données). */
	export function filterActive(errorsOnly = false) {
		const e = tableAt(active.row, active.col);
		if (!e || !source.setFilter) {
			say('Placez la cellule active dans la colonne d’un tableau pour filtrer');
			return;
		}
		const i = active.col - e.t.col;
		if (errorsOnly) {
			const f = e.t.columns[i]?.filter;
			const set = source.setFilter;
			menuAction(() => set(e.t, i, f?.errors ? (f.values ? { values: f.values } : null) : { ...f, errors: true }));
			return;
		}
		openFilter(e, i);
	}

	function filterShown() {
		const d = filterDlg;
		if (!d) return [];
		const q = d.search.trim().toLocaleLowerCase('fr');
		return q ? d.values.filter((v) => v.text.toLocaleLowerCase('fr').includes(q)) : d.values;
	}

	async function applyFilter(clear = false) {
		const d = filterDlg;
		if (!d || !source.setFilter) return;
		let f: ColumnFilter | null = null;
		if (!clear) {
			const all = d.values.every((v) => v.on);
			f = {};
			if (!all) f.values = d.values.filter((v) => v.on).map((v) => v.text);
			if (d.errors) f.errors = true;
			if (!f.values && !f.errors) f = null;
		}
		try {
			await source.setFilter(d.e.t, d.i, f);
			closeFilter();
		} catch (err) {
			d.error = String(err instanceof Error ? err.message : err);
		}
	}

	function closeFilter() {
		filterDlg = null;
		wrap.focus();
	}

	// --- Mise en forme conditionnelle -----------------------------------------

	let rulesRev = $state(0); // relit les règles après enregistrement
	// e/i : colonne d'un tableau ; line : lignes ou colonnes entières de la feuille.
	let rulesDlg = $state<{ e?: Entry; i: number; line?: { rows: boolean; i0: number; i1: number }; x: number; y: number; op: RuleOp; value: string; to: string; style: number; error: string } | null>(null);

	/** Règles d'une cellule libre (ou de TCD), d'après sa valeur affichée. */
	function freeRule(r: number, c: number): CellStyle | undefined {
		if (!sheetRowRules.size && !sheetColRules.size) return undefined;
		const k = key(r, c);
		const f = shown.get(k);
		const text = f ? f.text : cells.get(k);
		if (text === undefined) return undefined;
		return lineRule(r, c, ruleValue(text), text, !!f?.error);
	}

	/** Ouvre le dialogue des règles des lignes ou colonnes entières sélectionnées. */
	function openLineRules(rows: boolean) {
		const { r0, r1, c0, c1 } = selection();
		menu = freeMenu = null;
		const x = rows ? MARGIN + 8 : Math.max(MARGIN, MARGIN + colAxis.offset(c0) - scroller.scrollLeft);
		const y = rows ? TOP_H + rowTop(r1 + 1) - vs.top + 4 : TOP_H + 4;
		rulesDlg = { i: rows ? r0 : c0, line: { rows, i0: rows ? r0 : c0, i1: rows ? r1 : c1 }, x, y, op: '>', value: '', to: '', style: 0, error: '' };
	}

	/** Ouvre le dialogue des règles de la colonne i du tableau e. */
	function openRules(e: Entry, i: number) {
		menu = null;
		rulesDlg = { e, i, ...anchorUnder(e, i, 470), op: '>', value: '', to: '', style: 0, error: '' };
	}

	/** Règles de la colonne de la cellule active (ruban Accueil). */
	export function rulesActive() {
		const sel = selectionLines();
		if ((sel.rows || sel.cols) && source.setLineRules) {
			openLineRules(sel.rows);
			return;
		}
		const e = tableAt(active.row, active.col);
		if (!e || !source.setRules) {
			say('Placez la cellule active dans la colonne d’un tableau');
			return;
		}
		openRules(e, active.col - e.t.col);
	}

	async function saveRules(rules: Rule[]) {
		const d = rulesDlg;
		if (!d) return;
		try {
			if (d.line) {
				if (!source.setLineRules) return;
				await source.setLineRules(d.line.rows, d.line.i0, d.line.i1, rules);
			} else {
				if (!source.setRules || !d.e) return;
				await source.setRules(d.e.t, d.i, rules);
			}
			rulesRev++;
			d.error = '';
		} catch (err) {
			d.error = String(err instanceof Error ? err.message : err);
		}
	}

	// rev : dépendance réactive (le tableau est relu après chaque enregistrement).
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	function currentRules(rev = 0): Rule[] {
		const d = rulesDlg;
		if (!d) return [];
		if (d.line) return (d.line.rows ? sheetRowRules : sheetColRules).get(d.line.i0) ?? [];
		if (!d.e) return [];
		// Le tableau a pu être rechargé : relire ses colonnes par identifiant.
		const e = d.e;
		const t = entries.find((x) => x.t.id === e.t.id)?.t ?? e.t;
		return parseRules(t.columns[d.i]?.rules) ?? [];
	}

	function addRule() {
		const d = rulesDlg;
		if (!d) return;
		const args = RULE_OPS.find((o) => o.op === d.op)?.args ?? 0;
		if (args > 0 && d.value.trim() === '') {
			d.error = 'Indiquez une valeur';
			return;
		}
		const r: Rule = { op: d.op, style: RULE_STYLES[d.style].style };
		if (args > 0) r.value = d.value.trim();
		if (args > 1) r.to = d.to.trim();
		saveRules([...currentRules(), r]);
		d.value = '';
		d.to = '';
	}

	function closeRules() {
		rulesDlg = null;
		wrap.focus();
	}

	// --- Graphiques -------------------------------------------------------------

	const CHART_W = 480;
	const CHART_H = 288;
	// Graphique sélectionné (cadre, poignées, volet) et redimensionnement en cours.
	let selChart = $state<number | null>(null);
	let chartMenu = $state<{ ch: SheetChart; x: number; y: number } | null>(null);
	const CHART_CLIP = '\u2063slitable-chart:';

	function selectChart(ch: SheetChart) {
		selChart = ch.id;
		panel = { kind: 'chart', id: ch.id };
		wrap.focus();
	}

	/** Colonnes du tableau source d'un graphique ou d'un TCD (autre feuille comprise). */
	function sourceColumns(name: string): { name: string; numeric: boolean }[] {
		const e = entries.find((x) => x.t.name.toLowerCase() === name.toLowerCase());
		if (e) return e.t.columns.map((c) => ({ name: c.name, numeric: c.type === 'integer' || c.type === 'decimal' || c.type === 'formula' }));
		const t = sources.find((x) => x.name.toLowerCase() === name.toLowerCase());
		return (t?.columns ?? []).map((c, i) => ({ name: c, numeric: ['integer', 'decimal', 'formula'].includes(t!.types[i]) }));
	}

	/** Position et taille d'un graphique, en pixels de la feuille (ancrage libre). */
	function chartBox(ch: SheetChart) {
		if (chartGesture?.id === ch.id) return chartGesture.box;
		return {
			x: colAxis.offset(ch.col) + (ch.config.offX ?? 0),
			y: rowTop(ch.row) + (ch.config.offY ?? 0),
			w: ch.config.width ?? CHART_W,
			h: ch.config.height ?? CHART_H
		};
	}

	let chartGesture = $state<{ id: number; box: { x: number; y: number; w: number; h: number } } | null>(null);

	/** Déplace (dir vide) ou redimensionne (n, s, e, w et coins) un graphique. */
	function startChartGesture(ev: MouseEvent, ch: SheetChart, dir = '') {
		if (ev.button !== 0 || !source.placeChart) return;
		ev.preventDefault();
		ev.stopPropagation();
		selectChart(ch);
		const b0 = chartBox(ch);
		const x0 = ev.clientX;
		const y0 = ev.clientY;
		chartGesture = { id: ch.id, box: { ...b0 } };
		const move = (e: MouseEvent) => {
			if (!chartGesture) return;
			const dx = e.clientX - x0;
			const dy = e.clientY - y0;
			const b = { ...b0 };
			if (!dir) {
				b.x = Math.max(0, b0.x + dx);
				b.y = Math.max(0, b0.y + dy);
			}
			if (dir.includes('e')) b.w = Math.max(160, b0.w + dx);
			if (dir.includes('s')) b.h = Math.max(120, b0.h + dy);
			if (dir.includes('w')) {
				const w = Math.max(160, b0.w - dx);
				b.x = Math.max(0, b0.x + b0.w - w);
				b.w = b0.x + b0.w - b.x;
			}
			if (dir.includes('n')) {
				const h = Math.max(120, b0.h - dy);
				b.y = Math.max(0, b0.y + b0.h - h);
				b.h = b0.y + b0.h - b.y;
			}
			chartGesture.box = b;
		};
		const up = () => {
			window.removeEventListener('mousemove', move);
			window.removeEventListener('mouseup', up);
			const g = chartGesture;
			if (!g || (g.box.x === b0.x && g.box.y === b0.y && g.box.w === b0.w && g.box.h === b0.h)) {
				chartGesture = null;
				return;
			}
			const b = g.box;
			const col = colAxis.indexAt(b.x);
			const row = rowAxis.indexAt(b.y);
			const place = source.placeChart!;
			writing = writing.then(() =>
				place(ch.id, row, col, Math.round(b.x - colAxis.offset(col)), Math.round(b.y - rowTop(row)), Math.round(b.w), Math.round(b.h)).then(
					() => {
						chartGesture = null;
					},
					(err) => {
						chartGesture = null;
						say(String(err?.message ?? err));
					}
				)
			);
		};
		window.addEventListener('mousemove', move);
		window.addEventListener('mouseup', up);
	}

	function onChartMenu(ev: MouseEvent, ch: SheetChart) {
		ev.preventDefault();
		ev.stopPropagation();
		selectChart(ch);
		const r = wrap.getBoundingClientRect();
		chartMenu = { ch, x: ev.clientX - r.left, y: ev.clientY - r.top };
	}

	async function copyChart(ch: SheetChart, cut: boolean) {
		chartMenu = null;
		try {
			await navigator.clipboard.writeText(CHART_CLIP + JSON.stringify(ch.config));
		} catch {
			chartClip = ch.config;
		}
		chartClip = ch.config;
		if (cut) await removeChart(ch.id);
		else say('Graphique copié : Ctrl+V pour le coller à la cellule active');
	}
	let chartClip = $state.raw<ChartConfig | null>(null);

	function pasteChart(cfg: ChartConfig) {
		if (!source.createChart) return;
		const create = source.createChart;
		const { row, col } = active;
		writing = writing.then(() => create(row, col, cfg).then(() => say('Graphique collé'), (err) => say(String(err?.message ?? err))));
	}

	/** Insère un graphique du tableau actif à sa droite (ruban Insertion). */
	export function chartActive(kind: ChartConfig['kind']) {
		const pv = pivotAt(active.row, active.col);
		if (pv && source.createChart) {
			// Graphique d'un tableau croisé : X = premier champ de lignes, séries = mesures.
			const head = pv.cells[0];
			const nk = Math.max(1, pv.config.rowFields?.length ?? 0);
			let series = head.slice(nk).filter((h) => !h.startsWith('Total'));
			if (kind === 'pie') series = series.slice(0, 1);
			const create = source.createChart;
			const name = pv.config.name ?? '';
			writing = writing.then(() =>
				create(pv.row, pv.col + head.length + 1, { source: name, kind, x: head[0], series }).then(
					() => say(`Graphique du tableau croisé « ${name} » inséré`),
					(err) => say(String(err?.message ?? err))
				)
			);
			return;
		}
		const e = tableAt(active.row, active.col);
		if (!e || !source.createChart) {
			say('Placez la cellule active dans un tableau pour insérer un graphique');
			return;
		}
		const cols = e.t.columns;
		const num = (c: Column) => c.type === 'integer' || c.type === 'decimal' || (c.type === 'formula' && c.formula !== undefined);
		const cur = cols[active.col - e.t.col];
		// Nuage de points : X est une colonne de nombres (l'active, sinon la première).
		const x = kind === 'scatter' ? (cur && num(cur) ? cur : (cols.find(num) ?? cols[0])) : cur && !num(cur) ? cur : (cols.find((c) => !num(c)) ?? cols[0]);
		let series = cols.filter((c) => c !== x && num(c)).map((c) => c.name);
		if (kind === 'pie') series = series.slice(-1);
		if (!series.length) {
			say('Le tableau n’a pas de colonne de nombres à représenter');
			return;
		}
		const create = source.createChart;
		writing = writing.then(() =>
			create(e.t.row, e.lastCol + 2, { source: e.t.name, kind, x: x.name, series }).then(
				() => say('Graphique inséré : double-clic pour le modifier, glissez-le pour le déplacer'),
				(err) => say(String(err?.message ?? err))
			)
		);
	}

	function editChart(ch: SheetChart) {
		selectChart(ch);
	}
	async function removeChart(id: number) {
		chartMenu = null;
		if (selChart === id) selChart = null;
		if (panel?.kind === 'chart' && panel.id === id) panel = null;
		await source.deleteChart?.(id);
		wrap.focus();
	}


	// --- Tableau croisé dynamique ---------------------------------------------

	const pivotAt = (r: number, c: number) =>
		pivots.find((p) => !p.error && r >= p.row && r < p.row + p.cells.length && c >= p.col && c < p.col + p.cells[0].length);

	// Volet latéral : champs du TCD ou mise en forme du graphique.
	let panel = $state<{ kind: 'pivot' | 'chart'; id: number } | null>(null);
	let openPivotOnLoad = false;

	/** Crée un tableau croisé à partir du tableau actif, ou ouvre le volet de celui sous la cellule active. */
	export function pivotActive() {
		const p = pivotAt(active.row, active.col) ?? pivots.find((p) => p.row === active.row && p.col === active.col);
		if (p) {
			panel = { kind: 'pivot', id: p.id };
			return;
		}
		const e = tableAt(active.row, active.col);
		if (!e || !source.createPivot) {
			say('Placez la cellule active dans un tableau pour créer un tableau croisé');
			return;
		}
		const k = e.t.columns[active.col - e.t.col];
		// Comme Excel : la colonne active en lignes, la dernière colonne de
		// nombres en valeurs ; le volet s'ouvre pour la suite.
		const num = e.t.columns.findLast((c) => !c.formula && (c.type === 'integer' || c.type === 'decimal') && c !== k);
		const first = k && !k.formula ? k.name : (e.t.columns.find((c) => !c.formula)?.name ?? '');
		const cfg: PivotConfig = { source: e.t.name, rowFields: [first], values: [num ? { field: num.name, agg: 'sum' } : { agg: 'count' }] };
		openPivotOnLoad = true;
		const create = source.createPivot;
		writing = writing.then(() => create(cfg).then(() => undefined, (err) => say(String(err?.message ?? err))));
	}

	/** Supprime les doublons du tableau de la cellule active (ruban Données). */
	let dedupDlg = $state<{ e: Entry; on: boolean[]; count: number | null; error: string } | null>(null);

	export function dedupActive() {
		const e = tableAt(active.row, active.col);
		if (!e || !source.removeDuplicates) {
			say('Placez la cellule active dans un tableau');
			return;
		}
		// Comme Excel : toutes les colonnes cochées au départ.
		dedupDlg = { e, on: e.t.columns.map((c) => !c.formula), count: null, error: '' };
		previewDedup();
	}

	let dedupSeq = 0;
	async function previewDedup() {
		const d = dedupDlg;
		if (!d || !source.duplicatesPreview) return;
		const cols = d.on.flatMap((on, i) => (on ? [i] : []));
		const seq = ++dedupSeq;
		d.count = null;
		if (!cols.length) {
			d.count = 0;
			return;
		}
		try {
			const n = await source.duplicatesPreview(d.e.t, cols);
			if (seq === dedupSeq && dedupDlg === d) d.count = n;
		} catch (err) {
			d.error = String(err instanceof Error ? err.message : err);
		}
	}

	async function applyDedup() {
		const d = dedupDlg;
		if (!d || !source.removeDuplicates) return;
		const cols = d.on.flatMap((on, i) => (on ? [i] : []));
		try {
			const n = await source.removeDuplicates(d.e.t, cols);
			dedupDlg = null;
			wrap.focus();
			say(n ? `${n} ligne${n > 1 ? 's' : ''} en double supprimée${n > 1 ? 's' : ''} (Ctrl+Z pour annuler)` : 'Aucun doublon');
		} catch (err) {
			d.error = String(err instanceof Error ? err.message : err);
		}
	}

	// --- Ruban Formules ---------------------------------------------------------

	let fnDlg = $state<{ search: string; sel: string | null } | null>(null);

	/** Commence une formule dans la cellule active (Insérer une fonction, REFERENCE). */
	export function startFormula(text: string) {
		fnDlg = null;
		wrap.focus();
		startEdit(true, text);
		if (input) onFormulaInput(input, 'cell');
	}

	/** Somme automatique : SOMME de la colonne du tableau situé juste au-dessus. */
	export function autoSum() {
		const { row, col } = active;
		const inTbl = tableAt(row, col);
		const above = entries.filter((e) => col >= e.t.col && col <= e.lastCol && e.last < row).sort((a, b) => b.last - a.last)[0];
		if (!inTbl && above) {
			startFormula(`=SOMME(${above.t.name}[${above.t.columns[col - above.t.col].name}])`);
			return;
		}
		startFormula('=SOMME(');
	}

	export function openFunctions() {
		fnDlg = { search: '', sel: null };
	}

	/** Fonctions du dialogue : nom (français ou anglais), catégorie ou description. */
	function shownFunctions() {
		const q = fnDlg?.search.trim().toLowerCase() ?? '';
		const all = catalog?.functions ?? [];
		const hit = (f: FunctionDoc) => [f.name, f.other, f.category ?? '', f.description ?? ''].some((s) => s.toLowerCase().includes(q));
		return (q ? all.filter(hit) : all).slice(0, 200);
	}

	/** Tri du tableau de la cellule active (ruban Données). */
	export function sortActive(desc: boolean) {
		const e = tableAt(active.row, active.col);
		if (!e || !source.sortTable) {
			say('Placez la cellule active dans la colonne d’un tableau pour trier');
			return;
		}
		const sort = source.sortTable;
		menuAction(() => sort(e.t, active.col - e.t.col, desc));
	}

	/** Lignes (ou colonnes) entières couvertes par la sélection, comme Excel les insère ou les supprime. */
	function selectionLines(): { r0: number; r1: number; c0: number; c1: number; rows: boolean; cols: boolean } {
		const { r0, r1, c0, c1 } = selection();
		return { r0, r1, c0, c1, rows: c0 === 0 && c1 >= lastCol(), cols: r0 === 0 && r1 >= lastRow() };
	}

	/** Insère ou supprime des lignes ou colonnes entières pour la sélection. */
	export function sheetLines(op: LinesOp) {
		if (!source.sheetLines) return;
		const { r0, r1, c0, c1 } = selection();
		const lines = source.sheetLines;
		const args: Record<LinesOp, [number, number]> = {
			insertRows: [r0, r1 - r0 + 1],
			deleteRows: [r0, r1],
			insertCols: [c0, c1 - c0 + 1],
			deleteCols: [c0, c1]
		};
		menuAction(() => lines(op, ...args[op]));
	}

	// Insérer… / Supprimer… (comme la boîte d'Excel) : lignes ou colonnes entières.
	let linesDlg = $state<{ x: number; y: number; insert: boolean; choice: 'rows' | 'cols' } | null>(null);

	function openLinesDialog(insert: boolean, x?: number, y?: number) {
		const sel = selectionLines();
		if (sel.rows || sel.cols) {
			sheetLines(insert ? (sel.rows ? 'insertRows' : 'insertCols') : sel.rows ? 'deleteRows' : 'deleteCols');
			return;
		}
		menu = freeMenu = null;
		const px = x ?? MARGIN + colAxis.offset(active.col) - scroller.scrollLeft + 20;
		const py = y ?? TOP_H + rowTop(active.row + 1) - vs.top + 4;
		linesDlg = { x: px, y: py, insert, choice: 'rows' };
	}

	function applyLinesDialog() {
		const d = linesDlg;
		linesDlg = null;
		if (!d) return;
		sheetLines(d.insert ? (d.choice === 'rows' ? 'insertRows' : 'insertCols') : d.choice === 'rows' ? 'deleteRows' : 'deleteCols');
	}

	/** Insérer ou supprimer la ligne ou la colonne de la cellule active (ruban Accueil › Cellules). */
	export function cellsActive(op: 'insertRow' | 'deleteRow' | 'insertColumn' | 'deleteColumn') {
		const e = tableAt(active.row, active.col);
		const sel = selectionLines();
		// Hors tableau, ou lignes / colonnes entières sélectionnées : celles de la feuille.
		if (!e || ((op === 'insertRow' || op === 'deleteRow') && sel.rows) || ((op === 'insertColumn' || op === 'deleteColumn') && sel.cols)) {
			sheetLines(({ insertRow: 'insertRows', deleteRow: 'deleteRows', insertColumn: 'insertCols', deleteColumn: 'deleteCols' } as const)[op]);
			return;
		}
		const n = active.row - e.t.row - 1; // -1 : ligne d'en-tête
		const i = active.col - e.t.col;
		if (op === 'deleteRow' && n < 0) {
			say('La ligne d’en-tête ne se supprime pas : supprimez le tableau ou ses colonnes');
			return;
		}
		const fn = {
			insertRow: source.insertRow && (() => source.insertRow!(e.t, Math.max(0, n))),
			deleteRow: source.deleteRow && (() => source.deleteRow!(e.t, n)),
			insertColumn: source.insertColumn && (() => source.insertColumn!(e.t, i)),
			deleteColumn: source.deleteColumn && (() => source.deleteColumn!(e.t, i))
		}[op];
		menuAction(fn);
	}

	/** Échap ferme le dialogue ouvert, où que soit le focus. */
	function onGlobalKey(e: KeyboardEvent) {
		if (e.key !== 'Escape') return;
		const open = filterDlg || rulesDlg || dedupDlg || fnDlg || menu || freeMenu || typeMenu || chartMenu || linesDlg;
		if (!open) return;
		filterDlg = rulesDlg = dedupDlg = fnDlg = linesDlg = null;
		menu = freeMenu = typeMenu = chartMenu = null;
		wrap.focus();
	}

	/** Sélectionne une plage (scripts : ui.select ; à partir de 0). */
	export function selectRange(r0: number, c0: number, r1: number, c1: number) {
		moveTo(r0, c0);
		anchor = { row: clamp(r0, 0, SHEET_ROWS - 1), col: clamp(c0, 0, SHEET_COLS - 1) };
		active = { row: clamp(r1, 0, SHEET_ROWS - 1), col: clamp(c1, 0, SHEET_COLS - 1) };
		ensureVisible(r0, c0);
		refreshFx();
		refreshStatus();
		schedule();
	}

	/** Insertion › Bouton : bouton d'action à la cellule active. */
	export function insertButton() {
		if (!source.createButton) return;
		buttonsLayer?.openEditor();
	}

	function runButton(b: SheetButton) {
		if (!source.runButton) return;
		say(`« ${b.config.label} » : exécution…`);
		source.runButton(b.id).then(
			(msg) => say(typeof msg === 'string' ? msg : ''),
			(err) => say(String(err?.message ?? err))
		);
	}

	/** Sélectionne un tableau ou un tableau croisé de la feuille par son nom (onglet SQL). */
	export function revealObject(name: string): boolean {
		const lower = name.toLowerCase();
		const e = entries.find((x) => x.t.name.toLowerCase() === lower);
		if (e) {
			moveTo(e.t.row, e.t.col);
			anchor = { row: e.t.row, col: e.t.col };
			active = { row: e.last, col: e.lastCol };
			ensureVisible(e.t.row, e.t.col);
			refreshFx();
			schedule();
			return true;
		}
		const p = pivots.find((x) => (x.config.name ?? '').toLowerCase() === lower);
		if (p) {
			moveTo(p.row, p.col);
			return true;
		}
		return false;
	}

	// --- Audit de formules : flèches des antécédents et des dépendants --------

	let arrows = $state<{ row: number; col: number; pre: DepTarget[]; dep: DepTarget[] } | null>(null);
	const depLabel = (d: DepTarget) => (d.kind === 'column' ? `${d.table}[${d.column}]` : d.kind === 'table' ? (d.table ?? '') : d.name || 'cellule libre');

	/** Formules › Antécédents / Dépendants ; 'clear' efface les flèches. */
	export async function auditActive(kind: 'pre' | 'dep' | 'clear') {
		if (kind === 'clear' || !source.dependencies) {
			arrows = null;
			schedule();
			return;
		}
		const { row, col } = active;
		try {
			const d = await source.dependencies(row, col);
			const list = kind === 'pre' ? d.precedents : d.dependents;
			const keep = arrows && arrows.row === row && arrows.col === col ? arrows : { row, col, pre: [], dep: [] };
			arrows = { ...keep, [kind]: list };
			const here = list.filter((x) => x.sheet === sheetIdOf());
			const away = list.filter((x) => x.sheet !== sheetIdOf());
			const what = kind === 'pre' ? 'Antécédents' : 'Dépendants';
			say(
				list.length === 0
					? `${what} : aucun${d.subject ? ` pour ${d.subject}` : ''}`
					: `${what} : ${list.map(depLabel).join(', ')}${away.length ? ` (${away.length} sur d'autres feuilles)` : ''}${here.length ? ' · Échap pour effacer les flèches' : ''}`
			);
			schedule();
		} catch (err) {
			say(String(err instanceof Error ? err.message : err));
		}
	}

	/** Feuille affichée (les cibles des autres feuilles n'ont pas de flèche). */
	function sheetIdOf(): number {
		return data?.sheet ?? 1;
	}

	/** Ferme les menus, dialogues et volets (changement de feuille ou de vue). */
	export function closeOverlays() {
		menu = null;
		freeMenu = null;
		typeMenu = null;
		filterDlg = null;
		arrows = null;
		rulesDlg = null;
		dedupDlg = null;
		fnDlg = null;
		chartMenu = null;
		selChart = null;
		panel = null;
	}

	/** Erreur de formule de la cellule (code et formule), ou null. */
	function errorAt(r: number, c: number): { code: string; formula: string } | null {
		const e = tableAt(r, c);
		if (e) {
			const i = c - e.t.col;
			const col = e.t.columns[i];
			if (r === e.t.row || !col.formula) return null;
			const v = e.cache.row(r - e.t.row - 1)?.cells[i];
			return isErrorValue(v) ? { code: v as string, formula: col.formula } : null;
		}
		const f = shown.get(key(r, c));
		return f?.error ? { code: f.text, formula: cells.get(key(r, c)) ?? '' } : null;
	}

	function refreshFx() {
		nameBox = cellName(active.row, active.col);
		const err = errorAt(active.row, active.col);
		if (err) message = explainError(err.code, err.formula) || message;
		const pv = pivotAt(active.row, active.col);
		if (pv) panel = { kind: 'pivot', id: pv.id };
		else if (panel?.kind === 'pivot' && !openPivotOnLoad) panel = null;
		const e = tableAt(active.row, active.col);
		const label = (r: number, c: number) => {
			const k = e!.t.columns[c - e!.t.col];
			return r === e!.t.row ? k.name : `${k.name}[${r - e!.t.row}]`;
		};
		const sel = selection();
		const multi = sel.r0 !== sel.r1 || sel.c0 !== sel.c1;
		if (e) {
			const inside = multi && inTable(e, sel.r0, sel.c0) && inTable(e, sel.r1, sel.c1);
			tableBox = { name: e.t.name, col: inside ? `${label(sel.r0, sel.c0)}:${label(sel.r1, sel.c1)}` : label(active.row, active.col) };
		} else tableBox = null;
		if (multi && !e) nameBox = `${sel.r1 - sel.r0 + 1}L × ${sel.c1 - sel.c0 + 1}C`;
		if (fxEditing) return;
		fxText = cellInfo(active.row, active.col).value;
	}

	function say(m: string) {
		message = m;
		refreshStatus();
	}

	// --- Défilement -------------------------------------------------------

	/** Défilement horizontal piloté par la barre de la ligne des onglets. */
	export function scrollLeftTo(x: number, atEnd = false) {
		if (atEnd && holdCols) growAtEnd(false);
		scroller.scrollLeft = x;
		updateExtent();
		schedule();
	}

	// Barre verticale (native) tirée : repérée au clic dans sa zone.
	function onScrollbarDown(e: PointerEvent) {
		if (e.offsetX < scroller.clientWidth) return;
		holdRows = rowAxis.indexAt(vs.content);
		const up = () => {
			holdRows = 0;
			window.removeEventListener('pointerup', up);
			updateExtent();
		};
		window.addEventListener('pointerup', up);
	}

	function onScroll() {
		if (holdRows && scroller.scrollTop + scroller.clientHeight >= scroller.scrollHeight - 2) growAtEnd(true);
		if (!vs.inSync(scroller.scrollTop)) vs.fromNative(scroller.scrollTop);
		updateExtent();
		schedule();
	}

	function onWheel(e: WheelEvent) {
		if (e.ctrlKey) return;
		const unit = e.deltaMode === 1 ? ROW_H : e.deltaMode === 2 ? vs.view : 1;
		let dy = e.deltaY * unit;
		let dx = e.deltaX * unit;
		if (e.shiftKey && dx === 0) [dx, dy] = [dy, 0];
		e.preventDefault();
		// Molette de souris (crans de ±100 px ou défilement par lignes) : trois
		// lignes par cran, calées sur le haut des lignes, comme Excel. Le pavé
		// tactile (petits pas) garde un défilement au pixel près.
		const notch = e.deltaMode === 1 || (e.deltaMode === 0 && Math.abs(e.deltaY) >= 50 && e.deltaY % 1 === 0);
		if (dy && notch) {
			const steps = e.deltaMode === 1 ? Math.round(e.deltaY) : Math.sign(dy) * Math.max(1, Math.round(Math.abs(e.deltaY) / 100)) * 3;
			const first = rowAxis.indexAt(vs.top);
			const base = rowTop(first) < vs.top && steps < 0 ? first + 1 : first;
			dy = rowTop(clamp(base + steps, 0, SHEET_ROWS - 1)) - vs.top;
		}
		if (dx && notch && e.shiftKey) {
			const steps = Math.sign(dx) * 3;
			const first = colAxis.indexAt(scroller.scrollLeft);
			dx = colAxis.offset(clamp(first + steps, 0, SHEET_COLS - 1)) - scroller.scrollLeft;
		}
		if (dy) {
			// Défiler vers le bas agrandit la feuille d'autant.
			if (dy > 0) vs.content = Math.min(rowAxis.total, Math.max(vs.content, vs.top + dy + 2 * vs.view));
			vs.by(dy);
			updateExtent();
			scroller.scrollTop = vs.toNative();
		}
		if (dx) {
			// Vers la droite, la feuille s'agrandit d'autant avant de défiler.
			if (dx > 0 && spacer) {
				const want = colAxis.indexAt(scroller.scrollLeft + dx + viewW) + 2;
				spacer.style.width = `${MARGIN + colAxis.offset(Math.min(SHEET_COLS, Math.max(extentCols, want)))}px`;
			}
			scroller.scrollLeft += dx;
			updateExtent();
		}
		schedule();
	}

	function syncNative() {
		scroller.scrollTop = vs.toNative();
		schedule();
	}

	function ensureVisible(row: number, col: number) {
		updateExtent();
		const top = rowTop(row);
		if (top < vs.top) vs.to(top);
		else if (top + rh(row) > vs.top + vs.view) vs.to(top + rh(row) - vs.view);
		const left = scroller.scrollLeft;
		const cw = viewW - MARGIN;
		const x0 = colAxis.offset(col);
		const x1 = x0 + colAxis.size(col);
		if (x0 < left) scroller.scrollLeft = x0;
		else if (x1 > left + cw) scroller.scrollLeft = x1 - cw;
		syncNative();
	}

	// --- Sélection et clavier ---------------------------------------------

	// Saisie d'une ligne avec Tab : Entrée revient sous la cellule de départ, comme Excel.
	let tabStart: number | null = null;
	function tabMove(back: boolean) {
		const start = tabStart ?? active.col;
		moveTo(active.row, active.col + (back ? -1 : 1));
		tabStart = start;
	}
	function enterMove(back: boolean) {
		const col = tabStart ?? active.col;
		moveTo(active.row + (back ? -1 : 1), col);
	}

	function moveTo(row: number, col: number, extend = false) {
		tabStart = null;
		active = { row: clamp(row, 0, SHEET_ROWS - 1), col: clamp(col, 0, SHEET_COLS - 1) };
		if (!extend) anchor = { ...active };
		message = '';
		ensureVisible(active.row, active.col);
		refreshFx();
		refreshStatus();
	}

	// Ctrl + flèche, comme Excel : saute au bord du bloc de données.
	function jump(row: number, col: number, dr: number, dc: number): [number, number] {
		const filled = (r: number, c: number) => !!tableAt(r, c) || cells.has(key(r, c));
		if (dr !== 0) {
			const e = entries.find((e) => col >= e.t.col && col <= e.lastCol && (dr > 0 ? e.last > row : e.t.row < row));
			const edges = entries
				.filter((e) => col >= e.t.col && col <= e.lastCol)
				.flatMap((e) => [e.t.row, e.last])
				.filter((r) => (dr > 0 ? r > row : r < row))
				.sort((a, b) => (dr > 0 ? a - b : b - a));
			if (e || edges.length) return [edges[0] ?? row, col];
			for (let r = row + dr; r >= 0 && r < Math.min(SHEET_ROWS, row + 10000) && r > row - 10000; r += dr) if (filled(r, col)) return [r, col];
			return [dr > 0 ? SHEET_ROWS - 1 : 0, col];
		}
		const edges = entries
			.filter((e) => row >= e.t.row && row <= e.last)
			.flatMap((e) => [e.t.col, e.lastCol])
			.filter((c) => (dc > 0 ? c > col : c < col))
			.sort((a, b) => (dc > 0 ? a - b : b - a));
		if (edges.length) return [row, edges[0]];
		return [row, dc > 0 ? SHEET_COLS - 1 : 0];
	}

	function cellAt(e: MouseEvent): { row: number; col: number } | null {
		const r = scroller.getBoundingClientRect();
		const x = e.clientX - r.left;
		const y = e.clientY - r.top;
		if (x < MARGIN || y < TOP_H || x >= scroller.clientWidth || y >= scroller.clientHeight) return null;
		const col = colAxis.indexAt(x - MARGIN + scroller.scrollLeft);
		const row = Math.min(SHEET_ROWS - 1, rowAxis.indexAt(y - TOP_H + vs.top));
		return { row, col };
	}

	// --- Largeur des colonnes : glisser une séparation du bandeau du haut ----

	let resizing: { col: number; startX: number; startW: number; width: number } | null = null;
	let rowResizing: { row: number; startY: number; startH: number; height: number } | null = null;

	/** Ligne dont le bord bas est sous la souris (marge de gauche), ou null. */
	function rowBoundaryAt(e: MouseEvent): number | null {
		const r = scroller.getBoundingClientRect();
		const x = e.clientX - r.left;
		const y = e.clientY - r.top;
		if (x >= MARGIN || y <= TOP_H) return null;
		const py = y - TOP_H + vs.top;
		const row = rowAxis.indexAt(py);
		if (rowTop(row + 1) - py <= 3) return row;
		if (py - rowTop(row) <= 3 && row > 0) return row - 1;
		return null;
	}

	/** Colonne dont le bord droit est sous la souris (bandeau du haut), ou null. */
	function boundaryAt(e: MouseEvent): number | null {
		const r = scroller.getBoundingClientRect();
		const x = e.clientX - r.left;
		const y = e.clientY - r.top;
		if (y >= TOP_H || x <= MARGIN) return null;
		const px = x - MARGIN + scroller.scrollLeft;
		const c = colAxis.indexAt(px);
		const right = colAxis.offset(c + 1);
		const left = colAxis.offset(c);
		if (right - px <= 4) return c;
		if (px - left <= 4 && c > 0) return c - 1;
		return null;
	}

	/** Largeur ajustée au contenu affiché de la colonne c (double-clic sur le
	 * trait de droite de son en-tête, comme Excel) : en-têtes des tableaux,
	 * lignes visibles, cellules libres et tableaux croisés. */
	function fitWidth(c: number): number {
		let w = 0;
		const { first, last } = visibleRows();
		for (const e of entries) {
			if (c < e.t.col || c > e.lastCol) continue;
			const i = c - e.t.col;
			const col = e.t.columns[i];
			const icon = (col.filter ? '⧩ ' : '') + (typeIconName(col.type) ? ' ▾' : typeIcon(col.type) + ' ▾') + (e.t.key === i ? '🔑 ' : '');
			measurer ??= document.createElement('canvas').getContext('2d');
			if (measurer) measurer.font = FONT_SMALL;
			const iconW = (measurer?.measureText(icon).width ?? 0) + (typeIconName(col.type) ? 13 : 0);
			w = Math.max(w, textWidth(col.name, true) + iconW + 4);
			const d = visibleData(e, first, last);
			if (d) for (let k = d.first; k <= d.last; k++) w = Math.max(w, textWidth(e.cache.row(k)?.text[i] ?? ''));
		}
		for (const [k, raw] of cells) {
			const [r, cc] = k.split(',').map(Number);
			if (cc === c) w = Math.max(w, textWidth(shown.get(k)?.text ?? raw, !!styleAt(r, cc, undefined)?.b));
		}
		for (const p of pivots) {
			const j = c - p.col;
			if (p.error || j < 0 || j >= (p.cells[0]?.length ?? 0)) continue;
			p.cells.forEach((r, i) => (w = Math.max(w, textWidth(r[j] ?? '', i === 0 || i === p.cells.length - 1))));
		}
		return w ? clamp(Math.ceil(w) + 14, 24, MAX_FIT) : DEF_W;
	}

	function autoFitColumn(c: number) {
		const setWidth = source.setColumnWidth;
		if (!setWidth) return;
		const width = fitWidth(c);
		if (width === colAxis.size(c)) return;
		userWidths.set(c, width);
		rebuildAxis();
		updateExtent();
		schedule();
		writing = writing.then(() => setWidth(c, width).then(() => undefined, (err) => say(String(err?.message ?? err))));
	}

	/** La souris est-elle sur la bordure de la sélection (poignée de déplacement) ? */
	function onSelectionEdge(e: MouseEvent): boolean {
		if (!source.moveTable) return false;
		const r = scroller.getBoundingClientRect();
		const x = e.clientX - r.left;
		const y = e.clientY - r.top;
		if (x < MARGIN || y < TOP_H) return false;
		const { r0, r1, c0, c1 } = selection();
		const cx = (c: number) => MARGIN + colAxis.offset(c) - scroller.scrollLeft;
		const ry = (r: number) => TOP_H + rowTop(r) - vs.top;
		const left = cx(c0);
		const right = cx(c1) + colAxis.size(c1);
		const top = ry(r0);
		const bottom = ry(r1) + rh(r1);
		const near = (v: number, edge: number) => Math.abs(v - edge) <= 3;
		const inX = x >= left - 3 && x <= right + 3;
		const inY = y >= top - 3 && y <= bottom + 3;
		return (inY && (near(x, left) || near(x, right))) || (inX && (near(y, top) || near(y, bottom)));
	}

	async function dropMove() {
		const m = moving;
		moving = null;
		schedule();
		if (!m?.to) return;
		const dr = m.to.row - m.from.row;
		const dc = m.to.col - m.from.col;
		if (!dr && !dc) return;
		const { r0, r1, c0, c1 } = m.sel;
		const e = entries.find((e) => e.t.row === r0 && e.last === r1 && c0 >= e.t.col && c1 <= e.lastCol);
		if (!e) {
			// Hors tableau : les cellules libres de la plage se déplacent (comme Excel).
			const touched = entries.some((x) => x.t.row <= r1 && x.last >= r0 && x.t.col <= c1 && x.lastCol >= c0);
			if (touched || !source.moveCells) {
				say('Pour déplacer, sélectionnez un tableau entier (Ctrl+A) ou des colonnes entières, en-tête compris (Ctrl+Espace)');
				return;
			}
			const mv = source.moveCells;
			writing = writing.then(async () => {
				try {
					await mv(r0, c0, r1, c1, dr, dc);
					moveTo(r0 + dr, c0 + dc);
					anchor = { row: r1 + dr, col: c1 + dc };
					message = 'Cellules déplacées (Ctrl+Z pour annuler)';
				} catch (err) {
					message = String(err instanceof Error ? err.message : err);
				}
				refreshStatus();
				schedule();
			});
			return;
		}
		const whole = c0 === e.t.col && c1 === e.lastCol;
		const run = whole
			? source.moveTable && (() => source.moveTable!(e.t, r0 + dr, c0 + dc))
			: source.extractColumns && (() => source.extractColumns!(e.t, c0 - e.t.col, c1 - e.t.col, r0 + dr, c0 + dc));
		if (!run) return;
		writing = writing.then(async () => {
			try {
				await run();
				moveTo(r0 + dr, c0 + dc);
				anchor = { row: r1 + dr, col: c1 + dc };
				message = whole ? `Tableau « ${e.t.name} » déplacé (Ctrl+Z pour annuler)` : 'Colonnes déplacées (Ctrl+Z pour annuler)';
			} catch (err) {
				message = String(err instanceof Error ? err.message : err);
			}
			refreshStatus();
			schedule();
		});
	}

	// --- Poignée de recopie ------------------------------------------------------

	type Range = { r0: number; r1: number; c0: number; c1: number };
	let filling = $state<{ sel: Range; to?: Range } | null>(null);

	/** Position (écran) du coin bas droit de la sélection. */
	function fillHandle() {
		const { r1, c1 } = selection();
		return { x: MARGIN + colAxis.offset(c1 + 1) - scroller.scrollLeft, y: TOP_H + rowTop(r1 + 1) - vs.top };
	}

	function onFillHandle(e: MouseEvent): boolean {
		if (editing || !source.paste) return false;
		const r = scroller.getBoundingClientRect();
		const h = fillHandle();
		return Math.abs(e.clientX - r.left - h.x) <= 5 && Math.abs(e.clientY - r.top - h.y) <= 5;
	}

	/** Zone étendue par la poignée jusqu'à la cellule visée (une seule direction). */
	function fillTarget(sel: Range, cell: { row: number; col: number }): Range | undefined {
		const down = cell.row - sel.r1;
		const up = sel.r0 - cell.row;
		const right = cell.col - sel.c1;
		const left = sel.c0 - cell.col;
		const best = Math.max(down, up, right, left);
		if (best <= 0) return undefined;
		if (best === down) return { ...sel, r1: cell.row };
		if (best === up) return { ...sel, r0: cell.row };
		if (best === right) return { ...sel, c1: cell.col };
		return { ...sel, c0: cell.col };
	}

	/** Remplit to à partir de sel (séries, dates, mois… voir fill.ts). */
	function doFill(sel: Range, to: Range) {
		if (!source.paste) return;
		const val = (r: number, c: number) => cellInfo(r, c).value;
		let row = 0;
		let col = 0;
		let block: string[][] = [];
		if (to.r1 > sel.r1 || to.r0 < sel.r0) {
			const down = to.r1 > sel.r1;
			const n = down ? to.r1 - sel.r1 : sel.r0 - to.r0;
			const cols: string[][] = [];
			for (let c = sel.c0; c <= sel.c1; c++) {
				const src: string[] = [];
				for (let r = sel.r0; r <= sel.r1; r++) src.push(val(r, c));
				cols.push(down ? fillSeries(src, n) : fillSeries([...src].reverse(), n).reverse());
			}
			block = Array.from({ length: n }, (_, k) => cols.map((x) => x[k]));
			row = down ? sel.r1 + 1 : to.r0;
			col = sel.c0;
		} else {
			const right = to.c1 > sel.c1;
			const n = right ? to.c1 - sel.c1 : sel.c0 - to.c0;
			for (let r = sel.r0; r <= sel.r1; r++) {
				const src: string[] = [];
				for (let c = sel.c0; c <= sel.c1; c++) src.push(val(r, c));
				block.push(right ? fillSeries(src, n) : fillSeries([...src].reverse(), n).reverse());
			}
			row = sel.r0;
			col = right ? sel.c1 + 1 : to.c0;
		}
		const paste = source.paste;
		writing = writing.then(async () => {
			try {
				await paste(row, col, block);
				message = 'Recopie incrémentée (Ctrl+Z pour annuler)';
			} catch (err) {
				message = String(err instanceof Error ? err.message : err);
			}
			anchor = { row: to.r0, col: to.c0 };
			active = { row: to.r1, col: to.c1 };
			refreshFx();
			refreshStatus();
			schedule();
		});
	}

	/** Double-clic sur la poignée : recopie jusqu'en bas du tableau voisin. */
	function fillDown() {
		const sel = selection();
		const near = entries.find((e) => e.t.col <= sel.c1 + 1 && e.lastCol >= sel.c0 - 1 && e.t.row <= sel.r1 && e.last > sel.r1);
		if (!near) return;
		doFill(sel, { ...sel, r1: near.last });
	}

	// Sélection de lignes ou de colonnes entières par la marge ou le bandeau.
	let headerDrag: 'rows' | 'cols' | null = null;

	/** Ligne (marge de gauche) ou colonne (bandeau du haut) sous la souris. */
	function headerAt(e: MouseEvent): { kind: 'rows'; row: number } | { kind: 'cols'; col: number } | null {
		const r = scroller.getBoundingClientRect();
		const x = e.clientX - r.left;
		const y = e.clientY - r.top;
		if (x < MARGIN && y >= TOP_H) return { kind: 'rows', row: clamp(rowAxis.indexAt(y - TOP_H + vs.top), 0, SHEET_ROWS - 1) };
		if (y < TOP_H && x >= MARGIN) return { kind: 'cols', col: colAxis.indexAt(x - MARGIN + scroller.scrollLeft) };
		return null;
	}

	/** Dernière ligne et colonne de la zone affichée (la sélection entière s'y arrête). */
	function lastRow() {
		return Math.max(0, rowAxis.indexAt(vs.content) - 1);
	}
	function lastCol() {
		return Math.max(0, extentCols - 1);
	}

	// Sélection de lignes ou colonnes entières (marge ou bandeau) : tant que la
	// sélection n'a pas changé, la mise en forme vaut pour les lignes ou
	// colonnes entières.
	let wholeSel: { kind: 'rows' | 'cols'; anchor: { row: number; col: number }; active: { row: number; col: number } } | null = null;
	function wholeKind(): 'rows' | 'cols' | null {
		const w = wholeSel;
		if (!w) return null;
		const same = (a: { row: number; col: number }, b: { row: number; col: number }) => a.row === b.row && a.col === b.col;
		return same(w.anchor, anchor) && same(w.active, active) ? w.kind : null;
	}

	function selectRows(r0: number, r1: number) {
		anchor = { row: r0, col: 0 };
		active = { row: r1, col: lastCol() };
		wholeSel = { kind: 'rows', anchor: { ...anchor }, active: { ...active } };
		refreshFx();
		refreshStatus();
		schedule();
	}
	function selectCols(c0: number, c1: number) {
		anchor = { row: 0, col: c0 };
		active = { row: lastRow(), col: c1 };
		wholeSel = { kind: 'cols', anchor: { ...anchor }, active: { ...active } };
		refreshFx();
		refreshStatus();
		schedule();
	}

	/** La cellule est-elle dans la sélection en cours ? */
	function inSelection(row: number, col: number) {
		const { r0, r1, c0, c1 } = selection();
		return row >= r0 && row <= r1 && col >= c0 && col <= c1;
	}

	function onMouseDown(e: MouseEvent) {
		if (e.button === 0 && onFillHandle(e)) {
			e.preventDefault();
			if (e.detail === 2) {
				fillDown();
				return;
			}
			filling = { sel: selection() };
			return;
		}
		if (selChart !== null) {
			selChart = null;
			if (panel?.kind === 'chart') panel = null;
		}
		const hd = boundaryAt(e) === null && rowBoundaryAt(e) === null ? headerAt(e) : null;
		if (hd && e.button !== 1) {
			e.preventDefault();
			if (editing) commit();
			wrap.focus();
			// Clic droit dans la sélection : on la garde.
			if (e.button === 2 && (hd.kind === 'rows' ? inSelection(hd.row, active.col) : inSelection(active.row, hd.col))) return;
			if (hd.kind === 'rows') selectRows(e.shiftKey ? anchor.row : hd.row, hd.row);
			else selectCols(e.shiftKey ? anchor.col : hd.col, hd.col);
			headerDrag = e.button === 0 ? hd.kind : null;
			return;
		}
		if (e.button === 0 && !editing && onSelectionEdge(e)) {
			const cell = cellAt(e);
			if (cell) {
				e.preventDefault();
				moving = { sel: selection(), from: cell };
				return;
			}
		}
		const rb = rowBoundaryAt(e);
		if (rb !== null && source.setRowHeight) {
			e.preventDefault();
			if (e.detail === 2) {
				// Double-clic : hauteur par défaut.
				userHeights.delete(rb);
				rebuildRows();
				updateExtent();
				schedule();
				const set = source.setRowHeight;
				writing = writing.then(() => set(rb, 0).then(() => undefined, (err) => say(String(err?.message ?? err))));
				return;
			}
			rowResizing = { row: rb, startY: e.clientY, startH: rh(rb), height: rh(rb) };
			return;
		}
		const b = boundaryAt(e);
		if (b !== null && source.setColumnWidth) {
			e.preventDefault();
			resizing = { col: b, startX: e.clientX, startW: colAxis.size(b), width: colAxis.size(b) };
			return;
		}
		const cell = cellAt(e);
		if (!cell) return;
		if (e.button === 0 && formulaMode && insertRef(cell)) {
			e.preventDefault();
			return;
		}
		if (editing) commit();
		e.preventDefault();
		// Clic droit dans la sélection : elle reste (le menu porte sur elle).
		if (e.button === 2 && inSelection(cell.row, cell.col)) return;
		if (e.button === 0 && openTypeMenu(e, cell)) return;
		wrap.focus();
		moveTo(cell.row, cell.col, e.shiftKey);
		dragging = true;
	}

	// Dernière référence insérée au clic : un autre clic la remplace (comme Excel).
	let lastRef: { el: HTMLInputElement; start: number; end: number; value: string } | null = null;

	/**
	 * Référence d'une cellule dans une formule (nomenclature de Slitable) :
	 * Cell[A1] pour une cellule libre ; dans le tableau de la formule, [Col]
	 * pour sa ligne et [Col][k] pour une autre ; ailleurs Tableau[Col][k]
	 * (k : la clé si le tableau en a une, sinon le rang) ; un en-tête donne
	 * Tableau[Col], toute la colonne.
	 */
	function refFor(r: number, c: number): string {
		const e = tableAt(r, c);
		if (!e) return cellAddr(r, c);
		const col = e.t.columns[c - e.t.col].name;
		const here = tableAt(active.row, active.col);
		const same = here === e && active.row > e.t.row;
		// En-tête : toute la colonne.
		if (r === e.t.row) return `${e.t.name}[${col}]`;
		// Même ligne du tableau de la formule : la valeur de la ligne.
		if (same && r === active.row) return `[${col}]`;
		// Une autre ligne : par sa clé si le tableau en a une (stable au tri),
		// sinon par son rang (1 = première ligne de données).
		const n = r - e.t.row - 1;
		const k = e.t.key !== undefined ? e.cache.row(n)?.text[e.t.key] : undefined;
		let item = String(n + 1);
		if (k !== undefined && k !== '') item = isNumeric(k) ? k.replace(/[\s\u00a0\u202f]/g, '') : `"${k.replaceAll('"', '""')}"`;
		return same ? `[${col}][${item}]` : `${e.t.name}[${col}][${item}]`;
	}

	/** Pendant la saisie d'une formule, insère la référence de la cellule cliquée au curseur. */
	function insertRef(cell: { row: number; col: number }): boolean {
		const el = editing ? input : fxEditing ? fxInput : null;
		if (!el) return false;
		const v = el.value;
		const pos = el.selectionStart ?? v.length;
		let start = pos;
		if (lastRef && lastRef.el === el && lastRef.end === pos && lastRef.value === v) start = lastRef.start;
		else if (!/[=(;,+*/^&<>:-]$/.test(v.slice(0, pos).trimEnd())) return false;
		if (editing && cell.row === active.row && cell.col === active.col) return false;
		const ref = refFor(cell.row, cell.col);
		const next = v.slice(0, start) + ref + v.slice(pos);
		if (editing) editValueText = next;
		else fxText = next;
		el.value = next;
		const end = start + ref.length;
		el.setSelectionRange(end, end);
		el.focus();
		lastRef = { el, start, end, value: next };
		return true;
	}

	function onMouseMove(e: MouseEvent) {
		if (rowResizing) {
			rowResizing.height = Math.max(12, Math.round(rowResizing.startH + e.clientY - rowResizing.startY));
			rebuildRows({ row: rowResizing.row, height: rowResizing.height });
			updateExtent();
			schedule();
			return;
		}
		if (resizing) {
			resizing.width = Math.max(24, Math.round(resizing.startW + e.clientX - resizing.startX));
			rebuildAxis({ col: resizing.col, width: resizing.width });
			schedule();
			return;
		}
		if (filling) {
			const cell = cellAt(e);
			if (cell) {
				filling.to = fillTarget(filling.sel, cell);
				schedule();
			}
			return;
		}
		if (moving) {
			const cell = cellAt(e);
			if (cell) {
				moving.to = cell;
				schedule();
			}
			return;
		}
		if (headerDrag) {
			const r = scroller.getBoundingClientRect();
			if (headerDrag === 'rows') {
				const row = clamp(rowAxis.indexAt(e.clientY - r.top - TOP_H + vs.top), 0, SHEET_ROWS - 1);
				selectRows(anchor.row, row);
			} else {
				const col = colAxis.indexAt(Math.max(0, e.clientX - r.left - MARGIN + scroller.scrollLeft));
				selectCols(anchor.col, col);
			}
			return;
		}
		scroller.style.cursor =
			source.setRowHeight && rowBoundaryAt(e) !== null
				? 'row-resize'
				: source.setColumnWidth && boundaryAt(e) !== null
				? 'col-resize'
				: !dragging && onFillHandle(e)
					? 'crosshair'
					: !dragging && onSelectionEdge(e)
						? 'move'
						: headerAt(e)
							? 'pointer'
							: '';
		if (!dragging) updateHeaderTip(e);
		if (!dragging) return;
		const cell = cellAt(e);
		if (!cell) return;
		active = cell;
		schedule();
	}

	function onDblClick(e: MouseEvent) {
		const b = boundaryAt(e);
		if (b !== null && source.setColumnWidth) {
			autoFitColumn(b);
			return;
		}
		const cell = cellAt(e);
		if (cell && pivotAt(cell.row, cell.col)) {
			pivotActive();
			return;
		}
		if (cell) startEdit(false);
	}

	function onKeyDown(e: KeyboardEvent) {
		if (editing) return;
		if (e.key === 'Escape' && painter) painter = null;
		if (e.key === 'Escape' && arrows) {
			arrows = null;
			schedule();
		}
		if (e.key === 'Escape' && clipRange) {
			clipRange = null;
			schedule();
		}
		if (selChart !== null) {
			const ch = charts.find((c) => c.id === selChart);
			if (ch && (e.key === 'Delete' || e.key === 'Backspace')) {
				e.preventDefault();
				removeChart(ch.id);
				return;
			}
			if (e.key === 'Escape') {
				selChart = null;
				if (panel?.kind === 'chart') panel = null;
				return;
			}
		}
		const page = Math.max(1, Math.floor(vs.view / ROW_H) - 1);
		const { row, col } = active;
		const ext = e.shiftKey;
		const ctrl = e.ctrlKey || e.metaKey;
		const arrow = (dr: number, dc: number) => {
			const [r, c] = ctrl ? jump(row, col, dr, dc) : [row + dr, col + dc];
			moveTo(r, c, ext);
		};
		if (ctrl && (e.key === 'z' || e.key === 'Z') && source.undo) {
			e.preventDefault();
			source.undo().catch((err) => say(String(err?.message ?? err)));
			return;
		}
		if (ctrl && (e.key === 'a' || e.key === 'A' || e.key === ' ')) {
			const t = tableAt(row, col);
			if (t) {
				e.preventDefault();
				anchor = { row: t.t.row, col: e.key === ' ' ? col : t.t.col };
				active = { row: t.last, col: e.key === ' ' ? col : t.lastCol };
				refreshFx();
				refreshStatus();
				schedule();
				return;
			}
		}
		if (ctrl && (e.key.toLowerCase() === 'f' || e.key.toLowerCase() === 'h') && source.find) {
			e.preventDefault();
			openFinder(e.key.toLowerCase() === 'h');
			return;
		}
		if (ctrl && ['b', 'i', 'u'].includes(e.key.toLowerCase()) && source.applyStyle) {
			e.preventDefault();
			applyStyle({ [e.key.toLowerCase()]: 'toggle' });
			return;
		}
		// Ctrl + et Ctrl - : insérer ou supprimer des lignes ou colonnes (comme Excel).
		if (ctrl && (e.key === '+' || (e.key === '=' && e.shiftKey) || e.key === '-') && source.sheetLines) {
			e.preventDefault();
			openLinesDialog(e.key !== '-');
			return;
		}
		if (ctrl && (e.key === 'y' || e.key === 'Y') && source.redo) {
			e.preventDefault();
			source.redo().catch((err) => say(String(err?.message ?? err)));
			return;
		}
		switch (e.key) {
			case 'ArrowDown':
				arrow(1, 0);
				break;
			case 'ArrowUp':
				arrow(-1, 0);
				break;
			case 'ArrowRight':
				arrow(0, 1);
				break;
			case 'ArrowLeft':
				arrow(0, -1);
				break;
			case 'PageDown':
				moveTo(row + page, col, ext);
				break;
			case 'PageUp':
				moveTo(row - page, col, ext);
				break;
			case 'Home':
				moveTo(ctrl ? 0 : row, 0, ext);
				break;
			case 'End':
				if (ctrl) {
					const last = entries.reduce((m, e) => Math.max(m, e.last), 0);
					const lastCol = entries.reduce((m, e) => Math.max(m, e.lastCol), 0);
					moveTo(last, lastCol, ext);
				}
				break;
			case 'Tab':
				tabMove(e.shiftKey);
				break;
			case 'Enter':
				enterMove(e.shiftKey);
				break;
			case 'F2':
				startEdit(false);
				break;
			case 'F9':
				if (source.recalc) source.recalc().then(() => say('Recalculé'), (err) => say(String(err?.message ?? err)));
				break;
			case 'Delete':
				clearSelection();
				break;
			case 'Backspace':
				startEdit(true, '');
				break;
			default:
				if (e.key.length === 1 && !ctrl && !e.altKey) startEdit(true, e.key);
				else return;
		}
		e.preventDefault();
	}

	// --- Édition ------------------------------------------------------------

	function startEdit(byTyping: boolean, initial?: string) {
		const info = cellInfo(active.row, active.col);
		// Ligne pas encore chargée (juste après une saisie qui l'a créée) : la
		// frappe ne doit pas être perdue ; seule l'édition de la valeur (F2)
		// attend les données.
		if (info.kind === 'data' && !info.row && initial === undefined) return;
		anchor = { ...active };
		editByTyping = byTyping;
		editValueText = initial ?? info.value;
		// L'éditeur doit avoir le focus avant la frappe suivante : sinon une
		// saisie rapide perdrait des caractères.
		flushSync(() => (editing = true));
		ensureVisible(active.row, active.col);
		if (!input) return;
		placeEditor();
		input.focus();
		const n = input.value.length;
		input.setSelectionRange(n, n);
	}

	function placeEditor() {
		if (!input) return;
		const x = MARGIN + colAxis.offset(active.col) - scroller.scrollLeft;
		const y = TOP_H + rowTop(active.row) - vs.top;
		input.style.transform = `translate(${x}px, ${y}px)`;
		input.style.width = `${Math.max(colAxis.size(active.col), 80)}px`;
		const e = tableAt(active.row, active.col);
		input.style.textAlign = e && active.row > e.t.row ? alignment(e.t.columns[active.col - e.t.col].type) : 'left';
	}

	// Les saisies partent l'une après l'autre : la suivante peut dépendre de
	// la précédente (une saisie fait naître un tableau, la suivante le remplit).
	let writing: Promise<void> = Promise.resolve();
	function write(row: number, col: number, raw: string): Promise<void> {
		writing = writing.then(() => writeNow(row, col, raw));
		return writing;
	}

	async function writeNow(row: number, col: number, raw: string) {
		const info = cellInfo(row, col);
		const target =
			info.kind === 'free'
				? {}
				: { table: info.e!.t, column: info.i, index: info.kind === 'data' ? info.n : undefined, rowId: info.kind === 'data' ? info.row?.id : undefined };
		const t0 = performance.now();
		try {
			const m = await source.setCell(row, col, raw, target);
			lastEditMs = performance.now() - t0;
			if (m && typeof m === 'object') {
				if (info.kind === 'data' && info.n !== undefined) info.e!.cache.set(info.n, info.i!, m.value);
				message = '';
			} else message = m ?? '';
		} catch (e) {
			message = String(e instanceof Error ? e.message : e);
		}
		refreshFx();
		refreshStatus();
		schedule();
	}

	function commit() {
		if (!editing) return;
		editing = false;
		const { row, col } = active;
		if (editValueText !== cellInfo(row, col).value) write(row, col, editValueText);
		wrap.focus();
	}

	function cancel() {
		editing = false;
		wrap.focus();
		schedule();
	}

	async function clearSelection(range = selection()) {
		const { r0, r1, c0, c1 } = range;
		// Les tableaux entièrement compris dans la zone sont supprimés.
		const whole = entries.filter((e) => e.t.row >= r0 && e.last <= r1 && e.t.col >= c0 && e.lastCol <= c1);
		const rest = (r: number, c: number) => !whole.some((e) => inTable(e, r, c));
		let n = 0;
		for (const e of entries) {
			if (whole.includes(e)) continue;
			const rr = Math.max(0, Math.min(r1, e.last) - Math.max(r0, e.t.row) + 1);
			const cc = Math.max(0, Math.min(c1, e.lastCol) - Math.max(c0, e.t.col) + 1);
			n += rr * cc;
		}
		for (const k of cells.keys()) {
			const [r, c] = k.split(',').map(Number);
			if (r >= r0 && r <= r1 && c >= c0 && c <= c1) n++;
		}
		if (n > 1000) {
			say('Effacement limité à 1 000 cellules pour l’instant');
			return;
		}
		if (whole.length && source.deleteTables) {
			const del = source.deleteTables;
			const tables = whole.map((e) => e.t);
			writing = writing.then(() =>
				del(tables).then(
					() => say(tables.length > 1 ? `${tables.length} tableaux supprimés (Ctrl+Z pour annuler)` : `Tableau « ${tables[0].name} » supprimé (Ctrl+Z pour annuler)`),
					(err) => say(String(err?.message ?? err))
				)
			);
			await writing;
		}
		for (const e of entries) {
			if (whole.includes(e)) continue;
			for (let r = Math.max(r0, e.t.row); r <= Math.min(r1, e.last); r++)
				for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol); c++) {
					const info = cellInfo(r, c);
					if (info.kind !== 'header' && info.value !== '') await write(r, c, '');
				}
		}
		for (const k of [...cells.keys()]) {
			const [r, c] = k.split(',').map(Number);
			if (r >= r0 && r <= r1 && c >= c0 && c <= c1 && rest(r, c)) await write(r, c, '');
		}
	}

	function onEditorKey(e: KeyboardEvent) {
		if (input && completionKey(e, input, (v) => (editValueText = v))) return;
		e.stopPropagation();
		const { row, col } = active;
		switch (e.key) {
			case 'Enter':
				commit();
				enterMove(e.shiftKey);
				break;
			case 'Tab':
				commit();
				tabMove(e.shiftKey);
				break;
			case 'Escape':
				cancel();
				break;
			case 'ArrowDown':
			case 'ArrowUp':
			case 'ArrowLeft':
			case 'ArrowRight':
				if (!editByTyping) return;
				commit();
				moveTo(
					row + (e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0),
					col + (e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0)
				);
				break;
			default:
				return;
		}
		e.preventDefault();
	}

	// Barre de formule : même chemin d'écriture que l'éditeur de cellule.
	function onFxKey(e: KeyboardEvent) {
		if (fxInput && completionKey(e, fxInput, (v) => (fxText = v))) return;
		e.stopPropagation();
		if (e.key === 'Escape') {
			fxEditing = false;
			refreshFx();
			wrap.focus();
		} else if (e.key === 'Enter') {
			fxEditing = false;
			const { row, col } = active;
			if (fxText !== cellInfo(row, col).value) write(row, col, fxText);
			wrap.focus();
			moveTo(row + 1, col);
		} else return;
		e.preventDefault();
	}

	// --- Zone de nom ------------------------------------------------------------

	/** Nom du tableau : Entrée le renomme. */
	function onTableNameKey(e: KeyboardEvent) {
		e.stopPropagation();
		const el = e.currentTarget as HTMLInputElement;
		const t = tableAt(active.row, active.col)?.t;
		if (e.key === 'Escape' || !t) {
			el.value = t?.name ?? '';
			wrap.focus();
			return;
		}
		if (e.key !== 'Enter') return;
		e.preventDefault();
		const name = el.value.trim();
		wrap.focus();
		if (!source.renameTable || name === t.name) return;
		const rename = source.renameTable;
		writing = writing.then(async () => {
			try {
				await rename(t, name);
				message = `Tableau renommé « ${name} »`;
			} catch (err) {
				message = String(err instanceof Error ? err.message : err);
			}
			refreshFx();
			refreshStatus();
		});
	}

	/** Colonne[ligne] : Entrée renomme la colonne. */
	function onColumnNameKey(e: KeyboardEvent) {
		e.stopPropagation();
		const el = e.currentTarget as HTMLInputElement;
		const en = tableAt(active.row, active.col);
		if (e.key === 'Escape' || !en) {
			el.value = tableBox?.col ?? '';
			wrap.focus();
			return;
		}
		if (e.key !== 'Enter') return;
		e.preventDefault();
		const c = active.col;
		const name = el.value.replace(/\[.*$/, '').trim();
		wrap.focus();
		if (name && name !== en.t.columns[c - en.t.col].name) write(en.t.row, c, name);
	}

	function onNameKey(e: KeyboardEvent) {
		e.stopPropagation();
		const el = e.currentTarget as HTMLInputElement;
		if (e.key === 'Escape') {
			el.value = nameBox;
			wrap.focus();
			return;
		}
		if (e.key !== 'Enter') return;
		e.preventDefault();
		const text = el.value.trim();
		// Le nom d'une cellule existante : y aller (comme Excel).
		for (const [k, n] of names) {
			if (n.toLowerCase() === text.toLowerCase()) {
				const [r, c] = k.split(',').map(Number);
				wrap.focus();
				moveTo(r, c);
				return;
			}
		}
		// Une adresse (B3 ou Cell_B3) : y aller.
		const addr = /^(?:cell_|cell\[)?([a-z]{1,3})([1-9][0-9]*)\]?$/i.exec(text);
		if (addr) {
			let c = 0;
			for (const ch of addr[1].toUpperCase()) c = c * 26 + ch.charCodeAt(0) - 64;
			wrap.focus();
			moveTo(Number(addr[2]) - 1, c - 1);
			return;
		}
		const { row, col } = active;
		if (tableAt(row, col)) {
			say('Seule une cellule libre peut être nommée ; un tableau se référence par son nom : ' + tableAt(row, col)!.t.name);
			wrap.focus();
			return;
		}
		if (!source.setName || text === (names.get(key(row, col)) ?? '')) {
			wrap.focus();
			return;
		}
		const setName = source.setName;
		writing = writing.then(async () => {
			try {
				await setName(row, col, text);
				message = text ? `Cellule nommée « ${text} »` : 'Nom retiré';
			} catch (err) {
				message = String(err instanceof Error ? err.message : err);
			}
			refreshFx();
			refreshStatus();
		});
		wrap.focus();
	}

	// --- Rechercher / remplacer -------------------------------------------------

	let finder = $state<{ replace: boolean; text: string; by: string; info: string } | null>(null);
	let findInput = $state<HTMLInputElement>();
	let matches: { row: number; col: number }[] = [];
	let matchText = '';
	// Barre ouverte ou fermée : redessiner le surlignage des occurrences.
	$effect(() => {
		void finder;
		schedule();
	});

	/** Ouvre la barre de recherche (replace : avec le champ Remplacer par). */
	export function openFinder(replace: boolean) {
		if (!source.find) return;
		finder = { replace, text: finder?.text ?? '', by: finder?.by ?? '', info: '' };
		requestAnimationFrame(() => findInput?.select());
	}

	async function findNext(back = false) {
		if (!finder || !source.find || !finder.text) return;
		if (finder.text !== matchText) {
			matches = await source.find(finder.text);
			matchText = finder.text;
		}
		if (matches.length === 0) {
			finder.info = 'Aucune cellule trouvée';
			return;
		}
		const { row, col } = active;
		const after = (m: { row: number; col: number }) => m.row > row || (m.row === row && m.col > col);
		const before = (m: { row: number; col: number }) => m.row < row || (m.row === row && m.col < col);
		const i = back ? matches.findLastIndex(before) : matches.findIndex(after);
		const k = i >= 0 ? i : back ? matches.length - 1 : 0;
		moveTo(matches[k].row, matches[k].col);
		finder.info = `${k + 1} sur ${matches.length}${matches.length >= 5000 ? '+' : ''}`;
	}

	async function replaceAll() {
		if (!finder || !source.replace || !finder.text) return;
		const { text, by } = finder;
		try {
			const n = await source.replace(text, by);
			finder.info = n ? `${n} cellule${n > 1 ? 's' : ''} remplacée${n > 1 ? 's' : ''} (Ctrl+Z pour annuler)` : 'Aucune cellule trouvée';
			matchText = '';
		} catch (e) {
			finder.info = String(e instanceof Error ? e.message : e);
		}
	}

	function onFinderKey(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') {
			finder = null;
			wrap.focus();
		} else if (e.key === 'Enter') {
			e.preventDefault();
			findNext(e.shiftKey);
		}
	}

	// --- Presse-papiers -----------------------------------------------------

	function selection() {
		return {
			r0: Math.min(active.row, anchor.row),
			r1: Math.max(active.row, anchor.row),
			c0: Math.min(active.col, anchor.col),
			c1: Math.max(active.col, anchor.col)
		};
	}

	// Zone copiée ou coupée : cadre pointillé ; une zone coupée s'efface au collage.
	let clipRange = $state<{ r0: number; r1: number; c0: number; c1: number; cut: boolean; text: string } | null>(null);

	/** Texte TSV de la sélection (null si trop grande). */
	function selectionText(): string | null {
		const { r0, r1, c0, c1 } = selection();
		if ((r1 - r0 + 1) * (c1 - c0 + 1) > 100_000) {
			say('Copie limitée à 100 000 cellules pour l’instant');
			return null;
		}
		const rows: string[][] = [];
		for (let r = r0; r <= r1; r++) {
			const line: string[] = [];
			for (let c = c0; c <= c1; c++) line.push(cellInfo(r, c).value);
			rows.push(line);
		}
		return toTSV(rows);
	}

	/** Copier ou couper la sélection (ruban, clic droit). */
	export async function copySelection(cut = false) {
		wrap.focus();
		const sc = selChart !== null ? charts.find((c) => c.id === selChart) : undefined;
		if (sc) return copyChart(sc, cut);
		const text = selectionText();
		if (text === null) return;
		try {
			await navigator.clipboard.writeText(text);
		} catch {
			say('Le presse-papiers est inaccessible : utilisez Ctrl+C');
			return;
		}
		clipRange = { ...selection(), cut, text };
		const n = text.split('\n').length;
		say(cut ? 'Sélection coupée : collez-la avec Ctrl+V' : `${n} ligne(s) copiée(s)`);
		schedule();
	}

	/** Coller à la cellule active (ruban, clic droit). */
	export async function pasteClipboard() {
		wrap.focus();
		let text = '';
		try {
			text = await navigator.clipboard.readText();
		} catch {
			say('Le presse-papiers est inaccessible : utilisez Ctrl+V');
			return;
		}
		pasteText(text);
	}

	function onCut(e: ClipboardEvent) {
		if (document.activeElement !== wrap || editing) return;
		const sc = selChart !== null ? charts.find((c) => c.id === selChart) : undefined;
		if (sc) {
			e.preventDefault();
			e.clipboardData?.setData('text/plain', CHART_CLIP + JSON.stringify(sc.config));
			chartClip = sc.config;
			removeChart(sc.id);
			return;
		}
		const text = selectionText();
		if (text === null) return;
		e.clipboardData?.setData('text/plain', text);
		e.preventDefault();
		clipRange = { ...selection(), cut: true, text };
		say('Sélection coupée : collez-la avec Ctrl+V');
		schedule();
	}

	function onCopy(e: ClipboardEvent) {
		if (document.activeElement !== wrap || editing) return;
		const sc = selChart !== null ? charts.find((c) => c.id === selChart) : undefined;
		if (sc) {
			e.clipboardData?.setData('text/plain', CHART_CLIP + JSON.stringify(sc.config));
			e.preventDefault();
			chartClip = sc.config;
			say('Graphique copié : Ctrl+V pour le coller à la cellule active');
			return;
		}
		const { r0, r1, c0, c1 } = selection();
		if ((r1 - r0 + 1) * (c1 - c0 + 1) > 100_000) {
			say('Copie limitée à 100 000 cellules pour l’instant');
			return;
		}
		const rows: string[][] = [];
		for (let r = r0; r <= r1; r++) {
			const line: string[] = [];
			for (let c = c0; c <= c1; c++) line.push(cellInfo(r, c).value);
			rows.push(line);
		}
		e.clipboardData?.setData('text/plain', toTSV(rows));
		e.preventDefault();
		clipRange = { r0, r1, c0, c1, cut: false, text: toTSV(rows) };
		schedule();
		say(`${rows.length * (c1 - c0 + 1)} cellule(s) copiée(s)`);
	}

	function onPaste(e: ClipboardEvent) {
		if (document.activeElement !== wrap || editing || !source.paste) return;
		const text = e.clipboardData?.getData('text/plain') ?? '';
		if (pasteText(text)) e.preventDefault();
	}

	/** Colle un texte (TSV ou graphique) à la cellule active ; faux si rien à coller. */
	function pasteText(text: string): boolean {
		if (!source.paste) return false;
		if (text.startsWith(CHART_CLIP)) {
			try {
				pasteChart(JSON.parse(text.slice(CHART_CLIP.length)) as ChartConfig);
			} catch {
				say('Graphique illisible dans le presse-papiers');
			}
			return true;
		}
		const block = parseTSV(text);
		if (block.length === 0) return false;
		const { row, col } = active;
		// Collage d'une zone coupée : la source s'efface ensuite (hors cible).
		const cut = clipRange?.cut && clipRange.text === text ? clipRange : null;
		const paste = source.paste;
		writing = writing.then(async () => {
			try {
				const m = await paste(row, col, block);
				message = m ?? `${block.length} ligne(s) collée(s)`;
			} catch (err) {
				message = String(err instanceof Error ? err.message : err);
			}
			anchor = { row: row + block.length - 1, col: col + Math.max(...block.map((r) => r.length)) - 1 };
			refreshFx();
			refreshStatus();
			schedule();
		});
		if (cut) {
			const r1 = row + block.length - 1;
			const c1 = col + Math.max(...block.map((r) => r.length)) - 1;
			const overlap = cut.r0 <= r1 && cut.r1 >= row && cut.c0 <= c1 && cut.c1 >= col;
			clipRange = null;
			if (!overlap) writing = writing.then(() => clearSelection(cut));
		}
		return true;
	}

	// --- Menu de type (icône de l'en-tête) ------------------------------------

	const TYPES: ColumnType[] = ['text', 'integer', 'decimal', 'date', 'boolean'];
	let typeMenu = $state<{
		x: number;
		y: number;
		e: Entry;
		i: number;
		pending?: { to: ColumnType; n: number };
		keyWhy?: string | null;
		mix?: TypeMix | null;
	} | null>(null);

	// Une valeur hors type fait passer une colonne inférée en texte : l'infobulle
	// de l'en-tête et le menu ▾ disent combien de valeurs sortent du type
	// majoritaire, et le menu propose d'y revenir (ces valeurs marquées en erreur).
	const mixes = new Map<string, Promise<TypeMix | null>>();
	function mixOf(e: Entry, i: number): Promise<TypeMix | null> {
		const k = e.t.columns[i];
		if (!source.typeMix || k.type !== 'text' || k.fixed || k.formula) return Promise.resolve(null);
		const id = `${e.t.id}:${i}`;
		let p = mixes.get(id);
		if (!p) {
			p = source.typeMix(e.t, i).then(
				(m) => (m.type ? m : null),
				() => null
			);
			mixes.set(id, p);
		}
		return p;
	}

	function mixText(m: TypeMix): string {
		const n = m.off > 1;
		const what =
			m.type === 'date'
				? n ? 'qui ne sont pas des dates' : "qui n'est pas une date"
				: m.type === 'boolean'
					? n ? 'non booléennes' : 'non booléenne'
					: n ? 'non numériques' : 'non numérique';
		return `${m.off} valeur${n ? 's' : ''} ${what} sur ${m.total}`;
	}

	/** Infobulle d'un en-tête de colonne : nom complet et type. */
	let tipCell = '';
	function updateHeaderTip(ev: MouseEvent) {
		const cell = cellAt(ev);
		const e = cell && tableAt(cell.row, cell.col);
		if (!cell || !e || cell.row !== e.t.row) {
			if (tipCell) scroller.title = tipCell = '';
			return;
		}
		const i = cell.col - e.t.col;
		const id = `${e.t.id}:${i}`;
		if (id === tipCell) return;
		tipCell = id;
		const k = e.t.columns[i];
		scroller.title = `${k.name} — ${typeLabel(k.type)}`;
		mixOf(e, i).then((m) => {
			if (m && tipCell === id) scroller.title = `${k.name} — ${typeLabel(k.type)} · ${mixText(m)}`;
		});
	}

	/** Un clic sur l'icône de type d'un en-tête ouvre le menu de type. */
	function openTypeMenu(ev: MouseEvent, cell: { row: number; col: number }): boolean {
		const e = tableAt(cell.row, cell.col);
		if (!e || cell.row !== e.t.row || !source.setType) return false;
		if (e.t.columns[cell.col - e.t.col].formula) return false; // le type suit la formule
		const r = scroller.getBoundingClientRect();
		const right = MARGIN + colAxis.offset(cell.col + 1) - scroller.scrollLeft;
		const mx = ev.clientX - r.left;
		if (right - mx > 34) return false;
		moveTo(cell.row, cell.col);
		showTypeMenu(mx, ev.clientY - r.top + 8, e, cell.col - e.t.col);
		return true;
	}

	/** Ouvre le menu de type et de clé de la colonne i (icône d'en-tête ou clic droit). */
	function showTypeMenu(x: number, y: number, e: Entry, i: number) {
		typeMenu = { x, y, e, i, keyWhy: null };
		mixOf(e, i).then((mix) => typeMenu && typeMenu.e.t.id === e.t.id && typeMenu.i === i && (typeMenu.mix = mix));
		if (source.keyCheck && e.t.key !== i)
			source.keyCheck(e.t, i).then(
				(why) => typeMenu && typeMenu.i === i && (typeMenu.keyWhy = why),
				() => undefined
			);
	}

	async function chooseType(to: ColumnType, confirmed = false) {
		const m = typeMenu;
		if (!m) return;
		if (!confirmed && source.convertPreview && to !== m.e.t.columns[m.i].type) {
			const n = await source.convertPreview(m.e.t, m.i, to);
			if (n > 0) {
				typeMenu = { ...m, pending: { to, n } };
				return;
			}
		}
		typeMenu = null;
		const setType = source.setType!;
		await menuAction(() => setType(m.e.t, m.i, to));
	}

	// --- Menu contextuel ----------------------------------------------------

	// rowOnly : clic droit sur un numéro de ligne (pas d'actions de colonne).
	// confirmDelete : suppression d'un grand tableau en attente de confirmation.
	let menu = $state<{ x: number; y: number; e: Entry; n: number; i: number; header: boolean; rowOnly?: boolean; confirmDelete?: boolean } | null>(null);

	/** Au-delà de ce nombre de lignes, « Supprimer le tableau » demande confirmation. */
	const CONFIRM_DELETE_ROWS = 500;

	function deleteTable(t: SheetTable) {
		// Pas menuAction : il effacerait le message une fois l'action finie.
		const del = source.deleteTables!;
		menu = null;
		wrap.focus();
		writing = writing.then(() =>
			del([t]).then(
				() => say(`Tableau « ${t.name} » supprimé (Ctrl+Z pour annuler)`),
				(err) => say(String(err?.message ?? err))
			)
		);
	}

	function onContextMenu(ev: MouseEvent) {
		const cell = cellAt(ev);
		ev.preventDefault();
		menu = null;
		if (!cell) {
			// Clic droit sur un numéro de ligne : menu de la ligne du tableau ;
			// hors tableau (ou sur une lettre de colonne) : presse-papiers.
			const hd = headerAt(ev);
			const r = wrap.getBoundingClientRect();
			const te = hd?.kind === 'rows' ? entries.find((x) => hd.row > x.t.row && hd.row <= x.last) : undefined;
			freeMenu = null;
			if (te && hd?.kind === 'rows') menu = { x: ev.clientX - r.left, y: ev.clientY - r.top, e: te, n: hd.row - te.t.row - 1, i: 0, header: false, rowOnly: true };
			else if (hd) freeMenu = { x: ev.clientX - r.left, y: ev.clientY - r.top, kind: hd.kind };
			return;
		}
		const e = tableAt(cell.row, cell.col);
		if (!inSelection(cell.row, cell.col)) moveTo(cell.row, cell.col);
		const r = wrap.getBoundingClientRect();
		if (!e) {
			freeMenu = { x: ev.clientX - r.left, y: ev.clientY - r.top };
			return;
		}
		menu = { x: ev.clientX - r.left, y: ev.clientY - r.top, e, n: cell.row - e.t.row - 1, i: cell.col - e.t.col, header: cell.row === e.t.row };
	}

	// Menu du clic droit hors tableau : presse-papiers et effacement.
	let freeMenu = $state<{ x: number; y: number; kind?: 'rows' | 'cols' } | null>(null);

	async function menuAction(fn: (() => Promise<unknown>) | undefined) {
		menu = null;
		freeMenu = null;
		wrap.focus();
		if (!fn) return;
		writing = writing.then(async () => {
			try {
				await fn();
				message = '';
			} catch (err) {
				message = String(err instanceof Error ? err.message : err);
			}
			refreshStatus();
		});
	}

	// --- Mesures ------------------------------------------------------------

	/** Défilement continu, sauts aléatoires et saisies, sur le premier tableau. */
	export async function runBench() {
		const e = entries[0];
		if (!e) return;
		bench = 'Mesure en cours…';
		const frames = (n: number) =>
			new Promise<void>((res) => {
				let k = 0;
				const step = () => (++k >= n ? res() : requestAnimationFrame(step));
				requestAnimationFrame(step);
			});
		const tableTop = rowTop(e.t.row + 1);
		const tableSpan = Math.max(0, rspan(e.t.row + 1, e.t.row + e.t.rows) - vs.view);

		vs.to(tableTop + Math.random() * tableSpan * 0.5);
		syncNative();
		await frames(30);
		const intervals: number[] = [];
		missingFrames = 0;
		let drawn = 0;
		let prev = performance.now();
		const end = prev + 5000;
		await new Promise<void>((res) => {
			const step = () => {
				const now = performance.now();
				intervals.push(now - prev);
				prev = now;
				drawn++;
				vs.by(10 * ROW_H);
				scroller.scrollTop = vs.toNative();
				draw();
				if (now < end) requestAnimationFrame(step);
				else res();
			};
			requestAnimationFrame(step);
		});
		const scrollMissing = missingFrames;
		const janky = intervals.filter((d) => d > 1000 / 30).length;

		const jumps: number[] = [];
		for (let i = 0; i < 20; i++) {
			vs.to(tableTop + Math.random() * tableSpan);
			syncNative();
			const t0 = performance.now();
			const { first, last } = visibleRows();
			const d = visibleData(e, first, last)!;
			await new Promise<void>((res) => {
				const check = () => (e.cache.has(d.first, d.last) ? res() : requestAnimationFrame(check));
				check();
			});
			jumps.push(performance.now() - t0);
			await frames(2);
		}

		const edits: number[] = [];
		const { first: f0, last: f1 } = visibleRows();
		const d0 = visibleData(e, f0, f1)!;
		for (let i = 0; i < 20; i++) {
			const n = d0.first + i;
			const row = e.cache.row(n);
			if (!row) continue;
			const t0 = performance.now();
			await source.setCell(e.t.row + 1 + n, e.t.col + 1, editValue(e.t.columns[1].type, row.cells[1]), {
				table: e.t,
				column: 1,
				index: n,
				rowId: row.id
			});
			edits.push(performance.now() - t0);
		}

		const result = {
			date: new Date().toISOString(),
			userAgent: navigator.userAgent,
			rows: e.t.rows,
			scroll: {
				frames: drawn,
				fps: Math.round(drawn / 5),
				frameP50: +percentile(intervals, 0.5).toFixed(1),
				frameP95: +percentile(intervals, 0.95).toFixed(1),
				framesOver33ms: janky,
				framesWithPlaceholders: scrollMissing
			},
			jumpToDataP50: +percentile(jumps, 0.5).toFixed(1),
			jumpToDataP95: +percentile(jumps, 0.95).toFixed(1),
			editP50: +percentile(edits, 0.5).toFixed(2),
			editP95: +percentile(edits, 0.95).toFixed(2),
			drawP50: +percentile(drawTimes, 0.5).toFixed(2),
			drawP95: +percentile(drawTimes, 0.95).toFixed(2),
			blockRoundTripP50: +percentile(e.cache.stats.roundTrip, 0.5).toFixed(2),
			blockRoundTripP95: +percentile(e.cache.stats.roundTrip, 0.95).toFixed(2),
			blockGoP50: +percentile(e.cache.stats.server, 0.5).toFixed(2)
		};
		console.log('slitable-bench', JSON.stringify(result));
		(window as unknown as { slitableBench: unknown }).slitableBench = result;
		bench =
			`Défilement : ${result.scroll.fps} img/s, image p95 ${result.scroll.frameP95} ms, ` +
			`${janky} images > 33 ms, ${scrollMissing}/${drawn} images incomplètes · ` +
			`Saut : p50 ${result.jumpToDataP50} ms · Saisie : p50 ${result.editP50} ms`;
		refreshStatus();
		return result;
	}

	onMount(() => {
		ctx = canvas.getContext('2d', { alpha: false })!;
		mounted = true;
		const resize = () => {
			const dpr = window.devicePixelRatio || 1;
			const cw = scroller.clientWidth;
			const ch = scroller.clientHeight;
			const pw = Math.round(cw * dpr);
			const ph = Math.round(ch * dpr);
			if (cw === viewW && ch === viewH && canvas.width === pw && canvas.height === ph) return;
			viewW = cw;
			viewH = ch;
			// Changer la taille du canevas l'efface (en noir, sans transparence) :
			// on redessine aussitôt, sans attendre l'image suivante.
			canvas.width = pw;
			canvas.height = ph;
			canvas.style.width = `${viewW}px`;
			canvas.style.height = `${viewH}px`;
			vs.view = viewH - TOP_H;
			updateExtent();
			syncNative();
			if (frame) cancelAnimationFrame(frame);
			draw();
		};
		const ro = new ResizeObserver(resize);
		ro.observe(wrap);
		resize();
		scroller.addEventListener('wheel', onWheel, { passive: false });
		const up = () => {
			if (dragging && painter) applyPainter();
			dragging = false;
			headerDrag = null;
			if (filling) {
				const f = filling;
				filling = null;
				if (f.to) doFill(f.sel, f.to);
				schedule();
			}
			if (moving) dropMove();
			if (rowResizing) {
				const { row, height, startH } = rowResizing;
				rowResizing = null;
				if (height !== startH && source.setRowHeight) {
					userHeights.set(row, height);
					const setHeight = source.setRowHeight;
					writing = writing.then(() => setHeight(row, height).then(() => undefined, (err) => say(String(err?.message ?? err))));
				}
				return;
			}
			if (!resizing) return;
			const { col, width, startW } = resizing;
			resizing = null;
			if (width === startW || !source.setColumnWidth) return;
			userWidths.set(col, width);
			const setWidth = source.setColumnWidth;
			writing = writing.then(() => setWidth(col, width).then(() => undefined, (err) => say(String(err?.message ?? err))));
		};
		window.addEventListener('mouseup', up);
		const timer = setInterval(refreshStatus, 250);
		window.addEventListener('copy', onCopy);
		window.addEventListener('keydown', onGlobalKey);
		window.addEventListener('cut', onCut);
		window.addEventListener('paste', onPaste);
		wrap.focus();
		refreshStatus();
		if (autoBench) setTimeout(async () => autoBench(JSON.stringify(await runBench())), 1500);
		return () => {
			ro.disconnect();
			scroller.removeEventListener('wheel', onWheel);
			window.removeEventListener('mouseup', up);
			clearInterval(timer);
			window.removeEventListener('copy', onCopy);
			window.removeEventListener('keydown', onGlobalKey);
			window.removeEventListener('cut', onCut);
			window.removeEventListener('paste', onPaste);
			cancelAnimationFrame(frame);
		};
	});

	/** Nom du tableau de la cellule active (pour l'export), s'il y en a un. */
	export function activeTable(): string | undefined {
		return tableAt(active.row, active.col)?.t.name;
	}

	/** Redonne le focus clavier à la feuille (après un dialogue, un menu…). */
	export function focus() {
		wrap?.focus();
	}
</script>

{#snippet linesItems(kind: 'rows' | 'cols' | undefined, x: number, y: number)}
	{#if source.sheetLines}
		{@const { r0, r1, c0, c1 } = selection()}
		<hr />
		{#if kind === 'rows'}
			<button onclick={() => sheetLines('insertRows')}>Insérer {r1 > r0 ? `${r1 - r0 + 1} lignes` : 'une ligne'}</button>
			<button onclick={() => sheetLines('deleteRows')}>Supprimer {r1 > r0 ? 'les lignes' : 'la ligne'}</button>
			{#if source.setLineRules}<button onclick={() => openLineRules(true)}>Mise en forme conditionnelle…</button>{/if}
		{:else if kind === 'cols'}
			<button onclick={() => sheetLines('insertCols')}>Insérer {c1 > c0 ? `${c1 - c0 + 1} colonnes` : 'une colonne'}</button>
			<button onclick={() => sheetLines('deleteCols')}>Supprimer {c1 > c0 ? 'les colonnes' : 'la colonne'}</button>
			{#if source.setLineRules}<button onclick={() => openLineRules(false)}>Mise en forme conditionnelle…</button>{/if}
		{:else}
			<button onclick={() => openLinesDialog(true, x, y)}>Insérer…</button>
			<button onclick={() => { menu = freeMenu = null; clearSelection(); }}>Supprimer</button>
		{/if}
	{/if}
{/snippet}

{#snippet suggestions(where: 'cell' | 'fx')}
	{#if sig?.where === where}
		{@const sp = signatureParts(sig.fn, sig.index)}
		<div class="sig">
			<div class="sig-line">{#each sp.parts as p, i (i)}{#if p.on}<b>{p.text}</b>{:else}{p.text}{/if}{/each}</div>
			{#if sig.fn.description}<div class="sig-desc">{sig.fn.description}</div>{/if}
			{#if sp.arg}<div class="sig-arg"><b>{sp.arg.name}</b>{sp.arg.optional ? ' (facultatif)' : ''} : {sp.arg.desc}</div>{/if}
			{#if sig.fn.example}<div class="sig-ex">Exemple : <code>{sig.fn.example}</code></div>{/if}
		</div>
	{/if}
	{#if completion?.where === where}
		{#each completion.items as s, i (s.kind + s.label)}
			<!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
			<div
				class="sugg"
				class:on={i === completion.sel}
				onmousedown={(e) => {
					e.preventDefault();
					completion = { ...completion!, sel: i };
					const el = where === 'cell' ? input : fxInput;
					if (el) completionKey(new KeyboardEvent('keydown', { key: 'Enter' }), el, (v) => (where === 'cell' ? (editValueText = v) : (fxText = v)));
				}}
			>
				<span class="kind {s.kind}">{s.kind === 'function' ? 'fx' : s.kind === 'table' ? '▦' : s.kind === 'name' ? 'N' : '[ ]'}</span>
				<span class="lab">{s.label}</span>
				<span class="det" title={s.detail}>{s.detail}</span>
			</div>
		{/each}
	{/if}
{/snippet}

<div class="sheet-root">
	<div class="formula-bar">
		{#if tableBox}
			<div class="name-box split">
				<input
					class="table-name"
					title="Nom du tableau : modifiez puis Entrée pour le renommer"
					value={tableBox.name}
					onfocus={(e) => e.currentTarget.select()}
					onkeydown={onTableNameKey}
					spellcheck="false"
				/>
				<span class="slash">/</span>
				<input
					class="col-name"
					title="Colonne[ligne] : modifiez le nom puis Entrée pour renommer la colonne"
					value={tableBox.col}
					onfocus={(e) => e.currentTarget.select()}
					onkeydown={onColumnNameKey}
					spellcheck="false"
				/>
			</div>
		{:else}
			<input
				class="name-box"
				title="Nom de la cellule : tapez un nom puis Entrée pour la nommer, ou le nom d'une cellule pour y aller"
				value={nameBox}
				placeholder="—"
				onfocus={(e) => e.currentTarget.select()}
				onkeydown={onNameKey}
				spellcheck="false"
			/>
		{/if}
		<span class="fx">fx</span>
		<input
			class="fx-input"
			bind:this={fxInput}
			bind:value={fxText}
			oninput={(e) => onFormulaInput(e.currentTarget, 'fx')}
			onkeyup={(e) => moveSig(e, 'fx')}
			onclick={(e) => moveSig(e, 'fx')}
			onfocus={() => (fxEditing = true)}
			onkeydown={onFxKey}
			onblur={() => {
				fxEditing = false;
				completion = sig = null;
				refreshFx();
			}}
			spellcheck="false"
		/>
	</div>
	{#if bench}<div class="bench">{bench}</div>{/if}
	{#if completion?.where === 'fx' || sig?.where === 'fx'}
		<div class="suggest fx-suggest">{@render suggestions('fx')}</div>
	{/if}

	<div class="body">
	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	<div class="wrap" bind:this={wrap} tabindex="0" role="grid" onkeydown={onKeyDown}>
		<canvas bind:this={canvas}></canvas>
		{#if finder}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="finder" onkeydown={(e) => e.stopPropagation()}>
				<span>Rechercher</span>
				<input bind:this={findInput} bind:value={finder.text} onkeydown={onFinderKey} spellcheck="false" />
				<button onclick={() => findNext(true)} title="Précédent (Maj+Entrée)">◀</button>
				<button onclick={() => findNext()} title="Suivant (Entrée)">▶</button>
				{#if finder.replace}
					<span>Remplacer par</span>
					<input bind:value={finder.by} onkeydown={onFinderKey} spellcheck="false" />
					<button onclick={replaceAll}>Remplacer tout</button>
				{:else if source.replace}
					<button class="link" onclick={() => finder && (finder.replace = true)}>Remplacer…</button>
				{/if}
				<span class="muted">{finder.info}</span>
				<button onclick={() => {
					finder = null;
					wrap.focus();
				}} title="Fermer (Échap)">✕</button>
			</div>
		{/if}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="scroller"
			bind:this={scroller}
			onscroll={onScroll}
			onpointerdown={onScrollbarDown}
			onmousedown={onMouseDown}
			onmousemove={onMouseMove}
			ondblclick={onDblClick}
			oncontextmenu={onContextMenu}
		>
			<div bind:this={spacer}></div>
		</div>
		<!-- Calque découpé à la zone des cellules : les graphiques ne passent pas sur les barres de défilement. -->
		<div class="charts" style:left="{MARGIN}px" style:top="{TOP_H}px" style:width="{Math.max(0, viewW - MARGIN)}px" style:height="{Math.max(0, viewH - TOP_H)}px">
			{#each charts as ch (ch.id)}
				{@const box = chartBox(ch)}
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div
					class="chart"
					style:left="{box.x - viewPos.x}px"
					style:top="{box.y - viewPos.y}px"
					class:selected={selChart === ch.id}
					style:width="{box.w}px"
					style:height="{box.h}px"
					ondblclick={() => editChart(ch)}
					onmousedown={(ev) => startChartGesture(ev, ch)}
					oncontextmenu={(ev) => onChartMenu(ev, ch)}
					onwheel={(ev) => onWheel(ev)}
				>
					{#if ch.error}
						<p class="chart-error">Graphique : {ch.error}</p>
					{:else}
						<Chart kind={ch.config.kind} labels={ch.labels} series={ch.series} width={box.w} height={box.h} style={ch.config} />
					{/if}
					{#if selChart === ch.id}
						{#each ['n', 's', 'e', 'w', 'ne', 'nw', 'se', 'sw'] as dir (dir)}
							<!-- svelte-ignore a11y_no_static_element_interactions -->
							<div class="handle {dir}" onmousedown={(ev) => startChartGesture(ev, ch, dir)}></div>
						{/each}
					{/if}
				</div>
			{/each}
			<SheetButtons
				bind:this={buttonsLayer}
				{buttons}
				geo={{ colOffset: (c) => colAxis.offset(c), rowTop, colAt: (x) => colAxis.indexAt(x), rowAt: (y) => rowAxis.indexAt(y) }}
				view={viewPos}
				active={() => active}
				scripts={() => source.scripts?.() ?? Promise.resolve([])}
				onrun={runButton}
				oncreate={source.createButton}
				onset={source.setButton}
				onplace={source.placeButton}
				ondelete={source.deleteButton}
				onwheel={(ev) => onWheel(ev)}
				{say}
				done={() => wrap.focus()}
			/>
		</div>
		{#if typeMenu}
			{@const m = typeMenu}
			{@const col = m.e.t.columns[m.i]}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (typeMenu = null)}>
				<div class="menu-title">« {col.name} » · type {col.fixed ? 'fixé' : 'inféré automatiquement'}</div>
				{#if m.pending}
					<p class="menu-note">
						{m.pending.n > 1
							? `${m.pending.n} valeurs ne peuvent pas devenir « ${typeLabel(m.pending.to)} » : elles seront gardées et marquées en erreur.`
							: `1 valeur ne peut pas devenir « ${typeLabel(m.pending.to)} » : elle sera gardée et marquée en erreur.`}
					</p>
					<button onclick={() => chooseType(m.pending!.to, true)}>Convertir quand même</button>
					<button onclick={() => (typeMenu = { ...m, pending: undefined })}>Annuler</button>
				{:else}
					<!-- Tri et filtre de la colonne, comme le menu ▾ d'Excel. -->
					{#if source.sortTable}
						<button
							onclick={() => {
								const mm = { e: m.e, i: m.i };
								typeMenu = null;
								menuAction(() => source.sortTable!(mm.e.t, mm.i, false));
							}}>Trier de A à Z</button
						>
						<button
							onclick={() => {
								const mm = { e: m.e, i: m.i };
								typeMenu = null;
								menuAction(() => source.sortTable!(mm.e.t, mm.i, true));
							}}>Trier de Z à A</button
						>
					{/if}
					{#if source.setFilter}
						<button
							onclick={() => {
								const mm = { e: m.e, i: m.i };
								typeMenu = null;
								openFilter(mm.e, mm.i);
							}}>Filtrer…</button
						>
						{#if col.filter}
							<button
								onclick={() => {
									const mm = { e: m.e, i: m.i };
									typeMenu = null;
									menuAction(() => source.setFilter!(mm.e.t, mm.i, null));
								}}>Effacer le filtre de « {col.name} »</button
							>
						{/if}
					{/if}
					<hr />
					<div class="menu-title">Type de données</div>
					{#each TYPES as t (t)}
						<button onclick={() => chooseType(t)}>
							<span class="check">{t === col.type ? '✓' : ''}</span>{#if typeIconName(t)}<span class="type-icon"><Icon name={typeIconName(t)!} size={14} /></span>{:else}{typeIcon(t)}{/if}&nbsp;&nbsp;{typeLabel(t)}
						</button>
					{/each}
					{#if m.mix && m.mix.type}
						{@const to = m.mix.type}
						<p class="menu-note">{mixText(m.mix)}</p>
						<button onclick={() => chooseType(to, true)}
							>Repasser en {typeLabel(to)} ({m.mix.off} valeur{m.mix.off > 1 ? 's' : ''} marquée{m.mix.off > 1 ? 's' : ''} en erreur)</button
						>
					{/if}
					{#if col.errors}<p class="menu-note error">{col.errors} saisie{col.errors > 1 ? 's' : ''} hors type dans la colonne</p>{/if}
					{#if source.setKey}
						<hr />
						{#if m.e.t.key === m.i}
							<p class="menu-note">🔑 Clé du tableau : ses valeurs identifient les lignes.</p>
							<button
								onclick={() => {
									const mm = { e: m.e, i: m.i };
									typeMenu = null;
									menuAction(() => source.clearKey!(mm.e.t));
								}}>Retirer la clé</button
							>
						{:else if m.keyWhy === ''}
							<button
								onclick={() => {
									const mm = { e: m.e, i: m.i };
									typeMenu = null;
									menuAction(() => source.setKey!(mm.e.t, mm.i));
								}}>🔑 Définir comme clé (identifiant des lignes){m.e.t.key !== undefined ? ', à la place de l’actuelle' : ''}</button
							>
						{:else if m.keyWhy}
							<p class="menu-note">Clé impossible : {m.keyWhy}</p>
						{/if}
					{/if}
				{/if}
			</div>
		{/if}
		{#if menu}
			{@const m = menu}
			{@const col = m.e.t.columns[m.i]}
			{@const cols = !m.rowOnly}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (menu = null)}>
				<div class="menu-title">{m.e.t.name}</div>
				{#if m.confirmDelete}
					<p class="menu-note">
						Supprimer le tableau « {m.e.t.name} » et ses {(m.e.t.rows + (m.e.t.hidden ?? 0)).toLocaleString('fr-FR')} lignes ? Ctrl+Z permettra de l’annuler.
					</p>
					<button onclick={() => deleteTable(m.e.t)}>Supprimer le tableau</button>
					<button onclick={() => (menu = { ...m, confirmDelete: false })}>Annuler</button>
				{:else}
					<!-- Même ordre que le menu contextuel d'Excel : presse-papiers, insertion
					     et suppression, tri et filtre, mise en forme, puis ce qui est propre
					     à Slitable (type, clé, nom de colonne, tableau). -->
					<button
						onclick={() => {
							menu = null;
							copySelection(true);
						}}>Couper</button
					>
					<button
						onclick={() => {
							menu = null;
							copySelection();
						}}>Copier</button
					>
					<button
						onclick={() => {
							menu = null;
							pasteClipboard();
						}}>Coller</button
					>
					<hr />
					{#if !m.header && !m.rowOnly}
						<button onclick={() => menuAction(source.insertRow && (() => source.insertRow!(m.e.t, m.n)))}>Insérer une ligne au-dessus</button>
						<button onclick={() => menuAction(source.insertRow && (() => source.insertRow!(m.e.t, m.n + 1)))}>Insérer une ligne en dessous</button>
					{/if}
					{#if cols}
						<button onclick={() => menuAction(source.insertColumn && (() => source.insertColumn!(m.e.t, m.i)))}>Insérer une colonne à gauche</button>
						<button onclick={() => menuAction(source.insertColumn && (() => source.insertColumn!(m.e.t, m.i + 1)))}>Insérer une colonne à droite</button>
					{/if}
					{#if !m.header && !m.rowOnly}
						<button onclick={() => menuAction(source.deleteRow && (() => source.deleteRow!(m.e.t, m.n)))}>Supprimer la ligne</button>
					{/if}
					{#if cols}
						<button onclick={() => menuAction(source.deleteColumn && (() => source.deleteColumn!(m.e.t, m.i)))}>Supprimer la colonne « {col?.name} »</button>
					{/if}
					{#if m.rowOnly}{@render linesItems('rows', m.x, m.y)}{/if}
					<button
						onclick={() => {
							menu = null;
							clearSelection();
						}}>Effacer le contenu</button
					>
					{#if cols && !col?.formula && (source.sortTable || source.setFilter)}
						<hr />
						{#if source.sortTable}
							<button onclick={() => menuAction(() => source.sortTable!(m.e.t, m.i, false))}>Trier de A à Z (croissant)</button>
							<button onclick={() => menuAction(() => source.sortTable!(m.e.t, m.i, true))}>Trier de Z à A (décroissant)</button>
						{/if}
						{#if source.setFilter}
							<button onclick={() => openFilter(m.e, m.i)}>Filtrer par « {col?.name} »…</button>
						{/if}
					{/if}
					{#if cols && source.clearFilters && m.e.t.columns.some((c) => c.filter)}
						<button onclick={() => menuAction(() => source.clearFilters!(m.e.t))}>Effacer le filtre du tableau</button>
					{/if}
					{#if cols && source.setRules}
						<hr />
						<button onclick={() => openRules(m.e, m.i)}>Mise en forme conditionnelle…</button>
					{/if}
					<hr />
					{#if cols && source.setType && !col?.formula}
						<button
							onclick={() => {
								const { x, y, e, i } = m;
								menu = null;
								showTypeMenu(x, y, e, i);
							}}>Type de données et clé de « {col?.name} »…</button
						>
					{/if}
					{#if cols}
						<button
							onclick={() => {
								const t = m.e.t;
								const c = t.col + m.i;
								menu = null;
								moveTo(t.row, c);
								startEdit(false);
							}}>Renommer la colonne « {col?.name} »</button
						>
					{/if}
					{#if cols && source.promoteHeader && m.e.t.rows > 0}
						<button onclick={() => menuAction(() => source.promoteHeader!(m.e.t))}>Utiliser la 1re ligne comme en-têtes</button>
					{/if}
					{#if source.deleteTables}
						<button
							onclick={() => {
								if (m.e.t.rows + (m.e.t.hidden ?? 0) > CONFIRM_DELETE_ROWS) menu = { ...m, confirmDelete: true };
								else deleteTable(m.e.t);
							}}>Supprimer le tableau « {m.e.t.name} »</button
						>
					{/if}
				{/if}
			</div>
		{/if}
		{#if freeMenu}
			{@const m = freeMenu}
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (freeMenu = null)}>
				<button
					onclick={() => {
						freeMenu = null;
						copySelection(true);
					}}>Couper</button
				>
				<button
					onclick={() => {
						freeMenu = null;
						copySelection();
					}}>Copier</button
				>
				<button
					onclick={() => {
						freeMenu = null;
						pasteClipboard();
					}}>Coller</button
				>
				{#if chartClip}
					<button
						onclick={() => {
							freeMenu = null;
							pasteChart(chartClip!);
						}}>Coller le graphique</button
					>
				{/if}
				{#if buttonsLayer?.hasClip()}
					<button
						onclick={() => {
							freeMenu = null;
							buttonsLayer?.pasteAt(active.row, active.col);
						}}>Coller le bouton</button
					>
				{/if}
				{@render linesItems(m.kind, m.x, m.y)}
				<hr />
				<button
					onclick={() => {
						freeMenu = null;
						clearSelection();
					}}>Effacer le contenu</button
				>
			</div>
		{/if}
		{#if linesDlg}
			{@const d = linesDlg}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="rules lines-dlg" use:dismiss={() => (linesDlg = null)} onkeydown={(e) => e.key === 'Enter' && applyLinesDialog()}>
				<div class="menu-title">{d.insert ? 'Insérer' : 'Supprimer'}</div>
				<label><input type="radio" bind:group={d.choice} value="rows" /> {d.insert ? 'Ligne entière (au-dessus)' : 'Ligne entière'}</label>
				<label><input type="radio" bind:group={d.choice} value="cols" /> {d.insert ? 'Colonne entière (à gauche)' : 'Colonne entière'}</label>
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" onclick={applyLinesDialog}>OK</button>
					<button onclick={() => (linesDlg = null)}>Annuler</button>
				</div>
			</div>
		{/if}
		{#if chartMenu}
			{@const m = chartMenu}
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (chartMenu = null)}>
				<div class="menu-title">Graphique</div>
				<button onclick={() => copyChart(m.ch, true)}>Couper</button>
				<button onclick={() => copyChart(m.ch, false)}>Copier</button>
				<button
					disabled={!chartClip}
					onclick={() => {
						chartMenu = null;
						if (chartClip) pasteChart(chartClip);
					}}>Coller un graphique à la cellule active</button
				>
				<hr />
				<button onclick={() => removeChart(m.ch.id)}>Supprimer le graphique</button>
			</div>
		{/if}
		{#if dedupDlg}
			{@const d = dedupDlg}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="rules filter"
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Escape') {
						dedupDlg = null;
						wrap.focus();
					}
				}}
			>
				<div class="menu-title">Supprimer les doublons de « {d.e.t.name} »</div>
				<p class="menu-note">Deux lignes sont en double quand les colonnes cochées ont les mêmes valeurs. La première est gardée.</p>
				<div class="values">
					<label class="all">
						<input
							type="checkbox"
							checked={d.on.every((x, i) => x || !!d.e.t.columns[i].formula)}
							onchange={(ev) => {
								const on = (ev.currentTarget as HTMLInputElement).checked;
								d.on = d.e.t.columns.map((c) => on && !c.formula);
								previewDedup();
							}}
						/>
						(Toutes les colonnes)
					</label>
					{#each d.e.t.columns as c, i (i)}
						<label>
							<input
								type="checkbox"
								disabled={!!c.formula}
								checked={d.on[i]}
								onchange={(ev) => {
									d.on[i] = (ev.currentTarget as HTMLInputElement).checked;
									previewDedup();
								}}
							/>
							<span class="what">{c.name}</span>
						</label>
					{/each}
				</div>
				<p class="menu-note">
					{d.count === null ? 'Calcul…' : d.count === 0 ? 'Aucune ligne en double avec ces colonnes.' : `${d.count} ligne${d.count > 1 ? 's' : ''} en double ser${d.count > 1 ? 'ont' : 'a'} supprimée${d.count > 1 ? 's' : ''}.`}
				</p>
				{#if d.error}<p class="menu-note error">{d.error}</p>{/if}
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" disabled={!d.count} onclick={applyDedup}>Supprimer</button>
					<button
						onclick={() => {
							dedupDlg = null;
							wrap.focus();
						}}>Annuler</button
					>
				</div>
			</div>
		{/if}
		{#if fnDlg}
			{@const d = fnDlg}
			{@const shownFns = shownFunctions()}
			{@const cur = shownFns.find((f) => f.name === d.sel) ?? shownFns[0]}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="rules filter"
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Escape') {
						fnDlg = null;
						wrap.focus();
					}
				}}
			>
				<div class="menu-title">Insérer une fonction ({catalog?.functions.length ?? 0} disponibles)</div>
				<!-- svelte-ignore a11y_autofocus -->
				<div class="add"><input class="search" bind:value={d.search} placeholder="Rechercher (SOMME, RECHERCHEV, date, texte…)" autofocus /></div>
				<div class="values">
					{#each shownFns as f (f.name)}
						<button
							class="fn"
							class:on={f.name === cur?.name}
							onmouseenter={() => (d.sel = f.name)}
							onfocus={() => (d.sel = f.name)}
							onclick={() => startFormula(`=${f.name}(`)}
						>
							<strong>{f.name}</strong>
							<span class="count fn-desc">{f.description ?? ''}</span>
						</button>
					{/each}
				</div>
				{#if cur}
					<div class="fn-doc">
						<div class="fn-sig">{cur.signature || cur.name + '()'}</div>
						<div>{cur.description}</div>
						{#if cur.args?.length}
							<dl>
								{#each cur.args as arg (arg.name)}
									<dt>{arg.name}{arg.optional ? ' (facultatif)' : ''}</dt>
									<dd>{arg.desc}</dd>
								{/each}
							</dl>
							{#if cur.repeat}<div class="count">Les {cur.repeat > 1 ? `${cur.repeat} derniers arguments peuvent` : 'dernier argument peut'} se répéter.</div>{/if}
						{/if}
						{#if cur.example}<div>Exemple : <code>{cur.example}</code></div>{/if}
						<div class="count">{cur.category}{cur.other && cur.other !== cur.name ? ` · en anglais : ${cur.other}` : ''}</div>
					</div>
				{/if}
				<div class="foot">
					<span class="sp"></span>
					<button
						onclick={() => {
							fnDlg = null;
							wrap.focus();
						}}>Fermer</button
					>
				</div>
			</div>
		{/if}
		{#if filterDlg}
			{@const d = filterDlg}
			{@const shown = filterShown()}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="rules filter anchored"
				style:left="{d.x}px"
				style:top="{d.y}px"
				use:focusFirst
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Escape') closeFilter();
					if (ev.key === 'Enter') applyFilter();
				}}
			>
				<div class="menu-title">Filtrer « {d.e.t.columns[d.i]?.name} »</div>
				{#if source.sortTable}
					<!-- Tri en haut du panneau, comme Excel. -->
					<button
						class="sort"
						onclick={() => {
							filterDlg = null;
							menuAction(() => source.sortTable!(d.e.t, d.i, false));
						}}>Trier de A à Z</button
					>
					<button
						class="sort"
						onclick={() => {
							filterDlg = null;
							menuAction(() => source.sortTable!(d.e.t, d.i, true));
						}}>Trier de Z à A</button
					>
					<hr />
				{/if}
				<div class="add"><input class="search" bind:value={d.search} placeholder="Rechercher" /></div>
				<div class="values">
					<label class="all">
						<input
							type="checkbox"
							checked={shown.every((v) => v.on)}
							onchange={(ev) => {
								const on = (ev.currentTarget as HTMLInputElement).checked;
								for (const v of shown) v.on = on;
							}}
						/>
						(Sélectionner tout)
					</label>
					{#each shown as v (v.text)}
						<label>
							<input type="checkbox" bind:checked={v.on} />
							<span class="what">{v.text === '' ? '(Vides)' : v.text}</span>
							<span class="count">{v.count}</span>
						</label>
					{/each}
					{#if d.values.length >= 1000}<p class="menu-note">Seules les 1 000 premières valeurs sont listées.</p>{/if}
				</div>
				{#if d.e.t.columns[d.i]?.errors}
					<div class="add">
						<label><input type="checkbox" bind:checked={d.errors} /> Saisies hors type seulement ({d.e.t.columns[d.i]?.errors})</label>
					</div>
				{/if}
				{#if d.error}<p class="menu-note error">{d.error}</p>{/if}
				<div class="foot">
					{#if d.e.t.columns[d.i]?.filter}<button onclick={() => applyFilter(true)}>Effacer le filtre</button>{/if}
					<span class="sp"></span>
					<button class="primary" onclick={() => applyFilter()}>OK</button>
					<button onclick={closeFilter}>Annuler</button>
				</div>
			</div>
		{/if}
		{#if rulesDlg}
			{@const d = rulesDlg}
			{@const rules = currentRules(rulesRev)}
			{@const args = RULE_OPS.find((o) => o.op === d.op)?.args ?? 0}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="rules anchored"
				style:left="{d.x}px"
				style:top="{d.y}px"
				use:focusFirst
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Escape') closeRules();
				}}
			>
				<div class="menu-title">
					Mise en forme conditionnelle {d.line
						? d.line.rows
							? `des lignes ${d.line.i0 + 1}${d.line.i1 > d.line.i0 ? ` à ${d.line.i1 + 1}` : ''}`
							: `des colonnes ${addrA1(0, d.line.i0).slice(0, -1)}${d.line.i1 > d.line.i0 ? ` à ${addrA1(0, d.line.i1).slice(0, -1)}` : ''}`
						: `de « ${d.e?.t.columns[d.i]?.name} »`}
				</div>
				{#if rules.length === 0}
					<p class="menu-note">Aucune règle. La première règle vérifiée s’applique.</p>
				{/if}
				{#each rules as r, k (k)}
					<div class="rule">
						<span class="sample" style:background={r.style.bg ?? '#fff'} style:color={r.style.fg ?? '#1f2328'} style:font-weight={r.style.b ? 'bold' : 'normal'}>AaBb</span>
						<span class="what">{describeRule(r)}</span>
						<button title="Supprimer la règle" onclick={() => saveRules(rules.filter((_, j) => j !== k))}>✕</button>
					</div>
				{/each}
				<hr />
				<div class="add">
					<Select bind:value={d.op} options={RULE_OPS.map((o) => ({ value: o.op, label: o.label }))} />
					{#if args > 0}<input bind:value={d.value} placeholder="valeur" onkeydown={(ev) => ev.key === 'Enter' && addRule()} />{/if}
					{#if args > 1}<span>et</span><input bind:value={d.to} placeholder="valeur" onkeydown={(ev) => ev.key === 'Enter' && addRule()} />{/if}
					<Select bind:value={d.style} options={RULE_STYLES.map((s, k) => ({ value: k, label: s.label }))} />
					<button onclick={addRule}>Ajouter</button>
				</div>
				{#if d.error}<p class="menu-note error">{d.error}</p>{/if}
				<div class="foot">
					{#if rules.length}<button onclick={() => saveRules([])}>Effacer les règles</button>{/if}
					<span class="sp"></span>
					<button onclick={closeRules}>Fermer</button>
				</div>
			</div>
		{/if}
		{#if editing && (completion?.where === 'cell' || sig?.where === 'cell')}
			<div
				class="suggest"
				style:left="{suggestPos.x}px"
				style:top="{suggestPos.y}px"
			>
				{@render suggestions('cell')}
			</div>
		{/if}
		{#if editing}
			<input
				class="editor"
				bind:this={input}
				bind:value={editValueText}
				oninput={(e) => onFormulaInput(e.currentTarget, 'cell')}
				onkeyup={(e) => moveSig(e, 'cell')}
				onclick={(e) => moveSig(e, 'cell')}
				onkeydown={onEditorKey}
				onblur={() => {
					completion = sig = null;
					commit();
				}}
				title={(() => {
					const e = tableAt(active.row, active.col);
					return e ? typeLabel(e.t.columns[active.col - e.t.col].type) : 'Cellule libre';
				})()}
			/>
		{/if}
	</div>
	{#if panel?.kind === 'pivot'}
		{@const pv = pivots.find((p) => p.id === panel!.id)}
		{#if pv}
			<PivotPanel
				config={pv.config}
				fields={sourceColumns(pv.config.source)}
				error={pv.error}
				filterValues={source.pivotFieldValues ? (f) => source.pivotFieldValues!(pv.config.source, f) : undefined}
				onchange={(cfg) => menuAction(source.setPivot && (() => source.setPivot!(pv.id, cfg)))}
				ondelete={() => {
					panel = null;
					menuAction(source.deletePivot && (() => source.deletePivot!(pv.id)));
				}}
				onclose={() => (panel = null)}
			/>
		{/if}
	{:else if panel?.kind === 'chart'}
		{@const ch = charts.find((c) => c.id === panel!.id)}
		{#if ch}
			<ChartPanel
				config={ch.config}
				columns={sourceColumns(ch.config.source).map((c) => c.name)}
				onchange={(cfg) => menuAction(source.setChart && (() => source.setChart!(ch.id, cfg)))}
				onclose={() => {
					panel = null;
					selChart = null;
				}}
			/>
		{/if}
	{/if}
	</div>
</div>

<style>
	/* Bouton de validation des boîtes de dialogue : vert, comme Excel. */
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
	button.primary:disabled {
		opacity: 0.5;
	}
	.sheet-root {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.formula-bar {
		display: flex;
		align-items: center;
		gap: 6px;
		height: 30px;
		padding: 0 8px;
		border-bottom: 1px solid #d4d7db;
		background: #fff;
	}
	.name-box {
		width: 260px;
		font: inherit;
		outline: none;
		padding: 3px 8px;
		border: 1px solid #d4d7db;
		border-radius: 3px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		color: #374151;
	}
	.name-box.split {
		display: flex;
		align-items: center;
		padding: 0;
		overflow: visible;
	}
	.name-box.split input {
		font: inherit;
		min-width: 0;
		border: none;
		outline: none;
		padding: 3px 6px;
		background: none;
		color: #374151;
	}
	.name-box.split .table-name {
		flex: 0 1 45%;
		font-weight: 600;
	}
	.name-box.split .col-name {
		flex: 1 1 55%;
	}
	.name-box.split input:focus {
		background: #eef6f0;
	}
	.slash {
		color: #9aa0a6;
	}
	.fx {
		color: #6b7280;
		font-style: italic;
		padding: 0 6px;
		border-left: 1px solid #d4d7db;
	}
	.fx-input {
		flex: 1;
		font: inherit;
		border: none;
		outline: none;
		padding: 4px 2px;
	}
	/* Barre de recherche en surimpression (la grille ne bouge pas). */
	.finder {
		position: absolute;
		z-index: 7;
		top: 26px;
		right: 18px;
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 5px 8px 5px 10px;
		background: #f7f8f9;
		border: 1px solid #d4d7db;
		border-radius: 4px;
		box-shadow: 0 4px 14px rgba(0, 0, 0, 0.16);
	}
	.finder input {
		font: inherit;
		width: 200px;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	.finder button {
		font: inherit;
		padding: 2px 8px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.finder .link {
		border: none;
		background: none;
		color: #217346;
	}
	.finder .muted {
		color: #6b7280;
	}
	.bench {
		padding: 4px 10px;
		background: #eef6f1;
		color: #14532d;
		border-bottom: 1px solid #d3e7dc;
		white-space: nowrap;
		overflow: hidden;
	}
	.wrap {
		position: relative;
		flex: 1;
		min-height: 0;
		outline: none;
		overflow: hidden;
	}
	canvas {
		position: absolute;
		left: 0;
		top: 0;
	}
	.scroller {
		position: absolute;
		inset: 0;
		/* La barre horizontale est dans la ligne des onglets (SheetTabs). */
		overflow-x: hidden;
		overflow-y: auto;
	}
	.sheet-root {
		position: relative;
	}
	.suggest {
		position: absolute;
		z-index: 6;
		background: #fff;
		border: 1px solid #c9cdd2;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		min-width: 260px;
		max-width: 520px;
		font-size: 12px;
	}
	.fx-suggest {
		left: 300px;
		top: 30px;
	}
	.sig {
		padding: 5px 10px;
		background: #f7f8f9;
		color: #374151;
		border-bottom: 1px solid #e3e5e8;
		white-space: normal;
		line-height: 1.4;
	}
	.sig-line {
		font-family: Consolas, 'Courier New', monospace;
		color: #111827;
	}
	.sig-line b {
		color: #14532d;
	}
	.sig-desc {
		margin-top: 2px;
	}
	.sig-arg,
	.sig-ex {
		margin-top: 3px;
		color: #4b5563;
	}
	.sig code,
	.fn-doc code {
		font-family: Consolas, 'Courier New', monospace;
		background: #eef0f2;
		padding: 0 3px;
	}
	.sugg {
		display: flex;
		gap: 8px;
		align-items: center;
		padding: 3px 10px;
		cursor: pointer;
	}
	.sugg.on {
		background: #e3efe7;
	}
	.kind {
		width: 22px;
		color: #217346;
		font-weight: 600;
		text-align: center;
	}
	.lab {
		flex: 1;
	}
	.det {
		color: #6b7280;
		max-width: 260px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.menu {
		position: absolute;
		z-index: 5;
		background: #fff;
		border: 1px solid #d4d7db;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 240px;
	}
	.rules {
		position: absolute;
		z-index: 6;
		top: 60px;
		left: 50%;
		transform: translateX(-50%);
		background: #fff;
		border: 1px solid #d4d7db;
		box-shadow: 0 6px 18px rgba(0, 0, 0, 0.18);
		padding: 6px 0 8px;
		min-width: 460px;
	}
	.rules .rule,
	.rules .add,
	.rules .foot {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 3px 14px;
	}
	.rules .what {
		flex: 1;
	}
	.filter {
		min-width: 300px;
	}
	.rules.anchored {
		transform: none;
	}
	.filter .sort {
		display: block;
		width: 100%;
		text-align: left;
		padding: 5px 14px;
		border: none;
		border-radius: 0;
	}
	.filter .sort:hover {
		background: #e3efe7;
	}
	.charts {
		position: absolute;
		overflow: hidden;
		pointer-events: none;
	}
	.chart {
		position: absolute;
		pointer-events: auto;
		border: 1px solid #d4d7db;
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.12);
		background: #fff;
		cursor: move;
	}
	.chart-error {
		margin: 12px;
		color: #c0392b;
	}
	.body {
		flex: 1;
		display: flex;
		min-height: 0;
	}
	.chart.selected {
		outline: 2px solid #217346;
	}
	/* Poignée : un simple carré vert. */
	.handle {
		position: absolute;
		width: 8px;
		height: 8px;
		background: #217346;
	}
	.handle.n,
	.handle.s {
		left: calc(50% - 4px);
		cursor: ns-resize;
	}
	.handle.e,
	.handle.w {
		top: calc(50% - 4px);
		cursor: ew-resize;
	}
	.handle.n,
	.handle.ne,
	.handle.nw {
		top: -5px;
	}
	.handle.s,
	.handle.se,
	.handle.sw {
		bottom: -5px;
	}
	.handle.e,
	.handle.ne,
	.handle.se {
		right: -5px;
	}
	.handle.w,
	.handle.nw,
	.handle.sw {
		left: -5px;
	}
	.handle.ne,
	.handle.sw {
		cursor: nesw-resize;
	}
	.handle.nw,
	.handle.se {
		cursor: nwse-resize;
	}
	.filter .fn {
		display: flex;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		text-align: left;
		border: none;
		background: none;
		padding: 3px 8px;
		cursor: pointer;
	}
	.filter .fn:hover,
	.filter .fn.on {
		background: #e3efe7;
	}
	.filter .fn strong {
		flex: none;
	}
	.filter .fn-desc {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		text-align: right;
	}
	.fn-doc {
		margin: 4px 14px;
		max-width: 520px;
		line-height: 1.45;
	}
	.fn-doc > div {
		margin-top: 3px;
	}
	.fn-doc .fn-sig {
		font-family: Consolas, 'Courier New', monospace;
		font-weight: 600;
		color: #14532d;
	}
	.fn-doc dl {
		margin: 4px 0;
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 2px 10px;
	}
	.fn-doc dt {
		font-family: Consolas, 'Courier New', monospace;
		font-weight: 600;
	}
	.fn-doc dd {
		margin: 0;
		color: #4b5563;
	}
	.filter .search {
		width: 100%;
	}
	.filter .values {
		max-height: 260px;
		overflow: auto;
		margin: 4px 14px;
		border: 1px solid #e5e7eb;
		padding: 4px 0;
	}
	.filter label {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 1px 8px;
	}
	.filter .values input {
		width: auto;
	}
	.filter .count {
		color: #6b7280;
		font-size: 11px;
	}
	.rules .sample {
		padding: 1px 8px;
		border: 1px solid #d4d7db;
	}
	.rules input {
		font: inherit;
		width: 90px;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	.rules button {
		font: inherit;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.rules button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
	.lines-dlg input[type='radio'] {
		width: auto;
	}
	.rules .sp {
		flex: 1;
	}
	.rules hr {
		border: none;
		border-top: 1px solid #e5e7eb;
		margin: 6px 0;
	}
	.menu-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}
	.menu button {
		display: block;
		width: 100%;
		text-align: left;
		padding: 6px 14px;
		background: none;
		border: none;
		font: inherit;
		cursor: pointer;
	}
	.menu button:hover {
		background: #e3efe7;
	}
	.menu-note {
		margin: 4px 14px;
		max-width: 260px;
		font-size: 12px;
		color: #4b5563;
	}
	.menu-note.error {
		color: #c0392b;
	}
	.type-icon {
		display: inline-flex;
		vertical-align: -3px;
		color: #6b8f78;
		--icon-accent: #217346;
	}
	.check {
		display: inline-block;
		width: 16px;
		color: #217346;
	}
	.lines-dlg {
		left: 50%;
		top: 30%;
		transform: translateX(-50%);
		width: auto;
		min-width: 240px;
	}
	.lines-dlg label {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 3px 2px;
	}
	.menu hr {
		border: none;
		border-top: 1px solid #e3e5e8;
		margin: 4px 0;
	}
	.editor {
		position: absolute;
		left: 0;
		top: 0;
		height: 22px;
		box-sizing: border-box;
		border: 2px solid #217346;
		padding: 0 4px;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		outline: none;
		background: #fff;
		box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
		z-index: 2;
	}
</style>
