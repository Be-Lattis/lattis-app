<script lang="ts">
	// Dialogue des graphiques sparkline (Insertion › Sparklines) : type,
	// données et emplacement, points mis en valeur, couleurs, axe. Tiré d'un
	// tableau : une colonne de graphiques qui suit les lignes du tableau.
	import { untrack } from 'svelte';
	import { backdrop } from '#lib/ui/backdrop.js';
	import { modal } from '#lib/ui/focus.js';
	import { errorText, t } from '#lib/i18n/index.js';
	import { colorOf, type SPARK_COLORS, type SparklineConfig } from './sparkline';

	interface Props {
		config: SparklineConfig;
		/** Colonnes du tableau source (groupe tiré d'un tableau). */
		columns?: string[];
		/** Groupe existant : modifier, supprimer. */
		editing?: boolean;
		onok: (cfg: SparklineConfig) => unknown;
		onremove?: () => void;
		oncancel: () => void;
	}
	let { config, columns = [], editing = false, onok, onremove, oncancel }: Props = $props();

	let cfg = $state<SparklineConfig>(untrack(() => ({ ...config })));
	const isTable = $derived(!!cfg.table);
	// Tableau : colonnes de « De » à « À », dans l'ordre du tableau.
	let from = $state(untrack(() => config.cols?.[0] ?? columns[0] ?? ''));
	let to = $state(untrack(() => config.cols?.[(config.cols?.length ?? 1) - 1] ?? columns[columns.length - 1] ?? ''));
	let failed = $state('');
	let busy = $state(false);

	const KINDS = $derived([
		['line', t('grid.sparkline.kind.line')],
		['column', t('grid.sparkline.kind.column')],
		['winloss', t('grid.sparkline.kind.winloss')]
	] as const);
	const POINTS = $derived(
		[
			['high', 'highColor', t('grid.sparkline.high')],
			['low', 'lowColor', t('grid.sparkline.low')],
			['first', 'firstColor', t('grid.sparkline.first')],
			['last', 'lastColor', t('grid.sparkline.last')],
			['negative', 'negColor', t('grid.sparkline.negative')],
			['markers', 'markerColor', t('grid.sparkline.markers')]
		] as const
	);

	function result(): SparklineConfig {
		const out: SparklineConfig = { ...cfg };
		if (isTable) {
			const a = columns.indexOf(from);
			const b = columns.indexOf(to);
			out.cols = a >= 0 && b >= 0 ? columns.slice(Math.min(a, b), Math.max(a, b) + 1) : (cfg.cols ?? []);
		}
		return out;
	}

	async function ok() {
		busy = true;
		try {
			await onok(result());
		} catch (e) {
			failed = errorText(e);
		} finally {
			busy = false;
		}
	}

	function keys(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') oncancel();
		else if (e.key === 'Enter' && !(e.target instanceof HTMLSelectElement) && !(e.target instanceof HTMLButtonElement)) {
			e.preventDefault();
			void ok();
		}
	}
	const toggle = (k: keyof SparklineConfig, on: boolean) => (cfg = { ...cfg, [k]: on || undefined });
	const paint = (k: keyof typeof SPARK_COLORS, v: string) => (cfg = { ...cfg, [k]: v });
</script>

<div class="sp-back" role="presentation" use:backdrop={oncancel}>
	<div class="sp" role="dialog" aria-modal="true" aria-label={t('grid.sparkline.title')} tabindex="-1" use:modal onkeydown={keys}>
		<h2>{t('grid.sparkline.title')}</h2>
		<div class="kinds" role="radiogroup" aria-label={t('grid.sparkline.type')}>
			{#each KINDS as [k, label] (k)}
				<button role="radio" aria-checked={cfg.kind === k} class:on={cfg.kind === k} onclick={() => (cfg = { ...cfg, kind: k })}>{label}</button>
			{/each}
		</div>
		{#if isTable}
			<div class="row">
				<label><span>{t('grid.sparkline.from')}</span>
					<select bind:value={from} data-autofocus>{#each columns as c (c)}<option value={c}>{c}</option>{/each}</select>
				</label>
				<label><span>{t('grid.sparkline.to')}</span>
					<select bind:value={to}>{#each columns as c (c)}<option value={c}>{c}</option>{/each}</select>
				</label>
			</div>
			<label><span>{t('grid.sparkline.where')}</span>
				<select value={cfg.at ?? ''} onchange={(e) => (cfg = { ...cfg, at: e.currentTarget.value || undefined })}>
					<option value="">{t('grid.sparkline.right', { table: cfg.table ?? '' })}</option>
					{#each columns as c (c)}<option value={c}>{c}</option>{/each}
				</select>
			</label>
			<p class="hint">{t('grid.sparkline.tableHint')}</p>
		{:else}
			<div class="row">
				<label><span>{t('grid.sparkline.data')}</span>
					<input value={cfg.source ?? ''} data-autofocus spellcheck="false" autocomplete="off" oninput={(e) => ((cfg = { ...cfg, source: e.currentTarget.value.trim() }), (failed = ''))} />
				</label>
				<label><span>{t('grid.sparkline.location')}</span>
					<input value={cfg.location ?? ''} spellcheck="false" autocomplete="off" oninput={(e) => ((cfg = { ...cfg, location: e.currentTarget.value.trim() }), (failed = ''))} />
				</label>
			</div>
		{/if}
		<fieldset>
			<legend>{t('grid.sparkline.show')}</legend>
			<div class="points">
				{#each POINTS as [k, c, label] (k)}
					{#if k !== 'markers' || cfg.kind === 'line'}
						<label class="opt">
							<input type="checkbox" checked={!!cfg[k]} onchange={(e) => toggle(k, e.currentTarget.checked)} />
							<span>{label}</span>
							<input type="color" class="sw" title={label} value={colorOf(cfg, c)} onchange={(e) => paint(c, e.currentTarget.value)} />
						</label>
					{/if}
				{/each}
			</div>
		</fieldset>
		<div class="opts">
			<label class="opt"><span>{t('grid.sparkline.color')}</span><input type="color" class="sw" value={colorOf(cfg, 'color')} onchange={(e) => paint('color', e.currentTarget.value)} /></label>
			<label class="opt"><input type="checkbox" checked={!!cfg.axis} onchange={(e) => toggle('axis', e.currentTarget.checked)} /><span>{t('grid.sparkline.axis')}</span></label>
			<label class="opt" title={t('grid.sparkline.sameHint')}><input type="checkbox" checked={!!cfg.same} onchange={(e) => toggle('same', e.currentTarget.checked)} /><span>{t('grid.sparkline.same')}</span></label>
		</div>
		{#if failed}<div class="error" role="alert">{failed}</div>{/if}
		<div class="actions">
			{#if editing && onremove}<button class="remove" onclick={onremove}>{t('grid.sparkline.remove')}</button>{/if}
			<button class="primary" disabled={busy} onclick={ok}>{t('common.ok')}</button>
			<button onclick={oncancel}>{t('common.cancel')}</button>
		</div>
	</div>
</div>

<style>
	.sp-back {
		position: fixed;
		inset: 0;
		z-index: 40;
		background: rgba(0, 0, 0, 0.2);
		display: flex;
		align-items: center;
		justify-content: center;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.sp {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius, 6px);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		padding: 16px 18px;
		width: min(440px, calc(100vw - 32px));
		display: flex;
		flex-direction: column;
		gap: 10px;
		outline: none;
	}
	h2 {
		font-size: 15px;
		font-weight: 600;
		margin: 0;
	}
	.kinds {
		display: flex;
		border: 1px solid #d4d7db;
		border-radius: 3px;
		overflow: hidden;
	}
	.kinds button {
		flex: 1;
		font: inherit;
		border: 0;
		background: #fff;
		padding: 5px 8px;
		cursor: pointer;
	}
	.kinds button + button {
		border-left: 1px solid #d4d7db;
	}
	.kinds button.on {
		background: #e1efe7;
		color: #14532d;
		font-weight: 600;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 3px;
		flex: 1;
	}
	label > span {
		color: #4b5563;
		font-size: 12px;
	}
	.row {
		display: flex;
		gap: 10px;
	}
	input:not([type]),
	select {
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #d4d7db;
		width: 100%;
		box-sizing: border-box;
		background: #fff;
	}
	fieldset {
		border: 1px solid #e5e7eb;
		border-radius: 3px;
		margin: 0;
		padding: 6px 10px 8px;
	}
	legend {
		font-size: 12px;
		color: #4b5563;
		padding: 0 4px;
	}
	.points {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 4px 16px;
	}
	.opt {
		flex-direction: row;
		align-items: center;
		gap: 6px;
		flex: none;
	}
	.opt span {
		flex: 1;
		color: #1f2328;
		font-size: 13px;
	}
	.opts {
		display: flex;
		gap: 18px;
		align-items: center;
	}
	.sw {
		width: 24px;
		height: 18px;
		padding: 0;
		border: 1px solid #c9cdd2;
	}
	.hint {
		margin: 0;
		color: #6b7280;
		font-size: 12px;
	}
	.error {
		color: #c42b1c;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}
	.actions button {
		font: inherit;
		padding: 4px 16px;
		border: 1px solid #d4d7db;
		background: #fff;
		border-radius: 2px;
		cursor: pointer;
	}
	.actions .remove {
		margin-right: auto;
		padding: 4px 10px;
	}
	.actions button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	.actions button:disabled {
		opacity: 0.5;
		cursor: default;
	}
</style>
