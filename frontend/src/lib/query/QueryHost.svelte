<script lang="ts">
	import { resizePanel } from '#lib/ui/resize.js';
	import { panelKeys } from '#lib/ui/focus.js';
	import PanelHeader from '#lib/ui/PanelHeader.svelte';
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
	import { kindIcon, refreshedLabel } from './listview';
	import { CONN_LABELS, describe, isDbKind, type ConnKind } from './sources';
	import { requestLogin } from '#lib/remote/lattis.svelte.js';
	import { sharedWorkbooksEnabled } from '#lib/shell/settings.svelte.js';
	import { errorText, t } from '#lib/i18n/index.js';

	interface Props {
		/** Le classeur a changé ; sheet : feuille à afficher (tableau chargé). */
		onchanged: (sheet?: number) => void;
		onnotice: (msg: string) => void;
	}
	let { onchanged, onnotice }: Props = $props();

	const err = errorText;

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
		// Serveur Lattis : pas de mot de passe, la page de connexion du serveur.
		if (need.lattis) return requestLogin(need.lattis);
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
	const SOURCES: { group: string; items: { what: DataAction; label: string; icon: IconName }[] }[] = $derived([
		{
			group: t('query.host.group.file'),
			items: [
				{ what: 'csv', label: t('query.host.source.csv'), icon: 'file' },
				{ what: 'xlsx', label: t('query.sourceKind.xlsx'), icon: 'table' },
				{ what: 'json', label: 'JSON', icon: 'file' },
				{ what: 'parquet', label: 'Parquet', icon: 'file' },
				{ what: 'sqlite', label: t('query.host.source.sqlite'), icon: 'database' }
			]
		},
		{
			group: t('query.host.group.database'),
			items: [
				{ what: 'postgres', label: 'PostgreSQL', icon: 'database' },
				{ what: 'mysql', label: 'MySQL / MariaDB', icon: 'database' },
				{ what: 'sqlserver', label: 'SQL Server', icon: 'database' },
				...(sharedWorkbooksEnabled() ? [{ what: 'lattis' as DataAction, label: t('query.conn.lattis'), icon: 'database' as IconName }] : [])
			]
		},
		{
			group: t('query.host.group.other'),
			items: [
				{ what: 'web', label: t('query.conn.web'), icon: 'web' },
				{ what: 'table', label: t('query.sourceKind.table'), icon: 'sheet' }
			]
		}
	]);
	let picking = $state(false);
	// Menu ⋯ ouvert : « q<id> » (requête) ou « c<id> » (connexion).
	let menuFor = $state<string | null>(null);

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
						if (!ts.length) throw new Error(t('query.host.noExcelTable'));
						source.sheet = ts[0].sheet;
						source.index = ts[0].index;
						editSource = ts.length > 1;
					}
					editing = { query: newQuery(name, source), editSource };
					return;
				}
				case 'table': {
					if (!tables.length) throw new Error(t('query.host.noTable'));
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
	/** Ferme le volet des requêtes et connexions (autre classeur ouvert ou créé). */
	export function closePanel() {
		panel = false;
	}

	/** Classeur relu (Ctrl+Z, assistant, MCP, autre poste) : le volet ouvert relit ses listes. */
	export function reloadLists() {
		if (panel || editing) load().catch(() => {});
	}

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
		busy = id === null ? t('query.host.refreshingAll') : t('query.host.refreshing');
		try {
			const run = () => (id === null ? DocumentService.RefreshAllQueries() : DocumentService.RefreshQuery(id));
			let r: QueryRefresh = await run();
			while (r.needPassword) {
				if (!(await askPassword(r.needPassword))) return t('query.host.passwordAbandoned');
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
			if (id === null) onnotice(loads.length ? t('query.host.refreshedAll', { n: loads.length }) : t('query.host.nothingToRefresh'));
			else if (loads.length) onnotice(t('query.host.loaded', { table: loads[0].table, n: loads[0].rows }) + (warn.length ? ' ; ' + warn.join(' · ') : ''));
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
				onnotice(t('query.host.saved', { name: q.name }));
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
			onnotice(q.table ? t('query.host.deletedTable', { name: q.name, table: q.table }) : t('query.host.deleted', { name: q.name }));
		} catch (e) {
			onnotice(err(e));
		}
	}

	async function removeConn(c: Connection) {
		try {
			await DocumentService.DeleteConnection(c.id);
			await load();
			onchanged();
			onnotice(t('query.host.connDeleted', { name: c.name }));
		} catch (e) {
			onnotice(err(e));
		}
	}

	function editConn(c: Connection) {
		newConnection(c).then((n) => (n ? load() : undefined));
	}

	async function forget(c: Connection) {
		try {
			await DocumentService.ForgetConnectionPassword(c.name);
			await load();
			onnotice(t('query.host.forgotten', { name: c.name }));
		} catch (e) {
			onnotice(err(e));
		}
	}
</script>

{#if panel}
	<aside class="panel" use:resizePanel={{ key: 'queries' }} use:panelKeys={() => (panel = false)} aria-label={t('query.host.panel.title')}>
		<PanelHeader title={t('query.host.panel.title')} help="connexions" helpLabel={t('query.host.panel.help')} onclose={() => (panel = false)} />
		{#if busy}<p class="busy">{busy}</p>{/if}
		<div class="sec">
			<h3>{t('query.host.queries')}</h3>
			<span class="count">{queries.length}</span>
		</div>
		{#each queries as q (q.id)}
			<div class="row" class:open={menuFor === `q${q.id}`}>
				<button class="main" title={t('query.host.editQuery', { name: q.name })} onclick={() => (editing = { query: q, editSource: false })}>
					<Icon name={kindIcon(q.source?.kind ?? '')} size={16} />
					<span class="txt">
						<span class="name">{q.name}</span>
						<span class="sub" title={q.refreshed ? t('query.host.refreshedOn', { date: q.refreshed }) : ''}>{q.table ? `→ ${q.table}` : t('query.host.connectionOnly')}{q.refreshed ? ` · ${refreshedLabel(q.refreshed)}` : ''}</span>
					</span>
				</button>
				<div class="acts">
					<button class="ic" title={q.table ? t('query.host.refresh') : t('query.host.load')} aria-label={q.table ? t('query.host.refresh') : t('query.host.load')} onclick={() => refresh(q.id)} disabled={!!busy}><Icon name="refresh" size={14} /></button>
					<button class="ic more" title={t('query.host.more')} aria-label={t('query.host.more')} aria-haspopup="menu" aria-expanded={menuFor === `q${q.id}`} onclick={() => (menuFor = menuFor === `q${q.id}` ? null : `q${q.id}`)}>⋯</button>
					{#if menuFor === `q${q.id}`}
						<div class="row-menu" role="menu" use:dismiss={() => (menuFor = null)}>
							<button role="menuitem" onclick={() => ((menuFor = null), (editing = { query: q, editSource: false }))}><Icon name="edit" size={14} />{t('query.host.edit')}</button>
							<button role="menuitem" onclick={() => ((menuFor = null), refresh(q.id))} disabled={!!busy}><Icon name="refresh" size={14} />{q.table ? t('query.host.refresh') : t('query.host.load')}</button>
							<button role="menuitem" class="delete" onclick={() => ((menuFor = null), remove(q))}><Icon name="trash" size={14} />{t('query.host.delete')}</button>
						</div>
					{/if}
				</div>
			</div>
		{:else}
			<p class="empty">{t('query.host.noQuery')}</p>
		{/each}
		<div class="sec">
			<h3>{t('query.host.connections')}</h3>
			<span class="count">{conns.length}</span>
		</div>
		{#each conns as c (c.id)}
			<div class="row" class:open={menuFor === `c${c.id}`}>
				<button class="main" title={t('query.host.editConn', { name: c.name })} onclick={() => editConn(c)}>
					<Icon name={kindIcon(c.kind)} size={16} />
					<span class="txt">
						<span class="name">{c.name}</span>
						<span class="sub">{describe(c)}{c.remembered ? ` · ${c.kind === 'web' ? t('query.host.tokenRemembered') : t('query.host.passwordRemembered')}` : ''}</span>
					</span>
				</button>
				<div class="acts">
					<button class="ic more" title={t('query.host.more')} aria-label={t('query.host.more')} aria-haspopup="menu" aria-expanded={menuFor === `c${c.id}`} onclick={() => (menuFor = menuFor === `c${c.id}` ? null : `c${c.id}`)}>⋯</button>
					{#if menuFor === `c${c.id}`}
						<div class="row-menu" role="menu" use:dismiss={() => (menuFor = null)}>
							<button role="menuitem" onclick={() => ((menuFor = null), editConn(c))}><Icon name="edit" size={14} />{t('query.host.edit')}</button>
							{#if c.kind !== 'lattis'}
								<button role="menuitem" onclick={() => ((menuFor = null), forget(c))}><Icon name="key" size={14} />{c.kind === 'web' ? t('query.host.forgetToken') : t('query.host.forgetPassword')}</button>
							{/if}
							<button role="menuitem" class="delete" onclick={() => ((menuFor = null), removeConn(c))}><Icon name="trash" size={14} />{t('query.host.delete')}</button>
						</div>
					{/if}
				</div>
			</div>
		{/each}
		<div class="new-wrap">
			<button class="add" aria-haspopup="menu" aria-expanded={picking} onclick={() => (picking = !picking)}><Icon name="plus" size={14} />{t('query.host.newConn')}</button>
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
		position: relative;
		display: flex;
		flex-direction: column;
		padding: 8px 0;
		border-left: 1px solid #d4d7db;
		background: #fff;
		box-shadow: -4px 0 12px rgba(0, 0, 0, 0.08);
		overflow: auto;
		font-size: 12px;
	}
	.busy {
		margin: 0 12px 4px;
		color: #217346;
	}
	/* En-tête de section : libellé discret et compteur, à la manière d'Excel. */
	.sec {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 10px 12px 4px;
		border-bottom: 1px solid #e5e7eb;
		margin-bottom: 2px;
	}
	h3 {
		margin: 0;
		font-size: 11px;
		font-weight: 600;
		color: #4b5563;
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}
	.count {
		min-width: 16px;
		padding: 0 5px;
		border-radius: 8px;
		background: #eef0f2;
		color: #4b5563;
		font-size: 11px;
		text-align: center;
	}
	.empty {
		margin: 4px 12px;
		color: #6b7280;
	}
	/* Une ligne par requête ou connexion : clic = Modifier ; actions à droite, visibles au survol. */
	.row {
		position: relative;
		display: flex;
		align-items: center;
		padding: 0 6px 0 4px;
		--icon-accent: #217346;
	}
	.row:hover,
	.row:focus-within,
	.row.open {
		background: #eef5f0;
	}
	.main {
		flex: 1;
		min-width: 0;
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 5px 8px;
		border: none;
		background: none;
		text-align: left;
		color: #1f2937;
	}
	.txt {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.name,
	.sub {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.name {
		font-weight: 600;
	}
	.sub {
		color: #6b7280;
		font-size: 11px;
	}
	.acts {
		position: relative;
		display: flex;
		gap: 2px;
		opacity: 0;
	}
	.row:hover .acts,
	.row:focus-within .acts,
	.row.open .acts {
		opacity: 1;
	}
	.ic {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		padding: 0;
		border: 1px solid transparent;
		background: none;
		color: #374151;
	}
	.ic:hover:not(:disabled) {
		background: #fff;
		border-color: #c9cdd2;
	}
	.more {
		font-size: 15px;
		line-height: 1;
	}
	.row-menu,
	.src-menu {
		position: absolute;
		z-index: 20;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 6px 18px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		display: flex;
		flex-direction: column;
	}
	.row-menu {
		right: 0;
		top: calc(100% + 2px);
		min-width: 200px;
	}
	.src-menu {
		left: 0;
		top: calc(100% + 4px);
		min-width: 220px;
	}
	.src-group {
		padding: 6px 12px 2px;
		font-size: 11px;
		font-weight: 600;
		color: #6b7280;
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}
	.row-menu button,
	.src-menu button {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 5px 12px;
		border: none;
		border-radius: 0;
		background: none;
		text-align: left;
		white-space: nowrap;
		--icon-accent: #217346;
	}
	.row-menu button:hover:not(:disabled),
	.src-menu button:hover {
		background: #eef5f0;
	}
	.row-menu .delete {
		color: #c0392b;
		--icon-accent: #c0392b;
	}
	.new-wrap {
		position: relative;
		align-self: flex-start;
		margin: 6px 8px 0;
	}
	.add {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		border: none;
		background: none;
		color: #217346;
		padding: 4px 6px;
		--icon-accent: #217346;
	}
	.add:hover {
		background: #eef5f0;
	}
	button {
		font: inherit;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		padding: 1px 8px;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.5;
	}
</style>
