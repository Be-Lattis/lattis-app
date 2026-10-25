<script lang="ts">
	// Bas de la barre Rechercher / remplacer : les options dépliées
	// (« Options »), comme le dialogue d'un tableur, et la liste de
	// Rechercher tout (feuille, tableau, cellule, valeur ; un clic y va).
	import { t } from '#lib/i18n/index.js';
	import { zoneLabel } from './findzone';
	import { listLabel, type FindOpts, type FoundCell, type Look } from './findopts';

	interface Props {
		opts: FindOpts;
		/** Options dépliées. */
		more: boolean;
		/** Remplacer ouvert : Valeurs agit sur les formules. */
		replace: boolean;
		/** Liste de Rechercher tout (null : pas demandée). */
		hits: FoundCell[] | null;
		total: number;
		busy: boolean;
		/** Indice de la cellule trouvée en cours (-1 : aucune). */
		current: number;
		onchange: () => void;
		onpick: (i: number) => void;
	}
	let { opts = $bindable(), more, replace, hits, total, busy, current, onchange, onpick }: Props = $props();

	// Au-delà, la liste n'affiche que les premières (Suivant les parcourt toutes).
	const LIST_MAX = 1000;
	const looks: Look[] = ['formulas', 'values', 'notes'];
	const shown = $derived(hits ? hits.slice(0, LIST_MAX) : []);
	const multi = $derived(!!hits && hits.some((h) => h.sheet !== hits[0].sheet));

	let list = $state<HTMLDivElement>();
	// La cellule en cours reste visible dans la liste.
	$effect(() => {
		const i = current;
		list?.querySelector<HTMLElement>(`[data-i="${i}"]`)?.scrollIntoView({ block: 'nearest' });
	});

	const set = <K extends keyof FindOpts>(k: K, v: FindOpts[K]) => {
		opts = { ...opts, [k]: v };
		onchange();
	};
</script>

{#if more}
	<div class="fm-opts" role="group" aria-label={t('grid.find.options.tooltip')}>
		<label for="fm-in">{t('grid.find.in')}</label>
		<select id="fm-in" value={opts.book ? 'book' : 'sheet'} onchange={(e) => set('book', e.currentTarget.value === 'book')}>
			<option value="sheet">{t('grid.find.in.sheet')}</option>
			<option value="book">{t('grid.find.in.book')}</option>
		</select>
		<label class="fm-check"><input type="checkbox" checked={opts.matchCase} onchange={(e) => set('matchCase', e.currentTarget.checked)} />{t('grid.find.case')}</label>
		<label for="fm-order">{t('grid.find.order')}</label>
		<select id="fm-order" value={opts.byCols ? 'cols' : 'rows'} onchange={(e) => set('byCols', e.currentTarget.value === 'cols')}>
			<option value="rows">{t('grid.find.order.rows')}</option>
			<option value="cols">{t('grid.find.order.cols')}</option>
		</select>
		<label class="fm-check"><input type="checkbox" checked={opts.whole} onchange={(e) => set('whole', e.currentTarget.checked)} />{t('grid.find.whole')}</label>
		<label for="fm-look">{t('grid.find.look')}</label>
		<select
			id="fm-look"
			value={opts.look}
			onchange={(e) => set('look', e.currentTarget.value as Look)}
			title={replace && opts.look === 'values' ? t('grid.find.look.values.replace') : undefined}
		>
			{#each looks as l (l)}
				<option value={l}>{t(`grid.find.look.${l}`)}</option>
			{/each}
		</select>
	</div>
{/if}
{#if hits}
	<div class="fm-list" bind:this={list} role="listbox" aria-label={t('grid.find.list.aria')} aria-busy={busy}>
		<div class="fm-head" aria-hidden="true">
			{#if multi || opts.book}<span>{t('grid.find.list.sheet')}</span>{/if}
			<span>{t('grid.find.list.table')}</span>
			<span>{t('grid.find.list.cell')}</span>
			<span>{t('grid.find.list.value')}</span>
		</div>
		{#each shown as h, i (i)}
			<button
				type="button"
				class="fm-row"
				class:on={i === current}
				class:book={multi || opts.book}
				role="option"
				aria-selected={i === current}
				data-i={i}
				onclick={() => onpick(i)}
			>
				{#if multi || opts.book}<span>{h.name}</span>{/if}
				<span>{h.table ?? ''}</span>
				<span>{zoneLabel({ r0: h.row, c0: h.col, r1: h.row, c1: h.col })}</span>
				<span class="fm-text">{h.text}</span>
			</button>
		{/each}
	</div>
	<div class="fm-count" role="status">{busy ? '…' : listLabel(shown.length, total)}</div>
{/if}

<style>
	.fm-opts {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr) auto;
		align-items: center;
		gap: 4px 8px;
		padding: 6px 0 2px 26px;
		border-top: 1px solid #eceef0;
	}
	.fm-opts label {
		color: #4b5563;
		white-space: nowrap;
	}
	.fm-opts select {
		font: inherit;
		height: 22px;
		min-width: 0;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		background: #fff;
		color: #111827;
	}
	.fm-opts select:focus-visible,
	.fm-check input:focus-visible,
	.fm-row:focus-visible {
		outline: 2px solid #217346;
		outline-offset: 1px;
	}
	.fm-check {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}
	.fm-check input {
		margin: 0;
		accent-color: #217346;
	}
	.fm-list {
		max-height: 180px;
		overflow: auto;
		border: 1px solid #e5e7eb;
		border-radius: 4px;
	}
	.fm-head,
	.fm-row {
		display: grid;
		grid-template-columns: 1fr 52px 2fr;
		gap: 8px;
		padding: 3px 8px;
		text-align: left;
	}
	.fm-head:has(span:nth-child(4)),
	.fm-row.book {
		grid-template-columns: 1fr 1fr 52px 2fr;
	}
	.fm-head {
		position: sticky;
		top: 0;
		background: #f9fafb;
		color: #6b7280;
		font-size: 11px;
		border-bottom: 1px solid #e5e7eb;
	}
	.fm-row {
		width: 100%;
		border: none;
		background: none;
		font: inherit;
		color: #1f2937;
		cursor: pointer;
	}
	.fm-row span {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.fm-row:hover {
		background: #eef6f0;
	}
	.fm-row.on {
		background: #d3e7dc;
	}
	.fm-count {
		color: #6b7280;
		font-size: 11px;
	}
</style>
