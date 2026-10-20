<script lang="ts">
	// Paramètres › Mises à jour (bureau seulement) : la version installée, la
	// dernière recherche et son résultat, et « Rechercher maintenant » (sans
	// attendre les 12 h entre deux recherches). Une nouvelle version trouvée
	// est proposée dans la barre d'état (UpdateStatus), comme d'habitude.
	import { onMount } from 'svelte';
	import { Events } from '@wailsio/runtime';
	import { UpdateService } from '#bindings/internal/bridge/index.js';
	import { t } from '#lib/i18n/index.js';
	import { checkLine, type UpdateState } from './update';

	interface Props {
		/** updates.check (et son verrou) : relit l'état quand il change. */
		setting?: unknown;
	}
	let { setting }: Props = $props();

	let status = $state<UpdateState | null>(null);
	let asked = $state(false);
	const busy = $derived(asked || !!status?.checking);
	const line = $derived(checkLine(status));

	$effect(() => {
		void setting;
		UpdateService.Status()
			.then((s) => (status = s))
			.catch(() => {});
	});

	onMount(() =>
		Events.On('update:status', (e: { data?: UpdateState }) => {
			if (e.data) status = e.data;
		})
	);

	async function checkNow() {
		asked = true;
		try {
			status = await UpdateService.CheckNow();
		} catch {
			// L'évènement update:status donne l'état.
		} finally {
			asked = false;
		}
	}
</script>

{#if status}
	<div class="check">
		<p class="line" role="status" aria-live="polite">{line}</p>
		{#if !status.unavailable}
			<button disabled={busy} aria-busy={busy} onclick={() => void checkNow()}>
				{#if busy}<span class="spin" aria-hidden="true"></span>{/if}{t('shell.update.check.now')}
			</button>
		{/if}
	</div>
{/if}

<style>
	.check {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 6px;
		margin-bottom: 12px;
	}
	.line {
		margin: 0;
		color: #4b5563;
		font-size: 12px;
	}
	button {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font: inherit;
		padding: 3px 12px;
		border: 1px solid #c4c8cc;
		background: #fff;
		cursor: pointer;
	}
	button:disabled {
		cursor: default;
		color: #6b7280;
	}
	.spin {
		width: 10px;
		height: 10px;
		flex: none;
		border-radius: 50%;
		border: 2px solid rgba(33, 115, 70, 0.25);
		border-top-color: #217346;
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
