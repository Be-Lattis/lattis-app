<script lang="ts">
	// Onglets de feuilles en bas, comme Excel : clic pour changer de feuille,
	// double-clic pour renommer, clic droit pour supprimer, « + » pour ajouter.
	interface Sheet {
		id: number;
		name: string;
	}
	interface Props {
		sheets: Sheet[];
		active: number;
		onselect: (id: number) => void;
		onadd?: () => void;
		onrename?: (id: number, name: string) => void;
		ondelete?: (id: number) => void;
	}

	let { sheets, active, onselect, onadd, onrename, ondelete }: Props = $props();

	let renaming = $state<number | null>(null);
	let draft = $state('');
	let menu = $state<{ id: number; x: number } | null>(null);

	function startRename(s: Sheet) {
		if (!onrename) return;
		renaming = s.id;
		draft = s.name;
	}

	function finish(save: boolean) {
		if (renaming !== null && save && draft.trim()) onrename?.(renaming, draft.trim());
		renaming = null;
	}

	const index = $derived(sheets.findIndex((s) => s.id === active));
</script>

<div class="sheet-tabs">
	<div class="nav">
		<button disabled={index <= 0} onclick={() => onselect(sheets[index - 1].id)} title="Feuille précédente">◀</button>
		<button disabled={index < 0 || index >= sheets.length - 1} onclick={() => onselect(sheets[index + 1].id)} title="Feuille suivante">▶</button>
	</div>
	{#each sheets as s (s.id)}
		{#if renaming === s.id}
			<!-- svelte-ignore a11y_autofocus -->
			<input
				class="rename"
				bind:value={draft}
				autofocus
				onkeydown={(e) => {
					e.stopPropagation();
					if (e.key === 'Enter') finish(true);
					if (e.key === 'Escape') finish(false);
				}}
				onblur={() => finish(true)}
			/>
		{:else}
			<button
				class="sheet"
				class:on={s.id === active}
				onclick={() => onselect(s.id)}
				ondblclick={() => startRename(s)}
				oncontextmenu={(e) => {
					e.preventDefault();
					if (ondelete || onrename) menu = { id: s.id, x: (e.currentTarget as HTMLElement).offsetLeft };
				}}
				title="Double-clic pour renommer">{s.name}</button
			>
		{/if}
	{/each}
	<button class="add" disabled={!onadd} onclick={() => onadd?.()} title="Nouvelle feuille">+</button>
	{#if menu}
		{@const m = menu}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="menu" style:left="{m.x}px" onmouseleave={() => (menu = null)}>
			<button
				onclick={() => {
					const s = sheets.find((x) => x.id === m.id);
					menu = null;
					if (s) startRename(s);
				}}>Renommer</button
			>
			<button
				disabled={sheets.length <= 1}
				onclick={() => {
					menu = null;
					ondelete?.(m.id);
				}}>Supprimer</button
			>
		</div>
	{/if}
</div>

<style>
	.sheet-tabs {
		position: relative;
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
		color: #4b5563;
		font: inherit;
		width: 24px;
		cursor: pointer;
	}
	.nav button:disabled,
	.add:disabled {
		color: #c0c4c9;
		cursor: default;
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
	.rename {
		font: inherit;
		width: 120px;
		border: 1px solid #217346;
		outline: none;
		padding: 0 6px;
	}
	.add {
		font-size: 16px;
	}
	.menu {
		position: absolute;
		bottom: 28px;
		z-index: 10;
		background: #fff;
		border: 1px solid #d4d7db;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 140px;
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
	}
	.menu button:hover:not(:disabled) {
		background: #e3efe7;
	}
	.menu button:disabled {
		color: #9aa0a6;
	}
</style>
