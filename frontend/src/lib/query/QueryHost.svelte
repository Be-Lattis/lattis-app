<script lang="ts">
	import { resizePanel } from '#lib/ui/resize.js';
	// Données › Obtenir des données : tout ce qui touche aux requêtes d'import
	// (équivalent de Power Query) et aux connexions, hors de la grille. La page
	// n'appelle que data() et reçoit onchanged (classeur modifié, feuille à
	// afficher) et onnotice.
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { Connection, PasswordNeed, Query } from '#bindings/internal/query/models.js';
	import type { QueryRefresh } from '#bindings/internal/bridge/models.js';
	import type { DataAction } from '#lib/shell/Ribbon.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import type { IconName } from '#lib/icons/icons.js';
	import { dismiss } from '#lib/grid/dismiss.js';
	import QueryEditor from './QueryEditor.svelte';
	import ConnectionDialog from './ConnectionDialog.svelte';
	import PasswordDialog from './PasswordDialog.svelte';
	import { newQuery } from './steps';
	import { CONN_LABELS, describe, isDbKind, type ConnKind } from './sources';

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
	let connDialog = $state<{ connection: Connection | null; kind?: ConnKind; done: (name: string | null) => void } | null>(null);
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

	/** kind absent (Nouvelle connexion…) : le genre se choisit dans le dialogue. */
	function newConnection(connection: Connection | null = null, kind?: ConnKind): Promise<string | null> {
		return new Promise((resolve) => {
			connDialog = {
				connection,
				kind: (connection?.kind as ConnKind) ?? kind,
				done: (name) => {
					connDialog = null;
					resolve(name);
				}
			};
		});
	}

	// « Nouvelle connexion… » : toutes les sources, comme Données › Obtenir des
	// données. Une base de données s'enregistre comme connexion ; les autres
	// ouvrent l'éditeur de requête sur la source choisie.
	const SOURCES: { group: string; items: { what: DataAction; label: string; icon: IconName }[] }[] = [
		{
			group: 'Fichier',
			items: [
				{ what: 'csv', label: 'Texte/CSV', icon: 'file' },
				{ what: 'xlsx', label: 'Classeur Excel', icon: 'table' },
				{ what: 'json', label: 'JSON', icon: 'file' },
				{ what: 'parquet', label: 'Parquet', icon: 'file' },
				{ what: 'sqlite', label: 'Base SQLite', icon: 'database' }
			]
		},
		{
			group: 'Base de données',
			items: [
				{ what: 'postgres', label: 'PostgreSQL', icon: 'database' },
				{ what: 'mysql', label: 'MySQL / MariaDB', icon: 'database' },
				{ what: 'sqlserver', label: 'SQL Server', icon: 'database' }
			]
		},
		{
			group: 'Autres sources',
			items: [
				{ what: 'web', label: 'Web (API)', icon: 'web' },
				{ what: 'table', label: 'Tableau du classeur', icon: 'sheet' }
			]
		}
	];
	let picking = $state(false);

	async function newSource(what: DataAction) {
		picking = false;
		if (isDbKind(what)) {
			const n = await newConnection(null, what);
			if (n) await load().then(() => onchanged());
			return;
		}
		await data(what);
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
				case 'xlsx':
				case 'json':
				case 'parquet':
				case 'sqlite': {
					const path = await DocumentService.PickDataFile(what);
					if (!path) return;
					const name = await DocumentService.NewQueryName(await DocumentService.FileBase(path));
					// Base SQLite : la table se choisit dans la boîte de la source.
					let editSource = what === 'sqlite';
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
				case 'web': {
					const name = await DocumentService.NewQueryName('Web');
					editing = { query: newQuery(name, { kind: 'web', url: 'https://' }), editSource: true };
					return;
				}
				default: {
					if (!isDbKind(what)) return;
					let conn: string | null = conns.find((c) => c.kind === what)?.name ?? null;
					if (!conn) conn = await newConnection(null, what);
					if (!conn) return;
					await load();
					// La boîte de connexion permet de changer de genre : la source suit.
					const kind = conns.find((c) => c.name === conn)?.kind ?? what;
					const name = await DocumentService.NewQueryName(conn);
					editing = { query: newQuery(name, { kind, connection: conn, sql: '' }), editSource: true };
					return;
				}
			}
		} catch (e) {
			onnotice(err(e));
		}
	}

	/** Actualise la requête qui charge un tableau (menu du clic droit sur le tableau). */
	export function refreshOne(id: number): Promise<string> {
		return refresh(id);
	}

	/**
	 * Actualise toutes les requêtes (Ctrl+F5) ; renvoie le bilan pour la barre
	 * d'état (ok faux : erreur ou abandon, déjà signalés).
	 */
	export async function refreshAll(): Promise<{ ok: boolean; count: number }> {
		const msg = await refresh(null);
		return { ok: !msg, count: lastCount };
	}

	let lastCount = 0;

	/** Actualise une requête (id) ou toutes (null), mot de passe demandé au besoin. */
	async function refresh(id: number | null): Promise<string> {
		lastCount = 0;
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
			lastCount = loads.length;
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
	<aside class="panel" use:resizePanel={{ key: 'queries' }}>
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
					<button onclick={() => (editing = { query: q, editSource: false })}><Icon name="edit" size={14} />Modifier</button>
					<button onclick={() => refresh(q.id)} disabled={!!busy}><Icon name="refresh" size={14} />{q.table ? 'Actualiser' : 'Charger'}</button>
					<button class="delete" onclick={() => remove(q)}><Icon name="trash" size={14} />Supprimer</button>
				</div>
			</div>
		{:else}
			<p class="muted">Aucune requête : Données › Obtenir des données.</p>
		{/each}
		<h3>Connexions ({conns.length})</h3>
		{#each conns as c (c.id)}
			<div class="item">
				<span class="name">{c.name}</span>
				<span class="muted">{describe(c)}{c.remembered ? (c.kind === 'web' ? ' · jeton mémorisé' : ' · mot de passe mémorisé') : ''}</span>
				<div class="acts">
					<button onclick={() => newConnection(c).then((n) => (n ? load() : undefined))}><Icon name="edit" size={14} />Modifier</button>
					<button onclick={() => forget(c)}><Icon name="key" size={14} />{c.kind === 'web' ? 'Oublier le jeton' : 'Oublier le mot de passe'}</button>
					<button class="delete" onclick={() => removeConn(c)}><Icon name="trash" size={14} />Supprimer</button>
				</div>
			</div>
		{/each}
		<div class="new-wrap">
			<button class="add" aria-haspopup="menu" aria-expanded={picking} onclick={() => (picking = !picking)}><Icon name="plus" size={14} />Nouvelle connexion…</button>
			{#if picking}
				<div class="src-menu" role="menu" use:dismiss={() => (picking = false)}>
					{#each SOURCES as g (g.group)}
						<div class="src-group">{g.group}</div>
						{#each g.items as it (it.what)}
							<button role="menuitem" onclick={() => newSource(it.what)}><Icon name={it.icon} size={14} />{it.label}</button>
						{/each}
					{/each}
				</div>
			{/if}
		</div>
	</aside>
{/if}

{#if editing}
	<QueryEditor
		query={editing.query}
		editSource={editing.editSource}
		queries={queries.map((q) => q.name)}
		{tables}
		{askPassword}
		newConnection={(kind) => newConnection(null, kind)}
		onfinish={finish}
		onclose={() => (editing = null)}
	/>
{/if}
{#if connDialog}
	<ConnectionDialog connection={connDialog.connection} kind={connDialog.kind} onsaved={(n) => connDialog?.done(n)} oncancel={() => connDialog?.done(null)} />
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
		width: 450px;
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
	/* Boutons d'action : icône devant le libellé, accent vert (rouge pour supprimer). */
	.acts button,
	.new-wrap {
		position: relative;
		align-self: flex-start;
	}
	.src-menu {
		position: absolute;
		left: 0;
		top: calc(100% + 4px);
		z-index: 20;
		min-width: 220px;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 6px 18px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		display: flex;
		flex-direction: column;
	}
	.src-group {
		padding: 6px 12px 2px;
		font-size: 11px;
		font-weight: 600;
		color: #6b7280;
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}
	.src-menu button {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 5px 12px;
		border: none;
		background: none;
		font: inherit;
		text-align: left;
		cursor: pointer;
		--icon-accent: #217346;
	}
	.src-menu button:hover {
		background: #eef5f0;
	}
	.add {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		--icon-accent: #217346;
	}
	.acts .delete {
		--icon-accent: #c0392b;
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
