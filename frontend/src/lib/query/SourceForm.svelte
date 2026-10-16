<script lang="ts">
	// Source d'une requête (première des étapes appliquées) : fichier CSV,
	// tableau d'un classeur Excel, tableau du classeur, autre requête ou
	// PostgreSQL (connexion, navigateur des tables ou requête SQL).
	import { onMount } from 'svelte';
	import Select from '#lib/ui/Select.svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { Connection, PasswordNeed, PGTable, Source, XLSXTable } from '#bindings/internal/query/models.js';
	import { clone } from './steps';
	import { count } from '#lib/sql/result.js';

	interface Props {
		source: Source;
		/** Requêtes du classeur (sauf celle qu'on modifie). */
		queries: string[];
		tables: string[];
		/** Nouvelle connexion PostgreSQL : renvoie son nom (null : abandon). */
		newConnection: () => Promise<string | null>;
		askPassword: (need: PasswordNeed) => Promise<boolean>;
		onok: (s: Source) => void;
		oncancel: () => void;
	}
	let { source, queries, tables, newConnection, askPassword, onok, oncancel }: Props = $props();

	const initial = () => clone(source);
	let s = $state<Source>(initial());
	let error = $state('');
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);

	const KINDS = [
		{ value: 'csv', label: 'Fichier texte ou CSV' },
		{ value: 'xlsx', label: 'Classeur Excel' },
		{ value: 'table', label: 'Tableau du classeur' },
		{ value: 'query', label: 'Autre requête' },
		{ value: 'postgres', label: 'PostgreSQL' },
		{ value: 'sql', label: 'Requête SQL du classeur' }
	];

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

	// PostgreSQL : connexions et tables.
	let conns = $state<Connection[]>([]);
	let pgTables = $state<PGTable[]>([]);
	let pgFilter = $state('');
	async function loadConns() {
		conns = (await DocumentService.Connections()) ?? [];
		if (!s.connection && conns.length) s.connection = conns[0].name;
	}
	async function loadPgTables() {
		pgTables = [];
		if (!s.connection) return;
		error = '';
		try {
			let r = await DocumentService.PostgresTables(s.connection);
			if (r.needPassword && (await askPassword(r.needPassword))) r = await DocumentService.PostgresTables(s.connection);
			if (r.needPassword) {
				error = 'Mot de passe requis pour lister les tables.';
				return;
			}
			pgTables = r.tables ?? [];
		} catch (e) {
			error = err(e);
		}
	}
	async function pickTable(t: PGTable) {
		s.sql = await DocumentService.PostgresTableSQL(t.schema, t.name);
	}
	const shownTables = $derived(pgTables.filter((t) => !pgFilter || (t.schema + '.' + t.name).toLowerCase().includes(pgFilter.toLowerCase())).slice(0, 300));

	onMount(() => {
		if (s.kind === 'xlsx') loadXlsx();
		if (s.kind === 'postgres') loadConns().then(loadPgTables);
		if (s.kind === 'table' && !s.table) s.table = tables[0];
		if (s.kind === 'query' && !s.query) s.query = queries[0];
	});

	async function browse() {
		const p = await DocumentService.PickDataFile(s.kind === 'xlsx' ? 'xlsx' : 'csv');
		if (p) {
			s.path = p;
			if (s.kind === 'xlsx') await loadXlsx();
		}
	}

	async function addConnection() {
		const name = await newConnection();
		if (!name) return;
		await loadConns();
		s.connection = name;
		await loadPgTables();
	}

	function changeKind(k: string) {
		s = { kind: k };
		error = '';
		if (k === 'postgres') loadConns().then(loadPgTables);
		if (k === 'table') s.table = tables[0];
		if (k === 'query') s.query = queries[0];
	}

	function ok() {
		error = '';
		if ((s.kind === 'csv' || s.kind === 'xlsx') && !s.path) return void (error = 'Choisissez un fichier.');
		if (s.kind === 'xlsx' && !s.sheet) return void (error = 'Choisissez un tableau du classeur.');
		if (s.kind === 'table' && !s.table) return void (error = 'Choisissez un tableau.');
		if (s.kind === 'query' && !s.query) return void (error = 'Choisissez une requête.');
		if (s.kind === 'postgres' && (!s.connection || !s.sql?.trim())) return void (error = 'Choisissez une connexion et une table (ou écrivez une requête SQL).');
		if (s.kind === 'sql' && !s.sql?.trim()) return void (error = 'Écrivez une requête SQL.');
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
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') oncancel();
		}}
	>
		<header>Source</header>
		<div class="body">
			<label class="field">Type de source <Select value={s.kind} options={KINDS} width="100%" onchange={changeKind} /></label>
			{#if s.kind === 'csv' || s.kind === 'xlsx'}
				<div class="field">
					Fichier
					<div class="row">
						<input class="grow" bind:value={s.path} onchange={() => s.kind === 'xlsx' && loadXlsx()} placeholder="Chemin du fichier" />
						<button onclick={browse}>Parcourir…</button>
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
				{:else}
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
				{/if}
			{:else if s.kind === 'table'}
				<label class="field">Tableau <Select value={s.table ?? ''} options={tables.map((t) => ({ value: t, label: t }))} width="100%" onchange={(v) => (s.table = v)} /></label>
			{:else if s.kind === 'query'}
				<label class="field">Requête <Select value={s.query ?? ''} options={queries.map((t) => ({ value: t, label: t }))} width="100%" onchange={(v) => (s.query = v)} /></label>
				<p class="muted">La requête part du résultat de l'autre requête (référence) : modifier l'autre requête change aussi celle-ci.</p>
			{:else if s.kind === 'postgres'}
				<div class="field">
					Connexion
					<div class="row">
						<Select value={s.connection ?? ''} options={conns.map((c) => ({ value: c.name, label: `${c.name} (${c.postgres.user}@${c.postgres.host}/${c.postgres.database})` }))} width="100%" onchange={(v) => ((s.connection = v), loadPgTables())} />
						<button onclick={addConnection}>Nouvelle…</button>
					</div>
				</div>
				{#if pgTables.length}
					<div class="field">
						Tables et vues <input placeholder="Rechercher" bind:value={pgFilter} />
						<div class="list">
							{#each shownTables as t (t.schema + '.' + t.name)}
								<button class="item" onclick={() => pickTable(t)}>{t.schema}.<strong>{t.name}</strong>{t.view ? ' (vue)' : ''}</button>
							{/each}
						</div>
					</div>
				{/if}
				<label class="field">Requête SQL (lecture seule) <textarea rows="5" bind:value={s.sql} placeholder="SELECT * FROM public.ventes WHERE annee = 2026"></textarea></label>
				<p class="muted">Filtrez de préférence ici : la requête est exécutée par le serveur, les étapes ensuite par Slitable.</p>
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
		z-index: 60;
		background: rgba(0, 0, 0, 0.25);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.dialog {
		background: #fff;
		border: 1px solid #c9cdd2;
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
