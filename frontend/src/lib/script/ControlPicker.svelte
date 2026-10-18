<script lang="ts">
	// Liste filtrable des contrôles du classeur (onglet SCRIPT › Pointer un
	// contrôle) : un clic insère son nom au curseur ; « Choisir sur la
	// feuille » lance la pipette.
	import { dismissPanel } from '#lib/grid/dismiss.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { kindLabel, type ControlInfo } from './controls';

	interface Props {
		controls: ControlInfo[];
		error?: string;
		pick: (c: ControlInfo) => void;
		onsheet?: () => void;
		close: () => void;
	}
	let { controls, error = '', pick, onsheet, close }: Props = $props();

	let filter = $state('');
	let cursor = $state(0);

	const shown = $derived.by(() => {
		const q = filter.trim().toLowerCase();
		if (!q) return controls;
		return controls.filter((c) => [c.name, c.label, c.sheet, kindLabel(c.kind)].some((t) => t?.toLowerCase().includes(q)));
	});
	$effect(() => {
		void shown;
		cursor = 0;
	});

	// Le filtre prend la main dès l'ouverture (après le clic sur le bouton de la barre).
	function focus(node: HTMLInputElement) {
		requestAnimationFrame(() => node.focus());
	}

	function key(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') cursor = Math.min(cursor + 1, shown.length - 1);
		else if (e.key === 'ArrowUp') cursor = Math.max(cursor - 1, 0);
		else if (e.key === 'Enter' && shown[cursor]) pick(shown[cursor]);
		else if (e.key === 'Escape') close();
		else return;
		e.preventDefault();
	}
</script>

<div class="picker" use:dismissPanel={close}>
	<input class="filter" placeholder="Filtrer par nom, type ou onglet" bind:value={filter} onkeydown={key} use:focus spellcheck="false" />
	<div class="list" role="listbox" aria-label="Contrôles du classeur">
		{#each shown as c, i (c.id)}
			<button
				role="option"
				aria-selected={i === cursor}
				class:on={i === cursor}
				onmouseenter={() => (cursor = i)}
				onclick={() => pick(c)}
				title={c.label && c.label !== c.name ? `Libellé : ${c.label}` : undefined}
			>
				<span class="name">{c.name || `#${c.id}`}</span>
				<span class="kind">{kindLabel(c.kind)}</span>
				<span class="sheet">{c.sheet}</span>
			</button>
		{:else}
			<p class="empty">{error || (controls.length ? 'Aucun contrôle ne correspond.' : 'Aucun contrôle dans le classeur : Insertion › Bouton ou Input.')}</p>
		{/each}
	</div>
	{#if onsheet}
		<button class="sheet-pick" onclick={onsheet} disabled={!controls.length} title="Afficher les feuilles : cliquer sur un contrôle insère son nom ; Échap annule">
			<Icon name="pick-control" size={14} />Choisir sur la feuille
		</button>
	{/if}
</div>

<style>
	.picker {
		position: absolute;
		top: calc(100% + 4px);
		left: 0;
		z-index: 40;
		width: 380px;
		display: flex;
		flex-direction: column;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		font: 13px 'Segoe UI', system-ui, sans-serif;
	}
	.filter {
		margin: 8px;
		padding: 4px 8px;
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		outline: none;
	}
	.filter:focus {
		border-color: #217346;
	}
	.list {
		max-height: 280px;
		overflow: auto;
		border-top: 1px solid #e3e5e8;
	}
	.list button {
		display: grid;
		grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr) minmax(0, 1fr);
		gap: 8px;
		width: 100%;
		text-align: left;
		background: none;
		border: none;
		font: inherit;
		padding: 5px 10px;
		cursor: pointer;
	}
	.list button.on {
		background: #e3efe7;
	}
	.list span {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.name {
		font-family: Consolas, 'Cascadia Mono', monospace;
		color: #14532d;
	}
	.kind,
	.sheet {
		color: #6b7280;
		font-size: 12px;
	}
	.empty {
		margin: 8px 10px;
		color: #6b7280;
		font-size: 12px;
	}
	.sheet-pick {
		display: flex;
		align-items: center;
		gap: 6px;
		--icon-accent: #217346;
		margin: 0;
		padding: 7px 10px;
		border: none;
		border-top: 1px solid #e3e5e8;
		background: #f7f8f9;
		font: inherit;
		color: #217346;
		text-align: left;
		cursor: pointer;
	}
	.sheet-pick:hover:not(:disabled) {
		background: #e3efe7;
	}
	.sheet-pick:disabled {
		opacity: 0.5;
		cursor: default;
	}
</style>
