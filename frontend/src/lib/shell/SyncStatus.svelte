<script lang="ts">
	// Barre d'état d'un classeur du serveur (décision 0012) : les
	// modifications sont déjà faites en local ; ici, leur envoi au serveur
	// (« Synchronisation… », puis « Synchronisé »), ou leur attente hors
	// ligne. Remplace « Traitement… » (BusyBar) pour ce qui part au serveur.
	import { onDestroy } from 'svelte';
	import { t } from '#lib/i18n/index.js';
	import { syncCalls } from './busy';
	import { BUSY_MIN, DONE_FOR, syncActive, syncLabel, type SyncStatus } from './sync';

	interface Props {
		status: SyncStatus | null;
	}
	let { status }: Props = $props();

	let done = $state(false);
	// « Synchronisation… » reste au moins BUSY_MIN ms : pas de clignotement
	// quand l'envoi est très rapide.
	let holding = $state(false);
	let wasActive = false;
	let since = 0;
	let timer: ReturnType<typeof setTimeout> | undefined;
	$effect(() => {
		const active = syncActive(status, $syncCalls);
		if (active) {
			if (!wasActive && !holding) since = Date.now();
			done = false;
			holding = false;
			clearTimeout(timer);
		} else if (wasActive && status?.online) {
			clearTimeout(timer);
			const left = BUSY_MIN - (Date.now() - since);
			const finish = () => {
				holding = false;
				done = true;
				timer = setTimeout(() => (done = false), DONE_FOR);
			};
			if (left > 0) {
				holding = true;
				timer = setTimeout(finish, left);
			} else finish();
		}
		wasActive = active;
	});
	onDestroy(() => clearTimeout(timer));
	const label = $derived(syncLabel(status, $syncCalls + (holding ? 1 : 0), done));
</script>

{#if label}
	<span class="sync" class:warn={label.warn} role="status" aria-live="polite" aria-busy={label.busy} title={label.busy ? t('shell.sync.busyTooltip') : ''}>
		{#if label.busy}<span class="spin" aria-hidden="true"></span>{/if}{label.text}
	</span>
{/if}

<style>
	.sync {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex: none;
		color: #fff;
	}
	.sync.warn {
		background: #fff4e5;
		color: #8a4b00;
		border-radius: 8px;
		padding: 0 7px;
	}
	.spin {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		border: 2px solid rgba(255, 255, 255, 0.35);
		border-top-color: #fff;
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.spin {
			animation: none;
			border-color: #fff;
			opacity: 0.7;
		}
	}
</style>
