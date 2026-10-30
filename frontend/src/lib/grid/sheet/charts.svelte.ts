// Graphiques posés sur la feuille : sélection, déplacement et taille,
// copie (objet, image), collage, insertion depuis le ruban, carte. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { VScroll } from '../scroll';
import { Axis } from '../axis';
import { type AxisLike } from '../anim';
import { reveal } from '../panes';
import { type CellRange } from '../mergesel';
import { encodeObj } from '#lib/clip/objects.js';
import { SINGLE_SERIES, chartFieldsFrom, defaultChartTitle } from '#lib/chart/kinds.js';
import { svgToPng } from '#lib/chart/image.js';
import SheetButtons from '../SheetButtons.svelte';
import { BUTTON_H, BUTTON_W, type SheetButton } from '../buttons';
import { type FormulaHold } from '../crossref';
import { errorText, t as tr } from '#lib/i18n/index.js';
import { type Column, type SheetPivot, type ChartConfig, type SheetChart, type SheetSource, type Entry } from './model';

/** Ce que createCharts lit et modifie dans Sheet.svelte. */
export interface ChartsHost {
	selChart: number | null;
	panel: { kind: "pivot" | "chart"; id: number; } | null;
	readonly wrap: HTMLDivElement;
	readonly source: SheetSource;
	readonly cax: () => AxisLike;
	readonly rax: () => AxisLike;
	writing: Promise<void>;
	readonly rowTop: (r: number) => number;
	readonly say: (m: string) => void;
	readonly entries: Entry[];
	readonly sources: { name: string; columns: string[]; types: string[]; kind?: string; key?: number; }[];
	readonly CHART_W: number;
	readonly CHART_H: number;
	readonly locked: string;
	readonly structure: boolean;
	readonly zoomK: () => number;
	readonly hold: FormulaHold | null;
	readonly closeMenus: () => void;
	chartMenu: { ch: SheetChart; x: number; y: number; } | null;
	chartClip: ChartConfig | null;
	readonly charts: SheetChart[];
	readonly updateExtent: () => void;
	readonly vs: VScroll;
	readonly fRows: () => number;
	readonly scroller: HTMLDivElement;
	readonly viewW: number;
	readonly MARGIN: number;
	readonly fCols: () => number;
	readonly schedule: () => void;
	readonly syncNative: () => void;
	readonly motion: () => boolean;
	readonly colAxis: Axis;
	readonly rowAxis: Axis;
	readonly onrowlink: ((table: string, id: number) => void) | undefined;
	readonly active: { row: number; col: number; };
	readonly pivotAt: (r: number, c: number) => SheetPivot | undefined;
	readonly targetTable: () => Entry | undefined;
	readonly selection: () => CellRange;
	readonly targetCol: (e: Entry) => number;
}

export function createCharts(sh: ChartsHost) {

	function selectChart(ch: SheetChart) {
		sh.selChart = ch.id;
		sh.panel = { kind: 'chart', id: ch.id };
		sh.wrap.focus();
	}

	/** Déplace le graphique sélectionné au clavier (resize : le redimensionne), de dx, dy pixels. */
	function nudgeChart(ch: SheetChart, dx: number, dy: number, resize: boolean) {
		if (!sh.source.placeChart) return;
		const b = { ...chartBox(ch) };
		if (resize) {
			b.w = Math.max(160, b.w + dx);
			b.h = Math.max(120, b.h + dy);
		} else {
			b.x = Math.max(0, b.x + dx);
			b.y = Math.max(0, b.y + dy);
		}
		const col = sh.cax().indexAt(b.x);
		const row = sh.rax().indexAt(b.y);
		const place = sh.source.placeChart;
		sh.writing = sh.writing.catch(() => undefined).then(() =>
			place(ch.id, row, col, Math.round(b.x - sh.cax().offset(col)), Math.round(b.y - sh.rowTop(row)), Math.round(b.w), Math.round(b.h)).then(
				() => undefined,
				(err) => sh.say(errorText(err))
			)
		);
	}

	/** Colonnes du tableau source d'un graphique ou d'un TCD (autre feuille comprise). */
	function sourceColumns(name: string): { name: string; numeric: boolean; type: string }[] {
		const e = sh.entries.find((x) => x.t.name.toLowerCase() === name.toLowerCase());
		if (e) return e.t.columns.map((c) => ({ name: c.name, type: c.type, numeric: c.type === 'integer' || c.type === 'decimal' || c.type === 'formula' }));
		const t = sh.sources.find((x) => x.name.toLowerCase() === name.toLowerCase());
		return (t?.columns ?? []).map((c, i) => ({ name: c, type: t!.types[i], numeric: ['integer', 'decimal', 'formula'].includes(t!.types[i]) }));
	}

	/** Position et taille d'un graphique, en pixels de la feuille (ancrage libre). */
	function chartBox(ch: SheetChart) {
		if (chartGesture?.id === ch.id) return chartGesture.box;
		return {
			x: sh.cax().offset(ch.col) + (ch.config.offX ?? 0),
			y: sh.rowTop(ch.row) + (ch.config.offY ?? 0),
			w: ch.config.width ?? sh.CHART_W,
			h: ch.config.height ?? sh.CHART_H
		};
	}

	/** Position et taille d'un bouton ou contrôle, en pixels de la feuille (comme SheetButtons). */
	function buttonBox(b: SheetButton) {
		return {
			x: sh.cax().offset(b.col) + (b.config.offX ?? 0),
			y: sh.rowTop(b.row) + (b.config.offY ?? 0),
			w: b.config.width || BUTTON_W,
			h: b.config.height || BUTTON_H
		};
	}

	let chartGesture = $state<{ id: number; box: { x: number; y: number; w: number; h: number } } | null>(null);

	/** Déplace (dir vide) ou redimensionne (n, s, e, w et coins) un graphique. */
	function startChartGesture(ev: MouseEvent, ch: SheetChart, dir = '') {
		if (sh.locked || sh.structure) {
			ev.preventDefault(); // feuille protégée : le graphique ne se sélectionne pas
			return;
		}
		if (ev.button !== 0 || !sh.source.placeChart) return;
		ev.preventDefault();
		ev.stopPropagation();
		selectChart(ch);
		const b0 = chartBox(ch);
		const x0 = ev.clientX;
		const y0 = ev.clientY;
		chartGesture = { id: ch.id, box: { ...b0 } };
		const move = (e: MouseEvent) => {
			if (!chartGesture) return;
			const k = sh.zoomK();
			const dx = (e.clientX - x0) / k;
			const dy = (e.clientY - y0) / k;
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
			const col = sh.cax().indexAt(b.x);
			const row = sh.rax().indexAt(b.y);
			const place = sh.source.placeChart!;
			sh.writing = sh.writing.catch(() => undefined).then(() =>
				place(ch.id, row, col, Math.round(b.x - sh.cax().offset(col)), Math.round(b.y - sh.rowTop(row)), Math.round(b.w), Math.round(b.h)).then(
					() => {
						chartGesture = null;
					},
					(err) => {
						chartGesture = null;
						sh.say(errorText(err));
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
		if (sh.locked && !sh.hold) return;
		selectChart(ch);
		const r = sh.wrap.getBoundingClientRect();
		sh.closeMenus();
		sh.chartMenu = { ch, x: ev.clientX - r.left, y: ev.clientY - r.top };
	}

	async function copyChart(ch: SheetChart, cut: boolean) {
		sh.chartMenu = null;
		try {
			await navigator.clipboard.writeText(encodeObj({ kind: 'chart', config: ch.config as never }));
		} catch {
			sh.chartClip = ch.config;
		}
		sh.chartClip = ch.config;
		if (cut) await removeChart(ch.id);
		else sh.say(tr('grid.chart.copied'));
	}

	/** Copie le graphique comme image PNG (à coller dans un courriel, un document). */
	async function copyChartImage(ch: SheetChart) {
		sh.chartMenu = null;
		const svg = sh.wrap.querySelector<SVGSVGElement>('.chart.selected svg');
		if (!svg || sh.selChart !== ch.id) return;
		try {
			const blob = await svgToPng(svg);
			await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]);
			sh.say(tr('grid.chart.copiedImage'));
		} catch (err) {
			sh.say(tr('grid.chart.copyImageFailed', { error: errorText(err) }));
		}
	}

	/** Après la création d'un graphique : le nouveau est sélectionné (Excel), prêt à être déplacé, redimensionné ou mis en forme. */
	function selectNewChart(before: Set<number>): SheetChart | undefined {
		const ch = sh.charts.find((c) => !before.has(c.id));
		if (!ch) return;
		sh.selChart = ch.id;
		// Amené à l'écran, en entier s'il tient (son coin haut gauche sinon).
		sh.updateExtent();
		const b = chartBox(ch);
		const t = reveal(b.y, b.y + b.h, sh.vs.top, sh.vs.view, sh.rowTop(sh.fRows()));
		const l = reveal(b.x, b.x + b.w, sh.scroller.scrollLeft, sh.viewW - sh.MARGIN, sh.cax().offset(sh.fCols()));
		sh.wrap.focus();
		glideTo(t, l);
		return ch;
	}

	/** Défile jusqu'à (top, left) : en douceur avec les animations (Affichage › Animations), d'un coup sinon. */
	function glideTo(top: number, left: number) {
		const t0 = sh.vs.top;
		const l0 = sh.scroller.scrollLeft;
		if (top === t0 && left === l0) return sh.schedule();
		const set = (t: number, l: number) => {
			if (t !== sh.vs.top) sh.vs.to(t);
			if (l !== sh.scroller.scrollLeft) sh.scroller.scrollLeft = l;
			sh.syncNative();
			sh.schedule();
		};
		if (!sh.motion()) return set(top, left);
		const start = performance.now();
		const D = 320;
		const step = (now: number) => {
			const k = Math.min(1, (now - start) / D);
			const e = 1 - Math.pow(1 - k, 3); // décélération
			set(Math.round(t0 + (top - t0) * e), Math.round(l0 + (left - l0) * e));
			if (k < 1) requestAnimationFrame(step);
		};
		requestAnimationFrame(step);
	}

	/**
	 * Place libre pour un nouveau graphique (w × h px) à partir de (row, col) :
	 * sous les graphiques qu'il chevaucherait, avec une ligne d'écart, pour
	 * qu'aucun ne se superpose par défaut.
	 */
	function freeChartSpot(row: number, col: number, w = sh.CHART_W, h = sh.CHART_H): { row: number; col: number } {
		const x = sh.colAxis.offset(col);
		for (let k = 0; k < 50; k++) {
			const y = sh.rowTop(row);
			const hit = sh.charts.map(chartBox).find((b) => x < b.x + b.w && b.x < x + w && y < b.y + b.h && b.y < y + h);
			if (!hit) break;
			row = sh.rowAxis.indexAt(hit.y + hit.h) + 2;
		}
		return { row, col };
	}

	/** Carte zoomée ou déplacée (MapChart) : vue enregistrée dans sa configuration (annulable). */
	function setMapView(id: number, v: ChartConfig['mapView'] | null) {
		const ch = sh.charts.find((c) => c.id === id);
		if (!ch || !sh.source.setChart) return;
		const set = sh.source.setChart;
		sh.writing = sh.writing.catch(() => undefined).then(() => set(id, { ...ch.config, mapView: v ?? undefined }).then(undefined, (err) => sh.say(errorText(err))));
	}

	/** Point de carte cliqué : la carte se désélectionne (comme un clic dans la grille), puis la ligne est sélectionnée. */
	function pointToRow(ch: SheetChart, id: number) {
		if (sh.selChart === ch.id) sh.selChart = null;
		if (sh.panel?.kind === 'chart') sh.panel = null;
		sh.onrowlink?.(ch.config.source, id);
	}

	function pasteChart(cfg: ChartConfig) {
		if (!sh.source.createChart) return;
		const create = sh.source.createChart;
		const { row, col } = sh.active;
		const before = new Set(sh.charts.map((c) => c.id));
		sh.writing = sh.writing.catch(() => undefined).then(() =>
			create(freeChartSpot(row, col, cfg.width, cfg.height).row, col, cfg).then(
				() => {
					sh.say(tr('grid.chart.pasted'));
					selectNewChart(before);
				},
				(err) => sh.say(errorText(err))
			)
		);
	}

	/** Insère un graphique du tableau actif à sa droite (ruban Insertion). */
	function chartActive(kind: ChartConfig['kind']) {
		const pv = sh.pivotAt(sh.active.row, sh.active.col);
		if (pv && sh.source.createChart) {
			// Graphique d'un tableau croisé : X = premier champ de lignes, séries = mesures.
			const head = pv.cells[0];
			// En arborescence, une seule colonne d'étiquettes ; le graphique lit le TCD à plat (premier champ).
			const nk = pv.rows ? 1 : Math.max(1, pv.config.rowFields?.length ?? 0);
			const x = pv.rows ? (pv.config.rowFields?.[0] ?? head[0]) : head[0];
			let series = head.slice(nk).filter((h) => !h.startsWith('Total'));
			if (SINGLE_SERIES.has(kind)) series = series.slice(0, 1);
			const create = sh.source.createChart;
			const name = pv.config.name ?? '';
			const before = new Set(sh.charts.map((c) => c.id));
			sh.writing = sh.writing.catch(() => undefined).then(() =>
				create(freeChartSpot(pv.row, pv.col + head.length + 1).row, pv.col + head.length + 1, { source: name, kind, x, series, title: defaultChartTitle(series) }).then(
					() => {
						sh.say(tr('grid.chart.pivotInserted', { name }));
						selectNewChart(before);
					},
					(err) => sh.say(errorText(err))
				)
			);
			return;
		}
		const e = sh.targetTable();
		if (!e || !sh.source.createChart) {
			sh.say(tr('grid.chart.placeActive'));
			return;
		}
		const cols = e.t.columns;
		const num = (c: Column) => c.type === 'integer' || c.type === 'decimal' || (c.type === 'formula' && c.formula !== undefined);
		// Plusieurs colonnes du tableau sélectionnées : elles fixent X et les séries (comme Excel).
		const { c0, c1 } = sh.selection();
		const picked: number[] = [];
		for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol); c++) picked.push(c - e.t.col);
		const { x: xName, series } = chartFieldsFrom(kind, cols.map((c) => ({ name: c.name, numeric: num(c) })), picked, sh.targetCol(e));
		const x = cols.find((c) => c.name === xName) ?? cols[0];
		if (kind === 'map') {
			// Carte : un carré grisé, en attente de ses coordonnées ; le volet
			// s'ouvre avec les colonnes repérées par le classeur.
			const create = sh.source.createChart;
			const before = new Set(sh.charts.map((c) => c.id));
			const side = sh.CHART_H + 32;
			const at = freeChartSpot(e.t.row, e.lastCol + 2, side, side);
			sh.writing = sh.writing.catch(() => undefined).then(() =>
				create(at.row, e.lastCol + 2, { source: e.t.name, kind, x: '', series: [], mapDraft: true, width: side, height: side }).then(
					() => {
						sh.say(tr('grid.chart.mapInserted'));
						const ch = selectNewChart(before);
						if (ch) sh.panel = { kind: 'chart', id: ch.id };
					},
					(err) => sh.say(errorText(err))
				)
			);
			return;
		}
		if (!series.length) {
			sh.say(tr('grid.chart.noNumbers'));
			return;
		}
		const create = sh.source.createChart;
		const before = new Set(sh.charts.map((c) => c.id));
		sh.writing = sh.writing.catch(() => undefined).then(() =>
			create(freeChartSpot(e.t.row, e.lastCol + 2).row, e.lastCol + 2, { source: e.t.name, kind, x: x.name, series, title: defaultChartTitle(series) }).then(
				() => {
					sh.say(tr('grid.chart.inserted'));
					selectNewChart(before);
				},
				(err) => sh.say(errorText(err))
			)
		);
	}

	function editChart(ch: SheetChart) {
		if ((sh.locked || sh.structure) && !sh.hold) return;
		selectChart(ch);
	}
	async function removeChart(id: number) {
		sh.chartMenu = null;
		if (sh.selChart === id) sh.selChart = null;
		if (sh.panel?.kind === 'chart' && sh.panel.id === id) sh.panel = null;
		await sh.source.deleteChart?.(id);
		sh.wrap.focus();
	}

	return {
		chartActive,
		selectChart,
		nudgeChart,
		sourceColumns,
		chartBox,
		buttonBox,
		startChartGesture,
		onChartMenu,
		copyChart,
		copyChartImage,
		setMapView,
		pointToRow,
		pasteChart,
		editChart,
		removeChart
	};
}
