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
	import { alterRename } from '#lib/sql/rename.js';
	import SqlEditor from '#lib/sql/SqlEditor.svelte';
	import { Catalog, displayHeaders, quote, translate, type SqlObject } from '#lib/sql/names.js';
	import { load, record, save, search, when, type HistoryEntry } from '#lib/sql/history.js';
	import { isWrite } from '#lib/sql/write.js';
	import { DEFAULT_NAME_HINT, isDefaultTableName } from '#lib/grid/defaultname.js';
	import { example, isReadOnly, type ExampleKind } from '#lib/sql/examples.js';
	import { dismiss } from '#lib/grid/dismiss.js';
	import { clip, clipped } from '#lib/ui/menu.js';
	import Submenu from '#lib/ui/Submenu.svelte';
	import { favoriteNameProblem, refusedName, tableNameProblem } from '#lib/ui/names.js';
	import { nameAlertOpen, refuseName } from '#lib/ui/nameAlert.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { count, toTSV } from '#lib/sql/result.js';
	import { tick } from 'svelte';
	import type { RightsObject } from '#lib/remote/rights.js';
	import { dataExample, dataTarget, pgIdent, type DataGroup } from '#lib/remote/data.js';

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
		/** Renomme un tableau (même commande que la grille) ; absent : pas de renommage. */
		onrename?: (from: string, to: string) => Promise<void>;
		/** Classeur partagé : rôles et règles générés par le serveur (lecture seule). */
		rights?: RightsObject[];
		/** Propriétaire : ouvre l'écran Partager (modifier les droits). */
		onrights?: () => void;
		/** Données du Data Lake lisibles des serveurs connectés (vues donnees.*), un groupe par serveur. */
		data?: DataGroup[];
		/** Exécute une requête sur un serveur (vues donnees.*, SQL de PostgreSQL) ; absent : pas de données du Data Lake. */
		rundata?: (server: string, sql: string) => Promise<Result>;
	}

	let { objects, run, exec, ongoto, favorites, tosheet, onrename, rights = [], onrights, data = [], rundata }: Props = $props();
	let rightsOpen = $state<string | null>(null);
	// Vue donnees.* dépliée (serveur et nom).
	let dataOpen = $state<string | null>(null);

	// Renommage d'un tableau dans la liste des objets (double-clic, F2, clic droit).
	let tblRen = $state<{ name: string; draft: string } | null>(null);
	let tblRenInput = $state<HTMLInputElement>();
	// Renommage envoyé, réponse attendue : ni second envoi ni abandon.
	let tblRenBusy = false;
	function startTblRen(o: SqlObject) {
		if (!onrename || isReadOnly(o)) return;
		objMenu = null;
		tblRen = { name: o.name, draft: o.name };
	}
	/** Fin du renommage : un nom refusé ouvre la boîte « nom invalide » et la saisie reprend sur le texte tapé. */
	async function commitTblRen() {
		const r = tblRen;
		if (!r || tblRenBusy || nameAlertOpen()) return;
		const to = r.draft.trim();
		if (!onrename || to === r.name) {
			tblRen = null;
			return;
		}
		const problem = tableNameProblem(to, objects.filter((o) => !isReadOnly(o) && o.name !== r.name).map((o) => o.name));
		if (problem) {
			await refuseName(() => tblRenInput, problem);
			return;
		}
		tblRenBusy = true;
		try {
			await onrename(r.name, to);
			tblRen = null;
		} catch (e) {
			await refuseName(() => tblRenInput, refusedName('Nom de tableau refusé', e));
		} finally {
			tblRenBusy = false;
		}
	}
	function focusAll(node: HTMLInputElement) {
		requestAnimationFrame(() => {
			node.focus();
			node.select();
		});
	}

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
			// Vues donnees.* : la requête part au serveur qui les a (SQL de
			// PostgreSQL, lecture seule) ; elle ne cite pas les tableaux du classeur.
			const target = rundata ? dataTarget(text, data) : null;
			if (target && rundata) {
				sent = text;
				const r = await rundata(target.server, text);
				result = r;
				ran = '';
				copied = false;
				headers = r.columns ?? [];
				remember({ sql: text, at, rows: (r.rows ?? []).length, micros: r.micros });
				running = false;
				return;
			}
			// ALTER TABLE … RENAME TO … : le renommage du classeur (références comprises).
			const ren = onrename ? alterRename(text) : null;
			if (ren && onrename) {
				await onrename(ren.from, ren.to);
				result = { columns: ['Tableau renommé'], rows: [[`${ren.from} → ${ren.to}`]], truncated: false, micros: 0 };
				headers = ['Tableau renommé (Ctrl+Z pour annuler)'];
				remember({ sql: text, at, rows: 0, micros: 0 });
				running = false;
				return;
			}
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
	let favRenInput = $state<HTMLInputElement>();
	let favRenBusy = false;
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

	/** Fin du renommage d'un favori : nom vide ou refusé, la boîte l'explique et la saisie reprend. */
	async function commitRename() {
		const r = renaming;
		if (!r || favRenBusy || nameAlertOpen()) return;
		const f = favorites?.list.find((x) => x.id === r.id);
		if (!favorites || (f && f.name === r.name.trim())) {
			renaming = null;
			return;
		}
		const problem = favoriteNameProblem(r.name);
		if (problem) {
			await refuseName(() => favRenInput, problem);
			return;
		}
		favRenBusy = true;
		try {
			await favorites.rename(r.id, r.name);
			renaming = null;
		} catch (e) {
			await refuseName(() => favRenInput, refusedName('Nom de favori refusé', e));
		} finally {
			favRenBusy = false;
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
						{#if tblRen && tblRen.name === o.name}
							<div class="obj-head">
								<span>{open === o.sqlName ? '▾' : '▸'}</span>
								<input
									class="rename"
									bind:this={tblRenInput}
									bind:value={tblRen.draft}
									use:focusAll
									aria-label="Nouveau nom du tableau"
									onkeydown={(e) => {
										e.stopPropagation();
										if (e.key === 'Enter') {
											e.preventDefault();
											commitTblRen();
										}
										if (e.key === 'Escape' && !tblRenBusy) tblRen = null;
									}}
									onblur={commitTblRen}
								/>
							</div>
						{:else}
						<button
							class="obj-head"
							onclick={() => (open = open === o.sqlName ? null : o.sqlName)}
							ondblclick={() => startTblRen(o)}
							onkeydown={(e) => {
								if (e.key === 'F2') {
									e.preventDefault();
									startTblRen(o);
								}
							}}
							oncontextmenu={(e) => {
								e.preventDefault();
								objMenu = { o, x: e.clientX, y: e.clientY };
							}}
							title={onrename && !isReadOnly(o) ? 'Double-clic ou F2 : renommer ; clic droit : exemples SELECT, INSERT, UPDATE, DELETE, voir sur la feuille' : 'Clic droit : exemples SELECT, INSERT, UPDATE, DELETE, voir sur la feuille'}
						>
							<span>{open === o.sqlName ? '▾' : '▸'}</span>
							{#if !isReadOnly(o) && isDefaultTableName(o.name)}
								<strong class="default-name" title={DEFAULT_NAME_HINT}>{o.name}</strong>
							{:else}
								<strong>{o.name}</strong>
							{/if}
							<span class="muted">{isReadOnly(o) ? 'tableau croisé' : 'tableau'} · {count(o.rows)}</span>
						</button>
						{/if}
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
				{#if rundata}
					{#each data as g (g.server)}
						{#if g.views.length}
							<div class="rights-head"><span title={g.server}>Data Lake Lattis · {g.label}</span></div>
							{#each g.views as v (v.name)}
								{@const key = g.server + ' ' + v.name}
								<div class="obj">
									<button
										class="obj-head"
										onclick={() => (dataOpen = dataOpen === key ? null : key)}
										ondblclick={() => {
											cursor = -1;
											editor?.load(dataExample(v));
										}}
										title={`${v.label}${v.description ? ' — ' + v.description : ''} : vue du serveur en lecture seule, limitée aux lignes de votre audience (double-clic : exemple SELECT)`}
									>
										<span>{dataOpen === key ? '▾' : '▸'}</span>
										<strong>donnees.{v.name}</strong>
										<span class="muted">{v.label}</span>
									</button>
									{#if dataOpen === key}
										<ul>
											{#each v.columns as c (c.name)}
												<li>
													<button class="col" title="Insérer dans la requête" onclick={() => editor?.insert(pgIdent(c.name))}>{c.name}</button>
													<span class="muted">{typeLabel(c.type)}</span>
												</li>
											{/each}
										</ul>
									{/if}
								</div>
							{/each}
						{/if}
					{/each}
				{/if}
				{#if rights.length}
					<div class="rights-head">
						<span>Droits (serveur)</span>
						{#if onrights}<button class="link" onclick={onrights} title="Écran Partager : membres, rôles, restrictions">Modifier…</button>{/if}
					</div>
					{#each rights as r (r.name + r.kind)}
						<div class="obj">
							<button class="obj-head" onclick={() => (rightsOpen = rightsOpen === r.name + r.kind ? null : r.name + r.kind)} title="Généré par le serveur : lecture seule">
								<span>{rightsOpen === r.name + r.kind ? '▾' : '▸'}</span>
								<strong>{r.name}</strong>
								<span class="muted">{r.kind}</span>
							</button>
							{#if rightsOpen === r.name + r.kind}
								<ul>
									{#each r.lines as l, i (i)}<li class="rule">{l}</li>{/each}
								</ul>
							{/if}
						</div>
					{/each}
				{/if}
			</div>
			{#if objMenu}
				{@const m = objMenu}
				<div class="ctx" role="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (objMenu = null)}>
					<div class="ctx-title" title={clipped(m.o.name) ? m.o.name : undefined}>{isReadOnly(m.o) ? 'Tableau croisé' : 'Tableau'} « {clip(m.o.name)} »</div>
					{#if exec && !isReadOnly(m.o)}
						<!-- Exemples de requête : sous-menu (SELECT, INSERT, UPDATE, DELETE). -->
						<Submenu label="Exemple de requête">
							<button role="menuitem" title="Remplace la requête par un exemple de lecture" onclick={() => showExample(m.o, 'select')}>SELECT</button>
							<button role="menuitem" title="Remplace la requête par un exemple d'ajout de ligne" onclick={() => showExample(m.o, 'insert')}>INSERT</button>
							<button role="menuitem" title="Remplace la requête par un exemple de modification" onclick={() => showExample(m.o, 'update')}>UPDATE</button>
							<button role="menuitem" title="Remplace la requête par un exemple de suppression" onclick={() => showExample(m.o, 'delete')}>DELETE</button>
						</Submenu>
					{:else}
						<button role="menuitem" title="Remplace la requête par un exemple de lecture" onclick={() => showExample(m.o, 'select')}>Exemple SELECT</button>
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
					{#if onrename && !isReadOnly(m.o)}
						<button role="menuitem" onclick={() => startTblRen(m.o)}>Renommer… <span class="keys">F2</span></button>
					{/if}
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
								bind:this={favRenInput}
								bind:value={renaming.name}
								use:select
								onblur={commitRename}
								onkeydown={(e) => {
									if (e.key === 'Enter') {
										e.preventDefault();
										commitRename();
									} else if (e.key === 'Escape' && !favRenBusy) renaming = null;
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
	.rename {
		flex: 1;
		min-width: 0;
		font: inherit;
		font-weight: 600;
		padding: 1px 4px;
		border: 1px solid #217346;
		border-radius: 3px;
	}
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
		max-width: 320px; /* MENU_MAX_WIDTH */
		padding: 4px 0;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
	}
	.ctx-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.ctx button {
		text-align: left;
		background: none;
		border: none;
		font: inherit;
		padding: 6px 14px;
		cursor: pointer;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.ctx .keys {
		float: right;
		margin-left: 16px;
		color: #6b7280;
		font-size: 12px;
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
		/* Contexte d'empilement propre : rien de la vue ne passe au-dessus du volet d'aide. */
		isolation: isolate;
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
	.rights-head {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		margin: 10px 4px 2px;
		padding-top: 6px;
		border-top: 1px solid #e3e5e8;
		font-size: 12px;
		font-weight: 600;
		color: #4b5563;
	}
	li.rule {
		display: block;
		font-family: var(--mono, monospace);
		font-size: 11px;
		color: #374151;
		word-break: break-word;
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
