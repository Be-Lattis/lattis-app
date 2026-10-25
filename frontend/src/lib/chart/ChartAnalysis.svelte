<script lang="ts">
	// Volet du graphique, section Analyse : courbe de tendance et barres
	// d'erreur de chaque série (calculées par le classeur, chartfit.go).
	import Select from '#lib/ui/Select.svelte';
	import { t, tAny } from '#lib/i18n/index.js';
	import { ERROR_TYPES, TREND_TYPES, type ErrorBars, type Trendline } from './fits';

	interface Props {
		kind: string;
		series: string[];
		columns: string[];
		trendlines?: (Trendline | null)[] | null;
		errorBars?: (ErrorBars | null)[] | null;
		onchange: (patch: { trendlines?: (Trendline | null)[]; errorBars?: (ErrorBars | null)[] }) => void;
	}
	let { kind, series, columns, trendlines, errorBars, onchange }: Props = $props();

	// Séries proposées (bulles : les Y, une sur deux).
	const ranks = $derived(series.map((_, k) => k).filter((k) => kind !== 'bubble' || k % 2 === 0));
	let pick = $state(0);
	const k = $derived(ranks.includes(pick) ? pick : (ranks[0] ?? 0));
	const tl = $derived(trendlines?.[k] ?? null);
	const eb = $derived(errorBars?.[k] ?? null);

	/** Liste par rang avec l'élément k remplacé ; vide : undefined. */
	function put<X>(list: (X | null)[] | null | undefined, v: X | null): (X | null)[] | undefined {
		const out: (X | null)[] = Array.from({ length: Math.max(series.length, list?.length ?? 0) }, (_, j) => list?.[j] ?? null);
		out[k] = v;
		while (out.length && !out[out.length - 1]) out.pop();
		return out.length ? out : undefined;
	}
	const setTrend = (patch: Partial<Trendline> | null) => onchange({ trendlines: put(trendlines, patch === null ? null : { ...(tl ?? { type: 'linear' }), ...patch }) });
	const setErr = (patch: Partial<ErrorBars> | null) => onchange({ errorBars: put(errorBars, patch === null ? null : { ...(eb ?? { type: 'fixed' }), ...patch }) });
	const int = (s: string, lo: number, hi: number) => Math.min(hi, Math.max(lo, Math.round(Number(s.replace(',', '.')) || lo)));
	const pos = (s: string) => {
		const v = Number(s.replace(/[\s  ]/g, '').replace(',', '.'));
		return isFinite(v) && v > 0 ? v : undefined;
	};

	const trendOpts = $derived([{ value: '', label: t('chart.analysis.none') }, ...TREND_TYPES.map((v) => ({ value: v, label: tAny(`chart.trend.type.${v}`) }))]);
	const errOpts = $derived([{ value: '', label: t('chart.analysis.none') }, ...ERROR_TYPES.map((v) => ({ value: v, label: tAny(`chart.analysis.error.${v}`) }))]);
	const colOpts = $derived(columns.map((c) => ({ value: c, label: c })));
</script>

<section>
	<h3>{t('chart.analysis.title')}</h3>
	{#if ranks.length > 1}
		<label class="line">{t('chart.analysis.series')}
			<Select value={String(k)} options={ranks.map((j) => ({ value: String(j), label: series[j] }))} onchange={(v) => (pick = Number(v))} />
		</label>
	{/if}
	<label class="line">{t('chart.analysis.trend')}
		<Select value={tl?.type ?? ''} options={trendOpts} onchange={(v) => setTrend(v ? { type: v, order: v === 'poly' ? (tl?.order ?? 2) : undefined, period: v === 'movavg' ? (tl?.period ?? 2) : undefined } : null)} />
	</label>
	{#if tl}
		{#if tl.type === 'poly'}
			<label class="line">{t('chart.analysis.order')}<input type="number" min="2" max="6" value={tl.order ?? 2} onchange={(e) => setTrend({ order: int(e.currentTarget.value, 2, 6) })} /></label>
		{:else if tl.type === 'movavg'}
			<label class="line">{t('chart.analysis.period')}<input type="number" min="2" max="255" value={tl.period ?? 2} onchange={(e) => setTrend({ period: int(e.currentTarget.value, 2, 255) })} /></label>
		{/if}
		{#if tl.type !== 'movavg'}
			<div class="line">
				<span>{t('chart.analysis.forecast')}</span>
				<span class="pair">
					<label title={t('chart.analysis.backward')}>−<input value={tl.backward ?? ''} placeholder="0" onchange={(e) => setTrend({ backward: pos(e.currentTarget.value) })} /></label>
					<label title={t('chart.analysis.forward')}>+<input value={tl.forward ?? ''} placeholder="0" onchange={(e) => setTrend({ forward: pos(e.currentTarget.value) })} /></label>
				</span>
			</div>
			<div class="checks">
				<label class="opt"><input type="checkbox" checked={!!tl.equation} onchange={(e) => setTrend({ equation: e.currentTarget.checked || undefined })} /> {t('chart.analysis.equation')}</label>
				<label class="opt"><input type="checkbox" checked={!!tl.r2} onchange={(e) => setTrend({ r2: e.currentTarget.checked || undefined })} /> R²</label>
			</div>
		{/if}
	{/if}
	<label class="line">{t('chart.analysis.errors')}
		<Select value={eb?.type ?? ''} options={errOpts} onchange={(v) => setErr(v ? { type: v, value: v === 'fixed' || v === 'percent' ? (eb?.value ?? (v === 'percent' ? 5 : 1)) : v === 'stddev' ? (eb?.value ?? 1) : undefined, plus: v === 'custom' ? (eb?.plus ?? columns[0]) : undefined, minus: v === 'custom' ? eb?.minus : undefined } : null)} />
	</label>
	{#if eb}
		{#if eb.type === 'fixed' || eb.type === 'percent' || eb.type === 'stddev'}
			<label class="line">{eb.type === 'percent' ? '%' : eb.type === 'stddev' ? t('chart.analysis.times') : t('chart.analysis.value')}<input value={eb.value ?? ''} onchange={(e) => setErr({ value: pos(e.currentTarget.value) })} /></label>
		{:else if eb.type === 'custom'}
			<label class="line">{t('chart.analysis.plus')}<Select value={eb.plus ?? ''} options={colOpts} onchange={(v) => setErr({ plus: v })} /></label>
			<label class="line">{t('chart.analysis.minus')}<Select value={eb.minus ?? ''} options={[{ value: '', label: t('chart.analysis.samePlus') }, ...colOpts]} onchange={(v) => setErr({ minus: v || undefined })} /></label>
		{/if}
		<div class="line">
			<span>{t('chart.analysis.direction')}</span>
			<span class="seg" role="radiogroup" aria-label={t('chart.analysis.direction')}>
				{#each [['both', t('chart.analysis.both')], ['plus', t('chart.analysis.plus')], ['minus', t('chart.analysis.minus')]] as [d, label] (d)}
					<button type="button" role="radio" aria-checked={(eb.dir || 'both') === d} class:on={(eb.dir || 'both') === d} onclick={() => setErr({ dir: d === 'both' ? undefined : d })}>{label}</button>
				{/each}
			</span>
		</div>
		<label class="opt"><input type="checkbox" checked={!eb.noCap} onchange={(e) => setErr({ noCap: !e.currentTarget.checked || undefined })} /> {t('chart.analysis.caps')}</label>
	{/if}
</section>

<style>
	h3 {
		margin: 0 0 6px;
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: #6b7280;
	}
	section {
		border-bottom: 1px solid #e5e7eb;
		padding-bottom: 8px;
	}
	.line {
		display: grid;
		grid-template-columns: 80px 1fr;
		align-items: center;
		gap: 6px;
		margin-bottom: 4px;
	}
	.opt {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-top: 4px;
	}
	.checks {
		display: flex;
		gap: 14px;
	}
	.pair {
		display: flex;
		gap: 8px;
	}
	.pair label {
		display: flex;
		align-items: center;
		gap: 3px;
	}
	input:not([type]),
	input[type='number'] {
		font: inherit;
		padding: 2px 4px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		min-width: 0;
		width: 100%;
		box-sizing: border-box;
	}
	.pair input {
		width: 54px;
	}
	.seg {
		display: flex;
	}
	.seg button {
		flex: 1;
		font: inherit;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		background: #fff;
		cursor: pointer;
	}
	.seg button + button {
		margin-left: -1px;
	}
	.seg .on {
		border-color: #217346;
		background: #e3efe7;
		color: #185c37;
		font-weight: 600;
		position: relative;
	}
</style>
