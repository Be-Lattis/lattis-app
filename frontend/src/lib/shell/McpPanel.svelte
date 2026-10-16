<script lang="ts">
	// Fichier › Serveur MCP : un assistant (Claude Desktop, Claude Code…)
	// pilote le classeur ouvert avec les mêmes commandes que les scripts Lua.
	// Le serveur n'écoute que sur cet ordinateur et demande un jeton.
	import { onMount } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { MCPStatus } from '#bindings/internal/bridge/models.js';
	import { mcpConfigs } from './mcp';

	interface Props {
		onclose: () => void;
	}
	let { onclose }: Props = $props();

	let st = $state<MCPStatus | null>(null);
	let error = $state('');
	let copied = $state('');
	let showToken = $state(false);

	const err = (e: unknown) => String(e instanceof Error ? e.message : e);
	onMount(() => {
		DocumentService.MCPState().then((s) => (st = s), (e) => (error = err(e)));
	});

	async function toggle() {
		if (!st) return;
		try {
			st = await DocumentService.SetMCPEnabled(!st.enabled);
			error = st.error;
		} catch (e) {
			error = err(e);
		}
	}

	const configs = $derived(st ? mcpConfigs(st) : []);

	async function copy(label: string, text: string) {
		try {
			await navigator.clipboard.writeText(text);
			copied = label;
		} catch (e) {
			error = err(e);
		}
	}
</script>

<div class="back">
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label="Serveur MCP"
		tabindex="-1"
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') onclose();
		}}
	>
		<header>Serveur MCP</header>
		<p>
			Un assistant compatible MCP (Claude Desktop, Claude Code…) peut lire et modifier le classeur ouvert : feuilles, tableaux, cellules, sélection, SQL, scripts, boutons. Chacune de ses
			modifications s’annule avec Ctrl+Z. Le serveur n’écoute que sur cet ordinateur et exige un jeton.
		</p>
		{#if st}
			<label class="switch">
				<input type="checkbox" checked={st.enabled} onchange={toggle} />
				<span>Activer le serveur MCP {st.enabled ? (st.running ? '(en marche)' : '(arrêté)') : ''}</span>
			</label>
			{#if st.enabled}
				<dl>
					<dt>Adresse</dt>
					<dd><code>{st.url}</code></dd>
					<dt>Jeton</dt>
					<dd>
						<code>{showToken ? st.token : '•'.repeat(16)}</code>
						<button class="link" onclick={() => (showToken = !showToken)}>{showToken ? 'Masquer' : 'Afficher'}</button>
					</dd>
				</dl>
				{#each configs as c (c.label)}
					<div class="config">
						<div class="config-head">
							<strong>{c.label}</strong>
							<button onclick={() => copy(c.label, c.text)}>{copied === c.label ? 'Copié' : 'Copier'}</button>
						</div>
						<p class="hint">{c.hint}</p>
						<pre>{c.text}</pre>
					</div>
				{/each}
			{/if}
		{/if}
		{#if error}<p class="error">{error}</p>{/if}
		<div class="actions"><button class="primary" onclick={onclose}>Fermer</button></div>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: 50;
		background: rgba(0, 0, 0, 0.25);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 8vh;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 620px;
		max-width: calc(100vw - 32px);
		max-height: 84vh;
		overflow: auto;
		padding-bottom: 14px;
		outline: none;
	}
	header {
		padding: 10px 16px;
		font-weight: 600;
		border-bottom: 1px solid #e3e5e8;
	}
	p {
		margin: 10px 16px;
		color: #374151;
	}
	.switch {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 6px 16px 10px;
		font-weight: 600;
	}
	dl {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 4px 12px;
		margin: 0 16px 8px;
	}
	dt {
		color: #6b7280;
	}
	dd {
		margin: 0;
		display: flex;
		gap: 8px;
		align-items: center;
		user-select: text;
	}
	.config {
		margin: 8px 16px;
		border: 1px solid #e3e5e8;
		padding: 8px 10px;
	}
	.config-head {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}
	.hint {
		margin: 4px 0;
		font-size: 12px;
	}
	pre {
		margin: 4px 0 0;
		padding: 6px 8px;
		background: #f6f8fa;
		font-size: 12px;
		white-space: pre-wrap;
		word-break: break-all;
	}
	.error {
		color: #c0392b;
	}
	button {
		font: inherit;
		padding: 3px 12px;
		border: 1px solid #c4c8cc;
		background: #fff;
		cursor: pointer;
	}
	button.link {
		border: none;
		padding: 0;
		color: #217346;
		text-decoration: underline;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		padding: 8px 16px 0;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding: 5px 15px;
	}
</style>
