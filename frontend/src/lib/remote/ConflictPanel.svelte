<script lang="ts">
	// Volet Conflits d'un classeur collaboratif (spécification §7) : une
	// cellule modifiée par un collègue entre-temps. « Garder la mienne »
	// renvoie ma valeur, conditionnée à la valeur actuelle du serveur ;
	// « Garder la sienne » reprend la valeur du serveur.
	import type { Conflict } from '#bindings/internal/remote/collab/models.js';
	import { resizePanel } from '#lib/ui/resize.js';
	import { panelKeys } from '#lib/ui/focus.js';
	import PanelHeader from '#lib/ui/PanelHeader.svelte';
	import { conflictText } from './share';

	interface Props {
		conflicts: Conflict[];
		busy?: boolean;
		onresolve: (c: Conflict, mine: boolean) => void;
		ongoto: (c: Conflict) => void;
		onclose: () => void;
	}
	let { conflicts, busy = false, onresolve, ongoto, onclose }: Props = $props();
</script>

<aside class="panel" use:resizePanel={{ key: 'conflicts' }} use:panelKeys={onclose} aria-label="Conflits">
	<PanelHeader title="Conflits ({conflicts.length})" {onclose} />
	{#if !conflicts.length}
		<p class="muted">Aucun conflit en attente.</p>
	{:else}
		<p class="muted">Ces cellules ont été modifiées par un collègue pendant que vous les changiez. Votre valeur reste affichée jusqu'à votre choix.</p>
	{/if}
	{#each conflicts as c (c.id)}
		<div class="item">
			<button class="where" disabled={!c.found} title={c.found ? 'Aller à la cellule' : 'Ligne supprimée ou masquée par un filtre'} onclick={() => ongoto(c)}>
				« {c.table} » · {c.column}{c.found ? ` · ligne ${c.row}` : ''}
			</button>
			<span class="text">{conflictText(c)}</span>
			<div class="acts">
				<button disabled={busy} onclick={() => onresolve(c, true)}>Garder la mienne</button>
				<button disabled={busy} onclick={() => onresolve(c, false)}>Garder la sienne</button>
			</div>
		</div>
	{/each}
</aside>

<style>
	.panel {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 8px 10px;
		border-left: 1px solid #d4d7db;
		background: #fafbfb;
		box-shadow: -4px 0 12px rgba(0, 0, 0, 0.08);
		overflow: auto;
		font-size: 12px;
	}
	.muted {
		color: #6b7280;
		margin: 0;
	}
	.item {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 6px 8px;
		border: 1px solid #f0d9bd;
		border-left: 3px solid #d97706;
		border-radius: 3px;
		background: #fff;
	}
	.where {
		align-self: flex-start;
		border: none;
		background: none;
		padding: 0;
		font: inherit;
		font-weight: 600;
		color: #14532d;
		cursor: pointer;
		text-align: left;
	}
	.where:disabled {
		color: #6b7280;
		cursor: default;
	}
	.acts {
		display: flex;
		gap: 6px;
	}
	.acts button {
		font: inherit;
		padding: 2px 10px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		cursor: pointer;
	}
	.acts button:disabled {
		opacity: 0.55;
	}
</style>
