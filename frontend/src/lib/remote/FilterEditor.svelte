<script lang="ts">
	// Filtre d'un tableau (droits) : formulaire simple (colonne, opérateur,
	// valeur, « moi », « mon équipe ») ou SQL libre. Le filtre écrit est une
	// expression booléenne sur les colonnes du tableau, vérifiée par le
	// serveur (lattis.check_predicate) ; voir rights.ts.
	import type { RightsColumn } from '#bindings/internal/bridge/models.js';
	import { untrack } from 'svelte';
	import { FILTER_OPS, MY_TEAMS, checkFreeFilter, filterSQL, parseFilter, type Condition, type FilterOp } from './rights';

	interface Props {
		columns: RightsColumn[];
		/** Filtre SQL courant ('' : aucun). */
		value: string;
		disabled?: boolean;
		/** Libellé (« Lecture : lignes visibles si… »). */
		label: string;
		/** Nouveau filtre SQL, ou erreur de saisie (sql null). */
		onchange: (sql: string | null, error: string) => void;
		/** Le serveur connaît les équipes (migration 0012) : « mon équipe » permis. */
		teams?: boolean;
	}
	let { columns, value, disabled = false, label, onchange, teams = false }: Props = $props();

	const types = $derived(Object.fromEntries(columns.map((c) => [c.name, c.type])));
	const parsed = untrack(() => parseFilter(value, Object.fromEntries(columns.map((c) => [c.name, c.type]))));
	let mode = $state<'form' | 'sql'>(parsed === null ? 'sql' : 'form');
	let conds = $state<Condition[]>(parsed ?? []);
	let sql = $state(untrack(() => value));

	function emit() {
		if (mode === 'sql') {
			const e = checkFreeFilter(sql);
			onchange(e ? null : sql.trim(), e);
			return;
		}
		try {
			onchange(filterSQL(conds, types, teams), '');
		} catch (e) {
			onchange(null, String(e instanceof Error ? e.message : e));
		}
	}

	function add() {
		conds.push({ column: columns[0]?.name ?? '', op: '=', value: { kind: 'value', value: '' } });
		emit();
	}

	function switchMode(m: 'form' | 'sql') {
		if (m === mode) return;
		if (m === 'sql') {
			try {
				sql = filterSQL(conds, types, teams);
			} catch {
				// Saisie incomplète : le SQL repart de la dernière valeur.
			}
		} else {
			const p = parseFilter(sql, types);
			if (p === null) return; // SQL libre non traduisible : reste en SQL
			conds = p;
		}
		mode = m;
		emit();
	}

	const noValue = (op: FilterOp) => op === 'empty' || op === 'filled';
	const canForm = $derived(mode === 'form' || parseFilter(sql, types) !== null);
</script>

<div class="filter">
	<div class="head">
		<span>{label}</span>
		<span class="modes" role="radiogroup" aria-label="Forme du filtre">
			<button role="radio" aria-checked={mode === 'form'} class:on={mode === 'form'} disabled={disabled || !canForm} title={canForm ? '' : 'Ce filtre SQL ne se traduit pas en formulaire'} onclick={() => switchMode('form')}>Formulaire</button>
			<button role="radio" aria-checked={mode === 'sql'} class:on={mode === 'sql'} {disabled} onclick={() => switchMode('sql')}>SQL libre</button>
		</span>
	</div>
	{#if mode === 'form'}
		{#each conds as c, i (i)}
			<div class="cond">
				{#if i > 0}<span class="and">et</span>{/if}
				<select aria-label="Colonne" bind:value={c.column} {disabled} onchange={emit}>
					{#each columns as col (col.name)}<option value={col.name}>{col.name}</option>{/each}
				</select>
				<select aria-label="Opérateur" bind:value={c.op} {disabled} onchange={emit}>
					{#each FILTER_OPS as o (o.op)}<option value={o.op}>{o.label}</option>{/each}
				</select>
				{#if !noValue(c.op)}
					<select
						aria-label="Genre de valeur"
						value={c.value.kind}
						{disabled}
						onchange={(e) => {
							const k = (e.currentTarget as HTMLSelectElement).value as 'value' | 'me' | 'team';
							c.value = k === 'value' ? { kind: 'value', value: '' } : { kind: k };
							emit();
						}}
					>
						<option value="value">valeur</option>
						<option value="me">moi</option>
						<option
							value="team"
							disabled={!teams && c.value.kind !== 'team'}
							title={teams ? 'Les équipes Lattis de la personne qui lit (lattis.my_teams)' : 'Ce serveur ne connaît pas les équipes (mise à jour de lattisd nécessaire)'}>mon équipe</option
						>
					</select>
					{#if c.value.kind === 'value'}
						<input aria-label="Valeur" bind:value={c.value.value} {disabled} oninput={emit} />
					{/if}
				{/if}
				<button
					class="link"
					aria-label="Retirer la condition"
					{disabled}
					onclick={() => {
						conds.splice(i, 1);
						emit();
					}}>✕</button
				>
			</div>
		{:else}
			<p class="muted">Aucun filtre : toutes les lignes.</p>
		{/each}
		<button class="link" {disabled} onclick={add}>+ Condition</button>
	{:else}
		<textarea aria-label="Filtre SQL" rows="2" bind:value={sql} {disabled} oninput={emit} placeholder={`"Région" = 'Nord' AND "Commercial" = current_user`}></textarea>
		<p class="muted">
			Expression booléenne sur les colonnes du tableau ; <code>current_user</code> : la personne connectée{#if teams}
				; <code>"Équipe" IN {MY_TEAMS}</code> : une de ses équipes{/if}.
		</p>
	{/if}
</div>

<style>
	.filter {
		border: 1px solid #e3e5e8;
		border-radius: 3px;
		padding: 6px 8px;
		margin: 4px 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		font-size: 12px;
	}
	.modes button {
		font: inherit;
		font-size: 12px;
		padding: 1px 8px;
		border: 1px solid #c4c8cc;
		background: #fff;
		cursor: pointer;
	}
	.modes button.on {
		background: #e8f3ec;
		border-color: #217346;
	}
	.cond {
		display: flex;
		gap: 4px;
		align-items: center;
		flex-wrap: wrap;
	}
	.and {
		font-size: 12px;
		color: #6b7280;
	}
	select,
	input,
	textarea {
		font: inherit;
		font-size: 12px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 2px 4px;
	}
	textarea {
		font-family: var(--mono, monospace);
		width: 100%;
		box-sizing: border-box;
	}
	button.link {
		font: inherit;
		font-size: 12px;
		border: none;
		padding: 0;
		background: none;
		color: #1d4ed8;
		cursor: pointer;
		align-self: flex-start;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
		margin: 0;
	}
</style>
