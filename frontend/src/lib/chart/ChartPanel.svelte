<script lang="ts">
	import { resizePanel } from '#lib/ui/resize.js';
	// Volet « Mise en forme du graphique » : chaque réglage s'applique tout de
	// suite (aperçu direct, Ctrl+Z pour revenir en arrière).
	import type { ChartConfig } from '#lib/grid/Sheet.svelte';
	import { PALETTE } from './scale';
	import Select from '#lib/ui/Select.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import type { IconName } from '#lib/icons/icons.js';

	interface Props {
		config: ChartConfig;
		columns: string[];
		onchange: (cfg: ChartConfig) => void;
		onclose: () => void;
	}

	let { config, columns, onchange, onclose }: Props = $props();

	const KINDS: [ChartConfig['kind'], string, IconName][] = [
		['bar', 'Histogramme', 'chart-bar'],
		['column', 'Barres horizontales', 'chart-column'],
		['line', 'Courbes', 'chart-line'],
		['area', 'Aires', 'chart-area'],
		['pie', 'Secteurs', 'chart-pie'],
		['doughnut', 'Anneau', 'chart-doughnut'],
		['scatter', 'Nuage de points', 'chart-scatter']
	];

	// Le titre et les champs texte s'appliquent à la sortie du champ.
	function set(patch: Partial<ChartConfig>) {
		onchange({ ...config, ...patch });
	}
	function num(s: string): number | undefined {
		const t = s.trim().replace(',', '.');
		return t === '' || isNaN(Number(t)) ? undefined : Number(t);
	}
	const stackable = $derived(config.kind === 'bar' || config.kind === 'column' || config.kind === 'area');
	const axes = $derived(config.kind !== 'pie' && config.kind !== 'doughnut');
	// Axe secondaire (Excel) : histogramme, courbes, aires, nuage de points.
	const canSecondary = $derived(config.kind === 'bar' || config.kind === 'line' || config.kind === 'area' || config.kind === 'scatter');
	const hasSecondary = $derived(canSecondary && config.series.some((_, k) => config.secondary?.[k]));
	function toggleSecondary(k: number) {
		const secondary = config.series.map((_, j) => !!config.secondary?.[j]);
		secondary[k] = !secondary[k];
		set({ secondary: secondary.some(Boolean) ? secondary : undefined });
	}
</script>

<aside class="panel" use:resizePanel={{ key: 'chart' }}>
	<header>
		<strong>Graphique</strong>
		<button class="x" title="Fermer le volet" onclick={onclose}>✕</button>
	</header>

	<section>
		<h3>Type</h3>
		<div class="kinds">
			{#each KINDS as [k, label, icon] (k)}
				<button class:on={config.kind === k} onclick={() => set({ kind: k })}><Icon name={icon} size={16} />{label}</button>
			{/each}
		</div>
		{#if stackable}
			<label class="opt"><input type="checkbox" checked={!!config.stacked} onchange={(e) => set({ stacked: (e.currentTarget as HTMLInputElement).checked || undefined })} /> Empilé</label>
		{/if}
	</section>

	<section>
		<h3>Données</h3>
		<label class="line">Axe X
			<Select value={config.x} options={columns.map((c) => ({ value: c, label: c }))} onchange={(x) => set({ x, series: config.series.filter((s) => s !== x) })} />
		</label>
		<div class="series">
			{#each columns.filter((c) => c !== config.x) as c (c)}
				{@const k = config.series.indexOf(c)}
				<label>
					<input
						type="checkbox"
						checked={k >= 0}
						onchange={(e) => {
							const on = (e.currentTarget as HTMLInputElement).checked;
							const series = on ? [...config.series, c] : config.series.filter((x) => x !== c);
							if (series.length) set({ series, colors: undefined, secondary: undefined });
						}}
					/>
					<span>{c}</span>
					{#if k >= 0}
						<input
							type="color"
							title="Couleur de la série"
							value={config.colors?.[k] || PALETTE[k % PALETTE.length]}
							onchange={(e) => {
								const colors = config.series.map((_, j) => config.colors?.[j] || PALETTE[j % PALETTE.length]);
								colors[k] = (e.currentTarget as HTMLInputElement).value;
								set({ colors });
							}}
						/>
						{#if canSecondary && config.series.length > 1}
							<button
								class="sec"
								class:on={!!config.secondary?.[k]}
								title={config.secondary?.[k] ? 'Série sur l’axe secondaire (à droite) : cliquer pour la remettre sur l’axe principal' : 'Tracer la série sur un axe secondaire, à droite, avec sa propre échelle'}
								onclick={(e) => {
									e.preventDefault();
									toggleSecondary(k);
								}}>Axe 2</button
							>
						{/if}
					{/if}
				</label>
			{/each}
		</div>
	</section>

	<section>
		<h3>Titres</h3>
		<label class="line">Graphique<input value={config.title ?? ''} onchange={(e) => set({ title: (e.currentTarget as HTMLInputElement).value || undefined })} placeholder="(aucun)" /></label>
		{#if axes}
			<label class="line">Axe X<input value={config.xTitle ?? ''} onchange={(e) => set({ xTitle: (e.currentTarget as HTMLInputElement).value || undefined })} placeholder="(aucun)" /></label>
			<label class="line">Axe Y<input value={config.yTitle ?? ''} onchange={(e) => set({ yTitle: (e.currentTarget as HTMLInputElement).value || undefined })} placeholder="(aucun)" /></label>
			{#if hasSecondary}
				<label class="line">Axe Y 2<input value={config.y2Title ?? ''} onchange={(e) => set({ y2Title: (e.currentTarget as HTMLInputElement).value || undefined })} placeholder="(aucun)" /></label>
			{/if}
		{/if}
	</section>

	<section>
		<h3>Éléments</h3>
		<label class="line">Légende
			<Select
				value={config.legend || 'bottom'}
				options={[
					{ value: 'bottom', label: 'En bas' },
					{ value: 'top', label: 'En haut' },
					{ value: 'right', label: 'À droite' },
					{ value: 'none', label: 'Aucune' }
				]}
				onchange={(v) => set({ legend: v === 'bottom' ? undefined : v })}
			/>
		</label>
		<label class="opt"><input type="checkbox" checked={!!config.labels} onchange={(e) => set({ labels: (e.currentTarget as HTMLInputElement).checked || undefined })} /> Étiquettes de données</label>
		{#if axes}
			<label class="opt"><input type="checkbox" checked={!config.noGrid} onchange={(e) => set({ noGrid: !(e.currentTarget as HTMLInputElement).checked || undefined })} /> Quadrillage</label>
			<div class="minmax">
				<label>Min<input value={config.yMin ?? ''} onchange={(e) => set({ yMin: num((e.currentTarget as HTMLInputElement).value) })} placeholder="auto" /></label>
				<label>Max<input value={config.yMax ?? ''} onchange={(e) => set({ yMax: num((e.currentTarget as HTMLInputElement).value) })} placeholder="auto" /></label>
			</div>
		{/if}
	</section>
	<p class="hint">Glissez le graphique pour le déplacer, ses poignées pour le redimensionner. Clic droit : couper, copier, supprimer.</p>
</aside>

<style>
	.panel {
		width: 280px;
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
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		font-size: 13px;
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
	.x {
		border: none;
		background: none;
	}
	.hint {
		margin: 0;
		color: #6b7280;
	}
</style>
