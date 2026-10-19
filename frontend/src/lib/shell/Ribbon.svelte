<script lang="ts">
	// Ruban façon Excel : onglets en haut, groupes de commandes en dessous.
	// IA, SQL et SCRIPT sont des onglets à part entière : ils prennent la place
	// de la grille (précédence sur GRID). IA n'apparaît que s'il est activé
	// (Fichier › Paramètres › Fonctionnalités). Une commande est grisée faute de
	// classeur, ou par la protection et les droits d'un classeur partagé
	// (ribbon.ts, selon les drapeaux posés à sa définition).

	import { onMount, tick, untrack } from 'svelte';
	import { slide } from 'svelte/transition';
	import { reducedMotion } from '#lib/grid/anim.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { dismiss } from '#lib/grid/dismiss.js';
	import { clearPatch, type CellStyle } from '#lib/grid/style.js';
	import { DEFAULT_PT, FONTS, SIZES } from '#lib/grid/textwrap.js';
	import { ZOOM_LEVELS, zoomLabel } from '#lib/grid/zoom.js';
	import Select from '#lib/ui/Select.svelte';
	import { estimate, fit, groupWidth, type GroupShape, type Measure, type Mode } from './fit';
	import { scriptingEnabled, sharedWorkbooksEnabled } from './settings.svelte.js';
	import { keyLabel, pdfPrinterHint } from '#lib/web/platform.js';
	import type { IconName } from '#lib/icons/icons.js';
	import { kindInfo, MORE_KINDS, type ChartKind } from '#lib/chart/kinds.js';
	import Submenu from '#lib/ui/Submenu.svelte';
	import { alignPatch, COLOR_NAMES, COLORS, gate, protectedThing, tipOf, type Gated } from './ribbon';
	import { COMMAND_TIPS, TAB_TIPS, assignKeyTips, keyChars, keyOf, matchTip } from '#lib/ui/keytips.js';
	import { cycle, focusRegion, isTextField, modalOpen, regionOf, tabbables } from '#lib/ui/focus.js';
	import { cm, FIT_CHOICES, fitLabel, fitted, hasBreaks, MARGIN_LABELS, MARGINS, marginPreset, PAPERS, paperOf, SCALE_CHOICES, scaleLabel, type PageAction, type PageSetup } from './pagesetup';

	export type View = 'grid' | 'sql' | 'script';
	export type FormatTab = 'number' | 'align' | 'font' | 'border' | 'fill';
	export type FreezeAction = 'cell' | 'row' | 'col' | 'none';
	/** Onglet affiché : feuille (grille), page HTML ou Document ; le ruban ne montre que ce qui s'y applique. */
	export type RibbonContext = 'sheet' | 'html' | 'doc';
	/** Commandes du ruban propres à un onglet Document. */
	export type DocCommand =
		| 'undo'
		| 'redo'
		| 'cut'
		| 'copy'
		| 'paste'
		| 'strike'
		| 'link'
		| 'image'
		| 'button'
		| 'pivot'
		| 'pdf'
		| 'input'
		| `block:${'normal' | 'h1' | 'h2' | 'h3' | 'bullet' | 'ordered' | 'quote' | 'code'}`
		| `align:${'left' | 'center' | 'right' | 'justify'}`
		| `chart:${ChartKind}`;

	/** Commande du ruban ; ro, local et createsSheet disent ce qui la grise (ribbon.ts). */
	interface Command extends Gated {
		icon: IconName;
		color?: 'fg' | 'bg'; // commande avec un choix de couleur
		on?: boolean; // commande à bascule, enfoncée
		/** Commande à menu (▾) : ses choix (raccourci affiché à droite ; drapeaux hérités de la commande, sinon les leurs). */
		items?: { label: string; run?: () => void; on?: boolean; keys?: string; ro?: boolean; local?: boolean }[];
		/** Commande qui ouvre son propre menu (▾ affiché, voir anchor). */
		menu?: boolean;
	}
	interface Group {
		title: string;
		commands: Command[];
		/** Lanceur de boîte de dialogue (coin du titre), comme Excel. */
		launch?: () => void;
		launchHint?: string;
		/** Listes Police et Taille en tête du groupe. */
		fonts?: boolean;
		/** Icônes assez parlantes pour perdre leur libellé en premier quand la place manque. */
		iconOk?: boolean;
	}
	const FONTS_W = 134; // largeur des listes Police et Taille (fit.ts)

	export type FileAction = 'new' | 'open' | 'openRemote' | 'share' | 'rights' | 'republish' | 'save' | 'saveAs' | 'import' | 'export' | 'exportSheet' | 'exportXlsx' | 'exportOds' | 'print' | 'about' | 'close' | 'demo' | 'settings' | 'help' | 'report';
	/** Accueil › Édition (Remplissage, Effacer, Trier et filtrer), Mettre sous forme de tableau. */
	export type EditAction = 'fillDown' | 'fillRight' | 'clearContents' | 'clearFilters' | 'formatAsTable';

	export type DataAction = 'csv' | 'xlsx' | 'json' | 'parquet' | 'sqlite' | 'postgres' | 'mysql' | 'sqlserver' | 'lattis' | 'web' | 'table' | 'queries' | 'refreshAll';

	interface Props {
		view: View;
		undo: string; // libellé de l'action à annuler (vide : rien)
		redo: string;
		/** Aucun classeur ouvert (récupération, classeur fermé) : ruban grisé. */
		disabled?: boolean;
		/** Classeurs récents (chemins), pour Fichier › Récents. */
		recent?: string[];
		onrecent?: (path: string) => void;
		onview: (v: View) => void;
		/** Volet de l'Assistant IA ouvert, et sa bascule (bouton « Assistant ✦ », Alt+I). */
		ai?: boolean;
		onai?: () => void;
		onfile: (a: FileAction) => void;
		onundo: () => void;
		onredo: () => void;
		onbench?: () => void;
		onsort?: (desc: boolean) => void;
		/** Mise en forme de la sélection ('toggle' bascule gras, italique, souligné). */
		onstyle?: (patch: Record<string, unknown>) => void;
		onfind?: (replace: boolean) => void;
		/** Mise en forme conditionnelle : menu à ouvrir sous le bouton (coordonnées de la fenêtre). */
		onrules?: (x: number, y: number) => void;
		/** Filtrer la colonne active, n'en montrer que les erreurs (bascule), effacer les filtres du tableau. */
		onfilter?: (what: 'filter' | 'errors' | 'clear') => void;
		/** Tableau de la cellule active filtré, et sa colonne réduite aux erreurs (Erreurs seulement enfoncé). */
		filters?: boolean;
		errorsOnly?: boolean;
		/** La cellule active a une note (Révision › Modifier la note). */
		note?: boolean;
		onedit?: (what: EditAction) => void;
		/** Classeur partagé : la cellule active est dans un tableau publié en lecture seule (le motif). */
		tableReadOnly?: string;
		onrecalc?: () => void;
		ondedup?: () => void;
		/** Données › Convertir : assistant « Convertir le texte en colonnes ». */
		ontextcols?: () => void;
		onpivot?: () => void;
		/** Insertion › Data Lake Lattis (catalogue du serveur). */
		oninsertdata?: () => void;
		onformula?: (what: 'insert' | 'sum' | 'reference') => void;
		onclip?: (what: 'cut' | 'copy' | 'paste') => void;
		onpainter?: () => void;
		onaudit?: (kind: 'show' | 'clear') => void;
		onchart?: (kind: ChartKind) => void;
		oncells?: (op: 'insertRow' | 'deleteRow' | 'insertColumn' | 'deleteColumn') => void;
		/** Quadrillage de la feuille active, et sa bascule. */
		gridlines?: boolean;
		ongridlines?: (show: boolean) => void;
		/** En-têtes A1 (lettres et numéros) toujours affichés, et leur bascule. */
		headings?: boolean;
		onheadings?: (show: boolean) => void;
		/** Onglet affiché (feuille par défaut). */
		context?: RibbonContext;
		/** Document : commandes, et état de la sélection (bloc, alignement, barré, lien). */
		ondoc?: (cmd: DocCommand) => void;
		docState?: { block: string; align: string; strike: boolean; link: boolean; undo?: boolean; redo?: boolean };
		/** Insertion › Bouton : bouton d'action lié à un script. */
		onbutton?: (kind?: '' | 'checkbox' | 'select' | 'input' | 'textarea' | 'file') => void;
		/** Feuille active protégée : les commandes qui la modifient sont grisées. */
		sheetProtected?: boolean;
		/** La feuille active elle-même est protégée (sheetProtected : verrouillée, par elle ou par le classeur). */
		sheetOwnProtected?: boolean;
		/** Structure du classeur protégée (onglets). */
		workbookProtected?: boolean;
		/** Révision › Protéger (ou ôter la protection de) la feuille ou le classeur. */
		onprotect?: (what: 'sheet' | 'workbook') => void;
		/** Données › Obtenir des données : nouvelle requête d'import, volet des requêtes, tout actualiser. */
		ondata?: (what: DataAction) => void;
		/** Format de cellule (Ctrl+1) : lanceurs des groupes Police, Alignement et Nombre. */
		onformat?: (tab: FormatTab) => void;
		/** Fusionner et centrer (bascule), renvoyer à la ligne (bascule). */
		onmerge?: () => void;
		onwrap?: () => void;
		/** Accueil › Cellules › Format : masquer ou afficher les lignes ou colonnes de la sélection. */
		onlines?: (rows: boolean, hide: boolean) => void;
		/** Données › Validation des données (liste déroulante). */
		onvalidation?: () => void;
		/** Révision › Notes. */
		onnote?: (what: 'edit' | 'delete') => void;
		/** Volets figés de la feuille active, et Affichage › Figer les volets. */
		freeze?: { rows: number; cols: number };
		onfreeze?: (what: FreezeAction) => void;
		/** Mise en forme de la cellule active (police, taille, boutons enfoncés) ; merged : c'est une fusion. */
		cellStyle?: CellStyle;
		merged?: boolean;
		/** Zoom de la feuille active (1 : 100 %), et Affichage › Zoom. */
		zoom?: number;
		onzoom?: (z: number) => void;
		/** Cadre de sélection animé, et sa bascule. */
		animate?: boolean;
		onanimate?: (on: boolean) => void;
		/** Classeur partagé sur un Serveur Lattis (Fichier › Publier masqué), et Fichier › Republier permis. */
		shared?: boolean;
		republish?: boolean;
		/** Fichier › Gérer l’accès… (droits du classeur publié : propriétaire ou concepteur). */
		rights?: boolean;
		/** Mise en page de la feuille active, et les commandes de l'onglet Mise en page. */
		pageSetup?: PageSetup;
		onpage?: (a: PageAction) => void;
		/** Sauts de page affichés dans la grille (Affichage › Sauts de page), et leur bascule. */
		pageBreaks?: boolean;
		onpagebreaks?: (on: boolean) => void;
	}

	let { view, undo, redo, disabled = false, recent = [], onrecent, onview, ai = false, onai, onfile, onundo, onredo, onbench, onsort, onstyle, onfind, onrules, onfilter, filters = false, errorsOnly = false, note = false, onedit, tableReadOnly, onrecalc, ondedup, ontextcols, onpivot, oninsertdata, onchart, onformula, onclip, onpainter, onaudit, oncells, gridlines = true, ongridlines, headings = false, onheadings, context = 'sheet', ondoc, docState, onbutton, sheetProtected = false, sheetOwnProtected = sheetProtected, workbookProtected = false, onprotect, ondata, onformat, onmerge, onwrap, onlines, onvalidation, onnote, freeze, onfreeze, cellStyle, merged = false, zoom = 1, onzoom, animate = true, onanimate, shared = false, republish = false, rights = false, pageSetup, onpage, pageBreaks = false, onpagebreaks }: Props = $props();
	const fmt = (tab: FormatTab) => (onformat ? () => onformat(tab) : undefined);
	const lines = (rows: boolean, hide: boolean) => (onlines ? () => onlines(rows, hide) : undefined);
	const frz = (what: FreezeAction) => (onfreeze ? () => onfreeze(what) : undefined);
	const frozen = $derived(!!freeze && (freeze.rows > 0 || freeze.cols > 0));
	// Menu d'une commande à choix (Figer les volets, Format) : son run ne sert
	// qu'à dire qu'elle est active, le clic ouvre ses choix.
	let dropMenu = $state<{ label: string; x: number; y: number } | null>(null);
	const openItems = () => undefined;
	// Bas gauche du dernier bouton cliqué : une commande à menu propre
	// (Mise en forme conditionnelle) l'ouvre là.
	let anchor = { x: 0, y: 0 };
	function anchorAt(ev: MouseEvent) {
		const r = (ev.currentTarget as HTMLElement).getBoundingClientRect();
		anchor = { x: r.left, y: r.bottom + 2 };
	}
	function toggleDrop(c: Command, ev: MouseEvent) {
		const r = (ev.currentTarget as HTMLElement).getBoundingClientRect();
		dropMenu = dropMenu?.label === c.label ? null : { label: c.label, x: r.left, y: r.bottom + 2 };
	}
	let fg = $state('#c00000');
	let bg = $state('#fff2cc');
	const st = (patch: Record<string, unknown>) => (onstyle ? () => onstyle(patch) : undefined);
	let fileOpen = $state(false);
	const fileName = (p: string) => p.split(/[\\/]/).pop() ?? p;
	const data = (what: DataAction) => (ondata ? () => ondata(what) : undefined);
	const cells = (op: 'insertRow' | 'deleteRow' | 'insertColumn' | 'deleteColumn') => (oncells ? () => oncells(op) : undefined);
	// Palette ouverte sous le bouton Couleur ou Remplissage, comme dans Excel.
	let palette = $state<'fg' | 'bg' | null>(null);
	let palettePos = $state({ x: 0, y: 0 });
	function openPalette(kind: 'fg' | 'bg', ev: MouseEvent) {
		const r = (ev.currentTarget as HTMLElement).closest('.color-cmd')!.getBoundingClientRect();
		palettePos = { x: r.left, y: r.bottom + 2 };
		palette = palette === kind ? null : kind;
	}
	const ed = (w: EditAction) => (onedit ? () => onedit(w) : undefined);
	const flt = (w: 'filter' | 'errors' | 'clear') => (onfilter ? () => onfilter(w) : undefined);
	/** Mise en forme seulement : permise sur un tableau partagé en lecture seule (ribbon.ts). */
	const local = (cs: Command[]): Command[] => cs.map((c) => ({ local: true, ...c }));
	function pick(kind: 'fg' | 'bg', c: string | null) {
		palette = null;
		if (c) {
			if (kind === 'fg') fg = c;
			else bg = c;
		}
		onstyle?.({ [kind]: c });
	}

	// Ordre d'Excel.
	const tabs = ['Accueil', 'Insertion', 'Formules', 'Données', 'Révision', 'Affichage'] as const;
	let tab = $state<(typeof tabs)[number] | 'SQL' | 'SCRIPT'>('Accueil');
	/** Onglets qui remplacent la grille (pas de groupes de commandes). */
	const isView = (t: string) => t === 'SQL' || t === 'SCRIPT';
	// Largeur disponible pour les groupes : ils se compactent pour y tenir.
	let groupsW = $state(2000);
	let openGroup = $state<{ title: string; x: number; y: number } | null>(null);
	// Rapport entre largeur réelle et largeur estimée (polices, icônes) :
	// mesuré après chaque rendu, il corrige l'estimation de fit.
	let ratio = $state(1);
	let groupsEl: HTMLElement;
	function observe(node: HTMLElement) {
		groupsEl = node;
		const ro = new ResizeObserver(() => (groupsW = node.clientWidth));
		ro.observe(node);
		groupsW = node.clientWidth;
		return { destroy: () => ro.disconnect() };
	}

	const dc = (cmd: DocCommand) => (ondoc ? () => ondoc(cmd) : undefined);
	// Onglet Mise en page.
	const pg = (a: PageAction) => (onpage ? () => onpage(a) : undefined);
	const ps = $derived(pageSetup ?? { header: {}, footer: {} });
	const mPreset = $derived(marginPreset(ps));
	const marginHint = (k: keyof typeof MARGINS) => {
		const m = MARGINS[k];
		return `${MARGIN_LABELS[k]} — haut et bas : ${cm(m.top)}, gauche et droite : ${cm(m.left)}`;
	};
	const paperLabel = $derived(PAPERS.find((x) => x.id === paperOf(ps))?.label ?? 'A4');
	const blockOn = (b: string) => docState?.block === b;
	/** Commandes qui ne modifient rien (affichage, mise en page, lecture) : jamais grisées. */
	const readOnly = (cs: Command[]): Command[] => cs.map((c) => ({ ro: true, ...c }));
	const readOnlyGroups = (gs: Group[]): Group[] => gs.map((g) => ({ ...g, commands: readOnly(g.commands) }));
	const prot = $derived(protectedThing(context));
	const moreHint = `Autres graphiques : ${MORE_KINDS.map((k) => k.label).join(', ')}`;
	const animateCmd: Command = $derived({ label: 'Animations', icon: 'play', ro: true, on: animate, run: onanimate ? () => onanimate(!animate) : undefined, hint: animate ? 'Désactiver l’animation du cadre de sélection' : 'Animer le cadre de sélection (il glisse vers la nouvelle cellule)' });
	const sheetRibbons: Record<string, Group[]> = $derived({
		Accueil: [
			{
				title: 'Annuler',
				iconOk: true,
				commands: [
					{ label: 'Annuler', icon: 'undo', ro: true, run: undo ? onundo : undefined, hint: undo ? 'Annuler : ' + undo + ' (Ctrl+Z)' : 'Rien à annuler' },
					{ label: 'Rétablir', icon: 'redo', ro: true, run: redo ? onredo : undefined, hint: redo ? 'Rétablir : ' + redo + ' (Ctrl+Y)' : 'Rien à rétablir' }
				]
			},
			{
				title: 'Presse-papiers',
				iconOk: true,
				commands: [
					{ label: 'Coller', icon: 'paste', run: onclip ? () => onclip('paste') : undefined, hint: 'Coller (Ctrl+V)' },
					{ label: 'Couper', icon: 'cut', run: onclip ? () => onclip('cut') : undefined, hint: 'Couper (Ctrl+X)' },
					{ label: 'Copier', icon: 'copy', ro: true, run: onclip ? () => onclip('copy') : undefined, hint: 'Copier (Ctrl+C)' },
					{ label: 'Reproduire', icon: 'format-painter', local: true, run: onpainter, hint: 'Reproduire la mise en forme de la cellule active' }
				]
			},
			{
				title: 'Police',
				fonts: true,
				iconOk: true,
				launch: fmt('font'),
				launchHint: 'Format de cellule : police (Ctrl+1)',
				commands: local([
					{ label: 'Gras', icon: 'bold', run: st({ b: 'toggle' }), hint: 'Gras (Ctrl+B)', on: !!cellStyle?.b },
					{ label: 'Italique', icon: 'italic', run: st({ i: 'toggle' }), hint: 'Italique (Ctrl+I)', on: !!cellStyle?.i },
					{ label: 'Souligné', icon: 'underline', run: st({ u: 'toggle' }), hint: 'Souligné (Ctrl+U)', on: !!cellStyle?.u },
					{ label: 'Couleur de police', icon: 'font-color', run: st({ fg }), hint: 'Couleur de police (▾ : choisir la teinte)', color: 'fg' },
					{ label: 'Couleur de remplissage', icon: 'font-color', run: st({ bg }), hint: 'Couleur de remplissage (▾ : choisir la teinte)', color: 'bg' }
				])
			},
			{
				title: 'Bordures',
				iconOk: true,
				launch: fmt('border'),
				launchHint: 'Format de cellule : bordure (Ctrl+1)',
				commands: local([
					{ label: 'Toutes', icon: 'border-all', run: st({ border: 'all' }), hint: 'Toutes les bordures' },
					{ label: 'Contour', icon: 'border-outline', run: st({ border: 'outline' }), hint: 'Bordure extérieure de la sélection' },
					{ label: 'Aucune', icon: 'border-none', run: st({ border: 'none' }), hint: 'Retirer les bordures' }
				])
			},
			{
				title: 'Alignement',
				iconOk: true,
				launch: fmt('align'),
				launchHint: 'Format de cellule : alignement (Ctrl+1)',
				commands: [
					...local([
						{ label: 'Haut', icon: 'align-top', run: st({ va: 'top' }), hint: 'Aligner en haut', on: cellStyle?.va === 'top' },
						{ label: 'Milieu', icon: 'align-middle', run: st({ va: null }), hint: 'Aligner au milieu', on: !!cellStyle && !cellStyle.va },
						{ label: 'Bas', icon: 'align-bottom', run: st({ va: 'bottom' }), hint: 'Aligner en bas', on: cellStyle?.va === 'bottom' },
						// Comme Excel : cliquer sur l'alignement en cours revient au standard.
						{ label: 'Gauche', icon: 'align-left', run: st(alignPatch(cellStyle?.al, 'left')), hint: cellStyle?.al === 'left' ? 'Aligner à gauche (cliquer : revenir à l’alignement standard)' : 'Aligner à gauche', on: cellStyle?.al === 'left' },
						{ label: 'Centrer', icon: 'align-center', run: st(alignPatch(cellStyle?.al, 'center')), hint: cellStyle?.al === 'center' ? 'Centrer (cliquer : revenir à l’alignement standard)' : 'Centrer', on: cellStyle?.al === 'center' },
						{ label: 'Droite', icon: 'align-right', run: st(alignPatch(cellStyle?.al, 'right')), hint: cellStyle?.al === 'right' ? 'Aligner à droite (cliquer : revenir à l’alignement standard)' : 'Aligner à droite', on: cellStyle?.al === 'right' },
						{ label: 'Renvoyer à la ligne', icon: 'wrap-text', run: onwrap, hint: 'Afficher le texte sur plusieurs lignes dans la cellule', on: !!cellStyle?.wr }
					]),
					{ label: 'Fusionner et centrer', icon: 'merge-cells', run: onmerge, hint: merged ? 'Annuler la fusion des cellules' : 'Fusionner les cellules libres sélectionnées en une seule, contenu centré', on: merged }
				]
			},
			{
				title: 'Nombre',
				iconOk: true,
				launch: fmt('number'),
				launchHint: 'Format de cellule : nombre, date, heure… (Ctrl+1)',
				commands: local([
					{ label: '0,00', icon: 'decimal-fixed', run: st({ nf: '0,00', dp: null, ng: null, df: null }), hint: 'Nombre à deux décimales' },
					{ label: 'Monétaire', icon: 'currency', run: st({ nf: '€', ng: null, df: null }), hint: 'Format monétaire (€)' },
					{ label: 'Pourcentage', icon: 'percent', run: st({ nf: '%', ng: null, df: null }), hint: 'Style de pourcentage (0,25 s’affiche 25 %)' },
					// Les deux boutons de décimales restent côte à côte, dans le ruban
					// large comme empilés (même colonne de trois).
					{ label: 'Ajouter une décimale', icon: 'decimal-more', run: st({ dp: 'more' }), hint: 'Ajouter une décimale à l’affichage' },
					{ label: 'Réduire les décimales', icon: 'decimal-less', run: st({ dp: 'less' }), hint: 'Afficher une décimale de moins' },
					{ label: 'Date', icon: 'type-date', run: st({ nf: 'date', df: 'jj/mm/aaaa', dp: null, ng: null }), hint: 'Date courte (jj/mm/aaaa) ; Ctrl+1 pour les autres formats de date et d’heure' },
					{ label: 'Standard', icon: 'number-format', run: st({ nf: null, dp: null, ng: null, df: null }), hint: 'Format Standard' }
				])
			},
			{
				title: 'Styles',
				commands: [
					{
						label: 'Mise en forme conditionnelle',
						icon: 'conditional-format',
						menu: true,
						local: true,
						run: onrules ? () => onrules(anchor.x, anchor.y) : undefined,
						hint: 'Mise en évidence des cellules, barres de données, nuances de couleurs, jeux d’icônes, gestionnaire des règles'
					},
					{ label: 'Mettre sous forme de tableau', icon: 'table', run: ed('formatAsTable'), hint: 'Faire un tableau de la plage ou du bloc de cellules de la cellule active, sa première ligne pour en-têtes (Ctrl+L)' }
				]
			},
			{
				title: 'Cellules',
				// Comme Excel : Insérer, Supprimer et Format, chacun avec ses choix.
				commands: [
					{
						label: 'Insérer',
						icon: 'insert-row',
						run: oncells ? openItems : undefined,
						hint: 'Insérer une ligne ou une colonne',
						items: [
							{ label: 'Insérer une ligne (au-dessus)', run: cells('insertRow') },
							{ label: 'Insérer une colonne (à gauche)', run: cells('insertColumn') }
						]
					},
					{
						label: 'Supprimer',
						icon: 'delete-row',
						run: oncells ? openItems : undefined,
						hint: 'Supprimer la ligne ou la colonne de la cellule active',
						items: [
							{ label: 'Supprimer la ligne', run: cells('deleteRow') },
							{ label: 'Supprimer la colonne', run: cells('deleteColumn') }
						]
					},
					{
						label: 'Format',
						icon: 'cell-format',
						local: true,
						run: onformat ? openItems : undefined, // ouvre ses choix
						hint: 'Format de cellule, masquer et afficher',
						items: [
							{ label: 'Format de cellule…', keys: 'Ctrl+1', run: fmt('number') },
							{ label: 'Masquer les lignes', keys: 'Ctrl+9', run: lines(true, true) },
							{ label: 'Masquer les colonnes', keys: 'Ctrl+0', run: lines(false, true) },
							{ label: 'Afficher les lignes', run: lines(true, false) },
							{ label: 'Afficher les colonnes', run: lines(false, false) }
						]
					}
				]
			},
			{
				title: 'Édition',
				// Comme Excel : Somme automatique, Remplissage, Effacer, Trier et filtrer, Rechercher.
				commands: [
					{ label: 'Somme automatique', icon: 'autosum', run: onformula ? () => onformula('sum') : undefined, hint: 'SOMME de la colonne du tableau au-dessus (Alt+=)' },
					{
						label: 'Remplissage',
						icon: 'fill-down',
						run: onedit ? openItems : undefined,
						hint: 'Recopier la première ligne ou colonne de la sélection',
						items: [
							{ label: 'Vers le bas', keys: 'Ctrl+D', run: ed('fillDown') },
							{ label: 'Vers la droite', keys: 'Ctrl+R', run: ed('fillRight') }
						]
					},
					{
						label: 'Effacer',
						icon: 'clear-format',
						run: onstyle ? openItems : undefined,
						hint: 'Effacer les formats, le contenu ou la note de la sélection',
						items: [
							{ label: 'Effacer les formats', local: true, run: st(clearPatch()) },
							{ label: 'Effacer le contenu', keys: 'Suppr', run: ed('clearContents') },
							{ label: 'Effacer la note', local: true, run: note && onnote ? () => onnote('delete') : undefined }
						]
					},
					{
						label: 'Trier et filtrer',
						icon: 'filter',
						run: onsort || onfilter ? openItems : undefined,
						hint: 'Trier le tableau par la colonne active, le filtrer',
						items: [
							{ label: 'Trier de A à Z', local: true, run: onsort ? () => onsort(false) : undefined },
							{ label: 'Trier de Z à A', local: true, run: onsort ? () => onsort(true) : undefined },
							{ label: 'Filtrer', ro: true, keys: 'Ctrl+Maj+L', run: flt('filter') },
							{ label: 'Effacer les filtres', ro: true, run: filters ? flt('clear') : undefined }
						]
					},
					{ label: 'Rechercher', icon: 'search', ro: true, run: onfind ? () => onfind(false) : undefined, hint: 'Rechercher (Ctrl+F)' },
					{ label: 'Remplacer', icon: 'replace', run: onfind ? () => onfind(true) : undefined, hint: 'Remplacer (Ctrl+H)' }
				]
			}
		],
		Insertion: [
			{
				title: 'Tableaux',
				commands: [
					{ label: 'Tableau croisé dynamique', icon: 'pivot-table', ro: true, createsSheet: true, run: onpivot, hint: 'Tableau croisé dynamique du tableau actif, sur une nouvelle feuille (double-clic sur un TCD pour le modifier)' },
					{ label: 'Tableau', icon: 'table', run: ed('formatAsTable'), hint: 'Faire un tableau de la plage ou du bloc de cellules de la cellule active (Ctrl+L)' },
					...(sharedWorkbooksEnabled() ? [{ label: 'Data Lake Lattis', icon: 'database' as const, run: oninsertdata, hint: 'Insérer à la cellule active une donnée du catalogue du Data Lake Lattis : tableau lié, en lecture seule, à jour en direct' }] : [])
				]
			},
			{
				title: 'Graphiques',
				commands: local([
					...(['bar', 'line', 'pie', 'scatter'] as const).map((k) => ({
						label: kindInfo(k)!.label,
						icon: kindInfo(k)!.icon,
						run: onchart ? () => onchart(k) : undefined,
						hint: `${kindInfo(k)!.label} du tableau actif${k === 'bar' ? ' : barres verticales (Alt+F1)' : ''}`
					})),
					{ label: 'Autres graphiques', icon: 'chart-more', run: onchart ? openItems : undefined, hint: moreHint, items: MORE_KINDS.map((k) => ({ label: k.label, run: onchart ? () => onchart(k.kind) : undefined })) }
				])
			},
			{
				title: 'Contrôles',
				commands: local([
					...(scriptingEnabled() ? [{ label: 'Bouton', icon: 'button' as const, run: onbutton ? () => onbutton('') : undefined, hint: 'Bouton d’action à la cellule active : un clic exécute un script de l’onglet Script (ou rien)' }] : []),
					{
						label: 'Champ',
						icon: 'text-input',
						run: onbutton ? () => onbutton('input') : undefined,
						hint: `Contrôle lié à une cellule : champ texte, zone de texte, case à cocher, liste déroulante ou fichier (choix du type dans la fenêtre)${scriptingEnabled() ? ' ; il peut aussi lancer un script' : ''}`
					}
				])
			}
		],
		'Mise en page': readOnlyGroups([
			{
				title: 'Mise en page',
				launch: pg({ kind: 'dialog', tab: 'page' }),
				launchHint: 'Mise en page… : page, marges, en-tête et pied de page, feuille',
				commands: [
					{
						label: 'Marges',
						icon: 'page-margins',
						run: onpage ? openItems : undefined,
						hint: `Marges : ${mPreset === 'custom' ? 'personnalisées' : MARGIN_LABELS[mPreset]}`,
						items: [
							...(Object.keys(MARGINS) as (keyof typeof MARGINS)[]).map((k) => ({ label: marginHint(k), run: pg({ kind: 'margins', preset: k }), on: mPreset === k })),
							{ label: 'Marges personnalisées…', run: pg({ kind: 'dialog', tab: 'margins' }), on: mPreset === 'custom' }
						]
					},
					{
						label: 'Orientation',
						icon: 'page-orientation',
						run: onpage ? openItems : undefined,
						hint: `Orientation : ${ps.landscape ? 'paysage' : 'portrait'}`,
						items: [
							{ label: 'Portrait', run: pg({ kind: 'orientation', landscape: false }), on: !ps.landscape },
							{ label: 'Paysage', run: pg({ kind: 'orientation', landscape: true }), on: !!ps.landscape }
						]
					},
					{
						label: 'Taille',
						icon: 'page-size',
						run: onpage ? openItems : undefined,
						hint: `Format du papier : ${paperLabel}`,
						items: [...PAPERS.map((x) => ({ label: `${x.label} (${x.size})`, run: pg({ kind: 'paper', paper: x.id }), on: paperOf(ps) === x.id })), { label: 'Autres tailles de papier…', run: pg({ kind: 'dialog', tab: 'page' }) }]
					},
					{
						label: 'Zone d’impression',
						icon: 'print-area',
						run: onpage ? openItems : undefined,
						on: !!ps.area,
						hint: ps.area ? 'Une zone d’impression est définie : seule elle s’imprime' : 'Imprimer seulement une partie de la feuille',
						items: [
							{ label: 'Définir la zone d’impression', run: pg({ kind: 'area', clear: false }) },
							{ label: 'Annuler la zone d’impression', run: ps.area ? pg({ kind: 'area', clear: true }) : undefined }
						]
					},
					{
						label: 'Sauts de page',
						icon: 'page-break',
						run: onpage ? openItems : undefined,
						hint: 'Saut de page manuel au-dessus et à gauche de la cellule active (la nouvelle page commence là)',
						items: [
							{ label: 'Insérer un saut de page', run: pg({ kind: 'break', op: 'insert' }) },
							{ label: 'Supprimer le saut de page', run: hasBreaks(ps) ? pg({ kind: 'break', op: 'remove' }) : undefined },
							{ label: 'Rétablir tous les sauts de page', run: hasBreaks(ps) ? pg({ kind: 'break', op: 'reset' }) : undefined }
						]
					},
					{ label: 'Imprimer les titres', icon: 'print-titles', run: pg({ kind: 'dialog', tab: 'sheet' }), on: !!(ps.titleRows || ps.titleCols), hint: 'Lignes à répéter en haut et colonnes à répéter à gauche de chaque page imprimée' }
				]
			},
			{
				title: 'Mise à l’échelle',
				launch: pg({ kind: 'dialog', tab: 'page' }),
				launchHint: 'Mise en page… : échelle',
				commands: [
					{ label: 'Largeur', icon: 'fit-width', run: onpage ? openItems : undefined, hint: `Largeur : ${fitLabel(ps.fitWide ?? 0)} (réduit la feuille pour qu’elle tienne en ce nombre de pages)`, items: FIT_CHOICES.map((n) => ({ label: fitLabel(n), run: pg({ kind: 'fit', wide: true, n }), on: (ps.fitWide ?? 0) === n })) },
					{ label: 'Hauteur', icon: 'fit-height', run: onpage ? openItems : undefined, hint: `Hauteur : ${fitLabel(ps.fitTall ?? 0)}`, items: FIT_CHOICES.map((n) => ({ label: fitLabel(n), run: pg({ kind: 'fit', wide: false, n }), on: (ps.fitTall ?? 0) === n })) },
					{
						label: 'Échelle',
						icon: 'zoom',
						run: onpage ? openItems : undefined,
						hint: fitted(ps) ? `${scaleLabel(ps)} : choisir une échelle arrête l’ajustement` : `Échelle d’impression : ${scaleLabel(ps)}`,
						items: [...SCALE_CHOICES.map((n) => ({ label: `${n} %`, run: pg({ kind: 'scale', pct: n }), on: !fitted(ps) && (ps.scale || 100) === n })), { label: 'Personnalisée…', run: pg({ kind: 'dialog', tab: 'page' }) }]
					}
				]
			},
			{
				title: 'Options de la feuille',
				launch: pg({ kind: 'dialog', tab: 'sheet' }),
				launchHint: 'Mise en page… : feuille',
				iconOk: true,
				commands: [
					{ label: 'Afficher le quadrillage', icon: 'border-all', on: gridlines, run: ongridlines ? () => ongridlines(!gridlines) : undefined, hint: gridlines ? 'Masquer le quadrillage de la feuille à l’écran' : 'Afficher le quadrillage de la feuille à l’écran' },
					{ label: 'Imprimer le quadrillage', icon: 'printer', on: !!ps.gridlines, run: pg({ kind: 'print', what: 'gridlines', on: !ps.gridlines }), hint: 'Imprimer le quadrillage de toutes les cellules' },
					{ label: 'Afficher les en-têtes', icon: 'headings', on: headings, run: onheadings ? () => onheadings(!headings) : undefined, hint: headings ? 'Lettres et numéros seulement pendant la saisie d’une formule' : 'Toujours afficher les lettres de colonnes et numéros de lignes' },
					{ label: 'Imprimer les en-têtes', icon: 'printer', on: !!ps.headings, run: pg({ kind: 'print', what: 'headings', on: !ps.headings }), hint: 'Imprimer les lettres de colonnes et les numéros de lignes' }
				]
			}
		]),
		Données: [
			{
				title: 'Obtenir des données',
				// Une requête s'ajoute même sur une feuille protégée (le moteur vérifie où elle écrit).
				commands: readOnly([
					{ label: 'Texte/CSV', icon: 'file-csv', run: data('csv'), hint: 'Nouvelle requête à partir d’un fichier CSV (étapes de transformation, actualisable)' },
					{ label: 'Classeur Excel', icon: 'table', run: data('xlsx'), hint: 'Nouvelle requête à partir d’un tableau d’un classeur .xlsx' },
					{
						label: 'Autres fichiers',
						icon: 'open',
						run: ondata ? openItems : undefined,
						hint: 'Nouvelle requête à partir d’un fichier JSON, Parquet, ou d’une base SQLite',
						items: [
							{ label: 'JSON', run: data('json') },
							{ label: 'Parquet', run: data('parquet') },
							{ label: 'Base SQLite', run: data('sqlite') }
						]
					},
					{
						label: 'Base de données',
						icon: 'database',
						run: ondata ? openItems : undefined,
						hint: 'Nouvelle requête sur une base de données (lecture seule)',
						items: [
							{ label: 'PostgreSQL', run: data('postgres') },
							{ label: 'MySQL / MariaDB', run: data('mysql') },
							{ label: 'SQL Server', run: data('sqlserver') }
						]
					},
					// Serveur Lattis : bouton à lui, à côté des bases de données (comme les sources propres d'Excel).
					...(sharedWorkbooksEnabled() ? [{ label: 'Serveur Lattis', icon: 'link' as const, run: data('lattis'), hint: 'Nouvelle requête sur un tableau d’un Serveur Lattis ou une donnée de son Data Lake (lecture seule)' }] : []),
					{ label: 'Web', icon: 'web', run: data('web'), hint: 'Nouvelle requête à partir d’une API web qui répond en JSON (lecture seule)' },
					{ label: 'À partir du tableau', icon: 'sheet', run: data('table'), hint: 'Nouvelle requête à partir d’un tableau du classeur' }
				])
			},
			{
				title: 'Requêtes et connexions',
				commands: readOnly([
					{ label: 'Requêtes et connexions', icon: 'query', run: data('queries'), hint: 'Volet des requêtes d’import et des connexions du classeur' },
					{ label: 'Actualiser tout', icon: 'refresh', run: data('refreshAll'), hint: 'Relire les sources de toutes les requêtes chargées, remplacer leurs tableaux et recalculer (Ctrl+Alt+F5)' }
				])
			},
			{
				title: 'Trier et filtrer',
				commands: [
					{ label: 'Trier A → Z', icon: 'sort-asc', local: true, run: onsort ? () => onsort(false) : undefined, hint: 'Trier le tableau par la colonne active, croissant' },
					{ label: 'Trier Z → A', icon: 'sort-desc', local: true, run: onsort ? () => onsort(true) : undefined, hint: 'Trier le tableau par la colonne active, décroissant' },
					{ label: 'Filtrer', icon: 'filter', ro: true, run: flt('filter'), hint: 'Filtrer le tableau par la colonne active (Ctrl+Maj+L)' },
					{ label: 'Effacer', icon: 'clear-format', ro: true, run: filters ? flt('clear') : undefined, hint: filters ? 'Effacer les filtres du tableau actif' : 'Effacer : le tableau de la cellule active n’est pas filtré' },
					{ label: 'Erreurs seulement', icon: 'warning', ro: true, on: errorsOnly, run: flt('errors'), hint: errorsOnly ? 'Afficher de nouveau toutes les lignes de la colonne active' : 'Afficher seulement les saisies hors type de la colonne active' }
				]
			},
			{
				title: 'Outils de données',
				commands: [
					{ label: 'Supprimer les doublons', icon: 'remove-duplicates', run: ondedup, hint: 'Supprimer les lignes en double du tableau actif' },
					{ label: 'Convertir', icon: 'text-columns', run: ontextcols, hint: 'Convertir le texte en colonnes : couper le contenu de la colonne sélectionnée (séparateurs ou largeur fixe) en plusieurs colonnes' },
					{ label: 'Validation des données', icon: 'validation', local: true, run: onvalidation, hint: 'Liste déroulante : valeurs permises dans la sélection (ou toute une colonne de tableau), les autres saisies sont refusées' }
				]
			}
		],
		Formules: [
			{
				title: 'Bibliothèque',
				commands: [
					{ label: 'Insérer une fonction', icon: 'function', run: onformula ? () => onformula('insert') : undefined, hint: 'Choisir une fonction et commencer la formule dans la cellule active (Maj+F3)' },
					{ label: 'Somme automatique', icon: 'autosum', run: onformula ? () => onformula('sum') : undefined, hint: 'SOMME de la colonne du tableau au-dessus (Alt+=)' },
					{ label: 'REFERENCE', icon: 'reference', run: onformula ? () => onformula('reference') : undefined, hint: 'REFERENCE(Tableau[Colonne]; clé) : la RECHERCHEV de Lattis' }
				]
			},
			{
				title: 'Audit',
				commands: readOnly([
					{ label: 'Afficher les liens', icon: 'dependency-graph', run: onaudit ? () => onaudit('show') : undefined, hint: 'Flèches des liens de toutes les cellules sélectionnées : bleues depuis ce que lisent leurs formules, rouges vers les formules qui les lisent ; elles partent dès que la sélection change' },
					{ label: 'Supprimer les flèches', icon: 'trash', run: onaudit ? () => onaudit('clear') : undefined, hint: 'Retirer les flèches des liens (Échap)' }
				])
			},
			{ title: 'Calcul', commands: [{ label: 'Calculer', icon: 'calculate', ro: true, run: onrecalc, hint: 'Recalculer les formules et renouveler AUJOURDHUI, MAINTENANT et ALEA (F9, ou F5 hors d’un tableau importé)' }] }
		],
		Révision: [
			{
				title: 'Notes',
				commands: local([
					{ label: note ? 'Modifier la note' : 'Nouvelle note', icon: 'note', run: onnote ? () => onnote('edit') : undefined, hint: note ? 'Modifier la note de la cellule active (Maj+F2)' : 'Note sur la cellule active : coin rouge, affichée au survol (Maj+F2)' },
					{ label: 'Supprimer la note', icon: 'trash', run: onnote && note ? () => onnote('delete') : undefined, hint: note ? 'Supprimer la note de la cellule active' : 'Supprimer la note : la cellule active n’en a pas' }
				])
			},
			{
				title: 'Protéger',
				commands: readOnly([
					{ label: sheetOwnProtected ? prot.unprotect : prot.protect, icon: 'lock-sheet', on: sheetOwnProtected, run: onprotect ? () => onprotect('sheet') : undefined, hint: sheetOwnProtected ? prot.unprotect : `${prot.protect} : plus aucune modification de son contenu (mot de passe facultatif)` },
					{ label: workbookProtected ? 'Ôter la protection du classeur' : 'Protéger le classeur', icon: 'lock-book', on: workbookProtected, run: onprotect ? () => onprotect('workbook') : undefined, hint: workbookProtected ? 'Ôter la protection de la structure du classeur' : 'Protéger la structure du classeur : ajouter, supprimer, renommer ou déplacer des feuilles est refusé' }
				])
			}
		],
		Affichage: [
			{
				title: 'Afficher',
				commands: readOnly([
					{ label: 'Quadrillage', icon: 'gridlines', on: gridlines, run: ongridlines ? () => ongridlines(!gridlines) : undefined, hint: gridlines ? 'Masquer le quadrillage de la feuille' : 'Afficher le quadrillage de la feuille' },
					{ label: 'En-têtes A1', icon: 'headings', on: headings, run: onheadings ? () => onheadings(!headings) : undefined, hint: headings ? 'Lettres et numéros seulement pendant la saisie d’une formule' : 'Toujours afficher les lettres de colonnes et numéros de lignes' },
					animateCmd,
					{
						label: 'Figer les volets',
						icon: 'freeze-panes',
						run: onfreeze ? openItems : undefined,
						on: frozen,
						hint: frozen ? `Volets figés : ${freeze!.rows} ligne(s), ${freeze!.cols} colonne(s)` : 'Garder des lignes et des colonnes affichées pendant le défilement',
						items: [
							...(frozen ? [{ label: 'Libérer les volets', run: frz('none') }] : []),
							{ label: 'Figer les volets (à la cellule active)', run: frz('cell'), on: frozen && !(freeze!.rows === 1 && freeze!.cols === 0) && !(freeze!.rows === 0 && freeze!.cols === 1) },
							{ label: 'Figer la ligne supérieure', run: frz('row'), on: freeze?.rows === 1 && freeze.cols === 0 },
							{ label: 'Figer la première colonne', run: frz('col'), on: freeze?.rows === 0 && freeze.cols === 1 }
						]
					},
					{
						label: 'Zoom',
						icon: 'zoom',
						run: onzoom ? openItems : undefined,
						on: Math.abs(zoom - 1) > 0.001,
						hint: `Zoom de la feuille : ${zoomLabel(zoom)} (Ctrl+molette, ou le curseur de la barre d’état)`,
						items: ZOOM_LEVELS.map((z) => ({ label: zoomLabel(z), run: onzoom ? () => onzoom(z) : undefined, on: Math.abs(zoom - z) < 0.001 }))
					},
					{ label: '100 %', icon: 'zoom-100', run: onzoom ? () => onzoom(1) : undefined, hint: 'Revenir au zoom 100 %' },
					{ label: 'Sauts de page', icon: 'page-break', on: pageBreaks, run: onpagebreaks ? () => onpagebreaks(!pageBreaks) : undefined, hint: pageBreaks ? 'Masquer les sauts de page (traits pointillés) et la zone d’impression' : 'Afficher les sauts de page de l’impression (traits pointillés) et la zone d’impression' }
				])
			},
			...(onbench ? [{ title: 'Mesures (prototype)', commands: readOnly([{ label: 'Mesurer la grille', icon: 'stopwatch', run: () => onbench() }]) }] : [])
		]
	});

	// Document : un traitement de texte (police, paragraphes,
	// graphiques et boutons insérés dans le texte).
	const docRibbons: Record<string, Group[]> = $derived({
		Accueil: [
			{
				title: 'Annuler',
				iconOk: true,
				commands: [
					{ label: 'Annuler', icon: 'undo', run: docState?.undo === false ? undefined : dc('undo'), hint: docState?.undo === false ? 'Rien à annuler' : 'Annuler la dernière modification du texte (Ctrl+Z)' },
					{ label: 'Rétablir', icon: 'redo', run: docState?.redo === false ? undefined : dc('redo'), hint: docState?.redo === false ? 'Rien à rétablir' : 'Rétablir (Ctrl+Y)' }
				]
			},
			{
				title: 'Presse-papiers',
				iconOk: true,
				commands: [
					{ label: 'Coller', icon: 'paste', run: dc('paste'), hint: 'Coller le texte (Ctrl+V)' },
					{ label: 'Couper', icon: 'cut', run: dc('cut'), hint: 'Couper (Ctrl+X)' },
					{ label: 'Copier', icon: 'copy', ro: true, run: dc('copy'), hint: 'Copier (Ctrl+C)' }
				]
			},
			{
				title: 'Police',
				fonts: true,
				iconOk: true,
				commands: [
					{ label: 'Gras', icon: 'bold', run: st({ b: 'toggle' }), hint: 'Gras (Ctrl+B)', on: !!cellStyle?.b },
					{ label: 'Italique', icon: 'italic', run: st({ i: 'toggle' }), hint: 'Italique (Ctrl+I)', on: !!cellStyle?.i },
					{ label: 'Souligné', icon: 'underline', run: st({ u: 'toggle' }), hint: 'Souligné (Ctrl+U)', on: !!cellStyle?.u },
					{ label: 'Barré', icon: 'strike', run: dc('strike'), hint: 'Barré', on: !!docState?.strike },
					{ label: 'Couleur de police', icon: 'font-color', run: st({ fg }), hint: 'Couleur de police (▾ : choisir la teinte)', color: 'fg' },
					{ label: 'Surlignage', icon: 'font-color', run: st({ bg }), hint: 'Couleur de surlignage (▾ : choisir la teinte)', color: 'bg' },
					{ label: 'Effacer', icon: 'clear-format', run: st(clearPatch()), hint: 'Effacer la mise en forme des caractères' }
				]
			},
			{
				title: 'Paragraphe',
				iconOk: true,
				commands: [
					{ label: 'Puces', icon: 'list-bullet', run: dc('block:bullet'), on: blockOn('bullet'), hint: 'Liste à puces' },
					{ label: 'Numéros', icon: 'list-number', run: dc('block:ordered'), on: blockOn('ordered'), hint: 'Liste numérotée' },
					{ label: 'Citation', icon: 'quote', run: dc('block:quote'), on: blockOn('quote'), hint: 'Citation (retrait et trait à gauche)' },
					{ label: 'Gauche', icon: 'align-left', run: dc('align:left'), on: docState?.align === 'left', hint: 'Aligner à gauche' },
					{ label: 'Centrer', icon: 'align-center', run: dc('align:center'), on: docState?.align === 'center', hint: 'Centrer' },
					{ label: 'Droite', icon: 'align-right', run: dc('align:right'), on: docState?.align === 'right', hint: 'Aligner à droite' },
					{ label: 'Justifier', icon: 'align-justify', run: dc('align:justify'), on: docState?.align === 'justify', hint: 'Justifier' },
					{ label: 'Code', icon: 'code-block', run: dc('block:code'), on: blockOn('code'), hint: 'Bloc de code (police à chasse fixe)' }
				]
			},
			{
				title: 'Styles',
				commands: [
					{
						label: 'Style',
						icon: 'heading',
						run: ondoc ? openItems : undefined,
						hint: 'Style du paragraphe : normal ou titre',
						items: [
							{ label: 'Normal', run: dc('block:normal'), on: blockOn('normal') },
							{ label: 'Titre 1', run: dc('block:h1'), on: blockOn('h1') },
							{ label: 'Titre 2', run: dc('block:h2'), on: blockOn('h2') },
							{ label: 'Titre 3', run: dc('block:h3'), on: blockOn('h3') }
						]
					}
				]
			},
			// Imprimer ne modifie rien : permis sur un document protégé.
			{ title: 'Imprimer', commands: [{ label: 'Imprimer / PDF', icon: 'printer', ro: true, run: dc('pdf'), hint: `Imprimer le document, ou l’enregistrer en PDF (${pdfPrinterHint()}), graphiques et tableaux croisés compris` }] }
		],
		Insertion: [
			// Le tableau croisé d'un Document est inséré dans le texte (il ne crée pas de feuille).
			{ title: 'Tableaux', commands: [{ label: 'Tableau croisé dynamique', icon: 'pivot-table', run: dc('pivot'), hint: 'Tableau croisé dynamique d’un tableau du classeur, inséré dans le texte et recalculé avec lui' }] },
			{
				title: 'Graphiques',
				commands: [
					...(['bar', 'line', 'pie', 'scatter'] as const).map((k) => ({ label: kindInfo(k)!.label, icon: kindInfo(k)!.icon, run: dc(`chart:${k}`), hint: `${kindInfo(k)!.label} d’un tableau du classeur, inséré dans le texte` })),
					{ label: 'Autres graphiques', icon: 'chart-more', run: ondoc ? openItems : undefined, hint: moreHint, items: MORE_KINDS.map((k) => ({ label: k.label, run: dc(`chart:${k.kind}`) })) }
				]
			},
			{
				title: 'Contrôles',
				commands: [
					...(scriptingEnabled() ? [{ label: 'Bouton', icon: 'button' as const, run: dc('button'), hint: 'Bouton dans le texte : un clic exécute un script de l’onglet Script (ou l’une de ses fonctions)' }] : []),
					{ label: 'Champ', icon: 'text-input', run: dc('input'), hint: `Contrôle lié à une cellule d’une feuille : champ texte, zone de texte, case à cocher, liste déroulante ou fichier${scriptingEnabled() ? ', pilotable par les scripts (controls.*)' : ''}` }
				]
			},
			{
				title: 'Liens',
				commands: [
					{ label: 'Lien', icon: 'link', run: dc('link'), on: !!docState?.link, hint: 'Lien vers une page web (Ctrl+clic pour l’ouvrir)' },
					{ label: 'Image', icon: 'image', run: dc('image'), hint: 'Insérer une image (enregistrée dans le classeur)' }
				]
			}
		],
		Révision: sheetRibbons.Révision.filter((g) => g.title === 'Protéger'),
		Affichage: [{ title: 'Afficher', commands: [animateCmd] }]
	});
	// Page web : Annuler, Protéger et Animations (l'éditeur de la page a ses propres commandes).
	const htmlRibbons: Record<string, Group[]> = $derived({
		Accueil: sheetRibbons.Accueil.filter((g) => g.title === 'Annuler'),
		Révision: sheetRibbons.Révision.filter((g) => g.title === 'Protéger'),
		Affichage: docRibbons.Affichage
	});
	// Mise en page : des groupes de l'onglet Affichage (demande de Louis), sans
	// refaire ses cases « Afficher » (quadrillage, en-têtes) : seulement l'impression.
	const sheetWithPage: Record<string, Group[]> = $derived({
		...sheetRibbons,
		Affichage: [
			...sheetRibbons.Affichage,
			...(sheetRibbons['Mise en page'] ?? []).map((g) =>
				g.title === 'Options de la feuille' ? { ...g, title: 'Impression', commands: g.commands.filter((c) => c.label.startsWith('Imprimer')) } : g
			)
		]
	});
	const allRibbons: Record<string, Group[]> = $derived(context === 'doc' ? docRibbons : context === 'html' ? htmlRibbons : sheetWithPage);
	// Onglets du ruban qui ont des commandes pour l'onglet affiché.
	const shownTabs = $derived(tabs.filter((t) => (allRibbons[t]?.length ?? 0) > 0));
	$effect(() => {
		if (!isView(tab) && !shownTabs.includes(tab as (typeof tabs)[number])) tab = 'Accueil';
	});

	// Grisage (ribbon.ts) : protection de la feuille ou du Document, du
	// classeur, tableau partagé en lecture seule sous la cellule active.
	const ribbons: Record<string, Group[]> = $derived.by(() => {
		const s = { sheetProtected, sheetOwnProtected, workbookProtected, tableReadOnly: context === 'sheet' ? tableReadOnly : undefined, context };
		return Object.fromEntries(Object.entries(allRibbons).map(([k, gs]) => [k, gs.map((g) => ({ ...g, commands: g.commands.map((c) => gate(c, s)) }))]));
	});

	// Largeur réelle des libellés (police du ruban), pour l'estimation de fit.
	let measureCtx: CanvasRenderingContext2D | null = null;
	const measure: Measure = (s, px) => {
		measureCtx ??= typeof document === 'undefined' ? null : document.createElement('canvas').getContext('2d');
		if (!measureCtx) return estimate(s, px);
		measureCtx.font = `${px}px 'Segoe UI', system-ui, -apple-system, sans-serif`;
		return measureCtx.measureText(s).width;
	};
	// Groupes masqués (clic droit sur le nom d'un groupe), par onglet :
	// « onglet|groupe », gardés sur ce poste. Clic droit dans le vide à droite :
	// les réafficher.
	const HIDE_KEY = 'lattis.ribbon.hidden';
	let hiddenGroups = $state<string[]>(readHidden());
	let hideMenu = $state<{ x: number; y: number; title?: string } | null>(null);
	function readHidden(): string[] {
		try {
			const v = JSON.parse(localStorage.getItem(HIDE_KEY) ?? '[]');
			return Array.isArray(v) ? v.filter((x) => typeof x === 'string') : [];
		} catch {
			return [];
		}
	}
	function setHidden(list: string[]) {
		hiddenGroups = list;
		try {
			localStorage.setItem(HIDE_KEY, JSON.stringify(list));
		} catch {
			// stockage indisponible : le choix vaut pour la session
		}
	}
	const visibleGroups = $derived(isView(tab) ? [] : (ribbons[tab] ?? []).filter((g) => !hiddenGroups.includes(`${tab}|${g.title}`)));
	const hiddenHere = $derived((ribbons[tab] ?? []).filter((g) => hiddenGroups.includes(`${tab}|${g.title}`)).map((g) => g.title));

	type ShapesArr = GroupShape[];
	const shapes = $derived(
		isView(tab)
			? []
			: visibleGroups.map((g) => ({ title: g.title, iconOk: g.iconOk, labels: [...(g.fonts ? [FONTS_W] : []), ...g.commands.map((c) => (c.color ? null : c.items || c.menu ? c.label + ' ▾' : c.label))] }))
	);
	// Les formes ne changent qu'avec les libellés : un clic dans la grille
	// (annulation, quadrillage…) recrée les commandes sans recompacter le
	// ruban ni remesurer (sinon les boutons changent de taille).
	const shapesKey = $derived(JSON.stringify(shapes));
	const modesKey = $derived(
		isView(tab) ? '' : fit(JSON.parse(shapesKey), (groupsW - 12) / ratio, measure).join(',')
	);
	const modes: Mode[] = $derived(modesKey ? (modesKey.split(',') as Mode[]) : []);
	$effect(() => {
		const m = modes;
		if (!groupsEl || !m.length) return;
		const shapes = JSON.parse(untrack(() => shapesKey)) as ShapesArr;
		requestAnimationFrame(() => {
			const actual = [...groupsEl.children].reduce((s, c) => s + c.getBoundingClientRect().width, 0);
			const est = shapes.reduce((s, g, i) => s + groupWidth(g, m[i] ?? 'large', measure), 0);
			// Ruban qui déborde (fenêtre trop étroite même tout compacté) : les
			// groupes rétrécis faussent la mesure, qui ferait croire à de la place
			// (le ruban repassait en grand et ne se recompactait plus). Rapport borné.
			if (groupsEl.scrollWidth > groupsEl.clientWidth + 1) return;
			const r = actual && est ? Math.min(1.5, Math.max(0.7, actual / est)) : 1;
			if (Math.abs(r - ratio) / ratio > 0.03) ratio = r;
		});
	});

	// Ruban réduit (comme Excel : Ctrl+F1, double-clic sur un onglet) : seuls
	// les onglets restent ; un clic sur un onglet déplie ses commandes
	// par-dessus la feuille, repliées après une commande ou un clic ailleurs.
	// Le choix est gardé sur ce poste.
	const FOLD_KEY = 'lattis.ribbon.folded';
	let folded = $state(readFold());
	let peek = $state(false);

	function readFold(): boolean {
		try {
			return localStorage.getItem(FOLD_KEY) === '1';
		} catch {
			return false;
		}
	}

	// Réduire ou épingler : le ruban se replie vers le haut (ou se déplie),
	// pour qu'on voie où il va ; pas d'animation au changement d'onglet.
	let foldAnim = $state(false);

	/** Réduit ou épingle le ruban (Ctrl+F1). */
	export function toggleFold() {
		foldAnim = true;
		setTimeout(() => (foldAnim = false), 400);
		folded = !folded;
		peek = false;
		try {
			localStorage.setItem(FOLD_KEY, folded ? '1' : '0');
		} catch {
			// stockage indisponible : le choix vaut pour la session
		}
	}

	/** Clic dans les commandes dépliées : une commande lancée replie le ruban. */
	function peekClick(e: MouseEvent) {
		const b = (e.target as HTMLElement).closest('button');
		if (!folded || !b || b.hasAttribute('aria-haspopup') || b.classList.contains('fold')) return;
		setTimeout(() => (peek = false));
	}

	/** Ruban déplié par-dessus la feuille : replié sur un clic ailleurs ou Échap. */
	function peekWatch(node: HTMLElement) {
		const away = (e: Event) => {
			const t = e.target as HTMLElement;
			// Les menus des commandes (couleurs, listes) s'ouvrent hors du ruban.
			if (!headerEl.contains(t) && !t.closest?.('.palette, .drop-menu, .group-menu, .file-menu')) peek = false;
		};
		const key = (e: KeyboardEvent) => {
			if (e.key === 'Escape' && !kt) peek = false;
		};
		window.addEventListener('pointerdown', away, true);
		window.addEventListener('keydown', key);
		void node;
		return {
			destroy() {
				window.removeEventListener('pointerdown', away, true);
				window.removeEventListener('keydown', key);
			}
		};
	}

	/** keep : déplier sans replier (KeyTips), au lieu de basculer comme un clic. */
	function select(t: typeof tab, keep = false) {
		peek = folded && !isView(t) && (keep || !(peek && tab === t));
		tab = t;
		onview(t === 'SQL' ? 'sql' : t === 'SCRIPT' ? 'script' : 'grid');
	}

	// --- Clavier : KeyTips (Alt ou F10) et flèches, comme Excel -------------

	let headerEl: HTMLElement;
	type Tip = { el: HTMLElement; tip: string; x: number; y: number };
	/** KeyTips affichées : sur les onglets, puis sur les commandes de l'onglet choisi ; typed : touches déjà tapées. */
	let kt = $state<{ level: 'tabs' | 'cmds'; typed: string; tips: Tip[] } | null>(null);
	/** Élément qui avait le focus avant le ruban (Échap l'y ramène). */
	let returnTo: HTMLElement | null = null;
	let altAlone = false;

	const usable = (el: HTMLElement) => !(el as HTMLButtonElement).disabled && el.getClientRects().length > 0;
	const tabEls = () => [...headerEl.querySelectorAll<HTMLElement>('.tabs .tab')].filter(usable);
	const cmdEls = () => (groupsEl?.isConnected ? tabbables(groupsEl) : []);
	const tipLabel = (el: HTMLElement) => el.dataset.kt || el.getAttribute('aria-label') || el.title || el.textContent?.trim() || '';
	const activeTabEl = () => tabEls().find((el) => el.dataset.label === tab) ?? tabEls()[0];

	function tipsAt(level: 'tabs' | 'cmds'): Tip[] {
		const els = level === 'tabs' ? tabEls() : cmdEls();
		const tabName = (el: HTMLElement) => el.dataset.label ?? el.getAttribute('aria-label') ?? '';
		const codes = level === 'tabs' ? els.map((el) => TAB_TIPS[tabName(el)] ?? keyChars(tabName(el))[0] ?? '') : assignKeyTips(els.map(tipLabel), COMMAND_TIPS);
		return els.map((el, i) => {
			const r = el.getBoundingClientRect();
			return { el, tip: codes[i], x: r.left + r.width / 2, y: r.bottom - (level === 'tabs' ? 10 : 8) };
		});
	}

	function startTips() {
		if (modalOpen()) return;
		const a = document.activeElement as HTMLElement | null;
		if (!headerEl.contains(a)) returnTo = a;
		kt = { level: 'tabs', typed: '', tips: tipsAt('tabs') };
	}

	async function openTab(el: HTMLElement) {
		const t = el.dataset.label ?? '';
		if (!t || t === 'Fichier' || t === 'SQL' || t === 'SCRIPT' || t === 'Assistant ✦') {
			kt = null;
			el.click(); // Fichier : son menu prend le focus (ouvert au clavier) ; ? : l'aide
			return;
		}
		select(t as typeof tab, true);
		await tick();
		// Le ruban se compacte (fit) puis se remesure : deux images plus tard, il est en place.
		for (let i = 0; i < 2; i++) await new Promise((r) => requestAnimationFrame(r));
		const tips = tipsAt('cmds');
		kt = tips.length ? { level: 'cmds', typed: '', tips } : null;
	}

	// Le ruban se recompacte parfois juste après le changement d'onglet (fit
	// remesuré) : les boutons sont remplacés, les KeyTips sont recalculées.
	$effect(() => {
		if (kt?.level !== 'cmds' || !groupsEl) return;
		const mo = new MutationObserver(() => {
			if (kt?.level === 'cmds') kt = { level: 'cmds', typed: '', tips: tipsAt('cmds') };
		});
		mo.observe(groupsEl, { childList: true, subtree: true });
		return () => mo.disconnect();
	});

	function runTip(t: Tip, level: 'tabs' | 'cmds') {
		if (level === 'tabs') return openTab(t.el);
		kt = null;
		// Liste (police, taille) : elle s'ouvre et garde le clavier.
		if (t.el.getAttribute('aria-haspopup') === 'listbox') t.el.focus();
		t.el.click();
	}

	function tipKey(e: KeyboardEvent) {
		if (!kt || ['Shift', 'Control', 'Meta'].includes(e.key)) return;
		e.preventDefault();
		e.stopPropagation();
		if (e.key === 'Alt') {
			altAlone = true; // relâchée seule : les KeyTips disparaissent
			return;
		}
		if (e.key === 'F10') {
			kt = null;
			return;
		}
		if (e.key === 'Escape') {
			if (kt.typed) kt.typed = '';
			else if (kt.level === 'cmds') kt = { level: 'tabs', typed: '', tips: tipsAt('tabs') };
			else kt = null;
			return;
		}
		if (e.key.startsWith('Arrow') || e.key === 'Tab') {
			// Flèches : le focus entre dans le ruban, sur l'onglet actif ou la première commande.
			const level = kt.level;
			kt = null;
			(level === 'cmds' ? cmdEls()[0] : activeTabEl())?.focus();
			return;
		}
		const k = keyOf(e.key);
		if (!k) return;
		// Ruban recompacté entre-temps (boutons remplacés) : codes recalculés.
		if (kt.tips.some((t) => !t.el.isConnected)) kt = { level: kt.level, typed: '', tips: tipsAt(kt.level) };
		const typed = kt.typed + k;
		const m = matchTip(
			kt.tips.map((t) => t.tip),
			typed
		);
		if (m.hit >= 0) runTip(kt.tips[m.hit], kt.level);
		else if (m.live) kt.typed = typed;
	}

	onMount(() => {
		const down = (e: KeyboardEvent) => {
			if (kt) return tipKey(e);
			if (e.key === 'Alt') {
				// Alt seule (ni AltGr, ni Alt+touche) : les KeyTips au relâchement.
				if (!e.repeat) altAlone = !e.ctrlKey && !e.shiftKey && !e.metaKey;
				return;
			}
			altAlone = false;
			if (modalOpen()) return;
			if (e.key === 'F1' && e.ctrlKey && !e.altKey && !e.shiftKey && !e.metaKey) {
				e.preventDefault();
				e.stopPropagation();
				toggleFold();
				return;
			}
			if (e.key === 'F10' && !e.shiftKey && !e.ctrlKey && !e.altKey && !e.metaKey) {
				e.preventDefault();
				e.stopPropagation();
				startTips();
				return;
			}
			// Alt maintenue + lettre d'un onglet (Alt+H) : ses commandes directement.
			if (e.altKey && !e.ctrlKey && !e.metaKey && !e.shiftKey) {
				const k = keyOf(e.key);
				const t = k ? tipsAt('tabs').find((x) => x.tip === k) : undefined;
				if (!t) return;
				e.preventDefault();
				e.stopPropagation();
				const a = document.activeElement as HTMLElement | null;
				if (!headerEl.contains(a)) returnTo = a;
				openTab(t.el);
			}
		};
		const up = (e: KeyboardEvent) => {
			if (e.key !== 'Alt' || !altAlone) return;
			altAlone = false;
			e.preventDefault();
			if (kt) kt = null;
			else startTips();
		};
		const off = () => {
			altAlone = false;
			kt = null;
		};
		window.addEventListener('keydown', down, true);
		window.addEventListener('keyup', up, true);
		window.addEventListener('pointerdown', off, true);
		window.addEventListener('blur', off);
		window.addEventListener('resize', off);
		return () => {
			window.removeEventListener('keydown', down, true);
			window.removeEventListener('keyup', up, true);
			window.removeEventListener('pointerdown', off, true);
			window.removeEventListener('blur', off);
			window.removeEventListener('resize', off);
		};
	});

	/** Flèches dans le ruban : ← → entre onglets (et commandes), ↓ vers les commandes, ↑ vers l'onglet, Échap pour en sortir. */
	function navKey(e: KeyboardEvent) {
		if (kt || e.defaultPrevented || e.ctrlKey || e.altKey || e.metaKey) return;
		const a = document.activeElement as HTMLElement | null;
		if (!a || isTextField(a) || a.closest('.file-menu, .drop-menu, .group-menu, .palette')) return;
		const ts = tabEls();
		const cs = cmdEls();
		const ti = ts.indexOf(a);
		const ci = cs.indexOf(a);
		if (ti < 0 && ci < 0) return;
		if (e.key === 'Escape') {
			e.preventDefault();
			// Comme Excel : retour à la feuille (ou à la page affichée), là où on était.
			if (returnTo?.isConnected && regionOf(returnTo) === 'grid') returnTo.focus();
			else focusRegion('grid');
			return;
		}
		let to: HTMLElement | undefined;
		if (ti >= 0) {
			if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
				to = ts[cycle(ts.length, ti, e.key === 'ArrowRight' ? 1 : -1)];
				// Comme Excel, l'onglet du ruban suit (pas Fichier, ni SQL et SCRIPT qui changent de vue).
				const l = to.dataset.label as (typeof tabs)[number];
				if (tabs.includes(l)) select(l);
			} else if (e.key === 'ArrowDown') to = cs[0];
		} else if (e.key === 'ArrowRight' || e.key === 'ArrowDown') to = cs[cycle(cs.length, ci, 1)];
		else if (e.key === 'ArrowLeft') to = cs[cycle(cs.length, ci, -1)];
		else if (e.key === 'ArrowUp') to = activeTabEl();
		if (!to) return;
		e.preventDefault();
		to.focus();
	}

	$effect(() => {
		// Revenir à la grille depuis la page (onglet de feuille) remet un onglet de ruban.
		if (view === 'grid' && isView(tab)) tab = 'Accueil';
		// Vue changée sans le ruban (retour de la pipette de l'onglet Script).
		else if (view === 'script' && tab !== 'SCRIPT') tab = 'SCRIPT';
		else if (view === 'sql' && tab !== 'SQL') tab = 'SQL';
	});
</script>

{#snippet colorCmd(c: Command)}
	{@const kind = c.color!}
	{@const cur = kind === 'fg' ? fg : bg}
		<div class="color-cmd">
			<button class="cmd" disabled={!c.run} onclick={c.run} title={tipOf(c, disabled)} aria-label="{c.label} : {COLOR_NAMES[cur] ?? cur}" data-kt={c.label}>
				<!-- La lettre montre l'effet : couleur du texte, ou fond rempli. -->
				<span
					class="swatch-icon"
					style:color={kind === 'fg' ? fg : '#1f2328'}
					style:background={kind === 'bg' ? bg : 'transparent'}
					aria-hidden="true">A</span
				>
			</button>
			<button class="drop" disabled={!c.run} onclick={(ev) => openPalette(kind, ev)} title="Choisir la couleur" aria-label="{c.label} : choisir la couleur" aria-haspopup="menu" aria-expanded={palette === kind} data-kt="{c.label} ▾">▾</button>
			{#if palette === kind}
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="palette" role="menu" aria-label={c.label} style:left="{palettePos.x}px" style:top="{palettePos.y}px" use:dismiss={() => (palette = null)}>
					<div class="grid" role="none">
						{#each COLORS as col (col)}
							<button class="chip" role="menuitem" style:background={col} title={COLOR_NAMES[col]} aria-label={COLOR_NAMES[col]} onclick={() => pick(kind, col)}></button>
						{/each}
					</div>
					{#if kind === 'bg'}<button class="row" role="menuitem" onclick={() => pick('bg', null)}>Aucun remplissage</button>{/if}
					{#if kind === 'fg'}<button class="row" role="menuitem" onclick={() => pick('fg', null)}>Automatique</button>{/if}
					<label class="row">
						Autres couleurs…
						<input type="color" value={kind === 'fg' ? fg : bg} onchange={(e) => pick(kind, (e.currentTarget as HTMLInputElement).value)} />
					</label>
				</div>
			{/if}
		</div>
{/snippet}

<!-- svelte-ignore a11y_no_static_element_interactions -->
<header
	class="ribbon"
	bind:this={headerEl}
	data-region="ribbon"
	onkeydown={navKey}
	onfocusin={(e) => {
		const from = e.relatedTarget as HTMLElement | null;
		if (from && !headerEl.contains(from)) returnTo = from;
	}}
>
	<nav class="tabs" aria-label="Onglets du ruban">
		<div class="file-wrap">
			<button class="tab file" data-label="Fichier" aria-haspopup="menu" aria-expanded={fileOpen} aria-keyshortcuts="Alt+F" onclick={() => (fileOpen = !fileOpen)}>Fichier</button>
			{#if fileOpen}
				<!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
				<div class="file-menu" role="menu" aria-label="Fichier" use:dismiss={() => (fileOpen = false)}>
					{#each [['new', 'Nouveau', 'Ctrl+N'], ['open', 'Ouvrir…', 'Ctrl+O']] as [a, label, keys] (a)}
						<button role="menuitem" onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span><span class="keys">{keyLabel(keys)}</span></button>
					{/each}
					<!-- Comme Excel : les fichiers récents juste sous Ouvrir ; sous-menus au survol, par Entrée ou →, ← les referme. -->
					<Submenu label="Ouvrir un fichier récent" disabled={!recent.length} title={recent.length ? undefined : 'Aucun fichier récent'}>
						{#each recent as p (p)}
							<button role="menuitem" title={p} onclick={() => { fileOpen = false; onrecent?.(p); }}><span class="name">{fileName(p)}</span></button>
						{/each}
					</Submenu>
					{#each [...(sharedWorkbooksEnabled() ? [['openRemote', 'Ouvrir depuis un serveur…', '']] : []), ['close', 'Fermer', 'Ctrl+W']] as [a, label, keys] (a)}
						<button role="menuitem" disabled={disabled && a === 'close'} onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span><span class="keys">{keyLabel(keys)}</span></button>
					{/each}
					<hr />
					{#each [['save', 'Enregistrer', 'Ctrl+S'], ['saveAs', 'Enregistrer sous…', 'F12'], ...(sharedWorkbooksEnabled() && !shared ? [['share', 'Publier sur un serveur…', '']] : []), ...(sharedWorkbooksEnabled() && rights ? [['rights', 'Gérer l’accès…', '']] : []), ...(sharedWorkbooksEnabled() && republish ? [['republish', 'Republier…', '']] : []), ['import', 'Importer (CSV, Excel)…', '']] as [a, label, keys] (a)}
						<button role="menuitem" disabled={disabled && a !== 'import'} onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span><span class="keys">{keyLabel(keys)}</span></button>
					{/each}
					<hr />
					<!-- Exporter : CSV du tableau ou de la feuille, classeur .xlsx ou .ods ; l'impression reste à part. -->
					<Submenu label="Exporter" {disabled}>
						{#each [['export', 'Le tableau en CSV…'], ['exportSheet', 'La feuille entière en CSV…'], ['exportXlsx', 'Le classeur Excel (.xlsx)…'], ['exportOds', 'Le classeur OpenDocument (.ods)…']] as [a, label] (a)}
							<button role="menuitem" onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span></button>
						{/each}
					</Submenu>
					<button role="menuitem" {disabled} onclick={() => { fileOpen = false; onfile('print'); }}><span>Imprimer / PDF…</span><span class="keys">{keyLabel('Ctrl+P')}</span></button>
					<hr />
					{#each [['settings', 'Paramètres…', ''], ['help', 'Aide', 'F1'], ['report', 'Signaler un problème…', ''], ['about', 'À propos de Lattis…', ''], ...(import.meta.env.DEV ? [['demo', 'Démo : 1 M de lignes', '']] : [])] as [a, label, keys] (a)}
						<button role="menuitem" onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span><span class="keys">{keyLabel(keys)}</span></button>
					{/each}
				</div>
			{/if}
		</div>
		<div class="tablist" role="tablist" aria-label="Onglets du ruban">
			{#each shownTabs as t, i (t)}
				<button class="tab" id="ribbon-tab-{i}" role="tab" data-label={t} class:on={tab === t} aria-selected={tab === t} aria-controls={tab === t ? 'ribbon-panel' : undefined} tabindex={tab === t ? 0 : -1} aria-keyshortcuts="Alt+{TAB_TIPS[t]}" data-region-focus={tab === t ? '' : undefined} {disabled} onclick={() => select(t)} ondblclick={toggleFold}>{t}</button>
			{/each}
		</div>
		<span class="sep"></span>
		<!-- Bascule du volet de l'Assistant IA (pas un onglet : la grille reste) ;
		     visible même désactivé, le volet propose alors de l'activer. -->
		{#if onai}
			<button class="tab ai" data-label="Assistant ✦" class:on={ai} aria-pressed={ai} aria-keyshortcuts="Alt+{TAB_TIPS['Assistant ✦']}" {disabled} onclick={onai} title={ai ? 'Fermer l’Assistant IA (Alt+I)' : 'Ouvrir l’Assistant IA (Alt+I)'}
				>Assistant <Icon name="sparkle" size={13} /></button
			>
		{/if}
		<button class="tab special" data-label="SQL" class:on={tab === 'SQL'} aria-pressed={tab === 'SQL'} aria-keyshortcuts="Alt+{TAB_TIPS.SQL}" data-region-focus={tab === 'SQL' ? '' : undefined} {disabled} onclick={() => select('SQL')}>SQL</button>
		{#if scriptingEnabled() || view === 'script'}
		<button class="tab special" data-label="Script" class:on={tab === 'SCRIPT'} aria-pressed={tab === 'SCRIPT'} aria-keyshortcuts="Alt+{TAB_TIPS.SCRIPT}" data-region-focus={tab === 'SCRIPT' ? '' : undefined} {disabled} onclick={() => select('SCRIPT')}>Script</button>
		{/if}
		<button class="tab help" title="Aide (F1)" aria-label="Aide" onclick={() => onfile('help')}>?</button>
	</nav>
	{#if !isView(tab) && (!folded || peek)}
		<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
		<div class="fold-wrap" class:peek={folded} use:peekWatch onclick={peekClick} transition:slide={{ duration: foldAnim && !reducedMotion() ? 200 : 0 }}>
		<div class="groups" class:off={disabled} id="ribbon-panel" tabindex="-1" oncontextmenu={(e) => { if (e.target !== e.currentTarget) return; e.preventDefault(); hideMenu = { x: e.clientX, y: e.clientY }; }} role="tabpanel" aria-labelledby="ribbon-tab-{shownTabs.indexOf(tab as (typeof tabs)[number])}" use:observe>
			{#each visibleGroups as g, gi (g.title)}
				{@const mode = modes[gi] ?? 'large'}
				<section class="group" aria-labelledby="ribbon-group-{gi}">
					{#if mode === 'collapsed'}
						<div class="commands">
							<button
								class="cmd"
								data-kt={g.title}
								aria-haspopup="menu"
								aria-expanded={openGroup?.title === g.title}
								onclick={(ev) => {
									const r = (ev.currentTarget as HTMLElement).getBoundingClientRect();
									openGroup = openGroup?.title === g.title ? null : { title: g.title, x: r.left, y: r.bottom + 2 };
								}}
								title={g.title}
							>
								<Icon name={g.commands[0].icon} />
								<span class="label">{g.title} ▾</span>
							</button>
						</div>
						{#if openGroup?.title === g.title}
							<!-- svelte-ignore a11y_no_static_element_interactions -->
							<div class="group-menu" style:left="{openGroup.x}px" style:top="{openGroup.y}px" use:dismiss={() => (openGroup = null)}>
								{#if g.fonts}{@render fontCombos()}{/if}
								{#each g.commands as c (c.label)}
									{#if c.color}
										{@render colorCmd(c)}
									{:else if c.items}
										{#each c.items as it (it.label)}
											<button
												class="cmd row-cmd"
												role="menuitem"
												class:pressed={it.on}
												disabled={!c.run || !it.run}
												title={it.keys ? `${it.label} (${keyLabel(it.keys)})` : undefined}
												onclick={() => {
													openGroup = null;
													it.run?.();
												}}
											>
												<Icon name={c.icon} size={18} />
												<span class="label">{it.label}</span>
											</button>
										{/each}
									{:else}
										<button
											class="cmd row-cmd"
											role="menuitem"
											class:pressed={c.on}
											disabled={!c.run}
											onclick={(ev) => {
												anchorAt(ev);
												openGroup = null;
												c.run?.();
											}}
											title={tipOf(c, disabled)}
										>
											<Icon name={c.icon} size={18} />
											<span class="label">{c.label}</span>
										</button>
									{/if}
								{/each}
							</div>
						{/if}
					{:else}
						<div class="commands" class:stack={mode !== 'large'}>
							{#if g.fonts}{@render fontCombos()}{/if}
							{#each g.commands as c (c.label)}
								{#if c.color}
									{@render colorCmd(c)}
								{:else}
									<button
										class="cmd"
										class:row-cmd={mode !== 'large'}
										class:pressed={c.on}
										aria-pressed={c.on === undefined ? undefined : c.on}
										aria-haspopup={c.items || c.menu ? 'menu' : undefined}
										aria-expanded={c.items ? dropMenu?.label === c.label : undefined}
										aria-label={mode === 'icon' ? c.label : undefined}
										data-kt={c.label}
										disabled={!c.run}
										onclick={c.items
											? (ev) => toggleDrop(c, ev)
											: (ev) => {
													anchorAt(ev);
													c.run?.();
												}}
										title={tipOf(c, disabled)}
									>
										<Icon name={c.icon} size={mode === 'large' ? 24 : 16} />
										{#if mode !== 'icon'}<span class="label">{c.label}{c.items || c.menu ? ' ▾' : ''}</span>{/if}
									</button>
								{/if}
							{/each}
						</div>
					{/if}
					<div class="title" oncontextmenu={(e) => { e.preventDefault(); hideMenu = { x: e.clientX, y: e.clientY, title: g.title }; }}>
						<span id="ribbon-group-{gi}">{g.title}</span>
						{#if g.launch}
							<button class="launch" onclick={g.launch} title={g.launchHint ?? g.title} aria-label={g.launchHint ?? g.title} data-kt="{g.title} …">
								<svg viewBox="0 0 10 10" width="9" height="9" aria-hidden="true"><path d="M1.5 1.5h4M1.5 1.5v4M3.5 3.5l5 5M8.5 4.5v4h-4" fill="none" stroke="currentColor" stroke-width="1.1" /></svg>
							</button>
						{/if}
					</div>
				</section>
			{/each}
		</div>
		<button class="fold" title={folded ? 'Épingler le ruban (Ctrl+F1)' : 'Réduire le ruban : seuls les onglets restent (Ctrl+F1)'} aria-label={folded ? 'Épingler le ruban' : 'Réduire le ruban'} onclick={toggleFold}>
			<svg viewBox="0 0 12 12" width="12" height="12" aria-hidden="true">
				{#if folded}<path d="M3 1.5h6M6 1.5v5M3.5 6.5h5L7 4.5H5zM6 6.5v4" fill="none" stroke="currentColor" stroke-width="1.2" />{:else}<path d="M2.5 7.5 6 4l3.5 3.5" fill="none" stroke="currentColor" stroke-width="1.4" />{/if}
			</svg>
		</button>
		</div>
	{/if}
	{#if hideMenu}
		{@const m = hideMenu}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="drop-menu" role="menu" aria-label="Groupes du ruban" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (hideMenu = null)}>
			{#if m.title}
				<button role="menuitem" onclick={() => { const key = `${tab}|${m.title}`; hideMenu = null; setHidden([...hiddenGroups, key]); }}><span class="item">Masquer le groupe « {m.title} »</span></button>
			{/if}
			{#each hiddenHere as t (t)}
				<button role="menuitem" onclick={() => { hideMenu = null; setHidden(hiddenGroups.filter((x) => x !== `${tab}|${t}`)); }}><span class="item">Réafficher « {t} »</span></button>
			{/each}
			{#if hiddenHere.length > 1}
				<button role="menuitem" onclick={() => { hideMenu = null; setHidden(hiddenGroups.filter((x) => !x.startsWith(`${tab}|`))); }}><span class="item">Tout réafficher</span></button>
			{/if}
			{#if !m.title && !hiddenHere.length}
				<button role="menuitem" disabled><span class="item">Aucun groupe masqué dans cet onglet</span></button>
			{/if}
		</div>
	{/if}
	{#if dropMenu}
		{@const c = ribbons[tab]?.flatMap((g) => g.commands).find((x) => x.label === dropMenu!.label)}
		{#if c?.items}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="drop-menu" role="menu" aria-label={c.label} style:left="{dropMenu.x}px" style:top="{dropMenu.y}px" use:dismiss={() => (dropMenu = null)}>
				{#each c.items as it (it.label)}
					<button
						role={it.on === undefined ? 'menuitem' : 'menuitemcheckbox'}
						aria-checked={it.on === undefined ? undefined : it.on}
						disabled={!it.run}
						class:on={it.on}
						onclick={() => {
							dropMenu = null;
							it.run?.();
						}}><span class="check" aria-hidden="true">{it.on ? '✓' : ''}</span><span class="item">{it.label}</span>{#if it.keys}<span class="keys">{keyLabel(it.keys)}</span>{/if}</button
					>
				{/each}
			</div>
		{/if}
	{/if}
	{#if kt}
		{@const k = kt}
		<div class="keytips" aria-hidden="true">
			{#each k.tips as t, i (i)}
				{#if t.tip && t.tip.startsWith(k.typed)}<span class="keytip" style:left="{t.x}px" style:top="{t.y}px">{t.tip}</span>{/if}
			{/each}
		</div>
	{/if}
</header>

{#snippet fontCombos()}
	<!-- Police et taille de la cellule active, comme Excel. -->
	<div class="fonts">
		<Select
			value={cellStyle?.ff ?? FONTS[0]}
			width="{FONTS_W - 4}px"
			title="Police"
			disabled={!onstyle || sheetProtected}
			options={FONTS.map((f) => ({ value: f, label: f }))}
			onchange={(v) => onstyle?.({ ff: v === FONTS[0] ? null : v })}
		/>
		<Select
			value={cellStyle?.fs ?? DEFAULT_PT}
			width="64px"
			title="Taille de police (points)"
			disabled={!onstyle || sheetProtected}
			options={SIZES.map((s) => ({ value: s, label: String(s) }))}
			onchange={(v) => onstyle?.({ fs: v === DEFAULT_PT ? null : v })}
		/>
	</div>
{/snippet}

<style>
	.ribbon {
		position: relative;
		background: #f3f4f6;
		border-bottom: 1px solid #d4d7db;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
		user-select: none;
	}
	.tabs {
		display: flex;
		align-items: stretch;
		height: 32px;
		background: #217346;
		padding: 0 8px;
		gap: 2px;
	}
	.tab {
		font: inherit;
		color: #e8f3ec;
		background: none;
		border: none;
		padding: 0 14px;
		cursor: pointer;
		border-radius: 4px 4px 0 0;
		margin-top: 4px;
	}
	/* Largeur du texte en gras réservée : passer en gras (onglet actif) ne
	   change pas la largeur, les autres onglets ne bougent pas. */
	.tab::after {
		content: attr(data-label);
		display: block;
		height: 0;
		overflow: hidden;
		visibility: hidden;
		font-weight: 600;
	}
	.tab:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.18);
	}
	.tab.on:hover {
		background: #f3f4f6;
	}
	.tab.on {
		background: #f3f4f6;
		color: #14532d;
		font-weight: 600;
	}
	.tab.file {
		color: #fff;
	}
	/* Focus clavier sur le vert des onglets : contour blanc. */
	.tab:focus-visible {
		outline: 2px solid #fff;
		outline-offset: -3px;
	}
	.tab.on:focus-visible {
		outline-color: #217346;
	}
	/* KeyTips (Alt, F10) : lettre sous chaque onglet ou commande. */
	.keytip {
		position: fixed;
		z-index: 1000;
		transform: translateX(-50%);
		min-width: 10px;
		padding: 1px 4px;
		font: 600 11px/14px 'Segoe UI', system-ui, sans-serif;
		text-align: center;
		color: #fff;
		background: #3c4043;
		border-radius: 3px;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
		pointer-events: none;
	}
	.tab:disabled {
		opacity: 0.6;
		cursor: default;
	}
	.tab.on:disabled {
		opacity: 1;
		color: #6b7280;
	}
	.special {
		letter-spacing: 0.04em;
	}
	/* L'étoile suit le texte sur la même ligne : la barre garde sa hauteur. */
	.tab.ai {
		--icon-accent: #fde68a;
	}
	.tab.ai :global(svg) {
		display: inline-block;
		vertical-align: -2px;
	}
	.tab.ai.on {
		--icon-accent: #d97706;
	}
	.tab.help {
		align-self: center;
		width: 22px;
		height: 22px;
		margin: 0 0 0 6px;
		padding: 0;
		border: 1px solid rgba(255, 255, 255, 0.6);
		border-radius: 50%;
		font-weight: 600;
		color: #fff;
	}
	.tab.help::after {
		content: none;
	}
	.sep {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.file-wrap {
		position: relative;
		display: flex;
	}
	.file-menu {
		position: absolute;
		top: 32px;
		left: 0;
		z-index: 10;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		min-width: 220px;
		padding: 4px 0;
	}
	.file-menu button {
		display: flex;
		justify-content: space-between;
		width: 100%;
		padding: 6px 14px;
		background: none;
		border: none;
		font: inherit;
		cursor: pointer;
		text-align: left;
	}
	.file-menu button:hover:not(:disabled) {
		background: #e3efe7;
	}
	.file-menu button:disabled {
		color: #9aa0a6;
		cursor: default;
	}
	.file-menu hr {
		border: none;
		border-top: 1px solid #e5e7eb;
		margin: 4px 0;
	}
	.file-menu .name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	/* Liste des onglets : ses boutons restent dans la barre (flex de .tabs). */
	.tablist {
		display: contents;
	}
	.groups.off {
		/* Aucun classeur ouvert : rien à mettre en forme. */
		opacity: 0.45;
		pointer-events: none;
	}
	.keys {
		color: #6b7280;
		font-size: 12px;
	}
	/* Ruban réduit : les commandes dépliées passent par-dessus la feuille. */
	.fold-wrap {
		position: relative;
	}
	.fold-wrap.peek {
		position: absolute;
		left: 0;
		right: 0;
		top: 100%;
		z-index: 40;
		background: #f3f4f6;
		border-bottom: 1px solid #d4d7db;
		box-shadow: 0 6px 14px rgba(0, 0, 0, 0.18);
	}
	.fold {
		position: absolute;
		right: 6px;
		bottom: 4px;
		width: 20px;
		height: 20px;
		display: grid;
		place-items: center;
		padding: 0;
		border: 1px solid transparent;
		border-radius: 3px;
		background: transparent;
		color: #4b5563;
		cursor: pointer;
	}
	.fold:hover {
		background: #e2e5e9;
		border-color: #c9cdd2;
	}
	.groups {
		display: flex;
		height: 78px;
		padding: 4px 30px 4px 6px;
		overflow-x: auto;
		scrollbar-width: none; /* défilement à la molette, sans barre */
	}
	.group {
		/* Largeur naturelle : un groupe ne se tasse pas (libellés coupés ou
		   sur deux lignes), fit choisit les modes pour que tout tienne. */
		flex: none;
		display: flex;
		flex-direction: column;
		padding: 0 8px;
		border-right: 1px solid #d4d7db;
	}
	.commands {
		display: flex;
		gap: 2px;
		flex: 1;
	}
	.cmd {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 2px;
		min-width: 54px;
		padding: 2px 6px;
		font: inherit;
		font-size: 11px;
		background: none;
		border: 1px solid transparent;
		border-radius: 3px;
		cursor: pointer;
		color: #1f2328;
	}
	.cmd .label,
	.title {
		white-space: nowrap;
	}
	.cmd:hover:not(:disabled) {
		background: #e2e8e4;
		border-color: #c5d6cb;
	}
	.cmd.pressed {
		background: #d3e7dc;
		box-shadow: inset 0 0 0 1px #8fbc9f;
	}
	.cmd:disabled {
		color: #9aa0a6;
		cursor: default;
	}
	.cmd {
		--icon-accent: #217346;
	}
	.cmd:disabled {
		/* Une commande désactivée perd sa couleur d'accent. */
		--icon-accent: currentColor;
	}
	.commands.stack {
		display: grid;
		grid-template-rows: repeat(3, 1fr);
		grid-auto-flow: column;
		gap: 0 2px;
	}
	.cmd.row-cmd {
		flex-direction: row;
		justify-content: flex-start;
		min-width: 0;
		gap: 6px;
		padding: 1px 6px;
		font-size: 12px;
	}
	.commands.stack .color-cmd {
		grid-row: span 3;
	}
	.group-menu {
		position: fixed;
		z-index: 20;
		display: flex;
		flex-direction: column;
		align-items: stretch;
		gap: 2px;
		padding: 6px;
		background: #f3f4f6;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
	}
	.color-cmd {
		position: relative;
		display: flex;
		flex-direction: column;
	}
	.color-cmd .cmd {
		flex: 1;
	}
	.swatch-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		font: 700 18px/1 'Segoe UI', system-ui, sans-serif;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	.drop {
		font: inherit;
		font-size: 9px;
		line-height: 10px;
		padding: 0;
		background: none;
		border: 1px solid transparent;
		border-radius: 3px;
		cursor: pointer;
		color: #4b5563;
	}
	.drop:hover:not(:disabled) {
		background: #e2e8e4;
		border-color: #c5d6cb;
	}
	.palette {
		position: fixed;
		z-index: 20;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 8px;
		/* Marge invisible : la palette ne se ferme pas au moindre écart. */
		outline: 14px solid transparent;
	}
	.palette .grid {
		display: grid;
		grid-template-columns: repeat(5, 18px);
		gap: 4px;
		margin-bottom: 6px;
	}
	.chip {
		width: 18px;
		height: 18px;
		padding: 0;
		border: 1px solid #c9cdd2;
		cursor: pointer;
	}
	.chip:hover {
		outline: 2px solid #217346;
	}
	.palette .row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		padding: 4px 2px;
		font: inherit;
		font-size: 12px;
		background: none;
		border: none;
		cursor: pointer;
		white-space: nowrap;
		text-align: left;
	}
	.palette .row:hover {
		background: #e3efe7;
	}
	.palette input[type='color'] {
		width: 24px;
		height: 18px;
		padding: 0;
		border: none;
	}
	.title {
		position: relative;
		text-align: center;
		font-size: 11px;
		color: #6b7280;
		padding: 0 12px;
	}
	/* Lanceur de boîte de dialogue, dans le coin du titre (comme Excel). */
	.launch {
		position: absolute;
		right: -6px;
		bottom: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 15px;
		height: 15px;
		padding: 0;
		border: 1px solid transparent;
		border-radius: 2px;
		background: none;
		color: #4b5563;
		cursor: pointer;
	}
	.launch:hover {
		background: #e2e8e4;
		border-color: #c5d6cb;
	}
	.fonts {
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: 4px;
		padding: 0 4px;
	}
	.commands.stack .fonts {
		grid-row: span 3;
	}
	.group-menu .fonts {
		padding: 2px 0 6px;
	}
	.drop-menu {
		position: fixed;
		z-index: 20;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 220px;
	}
	.drop-menu button {
		display: flex;
		width: 100%;
		padding: 6px 14px 6px 6px;
		background: none;
		border: none;
		font: inherit;
		cursor: pointer;
		text-align: left;
	}
	.drop-menu button:hover:not(:disabled) {
		background: #e3efe7;
	}
	.drop-menu button:disabled {
		color: #9aa0a6;
		cursor: default;
	}
	.drop-menu .check {
		width: 18px;
		color: #217346;
	}
	.drop-menu .item {
		flex: 1;
	}
	.drop-menu .keys {
		margin-left: 18px;
	}
</style>
