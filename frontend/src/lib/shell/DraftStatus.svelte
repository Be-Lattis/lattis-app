<script lang="ts">
	// Barre d'état de l'édition web : enregistrement sur le serveur de ce qui n'est
	// que dans la session (brouillon).
	import { onDestroy } from 'svelte';
	import { draftLabel, draftStatus } from './draft';
	import { BUSY_MIN } from './sync';

	// « Enregistrement… » reste au moins BUSY_MIN ms : pas de clignotement à
	// chaque saisie sur un petit classeur.
	let shown = $state.raw($draftStatus);
	// Pas réactif : l'effet n'écoute que $draftStatus.
	let saving = false;
	let since = 0;
	let timer: ReturnType<typeof setTimeout> | undefined;
	$effect(() => {
		const next = $draftStatus;
		clearTimeout(timer);
		if (next.state === 'saving') {
			if (!saving) since = Date.now();
			saving = true;
			shown = next;
			return;
		}
		const left = saving ? BUSY_MIN - (Date.now() - since) : 0;
		const show = () => {
			saving = false;
			shown = next;
		};
		if (left > 0) timer = setTimeout(show, left);
		else show();
	});
	onDestroy(() => clearTimeout(timer));
	const label = $derived(draftLabel(shown));
</script>

{#if label}
	<span class="draft" class:warn={label.warn} role="status" aria-live="polite" aria-busy={label.busy}>
		{#if label.busy}<span class="spin" aria-hidden="true"></span>{/if}{label.text}
	</span>
{/if}

<style>
	.draft {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex: none;
		color: #fff;
	}
	.draft.warn {
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
