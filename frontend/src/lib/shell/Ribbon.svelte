<script lang="ts">
	// Ruban façon Excel : onglets en haut, groupes de commandes en dessous.
	// IA, SQL et SCRIPT sont des onglets à part entière : ils prennent la place
	// de la grille (précédence sur GRID). IA n'apparaît que s'il est activé
	// (Fichier › Paramètres › Fonctionnalités). Les commandes pas encore réalisées sont
	// affichées désactivées, pour montrer l'organisation cible.

	import { onMount, tick, untrack } from 'svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { dismiss } from '#lib/grid/dismiss.js';
	import { clearPatch, type CellStyle } from '#lib/grid/style.js';
	import { DEFAULT_PT, FONTS, SIZES } from '#lib/grid/textwrap.js';
	import { ZOOM_LEVELS, zoomLabel } from '#lib/grid/zoom.js';
	import Select from '#lib/ui/Select.svelte';
	import { estimate, fit, groupWidth, type GroupShape, type Measure, type Mode } from './fit';
	import { aiEnabled } from './settings.svelte.js';
	import type { IconName } from '#lib/icons/icons.js';
	import { MORE_KINDS, type ChartKind } from '#lib/chart/kinds.js';
	import { COMMAND_TIPS, TAB_TIPS, assignKeyTips, keyChars, keyOf, matchTip } from '#lib/ui/keytips.js';
	import { cycle, focusRegion, isTextField, modalOpen, regionOf, tabbables } from '#lib/ui/focus.js';

	export type View = 'grid' | 'ai' | 'sql' | 'script';
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

	interface Command {
		label: string;
		icon: IconName;
		run?: () => void;
		hint?: string;
		color?: 'fg' | 'bg'; // commande avec un choix de couleur
		on?: boolean; // commande à bascule, enfoncée
		/** Commande à menu (▾) : ses choix. */
		items?: { label: string; run?: () => void; on?: boolean }[];
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

	export type FileAction = 'new' | 'open' | 'save' | 'saveAs' | 'import' | 'export' | 'exportXlsx' | 'print' | 'pdf' | 'close' | 'demo' | 'mcp' | 'settings' | 'help';

	export type DataAction = 'csv' | 'xlsx' | 'json' | 'parquet' | 'sqlite' | 'postgres' | 'mysql' | 'sqlserver' | 'web' | 'table' | 'queries' | 'refreshAll';

	interface Props {
		view: View;
		title: string;
		dirty: boolean;
		undo: string; // libellé de l'action à annuler (vide : rien)
		redo: string;
		/** Aucun classeur ouvert (récupération, classeur fermé) : ruban grisé. */
		disabled?: boolean;
		/** Classeurs récents (chemins), pour Fichier › Récents. */
		recent?: string[];
		onrecent?: (path: string) => void;
		onview: (v: View) => void;
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
		onfilter?: (errorsOnly: boolean) => void;
		onrecalc?: () => void;
		ondedup?: () => void;
		/** Données › Convertir : assistant « Convertir le texte en colonnes ». */
		ontextcols?: () => void;
		onpivot?: () => void;
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
		docState?: { block: string; align: string; strike: boolean; link: boolean };
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
	}

	let { view, title, dirty, undo, redo, disabled = false, recent = [], onrecent, onview, onfile, onundo, onredo, onbench, onsort, onstyle, onfind, onrules, onfilter, onrecalc, ondedup, ontextcols, onpivot, onchart, onformula, onclip, onpainter, onaudit, oncells, gridlines = true, ongridlines, headings = false, onheadings, context = 'sheet', ondoc, docState, onbutton, sheetProtected = false, sheetOwnProtected = sheetProtected, workbookProtected = false, onprotect, ondata, onformat, onmerge, onwrap, onlines, onvalidation, onnote, freeze, onfreeze, cellStyle, merged = false, zoom = 1, onzoom, animate = true, onanimate }: Props = $props();
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
	let recentOpen = $state(false);
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
	const COLORS = [
		'#000000', '#404040', '#7f7f7f', '#bfbfbf', '#ffffff',
		'#c00000', '#ff0000', '#ffc000', '#ffff00', '#92d050',
		'#00b050', '#00b0f0', '#0070c0', '#002060', '#7030a0',
		'#ffc7ce', '#fff2cc', '#c6efce', '#ddebf7', '#e4dfec'
	];
	function pick(kind: 'fg' | 'bg', c: string | null) {
		palette = null;
		if (c) {
			if (kind === 'fg') fg = c;
			else bg = c;
		}
		onstyle?.({ [kind]: c });
	}

	const tabs = ['Accueil', 'Insertion', 'Données', 'Formules', 'Révision', 'Affichage'] as const;
	let tab = $state<(typeof tabs)[number] | 'IA' | 'SQL' | 'SCRIPT'>('Accueil');
	/** Onglets qui remplacent la grille (pas de groupes de commandes). */
	const isView = (t: string) => t === 'IA' || t === 'SQL' || t === 'SCRIPT';
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
	const blockOn = (b: string) => docState?.block === b;
	const sheetRibbons: Record<string, Group[]> = $derived({
		Accueil: [
			{
				title: 'Annuler',
				iconOk: true,
				commands: [
					{ label: 'Annuler', icon: 'undo', run: undo ? onundo : undefined, hint: undo ? 'Annuler : ' + undo + ' (Ctrl+Z)' : 'Rien à annuler' },
					{ label: 'Rétablir', icon: 'redo', run: redo ? onredo : undefined, hint: redo ? 'Rétablir : ' + redo + ' (Ctrl+Y)' : 'Rien à rétablir' }
				]
			},
			{ title: 'Presse-papiers', iconOk: true, commands: [{ label: 'Coller', icon: 'paste', run: onclip ? () => onclip('paste') : undefined, hint: 'Coller (Ctrl+V)' }, { label: 'Couper', icon: 'cut', run: onclip ? () => onclip('cut') : undefined, hint: 'Couper (Ctrl+X)' }, { label: 'Copier', icon: 'copy', run: onclip ? () => onclip('copy') : undefined, hint: 'Copier (Ctrl+C)' }, { label: 'Reproduire', icon: 'format-painter', run: onpainter, hint: 'Reproduire la mise en forme de la cellule active' }] },
			{
				title: 'Police',
				fonts: true,
				iconOk: true,
				launch: fmt('font'),
				launchHint: 'Format de cellule : police (Ctrl+1)',
				commands: [
					{ label: 'Gras', icon: 'bold', run: st({ b: 'toggle' }), hint: 'Gras (Ctrl+B)', on: !!cellStyle?.b },
					{ label: 'Italique', icon: 'italic', run: st({ i: 'toggle' }), hint: 'Italique (Ctrl+I)', on: !!cellStyle?.i },
					{ label: 'Souligné', icon: 'underline', run: st({ u: 'toggle' }), hint: 'Souligné (Ctrl+U)', on: !!cellStyle?.u },
					{ label: 'Couleur', icon: 'font-color', run: st({ fg }), hint: 'Couleur du texte (▾ : choisir la teinte)', color: 'fg' },
					{ label: 'Remplissage', icon: 'font-color', run: st({ bg }), hint: 'Couleur de remplissage (▾ : choisir la teinte)', color: 'bg' },
					{ label: 'Effacer', icon: 'remove-duplicates', run: st(clearPatch()), hint: 'Effacer la mise en forme' }
				]
			},
			{
				title: 'Bordures',
				iconOk: true,
				launch: fmt('border'),
				launchHint: 'Format de cellule : bordure (Ctrl+1)',
				commands: [
					{ label: 'Toutes', icon: 'border-all', run: st({ border: 'all' }), hint: 'Toutes les bordures' },
					{ label: 'Contour', icon: 'border-outline', run: st({ border: 'outline' }), hint: 'Bordure extérieure de la sélection' },
					{ label: 'Aucune', icon: 'border-none', run: st({ border: 'none' }), hint: 'Retirer les bordures' }
				]
			},
			{
				title: 'Alignement',
				iconOk: true,
				launch: fmt('align'),
				launchHint: 'Format de cellule : alignement (Ctrl+1)',
				commands: [
					{ label: 'Haut', icon: 'align-top', run: st({ va: 'top' }), hint: 'Aligner en haut', on: cellStyle?.va === 'top' },
					{ label: 'Milieu', icon: 'align-middle', run: st({ va: null }), hint: 'Aligner au milieu', on: !!cellStyle && !cellStyle.va },
					{ label: 'Bas', icon: 'align-bottom', run: st({ va: 'bottom' }), hint: 'Aligner en bas', on: cellStyle?.va === 'bottom' },
					{ label: 'Gauche', icon: 'align-left', run: st({ al: 'left' }), on: cellStyle?.al === 'left' },
					{ label: 'Centrer', icon: 'align-center', run: st({ al: 'center' }), on: cellStyle?.al === 'center' },
					{ label: 'Droite', icon: 'align-right', run: st({ al: 'right' }), on: cellStyle?.al === 'right' },
					{ label: 'Renvoyer à la ligne', icon: 'wrap-text', run: onwrap, hint: 'Afficher le texte sur plusieurs lignes dans la cellule', on: !!cellStyle?.wr },
					{ label: 'Fusionner et centrer', icon: 'merge-cells', run: onmerge, hint: merged ? 'Annuler la fusion des cellules' : 'Fusionner les cellules libres sélectionnées en une seule, contenu centré', on: merged }
				]
			},
			{
				title: 'Nombre',
				iconOk: true,
				launch: fmt('number'),
				launchHint: 'Format de cellule : nombre, date, heure… (Ctrl+1)',
				commands: [
					{ label: '0,00', icon: 'decimal-fixed', run: st({ nf: '0,00', dp: null, ng: null, df: null }), hint: 'Deux décimales' },
					{ label: 'Monétaire', icon: 'currency', run: st({ nf: '€', ng: null, df: null }) },
					{ label: 'Pourcentage', icon: 'percent', run: st({ nf: '%', ng: null, df: null }) },
					// Les deux boutons de décimales restent côte à côte, dans le ruban
					// large comme empilés (même colonne de trois).
					{ label: 'Ajouter une décimale', icon: 'decimal-more', run: st({ dp: 'more' }), hint: 'Ajouter une décimale à l’affichage' },
					{ label: 'Réduire les décimales', icon: 'decimal-less', run: st({ dp: 'less' }), hint: 'Afficher une décimale de moins' },
					{ label: 'Date', icon: 'type-date', run: st({ nf: 'date', df: 'jj/mm/aaaa', dp: null, ng: null }), hint: 'Date courte (jj/mm/aaaa) ; Ctrl+1 pour les autres formats de date et d’heure' },
					{ label: 'Standard', icon: 'number-format', run: st({ nf: null, dp: null, ng: null, df: null }), hint: 'Format Standard' }
				]
			},
			{
				title: 'Styles',
				commands: [
					{
						label: 'Mise en forme conditionnelle',
						icon: 'conditional-format',
						menu: true,
						run: onrules ? () => onrules(anchor.x, anchor.y) : undefined,
						hint: 'Mise en évidence des cellules, barres de données, nuances de couleurs, jeux d’icônes, gestionnaire des règles'
					}
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
						run: onformat ? openItems : undefined, // ouvre ses choix
						hint: 'Format de cellule, masquer et afficher',
						items: [
							{ label: 'Format de cellule… (Ctrl+1)', run: fmt('number') },
							{ label: 'Masquer les lignes', run: lines(true, true) },
							{ label: 'Masquer les colonnes', run: lines(false, true) },
							{ label: 'Afficher les lignes', run: lines(true, false) },
							{ label: 'Afficher les colonnes', run: lines(false, false) }
						]
					}
				]
			},
			{
				title: 'Édition',
				commands: [
					{ label: 'Trier A → Z', icon: 'sort', run: onsort ? () => onsort(false) : undefined },
					{ label: 'Rechercher', icon: 'search', run: onfind ? () => onfind(false) : undefined, hint: 'Rechercher (Ctrl+F)' },
					{ label: 'Remplacer', icon: 'search', run: onfind ? () => onfind(true) : undefined, hint: 'Remplacer (Ctrl+H)' }
				]
			}
		],
		Insertion: [
			{ title: 'Tableaux', commands: [{ label: 'Tableau croisé', icon: 'pivot-table', run: onpivot, hint: 'Tableau croisé dynamique du tableau actif (double-clic sur un TCD pour le modifier)' }] },
			{
				title: 'Graphiques',
				commands: [
					{ label: 'Barres', icon: 'chart-bar', run: onchart ? () => onchart('bar') : undefined, hint: 'Graphique en barres du tableau actif' },
					{ label: 'Courbes', icon: 'chart-line', run: onchart ? () => onchart('line') : undefined, hint: 'Graphique en courbes du tableau actif' },
					{ label: 'Secteurs', icon: 'chart-pie', run: onchart ? () => onchart('pie') : undefined, hint: 'Graphique en secteurs du tableau actif' },
					{ label: 'Nuage de points', icon: 'chart-scatter', run: onchart ? () => onchart('scatter') : undefined, hint: 'Nuage de points du tableau actif' },
					{ label: 'Autres graphiques', icon: 'chart-more', run: onchart ? openItems : undefined, hint: 'Combiné, 3D, carte, cascade, compartimentage, radar, entonnoir, boîte à moustaches, fréquences, jauge', items: MORE_KINDS.map((k) => ({ label: k.label, run: onchart ? () => onchart(k.kind) : undefined })) }
				]
			},
			{
				title: 'Contrôles',
				commands: [
					{ label: 'Bouton', icon: 'button', run: onbutton ? () => onbutton('') : undefined, hint: 'Bouton d’action à la cellule active : un clic exécute un script de l’onglet SCRIPT (ou rien)' },
					{
						label: 'Champ',
						icon: 'text-input',
						run: onbutton ? () => onbutton('input') : undefined,
						hint: 'Contrôle lié à une cellule, qui peut aussi lancer une action : champ texte, zone de texte, case à cocher, liste déroulante ou fichier (choix du type dans la fenêtre)'
					}
				]
			}
		],
		Données: [
			{
				title: 'Obtenir des données',
				commands: [
					{ label: 'Texte/CSV', icon: 'file', run: data('csv'), hint: 'Nouvelle requête à partir d’un fichier CSV (étapes de transformation, actualisable)' },
					{ label: 'Classeur Excel', icon: 'table', run: data('xlsx'), hint: 'Nouvelle requête à partir d’un tableau d’un classeur .xlsx' },
					{
						label: 'Autres fichiers',
						icon: 'file',
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
					{ label: 'Web', icon: 'web', run: data('web'), hint: 'Nouvelle requête à partir d’une API web qui répond en JSON (lecture seule)' },
					{ label: 'À partir du tableau', icon: 'sheet', run: data('table'), hint: 'Nouvelle requête à partir d’un tableau du classeur' }
				]
			},
			{
				title: 'Requêtes et connexions',
				commands: [
					{ label: 'Requêtes et connexions', icon: 'query', run: data('queries'), hint: 'Volet des requêtes d’import et des connexions du classeur' },
					{ label: 'Actualiser tout', icon: 'refresh', run: data('refreshAll'), hint: 'Relire les sources de toutes les requêtes chargées, remplacer leurs tableaux et recalculer (Ctrl+F5)' }
				]
			},
			{
				title: 'Trier et filtrer',
				commands: [
					{ label: 'Trier A → Z', icon: 'sort', run: onsort ? () => onsort(false) : undefined, hint: 'Trier le tableau par la colonne active, croissant' },
					{ label: 'Trier Z → A', icon: 'sort', run: onsort ? () => onsort(true) : undefined, hint: 'Trier le tableau par la colonne active, décroissant' },
					{ label: 'Filtrer', icon: 'filter', run: onfilter ? () => onfilter(false) : undefined, hint: 'Filtrer le tableau par la colonne active' }
				]
			},
			{ title: 'Outils de données', commands: [{ label: 'Supprimer les doublons', icon: 'remove-duplicates', run: ondedup, hint: 'Supprimer les lignes en double du tableau actif' }, { label: 'Convertir', icon: 'text-columns', run: ontextcols, hint: 'Convertir le texte en colonnes : couper le contenu de la colonne sélectionnée (séparateurs ou largeur fixe) en plusieurs colonnes' }, { label: 'Erreurs seulement', icon: 'warning', run: onfilter ? () => onfilter(true) : undefined, hint: 'Afficher seulement les saisies hors type de la colonne active (bascule)' }, { label: 'Validation des données', icon: 'validation', run: onvalidation, hint: 'Liste déroulante : valeurs permises dans la sélection (ou toute une colonne de tableau), les autres saisies sont refusées' }] }
		],
		Formules: [
			{ title: 'Bibliothèque', commands: [{ label: 'Insérer une fonction', icon: 'function', run: onformula ? () => onformula('insert') : undefined, hint: 'Choisir une fonction et commencer la formule dans la cellule active' }, { label: 'Somme automatique', icon: 'autosum', run: onformula ? () => onformula('sum') : undefined, hint: 'SOMME de la colonne du tableau au-dessus' }, { label: 'REFERENCE', icon: 'reference', run: onformula ? () => onformula('reference') : undefined, hint: 'REFERENCE(Tableau[Colonne]; clé) : la RECHERCHEV de Lattis' }] },
			{ title: 'Audit', commands: [{ label: 'Afficher les liens', icon: 'dependency-graph', run: onaudit ? () => onaudit('show') : undefined, hint: 'Flèches des liens de toutes les cellules sélectionnées : bleues depuis ce que lisent leurs formules, rouges vers les formules qui les lisent ; elles partent dès que la sélection change' }] },
			{ title: 'Calcul', commands: [{ label: 'Calculer', icon: 'calculate', run: onrecalc, hint: 'Recalculer les formules et renouveler AUJOURDHUI, MAINTENANT et ALEA (F9, ou F5 hors d’un tableau importé)' }] }
		],
		Révision: [
			{
				title: 'Notes',
				commands: [
					{ label: 'Nouvelle note', icon: 'note', run: onnote ? () => onnote('edit') : undefined, hint: 'Note sur la cellule active (ou la modifier) : coin rouge, affichée au survol' },
					{ label: 'Supprimer la note', icon: 'remove-duplicates', run: onnote ? () => onnote('delete') : undefined, hint: 'Supprimer la note de la cellule active' }
				]
			},
			{
				title: 'Protéger',
				commands: [
					{ label: sheetOwnProtected ? 'Ôter la protection' : 'Protéger la feuille', icon: 'lock-sheet', on: sheetOwnProtected, run: onprotect ? () => onprotect('sheet') : undefined, hint: sheetOwnProtected ? 'Ôter la protection de la feuille active' : 'Protéger la feuille active : plus aucune modification de son contenu (mot de passe facultatif)' },
					{ label: workbookProtected ? 'Ôter la protection du classeur' : 'Protéger le classeur', icon: 'lock-book', on: workbookProtected, run: onprotect ? () => onprotect('workbook') : undefined, hint: workbookProtected ? 'Ôter la protection de la structure du classeur' : 'Protéger la structure du classeur : ajouter, supprimer, renommer ou déplacer des feuilles est refusé' }
				]
			}
		],
		Affichage: [
			{ title: 'Afficher', commands: [{ label: 'Quadrillage', icon: 'border-all', on: gridlines, run: ongridlines ? () => ongridlines(!gridlines) : undefined, hint: gridlines ? 'Masquer le quadrillage de la feuille' : 'Afficher le quadrillage de la feuille' }, { label: 'En-têtes A1', icon: 'headings', on: headings, run: onheadings ? () => onheadings(!headings) : undefined, hint: headings ? 'Lettres et numéros seulement pendant la saisie d’une formule' : 'Toujours afficher les lettres de colonnes et numéros de lignes' }, { label: 'Animations', icon: 'play', on: animate, run: onanimate ? () => onanimate(!animate) : undefined, hint: animate ? 'Désactiver l’animation du cadre de sélection' : 'Animer le cadre de sélection (il glisse vers la nouvelle cellule)' }, {
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
					}, {
						label: 'Zoom',
						icon: 'zoom',
						run: onzoom ? openItems : undefined,
						on: Math.abs(zoom - 1) > 0.001,
						hint: `Zoom de la feuille : ${zoomLabel(zoom)} (Ctrl+molette, ou le curseur de la barre d’état)`,
						items: ZOOM_LEVELS.map((z) => ({ label: zoomLabel(z), run: onzoom ? () => onzoom(z) : undefined, on: Math.abs(zoom - z) < 0.001 }))
					}, { label: '100 %', icon: 'zoom-100', run: onzoom ? () => onzoom(1) : undefined, hint: 'Revenir au zoom 100 %' }] },
			...(onbench ? [{ title: 'Mesures (prototype)', commands: [{ label: 'Mesurer la grille', icon: 'stopwatch' as IconName, run: () => onbench() }] }] : [])
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
					{ label: 'Annuler', icon: 'undo', run: dc('undo'), hint: 'Annuler la dernière modification du texte (Ctrl+Z)' },
					{ label: 'Rétablir', icon: 'redo', run: dc('redo'), hint: 'Rétablir (Ctrl+Y)' }
				]
			},
			{ title: 'Presse-papiers', iconOk: true, commands: [{ label: 'Coller', icon: 'paste', run: dc('paste'), hint: 'Coller le texte (Ctrl+V)' }, { label: 'Couper', icon: 'cut', run: dc('cut'), hint: 'Couper (Ctrl+X)' }, { label: 'Copier', icon: 'copy', run: dc('copy'), hint: 'Copier (Ctrl+C)' }] },
			{
				title: 'Police',
				fonts: true,
				iconOk: true,
				commands: [
					{ label: 'Gras', icon: 'bold', run: st({ b: 'toggle' }), hint: 'Gras (Ctrl+B)', on: !!cellStyle?.b },
					{ label: 'Italique', icon: 'italic', run: st({ i: 'toggle' }), hint: 'Italique (Ctrl+I)', on: !!cellStyle?.i },
					{ label: 'Souligné', icon: 'underline', run: st({ u: 'toggle' }), hint: 'Souligné (Ctrl+U)', on: !!cellStyle?.u },
					{ label: 'Barré', icon: 'strike', run: dc('strike'), hint: 'Barré', on: !!docState?.strike },
					{ label: 'Couleur', icon: 'font-color', run: st({ fg }), hint: 'Couleur du texte (▾ : choisir la teinte)', color: 'fg' },
					{ label: 'Surlignage', icon: 'font-color', run: st({ bg }), hint: 'Couleur de surlignage (▾ : choisir la teinte)', color: 'bg' },
					{ label: 'Effacer', icon: 'remove-duplicates', run: st(clearPatch()), hint: 'Effacer la mise en forme des caractères' }
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
			{ title: 'Exporter', commands: [{ label: 'Exporter en PDF', icon: 'file', run: dc('pdf'), hint: 'Enregistrer le document en PDF, graphiques et tableaux croisés compris (imprimante « Microsoft Print to PDF »)' }] }
		],
		Insertion: [
			{ title: 'Tableaux', commands: [{ label: 'Tableau croisé', icon: 'pivot-table', run: dc('pivot'), hint: 'Tableau croisé dynamique d’un tableau du classeur, inséré dans le texte et recalculé avec lui' }] },
			{
				title: 'Graphiques',
				commands: [
					{ label: 'Barres', icon: 'chart-bar', run: dc('chart:bar'), hint: 'Graphique en barres d’un tableau du classeur, inséré dans le texte' },
					{ label: 'Courbes', icon: 'chart-line', run: dc('chart:line'), hint: 'Graphique en courbes d’un tableau du classeur' },
					{ label: 'Secteurs', icon: 'chart-pie', run: dc('chart:pie'), hint: 'Graphique en secteurs d’un tableau du classeur' },
					{ label: 'Nuage de points', icon: 'chart-scatter', run: dc('chart:scatter'), hint: 'Nuage de points d’un tableau du classeur' },
					{ label: 'Autres graphiques', icon: 'chart-more', run: openItems, hint: 'Combiné, 3D, carte, cascade, compartimentage, radar, entonnoir, boîte à moustaches, fréquences, jauge', items: MORE_KINDS.map((k) => ({ label: k.label, run: dc(`chart:${k.kind}`) })) }
				]
			},
			{
				title: 'Contrôles',
				commands: [
					{ label: 'Bouton', icon: 'button', run: dc('button'), hint: 'Bouton dans le texte : un clic exécute un script de l’onglet SCRIPT (ou l’une de ses fonctions)' },
					{ label: 'Champ', icon: 'text-input', run: dc('input'), hint: 'Contrôle lié à une cellule d’une feuille : champ texte, zone de texte, case à cocher, liste déroulante ou fichier, pilotable par les scripts (controls.*)' }
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
		Affichage: [{ title: 'Afficher', commands: sheetRibbons.Affichage[0].commands.filter((c) => c.label === 'Animations') }]
	});
	// Page web : elle a ses propres commandes (Modifier, et Site
	// web… si Fichier › Paramètres › Fonctionnalités le permet).
	const htmlRibbons: Record<string, Group[]> = $derived({
		Accueil: sheetRibbons.Accueil.filter((g) => g.title === 'Annuler'),
		Révision: sheetRibbons.Révision.filter((g) => g.title === 'Protéger'),
		Affichage: docRibbons.Affichage
	});
	const allRibbons: Record<string, Group[]> = $derived(context === 'doc' ? docRibbons : context === 'html' ? htmlRibbons : sheetRibbons);
	// Onglets du ruban qui ont des commandes pour l'onglet affiché.
	const shownTabs = $derived(tabs.filter((t) => (allRibbons[t]?.length ?? 0) > 0));
	$effect(() => {
		if (!isView(tab) && !shownTabs.includes(tab as (typeof tabs)[number])) tab = 'Accueil';
	});

	// Sur une feuille protégée, seules les commandes qui ne la modifient pas
	// restent actives (lecture, copie, filtres, affichage) ; un tableau croisé
	// crée une feuille : refusé quand la structure du classeur est protégée.
	const READ_ONLY = new Set(['Annuler', 'Rétablir', 'Copier', 'Rechercher', 'Filtrer', 'Erreurs seulement', 'Afficher les liens', 'Calculer', 'Quadrillage', 'En-têtes A1', 'Animations', 'Figer les volets', 'Zoom', '100 %', 'Mesurer la grille', 'Tableau croisé', 'Protéger la feuille', 'Protéger le classeur', 'Ôter la protection', 'Ôter la protection du classeur', 'Texte/CSV', 'Classeur Excel', 'Autres fichiers', 'Base de données', 'Web', 'À partir du tableau', 'Requêtes et connexions', 'Actualiser tout']);
	const ribbons: Record<string, Group[]> = $derived(
		Object.fromEntries(
			Object.entries(allRibbons).map(([k, gs]) => [
				k,
				gs.map((g) => ({
					...g,
					commands: g.commands.map((c) =>
						c.run && sheetProtected && !READ_ONLY.has(c.label)
							? { ...c, run: undefined, hint: sheetOwnProtected ? `${c.label} : feuille protégée (Révision › Ôter la protection)` : `${c.label} : classeur protégé (Révision › Ôter la protection du classeur)` }
							: c.run && workbookProtected && c.label === 'Tableau croisé'
								? { ...c, run: undefined, hint: 'Tableau croisé : le classeur est protégé (il crée une feuille)' }
								: c
					)
				}))
			])
		)
	);

	// Largeur réelle des libellés (police du ruban), pour l'estimation de fit.
	let measureCtx: CanvasRenderingContext2D | null = null;
	const measure: Measure = (s, px) => {
		measureCtx ??= typeof document === 'undefined' ? null : document.createElement('canvas').getContext('2d');
		if (!measureCtx) return estimate(s, px);
		measureCtx.font = `${px}px 'Segoe UI', system-ui, -apple-system, sans-serif`;
		return measureCtx.measureText(s).width;
	};
	type ShapesArr = GroupShape[];
	const shapes = $derived(
		isView(tab)
			? []
			: (ribbons[tab] ?? []).map((g) => ({ title: g.title, iconOk: g.iconOk, labels: [...(g.fonts ? [FONTS_W] : []), ...g.commands.map((c) => (c.color ? null : c.items || c.menu ? c.label + ' ▾' : c.label))] }))
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
			const r = actual && est ? actual / est : 1;
			if (Math.abs(r - ratio) / ratio > 0.03) ratio = r;
		});
	});

	function select(t: typeof tab) {
		tab = t;
		onview(t === 'IA' ? 'ai' : t === 'SQL' ? 'sql' : t === 'SCRIPT' ? 'script' : 'grid');
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
		if (!t || t === 'Fichier' || t === 'SQL' || t === 'SCRIPT') {
			kt = null;
			el.click(); // Fichier : son menu prend le focus (ouvert au clavier) ; ? : l'aide
			return;
		}
		select(t as typeof tab);
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
		// Vue changée sans le ruban (retour de la pipette de l'onglet SCRIPT).
		else if (view === 'script' && tab !== 'SCRIPT') tab = 'SCRIPT';
		else if (view === 'sql' && tab !== 'SQL') tab = 'SQL';
		else if (view === 'ai' && tab !== 'IA') tab = 'IA';
	});
</script>

{#snippet colorCmd(c: Command)}
	{@const kind = c.color!}
		<div class="color-cmd">
			<button class="cmd" disabled={!c.run} onclick={c.run} title={c.hint} data-kt={c.label}>
				<!-- La lettre montre l'effet : couleur du texte, ou fond rempli. -->
				<span
					class="swatch-icon"
					style:color={kind === 'fg' ? fg : '#1f2328'}
					style:background={kind === 'bg' ? bg : 'transparent'}
					aria-label={c.label}>A</span
				>
			</button>
			<button class="drop" disabled={!c.run} onclick={(ev) => openPalette(kind, ev)} title="Choisir la couleur" aria-label="{c.label} : choisir la couleur" aria-haspopup="menu" data-kt="{c.label} ▾">▾</button>
			{#if palette === kind}
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="palette" style:left="{palettePos.x}px" style:top="{palettePos.y}px" use:dismiss={() => (palette = null)}>
					<div class="grid">
						{#each COLORS as col (col)}
							<button class="chip" style:background={col} title={col} aria-label={col} onclick={() => pick(kind, col)}></button>
						{/each}
					</div>
					{#if kind === 'bg'}<button class="row" onclick={() => pick('bg', null)}>Aucun remplissage</button>{/if}
					{#if kind === 'fg'}<button class="row" onclick={() => pick('fg', null)}>Automatique</button>{/if}
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
				<div class="file-menu" use:dismiss={() => (fileOpen = false)}>
					{#each [['new', 'Nouveau', 'Ctrl+N'], ['open', 'Ouvrir…', 'Ctrl+O']] as [a, label, keys] (a)}
						<button onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span><span class="keys">{keys}</span></button>
					{/each}
					<div class="recent-wrap" role="presentation" onmouseenter={() => (recentOpen = true)} onmouseleave={() => (recentOpen = false)}>
						<button disabled={!recent.length} onclick={() => (recentOpen = !recentOpen)}><span>Récents</span><span class="keys">▸</span></button>
						{#if recentOpen && recent.length}
							<div class="file-menu sub">
								{#each recent as p (p)}
									<button title={p} onclick={() => { fileOpen = recentOpen = false; onrecent?.(p); }}><span>{fileName(p)}</span></button>
								{/each}
							</div>
						{/if}
					</div>
					<hr />
					{#each [['save', 'Enregistrer', 'Ctrl+S'], ['saveAs', 'Enregistrer sous…', 'F12'], ['import', 'Importer (CSV, Excel)…', ''], ['export', 'Exporter le tableau en CSV…', ''], ['exportXlsx', 'Exporter le classeur (.xlsx)…', ''], ['pdf', 'Exporter en PDF…', ''], ['print', 'Imprimer…', 'Ctrl+P'], ['close', 'Fermer', 'Ctrl+W'], ['mcp', 'Serveur MCP (assistants)…', ''], ['settings', 'Paramètres…', ''], ['help', 'Aide', 'F1'], ...(import.meta.env.DEV ? [['demo', 'Démo : 1 M de lignes', '']] : [])] as [a, label, keys] (a)}
						<button disabled={disabled && a !== 'import' && a !== 'demo' && a !== 'mcp' && a !== 'settings' && a !== 'help'} onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span><span class="keys">{keys}</span></button>
					{/each}
				</div>
			{/if}
		</div>
		{#each shownTabs as t (t)}
			<button class="tab" data-label={t} class:on={tab === t} aria-pressed={tab === t} aria-keyshortcuts="Alt+{TAB_TIPS[t]}" data-region-focus={tab === t ? '' : undefined} {disabled} onclick={() => select(t)}>{t}</button>
		{/each}
		<span class="sep"></span>
		{#if aiEnabled() || view === 'ai'}
			<button class="tab ai" data-label="Assistant ✦" class:on={tab === 'IA'} aria-pressed={tab === 'IA'} aria-keyshortcuts="Alt+{TAB_TIPS['Assistant ✦']}" data-region-focus={tab === 'IA' ? '' : undefined} {disabled} onclick={() => select('IA')} title="Assistant IA"
				>Assistant <Icon name="sparkle" size={13} /></button
			>
		{/if}
		<button class="tab special" data-label="SQL" class:on={tab === 'SQL'} aria-pressed={tab === 'SQL'} aria-keyshortcuts="Alt+{TAB_TIPS.SQL}" data-region-focus={tab === 'SQL' ? '' : undefined} {disabled} onclick={() => select('SQL')}>SQL</button>
		<button class="tab special" data-label="SCRIPT" class:on={tab === 'SCRIPT'} aria-pressed={tab === 'SCRIPT'} aria-keyshortcuts="Alt+{TAB_TIPS.SCRIPT}" data-region-focus={tab === 'SCRIPT' ? '' : undefined} {disabled} onclick={() => select('SCRIPT')}>SCRIPT</button>
		<button class="tab help" title="Aide (F1)" aria-label="Aide" onclick={() => onfile('help')}>?</button>
	</nav>
	{#if !isView(tab)}
		<div class="groups" class:off={disabled} use:observe>
			{#each ribbons[tab] ?? [] as g, gi (g.title)}
				{@const mode = modes[gi] ?? 'large'}
				<section class="group">
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
												class:pressed={it.on}
												disabled={!c.run || !it.run}
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
											class:pressed={c.on}
											disabled={!c.run}
											onclick={(ev) => {
												anchorAt(ev);
												openGroup = null;
												c.run?.();
											}}
											title={c.hint ?? (c.run ? c.label : `${c.label} : à venir`)}
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
										aria-label={mode === 'icon' ? c.label : undefined}
										data-kt={c.label}
										disabled={!c.run}
										onclick={c.items
											? (ev) => toggleDrop(c, ev)
											: (ev) => {
													anchorAt(ev);
													c.run?.();
												}}
										title={c.hint ?? (c.run ? c.label : `${c.label} : à venir`)}
									>
										<Icon name={c.icon} size={mode === 'large' ? 24 : 16} />
										{#if mode !== 'icon'}<span class="label">{c.label}{c.items || c.menu ? ' ▾' : ''}</span>{/if}
									</button>
								{/if}
							{/each}
						</div>
					{/if}
					<div class="title">
						{g.title}
						{#if g.launch}
							<button class="launch" onclick={g.launch} title={g.launchHint ?? g.title} aria-label={g.launchHint ?? g.title} data-kt="{g.title} …">
								<svg viewBox="0 0 10 10" width="9" height="9" aria-hidden="true"><path d="M1.5 1.5h4M1.5 1.5v4M3.5 3.5l5 5M8.5 4.5v4h-4" fill="none" stroke="currentColor" stroke-width="1.1" /></svg>
							</button>
						{/if}
					</div>
				</section>
			{/each}
		</div>
	{/if}
	{#if dropMenu}
		{@const c = ribbons[tab]?.flatMap((g) => g.commands).find((x) => x.label === dropMenu!.label)}
		{#if c?.items}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="drop-menu" style:left="{dropMenu.x}px" style:top="{dropMenu.y}px" use:dismiss={() => (dropMenu = null)}>
				{#each c.items as it (it.label)}
					<button
						disabled={!it.run}
						class:on={it.on}
						onclick={() => {
							dropMenu = null;
							it.run?.();
						}}><span class="check">{it.on ? '✓' : ''}</span>{it.label}</button
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
	.recent-wrap {
		position: relative;
	}
	.file-menu.sub {
		top: -5px;
		left: 100%;
		max-width: 420px;
	}
	.file-menu.sub span {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
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
	.groups {
		display: flex;
		height: 78px;
		padding: 4px 6px;
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
</style>
