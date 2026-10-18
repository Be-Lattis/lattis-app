<script lang="ts">
	// Erreur que rien n'a rattrapée : au lieu d'un « 500 » nu, le message, la
	// pile et l'endroit du journal ; Recharger relance l'interface.
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { reportError } from '#lib/shell/errors.js';

	let logged = $state('');
	onMount(() => {
		reportError('page', page.error ?? `erreur ${page.status}`).then((p) => (logged = p));
	});
</script>

<div class="crash">
	<h1>Lattis a rencontré une erreur</h1>
	<p>{page.error?.message ?? `Erreur ${page.status}`}</p>
	{#if logged}<p class="muted">Détail enregistré dans {logged}</p>{/if}
	<button onclick={() => location.reload()}>Recharger</button>
</div>

<style>
	.crash {
		font: 14px 'Segoe UI', system-ui, sans-serif;
		padding: 40px;
		color: #1f2328;
	}
	h1 {
		font-size: 18px;
		color: #b42318;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
</style>
