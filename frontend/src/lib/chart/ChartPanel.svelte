<script lang="ts">
	import { resizePanel } from '#lib/ui/resize.js';
	import { panelKeys } from '#lib/ui/focus.js';
	import HelpLink from '#lib/help/HelpLink.svelte';
	// Volet « Mise en forme du graphique » : chaque réglage s'applique tout de
	// suite (aperçu direct, Ctrl+Z pour revenir en arrière).
	import type { ChartConfig } from '#lib/grid/Sheet.svelte';
	import { PALETTE } from './scale';
	import Select from '#lib/ui/Select.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { CARTESIAN, KINDS, SECONDARY, STACKABLE, mapChoice, mapComplete, mapPatch, seriesShape, type MapChoice, type MapGuess } from './kinds';

	interface Props {
		/** Bord gauche à tirer (faux : inclus dans un volet qui se redimensionne lui-même). */
		resizable?: boolean;
		config: ChartConfig;
		columns: string[];
		/** Carte : colonnes de coordonnées repérées par le classeur (proposées en attente et au changement de mode). */
		guess?: MapGuess | null;
		/** Carte : coordonnées illisibles ou hors bornes, ignorées. */
		invalid?: number;
		onchange: (cfg: ChartConfig) => void;
		onclose: () => void;
		/** Feuille protégée : le message, et les champs grisés. */
		locked?: string;
	}

	let { config, columns, guess = null, invalid = 0, onchange, onclose, resizable = true, locked = '' }: Props = $props();

	// Le titre et les champs texte s'appliquent à la sortie du champ.
	function set(patch: Partial<ChartConfig>) {
		if (locked) return;
		onchange({ ...config, ...patch });
	}
	function num(s: string): number | undefined {
		const t = s.trim().replace(',', '.');
		return t === '' || isNaN(Number(t)) ? undefined : Number(t);
	}
	const stackable = $derived(STACKABLE.has(config.kind));
	const axes = $derived(CARTESIAN.has(config.kind));
	const isMap = $derived(config.kind === 'map');
	// Bornes : axes, radar et jauge (minimum et maximum du cadran).
	const bounds = $derived(axes || config.kind === 'radar' || config.kind === 'gauge');
	// Axe secondaire (Excel) : histogramme, courbes, aires, nuage, combiné.
	const canSecondary = $derived(SECONDARY.has(config.kind));
	const SHAPES = [
		{ value: 'bar', label: 'Barres' },
		{ value: 'line', label: 'Courbe' },
		{ value: 'area', label: 'Aire' }
	];
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
	const colOptions = $derived([{ value: none, label: '(choisir)' }, ...columns.map((c) => ({ value: c, label: c }))]);
	const hasSecondary = $derived(canSecondary && config.series.some((_, k) => config.secondary?.[k]));
	function toggleSecondary(k: number) {
		const secondary = config.series.map((_, j) => !!config.secondary?.[j]);
		secondary[k] = !secondary[k];
		set({ secondary: secondary.some(Boolean) ? secondary : undefined });
	}
</script>

<aside class="panel" use:resizePanel={{ key: 'chart', off: !resizable }} use:panelKeys={onclose} aria-label="Graphique">
	<header>
		<strong>Graphique</strong>
		<HelpLink target="graphiques" label="Aide sur les graphiques" />
		<button class="x" title="Fermer le volet (Échap)" aria-label="Fermer le volet" onclick={onclose}>✕</button>
	</header>
	{#if locked}<p class="locked" role="note"><Icon name="lock-sheet" size={14} /><span>{locked}</span></p>{/if}

	<fieldset class="body" disabled={!!locked}>
	<section>
		<h3>Type</h3>
		<div class="kinds">
			{#each KINDS as k (k.kind)}
				<button class:on={config.kind === k.kind} title={k.hint} onclick={() => setKind(k.kind)}><Icon name={k.icon} size={16} />{k.label}</button>
			{/each}
		</div>
		{#if stackable}
			<label class="opt"><input type="checkbox" checked={!!config.stacked} onchange={(e) => set({ stacked: (e.currentTarget as HTMLInputElement).checked || undefined })} /> Empilé</label>
		{/if}
	</section>

	<section>
		<h3>Données</h3>
		{#if isMap}
		<div class="line">Coordonnées
			<div class="seg" role="radiogroup" aria-label="Coordonnées">
				<button role="radio" aria-checked={map.coords === 'pair'} class:on={map.coords === 'pair'} title="Une colonne de latitudes, une colonne de longitudes" onclick={() => setCoords('pair')}>2 colonnes</button>
				<button role="radio" aria-checked={map.coords === 'single'} class:on={map.coords === 'single'} title="Latitude et longitude dans la même cellule, écrites de toute façon standard" onclick={() => setCoords('single')}>1 colonne</button>
			</div>
		</div>
		{#if map.coords === 'pair'}
			<label class="line">Latitude
				<Select value={map.cols[0] ?? none} options={colOptions} onchange={(c) => setMapCol(0, c)} />
			</label>
			<label class="line">Longitude
				<Select value={map.cols[1] ?? none} options={colOptions} onchange={(c) => setMapCol(1, c)} />
			</label>
			<p class="hint">Nombres décimaux (48,8566 ou 48.8566), ou degrés 48°51'24"N.</p>
		{:else}
			<label class="line">Position
				<Select value={map.cols[0] ?? none} options={colOptions} onchange={(c) => setMapCol(0, c)} />
			</label>
			<p class="hint">Toute écriture standard : 48.8566, 2.3522 · 48,8566;2,3522 · 48°51'24"N 2°21'03"E · 48°51.4'N 2°21.05'E · N48.85 E2.35 · POINT(2.3522 48.8566) · geo:48.8566,2.3522</p>
		{/if}
		{#if map.draft}
			<div class="draft">
				<p class="hint">{mapComplete(map) ? 'Colonnes proposées : vérifiez-les, puis affichez la carte.' : 'Choisissez les colonnes de coordonnées : la carte se dessine ensuite.'}</p>
				<button class="go" disabled={!mapComplete(map)} onclick={() => setMap({}, true)}>Afficher la carte</button>
			</div>
		{:else if invalid > 0}
			<p class="warn">{invalid === 1 ? '1 valeur illisible ignorée' : `${invalid} valeurs illisibles ignorées`}</p>
		{/if}
		<label class="line">Nom
			<Select value={map.name} options={[{ value: none, label: '(aucun)' }, ...columns.map((c) => ({ value: c, label: c }))]} onchange={(name) => setMap({ name }, false)} />
		</label>
		<label class="line">Valeur
			<Select value={map.value} options={[{ value: none, label: '(aucune)' }, ...columns.map((c) => ({ value: c, label: c }))]} onchange={(value) => setMap({ value }, false)} />
		</label>
		<label class="line">Affichage
			<Select
				value={config.mapMode || 'points'}
				options={[
					{ value: 'points', label: 'Points' },
					{ value: 'heat', label: 'Densité (chaleur)' }
				]}
				onchange={(v) => set({ mapMode: v === 'points' ? undefined : v })}
			/>
		</label>
		{#if config.mapMode === 'heat'}
			<label class="line" title="La densité est une seule nappe de chaleur : passez en « Points » pour colorer par valeur">Couleur par
				<Select value={none} options={[{ value: none, label: '(points seulement)' }]} disabled onchange={() => {}} />
			</label>
		{:else}
			<label class="line" title="Chaque valeur distincte de la colonne prend sa couleur, réparties sur tout le spectre">Couleur par
				<Select value={config.colorBy ?? none} options={[{ value: none, label: '(une seule couleur)' }, ...columns.map((c) => ({ value: c, label: c }))]} onchange={(c) => set({ colorBy: c || undefined })} />
			</label>
			{#if config.colorBy}
				<label class="opt"><input type="checkbox" checked={config.legend !== 'none'} onchange={(e) => set({ legend: (e.currentTarget as HTMLInputElement).checked ? undefined : 'none' })} /> Légende des couleurs</label>
			{/if}
		{/if}
		<div class="line mapview">
			<span>Vue</span>
			{#if config.mapView}
				<span class="vv">Enregistrée (zoom {config.mapView.zoom.toLocaleString('fr-FR', { maximumFractionDigits: 1 })})</span>
				<button type="button" title="Revenir à la vue ajustée aux points" onclick={() => set({ mapView: undefined })}>Recentrer</button>
			{:else}
				<span class="vv">Ajustée aux points</span>
			{/if}
		</div>
		<p class="hint">Carte sélectionnée : la molette zoome, glisser déplace la carte (Alt+glisser déplace le graphique), double-clic ou + / − zooment ; un clic sur un point va à sa ligne dans le tableau.</p>
		<p class="hint">Fond de carte © OpenStreetMap (tile.openstreetmap.org), soumis à l’autorisation réseau du classeur. La valeur règle la taille des points, ou le poids de chacun dans la densité.</p>
		{:else}
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
						{#if config.kind === 'combo'}
							<Select value={seriesShape(config.seriesKinds, k)} options={SHAPES} onchange={(v) => setShape(k, v)} title="Forme de la série" />
						{/if}
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
		{#if config.kind === 'histogram'}
			<label class="line">Classes<input value={config.bins ?? ''} onchange={(e) => set({ bins: num((e.currentTarget as HTMLInputElement).value) })} placeholder="auto" /></label>
		{/if}
		{/if}
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
		{#if !isMap && config.kind !== 'gauge'}
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
		{/if}
		<label class="opt"><input type="checkbox" checked={!!config.labels} onchange={(e) => set({ labels: (e.currentTarget as HTMLInputElement).checked || undefined })} /> Étiquettes de données</label>
		{#if axes || config.kind === 'radar'}
			<label class="opt"><input type="checkbox" checked={!config.noGrid} onchange={(e) => set({ noGrid: !(e.currentTarget as HTMLInputElement).checked || undefined })} /> Quadrillage</label>
		{/if}
		{#if bounds}
			<div class="minmax">
				<label>Min<input value={config.yMin ?? ''} onchange={(e) => set({ yMin: num((e.currentTarget as HTMLInputElement).value) })} placeholder="auto" /></label>
				<label>Max<input value={config.yMax ?? ''} onchange={(e) => set({ yMax: num((e.currentTarget as HTMLInputElement).value) })} placeholder="auto" /></label>
			</div>
		{/if}
	</section>
	</fieldset>
	<p class="hint">Glissez le graphique pour le déplacer, ses poignées pour le redimensionner. Clic droit : couper, copier, supprimer.</p>
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
</style>
