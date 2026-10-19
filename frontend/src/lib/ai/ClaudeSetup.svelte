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

	const err = (e: unknown) => String(e instanceof Error ? e.message : e);

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
		Avec un abonnement Claude (Pro, Max, Team), pas besoin de clé d’API : vous discutez dans Claude Desktop ou Claude Code, qui agit sur le classeur
		ouvert, et Lattis montre ce qui est fait.
	</p>
	<p class="hint sharing">
		<Icon name="lock" size={12} />Le réglage « Données transmises à l’assistant » (Fichier › Paramètres › Assistant IA) vaut aussi pour Claude :
		<strong>{sharingLabel(sharing)}</strong>, {sharingSummary(sharing)} Après un changement, redémarrez Claude pour qu’il voie la nouvelle liste d’outils.
	</p>
	{#if c}
		<div class="cards">
			<section class="card">
				<header>
					<strong>Claude Desktop</strong>
					{#if connected === 'Claude Desktop'}<span class="badge ok">Connecté</span>
					{:else if c.desktopConfigured}<span class="badge ok">Configuré</span>
					{:else if c.desktopInstalled}<span class="badge">Installé</span>
					{:else}<span class="badge off">Non installé</span>{/if}
				</header>
				<p class="hint">L’application de bureau de Claude. Lattis y apparaît dans les outils de la conversation.</p>
				{#if justDone.desktop}
					<p class="done">Configuré ✓ — redémarrez Claude Desktop (quittez-le complètement, puis rouvrez-le).</p>
				{:else}
					<div class="row">
						<button class="primary" disabled={busy !== ''} onclick={() => configure('desktop')}>
							<Icon name="plug" size={16} />{busy === 'desktop' ? 'Configuration…' : c.desktopConfigured ? 'Configurer à nouveau' : 'Configurer'}
						</button>
						{#if !c.desktopInstalled}<button class="link" onclick={() => openOutside(DESKTOP_URL)}>Télécharger Claude Desktop</button>{/if}
					</div>
				{/if}
				{#if desktopJson}
					<details>
						<summary>Configurer à la main</summary>
						<p class="hint">À fusionner dans <code>{c.desktopConfig}</code> :</p>
						<pre>{desktopJson}</pre>
						<button class="ico" onclick={() => copy('desktop', desktopJson)}><Icon name="copy" size={14} />{copied === 'desktop' ? 'Copié' : 'Copier'}</button>
					</details>
				{/if}
			</section>

			<section class="card">
				<header>
					<strong>Claude Code</strong>
					{#if connected === 'Claude Code'}<span class="badge ok">Connecté</span>
					{:else if c.codeConfigured}<span class="badge ok">Configuré</span>
					{:else if c.codeFound}<span class="badge">Installé</span>
					{:else}<span class="badge off">Introuvable</span>{/if}
				</header>
				<p class="hint">L’assistant de Claude dans le terminal (ou dans son application de bureau). Lattis est ajouté pour tous vos projets.</p>
				{#if justDone.code}
					<p class="done">Configuré ✓ — ouvrez une nouvelle session de Claude Code : <code>/mcp</code> y montre Lattis.</p>
				{:else}
					<div class="row">
						<button class="primary" disabled={busy !== '' || !c.codeFound} title={c.codeFound ? '' : 'Commande claude introuvable sur ce poste'} onclick={() => configure('code')}>
							<Icon name="plug" size={16} />{busy === 'code' ? 'Configuration…' : c.codeConfigured ? 'Configurer à nouveau' : 'Configurer'}
						</button>
						{#if !c.codeFound}<button class="link" onclick={() => openOutside(CODE_URL)}>Installer Claude Code</button>{/if}
					</div>
				{/if}
				<details>
					<summary>Commande à lancer soi-même</summary>
					<pre>{c.codeCommand}</pre>
					<button class="ico" onclick={() => copy('code', c?.codeCommand ?? '')}><Icon name="copy" size={14} />{copied === 'code' ? 'Copié' : 'Copier'}</button>
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
