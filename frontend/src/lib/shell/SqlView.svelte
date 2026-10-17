<script lang="ts" module>
	/** Requête favorite, enregistrée dans le classeur. */
	export interface Favorite {
		id: number;
		name: string;
		sql: string;
	}
	export interface Favorites {
		list: Favorite[];
		/** Ajoute (nom vide : la première ligne) et renvoie l'identifiant, liste à jour. */
		add: (name: string, sql: string) => Promise<number>;
		rename: (id: number, name: string) => Promise<unknown>;
		remove: (id: number) => Promise<unknown>;
	}
</script>

<script lang="ts">
	// Onglet SQL : liste des objets, éditeur avec les noms affichés (traduits en
	// noms internes t_1, c_0… avant l'exécution), historique des requêtes.
	// Les requêtes d'écriture (INSERT, UPDATE, DELETE) partent telles quelles au
	// backend, qui les rend annulables (Ctrl+Z).
	import { formatValue, typeLabel } from '#lib/grid/format.js';
	import SqlEditor from '#lib/sql/SqlEditor.svelte';
	import { Catalog, displayHeaders, quote, translate, type SqlObject } from '#lib/sql/names.js';
	import { load, record, save, search, when, type HistoryEntry } from '#lib/sql/history.js';
	import { isWrite } from '#lib/sql/write.js';
	import { DEFAULT_NAME_HINT, isDefaultTableName } from '#lib/grid/defaultname.js';
	import { example, isReadOnly, type ExampleKind } from '#lib/sql/examples.js';
	import { dismiss } from '#lib/grid/dismiss.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { count, toTSV } from '#lib/sql/result.js';
	import { tick } from 'svelte';

	interface Result {
		columns: string[] | null;
		rows: (unknown[] | null)[] | null;
		truncated: boolean;
		micros: number;
	}
	interface Props {
		objects: SqlObject[];
		run: (sql: string) => Promise<Result>;
		/** Requête d'écriture ; absent : lecture seule (classeur de démonstration). */
		exec?: (sql: string) => Promise<{ changed: number; micros: number }>;
		ongoto: (table: string) => void;
		/** Requêtes favorites, enregistrées dans le classeur ; absent : pas de favoris (démonstration). */
		favorites?: Favorites;
		/**
		 * Pose tout le résultat de la requête (SQL traduit) dans un nouveau
		 * tableau sur une nouvelle feuille nommée name (vide : « Requête1 »…) ;
		 * absent : pas d'envoi (démonstration).
		 */
		tosheet?: (sql: string, name: string, headers: string[], link: boolean, columns: string[]) => Promise<void>;
	}

	let { objects, run, exec, ongoto, favorites, tosheet }: Props = $props();

	const catalog = $derived(new Catalog(objects));
	let panel = $state<'objects' | 'favorites' | 'history'>('objects');
	let open = $state<string | null>(null);
	let sql = $state('');
	let result = $state<Result | null>(null);
	let headers = $state<string[]>([]);
	let sent = $state('');
	let error = $state('');
	let running = $state(false);
	let editor = $state<ReturnType<typeof SqlEditor>>();
	// Texte de la dernière requête de lecture exécutée (vide après une écriture).
	let ran = $state('');
	let copied = $state(false);
	let sending = $state(false);

	let entries = $state<HistoryEntry[]>(load());
	let filter = $state('');
	const shown = $derived(search(entries, filter));
	// Position dans l'historique pour Alt+↑ / Alt+↓ (-1 : brouillon en cours).
	let cursor = -1;
	let draft = '';

	// Exemple proposé une seule fois, à l'ouverture : un éditeur vidé reste vide.
	let prefilled = false;
	$effect(() => {
		if (!prefilled && !sql && objects.length > 0) {
			prefilled = true;
			const o = objects[0];
			sql = `SELECT ${o.columns.slice(0, 4).map((c) => quote(c.name)).join(', ')}\nFROM ${quote(o.name)}\nLIMIT 100`;
		}
	});

	async function execute(text: string) {
		if (!text.trim() || running) return;
		running = true;
		error = '';
		editor?.showError('');
		const at = Date.now();
		try {
			if (exec && isWrite(text)) {
				sent = text;
				const r = await exec(text);
				ran = '';
				result = { columns: ['Lignes touchées'], rows: [[r.changed]], truncated: false, micros: r.micros };
				headers = ['Lignes touchées (Ctrl+Z pour annuler)'];
				remember({ sql: text, at, rows: r.changed, micros: r.micros });
				running = false;
				return;
			}
			const t = translate(text, catalog);
			sent = t.sql;
			const r = await run(t.sql);
			result = r;
			ran = text;
			copied = false;
			headers = displayHeaders(r.columns ?? [], t.scope);
			remember({ sql: text, at, rows: (r.rows ?? []).length, micros: r.micros });
		} catch (e) {
			result = null;
			// Sans le préfixe technique du moteur.
			error = String(e instanceof Error ? e.message : e).replace(/^sqlite3:\s*(SQL logic error:\s*)?/i, '');
			editor?.showError(error);
			remember({ sql: text, at, error });
		}
		running = false;
	}

	// Menu du clic droit sur un objet de la liste.
	let objMenu = $state<{ o: SqlObject; x: number; y: number } | null>(null);

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

	/** Remplace le contenu de l'éditeur par un exemple détaillé pour l'objet. */
	function showExample(o: SqlObject, kind: ExampleKind) {
		objMenu = null;
		cursor = -1;
		editor?.load(example(o, kind));
	}

	// --- Favoris -------------------------------------------------------------

	const favList = $derived(favorites?.list ?? []);
	const same = (a: string, b: string) => a.trim().replace(/\s+/g, ' ') === b.trim().replace(/\s+/g, ' ');
	const starred = $derived(favList.find((f) => same(f.sql, sql)));
	// Favori en cours de renommage.
	let renaming = $state<{ id: number; name: string } | null>(null);
	let favError = $state('');

	async function addFavorite(text: string) {
		if (!favorites || !text.trim()) return;
		favError = '';
		try {
			const id = await favorites.add('', text);
			panel = 'favorites';
			await tick();
			const f = favorites.list.find((x) => x.id === id);
			renaming = { id, name: f?.name ?? '' };
		} catch (e) {
			favError = String(e instanceof Error ? e.message : e);
		}
	}

	async function commitRename() {
		const r = renaming;
		renaming = null;
		if (!favorites || !r || !r.name.trim()) return;
		const f = favorites.list.find((x) => x.id === r.id);
		if (f && f.name === r.name.trim()) return;
		try {
			await favorites.rename(r.id, r.name);
		} catch (e) {
			favError = String(e instanceof Error ? e.message : e);
		}
	}

	async function removeFavorite(f: Favorite) {
		try {
			await favorites?.remove(f.id);
		} catch (e) {
			favError = String(e instanceof Error ? e.message : e);
		}
	}

	function useFavorite(f: Favorite, now: boolean) {
		cursor = -1;
		editor?.load(f.sql);
		if (now) execute(f.sql);
	}

	const select = (node: HTMLInputElement) => {
		node.focus();
		node.select();
	};

	/** Copie le résultat affiché, à coller dans Excel. */
	async function copyResult() {
		if (!result) return;
		try {
			await navigator.clipboard.writeText(toTSV(headers, result.rows ?? []));
			copied = true;
			setTimeout(() => (copied = false), 1500);
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	}

	/**
	 * Envoie tout le résultat vers une nouvelle feuille, nommée comme le favori
	 * s'il y en a un : copié (valeurs figées) ou référencé (tableau lié à la
	 * requête, actualisable comme une source externe).
	 */
	async function sendToSheet(link: boolean) {
		if (!tosheet || !ran || sending) return;
		sending = true;
		try {
			const fav = favList.find((f) => same(f.sql, ran));
			await tosheet(sent, fav?.name ?? '', headers, link, result?.columns ?? []);
		} catch (e) {
			error = String(e instanceof Error ? e.message : e).replace(/^sqlite3:\s*(SQL logic error:\s*)?/i, '');
		}
		sending = false;
	}

	const cell = (v: unknown) => (v === null || v === undefined ? '' : typeof v === 'number' ? formatValue('decimal', v).replace(/,00$/, '') : String(v));
</script>

<div class="sql">
	<aside class="side">
		<div class="tabs" role="tablist">
			<button role="tab" aria-selected={panel === 'objects'} class:on={panel === 'objects'} onclick={() => (panel = 'objects')}>Objets</button>
			{#if favorites}
				<button role="tab" aria-selected={panel === 'favorites'} class:on={panel === 'favorites'} onclick={() => (panel = 'favorites')}>
					Favoris{favList.length ? ` (${favList.length})` : ''}
				</button>
			{/if}
			<button role="tab" aria-selected={panel === 'history'} class:on={panel === 'history'} onclick={() => (panel = 'history')}>
				Historique{entries.length ? ` (${entries.length})` : ''}
			</button>
		</div>
		{#if panel === 'objects'}
			<div class="list">
				{#each objects as o (o.sqlName)}
					<div class="obj">
						<button
							class="obj-head"
							onclick={() => (open = open === o.sqlName ? null : o.sqlName)}
							oncontextmenu={(e) => {
								e.preventDefault();
								objMenu = { o, x: e.clientX, y: e.clientY };
							}}
							title="Clic droit : exemples SELECT, INSERT, UPDATE, DELETE, voir sur la feuille"
						>
							<span>{open === o.sqlName ? '▾' : '▸'}</span>
							{#if !isReadOnly(o) && isDefaultTableName(o.name)}
								<strong class="default-name" title={DEFAULT_NAME_HINT}>{o.name}</strong>
							{:else}
								<strong>{o.name}</strong>
							{/if}
							<span class="muted">{isReadOnly(o) ? 'tableau croisé' : 'tableau'} · {count(o.rows)}</span>
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
						{/if}
					</div>
				{/each}
			</div>
			{#if objMenu}
				{@const m = objMenu}
				<div class="ctx" role="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (objMenu = null)}>
					<button role="menuitem" title="Remplace la requête par un exemple de lecture" onclick={() => showExample(m.o, 'select')}>SELECT</button>
					{#if exec && !isReadOnly(m.o)}
						<button role="menuitem" title="Remplace la requête par un exemple d'ajout de ligne" onclick={() => showExample(m.o, 'insert')}>INSERT</button>
						<button role="menuitem" title="Remplace la requête par un exemple de modification" onclick={() => showExample(m.o, 'update')}>UPDATE</button>
						<button role="menuitem" title="Remplace la requête par un exemple de suppression" onclick={() => showExample(m.o, 'delete')}>DELETE</button>
					{/if}
					<hr />
					<button
						role="menuitem"
						onclick={() => {
							// `m` suit objMenu : lire le nom avant de fermer le menu.
							const name = m.o.name;
							objMenu = null;
							ongoto(name);
						}}>Voir sur la feuille</button
					>
				</div>
			{/if}
		{:else if panel === 'favorites'}
			{#if favError}<p class="error fav-error">{favError}</p>{/if}
			<div class="list">
				{#each favList as f (f.id)}
					<div class="entry fav">
						{#if renaming?.id === f.id}
							<input
								class="fav-name"
								aria-label="Nom du favori"
								bind:value={renaming.name}
								use:select
								onblur={commitRename}
								onkeydown={(e) => {
									if (e.key === 'Enter') commitRename();
									else if (e.key === 'Escape') renaming = null;
								}}
							/>
						{:else}
							<button class="fav-title" title="Reprendre dans l'éditeur (double-clic : exécuter)" onclick={() => useFavorite(f, false)} ondblclick={() => useFavorite(f, true)}>★ {f.name}</button>
						{/if}
						<button class="entry-sql" title="Reprendre dans l'éditeur (double-clic : exécuter)" onclick={() => useFavorite(f, false)} ondblclick={() => useFavorite(f, true)}>{f.sql}</button>
						<div class="entry-foot">
							<span>
								<button class="link" onclick={() => (renaming = { id: f.id, name: f.name })}>Renommer</button>
								·
								<button class="link" onclick={() => removeFavorite(f)}>Retirer</button>
							</span>
							<button class="link" title="Exécuter" onclick={() => useFavorite(f, true)}>▶</button>
						</div>
					</div>
				{:else}
					<p class="muted empty">Aucune requête favorite. ☆ dans la barre de l'éditeur ou dans l'historique pour en ajouter une ; elles sont enregistrées dans le classeur.</p>
				{/each}
			</div>
		{:else}
			<div class="hist-tools">
				<input type="search" placeholder="Rechercher" bind:value={filter} />
				{#if entries.length}<button class="link clear" onclick={clearHistory}><Icon name="trash" size={14} />Effacer</button>{/if}
			</div>
			<div class="list">
				{#each shown as e (e.at + e.sql)}
					<div class="entry" class:failed={!!e.error}>
						<button class="entry-sql" title="Reprendre dans l'éditeur (double-clic : exécuter)" onclick={() => useEntry(e, false)} ondblclick={() => useEntry(e, true)}>{e.sql}</button>
						<div class="entry-foot">
							<span class="muted">{when(e.at)} · {e.error ? 'erreur' : count(e.rows ?? 0)}</span>
							<span>
								{#if favorites}
									{@const fav = favList.some((f) => same(f.sql, e.sql))}
									<button class="link star" class:on={fav} title={fav ? 'Déjà dans les favoris' : 'Ajouter aux favoris'} disabled={fav} onclick={() => addFavorite(e.sql)}>{fav ? '★' : '☆'}</button>
								{/if}
								<button class="link" title="Exécuter" onclick={() => useEntry(e, true)}>▶</button>
							</span>
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
			<button class="run" onclick={() => editor?.runCurrent()} disabled={running} title="Exécute la sélection, sinon l'instruction sous le curseur (Ctrl+Entrée)"><Icon name="play" size={14} />Exécuter</button>
			{#if favorites}
				<button
					class="fav-btn"
					class:on={!!starred}
					title={starred ? `Dans les favoris : « ${starred.name} » (cliquer pour le retirer)` : 'Ajouter la requête (ou la sélection) aux favoris du classeur'}
					onclick={() => (starred ? removeFavorite(starred) : addFavorite(editor?.selectedOrAll() ?? sql))}>{starred ? '★' : '☆'} Favori</button
				>
			{/if}
			<span class="muted hint" title="Les instructions sont séparées par « ; » ou une ligne vide. 1 000 lignes au plus.">Ctrl+Entrée : exécuter la sélection, sinon l'instruction sous le curseur · Ctrl+Espace : noms · Alt+↑↓ : historique</span>
		</div>
		<div class="code">
			<SqlEditor bind:this={editor} bind:value={sql} {catalog} onrun={execute} onhistory={step} />
		</div>
		<div class="result">
			{#if error}
				<p class="error">{error}</p>
			{:else if result}
				<div class="result-bar">
					<p class="muted">
						{count((result.rows ?? []).length)}{result.truncated ? ' (tronqué)' : ''} · {(result.micros / 1000).toLocaleString('fr-FR', { maximumFractionDigits: 1 })} ms
						<span class="sent" title={sent}>· SQL exécuté</span>
					</p>
					{#if ran}
						<button class="res-btn" onclick={copyResult} title={result.truncated ? 'Copie les lignes affichées (tronquées), à coller dans Excel' : 'Copie le résultat (en-têtes compris), à coller dans Excel'}
							><Icon name="copy" size={14} />{copied ? 'Copié ✓' : 'Presse-papiers'}</button
						>
						{#if tosheet}
							<span class="to-sheet">
								Vers une feuille :
								<button class="res-btn" onclick={() => sendToSheet(false)} disabled={sending} title="Copie les valeurs du résultat dans un nouveau tableau, sur une nouvelle feuille (Ctrl+Z pour annuler)"
									><Icon name="sheet" size={14} />Copier</button
								>
								<button class="res-btn" onclick={() => sendToSheet(true)} disabled={sending} title="Crée un tableau lié à la requête, sur une nouvelle feuille : fond bleu clair, actualisable au clic droit comme une source externe"
									><Icon name="reference" size={14} />Référencer</button
								>
							</span>
						{/if}
					{/if}
				</div>
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
	.result-bar {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.result-bar p {
		margin-right: auto;
	}
	.to-sheet {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		color: #57606a;
	}
	.res-btn {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		--icon-accent: #217346;
		font: inherit;
		background: #fff;
		color: #1f2328;
		border: 1px solid #d4d7db;
		border-radius: 3px;
		padding: 2px 10px;
		cursor: pointer;
		white-space: nowrap;
	}
	.res-btn:hover:not(:disabled) {
		background: #f5f9f6;
	}
	.ctx {
		position: fixed;
		z-index: 30;
		display: flex;
		flex-direction: column;
		min-width: 180px;
		padding: 4px 0;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
	}
	.ctx button {
		text-align: left;
		background: none;
		border: none;
		font: inherit;
		padding: 6px 14px;
		cursor: pointer;
	}
	.ctx button:hover {
		background: #e3efe7;
	}
	.ctx hr {
		border: none;
		border-top: 1px solid #e3e5e8;
		margin: 4px 0;
	}
	.fav-btn {
		font: inherit;
		background: #fff;
		color: #1f2328;
		border: 1px solid #d4d7db;
		border-radius: 3px;
		padding: 3px 10px;
		cursor: pointer;
		white-space: nowrap;
		flex-shrink: 0;
	}
	.fav-btn:hover {
		background: #f5f9f6;
	}
	.fav-btn.on,
	.star.on {
		color: #b7791f;
	}
	.star {
		font-size: 14px;
		margin-right: 6px;
	}
	.star:disabled {
		cursor: default;
	}
	.entry.fav {
		border-left-color: #d4a017;
	}
	.fav-title {
		display: block;
		width: 100%;
		text-align: left;
		background: none;
		border: none;
		font: inherit;
		font-weight: 600;
		padding: 4px 6px 0;
		cursor: pointer;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.fav-name {
		display: block;
		box-sizing: border-box;
		width: calc(100% - 8px);
		margin: 4px 4px 0;
		font: inherit;
		font-weight: 600;
		border: 1px solid #217346;
		border-radius: 3px;
		padding: 2px 4px;
	}
	.fav-error {
		margin: 8px 8px 0;
		font-size: 12px;
	}
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
	.link.clear {
		display: inline-flex;
		align-items: center;
		gap: 4px;
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
		display: inline-flex;
		align-items: center;
		gap: 5px;
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
	/* Nom de tableau par défaut (DEFAULT_NAME_COLOR) : à renommer. */
	.default-name {
		color: #c42b1c;
	}
</style>
