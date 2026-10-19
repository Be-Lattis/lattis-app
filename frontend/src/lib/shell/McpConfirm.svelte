<script lang="ts">
	// Accord avant une écriture d'un client MCP (Claude Desktop, Claude
	// Code…), quand « Demander avant de modifier » est coché (réglage
	// mcp.confirmWrites). Le serveur attend la réponse deux minutes, puis
	// refuse ; plusieurs demandes passent l'une après l'autre. Monté une fois
	// pour toute l'appli (+page.svelte), sans prop.
	import { modal } from '#lib/ui/focus.js';
	import { onMount } from 'svelte';
	import { Events } from '@wailsio/runtime';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import { dropConfirm, pushConfirm, type McpConfirmRequest } from './mcp';

	let queue = $state<McpConfirmRequest[]>([]);
	const cur = $derived(queue[0]);

	onMount(() => {
		const offAsk = Events.On('mcp:confirm', (e: { data: McpConfirmRequest }) => {
			queue = pushConfirm(queue, e.data);
		});
		const offDone = Events.On('mcp:confirm-done', (e: { data: { id: number } }) => {
			queue = dropConfirm(queue, e.data.id);
		});
		return () => {
			offAsk();
			offDone();
		};
	});

	function answer(allow: boolean) {
		const r = cur;
		if (!r) return;
		queue = dropConfirm(queue, r.id);
		DocumentService.MCPAnswer(r.id, allow).catch(() => {
			// Demande déjà close côté serveur : rien à faire.
		});
	}
</script>

{#if cur}
	{#key cur.id}
		<div class="back">
			<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
			<div
				class="dialog"
				role="alertdialog"
				aria-modal="true"
				aria-labelledby="mcp-confirm-title"
				aria-describedby="mcp-confirm-label"
				tabindex="-1"
				use:modal={() => answer(false)}
				onkeydown={(e) => e.stopPropagation()}
			>
				<header id="mcp-confirm-title">{cur.client || 'Claude'} veut modifier le classeur</header>
				<div class="body">
					<p id="mcp-confirm-label" class="what">{cur.label}</p>
					<p class="muted">
						Outil <code>{cur.tool}</code>. Sans réponse dans deux minutes, la demande est refusée.
						{#if queue.length > 1}<br />{queue.length - 1}&nbsp;autre{queue.length > 2 ? 's' : ''} demande{queue.length > 2 ? 's' : ''} en attente.{/if}
					</p>
				</div>
				<footer>
					<button class="primary" onclick={() => answer(true)}>Autoriser</button>
					<button onclick={() => answer(false)}>Refuser</button>
				</footer>
			</div>
		</div>
	{/key}
{/if}

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: var(--z-confirm);
		background: var(--backdrop);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.dialog {
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 420px;
		max-width: calc(100vw - 32px);
		font-size: 12px;
		outline: none;
	}
	header {
		padding: 10px 14px 4px;
		font-size: 14px;
		font-weight: 600;
	}
	.body {
		padding: 0 14px;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	p {
		margin: 0;
	}
	.what {
		font-size: 13px;
		padding: 6px 8px;
		background: #f3f4f6;
		border-radius: 3px;
		overflow-wrap: anywhere;
	}
	.muted {
		color: #6b7280;
	}
	code {
		font-size: 11px;
	}
	footer {
		display: flex;
		justify-content: flex-end;
		gap: 6px;
		padding: 10px 14px;
	}
	button {
		font: inherit;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		padding: 3px 10px;
		cursor: pointer;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
</style>
