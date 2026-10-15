<script lang="ts">
	// Onglets de feuilles en bas, comme Excel. Prototype : une seule feuille,
	// nommée d'après le tableau posé en haut à gauche.
	interface Props {
		sheets: string[];
		active: number;
		onselect: (i: number) => void;
	}

	let { sheets, active, onselect }: Props = $props();
</script>

<div class="sheet-tabs">
	<div class="nav">
		<button disabled title="Feuille précédente">◀</button>
		<button disabled title="Feuille suivante">▶</button>
	</div>
	{#each sheets as s, i (s)}
		<button class="sheet" class:on={i === active} onclick={() => onselect(i)}>{s}</button>
	{/each}
	<button class="add" disabled title="Nouvelle feuille : à venir">+</button>
</div>

<style>
	.sheet-tabs {
		display: flex;
		align-items: stretch;
		height: 28px;
		background: #f3f4f6;
		border-top: 1px solid #d4d7db;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		user-select: none;
	}
	.nav {
		display: flex;
		align-items: center;
		padding: 0 6px;
		gap: 2px;
	}
	.nav button,
	.add {
		background: none;
		border: none;
		color: #9aa0a6;
		font: inherit;
		width: 24px;
	}
	.sheet {
		font: inherit;
		padding: 0 18px;
		background: none;
		border: none;
		border-right: 1px solid #d4d7db;
		color: #374151;
		cursor: pointer;
	}
	.sheet.on {
		background: #fff;
		color: #217346;
		font-weight: 600;
		box-shadow: inset 0 -3px 0 #217346;
	}
	.add {
		font-size: 16px;
	}
</style>
