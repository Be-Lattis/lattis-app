<script lang="ts">
	// En attente du chantier C (fil des actions de Claude Desktop / Claude
	// Code dans Lattis) : fichier d'attente, remplacé à l'intégration.
	interface Props {
		/** Aller à la feuille ou au tableau visé par une action. */
		ongoto?: (target: string) => void;
	}
	let { ongoto }: Props = $props();
</script>

<p class="wait" data-goto={ongoto ? '' : undefined}>Activité de Claude dans Lattis : en attente du chantier C.</p>

<style>
	.wait {
		margin: 12px;
		color: #6b7280;
		font-size: 12px;
	}
</style>
