<script lang="ts">
	// Onglets de feuilles en bas, comme Excel : clic pour changer de feuille,
	// double-clic pour renommer, clic droit pour insérer, renommer ou supprimer,
	// glisser pour réordonner, « + » pour ajouter à la fin.
	interface Sheet {
		id: number;
		name: string;
	}
	interface Props {
		sheets: Sheet[];
		active: number;
		onselect: (id: number) => void;
		onadd?: () => void;
		/** Insère une feuille avant la feuille before. */
		oninsert?: (before: number) => void;
		/** Place la feuille id au rang to (0 : première). */
		onmove?: (id: number, to: number) => void;
		onrename?: (id: number, name: string) => void;
		ondelete?: (id: number) => void;
	}

	let { sheets, active, onselect, onadd, oninsert, onmove, onrename, ondelete }: Props = $props();

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

	// Glisser un onglet : au-delà de quelques pixels, l'onglet suit la souris et
	// un trait marque où il sera déposé (drop : rang d'insertion dans sheets).
	let drag = $state<{ id: number; x0: number; moved: boolean; drop: number } | null>(null);
	let tabEls: HTMLElement[] = $state([]);
	let dropX = $state(0);

	function dragStart(e: PointerEvent, s: Sheet) {
		if (!onmove || e.button !== 0 || sheets.length < 2) return;
		drag = { id: s.id, x0: e.clientX, moved: false, drop: -1 };
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
	}
	function dragMove(e: PointerEvent) {
		if (!drag) return;
		if (!drag.moved && Math.abs(e.clientX - drag.x0) < 6) return;
		drag.moved = true;
		const rects = tabEls.map((el) => el?.getBoundingClientRect());
		let k = rects.findIndex((r) => r && e.clientX < r.left + r.width / 2);
		if (k < 0) k = sheets.length;
		drag.drop = k;
		const host = (e.currentTarget as HTMLElement).parentElement!.getBoundingClientRect();
		const r = rects[Math.min(k, rects.length - 1)];
		dropX = r ? (k < rects.length ? r.left : r.right) - host.left : 0;
	}
	function dragEnd() {
		const d = drag;
		drag = null;
		if (!d?.moved) return;
		const from = sheets.findIndex((s) => s.id === d.id);
		const to = d.drop > from ? d.drop - 1 : d.drop;
		if (from >= 0 && d.drop >= 0 && to !== from) onmove?.(d.id, to);
	}
</script>

<div class="sheet-tabs">
	<div class="nav">
		<button disabled={index <= 0} onclick={() => onselect(sheets[index - 1].id)} title="Feuille précédente">◀</button>
		<button disabled={index < 0 || index >= sheets.length - 1} onclick={() => onselect(sheets[index + 1].id)} title="Feuille suivante">▶</button>
	</div>
	{#each sheets as s, si (s.id)}
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
				bind:this={tabEls[si]}
				class="sheet"
				class:on={s.id === active}
				class:dragging={drag?.moved && drag.id === s.id}
				onpointerdown={(e) => dragStart(e, s)}
				onpointermove={dragMove}
				onpointerup={dragEnd}
				onpointercancel={() => (drag = null)}
				onclick={() => onselect(s.id)}
				ondblclick={() => startRename(s)}
				oncontextmenu={(e) => {
					e.preventDefault();
					if (ondelete || onrename || oninsert) menu = { id: s.id, x: (e.currentTarget as HTMLElement).offsetLeft };
				}}
				title={onmove ? 'Double-clic pour renommer, glisser pour déplacer' : 'Double-clic pour renommer'}>{s.name}</button
			>
		{/if}
	{/each}
	{#if drag?.moved && drag.drop >= 0}<span class="drop-mark" style:left="{dropX - 1}px"></span>{/if}
	<button class="add" disabled={!onadd} onclick={() => onadd?.()} title="Nouvelle feuille">+</button>
	{#if menu}
		{@const m = menu}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="menu" style:left="{m.x}px" onmouseleave={() => (menu = null)}>
			<button
				disabled={!oninsert}
				onclick={() => {
					const id = m.id;
					menu = null;
					oninsert?.(id);
				}}>Insérer une feuille</button
			>
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
					const id = m.id;
					menu = null;
					ondelete?.(id);
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
	.sheet.dragging {
		opacity: 0.6;
		cursor: grabbing;
	}
	.drop-mark {
		position: absolute;
		top: 3px;
		bottom: 3px;
		width: 2px;
		background: #217346;
		pointer-events: none;
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
