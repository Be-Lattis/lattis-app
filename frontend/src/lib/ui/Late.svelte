<script lang="ts">
	// Attente à l'écran (« Lecture du serveur… ») : montrée seulement si elle
	// dure, pour qu'une réponse rapide ne fasse rien clignoter. Même seuil que
	// la barre d'état (work.Delay, internal/work).
	import { onMount, type Snippet } from 'svelte';

	let { children, delay = 300 }: { children: Snippet; delay?: number } = $props();
	let show = $state(false);
	onMount(() => {
		const id = setTimeout(() => (show = true), delay);
		return () => clearTimeout(id);
	});
</script>

{#if show}{@render children()}{/if}
