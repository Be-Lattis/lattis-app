<!--
	Couche d'accessibilité de la grille (WAI-ARIA, motif « grid ») : le canvas
	ne dit rien au lecteur d'écran. Une ligne et une cellule cachées portent la
	cellule active ; la grille la désigne par aria-activedescendant={cellId}.
	Une région aria-live annonce ce que le déplacement ne dit pas : passage en
	modification, taille de la sélection, entrée dans un tableau.
-->
<script lang="ts">
	import { announcement, cellLabel, type CellSpeech } from './a11y';

	interface Props {
		/** Identifiant de la cellule cachée, repris par aria-activedescendant. */
		cellId: string;
		speech: CellSpeech;
		/** Ligne et colonne (à partir de 1), pour aria-rowindex et aria-colindex. */
		row: number;
		col: number;
		editing: boolean;
		/** Libellé de la sélection (selectionLabel), vide pour une cellule seule. */
		selection?: string;
	}
	let { cellId, speech, row, col, editing, selection = '' }: Props = $props();

	const label = $derived(cellLabel(speech));

	// Annonce différée : maintenir une flèche ne doit pas lire chaque étape.
	let live = $state('');
	let lastTable: string | undefined;
	$effect(() => {
		const text = announcement(speech, editing, selection, lastTable);
		lastTable = speech.table;
		const t = setTimeout(() => (live = text), 250);
		return () => clearTimeout(t);
	});
</script>

<div class="sr-only" role="row" aria-rowindex={row}>
	<div id={cellId} role="gridcell" aria-colindex={col} aria-selected="true" aria-readonly={speech.computed || undefined}>{label}</div>
</div>
<div class="sr-only" role="status" aria-live="polite" aria-atomic="true">{live}</div>

<style>
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		margin: -1px;
		padding: 0;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
		border: 0;
	}
</style>
