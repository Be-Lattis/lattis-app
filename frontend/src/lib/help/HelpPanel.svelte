<script lang="ts">
	// Volet « Aide » : table des matières, recherche et pages (help.ts). Les
	// pages Formules, Scripts Lua, Raccourcis et Paramètres se complètent de
	// listes tirées du code : catalogue des fonctions (DocumentService.
	// FormulaHelp), référence Lua, raccourcis, réglages. Précédent revient à la
	// page d'avant ; Échap efface la recherche, puis ferme le volet.
	import { tick, untrack } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { FunctionDoc } from '#lib/grid/autocomplete.js';
	import { categories as luaCategories, reference as luaReference } from '#lib/script/reference.js';
	import { settingSections } from '#lib/shell/settings.js';
	import { shortcutGroups } from '#lib/shell/shortcuts.js';
	import { resizePanel } from '#lib/ui/resize.js';
	import { focusRegion } from '#lib/ui/focus.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { HOME, buildIndex, findFunction, functionCategories, helpPages, loadHelpPages, pageById, parseTarget, searchHelp, targetPage } from './help';
	import { slug } from './markdown';
	import Markdown from './Markdown.svelte';
	import { closeHelp, helpState } from './state.svelte';
	import { reportIssue } from '#lib/shell/report.js';
	import { requestTour } from '#lib/onboarding/request.svelte.js';
	import { lang, t } from '#lib/i18n/index.js';

	// over : posée au-dessus d'une boîte modale, hors de la zone des volets.
	let { over = false }: { over?: boolean } = $props();

	let functions = $state<FunctionDoc[]>([]);
	let loaded = $state(false);
	DocumentService.FormulaHelp()
		.then((h) => (functions = h.functions ?? []))
		.catch(() => {})
		.finally(() => (loaded = true));

	let current = $state(HOME);
	let history = $state<string[]>([]);
	let query = $state('');
	let luaOpen = $state<string | null>(null);
	let contentEl = $state<HTMLElement>();
	let searchEl = $state<HTMLInputElement>();

	// Pages dans la langue de l'interface (chargées à la demande ; le français en attendant).
	let pages = $state(helpPages());
	$effect(() => {
		const l = lang();
		let live = true;
		loadHelpPages(l).then((p) => {
			if (live) pages = p;
		});
		return () => {
			live = false;
		};
	});

	const index = $derived(buildIndex(functions, pages));
	const hits = $derived(query.trim() ? searchHelp(index, query) : []);
	const target = $derived(parseTarget(current));
	const fn = $derived('fn' in target ? findFunction(functions, target.fn) : undefined);
	const shownPage = $derived(pageById('page' in target ? target.page : 'fn' in target && !fn && loaded ? 'formules' : targetPage(current), pages));
	const tocPage = $derived(targetPage(current));
	const groups = $derived(functionCategories(functions));

	/** Va à une cible ; back : sans l'ajouter à l'historique. */
	async function go(t: string, back = false) {
		if (!back && t !== current) history = [...history.slice(-49), current];
		current = t;
		helpState.last = t;
		query = '';
		const x = parseTarget(t);
		if ('lua' in x) luaOpen = x.lua;
		await tick();
		const id = 'page' in x ? x.anchor && `aide-${x.anchor}` : 'lua' in x ? `aide-lua-${x.lua}` : '';
		const el = id ? contentEl?.querySelector(`#${CSS.escape(id)}`) : null;
		if (el) el.scrollIntoView({ block: 'start' });
		else contentEl?.scrollTo(0, 0);
	}

	function back() {
		const prev = history[history.length - 1];
		if (prev === undefined) return;
		history = history.slice(0, -1);
		go(prev, true);
	}

	// Élément qui avait le focus à l'ouverture : il le retrouve à la fermeture.
	let returnTo: HTMLElement | null = null;

	// Ouverture (F1, menus, « ? » des volets) : la cible demandée.
	$effect(() => {
		void helpState.seq;
		const t = helpState.target;
		const focus = helpState.focus;
		untrack(() => {
			const a = document.activeElement;
			if (a instanceof HTMLElement && a !== document.body && !a.closest('.help')) returnTo = a;
			go(t);
			if (focus) tick().then(() => searchEl?.focus());
		});
	});

	function close() {
		const to = returnTo;
		const had = !!document.activeElement?.closest('.help');
		closeHelp();
		if (!had) return;
		requestAnimationFrame(() => {
			if (to?.isConnected) to.focus({ preventScroll: true });
			else focusRegion('grid');
		});
	}

	// Seuls Échap et F1 s'arrêtent à l'aide : Ctrl+S, Ctrl+Z… gardent leur effet.
	function onKey(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.stopPropagation();
			if (query) query = '';
			else close();
		} else if (e.key === 'F1') {
			e.stopPropagation();
			e.preventDefault();
		}
	}
</script>

{#snippet fnLink(f: FunctionDoc)}
	<button class="link" onclick={() => go(`fonction:${f.name}`)}>{f.name}</button>
{/snippet}

<!-- svelte-ignore a11y_no_noninteractive_element_interactions (touches du volet captées à son niveau ; ses commandes sont des boutons) -->
<aside class="help" class:over data-modal-exempt={over ? '' : undefined} use:resizePanel={{ key: 'help', min: 420 }} aria-label={t('help.title')} onkeydown={onKey}>
	<header>
		<button class="icon" title={t('help.back')} aria-label={t('help.back')} disabled={!history.length} onclick={back}>←</button>
		<Icon name="book" size={16} />
		<strong>{t('help.title')}</strong>
		<button class="link report" title={t('onboarding.startTooltip')} onclick={() => (close(), requestTour())}>{t('onboarding.help')}</button>
		<button class="link report" title={t('help.report.tooltip')} onclick={reportIssue}>{t('help.report.label')}</button>
		<button class="icon close" title={t('help.close.tooltip')} aria-label={t('help.close.aria')} onclick={close}>✕</button>
	</header>
	<div class="search">
		<Icon name="search" size={14} />
		<input
			bind:this={searchEl}
			bind:value={query}
			type="search"
			placeholder={t('help.search.placeholder')}
			aria-label={t('help.search.aria')}
			spellcheck="false"
			onkeydown={(e) => {
				if (e.key === 'Enter' && hits[0]) go(hits[0].target);
			}}
		/>
	</div>
	<div class="body">
		<nav class="toc" aria-label={t('help.toc')}>
			{#each pages as p (p.id)}
				<button class:on={!query.trim() && tocPage === p.id} aria-current={tocPage === p.id ? 'page' : undefined} onclick={() => go(p.id)}>{p.title}</button>
			{/each}
		</nav>
		<div class="content" bind:this={contentEl}>
			{#if query.trim()}
				<p class="muted">{hits.length ? (hits.length >= 50 ? t('help.results.first', { n: hits.length }) : t('help.results.count', { n: hits.length })) : t('help.results.none')}</p>
				{#each hits as h (h.target + h.title + h.where)}
					<button class="hit" onclick={() => go(h.target)}>
						<span class="hit-title">{h.title}</span>
						<span class="where">{h.where}</span>
						{#if h.snippet}<span class="snippet">{h.snippet}</span>{/if}
					</button>
				{/each}
			{:else if 'fn' in target && fn}
				{@const same = groups.find((g) => g.name === fn.category)?.functions.filter((f) => f !== fn) ?? []}
				<h1>{fn.name}</h1>
				<p class="sub">
					{#if fn.other !== fn.name}{t('help.fn.english')} <code>{fn.other}</code>{' · '}{/if}{t('help.fn.category')}{' '}<button class="link" onclick={() => go(`formules#${slug(fn.category ?? '')}`)}>{fn.category}</button>
				</p>
				{#if fn.description}<p>{fn.description}</p>{/if}
				<pre>{fn.signature ?? fn.name}</pre>
				{#if fn.args?.length}
					<h2>{t('help.fn.arguments')}</h2>
					<table>
						<tbody>
							{#each fn.args as a, i (i)}
								<tr>
									<td><code>{a.name}</code>{#if a.optional}<span class="muted">{` ${t('help.fn.optional')}`}</span>{/if}{#if fn.repeat && i >= fn.args.length - fn.repeat}<span class="muted">{` ${t('help.fn.repeatable')}`}</span>{/if}</td>
									<td>{a.desc}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
				{#if fn.example}
					<h2>{t('help.fn.example')}</h2>
					<pre>{fn.example}</pre>
				{/if}
				{#if same.length}
					<h2>{t('help.fn.seeAlso')}</h2>
					<p class="see">{#each same as f, i (f.name)}{#if i}{', '}{/if}{@render fnLink(f)}{/each}</p>
				{/if}
			{:else if 'fn' in target && !loaded}
				<p class="muted">{t('help.fn.loading')}</p>
			{:else if shownPage}
				{#if 'fn' in target}<p class="note">{t('help.fn.unknown', { name: target.fn })}</p>{/if}
				<Markdown blocks={shownPage.blocks} {go} />
				{#if shownPage.id === 'formules'}
					{#each groups as g (g.name)}
						<h3 id="aide-{slug(g.name)}">{g.name}</h3>
						<ul class="defs">
							{#each g.functions as f (f.name)}
								<li>{@render fnLink(f)}{' '}<span>{f.description}</span></li>
							{/each}
						</ul>
					{:else}
						<p class="muted">{loaded ? t('help.fn.unavailable') : t('ai.panel.loading')}</p>
					{/each}
				{:else if shownPage.id === 'lua'}
					{#each luaCategories as c (c.id)}
						<h3>{c.label}</h3>
						{#each luaReference.filter((e) => e.category === c.id) as e (e.name)}
							<div class="entry" id="aide-lua-{e.name}" class:open={luaOpen === e.name}>
								<button class="entry-title" aria-expanded={luaOpen === e.name} onclick={() => (luaOpen = luaOpen === e.name ? null : e.name)}>
									<code>{e.name}</code>
									<span>{e.summary}</span>
								</button>
								{#if luaOpen === e.name}
									<pre>{e.signature}</pre>
									{#if e.deprecated}<p class="note">{t('script.ref.deprecated')} <code>{e.deprecated}</code>.</p>{/if}
									{#if e.details}<p>{e.details}</p>{/if}
									{#if e.example}<pre>{e.example}</pre>{/if}
								{/if}
							</div>
						{/each}
					{/each}
				{:else if shownPage.id === 'raccourcis'}
					{#each shortcutGroups as g (g.id)}
						<h2 id="aide-{slug(g.title)}">{g.title}</h2>
						<table>
							<tbody>
								{#each g.items as s, i (i)}
									<tr><td class="keys"><code>{s.keys}</code></td><td>{s.label}</td></tr>
								{/each}
							</tbody>
						</table>
					{/each}
				{:else if shownPage.id === 'parametres'}
					{#each settingSections as sec (sec.id)}
						<h3>{sec.title}</h3>
						<ul class="defs">
							{#each sec.settings as s (s.path)}
								<li><strong>{s.label}</strong>{#if s.help}<span>{` : ${s.help}`}</span>{/if}</li>
							{/each}
						</ul>
					{/each}
				{/if}
			{/if}
		</div>
	</div>
</aside>

<style>
	.help {
		position: relative;
		display: flex;
		flex-direction: column;
		min-height: 0;
		border-left: 1px solid #d4d7db;
		background: #fafbfb;
		box-shadow: -4px 0 12px rgba(0, 0, 0, 0.08);
		font-size: 13px;
		--icon-accent: #217346;
	}
	/* Ouverte depuis une boîte modale : au-dessus de toute boîte, contre le bord droit. */
	.help.over {
		position: fixed;
		top: 0;
		right: 0;
		bottom: 0;
		z-index: var(--z-help);
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	header {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 8px;
		border-bottom: 1px solid #e3e5e8;
	}
	header strong {
		flex: 1;
	}
	.report {
		border: none;
		background: none;
		padding: 2px 4px;
		color: #217346;
		font: inherit;
		font-size: 12px;
		cursor: pointer;
	}
	.report:hover {
		text-decoration: underline;
	}
	.icon {
		border: none;
		background: none;
		font: inherit;
		font-size: 15px;
		line-height: 1;
		padding: 3px 7px;
		border-radius: 3px;
		cursor: pointer;
		color: #374151;
	}
	.icon:hover:not(:disabled) {
		background: #e3e5e8;
	}
	.icon:disabled {
		color: #c4c8cd;
		cursor: default;
	}
	.search {
		display: flex;
		align-items: center;
		gap: 6px;
		margin: 8px;
		padding: 0 8px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		background: #fff;
		color: #6b7280;
	}
	.search:focus-within {
		border-color: #217346;
	}
	.search input {
		flex: 1;
		min-width: 0;
		border: none;
		outline: none;
		font: inherit;
		padding: 5px 0;
	}
	.body {
		flex: 1;
		display: flex;
		min-height: 0;
	}
	.toc {
		flex: 0 0 170px;
		overflow: auto;
		padding: 0 4px 12px 8px;
		display: flex;
		flex-direction: column;
		gap: 1px;
	}
	.toc button {
		text-align: left;
		border: none;
		background: none;
		font: inherit;
		padding: 5px 8px;
		border-radius: 3px;
		cursor: pointer;
		color: #1f2328;
	}
	.toc button:hover {
		background: #e3efe7;
	}
	.toc button.on {
		background: #d5e8dc;
		color: #14532d;
		font-weight: 600;
	}
	.content {
		flex: 1;
		min-width: 0;
		overflow: auto;
		padding: 0 16px 24px 12px;
		line-height: 1.5;
		color: #1f2328;
	}
	.content :global(h1) {
		font-size: 20px;
		margin: 6px 0 10px;
		color: #14532d;
	}
	.content :global(h2) {
		font-size: 15px;
		margin: 18px 0 6px;
		padding-bottom: 3px;
		border-bottom: 1px solid #e3e5e8;
	}
	.content :global(h3) {
		font-size: 13px;
		margin: 14px 0 4px;
		color: #374151;
	}
	.content :global(p) {
		margin: 6px 0;
	}
	.content :global(ul),
	.content :global(ol) {
		margin: 6px 0;
		padding-left: 22px;
	}
	.content :global(code) {
		font: 12px Consolas, 'Cascadia Mono', monospace;
		background: #eef0f2;
		padding: 0 3px;
		border-radius: 3px;
	}
	.content :global(pre) {
		margin: 6px 0;
		padding: 6px 8px;
		background: #f3f4f6;
		border: 1px solid #e3e5e8;
		border-radius: 3px;
		font: 12px Consolas, 'Cascadia Mono', monospace;
		white-space: pre-wrap;
		overflow-x: auto;
	}
	.content :global(table) {
		border-collapse: collapse;
		margin: 6px 0;
		width: 100%;
	}
	.content :global(th),
	.content :global(td) {
		border: 1px solid #e3e5e8;
		padding: 3px 6px;
		text-align: left;
		vertical-align: top;
	}
	.content :global(th) {
		background: #f3f4f6;
		font-weight: 600;
	}
	.content :global(.note) {
		border-left: 3px solid #217346;
		background: #eef6f1;
		padding: 4px 8px;
	}
	.content :global(.link) {
		border: none;
		background: none;
		padding: 0;
		font: inherit;
		color: #1a6b3e;
		text-decoration: underline;
		cursor: pointer;
	}
	.keys {
		white-space: nowrap;
		width: 1%;
	}
	.sub {
		color: #4b5563;
	}
	.defs {
		list-style: none;
		padding-left: 0 !important;
	}
	.defs li {
		margin: 3px 0;
	}
	.defs span {
		color: #4b5563;
	}
	.muted {
		color: #6b7280;
	}
	.hit {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 1px;
		width: 100%;
		text-align: left;
		border: none;
		background: none;
		font: inherit;
		padding: 6px 8px;
		border-radius: 3px;
		cursor: pointer;
	}
	.hit:hover {
		background: #e3efe7;
	}
	.hit-title {
		font-weight: 600;
		color: #14532d;
	}
	.where {
		font-size: 11px;
		color: #6b7280;
	}
	.snippet {
		font-size: 12px;
		color: #4b5563;
	}
	.entry {
		border-radius: 3px;
		padding: 0 4px;
	}
	.entry.open {
		background: #fff;
		box-shadow: 0 0 0 1px #e3e5e8;
		padding-bottom: 6px;
	}
	.entry-title {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		width: 100%;
		text-align: left;
		border: none;
		background: none;
		font: inherit;
		padding: 4px 2px;
		cursor: pointer;
	}
	.entry-title span {
		font-size: 12px;
		color: #4b5563;
	}
</style>
