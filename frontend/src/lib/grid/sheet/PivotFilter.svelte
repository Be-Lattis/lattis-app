<!--
	Menu ▾ d'un champ de tableau croisé dans la grille : tri, valeurs.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { dismissPanel } from '../dismiss';
	import { searchValues, type PanelValue } from '../filtercond';
	import { clip, clipped } from '#lib/ui/menu.js';
	import { t as tr } from '#lib/i18n/index.js';
	import { type SheetPivot, type SheetSource } from './model';
	import type { TabChoiceItem } from '../tabchoice';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface PivotFilterView {
		readonly applyPivotFilter: (clear?: boolean) => void;
		readonly focusFirst: (node: HTMLElement) => void;
		readonly loadPivotValues: () => void;
		readonly openPivotCustomSort: () => void;
		pivotDlg: {
		p: SheetPivot;
		field: string;
		/** filter : filtre de rapport ; rows : étiquette d'un champ des lignes (tri en plus). */
		kind: 'filter' | 'rows';
		/** Arborescence : champs des lignes au choix (un seul ▾ sur « Étiquettes de lignes »). */
		fields?: string[];
		x: number;
		y: number;
		values: PanelValue[];
		loading: boolean;
		search: string;
	} | null;
		readonly pivotFiltered: (p: SheetPivot, f: string) => boolean;
		readonly runTabChoice: (table: string, column: string, it: TabChoiceItem) => void;
		readonly sortPivot: (dir: "asc" | "desc") => void;
		readonly source: SheetSource;
		readonly tabChoiceFor: (table: string, column: string, o?: { formula?: boolean; field?: boolean; }) => TabChoiceItem;
		readonly toView: (v: number) => number;
		readonly viewH: number;
	}

	let { ui }: { ui: PivotFilterView } = $props();
</script>

{#if ui.pivotDlg}
	{@const d = ui.pivotDlg}
	{@const shown = searchValues(d.values, d.search)}
	<!-- Menu ▾ d'un TCD (comme Excel) : tri des étiquettes, valeurs à cocher. -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="rules filter colfilter anchored"
		style:left="{d.x}px"
		style:top="{d.y}px"
		style:max-height="{Math.max(200, ui.toView(ui.viewH) - d.y - 8)}px"
		use:ui.focusFirst
		use:dismissPanel={() => (ui.pivotDlg = null)}
		onkeydown={(ev) => {
			ev.stopPropagation();
			if (ev.key === 'Escape') ui.pivotDlg = null;
			if (ev.key === 'Enter' && !(ev.target instanceof HTMLButtonElement)) ui.applyPivotFilter();
		}}
	>
		{#if d.fields && d.fields.length > 1}
			<label class="pick-field"
				>{tr('grid.pivot.filter.field')}
				<select
					value={d.field}
					onchange={(ev) => {
						d.field = (ev.currentTarget as HTMLSelectElement).value;
						d.search = '';
						ui.loadPivotValues();
					}}
				>
					{#each d.fields as f (f)}<option value={f}>{f}</option>{/each}
				</select>
			</label>
		{:else}
			<div class="menu-title" title={clipped(d.field) ? d.field : undefined}>{d.kind === 'filter' ? tr('grid.pivot.filter.reportField', { field: clip(d.field) }) : tr('grid.quoted', { text: clip(d.field) })}</div>
		{/if}
		{#if d.kind === 'rows'}
			<button class="sort" class:on={d.p.config.sort?.[d.field] === 'asc'} onclick={() => ui.sortPivot('asc')}>{tr('grid.sort.text.asc')}</button>
			<button class="sort" class:on={d.p.config.sort?.[d.field] === 'desc'} onclick={() => ui.sortPivot('desc')}>{tr('grid.sort.text.desc')}</button>
			{#if ui.source.pivotFieldValues}
				<button class="sort" class:on={!!d.p.config.orders?.[d.field]} onclick={ui.openPivotCustomSort}>{tr('grid.pivot.menu.customSort')}</button>
			{/if}
			<hr />
		{/if}
		{#if d.kind === 'filter' ? d.p.config.filters?.some((f) => f.field === d.field && f.values.length) : ui.pivotFiltered(d.p, d.field)}
			<button class="sort" onclick={() => ui.applyPivotFilter(true)}>{tr('grid.filter.clearOf', { field: d.field })}</button>
		{/if}
		{#if ui.source.setTabChoice}
			{@const tc = ui.tabChoiceFor(d.p.config.source, d.field, { field: true })}
			<button class="sort" title={tc.title} disabled={tc.disabled} onclick={() => ui.runTabChoice(d.p.config.source, d.field, tc)}>{tc.label}</button>
			<hr />
		{/if}
		<div class="add">
			<input class="search" bind:value={d.search} placeholder={tr('grid.filter.search.placeholder')} />
		</div>
		<div class="values">
			{#if d.loading}
				<p class="loading"><span class="spinner" aria-hidden="true"></span> {tr('grid.filter.loading')}</p>
			{:else}
				<label class="all">
					<input
						type="checkbox"
						checked={shown.every((v) => v.on)}
						onchange={(ev) => {
							const on = (ev.currentTarget as HTMLInputElement).checked;
							for (const v of shown) v.on = on;
						}}
					/>
					{d.search.trim() ? tr('grid.filter.selectAllResults') : d.kind === 'filter' ? tr('grid.pivot.filter.all') : tr('grid.filter.selectAll')}
				</label>
				{#each shown as v (v.text)}
					<label>
						<input type="checkbox" bind:checked={v.on} />
						<span class="what">{v.text === '' ? tr('grid.filter.blanks') : v.text}</span>
						<span class="count">{v.count}</span>
					</label>
				{/each}
			{/if}
		</div>
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" disabled={d.loading} onclick={() => ui.applyPivotFilter()}>{tr('common.ok')}</button>
			<button onclick={() => (ui.pivotDlg = null)}>{tr('common.cancel')}</button>
		</div>
	</div>
{/if}

<style>
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}

	button.primary:disabled {
		opacity: 0.5;
	}

	.rules {
		position: absolute;
		z-index: 6;
		top: 60px;
		left: 50%;
		transform: translateX(-50%);
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 6px 18px rgba(0, 0, 0, 0.18);
		padding: 6px 0 8px;
		min-width: 460px;
	}

	.rules .add,
	.rules .foot {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 3px 14px;
	}

	.rules .what {
		flex: 1;
	}

	.filter {
		min-width: 300px;
	}

	.colfilter {
		max-width: 310px;
		overflow-y: auto;
		box-sizing: border-box;
	}

	.rules.anchored {
		transform: none;
	}

	.filter .sort {
		display: block;
		width: 100%;
		text-align: left;
		padding: 5px 14px;
		border: none;
		border-radius: 0;
	}

	.filter .sort:hover {
		background: #e3efe7;
	}

	.filter .search {
		width: 100%;
	}

	.loading {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 6px 10px;
		color: #6b7280;
	}

	.spinner {
		width: 12px;
		height: 12px;
		flex: none;
		border: 2px solid #d1d5db;
		border-top-color: #217346;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.spinner {
			animation-duration: 2.5s;
		}
	}

	.filter .values {
		max-height: 260px;
		overflow: auto;
		margin: 4px 14px;
		border: 1px solid #e5e7eb;
		padding: 4px 0;
	}

	.filter label {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 1px 8px;
	}

	.filter .values input {
		width: auto;
	}

	.filter .count {
		color: #6b7280;
		font-size: 11px;
	}

	.rules input {
		font: inherit;
		width: 90px;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}

	.rules button {
		font: inherit;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}

	.rules button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}

	.rules .sp {
		flex: 1;
	}

	.rules hr {
		border: none;
		border-top: 1px solid #e5e7eb;
		margin: 6px 0;
	}

	.menu-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}

	.pick-field {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}

	.pick-field select {
		flex: 1;
		font: inherit;
		color: #1f2328;
	}
</style>
