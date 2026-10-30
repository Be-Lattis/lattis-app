<script lang="ts">
	// Gérer l'accès › Membres : inviter (personne du serveur cherchée par
	// nom ou adresse, ou « Inviter à rejoindre » une adresse, avec ses rôles,
	// ce qu'ils permettent relu dessous ; e-mail du serveur ou lien à
	// transmettre), les personnes avec leurs rôles en pastilles
	// (✕ pour retirer un rôle donné en personne, « + Rôle » pour en donner
	// un, rôles reçus par une équipe en pointillés), ce que chacun peut faire
	// en une ligne, « Voir en tant que… » et « Retirer » ; le dernier
	// propriétaire est protégé. Puis les invitations en attente et les
	// équipes Lattis (serveur 0012).
	import Icon from '#lib/icons/Icon.svelte';
	import Select from '#lib/ui/Select.svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { InviteOutcome, RightsView } from '#bindings/internal/bridge/models.js';
	import type { Person, Role } from '#bindings/internal/remote/workbook/models.js';
	import TableSourcesList from './TableSourcesList.svelte';
	import { BUILTIN_LABELS, INVITE_ROLES, inviteOptions, inviteSubmitLabel, isEmail, isPipeline, type InviteOption, memberCount, members, ownerCount, roleLabel, teamRows, type MemberRow } from './rights';
	import { capsSummary, emailProblem, initial, memberCaps, NO_ACL, roleCaps, roleTone, type Run } from './access';
	import { fmtDate, t } from '#lib/i18n/index.js';

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
	const rolesErr = $derived(inviteRoles.length ? '' : t('remote.access.problem.who'));
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

	// Recherche des personnes du serveur (lattis.search_people, 2 caractères
	// au moins, après une courte pause de frappe) ; la dernière réponse seule
	// compte.
	let found = $state<Person[]>([]);
	let listOpen = $state(false);
	let active = $state(-1);
	/** Personne choisie dans la liste (elle a un compte : rôles donnés tout de suite). */
	let picked = $state<Person | null>(null);
	let searchSeq = 0;
	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	const options = $derived(inviteOptions(email, found));
	const sameEmail = (p: Person) => p.email.toLowerCase() === email.trim().toLowerCase();
	const known = $derived((!!picked && sameEmail(picked)) || found.some(sameEmail));
	const submitLabel = $derived(inviteSubmitLabel(known, view.mail));

	function emailInput() {
		result = null;
		if (picked && !sameEmail(picked)) picked = null;
		listOpen = true;
		active = -1;
		clearTimeout(searchTimer);
		const q = email.trim();
		const seq = ++searchSeq;
		if (q.length < 2) {
			found = [];
			return;
		}
		searchTimer = setTimeout(async () => {
			let r: Person[] = [];
			try {
				r = (await DocumentService.ShareSearchPeople(q)) ?? [];
			} catch {
				// Recherche indisponible : reste « Inviter à rejoindre ».
			}
			if (seq === searchSeq) found = r;
		}, 250);
	}

	function choose(o: InviteOption) {
		if (o.kind === 'join') {
			if (o.disabled) return;
			picked = null;
			email = o.email;
		} else {
			picked = o.person;
			email = o.person.email;
		}
		listOpen = false;
		active = -1;
	}

	function emailKey(e: KeyboardEvent) {
		if (!options.length) return;
		if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
			e.preventDefault();
			if (!listOpen) {
				listOpen = true;
				active = 0;
			} else active = (active + (e.key === 'ArrowDown' ? 1 : options.length - 1)) % options.length;
		} else if (listOpen && e.key === 'Enter' && active >= 0) {
			e.preventDefault();
			choose(options[active]);
		} else if (listOpen && e.key === 'Escape') {
			// La liste se ferme, pas la boîte.
			e.preventDefault();
			e.stopPropagation();
			listOpen = false;
		}
	}

	/** Issue de la dernière invitation, sous le formulaire. */
	let result = $state<{ text: string; link?: string } | null>(null);
	let linkCopied = $state(false);

	async function invite(e: SubmitEvent) {
		e.preventDefault();
		if (busy) return;
		listOpen = false;
		if (emailErr || rolesErr) {
			tried = true;
			if (emailErr) emailEl?.focus();
			return;
		}
		const to = email.trim();
		const who = picked?.display_name || to;
		const res: { out?: InviteOutcome } = {};
		result = null;
		const ok = await run(async () => {
			res.out = await DocumentService.ShareInvitePerson(to, inviteRoles);
			return res.out.view;
		});
		if (!ok || !res.out) return;
		const out = res.out;
		if (out.status === 'granted') result = { text: t('remote.members.invite.granted', { who }) };
		else if (out.status === 'invited' && out.mailed) result = { text: t('remote.members.invite.mailed', { email: to }) };
		else if (out.status === 'invited' && out.link) result = { text: t('remote.members.invite.linkHelp', { email: to }), link: out.link };
		else result = { text: t('remote.members.invited', { email: to }) };
		email = '';
		tried = false;
		picked = null;
		found = [];
	}

	async function copyInviteLink(link: string) {
		try {
			await navigator.clipboard.writeText(link);
			linkCopied = true;
			setTimeout(() => (linkCopied = false), 1500);
		} catch {
			// Presse-papiers refusé : le lien reste sélectionnable.
		}
	}

	// --- Rôles d'une personne ou d'une équipe ---------------------------------------------------
	const grant = (r: Role, principal: string, on: boolean, who: string) =>
		run(() => DocumentService.ShareGrantRole(r.id, principal, on), on ? t('remote.members.roleGiven', { role: roleLabel(r), who }) : t('remote.members.roleRemoved', { role: roleLabel(r), who }));

	/** Retire ses rôles à un membre ou à une équipe (principal : user_id ou team_id), un par un. */
	async function removeAll(principal: string, ids: string[], who: string) {
		if (!confirm(t('remote.members.removeAll.confirm', { who, n: ids.length }))) return;
		for (const id of ids) if (!(await run(() => DocumentService.ShareGrantRole(id, principal, false), t('remote.members.removeAll.done', { who })))) return;
	}

	/** Dernier propriétaire : son rôle ne se retire pas. */
	const lastOwner = (p: MemberRow) => owners <= 1 && p.personal.some((id) => roleById(id)?.builtin === 'owner');
	const addable = (held: string[]) => [{ value: '', label: t('remote.members.addRole'), disabled: true }, ...sorted.filter((r) => !held.includes(r.id)).map((r) => ({ value: r.id, label: roleLabel(r) }))];

	// --- Recherche -------------------------------------------------------------------------
	let query = $state('');
	const shown = $derived(query.trim() ? people.filter((p) => `${p.name} ${p.email}`.toLowerCase().includes(query.trim().toLowerCase())) : people);

	// --- Équipes -------------------------------------------------------------------------------
	let addTeam = $state('');
	let addTeamRole = $state('');
	const teamRole = $derived(roles.some((r) => r.id === addTeamRole) ? addTeamRole : (roles.find((r) => r.builtin === 'reader')?.id ?? ''));
	async function grantTeam(e: SubmitEvent) {
		e.preventDefault();
		const tm = allTeams.find((x) => x.id === addTeam);
		const r = roleById(teamRole);
		if (!tm || !r || busy) return;
		if (await run(() => DocumentService.ShareGrantRole(r.id, tm.id, true), t('remote.members.roleGiven', { role: roleLabel(r), who: t('remote.members.theTeam', { name: tm.name }) }))) addTeam = '';
	}
</script>

<section class="ac-section" aria-labelledby="am-invite">
	<form class="ac-card" onsubmit={invite} novalidate>
		<div class="ac-section-head"><h3 id="am-invite">{t('remote.members.invite.title')}</h3></div>
		<div class="ac-card-row">
			<div class="am-combo">
				<input
					bind:this={emailEl}
					class="sf-input"
					type="text"
					role="combobox"
					placeholder={t('remote.members.invite.placeholder')}
					aria-label={t('remote.members.invite.email')}
					aria-invalid={tried && !!emailErr}
					aria-describedby="am-invite-hint"
					aria-autocomplete="list"
					aria-expanded={listOpen && options.length > 0}
					aria-controls="am-invite-list"
					aria-activedescendant={listOpen && active >= 0 ? `am-invite-opt-${active}` : undefined}
					bind:value={email}
					oninput={emailInput}
					onkeydown={emailKey}
					onblur={() => (listOpen = false)}
					disabled={busy}
					autocomplete="off"
					spellcheck="false"
				/>
				{#if listOpen && options.length}
					<ul class="am-list" id="am-invite-list" role="listbox" aria-label={t('remote.members.invite.suggestions')}>
						{#each options as o, i (o.kind === 'person' ? o.person.id : 'join')}
							<!-- La souris choisit sans ôter le focus du champ ; le clavier passe par le champ (flèches, Entrée). -->
							<!-- svelte-ignore a11y_click_events_have_key_events (option de la liste : le clavier passe par le champ (flèches, Entrée)) -->
							<li
								id="am-invite-opt-{i}"
								role="option"
								aria-selected={i === active}
								aria-disabled={o.kind === 'join' && o.disabled}
								class:on={i === active}
								class:join={o.kind === 'join'}
								onmousedown={(e) => e.preventDefault()}
								onclick={() => choose(o)}
							>
								{#if o.kind === 'person'}
									<span class="ac-avatar" aria-hidden="true">{initial(o.person.display_name || o.person.email)}</span>
									<strong>{o.person.display_name || o.person.email}</strong>
									{#if o.person.display_name}<span class="ac-sub">{o.person.email}</span>{/if}
								{:else}
									<span class="ac-avatar" aria-hidden="true"><Icon name="plus" size={12} /></span>
									<span>{o.disabled && !isEmail(o.email) ? t('remote.members.invite.joinHint') : t('remote.members.invite.join', { email: o.email })}</span>
								{/if}
							</li>
						{/each}
					</ul>
				{/if}
			</div>
			<div class="ac-toggles" class:invalid={tried && !!rolesErr} role="group" aria-label={t('remote.members.invite.roles')}>
				{#each invitable as i (i.value)}
					<button type="button" class="ac-toggle" aria-pressed={inviteRoles.includes(i.value)} disabled={busy} title={i.role ? capsSummary(roleCaps(i.role, a, tables, collab)) : ''} onclick={() => toggleInvite(i.value)}>{i.label}</button>
				{/each}
			</div>
			<button type="submit" class="sf-btn primary" disabled={busy}>{submitLabel}</button>
		</div>
		{#if result}
			<div class="am-result" role="status">
				<span>{result.text}</span>
				{#if result.link}
					<span class="am-link">
						<input class="sf-input" readonly value={result.link} aria-label={t('remote.members.invite.linkAria')} onfocus={(e) => e.currentTarget.select()} />
						<button type="button" class="sf-btn" onclick={() => copyInviteLink(result!.link!)}>{linkCopied ? t('remote.rightsDialog.link.copied') : t('remote.formQR.copy')}</button>
					</span>
				{/if}
			</div>
		{/if}
		{#if tried && (emailErr || rolesErr)}
			<p class="sf-error" id="am-invite-hint" role="alert">{emailErr || rolesErr}</p>
		{:else}
			<p class="sf-hint" id="am-invite-hint">
				{t('remote.members.invite.gives')} <strong>{inviteGives}</strong>. {t('remote.members.invite.help')}{view.customInvite || !roles.some((r) => !r.builtin)
					? ''
					: ` ${t('remote.members.invite.custom')}`}
			</p>
		{/if}
	</form>
</section>

<section class="ac-section" aria-labelledby="am-people">
	<div class="ac-section-head">
		<h3 id="am-people">{t('remote.members.people')}</h3>
		<span class="ac-count">{people.length}</span>
		<span class="ac-grow"></span>
		{#if people.length > 8}
			<input class="sf-input" style="width: 200px; height: 26px" type="search" placeholder={t('remote.members.search')} aria-label={t('remote.members.search')} bind:value={query} />
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
							{#if pipe}<span class="ac-tag" title={t('remote.members.tag.pipeline.tooltip')}>{t('remote.tableSources.pipeline.label')}</span>{/if}
							{#if p.teams.length}<span class="ac-tag team" title={t('remote.members.tag.team.tooltip')}>{t('remote.members.tag.team.label', { names: p.teams.join(', ') })}</span>{/if}
							{#if last}<span class="ac-tag lock" title={t('remote.members.tag.lastOwner.tooltip')}>{t('remote.members.tag.lastOwner.label')}</span>{/if}
						</span>
					{/if}
				</div>
				<div class="ac-roles">
					<div class="ac-chips" role="list" aria-label={t('remote.members.rolesOf', { who })}>
						{#each sorted.filter((r) => p.personal.includes(r.id)) as r (r.id)}
							{@const locked = r.builtin === 'owner' && owners <= 1}
							<span class="ac-chip {roleTone(r)}" class:has-x={!locked && !pipe} role="listitem">
								{#if locked}<Icon name="lock" size={11} />{/if}{roleLabel(r)}
								{#if !locked && !pipe}
									<button type="button" class="ac-x" disabled={busy} aria-label={t('remote.members.removeRole.aria', { role: roleLabel(r), who })} title={t('remote.members.removeRole.tooltip')} onclick={() => grant(r, p.userId, false, who)}>✕</button>
								{/if}
							</span>
						{/each}
						{#each sorted.filter((r) => p.via[r.id] && !p.personal.includes(r.id)) as r (r.id)}
							<span class="ac-chip via {roleTone(r)}" role="listitem" title={t('remote.members.via', { names: p.via[r.id].join(', ') })}>{roleLabel(r)} · {p.via[r.id].join(', ')}</span>
						{/each}
						{#if !pipe}
							{#key p.personal.join()}
								<Select label={t('remote.members.giveRole', { who })} value="" options={addable(p.personal)} disabled={busy} onchange={(id) => id && roleById(id) && grant(roleById(id)!, p.userId, true, who)} />
							{/key}
						{/if}
					</div>
					<span class="ac-can" title={[...caps.read.why, ...caps.editAll.why].join('\n')}>{capsSummary(caps)}</span>
				</div>
				<div class="ac-acts">
					<button type="button" class="ac-icon" disabled={busy || !tables.length} title={tables.length ? t('remote.members.viewAs.tooltip', { who }) : t('remote.preview.noTable')} onclick={() => onpreview(p.userId)}><Icon name="show" size={14} />{t('remote.members.viewAs.label')}</button>
					{#if p.personal.length && !pipe && !last}
						<button type="button" class="ac-icon danger" disabled={busy} title={p.teams.length ? t('remote.members.removeAll.teamKept') : t('remote.members.removeAll.tooltip')} aria-label={t('remote.members.removeAll.aria', { who })} onclick={() => removeAll(p.userId, p.personal, who)}><Icon name="trash" size={14} /></button>
					{/if}
				</div>
			</li>
		{:else}
			<li class="ac-row"><span></span><span class="sf-hint">{query ? t('remote.members.noMatch') : t('remote.members.none')}</span></li>
		{/each}
	</ul>
</section>

{#if acl?.invitations?.length}
	<section class="ac-section" aria-labelledby="am-pending">
		<div class="ac-section-head"><h3 id="am-pending">{t('remote.members.pending')}</h3><span class="ac-count">{acl.invitations.length}</span></div>
		<ul class="ac-list">
			{#each acl.invitations as i (i.id)}
				<li class="ac-row">
					<span class="ac-avatar" aria-hidden="true" style="background: #f1f3f5; color: #6b7280">{initial(i.email)}</span>
					<div class="ac-who"><strong title={i.invited_by ? t('remote.members.invitedBy', { who: i.invited_by }) : undefined}>{i.email}</strong><span class="ac-sub">{t('remote.members.until', { date: fmtDate(new Date(i.expires_at)) })}</span></div>
					<div class="ac-chips">
						{#each i.roles ?? [] as r (r)}
							{@const role = roles.find((x) => x.builtin === r || (!x.builtin && x.name.toLowerCase() === r.toLowerCase()))}
							<span class="ac-chip {roleTone(role)}">{BUILTIN_LABELS[r] ?? r}</span>
						{/each}
					</div>
					<div class="ac-acts">
						<button type="button" class="ac-icon" disabled={busy} onclick={() => run(() => DocumentService.ShareRevokeInvitation(i.id), t('remote.members.revoked', { email: i.email }))}>{t('remote.members.revoke')}</button>
					</div>
				</li>
			{/each}
		</ul>
	</section>
{/if}

<section class="ac-section" aria-labelledby="am-teams">
	<div class="ac-section-head"><h3 id="am-teams">{t('remote.members.teams')}</h3>{#if teamsOn}<span class="ac-count">{teams.length}</span>{/if}</div>
	{#if !teamsOn}
		<p class="sf-hint">{t('remote.members.teamsUnsupported')}</p>
	{:else}
		{#if teams.length}
			<ul class="ac-list">
				{#each teams as tm (tm.teamId)}
					{@const who = t('remote.members.theTeam', { name: tm.name })}
					{@const caps = memberCaps(tm.roles, a, tables, collab)}
					<li class="ac-row">
						<span class="ac-avatar team" aria-hidden="true"><Icon name="user" size={14} /></span>
						<div class="ac-who">
							<strong>{tm.name}</strong>
							{#if tm.count !== undefined}<span class="ac-sub" title={tm.members ? tm.members.map((m) => m.display_name || m.email).join(', ') || t('remote.members.noMember') : ''}>{memberCount(tm.count)}{tm.members?.length ? ` : ${tm.members.map((m) => m.display_name || m.email).join(', ')}` : ''}</span>{/if}
						</div>
						<div class="ac-roles">
							<div class="ac-chips" role="list" aria-label={t('remote.members.rolesOf', { who })}>
								{#each sorted.filter((r) => tm.roles.includes(r.id)) as r (r.id)}
									<span class="ac-chip has-x {roleTone(r)}" role="listitem"
										>{roleLabel(r)}<button type="button" class="ac-x" disabled={busy} aria-label={t('remote.members.removeRole.aria', { role: roleLabel(r), who })} title={t('remote.members.removeRole.tooltip')} onclick={() => grant(r, tm.teamId, false, who)}>✕</button></span
									>
								{/each}
								{#key tm.roles.join()}
									<Select label={t('remote.members.giveRole', { who })} value="" options={addable(tm.roles)} disabled={busy} onchange={(id) => id && roleById(id) && grant(roleById(id)!, tm.teamId, true, who)} />
								{/key}
							</div>
							<span class="ac-can">{capsSummary(caps)}</span>
						</div>
						<div class="ac-acts">
							<button type="button" class="ac-icon danger" disabled={busy} aria-label={t('remote.members.removeAll.aria', { who })} title={t('remote.members.removeAll.team')} onclick={() => removeAll(tm.teamId, tm.roles, who)}><Icon name="trash" size={14} /></button>
						</div>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="sf-hint">{t('remote.members.noTeamRole')}</p>
		{/if}
		{#if allTeams.length}
			<form class="ac-card-row" style="margin-top: 8px" onsubmit={grantTeam}>
				<span class="sf-hint">{t('remote.members.teamGive.label')}</span>
				<Select label={t('remote.audience.team')} value={addTeam} options={[{ value: '', label: t('remote.formDialog.audience.teamChoose'), disabled: true }, ...allTeams.map((tm) => ({ value: tm.id, label: `${tm.name} (${memberCount(tm.member_count)})` }))]} disabled={busy} onchange={(v) => (addTeam = v)} />
				<Select label={t('remote.members.teamGive.role')} value={teamRole} options={sorted.map((r) => ({ value: r.id, label: roleLabel(r) }))} disabled={busy} onchange={(v) => (addTeamRole = v)} />
				<button type="submit" class="sf-btn" disabled={busy || !addTeam}>{t('remote.members.teamGive.submit')}</button>
			</form>
			<p class="sf-hint" style="margin-top: 4px">{t('remote.members.teamGive.help')}</p>
		{:else}
			<p class="sf-hint">{t('remote.members.noTeam')}</p>
		{/if}
	{/if}
</section>

<TableSourcesList />

<style>
	.am-combo {
		position: relative;
		flex: 1 1 220px;
		min-width: 180px;
		display: flex;
	}
	.am-combo .sf-input {
		flex: 1;
		min-width: 0;
	}
	.am-list {
		position: absolute;
		top: calc(100% + 2px);
		left: 0;
		right: 0;
		z-index: 5;
		margin: 0;
		padding: 4px 0;
		list-style: none;
		background: #fff;
		border: 1px solid #d1d5db;
		border-radius: 6px;
		box-shadow: 0 6px 16px rgba(0, 0, 0, 0.12);
		max-height: 260px;
		overflow-y: auto;
	}
	.am-list li {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 5px 10px;
		cursor: pointer;
		min-width: 0;
	}
	.am-list li .ac-sub {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.am-list li.on {
		background: #eef6f1;
	}
	.am-list li.join {
		border-top: 1px solid #eef0f2;
		color: #14532d;
		--icon-accent: #217346;
	}
	.am-list li[aria-disabled='true'] {
		color: #9ca3af;
		cursor: default;
	}
	.am-result {
		margin-top: 8px;
		color: #1b5e3a;
		font-size: 12px;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.am-link {
		display: flex;
		gap: 8px;
	}
	.am-link .sf-input {
		flex: 1;
		min-width: 0;
		height: 26px;
		color: #374151;
	}
	.am-link .sf-btn {
		height: 26px;
	}
</style>
