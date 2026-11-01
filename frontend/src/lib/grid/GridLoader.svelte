<script lang="ts">
	// Fine barre animée en haut de la grille : la feuille ou des lignes visibles
	// sont en route. N'apparaît qu'au-delà de 250 ms, pour ne rien faire
	// clignoter (même esprit que la barre de chargement du démarrage).
	let { on }: { on: boolean } = $props();
</script>

{#if on}<div class="gridload" aria-hidden="true"><span></span></div>{/if}

<style>
	.gridload {
		position: absolute;
		z-index: 5;
		top: 0;
		left: 0;
		right: 0;
		height: 3px;
		overflow: hidden;
		pointer-events: none;
		opacity: 0;
		animation: gridload-in 0.01s linear 0.25s forwards;
	}
	.gridload span {
		display: block;
		width: 30%;
		height: 100%;
		border-radius: 2px;
		background: #217346;
		animation: gridload 1.1s ease-in-out infinite;
	}
	@keyframes gridload-in {
		to {
			opacity: 1;
		}
	}
	@keyframes gridload {
		from {
			transform: translateX(-100%);
		}
		to {
			transform: translateX(340%);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.gridload span {
			animation: none;
			width: 100%;
			opacity: 0.6;
		}
	}
</style>
