<script lang="ts">
	// Barre de progression de la barre d'état (en bas à droite) pendant une
	// tâche longue : suppression de colonnes sur un million de lignes,
	// actualisation de requêtes, import… La durée n'étant pas connue, la
	// barre défile. Un appel qui attend le serveur d'un classeur partagé
	// (syncCall) est montré par SyncStatus (« Synchronisation… »).
	import { onMount } from 'svelte';
	import { busy, syncCalls, trackCalls } from './busy';

	onMount(trackCalls);
</script>

{#if $busy && !$syncCalls}
	<span class="busy" role="progressbar" aria-label="Traitement en cours" aria-busy="true">
		<span class="label">Traitement…</span>
		<span class="track"><span class="bar"></span></span>
	</span>
{/if}

<style>
	.busy {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		color: #fff;
	}
	.track {
		position: relative;
		width: 110px;
		height: 6px;
		border-radius: 3px;
		background: rgba(255, 255, 255, 0.25);
		overflow: hidden;
	}
	.bar {
		position: absolute;
		top: 0;
		bottom: 0;
		width: 40%;
		border-radius: 3px;
		background: #fff;
		animation: slide 1.1s ease-in-out infinite;
	}
	@keyframes slide {
		from {
			left: -40%;
		}
		to {
			left: 100%;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.bar {
			animation: none;
			left: 0;
			width: 100%;
			opacity: 0.6;
		}
	}
</style>
