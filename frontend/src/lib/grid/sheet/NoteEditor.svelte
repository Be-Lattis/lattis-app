<!--
	Note de la cellule active (Maj+F2) : texte, épinglage.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { dismissPanel } from '../dismiss';
	import Icon from '#lib/icons/Icon.svelte';
	import { t as tr } from '#lib/i18n/index.js';
	import { type SheetSource } from './model';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface NoteEditorView {
		readonly focusFirst: (node: HTMLElement) => void;
		readonly noteDlg: { row: number; col: number; text: string; had: boolean; pinned: boolean; x: number; y: number } | null;
		readonly saveNote: (remove?: boolean) => void;
		readonly source: SheetSource;
		readonly togglePin: () => void;
	}

	let { v }: { v: NoteEditorView } = $props();
</script>

{#if v.noteDlg}
	{@const d = v.noteDlg}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="note-dlg"
		style:left="{d.x}px"
		style:top="{d.y}px"
		use:v.focusFirst
		use:dismissPanel={() => v.noteDlg && v.saveNote()}
		onkeydown={(ev) => {
			ev.stopPropagation();
			if (ev.key === 'Escape' || (ev.key === 'Enter' && ev.ctrlKey)) v.saveNote();
		}}
	>
		<div class="note-bar">
			{#if v.source.pinNote}
				<button class="note-ico" class:on={d.pinned} title={d.pinned ? tr('grid.note.unpin') : tr('grid.note.pin')} aria-pressed={d.pinned} onclick={v.togglePin}
					><Icon name="pin" size={15} /></button
				>
			{/if}
			<button class="note-ico" title={tr('grid.note.delete')} onclick={() => v.saveNote(true)}><Icon name="trash" size={15} /></button>
		</div>
		<textarea bind:value={d.text} rows="6" placeholder={tr('grid.note.placeholder')} aria-label={tr('grid.note.placeholder')}></textarea>
	</div>
{/if}

<style>

	.note-dlg {
		position: absolute;
		z-index: 6;
		width: 250px;
		background: #fff3a8;
		box-shadow: 0 6px 14px rgba(0, 0, 0, 0.2), 0 1px 2px rgba(0, 0, 0, 0.12);
		padding: 2px 10px 10px;
	}

	.note-bar {
		display: flex;
		justify-content: flex-end;
		gap: 2px;
	}

	.note-ico {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		padding: 0;
		border: none;
		border-radius: 3px;
		background: transparent;
		color: #7a6a1f;
		cursor: pointer;
		--icon-accent: #7a6a1f;
	}

	.note-ico:hover {
		background: rgba(0, 0, 0, 0.07);
	}

	.note-ico.on {
		background: rgba(0, 0, 0, 0.12);
		color: #4a3f0f;
		--icon-accent: #c0392b;
	}

	.note-dlg textarea {
		display: block;
		width: 100%;
		box-sizing: border-box;
		font: inherit;
		line-height: 20px;
		padding: 0 2px;
		border: none;
		outline: none;
		resize: vertical;
		color: #3b3410;
		background: transparent repeating-linear-gradient(to bottom, transparent 0, transparent 19px, rgba(160, 130, 30, 0.35) 19px, rgba(160, 130, 30, 0.35) 20px);
		background-attachment: local;
	}
</style>
