<script lang="ts">
	// Source d'une requête (première des étapes appliquées) : fichier (CSV,
	// Excel, JSON, Parquet, base SQLite), tableau du classeur, autre
	// requête, requête SQL du classeur, SGBD (PostgreSQL, MySQL, SQL Server :
	// connexion, navigateur des tables ou requête SQL) ou API web JSON.
	import { modal } from '#lib/ui/focus.js';
	import { onMount } from 'svelte';
	import Select from '#lib/ui/Select.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { Connection, DBTable, PasswordNeed, Source, XLSXTable } from '#bindings/internal/query/models.js';
	import { clone } from './steps';
	import { CONN_LABELS, FILE_DB_KINDS, FILE_KINDS, describe, headersText, isDbKind, parseHeaders, sourceKinds, type ConnKind } from './sources';
	import { sharedWorkbooksEnabled } from '#lib/shell/settings.svelte.js';
	import { count } from '#lib/sql/result.js';
	import { errorText, t } from '#lib/i18n/index.js';

	interface Props {
		source: Source;
		/** Requêtes du classeur (sauf celle qu'on modifie). */
		queries: string[];
		tables: string[];
		/** Nouvelle connexion du genre kind : renvoie son nom (null : abandon). */
		newConnection: (kind: ConnKind) => Promise<string | null>;
		askPassword: (need: PasswordNeed) => Promise<boolean>;
		onok: (s: Source) => void;
		oncancel: () => void;
	}
	let { source, queries, tables, newConnection, askPassword, onok, oncancel }: Props = $props();

	const initial = () => clone(source);
	let s = $state<Source>(initial());
	let error = $state('');
	const err = errorText;

	const isFile = $derived(FILE_KINDS.includes(s.kind));
	const isFileDB = $derived(FILE_DB_KINDS.includes(s.kind));
	const isDB = $derived(isDbKind(s.kind));

	// Classeur Excel : tableaux du fichier.
	let xlsx = $state<XLSXTable[]>([]);
	async function loadXlsx() {
		xlsx = [];
		if (s.kind !== 'xlsx' || !s.path) return;
		try {
			xlsx = (await DocumentService.ExcelTables(s.path)) ?? [];
			if (xlsx.length && !xlsx.some((t) => t.sheet === s.sheet && t.index === (s.index ?? 0))) {
				s.sheet = xlsx[0].sheet;
				s.index = xlsx[0].index;
			}
		} catch (e) {
			error = err(e);
		}
	}

	// SGBD et fichiers de base : connexions et tables.
	let conns = $state<Connection[]>([]);
	let dbTables = $state<DBTable[]>([]);
	let tableFilter = $state('');
	const kindConns = $derived(conns.filter((c) => c.kind === (s.kind === 'web' ? 'web' : s.kind)));
	async function loadConns() {
		conns = (await DocumentService.Connections()) ?? [];
		if (isDbKind(s.kind) && !kindConns.some((c) => c.name === s.connection)) s.connection = kindConns[0]?.name;
	}
	async function loadTables() {
		dbTables = [];
		error = '';
		try {
			if (FILE_DB_KINDS.includes(s.kind)) {
				if (s.path) dbTables = (await DocumentService.FileTables(s.kind, s.path)) ?? [];
				return;
			}
			if (!isDbKind(s.kind) || !s.connection) return;
			let r = await DocumentService.ConnectionTables(s.connection);
			if (r.needPassword && (await askPassword(r.needPassword))) r = await DocumentService.ConnectionTables(s.connection);
			if (r.needPassword) {
				error = t('query.source.passwordRequired');
				return;
			}
			dbTables = r.tables ?? [];
		} catch (e) {
			error = err(e);
		}
	}
	async function pickTable(t: DBTable) {
		s.sql = await DocumentService.TableSQL(s.kind, t.schema, t.name);
	}
	const shownTables = $derived(dbTables.filter((t) => !tableFilter || (t.schema + '.' + t.name).toLowerCase().includes(tableFilter.toLowerCase())).slice(0, 300));

	// API web : en-têtes saisis en texte.
	let headers = $state(headersText(initial().headers));

	onMount(async () => {
		if (s.kind === 'xlsx') loadXlsx();
		if (isDbKind(s.kind) || s.kind === 'web') loadConns().then(loadTables);
		if (FILE_DB_KINDS.includes(s.kind)) loadTables();
		if (s.kind === 'table' && !s.table) s.table = tables[0];
		if (s.kind === 'query' && !s.query) s.query = queries[0];
	});

	async function browse() {
		const p = await DocumentService.PickDataFile(s.kind);
		if (p) {
			s.path = p;
			if (s.kind === 'xlsx') await loadXlsx();
			if (FILE_DB_KINDS.includes(s.kind)) await loadTables();
		}
	}

	async function addConnection() {
		const name = await newConnection((s.kind === 'web' ? 'web' : s.kind) as ConnKind);
		if (!name) return;
		await loadConns();
		s.connection = name;
		await loadTables();
	}

	function changeKind(k: string) {
		// Retour à la formule d'origine : sa feuille et ses en-têtes reviennent.
		s = k === 'formula' && source.kind === 'formula' ? initial() : { kind: k };
		error = '';
		dbTables = [];
		if (isDbKind(k) || k === 'web') loadConns().then(loadTables);
		if (k === 'table') s.table = tables[0];
		if (k === 'query') s.query = queries[0];
		if (k === 'web') s.url = 'https://';
	}

	function ok() {
		error = '';
		if (isFile && !s.path) return void (error = t('query.source.need.file'));
		if (s.kind === 'xlsx' && !s.sheet) return void (error = t('query.source.need.xlsxTable'));
		if (s.kind === 'table' && !s.table) return void (error = t('query.source.need.table'));
		if (s.kind === 'query' && !s.query) return void (error = t('query.source.need.query'));
		if (isDB && (!s.connection || !s.sql?.trim())) return void (error = t('query.source.need.connection'));
		if (isFileDB && !s.sql?.trim()) return void (error = t('query.source.need.fileTable'));
		if (s.kind === 'sql' && !s.sql?.trim()) return void (error = t('query.source.need.sql'));
		if (s.kind === 'formula') {
			const f = s.formula?.trim() ?? '';
			if (!f.startsWith('=')) return void (error = t('query.source.need.formula'));
			s.formula = f;
		}
		if (s.kind === 'web') {
			if (!/^https?:\/\/[^/]+/i.test(s.url?.trim() ?? '')) return void (error = t('query.source.need.url'));
			s.url = s.url!.trim();
			s.headers = parseHeaders(headers);
			if (!s.headers.length) delete s.headers;
			if (!s.connection) delete s.connection;
		}
		if (!s.recordPath?.trim()) delete s.recordPath;
		onok(clone(s));
	}
</script>

<div class="back">
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label={t('query.source.title')}
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') oncancel();
		}}
	>
		<header>{t('query.source.title')}</header>
		<div class="body">
			<label class="field">{t('query.source.kind')} <Select value={s.kind} options={sourceKinds(sharedWorkbooksEnabled(), source.kind)} width="100%" onchange={changeKind} /></label>
			{#if isFile}
				<div class="field">
					{t('query.source.file.label')}
					<div class="row">
						<input class="grow" bind:value={s.path} onchange={() => (s.kind === 'xlsx' ? loadXlsx() : isFileDB ? loadTables() : undefined)} placeholder={t('query.source.file.placeholder')} />
						<button class="ico" onclick={browse}><Icon name="open" size={14} />{t('query.source.file.browse')}</button>
					</div>
				</div>
				{#if s.kind === 'csv'}
					<label class="field"
						>{t('query.source.csv.delimiter')} <Select
							value={s.delimiter ?? ''}
							options={[
								{ value: '', label: t('query.source.csv.auto') },
								{ value: ';', label: t('query.source.csv.semicolon') },
								{ value: ',', label: t('query.source.csv.comma') },
								{ value: '\\t', label: t('query.source.csv.tab') },
								{ value: '|', label: t('query.source.csv.pipe') }
							]}
							width="100%"
							onchange={(v) => (s.delimiter = v || undefined)}
						/></label
					>
				{:else if s.kind === 'xlsx'}
					<div class="field">
						{t('query.sourceKind.table')}
						<div class="list">
							{#each xlsx as tb (tb.sheet + tb.index)}
								<button class="item" class:on={tb.sheet === s.sheet && tb.index === (s.index ?? 0)} onclick={() => ((s.sheet = tb.sheet), (s.index = tb.index))}>
									<strong>{tb.sheet}</strong> › {t('query.source.xlsx.table', { n: String(tb.index + 1) })} <span class="muted">({count(tb.rows)} : {(tb.headers ?? []).slice(0, 6).join(', ')}{(tb.headers ?? []).length > 6 ? '…' : ''})</span>
								</button>
							{:else}
								<p class="muted">{s.path ? t('query.host.noExcelTable') : t('query.source.need.file')}</p>
							{/each}
						</div>
					</div>
				{:else if s.kind === 'json'}
					<label class="field">{t('query.source.recordPath.label')} <input bind:value={s.recordPath} placeholder={t('query.source.recordPath.placeholder')} /></label>
					<p class="muted">{t('query.source.json.help')}</p>
				{:else if s.kind === 'parquet'}
					<p class="muted">{t('query.source.parquet.help')}</p>
				{/if}
			{:else if s.kind === 'table'}
				<label class="field">{t('query.source.table')} <Select value={s.table ?? ''} options={tables.map((tb) => ({ value: tb, label: tb }))} width="100%" onchange={(v) => (s.table = v)} /></label>
			{:else if s.kind === 'query'}
				<label class="field">{t('query.source.query')} <Select value={s.query ?? ''} options={queries.map((tb) => ({ value: tb, label: tb }))} width="100%" onchange={(v) => (s.query = v)} /></label>
				<p class="muted">{t('query.source.queryHelp')}</p>
			{:else if isDB}
				<div class="field">
					{t('query.conn.dialog.aria', { kind: CONN_LABELS[s.kind as ConnKind] })}
					<div class="row">
						<Select value={s.connection ?? ''} options={kindConns.map((c) => ({ value: c.name, label: `${c.name} (${describe(c)})` }))} width="100%" onchange={(v) => ((s.connection = v), loadTables())} />
						<button class="ico" onclick={addConnection}><Icon name="plus" size={14} />{t('query.source.newConn')}</button>
					</div>
				</div>
			{:else if s.kind === 'web'}
				<label class="field">{t('query.source.web.url')} <input bind:value={s.url} placeholder={t('query.source.web.urlPlaceholder')} /></label>
				<label class="field">{t('query.source.recordPath.label')} <input bind:value={s.recordPath} placeholder={t('query.source.recordPath.placeholder')} /></label>
				<label class="field"
					>{t('query.source.web.headers')} <textarea rows="2" bind:value={headers} placeholder="Accept-Language: fr"></textarea></label
				>
				<div class="field">
					{t('query.source.web.auth')}
					<div class="row">
						<Select
							value={s.connection ?? ''}
							options={[{ value: '', label: t('query.source.web.public') }, ...kindConns.map((c) => ({ value: c.name, label: `${c.name} (${c.config.url})` }))]}
							width="100%"
							onchange={(v) => (s.connection = v || undefined)}
						/>
						<button class="ico" onclick={addConnection}><Icon name="plus" size={14} />{t('query.source.newConn')}</button>
					</div>
				</div>
				<p class="muted">{t('query.source.web.help')}</p>
			{/if}
			{#if isDB || isFileDB}
				{#if dbTables.length}
					<div class="field">
						{t('query.source.tables')} <input placeholder={t('query.source.search')} bind:value={tableFilter} />
						<div class="list">
							{#each shownTables as tb (tb.schema + '.' + tb.name)}
								<button class="item" onclick={() => pickTable(tb)}>{tb.schema ? tb.schema + '.' : ''}<strong>{tb.name}</strong>{tb.view ? ` ${t('query.source.view')}` : ''}</button>
							{/each}
						</div>
					</div>
				{/if}
				<label class="field">{t('query.source.sql.label')} <textarea rows="5" bind:value={s.sql} placeholder="SELECT * FROM ventes WHERE annee = 2026"></textarea></label>
				<p class="muted">
					{isDB ? t('query.source.sql.dbHelp') : t('query.source.sql.fileHelp')}
				</p>
			{:else if s.kind === 'sql'}
				<label class="field">{t('query.source.sql.workbook')} <textarea rows="6" bind:value={s.sql} placeholder="SELECT * FROM Ventes WHERE annee = 2026"></textarea></label>
				<p class="muted">{t('query.source.sql.workbookHelp')}{s.sheet ? ` ${t('query.source.sql.sheet', { sheet: s.sheet })}` : ''}</p>
			{:else if s.kind === 'formula'}
				<label class="field">{t('query.source.formula.label')} <input bind:value={s.formula} placeholder="=UNIQUE(Ventes[Client])" /></label>
				<p class="muted">{t('query.source.formula.help')}</p>
			{/if}
		</div>
		{#if error}<p class="error">{error}</p>{/if}
		<footer>
			<button class="primary" onclick={ok}>{t('common.ok')}</button>
			<button onclick={oncancel}>{t('common.cancel')}</button>
		</footer>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: calc(var(--z-dialog) + 10);
		background: var(--backdrop);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.dialog {
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 620px;
		max-width: calc(100vw - 32px);
		max-height: calc(100vh - 64px);
		display: flex;
		flex-direction: column;
		font-size: 12px;
	}
	header {
		padding: 10px 14px 4px;
		font-size: 14px;
		font-weight: 600;
	}
	.body {
		padding: 4px 14px;
		overflow: auto;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.row {
		display: flex;
		gap: 6px;
		align-items: center;
	}
	.grow {
		flex: 1;
	}
	input,
	textarea {
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 3px 6px;
	}
	textarea {
		font-family: Consolas, 'Cascadia Mono', monospace;
		resize: vertical;
	}
	input:focus,
	textarea:focus {
		outline: 1px solid #217346;
		border-color: #217346;
	}
	.list {
		border: 1px solid #e5e7eb;
		max-height: 180px;
		overflow: auto;
		display: flex;
		flex-direction: column;
	}
	.item {
		text-align: left;
		border: none;
		border-radius: 0;
		padding: 3px 8px;
	}
	.item:hover,
	.item.on {
		background: #e3efe7;
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
		padding: 3px 10px;
		cursor: pointer;
	}
	/* Bouton avec icône devant le libellé. */
	.ico {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		--icon-accent: #217346;
	}
	.error {
		color: #b42318;
		margin: 6px 14px 0;
	}
	footer {
		display: flex;
		justify-content: flex-end;
		gap: 6px;
		padding: 10px 14px;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
</style>
