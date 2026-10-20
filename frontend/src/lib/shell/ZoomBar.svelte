<script lang="ts">
	// Zoom de la barre d'état, comme Excel : − / curseur / + et le pourcentage
	// (un clic dessus propose les niveaux usuels). 100 % au milieu du curseur.
	import { dismiss } from '#lib/grid/dismiss.js';
	import { ZOOM_LEVELS, sliderOf, stepZoom, zoomLabel, zoomOfSlider } from '#lib/grid/zoom.js';
	import { t } from '#lib/i18n/index.js';

	interface Props {
		zoom: number;
		onzoom: (z: number) => void;
	}

	let { zoom, onzoom }: Props = $props();
	// Liste des niveaux, posée au-dessus du pourcentage (la barre d'état coupe ce qui déborde).
	let open = $state<{ right: number; bottom: number } | null>(null);
	let track: HTMLDivElement;

	function fromPointer(e: PointerEvent) {
		const r = track.getBoundingClientRect();
		let z = zoomOfSlider((e.clientX - r.left) / r.width);
		// Le milieu « accroche » 100 %, comme Excel.
		if (Math.abs(z - 1) < 0.04) z = 1;
		onzoom(z);
	}

	function down(e: PointerEvent) {
		track.setPointerCapture(e.pointerId);
		fromPointer(e);
	}
</script>

<div class="zoombar">
	<button class="step" title={t('shell.zoom.out')} aria-label={t('shell.zoom.out')} onclick={() => onzoom(stepZoom(zoom, -1))}>−</button>
	<div
		class="track"
		bind:this={track}
		role="slider"
		tabindex="-1"
		aria-label={t('shell.zoom.slider')}
		aria-valuemin={10}
		aria-valuemax={400}
		aria-valuenow={Math.round(zoom * 100)}
		onpointerdown={down}
		onpointermove={(e) => e.buttons === 1 && fromPointer(e)}
	>
		<span class="mid"></span>
		<span class="thumb" style:left="{sliderOf(zoom) * 100}%"></span>
	</div>
	<button class="step" title={t('shell.zoom.in')} aria-label={t('shell.zoom.in')} onclick={() => onzoom(stepZoom(zoom, 1))}>+</button>
	<button
		class="pct"
		title={t('shell.zoom.level')}
		onclick={(e) => {
			const r = e.currentTarget.getBoundingClientRect();
			open = open ? null : { right: window.innerWidth - r.right, bottom: window.innerHeight - r.top + 2 };
		}}>{zoomLabel(zoom)}</button>
	{#if open}
		<div class="levels" style:right="{open.right}px" style:bottom="{open.bottom}px" use:dismiss={() => (open = null)}>
			{#each [...ZOOM_LEVELS].reverse() as z (z)}
				<button
					class:on={Math.abs(z - zoom) < 0.001}
					onclick={() => {
						open = null;
						onzoom(z);
					}}>{zoomLabel(z)}</button
				>
			{/each}
		</div>
	{/if}
</div>

<style>
	.zoombar {
		position: relative;
		display: flex;
		align-items: center;
		gap: 4px;
	}
	button {
		font: inherit;
		color: inherit;
		background: none;
		border: none;
		cursor: pointer;
		padding: 0 4px;
	}
	.step {
		font-size: 14px;
		line-height: 1;
	}
	.pct {
		min-width: 44px;
		text-align: right;
	}
	button:hover {
		color: #fff;
	}
	.track {
		position: relative;
		width: 100px;
		height: 14px;
		cursor: pointer;
	}
	.track::before {
		content: '';
		position: absolute;
		left: 0;
		right: 0;
		top: 6px;
		height: 2px;
		background: #b8d8c4;
	}
	.mid {
		position: absolute;
		left: 50%;
		top: 3px;
		width: 1px;
		height: 8px;
		background: #b8d8c4;
	}
	.thumb {
		position: absolute;
		top: 2px;
		width: 4px;
		height: 10px;
		margin-left: -2px;
		background: #fff;
		border-radius: 1px;
	}
	.levels {
		position: fixed;
		z-index: 50;
		display: flex;
		flex-direction: column;
		background: #fff;
		color: #1f2328;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.18);
		padding: 4px 0;
		min-width: 90px;
	}
	.levels button {
		text-align: left;
		padding: 4px 14px;
	}
	.levels button:hover {
		background: #e3efe7;
		color: #1f2328;
	}
	.levels button.on {
		font-weight: 600;
	}
</style>
