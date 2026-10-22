<script lang="ts">
	// Fichier › Gérer l’accès… (classeur publié) : droits du classeur
	// (spécification de l'application collaborative, §8.2). Pour les
	// propriétaires : Membres (personnes et équipes avec leurs rôles en
	// pastilles, invitations), Rôles et droits (matrice de ce que chaque rôle
	// permet, accès par classeur → feuille → tableau, filtres de lignes) et
	// Règles avancées (restrictions du serveur, en phrases) ; pour
	// propriétaires et concepteurs : Voir en tant que… et les écarts
	// détectés par le serveur (secondaires). Tout passe par
	// DocumentService.Share* (bridge/rights.go). Mise en page :
	// access.css et shareForm.css (docs/audits/audit-ux-gerer-acces.md).
	import { isTextField, modal, tabbables } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import Icon from '#lib/icons/Icon.svelte';
	import HelpLink from '#lib/help/HelpLink.svelte';
	import { onMount, tick, untrack } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { RightsView } from '#bindings/internal/bridge/models.js';
	import AccessMembers from './AccessMembers.svelte';
	import AccessPreview from './AccessPreview.svelte';
	import AccessRoles from './AccessRoles.svelte';
	import AccessRules from './AccessRules.svelte';
	import { hostOf } from './lattis';
	import { members } from './rights';
	import './shareForm.css';
	import './access.css';
	import { errorText, t, tAny } from '#lib/i18n/index.js';

	interface Props {
		title: string;
		server: string;
		/** Adresse à partager du classeur (<serveur>/w/<id>) ; vide : non affichée. */
		link?: string;
		onclose: () => void;
		/** Classeur partagé en direct : ajouter un tableau qui n'est que sur ce poste (confirmation et envoi par la page). */
		onaddshared?: (table: string) => Promise<void>;
		/** « Retirer du serveur… » (propriétaire) : la page ferme la boîte, confirme et retire. */
		onremove?: () => void;
	}
	let { title, server, link = '', onclose, onaddshared, onremove }: Props = $props();

	/** Page d'aide des droits. */
	const HELP = 'fichiers#gerer-l-acces-a-un-classeur-partage';

	type Tab = 'members' | 'roles' | 'rules' | 'preview' | 'drift';
	let view = $state<RightsView | null>(null);
	let tab = $state<Tab>('members');
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');
	let linkCopied = $state(false);
	let tabsEl = $state<HTMLElement>();
	let panel = $state<HTMLElement>();
	let dialogEl = $state<HTMLElement>();
	// Nouvel onglet : en haut, sans le message du précédent.
	$effect(() => {
		void tab;
		untrack(() => {
			if (panel) panel.scrollTop = 0;
			notice = '';
		});
	});
	const err = errorText;
	const manage = $derived(!!view?.canManage);
	const drift = $derived(view?.drift ?? []);

	onMount(load);

	async function load() {
		busy = true;
		error = '';
		try {
			view = await DocumentService.ShareRights(true);
			if (!view.canManage && (tab === 'members' || tab === 'roles' || tab === 'rules')) tab = 'preview';
			if (view.error) error = view.error;
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
		if (first) {
			// Première lecture : le focus va au premier champ (adresse à inviter).
			first = false;
			before = null;
			await tick();
			if (panel) tabbables(panel).find(isTextField)?.focus({ preventScroll: true });
		}
	}
	let first = true;

	/** Changement envoyé au serveur : la vue revient à jour ; false : refusé (message dans le pied). */
	async function run(fn: () => Promise<RightsView>, done?: string): Promise<boolean> {
		busy = true;
		error = '';
		notice = '';
		try {
			view = await fn();
			if (done) notice = done;
			return true;
		} catch (e) {
			error = err(e);
			return false;
		} finally {
			busy = false;
		}
	}

	// Les champs sont grisés pendant un envoi (busy) : le focus qu'ils
	// perdent revient ensuite où il était, ou sur la boîte (Échap, clavier).
	let before: HTMLElement | null = null;
	$effect.pre(() => {
		if (busy) before = document.activeElement as HTMLElement | null;
		else if (before) {
			const prev = before;
			before = null;
			refocus(prev);
		}
	});

	/** Les champs sont grisés pendant l'envoi : le focus revient où il était (ou sur la boîte), pour Échap et le clavier. */
	async function refocus(prev: HTMLElement | null) {
		await tick();
		if (!dialogEl || dialogEl.contains(document.activeElement)) return;
		if (prev?.isConnected && dialogEl.contains(prev) && !(prev as HTMLButtonElement).disabled) prev.focus({ preventScroll: true });
		else dialogEl.focus({ preventScroll: true });
	}

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(link);
			linkCopied = true;
			setTimeout(() => (linkCopied = false), 1500);
		} catch (e) {
			error = err(e);
		}
	}

	// --- Voir en tant que… depuis une ligne ---------------------------------------------------
	let request = $state<{ user: string; table: number; n: number } | null>(null);
	function previewUser(user: string) {
		request = { user, table: 0, n: (request?.n ?? 0) + 1 };
		tab = 'preview';
	}
	/** Premier membre (en personne ou par une équipe) qui a ce rôle. */
	function previewRole(roleId: string) {
		const p = members(view?.acl, view?.members).find((m) => m.roles.includes(roleId));
		if (p) previewUser(p.userId);
	}

	// --- Onglets ----------------------------------------------------------------------------
	const TABS: { id: Tab; manage: boolean }[] = [
		{ id: 'members', manage: true },
		{ id: 'roles', manage: true },
		{ id: 'rules', manage: true },
		{ id: 'preview', manage: false }
	];
	const tabs = $derived([...TABS.filter((x) => manage || !x.manage), { id: 'drift' as Tab, manage: false }].map((x) => ({ ...x, label: tAny(`remote.rightsDialog.tab.${x.id}`) })));

	async function tabKey(e: KeyboardEvent) {
		const i = tabs.findIndex((x) => x.id === tab);
		let j = -1;
		if (e.key === 'ArrowRight') j = (i + 1) % tabs.length;
		else if (e.key === 'ArrowLeft') j = (i - 1 + tabs.length) % tabs.length;
		else if (e.key === 'Home') j = 0;
		else if (e.key === 'End') j = tabs.length - 1;
		if (j < 0) return;
		e.preventDefault();
		tab = tabs[j].id;
		await tick();
		tabsEl?.querySelector<HTMLElement>('[aria-selected="true"]')?.focus();
	}
</script>

<div class="back" role="presentation" use:backdrop={() => !busy && onclose()}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog ac-dialog"
		bind:this={dialogEl}
		role="dialog"
		aria-modal="true"
		aria-labelledby="ra-title"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape' && !busy && !e.defaultPrevented) onclose();
		}}
	>
		<header>
			<Icon name="user" size={18} />
			<span id="ra-title" class="title">{t('remote.rightsDialog.title', { title })}</span>
			<span class="host">{hostOf(server)}{view && !view.collaborative ? ` · ${t('remote.rightsDialog.frozen')}` : ''}</span>
			<button type="button" class="hbtn" disabled={busy} onclick={load} title={t('remote.rightsDialog.reload.tooltip')} aria-label={t('remote.rightsDialog.reload.aria')}><Icon name="refresh" size={14} /></button>
			<HelpLink target={HELP} label={t('remote.rightsDialog.help')} />
			<button type="button" class="hbtn" aria-label={t('common.close')} title={t('remote.closeEsc')} disabled={busy} onclick={onclose}>✕</button>
		</header>
		{#if link}
			<div class="share-link">
				<Icon name="link" size={14} />
				<input class="sf-input" readonly value={link} aria-label={t('remote.rightsDialog.link.aria')} onfocus={(e) => e.currentTarget.select()} />
				<button type="button" class="sf-btn" onclick={copyLink} title={t('remote.rightsDialog.link.tooltip')}>{linkCopied ? t('remote.rightsDialog.link.copied') : t('remote.formQR.copy')}</button>
			</div>
		{/if}
		<!-- svelte-ignore a11y_interactive_supports_focus -->
		<div class="tabs" role="tablist" aria-label={t('remote.rightsDialog.tabs')} bind:this={tabsEl} onkeydown={tabKey}>
			{#each tabs as tb (tb.id)}
				<button
					type="button"
					role="tab"
					id="ra-tab-{tb.id}"
					aria-controls="ra-panel"
					aria-selected={tab === tb.id}
					tabindex={tab === tb.id ? 0 : -1}
					class:on={tab === tb.id}
					class:minor={tb.id === 'drift'}
					onclick={() => (tab = tb.id)}
				>
					{#if tb.id === 'preview'}<Icon name="show" size={13} />{/if}{tb.label}{#if tb.id === 'drift' && drift.length}<span class="badge" aria-label={t('remote.rightsDialog.drift.count', { n: drift.length })}>{drift.length}</span>{/if}
				</button>
			{/each}
		</div>
		<div class="sf-body" id="ra-panel" role="tabpanel" bind:this={panel} aria-labelledby="ra-tab-{tab}">
			{#if !view}
				<p class="sf-hint">{error ? '' : t('remote.rightsDialog.reading')}</p>
			{:else}
				{#if drift.length && tab !== 'drift'}
					<p class="ac-warn">
						<Icon name="warning" size={14} />
						<span>{t('remote.rightsDialog.drift.warning', { n: drift.length })}</span>
						<button type="button" class="sf-more" onclick={() => (tab = 'drift')}>{t('remote.rightsDialog.drift.show')}</button>
					</p>
				{/if}
				{#if tab === 'members'}
					<AccessMembers {view} {busy} {run} onpreview={previewUser} />
				{:else if tab === 'roles'}
					<AccessRoles {view} bind:busy {run} onpreview={previewRole} onrules={() => (tab = 'rules')} {onaddshared} reload={load} help={HELP} />
				{:else if tab === 'rules'}
					<AccessRules {view} {busy} {run} />
				{:else if tab === 'preview'}
					<AccessPreview {view} bind:busy {request} onerror={(m) => (error = m)} />
				{:else if tab === 'drift'}
					{#if !view.collaborative}
						<p class="sf-hint">{t('remote.rightsDialog.drift.frozen')}</p>
					{:else if !drift.length}
						<p>{t('remote.rightsDialog.drift.none')}</p>
					{:else}
						<p class="ac-intro">{t('remote.rightsDialog.drift.intro')}</p>
						<ul class="ac-rules">
							{#each drift as d (d.table_id)}
								<li class="ac-rule">
									<div>
										<strong>{d.name}</strong>
										{#if d.foreign_policies?.length}<span>{t('remote.rightsDialog.drift.policies')} <code class="sf-code">{d.foreign_policies.join(', ')}</code></span>{/if}
										{#if d.foreign_grants?.length}<span>{t('remote.rightsDialog.drift.grants')} <code class="sf-code">{d.foreign_grants.join(', ')}</code></span>{/if}
										{#if !d.foreign_policies?.length && !d.foreign_grants?.length}<span>{t('remote.rightsDialog.drift.changed')}</span>{/if}
									</div>
								</li>
							{/each}
						</ul>
						{#if manage || view.canDesign}
							<div class="ac-editor-actions">
								<span class="sf-hint">{t('remote.rightsDialog.drift.recompileHelp')}</span>
								<button type="button" class="sf-btn primary" disabled={busy} onclick={() => run(() => DocumentService.ShareRecompile(), t('remote.rightsDialog.drift.recompiled'))}>{t('remote.rightsDialog.drift.recompile')}</button>
							</div>
						{/if}
					{/if}
				{/if}
			{/if}
		</div>
		<footer class="sf-foot">
			{#if manage && onremove}
				<button type="button" class="ac-link danger" disabled={busy} onclick={onremove}>{t('remote.rightsDialog.remove')}</button>
			{/if}
			{#if error}
				<p class="sf-error" role="alert">{error}</p>
			{:else if notice}
				<p class="sf-grow notice" role="status">{notice}</p>
			{:else}
				<span class="sf-grow sf-hint">{busy ? t('remote.rightsDialog.sending') : view && !manage ? t('remote.rightsDialog.designerOnly') : ''}</span>
			{/if}
			<button type="button" class="sf-btn" onclick={onclose} disabled={busy}>{t('common.close')}</button>
		</footer>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: calc(var(--z-dialog) + 30);
		background: var(--backdrop);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.dialog {
		background: #fff;
		border-radius: 6px;
		box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
		width: min(940px, calc(100vw - 32px));
		height: min(720px, calc(100vh - 32px));
		display: flex;
		flex-direction: column;
		outline: none;
		font-size: 13px;
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
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.host {
		flex: 1;
		min-width: 0;
		font-weight: normal;
		color: #6b7280;
		font-size: 12px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	header :global(.help-link) {
		margin: 0;
	}
	.hbtn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border: none;
		background: none;
		width: 26px;
		height: 26px;
		border-radius: 4px;
		color: #6b7280;
		cursor: pointer;
		font: inherit;
		--icon-accent: #6b7280;
	}
	.hbtn:hover:not(:disabled) {
		background: #f0f1f3;
		color: #1f2328;
	}
	.hbtn:focus-visible {
		outline: 2px solid rgba(33, 115, 70, 0.45);
	}
	.share-link {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 18px;
		border-bottom: 1px solid #eef0f2;
		background: #fafbfb;
		flex: none;
		color: #6b7280;
		--icon-accent: #6b7280;
	}
	.share-link .sf-input {
		flex: 1;
		min-width: 0;
		height: 26px;
		color: #374151;
		font-size: 12px;
	}
	.share-link .sf-btn {
		height: 26px;
	}
	.tabs {
		display: flex;
		gap: 2px;
		padding: 0 14px;
		border-bottom: 1px solid #e5e7eb;
		flex: none;
	}
	.tabs button {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		border: none;
		border-bottom: 2px solid transparent;
		background: none;
		padding: 8px 10px 7px;
		font: inherit;
		color: #4b5563;
		cursor: pointer;
		--icon-accent: #217346;
	}
	.tabs button:hover {
		color: #1f2328;
	}
	.tabs button.on {
		color: #14532d;
		border-bottom-color: #217346;
		font-weight: 600;
	}
	.tabs button:focus-visible {
		outline: 2px solid rgba(33, 115, 70, 0.45);
		outline-offset: -2px;
	}
	.tabs button.minor {
		margin-left: auto;
		font-size: 12px;
		color: #6b7280;
	}
	.tabs button.minor.on {
		color: #14532d;
	}
	.badge {
		background: #b45309;
		color: #fff;
		border-radius: 8px;
		padding: 0 5px;
		font-size: 11px;
		font-weight: 600;
	}
	.notice {
		margin: 0;
		color: #1b5e3a;
		font-size: 12px;
	}
	footer .sf-error {
		white-space: pre-line;
	}
</style>
