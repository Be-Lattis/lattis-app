<script lang="ts">
	// Carte : points (taille selon une valeur facultative) ou densité, sur un
	// fond OpenStreetMap. Séries : latitude, longitude, valeur. La vue
	// s'ajuste aux points (zoom entier, tuiles de 256 px), ou suit celle que
	// l'utilisateur a choisie (style.mapView : centre et zoom fractionnaire).
	// Sans tuiles (refus, hors ligne), un fond neutre garde les points à leur
	// place. Tant que ses coordonnées ne sont pas choisies (mapDraft), la
	// carte n'est qu'un carré grisé : ni tuiles ni points.
	//
	// Zoom et déplacement (choix : la molette fait défiler la feuille ou le
	// document tant que la carte n'est pas sélectionnée) :
	// - carte sélectionnée (interactive) : la molette zoome autour du
	//   curseur, glisser déplace la carte (Alt+glisser laisse l'hôte déplacer
	//   le graphique), double-clic zoome, + et − au clavier ;
	// - boutons + / − / recentrer en haut à droite, au survol ou à la sélection ;
	// - la vue est enregistrée (onview, annulable) au relâchement, ou un
	//   instant après le dernier cran de molette ; « recentrer » revient à la
	//   vue ajustée aux points.
	import { onDestroy, untrack } from 'svelte';
	import { PALETTE, tickLabel } from './scale';
	import { fitView, heatColor, heatGrid, project, viewTiles, type MapView } from './shapes';
	import { ZOOM_STEP, camView, fallbackTiles, heatRadius, panBy, parseCam, roundCam, viewCam, wheelZoom, zoomAt, MAX_ZOOM, MIN_ZOOM, type MapCam } from './mapview';
	import { loadTile, tileNow } from './tiles';

	interface Props {
		labels: string[];
		series: { name: string; values: (number | null)[] }[];
		width: number;
		height: number;
		style?: { title?: string; mapMode?: string; colors?: string[]; labels?: boolean; mapDraft?: boolean; mapView?: MapCam | null };
		/** Carte sélectionnée : molette, glisser, double-clic et clavier la zooment ou la déplacent. */
		interactive?: boolean;
		/** Vue choisie (null : recentrée sur les points), à enregistrer dans la configuration. */
		onview?: (v: MapCam | null) => void;
	}

	let { labels, series, width, height, style = {}, interactive = false, onview }: Props = $props();

	let uid = Math.random().toString(36).slice(2, 8);
	const top = $derived(style.title ? 30 : 0);
	const w = $derived(Math.max(20, width));
	const h = $derived(Math.max(20, height - top));
	const heat = $derived(style.mapMode === 'heat');
	const color = $derived(style.colors?.[0] || PALETTE[0]);

	const draft = $derived(!!style.mapDraft);
	const pts = $derived.by(() => {
		if (draft) return [];
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
	// Vue ajustée aux points, et vue choisie (null : on suit l'ajustement).
	const fit = $derived(fitView(pts, w, h, heat ? 32 : 20));
	const saved = $derived(parseCam(style.mapView));
	const savedKey = $derived(saved ? `${saved.lat},${saved.lon},${saved.zoom}` : '');
	let cam = $state<MapCam | null>(null);
	// Geste en cours ou vue pas encore enregistrée : la vue reçue (rechargement
	// du graphique) ne l'écrase pas.
	let dirty = false;
	let dragging = $state(false);
	$effect(() => {
		void savedKey;
		untrack(() => {
			if (!dirty && !dragging) cam = saved;
		});
	});
	const view = $derived<MapView>(cam ? camView(cam, w, h) : fit);
	const tiles = $derived(draft ? [] : viewTiles(view, w, h));

	// Tuiles demandées : celles de la vue posée (pas à chaque cran de
	// molette ni à chaque pixel de déplacement).
	let settled = $state.raw<MapView>(untrack(() => view));
	$effect(() => {
		const v = view;
		if (v.z === settled.z && v.x0 === settled.x0 && v.y0 === settled.y0) return;
		const t = setTimeout(() => (settled = v), 160);
		return () => clearTimeout(t);
	});
	const wantedTiles = $derived(draft ? [] : viewTiles(settled, w, h));

	const placed = $derived(
		pts.map((p) => {
			const q = project(p.lat, p.lon, view.z);
			return { ...p, x: q.x - view.x0, y: top + q.y - view.y0 };
		})
	);
	const vmax = $derived(Math.max(0, ...pts.map((p) => Math.abs(p.v ?? 0))));
	const radius = (v: number | null) => (series[2] && vmax > 0 && v !== null ? 3 + 13 * Math.sqrt(Math.abs(v) / vmax) : 5);

	// Densité : grille lissée, pondérée par la valeur (positive) s'il y en a
	// une. Le rayon suit l'échelle de la carte (borné).
	const CELL = 6;
	const heatR = $derived(heatRadius(Math.max(20, Math.min(w, h) / 8), view.z, fit.z, w, h));
	const grid = $derived(heat ? heatGrid(placed.map((p) => ({ x: p.x, y: p.y - top, w: series[2] ? Math.max(0, p.v ?? 0) : 1 })), w, h, CELL, heatR) : null);
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

	// Tuiles : chargées une à une ; null = indisponible. Une demande encore en
	// file que la vue ne veut plus (zoom ou déplacement entre-temps) ne part pas.
	let images = $state<Record<string, string | null>>({});
	let alive = true;
	let wantedKeys = new Set<string>();
	const key = (z: number, x: number, y: number) => `${z}/${x}/${y}`;
	$effect(() => {
		const want = wantedTiles;
		let live = true;
		wantedKeys = new Set(want.map((t) => key(t.z, t.x, t.y)));
		for (const t of want) {
			const k = key(t.z, t.x, t.y);
			if (untrack(() => k in images) || tileNow(t.z, t.x, t.y)) continue;
			loadTile(t.z, t.x, t.y, () => live && wantedKeys.has(k)).then((src) => {
				if (alive && src !== undefined) images = { ...images, [k]: src };
			});
		}
		return () => (live = false);
	});
	const src = (z: number, x: number, y: number) => images[key(z, x, y)] ?? tileNow(z, x, y) ?? null;
	const has = (z: number, x: number, y: number) => !!src(z, x, y);
	const shown = $derived(tiles.filter((t) => has(t.z, t.x, t.y)));
	// Pendant le chargement : tuiles déjà là d'un autre niveau (parente agrandie ou filles réduites).
	const stand = $derived(fallbackTiles(tiles, view, has));
	const offline = $derived(wantedTiles.length > 0 && wantedTiles.every((t) => images[key(t.z, t.x, t.y)] === null));

	// --- Zoom et déplacement ------------------------------------------------

	let box = $state<HTMLDivElement>();
	let hover = $state(false);
	const current = (): MapCam => cam ?? viewCam(fit, w, h);
	let saveTimer: ReturnType<typeof setTimeout> | undefined;

	function flushSave() {
		clearTimeout(saveTimer);
		if (!dirty) return;
		dirty = false;
		onview?.(cam ? roundCam(cam) : null);
	}
	function changed(c: MapCam, now = false) {
		cam = c;
		dirty = true;
		clearTimeout(saveTimer);
		if (now) flushSave();
		else saveTimer = setTimeout(flushSave, 600);
	}
	/** Zoom de dz niveaux autour d'un point de la carte (par défaut son centre). */
	function zoomBy(dz: number, px = w / 2, py = h / 2) {
		const c = current();
		changed(zoomAt(c, w, h, px, py, c.zoom + dz));
	}
	function recenter() {
		cam = null;
		dirty = true;
		flushSave();
	}
	onDestroy(() => {
		alive = false;
		flushSave();
	});

	/** Échelle d'affichage (zoom de la feuille) : pixels d'écran par pixel de la carte. */
	function scale(): number {
		const r = box?.getBoundingClientRect();
		return r && width > 0 && r.width > 0 ? r.width / width : 1;
	}
	/** Position d'un événement dans la zone de la carte (sous le titre). */
	function local(e: MouseEvent): { x: number; y: number } {
		const r = box!.getBoundingClientRect();
		const k = scale();
		return { x: (e.clientX - r.left) / k, y: (e.clientY - r.top) / k - top };
	}
	const onControls = (e: Event) => !!(e.target as Element | null)?.closest?.('.mapctl');

	// Écouteurs natifs : la molette doit pouvoir empêcher le défilement
	// (passive: false), et un geste pris par la carte ne doit pas remonter à
	// l'hôte (la feuille déplacerait le graphique, le document le texte).
	let downs = 0; // mousedown successifs pris par la carte (double-clic : les deux)
	$effect(() => {
		const el = box;
		if (!el || draft) return;
		const wheel = (e: WheelEvent) => {
			if (!interactive || e.ctrlKey || !pts.length) return;
			e.preventDefault();
			e.stopPropagation();
			const p = local(e);
			zoomBy(wheelZoom(e.deltaY, e.deltaMode), p.x, p.y);
		};
		const down = (e: MouseEvent) => {
			if (onControls(e)) {
				e.stopPropagation();
				downs = 0;
				return;
			}
			if (e.button !== 0 || !interactive || e.altKey || !pts.length) {
				downs = 0;
				return;
			}
			e.preventDefault();
			e.stopPropagation();
			downs = e.detail <= 1 ? 1 : downs + 1;
			const x0 = e.clientX;
			const y0 = e.clientY;
			const c0 = current();
			const k = scale();
			let moved = false;
			const move = (m: MouseEvent) => {
				const dx = (m.clientX - x0) / k;
				const dy = (m.clientY - y0) / k;
				if (!moved && Math.hypot(dx, dy) < 3) return;
				moved = true;
				dragging = true;
				cam = panBy(c0, dx, dy);
				dirty = true;
				clearTimeout(saveTimer);
			};
			const up = () => {
				window.removeEventListener('mousemove', move);
				window.removeEventListener('mouseup', up);
				dragging = false;
				if (moved) flushSave();
			};
			window.addEventListener('mousemove', move);
			window.addEventListener('mouseup', up);
		};
		const dbl = (e: MouseEvent) => {
			if (onControls(e)) {
				e.stopPropagation();
				return;
			}
			if (!interactive || downs < 2 || !pts.length) return;
			e.preventDefault();
			e.stopPropagation();
			const p = local(e);
			zoomBy(e.shiftKey ? -ZOOM_STEP : ZOOM_STEP, p.x, p.y);
		};
		el.addEventListener('wheel', wheel, { passive: false });
		el.addEventListener('mousedown', down);
		el.addEventListener('dblclick', dbl);
		return () => {
			el.removeEventListener('wheel', wheel);
			el.removeEventListener('mousedown', down);
			el.removeEventListener('dblclick', dbl);
		};
	});

	// Clavier (+ et −) quand la carte est sélectionnée et que le focus est
	// autour d'elle (la feuille, le texte du document) : pris avant l'hôte,
	// qui commencerait sinon une saisie.
	$effect(() => {
		if (!interactive || draft) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.ctrlKey || e.metaKey || e.altKey || !box || !pts.length) return;
			const dz = e.key === '+' || e.key === '=' ? ZOOM_STEP : e.key === '-' || e.key === '_' ? -ZOOM_STEP : 0;
			if (!dz) return;
			const t = e.target as HTMLElement | null;
			if (t && t !== document.body) {
				if (t.closest('input, textarea, select')) return;
				if (!t.contains(box) && !box.contains(t)) return;
			}
			e.preventDefault();
			e.stopPropagation();
			zoomBy(dz);
		};
		window.addEventListener('keydown', onKey, true);
		return () => window.removeEventListener('keydown', onKey, true);
	});
	const showControls = $derived(!draft && pts.length > 0 && (hover || interactive));
</script>

{#if draft}
<svg {width} {height} viewBox="0 0 {width} {height}" role="img" aria-label="Carte : coordonnées à choisir">
	<rect {width} {height} fill="#e5e7eb" />
	<rect x="0.5" y="0.5" width={width - 1} height={height - 1} fill="none" stroke="#c4c8cd" stroke-dasharray="4 3" />
	<g transform="translate({width / 2} {height / 2 - 18})" fill="none" stroke="#9ca3af" stroke-width="2" stroke-linejoin="round">
		<path d="M -16 -10 L -6 -14 L 6 -10 L 16 -14 L 16 10 L 6 14 L -6 10 L -16 14 Z M -6 -14 L -6 10 M 6 -10 L 6 14" />
	</g>
	<text x={width / 2} y={height / 2 + 16} text-anchor="middle" class="draft">Carte</text>
	<text x={width / 2} y={height / 2 + 34} text-anchor="middle" class="small muted">Choisissez les coordonnées dans le volet</text>
</svg>
{:else}
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="mapbox" class:pan={interactive && pts.length > 0} class:dragging bind:this={box} onmouseenter={() => (hover = true)} onmouseleave={() => (hover = false)}>
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
		{#each stand as t (`s${t.z}/${t.u}/${t.y}`)}
			<image href={src(t.z, t.x, t.y)} x={t.left} y={top + t.top} width={t.size + 0.5} height={t.size + 0.5} preserveAspectRatio="none" />
		{/each}
		{#each shown as t (`${t.z}/${t.u}/${t.y}`)}
			<image href={src(t.z, t.x, t.y)} x={t.left} y={top + t.top} width={t.size === 256 ? 256 : t.size + 0.5} height={t.size === 256 ? 256 : t.size + 0.5} preserveAspectRatio="none" />
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
	{#if shown.length || stand.length}
		<rect x={w - 98} y={top + h - 16} width="98" height="16" fill="#fff" fill-opacity="0.75" />
		<text x={w - 4} y={top + h - 4} text-anchor="end" class="attr">© OpenStreetMap</text>
	{:else if offline}
		<text x={w - 6} y={top + h - 6} text-anchor="end" class="attr"><title>Le fond de carte vient de tile.openstreetmap.org : autorisez cet hôte (onglet SCRIPT › Accès réseau) ou vérifiez la connexion.</title>Fond de carte indisponible</text>
	{/if}
	{#if !pts.length}
		<text x={w / 2} y={top + h / 2} text-anchor="middle" class="small halo">Aucun point : latitude et longitude attendues</text>
	{/if}
</svg>
{#if showControls}
	<div class="mapctl" style:top="{top + 6}px">
		<button type="button" title="Zoom avant (+)" aria-label="Zoom avant" disabled={view.z >= MAX_ZOOM} onclick={() => zoomBy(ZOOM_STEP)}>+</button>
		<button type="button" title="Zoom arrière (−)" aria-label="Zoom arrière" disabled={view.z <= MIN_ZOOM} onclick={() => zoomBy(-ZOOM_STEP)}>−</button>
		{#if cam}<button type="button" class="fit" title="Recentrer sur les points" aria-label="Recentrer sur les points" onclick={recenter}>
			<svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path d="M1 5V1h4M9 1h4v4M13 9v4H9M5 13H1V9" /><circle cx="7" cy="7" r="1.6" fill="currentColor" stroke="none" /></svg>
		</button>{/if}
	</div>
{/if}
</div>
{/if}

<style>
	.mapbox {
		position: relative;
	}
	.mapbox.pan {
		cursor: grab;
	}
	.mapbox.dragging {
		cursor: grabbing;
	}
	.mapctl {
		position: absolute;
		right: 6px;
		display: flex;
		flex-direction: column;
		border: 1px solid #c4c8cd;
		border-radius: 4px;
		background: #fff;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
		overflow: hidden;
		cursor: default;
	}
	.mapctl button {
		width: 24px;
		height: 24px;
		padding: 0;
		border: 0;
		background: #fff;
		color: #1f2328;
		font: 600 16px/1 'Segoe UI', system-ui, sans-serif;
		display: grid;
		place-items: center;
		cursor: pointer;
	}
	.mapctl button + button {
		border-top: 1px solid #e1e4e8;
	}
	.mapctl button:hover:not(:disabled) {
		background: #e9f3ee;
		color: #217346;
	}
	.mapctl button:disabled {
		color: #b0b5bb;
		cursor: default;
	}
	.mapctl .fit svg {
		display: block;
	}

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
	.draft {
		font-size: 14px;
		font-weight: 600;
		fill: #6b7280;
	}
	.muted {
		fill: #6b7280;
	}
	.grat {
		stroke: #dde2e6;
	}
</style>
