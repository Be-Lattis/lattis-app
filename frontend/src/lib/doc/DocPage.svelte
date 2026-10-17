<script lang="ts">
	// Onglet « Document » : une zone de texte riche sans fin (Quill) où
	// l'on écrit, met en forme (ruban : Police, Paragraphe…) et insère des
	// graphiques et tableaux croisés alimentés par les tableaux du classeur, et
	// des boutons qui exécutent des scripts. Clic droit : sur un élément inséré,
	// le même menu que sur la grille ; ailleurs, un menu pour insérer. Le contenu (Delta Quill en JSON) s'enregistre un
	// instant après la dernière frappe, en une étape d'annulation du classeur ;
	// Ctrl+Z dans le texte annule la frappe elle-même.
	import 'quill/dist/quill.core.css';
	import { mount, onMount, unmount, untrack } from 'svelte';
	import type Quill from 'quill';
	import type { ChartInfo, PivotInfo } from '#bindings/internal/canvas/models.js';
	import type { ChartConfig, PivotConfig } from '#lib/grid/Sheet.svelte';
	import ChartPanel from '#lib/chart/ChartPanel.svelte';
	import PivotPanel from '#lib/grid/PivotPanel.svelte';
	import { dismiss } from '#lib/grid/dismiss.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { scriptFunctions } from '#lib/grid/buttons.js';
	import { BUTTON_BLOT, CHART_BLOT, PIVOT_BLOT, docClip, setupQuill, type DocButtonConfig } from './quill';
	import DocChart from './DocChart.svelte';
	import DocPivot from './DocPivot.svelte';
	import { blockFormats, docFormat, DOC_FONT, DOC_PT, parseDelta, quillFormats, type DocBlock, type DocFormat } from './format';

	export interface DocTable {
		name: string;
		columns: string[];
		types: string[];
		/** 'pivot' : tableau croisé dynamique. */
		kind?: string;
	}

	interface Props {
		name: string;
		/** Contenu enregistré (Delta Quill en JSON). */
		source: string;
		save: (source: string) => Promise<unknown>;
		/** Feuille ou classeur protégé : message, et lecture seule. */
		locked?: string;
		/** Change à chaque modification du classeur : les graphiques se relisent. */
		stamp?: unknown;
		animate?: boolean;
		tables: DocTable[];
		scripts: { name: string; source: string }[];
		loadChart: (cfg: ChartConfig) => Promise<ChartInfo>;
		loadPivot: (cfg: PivotConfig) => Promise<PivotInfo>;
		/** Valeurs d'un champ (filtres du volet de tableau croisé). */
		fieldValues?: (table: string, field: string) => Promise<{ text: string; count: number }[]>;
		/** Exécute un script (ou sa fonction) ; renvoie le message à afficher. */
		run: (script: string, fn: string) => Promise<string>;
		/** Mise en forme à la sélection, pour le ruban. */
		onformat?: (f: DocFormat) => void;
		onnotice?: (msg: string) => void;
	}

	let { name, source, save, locked = '', stamp, animate = true, tables, scripts, loadChart, loadPivot, fieldValues, run, onformat, onnotice }: Props = $props();

	let host = $state<HTMLDivElement>();
	let quill: Quill | undefined;
	let lastSaved = untrack(() => source);
	let timer: ReturnType<typeof setTimeout> | undefined;
	let pending = false;
	let current: DocFormat = docFormat({});
	let lastRange: { index: number; length: number } | null = null;
	const live = $state({ stamp: 0, animate: untrack(() => animate) });

	// Objet en cours de modification dans le volet de droite (par sa position).
	type Panel = { kind: 'chart' | 'button' | 'pivot'; index: number } | { kind: 'link'; index: number; length: number; url: string };
	let panel = $state<Panel | null>(null);
	let panelValue = $state.raw<Record<string, unknown> | null>(null);

	// --- Enregistrement --------------------------------------------------------

	function schedule() {
		pending = true;
		clearTimeout(timer);
		timer = setTimeout(flush, 600);
	}

	async function flush() {
		clearTimeout(timer);
		if (!quill || !pending) return;
		pending = false;
		const src = JSON.stringify(quill.getContents());
		if (src === lastSaved) return;
		lastSaved = src;
		try {
			await save(src);
		} catch (e) {
			onnotice?.(String(e instanceof Error ? e.message : e));
		}
	}

	// Annulation du classeur, script : le contenu enregistré fait foi (hors frappe en attente).
	$effect(() => {
		const s = source;
		untrack(() => {
			if (!quill || pending || s === lastSaved) return;
			lastSaved = s;
			quill.setContents(parseDelta(s) as never, 'silent');
			hydrate();
		});
	});

	$effect(() => {
		void stamp;
		untrack(() => live.stamp++);
	});
	$effect(() => {
		live.animate = animate;
	});
	$effect(() => {
		quill?.enable(!locked);
	});

	// --- Graphiques et tableaux croisés insérés : un composant par bloc --------

	type Mounted = { comp: ReturnType<typeof mount>; kind: 'chart' | 'pivot'; p: { config: Record<string, unknown>; selected: boolean } };
	const mounted = new Map<HTMLElement, Mounted>();
	const BLOT = { chart: CHART_BLOT, pivot: PIVOT_BLOT, button: BUTTON_BLOT } as const;

	function hydrate() {
		if (!quill) return;
		for (const node of quill.root.querySelectorAll<HTMLElement>('.ql-lattis-chart, .ql-lattis-pivot')) {
			if (mounted.has(node)) continue;
			const kind = node.classList.contains('ql-lattis-pivot') ? 'pivot' : 'chart';
			const p = $state({ config: JSON.parse(node.dataset.config ?? '{}') as Record<string, unknown>, selected: false });
			const props = {
				get config() {
					return p.config as never;
				},
				get selected() {
					return p.selected;
				},
				live
			};
			const comp = kind === 'pivot' ? mount(DocPivot, { target: node, props: { ...props, load: loadPivot } }) : mount(DocChart, { target: node, props: { ...props, load: loadChart } });
			mounted.set(node, { comp, kind, p });
		}
		for (const [node, m] of mounted) {
			if (!node.isConnected) {
				unmount(m.comp);
				mounted.delete(node);
			}
		}
		markSelected();
	}

	function indexOf(node: HTMLElement): number {
		const Q = quill!.constructor as typeof Quill;
		const blot = Q.find(node);
		return blot ? quill!.getIndex(blot as never) : -1;
	}

	function markSelected() {
		for (const [node, m] of mounted) m.p.selected = (panel?.kind === m.kind && indexOf(node) === panel.index) || (menu?.index === indexOf(node) && menu.kind === m.kind);
	}

	/** Valeur de l'objet inséré à la position index (null : il n'y en a plus). */
	function embedAt(index: number, type: string): Record<string, unknown> | null {
		const op = quill?.getContents(index, 1).ops[0];
		const ins = op?.insert as Record<string, unknown> | string | undefined;
		return ins && typeof ins === 'object' && type in ins ? (ins[type] as Record<string, unknown>) : null;
	}

	function openPanel(p: Panel | null) {
		panel = p;
		panelValue = p && p.kind !== 'link' ? embedAt(p.index, BLOT[p.kind]) : null;
		markSelected();
	}

	function replaceEmbed(index: number, type: string, value: unknown) {
		if (!quill || locked) return;
		const Delta = (quill.constructor as typeof Quill).import('delta') as unknown as new () => { retain(n: number): unknown };
		const d = new Delta() as { retain(n: number): { delete(n: number): { insert(v: unknown): unknown } } };
		quill.updateContents(d.retain(index).delete(1).insert({ [type]: value }) as never, 'user');
		panelValue = value as Record<string, unknown>;
		hydrate();
	}

	function deleteEmbed(index: number) {
		if (!quill || locked) return;
		quill.deleteText(index, 1, 'user');
		openPanel(null);
		hydrate();
	}

	// --- Commandes du ruban ----------------------------------------------------

	function range() {
		if (!quill) return null;
		// Le ruban prend le focus : la dernière sélection du texte compte.
		return quill.getSelection() ?? lastRange ?? { index: Math.max(0, quill.getLength() - 1), length: 0 };
	}

	function guard(): boolean {
		if (locked) {
			onnotice?.(locked);
			return false;
		}
		return !!quill;
	}

	/** Correctif de mise en forme du ruban (groupes Police et Alignement). */
	export function applyStyle(patch: Record<string, unknown>) {
		if (!guard()) return;
		const r = range()!;
		const f = quillFormats(patch, current.style);
		for (const [k, v] of Object.entries(f)) {
			if (k === 'align') quill!.formatLine(r.index, Math.max(r.length, 1), k, v, 'user');
			else if (r.length) quill!.formatText(r.index, r.length, k, v, 'user');
			else quill!.format(k, v, 'user');
		}
		quill!.focus();
		report();
	}

	export function setBlock(block: DocBlock) {
		if (!guard()) return;
		const r = range()!;
		quill!.formatLine(r.index, Math.max(r.length, 1), blockFormats(block, current.block), 'user');
		quill!.focus();
		report();
	}

	export function setAlign(al: 'left' | 'center' | 'right' | 'justify') {
		if (!guard()) return;
		const r = range()!;
		quill!.formatLine(r.index, Math.max(r.length, 1), 'align', al === 'left' ? false : al, 'user');
		quill!.focus();
		report();
	}

	export function command(cmd: 'undo' | 'redo' | 'strike' | 'link' | 'image' | 'cut' | 'copy' | 'paste' | 'clear') {
		if (!quill) return;
		if (cmd === 'copy') {
			quill.focus();
			document.execCommand('copy');
			return;
		}
		if (!guard()) return;
		const r = range()!;
		switch (cmd) {
			case 'undo':
				quill.history.undo();
				break;
			case 'redo':
				quill.history.redo();
				break;
			case 'strike':
				applyStyle({ x: 'toggle' });
				return;
			case 'clear':
				quill.removeFormat(r.index, r.length, 'user');
				break;
			case 'cut':
				quill.focus();
				document.execCommand('cut');
				return;
			case 'paste':
				navigator.clipboard
					?.readText()
					.then((t) => {
						const at = range()!;
						quill!.deleteText(at.index, at.length, 'user');
						quill!.insertText(at.index, t, 'user');
						quill!.setSelection(at.index + t.length, 0);
					})
					.catch(() => onnotice?.('Coller : utilisez Ctrl+V dans le texte'));
				return;
			case 'link': {
				const f = quill.getFormat(r.index, r.length);
				openPanel({ kind: 'link', index: r.index, length: r.length, url: typeof f.link === 'string' ? f.link : 'https://' });
				return;
			}
			case 'image':
				pickImage(r.index);
				return;
		}
		quill.focus();
		report();
	}

	function pickImage(index: number) {
		const input = document.createElement('input');
		input.type = 'file';
		input.accept = 'image/*';
		input.onchange = () => {
			const file = input.files?.[0];
			if (!file) return;
			const reader = new FileReader();
			reader.onload = () => {
				quill!.insertEmbed(index, 'image', String(reader.result), 'user');
				quill!.setSelection(index + 1, 0);
			};
			reader.readAsDataURL(file);
		};
		input.click();
	}

	/** Insère un graphique du premier tableau (modifiable dans le volet). */
	export function insertChart(kind: string) {
		if (!guard()) return;
		// Un tableau croisé (déjà agrégé) se prête le mieux à un graphique.
		const t = tables.find((x) => x.kind === 'pivot') ?? tables[0];
		if (!t) {
			onnotice?.('Aucun tableau dans le classeur : créez-en un sur une feuille pour le représenter ici.');
			return;
		}
		const at = range()!.index;
		quill!.insertEmbed(at, CHART_BLOT, defaultChart(t, kind), 'user');
		quill!.setSelection(at + 1, 0);
		hydrate();
		openPanel({ kind: 'chart', index: at });
	}

	function defaultChart(t: DocTable, kind: string): ChartConfig {
		const num = (i: number) => /^(int|integer|decimal|real|number)/i.test(t.types[i] ?? '');
		// Axe X : la première colonne de texte (sinon la première) ; séries : les nombres.
		const xi = Math.max(0, t.columns.findIndex((_, i) => !num(i)));
		const numeric = t.columns.filter((_, i) => i !== xi && num(i));
		const series = (numeric.length ? numeric : t.columns.filter((_, i) => i !== xi).slice(0, 1)).slice(0, 3);
		return { source: t.name, kind, x: t.columns[xi] ?? '', series, title: t.name } as ChartConfig;
	}

	/** Insère un tableau croisé d'un tableau du classeur (champs à choisir dans le volet). */
	export async function insertPivot() {
		if (!guard()) return;
		const t = tables.find((x) => x.kind !== 'pivot');
		if (!t) {
			onnotice?.('Aucun tableau dans le classeur : créez-en un sur une feuille pour l’analyser ici.');
			return;
		}
		const at = range()!.index;
		quill!.insertEmbed(at, PIVOT_BLOT, await defaultPivot(t), 'user');
		quill!.setSelection(at + 1, 0);
		hydrate();
		openPanel({ kind: 'pivot', index: at });
	}

	// Champs de départ : en lignes, la première colonne de texte qui regroupe
	// (2 à 50 valeurs distinctes) ; en valeurs, la somme du premier nombre qui
	// n'est pas un identifiant (sinon le nombre de lignes).
	async function defaultPivot(t: DocTable): Promise<PivotConfig> {
		const num = (i: number) => /^(int|integer|decimal|real|number)/i.test(t.types[i] ?? '');
		const idLike = (c: string) => /^(n°|no\b|num|id\b|code|matricule|réf)/i.test(c);
		const texts = t.columns.filter((_, i) => !num(i));
		let row = texts[0];
		if (fieldValues) {
			for (const c of texts) {
				const vals = await fieldValues(t.name, c).catch(() => []);
				if (vals.length >= 2 && vals.length <= 50) {
					row = c;
					break;
				}
			}
		}
		const val = t.columns.find((c, i) => num(i) && !idLike(c));
		return {
			source: t.name,
			rowFields: row ? [row] : [],
			values: val ? [{ field: val, agg: 'sum' }] : [{ field: row ?? t.columns[0], agg: 'count' }]
		} as PivotConfig;
	}

	// --- Menus contextuels et presse-papiers des éléments ---------------------

	type Menu = { x: number; y: number; kind: 'chart' | 'pivot' | 'button' | 'text'; index: number };
	let menu = $state<Menu | null>(null);
	let clip = $state(docClip.item);

	function copyItem(kind: 'chart' | 'pivot' | 'button', index: number, cut: boolean) {
		const value = embedAt(index, BLOT[kind]);
		if (!value) return;
		docClip.item = clip = { type: BLOT[kind], value: structuredClone($state.snapshot(value)) };
		if (cut) deleteEmbed(index);
		onnotice?.(cut ? 'Élément coupé : clic droit › Coller dans le texte' : 'Élément copié : clic droit › Coller dans le texte');
	}

	function pasteItem(at: number) {
		const c = docClip.item;
		if (!c || !guard()) return;
		quill!.insertEmbed(at, c.type, structuredClone(c.value), 'user');
		quill!.setSelection(at + 1, 0);
		hydrate();
	}

	function closeMenu() {
		menu = null;
		markSelected();
	}

	function act(fn: () => void) {
		closeMenu();
		fn();
	}

	export function insertButton() {
		if (!guard()) return;
		const at = range()!.index;
		const cfg: DocButtonConfig = { label: 'Bouton', script: scripts[0]?.name ?? '' };
		quill!.insertEmbed(at, BUTTON_BLOT, cfg, 'user');
		quill!.setSelection(at + 1, 0);
		openPanel({ kind: 'button', index: at });
	}

	export function focus() {
		quill?.focus();
	}

	function report() {
		if (!quill) return;
		const sel = quill.getSelection() ?? lastRange;
		current = docFormat(sel ? quill.getFormat(sel.index, sel.length) : {});
		onformat?.(current);
	}

	// --- Clics dans le texte -----------------------------------------------------

	async function runButton(cfg: DocButtonConfig) {
		if (!cfg.script) {
			onnotice?.('Ce bouton n’exécute rien : clic droit pour lui lier un script.');
			return;
		}
		await flush();
		try {
			const msg = await run(cfg.script, cfg.function ?? '');
			if (msg) onnotice?.(msg);
		} catch (e) {
			onnotice?.(String(e instanceof Error ? e.message : e));
		}
	}

	function onClick(e: MouseEvent) {
		const t = e.target as HTMLElement;
		const b = t.closest<HTMLElement>('.ql-lattis-button');
		if (b) {
			e.preventDefault();
			runButton(JSON.parse(b.dataset.config ?? '{}'));
			return;
		}
		const c = t.closest<HTMLElement>('.ql-lattis-chart, .ql-lattis-pivot');
		if (c) {
			openPanel({ kind: c.classList.contains('ql-lattis-pivot') ? 'pivot' : 'chart', index: indexOf(c) });
			return;
		}
		const a = t.closest<HTMLAnchorElement>('a[href]');
		if (a) {
			e.preventDefault();
			if (e.ctrlKey || e.metaKey) openOutside(a.href);
		}
		if (panel?.kind === 'chart' || panel?.kind === 'pivot') openPanel(null);
	}

	function onContext(e: MouseEvent) {
		e.preventDefault();
		const t = e.target as HTMLElement;
		const el = t.closest<HTMLElement>('.ql-lattis-button, .ql-lattis-chart, .ql-lattis-pivot');
		clip = docClip.item;
		if (el) {
			const kind = el.classList.contains('ql-lattis-button') ? 'button' : el.classList.contains('ql-lattis-pivot') ? 'pivot' : 'chart';
			menu = { x: e.clientX, y: e.clientY, kind, index: indexOf(el) };
		} else menu = { x: e.clientX, y: e.clientY, kind: 'text', index: range()?.index ?? 0 };
		markSelected();
	}

	function openOutside(u: string) {
		import('@wailsio/runtime').then(
			(rt) => rt.Browser.OpenURL(u),
			() => window.open(u, '_blank')
		);
	}

	onMount(() => {
		const Q = setupQuill();
		const q = new Q(host!, { modules: { history: { delay: 800, maxStack: 200, userOnly: true } }, placeholder: 'Écrivez ici… (Insertion : graphiques et boutons)' });
		quill = q;
		q.setContents(parseDelta(source) as never, 'silent');
		q.history.clear();
		q.enable(!locked);
		hydrate();
		q.on('text-change', (delta, _old, src) => {
			if (src === 'silent') return;
			if (panel) {
				const moved = (delta as unknown as { transformPosition(i: number): number }).transformPosition(panel.index);
				if (moved !== panel.index) panel = { ...panel, index: moved };
			}
			hydrate();
			schedule();
		});
		q.on('selection-change', (r) => {
			if (r) lastRange = r;
		});
		q.on('editor-change', () => report());
		q.root.addEventListener('click', onClick);
		q.root.addEventListener('contextmenu', onContext);
		report();
		return () => {
			flush();
			for (const m of mounted.values()) unmount(m.comp);
			mounted.clear();
		};
	});

	const btn = $derived(panel?.kind === 'button' ? (panelValue as unknown as DocButtonConfig | null) : null);
	const chart = $derived(panel?.kind === 'chart' ? (panelValue as unknown as ChartConfig | null) : null);
	const fns = $derived(btn?.script ? scriptFunctions(scripts.find((s) => s.name === btn.script)?.source ?? '') : []);
	const setButton = (patch: Partial<DocButtonConfig>) => panel && btn && replaceEmbed(panel.index, BUTTON_BLOT, { ...btn, ...patch });
	const setChart = (cfg: ChartConfig) => panel && replaceEmbed(panel.index, CHART_BLOT, cfg);
	const pivot = $derived(panel?.kind === 'pivot' ? (panelValue as unknown as PivotConfig | null) : null);
	const pivotFields = (src: string) => {
		const t = tables.find((x) => x.name.toLowerCase() === src.toLowerCase());
		return (t?.columns ?? []).map((c, i) => ({ name: c, type: t!.types[i], numeric: ['integer', 'decimal', 'formula'].includes(t!.types[i]) }));
	};

	function applyLink(url: string | null) {
		if (!quill || panel?.kind !== 'link') return;
		const { index, length } = panel;
		const u = url?.trim() || false;
		if (length) quill.formatText(index, length, 'link', u, 'user');
		else if (u) {
			quill.insertText(index, u, 'link', u, 'user');
			quill.setSelection(index + u.length, 0);
		}
		openPanel(null);
		quill.focus();
	}
</script>

<div class="doc" style:--doc-font={DOC_FONT} style:--doc-size="{DOC_PT}pt">
	<div class="scroll">
		<div class="page" aria-label="Document {name}">
			<div bind:this={host}></div>
		</div>
	</div>
	{#if panel?.kind === 'chart' && chart}
		{@const tbl = tables.find((t) => t.name.toLowerCase() === chart.source.toLowerCase())}
		<div class="side">
			<div class="srcbox">
				<label
					>Tableau source
					<select
						value={chart.source}
						disabled={!!locked}
						onchange={(e) => {
							const t = tables.find((x) => x.name === (e.currentTarget as HTMLSelectElement).value);
							if (t) setChart({ ...defaultChart(t, chart.kind), title: chart.title === chart.source ? t.name : chart.title });
						}}
					>
						{#each tables as t (t.name)}<option value={t.name}>{t.name}</option>{/each}
						{#if !tbl}<option value={chart.source}>{chart.source} (introuvable)</option>{/if}
					</select>
				</label>
				<label
					>Hauteur (px)
					<input type="number" min="120" max="1200" step="20" value={chart.height || 300} disabled={!!locked} onchange={(e) => setChart({ ...chart, height: Number((e.currentTarget as HTMLInputElement).value) || 300 })} />
				</label>
				<button class="danger" disabled={!!locked} onclick={() => panel && deleteEmbed(panel.index)}><Icon name="trash" size={14} />Supprimer le graphique</button>
			</div>
			<ChartPanel config={chart} columns={tbl?.columns ?? []} onchange={setChart} onclose={() => openPanel(null)} />
		</div>
	{:else if panel?.kind === 'pivot' && pivot}
		<div class="side">
			<div class="srcbox">
				<label
					>Tableau source
					<select
						value={pivot.source}
						disabled={!!locked}
						onchange={(e) => {
							const t = tables.find((x) => x.name === (e.currentTarget as HTMLSelectElement).value);
							const at = panel?.index;
							if (t && at !== undefined) defaultPivot(t).then((cfg) => replaceEmbed(at, PIVOT_BLOT, cfg));
						}}
					>
						{#each tables.filter((t) => t.kind !== 'pivot') as t (t.name)}<option value={t.name}>{t.name}</option>{/each}
					</select>
				</label>
			</div>
			{#key panel.index + ':' + pivot.source}
				<PivotPanel
					config={pivot}
					fields={pivotFields(pivot.source)}
					filterValues={fieldValues ? (f) => fieldValues!(pivot.source, f) : undefined}
					onchange={(cfg) => panel && replaceEmbed(panel.index, PIVOT_BLOT, cfg)}
					ondelete={() => panel && deleteEmbed(panel.index)}
					onclose={() => openPanel(null)}
				/>
			{/key}
		</div>
	{:else if panel?.kind === 'button' && btn}
		<aside class="side form">
			<header><span>Bouton</span><button class="x" title="Fermer" onclick={() => openPanel(null)}>✕</button></header>
			<label>Libellé<input type="text" value={btn.label} disabled={!!locked} onchange={(e) => setButton({ label: (e.currentTarget as HTMLInputElement).value })} /></label>
			<label
				>Script
				<select value={btn.script} disabled={!!locked} onchange={(e) => setButton({ script: (e.currentTarget as HTMLSelectElement).value, function: '' })}>
					<option value="">(aucune action)</option>
					{#each scripts as s (s.name)}<option value={s.name}>{s.name}</option>{/each}
				</select>
			</label>
			{#if btn.script}
				<label
					>Exécuter
					<select value={btn.function ?? ''} disabled={!!locked} onchange={(e) => setButton({ function: (e.currentTarget as HTMLSelectElement).value })}>
						<option value="">tout le script</option>
						{#each fns as f (f)}<option value={f}>la fonction {f}()</option>{/each}
					</select>
				</label>
			{/if}
			<div class="colors">
				<label>Fond<input type="color" value={btn.color || '#217346'} disabled={!!locked} onchange={(e) => setButton({ color: (e.currentTarget as HTMLInputElement).value })} /></label>
				<label>Texte<input type="color" value={btn.textColor || '#ffffff'} disabled={!!locked} onchange={(e) => setButton({ textColor: (e.currentTarget as HTMLInputElement).value })} /></label>
			</div>
			<p class="hint">Un clic sur le bouton exécute son action ; clic droit pour revenir ici.</p>
			<div class="row">
				<button onclick={() => btn && runButton(btn)}><Icon name="play" size={14} />Essayer</button>
				<button class="danger" disabled={!!locked} onclick={() => panel && deleteEmbed(panel.index)}><Icon name="trash" size={14} />Supprimer</button>
			</div>
		</aside>
	{:else if panel?.kind === 'link'}
		{@const p = panel}
		<aside class="side form">
			<header><span>Lien</span><button class="x" title="Fermer" onclick={() => openPanel(null)}>✕</button></header>
			<label
				>Adresse
				<!-- svelte-ignore a11y_autofocus -->
				<input
					type="url"
					value={p.url}
					autofocus
					onkeydown={(e) => {
						if (e.key === 'Enter') applyLink((e.currentTarget as HTMLInputElement).value);
						if (e.key === 'Escape') openPanel(null);
					}}
					onchange={(e) => (p.url = (e.currentTarget as HTMLInputElement).value)}
				/>
			</label>
			<p class="hint">{p.length ? 'Le texte sélectionné devient un lien.' : 'L’adresse est insérée comme lien.'} Ctrl+clic sur un lien l’ouvre dans le navigateur.</p>
			<div class="row">
				<button class="primary" onclick={(e) => applyLink(((e.currentTarget as HTMLElement).closest('aside')!.querySelector('input') as HTMLInputElement).value)}>Appliquer</button>
				{#if p.length}<button onclick={() => applyLink(null)}>Retirer le lien</button>{/if}
			</div>
		</aside>
	{/if}
</div>

{#if menu}
	{@const m = menu}
	<div class="menu" role="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={closeMenu}>
		{#if m.kind === 'text'}
			<button disabled={!!locked} onclick={() => act(() => command('cut'))}>Couper</button>
			<button onclick={() => act(() => command('copy'))}>Copier</button>
			<button disabled={!!locked} onclick={() => act(() => command('paste'))}>Coller</button>
			{#if clip}<button disabled={!!locked} onclick={() => act(() => pasteItem(m.index))}>Coller l’élément ({clip.type === CHART_BLOT ? 'graphique' : clip.type === PIVOT_BLOT ? 'tableau croisé' : 'bouton'})</button>{/if}
			<hr />
			<div class="menu-title">Insérer</div>
			<button disabled={!!locked} onclick={() => act(() => insertChart('bar'))}><Icon name="chart-bar" size={14} />Graphique en barres</button>
			<button disabled={!!locked} onclick={() => act(() => insertChart('line'))}><Icon name="chart-line" size={14} />Graphique en courbes</button>
			<button disabled={!!locked} onclick={() => act(() => insertChart('pie'))}><Icon name="chart-pie" size={14} />Graphique en secteurs</button>
			<button disabled={!!locked} onclick={() => act(() => insertChart('scatter'))}><Icon name="chart-scatter" size={14} />Nuage de points</button>
			<button disabled={!!locked} onclick={() => act(insertPivot)}><Icon name="pivot-table" size={14} />Tableau croisé dynamique</button>
			<button disabled={!!locked} onclick={() => act(insertButton)}><Icon name="button" size={14} />Bouton</button>
			<button disabled={!!locked} onclick={() => act(() => command('image'))}><Icon name="image" size={14} />Image…</button>
			<button disabled={!!locked} onclick={() => act(() => command('link'))}><Icon name="link" size={14} />Lien…</button>
		{:else}
			{@const label = m.kind === 'chart' ? 'Graphique' : m.kind === 'pivot' ? 'Tableau croisé' : 'Bouton'}
			<div class="menu-title">{label}</div>
			{#if m.kind === 'button'}
				{@const b = embedAt(m.index, BUTTON_BLOT) as DocButtonConfig | null}
				{#if b?.script}<button onclick={() => act(() => runButton(b))}>Exécuter l’action</button>{/if}
			{/if}
			<button onclick={() => act(() => openPanel({ kind: m.kind as 'chart' | 'pivot' | 'button', index: m.index }))}>Modifier…</button>
			<hr />
			<button disabled={!!locked} onclick={() => act(() => copyItem(m.kind as 'chart', m.index, true))}>Couper</button>
			<button onclick={() => act(() => copyItem(m.kind as 'chart', m.index, false))}>Copier</button>
			<button disabled={!clip || !!locked} onclick={() => act(() => pasteItem(m.index + 1))}>Coller l’élément après</button>
			<hr />
			<button disabled={!!locked} onclick={() => act(() => deleteEmbed(m.index))}>Supprimer {m.kind === 'chart' ? 'le graphique' : m.kind === 'pivot' ? 'le tableau croisé' : 'le bouton'}</button>
		{/if}
	</div>
{/if}

<style>
	.menu {
		position: fixed;
		z-index: 40;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius, 6px);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 220px;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
	}
	.menu-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}
	.menu button {
		display: flex;
		width: 100%;
		text-align: left;
		padding: 6px 14px;
		background: none;
		border: none;
		border-radius: 0;
		gap: 8px;
	}
	.menu button:hover:not(:disabled) {
		background: #e8f1ec;
	}
	.menu hr {
		border: none;
		border-top: 1px solid #e5e7eb;
		margin: 4px 0;
	}
	.doc {
		display: flex;
		height: 100%;
		min-height: 0;
		background: #fff;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.scroll {
		flex: 1;
		min-width: 0;
		overflow-y: auto;
		container-type: inline-size;
	}
	.page {
		max-width: 920px;
		margin: 0 auto;
		padding: 24px 40px 120px;
		min-height: 100%;
		box-sizing: border-box;
	}
	/* Fenêtre large (plus de 300 px vides de chaque côté) : un trait fin, à
	   une petite marge du texte, délimite la zone d'édition. */
	@container (min-width: 1521px) {
		.page {
			border-left: 1px solid #e3e5e8;
			border-right: 1px solid #e3e5e8;
		}
	}
	.page :global(.ql-container) {
		font-family: var(--doc-font);
		font-size: var(--doc-size);
		border: none;
		height: auto;
	}
	.page :global(.ql-editor) {
		padding: 0;
		min-height: 60vh;
		line-height: 1.5;
		overflow: visible;
	}
	.page :global(.ql-editor.ql-blank::before) {
		left: 0;
		right: 0;
		color: #9ca3af;
		font-style: normal;
	}
	.page :global(.ql-editor h1) {
		font-size: 2em;
		font-weight: 600;
		margin: 0.4em 0 0.3em;
		color: #1b5e3a;
	}
	.page :global(.ql-editor h2) {
		font-size: 1.5em;
		font-weight: 600;
		margin: 0.6em 0 0.2em;
		color: #217346;
	}
	.page :global(.ql-editor h3) {
		font-size: 1.2em;
		font-weight: 600;
		margin: 0.5em 0 0.2em;
	}
	.page :global(.ql-editor blockquote) {
		border-left: 3px solid #a7d3b8;
		margin: 6px 0;
		padding-left: 12px;
		color: #4b5563;
	}
	.page :global(.ql-editor .ql-code-block-container) {
		background: #f6f8fa;
		border: 1px solid #e3e5e8;
		border-radius: var(--float-radius, 6px);
		padding: 8px 12px;
		font: 12px Consolas, 'Cascadia Mono', monospace;
	}
	.page :global(.ql-editor a) {
		color: #0b6bcb;
		cursor: pointer;
	}
	.page :global(.ql-editor img) {
		max-width: 100%;
	}
	.page :global(.ql-lattis-button > span[contenteditable]) {
		display: inline-block;
		padding: 3px 12px;
		margin: 0 2px;
		border-radius: var(--float-radius, 6px);
		background: inherit;
		cursor: pointer;
		user-select: none;
	}
	.page :global(.ql-lattis-button) {
		background: #217346;
		color: #fff;
		border-radius: var(--float-radius, 6px);
		font-size: 0.95em;
		box-shadow: 0 1px 2px rgba(0, 0, 0, 0.15);
	}
	.page :global(.ql-lattis-button:hover) {
		filter: brightness(1.08);
	}
	.page :global(.ql-lattis-button:active) {
		filter: brightness(0.92);
	}
	.side {
		display: flex;
		flex-direction: column;
		border-left: 1px solid #d4d7db;
		background: #fafbfb;
		min-height: 0;
	}
	.srcbox,
	.form {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 10px;
		font-size: 12px;
	}
	.srcbox {
		border-bottom: 1px solid #e5e7eb;
		width: 280px;
		box-sizing: border-box;
	}
	.form {
		width: 280px;
		box-sizing: border-box;
	}
	.side :global(.panel) {
		flex: 1;
		border-left: none;
	}
	.form header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		font-size: 13px;
		font-weight: 600;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 3px;
		color: #4b5563;
	}
	input[type='text'],
	input[type='url'],
	input[type='number'],
	select {
		font: inherit;
		padding: 4px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		background: #fff;
		color: #1f2328;
	}
	.colors {
		display: flex;
		gap: 12px;
	}
	.colors input {
		width: 48px;
		height: 24px;
		padding: 0;
		border: 1px solid #c9cdd2;
	}
	.row {
		display: flex;
		gap: 6px;
		flex-wrap: wrap;
	}
	button {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		font: inherit;
		padding: 4px 10px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
		--icon-accent: #217346;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	button.danger {
		color: #b42318;
		--icon-accent: #b42318;
	}
	button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.x {
		border: none;
		background: none;
		color: #6b7280;
		padding: 2px 6px;
	}
	.hint {
		margin: 0;
		font-size: 11px;
		color: #6b7280;
	}
</style>
