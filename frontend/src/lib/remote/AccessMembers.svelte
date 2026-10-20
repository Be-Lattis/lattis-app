<script lang="ts">
	// Gérer l'accès › Membres : inviter (adresse et rôles, ce qu'ils
	// permettent relu dessous), les personnes avec leurs rôles en pastilles
	// (✕ pour retirer un rôle donné en personne, « + Rôle » pour en donner
	// un, rôles reçus par une équipe en pointillés), ce que chacun peut faire
	// en une ligne, « Voir en tant que… » et « Retirer » ; le dernier
	// propriétaire est protégé. Puis les invitations en attente et les
	// équipes Lattis (serveur 0012).
	import Icon from '#lib/icons/Icon.svelte';
	import Select from '#lib/ui/Select.svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { RightsView } from '#bindings/internal/bridge/models.js';
	import type { Role } from '#bindings/internal/remote/workbook/models.js';
	import TableSourcesList from './TableSourcesList.svelte';
	import { BUILTIN_LABELS, INVITE_ROLES, isPipeline, memberCount, members, ownerCount, roleLabel, teamRows, type MemberRow } from './rights';
	import { capsSummary, emailProblem, initial, memberCaps, NO_ACL, roleCaps, roleTone, type Run } from './access';
	import { fmtDate } from '#lib/i18n/index.js';

	interface Props {
		view: RightsView;
		busy: boolean;
		run: Run;
		/** Voir le classeur tel que ce membre le voit. */
		onpreview: (userId: string) => void;
	}
	let { view, busy, run, onpreview }: Props = $props();

	const acl = $derived(view.acl);
	const a = $derived(acl ?? NO_ACL);
	const roles = $derived(acl?.roles ?? []);
	const people = $derived(members(acl, view.members));
	const tables = $derived(view.tables ?? []);
	const collab = $derived(view.collaborative);
	const teamsOn = $derived(!!view.teams);
	const allTeams = $derived(view.teams ?? []);
	const teams = $derived(teamRows(acl, view.teams));
	const owners = $derived(ownerCount(acl));
	const roleById = (id: string) => roles.find((r) => r.id === id);
	/** Rôles du plus faible au plus fort, les personnalisés à la fin. */
	const RANK = ['reader', 'participant', 'editor', 'designer', 'owner'];
	const sorted = $derived([...roles].sort((a, b) => (a.builtin ? RANK.indexOf(a.builtin) : 9) - (b.builtin ? RANK.indexOf(b.builtin) : 9)));

	// --- Inviter ------------------------------------------------------------------------
	let email = $state('');
	let inviteRoles = $state<string[]>(['reader']);
	let tried = $state(false);
	let emailEl = $state<HTMLInputElement>();
	/** Rôles qu'une invitation peut donner : fournis (classeur figé : lecteur, concepteur, propriétaire), personnalisés par leur nom (serveur 0009). */
	const invitable = $derived([
		...INVITE_ROLES.filter((r) => collab || r === 'reader' || r === 'owner' || r === 'designer').map((r) => ({ value: r as string, label: BUILTIN_LABELS[r], role: roles.find((x) => x.builtin === r) })),
		...(view.customInvite ? roles.filter((r) => !r.builtin).map((r) => ({ value: r.name, label: r.name, role: r as Role | undefined })) : [])
	]);
	const emailErr = $derived(emailProblem(email));
	const rolesErr = $derived(inviteRoles.length ? '' : 'Choisissez au moins un rôle.');
	/** Ce que donneront les rôles choisis. */
	const inviteGives = $derived(
		capsSummary(
			memberCaps(
				invitable.filter((i) => inviteRoles.includes(i.value) && i.role).map((i) => i.role!.id),
				a,
				tables,
				collab
			)
		)
	);

	function toggleInvite(v: string) {
		inviteRoles = inviteRoles.includes(v) ? inviteRoles.filter((x) => x !== v) : [...inviteRoles, v];
	}

	async function invite(e: SubmitEvent) {
		e.preventDefault();
		if (busy) return;
		if (emailErr || rolesErr) {
			tried = true;
			if (emailErr) emailEl?.focus();
			return;
		}
		const to = email.trim();
		if (await run(() => DocumentService.ShareInvite(to, inviteRoles), `Invitation envoyée à ${to}`)) {
			email = '';
			tried = false;
		}
	}

	// --- Rôles d'une personne ou d'une équipe ---------------------------------------------------
	const grant = (r: Role, principal: string, on: boolean, who: string) =>
		run(() => DocumentService.ShareGrantRole(r.id, principal, on), on ? `Rôle ${roleLabel(r)} donné à ${who}` : `Rôle ${roleLabel(r)} retiré à ${who}`);

	/** Retire ses rôles à un membre ou à une équipe (principal : user_id ou team_id), un par un. */
	async function removeAll(principal: string, ids: string[], who: string) {
		if (!confirm(`Retirer l’accès de ${who} ? ${ids.length > 1 ? `Ses ${ids.length} rôles lui sont retirés` : 'Son rôle lui est retiré'}.`)) return;
		for (const id of ids) if (!(await run(() => DocumentService.ShareGrantRole(id, principal, false), `Accès retiré à ${who}`))) return;
	}

	/** Dernier propriétaire : son rôle ne se retire pas. */
	const lastOwner = (p: MemberRow) => owners <= 1 && p.personal.some((id) => roleById(id)?.builtin === 'owner');
	const addable = (held: string[]) => [{ value: '', label: '+ Rôle', disabled: true }, ...sorted.filter((r) => !held.includes(r.id)).map((r) => ({ value: r.id, label: roleLabel(r) }))];

	// --- Recherche -------------------------------------------------------------------------
	let query = $state('');
	const shown = $derived(query.trim() ? people.filter((p) => `${p.name} ${p.email}`.toLowerCase().includes(query.trim().toLowerCase())) : people);

	// --- Équipes -------------------------------------------------------------------------------
	let addTeam = $state('');
	let addTeamRole = $state('');
	const teamRole = $derived(roles.some((r) => r.id === addTeamRole) ? addTeamRole : (roles.find((r) => r.builtin === 'reader')?.id ?? ''));
	async function grantTeam(e: SubmitEvent) {
		e.preventDefault();
		const t = allTeams.find((x) => x.id === addTeam);
		const r = roleById(teamRole);
		if (!t || !r || busy) return;
		if (await run(() => DocumentService.ShareGrantRole(r.id, t.id, true), `Rôle ${roleLabel(r)} donné à l’équipe ${t.name}`)) addTeam = '';
	}
</script>

<section class="ac-section" aria-labelledby="am-invite">
	<form class="ac-card" onsubmit={invite} novalidate>
		<div class="ac-section-head"><h3 id="am-invite">Inviter une personne</h3></div>
		<div class="ac-card-row">
			<input
				bind:this={emailEl}
				class="sf-input"
				type="email"
				placeholder="adresse@exemple.fr"
				aria-label="Adresse à inviter"
				aria-invalid={tried && !!emailErr}
				aria-describedby="am-invite-hint"
				bind:value={email}
				disabled={busy}
				autocomplete="off"
			/>
			<div class="ac-toggles" class:invalid={tried && !!rolesErr} role="group" aria-label="Rôles à donner">
				{#each invitable as i (i.value)}
					<button type="button" class="ac-toggle" aria-pressed={inviteRoles.includes(i.value)} disabled={busy} title={i.role ? capsSummary(roleCaps(i.role, a, tables, collab)) : ''} onclick={() => toggleInvite(i.value)}>{i.label}</button>
				{/each}
			</div>
			<button type="submit" class="sf-btn primary" disabled={busy}>Inviter</button>
		</div>
		{#if tried && (emailErr || rolesErr)}
			<p class="sf-error" id="am-invite-hint" role="alert">{emailErr || rolesErr}</p>
		{:else}
			<p class="sf-hint" id="am-invite-hint">
				Pourra : <strong>{inviteGives}</strong>. Une personne déjà connue du serveur reçoit ses rôles tout de suite, sinon à sa première connexion.{view.customInvite || !roles.some((r) => !r.builtin)
					? ''
					: ' Les rôles personnalisés se donnent une fois la personne membre.'}
			</p>
		{/if}
	</form>
</section>

<section class="ac-section" aria-labelledby="am-people">
	<div class="ac-section-head">
		<h3 id="am-people">Personnes</h3>
		<span class="ac-count">{people.length}</span>
		<span class="ac-grow"></span>
		{#if people.length > 8}
			<input class="sf-input" style="width: 200px; height: 26px" type="search" placeholder="Chercher un membre" aria-label="Chercher un membre" bind:value={query} />
		{/if}
	</div>
	<ul class="ac-list">
		{#each shown as p (p.userId)}
			{@const pipe = isPipeline(p.email)}
			{@const last = lastOwner(p)}
			{@const caps = memberCaps(p.roles, a, tables, collab)}
			{@const who = p.name || p.email}
			<li class="ac-row">
				<span class="ac-avatar" aria-hidden="true">{initial(who)}</span>
				<div class="ac-who">
					<strong title={p.email}>{who}</strong>
					{#if p.name}<span class="ac-sub">{p.email}</span>{/if}
					{#if pipe || p.teams.length || last}
						<span class="ac-chips">
							{#if pipe}<span class="ac-tag" title="Membre technique d’un pipeline d’ingestion : il disparaît avec le pipeline, côté serveur">pipeline</span>{/if}
							{#if p.teams.length}<span class="ac-tag team" title="Membre du classeur par son équipe : ses rôles d’équipe se changent dans « Équipes »">équipe {p.teams.join(', ')}</span>{/if}
							{#if last}<span class="ac-tag lock" title="Un classeur garde toujours au moins un propriétaire : donnez d’abord ce rôle à quelqu’un d’autre">dernier propriétaire</span>{/if}
						</span>
					{/if}
				</div>
				<div class="ac-roles">
					<div class="ac-chips" role="list" aria-label="Rôles de {who}">
						{#each sorted.filter((r) => p.personal.includes(r.id)) as r (r.id)}
							{@const locked = r.builtin === 'owner' && owners <= 1}
							<span class="ac-chip {roleTone(r)}" class:has-x={!locked && !pipe} role="listitem">
								{#if locked}<Icon name="lock" size={11} />{/if}{roleLabel(r)}
								{#if !locked && !pipe}
									<button type="button" class="ac-x" disabled={busy} aria-label="Retirer le rôle {roleLabel(r)} à {who}" title="Retirer ce rôle" onclick={() => grant(r, p.userId, false, who)}>✕</button>
								{/if}
							</span>
						{/each}
						{#each sorted.filter((r) => p.via[r.id] && !p.personal.includes(r.id)) as r (r.id)}
							<span class="ac-chip via {roleTone(r)}" role="listitem" title="Reçu par l’équipe {p.via[r.id].join(', ')} : se retire dans « Équipes »">{roleLabel(r)} · {p.via[r.id].join(', ')}</span>
						{/each}
						{#if !pipe}
							{#key p.personal.join()}
								<Select label="Donner un rôle à {who}" value="" options={addable(p.personal)} disabled={busy} onchange={(id) => id && roleById(id) && grant(roleById(id)!, p.userId, true, who)} />
							{/key}
						{/if}
					</div>
					<span class="ac-can" title={[...caps.read.why, ...caps.editAll.why].join('\n')}>{capsSummary(caps)}</span>
				</div>
				<div class="ac-acts">
					<button type="button" class="ac-icon" disabled={busy || !tables.length} title={tables.length ? `Voir les tableaux comme ${who} les voit` : 'Aucun tableau publié'} onclick={() => onpreview(p.userId)}><Icon name="show" size={14} />Voir en tant que…</button>
					{#if p.personal.length && !pipe && !last}
						<button type="button" class="ac-icon danger" disabled={busy} title={p.teams.length ? 'Retire ses rôles en personne ; ceux de son équipe restent' : 'Retirer tous ses rôles'} aria-label="Retirer l’accès de {who}" onclick={() => removeAll(p.userId, p.personal, who)}><Icon name="trash" size={14} /></button>
					{/if}
				</div>
			</li>
		{:else}
			<li class="ac-row"><span></span><span class="sf-hint">{query ? 'Aucun membre ne correspond.' : 'Aucun membre.'}</span></li>
		{/each}
	</ul>
</section>

{#if acl?.invitations?.length}
	<section class="ac-section" aria-labelledby="am-pending">
		<div class="ac-section-head"><h3 id="am-pending">Invitations en attente</h3><span class="ac-count">{acl.invitations.length}</span></div>
		<ul class="ac-list">
			{#each acl.invitations as i (i.id)}
				<li class="ac-row">
					<span class="ac-avatar" aria-hidden="true" style="background: #f1f3f5; color: #6b7280">{initial(i.email)}</span>
					<div class="ac-who"><strong>{i.email}</strong><span class="ac-sub">jusqu’au {fmtDate(new Date(i.expires_at))}</span></div>
					<div class="ac-chips">
						{#each i.roles ?? [] as r (r)}
							{@const role = roles.find((x) => x.builtin === r || (!x.builtin && x.name.toLowerCase() === r.toLowerCase()))}
							<span class="ac-chip {roleTone(role)}">{BUILTIN_LABELS[r] ?? r}</span>
						{/each}
					</div>
					<div class="ac-acts">
						<button type="button" class="ac-icon" disabled={busy} onclick={() => run(() => DocumentService.ShareRevokeInvitation(i.id), `Invitation de ${i.email} annulée`)}>Annuler l’invitation</button>
					</div>
				</li>
			{/each}
		</ul>
	</section>
{/if}

<section class="ac-section" aria-labelledby="am-teams">
	<div class="ac-section-head"><h3 id="am-teams">Équipes</h3>{#if teamsOn}<span class="ac-count">{teams.length}</span>{/if}</div>
	{#if !teamsOn}
		<p class="sf-hint">Équipes et groupes d’annuaire : pas encore proposés par ce serveur.</p>
	{:else}
		{#if teams.length}
			<ul class="ac-list">
				{#each teams as t (t.teamId)}
					{@const who = `l’équipe ${t.name}`}
					{@const caps = memberCaps(t.roles, a, tables, collab)}
					<li class="ac-row">
						<span class="ac-avatar team" aria-hidden="true"><Icon name="user" size={14} /></span>
						<div class="ac-who">
							<strong>{t.name}</strong>
							{#if t.count !== undefined}<span class="ac-sub" title={t.members ? t.members.map((m) => m.display_name || m.email).join(', ') || 'Aucun membre' : ''}>{memberCount(t.count)}{t.members?.length ? ` : ${t.members.map((m) => m.display_name || m.email).join(', ')}` : ''}</span>{/if}
						</div>
						<div class="ac-roles">
							<div class="ac-chips" role="list" aria-label="Rôles de {who}">
								{#each sorted.filter((r) => t.roles.includes(r.id)) as r (r.id)}
									<span class="ac-chip has-x {roleTone(r)}" role="listitem"
										>{roleLabel(r)}<button type="button" class="ac-x" disabled={busy} aria-label="Retirer le rôle {roleLabel(r)} à {who}" title="Retirer ce rôle" onclick={() => grant(r, t.teamId, false, who)}>✕</button></span
									>
								{/each}
								{#key t.roles.join()}
									<Select label="Donner un rôle à {who}" value="" options={addable(t.roles)} disabled={busy} onchange={(id) => id && roleById(id) && grant(roleById(id)!, t.teamId, true, who)} />
								{/key}
							</div>
							<span class="ac-can">{capsSummary(caps)}</span>
						</div>
						<div class="ac-acts">
							<button type="button" class="ac-icon danger" disabled={busy} aria-label="Retirer l’accès de {who}" title="Retirer tous les rôles de l’équipe" onclick={() => removeAll(t.teamId, t.roles, who)}><Icon name="trash" size={14} /></button>
						</div>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="sf-hint">Aucune équipe n’a de rôle sur ce classeur.</p>
		{/if}
		{#if allTeams.length}
			<form class="ac-card-row" style="margin-top: 8px" onsubmit={grantTeam}>
				<span class="sf-hint">Donner un rôle à une équipe :</span>
				<Select label="Équipe" value={addTeam} options={[{ value: '', label: 'Équipe…', disabled: true }, ...allTeams.map((t) => ({ value: t.id, label: `${t.name} (${memberCount(t.member_count)})` }))]} disabled={busy} onchange={(v) => (addTeam = v)} />
				<Select label="Rôle de l’équipe" value={teamRole} options={sorted.map((r) => ({ value: r.id, label: roleLabel(r) }))} disabled={busy} onchange={(v) => (addTeamRole = v)} />
				<button type="submit" class="sf-btn" disabled={busy || !addTeam}>Donner le rôle</button>
			</form>
			<p class="sf-hint" style="margin-top: 4px">Les membres d’une équipe ont ses rôles tant qu’ils en font partie. Les équipes se créent dans l’administration du serveur ; groupes d’annuaire : plus tard.</p>
		{:else}
			<p class="sf-hint">Aucune équipe sur ce serveur : les propriétaires du serveur les créent dans son administration (« Équipes »).</p>
		{/if}
	{/if}
</section>

<TableSourcesList />
