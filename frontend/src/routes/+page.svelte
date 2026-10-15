<script lang="ts">
	// Classeur façon Excel : ruban en haut, feuille (ou onglet SQL / SCRIPT) au
	// centre, onglets de feuilles et barre d'état en bas.
	import { onMount } from 'svelte';
	import { GridService } from '#bindings/internal/bridge/index.js';
	import type { GridStatus } from '#bindings/internal/bridge/models.js';
	import Sheet, { type SheetStatus } from '#lib/grid/Sheet.svelte';
	import Ribbon, { type View } from '#lib/shell/Ribbon.svelte';
	import SheetTabs from '#lib/shell/SheetTabs.svelte';
	import SqlView from '#lib/shell/SqlView.svelte';
	import type { ColumnType } from '#lib/grid/format.js';
	import type { Fetched } from '#lib/grid/cache.js';

	let status = $state<GridStatus | null>(null);
	let failure = $state('');
	let bench = $state(false);
	let view = $state<View>('grid');
	let sheetStatus = $state<SheetStatus | null>(null);
	let sheet = $state<ReturnType<typeof Sheet>>();

	onMount(() => {
		let timer: ReturnType<typeof setTimeout>;
		const poll = async () => {
			try {
				status = await GridService.Status();
			} catch (e) {
				failure = String(e);
			}
			if (status?.state !== 'ready' && status?.state !== 'error') timer = setTimeout(poll, 250);
		};
		GridService.BenchRequested().then((b) => (bench = b));
		poll();
		return () => clearTimeout(timer);
	});

	const fetcher = (first: number, count: number) => GridService.Window(first, count) as Promise<Fetched>;
	const save = (id: number, col: number, raw: string) => GridService.SetCell(id, col, raw);
	const query = (sql: string) => GridService.Query(sql);

	const table = $derived(
		status?.state === 'ready'
			? {
					name: status.info.name,
					row: 0,
					col: 0,
					rows: status.info.rows,
					columns: (status.info.columns ?? []).map((c) => ({ name: c.name, type: c.type as ColumnType }))
				}
			: null
	);
	const objects = $derived(
		status?.state === 'ready'
			? [
					{
						name: status.info.name,
						sqlName: status.info.sqlName,
						rows: status.info.rows,
						columns: (status.info.columns ?? []).map((c, i) => ({
							name: c.name,
							type: c.type as ColumnType,
							sql: status!.info.sqlColumns?.[i] ?? ''
						}))
					}
				]
			: []
	);
</script>

<div class="app">
	<Ribbon {view} onview={(v) => (view = v)} onbench={() => sheet?.runBench()} />

	<main>
		{#if table}
			<div class="pane" class:hidden={view !== 'grid'}>
				<Sheet
					bind:this={sheet}
					{table}
					{fetcher}
					{save}
					onstatus={(s) => (sheetStatus = s)}
					autoBench={bench ? (r) => GridService.ReportBench(r) : undefined}
				/>
			</div>
			{#if view === 'sql'}
				<div class="pane"><SqlView {objects} run={query} ongoto={() => (view = 'grid')} /></div>
			{:else if view === 'script'}
				<div class="pane placeholder">
					<h2>SCRIPT</h2>
					<p>Éditeur Lua et bibliothèque de scripts : phase 5 du plan (sandbox gopher-lua, <code>db.query()</code>, <code>db.execute()</code>, bouton Arrêter).</p>
				</div>
			{/if}
		{:else}
			<div class="splash">
				<h1>Slitable</h1>
				{#if failure || status?.state === 'error'}
					<p class="error">{failure || status?.error}</p>
				{:else if status?.state === 'generating'}
					<p>Création du tableau de démonstration : {status.done.toLocaleString('fr-FR')} / {status.total.toLocaleString('fr-FR')} lignes</p>
					<progress max={status.total} value={status.done}></progress>
				{:else}
					<p>Ouverture…</p>
				{/if}
				{#if status?.path}<p class="muted">{status.path}</p>{/if}
			</div>
		{/if}
	</main>

	<SheetTabs sheets={table ? [table.name] : []} active={0} onselect={() => (view = 'grid')} />
	<footer class="statusbar">
		{#if sheetStatus}
			<span class="cell">{sheetStatus.cell}</span>
			{#if sheetStatus.message}<span class="message">{sheetStatus.message}</span>{/if}
			<span class="spacer"></span>
			<span>{sheetStatus.fetch}</span>
			<span>{sheetStatus.draw}</span>
			<span>{sheetStatus.fps}</span>
			<span>{sheetStatus.edit}</span>
		{:else}
			<span>Prêt</span>
		{/if}
	</footer>
</div>

<style>
	:global(html, body) {
		margin: 0;
		height: 100%;
		background: #fff;
		overflow: hidden;
	}
	.app {
		display: flex;
		flex-direction: column;
		height: 100vh;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	main {
		flex: 1;
		min-height: 0;
		position: relative;
	}
	.pane {
		position: absolute;
		inset: 0;
	}
	.hidden {
		visibility: hidden;
	}
	.placeholder {
		padding: 24px;
		color: #4b5563;
	}
	.statusbar {
		display: flex;
		align-items: center;
		gap: 14px;
		height: 24px;
		padding: 0 10px;
		background: #217346;
		color: #e8f3ec;
		font-size: 12px;
		white-space: nowrap;
		overflow: hidden;
	}
	.statusbar .cell {
		color: #fff;
	}
	.message {
		background: #fff7e0;
		color: #7a4b00;
		padding: 1px 8px;
		border-radius: 3px;
	}
	.spacer {
		flex: 1;
	}
	.splash {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		gap: 8px;
	}
	h1 {
		font-weight: 600;
		color: #217346;
		margin: 0 0 8px;
	}
	progress {
		width: 320px;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.error {
		color: #b42318;
	}
</style>
