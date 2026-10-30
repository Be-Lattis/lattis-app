<script lang="ts">
	// Mot de passe d'une connexion, demandé quand le serveur le réclame :
	// essayé, puis gardé pour la session (et dans le trousseau si coché).
	// S'il est déjà enregistré sur ce poste (need.stored), il suffit
	// d'autoriser ce classeur à s'en servir (accord retenu sur ce poste).
	import { modal } from '#lib/ui/focus.js';
	import { untrack } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { PasswordNeed } from '#bindings/internal/query/models.js';
	import { errorText, t } from '#lib/i18n/index.js';

	interface Props {
		need: PasswordNeed;
		ondone: (ok: boolean) => void;
	}
	let { need, ondone }: Props = $props();

	let password = $state('');
	let remember = $state(true);
	let error = $state('');
	let busy = $state(false);
	// Mot de passe enregistré : « Autoriser » d'abord ; « Saisir un autre
	// mot de passe » revient à la saisie.
	let typing = $state(untrack(() => !need.stored));

	async function allow() {
		busy = true;
		error = '';
		try {
			await DocumentService.AllowConnectionPassword(need.connection);
			ondone(true);
		} catch (e) {
			error = errorText(e);
		} finally {
			busy = false;
		}
	}

	async function ok() {
		busy = true;
		error = '';
		try {
			const msg = await DocumentService.SetConnectionPassword(need.connection, password, remember);
			if (msg) error = msg;
			else ondone(true);
		} catch (e) {
			error = errorText(e);
		} finally {
			busy = false;
		}
	}

	function focus(node: HTMLInputElement) {
		node.focus();
	}
</script>

<div class="back">
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label={t('query.password.title')}
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') ondone(false);
			if (e.key === 'Enter') (typing ? ok() : allow());
		}}
	>
		<header>{t('query.password.header', { name: need.connection })}</header>
		{#if !typing}
			<div class="body">
				<p>{t('query.password.stored')} <span class="muted">{need.account}</span></p>
				<p class="muted">{t('query.password.storedHelp')}</p>
			</div>
			{#if error}<p class="error">{error}</p>{/if}
			<footer>
				<button class="primary" onclick={allow} disabled={busy}>{t('query.password.allow')}</button>
				<button onclick={() => (typing = true)}>{t('query.password.other')}</button>
				<button onclick={() => ondone(false)}>{t('query.password.refuse')}</button>
			</footer>
		{:else}
			<div class="body">
				<p>{need.message === 'mot de passe refusé' ? t('query.password.refused') : t('query.password.asked')} <span class="muted">{need.account}</span></p>
				<label>{t('query.password.title')} <input type="password" bind:value={password} autocomplete="off" use:focus /></label>
				<label class="check"><input type="checkbox" bind:checked={remember} /> {t('query.password.remember')}</label>
				<p class="muted">{t('query.password.never')}</p>
			</div>
			{#if error}<p class="error">{error}</p>{/if}
			<footer>
				<button class="primary" onclick={ok} disabled={busy}>{t('common.ok')}</button>
				<button onclick={() => ondone(false)}>{t('common.cancel')}</button>
			</footer>
		{/if}
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
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
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
