<script lang="ts">
	// Graphique en SVG, sans bibliothèque : barres (verticales ou horizontales,
	// groupées ou empilées, en perspective), courbes, aires, combiné (barres et
	// courbes), secteurs, anneau, nuage de points, cascade, boîte à
	// moustaches, fréquences ; les types sans axes (secteurs 3D,
	// compartimentage, radar, entonnoir, jauge) sont dans ChartFree, la carte
	// dans MapChart. Les données arrivent déjà agrégées par le classeur.
	import { untrack } from 'svelte';
	import { PALETTE, axisFormat, categoryLayout, legendFlow, parseNumber, shortLabel, tickLabel, ticks } from './scale';
	import { BY_CATEGORY, SECONDARY, SINGLE_SERIES, STACKABLE, seriesShape } from './kinds';
	import { boxGroups, histogramBins, shade, waterfallSteps, type BoxStats } from './shapes';
	import ChartFree from './ChartFree.svelte';
	import MapChart from './MapChart.svelte';
	import { ease, lerpSeries, progress, reducedMotion, sameSeries } from '../grid/anim';

	export interface Series {
		name: string;
		values: (number | null)[];
	}

	export interface ChartStyle {
		title?: string;
		legend?: 'bottom' | 'top' | 'right' | 'none' | string;
		xTitle?: string;
		yTitle?: string;
		labels?: boolean;
		noGrid?: boolean;
		stacked?: boolean;
		yMin?: number;
		yMax?: number;
		colors?: string[];
		/** Séries sur l'axe secondaire (à droite, échelle propre), par rang. */
		secondary?: boolean[];
		y2Title?: string;
		/** Graphique combiné : forme de chaque série (bar, line, area). */
		seriesKinds?: string[];
		/** Carte : points (défaut) ou heat (densité). */
		mapMode?: string;
		/** Fréquences : nombre de classes (0 ou absent : automatique). */
		bins?: number;
	}

	interface Props {
		kind: string;
		labels: string[];
		series: Series[];
		width: number;
		height: number;
		style?: ChartStyle;
		/** Données qui changent : barres, courbes et secteurs vont de l'ancienne valeur à la nouvelle. */
		animate?: boolean;
		/** Graphique qui vient de naître d'une modification : il pousse depuis zéro. */
		grow?: boolean;
	}

	let { kind, labels: rawLabels, series: target, width, height, style = {}, animate = false, grow = false }: Props = $props();

	// Séries affichées : celles reçues, ou en chemin vers elles (choix de Louis :
	// le graphique se remplit après une modification, pas après une
	// navigation). L'échelle suit tout de suite les nouvelles valeurs.
	let shownSeries = $state.raw<Series[]>(untrack(() => (animate && grow && !reducedMotion() ? lerpSeries(null, target, 0) : target)));
	$effect(() => {
		const to = target;
		const from = untrack(() => shownSeries);
		if (!animate || reducedMotion() || sameSeries(from, to)) {
			shownSeries = to;
			return;
		}
		const t0 = performance.now();
		let raf = 0;
		const step = (now: number) => {
			const k = progress(now, t0);
			shownSeries = lerpSeries(from, to, ease(k));
			if (k < 1) raf = requestAnimationFrame(step);
		};
		raf = requestAnimationFrame(step);
		return () => cancelAnimationFrame(raf);
	});

	// Données dessinées : fréquences (classes), cascade (total ajouté), boîtes
	// (regroupées par catégorie), une seule série pour les types qui n'en
	// montrent qu'une.
	function prepare(labels: string[], ss: Series[]): { labels: string[]; series: Series[]; boxes: (BoxStats | null)[][] | null } {
		if (kind === 'histogram') {
			const b = histogramBins(
				ss.map((s) => s.values),
				style.bins
			);
			return { labels: b.labels, series: ss.map((s, k) => ({ name: s.name, values: b.counts[k] })), boxes: null };
		}
		if (kind === 'waterfall') return { labels: [...labels, 'Total'], series: ss.slice(0, 1), boxes: null };
		if (kind === 'boxplot') {
			const g = boxGroups(labels, ss);
			return { labels: g.labels, series: ss, boxes: g.boxes };
		}
		return { labels, series: SINGLE_SERIES.has(kind) ? ss.slice(0, 1) : ss, boxes: null };
	}
	const view = $derived(prepare(rawLabels, shownSeries));
	const goal = $derived(prepare(rawLabels, target));
	const labels = $derived(view.labels);
	const series = $derived(view.series);

	const color = (k: number) => style.colors?.[k] || PALETTE[k % PALETTE.length];
	const radial = $derived(kind === 'pie' || kind === 'doughnut');
	const free = $derived(kind === 'pie3d' || kind === 'treemap' || kind === 'radar' || kind === 'funnel' || kind === 'gauge');
	const noAxes = $derived(radial || free);
	const horizontal = $derived(kind === 'column');
	const stacked = $derived(!!style.stacked && STACKABLE.has(kind));
	const legend = $derived(style.legend || 'bottom');
	// Axe secondaire : histogramme, courbes, aires, nuage et combiné (pas les
	// barres horizontales ni les secteurs), dès qu'une série y est placée.
	const isSec = (k: number) => !!style.secondary?.[k] && SECONDARY.has(kind);
	const hasSec = $derived(series.some((_, k) => isSec(k)));
	// Forme de chaque série (combiné : au choix ; sinon celle du graphique).
	const shape = (k: number) => (kind === 'combo' ? seriesShape(style.seriesKinds, k) : kind === 'line' || kind === 'area' ? kind : 'bar');
	const isBar = (k: number) => (kind === 'combo' ? shape(k) === 'bar' : kind === 'bar' || kind === 'column' || kind === 'bar3d' || kind === 'histogram');
	const isLine = (k: number) => kind === 'line' || kind === 'area' || (kind === 'combo' && shape(k) !== 'bar');
	const isArea = (k: number) => shape(k) === 'area';

	const legendItems = $derived.by(() => {
		if (kind === 'gauge') return [];
		if (kind === 'waterfall')
			return [
				{ name: 'Hausse', color: color(0) },
				{ name: 'Baisse', color: color(1) },
				{ name: 'Total', color: color(2) }
			];
		if (BY_CATEGORY.has(kind)) return labels.map((l, i) => ({ name: l, color: color(i) }));
		return series.map((s, k) => ({ name: s.name, color: color(k) }));
	});

	// Perspective de l'histogramme 3D : profondeur des barres (vers la droite et le haut).
	const d3 = $derived(kind === 'bar3d' ? Math.min(16, Math.max(6, ((width - 90) / Math.max(1, labels.length)) * 0.22)) : 0);
	const d3y = $derived(d3 * 0.6);

	// Marges : titre, légende, titres d'axes, étiquettes de catégories
	// (inclinées si elles ne tiennent pas droites).
	const padLeft = $derived((noAxes ? 12 : horizontal ? 80 : 48) + (style.yTitle && !noAxes ? 18 : 0));
	const padRight = $derived(16 + d3 + (legend === 'right' && legendItems.length ? 130 : 0) + (hasSec ? 44 + (style.y2Title ? 18 : 0) : 0));
	const catLayout = $derived(
		noAxes || horizontal || kind === 'scatter'
			? { every: 1, angle: 0, depth: 14, chars: 12 }
			: categoryLayout(labels, Math.max(20, width - padLeft - padRight) / Math.max(1, labels.length))
	);
	// Légende en haut ou en bas : toute la largeur, sur plusieurs rangées au
	// besoin (au plus le tiers de la hauteur), libellés complets.
	const flow = $derived(
		legend === 'top' || legend === 'bottom' ? legendFlow(legendItems.map((it) => it.name), width, Math.max(1, Math.floor(height / 3 / 16))) : { items: [], rows: 0 }
	);
	const legendH = $derived(flow.rows ? 6 + flow.rows * 16 : 0);
	const pad = $derived({
		top: (style.title ? 30 : 12) + (legend === 'top' ? legendH : 0) + d3y,
		right: padRight,
		bottom: (noAxes ? 12 : horizontal ? 26 : 16 + catLayout.depth) + (style.xTitle && !noAxes ? 18 : 0) + (legend === 'bottom' ? legendH : 0),
		left: padLeft
	});
	const plotW = $derived(Math.max(20, width - pad.left - pad.right));
	const plotH = $derived(Math.max(20, height - pad.top - pad.bottom));

	// Échelle des valeurs (empilées : sommes positives et négatives ; cascade :
	// cumuls ; boîtes : extrêmes).
	const extent = $derived.by(() => {
		let lo = Infinity;
		let hi = -Infinity;
		const see = (v: number) => ((lo = Math.min(lo, v)), (hi = Math.max(hi, v)));
		if (kind === 'waterfall') {
			for (const s of waterfallSteps(goal.series[0]?.values ?? [])) see(s.from), see(s.to);
		} else if (kind === 'boxplot') {
			for (const row of goal.boxes ?? []) for (const b of row) if (b) [b.min, b.max, ...b.outliers].forEach(see);
		} else if (stacked) {
			for (let i = 0; i < goal.labels.length; i++) {
				let p = 0;
				let n = 0;
				goal.series.forEach((s, k) => {
					if (isSec(k)) return;
					const v = s.values[i] ?? 0;
					if (v >= 0) p += v;
					else n += v;
				});
				see(p);
				see(n);
			}
		} else {
			goal.series.forEach((s, k) => {
				if (isSec(k)) return;
				for (const v of s.values) if (v !== null) see(v);
			});
		}
		if (lo > hi) lo = hi = 0;
		return [style.yMin ?? lo, style.yMax ?? hi];
	});
	// Axe secondaire : sa propre échelle, sur ses séries seulement.
	const ticks2 = $derived.by(() => {
		let lo = 0;
		let hi = 0;
		goal.series.forEach((s, k) => {
			if (!isSec(k)) return;
			for (const v of s.values) if (v !== null) (lo = Math.min(lo, v)), (hi = Math.max(hi, v));
		});
		return ticks(lo, hi);
	});
	const w0 = $derived(ticks2[0]);
	const w1 = $derived(ticks2[ticks2.length - 1]);
	const fmt2 = $derived(axisFormat(ticks2));
	const vy2 = (v: number) => pad.top + plotH - ((Math.min(Math.max(v, w0), w1) - w0) / (w1 - w0 || 1)) * plotH;
	/** Position verticale d'une valeur de la série k (sur son axe). */
	const vyOf = (k: number) => (isSec(k) ? vy2 : vy);
	// Boîtes à moustaches : l'échelle ne part pas forcément de zéro.
	const vTicks = $derived(ticks(extent[0], extent[1], 5, kind !== 'boxplot'));
	const v0 = $derived(style.yMin ?? vTicks[0]);
	const v1 = $derived(style.yMax ?? vTicks[vTicks.length - 1]);
	const shownTicks = $derived(vTicks.filter((t) => t >= v0 - 1e-9 && t <= v1 + 1e-9));
	const span = $derived(v1 - v0 || 1);
	// Position d'une valeur : verticale (y) ou horizontale (x pour « column »).
	const vy = (v: number) => pad.top + plotH - ((Math.min(Math.max(v, v0), v1) - v0) / span) * plotH;
	const vx = (v: number) => pad.left + ((Math.min(Math.max(v, v0), v1) - v0) / span) * plotW;
	const n = $derived(labels.length);
	const band = $derived((horizontal ? plotH : plotW) / Math.max(1, n));
	const cat = (i: number) => (horizontal ? pad.top + band * (i + 0.5) : pad.left + band * (i + 0.5));
	const every = $derived(horizontal ? Math.max(1, Math.ceil(n / Math.max(1, Math.floor(plotH / 16)))) : catLayout.every);
	const short = shortLabel;
	// Étiquettes de données : chaque valeur pour elle-même ; graduations : un
	// seul format par axe.
	const fmt = (v: number) => tickLabel(v);
	const vFmt = $derived(axisFormat(shownTicks));

	// Barres : position de chaque rectangle (groupé ou empilé). Fréquences :
	// classes jointives. Combiné : seulement les séries en barres.
	const bars = $derived.by(() => {
		const out: { x: number; y: number; w: number; h: number; c: string; v: number }[] = [];
		const idx = series.map((_, k) => k).filter(isBar);
		if (!idx.length) return out;
		const fill = kind === 'histogram' ? 0.98 : 0.7;
		const gap = (1 - fill) / 2;
		const base = Math.max(v0, Math.min(0, v1));
		const base2 = Math.max(w0, Math.min(0, w1));
		for (let i = 0; i < n; i++) {
			let pos = 0;
			let neg = 0;
			idx.forEach((k, j) => {
				const v = series[k].values[i];
				if (v === null || v === undefined) return;
				let a: number;
				let b: number;
				let slot = 0;
				let size = band * fill;
				const y = vyOf(k);
				if (isSec(k) && stacked) {
					// Série de l'axe secondaire dans un histogramme empilé : barre
					// étroite, centrée, posée sur la pile (comme Excel).
					const x = pad.left + band * i + band * 0.35;
					const y0 = y(Math.max(base2, v));
					const y1 = y(Math.min(base2, v));
					out.push({ x, y: y0, w: Math.max(1, band * 0.3), h: Math.max(1, y1 - y0), c: color(k), v });
					return;
				}
				if (stacked) {
					if (v >= 0) (a = pos), (b = pos += v);
					else (a = neg), (b = neg += v);
				} else {
					a = isSec(k) ? base2 : base;
					b = v;
					size = (band * fill) / idx.length;
					slot = j;
				}
				const start = band * i + band * gap + size * slot;
				if (horizontal) {
					const x0 = vx(Math.min(a, b));
					const x1 = vx(Math.max(a, b));
					out.push({ x: x0, y: pad.top + start, w: Math.max(1, x1 - x0), h: Math.max(1, size - 1), c: color(k), v });
				} else {
					const y0 = y(Math.max(a, b));
					const y1 = y(Math.min(a, b));
					out.push({ x: pad.left + start, y: y0, w: Math.max(1, size - (kind === 'histogram' ? 0 : 1)), h: Math.max(1, y1 - y0), c: color(k), v });
				}
			});
		}
		return out;
	});

	// Cascade : chaque marche part du cumul précédent ; total à la fin.
	const steps = $derived(
		kind === 'waterfall'
			? waterfallSteps(series[0]?.values ?? []).map((s, i) => {
					const y0 = vy(Math.max(s.from, s.to));
					const y1 = vy(Math.min(s.from, s.to));
					return { s, x: pad.left + band * (i + 0.15), w: Math.max(1, band * 0.7), y: y0, h: Math.max(1, y1 - y0), c: color(s.kind === 'up' ? 0 : s.kind === 'down' ? 1 : 2) };
				})
			: []
	);

	// Boîtes à moustaches : une par série dans chaque catégorie.
	const boxes = $derived.by(() => {
		const out: { k: number; cx: number; w: number; b: BoxStats }[] = [];
		(view.boxes ?? []).forEach((row, i) => {
			const present = row.map((b, k) => ({ b, k })).filter((e): e is { b: BoxStats; k: number } => !!e.b);
			const size = (band * 0.7) / Math.max(1, present.length);
			present.forEach((e, j) => out.push({ k: e.k, b: e.b, w: Math.max(2, size * 0.8), cx: pad.left + band * i + band * 0.15 + size * (j + 0.5) }));
		});
		return out;
	});

	// Courbes et aires (empilées : cumul). Combiné : seulement les séries en courbes.
	const lines = $derived.by(() => {
		const acc = labels.map(() => 0);
		return series.map((s, k) => {
			if (!isLine(k)) return null;
			const sec = isSec(k);
			const y = vyOf(k);
			const pts = s.values.map((v, i) => {
				if (v === null) return null;
				// L'axe secondaire n'entre pas dans la pile.
				const lo = stacked && !sec ? acc[i] : sec ? Math.max(w0, Math.min(0, w1)) : Math.max(v0, Math.min(0, v1));
				const hi = stacked && !sec ? (acc[i] += v) : v;
				return { x: cat(i), y: y(hi), y0: y(lo), v };
			});
			return { pts, c: color(k), area: isArea(k) };
		});
	});
	const path = (pts: ({ x: number; y: number } | null)[]) =>
		pts
			.filter((p): p is { x: number; y: number } => !!p)
			.map((p, i) => `${i ? 'L' : 'M'} ${p.x} ${p.y}`)
			.join(' ');
	const areaPath = (pts: ({ x: number; y: number; y0: number } | null)[]) => {
		const p = pts.filter((q): q is { x: number; y: number; y0: number } => !!q);
		if (!p.length) return '';
		return `M ${p[0].x} ${p[0].y0} ` + p.map((q) => `L ${q.x} ${q.y}`).join(' ') + ` L ${p[p.length - 1].x} ${p[p.length - 1].y0} ` + [...p].reverse().map((q) => `L ${q.x} ${q.y0}`).join(' ') + ' Z';
	};

	// Nuage : X numérique tiré des libellés.
	const xs = $derived(labels.map((l, i) => parseNumber(l) ?? i));
	const xTicks = $derived(ticks(Math.min(...xs, 0), Math.max(...xs, 1)));
	const xFmt = $derived(axisFormat(xTicks));
	const sx = (v: number) => pad.left + ((v - xTicks[0]) / (xTicks[xTicks.length - 1] - xTicks[0] || 1)) * plotW;

	// Secteurs et anneau (première série).
	const pieTotal = $derived((series[0]?.values ?? []).reduce<number>((a, v) => a + Math.max(0, v ?? 0), 0));
	const slices = $derived.by(() => {
		let a = -Math.PI / 2;
		return (series[0]?.values ?? []).map((v, i) => {
			const part = pieTotal ? Math.max(0, v ?? 0) / pieTotal : 0;
			const s = { from: a, to: a + part * 2 * Math.PI, part, color: color(i) };
			a = s.to;
			return s;
		});
	});
	const pieR = $derived(Math.max(10, Math.min(plotH, plotW) / 2));
	const pieC = $derived({ x: pad.left + plotW / 2, y: pad.top + plotH / 2 });
	const hole = $derived(kind === 'doughnut' ? pieR * 0.55 : 0);
	function arc(from: number, to: number) {
		const { x, y } = pieC;
		const R = pieR;
		const r = hole;
		const full = to - from >= 2 * Math.PI - 1e-9;
		if (full) to = from + 2 * Math.PI - 1e-4;
		const large = to - from > Math.PI ? 1 : 0;
		const p = (rad: number, a: number) => `${x + rad * Math.cos(a)} ${y + rad * Math.sin(a)}`;
		if (!r) return `M ${x} ${y} L ${p(R, from)} A ${R} ${R} 0 ${large} 1 ${p(R, to)} Z`;
		return `M ${p(R, from)} A ${R} ${R} 0 ${large} 1 ${p(R, to)} L ${p(r, to)} A ${r} ${r} 0 ${large} 0 ${p(r, from)} Z`;
	}
	const pct = new Intl.NumberFormat('fr-FR', { style: 'percent', maximumFractionDigits: 1 });
</script>

{#if kind === 'map'}
	<MapChart labels={rawLabels} series={target} {width} {height} {style} />
{:else}
	<svg {width} {height} viewBox="0 0 {width} {height}" role="img" aria-label={style.title || 'Graphique'}>
		<rect {width} {height} fill="#fff" />
		{#if style.title}<text x={width / 2} y="21" text-anchor="middle" class="title">{style.title}</text>{/if}

		{#if radial}
			{#each slices as s, i (i)}
				{#if s.part > 0}
					<path d={arc(s.from, s.to)} fill={s.color} stroke="#fff" stroke-width="1" />
					{#if style.labels && s.part > 0.04}
						{@const mid = (s.from + s.to) / 2}
						{@const rr = hole ? (pieR + hole) / 2 : pieR * 0.65}
						<text x={pieC.x + rr * Math.cos(mid)} y={pieC.y + rr * Math.sin(mid) + 4} text-anchor="middle" class="on-color">{pct.format(s.part)}</text>
					{/if}
				{/if}
			{/each}
		{:else if free}
			<ChartFree {kind} {labels} {series} x={pad.left} y={pad.top} w={plotW} h={plotH} {color} {style} />
		{:else}
			{#if d3}
				<!-- Perspective : fond et plancher décalés en profondeur. -->
				{@const base = vy(Math.max(v0, Math.min(0, v1)))}
				<path d="M {pad.left} {base} l {d3} {-d3y} h {plotW} l {-d3} {d3y} Z" fill="#f3f4f6" stroke="#d1d5db" />
				{#each shownTicks as t (t)}
					{#if !style.noGrid}<path d="M {pad.left} {vy(t)} l {d3} {-d3y} h {plotW}" fill="none" class="grid" />{/if}
				{/each}
			{/if}
			<!-- Axe des valeurs et quadrillage. -->
			{#each shownTicks as t (t)}
				{#if horizontal}
					{#if !style.noGrid || t === 0}<line x1={vx(t)} x2={vx(t)} y1={pad.top} y2={pad.top + plotH} class={t === 0 ? 'axis' : 'grid'} />{/if}
					<text x={vx(t)} y={pad.top + plotH + 14} text-anchor="middle" class="small">{vFmt(t)}</text>
				{:else}
					{#if (!style.noGrid && !d3) || t === 0}<line x1={pad.left} x2={pad.left + plotW} y1={vy(t)} y2={vy(t)} class={t === 0 ? 'axis' : 'grid'} />{/if}
					<text x={pad.left - 6} y={vy(t) + 4} text-anchor="end" class="small">{vFmt(t)}</text>
				{/if}
			{/each}
			<!-- Axe des catégories. -->
			{#if kind === 'scatter'}
				{#each xTicks as t (t)}
					<text x={sx(t)} y={pad.top + plotH + 14} text-anchor="middle" class="small">{xFmt(t)}</text>
				{/each}
			{:else}
				{#each labels as l, i (i)}
					{#if i % every === 0}
						{#if horizontal}
							<text x={pad.left - 6} y={cat(i) + 4} text-anchor="end" class="small">{short(l)}</text>
						{:else if catLayout.angle}
							{@const y = pad.top + plotH + 10}
							<text x={cat(i)} {y} text-anchor="end" class="small" transform="rotate({catLayout.angle} {cat(i)} {y})"><title>{l}</title>{short(l, catLayout.chars)}</text>
						{:else}
							<text x={cat(i)} y={pad.top + plotH + 14} text-anchor="middle" class="small">{short(l)}</text>
						{/if}
					{/if}
				{/each}
			{/if}

			{#if kind === 'scatter'}
				{#each series as s, k (k)}
					{#each s.values as v, i (i)}
						{#if v !== null}<circle cx={sx(xs[i])} cy={vyOf(k)(v)} r="3.5" fill={color(k)} />{/if}
					{/each}
				{/each}
			{:else if kind === 'waterfall'}
				{#each steps as b, i (i)}
					<rect x={b.x} y={b.y} width={b.w} height={b.h} fill={b.c}><title>{labels[i]} : {fmt(b.s.value)}</title></rect>
					{#if i < steps.length - 1}
						{@const yy = vy(b.s.to)}
						<line x1={b.x + b.w} x2={steps[i + 1].x} y1={yy} y2={yy} class="link" />
					{/if}
					{#if style.labels}<text x={b.x + b.w / 2} y={b.y - 3} text-anchor="middle" class="small">{fmt(b.s.value)}</text>{/if}
				{/each}
			{:else if kind === 'boxplot'}
				{#each boxes as b, i (i)}
					{@const c = color(b.k)}
					<line x1={b.cx} x2={b.cx} y1={vy(b.b.max)} y2={vy(b.b.q3)} stroke="#4b5563" />
					<line x1={b.cx} x2={b.cx} y1={vy(b.b.q1)} y2={vy(b.b.min)} stroke="#4b5563" />
					<line x1={b.cx - b.w / 4} x2={b.cx + b.w / 4} y1={vy(b.b.max)} y2={vy(b.b.max)} stroke="#4b5563" />
					<line x1={b.cx - b.w / 4} x2={b.cx + b.w / 4} y1={vy(b.b.min)} y2={vy(b.b.min)} stroke="#4b5563" />
					<rect x={b.cx - b.w / 2} y={vy(b.b.q3)} width={b.w} height={Math.max(1, vy(b.b.q1) - vy(b.b.q3))} fill={shade(c, 0.35)} stroke={c}>
						<title>{series[b.k]?.name} — min {fmt(b.b.min)}, Q1 {fmt(b.b.q1)}, médiane {fmt(b.b.median)}, Q3 {fmt(b.b.q3)}, max {fmt(b.b.max)}, moyenne {fmt(b.b.mean)} ({b.b.n} valeurs)</title>
					</rect>
					<line x1={b.cx - b.w / 2} x2={b.cx + b.w / 2} y1={vy(b.b.median)} y2={vy(b.b.median)} stroke={shade(c, -0.4)} stroke-width="2" />
					<path d="M {b.cx - 3} {vy(b.b.mean) - 3} l 6 6 m 0 -6 l -6 6" stroke={shade(c, -0.4)} />
					{#each b.b.outliers as o, j (j)}<circle cx={b.cx} cy={vy(o)} r="2.5" fill="none" stroke={c} />{/each}
					{#if style.labels}<text x={b.cx + b.w / 2 + 3} y={vy(b.b.median) + 4} class="small">{fmt(b.b.median)}</text>{/if}
				{/each}
			{:else}
				{#each bars as b, i (i)}
					{#if d3}
						<path d="M {b.x} {b.y} l {d3} {-d3y} h {b.w} l {-d3} {d3y} Z" fill={shade(b.c, 0.3)} />
						<path d="M {b.x + b.w} {b.y} l {d3} {-d3y} v {b.h} l {-d3} {d3y} Z" fill={shade(b.c, -0.3)} />
					{/if}
					<rect x={b.x} y={b.y} width={b.w} height={b.h} fill={b.c} stroke={kind === 'histogram' ? '#fff' : 'none'} stroke-width="0.5" />
					{#if style.labels}
						{#if horizontal}
							<text x={b.x + b.w + 3} y={b.y + b.h / 2 + 4} class="small">{fmt(b.v)}</text>
						{:else}
							<text x={b.x + b.w / 2 + d3 / 2} y={b.y - 3 - d3y} text-anchor="middle" class="small">{fmt(b.v)}</text>
						{/if}
					{/if}
				{/each}
				{#each lines as l, k (k)}
					{#if l}
						{#if l.area}<path d={areaPath(l.pts)} fill={l.c} fill-opacity="0.35" stroke="none" />{/if}
						<path d={path(l.pts)} fill="none" stroke={l.c} stroke-width="2" />
						{#each l.pts as p, i (i)}
							{#if p}
								{#if !l.area}<circle cx={p.x} cy={p.y} r="2.5" fill={l.c} />{/if}
								{#if style.labels}<text x={p.x} y={p.y - 6} text-anchor="middle" class="small">{fmt(p.v)}</text>{/if}
							{/if}
						{/each}
					{/if}
				{/each}
			{/if}

			{#if style.xTitle}<text x={pad.left + plotW / 2} y={pad.top + plotH + (horizontal ? 14 : catLayout.depth) + 18} text-anchor="middle" class="axis-title">{style.xTitle}</text>{/if}
			{#if style.yTitle}
				<text x="14" y={pad.top + plotH / 2} text-anchor="middle" class="axis-title" transform="rotate(-90 14 {pad.top + plotH / 2})">{style.yTitle}</text>
			{/if}
			{#if hasSec}
				<!-- Axe secondaire, à droite. -->
				<line x1={pad.left + plotW} x2={pad.left + plotW} y1={pad.top} y2={pad.top + plotH} class="axis" />
				{#each ticks2 as t (t)}
					<text x={pad.left + plotW + 6} y={vy2(t) + 4} class="small">{fmt2(t)}</text>
				{/each}
				{#if style.y2Title}
					{@const tx = pad.left + plotW + 48}
					<text x={tx} y={pad.top + plotH / 2} text-anchor="middle" class="axis-title" transform="rotate(90 {tx} {pad.top + plotH / 2})">{style.y2Title}</text>
				{/if}
			{/if}
		{/if}

		<!-- Légende. -->
		{#if legend !== 'none'}
			{#if legend === 'right'}
				{#each legendItems.slice(0, 14) as it, i (i)}
					<rect x={width - 136} y={pad.top + i * 18} width="10" height="10" fill={it.color} />
					<text x={width - 122} y={pad.top + i * 18 + 9} class="small">{short(it.name, 18)}{radial && slices[i] ? ` (${pct.format(slices[i].part)})` : ''}</text>
				{/each}
			{:else}
				{@const y0 = legend === 'top' ? (style.title ? 34 : 12) : height - legendH + 10}
				{#each flow.items as f, i (i)}
					{@const y = y0 + f.row * 16}
					<rect x={f.x} y={y - 8} width="10" height="10" fill={legendItems[i].color} />
					<text x={f.x + 14} y={y + 1} class="small"><title>{legendItems[i].name}</title>{f.text}</text>
				{/each}
			{/if}
		{/if}
	</svg>
{/if}

<style>
	svg {
		display: block;
		font-family: 'Segoe UI', system-ui, sans-serif;
	}
	.title {
		font-size: 14px;
		fill: #1f2328;
	}
	.axis-title {
		font-size: 12px;
		fill: #374151;
	}
	.small {
		font-size: 11px;
		fill: #4b5563;
	}
	.on-color {
		font-size: 11px;
		fill: #fff;
		font-weight: 600;
	}
	.grid {
		stroke: #e5e7eb;
	}
	.axis {
		stroke: #9aa0a6;
	}
	.link {
		stroke: #9aa0a6;
		stroke-dasharray: 3 2;
	}
</style>
