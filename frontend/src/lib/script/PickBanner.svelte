<script lang="ts">
	// Bandeau de la pipette (onglet Script › Pointer un contrôle › Choisir sur
	// la feuille), affiché par-dessus les feuilles tant qu'elle est active.
	import { finishPick, picker } from './pick.svelte';
</script>

{#if picker.active}
	<div class="banner" role="status">
		<span>Cliquez sur un contrôle (bouton, champ…) pour insérer son nom dans le script — Échap pour annuler.</span>
		<button onclick={() => finishPick(null)}>Annuler</button>
	</div>
{/if}

<style>
	.banner {
		position: fixed;
		top: 8px;
		left: 50%;
		translate: -50% 0;
		z-index: var(--z-toast);
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 6px 8px 6px 14px;
		background: #14532d;
		color: #fff;
		border-radius: var(--float-radius, 6px);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
		font: 13px 'Segoe UI', system-ui, sans-serif;
	}
	button {
		font: inherit;
		padding: 3px 10px;
		border: 1px solid rgba(255, 255, 255, 0.6);
		border-radius: 3px;
		background: transparent;
		color: #fff;
		cursor: pointer;
	}
	button:hover {
		background: rgba(255, 255, 255, 0.15);
	}
</style>
