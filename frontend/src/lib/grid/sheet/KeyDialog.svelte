<!--
	Question posée quand une clé est proposée pour un tableau.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { type KeyChoice, type KeyQuestion } from '../keychoice';
	import { dismissPanel } from '../dismiss';
	import { t as tr } from '#lib/i18n/index.js';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface KeyDialogView {
		readonly applyKeyDialog: () => void;
		readonly cancelKeyDialog: () => void;
		readonly focusFirst: (node: HTMLElement) => void;
		readonly keyDlg: (KeyQuestion & { row: number; col: number; raw: string; keep: boolean; choice: KeyChoice }) | null;
	}

	let { v }: { v: KeyDialogView } = $props();
</script>

{#if v.keyDlg}
	{@const d = v.keyDlg}
	<div
		class="rules lines-dlg key-dlg"
		role="dialog"
		tabindex="-1"
		aria-label={tr('grid.keyDialog.title')}
		use:v.focusFirst
		use:dismissPanel={v.cancelKeyDialog}
		onkeydown={(ev) => {
			ev.stopPropagation();
			if (ev.key === 'Enter') v.applyKeyDialog();
			if (ev.key === 'Escape') v.cancelKeyDialog();
		}}
	>
		<div class="menu-title">{tr('grid.keyDialog.heading', { table: d.table })}</div>
		<label><input type="radio" bind:group={d.choice} value="index" /> {tr('grid.keyDialog.index')}</label>
		<label title={d.why ? tr('grid.keyDialog.notKey', { column: d.column, why: d.why }) : undefined}>
			<input type="radio" bind:group={d.choice} value="column" disabled={!!d.why} /> {tr('grid.keyDialog.column', { column: d.column })}
		</label>
		{#if d.why}
			<p class="menu-note">{tr('grid.keyDialog.notKey.note', { column: d.column, why: d.why })}</p>
		{:else if d.choice === 'index'}
			<p class="menu-note">{tr('grid.keyDialog.index.note')}</p>
		{/if}
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" onclick={v.applyKeyDialog}>{tr('grid.formula.validate')}</button>
			<button onclick={v.cancelKeyDialog}>{tr('common.cancel')}</button>
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

	.rules input {
		font: inherit;
		width: 90px;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
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

	.lines-dlg input[type='radio'] {
		width: auto;
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

	.lines-dlg {
		left: 50%;
		top: 30%;
		transform: translateX(-50%);
		width: auto;
		min-width: 240px;
	}

	.lines-dlg label {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 3px 2px;
	}

	.key-dlg {
		max-width: 440px;
	}

	.key-dlg label {
		align-items: flex-start;
	}

	.key-dlg .menu-note {
		max-width: none;
	}
</style>
