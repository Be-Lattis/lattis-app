<script lang="ts">
	// Mot de passe d'une connexion, demandé quand le serveur le réclame :
	// essayé, puis gardé pour la session (et dans le trousseau si coché).
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { PasswordNeed } from '#bindings/internal/query/models.js';

	interface Props {
		need: PasswordNeed;
		ondone: (ok: boolean) => void;
	}
	let { need, ondone }: Props = $props();

	let password = $state('');
	let remember = $state(true);
	let error = $state('');
	let busy = $state(false);

	async function ok() {
		busy = true;
		error = '';
		try {
			const msg = await DocumentService.SetConnectionPassword(need.connection, password, remember);
			if (msg) error = msg;
			else ondone(true);
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		} finally {
			busy = false;
		}
	}

	function focus(node: HTMLInputElement) {
		node.focus();
	}
</script>

<div class="back">
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label="Mot de passe"
		tabindex="-1"
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') ondone(false);
			if (e.key === 'Enter') ok();
		}}
	>
		<header>Connexion « {need.connection} »</header>
		<div class="body">
			<p>{need.message === 'mot de passe refusé' ? 'Le serveur a refusé le mot de passe.' : 'Le serveur demande un mot de passe.'} <span class="muted">{need.account}</span></p>
			<label>Mot de passe <input type="password" bind:value={password} autocomplete="off" use:focus /></label>
			<label class="check"><input type="checkbox" bind:checked={remember} /> Mémoriser dans le trousseau du système</label>
			<p class="muted">Il n'est jamais enregistré dans le classeur.</p>
		</div>
		{#if error}<p class="error">{error}</p>{/if}
		<footer>
			<button class="primary" onclick={ok} disabled={busy}>OK</button>
			<button onclick={() => ondone(false)}>Annuler</button>
		</footer>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: 80;
		background: rgba(0, 0, 0, 0.25);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.dialog {
		background: #fff;
		border: 1px solid #c9cdd2;
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 380px;
		font-size: 12px;
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
	label {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	label.check {
		flex-direction: row;
		align-items: center;
		gap: 4px;
	}
	input:not([type='checkbox']) {
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 3px 6px;
	}
	.muted {
		color: #6b7280;
		margin: 0;
	}
	p {
		margin: 0;
	}
	.error {
		color: #b42318;
		margin: 6px 14px 0;
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
