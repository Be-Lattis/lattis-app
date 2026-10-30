<script lang="ts">
	// Partager au Data Lake Lattis › « Publier uniquement ce tableau » : le
	// tableau devient une source de données du Data Lake liée à ce classeur,
	// sans publier le classeur. Le serveur en garde une copie, alimentée par
	// un pipeline dont ce classeur est la source : seules les colonnes
	// cochées sont envoyées, quand on le demande (Mettre à jour) ou à chaque
	// enregistrement. La même boîte modifie la source, la met à jour et la
	// retire (internal/bridge/table_source.go).
	//
	// Mise en page (docs/audits/audit-ux-partage-data-lake.md) : l'essentiel
	// d'abord (nom, dossier, colonnes, audience), le reste dans « Options
	// avancées » replié avec son résumé ; erreurs sous leur champ ; Entrée
	// publie, Échap ferme (DataShareDialog).
	import Icon from '#lib/icons/Icon.svelte';
	import Select from '#lib/ui/Select.svelte';
	import { openHelp } from '#lib/help/state.svelte.js';
	import { onMount, tick, untrack } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { RightsColumn } from '#bindings/internal/bridge/models.js';
	import { folderPath, type AudienceEntry, type DataAudience, type DataCatalog, type Dataset } from './data';
	import AudienceEditor from './AudienceEditor.svelte';
	import ColumnPicker from './ColumnPicker.svelte';
	import FolderPicker from './FolderPicker.svelte';
	import { connected, hostOf, userName } from './lattis';
	import { lattis, loadLattis, requestLogin } from './lattis.svelte.js';
	import { PUSH_MODES, advancedSummary, keptKeys, pushSummary, shareProblems, sourceStatus, sqlNameFor, type PushMode, type SourceLink } from './source';
	import './shareForm.css';
	import { errorText, t } from '#lib/i18n/index.js';

	interface Props {
		table: string;
		/** Retour au choix entre les deux voies (création seulement). */
		onback?: () => void;
		/** Fermeture ; notice : message de la barre d'état. */
		ondone: (notice?: string) => void;
	}
	let { table, onback, ondone }: Props = $props();

	type Info = {
		reason: string;
		server: string;
		table: string;
		fields: RightsColumn[] | null;
		keys: string[] | null;
		catalog: DataCatalog;
		teams: { id: string; name: string }[] | null;
		link: SourceLink | null;
		dataset: Dataset | null;
	};

	/** Page d'aide du partage au Data Lake. */
	const HELP = 'partage#partager-un-tableau-au-data-lake-lattis';

	let info = $state<Info | null>(null);
	let server = $state('');
	let name = $state(untrack(() => table));
	let description = $state('');
	let sqlName = $state('');
	let sqlTouched = $state(false);
	let folder = $state('');
	let sent = $state<Set<string>>(new Set());
	let key = $state('');
	let mode = $state<PushMode>('replace');
	let onSave = $state(true);
	let suspended = $state(false);
	let inherit = $state(true);
	let current = $state<DataAudience[]>([]);
	let removed = $state<Set<string>>(new Set());
	let adds = $state<AudienceEntry[]>([]);
	let busy = $state(false);
	let error = $state('');
	/** Options avancées dépliées. */
	let adv = $state(false);
	/** Envoi tenté avec des champs à corriger : le pied le dit. */
	let tried = $state(false);
	let body = $state<HTMLDivElement>();
	const err = errorText;
	const link = $derived(info?.link ?? null);
	const servers = $derived(connected(lattis.state.servers));
	const badFilter = $derived(adds.some((a) => !!a.error));
	const fields = $derived(info?.fields ?? []);
	const keyChoices = $derived(fields.filter((f) => sent.has(f.name)));
	const problems = $derived(shareProblems({ server, name, sqlName: link ? null : sqlName, columns: sent.size, mode, key, badFilter }));
	const firstProblem = $derived(Object.values(problems)[0] ?? '');
	const marked = $derived(mode === 'upsert' && key ? [key] : (info?.keys ?? []));
	const modeChanged = $derived(!!link && (mode !== ((link.mode || 'replace') as PushMode) || (mode === 'upsert' && key !== (link.keys?.[0] ?? ''))));
	const summary = $derived(advancedSummary({ sqlName: link ? link.sqlName : sqlName, mode, key, onSave, suspended }));
	const serverLabel = $derived.by(() => {
		const s = servers.find((x) => x.url === server);
		return s ? `${s.name || hostOf(s.url)} · ${userName(s.user)}` : hostOf(server);
	});

	$effect(() => {
		if (!sqlTouched && !link) sqlName = sqlNameFor(name);
	});

	onMount(async () => {
		await loadLattis();
		server = connected(lattis.state.servers)[0]?.url ?? '';
		await load();
	});

	async function load() {
		error = '';
		try {
			const i = (await DocumentService.TableSourceInfo(table, server)) as unknown as Info;
			const first = !info;
			info = i;
			if (i.link) {
				const l = i.link;
				server = l.server;
				name = i.dataset?.name ?? l.name;
				description = i.dataset?.description ?? '';
				sqlName = l.sqlName;
				folder = i.dataset?.folder_id ?? '';
				sent = new Set(l.columns ?? []);
				key = l.keys?.[0] ?? '';
				mode = (l.mode || 'replace') as PushMode;
				onSave = l.onSave;
				suspended = !!l.suspended;
				inherit = i.dataset?.source?.inherit_audience ?? true;
				current = i.dataset?.audience ?? [];
			} else if (first) {
				sent = new Set((i.fields ?? []).map((f) => f.name));
				key = i.keys?.[0] ?? '';
			}
		} catch (e) {
			error = err(e);
		}
	}

	async function login() {
		if (await requestLogin('')) {
			await loadLattis();
			server = connected(lattis.state.servers)[0]?.url ?? server;
			await load();
		}
	}

	function setColumns(s: Set<string>) {
		sent = s;
		key = keptKeys(key ? [key] : [], s)[0] ?? '';
	}

	async function createFolder(n: string, parent: string): Promise<string> {
		const id = await DocumentService.CreateDataFolder(server, n, parent);
		if (info) info.catalog = (await DocumentService.DataCatalog(server)) as unknown as DataCatalog;
		return id;
	}

	async function values(column: string): Promise<string[]> {
		return ((await DocumentService.FieldValues(table, column)) ?? []).map((v) => v.text);
	}

	function request() {
		return {
			server,
			table,
			folderId: folder,
			name: name.trim(),
			description: description.trim(),
			sqlName: link ? '' : sqlName.trim(),
			columns: fields.map((f) => f.name).filter((c) => sent.has(c)),
			mode,
			keys: mode === 'upsert' && key ? [key] : [],
			onSave,
			inherit,
			suspended,
			add: adds.filter((a) => a.email.trim() || a.team).map(({ email, team, predicate }) => ({ email, team, predicate })),
			remove: [...removed]
		};
	}

	/** Le compte rendu d'un envoi, ou son échec, pour la barre d'état. */
	function said(l: SourceLink): string {
		if (l.suspended) return t('remote.sourceForm.suspended', { table });
		return l.lastError ? t('remote.sourceForm.savedNotUpdated', { table, error: l.lastError }) : pushSummary(table, l.lastResult, l.blanked ?? 0);
	}

	/** Publier : d'abord les champs à corriger (déplie et met le focus sur le premier). */
	async function attempt() {
		if (!info || info.reason || busy) return;
		if (firstProblem) {
			tried = true;
			if (problems.sql || problems.key) adv = true;
			await tick();
			const sel = problems.name ? '#src-name' : problems.columns ? '.sf-cols input' : problems.sql ? '#src-sql' : problems.key ? '#src-key' : problems.audience ? '.sf-aud textarea, .sf-aud input' : '';
			body?.querySelector<HTMLElement>(sel)?.focus();
			return;
		}
		await save();
	}

	async function save() {
		if (!info) return;
		error = '';
		busy = true;
		try {
			const l = (link ? await DocumentService.UpdateTableSource(request()) : await DocumentService.CreateTableSource(request())) as unknown as SourceLink;
			ondone(said(l));
		} catch (e) {
			error = err(e);
			await load();
		} finally {
			busy = false;
		}
	}

	async function push() {
		busy = true;
		error = '';
		try {
			const l = (await DocumentService.PushTableSource(table)) as unknown as SourceLink;
			ondone(pushSummary(table, l.lastResult, l.blanked ?? 0));
		} catch (e) {
			error = err(e);
			await load();
		} finally {
			busy = false;
		}
	}

	async function withdraw() {
		busy = true;
		error = '';
		try {
			await DocumentService.DropTableSource(table);
			ondone(t('remote.sourceForm.withdrawn', { name }));
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}

	/** Entrée dans un champ texte : publier (pas dans un filtre SQL ni une liste ouverte). */
	function onKey(e: KeyboardEvent) {
		const el = e.target as HTMLElement;
		if (e.key !== 'Enter' || e.defaultPrevented || e.shiftKey || e.ctrlKey || e.altKey) return;
		if (!(el instanceof HTMLInputElement) || ['checkbox', 'radio', 'button'].includes(el.type)) return;
		e.preventDefault();
		attempt();
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions (Entrée et Échap captés pour toute la boîte ; ses commandes sont des boutons et des champs) -->
<div class="sf-body" bind:this={body} onkeydown={onKey}>
	{#if !info}
		<p class="sf-hint">{error ? '' : t('remote.sourceForm.reading')}</p>
	{:else}
		<p class="sf-banner">
			<Icon name="local" size={14} />
			<span>{t('remote.sourceForm.banner')}</span>
			<button type="button" class="sf-more" onclick={() => openHelp(HELP)}>{t('remote.roles.learnMore')}</button>
		</p>
		{#if link && !info.reason}
			<p class="sf-status" class:failed={!!link.lastError} role="status">
				<Icon name={link.lastError ? 'warning' : 'refresh'} size={14} />{sourceStatus(link)}
			</p>
		{/if}
		<div class="sf-fields">
			<span class="sf-label">{t('remote.serverPicker.aria')}</span>
			<div class="sf-static">
				{#if link}
					<span>{hostOf(link.server)}</span><span class="sf-tag" title={t('remote.tableSources.pipeline.tooltip')}>{t('remote.tableSources.pipeline.label')}</span>
				{:else if servers.length > 1}
					<Select
						label={t('remote.serverPicker.aria')}
						value={server}
						options={servers.map((s) => ({ value: s.url, label: `${s.name || hostOf(s.url)} · ${userName(s.user)}` }))}
						disabled={busy}
						onchange={(v) => {
							server = v;
							load();
						}}
					/>
				{:else if servers.length}
					<span>{serverLabel}</span>
				{:else}
					<button type="button" class="sf-btn" onclick={login} disabled={busy}>{t('remote.account.connectServer')}</button>
				{/if}
			</div>
		</div>
		{#if info.reason}
			<p class="sf-error" role="alert" style="margin-top: 10px">{info.reason}</p>
		{:else}
			<div class="sf-fields" style="margin-top: 10px">
				<label class="sf-label" for="src-name">{t('remote.rules.name')}</label>
				<div class="sf-field">
					<input id="src-name" class="sf-input" bind:value={name} disabled={busy} aria-invalid={!!problems.name} aria-describedby={problems.name ? 'src-name-err' : undefined} autocomplete="off" />
					{#if problems.name}<p class="sf-error" id="src-name-err">{problems.name}</p>{/if}
				</div>

				<span class="sf-label" id="src-folder-l">{t('remote.dataShare.folder')}</span>
				<div class="sf-field">
					<FolderPicker id="src-folder" catalog={info.catalog} bind:value={folder} disabled={busy || !server} oncreate={createFolder} />
				</div>

				<span class="sf-label">{t('remote.columnPicker.aria')}</span>
				<div class="sf-field sf-cols">
					<ColumnPicker label={t('remote.sourceForm.columns')} columns={fields} selected={sent} onchange={setColumns} keys={marked} disabled={busy} error={problems.columns ?? ''} />
					<p class="sf-hint">{t('remote.sourceForm.computed')}</p>
				</div>

				<span class="sf-label">{t('remote.dataShare.audience')}</span>
				<div class="sf-field sf-aud">
					<AudienceEditor {current} bind:removed bind:adds bind:inherit teams={info.teams ?? []} columns={keyChoices} folder={folderPath(info.catalog, folder)} {table} {values} disabled={busy} />
				</div>

				{#if info.dataset?.used_by?.length}
					<span class="sf-label">{t('remote.dataShare.usedBy')}</span>
					<ul class="sf-used">
						{#each info.dataset.used_by as u (u.workbook_id)}<li>{u.title}</li>{/each}
					</ul>
				{/if}
			</div>

			<details class="sf-adv" bind:open={adv}>
				<summary>{t('remote.dataShare.advanced')} <span class="sf-sum">{summary}</span></summary>
				<div class="sf-fields">
					<label class="sf-label" for="src-desc">{t('remote.dataShare.description.label')} <span class="sf-opt">{t('remote.rules.optional')}</span></label>
					<input id="src-desc" class="sf-input" bind:value={description} disabled={busy} placeholder={t('remote.dataShare.description.placeholder')} />

					<label class="sf-label" for="src-sql">{t('remote.dataShare.sqlName.label')}</label>
					<div class="sf-field">
						{#if link}
							<div class="sf-static"><code class="sf-code">donnees.{link.sqlName}</code><span class="sf-hint">{t('remote.dataShare.sqlName.fixed')}</span></div>
						{:else}
							<div class="sf-prefixed">
								<span>donnees.</span>
								<input id="src-sql" class="sf-input" bind:value={sqlName} oninput={() => (sqlTouched = true)} disabled={busy} placeholder={t('remote.sourceForm.sqlPlaceholder')} spellcheck="false" aria-invalid={!!problems.sql} aria-describedby="src-sql-hint" />
							</div>
							<p class={problems.sql ? 'sf-error' : 'sf-hint'} id="src-sql-hint">{problems.sql ?? t('remote.dataShare.sqlName.help')}</p>
						{/if}
					</div>

					<span class="sf-label" id="src-mode-l">{t('remote.sourceForm.mode')}</span>
					<div class="sf-field">
						<div class="sf-seg" role="radiogroup" aria-labelledby="src-mode-l">
							{#each PUSH_MODES as m (m.mode)}
								<label class:on={mode === m.mode} title={m.detail}><input type="radio" name="src-mode" value={m.mode} bind:group={mode} disabled={busy} />{m.label}</label>
							{/each}
						</div>
						<p class="sf-hint">{PUSH_MODES.find((m) => m.mode === mode)?.detail}</p>
						{#if mode === 'upsert'}
							<div class="sf-inline">
								<label class="sf-label" for="src-key" style="padding-top: 0">{t('remote.sourceForm.key.label')}</label>
								<Select id="src-key" label={t('remote.sourceForm.key.aria')} value={key} options={[{ value: '', label: t('remote.sourceForm.key.choose') }, ...keyChoices.map((c) => ({ value: c.name, label: c.name }))]} disabled={busy} onchange={(v) => (key = v)} />
							</div>
							{#if problems.key}<p class="sf-error">{problems.key}</p>{/if}
						{/if}
						{#if modeChanged}<p class="sf-hint">{t('remote.sourceForm.modeChanged')}</p>{/if}
					</div>

					<span class="sf-label">{t('remote.sourceForm.sending.label')}</span>
					<div class="sf-field">
						<label class="sf-check"><input type="checkbox" bind:checked={onSave} disabled={busy} /> {t('remote.sourceForm.sending.onSave')}</label>
						{#if link}
							<label class="sf-check" title={t('remote.sourceForm.sending.suspend.tooltip')}><input type="checkbox" bind:checked={suspended} disabled={busy} /> {t('remote.sourceForm.sending.suspend.label')}</label>
						{/if}
						<p class="sf-hint">{t('remote.sourceForm.sending.refused')}</p>
					</div>
				</div>
			</details>
		{/if}
	{/if}
	{#if error}<p class="sf-error" role="alert" style="margin-top: 10px">{error}</p>{/if}
</div>
<footer class="sf-foot">
	{#if link}
		<button type="button" class="sf-btn danger" onclick={withdraw} disabled={busy} title={t('remote.sourceForm.withdraw.tooltip')}>{t('remote.sourceForm.withdraw.label')}</button>
		<button type="button" class="sf-btn" onclick={push} disabled={busy || !!info?.reason || !!link.suspended} title={link.suspended ? t('remote.sourceForm.push.suspended') : t('remote.sourceForm.push.tooltip')}>{t('remote.sourceForm.push.label')}</button>
	{:else if onback}
		<button type="button" class="sf-btn ghost" onclick={onback} disabled={busy}>‹ {t('remote.sourceForm.back')}</button>
	{/if}
	{#if tried && firstProblem && info && !info.reason}
		<p class="sf-error" role="alert">{firstProblem}</p>
	{:else}
		<span class="sf-grow"></span>
	{/if}
	<button type="button" class="sf-btn" onclick={() => ondone()} disabled={busy}>{info?.reason ? t('common.close') : t('common.cancel')}</button>
	{#if info && !info.reason}
		<button type="button" class="sf-btn primary" onclick={attempt} disabled={busy}>{busy ? t('remote.tableSources.sending') : link ? (suspended ? t('remote.formDialog.save') : t('remote.sourceForm.saveUpdate')) : t('remote.sourceForm.publish')}</button>
	{/if}
</footer>
