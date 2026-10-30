<script lang="ts">
	// Données › Analyse de scénarios : valeur cible, table de données,
	// gestionnaire de scénarios et solveur, dans une boîte compacte. Les
	// recherches ne changent rien au classeur ; « Conserver » (ou Afficher, ou
	// OK pour une table de données) fait une seule étape d'annulation.
	import { onMount, untrack } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { GoalSeekResult, Scenario, SolverResult } from '#bindings/internal/canvas/models.js';
	import { errorText, t } from '#lib/i18n/index.js';
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import { cellAddr, keepSolverState, rangeAddr, solverState, type SolverRow, type WhatIfKind } from './whatif.js';

	interface Props {
		kind: WhatIfKind;
		sheet: number;
		active: { row: number; col: number };
		selection: { r0: number; c0: number; r1: number; c1: number };
		/** Le classeur a changé (relire la vue ; notice : message de la barre d'état). */
		onchanged: (st?: { notice?: string }) => Promise<void>;
		/** Synthèse de scénarios : l'onglet créé. */
		onsheet: (id: number) => Promise<void>;
		onclose: () => void;
	}
	let { kind, sheet, active, selection, onchanged, onsheet, onclose }: Props = $props();

	// Ouverte sur la cellule active et la sélection du moment.
	const here = untrack(() => cellAddr(active.row, active.col));
	let error = $state('');
	let busy = $state(false);

	async function run(fn: () => Promise<unknown>) {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await fn();
		} catch (e) {
			error = errorText(e);
		} finally {
			busy = false;
		}
	}

	// --- Valeur cible ---
	let gTarget = $state(here);
	let gValue = $state('');
	let gChange = $state('');
	let goal = $state<GoalSeekResult | null>(null);
	const seek = () => run(async () => (goal = await DocumentService.GoalSeek(sheet, gTarget, gValue, gChange)));
	const keepGoal = () =>
		run(async () => {
			if (goal) await onchanged(await DocumentService.SetWhatIfValues(sheet, 'goal', [goal.cell], [goal.raw]));
			onclose();
		});

	// --- Table de données ---
	const area = untrack(() => ({ r0: Math.min(selection.r0, selection.r1), c0: Math.min(selection.c0, selection.c1), r1: Math.max(selection.r0, selection.r1), c1: Math.max(selection.c0, selection.c1) }));
	let tRow = $state('');
	let tCol = $state('');
	let tKnown = $state(false);
	onMount(() => {
		if (kind === 'table')
			DocumentService.DataTableAt(sheet, area)
				.then((d) => {
					if (!d) return;
					tRow = d.row ?? '';
					tCol = d.col ?? '';
					tKnown = true;
				})
				.catch(() => {});
		if (kind === 'scenarios') loadList().catch((e) => (error = errorText(e)));
	});
	const fillTable = () =>
		run(async () => {
			await onchanged(await DocumentService.DataTable(sheet, area, tRow, tCol));
			onclose();
		});

	// --- Scénarios ---
	let list = $state<Scenario[]>([]);
	let picked = $state('');
	/** Formulaire : ajout (old vide), modification, ou synthèse. */
	let form = $state<null | { old: string; name: string; cells: string; values: { cell: string; value: string }[] }>(null);
	let summary = $state<string | null>(null);
	const loadList = () =>
		DocumentService.Scenarios(sheet).then((l) => {
			list = l ?? [];
			if (!list.some((s) => s.name === picked)) picked = list[0]?.name ?? '';
		});
	const current = $derived(list.find((s) => s.name === picked));
	function addForm() {
		form = { old: '', name: '', cells: rangeAddr(selection), values: [] };
		void loadCells();
	}
	function editForm() {
		const s = current;
		if (!s) return;
		form = { old: s.name, name: s.name, cells: (s.cells ?? []).join(';'), values: (s.cells ?? []).map((c, i) => ({ cell: c, value: s.values?.[i] ?? '' })) };
	}
	/** Cellules variables changées : leurs valeurs actuelles (gardées si déjà saisies). */
	async function loadCells() {
		const f = form;
		if (!f) return;
		await run(async () => {
			const cells = await DocumentService.WhatIfCells(sheet, f.cells);
			const typed = new Map(f.values.map((v) => [v.cell, v.value]));
			f.values = (cells ?? []).map((c) => ({ cell: c.cell, value: typed.get(c.cell) ?? c.value }));
		});
	}
	const saveForm = () =>
		run(async () => {
			const f = form;
			if (!f) return;
			await DocumentService.SaveScenario(sheet, f.old, { name: f.name, sheet, cells: f.values.map((v) => v.cell), values: f.values.map((v) => v.value) });
			picked = f.name.trim();
			form = null;
			await loadList();
		});
	const showPicked = () =>
		run(async () => {
			await onchanged(await DocumentService.ShowScenario(sheet, picked));
		});
	const deletePicked = () =>
		run(async () => {
			await DocumentService.DeleteScenario(sheet, picked);
			await loadList();
		});
	const makeSummary = () =>
		run(async () => {
			const id = await DocumentService.ScenarioSummary(sheet, summary ?? '');
			onclose();
			await onsheet(id);
		});

	// --- Solveur ---
	const st = $state(untrack(() => solverState(sheet, here)));
	let draft = $state<SolverRow>({ ref: '', op: '<=', rhs: '' });
	let solved = $state<SolverResult | null>(null);
	const OPS: SolverRow['op'][] = ['<=', '>=', '=', 'int', 'bin'];
	const opLabel = (o: SolverRow['op']) => (o === 'int' ? t('whatif.solver.int') : o === 'bin' ? t('whatif.solver.bin') : o);
	function addRow() {
		if (!draft.ref.trim()) return;
		st.rows.push({ ...draft, rhs: draft.op === 'int' || draft.op === 'bin' ? '' : draft.rhs });
		draft = { ref: '', op: draft.op, rhs: '' };
	}
	const solve = () =>
		run(async () => {
			keepSolverState(sheet, $state.snapshot(st));
			solved = await DocumentService.Solve(sheet, {
				objective: st.objective,
				goal: st.goal,
				value: st.value,
				variables: st.variables,
				constraints: st.rows.map((r) => ({ ref: r.ref, op: r.op, rhs: r.rhs })),
				nonNegative: st.nonNegative
			});
		});
	const keepSolved = () =>
		run(async () => {
			if (solved?.raws?.length) await onchanged(await DocumentService.SetWhatIfValues(sheet, 'solver', solved.cells ?? [], solved.raws));
			onclose();
		});
	const solvedOk = $derived(!!solved && (solved.status === 'optimal' || solved.status === 'approx' || (solved.status === 'limit' && !!solved.raws?.length)));

	const title = $derived(t(`whatif.${kind}.title`));
	function onKey(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') onclose();
	}
</script>

<div class="back" use:backdrop={() => onclose()}>
	<div class="dlg" role="dialog" aria-label={title} tabindex="-1" aria-modal="true" use:modal onkeydown={onKey}>
		<div class="head">{title}</div>
		<div class="body">
			{#if kind === 'goal'}
				{#if !goal}
					<form
						class="grid"
						onsubmit={(e) => {
							e.preventDefault();
							void seek();
						}}
					>
						<label for="wi-g1">{t('whatif.goal.set')}</label><input id="wi-g1" bind:value={gTarget} data-autofocus />
						<label for="wi-g2">{t('whatif.goal.to')}</label><input id="wi-g2" bind:value={gValue} />
						<label for="wi-g3">{t('whatif.goal.by')}</label><input id="wi-g3" bind:value={gChange} placeholder="B1" />
						<button type="submit" hidden aria-hidden="true" tabindex="-1"></button>
					</form>
				{:else}
					<p class="status" class:bad={!goal.found}>{goal.found ? t('whatif.goal.found') : t('whatif.goal.notFound')}</p>
					<p class="mono">{goal.cell} = {goal.value} · {gTarget.toUpperCase().replaceAll('$', '')} = {goal.result}</p>
				{/if}
			{:else if kind === 'table'}
				<div class="grid">
					<span>{t('whatif.table.range')}</span><span class="mono">{rangeAddr(area)}</span>
					<label for="wi-t1">{t('whatif.table.row')}</label><input id="wi-t1" bind:value={tRow} data-autofocus />
					<label for="wi-t2">{t('whatif.table.col')}</label><input id="wi-t2" bind:value={tCol} />
				</div>
				<p class="note">{t('whatif.table.note')}</p>
			{:else if kind === 'scenarios'}
				{#if form}
					<div class="grid">
						<label for="wi-s1">{t('whatif.scenarios.name')}</label><input id="wi-s1" bind:value={form.name} data-autofocus />
						<label for="wi-s2">{t('whatif.scenarios.cells')}</label><input id="wi-s2" bind:value={form.cells} onchange={() => void loadCells()} />
						{#each form.values as v (v.cell)}
							<label for={`wi-v-${v.cell}`} class="mono">{v.cell}</label><input id={`wi-v-${v.cell}`} bind:value={v.value} />
						{/each}
					</div>
				{:else if summary !== null}
					<div class="grid">
						<label for="wi-r">{t('whatif.scenarios.results')}</label><input id="wi-r" bind:value={summary} data-autofocus />
					</div>
				{:else}
					<div class="scen">
						<select size="6" bind:value={picked} aria-label={t('whatif.scenarios.title')}>
							{#each list as s (s.name)}<option value={s.name}>{s.name}</option>{/each}
						</select>
						<div class="side">
							<button onclick={showPicked} disabled={!current || busy}>{t('whatif.scenarios.show')}</button>
							<button onclick={addForm}>{t('whatif.scenarios.add')}</button>
							<button onclick={editForm} disabled={!current}>{t('whatif.scenarios.edit')}</button>
							<button onclick={deletePicked} disabled={!current || busy}>{t('common.delete')}</button>
							<button onclick={() => (summary = here)} disabled={!list.length}>{t('whatif.scenarios.summary')}</button>
						</div>
					</div>
					{#if current}
						<p class="mono note">{(current.cells ?? []).map((c, i) => `${c} = ${current.values?.[i] ?? ''}`).join(' · ')}</p>
					{:else}
						<p class="note">{t('whatif.scenarios.none')}</p>
					{/if}
				{/if}
			{:else}
				{#if !solved}
					<div class="grid">
						<label for="wi-o">{t('whatif.solver.objective')}</label><input id="wi-o" bind:value={st.objective} data-autofocus />
						<span></span>
						<div class="goal">
							<label><input type="radio" bind:group={st.goal} value="max" />{t('whatif.solver.max')}</label>
							<label><input type="radio" bind:group={st.goal} value="min" />{t('whatif.solver.min')}</label>
							<label><input type="radio" bind:group={st.goal} value="value" />{t('whatif.solver.value')}</label>
							<input class="small" bind:value={st.value} disabled={st.goal !== 'value'} aria-label={t('whatif.solver.value')} />
						</div>
						<label for="wi-x">{t('whatif.solver.vars')}</label><input id="wi-x" bind:value={st.variables} placeholder="B1:B3" />
					</div>
					<fieldset>
						<legend>{t('whatif.solver.constraints')}</legend>
						{#each st.rows as r, i (i)}
							<div class="row mono">
								<span>{r.ref} {opLabel(r.op)} {r.rhs}</span>
								<button class="x" aria-label={t('common.delete')} title={t('common.delete')} onclick={() => st.rows.splice(i, 1)}>×</button>
							</div>
						{/each}
						<div class="row">
							<input class="small" bind:value={draft.ref} placeholder="B4" aria-label={t('whatif.solver.ref')} />
							<select bind:value={draft.op} aria-label={t('whatif.solver.op')}>
								{#each OPS as o (o)}<option value={o}>{opLabel(o)}</option>{/each}
							</select>
							<input class="small" bind:value={draft.rhs} disabled={draft.op === 'int' || draft.op === 'bin'} placeholder="10" aria-label={t('whatif.solver.rhs')} />
							<button onclick={addRow} disabled={!draft.ref.trim()}>{t('whatif.solver.add')}</button>
						</div>
					</fieldset>
					<label class="check"><input type="checkbox" bind:checked={st.nonNegative} />{t('whatif.solver.nonNeg')}</label>
				{:else}
					<p class="status" class:bad={!solvedOk}>{t(`whatif.solver.${solved.status as 'optimal' | 'approx' | 'infeasible' | 'unbounded' | 'limit'}`)}</p>
					{#if solvedOk}
						<p class="mono">{(solved.cells ?? []).map((c, i) => `${c} = ${solved?.values?.[i] ?? ''}`).join(' · ')}</p>
						<p class="mono">{st.objective.toUpperCase().replaceAll('$', '')} = {solved.objective}</p>
					{/if}
				{/if}
			{/if}
			{#if error}<p class="err" role="alert">{error}</p>{/if}
		</div>
		<div class="foot">
			<span class="sp"></span>
			{#if kind === 'goal'}
				{#if goal}
					<button class="primary" onclick={keepGoal} disabled={busy}>{t('whatif.keep')}</button>
					<button onclick={() => (goal = null)}>{t('common.cancel')}</button>
				{:else}
					<button class="primary" onclick={seek} disabled={busy || !gTarget.trim() || !gValue.trim() || !gChange.trim()}>{t('common.ok')}</button>
					<button onclick={() => onclose()}>{t('common.cancel')}</button>
				{/if}
			{:else if kind === 'table'}
				<button class="primary" onclick={fillTable} disabled={busy || (!tRow.trim() && !tCol.trim())}>{tKnown ? t('whatif.table.recalc') : t('common.ok')}</button>
				<button onclick={() => onclose()}>{t('common.cancel')}</button>
			{:else if kind === 'scenarios'}
				{#if form}
					<button class="primary" onclick={saveForm} disabled={busy || !form.name.trim() || !form.values.length}>{t('common.ok')}</button>
					<button onclick={() => (form = null)}>{t('common.cancel')}</button>
				{:else if summary !== null}
					<button class="primary" onclick={makeSummary} disabled={busy || !summary.trim()}>{t('common.ok')}</button>
					<button onclick={() => (summary = null)}>{t('common.cancel')}</button>
				{:else}
					<button onclick={() => onclose()}>{t('common.close')}</button>
				{/if}
			{:else if solved}
				<button class="primary" onclick={keepSolved} disabled={busy || !solvedOk}>{t('whatif.keep')}</button>
				<button onclick={() => (solved = null)}>{t('common.cancel')}</button>
			{:else}
				<button class="primary" onclick={solve} disabled={busy || !st.objective.trim() || !st.variables.trim()}>{busy ? t('whatif.solver.solving') : t('whatif.solver.solve')}</button>
				<button onclick={() => onclose()}>{t('common.close')}</button>
			{/if}
		</div>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: var(--z-dialog);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 60px;
		background: var(--backdrop);
	}
	.dlg {
		display: flex;
		flex-direction: column;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		width: 420px;
		max-width: calc(100% - 24px);
		outline: none;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.head {
		padding: 8px 14px;
		font-weight: 600;
		border-bottom: 1px solid #d4d7db;
	}
	.body {
		padding: 10px 14px 4px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.grid {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: 6px 10px;
		align-items: center;
	}
	input:not([type]),
	select {
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		min-width: 0;
	}
	input.small {
		width: 72px;
	}
	.goal {
		display: flex;
		gap: 10px;
		align-items: center;
	}
	.goal label,
	.check {
		display: flex;
		align-items: center;
		gap: 4px;
		white-space: nowrap;
	}
	fieldset {
		border: 1px solid #e5e7eb;
		margin: 0;
		padding: 6px 10px 8px;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	legend {
		font-size: 12px;
		color: #4b5563;
	}
	.row {
		display: flex;
		gap: 6px;
		align-items: center;
	}
	.row > span {
		flex: 1;
	}
	.x {
		border: none;
		background: none;
		cursor: pointer;
		font-size: 15px;
		color: #6b7280;
		padding: 0 4px;
	}
	.scen {
		display: flex;
		gap: 10px;
	}
	.scen select {
		flex: 1;
	}
	.side {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.mono {
		font-variant-numeric: tabular-nums;
	}
	.note {
		margin: 0;
		color: #4b5563;
		font-size: 12px;
	}
	p {
		margin: 0;
	}
	.status {
		font-weight: 600;
		color: #217346;
	}
	.status.bad,
	.err {
		color: #b42318;
	}
	.foot {
		display: flex;
		gap: 8px;
		padding: 8px 14px 12px;
	}
	.sp {
		flex: 1;
	}
	button {
		font: inherit;
		padding: 4px 12px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.foot button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
</style>
