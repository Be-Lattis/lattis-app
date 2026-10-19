<script lang="ts">
	// Volet « Regrouper des onglets » (docs/conception/regrouper-onglets-
	// semblables.md, §5) : les onglets cochés, la colonne de catégorie (nom
	// pré-rempli et modifiable, type, valeur tirée de chaque onglet, corrigible),
	// la correspondance des colonnes, ce que devient le résultat et les anciens
	// onglets, un aperçu des premières lignes, puis Regrouper (une seule étape
	// d'annulation). Les avertissements (colonne appariée, clé retirée,
	// formules à reprendre) sont en orange, comme dans la barre d'état. Avant
	// de valider, le volet dit ce que deviennent les formules qui citaient
	// les anciens onglets : reprises sur le tableau commun filtré (avant →
	// après) ou à reprendre à la main (§8).
	import { untrack } from 'svelte';
	import type { MergePlan, MergePreview } from '#bindings/internal/canvas/models.js';
	import { resizePanel } from '#lib/ui/resize.js';
	import { panelKeys } from '#lib/ui/focus.js';
	import PanelHeader from '#lib/ui/PanelHeader.svelte';
	import { cellText, oldHint, oldLabel, previewWarnings, refCounts, withTables } from './mergesheets';

	interface Props {
		/** Plan de départ (groupe détecté, ou sélection d'onglets). */
		plan: MergePlan;
		/** Autres tableaux du classeur, qu'on peut ajouter (nom et feuille). */
		others?: { table: string; sheet: string }[];
		/** Aperçu d'un plan (le moteur complète ce qui manque). */
		preview: (p: MergePlan) => Promise<MergePreview>;
		/** Regroupe ; rend faux si c'est refusé (le volet reste ouvert). */
		onmerge: (p: MergePlan) => Promise<boolean>;
		onclose: () => void;
	}
	let { plan, others = [], preview, onmerge, onclose }: Props = $props();

	// Plan de départ : les décochages s'y rapportent (withTables).
	const base = untrack(() => structuredClone($state.snapshot(plan)) as MergePlan);
	const baseTables = base.tables ?? [];
	// Tableaux proposés (ceux du plan, puis ceux ajoutés) et cochés.
	let listed = $state<string[]>([...baseTables]);
	let checked = $state<Set<string>>(new Set(baseTables));
	let added = $state('');
	// Ce que l'utilisateur a changé : nom et type de la catégorie, valeurs
	// corrigées (par tableau), sans catégorie, nom, feuille, anciens onglets.
	let catName = $state(base.category?.name ?? '');
	let catType = $state<string>(base.category?.type ?? 'text');
	let catFormat = $state(base.category?.format ?? '');
	// Catégorie encore à déduire (plan sans catégorie, rien de touché) : le
	// moteur la propose, le volet la reprend.
	let catAuto = $state(!base.category);
	let values = $state<Record<string, string>>(Object.fromEntries(baseTables.map((t, i) => [t, base.category?.values?.[i] ?? ''])));
	let noCat = $state(!!base.noCategory);
	let name = $state(base.name ?? '');
	let sheet = $state(base.sheet ?? '');
	let old = $state<'' | 'delete' | 'hide'>('');

	let view = $state<MergePreview | null>(null);
	let error = $state('');
	let busy = $state(false);

	const chosen = $derived(listed.filter((t) => checked.has(t)));

	/** Le plan tel que réglé dans le volet. */
	function current(): MergePlan {
		const tables = chosen;
		const keep = tables.map((t) => baseTables.indexOf(t));
		let p: MergePlan;
		if (keep.every((i) => i >= 0)) p = withTables(base, keep);
		else p = { tables, name: '' }; // un tableau ajouté : correspondance et catégorie déduites à nouveau
		const known = tables.map((t) => values[t] ?? '');
		p.noCategory = noCat;
		p.category = noCat || catAuto ? null : { name: catName.trim(), type: catType as never, values: known.every((v) => v !== '') ? known : [], format: catType === 'date' ? catFormat : '' };
		p.name = name.trim();
		p.sheet = sheet.trim();
		p.old = old;
		return p;
	}

	// L'aperçu suit les réglages, après un court délai.
	let seq = 0;
	$effect(() => {
		const p = current();
		const n = ++seq;
		const timer = setTimeout(async () => {
			if (chosen.length < 2) {
				view = null;
				error = 'Cochez au moins deux onglets';
				return;
			}
			try {
				const v = await preview(p);
				if (n !== seq) return;
				view = v;
				error = '';
				// Catégorie déduite, ou valeurs d'un tableau ajouté : reprises dans les cases.
				const cat = v.plan.category;
				if (cat && catAuto) {
					catName = cat.name;
					catType = cat.type;
					catFormat = cat.format ?? '';
				}
				(v.plan.tables ?? []).forEach((t, i) => {
					if (cat && (catAuto || !values[t])) values[t] = cat.values?.[i] ?? '';
				});
				if (!name) name = v.plan.name ?? '';
			} catch (e) {
				if (n === seq) error = e instanceof Error ? e.message : String(e);
			}
		}, 250);
		return () => clearTimeout(timer);
	});

	const warnings = $derived(view ? previewWarnings(view) : []);
	const effectiveOld = $derived(old || view?.old || 'delete');
	const counts = $derived(view ? refCounts(view) : '');
	const hint = $derived(oldHint(view, effectiveOld));

	function toggle(t: string) {
		const s = new Set(checked);
		if (s.has(t)) s.delete(t);
		else s.add(t);
		checked = s;
	}

	function addTable() {
		const t = added;
		added = '';
		if (!t || listed.includes(t)) return;
		listed = [...listed, t];
		checked = new Set([...checked, t]);
	}

	async function run() {
		if (!view || view.refused || busy) return;
		busy = true;
		try {
			if (await onmerge(current())) onclose();
		} finally {
			busy = false;
		}
	}

	/** Case de la grille des colonnes : ✓, ∅ ou « Montant € → Montant ». */
	function mark(from: string, paired: boolean, col: string): string {
		if (!from) return '∅';
		if (paired || from.toLowerCase() !== col.toLowerCase()) return `${from} → ${col}`;
		return '✓';
	}
	const sheetOf = (t: string) => view?.tables?.find((x) => x.table === t)?.sheetName ?? others.find((o) => o.table === t)?.sheet ?? '';
	const rowsOf = (t: string) => view?.tables?.find((x) => x.table === t)?.rows;
</script>

<aside class="panel" use:resizePanel={{ key: 'mergesheets' }} use:panelKeys={onclose} aria-label="Regrouper des onglets">
	<PanelHeader title="Regrouper des onglets" help="prise-en-main#regrouper-des-onglets" {onclose}>
		<span class="sub">Un seul tableau, avec une colonne de catégorie</span>
	</PanelHeader>

	<section>
		<h3>1. Onglets</h3>
		<ul class="tables">
			{#each listed as t (t)}
				<li>
					<label>
						<input type="checkbox" checked={checked.has(t)} onchange={() => toggle(t)} />
						<span class="sheet">{sheetOf(t) || t}</span>
						<span class="muted">· {t}{rowsOf(t) !== undefined ? ` · ${rowsOf(t)} ligne${rowsOf(t) === 1 ? '' : 's'}` : ''}</span>
					</label>
				</li>
			{/each}
		</ul>
		{#if others.some((o) => !listed.includes(o.table))}
			<div class="row">
				<select bind:value={added} aria-label="Ajouter un tableau">
					<option value="">Ajouter un tableau…</option>
					{#each others.filter((o) => !listed.includes(o.table)) as o (o.table)}
						<option value={o.table}>{o.sheet} · {o.table}</option>
					{/each}
				</select>
				<button disabled={!added} onclick={addTable}>Ajouter</button>
			</div>
		{/if}
	</section>

	<section>
		<h3>2. Colonne de catégorie</h3>
		<label class="check"><input type="checkbox" bind:checked={noCat} /> Ne pas ajouter de colonne (la catégorie est déjà dans les données)</label>
		{#if !noCat}
			<div class="row">
				<label>Nom <input class="name" bind:value={catName} oninput={() => (catAuto = false)} aria-label="Nom de la colonne de catégorie" /></label>
				<label
					>Type
					<select bind:value={catType} onchange={() => (catAuto = false)} aria-label="Type de la colonne de catégorie">
						<option value="text">Texte</option>
						<option value="integer">Nombre entier</option>
						<option value="date">Date</option>
					</select></label
				>
			</div>
			<p class="muted">Valeur tirée du nom de chaque onglet, à corriger au besoin :</p>
			<table class="values">
				<tbody>
					{#each chosen as t (t)}
						<tr>
							<td>{sheetOf(t) || t}</td>
							<td><input bind:value={values[t]} oninput={() => (catAuto = false)} aria-label="Valeur de {sheetOf(t) || t}" /></td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</section>

	{#if view}
		<section>
			<h3>3. Colonnes</h3>
			<div class="scroll">
				<table class="grid">
					<thead>
						<tr>
							<th>Colonne</th>
							<th>Type</th>
							{#each view.tables ?? [] as t (t.table)}<th>{t.sheetName}</th>{/each}
						</tr>
					</thead>
					<tbody>
						{#each view.plan.columns ?? [] as c (c.name)}
							<tr>
								<td>{c.name}</td>
								<td class:warn={c.text}>{c.type}{c.text ? ' ⚠' : ''}</td>
								{#each c.from ?? [] as f, i (i)}
									<td class:paired={!!c.paired?.[i]} class:absent={!f}>{mark(f, !!c.paired?.[i], c.name)}</td>
								{/each}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</section>
	{/if}

	<section>
		<h3>4. Après</h3>
		<div class="row">
			<label>Tableau <input class="name" bind:value={name} aria-label="Nom du tableau commun" /></label>
			<label>Feuille <input class="name" bind:value={sheet} placeholder={name} aria-label="Nom de la nouvelle feuille" /></label>
		</div>
		<fieldset>
			<legend>Anciens onglets</legend>
			{#each ['delete', 'hide'] as o (o)}
				<label class="check"><input type="radio" name="old" value={o} checked={effectiveOld === o} onchange={() => (old = o as 'delete' | 'hide')} /> {oldLabel(o)}</label>
			{/each}
			{#if hint}<p class="hint">{hint}</p>{/if}
		</fieldset>
	</section>

	{#if warnings.length || error}
		<ul class="warnings" role="alert">
			{#if error}<li>{error}</li>{/if}
			{#each warnings as w (w)}<li>{w}</li>{/each}
		</ul>
	{/if}
	{#if counts}
		<p class="counts">{counts}</p>
	{/if}
	{#if view?.rewritten?.length}
		<details>
			<summary>Formules reprises ({view.rewritten.length})</summary>
			<ul class="pending">
				{#each view.rewritten as r, i (i)}<li><strong>{r.where}</strong> <code>{r.formula}</code> → <code>{r.after}</code></li>{/each}
			</ul>
		</details>
	{/if}
	{#if view?.pending?.length}
		<details open>
			<summary>À reprendre à la main ({view.pending.length})</summary>
			<ul class="pending">
				{#each view.pending as p, i (i)}<li><strong>{p.where}</strong> <code>{p.formula}</code>{#if p.why}<span class="why"> : {p.why}</span>{/if}</li>{/each}
			</ul>
		</details>
	{/if}
	{#if view?.notes?.length}
		<ul class="notes">
			{#each view.notes as n (n)}<li>{n}</li>{/each}
		</ul>
	{/if}

	{#if view}
		<section>
			<h3>5. Aperçu</h3>
			<div class="scroll">
				<table class="grid">
					<thead>
						<tr>{#each view.header ?? [] as h, i (i)}<th>{h}</th>{/each}</tr>
					</thead>
					<tbody>
						{#each view.rows ?? [] as r, i (i)}
							<tr>{#each r ?? [] as v, k (k)}<td>{cellText(v)}</td>{/each}</tr>
						{/each}
					</tbody>
				</table>
			</div>
			<p class="muted">{view.total.toLocaleString('fr-FR')} ligne{view.total === 1 ? '' : 's'} au total{view.key ? (view.keyKept ? ` · clé : ${view.key}` : '') : ''}</p>
		</section>
	{/if}

	<div class="acts">
		<button class="primary" disabled={!view || !!view.refused || busy || chosen.length < 2} onclick={run}>Regrouper</button>
		<button onclick={onclose}>Annuler</button>
	</div>
</aside>

<style>
	.panel {
		position: relative;
		flex: none;
		display: flex;
		flex-direction: column;
		gap: 8px;
		min-height: 0;
		padding: 8px 10px 10px;
		border-left: 1px solid #d4d7db;
		background: #fafbfb;
		box-shadow: -4px 0 12px rgba(0, 0, 0, 0.08);
		overflow: auto;
		font-size: 12px;
		color: #1f2328;
	}
	.sub,
	.muted {
		color: #6b7280;
		margin: 0;
	}
	h3 {
		margin: 0 0 4px;
		font-size: 12px;
		font-weight: 600;
	}
	section {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.tables {
		list-style: none;
		margin: 0;
		padding: 0;
		max-height: 180px;
		overflow: auto;
	}
	.tables .sheet {
		font-weight: 600;
	}
	.row {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
	}
	.check {
		display: flex;
		gap: 6px;
		align-items: center;
	}
	input.name {
		width: 140px;
	}
	input,
	select,
	button {
		font: inherit;
	}
	.values td {
		padding: 1px 6px 1px 0;
	}
	.values input {
		width: 160px;
	}
	.scroll {
		overflow: auto;
		max-height: 220px;
		border: 1px solid #e3e5e8;
		background: #fff;
	}
	table.grid {
		border-collapse: collapse;
		white-space: nowrap;
	}
	.grid th,
	.grid td {
		border: 1px solid #eceef0;
		padding: 2px 6px;
		text-align: left;
	}
	.grid th {
		background: #f3f4f6;
		font-weight: 600;
	}
	.grid td.absent {
		color: #9ca3af;
	}
	.grid td.paired,
	.grid td.warn {
		color: #8a4b00;
		background: #fff4e0;
	}
	fieldset {
		border: none;
		margin: 4px 0 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	legend {
		padding: 0;
		font-weight: 600;
		margin-bottom: 2px;
	}
	.warnings {
		margin: 0;
		padding: 6px 8px 6px 22px;
		background: #ffb02e;
		color: #3d2600;
		border-radius: 3px;
		box-shadow: 0 0 0 1px #e08a00;
		font-weight: 600;
	}
	.notes,
	.pending {
		margin: 0;
		padding-left: 18px;
		color: #4b5563;
	}
	.pending code {
		font-size: 11px;
	}
	.pending li {
		margin-bottom: 3px;
	}
	.why {
		color: #92400e;
	}
	.counts {
		margin: 0;
		font-weight: 600;
	}
	.hint {
		margin: 4px 0 0;
		color: #92400e;
		font-size: 11px;
	}
	.acts {
		display: flex;
		gap: 8px;
		margin-top: 4px;
	}
	.acts button {
		padding: 4px 12px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.acts button.primary {
		background: #217346;
		color: #fff;
		border-color: #217346;
	}
	.acts button:disabled {
		opacity: 0.55;
		cursor: default;
	}
</style>
