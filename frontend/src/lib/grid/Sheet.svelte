<script lang="ts">
	// Feuille façon Excel : une grille infinie (1 048 576 lignes × 16 384
	// colonnes, comme Excel) sur laquelle sont posés des tableaux et des
	// cellules libres. Pas d'adressage A1 : les numéros de ligne et les noms
	// de colonnes ne s'affichent que pour les tableaux, et le bandeau du haut
	// reprend les noms de colonnes d'un tableau quand son en-tête sort de
	// l'écran (comme Excel). Saisir n'importe où passe par la source, qui
	// décide (cellule libre, nouveau tableau, extension, élargissement).
	// Rendu <canvas> de la zone visible, cache de blocs de lignes par tableau,
	// éditeur DOM superposé à la cellule active, barre de formule.
	import { flushSync, onMount, untrack } from 'svelte';
	import { noticeTtl } from '#lib/shell/notice.js';
	import { RowCache, percentile, syncAt, type Fetched, type SyncKind } from './cache';
	import { VScroll, clamp } from './scroll';
	import { Axis } from './axis';
	import { DEFAULT_NAME_HINT, isDefaultTableName } from './defaultname';
	import { ease, flashAlpha, lerpBar, motionAt, progress, reducedMotion, sameBar, shiftAxis, tableMotions, zeroBar, type AxisLike, type BarSpan, type Geo, type LinesShift, type Motion } from './anim';
	import { followClip, overlaps, parseTSV, pasteArea, richClip, sameClipText, tileBlock, toTSV, type Clip, type Rect as ClipRect } from './clipboard';
	import { boxStyle, formatStat, isNumericText, painterPatch, parseStyle, resolveStyle, type CellStyle, type StyleLayer } from './style';
	import { formatCell } from './numfmt';
	import { autoHeight, BASE_PX, cellFont, fontPx, headerHeight, headerTextWidth, lineHeight, wrapLines } from './textwrap';
	import { AutoRows } from './rowfit';
	import { arrowNotes, arrowSpan, spanBox, spanPoint, type ArrowGeometry } from './auditarrows';
	import { stepZoom } from './zoom';
	import { panes, reveal, revealWhole, toGrid, type Pane } from './panes';
	import { cleanSource, dvDialog, dvPatch, dvTarget, dvPreview, dvSource, hiddenLines, mergeAt, mergeList, skipHidden, validate, validateSource, type DvList, type MergeRange } from './sheetfmt';
	import { expandRange, extendCorner, intersect, isSingleCell, mergeRect, rangeOf, stepCell, wholeLines, type CellRange } from './mergesel';
	import { jumpLine, lineSpans, type Area, type Span } from './jump';
	import { findKey, finderAction, findZone, foundCell, hitCount, inZone, nextHit, replacedLabel, zoneLabel, type Zone } from './findzone';
	import { edgeScroll, frameStep } from './autoscroll';
	import { blocksReach, boxesReach } from './objextent';
	import FormatDialog, { type FormatTab } from './FormatDialog.svelte';
	import TextToColumns from './TextToColumns.svelte';
	import CustomSort from './CustomSort.svelte';
	import { withoutField, type CustomList, type CustomOrder } from './customsort';
	import { handQuestion, keyQuestion, type KeyChoice, type KeyQuestion } from './keychoice';
	import type { TextColumnsOptions, TextColumnsPreview } from './textcols';
	import { dismiss, dismissPanel } from './dismiss';
	import { explainError } from './errors';
	import { freeDisplay } from './freecell';
	import { bandAt, bandLabel, bandOpen, bandRefusal, hiddenBand } from './hidden';
	import { choiceHiddenCols, type TabChoice } from '#lib/shell/choice.js';
	import { tabChoiceItem, type ChoiceCandidate, type TabChoiceItem } from './tabchoice';
	import { ReadRecovery, readFailNotice } from './readfail';
	import { followLabel } from '#lib/shell/mergesheets.js';
	import { editMode, viewKey } from './readonlyedit';
	import { bandMerges, groupSpans, headerLabel, isSingle, spanAt, splitGroup, tableTop } from './twolevel';
	import { grown as growFrame, type Grown } from './grown';
	import { pivotLayer, pivotValueText } from './pivotformat';
	import { canFoldAll, foldAll, foldTarget, pivotBoxHit, pivotBoxLeft, pivotLabelOffset, PIVOT_BOX, setFolded, type PivotRowInfo } from './pivottree';
	import { fillSeries, shiftTableColumns } from './fill';
	import {
		condArgs,
		condLabel,
		condOps,
		describeConds,
		filterKind,
		kindTitle,
		searchValues,
		valuesFilter,
		type ColumnFilter as Filter,
		type CondOp,
		type PanelValue
	} from './filtercond';
	import { cellLines, copyFillBlock, copyFillPlan, fillBlock, insertAt, readKey, shortcutOf, timeText, todayText } from './shortcuts';
	import { SheetViews } from './sheetview';
	import { besideGutter, gutterOwner } from './gutter';
	import Select from '#lib/ui/Select.svelte';
	import Submenu from '#lib/ui/Submenu.svelte';
	import { modal } from '#lib/ui/focus.js';
	import { clip, clipped } from '#lib/ui/menu.js';
	import Chart from '#lib/chart/Chart.svelte';
	import { encodeObj, objFromText, rememberObj } from '#lib/clip/objects.js';
	import { SINGLE_SERIES, chartFieldsFrom, defaultChartTitle, type ChartKind, type MapGuess } from '#lib/chart/kinds.js';
	import ChartPanel from '#lib/chart/ChartPanel.svelte';
	import type { BoxStatsIn } from '#lib/chart/shapes.js';
	import { svgToPng } from '#lib/chart/image.js';
	import PivotPanel from './PivotPanel.svelte';
	import { panelAtCell, changeSource } from './pivotfields';
	import { brokenPivotAt, pivotsCleared } from './pivotzone';
	import SheetButtons from './SheetButtons.svelte';
	import { BUTTON_H, BUTTON_W, type ButtonConfig, type ControlKind, type SheetButton } from './buttons';
	import { applyRules, computeStats, mergeStats, parseRules, prepareStats, reqKey, statsReq, todaySerial, type CfOut, type Rule, type RuleEnv, type RuleStats, type Stats, type StatsReq } from './rules';
	import { formatLocs, groupKey, rectLocs, type Loc, type RuleChanges, type SheetRules } from './cfmodel';
	import { ICON_SETS, drawCfIcon } from './cficons';
	import CondFormat, { type CondHost, type CondView } from './CondFormat.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import GridA11y from './GridA11y.svelte';
	import { selectionLabel, type CellSpeech } from './a11y';
	import { drawIcon } from '#lib/icons/canvas.js';
	import { apply as applySuggestion, argKindAt, needsNextArg, signature, toNextArg, signatureParts, suggest, type Catalog, type Completion, type FunctionDoc } from './autocomplete';
	import { checkFormula, closeParens, editorWidth, formulaRefs, highlightParts, isFormula, movedRef, pointedRef, rangeRef, refRect, serverErrorPos, type FormulaError, type Rect, type RefTable } from './formulaEdit';
	import { formulaTyping, fxLeave, holdFormula, insertRefAt, qualifyRef, updateHold, type FormulaHold } from './crossref';
	import { parseGoto, type GotoTable } from './goto';
	import { cellNameProblem, columnNameProblem, refusedName, tableNameProblem, type NameProblem } from '#lib/ui/names.js';
	import { nameAlertOpen, refuseName } from '#lib/ui/nameAlert.js';
	import {
		isErrorValue,
		valueAlignment,
		alignment,
		defaultWidth,
		editValue,
		formatColumnValue,
		typeIcon,
		typeIconName,
		typeLabel,
		sortLabels,
		type ColumnType
	} from './format';

	export interface Column {
		name: string;
		type: ColumnType;
		fixed?: boolean; // type choisi par l'utilisateur (sinon inféré)
		errors?: number; // saisies hors type
		formula?: string; // colonne calculée : formule affichée
		result?: string; // colonne calculée : type deviné du résultat (« decimal »…)
		hand?: boolean; // tableau chargé par une requête : colonne saisie à côté du résultat (modifiable, en-tête vert)
		style?: string; // mise en forme de toute la colonne (JSON)
		headerStyle?: string;
		rules?: string; // mise en forme conditionnelle (JSON)
		filter?: ColumnFilter; // filtre de la colonne
	}

	/** Colonne passée en texte par élargissement : type de la plupart de ses valeurs. */
	export interface TypeMix {
		type: ColumnType | ''; // vide : pas de type majoritaire
		off: number; // valeurs hors de ce type
		total: number; // valeurs non vides
	}

	export interface PivotValue {
		field?: string; // vide : nombre de lignes
		agg: 'sum' | 'count' | 'countDistinct' | 'avg' | 'min' | 'max';
		show?: string; // '', pctTotal, pctRow, pctCol
		label?: string; // nom personnalisé (« Somme de Ventes » sinon)
		format?: string; // format de nombre imposé (style JSON : nf, dp)
	}

	export interface PivotConfig {
		name?: string; // « TCD1 » : vue SQL et source de graphique
		source: string;
		rowFields?: string[];
		cols?: string;
		values?: PivotValue[];
		filters?: { field: string; values: string[] }[];
		/** Valeurs gardées d'un champ des lignes ou des colonnes (▾ de son étiquette). */
		items?: { field: string; values: string[] }[];
		/** Tri des étiquettes d'un champ : 'asc' ou 'desc' (absent : ordre de la source). */
		sort?: Record<string, 'asc' | 'desc'>;
		/** Ordre personnalisé d'un champ (Tri personnalisé…), qui l'emporte sur sort. */
		orders?: Record<string, CustomOrder>;
		noGrandTotal?: boolean;
		/** Sous-totaux des groupes (plusieurs champs en lignes) : '' en haut (ligne du groupe), 'bottom', 'none'. */
		subtotals?: string;
		/** Groupes pliés (−/+), par leurs étiquettes ; collapseAll : tout plié sauf expanded. */
		collapseAll?: boolean;
		collapsed?: (string[] | null)[] | null;
		expanded?: (string[] | null)[] | null;
	}

	/** Ligne du total général d'un TCD (la dernière), -1 quand il est masqué (« Totaux généraux » décoché ; sans champ en lignes, le total est la seule ligne). */
	function pivotTotalRow(p: { row: number; cells: string[][]; config: PivotConfig }): number {
		return p.config.noGrandTotal && p.config.rowFields?.length ? -1 : p.row + p.cells.length - 1;
	}

	/** Description de la ligne r d'un TCD en arborescence (undefined : en-tête, ou TCD à plat). */
	function pivotRowOf(p: { row: number; rows?: PivotRowInfo[] | null }, r: number): PivotRowInfo | undefined {
		return r > p.row ? (p.rows?.[r - p.row - 1] ?? undefined) : undefined;
	}

	/** Ligne d'un TCD en gras : en-tête, total général, groupes et sous-totaux de l'arborescence (comme Excel). */
	function pivotBold(p: { row: number; cells: string[][]; config: PivotConfig; rows?: PivotRowInfo[] | null }, r: number): boolean {
		if (r === p.row || r === pivotTotalRow(p)) return true;
		const info = pivotRowOf(p, r);
		return !!info && (!!info.group || !!info.subtotal);
	}

	export interface SheetPivot {
		id: number;
		/** Première ligne du TCD affiché (sous ses filtres de rapport). */
		row: number;
		col: number;
		/** Ligne où le TCD est posé : ses filtres de rapport d'abord (comme Excel), une ligne vide, puis le tableau. */
		anchor?: number;
		config: PivotConfig;
		cells: string[][]; // en-tête, groupes, total
		/** Arborescence (plusieurs champs en lignes) : une description par ligne sous l'en-tête. */
		rows?: PivotRowInfo[] | null;
		/** Format de nombre par défaut de chaque colonne (style JSON, voir pivotformat.ts). */
		formats?: string[] | null;
		error: string;
	}

	export interface ChartConfig {
		source: string;
		kind: ChartKind;
		x: string;
		series: string[];
		title?: string;
		width?: number;
		height?: number;
		legend?: string;
		xTitle?: string;
		yTitle?: string;
		labels?: boolean;
		noGrid?: boolean;
		stacked?: boolean;
		yMin?: number;
		yMax?: number;
		colors?: string[];
		/** Séries sur l'axe secondaire (à droite), par rang de série. */
		secondary?: boolean[];
		y2Title?: string;
		/** Graphique combiné : forme de chaque série (bar, line, area). */
		seriesKinds?: string[];
		/** Carte : points (défaut) ou heat (densité). */
		mapMode?: string;
		/** Carte : coordonnées en deux colonnes (pair, défaut) ou en une (single). */
		mapCoords?: string;
		/** Carte qui attend le choix de ses coordonnées (carré grisé). */
		mapDraft?: boolean;
		/** Carte : vue choisie (centre et zoom) ; absente, ajustée aux points. */
		mapView?: { lat: number; lon: number; zoom: number };
		/** Carte de points : colonne dont chaque valeur distincte prend sa couleur. */
		colorBy?: string;
		/** Fréquences : nombre de classes (absent : automatique). */
		bins?: number;
		/** Lit aussi les lignes masquées par le filtre du tableau. */
		hidden?: boolean;
		/** Ne lit que les lignes dont la colonne field affiche l'une des valeurs (Regrouper des onglets : Mois = Janvier). */
		filters?: { field: string; values: string[] | null }[] | null;
		/** Courbes : cellule vide en intervalle (défaut), à zéro (zero) ou reliée (span). */
		blanks?: string;
		offX?: number;
		offY?: number;
	}

	export interface SheetChart {
		id: number;
		row: number;
		col: number;
		config: ChartConfig;
		labels: string[];
		series: { name: string; values: (number | null)[] }[];
		error: string;
		/** Carte : coordonnées illisibles ignorées. */
		invalid?: number;
		/** Carte : colonnes de coordonnées repérées par le classeur. */
		guess?: MapGuess | null;
		/** Carte tirée d'un tableau : ligne (id) de chaque point. */
		rows?: number[] | null;
		/** Carte colorée : valeurs distinctes triées, et rang de la valeur de chaque point (−1 : vide). */
		categories?: string[] | null;
		groups?: number[] | null;
		/** Points de la source ; coupés aux premiers (truncated) ou courbe réduite (sampled), voir ChartExtra. */
		total?: number;
		truncated?: boolean;
		sampled?: boolean;
		xValues?: (number | null)[] | null;
		xDate?: boolean;
		edges?: number[] | null;
		boxes?: ((BoxStatsIn | null)[] | null)[] | null;
		formats?: string[] | null;
		xFormat?: string;
	}

	export interface DepTarget {
		kind: string; // column, table, row (lignes précises d'une colonne), cell
		table?: string;
		column?: string;
		name?: string;
		sheet: number;
		row: number;
		col: number;
		rows?: number; // kind row : lignes encadrées à partir de row
		note?: string; // pourquoi la flèche vise la colonne (ligne masquée…)
	}

	/** Filtre d'une colonne : valeurs, erreurs, conditions (voir filtercond.ts). */
	export type ColumnFilter = Filter;

	/** Sauts de page à dessiner (DocumentService.PageBreaks) : débuts de page, sauts manuels, zone imprimée. */
	export interface PageBreaksView {
		rows: number[];
		cols: number[];
		manualRows: number[];
		manualCols: number[];
		area: { r0: number; c0: number; r1: number; c1: number };
		/** Zone d'impression définie : encadrée. */
		framed?: boolean;
	}

	export interface SheetTable {
		id: number;
		name: string;
		row: number; // ancre : ligne d'en-tête
		col: number;
		rows: number; // lignes de données affichées
		/** Lignes masquées par le filtre (elles gardent leur place sur la feuille). */
		hidden?: number;
		/** Rang de la colonne clé (identifiant des lignes), absent sans clé. */
		key?: number;
		/** Sans clé : la première colonne semble identifier chaque ligne (proposée comme clé). */
		keyHint?: boolean;
		/** Rang de la colonne numérotée d'office (une ligne ajoutée reçoit le numéro suivant), absent sinon. */
		autoNumber?: number;
		/** En-têtes sur deux lignes : le groupe de chaque nom « Groupe.Sous » dans une bande, ligne row - 1. */
		twoLevel?: boolean;
		/** Requête d'import qui charge le tableau (source externe ou autre tableau). */
		query?: { id: number; name: string };
		/**
		 * Colonnes saisies à côté du résultat (docs/decisions/0008) : clé de
		 * jointure (colonne chargée, « # » : rang des lignes ; absente : pas
		 * encore choisie) et lignes disparues dont des valeurs sont mises de côté.
		 */
		handKey?: string;
		handKept?: number;
		/** Classeur partagé : tableau publié sur le serveur, ou resté privé (sur ce poste). */
		shared?: 'published' | 'private';
		/** Tableau publié en lecture seule : le message à afficher (« Classeur publié en lecture seule »). */
		readOnly?: string;
		/** Classeur collaboratif : niveau sur le tableau publié (participant : ses lignes seulement). */
		access?: string;
		/** Lecture filtrée par les droits de l'utilisateur (« Vue filtrée par vos droits »). */
		filtered?: boolean;
		/**
		 * Le tableau suit l'onglet à choix (il a la colonne de catégorie, rang
		 * choiceCol) : ses lignes masquées sont celles des autres valeurs, et
		 * on peut y saisir (la ligne reçoit la valeur choisie).
		 */
		follows?: boolean;
		choiceCol?: number;
		columns: Column[];
	}

	export interface SheetData {
		/** Feuille affichée. */
		sheet?: number;
		/** Onglet à choix de la feuille (colonne de catégorie masquée sur une valeur). */
		choice?: TabChoice | null;
		tables: SheetTable[];
		/** value : résultat d'une formule (sinon raw est affiché). */
		cells: { row: number; col: number; raw: string; value?: string; error?: boolean; spill?: string[][] }[];
		/** Largeurs choisies par l'utilisateur (sinon : selon le type). */
		widths?: { col: number; width: number }[];
		/** Hauteurs choisies par l'utilisateur (sinon ROW_H). */
		heights?: { row: number; height: number }[];
		/** Cellules nommées de la feuille. */
		names?: { name: string; row: number; col: number }[];
		/** Mise en forme des cellules hors tableaux (JSON). */
		styles?: { row: number; col: number; style: string }[];
		/** Mise en forme des lignes et colonnes entières de la feuille (JSON). */
		rowStyles?: { index: number; style: string }[];
		colStyles?: { index: number; style: string }[];
		/** Mise en forme conditionnelle des lignes et colonnes entières (JSON). */
		rowRules?: { index: number; rules: string }[];
		colRules?: { index: number; rules: string }[];
		/** Règles « formule » vérifiées par les cellules libres (rule : identifiant de la règle). */
		ruleMarks?: { row: number; col: number; rule: string }[] | null;
		/** Tableaux croisés dynamiques de la feuille, calculés. */
		pivots?: SheetPivot[];
		/** Graphiques de la feuille, avec leurs données. */
		charts?: SheetChart[];
		/** Boutons d'action de la feuille. */
		buttons?: SheetButton[] | null;
		/** Zones « plage » (tableaux convertis) : y écrire ne refait pas de tableau. */
		ranges?: { r0: number; c0: number; r1: number; c1: number }[];
	}

	/** Cible d'une saisie, pour la source. */
	export interface Target {
		table?: SheetTable;
		index?: number; // ligne de données
		column?: number; // colonne du tableau
		rowId?: number; // clé technique de la ligne
	}

	export type LinesOp = 'insertRows' | 'deleteRows' | 'insertCols' | 'deleteCols';

	export interface SheetSource {
		window(table: SheetTable, first: number, count: number): Promise<Fetched>;
		/** Relit la mise en page de la feuille (tableau rechargé après une lecture refusée). */
		reload?(): Promise<unknown>;
		/** Totaux de la barre d'état calculés en SQL : lignes first…first+count-1, colonnes cols. */
		rangeStats?(table: SheetTable, first: number, count: number, cols: number[]): Promise<{ count: number; nums: number; sum: number }>;
		/**
		 * Enregistre une saisie. Renvoie un message pour la barre d'état, ou la
		 * valeur stockée quand la source ne relit pas sa mise en page. key :
		 * réponse à la question de la clé (refus « clé à choisir »).
		 */
		setCell(row: number, col: number, raw: string, target: Target, key?: KeyChoice): Promise<string | void | { value: unknown }>;
		undo?(): Promise<unknown>;
		redo?(): Promise<unknown>;
		/** Colle un bloc (une seule étape d'annulation). */
		paste?(row: number, col: number, block: string[][]): Promise<string | void>;
		/** Lignes ou colonnes entières de la feuille : insertion (avant a, b lignes) ou suppression (a à b). */
		sheetLines?(op: LinesOp, a: number, b: number): Promise<unknown>;
		/** Mise en forme conditionnelle de lignes (rows) ou colonnes entières i0 à i1. */
		setLineRules?(rows: boolean, i0: number, i1: number, rules: Rule[]): Promise<unknown>;
		/** Changements de mise en forme conditionnelle de la feuille, en une étape d'annulation. */
		setSheetRules?(ch: RuleChanges): Promise<unknown>;
		/** Statistiques de colonnes d'un tableau pour la mise en forme conditionnelle (calculées en SQL). */
		ruleStats?(table: SheetTable, cols: number[], req: StatsReq): Promise<RuleStats>;
		/** Déplace les cellules libres d'une plage (glisser la bordure de la sélection). */
		moveCells?(r0: number, c0: number, r1: number, c1: number, dr: number, dc: number): Promise<unknown>;
		/** Vide entièrement les cellules libres d'une plage : valeurs, mise en forme, fusions, notes, validations. */
		clearCells?(r0: number, c0: number, r1: number, c1: number): Promise<unknown>;
		/**
		 * Efface une zone en une seule étape : tableaux entièrement compris
		 * supprimés, cellules de tableau vidées, cellules libres vidées (all :
		 * mise en forme comprise, comme Supprimer).
		 */
		clearRange?(r0: number, c0: number, r1: number, c1: number, all: boolean): Promise<unknown>;
		/**
		 * Copie ou déplace (move) la zone src de la feuille from vers la zone dst
		 * de la feuille affichée, comme Excel (copier-coller, couper-coller,
		 * glisser la bordure) : tableaux entiers avec leurs types, formules et
		 * clé, cellules libres avec formules et mise en forme, cellules de
		 * tableau en valeur. values : le bloc affiché de la source.
		 */
		transfer?(from: number, src: ClipRect, dst: ClipRect, move: boolean, values: string[][]): Promise<string | void>;
		/**
		 * Classeur collaboratif : saisie commencée (on) ou terminée dans la
		 * cellule (row, col) ; une modification reçue d'un collègue pour elle
		 * attend la validation.
		 */
		editing?(row: number, col: number, on: boolean): Promise<unknown>;
		/** Clé de jointure des colonnes saisies d'un tableau chargé (column : colonne chargée, « # » : rang des lignes). */
		setHandKey?(table: SheetTable, column: string): Promise<unknown>;
		/** Oublie les valeurs saisies mises de côté (lignes disparues du résultat). */
		forgetHandKept?(table: SheetTable): Promise<unknown>;
		insertRow?(table: SheetTable, n: number): Promise<unknown>;
		deleteRow?(table: SheetTable, n: number): Promise<unknown>;
		insertColumn?(table: SheetTable, i: number): Promise<unknown>;
		deleteColumn?(table: SheetTable, i: number): Promise<unknown>;
		/** Suppr sur les colonnes entières i à j, en une étape : vides au milieu, le tableau se coupe en deux. */
		suppressColumns?(table: SheetTable, i: number, j: number): Promise<unknown>;
		/** Supprime des colonnes de plusieurs tableaux (colonnes de la feuille), en une étape. */
		deleteColumnSpans?(spans: { table: string; from: number; to: number }[]): Promise<unknown>;
		/** Nombre de valeurs qui ne se convertiraient pas au type to. */
		convertPreview?(table: SheetTable, i: number, to: ColumnType): Promise<number>;
		setType?(table: SheetTable, i: number, to: ColumnType): Promise<unknown>;
		/** Type majoritaire d'une colonne en texte au type inféré. */
		typeMix?(table: SheetTable, i: number): Promise<TypeMix>;
		setColumnWidth?(col: number, width: number): Promise<unknown>;
		/** Hauteur d'une ligne de la feuille (0 : par défaut). */
		setRowHeight?(row: number, height: number): Promise<unknown>;
		/** Nomme une cellule libre (vide : retire le nom). */
		setName?(row: number, col: number, name: string): Promise<unknown>;
		sortTable?(table: SheetTable, i: number, desc: boolean): Promise<unknown>;
		/** Tri personnalisé selon la colonne i (ordre enregistré pour la colonne, une étape d'annulation). */
		sortTableCustom?(table: SheetTable, i: number, order: CustomOrder): Promise<unknown>;
		/** Ordre personnalisé enregistré de la colonne i (null : aucun). */
		columnOrder?(table: SheetTable, i: number): Promise<CustomOrder | null>;
		/** Listes personnalisées : prédéfinies (Jours, Mois…) puis celles du classeur. */
		customLists?(): Promise<CustomList[]>;
		/** Remplace les listes personnalisées du classeur. */
		setCustomLists?(lists: CustomList[]): Promise<unknown>;
		/** Met en forme le rectangle sélectionné (une valeur vide retire la propriété). */
		applyStyle?(r0: number, c0: number, r1: number, c1: number, patch: Record<string, unknown>): Promise<unknown>;
		/** Met en forme des lignes (rows) ou des colonnes entières de la feuille, comme Excel. */
		applyLineStyle?(rows: boolean, i0: number, i1: number, patch: Record<string, unknown>): Promise<unknown>;
		/** Note d'une cellule (texte vide : la retire). */
		setNote?(row: number, col: number, text: string): Promise<unknown>;
		/** Épingle la note (toujours affichée) ou la remet en infobulle. */
		pinNote?(row: number, col: number, pinned: boolean): Promise<unknown>;
		/** Fusionne et centre des cellules libres ; renvoie le nombre de valeurs effacées. */
		mergeCells?(r0: number, c0: number, r1: number, c1: number): Promise<number>;
		unmergeCells?(r0: number, c0: number, r1: number, c1: number): Promise<unknown>;
		/** Masque (ou affiche) des lignes (rows) ou des colonnes i0 à i1. */
		setLinesHidden?(rows: boolean, i0: number, i1: number, hide: boolean): Promise<unknown>;
		/** Fige les volets (0, 0 : les libère). */
		setFreeze?(rows: number, cols: number): Promise<unknown>;
		/** Cherche un texte dans la zone (la sélection), ou dans toute la feuille (null). */
		find?(text: string, zone: Zone | null): Promise<{ row: number; col: number }[]>;
		/** Remplace dans la zone (null : partout) ; renvoie le nombre de cellules modifiées. */
		replace?(text: string, by: string, zone: Zone | null): Promise<number>;
		/** Remplace les règles de mise en forme conditionnelle d'une colonne. */
		/** Actualise la requête d'import qui charge le tableau. */
		refreshTable?(table: SheetTable): Promise<unknown>;
		setRules?(table: SheetTable, i: number, rules: Rule[]): Promise<unknown>;
		/**
		 * Liste d'une validation tirée d'une plage (Tableau[Colonne],
		 * Cell[A1:A10], Feuille2!Cell[A1:A10], cellule nommée) pour la feuille
		 * affichée : valeurs distinctes dans l'ordre de la source, ou l'erreur.
		 */
		validationList?(src: string): Promise<DvList>;
		/** Valeurs distinctes d'une colonne (filtre). */
		filterValues?(table: SheetTable, i: number): Promise<{ text: string; count: number }[]>;
		/** Filtre d'une colonne (null : retire le filtre de la colonne). */
		setFilter?(table: SheetTable, i: number, f: ColumnFilter | null): Promise<unknown>;
		clearFilters?(table: SheetTable): Promise<unknown>;
		renameTable?(table: SheetTable, name: string): Promise<unknown>;
		/** Vide si la colonne peut devenir la clé du tableau, sinon la raison. */
		keyCheck?(table: SheetTable, i: number): Promise<string>;
		setKey?(table: SheetTable, i: number): Promise<unknown>;
		clearKey?(table: SheetTable): Promise<unknown>;
		/** Ne plus proposer la première colonne du tableau comme clé. */
		dismissKeyHint?(table: SheetTable): Promise<unknown>;
		/** Renouvelle AUJOURDHUI, MAINTENANT et ALEA (F9). */
		recalc?(): Promise<unknown>;
		/** Antécédents et dépendants d'une cellule (audit de formules). */
		dependencies?(row: number, col: number): Promise<{ subject: string; precedents: DepTarget[]; dependents: DepTarget[] }>;
		/** La 1re ligne de données devient les en-têtes. */
		promoteHeader?(table: SheetTable): Promise<unknown>;
		/** Sortir l'en-tête du tableau : c'est un titre, la 1re ligne devient l'en-tête. */
		headerToTitle?(table: SheetTable): Promise<unknown>;
		/** En-têtes sur deux lignes (ou une) : la ligne au-dessus de l'en-tête donne les groupes. */
		setTwoLevel?(table: SheetTable, on: boolean): Promise<unknown>;
		/** Sortir la dernière ligne du tableau : elle reste dessous comme note ou total. */
		rowToNote?(table: SheetTable, n: number): Promise<unknown>;
		/** Ajouter au tableau voisin une cellule gardée hors du tableau (note, total). */
		joinTable?(row: number, col: number): Promise<unknown>;
		createPivot?(cfg: PivotConfig): Promise<unknown>;
		/** Valeurs distinctes d'une colonne d'un tableau (filtre d'un TCD). */
		pivotFieldValues?(table: string, field: string): Promise<{ text: string; count: number }[]>;
		createChart?(row: number, col: number, cfg: ChartConfig): Promise<unknown>;
		setChart?(id: number, cfg: ChartConfig): Promise<unknown>;
		moveChart?(id: number, row: number, col: number): Promise<unknown>;
		/** Ancrage libre : cellule, décalage et taille en pixels. */
		placeChart?(id: number, row: number, col: number, offX: number, offY: number, width: number, height: number): Promise<unknown>;
		deleteChart?(id: number): Promise<unknown>;
		/** Boutons d'action : bibliothèque de scripts, clic, création, modification. */
		scripts?(): Promise<{ name: string; source: string }[]>;
		runButton?(id: number): Promise<unknown>;
		/** Évènement d'un contrôle (clic, nouvelle valeur) : action et écouteurs ; renvoie le message à afficher. */
		fireControl?(id: number, event: string, value: string, old: string): Promise<unknown>;
		/** Fenêtre de choix d'un fichier (sélecteur de fichier) ; '' si annulé. */
		pickFile?(title: string): Promise<string>;
		createButton?(row: number, col: number, cfg: ButtonConfig): Promise<unknown>;
		setButton?(id: number, cfg: ButtonConfig): Promise<unknown>;
		placeButton?(id: number, row: number, col: number, offX: number, offY: number, width: number, height: number): Promise<unknown>;
		deleteButton?(id: number): Promise<unknown>;
		setPivot?(id: number, cfg: PivotConfig): Promise<unknown>;
		deletePivot?(id: number): Promise<unknown>;
		/** Tableau croisé entier coupé puis collé : déplacé en (row, col) de la feuille affichée. */
		movePivot?(id: number, row: number, col: number): Promise<unknown>;
		/** Tableau croisé entier copié puis collé : une copie en (row, col). */
		copyPivot?(id: number, row: number, col: number): Promise<unknown>;
		/** Pose un tableau croisé de cette configuration (collé depuis un document). */
		placePivot?(row: number, col: number, cfg: PivotConfig): Promise<unknown>;
		/** Supprime les lignes en double ; renvoie leur nombre. */
		removeDuplicates?(table: SheetTable, cols: number[]): Promise<number>;
		/** Nombre de lignes que removeDuplicates retirerait. */
		duplicatesPreview?(table: SheetTable, cols: number[]): Promise<number>;
		/** Données › Convertir : aperçu et conversion du texte des lignes r0 à r1 de la colonne col. */
		textColumnsPreview?(r0: number, r1: number, col: number, o: TextColumnsOptions): Promise<TextColumnsPreview>;
		textToColumns?(r0: number, r1: number, col: number, o: TextColumnsOptions): Promise<number>;
		deleteTables?(tables: SheetTable[]): Promise<unknown>;
		/** Convertir en plage : le tableau devient des cellules libres, au même endroit. */
		convertToRange?(table: SheetTable): Promise<unknown>;
		/** « Suivre l'année » (onglet à choix, §6.5) : le tableau suit le choix de l'onglet, ou plus. */
		setFollow?(table: SheetTable, follow: boolean): Promise<unknown>;
		/**
		 * « Filtrer l'onglet par cette colonne » (tableau) ou « par ce champ »
		 * (tableau croisé : table est son tableau source) : la feuille affichée
		 * devient un onglet à choix par cette colonne ; clearTabChoice le retire.
		 */
		setTabChoice?(table: string, column: string): Promise<unknown>;
		clearTabChoice?(column: string): Promise<unknown>;
		/** Colonnes que « Choisir par… » propose pour la feuille affichée (tabchoice.ts). */
		choiceCandidates?(): Promise<ChoiceCandidate[]>;
		/** Mettre sous forme de tableau la plage (tableau converti) qui contient la cellule. */
		formatAsTable?(row: number, col: number): Promise<unknown>;
		/** Déplace un tableau, en-tête en (row, col) ; côte à côte d'un autre : fusion. */
		moveTable?(table: SheetTable, row: number, col: number): Promise<unknown>;
		/** Sort les colonnes i0..i1 vers un nouveau tableau en (row, col). */
		extractColumns?(table: SheetTable, i0: number, i1: number, row: number, col: number): Promise<unknown>;
	}

	export interface SheetStatus {
		cell: string;
		message: string;
		fetch: string;
		draw: string;
		fps: string;
		edit: string;
		blocks: number;
		/** Agrégats de la sélection, comme la barre d'état d'Excel. */
		stats: string;
		/** Sélection (à partir de 0, r0 ≤ r1, c0 ≤ c1). */
		sel: { r0: number; c0: number; r1: number; c1: number };
		/** Mise en forme de la cellule active (police et taille du ruban, boutons enfoncés). */
		style?: CellStyle;
		/** La cellule active est une fusion. */
		merged?: boolean;
		/** La cellule active a une note (Révision › Modifier la note). */
		note?: boolean;
		/** Tableau de la cellule active : filtré, et sa colonne n'affiche que les erreurs. */
		filters?: boolean;
		errorsOnly?: boolean;
		/** Tableau publié en lecture seule sous la cellule active (classeur partagé) : le motif. */
		readOnly?: string;
	}

	interface Props {
		data: SheetData;
		source: SheetSource;
		/** Fonctions et tableaux du classeur, pour l'aide à la saisie des formules. */
		catalog?: Catalog;
		/** Tableaux et tableaux croisés de toutes les feuilles (sources de graphiques et de TCD). */
		sources?: { name: string; columns: string[]; types: string[]; kind?: string }[];
		onstatus?: (s: SheetStatus) => void;
		/** Lance la mesure dès l'ouverture et transmet le résultat (mode LATTIS_BENCH). */
		autoBench?: (result: string) => void;
		/** Quadrillage de la feuille (masqué : seules les lignes des tableaux restent). */
		gridlines?: boolean;
		/** Lettres de colonnes et numéros de lignes toujours affichés (sinon : pendant la saisie d'une formule). */
		headings?: boolean;
		/** Défilement horizontal (largeur du contenu, largeur visible, position) : la barre est dans la ligne des onglets, comme Excel. */
		onhscroll?: (s: { content: number; view: number; left: number }) => void;
		/** Feuille protégée : message des commandes refusées (vide : modifiable). Les
		 * cellules ne prennent plus le focus (ni sélection ni cellule active
		 * affichée), sans message ; graphiques et contrôles ne se sélectionnent
		 * plus, les boutons s'exécutent toujours. */
		locked?: string;
		/** Volets figés : lignes et colonnes qui restent affichées pendant le défilement. */
		freeze?: { rows: number; cols: number };
		/** Zoom de la feuille (1 : 100 %), voir zoom.ts. */
		zoom?: number;
		/** Ctrl+molette : zoom demandé. */
		onzoom?: (z: number) => void;
		/** Cadre de sélection animé (Affichage › Animations). */
		animate?: boolean;
		/** Nom de la feuille affichée (références pointées depuis une formule d'une autre feuille). */
		sheetName?: string;
		/** Retour à une feuille (formule pointée sur une autre feuille, validée ou abandonnée). */
		onsheet?: (sheet: number) => void;
		/** Point de carte cliqué : aller à la ligne id du tableau (éventuellement sur une autre feuille). */
		onrowlink?: (table: string, id: number) => void;
		/** Partager le tableau au Data Lake Lattis (clic droit sur un tableau). */
		onsharedata?: (table: string) => void;
		/** Classeur partagé en direct : clic droit sur un tableau qui n'est que sur ce poste › « Ajouter au classeur partagé ». */
		onaddshared?: (table: string) => void;
		/** Tableaux publiés seuls comme source du Data Lake (sans le classeur). */
		sourceTables?: readonly string[];
		/** Clic droit › « Mettre à jour la source du Data Lake » d'un de ces tableaux. */
		onpushsource?: (table: string) => void;
		/** Clic droit › « Demander à l'assistant… » (absent : entrée masquée). */
		onask?: () => void;
		/** Sauts de page de l'impression (traits pointillés) et zone imprimée ; absent : rien. */
		pageBreaks?: PageBreaksView;
	}

	let { data, source, catalog, sources = [], onstatus, autoBench, gridlines = true, headings = false, onhscroll, locked = '', freeze, zoom = 1, onzoom, animate = true, sheetName = '', onsheet, onrowlink, onask, onsharedata, onaddshared, sourceTables = [], onpushsource, pageBreaks }: Props = $props();

	// Feuille sans fin pratique (choix de Louis) : au-delà des limites d'Excel,
	// bornée par la nomenclature Cell[ZZZZ999999999] (4 lettres, 9 chiffres).
	// L'export .xlsx coupe aux limites d'Excel, avec un message.
	export const SHEET_ROWS = 999_999_999;
	export const SHEET_COLS = 475_254;
	const ROW_H = 22;
	const DEF_W = 88;
	const TOP_H = 22;
	// Gouttière de gauche (choix de Louis) : la largeur du plus long numéro
	// que la feuille peut afficher (pas seulement l'écran : elle ne bouge pas
	// au défilement), 2 chiffres au moins. Numéros : le nombre de lignes de
	// chaque tableau ; clés : la plus longue déjà affichée (elles ne sont pas
	// toutes chargées) ; en-têtes A1 : la dernière ligne de la zone utilisée.
	const MIN_MARGIN = 28;
	const GUTTER_PAD = 14;
	const GUTTER_MAX = 160; // clés longues : coupées au-delà
	const keyWidths = new Map<string, number>(); // tableau → clé la plus large vue
	let extentRows = 1;
	let keyGrew = false; // une clé plus large a été dessinée pendant cette image

	function gutterWidth(): number {
		const m = (s: string) => ctx.measureText(s).width;
		let need = m('99');
		if (a1) need = Math.max(need, m(String(extentRows)));
		else
			for (const e of entries) {
				need = Math.max(need, m(String(e.t.rows)));
				if (e.t.key !== undefined) need = Math.max(need, keyWidths.get(e.t.name) ?? 0);
			}
		return Math.min(GUTTER_MAX, Math.max(MIN_MARGIN, Math.ceil(need) + GUTTER_PAD));
	}
	const FONT = '13px "Segoe UI", system-ui, -apple-system, sans-serif';
	const FONT_BOLD = '600 13px "Segoe UI", system-ui, -apple-system, sans-serif';
	const FONT_SMALL = '11px "Segoe UI", system-ui, -apple-system, sans-serif';
	const FONT_TINY = '9px "Segoe UI", system-ui, -apple-system, sans-serif';

	const C = {
		bg: '#ffffff',
		// Colonne calculée dont la formule est en cours d'édition.
		calc: 'rgba(33, 115, 70, 0.09)',
		grid: '#e2e4e7',
		strip: '#f5f6f7',
		stripLine: '#d4d7db',
		// Sauts de page (Affichage › Sauts de page).
		pageBreak: '#5f6368',
		stripText: '#5f6368',
		head: '#e3efe7',
		// Tableaux chargés par une requête (source externe) : bleu clair, même contraste que le vert.
		linkHead: '#e3ecf7',
		linkHeadText: '#14325a',
		linkBorder: '#8fa9cc',
		// Pastille d'un tableau qui suit l'onglet à choix (couleur d'accent).
		followBadge: '#217346',
		headText: '#14532d',
		icon: '#6b8f78',
		border: '#8fbc9f',
		gray: '#f1f2f4',
		text: '#1f2328',
		placeholder: '#eef0f3',
		select: 'rgba(33, 115, 70, 0.10)',
		active: '#217346',
		activeStrip: '#d3e7dc',
		error: '#c0392b',
		spill: '#5b9bd5',
		found: 'rgba(250, 204, 21, 0.30)',
		// Cellule trouvée (Ctrl+F) : sa ligne et sa colonne entières, teintées ; plus marqué dans les gouttières.
		findCross: 'rgba(250, 204, 21, 0.16)',
		findGutter: 'rgba(250, 204, 21, 0.55)',
		frozen: '#9aa0a6',
		note: '#c0392b',
		// Classeur collaboratif : modification en attente du serveur, conflit.
		pending: '#7f93b2',
		conflict: '#d97706'
	};

	// --- Tableaux posés sur la feuille ------------------------------------

	interface Entry {
		t: SheetTable;
		cache: RowCache;
		last: number; // dernière ligne de la feuille occupée
		lastCol: number;
	}

	let entries: Entry[] = [];
	let cells = new Map<string, string>();
	let names = new Map<string, string>(); // position → nom
	// Mise en forme, en couches (voir style.ts) : colonnes et lignes entières
	// de la feuille, colonnes et en-têtes des tableaux, cellules.
	let freeStyles = new Map<string, StyleLayer>(); // position → style (hors tableaux : cellules libres, TCD)
	let sheetRowStyles = new Map<number, StyleLayer>();
	let sheetColStyles = new Map<number, StyleLayer>();
	// Mise en forme conditionnelle (voir rules.ts, cfmodel.ts) : règles des
	// lignes et colonnes entières, des cellules libres (dans leur style) et
	// des colonnes de tableau (colRules, plus bas).
	let sheetRowRules = new Map<number, Rule[]>();
	let sheetColRules = new Map<number, Rule[]>();
	let cellRules = new Map<string, Rule[]>();
	// Règles « formule » vraies des cellules libres : « ligne,colonne,règle ».
	let freeMarks = new Set<string>();
	// Statistiques des règles (10 premiers, moyenne, doublons, barres…) par
	// règle et demande : recalculées après chaque modification, les
	// précédentes servent en attendant (pas de clignotement).
	let cfStats = new Map<string, Stats | null>();
	let cfStale = new Map<string, Stats | null>();
	let cfGen = 0;
	let cfToday = todaySerial();

	/** Statistiques en cache, ou calculées (une promesse : demandées au classeur, redessin à l'arrivée). */
	function cfStat(k: string, compute: () => Stats | undefined | Promise<Stats | undefined>): Stats | undefined {
		if (cfStats.has(k)) return cfStats.get(k) ?? cfStale.get(k) ?? undefined;
		const got = compute();
		if (!(got instanceof Promise)) {
			cfStats.set(k, got ?? null);
			return got;
		}
		const gen = cfGen;
		cfStats.set(k, null);
		got.then((s) => {
			if (gen !== cfGen) return;
			cfStats.set(k, s ?? null);
			schedule();
		}).catch(() => {});
		return cfStale.get(k) ?? undefined;
	}

	/** Valeur d'une cellule libre pour les règles (undefined : vide). */
	function freeValue(r: number, c: number): { v: unknown; text: string; error: boolean } | undefined {
		const k = key(r, c);
		const f = shown.get(k);
		const raw = cells.get(k);
		if (f === undefined && raw === undefined) {
			// Cellule d'un tableau croisé : sa valeur calculée (mise en forme
			// conditionnelle posée sur le TCD).
			const p = pivots.length ? pivotAt(r, c) : undefined;
			const text = p && r >= p.row ? (p.cells[r - p.row]?.[c - p.col] ?? '') : '';
			return text ? { v: ruleValue(text), text, error: false } : undefined;
		}
		const text = f ? f.text : freeDisplay(raw!, undefined).text;
		return { v: ruleValue(text), text, error: !!f?.error };
	}

	/**
	 * Statistiques de la plage d'une règle (clé gk, voir groupKey) : ses
	 * cellules libres, celles de ses lignes et colonnes entières, et ses
	 * colonnes de tableau (ou celles sous ses colonnes entières), en SQL.
	 */
	function rangeStats(gk: string, q: StatsReq): Stats | undefined {
		return cfStat(gk + '|' + reqKey(q), () => {
			const rows = new Set<number>();
			const cols = new Set<number>();
			for (const [i, rs] of sheetRowRules) if (rs.some((x) => groupKey(x, { k: 'row', i }) === gk)) rows.add(i);
			for (const [i, rs] of sheetColRules) if (rs.some((x) => groupKey(x, { k: 'col', i }) === gk)) cols.add(i);
			const values: unknown[] = [];
			const seen = new Set<string>();
			const add = (r: number, c: number) => {
				const k = key(r, c);
				if (seen.has(k) || tableAt(r, c)) return;
				seen.add(k);
				const fv = freeValue(r, c);
				if (fv && !fv.error) values.push(fv.v);
			};
			for (const [k, rs] of cellRules)
				if (rs.some((x) => groupKey(x, { k: 'cell', r: 0, c: 0 }) === gk)) {
					const [r, c] = k.split(',').map(Number);
					add(r, c);
				}
			if (rows.size || cols.size)
				for (const k of cells.keys()) {
					const [r, c] = k.split(',').map(Number);
					if (rows.has(r) || cols.has(c)) add(r, c);
				}
			const free = values.length ? computeStats(values, q) : undefined;
			const parts: Promise<Stats>[] = [];
			for (const e of entries) {
				const tr = colRules.get(e.t.id);
				const ci = e.t.columns.flatMap((_, i) => (tr?.[i]?.some((x) => groupKey(x, { k: 'tbl', t: e.t.id, c: i }) === gk) || cols.has(e.t.col + i) ? [i] : []));
				if (ci.length && source.ruleStats && e.t.rows > 0) parts.push(source.ruleStats(e.t, ci, q).then((s) => prepareStats(s, q)));
			}
			if (!parts.length) return free;
			return Promise.all(parts).then((list) => list.reduce<Stats | undefined>((a, b) => mergeStats(a, b), free));
		});
	}

	/** Contexte d'évaluation des règles d'un emplacement. */
	function cfEnv(loc: Loc, mark: (r: Rule, k: number) => boolean): RuleEnv {
		return {
			stats: (r) => {
				const q = statsReq(r);
				return q ? rangeStats(groupKey(r, loc), q) : undefined;
			},
			mark,
			today: cfToday
		};
	}

	const noMark = () => false;
	/** Règle « formule » d'une cellule libre vérifiée (calculée par le classeur). */
	const freeMark = (row: number, col: number) => (r: Rule) => freeMarks.has(`${row},${col},${r.id ?? r.value}`);

	/**
	 * Mise en forme conditionnelle d'une cellule : ses propres règles (colonne
	 * de tableau ou cellule libre), puis celles de sa ligne et de sa colonne
	 * entières (la ligne l'emporte, comme pour la mise en forme).
	 */
	function cfAt(
		r: number,
		c: number,
		val: { v: unknown; text: string; error: boolean },
		own?: { rules: Rule[] | undefined; loc: Loc; mark: (r: Rule, k: number) => boolean }
	): CfOut | undefined {
		const rr = sheetRowRules.get(r);
		const cr = sheetColRules.get(c);
		if (!own?.rules && !rr && !cr) return undefined;
		const out: CfOut = {};
		if (own?.rules) applyRules(own.rules, val, cfEnv(own.loc, own.mark), out);
		const fm = rr || cr ? freeMark(r, c) : noMark;
		if (rr) applyRules(rr, val, cfEnv({ k: 'row', i: r }, fm), out);
		if (cr) applyRules(cr, val, cfEnv({ k: 'col', i: c }, fm), out);
		return out;
	}

	/** Mise en forme conditionnelle d'une cellule libre (vide comprise). */
	function freeCf(r: number, c: number): CfOut | undefined {
		const k = key(r, c);
		const own = cellRules.get(k);
		if (!own && !sheetRowRules.size && !sheetColRules.size) return undefined;
		return cfAt(r, c, freeValue(r, c) ?? { v: null, text: '', error: false }, own ? { rules: own, loc: { k: 'cell', r, c }, mark: freeMark(r, c) } : undefined);
	}

	/** Barre de données dans la cellule (x, y, w, h). */
	function drawBar(b: NonNullable<CfOut['bar']>, x: number, y: number, w: number, h: number, k: string, r: number, c: number) {
		const pad = 2;
		const span = Math.max(0, w - 2 * pad);
		const shown = barShown(k, b, r, c);
		const x0 = x + pad + shown.a * span;
		const x1 = x + pad + shown.b * span;
		const by = y + 2;
		const bh = Math.max(2, h - 4);
		if (x1 - x0 >= 0.5) {
			if (b.gradient) {
				const g = b.rev ? ctx.createLinearGradient(x1, 0, x0, 0) : ctx.createLinearGradient(x0, 0, x1, 0);
				g.addColorStop(0, b.color);
				g.addColorStop(1, '#ffffff');
				ctx.fillStyle = g;
			} else ctx.fillStyle = b.color;
			ctx.fillRect(x0, by, x1 - x0, bh);
			if (b.border || b.gradient) {
				ctx.strokeStyle = b.color;
				ctx.lineWidth = 1;
				ctx.strokeRect(Math.round(x0) + 0.5, by + 0.5, Math.max(0, Math.round(x1 - x0) - 1), bh - 1);
			}
		}
		if (b.axis !== undefined) {
			const ax = Math.round(x + pad + b.axis * span) + 0.5;
			ctx.save();
			ctx.strokeStyle = '#000';
			ctx.setLineDash([2, 2]);
			ctx.beginPath();
			ctx.moveTo(ax, y + 1);
			ctx.lineTo(ax, y + h - 1);
			ctx.stroke();
			ctx.restore();
		}
	}

	/** Icône d'un jeu d'icônes à gauche de la cellule ; renvoie la place prise. */
	function drawIconAt(icon: NonNullable<CfOut['icon']>, x: number, y: number, h: number): number {
		const spec = ICON_SETS[icon.set]?.icons[icon.index];
		if (!spec) return 0;
		const s = Math.min(16, h - 4);
		drawCfIcon(ctx, spec, x + 3, y + (h - s) / 2, s);
		return s + 4;
	}

	/** Valeur d'un texte affiché pour les règles : nombre si c'en est un. */
	function ruleValue(text: string): unknown {
		if (!isNumeric(text)) return text;
		const n = Number(text.replace(/[\s\u00a0\u202f%]/g, '').replace(',', '.'));
		return Number.isFinite(n) ? n : text;
	}
	// Styles de colonne et d'en-tête, par tableau puis colonne.
	let colStyles = new Map<number, (StyleLayer | undefined)[]>();
	let headStyles = new Map<number, (StyleLayer | undefined)[]>();
	let colRules = new Map<number, (Rule[] | undefined)[]>();
	let shown = new Map<string, { text: string; error: boolean }>(); // formules des cellules libres
	// Tableaux dynamiques déversés depuis une cellule libre (FILTRE, TRIER…).
	let spills: { row: number; col: number; rows: string[][] }[] = [];
	let pivots = $state.raw<SheetPivot[]>([]);
	let charts = $state.raw<SheetChart[]>([]);
	let buttons = $state.raw<SheetButton[]>([]);
	let buttonsLayer = $state<ReturnType<typeof SheetButtons>>();
	// Décalage de la vue, pour placer les graphiques au-dessus de la grille.
	let viewPos = $state({ x: 0, y: 0 });
	let colAxis = $state.raw(new Axis(SHEET_COLS, DEF_W));
	const key = (r: number, c: number) => `${r},${c}`;

	// Dernière erreur de lecture d'un tableau signalée (une fois par mise en page).
	let fetchError = '';
	// Tableaux rechargés après une lecture refusée (une fois par période).
	const recovery = new ReadRecovery();

	/**
	 * Lecture d'un tableau refusée (le bloc n'est pas redemandé, cache.ts) :
	 * la mise en page est relue et le tableau rechargé, une fois, avec un
	 * message en français ; le détail technique va dans la console (le moteur
	 * l'écrit aussi dans son journal).
	 */
	function readFailed(e: Entry, err: string) {
		console.warn(`[grille] lecture du tableau « ${e.t.name} » (${e.t.rows} lignes affichées) refusée : ${err}`);
		if (recovery.take(e.t.id, Date.now())) {
			fetchError = err;
			say(readFailNotice(e.t.name));
			if (source.reload) void source.reload().catch(() => {});
			else e.cache.invalidate(e.t.rows);
			return;
		}
		if (err !== fetchError) {
			fetchError = err;
			say(readFailNotice(e.t.name));
		}
	}

	function makeEntry(t: SheetTable): Entry {
		const e: Entry = {
			t,
			last: t.row + t.rows,
			lastCol: t.col + t.columns.length - 1,
			cache: new RowCache(
				(first, count) => source.window(e.t, first, count),
				t.rows,
				(c, v) => formatColumnValue(e.t.columns[c], v),
				(first, count) => {
					// Lecture refusée : un tableau quitté (bloc en route au
					// changement d'onglet) ne dit rien ; sinon rechargé.
					const err = e.cache.error;
					if (err && entries.includes(e)) readFailed(e, err);
					// Bloc arrivé : ses lignes se mesurent (hauteur automatique) au prochain dessin.
					if (first !== undefined && count) autoRows.forget(e.t.row + 1 + first, count);
					schedule();
					refreshFx();
				}
			)
		};
		return e;
	}

	// Feuille des tableaux affichés (entries) : leurs positions avant une mise à jour de la même feuille.
	let placedSheet: number | undefined;
	const placed = () => entries.map((e) => ({ id: e.t.id, r0: e.t.row, c0: e.t.col, r1: e.last, c1: e.lastCol }));

	/** Recalcule la géométrie quand la mise en page change (après une saisie). */
	function applyData(d: SheetData) {
		// Changement de feuille pendant la saisie d'une formule : la saisie est
		// gardée (texte, curseur, cellule et feuille d'origine), pour pointer
		// des références sur l'autre feuille (crossref.ts).
		const switching = placedSheet !== undefined && placedSheet !== (d.sheet ?? 0);
		if (switching) keepFormula(placedSheet!);
		// Listes tirées d'une plage : relues (la source a pu changer).
		dvGen++;
		dvLists.clear();
		// Changement de feuille : garder la position de la feuille quittée,
		// retrouver celle de la nouvelle (A1 si elle n'a pas été visitée).
		const back = sheetViews.switchTo(d.sheet ?? 1, { active: { ...active }, anchor: { ...anchor }, top: vs.top, left: scroller?.scrollLeft ?? 0 });
		const old = new Map(entries.map((e) => [e.t.id, e]));
		fetchError = '';
		const placedBefore = placedSheet === (d.sheet ?? 0) ? placed() : null;
		// Navigation (ouverture, autre feuille) : rien ne s'anime.
		const nav = placedSheet !== (d.sheet ?? 0);
		if (nav) recovery.reset();
		// Recherche : occurrences relues après une saisie, un Ctrl+Z, une actualisation ou sur une autre feuille.
		matchKey = '';
		if (nav && finder) {
			finder.at = null;
			finder.count = '';
			finder.info = '';
		}
		const oldCharts = new Set(charts.map((c) => c.id));
		entries =d.tables.map((t) => {
			const e = old.get(t.id);
			if (!e) return makeEntry(t);
			// Colonnes existantes déplacées (une colonne ajoutée devant, par
			// exemple) : l'ancien contenu ne s'affiche pas en attendant.
			const kept = e.t.columns.every((c, i) => t.columns[i]?.name === c.name);
			e.t = t;
			e.last = t.row + t.rows;
			e.lastCol = t.col + t.columns.length - 1;
			e.cache.invalidate(t.rows, kept);
			return e;
		});
		cells = new Map(d.cells.map((c) => [key(c.row, c.col), c.raw]));
		mixes.clear();
		tipCell = '';
		// Cadre de copie : il suit le tableau copié poussé par un collage, ou
		// disparaît s'il ne désigne plus ce qui a été copié.
		if (clipRange && clipRange.sheet === (d.sheet ?? 0) && placedBefore) clipRange = followClip(clipRange, placedBefore, placed());
		placedSheet = d.sheet ?? 0;
		pivots = (d.pivots ?? []).map((p) => {
			const f = p.error ? 0 : (p.config.filters?.length ?? 0);
			return { ...p, anchor: p.row, row: p.row + (f ? f + 1 : 0) };
		});
		if (openPivotOnLoad && pivots.length) {
			openPivotOnLoad = false;
			const p = pivots[pivots.length - 1];
			panel = { kind: 'pivot', id: p.id };
			active = { row: p.row, col: p.col };
			anchor = { ...active };
		}
		charts = d.charts ?? [];
		buttons = d.buttons ?? [];
		controlsTick++;
		spills = d.cells.filter((c) => c.spill?.length).map((c) => ({ row: c.row, col: c.col, rows: c.spill! }));
		shown = new Map(d.cells.filter((c) => c.raw.startsWith('=')).map((c) => [key(c.row, c.col), { text: c.value ?? '', error: !!c.error }]));
		userWidths = new Map((d.widths ?? []).map((w) => [w.col, w.width]));
		userHeights = new Map((d.heights ?? []).map((h) => [h.row, h.height]));
		names = new Map((d.names ?? []).map((n) => [key(n.row, n.col), n.name]));
		// Règles des cellules libres : dans leur style (propriété cf), mises à part.
		cellRules = new Map();
		freeStyles = new Map(
			(d.styles ?? []).map((s) => {
				const l = (parseStyle(s.style) ?? {}) as StyleLayer & { cf?: Rule[] };
				if (Array.isArray(l.cf) && l.cf.length) cellRules.set(key(s.row, s.col), l.cf.map((r) => ({ ...r, style: r.style ?? {} })));
				delete l.cf;
				return [key(s.row, s.col), l];
			})
		);
		freeMarks = new Set((d.ruleMarks ?? []).map((m) => `${m.row},${m.col},${m.rule}`));
		for (const [k, v] of cfStats) if (v) cfStale.set(k, v);
		cfStats = new Map();
		cfGen++;
		cfToday = todaySerial();
		sheetRowStyles = new Map((d.rowStyles ?? []).map((s) => [s.index, parseStyle(s.style) ?? {}]));
		sheetColStyles = new Map((d.colStyles ?? []).map((s) => [s.index, parseStyle(s.style) ?? {}]));
		merges = mergeList(freeStyles);
		boxes = [...merges, ...bandMerges(d.tables, isSplit)];
		hiddenRows = hiddenLines(sheetRowStyles);
		styleHiddenCols = hiddenLines(sheetColStyles);
		// Onglet à choix sur une valeur : la colonne de catégorie (la même
		// partout) n'est pas affichée ; elle revient avec « (Toutes) ».
		hiddenCols = new Set([...styleHiddenCols, ...choiceHiddenCols(d.choice, d.tables, d.cells)]);
		sheetRowRules = new Map((d.rowRules ?? []).map((s) => [s.index, parseRules(s.rules) ?? []]));
		sheetColRules = new Map((d.colRules ?? []).map((s) => [s.index, parseRules(s.rules) ?? []]));
		colStyles = new Map(d.tables.map((t) => [t.id, t.columns.map((c) => parseStyle(c.style))]));
		headStyles = new Map(d.tables.map((t) => [t.id, t.columns.map((c) => parseStyle(c.headerStyle))]));
		colRules = new Map(d.tables.map((t) => [t.id, t.columns.map((c) => parseRules(c.rules))]));
		rebuildAxis();
		// Données ou mise en forme changées : les lignes de tableau affichées se
		// remesurent (elles gardent leur hauteur d'ici là, sans clignoter).
		if (back) autoRows.clear();
		else autoRows.forgetAll((r) => entries.some((e) => r >= topRow(e.t) && r <= e.t.row + e.t.rows));
		autoRows.free = measureHeights();
		rebuildRows();
		if (back) {
			active = back.active;
			anchor = back.anchor;
			wholeSel = null;
		}
		// Plage source d'une validation pointée sur une autre feuille : de
		// retour sur la feuille de la cellule, la sélection d'avant revient.
		const pick = dvPick;
		if (pick?.back && (d.sheet ?? 0) === pick.sheet) finishPick(pick, pick.ref);
		updateExtent();
		if (back && scroller) {
			vs.to(back.top);
			scroller.scrollLeft = back.left;
			updateExtent();
			syncNative();
		}
		startAnims(nav, new Set(old.keys()), oldCharts, performance.now());
		refreshFx();
		if (back) refreshStatus();
		schedule();
		if (switching && hold) {
			const sheet = d.sheet ?? 0;
			queueMicrotask(() => placeHold(sheet));
		}
	}
	const sheetViews = new SheetViews();

	// Largeurs : choisies par l'utilisateur, sinon selon le type de la colonne
	// du premier tableau qui l'occupe, sinon la largeur par défaut.
	let userWidths = new Map<number, number>();
	const MAX_FIT = 400;
	let measurer: CanvasRenderingContext2D | null = null;
	/** Largeur du texte dans la police de la grille (normale ou grasse). */
	function textWidth(s: string, bold = false): number {
		if (!s) return 0;
		measurer ??= document.createElement('canvas').getContext('2d');
		if (!measurer) return 0;
		measurer.font = bold ? FONT_BOLD : FONT;
		return measurer.measureText(s).width;
	}
	/**
	 * Largeur du contenu de l'en-tête de la colonne i d'un tableau (nom en
	 * gras, badge du type, ▾, entonnoir, clé posée ou proposée), sans les marges : le nom n'est
	 * pas coupé au-delà (headIcons et le dessin de l'en-tête).
	 */
	function headerWidth(e: Entry, i: number): number {
		const col = e.t.columns[i];
		const icon = (col.filter ? '⧩ ' : '') + (typeIconName(col.type) ? ' ▾' : typeIcon(col.type) + ' ▾') + (e.t.key === i || (i === 0 && !!e.t.keyHint && e.t.key === undefined && !!source.setKey) ? '🔑 ' : '');
		measurer ??= document.createElement('canvas').getContext('2d');
		if (measurer) measurer.font = FONT_SMALL;
		const iconW = (measurer?.measureText(icon).width ?? 0) + (typeIconName(col.type) ? 13 : 0);
		return textWidth(col.name, true) + iconW + 4;
	}
	function rebuildAxis(override?: { col: number; width: number }) {
		const widths = new Map<number, number>();
		for (const e of entries) {
			e.t.columns.forEach((c, i) => {
				// Largeur selon le type, élargie pour que l'en-tête (nom, badge
				// du type et ▾) ne soit pas coupé.
				if (!widths.has(e.t.col + i)) widths.set(e.t.col + i, Math.max(defaultWidth(c.type), Math.min(MAX_FIT, Math.ceil(headerWidth(e, i)) + 14)));
			});
		}
		// Tableaux croisés : largeur d'après le texte affiché (« Somme de Commande »).
		for (const p of pivots) {
			if (p.error) continue;
			p.cells[0]?.forEach((_, j) => {
				const c = p.col + j;
				if (widths.has(c)) return;
				const w = Math.max(...p.cells.map((r, i) => textWidth(r[j] ?? '', pivotBold(p, p.row + i)) + (j === 0 ? pivotLabelOffset(pivotRowOf(p, p.row + i)) : 0)));
				widths.set(c, clamp(Math.ceil(w) + 14, DEF_W, MAX_FIT));
			});
		}
		for (const [c, w] of userWidths) widths.set(c, w);
		if (override) widths.set(override.col, override.width);
		for (const c of hiddenCols) widths.set(c, 0); // masquée : largeur nulle
		colAxis = new Axis(SHEET_COLS, DEF_W, widths);
	}

	// Format de cellule : fusions, lignes et colonnes masquées (voir sheetfmt.ts)
	// et hauteurs automatiques (renvoyer à la ligne, grande police).
	let merges: MergeRange[] = [];
	// Fusions et groupes de la bande des tableaux (boxOf).
	let boxes: MergeRange[] = [];
	let hiddenRows = new Set<number>();
	let hiddenCols = new Set<number>();
	// Colonnes masquées par l'utilisateur (Masquer) : repères et Afficher ;
	// hiddenCols y ajoute la colonne de catégorie d'un onglet à choix.
	let styleHiddenCols = new Set<number>();
	// Hauteurs automatiques (voir rowfit.ts) : cellules libres à chaque mise en
	// page, lignes de tableau à mesure qu'elles s'affichent.
	const autoRows = new AutoRows();
	const MAX_AUTO_H = 400;

	/** Hauteur nécessaire au texte d'une cellule (0 : la hauteur par défaut suffit). */
	function fitHeight(text: string, c: number, st: CellStyle | undefined): number {
		if ((!st || (!st.wr && !st.fs)) && !text.includes('\n')) return 0;
		measurer ??= document.createElement('canvas').getContext('2d');
		if (!measurer) return 0;
		measurer.font = cellFont(st);
		const m = measurer;
		return Math.min(MAX_AUTO_H, autoHeight(text, cax().size(c), st ?? {}, (s) => m.measureText(s).width, ROW_H));
	}

	/** Hauteurs automatiques des lignes, comme Excel : cellules libres qui renvoient à la ligne, sur plusieurs lignes ou en grande police. */
	function measureHeights(): Map<number, number> {
		const out = new Map<number, number>();
		const consider = (r: number, c: number, text: string, st: CellStyle | undefined) => {
			if (mergeAt(merges, r, c)) return;
			const h = fitHeight(text, c, st);
			if (h > (out.get(r) ?? 0)) out.set(r, h);
		};
		for (const [k, raw] of cells) {
			const [r, c] = k.split(',').map(Number);
			consider(r, c, shown.get(k)?.text ?? freeDisplay(raw).text, styleAt(r, c, undefined));
		}
		// Grande police posée sur des lignes entières ou des cellules vides.
		for (const [r, st] of sheetRowStyles) if (st.fs) consider(r, -1, '', resolveStyle(st));
		return out;
	}

	/**
	 * Hauteur nécessaire aux cellules de tableau d'une ligne de la feuille
	 * (0 : la hauteur par défaut suffit), undefined si la ligne n'est pas
	 * encore chargée. Seules les cellules qui peuvent passer à la ligne ou
	 * grossir sont mesurées.
	 */
	function measureTableRow(r: number): number | undefined {
		let h = 0;
		const rowSt = sheetRowStyles.get(r);
		const rowFit = !!rowSt && ('wr' in rowSt || 'fs' in rowSt);
		for (const e of entries) {
			if (r === e.t.row) {
				h = Math.max(h, measureHeader(e));
				continue;
			}
			if (e.t.twoLevel && r === e.t.row - 1) {
				h = Math.max(h, measureBand(e));
				continue;
			}
			if (r < e.t.row || r > e.t.row + e.t.rows) continue;
			const row = e.cache.row(r - e.t.row - 1);
			if (!row) return undefined;
			const cs = colStyles.get(e.t.id);
			for (let i = 0; i < e.t.columns.length; i++) {
				const c = e.t.col + i;
				const text = row.text[i] ?? '';
				if (!cax().size(c) || (!rowFit && !cs?.[i] && !sheetColStyles.has(c) && !row.sty?.has(i) && !text.includes('\n'))) continue;
				h = Math.max(h, fitHeight(text, c, styleAt(r, c, e)));
			}
		}
		return h;
	}

	/**
	 * Hauteur nécessaire à l'en-tête d'un tableau dont des noms renvoient à la
	 * ligne (0 : la hauteur par défaut suffit). Une colonne sur un niveau
	 * (bande et en-tête réunis) profite déjà de la hauteur de la bande.
	 */
	function measureHeader(e: Entry): number {
		let h = 0;
		for (let i = 0; i < e.t.columns.length; i++) {
			const c = e.t.col + i;
			const cw = cax().size(c);
			const single = singleCol(e, i) && !!rh(e.t.row - 1);
			const hs = single ? bandStyle(e, i, i) : styleAt(e.t.row, c, e);
			if (!cw || !hs?.wr) continue;
			const label = single ? e.t.columns[i].name : headerLabel(e.t.columns[i].name, e.t.twoLevel);
			let need = headNeed(label, headerTextWidth(cw, headIconW.get(`${e.t.id}:${i}`) ?? HEAD_ICON_W), hs);
			if (single) need -= rh(e.t.row - 1);
			h = Math.max(h, need);
		}
		return Math.min(MAX_AUTO_H, h > ROW_H ? h : 0);
	}

	/** Hauteur nécessaire à la bande des groupes d'un tableau dont des groupes renvoient à la ligne (0 : la hauteur par défaut suffit). */
	function measureBand(e: Entry): number {
		let h = 0;
		for (const s of groupSpans(e.t.columns.map((k) => k.name))) {
			if (!s.group) continue;
			const st = bandStyle(e, s.i0, s.i1);
			const sw = cax().offset(e.t.col + s.i1 + 1) - cax().offset(e.t.col + s.i0);
			if (sw > 0 && st?.wr) h = Math.max(h, headNeed(s.group, sw - 12, st));
		}
		return Math.min(MAX_AUTO_H, h > ROW_H ? h : 0);
	}

	/** Hauteur d'un nom d'en-tête renvoyé à la ligne dans width (police de l'en-tête). */
	function headNeed(label: string, width: number, st: CellStyle): number {
		measurer ??= document.createElement('canvas').getContext('2d');
		if (!measurer) return 0;
		const m = measurer;
		m.font = (st.i ? 'italic ' : '') + FONT_BOLD;
		return headerHeight(label, width, st, (t) => m.measureText(t).width);
	}

	/**
	 * Style d'une cellule de la bande des groupes (colonnes i0 à i1 du
	 * tableau), ou d'une colonne sur un niveau (i0 = i1, bande et en-tête
	 * réunis) : la mise en forme posée par le ruban sur ses cellules (voir
	 * boxStyle), par-dessus l'aspect par défaut d'un en-tête.
	 */
	function bandStyle(e: Entry, i0: number, i1: number): CellStyle | undefined {
		const r = e.t.row - 1;
		const band: (CellStyle | undefined)[] = [];
		for (let i = i0; i <= i1; i++) band.push(styleAt(r, e.t.col + i, undefined));
		if (i0 === i1 && singleCol(e, i0)) return boxStyle([band, [styleAt(e.t.row, e.t.col + i0, e)]]);
		return boxStyle([band]);
	}

	// Largeur des boutons de l'en-tête de chaque colonne (posée au dessin,
	// lue à la mesure de la hauteur des en-têtes renvoyés à la ligne) ; avant
	// le premier dessin, une largeur habituelle.
	const headIconW = new Map<string, number>();
	const HEAD_ICON_W = 34;

	/** Mesure les lignes de tableau affichées pas encore mesurées et ajuste l'axe des lignes (seulement les lignes changées). */
	function fitVisibleRows() {
		if (!entries.length) return;
		for (let pass = 0; pass < 3; pass++) {
			const { first, last } = visibleRows();
			const changed = autoRows.measure(first, last, measureTableRow);
			if (fRows()) for (const [r, h] of autoRows.measure(0, fRows() - 1, measureTableRow)) changed.set(r, h);
			const sizes = new Map<number, number | undefined>();
			for (const [r, h] of changed) if (!userHeights.has(r) && !hiddenRows.has(r)) sizes.set(r, h);
			if (!sizes.size) return;
			rowAxis = rowAxis.with(sizes);
			updateExtent();
		}
	}

	/** Fusion qui couvre une cellule (cellules libres seulement). */
	const mergeOf = (r: number, c: number) => (merges.length ? mergeAt(merges, r, c) : undefined);
	/** Fusion, ou groupe de la bande d'un tableau sur plusieurs colonnes : une seule cellule pour le curseur, les flèches et la sélection. */
	const boxOf = (r: number, c: number) => (boxes.length ? mergeAt(boxes, r, c) : undefined);

	/** Cellule (r, c), ou le haut à gauche de sa fusion. */
	function snapMerge(r: number, c: number) {
		const g = boxOf(r, c);
		return g ? { row: g.row, col: g.col } : { row: r, col: c };
	}

	/** Cellule active, ou toute sa fusion. */
	// Cadre de sélection animé (choix de Louis) : il glisse de l'ancienne
	// cellule vers la nouvelle et s'étire vers la plage sélectionnée, en
	// SEL_MS avec une courbe ease-out. Coupé pendant une touche maintenue,
	// pour un saut lointain et si l'animation est désactivée. Les boîtes sont
	// en coordonnées de feuille (le défilement ne les change pas).
	type Box = { x: number; y: number; w: number; h: number };
	const SEL_MS = 80;
	let selShown: Box | null = null;
	let selAnim: { from: Box; to: Box; t0: number } | null = null;
	let noAnimUntil = 0;

	/** Cadre extérieur cible : la plage sélectionnée, ou la cellule active (sa fusion). */
	function frameTarget(): Box {
		const s = singleSelection() ? activeRect() : selection();
		return { x: cax().offset(s.c0), y: rowTop(s.r0), w: cax().offset(s.c1 + 1) - cax().offset(s.c0), h: rspan(s.r0, s.r1) };
	}

	/** Cadre à dessiner pour cette image ; demande l'image suivante tant qu'il bouge. */
	function frameNow(now: number): Box {
		const to = frameTarget();
		const same = (a: Box, b: Box) => a.x === b.x && a.y === b.y && a.w === b.w && a.h === b.h;
		const far = selShown && (Math.abs(to.x - selShown.x) > viewW * 2 || Math.abs(to.y - selShown.y) > viewH * 2);
		// Largeur ou hauteur tirée : le cadre suit la grille sans glisser derrière.
		if (!selShown || !motion() || now < noAnimUntil || far || lines || resizing || rowResizing) {
			selAnim = null;
			selShown = to;
			return to;
		}
		if (!same(selAnim?.to ?? selShown, to)) selAnim = { from: { ...selShown }, to, t0: now };
		if (!selAnim) return (selShown = to);
		const k = Math.min(1, (now - selAnim.t0) / SEL_MS);
		const e = 1 - (1 - k) ** 3;
		const { from } = selAnim;
		selShown = { x: from.x + (to.x - from.x) * e, y: from.y + (to.y - from.y) * e, w: from.w + (to.w - from.w) * e, h: from.h + (to.h - from.h) * e };
		if (k < 1) queueMicrotask(schedule);
		else selAnim = null;
		return selShown;
	}
	let selBox: Box = { x: 0, y: 0, w: 0, h: 0 };

	// --- Animations après une modification (voir anim.ts) ------------------
	// Courtes et discrètes (choix de Louis), coupées par Affichage ›
	// Animations et par prefers-reduced-motion. Chacune part d'un instant
	// posé par applyData (jamais pour une navigation : défilement, autre
	// feuille, ouverture) et vit au dessin, sans relire le classeur ; une
	// image de plus n'est demandée que tant qu'il en reste une en cours.
	const motion = () => animate && !reducedMotion();
	// Fenêtre après une modification où les valeurs qui changent s'animent
	// (lignes rechargées, statistiques des règles arrivées un peu après).
	const EDIT_WINDOW = 1500;
	let editAt = -Infinity;
	let animNow = 0;
	let animBusy = false;
	// 1. Lignes ou colonnes de la feuille insérées ou supprimées : axe décalé.
	let lines: (LinesShift & { t0: number; rows: boolean }) | null = null;
	// Insertion ou suppression demandée, en attente de la mise en page qui la reflète.
	let linesHint: { t: number; rows: boolean; at: number; n: number; px: number; table?: number } | null = null;
	// 1 et 5. Ligne ou colonne insérée ou supprimée dans un tableau : la suite glisse.
	let inner = new Map<number, { t0: number; rows: boolean; at: number; n: number; px: number }>();
	// 3 et 5. Tableaux déplacés, agrandis ou nés ; géométrie dessinée à l'image précédente.
	let motions = new Map<number, Motion>();
	let geoDrawn = new Map<number, Geo>();
	// 2. Barres de données : celles de l'image précédente (cible, affichée), animations en cours.
	let barsPrev = new Map<string, { t: BarSpan; s: BarSpan }>();
	let barsNow = new Map<string, { t: BarSpan; s: BarSpan }>();
	const barAnims = new Map<string, { from: BarSpan; to: BarSpan; t0: number }>();
	// Plage visible (volet principal) de l'image précédente : une barre qui y
	// apparaît après une modification pousse depuis zéro.
	let viewPrev: { r0: number; r1: number; c0: number; c1: number } | null = null;
	let viewNow: { r0: number; r1: number; c0: number; c1: number } | null = null;
	// 4. Valeurs calculées de l'image précédente, et éclats en cours.
	let valsPrev = new Map<string, string>();
	let valsNow = new Map<string, string>();
	const flashes = new Map<string, number>();
	// Cellule saisie : elle ne s'éclaire pas.
	let typed = { row: -1, col: -1, t: -Infinity };
	// 2. Graphiques nés d'une modification : ils poussent depuis zéro.
	let grownCharts = $state.raw(new Set<number>());
	// Graphiques : leurs valeurs glissent vers les nouvelles après une
	// modification seulement ; après une navigation (ouverture, autre feuille),
	// les données qui arrivent s'affichent telles quelles.
	let chartTween = $state(false);

	/** Début d'une image : axes animés, valeurs de l'image précédente. */
	function beginAnimFrame(now: number) {
		animNow = now;
		animBusy = false;
		rowDraw = rowAxis;
		colDraw = colAxis;
		if (lines) {
			const k = progress(now, lines.t0);
			if (k >= 1 || !motion()) lines = null;
			else {
				animBusy = true;
				if (lines.rows) rowDraw = shiftAxis(rowAxis, lines, ease(k));
				else colDraw = shiftAxis(colAxis, lines, ease(k));
			}
		}
		drawing = true;
		const drawn = geoDrawn;
		geoDrawn = new Map();
		if (!motion()) {
			grownSeen.clear();
			if (valsNow.size || barsNow.size || flashes.size || barAnims.size || motions.size || inner.size) {
				valsNow = new Map();
				barsNow = new Map();
				flashes.clear();
				barAnims.clear();
				motions.clear();
				inner.clear();
			}
			return;
		}
		valsPrev = valsNow;
		valsNow = new Map();
		barsPrev = barsNow;
		barsNow = new Map();
		viewPrev = viewNow;
		growMotions(drawn, now);
	}

	// Cadre prolongé par la cellule active (juste dessous ou à droite), ou
	// rendu quand elle s'en va, sans que le tableau change : il s'étire ou se
	// rétracte en douceur, depuis le cadre dessiné à l'image précédente.
	const GROW_MS = 160;
	let grownSeen = new Map<number, string>();
	function growMotions(drawn: Map<number, Geo>, now: number) {
		const seen = new Map<number, string>();
		for (const e of entries) {
			const g = grown(e);
			const base = `${e.t.row},${e.t.col},${e.last},${e.lastCol}`;
			const k = `${base}|${g.last},${g.lastCol},${g.firstCol}`;
			seen.set(e.t.id, k);
			const was = grownSeen.get(e.t.id);
			// Tableau lui-même modifié (saisie, insertion) : startAnims s'en charge.
			if (!was || was === k || !was.startsWith(base + '|')) continue;
			const from = drawn.get(e.t.id);
			if (from && !motions.get(e.t.id)?.born) motions.set(e.t.id, { t0: now, from, ms: GROW_MS, grow: true });
		}
		grownSeen = seen;
	}

	/** Fin d'une image : vrais axes, image suivante s'il reste une animation. */
	function endAnimFrame() {
		drawing = false;
		if (animBusy) schedule();
	}

	// Ouverture d'une feuille : les mises en page qui suivent de près (données
	// relues, tableaux, graphiques) appartiennent encore à la navigation, et
	// la feuille s'affiche d'un coup, sans tableaux qui se déplient.
	const NAV_SETTLE = 1000;
	let navAt = -Infinity;
	/**
	 * Animations d'une nouvelle mise en page (après rebuildAxis et rebuildRows) :
	 * insertion ou suppression demandée, tableaux déplacés, agrandis ou nés.
	 */
	function startAnims(nav: boolean, known: Set<number>, oldCharts: Set<number>, now: number) {
		if (nav) navAt = now;
		else if (now - navAt < NAV_SETTLE) nav = true;
		const hint = linesHint && now - linesHint.t < 3000 ? linesHint : null;
		linesHint = null;
		if (nav || !motion()) {
			editAt = -Infinity;
			chartTween = false;
			lines = null;
			motions.clear();
			inner.clear();
			flashes.clear();
			barAnims.clear();
			valsNow = new Map();
			barsNow = new Map();
			viewNow = null;
			if (grownCharts.size) grownCharts = new Set();
			return;
		}
		editAt = now;
		chartTween = true;
		const born = charts.filter((c) => !oldCharts.has(c.id)).map((c) => c.id);
		if (born.length || grownCharts.size) grownCharts = new Set(born);
		if (hint && hint.table === undefined) {
			// Lignes ou colonnes de la feuille : tout glisse avec l'axe ; les
			// cellules libres ont changé de place, leurs valeurs ne s'éclairent pas.
			const view = hint.rows ? viewH : viewW;
			lines = { t0: now, rows: hint.rows, at: hint.at, n: hint.n, px: Math.min(hint.px, view) };
			motions.clear();
			inner.clear();
			valsNow = new Map();
			barsNow = new Map();
			schedule();
			return;
		}
		// Une même modification peut arriver en deux mises en page (relecture,
		// puis événement du classeur) : les animations en cours continuent.
		if (lines) return;
		const geo = new Map(entries.map((e) => [e.t.id, tableGeo(e)]));
		for (const [id, m] of tableMotions(geoDrawn, known, geo, now)) if (!motions.get(id)?.born) motions.set(id, m);
		for (const id of inner.keys()) if (!geo.has(id)) inner.delete(id);
		if (hint && hint.table !== undefined && entries.some((e) => e.t.id === hint.table)) inner.set(hint.table, { t0: now, rows: hint.rows, at: hint.at, n: hint.n, px: hint.px });
		schedule();
	}

	/** Insertion ou suppression demandée (lignes ou colonnes de la feuille, ou d'un tableau). */
	function hintLines(rows: boolean, at: number, n: number, table?: number) {
		if (!motion()) return;
		const px = n > 0 ? (rows ? rh(at) : cax().size(at)) : rows ? rspan(at, at - n - 1) : cax().offset(at - n) - cax().offset(at);
		linesHint = { t: performance.now(), rows, at, n, px, table };
	}

	/** Cadre affiché d'un tableau (prolongé quand la cellule active est juste dessous, à droite ou à gauche), en pixels de la feuille. */
	function tableGeo(e: Entry): Geo {
		const g = grown(e);
		const x = cax().offset(g.firstCol);
		// La bande des groupes fait partie du cadre.
		const top = topRow(e.t);
		const y = rowTop(top);
		return { x, y, w: cax().offset(g.lastCol + 1) - x, h: rspan(top, g.last) };
	}

	// ox : bord gauche du cadre animé par rapport à sa place finale (cadre qui
	// s'étire vers la gauche, contenu immobile).
	type TableAnim = { dx: number; dy: number; ox: number; w: number; h: number; alpha: number; rowOff: (r: number) => number; colOff: (c: number) => number };
	const noOff = () => 0;
	let tableAnims = new Map<number, TableAnim | null>();
	let tableAnimsAt = -1;

	/** Mouvement d'un tableau à cette image (null : immobile). */
	function tableAnim(e: Entry): TableAnim | null {
		if (!motions.size && !inner.size) return null;
		if (tableAnimsAt !== animNow) {
			tableAnims = new Map();
			tableAnimsAt = animNow;
		}
		const id = e.t.id;
		if (tableAnims.has(id)) return tableAnims.get(id)!;
		let out: TableAnim | null = null;
		const to = tableGeo(e);
		const m = motions.get(id);
		if (m) {
			const k = progress(animNow, m.t0, m.ms);
			if (k >= 1) motions.delete(id);
			else {
				animBusy = true;
				const g = motionAt(m, to, ease(k), rh(e.t.row));
				// Cadre seul étiré : il s'ouvre vers la gauche, le contenu ne glisse pas.
				out = m.grow ? { dx: 0, dy: 0, ox: g.x - to.x, w: g.w, h: g.h, alpha: g.alpha, rowOff: noOff, colOff: noOff } : { dx: g.x - to.x, dy: g.y - to.y, ox: 0, w: g.w, h: g.h, alpha: g.alpha, rowOff: noOff, colOff: noOff };
			}
		}
		const s = inner.get(id);
		if (s) {
			const k = progress(animNow, s.t0);
			if (k >= 1) inner.delete(id);
			else {
				animBusy = true;
				const left = (1 - ease(k)) * s.px;
				// Insertion : ce qui suit la nouvelle ligne part de son ancienne place ;
				// suppression : ce qui suivait la ligne supprimée remonte.
				const off = (i: number) => (s.n > 0 ? (i > s.at ? -left : 0) : i >= s.at ? left : 0);
				out ??= { dx: 0, dy: 0, ox: 0, w: to.w, h: to.h, alpha: 1, rowOff: noOff, colOff: noOff };
				if (s.rows) out.rowOff = off;
				else out.colOff = off;
			}
		}
		tableAnims.set(id, out);
		return out;
	}

	/**
	 * Entre dans le repère d'un tableau en mouvement : décalage, opacité, et
	 * découpe à son cadre animé (agrandi de ce que la saisie prolonge) ;
	 * faux s'il est immobile (rien à restaurer).
	 */
	function enterTable(e: Entry, cx: (c: number) => number, ry: (r: number) => number): boolean {
		const a = tableAnim(e);
		if (!a) return false;
		ctx.save();
		ctx.translate(a.dx, a.dy);
		ctx.globalAlpha *= a.alpha;
		ctx.beginPath();
		ctx.rect(cx(grown(e).firstCol) + a.ox - 1, ry(topRow(e.t)) - 1, a.w + 2, a.h + 2);
		ctx.clip();
		return true;
	}

	/** Barre de données affichée pour la cellule k (sheet r, c) : de l'ancienne valeur, ou de zéro, à la nouvelle. */
	function barShown(k: string, b: BarSpan & { axis?: number; rev?: boolean }, r: number, c: number): BarSpan {
		if (!motion()) return b;
		const target = { a: b.a, b: b.b };
		const prev = barsPrev.get(k);
		let a = barAnims.get(k);
		if (animNow - editAt < EDIT_WINDOW) {
			const v = viewPrev;
			if (prev && !sameBar(prev.t, target)) barAnims.set(k, (a = { from: prev.s, to: target, t0: animNow }));
			else if (!prev && !a && v && r >= v.r0 && r <= v.r1 && c >= v.c0 && c <= v.c1) barAnims.set(k, (a = { from: zeroBar(b), to: target, t0: animNow }));
		}
		let s: BarSpan = target;
		if (a) {
			const p = progress(animNow, a.t0);
			if (p >= 1 || !sameBar(a.to, target)) barAnims.delete(k);
			else {
				s = lerpBar(a.from, target, ease(p));
				animBusy = true;
			}
		}
		barsNow.set(k, { t: target, s });
		return s;
	}

	// Formule de colonne validée : toutes ses cellules flashent, même si leur
	// valeur ne change pas (on voit que toute la colonne a été modifiée). Les
	// lignes chargées ensuite flashent à leur premier affichage.
	let columnFlash: { table: number; name: string; t: number; done: Set<string> } | null = null;
	function columnFlashes(table: number, name: string, k: string) {
		const f = columnFlash;
		if (!f || f.table !== table || f.name !== name || f.done.has(k)) return;
		if (animNow - f.t > 1500) {
			columnFlash = null;
			return;
		}
		f.done.add(k);
		flashes.set(k, animNow);
	}

	/** Éclat d'une valeur calculée (clé k, cellule r, c) qui vient de changer : opacité (0 : aucun). */
	function flashOf(k: string, v: string, r: number, c: number): number {
		if (!motion()) return 0;
		const prev = valsPrev.get(k);
		valsNow.set(k, v);
		if (prev !== undefined && prev !== v && animNow - editAt < EDIT_WINDOW && (r !== typed.row || c !== typed.col || animNow - typed.t > EDIT_WINDOW)) flashes.set(k, animNow);
		const t0 = flashes.get(k);
		if (t0 === undefined) return 0;
		const a = flashAlpha(animNow, t0);
		if (a <= 0) flashes.delete(k);
		else animBusy = true;
		return a;
	}

	/** Pose l'éclat d'une cellule ou d'une zone (sous son texte). */
	function paintFlash(a: number, x: number, y: number, w: number, h: number) {
		if (a <= 0) return;
		ctx.fillStyle = `rgba(33, 115, 70, ${(0.2 * a).toFixed(3)})`;
		ctx.fillRect(x, y, w, h);
	}

	function activeRect() {
		const g = boxOf(active.row, active.col);
		return g ? mergeRect(g) : { r0: active.row, r1: active.row, c0: active.col, c1: active.col };
	}

	/** Étendue affichée d'un tableau : une ligne ou une colonne de plus quand la
	 * cellule active est juste dessous ou juste à droite (la saisie l'étendra),
	 * jusqu'au tableau voisin qu'elle rejoindrait (fusion). */
	function grown(e: Entry): Grown {
		return growFrame(e, { row: active.row, col: active.col, entries, locked: !!locked, keptOut, inRange: inRangeZone });
	}

	const inTable = (e: Entry, r: number, c: number) => r >= e.t.row && r <= e.last && c >= e.t.col && c <= e.lastCol;
	const tableAt = (r: number, c: number) => entries.find((e) => inTable(e, r, c));
	/** Tableau aux en-têtes sur deux lignes dont la bande des groupes (ligne row - 1) contient la cellule. */
	/** Tableau dont l'en-tête est juste sous la sélection (sans groupes d'en-têtes) : sa ligne du dessus peut devenir ses groupes. */
	function groupCandidate(): Entry | undefined {
		if (!source.setTwoLevel) return undefined;
		const sel = selection();
		return entries.find((e) => !e.t.twoLevel && !e.t.query && e.t.row - 1 >= sel.r0 && e.t.row - 1 <= sel.r1 && e.t.col <= sel.c1 && e.lastCol >= sel.c0);
	}
	const groupBandAt = (r: number, c: number) => entries.find((e) => e.t.twoLevel && r === e.t.row - 1 && c >= e.t.col && c <= e.lastCol);
	const topRow = tableTop;
	/**
	 * Colonnes sur un niveau défusionnées à la main (Défusionner) : séparées
	 * en bande + en-tête le temps de leur taper un groupe (tableau, nom de la
	 * colonne). Rien n'est écrit tant qu'aucun groupe n'est tapé.
	 */
	const splitCols = new Set<string>();
	const splitKey = (table: string, name: string) => `${table}\u0001${name}`;
	const isSplit = (t: { name?: string }, name: string) => splitCols.has(splitKey(t.name ?? '', name));
	/** Colonne i d'un tableau à deux lignes sur un niveau : bande et en-tête ne font qu'une cellule. */
	const singleCol = (e: Entry, i: number) => !!e.t.twoLevel && !!e.t.columns[i] && isSingle(e.t.columns[i].name, isSplit(e.t, e.t.columns[i].name));
	/** Fusions de la feuille et cellules de la bande des groupes (curseur, flèches, sélection). */
	const rebuildBoxes = () => (boxes = [...merges, ...bandMerges(entries.map((e) => e.t), isSplit)]);
	/** Tableau de la cellule, sa bande des groupes comprise (zone de nom, gouttière, ruban). */
	const ownerAt = (r: number, c: number) => tableAt(r, c) ?? groupBandAt(r, c);
	/** Ligne de données d'un autre membre, en lecture seule pour un participant (§8.1). */
	function lockedRowAt(r: number, c: number): boolean {
		const e = tableAt(r, c);
		if (!e || e.t.access !== 'participant' || r <= e.t.row) return false;
		return !!e.cache.row(r - e.t.row - 1)?.locked;
	}
	const NOT_YOURS = "Ligne d'un autre membre : en tant que participant, vous ne modifiez que vos propres lignes";
	/** Ligne survolée en lecture seule (participant) : légèrement grisée ; -1 sinon. */
	let hoverLocked = -1;

	// La zone de défilement suit le contenu, comme Excel : la partie utilisée
	// de la feuille plus un écran, et elle grandit à mesure qu'on défile vers
	// le bas ou la droite (au lieu de barres faites pour 1 M de lignes vides).
	// Hauteurs des lignes : par défaut ROW_H, sinon celles choisies (marge de gauche).
	let rowAxis = $state.raw(new Axis(SHEET_ROWS, ROW_H));
	let userHeights = new Map<number, number>();
	// Pendant le dessin, les axes peuvent être ceux d'une insertion ou d'une
	// suppression en cours d'animation (voir anim.ts) ; ailleurs (clics,
	// défilement, calques), toujours les vrais.
	let drawing = false;
	let rowDraw: AxisLike = new Axis(SHEET_ROWS, ROW_H);
	let colDraw: AxisLike = new Axis(SHEET_COLS, DEF_W);
	const rax = (): AxisLike => (drawing ? rowDraw : rowAxis);
	const cax = (): AxisLike => (drawing ? colDraw : colAxis);
	const rh = (r: number) => rax().size(r);
	const rowTop = (r: number) => rax().offset(r);
	const rspan = (r0: number, r1: number) => rax().offset(r1 + 1) - rax().offset(r0);
	function rebuildRows(override?: { row: number; height: number }) {
		// Hauteur automatique (renvoyer à la ligne), sauf hauteur choisie ; masquée : nulle.
		const hs = autoRows.all();
		for (const [r, h] of userHeights) hs.set(r, h);
		if (override) hs.set(override.row, override.height);
		for (const r of hiddenRows) hs.set(r, 0);
		rowAxis = new Axis(SHEET_ROWS, ROW_H, hs);
	}

	const vs = new VScroll(100 * ROW_H, 0);
	let spacer: HTMLDivElement;
	let extentCols = 26;
	// Largeur de la gouttière : élargie d'après le plus grand numéro de ligne
	// visible, comme Excel (« 1000000 » ne doit pas être coupé).
	let MARGIN = $state(MIN_MARGIN);

	// Pendant qu'on tire une barre de défilement, la zone ne rétrécit pas (la
	// barre ne « glisse » plus sous la souris) ; arrivée au bout, elle
	// s'agrandit d'un écran, comme Excel.
	let holdRows = 0;
	let holdCols = 0;

	// Pendant un défilement (molette, pavé, barre), la zone ne rétrécit pas :
	// les barres gardent leur taille et ne sautent pas. Elle se réajuste un
	// instant après le dernier mouvement.
	let scrolling = 0;
	let scrollHoldRows = 0;
	let scrollHoldCols = 0;
	function holdWhileScrolling() {
		// Le plus grand atteint pendant le geste (il a pu s'agrandir en route).
		scrollHoldRows = Math.max(scrollHoldRows, rax().indexAt(vs.content));
		scrollHoldCols = Math.max(scrollHoldCols, extentCols);
		clearTimeout(scrolling);
		scrolling = window.setTimeout(() => {
			scrolling = 0;
			scrollHoldRows = scrollHoldCols = 0;
			updateExtent();
			schedule();
		}, 400);
	}

	/** Barre horizontale (ligne des onglets) tirée : figer la largeur, l'agrandir au bout. */
	export function holdHScroll(on: boolean) {
		holdCols = on ? extentCols : 0;
		updateExtent();
	}

	function growAtEnd(rows: boolean) {
		if (rows) {
			const lastVisible = rax().indexAt(vs.top + vs.view);
			holdRows = Math.min(SHEET_ROWS, Math.max(holdRows, lastVisible + Math.max(10, Math.floor(vs.view / ROW_H))));
		} else {
			const cw = Math.max(0, viewW - MARGIN);
			holdCols = Math.min(SHEET_COLS, Math.max(holdCols, cax().indexAt(scroller.scrollLeft + cw) + Math.max(3, Math.floor(cw / DEF_W))));
		}
	}

	function updateExtent() {
		if (!scroller) return;
		let usedRow = Math.max(active.row, anchor.row);
		let usedCol = Math.max(active.col, anchor.col);
		for (const e of entries) {
			usedRow = Math.max(usedRow, e.t.row + e.t.rows + (e.t.hidden ?? 0) + 1);
			usedCol = Math.max(usedCol, e.lastCol + 1);
		}
		for (const k of cells.keys()) {
			const [r, c] = k.split(',').map(Number);
			usedRow = Math.max(usedRow, r);
			usedCol = Math.max(usedCol, c);
		}
		// Objets posés sur la feuille (graphiques et cartes, boutons et
		// contrôles) : la barre de défilement va jusqu'à leur bord, qu'ils
		// soient sélectionnés ou non (objextent.ts) ; de même pour les
		// tableaux croisés et les tableaux déversés.
		const boxes = [...charts.map(chartBox), ...buttons.map(buttonBox)];
		const reach = boxesReach(boxes, (y) => rax().indexAt(y), (x) => cax().indexAt(x));
		const blocks = blocksReach([
			...pivots.map((p) => ({ row: p.row, col: p.col, rows: p.cells.length, cols: p.cells[0]?.length ?? 0 })),
			...spills.map((s) => ({ row: s.row, col: s.col, rows: s.rows.length, cols: s.rows[0]?.length ?? 0 }))
		]);
		usedRow = Math.max(usedRow, reach.row, blocks.row);
		usedCol = Math.max(usedCol, reach.col, blocks.col);
		const lastVisible = rax().indexAt(vs.top + vs.view);
		// Comme Excel : la barre s'arrête à la zone utilisée (ou à l'écran affiché) ;
		// molette, flèches et sélection vont au-delà et l'agrandissent.
		const rows = Math.min(SHEET_ROWS, Math.max(usedRow + 1, lastVisible + 1, holdRows, scrollHoldRows));
		extentRows = Math.min(SHEET_ROWS, usedRow + 1);
		const cw = Math.max(0, viewW - MARGIN);
		// Volet ouvert à droite (graphique, carte, tableau croisé…) : la zone
		// qui défile garde la largeur qu'elle aura volet fermé. Sinon, en
		// quittant l'objet, le volet se ferme, la grille s'élargit et le
		// navigateur ramène la barre en arrière : la feuille sautait.
		const side = wrap ? Math.max(0, (wrap.parentElement?.clientWidth ?? 0) - wrap.clientWidth) / zoomK() : 0;
		const lastVisibleCol = cax().indexAt(scroller.scrollLeft + cw + side);
		extentCols = Math.min(SHEET_COLS, Math.max(usedCol + 1, lastVisibleCol + 1, holdCols, scrollHoldCols));
		vs.content = rowTop(rows);
		vs.to(vs.top);
		onhscroll?.({ content: MARGIN + cax().offset(extentCols), view: scroller.clientWidth, left: scroller.scrollLeft });
		if (spacer) {
			spacer.style.height = `${TOP_H + vs.spacer}px`;
			spacer.style.width = `${MARGIN + cax().offset(extentCols)}px`;
		}
	}
	let viewW = $state(0);
	let viewH = $state(0);

	let wrap: HTMLDivElement;
	let scroller: HTMLDivElement;
	let canvas: HTMLCanvasElement;
	let input = $state<HTMLTextAreaElement>();
	// Champ de saisie : éditeur de cellule (zone de texte qui passe à la ligne) ou barre de formule.
	type TextField = HTMLInputElement | HTMLTextAreaElement;
	let ctx: CanvasRenderingContext2D;
	let mounted = false;

	// Sélection, comme Excel : la cellule active reste où la sélection a
	// commencé ; « anchor » est le coin opposé, celui qui bouge (Maj + flèches,
	// Maj + clic, glisser). La plage s'agrandit aux fusions qu'elle touche
	// (selection(), mergesel.ts).
	let active = $state.raw({ row: 0, col: 0 });
	let anchor = { row: 0, col: 0 };
	let dragging = false;
	// Déplacement par la bordure de la sélection (comme Excel).
	let moving = $state<{
		sel: { r0: number; r1: number; c0: number; c1: number };
		from: { row: number; col: number };
		to?: { row: number; col: number };
		/** Ctrl enfoncé : copie au lieu de déplacer (Excel). */
		copy?: boolean;
	} | null>(null);

	// Édition (dans la cellule ou dans la barre de formule).
	let editing = $state(false);
	let editByTyping = false;
	let editValueText = $state('');
	// Éditeur ouvert en lecture seule (readonlyedit.ts) : pourquoi la cellule
	// ne se modifie pas ; '' pour une saisie normale.
	let editRO = $state('');
	// Barre de formule sur une cellule en lecture seule : son texte se
	// sélectionne et se copie, sans se modifier (même motif).
	let fxRO = $state('');
	let fxText = $state('');
	let fxInput = $state<HTMLInputElement>();
	// Zone de nom d'une cellule libre (nom refusé : la saisie y reprend).
	let nameInput = $state<HTMLInputElement>();
	// Faute de syntaxe d'une formule refusée à la validation : l'éditeur reste
	// ouvert, le caractère fautif est souligné.
	let editErr = $state<FormulaError | null>(null);
	// Hauteur de l'éditeur de cellule (il passe à la ligne quand il ne peut plus s'élargir).
	let editorH = $state(22);
	let fxScroll = $state(0);

	// Aide à la saisie des formules (éditeur de cellule ou barre de formule).
	let completion = $state<(Completion & { sel: number; where: 'cell' | 'fx' }) | null>(null);
	let sig = $state<{ fn: FunctionDoc; index: number; where: 'cell' | 'fx' } | null>(null);
	let suggestPos = $state({ x: 0, y: 0 });

	function onFormulaInput(el: TextField, where: 'cell' | 'fx') {
		editErr = null;
		if (where === 'cell') {
			fxText = el.value;
			placeEditor();
			// Une formule se lit de gauche à droite, quelle que soit la colonne.
			if (el.value.startsWith('=')) el.style.textAlign = 'left';
		}
		if (!catalog) return;
		suggestPos = { x: toView(screenX(active.col)), y: toView(screenY(active.row) + Math.max(editorH, rh(active.row))) + 2 };
		const caret = el.selectionStart ?? el.value.length;
		const e = tableAt(active.row, active.col);
		const c = suggest(el.value, caret, catalog, e?.t.name);
		completion = c ? { ...c, sel: 0, where } : null;
		const s = signature(el.value, caret, catalog);
		sig = s ? { ...s, where } : null;
	}

	/** Le curseur bouge sans saisie (flèches, clic) : l'argument en gras suit. */
	function moveSig(e: Event, where: 'cell' | 'fx') {
		if (!catalog || (e instanceof KeyboardEvent && !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key))) return;
		const el = e.currentTarget as TextField;
		const s = signature(el.value, el.selectionStart ?? el.value.length, catalog);
		sig = s ? { ...s, where } : null;
	}

	/** Gère les touches de la liste de suggestions ; vrai si la touche est consommée. */
	function completionKey(e: KeyboardEvent, el: TextField, set: (v: string) => void): boolean {
		if (!completion) return false;
		const c = completion;
		switch (e.key) {
			case 'ArrowDown':
				completion = { ...c, sel: (c.sel + 1) % c.items.length };
				break;
			case 'ArrowUp':
				completion = { ...c, sel: (c.sel + c.items.length - 1) % c.items.length };
				break;
			case 'Tab':
			case 'Enter': {
				const caret = el.selectionStart ?? el.value.length;
				const r = applySuggestion(el.value, caret, c.from, c.items[c.sel]);
				set(r.text);
				completion = null;
				requestAnimationFrame(() => {
					el.setSelectionRange(r.caret, r.caret);
					onFormulaInput(el, c.where);
				});
				break;
			}
			case 'Escape':
				completion = null;
				break;
			default:
				return false;
		}
		e.preventDefault();
		e.stopPropagation();
		return true;
	}
	let nameBox = $state('');
	// Cellule d'un tableau : nom du tableau (modifiable) et colonne[ligne].
	let tableBox = $state<{ name: string; col: string } | null>(null);
	let fxEditing = $state(false);
	// Cellule active pour le lecteur d'écran (GridA11y.svelte).
	const a11yId = $props.id();
	let speech = $state.raw<CellSpeech>({ addr: 'A1', value: '' });
	let speechSel = $state('');

	// Pointage sur une autre feuille (comme Excel, crossref.ts) : un clic sur un
	// onglet pendant la saisie d'une formule ne la valide pas ; sur l'autre
	// feuille, la formule reste dans la barre de formule (hold : la saisie
	// gardée), un clic y insère la référence au curseur, Entrée la valide dans
	// sa cellule et Échap l'abandonne, en revenant à sa feuille.
	let hold = $state.raw<FormulaHold | null>(null);
	/** Motif de lecture seule de la barre de formule ('' : modifiable, comme pendant une formule pointée sur une autre feuille). */
	const fxLock = $derived(hold ? '' : locked || fxRO);
	// Appui en cours sur un onglet de feuille : le départ du focus de l'éditeur ne valide pas la formule.
	let tabPress = false;
	$effect(() => {
		const down = (e: PointerEvent) => {
			tabPress = !!(e.target as Element | null)?.closest?.('.sheet-tabs');
		};
		const up = () => setTimeout(() => (tabPress = false));
		window.addEventListener('pointerdown', down, true);
		window.addEventListener('pointerup', up, true);
		return () => {
			window.removeEventListener('pointerdown', down, true);
			window.removeEventListener('pointerup', up, true);
		};
	});

	/** La feuille from est quittée : une formule en cours de saisie est gardée. */
	function keepFormula(from: number) {
		if (hold) {
			// Déjà sur une autre feuille : on en change encore, la saisie suit.
			if (fxEditing && fxInput) hold = updateHold(hold, fxText, fxInput.selectionStart, fxInput.selectionEnd);
			return;
		}
		if (!editing && !fxEditing) return;
		// Éditeur en lecture seule : rien à garder, il se referme.
		if (editing && editRO) {
			editing = false;
			return;
		}
		const el = editing ? input : fxInput;
		const text = editing ? editValueText : fxText;
		const h = holdFormula({ sheet: from, row: active.row, col: active.col, where: editing ? 'cell' : 'fx' }, text, el?.selectionStart ?? null, el?.selectionEnd ?? null);
		if (!h) return;
		hold = h;
		editing = false;
		editErr = null;
		completion = sig = null;
		lastRef = null;
	}

	/** Après un changement de feuille, la saisie gardée : dans la barre de formule, ou de retour dans sa cellule. */
	function placeHold(sheet: number) {
		const h = hold;
		if (!h || sheet !== placedSheet) return;
		if (sheet !== h.sheet) {
			// Autre feuille : la formule dans la barre de formule, curseur gardé.
			flushSync(() => {
				fxText = h.text;
				fxEditing = true;
			});
			fxInput?.focus();
			fxInput?.setSelectionRange(h.start, h.end);
			if (fxInput) onFormulaInput(fxInput, 'fx');
			say('Formule en cours : cliquez une cellule ou une colonne pour insérer sa référence ; Entrée valide, Échap annule');
			return;
		}
		// Retour à la feuille de la formule.
		hold = null;
		lastRef = null;
		fxEditing = false;
		completion = sig = null;
		active = { row: h.row, col: h.col };
		anchor = { ...active };
		if (h.finish === 'cancel') {
			refreshFx();
			wrap.focus();
			schedule();
			return;
		}
		if (h.where === 'fx' && !h.finish) {
			// Saisie commencée dans la barre de formule : elle y reprend.
			flushSync(() => {
				fxText = h.text;
				fxEditing = true;
			});
			fxInput?.focus();
			fxInput?.setSelectionRange(h.start, h.end);
			schedule();
			return;
		}
		startEdit(true, h.text);
		if (!editing || !input) return;
		input.setSelectionRange(h.start, h.end);
		onFormulaInput(input, 'cell');
		// Retour pour l'argument suivant : le focus que le changement de feuille
		// remet à la grille ne doit pas valider la formule.
		if (!h.finish) keepEditorUntil = performance.now() + 400;
		// Entrée sur l'autre feuille : validée dans sa cellule (une faute laisse l'éditeur ouvert).
		if (h.finish === 'commit' && commit()) moveTo(h.row + 1, h.col);
		schedule();
	}

	// Jusqu'à cet instant, l'éditeur de cellule qui perd le focus le reprend (retour de pointage).
	let keepEditorUntil = 0;

	/** Entrée ou Échap sur l'autre feuille : retour à la feuille de la formule, qui est validée ou abandonnée. */
	function finishHold(finish: 'commit' | 'cancel') {
		if (!hold) return;
		hold = { ...(fxInput ? updateHold(hold, fxText, fxInput.selectionStart, fxInput.selectionEnd) : hold), finish };
		completion = sig = null;
		onsheet?.(hold.sheet);
	}

	/** Reprend la saisie d'une formule restée ouverte (clic sur l'onglet affiché). */
	export function resumeFormula() {
		if (editing) input?.focus();
		else if (fxEditing) fxInput?.focus();
	}

	let message = '';
	// Le message s'efface seul (noticeTtl) ; un nouveau message relance le délai.
	let shownMessage = '';
	let messageTimer: ReturnType<typeof setTimeout> | undefined;
	/** Message venu du classeur (tableaux fusionnés, cellules rattachées). */
	export function announce(m: string) {
		say(m);
	}
	/** Efface le message de la barre d'état (changement d'onglet). */
	export function clearMessage() {
		clearTimeout(messageTimer);
		message = shownMessage = '';
		refreshStatus();
	}
	let bench = $state('');

	// Mesures.
	const drawTimes: number[] = [];
	const frameStamps: number[] = [];
	let lastEditMs = NaN;
	let missingFrames = 0;

	$effect(() => {
		const d = data;
		untrack(() => {
			statsEpoch++;
			applyData(d);
			// Le ruban suit le tableau relu (filtres, note, lecture seule).
			refreshStatus();
		});
	});

	// Mode A1 : lettres et numéros dans les bandeaux pendant la saisie d'une
	// formule (ou toujours, Affichage › En-têtes A1) ; un clic sur une
	// cellule insère alors sa référence.
	// Barre de formule synchrone avec la saisie dans la cellule (et la cellule
	// avec la barre), frappe par frappe, comme Excel.
	$effect(() => {
		if (editing) fxText = editValueText;
	});
	$effect(() => {
		void fxText;
		// Pendant la saisie : aperçu dans la cellule, cadres des références.
		if (fxEditing || editing) untrack(() => schedule());
	});

	// Barre de formule : une formule y est colorée comme dans l'éditeur.
	const fxHl = $derived(isFormula(fxText));
	const formulaMode = $derived((editing && !editRO && editValueText.trimStart().startsWith('=')) || (fxEditing && fxText.trimStart().startsWith('=')));
	const a1 = $derived(headings || formulaMode);
	// Formule d'une colonne de tableau en cours d'édition : elle s'appliquera
	// à toute la colonne, teintée dès le focus (avec ƒx dans l'en-tête et un
	// bandeau dans la barre de formule).
	const calcEdit = $derived.by(() => {
		if ((!editing && !fxEditing) || (editing && editRO)) return null;
		const e = tableAt(active.row, active.col);
		if (!e || active.row <= e.t.row) return null;
		const i = active.col - e.t.col;
		return e.t.columns[i]?.formula || formulaMode ? { e, i } : null;
	});
	// Portée d'une modification qui touchera une colonne ou une plage entière
	// (demande de Louis) : teintée comme la colonne d'une formule en cours
	// d'édition. Menu ▾ d'une colonne (type, tri, filtre, clé), mise en forme
	// conditionnelle et validation des données ouvertes : leur cible.
	const scopeTints = $derived.by((): Range[] => {
		if (calcEdit) {
			const { e, i } = calcEdit;
			return [{ r0: e.t.row + 1, r1: e.last, c0: e.t.col + i, c1: e.t.col + i }];
		}
		if (filterDlg) {
			const { e, i } = filterDlg;
			return [{ r0: e.t.row + 1, r1: e.last, c0: e.t.col + i, c1: e.t.col + i }];
		}
		if (cfView || (validDlg && !dvPick)) {
			const sel = selection();
			// Dans un tableau, une règle vaut pour ses colonnes entières.
			const e = tableAt(active.row, active.col);
			if (e) return [{ r0: e.t.row + 1, r1: e.last, c0: Math.max(sel.c0, e.t.col), c1: Math.min(sel.c1, e.lastCol) }];
			return singleSelection() ? [] : [sel];
		}
		return [];
	});
	$effect(() => {
		void calcEdit;
		void scopeTints;
		untrack(() => schedule());
	});

	$effect(() => {
		void gridlines;
		void a1;
		void pageBreaks;
		untrack(() => schedule());
	});

	let frame = 0;
	function schedule() {
		if (mounted && !frame) frame = requestAnimationFrame(draw);
	}

	function visibleRows() {
		const first = rax().indexAt(vs.top);
		const last = Math.min(SHEET_ROWS - 1, rax().indexAt(vs.top + vs.view - 1));
		return { first, last };
	}

	/** Lignes de données d'un tableau visibles (indices de données), ou null. */
	function visibleData(e: Entry, first: number, last: number) {
		const a = Math.max(first, e.t.row + 1);
		const b = Math.min(last, e.last);
		return a <= b ? { first: a - e.t.row - 1, last: b - e.t.row - 1 } : null;
	}

	const isNumeric = isNumericText;

	/**
	 * Style effectif d'une cellule de la feuille, quelle qu'elle soit (cellule
	 * de tableau, en-tête, cellule libre, de tableau croisé ou vide) : colonne
	 * puis ligne de la feuille, colonne du tableau, cellule. Sans la mise en
	 * forme conditionnelle.
	 */
	function styleAt(r: number, c: number, e = tableAt(r, c)): CellStyle | undefined {
		const col = sheetColStyles.get(c);
		const row = sheetRowStyles.get(r);
		if (!e) return resolveStyle(pivotLayer(pivots, r, c), col, row, freeStyles.get(key(r, c)));
		const i = c - e.t.col;
		if (r === e.t.row) return resolveStyle(col, row, headStyles.get(e.t.id)?.[i]);
		return resolveStyle(col, row, colStyles.get(e.t.id)?.[i], parseStyle(e.cache.row(r - e.t.row - 1)?.sty?.get(i)));
	}

	// --- Volets figés (voir panes.ts) ---------------------------------------
	// La grille se dessine par volets : les lignes et colonnes figées ne
	// défilent pas. Toutes les positions à l'écran et tous les clics passent
	// par screenX / screenY et colAtX / rowAtY.

	const fRows = () => freeze?.rows ?? 0;
	const fCols = () => freeze?.cols ?? 0;
	/** Position à l'écran du début d'une colonne ou d'une ligne. */
	const screenX = (c: number) => MARGIN + cax().offset(c) - (c < fCols() ? 0 : scroller.scrollLeft);
	const screenY = (r: number) => TOP_H + rowTop(r) - (r < fRows() ? 0 : vs.top);
	/** Colonne ou ligne sous un point de la vue. */
	const colAtX = (x: number) => cax().indexAt(toGrid(x, MARGIN, cax().offset(fCols()), scroller.scrollLeft));
	const rowAtY = (y: number) => Math.min(SHEET_ROWS - 1, rax().indexAt(toGrid(y, TOP_H, rowTop(fRows()), vs.top)));

	$effect(() => {
		void freeze?.rows;
		void freeze?.cols;
		untrack(() => schedule());
	});

	// --- Zoom (voir zoom.ts) ------------------------------------------------------
	// La grille calcule en pixels de la feuille (100 %) ; le calque .zoom agrandit
	// l'affichage. Les menus et dialogues restent hors du calque, à taille normale.

	let resizeCanvas = () => {};
	let zoomWheel = 0;
	/** Facteur effectif entre pixels de l'écran et pixels de la feuille. */
	function zoomK(): number {
		const w = scroller?.offsetWidth;
		const k = w ? scroller.getBoundingClientRect().width / w : zoom;
		return k > 0 && Number.isFinite(k) ? k : 1;
	}
	/** Position de la souris dans la vue, en pixels de la feuille. */
	function toLocal(e: MouseEvent): { x: number; y: number } {
		const r = scroller.getBoundingClientRect();
		const k = zoomK();
		return { x: (e.clientX - r.left) / k, y: (e.clientY - r.top) / k };
	}
	/** Position (pixels de la feuille) d'un menu ou dialogue posé hors du calque zoomé. */
	const toView = (v: number) => v * zoom;

	$effect(() => {
		void zoom;
		untrack(() => {
			noteTip = null;
			listMenu = null;
			completion = sig = null;
			resizeCanvas();
		});
	});

	// Défilement vertical du volet en cours de dessin (drawText).
	let paneTop = 0;

	function draw() {
		frame = 0;
		pendingBorders = [];
		if (viewPos.x !== scroller.scrollLeft || viewPos.y !== vs.top) viewPos = { x: scroller.scrollLeft, y: vs.top };
		const t0 = performance.now();
		drawing = false;
		fitVisibleRows();
		beginAnimFrame(t0);
		const w = viewW;
		const h = viewH;
		const dpr = (window.devicePixelRatio || 1) * zoom;
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		selBox = frameNow(t0);
		findCross = foundRect();
		pinnedFrame = [];
		keyHintBoxes = [];
		followBadges = [];
		hiddenMarks = [];
		// Texte centré verticalement partout (les volets dessinent entre save et
		// restore : la ligne de base doit être posée avant, pour la marge et le bandeau).
		ctx.textBaseline = 'middle';
		ctx.fillStyle = C.bg;
		ctx.fillRect(0, 0, w, h);

		ctx.font = FONT_SMALL;
		const margin = gutterWidth();
		if (margin !== MARGIN) {
			MARGIN = margin;
			updateExtent();
		}
		keyGrew = false;
		const fw = cax().offset(fCols());
		const fh = rowTop(fRows());
		const all = panes(fw, fh, MARGIN, TOP_H, w, h, scroller.scrollLeft, vs.top);
		const main = all[all.length - 1] ?? { left: scroller.scrollLeft, top: vs.top, x0: MARGIN, x1: w, y0: TOP_H, y1: h };
		let incomplete = false;
		for (const p of all) {
			ctx.save();
			ctx.beginPath();
			ctx.rect(p.x0, p.y0, p.x1 - p.x0, p.y1 - p.y0);
			ctx.clip();
			if (drawCells(layer(p))) incomplete = true;
			ctx.restore();
		}
		if (incomplete) missingFrames++;
		if (motion())
			viewNow = {
				r0: rax().indexAt(main.y0 - TOP_H + main.top),
				r1: rax().indexAt(main.y1 - 1 - TOP_H + main.top),
				c0: cax().indexAt(main.x0 - MARGIN + main.left),
				c1: cax().indexAt(main.x1 - 1 - MARGIN + main.left)
			};
		// Bandeau du haut (par volet de colonnes) et marge de gauche (par volet de lignes).
		for (const p of all.filter((p) => p.y0 === main.y0)) {
			ctx.save();
			ctx.beginPath();
			ctx.rect(p.x0, 0, p.x1 - p.x0, TOP_H);
			ctx.clip();
			drawTopStrip(layer({ ...p, top: main.top, y0: main.y0, y1: main.y1 }));
			ctx.restore();
		}
		for (const p of all.filter((p) => p.x0 === main.x0)) {
			ctx.save();
			ctx.beginPath();
			ctx.rect(0, p.y0, MARGIN, p.y1 - p.y0);
			ctx.clip();
			drawMargin(layer({ ...p, left: main.left, x0: main.x0, x1: main.x1 }));
			ctx.restore();
		}
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, 0, MARGIN, TOP_H);
		ctx.beginPath();
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		ctx.moveTo(0, TOP_H - 0.5);
		ctx.lineTo(w, TOP_H - 0.5);
		ctx.moveTo(MARGIN - 0.5, 0);
		ctx.lineTo(MARGIN - 0.5, h);
		ctx.stroke();
		// Séparations des volets figés : un trait plus marqué, comme Excel.
		if (fh || fw) {
			ctx.beginPath();
			ctx.strokeStyle = C.frozen;
			if (fh) {
				ctx.moveTo(0, Math.round(TOP_H + fh) - 0.5);
				ctx.lineTo(w, Math.round(TOP_H + fh) - 0.5);
			}
			if (fw) {
				ctx.moveTo(Math.round(MARGIN + fw) - 0.5, 0);
				ctx.lineTo(Math.round(MARGIN + fw) - 0.5, h);
			}
			ctx.stroke();
		}

		if (editing) placeEditor();

		const seen = new Set<string>();
		const pins = pinnedFrame.filter((p) => !seen.has(p.x + ',' + p.y) && !!seen.add(p.x + ',' + p.y));
		if (JSON.stringify(pins) !== JSON.stringify(pinnedShown)) pinnedShown = pins;
		// Une clé plus large que toutes celles vues : gouttière élargie à
		// l'image suivante (les clés ne sont connues qu'une fois chargées).
		if (keyGrew) schedule();
		endAnimFrame();

		const t1 = performance.now();
		pushStat(drawTimes, t1 - t0);
		frameStamps.push(t1);
		while (frameStamps.length && frameStamps[0] < t1 - 1000) frameStamps.shift();
	}

	/** Ce qu'un volet affiche : lignes et colonnes visibles, positions à l'écran. */
	function layer(p: Pane) {
		const w = viewW;
		const h = viewH;
		const { left, top } = p;
		const r0 = rax().indexAt(p.y0 - TOP_H + top);
		const r1 = Math.min(SHEET_ROWS - 1, rax().indexAt(Math.max(p.y0, p.y1 - 1) - TOP_H + top));
		const c0 = cax().indexAt(p.x0 - MARGIN + left);
		const c1 = cax().indexAt(Math.max(p.x0, p.x1 - 1) - MARGIN + left);
		const cx = (c: number) => MARGIN + cax().offset(c) - left;
		const ry = (r: number) => TOP_H + rowTop(r) - top;
		const xEnd = Math.min(w, cx(c1 + 1));
		const spanW = (a: number, b: number) => cax().offset(b + 1) - cax().offset(a);
		const visible = entries.filter((e) => e.last + 1 >= r0 && topRow(e.t) <= r1 && e.lastCol + 1 >= c0 && e.t.col <= c1);
		// Plage sélectionnée.
		const { r0: s0, r1: s1, c0: k0, c1: k1 } = selection();
		return { w, h, top, r0, r1, c0, c1, cx, ry, xEnd, spanW, visible, s0, s1, k0, k1 };
	}
	type Layer = ReturnType<typeof layer>;

	/** Cellules d'un volet (dans sa découpe) ; vrai s'il manque des lignes pas encore chargées. */
	function drawCells(L: Layer): boolean {
		const { w, h, r0, r1, c0, c1, cx, ry, xEnd, spanW, visible, s0, s1, k0, k1 } = L;
		paneTop = L.top;
		let incomplete = false;
		for (const e of visible) {
			const d = visibleData(e, r0, r1);
			if (d) {
				e.cache.want(d.first, d.last);
				if (!e.cache.has(d.first, d.last)) incomplete = true;
			}
		}

		// En-têtes (agrandis à l'écran quand la cellule active les prolonge).
		for (const e of visible) {
			const g = grown(e);
			const moved = enterTable(e, cx, ry);
			ctx.fillStyle = e.t.query ? C.linkHead : C.head;
			// En mouvement : l'en-tête suit le cadre animé (il se rétracte avec lui).
			ctx.fillRect(cx(g.firstCol) + (moved ? tableAnim(e)!.ox : 0), ry(e.t.row), moved ? tableAnim(e)!.w : spanW(g.firstCol, g.lastCol), rh(e.t.row));
			// Colonnes saisies à côté du résultat chargé : en-tête d'un tableau ordinaire (vert).
			if (e.t.query && !moved) {
				ctx.fillStyle = C.head;
				e.t.columns.forEach((k, i) => {
					if (k.hand) ctx.fillRect(cx(e.t.col + i), ry(e.t.row), cax().size(e.t.col + i), rh(e.t.row));
				});
			}
			if (moved) ctx.restore();
		}

		// Plage sélectionnée.
		// Comme Excel, la cellule active (ou sa fusion) reste blanche.
		// Fusion active à cheval sur des lignes entières sélectionnées : seule sa
		// partie sélectionnée est réservée (le reste n'est pas à surligner).
		if (!singleSelection()) {
			const a = intersect(activeRect(), { r0: s0, r1: s1, c0: k0, c1: k1 });
			ctx.fillStyle = C.select;
			ctx.beginPath();
			ctx.rect(cx(k0), ry(s0), spanW(k0, k1), rspan(s0, s1));
			if (a) ctx.rect(cx(a.c0), ry(a.r0), spanW(a.c0, a.c1), rspan(a.r0, a.r1));
			ctx.fill('evenodd');
		}

		// Recherche : toutes les occurrences visibles, en léger (l'active reste encadrée).
		if (finder && matches.length && matchKey === findKey(finder.text, finder.zone)) {
			ctx.fillStyle = C.found;
			for (const m of matches)
				if (m.row >= r0 && m.row <= r1 && m.col >= c0 && m.col <= c1) ctx.fillRect(cx(m.col), ry(m.row), cax().size(m.col), rh(m.row));
		}
		// Cellule trouvée : sa ligne et sa colonne entières (toute la fusion), en
		// croix sous le contenu ; les deux bandes se superposent sur la cellule.
		if (findCross) {
			const f = findCross;
			ctx.fillStyle = C.findCross;
			if (f.r1 >= r0 && f.r0 <= r1) ctx.fillRect(cx(c0), ry(f.r0), xEnd - cx(c0), rspan(f.r0, f.r1));
			if (f.c1 >= c0 && f.c0 <= c1) ctx.fillRect(cx(f.c0), ry(r0), spanW(f.c0, f.c1), ry(r1 + 1) - ry(r0));
		}

		// Lignes masquées par un filtre : elles gardent leur place (les cellules
		// libres dessous ne bougent pas) ; une bande grisée les représente, avec
		// son libellé sur sa première ligne visible.
		for (const e of entries) {
			const b = hiddenBand(e.t);
			if (!b || b.last < r0 || b.first > r1 || e.lastCol < c0 || e.t.col > c1) continue;
			const top = Math.max(b.first, r0);
			const bottom = Math.min(b.last, r1);
			ctx.fillStyle = C.gray;
			ctx.fillRect(cx(e.t.col), ry(top), spanW(e.t.col, e.lastCol), rspan(top, bottom));
			ctx.save();
			ctx.beginPath();
			ctx.rect(cx(e.t.col), ry(top), spanW(e.t.col, e.lastCol), rspan(top, bottom));
			ctx.clip();
			ctx.font = 'italic ' + FONT_SMALL;
			ctx.fillStyle = C.icon;
			ctx.textAlign = 'left';
			ctx.textBaseline = 'middle';
			ctx.fillText(bandLabel(e.t.hidden ?? 0, bandOpen(e.t) ? data?.choice?.name : undefined), cx(e.t.col) + 6, ry(top) + rh(top) / 2);
			ctx.restore();
		}

		// Contenu des tableaux, colonne par colonne (une découpe par colonne).
		ctx.textBaseline = 'middle';
		for (const e of visible) {
			const tr0 = Math.max(r0, e.t.row);
			const tr1 = Math.min(r1, e.last);
			const mv = tableAnim(e);
			const moved = enterTable(e, cx, ry);
			// Colonne calculée : ses valeurs qui changent s'éclairent un instant.
			const calcCols = motion() ? e.t.columns.map((k) => !!k.formula || k.type === 'formula') : null;
			for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol) && tr0 <= tr1; c++) {
				const i = c - e.t.col;
				const col = e.t.columns[i];
				const x = cx(c) + (mv ? mv.colOff(c) : 0);
				const cw = cax().size(c);
				if (!cw) continue; // colonne masquée
				ctx.save();
				ctx.beginPath();
				ctx.rect(x, TOP_H, cw, h - TOP_H);
				ctx.clip();
				for (let r = tr0; r <= tr1; r++) {
					const y = ry(r) + (mv ? mv.rowOff(r) : 0);
					if (!rh(r)) continue; // ligne masquée
					if (r === e.t.row) {
						// Colonne sur un niveau : une cellule sur la bande et l'en-tête ;
						// ses bordures et sa note sont celles de la cellule entière (singleCell).
						const single = singleCol(e, i) && !!rh(r - 1);
						const hs = single ? bandStyle(e, i, i) : styleAt(r, c, e);
						if (hs && !single) borders(hs, x, y, cw, rh(r));
						if (hs?.bg) {
							ctx.fillStyle = hs.bg;
							ctx.fillRect(x, y, cw, rh(r));
						}
						if (hs?.nt && !single) noteCorner(x + cw, y, hs);
						const iconW = headIcons(e, i, x, y, cw, rh(r));
						noteHeadIcons(e, i, iconW, hs);
						// Colonne sur un niveau : son nom s'écrit sur la bande et l'en-tête
						// réunis (drawGroupBands, qui passe après le quadrillage).
						if (single) {
							singleIcons.set(`${e.t.id}:${i}`, iconW);
							continue;
						}
						headLabel(headerLabel(col.name, e.t.twoLevel), x, y, cw, rh(r), iconW + 10, hs, e.t.query && !col.hand ? C.linkHeadText : C.headText, 'left');
						continue;
					}
					const row = e.cache.row(r - e.t.row - 1);
					if (!row) {
						ctx.fillStyle = C.placeholder;
						ctx.fillRect(x + 6, y + 7, cw * 0.6, rh(r) - 14);
						continue;
					}
					const v = row.cells[i];
					const bad = row.bad?.has(i) || (col.type === 'formula' && isErrorValue(v));
					const own = colRules.get(e.t.id)?.[i];
					const cf = cfAt(r, c, { v, text: row.text[i] ?? '', error: !!bad }, own ? { rules: own, loc: { k: 'tbl', t: e.t.id, c: i }, mark: (_, k) => !!row.marks?.has(i * 1000 + k) } : undefined);
					const cs = resolveStyle(styleAt(r, c, e), cf?.style);
					if (cs?.bg) {
						ctx.fillStyle = cs.bg;
						ctx.fillRect(x, y, cw, rh(r));
					}
					if (r === hoverLocked && row.locked) {
						ctx.fillStyle = 'rgba(107, 114, 128, 0.12)';
						ctx.fillRect(x, y, cw, rh(r));
					}
					if (cf?.bar) drawBar(cf.bar, x, y, cw, rh(r), `t${e.t.id}:${row.id}:${col.name}`, r, c);
					if (cs) borders(cs, x, y, cw, rh(r));
					if (cs?.nt) noteCorner(x + cw, y, cs);
					const sk = syncAt(row.sync, i);
					if (sk) syncCorner(x + cw, y + rh(r), sk);
					let s = row.text[i];
					if (calcCols?.[i]) {
						const fk = `t${e.t.id}:${row.id}:${col.name}`;
						if (motion()) columnFlashes(e.t.id, col.name, fk);
						paintFlash(flashOf(fk, s ?? '', r, c), x, y, cw, rh(r));
					}
					if (!s) continue;
					// Jeu d'icônes : à gauche de la cellule ; barre ou icône seule : valeur masquée.
					const iconW = cf?.icon && !row.bad?.has(i) ? drawIconAt(cf.icon, x, y, rh(r)) : 0;
					if (cf?.hide && !row.bad?.has(i)) continue;
					if (cs && !row.bad?.has(i)) s = formatCell(s, row.cells[i], cs);
					if (row.bad?.has(i)) {
						// Saisie hors type : valeur brute en rouge, coin rouge comme le
						// triangle d'Excel.
						ctx.font = FONT;
						ctx.textAlign = 'left';
						ctx.fillStyle = C.error;
						ctx.fillText(s, x + 6, y + rh(r) / 2);
						ctx.beginPath();
						ctx.moveTo(x + cw - 7, y + 1);
						ctx.lineTo(x + cw - 1, y + 1);
						ctx.lineTo(x + cw - 1, y + 7);
						ctx.closePath();
						ctx.fill();
						continue;
					}
					const align = cs?.al ?? (col.type === 'formula' ? valueAlignment(v) : alignment(col.type));
					drawText(s, x + iconW, y, cw - iconW, align, cs, col.type === 'formula' && isErrorValue(v) ? C.error : C.text);
					if (col.type === 'formula' && isErrorValue(v)) errorCorner(x, y);
				}
				ctx.restore();
			}
			if (moved) ctx.restore();
		}

		// Mise en forme hors tableaux (cellules libres, tableaux croisés, cellules
		// vides des lignes et colonnes mises en forme) : remplissage et bordures.
		ctx.font = FONT;
		ctx.fillStyle = C.text;
		// Mise en forme conditionnelle des cellules libres, calculée une fois par dessin.
		const frameCf = new Map<string, CfOut | undefined>();
		const cfOf = (r: number, c: number) => {
			const k = key(r, c);
			if (!frameCf.has(k)) frameCf.set(k, freeCf(r, c));
			return frameCf.get(k);
		};
		const paint = (r: number, c: number) => {
			if (tableAt(r, c) || groupBandAt(r, c) || !rh(r) || !cax().size(c) || mergeOf(r, c)) return;
			const cf = cfOf(r, c);
			const fs = resolveStyle(styleAt(r, c, undefined), cf?.style);
			if (!fs && !cf?.bar) return;
			if (fs?.bg) {
				ctx.fillStyle = fs.bg;
				ctx.fillRect(cx(c), ry(r), cax().size(c), rh(r));
			}
			if (cf?.bar) drawBar(cf.bar, cx(c), ry(r), cax().size(c), rh(r), `f${r},${c}`, r, c);
			if (!fs) return;
			borders(fs, cx(c), ry(r), cax().size(c), rh(r));
			if (fs.nt) noteCorner(cx(c + 1), ry(r), fs);
		};
		// Lignes ou colonnes mises en forme (hors masquage seul) : toutes les cellules visibles.
		const lineFormats = [...sheetRowStyles.values(), ...sheetColStyles.values()].some((s) => Object.keys(s).some((k) => k !== 'hd'));
		if (lineFormats || sheetRowRules.size || sheetColRules.size) {
			for (let r = r0; r <= r1; r++) for (let c = c0; c <= c1; c++) paint(r, c);
		} else
			for (const k of freeStyles.keys()) {
				const [r, c] = k.split(',').map(Number);
				if (r >= r0 && r <= r1 && c >= c0 && c <= c1) paint(r, c);
			}
		// Valeurs déversées : comme des cellules, dans un cadre bleu.
		for (const sp of spills) {
			const sr1 = sp.row + sp.rows.length - 1;
			const sc1 = sp.col + sp.rows[0].length - 1;
			if (sr1 < r0 || sp.row > r1 || sc1 < c0 || sp.col > c1) continue;
			for (let i = Math.max(0, r0 - sp.row); i < sp.rows.length && sp.row + i <= r1; i++)
				sp.rows[i].forEach((v, j) => {
					if ((i === 0 && j === 0) || !v) return;
					const c = sp.col + j;
					const st = styleAt(sp.row + i, c, undefined);
					paintFlash(flashOf(`s${sp.row},${sp.col}:${i},${j}`, v, sp.row + i, c), cx(c), ry(sp.row + i), cax().size(c), rh(sp.row + i));
					drawText(formatCell(v, undefined, st), cx(c), ry(sp.row + i), cax().size(c), st?.al ?? freeDisplay('', v).align, st, isErrorValue(v) ? C.error : C.text);
				});
			ctx.save();
			ctx.strokeStyle = C.spill;
			ctx.lineWidth = 1;
			ctx.strokeRect(Math.round(cx(sp.col)) + 0.5, Math.round(ry(sp.row)) + 0.5, spanW(sp.col, sc1) - 1, rspan(sp.row, sp.row + sp.rows.length - 1) - 1);
			ctx.restore();
		}
		// Tableaux croisés : en-tête et total en gras, dans un cadre.
		for (const p of pivots) {
			if (p.error) {
				drawText('Tableau croisé : ' + p.error, cx(p.col), ry(p.row), cax().size(p.col), 'left', undefined, C.error, w);
				continue;
			}
			const pr1 = p.row + p.cells.length - 1;
			const pc1 = p.col + p.cells[0].length - 1;
			// Filtres de rapport au-dessus du tableau (Excel) : champ, valeur retenue, ▾.
			(p.config.filters ?? []).forEach((f, k) => {
				const r = (p.anchor ?? p.row) + k;
				if (r < r0 || r > r1 || p.col + 1 < c0 || p.col > c1) return;
				const w0 = cax().size(p.col);
				const w1 = cax().size(p.col + 1);
				ctx.fillStyle = C.linkHead; // comme les tableaux d'une requête : des données calculées
				ctx.fillRect(cx(p.col), ry(r), w0, rh(r));
				drawText(f.field, cx(p.col), ry(r), w0, 'left', { b: true }, C.text);
				drawText(pivotFilterText(f.values), cx(p.col + 1), ry(r), w1 - PIVOT_DROP, 'left', undefined, C.text);
				ctx.save();
				ctx.strokeStyle = C.linkBorder;
				ctx.lineWidth = 1;
				ctx.strokeRect(Math.round(cx(p.col)) + 0.5, Math.round(ry(r)) + 0.5, w0 + w1 - 1, rh(r) - 1);
				ctx.restore();
				pivotDrop(cx(p.col + 1) + w1, ry(r), rh(r), f.values.length > 0);
			});
			if (pr1 < r0 || p.row > r1 || pc1 < c0 || p.col > c1) continue;
			// Ligne du total général : la dernière, quand il est affiché.
			const tot = pivotTotalRow(p);
			// En-tête et total sur fond gris, sauf remplissage choisi.
			ctx.fillStyle = C.linkHead; // comme les tableaux d'une requête : des données calculées
			for (const r of tot >= 0 ? [p.row, tot] : [p.row])
				for (let c = p.col; c <= pc1; c++) if (!styleAt(r, c, undefined)?.bg) ctx.fillRect(cx(c), ry(r), cax().size(c), rh(r));
			for (let i = Math.max(0, r0 - p.row); i < p.cells.length && p.row + i <= r1; i++)
				p.cells[i].forEach((v, j) => {
					const c = p.col + j;
					// Arborescence : retrait du niveau et case −/+ des groupes (colonne des étiquettes).
					const info = j === 0 ? pivotRowOf(p, p.row + i) : undefined;
					if (info?.group) pivotFoldBox(cx(c) + pivotBoxLeft(info.depth), ry(p.row + i), rh(p.row + i), !!info.collapsed);
					if (!v) return;
					const bold = pivotBold(p, p.row + i);
					// Mise en forme posée sur les cellules du TCD, leurs lignes ou colonnes (format de nombre, décimales, police…).
					const st = resolveStyle(styleAt(p.row + i, c, undefined), freeCf(p.row + i, c)?.style);
					const style = bold ? { ...st, b: st?.b ?? true } : st;
					const ind = pivotLabelOffset(info);
					// Recalculé : seules les cellules dont la valeur change s'éclairent.
					paintFlash(flashOf(`p${p.id}:${i},${j}`, v, p.row + i, c), cx(c), ry(p.row + i), cax().size(c), rh(p.row + i));
					drawText(pivotValueText(formatCell(v, undefined, st), i, j), cx(c) + ind, ry(p.row + i), cax().size(c) - ind, st?.al ?? (j > 0 && i > 0 ? 'right' : 'left'), style, i === 0 || p.row + i === tot ? C.linkHeadText : C.text);
				});
			// ▾ des étiquettes des lignes : trier et filtrer le champ (Excel) ; en
			// arborescence, un seul ▾ sur « Étiquettes de lignes » (choix du champ dans le menu).
			if (p.row >= r0 && p.row <= r1)
				(p.rows ? (p.config.rowFields ?? []).slice(0, 1) : (p.config.rowFields ?? [])).forEach((f, j) => {
					const c = p.col + j;
					const on = p.rows ? (p.config.rowFields ?? []).some((x) => pivotFiltered(p, x)) : pivotFiltered(p, f);
					if (c >= c0 && c <= c1) pivotDrop(cx(c) + cax().size(c), ry(p.row), rh(p.row), on);
				});
			ctx.save();
			ctx.strokeStyle = C.linkBorder;
			ctx.lineWidth = 1;
			ctx.strokeRect(Math.round(cx(p.col)) + 0.5, Math.round(ry(p.row)) + 0.5, spanW(p.col, pc1) - 1, rspan(p.row, pr1) - 1);
			// Traits sous l'en-tête et au-dessus du total.
			ctx.beginPath();
			for (const y of tot >= 0 ? [ry(p.row + 1), ry(tot)] : [ry(p.row + 1)]) {
				ctx.moveTo(Math.round(cx(p.col)), Math.round(y) + 0.5);
				ctx.lineTo(Math.round(cx(p.col)) + spanW(p.col, pc1), Math.round(y) + 0.5);
			}
			ctx.stroke();
			ctx.restore();
		}
		// Texte aligné à gauche qui déborde (comme Excel) : seulement sur les
		// cellules voisines vides ; il est coupé à la première cellule occupée
		// (valeur, tableau, tableau croisé, fusion).
		const overflowW = (r: number, c: number): number => {
			let ow = cax().size(c);
			for (let cc = c + 1; cc <= c1 + 1; cc++) {
				const v = cells.get(key(r, cc));
				if ((v !== undefined && v !== '') || tableAt(r, cc) || pivotAt(r, cc) || mergeOf(r, cc)) break;
				ow += cax().size(cc);
			}
			return ow;
		};
		for (const [k, rawText] of cells) {
			const [r, c] = k.split(',').map(Number);
			if (r < r0 || r > r1 || c < c0 || c > c1 || !rh(r) || !cax().size(c) || mergeOf(r, c)) continue;
			// Cellule en cours d'édition : l'éditeur montre sa saisie ; sa valeur,
			// qui peut déborder plus loin que lui, ne se dessine pas dessous.
			if (editing && r === active.row && c === active.col) continue;
			const cf = cfOf(r, c);
			const fstyle = resolveStyle(styleAt(r, c, undefined), cf?.style);
			let x = cx(c);
			let cw = cax().size(c);
			const f = shown.get(k);
			if (f) paintFlash(flashOf('f' + k, f.text, r, c), x, ry(r), cw, rh(r));
			if (f?.error) errorCorner(x, ry(r));
			// Jeu d'icônes à gauche ; barre ou icône seule : valeur masquée.
			if (cf?.icon) {
				const iw = drawIconAt(cf.icon, x, ry(r), rh(r));
				x += iw;
				cw -= iw;
			}
			if (cf?.hide) continue;
			// Nombres, dates et heures à droite, booléens centrés, apostrophe de tête masquée ;
			// format de la cellule (nombre, date, heure, texte : numfmt.ts).
			const fd = freeDisplay(rawText, f?.text);
			const raw = fd.kind === 'text' ? fd.text : formatCell(fd.text, undefined, fstyle);
			const boxed = fd.align !== 'left';
			const spread = boxed ? cw : overflowW(r, c) - (x - cx(c));
			if (fstyle || raw.includes('\n')) {
				drawText(raw, x, ry(r), cw, fstyle?.al ?? fd.align, fstyle, f?.error ? C.error : C.text, spread);
				continue;
			}
			ctx.fillStyle = f?.error ? C.error : C.text;
			ctx.save();
			ctx.beginPath();
			ctx.rect(Math.max(x, MARGIN), TOP_H, spread - Math.max(0, MARGIN - x), h - TOP_H);
			ctx.clip();
			ctx.textAlign = fd.align;
			ctx.fillText(raw, fd.align === 'right' ? x + cw - 6 : fd.align === 'center' ? x + cw / 2 : x + 6, ry(r) + rh(r) / 2);
			ctx.restore();
		}

		// Quadrillage de toute la feuille ; masqué, il ne reste qu'à
		// l'intérieur des tableaux.
		ctx.save();
		if (!gridlines) {
			ctx.beginPath();
			for (const e of visible) {
				const g = grown(e);
				ctx.rect(cx(g.firstCol), ry(topRow(e.t)), spanW(g.firstCol, g.lastCol), rspan(topRow(e.t), g.last));
			}
			ctx.clip();
		} else if (pivots.length) {
			// Comme Excel : pas de quadrillage dans un tableau croisé (seulement
			// son cadre et les traits sous l'en-tête et au-dessus du total).
			ctx.beginPath();
			ctx.rect(0, 0, w, h);
			for (const p of pivots) {
				if (p.error) continue;
				const pc1 = p.col + p.cells[0].length - 1;
				ctx.rect(cx(p.col) + 1, ry(p.row) + 1, spanW(p.col, pc1) - 2, rspan(p.row, p.row + p.cells.length - 1) - 2);
			}
			ctx.clip('evenodd');
		}
		// Pas de quadrillage dans les en-têtes des tableaux (bande des groupes
		// comprise) : leurs bordures sont celles du cadre (headerLines), sans
		// liseré pâle à côté quand le zoom ou l'écran tombent entre deux pixels.
		const heads = visible.filter((e) => !tableAnim(e));
		if (heads.length) {
			ctx.beginPath();
			ctx.rect(0, 0, w, h);
			for (const e of heads) ctx.rect(cx(e.t.col), ry(topRow(e.t)), spanW(e.t.col, e.lastCol), ry(e.t.row + 1) - ry(topRow(e.t)));
			ctx.clip('evenodd');
		}
		ctx.beginPath();
		ctx.strokeStyle = C.grid;
		ctx.lineWidth = 1;
		for (let r = r0; r <= r1 + 1; r++) {
			const y = Math.round(ry(r)) - 0.5;
			ctx.moveTo(MARGIN, y);
			ctx.lineTo(xEnd, y);
		}
		for (let c = c0; c <= c1 + 1; c++) {
			const x = Math.round(cx(c)) - 0.5;
			ctx.moveTo(x, TOP_H);
			ctx.lineTo(x, h);
		}
		ctx.stroke();
		ctx.restore();
		drawMerges(L);
		drawGroupBands(L);

		ctx.save();
		ctx.beginPath();
		ctx.rect(MARGIN, TOP_H, w - MARGIN, h - TOP_H);
		ctx.clip();
		// Bordure discrète de chaque tableau, trait sous l'en-tête, bordures des en-têtes.
		for (const e of visible) {
			const g = grown(e);
			const a = tableAnim(e);
			const to = tableGeo(e);
			const fw = a ? a.w : to.w;
			const fh = a ? a.h : to.h;
			if (motion()) geoDrawn.set(e.t.id, { x: to.x + (a?.dx ?? 0) + (a?.ox ?? 0), y: to.y + (a?.dy ?? 0), w: fw, h: fh });
			const x = Math.round(cx(g.firstCol) + (a?.dx ?? 0) + (a?.ox ?? 0)) - 0.5;
			const y = Math.round(ry(topRow(e.t)) + (a?.dy ?? 0)) - 0.5;
			ctx.save();
			if (a) ctx.globalAlpha *= a.alpha;
			// Cadre, trait sous l'en-tête et bordures par défaut des en-têtes
			// (séparations des colonnes, trait sous la bande des groupes) :
			// même couleur et même épaisseur, nets à tout zoom (crispLines).
			const border = e.t.query ? C.linkBorder : C.border;
			const fx = cx(g.firstCol) + (a?.dx ?? 0) + (a?.ox ?? 0);
			const fy = ry(topRow(e.t)) + (a?.dy ?? 0);
			const fu = ry(e.t.row + 1) + (a?.dy ?? 0);
			const head = headerLines(e, L, a);
			const grownOut = g.last !== e.last || g.lastCol !== e.lastCol || g.firstCol !== e.t.col;
			const frame = [fx, fy, fx, fy + fh, fx + fw, fy, fx + fw, fy + fh, fx, fy, fx + fw, fy, fx, fy + fh, fx + fw, fy + fh, fx, fu, fx + fw, fu];
			// Cadre agrandi par la cellule active : en vert foncé, par-dessus les traits des en-têtes.
			ctx.fillStyle = border;
			crispLines(grownOut ? head : [...frame, ...head]);
			if (grownOut) {
				ctx.fillStyle = C.active;
				crispLines(frame);
			}
			// Pastille du tableau qui suit l'onglet à choix : la valeur choisie.
			if (e.t.follows && data?.choice && !data.choice.all) {
				const label = data.choice.value || '(vide)';
				ctx.font = FONT_SMALL;
				const bw = Math.ceil(ctx.measureText(label).width) + 10;
				const bh = 13;
				const bx = x + fw - bw;
				const by = y - bh;
				ctx.fillStyle = C.followBadge;
				ctx.beginPath();
				ctx.roundRect(bx, by, bw, bh, [4, 4, 0, 0]);
				ctx.fill();
				ctx.fillStyle = '#fff';
				ctx.textAlign = 'center';
				ctx.textBaseline = 'middle';
				ctx.fillText(label, bx + bw / 2, by + bh / 2 + 0.5);
				followBadges.push({ e, x0: bx, x1: bx + bw, y0: by, y1: by + bh });
			}
			// Classeur collaboratif : lecture filtrée par les droits (§8.1).
			if (e.t.filtered) {
				ctx.font = 'italic ' + FONT_SMALL;
				ctx.fillStyle = C.icon;
				ctx.textAlign = 'right';
				ctx.textBaseline = 'top';
				ctx.fillText('Vue filtrée par vos droits', x + fw, y + fh + 2);
			}
			ctx.restore();
		}
		// Bordures posées par le ruban : par-dessus le cadre discret des
		// tableaux (un Contour sur un en-tête ou sa bande reste visible).
		strokeBorders();
		// Sauts de page (Affichage › Sauts de page) : pointillés fins, plus
		// marqués pour un saut manuel, et la zone imprimée encadrée.
		if (pageBreaks) {
			const b = pageBreaks;
			const a = b.area;
			ctx.save();
			ctx.strokeStyle = C.pageBreak;
			for (const [manual, dash, lw] of [
				[false, [3, 3], 1],
				[true, [8, 3], 1.5]
			] as const) {
				ctx.beginPath();
				ctx.setLineDash(dash as unknown as number[]);
				ctx.lineWidth = lw;
				for (const r of manual ? b.manualRows : b.rows)
					if (r >= r0 && r <= r1 + 1 && (manual || !b.manualRows.includes(r))) {
						const y = Math.round(ry(r)) - 0.5;
						ctx.moveTo(Math.max(cx(a.c0), MARGIN), y);
						ctx.lineTo(Math.min(cx(a.c1 + 1), xEnd), y);
					}
				for (const c of manual ? b.manualCols : b.cols)
					if (c >= c0 && c <= c1 + 1 && (manual || !b.manualCols.includes(c))) {
						const x = Math.round(cx(c)) - 0.5;
						ctx.moveTo(x, Math.max(ry(a.r0), TOP_H));
						ctx.lineTo(x, Math.min(ry(a.r1 + 1), h));
					}
				ctx.stroke();
			}
			if (b.framed && a.r1 >= r0 && a.r0 <= r1 + 1 && a.c1 >= c0 && a.c0 <= c1 + 1) {
				ctx.setLineDash([3, 3]);
				ctx.lineWidth = 1;
				ctx.strokeRect(Math.round(cx(a.c0)) - 0.5, Math.round(ry(a.r0)) - 0.5, spanW(a.c0, a.c1), rspan(a.r0, a.r1));
			}
			ctx.restore();
		}
		// Zone copiée ou coupée : cadre pointillé qui défile (« fourmis »), comme Excel.
		if (clipRange && clipRange.sheet === sheetId()) {
			const k = clipRange;
			ctx.save();
			ctx.setLineDash([4, 3]);
			if (motion()) {
				ctx.lineDashOffset = -((animNow / 60) % 7);
				animBusy = true;
			}
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 1.5;
			ctx.strokeRect(cx(k.c0) + 1, ry(k.r0) + 1, spanW(k.c0, k.c1) - 2, rspan(k.r0, k.r1) - 2);
			ctx.restore();
		}
				// Déplacement en cours : contour de la destination.
		if (moving?.to) {
			const { r0, c0, r1, c1 } = moving.sel;
			const dr = moving.to.row - moving.from.row;
			const dc = moving.to.col - moving.from.col;
			ctx.save();
			ctx.setLineDash([5, 3]);
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 2;
			ctx.strokeRect(cx(c0 + dc) + 1, ry(r0 + dr) + 1, spanW(c0 + dc, c1 + dc) - 2, rspan(r0 + dr, r1 + dr) - 2);
			ctx.restore();
		}
		// Tableau actif qui ne touche pas la gouttière : ses numéros de ligne (ou
		// sa clé) en petit, juste à gauche de sa première colonne, là où la
		// cellule voisine est vide.
		const act = a1 ? undefined : (ownerAt(active.row, active.col) ?? entries.find((e) => grown(e).last !== e.last));
		if (act && besideGutter({ row: act.t.row, last: act.last, col: act.t.col, lastCol: act.lastCol }, c0, c1)) {
			const x = cx(act.t.col);
			const gw = cax().size(act.t.col - 1);
			ctx.save();
			ctx.beginPath();
			ctx.rect(Math.max(MARGIN, x - gw), TOP_H, Math.min(gw, x - MARGIN), h - TOP_H);
			ctx.clip();
			ctx.font = FONT_TINY;
			ctx.fillStyle = C.stripText;
			ctx.textAlign = 'right';
			ctx.textBaseline = 'middle';
			const last = grown(act).last;
			for (let r = Math.max(r0, act.t.row + 1); r <= Math.min(r1, last); r++) {
				if (tableAt(r, act.t.col - 1) || cells.has(key(r, act.t.col - 1))) continue;
				ctx.fillText(rowLabel(act, r), x - 3, ry(r) + rh(r) / 2);
			}
			ctx.restore();
		}
		// Colonne calculée en cours d'édition, ou cible d'une modification
		// de colonne (scopeTints) : toute sa portée teintée.
		for (const t of scopeTints) {
			const a = Math.max(t.r0, r0);
			const b = Math.min(t.r1, r1);
			if (a <= b && t.c0 <= t.c1) {
				ctx.fillStyle = C.calc;
				ctx.fillRect(cx(t.c0), ry(a), spanW(t.c0, t.c1), rspan(a, b));
			}
		}
		// Saisie d'une formule : chaque référence encadrée de sa couleur, comme
		// dans l'éditeur (Excel).
		if (formulaMode) {
			const { tables, at } = refContext();
			ctx.save();
			ctx.lineWidth = 2;
			// Référence glissée : son cadre coloré suit la souris, la zone
			// d'origine garde le même cadre en gris (choix de Louis).
			const box = (k: { r0: number; r1: number; c0: number; c1: number }, color: string) => {
				if (k.r1 < r0 - 1 || k.r0 > r1 + 1) return;
				ctx.strokeStyle = color;
				ctx.fillStyle = color + '14';
				const x = cx(k.c0) + 1;
				const y = ry(k.r0) + 1;
				ctx.fillRect(x, y, spanW(k.c0, k.c1) - 2, rspan(k.r0, k.r1) - 2);
				ctx.strokeRect(x, y, spanW(k.c0, k.c1) - 2, rspan(k.r0, k.r1) - 2);
			};
			for (const f of formulaRefs(editing ? editValueText : fxText)) {
				const k = refRect(f.text, tables, sheetName, at, !!hold);
				if (!k) continue;
				if (refMove && f.start === refMove.start) {
					const dr = refMove.to.row - refMove.from.row;
					const dc = refMove.to.col - refMove.from.col;
					const moved = { r0: k.r0 + dr, r1: k.r1 + dr, c0: k.c0 + dc, c1: k.c1 + dc };
					if (dr || dc) box(k, '#9ca3af');
					if (moved.r0 >= 0 && moved.c0 >= 0) box(moved, f.color);
					continue;
				}
				box(k, f.color);
			}
			ctx.restore();
		}
		// Cellule active (toute la fusion quand elle en est une), et la flèche de
		// sa liste déroulante (validation).
		// Plage de plusieurs cellules : cadre épais autour d'elle, trait fin
		// autour de la cellule active (Excel).
		const ar = activeRect();
		const many = !singleSelection();
		// Cadre extérieur (animé) : la plage, ou la cellule active seule.
		ctx.strokeStyle = C.active;
		ctx.lineWidth = 2;
		// Feuille protégée aussi : on y sélectionne pour lire, copier ou figer les volets.
		ctx.strokeRect(cx(0) + selBox.x + 0.5, ry(0) + selBox.y + 0.5, selBox.w - 1, selBox.h - 1);
		// Rechercher dans la sélection : la cellule trouvée, encadrée (la sélection ne bouge pas).
		if (finder?.zone && finder.at && inZone(finder.at, finder.zone)) {
			const f = finder.at;
			ctx.lineWidth = 2;
			ctx.strokeRect(cx(f.col) + 2, ry(f.row) + 2, cax().size(f.col) - 4, rh(f.row) - 4);
		}
		if (!locked && ar.r1 >= r0 - 1 && ar.r0 <= r1 + 1) {
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 1;
			if (many) ctx.strokeRect(cx(ar.c0) + 1.5, ry(ar.r0) + 1.5, spanW(ar.c0, ar.c1) - 3, rspan(ar.r0, ar.r1) - 3);
			if (!editing && listOf(active.row, active.col)) {
				const b = dropButton();
				ctx.fillStyle = C.strip;
				ctx.fillRect(b.x, b.y, b.w, b.h);
				ctx.strokeStyle = C.stripLine;
				ctx.lineWidth = 1;
				ctx.strokeRect(b.x + 0.5, b.y + 0.5, b.w - 1, b.h - 1);
				ctx.fillStyle = C.stripText;
				ctx.beginPath();
				ctx.moveTo(b.x + 5, b.y + b.h / 2 - 2);
				ctx.lineTo(b.x + b.w - 5, b.y + b.h / 2 - 2);
				ctx.lineTo(b.x + b.w / 2, b.y + b.h / 2 + 3);
				ctx.closePath();
				ctx.fill();
			}
			tableHintBox = null;
			if (!editing && !many && asTableHint()) {
				// Liste ou bloc resté en cellules libres : petite icône à côté de la
				// cellule, un clic en fait un tableau (comme la plage convertie).
				const b = dropButton();
				ctx.fillStyle = C.strip;
				ctx.fillRect(b.x, b.y, b.w + 2, b.h);
				ctx.strokeStyle = C.stripLine;
				ctx.lineWidth = 1;
				ctx.strokeRect(b.x + 0.5, b.y + 0.5, b.w + 1, b.h - 1);
				const s = Math.min(14, b.h - 2);
				// Petit tableau dessiné à la main (l'icône « table » a un <rect>,
				// que drawIcon refuse) : cadre, ligne d'en-tête pleine, quadrillage.
				const ix = Math.round(b.x + (b.w + 2 - s) / 2) + 0.5;
				const iy = Math.round(b.y + (b.h - s) / 2) + 0.5;
				ctx.fillStyle = C.icon;
				ctx.fillRect(ix, iy, s - 1, Math.round(s / 3));
				ctx.strokeStyle = C.icon;
				ctx.strokeRect(ix, iy, s - 1, s - 1);
				ctx.beginPath();
				ctx.moveTo(ix, iy + Math.round((2 * s) / 3));
				ctx.lineTo(ix + s - 1, iy + Math.round((2 * s) / 3));
				ctx.moveTo(ix + Math.round(s / 2), iy);
				ctx.lineTo(ix + Math.round(s / 2), iy + s - 1);
				ctx.stroke();
				tableHintBox = { x: b.x, y: b.y, w: b.w + 2, h: b.h };
			}
		}
		// Flèches d'audit : bleues vers la cellule (antécédents), rouges depuis
		// elle (dépendants), comme Excel.
		// Une même flèche (antécédent de l'une, dépendant de l'autre, toutes deux
		// sélectionnées) n'est dessinée qu'une fois.
		const drawn = new Set<string>();
		const geo: ArrowGeometry = { x: cx, w: (c) => cax().size(c), y: ry, h: rspan };
		for (const arrows of links) {
			const me = spanPoint({ row: arrows.row, rows: 1, col: arrows.col }, geo);
			const draw = (from: { x: number; y: number }, to: { x: number; y: number }, color: string) => {
				ctx.save();
				ctx.strokeStyle = color;
				ctx.fillStyle = color;
				ctx.lineWidth = 1.5;
				ctx.beginPath();
				ctx.arc(from.x, from.y, 3, 0, Math.PI * 2);
				ctx.fill();
				ctx.beginPath();
				ctx.moveTo(from.x, from.y);
				ctx.lineTo(to.x, to.y);
				ctx.stroke();
				const a = Math.atan2(to.y - from.y, to.x - from.x);
				ctx.beginPath();
				ctx.moveTo(to.x, to.y);
				ctx.lineTo(to.x - 9 * Math.cos(a - 0.4), to.y - 9 * Math.sin(a - 0.4));
				ctx.lineTo(to.x - 9 * Math.cos(a + 0.4), to.y - 9 * Math.sin(a + 0.4));
				ctx.closePath();
				ctx.fill();
				ctx.restore();
			};
			// Colonne du même tableau, depuis une ligne de données : la flèche part de
			// la cellule de cette ligne ([Quantité]*[Prix unitaire]), pas de l'en-tête ;
			// une ligne précise (Tableau[Col][1], [Col][-1], REFERENCE) part de sa
			// cellule, une plage (Tableau[Col][2:4]) de son cadre (auditarrows.ts).
			const own = tableAt(arrows.row, arrows.col);
			const ownT = own ? { name: own.t.name, row: own.t.row } : null;
			const once = (k: string) => !drawn.has(k) && !!drawn.add(k);
			const frame = (s: ReturnType<typeof arrowSpan>, color: string) => {
				if (s.rows <= 1) return;
				const b = spanBox(s, geo);
				ctx.save();
				ctx.strokeStyle = color;
				ctx.lineWidth = 1.5;
				ctx.strokeRect(b.x + 1.5, b.y + 1.5, b.w - 3, b.h - 3);
				ctx.restore();
			};
			for (const p of arrows.pre) {
				const s = arrowSpan(p, arrows, ownT);
				if (p.sheet === sheetIdOf() && once(`${s.row}+${s.rows},${s.col}>${arrows.row},${arrows.col}`)) {
					frame(s, '#2b6cb0');
					draw(spanPoint(s, geo), me, '#2b6cb0');
				}
			}
			for (const d of arrows.dep) {
				const s = arrowSpan(d, arrows, ownT);
				if (d.sheet === sheetIdOf() && once(`${arrows.row},${arrows.col}>${s.row}+${s.rows},${s.col}`)) {
					frame(s, '#c0392b');
					draw(me, spanPoint(s, geo), '#c0392b');
				}
			}
		}
		// Saisie dans la barre de formule : la cellule active l'affiche en direct
		// (pas une formule d'une autre feuille, en cours de pointage ici).
		if (fxEditing && !editing && !hold) {
			const x = cx(ar.c0);
			const y = ry(ar.r0);
			const cw = spanW(ar.c0, ar.c1);
			const ch = rspan(ar.r0, ar.r1);
			ctx.fillStyle = C.bg;
			ctx.fillRect(x + 1, y + 1, cw - 2, ch - 2);
			drawText(fxText, x, y, cw, 'left', undefined, C.text, w, ch);
		}
		// Poignée de recopie (coin bas droit de la sélection) et aperçu.
		if (!editing && selChart === null) {
			const h = fillHandle();
			ctx.fillStyle = C.active;
			ctx.fillRect(h.x - 3, h.y - 3, 6, 6);
			ctx.fillStyle = C.bg;
			ctx.fillRect(h.x - 3, h.y + 3, 6, 1);
			ctx.fillRect(h.x + 3, h.y - 3, 1, 7);
		}
		if (filling?.to) {
			const t = filling.to;
			ctx.save();
			ctx.setLineDash([3, 2]);
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 1.5;
			ctx.strokeRect(cx(t.c0) + 1, ry(t.r0) + 1, spanW(t.c0, t.c1) - 2, rspan(t.r0, t.r1) - 2);
			ctx.restore();
		}
		ctx.restore();
		return incomplete;
	}

	/** Bandeau du haut d'un volet de colonnes. */
	function drawTopStrip(L: Layer) {
		const { w, r0, r1, c0, c1, cx, ry, visible, k0, k1 } = L;
		// Bandeau du haut : vide (pas de lettres de colonnes), sauf quand
		// l'en-tête d'un tableau visible est sorti de l'écran.
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, 0, w, TOP_H);
		// Colonnes de la sélection : même surbrillance que les lignes dans la marge.
		ctx.fillStyle = C.activeStrip;
		for (let c = Math.max(c0, k0); c <= Math.min(c1, k1); c++) {
			ctx.fillRect(Math.max(cx(c), MARGIN), 0, cx(c + 1) - Math.max(cx(c), MARGIN), TOP_H);
		}
		// Colonne de la cellule trouvée (Ctrl+F).
		if (findCross && findCross.c1 >= c0 && findCross.c0 <= c1) {
			ctx.fillStyle = C.findGutter;
			const x = Math.max(cx(findCross.c0), MARGIN);
			ctx.fillRect(x, 0, cx(findCross.c1 + 1) - x, TOP_H);
		}
		if (a1) {
			ctx.font = FONT_SMALL;
			ctx.textAlign = 'center';
			for (let c = c0; c <= c1; c++) {
				const x = cx(c);
				if (x + cax().size(c) <= MARGIN || !cax().size(c)) continue;
				ctx.fillStyle = c >= k0 && c <= k1 ? C.headText : C.stripText;
				ctx.fillText(addrA1(0, c).slice(0, -1), x + cax().size(c) / 2, TOP_H / 2);
			}
		}
		for (const e of a1 ? [] : visible) {
			// En-tête encore visible (ou dans les lignes figées) : pas de rappel.
			if (e.t.row >= r0 || e.last < r0 || e.t.row < fRows()) continue;
			// Groupes d'en-têtes : en petit au-dessus des sous-en-têtes, sur toutes leurs colonnes.
			const gh = e.t.twoLevel ? 10 : 0;
			for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol); c++) {
				const x = cx(c);
				ctx.save();
				ctx.beginPath();
				ctx.rect(Math.max(x, MARGIN), 0, cax().size(c), TOP_H);
				ctx.clip();
				const linked = !!e.t.query && !e.t.columns[c - e.t.col]?.hand;
				ctx.fillStyle = c >= k0 && c <= k1 ? C.activeStrip : linked ? C.linkHead : C.head;
				ctx.fillRect(x, 0, cax().size(c), TOP_H);
				// Boutons ▾, type et entonnoir, cliquables comme dans l'en-tête.
				const iw = headIcons(e, c - e.t.col, x, gh, cax().size(c), TOP_H - gh);
				ctx.beginPath();
				ctx.rect(Math.max(x, MARGIN), 0, Math.max(0, cax().size(c) - iw - 10), TOP_H);
				ctx.clip();
				ctx.font = gh ? '600 11px "Segoe UI", system-ui, -apple-system, sans-serif' : FONT_BOLD;
				ctx.fillStyle = linked ? C.linkHeadText : C.headText;
				ctx.textAlign = 'left';
				ctx.fillText(headerLabel(e.t.columns[c - e.t.col].name, e.t.twoLevel), x + 6, (gh + TOP_H) / 2);
				ctx.restore();
			}
			if (!gh) continue;
			ctx.save();
			ctx.beginPath();
			ctx.rect(MARGIN, 0, w - MARGIN, gh);
			ctx.clip();
			for (const s of groupSpans(e.t.columns.map((k) => k.name))) {
				const a = e.t.col + s.i0;
				const b = e.t.col + s.i1;
				if (b < c0 || a > c1) continue;
				const x = cx(a);
				const sw = cax().offset(b + 1) - cax().offset(a);
				ctx.strokeStyle = e.t.query ? C.linkBorder : C.border;
				ctx.lineWidth = 1;
				ctx.beginPath();
				ctx.moveTo(Math.round(x) - 0.5, 0);
				ctx.lineTo(Math.round(x) - 0.5, gh);
				ctx.moveTo(x, gh - 0.5);
				ctx.lineTo(x + sw, gh - 0.5);
				ctx.stroke();
				if (!s.group) continue;
				ctx.font = '600 ' + FONT_TINY;
				ctx.fillStyle = e.t.query ? C.linkHeadText : C.headText;
				ctx.textAlign = 'center';
				ctx.fillText(ellipsize(s.group, sw - 8), Math.max(MARGIN, x) / 2 + Math.min(x + sw, w) / 2, gh / 2 + 1);
			}
			ctx.restore();
		}
		// Petits traits aux séparations de colonnes : on voit où redimensionner.
		ctx.beginPath();
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		for (let c = c0; c <= c1; c++) {
			const x = Math.round(cx(c + 1)) - 0.5;
			if (x <= MARGIN) continue;
			ctx.moveTo(x, TOP_H * 0.75);
			ctx.lineTo(x, TOP_H);
		}
		ctx.stroke();
		for (let c = Math.max(c0, k0); c <= Math.min(c1, k1); c++) {
			ctx.fillStyle = C.active;
			ctx.fillRect(cx(c), TOP_H - 2, cax().size(c), 2);
		}
		// Colonnes masquées : même repère, un clic les réaffiche.
		for (const [i0, i1] of hiddenRuns(styleHiddenCols)) {
			if (i1 < c0 - 1 || i0 > c1 + 1) continue;
			const x = Math.round(cx(i0));
			if (x <= MARGIN) continue;
			hiddenMark({ rows: false, i0, i1, x: x - 4, y: 3, w: 8, h: TOP_H - 6 });
		}
	}

	/** Marge de gauche d'un volet de lignes. */
	function drawMargin(L: Layer) {
		const { h, r0, r1, ry, visible, s0, s1 } = L;
		// Première colonne affichée (la gouttière) : la première figée s'il y en a.
		const c0 = fCols() > 0 ? 0 : L.c0;
		// Marge de gauche : numéros 1..n des lignes de données des tableaux.
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, TOP_H, MARGIN, h - TOP_H);
		ctx.font = FONT_SMALL;
		ctx.textAlign = 'right';
		// Numéros : ceux du tableau de la sélection (ou de celui qu'elle
		// prolonge) ; ceux des autres tableaux, plus discrets.
		const cur = ownerAt(active.row, active.col) ?? entries.find((e) => grown(e).last !== e.last);
		const curLast = cur ? grown(cur).last : -1;
		const blocks = visible.map((e) => ({ row: e.t.row, last: e.last, col: e.t.col, lastCol: e.lastCol, e }));
		for (let r = r0; r <= r1; r++) {
			const y = ry(r);
			if (!rh(r)) continue; // ligne masquée
			if (r >= s0 && r <= s1) {
				ctx.fillStyle = C.activeStrip;
				ctx.fillRect(0, y, MARGIN, rh(r));
				ctx.fillStyle = C.active;
				ctx.fillRect(MARGIN - 2, y, 2, rh(r));
			}
			// Ligne de la cellule trouvée (Ctrl+F).
			if (findCross && r >= findCross.r0 && r <= findCross.r1) {
				ctx.fillStyle = C.findGutter;
				ctx.fillRect(0, y, MARGIN - 2, rh(r));
			}
			if (a1) {
				ctx.fillStyle = r >= s0 && r <= s1 ? C.headText : C.stripText;
				ctx.fillText(String(r + 1), MARGIN - 8, y + rh(r) / 2);
				continue;
			}
			// Seul le tableau qui touche la gouttière (première colonne affichée)
			// y est numéroté : pas de numéros en face de lignes sans rapport.
			const mine = !!cur && r > cur.t.row && r <= curLast && cur.t.col <= c0 && cur.lastCol >= c0;
			const e = mine ? cur : gutterOwner(blocks, r, c0)?.e;
			if (e) {
				ctx.fillStyle = mine || !cur ? C.stripText : C.stripLine;
				const label = rowLabel(e, r);
				if (e.t.key !== undefined) {
					const kw = ctx.measureText(label).width;
					if (kw > (keyWidths.get(e.t.name) ?? 0)) {
						keyWidths.set(e.t.name, kw);
						keyGrew = true;
					}
				}
				ctx.save();
				ctx.beginPath();
				ctx.rect(0, y, MARGIN - 4, rh(r));
				ctx.clip();
				ctx.fillText(label, MARGIN - 8, y + rh(r) / 2);
				ctx.restore();
			}
		}
		// Petits traits aux séparations de lignes (un quart de la marge, à droite) :
		// on voit où redimensionner, comme pour les colonnes.
		ctx.beginPath();
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		for (let r = r0; r <= r1; r++) {
			const y = Math.round(ry(r + 1)) - 0.5;
			if (y <= TOP_H || !rh(r)) continue;
			ctx.moveTo(MARGIN * 0.75, y);
			ctx.lineTo(MARGIN, y);
		}
		ctx.stroke();
		// Lignes masquées : un repère à leur place (on s'en souvient), un clic
		// les réaffiche.
		for (const [i0, i1] of hiddenRuns(hiddenRows)) {
			if (i1 < r0 - 1 || i0 > r1 + 1) continue;
			const y = Math.round(ry(i0));
			if (y <= TOP_H) continue;
			const m = { rows: true, i0, i1, x: 2, y: y - 4, w: MARGIN - 4, h: 8 };
			hiddenMark(m);
		}
	}

	/**
	 * Cellules fusionnées d'un volet, par-dessus le quadrillage : remplissage,
	 * texte sur toute la fusion, bordures et note.
	 */
	function drawMerges(L: Layer) {
		const { r0, r1, c0, c1, cx, ry, spanW, s0, s1, k0, k1 } = L;
		const many = !singleSelection();
		for (const g of merges) {
			const gr1 = g.row + g.rows - 1;
			const gc1 = g.col + g.cols - 1;
			if (gr1 < r0 || g.row > r1 || gc1 < c0 || g.col > c1) continue;
			const k = key(g.row, g.col);
			const st = resolveStyle(styleAt(g.row, g.col, undefined), freeCf(g.row, g.col)?.style);
			const x = cx(g.col);
			const y = ry(g.row);
			const gw = spanW(g.col, gc1);
			const gh = rspan(g.row, gr1);
			if (gw <= 0 || gh <= 0) continue;
			// Le fond couvre le quadrillage à l'intérieur de la fusion seulement.
			ctx.fillStyle = st?.bg ?? C.bg;
			ctx.fillRect(Math.round(x), Math.round(y), gw - 1, gh - 1);
			// Dans la sélection (sauf la fusion active) : surlignée comme les autres
			// cellules ; traversée par des lignes ou colonnes entières, sur leur
			// partie seulement (comme Excel).
			const isActive = active.row === g.row && active.col === g.col;
			const part = many && !isActive ? intersect({ r0: g.row, r1: gr1, c0: g.col, c1: gc1 }, { r0: s0, r1: s1, c0: k0, c1: k1 }) : null;
			if (part) {
				const px = cx(part.c0);
				const py = ry(part.r0);
				ctx.fillStyle = C.select;
				ctx.fillRect(Math.round(px), Math.round(py), spanW(part.c0, part.c1) - (part.c1 === gc1 ? 1 : 0), rspan(part.r0, part.r1) - (part.r1 === gr1 ? 1 : 0));
			}
			const f = shown.get(k);
			const raw = f ? f.text : cells.get(k);
			if (raw) {
				const num = isNumeric(raw);
				drawText(formatCell(raw, undefined, st), x, y, gw, st?.al ?? (num ? 'right' : 'left'), st, f?.error ? C.error : C.text, gw, gh);
			}
			if (st) borders(st, x, y, gw, gh);
			if (st?.nt) noteCorner(x + gw, y, st);
		}
	}

	/**
	 * Bandes des groupes des tableaux aux en-têtes sur deux lignes, par-dessus
	 * le quadrillage : une cellule par groupe, fusionnée sur ses colonnes,
	 * centrée et en gras comme l'en-tête (vide pour une colonne sans groupe).
	 */
	function drawGroupBands(L: Layer) {
		const { r0, r1, c0, c1, cx, ry, spanW, s0, s1, k0, k1 } = L;
		const many = !singleSelection();
		const ar = activeRect();
		for (const e of L.visible) {
			const r = e.t.row - 1;
			if (!e.t.twoLevel || r > r1 || e.t.row < r0 || !rh(r)) continue;
			const y = ry(r);
			for (const s of groupSpans(e.t.columns.map((k) => k.name))) {
				const a = e.t.col + s.i0;
				const b = e.t.col + s.i1;
				if (b < c0 || a > c1) continue;
				const x = cx(a);
				const sw = spanW(a, b);
				if (sw <= 0) continue;
				if (s.i0 === s.i1 && singleCol(e, s.i0)) {
					singleCell(e, s.i0, x, y, sw, many, ar, L);
					continue;
				}
				if (r < r0) continue;
				// Aspect d'un en-tête (fond, quadrillage, gras), sous la mise en
				// forme posée par le ruban sur ses cellules (fond, bordures, police…).
				const st = bandStyle(e, s.i0, s.i1);
				ctx.fillStyle = st?.bg ?? (e.t.query ? C.linkHead : C.head);
				ctx.fillRect(x, y, sw, rh(r));
				// Dans la sélection (sauf la cellule active) : surlignée comme l'en-tête.
				if (many && r >= s0 && r <= s1 && b >= k0 && a <= k1 && !(ar.r0 === r && ar.c0 === a)) {
					ctx.fillStyle = C.select;
					ctx.fillRect(cx(Math.max(a, k0)), y, spanW(Math.max(a, k0), Math.min(b, k1)), rh(r));
				}
				// Ses bordures par défaut se tracent avec le cadre du tableau (headerLines).
				if (st) borders(st, x, y, sw, rh(r));
				if (st?.nt) noteCorner(x + sw, y, st);
				if (s.group) headLabel(s.group, x, y, sw, rh(r), 6, st, e.t.query ? C.linkHeadText : C.headText, 'center');
			}
		}
	}

	/** Largeur des boutons de l'en-tête des colonnes sur un niveau (posée par drawCells, lue par singleCell). */
	const singleIcons = new Map<string, number>();

	/**
	 * Colonne sur un niveau d'un tableau à deux lignes : une cellule sur la
	 * bande et l'en-tête, avec le nom de la colonne comme un en-tête (en gras,
	 * à gauche, centré en hauteur) ; les boutons restent dans l'en-tête.
	 */
	function singleCell(e: Entry, i: number, x: number, y: number, w: number, many: boolean, ar: { r0: number; c0: number }, L: Layer) {
		const c = e.t.col + i;
		const hr = e.t.row;
		const h2 = rh(hr - 1) + rh(hr);
		const hs = bandStyle(e, i, i);
		ctx.fillStyle = e.t.query ? C.linkHead : C.head;
		ctx.fillRect(x, y, w, rh(hr - 1));
		if (hs?.bg) {
			ctx.fillStyle = hs.bg;
			ctx.fillRect(x, y, w, rh(hr - 1));
		}
		// Partie de la bande : surlignée quand sa ligne est dans la sélection (la
		// ligne d'en-tête seule, choisie dans la gouttière, ne la prend pas).
		if (many && L.s0 <= hr - 1 && L.s1 >= hr - 1 && c >= L.k0 && c <= L.k1 && !(ar.r0 === hr - 1 && ar.c0 === c)) {
			ctx.fillStyle = C.select;
			ctx.fillRect(x, y, w, rh(hr - 1));
		}
		// Ses bordures par défaut se tracent avec le cadre du tableau (headerLines).
		if (hs) borders(hs, x, y, w, h2);
		if (hs?.nt) noteCorner(x + w, y, hs);
		const iw = singleIcons.get(`${e.t.id}:${i}`) ?? 0;
		headLabel(e.t.columns[i].name, x, y, w, h2, iw + 10, hs, e.t.query ? C.linkHeadText : C.headText, 'left');
	}

	/**
	 * Bordures par défaut des en-têtes d'un tableau, comme son cadre : une
	 * séparation entre chaque colonne de l'en-tête, et sur un tableau aux
	 * en-têtes sur deux lignes, une entre les cellules de la bande des groupes
	 * et un trait entre la bande et l'en-tête (sauf sous une colonne sur un
	 * niveau, d'un seul tenant). Segments [x0, y0, x1, y1] posés sur les
	 * limites des cellules, pour crispLines ; le cadre et le trait sous
	 * l'en-tête sont à part. La mise en forme du ruban se trace par-dessus.
	 */
	function headerLines(e: Entry, L: Layer, a: TableAnim | null): number[] {
		const out: number[] = [];
		const hr = e.t.row;
		const X = (c: number) => L.cx(c) + (a ? a.dx + a.colOff(c) : 0);
		const Y = (r: number) => L.ry(r) + (a ? a.dy + a.rowOff(r) : 0);
		// Colonnes visibles seulement (une limite de plus de chaque côté).
		const from = Math.max(e.t.col + 1, L.c0);
		const to = Math.min(e.lastCol, L.c1 + 1);
		if (rh(hr) && hr >= L.r0 && hr <= L.r1) for (let c = from; c <= to; c++) out.push(X(c), Y(hr), X(c), Y(hr + 1));
		const br = hr - 1;
		if (!e.t.twoLevel || !rh(br) || br > L.r1 || hr < L.r0) return out;
		for (const s of groupSpans(e.t.columns.map((k) => k.name))) {
			const c0 = e.t.col + s.i0;
			const c1 = e.t.col + s.i1;
			if (c1 + 1 < L.c0 || c0 > L.c1 + 1) continue;
			if (s.i0 > 0) out.push(X(c0), Y(br), X(c0), Y(hr));
			if (!(s.i0 === s.i1 && singleCol(e, s.i0))) out.push(X(c0), Y(hr), X(c1 + 1), Y(hr));
		}
		return out;
	}

	/**
	 * Traits nets d'un pixel de l'écran (deux à partir de 150 % : comme un
	 * trait de 1 px agrandi), de la couleur de remplissage courante : chaque
	 * segment [x0, y0, x1, y1] (vertical ou horizontal, posé sur une limite
	 * de cellules) devient un rectangle aligné sur les pixels de l'écran, juste
	 * avant la limite (à gauche ou au-dessus), comme le quadrillage, quels que
	 * soient le zoom et la densité de l'écran. Tous les traits se remplissent
	 * d'un coup : leurs croisements ne foncent pas et ne laissent pas de trou.
	 */
	function crispLines(l: number[]) {
		if (!l.length) return;
		const m = ctx.getTransform();
		const lw = Math.max(1, Math.round(m.a));
		const px = (v: number) => Math.round(v * m.a + m.e);
		const py = (v: number) => Math.round(v * m.d + m.f);
		ctx.save();
		ctx.setTransform(1, 0, 0, 1, 0, 0);
		ctx.beginPath();
		for (let i = 0; i < l.length; i += 4) {
			const x0 = px(Math.min(l[i], l[i + 2]));
			const x1 = px(Math.max(l[i], l[i + 2]));
			const y0 = py(Math.min(l[i + 1], l[i + 3]));
			const y1 = py(Math.max(l[i + 1], l[i + 3]));
			ctx.rect(x0 - lw, y0 - lw, x1 - x0 + lw, y1 - y0 + lw);
		}
		ctx.fill();
		ctx.restore();
	}

	/**
	 * Nom d'un en-tête de tableau ou d'un groupe de la bande, en gras, entre
	 * 6 px à gauche et right px à droite (ses boutons) : raccourci avec « … »,
	 * ou renvoyé à la ligne (wr) sur la hauteur de l'en-tête (grandie par
	 * measureHeader, measureBand). Couleur, italique, souligné, barré et
	 * alignements de la mise en forme l'emportent sur l'aspect par défaut.
	 */
	function headLabel(text: string, x: number, y: number, w: number, h: number, right: number, hs: CellStyle | undefined, color: string, align: 'left' | 'center') {
		const avail = Math.max(8, w - 6 - right);
		ctx.save();
		ctx.beginPath();
		ctx.rect(x, y, Math.max(0, w - right), h);
		ctx.clip();
		ctx.font = (hs?.i ? 'italic ' : '') + FONT_BOLD;
		ctx.fillStyle = hs?.fg ?? color;
		const al = hs?.al ?? align;
		ctx.textAlign = al;
		ctx.textBaseline = 'middle';
		const lines = hs?.wr ? wrapLines(text, avail, (t) => ctx.measureText(t).width) : [ellipsize(text, w - 6 - right)];
		const lh = lineHeight(BASE_PX);
		const total = lines.length * lh;
		const top = hs?.va === 'top' ? y + 2 : hs?.va === 'bottom' ? y + h - 2 - total : y + (h - total) / 2;
		const tx = al === 'left' ? x + 6 : al === 'right' ? x + 6 + avail : x + 6 + avail / 2;
		lines.forEach((line, k) => {
			const cy = top + lh * k + lh / 2;
			ctx.fillText(line, tx, cy);
			if (hs?.u || hs?.x) {
				const lw = ctx.measureText(line).width;
				const lx = al === 'left' ? tx : al === 'right' ? tx - lw : tx - lw / 2;
				if (hs.u) ctx.fillRect(lx, cy + 7, lw, 1);
				if (hs.x) ctx.fillRect(lx, cy, lw, 1);
			}
		});
		ctx.restore();
	}

	/** Boutons d'un en-tête renvoyé à la ligne plus larges ou plus étroits qu'à sa mesure : l'en-tête se remesure. */
	function noteHeadIcons(e: Entry, i: number, iconW: number, hs: CellStyle | undefined) {
		const k = `${e.t.id}:${i}`;
		const before = headIconW.get(k);
		headIconW.set(k, iconW);
		if (hs?.wr && Math.abs((before ?? HEAD_ICON_W) - iconW) > 0.5) {
			autoRows.forget(e.t.row, 1);
			schedule();
		}
	}

	/** Coin rouge d'une cellule qui a une note (en haut à droite, comme Excel). */
	function noteCorner(xr: number, y: number, st?: { nt?: string; np?: boolean }) {
		if (st?.np && st.nt) pinnedFrame.push({ x: xr + 6, y, text: st.nt });
		ctx.fillStyle = C.note;
		ctx.beginPath();
		ctx.moveTo(xr - 7, y + 1);
		ctx.lineTo(xr - 1, y + 1);
		ctx.lineTo(xr - 1, y + 7);
		ctx.closePath();
		ctx.fill();
	}

	// Bouton ▾ d'un TCD (filtre de rapport, étiquette des lignes) : largeur réservée à droite de la cellule.
	const PIVOT_DROP = 18;

	/** Bouton ▾ au bord droit d'une cellule (xr : son bord droit) ; filtré : entonnoir, comme Excel. */
	function pivotDrop(xr: number, y: number, h: number, filtered: boolean) {
		const size = Math.min(15, h - 4);
		const x = xr - size - 2;
		const top = y + (h - size) / 2;
		ctx.save();
		ctx.fillStyle = C.strip;
		ctx.fillRect(x, top, size, size);
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		ctx.strokeRect(Math.round(x) + 0.5, Math.round(top) + 0.5, size - 1, size - 1);
		ctx.fillStyle = filtered ? C.active : C.icon;
		ctx.font = FONT_SMALL;
		ctx.textAlign = 'center';
		ctx.textBaseline = 'middle';
		ctx.fillText(filtered ? '⧩' : '▾', x + size / 2, top + size / 2 + 0.5);
		ctx.restore();
	}

	/** Case −/+ d'un groupe de TCD en arborescence (x : son bord gauche), comme Excel. */
	function pivotFoldBox(x: number, y: number, h: number, folded: boolean) {
		const s = PIVOT_BOX;
		const left = Math.round(x) + 0.5;
		const top = Math.round(y + (h - s) / 2) + 0.5;
		ctx.save();
		ctx.fillStyle = C.strip;
		ctx.fillRect(left, top, s - 1, s - 1);
		ctx.strokeStyle = C.icon;
		ctx.lineWidth = 1;
		ctx.strokeRect(left, top, s - 1, s - 1);
		ctx.beginPath();
		const mid = top + (s - 1) / 2;
		ctx.moveTo(left + 2, mid);
		ctx.lineTo(left + s - 3, mid);
		if (folded) {
			const cxm = left + (s - 1) / 2;
			ctx.moveTo(cxm, top + 2);
			ctx.lineTo(cxm, top + s - 3);
		}
		ctx.stroke();
		ctx.restore();
	}

	/** Le clic tombe-t-il sur une case −/+ de TCD ? Plie ou déplie alors le groupe (une étape d'annulation). */
	function togglePivotFold(ev: MouseEvent, cell: { row: number; col: number }): boolean {
		const p = pivotAt(cell.row, cell.col);
		if (!p?.rows || cell.col !== p.col || !source.setPivot) return false;
		const info = pivotRowOf(p, cell.row);
		if (!pivotBoxHit(info, toLocal(ev).x - screenX(cell.col)) || !info?.path) return false;
		moveTo(cell.row, cell.col);
		const set = source.setPivot;
		const cfg = setFolded(p.config, info.path, !info.collapsed);
		menuAction(() => set(p.id, cfg));
		return true;
	}

	/** Développer, Réduire (le groupe de la cellule active), Développer tout, Réduire tout. */
	function foldPivot(p: SheetPivot, what: 'expand' | 'collapse' | 'expandAll' | 'collapseAll') {
		const set = source.setPivot;
		if (!set) return;
		let cfg: PivotConfig;
		if (what === 'expandAll' || what === 'collapseAll') cfg = foldAll(p.config, what === 'collapseAll');
		else {
			const path = foldTarget(pivotRowOf(p, active.row), what === 'expand');
			if (!path) return;
			cfg = setFolded(p.config, path, what === 'collapse');
		}
		menuAction(() => set(p.id, cfg));
	}

	/** Valeur affichée d'un filtre de rapport : « (Tous) », la valeur, ou « (Plusieurs éléments) ». */
	function pivotFilterText(values: string[]): string {
		if (!values.length) return '(Tous)';
		if (values.length === 1) return values[0] === '' ? '(Vides)' : values[0];
		return '(Plusieurs éléments)';
	}

	/** Le champ f du TCD est-il filtré ou trié par le ▾ de son étiquette ? */
	function pivotFiltered(p: SheetPivot, f: string): boolean {
		return !!p.config.items?.some((x) => x.field === f && x.values.length) || !!p.config.sort?.[f];
	}

	// --- Clé proposée -------------------------------------------------------
	// Quand la première colonne d'un tableau sans clé a des valeurs uniques
	// et remplies, une petite clé grisée dans son en-tête propose d'en faire
	// la clé, en un clic ; clic droit : « Ne plus proposer » pour ce tableau.
	// La clé n'est posée d'office qu'à la création, sur une première colonne
	// ID ou 1, 2, 3… (canvas/autokey.go, décision 0010).

	/** Infobulle d'une colonne numérotée d'office. */
	const AUTO_NUMBER_TIP = 'numérotée automatiquement : une ligne ajoutée reçoit le numéro suivant';

	const KEY_HINT_TIP = 'Cette colonne semble identifier chaque ligne : cliquer pour en faire la clé du tableau';
	/** Zones (écran) des clés proposées dessinées à cette image. */
	let keyHintBoxes: { e: Entry; x0: number; x1: number; y0: number; y1: number }[] = [];
	let hintMenu = $state<{ x: number; y: number; e: Entry } | null>(null);

	// --- Pastille « suit l'année » (§6.5) ------------------------------------
	// Un tableau qui suit l'onglet à choix porte, au coin de son cadre, une
	// pastille avec la valeur choisie (« 2025 ») ; un tableau sans pastille est
	// le même pour toutes les valeurs. Un clic sur la pastille ouvre son menu
	// (« Suivre l'année », à décocher).
	let followBadges: { e: Entry; x0: number; x1: number; y0: number; y1: number }[] = [];
	let followMenu = $state<{ x: number; y: number; e: Entry } | null>(null);

	function followBadgeAt(ev: MouseEvent): Entry | null {
		const { x, y } = toLocal(ev);
		return followBadges.find((b) => x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1)?.e ?? null;
	}

	/** Coche ou décoche « Suivre l'année » d'un tableau. */
	function toggleFollow(t: SheetTable, on: boolean) {
		const set = source.setFollow;
		if (!set) return;
		followMenu = menu = null;
		wrap.focus();
		writing = writing.catch(() => undefined).then(async () => {
			try {
				await set(t, on);
			} catch (err) {
				say(String(err instanceof Error ? err.message : err));
			}
		});
	}

	const hasKeyHint = (e: Entry, i: number) => i === 0 && !!e.t.keyHint && e.t.key === undefined && !!source.setKey;

	/** Tableau dont la clé proposée est sous la souris. */
	function keyHintAt(ev: MouseEvent): Entry | null {
		const { x, y } = toLocal(ev);
		return keyHintBoxes.find((b) => x >= b.x0 - 2 && x <= b.x1 + 1 && y >= b.y0 && y <= b.y1)?.e ?? null;
	}

	/** Pose la clé proposée (une étape d'annulation). */
	function acceptKeyHint(e: Entry) {
		const setKey = source.setKey;
		if (!setKey) return;
		const t = e.t;
		const name = t.columns[0]?.name ?? '';
		wrap.focus();
		writing = writing
			.catch(() => undefined)
			.then(async () => {
				try {
					await setKey(t, 0);
					say(`« ${name} » est maintenant la clé du tableau ${t.name} (Ctrl+Z pour annuler)`);
				} catch (err) {
					say(String(err instanceof Error ? err.message : err));
				}
			});
	}

	/** Clic droit sur la clé proposée : son petit menu. */
	function openHintMenu(ev: MouseEvent): boolean {
		const he = keyHintAt(ev);
		if (!he) return false;
		ev.preventDefault();
		closeMenus();
		const r = wrap.getBoundingClientRect();
		hintMenu = { x: ev.clientX - r.left, y: ev.clientY - r.top, e: he };
		return true;
	}

	/**
	 * Boutons d'un en-tête de colonne, alignés à droite : ▾ (filtre), type,
	 * entonnoir d'un filtre actif, clé, erreurs. Dans la ligne d'en-tête du
	 * tableau et dans le bandeau quand l'en-tête est figé. Renvoie leur largeur.
	 */
	function headIcons(e: Entry, i: number, x: number, y: number, cw: number, h: number): number {
		const col = e.t.columns[i];
		ctx.font = FONT_SMALL;
		ctx.fillStyle = C.icon;
		ctx.textAlign = 'right';
		ctx.textBaseline = 'middle';
		// Icône du type : SVG pour date et booléen, texte pour les autres.
		const svg = typeIconName(col.type);
		const ICON = 13;
		const icon = svg ? ' ▾' : (col.filter ? '⧩ ' : '') + typeIcon(col.type) + ' ▾';
		let iconW = ctx.measureText(icon).width;
		// Formule de la colonne en cours d'édition : son icône fx en vert.
		const calc = calcEdit?.e === e && calcEdit.i === i;
		const fx = !svg && typeIcon(col.type) === 'fx';
		if (calc && fx) ctx.fillStyle = C.active;
		ctx.fillText(icon, x + cw - 6, y + h / 2);
		ctx.fillStyle = C.icon;
		if (svg) {
			drawIcon(ctx, svg, x + cw - 6 - iconW - ICON, y + (h - ICON) / 2, ICON, C.icon, C.active, 1.3);
			iconW += ICON;
			if (col.filter) {
				ctx.fillText('⧩ ', x + cw - 6 - iconW, y + h / 2);
				iconW += ctx.measureText('⧩ ').width;
			}
		}
		if (e.t.key === i) {
			ctx.fillText('🔑 ', x + cw - 6 - iconW, y + h / 2);
			iconW += ctx.measureText('🔑 ').width;
		}
		// Classeur partagé, première colonne : tableau privé (resté sur ce
		// poste) ou publié en lecture seule.
		const share = i === 0 ? (e.t.shared === 'private' ? 'local' : e.t.readOnly ? 'readonly' : null) : null;
		if (share) {
			drawIcon(ctx, share, x + cw - 6 - iconW - ICON - 2, y + (h - ICON) / 2, ICON, C.icon, C.active, 1.3);
			iconW += ICON + 4;
		}
		if (hasKeyHint(e, i)) {
			// Clé proposée : la même clé, grisée et pâle ; un clic la pose.
			const kx = x + cw - 6 - iconW;
			const kw = ctx.measureText('🔑 ').width;
			ctx.save();
			ctx.globalAlpha = 0.4;
			ctx.filter = 'grayscale(1)';
			ctx.fillText('🔑 ', kx, y + h / 2);
			ctx.restore();
			keyHintBoxes.push({ e, x0: kx - kw, x1: kx, y0: y, y1: y + h });
			iconW += kw;
		}
		if (calc && !fx) {
			ctx.save();
			ctx.font = 'italic bold ' + FONT_SMALL;
			ctx.fillStyle = C.active;
			ctx.fillText('ƒx ', x + cw - 6 - iconW, y + h / 2);
			iconW += ctx.measureText('ƒx ').width;
			ctx.restore();
		}
		if (col.errors) {
			ctx.fillStyle = C.error;
			ctx.fillText('⚠ ' + col.errors, x + cw - 6 - iconW - ctx.measureText(' ').width, y + h / 2);
			ctx.fillStyle = C.icon;
		}
		return iconW;
	}

	/** Numéro d'une ligne de données du tableau (1 = première), ou sa clé : ses valeurs identifient les lignes (choix de Louis). */
	function rowLabel(e: Entry, r: number): string {
		if (e.t.key !== undefined && r <= e.last) {
			const k = e.cache.row(r - e.t.row - 1)?.text[e.t.key];
			if (k !== undefined) return k;
		}
		return String(r - e.t.row);
	}

	/** Texte raccourci avec « … » pour tenir dans width (police courante). */
	function ellipsize(text: string, width: number): string {
		if (ctx.measureText(text).width <= width) return text;
		let lo = 0;
		let hi = text.length;
		while (lo < hi) {
			const mid = (lo + hi + 1) >> 1;
			if (ctx.measureText(text.slice(0, mid) + '…').width <= width) lo = mid;
			else hi = mid - 1;
		}
		return lo > 0 ? text.slice(0, lo) + '…' : '…';
	}

	// Bordures des cellules : relevées pendant le dessin des cellules, tracées
	// après le quadrillage, sur ses lignes (deux cellules voisines partagent
	// le même trait), pour qu'il ne les recouvre pas.
	let pendingBorders: number[] = [];

	/** Bordures d'une cellule (style bt, bb, bl, br). */
	// Bordures de couleur (bc) : une liste de traits par couleur.
	let colorBorders = new Map<string, number[]>();

	function borders(st: CellStyle, x: number, y: number, w: number, h: number) {
		if (!st.bt && !st.bb && !st.bl && !st.br) return;
		if (w <= 0 || h <= 0) return; // ligne ou colonne masquée
		const X0 = Math.round(x) - 0.5;
		const X1 = Math.round(x + w) - 0.5;
		const Y0 = Math.round(y) - 0.5;
		const Y1 = Math.round(y + h) - 0.5;
		let list = pendingBorders;
		if (st.bc) {
			list = colorBorders.get(st.bc) ?? [];
			colorBorders.set(st.bc, list);
		}
		if (st.bt) list.push(X0 - 0.5, Y0, X1 + 0.5, Y0);
		if (st.bb) list.push(X0 - 0.5, Y1, X1 + 0.5, Y1);
		if (st.bl) list.push(X0, Y0 - 0.5, X0, Y1 + 0.5);
		if (st.br) list.push(X1, Y0 - 0.5, X1, Y1 + 0.5);
	}

	function strokeBorders() {
		if (!pendingBorders.length && !colorBorders.size) return;
		ctx.save();
		ctx.beginPath();
		ctx.rect(MARGIN, TOP_H, viewW - MARGIN, viewH - TOP_H);
		ctx.clip();
		ctx.lineWidth = 1;
		for (const [color, b] of [[C.text, pendingBorders] as const, ...colorBorders]) {
			if (!b.length) continue;
			ctx.strokeStyle = color;
			ctx.beginPath();
			for (let i = 0; i < b.length; i += 4) {
				ctx.moveTo(b[i], b[i + 1]);
				ctx.lineTo(b[i + 2], b[i + 3]);
			}
			ctx.stroke();
		}
		ctx.restore();
		pendingBorders = [];
		colorBorders = new Map();
	}

	/**
	 * Classeur collaboratif : petit triangle dans le coin bas droit d'une
	 * cellule dont la modification attend le serveur (gris bleu) ou en
	 * conflit (orange).
	 */
	function syncCorner(xr: number, yb: number, kind: SyncKind) {
		ctx.fillStyle = kind === 'conflict' ? C.conflict : C.pending;
		ctx.beginPath();
		ctx.moveTo(xr - 1, yb - 1);
		ctx.lineTo(xr - 7, yb - 1);
		ctx.lineTo(xr - 1, yb - 7);
		ctx.closePath();
		ctx.fill();
	}

	/** Petit triangle d'angle d'une cellule en erreur (le détail est dans la barre d'état). */
	function errorCorner(x: number, y: number) {
		ctx.fillStyle = C.error;
		ctx.beginPath();
		ctx.moveTo(x + 1, y + 1);
		ctx.lineTo(x + 7, y + 1);
		ctx.lineTo(x + 1, y + 7);
		ctx.closePath();
		ctx.fill();
	}

	/**
	 * Texte d'une cellule avec sa mise en forme : police, taille, gras,
	 * italique, souligné, barré, couleur, alignements horizontal et vertical,
	 * renvoi à la ligne. hh : hauteur de la cellule (par défaut celle de la
	 * ligne en y ; une fusion couvre plusieurs lignes).
	 */
	function drawText(s: string, x: number, y: number, cw: number, align: 'left' | 'center' | 'right', st: CellStyle | undefined, color: string, clipW = cw, hh?: number) {
		const h = hh ?? rh(rax().indexAt(y - TOP_H + paneTop + 1));
		if (h <= 0 || cw <= 0) return;
		ctx.save();
		ctx.beginPath();
		// Renvoyé à la ligne, ou fusion (hh) : le texte reste dans sa cellule.
		// La découpe s'arrête au bord droit de la cellule même quand son début
		// est caché sous la marge.
		const left = Math.max(x, MARGIN);
		if (st?.wr || hh !== undefined) {
			const top = Math.max(y, TOP_H);
			ctx.rect(left, top, x + (st?.wr ? cw : clipW) - left, y + h - top);
		} else ctx.rect(left, TOP_H, x + clipW - left, viewH - TOP_H);
		ctx.clip();
		const fancy = st && (st.ff || st.fs);
		ctx.font = fancy ? cellFont(st) : (st?.i ? 'italic ' : '') + (st?.b ? FONT_BOLD : FONT);
		ctx.textAlign = align;
		ctx.textBaseline = 'middle';
		ctx.fillStyle = st?.fg ?? color;
		const tx = align === 'left' ? x + 6 : align === 'right' ? x + cw - 6 : x + cw / 2;
		const px = fontPx(st?.fs);
		const lh = lineHeight(px);
		// Renvoi à la ligne (largeur de la cellule), ou sauts de ligne (Alt+Entrée) :
		// les lignes qui tiennent dans la hauteur.
		const lines = st?.wr
			? wrapLines(s, Math.max(8, cw - 12), (t) => ctx.measureText(t).width)
			: s.includes('\n')
				? cellLines(s, Math.max(1, Math.floor((h - 4) / lh)))
				: [s];
		const total = lines.length * lh;
		// Alignement vertical : centré par défaut.
		const top = st?.va === 'top' ? y + 2 : st?.va === 'bottom' ? y + h - 2 - total : y + (h - total) / 2;
		lines.forEach((line, k) => {
			const cy = top + lh * k + lh / 2;
			ctx.fillText(line, tx, cy);
			if (st?.u || st?.x) {
				const w = ctx.measureText(line).width;
				const lx = align === 'left' ? tx : align === 'right' ? tx - w : tx - w / 2;
				if (st.u) ctx.fillRect(lx, cy + Math.round(px * 0.55), w, 1);
				if (st.x) ctx.fillRect(lx, cy, w, 1);
			}
		});
		ctx.restore();
	}

	/**
	 * Style de la sélection (pour basculer gras, italique…, et le pinceau) :
	 * celui de la ligne ou colonne entière sélectionnée, sinon celui de la
	 * cellule active.
	 */
	function activeStyle(): CellStyle {
		const w = wholeKind();
		if (w === 'rows') return resolveStyle(sheetRowStyles.get(active.row)) ?? {};
		if (w === 'cols') return resolveStyle(sheetColStyles.get(active.col)) ?? {};
		return styleAt(active.row, active.col) ?? {};
	}

	/** Met en forme la sélection ; une bascule (b, i, u) suit la cellule active. */
	export function applyStyle(patch: Record<string, unknown>) {
		if (!source.applyStyle) return;
		lastRepeat = () => applyStyle(patch); // F4
		const p = { ...patch };
		for (const k of ['b', 'i', 'u', 'x'] as const) if (p[k] === 'toggle') p[k] = !activeStyle()[k];
		if (p.dp === 'more' || p.dp === 'less') {
			// Comme Excel : part des décimales affichées dans la cellule active
			// (pas de la valeur stockée ni, pour une colonne calculée, de la formule).
			const cur = activeStyle().dp ?? decimalsOf(shownText(active.row, active.col));
			p.dp = Math.max(0, Math.min(10, cur + (p.dp === 'more' ? 1 : -1)));
		}
		const { r0, r1, c0, c1 } = selection();
		const w = wholeKind();
		if (w && source.applyLineStyle) {
			// Lignes ou colonnes entières : la mise en forme vaut pour toutes
			// leurs cellules, présentes et à venir (comme Excel).
			const line = source.applyLineStyle;
			const rows = w === 'rows';
			const [i0, i1] = rows ? [r0, r1] : [c0, c1];
			if (p.border === 'outline') {
				menuAction(async () => {
					await line(rows, i0, i0, rows ? { bt: true } : { bl: true });
					await line(rows, i1, i1, rows ? { bb: true } : { br: true });
				});
				return;
			}
			menuAction(() => line(rows, i0, i1, borderPatch(p)));
			return;
		}
		const apply = source.applyStyle;
		if (p.border === 'outline') {
			// Contour : chaque bord de la plage reçoit son trait.
			menuAction(async () => {
				await apply(r0, c0, r0, c1, { bt: true });
				await apply(r1, c0, r1, c1, { bb: true });
				await apply(r0, c0, r1, c0, { bl: true });
				await apply(r0, c1, r1, c1, { br: true });
			});
			return;
		}
		menuAction(() => apply(r0, c0, r1, c1, borderPatch(p)));
	}

	/** Traduit border: 'all' | 'none' en propriétés de bordure. */
	function borderPatch(p: Record<string, unknown>) {
		if (p.border === 'all') {
			delete p.border;
			Object.assign(p, { bt: true, bb: true, bl: true, br: true });
		} else if (p.border === 'none') {
			delete p.border;
			Object.assign(p, { bt: null, bb: null, bl: null, br: null });
		}
		return p;
	}

	/** Texte affiché d'une cellule (avant la mise en forme de la cellule). */
	function shownText(r: number, c: number): string {
		const e = tableAt(r, c);
		if (e && r > e.t.row) return e.cache.row(r - e.t.row - 1)?.text[c - e.t.col] ?? '';
		if (e) return e.t.columns[c - e.t.col]?.name ?? '';
		const k = key(r, c);
		return shown.get(k)?.text ?? cells.get(k) ?? '';
	}

	function decimalsOf(text: string): number {
		const m = /[,.](\d+)/.exec(text.replace(/\s/g, ''));
		return m ? m[1].length : 0;
	}

	// Reproduire la mise en forme : la prochaine sélection reçoit le style copié.
	let painter = $state<Record<string, unknown> | null>(null);
	export function formatPainter() {
		painter = painterPatch(activeStyle());
		say('Reproduire la mise en forme : sélectionnez les cellules à mettre en forme (Échap pour annuler)');
	}
	function applyPainter() {
		if (!painter) return;
		const p = painter;
		painter = null;
		applyStyle(p);
	}

	// --- Format de cellule (Ctrl+1) et commandes voisines ----------------------

	let formatDlg = $state<{ tab: FormatTab; style: CellStyle; sample: { text: string; value: unknown }; canMerge: boolean; merged: boolean } | null>(null);

	/** La sélection est-elle faite de cellules libres seulement (fusion possible) ? */
	function freeSelection(): boolean {
		const { r0, r1, c0, c1 } = selection();
		if (entries.some((e) => e.t.row <= r1 && e.last >= r0 && e.t.col <= c1 && e.lastCol >= c0)) return false;
		return !pivots.some((p) => !p.error && p.row <= r1 && p.row + p.cells.length - 1 >= r0 && p.col <= c1 && p.col + (p.cells[0]?.length ?? 1) - 1 >= c0);
	}

	/** Ouvre le dialogue Format de cellule (Ctrl+1, ruban, clic droit). */
	export function openFormat(tab: FormatTab = 'number') {
		if (!source.applyStyle) return;
		if (locked) {
			say(locked);
			return;
		}
		closeMenus();
		const info = cellInfo(active.row, active.col);
		const sel = selection();
		const single = sel.r0 === sel.r1 && sel.c0 === sel.c1;
		const value = info.kind === 'data' && info.row ? info.row.cells[info.i!] : undefined;
		const text = info.kind === 'free' ? (shown.get(key(active.row, active.col))?.text ?? info.value) : info.kind === 'data' ? (info.row?.text[info.i!] ?? '') : info.value;
		formatDlg = { tab, style: activeStyle(), sample: { text, value }, canMerge: !single && (freeSelection() || bandSelection()) && !!source.mergeCells, merged: mergedAt(active.row, active.col) };
	}

	function applyFormat(patch: Record<string, unknown>, merge: boolean | null) {
		formatDlg = null;
		wrap.focus();
		const { border, ...rest } = patch;
		if (Object.keys(rest).length) applyStyle(rest);
		if (border) applyStyle({ border });
		if (merge === true) mergeActive();
		else if (merge === false) unmergeActive();
	}

	/** La sélection touche-t-elle la bande des groupes d'un tableau à deux lignes (le moteur y fusionne) ? */
	function bandSelection(): boolean {
		const { r0, r1, c0, c1 } = selection();
		return entries.some((e) => e.t.twoLevel && r0 <= e.t.row - 1 && r1 >= e.t.row - 1 && c0 <= e.lastCol && c1 >= e.t.col);
	}

	/** Cellule fusionnée : fusion de cellules libres, groupe de la bande ou colonne sur un niveau. */
	function mergedAt(r: number, c: number): boolean {
		if (mergeOf(r, c)) return true;
		const g = boxOf(r, c);
		return !!g && !!groupBandAt(g.row, g.col);
	}

	/** Fusionner et centrer (bascule, comme le bouton d'Excel) : cellules libres, ou bande des groupes d'en-têtes. */
	export function mergeActive(toggle = false) {
		if (toggle && mergedAt(active.row, active.col)) {
			unmergeActive();
			return;
		}
		const { r0, r1, c0, c1 } = selection();
		const mergeCells = source.mergeCells;
		if (!mergeCells) return;
		if (r0 === r1 && c0 === c1) {
			say('Sélectionnez plusieurs cellules libres pour les fusionner');
			return;
		}
		const band = bandSelection();
		if (!freeSelection() && !band) {
			say('Fusionner et centrer ne s’applique pas dans un tableau : un tableau garde une cellule par valeur. Fusionnez des cellules libres, hors des tableaux.');
			return;
		}
		writing = writing.catch(() => undefined).then(async () => {
			try {
				const n = await mergeCells(r0, c0, r1, c1);
				moveTo(r0, c0);
				// Bande des groupes : le moteur dit ce qui a été renommé.
				message = band ? '' : n ?`Cellules fusionnées : seule la valeur en haut à gauche est gardée (${n} effacée${n > 1 ? 's' : ''}, Ctrl+Z pour annuler)` : 'Cellules fusionnées et centrées';
			} catch (err) {
				linesHint = null; // refusée : rien ne glisse
				message = String(err instanceof Error ? err.message : err);
			}
			refreshStatus();
			schedule();
		});
	}

	export function unmergeActive() {
		const { r0, r1, c0, c1 } = selection();
		const unmerge = source.unmergeCells;
		// Colonnes sur un niveau de la sélection : séparées ici en bande +
		// en-tête, le temps de taper leur groupe dans la bande (rien n'est
		// écrit avant). Les groupes sur plusieurs colonnes : le moteur.
		const ones: { e: Entry; c: number }[] = [];
		let groups = false;
		for (const e of entries) {
			if (!e.t.twoLevel || r0 > e.t.row || r1 < e.t.row - 1) continue;
			for (const s of groupSpans(e.t.columns.map((k) => k.name))) {
				const a = e.t.col + s.i0;
				const b = e.t.col + s.i1;
				if (b < c0 || a > c1) continue;
				if (s.i1 > s.i0) groups = true;
				else if (singleCol(e, s.i0)) ones.push({ e, c: a });
			}
		}
		if (ones.length) {
			for (const o of ones) splitCols.add(splitKey(o.e.t.name, o.e.t.columns[o.c - o.e.t.col].name));
			rebuildBoxes();
			const first = ones[0];
			if (!groups) moveTo(first.e.t.row - 1, first.c);
			const names = ones.map((o) => `« ${o.e.t.columns[o.c - o.e.t.col].name} »`).join(', ');
			message = `${names} sur deux lignes : tapez le groupe dans la bande au-dessus (sans groupe, la colonne reste sur un niveau)`;
			refreshStatus();
			schedule();
			if (!groups) return;
		}
		menuAction(unmerge && (() => unmerge(r0, c0, r1, c1)));
	}

	/** Renvoyer à la ligne (bascule) et alignement vertical (ruban Alignement). */
	export function toggleWrap() {
		applyStyle({ wr: !activeStyle().wr || null });
	}

	// Masquer / afficher des lignes ou colonnes entières.
	function setHidden(rows: boolean, hide: boolean) {
		const set = source.setLinesHidden;
		if (!set) return;
		const { r0, r1, c0, c1 } = selection();
		const [i0, i1] = rows ? [r0, r1] : [c0, c1];
		menuAction(async () => {
			await set(rows, i0, i1, hide);
			if (hide) {
				// La cellule active quitte les lignes masquées.
				if (rows) moveTo(i1 + 1, c0);
				else moveTo(r0, i1 + 1);
			}
		});
	}

	// Repères des lignes et colonnes masquées (gouttière, bandeau), cliquables.
	type HiddenMark = { rows: boolean; i0: number; i1: number; x: number; y: number; w: number; h: number };
	let hiddenMarks: HiddenMark[] = [];
	/** Suites de lignes (ou colonnes) masquées consécutives. */
	function hiddenRuns(set: Set<number>): [number, number][] {
		const s = [...set].sort((a, b) => a - b);
		const out: [number, number][] = [];
		for (const i of s) {
			const last = out[out.length - 1];
			if (last && i === last[1] + 1) last[1] = i;
			else out.push([i, i]);
		}
		return out;
	}
	function hiddenMark(m: HiddenMark) {
		ctx.save();
		ctx.fillStyle = C.active;
		ctx.beginPath();
		ctx.roundRect(m.x, m.y, m.w, m.h, 3);
		ctx.fill();
		// Deux traits blancs : des lignes repliées.
		ctx.strokeStyle = '#fff';
		ctx.lineWidth = 1;
		ctx.beginPath();
		if (m.rows) {
			const cx0 = m.x + m.w / 2;
			ctx.moveTo(cx0 - 4, m.y + 2.5);
			ctx.lineTo(cx0 + 4, m.y + 2.5);
			ctx.moveTo(cx0 - 4, m.y + m.h - 2.5);
			ctx.lineTo(cx0 + 4, m.y + m.h - 2.5);
		} else {
			const cy0 = m.y + m.h / 2;
			ctx.moveTo(m.x + 2.5, cy0 - 4);
			ctx.lineTo(m.x + 2.5, cy0 + 4);
			ctx.moveTo(m.x + m.w - 2.5, cy0 - 4);
			ctx.lineTo(m.x + m.w - 2.5, cy0 + 4);
		}
		ctx.stroke();
		ctx.restore();
		hiddenMarks.push(m);
	}
	/** Repère sous la souris (coordonnées du canevas). */
	function hiddenMarkAt(e: MouseEvent): HiddenMark | undefined {
		const { x, y } = toLocal(e);
		return hiddenMarks.find((m) => x >= m.x - 2 && x <= m.x + m.w + 2 && y >= m.y - 2 && y <= m.y + m.h + 2);
	}

	/** Lignes ou colonnes masquées dans la sélection (pour proposer Afficher). */
	function hiddenIn(rows: boolean): boolean {
		const { r0, r1, c0, c1 } = selection();
		const [i0, i1] = rows ? [r0, r1] : [c0, c1];
		for (const i of rows ? hiddenRows : styleHiddenCols) if (i >= i0 - 1 && i <= i1 + 1) return true;
		return false;
	}

	/** Ruban (Accueil › Cellules › Format) : masquer ou afficher les lignes ou les colonnes de la sélection. */
	export function hideLines(rows: boolean, hide: boolean) {
		if (hide) setHidden(rows, true);
		else unhide(rows);
	}

	/** Afficher : les lignes (ou colonnes) masquées de la sélection et celles qui la bordent. */
	function unhide(rows: boolean) {
		const set = source.setLinesHidden;
		if (!set) return;
		const { r0, r1, c0, c1 } = selection();
		let [i0, i1] = rows ? [r0, r1] : [c0, c1];
		const hidden = rows ? hiddenRows : styleHiddenCols;
		while (i0 > 0 && hidden.has(i0 - 1)) i0--;
		while (hidden.has(i1 + 1)) i1++;
		menuAction(() => set(rows, i0, i1, false));
	}

	// Hauteur de ligne / largeur de colonne (clic droit), comme Excel.
	let sizeDlg = $state<{ rows: boolean; value: number; x: number; y: number } | null>(null);

	function openSize(rows: boolean, x: number, y: number) {
		closeMenus();
		sizeDlg = { rows, value: rows ? rh(active.row) || ROW_H : cax().size(active.col) || DEF_W, x, y };
	}

	function applySize() {
		const d = sizeDlg;
		sizeDlg = null;
		wrap.focus();
		if (!d) return;
		const v = Math.round(Number(d.value));
		if (!Number.isFinite(v) || v < 4 || v > 1000) {
			say('Taille entre 4 et 1 000 pixels');
			return;
		}
		const { r0, r1, c0, c1 } = selection();
		const set = d.rows ? source.setRowHeight : source.setColumnWidth;
		if (!set) return;
		menuAction(async () => {
			for (let i = d.rows ? r0 : c0; i <= (d.rows ? r1 : c1) && i - (d.rows ? r0 : c0) < 500; i++) await set(i, v);
		});
	}

	/**
	 * Ajuster la hauteur (lignes sélectionnées) : retour à la hauteur
	 * automatique, d'après le contenu (comme le double-clic sur le bord
	 * d'un numéro de ligne).
	 */
	function autoFitRows() {
		closeMenus();
		const set = source.setRowHeight;
		if (!set) return;
		const { r0, r1 } = selection();
		const rows: number[] = [];
		for (const r of userHeights.keys()) if (r >= r0 && r <= r1) rows.push(r);
		if (!rows.length) return;
		for (const r of rows) userHeights.delete(r);
		rebuildRows();
		updateExtent();
		schedule();
		menuAction(async () => {
			for (const r of rows) await set(r, 0);
		});
	}

	/** Ajuster la largeur de colonne (colonnes sélectionnées), d'après le contenu affiché. */
	function autoFitCols() {
		closeMenus();
		const { c0, c1 } = selection();
		for (let c = c0; c <= c1 && c - c0 < 500; c++) if (!hiddenCols.has(c)) autoFitColumn(c);
	}

	// Volets figés.
	/** Affichage › Figer les volets : à la cellule active, la ligne du haut, la première colonne, ou les libérer. */
	export function freezeActive(what: 'cell' | 'row' | 'col' | 'none') {
		const set = source.setFreeze;
		if (!set) return;
		let rows = 0;
		let cols = 0;
		if (what === 'row') rows = 1;
		else if (what === 'col') cols = 1;
		else if (what === 'cell') {
			rows = active.row;
			cols = active.col;
			if (!rows && !cols) {
				say('Placez la cellule active sous les lignes et à droite des colonnes à figer');
				return;
			}
		}
		// La partie figée part du haut de la feuille : elle doit tenir à l'écran.
		if (rowTop(rows) > vs.view * 0.75 || cax().offset(cols) > (viewW - MARGIN) * 0.75) {
			say('La partie à figer dépasse l’écran : faites défiler la feuille vers le haut, ou choisissez une cellule plus haute ou plus à gauche');
			return;
		}
		if (rows || cols) {
			vs.to(0);
			scroller.scrollLeft = 0;
			syncNative();
		}
		menuAction(() => set(rows, cols));
	}

	// --- Validation des données : liste déroulante ------------------------------

	/** Propriété dv de la cellule (liste saisie ou plage source), hors en-tête. */
	function dvAt(r: number, c: number): unknown {
		const e = tableAt(r, c);
		if (e && r === e.t.row) return undefined;
		return styleAt(r, c, e)?.dv;
	}

	/** Plage source de la validation de la cellule (liste relue), ou undefined. */
	function dvSrcAt(r: number, c: number): string | undefined {
		return dvSource(dvAt(r, c));
	}

	/**
	 * Valeurs permises de la cellule (validation par liste), ou undefined.
	 * Liste tirée d'une plage : les valeurs relues (vide tant qu'elles ne le
	 * sont pas : la cellule a quand même sa liste déroulante).
	 */
	function listOf(r: number, c: number): string[] | undefined {
		const dv = dvAt(r, c);
		const src = dvSource(dv);
		if (src) return dvLists.get(src)?.values ?? [];
		return Array.isArray(dv) && dv.length ? dv : undefined;
	}

	// Listes tirées d'une plage, relues sur le serveur (par source) : vidées à
	// chaque relecture de la feuille, relues à l'ouverture de la liste et
	// quand la cellule active en a une (la saisie est vérifiée avec elles).
	const dvLists = new Map<string, DvList>();
	let dvGen = 0;

	/** Relit la liste d'une plage source ; une erreur de lecture est une liste en erreur. */
	async function loadDv(src: string): Promise<DvList> {
		const gen = dvGen;
		let l: DvList;
		try {
			l = source.validationList ? await source.validationList(src) : { values: [], error: 'lecture impossible ici' };
		} catch (err) {
			l = { values: [], error: String((err as Error)?.message ?? err) };
		}
		if (gen === dvGen) dvLists.set(src, l);
		return l;
	}

	/** La cellule active a une liste tirée d'une plage : la relire (vérification de la saisie). */
	function prefetchDv() {
		const src = dvSrcAt(active.row, active.col);
		const k = `${dvGen}|${src}`;
		if (!src || dvLists.has(src) || dvLoading.has(k)) return;
		dvLoading.add(k);
		void loadDv(src).finally(() => dvLoading.delete(k));
	}
	const dvLoading = new Set<string>();

	/** Bouton ▾ de la liste, juste à droite de la cellule active (comme Excel). */
	function dropButton() {
		const g = mergeOf(active.row, active.col);
		const right = g ? screenX(g.col) + cax().offset(g.col + g.cols) - cax().offset(g.col) : screenX(active.col) + cax().size(active.col);
		const bottom = screenY(active.row) + (g ? rowTop(g.row + g.rows) - rowTop(g.row) : rh(active.row));
		const h = Math.min(18, rh(active.row) - 2);
		return { x: right + 1, y: bottom - h - 1, w: 16, h };
	}

	// Icône « Mettre sous forme de tableau » d'un îlot de cellules libres.
	let tableHintBox: { x: number; y: number; w: number; h: number } | null = null;
	/** La cellule active est dans un bloc d'au moins deux cellules libres, hors tableau : l'icône s'affiche. */
	function asTableHint(): boolean {
		const { row, col } = active;
		if (!source.formatAsTable || locked || !cells.has(key(row, col)) || tableAt(row, col) || keptOut(row, col) || listOf(row, col)) return false;
		// Le bloc libre (4-connexité, hors tableaux et légendes) : au moins trois
		// cellules, et pas seulement des phrases (un texte de plusieurs lignes
		// n'est pas une liste à mettre en tableau).
		const seen = new Set<string>();
		const stack = [[row, col]];
		let n = 0;
		let short = 0;
		while (stack.length && seen.size < 200) {
			const [r, c] = stack.pop()!;
			const k = key(r, c);
			if (seen.has(k) || !cells.has(k) || tableAt(r, c) || keptOut(r, c)) continue;
			seen.add(k);
			n++;
			const v = (cells.get(k) ?? '').trim();
			if (v.split(/s+/).length <= 3 && v.length <= 40) short++;
			stack.push([r + 1, c], [r - 1, c], [r, c + 1], [r, c - 1]);
		}
		return n >= 3 && short * 2 >= n;
	}
	function tableHintAt(ev: MouseEvent): boolean {
		const b = tableHintBox;
		if (!b) return false;
		const { x, y } = toLocal(ev);
		return x >= b.x && x <= b.x + b.w && y >= b.y && y <= b.y + b.h;
	}

	let listMenu = $state<{ x: number; y: number; values: string[]; row: number; col: number; error?: string; loading?: boolean } | null>(null);

	/** Ouvre la liste déroulante ; une liste tirée d'une plage est relue à chaque ouverture. */
	function openList() {
		const values = listOf(active.row, active.col);
		if (!values) return;
		const src = dvSrcAt(active.row, active.col);
		const b = dropButton();
		closeMenus();
		const at = { row: active.row, col: active.col };
		const cached = src ? dvLists.get(src) : undefined;
		listMenu = { x: toView(screenX(active.col)), y: toView(b.y + b.h) + 2, values, ...at, error: cached?.error, loading: !!src && !cached };
		if (!src) return;
		void loadDv(src).then((l) => {
			const m = listMenu;
			if (m && m.row === at.row && m.col === at.col) listMenu = { ...m, values: l.values, error: l.error, loading: false };
		});
	}

	function pickFromList(v: string) {
		const m = listMenu;
		listMenu = null;
		wrap.focus();
		if (!m || locked) {
			if (locked) say(locked);
			return;
		}
		if (v !== cellInfo(m.row, m.col).value) write(m.row, m.col, v);
	}

	// Dialogue : valeurs saisies (text) ou plage source (src), aperçu des
	// valeurs de la plage (relu à chaque changement de la référence).
	let validDlg = $state<{ target: string; mode: 'list' | 'range'; text: string; src: string; error: string; preview: DvList | null; loading: boolean } | null>(null);
	let previewTimer: ReturnType<typeof setTimeout> | undefined;

	/** Données › Validation des données : liste des valeurs permises de la sélection. */
	export function openValidation() {
		if (!source.applyStyle) return;
		if (locked) {
			say(locked);
			return;
		}
		closeMenus();
		validDlg = { target: validationTarget(), ...dvDialog(dvAt(active.row, active.col)), error: '', preview: null, loading: false };
		if (validDlg.mode === 'range') refreshPreview(0);
	}

	/** Ce que vise la validation (la sélection), pour le titre du dialogue. */
	function validationTarget(): string {
		const e = tableAt(active.row, active.col);
		const table = e ? { name: e.t.name, row: e.t.row, last: e.last, col: e.t.col, columns: e.t.columns.map((c) => c.name) } : null;
		return dvTarget({ sel: selection(), whole: wholeKind(), table, sheet: sheetName });
	}

	/** Relit l'aperçu de la plage source du dialogue (après delay ms). */
	function refreshPreview(delay = 250) {
		clearTimeout(previewTimer);
		const d = validDlg;
		if (!d) return;
		const src = cleanSource(d.src);
		if (d.mode !== 'range' || !src || !source.validationList) {
			d.preview = null;
			d.loading = false;
			return;
		}
		d.loading = true;
		previewTimer = setTimeout(async () => {
			const l = await loadDv(src);
			const cur = validDlg;
			if (!cur || cleanSource(cur.src) !== src) return;
			cur.preview = l;
			cur.loading = false;
		}, delay);
	}

	function applyValidation(clear = false) {
		const d = validDlg;
		if (!d) return;
		let dv: string[] | { src: string } | null = null;
		if (!clear) {
			const p = dvPatch(d.mode, d.text, d.src);
			if ('error' in p) {
				d.error = p.error;
				return;
			}
			dv = p.dv;
		}
		clearTimeout(previewTimer);
		validDlg = null;
		wrap.focus();
		applyStyle({ dv });
		if (!dv) say('Validation retirée');
		else if (Array.isArray(dv)) say(`Liste déroulante : ${dv.length} valeur${dv.length > 1 ? 's' : ''} permise${dv.length > 1 ? 's' : ''} (Alt+↓ pour l’ouvrir)`);
		else say(`Liste déroulante tirée de ${dv.src}, relue à chaque ouverture (Alt+↓ pour l’ouvrir)`);
	}

	// « Sélectionner » : le dialogue se retire, la plage source se désigne
	// dans la grille (clic, glissé, Maj+flèches, clic sur l'en-tête d'une
	// colonne de tableau), au besoin sur une autre feuille (clic sur son
	// onglet). Entrée la reprend dans le dialogue, Échap revient sans la
	// changer ; dans les deux cas la feuille de la cellule et la sélection
	// d'avant (la cible de la validation) sont rétablies. back : retour en
	// cours vers la feuille d'origine, ref : la référence à reprendre.
	let dvPick = $state<{
		active: { row: number; col: number };
		anchor: { row: number; col: number };
		whole: typeof wholeSel;
		sheet: number;
		back?: boolean;
		ref?: string | null;
	} | null>(null);
	let dvPickRef = $state('');

	function startPick() {
		if (!validDlg) return;
		dvPick = { active: { ...active }, anchor: { ...anchor }, whole: wholeSel, sheet: sheetId() };
		dvPickRef = pickedRef() ?? '';
		wrap.focus();
		say('Sélectionnez la plage source (sur cette feuille ou une autre), puis Entrée (Échap : annuler)');
	}

	/**
	 * Référence de la sélection pointée : Tableau[Colonne] (cellules de la
	 * colonne ou son en-tête), Tableau[Colonne][a:b] ou Cell[A1:A10] ; sur une
	 * autre feuille que celle de la cellule, Feuille2!Cell[A1:A10].
	 */
	function pickedRef(): string | null {
		const ref = rangeRef(selection(), refTables());
		return ref !== null && dvPick && dvPick.sheet !== sheetId() ? qualifyRef(ref, sheetName) : ref;
	}

	function endPick(keep: boolean) {
		const p = dvPick;
		if (!p || p.back) return;
		const ref = keep ? pickedRef() : null;
		if (keep && ref === null) {
			say('Une plage source couvre une seule colonne d’un tableau, ou des cellules libres');
			return;
		}
		if (p.sheet !== sheetId() && onsheet) {
			// Retour à la feuille de la cellule : fini quand elle est relue (applyData).
			dvPick = { ...p, back: true, ref };
			onsheet(p.sheet);
			return;
		}
		finishPick(p, ref);
	}

	/** Sélection d'avant rétablie, référence reprise dans le dialogue. */
	function finishPick(p: NonNullable<typeof dvPick>, ref: string | null | undefined) {
		dvPick = null;
		active = p.active;
		anchor = p.anchor;
		wholeSel = p.whole;
		ensureVisible(p.active.row, p.active.col);
		schedule();
		refreshStatus();
		const d = validDlg;
		if (d && ref) {
			d.src = ref;
			d.mode = 'range';
			d.error = '';
			refreshPreview(0);
		}
	}

	/** Saisie refusée par la validation de la cellule : le message, sinon null (value : la valeur de la liste). */
	function refusal(row: number, col: number, text: string): { message: string } | { value: string } {
		if (text.trimStart().startsWith('=')) return { value: text }; // une formule n'est pas vérifiée
		const src = dvSrcAt(row, col);
		const r = src ? validateSource(src, dvLists.get(src), text) : validate(listOf(row, col), text);
		return r.ok ? { value: r.value } : { message: r.message };
	}

	// --- Notes (Révision) ---------------------------------------------------------

	let noteTip = $state<{ x: number; y: number; text: string; key: string } | null>(null);
	let noteDlg = $state<{ row: number; col: number; text: string; had: boolean; pinned: boolean; x: number; y: number } | null>(null);
	// Notes épinglées visibles : relevées en dessinant, affichées en post-it.
	let pinnedFrame: { x: number; y: number; text: string }[] = [];
	let pinnedShown = $state<{ x: number; y: number; text: string }[]>([]);

	/** Note de la cellule, s'il y en a une. */
	function noteOf(r: number, c: number): string | undefined {
		const g = mergeOf(r, c);
		if (g) return styleAt(g.row, g.col, undefined)?.nt;
		return styleAt(r, c)?.nt;
	}

	/** Infobulle de la note sous la souris (comme Excel). */
	function updateNoteTip(cell: { row: number; col: number } | null) {
		const text = cell && !noteDlg ? noteOf(cell.row, cell.col) : undefined;
		if (!cell || !text) {
			if (noteTip) noteTip = null;
			return;
		}
		const g = mergeOf(cell.row, cell.col);
		const r = g?.row ?? cell.row;
		const c = g?.col ?? cell.col;
		const k = key(r, c);
		if (noteTip?.key === k) return;
		const right = screenX(c) + cax().offset(c + (g?.cols ?? 1)) - cax().offset(c);
		noteTip = { x: Math.min(right + 8, viewW - 240), y: screenY(r), text, key: k };
	}

	/** Révision › Nouvelle note (ou Modifier la note) sur la cellule active. */
	export function editNote() {
		if (!source.setNote) return;
		if (locked) {
			say(locked);
			return;
		}
		closeMenus();
		noteTip = null;
		const text = noteOf(active.row, active.col) ?? '';
		const g = mergeOf(active.row, active.col);
		const pinned = !!(g ? styleAt(g.row, g.col, undefined) : styleAt(active.row, active.col))?.np;
		const right = screenX(active.col) + cax().size(active.col);
		noteDlg = { row: active.row, col: active.col, text, had: !!text, pinned, x: Math.min(toView(right) + 8, Math.max(8, toView(viewW) - 270)), y: Math.max(4, toView(screenY(active.row))) };
	}

	// Enregistrement automatique (comme un post-it) : à la fermeture, par
	// clic ailleurs ou Échap ; la corbeille supprime la note.
	function saveNote(remove = false) {
		const d = noteDlg;
		noteDlg = null;
		wrap.focus();
		const set = source.setNote;
		if (!d || !set) return;
		const text = remove ? '' : d.text;
		const before = noteOf(d.row, d.col) ?? '';
		if (text.trim() === before.trim()) return;
		if (text.trim() === '' && !d.had) return;
		menuAction(() => set(d.row, d.col, text));
	}

	/** Épingle ou désépingle la note en cours (enregistrée d'abord si elle est neuve). */
	function togglePin() {
		const d = noteDlg;
		const pin = source.pinNote;
		const set = source.setNote;
		if (!d || !pin || !set) return;
		if (!d.text.trim()) {
			say('Écrivez la note avant de l’épingler');
			return;
		}
		d.pinned = !d.pinned;
		const { row, col, text, pinned } = d;
		const changed = text.trim() !== (noteOf(row, col) ?? '').trim();
		d.had = true;
		menuAction(async () => {
			if (changed) await set(row, col, text);
			await pin(row, col, pinned);
		});
	}

	/**
	 * Accueil › Édition (Remplissage, Effacer, Trier et filtrer) et Mettre
	 * sous forme de tableau (Ctrl+L) : sur la sélection ou la cellule active.
	 */
	export function editActive(what: 'fillDown' | 'fillRight' | 'clearContents' | 'clearFilters' | 'formatAsTable') {
		if (locked) {
			say(locked);
			return;
		}
		const { row, col } = active;
		switch (what) {
			case 'fillDown':
			case 'fillRight':
				copyFill(what === 'fillDown');
				return;
			case 'clearContents':
				clearSelection();
				lastRepeat = () => clearSelection();
				return;
			case 'clearFilters': {
				const e = tableAt(row, col);
				const clear = source.clearFilters;
				if (!e || !clear || !e.t.columns.some((c) => c.filter)) say('Le tableau de la cellule active n’est pas filtré');
				else menuAction(() => clear(e.t));
				return;
			}
			case 'formatAsTable': {
				const fmt = source.formatAsTable;
				if (!fmt) return;
				if (tableAt(row, col)) say('La cellule active est déjà dans un tableau');
				else menuAction(() => fmt(row, col));
				return;
			}
		}
	}

	/** Révision › Supprimer la note de la cellule active. */
	export function deleteNote() {
		const set = source.setNote;
		if (!set || !noteOf(active.row, active.col)) {
			say('La cellule active n’a pas de note');
			return;
		}
		const { row, col } = active;
		menuAction(() => set(row, col, ''));
	}

	function pushStat(a: number[], v: number) {
		a.push(v);
		if (a.length > 300) a.shift();
	}

	const ms = (v: number) => (Number.isFinite(v) ? v.toFixed(v < 10 ? 1 : 0) + ' ms' : '–');

	/** Ce que contient une cellule, pour l'édition et la barre de formule. */
	function cellInfo(r: number, c: number) {
		const e = tableAt(r, c);
		if (!e) {
			// Bande des groupes : la saisie y renomme le groupe (le moteur s'en charge).
			const g = groupBandAt(r, c);
			// Colonne sur un niveau : la bande et l'en-tête sont une cellule, son nom.
			if (g && singleCol(g, c - g.t.col)) return { e: g, i: c - g.t.col, kind: 'header' as const, value: g.t.columns[c - g.t.col].name };
			if (g) return { e, kind: 'free' as const, value: spanAt(g.t.columns.map((k) => k.name), c - g.t.col)?.group ?? '' };
			return { e, kind: 'free' as const, value: cells.get(key(r, c)) ?? '' };
		}
		const i = c - e.t.col;
		if (r === e.t.row) return { e, i, kind: 'header' as const, value: headerLabel(e.t.columns[i].name, e.t.twoLevel) };
		const n = r - e.t.row - 1;
		const row = e.cache.row(n);
		return {
			e,
			i,
			n,
			row,
			kind: 'data' as const,
			// Une saisie hors type se corrige à partir de sa valeur brute.
			// Colonne calculée : la barre de formule montre la formule, comme Excel.
			value: e.t.columns[i].formula ?? (row ? (row.bad?.get(i) ?? editValue(e.t.columns[i].type, row.cells[i])) : '')
		};
	}

	/** Désignation de la cellule active (pas d'adresse A1). */
	/** Adresse d'une cellule libre, telle qu'elle s'écrit dans une formule ou en SQL : Cell[A1]. */
	function cellAddr(r: number, c: number): string {
		return `Cell[${addrA1(r, c)}]`;
	}

	/** Lettre(s) de colonne et numéro de ligne : A1. */
	function addrA1(r: number, c: number): string {
		let s = '';
		for (let n = c + 1; n > 0; n = Math.floor((n - 1) / 26)) s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
		return s + (r + 1);
	}

	function cellName(r: number, c: number): string {
		const named = names.get(key(r, c));
		if (named) return named;
		const e = ownerAt(r, c);
		// Cellule libre : lettre et numéro seulement (A1) ; Cell_A1 dans une formule.
		if (!e) return addrA1(r, c);
		const k = e.t.columns[c - e.t.col];
		const one = r < e.t.row && singleCol(e, c - e.t.col);
		if (r < e.t.row && !one) {
			const s = spanAt(e.t.columns.map((x) => x.name), c - e.t.col);
			const n = s ? s.i1 - s.i0 + 1 : 1;
			return s?.group ? `${e.t.name} · groupe d'en-têtes « ${s.group} » (${n} colonne${n > 1 ? 's' : ''})` : `${e.t.name} · groupe d'en-têtes (vide)`;
		}
		const col = `${e.t.name}[${k.name}]`;
		if (r === e.t.row || one) {
			const errs = k.errors ? ` · ${k.errors} erreur${k.errors > 1 ? 's' : ''}` : '';
			return `${col} · ${typeLabel(k.type)} (${k.fixed ? 'fixé' : 'inféré'})${errs}` + (e.t.twoLevel && singleCol(e, c - e.t.col) ? ' · sur un niveau' : '');
		}
		const bad = e.cache.row(r - e.t.row - 1)?.bad?.get(c - e.t.col);
		return `${col} · ${r - e.t.row}` + (bad !== undefined ? ` · « ${bad} » n'est pas du type ${typeLabel(k.type)}` : '');
	}

	function refreshStatus() {
		syncFinderZone();
		if (dvPick) dvPickRef = pickedRef() ?? '';
		else prefetchDv();
		if (message !== shownMessage) {
			shownMessage = message;
			clearTimeout(messageTimer);
			if (message) {
				const m = message;
				messageTimer = setTimeout(() => {
					if (message === m) clearMessage();
				}, noticeTtl(m));
			}
		}
		const stats = entries[0]?.cache.stats ?? { roundTrip: [], server: [] };
		const here = ownerAt(active.row, active.col);
		onstatus?.({
			note: !!noteOf(active.row, active.col),
			filters: !!here?.t.columns.some((c) => c.filter),
			errorsOnly: !!here?.t.columns[active.col - here.t.col]?.filter?.errors,
			readOnly: here?.t.readOnly,
			// Une cellule libre : son adresse est déjà dans la zone de nom.
			cell: here || names.has(key(active.row, active.col)) ? cellName(active.row, active.col) : '',
			message,
			fetch: `bloc p50 ${ms(percentile(stats.roundTrip, 0.5))}`,
			draw: `rendu ${ms(percentile(drawTimes, 0.5))}`,
			fps: `${frameStamps.length} img/s`,
			edit: `saisie ${ms(lastEditMs)}`,
			blocks: entries.reduce((n, e) => n + e.cache.size, 0),
			stats: selectionStats(),
			sel: selection(),
			style: activeStyle(),
			merged: mergedAt(active.row, active.col)
		});
	}

	// Totaux d'une grande sélection (ou de lignes pas encore chargées) :
	// calculés en SQL par le moteur, en tâche de fond ; la barre d'état les
	// affiche dès qu'ils arrivent. statsEpoch change avec les données.
	let statsEpoch = 0;
	let statsCache: { key: string; text: string } | null = null;
	let statsPending = '';
	let statsTimer = 0;

	function statsText(count: number, nums: number, sum: number): string {
		if (count === 0) return '';
		const parts = [];
		// Comme Excel : au format de la cellule active, sinon arrondies.
		let st = styleAt(active.row, active.col);
		const e = tableAt(active.row, active.col);
		const col = e?.t.columns[active.col - e.t.col];
		// Colonne décimale sans format : deux décimales, comme ses cellules.
		if (col && st?.nf === undefined && st?.dp === undefined && (col.type === 'decimal' || (col.type === 'formula' && col.result === 'decimal'))) st = { ...st, dp: 2 };
		if (nums) parts.push(`Moyenne : ${formatStat(sum / nums, st)}`);
		parts.push(`Nb : ${count.toLocaleString('fr-FR')}`);
		if (nums) parts.push(`Somme : ${formatStat(sum, st)}`);
		return parts.join('   ');
	}

	/** Totaux de la sélection par le moteur (SQL) : cellules libres et en-têtes ici, données là-bas. */
	function remoteStats(r0: number, r1: number, c0: number, c1: number): string {
		const rs = source.rangeStats;
		if (!rs) return '';
		const key = `${statsEpoch}|${r0},${r1},${c0},${c1}`;
		if (statsCache?.key === key) return statsCache.text;
		if (statsPending !== key) {
			statsPending = key;
			window.clearTimeout(statsTimer);
			statsTimer = window.setTimeout(async () => {
				let count = 0;
				let nums = 0;
				let sum = 0;
				try {
					for (const e of entries) {
						if (e.t.row > r1 || e.last < r0 || e.t.col > c1 || e.lastCol < c0) continue;
						const ka = Math.max(c0, e.t.col) - e.t.col;
						const kb = Math.min(c1, e.lastCol) - e.t.col;
						if (r0 <= e.t.row && e.t.row <= r1) count += kb - ka + 1; // en-têtes
						const first = Math.max(r0, e.t.row + 1) - e.t.row - 1;
						const last = Math.min(r1, e.last) - e.t.row - 1;
						if (last < first) continue;
						const cols = Array.from({ length: kb - ka + 1 }, (_, i) => ka + i);
						const st = await rs(e.t, first, last - first + 1, cols);
						count += st.count;
						nums += st.nums;
						sum += st.sum;
					}
				} catch {
					// Échec : pas de nouvel essai pour cette sélection (pas de boucle).
					if (statsPending === key) {
						statsPending = '';
						statsCache = { key, text: '' };
						refreshStatus();
					}
					return;
				}
				// Cellules libres : toutes en mémoire.
				for (const [k, raw] of cells) {
					const [r, c] = k.split(',').map(Number);
					if (r < r0 || r > r1 || c < c0 || c > c1 || tableAt(r, c)) continue;
					const f = shown.get(k);
					let v: unknown = f ? f.text : raw;
					if (v === null || v === undefined || v === '') continue;
					count++;
					if (typeof v === 'string' && isNumeric(v)) v = Number(v.replace(/[\s\u00a0\u202f]/g, '').replace(',', '.'));
					if (typeof v === 'number' && Number.isFinite(v)) {
						nums++;
						sum += v;
					}
				}
				if (statsPending !== key) return; // sélection changée entre-temps
				statsPending = '';
				statsCache = { key, text: statsText(count, nums, sum) };
				refreshStatus();
			}, 150);
		}
		return statsCache && statsCache.key.endsWith(`|${r0},${r1},${c0},${c1}`) ? statsCache.text : 'Calcul des totaux…';
	}

	/** « Moyenne · Nb · Somme » de la sélection, si plusieurs cellules. */
	function selectionStats(): string {
		const { r0, r1, c0, c1 } = selection();
		const n = (r1 - r0 + 1) * (c1 - c0 + 1);
		if (n < 2 || singleSelection()) return '';
		// Grande sélection (colonne entière d'un million de lignes…) : SQL.
		if (n > 50_000) return source.rangeStats ? remoteStats(r0, r1, c0, c1) : 'Sélection trop grande pour les totaux';
		let count = 0;
		let nums = 0;
		let sum = 0;
		let missing = false;
		for (let r = r0; r <= r1; r++) {
			for (let c = c0; c <= c1; c++) {
				const e = tableAt(r, c);
				let v: unknown;
				if (e) {
					if (r === e.t.row) {
						count++;
						continue;
					}
					const row = e.cache.row(r - e.t.row - 1);
					if (!row) {
						missing = true;
						continue;
					}
					v = row.bad?.has(c - e.t.col) ? row.bad.get(c - e.t.col) : row.cells[c - e.t.col];
				} else {
					// Tableau crois\u00e9 : ses cellules calcul\u00e9es comptent aussi (nombre, somme, moyenne).
					const p = pivotAt(r, c);
					const pv = p && r >= p.row ? p.cells[r - p.row]?.[c - p.col] : undefined;
					const f = shown.get(key(r, c));
					v = pv !== undefined ? pv : f ? f.text : cells.get(key(r, c));
					if (typeof v === 'string' && isNumeric(v)) v = Number(v.replace(/[\s\u00a0\u202f]/g, '').replace(',', '.'));
				}
				if (v === null || v === undefined || v === '') continue;
				count++;
				if (typeof v === 'number' && Number.isFinite(v)) {
					nums++;
					sum += v;
				}
			}
		}
		// Lignes pas encore chargées : le moteur fait le compte exact.
		if (missing && source.rangeStats) return remoteStats(r0, r1, c0, c1);
		return statsText(count, nums, sum) + (missing ? ' (lignes en chargement)' : '');
	}

	// --- Filtres d'un TCD (▾ dans la grille, comme Excel) ---------------------------

	let pivotDlg = $state<{
		p: SheetPivot;
		field: string;
		/** filter : filtre de rapport ; rows : étiquette d'un champ des lignes (tri en plus). */
		kind: 'filter' | 'rows';
		/** Arborescence : champs des lignes au choix (un seul ▾ sur « Étiquettes de lignes »). */
		fields?: string[];
		x: number;
		y: number;
		values: PanelValue[];
		loading: boolean;
		search: string;
	} | null>(null);

	/** Le clic tombe-t-il sur un ▾ de TCD ? Ouvre alors son menu. */
	function openPivotMenu(ev: MouseEvent, cell: { row: number; col: number }): boolean {
		const p = pivotAt(cell.row, cell.col);
		if (!p || !source.setPivot) return false;
		const near = screenX(cell.col) + cax().size(cell.col) - toLocal(ev).x <= PIVOT_DROP + 2;
		if (!near) return false;
		const a = p.anchor ?? p.row;
		const filters = p.config.filters ?? [];
		let field: string | undefined;
		let kind: 'filter' | 'rows' = 'filter';
		if (cell.row - a < filters.length && cell.col === p.col + 1) field = filters[cell.row - a].field;
		else if (cell.row === p.row && cell.col - p.col < (p.rows ? Math.min(1, p.config.rowFields?.length ?? 0) : (p.config.rowFields?.length ?? 0))) {
			field = p.config.rowFields![cell.col - p.col];
			kind = 'rows';
		}
		if (field === undefined) return false;
		moveTo(cell.row, cell.col);
		closeMenus();
		pivotDlg = {
			p,
			field,
			kind,
			// Arborescence : un seul ▾ pour tous les champs des lignes (« Sélectionner un champ », Excel).
			fields: kind === 'rows' && p.rows ? [...(p.config.rowFields ?? [])] : undefined,
			x: Math.max(4, Math.min(toView(screenX(cell.col)), toView(scroller.clientWidth) - 280)),
			y: toView(screenY(cell.row) + rh(cell.row)),
			values: [],
			loading: !!source.pivotFieldValues,
			search: ''
		};
		loadPivotValues();
		return true;
	}

	/** Charge les valeurs du champ du menu ▾ ouvert (cochées : celles gardées). */
	function loadPivotValues() {
		const d = pivotDlg;
		if (!d) return;
		const { p, field, kind } = d;
		const kept = (kind === 'filter' ? (p.config.filters ?? []) : (p.config.items ?? [])).find((x) => x.field === field)?.values ?? [];
		const keep = kept.length ? new Set(kept) : null;
		d.values = [];
		d.loading = !!source.pivotFieldValues;
		const mine = () => pivotDlg?.p.id === p.id && pivotDlg.field === field;
		source.pivotFieldValues?.(p.config.source, field).then(
			(vals) => {
				if (!mine()) return;
				pivotDlg!.values = vals.map((v) => ({ ...v, on: !keep || keep.has(v.text) }));
				pivotDlg!.loading = false;
			},
			(err) => {
				if (mine()) pivotDlg!.loading = false;
				say(String(err instanceof Error ? err.message : err));
			}
		);
	}

	/** Enregistre la configuration du TCD du menu ▾ (annulable comme un changement du volet). */
	function savePivot(cfg: PivotConfig) {
		const d = pivotDlg;
		pivotDlg = null;
		if (!d || !source.setPivot) return;
		const set = source.setPivot;
		menuAction(() => set(d.p.id, cfg));
	}

	function sortPivot(dir: 'asc' | 'desc') {
		const d = pivotDlg;
		if (!d) return;
		const cfg: PivotConfig = { ...d.p.config, sort: { ...d.p.config.sort, [d.field]: dir }, orders: withoutField(d.p.config.orders, d.field) };
		if (!cfg.orders) delete cfg.orders;
		savePivot(cfg);
	}

	// --- Tri personnalisé… ------------------------------------------------------

	/** Dialogue « Tri personnalisé » ouvert : champ, valeurs, ordre enregistré et application. */
	let customDlg = $state<{
		field: string;
		loadValues: () => Promise<string[]>;
		loadOrder?: () => Promise<CustomOrder | null>;
		apply: (o: CustomOrder) => void;
	} | null>(null);

	/** Tri personnalisé… d'une colonne de tableau (menus ▾ et clic droit, ruban Données). */
	function openCustomSort(e: Entry, i: number) {
		const run = source.sortTableCustom;
		if (!run) return;
		const t = e.t;
		menu = null;
		filterDlg = null;
		customDlg = {
			field: t.columns[i]?.name ?? '',
			loadValues: async () => ((await source.filterValues?.(t, i)) ?? []).map((v) => v.text),
			loadOrder: source.columnOrder ? () => source.columnOrder!(t, i) : undefined,
			apply: (o) => {
				menuAction(() => run(t, i, o)).then((ok) => ok && say('Tri personnalisé appliqué (Ctrl+Z pour annuler)'));
			}
		};
	}

	/** Tri personnalisé… du champ du menu ▾ d'un TCD : enregistré dans sa configuration. */
	function openPivotCustomSort() {
		const d = pivotDlg;
		if (!d || !source.setPivot) return;
		const { p, field } = d;
		const set = source.setPivot;
		pivotDlg = null;
		customDlg = {
			field,
			loadValues: async () => ((await source.pivotFieldValues?.(p.config.source, field)) ?? []).map((v) => v.text),
			loadOrder: async () => p.config.orders?.[field] ?? null,
			apply: (o) => {
				const cfg: PivotConfig = { ...p.config, orders: { ...p.config.orders, [field]: o }, sort: withoutField(p.config.sort, field) };
				if (!cfg.sort) delete cfg.sort;
				menuAction(() => set(p.id, cfg));
			}
		};
	}

	/** Tri personnalisé de la colonne active (ruban : Trier et filtrer). */
	export function sortCustomActive() {
		const e = targetTable();
		if (!e || !source.sortTableCustom) {
			say('Placez la cellule active dans la colonne d’un tableau pour trier');
			return;
		}
		openCustomSort(e, targetCol(e));
	}

	/** OK : valeurs cochées gardées (toutes : plus de filtre) ; clear : efface le filtre et le tri du champ. */
	function applyPivotFilter(clear = false) {
		const d = pivotDlg;
		if (!d) return;
		const on = d.values.filter((v) => v.on).map((v) => v.text);
		if (!clear && !on.length) {
			say('Choisissez au moins une valeur');
			return;
		}
		const values = clear || on.length === d.values.length ? [] : on;
		const cfg = { ...d.p.config };
		if (d.kind === 'filter') {
			cfg.filters = (cfg.filters ?? []).map((f) => (f.field === d.field ? { field: f.field, values } : f));
		} else {
			const rest = (cfg.items ?? []).filter((x) => x.field !== d.field);
			cfg.items = values.length ? [...rest, { field: d.field, values }] : rest;
			if (!cfg.items.length) delete cfg.items;
			if (clear && cfg.sort) {
				const { [d.field]: _, ...sort } = cfg.sort;
				cfg.sort = Object.keys(sort).length ? sort : undefined;
			}
			if (clear && cfg.orders) cfg.orders = withoutField(cfg.orders, d.field);
		}
		savePivot(cfg);
	}

	// --- Filtre -----------------------------------------------------------------

	let filterDlg = $state<{
		e: Entry;
		i: number;
		x: number;
		y: number;
		values: PanelValue[];
		/** Liste des valeurs en cours de chargement (grand tableau). */
		loading?: boolean;
		search: string;
		errors: boolean;
		error: string;
		touched: boolean; // liste ou recherche modifiée
		sub: boolean; // sous-menu « Filtres textuels / numériques » ouvert
		// Filtre personnalisé en cours (deux conditions au plus, comme Excel).
		custom: { conds: { op: CondOp | ''; value: string; to: string }[]; or: boolean } | null;
		// Section « Type de données » : conversion en attente de confirmation,
		// raison pour laquelle la colonne ne peut pas être clé, valeurs hors type.
		pending?: { to: ColumnType; n: number };
		keyWhy?: string | null;
		mix?: TypeMix | null;
	} | null>(null);

	/** Position d'un panneau ancré sous l'en-tête de la colonne i du tableau e
	 * (sous le bandeau quand l'en-tête est figé), gardé dans la grille (width :
	 * largeur du panneau). */
	function anchorUnder(e: Entry, i: number, width: number) {
		// Hors du calque zoomé : en pixels de l'écran (toView).
		const x = toView(screenX(e.t.col + i));
		return {
			x: Math.max(4, Math.min(x, toView(scroller.clientWidth) - width - 4)),
			y: toView(clamp(screenY(e.t.row) + rh(e.t.row), TOP_H, Math.max(TOP_H, scroller.clientHeight - 200 / zoom)))
		};
	}

	/** Met le focus sur le premier champ d'un panneau à son ouverture. */
	function focusFirst(node: HTMLElement) {
		requestAnimationFrame(() => (node.querySelector<HTMLElement>('.add input, .add textarea') ?? node.querySelector<HTMLElement>('.add button'))?.focus());
	}

	/**
	 * Menu ▾ d'une colonne de tableau, comme celui d'Excel (le ▾ et l'icône de
	 * type de l'en-tête, figé ou non, et le clic droit l'ouvrent) : tri,
	 * filtres (conditions, recherche, liste à cocher, OK / Annuler), puis le
	 * type de données et la clé du tableau.
	 */
	async function openFilter(e: Entry, i: number) {
		closeMenus();
		const f = e.t.columns[i]?.filter;
		const keep = f?.values ? new Set(f.values) : null;
		const lists = !!source.filterValues;
		const checksKey = !!source.keyCheck && e.t.key !== i;
		// Le menu s'ouvre tout de suite (tri, filtres, type) ; la liste des
		// valeurs et le contrôle de la clé, longs sur un grand tableau,
		// arrivent ensuite à la place de leur indicateur de chargement.
		filterDlg = {
			e,
			i,
			...anchorUnder(e, i, 310),
			values: [],
			loading: lists,
			search: '',
			errors: !!f?.errors,
			error: '',
			touched: false,
			sub: false,
			custom: null,
			keyWhy: checksKey ? undefined : null
		};
		const mine = () => filterDlg !== null && filterDlg.e.t.id === e.t.id && filterDlg.i === i;
		mixOf(e, i).then((mix) => mine() && (filterDlg!.mix = mix));
		if (checksKey)
			source.keyCheck!(e.t, i).then(
				(why) => mine() && (filterDlg!.keyWhy = why),
				() => mine() && (filterDlg!.keyWhy = null)
			);
		if (lists)
			source.filterValues!(e.t, i).then(
				(vals) => {
					if (!mine()) return;
					filterDlg!.values = vals.map((v) => ({ ...v, on: !keep || keep.has(v.text) }));
					filterDlg!.loading = false;
				},
				(err) => {
					if (mine()) filterDlg!.loading = false;
					say(String(err instanceof Error ? err.message : err));
				}
			);
	}

	/** Filtre de la colonne de la cellule active (ruban Données). */
	export function filterActive(errorsOnly = false) {
		const e = targetTable();
		if (!e || !source.setFilter) {
			say('Placez la cellule active dans la colonne d’un tableau pour filtrer');
			return;
		}
		const i = targetCol(e);
		if (errorsOnly) {
			const f = e.t.columns[i]?.filter;
			const set = source.setFilter;
			const rest: ColumnFilter = { ...f };
			delete rest.errors;
			menuAction(() => set(e.t, i, f?.errors ? (rest.values || rest.conds?.length ? rest : null) : { ...f, errors: true }));
			return;
		}
		openFilter(e, i);
	}

	/** Ctrl+Maj+L : efface les filtres du tableau actif, sinon ouvre celui de la colonne. */
	function toggleFilters() {
		const e = ownerAt(active.row, active.col);
		if (e && source.clearFilters && e.t.columns.some((c) => c.filter)) {
			const clear = source.clearFilters;
			menuAction(() => clear(e.t));
			return;
		}
		filterActive();
	}

	function filterShown() {
		const d = filterDlg;
		return d ? searchValues(d.values, d.search) : [];
	}

	/** Filtre personnalisé : première condition op (sous-menu), ou les conditions du filtre actif. */
	function startCustom(op?: CondOp) {
		const d = filterDlg;
		if (!d) return;
		d.sub = false;
		d.error = '';
		const cur = d.e.t.columns[d.i]?.filter;
		if (op && condArgs(op) === 0) {
			d.custom = { conds: [{ op, value: '', to: '' }], or: false };
			applyFilter();
			return;
		}
		const conds = op
			? [{ op, value: op === 'top' || op === 'bottom' ? '10' : '', to: '' }]
			: (cur?.conds ?? []).map((c) => ({ op: c.op as CondOp | '', value: c.value ?? '', to: c.to ?? '' }));
		while (conds.length < 2) conds.push({ op: conds.length ? '' : 'eq', value: '', to: '' });
		d.custom = { conds, or: !op && !!cur?.or };
	}

	async function applyFilter(clear = false) {
		const d = filterDlg;
		if (!d || !source.setFilter) return;
		let f: ColumnFilter | null = null;
		const cur = d.e.t.columns[d.i]?.filter;
		if (clear) f = null;
		else if (d.custom) {
			const conds = d.custom.conds
				.filter((c) => c.op && (condArgs(c.op) === 0 || c.value.trim() !== ''))
				.map((c) => ({ op: c.op as CondOp, ...(condArgs(c.op as CondOp) ? { value: c.value } : {}), ...(condArgs(c.op as CondOp) === 2 ? { to: c.to } : {}) }));
			if (!conds.length) {
				d.error = 'Saisissez la valeur de la condition';
				return;
			}
			f = { conds };
			if (d.custom.or && conds.length > 1) f.or = true;
			if (d.errors) f.errors = true;
		} else if (!d.touched && cur?.conds?.length) {
			// Liste pas touchée : le filtre par condition reste.
			f = { ...cur };
			if (d.errors) f.errors = true;
			else delete f.errors;
		} else {
			const r = valuesFilter(d.values, d.search, d.errors);
			if (r.error) {
				d.error = r.error;
				return;
			}
			f = r.filter;
		}
		try {
			await source.setFilter(d.e.t, d.i, f);
			closeFilter();
		} catch (err) {
			d.error = String(err instanceof Error ? err.message : err);
		}
	}

	function closeFilter() {
		filterDlg = null;
		wrap.focus();
	}

	// Menu ▾ fermé par un clic ailleurs : si ce clic est sur le ▾ de la même
	// colonne, il ne le rouvre pas (le ▾ ouvre et ferme son menu, comme Excel).
	let filterClosed = { key: '', at: 0 };
	function dismissFilter() {
		const d = filterDlg;
		if (d) filterClosed = { key: `${d.e.t.id}:${d.i}`, at: performance.now() };
		filterDlg = null;
	}

	// --- Mise en forme conditionnelle -----------------------------------------
	// Menu du ruban, petites boîtes, nouvelle règle et gestionnaire des règles :
	// CondFormat.svelte ; la feuille lui donne ses règles et sa sélection.

	let cfView = $state<CondView | null>(null);

	/** Règles de la feuille, par emplacement (gestionnaire). */
	function sheetRules(): SheetRules {
		return {
			tables: entries.map((e) => ({ id: e.t.id, name: e.t.name, row: e.t.row, col: e.t.col, rows: e.t.rows, columns: e.t.columns.map((c) => c.name), rules: colRules.get(e.t.id) ?? [] })),
			rows: sheetRowRules,
			cols: sheetColRules,
			cells: cellRules
		};
	}

	/** Emplacements des règles de la sélection : lignes ou colonnes entières, colonnes de tableau, cellules libres. */
	function selectionLocs(): Loc[] | string {
		const { r0, r1, c0, c1 } = selection();
		const lines = selectionLines();
		if (lines.rows) return Array.from({ length: r1 - r0 + 1 }, (_, k) => ({ k: 'row' as const, i: r0 + k }));
		if (lines.cols) return Array.from({ length: c1 - c0 + 1 }, (_, k) => ({ k: 'col' as const, i: c0 + k }));
		return rectLocs(r0, c0, r1, c1, sheetRules().tables);
	}

	const cfHost: CondHost = {
		sheetRules,
		selectionLocs,
		selection: () => selection(),
		activeTable: () => {
			const e = ownerAt(active.row, active.col);
			return e ? sheetRules().tables.find((t) => t.id === e.t.id) : undefined;
		},
		async apply(ch: RuleChanges) {
			if (!source.setSheetRules) throw new Error('Mise en forme conditionnelle indisponible');
			await source.setSheetRules(ch);
		}
	};

	/** Menu « Mise en forme conditionnelle » du ruban, sous son bouton (coordonnées de la fenêtre). */
	export function rulesMenu(x: number, y: number) {
		if (!source.setSheetRules) return;
		if (locked) return say(locked);
		closeMenus();
		cfView = { kind: 'menu', x, y };
	}

	/** Gestionnaire des règles de la sélection. */
	export function rulesActive() {
		if (!source.setSheetRules) return;
		if (locked) return say(locked);
		closeMenus();
		cfView = { kind: 'manager' };
	}

	/** Gestionnaire des règles d'une colonne de tableau (menu ▾ de la colonne). */
	function openRules(e: Entry, i: number) {
		selectRange(e.t.row + 1, e.t.col + i, Math.max(e.t.row + 1, e.last), e.t.col + i);
		rulesActive();
	}

	/** « S'applique à » de la sélection (titre des boîtes). */
	export function rulesTarget(): string {
		const l = selectionLocs();
		return typeof l === 'string' ? '' : formatLocs(l, sheetRules().tables);
	}

	function closeRules() {
		cfView = null;
		wrap.focus();
	}

	// --- Graphiques -------------------------------------------------------------

	const CHART_W = 480;
	const CHART_H = 288;
	// Graphique sélectionné (cadre, poignées, volet) et redimensionnement en cours.
	let selChart = $state<number | null>(null);
	let chartMenu = $state<{ ch: SheetChart; x: number; y: number } | null>(null);

	function selectChart(ch: SheetChart) {
		selChart = ch.id;
		panel = { kind: 'chart', id: ch.id };
		wrap.focus();
	}

	/** Déplace le graphique sélectionné au clavier (resize : le redimensionne), de dx, dy pixels. */
	function nudgeChart(ch: SheetChart, dx: number, dy: number, resize: boolean) {
		if (!source.placeChart) return;
		const b = { ...chartBox(ch) };
		if (resize) {
			b.w = Math.max(160, b.w + dx);
			b.h = Math.max(120, b.h + dy);
		} else {
			b.x = Math.max(0, b.x + dx);
			b.y = Math.max(0, b.y + dy);
		}
		const col = cax().indexAt(b.x);
		const row = rax().indexAt(b.y);
		const place = source.placeChart;
		writing = writing.catch(() => undefined).then(() =>
			place(ch.id, row, col, Math.round(b.x - cax().offset(col)), Math.round(b.y - rowTop(row)), Math.round(b.w), Math.round(b.h)).then(
				() => undefined,
				(err) => say(String(err?.message ?? err))
			)
		);
	}

	/** Colonnes du tableau source d'un graphique ou d'un TCD (autre feuille comprise). */
	function sourceColumns(name: string): { name: string; numeric: boolean; type: string }[] {
		const e = entries.find((x) => x.t.name.toLowerCase() === name.toLowerCase());
		if (e) return e.t.columns.map((c) => ({ name: c.name, type: c.type, numeric: c.type === 'integer' || c.type === 'decimal' || c.type === 'formula' }));
		const t = sources.find((x) => x.name.toLowerCase() === name.toLowerCase());
		return (t?.columns ?? []).map((c, i) => ({ name: c, type: t!.types[i], numeric: ['integer', 'decimal', 'formula'].includes(t!.types[i]) }));
	}

	/** Position et taille d'un graphique, en pixels de la feuille (ancrage libre). */
	function chartBox(ch: SheetChart) {
		if (chartGesture?.id === ch.id) return chartGesture.box;
		return {
			x: cax().offset(ch.col) + (ch.config.offX ?? 0),
			y: rowTop(ch.row) + (ch.config.offY ?? 0),
			w: ch.config.width ?? CHART_W,
			h: ch.config.height ?? CHART_H
		};
	}

	/** Position et taille d'un bouton ou contrôle, en pixels de la feuille (comme SheetButtons). */
	function buttonBox(b: SheetButton) {
		return {
			x: cax().offset(b.col) + (b.config.offX ?? 0),
			y: rowTop(b.row) + (b.config.offY ?? 0),
			w: b.config.width || BUTTON_W,
			h: b.config.height || BUTTON_H
		};
	}

	let chartGesture = $state<{ id: number; box: { x: number; y: number; w: number; h: number } } | null>(null);

	/** Déplace (dir vide) ou redimensionne (n, s, e, w et coins) un graphique. */
	function startChartGesture(ev: MouseEvent, ch: SheetChart, dir = '') {
		if (locked) {
			ev.preventDefault(); // feuille protégée : le graphique ne se sélectionne pas
			return;
		}
		if (ev.button !== 0 || !source.placeChart) return;
		ev.preventDefault();
		ev.stopPropagation();
		selectChart(ch);
		const b0 = chartBox(ch);
		const x0 = ev.clientX;
		const y0 = ev.clientY;
		chartGesture = { id: ch.id, box: { ...b0 } };
		const move = (e: MouseEvent) => {
			if (!chartGesture) return;
			const k = zoomK();
			const dx = (e.clientX - x0) / k;
			const dy = (e.clientY - y0) / k;
			const b = { ...b0 };
			if (!dir) {
				b.x = Math.max(0, b0.x + dx);
				b.y = Math.max(0, b0.y + dy);
			}
			if (dir.includes('e')) b.w = Math.max(160, b0.w + dx);
			if (dir.includes('s')) b.h = Math.max(120, b0.h + dy);
			if (dir.includes('w')) {
				const w = Math.max(160, b0.w - dx);
				b.x = Math.max(0, b0.x + b0.w - w);
				b.w = b0.x + b0.w - b.x;
			}
			if (dir.includes('n')) {
				const h = Math.max(120, b0.h - dy);
				b.y = Math.max(0, b0.y + b0.h - h);
				b.h = b0.y + b0.h - b.y;
			}
			chartGesture.box = b;
		};
		const up = () => {
			window.removeEventListener('mousemove', move);
			window.removeEventListener('mouseup', up);
			const g = chartGesture;
			if (!g || (g.box.x === b0.x && g.box.y === b0.y && g.box.w === b0.w && g.box.h === b0.h)) {
				chartGesture = null;
				return;
			}
			const b = g.box;
			const col = cax().indexAt(b.x);
			const row = rax().indexAt(b.y);
			const place = source.placeChart!;
			writing = writing.catch(() => undefined).then(() =>
				place(ch.id, row, col, Math.round(b.x - cax().offset(col)), Math.round(b.y - rowTop(row)), Math.round(b.w), Math.round(b.h)).then(
					() => {
						chartGesture = null;
					},
					(err) => {
						chartGesture = null;
						say(String(err?.message ?? err));
					}
				)
			);
		};
		window.addEventListener('mousemove', move);
		window.addEventListener('mouseup', up);
	}

	function onChartMenu(ev: MouseEvent, ch: SheetChart) {
		ev.preventDefault();
		ev.stopPropagation();
		if (locked && !hold) return;
		selectChart(ch);
		const r = wrap.getBoundingClientRect();
		closeMenus();
		chartMenu = { ch, x: ev.clientX - r.left, y: ev.clientY - r.top };
	}

	async function copyChart(ch: SheetChart, cut: boolean) {
		chartMenu = null;
		try {
			await navigator.clipboard.writeText(encodeObj({ kind: 'chart', config: ch.config as never }));
		} catch {
			chartClip = ch.config;
		}
		chartClip = ch.config;
		if (cut) await removeChart(ch.id);
		else say('Graphique copié : Ctrl+V pour le coller à la cellule active');
	}
	let chartClip = $state.raw<ChartConfig | null>(null);

	/** Copie le graphique comme image PNG (à coller dans un courriel, un document). */
	async function copyChartImage(ch: SheetChart) {
		chartMenu = null;
		const svg = wrap.querySelector<SVGSVGElement>('.chart.selected svg');
		if (!svg || selChart !== ch.id) return;
		try {
			const blob = await svgToPng(svg);
			await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]);
			say('Graphique copié comme image : collez-le dans un courriel ou un document');
		} catch (err) {
			say(`Copie de l’image impossible : ${err instanceof Error ? err.message : err}`);
		}
	}

	/** Après la création d'un graphique : le nouveau est sélectionné (Excel), prêt à être déplacé, redimensionné ou mis en forme. */
	function selectNewChart(before: Set<number>): SheetChart | undefined {
		const ch = charts.find((c) => !before.has(c.id));
		if (!ch) return;
		selChart = ch.id;
		// Amené à l'écran, en entier s'il tient (son coin haut gauche sinon).
		updateExtent();
		const b = chartBox(ch);
		const t = reveal(b.y, b.y + b.h, vs.top, vs.view, rowTop(fRows()));
		const l = reveal(b.x, b.x + b.w, scroller.scrollLeft, viewW - MARGIN, cax().offset(fCols()));
		wrap.focus();
		glideTo(t, l);
		return ch;
	}

	/** Défile jusqu'à (top, left) : en douceur avec les animations (Affichage › Animations), d'un coup sinon. */
	function glideTo(top: number, left: number) {
		const t0 = vs.top;
		const l0 = scroller.scrollLeft;
		if (top === t0 && left === l0) return schedule();
		const set = (t: number, l: number) => {
			if (t !== vs.top) vs.to(t);
			if (l !== scroller.scrollLeft) scroller.scrollLeft = l;
			syncNative();
			schedule();
		};
		if (!motion()) return set(top, left);
		const start = performance.now();
		const D = 320;
		const step = (now: number) => {
			const k = Math.min(1, (now - start) / D);
			const e = 1 - Math.pow(1 - k, 3); // décélération
			set(Math.round(t0 + (top - t0) * e), Math.round(l0 + (left - l0) * e));
			if (k < 1) requestAnimationFrame(step);
		};
		requestAnimationFrame(step);
	}

	/**
	 * Place libre pour un nouveau graphique (w × h px) à partir de (row, col) :
	 * sous les graphiques qu'il chevaucherait, avec une ligne d'écart, pour
	 * qu'aucun ne se superpose par défaut.
	 */
	function freeChartSpot(row: number, col: number, w = CHART_W, h = CHART_H): { row: number; col: number } {
		const x = colAxis.offset(col);
		for (let k = 0; k < 50; k++) {
			const y = rowTop(row);
			const hit = charts.map(chartBox).find((b) => x < b.x + b.w && b.x < x + w && y < b.y + b.h && b.y < y + h);
			if (!hit) break;
			row = rowAxis.indexAt(hit.y + hit.h) + 2;
		}
		return { row, col };
	}

	/** Carte zoomée ou déplacée (MapChart) : vue enregistrée dans sa configuration (annulable). */
	function setMapView(id: number, v: ChartConfig['mapView'] | null) {
		const ch = charts.find((c) => c.id === id);
		if (!ch || !source.setChart) return;
		const set = source.setChart;
		writing = writing.catch(() => undefined).then(() => set(id, { ...ch.config, mapView: v ?? undefined }).then(undefined, (err) => say(String(err?.message ?? err))));
	}

	/** Point de carte cliqué : la carte se désélectionne (comme un clic dans la grille), puis la ligne est sélectionnée. */
	function pointToRow(ch: SheetChart, id: number) {
		if (selChart === ch.id) selChart = null;
		if (panel?.kind === 'chart') panel = null;
		onrowlink?.(ch.config.source, id);
	}

	function pasteChart(cfg: ChartConfig) {
		if (!source.createChart) return;
		const create = source.createChart;
		const { row, col } = active;
		const before = new Set(charts.map((c) => c.id));
		writing = writing.catch(() => undefined).then(() =>
			create(freeChartSpot(row, col, cfg.width, cfg.height).row, col, cfg).then(
				() => {
					say('Graphique collé');
					selectNewChart(before);
				},
				(err) => say(String(err?.message ?? err))
			)
		);
	}

	/** Insère un graphique du tableau actif à sa droite (ruban Insertion). */
	export function chartActive(kind: ChartConfig['kind']) {
		const pv = pivotAt(active.row, active.col);
		if (pv && source.createChart) {
			// Graphique d'un tableau croisé : X = premier champ de lignes, séries = mesures.
			const head = pv.cells[0];
			// En arborescence, une seule colonne d'étiquettes ; le graphique lit le TCD à plat (premier champ).
			const nk = pv.rows ? 1 : Math.max(1, pv.config.rowFields?.length ?? 0);
			const x = pv.rows ? (pv.config.rowFields?.[0] ?? head[0]) : head[0];
			let series = head.slice(nk).filter((h) => !h.startsWith('Total'));
			if (SINGLE_SERIES.has(kind)) series = series.slice(0, 1);
			const create = source.createChart;
			const name = pv.config.name ?? '';
			const before = new Set(charts.map((c) => c.id));
			writing = writing.catch(() => undefined).then(() =>
				create(freeChartSpot(pv.row, pv.col + head.length + 1).row, pv.col + head.length + 1, { source: name, kind, x, series, title: defaultChartTitle(series) }).then(
					() => {
						say(`Graphique du tableau croisé « ${name} » inséré`);
						selectNewChart(before);
					},
					(err) => say(String(err?.message ?? err))
				)
			);
			return;
		}
		const e = targetTable();
		if (!e || !source.createChart) {
			say('Placez la cellule active dans un tableau pour insérer un graphique');
			return;
		}
		const cols = e.t.columns;
		const num = (c: Column) => c.type === 'integer' || c.type === 'decimal' || (c.type === 'formula' && c.formula !== undefined);
		// Plusieurs colonnes du tableau sélectionnées : elles fixent X et les séries (comme Excel).
		const { c0, c1 } = selection();
		const picked: number[] = [];
		for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol); c++) picked.push(c - e.t.col);
		const { x: xName, series } = chartFieldsFrom(kind, cols.map((c) => ({ name: c.name, numeric: num(c) })), picked, targetCol(e));
		const x = cols.find((c) => c.name === xName) ?? cols[0];
		if (kind === 'map') {
			// Carte : un carré grisé, en attente de ses coordonnées ; le volet
			// s'ouvre avec les colonnes repérées par le classeur.
			const create = source.createChart;
			const before = new Set(charts.map((c) => c.id));
			const side = CHART_H + 32;
			const at = freeChartSpot(e.t.row, e.lastCol + 2, side, side);
			writing = writing.catch(() => undefined).then(() =>
				create(at.row, e.lastCol + 2, { source: e.t.name, kind, x: '', series: [], mapDraft: true, width: side, height: side }).then(
					() => {
						say('Carte insérée : choisissez ses coordonnées dans le volet');
						const ch = selectNewChart(before);
						if (ch) panel = { kind: 'chart', id: ch.id };
					},
					(err) => say(String(err?.message ?? err))
				)
			);
			return;
		}
		if (!series.length) {
			say('Le tableau n’a pas de colonne de nombres à représenter');
			return;
		}
		const create = source.createChart;
		const before = new Set(charts.map((c) => c.id));
		writing = writing.catch(() => undefined).then(() =>
			create(freeChartSpot(e.t.row, e.lastCol + 2).row, e.lastCol + 2, { source: e.t.name, kind, x: x.name, series, title: defaultChartTitle(series) }).then(
				() => {
					say('Graphique inséré : cliquez dessus pour le modifier, glissez-le pour le déplacer');
					selectNewChart(before);
				},
				(err) => say(String(err?.message ?? err))
			)
		);
	}

	function editChart(ch: SheetChart) {
		if (locked && !hold) return;
		selectChart(ch);
	}
	async function removeChart(id: number) {
		chartMenu = null;
		if (selChart === id) selChart = null;
		if (panel?.kind === 'chart' && panel.id === id) panel = null;
		await source.deleteChart?.(id);
		wrap.focus();
	}


	// --- Tableau croisé dynamique ---------------------------------------------

	const pivotAt = (r: number, c: number) =>
		pivots.find((p) => !p.error && r >= (p.anchor ?? p.row) && r < p.row + p.cells.length && c >= p.col && c < p.col + p.cells[0].length);
	/** Tableau croisé qui touche la plage, s'il y en a un. */
	const pivotIn = (r0: number, c0: number, r1: number, c1: number) =>
		pivots.find((p) => !p.error && (p.anchor ?? p.row) <= r1 && p.row + p.cells.length - 1 >= r0 && p.col <= c1 && p.col + p.cells[0].length - 1 >= c0);
	// Un tableau croisé est calculé : ses cellules ne se saisissent pas (comme Excel).
	const PIVOT_READONLY = 'Un tableau croisé ne se modifie pas cellule par cellule : double-cliquez dessus pour changer ses champs';
	// Suppr sur une partie d'un tableau croisé (Excel refuse aussi).
	const PIVOT_PARTIAL = 'Une partie d’un tableau croisé ne s’efface pas : sélectionnez-le en entier pour le supprimer, ou clic droit › Supprimer le tableau croisé';
	// Tableau chargé par une requête (Power Query) : ses cellules ne se
	// saisissent pas, l'actualisation les remplacerait. Ses bords (ligne
	// dessous, colonnes à côté) : le moteur décide (un total, une note ou une
	// remarque restent à part ; une nouvelle ligne ou colonne est refusée, avec
	// le même message, ErrQueryTable).
	const QUERY_READONLY = 'Tableau chargé par une requête : ses cellules ne se modifient pas ; changez la requête (Données › Requêtes) puis actualisez (F5)';
	// Ses colonnes saisies (tapées à droite de l'en-tête, docs/decisions/0008)
	// se modifient : leurs valeurs suivent les lignes du résultat.
	/** Colonnes chargées (pas saisies) du tableau de requête e touchées par les colonnes c0…c1 de la feuille. */
	const loadedIn = (e: Entry, c0: number, c1: number) => {
		for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol); c++) if (!e.t.columns[c - e.t.col]?.hand) return true;
		return false;
	};
	/** Tableau de requête qui contient la cellule (hors colonnes saisies). */
	const queryAt = (r: number, c: number) => entries.find((e) => e.t.query && r >= e.t.row && r <= e.last && c >= e.t.col && c <= e.lastCol && loadedIn(e, c, c));
	/** Un tableau de requête touché par la plage dans ses colonnes chargées (sans la couvrir en entier : le supprimer reste permis). */
	const queryIn = (r0: number, c0: number, r1: number, c1: number, whole = false) =>
		entries.find(
			(e) =>
				e.t.query &&
				e.t.row <= r1 &&
				e.last >= r0 &&
				e.t.col <= c1 &&
				e.lastCol >= c0 &&
				loadedIn(e, c0, c1) &&
				!(whole && e.t.row >= r0 && e.last <= r1 && e.t.col >= c0 && e.lastCol <= c1)
		);

	// Volet latéral : champs du TCD ou mise en forme du graphique.
	let panel = $state<{ kind: 'pivot' | 'chart'; id: number } | null>(null);
	/** TCD dont le volet s'ouvre sur la liste des sources (« Changer la source… »). */
	let pivotSourcePick = $state<number | null>(null);

	/** Supprime un tableau croisé, en erreur ou non (volet, clic droit). */
	function removePivot(p: SheetPivot) {
		if (panel?.kind === 'pivot' && panel.id === p.id) panel = null;
		const del = source.deletePivot;
		void menuAction(del && (() => del(p.id))).then((ok) => ok && say(`Tableau croisé « ${p.config.name ?? ''} » supprimé (Ctrl+Z pour annuler)`));
	}

	/** Tableaux qui peuvent devenir la source d'un TCD (pas un TCD). */
	const pivotSources = (p: SheetPivot) => sources.filter((s) => s.kind !== 'pivot' && s.name !== p.config.name).map((s) => s.name);

	/** Rattache un TCD à un autre tableau source : les champs présents y sont gardés. */
	function repointPivot(p: SheetPivot, name: string) {
		const set = source.setPivot;
		pivotSourcePick = null;
		const cfg = changeSource($state.snapshot(p.config) as PivotConfig, name, sourceColumns(name).map((c) => ({ name: c.name, type: c.type })));
		void menuAction(set && (() => set(p.id, cfg)), true);
	}

	/** « Changer la source… » : ouvre le volet du TCD sur la liste des sources. */
	function pickPivotSource(p: SheetPivot) {
		freeMenu = null;
		active = { row: p.anchor ?? p.row, col: p.col };
		anchor = { ...active };
		pivotSourcePick = p.id;
		panel = { kind: 'pivot', id: p.id };
	}
	let openPivotOnLoad = false;
	let pivotPanelAt = '';

	/** Crée un tableau croisé à partir du tableau actif, ou ouvre le volet de celui sous la cellule active. */
	export function pivotActive() {
		const p = pivotAt(active.row, active.col) ?? pivots.find((p) => p.row === active.row && p.col === active.col);
		if (p) {
			panel = { kind: 'pivot', id: p.id };
			return;
		}
		const e = targetTable();
		if (!e || !source.createPivot) {
			say('Placez la cellule active dans un tableau pour créer un tableau croisé');
			return;
		}
		const k = e.t.columns[targetCol(e)];
		// Comme Excel : la colonne active en lignes, la dernière colonne de
		// nombres en valeurs ; le volet s'ouvre pour la suite.
		const num = e.t.columns.findLast((c) => !c.formula && (c.type === 'integer' || c.type === 'decimal') && c !== k);
		const first = k && !k.formula ? k.name : (e.t.columns.find((c) => !c.formula)?.name ?? '');
		const cfg: PivotConfig = { source: e.t.name, rowFields: [first], values: [num ? { field: num.name, agg: 'sum' } : { agg: 'count' }] };
		openPivotOnLoad = true;
		const create = source.createPivot;
		writing = writing.catch(() => undefined).then(() => create(cfg).then(() => undefined, (err) => say(String(err?.message ?? err))));
	}

	/**
	 * Tableau visé par une commande de tableau (TCD, graphique, tri, filtre,
	 * doublons, SQL) : celui de la cellule active ; ou, quand la sélection est
	 * faite de lignes ou de colonnes entières qui ne contiennent qu'un tableau,
	 * ce tableau-là, comme s'il était sélectionné.
	 */
	function targetTable(): Entry | undefined {
		const here = ownerAt(active.row, active.col);
		if (here) return here;
		const sel = selectionLines();
		if (!sel.rows && !sel.cols) return undefined;
		const hit = entries.filter((e) => (sel.cols ? e.t.col <= sel.c1 && e.lastCol >= sel.c0 : e.t.row <= sel.r1 && e.last >= sel.r0));
		return hit.length === 1 ? hit[0] : undefined;
	}

	/** Colonne visée dans ce tableau : l'active si elle en fait partie, sinon la première colonne sélectionnée du tableau. */
	function targetCol(e: Entry): number {
		if (active.col >= e.t.col && active.col <= e.lastCol) return active.col - e.t.col;
		const { c0 } = selection();
		return Math.min(e.lastCol, Math.max(c0, e.t.col)) - e.t.col;
	}

	/** Données › Convertir : assistant « Convertir le texte en colonnes » sur une sélection d'une seule colonne. */
	let ttcDlg = $state<{ r0: number; r1: number; col: number } | null>(null);

	export function textToColumnsActive() {
		if (!source.textToColumns || !source.textColumnsPreview) return;
		if (locked) {
			say(locked);
			return;
		}
		const { r0, r1, c0, c1 } = selection();
		if (c0 !== c1) {
			say('Convertir ne peut traiter qu’une seule colonne à la fois : sélectionnez des cellules d’une seule colonne');
			return;
		}
		closeMenus();
		ttcDlg = { r0, r1, col: c0 };
	}

	/** Supprime les doublons du tableau de la cellule active (ruban Données). */
	let dedupDlg = $state<{ e: Entry; on: boolean[]; count: number | null; error: string } | null>(null);

	export function dedupActive() {
		const e = targetTable();
		if (!e || !source.removeDuplicates) {
			say('Placez la cellule active dans un tableau');
			return;
		}
		// Comme Excel : toutes les colonnes cochées au départ.
		closeMenus();
		dedupDlg = { e, on: e.t.columns.map((c) => !c.formula), count: null, error: '' };
		previewDedup();
	}

	let dedupSeq = 0;
	async function previewDedup() {
		const d = dedupDlg;
		if (!d || !source.duplicatesPreview) return;
		const cols = d.on.flatMap((on, i) => (on ? [i] : []));
		const seq = ++dedupSeq;
		d.count = null;
		if (!cols.length) {
			d.count = 0;
			return;
		}
		try {
			const n = await source.duplicatesPreview(d.e.t, cols);
			if (seq === dedupSeq && dedupDlg === d) d.count = n;
		} catch (err) {
			d.error = String(err instanceof Error ? err.message : err);
		}
	}

	async function applyDedup() {
		const d = dedupDlg;
		if (!d || !source.removeDuplicates) return;
		const cols = d.on.flatMap((on, i) => (on ? [i] : []));
		try {
			const n = await source.removeDuplicates(d.e.t, cols);
			dedupDlg = null;
			wrap.focus();
			say(n ? `${n} ligne${n > 1 ? 's' : ''} en double supprimée${n > 1 ? 's' : ''} (Ctrl+Z pour annuler)` : 'Aucun doublon');
		} catch (err) {
			d.error = String(err instanceof Error ? err.message : err);
		}
	}

	// --- Ruban Formules ---------------------------------------------------------

	let fnDlg = $state<{ search: string; sel: string | null } | null>(null);

	/** Commence une formule dans la cellule active (Insérer une fonction, REFERENCE). */
	export function startFormula(text: string) {
		fnDlg = null;
		wrap.focus();
		startEdit(true, text);
		if (input) onFormulaInput(input, 'cell');
	}

	/** Somme automatique : SOMME de la colonne du tableau situé juste au-dessus. */
	export function autoSum() {
		const { row, col } = active;
		const inTbl = tableAt(row, col);
		const above = entries.filter((e) => col >= e.t.col && col <= e.lastCol && e.last < row).sort((a, b) => b.last - a.last)[0];
		if (!inTbl && above) {
			startFormula(`=SOMME(${above.t.name}[${above.t.columns[col - above.t.col].name}])`);
			return;
		}
		startFormula('=SOMME(');
	}

	export function openFunctions() {
		closeMenus();
		fnDlg = { search: '', sel: null };
	}

	/** Fonctions du dialogue : nom (français ou anglais), catégorie ou description. */
	function shownFunctions() {
		const q = fnDlg?.search.trim().toLowerCase() ?? '';
		const all = catalog?.functions ?? [];
		const hit = (f: FunctionDoc) => [f.name, f.other, f.category ?? '', f.description ?? ''].some((s) => s.toLowerCase().includes(q));
		return (q ? all.filter(hit) : all).slice(0, 200);
	}

	/** Tri du tableau de la cellule active (ruban Données). */
	export function sortActive(desc: boolean) {
		const e = targetTable();
		if (!e || !source.sortTable) {
			say('Placez la cellule active dans la colonne d’un tableau pour trier');
			return;
		}
		const sort = source.sortTable;
		menuAction(() => sort(e.t, targetCol(e), desc));
	}

	/** Lignes (ou colonnes) entières couvertes par la sélection, comme Excel les insère ou les supprime. */
	function selectionLines(): { r0: number; r1: number; c0: number; c1: number; rows: boolean; cols: boolean } {
		const { r0, r1, c0, c1 } = selection();
		return { r0, r1, c0, c1, rows: c0 === 0 && c1 >= lastCol(), cols: r0 === 0 && r1 >= lastRow() };
	}

	/** Insère ou supprime des lignes ou colonnes entières pour la sélection. */
	export function sheetLines(op: LinesOp) {
		if (!source.sheetLines) return;
		lastRepeat = () => sheetLines(op); // F4
		const { r0, r1, c0, c1 } = selection();
		const lines = source.sheetLines;
		const args: Record<LinesOp, [number, number]> = {
			insertRows: [r0, r1 - r0 + 1],
			deleteRows: [r0, r1],
			insertCols: [c0, c1 - c0 + 1],
			deleteCols: [c0, c1]
		};
		// Animation : ce qui suit glisse (posée quand l'opération part, dans la file des écritures).
		const hint: Record<LinesOp, () => void> = {
			insertRows: () => hintLines(true, r0, r1 - r0 + 1),
			deleteRows: () => hintLines(true, r0, -(r1 - r0 + 1)),
			insertCols: () => hintLines(false, c0, c1 - c0 + 1),
			deleteCols: () => hintLines(false, c0, -(c1 - c0 + 1))
		};
		menuAction(() => (hint[op](), lines(op, ...args[op])));
	}

	// Recherche tapée dans un tableau sans clé : numéro de ligne, ou colonne
	// qui devient la clé (jamais posée sans le demander). Valider réécrit la
	// saisie avec la réponse, en une étape d'annulation ; Annuler rouvre la
	// formule dans l'éditeur.
	let keyDlg = $state<(KeyQuestion & { row: number; col: number; raw: string; keep: boolean; choice: KeyChoice }) | null>(null);

	function applyKeyDialog() {
		const d = keyDlg;
		keyDlg = null;
		if (!d) return;
		wrap.focus();
		void write(d.row, d.col, d.raw, d.keep, undefined, d.choice);
	}

	function cancelKeyDialog() {
		const d = keyDlg;
		keyDlg = null;
		if (!d) return;
		if (d.keep) reopenFormula(d.row, d.col, d.raw, 'recherche annulée : la formule n’est pas enregistrée');
		else wrap.focus();
	}

	// Colonnes saisies à côté d'un tableau chargé (docs/decisions/0008) : la
	// colonne du résultat qui relie leurs valeurs aux lignes d'une
	// actualisation à l'autre, demandée à la première valeur saisie (refus
	// « jointure à choisir ») ou changée depuis le menu du tableau. « # » : le
	// rang des lignes.
	let handDlg = $state<{ table: string; columns: string[]; choice: string; write?: { row: number; col: number; raw: string; keep: boolean }; t?: SheetTable } | null>(null);

	function applyHandDialog() {
		const d = handDlg;
		handDlg = null;
		if (!d) return;
		wrap.focus();
		if (d.write) void write(d.write.row, d.write.col, d.write.raw, d.write.keep, undefined, `hand:${d.choice}`);
		else if (d.t && source.setHandKey) void menuAction(() => source.setHandKey!(d.t!, d.choice));
	}

	function cancelHandDialog() {
		handDlg = null;
		wrap.focus();
	}

	function openHandKey(t: SheetTable) {
		menu = null;
		const columns = t.columns.filter((k) => !k.hand && !k.formula).map((k) => k.name);
		// Clé de plusieurs colonnes (« id + Année ») : proposée telle quelle.
		if (t.handKey && t.handKey !== '#' && !columns.includes(t.handKey)) columns.unshift(t.handKey);
		handDlg = { table: t.name, columns, choice: t.handKey ?? columns[0] ?? '#', t };
	}

	// Insérer… / Supprimer… (comme la boîte d'Excel) : lignes ou colonnes entières.
	let linesDlg =$state<{ x: number; y: number; insert: boolean; choice: 'rows' | 'cols' } | null>(null);

	function openLinesDialog(insert: boolean, x?: number, y?: number) {
		const sel = selectionLines();
		if (sel.rows || sel.cols) {
			sheetLines(insert ? (sel.rows ? 'insertRows' : 'insertCols') : sel.rows ? 'deleteRows' : 'deleteCols');
			return;
		}
		closeMenus();
		const px = x ?? toView(screenX(active.col) + 20);
		const py = y ?? toView(screenY(active.row) + rh(active.row) + 4);
		linesDlg = { x: px, y: py, insert, choice: 'rows' };
	}

	function applyLinesDialog() {
		const d = linesDlg;
		linesDlg = null;
		if (!d) return;
		sheetLines(d.insert ? (d.choice === 'rows' ? 'insertRows' : 'insertCols') : d.choice === 'rows' ? 'deleteRows' : 'deleteCols');
	}

	/** Insérer ou supprimer la ligne ou la colonne de la cellule active (ruban Accueil › Cellules). */
	export function cellsActive(op: 'insertRow' | 'deleteRow' | 'insertColumn' | 'deleteColumn') {
		const e = tableAt(active.row, active.col);
		const sel = selectionLines();
		// Hors tableau, ou lignes / colonnes entières sélectionnées : celles de la feuille.
		if (!e || ((op === 'insertRow' || op === 'deleteRow') && sel.rows) || ((op === 'insertColumn' || op === 'deleteColumn') && sel.cols)) {
			sheetLines(({ insertRow: 'insertRows', deleteRow: 'deleteRows', insertColumn: 'insertCols', deleteColumn: 'deleteCols' } as const)[op]);
			return;
		}
		lastRepeat = () => cellsActive(op); // F4
		const n = active.row - e.t.row - 1; // -1 : ligne d'en-tête
		const i = active.col - e.t.col;
		if (op === 'deleteRow' && n < 0) {
			say('La ligne d’en-tête ne se supprime pas : supprimez le tableau ou ses colonnes');
			return;
		}
		const fn = {
			insertRow: source.insertRow && (() => tableLines(e, 'insertRow', Math.max(0, n))),
			deleteRow: source.deleteRow && (() => tableLines(e, 'deleteRow', n)),
			insertColumn: source.insertColumn && (() => tableLines(e, 'insertColumn', i)),
			deleteColumn: source.deleteColumn && (() => tableLines(e, 'deleteColumn', i))
		}[op];
		menuAction(fn);
	}

	/** Insère ou supprime la ligne n (ou la colonne n) d'un tableau ; ce qui suit glisse à sa place. */
	function tableLines(e: Entry, op: 'insertRow' | 'deleteRow' | 'insertColumn' | 'deleteColumn', n: number): Promise<unknown> {
		const rows = op === 'insertRow' || op === 'deleteRow';
		hintLines(rows, rows ? e.t.row + 1 + n : e.t.col + n, op.startsWith('insert') ? 1 : -1, e.t.id);
		return { insertRow: source.insertRow, deleteRow: source.deleteRow, insertColumn: source.insertColumn, deleteColumn: source.deleteColumn }[op]!(e.t, n);
	}

	/** Suppr sur des colonnes entières d'un tableau : « Supprimer la colonne » ; faux sinon. */
	function deleteWholeColumns(): boolean {
		const { r0, r1, c0, c1 } = selection();
		// Colonnes entières de la feuille (lettres) : les colonnes des tableaux
		// qu'elles traversent sont retirées de ces tableaux (rien d'autre ne
		// bouge) ; un tableau entièrement dedans est laissé à « Effacer ».
		if (wholeKind() === 'cols' && source.deleteColumn) {
			const cut = entries.filter((e) => e.t.col <= c1 && e.lastCol >= c0 && !(e.t.col >= c0 && e.lastCol <= c1));
			if (!cut.length) return false;
			if (cut.some((e) => queryIn(e.t.row, Math.max(c0, e.t.col), e.last, Math.min(c1, e.lastCol), true))) {
				say(QUERY_READONLY);
				return true;
			}
			writing = writing.catch(() => undefined).then(async () => {
				try {
					// Une seule étape d'annulation pour toutes les colonnes.
					if (source.deleteColumnSpans) {
						for (const e of cut) hintLines(false, Math.max(c0, e.t.col), -(Math.min(c1, e.lastCol) - Math.max(c0, e.t.col) + 1), e.t.id);
						await source.deleteColumnSpans(cut.map((e) => ({ table: e.t.name, from: Math.max(c0, e.t.col) - e.t.col, to: Math.min(c1, e.lastCol) - e.t.col })));
					} else for (const e of cut) for (let c = Math.min(c1, e.lastCol); c >= Math.max(c0, e.t.col); c--) await tableLines(e, 'deleteColumn', c - e.t.col);
					schedule();
				} catch (err) {
					say(String((err as Error)?.message ?? err));
				}
			});
			return true;
		}
		const e = tableAt(r0, c0);
		// En-tête compris ou données seules (Ctrl+Espace), toutes les lignes, au moins une.
		if (!e || !source.deleteColumn || e.last <= e.t.row || (r0 !== e.t.row && (r0 !== e.t.row + 1 || r1 === r0)) || r1 < e.last || c1 > e.lastCol || (c0 === e.t.col && c1 === e.lastCol)) return false;
		if (queryIn(r0, c0, r1, c1, true)) {
			say(QUERY_READONLY);
			return true;
		}
		// De droite à gauche : les numéros des colonnes restantes ne bougent pas.
		writing = writing.catch(() => undefined).then(async () => {
			try {
				// Une seule étape d'annulation ; des colonnes vides au milieu coupent le tableau.
				if (source.suppressColumns) {
					hintLines(false, c0, -(c1 - c0 + 1), e.t.id);
					await source.suppressColumns(e.t, c0 - e.t.col, c1 - e.t.col);
				} else for (let c = c1; c >= c0; c--) await tableLines(e, 'deleteColumn', c - e.t.col);
				schedule();
			} catch (err) {
				say(String((err as Error)?.message ?? err));
			}
		});
		return true;
	}

	/** Échap ferme le dialogue ouvert, où que soit le focus. */
	function onGlobalKey(e: KeyboardEvent) {
		if (e.key !== 'Escape') return;
		if (!menusOpen()) return;
		closeMenus();
		wrap.focus();
	}

	/** Un menu, panneau ou boîte de la grille est-il ouvert ? */
	function menusOpen(): boolean {
		return !!(filterDlg || cfView || dedupDlg || fnDlg || menu || freeMenu || chartMenu || linesDlg || formatDlg || validDlg || noteDlg || listMenu || sizeDlg);
	}

	/**
	 * Ferme les menus, panneaux et boîtes ouverts sur la grille : un seul à la
	 * fois (ouvrir le ▾ d'une colonne ferme celui d'une autre). Chaque ouverture
	 * passe par ici ; un clic en dehors les ferme aussi (dismiss.ts).
	 */
	// --- « Filtrer l'onglet par cette colonne / ce champ » (tabchoice.ts) ---

	/** L'entrée de menu pour la colonne column du tableau table (field : champ d'un TCD dont table est la source). */
	function tabChoiceFor(table: string, column: string, o: { formula?: boolean; field?: boolean } = {}): TabChoiceItem {
		return tabChoiceItem({ table, column, ...o, choice: data?.choice, candidates: choiceCands });
	}

	/** Pose ou retire l'onglet à choix (une étape d'annulation). */
	function runTabChoice(table: string, column: string, it: TabChoiceItem) {
		if (it.disabled) return;
		closeMenus();
		const set = source.setTabChoice;
		const clear = source.clearTabChoice;
		menuAction(it.clear ? clear && (() => clear(column)) : set && (() => set(table, column)));
	}

	function closeMenus() {
		menu = freeMenu = chartMenu = listMenu = followMenu = null;
		filterDlg = cfView = dedupDlg = fnDlg = linesDlg = pivotDlg = null;
		formatDlg = noteDlg = sizeDlg = null;
		// Plage source d'une validation en cours de pointage (peut-être sur
		// une autre feuille) : le dialogue reste, retiré, jusqu'à Entrée ou Échap.
		if (!dvPick) validDlg = null;
		noteTip = null;
	}

	/** Plage sélectionnée (lignes et colonnes de la feuille, à partir de 0). */
	export function selectedRange(): { r0: number; c0: number; r1: number; c1: number } {
		const s = selection();
		return { r0: s.r0, c0: s.c0, r1: s.r1, c1: s.c1 };
	}

	/** Sélectionne une plage (scripts : ui.select ; à partir de 0). */
	export function selectRange(r0: number, c0: number, r1: number, c1: number) {
		moveTo(r0, c0);
		anchor = { row: clamp(r1, 0, SHEET_ROWS - 1), col: clamp(c1, 0, SHEET_COLS - 1) };
		ensureVisible(r0, c0);
		refreshFx();
		refreshStatus();
		schedule();
	}

	/** Insertion › Contrôles (bouton, case, liste, zone de saisie, sélecteur de fichier) : contrôle à la cellule active. */
	export function insertButton(kind: ControlKind = '') {
		if (!source.createButton) return;
		buttonsLayer?.openEditor(undefined, kind);
	}
	// Les cases à cocher et listes relisent leur cellule liée à chaque mise à jour.
	let controlsTick = $state(0);

	function runButton(b: SheetButton) {
		if (!source.runButton) return;
		say(`« ${b.config.name || b.config.label} » : exécution…`);
		source.runButton(b.id).then(
			(msg) => say(typeof msg === 'string' ? msg : ''),
			(err) => say(String(err?.message ?? err))
		);
	}

	/** Évènement d'un contrôle de la feuille : son action et les écouteurs des scripts. */
	function fireControl(b: SheetButton, event: 'click' | 'change', value: string, old: string) {
		if (!source.fireControl) return runButton(b);
		say(`« ${b.config.name || b.config.label} » : exécution…`);
		source.fireControl(b.id, event, value, old).then(
			(msg) => say(typeof msg === 'string' ? msg : ''),
			(err) => say(String(err?.message ?? err))
		);
	}

	/** Sélectionne un tableau ou un tableau croisé de la feuille par son nom (onglet SQL). */
	export function revealObject(name: string): boolean {
		const lower = name.toLowerCase();
		const e = entries.find((x) => x.t.name.toLowerCase() === lower);
		if (e) {
			moveTo(e.t.row, e.t.col);
			anchor = { row: e.last, col: e.lastCol };
			ensureVisible(e.t.row, e.t.col);
			refreshFx();
			schedule();
			return true;
		}
		const p = pivots.find((x) => (x.config.name ?? '').toLowerCase() === lower);
		if (p) {
			moveTo(p.row, p.col);
			return true;
		}
		return false;
	}

	// --- Audit de formules : flèches des antécédents et des dépendants --------

	// Liens affichés : pour chaque cellule de la sélection, ses antécédents
	// (flèches bleues vers elle) et ses dépendants (flèches rouges depuis elle).
	type Link = { row: number; col: number; pre: DepTarget[]; dep: DepTarget[] };
	let links = $state<Link[]>([]);
	/** Cellules analysées au plus par « Afficher les liens » (la sélection peut être une colonne entière). */
	const MAX_LINK_CELLS = 400;
	// Sélection dont les liens sont affichés : ils partent dès qu'elle change.
	let linksOf = '';
	const selKey = () => `${sheetIdOf()}:${active.row},${active.col}:${anchor.row},${anchor.col}`;
	$effect(() => {
		const k = selKey();
		if (k !== linksOf && untrack(() => links.length)) {
			links = [];
			schedule();
		}
	});

	/** Formules › Afficher les liens (toute la sélection) ; clear : les retire (Échap). */
	export async function auditActive(kind: 'show' | 'clear') {
		if (kind === 'clear' || !source.dependencies) {
			links = [];
			schedule();
			return;
		}
		const deps = source.dependencies;
		const { r0, r1, c0, c1 } = selection();
		// Lignes visibles d'abord : sur une colonne entière, ce sont celles qu'on voit.
		const { first, last } = visibleRows();
		const rows: number[] = [];
		for (let r = Math.max(r0, first); r <= Math.min(r1, last); r++) rows.push(r);
		for (let r = r0; r <= r1 && rows.length * (c1 - c0 + 1) < MAX_LINK_CELLS; r++) if (r < first || r > last) rows.push(r);
		const cellsToSee: { row: number; col: number }[] = [];
		for (const r of rows) for (let c = c0; c <= c1 && cellsToSee.length < MAX_LINK_CELLS; c++) cellsToSee.push({ row: r, col: c });
		const cut = (r1 - r0 + 1) * (c1 - c0 + 1) > cellsToSee.length;
		try {
			const found: Link[] = [];
			let pre = 0;
			let dep = 0;
			let away = 0;
			// Par petits groupes : le moteur n'est pas inondé de requêtes.
			for (let i = 0; i < cellsToSee.length; i += 16) {
				const part = await Promise.all(cellsToSee.slice(i, i + 16).map((p) => deps(p.row, p.col).then((d) => ({ ...p, d }))));
				for (const { row, col, d } of part) {
					if (!d.precedents.length && !d.dependents.length) continue;
					found.push({ row, col, pre: d.precedents, dep: d.dependents });
					pre += d.precedents.length;
					dep += d.dependents.length;
					away += [...d.precedents, ...d.dependents].filter((x) => x.sheet !== sheetIdOf()).length;
				}
			}
			links = found;
			linksOf = selKey();
			// Ligne précise sans place (masquée, clé introuvable…) : la flèche vise la colonne, la note dit pourquoi.
			const notes = arrowNotes(found.flatMap((l) => [...l.pre, ...l.dep]));
			say(
				found.length === 0
					? 'Aucun lien : la sélection ne contient ni formule ni cellule lue par une formule'
					: `Liens de ${found.length} cellule(s) : ${pre} antécédent(s), ${dep} dépendant(s)` +
							(away ? ` (${away} sur d'autres feuilles)` : '') +
							(cut ? ` · ${MAX_LINK_CELLS} premières cellules seulement` : '') +
							(notes.length ? ` · ${notes.slice(0, 2).join(' ; ')}${notes.length > 2 ? ` (+${notes.length - 2})` : ''}` : '') +
							' · un clic dans la grille (ou Échap) les efface'
			);
			schedule();
		} catch (err) {
			say(String(err instanceof Error ? err.message : err));
		}
	}

	/** Feuille affichée (les cibles des autres feuilles n'ont pas de flèche). */
	function sheetIdOf(): number {
		return data?.sheet ?? 1;
	}

	/** Ferme les menus, dialogues et volets (changement de feuille ou de vue). */
	export function closeOverlays() {
		closeMenus();
		links = [];
		selChart = null;
		panel = null;
		// Volet d'un contrôle : fermé aussi (ses réglages restent appliqués).
		buttonsLayer?.closeEditor();
	}

	/** Erreur de formule de la cellule (code et formule), ou null. */
	function errorAt(r: number, c: number): { code: string; formula: string } | null {
		const e = tableAt(r, c);
		if (e) {
			const i = c - e.t.col;
			const col = e.t.columns[i];
			if (r === e.t.row || !col.formula) return null;
			const v = e.cache.row(r - e.t.row - 1)?.cells[i];
			return isErrorValue(v) ? { code: v as string, formula: col.formula } : null;
		}
		const f = shown.get(key(r, c));
		return f?.error ? { code: f.text, formula: cells.get(key(r, c)) ?? '' } : null;
	}

	function refreshFx() {
		nameBox = cellName(active.row, active.col);
		const err = errorAt(active.row, active.col);
		if (err) message = explainError(err.code, err.formula, (r, c) => shown.get(key(r, c))?.text ?? cells.get(key(r, c))) || message;
		// Un TCD en erreur (source introuvable) ouvre aussi son volet sur la
		// cellule de son message : le supprimer ou changer sa source.
		const pv = pivotAt(active.row, active.col) ?? brokenPivotAt(pivots, active.row, active.col);
		// Le volet du TCD reste ouvert quand le TCD rétrécit sous la cellule
		// active (filtre, champ retiré) : il se ferme quand on la déplace.
		// Graphique sélectionné (même posé sur le TCD, ou relu après son
		// déplacement) : son volet reste ouvert, la cellule active n'y change rien.
		const at = `${active.row}:${active.col}`;
		const next = panelAtCell(panel, pv?.id, selChart !== null, at !== pivotPanelAt, openPivotOnLoad);
		if (next !== panel) panel = next;
		pivotPanelAt = at;
		const e = ownerAt(active.row, active.col);
		const label = (r: number, c: number) => {
			const k = e!.t.columns[c - e!.t.col];
			// Bande des groupes : le groupe, qu'on renomme ici comme une colonne.
			if (r < e!.t.row && !singleCol(e!, c - e!.t.col)) return splitGroup(k.name).group;
			return r === e!.t.row ? k.name : `${k.name}[${r - e!.t.row}]`;
		};
		const sel = selection();
		const multi = !singleSelection();
		if (e) {
			const inside = multi && inTable(e, sel.r0, sel.c0) && inTable(e, sel.r1, sel.c1);
			tableBox = { name: e.t.name, col: inside ? `${label(sel.r0, sel.c0)}:${label(sel.r1, sel.c1)}` : label(active.row, active.col) };
		} else tableBox = null;
		// Taille de la plage pendant le glissé seulement, comme Excel ; ensuite
		// l'adresse de la cellule active revient.
		if (multi && (dragging || headerDrag)) nameBox = `${sel.r1 - sel.r0 + 1}L × ${sel.c1 - sel.c0 + 1}C`;
		refreshSpeech(e, sel, multi);
		if (fxEditing) return;
		fxRO = readOnlyReason(active.row, active.col);
		fxText = editText(active.row, active.col);
	}

	/** Ce que le lecteur d'écran dit de la cellule active (a11y.ts). */
	function refreshSpeech(e: ReturnType<typeof tableAt>, sel: ReturnType<typeof selection>, multi: boolean) {
		const { row: r, col: c } = active;
		const s: CellSpeech = { addr: addrA1(r, c), value: shownText(r, c), name: names.get(key(r, c)) };
		if (e) {
			const k = e.t.columns[c - e.t.col];
			Object.assign(s, { table: e.t.name, column: k.name, header: r === e.t.row, computed: !!k.formula });
		}
		s.error = !!errorAt(r, c);
		speech = s;
		speechSel = multi ? selectionLabel(`${addrA1(sel.r0, sel.c0)}:${addrA1(sel.r1, sel.c1)}`, sel.r1 - sel.r0 + 1, sel.c1 - sel.c0 + 1) : '';
	}

	function say(m: string) {
		message = m;
		refreshStatus();
	}

	// --- Défilement -------------------------------------------------------

	/** Défilement horizontal piloté par la barre de la ligne des onglets. */
	export function scrollLeftTo(x: number, atEnd = false) {
		if (atEnd && holdCols) growAtEnd(false);
		scroller.scrollLeft = x;
		updateExtent();
		schedule();
	}

	// Barre verticale (native) tirée : repérée au clic dans sa zone.
	function onScrollbarDown(e: PointerEvent) {
		if (toLocal(e).x < scroller.clientWidth) return;
		holdRows = rax().indexAt(vs.content);
		const up = () => {
			holdRows = 0;
			window.removeEventListener('pointerup', up);
			updateExtent();
		};
		window.addEventListener('pointerup', up);
	}

	function onScroll() {
		holdWhileScrolling();
		noteTip = null;
		if (holdRows && scroller.scrollTop + scroller.clientHeight >= scroller.scrollHeight - 2) growAtEnd(true);
		if (!vs.inSync(scroller.scrollTop)) vs.fromNative(scroller.scrollTop);
		updateExtent();
		schedule();
	}

	function onWheel(e: WheelEvent) {
		if (e.ctrlKey) {
			// Ctrl+molette : zoom de la feuille (pas celui de toute la page), par pas de 10 %.
			// Pavé tactile (pincement : petits pas) : un pas tous les 50 px cumulés.
			e.preventDefault();
			zoomWheel += e.deltaMode === 0 ? e.deltaY : e.deltaY * 50;
			if (onzoom && Math.abs(zoomWheel) >= 50) {
				onzoom(stepZoom(zoom, -Math.sign(zoomWheel)));
				zoomWheel = 0;
			}
			return;
		}
		holdWhileScrolling();
		noteTip = null;
		const unit = e.deltaMode === 1 ? ROW_H : e.deltaMode === 2 ? vs.view : 1;
		let dy = e.deltaY * unit;
		let dx = e.deltaX * unit;
		if (e.shiftKey && dx === 0) [dx, dy] = [dy, 0];
		e.preventDefault();
		// Molette de souris (crans de ±100 px ou défilement par lignes) : trois
		// lignes par cran, calées sur le haut des lignes, comme Excel. Le pavé
		// tactile (petits pas) garde un défilement au pixel près.
		const notch = e.deltaMode === 1 || (e.deltaMode === 0 && Math.abs(e.deltaY) >= 50 && e.deltaY % 1 === 0);
		if (dy && notch) {
			const steps = e.deltaMode === 1 ? Math.round(e.deltaY) : Math.sign(dy) * Math.max(1, Math.round(Math.abs(e.deltaY) / 100)) * 3;
			const first = rax().indexAt(vs.top);
			const base = rowTop(first) < vs.top && steps < 0 ? first + 1 : first;
			dy = rowTop(clamp(base + steps, 0, SHEET_ROWS - 1)) - vs.top;
		}
		if (dx && notch && e.shiftKey) {
			const steps = Math.sign(dx) * 3;
			const first = cax().indexAt(scroller.scrollLeft);
			dx = cax().offset(clamp(first + steps, 0, SHEET_COLS - 1)) - scroller.scrollLeft;
		}
		if (dy) {
			// Défiler vers le bas agrandit la feuille d'autant.
			if (dy > 0) vs.content = Math.min(rax().total, Math.max(vs.content, vs.top + dy + 2 * vs.view));
			vs.by(dy);
			updateExtent();
			scroller.scrollTop = vs.toNative();
		}
		if (dx) {
			// Vers la droite, la feuille s'agrandit d'autant avant de défiler.
			if (dx > 0 && spacer) {
				const want = cax().indexAt(scroller.scrollLeft + dx + viewW) + 2;
				spacer.style.width = `${MARGIN + cax().offset(Math.min(SHEET_COLS, Math.max(extentCols, want)))}px`;
			}
			scroller.scrollLeft += dx;
			updateExtent();
		}
		schedule();
	}

	function syncNative() {
		scroller.scrollTop = vs.toNative();
		schedule();
	}

	function ensureVisible(row: number, col: number) {
		updateExtent();
		// Sous les volets figés : la case ne doit pas rester cachée dessous.
		// Comme Excel, la case est montrée entière, et la première ligne ou
		// colonne montrée n'est pas coupée (ni contre les volets figés).
		const t = revealWhole(row, rax(), vs.top, vs.view, rowTop(fRows()));
		if (t !== vs.top) vs.to(t);
		const l = revealWhole(col, cax(), scroller.scrollLeft, viewW - MARGIN, cax().offset(fCols()));
		if (l !== scroller.scrollLeft) scroller.scrollLeft = l;
		syncNative();
	}

	// --- Sélection et clavier ---------------------------------------------

	/** Bord de la cellule active dans le sens du déplacement (au-delà de sa fusion). */
	function mergeEdge(dr: number, dc: number) {
		const g = boxOf(active.row, active.col);
		if (!g) return { row: active.row, col: active.col };
		return { row: dr > 0 ? g.row + g.rows - 1 : active.row, col: dc > 0 ? g.col + g.cols - 1 : active.col };
	}

	// Saisie d'une ligne avec Tab : Entrée revient sous la cellule de départ, comme Excel.
	let tabStart: number | null = null;
	/** Cellule voisine de la cellule active (une fusion compte pour une cellule). */
	const step = (dr: number, dc: number) => stepCell(active.row, active.col, dr, dc, boxes, SHEET_ROWS - 1, SHEET_COLS - 1);
	function tabMove(back: boolean) {
		const start = tabStart ?? active.col;
		const n = step(0, back ? -1 : 1);
		moveTo(n.row, n.col);
		tabStart = start;
	}
	function enterMove(back: boolean) {
		const n = step(back ? -1 : 1, 0);
		moveTo(n.row, tabStart ?? n.col);
	}

	/**
	 * Va à la cellule (row, col) ; extend (Maj) : y porte le coin mobile de la
	 * sélection, la cellule active ne bouge pas (comme Excel).
	 */
	function moveTo(row: number, col: number, extend = false) {
		tabStart = null;
		row = clamp(row, 0, SHEET_ROWS - 1);
		col = clamp(col, 0, SHEET_COLS - 1);
		// Lignes et colonnes masquées : on passe au-delà, dans le sens du déplacement.
		const from = extend ? anchor : active;
		row = skipHidden(row, Math.sign(row - from.row), hiddenRows, SHEET_ROWS);
		col = skipHidden(col, Math.sign(col - from.col), hiddenCols, SHEET_COLS);
		if (extend) anchor = { row, col };
		else {
			// Cellules fusionnées : la cellule en haut à gauche, comme Excel.
			const g = boxOf(row, col);
			if (g) {
				row = g.row;
				col = g.col;
			}
			active = { row, col };
			anchor = { ...active };
		}
		message = '';
		ensureVisible(row, col);
		refreshFx();
		refreshStatus();
	}

	// Ctrl + flèche, comme Excel : au bord du bloc de données, ou au prochain
	// bloc (tableaux, cellules libres, déversements, tableaux croisés).
	function jump(row: number, col: number, dr: number, dc: number): [number, number] {
		const areas: Area[] = [];
		const add = (r0: number, c0: number, r1: number, c1: number) => areas.push({ r0, c0, r1, c1 });
		// Un tableau : de sa bande des groupes à sa dernière ligne (voir jump.ts).
		for (const e of entries) add(topRow(e.t), e.t.col, e.last, e.lastCol);
		for (const k of cells.keys()) {
			const [r, c] = k.split(',').map(Number);
			add(r, c, r, c);
		}
		for (const sp of spills) add(sp.row, sp.col, sp.row + sp.rows.length - 1, sp.col + (sp.rows[0]?.length ?? 1) - 1);
		for (const p of pivots) if (!p.error) add(p.anchor ?? p.row, p.col, p.row + p.cells.length - 1, p.col + (p.cells[0]?.length ?? 1) - 1);
		const spans: Span[] = lineSpans(areas, row, col, dr !== 0);
		return dr !== 0 ? [jumpLine(spans, row, dr), col] : [row, jumpLine(spans, col, dc)];
	}

	function cellAt(e: MouseEvent): { row: number; col: number } | null {
		const { x, y } = toLocal(e);
		if (x < MARGIN || y < TOP_H || x >= scroller.clientWidth || y >= scroller.clientHeight) return null;
		return { row: rowAtY(y), col: colAtX(x) };
	}

	// --- Largeur des colonnes : glisser une séparation du bandeau du haut ----

	let resizing: { col: number; startX: number; startW: number; width: number } | null = null;
	let rowResizing: { row: number; startY: number; startH: number; height: number } | null = null;

	/** Ligne dont le bord bas est sous la souris (marge de gauche), ou null. */
	function rowBoundaryAt(e: MouseEvent): number | null {
		const { x, y } = toLocal(e);
		if (x >= MARGIN || y <= TOP_H) return null;
		const py = toGrid(y, TOP_H, rowTop(fRows()), vs.top);
		const row = rax().indexAt(py);
		if (rowTop(row + 1) - py <= 3) return row;
		if (py - rowTop(row) <= 3 && row > 0) return row - 1;
		return null;
	}

	/** Colonne dont le bord droit est sous la souris (bandeau du haut), ou null. */
	function boundaryAt(e: MouseEvent): number | null {
		const { x, y } = toLocal(e);
		if (y >= TOP_H || x <= MARGIN) return null;
		const px = toGrid(x, MARGIN, cax().offset(fCols()), scroller.scrollLeft);
		const c = cax().indexAt(px);
		const right = cax().offset(c + 1);
		const left = cax().offset(c);
		if (right - px <= 4) return c;
		if (px - left <= 4 && c > 0) return c - 1;
		return null;
	}

	/** Largeur ajustée au contenu affiché de la colonne c (double-clic sur le
	 * trait de droite de son en-tête, comme Excel) : en-têtes des tableaux,
	 * lignes visibles, cellules libres et tableaux croisés. */
	function fitWidth(c: number): number {
		let w = 0;
		const { first, last } = visibleRows();
		for (const e of entries) {
			if (c < e.t.col || c > e.lastCol) continue;
			const i = c - e.t.col;
			w = Math.max(w, headerWidth(e, i));
			const d = visibleData(e, first, last);
			if (d) for (let k = d.first; k <= d.last; k++) w = Math.max(w, textWidth(e.cache.row(k)?.text[i] ?? ''));
		}
		for (const [k, raw] of cells) {
			const [r, cc] = k.split(',').map(Number);
			if (cc === c) w = Math.max(w, textWidth(shown.get(k)?.text ?? raw, !!styleAt(r, cc, undefined)?.b));
		}
		for (const p of pivots) {
			const j = c - p.col;
			if (p.error || j < 0 || j >= (p.cells[0]?.length ?? 0)) continue;
			p.cells.forEach((r, i) => (w = Math.max(w, textWidth(r[j] ?? '', pivotBold(p, p.row + i)) + (j === 0 ? pivotLabelOffset(pivotRowOf(p, p.row + i)) : 0))));
		}
		return w ? clamp(Math.ceil(w) + 14, 24, MAX_FIT) : DEF_W;
	}

	function autoFitColumn(c: number) {
		const setWidth = source.setColumnWidth;
		if (!setWidth) return;
		const width = fitWidth(c);
		if (width === cax().size(c)) return;
		userWidths.set(c, width);
		rebuildAxis();
		updateExtent();
		schedule();
		writing = writing.catch(() => undefined).then(() => setWidth(c, width).then(() => undefined, (err) => say(String(err?.message ?? err))));
	}

	/** La souris est-elle sur la bordure de la sélection (poignée de déplacement) ? */
	function onSelectionEdge(e: MouseEvent): boolean {
		if (!source.transfer) return false;
		const { x, y } = toLocal(e);
		if (x < MARGIN || y < TOP_H) return false;
		const { r0, r1, c0, c1 } = selection();
		const left = screenX(c0);
		const right = screenX(c1) + cax().size(c1);
		const top = screenY(r0);
		const bottom = screenY(r1) + rh(r1);
		const near = (v: number, edge: number) => Math.abs(v - edge) <= 3;
		const inX = x >= left - 3 && x <= right + 3;
		const inY = y >= top - 3 && y <= bottom + 3;
		return (inY && (near(x, left) || near(x, right))) || (inX && (near(y, top) || near(y, bottom)));
	}

	/**
	 * Fin du glisser de la bordure de la sélection, comme Excel : la zone se
	 * déplace (Ctrl : se copie) d'un seul bloc, en une étape d'annulation —
	 * tableaux entiers, colonnes entières, cellules libres et cellules de
	 * tableau mêlés (voir transfer). Posée sur un tableau, elle remplace ses
	 * valeurs ; juste à côté, elle l'étend ou le fusionne ; un tableau
	 * chevauché s'écarte vers la droite.
	 */
	async function dropMove() {
		const m = moving;
		moving = null;
		scroller.style.cursor = '';
		schedule();
		if (!m?.to || !source.transfer) return;
		const dr = m.to.row - m.from.row;
		const dc = m.to.col - m.from.col;
		if (!dr && !dc) return;
		const { r0, r1, c0, c1 } = m.sel;
		const dst = { r0: r0 + dr, c0: c0 + dc, r1: r1 + dr, c1: c1 + dc };
		if (dst.r0 < 0 || dst.c0 < 0) return;
		if (pivotIn(dst.r0, dst.c0, dst.r1, dst.c1) || (!m.copy && pivotIn(r0, c0, r1, c1))) {
			say(PIVOT_READONLY);
			return;
		}
		// Un tableau de requête déplacé en entier reste permis ; ses cellules, non.
		const carried = (e: Entry) => !m.copy && e.t.row >= r0 && e.last <= r1 && e.t.col >= c0 && e.lastCol <= c1;
		const hit = entries.some((e) => e.t.query && !carried(e) && e.t.row <= dst.r1 && e.last >= dst.r0 && e.t.col <= dst.c1 && e.lastCol >= dst.c0);
		if (hit || (!m.copy && queryIn(r0, c0, r1, c1, true))) {
			say(QUERY_READONLY);
			return;
		}
		const values = rangeValues(m.sel);
		if (!values) return;
		const copy = !!m.copy;
		const transfer = source.transfer;
		clipRange = null;
		writing = writing.catch(() => undefined).then(async () => {
			try {
				const msg = await transfer(sheetId(), m.sel, dst, !copy, values);
				active = snapMerge(dst.r0, dst.c0);
				anchor = { row: dst.r1, col: dst.c1 };
				message = msg || (copy ? 'Cellules copiées (Ctrl+Z pour annuler)' : 'Cellules déplacées (Ctrl+Z pour annuler)');
			} catch (err) {
				message = String(err instanceof Error ? err.message : err);
			}
			refreshFx();
			refreshStatus();
			schedule();
		});
	}

	// --- Poignée de recopie ------------------------------------------------------

	type Range = { r0: number; r1: number; c0: number; c1: number };
	let filling = $state<{ sel: Range; to?: Range } | null>(null);

	/** Position (écran) du coin bas droit de la sélection. */
	function fillHandle() {
		let { r1, c1 } = selection();
		const g = boxOf(r1, c1);
		if (g) {
			r1 = g.row + g.rows - 1;
			c1 = g.col + g.cols - 1;
		}
		return { x: screenX(c1) + cax().size(c1), y: screenY(r1) + rh(r1) };
	}

	function onFillHandle(e: MouseEvent): boolean {
		if (editing || !source.paste) return false;
		const p = toLocal(e);
		const h = fillHandle();
		return Math.abs(p.x - h.x) <= 5 && Math.abs(p.y - h.y) <= 5;
	}

	/** Zone étendue par la poignée jusqu'à la cellule visée (une seule direction). */
	function fillTarget(sel: Range, cell: { row: number; col: number }): Range | undefined {
		const down = cell.row - sel.r1;
		const up = sel.r0 - cell.row;
		const right = cell.col - sel.c1;
		const left = sel.c0 - cell.col;
		const best = Math.max(down, up, right, left);
		if (best <= 0) return undefined;
		if (best === down) return { ...sel, r1: cell.row };
		if (best === up) return { ...sel, r0: cell.row };
		if (best === right) return { ...sel, c1: cell.col };
		return { ...sel, c0: cell.col };
	}

	/** Remplit to à partir de sel (séries, dates, mois… voir fill.ts). */
	function doFill(sel: Range, to: Range) {
		if (!source.paste) return;
		const val = (r: number, c: number) => cellInfo(r, c).value;
		let row = 0;
		let col = 0;
		let block: string[][] = [];
		if (to.r1 > sel.r1 || to.r0 < sel.r0) {
			const down = to.r1 > sel.r1;
			const n = down ? to.r1 - sel.r1 : sel.r0 - to.r0;
			const cols: string[][] = [];
			for (let c = sel.c0; c <= sel.c1; c++) {
				const src: string[] = [];
				for (let r = sel.r0; r <= sel.r1; r++) src.push(val(r, c));
				cols.push(down ? fillSeries(src, n) : fillSeries([...src].reverse(), n).reverse());
			}
			block = Array.from({ length: n }, (_, k) => cols.map((x) => x[k]));
			row = down ? sel.r1 + 1 : to.r0;
			col = sel.c0;
		} else {
			const right = to.c1 > sel.c1;
			const n = right ? to.c1 - sel.c1 : sel.c0 - to.c0;
			const w = sel.c1 - sel.c0 + 1;
			const tables = refTables();
			for (let r = sel.r0; r <= sel.r1; r++) {
				const src: string[] = [];
				for (let c = sel.c0; c <= sel.c1; c++) src.push(val(r, c));
				const line = right ? fillSeries(src, n) : fillSeries([...src].reverse(), n).reverse();
				// Les colonnes de tableau citées suivent le décalage (Ventes[Prix] → Ventes[Qté]).
				block.push(
					line.map((v, j) => {
						const dest = right ? sel.c1 + 1 + j : to.c0 + j;
						const from = right ? sel.c0 + (j % w) : sel.c1 - ((sel.c0 - 1 - dest) % w);
						return shiftTableColumns(v, dest - from, tables);
					})
				);
			}
			row = sel.r0;
			col = right ? sel.c1 + 1 : to.c0;
		}
		const paste = source.paste;
		writing = writing.catch(() => undefined).then(async () => {
			try {
				await paste(row, col, block);
				message = 'Recopie incrémentée (Ctrl+Z pour annuler)';
			} catch (err) {
				message = String(err instanceof Error ? err.message : err);
			}
			active = { row: to.r0, col: to.c0 };
			anchor = { row: to.r1, col: to.c1 };
			refreshFx();
			refreshStatus();
			schedule();
		});
	}

	/** Double-clic sur la poignée : recopie jusqu'en bas du tableau voisin. */
	function fillDown() {
		const sel = selection();
		const near = entries.find((e) => e.t.col <= sel.c1 + 1 && e.lastCol >= sel.c0 - 1 && e.t.row <= sel.r1 && e.last > sel.r1);
		if (!near) return;
		doFill(sel, { ...sel, r1: near.last });
	}

	// Sélection de lignes ou de colonnes entières par la marge ou le bandeau.
	let headerDrag: 'rows' | 'cols' | null = null;

	/** Ligne (marge de gauche) ou colonne (bandeau du haut) sous la souris. */
	function headerAt(e: MouseEvent): { kind: 'rows'; row: number } | { kind: 'cols'; col: number } | null {
		const { x, y } = toLocal(e);
		if (x < MARGIN && y >= TOP_H) return { kind: 'rows', row: clamp(rowAtY(y), 0, SHEET_ROWS - 1) };
		if (y < TOP_H && x >= MARGIN) return { kind: 'cols', col: colAtX(x) };
		return null;
	}

	/** Dernière ligne et colonne de la zone affichée (la sélection entière s'y arrête). */
	function lastRow() {
		return Math.max(0, rax().indexAt(vs.content) - 1);
	}
	function lastCol() {
		return Math.max(0, extentCols - 1);
	}

	// Sélection de lignes ou colonnes entières (marge ou bandeau) : tant que la
	// sélection n'a pas changé, la mise en forme vaut pour les lignes ou
	// colonnes entières. Elles restent exactement celles choisies (range), sans
	// s'agrandir aux fusions qui les traversent, comme Excel (voir wholeLines) ;
	// from : la ligne (colonne) où le clic a commencé, d'où partent le glisser
	// et Maj+clic (la cellule active, elle, est le haut de sa fusion).
	let wholeSel: { kind: 'rows' | 'cols'; from: number; range: CellRange; anchor: { row: number; col: number }; active: { row: number; col: number } } | null = null;
	function wholeKind(): 'rows' | 'cols' | null {
		const w = wholeSel;
		if (!w) return null;
		const same = (a: { row: number; col: number }, b: { row: number; col: number }) => a.row === b.row && a.col === b.col;
		return same(w.anchor, anchor) && same(w.active, active) ? w.kind : null;
	}
	/** Début d'un Maj+clic ou d'un glisser dans la gouttière (rows) ou le bandeau. */
	function wholeFrom(kind: 'rows' | 'cols'): number {
		return wholeKind() === kind ? wholeSel!.from : kind === 'rows' ? active.row : active.col;
	}

	/** Lignes from à to entières ; la cellule active est au début de from (comme Excel). */
	function selectRows(from: number, to: number) {
		const w = wholeLines('rows', from, to, lastCol(), boxes);
		active = w.active;
		anchor = { row: to, col: lastCol() };
		wholeSel = { kind: 'rows', from, range: w.range, anchor: { ...anchor }, active: { ...active } };
		refreshFx();
		refreshStatus();
		schedule();
	}
	function selectCols(from: number, to: number) {
		const w = wholeLines('cols', from, to, lastRow(), boxes);
		active = w.active;
		anchor = { row: lastRow(), col: to };
		wholeSel = { kind: 'cols', from, range: w.range, anchor: { ...anchor }, active: { ...active } };
		refreshFx();
		refreshStatus();
		schedule();
	}

	/** La cellule est-elle dans la sélection en cours ? */
	function inSelection(row: number, col: number) {
		const { r0, r1, c0, c1 } = selection();
		return row >= r0 && row <= r1 && col >= c0 && col <= c1;
	}

	function onMouseDown(e: MouseEvent) {
		if (hold) {
			pointAway(e);
			return;
		}
		// Flèches d'audit : un clic dans la grille les efface, même sur la cellule
		// active (le ruban n'a plus de « Supprimer les flèches »).
		if (links.length) {
			links = [];
			schedule();
		}
		// Feuille protégée : comme Excel, un clic place la cellule active (pour
		// lire, copier, figer les volets), sans glisser ni menus de tableau ni
		// redimensionnement.
		if (locked) {
			e.preventDefault();
			wrap.focus();
			if (selChart !== null) selChart = null;
			const cell = e.button === 0 ? cellAt(e) : null;
			if (cell) moveTo(cell.row, cell.col, e.shiftKey);
			return;
		}
		{
			const hm = e.button === 0 ? hiddenMarkAt(e) : undefined;
			if (hm && source.setLinesHidden) {
				e.preventDefault();
				const set = source.setLinesHidden;
				menuAction(() => set(hm.rows, hm.i0, hm.i1, false));
				return;
			}
		}
		if (e.button === 0 && onFillHandle(e)) {
			e.preventDefault();
			if (e.detail === 2) {
				fillDown();
				return;
			}
			filling = { sel: selection() };
			return;
		}
		// Flèche de la liste déroulante (validation) de la cellule active.
		if (e.button === 0 && !editing && listOf(active.row, active.col)) {
			const b = dropButton();
			const { x, y } = toLocal(e);
			if (x >= b.x && x <= b.x + b.w && y >= b.y && y <= b.y + b.h) {
				e.preventDefault();
				if (listMenu) listMenu = null;
				else openList();
				return;
			}
		}
		if (selChart !== null) {
			selChart = null;
			if (panel?.kind === 'chart') panel = null;
		}
		if (e.button === 0 && !formulaMode && tableHintAt(e) && source.formatAsTable) {
			e.preventDefault();
			if (!commitEditor()) return;
			const { row, col } = active;
			const fmt = source.formatAsTable;
			writing = writing.catch(() => undefined).then(() => fmt(row, col).then(
					() => undefined,
					(err) => say(String(err?.message ?? err))
				)
			);
			return;
		}
		if (e.button === 0 && !formulaMode && followBadgeAt(e)) {
			e.preventDefault();
			if (!commitEditor()) return;
			const fe = followBadgeAt(e)!;
			closeMenus();
			const r = wrap.getBoundingClientRect();
			followMenu = { x: e.clientX - r.left, y: e.clientY - r.top, e: fe };
			return;
		}
		if (e.button !== 1 && !formulaMode && keyHintAt(e)) {
			e.preventDefault();
			if (!commitEditor()) return;
			if (e.button === 0) acceptKeyHint(keyHintAt(e)!);
			return;
		}
		const frozen = e.button === 0 && boundaryAt(e) === null ? frozenHeaderAt(e) : null;
		if (frozen) {
			commitEditor();
			if (headerButton(e, frozen.e, frozen.col, true)) {
				e.preventDefault();
				return;
			}
		}
		// Coin en haut à gauche (entre la marge et le bandeau) : toute la
		// feuille, toutes les lignes et toutes les colonnes, comme dans Excel.
		{
			const { x, y } = toLocal(e);
			if (e.button === 0 && x < MARGIN && y < TOP_H) {
				e.preventDefault();
				if (!commitEditor()) return;
				wrap.focus();
				selectRows(0, lastRow());
				return;
			}
		}
		const hd = boundaryAt(e) === null && rowBoundaryAt(e) === null ? headerAt(e) : null;
		if (hd && e.button !== 1) {
			e.preventDefault();
			if (!commitEditor()) return; // saisie refusée : l'éditeur reste ouvert
			wrap.focus();
			// Clic droit dans la sélection : on la garde.
			if (e.button === 2 && (hd.kind === 'rows' ? inSelection(hd.row, active.col) : inSelection(active.row, hd.col))) return;
			if (hd.kind === 'rows') selectRows(e.shiftKey ? wholeFrom('rows') : hd.row, hd.row);
			else selectCols(e.shiftKey ? wholeFrom('cols') : hd.col, hd.col);
			headerDrag = e.button === 0 ? hd.kind : null;
			return;
		}
		if (e.button === 0 && !editing && onSelectionEdge(e)) {
			const cell = cellAt(e);
			if (cell) {
				e.preventDefault();
				// Point saisi ramené dans la sélection : la bordure se prend à
				// quelques pixels de part et d'autre, et une prise juste à
				// l'extérieur décalait le dépôt d'une ligne ou d'une colonne.
				const sel = selection();
				moving = { sel, from: { row: clamp(cell.row, sel.r0, sel.r1), col: clamp(cell.col, sel.c0, sel.c1) } };
				return;
			}
		}
		const rb = rowBoundaryAt(e);
		if (rb !== null && source.setRowHeight) {
			e.preventDefault();
			if (e.detail === 2) {
				// Double-clic : hauteur par défaut.
				userHeights.delete(rb);
				rebuildRows();
				updateExtent();
				schedule();
				const set = source.setRowHeight;
				writing = writing.catch(() => undefined).then(() => set(rb, 0).then(() => undefined, (err) => say(String(err?.message ?? err))));
				return;
			}
			rowResizing = { row: rb, startY: e.clientY, startH: rh(rb), height: rh(rb) };
			return;
		}
		const b = boundaryAt(e);
		if (b !== null && source.setColumnWidth) {
			e.preventDefault();
			resizing = { col: b, startX: e.clientX, startW: cax().size(b), width: cax().size(b) };
			return;
		}
		const cell = cellAt(e);
		if (!cell) return;
		if (e.button === 0 && formulaMode && grabRefMark(e, cell)) {
			e.preventDefault();
			return;
		}
		if (e.button === 0 && formulaMode && insertRef(cell)) {
			e.preventDefault();
			// Glisser ensuite insère une plage (Cell[A1:B3], Tableau[Col][1:3]…).
			refDrag = { from: cell, to: cell };
			return;
		}
		e.preventDefault();
		if (!commitEditor()) return; // saisie refusée : l'éditeur reste ouvert
		// Clic droit dans la sélection : elle reste (le menu porte sur elle).
		if (e.button === 2 && inSelection(cell.row, cell.col)) return;
		// Volet d'un contrôle ouvert, champ actif : le clic y met l'adresse ou la valeur de la cellule.
		if (e.button === 0 && buttonsLayer?.pickCell(cell.row, cell.col, shownValue(cell.row, cell.col))) return;
		if (e.button === 0 && openTypeMenu(e, cell)) return;
		if (e.button === 0 && openPivotMenu(e, cell)) return;
		if (e.button === 0 && togglePivotFold(e, cell)) {
			wrap.focus();
			return;
		}
		wrap.focus();
		moveTo(cell.row, cell.col, e.shiftKey);
		dragging = true;
	}

	/**
	 * Clic sur la grille pendant le pointage d'une formule d'une autre feuille :
	 * la référence au curseur (ou une plage au glissé), rien d'autre ; la saisie
	 * reste ouverte dans la barre de formule.
	 */
	function pointAway(e: MouseEvent) {
		e.preventDefault();
		const cell = boundaryAt(e) === null && rowBoundaryAt(e) === null ? cellAt(e) : null;
		if (!cell || e.button !== 0) return;
		if (grabRefMark(e, cell)) return;
		if (insertRef(cell)) {
			refDrag = { from: cell, to: cell };
			return;
		}
		say('Placez le curseur de la formule après « ( », « ; » ou un opérateur pour y insérer une référence');
	}

	// Dernière référence insérée au clic : un autre clic la remplace (comme Excel).
	let lastRef: { el: TextField; start: number; end: number; value: string } | null = null;

	/**
	 * Référence d'une cellule dans une formule (nomenclature de Lattis) :
	 * Cell[A1] pour une cellule libre ; dans le tableau de la formule, [Col]
	 * pour sa ligne et [Col][k] pour une autre ; ailleurs Tableau[Col][k]
	 * (k : le rang de la ligne, même si le tableau a une clé) ; un en-tête donne
	 * Tableau[Col], toute la colonne.
	 */
	function refFor(r: number, c: number, kind: '' | 'table' | 'column' = ''): string {
		const e = tableAt(r, c);
		if (!e) return cellAddr(r, c);
		const col = e.t.columns[c - e.t.col].name;
		// L'argument en cours attend un tableau ou une colonne entière
		// (REFERENCE(Tableau[Colonne]; …), SOMME(T[Col]), EXISTE(Tableau; …)) :
		// le clic dans le tableau donne cette forme, quelle que soit la ligne.
		if (kind === 'table') return e.t.name;
		if (kind === 'column') return `${e.t.name}[${col}]`;
		// Formule d'une autre feuille (pointage) : elle n'est dans aucun tableau
		// d'ici. Sinon [Col] / [Col][-1] dans le tableau de la formule (ou à côté),
		// Tableau[Col][n] depuis une cellule libre : une valeur, pas la colonne
		// (demande de Louis) ; l'en-tête seul donne Tableau[Col] (voir pointedRef).
		return pointedRef(r, c, refTables(), hold ? null : active);
	}

	/** Pendant la saisie d'une formule, insère la référence de la cellule cliquée au curseur. */
	function insertRef(cell: { row: number; col: number }, text?: string): boolean {
		const el = editing ? input : fxEditing ? fxInput : null;
		if (!el) return false;
		if (text === undefined && editing && cell.row === active.row && cell.col === active.col) return false;
		const v = el.value;
		const s0 = el.selectionStart ?? v.length;
		const s1 = el.selectionEnd ?? s0;
		const last = lastRef?.el === el ? lastRef : null;
		const at = insertRefAt(v, s0, s1, '', last);
		if (!at) return false;
		let ref = text ?? refFor(cell.row, cell.col, catalog ? argKindAt(v, at.start, catalog) : '');
		// Formule d'une autre feuille : une cellule libre d'ici prend le nom de la feuille.
		if (hold) ref = qualifyRef(ref, sheetName);
		const r = insertRefAt(v, s0, s1, ref, last)!;
		if (editing) editValueText = r.value;
		else fxText = r.value;
		el.value = r.value;
		el.setSelectionRange(r.end, r.end);
		el.focus();
		lastRef = { el, start: r.start, end: r.end, value: r.value };
		return true;
	}

	/**
	 * Référence pointée (clic ou glissé terminé) : si la fonction attend encore
	 * un argument obligatoire, la saisie passe au suivant (« ; »), comme le
	 * ferait l'utilisateur (choix de Louis) ; pas après le dernier argument.
	 */
	function nextArg() {
		const el = editing ? input : fxEditing ? fxInput : null;
		if (!el || !catalog) return;
		const caret = el.selectionEnd ?? el.value.length;
		if (!needsNextArg(el.value, caret, catalog)) return;
		const r = toNextArg(el.value, caret);
		if (editing) editValueText = r.text;
		else fxText = r.text;
		el.value = r.text;
		el.setSelectionRange(r.caret, r.caret);
		lastRef = null; // le clic suivant ajoute une référence, il ne remplace plus celle-ci
		onFormulaInput(el, editing ? 'cell' : 'fx');
		// Pointage sur une autre feuille : l'argument suivant se saisit dans la
		// feuille de la formule, où l'on revient (demande de Louis).
		if (hold) {
			hold = updateHold(hold, r.text, r.caret, r.caret);
			completion = sig = null;
			// Après la fin du clic (mouseup) : la saisie reprend dans sa cellule sans être validée.
			const back = hold.sheet;
			setTimeout(() => onsheet?.(back));
		}
	}

	// Glissé en mode formule depuis une référence insérée : la plage couverte.
	let refDrag: { from: { row: number; col: number }; to: { row: number; col: number } } | null = null;

	// Marqueur de référence glissé (comme Excel : on attrape le bord du cadre
	// coloré d'une référence pendant la saisie d'une formule, on le dépose
	// ailleurs, la référence est réécrite).
	let refMove = $state<{ start: number; end: number; rect: Rect; from: { row: number; col: number }; to: { row: number; col: number } } | null>(null);

	/** Contexte des références : tableaux et ligne de la formule. */
	function refContext() {
		const tables = refTables();
		// Formule d'une autre feuille (pointage) : ni tableau ni ligne d'ici.
		if (hold) return { tables, at: undefined };
		const here = tableAt(active.row, active.col);
		const at = here && active.row > here.t.row ? { table: tables[entries.indexOf(here)], row: active.row } : undefined;
		return { tables, at };
	}

	/** Le bord d'un marqueur de référence est-il sous la souris ? */
	function hitRefMark(e: MouseEvent): boolean {
		return markAt(e) !== null;
	}

	/** Attrape le marqueur dont le bord est sous la souris ; faux s'il n'y en a pas. */
	function grabRefMark(e: MouseEvent, cell: { row: number; col: number }): boolean {
		const m = markAt(e);
		if (!m) return false;
		refMove = { start: m.f.start, end: m.f.end, rect: m.k, from: cell, to: cell };
		return true;
	}

	/** Marqueur (référence et plage) dont le bord est à moins de 4 px de la souris. */
	function markAt(e: MouseEvent) {
		const { tables, at } = refContext();
		const { x, y } = toLocal(e);
		for (const f of formulaRefs(editing ? editValueText : fxText)) {
			const k = refRect(f.text, tables, sheetName, at, !!hold);
			if (!k) continue;
			const left = screenX(k.c0);
			const right = screenX(k.c1) + cax().size(k.c1);
			const top = screenY(k.r0);
			const bottom = screenY(k.r1) + rh(k.r1);
			const near = (v: number, edge: number) => Math.abs(v - edge) <= 4;
			const inX = x >= left - 4 && x <= right + 4;
			const inY = y >= top - 4 && y <= bottom + 4;
			if ((inY && (near(x, left) || near(x, right))) || (inX && (near(y, top) || near(y, bottom)))) return { f, k };
		}
		return null;
	}

	/** Dépose le marqueur glissé : la référence est réécrite dans la formule. */
	function dropRefMark() {
		const m = refMove;
		refMove = null;
		schedule();
		if (!m) return;
		const dr = m.to.row - m.from.row;
		const dc = m.to.col - m.from.col;
		if (!dr && !dc) return;
		const { tables, at } = refContext();
		const moved = movedRef(m.rect, dr, dc, tables, at, m.to);
		const text = moved !== null && hold ? qualifyRef(moved, sheetName) : moved;
		if (text === null) {
			say('Cette plage ne peut pas devenir une référence : une plage couvre une seule colonne d’un tableau');
			return;
		}
		const el = editing ? input : fxEditing ? fxInput : null;
		if (!el) return;
		const next = el.value.slice(0, m.start) + text + el.value.slice(m.end);
		if (editing) editValueText = next;
		else fxText = next;
		el.value = next;
		const caret = m.start + text.length;
		el.setSelectionRange(caret, caret);
		el.focus();
		lastRef = null;
	}

	/** Tableaux de la feuille, pour situer et écrire les références. */
	function refTables(): RefTable[] {
		return entries.map((e) => ({
			name: e.t.name,
			row: e.t.row,
			col: e.t.col,
			columns: e.t.columns.map((c) => c.name),
			rows: e.t.rows,
			keyed: false, // [k] est toujours un rang
			keyRow: (k: string) => {
				if (e.t.key === undefined) return undefined;
				for (let n = 0; n < e.t.rows; n++) {
					const v = e.cache.row(n)?.text[e.t.key];
					if (v !== undefined && (v === k || v.replace(/\s/g, '') === k)) return n;
				}
				return undefined;
			}
		}));
	}

	/** Plage glissée en mode formule : sa référence remplace celle insérée au clic. */
	function dragRef(cell: { row: number; col: number }) {
		if (!refDrag || (cell.row === refDrag.to.row && cell.col === refDrag.to.col)) return;
		refDrag.to = cell;
		const { from } = refDrag;
		const { tables, at } = refContext();
		const hereT = at?.table;
		const text =
			from.row === cell.row && from.col === cell.col
				? refFor(cell.row, cell.col)
				: rangeRef({ r0: from.row, c0: from.col, r1: cell.row, c1: cell.col }, tables, hereT);
		if (text === null) {
			say('Une plage ne peut couvrir qu’une seule colonne d’un tableau : glissez dans une colonne, ou sur des cellules libres');
			return;
		}
		insertRef(cell, text);
		schedule();
	}

	function onMouseMove(e: MouseEvent) {
		{
			const hc = entries.some((x) => x.t.access === 'participant') ? cellAt(e) : null;
			const hl = hc && lockedRowAt(hc.row, hc.col) ? hc.row : -1;
			if (hl !== hoverLocked) {
				hoverLocked = hl;
				schedule();
			}
		}
		if (refMove) {
			const cell = cellAt(e);
			if (cell && (cell.row !== refMove.to.row || cell.col !== refMove.to.col)) {
				refMove.to = cell;
				schedule();
			}
			return;
		}
		if (refDrag) {
			const cell = cellAt(e);
			if (cell) dragRef(cell);
			return;
		}
		if (rowResizing) {
			rowResizing.height = Math.max(12, Math.round(rowResizing.startH + (e.clientY - rowResizing.startY) / zoomK()));
			rebuildRows({ row: rowResizing.row, height: rowResizing.height });
			updateExtent();
			schedule();
			return;
		}
		if (resizing) {
			resizing.width = Math.max(24, Math.round(resizing.startW + (e.clientX - resizing.startX) / zoomK()));
			rebuildAxis({ col: resizing.col, width: resizing.width });
			schedule();
			return;
		}
		if (filling) {
			const cell = cellAt(e);
			if (cell) {
				filling.to = fillTarget(filling.sel, cell);
				schedule();
			}
			return;
		}
		if (moving) {
			const cell = cellAt(e);
			moving.copy = e.ctrlKey || e.metaKey;
			// Ctrl : petit + sur le curseur, comme Excel.
			scroller.style.cursor = moving.copy ? 'copy' : 'move';
			if (cell) {
				moving.to = cell;
				schedule();
			}
			return;
		}
		if (headerDrag) {
			const p = toLocal(e);
			if (headerDrag === 'rows') {
				const row = clamp(rowAtY(p.y), 0, SHEET_ROWS - 1);
				selectRows(wholeFrom('rows'), row);
			} else {
				const col = colAtX(Math.max(MARGIN, p.x));
				selectCols(wholeFrom('cols'), col);
			}
			return;
		}
		scroller.style.cursor =
			formulaMode && !refDrag && hitRefMark(e)
				? 'move' // bord d'un marqueur de référence : il se glisse
				: source.setRowHeight && rowBoundaryAt(e) !== null
				? 'row-resize'
				: source.setColumnWidth && boundaryAt(e) !== null
				? 'col-resize'
				: !dragging && onFillHandle(e)
					? 'crosshair'
					: !dragging && onSelectionEdge(e)
						? 'move'
						: headerAt(e)
							? 'pointer'
							: '';
		if (!dragging) updateHeaderTip(e);
		updateNoteTip(dragging ? null : cellAt(e));
		if (!dragging) return;
		const cell = cellAt(e);
		if (!cell) return;
		// Glisser : le coin mobile suit la souris, la cellule active reste.
		anchor = cell;
		const sel = selection();
		nameBox = !singleSelection() ? `${sel.r1 - sel.r0 + 1}L × ${sel.c1 - sel.c0 + 1}C` : cellName(active.row, active.col);
		schedule();
	}

	// --- Défilement automatique pendant un glisser (autoscroll.ts) ----------

	// Dernière position de la souris pendant un glisser (coordonnées de l'écran).
	let dragPtr: { x: number; y: number; ctrl: boolean; meta: boolean; shift: boolean } | null = null;
	let autoFrame = 0;
	let autoLast = 0;

	/** Un glisser qui suit la souris sur les cellules est-il en cours ? */
	const cellDrag = () => dragging || !!filling || !!moving || !!refDrag || !!refMove || !!headerDrag;

	/** Zone qui défile, en pixels locaux (hors marge, bandeau et volets figés). */
	function scrollArea() {
		return {
			left: MARGIN + cax().offset(fCols()),
			top: TOP_H + rowTop(fRows()),
			right: scroller.clientWidth,
			bottom: scroller.clientHeight
		};
	}

	/** Défilement voulu (pixels par image, signés) pour la souris p (coordonnées de l'écran). */
	function dragScroll(p: { x: number; y: number }) {
		const r = scroller.getBoundingClientRect();
		const k = zoomK();
		const a = scrollArea();
		// Rien à défiler en largeur si les colonnes figées prennent tout.
		const dx = headerDrag === 'rows' ? 0 : edgeScroll((p.x - r.left) / k, a.left, a.right);
		const dy = headerDrag === 'cols' ? 0 : edgeScroll((p.y - r.top) / k, a.top, a.bottom);
		return { dx: scroller.scrollLeft <= 0 && dx < 0 ? 0 : dx, dy: vs.top <= 0 && dy < 0 ? 0 : dy };
	}

	/**
	 * Rejoue le glisser à la souris ramenée sur la zone des cellules (au bord
	 * qui défile) : hors de la grille, la sélection suit la ligne ou la
	 * colonne du bord, comme Excel.
	 */
	function dragAt(p: NonNullable<typeof dragPtr>, dir: { dx: number; dy: number }) {
		const r = scroller.getBoundingClientRect();
		const k = zoomK();
		const a = scrollArea();
		let x = (p.x - r.left) / k;
		let y = (p.y - r.top) / k;
		// Dans le sens du défilement, la souris est ramenée sur la zone qui
		// défile (pas sur les volets figés) ; sinon sur la grille.
		x = clamp(x, dir.dx < 0 ? a.left : MARGIN, scroller.clientWidth - 1);
		y = clamp(y, dir.dy < 0 ? a.top : TOP_H, scroller.clientHeight - 1);
		if (headerDrag === 'rows') x = Math.min(x, MARGIN - 1);
		if (headerDrag === 'cols') y = Math.min(y, TOP_H - 1);
		onMouseMove(new MouseEvent('mousemove', { clientX: r.left + x * k, clientY: r.top + y * k, ctrlKey: p.ctrl, metaKey: p.meta, shiftKey: p.shift, buttons: 1 }));
	}

	/** Souris pendant un glisser, sur toute la fenêtre : hors de la grille, la sélection continue et la feuille défile. */
	function onDragMove(e: MouseEvent) {
		if (!cellDrag() || !scroller) return;
		dragPtr = { x: e.clientX, y: e.clientY, ctrl: e.ctrlKey, meta: e.metaKey, shift: e.shiftKey };
		const dir = dragScroll(dragPtr);
		// Hors de la grille (ou sur un graphique) : la grille ne reçoit pas l'évènement.
		if (!(e.target instanceof Node && scroller.contains(e.target))) dragAt(dragPtr, dir);
		if ((dir.dx || dir.dy) && !autoFrame) {
			autoLast = performance.now();
			autoFrame = requestAnimationFrame(autoTick);
		}
	}

	function stopAutoScroll() {
		cancelAnimationFrame(autoFrame);
		autoFrame = 0;
		dragPtr = null;
	}

	function autoTick(now: number) {
		autoFrame = 0;
		if (!cellDrag() || !dragPtr || !scroller) return stopAutoScroll();
		const dir = dragScroll(dragPtr);
		if (!dir.dx && !dir.dy) return;
		const dt = now - autoLast;
		autoLast = now;
		holdWhileScrolling();
		const dy = frameStep(dir.dy, dt);
		const dx = frameStep(dir.dx, dt);
		if (dy) {
			// Vers le bas, la feuille s'agrandit d'autant (comme à la molette).
			if (dy > 0) vs.content = Math.min(rax().total, Math.max(vs.content, vs.top + dy + 2 * vs.view));
			vs.by(dy);
			updateExtent();
			scroller.scrollTop = vs.toNative();
		}
		if (dx) {
			if (dx > 0 && spacer) {
				const want = cax().indexAt(scroller.scrollLeft + dx + viewW) + 2;
				spacer.style.width = `${MARGIN + cax().offset(Math.min(SHEET_COLS, Math.max(extentCols, want)))}px`;
			}
			scroller.scrollLeft += dx;
			updateExtent();
		}
		dragAt(dragPtr, dir);
		schedule();
		autoFrame = requestAnimationFrame(autoTick);
	}

	function onDblClick(e: MouseEvent) {
		if (hold) return; // pointage d'une formule d'une autre feuille
		if (locked) return; // feuille protégée : pas de saisie, sans message
		const b = boundaryAt(e);
		if (b !== null && source.setColumnWidth) {
			autoFitColumn(b);
			return;
		}
		const cell = cellAt(e);
		const pv = cell && pivotAt(cell.row, cell.col);
		if (cell && pv) {
			// Double clic sur −/+ : deux bascules, pas le volet.
			if (pv.rows && cell.col === pv.col && pivotBoxHit(pivotRowOf(pv, cell.row), toLocal(e).x - screenX(cell.col))) return;
			pivotActive();
			return;
		}
		if (cell) startEdit(false);
	}

	// --- Raccourcis d'Excel -------------------------------------------------------

	// Fin pressée : la flèche suivante va au bout de la zone.
	let endArmed = false;
	// Dernière action répétable (F4) : saisie, mise en forme, insertion…
	let lastRepeat: (() => void) | null = null;

	/** Exécute un raccourci de la grille ; faux s'il ne s'applique pas. */
	function runShortcut(sc: string): boolean {
		switch (sc) {
			case 'fillDown':
			case 'fillRight':
				copyFill(sc === 'fillDown');
				return true;
			case 'today':
			case 'now':
				startEdit(true, sc === 'today' ? todayText() : timeText());
				return true;
			case 'autoSum':
				autoSum();
				return true;
			case 'formatCells':
				// Ctrl+1 (shortcuts.ts, claviers AZERTY compris).
				openFormat();
				return true;
			case 'selectRow': {
				const t = tableAt(active.row, active.col);
				if (t && active.row > t.t.row) {
					// Dans un tableau : sa ligne, comme Excel.
					active = { row: active.row, col: t.t.col };
					anchor = { row: active.row, col: t.lastCol };
					refreshFx();
					refreshStatus();
					schedule();
				} else {
					// Cellule fusionnée : toutes ses lignes (Excel).
					const ar = activeRect();
					selectRows(ar.r0, ar.r1);
				}
				return true;
			}
			case 'toggleFilter':
				toggleFilters();
				return true;
			case 'formatTable':
				editActive('formatAsTable');
				return true;
			case 'strike':
				if (source.applyStyle) applyStyle({ x: 'toggle' });
				return true;
			case 'hideRows':
			case 'hideCols':
				hideLines(sc === 'hideRows', true);
				return true;
			case 'note':
				editNote();
				return true;
			case 'insertFunction':
				openFunctions();
				return true;
			case 'chart':
				chartActive('bar');
				return true;
			case 'repeat':
				if (lastRepeat) lastRepeat();
				else say('Rien à répéter');
				return true;
		}
		return false;
	}

	/** Ctrl+D / Ctrl+R : recopie la première ligne (colonne) de la sélection, ou celle du dessus (de gauche). */
	function copyFill(down: boolean) {
		if (!source.paste) return;
		const plan = copyFillPlan(selection(), down);
		if (!plan) return;
		const { dst } = plan;
		if ((dst.r1 - dst.r0 + 1) * (dst.c1 - dst.c0 + 1) > 10000) {
			say('Recopie limitée à 10 000 cellules');
			return;
		}
		const block = copyFillBlock(plan, down, (r, c) => cellInfo(r, c).value);
		const paste = source.paste;
		lastRepeat = () => copyFill(down);
		writing = writing.catch(() => undefined).then(async () => {
			try {
				await paste(dst.r0, dst.c0, block);
				message = `Recopié vers ${down ? 'le bas' : 'la droite'} (Ctrl+Z pour annuler)`;
			} catch (err) {
				message = String(err instanceof Error ? err.message : err);
			}
			refreshFx();
			refreshStatus();
			schedule();
		});
	}

	function onKeyDown(e: KeyboardEvent) {
		// Touche maintenue : le cadre suit sans animation.
		if (e.repeat) noAnimUntil = performance.now() + 150;
		if (editing) return;
		// Plage source d'une validation pointée : Entrée la reprend, Échap
		// annule ; on ne saisit rien dans la grille, on s'y déplace seulement.
		if (dvPick) {
			if (e.key === 'Enter' || e.key === 'Escape') {
				e.preventDefault();
				endPick(e.key === 'Enter');
				return;
			}
			const mod = e.ctrlKey || e.metaKey;
			if ((e.key.length === 1 && (!mod || /^[vxzyd]$/i.test(e.key))) || e.key === 'F2' || e.key === 'Delete' || e.key === 'Backspace') {
				e.preventDefault();
				return;
			}
		}
		// Feuille protégée : déplacement et lecture seulement (Rechercher,
		// copier, annuler, recalculer, comme le ruban), pas de saisie, sans
		// message ; les raccourcis de la page (Ctrl+S, onglets…) passent.
		if (locked && !hold && !readKey(e)) return;
		// Maj+F10 ou touche Menu : menu contextuel de la cellule active (ou du graphique sélectionné).
		if ((e.shiftKey && e.key === 'F10' && !e.ctrlKey && !e.altKey) || e.key === 'ContextMenu') {
			e.preventDefault();
			keyContextMenu();
			return;
		}
		if (e.key === 'Escape' && painter) painter = null;
		if (e.key === 'Escape' && links.length) {
			links = [];
			schedule();
		}
		if (e.key === 'Escape' && clipRange) {
			clipRange = null;
			schedule();
		}
		if (selChart !== null) {
			const ch = charts.find((c) => c.id === selChart);
			if (ch && (e.key === 'Delete' || e.key === 'Backspace')) {
				e.preventDefault();
				removeChart(ch.id);
				return;
			}
			if (e.key === 'Escape') {
				selChart = null;
				if (panel?.kind === 'chart') panel = null;
				return;
			}
			// Clavier : flèches pour déplacer, Maj+flèches pour redimensionner
			// (10 pixels), Tab et Maj+Tab pour passer au graphique suivant.
			const step = ({ ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] } as Record<string, number[]>)[e.key];
			if (ch && step && !e.ctrlKey && !e.altKey && !e.metaKey && !locked) {
				e.preventDefault();
				nudgeChart(ch, step[0] * 10, step[1] * 10, e.shiftKey);
				return;
			}
			if (ch && e.key === 'Tab' && charts.length > 1) {
				e.preventDefault();
				const i = charts.indexOf(ch);
				selectChart(charts[(i + (e.shiftKey ? charts.length - 1 : 1)) % charts.length]);
				say(`Graphique ${charts.indexOf(charts.find((c) => c.id === selChart)!) + 1} sur ${charts.length}`);
				return;
			}
		}
		// Raccourcis d'Excel (shortcuts.ts) ; Fin puis flèche saute comme Ctrl+flèche.
		const ended = endArmed;
		if (!['Shift', 'Control', 'Alt', 'Meta'].includes(e.key)) endArmed = false;
		const sc = shortcutOf(e);
		if (sc === 'nextSheet' || sc === 'prevSheet' || sc === 'insertSheet' || sc === 'nextChoice' || sc === 'prevChoice') return; // onglets : la page s'en charge
		if (sc === 'refresh' || sc === 'refreshAll') return; // F5 : la page s'en charge (tableHere)
		if (sc === 'endMode') {
			endArmed = !ended;
			say(endArmed ? 'Fin : une flèche va au bout de la zone' : '');
			e.preventDefault();
			return;
		}
		if (sc && runShortcut(sc)) {
			e.preventDefault();
			return;
		}
		const page = Math.max(1, Math.floor(vs.view / ROW_H) - 1);
		const { row, col } = active;
		const ext = e.shiftKey;
		const ctrl = e.ctrlKey || e.metaKey;
		// Maj : le coin mobile de la sélection bouge, la cellule active reste.
		const cur = ext ? anchor : active;
		const arrow = (dr: number, dc: number) => {
			if (ctrl || ended) {
				const from = ext ? anchor : mergeEdge(dr, dc);
				const [r, c] = jump(from.row, from.col, dr, dc);
				moveTo(r, c, ext);
			} else if (ext) {
				// Le bord de la plage (agrandie aux fusions) avance d'une cellule.
				const k = extendCorner(active, anchor, dr, dc, boxes, SHEET_ROWS - 1, SHEET_COLS - 1);
				moveTo(k.row, k.col, true);
			} else {
				const n = step(dr, dc);
				moveTo(n.row, n.col);
			}
		};
		// Alt+↓ : liste déroulante de la cellule (validation).
		if (e.altKey && e.key === 'ArrowDown' && listOf(row, col)) {
			e.preventDefault();
			openList();
			return;
		}
		if (ctrl && (e.key === 'z' || e.key === 'Z') && source.undo) {
			e.preventDefault();
			source.undo().catch((err) => say(String(err?.message ?? err)));
			return;
		}
		if (ctrl && (e.key === 'a' || e.key === 'A' || e.key === ' ')) {
			const t = tableAt(row, col);
			if (t) {
				e.preventDefault();
				active = { row: t.t.row, col: e.key === ' ' ? col : t.t.col };
				anchor = { row: t.last, col: e.key === ' ' ? col : t.lastCol };
				refreshFx();
				refreshStatus();
				schedule();
				return;
			}
		}
		if (ctrl && (e.key.toLowerCase() === 'f' || e.key.toLowerCase() === 'h') && source.find) {
			e.preventDefault();
			openFinder(e.key.toLowerCase() === 'h');
			return;
		}
		if (ctrl && ['b', 'i', 'u'].includes(e.key.toLowerCase()) && source.applyStyle) {
			e.preventDefault();
			applyStyle({ [e.key.toLowerCase()]: 'toggle' });
			return;
		}
		// Ctrl + et Ctrl - : insérer ou supprimer des lignes ou colonnes (comme Excel).
		if (ctrl && (e.key === '+' || (e.key === '=' && e.shiftKey) || e.key === '-') && source.sheetLines) {
			e.preventDefault();
			openLinesDialog(e.key !== '-');
			return;
		}
		if (ctrl && (e.key === 'y' || e.key === 'Y') && source.redo) {
			e.preventDefault();
			source.redo().catch((err) => say(String(err?.message ?? err)));
			return;
		}
		switch (e.key) {
			case 'ArrowDown':
				arrow(1, 0);
				break;
			case 'ArrowUp':
				arrow(-1, 0);
				break;
			case 'ArrowRight':
				arrow(0, 1);
				break;
			case 'ArrowLeft':
				arrow(0, -1);
				break;
			case 'PageDown':
				moveTo(cur.row + page, cur.col, ext);
				break;
			case 'PageUp':
				moveTo(cur.row - page, cur.col, ext);
				break;
			case 'Home':
				moveTo(ctrl ? 0 : cur.row, 0, ext);
				break;
			case 'End':
				if (ctrl) {
					// Dernière ligne et dernière colonne utilisées (tableaux et cellules libres), comme Excel.
					let last = entries.reduce((m, e) => Math.max(m, e.last), 0);
					let lastCol = entries.reduce((m, e) => Math.max(m, e.lastCol), 0);
					for (const k of cells.keys()) {
						const [r, c] = k.split(',').map(Number);
						last = Math.max(last, r);
						lastCol = Math.max(lastCol, c);
					}
					moveTo(last, lastCol, ext);
				}
				break;
			case 'Tab':
				tabMove(e.shiftKey);
				break;
			case 'Enter':
				enterMove(e.shiftKey);
				break;
			case 'F2':
				startEdit(false);
				break;
			case 'F9':
				if (source.recalc) source.recalc().then(() => say('Recalculé'), (err) => say(String(err?.message ?? err)));
				break;
			case 'Delete':
				// Colonnes entières d'un tableau (en-tête compris), pas tout le tableau : supprimées.
				if (deleteWholeColumns()) break;
				clearSelection();
				lastRepeat = () => clearSelection();
				break;
			case 'Backspace':
				startEdit(true, '');
				break;
			default:
				if (e.key.length === 1 && !ctrl && !e.altKey) startEdit(true, e.key);
				else return;
		}
		e.preventDefault();
	}

	// --- Édition ------------------------------------------------------------

	/**
	 * Pourquoi la cellule ne se modifie pas ('' si elle se modifie) : feuille
	 * protégée, tableau publié figé, ligne d'un autre membre, tableau croisé,
	 * tableau chargé par une requête (hors colonnes saisies). Elle s'ouvre
	 * alors en lecture seule (double clic, F2, barre de formule), pour copier.
	 */
	function readOnlyReason(r: number, c: number): string {
		if (locked) return locked;
		// Tableau publié en lecture seule (classeur partagé figé, hors ligne).
		const ro = ownerAt(r, c)?.t.readOnly;
		if (ro) return ro;
		// Participant (classeur collaboratif) : les lignes des autres membres.
		if (lockedRowAt(r, c)) return NOT_YOURS;
		if (pivotAt(r, c)) return PIVOT_READONLY;
		if (queryAt(r, c)) return QUERY_READONLY;
		return '';
	}

	/** Texte de la cellule dans l'éditeur et la barre de formule (un tableau croisé : sa valeur affichée). */
	function editText(r: number, c: number): string {
		const v = cellInfo(r, c).value;
		return v === '' && pivotAt(r, c) ? shownValue(r, c) : v;
	}

	function startEdit(byTyping: boolean, initial?: string) {
		const reason = readOnlyReason(active.row, active.col);
		const mode = editMode(reason, byTyping);
		if (mode === 'refuse') {
			say(reason);
			return;
		}
		if (mode === 'view') {
			openReadOnly(reason);
			return;
		}
		const band = bandAt(
			entries.map((x) => x.t),
			active.row,
			active.col
		);
		// Onglet à choix : les lignes des autres valeurs se remplissent comme
		// un onglet neuf (la ligne saisie reçoit la valeur choisie).
		if (band && !bandOpen(band)) {
			say(bandRefusal(band.name));
			return;
		}
		const info = cellInfo(active.row, active.col);
		// Ligne pas encore chargée (juste après une saisie qui l'a créée) : la
		// frappe ne doit pas être perdue ; seule l'édition de la valeur (F2)
		// attend les données.
		if (info.kind === 'data' && !info.row && initial === undefined) return;
		// Sélection d'avant la saisie : Ctrl+Entrée valide dans toute la plage.
		editRange = { sel: selection(), anchor: { ...anchor } };
		anchor = { ...active };
		editByTyping = byTyping;
		editValueText = initial ?? info.value;
		// Saisir dans une cellule annule le copier-coller en cours (Excel).
		if (clipRange) {
			clipRange = null;
			schedule();
		}
		// L'éditeur doit avoir le focus avant la frappe suivante : sinon une
		// saisie rapide perdrait des caractères.
		flushSync(() => {
			editRO = '';
			editing = true;
		});
		ensureVisible(active.row, active.col);
		if (!input) return;
		placeEditor();
		input.focus();
		const n = input.value.length;
		input.setSelectionRange(n, n);
	}

	/**
	 * Double clic ou F2 sur une cellule en lecture seule (readonlyedit.ts) :
	 * l'éditeur s'ouvre sur son texte, à sélectionner et copier ; rien ne s'y
	 * saisit, la barre d'état dit pourquoi.
	 */
	function openReadOnly(reason: string) {
		const info = cellInfo(active.row, active.col);
		if (info.kind === 'data' && !info.row) return; // ligne pas encore chargée
		editRange = null;
		anchor = { ...active };
		editByTyping = false;
		editValueText = editText(active.row, active.col);
		flushSync(() => {
			editRO = reason;
			editing = true;
		});
		say(reason);
		ensureVisible(active.row, active.col);
		if (!input) return;
		placeEditor();
		input.focus();
		const n = input.value.length;
		input.setSelectionRange(n, n);
	}

	/** Referme l'éditeur en lecture seule, sans rien écrire ni message de refus. */
	function closeReadOnly(focusGrid = true) {
		if (message === editRO) say('');
		editing = false;
		editRO = '';
		editErr = null;
		completion = sig = null;
		refreshFx();
		if (focusGrid) wrap.focus();
		schedule();
	}

	function placeEditor() {
		if (!input) return;
		const x = screenX(active.col);
		const y = screenY(active.row);
		const box = input.parentElement!;
		box.style.transform = `translate(${x}px, ${y}px)`;
		// Comme Excel : l'éditeur s'élargit vers la droite tant que les cellules
		// voisines sont vides, puis passe à la ligne ; une fusion le fait au moins
		// aussi grand qu'elle.
		const row = active.row;
		const g = boxOf(active.row, active.col);
		const longest = Math.max(...input.value.split('\n').map((l) => textWidth(l)));
		const empty = (c: number) => !tableAt(row, c) && !cells.has(key(row, c));
		const last = g ? g.col + g.cols - 1 : active.col;
		const width = editorWidth(longest + 16, active.col, (c) => cax().size(c), (c) => c > last && empty(c), Math.max(80, viewW - x - 4));
		const mergedW = g ? cax().offset(g.col + g.cols) - cax().offset(g.col) : 0;
		// Largeur de confort seulement si la cellule voisine est vide : l'éditeur
		// ne passe jamais par-dessus une cellule qui a un contenu (il passe à la
		// ligne dans sa colonne).
		const floor = empty(last + 1) ? 80 : 0;
		box.style.width = `${Math.max(width, mergedW, floor)}px`;
		input.style.height = '0px';
		editorH = Math.max(g ? rspan(g.row, g.row + g.rows - 1) : rh(row), input.scrollHeight + 4);
		input.style.height = '';
		box.style.height = `${editorH}px`;
		const e = tableAt(active.row, active.col);
		const st = styleAt(active.row, active.col, e);
		input.style.textAlign = isFormula(input.value) ? 'left' : (st?.al ?? (e && active.row > e.t.row ? alignment(e.t.columns[active.col - e.t.col].type) : 'left'));
		input.style.font = st?.ff || st?.fs || st?.b || st?.i ? cellFont(st) : '';
	}

	// Les saisies partent l'une après l'autre : la suivante peut dépendre de
	// la précédente (une saisie fait naître un tableau, la suivante le remplit).
	// Une écriture qui échoue ne bloque jamais les suivantes (catch).
	let writing: Promise<void> = Promise.resolve();

	// Classeur collaboratif : la cellule en cours de saisie est signalée à la
	// source, et libérée une fois la saisie écrite (ou abandonnée).
	let editingAt: { row: number; col: number } | null = null;
	$effect(() => {
		// Éditeur en lecture seule : rien ne se saisit, rien à signaler.
		const on = editing && !editRO;
		if (!source.editing) return;
		if (on && !editingAt) {
			editingAt = { row: active.row, col: active.col };
			source.editing(editingAt.row, editingAt.col, true).catch(() => undefined);
		} else if (!on && editingAt) {
			const at = editingAt;
			editingAt = null;
			const done = source.editing;
			writing = writing.catch(() => undefined).then(() => done(at.row, at.col, false).then(() => undefined, () => undefined));
		}
	});
	// keep : saisie au clavier ; refusée, son texte revient dans l'éditeur
	// (sur sa cellule) au lieu d'être perdu.
	// onrefused : reçoit le refus du moteur (renommage depuis la zone de nom).
	// key : réponse à la question de la clé (keyDlg).
	function write(row: number, col: number, raw: string, keep = false, onrefused?: (e: unknown) => void, key?: KeyChoice): Promise<void> {
		writing = writing.catch(() => undefined).then(() => writeNow(row, col, raw, keep, onrefused, key));
		return writing;
	}

	async function writeNow(row: number, col: number, raw: string, keep = false, onrefused?: (e: unknown) => void, key?: KeyChoice) {
		if (pivotAt(row, col)) {
			message = PIVOT_READONLY;
			return;
		}
		if (queryAt(row, col)) {
			message = QUERY_READONLY;
			return;
		}
		const info = cellInfo(row, col);
		const target =
			info.kind === 'free'
				? {}
				: { table: info.e!.t, column: info.i, index: info.kind === 'data' ? info.n : undefined, rowId: info.kind === 'data' ? info.row?.id : undefined };
		const t0 = performance.now();
		// Affichage immédiat (optimiste) d'une valeur saisie dans un tableau :
		// le moteur confirme ensuite (valeur typée), ou la saisie est annulée.
		const optimistic = info.kind === 'data' && info.n !== undefined && info.row && !isFormula(raw) && !info.e!.t.columns[info.i!]?.formula;
		const before = optimistic ? info.row!.cells[info.i!] : undefined;
		if (optimistic) {
			const v = isNumeric(raw) ? Number(raw.replace(/[\s  ]/g, '').replace(',', '.')) : raw;
			info.e!.cache.set(info.n!, info.i!, raw.trim() === '' ? null : v);
			schedule();
		}
		typed = { row, col, t: performance.now() };
		try {
			// Colonne sur un niveau saisie depuis la bande : c'est son en-tête.
			const m = await source.setCell(info.kind === 'header' ? info.e!.t.row : row, col, raw, target, key);
			lastEditMs = performance.now() - t0;
			if (info.kind === 'data' && isFormula(raw) && info.e) {
				const name = info.e.t.columns[info.i!]?.name;
				if (name) columnFlash = { table: info.e.t.id, name, t: performance.now(), done: new Set() };
				schedule();
			}
			if (m && typeof m === 'object') {
				if (info.kind === 'data' && info.n !== undefined) info.e!.cache.set(info.n, info.i!, m.value);
				message = '';
			} else message = m ?? '';
		} catch (e) {
			if (optimistic) info.e!.cache.set(info.n!, info.i!, before); // saisie refusée : valeur d'avant
			message = String(e instanceof Error ? e.message : e);
			// Formule refusée : la saisie n'est pas perdue, l'éditeur se rouvre.
			const k = message.indexOf('formule : ');
			const q = key ? null : keyQuestion(message);
			const hq = key ? null : handQuestion(message);
			if (q) {
				// Recherche dans un tableau sans clé : la boîte demande quoi utiliser.
				message = '';
				keyDlg = { ...q, row, col, raw, keep, choice: 'index' };
			} else if (hq) {
				// Première valeur d'une colonne saisie : la boîte demande la jointure.
				message = '';
				handDlg = { ...hq, choice: hq.columns[0] ?? '#', write: { row, col, raw, keep } };
			} else if (onrefused) onrefused(e);
			else if (k >= 0) {
				const msg = message.slice(k);
				queueMicrotask(() => reopenFormula(row, col, raw, msg));
			} else if (keep && !editing) {
				// Saisie refusée : le texte tapé reste dans l'éditeur.
				const why = message;
				active = { row, col };
				anchor = { ...active };
				startEdit(true, raw);
				message = why;
				// En-tête refusé (nom de colonne pris…) : la boîte l'explique.
				if (info.kind === 'header') void refuseName(() => input, refusedName('Nom de colonne refusé', e));
			}
		}
		refreshFx();
		refreshStatus();
		schedule();
	}

	/**
	 * Valide la saisie ; faux si une formule a une faute de syntaxe (l'éditeur
	 * reste alors ouvert sur le texte tapé, la faute soulignée et expliquée ;
	 * les parenthèses oubliées en fin de formule sont fermées d'office), ou si
	 * la validation de la cellule (liste déroulante) refuse la valeur.
	 */
	/** Nom de colonne tapé dans un en-tête : pourquoi il est refusé (déjà pris dans le tableau), null s'il passe. */
	function headerProblem(row: number, col: number, text: string): NameProblem | null {
		const info = cellInfo(row, col);
		if (info.kind !== 'header' || text === info.value || isFormula(text)) return null;
		const t = info.e!.t;
		// Deux lignes : l'en-tête se saisit sans son groupe, qui reste.
		const group = t.twoLevel ? splitGroup(t.columns[info.i].name).group : '';
		const full = group && text.trim() ? `${group}.${text}` : text;
		return columnNameProblem(full, t.columns.filter((_, i) => i !== info.i).map((c) => c.name), t.name);
	}

	function commit(): boolean {
		if (!editing) return true;
		// Lecture seule : rien à valider, l'éditeur se referme.
		if (editRO) {
			closeReadOnly();
			return true;
		}
		if (nameAlertOpen()) return false;
		if (isFormula(editValueText)) {
			const text = closeParens(editValueText);
			const err = checkFormula(text);
			if (err) {
				showFormulaError(err);
				return false;
			}
			editValueText = text;
		}
		const { row, col } = active;
		let text = editValueText;
		const named = headerProblem(row, col, text);
		if (named) {
			void refuseName(() => input, named);
			return false;
		}
		if (text !== cellInfo(row, col).value) {
			const r = refusal(row, col, text);
			if ('message' in r) {
				say(r.message);
				return false;
			}
			text = r.value;
		}
		editing = false;
		editErr = null;
		if (text !== cellInfo(row, col).value) {
			write(row, col, text, true);
			lastRepeat = () => write(active.row, active.col, text);
		}
		wrap.focus();
		return true;
	}

	// Sélection au début de la saisie (startEdit ne garde que la cellule active).
	let editRange: { sel: Range; anchor: { row: number; col: number } } | null = null;

	/** Ctrl+Entrée : la saisie va dans toute la sélection (une seule étape d'annulation), qui reste sélectionnée. */
	function commitAll() {
		const r = editRange;
		const sel = r?.sel;
		if (!r || !sel || isSingleCell(sel, active, boxes) || !source.paste) {
			commit();
			return;
		}
		if ((sel.r1 - sel.r0 + 1) * (sel.c1 - sel.c0 + 1) > 10000) {
			say('Ctrl+Entrée : sélection limitée à 10 000 cellules');
			return;
		}
		editing = false;
		const text = editValueText;
		const paste = source.paste;
		const fill = (s: Range) =>
			(writing = writing.catch(() => undefined).then(async () => {
				try {
					message = (await paste(s.r0, s.c0, fillBlock(s, text))) ?? '';
				} catch (err) {
					message = String(err instanceof Error ? err.message : err);
				}
				refreshFx();
				refreshStatus();
				schedule();
			}));
		fill(sel);
		lastRepeat = () => fill(selection());
		anchor = r.anchor;
		wrap.focus();
		refreshFx();
		schedule();
	}

	/** Remplace le texte de l'éditeur de cellule et place le curseur. */
	function setEditorText(value: string, caret: number) {
		flushSync(() => (editValueText = value));
		input?.setSelectionRange(caret, caret);
		if (input) onFormulaInput(input, 'cell');
	}

	/** Hauteur de l'éditeur : une ligne de plus par saut de ligne (Alt+Entrée). */
	function editorHeight(s: string): number {
		return 18 * s.split('\n').length + 4;
	}

	/**
	 * Formule validable (bouton « Valider ») : sans faute, ou presque, quand il
	 * ne manque que des parenthèses fermantes, que la validation ajoute.
	 */
	function formulaReady(text: string): boolean {
		return !checkFormula(closeParens(text));
	}

	/** Montre une faute de formule dans l'éditeur ouvert : message, caractère sélectionné. */
	function showFormulaError(err: FormulaError) {
		editErr = err;
		say(`Formule : ${err.msg}`);
		const el = input;
		if (!el) return;
		requestAnimationFrame(() => {
			el.focus();
			el.setSelectionRange(err.pos, Math.max(err.end, err.pos));
		});
	}

	/**
	 * Une formule refusée par le serveur (fonction mal employée, plage
	 * impossible…) : l'éditeur se rouvre sur le texte tapé, si l'utilisateur
	 * n'a pas commencé autre chose.
	 */
	function reopenFormula(row: number, col: number, raw: string, msg: string) {
		if (editing || fxEditing || !isFormula(raw)) return;
		moveTo(row, col);
		startEdit(true, raw);
		const pos = serverErrorPos(raw, msg);
		const text = msg.replace(/^formule : (position \d+ : )?/, '');
		if (pos !== null) showFormulaError({ pos, end: pos + 1, msg: text });
		else say(`Formule : ${text}`);
	}

	function cancel() {
		if (editRO) {
			closeReadOnly();
			return;
		}
		editing = false;
		editErr = null;
		refreshFx(); // la barre de formule reprend le contenu de la cellule
		wrap.focus();
		schedule();
	}

	/** ↑ / ↓ dans une formule : curseur au début ou à la fin du texte (Maj : sélection jusque-là). */
	function caretToEdge(end: boolean, extend: boolean) {
		const el = input;
		if (!el) return;
		const n = el.value.length;
		if (!extend) el.setSelectionRange(end ? n : 0, end ? n : 0);
		else if (end) el.setSelectionRange(el.selectionStart ?? 0, n, 'forward');
		else el.setSelectionRange(0, el.selectionEnd ?? n, 'backward');
		const s = catalog ? signature(el.value, el.selectionStart ?? n, catalog) : null;
		sig = s ? { ...s, where: 'cell' } : null;
	}

	/**
	 * Efface la sélection : tableaux entièrement compris supprimés, contenu des
	 * autres cellules de tableau effacé. Cellules libres : contenu seul, ou
	 * tout (all : Supprimer, avec mise en forme, fusions, notes, validations).
	 */
	async function clearSelection(range = selection(), all = false) {
		const { r0, r1, c0, c1 } = range;
		if (queryIn(r0, c0, r1, c1, true)) {
			say(QUERY_READONLY);
			return;
		}
		// Tableaux croisés : pris en entier (ou en erreur), supprimés avec le
		// reste ; touchés en partie seulement, rien ne s'efface (Excel).
		const pv = pivotsCleared(pivots, range);
		if (pv.partial) {
			say(PIVOT_PARTIAL);
			return;
		}
		const pivotGone = pv.gone.length === 1 ? `Tableau croisé « ${pv.gone[0].config.name ?? ''} » supprimé (Ctrl+Z pour annuler)` : pv.gone.length ? `${pv.gone.length} tableaux croisés supprimés (Ctrl+Z pour annuler)` : '';
		if (panel?.kind === 'pivot' && pv.gone.some((p) => p.id === panel!.id)) panel = null;
		const wipe = all ? source.clearCells : undefined;
		// Les tableaux entièrement compris dans la zone sont supprimés.
		const whole = entries.filter((e) => e.t.row >= r0 && e.last <= r1 && e.t.col >= c0 && e.lastCol <= c1);
		if (source.clearRange) {
			// Tout en une seule étape d'annulation.
			const clear = source.clearRange;
			const tables = whole.map((e) => e.t);
			writing = writing.catch(() => undefined).then(() =>
				clear(r0, c0, r1, c1, all).then(
					() => {
						if (tables.length > 1) say(`${tables.length} tableaux supprimés (Ctrl+Z pour annuler)`);
						else if (tables.length) say(`Tableau « ${tables[0].name} » supprimé (Ctrl+Z pour annuler)`);
						else if (pivotGone) say(pivotGone);
						else if (all) say('Cellules supprimées (Ctrl+Z pour annuler)');
						refreshFx();
						schedule();
					},
					(err) => say(String(err?.message ?? err))
				)
			);
			await writing;
			return;
		}
		const rest = (r: number, c: number) => !whole.some((e) => inTable(e, r, c));
		let n = 0;
		for (const e of entries) {
			if (whole.includes(e)) continue;
			const rr = Math.max(0, Math.min(r1, e.last) - Math.max(r0, e.t.row) + 1);
			const cc = Math.max(0, Math.min(c1, e.lastCol) - Math.max(c0, e.t.col) + 1);
			n += rr * cc;
		}
		if (!wipe)
			for (const k of cells.keys()) {
				const [r, c] = k.split(',').map(Number);
				if (r >= r0 && r <= r1 && c >= c0 && c <= c1) n++;
			}
		if (n > 1000) {
			say('Effacement limité à 1 000 cellules pour l’instant');
			return;
		}
		if (pv.gone.length && source.deletePivot) {
			const del = source.deletePivot;
			const ids = pv.gone.map((p) => p.id);
			writing = writing.catch(() => undefined).then(async () => {
				try {
					for (const id of ids) await del(id);
					say(pivotGone);
				} catch (err) {
					say(String((err as Error)?.message ?? err));
				}
			});
			await writing;
		}
		if (whole.length && source.deleteTables) {
			const del = source.deleteTables;
			const tables = whole.map((e) => e.t);
			writing = writing.catch(() => undefined).then(() =>
				del(tables).then(
					() => say(tables.length > 1 ? `${tables.length} tableaux supprimés (Ctrl+Z pour annuler)` : `Tableau « ${tables[0].name} » supprimé (Ctrl+Z pour annuler)`),
					(err) => say(String(err?.message ?? err))
				)
			);
			await writing;
		}
		for (const e of entries) {
			if (whole.includes(e)) continue;
			for (let r = Math.max(r0, e.t.row); r <= Math.min(r1, e.last); r++)
				for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol); c++) {
					const info = cellInfo(r, c);
					if (info.kind !== 'header' && info.value !== '') await write(r, c, '');
				}
		}
		if (wipe) {
			writing = writing.catch(() => undefined).then(() =>
				wipe(r0, c0, r1, c1).then(
					() => say('Cellules supprimées (Ctrl+Z pour annuler)'),
					(err) => say(String(err?.message ?? err))
				)
			);
			await writing;
			return;
		}
		for (const k of [...cells.keys()]) {
			const [r, c] = k.split(',').map(Number);
			if (r >= r0 && r <= r1 && c >= c0 && c <= c1 && rest(r, c)) await write(r, c, '');
		}
	}

	function onEditorKey(e: KeyboardEvent) {
		if (editRO) {
			// Lecture seule : copier, sélectionner, déplacer le curseur ; rien d'autre.
			e.stopPropagation();
			const k = viewKey(e);
			if (k === 'native') return;
			e.preventDefault();
			if (k === 'close') closeReadOnly();
			else if (k === 'enter') {
				closeReadOnly();
				enterMove(e.shiftKey);
			} else if (k === 'tab') {
				closeReadOnly();
				tabMove(e.shiftKey);
			}
			return;
		}
		if (input && completionKey(e, input, (v) => (editValueText = v))) return;
		e.stopPropagation();
		const { row, col } = active;
		const sc = shortcutOf(e);
		if ((sc === 'today' || sc === 'now') && input) {
			// Date ou heure du jour au curseur, comme Excel pendant la saisie.
			const r = insertAt(input.value, input.selectionStart, input.selectionEnd, sc === 'today' ? todayText() : timeText());
			setEditorText(r.value, r.caret);
			e.preventDefault();
			return;
		}
		switch (e.key) {
			case 'Enter':
				if (e.altKey && input) {
					// Alt+Entrée : saut de ligne dans la cellule ; les flèches
					// déplacent alors le curseur dans le texte.
					const r = insertAt(input.value, input.selectionStart, input.selectionEnd, '\n');
					setEditorText(r.value, r.caret);
					editByTyping = false;
					break;
				}
				if (e.ctrlKey || e.metaKey) {
					commitAll();
					break;
				}
				if (commit()) enterMove(e.shiftKey);
				break;
			case 'Tab':
				if (commit()) tabMove(e.shiftKey);
				break;
			case 'Escape':
				cancel();
				break;
			case 'ArrowDown':
			case 'ArrowUp':
			case 'ArrowLeft':
			case 'ArrowRight':
				// Formule : les flèches déplacent le curseur dans le texte (← → d'un
				// caractère, Ctrl d'un mot, ↑ ↓ au début et à la fin, Maj pour
				// sélectionner) ; seuls Entrée, Tab et le clic ailleurs valident.
				if (isFormula(editValueText) || editValueText.trimStart() === '=') {
					if (e.key === 'ArrowUp' || e.key === 'ArrowDown') caretToEdge(e.key === 'ArrowDown', e.shiftKey);
					else return;
					break;
				}
				if (!editByTyping) return;
				if (!commit()) break;
				moveTo(
					row + (e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0),
					col + (e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0)
				);
				break;
			default:
				return;
		}
		e.preventDefault();
	}

	// Barre de formule : même chemin d'écriture que l'éditeur de cellule.
	function onFxKey(e: KeyboardEvent) {
		if (fxInput && completionKey(e, fxInput, (v) => (fxText = v))) return;
		e.stopPropagation();
		if (hold && (e.key === 'Enter' || e.key === 'Escape')) {
			// Formule pointée sur une autre feuille : validée (Entrée) ou
			// abandonnée (Échap) en revenant à sa feuille, comme Excel.
			e.preventDefault();
			finishHold(e.key === 'Enter' ? 'commit' : 'cancel');
			return;
		}
		if (fxLock && !fxEditing) {
			// Lecture seule : Entrée ou Échap rendent la main à la grille, sans rien changer.
			if (e.key === 'Enter' || e.key === 'Escape') {
				e.preventDefault();
				wrap.focus();
			}
			return;
		}
		if (e.key === 'Escape') {
			fxEditing = false;
			editErr = null;
			refreshFx();
			wrap.focus();
		} else if (e.key === 'Enter') {
			const { row, col } = active;
			e.preventDefault();
			if (commitFx(true) === true) moveTo(row + 1, col);
			return;
		} else return;
		e.preventDefault();
	}

	/**
	 * Valide la saisie de la barre de formule dans la cellule active (Entrée,
	 * clic dans la grille, focus parti ailleurs), comme l'éditeur de cellule.
	 * 'formula' (formule fautive) ou 'refused' (valeur ou nom refusé) quand la
	 * saisie reste ouverte ; true si validée (ou rien à valider).
	 */
	function commitFx(focusGrid: boolean): true | 'formula' | 'refused' {
		if (!fxEditing) return true;
		if (nameAlertOpen()) return 'refused';
		const { row, col } = active;
		// Rien de changé (focus pris puis rendu) : la barre se referme, sans vérifier la formule.
		if (fxText !== cellInfo(row, col).value) {
			if (isFormula(fxText)) {
				const text = closeParens(fxText);
				const err = checkFormula(text);
				if (err) {
					editErr = err;
					say(`Formule : ${err.msg}`);
					const el = fxInput;
					if (el && document.activeElement === el) el.setSelectionRange(err.pos, Math.max(err.end, err.pos));
					else
						requestAnimationFrame(() => {
							el?.focus();
							el?.setSelectionRange(err.pos, Math.max(err.end, err.pos));
						});
					return 'formula';
				}
				fxText = text;
			}
		}
		editErr = null;
		let text = fxText;
		const named = headerProblem(row, col, text);
		if (named) {
			void refuseName(fxInput, named);
			return 'refused';
		}
		if (text !== cellInfo(row, col).value) {
			const r = refusal(row, col, text);
			if ('message' in r) {
				say(r.message);
				return 'refused';
			}
			text = r.value;
		}
		fxEditing = false;
		completion = sig = null;
		if (text !== cellInfo(row, col).value) {
			write(row, col, text);
			lastRepeat = () => write(active.row, active.col, text);
		}
		if (focusGrid) wrap.focus();
		refreshFx();
		return true;
	}

	/** Saisie en cours (cellule ou barre de formule) validée avant un clic dans la grille ; faux si elle reste ouverte. */
	function commitEditor(): boolean {
		if (editing) return commit();
		return commitFx(true) === true;
	}

	/** Barre de formule quittée (blur) : validée, gardée ou refermée (crossref.ts, fxLeave). */
	function leaveFx() {
		completion = sig = null;
		const action = fxLeave({
			editing: fxEditing,
			stillFocused: !!fxInput && document.activeElement === fxInput,
			alertOpen: nameAlertOpen(),
			pointing: !!hold || tabPress,
			text: fxText
		});
		if (action === 'keep') return;
		if (action === 'commit') {
			const r = commitFx(false);
			// Formule fautive : la saisie reste ouverte, le focus y revient (commitFx).
			if (r === 'formula') return;
			// Valeur refusée quand le focus part ailleurs : abandonnée, le message reste (comme l'éditeur de cellule).
			if (r === 'refused' && nameAlertOpen()) return;
		}
		fxEditing = false;
		editErr = null;
		refreshFx();
	}

	// --- Zone de nom ------------------------------------------------------------

	const gotoTable = (e: Entry): GotoTable => ({ name: e.t.name, row: e.t.row, col: e.t.col, last: e.last, lastCol: e.lastCol, columns: e.t.columns.map((c) => c.name) });

	/**
	 * Zone de nom, comme Excel : une adresse (F10, Cell[F10], A1:C3), un
	 * tableau ou une de ses cellules (Ventes[Prix][3]) y va ; faux sinon.
	 */
	function goToName(text: string): boolean {
		const cur = ownerAt(active.row, active.col);
		const g = parseGoto(text, entries.map(gotoTable), cur ? gotoTable(cur) : undefined);
		if (!g) return false;
		wrap.focus();
		if (g.r0 === g.r1 && g.c0 === g.c1) moveTo(g.r0, g.c0);
		else selectRange(g.r0, g.c0, g.r1, g.c1);
		return true;
	}

	/** Nom du tableau : une adresse y va (comme Excel), un autre nom renomme le tableau. */
	function onTableNameKey(e: KeyboardEvent) {
		e.stopPropagation();
		const el = e.currentTarget as HTMLInputElement;
		const t = ownerAt(active.row, active.col)?.t;
		if (e.key === 'Escape' || !t) {
			el.value = t?.name ?? '';
			wrap.focus();
			return;
		}
		if (e.key !== 'Enter') return;
		e.preventDefault();
		const typed = el.value;
		const name = typed.trim();
		// Le champ revient au nom du tableau (aller ailleurs ne le change pas
		// toujours : même tableau, la valeur liée ne bouge pas).
		el.value = t.name;
		if (name !== t.name && goToName(name)) return;
		if (!source.renameTable || name === t.name) {
			wrap.focus();
			return;
		}
		// Nom refusé : la boîte l'explique, le champ garde le texte tapé.
		const problem = tableNameProblem(name, entries.filter((x) => x.t !== t).map((x) => x.t.name));
		if (problem) {
			void refuseName(el, problem, typed);
			return;
		}
		el.value = typed;
		const rename = source.renameTable;
		writing = writing.catch(() => undefined).then(async () => {
			try {
				await rename(t, name);
				message = `Tableau renommé « ${name} »`;
				if (document.activeElement === el) wrap.focus();
			} catch (err) {
				void refuseName(el, refusedName('Nom de tableau refusé', err), typed);
			}
			refreshFx();
			refreshStatus();
		});
	}

	/** Colonne[ligne] : Entrée renomme la colonne. */
	function onColumnNameKey(e: KeyboardEvent) {
		e.stopPropagation();
		const el = e.currentTarget as HTMLInputElement;
		const en = ownerAt(active.row, active.col);
		if (e.key === 'Escape' || !en) {
			el.value = tableBox?.col ?? '';
			wrap.focus();
			return;
		}
		if (e.key !== 'Enter') return;
		e.preventDefault();
		const c = active.col;
		const text = el.value.trim();
		// Bande des groupes : renomme le groupe (toutes ses colonnes).
		if (active.row < en.t.row && !singleCol(en, c - en.t.col)) {
			wrap.focus();
			if (text !== tableBox?.col) write(active.row, c, text, false, (err) => void refuseName(el, refusedName("Groupe d'en-têtes refusé", err), text));
			return;
		}
		// Une autre cellule (Prix[3], Ventes[Prix][2], Cell[F10], A1:C3) : y
		// aller ; un nom seul (même « Q1 ») renomme la colonne.
		if (text !== tableBox?.col && /[[:]/.test(text)) {
			el.value = tableBox?.col ?? '';
			if (goToName(text)) return;
			el.value = text;
		}
		const name = text.replace(/\[.*$/, '').trim();
		const old = en.t.columns[c - en.t.col].name;
		if (name === old) {
			el.value = tableBox?.col ?? '';
			wrap.focus();
			return;
		}
		// Nom vide ou pris : la boîte l'explique, le champ garde le texte tapé.
		const problem = name
			? columnNameProblem(name, en.t.columns.filter((_, i) => i !== c - en.t.col).map((x) => x.name), en.t.name)
			: { title: 'Nom de colonne invalide', message: 'Le nom de la colonne ne peut pas être vide.\n\nTapez un nom, ou Échap pour garder l’ancien.' };
		if (problem) {
			void refuseName(el, problem, text);
			return;
		}
		wrap.focus();
		write(en.t.row, c, name, false, (err) => void refuseName(el, refusedName('Nom de colonne refusé', err), text));
	}

	function onNameKey(e: KeyboardEvent) {
		e.stopPropagation();
		const el = e.currentTarget as HTMLInputElement;
		if (e.key === 'Escape') {
			el.value = nameBox;
			wrap.focus();
			return;
		}
		if (e.key !== 'Enter') return;
		e.preventDefault();
		const text = el.value.trim();
		// Le nom d'une cellule existante : y aller (comme Excel).
		for (const [k, n] of names) {
			if (n.toLowerCase() === text.toLowerCase()) {
				const [r, c] = k.split(',').map(Number);
				wrap.focus();
				moveTo(r, c);
				return;
			}
		}
		// Une adresse (B3, Cell[B3], A1:C3) ou un tableau : y aller.
		if (goToName(text)) return;
		const { row, col } = active;
		if (tableAt(row, col)) {
			say('Seule une cellule libre peut être nommée ; un tableau se référence par son nom : ' + tableAt(row, col)!.t.name);
			wrap.focus();
			return;
		}
		if (!source.setName || text === (names.get(key(row, col)) ?? '')) {
			wrap.focus();
			return;
		}
		// Nom refusé : la boîte l'explique, le champ garde le texte tapé.
		const problem = cellNameProblem(text);
		if (problem) {
			void refuseName(el, problem, text);
			return;
		}
		const setName = source.setName;
		writing = writing.catch(() => undefined).then(async () => {
			try {
				await setName(row, col, text);
				message = text ? `Cellule nommée « ${text} »` : 'Nom retiré';
			} catch (err) {
				void refuseName(() => nameInput, refusedName('Nom de cellule refusé', err), text);
			}
			refreshFx();
			refreshStatus();
		});
		wrap.focus();
	}

	// --- Rechercher / remplacer -------------------------------------------------

	// Comme Excel, une sélection de plusieurs cellules borne la recherche
	// (zone) : elle reste affichée et la cellule trouvée (at) y est
	// encadrée ; une seule cellule : toute la feuille, la cellule active va
	// sur chaque occurrence.
	// count : compteur du champ (« 3 sur 12 », « Aucun résultat ») ; info :
	// bilan de Remplacer tout ou erreur, sous les champs.
	let finder = $state<{
		replace: boolean;
		text: string;
		by: string;
		count: string;
		info: string;
		zone: Zone | null;
		at: { row: number; col: number } | null;
	} | null>(null);
	let findInput = $state<HTMLInputElement>();
	let replaceInput = $state<HTMLInputElement>();
	let matches: { row: number; col: number }[] = [];
	// Recherche des occurrences en mémoire (texte et zone, findKey).
	let matchKey = '';
	// Barre ouverte ou fermée : redessiner le surlignage des occurrences.
	$effect(() => {
		void finder;
		schedule();
	});

	// Cellule trouvée, repérée à l'image en cours (ligne et colonne entières teintées).
	let findCross: Zone | null = null;

	/** Étendue de la cellule trouvée à repérer (toute sa fusion), ou null. */
	function foundRect(): Zone | null {
		const f = finder && foundCell(finder.at, finder.zone, active, editing);
		if (!f) return null;
		const g = boxOf(f.row, f.col);
		return g ? mergeRect(g) : { r0: f.row, r1: f.row, c0: f.col, c1: f.col };
	}

	/** Zone de recherche : la sélection si elle compte plusieurs cellules, sinon null (toute la feuille). */
	const searchZone = () => findZone(selection(), singleSelection());

	/** Suit la sélection : zone affichée dans la barre, cellule trouvée oubliée hors zone. */
	function syncFinderZone() {
		if (!finder) return;
		const z = searchZone();
		const old = finder.zone;
		if (findKey('', z) !== findKey('', old)) {
			finder.zone = z;
			finder.count = '';
		}
		if (finder.at && (z ? !inZone(finder.at, z) : finder.at.row !== active.row || finder.at.col !== active.col)) finder.at = null;
	}

	/** Ouvre la barre de recherche (replace : avec le champ Remplacer par). */
	export function openFinder(replace: boolean) {
		if (!source.find) return;
		// Rendue tout de suite : le focus y est avant la frappe suivante.
		flushSync(() => {
			finder = { replace, text: finder?.text ?? '', by: finder?.by ?? '', count: '', info: '', zone: searchZone(), at: null };
		});
		findInput?.select();
		// Ouverte depuis un menu qui reprend le focus en se fermant : y revenir.
		requestAnimationFrame(() => {
			if (findInput && !findInput.closest('.finder')?.contains(document.activeElement)) findInput.select();
		});
	}

	/** Ferme la barre et rend la main à la grille. */
	function closeFinder() {
		finder = null;
		wrap.focus();
	}

	/** Affiche ou masque la ligne Remplacer par (le focus y va à l'ouverture). */
	function toggleReplace(open = !finder?.replace) {
		if (!finder || !source.replace) return;
		const f = finder;
		flushSync(() => (f.replace = open));
		if (open) replaceInput?.select();
		else findInput?.focus();
	}

	/** Occurrences du texte cherché dans la zone (relues si le texte ou la zone a changé). */
	async function findAll(zone: Zone | null) {
		if (!finder || !source.find) return;
		const k = findKey(finder.text, zone, data.sheet ?? 0);
		if (k !== matchKey) {
			matches = await source.find(finder.text, zone);
			matchKey = k;
		}
	}

	async function findNext(back = false) {
		if (!finder || !source.find || !finder.text) return;
		const zone = searchZone();
		finder.zone = zone;
		await findAll(zone);
		if (!finder) return;
		finder.info = '';
		if (matches.length === 0) {
			finder.at = null;
			finder.count = 'Aucun résultat';
			schedule();
			return;
		}
		// Dans la sélection : à partir de la dernière cellule trouvée (la
		// sélection ne bouge pas) ; sur la feuille : de la cellule active.
		const from = zone ? (finder.at && inZone(finder.at, zone) ? finder.at : null) : active;
		const k = nextHit(matches, from, back);
		const m = matches[k];
		if (zone) {
			finder.at = { row: m.row, col: m.col };
			ensureVisible(m.row, m.col);
			schedule();
		} else {
			moveTo(m.row, m.col);
			finder.at = { row: active.row, col: active.col };
		}
		finder.count = hitCount(k, matches.length);
	}

	/** Remplace la cellule trouvée (si elle contient le texte), puis passe à la suivante (Excel). */
	async function replaceOne() {
		if (!finder || !source.replace || !finder.text) return;
		const zone = searchZone();
		await findAll(zone);
		if (!finder) return;
		const at = zone ? finder.at : active;
		if (at && inZone(at, zone) && matches.some((m) => m.row === at.row && m.col === at.col)) {
			try {
				await source.replace(finder.text, finder.by, { r0: at.row, c0: at.col, r1: at.row, c1: at.col });
			} catch (e) {
				finder.info = String(e instanceof Error ? e.message : e);
				return;
			}
			matchKey = '';
		}
		await findNext();
	}

	async function replaceAll() {
		if (!finder || !source.replace || !finder.text) return;
		const { text, by } = finder;
		const zone = searchZone();
		try {
			const n = await source.replace(text, by, zone);
			if (!finder) return;
			finder.info = replacedLabel(n, !!zone);
			finder.count = '';
			finder.at = null;
			matchKey = '';
		} catch (e) {
			if (finder) finder.info = String(e instanceof Error ? e.message : e);
		}
	}

	/** Touches dans toute la barre (champs et boutons), voir finderAction. */
	function onFinderKey(e: KeyboardEvent) {
		e.stopPropagation();
		const a = finderAction(e);
		// Entrée sur un bouton : le bouton agit, pas la recherche.
		if (!a || (e.key === 'Enter' && e.target instanceof HTMLButtonElement)) return;
		e.preventDefault();
		if (a === 'close') closeFinder();
		else if (a === 'next' || a === 'prev') findNext(a === 'prev');
		else if (a === 'focusFind') findInput?.select();
		else if (a === 'openReplace') toggleReplace(true);
	}

	// --- Presse-papiers -----------------------------------------------------

	function selection(): CellRange {
		// Lignes ou colonnes entières : exactement celles choisies (Excel).
		if (wholeKind()) return { ...wholeSel!.range };
		// Agrandie aux fusions qu'elle touche, entières (comme Excel).
		return expandRange(rangeOf(active, anchor), boxes);
	}

	/** La sélection n'est-elle que la cellule active (ou sa fusion) ? */
	const singleSelection = () => isSingleCell(selection(), active, boxes);

	// Zone copiée ou coupée : cadre pointillé, sur sa feuille. Collée dans
	// Lattis, elle emporte formules, mise en forme et tableaux (transfer) ;
	// les autres applications reçoivent le texte tabulé des valeurs affichées.
	// Le cadre disparaît au collage d'une zone coupée, sur Échap et dès qu'on
	// saisit dans une cellule (Excel).
	let clipRange = $state<Clip | null>(null);
	const sheetId = () => data.sheet ?? 0;

	/**
	 * Valeur affichée d'une cellule, comme Excel la donne aux autres
	 * applications : le résultat d'une formule (cellule libre, colonne
	 * calculée), les valeurs d'un tableau croisé ou d'un tableau déversé.
	 */
	function shownValue(r: number, c: number): string {
		const info = cellInfo(r, c);
		if (info.kind === 'free') {
			const f = shown.get(key(r, c));
			if (f) return f.text;
			if (info.value) return info.value;
			const p = pivotAt(r, c);
			if (p) {
				// Comme affichées (format de nombre du TCD), comme Excel les copie.
				const v = p.cells[r - p.row]?.[c - p.col] ?? '';
				return v === '' ? '' : pivotValueText(formatCell(v, undefined, styleAt(r, c)), r - p.row, c - p.col);
			}
			for (const sp of spills) {
				const v = sp.rows[r - sp.row]?.[c - sp.col];
				if (v !== undefined) return v;
			}
			return '';
		}
		if (info.kind === 'data' && info.e.t.columns[info.i].formula !== undefined) {
			const k = info.e.t.columns[info.i];
			return info.row ? editValue((k.result ?? 'text') as ColumnType, info.row.cells[info.i]) : '';
		}
		return info.value;
	}

	/** Valeurs affichées d'une zone (null si trop grande). */
	function rangeValues(sel: ClipRect): string[][] | null {
		const { r0, r1, c0, c1 } = sel;
		if ((r1 - r0 + 1) * (c1 - c0 + 1) > 100_000) {
			say('Copie limitée à 100 000 cellules pour l’instant');
			return null;
		}
		const rows: string[][] = [];
		for (let r = r0; r <= r1; r++) {
			const line: string[] = [];
			for (let c = c0; c <= c1; c++) line.push(shownValue(r, c));
			rows.push(line);
		}
		return rows;
	}

	/** Lignes de tableau de la zone (rangs de données) pas encore chargées : [entrée, première, dernière]. */
	function unloaded(sel: ClipRect): [Entry, number, number][] {
		const out: [Entry, number, number][] = [];
		// Trop grande pour être copiée (voir rangeValues) : rien à lire.
		if ((sel.r1 - sel.r0 + 1) * (sel.c1 - sel.c0 + 1) > 100_000) return out;
		for (const e of entries) {
			if (e.t.col > sel.c1 || e.lastCol < sel.c0) continue;
			const a = Math.max(sel.r0, e.t.row + 1) - e.t.row - 1;
			const b = Math.min(sel.r1, e.last) - e.t.row - 1;
			if (a <= b && !e.cache.has(a, b)) out.push([e, a, b]);
		}
		return out;
	}

	/** Charge les lignes de la zone jamais affichées (copie d'une grande plage), barre de progression comprise. */
	async function loadRange(sel: ClipRect): Promise<boolean> {
		const todo = unloaded(sel);
		if (!todo.length) return true;
		say('Lecture des lignes à copier…');
		const ok = await Promise.all(todo.map(([e, a, b]) => e.cache.ensure(a, b)));
		return ok.every(Boolean);
	}

	/** Copie ou coupe après avoir chargé les lignes manquantes (le presse-papiers est écrit ensuite). */
	async function copyLater(cut: boolean) {
		if (!(await loadRange(selection()))) {
			say('Le tableau a changé pendant la copie : recommencez');
			return;
		}
		const text = takeClip(cut);
		if (text === null) return;
		try {
			await navigator.clipboard.writeText(text);
		} catch {
			clipRange = null;
			say('Le presse-papiers est inaccessible');
			return;
		}
		clipMessage(cut);
	}

	/** Retient la sélection copiée ou coupée ; renvoie le texte pour le presse-papiers (null : rien). */
	function takeClip(cut: boolean): string | null {
		const sel = selection();
		const values = rangeValues(sel);
		if (!values) return null;
		const text = toTSV(values);
		// Un tableau croisé se copie en valeurs, sauf pris en entier : il se
		// colle alors en tableau croisé (copie, ou déplacé s'il est coupé).
		// Une partie seulement ne se coupe pas (Excel).
		const pv = pivotIn(sel.r0, sel.c0, sel.r1, sel.c1);
		let pivot: Clip['pivot'];
		if (pv) {
			const top = pv.anchor ?? pv.row;
			const whole = sel.r0 <= top && sel.c0 <= pv.col && sel.r1 >= pv.row + pv.cells.length - 1 && sel.c1 >= pv.col + pv.cells[0].length - 1;
			if (whole) pivot = { id: pv.id, dr: top - sel.r0, dc: pv.col - sel.c0 };
			else if (cut) {
				say('Une partie d’un tableau croisé ne se coupe pas : sélectionnez-le en entier pour le déplacer, ou copiez ses valeurs (Ctrl+C)');
				return null;
			}
		}
		const plain = !source.transfer || !!pv;
		clipRange = { ...sel, sheet: sheetId(), cut, text, values, plain, pivot };
		// Un tableau croisé entier se colle aussi dans un document.
		rememberObj(text, pivot && pv ? { kind: 'pivot', config: $state.snapshot(pv.config) as never } : null);
		schedule();
		return text;
	}

	function clipMessage(cut: boolean) {
		const { r0, r1, c0, c1 } = clipRange ?? selection();
		const n = (r1 - r0 + 1) * (c1 - c0 + 1);
		say(cut ? 'Sélection coupée : collez-la avec Ctrl+V (Échap pour annuler)' : `${n} cellule${n > 1 ? 's' : ''} copiée${n > 1 ? 's' : ''}`);
	}

	/** Copier ou couper la sélection (ruban, clic droit). */
	export async function copySelection(cut = false) {
		wrap.focus();
		const sc = selChart !== null ? charts.find((c) => c.id === selChart) : undefined;
		if (sc) return copyChart(sc, cut);
		if (unloaded(selection()).length) return copyLater(cut);
		const text = takeClip(cut);
		if (text === null) return;
		try {
			await navigator.clipboard.writeText(text);
		} catch {
			clipRange = null;
			say('Le presse-papiers est inaccessible : utilisez Ctrl+C');
			return;
		}
		clipMessage(cut);
	}

	/** Coller à la cellule active (ruban, clic droit). */
	export async function pasteClipboard() {
		wrap.focus();
		let text = '';
		try {
			text = await navigator.clipboard.readText();
		} catch {
			say('Le presse-papiers est inaccessible : utilisez Ctrl+V');
			return;
		}
		pasteText(text);
	}

	function onCut(e: ClipboardEvent) {
		if (document.activeElement !== wrap || editing) return;
		const sc = selChart !== null ? charts.find((c) => c.id === selChart) : undefined;
		if (sc) {
			e.preventDefault();
			e.clipboardData?.setData('text/plain', encodeObj({ kind: 'chart', config: sc.config as never }));
			chartClip = sc.config;
			removeChart(sc.id);
			return;
		}
		// Lignes jamais affichées : lues d'abord, le presse-papiers est écrit ensuite.
		if (unloaded(selection()).length) {
			e.preventDefault();
			void copyLater(true);
			return;
		}
		const text = takeClip(true);
		if (text === null) return;
		e.clipboardData?.setData('text/plain', text);
		e.preventDefault();
		clipMessage(true);
	}

	function onCopy(e: ClipboardEvent) {
		if (document.activeElement !== wrap || editing) return;
		const sc = selChart !== null ? charts.find((c) => c.id === selChart) : undefined;
		if (sc) {
			e.clipboardData?.setData('text/plain', encodeObj({ kind: 'chart', config: sc.config as never }));
			e.preventDefault();
			chartClip = sc.config;
			say('Graphique copié : Ctrl+V pour le coller à la cellule active');
			return;
		}
		if (unloaded(selection()).length) {
			e.preventDefault();
			void copyLater(false);
			return;
		}
		const text = takeClip(false);
		if (text === null) return;
		e.clipboardData?.setData('text/plain', text);
		e.preventDefault();
		clipMessage(false);
	}

	function onPaste(e: ClipboardEvent) {
		if (document.activeElement !== wrap || editing || !source.paste) return;
		const text = e.clipboardData?.getData('text/plain') ?? '';
		if (pasteText(text)) e.preventDefault();
	}

	/**
	 * Colle un texte (TSV ou graphique) sur la sélection ; faux si rien à
	 * coller. Le texte de la dernière copie de Lattis se colle « riche »
	 * (transfer : formules, mise en forme, tableaux ; une zone coupée se
	 * déplace). Comme Excel, une sélection multiple du bloc copié le reçoit
	 * répété (une cellule copiée remplit toute la sélection).
	 */
	function pasteText(text: string): boolean {
		if (!source.paste) return false;
		const sel = selection();
		// Graphique (d'une feuille ou d'un document) ; tableau croisé
		// d'un document, posé à la cellule active par sa configuration.
		const obj = objFromText(text);
		if (obj?.kind === 'chart') {
			pasteChart(obj.config as unknown as ChartConfig);
			return true;
		}
		if (obj?.kind === 'pivot' && source.placePivot && !(clipRange?.pivot && sameClipText(clipRange.text, text))) {
			const place = source.placePivot;
			writing = writing.catch(() => undefined).then(() =>
				place(sel.r0, sel.c0, obj.config as unknown as PivotConfig).then(
					() => say('Tableau croisé collé (Ctrl+Z pour annuler)'),
					(err) => say(String(err instanceof Error ? err.message : err))
				)
			);
			return true;
		}
		const block = parseTSV(text);
		if (block.length === 0) return false;
		// Tableau croisé copié ou coupé en entier : collé en tableau croisé.
		const pc = clipRange?.pivot && sameClipText(clipRange.text, text) ? clipRange : null;
		if (pc?.pivot && source.movePivot && source.copyPivot) {
			const { id, dr, dc } = pc.pivot;
			const op = pc.cut ? source.movePivot : source.copyPivot;
			writing = writing.catch(() => undefined).then(() =>
				op(id, sel.r0 + dr, sel.c0 + dc).then(
					() => {
						// Coupé : le cadre part une fois le déplacement fait (refusé, on peut recoller ailleurs).
						if (pc.cut && clipRange === pc) clipRange = null;
						message = pc.cut ? 'Tableau croisé déplacé (Ctrl+Z pour annuler)' : 'Tableau croisé collé (Ctrl+Z pour annuler)';
						refreshStatus();
						schedule();
					},
					(err) => {
						message = String(err instanceof Error ? err.message : err);
						refreshStatus();
						schedule();
					}
				)
			);
			return true;
		}
		const rich = source.transfer ? richClip(clipRange, text) : null;
		const h = rich ? rich.r1 - rich.r0 + 1 : block.length;
		const w = rich ? rich.c1 - rich.c0 + 1 : Math.max(...block.map((r) => r.length));
		// Une zone coupée se colle une fois, à partir du coin de la sélection.
		const area = rich?.cut ? { r0: sel.r0, c0: sel.c0, r1: sel.r0 + h - 1, c1: sel.c0 + w - 1 } : pasteArea(h, w, sel);
		if (pivotIn(area.r0, area.c0, area.r1, area.c1)) {
			say(PIVOT_READONLY);
			return true;
		}
		if (queryIn(area.r0, area.c0, area.r1, area.c1)) {
			say(QUERY_READONLY);
			return true;
		}
		const done = (m: string | void, fallback: string) => {
			message = m || fallback;
			active = { row: area.r0, col: area.c0 };
			anchor = { row: area.r1, col: area.c1 };
			refreshFx();
			refreshStatus();
			schedule();
		};
		const fail = (err: unknown) => {
			message = String(err instanceof Error ? err.message : err);
			refreshStatus();
			schedule();
		};
		if (rich && source.transfer) {
			const transfer = source.transfer;
			const src = { r0: rich.r0, c0: rich.c0, r1: rich.r1, c1: rich.c1 };
			if (rich.cut) clipRange = null;
			writing = writing.catch(() => undefined).then(() =>
				transfer(rich.sheet, src, area, rich.cut, rich.values).then(
					(m) => done(m, rich.cut ? 'Cellules déplacées (Ctrl+Z pour annuler)' : 'Collé (Ctrl+Z pour annuler)'),
					fail
				)
			);
			return true;
		}
		// Texte venu d'ailleurs (ou d'un tableau croisé) : des valeurs.
		const tiled = tileBlock(block, sel);
		const cut = clipRange?.cut && richClip({ ...clipRange, plain: false }, text) ? clipRange : null;
		const paste = source.paste;
		writing = writing.catch(() => undefined).then(() => paste(area.r0, area.c0, tiled).then((m) => done(m, `${tiled.length} ligne(s) collée(s)`), fail));
		if (cut) {
			clipRange = null;
			// Une zone coupée de la feuille s'efface ensuite (hors cible).
			// clearSelection se met elle-même à la suite des écritures (après le
			// collage) : l'appeler depuis la file la ferait s'attendre elle-même
			// et bloquerait toute saisie ensuite.
			if (cut.sheet === sheetId() && !overlaps(cut, area)) void clearSelection(cut);
		}
		return true;
	}

	// --- Type de données (section du menu ▾ de la colonne) ----------------------

	const TYPES: ColumnType[] = ['text', 'integer', 'decimal', 'date', 'boolean'];

	// Une valeur hors type fait passer une colonne inférée en texte : l'infobulle
	// de l'en-tête et le menu ▾ disent combien de valeurs sortent du type
	// majoritaire, et le menu propose d'y revenir (ces valeurs marquées en erreur).
	const mixes = new Map<string, Promise<TypeMix | null>>();
	function mixOf(e: Entry, i: number): Promise<TypeMix | null> {
		const k = e.t.columns[i];
		if (!source.typeMix || k.type !== 'text' || k.fixed || k.formula) return Promise.resolve(null);
		const id = `${e.t.id}:${i}`;
		let p = mixes.get(id);
		if (!p) {
			p = source.typeMix(e.t, i).then(
				(m) => (m.type ? m : null),
				() => null
			);
			mixes.set(id, p);
		}
		return p;
	}

	function mixText(m: TypeMix): string {
		const n = m.off > 1;
		const what =
			m.type === 'date'
				? n ? 'qui ne sont pas des dates' : "qui n'est pas une date"
				: m.type === 'boolean'
					? n ? 'non booléennes' : 'non booléenne'
					: n ? 'non numériques' : 'non numérique';
		return `${m.off} valeur${n ? 's' : ''} ${what} sur ${m.total}`;
	}

	/** Infobulle d'un en-tête de colonne : nom complet et type. */
	let tipCell = '';
	function updateHeaderTip(ev: MouseEvent) {
		if (followBadgeAt(ev)) {
			const name = data?.choice?.name ?? '';
			if (tipCell !== 'follow') scroller.title = `Ce tableau suit l'onglet à choix (${name} : ${data?.choice?.value ?? ''}) : il ne montre que ses lignes ; cliquer pour ne plus le suivre`;
			tipCell = 'follow';
			return;
		}
		if (keyHintAt(ev)) {
			if (tipCell !== 'keyhint') scroller.title = KEY_HINT_TIP;
			tipCell = 'keyhint';
			return;
		}
		const cell = cellAt(ev);
		const e = cell && tableAt(cell.row, cell.col);
		if (!cell || !e || cell.row !== e.t.row) {
			if (tipCell) scroller.title = tipCell = '';
			return;
		}
		const i = cell.col - e.t.col;
		const id = `${e.t.id}:${i}`;
		if (id === tipCell) return;
		tipCell = id;
		const k = e.t.columns[i];
		const auto = e.t.autoNumber === i ? ` · ${e.t.key === i ? 'clé, ' : ''}${AUTO_NUMBER_TIP}` : e.t.key === i ? ' · clé du tableau' : '';
		scroller.title = `${k.name} — ${typeLabel(k.type)}${auto}`;
		mixOf(e, i).then((m) => {
			if (m && tipCell === id) scroller.title = `${k.name} — ${typeLabel(k.type)}${auto} · ${mixText(m)}`;
		});
	}

	/** Un clic sur les boutons d'un en-tête (▾ ou icône de type) : le menu de la colonne. */
	function openTypeMenu(ev: MouseEvent, cell: { row: number; col: number }): boolean {
		const e = tableAt(cell.row, cell.col);
		if (!e || cell.row !== e.t.row) return false;
		return headerButton(ev, e, cell.col, false);
	}

	/**
	 * Boutons d'en-tête de la colonne c du tableau e (frozen : en-tête figé
	 * dans le bandeau du haut) : le ▾ et l'icône de type (indicative) ouvrent
	 * le même menu de la colonne, comme le ▾ d'Excel ; un second clic le ferme.
	 */
	function headerButton(ev: MouseEvent, e: Entry, c: number, frozen: boolean): boolean {
		const i = c - e.t.col;
		const right = screenX(c) + cax().size(c);
		if (right - toLocal(ev).x > 34) return false;
		if (!source.sortTable && !(source.filterValues && source.setFilter) && !source.setType) return false;
		if (!frozen) moveTo(e.t.row, c);
		if (filterClosed.key === `${e.t.id}:${i}` && performance.now() - filterClosed.at < 400) {
			filterClosed = { key: '', at: 0 };
			return true;
		}
		openFilter(e, i);
		return true;
	}

	/** Colonne d'un en-tête figé (bandeau du haut) sous la souris. */
	function frozenHeaderAt(ev: MouseEvent): { e: Entry; col: number } | null {
		if (a1) return null;
		const { x, y } = toLocal(ev);
		if (y >= TOP_H || x < MARGIN) return null;
		const col = colAtX(x);
		// Première ligne de la partie qui défile (sous les volets figés).
		const r0 = rowAtY(TOP_H + rowTop(fRows()));
		const e = entries.find((e) => e.t.row < r0 && e.t.row >= fRows() && e.last >= r0 && col >= e.t.col && col <= e.lastCol);
		return e ? { e, col } : null;
	}

	/** Type de données choisi dans le menu de la colonne (confirmé : malgré les valeurs qui ne s'y convertissent pas). */
	async function chooseType(to: ColumnType, confirmed = false) {
		const m = filterDlg;
		if (!m || !source.setType) return;
		if (!confirmed && source.convertPreview && to !== m.e.t.columns[m.i].type) {
			const n = await source.convertPreview(m.e.t, m.i, to);
			if (n > 0) {
				m.pending = { to, n };
				return;
			}
		}
		closeFilter();
		const setType = source.setType;
		await menuAction(() => setType(m.e.t, m.i, to));
	}

	/** Clé du tableau (pied du menu de la colonne) : la poser sur la colonne ou la retirer. */
	function chooseKey(set: boolean) {
		const m = filterDlg;
		if (!m) return;
		closeFilter();
		menuAction(() => (set ? source.setKey!(m.e.t, m.i) : source.clearKey!(m.e.t)));
	}

	// --- Menu contextuel ----------------------------------------------------

	// rowOnly : clic droit sur un numéro de ligne (pas d'actions de colonne).
	// confirmDelete : suppression d'un grand tableau en attente de confirmation.
	let menu = $state<{ x: number; y: number; e: Entry; n: number; i: number; header: boolean; rowOnly?: boolean; confirmDelete?: boolean } | null>(null);

	/** Colonnes que « Choisir par… » propose (null : pas encore connues), relues à l'ouverture d'un menu. */
	let choiceCands = $state.raw<ChoiceCandidate[] | null>(null);
	const wantsCands = $derived(!!source.setTabChoice && (!!menu || !!filterDlg || !!pivotDlg || panel?.kind === 'pivot'));
	$effect(() => {
		if (!wantsCands) return;
		untrack(() => {
			choiceCands = null;
			source.choiceCandidates?.().then(
				(c) => (choiceCands = c ?? []),
				() => undefined
			);
		});
	});

	/** Au-delà de ce nombre de lignes, « Supprimer le tableau » demande confirmation. */
	const CONFIRM_DELETE_ROWS = 500;

	function deleteTable(t: SheetTable) {
		// Pas menuAction : il effacerait le message une fois l'action finie.
		const del = source.deleteTables!;
		menu = null;
		wrap.focus();
		writing = writing.catch(() => undefined).then(() =>
			del([t]).then(
				() => say(`Tableau « ${t.name} » supprimé (Ctrl+Z pour annuler)`),
				(err) => say(String(err?.message ?? err))
			)
		);
	}

	const pointAt = (clientX: number, clientY: number) => ({ clientX, clientY, preventDefault() {}, stopPropagation() {} }) as unknown as MouseEvent;

	/** Menu contextuel ouvert au clavier : posé sur la cellule active, ou sur le graphique sélectionné. */
	function keyContextMenu() {
		const el = selChart !== null ? wrap.querySelector<HTMLElement>('.chart.selected') : null;
		const ch = el ? charts.find((c) => c.id === selChart) : undefined;
		if (el && ch) {
			const r = el.getBoundingClientRect();
			onChartMenu(pointAt(r.left + Math.min(r.width / 2, 40), r.top + Math.min(r.height / 2, 20)), ch);
			return;
		}
		ensureVisible(active.row, active.col);
		const k = zoomK();
		const r = scroller.getBoundingClientRect();
		const x = screenX(active.col) + Math.min(cax().size(active.col), 60) / 2;
		const y = screenY(active.row) + rh(active.row) / 2;
		onContextMenu(pointAt(r.left + x * k, r.top + y * k));
	}

	function onContextMenu(ev: MouseEvent) {
		if (locked && !hold) {
			ev.preventDefault(); // feuille protégée : aucune cellule à cibler
			return;
		}
		if (!formulaMode && openHintMenu(ev)) return;
		const cell = cellAt(ev);
		ev.preventDefault();
		closeMenus();
		if (!cell) {
			// Clic droit sur un numéro de ligne : menu de la ligne du tableau ;
			// hors tableau (ou sur une lettre de colonne) : presse-papiers.
			const hd = headerAt(ev);
			const r = wrap.getBoundingClientRect();
			const te = hd?.kind === 'rows' ? entries.find((x) => hd.row > x.t.row && hd.row <= x.last) : undefined;
			freeMenu = null;
			if (te && hd?.kind === 'rows') menu = { x: ev.clientX - r.left, y: ev.clientY - r.top, e: te, n: hd.row - te.t.row - 1, i: 0, header: false, rowOnly: true };
			else if (hd) freeMenu = { x: ev.clientX - r.left, y: ev.clientY - r.top, kind: hd.kind };
			return;
		}
		// La bande des groupes donne le menu de l'en-tête.
		const e = tableAt(cell.row, cell.col) ?? groupBandAt(cell.row, cell.col);
		if (!inSelection(cell.row, cell.col)) moveTo(cell.row, cell.col);
		const r = wrap.getBoundingClientRect();
		if (!e) {
			freeMenu = { x: ev.clientX - r.left, y: ev.clientY - r.top };
			return;
		}
		menu = { x: ev.clientX - r.left, y: ev.clientY - r.top, e, n: cell.row - e.t.row - 1, i: cell.col - e.t.col, header: cell.row <= e.t.row };
	}

	/** La cellule est dans une zone « plage » (tableau converti). */
	// Cellule libre gardée hors du tableau voisin (note, total : style « out »).
	const keptOut = (row: number, col: number) => !!(freeStyles.get(key(row, col)) as { out?: boolean } | undefined)?.out;
	const inRangeZone = (row: number, col: number) => (data.ranges ?? []).some((z) => row >= z.r0 && row <= z.r1 && col >= z.c0 && col <= z.c1);

	// Menu du clic droit hors tableau : presse-papiers et effacement.
	let freeMenu = $state<{ x: number; y: number; kind?: 'rows' | 'cols' } | null>(null);

	/**
	 * Écriture d'un menu ou d'un volet, à la suite des précédentes ; rend
	 * faux si elle est refusée. keepFocus : réglage d'un volet (graphique,
	 * TCD), le focus y reste au lieu de revenir à la grille.
	 */
	async function menuAction(fn: (() => Promise<unknown>) | undefined, keepFocus = false): Promise<boolean> {
		menu = null;
		freeMenu = null;
		if (!keepFocus) wrap.focus();
		if (!fn) return false;
		let ok = true;
		writing = writing.catch(() => undefined).then(async () => {
			try {
				await fn();
				message = '';
			} catch (err) {
				ok = false;
				linesHint = null; // refusée : rien ne glisse
				message = String(err instanceof Error ? err.message : err);
			}
			refreshStatus();
		});
		await writing;
		return ok;
	}

	// --- Mesures ------------------------------------------------------------

	/** Défilement continu, sauts aléatoires et saisies, sur le premier tableau. */
	export async function runBench() {
		const e = entries[0];
		if (!e) return;
		bench = 'Mesure en cours…';
		const frames = (n: number) =>
			new Promise<void>((res) => {
				let k = 0;
				const step = () => (++k >= n ? res() : requestAnimationFrame(step));
				requestAnimationFrame(step);
			});
		const tableTop = rowTop(e.t.row + 1);
		const tableSpan = Math.max(0, rspan(e.t.row + 1, e.t.row + e.t.rows) - vs.view);

		vs.to(tableTop + Math.random() * tableSpan * 0.5);
		syncNative();
		await frames(30);
		const intervals: number[] = [];
		missingFrames = 0;
		let drawn = 0;
		let prev = performance.now();
		const end = prev + 5000;
		await new Promise<void>((res) => {
			const step = () => {
				const now = performance.now();
				intervals.push(now - prev);
				prev = now;
				drawn++;
				vs.by(10 * ROW_H);
				scroller.scrollTop = vs.toNative();
				draw();
				if (now < end) requestAnimationFrame(step);
				else res();
			};
			requestAnimationFrame(step);
		});
		const scrollMissing = missingFrames;
		const janky = intervals.filter((d) => d > 1000 / 30).length;

		const jumps: number[] = [];
		for (let i = 0; i < 20; i++) {
			vs.to(tableTop + Math.random() * tableSpan);
			syncNative();
			const t0 = performance.now();
			const { first, last } = visibleRows();
			const d = visibleData(e, first, last)!;
			await new Promise<void>((res) => {
				const check = () => (e.cache.has(d.first, d.last) ? res() : requestAnimationFrame(check));
				check();
			});
			jumps.push(performance.now() - t0);
			await frames(2);
		}

		const edits: number[] = [];
		const { first: f0, last: f1 } = visibleRows();
		const d0 = visibleData(e, f0, f1)!;
		for (let i = 0; i < 20; i++) {
			const n = d0.first + i;
			const row = e.cache.row(n);
			if (!row) continue;
			const t0 = performance.now();
			await source.setCell(e.t.row + 1 + n, e.t.col + 1, editValue(e.t.columns[1].type, row.cells[1]), {
				table: e.t,
				column: 1,
				index: n,
				rowId: row.id
			});
			edits.push(performance.now() - t0);
		}

		const result = {
			date: new Date().toISOString(),
			userAgent: navigator.userAgent,
			rows: e.t.rows,
			scroll: {
				frames: drawn,
				fps: Math.round(drawn / 5),
				frameP50: +percentile(intervals, 0.5).toFixed(1),
				frameP95: +percentile(intervals, 0.95).toFixed(1),
				framesOver33ms: janky,
				framesWithPlaceholders: scrollMissing
			},
			jumpToDataP50: +percentile(jumps, 0.5).toFixed(1),
			jumpToDataP95: +percentile(jumps, 0.95).toFixed(1),
			editP50: +percentile(edits, 0.5).toFixed(2),
			editP95: +percentile(edits, 0.95).toFixed(2),
			drawP50: +percentile(drawTimes, 0.5).toFixed(2),
			drawP95: +percentile(drawTimes, 0.95).toFixed(2),
			blockRoundTripP50: +percentile(e.cache.stats.roundTrip, 0.5).toFixed(2),
			blockRoundTripP95: +percentile(e.cache.stats.roundTrip, 0.95).toFixed(2),
			blockGoP50: +percentile(e.cache.stats.server, 0.5).toFixed(2)
		};
		console.log('lattis-bench', JSON.stringify(result));
		(window as unknown as { lattisBench: unknown }).lattisBench = result;
		bench =
			`Défilement : ${result.scroll.fps} img/s, image p95 ${result.scroll.frameP95} ms, ` +
			`${janky} images > 33 ms, ${scrollMissing}/${drawn} images incomplètes · ` +
			`Saut : p50 ${result.jumpToDataP50} ms · Saisie : p50 ${result.editP50} ms`;
		refreshStatus();
		return result;
	}

	onMount(() => {
		ctx = canvas.getContext('2d', { alpha: false })!;
		mounted = true;
		const resize = (resizeCanvas = () => {
			// Zoom : la zone fait clientWidth px de la feuille, le canevas a la résolution de l'écran.
			const dpr = (window.devicePixelRatio || 1) * zoom;
			const cw = scroller.clientWidth;
			const ch = scroller.clientHeight;
			const pw = Math.round(cw * dpr);
			const ph = Math.round(ch * dpr);
			if (cw === viewW && ch === viewH && canvas.width === pw && canvas.height === ph) return;
			viewW = cw;
			viewH = ch;
			// Changer la taille du canevas l'efface (en noir, sans transparence) :
			// on redessine aussitôt, sans attendre l'image suivante.
			canvas.width = pw;
			canvas.height = ph;
			canvas.style.width = `${viewW}px`;
			canvas.style.height = `${viewH}px`;
			vs.view = viewH - TOP_H;
			updateExtent();
			syncNative();
			if (frame) cancelAnimationFrame(frame);
			draw();
		});
		const ro = new ResizeObserver(resize);
		ro.observe(wrap);
		resize();
		scroller.addEventListener('wheel', onWheel, { passive: false });
		const up = (ev?: MouseEvent) => {
			stopAutoScroll();
			if (refDrag) nextArg();
			refDrag = null;
			if (refMove) dropRefMark();
			if (dragging && painter) applyPainter();
			const dragged = dragging || headerDrag;
			dragging = false;
			headerDrag = null;
			if (dragged) refreshFx(); // fin du glissé : l'adresse revient dans la zone de nom
			if (filling) {
				const f = filling;
				filling = null;
				if (f.to) doFill(f.sel, f.to);
				schedule();
			}
			if (moving) {
				if (ev) moving.copy = ev.ctrlKey || ev.metaKey;
				dropMove();
			}
			if (rowResizing || resizing) noAnimUntil = performance.now() + 400;
			if (rowResizing) {
				const { row, height, startH } = rowResizing;
				rowResizing = null;
				if (height !== startH && source.setRowHeight) {
					userHeights.set(row, height);
					const setHeight = source.setRowHeight;
					writing = writing.catch(() => undefined).then(() => setHeight(row, height).then(() => undefined, (err) => say(String(err?.message ?? err))));
				}
				return;
			}
			if (!resizing) return;
			const { col, width, startW } = resizing;
			resizing = null;
			if (width === startW || !source.setColumnWidth) return;
			userWidths.set(col, width);
			const setWidth = source.setColumnWidth;
			writing = writing.catch(() => undefined).then(() => setWidth(col, width).then(() => undefined, (err) => say(String(err?.message ?? err))));
		};
		window.addEventListener('mouseup', up);
		window.addEventListener('mousemove', onDragMove);
		const timer = setInterval(refreshStatus, 250);
		window.addEventListener('copy', onCopy);
		window.addEventListener('keydown', onGlobalKey);
		window.addEventListener('cut', onCut);
		window.addEventListener('paste', onPaste);
		wrap.focus();
		refreshStatus();
		if (autoBench) setTimeout(async () => autoBench(JSON.stringify(await runBench())), 1500);
		return () => {
			ro.disconnect();
			scroller.removeEventListener('wheel', onWheel);
			window.removeEventListener('mouseup', up);
			window.removeEventListener('mousemove', onDragMove);
			stopAutoScroll();
			clearInterval(timer);
			window.removeEventListener('copy', onCopy);
			window.removeEventListener('keydown', onGlobalKey);
			window.removeEventListener('cut', onCut);
			window.removeEventListener('paste', onPaste);
			cancelAnimationFrame(frame);
		};
	});

	/** Nom du tableau de la cellule active (pour l'export), s'il y en a un. */
	/** Cellule active (lignes et colonnes de la feuille, à partir de 0). */
	export function activeCell(): { row: number; col: number } {
		return { row: active.row, col: active.col };
	}

	export function activeTable(): string | undefined {
		return targetTable()?.t.name;
	}

	/**
	 * Ce que l'utilisateur regarde, pour l'assistant (bridge.AIContext) : le
	 * tableau de la cellule active et la sélection lisible, comme la zone
	 * d'adresse : « Commandes[Montant][3:10] » (lignes du tableau, à partir
	 * de 1), « Commandes[Montant:Prix][3] », « Commandes[Montant] » (en-tête),
	 * ou « B2:D5 » hors tableau.
	 */
	export function aiContext(): { table: string; selection: string } {
		const sel = selection();
		const e = ownerAt(active.row, active.col);
		const a1 = (r0: number, c0: number, r1: number, c1: number) => (r0 === r1 && c0 === c1 ? addrA1(r0, c0) : `${addrA1(r0, c0)}:${addrA1(r1, c1)}`);
		if (!e) return { table: '', selection: a1(sel.r0, sel.c0, sel.r1, sel.c1) };
		// Sélection débordant du tableau : la cellule active seulement.
		const inside = inTable(e, sel.r0, sel.c0) && inTable(e, sel.r1, sel.c1);
		const [r0, c0, r1, c1] = inside ? [sel.r0, sel.c0, sel.r1, sel.c1] : [active.row, active.col, active.row, active.col];
		const name = (c: number) => e.t.columns[c - e.t.col]?.name ?? '';
		const cols = c0 === c1 ? name(c0) : `${name(c0)}:${name(c1)}`;
		// Lignes de données (l'en-tête n'en est pas une).
		const d0 = Math.max(r0, e.t.row + 1) - e.t.row;
		const d1 = r1 - e.t.row;
		const rows = r1 <= e.t.row ? '' : d0 === d1 ? `[${d0}]` : `[${d0}:${d1}]`;
		return { table: e.t.name, selection: `${e.t.name}[${cols}]${rows}` };
	}

	/** Tableau de la cellule active, s'il y en a un (F5). */
	export function tableHere(): SheetTable | undefined {
		return ownerAt(active.row, active.col)?.t;
	}

	/** Redonne le focus clavier à la feuille (après un dialogue, un menu…). */
	export function focus() {
		// Formule pointée sur une autre feuille : la barre de formule garde la main.
		if (hold && fxEditing) {
			fxInput?.focus();
			return;
		}
		wrap?.focus();
	}
</script>

{#snippet linesItems(kind: 'rows' | 'cols' | undefined, x: number, y: number)}
	<!-- Insertion, suppression et effacement ; sur les en-têtes de la feuille, puis
	     la mise en forme et la taille ou l'affichage des lignes ou colonnes. -->
	{#if source.sheetLines}
		{@const { r0, r1, c0, c1 } = selection()}
		{#if kind === 'rows'}
			<button onclick={() => sheetLines('insertRows')}>Insérer {r1 > r0 ? `${r1 - r0 + 1} lignes` : 'une ligne'}</button>
			<button onclick={() => sheetLines('deleteRows')}>Supprimer {r1 > r0 ? 'les lignes' : 'la ligne'}</button>
		{:else if kind === 'cols'}
			<button onclick={() => sheetLines('insertCols')}>Insérer {c1 > c0 ? `${c1 - c0 + 1} colonnes` : 'une colonne'}</button>
			<button onclick={() => sheetLines('deleteCols')}>Supprimer {c1 > c0 ? 'les colonnes' : 'la colonne'}</button>
		{:else}
			<button onclick={() => openLinesDialog(true, x, y)}>Insérer…</button>
			<button title="Vide les cellules : valeurs, mise en forme, fusions, notes et validations" onclick={() => { menu = freeMenu = null; clearSelection(selection(), true); }}>Supprimer</button>
		{/if}
	{/if}
	<button
		onclick={() => {
			menu = freeMenu = null;
			if (!deleteWholeColumns()) clearSelection();
		}}>Effacer le contenu</button
	>
	{#if source.sheetLines && kind}
		{@const rows = kind === 'rows'}
		{#if source.applyStyle || source.setSheetRules}
			<hr />
			{#if source.applyStyle}<button data-shed="1" onclick={() => openFormat()}>Format de cellule…</button>{/if}
			{#if source.setSheetRules}<button data-shed="1" onclick={rulesActive}>Mise en forme conditionnelle…</button>{/if}
		{/if}
		{#if (rows ? source.setRowHeight : source.setColumnWidth) || source.setLinesHidden}
			<hr />
			{#if rows && source.setRowHeight}
				<button data-shed="2" onclick={() => openSize(true, x, y)}>Hauteur de ligne…</button>
				<button data-shed="1" onclick={autoFitRows} title="Hauteur d’après le contenu (double-clic sur le bord du numéro de ligne)">Ajuster la hauteur de ligne</button>
			{:else if !rows && source.setColumnWidth}
				<button data-shed="2" onclick={() => openSize(false, x, y)}>Largeur de colonne…</button>
				<button data-shed="1" onclick={autoFitCols} title="Largeur d’après le contenu affiché (double-clic sur le bord de l’en-tête)">Ajuster la largeur de colonne</button>
			{/if}
			{#if source.setLinesHidden}
				<button onclick={() => setHidden(rows, true)}>Masquer</button>
				{#if hiddenIn(rows)}<button onclick={() => unhide(rows)}>Afficher</button>{/if}
			{/if}
		{/if}
	{/if}
{/snippet}

{#snippet noteItems()}
	{#if noteOf(active.row, active.col)}
		<button data-shed="1" onclick={editNote}>Modifier la note</button>
		<button data-shed="1"
			onclick={() => {
				menu = freeMenu = null;
				deleteNote();
			}}>Supprimer la note</button
		>
	{:else}
		<button data-shed="1" onclick={editNote}>Insérer une note</button>
	{/if}
{/snippet}
<!-- Formule colorée (références, faute soulignée), sur une seule ligne : le texte est affiché tel quel (pre-wrap). -->
{#snippet coloured(text: string)}{#each highlightParts(text, formulaRefs(text), editErr) as p, i (i)}{#if p.err}<span class="hl-err">{p.text}</span>{:else if p.color}<span style:color={p.color}>{p.text}</span>{:else}{p.text}{/if}{/each}{'​'}{/snippet}

{#snippet suggestions(where: 'cell' | 'fx')}
	{#if sig?.where === where}
		{@const sp = signatureParts(sig.fn, sig.index)}
		<div class="sig">
			<div class="sig-line">{#each sp.parts as p, i (i)}{#if p.on}<b>{p.text}</b>{:else}{p.text}{/if}{/each}</div>
			{#if sig.fn.description}<div class="sig-desc">{sig.fn.description}</div>{/if}
			{#if sp.arg}<div class="sig-arg"><b>{sp.arg.name}</b>{sp.arg.optional ? ' (facultatif)' : ''} : {sp.arg.desc}</div>{/if}
			{#if sig.fn.example}<div class="sig-ex">Exemple : <code>{sig.fn.example}</code></div>{/if}
		</div>
	{/if}
	{#if completion?.where === where}
		{#each completion.items as s, i (s.kind + s.label)}
			<!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
			<div
				class="sugg"
				class:on={i === completion.sel}
				onmousedown={(e) => {
					e.preventDefault();
					completion = { ...completion!, sel: i };
					const el = where === 'cell' ? input : fxInput;
					if (el) completionKey(new KeyboardEvent('keydown', { key: 'Enter' }), el, (v) => (where === 'cell' ? (editValueText = v) : (fxText = v)));
				}}
			>
				<span class="kind {s.kind}">{s.kind === 'function' ? 'fx' : s.kind === 'table' ? '▦' : s.kind === 'name' ? 'N' : '[ ]'}</span>
				<span class="lab">{s.label}</span>
				<span class="det" title={s.detail}>{s.detail}</span>
			</div>
		{/each}
	{/if}
	{#if isFormula(where === 'cell' ? editValueText : fxText)}
		{@const ok = formulaReady(where === 'cell' ? editValueText : fxText)}
		<div class="sig-foot">
			<button
				class="validate"
				disabled={!ok}
				title={ok ? 'Valider la formule (Entrée)' : 'La formule a une faute : corrigez-la pour la valider'}
				onmousedown={(e) => {
					// Le focus reste dans l'éditeur : la validation passe par Entrée.
					e.preventDefault();
					(where === 'cell' ? input : fxInput)?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
				}}>Valider</button
			>
		</div>
	{/if}
{/snippet}

<div class="sheet-root">
	<div class="formula-bar" data-region="formula">
		{#if tableBox}
			<div class="name-box split">
				<input
					class="table-name"
					class:default-name={isDefaultTableName(tableBox.name)}
					title={isDefaultTableName(tableBox.name) ? `${DEFAULT_NAME_HINT} (modifiez puis Entrée)` : 'Nom du tableau : modifiez puis Entrée pour le renommer'}
					value={tableBox.name}
					onfocus={(e) => e.currentTarget.select()}
					onkeydown={onTableNameKey}
					spellcheck="false"
				/>
				<span class="slash">/</span>
				<input
					class="col-name"
					title="Colonne[ligne] : modifiez le nom puis Entrée pour renommer la colonne"
					value={tableBox.col}
					onfocus={(e) => e.currentTarget.select()}
					onkeydown={onColumnNameKey}
					spellcheck="false"
				/>
			</div>
		{:else}
			<input
				class="name-box"
				bind:this={nameInput}
				title="Nom de la cellule : tapez un nom puis Entrée pour la nommer, ou le nom d'une cellule pour y aller"
				value={nameBox}
				placeholder="—"
				onfocus={(e) => e.currentTarget.select()}
				onkeydown={onNameKey}
				spellcheck="false"
			/>
		{/if}
		<span class="fx">fx</span>
		<div class="fx-wrap">
		{#if fxHl}
			<!-- Formule : références colorées sous la saisie (transparente), défilement suivi. -->
			<div class="fx-hl" aria-hidden="true" style:transform="translateX({-fxScroll}px)">{@render coloured(fxText)}</div>
		{/if}
		<input
			class="fx-input"
			data-region-focus
			aria-label="Barre de formule"
			class:hl={fxHl}
			bind:this={fxInput}
			bind:value={fxText}
			oninput={(e) => {
				onFormulaInput(e.currentTarget, 'fx');
				fxScroll = e.currentTarget.scrollLeft;
			}}
			onkeyup={(e) => {
				if (!fxLock) moveSig(e, 'fx');
				fxScroll = e.currentTarget.scrollLeft;
			}}
			onclick={(e) => !fxLock && moveSig(e, 'fx')}
			onscroll={(e) => (fxScroll = e.currentTarget.scrollLeft)}
			class:ro={!!fxLock}
			readonly={!!fxLock}
			aria-readonly={!!fxLock}
			title={fxLock || undefined}
			onpaste={(e) => fxLock && e.preventDefault()}
			oncut={(e) => fxLock && e.preventDefault()}
			onfocus={() => {
				// Cellule en lecture seule : le texte se sélectionne et se copie, sans se modifier.
				if (fxLock) say(fxLock);
				else fxEditing = true;
			}}
			onkeydown={onFxKey}
			onblur={leaveFx}
			spellcheck="false"
		/>
		{#if calcEdit}
			<span class="fx-scope" title="La formule est celle de toute la colonne : elle s'applique à chacune de ses lignes."
				>Colonne calculée : s'applique {calcEdit.e.t.rows > 1 ? `aux ${calcEdit.e.t.rows.toLocaleString('fr-FR')} lignes` : 'à la ligne'}</span
			>
		{/if}
		</div>
	</div>
	{#if bench}<div class="bench">{bench}</div>{/if}
	{#if completion?.where === 'fx' || sig?.where === 'fx' || (fxEditing && isFormula(fxText))}
		<div class="suggest fx-suggest">{@render suggestions('fx')}</div>
	{/if}

	<div class="body">
	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	<div class="wrap" bind:this={wrap} tabindex="0" role="grid" aria-label="Feuille {sheetName}" aria-rowcount={-1} aria-colcount={-1} aria-activedescendant={a11yId} aria-keyshortcuts="Shift+F10" data-region-focus onkeydown={onKeyDown}>
		<GridA11y cellId={a11yId} {speech} row={active.row + 1} col={active.col + 1} editing={editing || fxEditing} selection={speechSel} />
		<!-- Calque zoomé (zoom.ts) : grille, graphiques, boutons, éditeur et notes ; menus et dialogues hors du calque. -->
		<div class="zoom" style:zoom={zoom === 1 ? undefined : zoom}>
			<canvas bind:this={canvas}></canvas>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="scroller"
				bind:this={scroller}
				onscroll={onScroll}
				onpointerdown={onScrollbarDown}
				onmousedown={onMouseDown}
				onmousemove={onMouseMove}
				onmouseleave={() => ((noteTip = null), hoverLocked >= 0 && ((hoverLocked = -1), schedule()))}
				ondblclick={onDblClick}
				oncontextmenu={onContextMenu}
			>
				<div bind:this={spacer}></div>
			</div>
			<!-- Calque découpé à la zone des cellules : les graphiques ne passent pas sur les barres de défilement. -->
			<div class="charts" style:left="{MARGIN}px" style:top="{TOP_H}px" style:width="{Math.max(0, viewW - MARGIN)}px" style:height="{Math.max(0, viewH - TOP_H)}px">
				{#each charts as ch (ch.id)}
					{@const box = chartBox(ch)}
					<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
					<div
						class="chart"
						role="group"
						aria-label={ch.config.title ? `Graphique « ${ch.config.title} »` : 'Graphique'}
						style:left="{box.x - viewPos.x}px"
						style:top="{box.y - viewPos.y}px"
						class:selected={selChart === ch.id}
						style:width="{box.w}px"
						style:height="{box.h}px"
						ondblclick={() => editChart(ch)}
						onmousedown={(ev) => startChartGesture(ev, ch)}
						oncontextmenu={(ev) => onChartMenu(ev, ch)}
						onwheel={(ev) => onWheel(ev)}
					>
						{#if ch.error}
							<p class="chart-error">Graphique : {ch.error}</p>
						{:else}
							<Chart kind={ch.config.kind} labels={ch.labels} series={ch.series} width={box.w} height={box.h} style={ch.config} animate={animate && chartTween} grow={grownCharts.has(ch.id)} interactive={selChart === ch.id} onview={(v) => setMapView(ch.id, v)} map={ch} info={ch} onpoint={onrowlink && ((r) => pointToRow(ch, r))} />
						{/if}
						{#if selChart === ch.id}
							{#each ['n', 's', 'e', 'w', 'ne', 'nw', 'se', 'sw'] as dir (dir)}
								<!-- svelte-ignore a11y_no_static_element_interactions -->
								<div class="handle {dir}" onmousedown={(ev) => startChartGesture(ev, ch, dir)}></div>
							{/each}
						{/if}
					</div>
				{/each}
				<SheetButtons
					bind:this={buttonsLayer}
					locked={!!locked}
					{buttons}
					geo={{ colOffset: (c) => cax().offset(c), rowTop, colAt: (x) => cax().indexAt(x), rowAt: (y) => rax().indexAt(y) }}
					{zoom}
					view={viewPos}
					active={() => active}
					scripts={() => source.scripts?.() ?? Promise.resolve([])}
					onrun={runButton}
					onfire={fireControl}
					pickFile={source.pickFile}
					oncreate={source.createButton}
					onset={source.setButton}
					onplace={source.placeButton}
					ondelete={source.deleteButton}
					onwheel={(ev) => onWheel(ev)}
					{say}
					done={() => wrap.focus()}
					cellValue={(r, c) => shownValue(r, c)}
					setCell={(r, c, raw) => write(r, c, raw)}
					tick={controlsTick}
					columnRefs={sources.flatMap((t) => t.columns.map((c) => `${t.name}[${c}]`))}
					dock={() => {
						const r = wrap.getBoundingClientRect();
						return { top: Math.round(r.top), bottom: Math.round(window.innerHeight - r.bottom) };
					}}
				/>
			</div>
			{#if editing}
				{@const hl = !editRO && isFormula(editValueText)}
				<div class="editor-box" class:ro={!!editRO}>
					{#if editRO}
						<!-- Lecture seule : le texte se sélectionne et se copie, sans se modifier. -->
						<span class="editor-ro" aria-hidden="true">lecture seule</span>
					{/if}
					{#if hl}
						<!-- Formule : texte coloré sous la zone de saisie (transparente). -->
						<div class="editor-hl" aria-hidden="true">{@render coloured(editValueText)}</div>
					{/if}
					<textarea
						class="editor"
						class:hl
						rows="1"
						spellcheck="false"
						bind:this={input}
						class:ro={!!editRO}
						readonly={!!editRO}
						aria-readonly={!!editRO}
						bind:value={editValueText}
						oninput={(e) => onFormulaInput(e.currentTarget, 'cell')}
						onkeyup={(e) => !editRO && moveSig(e, 'cell')}
						onclick={(e) => !editRO && moveSig(e, 'cell')}
						onpaste={(e) => editRO && e.preventDefault()}
						oncut={(e) => editRO && e.preventDefault()}
						ondrop={(e) => editRO && e.preventDefault()}
						onkeydown={onEditorKey}
						onblur={() => {
							completion = sig = null;
							// Lecture seule : le focus parti ailleurs referme l'éditeur, sans rien écrire.
							if (editRO) {
								closeReadOnly(false);
								return;
							}
							// Boîte « nom invalide » ouverte : la saisie reprend à sa fermeture.
							if (nameAlertOpen()) return;
							// Clic sur un onglet de feuille : la formule est gardée pour pointer sur l'autre feuille.
							if (tabPress && formulaTyping(editValueText)) return;
							// Retour de pointage sur une autre feuille : la saisie continue.
							if (performance.now() < keepEditorUntil && formulaTyping(editValueText)) {
								const el = input;
								const at = [el?.selectionStart ?? 0, el?.selectionEnd ?? 0];
								requestAnimationFrame(() => {
									el?.focus();
									el?.setSelectionRange(at[0], at[1]);
								});
								return;
							}
							// Saisie refusée par la liste déroulante quand le focus part ailleurs : abandonnée, le message reste.
							if (!commit() && !editErr && !nameAlertOpen()) {
								editing = false;
								schedule();
							}
						}}
						title={editRO ||
							(() => {
							const e = tableAt(active.row, active.col);
							return e ? typeLabel(e.t.columns[active.col - e.t.col].type) : 'Cellule libre';
						})()}
					></textarea>
				</div>
			{/if}
			{#each pinnedShown as p (p.x + ',' + p.y)}
				<div class="note-tip pinned" style:left="{toView(p.x)}px" style:top="{toView(p.y)}px">{p.text}</div>
			{/each}
			{#if noteTip && !noteDlg}
				<div class="note-tip" style:left="{noteTip.x}px" style:top="{noteTip.y}px">{noteTip.text}</div>
			{/if}
		</div>
		{#if finder}
			<!-- Entrée, F3, Échap, Ctrl+F et Ctrl+H dans toute la barre (onFinderKey) ;
			     les boutons d'icône gardent le focus dans le champ (mousedown). -->
			<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
			<div class="finder" role="search" aria-label="Rechercher et remplacer" onkeydown={onFinderKey}>
				<div class="fd-row">
					{#if source.replace}
						<button
							class="fd-icon fd-toggle"
							class:open={finder.replace}
							onclick={() => toggleReplace()}
							aria-expanded={finder.replace}
							aria-controls="fd-replace"
							aria-label={finder.replace ? 'Masquer le remplacement' : 'Afficher le remplacement'}
							title={finder.replace ? 'Masquer le remplacement' : 'Remplacer (Ctrl+H)'}
						><Icon name="chevron-down" size={14} /></button>
					{/if}
					<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
					<div class="fd-field" class:miss={finder.count === 'Aucun résultat'} onclick={() => findInput?.focus()}>
						<Icon name="search" size={14} />
						<input
							bind:this={findInput}
							bind:value={finder.text}
							oninput={() => finder && ((finder.count = ''), (finder.info = ''))}
							placeholder="Rechercher"
							aria-label="Rechercher"
							aria-describedby="fd-count"
							spellcheck="false"
							autocomplete="off"
						/>
						<span id="fd-count" class="fd-count" aria-live="polite">{finder.count}</span>
					</div>
					<button class="fd-icon fd-up" onmousedown={(e) => e.preventDefault()} onclick={() => findNext(true)} disabled={!finder.text} aria-label="Précédent" title="Précédent (Maj+Entrée ou Maj+F3)"
						><Icon name="chevron-down" size={16} /></button
					>
					<button class="fd-icon" onmousedown={(e) => e.preventDefault()} onclick={() => findNext()} disabled={!finder.text} aria-label="Suivant" title="Suivant (Entrée ou F3)"
						><Icon name="chevron-down" size={16} /></button
					>
					<button class="fd-icon fd-close" onclick={closeFinder} aria-label="Fermer la recherche" title="Fermer (Échap)"><Icon name="win-close" size={14} /></button>
				</div>
				{#if finder.replace}
					<div class="fd-row fd-indent" id="fd-replace">
						<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
						<div class="fd-field" onclick={() => replaceInput?.focus()}>
							<Icon name="replace" size={14} />
							<input bind:this={replaceInput} bind:value={finder.by} placeholder="Remplacer par" aria-label="Remplacer par" spellcheck="false" autocomplete="off" />
						</div>
					</div>
				{/if}
				<div class="fd-row fd-foot" class:fd-indent={!!source.replace}>
					<span
						class="fd-chip"
						class:on={!!finder.zone}
						title={finder.zone
							? 'La sélection borne la recherche ; sélectionnez une seule cellule pour chercher dans toute la feuille'
							: 'Toute la feuille ; sélectionnez plusieurs cellules pour borner la recherche'}
						aria-label={finder.zone ? `Dans la sélection ${zoneLabel(finder.zone)}` : 'Dans toute la feuille'}
					>
						<Icon name={finder.zone ? 'print-area' : 'sheet'} size={13} />
						<span>Dans : {finder.zone ? zoneLabel(finder.zone) : 'feuille'}</span>
					</span>
					{#if finder.replace}
						<span class="fd-spacer"></span>
						<button class="fd-btn" onclick={replaceOne} disabled={!finder.text} title="Remplace la cellule trouvée, puis passe à la suivante">Remplacer</button>
						<button class="fd-btn primary" onclick={replaceAll} disabled={!finder.text} title="Remplace toutes les occurrences (Ctrl+Z pour annuler)">Tout remplacer</button>
					{/if}
				</div>
				{#if finder.info}
					<div class="fd-info" class:fd-indent={!!source.replace} role="status">{finder.info}</div>
				{/if}
			</div>
		{/if}
		{#if menu}
			{@const m = menu}
			{@const col = m.e.t.columns[m.i]}
			{@const cols = !m.rowOnly}
			{@const rowItems = !m.header && !m.rowOnly}
			{@const filtered = cols && !!source.clearFilters && m.e.t.columns.some((c) => c.filter)}
			{@const sortCount = (cols && source.sortTable ? 2 : 0) + (cols && source.sortTableCustom ? 1 : 0) + (cols && source.setFilter ? 1 : 0) + (filtered ? 1 : 0)}
			{@const toTitle = cols && !!source.headerToTitle && m.e.t.rows > 0}
			{@const toNote = rowItems && !!source.rowToNote && m.n === m.e.t.rows - 1}
			{@const refresh = !!m.e.t.query && !!source.refreshTable}
			{@const share = cols && !!onsharedata && !m.e.t.query}
			{@const addShared = cols && !!onaddshared && m.e.t.shared === 'private' && !m.e.t.query}
			{@const pushSource = cols && !!onpushsource && sourceTables.includes(m.e.t.name)}
			{@const twoLevel = cols && !!source.setTwoLevel && !m.e.t.query}
			{@const follow = cols && !!source.setFollow && !!data?.choice && !m.e.t.query}
			{@const tableItems = toTitle || twoLevel || refresh || share || pushSource || follow || !!source.convertToRange || !!source.deleteTables}
			{#snippet sortItems()}
				{#if cols && source.sortTable}
					{@const lab = sortLabels(col?.type ?? 'text', col?.result)}
					<button onclick={() => menuAction(() => source.sortTable!(m.e.t, m.i, false))}>{lab[0]}</button>
					<button onclick={() => menuAction(() => source.sortTable!(m.e.t, m.i, true))}>{lab[1]}</button>
				{/if}
				{#if cols && source.sortTableCustom}
					<button onclick={() => openCustomSort(m.e, m.i)}>Tri personnalisé…</button>
				{/if}
				{#if cols && source.setFilter}
					<button onclick={() => openFilter(m.e, m.i)}>Filtrer la colonne…</button>
				{/if}
				{#if filtered}
					<button onclick={() => menuAction(() => source.clearFilters!(m.e.t))}>Effacer le filtre du tableau</button>
				{/if}
			{/snippet}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (menu = null)}>
				<!-- Les noms (tableau, colonne) n'apparaissent qu'en tête de section, abrégés ; jamais dans les libellés. -->
				<div class="menu-title" class:default-name={isDefaultTableName(m.e.t.name)} title={isDefaultTableName(m.e.t.name) ? DEFAULT_NAME_HINT : clipped(m.e.t.name) ? m.e.t.name : undefined}>Tableau « {clip(m.e.t.name)} »</div>
				{#if m.confirmDelete}
					<p class="menu-note">
						Supprimer ce tableau et ses {(m.e.t.rows + (m.e.t.hidden ?? 0)).toLocaleString('fr-FR')} lignes ? Ctrl+Z permettra de l’annuler.
					</p>
					<button onclick={() => deleteTable(m.e.t)}>Supprimer le tableau</button>
					<button onclick={() => (menu = { ...m, confirmDelete: false })}>Annuler</button>
				{:else}
					<!-- Groupes du menu contextuel d'Excel : presse-papiers ; insertion et
					     suppression ; tri et filtre ; mise en forme ; puis ce qui est propre à
					     Lattis : la colonne (type, clé, nom), le tableau (sous-menu), l'assistant. -->
					<button data-shed="3"
						onclick={() => {
							menu = null;
							copySelection(true);
						}}>Couper</button
					>
					<button data-shed="3"
						onclick={() => {
							menu = null;
							copySelection();
						}}>Copier</button
					>
					<button data-shed="3"
						onclick={() => {
							menu = null;
							pasteClipboard();
						}}>Coller</button
					>
					<hr />
					{#if m.rowOnly}
						{@render linesItems('rows', m.x, m.y)}
					{:else}
						<Submenu label="Insérer">
							{#if rowItems}
								<button onclick={() => menuAction(source.insertRow && (() => tableLines(m.e, 'insertRow', m.n)))}>Ligne au-dessus</button>
								<button onclick={() => menuAction(source.insertRow && (() => tableLines(m.e, 'insertRow', m.n + 1)))}>Ligne en dessous</button>
							{/if}
							<button onclick={() => menuAction(source.insertColumn && (() => tableLines(m.e, 'insertColumn', m.i)))}>Colonne à gauche</button>
							<button onclick={() => menuAction(source.insertColumn && (() => tableLines(m.e, 'insertColumn', m.i + 1)))}>Colonne à droite</button>
						</Submenu>
						{#if rowItems}
							<Submenu label="Supprimer">
								<button onclick={() => menuAction(source.deleteRow && (() => tableLines(m.e, 'deleteRow', m.n)))}>Ligne du tableau</button>
								<button onclick={() => menuAction(source.deleteColumn && (() => tableLines(m.e, 'deleteColumn', m.i)))}>Colonne du tableau</button>
							</Submenu>
						{:else}
							<button onclick={() => menuAction(source.deleteColumn && (() => tableLines(m.e, 'deleteColumn', m.i)))}>Supprimer la colonne</button>
						{/if}
						<button
							onclick={() => {
								menu = null;
								clearSelection();
							}}>Effacer le contenu</button
						>
						{#if toNote}
							<!-- Seulement sur la dernière ligne de données (le moteur refuse les autres). -->
							<button title="La ligne reste juste sous le tableau, gardée à part comme total ou note (Ctrl+Z pour annuler)" onclick={() => menuAction(() => source.rowToNote!(m.e.t, m.n))}>Sortir cette ligne du tableau (total ou note)</button>
						{/if}
					{/if}
					{#if sortCount}
						<hr />
						{#if sortCount >= 3}
							<Submenu label="Trier et filtrer">{@render sortItems()}</Submenu>
						{:else}
							{@render sortItems()}
						{/if}
					{/if}
					{#if !m.rowOnly && (source.applyStyle || source.setSheetRules || source.setNote)}
						<hr />
						{#if source.applyStyle}
							<button data-shed="1" onclick={() => openFormat()}>Format de cellule… <span class="keys">Ctrl+1</span></button>
						{/if}
						{#if source.setSheetRules}
							<button data-shed="1" onclick={() => openRules(m.e, m.i)}>Mise en forme conditionnelle…</button>
						{/if}
						{#if source.setNote}{@render noteItems()}{/if}
					{/if}
					{#if cols}
						<hr />
						<div class="menu-title" data-shed="1" title={clipped(col?.name) ? col?.name : undefined}>Colonne « {clip(col?.name)} »</div>
						{#if source.setType && !col?.formula}
							<!-- Le menu ▾ de la colonne (tri, filtre, type, clé), sous son en-tête. -->
							<button data-shed="1" onclick={() => openFilter(m.e, m.i)}>Type de données et clé…</button>
						{/if}
						<button data-shed="1"
							onclick={() => {
								const t = m.e.t;
								const c = t.col + m.i;
								menu = null;
								moveTo(t.row, c);
								startEdit(false);
							}}>Renommer la colonne</button
						>
						{#if source.applyStyle && !col?.formula}
							<!-- Validation des données de toute la colonne (ses lignes de données). -->
							<button data-shed="1"
								onclick={() => {
									const e = m.e;
									const c = e.t.col + m.i;
									menu = null;
									selectRange(e.t.row + 1, c, Math.max(e.last, e.t.row + 1), c);
									openValidation();
								}}>Validation des données…</button
							>
						{/if}
						{#if source.setTabChoice && col}
							{@const tc = tabChoiceFor(m.e.t.name, col.name, { formula: !!col.formula })}
							<button data-shed="1" title={tc.title} disabled={tc.disabled} onclick={() => runTabChoice(m.e.t.name, col.name, tc)}>{tc.label}</button>
						{/if}
					{/if}
					{#if tableItems}
						<hr />
						<Submenu label="Tableau">
							{#if toTitle}
								<button title="La 1re ligne de données devient l'en-tête ; l'en-tête actuel reste au-dessus du tableau, en titre (sauf les noms « Colonne n »)" onclick={() => menuAction(() => source.headerToTitle!(m.e.t))}>Sortir l'en-tête</button>
							{/if}
							{#if twoLevel}
								{@const on = !!m.e.t.twoLevel}
									<button
										title={on
											? 'Les colonnes reprennent leur nom complet « Groupe.Nom » sur une seule ligne ; la bande des groupes disparaît'
											: 'La ligne juste au-dessus du tableau devient une bande de groupes : « BOISEMENT » posé au-dessus de Nom et Statut donne les colonnes « BOISEMENT.Nom » et « BOISEMENT.Statut ». Cellules fusionnées ou non.'}
										onclick={() => menuAction(() => source.setTwoLevel!(m.e.t, !on))}>{on ? "Retirer les groupes d'en-têtes" : 'En-têtes sur 2 lignes (groupes au-dessus)'}</button
								>
							{/if}
							{#if follow}
								{@const on = !!m.e.t.follows}
								{@const what = followLabel(data?.choice?.name ?? '')}
								<button
									role="menuitemcheckbox"
									aria-checked={on}
									title={on
										? `Le tableau montre toutes ses lignes, quelle que soit la valeur de l'onglet (sa colonne « ${data?.choice?.name} » est retirée si elle n'a qu'une valeur)`
										: `Le tableau reçoit la colonne « ${data?.choice?.name} », remplie avec la valeur affichée, et ne montre plus que ses lignes`}
									onclick={() => toggleFollow(m.e.t, !on)}><span class="check">{on ? '✓' : ''}</span>Suivre {what}</button
								>
							{/if}
							{#if refresh}
								{@const t = m.e.t}
								<button title="Relance la requête « {t.query?.name} » (F5 dans le tableau)" onclick={() => menuAction(() => source.refreshTable!(t))}>Actualiser la requête</button>
									{#if source.setHandKey && t.columns.some((k) => k.hand && !k.formula)}
										<button title="Colonne du résultat qui relie les valeurs saisies à leurs lignes d'une actualisation à l'autre" onclick={() => openHandKey(t)}>Clé des colonnes saisies…</button>
									{/if}
									{#if source.forgetHandKept && t.handKept}
										<button title="Valeurs saisies sur des lignes disparues du résultat, gardées au cas où elles reviendraient (Ctrl+Z pour annuler)" onclick={() => menuAction(() => source.forgetHandKept!(t))}>Oublier les valeurs mises de côté ({t.handKept} {t.handKept > 1 ? 'lignes' : 'ligne'})</button>
									{/if}
							{/if}
							{#if addShared}
								<button title="Ce tableau n'est que sur votre poste : le créer dans le classeur partagé, avec ses lignes, pour tous les membres" onclick={() => { const n = m.e.t.name; menu = null; onaddshared?.(n); }}>Ajouter au classeur partagé…</button>
							{/if}
							{#if share}
								<button title="Rendre ce tableau lisible depuis d'autres classeurs du serveur (catalogue du Data Lake Lattis)" onclick={() => { const n = m.e.t.name; menu = null; onsharedata?.(n); }}>Partager au Data Lake Lattis…</button>
							{/if}
							{#if pushSource}
								<button title="Envoie maintenant les lignes de ce tableau à sa source du Data Lake (colonnes choisies seulement ; le classeur n'est pas publié)" onclick={() => { const n = m.e.t.name; menu = null; onpushsource?.(n); }}>Mettre à jour la source du Data Lake</button>
							{/if}
							{#if source.convertToRange}
								<button title="Le tableau devient des cellules libres, au même endroit" onclick={() => menuAction(() => source.convertToRange!(m.e.t))}>Convertir en plage</button>
							{/if}
							{#if source.deleteTables}
								<hr />
								<button
									onclick={() => {
										if (m.e.t.rows + (m.e.t.hidden ?? 0) > CONFIRM_DELETE_ROWS) menu = { ...m, confirmDelete: true };
										else deleteTable(m.e.t);
									}}>Supprimer le tableau</button
								>
							{/if}
						</Submenu>
					{/if}
					{#if onask}
						<hr />
						<button
							onclick={() => {
								menu = null;
								onask();
							}}>Demander à l’assistant…</button
						>
					{/if}
				{/if}
			</div>
		{/if}
		{#if followMenu}
			{@const m = followMenu}
			{@const what = followLabel(data?.choice?.name ?? '')}
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (followMenu = null)}>
				<div class="menu-title" title={clipped(m.e.t.name) ? m.e.t.name : undefined}>Tableau « {clip(m.e.t.name)} »</div>
				<button role="menuitemcheckbox" aria-checked="true" disabled={!source.setFollow} onclick={() => toggleFollow(m.e.t, false)}><span class="check">✓</span>Suivre {what}</button>
			</div>
		{/if}
		{#if hintMenu}
			{@const m = hintMenu}
			{@const name = m.e.t.columns[0]?.name}
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (hintMenu = null)}>
				<div class="menu-title" title={clipped(name) ? name : undefined}>Colonne « {clip(name)} »</div>
				<button
					onclick={() => {
						hintMenu = null;
						acceptKeyHint(m.e);
					}}>En faire la clé du tableau</button
				>
				{#if source.dismissKeyHint}
					{@const dismissHint = source.dismissKeyHint}
					<button
						onclick={() => {
							hintMenu = null;
							menuAction(() => dismissHint(m.e.t));
						}}>Ne plus proposer</button
					>
				{/if}
			</div>
		{/if}
		{#if freeMenu}
			{@const m = freeMenu}
			{@const below = groupCandidate()}
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (freeMenu = null)}>
				{#if below}
					<!-- Ligne juste au-dessus de l'en-tête d'un tableau : ses cases peuvent devenir les groupes d'en-têtes. -->
					<button title="Les cases de cette ligne, fusionnées ou non, deviennent les groupes d'en-têtes du tableau « {below.t.name} »" onclick={() => { freeMenu = null; menuAction(() => source.setTwoLevel!(below.t, true)); }}>Intégrer comme groupes d'en-têtes du tableau</button>
					<hr />
				{/if}
				<button data-shed="3"
					onclick={() => {
						freeMenu = null;
						copySelection(true);
					}}>Couper</button
				>
				<button data-shed="3"
					onclick={() => {
						freeMenu = null;
						copySelection();
					}}>Copier</button
				>
				<button data-shed="3"
					onclick={() => {
						freeMenu = null;
						pasteClipboard();
					}}>Coller</button
				>
				{#if chartClip}
					<button
						onclick={() => {
							freeMenu = null;
							pasteChart(chartClip!);
						}}>Coller le graphique</button
					>
				{/if}
				{#if buttonsLayer?.hasClip()}
					<button
						onclick={() => {
							freeMenu = null;
							buttonsLayer?.pasteAt(active.row, active.col);
						}}>Coller le bouton</button
					>
				{/if}
				<hr />
				{@render linesItems(m.kind, m.x, m.y)}
				{#if !m.kind && source.setPivot}
					{@const fp = pivotAt(active.row, active.col)}
					{#if fp?.rows}
						{@const info = pivotRowOf(fp, active.row)}
						<hr />
						<!-- Comme le sous-menu « Développer/Réduire » d'Excel. -->
						<Submenu label="Développer / Réduire">
							<button disabled={!foldTarget(info, true)} onclick={() => foldPivot(fp, 'expand')}>Développer</button>
							<button disabled={!foldTarget(info, false)} onclick={() => foldPivot(fp, 'collapse')}>Réduire</button>
							<hr />
							<button disabled={!canFoldAll(fp.rows, true)} onclick={() => foldPivot(fp, 'expandAll')}>Développer tout</button>
							<button disabled={!canFoldAll(fp.rows, false)} onclick={() => foldPivot(fp, 'collapseAll')}>Réduire tout</button>
						</Submenu>
					{/if}
				{/if}
				{#if !m.kind && source.deletePivot}
					<!-- Tout tableau croisé, même en erreur (source introuvable) : il se supprime ; en erreur, il se rattache à un autre tableau. -->
					{@const dp = pivotAt(active.row, active.col) ?? brokenPivotAt(pivots, active.row, active.col)}
					{#if dp}
						<hr />
						{#if dp.error && source.setPivot && pivotSources(dp).length}
							<button title="Rattacher le tableau croisé à un autre tableau : les champs qui y existent sont gardés" onclick={() => pickPivotSource(dp)}>Changer la source…</button>
						{/if}
						<button onclick={() => removePivot(dp)}>Supprimer le tableau croisé</button>
					{/if}
				{/if}
				{#if !m.kind}
					{@const join = !!source.joinTable && keptOut(active.row, active.col)}
					{@const asTable = !!source.formatAsTable && inRangeZone(active.row, active.col)}
					{#if source.applyStyle || source.setNote}
						<hr />
						{#if source.applyStyle}
							<button data-shed="1" onclick={() => openFormat()}>Format de cellule… <span class="keys">Ctrl+1</span></button>
						{/if}
						{#if source.setNote}{@render noteItems()}{/if}
					{/if}
					{#if join || asTable}
						{@const at = { row: active.row, col: active.col }}
						<hr />
						{#if join}
							<button data-shed="1" title="Cette cellule est gardée hors du tableau voisin (note, total) ; elle le rejoint" onclick={() => menuAction(() => source.joinTable!(at.row, at.col))}>Ajouter au tableau</button>
						{/if}
						{#if asTable}
							<button data-shed="1" title="La plage redevient un tableau (sa première ligne donne les en-têtes)" onclick={() => menuAction(() => source.formatAsTable!(at.row, at.col))}>Mettre sous forme de tableau</button>
						{/if}
					{/if}
				{/if}
				{#if onask}
					<hr />
					<button
						onclick={() => {
							freeMenu = null;
							onask();
						}}>Demander à l’assistant…</button
					>
				{/if}
			</div>
		{/if}
		{#if linesDlg}
			{@const d = linesDlg}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="rules lines-dlg" use:dismissPanel={() => (linesDlg = null)} onkeydown={(e) => e.key === 'Enter' && applyLinesDialog()}>
				<div class="menu-title">{d.insert ? 'Insérer' : 'Supprimer'}</div>
				<label><input type="radio" bind:group={d.choice} value="rows" /> {d.insert ? 'Ligne entière (au-dessus)' : 'Ligne entière'}</label>
				<label><input type="radio" bind:group={d.choice} value="cols" /> {d.insert ? 'Colonne entière (à gauche)' : 'Colonne entière'}</label>
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" onclick={applyLinesDialog}>OK</button>
					<button onclick={() => (linesDlg = null)}>Annuler</button>
				</div>
			</div>
		{/if}
		{#if keyDlg}
			{@const d = keyDlg}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="rules lines-dlg key-dlg"
				role="dialog"
				tabindex="-1"
				aria-label="Clé de la recherche"
				use:focusFirst
				use:dismissPanel={cancelKeyDialog}
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Enter') applyKeyDialog();
					if (ev.key === 'Escape') cancelKeyDialog();
				}}
			>
				<div class="menu-title">La recherche dans « {d.table} » utilise :</div>
				<label><input type="radio" bind:group={d.choice} value="index" /> le numéro de ligne actuel du tableau (index)</label>
				<label title={d.why ? `« ${d.column} » ne peut pas servir de clé : ${d.why}` : undefined}>
					<input type="radio" bind:group={d.choice} value="column" disabled={!!d.why} /> la colonne « {d.column} », qui devient la clé du tableau (les doublons y seront refusés)
				</label>
				{#if d.why}
					<p class="menu-note">« {d.column} » ne peut pas servir de clé : {d.why}.</p>
				{:else if d.choice === 'index'}
					<p class="menu-note">Le numéro change si vous triez ou insérez des lignes. Lattis ne proposera plus de clé pour ce tableau.</p>
				{/if}
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" onclick={applyKeyDialog}>Valider</button>
					<button onclick={cancelKeyDialog}>Annuler</button>
				</div>
			</div>
		{/if}
		{#if handDlg}
			{@const d = handDlg}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="rules lines-dlg key-dlg"
				role="dialog"
				tabindex="-1"
				aria-label="Clé des colonnes saisies"
				use:focusFirst
				use:dismissPanel={cancelHandDialog}
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Enter') applyHandDialog();
					if (ev.key === 'Escape') cancelHandDialog();
				}}
			>
				<div class="menu-title">Les valeurs saisies à côté de « {d.table} » suivent ses lignes par :</div>
				{#each d.columns as c (c)}
					<label><input type="radio" bind:group={d.choice} value={c} /> {c.includes(' + ') ? 'les colonnes' : 'la colonne'} « {c} »</label>
				{/each}
				<label><input type="radio" bind:group={d.choice} value="#" /> le rang des lignes (déconseillé)</label>
				{#if d.choice === '#'}
					<p class="menu-note">Si la requête rend ses lignes dans un autre ordre, ou en ajoute ou en retire, les valeurs saisies se retrouveront en face d’autres lignes.</p>
				{:else if d.t}
					<p class="menu-note">Les valeurs déjà saisies restent sur leurs lignes ; celles mises de côté (lignes disparues) sont oubliées. Les doublons et cases vides de la colonne sont refusés.</p>
				{:else}
					<p class="menu-note">À chaque actualisation, une valeur revient sur la ligne qui a la même « {d.choice} » ; si cette ligne disparaît, la valeur est mise de côté et revient avec elle.</p>
				{/if}
				{#if !d.columns.length && !d.t}
					<p class="menu-note">Aucune colonne du résultat n’identifie ses lignes (valeurs en double ou cases vides) : ajoutez-en une à la requête (un identifiant) pour une jointure fiable.</p>
				{/if}
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" onclick={applyHandDialog}>Valider</button>
					<button onclick={cancelHandDialog}>Annuler</button>
				</div>
			</div>
		{/if}
		{#if chartMenu}
			{@const m = chartMenu}
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (chartMenu = null)}>
				<div class="menu-title">Graphique</div>
				<button onclick={() => copyChart(m.ch, true)}>Couper</button>
				<button onclick={() => copyChart(m.ch, false)}>Copier</button>
				{#if m.ch.config.kind !== 'map' && !m.ch.error}<button onclick={() => copyChartImage(m.ch)}>Copier comme image</button>{/if}
				<button
					disabled={!chartClip}
					onclick={() => {
						chartMenu = null;
						if (chartClip) pasteChart(chartClip);
					}}>Coller un graphique à la cellule active</button
				>
				<hr />
				<button onclick={() => removeChart(m.ch.id)}>Supprimer le graphique</button>
			</div>
		{/if}
		{#if customDlg}
			{@const d = customDlg}
			<CustomSort
				field={d.field}
				loadValues={d.loadValues}
				loadOrder={d.loadOrder}
				loadLists={source.customLists}
				saveLists={source.setCustomLists}
				onapply={(o) => {
					customDlg = null;
					d.apply(o);
				}}
				onclose={() => {
					customDlg = null;
					wrap.focus();
				}}
			/>
		{/if}
		{#if ttcDlg && source.textColumnsPreview && source.textToColumns}
			{@const d = ttcDlg}
			{@const pv = source.textColumnsPreview}
			{@const run = source.textToColumns}
			<TextToColumns
				preview={(o) => pv(d.r0, d.r1, d.col, o)}
				apply={(o) => run(d.r0, d.r1, d.col, o)}
				ondone={(n, table) => {
					ttcDlg = null;
					wrap.focus();
					say(`Texte converti en ${n} colonne${n > 1 ? 's' : ''}${table ? ` (tableau ${table})` : ''} (Ctrl+Z pour annuler)`);
				}}
				onclose={() => {
					ttcDlg = null;
					wrap.focus();
				}}
			/>
		{/if}
		{#if dedupDlg}
			{@const d = dedupDlg}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="rules filter"
				use:dismissPanel={() => (dedupDlg = null)}
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Escape') {
						dedupDlg = null;
						wrap.focus();
					}
				}}
			>
				<div class="menu-title">Supprimer les doublons de « {d.e.t.name} »</div>
				<p class="menu-note">Deux lignes sont en double quand les colonnes cochées ont les mêmes valeurs. La première est gardée.</p>
				<div class="values">
					<label class="all">
						<input
							type="checkbox"
							checked={d.on.every((x, i) => x || !!d.e.t.columns[i].formula)}
							onchange={(ev) => {
								const on = (ev.currentTarget as HTMLInputElement).checked;
								d.on = d.e.t.columns.map((c) => on && !c.formula);
								previewDedup();
							}}
						/>
						(Toutes les colonnes)
					</label>
					{#each d.e.t.columns as c, i (i)}
						<label>
							<input
								type="checkbox"
								disabled={!!c.formula}
								checked={d.on[i]}
								onchange={(ev) => {
									d.on[i] = (ev.currentTarget as HTMLInputElement).checked;
									previewDedup();
								}}
							/>
							<span class="what">{c.name}</span>
						</label>
					{/each}
				</div>
				<p class="menu-note">
					{d.count === null ? 'Calcul…' : d.count === 0 ? 'Aucune ligne en double avec ces colonnes.' : `${d.count} ligne${d.count > 1 ? 's' : ''} en double ser${d.count > 1 ? 'ont' : 'a'} supprimée${d.count > 1 ? 's' : ''}.`}
				</p>
				{#if d.error}<p class="menu-note error">{d.error}</p>{/if}
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" disabled={!d.count} onclick={applyDedup}>Supprimer</button>
					<button
						onclick={() => {
							dedupDlg = null;
							wrap.focus();
						}}>Annuler</button
					>
				</div>
			</div>
		{/if}
		{#if fnDlg}
			{@const d = fnDlg}
			{@const shownFns = shownFunctions()}
			{@const cur = shownFns.find((f) => f.name === d.sel) ?? shownFns[0]}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="rules filter"
				use:dismissPanel={() => (fnDlg = null)}
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Escape') {
						fnDlg = null;
						wrap.focus();
					}
				}}
			>
				<div class="menu-title">Insérer une fonction ({catalog?.functions.length ?? 0} disponibles)</div>
				<!-- svelte-ignore a11y_autofocus -->
				<div class="add"><input class="search" bind:value={d.search} placeholder="Rechercher (SOMME, RECHERCHEV, date, texte…)" autofocus /></div>
				<div class="values">
					{#each shownFns as f (f.name)}
						<button
							class="fn"
							class:on={f.name === cur?.name}
							onmouseenter={() => (d.sel = f.name)}
							onfocus={() => (d.sel = f.name)}
							onclick={() => startFormula(`=${f.name}(`)}
						>
							<strong>{f.name}</strong>
							<span class="count fn-desc">{f.description ?? ''}</span>
						</button>
					{/each}
				</div>
				{#if cur}
					<div class="fn-doc">
						<div class="fn-sig">{cur.signature || cur.name + '()'}</div>
						<div>{cur.description}</div>
						{#if cur.args?.length}
							<dl>
								{#each cur.args as arg (arg.name)}
									<dt>{arg.name}{arg.optional ? ' (facultatif)' : ''}</dt>
									<dd>{arg.desc}</dd>
								{/each}
							</dl>
							{#if cur.repeat}<div class="count">Les {cur.repeat > 1 ? `${cur.repeat} derniers arguments peuvent` : 'dernier argument peut'} se répéter.</div>{/if}
						{/if}
						{#if cur.example}<div>Exemple : <code>{cur.example}</code></div>{/if}
						<div class="count">{cur.category}{cur.other && cur.other !== cur.name ? ` · en anglais : ${cur.other}` : ''}</div>
					</div>
				{/if}
				<div class="foot">
					<span class="sp"></span>
					<button
						onclick={() => {
							fnDlg = null;
							wrap.focus();
						}}>Fermer</button
					>
				</div>
			</div>
		{/if}
		{#if pivotDlg}
			{@const d = pivotDlg}
			{@const shown = searchValues(d.values, d.search)}
			<!-- Menu ▾ d'un TCD (comme Excel) : tri des étiquettes, valeurs à cocher. -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="rules filter colfilter anchored"
				style:left="{d.x}px"
				style:top="{d.y}px"
				style:max-height="{Math.max(200, toView(viewH) - d.y - 8)}px"
				use:focusFirst
				use:dismissPanel={() => (pivotDlg = null)}
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Escape') pivotDlg = null;
					if (ev.key === 'Enter' && !(ev.target instanceof HTMLButtonElement)) applyPivotFilter();
				}}
			>
				{#if d.fields && d.fields.length > 1}
					<label class="pick-field"
						>Champ :
						<select
							value={d.field}
							onchange={(ev) => {
								d.field = (ev.currentTarget as HTMLSelectElement).value;
								d.search = '';
								loadPivotValues();
							}}
						>
							{#each d.fields as f (f)}<option value={f}>{f}</option>{/each}
						</select>
					</label>
				{:else}
					<div class="menu-title" title={clipped(d.field) ? d.field : undefined}>« {clip(d.field)} »{d.kind === 'filter' ? ' · filtre du rapport' : ''}</div>
				{/if}
				{#if d.kind === 'rows'}
					<button class="sort" class:on={d.p.config.sort?.[d.field] === 'asc'} onclick={() => sortPivot('asc')}>Trier de A à Z</button>
					<button class="sort" class:on={d.p.config.sort?.[d.field] === 'desc'} onclick={() => sortPivot('desc')}>Trier de Z à A</button>
					{#if source.pivotFieldValues}
						<button class="sort" class:on={!!d.p.config.orders?.[d.field]} onclick={openPivotCustomSort}>Tri personnalisé…</button>
					{/if}
					<hr />
				{/if}
				{#if d.kind === 'filter' ? d.p.config.filters?.some((f) => f.field === d.field && f.values.length) : pivotFiltered(d.p, d.field)}
					<button class="sort" onclick={() => applyPivotFilter(true)}>Effacer le filtre de « {d.field} »</button>
				{/if}
				{#if source.setTabChoice}
					{@const tc = tabChoiceFor(d.p.config.source, d.field, { field: true })}
					<button class="sort" title={tc.title} disabled={tc.disabled} onclick={() => runTabChoice(d.p.config.source, d.field, tc)}>{tc.label}</button>
					<hr />
				{/if}
				<div class="add">
					<input class="search" bind:value={d.search} placeholder="Rechercher" />
				</div>
				<div class="values">
					{#if d.loading}
						<p class="loading"><span class="spinner" aria-hidden="true"></span> Chargement des valeurs…</p>
					{:else}
						<label class="all">
							<input
								type="checkbox"
								checked={shown.every((v) => v.on)}
								onchange={(ev) => {
									const on = (ev.currentTarget as HTMLInputElement).checked;
									for (const v of shown) v.on = on;
								}}
							/>
							{d.search.trim() ? '(Sélectionner tous les résultats)' : d.kind === 'filter' ? '(Tous)' : '(Sélectionner tout)'}
						</label>
						{#each shown as v (v.text)}
							<label>
								<input type="checkbox" bind:checked={v.on} />
								<span class="what">{v.text === '' ? '(Vides)' : v.text}</span>
								<span class="count">{v.count}</span>
							</label>
						{/each}
					{/if}
				</div>
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" disabled={d.loading} onclick={() => applyPivotFilter()}>OK</button>
					<button onclick={() => (pivotDlg = null)}>Annuler</button>
				</div>
			</div>
		{/if}
		{#if filterDlg}
			{@const d = filterDlg}
			{@const shown = filterShown()}
			{@const fcol = d.e.t.columns[d.i]}
			{@const kind = filterKind(((fcol?.formula ? fcol?.result : fcol?.type) || 'text') as ColumnType)}
			{@const filterable = !!source.setFilter}
			{@const typed = !!source.setType && !fcol?.formula}
			<!-- Menu ▾ de la colonne (comme Excel) : tri, filtres, type de données, clé. -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="rules filter colfilter anchored"
				style:left="{d.x}px"
				style:top="{d.y}px"
				style:max-height="{Math.max(200, toView(viewH) - d.y - 8)}px"
				use:focusFirst
				use:dismissPanel={dismissFilter}
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Escape') closeFilter();
					if (ev.key === 'Enter' && !(ev.target instanceof HTMLButtonElement)) applyFilter();
				}}
			>
				<div class="menu-title" title={clipped(fcol?.name) ? fcol?.name : undefined}>« {clip(fcol?.name)} »</div>
				{#if source.sortTable}
					<!-- Tri en haut du panneau, comme Excel. -->
					<button
						class="sort"
						onclick={() => {
							filterDlg = null;
							menuAction(() => source.sortTable!(d.e.t, d.i, false));
						}}>{sortLabels(fcol?.type ?? 'text', fcol?.result)[0]}</button
					>
					<button
						class="sort"
						onclick={() => {
							filterDlg = null;
							menuAction(() => source.sortTable!(d.e.t, d.i, true));
						}}>{sortLabels(fcol?.type ?? 'text', fcol?.result)[1]}</button
					>
					{#if source.sortTableCustom}
						<button class="sort" onclick={() => openCustomSort(d.e, d.i)}>Tri personnalisé…</button>
					{/if}
					<hr />
				{/if}
				{#if fcol?.filter}
					<button class="sort" onclick={() => applyFilter(true)}>Effacer le filtre de « {fcol.name} »</button>
				{/if}
				{#if source.setTabChoice && fcol}
					{@const tc = tabChoiceFor(d.e.t.name, fcol.name, { formula: !!fcol.formula })}
					<button class="sort" title={tc.title} disabled={tc.disabled} onclick={() => runTabChoice(d.e.t.name, fcol.name, tc)}>{tc.label}</button>
					<hr />
				{/if}
				{#if fcol?.formula}
					<p class="menu-note">Colonne calculée : triée et filtrée sur ses valeurs ; son type suit sa formule.</p>
				{/if}
				{#if filterable}
					<!-- Filtres par condition, comme le sous-menu d'Excel. -->
					<button class="sort cond-menu" class:on={d.sub} aria-expanded={d.sub} onclick={() => (d.sub = !d.sub)}>
						{kindTitle(kind)}<span class="sp"></span>{d.sub ? '▾' : '▸'}
					</button>
					{#if d.sub}
						<div class="cond-list">
							{#each condOps(kind) as op (op)}
								<button class="sort" onclick={() => startCustom(op)}>{condLabel(op, kind)}{condArgs(op) ? '…' : ''}</button>
							{/each}
							<button class="sort" onclick={() => startCustom('eq')}>Filtre personnalisé…</button>
						</div>
					{/if}
					{#if fcol?.filter?.conds?.length && !d.custom}
						<p class="menu-note">
							Filtre actif : {describeConds(fcol.filter, kind)}
							<button class="link" onclick={() => startCustom()}>Modifier…</button>
						</p>
					{/if}
					<hr />
					{#if d.custom}
						{@const cu = d.custom}
						<div class="custom">
							<p class="menu-note">Afficher les lignes dont « {fcol?.name} » :</p>
							{#each cu.conds as c, k (k)}
								{#if k === 1}
									<div class="andor">
										<label><input type="radio" bind:group={cu.or} value={false} /> Et</label>
										<label><input type="radio" bind:group={cu.or} value={true} /> Ou</label>
									</div>
								{/if}
								<div class="add">
									<select bind:value={c.op} aria-label="Condition {k + 1}">
										{#if k > 0}<option value="">(aucune)</option>{/if}
										{#each condOps(kind) as op (op)}<option value={op}>{condLabel(op, kind)}</option>{/each}
									</select>
								</div>
								{#if c.op && condArgs(c.op) > 0}
									<div class="add">
										<input bind:value={c.value} placeholder={c.op === 'top' || c.op === 'bottom' ? 'nombre' : kind === 'date' ? 'jj/mm/aaaa' : 'valeur'} />
										{#if condArgs(c.op) === 2}<span>et</span><input bind:value={c.to} placeholder={kind === 'date' ? 'jj/mm/aaaa' : 'valeur'} />{/if}
									</div>
								{/if}
							{/each}
							{#if kind === 'text'}<p class="menu-note">* remplace plusieurs caractères, ? un seul.</p>{/if}
						</div>
					{:else}
						<div class="add">
							<input class="search" bind:value={d.search} oninput={() => (d.touched = true)} placeholder="Rechercher" />
						</div>
						<div class="values">
							{#if d.loading}
								<p class="loading"><span class="spinner" aria-hidden="true"></span> Chargement des valeurs…</p>
							{:else}
							<label class="all">
								<input
									type="checkbox"
									checked={shown.every((v) => v.on)}
									onchange={(ev) => {
										const on = (ev.currentTarget as HTMLInputElement).checked;
										for (const v of shown) v.on = on;
										d.touched = true;
									}}
								/>
								{d.search.trim() ? '(Sélectionner tous les résultats)' : '(Sélectionner tout)'}
							</label>
							{#each shown as v (v.text)}
								<label>
									<input type="checkbox" bind:checked={v.on} onchange={() => (d.touched = true)} />
									<span class="what">{v.text === '' ? '(Vides)' : v.text}</span>
									<span class="count">{v.count}</span>
								</label>
							{/each}
							{#if d.values.length >= 1000}<p class="menu-note">Seules les 1 000 premières valeurs sont listées.</p>{/if}
							{/if}
						</div>
					{/if}
				{/if}
				{#if d.e.t.columns[d.i]?.errors}
					<div class="add">
						<label><input type="checkbox" bind:checked={d.errors} /> Saisies hors type seulement ({d.e.t.columns[d.i]?.errors})</label>
					</div>
				{/if}
				{#if d.error}<p class="menu-note error">{d.error}</p>{/if}
				{#if filterable}
					<div class="foot">
						{#if d.custom}<button onclick={() => (d.custom = null)}>‹ Liste des valeurs</button>{/if}
						<span class="sp"></span>
						<button class="primary" disabled={d.loading && !d.custom} onclick={() => applyFilter()}>OK</button>
						<button onclick={closeFilter}>Annuler</button>
					</div>
				{/if}
				{#if typed}
					<!-- Type de données de la colonne (inféré ou fixé), puis la clé du tableau. -->
					<hr />
					<div class="menu-title">Type de données · {fcol.fixed ? 'fixé' : 'inféré'}</div>
					{#if d.pending}
						{@const p = d.pending}
						<p class="menu-note">
							{p.n > 1 ? `${p.n} valeurs ne peuvent` : '1 valeur ne peut'} pas devenir « {typeLabel(p.to)} » : {p.n > 1 ? 'elles seront gardées et marquées' : 'elle sera gardée et marquée'} en erreur.
						</p>
						<div class="foot">
							<span class="sp"></span>
							<button class="primary" onclick={() => chooseType(p.to, true)}>Convertir quand même</button>
							<button onclick={() => (d.pending = undefined)}>Annuler</button>
						</div>
					{:else}
						<div class="types" role="radiogroup" aria-label="Type de données">
							{#each TYPES as t (t)}
								<button
									class="sort type"
									class:on={t === fcol.type}
									role="radio"
									aria-checked={t === fcol.type}
									title={t === fcol.type ? (fcol.fixed ? 'Type fixé' : 'Type inféré : le choisir le fixe') : `Convertir en ${typeLabel(t)}`}
									onclick={() => chooseType(t)}
								>
									<span class="check">{t === fcol.type ? '✓' : ''}</span>{#if typeIconName(t)}<span class="type-icon"><Icon name={typeIconName(t)!} size={14} /></span>{:else}<span class="type-icon">{typeIcon(t)}</span>{/if}{typeLabel(t)}
								</button>
							{/each}
						</div>
						{#if d.mix && d.mix.type}
							{@const to = d.mix.type}
							<p class="menu-note">{mixText(d.mix)}</p>
							<button class="sort" onclick={() => chooseType(to, true)}
								>Repasser en {typeLabel(to)} ({d.mix.off} valeur{d.mix.off > 1 ? 's' : ''} marquée{d.mix.off > 1 ? 's' : ''} en erreur)</button
							>
						{/if}
						{#if fcol.errors}<p class="menu-note error">{fcol.errors} valeur{fcol.errors > 1 ? 's' : ''} hors type dans la colonne</p>{/if}
					{/if}
				{/if}
				{#if source.setKey && !fcol?.formula}
					<hr />
					{#if d.e.t.key === d.i}
						<p class="menu-note">🔑 Clé du tableau : ses valeurs identifient les lignes.{d.e.t.autoNumber === d.i ? ' Une ligne ajoutée reçoit le numéro suivant.' : ''}</p>
						<button class="sort" onclick={() => chooseKey(false)}>Retirer la clé</button>
					{:else if d.keyWhy === undefined}
						<p class="menu-note loading"><span class="spinner" aria-hidden="true"></span> Vérification de la clé…</p>
					{:else if d.keyWhy === ''}
						<button class="sort" onclick={() => chooseKey(true)}>🔑 Utiliser comme clé{d.e.t.key !== undefined ? ' (à la place de l’actuelle)' : ''}</button>
					{:else if d.keyWhy}
						<p class="menu-note">Clé impossible : {d.keyWhy}</p>
					{/if}
					{#if d.e.t.autoNumber === d.i && d.e.t.key !== d.i}
						<p class="menu-note">Suite de numéros : une ligne ajoutée à la fin reçoit le numéro suivant.</p>
					{/if}
				{/if}
			</div>
		{/if}
		{#if cfView}
			{#key cfView}
				<CondFormat host={cfHost} view={cfView} onclose={closeRules} />
			{/key}
		{/if}
		{#if formatDlg}
			{@const d = formatDlg}
			<FormatDialog
				style={d.style}
				sample={d.sample}
				tab={d.tab}
				canMerge={d.canMerge}
				merged={d.merged}
				onapply={applyFormat}
				onclose={() => {
					formatDlg = null;
					wrap.focus();
				}}
			/>
		{/if}
		{#if validDlg && dvPick}
			<!-- Plage source pointée dans la grille : le dialogue est retiré, comme le bouton « réduire » d'Excel. -->
			<div class="pick-bar" role="status">
				<span>Plage source : <b>{dvPickRef || '…'}</b></span>
				<span class="sp"></span>
				<button class="primary" onclick={() => endPick(true)}>OK <span class="keys">Entrée</span></button>
				<button onclick={() => endPick(false)}>Annuler <span class="keys">Échap</span></button>
			</div>
		{:else if validDlg}
			{@const d = validDlg}
			<!-- Boîte modale, comme celle d'Excel : un clic à côté ou un redimensionnement ne la ferme plus. -->
			<div class="dlg-back" role="presentation">
			<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
			<div
				class="rules valid-dlg"
				role="dialog"
				aria-modal="true"
				aria-label="Validation des données — {d.target}"
				tabindex="-1"
				use:focusFirst
				use:modal
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Escape') {
						clearTimeout(previewTimer);
						validDlg = null;
						wrap.focus();
					}
				}}
			>
				<div class="menu-title" title={`Validation des données — ${d.target}`}>Validation des données — {d.target}</div>
				<div class="dv-mode" role="radiogroup" aria-label="Source de la liste">
					<label><input type="radio" name="dv-mode" value="list" bind:group={d.mode} onchange={() => (d.error = '')} /> Valeurs saisies</label>
					<label><input type="radio" name="dv-mode" value="range" bind:group={d.mode} onchange={() => ((d.error = ''), refreshPreview(0))} /> Plage</label>
				</div>
				{#if d.mode === 'list'}
					<p class="menu-note">
						Valeurs permises dans la sélection, une par ligne (ou séparées par « ; »). Une flèche ▾ propose la liste dans la cellule (Alt+↓) ; toute autre saisie est
						refusée. Sur toute une colonne de tableau, la liste vaut aussi pour les lignes ajoutées ensuite.
					</p>
					<div class="add"><textarea bind:value={d.text} rows="7" placeholder={'Oui\nNon'}></textarea></div>
				{:else}
					<p class="menu-note">
						Les valeurs de la plage (distinctes, sans les vides, dans l’ordre) sont relues à chaque ouverture de la liste et à chaque saisie : une colonne de tableau
						(Clients[Ville]), des cellules (Cell[A1:A10], Feuille2!Cell[A1:A10]) ou une cellule nommée. La référence suit les renommages et déplacements.
					</p>
					<div class="add dv-src">
						<input
							type="text"
							aria-label="Plage source"
							placeholder="Clients[Ville]"
							spellcheck="false"
							bind:value={d.src}
							oninput={() => ((d.error = ''), refreshPreview())}
							onkeydown={(ev) => {
								if (ev.key === 'Enter') applyValidation();
							}}
						/>
						<button title="Désigner la plage dans la feuille (Entrée pour la reprendre)" onclick={startPick}>Sélectionner</button>
					</div>
					{#if d.loading}
						<p class="menu-note">Lecture de la plage…</p>
					{:else if d.preview}
						<p class="menu-note" class:error={!!d.preview.error || !d.preview.values.length}>{dvPreview(d.preview)}</p>
					{/if}
				{/if}
				{#if d.error}<p class="menu-note error">{d.error}</p>{/if}
				<div class="foot">
					{#if listOf(active.row, active.col)}<button onclick={() => applyValidation(true)}>Effacer la validation</button>{/if}
					<span class="sp"></span>
					<button class="primary" onclick={() => applyValidation()}>OK</button>
					<button
						onclick={() => {
							clearTimeout(previewTimer);
							validDlg = null;
							wrap.focus();
						}}>Annuler</button
					>
				</div>
			</div>
			</div>
		{/if}
		{#if noteDlg}
			{@const d = noteDlg}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="note-dlg"
				style:left="{d.x}px"
				style:top="{d.y}px"
				use:focusFirst
				use:dismissPanel={() => noteDlg && saveNote()}
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Escape' || (ev.key === 'Enter' && ev.ctrlKey)) saveNote();
				}}
			>
				<div class="note-bar">
					{#if source.pinNote}
						<button class="note-ico" class:on={d.pinned} title={d.pinned ? 'Ne plus épingler (infobulle au survol)' : 'Épingler : toujours afficher la note'} aria-pressed={d.pinned} onclick={togglePin}
							><Icon name="pin" size={15} /></button
						>
					{/if}
					<button class="note-ico" title="Supprimer la note" onclick={() => saveNote(true)}><Icon name="trash" size={15} /></button>
				</div>
				<textarea bind:value={d.text} rows="6" placeholder="Note de la cellule" aria-label="Note de la cellule"></textarea>
			</div>
		{/if}
		{#if listMenu}
			{@const m = listMenu}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="menu list-menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (listMenu = null)}>
				{#each m.values as v (v)}
					<button class:on={cellInfo(m.row, m.col).value === v} onclick={() => pickFromList(v)}>{v}</button>
				{/each}
				{#if m.error}<p class="menu-note error">Liste introuvable : {m.error}</p>{:else if m.loading && !m.values.length}<p class="menu-note">Lecture de la liste…</p>{:else if !m.values.length}<p class="menu-note">La plage de la liste est vide</p>{/if}
			</div>
		{/if}
		{#if sizeDlg}
			{@const d = sizeDlg}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="rules lines-dlg"
				use:focusFirst
				use:dismissPanel={() => (sizeDlg = null)}
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Enter') applySize();
					if (ev.key === 'Escape') {
						sizeDlg = null;
						wrap.focus();
					}
				}}
			>
				<div class="menu-title">{d.rows ? 'Hauteur de ligne' : 'Largeur de colonne'} (pixels)</div>
				<div class="add"><input type="number" min="4" max="1000" bind:value={d.value} /></div>
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" onclick={applySize}>OK</button>
					<button
						onclick={() => {
							sizeDlg = null;
							wrap.focus();
						}}>Annuler</button
					>
				</div>
			</div>
		{/if}
		{#if editing && !editRO && (completion?.where === 'cell' || sig?.where === 'cell' || isFormula(editValueText))}
			<div
				class="suggest"
				style:left="{suggestPos.x}px"
				style:top="{suggestPos.y}px"
			>
				{@render suggestions('cell')}
			</div>
		{/if}
	</div>
	{#if panel?.kind === 'pivot'}
		{@const pv = pivots.find((p) => p.id === panel!.id)}
		{#if pv}
			{#key pv.id}
			<PivotPanel
				config={pv.config}
				fields={sourceColumns(pv.config.source)}
				error={pv.error}
				filterValues={source.pivotFieldValues ? (f) => source.pivotFieldValues!(pv.config.source, f) : undefined}
				tabChoice={source.setTabChoice ? (f) => tabChoiceFor(pv.config.source, f, { field: true }) : undefined}
				ontabchoice={(f, it) => runTabChoice(pv.config.source, f, it)}
				customLists={source.customLists}
				setCustomLists={source.setCustomLists}
				onchange={(cfg) => menuAction(source.setPivot && (() => source.setPivot!(pv.id, cfg)), true)}
				ondelete={() => removePivot(pv)}
				sources={pv.error && source.setPivot ? pivotSources(pv) : []}
				onsource={(name) => repointPivot(pv, name)}
				pickSource={pivotSourcePick === pv.id}
				{locked}
				onclose={() => {
					panel = null;
					pivotSourcePick = null;
				}}
			/>
			{/key}
		{/if}
	{:else if panel?.kind === 'chart'}
		{@const ch = charts.find((c) => c.id === panel!.id)}
		{#if ch}
			<ChartPanel
				config={ch.config}
				guess={ch.guess}
				invalid={ch.invalid}
				info={ch}
				onSheet
				sources={sources.map((s) => ({ name: s.name, columns: sourceColumns(s.name) }))}
				columns={sourceColumns(ch.config.source).map((c) => c.name)}
				onchange={(cfg) => menuAction(source.setChart && (() => source.setChart!(ch.id, cfg)), true)}
				onclose={() => {
					panel = null;
					selChart = null;
				}}
				{locked}
			/>
		{/if}
	{/if}
	</div>
</div>

<style>
	/* Bouton de validation des boîtes de dialogue : vert, comme Excel. */
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
	button.primary:disabled {
		opacity: 0.5;
	}
	.sheet-root {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.formula-bar {
		display: flex;
		align-items: center;
		gap: 6px;
		height: 30px;
		padding: 0 8px;
		border-bottom: 1px solid #d4d7db;
		background: #fff;
	}
	.name-box {
		width: 260px;
		font: inherit;
		outline: none;
		padding: 3px 8px;
		border: 1px solid #d4d7db;
		border-radius: 3px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		color: #374151;
	}
	.name-box.split {
		display: flex;
		align-items: center;
		padding: 0;
		overflow: visible;
	}
	.name-box.split input {
		font: inherit;
		min-width: 0;
		border: none;
		outline: none;
		padding: 3px 6px;
		background: none;
		color: #374151;
	}
	.name-box.split .table-name {
		flex: 0 1 45%;
		font-weight: 600;
	}
	.name-box.split .table-name.default-name {
		color: #c42b1c; /* DEFAULT_NAME_COLOR : nom par défaut, à renommer */
	}
	.name-box.split .col-name {
		flex: 1 1 55%;
	}
	.name-box.split input:focus {
		background: #eef6f0;
	}
	.slash {
		color: #9aa0a6;
	}
	.fx {
		color: #6b7280;
		font-style: italic;
		padding: 0 6px;
		border-left: 1px solid #d4d7db;
	}
	.fx-input {
		flex: 1;
		font: inherit;
		border: none;
		outline: none;
		padding: 4px 2px;
	}
	/* Barre de recherche en surimpression (la grille ne bouge pas). */
	/* Volet compact en haut à droite : champ Rechercher (loupe, compteur),
	   précédent / suivant, fermer ; Remplacer par repliable ; portée en
	   pastille. Champs et boutons comme les boîtes (DataCatalogDialog). */
	.finder {
		position: absolute;
		z-index: 7;
		top: 26px;
		right: 18px;
		width: 380px;
		max-width: calc(100% - 36px);
		box-sizing: border-box;
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 8px;
		background: #fff;
		border: 1px solid #d4d7db;
		border-top: 2px solid #217346;
		border-radius: var(--float-radius);
		box-shadow:
			0 6px 20px rgba(0, 0, 0, 0.14),
			0 1px 3px rgba(0, 0, 0, 0.08);
		font-size: 12px;
		color: #1f2937;
	}
	.fd-row {
		display: flex;
		align-items: center;
		gap: 2px;
		min-width: 0;
	}
	/* Sous le champ Rechercher, alignés sur lui (après la flèche Remplacer). */
	.fd-indent {
		padding-left: 26px;
	}
	.fd-field {
		flex: 1;
		min-width: 0;
		display: flex;
		align-items: center;
		gap: 6px;
		height: 26px;
		box-sizing: border-box;
		padding: 0 7px;
		margin-right: 4px;
		border: 1px solid #c9cdd2;
		border-radius: 4px;
		background: #fff;
		color: #6b7280;
		cursor: text;
	}
	.fd-field:hover {
		border-color: #a8aeb5;
	}
	.fd-field:focus-within {
		border-color: #217346;
		box-shadow: 0 0 0 2px rgba(33, 115, 70, 0.18);
		color: #217346;
	}
	.fd-field.miss {
		border-color: #dc2626;
		background: #fef6f6;
	}
	.fd-field.miss:focus-within {
		box-shadow: 0 0 0 2px rgba(220, 38, 38, 0.15);
	}
	.fd-field input {
		flex: 1;
		min-width: 0;
		font: inherit;
		font-size: 13px;
		color: #111827;
		border: none;
		outline: none;
		padding: 0;
		background: transparent;
	}
	.fd-field input::placeholder {
		color: #9ca3af;
	}
	.fd-count {
		flex: none;
		color: #6b7280;
		font-size: 11px;
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
	.fd-field.miss .fd-count {
		color: #b91c1c;
	}
	/* Boutons d'icône sans cadre, comme ceux des volets. */
	.fd-icon {
		flex: none;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		padding: 0;
		border: 1px solid transparent;
		border-radius: 4px;
		background: none;
		color: #4b5563;
		cursor: pointer;
	}
	.fd-icon:hover:not(:disabled) {
		background: #eef6f0;
		color: #217346;
	}
	.fd-icon:active:not(:disabled) {
		background: #d3e7dc;
	}
	.fd-icon:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.fd-icon:focus-visible,
	.fd-btn:focus-visible {
		outline: 2px solid #217346;
		outline-offset: 1px;
	}
	.fd-up :global(svg) {
		transform: rotate(180deg);
	}
	.fd-toggle {
		margin-right: 2px;
	}
	.fd-toggle :global(svg) {
		transform: rotate(-90deg);
		transition: transform 0.12s ease;
	}
	.fd-toggle.open :global(svg) {
		transform: none;
	}
	.fd-close {
		margin-left: 2px;
	}
	.fd-close:hover:not(:disabled) {
		background: #f3f4f6;
		color: #111827;
	}
	.fd-foot {
		gap: 6px;
		min-height: 24px;
	}
	.fd-spacer {
		flex: 1;
	}
	/* Portée : la feuille, ou la sélection qui borne la recherche (verte). */
	.fd-chip {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		min-width: 0;
		height: 20px;
		padding: 0 8px 0 6px;
		border-radius: 10px;
		background: #f3f4f6;
		color: #4b5563;
		font-size: 11px;
		white-space: nowrap;
		cursor: default;
	}
	.fd-chip span {
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.fd-chip.on {
		background: #e3efe7;
		color: #14532d;
		--icon-accent: #217346;
	}
	.fd-btn {
		flex: none;
		font: inherit;
		height: 24px;
		padding: 0 10px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		color: inherit;
		cursor: pointer;
		white-space: nowrap;
	}
	.fd-btn:hover:not(:disabled) {
		border-color: #217346;
		background: #eef6f0;
	}
	.fd-btn.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	.fd-btn.primary:hover:not(:disabled) {
		background: #1a5c38;
		border-color: #1a5c38;
	}
	.fd-btn:disabled {
		opacity: 0.55;
		cursor: default;
	}
	.fd-info {
		color: #4b5563;
		font-size: 11px;
		line-height: 1.35;
	}
	.bench {
		padding: 4px 10px;
		background: #eef6f1;
		color: #14532d;
		border-bottom: 1px solid #d3e7dc;
		white-space: nowrap;
		overflow: hidden;
	}
	.wrap {
		position: relative;
		flex: 1;
		min-height: 0;
		outline: none;
		overflow: hidden;
	}
	/* Calque zoomé : il couvre la zone de la grille, en pixels de la feuille. */
	.zoom {
		position: absolute;
		inset: 0;
		/* Sous la barre verticale (hors du canevas) : la ligne des lettres de colonnes se prolonge jusqu'au bord. */
		background: linear-gradient(to bottom, #f5f6f7 21px, #d4d7db 21px 22px, #fff 22px);
	}
	canvas {
		position: absolute;
		left: 0;
		top: 0;
	}
	.scroller {
		position: absolute;
		inset: 0;
		/* La barre horizontale est dans la ligne des onglets (SheetTabs). */
		overflow-x: hidden;
		overflow-y: auto;
	}
	/* Barre verticale dessinée (comme celle des onglets) : elle commence sous la
	   ligne des lettres de colonnes, sur fond transparent ; le curseur s'épaissit au survol. */
	.scroller::-webkit-scrollbar {
		width: 12px;
		background: transparent;
	}
	.scroller::-webkit-scrollbar-track {
		margin-top: 22px; /* TOP_H */
		background: transparent;
	}
	.scroller::-webkit-scrollbar-thumb {
		min-height: 32px;
		border: 3px solid transparent;
		border-radius: 6px;
		background: rgba(60, 64, 67, 0.35) padding-box;
	}
	.scroller::-webkit-scrollbar-thumb:hover {
		border-width: 2px;
		background-color: rgba(60, 64, 67, 0.55);
	}
	.scroller::-webkit-scrollbar-thumb:active {
		border-width: 2px;
		background-color: rgba(60, 64, 67, 0.65);
	}
	.sheet-root {
		position: relative;
	}
	.suggest {
		position: absolute;
		z-index: 6;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		min-width: 260px;
		max-width: 520px;
		font-size: 12px;
	}
	.fx-suggest {
		left: 300px;
		top: 30px;
	}
	.sig-foot {
		display: flex;
		padding: 5px 10px;
		border-top: 1px solid #e3e5e8;
	}
	.sig-foot:first-child {
		border-top: none;
	}
	.validate {
		font: inherit;
		font-size: 12px;
		padding: 2px 12px;
		border: 1px solid #1b5e3a;
		border-radius: 3px;
		background: #217346;
		color: #fff;
		cursor: pointer;
	}
	.validate:disabled {
		border-color: #d4d7db;
		background: #eef0f2;
		color: #9aa1a9;
		cursor: default;
	}
	.sig {
		padding: 5px 10px;
		background: #f7f8f9;
		color: #374151;
		border-bottom: 1px solid #e3e5e8;
		white-space: normal;
		line-height: 1.4;
	}
	.sig-line {
		font-family: Consolas, 'Courier New', monospace;
		color: #111827;
	}
	.sig-line b {
		color: #14532d;
	}
	.sig-desc {
		margin-top: 2px;
	}
	.sig-arg,
	.sig-ex {
		margin-top: 3px;
		color: #4b5563;
	}
	.sig code,
	.fn-doc code {
		font-family: Consolas, 'Courier New', monospace;
		background: #eef0f2;
		padding: 0 3px;
	}
	.sugg {
		display: flex;
		gap: 8px;
		align-items: center;
		padding: 3px 10px;
		cursor: pointer;
	}
	.sugg.on {
		background: #e3efe7;
	}
	.kind {
		width: 22px;
		color: #217346;
		font-weight: 600;
		text-align: center;
	}
	.lab {
		flex: 1;
	}
	.det {
		color: #6b7280;
		max-width: 260px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.menu {
		position: absolute;
		z-index: 5;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 240px;
		max-width: 320px; /* MENU_MAX_WIDTH : au-delà, les libellés s'abrègent (…) */
	}
	.menu > .menu-title {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.rules {
		position: absolute;
		z-index: 6;
		top: 60px;
		left: 50%;
		transform: translateX(-50%);
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 6px 18px rgba(0, 0, 0, 0.18);
		padding: 6px 0 8px;
		min-width: 460px;
	}
	.rules .rule,
	.rules .add,
	.rules .foot {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 3px 14px;
	}
	.rules .what {
		flex: 1;
	}
	.filter {
		min-width: 300px;
	}
	.colfilter {
		max-width: 310px;
		overflow-y: auto;
		box-sizing: border-box;
	}
	.rules.anchored {
		transform: none;
	}
	.filter .sort {
		display: block;
		width: 100%;
		text-align: left;
		padding: 5px 14px;
		border: none;
		border-radius: 0;
	}
	.filter .sort:hover {
		background: #e3efe7;
	}
	/* Section « Type de données » du menu ▾ de la colonne. */
	.filter .sort.type {
		padding: 3px 14px;
	}
	.filter .sort.type .type-icon {
		width: 18px;
		margin-right: 6px;
		justify-content: center;
	}
	.filter .sort.type.on {
		font-weight: 600;
	}
	.charts {
		position: absolute;
		overflow: hidden;
		pointer-events: none;
	}
	.chart {
		position: absolute;
		pointer-events: auto;
		border: 1px solid #d4d7db;
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.12);
		background: #fff;
		cursor: move;
	}
	.chart-error {
		margin: 12px;
		color: #c0392b;
	}
	.body {
		flex: 1;
		display: flex;
		min-height: 0;
	}
	.chart.selected {
		outline: 2px solid #217346;
	}
	/* Poignée : un simple carré vert. */
	.handle {
		position: absolute;
		width: 8px;
		height: 8px;
		background: #217346;
	}
	.handle.n,
	.handle.s {
		left: calc(50% - 4px);
		cursor: ns-resize;
	}
	.handle.e,
	.handle.w {
		top: calc(50% - 4px);
		cursor: ew-resize;
	}
	.handle.n,
	.handle.ne,
	.handle.nw {
		top: -5px;
	}
	.handle.s,
	.handle.se,
	.handle.sw {
		bottom: -5px;
	}
	.handle.e,
	.handle.ne,
	.handle.se {
		right: -5px;
	}
	.handle.w,
	.handle.nw,
	.handle.sw {
		left: -5px;
	}
	.handle.ne,
	.handle.sw {
		cursor: nesw-resize;
	}
	.handle.nw,
	.handle.se {
		cursor: nwse-resize;
	}
	.filter .fn {
		display: flex;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		text-align: left;
		border: none;
		background: none;
		padding: 3px 8px;
		cursor: pointer;
	}
	.filter .fn:hover,
	.filter .fn.on {
		background: #e3efe7;
	}
	.filter .fn strong {
		flex: none;
	}
	.filter .fn-desc {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		text-align: right;
	}
	.fn-doc {
		margin: 4px 14px;
		max-width: 520px;
		line-height: 1.45;
	}
	.fn-doc > div {
		margin-top: 3px;
	}
	.fn-doc .fn-sig {
		font-family: Consolas, 'Courier New', monospace;
		font-weight: 600;
		color: #14532d;
	}
	.fn-doc dl {
		margin: 4px 0;
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 2px 10px;
	}
	.fn-doc dt {
		font-family: Consolas, 'Courier New', monospace;
		font-weight: 600;
	}
	.fn-doc dd {
		margin: 0;
		color: #4b5563;
	}
	.filter .search {
		width: 100%;
	}
	/* Indicateur de chargement (liste des valeurs, contrôle de la clé). */
	.loading {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 6px 10px;
		color: #6b7280;
	}
	.spinner {
		width: 12px;
		height: 12px;
		flex: none;
		border: 2px solid #d1d5db;
		border-top-color: #217346;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.spinner {
			animation-duration: 2.5s;
		}
	}
	.filter .values {
		max-height: 260px;
		overflow: auto;
		margin: 4px 14px;
		border: 1px solid #e5e7eb;
		padding: 4px 0;
	}
	.filter label {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 1px 8px;
	}
	.filter .values input {
		width: auto;
	}
	.filter .count {
		color: #6b7280;
		font-size: 11px;
	}
	.filter .cond-menu {
		display: flex;
	}
	.filter .cond-menu.on {
		background: #e3efe7;
	}
	.filter .cond-list .sort {
		padding-left: 28px;
	}
	.filter .custom select {
		flex: 1;
		font: inherit;
		padding: 2px 4px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	.filter .custom input {
		flex: 1;
		min-width: 0;
	}
	.filter .andor {
		display: flex;
		gap: 12px;
		padding: 0 14px;
	}
	.filter .andor input {
		width: auto;
	}
	.filter .menu-note .link {
		border: none;
		background: none;
		color: #217346;
		padding: 0 4px;
		text-decoration: underline;
	}
	.rules .sample {
		padding: 1px 8px;
		border: 1px solid #d4d7db;
	}
	.rules input {
		font: inherit;
		width: 90px;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	.rules button {
		font: inherit;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.rules button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
	.lines-dlg input[type='radio'] {
		width: auto;
	}
	.rules .sp {
		flex: 1;
	}
	.rules hr {
		border: none;
		border-top: 1px solid #e5e7eb;
		margin: 6px 0;
	}
	.menu-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}
	.menu-title.default-name {
		color: #c42b1c; /* DEFAULT_NAME_COLOR */
	}
	/* Menu ▾ d'un TCD en arborescence : choix du champ des lignes (Excel). */
	.pick-field {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}
	.pick-field select {
		flex: 1;
		font: inherit;
		color: #1f2328;
	}
	.menu button {
		display: block;
		width: 100%;
		text-align: left;
		padding: 6px 14px;
		background: none;
		border: none;
		font: inherit;
		cursor: pointer;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.menu button:disabled {
		color: #9aa0a6;
		cursor: default;
	}
	.menu button:hover {
		background: #e3efe7;
	}
	.menu-note {
		margin: 4px 14px;
		max-width: 260px;
		font-size: 12px;
		color: #4b5563;
	}
	.menu-note.error {
		color: #c0392b;
	}
	.type-icon {
		display: inline-flex;
		vertical-align: -3px;
		color: #6b8f78;
		--icon-accent: #217346;
	}
	.check {
		display: inline-block;
		width: 16px;
		color: #217346;
	}
	.lines-dlg {
		left: 50%;
		top: 30%;
		transform: translateX(-50%);
		width: auto;
		min-width: 240px;
	}
	.lines-dlg label {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 3px 2px;
	}
	.menu hr {
		border: none;
		border-top: 1px solid #e3e5e8;
		margin: 4px 0;
	}
	.menu .keys {
		float: right;
		color: #6b7280;
		font-size: 12px;
		margin-left: 16px;
	}
	.list-menu {
		min-width: 140px;
		max-height: 260px;
		overflow: auto;
	}
	.list-menu button.on {
		background: #d3e7dc;
	}
	/* Fond des boîtes modales de la grille (validation des données). */
	.dlg-back {
		position: fixed;
		inset: 0;
		z-index: var(--z-dialog);
		background: var(--backdrop);
	}
	.valid-dlg {
		width: 420px;
		min-width: 0;
	}
	.key-dlg {
		max-width: 440px;
	}
	.key-dlg label {
		align-items: flex-start;
	}
	.key-dlg .menu-note {
		max-width: none;
	}
	.valid-dlg .menu-note {
		max-width: none;
	}
	.dv-mode {
		display: flex;
		gap: 16px;
		margin: 4px 14px;
	}
	.valid-dlg .dv-mode input {
		width: auto;
	}
	.dv-mode label {
		display: flex;
		align-items: center;
		gap: 4px;
	}
	.valid-dlg .dv-src input {
		flex: 1;
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	/* Plage source d'une validation pointée dans la grille : le dialogue retiré laisse cette barre. */
	.pick-bar {
		position: fixed;
		left: 50%;
		top: 12px;
		transform: translateX(-50%);
		z-index: var(--z-dialog);
		display: flex;
		align-items: center;
		gap: 8px;
		max-width: calc(100vw - 32px);
		padding: 6px 10px;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: 4px;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.18);
		font-size: 13px;
	}
	.pick-bar .sp {
		flex: 1;
	}
	.pick-bar button {
		font: inherit;
		padding: 2px 8px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.pick-bar button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	.pick-bar button.primary .keys {
		color: #d9eadf;
	}
	.pick-bar .keys {
		color: #6b7280;
		font-size: 11px;
		margin-left: 4px;
	}
	.valid-dlg textarea {
		flex: 1;
		font: inherit;
		padding: 4px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		resize: vertical;
	}
	/* Note : un post-it, sans fond blanc ni cadre de saisie ; les lignes
	   du papier suivent le texte (même interligne). */
	.note-dlg {
		position: absolute;
		z-index: 6;
		width: 250px;
		background: #fff3a8;
		box-shadow: 0 6px 14px rgba(0, 0, 0, 0.2), 0 1px 2px rgba(0, 0, 0, 0.12);
		padding: 2px 10px 10px;
	}
	.note-bar {
		display: flex;
		justify-content: flex-end;
		gap: 2px;
	}
	.note-ico {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		padding: 0;
		border: none;
		border-radius: 3px;
		background: transparent;
		color: #7a6a1f;
		cursor: pointer;
		--icon-accent: #7a6a1f;
	}
	.note-ico:hover {
		background: rgba(0, 0, 0, 0.07);
	}
	.note-ico.on {
		background: rgba(0, 0, 0, 0.12);
		color: #4a3f0f;
		--icon-accent: #c0392b;
	}
	.note-dlg textarea {
		display: block;
		width: 100%;
		box-sizing: border-box;
		font: inherit;
		line-height: 20px;
		padding: 0 2px;
		border: none;
		outline: none;
		resize: vertical;
		color: #3b3410;
		background: transparent repeating-linear-gradient(to bottom, transparent 0, transparent 19px, rgba(160, 130, 30, 0.35) 19px, rgba(160, 130, 30, 0.35) 20px);
		background-attachment: local;
	}
	.note-tip.pinned {
		background: #fff3a8;
		border: none;
		box-shadow: 0 3px 8px rgba(0, 0, 0, 0.18);
		z-index: 5;
	}
	.note-tip {
		position: absolute;
		z-index: 6;
		max-width: 240px;
		padding: 5px 8px;
		background: #fffbe6;
		border: 1px solid #d6c27a;
		border-radius: var(--float-radius);
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
		white-space: pre-wrap;
		pointer-events: none;
		font-size: 12px;
	}
	/* Éditeur de cellule : il s'élargit vers la droite sur les cellules vides,
	   puis passe à la ligne (placeEditor) ; une formule y est colorée par un
	   calque de texte sous la zone de saisie transparente. */
	.editor-box {
		position: absolute;
		left: 0;
		top: 0;
		height: 22px;
		box-sizing: border-box;
		border: 2px solid #217346;
		background: #fff;
		box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
		z-index: 2;
	}
	.editor,
	.editor-hl {
		position: absolute;
		inset: 0;
		box-sizing: border-box;
		margin: 0;
		border: 0;
		padding: 0 4px;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		line-height: 18px;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
		overflow: hidden;
	}
	.editor {
		width: 100%;
		height: 100%;
		resize: none;
		outline: none;
		background: transparent;
		color: #1f2328;
	}
	/* Éditeur en lecture seule (readonlyedit.ts) : fond grisé, bordure neutre,
	   mention « lecture seule » au-dessus ; le texte se sélectionne et se copie. */
	.editor-box.ro {
		border-color: #8c959f;
		background: #f3f4f6;
	}
	.editor.ro {
		color: #57606a;
		cursor: text;
	}
	.editor-ro {
		position: absolute;
		right: -2px;
		bottom: 100%;
		padding: 0 5px;
		background: #8c959f;
		color: #fff;
		font: 10px/14px 'Segoe UI', system-ui, -apple-system, sans-serif;
		white-space: nowrap;
		pointer-events: none;
	}
	.fx-input.ro {
		background: #f3f4f6;
		color: #57606a;
	}
	.editor.hl,
	.fx-input.hl {
		color: transparent;
		caret-color: #1f2328;
	}
	.editor-hl {
		color: #1f2328;
		pointer-events: none;
	}
	.hl-err {
		text-decoration: underline wavy #c8302c;
		text-decoration-skip-ink: none;
		background: rgba(200, 48, 44, 0.12);
	}
	.fx-wrap {
		position: relative;
		flex: 1;
		display: flex;
		overflow: hidden;
	}
	.fx-scope {
		flex: none;
		align-self: center;
		margin: 0 6px;
		padding: 1px 8px;
		border-radius: 10px;
		background: #e3efe7;
		color: #185c37;
		font-size: 11px;
		white-space: nowrap;
	}
	.fx-hl {
		position: absolute;
		left: 0;
		top: 0;
		bottom: 0;
		padding: 4px 2px;
		white-space: pre;
		font: inherit;
		color: #1f2328;
		pointer-events: none;
	}
	.fx-input.hl {
		background: transparent;
		position: relative;
	}
</style>
