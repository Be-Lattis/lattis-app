<script lang="ts">
	// Barre du plan (Données › Grouper), comme un tableur : à gauche de la
	// grille pour les lignes groupées, au-dessus pour les colonnes. Chaque
	// groupe ouvert a son trait le long de ses lignes et son bouton [−] sur
	// sa ligne de synthèse ; un groupe réduit, son bouton [+]. Les boutons
	// 1 2 3 du coin affichent le plan jusqu'à ce niveau. Dessin sur deux
	// canevas, refait par la grille à chaque image (paint).
	import { t } from '#lib/i18n/index.js';
	import { LEVEL_W, bandSize, levelCenter, outlineDepth, outlineGroups, outlineHit, summaryLine, type OutlineGeom, type OutlineGroup, type SheetOutline } from './outline';

	interface Props {
		outline: SheetOutline | null;
		geom: () => OutlineGeom | null;
		/** Réduire (fold) ou rouvrir le groupe de niveau level qui contient la ligne line. */
		onfold: (rows: boolean, level: number, line: number, fold: boolean) => void;
		/** Afficher le plan jusqu'au niveau level. */
		onlevel: (rows: boolean, level: number) => void;
	}
	let { outline, geom, onfold, onlevel }: Props = $props();

	const rowDepth = $derived(outlineDepth(outline?.rows));
	const colDepth = $derived(outlineDepth(outline?.cols));
	const rowGroups = $derived(outlineGroups(outline?.rows, !!outline?.rowsAbove));
	const colGroups = $derived(outlineGroups(outline?.cols, !!outline?.colsLeft));
	/** Largeur de la barre des lignes, hauteur de celle des colonnes (pixels). */
	const W = $derived(bandSize(rowDepth));
	const H = $derived(bandSize(colDepth));

	let left = $state<HTMLCanvasElement>();
	let topCv = $state<HTMLCanvasElement>();

	const C = { bg: '#f5f6f7', line: '#5f6368', box: '#ffffff', border: '#8a8f96', text: '#3c4043' };

	/** Prépare un canevas à la taille de son élément ; rend son contexte. */
	function prep(cv: HTMLCanvasElement | undefined): { g: CanvasRenderingContext2D; w: number; h: number } | null {
		if (!cv) return null;
		const w = cv.clientWidth;
		const h = cv.clientHeight;
		const dpr = window.devicePixelRatio || 1;
		if (cv.width !== Math.round(w * dpr) || cv.height !== Math.round(h * dpr)) {
			cv.width = Math.round(w * dpr);
			cv.height = Math.round(h * dpr);
		}
		const g = cv.getContext('2d');
		if (!g) return null;
		g.setTransform(dpr, 0, 0, dpr, 0, 0);
		g.fillStyle = C.bg;
		g.fillRect(0, 0, w, h);
		return { g, w, h };
	}

	/** Bouton carré centré en (x, y) : un chiffre, ou − / +. */
	function button(g: CanvasRenderingContext2D, x: number, y: number, label: string) {
		const s = 11;
		g.fillStyle = C.box;
		g.strokeStyle = C.border;
		g.lineWidth = 1;
		g.fillRect(Math.round(x - s / 2) + 0.5, Math.round(y - s / 2) + 0.5, s, s);
		g.strokeRect(Math.round(x - s / 2) + 0.5, Math.round(y - s / 2) + 0.5, s, s);
		g.fillStyle = C.text;
		if (label === '−' || label === '+') {
			const cx = Math.round(x) + 0.5;
			const cy = Math.round(y) + 0.5;
			g.beginPath();
			g.strokeStyle = C.text;
			g.moveTo(cx - 3, cy);
			g.lineTo(cx + 3, cy);
			if (label === '+') {
				g.moveTo(cx, cy - 3);
				g.lineTo(cx, cy + 3);
			}
			g.stroke();
			return;
		}
		g.font = '600 9px "Segoe UI", system-ui, sans-serif';
		g.textAlign = 'center';
		g.textBaseline = 'middle';
		g.fillText(label, x, y + 0.5);
	}

	/**
	 * Une barre : rows, lignes (le long de y) ou colonnes (le long de x).
	 * off : décalage de la grille dans la barre ; head : fin de l'en-tête
	 * (boutons de niveau) ; at : place d'une ligne (pixels de la barre).
	 */
	function band(g: CanvasRenderingContext2D, len: number, depth: number, groups: OutlineGroup[], before: boolean, head: number, at: (i: number) => [number, number] | null, rows: boolean) {
		// across, along → x, y
		const P = (across: number, along: number): [number, number] => (rows ? [across, along] : [along, across]);
		for (let k = 1; k <= depth + 1; k++) {
			const [x, y] = P(levelCenter(k), head / 2);
			button(g, x, y, String(k));
		}
		g.save();
		g.beginPath();
		const [cx, cy] = P(0, head);
		const [cw, ch] = P(LEVEL_W * (depth + 1) + 4, Math.max(0, len - head));
		g.rect(cx, cy, cw, ch);
		g.clip();
		g.strokeStyle = C.line;
		g.lineWidth = 1;
		for (const gr of groups) {
			const c = Math.round(levelCenter(gr.level)) + 0.5;
			const s = summaryLine(gr, before);
			const sp = s >= 0 ? at(s) : null;
			if (!gr.folded) {
				const a = at(gr.from);
				const b = at(gr.to);
				// Lignes visibles du groupe : du début de la première à la fin de la dernière.
				let p0 = a ? a[0] : null;
				let p1 = b ? b[0] + b[1] : null;
				if (p0 === null || p1 === null) {
					for (let i = gr.from; i <= gr.to && (p0 === null || p1 === null); i++) {
						const q = at(i);
						if (q && p0 === null) p0 = q[0];
					}
					for (let i = gr.to; i >= gr.from && p1 === null; i--) {
						const q = at(i);
						if (q) p1 = q[0] + q[1];
					}
				}
				if (p0 !== null && p1 !== null && p1 > head && p0 < len) {
					p0 = Math.round(p0) + 0.5;
					p1 = Math.round(p1) - 0.5;
					g.beginPath();
					// Trait le long du groupe, crochet du côté opposé à la synthèse.
					const [x0, y0] = P(c, p0 + 2);
					const [x1, y1] = P(c, p1 - 2);
					g.moveTo(x0, y0);
					g.lineTo(x1, y1);
					const [tx, ty] = P(c - 4, before ? p1 - 2 : p0 + 2);
					const [ux, uy] = P(c, before ? p1 - 2 : p0 + 2);
					g.moveTo(tx, ty);
					g.lineTo(ux, uy);
					g.stroke();
				}
			}
			if (sp) {
				const [x, y] = P(c, sp[0] + sp[1] / 2);
				button(g, x, y, gr.folded ? '+' : '−');
			}
		}
		g.restore();
	}

	/** Redessine les deux barres (appelé par la grille après son dessin). */
	export function paint() {
		const geo = geom();
		if (!geo || !outline) return;
		const z = geo.zoom;
		if (W > 0) {
			const p = prep(left);
			if (p) {
				const at = (r: number): [number, number] | null => {
					const q = geo.row(r);
					return q && q[1] > 0 ? [H + q[0] * z, q[1] * z] : null;
				};
				// Boutons de niveau en face de la ligne des lettres de colonnes.
				band(p.g, p.h, rowDepth, rowGroups, !!outline.rowsAbove, H + geo.top * z, at, true);
			}
		}
		if (H > 0) {
			const p = prep(topCv);
			if (p) {
				const at = (c: number): [number, number] | null => {
					const q = geo.col(c);
					return q && q[1] > 0 ? [q[0] * z, q[1] * z] : null;
				};
				band(p.g, p.w, colDepth, colGroups, !!outline.colsLeft, geo.margin * z, at, false);
			}
		}
	}

	function click(e: MouseEvent, rows: boolean) {
		const geo = geom();
		if (!geo || !outline) return;
		const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
		const x = e.clientX - r.left;
		const y = e.clientY - r.top;
		const z = geo.zoom;
		const hit = rows
			? outlineHit(rowGroups, rowDepth, !!outline.rowsAbove, y, x, H + geo.top * z, (i) => {
					const q = geo.row(i);
					return q && q[1] > 0 ? [H + q[0] * z, q[1] * z] : null;
				})
			: outlineHit(colGroups, colDepth, !!outline.colsLeft, x, y, geo.margin * z, (i) => {
					const q = geo.col(i);
					return q && q[1] > 0 ? [q[0] * z, q[1] * z] : null;
				});
		if (!hit) return;
		e.preventDefault();
		if (hit.kind === 'level') onlevel(rows, hit.level);
		else onfold(rows, hit.group.level, hit.group.from, !hit.group.folded);
	}

	$effect(() => {
		void outline;
		void W;
		void H;
		paint();
	});
</script>

{#if W > 0}
	<canvas class="ol-rows" bind:this={left} style:width="{W}px" aria-label={t('grid.outline.rows')} onmousedown={(e) => click(e, true)}></canvas>
{/if}
{#if H > 0}
	<canvas class="ol-cols" bind:this={topCv} style:height="{H}px" style:left="{W}px" style:width="calc(100% - {W}px)" aria-label={t('grid.outline.cols')} onmousedown={(e) => click(e, false)}></canvas>
{/if}

<style>
	canvas {
		position: absolute;
		cursor: default;
	}
	.ol-rows {
		left: 0;
		top: 0;
		bottom: 0;
		height: 100%;
		border-right: 1px solid #d4d7db;
		box-sizing: border-box;
	}
	.ol-cols {
		top: 0;
		border-bottom: 1px solid #d4d7db;
		box-sizing: border-box;
	}
</style>
