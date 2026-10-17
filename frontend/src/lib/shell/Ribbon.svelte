<script lang="ts">
	// Ruban façon Excel : onglets en haut, groupes de commandes en dessous.
	// SQL et SCRIPT sont des onglets à part entière : ils prennent la place de
	// la grille (précédence sur GRID). Les commandes pas encore réalisées sont
	// affichées désactivées, pour montrer l'organisation cible.

	import { untrack } from 'svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { dismiss } from '#lib/grid/dismiss.js';
	import { clearPatch, type CellStyle } from '#lib/grid/style.js';
	import { DEFAULT_PT, FONTS, SIZES } from '#lib/grid/textwrap.js';
	import { ZOOM_LEVELS, zoomLabel } from '#lib/grid/zoom.js';
	import Select from '#lib/ui/Select.svelte';
	import { fit, groupWidth, type GroupShape, type Mode } from './fit';
	import type { IconName } from '#lib/icons/icons.js';

	export type View = 'grid' | 'sql' | 'script';
	export type FormatTab = 'number' | 'align' | 'font' | 'border' | 'fill';
	export type FreezeAction = 'cell' | 'row' | 'col' | 'none';

	interface Command {
		label: string;
		icon: IconName;
		run?: () => void;
		hint?: string;
		color?: 'fg' | 'bg'; // commande avec un choix de couleur
		on?: boolean; // commande à bascule, enfoncée
		/** Commande à menu (▾) : ses choix. */
		items?: { label: string; run?: () => void; on?: boolean }[];
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

	export type FileAction = 'new' | 'open' | 'save' | 'saveAs' | 'import' | 'export' | 'exportXlsx' | 'print' | 'pdf' | 'close' | 'demo' | 'mcp';

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
		onrules?: () => void;
		onfilter?: (errorsOnly: boolean) => void;
		onrecalc?: () => void;
		ondedup?: () => void;
		onpivot?: () => void;
		onformula?: (what: 'insert' | 'sum' | 'reference') => void;
		onclip?: (what: 'cut' | 'copy' | 'paste') => void;
		onpainter?: () => void;
		onaudit?: (kind: 'pre' | 'dep' | 'clear') => void;
		onchart?: (kind: 'bar' | 'line' | 'pie' | 'scatter') => void;
		oncells?: (op: 'insertRow' | 'deleteRow' | 'insertColumn' | 'deleteColumn') => void;
		/** Quadrillage de la feuille active, et sa bascule. */
		gridlines?: boolean;
		ongridlines?: (show: boolean) => void;
		/** En-têtes A1 (lettres et numéros) toujours affichés, et leur bascule. */
		headings?: boolean;
		onheadings?: (show: boolean) => void;
		/** Insertion › Bouton : bouton d'action lié à un script. */
		onbutton?: () => void;
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

	let { view, title, dirty, undo, redo, disabled = false, recent = [], onrecent, onview, onfile, onundo, onredo, onbench, onsort, onstyle, onfind, onrules, onfilter, onrecalc, ondedup, onpivot, onchart, onformula, onclip, onpainter, onaudit, oncells, gridlines = true, ongridlines, headings = false, onheadings, onbutton, sheetProtected = false, sheetOwnProtected = sheetProtected, workbookProtected = false, onprotect, ondata, onformat, onmerge, onwrap, onlines, onvalidation, onnote, freeze, onfreeze, cellStyle, merged = false, zoom = 1, onzoom, animate = true, onanimate }: Props = $props();
	const fmt = (tab: FormatTab) => (onformat ? () => onformat(tab) : undefined);
	const lines = (rows: boolean, hide: boolean) => (onlines ? () => onlines(rows, hide) : undefined);
	const frz = (what: FreezeAction) => (onfreeze ? () => onfreeze(what) : undefined);
	const frozen = $derived(!!freeze && (freeze.rows > 0 || freeze.cols > 0));
	// Menu d'une commande à choix (Figer les volets, Format) : son run ne sert
	// qu'à dire qu'elle est active, le clic ouvre ses choix.
	let dropMenu = $state<{ label: string; x: number; y: number } | null>(null);
	const openItems = () => undefined;
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
	let tab = $state<(typeof tabs)[number] | 'SQL' | 'SCRIPT'>('Accueil');
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

	const allRibbons: Record<string, Group[]> = $derived({
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
					{ label: 'Date', icon: 'type-date', run: st({ nf: 'date', df: 'jj/mm/aaaa', dp: null, ng: null }), hint: 'Date courte (jj/mm/aaaa) ; Ctrl+1 pour les autres formats de date et d’heure' },
					{ label: 'Standard', icon: 'number-format', run: st({ nf: null, dp: null, ng: null, df: null }), hint: 'Format Standard' },
					{ label: 'Ajouter une décimale', icon: 'decimal-more', run: st({ dp: 'more' }) },
					{ label: 'Réduire les décimales', icon: 'decimal-less', run: st({ dp: 'less' }) }
				]
			},
			{
				title: 'Styles',
				commands: [{ label: 'Mise en forme conditionnelle', icon: 'warning', run: onrules, hint: 'Règles de mise en forme de la colonne active' }]
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
					{ label: 'Nuage de points', icon: 'chart-scatter', run: onchart ? () => onchart('scatter') : undefined, hint: 'Nuage de points du tableau actif' }
				]
			},
			{ title: 'Automatisation', commands: [{ label: 'Bouton', icon: 'button', run: onbutton, hint: 'Bouton d’action à la cellule active : un clic exécute un script de l’onglet SCRIPT' }] }
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
					{ label: 'Actualiser tout', icon: 'refresh', run: data('refreshAll'), hint: 'Relire les sources de toutes les requêtes chargées et remplacer leurs tableaux' }
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
			{ title: 'Outils de données', commands: [{ label: 'Supprimer les doublons', icon: 'remove-duplicates', run: ondedup, hint: 'Supprimer les lignes en double du tableau actif' }, { label: 'Convertir le type', icon: 'convert-type' }, { label: 'Erreurs seulement', icon: 'warning', run: onfilter ? () => onfilter(true) : undefined, hint: 'Afficher seulement les saisies hors type de la colonne active (bascule)' }, { label: 'Validation des données', icon: 'validation', run: onvalidation, hint: 'Liste déroulante : valeurs permises dans la sélection (ou toute une colonne de tableau), les autres saisies sont refusées' }] }
		],
		Formules: [
			{ title: 'Bibliothèque', commands: [{ label: 'Insérer une fonction', icon: 'function', run: onformula ? () => onformula('insert') : undefined, hint: 'Choisir une fonction et commencer la formule dans la cellule active' }, { label: 'Somme automatique', icon: 'autosum', run: onformula ? () => onformula('sum') : undefined, hint: 'SOMME de la colonne du tableau au-dessus' }, { label: 'REFERENCE', icon: 'reference', run: onformula ? () => onformula('reference') : undefined, hint: 'REFERENCE(Tableau[Colonne]; clé) : la RECHERCHEV de Slitable' }] },
			{ title: 'Audit', commands: [{ label: 'Antécédents', icon: 'precedents', run: onaudit ? () => onaudit('pre') : undefined, hint: 'Flèches depuis ce que lit la formule de la cellule active' }, { label: 'Dépendants', icon: 'dependents', run: onaudit ? () => onaudit('dep') : undefined, hint: 'Flèches vers les formules qui lisent la cellule active' }, { label: 'Effacer les flèches', icon: 'dependency-graph', run: onaudit ? () => onaudit('clear') : undefined, hint: 'Effacer les flèches (Échap)' }] },
			{ title: 'Calcul', commands: [{ label: 'Calculer', icon: 'function', run: onrecalc, hint: 'Renouveler AUJOURDHUI, MAINTENANT et ALEA (F9)' }] }
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
					}, { label: '100 %', icon: 'zoom', run: onzoom ? () => onzoom(1) : undefined, hint: 'Revenir au zoom 100 %' }] },
			...(onbench ? [{ title: 'Mesures (prototype)', commands: [{ label: 'Mesurer la grille', icon: 'stopwatch' as IconName, run: () => onbench() }] }] : [])
		]
	});

	// Sur une feuille protégée, seules les commandes qui ne la modifient pas
	// restent actives (lecture, copie, filtres, affichage) ; un tableau croisé
	// crée une feuille : refusé quand la structure du classeur est protégée.
	const READ_ONLY = new Set(['Annuler', 'Rétablir', 'Copier', 'Rechercher', 'Filtrer', 'Erreurs seulement', 'Antécédents', 'Dépendants', 'Effacer les flèches', 'Calculer', 'Quadrillage', 'En-têtes A1', 'Animations', 'Figer les volets', 'Zoom', '100 %', 'Mesurer la grille', 'Tableau croisé', 'Protéger la feuille', 'Protéger le classeur', 'Ôter la protection', 'Ôter la protection du classeur', 'Texte/CSV', 'Classeur Excel', 'Autres fichiers', 'Base de données', 'Web', 'À partir du tableau', 'Requêtes et connexions', 'Actualiser tout']);
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

	type ShapesArr = GroupShape[];
	const shapes = $derived(
		tab === 'SQL' || tab === 'SCRIPT'
			? []
			: ribbons[tab].map((g) => ({ title: g.title, iconOk: g.iconOk, labels: [...(g.fonts ? [FONTS_W] : []), ...g.commands.map((c) => (c.color ? null : c.label))] }))
	);
	// Les formes ne changent qu'avec les libellés : un clic dans la grille
	// (annulation, quadrillage…) recrée les commandes sans recompacter le
	// ruban ni remesurer (sinon les boutons changent de taille).
	const shapesKey = $derived(JSON.stringify(shapes));
	const modesKey = $derived(
		tab === 'SQL' || tab === 'SCRIPT' ? '' : fit(JSON.parse(shapesKey), (groupsW - 12) / ratio).join(',')
	);
	const modes: Mode[] = $derived(modesKey ? (modesKey.split(',') as Mode[]) : []);
	$effect(() => {
		const m = modes;
		if (!groupsEl || !m.length) return;
		const shapes = JSON.parse(untrack(() => shapesKey)) as ShapesArr;
		requestAnimationFrame(() => {
			const actual = [...groupsEl.children].reduce((s, c) => s + c.getBoundingClientRect().width, 0);
			const est = shapes.reduce((s, g, i) => s + groupWidth(g, m[i] ?? 'large'), 0);
			const r = actual && est ? actual / est : 1;
			if (Math.abs(r - ratio) / ratio > 0.03) ratio = r;
		});
	});

	function select(t: typeof tab) {
		tab = t;
		onview(t === 'SQL' ? 'sql' : t === 'SCRIPT' ? 'script' : 'grid');
	}

	$effect(() => {
		// Revenir à la grille depuis la page (onglet de feuille) remet un onglet de ruban.
		if (view === 'grid' && (tab === 'SQL' || tab === 'SCRIPT')) tab = 'Accueil';
	});
</script>

{#snippet colorCmd(c: Command)}
	{@const kind = c.color!}
		<div class="color-cmd">
			<button class="cmd" disabled={!c.run} onclick={c.run} title={c.hint}>
				<!-- La lettre montre l'effet : couleur du texte, ou fond rempli. -->
				<span
					class="swatch-icon"
					style:color={kind === 'fg' ? fg : '#1f2328'}
					style:background={kind === 'bg' ? bg : 'transparent'}
					aria-label={c.label}>A</span
				>
			</button>
			<button class="drop" disabled={!c.run} onclick={(ev) => openPalette(kind, ev)} title="Choisir la couleur">▾</button>
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

<header class="ribbon">
	<nav class="tabs">
		<div class="file-wrap">
			<button class="tab file" data-label="Fichier" onclick={() => (fileOpen = !fileOpen)}>Fichier</button>
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
					{#each [['save', 'Enregistrer', 'Ctrl+S'], ['saveAs', 'Enregistrer sous…', 'F12'], ['import', 'Importer (CSV, Excel)…', ''], ['export', 'Exporter le tableau en CSV…', ''], ['exportXlsx', 'Exporter le classeur (.xlsx)…', ''], ['pdf', 'Exporter en PDF…', ''], ['print', 'Imprimer…', 'Ctrl+P'], ['close', 'Fermer', 'Ctrl+W'], ['mcp', 'Serveur MCP (assistants)…', ''], ...(import.meta.env.DEV ? [['demo', 'Démo : 1 M de lignes', '']] : [])] as [a, label, keys] (a)}
						<button disabled={disabled && a !== 'import' && a !== 'demo' && a !== 'mcp'} onclick={() => { fileOpen = false; onfile(a as FileAction); }}><span>{label}</span><span class="keys">{keys}</span></button>
					{/each}
				</div>
			{/if}
		</div>
		{#each tabs as t (t)}
			<button class="tab" data-label={t} class:on={tab === t} {disabled} onclick={() => select(t)}>{t}</button>
		{/each}
		<span class="sep"></span>
		<button class="tab special" data-label="SQL" class:on={tab === 'SQL'} {disabled} onclick={() => select('SQL')}>SQL</button>
		<button class="tab special" data-label="SCRIPT" class:on={tab === 'SCRIPT'} {disabled} onclick={() => select('SCRIPT')}>SCRIPT</button>
	</nav>
	{#if tab !== 'SQL' && tab !== 'SCRIPT'}
		<div class="groups" class:off={disabled} use:observe>
			{#each ribbons[tab] as g, gi (g.title)}
				{@const mode = modes[gi] ?? 'large'}
				<section class="group">
					{#if mode === 'collapsed'}
						<div class="commands">
							<button
								class="cmd"
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
											onclick={() => {
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
										disabled={!c.run}
										onclick={c.items ? (ev) => toggleDrop(c, ev) : c.run}
										title={c.hint ?? (c.run ? c.label : `${c.label} : à venir`)}
									>
										<Icon name={c.icon} size={mode === 'large' ? 24 : 16} />
										{#if mode !== 'icon'}<span class="label">{c.label}{c.items ? ' ▾' : ''}</span>{/if}
									</button>
								{/if}
							{/each}
						</div>
					{/if}
					<div class="title">
						{g.title}
						{#if g.launch}
							<button class="launch" onclick={g.launch} title={g.launchHint ?? g.title} aria-label={g.launchHint ?? g.title}>
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
		background: rgba(255, 255, 255, 0.12);
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
