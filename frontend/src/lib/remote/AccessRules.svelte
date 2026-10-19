<script lang="ts">
	// Gérer l'accès › Règles avancées (restrictions du serveur, propriétaires
	// seulement) : chaque règle en une phrase (« Personne ne peut supprimer
	// de lignes dans « Ventes », sauf les propriétaires et concepteurs. »),
	// et une nouvelle règle par préréglage ou par formulaire en clair
	// (tableau, qui, quelles actions, jamais ou seulement certaines lignes),
	// relue en une phrase avant l'ajout. Une règle retire un droit quel que
	// soit l'accès du rôle ; logique dans access.ts.
	import Icon from '#lib/icons/Icon.svelte';
	import Select from '#lib/ui/Select.svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { RightsView } from '#bindings/internal/bridge/models.js';
	import FilterEditor from './FilterEditor.svelte';
	import { columnTypes, fullRights, roleLabel } from './rights';
	import { PRESETS, RULE_ACTIONS, applyPreset, draftProblems, draftRules, emptyDraft, groupRules, roleNames, ruleSentence, toggleCommand, type RuleDraft, type Run } from './access';

	interface Props {
		view: RightsView;
		busy: boolean;
		run: Run;
	}
	let { view, busy, run }: Props = $props();

	const acl = $derived(view.acl);
	const roles = $derived(acl?.roles ?? []);
	const tables = $derived(view.tables ?? []);
	const teamsOn = $derived(!!view.teams);
	const groups = $derived(groupRules(acl?.restrictions));
	const tableName = (id: string) => tables.find((t) => t.remoteId === id)?.name ?? acl?.tables?.find((t) => t.table_id === id)?.name ?? id;
	const typesOf = (id: string) => columnTypes(tables.find((t) => t.remoteId === id));

	let d = $state<RuleDraft>(emptyDraft());
	let preset = $state('');
	let predErr = $state('');
	let tried = $state(false);
	/** Remet l'éditeur de condition à zéro (préréglage, tableau changé, règle ajoutée). */
	let edKey = $state(0);
	let form = $state<HTMLElement>();
	const target = $derived(tables.find((t) => t.remoteId === d.table) ?? tables[0]);
	$effect(() => {
		if (!d.table && tables[0]) d.table = tables[0].remoteId;
	});
	const types = $derived(columnTypes(target));
	const problems = $derived(draftProblems(d, types, predErr));
	const first = $derived(Object.values(problems)[0] ?? '');
	const choosable = $derived(roles.filter((r) => !fullRights(r)));
	const sentence = $derived(d.commands.length && target ? ruleSentence({ commands: d.commands, roles: d.who === 'all' ? [] : roleNames(d.roles, roles), table: target.name, predicate: d.when === 'never' ? 'false' : d.predicate }, types) : '');

	function usePreset(id: string) {
		preset = id;
		d = applyPreset(id, { ...d, table: target?.remoteId ?? '' }, { columns: target?.columns ?? [], participant: roles.find((r) => r.builtin === 'participant')?.id });
		predErr = '';
		tried = false;
		edKey++;
	}

	async function add() {
		if (busy) return;
		if (first) {
			tried = true;
			const sels = problems.table ? ['#rr-table'] : problems.who ? ['#rr-who .ac-toggle', '#rr-who input'] : problems.commands ? ['#rr-actions button'] : ['#rr-when input.val', '#rr-when textarea', '#rr-when button'];
			sels.map((s) => form?.querySelector<HTMLElement>(s)).find((el) => !!el)?.focus();
			return;
		}
		const rules = draftRules(d);
		for (const r of rules) {
			if (!(await run(() => DocumentService.ShareAddRestriction(r), rules.length > 1 ? `${rules.length} règles ajoutées` : 'Règle ajoutée'))) return;
		}
		d = emptyDraft(d.table);
		preset = '';
		tried = false;
		edKey++;
	}

	async function drop(ids: string[]) {
		for (const id of ids) if (!(await run(() => DocumentService.ShareDropRestriction(id), 'Règle retirée'))) return;
	}
</script>

{#if !view.collaborative}
	<p class="sf-hint">Règles avancées : classeurs partagés en direct seulement (un classeur figé est en lecture seule pour tous).</p>
{:else}
	<p class="ac-intro">Une règle avancée retire un droit sur un tableau, quel que soit l’accès du rôle : ne plus supprimer, ne plus ajouter, ne modifier que certaines lignes. Les propriétaires et concepteurs n’y sont pas soumis, sauf s’ils sont nommés.</p>
	<section class="ac-section" aria-labelledby="rr-list">
		<div class="ac-section-head"><h3 id="rr-list">Règles en place</h3><span class="ac-count">{groups.length}</span></div>
		{#if groups.length}
			<ul class="ac-rules">
				{#each groups as g (g.ids.join())}
					<li class="ac-rule">
						<div>
							{#if g.label}<span class="ac-rule-label">{g.label}</span>{/if}
							<span>{ruleSentence({ commands: g.commands, roles: roleNames(g.appliesTo, roles), table: tableName(g.tableId), predicate: g.predicate }, typesOf(g.tableId))}</span>
						</div>
						<button type="button" class="ac-icon danger" disabled={busy} title="Retirer cette règle" aria-label="Retirer la règle {g.label || ''}" onclick={() => drop(g.ids)}><Icon name="trash" size={14} /> Retirer</button>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="sf-hint">Aucune règle : seuls les accès et les filtres des rôles s’appliquent.</p>
		{/if}
	</section>

	<section class="ac-section ac-card" aria-labelledby="rr-new" bind:this={form}>
		<div class="ac-section-head"><h3 id="rr-new">Nouvelle règle</h3></div>
		{#if !tables.length}
			<p class="sf-hint">Aucun tableau publié.</p>
		{:else}
			<div class="ac-presets" role="group" aria-label="Préréglages">
				{#each PRESETS as p (p.id)}
					<button type="button" class="ac-preset" class:on={preset === p.id} aria-pressed={preset === p.id} title={p.hint} disabled={busy} onclick={() => usePreset(p.id)}>{p.label}</button>
				{/each}
			</div>
			{#if preset}<p class="sf-hint" style="margin: -4px 0 10px">{PRESETS.find((p) => p.id === preset)?.hint}.</p>{/if}
			<div class="sf-fields">
				<span class="sf-label">Tableau</span>
				<div class="sf-field">
					<Select
						id="rr-table"
						label="Tableau"
						value={d.table}
						options={tables.map((t) => ({ value: t.remoteId, label: `${t.name} (${t.sheetName})` }))}
						disabled={busy}
						onchange={(v) => {
							d.table = v;
							if (d.when === 'if') d.predicate = '';
							edKey++;
						}}
					/>
				</div>

				<span class="sf-label">Pour</span>
				<div class="sf-field" id="rr-who">
					<div class="sf-seg" role="radiogroup" aria-label="Pour qui">
						<label class:on={d.who === 'all'}><input type="radio" name="rr-who" value="all" bind:group={d.who} disabled={busy} />Tous les rôles</label>
						<label class:on={d.who === 'roles'}><input type="radio" name="rr-who" value="roles" bind:group={d.who} disabled={busy} />Certains rôles</label>
					</div>
					{#if d.who === 'roles'}
						<div class="ac-toggles" class:invalid={tried && !!problems.who} role="group" aria-label="Rôles visés">
							{#each choosable as r (r.id)}
								<button type="button" class="ac-toggle" aria-pressed={d.roles.includes(r.id)} disabled={busy} onclick={() => (d.roles = d.roles.includes(r.id) ? d.roles.filter((x) => x !== r.id) : [...d.roles, r.id])}>{roleLabel(r)}</button>
							{/each}
						</div>
					{:else}
						<p class="sf-hint">Sauf propriétaires et concepteurs.</p>
					{/if}
					{#if tried && problems.who}<p class="sf-error">{problems.who}</p>{/if}
				</div>

				<span class="sf-label">Interdire</span>
				<div class="sf-field" id="rr-actions">
					<div class="ac-toggles" class:invalid={tried && !!problems.commands} role="group" aria-label="Actions limitées">
						{#each RULE_ACTIONS as x (x.cmd)}
							<button type="button" class="ac-toggle" aria-pressed={d.commands.includes(x.cmd)} disabled={busy} onclick={() => (d.commands = toggleCommand(d.commands, x.cmd))}>{x.label}</button>
						{/each}
					</div>
					{#if tried && problems.commands}<p class="sf-error">{problems.commands}</p>{/if}
				</div>

				<span class="sf-label">Exception</span>
				<div class="sf-field" id="rr-when">
					<div class="sf-seg" role="radiogroup" aria-label="Exception">
						<label class:on={d.when === 'never'}><input type="radio" name="rr-when" value="never" bind:group={d.when} disabled={busy} />Aucune</label>
						<label class:on={d.when === 'if'}><input type="radio" name="rr-when" value="if" bind:group={d.when} disabled={busy} />Sauf les lignes où…</label>
					</div>
					{#if d.when === 'if'}
						{#key `${edKey}:${target?.remoteId}`}
							<FilterEditor
								label="Permis seulement pour les lignes où…"
								readback={{ prefix: 'Permis pour les lignes où', none: 'Ajoutez une condition' }}
								columns={target?.columns ?? []}
								value={d.predicate}
								disabled={busy}
								teams={teamsOn}
								values={async (c) => ((await DocumentService.FieldValues(target?.name ?? '', c)) ?? []).map((v) => v.text)}
								assist="les lignes du tableau « {target?.name} » sur lesquelles l’action reste permise"
								onchange={(sql, e) => {
									d.predicate = sql ?? d.predicate;
									predErr = e;
								}}
							/>
						{/key}
					{/if}
					{#if tried && problems.predicate}<p class="sf-error">{problems.predicate}</p>{/if}
				</div>

				<label class="sf-label" for="rr-label">Nom <span class="sf-opt">(facultatif)</span></label>
				<input id="rr-label" class="sf-input" bind:value={d.label} disabled={busy} placeholder="Pas de suppression" autocomplete="off" />
			</div>
			{#if sentence}
				<p class="ac-readback" aria-live="polite"><Icon name="warning" size={14} /><span>{sentence}</span></p>
			{/if}
			<div class="ac-editor-actions">
				{#if tried && first}<p class="sf-error" role="alert" style="flex: 1">{first}</p>{:else}<span class="sf-hint">{d.commands.length > 1 ? `Ajoute ${d.commands.length} restrictions sur le serveur, une par action.` : ''}</span>{/if}
				<button type="button" class="sf-btn primary" disabled={busy} onclick={add}>Ajouter la règle</button>
			</div>
		{/if}
	</section>
{/if}
