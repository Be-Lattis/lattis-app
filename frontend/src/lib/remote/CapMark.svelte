<script lang="ts">
	// Marque d'une capacité (matrice des droits, Voir en tant que…) : oui
	// (✓), en partie (◐ : certaines lignes, certains tableaux), non (—), avec
	// la raison en infobulle et pour les lecteurs d'écran.
	import type { CapCell } from './access';

	let { cell, label }: { cell: CapCell; label: string } = $props();
	const word = $derived(cell.state === 'yes' ? 'oui' : cell.state === 'some' ? 'en partie' : 'non');
	const why = $derived(cell.why.join('\n'));
</script>

<span class="ac-cap {cell.state}" role="img" aria-label="{label} : {word}{why ? ` — ${cell.why.join(' ; ')}` : ''}" title="{label} : {word}{why ? `\n${why}` : ''}">{cell.state === 'yes' ? '✓' : cell.state === 'some' ? '◐' : '–'}</span>
