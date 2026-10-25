<script lang="ts">
	// Boîte Sous-total (Données › Sous-total), comme un tableur : à chaque
	// changement d'une colonne, une fonction sur les colonnes choisies. Le
	// résultat va sur une feuille à côté, avec son plan 1 2 3 ; le tableau
	// ne change pas (canvas/subtotal.go).
	import { untrack } from 'svelte';
	import { backdrop } from '#lib/ui/backdrop.js';
	import { modal } from '#lib/ui/focus.js';
	import { errorText, t, tAny } from '#lib/i18n/index.js';
	import { SUBTOTAL_FUNCS, subtotalDefaults, type SubtotalOptions, type SubtotalTable } from './subtotal';

	interface Props {
		tables: SubtotalTable[];
		/** Tableau et colonne de la cellule active. */
		table?: string;
		column?: number;
		onok: (o: SubtotalOptions) => unknown;
		oncancel: () => void;
	}
	let { tables, table, column, onok, oncancel }: Props = $props();

	const start = untrack(() => subtotalDefaults(tables, table, column));
	let name = $state(start.table);
	let by = $state(start.by);
	let fn = $state(start.func);
	let totals = $state<string[]>(start.columns);
	let replace = $state(true);
	let breaks = $state(false);
	let below = $state(true);
	let failed = $state('');
	let busy = $state(false);

	const cur = $derived(tables.find((x) => x.name === name) ?? tables[0]);

	function pickTable(n: string) {
		name = n;
		const d = subtotalDefaults(tables, n);
		by = d.by;
		totals = d.columns;
	}

	function toggle(c: string, on: boolean) {
		totals = on ? [...totals, c] : totals.filter((x) => x !== c);
	}

	async function ok() {
		const columns = cur.columns.filter((c) => totals.includes(c) && c !== by);
		if (!columns.length) {
			failed = t('grid.subtotal.noColumn');
			return;
		}
		busy = true;
		try {
			await onok({ table: cur.name, by, func: fn, columns, replace, pageBreaks: breaks, below });
		} catch (e) {
			failed = errorText(e);
		} finally {
			busy = false;
		}
	}

	function keys(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') oncancel();
		else if (e.key === 'Enter' && !(e.target instanceof HTMLSelectElement)) {
			e.preventDefault();
			void ok();
		}
	}
</script>

<div class="st-back" role="presentation" use:backdrop={oncancel}>
	<div class="st" role="dialog" aria-modal="true" aria-label={t('grid.subtotal.title')} tabindex="-1" use:modal onkeydown={keys}>
		<h2>{t('grid.subtotal.title')}</h2>
		{#if tables.length > 1}
			<label>
				<span>{t('grid.subtotal.table')}</span>
				<select value={name} onchange={(e) => pickTable(e.currentTarget.value)}>
					{#each tables as x (x.name)}<option value={x.name}>{x.name}</option>{/each}
				</select>
			</label>
		{/if}
		<label>
			<span>{t('grid.subtotal.by')}</span>
			<select bind:value={by} data-autofocus>
				{#each cur.columns as c (c)}<option value={c}>{c}</option>{/each}
			</select>
		</label>
		<label>
			<span>{t('grid.subtotal.func')}</span>
			<select bind:value={fn}>
				{#each SUBTOTAL_FUNCS as f (f.code)}<option value={f.code}>{tAny(f.label)}</option>{/each}
			</select>
		</label>
		<fieldset>
			<legend>{t('grid.subtotal.columns')}</legend>
			<div class="cols">
				{#each cur.columns as c (c)}
					<label class="check"><input type="checkbox" checked={totals.includes(c)} disabled={c === by} onchange={(e) => toggle(c, e.currentTarget.checked)} />{c}</label>
				{/each}
			</div>
		</fieldset>
		<label class="check"><input type="checkbox" bind:checked={replace} />{t('grid.subtotal.replace')}</label>
		<label class="check"><input type="checkbox" bind:checked={breaks} />{t('grid.subtotal.breaks')}</label>
		<label class="check"><input type="checkbox" bind:checked={below} />{t('grid.subtotal.below')}</label>
		<p class="note">{t('grid.subtotal.note')}</p>
		{#if failed}<div class="error" role="alert">{failed}</div>{/if}
		<div class="actions">
			<button class="primary" disabled={busy} onclick={ok}>{t('common.ok')}</button>
			<button onclick={oncancel}>{t('common.cancel')}</button>
		</div>
	</div>
</div>

<style>
	.st-back {
		position: fixed;
		inset: 0;
		z-index: 40;
		background: rgba(0, 0, 0, 0.2);
		display: flex;
		align-items: center;
		justify-content: center;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.st {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius, 6px);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		padding: 16px 18px;
		width: min(360px, calc(100vw - 32px));
		max-height: calc(100vh - 32px);
		overflow: auto;
		display: flex;
		flex-direction: column;
		gap: 8px;
		outline: none;
		box-sizing: border-box;
	}
	h2 {
		font-size: 15px;
		font-weight: 600;
		margin: 0;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 3px;
	}
	label span,
	legend {
		color: #4b5563;
		font-size: 12px;
	}
	label.check {
		flex-direction: row;
		align-items: center;
		gap: 6px;
	}
	select {
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #d4d7db;
		width: 100%;
		box-sizing: border-box;
		background: #fff;
	}
	fieldset {
		border: 1px solid #d4d7db;
		border-radius: 3px;
		margin: 0;
		padding: 4px 8px 6px;
	}
	.cols {
		max-height: 132px;
		overflow: auto;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.note {
		margin: 0;
		color: #6b7280;
		font-size: 12px;
	}
	.error {
		color: #c42b1c;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}
	.actions button {
		font: inherit;
		padding: 4px 16px;
		border: 1px solid #d4d7db;
		background: #fff;
		border-radius: 2px;
		cursor: pointer;
	}
	.actions button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	.actions button:disabled {
		opacity: 0.5;
		cursor: default;
	}
</style>
