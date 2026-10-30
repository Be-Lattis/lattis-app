<!--
	Panneau ▾ d'une colonne de tableau : tri, filtres (valeurs, conditions),
	type de données, clé ; la logique est dans filters.svelte.ts.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { dismissPanel } from '../dismiss';
	import { condArgs, condLabel, condOps, describeConds, filterKind, kindTitle, type CondOp, type PanelValue } from '../filtercond';
	import { clip, clipped } from '#lib/ui/menu.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { typeIcon, typeIconName, typeLabel, sortLabels, type ColumnType } from '../format';
	import { t as tr } from '#lib/i18n/index.js';
	import { type TypeMix, type SheetSource, type Entry } from './model';
	import type { Section } from '../../sql/reference.svelte.js';
	import type { TabChoiceItem } from '../tabchoice';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface FilterPanelView {
		readonly TYPES: ColumnType[];
		readonly applyFilter: (clear?: boolean) => Promise<void>;
		readonly chooseKey: (set: boolean) => void;
		readonly chooseType: (to: ColumnType, confirmed?: boolean) => Promise<void>;
		readonly closeFilter: () => void;
		readonly dismissFilter: () => void;
		filterDlg: {
		e: Entry;
		i: number;
		x: number;
		y: number;
		values: PanelValue[];
		/** Liste des valeurs en cours de chargement (grand tableau). */
		loading?: boolean;
		search: string;
		errors: boolean;
		error: string;
		touched: boolean; // liste ou recherche modifiée
		sub: boolean; // sous-menu « Filtres textuels / numériques » ouvert
		// Filtre personnalisé en cours (deux conditions au plus, comme Excel).
		custom: { conds: { op: CondOp | ''; value: string; to: string }[]; or: boolean } | null;
		// Section « Type de données » : conversion en attente de confirmation,
		// raison pour laquelle la colonne ne peut pas être clé, valeurs hors type.
		pending?: { to: ColumnType; n: number };
		keyWhy?: string | null;
		mix?: TypeMix | null;
	} | null;
		readonly filterShown: () => PanelValue[];
		readonly focusFirst: (node: HTMLElement) => void;
		readonly menuAction: (fn: (() => Promise<unknown>) | undefined, keepFocus?: boolean) => Promise<boolean>;
		readonly mixText: (m: TypeMix) => string;
		readonly openCustomSort: (e: Entry, i: number) => void;
		readonly runTabChoice: (table: string, column: string, it: TabChoiceItem) => void;
		readonly source: SheetSource;
		readonly startCustom: (op?: CondOp) => void;
		readonly tabChoiceFor: (table: string, column: string, o?: { formula?: boolean; field?: boolean; }) => TabChoiceItem;
		readonly toView: (v: number) => number;
		readonly viewH: number;
	}

	let { v }: { v: FilterPanelView } = $props();
</script>

{#if v.filterDlg}
	{@const d = v.filterDlg}
	{@const shown = v.filterShown()}
	{@const fcol = d.e.t.columns[d.i]}
	{@const kind = filterKind(((fcol?.formula ? fcol?.result : fcol?.type) || 'text') as ColumnType)}
	{@const filterable = !!v.source.setFilter}
	{@const typed = !!v.source.setType && !fcol?.formula}
	<!-- Menu ▾ de la colonne (comme Excel) : tri, filtres, type de données, clé. -->
	<div
		class="rules filter colfilter anchored"
		role="dialog"
		tabindex="-1"
		aria-label={tr('grid.quoted', { text: fcol?.name ?? '' })}
		style:left="{d.x}px"
		style:top="{d.y}px"
		style:max-height="{Math.max(200, v.toView(v.viewH) - d.y - 8)}px"
		use:v.focusFirst
		use:dismissPanel={v.dismissFilter}
		onkeydown={(ev) => {
			ev.stopPropagation();
			if (ev.key === 'Escape') v.closeFilter();
			if (ev.key === 'Enter' && !(ev.target instanceof HTMLButtonElement)) v.applyFilter();
		}}
	>
		<div class="menu-title" title={clipped(fcol?.name) ? fcol?.name : undefined}>{tr('grid.quoted', { text: clip(fcol?.name) })}</div>
		{#if v.source.sortTable}
			<!-- Tri en haut du panneau, comme Excel. -->
			<button
				class="sort"
				onclick={() => {
					v.filterDlg = null;
					v.menuAction(() => v.source.sortTable!(d.e.t, d.i, false));
				}}>{sortLabels(fcol?.type ?? 'text', fcol?.result)[0]}</button
			>
			<button
				class="sort"
				onclick={() => {
					v.filterDlg = null;
					v.menuAction(() => v.source.sortTable!(d.e.t, d.i, true));
				}}>{sortLabels(fcol?.type ?? 'text', fcol?.result)[1]}</button
			>
			{#if v.source.sortTableCustom}
				<button class="sort" onclick={() => v.openCustomSort(d.e, d.i)}>{tr('grid.pivot.menu.customSort')}</button>
			{/if}
			<hr />
		{/if}
		{#if fcol?.filter}
			<button class="sort" onclick={() => v.applyFilter(true)}>{tr('grid.filter.clearOf', { field: fcol.name })}</button>
		{/if}
		{#if v.source.setTabChoice && fcol}
			{@const tc = v.tabChoiceFor(d.e.t.name, fcol.name, { formula: !!fcol.formula })}
			<button class="sort" title={tc.title} disabled={tc.disabled} onclick={() => v.runTabChoice(d.e.t.name, fcol.name, tc)}>{tc.label}</button>
			<hr />
		{/if}
		{#if filterable}
			<!-- Filtres par condition, comme le sous-menu d'Excel. -->
			<button class="sort cond-menu" class:on={d.sub} aria-expanded={d.sub} onclick={() => (d.sub = !d.sub)}>
				{kindTitle(kind)}<span class="sp"></span>{d.sub ? '▾' : '▸'}
			</button>
			{#if d.sub}
				<div class="cond-list">
					{#each condOps(kind) as op (op)}
						<button class="sort" onclick={() => v.startCustom(op)}>{condLabel(op, kind)}{condArgs(op) ? '…' : ''}</button>
					{/each}
					<button class="sort" onclick={() => v.startCustom('eq')}>{tr('grid.filter.custom')}</button>
				</div>
			{/if}
			{#if fcol?.filter?.conds?.length && !d.custom}
				<p class="menu-note">
					{tr('grid.filter.active', { conds: describeConds(fcol.filter, kind) })}
					<button class="link" onclick={() => v.startCustom()}>{tr('grid.controls.menu.edit')}</button>
				</p>
			{/if}
			<hr />
			{#if d.custom}
				{@const cu = d.custom}
				<div class="custom">
					<p class="menu-note">{tr('grid.filter.custom.heading', { column: fcol?.name ?? '' })}</p>
					{#each cu.conds as c, k (k)}
						{#if k === 1}
							<div class="andor">
								<label><input type="radio" bind:group={cu.or} value={false} /> {tr('grid.filter.custom.and')}</label>
								<label><input type="radio" bind:group={cu.or} value={true} /> {tr('grid.filter.custom.or')}</label>
							</div>
						{/if}
						<div class="add">
							<select bind:value={c.op} aria-label={tr('grid.filter.custom.cond', { n: k + 1 })}>
								{#if k > 0}<option value="">{tr('grid.filter.custom.none')}</option>{/if}
								{#each condOps(kind) as op (op)}<option value={op}>{condLabel(op, kind)}</option>{/each}
							</select>
						</div>
						{#if c.op && condArgs(c.op) > 0}
							<div class="add">
								<input bind:value={c.value} placeholder={c.op === 'top' || c.op === 'bottom' ? tr('grid.filter.custom.number') : kind === 'date' ? tr('grid.filter.custom.date') : tr('grid.cf.placeholder.value')} />
								{#if condArgs(c.op) === 2}<span>{tr('grid.cf.and')}</span><input bind:value={c.to} placeholder={kind === 'date' ? tr('grid.filter.custom.date') : tr('grid.cf.placeholder.value')} />{/if}
							</div>
						{/if}
					{/each}
					{#if kind === 'text'}<p class="menu-note">{tr('grid.filter.custom.wildcards')}</p>{/if}
				</div>
			{:else}
				<div class="add">
					<input class="search" bind:value={d.search} oninput={() => (d.touched = true)} placeholder={tr('grid.filter.search.placeholder')} />
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
								d.touched = true;
							}}
						/>
						{d.search.trim() ? tr('grid.filter.selectAllResults') : tr('grid.filter.selectAll')}
					</label>
					{#each shown as v (v.text)}
						<label>
							<input type="checkbox" bind:checked={v.on} onchange={() => (d.touched = true)} />
							<span class="what">{v.text === '' ? tr('grid.filter.blanks') : v.text}</span>
							<span class="count">{v.count}</span>
						</label>
					{/each}
					{#if d.values.length >= 1000}<p class="menu-note">{tr('grid.filter.firstValues')}</p>{/if}
					{/if}
				</div>
			{/if}
		{/if}
		{#if d.e.t.columns[d.i]?.errors}
			<div class="add">
				<label><input type="checkbox" bind:checked={d.errors} /> {tr('grid.filter.errorsOnly', { n: d.e.t.columns[d.i]?.errors ?? 0 })}</label>
			</div>
		{/if}
		{#if d.error}<p class="menu-note error">{d.error}</p>{/if}
		{#if filterable}
			<div class="foot">
				{#if d.custom}<button onclick={() => (d.custom = null)}>{tr('grid.filter.custom.back')}</button>{/if}
				<span class="sp"></span>
				<button class="primary" disabled={d.loading && !d.custom} onclick={() => v.applyFilter()}>{tr('common.ok')}</button>
				<button onclick={v.closeFilter}>{tr('common.cancel')}</button>
			</div>
		{/if}
		{#if typed}
			<!-- Type de données de la colonne (inféré ou fixé), puis la clé du tableau. -->
			<hr />
			<div class="menu-title">{fcol.fixed ? tr('grid.type.title.fixed') : tr('grid.type.title.inferred')}</div>
			{#if d.pending}
				{@const p = d.pending}
				<p class="menu-note">
					{tr('grid.type.pending', { n: p.n, type: typeLabel(p.to) })}
				</p>
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" onclick={() => v.chooseType(p.to, true)}>{tr('grid.type.convertAnyway')}</button>
					<button onclick={() => (d.pending = undefined)}>{tr('common.cancel')}</button>
				</div>
			{:else}
				<div class="types" role="radiogroup" aria-label={tr('grid.type.aria')}>
					{#each v.TYPES as t (t)}
						<button
							class="sort type"
							class:on={t === fcol.type}
							role="radio"
							aria-checked={t === fcol.type}
							title={t === fcol.type ? (fcol.fixed ? tr('grid.type.fixed.tooltip') : tr('grid.type.inferred.tooltip')) : tr('grid.type.convert.tooltip', { type: typeLabel(t) })}
							onclick={() => v.chooseType(t)}
						>
							<span class="check">{t === fcol.type ? '✓' : ''}</span>{#if typeIconName(t)}<span class="type-icon"><Icon name={typeIconName(t)!} size={14} /></span>{:else}<span class="type-icon">{typeIcon(t)}</span>{/if}{typeLabel(t)}
						</button>
					{/each}
				</div>
				{#if d.mix && d.mix.type}
					{@const to = d.mix.type}
					<p class="menu-note">{v.mixText(d.mix)}</p>
					<button class="sort" onclick={() => v.chooseType(to, true)}
						>{tr('grid.type.revert', { type: typeLabel(to), n: d.mix.off })}</button
					>
				{/if}
				{#if fcol.errors}<p class="menu-note error">{tr('grid.type.errors', { n: fcol.errors })}</p>{/if}
			{/if}
		{:else if fcol?.formula && v.source.columnsToValues}
			<!-- Colonne calculée : pas de type à choisir ; elle peut devenir une colonne de valeurs. -->
			<hr />
			<div class="menu-title">{tr('grid.type.computed')}</div>
			<button
				class="sort"
				title={tr('grid.type.toValues.tooltip')}
				onclick={() => {
					v.filterDlg = null;
					v.menuAction(() => v.source.columnsToValues!(d.e.t, [d.i]));
				}}>{tr('grid.menu.toValues')}</button
			>
		{/if}
		{#if v.source.setKey && !fcol?.formula}
			<hr />
			{#if d.e.t.key === d.i}
				<p class="menu-note">🔑 {d.e.t.autoNumber === d.i ? tr('grid.key.note.auto') : tr('grid.key.note')}</p>
				<button class="sort" onclick={() => v.chooseKey(false)}>{tr('grid.key.remove')}</button>
			{:else if d.keyWhy === undefined}
				<p class="menu-note loading"><span class="spinner" aria-hidden="true"></span> {tr('grid.key.checking')}</p>
			{:else if d.keyWhy === ''}
				<button class="sort" onclick={() => v.chooseKey(true)}>🔑 {d.e.t.key !== undefined ? tr('grid.key.use.replace') : tr('grid.key.use')}</button>
			{:else if d.keyWhy}
				<p class="menu-note">{tr('grid.key.impossible', { why: d.keyWhy })}</p>
			{/if}
			{#if d.e.t.autoNumber === d.i && d.e.t.key !== d.i}
				<p class="menu-note">{tr('grid.key.sequence')}</p>
			{/if}
		{/if}
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

	.filter .sort.type {
		padding: 3px 14px;
	}

	.filter .sort.type .type-icon {
		width: 18px;
		margin-right: 6px;
		justify-content: center;
	}

	.filter .sort.type.on {
		font-weight: 600;
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

	.filter .cond-menu {
		display: flex;
	}

	.filter .cond-menu.on {
		background: #e3efe7;
	}

	.filter .cond-list .sort {
		padding-left: 28px;
	}

	.filter .custom select {
		flex: 1;
		font: inherit;
		padding: 2px 4px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}

	.filter .custom input {
		flex: 1;
		min-width: 0;
	}

	.filter .andor {
		display: flex;
		gap: 12px;
		padding: 0 14px;
	}

	.filter .andor input {
		width: auto;
	}

	.filter .menu-note .link {
		border: none;
		background: none;
		color: #217346;
		padding: 0 4px;
		text-decoration: underline;
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

	.menu-note {
		margin: 4px 14px;
		max-width: 260px;
		font-size: 12px;
		color: #4b5563;
	}

	.menu-note.error {
		color: #c0392b;
	}

	.type-icon {
		display: inline-flex;
		vertical-align: -3px;
		color: #6b8f78;
		--icon-accent: #217346;
	}

	.check {
		display: inline-block;
		width: 16px;
		color: #217346;
	}

</style>
