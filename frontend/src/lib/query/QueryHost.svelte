<script lang="ts">
	// Données › Obtenir des données : tout ce qui touche aux requêtes d'import
	// (équivalent de Power Query) et aux connexions, hors de la grille. La page
	// n'appelle que data() et reçoit onchanged (classeur modifié, feuille à
	// afficher) et onnotice.
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { Connection, PasswordNeed, Query } from '#bindings/internal/query/models.js';
	import type { QueryRefresh } from '#bindings/internal/bridge/models.js';
	import type { DataAction } from '#lib/shell/Ribbon.svelte';
	import QueryEditor from './QueryEditor.svelte';
	import ConnectionDialog from './ConnectionDialog.svelte';
	import PasswordDialog from './PasswordDialog.svelte';
	import { newQuery } from './steps';

	interface Props {
		/** Le classeur a changé ; sheet : feuille à afficher (tableau chargé). */
		onchanged: (sheet?: number) => void;
		onnotice: (msg: string) => void;
	}
	let { onchanged, onnotice }: Props = $props();

	const err = (e: unknown) => String(e instanceof Error ? e.message : e);

	let panel = $state(false);
	let queries = $state<Query[]>([]);
	let conns = $state<Connection[]>([]);
	let tables = $state<string[]>([]);
	let editing = $state<{ query: Query; editSource: boolean } | null>(null);
	let connDialog = $state<{ connection: Connection | null; done: (name: string | null) => void } | null>(null);
	let pwNeed = $state<{ need: PasswordNeed; done: (ok: boolean) => void } | null>(null);
	let busy = $state('');

	async function load() {
		[queries, conns, tables] = await Promise.all([
			DocumentService.Queries().then((q) => q ?? []),
			DocumentService.Connections().then((c) => c ?? []),
			DocumentService.WorkbookTables().then((t) => t ?? [])
		]);
	}

	function askPassword(need: PasswordNeed): Promise<boolean> {
		return new Promise((resolve) => {
			pwNeed = {
				need,
				done: (ok) => {
					pwNeed = null;
					resolve(ok);
				}
			};
		});
	}

	function newConnection(connection: Connection | null = null): Promise<string | null> {
		return new Promise((resolve) => {
			connDialog = {
				connection,
				done: (name) => {
					connDialog = null;
					resolve(name);
				}
			};
		});
	}

	/** Commande du ruban (Données). */
	export async function data(what: DataAction) {
		try {
			await load();
			switch (what) {
				case 'queries':
					panel = !panel;
					return;
				case 'refreshAll':
					await refresh(null);
					return;
				case 'csv':
				case 'xlsx': {
					const path = await DocumentService.PickDataFile(what);
					if (!path) return;
					const name = await DocumentService.NewQueryName(await DocumentService.FileBase(path));
					let editSource = false;
					const source = { kind: what, path } as Query['source'];
					if (what === 'xlsx') {
						const ts = (await DocumentService.ExcelTables(path)) ?? [];
						if (!ts.length) throw new Error('Aucun tableau de deux lignes ou plus dans ce classeur.');
						source.sheet = ts[0].sheet;
						source.index = ts[0].index;
						editSource = ts.length > 1;
					}
					editing = { query: newQuery(name, source), editSource };
					return;
				}
				case 'table': {
					if (!tables.length) throw new Error('Le classeur n’a aucun tableau.');
					const name = await DocumentService.NewQueryName(tables[0]);
					editing = { query: newQuery(name, { kind: 'table', table: tables[0] }), editSource: tables.length > 1 };
					return;
				}
				case 'postgres': {
					let conn: string | null = conns[0]?.name ?? null;
					if (!conn) conn = await newConnection();
					if (!conn) return;
					await load();
					const name = await DocumentService.NewQueryName(conn);
					editing = { query: newQuery(name, { kind: 'postgres', connection: conn, sql: '' }), editSource: true };
					return;
				}
			}
		} catch (e) {
			onnotice(err(e));
		}
	}

	/** Actualise une requête (id) ou toutes (null), mot de passe demandé au besoin. */
	async function refresh(id: number | null): Promise<string> {
		busy = id === null ? 'Actualisation de toutes les requêtes…' : 'Actualisation…';
		try {
			const run = () => (id === null ? DocumentService.RefreshAllQueries() : DocumentService.RefreshQuery(id));
			let r: QueryRefresh = await run();
			while (r.needPassword) {
				if (!(await askPassword(r.needPassword))) return 'Actualisation abandonnée : mot de passe non saisi.';
				r = await run();
			}
			const loads = r.loads ?? [];
			const errors = r.errors ?? [];
			await load();
			onchanged(loads.length === 1 && id !== null ? loads[0].sheet : undefined);
			if (errors.length) {
				onnotice(errors.join(' · '));
				return errors.join('\n');
			}
			const warn = loads.flatMap((l) => l.warnings ?? []);
			if (id === null) onnotice(loads.length ? `${loads.length} requête(s) actualisée(s)` : 'Aucune requête chargée à actualiser');
			else if (loads.length) onnotice(`Tableau « ${loads[0].table} » : ${loads[0].rows.toLocaleString('fr-FR')} ligne(s) chargée(s)${warn.length ? ' ; ' + warn.join(' · ') : ''}`);
			return '';
		} catch (e) {
			onnotice(err(e));
			return err(e);
		} finally {
			busy = '';
		}
	}

	async function finish(q: Query, loadIt: boolean): Promise<string> {
		try {
			const id = await DocumentService.SaveQuery(q);
			if (loadIt) {
				const msg = await refresh(id);
				if (msg) return msg;
			} else {
				await load();
				onchanged();
				onnotice(`Requête « ${q.name} » enregistrée`);
			}
			editing = null;
			return '';
		} catch (e) {
			return err(e);
		}
	}

	async function remove(q: Query) {
		try {
			await DocumentService.DeleteQuery(q.id);
			await load();
			onchanged();
			onnotice(`Requête « ${q.name} » supprimée${q.table ? ` (le tableau ${q.table} reste sur sa feuille)` : ''}`);
		} catch (e) {
			onnotice(err(e));
		}
	}

	async function removeConn(c: Connection) {
		try {
			await DocumentService.DeleteConnection(c.id);
			await load();
			onchanged();
		} catch (e) {
			onnotice(err(e));
		}
	}

	async function forget(c: Connection) {
		try {
			await DocumentService.ForgetConnectionPassword(c.name);
			await load();
			onnotice(`Mot de passe de « ${c.name} » oublié`);
		} catch (e) {
			onnotice(err(e));
		}
	}
</script>

{#if panel}
	<aside class="panel">
		<header>
			<strong>Requêtes et connexions</strong>
			<button class="x" title="Fermer le volet" onclick={() => (panel = false)}>✕</button>
		</header>
		{#if busy}<p class="muted">{busy}</p>{/if}
		<h3>Requêtes ({queries.length})</h3>
		{#each queries as q (q.id)}
			<div class="item">
				<button class="name" title="Modifier la requête" onclick={() => (editing = { query: q, editSource: false })}>{q.name}</button>
				<span class="muted">{q.table ? `→ ${q.table}` : 'connexion seulement'}{q.refreshed ? ` · ${q.refreshed}` : ''}</span>
				<div class="acts">
					<button onclick={() => (editing = { query: q, editSource: false })}>Modifier</button>
					<button onclick={() => refresh(q.id)} disabled={!!busy}>{q.table ? 'Actualiser' : 'Charger'}</button>
					<button onclick={() => remove(q)}>Supprimer</button>
				</div>
			</div>
		{:else}
			<p class="muted">Aucune requête : Données › Obtenir des données.</p>
		{/each}
		<h3>Connexions ({conns.length})</h3>
		{#each conns as c (c.id)}
			<div class="item">
				<span class="name">{c.name}</span>
				<span class="muted">PostgreSQL {c.postgres.user}@{c.postgres.host}:{c.postgres.port || 5432}/{c.postgres.database}{c.remembered ? ' · mot de passe mémorisé' : ''}</span>
				<div class="acts">
					<button onclick={() => newConnection(c).then((n) => (n ? load() : undefined))}>Modifier</button>
					<button onclick={() => forget(c)}>Oublier le mot de passe</button>
					<button onclick={() => removeConn(c)}>Supprimer</button>
				</div>
			</div>
		{/each}
		<button class="add" onclick={() => newConnection().then((n) => (n ? load().then(() => onchanged()) : undefined))}>Nouvelle connexion PostgreSQL…</button>
	</aside>
{/if}

{#if editing}
	<QueryEditor
		query={editing.query}
		editSource={editing.editSource}
		queries={queries.map((q) => q.name)}
		{tables}
		{askPassword}
		newConnection={() => newConnection()}
		onfinish={finish}
		onclose={() => (editing = null)}
	/>
{/if}
{#if connDialog}
	<ConnectionDialog connection={connDialog.connection} onsaved={(n) => connDialog?.done(n)} oncancel={() => connDialog?.done(null)} />
{/if}
{#if pwNeed}
	<PasswordDialog need={pwNeed.need} ondone={(ok) => pwNeed?.done(ok)} />
{/if}

<style>
	.panel {
		position: absolute;
		top: 0;
		right: 0;
		bottom: 0;
		z-index: 20;
		width: 300px;
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 8px 10px;
		border-left: 1px solid #d4d7db;
		background: #fafbfb;
		box-shadow: -4px 0 12px rgba(0, 0, 0, 0.08);
		overflow: auto;
		font-size: 12px;
	}
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		font-size: 13px;
	}
	h3 {
		margin: 6px 0 0;
		font-size: 12px;
		font-weight: 600;
	}
	.item {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 6px;
		background: #fff;
		border: 1px solid #e5e7eb;
	}
	.name {
		font-weight: 600;
		text-align: left;
		border: none;
		background: none;
		padding: 0;
		cursor: pointer;
	}
	.acts {
		display: flex;
		gap: 4px;
		flex-wrap: wrap;
	}
	.muted {
		color: #6b7280;
		margin: 0;
	}
	button {
		font: inherit;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		padding: 1px 8px;
		cursor: pointer;
	}
	button:hover:not(:disabled) {
		border-color: #217346;
	}
	button:disabled {
		opacity: 0.5;
	}
	.x {
		border: none;
		background: none;
	}
	.add {
		align-self: flex-start;
	}
</style>
