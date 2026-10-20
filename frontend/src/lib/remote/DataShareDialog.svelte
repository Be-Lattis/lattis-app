<script lang="ts">
	// Clic droit sur un tableau › Partager au Data Lake Lattis… : le
	// tableau publié devient une donnée du catalogue du serveur, lisible
	// depuis d'autres classeurs par son audience. La même boîte modifie une
	// donnée existante (nom, dossier, colonnes, audience) et retire son
	// partage ; elle montre les classeurs qui l'utilisent.
	//
	// Un tableau hors d'un classeur publié offre deux voies : publier le
	// classeur entier avec gestion des droits (Publier sur un serveur, puis
	// revenir ici), ou publier uniquement ce tableau comme source de données
	// liée au classeur (TableSourceForm : pipeline alimenté par le poste,
	// sans mettre le classeur à disposition).
	//
	// Mise en page commune aux deux formulaires : shareForm.css (voir
	// docs/audits/audit-ux-partage-data-lake.md).
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import Icon from '#lib/icons/Icon.svelte';
	import HelpLink from '#lib/help/HelpLink.svelte';
	import { onMount, tick, untrack } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import { folderPath, type AudienceEntry, type DataAudience, type DataCatalog, type Dataset } from './data';
	import AudienceEditor from './AudienceEditor.svelte';
	import ColumnPicker from './ColumnPicker.svelte';
	import FolderPicker from './FolderPicker.svelte';
	import TableSourceForm from './TableSourceForm.svelte';
	import { chooser, shareProblems, type ShareWay, type SourceLink } from './source';
	import type { RightsColumn } from '#bindings/internal/bridge/models.js';
	import './shareForm.css';
	import { errorText, fmtDateTime, t } from '#lib/i18n/index.js';

	interface Props {
		table: string;
		ondone: () => void;
		/** Publier (ou republier) le classeur : la boîte se ferme pour Fichier › Publier. */
		onpublish?: (republish: boolean) => void;
		/** Message de la barre d'état (source créée, mise à jour, retirée). */
		onnotice?: (msg: string) => void;
		/** Ajouter le tableau au classeur partagé en direct (la boîte se ferme ; confirmation et envoi par la page). */
		onaddshared?: () => void;
	}
	let { table, ondone, onpublish, onnotice, onaddshared }: Props = $props();

	type Info = {
		reason: string;
		server: string;
		workbookId: string;
		tableId: string;
		columns: string[] | null;
		/** Colonnes et types : formulaire du filtre (comme ceux des rôles). */
		fields: RightsColumn[] | null;
		catalog: DataCatalog;
		teams: { id: string; name: string }[] | null;
		dataset: Dataset | null;
		/** workbook, table : deux voies à proposer ; "" : partage direct. */
		choice: string;
		source: SourceLink | null;
	};
	/** Page d'aide du partage au Data Lake. */
	const HELP = 'fichiers#partager-un-tableau-au-data-lake-lattis';
	/** Étape : choix de la voie, partage d'un tableau publié, source sans le classeur. */
	let step = $state<'choose' | 'dataset' | 'source'>('dataset');
	let way = $state<ShareWay>('workbook');
	/** « Pourquoi pas tout le classeur ? » déplié. */
	let whyOpen = $state(false);

	let info = $state<Info | null>(null);
	let name = $state(untrack(() => table));
	let description = $state('');
	let sqlName = $state('');
	let folder = $state('');
	let shown = $state<Set<string>>(new Set());
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
	const ds = $derived(info?.dataset ?? null);
	const choose = $derived(info ? chooser(info.choice, info.reason) : null);
	const ways = $derived(choose?.ways ?? []);
	const badFilter = $derived(adds.some((a) => !!a.error));
	const problems = $derived(info ? shareProblems({ server: info.server, name, sqlName: ds ? null : sqlName, columns: shown.size, badFilter }) : {});
	const firstProblem = $derived(Object.values(problems)[0] ?? '');
	const colTypes = $derived((info?.columns ?? []).map((c) => ({ name: c, type: info?.fields?.find((f) => f.name === c)?.type })));
	const wayOk = $derived(!ways.find((w) => w.way === way)?.disabled && !(way === 'workbook' && !onpublish) && !(way === 'shared' && !onaddshared));

	onMount(load);

	async function load() {
		try {
			const i = (await DocumentService.DataShareInfo(table)) as unknown as Info;
			info = i;
			if (i.source) step = 'source';
			else if (i.choice) {
				step = 'choose';
				way = i.choice === 'collab' ? (i.reason ? 'table' : 'shared') : i.reason ? 'table' : 'workbook';
			}
			const d = i.dataset;
			shown = new Set(i.columns ?? []);
			if (d) {
				name = d.name;
				description = d.description ?? '';
				sqlName = d.sql_name;
				folder = d.folder_id ?? '';
				inherit = d.source?.inherit_audience ?? true;
				for (const h of d.source?.hidden_columns ?? []) shown.delete(h);
				shown = new Set(shown);
				current = d.audience ?? [];
			}
		} catch (e) {
			error = err(e);
		}
	}

	async function createFolder(n: string, parent: string): Promise<string> {
		if (!info) return '';
		const id = await DocumentService.CreateDataFolder(info.server, n, parent);
		info.catalog = (await DocumentService.DataCatalog(info.server)) as unknown as DataCatalog;
		return id;
	}

	async function values(column: string): Promise<string[]> {
		return ((await DocumentService.FieldValues(table, column)) ?? []).map((v) => v.text);
	}

	/** Choix de la voie validé (Continuer, Entrée, double-clic). */
	function proceed() {
		if (!wayOk) return;
		if (way === 'table') step = 'source';
		else if (way === 'shared') {
			ondone();
			onaddshared?.();
		} else {
			ondone();
			onpublish?.(info?.choice === 'table');
		}
	}

	/** Partager : d'abord les champs à corriger (focus sur le premier). */
	async function attempt() {
		if (!info || info.reason || busy) return;
		if (firstProblem) {
			tried = true;
			if (problems.sql) adv = true;
			await tick();
			const sel = problems.name ? '#ds-name' : problems.columns ? '.sf-cols input' : problems.sql ? '#ds-sql' : '';
			if (sel) body?.querySelector<HTMLElement>(sel)?.focus();
			return;
		}
		await save();
	}

	/** Entrée dans un champ texte : partager. */
	function onKey(e: KeyboardEvent) {
		const el = e.target as HTMLElement;
		if (e.key !== 'Enter' || e.defaultPrevented || e.shiftKey || e.ctrlKey || e.altKey) return;
		if (!(el instanceof HTMLInputElement) || ['checkbox', 'radio', 'button'].includes(el.type)) return;
		e.preventDefault();
		attempt();
	}

	async function save() {
		if (!info) return;
		error = '';
		busy = true;
		try {
			await DocumentService.ShareDataset({
				server: info.server,
				dataset: ds?.id ?? '',
				workbookId: info.workbookId,
				tableId: info.tableId,
				folderId: folder,
				name: name.trim(),
				description: description.trim(),
				sqlName: ds ? '' : sqlName.trim(),
				hidden: (info.columns ?? []).filter((c) => !shown.has(c)),
				inherit,
				add: adds.filter((a) => a.email.trim() || a.team).map(({ email, team, predicate }) => ({ email, team, predicate })),
				remove: [...removed]
			});
			ondone();
		} catch (e) {
			error = err(e);
			await load(); // ce qui est passé (donnée créée) s'affiche
		} finally {
			busy = false;
		}
	}

	async function withdraw() {
		if (!info || !ds) return;
		busy = true;
		try {
			await DocumentService.DropDataset(info.server, ds.id);
			ondone();
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}
</script>

<div class="back" role="presentation" use:backdrop={() => !busy && ondone()}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-labelledby="dls-title"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape' && !busy && !e.defaultPrevented) ondone();
		}}
	>
		<header>
			<Icon name="database" size={18} /><span id="dls-title" class="title"
				>{step === 'source' && info?.source ? t('remote.dataShare.title.source', { table }) : ds ? t('remote.dataShare.title.dataset', { name: ds.name }) : t('remote.dataShare.title.share', { table })}</span
			>
			<HelpLink target={HELP} label={t('remote.dataShare.help')} />
			<button type="button" class="close" aria-label={t('common.close')} title={t('remote.closeEsc')} disabled={busy} onclick={ondone}>✕</button>
		</header>
		{#if step === 'source'}
			<TableSourceForm
				{table}
				onback={info?.choice && !info.source ? () => (step = 'choose') : undefined}
				ondone={(msg) => {
					if (msg) onnotice?.(msg);
					ondone();
				}}
			/>
		{:else if step === 'choose' && info}
			<div class="sf-body">
				<h2 class="ctitle">{choose?.title}</h2>
				<p class="lead">
					{choose?.lead}
					{#if choose?.why}<button type="button" class="why" title={choose.why} aria-expanded={whyOpen} onclick={() => (whyOpen = !whyOpen)}>{t('remote.dataShare.whyNotWorkbook')}</button>{/if}
				</p>
				{#if whyOpen && choose?.why}<p class="sf-hint whytext">{choose.why}</p>{/if}
				<!-- svelte-ignore a11y_interactive_supports_focus -->
				<div
					class="ways"
					role="radiogroup"
					aria-label={t('remote.dataShare.ways')}
					onkeydown={(e) => {
						if (e.key === 'Enter') {
							e.preventDefault();
							proceed();
						}
					}}
				>
					{#each ways as w (w.way)}
						<label class="way" class:on={way === w.way} class:off={!!w.disabled} ondblclick={() => !w.disabled && proceed()}>
							<input type="radio" name="way" value={w.way} bind:group={way} disabled={!!w.disabled} />
							<span class="way-icon"><Icon name={w.way === 'workbook' ? 'lock-book' : w.way === 'shared' ? 'book' : 'table'} size={22} /></span>
							<span class="way-text">
								<strong>{w.title}</strong>
								<span class="sf-hint">{w.detail}</span>
								{#if w.disabled}<span class="sf-error">{w.disabled}</span>{/if}
							</span>
						</label>
					{/each}
				</div>
				{#if error}<p class="sf-error" role="alert">{error}</p>{/if}
			</div>
			<footer class="sf-foot">
				<span class="sf-grow"></span>
				<button type="button" class="sf-btn" onclick={ondone}>{t('common.cancel')}</button>
				<button type="button" class="sf-btn primary" disabled={!wayOk} onclick={proceed}
				>{way === 'workbook' ? (info.choice === 'table' ? t('remote.dataShare.go.republish') : t('remote.dataShare.go.publish')) : way === 'shared' ? t('remote.dataShare.go.shared') : t('remote.login.continue')}</button
				>
			</footer>
		{:else}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="sf-body" bind:this={body} onkeydown={onKey}>
				{#if !info}
					<p class="sf-hint">{error ? '' : t('remote.dataShare.reading')}</p>
				{:else if info.reason}
					<p>{info.reason}</p>
				{:else}
					<div class="sf-fields">
						<label class="sf-label" for="ds-name">{t('remote.rules.name')}</label>
						<div class="sf-field">
							<input id="ds-name" class="sf-input" bind:value={name} disabled={busy} aria-invalid={!!problems.name} autocomplete="off" />
							{#if problems.name}<p class="sf-error">{problems.name}</p>{/if}
						</div>

						<span class="sf-label">{t('remote.dataShare.folder')}</span>
						<div class="sf-field">
							<FolderPicker id="ds-folder" catalog={info.catalog} bind:value={folder} disabled={busy} oncreate={createFolder} />
						</div>

						<span class="sf-label">{t('remote.columnPicker.aria')}</span>
						<div class="sf-field sf-cols">
							<ColumnPicker label={t('remote.dataShare.columns')} columns={colTypes} selected={shown} onchange={(s) => (shown = s)} disabled={busy} error={problems.columns ?? ''} />
						</div>

						<span class="sf-label">{t('remote.dataShare.audience')}</span>
						<div class="sf-field">
							<AudienceEditor {current} bind:removed bind:adds bind:inherit teams={info.teams ?? []} columns={info.fields ?? []} folder={folderPath(info.catalog, folder)} {table} {values} disabled={busy} />
						</div>

						{#if ds?.used_by?.length}
							<span class="sf-label">{t('remote.dataShare.usedBy')}</span>
							<ul class="sf-used">
								{#each ds.used_by as u (u.workbook_id)}
									<li>
										{u.title}
										{#if u.last_read}<span class="sf-hint"> · {t('remote.dataShare.readBy', { email: u.last_read.email, date: fmtDateTime(new Date(u.last_read.at), { dateStyle: 'short', timeStyle: 'short' }) })}</span>{/if}
									</li>
								{/each}
							</ul>
						{/if}
					</div>

					<details class="sf-adv" bind:open={adv}>
						<summary
							>{t('remote.dataShare.advanced')} <span class="sf-sum"
								>{ds ? `donnees.${ds.sql_name}` : sqlName.trim() ? `donnees.${sqlName.trim()}` : t('remote.dataShare.sqlProposed')}{description.trim() ? ` · ${t('remote.dataShare.withDescription')}` : ''}</span
							></summary
						>
						<div class="sf-fields">
							<label class="sf-label" for="ds-desc">{t('remote.dataShare.description.label')} <span class="sf-opt">{t('remote.rules.optional')}</span></label>
							<input id="ds-desc" class="sf-input" bind:value={description} disabled={busy} placeholder={t('remote.dataShare.description.placeholder')} />
							<label class="sf-label" for="ds-sql">{t('remote.dataShare.sqlName.label')}</label>
							<div class="sf-field">
								{#if ds}
									<div class="sf-static"><code class="sf-code">donnees.{ds.sql_name}</code><span class="sf-hint">{t('remote.dataShare.sqlName.fixed')}</span></div>
								{:else}
									<div class="sf-prefixed">
										<span>donnees.</span>
										<input id="ds-sql" class="sf-input" bind:value={sqlName} disabled={busy} placeholder={t('remote.dataShare.sqlName.placeholder')} spellcheck="false" aria-invalid={!!problems.sql} />
									</div>
									<p class={problems.sql ? 'sf-error' : 'sf-hint'}>{problems.sql ?? t('remote.dataShare.sqlName.help')}</p>
								{/if}
							</div>
						</div>
					</details>
				{/if}
				{#if error}<p class="sf-error" role="alert" style="margin-top: 10px">{error}</p>{/if}
			</div>
			<footer class="sf-foot">
				{#if ds}<button type="button" class="sf-btn danger" onclick={withdraw} disabled={busy} title={t('remote.dataShare.withdraw.tooltip')}>{t('remote.dataShare.withdraw.label')}</button>{/if}
				{#if tried && firstProblem && info && !info.reason}
					<p class="sf-error" role="alert">{firstProblem}</p>
				{:else}
					<span class="sf-grow"></span>
				{/if}
				<button type="button" class="sf-btn" onclick={ondone} disabled={busy}>{info?.reason ? t('common.close') : t('common.cancel')}</button>
				{#if info && !info.reason}
					<button type="button" class="sf-btn primary" onclick={attempt} disabled={busy}>{busy ? t('remote.formDialog.saving') : ds ? t('remote.formDialog.save') : t('remote.dataShare.share')}</button>
				{/if}
			</footer>
		{/if}
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		background: var(--backdrop);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: var(--z-dialog);
	}
	.dialog {
		background: #fff;
		border-radius: 6px;
		box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
		width: min(680px, calc(100vw - 32px));
		max-height: min(88vh, calc(100vh - 32px));
		min-height: 0;
		display: flex;
		flex-direction: column;
	}
	header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 10px 10px 18px;
		font-weight: 600;
		border-bottom: 1px solid #e5e7eb;
		flex: none;
		--icon-accent: #217346;
	}
	.title {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	header :global(.help-link) {
		margin: 0;
	}
	.close {
		border: none;
		background: none;
		width: 26px;
		height: 26px;
		border-radius: 4px;
		color: #6b7280;
		cursor: pointer;
		font: inherit;
	}
	.close:hover:not(:disabled) {
		background: #f0f1f3;
		color: #1f2328;
	}
	.ctitle {
		margin: 0 0 4px;
		font-size: 15px;
		font-weight: 600;
	}
	.lead {
		margin: 0 0 12px;
		color: #4b5563;
	}
	.whytext {
		margin: -6px 0 12px;
	}
	.why {
		margin-left: 6px;
		padding: 0;
		border: none;
		border-bottom: 1px dotted #217346;
		background: none;
		font: inherit;
		font-size: 12px;
		color: #217346;
		cursor: pointer;
		white-space: nowrap;
	}
	.why:focus-visible {
		outline: 2px solid rgba(33, 115, 70, 0.45);
		outline-offset: 1px;
	}
	.ways {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.way {
		position: relative;
		display: flex;
		align-items: flex-start;
		gap: 12px;
		padding: 12px 14px;
		border: 1px solid #d6d9dd;
		border-radius: 6px;
		cursor: pointer;
	}
	.way:hover:not(.off) {
		border-color: #8fb79d;
	}
	.way.on {
		border-color: #217346;
		background: #f3f8f5;
		box-shadow: inset 0 0 0 1px #217346;
	}
	.way:has(input:focus-visible) {
		outline: 2px solid rgba(33, 115, 70, 0.45);
		outline-offset: 2px;
	}
	.way.off {
		cursor: default;
		opacity: 0.7;
	}
	.way input {
		position: absolute;
		opacity: 0;
		pointer-events: none;
	}
	.way-icon {
		color: #217346;
		--icon-accent: #217346;
	}
	.way-text {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
</style>
