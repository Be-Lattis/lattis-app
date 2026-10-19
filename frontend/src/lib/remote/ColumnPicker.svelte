<script lang="ts">
	// Colonnes d'un tableau à envoyer ou à exposer (boîtes de partage au
	// Data Lake) : liste compacte à cocher, avec le type de chaque colonne,
	// la clé marquée, Tout / Aucune, et une recherche quand il y en a
	// beaucoup.
	import Icon from '#lib/icons/Icon.svelte';
	import type { IconName } from '#lib/icons/icons.js';
	import { typeLabel } from './data';

	interface Props {
		columns: { name: string; type?: string }[];
		/** Colonnes cochées. */
		selected: ReadonlySet<string>;
		onchange: (next: Set<string>) => void;
		/** Colonnes marquées d'une clé (clé du tableau, ou de la mise à jour par clé). */
		keys?: readonly string[];
		disabled?: boolean;
		/** Message sous la liste (aucune colonne cochée…). */
		error?: string;
		/** Libellé de la liste pour les lecteurs d'écran. */
		label?: string;
	}
	let { columns, selected, onchange, keys = [], disabled = false, error = '', label = 'Colonnes' }: Props = $props();

	let query = $state('');
	const q = $derived(query.trim().toLowerCase());
	const shown = $derived(q ? columns.filter((c) => c.name.toLowerCase().includes(q)) : columns);
	const count = $derived(columns.filter((c) => selected.has(c.name)).length);

	const icon = (t: string | undefined): IconName => (t === 'date' || t === 'timestamp' ? 'type-date' : t === 'boolean' ? 'type-boolean' : t === 'integer' || t === 'decimal' || t === 'number' ? 'type-number' : 'type-text');

	function set(name: string, on: boolean) {
		const s = new Set(selected);
		if (on) s.add(name);
		else s.delete(name);
		onchange(s);
	}

	function all(on: boolean) {
		const s = new Set(selected);
		for (const c of shown) {
			if (on) s.add(c.name);
			else s.delete(c.name);
		}
		onchange(s);
	}
</script>

<div class="cols" class:invalid={!!error}>
	<div class="bar">
		<span class="count" aria-live="polite">{count} sur {columns.length}</span>
		<button type="button" class="link" disabled={disabled || shown.every((c) => selected.has(c.name))} onclick={() => all(true)}>Tout</button>
		<button type="button" class="link" disabled={disabled || shown.every((c) => !selected.has(c.name))} onclick={() => all(false)}>Aucune</button>
		{#if columns.length > 10}
			<span class="search">
				<Icon name="search" size={13} />
				<input bind:value={query} placeholder="Chercher" aria-label="Chercher une colonne" {disabled} />
			</span>
		{/if}
	</div>
	<div class="list" role="group" aria-label={label}>
		{#each shown as c (c.name)}
			<label class="col" class:off={!selected.has(c.name)} title="{c.name} · {typeLabel(c.type)}{keys.includes(c.name) ? ' · clé' : ''}">
				<input type="checkbox" checked={selected.has(c.name)} {disabled} onchange={(e) => set(c.name, e.currentTarget.checked)} />
				<span class="ty" aria-hidden="true"><Icon name={icon(c.type)} size={14} /></span>
				<span class="name">{c.name}</span>
				{#if keys.includes(c.name)}<span class="key" aria-label="clé"><Icon name="key" size={13} /></span>{/if}
			</label>
		{:else}
			<p class="none">{q ? 'Aucune colonne ne correspond.' : 'Ce tableau n’a pas de colonne.'}</p>
		{/each}
	</div>
	{#if error}<p class="error" role="alert">{error}</p>{/if}
</div>

<style>
	.cols {
		min-width: 0;
	}
	.bar {
		display: flex;
		align-items: center;
		gap: 4px;
		margin-bottom: 4px;
		font-size: 12px;
	}
	.count {
		color: #6b7280;
		margin-right: 6px;
	}
	.search {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		margin-left: auto;
		color: #6b7280;
		border: 1px solid #d4d7db;
		border-radius: 4px;
		padding: 0 6px;
		height: 22px;
	}
	.search input {
		border: none;
		outline: none;
		font: inherit;
		width: 120px;
		color: #1f2328;
		background: transparent;
	}
	.list {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
		gap: 1px 8px;
		max-height: 150px;
		overflow: auto;
		border: 1px solid #e3e5e8;
		border-radius: 4px;
		padding: 4px 6px;
		background: #fff;
	}
	.invalid .list {
		border-color: #b91c1c;
	}
	.col {
		display: flex;
		align-items: center;
		gap: 5px;
		min-width: 0;
		height: 24px;
		padding: 0 4px;
		border-radius: 3px;
		cursor: pointer;
		--icon-accent: #217346;
	}
	.col:hover {
		background: #f3f8f5;
	}
	.col input {
		margin: 0;
		accent-color: #217346;
	}
	.col.off .name,
	.col.off .ty {
		color: #9ca3af;
	}
	.ty {
		display: inline-flex;
		color: #6b7280;
		flex: none;
	}
	.name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.key {
		display: inline-flex;
		color: #b45309;
		flex: none;
	}
	.none {
		margin: 4px 0;
		color: #6b7280;
		grid-column: 1 / -1;
	}
	.link {
		border: none;
		background: none;
		color: #217346;
		cursor: pointer;
		padding: 0 4px;
		font: inherit;
	}
	.link:hover:not(:disabled) {
		text-decoration: underline;
	}
	.link:disabled {
		color: #9ca3af;
		cursor: default;
	}
	.error {
		margin: 4px 0 0;
		font-size: 12px;
		color: #b91c1c;
	}
</style>
