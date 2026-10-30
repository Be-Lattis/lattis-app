// Animations après une modification (anim.ts) : tableaux qui glissent ou
// grandissent, barres, valeurs qui clignotent, graphiques qui poussent. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { Axis } from '../axis';
import { ease, flashAlpha, lerpBar, motionAt, progress, sameBar, shiftAxis, tableMotions, zeroBar, type AxisLike, type BarSpan, type Geo, type LinesShift, type Motion } from '../anim';
import { type MergeRange } from '../sheetfmt';
import { mergeRect } from '../mergesel';
import { grown as growFrame, type Grown } from '../grown';
import { type SheetChart, type TableAnim, type Entry } from './model';

/** Ce que createAnim lit et modifie dans Sheet.svelte. */
export interface AnimHost {
	animNow: number;
	animBusy: boolean;
	rowDraw: AxisLike;
	readonly rowAxis: Axis;
	colDraw: AxisLike;
	readonly colAxis: Axis;
	lines: (LinesShift & { t0: number; rows: boolean }) | null;
	readonly motion: () => boolean;
	drawing: boolean;
	geoDrawn: Map<number, Geo>;
	grownSeen: Map<number, string>;
	valsNow: Map<string, string>;
	barsNow: Map<string, { t: BarSpan; s: BarSpan; }>;
	readonly flashes: Map<string, number>;
	readonly barAnims: Map<string, { from: BarSpan; to: BarSpan; t0: number; }>;
	readonly motions: Map<number, Motion>;
	readonly inner: Map<number, { t0: number; rows: boolean; at: number; n: number; px: number; }>;
	valsPrev: Map<string, string>;
	barsPrev: Map<string, { t: BarSpan; s: BarSpan; }>;
	viewPrev: { r0: number; r1: number; c0: number; c1: number } | null;
	viewNow: { r0: number; r1: number; c0: number; c1: number } | null;
	readonly entries: Entry[];
	readonly GROW_MS: number;
	readonly schedule: () => void;
	navAt: number;
	readonly NAV_SETTLE: number;
	linesHint: { t: number; rows: boolean; at: number; n: number; px: number; table?: number } | null;
	editAt: number;
	chartTween: boolean;
	grownCharts: Set<number>;
	readonly charts: SheetChart[];
	readonly viewH: number;
	readonly viewW: number;
	readonly rh: (r: number) => number;
	readonly cax: () => AxisLike;
	readonly rspan: (r0: number, r1: number) => number;
	readonly topRow: (t: { row: number; twoLevel?: boolean; }) => number;
	readonly rowTop: (r: number) => number;
	tableAnimsAt: number;
	tableAnims: Map<number, TableAnim | null>;
	readonly noOff: () => number;
	readonly ctx: CanvasRenderingContext2D;
	readonly EDIT_WINDOW: number;
	columnFlash: { table: number; name: string; t: number; done: Set<string> } | null;
	readonly typed: { row: number; col: number; t: number; };
	readonly boxOf: (r: number, c: number) => MergeRange | undefined;
	readonly active: { row: number; col: number; };
	readonly locked: string;
	readonly structure: boolean;
	readonly keptOut: (row: number, col: number) => boolean;
	readonly inRangeZone: (row: number, col: number) => boolean;
}

export function createAnim(sh: AnimHost) {
	/** Début d'une image : axes animés, valeurs de l'image précédente. */
	function beginAnimFrame(now: number) {
		sh.animNow = now;
		sh.animBusy = false;
		sh.rowDraw = sh.rowAxis;
		sh.colDraw = sh.colAxis;
		if (sh.lines) {
			const k = progress(now, sh.lines.t0);
			if (k >= 1 || !sh.motion()) sh.lines = null;
			else {
				sh.animBusy = true;
				if (sh.lines.rows) sh.rowDraw = shiftAxis(sh.rowAxis, sh.lines, ease(k));
				else sh.colDraw = shiftAxis(sh.colAxis, sh.lines, ease(k));
			}
		}
		sh.drawing = true;
		const drawn = sh.geoDrawn;
		sh.geoDrawn = new Map();
		if (!sh.motion()) {
			sh.grownSeen.clear();
			if (sh.valsNow.size || sh.barsNow.size || sh.flashes.size || sh.barAnims.size || sh.motions.size || sh.inner.size) {
				sh.valsNow = new Map();
				sh.barsNow = new Map();
				sh.flashes.clear();
				sh.barAnims.clear();
				sh.motions.clear();
				sh.inner.clear();
			}
			return;
		}
		sh.valsPrev = sh.valsNow;
		sh.valsNow = new Map();
		sh.barsPrev = sh.barsNow;
		sh.barsNow = new Map();
		sh.viewPrev = sh.viewNow;
		growMotions(drawn, now);
	}

	function growMotions(drawn: Map<number, Geo>, now: number) {
		const seen = new Map<number, string>();
		for (const e of sh.entries) {
			const g = grown(e);
			const base = `${e.t.row},${e.t.col},${e.last},${e.lastCol}`;
			const k = `${base}|${g.last},${g.lastCol},${g.firstCol}`;
			seen.set(e.t.id, k);
			const was = sh.grownSeen.get(e.t.id);
			// Tableau lui-même modifié (saisie, insertion) : startAnims s'en charge.
			if (!was || was === k || !was.startsWith(base + '|')) continue;
			const from = drawn.get(e.t.id);
			if (from && !sh.motions.get(e.t.id)?.born) sh.motions.set(e.t.id, { t0: now, from, ms: sh.GROW_MS, grow: true });
		}
		sh.grownSeen = seen;
	}

	/** Fin d'une image : vrais axes, image suivante s'il reste une animation. */
	function endAnimFrame() {
		sh.drawing = false;
		if (sh.animBusy) sh.schedule();
	}

	/**
	 * Animations d'une nouvelle mise en page (après rebuildAxis et rebuildRows) :
	 * insertion ou suppression demandée, tableaux déplacés, agrandis ou nés.
	 */
	function startAnims(nav: boolean, known: Set<number>, oldCharts: Set<number>, now: number) {
		if (nav) sh.navAt = now;
		else if (now - sh.navAt < sh.NAV_SETTLE) nav = true;
		const hint = sh.linesHint && now - sh.linesHint.t < 3000 ? sh.linesHint : null;
		sh.linesHint = null;
		if (nav || !sh.motion()) {
			sh.editAt = -Infinity;
			sh.chartTween = false;
			sh.lines = null;
			sh.motions.clear();
			sh.inner.clear();
			sh.flashes.clear();
			sh.barAnims.clear();
			sh.valsNow = new Map();
			sh.barsNow = new Map();
			sh.viewNow = null;
			if (sh.grownCharts.size) sh.grownCharts = new Set();
			return;
		}
		sh.editAt = now;
		sh.chartTween = true;
		const born = sh.charts.filter((c) => !oldCharts.has(c.id)).map((c) => c.id);
		if (born.length || sh.grownCharts.size) sh.grownCharts = new Set(born);
		if (hint && hint.table === undefined) {
			// Lignes ou colonnes de la feuille : tout glisse avec l'axe ; les
			// cellules libres ont changé de place, leurs valeurs ne s'éclairent pas.
			const view = hint.rows ? sh.viewH : sh.viewW;
			sh.lines = { t0: now, rows: hint.rows, at: hint.at, n: hint.n, px: Math.min(hint.px, view) };
			sh.motions.clear();
			sh.inner.clear();
			sh.valsNow = new Map();
			sh.barsNow = new Map();
			sh.schedule();
			return;
		}
		// Une même modification peut arriver en deux mises en page (relecture,
		// puis événement du classeur) : les animations en cours continuent.
		if (sh.lines) return;
		const geo = new Map(sh.entries.map((e) => [e.t.id, tableGeo(e)]));
		for (const [id, m] of tableMotions(sh.geoDrawn, known, geo, now)) if (!sh.motions.get(id)?.born) sh.motions.set(id, m);
		for (const id of sh.inner.keys()) if (!geo.has(id)) sh.inner.delete(id);
		if (hint && hint.table !== undefined && sh.entries.some((e) => e.t.id === hint.table)) sh.inner.set(hint.table, { t0: now, rows: hint.rows, at: hint.at, n: hint.n, px: hint.px });
		sh.schedule();
	}

	/** Insertion ou suppression demandée (lignes ou colonnes de la feuille, ou d'un tableau). */
	function hintLines(rows: boolean, at: number, n: number, table?: number) {
		if (!sh.motion()) return;
		const px = n > 0 ? (rows ? sh.rh(at) : sh.cax().size(at)) : rows ? sh.rspan(at, at - n - 1) : sh.cax().offset(at - n) - sh.cax().offset(at);
		sh.linesHint = { t: performance.now(), rows, at, n, px, table };
	}

	/** Cadre affiché d'un tableau (prolongé quand la cellule active est juste dessous, à droite ou à gauche), en pixels de la feuille. */
	function tableGeo(e: Entry): Geo {
		const g = grown(e);
		const x = sh.cax().offset(g.firstCol);
		// La bande des groupes fait partie du cadre.
		const top = sh.topRow(e.t);
		const y = sh.rowTop(top);
		return { x, y, w: sh.cax().offset(g.lastCol + 1) - x, h: sh.rspan(top, g.last) };
	}

	/** Mouvement d'un tableau à cette image (null : immobile). */
	function tableAnim(e: Entry): TableAnim | null {
		if (!sh.motions.size && !sh.inner.size) return null;
		if (sh.tableAnimsAt !== sh.animNow) {
			sh.tableAnims = new Map();
			sh.tableAnimsAt = sh.animNow;
		}
		const id = e.t.id;
		if (sh.tableAnims.has(id)) return sh.tableAnims.get(id)!;
		let out: TableAnim | null = null;
		const to = tableGeo(e);
		const m = sh.motions.get(id);
		if (m) {
			const k = progress(sh.animNow, m.t0, m.ms);
			if (k >= 1) sh.motions.delete(id);
			else {
				sh.animBusy = true;
				const g = motionAt(m, to, ease(k), sh.rh(e.t.row));
				// Cadre seul étiré : il s'ouvre vers la gauche, le contenu ne glisse pas.
				out = m.grow ? { dx: 0, dy: 0, ox: g.x - to.x, w: g.w, h: g.h, alpha: g.alpha, rowOff: sh.noOff, colOff: sh.noOff } : { dx: g.x - to.x, dy: g.y - to.y, ox: 0, w: g.w, h: g.h, alpha: g.alpha, rowOff: sh.noOff, colOff: sh.noOff };
			}
		}
		const s = sh.inner.get(id);
		if (s) {
			const k = progress(sh.animNow, s.t0);
			if (k >= 1) sh.inner.delete(id);
			else {
				sh.animBusy = true;
				const left = (1 - ease(k)) * s.px;
				// Insertion : ce qui suit la nouvelle ligne part de son ancienne place ;
				// suppression : ce qui suivait la ligne supprimée remonte.
				const off = (i: number) => (s.n > 0 ? (i > s.at ? -left : 0) : i >= s.at ? left : 0);
				out ??= { dx: 0, dy: 0, ox: 0, w: to.w, h: to.h, alpha: 1, rowOff: sh.noOff, colOff: sh.noOff };
				if (s.rows) out.rowOff = off;
				else out.colOff = off;
			}
		}
		sh.tableAnims.set(id, out);
		return out;
	}

	/**
	 * Entre dans le repère d'un tableau en mouvement : décalage, opacité, et
	 * découpe à son cadre animé (agrandi de ce que la saisie prolonge) ;
	 * faux s'il est immobile (rien à restaurer).
	 */
	function enterTable(e: Entry, cx: (c: number) => number, ry: (r: number) => number): boolean {
		const a = tableAnim(e);
		if (!a) return false;
		sh.ctx.save();
		sh.ctx.translate(a.dx, a.dy);
		sh.ctx.globalAlpha *= a.alpha;
		sh.ctx.beginPath();
		sh.ctx.rect(cx(grown(e).firstCol) + a.ox - 1, ry(sh.topRow(e.t)) - 1, a.w + 2, a.h + 2);
		sh.ctx.clip();
		return true;
	}

	/** Barre de données affichée pour la cellule k (sheet r, c) : de l'ancienne valeur, ou de zéro, à la nouvelle. */
	function barShown(k: string, b: BarSpan & { axis?: number; rev?: boolean }, r: number, c: number): BarSpan {
		if (!sh.motion()) return b;
		const target = { a: b.a, b: b.b };
		const prev = sh.barsPrev.get(k);
		let a = sh.barAnims.get(k);
		if (sh.animNow - sh.editAt < sh.EDIT_WINDOW) {
			const v = sh.viewPrev;
			if (prev && !sameBar(prev.t, target)) sh.barAnims.set(k, (a = { from: prev.s, to: target, t0: sh.animNow }));
			else if (!prev && !a && v && r >= v.r0 && r <= v.r1 && c >= v.c0 && c <= v.c1) sh.barAnims.set(k, (a = { from: zeroBar(b), to: target, t0: sh.animNow }));
		}
		let s: BarSpan = target;
		if (a) {
			const p = progress(sh.animNow, a.t0);
			if (p >= 1 || !sameBar(a.to, target)) sh.barAnims.delete(k);
			else {
				s = lerpBar(a.from, target, ease(p));
				sh.animBusy = true;
			}
		}
		sh.barsNow.set(k, { t: target, s });
		return s;
	}

	function columnFlashes(table: number, name: string, k: string) {
		const f = sh.columnFlash;
		if (!f || f.table !== table || f.name !== name || f.done.has(k)) return;
		if (sh.animNow - f.t > 1500) {
			sh.columnFlash = null;
			return;
		}
		f.done.add(k);
		sh.flashes.set(k, sh.animNow);
	}

	/** Éclat d'une valeur calculée (clé k, cellule r, c) qui vient de changer : opacité (0 : aucun). */
	function flashOf(k: string, v: string, r: number, c: number): number {
		if (!sh.motion()) return 0;
		const prev = sh.valsPrev.get(k);
		sh.valsNow.set(k, v);
		if (prev !== undefined && prev !== v && sh.animNow - sh.editAt < sh.EDIT_WINDOW && (r !== sh.typed.row || c !== sh.typed.col || sh.animNow - sh.typed.t > sh.EDIT_WINDOW)) sh.flashes.set(k, sh.animNow);
		const t0 = sh.flashes.get(k);
		if (t0 === undefined) return 0;
		const a = flashAlpha(sh.animNow, t0);
		if (a <= 0) sh.flashes.delete(k);
		else sh.animBusy = true;
		return a;
	}

	/** Pose l'éclat d'une cellule ou d'une zone (sous son texte). */
	function paintFlash(a: number, x: number, y: number, w: number, h: number) {
		if (a <= 0) return;
		sh.ctx.fillStyle = `rgba(33, 115, 70, ${(0.2 * a).toFixed(3)})`;
		sh.ctx.fillRect(x, y, w, h);
	}

	function activeRect() {
		const g = sh.boxOf(sh.active.row, sh.active.col);
		return g ? mergeRect(g) : { r0: sh.active.row, r1: sh.active.row, c0: sh.active.col, c1: sh.active.col };
	}

	/** Étendue affichée d'un tableau : une ligne ou une colonne de plus quand la
	 * cellule active est juste dessous ou juste à droite (la saisie l'étendra),
	 * jusqu'au tableau voisin qu'elle rejoindrait (fusion). */
	function grown(e: Entry): Grown {
		return growFrame(e, { row: sh.active.row, col: sh.active.col, entries: sh.entries, locked: !!sh.locked, structure: sh.structure, keptOut: sh.keptOut, inRange: sh.inRangeZone });
	}

	return {
		beginAnimFrame,
		endAnimFrame,
		startAnims,
		hintLines,
		tableGeo,
		tableAnim,
		enterTable,
		barShown,
		columnFlashes,
		flashOf,
		paintFlash,
		activeRect,
		grown
	};
}
