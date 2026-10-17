<script lang="ts">
	// Tableau croisé d'un onglet Document : recalculé à chaque
	// modification du classeur (live.stamp), mis en forme comme sur la
	// feuille (en-tête, nombres, ligne de total).
	import { untrack } from 'svelte';
	import type { PivotInfo } from '#bindings/internal/canvas/models.js';
	import type { PivotConfig } from '#lib/grid/Sheet.svelte';
	import type { CellStyle } from '#lib/grid/style.js';
	import { formatCell } from '#lib/grid/numfmt.js';

	interface Props {
		config: PivotConfig;
		live: { stamp: number; animate: boolean };
		load: (cfg: PivotConfig) => Promise<PivotInfo>;
		selected?: boolean;
	}

	let { config, live, load, selected = false }: Props = $props();

	let info = $state.raw<PivotInfo | null>(null);
	// Éclat discret de tout le tableau quand son résultat change (Affichage › Animations).
	let flash = $state(false);

	$effect(() => {
		void live.stamp;
		const cfg = config;
		untrack(() => {
			load(cfg).then(
				(r) => {
					const changed = !!info && JSON.stringify(info.cells) !== JSON.stringify(r.cells);
					info = r;
					if (changed && live.animate) {
						flash = false;
						requestAnimationFrame(() => (flash = true));
					}
				},
				(e) => (info = { id: 0, row: 0, col: 0, config: cfg as never, cells: [], formats: [], error: String(e instanceof Error ? e.message : e) })
			);
		});
	});

	const styles = $derived(
		(info?.formats ?? []).map((f) => {
			try {
				return f ? (JSON.parse(f) as CellStyle) : undefined;
			} catch {
				return undefined;
			}
		})
	);
	const rows = $derived(info?.cells ?? []);
	const total = $derived(!config.noGrandTotal && rows.length > 2);
	const num = (s: string) => s !== '' && !isNaN(Number(s));
	function shown(text: string, j: number): string {
		if (!num(text)) return text;
		return formatCell(text, Number(text), styles[j]);
	}
</script>

<div
	class="dpivot"
	class:selected
	class:flash
	class:sized={!!(config as { height?: number }).height}
	style:width={(config as { width?: number }).width ? (config as { width?: number }).width + 'px' : null}
	style:height={(config as { height?: number }).height ? (config as { height?: number }).height + 'px' : null}
	onanimationend={() => (flash = false)}
>
	{#if info?.error}
		<div class="err">Tableau croisé : {info.error}</div>
	{:else if rows.length}
		<table>
			<thead>
				<tr>{#each rows[0] as h, j (j)}<th class:num={j > 0}>{h}</th>{/each}</tr>
			</thead>
			<tbody>
				{#each rows.slice(1) as r, i (i)}
					<tr class:total={total && i === rows.length - 2}>
						{#each r as c, j (j)}<td class:num={num(c)}>{shown(c, j)}</td>{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	{:else}
		<div class="muted">Tableau croisé de « {config.source} » : choisissez des champs (clic droit › Modifier…).</div>
	{/if}
</div>

<style>
	.dpivot {
		margin: 6px 0;
		display: inline-block;
		max-width: 100%;
		overflow-x: auto;
		border: 1px solid transparent;
		border-radius: var(--float-radius, 6px);
		cursor: default;
		user-select: none;
		font-size: 0.95em;
	}
	.dpivot.sized {
		overflow: auto;
	}
	.dpivot:hover {
		border-color: #d4d7db;
	}
	.dpivot.selected {
		border-color: #217346;
		box-shadow: 0 0 0 1px #217346;
	}
	.dpivot.flash {
		animation: pvflash 0.45s ease-out;
	}
	@keyframes pvflash {
		from {
			background: rgba(33, 115, 70, 0.14);
		}
		to {
			background: transparent;
		}
	}
	/* Pas les bordures noires des tableaux de Quill. */
	table {
		width: auto;
		table-layout: auto;
		border-collapse: collapse;
		border: 1px solid #a7d3b8;
	}
	th,
	td {
		padding: 3px 10px;
		white-space: nowrap;
		border: none;
		border-bottom: 1px solid #e8f1ec;
		text-align: left;
	}
	th {
		background: #e2efe7;
		font-weight: 600;
		border-bottom: 1px solid #a7d3b8;
	}
	.num {
		text-align: right;
		font-variant-numeric: tabular-nums;
	}
	tr.total td {
		font-weight: 600;
		background: #e2efe7;
		border-top: 1px solid #a7d3b8;
	}
	.err {
		padding: 12px 16px;
		color: #b42318;
		font-size: 12px;
		background: #fef3f2;
	}
	.muted {
		padding: 12px 16px;
		color: #6b7280;
		font-size: 12px;
		background: #f7f8f9;
	}
</style>
