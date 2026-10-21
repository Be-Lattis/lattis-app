<script lang="ts">
	// Erreur que rien n'a rattrapée : au lieu d'un « 500 » nu, le message, la
	// pile et l'endroit du journal ; Recharger relance l'interface.
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { reportError } from '#lib/shell/errors.js';
	import { lang, t } from '#lib/i18n/index.js';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import { resetPaths } from '#lib/shell/settings.js';

	let logged = $state('');
	onMount(() => {
		reportError('page', page.error ?? `erreur ${page.status}`).then((p) => (logged = p));
	});

	// Un réglage peut être la cause (une langue qui fait planter) : les textes
	// de secours restent lisibles, en anglais à côté de la traduction, et en
	// dur si la traduction elle-même échoue.
	function say(key: Parameters<typeof t>[0], fallback: string): string {
		try {
			const s = t(key);
			return lang() === 'en' || lang() === 'fr' ? s : `${s} (${fallback})`;
		} catch {
			return fallback;
		}
	}

	let resetting = $state(false);
	async function reset() {
		if (!confirm(say('shell.crash.resetConfirm', 'Reset all settings to their defaults, then reload?'))) return;
		resetting = true;
		try {
			await DocumentService.ResetSettings(resetPaths());
		} finally {
			location.reload();
		}
	}
</script>

<div class="crash">
	<h1>{t('shell.crash.title')}</h1>
	<p>{page.error?.message ?? t('shell.crash.status', { status: String(page.status) })}</p>
	{#if logged}<p class="muted">{t('shell.crash.logged', { path: logged })}</p>{/if}
	<div class="actions">
		<button onclick={() => location.reload()}>{say('shell.crash.reload', 'Reload')}</button>
		<button class="secondary" disabled={resetting} onclick={reset}>{say('shell.crash.reset', 'Reset settings to defaults')}</button>
	</div>
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
	.actions {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
	}
	button.secondary {
		background: #fff;
		color: #1f2328;
		border-color: #c4c8cc;
	}
	button.secondary:hover {
		background: #f3f4f6;
		border-color: #9ca3af;
	}
</style>
