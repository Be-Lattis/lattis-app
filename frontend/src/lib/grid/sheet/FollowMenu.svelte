<!--
	Menu de la pastille d'un tableau qui suit l'onglet à choix.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { dismiss } from '../dismiss';
	import { followLabel } from '#lib/shell/mergesheets.js';
	import { clip, clipped } from '#lib/ui/menu.js';
	import { t as tr } from '#lib/i18n/index.js';
	import { type SheetTable, type SheetData, type SheetSource, type Entry } from './model';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface FollowMenuView {
		readonly data: SheetData;
		followMenu: { x: number; y: number; e: Entry } | null;
		readonly source: SheetSource;
		readonly toggleFollow: (t: SheetTable, on: boolean) => void;
	}

	let { v }: { v: FollowMenuView } = $props();
</script>

{#if v.followMenu}
	{@const m = v.followMenu}
	{@const what = followLabel(v.data?.choice?.name ?? '')}
	<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (v.followMenu = null)}>
		<div class="menu-title" title={clipped(m.e.t.name) ? m.e.t.name : undefined}>{tr('grid.menu.table', { name: clip(m.e.t.name) })}</div>
		<button role="menuitemcheckbox" aria-checked="true" disabled={!v.source.setFollow} onclick={() => v.toggleFollow(m.e.t, false)}><span class="check">✓</span>{tr('grid.menu.follow', { what })}</button>
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

	.check {
		display: inline-block;
		width: 16px;
		color: #217346;
	}
</style>
