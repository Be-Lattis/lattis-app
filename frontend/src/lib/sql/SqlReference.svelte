<script lang="ts">
	// Panneau « Référence » de l'onglet SQL : syntaxe courante, fonctions de
	// SQLite et règles propres à Lattis, en sections repliables, avec
	// recherche. Un clic déplie une entrée ; Insérer place son exemple (aux
	// noms du classeur ouvert) dans l'éditeur. F1 dans l'éditeur ouvre
	// l'entrée du mot sous le curseur (focus). Données : reference.svelte.ts.
	import { tick, untrack } from 'svelte';
	import { resizePanel } from '#lib/ui/resize.js';
	import { panelKeys } from '#lib/ui/focus.js';
	import Icon from '#lib/icons/Icon.svelte';
	import PanelHeader from '#lib/ui/PanelHeader.svelte';
	import { adapt, reference, search, sectionLabel, SECTIONS, summary, title, type RefEntry, type Section } from './reference.svelte';
	import type { SqlObject } from './names';
	import { t } from '#lib/i18n/index.js';

	interface Props {
		objects: SqlObject[];
		/** Insère un exemple dans l'éditeur. */
		insert: (text: string) => void;
		close: () => void;
		/** Entrée à ouvrir (F1) ; seq change à chaque demande. */
		focus?: { id: string; seq: number } | null;
	}

	let { objects, insert, close, focus = null }: Props = $props();

	let query = $state('');
	let openId = $state<string | null>(null);
	let list = $state<HTMLDivElement>();

	const results = $derived(search(query));
	const searching = $derived(!!query.trim());
	const groups = $derived(
		SECTIONS.map((s) => ({ id: s, entries: (searching ? results : reference).filter((e) => e.section === s) })).filter((g) => g.entries.length > 0)
	);
	// Sections ouvertes à la main (pendant la recherche, toutes celles qui ont des résultats).
	let openSecs = $state<Set<Section>>(new Set(['syntax']));
	const isOpen = (id: Section) => searching || openSecs.has(id);
	function toggleSec(id: Section) {
		const s = new Set(openSecs);
		if (s.has(id)) s.delete(id);
		else s.add(id);
		openSecs = s;
	}

	// F1 : ouvre la section et l'entrée, et la montre.
	$effect(() => {
		const f = focus;
		if (!f) return;
		const e = reference.find((x) => x.id === f.id);
		if (!e) return;
		untrack(() => {
			query = '';
			openSecs = new Set([...openSecs, e.section]);
			openId = e.id;
		});
		tick().then(() => list?.querySelector(`[data-id="${e.id}"]`)?.scrollIntoView({ block: 'nearest' }));
	});

	function toggle(e: RefEntry) {
		openId = openId === e.id ? null : e.id;
	}
</script>

<aside class="ref" use:resizePanel={{ key: 'sql-ref' }} use:panelKeys={close} aria-label={t('sql.ref.title')}>
	<div class="head">
		<PanelHeader title={t('sql.ref.title')} help="sql" helpLabel={t('sql.ref.help')} onclose={close} />
	</div>
	<input
		class="search"
		type="search"
		placeholder={t('sql.ref.search')}
		bind:value={query}
		onkeydown={(e) => {
			if (e.key === 'Enter' && results[0]) openId = results[0].id;
			else if (e.key === 'Escape' && query) {
				query = ''; // Échap vide la recherche ; une seconde fois, ferme le volet
				e.preventDefault();
			}
		}}
		spellcheck="false"
	/>
	<div class="list" bind:this={list}>
		{#if searching}<p class="count">{t('sql.ref.results', { n: results.length })}</p>{/if}
		{#each groups as g (g.id)}
			<section class="cat">
				<button class="cat-head" aria-expanded={isOpen(g.id)} disabled={searching} onclick={() => toggleSec(g.id)}>
					<span class="chev" class:open={isOpen(g.id)}><Icon name="chevron-down" size={12} /></span>
					<span class="cat-label">{sectionLabel(g.id)}</span>
					<span class="cat-n">{g.entries.length}</span>
				</button>
				{#if isOpen(g.id)}
					{#each g.entries as e (e.id)}
						<div class="entry" class:open={openId === e.id} data-id={e.id}>
							<button class="title" onclick={() => toggle(e)} aria-expanded={openId === e.id}>
								<code>{title(e)}</code>
								<span class="summary">{summary(e)}</span>
							</button>
							{#if openId === e.id}
								{@const sql = adapt(e, objects)}
								<div class="body">
									<pre>{sql}</pre>
									<button class="insert" onclick={() => insert(sql)}><Icon name="paste" size={14} />{t('sql.ref.insert')}</button>
								</div>
							{/if}
						</div>
					{/each}
				{/if}
			</section>
		{/each}
		{#if searching && results.length === 0}<p class="muted">{t('sql.ref.noMatch')}</p>{/if}
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
		padding: 8px 10px 0;
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
	.count {
		font-size: 11px;
		color: #6b7280;
		margin: 4px 4px 2px;
	}
	.cat {
		border-bottom: 1px solid #eceef0;
	}
	.cat-head {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		border: none;
		background: none;
		font: inherit;
		padding: 7px 4px;
		cursor: pointer;
		color: #374151;
		text-align: left;
	}
	.cat-head:disabled {
		cursor: default;
		color: #374151;
	}
	.cat-head:hover:not(:disabled) {
		background: #eef0f2;
	}
	.cat-label {
		flex: 1;
		font-size: 12px;
		font-weight: 600;
	}
	.cat-n {
		font-size: 11px;
		color: #6b7280;
	}
	.chev {
		display: inline-flex;
		transform: rotate(-90deg);
		transition: transform 0.12s;
	}
	.chev.open {
		transform: none;
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
		color: #1d4ed8;
	}
	.summary {
		font-size: 12px;
		color: #4b5563;
	}
	.body {
		padding: 2px 8px 8px;
	}
	pre {
		margin: 4px 0 6px;
		padding: 6px 8px;
		background: #f7f8f9;
		border: 1px solid #e3e5e8;
		border-radius: 3px;
		font: 12px Consolas, 'Cascadia Mono', monospace;
		overflow-x: auto;
		white-space: pre;
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
