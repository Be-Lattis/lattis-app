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
	import { t } from '#lib/i18n/index.js';

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
		return { people, teams, text: `${memberCount(people)}${teams ? ` · ${t('remote.roles.teamCount', { n: teams })}` : ''}` };
	};
	const roleKind = (r: Role) => (r.builtin === 'owner' ? t('remote.roles.kind.owner') : r.builtin === 'designer' ? t('remote.roles.kind.designer') : r.builtin ? t('remote.rights.sql.builtinRole') : t('remote.rights.sql.customRole'));

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

	function openFilters(tb: RightsTable) {
		if (!role) return;
		const cur = tableLevel(role, wbId, tb.remoteId, tb.sheetKey || undefined);
		editing = { table: tb.table, read: cur.read ?? '', write: cur.write ?? '', readErr: '', writeErr: '' };
		tick().then(() => document.querySelector('.ac-editor')?.scrollIntoView({ block: 'nearest' }));
	}

	async function saveFilters(tb: RightsTable) {
		const e = editing;
		if (!e || !role || busy) return;
		if (e.read === null || e.write === null) return;
		const cur = tableLevel(role, wbId, tb.remoteId, tb.sheetKey || undefined);
		const level = cur.own ?? cur.effective;
		const ok = await run(
			() => DocumentService.ShareSetLevels([{ roleId: role.id, nodeKind: 'table', nodeId: tb.remoteId, level, readFilter: e.read ?? '', writeFilter: e.write ?? '' }]),
			t('remote.roles.filtersSaved', { table: tb.name, role: roleLabel(role) })
		);
		if (ok) editing = null;
	}

	async function values(tb: RightsTable, column: string): Promise<string[]> {
		return ((await DocumentService.FieldValues(tb.name, column)) ?? []).map((v) => v.text);
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
	<span>{t('remote.roles.intro')}</span>
	<button type="button" class="sf-more" onclick={() => openHelp(help)}>{t('remote.roles.learnMore')}</button>
</p>

<table class="ac-matrix" aria-label={t('remote.roles.matrix.aria')}>
	<thead>
		<tr>
			<th scope="col">{t('remote.roles.matrix.role')}</th>
			{#each CAPS as c (c.id)}<th scope="col" title={c.hint}>{c.label}</th>{/each}
			<th scope="col">{t('remote.roles.matrix.members')}</th>
			<th scope="col" aria-label={t('remote.members.viewAs.label')}></th>
		</tr>
	</thead>
	<tbody>
		{#each roles as r (r.id)}
			{@const caps = roleCaps(r, a, tables, collab)}
			{@const h = holders(r)}
			<tr class:on={role?.id === r.id}>
				<th scope="row" style="font-weight: normal; padding: 0; text-align: left">
					<button type="button" class="ac-rolebtn" aria-pressed={role?.id === r.id} title={t('remote.roles.matrix.select')} onclick={() => ((roleId = r.id), (editing = null))}>
						<strong>{roleLabel(r)}</strong><span>{roleKind(r)}</span>
					</button>
				</th>
				{#each CAPS as c (c.id)}<td><CapMark cell={caps[c.id]} label="{roleLabel(r)} · {c.label}" /></td>{/each}
				<td class="ac-members">{h.text}</td>
				<td>
					<button type="button" class="ac-icon" disabled={busy || !h.people || !tables.length} title={h.people ? t('remote.roles.viewAs.tooltip') : t('remote.roles.viewAs.nobody')} aria-label={t('remote.roles.viewAs.aria', { role: roleLabel(r) })} onclick={() => onpreview(r.id)}><Icon name="show" size={14} /></button>
				</td>
			</tr>
		{/each}
	</tbody>
</table>
<div class="ac-legend">
	<span><span class="ac-cap yes" aria-hidden="true">✓</span>{t('remote.cap.yes')}</span>
	<span><span class="ac-cap some" aria-hidden="true">◐</span>{t('remote.roles.legend.some')}</span>
	<span><span class="ac-cap no" aria-hidden="true">–</span>{t('remote.cap.no')}</span>
	<span class="ac-grow"></span>
	{#if collab}
		<form
			class="ac-newrole"
			onsubmit={async (e) => {
				e.preventDefault();
				const n = newRole.trim();
				if (!n || busy) return;
				if (await run(() => DocumentService.ShareCreateRole(n), t('remote.roles.created', { name: n }))) {
					newRole = '';
					roleId = (view.acl?.roles ?? []).find((r) => r.name === n)?.id ?? roleId;
				}
			}}
		>
			<input class="sf-input" placeholder={t('remote.roles.new.placeholder')} aria-label={t('remote.roles.new.aria')} bind:value={newRole} disabled={busy} />
			<button type="submit" class="sf-btn" disabled={busy || !newRole.trim()}><Icon name="plus" size={13} /> {t('remote.folder.create')}</button>
		</form>
	{/if}
</div>

{#if role}
	<section class="ac-detail" aria-labelledby="ar-detail">
		<div class="ac-detail-head">
			<h3 id="ar-detail">{t('remote.roles.detail.title', { role: roleLabel(role) })}</h3>
			{#if !role.builtin}
				<button
					type="button"
					class="ac-link danger"
					disabled={busy}
					onclick={() => {
						if (confirm(t('remote.roles.drop.confirm', { name: role.name }))) run(() => DocumentService.ShareDropRole(role.id), t('remote.roles.drop.done', { name: role.name }));
					}}>{t('remote.roles.drop.label')}</button
				>
			{/if}
		</div>
		<div class="ac-detail-body">
			{#if fullRights(role)}
				<p style="margin: 8px 0">
					{role.builtin === 'owner'
						? t('remote.roles.detail.owner')
						: t('remote.roles.detail.designer')}
				</p>
			{:else if !collab}
				<p class="sf-hint" style="margin: 8px 0">{t('remote.roles.detail.frozen')}</p>
			{:else}
				{@const wl = workbookLevel(role, wbId)}
				<ul class="ac-tree">
					<li class="ac-node">
						<span class="ac-where"><Icon name="book" size={14} /><span>{t('remote.roles.tree.workbook')}</span></span>
						<span class="ac-level"><Select label={t('remote.roles.tree.workbookAccess')} value={wl} options={levelOptions()} disabled={busy} width="100%" title={t('remote.roles.tree.level', { word: LEVEL_WORD[wl] ?? wl })} onchange={(v) => setLevel('workbook', wbId, v)} /></span>
						<span class="ac-gives">{capsSummary(levelCaps(wl))}</span>
					</li>
					{#each sheets as s (s.sheet)}
						{@const sl = sheetLevel(role, wbId, s)}
						<li class="ac-node sheet">
							<span class="ac-where"><Icon name="sheet" size={14} /><span title={t('remote.roles.tree.sheet')}>{s.name}</span></span>
							<span class="ac-level">
								<Select label={t('remote.roles.tree.sheetAccess', { name: s.name })} value={sl.mixed ? '*' : (sl.own ?? '')} options={levelOptions({ from: 'workbook', level: wl }, sl.mixed)} disabled={busy} width="100%" onchange={(v) => setLevel('sheet', String(s.sheet), v)} />
							</span>
							<span class="ac-gives">
								{#if sl.mixed}<span class="ac-inherit">{t('remote.roles.tree.mixed')}</span>{:else}{capsSummary(levelCaps(sl.effective))}{#if sl.own === undefined}<span class="ac-inherit">· {t('remote.roles.tree.fromWorkbook')}</span>{/if}{/if}
							</span>
						</li>
						{#each s.tables as tb (tb.table)}
							{@const tl = tableLevel(role, wbId, tb.remoteId, tb.sheetKey || undefined)}
							{@const from = serverSheet(s) ? inheritedFrom(role, tb.sheetKey || undefined) : 'workbook'}
							{@const caps = tableCaps(role, a, tb)}
							{@const types = columnTypes(tb)}
							{@const rules = (acl?.restrictions ?? []).filter((x) => x.table_id === tb.remoteId && ruleApplies(x, role))}
							{@const ed = editing?.table === tb.table ? editing : null}
							<li class="ac-node table">
								<span class="ac-where"><Icon name="table" size={14} /><span>{tb.name}</span></span>
								<span class="ac-level">
									<Select label={t('remote.roles.tree.tableAccess', { name: tb.name })} value={tl.own ?? ''} options={levelOptions({ from, level: parentLevel(role, wbId, tb.sheetKey || undefined) })} disabled={busy} width="100%" onchange={(v) => setLevel('table', tb.remoteId, v)} />
								</span>
								<span class="ac-gives" title={[...new Set(CAPS.flatMap((c) => caps[c.id].why))].join('\n')}>
									{capsSummary(caps)}{#if tl.own === undefined}<span class="ac-inherit">· {from === 'sheet' ? t('remote.roles.tree.fromSheet') : t('remote.roles.tree.fromWorkbook')}</span>{/if}
								</span>
								{#if tl.effective !== 'none'}
									<div class="ac-filters">
										<div class="ac-fline" class:limited={!!tl.read}>
											<span>{tl.read ? readFilterSentence(filterText(tl.read, types)) : t('remote.roles.filter.seesAll')}</span>
											{#if !ed}<button type="button" class="ac-link" disabled={busy} onclick={() => openFilters(tb)} aria-label={tl.read ? t('remote.roles.filter.editRead', { table: tb.name }) : t('remote.roles.filter.limitRead', { table: tb.name })}>{tl.read ? t('remote.roles.filter.edit') : t('remote.roles.filter.limit')}</button>{/if}
										</div>
										{#if tl.effective !== 'reader'}
											<div class="ac-fline" class:limited={!!tl.write}>
												<span>{tl.write ? writeFilterSentence(filterText(tl.write, types)) : tl.effective === 'participant' ? t('remote.roles.filter.writesOwn') : t('remote.roles.filter.writesAll')}</span>
												{#if !ed}<button type="button" class="ac-link" disabled={busy} onclick={() => openFilters(tb)} aria-label={tl.write ? t('remote.roles.filter.editWrite', { table: tb.name }) : t('remote.roles.filter.limitWrite', { table: tb.name })}>{tl.write ? t('remote.roles.filter.edit') : t('remote.roles.filter.limit')}</button>{/if}
											</div>
										{/if}
										{#each rules as x (x.id)}
											<div class="ac-fline rule">
												<span><Icon name="warning" size={12} /> {ruleSentence({ commands: [x.command], roles: x.applies_to?.length ? [roleLabel(role)] : [], table: tb.name, predicate: x.predicate }, types)}</span>
												<button type="button" class="ac-link" onclick={onrules}>{t('remote.roles.advancedRules')}</button>
											</div>
										{/each}
									</div>
								{/if}
								{#if ed}
									<div class="ac-editor" role="group" aria-label={t('remote.roles.editor.aria', { table: tb.name, role: roleLabel(role) })}>
										<FilterEditor
											label={t('remote.roles.editor.read.label')}
											readback={{ prefix: t('remote.roles.editor.read.prefix'), none: t('remote.roles.filter.seesAll') }}
											columns={tb.columns ?? []}
											value={ed.read ?? ''}
											disabled={busy}
											teams={teamsOn}
											values={(c) => values(tb, c)}
											assist={t('remote.roles.editor.read.assist', { table: tb.name, role: roleLabel(role) })}
											onchange={(sql, e) => ((ed.read = sql), (ed.readErr = e))}
										/>
										{#if tl.effective !== 'reader'}
											<FilterEditor
												label={t('remote.roles.editor.write.label')}
												readback={{ prefix: t('remote.roles.editor.write.prefix'), none: t('remote.audience.allRows') }}
												columns={tb.columns ?? []}
												value={ed.write ?? ''}
												disabled={busy}
												teams={teamsOn}
												values={(c) => values(tb, c)}
												assist={t('remote.roles.editor.write.assist', { table: tb.name, role: roleLabel(role) })}
												onchange={(sql, e) => ((ed.write = sql), (ed.writeErr = e))}
											/>
										{/if}
										<div class="ac-editor-actions">
											<span class="sf-hint">{tl.own === undefined ? t('remote.roles.editor.fixes', { level: levelOptions().find((o) => o.value === tl.effective)?.label ?? '' }) : t('remote.roles.editor.noCondition')}</span>
											<button type="button" class="sf-btn" disabled={busy} onclick={() => (editing = null)}>{t('common.cancel')}</button>
											<button type="button" class="sf-btn primary" disabled={busy || ed.read === null || ed.write === null} title={ed.readErr || ed.writeErr || ''} onclick={() => saveFilters(tb)}>{t('remote.roles.editor.save')}</button>
										</div>
									</div>
								{/if}
							</li>
						{/each}
					{/each}
				</ul>
				{#if sheets.some((s) => !serverSheet(s))}
					<p class="sf-hint" style="margin-top: 6px">{t('remote.roles.oldSheet')}</p>
				{/if}
				{#if !tables.length}
					<p class="sf-hint" style="margin-top: 6px">{t('remote.roles.noTable')}</p>
				{/if}
			{/if}
			{#if teamsOn}
				<p class="sf-hint" style="margin-top: 8px">
					{#if role.teams?.length}
						{t('remote.roles.teams')}
						{#each role.teams as tm, i (tm.team_id)}{@const n = (view.teams ?? []).find((x) => x.id === tm.team_id)?.member_count}{i ? ', ' : ''}<strong>{tm.name}</strong>{n !== undefined ? ` (${memberCount(n)})` : ''}{/each}.
					{:else}
						{t('remote.roles.noTeams')}
					{/if}
				</p>
			{/if}
		</div>
	</section>
{/if}

{#if onaddshared && view.localTables?.length}
	<section class="ac-section" style="margin-top: 14px" aria-labelledby="ar-local">
		<div class="ac-section-head"><h3 id="ar-local">{t('remote.roles.local.title')}</h3></div>
		<p class="sf-hint">{t('remote.roles.local.help')}</p>
		<div class="ac-chips" style="margin-top: 4px">
			{#each view.localTables as lt (lt)}
				<button type="button" class="sf-btn" disabled={busy || !view.online} title={view.online ? t('remote.roles.local.add.tooltip', { table: lt }) : t('remote.roles.local.offline')} onclick={() => addShared(lt)}
					><Icon name="plus" size={13} /> {t('remote.roles.local.add.label', { table: lt })}</button
				>
			{/each}
		</div>
	</section>
{/if}
