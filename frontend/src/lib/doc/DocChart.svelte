<script lang="ts" module>
	import type { ChartInfo } from '#bindings/internal/canvas/models.js';
	// Dernières données et largeur lues, par source (voir dataKey).
	const lastInfo = new Map<string, ChartInfo>();
	const lastWidth = new Map<string, number>();
</script>

<script lang="ts">
	// Graphique d'un onglet Document : ses données sont relues à chaque
	// modification du classeur (live.stamp), sa largeur suit celle du texte.
	import { untrack } from 'svelte';
	import Chart from '#lib/chart/Chart.svelte';
	import type { ChartConfig } from '#lib/grid/Sheet.svelte';

	interface Props {
		config: ChartConfig;
		/** Change à chaque modification du classeur. */
		live: { stamp: number; animate: boolean };
		load: (cfg: ChartConfig) => Promise<ChartInfo>;
		selected?: boolean;
		/** Nouvelle configuration (carte zoomée ou déplacée : vue enregistrée, annulable). */
		onconfig?: (cfg: ChartConfig) => void;
	}

	let { config, live, load, selected = false, onconfig }: Props = $props();

	// Données déjà lues pour la même source : un graphique remplacé (vue de
	// carte enregistrée) renaît sans clignoter, puis se relit.
	const dataKey = (c: ChartConfig) => JSON.stringify([c.source, c.kind, c.x, c.series, c.mapCoords, c.mapDraft]);
	let info = $state.raw<ChartInfo | null>(untrack(() => lastInfo.get(dataKey(config)) ?? null));
	let width = $state(untrack(() => lastWidth.get(dataKey(config)) ?? 0));
	$effect(() => {
		if (width > 0) lastWidth.set(dataKey(untrack(() => config)), width);
	});
	// Hauteur mesurée : le graphique suit la poignée pendant le geste.
	let boxH = $state(0);
	const height = $derived(config.height || 300);

	$effect(() => {
		void live.stamp;
		const cfg = config;
		untrack(() => {
			load(cfg).then(
				(r) => {
					info = r;
					if (!r.error) lastInfo.set(dataKey(cfg), r);
				},
				(e) => (info = { id: 0, row: 0, col: 0, config: cfg as never, labels: [], series: [], error: String(e instanceof Error ? e.message : e) })
			);
		});
	});
</script>

<div class="dchart" class:selected bind:clientWidth={width} bind:clientHeight={boxH} style:height="{height}px" style:width={config.width ? config.width + 'px' : null} style:max-width="100%">
	{#if info?.error}
		<div class="err">Graphique « {config.title || config.source} » : {info.error}</div>
	{:else if info && width > 0}
		<Chart kind={config.kind} labels={info.labels ?? []} series={(info.series ?? []) as never} {width} height={boxH || height} style={config} animate={live.animate} interactive={selected} onview={(v) => onconfig?.({ ...config, mapView: v ?? undefined })} />
	{/if}
</div>

<style>
	.dchart {
		position: relative;
		margin: 6px 0;
		border: 1px solid transparent;
		border-radius: var(--float-radius, 6px);
		cursor: pointer;
		user-select: none;
	}
	.dchart:hover {
		border-color: #d4d7db;
	}
	.dchart.selected {
		border-color: #217346;
		box-shadow: 0 0 0 1px #217346;
	}
	.err {
		padding: 16px;
		color: #b42318;
		font-size: 12px;
		background: #fef3f2;
		border-radius: var(--float-radius, 6px);
	}
</style>
