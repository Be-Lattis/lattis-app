<script lang="ts">
	// Mise à jour dans la barre d'état (bureau seulement : +page.svelte ne
	// l'affiche pas en édition web). Voir update.ts.
	import { onMount } from 'svelte';
	import { Events, Window } from '@wailsio/runtime';
	import { UpdateService } from '#bindings/internal/bridge/index.js';
	import { openOutside } from '#lib/ai/links.js';
	import { t } from '#lib/i18n/index.js';
	import { duplicateView, updateView, type UpdateState } from './update';

	let status = $state<UpdateState | null>(null);
	const view = $derived(updateView(status));
	const dup = $derived(duplicateView(status));

	onMount(() => {
		// Interface chargée : la recherche d'une nouvelle version part tout de suite.
		UpdateService.Loaded().catch(() => {});
		UpdateService.Status()
			.then((s) => (status = s))
			.catch(() => {});
		return Events.On('update:status', (e: { data?: UpdateState }) => {
			if (e.data) status = e.data;
		});
	});

	// Deux installations sur le poste : le désinstalleur de l'autre (invite
	// UAC si elle est pour tous), suivi par update:status jusqu'à la fin.
	function removeDuplicate() {
		if (status?.duplicate) status = { ...status, duplicate: { ...status.duplicate, removing: true, error: '' } };
		UpdateService.RemoveDuplicate().catch(() => {});
	}

	function dismiss() {
		if (status) status = { ...status, updated: '' };
		UpdateService.DismissUpdated().catch(() => {});
	}

	async function act() {
		const v = view;
		if (!v) return;
		switch (v.action) {
			case 'prepare':
				if (status) status = { ...status, phase: 'downloading', error: '' };
				UpdateService.Prepare().catch(() => {});
				break;
			case 'open':
				if (v.url) openOutside(v.url);
				break;
			case 'notes':
				if (v.url) openOutside(v.url);
				dismiss();
				break;
			case 'restart':
				// Fermeture habituelle (questions d'enregistrement, écouteurs
				// before_close) : la mise à jour s'installe, puis Lattis se relance.
				await UpdateService.RequestRestart().catch(() => {});
				Window.Close().catch(() => {});
				break;
			case 'install':
				// Installeur avec son interface (ses erreurs se voient), puis
				// fermeture habituelle pour qu'il puisse remplacer lattis.exe.
				try {
					await UpdateService.InstallManually();
				} catch {
					// L'échec du lancement arrive aussi par update:status.
					return;
				}
				Window.Close().catch(() => {});
				break;
		}
	}
</script>

{#if dup}
	<span class="update" class:warn={dup.warn} role="status" aria-live="polite">
		{#if dup.busy}
			<span class="spin" aria-hidden="true"></span><span class="text" title={dup.title}>{dup.text}</span>
		{:else}
			<button class="text" title={dup.title} onclick={removeDuplicate}>{dup.text}</button>
		{/if}
	</span>
{/if}
{#if view}
	<span class="update" class:warn={view.warn} role="status" aria-live="polite">
		{#if view.busy}
			<span class="spin" aria-hidden="true"></span><span class="text" title={view.title}>{view.text}</span>
		{:else}
			<button class="text" title={view.title} onclick={() => void act()}>{view.text}</button>
		{/if}
		{#if view.link}
			{@const link = view.link}
			<button class="link" onclick={() => openOutside(link.url)}>{link.text}</button>
		{/if}
		{#if view.dismissible}
			<button class="close" title={t('common.close')} aria-label={t('common.close')} onclick={dismiss}>×</button>
		{/if}
	</span>
{/if}

<style>
	.update {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex: none;
		max-width: 60%;
		background: #dbeafe;
		color: #0b3b75;
		padding: 1px 8px;
		border-radius: 3px;
		overflow: hidden;
	}
	.update.warn {
		background: #ffb02e;
		color: #3d2600;
		box-shadow: 0 0 0 1px #e08a00;
	}
	.text {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	button {
		background: none;
		border: none;
		padding: 0;
		color: inherit;
		font: inherit;
		font-size: 12px;
		cursor: pointer;
	}
	button.text {
		text-decoration: underline;
		font-weight: 600;
	}
	button.link {
		text-decoration: underline;
		white-space: nowrap;
		flex: none;
	}
	.close {
		font-size: 14px;
		line-height: 1;
		opacity: 0.7;
	}
	.close:hover {
		opacity: 1;
	}
	.spin {
		width: 10px;
		height: 10px;
		flex: none;
		border-radius: 50%;
		border: 2px solid rgba(11, 59, 117, 0.25);
		border-top-color: #0b3b75;
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
