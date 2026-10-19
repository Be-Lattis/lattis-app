<script lang="ts">
	// Filtre d'un tableau (droits, audience d'une donnée du Data Lake) :
	// formulaire en clair (colonne, opérateur, valeur, « la personne
	// connectée », « une de ses équipes »), avec la règle relue en une phrase,
	// ou SQL libre aidé (colonnes et morceaux à insérer d'un clic, contrôle
	// immédiat), ou rédigé par l'assistant. Le filtre écrit est une
	// expression booléenne sur les colonnes du tableau, vérifiée par le
	// serveur (lattis.check_predicate) ; voir rights.ts.
	import type { RightsColumn } from '#bindings/internal/bridge/models.js';
	import Icon from '#lib/icons/Icon.svelte';
	import Select from '#lib/ui/Select.svelte';
	import { askAssistant } from '#lib/ai/state.svelte.js';
	import { tick, untrack } from 'svelte';
	import { typeLabel } from './data';
	import { FILTER_OPS, ME_LABEL, MY_TEAMS, TEAM_LABEL, checkFreeFilter, filterPrompt, filterSQL, filterSentence, opsFor, parseFilter, quoteIdent, type Condition, type FilterOp } from './rights';

	interface Props {
		columns: RightsColumn[];
		/** Filtre SQL courant ('' : aucun). */
		value: string;
		disabled?: boolean;
		/** Libellé (« Lecture : lignes visibles si… »). */
		label: string;
		/** Nouveau filtre SQL, ou erreur de saisie (sql null). */
		onchange: (sql: string | null, error: string) => void;
		/** Le serveur connaît les équipes (migration 0012) : « une de ses équipes » permis. */
		teams?: boolean;
		/** Valeurs distinctes d'une colonne (suggestions de la valeur) ; absent : saisie libre. */
		values?: (column: string) => Promise<string[]>;
		/** Ce que filtre la règle, pour l'assistant (« les lignes du tableau « Ventes » que voit… ») ; absent : pas de bouton. */
		assist?: string;
	}
	let { columns, value, disabled = false, label, onchange, teams = false, values, assist }: Props = $props();

	const uid = Math.random().toString(36).slice(2, 8);
	const types = $derived(Object.fromEntries(columns.map((c) => [c.name, c.type])));
	const parsed = untrack(() => parseFilter(value, Object.fromEntries(columns.map((c) => [c.name, c.type]))));
	let mode = $state<'form' | 'sql'>(parsed === null ? 'sql' : 'form');
	let conds = $state<Condition[]>(parsed ?? []);
	let sql = $state(untrack(() => value));
	let error = $state('');
	let area = $state<HTMLTextAreaElement>();
	let suggestions = $state<Record<string, string[]>>({});

	function emit() {
		if (mode === 'sql') {
			error = checkFreeFilter(sql);
			onchange(error ? null : sql.trim(), error);
			return;
		}
		try {
			const s = filterSQL(conds, types, teams);
			error = '';
			onchange(s, '');
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
			onchange(null, error);
		}
	}

	function add() {
		const first = columns.find((c) => c.type === 'text') ?? columns[0];
		conds.push({ column: first?.name ?? '', op: '=', value: { kind: 'value', value: '' } });
		emit();
	}

	function setColumn(c: Condition, name: string) {
		c.column = name;
		const t = types[name];
		if (!opsFor(t).includes(c.op)) c.op = '=';
		if (c.value.kind !== 'value' && t !== 'text') c.value = { kind: 'value', value: '' };
		emit();
	}

	function setOp(c: Condition, op: FilterOp) {
		c.op = op;
		if (c.value.kind !== 'value' && op !== '=' && op !== '<>') c.value = { kind: 'value', value: '' };
		emit();
	}

	async function suggest(column: string) {
		if (!values || column in suggestions) return;
		suggestions[column] = [];
		try {
			suggestions[column] = (await values(column)).filter((v) => v !== '').slice(0, 200);
		} catch {
			// Pas de valeurs : saisie libre.
		}
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

	/** Insère un morceau à la place de la sélection du SQL libre. */
	async function insert(text: string) {
		const el = area;
		const a = el?.selectionStart ?? sql.length;
		const b = el?.selectionEnd ?? sql.length;
		const before = sql.slice(0, a);
		const pad = before && !/\s$/.test(before) && !/^\s/.test(text) ? ' ' : '';
		sql = before + pad + text + sql.slice(b);
		emit();
		await tick();
		const at = a + pad.length + text.length;
		el?.focus();
		el?.setSelectionRange(at, at);
	}

	function ask() {
		switchMode('sql');
		askAssistant(filterPrompt(assist ?? 'ces lignes', columns, teams));
	}

	const noValue = (op: FilterOp) => op === 'empty' || op === 'filled';
	const canForm = $derived(mode === 'form' || parseFilter(sql, types) !== null);
	const colOptions = $derived(columns.map((c) => ({ value: c.name, label: `${c.name} · ${typeLabel(c.type).toLowerCase()}` })));
	const textCol = $derived(columns.find((c) => c.type === 'text')?.name ?? columns[0]?.name ?? 'Colonne');
	const snippets = $derived([
		{ label: `${textCol} = valeur`, sql: `${quoteIdent(textCol)} = 'valeur'`, title: 'Égal à une valeur (texte entre apostrophes)' },
		{ label: `= ${ME_LABEL}`, sql: '= current_user', title: 'current_user : adresse de la personne qui lit' },
		...(teams ? [{ label: `est ${TEAM_LABEL}`, sql: `IN ${MY_TEAMS}`, title: 'lattis.my_teams() : équipes de la personne qui lit' }] : []),
		{ label: 'et', sql: 'AND ', title: 'Les deux conditions' },
		{ label: 'ou', sql: 'OR ', title: "L'une ou l'autre condition" },
		{ label: 'est vide', sql: 'IS NULL', title: 'Cellule vide' }
	]);
</script>

<div class="filter" role="group" aria-label={label}>
	<div class="head">
		<span class="title">{label}</span>
		<span class="modes" role="radiogroup" aria-label="Forme du filtre">
			<button type="button" role="radio" aria-checked={mode === 'form'} class:on={mode === 'form'} disabled={disabled || !canForm} title={canForm ? 'Conditions en clair' : 'Ce filtre SQL ne se traduit pas en formulaire'} onclick={() => switchMode('form')}>Formulaire</button>
			<button type="button" role="radio" aria-checked={mode === 'sql'} class:on={mode === 'sql'} {disabled} onclick={() => switchMode('sql')}>SQL libre</button>
		</span>
		{#if assist}
			<button type="button" class="assist" {disabled} onclick={ask} title="Ouvre l'assistant avec une demande à compléter ; il ne reçoit que les noms et les types des colonnes">
				<Icon name="sparkle" size={13} />Rédiger avec l'assistant
			</button>
		{/if}
	</div>
	{#if mode === 'form'}
		{#each conds as c, i (i)}
			{@const t = types[c.column]}
			<div class="cond">
				<span class="and">{i > 0 ? 'et' : ''}</span>
				<Select label="Colonne" value={c.column} options={colOptions} {disabled} onchange={(v) => setColumn(c, v)} />
				<Select label="Condition" value={c.op} options={FILTER_OPS.filter((o) => opsFor(t).includes(o.op)).map((o) => ({ value: o.op, label: o.label }))} {disabled} onchange={(v) => setOp(c, v)} />
				{#if !noValue(c.op)}
					{#if t === 'text' && (c.op === '=' || c.op === '<>')}
						<Select
							label="Valeur"
							value={c.value.kind}
							options={[
								{ value: 'value', label: 'la valeur…' },
								{ value: 'me', label: ME_LABEL },
								{ value: 'team', label: TEAM_LABEL, disabled: !teams && c.value.kind !== 'team' }
							]}
							title={teams ? '' : '« une de ses équipes » : ce serveur ne connaît pas les équipes'}
							{disabled}
							onchange={(k) => {
								c.value = k === 'value' ? { kind: 'value', value: '' } : { kind: k };
								emit();
							}}
						/>
					{/if}
					{#if c.value.kind === 'value'}
						<input
							class="val"
							aria-label="Valeur"
							list={values ? `fv-${uid}-${i}` : undefined}
							placeholder={t === 'date' ? 'jj/mm/aaaa' : t === 'boolean' ? 'vrai ou faux' : t === 'integer' || t === 'decimal' ? 'nombre' : 'valeur'}
							bind:value={c.value.value}
							{disabled}
							onfocus={() => suggest(c.column)}
							oninput={emit}
						/>
						{#if values}
							<datalist id="fv-{uid}-{i}">
								{#each suggestions[c.column] ?? [] as v (v)}<option value={v}></option>{/each}
							</datalist>
						{/if}
					{/if}
				{/if}
				<button
					type="button"
					class="x"
					aria-label="Retirer la condition"
					title="Retirer la condition"
					{disabled}
					onclick={() => {
						conds.splice(i, 1);
						emit();
					}}>✕</button
				>
			</div>
		{/each}
		<div class="foot">
			<button type="button" class="link" disabled={disabled || !columns.length} onclick={add}>+ Condition</button>
			<span class="rule" aria-live="polite"><Icon name="show" size={13} />{filterSentence(conds)}</span>
		</div>
	{:else}
		<div class="chips" aria-label="Colonnes à insérer">
			{#each columns as col (col.name)}
				<button type="button" class="chip col" {disabled} title="{typeLabel(col.type)} : insère {quoteIdent(col.name)}" onclick={() => insert(quoteIdent(col.name))}>{col.name}</button>
			{/each}
		</div>
		<textarea
			bind:this={area}
			aria-label="Filtre SQL"
			aria-invalid={!!error}
			aria-describedby={error ? `fe-err-${uid}` : undefined}
			rows="2"
			bind:value={sql}
			{disabled}
			oninput={emit}
			spellcheck="false"
			placeholder={`${quoteIdent(textCol)} = current_user`}
		></textarea>
		<div class="chips snippets" aria-label="Morceaux à insérer">
			{#each snippets as s (s.label)}
				<button type="button" class="chip" {disabled} title="{s.title} : insère {s.sql.trim()}" onclick={() => insert(s.sql)}>{s.label}</button>
			{/each}
		</div>
	{/if}
	{#if error}<p class="error" id="fe-err-{uid}" role="alert">{error}</p>{/if}
</div>

<style>
	.filter {
		border: 1px solid #e3e5e8;
		border-radius: 4px;
		padding: 6px 8px;
		margin: 4px 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
		background: #fcfcfd;
	}
	.head {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
	}
	.title {
		flex: 1;
		color: #374151;
		font-weight: 600;
	}
	.modes {
		display: inline-flex;
	}
	.modes button {
		font: inherit;
		font-size: 12px;
		padding: 1px 8px;
		border: 1px solid #c4c8cc;
		background: #fff;
		color: inherit;
		cursor: pointer;
	}
	.modes button:first-child {
		border-radius: 3px 0 0 3px;
	}
	.modes button:last-child {
		border-radius: 0 3px 3px 0;
		border-left: none;
	}
	.modes button.on {
		background: #e8f3ec;
		border-color: #217346;
		color: #14532d;
	}
	.modes button:last-child.on {
		border-left: 1px solid #217346;
	}
	.assist {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font: inherit;
		font-size: 12px;
		padding: 1px 8px;
		border: 1px solid #e8d3b0;
		border-radius: 3px;
		background: #fffaf2;
		color: #92400e;
		cursor: pointer;
		--icon-accent: #d97706;
	}
	.assist:hover:not(:disabled) {
		border-color: #d97706;
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
		width: 14px;
	}
	.val {
		font: inherit;
		font-size: 12px;
		height: 24px;
		box-sizing: border-box;
		border: 1px solid #c9cdd2;
		border-radius: 4px;
		padding: 2px 6px;
		width: 150px;
		background: #fff;
		color: inherit;
	}
	.val:focus,
	textarea:focus {
		outline: none;
		border-color: #217346;
		box-shadow: 0 0 0 2px rgba(33, 115, 70, 0.18);
	}
	.x {
		border: none;
		background: none;
		color: #6b7280;
		cursor: pointer;
		font-size: 12px;
		padding: 2px 4px;
		border-radius: 3px;
	}
	.x:hover:not(:disabled) {
		color: #b91c1c;
		background: #fdf0ef;
	}
	.foot {
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
	}
	.rule {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-size: 12px;
		color: #14532d;
		min-width: 0;
	}
	textarea {
		font: inherit;
		font-family: var(--mono, Consolas, monospace);
		font-size: 12px;
		width: 100%;
		box-sizing: border-box;
		border: 1px solid #c9cdd2;
		border-radius: 4px;
		padding: 4px 6px;
		resize: vertical;
	}
	textarea[aria-invalid='true'] {
		border-color: #b91c1c;
	}
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}
	.chip {
		font: inherit;
		font-size: 11.5px;
		padding: 1px 7px;
		border: 1px solid #d4d7db;
		border-radius: 10px;
		background: #fff;
		color: #374151;
		cursor: pointer;
	}
	.chip.col {
		background: #eef6f0;
		border-color: #c5dfcd;
		color: #14532d;
	}
	.chip:hover:not(:disabled) {
		border-color: #217346;
	}
	button.link {
		font: inherit;
		font-size: 12px;
		border: none;
		padding: 0;
		background: none;
		color: #217346;
		cursor: pointer;
	}
	button.link:hover:not(:disabled) {
		text-decoration: underline;
	}
	button:disabled {
		opacity: 0.55;
		cursor: default;
	}
	.error {
		color: #b91c1c;
		font-size: 12px;
		margin: 0;
	}
</style>
