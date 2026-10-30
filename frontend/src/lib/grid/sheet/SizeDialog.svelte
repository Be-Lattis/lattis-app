<!--
	Hauteur des lignes ou largeur des colonnes (Accueil › Format).
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { dismissPanel } from '../dismiss';
	import { t as tr } from '#lib/i18n/index.js';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface SizeDialogView {
		readonly applySize: () => void;
		readonly focusFirst: (node: HTMLElement) => void;
		sizeDlg: { rows: boolean; value: number; x: number; y: number } | null;
		readonly wrap: HTMLDivElement;
	}

	let { v }: { v: SizeDialogView } = $props();
</script>

{#if v.sizeDlg}
	{@const d = v.sizeDlg}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="rules lines-dlg"
		use:v.focusFirst
		use:dismissPanel={() => (v.sizeDlg = null)}
		onkeydown={(ev) => {
			ev.stopPropagation();
			if (ev.key === 'Enter') v.applySize();
			if (ev.key === 'Escape') {
				v.sizeDlg = null;
				v.wrap.focus();
			}
		}}
	>
		<div class="menu-title">{d.rows ? tr('grid.size.rowHeight') : tr('grid.size.colWidth')}</div>
		<div class="add"><input type="number" min="4" max="1000" bind:value={d.value} /></div>
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" onclick={v.applySize}>{tr('common.ok')}</button>
			<button
				onclick={() => {
					v.sizeDlg = null;
					v.wrap.focus();
				}}>{tr('common.cancel')}</button
			>
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

	.rules .add,
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

</style>
