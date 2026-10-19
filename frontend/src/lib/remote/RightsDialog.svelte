<script lang="ts">
	// Fichier › Partager… (classeur publié) : droits du classeur
	// (spécification de l'application collaborative, §8.2). Trois onglets
	// pour les propriétaires — Membres (rôles de chacun, invitations, rôles
	// des équipes Lattis sur un serveur qui les connaît, migration 0012), Rôles
	// (fournis et personnalisés, niveau par nœud classeur → feuilles →
	// tableaux, filtres de lecture et d'écriture), Restrictions — plus « Voir
	// en tant que… » et les écarts détectés par le serveur (propriétaires et
	// concepteurs). Tout passe par DocumentService.Share* (bridge/rights.go).
	import { modal } from '#lib/ui/focus.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { onMount } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { PreviewView, RightsTable, RightsView } from '#bindings/internal/bridge/models.js';
	import type { Level as LevelChange, Role } from '#bindings/internal/remote/workbook/models.js';
	import FilterEditor from './FilterEditor.svelte';
	import { hostOf } from './lattis';
	import {
		BUILTIN_LABELS,
		COMMANDS,
		INVITE_ROLES,
		LEVELS,
		checkFreeFilter,
		commandLabel,
		fullRights,
		levelLabel,
		memberCount,
		members,
		ownerCount,
		isPipeline,
		parentLevel,
		roleLabel,
		serverSheet,
		sheetLevel,
		tableLevel,
		teamRows,
		tree,
		workbookLevel
	} from './rights';

	interface Props {
		title: string;
		server: string;
		/** Adresse à partager du classeur (<serveur>/w/<id>) ; vide : non affichée. */
		link?: string;
		onclose: () => void;
	}
	let { title, server, link = '', onclose }: Props = $props();
	let linkCopied = $state(false);
	async function copyLink() {
		try {
			await navigator.clipboard.writeText(link);
			linkCopied = true;
			setTimeout(() => (linkCopied = false), 1500);
		} catch (e) {
			error = err(e);
		}
	}

	type Tab = 'members' | 'roles' | 'restrictions' | 'preview' | 'drift';
	let view = $state<RightsView | null>(null);
	let tab = $state<Tab>('members');
	let busy = $state(false);
	let error = $state('');
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);

	const acl = $derived(view?.acl ?? null);
	const manage = $derived(!!view?.canManage);
	const people = $derived(members(acl, view?.members));
	const roles = $derived(acl?.roles ?? []);
	const sheets = $derived(tree(view?.tables));
	const wbId = $derived(acl?.workbook.id ?? '');
	const tables = $derived(view?.tables ?? []);
	/** Le serveur connaît les équipes (migration 0012). */
	const teamsOn = $derived(!!view?.teams);
	const allTeams = $derived(view?.teams ?? []);
	const teamsWithRoles = $derived(teamRows(acl, view?.teams));

	onMount(load);

	async function load() {
		busy = true;
		error = '';
		try {
			view = await DocumentService.ShareRights(true);
			if (!view.canManage && (tab === 'members' || tab === 'roles' || tab === 'restrictions')) tab = 'drift';
			if (view.error) error = view.error;
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}

	/** Appel d'un changement : la liste revient à jour. */
	async function run(fn: () => Promise<RightsView>, done?: string) {
		busy = true;
		error = '';
		notice = '';
		try {
			view = await fn();
			if (done) notice = done;
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}
	let notice = $state('');

	// --- Membres -----------------------------------------------------------------------
	let inviteEmail = $state('');
	let inviteRoles = $state<string[]>(['reader']);

	function invite() {
		const email = inviteEmail.trim();
		run(() => DocumentService.ShareInvite(email, inviteRoles), `Invitation envoyée à ${email}`).then(() => {
			if (!error) inviteEmail = '';
		});
	}

	function toggleRole(r: Role, userId: string, on: boolean) {
		run(() => DocumentService.ShareGrantRole(r.id, userId, on));
	}

	/** Retire ses rôles à un membre ou à une équipe (principal : user_id ou team_id). */
	async function removeMember(principal: string, ids: string[]) {
		for (const id of ids) {
			await run(() => DocumentService.ShareGrantRole(id, principal, false));
			if (error) return;
		}
	}

	// --- Équipes (serveur 0012) ---------------------------------------------------------
	let addTeam = $state('');
	let addTeamRole = $state('');
	/** Rôle à donner à l'équipe (lecteur par défaut). */
	const teamRole = $derived(roles.some((r) => r.id === addTeamRole) ? addTeamRole : (roles.find((r) => r.builtin === 'reader')?.id ?? ''));

	function grantTeam() {
		const t = allTeams.find((x) => x.id === addTeam);
		const r = roles.find((x) => x.id === teamRole);
		if (!t || !r) return;
		run(() => DocumentService.ShareGrantRole(r.id, t.id, true), `Rôle ${roleLabel(r)} donné à l’équipe ${t.name}`).then(() => {
			if (!error) addTeam = '';
		});
	}

	/** Membres d'une équipe (propriétaire du serveur) pour une infobulle. */
	const teamMembersTitle = (ms: { email: string; display_name: string }[] | null) => (ms ? ms.map((m) => m.display_name || m.email).join(', ') || 'Aucun membre' : '');

	// --- Rôles ---------------------------------------------------------------------------
	let roleId = $state('');
	let newRole = $state('');
	const role = $derived(roles.find((r) => r.id === roleId) ?? roles.find((r) => !fullRights(r)) ?? roles[0]);
	/** Tableau dont les filtres sont ouverts, et leur brouillon. */
	let editing = $state<{ table: RightsTable; read: string | null; write: string | null; readErr: string; writeErr: string } | null>(null);

	function setLevel(kind: string, nodeId: string, level: string) {
		if (!role) return;
		const l: LevelChange = { roleId: role.id, nodeKind: kind, nodeId, level, readFilter: '', writeFilter: '' };
		if (kind === 'table') {
			// Les filtres du tableau suivent son niveau propre.
			const cur = tableLevel(role, wbId, nodeId, tables.find((t) => t.remoteId === nodeId)?.sheetKey);
			if (level) {
				l.readFilter = cur.read ?? '';
				l.writeFilter = cur.write ?? '';
			}
		}
		run(() => DocumentService.ShareSetLevels([l]));
	}

	function openFilters(t: RightsTable) {
		if (!role) return;
		const cur = tableLevel(role, wbId, t.remoteId, t.sheetKey);
		editing = { table: t, read: cur.read ?? '', write: cur.write ?? '', readErr: '', writeErr: '' };
	}

	function saveFilters() {
		const e = editing;
		if (!e || !role || e.read === null || e.write === null) return;
		const cur = tableLevel(role, wbId, e.table.remoteId, e.table.sheetKey);
		const level = cur.own ?? cur.effective;
		run(() => DocumentService.ShareSetLevels([{ roleId: role.id, nodeKind: 'table', nodeId: e.table.remoteId, level, readFilter: e.read ?? '', writeFilter: e.write ?? '' }]), 'Filtres enregistrés').then(() => {
			if (!error) editing = null;
		});
	}

	// --- Restrictions -------------------------------------------------------------------
	let rTable = $state<number>(0);
	let rCommand = $state('all');
	let rAll = $state(true);
	let rRoles = $state<string[]>([]);
	let rLabel = $state('');
	let rPred = $state<string | null>('');
	let rPredErr = $state('');
	let rKey = $state(0); // remet l'éditeur de condition à zéro
	const rTarget = $derived(tables.find((t) => t.table === rTable) ?? tables[0]);

	function addRestriction() {
		const t = rTarget;
		if (!t || !rPred) return;
		run(
			() => DocumentService.ShareAddRestriction({ id: '', table_id: t.remoteId, command: rCommand, applies_to: rAll ? [] : rRoles, predicate: rPred ?? '', label: rLabel.trim() }),
			'Restriction ajoutée'
		).then(() => {
			if (!error) {
				rLabel = '';
				rPred = '';
				rKey++;
			}
		});
	}

	// --- Voir en tant que… -------------------------------------------------------------
	let pUser = $state('');
	let pTable = $state(0);
	let preview = $state<PreviewView | null>(null);
	async function showPreview() {
		const t = tables.find((x) => x.table === pTable) ?? tables[0];
		if (!pUser || !t) return;
		busy = true;
		error = '';
		preview = null;
		try {
			preview = await DocumentService.SharePreview(pUser, t.table);
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}
	const cell = (v: unknown) => (v === null || v === undefined ? '' : String(v));

	// --- Écarts ------------------------------------------------------------------------------
	/** Recompile les droits (lattis.compile_acl) : remet d'aplomb les écarts. */
	function recompile() {
		run(() => DocumentService.ShareRecompile(), 'Droits recompilés');
	}

	const TABS: { id: Tab; label: string; manage: boolean }[] = [
		{ id: 'members', label: 'Membres', manage: true },
		{ id: 'roles', label: 'Rôles', manage: true },
		{ id: 'restrictions', label: 'Restrictions', manage: true },
		{ id: 'preview', label: 'Voir en tant que…', manage: false },
		{ id: 'drift', label: 'Écarts', manage: false }
	];
</script>

<div class="back" role="presentation" onclick={(e) => e.target === e.currentTarget && !busy && onclose()}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label="Partager"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape' && !busy) onclose();
		}}
	>
		<header>
			<Icon name="user" size={18} /><span>Partager « {title} »</span>
			<span class="muted grow">{hostOf(server)}{view && !view.collaborative ? ' · classeur figé : lecteurs seulement' : ''}</span>
			<button class="link" disabled={busy} onclick={load} title="Relire les droits sur le serveur">Actualiser</button>
		</header>
		{#if link}
			<div class="share-link">
				<span class="muted">Lien du classeur</span>
				<input readonly value={link} aria-label="Lien du classeur" onfocus={(e) => e.currentTarget.select()} />
				<button onclick={copyLink} title="Copier le lien : il ouvre le classeur dans Lattis (ou le navigateur) pour qui y a accès">{linkCopied ? 'Copié' : 'Copier'}</button>
			</div>
		{/if}
		<div class="tabs" role="tablist">
			{#each TABS.filter((t) => manage || !t.manage) as t (t.id)}
				<button role="tab" aria-selected={tab === t.id} class:on={tab === t.id} onclick={() => (tab = t.id)}>
					{t.label}{#if t.id === 'drift' && view?.drift?.length}<span class="badge">{view.drift.length}</span>{/if}
				</button>
			{/each}
		</div>
		<div class="body">
			{#if !view}
				<p class="muted">Lecture des droits…</p>
			{:else if tab === 'members'}
				<form
					class="invite"
					onsubmit={(e) => {
						e.preventDefault();
						invite();
					}}
				>
					<input type="email" placeholder="adresse@exemple.fr" aria-label="Adresse à inviter" bind:value={inviteEmail} disabled={busy} />
					{#each INVITE_ROLES.filter((r) => view?.collaborative || r === 'reader' || r === 'owner' || r === 'designer') as r (r)}
						<label><input type="checkbox" bind:group={inviteRoles} value={r} disabled={busy} />{BUILTIN_LABELS[r]}</label>
					{/each}
					{#if view.customInvite}
						{#each roles.filter((r) => !r.builtin) as r (r.id)}
							<label title="Rôle personnalisé"><input type="checkbox" bind:group={inviteRoles} value={r.name} disabled={busy} />{r.name}</label>
						{/each}
					{/if}
					<button class="primary" type="submit" disabled={busy || !inviteEmail.includes('@') || !inviteRoles.length}>Inviter</button>
				</form>
				<p class="muted">
					Une personne déjà connue du serveur reçoit ses rôles tout de suite ; sinon à sa première connexion.{view.customInvite
						? ''
						: ' Les rôles personnalisés se donnent ensuite, ci-dessous (ce serveur ne les propose pas à l’invitation).'}
				</p>
				<table class="members">
					<thead>
						<tr>
							<th>Membre</th>
							{#each roles as r (r.id)}<th title={r.builtin ? 'Rôle fourni' : 'Rôle personnalisé'}>{roleLabel(r)}</th>{/each}
							<th></th>
						</tr>
					</thead>
					<tbody>
						{#each people as p (p.userId)}
							<tr>
								<td>
									<strong>{p.name || p.email}</strong>{#if isPipeline(p.email)}<span class="pipe" title="Membre technique d’un pipeline d’ingestion : il disparaît avec la suppression du pipeline, côté serveur">pipeline</span>{/if}{#if p.name}<br /><span class="muted">{p.email}</span>{/if}
									{#if p.teams.length}<br /><span class="via" title="Membre du classeur par son équipe : ses rôles d’équipe se changent dans « Équipes » ci-dessous">via équipe {p.teams.join(', ')}</span>{/if}
								</td>
								{#each roles as r (r.id)}
									{@const on = p.personal.includes(r.id)}
									{@const via = p.via[r.id]}
									{@const lastOwner = on && r.builtin === 'owner' && ownerCount(acl) <= 1}
									<td class="c">
										<input
											type="checkbox"
											aria-label="{roleLabel(r)} pour {p.email}"
											checked={on}
											disabled={busy || lastOwner || isPipeline(p.email)}
											title={lastOwner ? 'Un classeur garde au moins un propriétaire' : on ? 'Rôle donné en personne' : ''}
											onchange={(e) => toggleRole(r, p.userId, (e.currentTarget as HTMLInputElement).checked)}
										/>
										{#if via}<span class="via" title="Rôle reçu par l’équipe {via.join(', ')}">✓ {via.join(', ')}</span>{/if}
									</td>
								{/each}
								<td>
									{#if p.personal.length && !isPipeline(p.email) && !(p.personal.some((id) => roles.find((r) => r.id === id)?.builtin === 'owner') && ownerCount(acl) <= 1)}
										<button class="link" disabled={busy} onclick={() => removeMember(p.userId, p.personal)} title={p.teams.length ? 'Retire ses rôles en personne ; ceux de son équipe restent' : ''}>Retirer</button>
									{/if}
								</td>
							</tr>
						{:else}
							<tr><td class="muted" colspan={roles.length + 2}>Aucun membre.</td></tr>
						{/each}
					</tbody>
				</table>
				{#if acl?.invitations?.length}
					<h3>Invitations en attente</h3>
					<ul class="plain">
						{#each acl.invitations as i (i.id)}
							<li>
								{i.email} · {(i.roles ?? []).map((r) => BUILTIN_LABELS[r] ?? r).join(', ')}
								<span class="muted">jusqu’au {new Date(i.expires_at).toLocaleDateString('fr-FR')}</span>
								<button class="link" disabled={busy} onclick={() => run(() => DocumentService.ShareRevokeInvitation(i.id))}>Retirer</button>
							</li>
						{/each}
					</ul>
				{/if}
				{#if teamsOn}
					<h3>Équipes</h3>
					<table class="members">
						<thead>
							<tr>
								<th>Équipe</th>
								{#each roles as r (r.id)}<th>{roleLabel(r)}</th>{/each}
								<th></th>
							</tr>
						</thead>
						<tbody>
							{#each teamsWithRoles as t (t.teamId)}
								<tr>
									<td>
										<strong>{t.name}</strong>
										{#if t.count !== undefined}<br /><span class="muted" title={teamMembersTitle(t.members)}>{memberCount(t.count)}</span>{/if}
										{#if t.members?.length}<br /><span class="muted">{t.members.map((m) => m.display_name || m.email).join(', ')}</span>{/if}
									</td>
									{#each roles as r (r.id)}
										<td class="c">
											<input
												type="checkbox"
												aria-label="{roleLabel(r)} pour l’équipe {t.name}"
												checked={t.roles.includes(r.id)}
												disabled={busy}
												onchange={(e) => toggleRole(r, t.teamId, (e.currentTarget as HTMLInputElement).checked)}
											/>
										</td>
									{/each}
									<td><button class="link" disabled={busy} onclick={() => removeMember(t.teamId, t.roles)}>Retirer</button></td>
								</tr>
							{:else}
								<tr><td class="muted" colspan={roles.length + 2}>Aucune équipe n’a de rôle sur ce classeur.</td></tr>
							{/each}
						</tbody>
					</table>
					{#if allTeams.length}
						<form
							class="invite"
							onsubmit={(e) => {
								e.preventDefault();
								grantTeam();
							}}
						>
							<select aria-label="Équipe" bind:value={addTeam} disabled={busy}>
								<option value="" disabled>Équipe…</option>
								{#each allTeams as t (t.id)}<option value={t.id}>{t.name} ({memberCount(t.member_count)})</option>{/each}
							</select>
							<select aria-label="Rôle de l’équipe" value={teamRole} disabled={busy} onchange={(e) => (addTeamRole = (e.currentTarget as HTMLSelectElement).value)}>
								{#each roles as r (r.id)}<option value={r.id}>{roleLabel(r)}</option>{/each}
							</select>
							<button class="primary" type="submit" disabled={busy || !addTeam}>Donner le rôle</button>
						</form>
						<p class="muted">Les membres de l’équipe reçoivent ses rôles tant qu’ils en font partie. Les équipes se créent et se composent dans l’administration du serveur (propriétaires du serveur).</p>
					{:else}
						<p class="muted">Aucune équipe sur ce serveur : les propriétaires du serveur les créent dans son administration (écran « Équipes »).</p>
					{/if}
					<p class="muted">Groupes d’annuaire : plus tard (ils alimenteront les équipes).</p>
				{:else}
					<p class="muted">Équipes et groupes d’annuaire : pas encore proposés par ce serveur.</p>
				{/if}
			{:else if tab === 'roles'}
				<div class="roles">
					<aside>
						{#each roles as r (r.id)}
							<button class="role" class:on={role?.id === r.id} onclick={() => ((roleId = r.id), (editing = null))}>
								{roleLabel(r)}<span class="muted"
									>{r.builtin ? 'fourni' : 'personnalisé'} · {r.members?.length ?? 0}{#if r.teams?.length}
										+ {r.teams.length} équipe{r.teams.length > 1 ? 's' : ''}{/if}</span
								>
							</button>
						{/each}
						{#if view.collaborative}
							<form
								class="new"
								onsubmit={(e) => {
									e.preventDefault();
									const n = newRole.trim();
									run(() => DocumentService.ShareCreateRole(n), `Rôle « ${n} » créé : aucun droit tant qu’aucun niveau n’est posé`).then(() => {
										if (!error) {
											newRole = '';
											roleId = roles.find((r) => r.name === n)?.id ?? roleId;
										}
									});
								}}
							>
								<input placeholder="Nouveau rôle" aria-label="Nom du nouveau rôle" bind:value={newRole} disabled={busy} />
								<button type="submit" disabled={busy || !newRole.trim()}>Créer</button>
							</form>
						{/if}
					</aside>
					<section>
						{#if !role}
							<p class="muted">Aucun rôle.</p>
						{:else if fullRights(role)}
							<p>{roleLabel(role)} : tous les droits sur les données{role.builtin === 'owner' ? ', la structure et les droits' : ' et la structure (pas les droits)'}.</p>
						{:else if !view.collaborative}
							<p class="muted">Classeur figé : tous les membres lisent tout ; les niveaux et les filtres sont ceux d’un classeur collaboratif.</p>
						{:else}
							<div class="rolehead">
								<strong>{roleLabel(role)}</strong>
								{#if !role.builtin}
									<button
										class="link"
										disabled={busy}
										onclick={() => {
											if (confirm(`Supprimer le rôle « ${role.name} » ? Ses membres perdent les droits qu’il donnait.`)) run(() => DocumentService.ShareDropRole(role.id));
										}}>Supprimer le rôle</button
									>
								{/if}
							</div>
							<table class="tree">
								<tbody>
									<tr class="wb">
										<td><Icon name="database" size={14} /> Classeur</td>
										<td>
											<select aria-label="Niveau sur le classeur" value={workbookLevel(role, wbId)} disabled={busy} onchange={(e) => setLevel('workbook', wbId, (e.currentTarget as HTMLSelectElement).value)}>
												{#each LEVELS as l (l)}<option value={l}>{levelLabel(l)}</option>{/each}
											</select>
										</td>
										<td></td>
									</tr>
									{#each sheets as s (s.sheet)}
										{@const sl = sheetLevel(role, wbId, s)}
										<tr class="sheet">
											<td>▸ {s.name}</td>
											<td>
												<select aria-label="Niveau sur la feuille {s.name}" value={sl.mixed ? '*' : (sl.own ?? '')} disabled={busy} onchange={(e) => setLevel('sheet', String(s.sheet), (e.currentTarget as HTMLSelectElement).value)}>
													{#if sl.mixed}<option value="*" disabled>mixte</option>{/if}
													<option value="">hérité ({levelLabel(workbookLevel(role, wbId))})</option>
													{#each LEVELS as l (l)}<option value={l}>{levelLabel(l)}</option>{/each}
												</select>
											</td>
											<td></td>
										</tr>
										{#each s.tables as t (t.table)}
											{@const tl = tableLevel(role, wbId, t.remoteId, t.sheetKey)}
											<tr class="table">
												<td>· {t.name}</td>
												<td>
													<select aria-label="Niveau sur le tableau {t.name}" value={tl.own ?? ''} disabled={busy} onchange={(e) => setLevel('table', t.remoteId, (e.currentTarget as HTMLSelectElement).value)}>
														<option value="">hérité ({levelLabel(parentLevel(role, wbId, t.sheetKey))})</option>
														{#each LEVELS as l (l)}<option value={l}>{levelLabel(l)}</option>{/each}
													</select>
												</td>
												<td>
													{#if tl.effective !== 'none'}
														<button class="link" disabled={busy} onclick={() => openFilters(t)}>
															{tl.read || tl.write ? 'Filtres ✓' : 'Filtres…'}
														</button>
													{/if}
												</td>
											</tr>
											{#if editing && editing.table.table === t.table}
												{@const ed = editing}
												<tr>
													<td colspan="3">
														<FilterEditor
															label="Lecture : lignes visibles si…"
															columns={t.columns ?? []}
															value={ed.read ?? ''}
															disabled={busy}
															teams={teamsOn}
															onchange={(sql, e) => ((ed.read = sql), (ed.readErr = e))}
														/>
														{#if ed.readErr}<p class="problem">{ed.readErr}</p>{/if}
														{#if tl.effective !== 'reader'}
															<FilterEditor
																label="Écriture : lignes ajoutées ou modifiées seulement si…"
																columns={t.columns ?? []}
																value={ed.write ?? ''}
																disabled={busy}
																teams={teamsOn}
																onchange={(sql, e) => ((ed.write = sql), (ed.writeErr = e))}
															/>
															{#if ed.writeErr}<p class="problem">{ed.writeErr}</p>{/if}
														{/if}
														<div class="actions">
															<button disabled={busy} onclick={() => (editing = null)}>Annuler</button>
															<button class="primary" disabled={busy || ed.read === null || ed.write === null} onclick={saveFilters}>Enregistrer les filtres</button>
														</div>
													</td>
												</tr>
											{/if}
										{/each}
									{/each}
								</tbody>
							</table>
							<p class="muted">
								Un nœud sans niveau propre hérite de son parent.{sheets.some((s) => !serverSheet(s))
									? ' Feuille marquée « mixte » ou publiée avant que le serveur connaisse les feuilles : son niveau est posé sur chacun de ses tableaux.'
									: ''}
								Participant : tout lire, ajouter des lignes, modifier et supprimer les siennes.
							</p>
						{/if}
						{#if role && teamsOn}
							<p class="muted teams">
								{#if role.teams?.length}
									Équipes de ce rôle :
									{#each role.teams as t, i (t.team_id)}{@const n = allTeams.find((x) => x.id === t.team_id)?.member_count}{i ? ', ' : ''}<strong>{t.name}</strong>{n !== undefined ? ` (${memberCount(n)})` : ''}{/each}.
								{:else}
									Aucune équipe n’a ce rôle (Membres › Équipes pour en donner un).
								{/if}
							</p>
						{/if}
					</section>
				</div>
			{:else if tab === 'restrictions'}
				{#if !view.collaborative}
					<p class="muted">Restrictions : classeurs collaboratifs seulement.</p>
				{:else}
					<p class="muted">Une restriction retire des droits sur un tableau : les lignes qui ne vérifient pas la condition ne sont plus lues, ajoutées, modifiées ou supprimées (selon la commande), pour tous les rôles (sauf propriétaires et concepteurs) ou pour certains.</p>
					<ul class="plain restr">
						{#each acl?.restrictions ?? [] as x (x.id)}
							<li>
								<strong>{x.label || 'Sans nom'}</strong> · {tables.find((t) => t.remoteId === x.table_id)?.name ?? x.table_id} · {commandLabel(x.command)} ·
								{(x.applies_to ?? []).length ? (x.applies_to ?? []).map((id) => roleLabel(roles.find((r) => r.id === id) ?? { name: id, builtin: '' })).join(', ') : 'tous les rôles'}
								<br /><code>{x.predicate}</code>
								<button class="link" disabled={busy} onclick={() => run(() => DocumentService.ShareDropRestriction(x.id))}>Retirer</button>
							</li>
						{:else}
							<li class="muted">Aucune restriction.</li>
						{/each}
					</ul>
					<h3>Nouvelle restriction</h3>
					<div class="grid">
						<span>Tableau</span>
						<select aria-label="Tableau" bind:value={rTable} disabled={busy}>
							{#each tables as t (t.table)}<option value={t.table}>{t.sheetName} · {t.name}</option>{/each}
						</select>
						<span>Commande</span>
						<select aria-label="Commande" bind:value={rCommand} disabled={busy}>
							{#each COMMANDS as c (c.cmd)}<option value={c.cmd}>{c.label}</option>{/each}
						</select>
						<span>Pour</span>
						<div class="who">
							<label><input type="checkbox" bind:checked={rAll} disabled={busy} />Tous les rôles</label>
							{#if !rAll}
								{#each roles.filter((r) => !fullRights(r)) as r (r.id)}
									<label><input type="checkbox" bind:group={rRoles} value={r.id} disabled={busy} />{roleLabel(r)}</label>
								{/each}
							{/if}
						</div>
						<span>Libellé</span>
						<input aria-label="Libellé" bind:value={rLabel} disabled={busy} placeholder="Pas les dossiers clos" />
					</div>
					{#key `${rKey}:${rTarget?.table}`}
						<FilterEditor label="Condition à respecter" columns={rTarget?.columns ?? []} value="" disabled={busy} teams={teamsOn} onchange={(sql, e) => ((rPred = sql), (rPredErr = e))} />
					{/key}
					{#if rPredErr}<p class="problem">{rPredErr}</p>{/if}
					<div class="actions">
						<button class="primary" disabled={busy || !rTarget || !rPred || (!rAll && !rRoles.length) || !!checkFreeFilter(rPred ?? '')} onclick={addRestriction}>Ajouter la restriction</button>
					</div>
				{/if}
			{:else if tab === 'preview'}
				{#if !manage && !view.members}
					<p class="muted">Ce serveur réserve la liste des membres aux propriétaires du classeur : demandez-leur « Voir en tant que… ».</p>
				{:else}
					<div class="grid">
						<span>Membre</span>
						<select aria-label="Membre" bind:value={pUser} disabled={busy}>
							<option value="" disabled>Choisir…</option>
							{#each people as p (p.userId)}<option value={p.userId}>{p.name || p.email}</option>{/each}
						</select>
						<span>Tableau</span>
						<select aria-label="Tableau à voir" bind:value={pTable} disabled={busy}>
							{#each tables as t (t.table)}<option value={t.table}>{t.sheetName} · {t.name}</option>{/each}
						</select>
					</div>
					<div class="actions"><button class="primary" disabled={busy || !pUser} onclick={showPreview}>Voir</button></div>
					{#if preview}
						<p class="muted">
							{preview.rows?.length ?? 0} ligne(s) visible(s){preview.truncated ? ' (les 1 000 premières)' : ''} — lecture seule, telle que ce membre la voit (règles et restrictions comprises).
						</p>
						<div class="scroll">
							<table class="preview">
								<thead><tr>{#each preview.columns ?? [] as c (c)}<th>{c}</th>{/each}</tr></thead>
								<tbody>
									{#each preview.rows ?? [] as r, i (i)}
										<tr>{#each r ?? [] as v, j (j)}<td>{cell(v)}</td>{/each}</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{/if}
				{/if}
			{:else if tab === 'drift'}
				{#if !view.collaborative}
					<p class="muted">Classeur figé : pas d’écart possible (lecture seule pour tous).</p>
				{:else if !(view.drift ?? []).length}
					<p>Aucun écart : les règles et droits du serveur sont ceux que Lattis a générés.</p>
				{:else}
					<p>Règles ou droits changés hors de Lattis (à la main, dans PostgreSQL) :</p>
					<ul class="plain">
						{#each view.drift ?? [] as d (d.table_id)}
							<li><strong>{d.name}</strong>{#if d.foreign_policies?.length} · règles ajoutées : <code>{d.foreign_policies.join(', ')}</code>{/if}{#if d.foreign_grants?.length} · droits donnés à : <code>{d.foreign_grants.join(', ')}</code>{/if}{#if !d.foreign_policies?.length && !d.foreign_grants?.length} · règles générées retirées ou modifiées{/if}</li>
						{/each}
					</ul>
					{#if manage || view.canDesign}
						<div class="actions"><button class="primary" disabled={busy} onclick={recompile}>Recompiler les droits</button></div>
						<p class="muted">Recompiler remet les règles générées par Lattis. Il ne retire ni les règles ni les droits ajoutés à la main : ils restent signalés ici tant qu’ils ne sont pas retirés dans PostgreSQL.</p>
					{/if}
				{/if}
			{/if}
			{#if notice}<p class="notice">{notice}</p>{/if}
			{#if error}<p class="error" role="alert">{error}</p>{/if}
		</div>
		<footer>
			<span class="grow muted">{view && !manage ? 'Concepteur : consultation seulement' : ''}</span>
			<button onclick={onclose} disabled={busy}>Fermer</button>
		</footer>
	</div>
</div>

<style>
	.share-link {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 14px;
		border-bottom: 1px solid #e5e7eb;
	}
	.share-link input {
		flex: 1;
		min-width: 0;
		font: inherit;
	}
	.back {
		position: fixed;
		inset: 0;
		z-index: calc(var(--z-dialog) + 30);
		background: var(--backdrop);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 5vh;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 820px;
		max-width: calc(100vw - 32px);
		max-height: 88vh;
		display: flex;
		flex-direction: column;
		outline: none;
		--icon-accent: #217346;
	}
	header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 16px;
		font-weight: 600;
		border-bottom: 1px solid #e3e5e8;
	}
	header .muted {
		font-weight: normal;
	}
	.tabs {
		display: flex;
		gap: 2px;
		padding: 6px 16px 0;
		border-bottom: 1px solid #e3e5e8;
	}
	.tabs button {
		border: 1px solid transparent;
		border-bottom: none;
		border-radius: 3px 3px 0 0;
		background: none;
		padding: 4px 12px;
	}
	.tabs button.on {
		border-color: #e3e5e8;
		background: #fff;
		margin-bottom: -1px;
		font-weight: 600;
	}
	.badge {
		margin-left: 4px;
		background: #b45309;
		color: #fff;
		border-radius: 8px;
		padding: 0 5px;
		font-size: 11px;
	}
	.body {
		padding: 12px 16px;
		overflow: auto;
		min-height: 240px;
	}
	h3 {
		font-size: 13px;
		margin: 14px 0 6px;
	}
	p {
		margin: 0 0 6px;
	}
	.invite {
		display: flex;
		gap: 8px;
		align-items: center;
		flex-wrap: wrap;
		margin-bottom: 6px;
	}
	.invite label,
	.who label {
		display: inline-flex;
		gap: 3px;
		align-items: center;
		margin-right: 6px;
	}
	table {
		border-collapse: collapse;
		width: 100%;
	}
	th,
	td {
		text-align: left;
		padding: 4px 6px;
		border-bottom: 1px solid #eef0f2;
		font-size: 13px;
		vertical-align: top;
	}
	th {
		font-weight: 600;
		font-size: 12px;
		color: #4b5563;
	}
	td.c {
		text-align: center;
	}
	.roles {
		display: grid;
		grid-template-columns: 200px 1fr;
		gap: 12px;
	}
	aside {
		display: flex;
		flex-direction: column;
		gap: 2px;
		border-right: 1px solid #eef0f2;
		padding-right: 8px;
	}
	.role {
		text-align: left;
		border: none;
		background: none;
		padding: 4px 6px;
		display: flex;
		flex-direction: column;
	}
	.role.on {
		background: #e8f3ec;
	}
	.new {
		display: flex;
		gap: 4px;
		margin-top: 8px;
	}
	.new input {
		min-width: 0;
		flex: 1;
	}
	.rolehead {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 6px;
	}
	.tree tr.sheet td:first-child {
		padding-left: 18px;
	}
	.tree tr.table td:first-child {
		padding-left: 34px;
	}
	.grid {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: 6px 12px;
		align-items: center;
		margin-bottom: 6px;
	}
	.plain {
		list-style: none;
		padding: 0;
		margin: 0 0 6px;
	}
	.plain li {
		padding: 4px 0;
		border-bottom: 1px solid #eef0f2;
	}
	.actions {
		display: flex;
		gap: 6px;
		justify-content: flex-end;
		margin: 6px 0;
	}
	.scroll {
		overflow: auto;
		max-height: 40vh;
		border: 1px solid #e3e5e8;
	}
	.preview td {
		white-space: nowrap;
	}
	code {
		font-size: 12px;
		background: #f3f4f6;
		padding: 0 3px;
	}
	button {
		font: inherit;
		padding: 3px 12px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.55;
		cursor: default;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	button.link {
		border: none;
		padding: 0;
		background: none;
		color: #1d4ed8;
		text-decoration: underline;
	}
	input:not([type='checkbox']):not([type='radio']),
	select {
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 3px 6px;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.pipe {
		background: #ede9fe;
		color: #5b21b6;
		border-radius: 3px;
		font-size: 11px;
		padding: 0 5px;
		margin-left: 6px;
	}
	.via {
		color: #1d4ed8;
		font-size: 11px;
		margin-left: 2px;
	}
	p.teams {
		margin-top: 8px;
	}
	.problem {
		color: #b45309;
		font-size: 12px;
	}
	.notice {
		color: #1b5e3a;
		margin-top: 8px;
	}
	.error {
		color: #c0392b;
		margin-top: 10px;
		white-space: pre-line;
	}
	footer {
		display: flex;
		gap: 6px;
		align-items: center;
		padding: 10px 16px;
		border-top: 1px solid #e3e5e8;
	}
	.grow {
		flex: 1;
	}
</style>
