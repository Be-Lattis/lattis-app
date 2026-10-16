<script lang="ts">
	// Onglet SQL : liste des objets, éditeur avec les noms affichés (traduits en
	// noms internes t_1, c_0… avant l'exécution), historique des requêtes.
	// Requêtes en lecture seule.
	import { formatValue, typeLabel } from '#lib/grid/format.js';
	import SqlEditor from '#lib/sql/SqlEditor.svelte';
	import { Catalog, displayHeaders, quote, translate, type SqlObject } from '#lib/sql/names.js';
	import { load, record, save, search, when, type HistoryEntry } from '#lib/sql/history.js';

	interface Result {
		columns: string[] | null;
		rows: (unknown[] | null)[] | null;
		truncated: boolean;
		micros: number;
	}
	interface Props {
		objects: SqlObject[];
		run: (sql: string) => Promise<Result>;
		ongoto: (table: string) => void;
	}

	let { objects, run, ongoto }: Props = $props();

	const catalog = $derived(new Catalog(objects));
	let panel = $state<'objects' | 'history'>('objects');
	let open = $state<string | null>(null);
	let sql = $state('');
	let result = $state<Result | null>(null);
	let headers = $state<string[]>([]);
	let sent = $state('');
	let error = $state('');
	let running = $state(false);
	let editor = $state<ReturnType<typeof SqlEditor>>();

	let entries = $state<HistoryEntry[]>(load());
	let filter = $state('');
	const shown = $derived(search(entries, filter));
	// Position dans l'historique pour Alt+↑ / Alt+↓ (-1 : brouillon en cours).
	let cursor = -1;
	let draft = '';

	$effect(() => {
		if (!sql && objects.length > 0) {
			const o = objects[0];
			sql = `SELECT ${o.columns.slice(0, 4).map((c) => quote(c.name)).join(', ')}\nFROM ${quote(o.name)}\nLIMIT 100`;
		}
	});

	async function execute(text: string) {
		if (!text.trim() || running) return;
		running = true;
		error = '';
		const at = Date.now();
		try {
			const t = translate(text, catalog);
			sent = t.sql;
			const r = await run(t.sql);
			result = r;
			headers = displayHeaders(r.columns ?? [], t.scope);
			remember({ sql: text, at, rows: (r.rows ?? []).length, micros: r.micros });
		} catch (e) {
			result = null;
			error = String(e instanceof Error ? e.message : e);
			remember({ sql: text, at, error });
		}
		running = false;
	}

	function remember(e: HistoryEntry) {
		entries = record(entries, e);
		save(entries);
		cursor = -1;
	}

	function step(dir: 1 | -1): string | null {
		if (entries.length === 0) return null;
		if (cursor === -1) draft = sql;
		const next = Math.max(-1, Math.min(entries.length - 1, cursor + dir));
		if (next === cursor) return null;
		cursor = next;
		return next === -1 ? draft : entries[next].sql;
	}

	function useEntry(e: HistoryEntry, now: boolean) {
		sql = e.sql;
		cursor = -1;
		if (now) execute(e.sql);
		else editor?.focus();
	}

	function clearHistory() {
		entries = [];
		save(entries);
	}

	const cell = (v: unknown) => (v === null || v === undefined ? '' : typeof v === 'number' ? formatValue('decimal', v).replace(/,00$/, '') : String(v));
</script>

<div class="sql">
	<aside class="side">
		<div class="tabs" role="tablist">
			<button role="tab" aria-selected={panel === 'objects'} class:on={panel === 'objects'} onclick={() => (panel = 'objects')}>Objets</button>
			<button role="tab" aria-selected={panel === 'history'} class:on={panel === 'history'} onclick={() => (panel = 'history')}>
				Historique{entries.length ? ` (${entries.length})` : ''}
			</button>
		</div>
		{#if panel === 'objects'}
			<div class="list">
				{#each objects as o (o.sqlName)}
					<div class="obj">
						<button class="obj-head" onclick={() => (open = open === o.sqlName ? null : o.sqlName)}>
							<span>{open === o.sqlName ? '▾' : '▸'}</span>
							<strong>{o.name}</strong>
							<span class="muted">tableau · {o.sqlName} · {o.rows.toLocaleString('fr-FR')} lignes</span>
						</button>
						{#if open === o.sqlName}
							<ul>
								{#each o.columns as c (c.sql)}
									<li>
										<button class="col" title="Insérer dans la requête" onclick={() => editor?.insert(quote(c.name))}>{c.name}</button>
										<span class="muted">{c.sql} · {typeLabel(c.type)}</span>
									</li>
								{/each}
							</ul>
							<div class="actions">
								<button class="link" onclick={() => (sql = `SELECT *\nFROM ${quote(o.name)}\nLIMIT 100`)}>Interroger</button>
								<button class="link" onclick={() => editor?.insert(quote(o.name))}>Insérer le nom</button>
								<button class="link" onclick={() => ongoto(o.name)}>Afficher sur la feuille</button>
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{:else}
			<div class="hist-tools">
				<input type="search" placeholder="Rechercher" bind:value={filter} />
				{#if entries.length}<button class="link" onclick={clearHistory}>Effacer</button>{/if}
			</div>
			<div class="list">
				{#each shown as e (e.at + e.sql)}
					<div class="entry" class:failed={!!e.error}>
						<button class="entry-sql" title="Reprendre dans l'éditeur (double-clic : exécuter)" onclick={() => useEntry(e, false)} ondblclick={() => useEntry(e, true)}>{e.sql}</button>
						<div class="entry-foot">
							<span class="muted">{when(e.at)} · {e.error ? 'erreur' : `${(e.rows ?? 0).toLocaleString('fr-FR')} ligne(s)`}</span>
							<button class="link" title="Exécuter" onclick={() => useEntry(e, true)}>▶</button>
						</div>
					</div>
				{:else}
					<p class="muted empty">{entries.length ? 'Aucune requête ne correspond.' : 'Les requêtes exécutées apparaîtront ici.'}</p>
				{/each}
			</div>
		{/if}
	</aside>
	<section class="main">
		<div class="toolbar">
			<button class="run" onclick={() => editor?.runCurrent()} disabled={running}>▶ Exécuter</button>
			<span class="muted hint" title="Les instructions sont séparées par « ; » ou une ligne vide. Lecture seule, 1 000 lignes au plus.">Ctrl+Entrée : exécuter l'instruction sous le curseur · Ctrl+Espace : noms · Alt+↑↓ : historique</span>
		</div>
		<div class="code">
			<SqlEditor bind:this={editor} bind:value={sql} {catalog} onrun={execute} onhistory={step} />
		</div>
		<div class="result">
			{#if error}
				<p class="error">{error}</p>
			{:else if result}
				<p class="muted">
					{(result.rows ?? []).length.toLocaleString('fr-FR')} ligne(s){result.truncated ? ' (tronqué)' : ''} · {(result.micros / 1000).toFixed(1)} ms
					<span class="sent" title={sent}>· SQL exécuté</span>
				</p>
				<div class="table-wrap">
					<table>
						<thead>
							<tr>{#each headers as c, i (i)}<th>{c}</th>{/each}</tr>
						</thead>
						<tbody>
							{#each result.rows ?? [] as r, i (i)}
								<tr>{#each r ?? [] as v, j (j)}<td class:num={typeof v === 'number'}>{cell(v)}</td>{/each}</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	</section>
</div>

<style>
	.sql {
		display: flex;
		height: 100%;
		min-height: 0;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.side {
		width: 300px;
		border-right: 1px solid #d4d7db;
		background: #fafbfb;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}
	.tabs {
		display: flex;
		border-bottom: 1px solid #e3e5e8;
		padding: 0 6px;
	}
	.tabs button {
		font: inherit;
		font-size: 12px;
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		padding: 8px 8px 6px;
		color: #6b7280;
		cursor: pointer;
	}
	.tabs button.on {
		color: #217346;
		border-bottom-color: #217346;
		font-weight: 600;
	}
	.list {
		flex: 1;
		overflow: auto;
		padding: 8px;
	}
	.hist-tools {
		display: flex;
		gap: 8px;
		align-items: center;
		padding: 8px 8px 0;
	}
	.hist-tools input {
		flex: 1;
		font: inherit;
		border: 1px solid #d4d7db;
		border-radius: 3px;
		padding: 3px 6px;
	}
	.entry {
		border: 1px solid #e3e5e8;
		border-left: 3px solid #217346;
		border-radius: 3px;
		background: #fff;
		margin-bottom: 6px;
	}
	.entry.failed {
		border-left-color: #b42318;
	}
	.entry-sql {
		display: block;
		width: 100%;
		text-align: left;
		background: none;
		border: none;
		padding: 4px 6px;
		font: 12px Consolas, 'Cascadia Mono', monospace;
		white-space: pre-wrap;
		word-break: break-word;
		max-height: 4.5em;
		overflow: hidden;
		cursor: pointer;
	}
	.entry-sql:hover {
		background: #f5f9f6;
	}
	.entry-foot {
		display: flex;
		justify-content: space-between;
		padding: 0 6px 2px;
	}
	.empty {
		margin: 8px 4px;
	}
	.col {
		background: none;
		border: none;
		font: inherit;
		padding: 0;
		cursor: copy;
		text-align: left;
	}
	.col:hover {
		color: #217346;
		text-decoration: underline;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0 12px;
		margin-left: 22px;
	}
	.sent {
		cursor: help;
		text-decoration: underline dotted;
	}
	.obj-head {
		display: flex;
		gap: 6px;
		align-items: baseline;
		flex-wrap: wrap;
		width: 100%;
		text-align: left;
		background: none;
		border: none;
		font: inherit;
		padding: 4px;
		cursor: pointer;
		border-radius: 3px;
	}
	.obj-head:hover {
		background: #eef2ef;
	}
	ul {
		list-style: none;
		margin: 0 0 4px 22px;
		padding: 0;
	}
	li {
		display: flex;
		justify-content: space-between;
		gap: 8px;
		padding: 2px 0;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.link {
		background: none;
		border: none;
		color: #217346;
		cursor: pointer;
		font: inherit;
		padding: 2px 0;
	}
	.main {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.toolbar {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 6px 10px;
		border-bottom: 1px solid #e3e5e8;
	}
	.run {
		font: inherit;
		background: #217346;
		color: #fff;
		border: none;
		border-radius: 3px;
		padding: 4px 12px;
		cursor: pointer;
	}
	.run {
		white-space: nowrap;
		flex-shrink: 0;
	}
	.hint {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.run:disabled {
		opacity: 0.6;
	}
	.code {
		height: 180px;
		min-height: 60px;
		border-bottom: 1px solid #e3e5e8;
		resize: vertical;
		overflow: hidden;
	}
	.result {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
		padding: 0 10px;
	}
	.table-wrap {
		flex: 1;
		overflow: auto;
	}
	table {
		border-collapse: collapse;
		font-size: 12px;
	}
	th,
	td {
		border: 1px solid #e3e5e8;
		padding: 2px 8px;
		white-space: nowrap;
	}
	th {
		background: #e3efe7;
		position: sticky;
		top: 0;
		text-align: left;
	}
	td.num {
		text-align: right;
	}
	.error {
		color: #b42318;
		white-space: pre-wrap;
	}
</style>
