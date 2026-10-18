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
	import { MORE_KINDS, chartFields } from '#lib/chart/kinds.js';
	import { encodeObj, objFromText, type ObjClip } from '#lib/clip/objects.js';
	import PivotPanel from '#lib/grid/PivotPanel.svelte';
	import { dismiss } from '#lib/grid/dismiss.js';
	import { resizePanel } from '#lib/ui/resize.js';
	import { panelKeys } from '#lib/ui/focus.js';
	import HelpLink from '#lib/help/HelpLink.svelte';
	import { printSheet } from '#lib/shell/print.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { scriptFunctions } from '#lib/grid/buttons.js';
	import { BUTTON_BLOT, CHART_BLOT, CONTROL_BLOT, PIVOT_BLOT, docClip, setupQuill, type DocButtonConfig } from './quill';
	import DocControl, { type ControlState } from './DocControl.svelte';
	import { CONTROL_KINDS, VALUE_KINDS, type ButtonConfig, type ControlKind } from '#lib/grid/buttons.js';
	import { isDefaultTableName } from '#lib/grid/defaultname.js';
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
		/** Feuilles (grilles) du classeur, pour la cellule liée des contrôles. */
		sheets?: string[];
		/** Contrôles du classeur posés dans le document (champ, case, liste, fichier). */
		controls?: {
			load: (id: number) => Promise<ControlState>;
			create: (cfg: ButtonConfig) => Promise<number>;
			update: (id: number, cfg: ButtonConfig) => Promise<void>;
			setValue: (id: number, raw: string) => Promise<void>;
			click: (id: number) => Promise<void>;
			pickFile: (title: string) => Promise<string>;
		};
		/** Valeurs d'un champ (filtres du volet de tableau croisé). */
		fieldValues?: (table: string, field: string) => Promise<{ text: string; count: number }[]>;
		/** Exécute un script (ou sa fonction) ; renvoie le message à afficher. */
		run: (script: string, fn: string) => Promise<string>;
		/** Mise en forme à la sélection, pour le ruban. */
		onformat?: (f: DocFormat) => void;
		onnotice?: (msg: string) => void;
	}

	let { name, source, save, locked = '', stamp, animate = true, tables, scripts, loadChart, loadPivot, fieldValues, sheets = [], controls, run, onformat, onnotice }: Props = $props();

	let host = $state<HTMLDivElement>();
	let quill: Quill | undefined;
	let lastSaved = untrack(() => source);
	let timer: ReturnType<typeof setTimeout> | undefined;
	let pending = false;
	let current: DocFormat = docFormat({});
	let lastRange: { index: number; length: number } | null = null;
	const live = $state({ stamp: 0, animate: untrack(() => animate) });

	// Objet en cours de modification dans le volet de droite (par sa position).
	type Panel =
		| { kind: 'chart' | 'button' | 'pivot'; index: number }
		| { kind: 'link'; index: number; length: number; url: string }
		| { kind: 'control'; index: number; id: number | null; cfg: ButtonConfig };
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
	const BLOT = { chart: CHART_BLOT, pivot: PIVOT_BLOT, button: BUTTON_BLOT, control: CONTROL_BLOT } as const;
	const ctlMounted = new Map<HTMLElement, ReturnType<typeof mount>>();

	function hydrate() {
		if (!quill) return;
		if (controls) {
			const api = controls;
			for (const node of quill.root.querySelectorAll<HTMLElement>('.ql-lattis-control')) {
				if (ctlMounted.has(node)) continue;
				const v = JSON.parse(node.dataset.config ?? '{}') as { id?: number; width?: number; height?: number };
				const target = node.querySelector<HTMLElement>('span[contenteditable]') ?? node;
				const comp = mount(DocControl, {
					target,
					props: {
						id: v.id ?? 0,
						width: v.width,
						height: v.height,
						live,
						load: api.load,
						setValue: api.setValue,
						click: api.click,
						pickFile: api.pickFile,
						get locked() {
							return !!locked;
						}
					}
				});
				ctlMounted.set(node, comp);
			}
			for (const [node, comp] of ctlMounted) {
				if (!node.isConnected) {
					unmount(comp);
					ctlMounted.delete(node);
				}
			}
		}
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
			// −/+ d'un tableau croisé : nouvelle configuration de l'élément (annulable).
			const onconfig = (cfg: PivotConfig) => (locked ? onnotice?.(locked) : replaceEmbed(indexOf(node), PIVOT_BLOT, cfg));
			const comp = kind === 'pivot' ? mount(DocPivot, { target: node, props: { ...props, load: loadPivot, onconfig } }) : mount(DocChart, { target: node, props: { ...props, load: loadChart } });
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

	// Élément sélectionné hors volet (image cliquée) : Suppr le retire.
	let picked = $state<{ kind: 'image'; index: number } | null>(null);

	/** Suppr ou Retour arrière : supprime l'élément sélectionné (graphique, tableau croisé, bouton, contrôle, image). */
	function onKey(ev: KeyboardEvent) {
		if (ev.key !== 'Delete' && ev.key !== 'Backspace') return;
		const target = ev.target as HTMLElement | null;
		// Saisie dans un champ du volet ou d'un contrôle : la touche lui revient.
		if (target?.closest('input, textarea, select') ) return;
		// Graphique, tableau croisé ou image sélectionnés (cadre vert) ; bouton ou
		// contrôle dont le volet est ouvert, si l'on n'écrit pas dans le texte.
		const inText = !!target?.closest('.ql-editor');
		const sel =
			panel && (panel.kind === 'chart' || panel.kind === 'pivot')
				? panel.index
				: panel && !inText && (panel.kind === 'button' || (panel.kind === 'control' && panel.id !== null))
					? panel.index
					: picked?.index;
		if (sel === undefined || sel === null || locked) return;
		ev.preventDefault();
		ev.stopPropagation();
		picked = null;
		deleteEmbed(sel);
	}

	function markSelected() {
		quill?.root.querySelectorAll('img.picked').forEach((i) => i.classList.remove('picked'));
		if (picked && quill) {
			const [leaf] = quill.getLeaf(picked.index + 1) as unknown as [{ domNode?: HTMLElement } | null];
			if (leaf?.domNode?.tagName === 'IMG') leaf.domNode.classList.add('picked');
		}
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
		panelValue = p && p.kind !== 'link' && p.kind !== 'control' ? embedAt(p.index, BLOT[p.kind]) : null;
		markSelected();
	}

	// Remplacement en cours : la position de l'élément ne bouge pas (sans quoi
	// le volet viserait le caractère suivant et dupliquerait l'élément).
	let replacing = false;

	function replaceEmbed(index: number, type: string, value: unknown) {
		if (!quill || locked) return;
		// Jamais à l'aveugle : il faut que l'élément soit bien là.
		if (!embedAt(index, type)) return;
		const Delta = (quill.constructor as typeof Quill).import('delta') as unknown as new () => { retain(n: number): unknown };
		const d = new Delta() as { retain(n: number): { delete(n: number): { insert(v: unknown): unknown } } };
		replacing = true;
		try {
			quill.updateContents(d.retain(index).delete(1).insert({ [type]: value }) as never, 'user');
		} finally {
			replacing = false;
		}
		if (panel && panel.index === index && panel.kind !== 'link' && panel.kind !== 'control' && BLOT[panel.kind] === type) panelValue = value as Record<string, unknown>;
		hydrate();
	}

	function deleteEmbed(index: number) {
		if (!quill || locked) return;
		// Seulement un élément inséré, jamais du texte.
		const ins = quill.getContents(index, 1).ops[0]?.insert;
		if (!ins || typeof ins !== 'object') return;
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
						if (pasteObject(t, at)) return;
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
		// Une carte lit les lignes d'un tableau (coordonnées).
		const t = kind === 'map' ? (tables.find((x) => x.kind !== 'pivot') ?? tables[0]) : (tables.find((x) => x.kind === 'pivot') ?? tables[0]);
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
		// Carte : carré grisé, coordonnées à choisir dans le volet.
		if (kind === 'map') return { source: t.name, kind, x: '', series: [], mapDraft: true, title: t.name } as ChartConfig;
		// Même règle que sur une feuille (chartFields) ; sans colonne de nombres,
		// la deuxième colonne, à compléter dans le volet.
		const f = chartFields(kind, t.columns.map((name, i) => ({ name, numeric: num(i) })));
		const series = f.series.length ? f.series : t.columns.filter((c) => c !== f.x).slice(0, 1);
		return { source: t.name, kind, x: f.x, series, title: t.name } as ChartConfig;
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

	// --- Taille des éléments insérés (poignée en bas à droite) ----------------

	type Grip = { node: HTMLElement; kind: 'chart' | 'pivot' | 'button' | 'control' | 'image'; x: number; y: number };
	let grip = $state<Grip | null>(null);
	let pageEl = $state<HTMLDivElement>();
	let sizing: { grip: Grip; x0: number; y0: number; w0: number; h0: number; w: number; h: number } | null = null;

	const SIZED = '.ql-lattis-chart, .ql-lattis-pivot, .ql-lattis-button, .ql-lattis-control, .ql-editor img';
	function kindOf(el: HTMLElement): Grip['kind'] {
		return el.tagName === 'IMG' ? 'image' : el.classList.contains('ql-lattis-pivot') ? 'pivot' : el.classList.contains('ql-lattis-button') ? 'button' : el.classList.contains('ql-lattis-control') ? 'control' : 'chart';
	}
	// Le cadre visible de l'élément (le bloc d'un graphique s'étend sur toute la ligne).
	function boxOf(el: HTMLElement): HTMLElement {
		return (el.querySelector('.dchart, .dpivot, .dctl') as HTMLElement | null) ?? el;
	}
	function placeGrip(el: HTMLElement | null) {
		if (sizing) return;
		if (!el || !pageEl || locked) {
			grip = null;
			return;
		}
		const r = boxOf(el).getBoundingClientRect();
		const p = pageEl.getBoundingClientRect();
		grip = { node: el, kind: kindOf(el), x: r.right - p.left, y: r.bottom - p.top };
	}
	function onHover(e: MouseEvent) {
		if (sizing) return;
		const t = e.target as HTMLElement;
		if (t.closest('.grip')) return;
		const el = t.closest<HTMLElement>(SIZED);
		// En allant vers la poignée, le pointeur peut quitter l'élément : elle reste.
		if (!el && grip && pageEl) {
			const p = pageEl.getBoundingClientRect();
			if (Math.hypot(e.clientX - p.left - grip.x, e.clientY - p.top - grip.y) < 28) return;
		}
		placeGrip(el);
	}
	function startSize(e: PointerEvent) {
		if (!grip || e.button !== 0) return;
		e.preventDefault();
		e.stopPropagation();
		const box = boxOf(grip.node);
		const r = box.getBoundingClientRect();
		sizing = { grip, x0: e.clientX, y0: e.clientY, w0: r.width, h0: r.height, w: r.width, h: r.height };
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
	}
	function moveSize(e: PointerEvent) {
		if (!sizing) return;
		const s = sizing;
		const maxW = (quill?.root.clientWidth ?? 900) + 1;
		s.w = Math.round(Math.max(40, Math.min(maxW, s.w0 + e.clientX - s.x0)));
		// Un tableau croisé garde la hauteur de son contenu.
		s.h = s.grip.kind === 'pivot' ? s.h0 : Math.round(Math.max(s.grip.kind === 'button' || s.grip.kind === 'control' ? 20 : 60, s.h0 + e.clientY - s.y0));
		// Aperçu pendant le geste ; la valeur est enregistrée au lâcher.
		const box = boxOf(s.grip.node);
		box.style.width = s.w + 'px';
		if (s.grip.kind !== 'pivot') box.style.height = s.h + 'px';
		const p = pageEl!.getBoundingClientRect();
		const r = box.getBoundingClientRect();
		grip = { ...s.grip, x: r.right - p.left, y: r.bottom - p.top };
	}
	function endSize() {
		const s = sizing;
		sizing = null;
		if (!s || !quill) return;
		const i = indexOf(s.grip.node);
		if (i < 0) return;
		if (s.grip.kind === 'image') {
			quill.formatText(i, 1, { width: String(s.w), height: String(s.h) }, 'user');
		} else {
			const type = BLOT[s.grip.kind];
			const v = embedAt(i, type);
			if (v) replaceEmbed(i, type, s.grip.kind === 'pivot' ? { ...v, width: s.w } : { ...v, width: s.w, height: s.h });
		}
		grip = null;
	}

	// --- Menus contextuels et presse-papiers des éléments ---------------------

	type Menu = { x: number; y: number; kind: 'chart' | 'pivot' | 'button' | 'control' | 'text'; index: number };
	let menu = $state<Menu | null>(null);
	// « Autres graphiques » déplié dans le menu contextuel.
	let moreCharts = $state(false);
	let clip = $state(docClip.item);

	/** Graphique ou tableau croisé à la position index, pour le presse-papiers (feuilles comprises). */
	function objAt(index: number): ObjClip | null {
		for (const kind of ['chart', 'pivot'] as const) {
			const v = embedAt(index, BLOT[kind]);
			if (v) return { kind, config: structuredClone($state.snapshot(v)) };
		}
		return null;
	}

	/**
	 * Collage d'un graphique ou d'un tableau croisé copié sur une feuille ou
	 * dans un Onglet Document (objFromText) : inséré dans le texte, à la place
	 * de la sélection. Faux : autre chose, collé comme du texte.
	 */
	function pasteObject(text: string, at: { index: number; length: number }): boolean {
		const o = objFromText(text);
		if (!o || !quill || !guard()) return false;
		// Place sur la feuille et nom du TCD : propres à la feuille.
		const config = { ...o.config };
		for (const k of ['offX', 'offY', 'name']) delete config[k];
		if (at.length) quill.deleteText(at.index, at.length, 'user');
		quill.insertEmbed(at.index, o.kind === 'chart' ? CHART_BLOT : PIVOT_BLOT, config, 'user');
		quill.setSelection(at.index + 1, 0);
		hydrate();
		onnotice?.(o.kind === 'chart' ? 'Graphique collé' : 'Tableau croisé collé');
		return true;
	}

	function onPaste(e: ClipboardEvent) {
		const text = e.clipboardData?.getData('text/plain') ?? '';
		if (!objFromText(text)) return;
		e.preventDefault();
		e.stopPropagation();
		pasteObject(text, range()!);
	}

	// Ctrl+C / Ctrl+X sur un graphique ou un tableau croisé sélectionné (cadre
	// vert) : copié aussi pour les feuilles.
	function onCopyCut(e: ClipboardEvent) {
		if ((e.target as HTMLElement | null)?.closest?.('input, textarea, select')) return;
		if (!panel || (panel.kind !== 'chart' && panel.kind !== 'pivot')) return;
		const o = objAt(panel.index);
		if (!o) return;
		e.preventDefault();
		e.clipboardData?.setData('text/plain', encodeObj(o));
		docClip.item = clip = { type: BLOT[o.kind], value: o.config };
		if (e.type === 'cut' && !locked) deleteEmbed(panel.index);
		onnotice?.(o.kind === 'chart' ? 'Graphique copié : Ctrl+V dans le texte ou sur une feuille' : 'Tableau croisé copié : Ctrl+V dans le texte ou sur une feuille');
	}

	async function copyItem(kind: 'chart' | 'pivot' | 'button' | 'control', index: number, cut: boolean) {
		let value = embedAt(index, BLOT[kind]);
		if (!value) return;
		const o = objAt(index);
		if (o) await navigator.clipboard?.writeText(encodeObj(o)).catch(() => undefined);
		// Un contrôle se copie par sa configuration : le coller en crée un nouveau.
		if (kind === 'control' && controls) value = { cfg: (await controls.load(Number(value.id))).config, width: value.width, height: value.height };
		docClip.item = clip = { type: BLOT[kind], value: structuredClone($state.snapshot(value)) };
		if (cut) deleteEmbed(index);
		onnotice?.(o ? (cut ? 'Élément coupé : Ctrl+V dans le texte ou sur une feuille' : 'Élément copié : Ctrl+V dans le texte ou sur une feuille') : cut ? 'Élément coupé : clic droit › Coller dans le texte' : 'Élément copié : clic droit › Coller dans le texte');
	}

	async function pasteItem(at: number) {
		const c = docClip.item;
		if (!c || !guard()) return;
		let value = structuredClone(c.value) as Record<string, unknown>;
		if (c.type === CONTROL_BLOT) {
			if (!controls) return;
			const cfg = { ...(value.cfg as ButtonConfig), name: '' };
			try {
				value = { id: await controls.create(cfg), width: value.width, height: value.height };
			} catch (e) {
				onnotice?.(String(e instanceof Error ? e.message : e));
				return;
			}
		}
		quill!.insertEmbed(at, c.type, value, 'user');
		quill!.setSelection(at + 1, 0);
		hydrate();
	}

	function closeMenu() {
		menu = null;
		moreCharts = false;
		markSelected();
	}

	// L'action d'abord : elle lit le menu (sa position), que la fermeture efface.
	function act(fn: () => void) {
		fn();
		closeMenu();
	}

	/**
	 * Insertion › Champ : un contrôle inséré tout de suite (le volet s'ouvre
	 * pour le régler). Sans cellule liée, sa valeur est gardée dans le
	 * contrôle ; le volet peut la lier à une cellule d'une feuille.
	 */
	export async function insertControl(kind: ControlKind = 'input') {
		if (!guard() || !controls) return;
		const api = controls;
		const at = range()!.index;
		const cfg: ButtonConfig = { kind, label: '', script: '', options: kind === 'select' ? ['Choix 1', 'Choix 2'] : undefined };
		try {
			const id = await api.create(cfg);
			quill!.insertEmbed(at, CONTROL_BLOT, { id }, 'user');
			quill!.setSelection(at + 1, 0);
			hydrate();
			openPanel({ kind: 'control', index: at, id, cfg: (await api.load(id)).config });
		} catch (e) {
			onnotice?.(String(e instanceof Error ? e.message : e));
		}
	}

	async function editControl(index: number) {
		const v = embedAt(index, CONTROL_BLOT);
		if (!v || !controls) return;
		try {
			const st = await controls.load(Number(v.id));
			openPanel({ kind: 'control', index, id: Number(v.id), cfg: structuredClone(st.config) });
		} catch {
			onnotice?.('Ce contrôle n’existe plus : supprimez-le du texte.');
		}
	}

	async function saveControl() {
		if (panel?.kind !== 'control' || !controls || !guard()) return;
		const p = panel;
		const cfg = $state.snapshot(p.cfg) as ButtonConfig;
		try {
			if (p.id === null) {
				const id = await controls.create(cfg);
				quill!.insertEmbed(p.index, CONTROL_BLOT, { id }, 'user');
				quill!.setSelection(p.index + 1, 0);
				hydrate();
				panel = { ...p, id };
			} else {
				await controls.update(p.id, cfg);
				live.stamp++;
			}
		} catch (e) {
			onnotice?.(String(e instanceof Error ? e.message : e));
		}
	}

	export function insertButton() {
		if (!guard()) return;
		const at = range()!.index;
		const cfg: DocButtonConfig = { label: 'Bouton', script: scripts[0]?.name ?? '' };
		quill!.insertEmbed(at, BUTTON_BLOT, cfg, 'user');
		quill!.setSelection(at + 1, 0);
		openPanel({ kind: 'button', index: at });
	}

	/** Imprime le document (ou l'enregistre en PDF) tel qu'il est affiché : graphiques et tableaux croisés compris. */
	export async function print(title: string) {
		await flush();
		if (!pageEl) return;
		const clone = pageEl.cloneNode(true) as HTMLElement;
		clone.querySelectorAll('.grip, .ql-clipboard, .ql-tooltip').forEach((e) => e.remove());
		clone.querySelectorAll('.selected').forEach((e) => e.classList.remove('selected'));
		clone.style.setProperty('--doc-font', DOC_FONT);
		clone.style.setProperty('--doc-size', `${DOC_PT}pt`);
		// Graphiques à l'échelle de la page imprimée.
		clone.querySelectorAll('svg').forEach((svg) => {
			const w = svg.getAttribute('width');
			const h = svg.getAttribute('height');
			if (w && h && !svg.getAttribute('viewBox')) svg.setAttribute('viewBox', `0 0 ${w} ${h}`);
		});
		clone.style.maxWidth = 'none';
		clone.style.border = 'none';
		clone.style.padding = '0';
		printSheet(`<div class="doc-print">${clone.outerHTML}</div>`, title);
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
		if (t.closest('.ql-lattis-control')) return;
		if (t.tagName === 'IMG') {
			picked = { kind: 'image', index: indexOf(t) };
			markSelected();
			return;
		}
		picked = null;
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
		const t = e.target as HTMLElement;
		// Maj + clic droit dans le texte : menu du système, avec les suggestions
		// d'orthographe (le navigateur ne les donne pas à la page).
		if (e.shiftKey && !t.closest('.ql-lattis-button, .ql-lattis-chart, .ql-lattis-pivot, .ql-lattis-control')) return;
		e.preventDefault();
		const el = t.closest<HTMLElement>('.ql-lattis-button, .ql-lattis-chart, .ql-lattis-pivot, .ql-lattis-control');
		clip = docClip.item;
		if (el) {
			const kind = el.classList.contains('ql-lattis-button') ? 'button' : el.classList.contains('ql-lattis-pivot') ? 'pivot' : el.classList.contains('ql-lattis-control') ? 'control' : 'chart';
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
			if (panel && !replacing) {
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
		q.root.addEventListener('paste', onPaste, true);
		window.addEventListener('keydown', onKey, true);
		window.addEventListener('copy', onCopyCut);
		window.addEventListener('cut', onCopyCut);
		report();
		// Document créé ou ouvert : le curseur dans le texte, à la fin (sous le
		// titre d'un nouveau document) ; le focus ne reste pas sur le bouton du
		// ruban ou l'onglet cliqué, où la frappe se perdait et où Espace
		// recréait un document.
		const raf = requestAnimationFrame(() => {
			if (quill !== q) return;
			q.focus();
			q.setSelection(Math.max(0, q.getLength() - 1), 0);
		});
		return () => {
			cancelAnimationFrame(raf);
			window.removeEventListener('keydown', onKey, true);
			window.removeEventListener('copy', onCopyCut);
			window.removeEventListener('cut', onCopyCut);
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
	// Carte du volet : colonnes repérées (en attente) et coordonnées illisibles.
	let mapInfo = $state.raw<ChartInfo | null>(null);
	$effect(() => {
		const c = chart;
		void live.stamp;
		if (!c || c.kind !== 'map') {
			mapInfo = null;
			return;
		}
		let on = true;
		untrack(() => loadChart(c)).then(
			(r) => on && (mapInfo = r),
			() => on && (mapInfo = null)
		);
		return () => (on = false);
	});
	// Colonnes des tableaux qui peuvent donner les choix d'une liste.
	const colRefs = $derived(tables.filter((t) => t.kind !== 'pivot').flatMap((t) => t.columns.map((c) => `${t.name}[${c}]`)));
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
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="page" aria-label="Document {name}" bind:this={pageEl} onmouseover={onHover} onfocus={() => {}} onmouseleave={() => !sizing && (grip = null)}>
			<div bind:this={host}></div>
			{#if grip}
				<div
					class="grip"
					style:left="{grip.x - 13}px"
					style:top="{grip.y - 13}px"
					title="Tirer pour redimensionner"
					onpointerdown={startSize}
					onpointermove={moveSize}
					onpointerup={endSize}
					role="presentation"
				></div>
			{/if}
		</div>
	</div>
	{#if panel?.kind === 'chart' && chart}
		{@const tbl = tables.find((t) => t.name.toLowerCase() === chart.source.toLowerCase())}
		<div class="side panelbox" use:resizePanel={{ key: 'doc-side', min: 260 }} use:panelKeys={() => openPanel(null)}>
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
						{#each tables as t (t.name)}<option value={t.name} class:dflt={isDefaultTableName(t.name)}>{t.name}</option>{/each}
						{#if !tbl}<option value={chart.source}>{chart.source} (introuvable)</option>{/if}
					</select>
				</label>
				<label
					>Hauteur (px)
					<input type="number" min="120" max="1200" step="20" value={chart.height || 300} disabled={!!locked} onchange={(e) => setChart({ ...chart, height: Number((e.currentTarget as HTMLInputElement).value) || 300 })} />
				</label>
				<button class="danger" disabled={!!locked} onclick={() => panel && deleteEmbed(panel.index)}><Icon name="trash" size={14} />Supprimer le graphique</button>
			</div>
			<ChartPanel config={chart} guess={mapInfo?.guess} invalid={mapInfo?.invalid} columns={tbl?.columns ?? []} onchange={setChart} onclose={() => openPanel(null)} resizable={false} />
		</div>
	{:else if panel?.kind === 'pivot' && pivot}
		<div class="side panelbox" use:resizePanel={{ key: 'doc-side', min: 260 }} use:panelKeys={() => openPanel(null)}>
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
						{#each tables.filter((t) => t.kind !== 'pivot') as t (t.name)}<option value={t.name} class:dflt={isDefaultTableName(t.name)}>{t.name}</option>{/each}
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
					resizable={false}
				/>
			{/key}
		</div>
	{:else if panel?.kind === 'control'}
		{@const p = panel}
		{@const at = p.cfg.link ? (p.cfg.link.lastIndexOf('!') >= 0 ? [p.cfg.link.slice(0, p.cfg.link.lastIndexOf('!')).replace(/^'(.*)'$/, '$1').replace(/''/g, "'"), p.cfg.link.slice(p.cfg.link.lastIndexOf('!') + 1)] : [name, p.cfg.link]) : ['', '']}
		{@const setLink = (sheet: string, cell: string) => (p.cfg.link = !sheet ? '' : sheet === name ? (cell.trim() || 'A1').toUpperCase() : `${/^[\p{L}_][\p{L}\p{N}_]*$/u.test(sheet) ? sheet : `'${sheet.replace(/'/g, "''")}'`}!${(cell.trim() || 'A1').toUpperCase()}`)}
		{@const cfns = p.cfg.script ? scriptFunctions(scripts.find((s) => s.name === p.cfg.script)?.source ?? '') : []}
		<aside class="side form" use:resizePanel={{ key: 'doc-control' }} use:panelKeys={() => openPanel(null)}>
			<header><span>{p.id === null ? 'Insérer' : 'Modifier'} : {CONTROL_KINDS[p.cfg.kind ?? 'input'].toLowerCase()}</span><HelpLink target="controles" label="Aide sur les boutons et contrôles" /><button class="x" title="Fermer (Échap)" aria-label="Fermer le volet" onclick={() => openPanel(null)}>✕</button></header>
			<label
				>Type
				<select bind:value={p.cfg.kind} disabled={!!locked}>
					{#each VALUE_KINDS as k (k)}<option value={k}>{CONTROL_KINDS[k]}</option>{/each}
				</select>
			</label>
			<label>Nom (pour les scripts)<input type="text" bind:value={p.cfg.name} placeholder="automatique" disabled={!!locked} /></label>
			{#if p.cfg.kind !== 'select'}
				<label>{p.cfg.kind === 'checkbox' ? 'Libellé' : p.cfg.kind === 'file' ? 'Texte du bouton' : 'Texte indicatif (facultatif)'}<input type="text" bind:value={p.cfg.label} disabled={!!locked} /></label>
			{/if}
			<div class="link">
				<label
					>Valeur gardée dans
					<select value={at[0]} disabled={!!locked} onchange={(e) => setLink((e.currentTarget as HTMLSelectElement).value, at[1])}>
						<option value="">le contrôle (aucune cellule liée)</option>
						{#each sheets as s (s)}<option value={s}>la feuille {s}</option>{/each}
						{#if at[0] && !sheets.includes(at[0])}<option value={at[0]}>{at[0]} (introuvable)</option>{/if}
					</select>
				</label>
				{#if at[0]}<label>Cellule<input type="text" value={at[1]} disabled={!!locked} onchange={(e) => setLink(at[0], (e.currentTarget as HTMLInputElement).value)} /></label>{/if}
			</div>
			{#if p.cfg.kind === 'select'}
				<div class="field">
					<span>Source des choix</span>
					<div class="seg" role="radiogroup" aria-label="Source des choix">
						<button type="button" role="radio" aria-checked={!p.cfg.optionsFrom} class:on={!p.cfg.optionsFrom} disabled={!!locked} onclick={() => (p.cfg.optionsFrom = '')}>Saisis</button>
						<button type="button" role="radio" aria-checked={!!p.cfg.optionsFrom} class:on={!!p.cfg.optionsFrom} disabled={!!locked || !colRefs.length} onclick={() => (p.cfg.optionsFrom = p.cfg.optionsFrom || colRefs[0])}>Colonne d’un tableau</button>
					</div>
				</div>
			{/if}
			{#if p.cfg.kind === 'select' && p.cfg.optionsFrom}
				<label
					>Choix tirés de (valeurs distinctes)
					<select bind:value={p.cfg.optionsFrom} disabled={!!locked}>
						{#each colRefs as r (r)}<option value={r}>{r}</option>{/each}
					</select>
				</label>
			{:else if p.cfg.kind === 'select'}
				<label
					>Choix (un par ligne)
					<textarea rows="4" disabled={!!locked} value={(p.cfg.options ?? []).join('\n')} onchange={(e) => (p.cfg.options = (e.currentTarget as HTMLTextAreaElement).value.split('\n').map((x) => x.trim()).filter(Boolean))}></textarea>
				</label>
			{/if}
			<label
				>Script (à chaque changement)
				<select value={p.cfg.script} disabled={!!locked} onchange={(e) => ((p.cfg.script = (e.currentTarget as HTMLSelectElement).value), (p.cfg.function = ''))}>
					<option value="">(aucune action)</option>
					{#each scripts as s (s.name)}<option value={s.name}>{s.name}</option>{/each}
				</select>
			</label>
			{#if p.cfg.script}
				<label
					>Exécuter
					<select bind:value={p.cfg.function} disabled={!!locked}>
						<option value="">tout le script</option>
						{#each cfns as f (f)}<option value={f}>la fonction {f}()</option>{/each}
					</select>
				</label>
			{/if}
			<p class="hint">La valeur du contrôle est celle de la cellule liée (dans ce document, ou sur une feuille pour la lire dans les formules). Les scripts le pilotent par controls.get / controls.set et controls.on("change", …).</p>
			<div class="row">
				<button class="primary" disabled={!!locked} onclick={saveControl}>{p.id === null ? 'Insérer' : 'Enregistrer'}</button>
				<button onclick={() => openPanel(null)}>{p.id === null ? 'Annuler' : 'Fermer'}</button>
			</div>
		</aside>
	{:else if panel?.kind === 'button' && btn}
		<aside class="side form" use:resizePanel={{ key: 'doc-control' }} use:panelKeys={() => openPanel(null)}>
			<header><span>Bouton</span><HelpLink target="controles" label="Aide sur les boutons et contrôles" /><button class="x" title="Fermer (Échap)" aria-label="Fermer le volet" onclick={() => openPanel(null)}>✕</button></header>
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
		<aside class="side form" use:resizePanel={{ key: 'doc-link' }} use:panelKeys={() => openPanel(null)}>
			<header><span>Lien</span><button class="x" title="Fermer (Échap)" aria-label="Fermer le volet" onclick={() => openPanel(null)}>✕</button></header>
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
			<div class="menu-hint">Maj + clic droit : suggestions d’orthographe</div>
			{#if clip}<button disabled={!!locked} onclick={() => act(() => pasteItem(m.index))}>Coller l’élément ({clip.type === CHART_BLOT ? 'graphique' : clip.type === PIVOT_BLOT ? 'tableau croisé' : clip.type === CONTROL_BLOT ? 'contrôle' : 'bouton'})</button>{/if}
			<hr />
			<div class="menu-title">Insérer</div>
			<button disabled={!!locked} onclick={() => act(() => insertChart('bar'))}><Icon name="chart-bar" size={14} />Graphique en barres</button>
			<button disabled={!!locked} onclick={() => act(() => insertChart('line'))}><Icon name="chart-line" size={14} />Graphique en courbes</button>
			<button disabled={!!locked} onclick={() => act(() => insertChart('pie'))}><Icon name="chart-pie" size={14} />Graphique en secteurs</button>
			<button disabled={!!locked} onclick={() => act(() => insertChart('scatter'))}><Icon name="chart-scatter" size={14} />Nuage de points</button>
			<button disabled={!!locked} onclick={(e) => { e.stopPropagation(); moreCharts = !moreCharts; }}><Icon name="chart-more" size={14} />Autres graphiques {moreCharts ? '▾' : '▸'}</button>
			{#if moreCharts}
				{#each MORE_KINDS as k (k.kind)}
					<button class="sub" disabled={!!locked} title={k.hint} onclick={() => act(() => insertChart(k.kind))}><Icon name={k.icon} size={14} />{k.label}</button>
				{/each}
			{/if}
			<button disabled={!!locked} onclick={() => act(insertPivot)}><Icon name="pivot-table" size={14} />Tableau croisé dynamique</button>
			<button disabled={!!locked} onclick={() => act(insertButton)}><Icon name="button" size={14} />Bouton</button>
			{#if controls}<button disabled={!!locked} onclick={() => act(() => insertControl('input'))}><Icon name="text-input" size={14} />Champ (texte, case, liste, fichier)…</button>{/if}
			<button disabled={!!locked} onclick={() => act(() => command('image'))}><Icon name="image" size={14} />Image…</button>
			<button disabled={!!locked} onclick={() => act(() => command('link'))}><Icon name="link" size={14} />Lien…</button>
		{:else}
			{@const label = m.kind === 'chart' ? 'Graphique' : m.kind === 'pivot' ? 'Tableau croisé' : m.kind === 'control' ? 'Contrôle' : 'Bouton'}
			<div class="menu-title">{label}</div>
			{#if m.kind === 'button'}
				{@const b = embedAt(m.index, BUTTON_BLOT) as DocButtonConfig | null}
				{#if b?.script}<button onclick={() => act(() => runButton(b))}>Exécuter l’action</button>{/if}
			{/if}
			<button onclick={() => act(() => (m.kind === 'control' ? editControl(m.index) : openPanel({ kind: m.kind as 'chart' | 'pivot' | 'button', index: m.index })))}>Modifier…</button>
			<hr />
			<button disabled={!!locked} onclick={() => act(() => copyItem(m.kind as 'chart', m.index, true))}>Couper</button>
			<button onclick={() => act(() => copyItem(m.kind as 'chart', m.index, false))}>Copier</button>
			<button disabled={!clip || !!locked} onclick={() => act(() => pasteItem(m.index + 1))}>Coller l’élément après</button>
			<hr />
			<button disabled={!!locked} onclick={() => act(() => deleteEmbed(m.index))}>Supprimer {m.kind === 'chart' ? 'le graphique' : m.kind === 'pivot' ? 'le tableau croisé' : m.kind === 'control' ? 'le contrôle' : 'le bouton'}</button>
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
	.menu-hint {
		padding: 0 14px 4px;
		font-size: 11px;
		color: #6b7280;
	}
	.menu button.sub {
		padding-left: 28px;
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
	.grip {
		position: absolute;
		width: 14px;
		height: 14px;
		box-sizing: border-box;
		background: #fff;
		border: 1.5px solid #217346;
		border-radius: 2px;
		cursor: nwse-resize;
		z-index: 6;
		touch-action: none;
	}
	.page {
		position: relative;
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
	.page :global(.ql-editor img.picked) {
		outline: 2px solid #217346;
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
		display: inline-flex;
		align-items: center;
		justify-content: center;
		vertical-align: middle;
		box-sizing: border-box;
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
		box-sizing: border-box;
	}
	.form {
		box-sizing: border-box;
	}
	.panelbox {
		flex: none;
		overflow: hidden;
	}
	.side :global(.panel) {
		flex: 1;
		min-height: 0;
		width: auto !important;
		flex-basis: auto !important;
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
	.link {
		display: flex;
		gap: 8px;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 3px;
		color: #4b5563;
	}
	.seg {
		display: flex;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		overflow: hidden;
	}
	.seg button {
		flex: 1;
		justify-content: center;
		border: none;
		border-radius: 0;
		padding: 4px 8px;
		white-space: nowrap;
	}
	.seg button + button {
		border-left: 1px solid #c9cdd2;
	}
	.seg button.on {
		background: #e2efe7;
		color: #1b5e3a;
		font-weight: 600;
	}
	.link label:first-child {
		flex: 1;
		min-width: 0;
	}
	.link label:last-child input {
		width: 64px;
	}
	.side textarea {
		font: inherit;
		padding: 4px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		resize: vertical;
	}
	.side option.dflt {
		color: #c42b1c;
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
