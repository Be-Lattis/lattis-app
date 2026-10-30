<!--
	Menu contextuel d'un graphique.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { dismiss } from '../dismiss';
	import Submenu from '#lib/ui/Submenu.svelte';
	import { clip, clipped } from '#lib/ui/menu.js';
	import { chartMenu as chartMenuGroups, chartSubmenu } from '../ctxmenu';
	import { type FormulaHold } from '../crossref';
	import { t as tr } from '#lib/i18n/index.js';
	import { type ChartConfig, type SheetChart } from './model';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface ChartMenuView {
		readonly chartClip: ChartConfig | null;
		chartMenu: { ch: SheetChart; x: number; y: number } | null;
		readonly copyChart: (ch: SheetChart, cut: boolean) => Promise<void>;
		readonly copyChartImage: (ch: SheetChart) => Promise<void>;
		readonly editChart: (ch: SheetChart) => void;
		readonly hold: FormulaHold | null;
		readonly locked: string;
		readonly pasteChart: (cfg: ChartConfig) => void;
		readonly removeChart: (id: number) => Promise<void>;
		readonly structure: boolean;
	}

	let { v }: { v: ChartMenuView } = $props();
</script>

{#if v.chartMenu}
	{@const m = v.chartMenu}
	{@const name = m.ch.config.title?.trim() ? tr('grid.chart.aria.titled', { title: clip(m.ch.config.title) }) : tr('grid.chart.aria')}
	{#snippet chartItem(id: string)}
		{#if id === 'cut'}
			<button onclick={() => v.copyChart(m.ch, true)}>{tr('common.cut')} <span class="keys">Ctrl+X</span></button>
		{:else if id === 'copy'}
			<button onclick={() => v.copyChart(m.ch, false)}>{tr('common.copy')} <span class="keys">Ctrl+C</span></button>
		{:else if id === 'copyImage'}
			<button onclick={() => v.copyChartImage(m.ch)}>{tr('grid.chart.copyImage')}</button>
		{:else if id === 'pasteHere'}
			<button
				disabled={!v.chartClip}
				onclick={() => {
					v.chartMenu = null;
					if (v.chartClip) v.pasteChart(v.chartClip);
				}}>{tr('grid.chart.pasteHere')}</button
			>
		{:else if id === 'edit'}
			<button
				onclick={() => {
					v.chartMenu = null;
					v.editChart(m.ch);
				}}>{tr('grid.chart.edit')}</button
			>
		{:else if id === 'delete'}
			<button onclick={() => v.removeChart(m.ch.id)}>{tr('grid.chart.delete')}</button>
		{:else if id === 'chart'}
			<!-- Le graphique, titré de son nom (son titre) quand il en a un ; sa suppression en dernier. -->
			<Submenu label={name} title={clipped(m.ch.config.title) ? m.ch.config.title : undefined}>
				{#each chartSubmenu({ edit: !(v.locked || v.structure) || !!v.hold }) as g, gi (gi)}
					{#if gi}<hr />{/if}
					{#each g as cid (cid)}{@render chartItem(cid)}{/each}
				{/each}
			</Submenu>
		{/if}
	{/snippet}
	<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (v.chartMenu = null)}>
		<!-- Le sous-menu du graphique, puis le presse-papiers (ctxmenu.ts). -->
		{#each chartMenuGroups({ image: m.ch.config.kind !== 'map' && !m.ch.error }) as g, gi (gi)}
			{#if gi}<hr />{/if}
			{#each g as id (id)}{@render chartItem(id)}{/each}
		{/each}
	</div>
{/if}

<style>

	.menu {
		position: absolute;
		z-index: 5;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 240px;
		max-width: 320px; /* MENU_MAX_WIDTH : au-delà, les libellés s'abrègent (…) */
	}

	.menu button {
		display: block;
		width: 100%;
		text-align: left;
		padding: 6px 14px;
		background: none;
		border: none;
		font: inherit;
		cursor: pointer;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.menu button:disabled {
		color: #9aa0a6;
		cursor: default;
	}

	.menu button:hover {
		background: #e3efe7;
	}

	.menu hr {
		border: none;
		border-top: 1px solid #e3e5e8;
		margin: 4px 0;
	}

	.menu .keys {
		float: right;
		color: #6b7280;
		font-size: 12px;
		margin-left: 16px;
	}
</style>
