<script lang="ts">
	import { onMount } from 'svelte';
	import { GridService } from '#bindings/internal/bridge/index.js';
	import type { GridStatus } from '#bindings/internal/bridge/models.js';
	import Grid from '#lib/grid/Grid.svelte';
	import type { ColumnType } from '#lib/grid/format.js';
	import type { Fetched } from '#lib/grid/cache.js';

	let status = $state<GridStatus | null>(null);
	let failure = $state('');
	let bench = $state(false);

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
</script>

<main>
	{#if status?.state === 'ready'}
		<Grid
			name={status.info.name}
			rows={status.info.rows}
			columns={(status.info.columns ?? []).map((c) => ({ name: c.name, type: c.type as ColumnType }))}
			{fetcher}
			{save}
			autoBench={bench ? (r) => GridService.ReportBench(r) : undefined}
		/>
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

<style>
	:global(html, body) {
		margin: 0;
		height: 100%;
		background: #fff;
	}
	main {
		height: 100vh;
	}
	.splash {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		gap: 8px;
		font: 14px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
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
