// Modèle de la feuille (Sheet.svelte) : ce que la page et le classeur
// échangent avec la grille (données, source, état, TCD, graphiques), les
// types internes partagés par les modules de sheet/, et les lignes d'un TCD.
import { RowCache, type Fetched } from '../cache';
import { type Rect as ClipRect } from '../clipboard';
import { type PasteOptions } from '../pastespecial';
import { type CellStyle } from '../style';
import { type StylePart } from '../borders';
import { type DvList } from '../sheetfmt';
import { type DvRuleData } from '../dvrule';
import { type Zone } from '../findzone';
import { type ReplaceResult, type SearchQuery, type SearchResult } from '../findopts';
import { type SparklineConfig, type SparklineGroup } from '../sparkline';
import type { ErrorBars, ErrorValues, TrendFit, Trendline } from '#lib/chart/fits.js';
import { type CustomList, type CustomOrder } from '../customsort';
import { type KeyChoice } from '../keychoice';
import type { TextColumnsOptions, TextColumnsPreview } from '../textcols';
import { type TabChoice } from '#lib/shell/choice.js';
import { type ChoiceCandidate } from '../tabchoice';
import { type PivotRowInfo } from '../pivottree';
import type { PivotGroup } from '../pivotgroup';
import { type PivotSlicer, type PivotSlicerInfo } from '../pivotslicer';
import { type Citation, type KeyRow } from '../reflink';
import { type ColumnFilter as Filter } from '../filtercond';
import { type ChartKind, type MapGuess } from '#lib/chart/kinds.js';
import type { BoxStatsIn } from '#lib/chart/shapes.js';
import type { RowFormulaPreview } from '#lib/ui/rowformula.js';
import { type ObjectConfig, type SheetObject } from '../objects';
import { type ButtonConfig, type SheetButton } from '../buttons';
import { type Rule, type RuleStats, type StatsReq } from '../rules';
import { type RuleChanges } from '../cfmodel';
import { type SheetOutline } from '../outline';
import { type ColumnType } from '../format';

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
	show?: string; // '', pctTotal, pctRow, pctCol… (voir shows dans pivotfields.ts)
	baseField?: string; // champ de base (% de, différence, cumul, rang…)
	baseItem?: string; // élément de base (% de, différence)
	basePos?: 'prev' | 'next'; // ou l'élément précédent, suivant
	label?: string; // nom personnalisé (« Somme de Ventes » sinon)
	format?: string; // format de nombre imposé (style JSON : nf, dp)
}

export interface PivotConfig {
	name?: string; // « TCD1 » : vue SQL et source de graphique
	source: string;
	rowFields?: string[];
	/** Champs en colonnes, du plus large au plus fin (en-têtes imbriqués). */
	colFields?: string[];
	values?: PivotValue[];
	filters?: { field: string; values: string[] }[];
	/**
	 * Champs calculés (« Champ calculé… ») : une formule sur les colonnes de
	 * chaque ligne de la source ; text en français, formula en anglais
	 * canonique (rendu par le moteur).
	 */
	calc?: { name: string; formula?: string; text?: string }[];
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
	/** Même chose pour les groupes des champs en colonnes. */
	colCollapseAll?: boolean;
	colCollapsed?: (string[] | null)[] | null;
	colExpanded?: (string[] | null)[] | null;
	/** Champs groupés (dates par années…, nombres par intervalles), voir pivotgroup.ts. */
	groups?: PivotGroup[];
	/** Segments et chronologies posés sur la feuille, voir pivotslicer.ts. */
	slicers?: PivotSlicer[];
}

/** Ligne du total général d'un TCD (la dernière), -1 quand il est masqué (« Totaux généraux » décoché ; sans champ en lignes, le total est la seule ligne). */
export function pivotTotalRow(p: { row: number; cells: string[][]; config: PivotConfig }): number {
	return p.config.noGrandTotal && p.config.rowFields?.length ? -1 : p.row + p.cells.length - 1;
}

/** Description de la ligne r d'un TCD en arborescence (undefined : en-tête, ou TCD à plat). */
export function pivotRowOf(p: { row: number; rows?: PivotRowInfo[] | null }, r: number): PivotRowInfo | undefined {
	return r > p.row ? (p.rows?.[r - p.row - 1] ?? undefined) : undefined;
}

/** Ligne d'un TCD en gras : en-tête, total général, groupes et sous-totaux de l'arborescence (comme Excel). */
export function pivotBold(p: { row: number; cells: string[][]; config: PivotConfig; rows?: PivotRowInfo[] | null }, r: number): boolean {
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
	/** Plusieurs champs en colonnes : lignes d'en-tête au-dessus de cells[0], et description de chaque colonne. */
	colHead?: (string[] | null)[] | null;
	cols?: PivotRowInfo[] | null;
	/** Éléments des segments (config.slicers), dans l'ordre. */
	slicers?: PivotSlicerInfo[] | null;
	/** Format de nombre par défaut de chaque colonne (style JSON, voir pivotformat.ts). */
	formats?: string[] | null;
	/** Mise en forme de chaque colonne, de ses lignes et de son en-tête (style JSON, voir pivotformat.ts). */
	styles?: string[] | null;
	headStyles?: string[] | null;
	error: string;
	/** Pas encore calculé (grand tableau source) : la feuille s'affiche d'abord, le TCD arrive ensuite. */
	pending?: boolean;
	/** Colonnes saisies à droite (les dernières de cells, voir pivothand.ts) : nom, formule (« = » compris). */
	hand?: { name: string; formula?: string }[] | null;
	/** Lignes dont les valeurs saisies sont mises de côté ; handNote : pourquoi elles le sont toutes. */
	handKept?: number;
	handNote?: string;
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
	/** Carte de points : facteur de taille des points (absent : 1). */
	mapScale?: number;
	/** Fréquences : nombre de classes (absent : automatique). */
	bins?: number;
	/** Lit aussi les lignes masquées par le filtre du tableau. */
	hidden?: boolean;
	/** Ne lit que les lignes dont la colonne field affiche l'une des valeurs (Regrouper des onglets : Mois = Janvier). */
	filters?: { field: string; values: string[] | null }[] | null;
	/** Courbes : cellule vide en intervalle (défaut), à zéro (zero) ou reliée (span). */
	blanks?: string;
	/** Courbe de tendance et barres d'erreur de chaque série (par rang). */
	trendlines?: (Trendline | null)[] | null;
	errorBars?: (ErrorBars | null)[] | null;
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
	/** Données pas encore calculées (grand tableau) : elles arrivent après l'affichage de la feuille. */
	pending?: boolean;
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
	trends?: (TrendFit | null)[] | null;
	errors?: (ErrorValues | null)[] | null;
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
	/** Suggestion de clé pas encore calculée (vue d'abord) : sa place est gardée dans l'en-tête. */
	keyPending?: boolean;
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
	cells: { row: number; col: number; raw: string; value?: string; error?: boolean; spill?: string[][]; link?: string }[];
	/** Noms des feuilles du classeur (dialogue Lien). */
	sheets?: string[];
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
	/** Images, formes et zones de texte de la feuille, du fond vers le premier plan. */
	objects?: SheetObject[] | null;
	/** Graphiques sparkline (sparkline.ts). */
	sparklines?: SparklineGroup[] | null;
	/** Zones « plage » (tableaux convertis) : y écrire ne refait pas de tableau. */
	ranges?: { r0: number; c0: number; r1: number; c1: number }[];
	/** Plan de la feuille (Données › Grouper), null sans groupe. */
	outline?: SheetOutline | null;
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
	/** Collage spécial d'une zone copiée dans Lattis (valeurs, formules, formats, opération, transposé…). */
	pasteSpecial?(from: number, src: ClipRect, dst: ClipRect, opt: PasteOptions, values: string[][]): Promise<string | void>;
	/** Bloc venu d'ailleurs, avec la mise en forme d'un tableur (JSON par cellule) et les choix du collage spécial. */
	pasteBlock?(dst: ClipRect, block: string[][], styles: string[][] | null, opt: PasteOptions): Promise<string | void>;
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
	/** Largeurs de plusieurs colonnes, en une seule étape d'annulation (ajustement de la sélection). */
	setColumnWidths?(widths: { col: number; width: number }[]): Promise<unknown>;
	/** Hauteur d'une ligne de la feuille (0 : par défaut). */
	setRowHeight?(row: number, height: number): Promise<unknown>;
	/** Hauteurs de plusieurs lignes, en une seule étape d'annulation (0 : par défaut). */
	setRowHeights?(heights: { row: number; height: number }[]): Promise<unknown>;
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
	/** Met en forme plusieurs plages (ou lignes et colonnes entières : line) en une étape d'annulation. */
	applyStyleParts?(parts: (StylePart & { line?: 'rows' | 'cols' })[]): Promise<unknown>;
	/** Note d'une cellule (texte vide : la retire). */
	setNote?(row: number, col: number, text: string): Promise<unknown>;
	/** Lien hypertexte d'une cellule (adresse vide : le retire) ; text non vide : nouveau texte affiché. Une étape. */
	setLink?(row: number, col: number, target: string, text: string): Promise<unknown>;
	/** Destination d'un lien dans le classeur (« #Feuille2!B3 », « #Ventes »). */
	linkPlace?(target: string): Promise<{ sheet: number; r0: number; c0: number; r1: number; c1: number }>;
	/** Épingle la note (toujours affichée) ou la remet en infobulle. */
	pinNote?(row: number, col: number, pinned: boolean): Promise<unknown>;
	/** Fusionne et centre des cellules libres ; renvoie le nombre de valeurs effacées. */
	mergeCells?(r0: number, c0: number, r1: number, c1: number): Promise<number>;
	unmergeCells?(r0: number, c0: number, r1: number, c1: number): Promise<unknown>;
	/** Masque (ou affiche) des lignes (rows) ou des colonnes i0 à i1. */
	setLinesHidden?(rows: boolean, i0: number, i1: number, hide: boolean): Promise<unknown>;
	/** Plan (Données › Grouper) : grouper ou dissocier des lignes (rows) ou des colonnes i0 à i1, effacer le plan. */
	groupLines?(rows: boolean, i0: number, i1: number, group: boolean): Promise<unknown>;
	clearOutline?(): Promise<unknown>;
	/** Réduit (fold) ou rouvre le groupe de niveau level qui contient la ligne line ; affiche le plan jusqu'au niveau level. */
	foldGroup?(rows: boolean, level: number, line: number, fold: boolean): Promise<unknown>;
	showOutlineLevel?(rows: boolean, level: number): Promise<unknown>;
	/** Fige les volets (0, 0 : les libère). */
	setFreeze?(rows: number, cols: number): Promise<unknown>;
	/** Cherche un texte dans la zone (la sélection), ou dans toute la feuille (null). */
	find?(text: string, zone: Zone | null): Promise<{ row: number; col: number }[]>;
	/** Remplace dans la zone (null : partout) ; renvoie le nombre de cellules modifiées. */
	replace?(text: string, by: string, zone: Zone | null): Promise<number>;
	/** Recherche avec options (findopts.ts) : une feuille par appel, la suivante dans le classeur. */
	search?(q: SearchQuery): Promise<SearchResult>;
	/** Remplacer tout avec options, en une étape d'annulation ; le bilan. */
	replaceAll?(q: SearchQuery, by: string): Promise<ReplaceResult>;
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
	/** Validation de la cellule (dialogue) : bornes et formule dans la langue de la session, pour cette cellule. */
	validationAt?(row: number, col: number): Promise<DvRuleData>;
	/** Cellules de la feuille dont la valeur ne respecte pas leur validation (Entourer les données non valides). */
	invalidCells?(): Promise<{ row: number; col: number }[]>;
	/** Colonne de référence (validation tirée de la clé d'un tableau) : la ligne citée par value. */
	referenceRow?(src: string, value: string): Promise<KeyRow>;
	/** Clé d'un tableau : les colonnes de référence qui citent value (lignes, première ligne). */
	citations?(table: string, value: string): Promise<Citation[]>;
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
	/** L'inverse : la ligne juste au-dessus du tableau donne les noms des colonnes, l'en-tête devient la 1re ligne. */
	titleToHeader?(table: SheetTable): Promise<unknown>;
	/** En-têtes sur deux lignes (ou une) : la ligne au-dessus de l'en-tête donne les groupes. */
	setTwoLevel?(table: SheetTable, on: boolean): Promise<unknown>;
	/** Sortir la dernière ligne du tableau : elle reste dessous comme note ou total. */
	rowToNote?(table: SheetTable, n: number): Promise<unknown>;
	/** Ajouter au tableau voisin une cellule gardée hors du tableau (note, total). */
	joinTable?(row: number, col: number): Promise<unknown>;
	/** Tableau croisé sur une nouvelle feuille (« Nouvelle feuille » pendant la pose). */
	createPivot?(cfg: PivotConfig): Promise<unknown>;
	/** Tableau croisé créé à la cellule (row, col) de la feuille affichée. */
	createPivotAt?(row: number, col: number, cfg: PivotConfig): Promise<unknown>;
	/** Valeurs distinctes d'une colonne d'un tableau (filtre d'un TCD). */
	pivotFieldValues?(table: string, field: string): Promise<{ text: string; count: number }[]>;
	/** Valeurs d'un champ d'un TCD de configuration cfg (groupes, champs calculés compris). */
	pivotValues?(cfg: PivotConfig, field: string): Promise<{ text: string; count: number }[]>;
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
	/** Images, formes et zones de texte (objects.ts) : création (rend l'identifiant), présentation, place, rang, suppression. */
	createObject?(row: number, col: number, cfg: ObjectConfig): Promise<unknown>;
	createImage?(row: number, col: number, data: string, cfg: ObjectConfig): Promise<unknown>;
	setObject?(id: number, cfg: ObjectConfig): Promise<unknown>;
	placeObject?(id: number, row: number, col: number, offX: number, offY: number, width: number, height: number): Promise<unknown>;
	placeLine?(id: number, row: number, col: number, offX: number, offY: number, width: number, height: number, flipH: boolean, flipV: boolean): Promise<unknown>;
	orderObject?(id: number, front: boolean): Promise<unknown>;
	deleteObject?(id: number): Promise<unknown>;
	/** Graphiques sparkline : groupe posé, modifié, supprimé. */
	createSparkline?(cfg: SparklineConfig): Promise<unknown>;
	setSparkline?(id: number, cfg: SparklineConfig): Promise<unknown>;
	deleteSparkline?(id: number): Promise<unknown>;
	/** Image du classeur en adresse data:. */
	imageData?(hash: string): Promise<string>;
	setPivot?(id: number, cfg: PivotConfig): Promise<unknown>;
	/** Ajoute (old vide), modifie ou retire (formula vide) un champ calculé ; une erreur est levée telle quelle. */
	setPivotCalc?(id: number, old: string, name: string, formula: string): Promise<unknown>;
	/** Aperçu d'un champ calculé sur le tableau source. */
	previewPivotFormula?(source: string, formula: string): Promise<RowFormulaPreview>;
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
	/** Convertir en valeurs : les colonnes calculées cols deviennent des colonnes saisies (valeurs actuelles). */
	columnsToValues?(table: SheetTable, cols: number[]): Promise<unknown>;
	/** Convertir en valeurs : formules de la plage (cellules libres, colonnes calculées touchées). */
	rangeToValues?(r0: number, c0: number, r1: number, c1: number): Promise<unknown>;
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
	/** Remplace le résultat déversé de la formule en (row, col) par un tableau : lié (recalculé) ou de valeurs. */
	convertSpill?(row: number, col: number, linked: boolean): Promise<unknown>;
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
	/** La cellule active a un fil de commentaires (Révision › Répondre). */
	comment?: boolean;
	/** Tableau de la cellule active : filtré, et sa colonne n'affiche que les erreurs. */
	filters?: boolean;
	errorsOnly?: boolean;
	/** Tableau publié en lecture seule sous la cellule active (classeur partagé) : le motif. */
	readOnly?: string;
}

export interface Entry {
	t: SheetTable;
	cache: RowCache;
	last: number; // dernière ligne de la feuille occupée
	lastCol: number;
}

/** Ce qu'un volet affiche : lignes et colonnes visibles, positions à l'écran (Sheet.svelte, layer). */
export interface Layer {
	w: number;
	h: number;
	top: number;
	r0: number;
	r1: number;
	c0: number;
	c1: number;
	cx: (c: number) => number;
	ry: (r: number) => number;
	xEnd: number;
	spanW: (a: number, b: number) => number;
	visible: Entry[];
	s0: number;
	s1: number;
	k0: number;
	k1: number;
}

export type Box = { x: number; y: number; w: number; h: number };
export type TableAnim = { dx: number; dy: number; ox: number; w: number; h: number; alpha: number; rowOff: (r: number) => number; colOff: (c: number) => number };
export type TextField = HTMLInputElement | HTMLTextAreaElement;
export type HiddenMark = { rows: boolean; i0: number; i1: number; x: number; y: number; w: number; h: number };
export type Link = { row: number; col: number; pre: DepTarget[]; dep: DepTarget[] };
export type Range = { r0: number; r1: number; c0: number; c1: number };
