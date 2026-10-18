<script lang="ts">
	import { resizePanel } from '#lib/ui/resize.js';
	import { panelKeys } from '#lib/ui/focus.js';
	import Icon from '#lib/icons/Icon.svelte';
	// Panneau « Référence » de l'onglet SCRIPT : les fonctions disponibles dans
	// un script Lua, par catégorie, avec recherche. Un clic déplie une entrée ;
	// Insérer place son exemple dans l'éditeur. Données : reference.ts.
	import { categories, reference, search, type RefEntry } from './reference';

	interface Props {
		/** Insère un exemple dans l'éditeur, à la position du curseur. */
		insert: (text: string) => void;
		close: () => void;
	}

	let { insert, close }: Props = $props();

	let query = $state('');
	let openName = $state<string | null>(null);

	const results = $derived(search(query));
	const groups = $derived(
		query.trim()
			? [{ id: 'results', label: `${results.length} résultat(s)`, entries: results }]
			: categories.map((c) => ({ id: c.id, label: c.label, entries: reference.filter((e) => e.category === c.id) }))
	);

	function toggle(e: RefEntry) {
		openName = openName === e.name ? null : e.name;
	}
</script>

<aside class="ref" use:resizePanel={{ key: 'lua-ref' }} use:panelKeys={close} aria-label="Référence Lua">
	<div class="head">
		<h2>Référence Lua</h2>
		<button class="close" onclick={close} title="Masquer la référence" aria-label="Masquer la référence">×</button>
	</div>
	<input
		class="search"
		type="search"
		placeholder="Rechercher : query, date, majuscules…"
		bind:value={query}
		onkeydown={(e) => {
			if (e.key === 'Enter' && results[0]) openName = results[0].name;
			else if (e.key === 'Escape' && query) {
				query = ''; // Échap vide la recherche ; une seconde fois, ferme le volet
				e.preventDefault();
			}
		}}
		spellcheck="false"
	/>
	<div class="list">
		{#each groups as g (g.id)}
			<h3>{g.label}</h3>
			{#each g.entries as e (e.name)}
				<div class="entry" class:open={openName === e.name}>
					<button class="title" onclick={() => toggle(e)} aria-expanded={openName === e.name}>
						<code>{e.name}</code>
						<span class="summary">{e.summary}</span>
					</button>
					{#if openName === e.name}
						<div class="body">
							<code class="sig">{e.signature}</code>
							{#if e.deprecated}<p class="note">Ancien nom : préférez <code>{e.deprecated}</code>.</p>{/if}
							{#if e.details}<p>{e.details}</p>{/if}
							{#if e.example}
								<pre>{e.example}</pre>
								<button class="insert" onclick={() => insert(e.example!)}><Icon name="paste" size={14} />Insérer l’exemple</button>
							{/if}
						</div>
					{/if}
				</div>
			{/each}
		{/each}
		{#if query.trim() && results.length === 0}<p class="muted">Aucune fonction ne correspond.</p>{/if}
	</div>
</aside>

<style>
	.ref {
		flex-shrink: 0;
		display: flex;
		flex-direction: column;
		min-height: 0;
		border-left: 1px solid #d4d7db;
		background: #fafbfb;
	}
	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 6px 8px 0 12px;
	}
	h2 {
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #6b7280;
		margin: 4px 0;
	}
	.close {
		border: none;
		background: none;
		font-size: 18px;
		line-height: 1;
		color: #6b7280;
		cursor: pointer;
		padding: 2px 6px;
		border-radius: 3px;
	}
	.close:hover {
		background: #e3e5e8;
	}
	.search {
		margin: 6px 8px;
		font: inherit;
		padding: 5px 8px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		outline: none;
	}
	.search:focus {
		border-color: #217346;
	}
	.list {
		flex: 1;
		overflow: auto;
		padding: 0 8px 12px;
	}
	h3 {
		font-size: 11px;
		font-weight: 600;
		color: #6b7280;
		margin: 12px 4px 4px;
	}
	.entry {
		border-radius: 3px;
	}
	.entry.open {
		background: #fff;
		box-shadow: 0 0 0 1px #e3e5e8;
	}
	.title {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 1px;
		width: 100%;
		text-align: left;
		background: none;
		border: none;
		font: inherit;
		padding: 5px 6px;
		border-radius: 3px;
		cursor: pointer;
	}
	.title:hover {
		background: #e3efe7;
	}
	.title code {
		font: 600 12px Consolas, 'Cascadia Mono', monospace;
		color: #14532d;
	}
	.summary {
		font-size: 12px;
		color: #4b5563;
	}
	.body {
		padding: 2px 8px 8px;
		font-size: 12px;
		color: #1f2328;
	}
	.body p {
		margin: 6px 0;
		line-height: 1.4;
	}
	.sig {
		display: block;
		font: 12px Consolas, 'Cascadia Mono', monospace;
		background: #f3f4f6;
		padding: 4px 6px;
		border-radius: 3px;
		white-space: pre-wrap;
	}
	.note {
		color: #92400e;
	}
	pre {
		margin: 6px 0;
		padding: 6px 8px;
		background: #f7f8f9;
		border: 1px solid #e3e5e8;
		border-radius: 3px;
		font: 12px Consolas, 'Cascadia Mono', monospace;
		overflow-x: auto;
	}
	.insert {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		--icon-accent: #217346;
		font: inherit;
		font-size: 12px;
		padding: 3px 10px;
		border: 1px solid #217346;
		color: #217346;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.insert:hover {
		background: #e3efe7;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
		margin: 8px 4px;
	}
</style>
