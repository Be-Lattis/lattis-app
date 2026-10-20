<script lang="ts">
	// Erreur que rien n'a rattrapée : au lieu d'un « 500 » nu, le message, la
	// pile et l'endroit du journal ; Recharger relance l'interface.
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { reportError } from '#lib/shell/errors.js';
	import { t } from '#lib/i18n/index.js';

	let logged = $state('');
	onMount(() => {
		reportError('page', page.error ?? `erreur ${page.status}`).then((p) => (logged = p));
	});
</script>

<div class="crash">
	<h1>{t('shell.crash.title')}</h1>
	<p>{page.error?.message ?? t('shell.crash.status', { status: String(page.status) })}</p>
	{#if logged}<p class="muted">{t('shell.crash.logged', { path: logged })}</p>{/if}
	<button onclick={() => location.reload()}>{t('shell.crash.reload')}</button>
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
	button {
		font: inherit;
		background: #217346;
		color: #fff;
		border: 1px solid #217346;
		border-radius: 3px;
		padding: 4px 15px;
		cursor: pointer;
	}
	button:hover {
		background: #1a5c38;
		border-color: #1a5c38;
	}
</style>
