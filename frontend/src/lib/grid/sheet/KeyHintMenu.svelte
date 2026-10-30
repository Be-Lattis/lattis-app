<!--
	Menu de la clé proposée pour un tableau.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { dismiss } from '../dismiss';
	import { clip, clipped } from '#lib/ui/menu.js';
	import { t as tr } from '#lib/i18n/index.js';
	import { type SheetSource, type Entry } from './model';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface KeyHintMenuView {
		readonly acceptKeyHint: (e: Entry) => void;
		hintMenu: { x: number; y: number; e: Entry } | null;
		readonly menuAction: (fn: (() => Promise<unknown>) | undefined, keepFocus?: boolean) => Promise<boolean>;
		readonly source: SheetSource;
	}

	let { v }: { v: KeyHintMenuView } = $props();
</script>

{#if v.hintMenu}
	{@const m = v.hintMenu}
	{@const name = m.e.t.columns[0]?.name}
	<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (v.hintMenu = null)}>
		<div class="menu-title" title={clipped(name) ? name : undefined}>{tr('grid.menu.column', { name: clip(name) })}</div>
		<button
			onclick={() => {
				v.hintMenu = null;
				v.acceptKeyHint(m.e);
			}}>{tr('grid.key.accept')}</button
		>
		{#if v.source.dismissKeyHint}
			{@const dismissHint = v.source.dismissKeyHint}
			<button
				onclick={() => {
					v.hintMenu = null;
					v.menuAction(() => dismissHint(m.e.t));
				}}>{tr('grid.key.dismiss')}</button
			>
		{/if}
	</div>
{/if}

<style>

	.menu {
		position: absolute;
		z-index: 5;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 240px;
		max-width: 320px; /* MENU_MAX_WIDTH : au-delà, les libellés s'abrègent (…) */
	}

	.menu > .menu-title {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.menu-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}

	.menu button {
		display: block;
		width: 100%;
		text-align: left;
		padding: 6px 14px;
		background: none;
		border: none;
		font: inherit;
		cursor: pointer;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.menu button:disabled {
		color: #9aa0a6;
		cursor: default;
	}

	.menu button:hover {
		background: #e3efe7;
	}
</style>
