<script lang="ts">
	// Éditeur de requête, comme l'éditeur Power Query d'Excel : un ruban
	// d'étapes en haut, l'aperçu au centre (sur les 1 000 premières lignes de
	// chaque source), les étapes appliquées à droite. Cliquer une étape montre
	// l'aperçu à cette étape ; une nouvelle étape s'insère après celle qui est
	// choisie. « Fermer et charger » enregistre la requête et charge son
	// résultat dans un tableau de feuille.
	import { onMount } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { Column, PasswordNeed, Preview, Query, Source, Step } from '#bindings/internal/query/models.js';
	import StepForm from './StepForm.svelte';
	import SourceForm from './SourceForm.svelte';
	import { OPS, clone, defaultStep, insertStep, moveStep, newQuery, otherSource, typeName, type Op } from './steps';

	interface Props {
		query: Query;
		/** Ouvrir d'abord la boîte de la source (nouvelle requête à préciser). */
		editSource?: boolean;
		queries: string[];
		tables: string[];
		askPassword: (need: PasswordNeed) => Promise<boolean>;
		newConnection: () => Promise<string | null>;
		/** Enregistrer (et charger si load) ; renvoie un message d'erreur ou "". */
		onfinish: (q: Query, load: boolean) => Promise<string>;
		onclose: () => void;
	}
	let { query, editSource = false, queries, tables, askPassword, newConnection, onfinish, onclose }: Props = $props();

	const initial = (): Query => ({ ...clone(query), steps: clone(query.steps ?? []) });
	let q = $state<Query>(initial());
	// Étape affichée : -1 pour la source, sinon le rang de l'étape.
	let sel = $state(initial().steps!.length - 1);
	let preview = $state<Preview | null>(null);
	let loading = $state(false);
	let selCol = $state('');
	let form = $state<{ step: Step; cols: Column[]; edit: number | null } | null>(null);
	const initialSource = () => editSource;
	let sourceOpen = $state(initialSource());
	let error = $state('');
	let busy = $state(false);
	let showSQL = $state(false);
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);

	const otherQueries = $derived(queries.filter((n) => n.toLowerCase() !== q.name.toLowerCase()));
	const others = $derived([...otherQueries.map((n) => ({ key: 'query:' + n, label: 'Requête ' + n })), ...tables.map((n) => ({ key: 'table:' + n, label: 'Tableau ' + n }))]);

	let seq = 0;
	async function refresh(fresh = false) {
		const n = ++seq;
		loading = true;
		try {
			let pv = await DocumentService.PreviewQuery(q, sel + 1, fresh);
			if (pv.needPassword && (await askPassword(pv.needPassword))) pv = await DocumentService.PreviewQuery(q, sel + 1, fresh);
			if (n === seq) preview = pv;
		} catch (e) {
			if (n === seq) error = err(e);
		} finally {
			if (n === seq) loading = false;
		}
	}
	onMount(() => {
		if (!editSource) refresh();
	});

	function select(i: number) {
		sel = i;
		refresh();
	}

	async function columnsAt(i: number): Promise<Column[]> {
		if (i === sel + 1 && preview && preview.errorStep < 0) return preview.columns ?? [];
		const pv = await DocumentService.PreviewQuery(q, i, false);
		return pv.columns ?? [];
	}

	async function addStep(op: Op) {
		error = '';
		const cols = await columnsAt(sel + 1);
		form = { step: defaultStep(op, cols, selCol && cols.some((c) => c.name === selCol) ? selCol : undefined), cols, edit: null };
	}

	async function editStep(i: number) {
		const cols = await columnsAt(i);
		form = { step: clone(q.steps![i]), cols, edit: i };
	}

	function applyForm(s: Step) {
		const f = form!;
		form = null;
		if (f.edit !== null) {
			q.steps = q.steps!.map((x, k) => (k === f.edit ? s : x));
			sel = f.edit;
		} else {
			q.steps = insertStep(q.steps ?? [], sel, s);
			sel = sel + 1;
		}
		refresh();
	}

	function removeStep(i: number) {
		q.steps = (q.steps ?? []).filter((_, k) => k !== i);
		if (sel >= i) sel = Math.max(-1, sel - 1);
		refresh();
	}

	function move(i: number, d: number) {
		q.steps = moveStep(q.steps ?? [], i, d);
		if (sel === i) sel = i + d;
		refresh();
	}

	function setSource(s: Source) {
		sourceOpen = false;
		q.source = s;
		refresh(true);
	}

	async function otherColumns(key: string): Promise<Column[]> {
		const pv = await DocumentService.PreviewQuery(newQuery('__autre', otherSource(key)), -1, false);
		if (pv.error) throw new Error(pv.error);
		return pv.columns ?? [];
	}

	async function finish(load: boolean) {
		error = '';
		if (!q.name.trim()) return void (error = 'Donnez un nom à la requête.');
		busy = true;
		try {
			const msg = await onfinish(clone(q), load);
			if (msg) error = msg;
		} finally {
			busy = false;
		}
	}

	const labels = $derived(preview?.labels ?? ['Source', ...(q.steps ?? []).map((s) => s.op)]);
	const groups = ['Colonnes', 'Lignes', 'Transformer', 'Combiner'] as const;
</script>

<div class="editor" role="dialog" aria-modal="true" aria-label="Éditeur de requête">
	<header class="title">
		<span class="app">Éditeur de requête</span>
		<span class="name">{q.name}</span>
		<span class="grow"></span>
		<button class="close" title="Fermer sans enregistrer" onclick={onclose}>✕</button>
	</header>
	<nav class="ribbon">
		<div class="group">
			<div class="cmds">
				<button class="primary big" disabled={busy} onclick={() => finish(true)} title="Enregistrer la requête et charger son résultat dans un tableau">Fermer et charger</button>
				<button disabled={busy} onclick={() => finish(false)} title="Enregistrer la requête sans la charger (connexion seulement)">Enregistrer</button>
				<button onclick={() => refresh(true)} title="Relire les sources">Actualiser l'aperçu</button>
			</div>
			<span class="gt">Requête</span>
		</div>
		{#each groups as g (g)}
			<div class="group">
				<div class="cmds">
					{#each OPS.filter((o) => o.group === g) as o (o.op)}
						<button title={o.hint} onclick={() => addStep(o.op)}>{o.label}</button>
					{/each}
				</div>
				<span class="gt">{g}</span>
			</div>
		{/each}
	</nav>
	<div class="main">
		<section class="preview">
			{#if preview?.error}
				<div class="err">
					<strong>{preview.errorStep === -1 ? 'Source' : `Étape ${preview.errorStep + 1} (${labels[preview.errorStep + 1] ?? ''})`}</strong> : {preview.error}
					{#if preview.errorStep >= 0 && preview.step < (q.steps ?? []).length}<span class="muted"> — aperçu de l'étape précédente</span>{/if}
				</div>
			{/if}
			<div class="table-wrap" class:dim={loading}>
				{#if preview && (preview.columns ?? []).length}
					<table>
						<thead>
							<tr>
								<th class="num"></th>
								{#each preview.columns ?? [] as c (c.name)}
									<th class:on={selCol === c.name} onclick={() => (selCol = selCol === c.name ? '' : c.name)} title="{c.name} : {typeName(c.type)}">
										<span class="type">{typeName(c.type)}</span>
										{c.name}
									</th>
								{/each}
							</tr>
						</thead>
						<tbody>
							{#each preview.rows ?? [] as r, i (i)}
								<tr>
									<td class="num">{i + 1}</td>
									{#each r as v, j (j)}
										<td class:on={selCol === preview.columns?.[j]?.name} class:right={preview.columns?.[j]?.type === 'integer' || preview.columns?.[j]?.type === 'decimal'}>{v}</td>
									{/each}
								</tr>
							{/each}
						</tbody>
					</table>
				{:else if loading}
					<p class="muted pad">Calcul de l'aperçu…</p>
				{:else if !preview?.error}
					<p class="muted pad">Aucune donnée.</p>
				{/if}
			</div>
			<footer class="status">
				{#if preview}
					{(preview.columns ?? []).length} colonne(s), {preview.count.toLocaleString('fr-FR')} ligne(s){preview.sample ? ' — aperçu calculé sur les 1 000 premières lignes de chaque source' : ''}{preview.count > (preview.rows ?? []).length ? ` (${(preview.rows ?? []).length} affichées)` : ''}
					<button class="link" onclick={() => (showSQL = !showSQL)}>{showSQL ? 'Masquer' : 'Voir'} le SQL</button>
				{/if}
			</footer>
			{#if showSQL && preview?.sql}<pre class="sql">{preview.sql}</pre>{/if}
		</section>
		<aside class="side">
			<h3>Propriétés</h3>
			<label class="field">Nom <input bind:value={q.name} /></label>
			<h3>Étapes appliquées</h3>
			<ol class="steps">
				<li class:on={sel === -1} class:bad={preview?.errorStep === -1}>
					<button class="step" onclick={() => select(-1)}>{labels[0]}</button>
					<button class="x" title="Modifier la source" onclick={() => (sourceOpen = true)}>⚙</button>
				</li>
				{#each q.steps ?? [] as s, i (i)}
					<li class:on={sel === i} class:bad={preview?.errorStep === i}>
						<button class="step" onclick={() => select(i)}>{labels[i + 1] ?? s.op}</button>
						<button class="x" title="Modifier l'étape" onclick={() => editStep(i)}>⚙</button>
						<button class="x" title="Monter" disabled={i === 0} onclick={() => move(i, -1)}>↑</button>
						<button class="x" title="Descendre" disabled={i === (q.steps ?? []).length - 1} onclick={() => move(i, 1)}>↓</button>
						<button class="x" title="Supprimer l'étape" onclick={() => removeStep(i)}>✕</button>
					</li>
				{/each}
			</ol>
			<p class="muted">Une nouvelle étape s'insère après l'étape choisie. Cliquez un en-tête de colonne pour la proposer dans la prochaine étape.</p>
			{#if error}<p class="error">{error}</p>{/if}
		</aside>
	</div>
</div>

{#if form}
	<StepForm step={form.step} cols={form.cols} {others} {otherColumns} onok={applyForm} oncancel={() => (form = null)} />
{/if}
{#if sourceOpen}
	<SourceForm
		source={q.source}
		queries={otherQueries}
		{tables}
		{newConnection}
		{askPassword}
		onok={setSource}
		oncancel={() => {
			sourceOpen = false;
			if (!preview) refresh();
		}}
	/>
{/if}

<style>
	.editor {
		position: fixed;
		inset: 0;
		z-index: 50;
		background: #fff;
		display: flex;
		flex-direction: column;
		font-size: 12px;
		color: #1f2328;
	}
	.title {
		display: flex;
		align-items: center;
		gap: 10px;
		height: 32px;
		padding: 0 4px 0 12px;
		background: #217346;
		color: #fff;
	}
	.title .app {
		font-weight: 600;
	}
	.title .name {
		opacity: 0.9;
	}
	.grow {
		flex: 1;
	}
	.close {
		background: none;
		border: none;
		color: #fff;
		font-size: 14px;
		width: 32px;
		height: 28px;
	}
	.close:hover {
		background: rgba(255, 255, 255, 0.2);
	}
	.ribbon {
		display: flex;
		gap: 0;
		border-bottom: 1px solid #d4d7db;
		background: #f7f8f9;
		padding: 4px 4px 0;
		overflow-x: auto;
	}
	.group {
		display: flex;
		flex-direction: column;
		border-right: 1px solid #e1e4e8;
		padding: 0 6px;
	}
	.cmds {
		display: flex;
		flex-wrap: wrap;
		gap: 3px;
		max-width: 420px;
		align-content: flex-start;
		min-height: 52px;
	}
	.gt {
		text-align: center;
		color: #6b7280;
		font-size: 11px;
		padding: 2px 0;
	}
	button {
		font: inherit;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		padding: 2px 8px;
		cursor: pointer;
		height: 24px;
	}
	button:hover:not(:disabled) {
		border-color: #217346;
		background: #eef6f0;
	}
	button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
	button.primary:hover:not(:disabled) {
		background: #1b5e39;
	}
	.main {
		flex: 1;
		display: flex;
		min-height: 0;
	}
	.preview {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.err {
		padding: 6px 10px;
		background: #fdecea;
		color: #b42318;
		border-bottom: 1px solid #f4c7c3;
	}
	.table-wrap {
		flex: 1;
		overflow: auto;
	}
	.table-wrap.dim {
		opacity: 0.55;
	}
	table {
		border-collapse: collapse;
		font-variant-numeric: tabular-nums;
	}
	th,
	td {
		border: 1px solid #e1e4e8;
		padding: 2px 6px;
		white-space: nowrap;
		max-width: 260px;
		overflow: hidden;
		text-overflow: ellipsis;
		text-align: left;
	}
	th {
		position: sticky;
		top: 0;
		background: #f3f4f6;
		font-weight: 600;
		cursor: pointer;
	}
	th .type {
		display: block;
		font-weight: 400;
		font-size: 10px;
		color: #6b7280;
	}
	th.on,
	td.on {
		background: #e3efe7;
	}
	td.right {
		text-align: right;
	}
	.num {
		color: #9aa0a6;
		background: #f7f8f9;
		text-align: right;
		position: sticky;
		left: 0;
	}
	.status {
		border-top: 1px solid #e1e4e8;
		padding: 3px 10px;
		color: #4b5563;
		display: flex;
		gap: 8px;
		align-items: center;
	}
	.link {
		border: none;
		background: none;
		color: #217346;
		padding: 0;
		height: auto;
	}
	.sql {
		margin: 0;
		max-height: 160px;
		overflow: auto;
		background: #f7f8f9;
		border-top: 1px solid #e1e4e8;
		padding: 6px 10px;
		font-size: 11px;
	}
	.side {
		width: 280px;
		flex: none;
		border-left: 1px solid #d4d7db;
		background: #fafbfb;
		padding: 8px 10px;
		overflow: auto;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	h3 {
		margin: 4px 0 0;
		font-size: 12px;
		font-weight: 600;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	input {
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 3px 6px;
	}
	.steps {
		list-style: none;
		margin: 0;
		padding: 0;
		border: 1px solid #e5e7eb;
		background: #fff;
	}
	.steps li {
		display: flex;
		align-items: center;
	}
	.steps li.on {
		background: #d9eadf;
	}
	.steps li.bad .step {
		color: #b42318;
	}
	.steps li.bad .step::before {
		content: '⚠ ';
	}
	.step {
		flex: 1;
		text-align: left;
		border: none;
		background: none;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.x {
		border: none;
		background: none;
		padding: 0 3px;
		color: #4b5563;
	}
	.muted {
		color: #6b7280;
		margin: 0;
	}
	.pad {
		padding: 12px;
	}
	.error {
		color: #b42318;
		margin: 0;
	}
</style>
