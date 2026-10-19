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
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);

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
				error = 'Mot de passe requis pour lister les tables.';
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
		s = { kind: k };
		error = '';
		dbTables = [];
		if (isDbKind(k) || k === 'web') loadConns().then(loadTables);
		if (k === 'table') s.table = tables[0];
		if (k === 'query') s.query = queries[0];
		if (k === 'web') s.url = 'https://';
	}

	function ok() {
		error = '';
		if (isFile && !s.path) return void (error = 'Choisissez un fichier.');
		if (s.kind === 'xlsx' && !s.sheet) return void (error = 'Choisissez un tableau du classeur.');
		if (s.kind === 'table' && !s.table) return void (error = 'Choisissez un tableau.');
		if (s.kind === 'query' && !s.query) return void (error = 'Choisissez une requête.');
		if (isDB && (!s.connection || !s.sql?.trim())) return void (error = 'Choisissez une connexion et une table (ou écrivez une requête SQL).');
		if (isFileDB && !s.sql?.trim()) return void (error = 'Choisissez une table (ou écrivez une requête SQL).');
		if (s.kind === 'sql' && !s.sql?.trim()) return void (error = 'Écrivez une requête SQL.');
		if (s.kind === 'web') {
			if (!/^https?:\/\/[^/]+/i.test(s.url?.trim() ?? '')) return void (error = 'L’adresse doit commencer par http:// ou https://.');
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
		aria-label="Source"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') oncancel();
		}}
	>
		<header>Source</header>
		<div class="body">
			<label class="field">Type de source <Select value={s.kind} options={sourceKinds(sharedWorkbooksEnabled(), source.kind)} width="100%" onchange={changeKind} /></label>
			{#if isFile}
				<div class="field">
					Fichier
					<div class="row">
						<input class="grow" bind:value={s.path} onchange={() => (s.kind === 'xlsx' ? loadXlsx() : isFileDB ? loadTables() : undefined)} placeholder="Chemin du fichier" />
						<button class="ico" onclick={browse}><Icon name="open" size={14} />Parcourir…</button>
					</div>
				</div>
				{#if s.kind === 'csv'}
					<label class="field"
						>Séparateur <Select
							value={s.delimiter ?? ''}
							options={[
								{ value: '', label: 'Détecté automatiquement' },
								{ value: ';', label: 'Point-virgule (;)' },
								{ value: ',', label: 'Virgule (,)' },
								{ value: '\\t', label: 'Tabulation' },
								{ value: '|', label: 'Barre verticale (|)' }
							]}
							width="100%"
							onchange={(v) => (s.delimiter = v || undefined)}
						/></label
					>
				{:else if s.kind === 'xlsx'}
					<div class="field">
						Tableau du classeur
						<div class="list">
							{#each xlsx as t (t.sheet + t.index)}
								<button class="item" class:on={t.sheet === s.sheet && t.index === (s.index ?? 0)} onclick={() => ((s.sheet = t.sheet), (s.index = t.index))}>
									<strong>{t.sheet}</strong> › tableau {t.index + 1} <span class="muted">({count(t.rows)} : {(t.headers ?? []).slice(0, 6).join(', ')}{(t.headers ?? []).length > 6 ? '…' : ''})</span>
								</button>
							{:else}
								<p class="muted">{s.path ? 'Aucun tableau de deux lignes ou plus dans ce classeur.' : 'Choisissez un fichier.'}</p>
							{/each}
						</div>
					</div>
				{:else if s.kind === 'json'}
					<label class="field">Chemin des enregistrements <input bind:value={s.recordPath} placeholder="vide : la racine ou son seul tableau ; ex. data.items" /></label>
					<p class="muted">Tableau d’objets, ou un objet JSON par ligne (NDJSON). Les objets imbriqués deviennent des colonnes « parent.enfant ».</p>
				{:else if s.kind === 'parquet'}
					<p class="muted">Chaque colonne du fichier donne une colonne (« parent.enfant » pour une structure ; une liste devient du texte).</p>
				{/if}
			{:else if s.kind === 'table'}
				<label class="field">Tableau <Select value={s.table ?? ''} options={tables.map((t) => ({ value: t, label: t }))} width="100%" onchange={(v) => (s.table = v)} /></label>
			{:else if s.kind === 'query'}
				<label class="field">Requête <Select value={s.query ?? ''} options={queries.map((t) => ({ value: t, label: t }))} width="100%" onchange={(v) => (s.query = v)} /></label>
				<p class="muted">La requête part du résultat de l'autre requête (référence) : modifier l'autre requête change aussi celle-ci.</p>
			{:else if isDB}
				<div class="field">
					Connexion {CONN_LABELS[s.kind as ConnKind]}
					<div class="row">
						<Select value={s.connection ?? ''} options={kindConns.map((c) => ({ value: c.name, label: `${c.name} (${describe(c)})` }))} width="100%" onchange={(v) => ((s.connection = v), loadTables())} />
						<button class="ico" onclick={addConnection}><Icon name="plus" size={14} />Nouvelle…</button>
					</div>
				</div>
			{:else if s.kind === 'web'}
				<label class="field">Adresse (GET) <input bind:value={s.url} placeholder="https://api.exemple.fr/v1/clients?page=1" /></label>
				<label class="field">Chemin des enregistrements <input bind:value={s.recordPath} placeholder="vide : la racine ou son seul tableau ; ex. data.items" /></label>
				<label class="field"
					>En-têtes (un par ligne, « Nom: valeur ») <textarea rows="2" bind:value={headers} placeholder="Accept-Language: fr"></textarea></label
				>
				<div class="field">
					Authentification
					<div class="row">
						<Select
							value={s.connection ?? ''}
							options={[{ value: '', label: 'Aucune (API publique)' }, ...kindConns.map((c) => ({ value: c.name, label: `${c.name} (${c.config.url})` }))]}
							width="100%"
							onchange={(v) => (s.connection = v || undefined)}
						/>
						<button class="ico" onclick={addConnection}><Icon name="plus" size={14} />Nouvelle…</button>
					</div>
				</div>
				<p class="muted">Les en-têtes sont enregistrés dans le classeur : un jeton ou une clé d’API passe par une connexion Web (gardée dans le trousseau du système).</p>
			{/if}
			{#if isDB || isFileDB}
				{#if dbTables.length}
					<div class="field">
						Tables et vues <input placeholder="Rechercher" bind:value={tableFilter} />
						<div class="list">
							{#each shownTables as t (t.schema + '.' + t.name)}
								<button class="item" onclick={() => pickTable(t)}>{t.schema ? t.schema + '.' : ''}<strong>{t.name}</strong>{t.view ? ' (vue)' : ''}</button>
							{/each}
						</div>
					</div>
				{/if}
				<label class="field">Requête SQL (lecture seule) <textarea rows="5" bind:value={s.sql} placeholder="SELECT * FROM ventes WHERE annee = 2026"></textarea></label>
				<p class="muted">
					{isDB ? 'Filtrez de préférence ici : la requête est exécutée par le serveur, les étapes ensuite par Lattis.' : 'Le fichier est ouvert en lecture seule ; les étapes s’appliquent ensuite.'}
				</p>
			{:else if s.kind === 'sql'}
				<label class="field">Requête SQL sur le classeur (lecture seule) <textarea rows="6" bind:value={s.sql} placeholder="SELECT * FROM Ventes WHERE annee = 2026"></textarea></label>
				<p class="muted">Comme dans l'onglet SQL : la requête est relue à chaque actualisation.{s.sheet ? ` Les adresses Cell[A1] sans feuille se lisent sur « ${s.sheet} ».` : ''}</p>
			{/if}
		</div>
		{#if error}<p class="error">{error}</p>{/if}
		<footer>
			<button class="primary" onclick={ok}>OK</button>
			<button onclick={oncancel}>Annuler</button>
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
