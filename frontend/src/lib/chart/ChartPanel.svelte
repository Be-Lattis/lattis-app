<script lang="ts">
	// Volet « Mise en forme du graphique » : chaque réglage s'applique tout de
	// suite (aperçu direct, Ctrl+Z pour revenir en arrière).
	import type { ChartConfig } from '#lib/grid/Sheet.svelte';
	import { PALETTE } from './scale';

	interface Props {
		config: ChartConfig;
		columns: string[];
		onchange: (cfg: ChartConfig) => void;
		onclose: () => void;
	}

	let { config, columns, onchange, onclose }: Props = $props();

	const KINDS: [ChartConfig['kind'], string][] = [
		['bar', 'Histogramme'],
		['column', 'Barres horizontales'],
		['line', 'Courbes'],
		['area', 'Aires'],
		['pie', 'Secteurs'],
		['doughnut', 'Anneau'],
		['scatter', 'Nuage de points']
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
</script>

<aside class="panel">
	<header>
		<strong>Graphique</strong>
		<button class="x" title="Fermer le volet" onclick={onclose}>✕</button>
	</header>

	<section>
		<h3>Type</h3>
		<div class="kinds">
			{#each KINDS as [k, label] (k)}
				<button class:on={config.kind === k} onclick={() => set({ kind: k })}>{label}</button>
			{/each}
		</div>
		{#if stackable}
			<label class="opt"><input type="checkbox" checked={!!config.stacked} onchange={(e) => set({ stacked: (e.currentTarget as HTMLInputElement).checked || undefined })} /> Empilé</label>
		{/if}
	</section>

	<section>
		<h3>Données</h3>
		<label class="line">Axe X
			<select value={config.x} onchange={(e) => set({ x: (e.currentTarget as HTMLSelectElement).value, series: config.series.filter((s) => s !== (e.currentTarget as HTMLSelectElement).value) })}>
				{#each columns as c (c)}<option value={c}>{c}</option>{/each}
			</select>
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
							if (series.length) set({ series, colors: undefined });
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
		{/if}
	</section>

	<section>
		<h3>Éléments</h3>
		<label class="line">Légende
			<select value={config.legend || 'bottom'} onchange={(e) => set({ legend: (e.currentTarget as HTMLSelectElement).value === 'bottom' ? undefined : (e.currentTarget as HTMLSelectElement).value })}>
				<option value="bottom">En bas</option>
				<option value="top">En haut</option>
				<option value="right">À droite</option>
				<option value="none">Aucune</option>
			</select>
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
		text-align: left;
		padding: 4px 6px;
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
	input:not([type]),
	select {
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
