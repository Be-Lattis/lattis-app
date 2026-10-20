<script lang="ts">
	// Marque d'une capacité (matrice des droits, Voir en tant que…) : oui
	// (✓), en partie (◐ : certaines lignes, certains tableaux), non (—), avec
	// la raison en infobulle et pour les lecteurs d'écran.
	import type { CapCell } from './access';
	import { t } from '#lib/i18n/index.js';

	let { cell, label }: { cell: CapCell; label: string } = $props();
	const word = $derived(cell.state === 'yes' ? t('remote.cap.yes') : cell.state === 'some' ? t('remote.cap.some') : t('remote.cap.no'));
	const why = $derived(cell.why.join('\n'));
	const head = $derived(t('remote.cap.label', { label, word }));
</script>

<span class="ac-cap {cell.state}" role="img" aria-label="{head}{why ? ` — ${cell.why.join(' ; ')}` : ''}" title="{head}{why ? `\n${why}` : ''}">{cell.state === 'yes' ? '✓' : cell.state === 'some' ? '◐' : '–'}</span>
