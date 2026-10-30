<!--
	Liste déroulante d'une cellule validée (Alt+↓).
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { type SyncKind } from '../cache';
	import { dismiss } from '../dismiss';
	import { t as tr } from '#lib/i18n/index.js';
	import { type Entry } from './model';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface ListMenuView {
		readonly cellInfo: (r: number, c: number) => { e: Entry; i: number; kind: "header"; value: string; n?: undefined; row?: undefined; } | { e: undefined; kind: "free"; value: string; i?: undefined; n?: undefined; row?: undefined; } | { e: Entry; i: number; n: number; row: { id: number; cells: unknown[]; text: string[]; bad?: Map<number, string>; sty?: Map<number, string>; marks?: Set<number>; links?: Map<number, string>; sync?: Map<number, SyncKind>; locked?: boolean; } | undefined; kind: "data"; value: string; };
		listMenu: { x: number; y: number; values: string[]; row: number; col: number; error?: string; loading?: boolean } | null;
		readonly pickFromList: (v: string) => void;
	}

	let { ui }: { ui: ListMenuView } = $props();
</script>

{#if ui.listMenu}
	{@const m = ui.listMenu}
	<div class="menu list-menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (ui.listMenu = null)}>
		{#each m.values as v (v)}
			<button class:on={ui.cellInfo(m.row, m.col).value === v} onclick={() => ui.pickFromList(v)}>{v}</button>
		{/each}
		{#if m.error}<p class="menu-note error">{tr('grid.validation.listMenu.notFound', { error: m.error })}</p>{:else if m.loading && !m.values.length}<p class="menu-note">{tr('grid.validation.listMenu.reading')}</p>{:else if !m.values.length}<p class="menu-note">{tr('grid.validation.listMenu.empty')}</p>{/if}
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

	.menu-note {
		margin: 4px 14px;
		max-width: 260px;
		font-size: 12px;
		color: #4b5563;
	}

	.menu-note.error {
		color: #c0392b;
	}

	.list-menu {
		min-width: 140px;
		max-height: 260px;
		overflow: auto;
	}

	.list-menu button.on {
		background: #d3e7dc;
	}
</style>
