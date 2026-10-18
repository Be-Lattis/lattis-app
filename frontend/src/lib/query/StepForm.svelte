<script lang="ts">
	// Paramètres d'une étape de requête (boîte de dialogue de l'éditeur),
	// comme les boîtes de Power Query : Filtrer les lignes, Regrouper par,
	// Fusionner…
	import { modal } from '#lib/ui/focus.js';
	import Select from '#lib/ui/Select.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import type { Column, Step } from '#bindings/internal/query/models.js';
	import { AGGS, CMPS, JOINS, OPS, TYPES, clone, otherKey, otherSource, type Op } from './steps';

	interface Props {
		step: Step;
		/** Colonnes à l'entrée de l'étape. */
		cols: Column[];
		/** Autres requêtes et tableaux du classeur (fusion, ajout) : clé « query:Nom » ou « table:Nom ». */
		others: { key: string; label: string }[];
		/** Colonnes d'une autre requête ou d'un tableau. */
		otherColumns: (key: string) => Promise<Column[]>;
		onok: (s: Step) => void;
		oncancel: () => void;
	}
	let { step, cols, others, otherColumns, onok, oncancel }: Props = $props();

	const initial = () => clone(step);
	let s = $state<Step>(initial());
	const info = $derived(OPS.find((o) => o.op === s.op));
	const names = $derived(cols.map((c) => ({ value: c.name, label: c.name })));
	let error = $state('');

	// Colonnes de l'autre côté (fusion).
	let theirs = $state<Column[]>([]);
	const initialOther = () => otherKey(step.other) || others[0]?.key || '';
	let other = $state(initialOther());
	$effect(() => {
		const k = other;
		if ((s.op !== 'merge' && s.op !== 'append') || !k) return;
		s.other = otherSource(k);
		otherColumns(k).then(
			(cs) => {
				theirs = cs;
				if (s.op === 'merge' && s.otherKeys && s.otherKeys.length && !s.otherKeys[0]) {
					const same = cs.find((c) => c.name === s.keys?.[0]) ?? cs[0];
					s.otherKeys = [same?.name ?? ''];
				}
			},
			() => (theirs = [])
		);
	});
	const theirNames = $derived(theirs.map((c) => ({ value: c.name, label: c.name })));

	function toggle(list: string[] | null | undefined, name: string, on: boolean): string[] {
		const l = (list ?? []).filter((n) => n !== name);
		if (on) l.push(name);
		return l;
	}
	/** Garde l'ordre des colonnes de l'entrée pour une liste cochée. */
	const ordered = (list: string[] | null | undefined) => cols.map((c) => c.name).filter((n) => (list ?? []).includes(n));

	function ok() {
		error = '';
		const st = clone(s);
		switch (st.op as Op) {
			case 'remove':
			case 'unpivot':
				st.columns = ordered(st.columns);
				if (!st.columns.length) return void (error = 'Cochez au moins une colonne.');
				break;
			case 'select':
				if (!st.columns?.length) return void (error = 'Cochez au moins une colonne.');
				break;
			case 'group':
				st.columns = ordered(st.columns);
				break;
			case 'distinct':
				st.columns = ordered(st.columns);
				break;
			case 'rename':
				if (!st.name?.trim()) return void (error = 'Donnez un nom.');
				break;
			case 'addcolumn':
				if (!st.name?.trim() || !st.expr?.trim()) return void (error = 'Donnez un nom et une expression.');
				break;
			case 'merge':
				if (!st.other) return void (error = 'Choisissez la requête ou le tableau à fusionner.');
				if ((st.keys ?? []).some((k) => !k) || (st.otherKeys ?? []).some((k) => !k)) return void (error = 'Choisissez les colonnes de correspondance des deux côtés.');
				break;
			case 'append':
				if (!st.other) return void (error = 'Choisissez la requête ou le tableau à ajouter.');
				break;
		}
		onok(st);
	}
</script>

<div class="back">
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label={info?.label}
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') oncancel();
			if (e.key === 'Enter' && (e.target as HTMLElement).tagName !== 'TEXTAREA') ok();
		}}
	>
		<header>{info?.label}</header>
		<p class="hint">{info?.hint}</p>
		<div class="body">
			{#if s.op === 'filter'}
				{#each s.conditions ?? [] as c, i (i)}
					<div class="row">
						{#if i > 0}<span class="conj">{s.any ? 'Ou' : 'Et'}</span>{/if}
						<Select bind:value={c.column} options={names} width="150px" />
						<Select bind:value={c.cmp} options={CMPS.map(([v, label]) => ({ value: v, label }))} width="170px" />
						{#if c.cmp !== 'empty' && c.cmp !== 'notempty'}<input bind:value={c.value} placeholder="Valeur" />{/if}
						{#if (s.conditions ?? []).length > 1}<button class="x" title="Retirer" onclick={() => (s.conditions = (s.conditions ?? []).filter((_, k) => k !== i))}>✕</button>{/if}
					</div>
				{/each}
				<div class="row">
					<button class="ico" onclick={() => (s.conditions = [...(s.conditions ?? []), { column: cols[0]?.name ?? '', cmp: '=', value: '' }])}><Icon name="plus" size={14} />Ajouter une condition</button>
					<label><input type="radio" checked={!s.any} onchange={() => (s.any = false)} /> Et</label>
					<label><input type="radio" checked={!!s.any} onchange={() => (s.any = true)} /> Ou</label>
				</div>
				<p class="hint">Les nombres et les dates s'écrivent comme dans la grille (3,5 ; 31/12/2026). Contient, commence par et se termine par ne tiennent pas compte de la casse.</p>
			{:else if s.op === 'rename'}
				<label class="field">Colonne <Select bind:value={s.column!} options={names} width="100%" /></label>
				<label class="field">Nouveau nom <input bind:value={s.name} /></label>
			{:else if s.op === 'type'}
				<label class="field">Colonne <Select bind:value={s.column!} options={names} width="100%" /></label>
				<label class="field">Type <Select value={s.type ?? 'text'} options={TYPES.map(([v, label]) => ({ value: v, label }))} width="100%" onchange={(v) => (s.type = v as Step['type'])} /></label>
			{:else if s.op === 'remove' || s.op === 'unpivot' || s.op === 'distinct'}
				<div class="checks">
					{#each cols as c (c.name)}
						<label><input type="checkbox" checked={(s.columns ?? []).includes(c.name)} onchange={(e) => (s.columns = toggle(s.columns, c.name, (e.currentTarget as HTMLInputElement).checked))} /> {c.name}</label>
					{/each}
				</div>
				{#if s.op === 'distinct'}<p class="hint">Aucune colonne cochée : une ligne est un doublon si toutes ses valeurs sont identiques.</p>{/if}
				{#if s.op === 'unpivot'}
					<label class="field">Colonne des attributs <input bind:value={s.name} /></label>
					<label class="field">Colonne des valeurs <input bind:value={s.valueName} /></label>
				{/if}
			{:else if s.op === 'select'}
				<p class="hint">Cochez les colonnes à garder ; ↑ ↓ changent leur ordre.</p>
				<div class="checks">
					{#each [...(s.columns ?? []), ...cols.map((c) => c.name).filter((n) => !(s.columns ?? []).includes(n))] as n (n)}
						{@const i = (s.columns ?? []).indexOf(n)}
						<div class="sel">
							<label><input type="checkbox" checked={i >= 0} onchange={(e) => (s.columns = toggle(s.columns, n, (e.currentTarget as HTMLInputElement).checked))} /> {n}</label>
							{#if i >= 0}
								<button class="x" disabled={i === 0} title="Monter" onclick={() => { const l = [...(s.columns ?? [])]; [l[i - 1], l[i]] = [l[i], l[i - 1]]; s.columns = l; }}>↑</button>
								<button class="x" disabled={i === (s.columns ?? []).length - 1} title="Descendre" onclick={() => { const l = [...(s.columns ?? [])]; [l[i + 1], l[i]] = [l[i], l[i + 1]]; s.columns = l; }}>↓</button>
							{/if}
						</div>
					{/each}
				</div>
			{:else if s.op === 'split'}
				<label class="field">Colonne <Select bind:value={s.column!} options={names} width="100%" /></label>
				<label class="field">Délimiteur <input bind:value={s.delimiter} /></label>
				<label class="field">Nombre de colonnes <input type="number" min="2" max="100" bind:value={s.parts} /></label>
			{:else if s.op === 'replace'}
				<label class="field">Colonne <Select bind:value={s.column!} options={names} width="100%" /></label>
				<label class="field">Valeur à rechercher <input bind:value={s.value} /></label>
				<label class="field">Remplacer par <input bind:value={s.with} /></label>
				<p class="hint">Dans une colonne de texte, chaque occurrence est remplacée ; ailleurs, la valeur entière.</p>
			{:else if s.op === 'transform'}
				<label class="field">Colonne <Select bind:value={s.column!} options={names} width="100%" /></label>
				<label class="field"
					>Format <Select
						bind:value={s.fn!}
						options={[
							{ value: 'trim', label: 'Supprimer les espaces (SUPPRESPACE)' },
							{ value: 'upper', label: 'MAJUSCULE' },
							{ value: 'lower', label: 'minuscule' }
						]}
						width="100%"
					/></label
				>
			{:else if s.op === 'addcolumn'}
				<label class="field">Nom de la nouvelle colonne <input bind:value={s.name} /></label>
				<label class="field">Formule (SQLite) <textarea rows="3" bind:value={s.expr} placeholder="[Quantité] * [Prix]"></textarea></label>
				<p class="hint">Colonnes disponibles : {cols.map((c) => `[${c.name}]`).join(', ')}. Exemples : [Prix] * 1.2, upper([Ville]), [Nom] || ' ' || [Prénom], CASE WHEN [Montant] > 100 THEN 'gros' ELSE 'petit' END.</p>
				<label class="field">Type <Select value={s.type ?? ''} options={[{ value: '', label: 'Auto (déduit des valeurs)' }, ...TYPES.map(([v, label]) => ({ value: v, label }))]} width="100%" onchange={(v) => (s.type = (v || undefined) as Step['type'])} /></label>
			{:else if s.op === 'index'}
				<label class="field">Nom <input bind:value={s.name} /></label>
				<label class="field">Commencer à <input type="number" bind:value={s.start} /></label>
			{:else if s.op === 'merge' || s.op === 'append'}
				<label class="field">{s.op === 'merge' ? 'Fusionner avec' : 'Ajouter les lignes de'} <Select bind:value={other} options={others.map((o) => ({ value: o.key, label: o.label }))} width="100%" /></label>
				{#if s.op === 'merge'}
					<p class="hint">Colonnes de correspondance (première requête ↔ seconde) :</p>
					{#each s.keys ?? [] as _, i (i)}
						<div class="row">
							<Select bind:value={s.keys![i]} options={names} width="170px" />
							<span>↔</span>
							<Select bind:value={s.otherKeys![i]} options={theirNames} width="170px" />
							{#if (s.keys ?? []).length > 1}<button class="x" title="Retirer" onclick={() => { s.keys = s.keys!.filter((_, k) => k !== i); s.otherKeys = s.otherKeys!.filter((_, k) => k !== i); }}>✕</button>{/if}
						</div>
					{/each}
					<button class="ico" onclick={() => { s.keys = [...(s.keys ?? []), cols[0]?.name ?? '']; s.otherKeys = [...(s.otherKeys ?? []), theirs[0]?.name ?? '']; }}><Icon name="plus" size={14} />Ajouter une colonne de correspondance</button>
					<label class="field">Type de jointure <Select bind:value={s.join!} options={JOINS.map(([v, label]) => ({ value: v, label }))} width="100%" /></label>
					{#if s.join !== 'anti'}
						<p class="hint">Colonnes à reprendre de la seconde (aucune cochée : toutes sauf les colonnes de correspondance) :</p>
						<div class="checks">
							{#each theirs as c (c.name)}
								<label><input type="checkbox" checked={(s.columns ?? []).includes(c.name)} onchange={(e) => (s.columns = toggle(s.columns, c.name, (e.currentTarget as HTMLInputElement).checked))} /> {c.name}</label>
							{/each}
						</div>
					{/if}
				{:else}
					<p class="hint">Les colonnes sont rapprochées par leur nom ; une colonne absente d'un côté reste vide.</p>
				{/if}
			{:else if s.op === 'group'}
				<p class="hint">Regrouper par :</p>
				<div class="checks">
					{#each cols as c (c.name)}
						<label><input type="checkbox" checked={(s.columns ?? []).includes(c.name)} onchange={(e) => (s.columns = toggle(s.columns, c.name, (e.currentTarget as HTMLInputElement).checked))} /> {c.name}</label>
					{/each}
				</div>
				<p class="hint">Agrégats :</p>
				{#each s.aggs ?? [] as a, i (i)}
					<div class="row">
						<input bind:value={a.name} placeholder="Nom" />
						<Select bind:value={a.op} options={AGGS.map(([v, label]) => ({ value: v, label }))} width="140px" />
						{#if a.op !== 'count'}<Select value={a.column ?? ''} options={names} width="140px" onchange={(v) => (a.column = v)} />{/if}
						<button class="x" title="Retirer" onclick={() => (s.aggs = (s.aggs ?? []).filter((_, k) => k !== i))}>✕</button>
					</div>
				{/each}
				<button class="ico" onclick={() => (s.aggs = [...(s.aggs ?? []), { name: '', op: 'sum', column: cols.find((c) => c.type === 'integer' || c.type === 'decimal')?.name ?? cols[0]?.name }])}><Icon name="plus" size={14} />Ajouter un agrégat</button>
			{:else if s.op === 'sort'}
				{#each s.sort ?? [] as k, i (i)}
					<div class="row">
						<span class="conj">{i === 0 ? 'Trier par' : 'Puis par'}</span>
						<Select bind:value={k.column} options={names} width="170px" />
						<Select value={k.desc ? 1 : 0} options={[{ value: 0, label: 'Croissant' }, { value: 1, label: 'Décroissant' }]} width="120px" onchange={(v) => (k.desc = v === 1 || undefined)} />
						{#if (s.sort ?? []).length > 1}<button class="x" title="Retirer" onclick={() => (s.sort = (s.sort ?? []).filter((_, j) => j !== i))}>✕</button>{/if}
					</div>
				{/each}
				<button class="ico" onclick={() => (s.sort = [...(s.sort ?? []), { column: cols[0]?.name ?? '' }])}><Icon name="plus" size={14} />Ajouter un niveau</button>
			{:else if s.op === 'top'}
				<label class="field">Nombre de lignes <input type="number" min="0" bind:value={s.count} /></label>
			{:else if s.op === 'pivot'}
				<label class="field">Colonne dont les valeurs deviennent des colonnes <Select bind:value={s.column!} options={names} width="100%" /></label>
				<label class="field">Colonne des valeurs <Select bind:value={s.valueColumn!} options={names} width="100%" /></label>
				<label class="field">Agrégat <Select bind:value={s.agg!} options={AGGS.filter(([v]) => v !== 'countdistinct').map(([v, label]) => ({ value: v, label: v === 'count' ? 'Nombre' : label }))} width="100%" /></label>
				<p class="hint">50 valeurs distinctes au plus ; les autres colonnes regroupent les lignes.</p>
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
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 560px;
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
		gap: 6px;
	}
	.hint {
		margin: 0 14px;
		color: #6b7280;
	}
	.body .hint {
		margin: 0;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
	}
	.conj {
		min-width: 56px;
		color: #4b5563;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	input:not([type='checkbox']):not([type='radio']),
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
	.checks {
		border: 1px solid #e5e7eb;
		max-height: 200px;
		overflow: auto;
		padding: 4px 8px;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.sel {
		display: flex;
		align-items: center;
		gap: 4px;
	}
	.sel label {
		flex: 1;
	}
	button {
		font: inherit;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		padding: 3px 10px;
		cursor: pointer;
		align-self: flex-start;
	}
	/* Bouton avec icône devant le libellé. */
	.ico {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		--icon-accent: #217346;
	}
	button:hover:not(:disabled) {
		border-color: #217346;
	}
	.x {
		border: none;
		background: none;
		padding: 0 4px;
		color: #4b5563;
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
