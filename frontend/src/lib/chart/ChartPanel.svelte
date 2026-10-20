<script lang="ts">
	import { resizePanel } from '#lib/ui/resize.js';
	import { panelKeys } from '#lib/ui/focus.js';
	import PanelHeader from '#lib/ui/PanelHeader.svelte';
	// Volet « Mise en forme du graphique » : chaque réglage s'applique tout de
	// suite (aperçu direct, Ctrl+Z pour revenir en arrière).
	import type { ChartConfig } from '#lib/grid/Sheet.svelte';
	import { PALETTE } from './scale';
	import Select from '#lib/ui/Select.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { BY_CATEGORY, CARTESIAN, KINDS, SECONDARY, SINGLE_SERIES, STACKABLE, chartFields, chartNotes, mapChoice, mapComplete, mapPatch, reorderSeries, seriesShape, type MapChoice, type MapGuess } from './kinds';
	import { fmtNumber, t } from '#lib/i18n/index.js';

	/** Une source possible du graphique (tableau ou TCD) et ses colonnes. */
	export interface ChartSource {
		name: string;
		pivot?: boolean;
		columns: { name: string; numeric: boolean }[];
	}

	interface Props {
		/** Bord gauche à tirer (faux : inclus dans un volet qui se redimensionne lui-même). */
		resizable?: boolean;
		config: ChartConfig;
		columns: string[];
		/** Carte : colonnes de coordonnées repérées par le classeur (proposées en attente et au changement de mode). */
		guess?: MapGuess | null;
		/** Carte : coordonnées illisibles ou hors bornes, ignorées. */
		invalid?: number;
		/** Données dessinées (remarques : coupure, valeurs négatives ignorées…, couleurs par catégorie). */
		info?: { labels?: string[] | null; series?: { values: (number | null)[] | null }[] | null; xValues?: (number | null)[] | null; total?: number; truncated?: boolean; sampled?: boolean } | null;
		/** Graphique posé sur une feuille (il se déplace aussi au clavier). */
		onSheet?: boolean;
		/** Tableaux et TCD du classeur : changer la source du graphique. */
		sources?: ChartSource[];
		onchange: (cfg: ChartConfig) => void;
		onclose: () => void;
		/** Feuille protégée : le message, et les champs grisés. */
		locked?: string;
	}

	let { config, columns, guess = null, invalid = 0, info = null, sources = [], onSheet = false, onchange, onclose, resizable = true, locked = '' }: Props = $props();

	// Le titre et les champs texte s'appliquent à la sortie du champ.
	function set(patch: Partial<ChartConfig>) {
		if (locked) return;
		onchange({ ...config, ...patch });
	}
	function num(s: string): number | undefined {
		// Espaces (séparateur de milliers « 1 000 ») et virgule décimale acceptés.
		const t = s.replace(/[\s  ]/g, '').replace(',', '.');
		return t === '' || isNaN(Number(t)) ? undefined : Number(t);
	}
	const notes = $derived(isMapKind(config.kind) ? [] : chartNotes(config, info));
	function isMapKind(k: string) {
		return k === 'map';
	}
	// Séries : cochées dans leur ordre (celui du dessin et de la légende), puis les autres colonnes.
	const listed = $derived([...config.series.filter((s) => columns.includes(s)), ...columns.filter((c) => c !== config.x && !config.series.includes(c))]);
	function setSeries(next: string[]) {
		if (next.length) set(reorderSeries(config, next));
	}
	function toggleSeries(c: string, on: boolean) {
		// Types à une seule série : cocher remplace la série dessinée.
		if (on && SINGLE_SERIES.has(config.kind)) return setSeries([c]);
		setSeries(on ? [...config.series, c] : config.series.filter((x) => x !== c));
	}
	function moveSeries(k: number, d: number) {
		const next = [...config.series];
		const j = k + d;
		if (j < 0 || j >= next.length) return;
		[next[k], next[j]] = [next[j], next[k]];
		setSeries(next);
	}
	// Autre source (tableau ou TCD) : axe X et séries choisis comme à l'insertion.
	function setSource(name: string) {
		const src = sources.find((s) => s.name === name);
		if (!src || name === config.source) return;
		const f = chartFields(config.kind, src.columns);
		if (!f.series.length) return;
		set({ source: name, x: f.x, series: f.series, colors: BY_CATEGORY.has(config.kind) ? config.colors : undefined, secondary: undefined, seriesKinds: undefined, filters: undefined });
	}
	// Filtres du graphique (posés par Regrouper des onglets : Mois = Janvier) : montrés, retirables.
	const filters = $derived((config.filters ?? []).filter((f) => f.values?.length));
	function dropFilter(i: number) {
		const next = filters.filter((_, j) => j !== i);
		set({ filters: next.length ? next : undefined });
	}
	// Secteurs, anneau, entonnoir, compartimentage : une couleur par catégorie.
	const byCategory = $derived(BY_CATEGORY.has(config.kind));
	const catLabels = $derived((info?.labels ?? []).slice(0, 24));
	function setColorAt(i: number, c: string, n: number) {
		const colors = Array.from({ length: Math.max(n, config.colors?.length ?? 0) }, (_, j) => config.colors?.[j] || PALETTE[j % PALETTE.length]);
		colors[i] = c;
		set({ colors });
	}
	const stackable = $derived(STACKABLE.has(config.kind));
	const axes = $derived(CARTESIAN.has(config.kind));
	const isMap = $derived(config.kind === 'map');
	// Bornes : axes, radar et jauge (minimum et maximum du cadran).
	const bounds = $derived(axes || config.kind === 'radar' || config.kind === 'gauge');
	// Axe secondaire (Excel) : histogramme, courbes, aires, nuage, combiné.
	const canSecondary = $derived(SECONDARY.has(config.kind));
	const SHAPES = $derived([
		{ value: 'bar', label: t('chart.panel.shape.bar') },
		{ value: 'line', label: t('chart.panel.shape.line') },
		{ value: 'area', label: t('chart.panel.shape.area') }
	]);
	function setShape(k: number, v: string) {
		const seriesKinds = config.series.map((_, j) => seriesShape(config.seriesKinds, j) as string);
		seriesKinds[k] = v;
		set({ seriesKinds });
	}
	// Changer de type : la carte attend le choix de ses coordonnées (carré
	// grisé, colonnes repérées proposées) ; en la quittant, la valeur
	// redevient la série.
	function setKind(kind: ChartConfig['kind']) {
		if (kind === config.kind) return;
		if (kind === 'map') {
			set({ kind, series: [], mapDraft: true, mapCoords: undefined, colors: undefined, secondary: undefined, seriesKinds: undefined });
			return;
		}
		if (config.kind === 'map') {
			const m = mapChoice(config, guess);
			const first = m.value || m.cols.find(Boolean) || columns.find((c) => c !== config.x) || '';
			set({ kind, series: first ? [first] : [], colors: undefined, secondary: undefined, mapMode: undefined, mapCoords: undefined, mapDraft: undefined, colorBy: undefined });
			return;
		}
		set({ kind });
	}
	const none = '';
	// Carte : choix affiché (celui de la carte, ou les colonnes repérées).
	const map = $derived(mapChoice(config, guess));
	function setMap(patch: Partial<MapChoice>, validate: boolean) {
		set(mapPatch({ ...map, ...patch }, validate));
	}
	function setMapCol(i: number, c: string) {
		const cols = [...map.cols];
		cols[i] = c;
		setMap({ cols }, true);
	}
	function setCoords(coords: 'pair' | 'single') {
		if (coords === map.coords) return;
		const cols = coords === 'single' ? [guess?.coord ?? ''] : [guess?.lat ?? '', guess?.lon ?? ''];
		setMap({ coords, cols }, false);
	}
	const colOptions = $derived([{ value: none, label: t('chart.panel.choose') }, ...columns.map((c) => ({ value: c, label: c }))]);
	const hasSecondary = $derived(canSecondary && config.series.some((_, k) => config.secondary?.[k]));
	function toggleSecondary(k: number) {
		const secondary = config.series.map((_, j) => !!config.secondary?.[j]);
		secondary[k] = !secondary[k];
		set({ secondary: secondary.some(Boolean) ? secondary : undefined });
	}
</script>

<aside class="panel" use:resizePanel={{ key: 'chart', off: !resizable }} use:panelKeys={onclose} aria-label={t('chart.chart')}>
	<PanelHeader title={t('chart.chart')} help="graphiques" helpLabel={t('chart.panel.help')} {onclose} />
	{#if locked}<p class="locked" role="note"><Icon name="lock-sheet" size={14} /><span>{locked}</span></p>{/if}

	<fieldset class="body" disabled={!!locked}>
	<section>
		<h3>{t('chart.panel.type')}</h3>
		<div class="kinds">
			{#each KINDS as k (k.kind)}
				<button class:on={config.kind === k.kind} title={k.hint} onclick={() => setKind(k.kind)}><Icon name={k.icon} size={16} />{k.label}</button>
			{/each}
		</div>
		{#if stackable}
			<label class="opt"><input type="checkbox" checked={!!config.stacked} onchange={(e) => set({ stacked: (e.currentTarget as HTMLInputElement).checked || undefined })} /> {t('chart.panel.stacked')}</label>
		{/if}
	</section>

	<section>
		<h3>{t('chart.panel.data')}</h3>
		{#if isMap}
		<div class="line">{t('chart.panel.coords.label')}
			<div class="seg" role="radiogroup" aria-label={t('chart.panel.coords.label')}>
				<button role="radio" aria-checked={map.coords === 'pair'} class:on={map.coords === 'pair'} title={t('chart.panel.coords.pair.tooltip')} onclick={() => setCoords('pair')}>{t('chart.panel.coords.pair.label')}</button>
				<button role="radio" aria-checked={map.coords === 'single'} class:on={map.coords === 'single'} title={t('chart.panel.coords.single.tooltip')} onclick={() => setCoords('single')}>{t('chart.panel.coords.single.label')}</button>
			</div>
		</div>
		{#if map.coords === 'pair'}
			<label class="line">{t('chart.panel.latitude')}
				<Select value={map.cols[0] ?? none} options={colOptions} onchange={(c) => setMapCol(0, c)} />
			</label>
			<label class="line">{t('chart.panel.longitude')}
				<Select value={map.cols[1] ?? none} options={colOptions} onchange={(c) => setMapCol(1, c)} />
			</label>
			<p class="hint">{t('chart.panel.coords.pairHelp')}</p>
		{:else}
			<label class="line">{t('chart.panel.position')}
				<Select value={map.cols[0] ?? none} options={colOptions} onchange={(c) => setMapCol(0, c)} />
			</label>
			<p class="hint">{t('chart.panel.coords.singleHelp')}</p>
		{/if}
		{#if map.draft}
			<div class="draft">
				<p class="hint">{mapComplete(map) ? t('chart.panel.draft.proposed') : t('chart.panel.draft.choose')}</p>
				<button class="go" disabled={!mapComplete(map)} onclick={() => setMap({}, true)}>{t('chart.panel.draft.show')}</button>
			</div>
		{:else if invalid > 0}
			<p class="warn">{t('chart.panel.invalid', { n: invalid })}</p>
		{/if}
		<label class="line">{t('chart.panel.name')}
			<Select value={map.name} options={[{ value: none, label: t('chart.panel.noneM') }, ...columns.map((c) => ({ value: c, label: c }))]} onchange={(name) => setMap({ name }, false)} />
		</label>
		<label class="line">{t('chart.panel.value')}
			<Select value={map.value} options={[{ value: none, label: t('chart.panel.noneF') }, ...columns.map((c) => ({ value: c, label: c }))]} onchange={(value) => setMap({ value }, false)} />
		</label>
		<label class="line">{t('chart.panel.display')}
			<Select
				value={config.mapMode || 'points'}
				options={[
					{ value: 'points', label: t('chart.panel.points') },
					{ value: 'heat', label: t('chart.panel.heat') }
				]}
				onchange={(v) => set({ mapMode: v === 'points' ? undefined : v })}
			/>
		</label>
		{#if config.mapMode === 'heat'}
			<label class="line" title={t('chart.panel.colorBy.heatTooltip')}>{t('chart.panel.colorBy.label')}
				<Select value={none} options={[{ value: none, label: t('chart.panel.colorBy.pointsOnly') }]} disabled onchange={() => {}} />
			</label>
		{:else}
			<label class="line" title={t('chart.panel.colorBy.tooltip')}>{t('chart.panel.colorBy.label')}
				<Select value={config.colorBy ?? none} options={[{ value: none, label: t('chart.panel.colorBy.single') }, ...columns.map((c) => ({ value: c, label: c }))]} onchange={(c) => set({ colorBy: c || undefined })} />
			</label>
			{#if config.colorBy}
				<label class="opt"><input type="checkbox" checked={config.legend !== 'none'} onchange={(e) => set({ legend: (e.currentTarget as HTMLInputElement).checked ? undefined : 'none' })} /> {t('chart.panel.colorLegend')}</label>
			{/if}
			<label class="line" title={t('chart.panel.pointSize.tooltip')}>{t('chart.panel.pointSize.label')}
				<span class="ptscale">
					<input type="range" min="0.25" max="4" step="0.25" value={config.mapScale ?? 1} onchange={(e) => { const v = Number((e.currentTarget as HTMLInputElement).value); set({ mapScale: v === 1 ? undefined : v }); }} />
					<span>×{fmtNumber(config.mapScale ?? 1)}</span>
				</span>
			</label>
		{/if}
		<label class="opt" title={t('chart.panel.hidden.mapTooltip')}><input type="checkbox" checked={!!config.hidden} onchange={(e) => set({ hidden: (e.currentTarget as HTMLInputElement).checked || undefined })} /> {t('chart.panel.hidden.label')}</label>
		<div class="line mapview">
			<span>{t('chart.panel.view.label')}</span>
			{#if config.mapView}
				<span class="vv">{t('chart.panel.view.saved', { zoom: fmtNumber(config.mapView.zoom, { maximumFractionDigits: 1 }) })}</span>
				<button type="button" title={t('chart.panel.view.recenterTooltip')} onclick={() => set({ mapView: undefined })}>{t('chart.panel.view.recenter')}</button>
			{:else}
				<span class="vv">{t('chart.panel.view.fitted')}</span>
			{/if}
		</div>
		<p class="hint">{t('chart.panel.mapHelp')}</p>
		<p class="hint">{t('chart.panel.mapTiles')}</p>
		{:else}
		{#if sources.length > 1}
			<label class="line" title={t('chart.panel.source.tooltip')}>{t('chart.panel.source.label')}
				<Select value={config.source} options={sources.map((s) => ({ value: s.name, label: s.pivot ? t('chart.panel.source.pivot', { name: s.name }) : s.name }))} onchange={setSource} />
			</label>
		{/if}
		{#each filters as f, i (f.field)}
			<div class="line filter" title={t('chart.panel.filter.tooltip', { field: f.field, values: (f.values ?? []).join(t('chart.panel.filter.or')) })}>
				<span>{t('chart.panel.filter.label')}</span>
				<span class="fv">{f.field} = {(f.values ?? []).join(', ')} <button type="button" title={t('chart.panel.filter.remove')} disabled={!!locked} onclick={() => dropFilter(i)}>✕</button></span>
			</div>
		{/each}
		<label class="line">{t('chart.panel.axisX')}
			<Select value={config.x} options={columns.map((c) => ({ value: c, label: c }))} onchange={(x) => set({ x, ...reorderSeries(config, config.series.filter((s) => s !== x)) })} />
		</label>
		<div class="series">
			{#each listed as c (c)}
				{@const k = config.series.indexOf(c)}
				<label>
					<input
						type={SINGLE_SERIES.has(config.kind) ? 'radio' : 'checkbox'}
						checked={k >= 0}
						title={SINGLE_SERIES.has(config.kind) ? t('chart.panel.singleSeries') : undefined}
						onchange={(e) => toggleSeries(c, (e.currentTarget as HTMLInputElement).checked)}
					/>
					<span>{c}</span>
					{#if k >= 0 && config.series.length > 1}
						<span class="order">
							<button type="button" title={t('chart.panel.series.up.tooltip')} aria-label={t('chart.panel.series.up.aria', { name: c })} disabled={k === 0} onclick={(e) => (e.preventDefault(), moveSeries(k, -1))}>▲</button>
							<button type="button" title={t('chart.panel.series.down.tooltip')} aria-label={t('chart.panel.series.down.aria', { name: c })} disabled={k === config.series.length - 1} onclick={(e) => (e.preventDefault(), moveSeries(k, 1))}>▼</button>
						</span>
					{/if}
					{#if k >= 0 && !byCategory && config.kind !== 'waterfall'}
						<input
							type="color"
							title={t('chart.panel.series.color')}
							value={config.colors?.[k] || PALETTE[k % PALETTE.length]}
							onchange={(e) => {
								const colors = config.series.map((_, j) => config.colors?.[j] || PALETTE[j % PALETTE.length]);
								colors[k] = (e.currentTarget as HTMLInputElement).value;
								set({ colors });
							}}
						/>
						{#if config.kind === 'combo'}
							<Select value={seriesShape(config.seriesKinds, k)} options={SHAPES} onchange={(v) => setShape(k, v)} title={t('chart.panel.series.shape')} />
						{/if}
						{#if canSecondary && config.series.length > 1}
							<button
								class="sec"
								class:on={!!config.secondary?.[k]}
								title={config.secondary?.[k] ? t('chart.panel.series.secondaryOn') : t('chart.panel.series.secondaryOff')}
								onclick={(e) => {
									e.preventDefault();
									toggleSecondary(k);
								}}>{t('chart.panel.series.axis2')}</button
							>
						{/if}
					{/if}
				</label>
			{/each}
		</div>
		<label class="opt" title={t('chart.panel.hidden.tooltip')}><input type="checkbox" checked={!!config.hidden} onchange={(e) => set({ hidden: (e.currentTarget as HTMLInputElement).checked || undefined })} /> {t('chart.panel.hidden.label')}</label>
		{#if config.kind === 'histogram'}
			<label class="line">{t('chart.panel.bins')}<input value={config.bins ?? ''} onchange={(e) => set({ bins: num((e.currentTarget as HTMLInputElement).value) })} placeholder="auto" /></label>
		{/if}
		{#if config.kind === 'line' || config.kind === 'area' || config.kind === 'combo'}
			<label class="line" title={t('chart.panel.blanks.tooltip')}>{t('chart.panel.blanks.label')}
				<Select
					value={config.blanks || 'gap'}
					options={[
						{ value: 'gap', label: t('chart.panel.blanks.gap') },
						{ value: 'zero', label: t('chart.panel.blanks.zero') },
						{ value: 'span', label: t('chart.panel.blanks.span') }
					]}
					onchange={(v) => set({ blanks: v === 'gap' ? undefined : v })}
				/>
			</label>
		{/if}
		{#each notes as n (n)}<p class="warn">{n}</p>{/each}
		{/if}
	</section>

	{#if byCategory && catLabels.length}
		<section>
			<h3>{t('chart.panel.colors')}</h3>
			<div class="colors">
				{#each catLabels as l, i (i)}
					<label title={l}><input type="color" value={config.colors?.[i] || PALETTE[i % PALETTE.length]} onchange={(e) => setColorAt(i, (e.currentTarget as HTMLInputElement).value, catLabels.length)} /><span>{l || t('remote.share.conflict.empty')}</span></label>
				{/each}
			</div>
			{#if (info?.labels?.length ?? 0) > catLabels.length}<p class="hint">{t('chart.panel.paletteRepeat')}</p>{/if}
		</section>
	{:else if config.kind === 'waterfall'}
		<section>
			<h3>{t('chart.panel.colors')}</h3>
			<div class="colors">
				{#each [t('chart.waterfall.up'), t('chart.waterfall.down'), t('chart.waterfall.total')] as l, i (i)}
					<label><input type="color" value={config.colors?.[i] || PALETTE[i % PALETTE.length]} onchange={(e) => setColorAt(i, (e.currentTarget as HTMLInputElement).value, 3)} /><span>{l}</span></label>
				{/each}
			</div>
		</section>
	{/if}

	<section>
		<h3>{t('chart.panel.titles')}</h3>
		<label class="line">{t('chart.chart')}<input value={config.title ?? ''} onchange={(e) => set({ title: (e.currentTarget as HTMLInputElement).value || undefined })} placeholder={t('chart.panel.noneM')} /></label>
		{#if axes}
			<label class="line">{t('chart.panel.axisX')}<input value={config.xTitle ?? ''} onchange={(e) => set({ xTitle: (e.currentTarget as HTMLInputElement).value || undefined })} placeholder={t('chart.panel.noneM')} /></label>
			<label class="line">{t('chart.panel.axisY')}<input value={config.yTitle ?? ''} onchange={(e) => set({ yTitle: (e.currentTarget as HTMLInputElement).value || undefined })} placeholder={t('chart.panel.noneM')} /></label>
			{#if hasSecondary}
				<label class="line">{t('chart.panel.axisY2')}<input value={config.y2Title ?? ''} onchange={(e) => set({ y2Title: (e.currentTarget as HTMLInputElement).value || undefined })} placeholder={t('chart.panel.noneM')} /></label>
			{/if}
		{/if}
	</section>

	<section>
		<h3>{t('chart.panel.elements')}</h3>
		{#if !isMap && config.kind !== 'gauge'}
		<label class="line">{t('chart.panel.legend.label')}
			<Select
				value={config.legend || 'bottom'}
				options={[
					{ value: 'bottom', label: t('chart.panel.legend.bottom') },
					{ value: 'top', label: t('chart.panel.legend.top') },
					{ value: 'right', label: t('chart.panel.legend.right') },
					{ value: 'none', label: t('chart.panel.legend.none') }
				]}
				onchange={(v) => set({ legend: v === 'bottom' ? undefined : v })}
			/>
		</label>
		{/if}
		<label class="opt"><input type="checkbox" checked={!!config.labels} onchange={(e) => set({ labels: (e.currentTarget as HTMLInputElement).checked || undefined })} /> {t('chart.panel.dataLabels')}</label>
		{#if axes || config.kind === 'radar'}
			<label class="opt"><input type="checkbox" checked={!config.noGrid} onchange={(e) => set({ noGrid: !(e.currentTarget as HTMLInputElement).checked || undefined })} /> {t('chart.panel.grid')}</label>
		{/if}
		{#if bounds}
			<div class="minmax">
				<label>Min<input value={config.yMin ?? ''} onchange={(e) => set({ yMin: num((e.currentTarget as HTMLInputElement).value) })} placeholder="auto" /></label>
				<label>Max<input value={config.yMax ?? ''} onchange={(e) => set({ yMax: num((e.currentTarget as HTMLInputElement).value) })} placeholder="auto" /></label>
			</div>
		{/if}
	</section>
	</fieldset>
	<p class="hint">{t('chart.panel.moveHelp')}{#if onSheet} {t('chart.panel.keyboardHelp')}{/if}</p>
</aside>

<style>
	.panel {
		flex: none;
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 8px 10px;
		border-left: 1px solid #d4d7db;
		background: #fafbfb;
		overflow: auto;
		font-size: 12px;
	}
	/* Feuille protégée : le volet se lit, ses champs sont grisés. */
	fieldset.body {
		display: contents;
	}
	fieldset.body:disabled section {
		opacity: 0.6;
	}
	.locked {
		display: flex;
		gap: 6px;
		align-items: flex-start;
		margin: 0;
		padding: 6px 8px;
		background: #fff8e1;
		border: 1px solid #f0d58a;
		border-radius: 3px;
		color: #5c4400;
	}
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
	.kinds {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 4px;
	}
	.kinds button {
		display: flex;
		align-items: center;
		gap: 6px;
		text-align: left;
		padding: 4px 6px;
		--icon-accent: #217346;
	}
	.kinds .on {
		border-color: #217346;
		background: #e3efe7;
		font-weight: 600;
	}
	.ptscale {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.ptscale input {
		flex: 1;
		min-width: 0;
	}
	.line {
		display: grid;
		grid-template-columns: 80px 1fr;
		align-items: center;
		gap: 6px;
		margin-bottom: 4px;
	}
	.series label {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 2px 0;
	}
	.series span {
		flex: 1;
	}
	.sec {
		font: inherit;
		font-size: 11px;
		padding: 0 6px;
		height: 18px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 2px;
		color: #6b7280;
		cursor: pointer;
	}
	.sec.on {
		background: #e3efe7;
		border-color: #217346;
		color: #185c37;
		font-weight: 600;
	}
	input[type='color'] {
		width: 26px;
		height: 18px;
		padding: 0;
		border: 1px solid #c9cdd2;
	}
	.opt {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-top: 4px;
	}
	.minmax {
		display: flex;
		gap: 8px;
		margin-top: 6px;
	}
	.minmax label {
		display: flex;
		align-items: center;
		gap: 4px;
	}
	.minmax input {
		width: 70px;
	}
	input:not([type]) {
		font: inherit;
		padding: 2px 4px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		min-width: 0;
	}
	button {
		font: inherit;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.hint {
		margin: 0;
		color: #6b7280;
	}
	.mapview {
		grid-template-columns: 80px 1fr auto;
	}
	.mapview .vv {
		color: #374151;
	}
	.mapview button {
		padding: 2px 8px;
		border: 1px solid #c4c8cd;
		border-radius: 4px;
		background: #fff;
		font: inherit;
		cursor: pointer;
	}
	.mapview button:hover {
		border-color: #217346;
		color: #217346;
	}
	.seg {
		display: flex;
	}
	.seg button {
		flex: 1;
		padding: 2px 6px;
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
	.hint + .line,
	.line + .hint {
		margin-bottom: 6px;
	}
	.filter .fv {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.filter button {
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
		padding: 0 5px;
	}
	.draft {
		display: flex;
		flex-direction: column;
		gap: 6px;
		margin: 6px 0;
		padding: 6px 8px;
		border: 1px solid #c6dccd;
		background: #f1f7f3;
		border-radius: 3px;
	}
	.go {
		align-self: flex-start;
		padding: 3px 10px;
		border-color: #217346;
		background: #217346;
		color: #fff;
		font-weight: 600;
	}
	.go:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.warn {
		margin: 4px 0 6px;
		color: #9a3412;
	}
	.series .order {
		display: inline-flex;
		flex: none;
	}
	.order button {
		font-size: 9px;
		line-height: 1;
		padding: 2px 3px;
		border-color: transparent;
		background: none;
		color: #6b7280;
	}
	.order button:hover:not(:disabled) {
		color: #217346;
	}
	.order button:disabled {
		opacity: 0.3;
		cursor: default;
	}
	.colors {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 2px 8px;
	}
	.colors label {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
	}
	.colors span {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
