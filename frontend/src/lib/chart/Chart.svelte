<script lang="ts">
	// Graphique en SVG (barres, courbes, secteurs, nuage de points), sans
	// bibliothèque : les données arrivent déjà agrégées par le classeur.
	import { PALETTE, parseNumber, tickLabel, ticks } from './scale';

	export interface Series {
		name: string;
		values: (number | null)[];
	}

	interface Props {
		kind: 'bar' | 'line' | 'pie' | 'scatter';
		title?: string;
		labels: string[];
		series: Series[];
		width: number;
		height: number;
	}

	let { kind, title = '', labels, series, width, height }: Props = $props();

	const PAD = { top: 34, right: 16, bottom: 46, left: 52 };
	const plotW = $derived(width - PAD.left - PAD.right);
	const plotH = $derived(height - PAD.top - PAD.bottom);
	const all = $derived(series.flatMap((s) => s.values.filter((v): v is number => v !== null)));
	const yTicks = $derived(ticks(Math.min(0, ...all), Math.max(0, ...all)));
	const y0 = $derived(yTicks[0]);
	const y1 = $derived(yTicks[yTicks.length - 1]);
	const yAt = (v: number) => PAD.top + plotH - ((v - y0) / (y1 - y0 || 1)) * plotH;
	const n = $derived(labels.length);
	const band = $derived(plotW / Math.max(1, n));
	const xAt = (i: number) => PAD.left + band * (i + 0.5);
	// Nuage : X numérique tiré des libellés.
	const xs = $derived(labels.map((l, i) => parseNumber(l) ?? i));
	const xTicks = $derived(ticks(Math.min(...xs, 0), Math.max(...xs, 1)));
	const sxAt = (v: number) => PAD.left + ((v - xTicks[0]) / (xTicks[xTicks.length - 1] - xTicks[0] || 1)) * plotW;
	// Une étiquette sur k, pour qu'elles ne se chevauchent pas.
	const every = $derived(Math.max(1, Math.ceil(n / Math.max(1, Math.floor(plotW / 60)))));
	const short = (s: string) => (s.length > 12 ? s.slice(0, 11) + '…' : s);

	const pieTotal = $derived((series[0]?.values ?? []).reduce<number>((a, v) => a + Math.max(0, v ?? 0), 0));
	const slices = $derived.by(() => {
		let a = -Math.PI / 2;
		return (series[0]?.values ?? []).map((v, i) => {
			const part = pieTotal ? Math.max(0, v ?? 0) / pieTotal : 0;
			const s = { from: a, to: a + part * 2 * Math.PI, part, label: labels[i] ?? '', color: PALETTE[i % PALETTE.length] };
			a = s.to;
			return s;
		});
	});
	const pieR = $derived(Math.max(10, Math.min(plotH, width * 0.5) / 2));
	const pieC = $derived({ x: PAD.left + pieR, y: PAD.top + plotH / 2 });
	function arc(from: number, to: number) {
		const { x, y } = pieC;
		if (to - from >= 2 * Math.PI - 1e-9) return `M ${x - pieR} ${y} a ${pieR} ${pieR} 0 1 0 ${2 * pieR} 0 a ${pieR} ${pieR} 0 1 0 ${-2 * pieR} 0`;
		const large = to - from > Math.PI ? 1 : 0;
		return `M ${x} ${y} L ${x + pieR * Math.cos(from)} ${y + pieR * Math.sin(from)} A ${pieR} ${pieR} 0 ${large} 1 ${x + pieR * Math.cos(to)} ${y + pieR * Math.sin(to)} Z`;
	}
	const pct = new Intl.NumberFormat('fr-FR', { style: 'percent', maximumFractionDigits: 1 });
</script>

<svg {width} {height} viewBox="0 0 {width} {height}" role="img" aria-label={title || 'Graphique'}>
	<rect {width} {height} fill="#fff" />
	{#if title}<text x={width / 2} y="20" text-anchor="middle" class="title">{title}</text>{/if}
	{#if kind === 'pie'}
		{#each slices as s, i (i)}
			{#if s.part > 0}<path d={arc(s.from, s.to)} fill={s.color} stroke="#fff" stroke-width="1" />{/if}
		{/each}
		{#each slices.slice(0, 12) as s, i (i)}
			<rect x={pieC.x + pieR + 20} y={PAD.top + i * 18} width="10" height="10" fill={s.color} />
			<text x={pieC.x + pieR + 36} y={PAD.top + i * 18 + 9} class="small">{short(s.label)} ({pct.format(s.part)})</text>
		{/each}
	{:else}
		{#each yTicks as t (t)}
			<line x1={PAD.left} x2={PAD.left + plotW} y1={yAt(t)} y2={yAt(t)} class={t === 0 ? 'axis' : 'grid'} />
			<text x={PAD.left - 6} y={yAt(t) + 4} text-anchor="end" class="small">{tickLabel(t)}</text>
		{/each}
		{#if kind === 'scatter'}
			{#each xTicks as t (t)}
				<text x={sxAt(t)} y={PAD.top + plotH + 16} text-anchor="middle" class="small">{tickLabel(t)}</text>
			{/each}
			{#each series as s, k (k)}
				{#each s.values as v, i (i)}
					{#if v !== null}<circle cx={sxAt(xs[i])} cy={yAt(v)} r="3.5" fill={PALETTE[k % PALETTE.length]} />{/if}
				{/each}
			{/each}
		{:else}
			{#each labels as l, i (i)}
				{#if i % every === 0}<text x={xAt(i)} y={PAD.top + plotH + 16} text-anchor="middle" class="small">{short(l)}</text>{/if}
			{/each}
			{#if kind === 'bar'}
				{#each series as s, k (k)}
					{#each s.values as v, i (i)}
						{#if v !== null}
							{@const bw = (band * 0.7) / series.length}
							<rect
								x={PAD.left + band * i + band * 0.15 + bw * k}
								y={Math.min(yAt(v), yAt(0))}
								width={Math.max(1, bw - 1)}
								height={Math.abs(yAt(v) - yAt(0))}
								fill={PALETTE[k % PALETTE.length]}
							/>
						{/if}
					{/each}
				{/each}
			{:else}
				{#each series as s, k (k)}
					<polyline
						points={s.values.map((v, i) => (v === null ? '' : `${xAt(i)},${yAt(v)}`)).filter(Boolean).join(' ')}
						fill="none"
						stroke={PALETTE[k % PALETTE.length]}
						stroke-width="2"
					/>
				{/each}
			{/if}
		{/if}
		{#each series as s, k (k)}
			<rect x={PAD.left + k * 120} y={height - 16} width="10" height="10" fill={PALETTE[k % PALETTE.length]} />
			<text x={PAD.left + k * 120 + 14} y={height - 7} class="small">{short(s.name)}</text>
		{/each}
	{/if}
</svg>

<style>
	svg {
		display: block;
		font-family: 'Segoe UI', system-ui, sans-serif;
	}
	.title {
		font-size: 14px;
		fill: #1f2328;
	}
	.small {
		font-size: 11px;
		fill: #4b5563;
	}
	.grid {
		stroke: #e5e7eb;
	}
	.axis {
		stroke: #9aa0a6;
	}
</style>
