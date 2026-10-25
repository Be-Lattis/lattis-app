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
	import ColorPalette from '#lib/grid/ColorPalette.svelte';
	import { colorName, resolveColor } from '#lib/grid/colors.js';
	import { tableTheme } from '#lib/grid/theme.svelte.js';
	import { clearPatch, type CellStyle } from '#lib/grid/style.js';
	import { DEFAULT_PT, FONTS, SIZES } from '#lib/grid/textwrap.js';
	import { ZOOM_LEVELS, zoomLabel } from '#lib/grid/zoom.js';
	import Select from '#lib/ui/Select.svelte';
	import { estimate, fit, groupWidth, type GroupShape, type Measure, type Mode, type Size } from './fit';
	import { networkSourcesEnabled, platform, scriptingEnabled, sharedWorkbooksEnabled } from './settings.svelte.js';
	import { lattis } from '#lib/remote/lattis.svelte.js';
	import { keyLabel, pdfPrinterHint } from '#lib/web/platform.js';
	import { isNarrow, revealX } from '#lib/ui/mobile.js';
	import { fontIcon, type IconName } from '#lib/icons/icons.js';
	import { kindInfo, MORE_KINDS, type ChartKind } from '#lib/chart/kinds.js';
	import Submenu from '#lib/ui/Submenu.svelte';
	import type { RecentFile } from '#bindings/internal/bridge/models.js';
	import { recentDetail, recentKey, recentLabel, recentRemote } from './doctitle';
	import { alignPatch, gate, protectedThing, tabLabel, tipOf, type Gated } from './ribbon';
	import { commandTips, TAB_TIPS, assignKeyTips, keyChars, keyOf, matchTip } from '#lib/ui/keytips.js';
	import { cycle, focusRegion, isTextField, modalOpen, regionOf, tabbables } from '#lib/ui/focus.js';
	import { cm, FIT_CHOICES, fitLabel, fitted, hasBreaks, marginLabel, MARGINS, marginPreset, PAPERS, paperOf, SCALE_CHOICES, scaleLabel, type PageAction, type PageSetup } from './pagesetup';
	import { lang, t, tAny } from '#lib/i18n/index.js';
	import frRibbon from '#lib/i18n/messages/fr/ribbon.json';

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
		items?: { label: string; icon?: IconName; run?: () => void; on?: boolean; keys?: string; ro?: boolean; local?: boolean; data?: boolean; swatch?: [string, string]; sep?: boolean }[];
		/** Commande qui ouvre son propre menu (▾ affiché, voir anchor). */
		menu?: boolean;
		/** Forme dans un groupe compact (par défaut petite icône seule) : 'small' petite icône à libellé, 'big' grand bouton (Coller, Fusionner et centrer). */
		size?: Size;
		/** Liste en tête d'un groupe en lignes (Format de nombre) : le choix en cours, affiché dans la liste ; items donne les choix. */
		combo?: string;
	}
	interface Group {
		/** Identifiant stable (groupes masqués, menus) ; title est le libellé affiché. */
		id: string;
		title: string;
		commands: Command[];
		/** Lanceur de boîte de dialogue (coin du titre), comme Excel. */
		launch?: () => void;
		launchHint?: string;
		/** Listes Police et Taille en tête du groupe. */
		fonts?: boolean;
		/** Icônes assez parlantes pour perdre leur libellé en premier quand la place manque. */
		iconOk?: boolean;
		/** Forme la plus large permise (fit.ts) : 'icon' = jamais de grands boutons à libellé. */
		widest?: Mode;
		/** Chaque commande à sa forme (size : petite icône seule par défaut, petite à libellé, grand bouton), comme Excel ; les grandes rapetissent puis tout passe en icônes quand la place manque. */
		compact?: boolean;
		/** Petites icônes rangées ligne par ligne sur ce nombre de lignes (Nombre, Paragraphe) ; une commande combo en tête prend toute la première. */
		rows?: number;
	}
	const FONTS_W = 134; // largeur des listes Police et Taille (fit.ts)
	const COMBO_W = 108; // largeur de la liste Format de nombre (fit.ts)
	/** Forme d'une commande dans un mode : grand bouton, petite icône à libellé, ou icône seule. */
	function sizeOf(g: Group, c: Command, mode: Mode): Size {
		if (mode === 'icon' || g.rows) return 'icon';
		const s: Size = g.compact ? (c.size ?? 'icon') : 'big';
		return mode === 'large' || s !== 'big' ? s : 'small';
	}
	/** Colonnes d'un groupe en lignes : ses icônes réparties sur les lignes laissées par la liste. */
	const rowCols = (g: Group) => {
		const combo = g.commands.some((c) => c.combo !== undefined);
		return Math.ceil((g.commands.length - (combo ? 1 : 0)) / Math.max(1, (g.rows ?? 1) - (combo ? 1 : 0)));
	};
	/** Nom du format de nombre de la cellule active (liste Format de nombre). */
	const NF_NAMES: Record<string, string> = { '0': 'number', '0,00': 'number', '%': 'percent', '€': 'currency', date: 'date', time: 'time', text: 'text' };
	const nfName = (nf: string) => tAny(`ribbon.numberFormat.${NF_NAMES[nf] ?? 'general'}`);

	export type FileAction = 'new' | 'open' | 'openRemote' | 'share' | 'rights' | 'republish' | 'reconnect' | 'detach' | 'remove' | 'save' | 'saveAs' | 'download' | 'import' | 'export' | 'exportSheet' | 'exportXlsx' | 'exportOds' | 'print' | 'about' | 'close' | 'demo' | 'settings' | 'help' | 'report';
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
		recent?: RecentFile[];
		onrecent?: (r: RecentFile) => void;
		onview: (v: View) => void;
		/** Volet de l'Assistant IA ouvert, et sa bascule (bouton « Assistant ✦ », Alt+I). */
		ai?: boolean;
		onai?: () => void;
		onfile: (a: FileAction) => void;
		onundo: () => void;
		onredo: () => void;
		onbench?: () => void;
		onsort?: (desc: boolean) => void;
		/** Tri personnalisé… de la colonne active (ordre des valeurs choisi). */
		onsortcustom?: () => void;
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
		/** Formules › Convertir en valeurs : formules de la sélection (colonnes calculées, cellules libres) remplacées par leurs valeurs. */
		ontovalues?: () => void;
		ondedup?: () => void;
		/** Données › Convertir : assistant « Convertir le texte en colonnes ». */
		ontextcols?: () => void;
		onpivot?: () => void;
		/** Insertion › Data Lake Lattis (catalogue du serveur). */
		oninsertdata?: () => void;
		/** Insertion › Formulaire : partager le tableau actif comme formulaire. */
		onform?: () => void;
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
		/** Structure de la feuille active protégée : seules les commandes de saisie dans les tableaux restent. */
		structureProtected?: boolean;
		/** … par la protection du classeur (structure de toutes les feuilles) : son motif renvoie au classeur. */
		structureByBook?: boolean;
		/** Classeur collaboratif ouvert par un éditeur ou un lecteur : cellules libres et mise en page aux concepteurs. */
		layoutShared?: boolean;
		/** Révision › Protéger (ou ôter la protection de) la feuille, sa structure ou le classeur. */
		onprotect?: (what: 'sheet' | 'structure' | 'workbook') => void;
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
		/** Données › Regrouper des onglets… (onglets aux mêmes colonnes réunis en un seul tableau). */
		onmergesheets?: () => void;
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
		/** Fichier › Réessayer la connexion (classeur partagé hors ligne) et Détacher du serveur… (propriétaire). */
		reconnect?: boolean;
		detach?: boolean;
		/** « Retirer du serveur… » : propriétaire du classeur partagé ouvert (action dangereuse, en rouge). */
		remove?: boolean;
		/** Fichier › Gérer l’accès… (droits du classeur publié : propriétaire ou concepteur). */
		rights?: boolean;
		/** Mise en page de la feuille active, et les commandes de l'onglet Mise en page. */
		pageSetup?: PageSetup;
		onpage?: (a: PageAction) => void;
		/** Sauts de page affichés dans la grille (Affichage › Sauts de page), et leur bascule. */
		pageBreaks?: boolean;
		onpagebreaks?: (on: boolean) => void;
		/** Affichage › Thème : thèmes proposés (on : celui du classeur), et le choix (« custom » : Personnaliser…). */
		themes?: { id: string; name: string; table: string; source: string; on: boolean }[];
		ontheme?: (id: string) => void;
	}

	let { view, undo, redo, disabled = false, recent = [], onrecent, onview, ai = false, onai, onfile, onundo, onredo, onbench, onsort, onsortcustom, onstyle, onfind, onrules, onfilter, filters = false, errorsOnly = false, note = false, onedit, tableReadOnly, onrecalc, ontovalues, ondedup, ontextcols, onpivot, oninsertdata, onform, onchart, onformula, onclip, onpainter, onaudit, oncells, gridlines = true, ongridlines, headings = false, onheadings, context = 'sheet', ondoc, docState, onbutton, sheetProtected = false, sheetOwnProtected = sheetProtected, workbookProtected = false, structureProtected = false, structureByBook = false, layoutShared = false, onprotect, ondata, onformat, onmerge, onwrap, onlines, onvalidation, onmergesheets, onnote, freeze, onfreeze, cellStyle, merged = false, zoom = 1, onzoom, animate = true, onanimate, shared = false, republish = false, reconnect = false, detach = false, remove = false, rights = false, pageSetup, onpage, pageBreaks = false, onpagebreaks, themes = [], ontheme }: Props = $props();
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
	// Dernières couleurs choisies (bouton principal) : #rrggbb ou référence au thème (colors.ts).
	let fg = $state('#c00000');
	let bg = $state('#ffff00');
	const shown = (c: string) => String(resolveColor(c, tableTheme.current.colors));
	const st = (patch: Record<string, unknown>) => (onstyle ? () => onstyle(patch) : undefined);
	let fileOpen = $state(false);
	// Menu Fichier posé sous son onglet, hors de la barre des onglets (qui défile sur un téléphone).
	let filePos = $state({ x: 0, y: 0 });
	function openFile(ev: MouseEvent) {
		const r = (ev.currentTarget as HTMLElement).getBoundingClientRect();
		filePos = { x: Math.max(0, r.left), y: r.bottom };
		fileOpen = !fileOpen;
	}
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

	// Ordre d'Excel. Identifiants des onglets (libellés : ribbon.tabs.*).
	const tabs = ['home', 'insert', 'formulas', 'data', 'review', 'view'] as const;
	type Tab = (typeof tabs)[number] | 'sql' | 'script';
	let tab = $state<Tab>('home');
	/** Onglets qui remplacent la grille (pas de groupes de commandes). */
	const isView = (t: string) => t === 'sql' || t === 'script';
	// Largeur disponible pour les groupes : ils se compactent pour y tenir.
	let groupsW = $state(2000);
	let openGroup = $state<{ id: string; x: number; y: number } | null>(null);
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
	/** Connecté à au moins un Serveur Lattis (bouton en grand dans Données). */
	const lattisSignedIn = $derived(sharedWorkbooksEnabled() && (lattis.state.servers ?? []).some((x) => !!x.user));
	const mPreset = $derived(marginPreset(ps));
	const marginHint = (k: keyof typeof MARGINS) => {
		const m = MARGINS[k];
		return t('ribbon.cmd.margins.preset', { name: marginLabel(k), topBottom: cm(m.top), leftRight: cm(m.left) });
	};
	const paperLabel = $derived(PAPERS.find((x) => x.id === paperOf(ps))?.label ?? 'A4');
	const blockOn = (b: string) => docState?.block === b;
	/** Commandes qui ne modifient rien (affichage, mise en page, lecture) : jamais grisées. */
	const readOnly = (cs: Command[]): Command[] => cs.map((c) => ({ ro: true, ...c }));
	const readOnlyGroups = (gs: Group[]): Group[] => gs.map((g) => ({ ...g, commands: readOnly(g.commands) }));
	const prot = $derived(protectedThing(context));
	const moreHint = $derived(t('ribbon.cmd.moreCharts.tooltip', { list: MORE_KINDS.map((k) => k.label).join(', ') }));
	const animateCmd: Command = $derived({ label: t('ribbon.cmd.animate.label'), icon: 'play', size: 'small', ro: true, on: animate, run: onanimate ? () => onanimate(!animate) : undefined, hint: animate ? t('ribbon.cmd.animate.tooltipOn') : t('ribbon.cmd.animate.tooltipOff') });
	const sheetRibbons: Record<string, Group[]> = $derived({
		home: [
			{
				id: 'undo',
				title: t('ribbon.group.undo'),
				iconOk: true,
				// Annuler et Rétablir l'un sous l'autre, en petites icônes.
				compact: true,
				commands: [
					{ label: t('ribbon.cmd.undo.label'), icon: 'undo', ro: true, run: undo ? onundo : undefined, hint: undo ? t('ribbon.cmd.undo.tooltip', { action: undo }) : t('ribbon.cmd.undo.none') },
					{ label: t('ribbon.cmd.redo.label'), icon: 'redo', ro: true, run: redo ? onredo : undefined, hint: redo ? t('ribbon.cmd.redo.tooltip', { action: redo }) : t('ribbon.cmd.redo.none') }
				]
			},
			{
				id: 'clipboard',
				title: t('ribbon.group.clipboard'),
				iconOk: true,
				// Comme Excel : Coller en grand, les autres en petit avec leur libellé.
				compact: true,
				commands: [
					{ label: t('ribbon.cmd.paste.label'), icon: 'paste', size: 'big', data: true, run: onclip ? () => onclip('paste') : undefined, hint: t('ribbon.cmd.paste.tooltip') },
					{ label: t('ribbon.cmd.cut.label'), icon: 'cut', size: 'small', run: onclip ? () => onclip('cut') : undefined, hint: t('ribbon.cmd.cut.tooltip') },
					{ label: t('ribbon.cmd.copy.label'), icon: 'copy', size: 'small', ro: true, run: onclip ? () => onclip('copy') : undefined, hint: t('ribbon.cmd.copy.tooltip') },
					{ label: t('ribbon.cmd.painter.label'), icon: 'format-painter', size: 'small', local: true, run: onpainter, hint: t('ribbon.cmd.painter.tooltip') }
				]
			},
			{
				id: 'font',
				title: t('ribbon.group.font'),
				fonts: true,
				iconOk: true,
				// Gras, Italique, Souligné : petites icônes, comme Excel.
				widest: 'icon',
				launch: fmt('font'),
				launchHint: t('ribbon.launch.font'),
				commands: local([
					{ label: t('ribbon.cmd.bold.label'), icon: fontIcon('bold', lang()), run: st({ b: 'toggle' }), hint: t('ribbon.cmd.bold.tooltip'), on: !!cellStyle?.b },
					{ label: t('ribbon.cmd.italic.label'), icon: fontIcon('italic', lang()), run: st({ i: 'toggle' }), hint: t('ribbon.cmd.italic.tooltip'), on: !!cellStyle?.i },
					{ label: t('ribbon.cmd.underline.label'), icon: fontIcon('underline', lang()), run: st({ u: 'toggle' }), hint: t('ribbon.cmd.underline.tooltip'), on: !!cellStyle?.u },
					{ label: t('ribbon.cmd.fontColor.label'), icon: 'font-color', run: st({ fg }), hint: t('ribbon.cmd.fontColor.tooltip'), color: 'fg' },
					{ label: t('ribbon.cmd.fillColor.label'), icon: 'font-color', run: st({ bg }), hint: t('ribbon.cmd.fillColor.tooltip'), color: 'bg' }
				])
			},
			{
				id: 'borders',
				title: t('ribbon.group.borders'),
				iconOk: true,
				// Trois petites icônes l'une sous l'autre.
				compact: true,
				launch: fmt('border'),
				launchHint: t('ribbon.launch.borders'),
				commands: local([
					{ label: t('ribbon.cmd.bordersAll.label'), icon: 'border-all', run: st({ border: 'all' }), hint: t('ribbon.cmd.bordersAll.tooltip') },
					{ label: t('ribbon.cmd.bordersOutline.label'), icon: 'border-outline', run: st({ border: 'outline' }), hint: t('ribbon.cmd.bordersOutline.tooltip') },
					{ label: t('ribbon.cmd.bordersNone.label'), icon: 'border-none', run: st({ border: 'none' }), hint: t('ribbon.cmd.bordersNone.tooltip') }
				])
			},
			{
				id: 'alignment',
				title: t('ribbon.group.alignment'),
				iconOk: true,
				// Comme Excel : les alignements en petites icônes (libellé dans
				// l'info-bulle), Renvoyer à la ligne et Fusionner en grands boutons.
				compact: true,
				launch: fmt('align'),
				launchHint: t('ribbon.launch.alignment'),
				commands: [
					...local([
						{ label: t('ribbon.cmd.alignTop.label'), icon: 'align-top', run: st({ va: 'top' }), hint: t('ribbon.cmd.alignTop.tooltip'), on: cellStyle?.va === 'top' },
						{ label: t('ribbon.cmd.alignMiddle.label'), icon: 'align-middle', run: st({ va: null }), hint: t('ribbon.cmd.alignMiddle.tooltip'), on: !!cellStyle && !cellStyle.va },
						{ label: t('ribbon.cmd.alignBottom.label'), icon: 'align-bottom', run: st({ va: 'bottom' }), hint: t('ribbon.cmd.alignBottom.tooltip'), on: cellStyle?.va === 'bottom' },
						// Comme Excel : cliquer sur l'alignement en cours revient au standard.
						{ label: t('ribbon.cmd.alignLeft.label'), icon: 'align-left', run: st(alignPatch(cellStyle?.al, 'left')), hint: cellStyle?.al === 'left' ? t('ribbon.cmd.alignLeft.tooltipOn') : t('ribbon.cmd.alignLeft.tooltip'), on: cellStyle?.al === 'left' },
						{ label: t('ribbon.cmd.alignCenter.label'), icon: 'align-center', run: st(alignPatch(cellStyle?.al, 'center')), hint: cellStyle?.al === 'center' ? t('ribbon.cmd.alignCenter.tooltipOn') : t('ribbon.cmd.alignCenter.tooltip'), on: cellStyle?.al === 'center' },
						{ label: t('ribbon.cmd.alignRight.label'), icon: 'align-right', run: st(alignPatch(cellStyle?.al, 'right')), hint: cellStyle?.al === 'right' ? t('ribbon.cmd.alignRight.tooltipOn') : t('ribbon.cmd.alignRight.tooltip'), on: cellStyle?.al === 'right' },
						{ label: t('ribbon.cmd.wrap.label'), icon: 'wrap-text', run: onwrap, size: 'big', hint: t('ribbon.cmd.wrap.tooltip'), on: !!cellStyle?.wr }
					]),
					{ label: t('ribbon.cmd.merge.label'), icon: 'merge-cells', run: onmerge, size: 'big', hint: merged ? t('ribbon.cmd.merge.tooltipOn') : t('ribbon.cmd.merge.tooltip'), on: merged }
				]
			},
			{
				id: 'number',
				title: t('ribbon.group.number'),
				iconOk: true,
				launch: fmt('number'),
				launchHint: t('ribbon.launch.number'),
				// Comme Excel : la liste des formats (Standard ▾) en haut, puis les
				// formats en petites icônes et, en dessous, les décimales côte à côte.
				rows: 3,
				commands: local([
					{
						label: t('ribbon.cmd.numberFormat.label'),
						icon: 'number-format',
						combo: nfName(cellStyle?.nf ?? ''),
						run: onstyle ? openItems : undefined,
						hint: t('ribbon.cmd.numberFormat.tooltip'),
						items: [
							{ label: t('ribbon.numberFormat.general'), run: st({ nf: null, dp: null, ng: null, df: null }), on: !cellStyle?.nf },
							{ label: t('ribbon.cmd.numberFormat.number'), run: st({ nf: '0,00', dp: null, ng: null, df: null }), on: cellStyle?.nf === '0' || cellStyle?.nf === '0,00' },
							{ label: t('ribbon.cmd.currency.label'), run: st({ nf: '€', ng: null, df: null }), on: cellStyle?.nf === '€' },
							{ label: t('ribbon.cmd.percent.label'), run: st({ nf: '%', ng: null, df: null }), on: cellStyle?.nf === '%' },
							{ label: t('ribbon.cmd.numberFormat.date'), run: st({ nf: 'date', df: 'jj/mm/aaaa', dp: null, ng: null }), on: cellStyle?.nf === 'date' },
							{ label: t('ribbon.cmd.numberFormat.more'), keys: 'Ctrl+1', run: fmt('number') }
						]
					},
					{ label: t('ribbon.cmd.currency.label'), icon: 'currency', run: st({ nf: '€', ng: null, df: null }), hint: t('ribbon.cmd.currency.tooltip') },
					{ label: t('ribbon.cmd.percent.label'), icon: 'percent', run: st({ nf: '%', ng: null, df: null }), hint: t('ribbon.cmd.percent.tooltip') },
					{ label: t('ribbon.cmd.date.label'), icon: 'type-date', run: st({ nf: 'date', df: 'jj/mm/aaaa', dp: null, ng: null }), hint: t('ribbon.cmd.date.tooltip') },
					{ label: t('ribbon.cmd.fixed2.label'), icon: 'decimal-fixed', run: st({ nf: '0,00', dp: null, ng: null, df: null }), hint: t('ribbon.cmd.fixed2.tooltip') },
					{ label: t('ribbon.cmd.decimalMore.label'), icon: 'decimal-more', run: st({ dp: 'more' }), hint: t('ribbon.cmd.decimalMore.tooltip') },
					{ label: t('ribbon.cmd.decimalLess.label'), icon: 'decimal-less', run: st({ dp: 'less' }), hint: t('ribbon.cmd.decimalLess.tooltip') }
				])
			},
			{
				id: 'styles',
				title: t('ribbon.group.styles'),
				commands: [
					{
						label: t('ribbon.cmd.conditional.label'),
						icon: 'conditional-format',
						menu: true,
						local: true,
						run: onrules ? () => onrules(anchor.x, anchor.y) : undefined,
						hint: t('ribbon.cmd.conditional.tooltip')
					},
					{ label: t('ribbon.cmd.formatAsTable.label'), icon: 'table', run: ed('formatAsTable'), hint: t('ribbon.cmd.formatAsTable.tooltip') }
				]
			},
			{
				id: 'cells',
				title: t('ribbon.group.cells'),
				// Comme Excel : Insérer, Supprimer et Format, chacun avec ses choix.
				commands: [
					{
						label: t('ribbon.cmd.insert.label'),
						icon: 'insert-row',
						run: oncells ? openItems : undefined,
						hint: t('ribbon.cmd.insert.tooltip'),
						items: [
							{ label: t('ribbon.cmd.insert.row'), data: true, run: cells('insertRow') },
							{ label: t('ribbon.cmd.insert.column'), run: cells('insertColumn') }
						]
					},
					{
						label: t('ribbon.cmd.delete.label'),
						icon: 'delete-row',
						run: oncells ? openItems : undefined,
						hint: t('ribbon.cmd.delete.tooltip'),
						items: [
							{ label: t('ribbon.cmd.delete.row'), data: true, run: cells('deleteRow') },
							{ label: t('ribbon.cmd.delete.column'), run: cells('deleteColumn') }
						]
					},
					{
						label: t('ribbon.cmd.format.label'),
						icon: 'cell-format',
						local: true,
						run: onformat ? openItems : undefined, // ouvre ses choix
						hint: t('ribbon.cmd.format.tooltip'),
						items: [
							{ label: t('ribbon.cmd.format.cells'), keys: 'Ctrl+1', run: fmt('number') },
							{ label: t('ribbon.cmd.format.hideRows'), keys: 'Ctrl+9', run: lines(true, true) },
							{ label: t('ribbon.cmd.format.hideColumns'), keys: 'Ctrl+0', run: lines(false, true) },
							{ label: t('ribbon.cmd.format.showRows'), run: lines(true, false) },
							{ label: t('ribbon.cmd.format.showColumns'), run: lines(false, false) }
						]
					}
				]
			},
			{
				id: 'editing',
				title: t('ribbon.group.editing'),
				// Comme Excel : Somme automatique, Remplissage et Effacer en petit avec
				// leur libellé, Trier et filtrer en grand, puis Rechercher et Remplacer.
				compact: true,
				commands: [
					{ label: t('ribbon.cmd.autoSum.label'), icon: 'autosum', size: 'small', run: onformula ? () => onformula('sum') : undefined, hint: t('ribbon.cmd.autoSum.tooltip') },
					{
						label: t('ribbon.cmd.fill.label'),
						data: true,
						icon: 'fill-down',
						size: 'small',
						run: onedit ? openItems : undefined,
						hint: t('ribbon.cmd.fill.tooltip'),
						items: [
							{ label: t('ribbon.cmd.fill.down'), keys: 'Ctrl+D', run: ed('fillDown') },
							{ label: t('ribbon.cmd.fill.right'), keys: 'Ctrl+R', run: ed('fillRight') }
						]
					},
					{
						label: t('ribbon.cmd.clear.label'),
						icon: 'clear-format',
						size: 'small',
						run: onstyle ? openItems : undefined,
						hint: t('ribbon.cmd.clear.tooltip'),
						items: [
							{ label: t('ribbon.cmd.clear.formats'), local: true, run: st(clearPatch()) },
							{ label: t('ribbon.cmd.clear.contents'), keys: 'Suppr', data: true, run: ed('clearContents') },
							{ label: t('ribbon.cmd.clear.note'), local: true, run: note && onnote ? () => onnote('delete') : undefined }
						]
					},
					{
						label: t('ribbon.cmd.sortFilter.label'),
						icon: 'filter',
						size: 'big',
						run: onsort || onfilter ? openItems : undefined,
						hint: t('ribbon.cmd.sortFilter.tooltip'),
						items: [
							{ label: t('ribbon.cmd.sortFilter.az'), local: true, data: true, run: onsort ? () => onsort(false) : undefined },
							{ label: t('ribbon.cmd.sortFilter.za'), local: true, data: true, run: onsort ? () => onsort(true) : undefined },
							{ label: t('ribbon.cmd.sortCustom.label'), local: true, data: true, run: onsortcustom },
							{ label: t('ribbon.cmd.filter.label'), ro: true, keys: 'Ctrl+Maj+L', run: flt('filter') },
							{ label: t('ribbon.cmd.sortFilter.clearFilters'), ro: true, run: filters ? flt('clear') : undefined }
						]
					},
					{ label: t('ribbon.cmd.find.label'), icon: 'search', size: 'small', ro: true, run: onfind ? () => onfind(false) : undefined, hint: t('ribbon.cmd.find.tooltip') },
					{ label: t('ribbon.cmd.replace.label'), icon: 'replace', size: 'small', run: onfind ? () => onfind(true) : undefined, hint: t('ribbon.cmd.replace.tooltip') }
				]
			}
		],
		insert: [
			{
				id: 'tables',
				title: t('ribbon.group.tables'),
				commands: [
					{ label: t('ribbon.cmd.pivot.label'), icon: 'pivot-table', ro: true, run: onpivot, hint: t('ribbon.cmd.pivot.tooltip') },
					{ label: t('ribbon.cmd.table.label'), icon: 'table', run: ed('formatAsTable'), hint: t('ribbon.cmd.table.tooltip') },
					...(sharedWorkbooksEnabled() ? [{ label: t('ribbon.cmd.dataLake.label'), icon: 'database' as const, run: oninsertdata, hint: t('ribbon.cmd.dataLake.tooltip') }, { label: t('ribbon.cmd.form.label'), icon: 'form' as const, run: onform, hint: t('ribbon.cmd.form.tooltip') }] : [])
				]
			},
			{
				id: 'charts',
				title: t('ribbon.group.charts'),
				commands: local([
					...(['bar', 'line', 'pie', 'scatter'] as const).map((k) => ({
						label: kindInfo(k)!.label,
						icon: kindInfo(k)!.icon,
						run: onchart ? () => onchart(k) : undefined,
						hint: k === 'bar' ? t('ribbon.cmd.chart.tooltipBar', { chart: kindInfo(k)!.label }) : t('ribbon.cmd.chart.tooltip', { chart: kindInfo(k)!.label })
					})),
					{ label: t('ribbon.cmd.moreCharts.label'), icon: 'chart-more', run: onchart ? openItems : undefined, hint: moreHint, items: MORE_KINDS.map((k) => ({ label: k.label, icon: k.icon, run: onchart ? () => onchart(k.kind) : undefined })) }
				])
			},
			{
				id: 'controls',
				title: t('ribbon.group.controls'),
				commands: local([
					...(scriptingEnabled() ? [{ label: t('ribbon.cmd.button.label'), icon: 'button' as const, run: onbutton ? () => onbutton('') : undefined, hint: t('ribbon.cmd.button.tooltip') }] : []),
					{
						label: t('ribbon.cmd.field.label'),
						icon: 'text-input',
						run: onbutton ? () => onbutton('input') : undefined,
						hint: scriptingEnabled() ? t('ribbon.cmd.field.tooltipScript') : t('ribbon.cmd.field.tooltip')
					}
				])
			}
		],
		pageLayout: readOnlyGroups([
			{
				id: 'pageSetup',
				title: t('ribbon.group.pageSetup'),
				launch: pg({ kind: 'dialog', tab: 'page' }),
				launchHint: t('ribbon.launch.pageSetup'),
				commands: [
					{
						label: t('ribbon.cmd.margins.label'),
						icon: 'page-margins',
						run: onpage ? openItems : undefined,
						hint: t('ribbon.cmd.margins.tooltip', { preset: mPreset === 'custom' ? t('ribbon.cmd.margins.customName') : marginLabel(mPreset) }),
						items: [
							...(Object.keys(MARGINS) as (keyof typeof MARGINS)[]).map((k) => ({ label: marginHint(k), run: pg({ kind: 'margins', preset: k }), on: mPreset === k })),
							{ label: t('ribbon.cmd.margins.custom'), run: pg({ kind: 'dialog', tab: 'margins' }), on: mPreset === 'custom' }
						]
					},
					{
						label: t('ribbon.cmd.orientation.label'),
						icon: 'page-orientation',
						run: onpage ? openItems : undefined,
						hint: ps.landscape ? t('ribbon.cmd.orientation.tooltipLandscape') : t('ribbon.cmd.orientation.tooltipPortrait'),
						items: [
							{ label: t('ribbon.cmd.orientation.portrait'), run: pg({ kind: 'orientation', landscape: false }), on: !ps.landscape },
							{ label: t('ribbon.cmd.orientation.landscape'), run: pg({ kind: 'orientation', landscape: true }), on: !!ps.landscape }
						]
					},
					{
						label: t('ribbon.cmd.size.label'),
						icon: 'page-size',
						run: onpage ? openItems : undefined,
						hint: t('ribbon.cmd.size.tooltip', { paper: paperLabel }),
						items: [...PAPERS.map((x) => ({ label: t('ribbon.cmd.size.item', { name: x.label, size: x.size }), run: pg({ kind: 'paper', paper: x.id }), on: paperOf(ps) === x.id })), { label: t('ribbon.cmd.size.more'), run: pg({ kind: 'dialog', tab: 'page' }) }]
					},
					{
						label: t('ribbon.cmd.printArea.label'),
						icon: 'print-area',
						run: onpage ? openItems : undefined,
						on: !!ps.area,
						hint: ps.area ? t('ribbon.cmd.printArea.tooltipOn') : t('ribbon.cmd.printArea.tooltip'),
						items: [
							{ label: t('ribbon.cmd.printArea.set'), run: pg({ kind: 'area', clear: false }) },
							{ label: t('ribbon.cmd.printArea.clear'), run: ps.area ? pg({ kind: 'area', clear: true }) : undefined }
						]
					},
					{
						label: t('ribbon.cmd.breaks.label'),
						icon: 'page-break',
						run: onpage ? openItems : undefined,
						hint: t('ribbon.cmd.breaks.tooltip'),
						items: [
							{ label: t('ribbon.cmd.breaks.insert'), run: pg({ kind: 'break', op: 'insert' }) },
							{ label: t('ribbon.cmd.breaks.remove'), run: hasBreaks(ps) ? pg({ kind: 'break', op: 'remove' }) : undefined },
							{ label: t('ribbon.cmd.breaks.reset'), run: hasBreaks(ps) ? pg({ kind: 'break', op: 'reset' }) : undefined }
						]
					},
					{ label: t('ribbon.cmd.printTitles.label'), icon: 'print-titles', run: pg({ kind: 'dialog', tab: 'sheet' }), on: !!(ps.titleRows || ps.titleCols), hint: t('ribbon.cmd.printTitles.tooltip') }
				]
			},
			{
				id: 'scale',
				title: t('ribbon.group.scale'),
				// Comme Excel : Largeur, Hauteur et Échelle l'une sous l'autre, en petit.
				widest: 'small',
				launch: pg({ kind: 'dialog', tab: 'page' }),
				launchHint: t('ribbon.launch.scale'),
				commands: [
					{ label: t('ribbon.cmd.width.label'), icon: 'fit-width', run: onpage ? openItems : undefined, hint: t('ribbon.cmd.width.tooltip', { fit: fitLabel(ps.fitWide ?? 0) }), items: FIT_CHOICES.map((n) => ({ label: fitLabel(n), run: pg({ kind: 'fit', wide: true, n }), on: (ps.fitWide ?? 0) === n })) },
					{ label: t('ribbon.cmd.height.label'), icon: 'fit-height', run: onpage ? openItems : undefined, hint: t('ribbon.cmd.height.tooltip', { fit: fitLabel(ps.fitTall ?? 0) }), items: FIT_CHOICES.map((n) => ({ label: fitLabel(n), run: pg({ kind: 'fit', wide: false, n }), on: (ps.fitTall ?? 0) === n })) },
					{
						label: t('ribbon.cmd.scale.label'),
						icon: 'zoom',
						run: onpage ? openItems : undefined,
						hint: fitted(ps) ? t('ribbon.cmd.scale.tooltipFitted', { scale: scaleLabel(ps) }) : t('ribbon.cmd.scale.tooltip', { scale: scaleLabel(ps) }),
						items: [...SCALE_CHOICES.map((n) => ({ label: t('ribbon.cmd.scale.percent', { n }), run: pg({ kind: 'scale', pct: n }), on: !fitted(ps) && (ps.scale || 100) === n })), { label: t('ribbon.cmd.scale.custom'), run: pg({ kind: 'dialog', tab: 'page' }) }]
					}
				]
			},
			{
				id: 'sheetOptions',
				title: t('ribbon.group.sheetOptions'),
				launch: pg({ kind: 'dialog', tab: 'sheet' }),
				launchHint: t('ribbon.launch.sheetOptions'),
				// Cases en petit avec leur libellé, gardé le plus longtemps possible
				// (deux icônes d'imprimante : l'icône seule ne dit pas laquelle).
				widest: 'small',
				commands: [
					{ label: t('ribbon.cmd.printPdf.label'), icon: 'printer', run: () => onfile('print'), hint: t('ribbon.cmd.printPdf.tooltip') },
					{ label: t('ribbon.cmd.showGridlines.label'), icon: 'border-all', on: gridlines, run: ongridlines ? () => ongridlines(!gridlines) : undefined, hint: gridlines ? t('ribbon.cmd.showGridlines.tooltipOn') : t('ribbon.cmd.showGridlines.tooltipOff') },
					{ label: t('ribbon.cmd.printGridlines.label'), icon: 'printer', on: !!ps.gridlines, run: pg({ kind: 'print', what: 'gridlines', on: !ps.gridlines }), hint: t('ribbon.cmd.printGridlines.tooltip') },
					{ label: t('ribbon.cmd.showHeadings.label'), icon: 'headings', on: headings, run: onheadings ? () => onheadings(!headings) : undefined, hint: headings ? t('ribbon.cmd.showHeadings.tooltipOn') : t('ribbon.cmd.showHeadings.tooltipOff') },
					{ label: t('ribbon.cmd.printHeadings.label'), icon: 'printer', on: !!ps.headings, run: pg({ kind: 'print', what: 'headings', on: !ps.headings }), hint: t('ribbon.cmd.printHeadings.tooltip') }
				]
			}
		]),
		data: [
			{
				id: 'getData',
				title: t('ribbon.group.getData'),
				// Une requête s'ajoute même sur une feuille protégée (le moteur vérifie où elle écrit).
				// « À partir du tableau » en grand et en tête (choix de Louis, 19/10 :
				// la fonction phare, plus puissante qu'Excel ici) ; les autres sources
				// en petit avec leur libellé, en colonnes de trois.
				compact: true,
				commands: readOnly([
					{ label: t('ribbon.cmd.fromTable.label'), icon: 'sheet', size: 'big', run: data('table'), hint: t('ribbon.cmd.fromTable.tooltip') },
					// Connecté à un Serveur Lattis : son bouton passe en grand, à côté (demande de Louis, 21/10).
					...(lattisSignedIn ? [{ label: t('ribbon.cmd.lattisServer.label'), icon: 'link' as const, size: 'big' as const, run: data('lattis'), hint: t('ribbon.cmd.lattisServer.tooltip') }] : []),
					{ label: t('ribbon.cmd.csv.label'), icon: 'file-csv', size: 'small' as const, run: data('csv'), hint: t('ribbon.cmd.csv.tooltip') },
					{ label: t('ribbon.cmd.xlsx.label'), icon: 'table', size: 'small' as const, run: data('xlsx'), hint: t('ribbon.cmd.xlsx.tooltip') },
					{
						label: t('ribbon.cmd.otherFiles.label'), size: 'small' as const,
						icon: 'open',
						run: ondata ? openItems : undefined,
						hint: t('ribbon.cmd.otherFiles.tooltip'),
						items: [
							{ label: 'JSON', run: data('json') },
							{ label: 'Parquet', run: data('parquet') },
							{ label: t('ribbon.cmd.otherFiles.sqlite'), run: data('sqlite') }
						]
					},
					// Démo publique : pas de base de données ni d'API web (aucune connexion sortante).
					...(networkSourcesEnabled() ? [{
						label: t('ribbon.cmd.database.label'), size: 'small' as const,
						icon: 'database' as const,
						run: ondata ? openItems : undefined,
						hint: t('ribbon.cmd.database.tooltip'),
						items: [
							{ label: 'PostgreSQL', run: data('postgres') },
							{ label: 'MySQL / MariaDB', run: data('mysql') },
							{ label: 'SQL Server', run: data('sqlserver') }
						]
					}] : []),
					// Serveur Lattis : bouton à lui, à côté des bases de données (comme les sources propres d'Excel).
					...(sharedWorkbooksEnabled() && !lattisSignedIn ? [{ label: t('ribbon.cmd.lattisServer.label'), icon: 'link' as const, size: 'small' as const, run: data('lattis'), hint: t('ribbon.cmd.lattisServer.tooltip') }] : []),
					...(networkSourcesEnabled() ? [{ label: t('ribbon.cmd.web.label'), icon: 'web' as const, size: 'small' as const, run: data('web'), hint: t('ribbon.cmd.web.tooltip') }] : []),
				])
			},
			{
				id: 'queries',
				title: t('ribbon.group.queries'),
				// Les deux en grand (choix de Louis, 19/10) : un petit bouton seul à côté faisait vide.
				compact: true,
				commands: readOnly([
					{ label: t('ribbon.cmd.refreshAll.label'), icon: 'refresh', size: 'big', run: data('refreshAll'), hint: t('ribbon.cmd.refreshAll.tooltip') },
					{ label: t('ribbon.cmd.queries.label'), icon: 'query', size: 'big', run: data('queries'), hint: t('ribbon.cmd.queries.tooltip') }
				])
			},
			{
				id: 'sortFilter',
				title: t('ribbon.group.sortFilter'),
				// Comme Excel : les deux tris en petites icônes l'une sous l'autre,
				// Filtrer en grand, Effacer et Erreurs seulement en petit à côté.
				compact: true,
				commands: [
					{ label: t('ribbon.cmd.sortAsc.label'), icon: 'sort-asc', local: true, data: true, run: onsort ? () => onsort(false) : undefined, hint: t('ribbon.cmd.sortAsc.tooltip') },
					{ label: t('ribbon.cmd.sortDesc.label'), icon: 'sort-desc', local: true, data: true, run: onsort ? () => onsort(true) : undefined, hint: t('ribbon.cmd.sortDesc.tooltip') },
					{ label: t('ribbon.cmd.sortCustom.label'), icon: 'sort', local: true, data: true, run: onsortcustom, hint: t('ribbon.cmd.sortCustom.tooltip') },
					{ label: t('ribbon.cmd.filter.label'), icon: 'filter', size: 'big', ro: true, run: flt('filter'), hint: t('ribbon.cmd.filter.tooltip') },
					{ label: t('ribbon.cmd.clearFilter.label'), icon: 'clear-format', size: 'small', ro: true, run: filters ? flt('clear') : undefined, hint: filters ? t('ribbon.cmd.clearFilter.tooltipOn') : t('ribbon.cmd.clearFilter.tooltipOff') },
					{ label: t('ribbon.cmd.errorsOnly.label'), icon: 'warning', size: 'small', ro: true, on: errorsOnly, run: flt('errors'), hint: errorsOnly ? t('ribbon.cmd.errorsOnly.tooltipOn') : t('ribbon.cmd.errorsOnly.tooltipOff') }
				]
			},
			{
				id: 'dataTools',
				title: t('ribbon.group.dataTools'),
				// Comme Excel : Convertir en grand, les autres outils en petit à côté.
				compact: true,
				commands: [
					{ label: t('ribbon.cmd.textToColumns.label'), icon: 'text-columns', size: 'big', run: ontextcols, hint: t('ribbon.cmd.textToColumns.tooltip') },
					{ label: t('ribbon.cmd.dedup.label'), icon: 'remove-duplicates', size: 'small', data: true, run: ondedup, hint: t('ribbon.cmd.dedup.tooltip') },
					{ label: t('ribbon.cmd.validation.label'), icon: 'validation', size: 'small', local: true, run: onvalidation, hint: t('ribbon.cmd.validation.tooltip') },
					{ label: t('ribbon.cmd.mergeSheets.label'), icon: 'append-queries', size: 'small', createsSheet: true, run: onmergesheets, hint: t('ribbon.cmd.mergeSheets.tooltip') }
				]
			}
		],
		formulas: [
			{
				id: 'library',
				title: t('ribbon.group.library'),
				commands: [
					{ label: t('ribbon.cmd.insertFunction.label'), icon: 'function', run: onformula ? () => onformula('insert') : undefined, hint: t('ribbon.cmd.insertFunction.tooltip') },
					{ label: t('ribbon.cmd.autoSum.label'), icon: 'autosum', run: onformula ? () => onformula('sum') : undefined, hint: t('ribbon.cmd.autoSum.tooltip') },
					{ label: 'REFERENCE', icon: 'reference', run: onformula ? () => onformula('reference') : undefined, hint: t('ribbon.cmd.reference.tooltip') }
				]
			},
			{
				id: 'audit',
				title: t('ribbon.group.audit'),
				// Pas de « Supprimer les flèches » : un clic dans la grille (ou Échap) les efface.
				commands: readOnly([
					{ label: t('ribbon.cmd.showLinks.label'), icon: 'dependency-graph', run: onaudit ? () => onaudit('show') : undefined, hint: t('ribbon.cmd.showLinks.tooltip') }
				])
			},
			{
				id: 'calculation',
				title: t('ribbon.group.calculation'),
				commands: [
					{ label: t('ribbon.cmd.calculate.label'), icon: 'calculate', ro: true, run: onrecalc, hint: t('ribbon.cmd.calculate.tooltip') },
					{ label: t('ribbon.cmd.toValues.label'), icon: 'type-number', run: ontovalues, hint: t('ribbon.cmd.toValues.tooltip') }
				]
			}
		],
		review: [
			{
				id: 'notes',
				title: t('ribbon.group.notes'),
				// Comme les commentaires d'Excel : la note en grand, Supprimer en petit à côté.
				compact: true,
				commands: local([
					{ label: note ? t('ribbon.cmd.note.edit') : t('ribbon.cmd.note.new'), icon: 'note', size: 'big', run: onnote ? () => onnote('edit') : undefined, hint: note ? t('ribbon.cmd.note.tooltipEdit') : t('ribbon.cmd.note.tooltipNew') },
					{ label: t('ribbon.cmd.deleteNote.label'), icon: 'trash', size: 'small', run: onnote && note ? () => onnote('delete') : undefined, hint: note ? t('ribbon.cmd.deleteNote.tooltip') : t('ribbon.cmd.deleteNote.tooltipNone') }
				])
			},
			{
				id: 'protect',
				title: t('ribbon.group.protect'),
				commands: readOnly([
					{ label: sheetOwnProtected ? prot.unprotect : prot.protect, icon: 'lock-sheet', on: sheetOwnProtected, run: onprotect ? () => onprotect('sheet') : undefined, hint: sheetOwnProtected ? prot.unprotect : t('ribbon.cmd.protectSheet.tooltip', { action: prot.protect }) },
					...(context === 'sheet'
						? [
								{
									label: structureProtected && !structureByBook ? t('ribbon.cmd.protectStructure.unprotect') : t('ribbon.cmd.protectStructure.protect'),
									icon: 'lock-structure' as const,
									on: structureProtected && !structureByBook,
									// Feuille déjà protégée en entier, ou structure verrouillée par le classeur : rien à poser ici.
									run: onprotect && (structureProtected ? !structureByBook : !sheetProtected) ? () => onprotect('structure') : undefined,
									hint:
										structureProtected && !structureByBook
											? t('ribbon.cmd.protectStructure.tooltipUnprotect')
											: structureByBook
												? t('ribbon.cmd.protectStructure.tooltipByBook')
												: sheetProtected
													? t('ribbon.cmd.protectStructure.tooltipSheet')
													: t('ribbon.cmd.protectStructure.tooltip')
								}
							]
						: []),
					{ label: workbookProtected ? t('ribbon.cmd.protectWorkbook.unprotect') : t('ribbon.cmd.protectWorkbook.protect'), icon: 'lock-book', on: workbookProtected, run: onprotect ? () => onprotect('workbook') : undefined, hint: workbookProtected ? t('ribbon.cmd.protectWorkbook.tooltipUnprotect') : t('ribbon.cmd.protectWorkbook.tooltip') }
				])
			}
		],
		view: [
			{
				id: 'show',
				title: t('ribbon.group.show'),
				// Comme Excel : Sauts de page (mode d'affichage) en grand, les cases
				// à cocher en petit avec leur libellé, puis Zoom, 100 % et Figer les volets.
				compact: true,
				commands: readOnly([
					{ label: t('ribbon.cmd.breaks.label'), icon: 'page-break', size: 'big', on: pageBreaks, run: onpagebreaks ? () => onpagebreaks(!pageBreaks) : undefined, hint: pageBreaks ? t('ribbon.cmd.pageBreaks.tooltipOn') : t('ribbon.cmd.pageBreaks.tooltipOff') },
					{ label: t('ribbon.cmd.gridlines.label'), icon: 'gridlines', size: 'small', on: gridlines, run: ongridlines ? () => ongridlines(!gridlines) : undefined, hint: gridlines ? t('ribbon.cmd.gridlines.tooltipOn') : t('ribbon.cmd.gridlines.tooltipOff') },
					{ label: t('ribbon.cmd.headings.label'), icon: 'headings', size: 'small', on: headings, run: onheadings ? () => onheadings(!headings) : undefined, hint: headings ? t('ribbon.cmd.showHeadings.tooltipOn') : t('ribbon.cmd.showHeadings.tooltipOff') },
					animateCmd,
					{
						label: t('ribbon.cmd.zoom.label'),
						icon: 'zoom',
						size: 'big',
						run: onzoom ? openItems : undefined,
						on: Math.abs(zoom - 1) > 0.001,
						hint: t('ribbon.cmd.zoom.tooltip', { zoom: zoomLabel(zoom) }),
						items: ZOOM_LEVELS.map((z) => ({ label: zoomLabel(z), run: onzoom ? () => onzoom(z) : undefined, on: Math.abs(zoom - z) < 0.001 }))
					},
					{ label: t('ribbon.cmd.zoom100.label'), icon: 'zoom-100', size: 'big', run: onzoom ? () => onzoom(1) : undefined, hint: t('ribbon.cmd.zoom100.tooltip') },
					{
						label: t('ribbon.cmd.freeze.label'),
						icon: 'freeze-panes',
						size: 'big',
						run: onfreeze ? openItems : undefined,
						on: frozen,
						hint: frozen ? t('ribbon.cmd.freeze.tooltipOn', { rows: freeze!.rows, cols: freeze!.cols }) : t('ribbon.cmd.freeze.tooltipOff'),
						items: [
							...(frozen ? [{ label: t('ribbon.cmd.freeze.unfreeze'), run: frz('none') }] : []),
							{ label: t('ribbon.cmd.freeze.cell'), run: frz('cell'), on: frozen && !(freeze!.rows === 1 && freeze!.cols === 0) && !(freeze!.rows === 0 && freeze!.cols === 1) },
							{ label: t('ribbon.cmd.freeze.row'), run: frz('row'), on: freeze?.rows === 1 && freeze.cols === 0 },
							{ label: t('ribbon.cmd.freeze.col'), run: frz('col'), on: freeze?.rows === 0 && freeze.cols === 1 }
						]
					}
				])
			},
			{
				id: 'theme',
				title: t('ribbon.group.theme'),
				commands: [
					{
						label: t('ribbon.cmd.theme.label'),
						icon: 'theme',
						size: 'big',
						run: ontheme ? openItems : undefined,
						hint: t('ribbon.cmd.theme.tooltip', { theme: themes.find((t) => t.on)?.name ?? t('ribbon.cmd.theme.customName') }),
						items: [
							...themes.map((t) => ({ label: t.name, swatch: [t.table, t.source] as [string, string], on: t.on, run: ontheme ? () => ontheme(t.id) : undefined })),
							{ label: t('ribbon.cmd.theme.customize'), sep: true, on: !themes.some((t) => t.on), run: ontheme ? () => ontheme('custom') : undefined }
						]
					}
				]
			},
			...(onbench ? [{ id: 'bench', title: t('ribbon.group.bench'), commands: readOnly([{ label: t('ribbon.cmd.bench.label'), icon: 'stopwatch', run: () => onbench() }]) }] : [])
		]
	});

	// Document : un traitement de texte (police, paragraphes,
	// graphiques et boutons insérés dans le texte).
	const docRibbons: Record<string, Group[]> = $derived({
		home: [
			{
				id: 'undo',
				title: t('ribbon.group.undo'),
				iconOk: true,
				compact: true,
				commands: [
					{ label: t('ribbon.cmd.undo.label'), icon: 'undo', run: docState?.undo === false ? undefined : dc('undo'), hint: docState?.undo === false ? t('ribbon.cmd.undo.none') : t('ribbon.cmd.undo.docTooltip') },
					{ label: t('ribbon.cmd.redo.label'), icon: 'redo', run: docState?.redo === false ? undefined : dc('redo'), hint: docState?.redo === false ? t('ribbon.cmd.redo.none') : t('ribbon.cmd.redo.docTooltip') }
				]
			},
			{
				id: 'clipboard',
				title: t('ribbon.group.clipboard'),
				iconOk: true,
				compact: true,
				commands: [
					{ label: t('ribbon.cmd.paste.label'), icon: 'paste', size: 'big', run: dc('paste'), hint: t('ribbon.cmd.paste.docTooltip') },
					{ label: t('ribbon.cmd.cut.label'), icon: 'cut', size: 'small', run: dc('cut'), hint: t('ribbon.cmd.cut.tooltip') },
					{ label: t('ribbon.cmd.copy.label'), icon: 'copy', size: 'small', ro: true, run: dc('copy'), hint: t('ribbon.cmd.copy.tooltip') }
				]
			},
			{
				id: 'font',
				title: t('ribbon.group.font'),
				fonts: true,
				iconOk: true,
				// Comme Word : gras, italique, souligné, barré en petites icônes.
				widest: 'icon',
				commands: [
					{ label: t('ribbon.cmd.bold.label'), icon: fontIcon('bold', lang()), run: st({ b: 'toggle' }), hint: t('ribbon.cmd.bold.tooltip'), on: !!cellStyle?.b },
					{ label: t('ribbon.cmd.italic.label'), icon: fontIcon('italic', lang()), run: st({ i: 'toggle' }), hint: t('ribbon.cmd.italic.tooltip'), on: !!cellStyle?.i },
					{ label: t('ribbon.cmd.underline.label'), icon: fontIcon('underline', lang()), run: st({ u: 'toggle' }), hint: t('ribbon.cmd.underline.tooltip'), on: !!cellStyle?.u },
					{ label: t('ribbon.cmd.strike.label'), icon: 'strike', run: dc('strike'), hint: t('ribbon.cmd.strike.tooltip'), on: !!docState?.strike },
					{ label: t('ribbon.cmd.fontColor.label'), icon: 'font-color', run: st({ fg }), hint: t('ribbon.cmd.fontColor.tooltip'), color: 'fg' },
					{ label: t('ribbon.cmd.highlight.label'), icon: 'font-color', run: st({ bg }), hint: t('ribbon.cmd.highlight.tooltip'), color: 'bg' },
					{ label: t('ribbon.cmd.clearChars.label'), icon: 'clear-format', run: st(clearPatch()), hint: t('ribbon.cmd.clearChars.tooltip') }
				]
			},
			{
				id: 'paragraph',
				title: t('ribbon.group.paragraph'),
				iconOk: true,
				// Comme Word : listes et blocs sur la première ligne, alignements sur la seconde.
				rows: 2,
				commands: [
					{ label: t('ribbon.cmd.bullets.label'), icon: 'list-bullet', run: dc('block:bullet'), on: blockOn('bullet'), hint: t('ribbon.cmd.bullets.tooltip') },
					{ label: t('ribbon.cmd.numbered.label'), icon: 'list-number', run: dc('block:ordered'), on: blockOn('ordered'), hint: t('ribbon.cmd.numbered.tooltip') },
					{ label: t('ribbon.cmd.quote.label'), icon: 'quote', run: dc('block:quote'), on: blockOn('quote'), hint: t('ribbon.cmd.quote.tooltip') },
					{ label: t('ribbon.cmd.code.label'), icon: 'code-block', run: dc('block:code'), on: blockOn('code'), hint: t('ribbon.cmd.code.tooltip') },
					{ label: t('ribbon.cmd.alignLeft.label'), icon: 'align-left', run: dc('align:left'), on: docState?.align === 'left', hint: t('ribbon.cmd.alignLeft.tooltip') },
					{ label: t('ribbon.cmd.alignCenter.label'), icon: 'align-center', run: dc('align:center'), on: docState?.align === 'center', hint: t('ribbon.cmd.alignCenter.tooltip') },
					{ label: t('ribbon.cmd.alignRight.label'), icon: 'align-right', run: dc('align:right'), on: docState?.align === 'right', hint: t('ribbon.cmd.alignRight.tooltip') },
					{ label: t('ribbon.cmd.justify.label'), icon: 'align-justify', run: dc('align:justify'), on: docState?.align === 'justify', hint: t('ribbon.cmd.justify.tooltip') }
				]
			},
			{
				id: 'styles',
				title: t('ribbon.group.styles'),
				commands: [
					{
						label: t('ribbon.cmd.style.label'),
						icon: 'heading',
						run: ondoc ? openItems : undefined,
						hint: t('ribbon.cmd.style.tooltip'),
						items: [
							{ label: t('ribbon.cmd.style.normal'), run: dc('block:normal'), on: blockOn('normal') },
							{ label: t('ribbon.cmd.style.h1'), run: dc('block:h1'), on: blockOn('h1') },
							{ label: t('ribbon.cmd.style.h2'), run: dc('block:h2'), on: blockOn('h2') },
							{ label: t('ribbon.cmd.style.h3'), run: dc('block:h3'), on: blockOn('h3') }
						]
					}
				]
			},
			// Imprimer ne modifie rien : permis sur un document protégé.
			{ id: 'print', title: t('ribbon.group.print'), commands: [{ label: t('ribbon.cmd.docPrint.label'), icon: 'printer', ro: true, run: dc('pdf'), hint: t('ribbon.cmd.docPrint.tooltip', { printer: pdfPrinterHint() }) }] }
		],
		insert: [
			// Le tableau croisé d'un Document est inséré dans le texte (il ne crée pas de feuille).
			{ id: 'tables', title: t('ribbon.group.tables'), commands: [{ label: t('ribbon.cmd.pivot.label'), icon: 'pivot-table', run: dc('pivot'), hint: t('ribbon.cmd.pivot.docTooltip') }] },
			{
				id: 'charts',
				title: t('ribbon.group.charts'),
				commands: [
					...(['bar', 'line', 'pie', 'scatter'] as const).map((k) => ({ label: kindInfo(k)!.label, icon: kindInfo(k)!.icon, run: dc(`chart:${k}`), hint: t('ribbon.cmd.chart.docTooltip', { chart: kindInfo(k)!.label }) })),
					{ label: t('ribbon.cmd.moreCharts.label'), icon: 'chart-more', run: ondoc ? openItems : undefined, hint: moreHint, items: MORE_KINDS.map((k) => ({ label: k.label, icon: k.icon, run: dc(`chart:${k.kind}`) })) }
				]
			},
			{
				id: 'controls',
				title: t('ribbon.group.controls'),
				commands: [
					...(scriptingEnabled() ? [{ label: t('ribbon.cmd.button.label'), icon: 'button' as const, run: dc('button'), hint: t('ribbon.cmd.button.docTooltip') }] : []),
					{ label: t('ribbon.cmd.field.label'), icon: 'text-input', run: dc('input'), hint: scriptingEnabled() ? t('ribbon.cmd.field.docTooltipScript') : t('ribbon.cmd.field.docTooltip') }
				]
			},
			{
				id: 'links',
				title: t('ribbon.group.links'),
				commands: [
					{ label: t('ribbon.cmd.link.label'), icon: 'link', run: dc('link'), on: !!docState?.link, hint: t('ribbon.cmd.link.tooltip') },
					{ label: t('ribbon.cmd.image.label'), icon: 'image', run: dc('image'), hint: t('ribbon.cmd.image.tooltip') }
				]
			}
		],
		review: sheetRibbons.review.filter((g) => g.id === 'protect'),
		view: [{ id: 'show', title: t('ribbon.group.show'), commands: [animateCmd] }]
	});
	// Page web : Annuler, Protéger et Animations (l'éditeur de la page a ses propres commandes).
	const htmlRibbons: Record<string, Group[]> = $derived({
		home: sheetRibbons.home.filter((g) => g.id === 'undo'),
		review: sheetRibbons.review.filter((g) => g.id === 'protect'),
		view: docRibbons.view
	});
	// Mise en page : des groupes de l'onglet Affichage (demande de Louis), sans
	// refaire ses cases « Afficher » (quadrillage, en-têtes) : seulement l'impression.
	const sheetWithPage: Record<string, Group[]> = $derived({
		...sheetRibbons,
		view: [
			...sheetRibbons.view,
			...(sheetRibbons.pageLayout ?? []).map((g) =>
				g.id === 'sheetOptions' ? { ...g, id: 'printing', title: t('ribbon.group.printing'), commands: g.commands.filter((c) => c.icon === 'printer') } : g
			)
		]
	});
	const allRibbons: Record<string, Group[]> = $derived(context === 'doc' ? docRibbons : context === 'html' ? htmlRibbons : sheetWithPage);
	// Onglets du ruban qui ont des commandes pour l'onglet affiché.
	const shownTabs = $derived(tabs.filter((t) => (allRibbons[t]?.length ?? 0) > 0));
	$effect(() => {
		if (!isView(tab) && !shownTabs.includes(tab as (typeof tabs)[number])) tab = 'home';
	});

	// Grisage (ribbon.ts) : protection de la feuille ou du Document, du
	// classeur, tableau partagé en lecture seule sous la cellule active.
	const ribbons: Record<string, Group[]> = $derived.by(() => {
		const s = { sheetProtected, sheetOwnProtected, workbookProtected, structureProtected: context === 'sheet' && structureProtected, structureByBook, layoutShared, tableReadOnly: context === 'sheet' ? tableReadOnly : undefined, context };
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
	// « onglet|groupe » (identifiants), gardés sur ce poste. Clic droit dans le
	// vide à droite : les réafficher.
	const HIDE_KEY = 'lattis.ribbon.hidden';
	let hiddenGroups = $state<string[]>(readHidden());
	let hideMenu = $state<{ x: number; y: number; group?: { id: string; title: string } } | null>(null);
	/** Entrée d'avant les identifiants (« Accueil|Presse-papiers ») : convertie par les libellés français. */
	function legacyHidden(x: string): string {
		const [tb, gr] = x.split('|');
		const tabId = Object.entries(frRibbon.tabs).find(([, v]) => v === tb)?.[0];
		const groupId = Object.entries(frRibbon.group).find(([, v]) => v === gr)?.[0];
		return tabId && groupId ? `${tabId}|${groupId}` : x;
	}
	function readHidden(): string[] {
		try {
			const v = JSON.parse(localStorage.getItem(HIDE_KEY) ?? '[]');
			return Array.isArray(v) ? v.filter((x) => typeof x === 'string').map(legacyHidden) : [];
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
	const visibleGroups = $derived(isView(tab) ? [] : (ribbons[tab] ?? []).filter((g) => !hiddenGroups.includes(`${tab}|${g.id}`)));
	const hiddenHere = $derived((ribbons[tab] ?? []).filter((g) => hiddenGroups.includes(`${tab}|${g.id}`)).map((g) => ({ id: g.id, title: g.title })));

	type ShapesArr = GroupShape[];
	const shapes = $derived(
		isView(tab)
			? []
			: visibleGroups.map(
					(g): GroupShape => ({
						title: g.title,
						iconOk: g.iconOk,
						widest: g.widest,
						labels: [...(g.fonts ? [FONTS_W] : []), ...g.commands.map((c) => (c.color ? null : c.items || c.menu ? c.label + ' ▾' : c.label))],
						sizes: g.compact ? [...(g.fonts ? ['big' as const] : []), ...g.commands.map((c) => c.size ?? 'icon')] : undefined,
						rows: g.rows ? { cols: rowCols(g), combo: g.commands.some((c) => c.combo !== undefined) ? COMBO_W : undefined } : undefined
					})
				)
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
	// Le choix est gardé sur ce poste. Sans choix, réduit sur un téléphone
	// (comme Excel mobile) : un onglet touché déplie ses commandes par-dessus.
	const FOLD_KEY = 'lattis.ribbon.folded';
	let folded = $state(readFold());
	let peek = $state(false);

	function readFold(): boolean {
		try {
			const v = localStorage.getItem(FOLD_KEY);
			if (v !== null) return v === '1';
		} catch {
			// stockage indisponible : réglage par défaut
		}
		return isNarrow();
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

	// Barre des onglets qui défile (téléphone) : l'onglet choisi reste visible.
	let tabsEl: HTMLElement;
	$effect(() => {
		void tab;
		const bar = tabsEl;
		if (!bar || bar.scrollWidth <= bar.clientWidth) return;
		const el = untrack(() => activeTabEl());
		if (!el) return;
		const b = bar.getBoundingClientRect();
		const r = el.getBoundingClientRect();
		const left = r.left - b.left + bar.scrollLeft;
		const to = revealX(left, left + r.width, bar.scrollLeft, bar.clientWidth);
		if (to !== bar.scrollLeft) bar.scrollTo({ left: to, behavior: reducedMotion() ? 'auto' : 'smooth' });
	});

	/** keep : déplier sans replier (KeyTips), au lieu de basculer comme un clic. */
	function select(t: typeof tab, keep = false) {
		peek = folded && !isView(t) && (keep || !(peek && tab === t));
		tab = t;
		onview(t === 'sql' ? 'sql' : t === 'script' ? 'script' : 'grid');
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
	const activeTabEl = () => tabEls().find((el) => el.dataset.tab === tab) ?? tabEls()[0];

	function tipsAt(level: 'tabs' | 'cmds'): Tip[] {
		const els = level === 'tabs' ? tabEls() : cmdEls();
		const tabId = (el: HTMLElement) => el.dataset.tab ?? '';
		const codes = level === 'tabs' ? els.map((el) => TAB_TIPS[tabId(el)] ?? keyChars(el.dataset.label ?? el.getAttribute('aria-label') ?? '')[0] ?? '') : assignKeyTips(els.map(tipLabel), commandTips());
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
		const id = el.dataset.tab ?? '';
		if (!tabs.includes(id as (typeof tabs)[number])) {
			kt = null;
			el.click(); // Fichier : son menu prend le focus (ouvert au clavier) ; ? : l'aide
			return;
		}
		select(id as typeof tab, true);
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
				const l = to.dataset.tab as (typeof tabs)[number];
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
		if (view === 'grid' && isView(tab)) tab = 'home';
		// Vue changée sans le ruban (retour de la pipette de l'onglet Script).
		else if (view === 'script' && tab !== 'script') tab = 'script';
		else if (view === 'sql' && tab !== 'sql') tab = 'sql';
	});
</script>

{#snippet colorCmd(c: Command)}
	{@const kind = c.color!}
	{@const cur = kind === 'fg' ? fg : bg}
		<div class="color-cmd">
			<button class="cmd" disabled={!c.run} onclick={c.run} title={t('ribbon.color.tooltip', { tip: tipOf(c, disabled), color: colorName(cur, tableTheme.current.colors) })} aria-label={t('ribbon.color.aria', { command: c.label, color: colorName(cur, tableTheme.current.colors) })} data-kt={c.label}>
				<!-- La lettre montre l'effet : couleur du texte, ou fond rempli. -->
				<span
					class="swatch-icon"
					style:color={kind === 'fg' ? shown(fg) : '#1f2328'}
					style:background={kind === 'bg' ? shown(bg) : 'transparent'}
					aria-hidden="true">A</span
				>
			</button>
			<button class="drop" disabled={!c.run} onclick={(ev) => openPalette(kind, ev)} title={t('ribbon.color.choose')} aria-label={t('ribbon.color.chooseAria', { command: c.label })} aria-haspopup="menu" aria-expanded={palette === kind} data-kt="{c.label} ▾">▾</button>
			{#if palette === kind}
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="palette" role="dialog" data-own-keys aria-label={c.label} style:left="{palettePos.x}px" style:top="{palettePos.y}px" use:dismiss={() => (palette = null)}>
					<ColorPalette value={cur} none={kind === 'bg' ? t('ribbon.color.noFill') : t('ribbon.color.auto')} autofocus onpick={(col) => pick(kind, col)} />
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
	<nav class="tabs" aria-label={t('ribbon.tabsAria')} bind:this={tabsEl}>
		<div class="file-wrap">
			<button class="tab file" data-tab="file" data-label={tabLabel('file')} aria-haspopup="menu" aria-expanded={fileOpen} aria-keyshortcuts="Alt+F" onclick={(ev) => openFile(ev)}>{tabLabel('file')}</button>
			{#if fileOpen}
				<!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
				<div class="file-menu" role="menu" aria-label={tabLabel('file')} style:left="{filePos.x}px" style:top="{filePos.y}px" use:dismiss={() => (fileOpen = false)}>
					{#each [['new', t('ribbon.file.new'), 'Ctrl+N'], ['open', t('ribbon.file.open'), 'Ctrl+O']] as [a, label, keys] (a)}
						<button role="menuitem" onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span><span class="keys">{keyLabel(keys)}</span></button>
					{/each}
					<!-- Comme Excel : les fichiers récents juste sous Ouvrir ; sous-menus au survol, par Entrée ou →, ← les referme. -->
					<Submenu label={t('ribbon.file.recent')} disabled={!recent.length} title={recent.length ? undefined : t('ribbon.file.recentNone')}>
						{#each recent as r (recentKey(r))}
							<button role="menuitem" title={recentDetail(r)} onclick={() => { fileOpen = false; onrecent?.(r); }}>{#if recentRemote(r)}<Icon name="cloud" size={14} />{/if}<span class="name">{recentLabel(r)}</span></button>
						{/each}
					</Submenu>
					{#each [...(sharedWorkbooksEnabled() ? [['openRemote', t('ribbon.file.openRemote'), '']] : []), ['close', t('ribbon.file.close'), 'Ctrl+W']] as [a, label, keys] (a)}
						<button role="menuitem" disabled={disabled && a === 'close'} onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span><span class="keys">{keyLabel(keys)}</span></button>
					{/each}
					<hr />
					{#each [['save', t('ribbon.file.save'), 'Ctrl+S'], ['saveAs', t('ribbon.file.saveAs'), 'F12'], ...(platform.web && !platform.demo && !shared ? [['download', t('ribbon.file.download'), '']] : []), ...(sharedWorkbooksEnabled() && !shared ? [['share', t('ribbon.file.share'), '']] : []), ...(sharedWorkbooksEnabled() && rights ? [['rights', t('ribbon.file.rights'), '']] : []), ...(sharedWorkbooksEnabled() && republish ? [['republish', t('ribbon.file.republish'), '']] : []), ...(sharedWorkbooksEnabled() && reconnect ? [['reconnect', t('ribbon.file.reconnect'), '']] : []), ...(sharedWorkbooksEnabled() && detach ? [['detach', t('ribbon.file.detach'), '']] : []), ...(sharedWorkbooksEnabled() && remove ? [['remove', t('ribbon.file.remove'), '']] : []), ['import', t('ribbon.file.import'), '']] as [a, label, keys] (a)}
						<button role="menuitem" class:danger={a === 'remove'} disabled={disabled && a !== 'import'} onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span><span class="keys">{keyLabel(keys)}</span></button>
					{/each}
					<hr />
					<!-- Exporter : CSV du tableau ou de la feuille, classeur .xlsx ou .ods ; l'impression reste à part. -->
					<Submenu label={t('ribbon.file.export')} {disabled}>
						{#each [['export', t('ribbon.file.exportTable')], ['exportSheet', t('ribbon.file.exportSheet')], ['exportXlsx', t('ribbon.file.exportXlsx')], ['exportOds', t('ribbon.file.exportOds')]] as [a, label] (a)}
							<button role="menuitem" onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span></button>
						{/each}
					</Submenu>
					<button role="menuitem" {disabled} onclick={() => { fileOpen = false; onfile('print'); }}><span>{t('ribbon.file.print')}</span><span class="keys">{keyLabel('Ctrl+P')}</span></button>
					<hr />
					{#each [['settings', t('ribbon.file.settings'), ''], ['help', t('ribbon.file.help'), 'F1'], ['report', t('ribbon.file.report'), ''], ['about', t('ribbon.file.about'), ''], ...(import.meta.env.DEV ? [['demo', t('ribbon.file.demo'), '']] : [])] as [a, label, keys] (a)}
						<button role="menuitem" onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span><span class="keys">{keyLabel(keys)}</span></button>
					{/each}
				</div>
			{/if}
		</div>
		<div class="tablist" role="tablist" aria-label={t('ribbon.tabsAria')}>
			{#each shownTabs as id, i (id)}
				<button class="tab" id="ribbon-tab-{i}" role="tab" data-tab={id} data-label={tabLabel(id)} class:on={tab === id} aria-selected={tab === id} aria-controls={tab === id ? 'ribbon-panel' : undefined} tabindex={tab === id ? 0 : -1} aria-keyshortcuts="Alt+{TAB_TIPS[id]}" data-region-focus={tab === id ? '' : undefined} {disabled} onclick={() => select(id)} ondblclick={toggleFold}>{tabLabel(id)}</button>
			{/each}
		</div>
		<span class="sep"></span>
		<!-- Bascule du volet de l'Assistant IA (pas un onglet : la grille reste) ;
		     visible même désactivé, le volet propose alors de l'activer. -->
		{#if onai}
			<button class="tab ai" data-tab="ai" data-label={tabLabel('ai')} class:on={ai} aria-pressed={ai} aria-keyshortcuts="Alt+{TAB_TIPS.ai}" {disabled} onclick={onai} title={ai ? t('ribbon.aiButton.close') : t('ribbon.aiButton.open')}
				>{t('ribbon.aiButton.label')} <Icon name="sparkle" size={13} /></button
			>
		{/if}
		<button class="tab special" data-tab="sql" data-label={tabLabel('sql')} class:on={tab === 'sql'} aria-pressed={tab === 'sql'} aria-keyshortcuts="Alt+{TAB_TIPS.sql}" data-region-focus={tab === 'sql' ? '' : undefined} {disabled} onclick={() => select('sql')}>{tabLabel('sql')}</button>
		{#if scriptingEnabled() || view === 'script'}
		<button class="tab special" data-tab="script" data-label={tabLabel('script')} class:on={tab === 'script'} aria-pressed={tab === 'script'} aria-keyshortcuts="Alt+{TAB_TIPS.script}" data-region-focus={tab === 'script' ? '' : undefined} {disabled} onclick={() => select('script')}>{tabLabel('script')}</button>
		{/if}
		<button class="tab help" data-tab="help" title={t('ribbon.helpButton.tooltip')} aria-label={t('ribbon.helpButton.aria')} onclick={() => onfile('help')}>?</button>
	</nav>
	{#if !isView(tab) && (!folded || peek)}
		<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
		<div class="fold-wrap" class:peek={folded} use:peekWatch onclick={peekClick} transition:slide={{ duration: foldAnim && !reducedMotion() ? 200 : 0 }}>
		<div class="groups" class:off={disabled} id="ribbon-panel" tabindex="-1" oncontextmenu={(e) => { if (e.target !== e.currentTarget) return; e.preventDefault(); hideMenu = { x: e.clientX, y: e.clientY }; }} role="tabpanel" aria-labelledby="ribbon-tab-{shownTabs.indexOf(tab as (typeof tabs)[number])}" use:observe>
			{#each visibleGroups as g, gi (g.id)}
				{@const mode = modes[gi] ?? 'large'}
				<section class="group" aria-labelledby="ribbon-group-{gi}">
					{#if mode === 'collapsed'}
						<div class="commands">
							<button
								class="cmd"
								data-kt={g.title}
								aria-haspopup="menu"
								aria-expanded={openGroup?.id === g.id}
								onclick={(ev) => {
									const r = (ev.currentTarget as HTMLElement).getBoundingClientRect();
									openGroup = openGroup?.id === g.id ? null : { id: g.id, x: r.left, y: r.bottom + 2 };
								}}
								title={g.title}
							>
								<Icon name={g.commands[0].icon} />
								<span class="label">{g.title} ▾</span>
							</button>
						</div>
						{#if openGroup?.id === g.id}
							<!-- svelte-ignore a11y_no_static_element_interactions -->
							<div class="group-menu" style:left="{openGroup.x}px" style:top="{openGroup.y}px" use:dismiss={() => (openGroup = null)}>
								{#if g.fonts}{@render fontCombos()}{/if}
								{#each g.commands as c, cIndex (cIndex)}
									{#if c.color}
										{@render colorCmd(c)}
									{:else if c.items}
										<!-- Liste Format de nombre : sans les choix qui ont leur bouton dans le groupe (Monétaire, Pourcentage). -->
										{#each c.combo === undefined ? c.items : c.items.filter((it) => !g.commands.some((o) => o.label === it.label)) as it, itIndex (itIndex)}
											<button
												class="cmd row-cmd"
												role="menuitem"
												class:pressed={it.on}
												disabled={!c.run || !it.run}
												title={it.keys ? t('ribbon.withKeys', { label: it.label, keys: keyLabel(it.keys) }) : undefined}
												onclick={() => {
													openGroup = null;
													it.run?.();
												}}
											>
												<Icon name={it.icon ?? c.icon} size={18} />
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
						<div
							class="commands"
							class:stack={!g.rows && (mode !== 'large' || g.compact)}
							class:rows={!!g.rows}
							style:grid-template-rows={g.rows ? `repeat(${g.rows}, 1fr)` : undefined}
							style:grid-template-columns={g.rows ? `repeat(${rowCols(g)}, 1fr)` : undefined}
						>
							{#if g.fonts}{@render fontCombos()}{/if}
							{#each g.commands as c, cIndex (cIndex)}
								{@const size = sizeOf(g, c, mode)}
								{#if c.color}
									{@render colorCmd(c)}
								{:else if c.combo !== undefined}
									<!-- Liste du format en cours (Standard ▾), comme Excel : ses choix s'ouvrent dessous. -->
									<button
										class="combo"
										style:min-width="{COMBO_W}px"
										aria-haspopup="menu"
										aria-expanded={dropMenu?.label === c.label}
										aria-label={t('ribbon.comboAria', { command: c.label, value: c.combo })}
										data-kt={c.label}
										disabled={!c.run}
										onclick={(ev) => toggleDrop(c, ev)}
										title={tipOf(c, disabled)}><span class="label">{c.combo}</span><span class="chev" aria-hidden="true">▾</span></button
									>
								{:else}
									<button
										class="cmd"
										class:row-cmd={size !== 'big'}
										class:big-cmd={size === 'big' && !!g.compact}
										class:pressed={c.on}
										aria-pressed={c.on === undefined ? undefined : c.on}
										aria-haspopup={c.items || c.menu ? 'menu' : undefined}
										aria-expanded={c.items ? dropMenu?.label === c.label : undefined}
										aria-label={size === 'icon' ? c.label : undefined}
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
										<Icon name={c.icon} size={size === 'big' ? 24 : 16} />
										{#if size !== 'icon'}<span class="label">{c.label}{c.items || c.menu ? ' ▾' : ''}</span>{/if}
									</button>
								{/if}
							{/each}
						</div>
					{/if}
					<div class="title" oncontextmenu={(e) => { e.preventDefault(); hideMenu = { x: e.clientX, y: e.clientY, group: { id: g.id, title: g.title } }; }}>
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
		<button class="fold" title={folded ? t('ribbon.fold.pinTooltip') : t('ribbon.fold.foldTooltip')} aria-label={folded ? t('ribbon.fold.pinAria') : t('ribbon.fold.foldAria')} onclick={toggleFold}>
			<svg viewBox="0 0 12 12" width="12" height="12" aria-hidden="true">
				{#if folded}<path d="M3 1.5h6M6 1.5v5M3.5 6.5h5L7 4.5H5zM6 6.5v4" fill="none" stroke="currentColor" stroke-width="1.2" />{:else}<path d="M2.5 7.5 6 4l3.5 3.5" fill="none" stroke="currentColor" stroke-width="1.4" />{/if}
			</svg>
		</button>
		</div>
	{/if}
	{#if hideMenu}
		{@const m = hideMenu}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="drop-menu" role="menu" aria-label={t('ribbon.hideMenu.aria')} style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (hideMenu = null)}>
			{#if m.group}
				{@const grp = m.group}
				<button role="menuitem" onclick={() => { const key = `${tab}|${grp.id}`; hideMenu = null; setHidden([...hiddenGroups, key]); }}><span class="item">{t('ribbon.hideMenu.hide', { group: grp.title })}</span></button>
			{/if}
			{#each hiddenHere as h (h.id)}
				<button role="menuitem" onclick={() => { hideMenu = null; setHidden(hiddenGroups.filter((x) => x !== `${tab}|${h.id}`)); }}><span class="item">{t('ribbon.hideMenu.show', { group: h.title })}</span></button>
			{/each}
			{#if hiddenHere.length > 1}
				<button role="menuitem" onclick={() => { hideMenu = null; setHidden(hiddenGroups.filter((x) => !x.startsWith(`${tab}|`))); }}><span class="item">{t('ribbon.hideMenu.showAll')}</span></button>
			{/if}
			{#if !m.group && !hiddenHere.length}
				<button role="menuitem" disabled><span class="item">{t('ribbon.hideMenu.none')}</span></button>
			{/if}
		</div>
	{/if}
	{#if dropMenu}
		{@const c = ribbons[tab]?.flatMap((g) => g.commands).find((x) => x.label === dropMenu!.label)}
		{#if c?.items}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="drop-menu" role="menu" aria-label={c.label} style:left="{dropMenu.x}px" style:top="{dropMenu.y}px" use:dismiss={() => (dropMenu = null)}>
				{#each c.items as it, i (i)}
					<button
						role={it.on === undefined ? 'menuitem' : 'menuitemcheckbox'}
						aria-checked={it.on === undefined ? undefined : it.on}
						disabled={!it.run}
						class:on={it.on}
						class:sep={it.sep}
						onclick={() => {
							dropMenu = null;
							it.run?.();
						}}>{#if it.swatch}<span class="check" aria-hidden="true">{it.on ? '✓' : ''}</span><span class="swatch2" aria-hidden="true"><i style:background={it.swatch[0]}></i><i style:background={it.swatch[1]}></i></span>{:else if it.icon}<span class="ico" aria-hidden="true"><Icon name={it.icon} size={16} /></span>{:else}<span class="check" aria-hidden="true">{it.on ? '✓' : ''}</span>{/if}<span class="item">{it.label}</span>{#if it.keys}<span class="keys">{keyLabel(it.keys)}</span>{/if}</button
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
			title={t('ribbon.group.font')}
			disabled={!onstyle || sheetProtected || structureProtected || layoutShared}
			options={FONTS.map((f) => ({ value: f, label: f }))}
			onchange={(v) => onstyle?.({ ff: v === FONTS[0] ? null : v })}
		/>
		<Select
			value={cellStyle?.fs ?? DEFAULT_PT}
			width="64px"
			title={t('ribbon.fontSize')}
			disabled={!onstyle || sheetProtected || structureProtected || layoutShared}
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
	/* Aide : un onglet comme les autres, libellé « ? » (demande de Louis, 21/10). */
	.tab.help {
		font-weight: 600;
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
		position: fixed;
		z-index: 30;
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
	/* Action qui supprime pour tous (Retirer du serveur…). */
	.file-menu button.danger:not(:disabled) {
		color: #b91c1c;
	}
	.file-menu hr {
		border: none;
		border-top: 1px solid #e5e7eb;
		margin: 4px 0;
	}
	/* Récent du serveur : nuage puis nom, à gauche (pas écartés par space-between). */
	.file-menu .name {
		flex: 1;
		margin-left: 6px;
		text-align: left;
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
		/* Groupe plus étroit que son titre (Annuler) : les colonnes au milieu. */
		justify-content: center;
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
	.commands.stack .big-cmd {
		grid-row: span 3;
	}
	/* Groupe en lignes (Nombre, Paragraphe) : petites icônes rangées ligne par
	   ligne, sous la liste du format qui prend toute la première. */
	.commands.rows {
		display: grid;
		grid-auto-flow: row;
		gap: 0 2px;
	}
	.commands.rows .cmd {
		justify-content: center;
	}
	.combo {
		grid-column: 1 / -1;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 6px;
		height: 21px;
		padding: 0 4px 0 6px;
		font: inherit;
		font-size: 12px;
		color: #1f2328;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		cursor: pointer;
		text-align: left;
	}
	.combo:hover:not(:disabled) {
		border-color: #8fb79d;
		background: #f6faf7;
	}
	.combo[aria-expanded='true'] {
		border-color: #217346;
	}
	.combo:disabled {
		color: #9aa0a6;
		cursor: default;
	}
	.combo .label {
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.combo .chev {
		flex: none;
		font-size: 9px;
		color: #6b7280;
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
	.drop-menu .ico {
		display: inline-flex;
		align-items: center;
		width: 18px;
		margin-right: 6px;
		--icon-accent: #217346;
	}
	.drop-menu .item {
		flex: 1;
	}
	.drop-menu button.sep {
		border-top: 1px solid #e2e4e7;
	}
	/* Thème : couleur des tableaux ordinaires, puis des sources externes. */
	.drop-menu .swatch2 {
		display: inline-flex;
		align-self: center;
		margin-right: 8px;
		border: 1px solid rgba(0, 0, 0, 0.2);
		border-radius: 3px;
		overflow: hidden;
	}
	.drop-menu .swatch2 i {
		width: 12px;
		height: 14px;
	}
	.drop-menu .keys {
		margin-left: 18px;
	}
	/* Téléphone : les onglets ne passent pas à la ligne et défilent au doigt. */
	@media (max-width: 600px) {
		.tabs {
			overflow-x: auto;
			overflow-y: hidden;
			scrollbar-width: none;
			overscroll-behavior-x: contain;
			padding: 0 4px;
		}
		.tabs::-webkit-scrollbar {
			display: none;
		}
		.tab {
			flex: none;
			white-space: nowrap;
			padding: 0 12px;
		}
		.sep {
			flex: 1 0 6px;
		}
	}
	/* Doigt : onglets plus hauts (cibles d'au moins 32 px). */
	@media (pointer: coarse) {
		.tabs {
			height: 40px;
		}
	}
</style>
