<script lang="ts" module>
	import type { PivotInfo as Info } from '#bindings/internal/canvas/models.js';
	import { t } from '#lib/i18n/index.js';
	// Dernier résultat de chaque tableau inséré, hors état plié.
	const recent = new Map<string, Info>();
	function identity(cfg: object): string {
		const { collapsed: _a, expanded: _b, collapseAll: _c, width: _d, ...rest } = cfg as Record<string, unknown>;
		return JSON.stringify(rest);
	}
</script>

<script lang="ts">
	// Tableau croisé d'un onglet Document : recalculé à chaque
	// modification du classeur (live.stamp), mis en forme comme sur la
	// feuille (en-tête, nombres, ligne de total) ; avec plusieurs champs en
	// lignes, en arborescence avec ses −/+ (pivottree.ts).
	import { untrack } from 'svelte';
	import type { PivotInfo } from '#bindings/internal/canvas/models.js';
	import type { PivotConfig } from '#lib/grid/Sheet.svelte';
	import type { CellStyle } from '#lib/grid/style.js';
	import { formatCell } from '#lib/grid/numfmt.js';
	import { pivotLabelOffset, setFolded, type PivotRowInfo } from '#lib/grid/pivottree.js';

	interface Props {
		config: PivotConfig;
		live: { stamp: number; animate: boolean };
		load: (cfg: PivotConfig) => Promise<PivotInfo>;
		selected?: boolean;
		/** Nouvelle configuration de l'élément inséré (groupe plié ou déplié par −/+). */
		onconfig?: (cfg: PivotConfig) => void;
	}

	let { config, live, load, selected = false, onconfig }: Props = $props();

	// Plier un groupe remplace l'élément inséré (nouveau composant) : il
	// reprend le dernier résultat du même tableau le temps du calcul, sans
	// clignoter.
	let info = $state.raw<PivotInfo | null>(untrack(() => recent.get(identity(config))) ?? null);
	// Éclat discret de tout le tableau quand son résultat change (Affichage › Animations).
	let flash = $state(false);

	$effect(() => {
		void live.stamp;
		const cfg = config;
		untrack(() => {
			load(cfg).then(
				(r) => {
					const changed = !!info && JSON.stringify(info.cells) !== JSON.stringify(r.cells) && JSON.stringify(info.config) === JSON.stringify(r.config);
					info = r;
					recent.set(identity(cfg), r);
					if (changed && live.animate) {
						flash = false;
						requestAnimationFrame(() => (flash = true));
					}
				},
				(e) => (info = { id: 0, row: 0, col: 0, config: cfg as never, cells: [], rows: null, formats: [], error: String(e instanceof Error ? e.message : e) })
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
	/** Arborescence : description de chaque ligne sous l'en-tête (null : à plat). */
	const tree = $derived((info?.rows ?? null) as PivotRowInfo[] | null);
	function toggle(r: PivotRowInfo) {
		if (r.path && onconfig) onconfig(setFolded(config, r.path, !r.collapsed));
	}
	const total = $derived(!config.noGrandTotal && rows.length > 2);
	// Les valeurs arrivent écrites à la française (« 217731,43 »).
	const value = (s: string) => (/^-?\d+([.,]\d+)?(e[-+]?\d+)?$/i.test(s) ? Number(s.replace(',', '.')) : NaN);
	const num = (s: string) => s !== '' && !isNaN(value(s));
	function shown(text: string, j: number): string {
		if (!num(text)) return text;
		return formatCell(text, value(text), styles[j]);
	}
</script>

<div
	class="dpivot"
	class:selected
	class:flash
	style:width={(config as { width?: number }).width ? (config as { width?: number }).width + 'px' : null}
	onanimationend={() => (flash = false)}
>
	{#if info?.error}
		<div class="err">{t('doc.pivot.error', { error: info.error })}</div>
	{:else if rows.length}
		<table>
			<thead>
				<tr>{#each rows[0] as h, j (j)}<th class:num={j > 0}>{h}</th>{/each}</tr>
			</thead>
			<tbody>
				{#each rows.slice(1) as r, i (i)}
					{@const tn = tree?.[i]}
					<tr class:total={total && i === rows.length - 2} class:group={!!tn && (tn.group || tn.subtotal)}>
						{#each r as c, j (j)}
							{#if j === 0 && tn}
								<td class="label" style:padding-left="{10 + pivotLabelOffset(tn)}px">
									{#if tn.group}
										<button
											class="fold"
											style:left="{6 + tn.depth * 14}px"
											title={tn.collapsed ? t('doc.pivot.expand') : t('doc.pivot.collapse')}
											aria-label={tn.collapsed ? t('doc.pivot.expandName', { name: c }) : t('doc.pivot.collapseName', { name: c })}
											aria-expanded={!tn.collapsed}
											disabled={!onconfig}
											onmousedown={(e) => e.stopPropagation()}
											onclick={(e) => {
												e.stopPropagation();
												toggle(tn);
											}}>{tn.collapsed ? '+' : '−'}</button
										>
									{/if}{c}
								</td>
							{:else}
								<td class:num={num(c)}>{shown(c, j)}</td>
							{/if}
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	{:else if info}
		<div class="muted">{t('doc.pivot.empty', { source: config.source })}</div>
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
	td.label {
		position: relative;
	}
	tr.group td {
		font-weight: 600;
	}
	.fold {
		position: absolute;
		top: 50%;
		transform: translateY(-50%);
		width: 11px;
		height: 11px;
		padding: 0;
		border: 1px solid #8a9199;
		border-radius: 2px;
		background: #fff;
		color: #374151;
		font: 600 10px/9px system-ui, sans-serif;
		text-align: center;
		cursor: pointer;
	}
	.fold:hover:not(:disabled) {
		border-color: #217346;
		color: #217346;
	}
	.fold:disabled {
		cursor: default;
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
