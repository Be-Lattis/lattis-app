<script lang="ts">
	// Carte : points (taille selon une valeur facultative) ou densité, sur un
	// fond OpenStreetMap. Séries : latitude, longitude, valeur. La vue
	// s'ajuste aux points (zoom entier, tuiles de 256 px). Sans tuiles
	// (refus, hors ligne), un fond neutre garde les points à leur place.
	import { untrack } from 'svelte';
	import { PALETTE, tickLabel } from './scale';
	import { fitView, heatColor, heatGrid, project, viewTiles } from './shapes';
	import { loadTile } from './tiles';

	interface Props {
		labels: string[];
		series: { name: string; values: (number | null)[] }[];
		width: number;
		height: number;
		style?: { title?: string; mapMode?: string; colors?: string[]; labels?: boolean };
	}

	let { labels, series, width, height, style = {} }: Props = $props();

	let uid = Math.random().toString(36).slice(2, 8);
	const top = $derived(style.title ? 30 : 0);
	const w = $derived(Math.max(20, width));
	const h = $derived(Math.max(20, height - top));
	const heat = $derived(style.mapMode === 'heat');
	const color = $derived(style.colors?.[0] || PALETTE[0]);

	const pts = $derived.by(() => {
		const lat = series[0]?.values ?? [];
		const lon = series[1]?.values ?? [];
		const val = series[2]?.values;
		const out: { lat: number; lon: number; v: number | null; label: string }[] = [];
		lat.forEach((a, i) => {
			const o = lon[i];
			if (a === null || o === null || o === undefined || Math.abs(a) > 90 || Math.abs(o) > 180) return;
			out.push({ lat: a, lon: o, v: val ? (val[i] ?? null) : null, label: labels[i] ?? '' });
		});
		return out;
	});
	const view = $derived(fitView(pts, w, h, heat ? 32 : 20));
	const tiles = $derived(viewTiles(view, w, h));
	const placed = $derived(
		pts.map((p) => {
			const q = project(p.lat, p.lon, view.z);
			return { ...p, x: q.x - view.x0, y: top + q.y - view.y0 };
		})
	);
	const vmax = $derived(Math.max(0, ...pts.map((p) => Math.abs(p.v ?? 0))));
	const radius = (v: number | null) => (series[2] && vmax > 0 && v !== null ? 3 + 13 * Math.sqrt(Math.abs(v) / vmax) : 5);

	// Densité : grille lissée, pondérée par la valeur (positive) s'il y en a une.
	const CELL = 6;
	const grid = $derived(heat ? heatGrid(placed.map((p) => ({ x: p.x, y: p.y - top, w: series[2] ? Math.max(0, p.v ?? 0) : 1 })), w, h, CELL, Math.max(20, Math.min(w, h) / 8)) : null);
	const cells = $derived.by(() => {
		if (!grid || !grid.max) return [];
		const out: { x: number; y: number; t: number }[] = [];
		for (let r = 0; r < grid.rows; r++)
			for (let c = 0; c < grid.cols; c++) {
				const t = grid.values[r * grid.cols + c] / grid.max;
				if (t > 0.03) out.push({ x: c * CELL, y: top + r * CELL, t });
			}
		return out;
	});

	// Tuiles : chargées une à une ; null = indisponible.
	let images = $state<Record<string, string | null>>({});
	$effect(() => {
		let live = true;
		for (const t of tiles) {
			const k = `${t.z}/${t.x}/${t.y}`;
			if (untrack(() => k in images)) continue;
			loadTile(t.z, t.x, t.y).then((src) => {
				if (live) images = { ...images, [k]: src };
			});
		}
		return () => (live = false);
	});
	const shown = $derived(tiles.filter((t) => images[`${t.z}/${t.x}/${t.y}`]));
	const offline = $derived(tiles.length > 0 && tiles.every((t) => images[`${t.z}/${t.x}/${t.y}`] === null));
</script>

<svg {width} {height} viewBox="0 0 {width} {height}" role="img" aria-label={style.title || 'Carte'}>
	<defs>
		<clipPath id="map-{uid}"><rect x="0" y={top} width={w} height={h} /></clipPath>
		<filter id="blur-{uid}" x="-10%" y="-10%" width="120%" height="120%"><feGaussianBlur stdDeviation={CELL * 0.8} /></filter>
	</defs>
	<rect {width} {height} fill="#fff" />
	{#if style.title}<text x={width / 2} y="21" text-anchor="middle" class="title">{style.title}</text>{/if}
	<g clip-path="url(#map-{uid})">
		<!-- Fond neutre : visible tant que les tuiles manquent. -->
		<rect x="0" y={top} width={w} height={h} fill="#e9edf0" />
		{#each { length: Math.ceil(w / 64) } as _, i (i)}<line x1={i * 64} x2={i * 64} y1={top} y2={top + h} class="grat" />{/each}
		{#each { length: Math.ceil(h / 64) } as _, i (i)}<line x1="0" x2={w} y1={top + i * 64} y2={top + i * 64} class="grat" />{/each}
		{#each shown as t (`${t.z}/${t.x}/${t.y}/${t.left}`)}
			<image href={images[`${t.z}/${t.x}/${t.y}`]} x={t.left} y={top + t.top} width="256" height="256" />
		{/each}
		{#if heat}
			<g filter="url(#blur-{uid})">
				{#each cells as c, i (i)}<rect x={c.x} y={c.y} width={CELL} height={CELL} fill={heatColor(c.t)} fill-opacity={0.4 + 0.5 * c.t} />{/each}
			</g>
		{:else}
			{#each placed as p, i (i)}
				<circle cx={p.x} cy={p.y} r={radius(p.v)} fill={color} fill-opacity="0.75" stroke="#fff" stroke-width="1"><title>{p.label}{p.v !== null ? ` : ${tickLabel(p.v)}` : ''}</title></circle>
				{#if style.labels && p.label}<text x={p.x + radius(p.v) + 3} y={p.y + 4} class="small halo">{p.label}</text>{/if}
			{/each}
		{/if}
	</g>
	{#if heat && cells.length}
		<defs>
			<linearGradient id="ramp-{uid}">
				{#each [0, 0.33, 0.66, 1] as t (t)}<stop offset={t} stop-color={heatColor(t)} />{/each}
			</linearGradient>
		</defs>
		<rect x="8" y={top + h - 22} width="70" height="8" fill="url(#ramp-{uid})" />
		<text x="82" y={top + h - 14} class="small halo">Densité</text>
	{/if}
	{#if shown.length}
		<rect x={w - 98} y={top + h - 16} width="98" height="16" fill="#fff" fill-opacity="0.75" />
		<text x={w - 4} y={top + h - 4} text-anchor="end" class="attr">© OpenStreetMap</text>
	{:else if offline}
		<text x={w - 6} y={top + h - 6} text-anchor="end" class="attr"><title>Le fond de carte vient de tile.openstreetmap.org : autorisez cet hôte (onglet SCRIPT › Accès réseau) ou vérifiez la connexion.</title>Fond de carte indisponible</text>
	{/if}
	{#if !pts.length}
		<text x={w / 2} y={top + h / 2} text-anchor="middle" class="small halo">Aucun point : latitude et longitude attendues</text>
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
		fill: #1f2328;
	}
	.halo {
		paint-order: stroke;
		stroke: #fff;
		stroke-width: 3px;
		stroke-linejoin: round;
	}
	.attr {
		font-size: 10px;
		fill: #374151;
	}
	.grat {
		stroke: #dde2e6;
	}
</style>
