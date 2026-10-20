<script lang="ts">
	// Voie « abonnement Claude » : Claude Desktop ou Claude Code pilote
	// Lattis par son serveur MCP. Deux cartes : état (installé, configuré),
	// configuration en un clic (Lattis active son serveur et écrit lui-même
	// la configuration du client), lien de téléchargement, et la commande ou
	// le JSON à copier si l'on préfère faire soi-même. Rappelle que le
	// partage des données de l'assistant vaut aussi pour Claude. Aucune prop
	// obligatoire ; onchange prévient l'hôte quand le
	// serveur vient d'être activé.
	import { onMount } from 'svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { MCPClients, MCPStatus } from '#bindings/internal/bridge/models.js';
	import { mcpConfigs } from '#lib/shell/mcp.js';
	import { external } from './external.svelte.js';
	import { findSetting } from '#lib/shell/settings.js';
	import { userSettings } from '#lib/shell/settings.svelte.js';
	import { sharingLabel, sharingSummary } from './sharing';
	import { errorText, t } from '#lib/i18n/index.js';

	interface Props {
		/** Un client vient d'être configuré (le serveur MCP est alors activé). */
		onchange?: (c: MCPClients) => void;
	}
	let { onchange }: Props = $props();

	const DESKTOP_URL = 'https://claude.ai/download';
	const CODE_URL = 'https://code.claude.com/docs/fr/setup';

	let c = $state<MCPClients | null>(null);
	let mcpState = $state<MCPStatus | null>(null);
	let error = $state('');
	let busy = $state<'' | 'desktop' | 'code'>('');
	// Configuré pendant cette visite : il faut (re)démarrer le client.
	let justDone = $state<{ desktop: boolean; code: boolean }>({ desktop: false, code: false });
	let copied = $state('');

	const err = errorText;

	async function load() {
		try {
			c = await DocumentService.MCPClients();
			external.clients = c;
			mcpState = await DocumentService.MCPState();
		} catch (e) {
			error = err(e);
		}
	}
	onMount(load);

	async function configure(which: 'desktop' | 'code') {
		busy = which;
		error = '';
		try {
			c = which === 'desktop' ? await DocumentService.MCPConfigureDesktop() : await DocumentService.MCPConfigureCode();
			external.clients = c;
			justDone[which] = true;
			mcpState = await DocumentService.MCPState();
			onchange?.(c);
		} catch (e) {
			error = err(e);
		} finally {
			busy = '';
		}
	}

	function openOutside(u: string) {
		import('@wailsio/runtime').then(
			(rt) => rt.Browser.OpenURL(u),
			() => window.open(u, '_blank')
		);
	}

	async function copy(label: string, text: string) {
		try {
			await navigator.clipboard.writeText(text);
			copied = label;
		} catch (e) {
			error = err(e);
		}
	}

	const desktopJson = $derived(mcpState ? mcpConfigs(mcpState)[0].text : '');
	const connected = $derived(external.connected || c?.connected || '');
	// Partage des données (ai.dataSharing) : le serveur MCP l'applique aussi.
	const sharing = $derived(findSetting(userSettings.list, 'ai.dataSharing')?.value ?? 'schema-only');
</script>

<div class="setup">
	<p class="intro">
		{t('ai.setup.intro')}
	</p>
	<p class="hint sharing">
		<Icon name="lock" size={12} />{t('ai.setup.sharing.before')}
		<strong>{sharingLabel(sharing)}</strong>, {sharingSummary(sharing)} {t('ai.setup.sharing.after')}
	</p>
	{#if c}
		<div class="cards">
			<section class="card">
				<header>
					<strong>Claude Desktop</strong>
					{#if connected === 'Claude Desktop'}<span class="badge ok">{t('ai.setup.badge.connected')}</span>
					{:else if c.desktopConfigured}<span class="badge ok">{t('ai.setup.badge.configured')}</span>
					{:else if c.desktopInstalled}<span class="badge">{t('ai.setup.badge.installed')}</span>
					{:else}<span class="badge off">{t('ai.setup.badge.notInstalled')}</span>{/if}
				</header>
				<p class="hint">{t('ai.setup.desktop.help')}</p>
				{#if justDone.desktop}
					<p class="done">{t('ai.setup.desktop.done')}</p>
				{:else}
					<div class="row">
						<button class="primary" disabled={busy !== ''} onclick={() => configure('desktop')}>
							<Icon name="plug" size={16} />{busy === 'desktop' ? t('ai.setup.configuring') : c.desktopConfigured ? t('ai.setup.reconfigure') : t('ai.setup.configure')}
						</button>
						{#if !c.desktopInstalled}<button class="link" onclick={() => openOutside(DESKTOP_URL)}>{t('ai.setup.desktop.download')}</button>{/if}
					</div>
				{/if}
				{#if desktopJson}
					<details>
						<summary>{t('ai.setup.desktop.manual')}</summary>
						<p class="hint">{t('ai.setup.desktop.mergeInto')} <code>{c.desktopConfig}</code> :</p>
						<pre>{desktopJson}</pre>
						<button class="ico" onclick={() => copy('desktop', desktopJson)}><Icon name="copy" size={14} />{copied === 'desktop' ? t('remote.rightsDialog.link.copied') : t('common.copy')}</button>
					</details>
				{/if}
			</section>

			<section class="card">
				<header>
					<strong>Claude Code</strong>
					{#if connected === 'Claude Code'}<span class="badge ok">{t('ai.setup.badge.connected')}</span>
					{:else if c.codeConfigured}<span class="badge ok">{t('ai.setup.badge.configured')}</span>
					{:else if c.codeFound}<span class="badge">{t('ai.setup.badge.installed')}</span>
					{:else}<span class="badge off">{t('ai.setup.badge.notFound')}</span>{/if}
				</header>
				<p class="hint">{t('ai.setup.code.help')}</p>
				{#if justDone.code}
					<p class="done">{t('ai.setup.code.done.before')} <code>/mcp</code> {t('ai.setup.code.done.after')}</p>
				{:else}
					<div class="row">
						<button class="primary" disabled={busy !== '' || !c.codeFound} title={c.codeFound ? '' : t('ai.setup.code.notFound')} onclick={() => configure('code')}>
							<Icon name="plug" size={16} />{busy === 'code' ? t('ai.setup.configuring') : c.codeConfigured ? t('ai.setup.reconfigure') : t('ai.setup.configure')}
						</button>
						{#if !c.codeFound}<button class="link" onclick={() => openOutside(CODE_URL)}>{t('ai.setup.code.install')}</button>{/if}
					</div>
				{/if}
				<details>
					<summary>{t('ai.setup.code.manual')}</summary>
					<pre>{c.codeCommand}</pre>
					<button class="ico" onclick={() => copy('code', c?.codeCommand ?? '')}><Icon name="copy" size={14} />{copied === 'code' ? t('remote.rightsDialog.link.copied') : t('common.copy')}</button>
				</details>
			</section>
		</div>
	{/if}
	{#if error}<p class="error">{error}</p>{/if}
</div>

<style>
	.setup {
		--icon-accent: #217346;
	}
	p {
		margin: 0;
	}
	.intro {
		margin-bottom: 10px;
		color: #374151;
	}
	.cards {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
		gap: 10px;
	}
	.card {
		border: 1px solid #e3e5e8;
		border-radius: var(--float-radius, 8px);
		padding: 8px 10px;
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
	}
	.card header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}
	.badge {
		font-size: 11px;
		padding: 0 6px;
		border-radius: 8px;
		background: #f3f4f6;
		color: #4b5563;
	}
	.badge.ok {
		background: #e3efe8;
		color: #185c37;
	}
	.badge.off {
		color: #6b7280;
	}
	.hint {
		font-size: 12px;
		color: #6b7280;
	}
	.sharing {
		margin: 0 0 10px;
	}
	.sharing :global(svg) {
		vertical-align: -2px;
		margin-right: 4px;
	}
	.done {
		color: #185c37;
		font-weight: 600;
	}
	.row {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-items: center;
	}
	details summary {
		cursor: pointer;
		font-size: 12px;
		color: #4b5563;
	}
	details[open] {
		display: flex;
		flex-direction: column;
		gap: 4px;
		align-items: flex-start;
	}
	pre {
		margin: 0;
		padding: 6px 8px;
		background: #f6f8fa;
		font-size: 12px;
		white-space: pre-wrap;
		word-break: break-all;
		align-self: stretch;
		user-select: text;
	}
	code {
		font-size: 11px;
		word-break: break-all;
	}
	button {
		font: inherit;
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 3px 10px;
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
		--icon-accent: #fff;
	}
	button.link {
		border: none;
		background: none;
		padding: 0;
		color: #185c37;
		text-decoration: underline;
	}
	.error {
		margin-top: 8px;
		color: #b91c1c;
	}
</style>
