<script lang="ts">
	// Indicateur de la barre d'état pendant le travail à plusieurs sur un
	// classeur d'un dossier partagé (décision 0011) : qui d'autre travaille
	// dessus, conflits ouverts, dossier injoignable.
	import type { State } from '#bindings/internal/coedit/models.js';
	import { presenceText, presenceTitle, presenceWarning } from './coedit';
	import { t } from '#lib/i18n/index.js';

	interface Props {
		state: State | null;
		onconflicts: () => void;
	}
	let { state, onconflicts }: Props = $props();
	const text = $derived(presenceText(state));
	const warning = $derived(presenceWarning(state));
</script>

{#if text}
	<span class="coedit" title={presenceTitle(state)} role="status">
		<span class="dot" class:alone={!(state?.peers ?? []).some((p) => !p.joining)} aria-hidden="true"></span>{text}
	</span>
	{#if state?.conflicts}
		<button class="conflicts" onclick={onconflicts}>{t('remote.coedit.conflicts', { n: state.conflicts })}</button>
	{/if}
	{#if warning}
		<span class="warn" role="alert" title={warning}>{warning}</span>
	{/if}
{/if}

<style>
	.coedit {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		flex: none;
		color: #fff;
	}
	.dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: #8ef0b0;
		box-shadow: 0 0 0 2px rgba(142, 240, 176, 0.3);
	}
	.dot.alone {
		background: #cfd8d3;
		box-shadow: none;
	}
	.conflicts {
		flex: none;
		background: #d97706;
		border: none;
		border-radius: 8px;
		padding: 0 7px;
		color: #fff;
		font-size: 11px;
		line-height: 16px;
		cursor: pointer;
	}
	.warn {
		max-width: 360px;
		overflow: hidden;
		text-overflow: ellipsis;
		background: #fff4e5;
		color: #8a4b00;
		border-radius: 8px;
		padding: 0 7px;
	}
</style>
