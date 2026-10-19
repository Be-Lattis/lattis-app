<script lang="ts">
	// Gérer l'accès › Rôles et droits : la matrice de ce que chaque rôle
	// permet (Voir · Ajouter des lignes · Modifier ses lignes · Modifier
	// toutes les lignes · Structure et droits), puis, pour le rôle choisi,
	// l'arborescence classeur → feuilles → tableaux avec un niveau par nœud
	// (libellé par ce qu'il permet, hérité de la feuille ou du classeur), ce
	// que ça donne, les filtres de lignes en phrases (« Voit seulement… »,
	// « Modifie seulement… ») et les règles avancées qui le visent.
	import Icon from '#lib/icons/Icon.svelte';
	import Select from '#lib/ui/Select.svelte';
	import { openHelp } from '#lib/help/state.svelte.js';
	import { tick } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { RightsTable, RightsView } from '#bindings/internal/bridge/models.js';
	import type { Level as LevelChange, Role } from '#bindings/internal/remote/workbook/models.js';
	import CapMark from './CapMark.svelte';
	import FilterEditor from './FilterEditor.svelte';
	import { columnTypes, fullRights, memberCount, parentLevel, roleLabel, serverSheet, sheetLevel, tableLevel, tree, workbookLevel } from './rights';
	import {
		CAPS,
		LEVEL_WORD,
		NO_ACL,
		capsSummary,
		filterText,
		inheritedFrom,
		levelCaps,
		levelOptions,
		readFilterSentence,
		roleCaps,
		ruleApplies,
		ruleSentence,
		tableCaps,
		writeFilterSentence,
		type Run
	} from './access';

	interface Props {
		view: RightsView;
		busy: boolean;
		run: Run;
		/** Voir en tant qu'un membre de ce rôle. */
		onpreview: (roleId: string) => void;
		/** Aller aux règles avancées. */
		onrules: () => void;
		/** Classeur partagé en direct : ajouter un tableau qui n'est que sur ce poste. */
		onaddshared?: (table: string) => Promise<void>;
		/** Relire les droits (après l'ajout d'un tableau). */
		reload: () => Promise<void>;
		/** Page d'aide. */
		help: string;
	}
	let { view, busy = $bindable(), run, onpreview, onrules, onaddshared, reload, help }: Props = $props();

	const acl = $derived(view.acl);
	const a = $derived(acl ?? NO_ACL);
	const collab = $derived(view.collaborative);
	const RANK = ['owner', 'designer', 'editor', 'participant', 'reader'];
	const roles = $derived([...(acl?.roles ?? [])].sort((x, y) => (x.builtin ? RANK.indexOf(x.builtin) : 9) - (y.builtin ? RANK.indexOf(y.builtin) : 9)));
	const tables = $derived(view.tables ?? []);
	const sheets = $derived(tree(view.tables));
	const wbId = $derived(acl?.workbook.id ?? '');
	const teamsOn = $derived(!!view.teams);

	let roleId = $state('');
	let newRole = $state('');
	const role = $derived(roles.find((r) => r.id === roleId) ?? roles.find((r) => !fullRights(r)) ?? roles[0]);
	/** Personnes et équipes qui ont un rôle (en personne ou par une équipe). */
	const holders = (r: Role) => {
		const people = (view.members ?? []).filter((m) => (m.roles ?? []).some((n) => (r.builtin ? n === r.builtin : n.toLowerCase() === r.name.toLowerCase()))).length || (r.members?.length ?? 0);
		const teams = r.teams?.length ?? 0;
		return { people, teams, text: `${memberCount(people)}${teams ? ` · ${teams} équipe${teams > 1 ? 's' : ''}` : ''}` };
	};
	const roleKind = (r: Role) => (r.builtin === 'owner' ? 'tous les droits' : r.builtin === 'designer' ? 'structure, pas les droits' : r.builtin ? 'rôle fourni' : 'rôle personnalisé');

	// --- Niveaux -----------------------------------------------------------------------------
	function setLevel(kind: string, nodeId: string, level: string) {
		if (!role || level === '*') return;
		const l: LevelChange = { roleId: role.id, nodeKind: kind, nodeId, level, readFilter: '', writeFilter: '' };
		if (kind === 'table' && level) {
			// Les filtres du tableau suivent son niveau propre.
			const cur = tableLevel(role, wbId, nodeId, tables.find((t) => t.remoteId === nodeId)?.sheetKey || undefined);
			l.readFilter = cur.read ?? '';
			l.writeFilter = cur.write ?? '';
		}
		run(() => DocumentService.ShareSetLevels([l]));
	}

	// --- Filtres -------------------------------------------------------------------------------
	let editing = $state<{ table: number; read: string | null; write: string | null; readErr: string; writeErr: string } | null>(null);

	function openFilters(t: RightsTable) {
		if (!role) return;
		const cur = tableLevel(role, wbId, t.remoteId, t.sheetKey || undefined);
		editing = { table: t.table, read: cur.read ?? '', write: cur.write ?? '', readErr: '', writeErr: '' };
		tick().then(() => document.querySelector('.ac-editor')?.scrollIntoView({ block: 'nearest' }));
	}

	async function saveFilters(t: RightsTable) {
		const e = editing;
		if (!e || !role || busy) return;
		if (e.read === null || e.write === null) return;
		const cur = tableLevel(role, wbId, t.remoteId, t.sheetKey || undefined);
		const level = cur.own ?? cur.effective;
		const ok = await run(
			() => DocumentService.ShareSetLevels([{ roleId: role.id, nodeKind: 'table', nodeId: t.remoteId, level, readFilter: e.read ?? '', writeFilter: e.write ?? '' }]),
			`Filtres de « ${t.name} » enregistrés pour ${roleLabel(role)}`
		);
		if (ok) editing = null;
	}

	async function values(t: RightsTable, column: string): Promise<string[]> {
		return ((await DocumentService.FieldValues(t.name, column)) ?? []).map((v) => v.text);
	}

	async function addShared(name: string) {
		if (!onaddshared) return;
		busy = true;
		try {
			await onaddshared(name);
		} finally {
			busy = false;
		}
		await reload();
	}
</script>

<p class="sf-banner">
	<Icon name="key" size={14} />
	<span>Chaque membre a un ou plusieurs rôles ; un rôle donne un accès par tableau, que des filtres peuvent limiter à certaines lignes. Les droits des rôles s’additionnent.</span>
	<button type="button" class="sf-more" onclick={() => openHelp(help)}>En savoir plus</button>
</p>

<table class="ac-matrix" aria-label="Ce que chaque rôle permet">
	<thead>
		<tr>
			<th scope="col">Rôle</th>
			{#each CAPS as c (c.id)}<th scope="col" title={c.hint}>{c.label}</th>{/each}
			<th scope="col">Membres</th>
			<th scope="col" aria-label="Voir en tant que…"></th>
		</tr>
	</thead>
	<tbody>
		{#each roles as r (r.id)}
			{@const caps = roleCaps(r, a, tables, collab)}
			{@const h = holders(r)}
			<tr class:on={role?.id === r.id}>
				<th scope="row" style="font-weight: normal; padding: 0; text-align: left">
					<button type="button" class="ac-rolebtn" aria-pressed={role?.id === r.id} title="Régler les droits de ce rôle, tableau par tableau" onclick={() => ((roleId = r.id), (editing = null))}>
						<strong>{roleLabel(r)}</strong><span>{roleKind(r)}</span>
					</button>
				</th>
				{#each CAPS as c (c.id)}<td><CapMark cell={caps[c.id]} label="{roleLabel(r)} · {c.label}" /></td>{/each}
				<td class="ac-members">{h.text}</td>
				<td>
					<button type="button" class="ac-icon" disabled={busy || !h.people || !tables.length} title={h.people ? `Voir les tableaux comme un membre de ce rôle les voit` : 'Aucune personne n’a ce rôle'} aria-label="Voir en tant qu’un membre du rôle {roleLabel(r)}" onclick={() => onpreview(r.id)}><Icon name="show" size={14} /></button>
				</td>
			</tr>
		{/each}
	</tbody>
</table>
<div class="ac-legend">
	<span><span class="ac-cap yes" aria-hidden="true">✓</span>oui</span>
	<span><span class="ac-cap some" aria-hidden="true">◐</span>en partie (certaines lignes ou certains tableaux)</span>
	<span><span class="ac-cap no" aria-hidden="true">–</span>non</span>
	<span class="ac-grow"></span>
	{#if collab}
		<form
			class="ac-newrole"
			onsubmit={async (e) => {
				e.preventDefault();
				const n = newRole.trim();
				if (!n || busy) return;
				if (await run(() => DocumentService.ShareCreateRole(n), `Rôle « ${n} » créé : il ne donne rien tant qu’aucun accès n’est réglé ci-dessous`)) {
					newRole = '';
					roleId = (view.acl?.roles ?? []).find((r) => r.name === n)?.id ?? roleId;
				}
			}}
		>
			<input class="sf-input" placeholder="Nouveau rôle" aria-label="Nom du nouveau rôle" bind:value={newRole} disabled={busy} />
			<button type="submit" class="sf-btn" disabled={busy || !newRole.trim()}><Icon name="plus" size={13} /> Créer</button>
		</form>
	{/if}
</div>

{#if role}
	<section class="ac-detail" aria-labelledby="ar-detail">
		<div class="ac-detail-head">
			<h3 id="ar-detail">{roleLabel(role)} : accès tableau par tableau</h3>
			{#if !role.builtin}
				<button
					type="button"
					class="ac-link danger"
					disabled={busy}
					onclick={() => {
						if (confirm(`Supprimer le rôle « ${role.name} » ? Ses membres perdent les droits qu’il donnait.`)) run(() => DocumentService.ShareDropRole(role.id), `Rôle « ${role.name} » supprimé`);
					}}>Supprimer le rôle</button
				>
			{/if}
		</div>
		<div class="ac-detail-body">
			{#if fullRights(role)}
				<p style="margin: 8px 0">
					{role.builtin === 'owner'
						? 'Propriétaire : voit et modifie toutes les lignes, change la structure (colonnes, tableaux, mise en page, scripts) et gère les membres, les rôles et les règles. Rien à régler ici.'
						: 'Concepteur : voit et modifie toutes les lignes et change la structure (colonnes, tableaux, mise en page, scripts), mais ne gère pas les droits. Rien à régler ici.'}
				</p>
			{:else if !collab}
				<p class="sf-hint" style="margin: 8px 0">Classeur figé : tous les membres voient tout, sans rien modifier. Les accès par tableau et les filtres sont ceux d’un classeur partagé en direct.</p>
			{:else}
				{@const wl = workbookLevel(role, wbId)}
				<ul class="ac-tree">
					<li class="ac-node">
						<span class="ac-where"><Icon name="book" size={14} /><span>Tout le classeur</span></span>
						<span class="ac-level"><Select label="Accès au classeur" value={wl} options={levelOptions()} disabled={busy} width="100%" title="Niveau « {LEVEL_WORD[wl]} »" onchange={(v) => setLevel('workbook', wbId, v)} /></span>
						<span class="ac-gives">{capsSummary(levelCaps(wl))}</span>
					</li>
					{#each sheets as s (s.sheet)}
						{@const sl = sheetLevel(role, wbId, s)}
						<li class="ac-node sheet">
							<span class="ac-where"><Icon name="sheet" size={14} /><span title="Feuille">{s.name}</span></span>
							<span class="ac-level">
								<Select label="Accès à la feuille {s.name}" value={sl.mixed ? '*' : (sl.own ?? '')} options={levelOptions({ from: 'workbook', level: wl }, sl.mixed)} disabled={busy} width="100%" onchange={(v) => setLevel('sheet', String(s.sheet), v)} />
							</span>
							<span class="ac-gives">
								{#if sl.mixed}<span class="ac-inherit">différent selon le tableau</span>{:else}{capsSummary(levelCaps(sl.effective))}{#if sl.own === undefined}<span class="ac-inherit">· hérité du classeur</span>{/if}{/if}
							</span>
						</li>
						{#each s.tables as t (t.table)}
							{@const tl = tableLevel(role, wbId, t.remoteId, t.sheetKey || undefined)}
							{@const from = serverSheet(s) ? inheritedFrom(role, t.sheetKey || undefined) : 'workbook'}
							{@const caps = tableCaps(role, a, t)}
							{@const types = columnTypes(t)}
							{@const rules = (acl?.restrictions ?? []).filter((x) => x.table_id === t.remoteId && ruleApplies(x, role))}
							{@const ed = editing?.table === t.table ? editing : null}
							<li class="ac-node table">
								<span class="ac-where"><Icon name="table" size={14} /><span>{t.name}</span></span>
								<span class="ac-level">
									<Select label="Accès au tableau {t.name}" value={tl.own ?? ''} options={levelOptions({ from, level: parentLevel(role, wbId, t.sheetKey || undefined) })} disabled={busy} width="100%" onchange={(v) => setLevel('table', t.remoteId, v)} />
								</span>
								<span class="ac-gives" title={[...new Set(CAPS.flatMap((c) => caps[c.id].why))].join('\n')}>
									{capsSummary(caps)}{#if tl.own === undefined}<span class="ac-inherit">· hérité {from === 'sheet' ? 'de la feuille' : 'du classeur'}</span>{/if}
								</span>
								{#if tl.effective !== 'none'}
									<div class="ac-filters">
										<div class="ac-fline" class:limited={!!tl.read}>
											<span>{tl.read ? readFilterSentence(filterText(tl.read, types)) : 'Voit toutes les lignes'}</span>
											{#if !ed}<button type="button" class="ac-link" disabled={busy} onclick={() => openFilters(t)} aria-label="{tl.read ? 'Modifier' : 'Limiter'} les lignes visibles de {t.name}">{tl.read ? 'Modifier' : 'Limiter…'}</button>{/if}
										</div>
										{#if tl.effective !== 'reader'}
											<div class="ac-fline" class:limited={!!tl.write}>
												<span>{tl.write ? writeFilterSentence(filterText(tl.write, types)) : tl.effective === 'participant' ? 'Ajoute des lignes, modifie et supprime les siennes' : 'Ajoute, modifie et supprime toutes les lignes'}</span>
												{#if !ed}<button type="button" class="ac-link" disabled={busy} onclick={() => openFilters(t)} aria-label="{tl.write ? 'Modifier' : 'Limiter'} les lignes modifiables de {t.name}">{tl.write ? 'Modifier' : 'Limiter…'}</button>{/if}
											</div>
										{/if}
										{#each rules as x (x.id)}
											<div class="ac-fline rule">
												<span><Icon name="warning" size={12} /> {ruleSentence({ commands: [x.command], roles: x.applies_to?.length ? [roleLabel(role)] : [], table: t.name, predicate: x.predicate }, types)}</span>
												<button type="button" class="ac-link" onclick={onrules}>Règles avancées</button>
											</div>
										{/each}
									</div>
								{/if}
								{#if ed}
									<div class="ac-editor" role="group" aria-label="Filtres de {t.name} pour {roleLabel(role)}">
										<FilterEditor
											label="Voit seulement les lignes où…"
											readback={{ prefix: 'Voit les lignes où', none: 'Voit toutes les lignes' }}
											columns={t.columns ?? []}
											value={ed.read ?? ''}
											disabled={busy}
											teams={teamsOn}
											values={(c) => values(t, c)}
											assist="les lignes du tableau « {t.name} » que voient les membres du rôle {roleLabel(role)}"
											onchange={(sql, e) => ((ed.read = sql), (ed.readErr = e))}
										/>
										{#if tl.effective !== 'reader'}
											<FilterEditor
												label="Ajoute et modifie seulement les lignes où…"
												readback={{ prefix: 'Ajoute et modifie les lignes où', none: 'Toutes les lignes' }}
												columns={t.columns ?? []}
												value={ed.write ?? ''}
												disabled={busy}
												teams={teamsOn}
												values={(c) => values(t, c)}
												assist="les lignes du tableau « {t.name} » que les membres du rôle {roleLabel(role)} peuvent ajouter ou modifier"
												onchange={(sql, e) => ((ed.write = sql), (ed.writeErr = e))}
											/>
										{/if}
										<div class="ac-editor-actions">
											<span class="sf-hint">{tl.own === undefined ? `Enregistrer fixe l’accès de ce tableau sur « ${levelOptions().find((o) => o.value === tl.effective)?.label} ».` : 'Sans condition : toutes les lignes.'}</span>
											<button type="button" class="sf-btn" disabled={busy} onclick={() => (editing = null)}>Annuler</button>
											<button type="button" class="sf-btn primary" disabled={busy || ed.read === null || ed.write === null} title={ed.readErr || ed.writeErr || ''} onclick={() => saveFilters(t)}>Enregistrer les filtres</button>
										</div>
									</div>
								{/if}
							</li>
						{/each}
					{/each}
				</ul>
				{#if sheets.some((s) => !serverSheet(s))}
					<p class="sf-hint" style="margin-top: 6px">Feuille publiée avant que le serveur connaisse les feuilles : son accès est posé sur chacun de ses tableaux.</p>
				{/if}
				{#if !tables.length}
					<p class="sf-hint" style="margin-top: 6px">Aucun tableau publié : l’accès au classeur vaudra pour les tableaux ajoutés.</p>
				{/if}
			{/if}
			{#if teamsOn}
				<p class="sf-hint" style="margin-top: 8px">
					{#if role.teams?.length}
						Équipes de ce rôle :
						{#each role.teams as t, i (t.team_id)}{@const n = (view.teams ?? []).find((x) => x.id === t.team_id)?.member_count}{i ? ', ' : ''}<strong>{t.name}</strong>{n !== undefined ? ` (${memberCount(n)})` : ''}{/each}.
					{:else}
						Aucune équipe n’a ce rôle (Membres › Équipes).
					{/if}
				</p>
			{/if}
		</div>
	</section>
{/if}

{#if onaddshared && view.localTables?.length}
	<section class="ac-section" style="margin-top: 14px" aria-labelledby="ar-local">
		<div class="ac-section-head"><h3 id="ar-local">Sur votre poste seulement</h3></div>
		<p class="sf-hint">Ces tableaux ne sont pas dans le classeur partagé : personne d’autre ne les voit.</p>
		<div class="ac-chips" style="margin-top: 4px">
			{#each view.localTables as lt (lt)}
				<button type="button" class="sf-btn" disabled={busy || !view.online} title={view.online ? `Créer « ${lt} » dans le classeur partagé, avec ses lignes, pour tous les membres` : 'Hors ligne'} onclick={() => addShared(lt)}
					><Icon name="plus" size={13} /> Ajouter « {lt} » au classeur partagé</button
				>
			{/each}
		</div>
	</section>
{/if}
