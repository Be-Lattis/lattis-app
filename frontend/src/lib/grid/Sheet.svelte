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
	import { formatNumber, mergeStyles, parseStyle, type CellStyle } from './style';
	import { dismiss } from './dismiss';
	import { describeRule, parseRules, ruleStyle, RULE_OPS, RULE_STYLES, type Rule, type RuleOp } from './rules';
	import { apply as applySuggestion, signature, suggest, type Catalog, type Completion } from './autocomplete';
	import {
		isErrorValue,
		valueAlignment,
		alignment,
		defaultWidth,
		editValue,
		formatValue,
		typeIcon,
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
		columns: Column[];
	}

	export interface SheetData {
		tables: SheetTable[];
		/** value : résultat d'une formule (sinon raw est affiché). */
		cells: { row: number; col: number; raw: string; value?: string; error?: boolean }[];
		/** Largeurs choisies par l'utilisateur (sinon : selon le type). */
		widths?: { col: number; width: number }[];
		/** Cellules nommées de la feuille. */
		names?: { name: string; row: number; col: number }[];
		/** Mise en forme des cellules hors tableaux (JSON). */
		styles?: { row: number; col: number; style: string }[];
	}

	/** Cible d'une saisie, pour la source. */
	export interface Target {
		table?: SheetTable;
		index?: number; // ligne de données
		column?: number; // colonne du tableau
		rowId?: number; // clé technique de la ligne
	}

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
		insertRow?(table: SheetTable, n: number): Promise<unknown>;
		deleteRow?(table: SheetTable, n: number): Promise<unknown>;
		insertColumn?(table: SheetTable, i: number): Promise<unknown>;
		deleteColumn?(table: SheetTable, i: number): Promise<unknown>;
		/** Nombre de valeurs qui ne se convertiraient pas au type to. */
		convertPreview?(table: SheetTable, i: number, to: ColumnType): Promise<number>;
		setType?(table: SheetTable, i: number, to: ColumnType): Promise<unknown>;
		setColumnWidth?(col: number, width: number): Promise<unknown>;
		/** Nomme une cellule libre (vide : retire le nom). */
		setName?(row: number, col: number, name: string): Promise<unknown>;
		sortTable?(table: SheetTable, i: number, desc: boolean): Promise<unknown>;
		/** Met en forme le rectangle sélectionné (une valeur vide retire la propriété). */
		applyStyle?(r0: number, c0: number, r1: number, c1: number, patch: Record<string, unknown>): Promise<unknown>;
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
		/** Renouvelle AUJOURDHUI, MAINTENANT et ALEA (F9). */
		recalc?(): Promise<unknown>;
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
	}

	interface Props {
		data: SheetData;
		source: SheetSource;
		/** Fonctions et tableaux du classeur, pour l'aide à la saisie des formules. */
		catalog?: Catalog;
		onstatus?: (s: SheetStatus) => void;
		/** Lance la mesure dès l'ouverture et transmet le résultat (mode SLITABLE_BENCH). */
		autoBench?: (result: string) => void;
	}

	let { data, source, catalog, onstatus, autoBench }: Props = $props();

	export const SHEET_ROWS = 1_048_576;
	export const SHEET_COLS = 16_384;
	const ROW_H = 22;
	const DEF_W = 88;
	const TOP_H = 22;
	const MARGIN = 52;
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
		error: '#c0392b'
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
	let freeStyles = new Map<string, CellStyle>(); // position → style (hors tableaux)
	// Styles de colonne et d'en-tête, par tableau puis colonne.
	let colStyles = new Map<number, (CellStyle | undefined)[]>();
	let headStyles = new Map<number, (CellStyle | undefined)[]>();
	let colRules = new Map<number, (Rule[] | undefined)[]>();
	let shown = new Map<string, { text: string; error: boolean }>(); // formules des cellules libres
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
		shown = new Map(d.cells.filter((c) => c.raw.startsWith('=')).map((c) => [key(c.row, c.col), { text: c.value ?? '', error: !!c.error }]));
		userWidths = new Map((d.widths ?? []).map((w) => [w.col, w.width]));
		names = new Map((d.names ?? []).map((n) => [key(n.row, n.col), n.name]));
		freeStyles = new Map((d.styles ?? []).map((s) => [key(s.row, s.col), parseStyle(s.style) ?? {}]));
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
	function rebuildAxis(override?: { col: number; width: number }) {
		const widths = new Map<number, number>();
		for (const e of entries) {
			e.t.columns.forEach((c, i) => {
				if (!widths.has(e.t.col + i)) widths.set(e.t.col + i, defaultWidth(c.type));
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
	const vs = new VScroll(100 * ROW_H, 0);
	let spacer: HTMLDivElement;
	let extentCols = 26;

	function updateExtent() {
		if (!scroller) return;
		const viewRows = Math.max(1, Math.ceil(vs.view / ROW_H));
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
		const lastVisible = Math.floor((vs.top + vs.view) / ROW_H);
		const rows = Math.min(SHEET_ROWS, Math.max(usedRow + 1, lastVisible + 1) + viewRows);
		const cw = Math.max(0, viewW - MARGIN);
		const lastVisibleCol = colAxis.indexAt(scroller.scrollLeft + cw);
		const viewCols = Math.max(1, Math.ceil(cw / DEF_W));
		extentCols = Math.min(SHEET_COLS, Math.max(usedCol + 1, lastVisibleCol + 1) + viewCols);
		vs.content = rows * ROW_H;
		vs.to(vs.top);
		if (spacer) {
			spacer.style.height = `${TOP_H + vs.spacer}px`;
			spacer.style.width = `${MARGIN + colAxis.offset(extentCols)}px`;
		}
	}
	let viewW = 0;
	let viewH = 0;

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
	let sig = $state<{ text: string; where: 'cell' | 'fx' } | null>(null);
	let suggestPos = $state({ x: 0, y: 0 });

	function onFormulaInput(el: HTMLInputElement, where: 'cell' | 'fx') {
		if (!catalog) return;
		suggestPos = { x: MARGIN + colAxis.offset(active.col) - scroller.scrollLeft, y: TOP_H + (active.row + 1) * ROW_H - vs.top + 2 };
		const caret = el.selectionStart ?? el.value.length;
		const e = tableAt(active.row, active.col);
		const c = suggest(el.value, caret, catalog, e?.t.name);
		completion = c ? { ...c, sel: 0, where } : null;
		const s = signature(el.value, caret, catalog);
		sig = s ? { text: s, where } : null;
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
	let fxEditing = false;

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

	let frame = 0;
	function schedule() {
		if (mounted && !frame) frame = requestAnimationFrame(draw);
	}

	function visibleRows() {
		const first = Math.floor(vs.top / ROW_H);
		const last = Math.min(SHEET_ROWS - 1, Math.floor((vs.top + vs.view - 1) / ROW_H));
		return { first, last };
	}

	/** Lignes de données d'un tableau visibles (indices de données), ou null. */
	function visibleData(e: Entry, first: number, last: number) {
		const a = Math.max(first, e.t.row + 1);
		const b = Math.min(last, e.last);
		return a <= b ? { first: a - e.t.row - 1, last: b - e.t.row - 1 } : null;
	}

	function isNumeric(s: string) {
		return /^[-+]?[\d\s  ]+([,.]\d+)?%?$/.test(s.trim());
	}

	function draw() {
		frame = 0;
		const t0 = performance.now();
		const w = viewW;
		const h = viewH;
		const dpr = window.devicePixelRatio || 1;
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		ctx.fillStyle = C.bg;
		ctx.fillRect(0, 0, w, h);

		const left = scroller.scrollLeft;
		const { first: r0, last: r1 } = visibleRows();
		const c0 = colAxis.indexAt(left);
		const c1 = colAxis.indexAt(left + w - MARGIN);
		const cx = (c: number) => MARGIN + colAxis.offset(c) - left;
		const ry = (r: number) => TOP_H + r * ROW_H - vs.top;
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
			ctx.fillRect(cx(e.t.col), ry(e.t.row), spanW(e.t.col, g.lastCol), ROW_H);
		}

		// Plage sélectionnée.
		const s0 = Math.min(active.row, anchor.row);
		const s1 = Math.max(active.row, anchor.row);
		const k0 = Math.min(active.col, anchor.col);
		const k1 = Math.max(active.col, anchor.col);
		if (s0 !== s1 || k0 !== k1) {
			ctx.fillStyle = C.select;
			ctx.fillRect(cx(k0), ry(s0), spanW(k0, k1), (s1 - s0 + 1) * ROW_H);
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
			ctx.fillText(`⧩ ${n} ligne${n > 1 ? 's' : ''} masquée${n > 1 ? 's' : ''} par le filtre`, cx(e.t.col) + 6, ry(r) + ROW_H / 2);
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
						const hs = headStyles.get(e.t.id)?.[i];
						if (hs?.bg) {
							ctx.fillStyle = hs.bg;
							ctx.fillRect(x, y, cw, ROW_H);
						}
						ctx.font = FONT_SMALL;
						ctx.fillStyle = C.icon;
						ctx.textAlign = 'right';
						const icon = (col.filter ? '⧩ ' : '') + typeIcon(col.type) + ' ▾';
						if (col.errors) {
							ctx.fillStyle = C.error;
							ctx.fillText('⚠ ' + col.errors, x + cw - 6 - ctx.measureText(icon + ' ').width, y + ROW_H / 2);
							ctx.fillStyle = C.icon;
						}
						ctx.fillText(icon, x + cw - 6, y + ROW_H / 2);
						ctx.save();
						ctx.beginPath();
						ctx.rect(x, y, cw - ctx.measureText(icon).width - 10, ROW_H);
						ctx.clip();
						ctx.font = (hs?.i ? 'italic ' : '') + FONT_BOLD;
						ctx.fillStyle = hs?.fg ?? C.headText;
						ctx.textAlign = 'left';
						ctx.fillText(col.name, x + 6, y + ROW_H / 2);
						ctx.restore();
						continue;
					}
					const row = e.cache.row(r - e.t.row - 1);
					if (!row) {
						ctx.fillStyle = C.placeholder;
						ctx.fillRect(x + 6, y + 7, cw * 0.6, ROW_H - 14);
						continue;
					}
					const v = row.cells[i];
					const bad = row.bad?.has(i) || (col.type === 'formula' && isErrorValue(v));
					const cs = mergeStyles(mergeStyles(colStyles.get(e.t.id)?.[i], parseStyle(row.sty?.get(i))), ruleStyle(colRules.get(e.t.id)?.[i], v, row.text[i] ?? '', !!bad));
					if (cs?.bg) {
						ctx.fillStyle = cs.bg;
						ctx.fillRect(x, y, cw, ROW_H);
					}
					let s = row.text[i];
					if (!s) continue;
					if (cs?.nf && typeof row.cells[i] === 'number' && !row.bad?.has(i)) s = formatNumber(cs.nf, row.cells[i] as number) ?? s;
					if (row.bad?.has(i)) {
						// Saisie hors type : valeur brute en rouge, coin rouge comme le
						// triangle d'Excel.
						ctx.font = FONT;
						ctx.textAlign = 'left';
						ctx.fillStyle = C.error;
						ctx.fillText(s, x + 6, y + ROW_H / 2);
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
				}
				ctx.restore();
			}
		}

		// Cellules libres : texte à gauche, nombres à droite, comme Excel.
		ctx.font = FONT;
		ctx.fillStyle = C.text;
		for (const [k, fs] of freeStyles) {
			if (!fs.bg) continue;
			const [r, c] = k.split(',').map(Number);
			if (r < r0 || r > r1 || c < c0 || c > c1) continue;
			ctx.fillStyle = fs.bg;
			ctx.fillRect(cx(c), ry(r), colAxis.size(c), ROW_H);
		}
		for (const [k, rawText] of cells) {
			const [r, c] = k.split(',').map(Number);
			if (r < r0 || r > r1 || c < c0 || c > c1) continue;
			const fstyle = freeStyles.get(k);
			const x = cx(c);
			const cw = colAxis.size(c);
			const f = shown.get(k);
			let raw = f ? f.text : rawText;
			const num = isNumeric(raw);
			if (num && fstyle?.nf) {
				const n = Number(raw.replace(/[\s\u00a0\u202f]/g, '').replace(',', '.'));
				if (Number.isFinite(n)) raw = formatNumber(fstyle.nf, n) ?? raw;
			}
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
			ctx.fillText(raw, num ? x + cw - 6 : x + 6, ry(r) + ROW_H / 2);
			ctx.restore();
		}

		// Quadrillage de toute la feuille.
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
			ctx.strokeRect(x, y, spanW(e.t.col, g.lastCol), (g.last - e.t.row + 1) * ROW_H);
			ctx.beginPath();
			ctx.moveTo(x, y + ROW_H);
			ctx.lineTo(x + spanW(e.t.col, g.lastCol), y + ROW_H);
			ctx.stroke();
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
			ctx.strokeRect(cx(c0 + dc) + 1, ry(r0 + dr) + 1, spanW(c0 + dc, c1 + dc) - 2, (r1 - r0 + 1) * ROW_H - 2);
			ctx.restore();
		}
		// Cellule active.
		if (active.row >= r0 - 1 && active.row <= r1 + 1) {
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 2;
			ctx.strokeRect(cx(active.col) + 0.5, ry(active.row) + 0.5, colAxis.size(active.col) - 1, ROW_H - 1);
		}
		ctx.restore();

		// Bandeau du haut : vide (pas de lettres de colonnes), sauf quand
		// l'en-tête d'un tableau visible est sorti de l'écran.
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, 0, w, TOP_H);
		for (const e of visible) {
			if (e.t.row >= r0 || e.last < r0) continue;
			for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol); c++) {
				const x = cx(c);
				ctx.save();
				ctx.beginPath();
				ctx.rect(Math.max(x, MARGIN), 0, colAxis.size(c), TOP_H);
				ctx.clip();
				ctx.fillStyle = C.head;
				ctx.fillRect(x, 0, colAxis.size(c), TOP_H);
				ctx.font = FONT_BOLD;
				ctx.fillStyle = C.headText;
				ctx.textAlign = 'left';
				ctx.fillText(e.t.columns[c - e.t.col].name, x + 6, TOP_H / 2);
				ctx.restore();
			}
		}
		for (let c = Math.max(c0, k0); c <= Math.min(c1, k1); c++) {
			ctx.fillStyle = C.active;
			ctx.fillRect(cx(c), TOP_H - 2, colAxis.size(c), 2);
		}

		// Marge de gauche : numéros 1..n des lignes de données des tableaux.
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, TOP_H, MARGIN, h - TOP_H);
		ctx.font = FONT_SMALL;
		ctx.textAlign = 'right';
		const cur = tableAt(active.row, active.col);
		for (let r = r0; r <= r1; r++) {
			const y = ry(r);
			if (r >= s0 && r <= s1) {
				ctx.fillStyle = C.activeStrip;
				ctx.fillRect(0, y, MARGIN, ROW_H);
			}
			const e = cur && r > cur.t.row && r <= cur.last ? cur : visible.find((e) => r > e.t.row && r <= e.last);
			if (e) {
				ctx.fillStyle = C.stripText;
				ctx.fillText(String(r - e.t.row), MARGIN - 8, y + ROW_H / 2);
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
		ctx.fillText(s, tx, y + ROW_H / 2);
		if (st?.u) {
			const w = ctx.measureText(s).width;
			const lx = align === 'left' ? tx : align === 'right' ? tx - w : tx - w / 2;
			ctx.fillRect(lx, y + ROW_H / 2 + 7, w, 1);
		}
		ctx.restore();
	}

	/** Style de la cellule active (pour basculer gras, italique…). */
	function activeStyle(): CellStyle {
		const { row, col } = active;
		const e = tableAt(row, col);
		if (!e) return freeStyles.get(key(row, col)) ?? {};
		const i = col - e.t.col;
		if (row === e.t.row) return headStyles.get(e.t.id)?.[i] ?? {};
		return mergeStyles(colStyles.get(e.t.id)?.[i], parseStyle(e.cache.row(row - e.t.row - 1)?.sty?.get(i))) ?? {};
	}

	/** Met en forme la sélection ; une bascule (b, i, u) suit la cellule active. */
	export function applyStyle(patch: Record<string, unknown>) {
		if (!source.applyStyle) return;
		const p = { ...patch };
		for (const k of ['b', 'i', 'u'] as const) if (p[k] === 'toggle') p[k] = !activeStyle()[k];
		const r0 = Math.min(active.row, anchor.row);
		const r1 = Math.max(active.row, anchor.row);
		const c0 = Math.min(active.col, anchor.col);
		const c1 = Math.max(active.col, anchor.col);
		const apply = source.applyStyle;
		menuAction(() => apply(r0, c0, r1, c1, p));
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
	function cellName(r: number, c: number): string {
		const named = names.get(key(r, c));
		if (named) return named;
		const e = tableAt(r, c);
		if (!e) return cells.has(key(r, c)) ? 'Cellule libre' : '';
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
			cell: cellName(active.row, active.col),
			message,
			fetch: `bloc p50 ${ms(percentile(stats.roundTrip, 0.5))}`,
			draw: `rendu ${ms(percentile(drawTimes, 0.5))}`,
			fps: `${frameStamps.length} img/s`,
			edit: `saisie ${ms(lastEditMs)}`,
			blocks: entries.reduce((n, e) => n + e.cache.size, 0),
			stats: selectionStats()
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
		values: { text: string; count: number; on: boolean }[];
		search: string;
		errors: boolean;
		error: string;
	} | null>(null);

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
	let rulesDlg = $state<{ e: Entry; i: number; op: RuleOp; value: string; to: string; style: number; error: string } | null>(null);

	/** Ouvre le dialogue des règles de la colonne i du tableau e. */
	function openRules(e: Entry, i: number) {
		menu = null;
		rulesDlg = { e, i, op: '>', value: '', to: '', style: 0, error: '' };
	}

	/** Règles de la colonne de la cellule active (ruban Accueil). */
	export function rulesActive() {
		const e = tableAt(active.row, active.col);
		if (!e || !source.setRules) {
			say('Placez la cellule active dans la colonne d’un tableau');
			return;
		}
		openRules(e, active.col - e.t.col);
	}

	async function saveRules(rules: Rule[]) {
		const d = rulesDlg;
		if (!d || !source.setRules) return;
		try {
			await source.setRules(d.e.t, d.i, rules);
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
		// Le tableau a pu être rechargé : relire ses colonnes par identifiant.
		const t = entries.find((x) => x.t.id === d.e.t.id)?.t ?? d.e.t;
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

	function refreshFx() {
		nameBox = cellName(active.row, active.col);
		const e = tableAt(active.row, active.col);
		if (e) {
			const k = e.t.columns[active.col - e.t.col];
			tableBox = { name: e.t.name, col: active.row === e.t.row ? k.name : `${k.name}[${active.row - e.t.row}]` };
		} else tableBox = null;
		if (fxEditing) return;
		fxText = cellInfo(active.row, active.col).value;
	}

	function say(m: string) {
		message = m;
		refreshStatus();
	}

	// --- Défilement -------------------------------------------------------

	function onScroll() {
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
		if (dy) {
			// Défiler vers le bas agrandit la feuille d'autant.
			if (dy > 0) vs.content = Math.min(SHEET_ROWS * ROW_H, Math.max(vs.content, vs.top + dy + 2 * vs.view));
			vs.by(dy);
			updateExtent();
			scroller.scrollTop = vs.toNative();
		}
		if (dx) {
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
		const top = row * ROW_H;
		if (top < vs.top) vs.to(top);
		else if (top + ROW_H > vs.top + vs.view) vs.to(top + ROW_H - vs.view);
		const left = scroller.scrollLeft;
		const cw = viewW - MARGIN;
		const x0 = colAxis.offset(col);
		const x1 = x0 + colAxis.size(col);
		if (x0 < left) scroller.scrollLeft = x0;
		else if (x1 > left + cw) scroller.scrollLeft = x1 - cw;
		syncNative();
	}

	// --- Sélection et clavier ---------------------------------------------

	function moveTo(row: number, col: number, extend = false) {
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
		const row = Math.min(SHEET_ROWS - 1, Math.floor((y - TOP_H + vs.top) / ROW_H));
		return { row, col };
	}

	// --- Largeur des colonnes : glisser une séparation du bandeau du haut ----

	let resizing: { col: number; startX: number; startW: number; width: number } | null = null;

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

	/** La souris est-elle sur la bordure de la sélection (poignée de déplacement) ? */
	function onSelectionEdge(e: MouseEvent): boolean {
		if (!source.moveTable) return false;
		const r = scroller.getBoundingClientRect();
		const x = e.clientX - r.left;
		const y = e.clientY - r.top;
		if (x < MARGIN || y < TOP_H) return false;
		const { r0, r1, c0, c1 } = selection();
		const cx = (c: number) => MARGIN + colAxis.offset(c) - scroller.scrollLeft;
		const ry = (r: number) => TOP_H + r * ROW_H - vs.top;
		const left = cx(c0);
		const right = cx(c1) + colAxis.size(c1);
		const top = ry(r0);
		const bottom = ry(r1) + ROW_H;
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
			say('Pour déplacer, sélectionnez un tableau entier (Ctrl+A) ou des colonnes entières, en-tête compris (Ctrl+Espace)');
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

	function onMouseDown(e: MouseEvent) {
		if (e.button === 0 && !editing && onSelectionEdge(e)) {
			const cell = cellAt(e);
			if (cell) {
				e.preventDefault();
				moving = { sel: selection(), from: cell };
				return;
			}
		}
		const b = boundaryAt(e);
		if (b !== null && source.setColumnWidth) {
			e.preventDefault();
			resizing = { col: b, startX: e.clientX, startW: colAxis.size(b), width: colAxis.size(b) };
			return;
		}
		const cell = cellAt(e);
		if (!cell) return;
		if (editing) commit();
		e.preventDefault();
		if (e.button === 0 && openTypeMenu(e, cell)) return;
		wrap.focus();
		moveTo(cell.row, cell.col, e.shiftKey);
		dragging = true;
	}

	function onMouseMove(e: MouseEvent) {
		if (resizing) {
			resizing.width = Math.max(24, Math.round(resizing.startW + e.clientX - resizing.startX));
			rebuildAxis({ col: resizing.col, width: resizing.width });
			schedule();
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
		scroller.style.cursor = source.setColumnWidth && boundaryAt(e) !== null ? 'col-resize' : !dragging && onSelectionEdge(e) ? 'move' : '';
		if (!dragging) return;
		const cell = cellAt(e);
		if (!cell) return;
		active = cell;
		schedule();
	}

	function onDblClick(e: MouseEvent) {
		if (cellAt(e)) startEdit(false);
	}

	function onKeyDown(e: KeyboardEvent) {
		if (editing) return;
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
				moveTo(row, col + (e.shiftKey ? -1 : 1));
				break;
			case 'Enter':
				moveTo(row + (e.shiftKey ? -1 : 1), col);
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
		const y = TOP_H + active.row * ROW_H - vs.top;
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

	async function clearSelection() {
		const r0 = Math.min(active.row, anchor.row);
		const r1 = Math.max(active.row, anchor.row);
		const c0 = Math.min(active.col, anchor.col);
		const c1 = Math.max(active.col, anchor.col);
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
				moveTo(row + (e.shiftKey ? -1 : 1), col);
				break;
			case 'Tab':
				commit();
				moveTo(row, col + (e.shiftKey ? -1 : 1));
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

	function onCopy(e: ClipboardEvent) {
		if (document.activeElement !== wrap || editing) return;
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
		say(`${rows.length * (c1 - c0 + 1)} cellule(s) copiée(s)`);
	}

	function onPaste(e: ClipboardEvent) {
		if (document.activeElement !== wrap || editing || !source.paste) return;
		const text = e.clipboardData?.getData('text/plain') ?? '';
		const block = parseTSV(text);
		if (block.length === 0) return;
		e.preventDefault();
		const { row, col } = active;
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
	}

	// --- Menu de type (icône de l'en-tête) ------------------------------------

	const TYPES: ColumnType[] = ['text', 'integer', 'decimal', 'date', 'boolean'];
	let typeMenu = $state<{ x: number; y: number; e: Entry; i: number; pending?: { to: ColumnType; n: number } } | null>(null);

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
		typeMenu = { x: mx, y: ev.clientY - r.top + 8, e, i: cell.col - e.t.col };
		return true;
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

	let menu = $state<{ x: number; y: number; e: Entry; n: number; i: number; header: boolean } | null>(null);

	function onContextMenu(ev: MouseEvent) {
		const cell = cellAt(ev);
		ev.preventDefault();
		menu = null;
		if (!cell) return;
		const e = tableAt(cell.row, cell.col);
		if (!e) return;
		if (cell.row !== active.row || cell.col !== active.col) moveTo(cell.row, cell.col);
		const r = wrap.getBoundingClientRect();
		menu = { x: ev.clientX - r.left, y: ev.clientY - r.top, e, n: cell.row - e.t.row - 1, i: cell.col - e.t.col, header: cell.row === e.t.row };
	}

	async function menuAction(fn: (() => Promise<unknown>) | undefined) {
		menu = null;
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
		const tableTop = (e.t.row + 1) * ROW_H;
		const tableSpan = Math.max(0, e.t.rows * ROW_H - vs.view);

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
			viewW = scroller.clientWidth;
			viewH = scroller.clientHeight;
			canvas.width = Math.round(viewW * dpr);
			canvas.height = Math.round(viewH * dpr);
			canvas.style.width = `${viewW}px`;
			canvas.style.height = `${viewH}px`;
			vs.view = viewH - TOP_H;
			updateExtent();
			syncNative();
		};
		const ro = new ResizeObserver(resize);
		ro.observe(wrap);
		resize();
		scroller.addEventListener('wheel', onWheel, { passive: false });
		const up = () => {
			dragging = false;
			if (moving) dropMove();
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

{#snippet suggestions(where: 'cell' | 'fx')}
	{#if sig?.where === where}<div class="sig">{sig.text}</div>{/if}
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
				<span class="det">{s.detail}</span>
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
	{#if finder}
		<div class="finder">
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
			<span class="sp"></span>
			<button onclick={() => {
				finder = null;
				wrap.focus();
			}} title="Fermer (Échap)">✕</button>
		</div>
	{/if}
	{#if completion?.where === 'fx' || sig?.where === 'fx'}
		<div class="suggest fx-suggest">{@render suggestions('fx')}</div>
	{/if}

	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	<div class="wrap" bind:this={wrap} tabindex="0" role="grid" onkeydown={onKeyDown}>
		<canvas bind:this={canvas}></canvas>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="scroller"
			bind:this={scroller}
			onscroll={onScroll}
			onmousedown={onMouseDown}
			onmousemove={onMouseMove}
			ondblclick={onDblClick}
			oncontextmenu={onContextMenu}
		>
			<div bind:this={spacer}></div>
		</div>
		{#if typeMenu}
			{@const m = typeMenu}
			{@const col = m.e.t.columns[m.i]}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (typeMenu = null)}>
				<div class="menu-title">Type de « {col.name} » : {col.fixed ? 'fixé' : 'inféré automatiquement'}</div>
				{#if m.pending}
					<p class="menu-note">
						{m.pending.n > 1
							? `${m.pending.n} valeurs ne peuvent pas devenir « ${typeLabel(m.pending.to)} » : elles seront gardées et marquées en erreur.`
							: `1 valeur ne peut pas devenir « ${typeLabel(m.pending.to)} » : elle sera gardée et marquée en erreur.`}
					</p>
					<button onclick={() => chooseType(m.pending!.to, true)}>Convertir quand même</button>
					<button onclick={() => (typeMenu = { ...m, pending: undefined })}>Annuler</button>
				{:else}
					{#each TYPES as t (t)}
						<button onclick={() => chooseType(t)}>
							<span class="check">{t === col.type ? '✓' : ''}</span>{typeIcon(t)}&nbsp;&nbsp;{typeLabel(t)}
						</button>
					{/each}
					{#if col.errors}<p class="menu-note error">{col.errors} saisie{col.errors > 1 ? 's' : ''} hors type dans la colonne</p>{/if}
				{/if}
			</div>
		{/if}
		{#if menu}
			{@const m = menu}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (menu = null)}>
				<div class="menu-title">{m.e.t.name}</div>
				{#if !m.header}
					<button onclick={() => menuAction(source.insertRow && (() => source.insertRow!(m.e.t, m.n)))}>Insérer une ligne au-dessus</button>
					<button onclick={() => menuAction(source.insertRow && (() => source.insertRow!(m.e.t, m.n + 1)))}>Insérer une ligne en dessous</button>
					<button onclick={() => menuAction(source.deleteRow && (() => source.deleteRow!(m.e.t, m.n)))}>Supprimer la ligne</button>
					<hr />
				{/if}
				{#if source.sortTable && !m.e.t.columns[m.i]?.formula}
					<button onclick={() => menuAction(() => source.sortTable!(m.e.t, m.i, false))}>Trier de A à Z (croissant)</button>
					<button onclick={() => menuAction(() => source.sortTable!(m.e.t, m.i, true))}>Trier de Z à A (décroissant)</button>
					<hr />
				{/if}
				{#if source.setFilter && !m.e.t.columns[m.i]?.formula}
					<button onclick={() => openFilter(m.e, m.i)}>Filtrer par « {m.e.t.columns[m.i]?.name} »…</button>
				{/if}
				{#if source.clearFilters && m.e.t.columns.some((c) => c.filter)}
					<button onclick={() => menuAction(() => source.clearFilters!(m.e.t))}>Effacer le filtre du tableau</button>
				{/if}
				{#if source.setRules}
					<button onclick={() => openRules(m.e, m.i)}>Mise en forme conditionnelle…</button>
					<hr />
				{/if}
				<button onclick={() => menuAction(source.insertColumn && (() => source.insertColumn!(m.e.t, m.i)))}>Insérer une colonne à gauche</button>
				<button onclick={() => menuAction(source.insertColumn && (() => source.insertColumn!(m.e.t, m.i + 1)))}>Insérer une colonne à droite</button>
				<button onclick={() => menuAction(source.deleteColumn && (() => source.deleteColumn!(m.e.t, m.i)))}>Supprimer la colonne « {m.e.t.columns[m.i]?.name} »</button>
			</div>
		{/if}
		{#if filterDlg}
			{@const d = filterDlg}
			{@const shown = filterShown()}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="rules filter"
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Escape') closeFilter();
					if (ev.key === 'Enter') applyFilter();
				}}
			>
				<div class="menu-title">Filtrer « {d.e.t.columns[d.i]?.name} »</div>
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
					<button onclick={() => applyFilter()}>OK</button>
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
				class="rules"
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Escape') closeRules();
				}}
			>
				<div class="menu-title">Mise en forme conditionnelle de « {d.e.t.columns[d.i]?.name} »</div>
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
					<select bind:value={d.op}>
						{#each RULE_OPS as o (o.op)}<option value={o.op}>{o.label}</option>{/each}
					</select>
					{#if args > 0}<input bind:value={d.value} placeholder="valeur" onkeydown={(ev) => ev.key === 'Enter' && addRule()} />{/if}
					{#if args > 1}<span>et</span><input bind:value={d.to} placeholder="valeur" onkeydown={(ev) => ev.key === 'Enter' && addRule()} />{/if}
					<select bind:value={d.style}>
						{#each RULE_STYLES as s, k (k)}<option value={k}>{s.label}</option>{/each}
					</select>
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
</div>

<style>
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
	.finder {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 4px 10px;
		background: #f7f8f9;
		border-bottom: 1px solid #d4d7db;
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
	.finder .sp {
		flex: 1;
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
		overflow: auto;
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
		max-width: 420px;
		font-size: 12px;
	}
	.fx-suggest {
		left: 300px;
		top: 30px;
	}
	.sig {
		padding: 4px 10px;
		background: #f7f8f9;
		color: #374151;
		border-bottom: 1px solid #e3e5e8;
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
	.rules select,
	.rules button {
		font: inherit;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
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
	.check {
		display: inline-block;
		width: 16px;
		color: #217346;
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
