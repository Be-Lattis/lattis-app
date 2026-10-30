// Défilement de la feuille : barre horizontale, doigt (touch.ts), main et
// barre d'espace (pan.ts), molette, et objet gardé à l'écran quand un volet
// s'ouvre à droite. Sorti de Sheet.svelte (docs/decoupage-sheet.md).
import { VScroll, clamp } from '../scroll';
import { ease, progress, type AxisLike } from '../anim';
import { stepZoom } from '../zoom';
import { isDrag, inertia, LONG_PRESS_MS, MIN_SPEED, pinchZoom, Velocity } from '../touch';
import { onObject, PanGesture, pansWith, SpaceHold } from '../pan';
import { revealWhole } from '../panes';
import { keepInView, seenIn, type Span as XSpan } from '../keepinview';
import { type MergeRange } from '../sheetfmt';
import { type CellRange } from '../mergesel';
import { type FormulaHold } from '../crossref';
import { t as tr } from '#lib/i18n/index.js';
import { type SheetPivot, type SheetChart } from './model';
import { SHEET_ROWS, SHEET_COLS, ROW_H } from './consts';

/** Ce que createScrolling lit et modifie dans Sheet.svelte. */
export interface ScrollingHost {
	readonly holdCols: number;
	readonly growAtEnd: (rows: boolean) => void;
	readonly scroller: HTMLDivElement;
	readonly updateExtent: () => void;
	readonly schedule: () => void;
	readonly toLocal: (e: MouseEvent) => { x: number; y: number; };
	holdRows: number;
	readonly rax: () => AxisLike;
	readonly vs: VScroll;
	readonly space: SpaceHold;
	readonly editing: boolean;
	readonly fxEditing: boolean;
	readonly pan: PanGesture<MouseEvent>;
	readonly wrap: HTMLDivElement;
	readonly holdWhileScrolling: () => void;
	noteTip: { x: number; y: number; text: string; key: string; } | null;
	dragging: boolean;
	headerDrag: "rows" | "cols" | null;
	panCursor: "" | "grab" | "grabbing";
	readonly zoomK: () => number;
	readonly onContextMenu: (ev: MouseEvent) => void;
	readonly locked: string;
	readonly hold: FormulaHold | null;
	readonly dvPick: { active: { row: number; col: number; }; anchor: { row: number; col: number; }; whole: { kind: "rows" | "cols"; from: number; range: CellRange; anchor: { row: number; col: number; }; active: { row: number; col: number; }; } | null; sheet: number; back?: boolean; ref?: string | null; } | null;
	readonly startEdit: (byTyping: boolean, initial?: string) => void;
	readonly onzoom: ((z: number) => void) | undefined;
	readonly zoom: number;
	touch: { id: number; x0: number; y0: number; x: number; y: number; t0: number; drag: boolean; vel: Velocity; } | null;
	readonly cellAt: (e: MouseEvent) => { row: number; col: number; } | null;
	readonly active: { row: number; col: number; };
	readonly selChart: number | null;
	readonly listMenu: { x: number; y: number; values: string[]; row: number; col: number; error?: string; loading?: boolean; } | null;
	readonly onDblClick: (e: MouseEvent) => void;
	zoomWheel: number;
	readonly rowTop: (r: number) => number;
	readonly cax: () => AxisLike;
	readonly spacer: HTMLDivElement;
	readonly viewW: number;
	readonly MARGIN: number;
	readonly extentCols: number;
	readonly motion: () => boolean;
	readonly boxOf: (r: number, c: number) => MergeRange | undefined;
	readonly charts: SheetChart[];
	readonly chartBox: (ch: SheetChart) => { x: number; y: number; w: number; h: number; };
	readonly panel: { kind: "pivot" | "chart"; id: number; } | null;
	readonly pivots: SheetPivot[];
	readonly anchor: { row: number; col: number; };
	readonly fCols: () => number;
	readonly fRows: () => number;
}

export function createScrolling(h: ScrollingHost) {
	// --- Défilement -------------------------------------------------------

	/** Défilement horizontal piloté par la barre de la ligne des onglets. */
	function scrollLeftTo(x: number, atEnd = false) {
		if (atEnd && h.holdCols) h.growAtEnd(false);
		h.scroller.scrollLeft = x;
		h.updateExtent();
		h.schedule();
	}

	// Barre verticale (native) tirée : repérée au clic dans sa zone.
	function onScrollbarDown(e: PointerEvent) {
		if (h.toLocal(e).x < h.scroller.clientWidth) return;
		h.holdRows = h.rax().indexAt(h.vs.content);
		const up = () => {
			h.holdRows = 0;
			window.removeEventListener('pointerup', up);
			h.updateExtent();
		};
		window.addEventListener('pointerup', up);
	}

	let pinch: { z0: number; d0: number } | null = null;
	const fingers = new Map<number, { x: number; y: number }>();
	let fling = 0;

	function stopFling() {
		if (fling) cancelAnimationFrame(fling);
		fling = 0;
	}

	/** Élan au lâcher, (vx, vy) en px de la feuille par ms. */
	function startFling(vx: number, vy: number) {
		stopFling();
		if (Math.hypot(vx, vy) < MIN_SPEED) return;
		let last = performance.now();
		let rx = 0;
		let ry = 0;
		const step = (now: number) => {
			const dt = Math.min(50, now - last);
			last = now;
			const a = inertia(vx, dt);
			const b = inertia(vy, dt);
			vx = a.v;
			vy = b.v;
			// Pixels entiers : le reste s'ajoute à l'image suivante.
			rx += a.d;
			ry += b.d;
			const dx = Math.trunc(rx);
			const dy = Math.trunc(ry);
			rx -= dx;
			ry -= dy;
			const moved = scrollByPx(dx, dy);
			// Bord de la feuille atteint : l'élan s'arrête dans ce sens.
			if (dx && !moved.dx) vx = 0;
			if (dy && !moved.dy) vy = 0;
			fling = Math.hypot(vx, vy) >= MIN_SPEED ? requestAnimationFrame(step) : 0;
		};
		fling = requestAnimationFrame(step);
	}

	/** Appui d'un bouton qui fait glisser la feuille : true si le clic s'arrête là. */
	function panDown(e: MouseEvent): boolean {
		const sp = e.button === 0 && h.space.down && !h.editing && !h.fxEditing;
		if (!pansWith(e.button, sp)) return false;
		h.pan.begin(e.button, e.clientX, e.clientY);
		if (e.button === 2) return false; // le clic droit garde son rôle (cellule, menu)
		e.preventDefault(); // milieu : pas de défilement automatique du navigateur
		if (sp) h.space.click();
		h.wrap.focus();
		return true;
	}

	function panMove(e: MouseEvent) {
		if (!h.pan.active) return;
		const was = h.pan.panning;
		const d = h.pan.move(e.clientX, e.clientY);
		if (!d) return;
		if (!was) {
			stopFling();
			stopGlide();
			h.holdWhileScrolling();
			h.noteTip = null;
			h.dragging = false;
			h.headerDrag = null;
			h.panCursor = 'grabbing';
		}
		e.preventDefault();
		const k = h.zoomK();
		scrollByPx(d.dx / k, d.dy / k);
	}

	function panUp(e: MouseEvent) {
		const r = h.pan.end(e.button, performance.now());
		if (!r) return;
		h.panCursor = h.space.down && !h.editing && !h.fxEditing ? 'grab' : '';
		// Clic droit sans glissé dont le menu est arrivé à l'appui (macOS) : il
		// s'ouvre maintenant ; sur un objet posé sur la feuille, le sien.
		const m = r.menu;
		if (!m) return;
		const at = m.target instanceof Element ? m.target : null;
		if (at && !h.scroller?.contains(at) && h.wrap?.contains(at)) at.dispatchEvent(new MouseEvent('contextmenu', m));
		else h.onContextMenu(m);
	}

	/**
	 * Appui sur un objet posé sur la feuille (capture sur leur calque) : clic
	 * du milieu ou Espace + gauche font glisser la feuille sans toucher à
	 * l'objet ; le clic droit glissé aussi, et sans glissé l'objet ouvre son
	 * menu. Le clic gauche seul reste à l'objet.
	 */
	function objectPanDown(e: MouseEvent) {
		const sp = e.button === 0 && h.space.down && !h.editing && !h.fxEditing;
		const r = onObject(e.button, sp);
		if (r === 'object') return;
		panDown(e);
		if (r === 'pan') e.stopPropagation();
	}

	/** Menu contextuel (capture sur la fenêtre) : retenu ou écarté pendant le geste. */
	function panMenu(e: MouseEvent) {
		if (h.pan.menu(e, performance.now()) === 'open') return;
		e.preventDefault();
		e.stopPropagation();
	}

	function panReset() {
		h.pan.cancel();
		h.space.reset();
		h.panCursor = '';
	}

	/** Espace en attente saisie dans la cellule active (suivie de extra). */
	function typeSpace(extra = '') {
		if ((h.locked && !h.hold) || h.dvPick || h.editing || h.fxEditing) return;
		h.startEdit(true, ' ' + extra);
	}

	/** Espace sur la grille (hors saisie) : la main ; l'espace se saisit au relâcher, sans clic entre-temps. */
	function spaceKey(e: KeyboardEvent): boolean {
		if (h.fxEditing) return false;
		const mod = e.ctrlKey || e.metaKey || e.altKey;
		if (e.key === ' ' && !mod && !e.shiftKey) {
			e.preventDefault();
			if (!e.repeat && !h.space.down) {
				h.space.press();
				if (!h.pan.active) h.panCursor = 'grab';
			}
			return true;
		}
		// Frappe rapide (« a b ») : l'espace tenue passe avant la touche suivante.
		if (h.space.down && !['Shift', 'Control', 'Alt', 'Meta'].includes(e.key) && h.space.flush() && e.key.length === 1 && !mod) {
			e.preventDefault();
			typeSpace(e.key);
			return true;
		}
		return false;
	}

	function onSpaceUp(e: KeyboardEvent) {
		if ((e.key !== ' ' && e.code !== 'Space') || !h.space.down) return;
		const typed = h.space.release();
		if (!h.pan.active) h.panCursor = '';
		if (typed && h.wrap?.contains(document.activeElement)) typeSpace();
	}

	function onPointerDown(e: PointerEvent) {
		if (e.pointerType !== 'touch') return onScrollbarDown(e);
		// Pas d'événements souris simulés : le geste est traité ici.
		e.preventDefault();
		stopFling();
		stopGlide();
		fingers.set(e.pointerId, { x: e.clientX, y: e.clientY });
		if (fingers.size === 2) {
			const [a, b] = [...fingers.values()];
			pinch = h.onzoom ? { z0: h.zoom, d0: Math.hypot(a.x - b.x, a.y - b.y) } : null;
			h.touch = null;
			return;
		}
		if (fingers.size > 1) return;
		h.touch = { id: e.pointerId, x0: e.clientX, y0: e.clientY, x: e.clientX, y: e.clientY, t0: e.timeStamp, drag: false, vel: new Velocity() };
		h.touch.vel.add(e.timeStamp, e.clientX, e.clientY);
	}

	function onPointerMove(e: PointerEvent) {
		if (e.pointerType !== 'touch' || !fingers.has(e.pointerId)) return;
		fingers.set(e.pointerId, { x: e.clientX, y: e.clientY });
		if (pinch) {
			const [a, b] = [...fingers.values()];
			if (!a || !b) return;
			const z = pinchZoom(pinch.z0, pinch.d0, Math.hypot(a.x - b.x, a.y - b.y));
			if (z !== h.zoom) h.onzoom?.(z);
			return;
		}
		const t = h.touch;
		if (!t || t.id !== e.pointerId) return;
		if (!t.drag && !isDrag(e.clientX - t.x0, e.clientY - t.y0)) return;
		if (!t.drag) {
			t.drag = true;
			h.holdWhileScrolling();
			h.noteTip = null;
		}
		const k = h.zoomK();
		scrollByPx((t.x - e.clientX) / k, (t.y - e.clientY) / k);
		t.x = e.clientX;
		t.y = e.clientY;
		t.vel.add(e.timeStamp, e.clientX, e.clientY);
	}

	function onPointerUp(e: PointerEvent) {
		if (e.pointerType !== 'touch' || !fingers.delete(e.pointerId)) return;
		if (pinch || fingers.size) {
			// Fin d'un pincement : aucun appui ni élan.
			if (fingers.size < 2) pinch = null;
			h.touch = null;
			return;
		}
		const t = h.touch;
		h.touch = null;
		if (!t || t.id !== e.pointerId || e.type === 'pointercancel') return;
		if (t.drag) {
			const k = h.zoomK();
			const { vx, vy } = t.vel.get(e.timeStamp);
			startFling(-vx / k, -vy / k);
			return;
		}
		if (e.timeStamp - t.t0 >= LONG_PRESS_MS) return; // appui long : menu contextuel
		tapAt(e.clientX, e.clientY);
	}

	// Le lever du doigt est traité par onPointerUp : touchend annulé (ontouchend),
	// sans quoi le navigateur donnerait ensuite le focus à la grille, ôté à l'éditeur.

	/** Appui bref : un clic ; sur la cellule déjà active, la saisie (comme un double clic). */
	function tapAt(clientX: number, clientY: number) {
		const init = { bubbles: true, cancelable: true, clientX, clientY, button: 0, buttons: 1, detail: 1, view: window };
		const probe = new MouseEvent('mousedown', init);
		const cell = h.cellAt(probe);
		const again = !h.editing && !!cell && cell.row === h.active.row && cell.col === h.active.col && h.selChart === null;
		h.scroller.dispatchEvent(probe);
		window.dispatchEvent(new MouseEvent('mouseup', { ...init, buttons: 0 }));
		if (again && cell && !h.editing && !h.listMenu && h.active.row === cell.row && h.active.col === cell.col) h.onDblClick(new MouseEvent('dblclick', { ...init, detail: 2 }));
	}

	function onScroll() {
		h.holdWhileScrolling();
		h.noteTip = null;
		if (h.holdRows && h.scroller.scrollTop + h.scroller.clientHeight >= h.scroller.scrollHeight - 2) h.growAtEnd(true);
		if (!h.vs.inSync(h.scroller.scrollTop)) h.vs.fromNative(h.scroller.scrollTop);
		h.updateExtent();
		h.schedule();
	}

	function onWheel(e: WheelEvent) {
		if (e.ctrlKey) {
			// Ctrl+molette : zoom de la feuille (pas celui de toute la page), par pas de 10 %.
			// Pavé tactile (pincement : petits pas) : un pas tous les 50 px cumulés.
			e.preventDefault();
			h.zoomWheel += e.deltaMode === 0 ? e.deltaY : e.deltaY * 50;
			if (h.onzoom && Math.abs(h.zoomWheel) >= 50) {
				h.onzoom(stepZoom(h.zoom, -Math.sign(h.zoomWheel)));
				h.zoomWheel = 0;
			}
			return;
		}
		h.holdWhileScrolling();
		stopGlide();
		stopFling();
		h.noteTip = null;
		const unit = e.deltaMode === 1 ? ROW_H : e.deltaMode === 2 ? h.vs.view : 1;
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
			const first = h.rax().indexAt(h.vs.top);
			const base = h.rowTop(first) < h.vs.top && steps < 0 ? first + 1 : first;
			dy = h.rowTop(clamp(base + steps, 0, SHEET_ROWS - 1)) - h.vs.top;
		}
		if (dx && notch && e.shiftKey) {
			const steps = Math.sign(dx) * 3;
			const first = h.cax().indexAt(h.scroller.scrollLeft);
			dx = h.cax().offset(clamp(first + steps, 0, SHEET_COLS - 1)) - h.scroller.scrollLeft;
		}
		scrollByPx(dx, dy);
	}

	/** Défile de (dx, dy) px de la feuille (molette, doigt) ; rend le déplacement effectif. */
	function scrollByPx(dx: number, dy: number): { dx: number; dy: number } {
		const t0 = h.vs.top;
		const l0 = h.scroller.scrollLeft;
		if (dy) {
			// Défiler vers le bas agrandit la feuille d'autant.
			if (dy > 0) h.vs.content = Math.min(h.rax().total, Math.max(h.vs.content, h.vs.top + dy + 2 * h.vs.view));
			h.vs.by(dy);
			h.updateExtent();
			h.scroller.scrollTop = h.vs.toNative();
		}
		if (dx) {
			// Vers la droite, la feuille s'agrandit d'autant avant de défiler.
			if (dx > 0 && h.spacer) {
				const want = h.cax().indexAt(h.scroller.scrollLeft + dx + h.viewW) + 2;
				h.spacer.style.width = `${h.MARGIN + h.cax().offset(Math.min(SHEET_COLS, Math.max(h.extentCols, want)))}px`;
			}
			h.scroller.scrollLeft += dx;
			h.updateExtent();
		}
		h.schedule();
		return { dx: h.scroller.scrollLeft - l0, dy: h.vs.top - t0 };
	}

	function syncNative() {
		h.scroller.scrollTop = h.vs.toNative();
		h.schedule();
	}

	// --- Volet ouvert à droite : l'objet cliqué reste à l'écran -------------
	// Un volet (champs du TCD, graphique, assistant, conflits…) prend sa
	// place à droite : la grille rétrécit (resize) et ce qui était au bord
	// passerait dessous. Si l'objet (graphique sélectionné, TCD du volet,
	// sinon la sélection) se voyait avant, on fait défiler vers la droite
	// (le contenu glisse à gauche) pour le montrer en entier s'il tient,
	// sinon son début ou la cellule active (keepinview.ts). Glissé court,
	// abandonné dès que l'utilisateur défile lui-même ; rien à la fermeture.
	let glide: { from: number; to: number; t0: number; last: number; frame: number } | null = null;
	const GLIDE_MS = 220;

	function stopGlide() {
		if (glide) cancelAnimationFrame(glide.frame);
		glide = null;
	}

	/** Défilement horizontal posé à x, la feuille agrandie d'autant s'il le faut. */
	function setScrollX(x: number) {
		if (h.spacer) {
			const want = h.cax().indexAt(x + h.viewW) + 2;
			h.spacer.style.width = `${h.MARGIN + h.cax().offset(Math.min(SHEET_COLS, Math.max(h.extentCols, want)))}px`;
		}
		h.scroller.scrollLeft = x;
		h.updateExtent();
		h.schedule();
	}

	function slideX(to: number) {
		stopGlide();
		if (!h.motion()) {
			setScrollX(to);
			return;
		}
		const g = { from: h.scroller.scrollLeft, to, t0: performance.now(), last: h.scroller.scrollLeft, frame: 0 };
		const step = (now: number) => {
			if (glide !== g) return;
			// Défilé entre-temps (molette, barre, clavier) : on le laisse faire.
			if (Math.abs(h.scroller.scrollLeft - g.last) > 1.5) {
				glide = null;
				return;
			}
			const k = ease(progress(now, g.t0, GLIDE_MS));
			setScrollX(Math.round(g.from + (g.to - g.from) * k));
			g.last = h.scroller.scrollLeft;
			if (k < 1) g.frame = requestAnimationFrame(step);
			else glide = null;
		};
		glide = g;
		g.frame = requestAnimationFrame(step);
	}

	/** Objet à garder à l'écran, et la cellule active à montrer s'il est trop large. */
	function focusSpan(): { o: XSpan; must: XSpan } {
		const ax = h.cax();
		const g = h.boxOf(h.active.row, h.active.col);
		const c0 = g?.col ?? h.active.col;
		const must = { x0: ax.offset(c0), x1: ax.offset(g ? g.col + g.cols : h.active.col + 1) };
		if (h.selChart !== null) {
			const ch = h.charts.find((c) => c.id === h.selChart);
			if (ch) {
				const b = h.chartBox(ch);
				return { o: { x0: b.x, x1: b.x + b.w }, must: { x0: b.x, x1: b.x + Math.min(b.w, 100) } };
			}
		}
		if (h.panel?.kind === 'pivot') {
			const id = h.panel.id;
			const p = h.pivots.find((p) => p.id === id);
			if (p && !p.error) return { o: { x0: ax.offset(p.col), x1: ax.offset(p.col + Math.max(1, p.cells[0]?.length ?? 1)) }, must };
		}
		const s0 = Math.min(h.anchor.col, c0);
		const s1 = Math.max(h.anchor.col + 1, c0 + (g?.cols ?? 1));
		return { o: { x0: ax.offset(s0), x1: ax.offset(s1) }, must };
	}

	/** La grille vient de rétrécir (de oldW à viewW) : l'objet qui se voyait y reste. */
	function keepFocusAfterShrink(oldW: number) {
		if (!h.scroller || h.viewW - h.MARGIN < 80) return;
		const f = focusSpan();
		const fr = h.cax().offset(h.fCols());
		const x = h.scroller.scrollLeft;
		if (!seenIn(f.o, x, oldW - h.MARGIN, fr) && !seenIn(f.must, x, oldW - h.MARGIN, fr)) return;
		const to = keepInView(f.o, x, h.viewW - h.MARGIN, fr, h.cax(), f.must);
		if (Math.abs(to - x) >= 1) slideX(to);
	}

	function ensureVisible(row: number, col: number) {
		// Grille démontée (changement de classeur ou de mode en cours) : rien à montrer.
		if (!h.scroller) return;
		stopGlide();
		h.updateExtent();
		// Sous les volets figés : la case ne doit pas rester cachée dessous.
		// Comme Excel, la case est montrée entière, et la première ligne ou
		// colonne montrée n'est pas coupée (ni contre les volets figés).
		const t = revealWhole(row, h.rax(), h.vs.top, h.vs.view, h.rowTop(h.fRows()));
		if (t !== h.vs.top) h.vs.to(t);
		const l = revealWhole(col, h.cax(), h.scroller.scrollLeft, h.viewW - h.MARGIN, h.cax().offset(h.fCols()));
		if (l !== h.scroller.scrollLeft) h.scroller.scrollLeft = l;
		syncNative();
	}

	return {
		scrollLeftTo,
		stopFling,
		panDown,
		panMove,
		panUp,
		objectPanDown,
		panMenu,
		panReset,
		spaceKey,
		onSpaceUp,
		onPointerDown,
		onPointerMove,
		onPointerUp,
		onScroll,
		onWheel,
		syncNative,
		stopGlide,
		keepFocusAfterShrink,
		ensureVisible
	};
}
