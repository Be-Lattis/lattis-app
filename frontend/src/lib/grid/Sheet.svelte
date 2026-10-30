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
	import { onMount, tick, untrack } from 'svelte';
	import { noticeTtl } from '#lib/shell/notice.js';
	import { VScroll } from './scroll';
	import { Axis } from './axis';
	import { isDefaultTableName } from './defaultname';
	import { ease, reducedMotion, type AxisLike, type BarSpan, type Geo, type LinesShift, type Motion } from './anim';
	import { watchSource, WORK_DELAY, type WorkLookup, type WorkZone } from './working';
	import { overlaps, parseTSV, pasteArea, richClip, sameClipText, tileBlock, toTSV, type Clip, type Rect as ClipRect } from './clipboard';
	import { clipValues, hasClipStyles, parseClipHtml, toClipHtml, type ClipStyle } from './clipboardHtml';
	import { pasteOptions, plainPaste, type PasteKind, type PasteOptions } from './pastespecial';
	import PasteSpecial from './PasteSpecial.svelte';
	import { boxStyle, isNumericText, painterPatch, parseStyle, resolveStyle, type CellStyle, type StyleLayer } from './style';
	import { bumpDecimals, formatCell } from './numfmt';
	import { plain } from './numcode';
	import { BorderSet, borderParts, type BorderCmd, type Edge, type StylePart } from './borders';
	import { cellFont } from './textwrap';
	import { AutoRows } from './rowfit';
	import { Velocity } from './touch';
	import { PanGesture, SpaceHold } from './pan';
	import { panes, toGrid, type Pane } from './panes';
	import { afterWidthChange, sideRoom } from './keepinview';
	import { mergeAt, validate, type DvList, type MergeRange } from './sheetfmt';
	import { type DvAlert, type DvForm } from './dvrule';
	import { expandRange, isSingleCell, rangeOf, stepCell, wholeLines, type CellRange } from './mergesel';
	import { findZone, type Zone } from './findzone';
	import { type FindOpts, type FoundCell } from './findopts';
	import { blocksReach, boxesReach } from './objextent';
	import FormatDialog, { type FormatTab } from './FormatDialog.svelte';
	import TextToColumns from './TextToColumns.svelte';
	import CustomSort from './CustomSort.svelte';
	import LinkDialog from './LinkDialog.svelte';
	import CommentThread from './CommentThread.svelte';
	import { threadAt, threadPlace, type CommentHost } from './comments';
	import SparklineDialog from './SparklineDialog.svelte';
	import { type SparklineConfig, type SparklineGroup } from './sparkline';
	import { autoLink, isLinkClick, linkKind, onLinkText, openLink } from './hyperlink';
	import { type CustomOrder } from './customsort';
	import { type KeyChoice, type KeyQuestion } from './keychoice';
	import { dismissPanel } from './dismiss';
	import { freeDisplay } from './freecell';
	import { type ChoiceCandidate } from './tabchoice';
	import { ReadRecovery } from './readfail';
	import { bandMerges, groupSpans, headerLabel, isSingle, tableTop } from './twolevel';
	import { tableUnderTitle, titleAboveFilled } from './titlehead';
	import { pivotColLayer, pivotLayer, pivotValueText } from './pivotformat';
	import { pivotLabelOffset } from './pivottree';
	import SheetSlicers from './SheetSlicers.svelte';
	import { type Citation } from './reflink';
	import { gridPalette } from './theme';
	import { tableTheme } from './theme.svelte.js';
	import { type CondOp, type PanelValue } from './filtercond';
	import { SheetViews } from './sheetview';
	import { clip } from '#lib/ui/menu.js';
	import Chart from '#lib/chart/Chart.svelte';
	import { encodeObj, objFromText, rememberObj } from '#lib/clip/objects.js';
	import ChartPanel from '#lib/chart/ChartPanel.svelte';
	import type { PrintCell, PrintChart, PrintSource } from '#lib/print/sheetprint.js';
	import PivotPanel from './PivotPanel.svelte';
	import { handAlign, handKeptNote, handStart } from './pivothand';
	import { type Placement } from './pivotplace';
	import SheetButtons from './SheetButtons.svelte';
	import SheetObjects from './SheetObjects.svelte';
	import { objectBox, type ObjectConfig, type SheetObject } from './objects';
	import { type SheetButton } from './buttons';
	import { todaySerial, type Rule, type Stats } from './rules';
	import { type RuleChanges } from './cfmodel';
	import CondFormat, { type CondHost, type CondView } from './CondFormat.svelte';
	import GridA11y from './GridA11y.svelte';
	import { type CellSpeech } from './a11y';
	import { signatureParts, suggest, type Catalog, type Completion, type FunctionDoc } from './autocomplete';
	import { formulaRefs, highlightParts, isFormula, previewShift, type FormulaError, type Rect } from './formulaEdit';
	import { editorLeave, type FormulaHold } from './crossref';
	import OutlineBar from './OutlineBar.svelte';
	import { bandSize, outlineDepth, type SheetOutline } from './outline';
	import { nameAlertOpen } from '#lib/ui/nameAlert.js';
	import { isErrorValue, valueAlignment, alignment, editValue, NULL_MARK, showsNull, typeLabel, type ColumnType } from './format';
	import { errorText, t as tr } from '#lib/i18n/index.js'; // t est souvent un tableau ici : la traduction s'appelle tr
	import { type TypeMix, type PivotConfig, type SheetPivot, type ChartConfig, type SheetChart, type PageBreaksView, type SheetTable, type SheetData, type SheetSource, type SheetStatus, type Box, type TableAnim, type TextField, type HiddenMark, type Link, type Range, type Entry, pivotTotalRow, pivotRowOf, pivotBold, type WholeSel, type DvPick, type InputTip } from './sheet/model';
	import { SHEET_ROWS, SHEET_COLS, ROW_H, DEF_W, TOP_H, MIN_MARGIN, FONT, FONT_BOLD } from './sheet/consts';
	import { createFind } from './sheet/find.svelte.js';
	import { createScrolling } from './sheet/scrolling.svelte.js';
	import { createCharts } from './sheet/charts.svelte.js';
	import { createFilters } from './sheet/filters.svelte.js';
	import { createPivot } from './sheet/pivot.svelte.js';
	import { createDataTools } from './sheet/datatools.svelte.js';
	import { createCommands } from './sheet/commands.svelte.js';
	import { createAudit } from './sheet/audit.svelte.js';
	import { createEdit } from './sheet/edit.svelte.js';
	import { createKeyboard } from './sheet/keyboard.svelte.js';
	import { createMouse } from './sheet/mouse.svelte.js';
	import { createMenus } from './sheet/menus.svelte.js';
	import { createNotes } from './sheet/notes.svelte.js';
	import { createValidation } from './sheet/validation.svelte.js';
	import { createOutline } from './sheet/outline.svelte.js';
	import { createStatus } from './sheet/status.svelte.js';
	import { createBench } from './sheet/bench.svelte.js';
	import { createAnim } from './sheet/anim.svelte.js';
	import { createFormula } from './sheet/formula.svelte.js';
	import { createMeasure } from './sheet/measure.svelte.js';
	import { createCondFormat } from './sheet/condformat.svelte.js';
	import FindBar, { type FindBarView } from './sheet/FindBar.svelte';
	import CellMenus, { type CellMenusView } from './sheet/CellMenus.svelte';
	import FilterPanel, { type FilterPanelView } from './sheet/FilterPanel.svelte';
	import ValidationDialog, { type ValidationDialogView } from './sheet/ValidationDialog.svelte';
	import FunctionsDialog, { type FunctionsDialogView } from './sheet/FunctionsDialog.svelte';
	import DedupDialog, { type DedupDialogView } from './sheet/DedupDialog.svelte';
	import ChartMenu, { type ChartMenuView } from './sheet/ChartMenu.svelte';
	import NoteEditor, { type NoteEditorView } from './sheet/NoteEditor.svelte';
	import LinesDialog, { type LinesDialogView } from './sheet/LinesDialog.svelte';
	import SizeDialog, { type SizeDialogView } from './sheet/SizeDialog.svelte';
	import DvAlertBox, { type DvAlertBoxView } from './sheet/DvAlertBox.svelte';
	import ListMenu, { type ListMenuView } from './sheet/ListMenu.svelte';
	import PivotFilter, { type PivotFilterView } from './sheet/PivotFilter.svelte';
	import KeyDialog, { type KeyDialogView } from './sheet/KeyDialog.svelte';
	import HandDialog, { type HandDialogView } from './sheet/HandDialog.svelte';
	import KeyHintMenu, { type KeyHintMenuView } from './sheet/KeyHintMenu.svelte';
	import FollowMenu, { type FollowMenuView } from './sheet/FollowMenu.svelte';
	import { createDraw } from './sheet/draw.svelte.js';
	import { createPivotFold } from './sheet/pivotfold.svelte.js';
	import { createSparklines } from './sheet/sparklines.svelte.js';
	import { createBadges } from './sheet/badges.svelte.js';
	import { createData } from './sheet/data.svelte.js';

	interface Props {
		data: SheetData;
		source: SheetSource;
		/** Fonctions et tableaux du classeur, pour l'aide à la saisie des formules. */
		catalog?: Catalog;
		/** Tableaux et tableaux croisés de toutes les feuilles (sources de graphiques et de TCD). */
		sources?: { name: string; columns: string[]; types: string[]; kind?: string; key?: number }[];
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
		/** Structure protégée (Révision › Protéger la structure, ou le classeur) : seule
		 * la saisie dans les tableaux ; ni colonne de plus, ni graphique ou contrôle à
		 * sélectionner, déplacer ou redimensionner. */
		structure?: boolean;
		/** Classeur collaboratif ouvert par un éditeur ou un lecteur : cellules libres et
		 * mise en page réservées aux concepteurs (décision 0018) ; la saisie hors d'un
		 * tableau est refusée dès la première touche avec ce message (vide : permise). */
		layoutLocked?: string;
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
		/**
		 * « Basculer sur » (colonne de référence) ou « Lignes qui citent » :
		 * aller à la ligne id du tableau (filter : filtrer d'abord sa colonne
		 * sur la valeur), en notant l'endroit d'où l'on part pour Alt+←.
		 */
		onreflink?: (to: { table: string; id: number; key?: string; filter?: { column: string; value: string } }, from: { row: number; col: number; label: string }) => void;
		/** Alt+← : revenir à l'endroit d'où l'on a basculé. */
		onrefback?: () => void;
		/** Partager le tableau au Data Lake Lattis (clic droit sur un tableau). */
		onsharedata?: (table: string) => void;
		/** Classeur partagé en direct : clic droit sur un tableau qui n'est que sur ce poste › « Ajouter au classeur partagé ». */
		onaddshared?: (table: string) => void;
		/** Tableaux publiés seuls comme source du Data Lake (sans le classeur). */
		sourceTables?: readonly string[];
		/** Clic droit › « Mettre à jour la source du Data Lake » d'un de ces tableaux. */
		onpushsource?: (table: string) => void;
		/** Sauts de page de l'impression (traits pointillés) et zone imprimée ; absent : rien. */
		pageBreaks?: PageBreaksView;
		/** Pose d'un tableau croisé en cours (pivotplace.ts) : indication de la barre d'état, ou null. */
		onplacing?: (p: { hint: string; create: boolean } | null) => void;
		/** Commentaires en fil de la feuille (coin violet, fil au survol, Ctrl+Alt+M) ; absent : aucun. */
		comments?: CommentHost;
	}

	let { data, source: rawSource, catalog, sources = [], onstatus, autoBench, gridlines = true, headings = false, onhscroll, locked = '', structure = false, layoutLocked = '', freeze, zoom = 1, onzoom, animate = true, sheetName = '', onsheet, onrowlink, onreflink, onrefback, onsharedata, onaddshared, sourceTables = [], onpushsource, pageBreaks, onplacing, comments }: Props = $props();

	// Actions sur des cellules, lignes ou colonnes : leur zone reçoit un
	// reflet animé jusqu'à leur fin (working.ts, drawWorking).
	const workLook: WorkLookup = {
		table(t) {
			const e = typeof t === 'string' ? entries.find((x) => x.t.name === t) : entries.find((x) => x.t.id === (t as SheetTable | undefined)?.id);
			return e && { row: e.t.row, col: e.t.col, last: e.last, lastCol: e.lastCol };
		}
	};
	const source = $derived(watchSource(rawSource, workLook, startWork, endWork));

	// Gouttière de gauche : voir sheet/consts.ts.
	const keyWidths = new Map<string, number>(); // tableau → clé la plus large vue
	let extentRows = 1;
	let keyGrew = false; // une clé plus large a été dessinée pendant cette image

	// Accès des modules de sheet/ et des composants posés sur la grille à l'état
	// de la feuille : un seul objet, des accesseurs (lus au moment de l'appel).
	// Chaque module déclare dans son interface (XHost, XView) ce qu'il en lit.
	const sheetCtx = {
		get a1() { return a1; },
		get acceptKeyHint() { return acceptKeyHint; },
		get active() { return active; }, set active(v) { active = v; },
		get activeRect() { return activeRect; },
		get activeStyle() { return activeStyle; },
		get activeThread() { return activeThread; },
		get addrA1() { return addrA1; },
		get anchor() { return anchor; }, set anchor(v) { anchor = v; },
		get animBusy() { return animBusy; }, set animBusy(v) { animBusy = v; },
		get animNow() { return animNow; }, set animNow(v) { animNow = v; },
		get answerDvAlert() { return answerDvAlert; },
		get applyDedup() { return applyDedup; },
		get applyFilter() { return applyFilter; },
		get applyHandDialog() { return applyHandDialog; },
		get applyKeyDialog() { return applyKeyDialog; },
		get applyLinesDialog() { return applyLinesDialog; },
		get applyPivotFilter() { return applyPivotFilter; },
		get applySize() { return applySize; },
		get applyStyle() { return applyStyle; },
		get applyValidation() { return applyValidation; },
		get asTableHint() { return asTableHint; },
		get AUTO_NUMBER_TIP() { return AUTO_NUMBER_TIP; },
		get autoFitCols() { return autoFitCols; },
		get autoFitColumns() { return autoFitColumns; },
		get autoFitRows() { return autoFitRows; },
		get autoLinkAfter() { return autoLinkAfter; },
		get autoRows() { return autoRows; },
		get autoSum() { return autoSum; },
		get bandStyle() { return bandStyle; },
		get barAnims() { return barAnims; },
		get barShown() { return barShown; },
		get barsNow() { return barsNow; }, set barsNow(v) { barsNow = v; },
		get barsPrev() { return barsPrev; }, set barsPrev(v) { barsPrev = v; },
		get beginAnimFrame() { return beginAnimFrame; },
		get bench() { return bench; }, set bench(v) { bench = v; },
		get borders() { return borders; },
		get borderSet() { return borderSet; },
		get boxes() { return boxes; }, set boxes(v) { boxes = v; },
		get boxOf() { return boxOf; },
		get buttons() { return buttons; }, set buttons(v) { buttons = v; },
		get buttonsLayer() { return buttonsLayer; },
		get C() { return C; },
		get calcEdit() { return calcEdit; },
		get cancelHandDialog() { return cancelHandDialog; },
		get cancelKeyDialog() { return cancelKeyDialog; },
		get catalog() { return catalog; },
		get cax() { return cax; },
		get cellAddr() { return cellAddr; },
		get cellAt() { return cellAt; },
		get cellInfo() { return cellInfo; },
		get cellName() { return cellName; },
		get cellRules() { return cellRules; }, set cellRules(v) { cellRules = v; },
		get cells() { return cells; }, set cells(v) { cells = v; },
		get cellWidth() { return cellWidth; },
		get cfAt() { return cfAt; },
		get cfGen() { return cfGen; }, set cfGen(v) { cfGen = v; },
		get cfStale() { return cfStale; },
		get cfStats() { return cfStats; }, set cfStats(v) { cfStats = v; },
		get cfToday() { return cfToday; }, set cfToday(v) { cfToday = v; },
		get cfView() { return cfView; }, set cfView(v) { cfView = v; },
		get cfWaits() { return cfWaits; },
		get CHART_H() { return CHART_H; },
		get CHART_W() { return CHART_W; },
		get chartActive() { return chartActive; },
		get chartBox() { return chartBox; },
		get chartClip() { return chartClip; }, set chartClip(v) { chartClip = v; },
		get chartMenu() { return chartMenu; }, set chartMenu(v) { chartMenu = v; },
		get charts() { return charts; }, set charts(v) { charts = v; },
		get chartTween() { return chartTween; }, set chartTween(v) { chartTween = v; },
		get choiceCands() { return choiceCands; }, set choiceCands(v) { choiceCands = v; },
		get chooseKey() { return chooseKey; },
		get chooseType() { return chooseType; },
		get clearMessage() { return clearMessage; },
		get clearSelection() { return clearSelection; },
		get clipRange() { return clipRange; }, set clipRange(v) { clipRange = v; },
		get closeFilter() { return closeFilter; },
		get closeFinder() { return closeFinder; },
		get closeMenus() { return closeMenus; },
		get colAtX() { return colAtX; },
		get colAxis() { return colAxis; }, set colAxis(v) { colAxis = v; },
		get colDraw() { return colDraw; }, set colDraw(v) { colDraw = v; },
		get colRules() { return colRules; }, set colRules(v) { colRules = v; },
		get colStyles() { return colStyles; }, set colStyles(v) { colStyles = v; },
		get columnFlash() { return columnFlash; }, set columnFlash(v) { columnFlash = v; },
		get columnFlashes() { return columnFlashes; },
		get commentDlg() { return commentDlg; }, set commentDlg(v) { commentDlg = v; },
		get comments() { return comments; },
		get commit() { return commit; },
		get commitEditor() { return commitEditor; },
		get completion() { return completion; }, set completion(v) { completion = v; },
		get completionKey() { return completionKey; },
		get CONFIRM_DELETE_ROWS() { return CONFIRM_DELETE_ROWS; },
		get controlsTick() { return controlsTick; }, set controlsTick(v) { controlsTick = v; },
		get copyChart() { return copyChart; },
		get copyChartImage() { return copyChartImage; },
		get copyFill() { return copyFill; },
		get copySelection() { return copySelection; },
		get ctx() { return ctx; },
		get curPane() { return curPane; }, set curPane(v) { curPane = v; },
		get customDlg() { return customDlg; }, set customDlg(v) { customDlg = v; },
		get data() { return data; },
		get dedupDlg() { return dedupDlg; }, set dedupDlg(v) { dedupDlg = v; },
		get deleteNote() { return deleteNote; },
		get deleteTable() { return deleteTable; },
		get deleteWholeColumns() { return deleteWholeColumns; },
		get designWhy() { return designWhy; },
		get dismissFilter() { return dismissFilter; },
		get dragging() { return dragging; }, set dragging(v) { dragging = v; },
		get draw() { return draw; },
		get drawCommentMarks() { return drawCommentMarks; },
		get drawing() { return drawing; }, set drawing(v) { drawing = v; },
		get drawTimes() { return drawTimes; },
		get drawValidation() { return drawValidation; },
		get dropButton() { return dropButton; },
		get dvAfterData() { return dvAfterData; },
		get dvAlert() { return dvAlert; }, set dvAlert(v) { dvAlert = v; },
		get dvAt() { return dvAt; },
		get dvGen() { return dvGen; }, set dvGen(v) { dvGen = v; },
		get dvLists() { return dvLists; },
		get dvPick() { return dvPick; }, set dvPick(v) { dvPick = v; },
		get dvPickRef() { return dvPickRef; }, set dvPickRef(v) { dvPickRef = v; },
		get EDIT_WINDOW() { return EDIT_WINDOW; },
		get editActive() { return editActive; },
		get editAt() { return editAt; }, set editAt(v) { editAt = v; },
		get editByTyping() { return editByTyping; }, set editByTyping(v) { editByTyping = v; },
		get editChart() { return editChart; },
		get editErr() { return editErr; }, set editErr(v) { editErr = v; },
		get editing() { return editing; }, set editing(v) { editing = v; },
		get editLink() { return editLink; },
		get editNote() { return editNote; },
		get editorH() { return editorH; }, set editorH(v) { editorH = v; },
		get editRO() { return editRO; }, set editRO(v) { editRO = v; },
		get editText() { return editText; },
		get editValueText() { return editValueText; }, set editValueText(v) { editValueText = v; },
		get endAnimFrame() { return endAnimFrame; },
		get endPick() { return endPick; },
		get ensureVisible() { return ensureVisible; },
		get enterMove() { return enterMove; },
		get enterTable() { return enterTable; },
		get entries() { return entries; }, set entries(v) { entries = v; },
		get extentCols() { return extentCols; },
		get extentRows() { return extentRows; },
		get fCols() { return fCols; },
		get fetchError() { return fetchError; }, set fetchError(v) { fetchError = v; },
		get fillHandle() { return fillHandle; },
		get filling() { return filling; }, set filling(v) { filling = v; },
		get filterClosed() { return filterClosed; }, set filterClosed(v) { filterClosed = v; },
		get filterDlg() { return filterDlg; }, set filterDlg(v) { filterDlg = v; },
		get filterShown() { return filterShown; },
		get findArrived() { return findArrived; },
		get findChanged() { return findChanged; },
		get findCross() { return findCross; }, set findCross(v) { findCross = v; },
		get finder() { return finder; }, set finder(v) { finder = v; },
		get findInput() { return findInput; }, set findInput(v) { findInput = v; },
		get findNext() { return findNext; },
		get finishHold() { return finishHold; },
		get finishPick() { return finishPick; },
		get fitText() { return fitText; },
		get fitVisibleRows() { return fitVisibleRows; },
		get flashes() { return flashes; },
		get flashOf() { return flashOf; },
		get fnDlg() { return fnDlg; }, set fnDlg(v) { fnDlg = v; },
		get focusFirst() { return focusFirst; },
		get foldPivot() { return foldPivot; },
		get followBadgeAt() { return followBadgeAt; },
		get followBadges() { return followBadges; }, set followBadges(v) { followBadges = v; },
		get followLink() { return followLink; },
		get followMenu() { return followMenu; }, set followMenu(v) { followMenu = v; },
		get followRef() { return followRef; },
		get formatDlg() { return formatDlg; }, set formatDlg(v) { formatDlg = v; },
		get formulaMode() { return formulaMode; },
		get formulasInSelection() { return formulasInSelection; },
		get foundRect() { return foundRect; },
		get frame() { return frame; }, set frame(v) { frame = v; },
		get frameNow() { return frameNow; },
		get frameStamps() { return frameStamps; },
		get freeCf() { return freeCf; },
		get freeLinks() { return freeLinks; }, set freeLinks(v) { freeLinks = v; },
		get freeMark() { return freeMark; },
		get freeMarks() { return freeMarks; }, set freeMarks(v) { freeMarks = v; },
		get freeMenu() { return freeMenu; }, set freeMenu(v) { freeMenu = v; },
		get freeStyles() { return freeStyles; }, set freeStyles(v) { freeStyles = v; },
		get fRows() { return fRows; },
		get frozenHeaderAt() { return frozenHeaderAt; },
		get fxEditing() { return fxEditing; }, set fxEditing(v) { fxEditing = v; },
		get fxInput() { return fxInput; },
		get fxLock() { return fxLock; },
		get fxRO() { return fxRO; }, set fxRO(v) { fxRO = v; },
		get fxText() { return fxText; }, set fxText(v) { fxText = v; },
		get geoDrawn() { return geoDrawn; }, set geoDrawn(v) { geoDrawn = v; },
		get goFound() { return goFound; },
		get gridlines() { return gridlines; },
		get groupBandAt() { return groupBandAt; },
		get groupCandidate() { return groupCandidate; },
		get GROW_MS() { return GROW_MS; },
		get growAtEnd() { return growAtEnd; },
		get grown() { return grown; },
		get grownCharts() { return grownCharts; }, set grownCharts(v) { grownCharts = v; },
		get grownSeen() { return grownSeen; }, set grownSeen(v) { grownSeen = v; },
		get handDlg() { return handDlg; }, set handDlg(v) { handDlg = v; },
		get hasKeyHint() { return hasKeyHint; },
		get HEAD_ICON_W() { return HEAD_ICON_W; },
		get headerAt() { return headerAt; },
		get headerButton() { return headerButton; },
		get headerDrag() { return headerDrag; }, set headerDrag(v) { headerDrag = v; },
		get headerWidth() { return headerWidth; },
		get headIconW() { return headIconW; },
		get headStyles() { return headStyles; }, set headStyles(v) { headStyles = v; },
		get hiddenCols() { return hiddenCols; }, set hiddenCols(v) { hiddenCols = v; },
		get hiddenIn() { return hiddenIn; },
		get hiddenMark() { return hiddenMark; },
		get hiddenMarkAt() { return hiddenMarkAt; },
		get hiddenMarks() { return hiddenMarks; }, set hiddenMarks(v) { hiddenMarks = v; },
		get hiddenRows() { return hiddenRows; }, set hiddenRows(v) { hiddenRows = v; },
		get hiddenRuns() { return hiddenRuns; },
		get hideLines() { return hideLines; },
		get hintLines() { return hintLines; },
		get hintMenu() { return hintMenu; }, set hintMenu(v) { hintMenu = v; },
		get hold() { return hold; }, set hold(v) { hold = v; },
		get holdCols() { return holdCols; },
		get holdRows() { return holdRows; }, set holdRows(v) { holdRows = v; },
		get holdWhileScrolling() { return holdWhileScrolling; },
		get hoverLocked() { return hoverLocked; }, set hoverLocked(v) { hoverLocked = v; },
		get inner() { return inner; },
		get input() { return input; },
		get inputTip() { return inputTip; }, set inputTip(v) { inputTip = v; },
		get inRangeZone() { return inRangeZone; },
		get inSelection() { return inSelection; },
		get inTable() { return inTable; },
		get isNumeric() { return isNumeric; },
		get isSplit() { return isSplit; },
		get JUMP_FLASH_MS() { return JUMP_FLASH_MS; },
		get jumpFlash() { return jumpFlash; },
		get keepEditorUntil() { return keepEditorUntil; }, set keepEditorUntil(v) { keepEditorUntil = v; },
		get keepFormula() { return keepFormula; },
		get keptOut() { return keptOut; },
		get key() { return key; },
		get KEY_HINT_TIP() { return KEY_HINT_TIP; },
		get keyContextMenu() { return keyContextMenu; },
		get keyDlg() { return keyDlg; }, set keyDlg(v) { keyDlg = v; },
		get keyGrew() { return keyGrew; }, set keyGrew(v) { keyGrew = v; },
		get keyHintAt() { return keyHintAt; },
		get keyHintBoxes() { return keyHintBoxes; }, set keyHintBoxes(v) { keyHintBoxes = v; },
		get keyWidths() { return keyWidths; },
		get lastCol() { return lastCol; },
		get lastEditMs() { return lastEditMs; }, set lastEditMs(v) { lastEditMs = v; },
		get lastRef() { return lastRef; }, set lastRef(v) { lastRef = v; },
		get lastRepeat() { return lastRepeat; }, set lastRepeat(v) { lastRepeat = v; },
		get lastRow() { return lastRow; },
		get layoutLocked() { return layoutLocked; },
		get lines() { return lines; }, set lines(v) { lines = v; },
		get linesDlg() { return linesDlg; }, set linesDlg(v) { linesDlg = v; },
		get linesHint() { return linesHint; }, set linesHint(v) { linesHint = v; },
		get linesOpts() { return linesOpts; },
		get linkButton() { return linkButton; },
		get linkGo() { return linkGo; }, set linkGo(v) { linkGo = v; },
		get linkOf() { return linkOf; },
		get links() { return links; }, set links(v) { links = v; },
		get linkUnder() { return linkUnder; },
		get listAll() { return listAll; },
		get listMenu() { return listMenu; }, set listMenu(v) { listMenu = v; },
		get listOf() { return listOf; },
		get loadPivotValues() { return loadPivotValues; },
		get loadRefItems() { return loadRefItems; },
		get localBadgeAt() { return localBadgeAt; },
		get locked() { return locked; },
		get lockedRowAt() { return lockedRowAt; },
		get MARGIN() { return MARGIN; }, set MARGIN(v) { MARGIN = v; },
		get matches() { return matches; }, set matches(v) { matches = v; },
		get matchesKey() { return matchesKey; },
		get matchKey() { return matchKey; }, set matchKey(v) { matchKey = v; },
		get MAX_AUTO_H() { return MAX_AUTO_H; },
		get MAX_FIT() { return MAX_FIT; },
		get measureHeights() { return measureHeights; },
		get measurer() { return measurer; }, set measurer(v) { measurer = v; },
		get menu() { return menu; }, set menu(v) { menu = v; },
		get menuAction() { return menuAction; },
		get mergedAt() { return mergedAt; },
		get mergeOf() { return mergeOf; },
		get merges() { return merges; }, set merges(v) { merges = v; },
		get message() { return message; }, set message(v) { message = v; },
		get messageTimer() { return messageTimer; }, set messageTimer(v) { messageTimer = v; },
		get missingFrames() { return missingFrames; }, set missingFrames(v) { missingFrames = v; },
		get mixes() { return mixes; },
		get mixOf() { return mixOf; },
		get mixText() { return mixText; },
		get motion() { return motion; },
		get motions() { return motions; },
		get mounted() { return mounted; },
		get moveTo() { return moveTo; },
		get moving() { return moving; }, set moving(v) { moving = v; },
		get ms() { return ms; },
		get nameBadgeAt() { return nameBadgeAt; },
		get nameBox() { return nameBox; }, set nameBox(v) { nameBox = v; },
		get nameInput() { return nameInput; },
		get names() { return names; }, set names(v) { names = v; },
		get NAV_SETTLE() { return NAV_SETTLE; },
		get navAt() { return navAt; }, set navAt(v) { navAt = v; },
		get newComment() { return newComment; },
		get noAnimUntil() { return noAnimUntil; }, set noAnimUntil(v) { noAnimUntil = v; },
		get noMark() { return noMark; },
		get noOff() { return noOff; },
		get NOT_YOURS() { return NOT_YOURS; },
		get noteDlg() { return noteDlg; }, set noteDlg(v) { noteDlg = v; },
		get noteOf() { return noteOf; },
		get noteTip() { return noteTip; }, set noteTip(v) { noteTip = v; },
		get nudgeChart() { return nudgeChart; },
		get objectsLayer() { return objectsLayer; },
		get olH() { return olH; },
		get olW() { return olW; },
		get onaddshared() { return onaddshared; },
		get onChartMenu() { return onChartMenu; },
		get onContextMenu() { return onContextMenu; },
		get onDblClick() { return onDblClick; },
		get onFinderKey() { return onFinderKey; },
		get onFormulaInput() { return onFormulaInput; },
		get onplacing() { return onplacing; },
		get onpushsource() { return onpushsource; },
		get onrefback() { return onrefback; },
		get onreflink() { return onreflink; },
		get onrowlink() { return onrowlink; },
		get onsharedata() { return onsharedata; },
		get onsheet() { return onsheet; },
		get onstatus() { return onstatus; },
		get onzoom() { return onzoom; },
		get openCitation() { return openCitation; },
		get openCustomSort() { return openCustomSort; },
		get openFilter() { return openFilter; },
		get openFinder() { return openFinder; },
		get openFormat() { return openFormat; },
		get openFunctions() { return openFunctions; },
		get openHandKey() { return openHandKey; },
		get openHintMenu() { return openHintMenu; },
		get openLinesDialog() { return openLinesDialog; },
		get openList() { return openList; },
		get openPivotCustomSort() { return openPivotCustomSort; },
		get openPivotMenu() { return openPivotMenu; },
		get openPivotOnLoad() { return openPivotOnLoad; }, set openPivotOnLoad(v) { openPivotOnLoad = v; },
		get openRules() { return openRules; },
		get openSize() { return openSize; },
		get openTypeMenu() { return openTypeMenu; },
		get openValidation() { return openValidation; },
		get outline() { return outline; }, set outline(v) { outline = v; },
		get outlineAction() { return outlineAction; },
		get outlineBar() { return outlineBar; },
		get ownerAt() { return ownerAt; },
		get pageBreaks() { return pageBreaks; },
		get painter() { return painter; }, set painter(v) { painter = v; },
		get paintFlash() { return paintFlash; },
		get pan() { return pan; },
		get panCursor() { return panCursor; }, set panCursor(v) { panCursor = v; },
		get panDown() { return panDown; },
		get panel() { return panel; }, set panel(v) { panel = v; },
		get paneTop() { return paneTop; }, set paneTop(v) { paneTop = v; },
		get pasteChart() { return pasteChart; },
		get pasteClipboard() { return pasteClipboard; },
		get pasteSpecial() { return pasteSpecial; },
		get pickedRef() { return pickedRef; },
		get pickFromList() { return pickFromList; },
		get pickPivotSource() { return pickPivotSource; },
		get pinnedFrame() { return pinnedFrame; }, set pinnedFrame(v) { pinnedFrame = v; },
		get pinnedShown() { return pinnedShown; }, set pinnedShown(v) { pinnedShown = v; },
		get PIVOT_DROP() { return PIVOT_DROP; },
		get PIVOT_PARTIAL() { return PIVOT_PARTIAL; },
		get PIVOT_READONLY() { return PIVOT_READONLY; },
		get pivotActive() { return pivotActive; },
		get pivotAt() { return pivotAt; },
		get pivotBadgeAt() { return pivotBadgeAt; },
		get pivotBadges() { return pivotBadges; }, set pivotBadges(v) { pivotBadges = v; },
		get pivotDlg() { return pivotDlg; }, set pivotDlg(v) { pivotDlg = v; },
		get pivotFiltered() { return pivotFiltered; },
		get pivotFilterText() { return pivotFilterText; },
		get pivotHead() { return pivotHead; },
		get pivotHeld() { return pivotHeld; },
		get pivotIn() { return pivotIn; },
		get pivotPanelAt() { return pivotPanelAt; }, set pivotPanelAt(v) { pivotPanelAt = v; },
		get pivots() { return pivots; }, set pivots(v) { pivots = v; },
		get pivotSourcePick() { return pivotSourcePick; }, set pivotSourcePick(v) { pivotSourcePick = v; },
		get pivotSources() { return pivotSources; },
		get pivotValuesOf() { return pivotValuesOf; },
		get pivotZone() { return pivotZone; },
		get placed() { return placed; },
		get placedSheet() { return placedSheet; }, set placedSheet(v) { placedSheet = v; },
		get placeEditor() { return placeEditor; },
		get placeHold() { return placeHold; },
		get placeHover() { return placeHover; }, set placeHover(v) { placeHover = v; },
		get placePivotAt() { return placePivotAt; },
		get placeSize() { return placeSize; }, set placeSize(v) { placeSize = v; },
		get placing() { return placing; }, set placing(v) { placing = v; },
		get prefetchDv() { return prefetchDv; },
		get pressLink() { return pressLink; },
		get previewDedup() { return previewDedup; },
		get previewTimer() { return previewTimer; }, set previewTimer(v) { previewTimer = v; },
		get pushStat() { return pushStat; },
		get QUERY_READONLY() { return QUERY_READONLY; },
		get queryAt() { return queryAt; },
		get queryIn() { return queryIn; },
		get rangeValues() { return rangeValues; },
		get rax() { return rax; },
		get readOnlyReason() { return readOnlyReason; },
		get rebuildAxis() { return rebuildAxis; },
		get rebuildRows() { return rebuildRows; },
		get recovery() { return recovery; },
		get refAt() { return refAt; },
		get refContext() { return refContext; },
		get refDrag() { return refDrag; }, set refDrag(v) { refDrag = v; },
		get refItems() { return refItems; }, set refItems(v) { refItems = v; },
		get refMove() { return refMove; }, set refMove(v) { refMove = v; },
		get refreshFx() { return refreshFx; },
		get refreshPreview() { return refreshPreview; },
		get refreshStatus() { return refreshStatus; },
		get refTables() { return refTables; },
		get refusal() { return refusal; },
		get removeChart() { return removeChart; },
		get removeLink() { return removeLink; },
		get removePivot() { return removePivot; },
		get renameFromBadge() { return renameFromBadge; },
		get reopenFormula() { return reopenFormula; },
		get replaceAll() { return replaceAll; },
		get replaceInput() { return replaceInput; }, set replaceInput(v) { replaceInput = v; },
		get replaceOne() { return replaceOne; },
		get resetRowHeights() { return resetRowHeights; },
		get resizeCanvas() { return resizeCanvas; },
		get resizing() { return resizing; }, set resizing(v) { resizing = v; },
		get rh() { return rh; },
		get rowAtY() { return rowAtY; },
		get rowAxis() { return rowAxis; }, set rowAxis(v) { rowAxis = v; },
		get rowDraw() { return rowDraw; }, set rowDraw(v) { rowDraw = v; },
		get rowLabel() { return rowLabel; },
		get rowResizing() { return rowResizing; }, set rowResizing(v) { rowResizing = v; },
		get rowTop() { return rowTop; },
		get rspan() { return rspan; },
		get rulesActive() { return rulesActive; },
		get runTabChoice() { return runTabChoice; },
		get saveNote() { return saveNote; },
		get say() { return say; },
		get schedule() { return schedule; },
		get scopeTints() { return scopeTints; },
		get screenX() { return screenX; },
		get screenY() { return screenY; },
		get scroller() { return scroller; },
		get searchZone() { return searchZone; },
		get selBox() { return selBox; }, set selBox(v) { selBox = v; },
		get selChart() { return selChart; }, set selChart(v) { selChart = v; },
		get selectChart() { return selectChart; },
		get selection() { return selection; },
		get selectionLines() { return selectionLines; },
		get selectPlace() { return selectPlace; },
		get selectRange() { return selectRange; },
		get selectRows() { return selectRows; },
		get setHidden() { return setHidden; },
		get sheetColRules() { return sheetColRules; }, set sheetColRules(v) { sheetColRules = v; },
		get sheetColStyles() { return sheetColStyles; }, set sheetColStyles(v) { sheetColStyles = v; },
		get sheetId() { return sheetId; },
		get sheetIdOf() { return sheetIdOf; },
		get sheetLines() { return sheetLines; },
		get sheetName() { return sheetName; },
		get sheetObjects() { return sheetObjects; }, set sheetObjects(v) { sheetObjects = v; },
		get sheetRowRules() { return sheetRowRules; }, set sheetRowRules(v) { sheetRowRules = v; },
		get sheetRowStyles() { return sheetRowStyles; }, set sheetRowStyles(v) { sheetRowStyles = v; },
		get sheetViews() { return sheetViews; },
		get shown() { return shown; }, set shown(v) { shown = v; },
		get shownFunctions() { return shownFunctions; },
		get shownMessage() { return shownMessage; }, set shownMessage(v) { shownMessage = v; },
		get shownText() { return shownText; },
		get shownValue() { return shownValue; },
		get sig() { return sig; }, set sig(v) { sig = v; },
		get singleCol() { return singleCol; },
		get singleIcons() { return singleIcons; },
		get singleSelection() { return singleSelection; },
		get sizeDlg() { return sizeDlg; }, set sizeDlg(v) { sizeDlg = v; },
		get snapMerge() { return snapMerge; },
		get sortPivot() { return sortPivot; },
		get source() { return source; },
		get sourceColumns() { return sourceColumns; },
		get sources() { return sources; },
		get sourceTables() { return sourceTables; },
		get space() { return space; },
		get spaceKey() { return spaceKey; },
		get spacer() { return spacer; },
		get sparkAt() { return sparkAt; },
		get sparkDlg() { return sparkDlg; }, set sparkDlg(v) { sparkDlg = v; },
		get sparkGroups() { return sparkGroups; }, set sparkGroups(v) { sparkGroups = v; },
		get speech() { return speech; }, set speech(v) { speech = v; },
		get speechSel() { return speechSel; }, set speechSel(v) { speechSel = v; },
		get spilledAt() { return spilledAt; },
		get spills() { return spills; }, set spills(v) { spills = v; },
		get spillUnder() { return spillUnder; },
		get startAnims() { return startAnims; },
		get startCustom() { return startCustom; },
		get startEdit() { return startEdit; },
		get startFormula() { return startFormula; },
		get startPick() { return startPick; },
		get startPivotMove() { return startPivotMove; },
		get statsEpoch() { return statsEpoch; },
		get step() { return step; },
		get structure() { return structure; },
		get styleAt() { return styleAt; },
		get styleHiddenCols() { return styleHiddenCols; }, set styleHiddenCols(v) { styleHiddenCols = v; },
		get styleHiddenRows() { return styleHiddenRows; }, set styleHiddenRows(v) { styleHiddenRows = v; },
		get suggestPos() { return suggestPos; }, set suggestPos(v) { suggestPos = v; },
		get syncFinderZone() { return syncFinderZone; },
		get syncNative() { return syncNative; },
		get tabChoiceFor() { return tabChoiceFor; },
		get tableAnim() { return tableAnim; },
		get tableAnims() { return tableAnims; }, set tableAnims(v) { tableAnims = v; },
		get tableAnimsAt() { return tableAnimsAt; }, set tableAnimsAt(v) { tableAnimsAt = v; },
		get tableAt() { return tableAt; },
		get tableBox() { return tableBox; }, set tableBox(v) { tableBox = v; },
		get tableGeo() { return tableGeo; },
		get tableHintAt() { return tableHintAt; },
		get tableHintBox() { return tableHintBox; }, set tableHintBox(v) { tableHintBox = v; },
		get tableLines() { return tableLines; },
		get tableNameEl() { return tableNameEl; },
		get tabMove() { return tabMove; },
		get tabPress() { return tabPress; },
		get targetCol() { return targetCol; },
		get targetTable() { return targetTable; },
		get textWidth() { return textWidth; },
		get tipCell() { return tipCell; }, set tipCell(v) { tipCell = v; },
		get titleAbove() { return titleAbove; },
		get titleTableAt() { return titleTableAt; },
		get toggleFilters() { return toggleFilters; },
		get toggleFollow() { return toggleFollow; },
		get togglePin() { return togglePin; },
		get togglePivotFold() { return togglePivotFold; },
		get toggleReplace() { return toggleReplace; },
		get toLocal() { return toLocal; },
		get topRow() { return topRow; },
		get touch() { return touch; }, set touch(v) { touch = v; },
		get toValuesActive() { return toValuesActive; },
		get toView() { return toView; },
		get ttcDlg() { return ttcDlg; }, set ttcDlg(v) { ttcDlg = v; },
		get typed() { return typed; }, set typed(v) { typed = v; },
		get TYPES() { return TYPES; },
		get unhide() { return unhide; },
		get updateCommentTip() { return updateCommentTip; },
		get updateExtent() { return updateExtent; },
		get updateHeaderTip() { return updateHeaderTip; },
		get updateNoteTip() { return updateNoteTip; },
		get userHeights() { return userHeights; }, set userHeights(v) { userHeights = v; },
		get userWidths() { return userWidths; }, set userWidths(v) { userWidths = v; },
		get validDlg() { return validDlg; }, set validDlg(v) { validDlg = v; },
		get valsNow() { return valsNow; }, set valsNow(v) { valsNow = v; },
		get valsPrev() { return valsPrev; }, set valsPrev(v) { valsPrev = v; },
		get valueColumns() { return valueColumns; },
		get viewH() { return viewH; },
		get viewNow() { return viewNow; }, set viewNow(v) { viewNow = v; },
		get viewPos() { return viewPos; }, set viewPos(v) { viewPos = v; },
		get viewPrev() { return viewPrev; }, set viewPrev(v) { viewPrev = v; },
		get viewW() { return viewW; },
		get visibleData() { return visibleData; },
		get visibleRows() { return visibleRows; },
		get vs() { return vs; },
		get wholeKind() { return wholeKind; },
		get wholeSel() { return wholeSel; }, set wholeSel(v) { wholeSel = v; },
		get working() { return working; },
		get wrap() { return wrap; },
		get write() { return write; },
		get writing() { return writing; }, set writing(v) { writing = v; },
		get zoom() { return zoom; },
		get zoomK() { return zoomK; },
		get zoomWheel() { return zoomWheel; }, set zoomWheel(v) { zoomWheel = v; }
	};

	// --- Dessin de la grille : sheet/draw.svelte.ts ---
	const drawCtl = createDraw(sheetCtx);
	const { draw, sparkAt, drawCells, singleCell, pivotHead, rowLabel, codeShown, drawText, drawWorking } = drawCtl;

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
		// Couleurs des tableaux : celles du thème (Affichage › Thème, theme.ts),
		// remplacées par gridPalette ; ici « Vert et bleu ».
		head: '#e3efe7',
		// Tableaux chargés par une requête (source externe) : bleu clair, même contraste que le vert.
		linkHead: '#e3ecf7',
		linkHeadText: '#14325a',
		linkBorder: '#8fa9cc',
		linkIcon: '#6b7f99',
		// Pastille d'un tableau qui suit l'onglet à choix (nuance foncée du thème).
		followBadge: '#217346',
		// Pastille du nom d'un tableau : nommé (couleur du thème, plus claire), ou nom par défaut (gris, à renommer).
		nameBadge: '#2f7d55',
		unnamedBadge: '#8a9099',
		// Pastille du nom d'un tableau croisé (TCD1) : couleur des sources externes.
		sourceBadge: '#3b6fb6',
		// Pastille « sur ce poste » : tableau d'un classeur collaboratif pas
		// (encore) dans le classeur partagé (ambre : à remarquer).
		localBadge: '#a15c07',
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
		// Liens hypertexte (hyperlink.ts) : le bleu des liens d'un tableur.
		link: '#0563c1',
		spill: '#5b9bd5',
		found: 'rgba(250, 204, 21, 0.30)',
		// Cellule trouvée (Ctrl+F) : sa ligne et sa colonne entières, teintées ; plus marqué dans les gouttières.
		findCross: 'rgba(250, 204, 21, 0.16)',
		findGutter: 'rgba(250, 204, 21, 0.55)',
		frozen: '#9aa0a6',
		// Tiret d'une case NULL de tableau (format.ts, NULL_MARK) : discret.
		nullMark: '#b4b9bf',
		note: '#c0392b',
		// Classeur collaboratif : modification en attente du serveur, conflit.
		pending: '#7f93b2',
		conflict: '#d97706'
	};

	// --- Tableaux posés sur la feuille ------------------------------------

	let entries: Entry[] = [];
	let cells = new Map<string, string>();
	let names = new Map<string, string>(); // position → nom
	// Mise en forme, en couches (voir style.ts) : colonnes et lignes entières
	// de la feuille, colonnes et en-têtes des tableaux, cellules.
	let freeStyles = new Map<string, StyleLayer>(); // position → style (hors tableaux : cellules libres, TCD)
	// Cellules libres =LIEN_HYPERTEXTE(…) : position → adresse calculée.
	let freeLinks = new Map<string, string>();
	/** Couche de style d'un lien calculé (il passe devant le lien posé). */
	const lnLayer = (url: string | undefined): StyleLayer | undefined => (url ? { ln: url } : undefined);
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
	// Statistiques demandées au classeur (impression : attendues avant d'écrire les pages).
	let cfWaits: Promise<unknown>[] | null = null;
	// --- Mise en forme conditionnelle : sheet/condformat.svelte.ts ---
	const cfCtl = createCondFormat(sheetCtx);
	const { cfAt, freeCf, sheetRules, selectionLocs, openRules, closeRules } = cfCtl;
	export const rulesMenu = cfCtl.rulesMenu;
	export const rulesActive = cfCtl.rulesActive;
	export const rulesTarget = cfCtl.rulesTarget;

	const noMark = () => false;
	/** Règle « formule » d'une cellule libre vérifiée (calculée par le classeur). */
	const freeMark = (row: number, col: number) => (r: Rule) => freeMarks.has(`${row},${col},${r.id ?? r.value}`);

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
	let sheetObjects = $state.raw<SheetObject[]>([]);
	let sparkGroups: SparklineGroup[] = [];
	let objectsLayer = $state<ReturnType<typeof SheetObjects>>();
	// Décalage de la vue, pour placer les graphiques au-dessus de la grille.
	let viewPos = $state({ x: 0, y: 0 });
	let colAxis = $state.raw(new Axis(SHEET_COLS, DEF_W));
	const key = (r: number, c: number) => `${r},${c}`;

	// Dernière erreur de lecture d'un tableau signalée (une fois par mise en page).
	let fetchError = '';
	// Tableaux rechargés après une lecture refusée (une fois par période).
	const recovery = new ReadRecovery();

	// --- Données de la feuille : sheet/data.svelte.ts ---
	const dataCtl = createData(sheetCtx);
	const { applyData } = dataCtl;
	// Feuille des tableaux affichés (entries) : leurs positions avant une mise à jour de la même feuille.
	let placedSheet: number | undefined;
	const placed = () => entries.map((e) => ({ id: e.t.id, r0: e.t.row, c0: e.t.col, r1: e.last, c1: e.lastCol }));

	const sheetViews = new SheetViews();

	// Largeurs : choisies par l'utilisateur, sinon selon le type de la colonne
	// du premier tableau qui l'occupe, sinon la largeur par défaut.
	let userWidths = new Map<number, number>();
	const MAX_FIT = 400;
	let measurer: CanvasRenderingContext2D | null = null;
	// --- Mesure du texte : sheet/measure.svelte.ts ---
	const measureCtl = createMeasure(sheetCtx);
	const { textWidth, cellWidth, headerWidth, rebuildAxis, measureHeights, measureHeader, measureBand, fitVisibleRows } = measureCtl;

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
	// Lignes masquées par l'utilisateur ; hiddenRows y ajoute celles des
	// groupes réduits du plan (outline.ts), sans repère : leur bouton [+] est
	// dans la barre du plan.
	let styleHiddenRows = new Set<number>();
	let outline = $state.raw<SheetOutline | null>(null);
	let outlineBar = $state<ReturnType<typeof OutlineBar>>();
	// Hauteurs automatiques (voir rowfit.ts) : cellules libres à chaque mise en
	// page, lignes de tableau à mesure qu'elles s'affichent.
	const autoRows = new AutoRows();
	const MAX_AUTO_H = 400;

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

	// --- Animations : sheet/anim.svelte.ts ---
	const animCtl = createAnim(sheetCtx);
	const { beginAnimFrame, endAnimFrame, startAnims, hintLines, tableGeo, tableAnim, enterTable, barShown, columnFlashes, flashOf, paintFlash, activeRect, grown } = animCtl;
	// Cadre prolongé par la cellule active (juste dessous ou à droite), ou
	// rendu quand elle s'en va, sans que le tableau change : il s'étire ou se
	// rétracte en douceur, depuis le cadre dessiné à l'image précédente.
	const GROW_MS = 160;
	let grownSeen = new Map<number, string>();

	// Ouverture d'une feuille : les mises en page qui suivent de près (données
	// relues, tableaux, graphiques) appartiennent encore à la navigation, et
	// la feuille s'affiche d'un coup, sans tableaux qui se déplient.
	const NAV_SETTLE = 1000;
	let navAt = -Infinity;

	// ox : bord gauche du cadre animé par rapport à sa place finale (cadre qui
	// s'étire vers la gauche, contenu immobile).
	const noOff = () => 0;
	let tableAnims = new Map<number, TableAnim | null>();
	let tableAnimsAt = -1;

	// Formule de colonne validée : toutes ses cellules flashent, même si leur
	// valeur ne change pas (on voit que toute la colonne a été modifiée). Les
	// lignes chargées ensuite flashent à leur premier affichage.
	let columnFlash: { table: number; name: string; t: number; done: Set<string> } | null = null;

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
	const NOT_YOURS = $derived(tr('grid.readOnly.notYours'));
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
		const boxes = [...charts.map(chartBox), ...buttons.map(buttonBox), ...sheetObjects.map((o) => objectBox(o, objGeo))];
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
		// navigateur ramène la barre en arrière : la feuille sautait. Mesuré
		// jusqu'au bord de la fenêtre : volets de la feuille et de l'application.
		const side = wrap ? sideRoom(document.documentElement.clientWidth, wrap.getBoundingClientRect().right, zoomK()) : 0;
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
		/** Tableau croisé emporté en entier (sélection prise dans un seul TCD) : sel est sa zone. */
		pivot?: number;
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

	// --- Saisie de formule : sheet/formula.svelte.ts ---
	const formulaCtl = createFormula(sheetCtx);
	const { onFormulaInput, moveSig, completionKey, keepFormula, placeHold, finishHold } = formulaCtl;
	export const resumeFormula = formulaCtl.resumeFormula;

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

	// Jusqu'à cet instant, l'éditeur de cellule qui perd le focus le reprend (retour de pointage).
	let keepEditorUntil = 0;

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

	// Thème de couleur (Affichage › Thème) : couleurs des tableaux, redessinées tout de suite.
	$effect(() => {
		Object.assign(C, gridPalette(tableTheme.current));
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
		if (!e) return resolveStyle(pivotLayer(pivots, r, c), col, row, pivotColLayer(pivots, r, c), freeStyles.get(key(r, c)), lnLayer(freeLinks.get(key(r, c))));
		const i = c - e.t.col;
		if (r === e.t.row) return resolveStyle(col, row, headStyles.get(e.t.id)?.[i]);
		const line = e.cache.row(r - e.t.row - 1);
		return resolveStyle(col, row, colStyles.get(e.t.id)?.[i], parseStyle(line?.sty?.get(i)), lnLayer(line?.links?.get(i)));
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

	// --- Sparklines : sheet/sparklines.svelte.ts ---
	const sparklinesCtl = createSparklines(sheetCtx);
	const { printSpark, saveSparkline, removeSparkline } = sparklinesCtl;
	export const insertSparkline = sparklinesCtl.insertSparkline;
	// Insertion › Sparklines : dialogue (groupe neuf ou celui de la cellule active).
	let sparkDlg = $state<{ config: SparklineConfig; columns: string[]; id?: number } | null>(null);

	/** Largeur des boutons de l'en-tête des colonnes sur un niveau (posée par drawCells, lue par singleCell). */
	const singleIcons = new Map<string, number>();

	// Bouton ▾ d'un TCD (filtre de rapport, étiquette des lignes) : largeur réservée à droite de la cellule.
	const PIVOT_DROP = 18;

	// --- Arborescence des TCD : sheet/pivotfold.svelte.ts ---
	const pivotFoldCtl = createPivotFold(sheetCtx);
	const { addSlicer, pivotValuesOf, togglePivotFold, foldPivot, pivotFilterText, pivotFiltered } = pivotFoldCtl;

	// --- Clé proposée -------------------------------------------------------
	// Quand la première colonne d'un tableau sans clé a des valeurs uniques
	// et remplies, une petite clé grisée dans son en-tête propose d'en faire
	// la clé, en un clic ; clic droit : « Ne plus proposer » pour ce tableau.
	// La clé n'est posée d'office qu'à la création, sur une première colonne
	// ID ou 1, 2, 3… (canvas/autokey.go, décision 0010).

	/** Infobulle d'une colonne numérotée d'office. */
	const AUTO_NUMBER_TIP = $derived(tr('grid.key.autoNumber'));

	const KEY_HINT_TIP = $derived(tr('grid.key.hint'));
	/** Zones (écran) des clés proposées dessinées à cette image. */
	let keyHintBoxes: { e: Entry; x0: number; x1: number; y0: number; y1: number }[] = [];
	let hintMenu = $state<{ x: number; y: number; e: Entry } | null>(null);

	// --- Pastille « suit l'année » (§6.5) ------------------------------------
	// Un tableau qui suit l'onglet à choix porte, au coin de son cadre, une
	// pastille avec la valeur choisie (« 2025 ») ; un tableau sans pastille est
	// le même pour toutes les valeurs. Un clic sur la pastille ouvre son menu
	// (« Suivre l'année », à décocher).
	let followBadges: { kind: 'follow' | 'name' | 'unnamed' | 'local'; e: Entry; label: string; radii: number[]; x0: number; x1: number; y0: number; y1: number; clip: { x0: number; x1: number; y0: number; y1: number } }[] = [];
	/** Pastilles du nom des tableaux croisés (TCD1) : un clic ouvre leur volet. */
	let pivotBadges: { p: SheetPivot; label: string; radii: number[]; x0: number; x1: number; y0: number; y1: number; clip: { x0: number; x1: number; y0: number; y1: number } }[] = [];
	// Champ du nom du tableau (barre de formule) : la pastille du nom l'ouvre.
	let tableNameEl = $state<HTMLInputElement>();
	/** Volet en cours de dessin (découpe des pastilles). */
	let curPane: Pane | null = null;

	let followMenu = $state<{ x: number; y: number; e: Entry } | null>(null);

	// --- Pastilles des tableaux : sheet/badges.svelte.ts ---
	const badgesCtl = createBadges(sheetCtx);
	const { followBadgeAt, nameBadgeAt, pivotBadgeAt, localBadgeAt, fitText, renameFromBadge, toggleFollow, keyHintAt, acceptKeyHint, openHintMenu } = badgesCtl;

	const hasKeyHint = (e: Entry, i: number) => i === 0 && !!e.t.keyHint && e.t.key === undefined && !!source.setKey;

	// Bordures des cellules : relevées pendant le dessin des cellules, tracées
	// après le quadrillage, sur ses lignes (deux cellules voisines partagent
	// le même trait), pour qu'il ne les recouvre pas (borders.ts : style et
	// couleur de chaque bord).
	const borderSet = new BorderSet();

	/** Bordures d'une cellule (style bt, bb, bl, br). */
	function borders(st: CellStyle, x: number, y: number, w: number, h: number) {
		borderSet.add(st, x, y, w, h);
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
		if ((p.dp === 'more' || p.dp === 'less') && activeStyle().nc) {
			// Code personnalisé : une décimale de plus ou de moins dans le code.
			p.nc = bumpDecimals(activeStyle().nc!, p.dp === 'more' ? 1 : -1);
			delete p.dp;
		}
		if (p.dp === 'more' || p.dp === 'less') {
			// Comme Excel : part des décimales affichées dans la cellule active
			// (pas de la valeur stockée ni, pour une colonne calculée, de la formule).
			const cur = activeStyle().dp ?? decimalsOf(shownText(active.row, active.col));
			p.dp = Math.max(0, Math.min(10, cur + (p.dp === 'more' ? 1 : -1)));
		}
		const { r0, r1, c0, c1 } = selection();
		const whole = wholeKind();
		const pen = (p.pen as Edge | undefined) ?? { style: 'thin' };
		delete p.pen;
		const cmd = p.border as BorderCmd | undefined;
		delete p.border;
		// Bordures : des morceaux de la sélection (Contour : ses quatre bords),
		// avec le reste du correctif, en une étape d'annulation.
		const parts: (StylePart & { line?: 'rows' | 'cols' })[] = cmd ? borderParts(cmd, pen, r0, c0, r1, c1) : [];
		const w = whole && source.applyLineStyle ? whole : null;
		if (w) {
			// Lignes ou colonnes entières : la mise en forme vaut pour toutes
			// leurs cellules, présentes et à venir (comme Excel) ; un bord de
			// gauche ou de droite (de lignes), du haut ou du bas (de colonnes)
			// est sans objet.
			const rows = w === 'rows';
			for (const x of parts) x.line = rows ? 'rows' : 'cols';
			const side = rows ? ['bl', 'br'] : ['bt', 'bb'];
			const keep = parts.filter((x) => !Object.keys(x.patch).every((k) => side.includes(k)));
			parts.length = 0;
			parts.push(...keep);
		}
		if (Object.keys(p).length) parts.unshift({ r0, c0, r1, c1, patch: p, line: w ? (w === 'rows' ? 'rows' : 'cols') : undefined });
		if (!parts.length) return;
		menuAction(() => applyParts(parts));
	}

	/** Pose des morceaux de mise en forme, en une étape quand la source le permet. */
	async function applyParts(parts: (StylePart & { line?: 'rows' | 'cols' })[]) {
		if (parts.length > 1 && source.applyStyleParts) return source.applyStyleParts(parts);
		for (const x of parts) {
			if (x.line && source.applyLineStyle) await source.applyLineStyle(x.line === 'rows', x.line === 'rows' ? x.r0 : x.c0, x.line === 'rows' ? x.r1 : x.c1, x.patch);
			else if (!x.line) await source.applyStyle!(x.r0, x.c0, x.r1, x.c1, x.patch);
		}
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
		say(tr('grid.painter.hint'));
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
		if (Object.keys(patch).length) applyStyle(patch);
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
			say(tr('grid.merge.selectSeveral'));
			return;
		}
		const band = bandSelection();
		if (!freeSelection() && !band) {
			say(tr('grid.merge.inTable'));
			return;
		}
		writing = writing.catch(() => undefined).then(async () => {
			try {
				const n = await mergeCells(r0, c0, r1, c1);
				moveTo(r0, c0);
				// Bande des groupes : le moteur dit ce qui a été renommé.
				message = band ? '' : n ? tr('grid.merge.done.cleared', { n }) : tr('grid.merge.done');
			} catch (err) {
				linesHint = null; // refusée : rien ne glisse
				message = errorText(err);
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
			const names = ones.map((o) => tr('grid.quoted', { text: o.e.t.columns[o.c - o.e.t.col].name })).join(', ');
			message = tr('grid.twoLevel.split', { names });
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

	// --- Plan : sheet/outline.svelte.ts ---

	const olW = $derived(bandSize(outlineDepth(outline?.rows)));
	const olH = $derived(bandSize(outlineDepth(outline?.cols)));
	// Hauteur de ligne / largeur de colonne (clic droit), comme Excel.
	let sizeDlg = $state<{ rows: boolean; value: number; x: number; y: number } | null>(null);
	const outlineCtl = createOutline(sheetCtx);
	const { outlineGeom, foldOutline, outlineLevel, hiddenIn, unhide, openSize, applySize, autoFitRows, resetRowHeights, autoFitCols } = outlineCtl;
	export const outlineAction = outlineCtl.outlineAction;
	export const hideLines = outlineCtl.hideLines;
	export const freezeActive = outlineCtl.freezeActive;

	// --- Validation des données : liste déroulante ------------------------------

	// --- Validation des données et colonnes de référence : sheet/validation.svelte.ts ---
	// Listes tirées d'une plage, relues sur le serveur (par source) : vidées à
	// chaque relecture de la feuille, relues à l'ouverture de la liste et
	// quand la cellule active en a une (la saisie est vérifiée avec elles).
	const dvLists = new Map<string, DvList>();
	let dvGen = 0;
	// Icône « Mettre sous forme de tableau » d'un îlot de cellules libres.
	let tableHintBox: { x: number; y: number; w: number; h: number } | null = null;
	let listMenu = $state<{ x: number; y: number; values: string[]; row: number; col: number; error?: string; loading?: boolean } | null>(null);
	// Entrées du clic droit : « Basculer sur » (cellule d'une référence),
	// « Lignes qui citent » (cellule de la clé d'un tableau), relues à
	// l'ouverture du menu.
	let refItems = $state<{
		row: number;
		col: number;
		go?: { table: string; value: string; found?: boolean };
		cite?: { table: string; value: string; list: Citation[] | null };
	} | null>(null);
	// Dialogue : valeurs saisies (text) ou plage source (src), aperçu des
	// valeurs de la plage (relu à chaque changement de la référence).
	// Trois onglets (dvrule.ts) : Options, Message de saisie, Alerte d'erreur.
	let validDlg = $state<(DvForm & { target: string; error: string; preview: DvList | null; loading: boolean }) | null>(null);
	let previewTimer: ReturnType<typeof setTimeout> | undefined;
	// « Sélectionner » : le dialogue se retire, la plage source se désigne
	// dans la grille (clic, glissé, Maj+flèches, clic sur l'en-tête d'une
	// colonne de tableau), au besoin sur une autre feuille (clic sur son
	// onglet). Entrée la reprend dans le dialogue, Échap revient sans la
	// changer ; dans les deux cas la feuille de la cellule et la sélection
	// d'avant (la cible de la validation) sont rétablies. back : retour en
	// cours vers la feuille d'origine, ref : la référence à reprendre.
	let dvPick = $state<DvPick | null>(null);
	let dvPickRef = $state('');

	// Saisie refusée par le moteur (dvrules.go) : Arrêt (Réessayer, Annuler),
	// Avertissement (Continuer ? Oui, Non, Annuler), Informations (OK, Annuler).
	let dvAlert = $state<(DvAlert & { row: number; col: number; raw: string; keep: boolean }) | null>(null);
	// Message de saisie de la cellule active, relevé en dessinant.
	let inputTip = $state<InputTip | null>(null);
	const validationCtl = createValidation(sheetCtx);
	const { dvAt, listOf, prefetchDv, dropButton, asTableHint, tableHintAt, openList, pickFromList, refAt, followRef, loadRefItems, openCitation, linkButton, refreshPreview, applyValidation, startPick, pickedRef, endPick, finishPick, refusal, answerDvAlert, dvAfterData, drawValidation } = validationCtl;
	export const openValidation = validationCtl.openValidation;
	export const circleInvalid = validationCtl.circleInvalid;
	export const clearCircles = validationCtl.clearCircles;

	// --- Colonnes de référence (décision 0016) ---------------------------------

	// Ligne atteinte par « Basculer sur » ou « Revenir » : éclat bref.
	const JUMP_FLASH_MS = 1200;
	let jumpFlash: { r: number; c0: number; c1: number; t: number } | null = null;
	let jumpTimer: ReturnType<typeof setTimeout> | undefined;

	/** Éclaire brièvement les cellules c0 à c1 de la ligne r (la ligne atteinte). */
	export function flashRow(r: number, c0: number, c1: number) {
		jumpFlash = { r, c0, c1, t: performance.now() };
		clearTimeout(jumpTimer);
		// Sans animation : l'éclat reste puis s'efface d'un coup.
		jumpTimer = setTimeout(() => {
			jumpFlash = null;
			schedule();
		}, JUMP_FLASH_MS);
		schedule();
	}

	// Zones en cours de traitement (working.ts) : reflet bleu qui passe, contour en pointillés qui défile.
	let working: { id: number; zones: WorkZone[]; t0: number }[] = [];
	let workSeq = 0;

	function startWork(zones: WorkZone[]): number {
		const id = ++workSeq;
		working = [...working, { id, zones, t0: performance.now() }];
		// Le reflet paraît après WORK_DELAY : une image à ce moment-là, même sans animation.
		setTimeout(schedule, WORK_DELAY + 10);
		return id;
	}

	function endWork(id: number) {
		working = working.filter((w) => w.id !== id);
		schedule();
	}

	// --- Validation des données : alerte, message de saisie, cercles ---------------

	// --- Notes et commentaires : sheet/notes.svelte.ts ---

	let noteTip = $state<{ x: number; y: number; text: string; key: string } | null>(null);
	let noteDlg = $state<{ row: number; col: number; text: string; had: boolean; pinned: boolean; x: number; y: number } | null>(null);
	// Notes épinglées visibles : relevées en dessinant, affichées en post-it.
	let pinnedFrame: { x: number; y: number; text: string }[] = [];
	let pinnedShown = $state<{ x: number; y: number; text: string }[]>([]);

	// Fil ouvert (preview : aperçu au survol, en lecture seule).
	let commentDlg = $state<{ row: number; col: number; x: number; y: number; preview: boolean } | null>(null);
	const notesCtl = createNotes(sheetCtx);
	const { noteOf, updateNoteTip, saveNote, togglePin, drawCommentMarks, updateCommentTip } = notesCtl;
	export const editNote = notesCtl.editNote;
	export const editActive = notesCtl.editActive;
	export const deleteNote = notesCtl.deleteNote;
	export const newComment = notesCtl.newComment;
	export const deleteComment = notesCtl.deleteComment;
	export const activeThread = notesCtl.activeThread;
	export const showComment = notesCtl.showComment;

	// --- Liens hypertexte (hyperlink.ts) --------------------------------------------

	let linkDlg = $state<{ row: number; col: number; text: string; locked: boolean; target: string; names: string[] } | null>(null);
	// Clic sur le texte d'un lien : suivi au relâcher s'il est court (maintenu : sélection seule).
	let linkPress: { row: number; col: number; t: number; x: number; y: number; url: string } | null = null;
	// Lien suivi vers une autre feuille : sa destination, sélectionnée à l'arrivée (applyData).
	let linkGo: { sheet: number; r0: number; c0: number; r1: number; c1: number } | null = null;

	/** Lien de la cellule (fusion : celui de la cellule en haut à gauche), s'il est permis. */
	function linkOf(r: number, c: number): string | undefined {
		const g = mergeOf(r, c);
		const ln = (g ? styleAt(g.row, g.col, undefined) : styleAt(r, c))?.ln;
		return ln && linkKind(ln) ? ln : undefined;
	}

	/** Le lien sous la souris, quand elle est sur le texte de sa cellule (à côté : la cellule se sélectionne). */
	function linkUnder(e: MouseEvent): { row: number; col: number; url: string } | null {
		const cell = cellAt(e);
		if (!cell || editing) return null;
		const g = mergeOf(cell.row, cell.col);
		const r = g?.row ?? cell.row;
		const c = g?.col ?? cell.col;
		const url = linkOf(r, c);
		const text = url ? shownValue(r, c) : '';
		if (!url || !text) return null;
		const st = styleAt(r, c);
		ctx.save();
		ctx.font = st && (st.ff || st.fs) ? cellFont(st) : (st?.i ? 'italic ' : '') + (st?.b ? FONT_BOLD : FONT);
		const tw = Math.max(...text.split('\n').map((l) => ctx.measureText(l).width));
		ctx.restore();
		const cw = cax().offset(c + (g?.cols ?? 1)) - cax().offset(c);
		const align = st?.al ?? (isNumeric(text) ? 'right' : 'left');
		return onLinkText(toLocal(e).x, screenX(c), cw, tw, align) ? { row: r, col: c, url } : null;
	}

	/** Suit un lien : page web ou e-mail hors de l'application, endroit du classeur sélectionné. */
	async function followLink(url: string) {
		if (linkKind(url) !== 'place') {
			if (!openLink(url)) say(tr('grid.link.refused'));
			return;
		}
		if (!source.linkPlace) return;
		try {
			const p = await source.linkPlace(url);
			if (p.sheet !== sheetId() && onsheet) {
				linkGo = p;
				onsheet(p.sheet);
				return;
			}
			selectPlace(p);
		} catch (err) {
			say(errorText(err));
		}
	}

	function selectPlace(p: { r0: number; c0: number; r1: number; c1: number }) {
		wrap.focus();
		if (p.r0 === p.r1 && p.c0 === p.c1) {
			moveTo(p.r0, p.c0);
			ensureVisible(p.r0, p.c0);
		} else selectRange(p.r0, p.c0, p.r1, p.c1);
	}

	/** Bouton enfoncé sur le texte d'un lien : retenu pour le relâcher (Ctrl+clic : suivi tout de suite). */
	function pressLink(e: MouseEvent): boolean {
		linkPress = null;
		if (e.button !== 0 || e.shiftKey || formulaMode || hold) return false;
		const l = linkUnder(e);
		if (!l) return false;
		if (e.ctrlKey || e.metaKey) {
			void followLink(l.url);
			return true;
		}
		linkPress = { row: l.row, col: l.col, t: performance.now(), x: e.clientX, y: e.clientY, url: l.url };
		return false;
	}

	/** Bouton relâché : un clic court sur le même lien le suit. */
	function releaseLink(e?: MouseEvent) {
		const p = linkPress;
		linkPress = null;
		if (!p || !e) return;
		const at = cellAt(e);
		const g = at ? mergeOf(at.row, at.col) : undefined;
		if (isLinkClick(p, performance.now(), g ? { row: g.row, col: g.col } : at, e.clientX, e.clientY)) void followLink(p.url);
	}

	/** Insertion › Lien (Ctrl+K) : le dialogue sur la cellule active. */
	export function editLink() {
		if (!source.setLink) return;
		if (locked) {
			say(locked);
			return;
		}
		closeMenus();
		const g = mergeOf(active.row, active.col);
		const row = g?.row ?? active.row;
		const col = g?.col ?? active.col;
		const raw = editText(row, col);
		const info = cellInfo(row, col);
		const formula = raw.startsWith('=') || (info.kind === 'data' && info.e.t.columns[info.i].formula !== undefined);
		const own = (g ? styleAt(row, col, undefined) : styleAt(row, col))?.ln ?? '';
		// Destinations nommées : tableaux du classeur et cellules nommées de la feuille.
		const named = [...new Set([...sources.filter((s) => s.kind !== 'pivot').map((s) => s.name), ...names.values()])].sort((a, b) => a.localeCompare(b));
		linkDlg = { row, col, text: formula ? shownValue(row, col) : raw, locked: formula, target: linkKind(own) ? own : '', names: named };
	}

	function saveLink(target: string, text: string) {
		const d = linkDlg;
		const set = source.setLink;
		linkDlg = null;
		if (!d || !set) return;
		menuAction(() => set(d.row, d.col, target, text));
	}

	/** Supprimer le lien : sur la sélection (le texte reste). */
	export function removeLink() {
		linkDlg = null;
		const apply = source.applyStyle;
		if (!apply) return;
		const sel = selection();
		menuAction(() => apply(sel.r0, sel.c0, sel.r1, sel.c1, { ln: null })).then((ok) => ok && say(tr('grid.link.removed')));
	}

	/** Adresse web ou e-mail tapée seule dans une cellule sans lien : elle devient un lien, une étape à part (Ctrl+Z la retire). */
	function autoLinkAfter(row: number, col: number, raw: string) {
		const url = autoLink(raw);
		const set = source.setLink;
		if (!url || !set || linkOf(row, col)) return;
		writing = writing.catch(() => undefined).then(() =>
			set(row, col, url, '').then(
				() => say(tr('grid.link.auto')),
				() => undefined
			)
		);
	}

	// --- Barre d'état : sheet/status.svelte.ts ---
	const ms = (v: number) => (Number.isFinite(v) ? v.toFixed(v < 10 ? 1 : 0) + ' ms' : '–');
	// Totaux d'une grande sélection (ou de lignes pas encore chargées) :
	// calculés en SQL par le moteur, en tâche de fond ; la barre d'état les
	// affiche dès qu'ils arrivent. statsEpoch change avec les données.
	let statsEpoch = 0;
	const statusCtl = createStatus(sheetCtx);
	const { pushStat, cellInfo, cellAddr, addrA1, cellName, refreshStatus } = statusCtl;

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
	// --- Tri personnalisé… ------------------------------------------------------

	/** Dialogue « Tri personnalisé » ouvert : champ, valeurs, ordre enregistré et application. */
	let customDlg = $state<{
		field: string;
		loadValues: () => Promise<string[]>;
		loadOrder?: () => Promise<CustomOrder | null>;
		apply: (o: CustomOrder) => void;
	} | null>(null);
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
	// Menu ▾ fermé par un clic ailleurs : si ce clic est sur le ▾ de la même
	// colonne, il ne le rouvre pas (le ▾ ouvre et ferme son menu, comme Excel).
	let filterClosed = { key: '', at: 0 };
	// --- Filtres et tri personnalisé : sheet/filters.svelte.ts ---
	const filtersCtl = createFilters(sheetCtx);
	const { openPivotMenu, loadPivotValues, sortPivot, openCustomSort, openPivotCustomSort, applyPivotFilter, focusFirst, openFilter, toggleFilters, filterShown, startCustom, applyFilter, closeFilter, dismissFilter } = filtersCtl;
	export const sortCustomActive = filtersCtl.sortCustomActive;
	export const filterActive = filtersCtl.filterActive;

	// --- Mise en forme conditionnelle -----------------------------------------
	// Menu du ruban, petites boîtes, nouvelle règle et gestionnaire des règles :
	// CondFormat.svelte ; la feuille lui donne ses règles et sa sélection.

	let cfView = $state<CondView | null>(null);

	const cfHost: CondHost = {
		sheetRules,
		selectionLocs,
		selection: () => selection(),
		activeTable: () => {
			const e = ownerAt(active.row, active.col);
			return e ? sheetRules().tables.find((t) => t.id === e.t.id) : undefined;
		},
		async apply(ch: RuleChanges) {
			if (!source.setSheetRules) throw new Error(tr('grid.cf.unavailable'));
			await source.setSheetRules(ch);
		}
	};

	// --- Graphiques -------------------------------------------------------------

	const CHART_W = 480;
	const CHART_H = 288;
	// Graphique sélectionné (cadre, poignées, volet) et redimensionnement en cours.
	let selChart = $state<number | null>(null);
	let chartMenu = $state<{ ch: SheetChart; x: number; y: number } | null>(null);
	let chartClip = $state.raw<ChartConfig | null>(null);
	// --- Graphiques : sheet/charts.svelte.ts ---
	const chartsCtl = createCharts(sheetCtx);
	const { selectChart, nudgeChart, sourceColumns, chartBox, buttonBox, startChartGesture, onChartMenu, copyChart, copyChartImage, setMapView, pointToRow, pasteChart, editChart, removeChart } = chartsCtl;
	export const chartActive = chartsCtl.chartActive;

	// --- Tableau croisé dynamique ---------------------------------------------

	const pivotAt = (r: number, c: number) =>
		pivots.find((p) => !p.error && r >= (p.anchor ?? p.row) && r < p.row + p.cells.length && c >= p.col && c < p.col + p.cells[0].length);
	/** Tableau croisé qui touche la plage, s'il y en a un. */
	const pivotIn = (r0: number, c0: number, r1: number, c1: number) =>
		pivots.find((p) => !p.error && (p.anchor ?? p.row) <= r1 && p.row + p.cells.length - 1 >= r0 && p.col <= c1 && p.col + p.cells[0].length - 1 >= c0);
	/** Zone d'un tableau croisé sain sur la feuille, filtres de rapport compris. */
	const pivotZone = (p: SheetPivot) => ({ r0: p.anchor ?? p.row, c0: p.col, r1: p.row + p.cells.length - 1, c1: p.col + p.cells[0].length - 1 });
	// Ses colonnes saisies (tapées à droite de l'en-tête, docs/decisions/0008)
	// se modifient : leurs valeurs suivent les lignes du résultat.
	/** Colonnes chargées (pas saisies) du tableau de requête e touchées par les colonnes c0…c1 de la feuille. */
	const loadedIn = (e: Entry, c0: number, c1: number) => {
		for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol); c++) if (!e.t.columns[c - e.t.col]?.hand) return true;
		return false;
	};
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
	// --- Tableau croisé dynamique : sheet/pivot.svelte.ts ---
	const pivotCtl = createPivot(sheetCtx);
	const { pivotHeld, removePivot, repointPivot, pickPivotSource, onPlaceKey, startPivotMove, placePivotAt } = pivotCtl;
	export const pivotActive = pivotCtl.pivotActive;
	export const cancelPlacement = pivotCtl.cancelPlacement;
	export const placeOnNewSheet = pivotCtl.placeOnNewSheet;

	// Un tableau croisé est calculé : ses cellules ne se saisissent pas (comme Excel).
	const PIVOT_READONLY = $derived(tr('grid.pivot.readOnly'));
	// Suppr sur une partie d'un tableau croisé (Excel refuse aussi).
	const PIVOT_PARTIAL = $derived(tr('grid.pivot.partial'));
	// Tableau chargé par une requête (Power Query) : ses cellules ne se
	// saisissent pas, l'actualisation les remplacerait. Ses bords (ligne
	// dessous, colonnes à côté) : le moteur décide (un total, une note ou une
	// remarque restent à part ; une nouvelle ligne ou colonne est refusée, avec
	// le même message, ErrQueryTable).
	const QUERY_READONLY = $derived(tr('grid.query.readOnly'));
	/** Tableau de requête qui contient la cellule (hors colonnes saisies). */
	const queryAt = (r: number, c: number) => entries.find((e) => e.t.query && r >= e.t.row && r <= e.last && c >= e.t.col && c <= e.lastCol && loadedIn(e, c, c));

	// Volet latéral : champs du TCD ou mise en forme du graphique.
	let panel = $state<{ kind: 'pivot' | 'chart'; id: number } | null>(null);
	/** TCD dont le volet s'ouvre sur la liste des sources (« Changer la source… »). */
	let pivotSourcePick = $state<number | null>(null);

	/** Tableaux qui peuvent devenir la source d'un TCD (pas un TCD). */
	const pivotSources = (p: SheetPivot) => sources.filter((s) => s.kind !== 'pivot' && s.name !== p.config.name).map((s) => s.name);

	let openPivotOnLoad = false;
	let pivotPanelAt = '';

	// --- Pose d'un tableau croisé à la cellule cliquée (pivotplace.ts) -------
	// Insertion › Tableau croisé, clic droit › Déplacer : le mode ne bloque
	// ni le défilement, ni le clavier, ni les onglets, ni le ruban ; le
	// premier clic gauche sur une cellule pose le coin haut gauche, Échap
	// annule. Le composant reste le même d'une feuille à l'autre : la pose
	// suit le changement d'onglet.
	let placing = $state<Placement<PivotConfig> | null>(null);
	/** Cellule survolée pendant la pose (contour de l'emplacement). */
	let placeHover = $state<{ row: number; col: number } | null>(null);
	/** Taille du tableau croisé déplacé (lignes × colonnes) ; une cellule pour une création. */
	let placeSize = { h: 1, w: 1 };

	// --- Outils de données : sheet/datatools.svelte.ts ---
	const dataToolsCtl = createDataTools(sheetCtx);
	const { targetTable, targetCol, valueColumns, formulasInSelection, previewDedup, applyDedup } = dataToolsCtl;
	export const textToColumnsActive = dataToolsCtl.textToColumnsActive;
	export const toValuesActive = dataToolsCtl.toValuesActive;
	export const dedupActive = dataToolsCtl.dedupActive;

	/** Données › Convertir : assistant « Convertir le texte en colonnes » sur une sélection d'une seule colonne. */
	let ttcDlg = $state<{ r0: number; r1: number; col: number } | null>(null);

	/** Supprime les doublons du tableau de la cellule active (ruban Données). */
	let dedupDlg = $state<{ e: Entry; on: boolean[]; count: number | null; error: string } | null>(null);

	// --- Ruban Formules ---------------------------------------------------------

	let fnDlg = $state<{ search: string; sel: string | null } | null>(null);
	// Recherche tapée dans un tableau sans clé : numéro de ligne, ou colonne
	// qui devient la clé (jamais posée sans le demander). Valider réécrit la
	// saisie avec la réponse, en une étape d'annulation ; Annuler rouvre la
	// formule dans l'éditeur.
	let keyDlg = $state<(KeyQuestion & { row: number; col: number; raw: string; keep: boolean; choice: KeyChoice }) | null>(null);
	// Colonnes saisies à côté d'un tableau chargé (docs/decisions/0008) : la
	// colonne du résultat qui relie leurs valeurs aux lignes d'une
	// actualisation à l'autre, demandée à la première valeur saisie (refus
	// « jointure à choisir ») ou changée depuis le menu du tableau. « # » : le
	// rang des lignes.
	let handDlg = $state<{ table: string; columns: string[]; choice: string; write?: { row: number; col: number; raw: string; keep: boolean }; t?: SheetTable } | null>(null);
	// Insérer… / Supprimer… (comme la boîte d'Excel) : lignes ou colonnes entières.
	let linesDlg =$state<{ x: number; y: number; insert: boolean; choice: 'rows' | 'cols' } | null>(null);
	/** Géométrie de la feuille pour les objets posés (pixels de la feuille). */
	const objGeo = { colOffset: (c: number) => cax().offset(c), rowTop: (r: number) => rowTop(r), colAt: (x: number) => cax().indexAt(x), rowAt: (y: number) => rax().indexAt(y) };
	// Les cases à cocher et listes relisent leur cellule liée à chaque mise à jour.
	let controlsTick = $state(0);
	// --- Commandes du ruban : sheet/commands.svelte.ts ---
	const commandsCtl = createCommands(sheetCtx);
	const { shownFunctions, selectionLines, applyKeyDialog, cancelKeyDialog, applyHandDialog, cancelHandDialog, openHandKey, openLinesDialog, applyLinesDialog, tableLines, deleteWholeColumns, onGlobalKey, tabChoiceFor, runTabChoice, closeMenus, runButton, fireControl } = commandsCtl;
	export const startFormula = commandsCtl.startFormula;
	export const autoSum = commandsCtl.autoSum;
	export const openFunctions = commandsCtl.openFunctions;
	export const sortActive = commandsCtl.sortActive;
	export const sheetLines = commandsCtl.sheetLines;
	export const cellsActive = commandsCtl.cellsActive;
	export const selectedRange = commandsCtl.selectedRange;
	export const selectRange = commandsCtl.selectRange;
	export const cellBox = commandsCtl.cellBox;
	export const insertObject = commandsCtl.insertObject;
	export const insertButton = commandsCtl.insertButton;
	export const revealObject = commandsCtl.revealObject;

	// --- Audit de formules : flèches des antécédents et des dépendants --------

	// Liens affichés : pour chaque cellule de la sélection, ses antécédents
	// (flèches bleues vers elle) et ses dépendants (flèches rouges depuis elle).
	let links = $state<Link[]>([]);
	// --- Audit de formules : sheet/audit.svelte.ts ---
	const auditCtl = createAudit(sheetCtx);
	const { sheetIdOf, refreshFx, say } = auditCtl;
	export const auditActive = auditCtl.auditActive;
	export const closeOverlays = auditCtl.closeOverlays;

	// --- Doigt (touch.ts) ----------------------------------------------------------
	// Glisser fait défiler dans les deux sens, avec élan ; un appui choisit la
	// cellule comme un clic (sur la cellule active : la saisie, clavier à
	// l'écran) ; l'appui long donne le menu contextuel (ui/mobile.ts) ; deux
	// doigts zooment. Pas d'extension de sélection, de poignée de recopie ni
	// de glissé d'en-tête au doigt : la souris et le stylet les gardent.
	let touch: { id: number; x0: number; y0: number; x: number; y: number; t0: number; drag: boolean; vel: Velocity } | null = null;
	// --- Main (pan.ts) --------------------------------------------------------------
	// Clic droit ou du milieu glissé, ou Espace + clic gauche glissé : la
	// feuille suit le pointeur. Pas de menu contextuel après un glissé.
	const pan = new PanGesture<MouseEvent>();
	const space = new SpaceHold();
	let panCursor = $state<'' | 'grab' | 'grabbing'>('');
	let sizedZoom = 0;
	// --- Défilement, doigt, main, volet ouvert : sheet/scrolling.svelte.ts ---
	const scrollCtl = createScrolling(sheetCtx);
	const { stopFling, panDown, panMove, panUp, objectPanDown, panMenu, panReset, spaceKey, onSpaceUp, onPointerDown, onPointerMove, onPointerUp, onScroll, onWheel, syncNative, stopGlide, keepFocusAfterShrink, ensureVisible } = scrollCtl;
	export const scrollLeftTo = scrollCtl.scrollLeftTo;

	// --- Clavier : sheet/keyboard.svelte.ts ---
	/** Cellule voisine de la cellule active (une fusion compte pour une cellule). */
	const step = (dr: number, dc: number) => stepCell(active.row, active.col, dr, dc, boxes, SHEET_ROWS - 1, SHEET_COLS - 1);
	// Dernière action répétable (F4) : saisie, mise en forme, insertion…
	let lastRepeat: (() => void) | null = null;
	// Les saisies partent l'une après l'autre : la suivante peut dépendre de
	// la précédente (une saisie fait naître un tableau, la suivante le remplit).
	// Une écriture qui échoue ne bloque jamais les suivantes (catch).
	let writing: Promise<void> = Promise.resolve();
	const keyboardCtl = createKeyboard(sheetCtx);
	const { tabMove, enterMove, moveTo, jump, cellAt, copyFill, onKeyDown } = keyboardCtl;
	// --- Souris : sheet/mouse.svelte.ts ---

	let resizing: { col: number; startX: number; startW: number; width: number } | null = null;
	let rowResizing: { row: number; startY: number; startH: number; height: number } | null = null;

	let filling = $state<{ sel: Range; to?: Range } | null>(null);
	// Sélection de lignes ou de colonnes entières par la marge ou le bandeau.
	let headerDrag: 'rows' | 'cols' | null = null;
	// Sélection de lignes ou colonnes entières (marge ou bandeau) : tant que la
	// sélection n'a pas changé, la mise en forme vaut pour les lignes ou
	// colonnes entières. Elles restent exactement celles choisies (range), sans
	// s'agrandir aux fusions qui les traversent, comme Excel (voir wholeLines) ;
	// from : la ligne (colonne) où le clic a commencé, d'où partent le glisser
	// et Maj+clic (la cellule active, elle, est le haut de sa fusion).
	let wholeSel: WholeSel | null = null;
	// Dernière référence insérée au clic : un autre clic la remplace (comme Excel).
	let lastRef: { el: TextField; start: number; end: number; value: string } | null = null;
	// Glissé en mode formule depuis une référence insérée : la plage couverte.
	let refDrag: { from: { row: number; col: number }; to: { row: number; col: number } } | null = null;
	// Marqueur de référence glissé (comme Excel : on attrape le bord du cadre
	// coloré d'une référence pendant la saisie d'une formule, on le dépose
	// ailleurs, la référence est réécrite).
	let refMove = $state<{ start: number; end: number; rect: Rect; from: { row: number; col: number }; to: { row: number; col: number } } | null>(null);
	const mouseCtl = createMouse(sheetCtx);
	const { autoFitColumns, dropMove, fillHandle, doFill, fillDown, headerAt, lastRow, lastCol, wholeKind, selectRows, inSelection, onMouseDown, nextArg, refContext, dropRefMark, refTables, onMouseMove, onDragMove, stopAutoScroll, onDblClick } = mouseCtl;

	// --- Édition et zone de nom : sheet/edit.svelte.ts ---
	const editCtl = createEdit(sheetCtx);
	const { readOnlyReason, designWhy, editText, startEdit, closeReadOnly, spilledAt, placeEditor, write, commit, handToFx, formulaReady, reopenFormula, cancel, clearSelection, onEditorKey, onFxKey, commitEditor, leaveFx, onTableNameKey, onColumnNameKey, onNameKey } = editCtl;

	// --- Rechercher / remplacer -------------------------------------------------

	// Comme Excel, une sélection de plusieurs cellules borne la recherche
	// (zone) : elle reste affichée et la cellule trouvée (at) y est
	// encadrée ; une seule cellule : toute la feuille, la cellule active va
	// sur chaque occurrence.
	// count : compteur du champ (« 3 sur 12 », « Aucun résultat ») ; info :
	// bilan de Remplacer tout ou erreur, sous les champs.
	// opts : options (« Options », findopts.ts), gardées d'une ouverture à
	// l'autre ; more : options dépliées ; list : Rechercher tout (null : pas
	// demandé), total : toutes les cellules trouvées, busy : en cours.
	let finder = $state<{
		replace: boolean;
		text: string;
		by: string;
		count: string;
		info: string;
		zone: Zone | null;
		at: { row: number; col: number } | null;
		opts: FindOpts;
		more: boolean;
		list: FoundCell[] | null;
		total: number;
		busy: boolean;
		current: number;
	} | null>(null);
	let findInput = $state<HTMLInputElement>();
	let replaceInput = $state<HTMLInputElement>();
	// Occurrences (toutes feuilles), ordre des feuilles parcourues (rang k), total.
	let matches: FoundCell[] = [];
	// Recherche des occurrences en mémoire (texte, zone, feuille et options).
	let matchKey = '';
	// Cellule trouvée, repérée à l'image en cours (ligne et colonne entières teintées).
	let findCross: Zone | null = null;
	/** Zone de recherche : la sélection si elle compte plusieurs cellules, sinon null (toute la feuille). */
	const searchZone = () => findZone(selection(), singleSelection());
	// --- Rechercher / remplacer : sheet/find.svelte.ts ---
	const findCtl = createFind(sheetCtx);
	const { foundRect, matchesKey, syncFinderZone, closeFinder, toggleReplace, findChanged, goFound, findArrived, findNext, listAll, replaceOne, replaceAll, onFinderKey } = findCtl;
	export const openFinder = findCtl.openFinder;

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
			say(tr('grid.copy.limit'));
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
		say(tr('grid.copy.reading'));
		const ok = await Promise.all(todo.map(([e, a, b]) => e.cache.ensure(a, b)));
		return ok.every(Boolean);
	}

	/** Copie ou coupe après avoir chargé les lignes manquantes (le presse-papiers est écrit ensuite). */
	async function copyLater(cut: boolean) {
		if (!(await loadRange(selection()))) {
			say(tr('grid.copy.changed'));
			return;
		}
		const text = takeClip(cut);
		if (text === null) return;
		try {
			await writeClip(text);
		} catch {
			clipRange = null;
			say(tr('grid.clipboard.unavailable'));
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
				say(tr('grid.pivot.partialCut'));
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
		say(cut ? tr('grid.clipboard.cut') : tr('grid.clipboard.copied', { n }));
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
			await writeClip(text);
		} catch {
			clipRange = null;
			say(tr('grid.clipboard.unavailable.copy'));
			return;
		}
		clipMessage(cut);
	}

	/** Version HTML de la dernière copie (valeurs et mise en forme), pour un collage dans un tableur. */
	function clipHtml(): string {
		const c = clipRange;
		if (!c) return '';
		return toClipHtml(c.values, (i, j) => (styleAt(c.r0 + i, c.c0 + j) ?? null) as ClipStyle | null);
	}

	/** Écrit la copie dans le presse-papiers : texte tabulé et, s'il le permet, HTML avec la mise en forme. */
	async function writeClip(text: string) {
		const html = clipHtml();
		if (html && typeof ClipboardItem !== 'undefined' && navigator.clipboard.write) {
			try {
				await navigator.clipboard.write([new ClipboardItem({ 'text/plain': new Blob([text], { type: 'text/plain' }), 'text/html': new Blob([html], { type: 'text/html' }) })]);
				return;
			} catch {
				// HTML refusé : le texte seul.
			}
		}
		await navigator.clipboard.writeText(text);
	}

	/** Lit le presse-papiers : texte et, s'il y en a, sa version HTML (copie d'un tableur) ; null s'il est inaccessible. */
	async function readClip(): Promise<{ text: string; html: string; image?: Blob } | null> {
		try {
			let text = '';
			let html = '';
			let image: Blob | undefined;
			for (const it of await navigator.clipboard.read()) {
				if (!text && it.types.includes('text/plain')) text = await (await it.getType('text/plain')).text();
				if (!html && it.types.includes('text/html')) html = await (await it.getType('text/html')).text();
				// Image copiée (capture, navigateur) : posée sur la feuille (SheetObjects).
				const it2 = it.types.find((ty) => ty.startsWith('image/'));
				if (!image && it2) image = await it.getType(it2);
			}
			return { text, html, image };
		} catch {
			try {
				return { text: await navigator.clipboard.readText(), html: '' };
			} catch {
				return null;
			}
		}
	}

	/** Coller à la cellule active (ruban, clic droit). */
	export async function pasteClipboard() {
		wrap.focus();
		const clip = await readClip();
		if (!clip) {
			say(tr('grid.clipboard.unavailable.paste'));
			return;
		}
		if (clip.image && !clip.text.trim()) {
			objectsLayer?.insertImageFile(new File([clip.image], 'image', { type: clip.image.type }));
			return;
		}
		pasteText(clip.text, clip.html);
	}

	// --- Collage spécial (Ctrl+Alt+V) ---------------------------------------------

	let pasteDlg = $state<{ text: string; html: string; internal: boolean; formats: boolean } | null>(null);

	/** Collage spécial : la boîte, ou une entrée directe (Valeurs, Formules, Formats, Transposer). */
	export async function pasteSpecial(kind: PasteKind = 'dialog') {
		wrap.focus();
		if (editing || !source.paste) return;
		const clip = await readClip();
		if (!clip) {
			say(tr('grid.clipboard.unavailable.paste'));
			return;
		}
		if (kind === 'dialog') {
			const internal = !!source.pasteSpecial && !!richClip(clipRange, clip.text);
			pasteDlg = { ...clip, internal, formats: internal || hasClipStyles(parseClipHtml(clip.html)) };
			return;
		}
		if (!pasteWith(pasteOptions(kind), clip.text, clip.html)) say(tr('grid.pasteSpecial.empty'));
	}

	/**
	 * Colle le presse-papiers (text, html) selon les choix du collage spécial ;
	 * faux si rien à coller. styled : collage ordinaire d'un bloc copié dans
	 * un tableur, avec sa mise en forme (depuis pasteText).
	 */
	function pasteWith(opt: PasteOptions, text: string, html: string, styled = false): boolean {
		if (!source.paste) return false;
		if (plainPaste(opt) && !styled) return pasteText(text, html);
		const sel = selection();
		const finish = (area: ClipRect) => (m: string | void) => {
			message = m || tr('grid.clipboard.pasted');
			active = { row: area.r0, col: area.c0 };
			anchor = { row: area.r1, col: area.c1 };
			refreshFx();
			refreshStatus();
			schedule();
		};
		const fail = (err: unknown) => {
			message = errorText(err);
			refreshStatus();
			schedule();
		};
		const blocked = (area: ClipRect) => {
			if (pivotIn(area.r0, area.c0, area.r1, area.c1)) say(PIVOT_READONLY);
			else if (queryIn(area.r0, area.c0, area.r1, area.c1)) say(QUERY_READONLY);
			else return false;
			return true;
		};
		// Copie faite dans Lattis : la zone source, lue par le modèle.
		const rich = source.pasteSpecial ? richClip(clipRange, text) : null;
		if (rich && source.pasteSpecial) {
			const run = source.pasteSpecial;
			const [h, w] = opt.transpose ? [rich.c1 - rich.c0 + 1, rich.r1 - rich.r0 + 1] : [rich.r1 - rich.r0 + 1, rich.c1 - rich.c0 + 1];
			const area = opt.what === 'widths' ? sel : pasteArea(h, w, sel);
			if (opt.what !== 'widths' && blocked(area)) return true;
			const src = { r0: rich.r0, c0: rich.c0, r1: rich.r1, c1: rich.c1 };
			writing = writing.catch(() => undefined).then(() => run(rich.sheet, src, sel, opt, rich.values).then(finish(area), fail));
			return true;
		}
		// Venue d'ailleurs : le texte, et la mise en forme de sa version HTML.
		const parsed = html ? parseClipHtml(html) : null;
		const tsv = parseTSV(text);
		const block = parsed ? clipValues(parsed, tsv) : tsv;
		if (!block.length || !source.pasteBlock) return false;
		if (opt.what === 'widths') {
			say(tr('grid.pasteSpecial.noWidths'));
			return true;
		}
		const withStyles = hasClipStyles(parsed);
		if ((opt.what === 'formats' || opt.what === 'valuesNumbers') && !withStyles) {
			if (opt.what === 'formats') {
				say(tr('grid.pasteSpecial.noFormats'));
				return true;
			}
			opt = { ...opt, what: 'values' };
		}
		const h0 = block.length;
		const w0 = Math.max(...block.map((r) => r.length));
		const area = opt.transpose ? pasteArea(w0, h0, sel) : pasteArea(h0, w0, sel);
		if (blocked(area)) return true;
		const run = source.pasteBlock;
		const styles = withStyles && parsed ? parsed.styles.map((r) => r.map((s) => (s ? JSON.stringify(s) : ''))) : null;
		writing = writing.catch(() => undefined).then(() => run(sel, block, styles, opt).then(finish(area), fail));
		return true;
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
		e.clipboardData?.setData('text/html', clipHtml());
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
			say(tr('grid.chart.copied'));
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
		e.clipboardData?.setData('text/html', clipHtml());
		e.preventDefault();
		clipMessage(false);
	}

	function onPaste(e: ClipboardEvent) {
		if (document.activeElement !== wrap || editing || !source.paste) return;
		const text = e.clipboardData?.getData('text/plain') ?? '';
		if (pasteText(text, e.clipboardData?.getData('text/html') ?? '')) e.preventDefault();
	}

	/**
	 * Colle un texte (TSV ou graphique) sur la sélection ; faux si rien à
	 * coller. Le texte de la dernière copie de Lattis se colle « riche »
	 * (transfer : formules, mise en forme, tableaux ; une zone coupée se
	 * déplace). Comme Excel, une sélection multiple du bloc copié le reçoit
	 * répété (une cellule copiée remplit toute la sélection). Copié d'un
	 * tableur (html), le bloc garde sa mise en forme (pasteBlock).
	 */
	function pasteText(text: string, html = ''): boolean {
		if (!source.paste) return false;
		const sel = selection();
		// Graphique (d'une feuille ou d'un document) ; tableau croisé
		// d'un document, posé à la cellule active par sa configuration.
		const obj = objFromText(text);
		if (obj?.kind === 'shape') {
			objectsLayer?.pasteObject(obj.config as unknown as ObjectConfig);
			return true;
		}
		if (obj?.kind === 'chart') {
			pasteChart(obj.config as unknown as ChartConfig);
			return true;
		}
		if (obj?.kind === 'pivot' && source.placePivot && !(clipRange?.pivot && sameClipText(clipRange.text, text))) {
			const place = source.placePivot;
			writing = writing.catch(() => undefined).then(() =>
				place(sel.r0, sel.c0, obj.config as unknown as PivotConfig).then(
					() => say(tr('grid.pivot.pasted')),
					(err) => say(errorText(err))
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
						message = pc.cut ? tr('grid.pivot.place.moved') : tr('grid.pivot.pasted');
						refreshStatus();
						schedule();
					},
					(err) => {
						message = errorText(err);
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
			message = errorText(err);
			refreshStatus();
			schedule();
		};
		if (rich && source.transfer) {
			const transfer = source.transfer;
			const src = { r0: rich.r0, c0: rich.c0, r1: rich.r1, c1: rich.c1 };
			if (rich.cut) clipRange = null;
			writing = writing.catch(() => undefined).then(() =>
				transfer(rich.sheet, src, area, rich.cut, rich.values).then(
					(m) => done(m, rich.cut ? tr('grid.cells.moved') : tr('grid.clipboard.pasted')),
					fail
				)
			);
			return true;
		}
		// Copié d'un tableur : valeurs et mise en forme (collage ordinaire).
		if (!rich && html && source.pasteBlock && hasClipStyles(parseClipHtml(html))) {
			return pasteWith({ what: 'all', op: '', skipBlanks: false, transpose: false }, text, html, true);
		}
		// Texte venu d'ailleurs (ou d'un tableau croisé) : des valeurs.
		const tiled = tileBlock(block, sel);
		const cut = clipRange?.cut && richClip({ ...clipRange, plain: false }, text) ? clipRange : null;
		const paste = source.paste;
		writing = writing.catch(() => undefined).then(() => paste(area.r0, area.c0, tiled).then((m) => done(m, tr('grid.clipboard.pastedRows', { n: tiled.length })), fail));
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

	// --- Type de données et menu contextuel : sheet/menus.svelte.ts ---

	const TYPES: ColumnType[] = ['text', 'integer', 'decimal', 'date', 'boolean'];
	// Une valeur hors type fait passer une colonne inférée en texte : l'infobulle
	// de l'en-tête et le menu ▾ disent combien de valeurs sortent du type
	// majoritaire, et le menu propose d'y revenir (ces valeurs marquées en erreur).
	const mixes = new Map<string, Promise<TypeMix | null>>();
	/** Infobulle d'un en-tête de colonne : nom complet et type. */
	let tipCell = '';

	// rowOnly : clic droit sur un numéro de ligne (pas d'actions de colonne).
	// confirmDelete : suppression d'un grand tableau en attente de confirmation.
	let menu = $state<{ x: number; y: number; e: Entry; n: number; i: number; header: boolean; rowOnly?: boolean; confirmDelete?: boolean } | null>(null);
	/** Colonnes que « Choisir par… » propose (null : pas encore connues), relues à l'ouverture d'un menu. */
	let choiceCands = $state.raw<ChoiceCandidate[] | null>(null);
	/** Au-delà de ce nombre de lignes, « Supprimer le tableau » demande confirmation. */
	const CONFIRM_DELETE_ROWS = 500;
	/** La cellule est dans une zone « plage » (tableau converti). */
	// Cellule libre gardée hors du tableau voisin (note, total : style « out »).
	const keptOut = (row: number, col: number) => !!(freeStyles.get(key(row, col)) as { out?: boolean } | undefined)?.out;
	// « Utiliser comme en-têtes » : le tableau (hors requête) dont l'en-tête est
	// juste sous la cellule libre remplie (row, col) ; sur l'en-tête, une
	// cellule remplie au-dessus de ses colonnes.
	const titleTableAt = (row: number, col: number) => {
		if (!source.titleToHeader || !(cells.get(key(row, col)) ?? '').trim() || tableAt(row, col)) return undefined;
		const t = tableUnderTitle(entries.map((e) => e.t), row, col);
		return t && !t.query ? t : undefined;
	};
	const titleAbove = (t: SheetTable) => !!source.titleToHeader && !t.query && titleAboveFilled(t, (r, c) => cells.get(key(r, c)));
	const inRangeZone = (row: number, col: number) => (data.ranges ?? []).some((z) => row >= z.r0 && row <= z.r1 && col >= z.c0 && col <= z.c1);
	// Menu du clic droit hors tableau : presse-papiers et effacement.
	let freeMenu = $state<{ x: number; y: number; kind?: 'rows' | 'cols' } | null>(null);
	const menusCtl = createMenus(sheetCtx);
	const { mixOf, mixText, spillUnder, updateHeaderTip, openTypeMenu, headerButton, frozenHeaderAt, chooseType, chooseKey, deleteTable, keyContextMenu, onContextMenu, linesOpts, menuAction } = menusCtl;

	// --- Mesures ------------------------------------------------------------

	// --- Mesure de la grille : sheet/bench.svelte.ts ---
	const benchCtl = createBench(sheetCtx);
	export const runBench = benchCtl.runBench;

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
			// Largeur d'avant, si seul l'écran a changé (pas le zoom).
			const oldW = zoom === sizedZoom ? viewW : 0;
			const oldH = viewH;
			sizedZoom = zoom;
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
			// Volet ouvert à droite : l'objet cliqué ne passe pas dessous.
			if (afterWidthChange(oldW, cw) === 'keep') keepFocusAfterShrink(oldW);
			// Clavier à l'écran (téléphone) : la cellule en saisie reste visible au-dessus.
			else if (editing && oldH > 0 && ch < oldH) ensureVisible(active.row, active.col);
		});
		const ro = new ResizeObserver(resize);
		ro.observe(wrap);
		resize();
		scroller.addEventListener('wheel', onWheel, { passive: false });
		const up = (ev?: MouseEvent) => {
			releaseLink(ev);
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
					writing = writing.catch(() => undefined).then(() => setHeight(row, height).then(() => undefined, (err) => say(errorText(err))));
				}
				return;
			}
			if (!resizing) return;
			const { col, width, startW } = resizing;
			resizing = null;
			if (width === startW || !source.setColumnWidth) return;
			userWidths.set(col, width);
			const setWidth = source.setColumnWidth;
			writing = writing.catch(() => undefined).then(() => setWidth(col, width).then(() => undefined, (err) => say(errorText(err))));
		};
		window.addEventListener('mouseup', up);
		window.addEventListener('mousemove', onDragMove);
		window.addEventListener('mousemove', panMove);
		window.addEventListener('mouseup', panUp);
		window.addEventListener('contextmenu', panMenu, true);
		window.addEventListener('keyup', onSpaceUp);
		window.addEventListener('blur', panReset);
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
			stopFling();
			window.removeEventListener('mouseup', up);
			window.removeEventListener('mousemove', onDragMove);
			window.removeEventListener('mousemove', panMove);
			window.removeEventListener('mouseup', panUp);
			window.removeEventListener('contextmenu', panMenu, true);
			window.removeEventListener('keyup', onSpaceUp);
			window.removeEventListener('blur', panReset);
			stopAutoScroll();
			clearInterval(timer);
			window.removeEventListener('copy', onCopy);
			window.removeEventListener('keydown', onGlobalKey);
			window.removeEventListener('cut', onCut);
			window.removeEventListener('paste', onPaste);
			window.removeEventListener('keydown', onPlaceKey, true);
			if (placing) onplacing?.(null);
			cancelAnimationFrame(frame);
			stopGlide();
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

	// --- Impression (lib/print/sheetprint.ts) -----------------------------------
	// Ce que drawCells dessine, cellule par cellule, avec les mêmes fonctions
	// (styleAt, cfAt, formatCell, freeDisplay…) : la page imprimée montre la
	// grille, sans la sélection ni les boutons.

	/** Tailles de la grille affichée (découpage des pages, DocumentService.PrintPlan). */
	export function printSizes(): { row: number; col: number; rows: number[][]; cols: number[][] } {
		return { row: rowAxis.def, col: colAxis.def, rows: [...rowAxis.entries()], cols: [...colAxis.entries()] };
	}

	/** Une cellule telle que la grille l'affiche (undefined : vide, sans mise en forme). */
	function printCell(r: number, c: number): PrintCell | undefined {
		const e = tableAt(r, c);
		if (e) {
			const i = c - e.t.col;
			const col = e.t.columns[i];
			const head = { fill: e.t.query ? C.linkHead : C.head, color: e.t.query && !col.hand ? C.linkHeadText : C.headText, bold: true, framed: true };
			if (r === e.t.row) {
				// Colonne sur un niveau : son nom est sur la bande (fusionnée avec l'en-tête).
				if (singleCol(e, i) && rh(r - 1)) return { text: '', ...head };
				return { text: headerLabel(col.name, e.t.twoLevel), style: styleAt(r, c, e), align: 'left', ...head };
			}
			const row = e.cache.row(r - e.t.row - 1);
			if (!row) return { text: '', style: styleAt(r, c, e), framed: true };
			const v = row.cells[i];
			const bad = row.bad?.has(i);
			const own = colRules.get(e.t.id)?.[i];
			const cf = cfAt(r, c, { v, text: row.text[i] ?? '', error: !!bad || (col.type === 'formula' && isErrorValue(v)) }, own ? { rules: own, loc: { k: 'tbl', t: e.t.id, c: i }, mark: (_, k) => !!row.marks?.has(i * 1000 + k) } : undefined);
			const cs = resolveStyle(styleAt(r, c, e), cf?.style);
			const base = { style: cs, framed: true, bar: cf?.bar };
			if (showsNull(col.type, v, bad)) return { ...base, text: NULL_MARK, align: 'center', color: C.nullMark };
			let s = row.text[i] ?? '';
			const icon = cf?.icon && !bad ? cf.icon : undefined;
			if (cf?.hide && !bad) s = '';
			let ps = cs;
			if (cs && !bad && s) [s, ps] = codeShown(s, v, cs);
			s = plain(s);
			if (bad) return { ...base, text: s, align: 'left', color: C.error };
			const err = col.type === 'formula' && isErrorValue(v);
			return { ...base, style: ps, text: s, icon, align: cs?.al ?? (col.type === 'formula' ? valueAlignment(v) : alignment(col.type)), color: err ? C.error : undefined };
		}
		const band = groupBandAt(r, c);
		if (band) {
			const i = c - band.t.col;
			const head = { fill: band.t.query ? C.linkHead : C.head, color: band.t.query ? C.linkHeadText : C.headText, bold: true, framed: true };
			if (singleCol(band, i)) return { text: band.t.columns[i].name, style: bandStyle(band, i, i), align: 'left', ...head };
			const s = groupSpans(band.t.columns.map((k) => k.name)).find((x) => band.t.col + x.i0 === c);
			if (!s) return { text: '', ...head };
			return { text: s.group ?? '', style: bandStyle(band, s.i0, s.i1), align: 'center', ...head };
		}
		const p = pivots.length ? pivotAt(r, c) : undefined;
		if (p) {
			if (r < p.row) {
				// Filtres de rapport au-dessus du tableau croisé : champ, valeur retenue.
				const f = (p.config.filters ?? [])[r - (p.anchor ?? p.row)];
				if (!f || c > p.col + 1) return undefined;
				return { text: c === p.col ? f.field : pivotFilterText(f.values), bold: c === p.col, fill: C.linkHead, framed: true };
			}
			const i = r - p.row;
			const j = c - p.col;
			const v = p.cells[i]?.[j] ?? '';
			const st = resolveStyle(styleAt(r, c, undefined), freeCf(r, c)?.style);
			const hs = handStart(p);
			const tot = pivotTotalRow(p);
			const info = j === 0 ? pivotRowOf(p, r) : undefined;
			const fill = r === p.row || r === tot ? (r === p.row && c >= hs ? C.head : C.linkHead) : undefined;
			return {
				text: v ? pivotValueText(formatCell(v, undefined, st), i, j) : '',
				style: st,
				bold: pivotBold(p, r),
				fill,
				color: i === 0 && c >= hs ? C.headText : i === 0 || r === tot ? C.linkHeadText : undefined,
				align: st?.al ?? (c >= hs && i > 0 ? handAlign(v) : j > 0 && i > 0 ? 'right' : 'left'),
				indent: pivotLabelOffset(info) || undefined,
				framed: true
			};
		}
		for (const sp of spills) {
			const v = sp.rows[r - sp.row]?.[c - sp.col];
			if (!v || (r === sp.row && c === sp.col)) continue;
			const st = styleAt(r, c, undefined);
			return { text: formatCell(v, undefined, st), style: st, align: st?.al ?? freeDisplay('', v).align, color: isErrorValue(v) ? C.error : undefined };
		}
		const k = key(r, c);
		const raw = cells.get(k);
		const f = shown.get(k);
		const cf = freeCf(r, c);
		const st = resolveStyle(styleAt(r, c, undefined), cf?.style);
		if (raw === undefined && f === undefined) return st || cf?.bar ? { text: '', style: st, bar: cf?.bar } : undefined;
		const fd = freeDisplay(raw ?? '', f?.text);
		const [shownText, ps] = cf?.hide ? ['', st] : fd.kind === 'text' ? [fd.text, st] : codeShown(fd.text, undefined, st);
		const text = plain(shownText);
		return { text, style: ps, bar: cf?.bar, icon: cf?.icon, align: st?.al ?? fd.align, color: f?.error ? C.error : undefined, overflow: !mergeOf(r, c) };
	}

	/** Graphiques de la feuille, dessinés comme à l'écran (SVG cloné ; canevas en image). */
	function printCharts(): PrintChart[] {
		const out: PrintChart[] = [];
		for (const ch of charts) {
			const el = wrap?.querySelector<HTMLElement>(`.chart[data-chart-id="${ch.id}"]`);
			if (!el) continue;
			const copy = el.cloneNode(true) as HTMLElement;
			copy.querySelectorAll('.handle').forEach((h) => h.remove());
			const live = el.querySelectorAll('canvas');
			copy.querySelectorAll('canvas').forEach((cv, n) => {
				const img = document.createElement('img');
				try {
					img.src = live[n].toDataURL('image/png');
				} catch {
					// Canevas teinté par une image d'un autre site (tuiles de carte) : laissé vide.
				}
				img.width = cv.width;
				img.style.cssText = cv.style.cssText;
				cv.replaceWith(img);
			});
			out.push({ row: ch.row, col: ch.col, x: ch.config.offX ?? 0, y: ch.config.offY ?? 0, w: ch.config.width ?? CHART_W, h: ch.config.height ?? CHART_H, html: copy.innerHTML });
		}
		return out;
	}

	/** Ce que la feuille donne à imprimer. */
	export function printSource(): PrintSource {
		const anyRules = sheetRowRules.size > 0 || sheetColRules.size > 0 || cellRules.size > 0 || [...colRules.values()].some((l) => l.some((x) => x?.length));
		return {
			merges: [...boxes],
			// Images, formes et zones de texte au-dessus des graphiques, comme à l'écran.
			charts: [...printCharts(), ...(objectsLayer?.printItems() ?? [])],
			async prepare(r0, r1, c0, c1) {
				for (const e of entries) {
					const a = Math.max(r0, e.t.row + 1);
					const b = Math.min(r1, e.last);
					if (a <= b && e.lastCol >= c0 && e.t.col <= c1) await e.cache.ensure(a - e.t.row - 1, b - e.t.row - 1);
				}
				if (!anyRules) return;
				// Statistiques des règles (barres, échelles, rangs…) : demandées une
				// fois, attendues avant d'écrire les pages.
				cfWaits = [];
				try {
					for (let r = r0; r <= r1; r++) for (let c = c0; c <= c1; c++) if (rh(r) && cax().size(c)) printCell(r, c);
					await Promise.allSettled(cfWaits);
				} finally {
					cfWaits = null;
				}
			},
			cell: (r, c) => printSpark(printCell(r, c), r, c)
		};
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
	// Barre Rechercher / remplacer : sheet/FindBar.svelte.
	const findView: FindBarView = sheetCtx;
	// Menus contextuels de la grille : sheet/CellMenus.svelte.
	const cellMenusView: CellMenusView = sheetCtx;
	// Panneau ▾ d'une colonne : sheet/FilterPanel.svelte.
	const filterView: FilterPanelView = sheetCtx;
	// Validation des données : sheet/ValidationDialog.svelte.
	const validationView: ValidationDialogView = sheetCtx;
	// Insérer une fonction : sheet/FunctionsDialog.svelte.
	const functionsView: FunctionsDialogView = sheetCtx;
	// Supprimer les doublons : sheet/DedupDialog.svelte.
	const dedupView: DedupDialogView = sheetCtx;
	// Menu d'un graphique : sheet/ChartMenu.svelte.
	const chartMenuView: ChartMenuView = sheetCtx;
	// Note de la cellule : sheet/NoteEditor.svelte.
	const noteView: NoteEditorView = sheetCtx;
	// Insérer ou supprimer des lignes : sheet/LinesDialog.svelte.
	const linesView: LinesDialogView = sheetCtx;
	// Hauteur ou largeur : sheet/SizeDialog.svelte.
	const sizeView: SizeDialogView = sheetCtx;
	// Alerte de validation : sheet/DvAlertBox.svelte.
	const dvAlertView: DvAlertBoxView = sheetCtx;
	// Liste déroulante : sheet/ListMenu.svelte.
	const listMenuView: ListMenuView = sheetCtx;
	// Filtres d'un TCD : sheet/PivotFilter.svelte.
	const pivotFilterView: PivotFilterView = sheetCtx;
	// Clé du tableau : sheet/KeyDialog.svelte.
	const keyDialogView: KeyDialogView = sheetCtx;
	// Colonne saisie à la main : sheet/HandDialog.svelte.
	const handDialogView: HandDialogView = sheetCtx;
	// Clé proposée : sheet/KeyHintMenu.svelte.
	const hintMenuView: KeyHintMenuView = sheetCtx;
	// Pastille « suit l'onglet » : sheet/FollowMenu.svelte.
	const followMenuView: FollowMenuView = sheetCtx;
</script>

<!-- Formule colorée (références, faute soulignée), sur une seule ligne : le texte est affiché tel quel (pre-wrap). -->
{#snippet coloured(text: string)}{#each highlightParts(text, formulaRefs(text), editErr) as p, i (i)}{#if p.err}<span class="hl-err">{p.text}</span>{:else if p.color}<span style:color={p.color}>{p.text}</span>{:else}{p.text}{/if}{/each}{'​'}{/snippet}

{#snippet suggestions(where: 'cell' | 'fx')}
	{#if sig?.where === where}
		{@const sp = signatureParts(sig.fn, sig.index)}
		<div class="sig">
			<div class="sig-line">{#each sp.parts as p, i (i)}{#if p.on}<b>{p.text}</b>{:else}{p.text}{/if}{/each}</div>
			{#if sig.fn.description}<div class="sig-desc">{sig.fn.description}</div>{/if}
			{#if sp.arg}<div class="sig-arg"><b>{sp.arg.name}</b>{sp.arg.optional ? ' ' + tr('grid.sig.optional') : ''} : {sp.arg.desc}</div>{/if}
			{#if sig.fn.example}<div class="sig-ex">{tr('grid.sig.example')} <code>{sig.fn.example}</code></div>{/if}
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
				title={ok ? tr('grid.formula.validate.tooltip') : tr('grid.formula.validate.error')}
				onmousedown={(e) => {
					// Le focus reste dans l'éditeur : la validation passe par Entrée.
					e.preventDefault();
					(where === 'cell' ? input : fxInput)?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
				}}>{tr('grid.formula.validate')}</button
			>
		</div>
	{/if}
{/snippet}

<div class="sheet-root">
	<div class="formula-bar" data-region="formula">
		{#if tableBox}
			<div class="name-box split">
				<input
					bind:this={tableNameEl}
					class="table-name"
					class:default-name={isDefaultTableName(tableBox.name)}
					title={isDefaultTableName(tableBox.name) ? tr('grid.nameBox.table.default') : tr('grid.nameBox.table')}
					value={tableBox.name}
					onfocus={(e) => e.currentTarget.select()}
					onkeydown={onTableNameKey}
					spellcheck="false"
				/>
				<span class="slash">/</span>
				<input
					class="col-name"
					title={tr('grid.nameBox.column')}
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
				title={tr('grid.nameBox.cell')}
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
			aria-label={tr('grid.formulaBar.aria')}
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
				// Curseur déplacé : l'aperçu dans la cellule le suit (previewShift).
				if (fxEditing) schedule();
			}}
			onclick={(e) => {
				if (!fxLock) moveSig(e, 'fx');
				if (fxEditing) schedule();
			}}
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
			<span class="fx-scope" title={tr('grid.formulaBar.computed.tooltip')}
				>{tr('grid.formulaBar.computed', { n: calcEdit.e.t.rows })}</span
			>
		{/if}
		</div>
	</div>
	{#if bench}<div class="bench">{bench}</div>{/if}
	{#if completion?.where === 'fx' || sig?.where === 'fx' || (fxEditing && isFormula(fxText))}
		<div class="suggest fx-suggest">{@render suggestions('fx')}</div>
	{/if}

	<div class="body">
	<!-- Barre du plan (Données › Grouper) à gauche et au-dessus de la grille. -->
	<div class="ol-frame" style:padding-left="{olW}px" style:padding-top="{olH}px">
	{#if olW || olH}<OutlineBar bind:this={outlineBar} {outline} geom={outlineGeom} onfold={foldOutline} onlevel={outlineLevel} />{/if}
	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	<div class="wrap" bind:this={wrap} tabindex="0" role="grid" aria-label={tr('grid.sheet.aria', { name: sheetName })} aria-rowcount={-1} aria-colcount={-1} aria-activedescendant={a11yId} aria-keyshortcuts="Shift+F10" data-region-focus onkeydown={onKeyDown}>
		<GridA11y cellId={a11yId} {speech} row={active.row + 1} col={active.col + 1} editing={editing || fxEditing} selection={speechSel} />
		<!-- Calque zoomé (zoom.ts) : grille, graphiques, boutons, éditeur et notes ; menus et dialogues hors du calque. -->
		<div class="zoom" style:zoom={zoom === 1 ? undefined : zoom}>
			<canvas bind:this={canvas}></canvas>
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				class="scroller"
				class:grab={panCursor === 'grab'}
				class:grabbing={panCursor === 'grabbing'}
				bind:this={scroller}
				onscroll={onScroll}
				onpointerdown={onPointerDown}
				onpointermove={onPointerMove}
				onpointerup={onPointerUp}
				onpointercancel={onPointerUp}
				ontouchend={(e) => e.cancelable && e.preventDefault()}
				onmousedown={onMouseDown}
				onmousemove={onMouseMove}
				onmouseleave={() => ((noteTip = null), commentDlg?.preview && (commentDlg = null), hoverLocked >= 0 && ((hoverLocked = -1), schedule()))}
				ondblclick={onDblClick}
				oncontextmenu={onContextMenu}
			>
				<div bind:this={spacer}></div>
			</div>
			<!-- Calque découpé à la zone des cellules : les graphiques ne passent pas sur les barres de défilement. -->
			<!-- Main (pan.ts) : milieu, droit glissé ou Espace + gauche font aussi glisser la feuille depuis un objet. -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="charts" class:grabbing={panCursor === 'grabbing'} style:left="{MARGIN}px" style:top="{TOP_H}px" style:width="{Math.max(0, viewW - MARGIN)}px" style:height="{Math.max(0, viewH - TOP_H)}px" onmousedowncapture={objectPanDown}>
				{#each charts as ch (ch.id)}
					{@const box = chartBox(ch)}
					<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
					<div
						class="chart"
						data-chart-id={ch.id}
						role="group"
						aria-label={ch.config.title ? tr('grid.chart.aria.titled', { title: ch.config.title }) : tr('grid.chart.aria')}
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
						{#if ch.pending}
							<p class="chart-pending" aria-busy="true">{tr('grid.chart.computing')}</p>
						{:else if ch.error}
							<p class="chart-error">{tr('grid.chart.error', { error: ch.error })}</p>
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
				<SheetObjects
					bind:this={objectsLayer}
					objects={sheetObjects}
					geo={objGeo}
					{zoom}
					view={viewPos}
					active={() => active}
					gridFocused={() => document.activeElement === wrap && !editing}
					locked={!!locked || structure}
					imageData={source.imageData}
					oncreate={source.createObject}
					oncreateImage={source.createImage}
					onset={source.setObject}
					onplace={source.placeObject}
					onplaceLine={source.placeLine}
					onorder={source.orderObject}
					ondelete={source.deleteObject}
					onwheel={(ev) => onWheel(ev)}
					{say}
					done={() => wrap.focus()}
				/>
				<SheetSlicers
					{pivots}
					geo={{ colOffset: (c) => cax().offset(c), rowTop, colAt: (x) => cax().indexAt(x), rowAt: (y) => rax().indexAt(y) }}
					{zoom}
					view={viewPos}
					locked={!!locked || structure}
					onset={source.setPivot ? (id, cfg) => menuAction(() => source.setPivot!(id, cfg), true) : undefined}
					onwheel={(ev) => onWheel(ev)}
					{say}
				/>
				<SheetButtons
					bind:this={buttonsLayer}
					locked={!!locked || structure}
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
						<span class="editor-ro" aria-hidden="true">{tr('grid.editor.readOnly')}</span>
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
						onblur={(e) => {
							completion = sig = null;
							// Départ du focus (crossref.ts, editorLeave) : lecture seule refermée,
							// boîte « nom invalide » ou onglet de feuille pressé (saisie gardée),
							// barre de formule cliquée (la saisie y passe), retour de pointage
							// (le focus revient), sinon validée.
							const action = editorLeave({
								readOnly: !!editRO,
								alertOpen: nameAlertOpen(),
								toFx: !!fxInput && e.relatedTarget === fxInput,
								tabPress,
								returning: performance.now() < keepEditorUntil,
								text: editValueText
							});
							if (action === 'close') {
								closeReadOnly(false);
								return;
							}
							if (action === 'keep') return;
							if (action === 'fx') {
								// Clic dans la barre de formule (après avoir pointé une référence,
								// par exemple) : la saisie continue dans la barre, au point du clic,
								// sans être validée ; avant, elle était validée et le focus rendu à
								// la grille, et il fallait cliquer deux fois.
								handToFx();
								return;
							}
							if (action === 'refocus') {
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
							return e ? typeLabel(e.t.columns[active.col - e.t.col].type) : tr('grid.editor.freeCell');
						})()}
					></textarea>
				</div>
			{/if}
			{#each pinnedShown as p (p.x + ',' + p.y)}
				<div class="note-tip pinned" style:left="{toView(p.x)}px" style:top="{toView(p.y)}px">{p.text}</div>
			{/each}
			{#if noteTip && !noteDlg && !commentDlg}
				<div class="note-tip" style:left="{noteTip.x}px" style:top="{noteTip.y}px">{noteTip.text}</div>
			{/if}
			{#if inputTip && !noteTip}
				<div class="input-tip" role="tooltip" style:left="{inputTip.x}px" style:top="{inputTip.y}px">
					{#if inputTip.title}<b>{inputTip.title}</b>{/if}
					{#if inputTip.msg}<span>{inputTip.msg}</span>{/if}
				</div>
			{/if}
		</div>
		<FindBar v={findView} />
		<CellMenus v={cellMenusView} />
		<FollowMenu v={followMenuView} />
		<KeyHintMenu v={hintMenuView} />
		<LinesDialog v={linesView} />
		<KeyDialog v={keyDialogView} />
		<HandDialog v={handDialogView} />
		<ChartMenu v={chartMenuView} />
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
		{#if pasteDlg}
			{@const d = pasteDlg}
			<PasteSpecial
				internal={d.internal}
				formats={d.formats}
				onok={(o) => {
					// Lu avant de fermer la boîte (d suit pasteDlg).
					const { text, html } = d;
					pasteDlg = null;
					wrap.focus();
					if (!pasteWith(o, text, html)) say(tr('grid.pasteSpecial.empty'));
				}}
				onclose={() => {
					pasteDlg = null;
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
					say(table ? tr('grid.textCols.done.table', { n, table }) : tr('grid.textCols.done', { n }));
				}}
				onclose={() => {
					ttcDlg = null;
					wrap.focus();
				}}
			/>
		{/if}
		<DedupDialog v={dedupView} />
		<FunctionsDialog v={functionsView} />
		<PivotFilter ui={pivotFilterView} />
		<FilterPanel v={filterView} />
		{#if cfView}
			{#key cfView}
				<CondFormat host={cfHost} view={cfView} onclose={closeRules} />
			{/key}
		{/if}
		{#if linkDlg}
			{@const d = linkDlg}
			<LinkDialog
				text={d.text}
				textLocked={d.locked}
				target={d.target}
				sheets={data.sheets?.length ? data.sheets : [sheetName]}
				current={sheetName}
				names={d.names}
				onok={saveLink}
				onremove={removeLink}
				oncancel={() => {
					linkDlg = null;
					wrap.focus();
				}}
			/>
		{/if}
		{#if sparkDlg}
			{@const d = sparkDlg}
			<SparklineDialog
				config={d.config}
				columns={d.columns}
				editing={d.id !== undefined}
				onok={saveSparkline}
				onremove={removeSparkline}
				oncancel={() => {
					sparkDlg = null;
					wrap.focus();
				}}
			/>
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
		<ValidationDialog v={validationView} />
		<DvAlertBox v={dvAlertView} />
		{#if commentDlg && comments}
			{@const d = commentDlg}
			{@const host = comments}
			{@const th = threadAt(host.threads, d.row, d.col)}
			{#if th || !d.preview}
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div class="comment-pop" class:preview={d.preview} style:left="{d.x}px" style:top="{d.y}px" use:dismissPanel={() => commentDlg && !commentDlg.preview && ((commentDlg = null), wrap.focus())}>
					<CommentThread
						thread={th}
						place={threadPlace(d)}
						{host}
						preview={d.preview}
						onsend={(text) => host.add(d.row, d.col, text)}
						onclose={() => {
							commentDlg = null;
							wrap.focus();
						}}
					/>
				</div>
			{/if}
		{/if}
		<NoteEditor v={noteView} />
		<ListMenu ui={listMenuView} />
		<SizeDialog v={sizeView} />
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
	</div>
	{#if panel?.kind === 'pivot'}
		{@const pv = pivots.find((p) => p.id === panel!.id)}
		{#if pv}
			{#key pv.id}
			<PivotPanel
				config={pv.config}
				fields={sourceColumns(pv.config.source)}
				error={pv.error}
				filterValues={source.pivotValues || source.pivotFieldValues ? (f) => pivotValuesOf(pv.config, f) : undefined}
				onslicer={source.setPivot ? (f, tl) => addSlicer(pv, f, tl) : undefined}
				tabChoice={source.setTabChoice ? (f) => tabChoiceFor(pv.config.source, f, { field: true }) : undefined}
				ontabchoice={(f, it) => runTabChoice(pv.config.source, f, it)}
				customLists={source.customLists}
				setCustomLists={source.setCustomLists}
				onchange={(cfg) => menuAction(source.setPivot && (() => source.setPivot!(pv.id, cfg)), true)}
				calcPreview={source.previewPivotFormula ? (f) => source.previewPivotFormula!(pv.config.source, f) : undefined}
				oncalc={source.setPivotCalc ? (old, name, f) => source.setPivotCalc!(pv.id, old, name, f) : undefined}
				formulaHelp={{ functions: catalog?.functions ?? [], columns: sourceColumns(pv.config.source).map((c) => c.name) }}
				handNote={handKeptNote(pv.handKept, pv.handNote)}
				onforgethand={pv.handKept && source.forgetHandKept ? () => menuAction(() => source.forgetHandKept!({ name: pv.config.name ?? '' } as SheetTable), true) : undefined}
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
	/* Sous le champ Rechercher, alignés sur lui (après la flèche Remplacer). */
	/* Boutons d'icône sans cadre, comme ceux des volets. */
	/* Portée : la feuille, ou la sélection qui borne la recherche (verte). */
	/* « Options >> » : un lien discret, comme dans le dialogue d'un tableur. */
	.bench {
		padding: 4px 10px;
		background: #eef6f1;
		color: #14532d;
		border-bottom: 1px solid #d3e7dc;
		white-space: nowrap;
		overflow: hidden;
	}
	.ol-frame {
		position: relative;
		flex: 1;
		min-width: 0;
		min-height: 0;
		display: flex;
		flex-direction: column;
		box-sizing: border-box;
		background: #f5f6f7;
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
	/* Main (pan.ts) : Espace tenue, puis feuille saisie ; passe devant le curseur du survol. */
	.scroller.grab {
		cursor: grab !important;
	}
	.scroller.grabbing,
	.charts.grabbing :global(*) {
		cursor: grabbing !important;
	}
	.scroller {
		position: absolute;
		inset: 0;
		/* Au doigt, la grille gère elle-même défilement, appui et pincement (touch.ts). */
		touch-action: none;
		-webkit-touch-callout: none;
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
	.sig code {
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
	.filter {
		min-width: 300px;
	}
	/* Section « Type de données » du menu ▾ de la colonne. */
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
	/* Graphique d'un grand tableau, calculé après l'affichage : fond d'attente discret. */
	.chart-pending {
		margin: 0;
		height: 100%;
		box-sizing: border-box;
		padding: 12px;
		color: #5f6368;
		background: #f5f6f7;
		animation: chart-wait 1.6s ease-in-out infinite alternate;
	}
	@keyframes chart-wait {
		to {
			background: #eceef1;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.chart-pending {
			animation: none;
		}
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
	/* Indicateur de chargement (liste des valeurs, contrôle de la clé). */
	.loading {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 6px 10px;
		color: #6b7280;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
	/* Menu ▾ d'un TCD en arborescence : choix du champ des lignes (Excel). */
	/* Fond des boîtes modales de la grille (validation des données). */
	/* Validation des données : onglets, lignes libellé + champ, alerte. */
	.input-tip {
		position: absolute;
		z-index: 6;
		max-width: 240px;
		padding: 5px 8px;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
		white-space: pre-wrap;
		pointer-events: none;
		font-size: 12px;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	/* Plage source d'une validation pointée dans la grille : le dialogue retiré laisse cette barre. */
	/* Note : un post-it, sans fond blanc ni cadre de saisie ; les lignes
	   du papier suivent le texte (même interligne). */
	.comment-pop {
		position: absolute;
		z-index: 7;
	}
	.comment-pop.preview {
		pointer-events: none;
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
