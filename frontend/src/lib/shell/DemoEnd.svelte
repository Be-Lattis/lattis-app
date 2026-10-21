<script lang="ts">
	// Fin d'une démo publique (docs/collaboration/version-web.md § 6.2) :
	// pendant le préavis, un bandeau qui ne bloque rien (minutes restantes,
	// compte à rebours, « Télécharger ») ; à l'heure, la boîte « Démo
	// terminée », qu'on ne ferme pas : le classeur ne se modifie plus (le
	// moteur refuse aussi les modifications), il se télécharge encore.
	import { onDestroy } from 'svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { modal } from '#lib/ui/focus.js';
	import { t } from '#lib/i18n/index.js';
	import { demoEndState, type DemoClock } from './demoend';

	interface Props {
		/** Fin annoncée (demoClock) ; null : rien à montrer. */
		clock: DemoClock | null;
		/** Télécharge le classeur (Enregistrer de la démo, comme Ctrl+S). */
		ondownload: () => Promise<unknown> | void;
	}
	let { clock, ondownload }: Props = $props();

	let now = $state(Date.now());
	const timer = setInterval(() => (now = Date.now()), 250);
	onDestroy(() => clearInterval(timer));

	const s = $derived(clock ? demoEndState(clock, now) : null);
	let busy = $state(false);

	async function download() {
		if (busy) return;
		busy = true;
		try {
			await ondownload();
		} finally {
			busy = false;
		}
	}
</script>

{#if s?.phase === 'warning'}
	<div class="demo-end" role="status">
		<Icon name="warning" size={14} />
		<span class="text">{t('shell.demoEnd.warning', { n: s.minutes })}</span>
		<span class="countdown" role="timer" aria-label={t('shell.demoEnd.countdown')}>{s.countdown}</span>
		<button disabled={busy} onclick={download}>{t('shell.demoEnd.download')}</button>
	</div>
{:else if s?.phase === 'ended'}
	<div class="modal-back">
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<div
			class="dialog"
			role="alertdialog"
			aria-modal="true"
			aria-labelledby="demo-end-title"
			aria-describedby="demo-end-text"
			tabindex="-1"
			use:modal
			onkeydown={(e) => {
				// Rien n'atteint le classeur derrière ; Ctrl+S télécharge encore.
				e.stopPropagation();
				if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
					e.preventDefault();
					void download();
				}
			}}
		>
			<h2 id="demo-end-title">{t('shell.demoEnd.title')}</h2>
			<p id="demo-end-text">{t('shell.demoEnd.ended')}</p>
			<div class="actions">
				<button class="primary" disabled={busy} onclick={download}>{t('shell.demoEnd.download')}</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.demo-end {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 3px 12px;
		font-size: 12px;
		background: #fdf3e1;
		color: #7c4a03;
		border-bottom: 1px solid #e3e5e8;
		--icon-accent: #b45309;
	}
	.text {
		flex: 1;
		min-width: 0;
	}
	.countdown {
		font-variant-numeric: tabular-nums;
		font-weight: 600;
	}
	.demo-end button {
		font: inherit;
		padding: 1px 10px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		cursor: pointer;
	}
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
		padding: 16px 20px;
		width: 400px;
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
	.actions {
		display: flex;
		justify-content: flex-end;
	}
	.dialog button {
		font: inherit;
		padding: 4px 12px;
		border-radius: 2px;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.primary {
		background: #217346;
		border: 1px solid #217346;
		color: #fff;
	}
</style>
