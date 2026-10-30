<!--
	Insérer ou supprimer des lignes ou des colonnes (Ctrl+ +, Ctrl+ -).
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { dismissPanel } from '../dismiss';
	import { t as tr } from '#lib/i18n/index.js';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface LinesDialogView {
		readonly applyLinesDialog: () => void;
		linesDlg: { x: number; y: number; insert: boolean; choice: 'rows' | 'cols' } | null;
	}

	let { v }: { v: LinesDialogView } = $props();
</script>

{#if v.linesDlg}
	{@const d = v.linesDlg}
	<div class="rules lines-dlg" role="dialog" tabindex="-1" aria-label={d.insert ? tr('grid.menu.insert') : tr('common.delete')} use:dismissPanel={() => (v.linesDlg = null)} onkeydown={(e) => e.key === 'Enter' && v.applyLinesDialog()}>
		<div class="menu-title">{d.insert ? tr('grid.menu.insert') : tr('common.delete')}</div>
		<label><input type="radio" bind:group={d.choice} value="rows" /> {d.insert ? tr('grid.lines.dialog.rowAbove') : tr('grid.lines.dialog.row')}</label>
		<label><input type="radio" bind:group={d.choice} value="cols" /> {d.insert ? tr('grid.lines.dialog.colLeft') : tr('grid.lines.dialog.col')}</label>
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" onclick={v.applyLinesDialog}>{tr('common.ok')}</button>
			<button onclick={() => (v.linesDlg = null)}>{tr('common.cancel')}</button>
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
</style>
