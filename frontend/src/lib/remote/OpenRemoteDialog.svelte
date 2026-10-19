<script lang="ts">
	// Fichier › Ouvrir depuis un serveur : classeurs accessibles sur un
	// Serveur Lattis (lattis.my_workbooks), puis chargement complet dans un
	// cache local, consultable hors ligne (§5.1, §10).
	import { modal } from '#lib/ui/focus.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { onMount } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { OpenResult } from '#bindings/internal/bridge/models.js';
	import type { Listing } from '#bindings/internal/remote/workbook/models.js';
	import { connected } from './lattis';
	import ServerPicker from './ServerPicker.svelte';
	import { lattis, loadLattis, requestLogin } from './lattis.svelte.js';
	import { modeLabel, rolesLabel } from './share';

	interface Props {
		/** Ouvert (résultat) ou abandon (null). */
		ondone: (r: OpenResult | null) => void;
	}
	let { ondone }: Props = $props();

	let server = $state('');
	let list = $state<Listing[] | null>(null);
	let busy = $state(false);
	let error = $state('');
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);
	const servers = $derived(connected(lattis.state.servers));

	onMount(async () => {
		await loadLattis();
		server = connected(lattis.state.servers)[0]?.url ?? '';
		if (server) await load();
	});

	async function load() {
		error = '';
		list = null;
		if (!server) return;
		busy = true;
		try {
			list = (await DocumentService.ShareWorkbooks(server)) ?? [];
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}

	async function login() {
		if (await requestLogin('')) {
			await loadLattis();
			server = connected(lattis.state.servers)[0]?.url ?? '';
			await load();
		}
	}

	async function open(w: Listing) {
		error = '';
		busy = true;
		try {
			ondone(await DocumentService.ShareOpen(server, w.id, w.title));
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}
</script>

<div class="back" role="presentation" onclick={(e) => e.target === e.currentTarget && !busy && ondone(null)}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label="Ouvrir depuis un serveur"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape' && !busy) ondone(null);
		}}
	>
		<header><Icon name="open" size={18} /><span>Ouvrir depuis un serveur</span></header>
		<div class="body">
			<div class="row">
				<label for="open-remote-server">Serveur</label>
				<ServerPicker id="open-remote-server" bind:value={server} {servers} disabled={busy} onchange={load}>
					{#snippet none()}<span class="grow"><button class="primary" onclick={login}>Se connecter à un serveur…</button></span>{/snippet}
				</ServerPicker>
			</div>
			{#if !servers.length}
				<p class="muted">Connectez-vous à un Serveur Lattis pour voir ses classeurs.</p>
			{:else}
				{#if list === null}
					<p class="muted">{busy ? 'Lecture des classeurs…' : ''}</p>
				{:else if !list.length}
					<p class="muted">Aucun classeur partagé avec vous sur ce serveur.</p>
				{:else}
					<ul>
						{#each list as w (w.id)}
							<li>
								<button class="item" disabled={busy} onclick={() => open(w)} ondblclick={() => open(w)}>
									<strong>{w.title}</strong>
									<span class="muted">{modeLabel(w.mode)}{w.mode === 'frozen' ? ` · version ${w.version}` : ''} · {rolesLabel(w.roles)}{w.state === 'republishing' ? ' · nouvelle version en préparation' : ''}</span>
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			{/if}
			{#if error}<p class="error" role="alert">{error}</p>{/if}
		</div>
		<footer>
			<span class="muted grow">{busy && list ? 'Chargement du classeur…' : 'Le classeur est gardé sur ce poste : il reste consultable hors ligne.'}</span>
			<button onclick={() => ondone(null)} disabled={busy}>Fermer</button>
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
		align-items: flex-start;
		justify-content: center;
		padding-top: 8vh;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 480px;
		max-width: calc(100vw - 32px);
		max-height: 84vh;
		display: flex;
		flex-direction: column;
		outline: none;
		--icon-accent: #217346;
	}
	header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 16px;
		font-weight: 600;
		border-bottom: 1px solid #e3e5e8;
	}
	.body {
		padding: 12px 16px;
		overflow: auto;
	}
	p {
		margin: 8px 0;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.row :global(.picker) {
		flex: 1;
	}
	ul {
		list-style: none;
		margin: 10px 0 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	button {
		font: inherit;
		padding: 3px 12px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		color: inherit;
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
	}
	.item {
		width: 100%;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 2px;
		padding: 6px 10px;
		text-align: left;
	}
	.item:hover:not(:disabled) {
		background: #f3f4f6;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.error {
		color: #c0392b;
	}
	footer {
		display: flex;
		gap: 6px;
		align-items: center;
		padding: 10px 16px;
		border-top: 1px solid #e3e5e8;
	}
	.grow {
		flex: 1;
	}
</style>
