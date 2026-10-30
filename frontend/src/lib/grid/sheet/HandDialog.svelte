<!--
	Question posée pour une colonne saisie à la main à côté d'une clé.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { dismissPanel } from '../dismiss';
	import { t as tr } from '#lib/i18n/index.js';
	import { type SheetTable } from './model';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface HandDialogView {
		readonly applyHandDialog: () => void;
		readonly cancelHandDialog: () => void;
		readonly focusFirst: (node: HTMLElement) => void;
		readonly handDlg: { table: string; columns: string[]; choice: string; write?: { row: number; col: number; raw: string; keep: boolean }; t?: SheetTable } | null;
	}

	let { v }: { v: HandDialogView } = $props();
</script>

{#if v.handDlg}
	{@const d = v.handDlg}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="rules lines-dlg key-dlg"
		role="dialog"
		tabindex="-1"
		aria-label={tr('grid.handDialog.title')}
		use:v.focusFirst
		use:dismissPanel={v.cancelHandDialog}
		onkeydown={(ev) => {
			ev.stopPropagation();
			if (ev.key === 'Enter') v.applyHandDialog();
			if (ev.key === 'Escape') v.cancelHandDialog();
		}}
	>
		<div class="menu-title">{tr('grid.handDialog.heading', { table: d.table })}</div>
		{#each d.columns as c (c)}
			<label><input type="radio" bind:group={d.choice} value={c} /> {c.includes(' + ') ? tr('grid.handDialog.columns', { name: c }) : tr('grid.handDialog.column', { name: c })}</label>
		{/each}
		<label><input type="radio" bind:group={d.choice} value="#" /> {tr('grid.handDialog.rank')}</label>
		{#if d.choice === '#'}
			<p class="menu-note">{tr('grid.handDialog.rank.note')}</p>
		{:else if d.t}
			<p class="menu-note">{tr('grid.handDialog.change.note')}</p>
		{:else}
			<p class="menu-note">{tr('grid.handDialog.key.note', { key: d.choice })}</p>
		{/if}
		{#if !d.columns.length && !d.t}
			<p class="menu-note">{tr('grid.handDialog.none.note')}</p>
		{/if}
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" onclick={v.applyHandDialog}>{tr('grid.formula.validate')}</button>
			<button onclick={v.cancelHandDialog}>{tr('common.cancel')}</button>
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
