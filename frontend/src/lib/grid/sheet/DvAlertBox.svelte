<!--
	Alerte d'une saisie refusée par la validation des données.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { type DvAlert } from '../dvrule';
	import { modal } from '#lib/ui/focus.js';
	import { t as tr } from '#lib/i18n/index.js';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface DvAlertBoxView {
		readonly answerDvAlert: (a: "yes" | "retry" | "cancel") => void;
		readonly dvAlert: (DvAlert & { row: number; col: number; raw: string; keep: boolean }) | null;
		readonly focusFirst: (node: HTMLElement) => void;
	}

	let { v }: { v: DvAlertBoxView } = $props();
</script>

{#if v.dvAlert}
	{@const d = v.dvAlert}
	<div class="dlg-back" role="presentation">
		<div
			class="rules dv-alert"
			role="alertdialog"
			aria-modal="true"
			aria-label={d.title || tr('grid.validation.alert.title')}
			tabindex="-1"
			use:v.focusFirst
			use:modal
			onkeydown={(ev) => {
				ev.stopPropagation();
				if (ev.key === 'Escape') v.answerDvAlert('cancel');
			}}
		>
			<div class="menu-title">{d.title || tr('grid.validation.alert.title')}</div>
			<p class="dv-alert-text" class:stop={d.style === 'stop'} class:warning={d.style === 'warning'}>{d.text}</p>
			{#if d.style === 'warning'}<p class="menu-note">{tr('grid.validation.alert.continue')}</p>{/if}
			<div class="foot">
				<span class="sp"></span>
				{#if d.style === 'stop'}
					<button class="primary" onclick={() => v.answerDvAlert('retry')}>{tr('grid.validation.alert.retry')}</button>
				{:else if d.style === 'warning'}
					<button class="primary" onclick={() => v.answerDvAlert('yes')}>{tr('grid.validation.alert.yes')}</button>
					<button onclick={() => v.answerDvAlert('retry')}>{tr('grid.validation.alert.no')}</button>
				{:else}
					<button class="primary" onclick={() => v.answerDvAlert('yes')}>{tr('common.ok')}</button>
				{/if}
				<button onclick={() => v.answerDvAlert('cancel')}>{tr('common.cancel')}</button>
			</div>
		</div>
	</div>
{/if}

<style>
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}

	button.primary:disabled {
		opacity: 0.5;
	}

	.rules {
		position: absolute;
		z-index: 6;
		top: 60px;
		left: 50%;
		transform: translateX(-50%);
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 6px 18px rgba(0, 0, 0, 0.18);
		padding: 6px 0 8px;
		min-width: 460px;
	}

	.rules .foot {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 3px 14px;
	}

	.rules button {
		font: inherit;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}

	.rules button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}

	.rules .sp {
		flex: 1;
	}

	.menu-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}

	.menu-note {
		margin: 4px 14px;
		max-width: 260px;
		font-size: 12px;
		color: #4b5563;
	}

	.dlg-back {
		position: fixed;
		inset: 0;
		z-index: var(--z-dialog);
		background: var(--backdrop);
	}

	.dv-alert {
		width: 360px;
		min-width: 0;
	}

	.dv-alert-text {
		margin: 6px 14px;
		padding-left: 10px;
		border-left: 3px solid #2f80ed;
		white-space: pre-wrap;
	}

	.dv-alert-text.stop {
		border-left-color: #d93025;
	}

	.dv-alert-text.warning {
		border-left-color: #e8a400;
	}
</style>
