<script lang="ts">
	// Onglet Script › Accès réseau : les hôtes que les scripts et les pages
	// HTML du classeur joignent sans redemander (module http). Un hôte
	// retiré redemandera l'autorisation à la prochaine requête.
	import { modal } from '#lib/ui/focus.js';
	import { onMount } from 'svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { hostOf } from '#lib/page/hosts.js';

	export interface HostAccess {
		list: () => Promise<string[]>;
		allow: (host: string) => Promise<string[]>;
		forget: (host: string) => Promise<string[]>;
	}

	interface Props extends HostAccess {
		onclose: () => void;
	}

	let { list, allow, forget, onclose }: Props = $props();

	let hosts = $state<string[]>([]);
	let draft = $state('');
	let error = $state('');

	const run = async (fn: () => Promise<string[]>) => {
		try {
			hosts = (await fn()) ?? [];
			error = '';
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	};

	function add() {
		const h = hostOf(draft);
		if (!h) {
			error = 'Nom d’hôte attendu : api.exemple.fr, ou une adresse https://…';
			return;
		}
		draft = '';
		run(() => allow(h));
	}

	onMount(() => {
		run(list);
	});
</script>

<div class="modal-back" role="presentation" onclick={(e) => e.target === e.currentTarget && onclose()}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label="Accès réseau"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') onclose();
		}}
	>
		<h2>Accès réseau du classeur</h2>
		<p class="muted">
			Avant la première requête HTTP vers un hôte (http.get, http.post… dans un script ou une page HTML), Lattis vous demande l’autorisation. Les hôtes autorisés avec « Mémoriser pour ce classeur » sont listés ici : ils sont mémorisés sur ce poste, pas dans le fichier (un classeur reçu redemande toujours). Le serveur MCP n’atteint que ces hôtes.
		</p>
		{#if hosts.length}
			<ul>
				{#each hosts as h (h)}
					<li>
						<span><Icon name="web" size={14} />{h}</span>
						<button onclick={() => run(() => forget(h))} title="Retirer : la prochaine requête redemandera">Retirer</button>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="muted">Aucun hôte mémorisé.</p>
		{/if}
		<form
			class="add"
			onsubmit={(e) => {
				e.preventDefault();
				add();
			}}
		>
			<input bind:value={draft} placeholder="api.exemple.fr" spellcheck="false" aria-label="Hôte à autoriser" />
			<button type="submit" disabled={!draft.trim()}>Autoriser</button>
		</form>
		{#if error}<p class="error">{error}</p>{/if}
		<div class="actions">
			<button class="primary" onclick={onclose}>Fermer</button>
		</div>
	</div>
</div>

<style>
	.modal-back {
		position: fixed;
		inset: 0;
		z-index: var(--z-dialog);
		background: var(--backdrop);
		display: flex;
		align-items: center;
		justify-content: center;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		padding: 16px 18px;
		width: 460px;
		max-width: calc(100vw - 32px);
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	h2 {
		font-size: 15px;
		font-weight: 600;
		margin: 0;
	}
	p {
		margin: 0;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.error {
		color: #b42318;
	}
	ul {
		list-style: none;
		margin: 0;
		padding: 0;
		max-height: 240px;
		overflow: auto;
		border: 1px solid #e3e5e8;
		border-radius: 3px;
	}
	li {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 4px 8px;
		border-bottom: 1px solid #f0f1f3;
	}
	li:last-child {
		border-bottom: none;
	}
	li span {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		--icon-accent: #217346;
	}
	.add {
		display: flex;
		gap: 8px;
	}
	.add input {
		flex: 1;
		font: inherit;
		padding: 4px 8px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	button {
		font: inherit;
		padding: 4px 12px;
		border: 1px solid #d4d7db;
		background: #fff;
		border-radius: 2px;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
	}
	.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
</style>
