<script lang="ts">
	// Barre de progression de la barre d'état (en bas à droite) pendant une
	// tâche longue. Le moteur dit ce qu'il fait (work.ts : « Suppression de
	// la colonne Prix : réécriture du tableau », avec l'avancement quand il
	// le connaît) et, si l'opération peut être interrompue sans rien laisser,
	// « Annuler » la défait. Sinon, faute de mieux : « Traitement… ». Un
	// appel qui attend le serveur d'un classeur partagé (syncCall) est
	// montré par SyncStatus (« Synchronisation… »).
	import { onDestroy, onMount } from 'svelte';
	import { Events } from '@wailsio/runtime';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import { t } from '#lib/i18n/index.js';
	import { busy, syncCalls, trackCalls } from './busy';
	import { onWorkStep, work, workRatio, workText, type WorkStep } from './work';

	// Durée affichée au-delà de 3 s : le moteur travaille encore.
	const SHOW_ELAPSED = 3000;
	let since = $state(0);
	let now = $state(0);
	let shownId = 0;
	let tick: ReturnType<typeof setInterval> | undefined;
	let stopping = $state(0);

	let off: (() => void) | undefined;
	onMount(() => {
		trackCalls();
		off = Events.On('work:step', (e: { data?: WorkStep }) => onWorkStep(e?.data));
	});
	onDestroy(() => {
		off?.();
		clearInterval(tick);
	});

	$effect(() => {
		const w = $work;
		if (w && w.id !== shownId) {
			shownId = w.id;
			since = now = Date.now();
		}
		clearInterval(tick);
		if (w) tick = setInterval(() => (now = Date.now()), 1000);
		else shownId = 0;
	});

	const elapsed = $derived(now - since >= SHOW_ELAPSED ? Math.floor((now - since) / 1000) : 0);
	const ratio = $derived($work ? workRatio($work) : null);

	function cancel() {
		const w = $work;
		if (!w?.cancel) return;
		stopping = w.id;
		DocumentService.CancelWork(w.id).catch(() => undefined);
	}
</script>

{#if $work}
	{@const w = $work}
	<span
		class="busy"
		role="progressbar"
		aria-label={workText(w)}
		aria-busy="true"
		aria-valuemin={ratio === null ? undefined : 0}
		aria-valuemax={ratio === null ? undefined : 100}
		aria-valuenow={ratio === null ? undefined : Math.round(ratio * 100)}
	>
		<span class="label" title={workText(w)}>{workText(w)}{#if elapsed}<span class="elapsed">{' · '}{t('shell.work.elapsed', { s: elapsed })}</span>{/if}</span>
		<span class="track">
			{#if ratio === null}<span class="bar"></span>{:else}<span class="fill" style:width="{ratio * 100}%"></span>{/if}
		</span>
		{#if w.cancel}
			<button class="stop" disabled={stopping === w.id} title={t('shell.work.cancelTooltip')} onclick={cancel}>{stopping === w.id ? t('shell.work.cancelling') : t('shell.work.cancel')}</button>
		{/if}
	</span>
{:else if $busy && !$syncCalls}
	<span class="busy" role="progressbar" aria-label={t('shell.busy.aria')} aria-busy="true">
		<span class="label">{t('shell.busy.label')}</span>
		<span class="track"><span class="bar"></span></span>
	</span>
{/if}

<style>
	.busy {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		color: #fff;
		min-width: 0;
	}
	.label {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		max-width: 46ch;
	}
	.elapsed {
		opacity: 0.8;
	}
	.track {
		position: relative;
		flex: none;
		width: 110px;
		height: 6px;
		border-radius: 3px;
		background: rgba(255, 255, 255, 0.25);
		overflow: hidden;
	}
	.bar {
		position: absolute;
		top: 0;
		bottom: 0;
		width: 40%;
		border-radius: 3px;
		background: #fff;
		animation: slide 1.1s ease-in-out infinite;
	}
	.fill {
		position: absolute;
		top: 0;
		bottom: 0;
		left: 0;
		border-radius: 3px;
		background: #fff;
		transition: width 0.2s ease-out;
	}
	.stop {
		flex: none;
		font: inherit;
		font-size: 11px;
		line-height: 16px;
		padding: 0 8px;
		color: #fff;
		background: rgba(255, 255, 255, 0.15);
		border: 1px solid rgba(255, 255, 255, 0.55);
		border-radius: 3px;
		cursor: pointer;
	}
	.stop:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.3);
	}
	.stop:focus-visible {
		outline: 2px solid #fff;
		outline-offset: 1px;
	}
	.stop:disabled {
		cursor: default;
		opacity: 0.7;
	}
	@keyframes slide {
		from {
			left: -40%;
		}
		to {
			left: 100%;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.bar {
			animation: none;
			left: 0;
			width: 100%;
			opacity: 0.6;
		}
		.fill {
			transition: none;
		}
	}
</style>
