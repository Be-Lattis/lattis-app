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
	import { RowCache, syncAt, type SyncKind } from './cache';
	import { VScroll } from './scroll';
	import { Axis } from './axis';
	import { allGenerated, defaultNameHint, isDefaultTableName, isGeneratedColumnName } from './defaultname';
	import { ease, reducedMotion, type AxisLike, type BarSpan, type Geo, type LinesShift, type Motion } from './anim';
	import { sweepAt, watchSource, WORK_DELAY, workAlpha, type WorkLookup, type WorkZone } from './working';
	import { followClip, overlaps, parseTSV, pasteArea, richClip, sameClipText, tileBlock, toTSV, type Clip, type Rect as ClipRect } from './clipboard';
	import { clipValues, hasClipStyles, parseClipHtml, toClipHtml, type ClipStyle } from './clipboardHtml';
	import { pasteOptions, plainPaste, type PasteKind, type PasteOptions } from './pastespecial';
	import PasteSpecial from './PasteSpecial.svelte';
	import { boxStyle, isNumericText, painterPatch, parseStyle, resolveStyle, type CellStyle, type StyleLayer } from './style';
	import { bumpDecimals, formatCell, shownCell } from './numfmt';
	import { FILL, plain } from './numcode';
	import { BorderSet, borderParts, type BorderCmd, type Edge, type StylePart } from './borders';
	import { BASE_PX, cellFont, fontPx, lineHeight, wrapLines } from './textwrap';
	import { AutoRows } from './rowfit';
	import { arrowSpan, spanBox, spanPoint, type ArrowGeometry } from './auditarrows';
	import { Velocity } from './touch';
	import { PanGesture, SpaceHold } from './pan';
	import { panes, toGrid, type Pane } from './panes';
	import { afterWidthChange, sideRoom } from './keepinview';
	import { dvPreview, hiddenLines, mergeAt, mergeList, validate, type DvList, type MergeRange } from './sheetfmt';
	import { DV_OP_KEYS, DV_OPS, DV_TYPES, dvRule, hasBounds, twoBounds, type DvAlert, type DvForm } from './dvrule';
	import { expandRange, intersect, isSingleCell, rangeOf, stepCell, wholeLines, type CellRange } from './mergesel';
	import { findZone, inZone, type Zone } from './findzone';
	import { type FindOpts, type FoundCell } from './findopts';
	import { blocksReach, boxesReach } from './objextent';
	import FormatDialog, { type FormatTab } from './FormatDialog.svelte';
	import TextToColumns from './TextToColumns.svelte';
	import CustomSort from './CustomSort.svelte';
	import LinkDialog from './LinkDialog.svelte';
	import CommentThread from './CommentThread.svelte';
	import { threadAt, threadPlace, type CommentHost } from './comments';
	import SparklineDialog from './SparklineDialog.svelte';
	import { a1 as sparkA1, defaultLocation, drawSparkline, freeSparkAt, sparkGeometry, sparkSvg, sparkValue, type SparkKind, type SparklineConfig, type SparklineGroup } from './sparkline';
	import { autoLink, isLinkClick, linkKind, onLinkText, openLink } from './hyperlink';
	import { type CustomOrder } from './customsort';
	import { type KeyChoice, type KeyQuestion } from './keychoice';
	import { dismiss, dismissPanel } from './dismiss';
	import { freeDisplay } from './freecell';
	import { bandLabel, bandOpen, hiddenBand } from './hidden';
	import { choiceHiddenCols } from '#lib/shell/choice.js';
	import { type ChoiceCandidate } from './tabchoice';
	import { placeFollowBadge, showNameBadge } from './followbadge';
	import { ReadRecovery, readFailNotice } from './readfail';
	import { followLabel } from '#lib/shell/mergesheets.js';
	import { bandMerges, groupSpans, headerLabel, isSingle, tableTop } from './twolevel';
	import { tableUnderTitle, titleAboveFilled } from './titlehead';
	import { pivotColLayer, pivotLayer, pivotValueText } from './pivotformat';
	import { canFoldAll, foldAll, foldTarget, pivotBoxHit, pivotBoxLeft, pivotLabelOffset, PIVOT_BOX, setColFolded, setFolded } from './pivottree';
	import { newSlicer, withSlicer } from './pivotslicer';
	import SheetSlicers from './SheetSlicers.svelte';
	import { citeLabel, type Citation } from './reflink';
	import { gridPalette } from './theme';
	import { paint as themed } from './colors';
	import { tableTheme } from './theme.svelte.js';
	import { condArgs, condLabel, condOps, describeConds, filterKind, kindTitle, searchValues, type CondOp, type PanelValue } from './filtercond';
	import { cellLines } from './shortcuts';
	import { SheetViews } from './sheetview';
	import { besideGutter, gutterOwner } from './gutter';
	import Submenu from '#lib/ui/Submenu.svelte';
	import { modal } from '#lib/ui/focus.js';
	import { clip, clipped } from '#lib/ui/menu.js';
	import Chart from '#lib/chart/Chart.svelte';
	import { encodeObj, objFromText, rememberObj } from '#lib/clip/objects.js';
	import ChartPanel from '#lib/chart/ChartPanel.svelte';
	import type { PrintCell, PrintChart, PrintSource } from '#lib/print/sheetprint.js';
	import PivotPanel from './PivotPanel.svelte';
	import { brokenPivotAt } from './pivotzone';
	import { midEllipsis } from './midellipsis';
	import { handAlign, handKeptNote, handStart } from './pivothand';
	import { type Placement } from './pivotplace';
	import SheetButtons from './SheetButtons.svelte';
	import SheetObjects from './SheetObjects.svelte';
	import { objectBox, type ObjectConfig, type SheetObject } from './objects';
	import { type SheetButton } from './buttons';
	import { parseRules, todaySerial, type CfOut, type Rule, type Stats } from './rules';
	import { chartMenu as chartMenuGroups, chartSubmenu, freeCellMenu, headerMenu, linesSubmenu, pivotSubmenu, tableCellMenu, tableColumnSubmenu, tableRowSubmenu } from './ctxmenu';
	import { type RuleChanges } from './cfmodel';
	import { ICON_SETS, drawCfIcon } from './cficons';
	import CondFormat, { type CondHost, type CondView } from './CondFormat.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import GridA11y from './GridA11y.svelte';
	import { type CellSpeech } from './a11y';
	import { drawIcon } from '#lib/icons/canvas.js';
	import { signatureParts, suggest, type Catalog, type Completion, type FunctionDoc } from './autocomplete';
	import { formulaRefs, highlightParts, isFormula, previewShift, refRect, type FormulaError, type Rect } from './formulaEdit';
	import { editorLeave, type FormulaHold } from './crossref';
	import OutlineBar from './OutlineBar.svelte';
	import { bandSize, outlineDepth, type SheetOutline } from './outline';
	import { nameAlertOpen } from '#lib/ui/nameAlert.js';
	import { isErrorValue, valueAlignment, alignment, editValue, formatColumnValue, NULL_MARK, showsNull, typeIcon, typeIconName, typeLabel, sortLabels, type ColumnType } from './format';
	import { errorText, t as tr } from '#lib/i18n/index.js'; // t est souvent un tableau ici : la traduction s'appelle tr
	import { type TypeMix, type PivotConfig, type SheetPivot, type ChartConfig, type SheetChart, type PageBreaksView, type SheetTable, type SheetData, type SheetSource, type SheetStatus, type Box, type TableAnim, type TextField, type HiddenMark, type Link, type Range, type Entry, pivotTotalRow, pivotRowOf, pivotBold, type Layer, type WholeSel, type DvPick, type InputTip } from './sheet/model';
	import { SHEET_ROWS, SHEET_COLS, ROW_H, DEF_W, TOP_H, MIN_MARGIN, GUTTER_PAD, GUTTER_MAX, GUTTER_KEY_MAX, FONT, FONT_BOLD, FONT_SMALL, FONT_TINY } from './sheet/consts';
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
	const cfCtl = createCondFormat({
		get cfStats() { return cfStats; },
		get cfStale() { return cfStale; },
		get cfGen() { return cfGen; },
		get cfWaits() { return cfWaits; },
		get schedule() { return schedule; },
		get key() { return key; },
		get shown() { return shown; },
		get cells() { return cells; },
		get pivots() { return pivots; },
		get pivotAt() { return pivotAt; },
		get sheetRowRules() { return sheetRowRules; },
		get sheetColRules() { return sheetColRules; },
		get tableAt() { return tableAt; },
		get cellRules() { return cellRules; },
		get entries() { return entries; },
		get colRules() { return colRules; },
		get source() { return source; },
		get cfToday() { return cfToday; },
		get freeMark() { return freeMark; },
		get noMark() { return noMark; },
		get isNumeric() { return isNumeric; },
		get selection() { return selection; },
		get selectionLines() { return selectionLines; },
		get locked() { return locked; },
		get say() { return say; },
		get closeMenus() { return closeMenus; },
		get cfView() { return cfView; }, set cfView(v) { cfView = v; },
		get selectRange() { return selectRange; },
		get wrap() { return wrap; }
	});
	const { cfAt, freeCf, sheetRules, selectionLocs, openRules, closeRules } = cfCtl;
	export const rulesMenu = cfCtl.rulesMenu;
	export const rulesActive = cfCtl.rulesActive;
	export const rulesTarget = cfCtl.rulesTarget;

	const noMark = () => false;
	/** Règle « formule » d'une cellule libre vérifiée (calculée par le classeur). */
	const freeMark = (row: number, col: number) => (r: Rule) => freeMarks.has(`${row},${col},${r.id ?? r.value}`);

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
		dvAfterData(switching);
		// Changement de feuille : garder la position de la feuille quittée,
		// retrouver celle de la nouvelle (A1 si elle n'a pas été visitée).
		const back = sheetViews.switchTo(d.sheet ?? 1, { active: { ...active }, anchor: { ...anchor }, top: vs.top, left: scroller?.scrollLeft ?? 0 });
		const old = new Map(entries.map((e) => [e.t.id, e]));
		fetchError = '';
		const placedBefore = placedSheet === (d.sheet ?? 0) ? placed() : null;
		// Navigation (ouverture, autre feuille) : rien ne s'anime.
		const nav = placedSheet !== (d.sheet ?? 0);
		if (nav) recovery.reset();
		// Recherche : occurrences relues après une saisie, un Ctrl+Z, une actualisation ou sur une autre feuille
		// (sauf dans tout le classeur : passer d'une feuille à l'autre ne les change pas).
		if (!nav || !finder?.opts.book) matchKey = '';
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
			// Pas encore calculé (LayoutFirst) : traité comme un TCD en erreur
			// (une seule cellule, sans contenu), avec son propre message.
			if (p.pending) p = { ...p, error: p.error || '…', cells: [] };
			const f = p.error ? 0 : (p.config.filters?.length ?? 0);
			return { ...p, anchor: p.row, row: p.row + (f ? f + 1 : 0) + (p.error ? 0 : (p.colHead?.length ?? 0)) };
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
		sheetObjects = d.objects ?? [];
		sparkGroups = d.sparklines ?? [];
		controlsTick++;
		spills = d.cells.filter((c) => c.spill?.length).map((c) => ({ row: c.row, col: c.col, rows: c.spill! }));
		shown = new Map(d.cells.filter((c) => c.raw.startsWith('=')).map((c) => [key(c.row, c.col), { text: c.value ?? '', error: !!c.error }]));
		freeLinks = new Map(d.cells.filter((c) => c.link).map((c) => [key(c.row, c.col), c.link!]));
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
		outline = d.outline ?? null;
		styleHiddenRows = hiddenLines(sheetRowStyles);
		hiddenRows = new Set([...styleHiddenRows, ...(outline?.hiddenRows ?? [])]);
		styleHiddenCols = hiddenLines(sheetColStyles);
		// Onglet à choix sur une valeur : la colonne de catégorie (la même
		// partout) n'est pas affichée ; elle revient avec « (Toutes) ».
		hiddenCols = new Set([...styleHiddenCols, ...(outline?.hiddenCols ?? []), ...choiceHiddenCols(d.choice, d.tables, d.cells)]);
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
		// Occurrence trouvée sur une autre feuille : arrivé, elle se sélectionne.
		findArrived(d.sheet ?? 0);
		// Lien suivi vers une autre feuille : arrivé, sa destination se sélectionne.
		const go = linkGo;
		if (go && (d.sheet ?? 0) === go.sheet) {
			linkGo = null;
			selectPlace(go);
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
	// --- Mesure du texte : sheet/measure.svelte.ts ---
	const measureCtl = createMeasure({
		get measurer() { return measurer; }, set measurer(v) { measurer = v; },
		get source() { return source; },
		get entries() { return entries; },
		get MAX_FIT() { return MAX_FIT; },
		get pivots() { return pivots; },
		get userWidths() { return userWidths; },
		get hiddenCols() { return hiddenCols; },
		get colAxis() { return colAxis; }, set colAxis(v) { colAxis = v; },
		get MAX_AUTO_H() { return MAX_AUTO_H; },
		get cax() { return cax; },
		get merges() { return merges; },
		get cells() { return cells; },
		get shown() { return shown; },
		get styleAt() { return styleAt; },
		get sheetRowStyles() { return sheetRowStyles; },
		get colStyles() { return colStyles; },
		get sheetColStyles() { return sheetColStyles; },
		get singleCol() { return singleCol; },
		get rh() { return rh; },
		get bandStyle() { return bandStyle; },
		get headIconW() { return headIconW; },
		get HEAD_ICON_W() { return HEAD_ICON_W; },
		get visibleRows() { return visibleRows; },
		get autoRows() { return autoRows; },
		get fRows() { return fRows; },
		get userHeights() { return userHeights; },
		get hiddenRows() { return hiddenRows; },
		get rowAxis() { return rowAxis; }, set rowAxis(v) { rowAxis = v; },
		get updateExtent() { return updateExtent; }
	});
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
	const animCtl = createAnim({
		get animNow() { return animNow; }, set animNow(v) { animNow = v; },
		get animBusy() { return animBusy; }, set animBusy(v) { animBusy = v; },
		get rowDraw() { return rowDraw; }, set rowDraw(v) { rowDraw = v; },
		get rowAxis() { return rowAxis; },
		get colDraw() { return colDraw; }, set colDraw(v) { colDraw = v; },
		get colAxis() { return colAxis; },
		get lines() { return lines; }, set lines(v) { lines = v; },
		get motion() { return motion; },
		get drawing() { return drawing; }, set drawing(v) { drawing = v; },
		get geoDrawn() { return geoDrawn; }, set geoDrawn(v) { geoDrawn = v; },
		get grownSeen() { return grownSeen; }, set grownSeen(v) { grownSeen = v; },
		get valsNow() { return valsNow; }, set valsNow(v) { valsNow = v; },
		get barsNow() { return barsNow; }, set barsNow(v) { barsNow = v; },
		get flashes() { return flashes; },
		get barAnims() { return barAnims; },
		get motions() { return motions; },
		get inner() { return inner; },
		get valsPrev() { return valsPrev; }, set valsPrev(v) { valsPrev = v; },
		get barsPrev() { return barsPrev; }, set barsPrev(v) { barsPrev = v; },
		get viewPrev() { return viewPrev; }, set viewPrev(v) { viewPrev = v; },
		get viewNow() { return viewNow; }, set viewNow(v) { viewNow = v; },
		get entries() { return entries; },
		get GROW_MS() { return GROW_MS; },
		get schedule() { return schedule; },
		get navAt() { return navAt; }, set navAt(v) { navAt = v; },
		get NAV_SETTLE() { return NAV_SETTLE; },
		get linesHint() { return linesHint; }, set linesHint(v) { linesHint = v; },
		get editAt() { return editAt; }, set editAt(v) { editAt = v; },
		get chartTween() { return chartTween; }, set chartTween(v) { chartTween = v; },
		get grownCharts() { return grownCharts; }, set grownCharts(v) { grownCharts = v; },
		get charts() { return charts; },
		get viewH() { return viewH; },
		get viewW() { return viewW; },
		get rh() { return rh; },
		get cax() { return cax; },
		get rspan() { return rspan; },
		get topRow() { return topRow; },
		get rowTop() { return rowTop; },
		get tableAnimsAt() { return tableAnimsAt; }, set tableAnimsAt(v) { tableAnimsAt = v; },
		get tableAnims() { return tableAnims; }, set tableAnims(v) { tableAnims = v; },
		get noOff() { return noOff; },
		get ctx() { return ctx; },
		get EDIT_WINDOW() { return EDIT_WINDOW; },
		get columnFlash() { return columnFlash; }, set columnFlash(v) { columnFlash = v; },
		get typed() { return typed; },
		get boxOf() { return boxOf; },
		get active() { return active; },
		get locked() { return locked; },
		get structure() { return structure; },
		get keptOut() { return keptOut; },
		get inRangeZone() { return inRangeZone; }
	});
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
	const formulaCtl = createFormula({
		get editErr() { return editErr; }, set editErr(v) { editErr = v; },
		get fxText() { return fxText; }, set fxText(v) { fxText = v; },
		get placeEditor() { return placeEditor; },
		get catalog() { return catalog; },
		get suggestPos() { return suggestPos; }, set suggestPos(v) { suggestPos = v; },
		get toView() { return toView; },
		get screenX() { return screenX; },
		get active() { return active; }, set active(v) { active = v; },
		get screenY() { return screenY; },
		get editorH() { return editorH; },
		get rh() { return rh; },
		get tableAt() { return tableAt; },
		get completion() { return completion; }, set completion(v) { completion = v; },
		get sig() { return sig; }, set sig(v) { sig = v; },
		get hold() { return hold; }, set hold(v) { hold = v; },
		get fxEditing() { return fxEditing; }, set fxEditing(v) { fxEditing = v; },
		get fxInput() { return fxInput; },
		get editing() { return editing; }, set editing(v) { editing = v; },
		get editRO() { return editRO; },
		get input() { return input; },
		get editValueText() { return editValueText; },
		get lastRef() { return lastRef; }, set lastRef(v) { lastRef = v; },
		get placedSheet() { return placedSheet; },
		get say() { return say; },
		get anchor() { return anchor; }, set anchor(v) { anchor = v; },
		get refreshFx() { return refreshFx; },
		get wrap() { return wrap; },
		get schedule() { return schedule; },
		get startEdit() { return startEdit; },
		get keepEditorUntil() { return keepEditorUntil; }, set keepEditorUntil(v) { keepEditorUntil = v; },
		get commit() { return commit; },
		get moveTo() { return moveTo; },
		get onsheet() { return onsheet; }
	});
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

	function draw() {
		frame = 0;
		borderSet.clear();
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
		pivotBadges = [];
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
			curPane = p;
			if (drawCells(layer(p))) incomplete = true;
			curPane = null;
			ctx.restore();
		}
		if (incomplete) missingFrames++;
		drawJumpFlash(t0);
		drawWorking(t0);
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
		drawFollowBadges();
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
		drawValidation();

		const seen = new Set<string>();
		const pins = pinnedFrame.filter((p) => !seen.has(p.x + ',' + p.y) && !!seen.add(p.x + ',' + p.y));
		if (JSON.stringify(pins) !== JSON.stringify(pinnedShown)) pinnedShown = pins;
		// Une clé plus large que toutes celles vues : gouttière élargie à
		// l'image suivante (les clés ne sont connues qu'une fois chargées).
		if (keyGrew) schedule();
		outlineBar?.paint();
		endAnimFrame();

		const t1 = performance.now();
		pushStat(drawTimes, t1 - t0);
		frameStamps.push(t1);
		while (frameStamps.length && frameStamps[0] < t1 - 1000) frameStamps.shift();
	}

	/** Ce qu'un volet affiche : lignes et colonnes visibles, positions à l'écran. */
	function layer(p: Pane): Layer {
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

	/**
	 * Graphiques sparkline des cellules visibles du volet : ceux d'un tableau
	 * lisent les lignes déjà chargées (rien n'est demandé de plus), ceux des
	 * cellules libres leurs valeurs données avec la mise en page.
	 */
	function drawSparklines(L: Layer) {
		if (!sparkGroups.length) return;
		const { r0, r1, c0, c1, cx, ry } = L;
		const one = (g: SparklineGroup, values: (number | null)[], r: number, c: number) => {
			const w = cax().size(c);
			const h = rh(r);
			if (!w || !h) return;
			const x = cx(c);
			const y = ry(r);
			ctx.save();
			ctx.beginPath();
			ctx.rect(x, y, w, h);
			ctx.clip();
			drawSparkline(ctx, sparkGeometry(values, g.config, w, h, g.config.same ? { lo: g.lo, hi: g.hi } : undefined), x, y, w);
			ctx.restore();
		};
		for (const g of sparkGroups) {
			if (g.error) continue;
			if (g.config.table) {
				const e = entries.find((x) => x.t.name === g.config.table);
				if (!e || !g.src || g.atCol < c0 || g.atCol > c1) continue;
				for (let r = Math.max(r0, e.t.row + 1); r <= Math.min(r1, e.last); r++) {
					const row = e.cache.row(r - e.t.row - 1);
					if (row) one(g, g.src.map((i) => sparkValue(row.cells[i])), r, g.atCol);
				}
				continue;
			}
			for (const s of g.cells ?? []) if (s.row >= r0 && s.row <= r1 && s.col >= c0 && s.col <= c1) one(g, s.values ?? [], s.row, s.col);
		}
	}

	/** Groupe sparkline dessiné en (r, c), et ses valeurs (lignes de tableau chargées). */
	function sparkAt(r: number, c: number): { g: SparklineGroup; values: (number | null)[] } | null {
		const f = freeSparkAt(sparkGroups, r, c);
		if (f) return f;
		for (const g of sparkGroups) {
			if (!g.config.table || g.atCol !== c || !g.src) continue;
			const e = entries.find((x) => x.t.name === g.config.table);
			if (!e || r <= e.t.row || r > e.last) continue;
			const row = e.cache.row(r - e.t.row - 1);
			return { g, values: row ? g.src.map((i) => sparkValue(row.cells[i])) : [] };
		}
		return null;
	}

	/** Cellule imprimée avec son graphique sparkline (SVG). */
	function printSpark(cell: PrintCell | undefined, r: number, c: number): PrintCell | undefined {
		if (!sparkGroups.length) return cell;
		const s = sparkAt(r, c);
		if (!s || s.g.error) return cell;
		const w = cax().size(c);
		const h = rh(r);
		const svg = sparkSvg(sparkGeometry(s.values, s.g.config, w, h, s.g.config.same ? { lo: s.g.lo, hi: s.g.hi } : undefined), w, h);
		return { ...(cell ?? { text: '' }), spark: svg };
	}

	// Insertion › Sparklines : dialogue (groupe neuf ou celui de la cellule active).
	let sparkDlg = $state<{ config: SparklineConfig; columns: string[]; id?: number } | null>(null);
	export function insertSparkline(kind: SparkKind) {
		if (!source.createSparkline) return;
		const here = sparkAt(active.row, active.col);
		if (here) {
			const e = here.g.config.table ? entries.find((x) => x.t.name === here.g.config.table) : undefined;
			sparkDlg = { config: { ...here.g.config }, columns: e?.t.columns.map((k) => k.name) ?? [], id: here.g.id };
			return;
		}
		const { r0, r1, c0, c1 } = selection();
		const e = tableAt(r0, c0);
		if (e && r0 > e.t.row && c1 <= e.lastCol) {
			// Dans un tableau : une colonne de graphiques, les colonnes choisies de chaque ligne.
			const cols = e.t.columns.slice(c0 - e.t.col, c1 - e.t.col + 1).map((k) => k.name);
			const all = e.t.columns.map((k) => k.name);
			const numeric = e.t.columns.filter((k) => k.type === 'integer' || k.type === 'decimal').map((k) => k.name);
			const nums = cols.length > 1 ? cols : numeric.length ? numeric : all.slice(1);
			sparkDlg = { config: { kind, table: e.t.name, cols: nums }, columns: all };
			return;
		}
		sparkDlg = { config: { kind, source: sparkA1(r0, c0, r1, c1), location: defaultLocation(r0, c0, r1, c1) }, columns: [] };
	}
	async function saveSparkline(cfg: SparklineConfig) {
		const d = sparkDlg;
		if (!d) return;
		if (d.id !== undefined) await source.setSparkline?.(d.id, cfg);
		else await source.createSparkline?.(cfg);
		sparkDlg = null;
		wrap.focus();
	}
	async function removeSparkline() {
		const d = sparkDlg;
		if (d?.id !== undefined) await source.deleteSparkline?.(d.id);
		sparkDlg = null;
		wrap.focus();
	}

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
		if (finder && matches.length && matchKey === matchesKey(finder.text, finder.zone, finder.opts)) {
			ctx.fillStyle = C.found;
			const here = data.sheet ?? 0;
			for (const m of matches)
				if (m.sheet === here && m.row >= r0 && m.row <= r1 && m.col >= c0 && m.col <= c1) ctx.fillRect(cx(m.col), ry(m.row), cax().size(m.col), rh(m.row));
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
							ctx.fillStyle = themed(hs.bg);
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
						// Nom donné par Lattis (« Colonne n ») : gris italique, à remplacer.
						const gen = isGeneratedColumnName(col.name);
						headLabel(headerLabel(col.name, e.t.twoLevel), x, y, cw, rh(r), iconW + 10, gen ? { ...hs, i: true } : hs, gen ? C.unnamedBadge : e.t.query && !col.hand ? C.linkHeadText : C.headText, 'left');
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
						ctx.fillStyle = themed(cs.bg);
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
					// Case NULL d'une colonne saisie : tiret gris centré (dessiné seulement).
					if (showsNull(col.type, v, row.bad?.has(i))) {
						drawText(NULL_MARK, x, y, cw, 'center', undefined, C.nullMark);
						continue;
					}
					if (!s) continue;
					// Jeu d'icônes : à gauche de la cellule ; barre ou icône seule : valeur masquée.
					const iconW = cf?.icon && !row.bad?.has(i) ? drawIconAt(cf.icon, x, y, rh(r)) : 0;
					if (cf?.hide && !row.bad?.has(i)) continue;
					let ds = cs;
					if (cs && !row.bad?.has(i)) [s, ds] = codeShown(s, row.cells[i], cs);
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
					drawText(s, x + iconW, y, cw - iconW, align, ds, col.type === 'formula' && isErrorValue(v) ? C.error : C.text);
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
				ctx.fillStyle = themed(fs.bg);
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
		drawSparklines(L);
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
			if (p.pending) {
				// Grand tableau source : calculé après l'affichage de la feuille.
				drawText(tr('grid.pivot.computing'), cx(p.col), ry(p.row), cax().size(p.col), 'left', undefined, C.stripText, w);
				continue;
			}
			if (p.error) {
				drawText(tr('grid.pivot.error', { error: p.error }), cx(p.col), ry(p.row), cax().size(p.col), 'left', undefined, C.error, w);
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
			if (pr1 < r0 || pivotHead(p) > r1 || pc1 < c0 || p.col > c1) continue;
			// En-têtes imbriqués (plusieurs champs en colonnes), au-dessus de l'en-tête.
			drawPivotColHead(p, pc1, r0, r1, { cx, ry, spanW });
			// Ligne du total général : la dernière, quand il est affiché.
			const tot = pivotTotalRow(p);
			// En-tête et total sur fond gris, sauf remplissage choisi.
			// Colonnes saisies à droite (pivothand.ts) : en-tête vert, comme un tableau ordinaire.
			const hs = handStart(p);
			for (const r of tot >= 0 ? [p.row, tot] : [p.row])
				for (let c = p.col; c <= pc1; c++)
					if (!styleAt(r, c, undefined)?.bg) {
						ctx.fillStyle = r === p.row && c >= hs ? C.head : C.linkHead; // comme les tableaux d'une requête : des données calculées
						ctx.fillRect(cx(c), ry(r), cax().size(c), rh(r));
					}
			for (let i = Math.max(0, r0 - p.row); i < p.cells.length && p.row + i <= r1; i++)
				p.cells[i].forEach((v, j) => {
					const c = p.col + j;
					// Arborescence : retrait du niveau et case −/+ des groupes (colonne des étiquettes).
					const info = j === 0 ? pivotRowOf(p, p.row + i) : undefined;
					if (info?.group) pivotFoldBox(cx(c) + pivotBoxLeft(info.depth), ry(p.row + i), rh(p.row + i), !!info.collapsed);
					// Groupe de colonnes plié : case + devant son étiquette.
					const folded = i === 0 && !!p.cols?.[j]?.collapsed;
					if (folded) pivotFoldBox(cx(c) + pivotBoxLeft(0), ry(p.row), rh(p.row), true);
					if (!v) return;
					const bold = pivotBold(p, p.row + i);
					// Mise en forme posée sur les cellules du TCD, leurs lignes ou colonnes (format de nombre, décimales, police…).
					const st = resolveStyle(styleAt(p.row + i, c, undefined), freeCf(p.row + i, c)?.style);
					const style = bold ? { ...st, b: st?.b ?? true } : st;
					const ind = pivotLabelOffset(info) + (folded ? PIVOT_BOX + 5 : 0);
					// Recalculé : seules les cellules dont la valeur change s'éclairent.
					paintFlash(flashOf(`p${p.id}:${i},${j}`, v, p.row + i, c), cx(c), ry(p.row + i), cax().size(c), rh(p.row + i));
					drawText(pivotValueText(formatCell(v, undefined, st), i, j), cx(c) + ind, ry(p.row + i), cax().size(c) - ind, st?.al ?? (c >= hs && i > 0 ? handAlign(v) : j > 0 && i > 0 ? 'right' : 'left'), style, i === 0 && c >= hs ? C.headText : i === 0 || p.row + i === tot ? C.linkHeadText : C.text);
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
			ctx.strokeRect(Math.round(cx(p.col)) + 0.5, Math.round(ry(pivotHead(p))) + 0.5, spanW(p.col, pc1) - 1, rspan(pivotHead(p), pr1) - 1);
			// Traits sous l'en-tête et au-dessus du total.
			ctx.beginPath();
			for (const y of tot >= 0 ? [ry(p.row + 1), ry(tot)] : [ry(p.row + 1)]) {
				ctx.moveTo(Math.round(cx(p.col)), Math.round(y) + 0.5);
				ctx.lineTo(Math.round(cx(p.col)) + spanW(p.col, pc1), Math.round(y) + 0.5);
			}
			// Trait entre le TCD calculé et ses colonnes saisies.
			if (hs > p.col && hs <= pc1) {
				ctx.moveTo(Math.round(cx(hs)) + 0.5, Math.round(ry(p.row)));
				ctx.lineTo(Math.round(cx(hs)) + 0.5, Math.round(ry(p.row)) + rspan(p.row, pr1));
			}
			ctx.stroke();
			ctx.restore();
			// Pastille du nom (TCD1), comme celle d'un tableau : un TCD se lit
			// dans les formules comme un tableau nommé (TCD1[Nombre]). Toujours
			// montrée : l'onglet « TCD 1 » ne porte pas son nom. Un clic ouvre le volet.
			if (curPane && p.config.name) {
				ctx.font = FONT_SMALL;
				const bh = 13;
				const bx = Math.round(cx(p.col)) - 0.5;
				const by = Math.round(ry(pivotHead(p))) - 0.5;
				let aboveFree = true;
				for (let r = pivotHead(p) - 1; aboveFree && r >= 0 && ry(r + 1) > by - bh; r--) {
					if (!rh(r)) continue;
					for (let c = p.col; c <= pc1; c++) {
						const v = cells.get(key(r, c));
						if ((v !== undefined && v !== '') || tableAt(r, c) || (pivotAt(r, c) && pivotAt(r, c) !== p) || mergeOf(r, c)) {
							aboveFree = false;
							break;
						}
					}
				}
				const fw = spanW(p.col, pc1);
				const label = fitText(p.config.name, fw - 10);
				const bw = Math.ceil(ctx.measureText(label).width) + 10;
				const pp = curPane;
				if (fw >= 30) {
					const b = placeFollowBadge({ x: bx, y: by, w: fw, bw, bh, paneTop: pp.y0, strip: pp.y0 === TOP_H, aboveFree, left: true });
					pivotBadges.push({ p, label, radii: b.radii, x0: b.x, x1: b.x + bw, y0: b.y, y1: b.y + bh, clip: { x0: pp.x0, x1: pp.x1, y0: pp.y0 === TOP_H ? 0 : pp.y0, y1: pp.y1 } });
				}
			}
		}
		// Texte aligné à gauche qui déborde (comme Excel) : seulement sur les
		// cellules voisines vides ; il est coupé à la première cellule occupée
		// (valeur, tableau, tableau croisé, fusion).
		const overflowW = (r: number, c: number): number => {
			let ow = cax().size(c);
			for (let cc = c + 1; cc <= c1 + 1; cc++) {
				const v = cells.get(key(r, cc));
				if ((v !== undefined && v !== '') || tableAt(r, cc) || pivotAt(r, cc) || mergeOf(r, cc) || spilledAt(r, cc)) break;
				ow += cax().size(cc);
			}
			return ow;
		};
		for (const [k, rawText] of cells) {
			const [r, c] = k.split(',').map(Number);
			if (r < r0 || r > r1 || c < c0 || c > c1 || !rh(r) || !cax().size(c) || mergeOf(r, c)) continue;
			// Cellule en cours d'édition : l'éditeur montre sa saisie ; sa valeur,
			// qui peut déborder plus loin que lui, ne se dessine pas dessous.
			// Même chose pour une saisie dans la barre de formule, montrée dans la cellule.
			if ((editing || (fxEditing && !hold)) && r === active.row && c === active.col) continue;
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
			const [raw, dstyle] = fd.kind === 'text' ? [fd.text, fstyle] : codeShown(fd.text, undefined, fstyle);
			const boxed = fd.align !== 'left';
			const spread = boxed ? cw : overflowW(r, c) - (x - cx(c));
			if (fstyle || raw.includes('\n')) {
				drawText(raw, x, ry(r), cw, fstyle?.al ?? fd.align, dstyle, f?.error ? C.error : C.text, spread);
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
				ctx.rect(cx(p.col) + 1, ry(pivotHead(p)) + 1, spanW(p.col, pc1) - 2, rspan(pivotHead(p), p.row + p.cells.length - 1) - 2);
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
			// Pastilles du cadre, au-dessus de lui s'il y a la place (bandeau du
			// haut compris pour un tableau en ligne 1), sinon dans son coin
			// (followbadge.ts) ; dessinées après le bandeau, dans la découpe du
			// volet. À gauche, le nom du tableau (cliquer : le renommer) ; à
			// droite, la valeur de l'onglet à choix qu'il suit.
			if (curPane) {
				ctx.font = FONT_SMALL;
				const bh = 13;
				const top = topRow(e.t);
				let aboveFree = true;
				for (let r = top - 1; aboveFree && r >= 0 && ry(r + 1) > y - bh; r--) {
					if (!rh(r)) continue;
					for (let c = e.t.col; c <= e.lastCol; c++) {
						const v = cells.get(key(r, c));
						if ((v !== undefined && v !== '') || tableAt(r, c) || pivotAt(r, c) || mergeOf(r, c)) {
							aboveFree = false;
							break;
						}
					}
				}
				const p = curPane;
				const clip = { x0: p.x0, x1: p.x1, y0: p.y0 === TOP_H ? 0 : p.y0, y1: p.y1 };
				let room = fw;
				if (e.t.follows && data?.choice && !data.choice.all) {
					const label = data.choice.value || tr('grid.follow.empty');
					const bw = Math.ceil(ctx.measureText(label).width) + 10;
					const b = placeFollowBadge({ x, y, w: fw, bw, bh, paneTop: p.y0, strip: p.y0 === TOP_H, aboveFree });
					followBadges.push({ kind: 'follow', e, label, radii: b.radii, x0: b.x, x1: b.x + bw, y0: b.y, y1: b.y + bh, clip });
					room -= bw + 4;
				}
				// Classeur collaboratif : tableau resté sur ce poste (pas dans
				// le classeur partagé) ; un clic propose de l'y ajouter.
				if (e.t.shared === 'private' && onaddshared) {
					const label = tr('grid.local.badge');
					const bw = Math.ceil(ctx.measureText(label).width) + 10;
					if (room >= bw + 30) {
						const b = placeFollowBadge({ x, y, w: room, bw, bh, paneTop: p.y0, strip: p.y0 === TOP_H, aboveFree });
						followBadges.push({ kind: 'local', e, label, radii: b.radii, x0: b.x, x1: b.x + bw, y0: b.y, y1: b.y + bh, clip });
						room -= bw + 4;
					}
				}
				if (e.t.name && room >= 30 && showNameBadge(e.t.name, sheetName)) {
					const dflt = isDefaultTableName(e.t.name);
					const label = fitText(e.t.name + (dflt ? ' ✎' : ''), room - 10);
					const bw = Math.ceil(ctx.measureText(label).width) + 10;
					const b = placeFollowBadge({ x, y, w: fw, bw, bh, paneTop: p.y0, strip: p.y0 === TOP_H, aboveFree, left: true });
					followBadges.push({ kind: dflt ? 'unnamed' : 'name', e, label, radii: b.radii, x0: b.x, x1: b.x + bw, y0: b.y, y1: b.y + bh, clip });
				}
			}
			// Classeur collaboratif : lecture filtrée par les droits (§8.1).
			if (e.t.filtered) {
				ctx.font = 'italic ' + FONT_SMALL;
				ctx.fillStyle = C.icon;
				ctx.textAlign = 'right';
				ctx.textBaseline = 'top';
				ctx.fillText(tr('grid.rights.filtered'), x + fw, y + fh + 2);
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
		// Pose d'un tableau croisé : contour de l'emplacement sous la souris.
		if (placing && placeHover) {
			const { row, col } = placeHover;
			ctx.save();
			ctx.setLineDash([5, 3]);
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 2;
			ctx.strokeRect(cx(col) + 1, ry(row) + 1, spanW(col, col + placeSize.w - 1) - 2, rspan(row, row + placeSize.h - 1) - 2);
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
		// Plage source d'une validation en cours de pointage : toute la colonne
		// (ou la plage) teintée et encadrée, comme une colonne calculée en cours
		// d'édition.
		if (dvPick && dvPickRef) {
			const k = refRect(dvPickRef, refContext().tables, sheetName);
			if (k && k.r1 >= r0 - 1 && k.r0 <= r1 + 1) {
				const a = Math.max(k.r0, r0);
				const b = Math.min(k.r1, r1);
				ctx.fillStyle = C.calc;
				ctx.fillRect(cx(k.c0), ry(a), spanW(k.c0, k.c1), rspan(a, b));
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
				// Colonne de référence : petit bouton ↗ « Basculer sur » (aussi Ctrl+clic).
				if (!many && refAt(active.row, active.col)) {
					const l = linkButton();
					ctx.fillStyle = C.strip;
					ctx.fillRect(l.x, l.y, l.w, l.h);
					ctx.strokeStyle = C.stripLine;
					ctx.strokeRect(l.x + 0.5, l.y + 0.5, l.w - 1, l.h - 1);
					ctx.save();
					ctx.font = FONT_SMALL;
					ctx.textAlign = 'center';
					ctx.fillStyle = C.stripText;
					ctx.fillText('↗', l.x + l.w / 2, l.y + l.h / 2 + 1);
					ctx.restore();
				}
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
			// Coupée au bord de la cellule (ou de la fusion), sur fond opaque : la
			// saisie ne passe plus sur les valeurs voisines (une formule qui déverse,
			// demande de Louis) ; le texte défile pour garder le curseur visible.
			ctx.font = FONT;
			const caret = fxInput?.selectionEnd ?? fxText.length;
			const shift = previewShift(ctx.measureText(fxText.slice(0, caret)).width, ctx.measureText(fxText).width, cw - 12);
			ctx.save();
			ctx.beginPath();
			const left = Math.max(x, MARGIN);
			const top = Math.max(y, TOP_H);
			ctx.rect(left, top, x + cw - left, y + ch - top);
			ctx.clip();
			drawText(fxText, x - shift, y, cw + shift, 'left', undefined, C.text, cw + shift, ch);
			ctx.restore();
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
		drawCommentMarks(L);
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
				let label = rowLabel(e, r);
				if (e.t.key !== undefined) {
					label = midEllipsis(label, GUTTER_KEY_MAX - GUTTER_PAD, (s) => ctx.measureText(s).width);
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
		for (const [i0, i1] of hiddenRuns(styleHiddenRows)) {
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
			ctx.fillStyle = themed(st?.bg) ?? C.bg;
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
				const [mt, mst] = codeShown(raw, undefined, st);
				drawText(mt, x, y, gw, st?.al ?? (num ? 'right' : 'left'), mst, f?.error ? C.error : C.text, gw, gh);
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
				ctx.fillStyle = themed(st?.bg) ?? (e.t.query ? C.linkHead : C.head);
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
			ctx.fillStyle = themed(hs.bg);
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
		ctx.fillStyle = themed(hs?.fg) ?? color;
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

	/** Première ligne des en-têtes d'un TCD (au-dessus de p.row avec plusieurs champs en colonnes). */
	function pivotHead(p: SheetPivot): number {
		return p.row - (p.error ? 0 : (p.colHead?.length ?? 0));
	}

	/** En-têtes imbriqués d'un TCD : étiquette d'un groupe de colonnes au-dessus de sa première colonne, avec sa case −. */
	function drawPivotColHead(p: SheetPivot, pc1: number, r0: number, r1: number, g: { cx: (c: number) => number; ry: (r: number) => number; spanW: (c0: number, c1: number) => number }) {
		const { cx, ry, spanW } = g;
		const head = p.colHead ?? [];
		head.forEach((row, d) => {
			const hr = row ?? [];
			const r = pivotHead(p) + d;
			if (r < r0 || r > r1 || !rh(r)) return;
			for (let c = p.col; c <= pc1; c++)
				if (!styleAt(r, c, undefined)?.bg) {
					ctx.fillStyle = C.linkHead;
					ctx.fillRect(cx(c), ry(r), cax().size(c), rh(r));
				}
			hr.forEach((v, j) => {
				if (!v) return;
				const c = p.col + j;
				let end = j + 1;
				while (end < hr.length && !hr[end] && (p.cols?.[end]?.path?.length ?? 0) > d + 1 && p.cols?.[end]?.path?.[d] === v) end++;
				pivotFoldBox(cx(c) + pivotBoxLeft(0), ry(r), rh(r), false);
				const ind = pivotBoxLeft(0) + PIVOT_BOX + 1;
				const st = resolveStyle(styleAt(r, c, undefined), undefined);
				drawText(v, cx(c) + ind, ry(r), spanW(c, Math.min(pc1, p.col + end - 1)) - ind, 'left', { ...st, b: st?.b ?? true }, C.linkHeadText);
			});
		});
	}

	/** Clic sur la case −/+ d'un groupe de colonnes : le plie ou le déplie. */
	function toggleColFold(ev: MouseEvent, p: SheetPivot, cell: { row: number; col: number }): boolean {
		if (!p.cols || !source.setPivot || cell.row < pivotHead(p) || cell.row > p.row) return false;
		const j = cell.col - p.col;
		const d = cell.row - pivotHead(p);
		const info = p.cols[j];
		const x = toLocal(ev).x - screenX(cell.col);
		if (!info?.path || x < pivotBoxLeft(0) - 3 || x > pivotBoxLeft(0) + PIVOT_BOX + 3) return false;
		let path: string[] | null = null;
		let fold = false;
		if (cell.row === p.row && info.collapsed) path = info.path;
		else if (cell.row < p.row && p.colHead?.[d]?.[j] && info.path.length > d + 1) {
			path = info.path.slice(0, d + 1);
			fold = true;
		}
		if (!path) return false;
		moveTo(cell.row, cell.col);
		const set = source.setPivot;
		const cfg = setColFolded(p.config, path, fold);
		menuAction(() => set(p.id, cfg));
		return true;
	}

	/** Nouveau segment (ou chronologie) du champ field, posé à droite du TCD. */
	function addSlicer(p: SheetPivot, field: string, timeline: boolean) {
		const set = source.setPivot;
		if (!set) return;
		const s = newSlicer(p.config, field, p.anchor ?? p.row, p.col + (p.cells[0]?.length ?? 1) - 1, timeline);
		menuAction(() => set(p.id, withSlicer(p.config, p.config.slicers?.length ?? 0, s)), true);
	}

	/** Valeurs d'un champ d'un TCD (groupes et champs calculés compris). */
	function pivotValuesOf(cfg: PivotConfig, field: string): Promise<{ text: string; count: number }[]> {
		if (source.pivotValues) return source.pivotValues(cfg, field);
		return source.pivotFieldValues?.(cfg.source, field) ?? Promise.resolve([]);
	}

	/** Le clic tombe-t-il sur une case −/+ de TCD ? Plie ou déplie alors le groupe (une étape d'annulation). */
	function togglePivotFold(ev: MouseEvent, cell: { row: number; col: number }): boolean {
		const pc = pivotAt(cell.row, cell.col);
		if (pc && toggleColFold(ev, pc, cell)) return true;
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
		if (!values.length) return tr('grid.pivot.filter.all');
		if (values.length === 1) return values[0] === '' ? tr('grid.filter.blanks') : values[0];
		return tr('grid.pivot.filter.several');
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

	/** Pastilles posées pendant le dessin des cadres, par-dessus le bandeau du haut. */
	function drawFollowBadges() {
		for (const b of [...followBadges, ...pivotBadges.map((x) => ({ ...x, kind: 'pivot' as const }))]) {
			ctx.save();
			ctx.beginPath();
			ctx.rect(Math.max(b.clip.x0, MARGIN), b.clip.y0, b.clip.x1 - Math.max(b.clip.x0, MARGIN), b.clip.y1 - b.clip.y0);
			ctx.clip();
			ctx.fillStyle = b.kind === 'pivot' ? C.sourceBadge : b.kind === 'follow' ? C.followBadge : b.kind === 'name' ? C.nameBadge : b.kind === 'local' ? C.localBadge : C.unnamedBadge;
			ctx.beginPath();
			ctx.roundRect(b.x0, b.y0, b.x1 - b.x0, b.y1 - b.y0, b.radii);
			ctx.fill();
			ctx.font = FONT_SMALL;
			ctx.fillStyle = '#fff';
			ctx.textAlign = 'center';
			ctx.textBaseline = 'middle';
			ctx.fillText(b.label, (b.x0 + b.x1) / 2, (b.y0 + b.y1) / 2 + 0.5);
			ctx.restore();
		}
	}
	let followMenu = $state<{ x: number; y: number; e: Entry } | null>(null);

	function badgeAt(ev: MouseEvent) {
		const { x, y } = toLocal(ev);
		return followBadges.find((b) => x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1) ?? null;
	}
	function followBadgeAt(ev: MouseEvent): Entry | null {
		const b = badgeAt(ev);
		return b?.kind === 'follow' ? b.e : null;
	}
	/** Pastille du nom d'un tableau sous la souris : son tableau. */
	function nameBadgeAt(ev: MouseEvent): Entry | null {
		const b = badgeAt(ev);
		return b && (b.kind === 'name' || b.kind === 'unnamed') ? b.e : null;
	}
	/** Pastille du nom d'un tableau croisé sous la souris : son TCD. */
	function pivotBadgeAt(ev: MouseEvent): SheetPivot | null {
		const { x, y } = toLocal(ev);
		return pivotBadges.find((b) => x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1)?.p ?? null;
	}
	/** Pastille « sur ce poste » sous la souris : son tableau. */
	function localBadgeAt(ev: MouseEvent): Entry | null {
		const b = badgeAt(ev);
		return b?.kind === 'local' ? b.e : null;
	}

	/** Texte coupé (…) pour tenir en w pixels (police courante). */
	function fitText(t: string, w: number): string {
		if (ctx.measureText(t).width <= w) return t;
		let s = t;
		while (s.length > 1 && ctx.measureText(s + '…').width > w) s = s.slice(0, -1);
		return s + '…';
	}

	/** Clic sur la pastille du nom : la cellule passe sur l'en-tête du tableau et le champ du nom s'ouvre. */
	async function renameFromBadge(en: Entry) {
		moveTo(en.t.row, en.t.col);
		ensureVisible(en.t.row, en.t.col);
		await tick();
		tableNameEl?.focus();
		tableNameEl?.select();
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
				say(errorText(err));
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
					say(tr('grid.key.set', { name, table: t.name }));
				} catch (err) {
					say(errorText(err));
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
		// Icônes dans la couleur du tableau : celle des sources externes pour une requête.
		const ic = e.t.query && !col.hand ? C.linkIcon : C.icon;
		ctx.font = FONT_SMALL;
		ctx.fillStyle = ic;
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
		ctx.fillStyle = ic;
		if (svg) {
			drawIcon(ctx, svg, x + cw - 6 - iconW - ICON, y + (h - ICON) / 2, ICON, ic, C.active, 1.3);
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
			drawIcon(ctx, share, x + cw - 6 - iconW - ICON - 2, y + (h - ICON) / 2, ICON, ic, C.active, 1.3);
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
			ctx.fillStyle = ic;
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
	// le même trait), pour qu'il ne les recouvre pas (borders.ts : style et
	// couleur de chaque bord).
	const borderSet = new BorderSet();

	/** Bordures d'une cellule (style bt, bb, bl, br). */
	function borders(st: CellStyle, x: number, y: number, w: number, h: number) {
		borderSet.add(st, x, y, w, h);
	}

	function strokeBorders() {
		if (borderSet.empty) return;
		ctx.save();
		ctx.beginPath();
		ctx.rect(MARGIN, TOP_H, viewW - MARGIN, viewH - TOP_H);
		ctx.clip();
		borderSet.stroke(ctx, (c) => (c ? themed(c) : C.text), C.bg);
		ctx.restore();
	}

	/** Texte affiché et style d'une cellule : la couleur d'un code de format ([Rouge]) remplace celle du texte. */
	function codeShown(text: string, value: unknown, st: CellStyle | undefined): [string, CellStyle | undefined] {
		if (!st?.nc) return [formatCell(text, value, st), st];
		const r = shownCell(text, value, st);
		return [r.text, r.color ? { ...st, fg: r.color } : st];
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
		// Lien hypertexte : bleu (sauf couleur choisie) et souligné.
		const link = !!st?.ln && !!linkKind(st.ln);
		ctx.fillStyle = themed(st?.fg) ?? (link ? C.link : color);
		const tx = align === 'left' ? x + 6 : align === 'right' ? x + cw - 6 : x + cw / 2;
		const px = fontPx(st?.fs);
		const lh = lineHeight(px);
		// Renvoi à la ligne (largeur de la cellule), ou sauts de ligne (Alt+Entrée) :
		// les lignes qui tiennent dans la hauteur.
		// Remplissage (*x d'un code, comptabilité) : la suite du texte va à droite.
		const fillAt = st?.wr ? -1 : s.indexOf(FILL);
		if (st?.wr) s = plain(s);
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
			if (k === 0 && fillAt >= 0) {
				const after = Array.from(line.slice(fillAt + 1)).slice(1).join('');
				ctx.textAlign = 'left';
				ctx.fillText(line.slice(0, fillAt), x + 6, cy);
				ctx.textAlign = 'right';
				ctx.fillText(after, x + cw - 6, cy);
				return;
			}
			ctx.fillText(line, tx, cy);
			if (st?.u || st?.x || link) {
				const w = ctx.measureText(line).width;
				const lx = align === 'left' ? tx : align === 'right' ? tx - w : tx - w / 2;
				if (st?.u || link) ctx.fillRect(lx, cy + Math.round(px * 0.55), w, 1);
				if (st?.x) ctx.fillRect(lx, cy, w, 1);
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
	const outlineCtl = createOutline({
		get olW() { return olW; },
		get olH() { return olH; },
		get resizeCanvas() { return resizeCanvas; },
		get mounted() { return mounted; },
		get zoom() { return zoom; },
		get MARGIN() { return MARGIN; },
		get rh() { return rh; },
		get screenY() { return screenY; },
		get cax() { return cax; },
		get screenX() { return screenX; },
		get source() { return source; },
		get menuAction() { return menuAction; },
		get selection() { return selection; },
		get wholeKind() { return wholeKind; },
		get outline() { return outline; },
		get active() { return active; },
		get say() { return say; },
		get styleHiddenRows() { return styleHiddenRows; },
		get styleHiddenCols() { return styleHiddenCols; },
		get setHidden() { return setHidden; },
		get closeMenus() { return closeMenus; },
		get sizeDlg() { return sizeDlg; }, set sizeDlg(v) { sizeDlg = v; },
		get wrap() { return wrap; },
		get userHeights() { return userHeights; },
		get rebuildRows() { return rebuildRows; },
		get updateExtent() { return updateExtent; },
		get schedule() { return schedule; },
		get hiddenCols() { return hiddenCols; },
		get autoFitColumns() { return autoFitColumns; },
		get rowTop() { return rowTop; },
		get vs() { return vs; },
		get viewW() { return viewW; },
		get scroller() { return scroller; },
		get syncNative() { return syncNative; }
	});
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
	const validationCtl = createValidation({
		get tableAt() { return tableAt; },
		get styleAt() { return styleAt; },
		get dvLists() { return dvLists; },
		get dvGen() { return dvGen; },
		get source() { return source; },
		get active() { return active; }, set active(v) { active = v; },
		get mergeOf() { return mergeOf; },
		get screenX() { return screenX; },
		get cax() { return cax; },
		get screenY() { return screenY; },
		get rowTop() { return rowTop; },
		get rh() { return rh; },
		get locked() { return locked; },
		get cells() { return cells; },
		get key() { return key; },
		get keptOut() { return keptOut; },
		get tableHintBox() { return tableHintBox; },
		get toLocal() { return toLocal; },
		get closeMenus() { return closeMenus; },
		get listMenu() { return listMenu; }, set listMenu(v) { listMenu = v; },
		get toView() { return toView; },
		get wrap() { return wrap; },
		get say() { return say; },
		get cellInfo() { return cellInfo; },
		get write() { return write; },
		get onreflink() { return onreflink; },
		get sources() { return sources; },
		get shownText() { return shownText; },
		get sheetName() { return sheetName; },
		get addrA1() { return addrA1; },
		get refItems() { return refItems; }, set refItems(v) { refItems = v; },
		get menu() { return menu; }, set menu(v) { menu = v; },
		get validDlg() { return validDlg; }, set validDlg(v) { validDlg = v; },
		get selection() { return selection; },
		get wholeKind() { return wholeKind; },
		get previewTimer() { return previewTimer; }, set previewTimer(v) { previewTimer = v; },
		get applyStyle() { return applyStyle; },
		get dvPick() { return dvPick; }, set dvPick(v) { dvPick = v; },
		get anchor() { return anchor; }, set anchor(v) { anchor = v; },
		get wholeSel() { return wholeSel; }, set wholeSel(v) { wholeSel = v; },
		get sheetId() { return sheetId; },
		get dvPickRef() { return dvPickRef; }, set dvPickRef(v) { dvPickRef = v; },
		get refTables() { return refTables; },
		get onsheet() { return onsheet; },
		get ensureVisible() { return ensureVisible; },
		get schedule() { return schedule; },
		get refreshStatus() { return refreshStatus; },
		get dvAlert() { return dvAlert; }, set dvAlert(v) { dvAlert = v; },
		get startEdit() { return startEdit; },
		get ctx() { return ctx; },
		get MARGIN() { return MARGIN; },
		get viewW() { return viewW; },
		get viewH() { return viewH; },
		get inputTip() { return inputTip; }, set inputTip(v) { inputTip = v; }
	});
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

	function drawWorking(now: number) {
		if (!working.length) return;
		const anim = motion();
		const rFirst = rowAtY(TOP_H + rowTop(fRows()));
		const cFirst = colAtX(MARGIN + cax().offset(fCols()));
		const rLast = rowAtY(viewH - 1);
		const cLast = colAtX(viewW - 1);
		ctx.save();
		ctx.beginPath();
		ctx.rect(MARGIN, TOP_H, viewW - MARGIN, viewH - TOP_H);
		ctx.clip();
		for (const w of working) {
			const a = anim ? workAlpha(now, w.t0) : workAlpha(now, w.t0) > 0 ? 1 : 0;
			if (anim && a < 1) animBusy = true;
			if (a <= 0) continue;
			for (const z of w.zones) {
				// Bornée à la vue : une colonne d'un million de lignes, des lignes entières.
				const r0 = z.r0 >= fRows() ? Math.max(z.r0, rFirst - 1) : z.r0;
				const c0 = z.c0 >= fCols() ? Math.max(z.c0, cFirst - 1) : z.c0;
				const r1 = Math.min(z.r1, rLast + 1);
				const c1 = Math.min(z.c1, cLast + 1);
				if (r0 > r1 || c0 > c1) continue;
				const x = screenX(c0);
				const y = screenY(r0);
				const zw = cax().offset(c1 + 1) - cax().offset(c0);
				const zh = rowTop(r1 + 1) - rowTop(r0);
				if (zw <= 0 || zh <= 0) continue;
				ctx.fillStyle = `rgba(15, 108, 189, ${(0.08 * a).toFixed(3)})`;
				ctx.fillRect(x, y, zw, zh);
				if (anim) {
					// Bande lumineuse qui parcourt la zone dans sa longueur.
					const tall = zh > zw * 1.5;
					const len = tall ? zh : zw;
					const band = Math.max(24, Math.min(180, len * 0.45));
					const at = (tall ? y : x) - band + sweepAt(now, w.t0) * (len + 2 * band);
					const g = tall ? ctx.createLinearGradient(0, at - band, 0, at + band) : ctx.createLinearGradient(at - band, 0, at + band, 0);
					g.addColorStop(0, 'rgba(15, 108, 189, 0)');
					g.addColorStop(0.5, `rgba(15, 108, 189, ${(0.2 * a).toFixed(3)})`);
					g.addColorStop(1, 'rgba(15, 108, 189, 0)');
					ctx.fillStyle = g;
					ctx.fillRect(x, y, zw, zh);
					ctx.setLineDash([6, 4]);
					ctx.lineDashOffset = -((now - w.t0) / 40) % 10;
					animBusy = true;
				}
				ctx.strokeStyle = `rgba(15, 108, 189, ${(0.7 * a).toFixed(3)})`;
				ctx.lineWidth = 1.5;
				ctx.strokeRect(x + 0.75, y + 0.75, zw - 1.5, zh - 1.5);
				ctx.setLineDash([]);
			}
		}
		ctx.restore();
	}

	function drawJumpFlash(now: number) {
		const f = jumpFlash;
		if (!f) return;
		const a = motion() ? 1 - Math.min(1, (now - f.t) / JUMP_FLASH_MS) : 1;
		if (a <= 0) return;
		const x = screenX(f.c0);
		const w = cax().offset(f.c1 + 1) - cax().offset(f.c0);
		ctx.save();
		ctx.beginPath();
		ctx.rect(MARGIN, TOP_H, viewW - MARGIN, viewH - TOP_H);
		ctx.clip();
		ctx.fillStyle = `rgba(255, 196, 0, ${(0.35 * a).toFixed(3)})`;
		ctx.fillRect(x, screenY(f.r), w, rh(f.r));
		ctx.restore();
		if (motion()) animBusy = true;
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
	const notesCtl = createNotes({
		get mergeOf() { return mergeOf; },
		get styleAt() { return styleAt; },
		get noteDlg() { return noteDlg; }, set noteDlg(v) { noteDlg = v; },
		get noteTip() { return noteTip; }, set noteTip(v) { noteTip = v; },
		get key() { return key; },
		get screenX() { return screenX; },
		get cax() { return cax; },
		get viewW() { return viewW; },
		get screenY() { return screenY; },
		get source() { return source; },
		get locked() { return locked; },
		get say() { return say; },
		get closeMenus() { return closeMenus; },
		get active() { return active; },
		get toView() { return toView; },
		get wrap() { return wrap; },
		get menuAction() { return menuAction; },
		get copyFill() { return copyFill; },
		get clearSelection() { return clearSelection; },
		get lastRepeat() { return lastRepeat; }, set lastRepeat(v) { lastRepeat = v; },
		get tableAt() { return tableAt; },
		get cells() { return cells; },
		get inRangeZone() { return inRangeZone; },
		get moveTo() { return moveTo; },
		get comments() { return comments; },
		get mounted() { return mounted; },
		get schedule() { return schedule; },
		get refreshStatus() { return refreshStatus; },
		get ctx() { return ctx; },
		get commentDlg() { return commentDlg; }, set commentDlg(v) { commentDlg = v; }
	});
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
	const statusCtl = createStatus({
		get tableAt() { return tableAt; },
		get groupBandAt() { return groupBandAt; },
		get singleCol() { return singleCol; },
		get cells() { return cells; },
		get key() { return key; },
		get names() { return names; },
		get ownerAt() { return ownerAt; },
		get syncFinderZone() { return syncFinderZone; },
		get dvPick() { return dvPick; },
		get dvPickRef() { return dvPickRef; }, set dvPickRef(v) { dvPickRef = v; },
		get pickedRef() { return pickedRef; },
		get prefetchDv() { return prefetchDv; },
		get message() { return message; },
		get shownMessage() { return shownMessage; }, set shownMessage(v) { shownMessage = v; },
		get messageTimer() { return messageTimer; }, set messageTimer(v) { messageTimer = v; },
		get clearMessage() { return clearMessage; },
		get entries() { return entries; },
		get active() { return active; },
		get onstatus() { return onstatus; },
		get noteOf() { return noteOf; },
		get activeThread() { return activeThread; },
		get ms() { return ms; },
		get drawTimes() { return drawTimes; },
		get frameStamps() { return frameStamps; },
		get lastEditMs() { return lastEditMs; },
		get selection() { return selection; },
		get activeStyle() { return activeStyle; },
		get mergedAt() { return mergedAt; },
		get styleAt() { return styleAt; },
		get source() { return source; },
		get statsEpoch() { return statsEpoch; },
		get shown() { return shown; },
		get isNumeric() { return isNumeric; },
		get singleSelection() { return singleSelection; },
		get pivotAt() { return pivotAt; }
	});
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
	const filtersCtl = createFilters({
		get pivotAt() { return pivotAt; },
		get source() { return source; },
		get screenX() { return screenX; },
		get cax() { return cax; },
		get toLocal() { return toLocal; },
		get PIVOT_DROP() { return PIVOT_DROP; },
		get moveTo() { return moveTo; },
		get closeMenus() { return closeMenus; },
		get pivotDlg() { return pivotDlg; }, set pivotDlg(v) { pivotDlg = v; },
		get toView() { return toView; },
		get scroller() { return scroller; },
		get screenY() { return screenY; },
		get rh() { return rh; },
		get pivotValuesOf() { return pivotValuesOf; },
		get say() { return say; },
		get menuAction() { return menuAction; },
		get menu() { return menu; }, set menu(v) { menu = v; },
		get filterDlg() { return filterDlg; }, set filterDlg(v) { filterDlg = v; },
		get customDlg() { return customDlg; }, set customDlg(v) { customDlg = v; },
		get targetTable() { return targetTable; },
		get targetCol() { return targetCol; },
		get zoom() { return zoom; },
		get mixOf() { return mixOf; },
		get ownerAt() { return ownerAt; },
		get active() { return active; },
		get wrap() { return wrap; },
		get filterClosed() { return filterClosed; }, set filterClosed(v) { filterClosed = v; }
	});
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
	const chartsCtl = createCharts({
		get selChart() { return selChart; }, set selChart(v) { selChart = v; },
		get panel() { return panel; }, set panel(v) { panel = v; },
		get wrap() { return wrap; },
		get source() { return source; },
		get cax() { return cax; },
		get rax() { return rax; },
		get writing() { return writing; }, set writing(v) { writing = v; },
		get rowTop() { return rowTop; },
		get say() { return say; },
		get entries() { return entries; },
		get sources() { return sources; },
		get CHART_W() { return CHART_W; },
		get CHART_H() { return CHART_H; },
		get locked() { return locked; },
		get structure() { return structure; },
		get zoomK() { return zoomK; },
		get hold() { return hold; },
		get closeMenus() { return closeMenus; },
		get chartMenu() { return chartMenu; }, set chartMenu(v) { chartMenu = v; },
		get chartClip() { return chartClip; }, set chartClip(v) { chartClip = v; },
		get charts() { return charts; },
		get updateExtent() { return updateExtent; },
		get vs() { return vs; },
		get fRows() { return fRows; },
		get scroller() { return scroller; },
		get viewW() { return viewW; },
		get MARGIN() { return MARGIN; },
		get fCols() { return fCols; },
		get schedule() { return schedule; },
		get syncNative() { return syncNative; },
		get motion() { return motion; },
		get colAxis() { return colAxis; },
		get rowAxis() { return rowAxis; },
		get onrowlink() { return onrowlink; },
		get active() { return active; },
		get pivotAt() { return pivotAt; },
		get targetTable() { return targetTable; },
		get selection() { return selection; },
		get targetCol() { return targetCol; }
	});
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
	const pivotCtl = createPivot({
		get pivotIn() { return pivotIn; },
		get pivotZone() { return pivotZone; },
		get entries() { return entries; },
		get topRow() { return topRow; },
		get pivots() { return pivots; },
		get cells() { return cells; },
		get panel() { return panel; }, set panel(v) { panel = v; },
		get source() { return source; },
		get menuAction() { return menuAction; },
		get say() { return say; },
		get pivotSourcePick() { return pivotSourcePick; }, set pivotSourcePick(v) { pivotSourcePick = v; },
		get sourceColumns() { return sourceColumns; },
		get freeMenu() { return freeMenu; }, set freeMenu(v) { freeMenu = v; },
		get active() { return active; }, set active(v) { active = v; },
		get anchor() { return anchor; }, set anchor(v) { anchor = v; },
		get pivotAt() { return pivotAt; },
		get targetTable() { return targetTable; },
		get targetCol() { return targetCol; },
		get openPivotOnLoad() { return openPivotOnLoad; }, set openPivotOnLoad(v) { openPivotOnLoad = v; },
		get writing() { return writing; }, set writing(v) { writing = v; },
		get placing() { return placing; }, set placing(v) { placing = v; },
		get closeMenus() { return closeMenus; },
		get menu() { return menu; }, set menu(v) { menu = v; },
		get placeHover() { return placeHover; }, set placeHover(v) { placeHover = v; },
		get placeSize() { return placeSize; }, set placeSize(v) { placeSize = v; },
		get onplacing() { return onplacing; },
		get wrap() { return wrap; },
		get schedule() { return schedule; },
		get scroller() { return scroller; },
		get sheetId() { return sheetId; },
		get message() { return message; }, set message(v) { message = v; },
		get refreshFx() { return refreshFx; },
		get refreshStatus() { return refreshStatus; }
	});
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
	const dataToolsCtl = createDataTools({
		get ownerAt() { return ownerAt; },
		get active() { return active; },
		get selectionLines() { return selectionLines; },
		get entries() { return entries; },
		get selection() { return selection; },
		get source() { return source; },
		get locked() { return locked; },
		get say() { return say; },
		get closeMenus() { return closeMenus; },
		get ttcDlg() { return ttcDlg; }, set ttcDlg(v) { ttcDlg = v; },
		get menuAction() { return menuAction; },
		get cells() { return cells; },
		get dedupDlg() { return dedupDlg; }, set dedupDlg(v) { dedupDlg = v; },
		get wrap() { return wrap; }
	});
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
	const commandsCtl = createCommands({
		get fnDlg() { return fnDlg; }, set fnDlg(v) { fnDlg = v; },
		get wrap() { return wrap; },
		get startEdit() { return startEdit; },
		get input() { return input; },
		get onFormulaInput() { return onFormulaInput; },
		get active() { return active; },
		get tableAt() { return tableAt; },
		get entries() { return entries; },
		get catalog() { return catalog; },
		get targetTable() { return targetTable; },
		get source() { return source; },
		get say() { return say; },
		get menuAction() { return menuAction; },
		get targetCol() { return targetCol; },
		get selection() { return selection; },
		get lastCol() { return lastCol; },
		get lastRow() { return lastRow; },
		get lastRepeat() { return lastRepeat; }, set lastRepeat(v) { lastRepeat = v; },
		get hintLines() { return hintLines; },
		get keyDlg() { return keyDlg; }, set keyDlg(v) { keyDlg = v; },
		get write() { return write; },
		get reopenFormula() { return reopenFormula; },
		get handDlg() { return handDlg; }, set handDlg(v) { handDlg = v; },
		get menu() { return menu; }, set menu(v) { menu = v; },
		get toView() { return toView; },
		get screenX() { return screenX; },
		get screenY() { return screenY; },
		get rh() { return rh; },
		get linesDlg() { return linesDlg; }, set linesDlg(v) { linesDlg = v; },
		get wholeKind() { return wholeKind; },
		get queryIn() { return queryIn; },
		get QUERY_READONLY() { return QUERY_READONLY; },
		get writing() { return writing; }, set writing(v) { writing = v; },
		get schedule() { return schedule; },
		get filterDlg() { return filterDlg; }, set filterDlg(v) { filterDlg = v; },
		get cfView() { return cfView; }, set cfView(v) { cfView = v; },
		get dedupDlg() { return dedupDlg; }, set dedupDlg(v) { dedupDlg = v; },
		get freeMenu() { return freeMenu; }, set freeMenu(v) { freeMenu = v; },
		get chartMenu() { return chartMenu; }, set chartMenu(v) { chartMenu = v; },
		get formatDlg() { return formatDlg; }, set formatDlg(v) { formatDlg = v; },
		get validDlg() { return validDlg; }, set validDlg(v) { validDlg = v; },
		get dvAlert() { return dvAlert; },
		get noteDlg() { return noteDlg; }, set noteDlg(v) { noteDlg = v; },
		get listMenu() { return listMenu; }, set listMenu(v) { listMenu = v; },
		get sizeDlg() { return sizeDlg; }, set sizeDlg(v) { sizeDlg = v; },
		get commentDlg() { return commentDlg; }, set commentDlg(v) { commentDlg = v; },
		get data() { return data; },
		get choiceCands() { return choiceCands; },
		get followMenu() { return followMenu; }, set followMenu(v) { followMenu = v; },
		get pivotDlg() { return pivotDlg; }, set pivotDlg(v) { pivotDlg = v; },
		get dvPick() { return dvPick; },
		get noteTip() { return noteTip; }, set noteTip(v) { noteTip = v; },
		get moveTo() { return moveTo; },
		get anchor() { return anchor; }, set anchor(v) { anchor = v; },
		get ensureVisible() { return ensureVisible; },
		get refreshFx() { return refreshFx; },
		get refreshStatus() { return refreshStatus; },
		get scroller() { return scroller; },
		get zoomK() { return zoomK; },
		get cax() { return cax; },
		get objectsLayer() { return objectsLayer; },
		get buttonsLayer() { return buttonsLayer; },
		get pivots() { return pivots; }
	});
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
	const auditCtl = createAudit({
		get active() { return active; },
		get anchor() { return anchor; },
		get links() { return links; }, set links(v) { links = v; },
		get schedule() { return schedule; },
		get source() { return source; },
		get selection() { return selection; },
		get visibleRows() { return visibleRows; },
		get data() { return data; },
		get closeMenus() { return closeMenus; },
		get selChart() { return selChart; }, set selChart(v) { selChart = v; },
		get panel() { return panel; }, set panel(v) { panel = v; },
		get buttonsLayer() { return buttonsLayer; },
		get objectsLayer() { return objectsLayer; },
		get tableAt() { return tableAt; },
		get shown() { return shown; },
		get key() { return key; },
		get cells() { return cells; },
		get nameBox() { return nameBox; }, set nameBox(v) { nameBox = v; },
		get cellName() { return cellName; },
		get message() { return message; }, set message(v) { message = v; },
		get pivotAt() { return pivotAt; },
		get pivots() { return pivots; },
		get pivotPanelAt() { return pivotPanelAt; }, set pivotPanelAt(v) { pivotPanelAt = v; },
		get openPivotOnLoad() { return openPivotOnLoad; },
		get ownerAt() { return ownerAt; },
		get singleCol() { return singleCol; },
		get singleSelection() { return singleSelection; },
		get inTable() { return inTable; },
		get tableBox() { return tableBox; }, set tableBox(v) { tableBox = v; },
		get dragging() { return dragging; },
		get headerDrag() { return headerDrag; },
		get fxEditing() { return fxEditing; },
		get fxRO() { return fxRO; }, set fxRO(v) { fxRO = v; },
		get readOnlyReason() { return readOnlyReason; },
		get fxText() { return fxText; }, set fxText(v) { fxText = v; },
		get editText() { return editText; },
		get addrA1() { return addrA1; },
		get shownText() { return shownText; },
		get names() { return names; },
		get speech() { return speech; }, set speech(v) { speech = v; },
		get speechSel() { return speechSel; }, set speechSel(v) { speechSel = v; },
		get refreshStatus() { return refreshStatus; }
	});
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
	const scrollCtl = createScrolling({
		get holdCols() { return holdCols; },
		get growAtEnd() { return growAtEnd; },
		get scroller() { return scroller; },
		get updateExtent() { return updateExtent; },
		get schedule() { return schedule; },
		get toLocal() { return toLocal; },
		get holdRows() { return holdRows; }, set holdRows(v) { holdRows = v; },
		get rax() { return rax; },
		get vs() { return vs; },
		get space() { return space; },
		get editing() { return editing; },
		get fxEditing() { return fxEditing; },
		get pan() { return pan; },
		get wrap() { return wrap; },
		get holdWhileScrolling() { return holdWhileScrolling; },
		get noteTip() { return noteTip; }, set noteTip(v) { noteTip = v; },
		get dragging() { return dragging; }, set dragging(v) { dragging = v; },
		get headerDrag() { return headerDrag; }, set headerDrag(v) { headerDrag = v; },
		get panCursor() { return panCursor; }, set panCursor(v) { panCursor = v; },
		get zoomK() { return zoomK; },
		get onContextMenu() { return onContextMenu; },
		get locked() { return locked; },
		get hold() { return hold; },
		get dvPick() { return dvPick; },
		get startEdit() { return startEdit; },
		get onzoom() { return onzoom; },
		get zoom() { return zoom; },
		get touch() { return touch; }, set touch(v) { touch = v; },
		get cellAt() { return cellAt; },
		get active() { return active; },
		get selChart() { return selChart; },
		get listMenu() { return listMenu; },
		get onDblClick() { return onDblClick; },
		get zoomWheel() { return zoomWheel; }, set zoomWheel(v) { zoomWheel = v; },
		get rowTop() { return rowTop; },
		get cax() { return cax; },
		get spacer() { return spacer; },
		get viewW() { return viewW; },
		get MARGIN() { return MARGIN; },
		get extentCols() { return extentCols; },
		get motion() { return motion; },
		get boxOf() { return boxOf; },
		get charts() { return charts; },
		get chartBox() { return chartBox; },
		get panel() { return panel; },
		get pivots() { return pivots; },
		get anchor() { return anchor; },
		get fCols() { return fCols; },
		get fRows() { return fRows; }
	});
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
	const keyboardCtl = createKeyboard({
		get boxOf() { return boxOf; },
		get active() { return active; }, set active(v) { active = v; },
		get step() { return step; },
		get anchor() { return anchor; }, set anchor(v) { anchor = v; },
		get hiddenRows() { return hiddenRows; },
		get hiddenCols() { return hiddenCols; },
		get message() { return message; }, set message(v) { message = v; },
		get ensureVisible() { return ensureVisible; },
		get refreshFx() { return refreshFx; },
		get refreshStatus() { return refreshStatus; },
		get entries() { return entries; },
		get topRow() { return topRow; },
		get cells() { return cells; },
		get spills() { return spills; },
		get pivots() { return pivots; },
		get toLocal() { return toLocal; },
		get MARGIN() { return MARGIN; },
		get scroller() { return scroller; },
		get rowAtY() { return rowAtY; },
		get colAtX() { return colAtX; },
		get startEdit() { return startEdit; },
		get autoSum() { return autoSum; },
		get openFormat() { return openFormat; },
		get tableAt() { return tableAt; },
		get schedule() { return schedule; },
		get activeRect() { return activeRect; },
		get selectRows() { return selectRows; },
		get toggleFilters() { return toggleFilters; },
		get editActive() { return editActive; },
		get source() { return source; },
		get applyStyle() { return applyStyle; },
		get hideLines() { return hideLines; },
		get editNote() { return editNote; },
		get newComment() { return newComment; },
		get openFunctions() { return openFunctions; },
		get chartActive() { return chartActive; },
		get pasteSpecial() { return pasteSpecial; },
		get outlineAction() { return outlineAction; },
		get lastRepeat() { return lastRepeat; }, set lastRepeat(v) { lastRepeat = v; },
		get say() { return say; },
		get selection() { return selection; },
		get cellInfo() { return cellInfo; },
		get writing() { return writing; }, set writing(v) { writing = v; },
		get noAnimUntil() { return noAnimUntil; }, set noAnimUntil(v) { noAnimUntil = v; },
		get editing() { return editing; },
		get spaceKey() { return spaceKey; },
		get dvPick() { return dvPick; },
		get endPick() { return endPick; },
		get locked() { return locked; },
		get hold() { return hold; },
		get keyContextMenu() { return keyContextMenu; },
		get painter() { return painter; }, set painter(v) { painter = v; },
		get links() { return links; }, set links(v) { links = v; },
		get clipRange() { return clipRange; }, set clipRange(v) { clipRange = v; },
		get selChart() { return selChart; }, set selChart(v) { selChart = v; },
		get charts() { return charts; },
		get removeChart() { return removeChart; },
		get panel() { return panel; }, set panel(v) { panel = v; },
		get nudgeChart() { return nudgeChart; },
		get selectChart() { return selectChart; },
		get vs() { return vs; },
		get boxes() { return boxes; },
		get listOf() { return listOf; },
		get openList() { return openList; },
		get onrefback() { return onrefback; },
		get openFinder() { return openFinder; },
		get editLink() { return editLink; },
		get openLinesDialog() { return openLinesDialog; },
		get deleteWholeColumns() { return deleteWholeColumns; },
		get clearSelection() { return clearSelection; }
	});
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
	const mouseCtl = createMouse({
		get structure() { return structure; },
		get toLocal() { return toLocal; },
		get MARGIN() { return MARGIN; },
		get rowTop() { return rowTop; },
		get fRows() { return fRows; },
		get vs() { return vs; },
		get rax() { return rax; },
		get cax() { return cax; },
		get fCols() { return fCols; },
		get scroller() { return scroller; },
		get visibleRows() { return visibleRows; },
		get entries() { return entries; },
		get visibleData() { return visibleData; },
		get headerWidth() { return headerWidth; },
		get cellWidth() { return cellWidth; },
		get styleAt() { return styleAt; },
		get cells() { return cells; },
		get shown() { return shown; },
		get pivots() { return pivots; },
		get textWidth() { return textWidth; },
		get MAX_FIT() { return MAX_FIT; },
		get source() { return source; },
		get userWidths() { return userWidths; },
		get rebuildAxis() { return rebuildAxis; },
		get updateExtent() { return updateExtent; },
		get schedule() { return schedule; },
		get writing() { return writing; }, set writing(v) { writing = v; },
		get say() { return say; },
		get selection() { return selection; },
		get screenX() { return screenX; },
		get screenY() { return screenY; },
		get rh() { return rh; },
		get moving() { return moving; }, set moving(v) { moving = v; },
		get clipRange() { return clipRange; }, set clipRange(v) { clipRange = v; },
		get active() { return active; }, set active(v) { active = v; },
		get anchor() { return anchor; }, set anchor(v) { anchor = v; },
		get message() { return message; }, set message(v) { message = v; },
		get refreshFx() { return refreshFx; },
		get refreshStatus() { return refreshStatus; },
		get pivotIn() { return pivotIn; },
		get queryIn() { return queryIn; },
		get QUERY_READONLY() { return QUERY_READONLY; },
		get rangeValues() { return rangeValues; },
		get sheetId() { return sheetId; },
		get snapMerge() { return snapMerge; },
		get boxOf() { return boxOf; },
		get editing() { return editing; },
		get cellInfo() { return cellInfo; },
		get rowAtY() { return rowAtY; },
		get colAtX() { return colAtX; },
		get extentCols() { return extentCols; },
		get wholeSel() { return wholeSel; }, set wholeSel(v) { wholeSel = v; },
		get boxes() { return boxes; },
		get panDown() { return panDown; },
		get hold() { return hold; }, set hold(v) { hold = v; },
		get placing() { return placing; },
		get cellAt() { return cellAt; },
		get commitEditor() { return commitEditor; },
		get wrap() { return wrap; },
		get placePivotAt() { return placePivotAt; },
		get links() { return links; }, set links(v) { links = v; },
		get locked() { return locked; },
		get selChart() { return selChart; }, set selChart(v) { selChart = v; },
		get moveTo() { return moveTo; },
		get hiddenMarkAt() { return hiddenMarkAt; },
		get menuAction() { return menuAction; },
		get filling() { return filling; }, set filling(v) { filling = v; },
		get listOf() { return listOf; },
		get dropButton() { return dropButton; },
		get listMenu() { return listMenu; }, set listMenu(v) { listMenu = v; },
		get openList() { return openList; },
		get linkButton() { return linkButton; },
		get refAt() { return refAt; },
		get followRef() { return followRef; },
		get formulaMode() { return formulaMode; },
		get panel() { return panel; }, set panel(v) { panel = v; },
		get tableHintAt() { return tableHintAt; },
		get nameBadgeAt() { return nameBadgeAt; },
		get closeMenus() { return closeMenus; },
		get renameFromBadge() { return renameFromBadge; },
		get pivotBadgeAt() { return pivotBadgeAt; },
		get ensureVisible() { return ensureVisible; },
		get localBadgeAt() { return localBadgeAt; },
		get onaddshared() { return onaddshared; },
		get followBadgeAt() { return followBadgeAt; },
		get followMenu() { return followMenu; }, set followMenu(v) { followMenu = v; },
		get keyHintAt() { return keyHintAt; },
		get acceptKeyHint() { return acceptKeyHint; },
		get frozenHeaderAt() { return frozenHeaderAt; },
		get headerButton() { return headerButton; },
		get headerDrag() { return headerDrag; }, set headerDrag(v) { headerDrag = v; },
		get pivotHeld() { return pivotHeld; },
		get pivotZone() { return pivotZone; },
		get resetRowHeights() { return resetRowHeights; },
		get userHeights() { return userHeights; },
		get rowResizing() { return rowResizing; }, set rowResizing(v) { rowResizing = v; },
		get resizing() { return resizing; }, set resizing(v) { resizing = v; },
		get refDrag() { return refDrag; }, set refDrag(v) { refDrag = v; },
		get buttonsLayer() { return buttonsLayer; },
		get shownValue() { return shownValue; },
		get openTypeMenu() { return openTypeMenu; },
		get openPivotMenu() { return openPivotMenu; },
		get togglePivotFold() { return togglePivotFold; },
		get pressLink() { return pressLink; },
		get dragging() { return dragging; }, set dragging(v) { dragging = v; },
		get tableAt() { return tableAt; },
		get pivotAt() { return pivotAt; },
		get cellAddr() { return cellAddr; },
		get input() { return input; },
		get fxEditing() { return fxEditing; },
		get fxInput() { return fxInput; },
		get lastRef() { return lastRef; }, set lastRef(v) { lastRef = v; },
		get catalog() { return catalog; },
		get sheetName() { return sheetName; },
		get editValueText() { return editValueText; }, set editValueText(v) { editValueText = v; },
		get fxText() { return fxText; }, set fxText(v) { fxText = v; },
		get onFormulaInput() { return onFormulaInput; },
		get completion() { return completion; }, set completion(v) { completion = v; },
		get sig() { return sig; }, set sig(v) { sig = v; },
		get onsheet() { return onsheet; },
		get refMove() { return refMove; }, set refMove(v) { refMove = v; },
		get pan() { return pan; },
		get lockedRowAt() { return lockedRowAt; },
		get hoverLocked() { return hoverLocked; }, set hoverLocked(v) { hoverLocked = v; },
		get zoomK() { return zoomK; },
		get rebuildRows() { return rebuildRows; },
		get placeHover() { return placeHover; }, set placeHover(v) { placeHover = v; },
		get linkUnder() { return linkUnder; },
		get updateHeaderTip() { return updateHeaderTip; },
		get updateNoteTip() { return updateNoteTip; },
		get updateCommentTip() { return updateCommentTip; },
		get nameBox() { return nameBox; }, set nameBox(v) { nameBox = v; },
		get singleSelection() { return singleSelection; },
		get cellName() { return cellName; },
		get holdWhileScrolling() { return holdWhileScrolling; },
		get spacer() { return spacer; },
		get viewW() { return viewW; },
		get hiddenCols() { return hiddenCols; },
		get startEdit() { return startEdit; },
		get pivotActive() { return pivotActive; }
	});
	const { autoFitColumns, dropMove, fillHandle, doFill, fillDown, headerAt, lastRow, lastCol, wholeKind, selectRows, inSelection, onMouseDown, nextArg, refContext, dropRefMark, refTables, onMouseMove, onDragMove, stopAutoScroll, onDblClick } = mouseCtl;

	// --- Édition et zone de nom : sheet/edit.svelte.ts ---
	const editCtl = createEdit({
		get locked() { return locked; },
		get ownerAt() { return ownerAt; },
		get layoutLocked() { return layoutLocked; },
		get lockedRowAt() { return lockedRowAt; },
		get NOT_YOURS() { return NOT_YOURS; },
		get pivotAt() { return pivotAt; },
		get PIVOT_READONLY() { return PIVOT_READONLY; },
		get queryAt() { return queryAt; },
		get QUERY_READONLY() { return QUERY_READONLY; },
		get structure() { return structure; },
		get cellInfo() { return cellInfo; },
		get shownValue() { return shownValue; },
		get active() { return active; }, set active(v) { active = v; },
		get say() { return say; },
		get entries() { return entries; },
		get selection() { return selection; },
		get anchor() { return anchor; }, set anchor(v) { anchor = v; },
		get editByTyping() { return editByTyping; }, set editByTyping(v) { editByTyping = v; },
		get editValueText() { return editValueText; }, set editValueText(v) { editValueText = v; },
		get clipRange() { return clipRange; }, set clipRange(v) { clipRange = v; },
		get schedule() { return schedule; },
		get editRO() { return editRO; }, set editRO(v) { editRO = v; },
		get editing() { return editing; }, set editing(v) { editing = v; },
		get ensureVisible() { return ensureVisible; },
		get input() { return input; },
		get message() { return message; }, set message(v) { message = v; },
		get editErr() { return editErr; }, set editErr(v) { editErr = v; },
		get completion() { return completion; }, set completion(v) { completion = v; },
		get sig() { return sig; }, set sig(v) { sig = v; },
		get refreshFx() { return refreshFx; },
		get wrap() { return wrap; },
		get spills() { return spills; },
		get screenX() { return screenX; },
		get screenY() { return screenY; },
		get boxOf() { return boxOf; },
		get textWidth() { return textWidth; },
		get tableAt() { return tableAt; },
		get cells() { return cells; },
		get key() { return key; },
		get mergeOf() { return mergeOf; },
		get cax() { return cax; },
		get viewW() { return viewW; },
		get editorH() { return editorH; }, set editorH(v) { editorH = v; },
		get rspan() { return rspan; },
		get rh() { return rh; },
		get styleAt() { return styleAt; },
		get source() { return source; },
		get writing() { return writing; }, set writing(v) { writing = v; },
		get isNumeric() { return isNumeric; },
		get typed() { return typed; }, set typed(v) { typed = v; },
		get lastEditMs() { return lastEditMs; }, set lastEditMs(v) { lastEditMs = v; },
		get columnFlash() { return columnFlash; }, set columnFlash(v) { columnFlash = v; },
		get autoLinkAfter() { return autoLinkAfter; },
		get dvAlert() { return dvAlert; }, set dvAlert(v) { dvAlert = v; },
		get keyDlg() { return keyDlg; }, set keyDlg(v) { keyDlg = v; },
		get handDlg() { return handDlg; }, set handDlg(v) { handDlg = v; },
		get refreshStatus() { return refreshStatus; },
		get refusal() { return refusal; },
		get lastRepeat() { return lastRepeat; }, set lastRepeat(v) { lastRepeat = v; },
		get fxText() { return fxText; }, set fxText(v) { fxText = v; },
		get lastRef() { return lastRef; }, set lastRef(v) { lastRef = v; },
		get fxEditing() { return fxEditing; }, set fxEditing(v) { fxEditing = v; },
		get boxes() { return boxes; },
		get onFormulaInput() { return onFormulaInput; },
		get moveTo() { return moveTo; },
		get catalog() { return catalog; },
		get queryIn() { return queryIn; },
		get pivots() { return pivots; },
		get PIVOT_PARTIAL() { return PIVOT_PARTIAL; },
		get panel() { return panel; }, set panel(v) { panel = v; },
		get inTable() { return inTable; },
		get enterMove() { return enterMove; },
		get tabMove() { return tabMove; },
		get completionKey() { return completionKey; },
		get fxInput() { return fxInput; },
		get hold() { return hold; },
		get finishHold() { return finishHold; },
		get fxLock() { return fxLock; },
		get tabPress() { return tabPress; },
		get selectRange() { return selectRange; },
		get tableBox() { return tableBox; },
		get singleCol() { return singleCol; },
		get nameBox() { return nameBox; },
		get names() { return names; },
		get nameInput() { return nameInput; }
	});
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
	const findCtl = createFind({
		get finder() { return finder; }, set finder(v) { finder = v; },
		get schedule() { return schedule; },
		get active() { return active; },
		get editing() { return editing; },
		get boxOf() { return boxOf; },
		get data() { return data; },
		get searchZone() { return searchZone; },
		get source() { return source; },
		get findInput() { return findInput; },
		get wrap() { return wrap; },
		get replaceInput() { return replaceInput; },
		get matchKey() { return matchKey; }, set matchKey(v) { matchKey = v; },
		get matches() { return matches; }, set matches(v) { matches = v; },
		get sheetName() { return sheetName; },
		get onsheet() { return onsheet; },
		get ensureVisible() { return ensureVisible; },
		get moveTo() { return moveTo; }
	});
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
	const menusCtl = createMenus({
		get source() { return source; },
		get mixes() { return mixes; },
		get spills() { return spills; },
		get nameBadgeAt() { return nameBadgeAt; },
		get tipCell() { return tipCell; }, set tipCell(v) { tipCell = v; },
		get scroller() { return scroller; },
		get pivotBadgeAt() { return pivotBadgeAt; },
		get localBadgeAt() { return localBadgeAt; },
		get followBadgeAt() { return followBadgeAt; },
		get data() { return data; },
		get keyHintAt() { return keyHintAt; },
		get KEY_HINT_TIP() { return KEY_HINT_TIP; },
		get linkUnder() { return linkUnder; },
		get cellAt() { return cellAt; },
		get tableAt() { return tableAt; },
		get AUTO_NUMBER_TIP() { return AUTO_NUMBER_TIP; },
		get screenX() { return screenX; },
		get cax() { return cax; },
		get toLocal() { return toLocal; },
		get moveTo() { return moveTo; },
		get filterClosed() { return filterClosed; }, set filterClosed(v) { filterClosed = v; },
		get openFilter() { return openFilter; },
		get a1() { return a1; },
		get MARGIN() { return MARGIN; },
		get colAtX() { return colAtX; },
		get rowAtY() { return rowAtY; },
		get rowTop() { return rowTop; },
		get fRows() { return fRows; },
		get entries() { return entries; },
		get filterDlg() { return filterDlg; },
		get closeFilter() { return closeFilter; },
		get menu() { return menu; }, set menu(v) { menu = v; },
		get pivotDlg() { return pivotDlg; },
		get panel() { return panel; },
		get choiceCands() { return choiceCands; }, set choiceCands(v) { choiceCands = v; },
		get wrap() { return wrap; },
		get writing() { return writing; }, set writing(v) { writing = v; },
		get say() { return say; },
		get selChart() { return selChart; },
		get charts() { return charts; },
		get onChartMenu() { return onChartMenu; },
		get ensureVisible() { return ensureVisible; },
		get active() { return active; },
		get zoomK() { return zoomK; },
		get screenY() { return screenY; },
		get rh() { return rh; },
		get locked() { return locked; },
		get hold() { return hold; },
		get formulaMode() { return formulaMode; },
		get openHintMenu() { return openHintMenu; },
		get closeMenus() { return closeMenus; },
		get headerAt() { return headerAt; },
		get freeMenu() { return freeMenu; }, set freeMenu(v) { freeMenu = v; },
		get refItems() { return refItems; }, set refItems(v) { refItems = v; },
		get groupBandAt() { return groupBandAt; },
		get inSelection() { return inSelection; },
		get loadRefItems() { return loadRefItems; },
		get hiddenIn() { return hiddenIn; },
		get selection() { return selection; },
		get rowLabel() { return rowLabel; },
		get message() { return message; }, set message(v) { message = v; },
		get linesHint() { return linesHint; }, set linesHint(v) { linesHint = v; },
		get refreshStatus() { return refreshStatus; }
	});
	const { mixOf, mixText, spillUnder, updateHeaderTip, openTypeMenu, headerButton, frozenHeaderAt, chooseType, chooseKey, deleteTable, keyContextMenu, onContextMenu, linesOpts, menuAction } = menusCtl;

	// --- Mesures ------------------------------------------------------------

	// --- Mesure de la grille : sheet/bench.svelte.ts ---
	const benchCtl = createBench({
		get entries() { return entries; },
		get bench() { return bench; }, set bench(v) { bench = v; },
		get rowTop() { return rowTop; },
		get rspan() { return rspan; },
		get vs() { return vs; },
		get syncNative() { return syncNative; },
		get missingFrames() { return missingFrames; }, set missingFrames(v) { missingFrames = v; },
		get scroller() { return scroller; },
		get draw() { return draw; },
		get visibleRows() { return visibleRows; },
		get visibleData() { return visibleData; },
		get source() { return source; },
		get drawTimes() { return drawTimes; },
		get refreshStatus() { return refreshStatus; }
	});
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
	const findView: FindBarView = {
		get closeFinder() { return closeFinder; },
		get findChanged() { return findChanged; },
		get findInput() { return findInput; }, set findInput(v) { findInput = v; },
		get findNext() { return findNext; },
		get finder() { return finder; },
		get goFound() { return goFound; },
		get listAll() { return listAll; },
		get onFinderKey() { return onFinderKey; },
		get replaceAll() { return replaceAll; },
		get replaceInput() { return replaceInput; }, set replaceInput(v) { replaceInput = v; },
		get replaceOne() { return replaceOne; },
		get searchZone() { return searchZone; },
		get source() { return source; },
		get toggleReplace() { return toggleReplace; }
	};
</script>

{#snippet linesSub(kind: 'rows' | 'cols', x: number, y: number)}
	<!-- Sous-menu Ligne « … » ou Colonne « … » d'un numéro ou d'une lettre : insertion et suppression, taille, affichage (ctxmenu.ts). -->
	{@const rows = kind === 'rows'}
	{@const { r0, r1, c0, c1 } = selection()}
	{#each linesSubmenu(linesOpts(kind)) as g, gi (gi)}
		{#if gi}<hr />{/if}
		{#each g as id (id)}
			{#if id === 'insert'}
				<button onclick={() => sheetLines(rows ? 'insertRows' : 'insertCols')}>{rows ? tr('grid.lines.insertRows', { n: r1 - r0 + 1 }) : tr('grid.lines.insertCols', { n: c1 - c0 + 1 })}</button>
			{:else if id === 'delete'}
				<button onclick={() => sheetLines(rows ? 'deleteRows' : 'deleteCols')}
					>{rows ? (r1 > r0 ? tr('grid.lines.deleteRows') : tr('grid.lines.deleteRow')) : c1 > c0 ? tr('grid.lines.deleteCols') : tr('grid.lines.deleteCol')}</button
				>
			{:else if id === 'size'}
				<button onclick={() => openSize(rows, x, y)}>{rows ? tr('grid.menu.rowHeight') : tr('grid.menu.colWidth')}</button>
			{:else if id === 'autoFit'}
				{#if rows}
					<button onclick={autoFitRows} title={tr('grid.menu.autoFitRows.tooltip')}>{tr('grid.menu.autoFitRows')}</button>
				{:else}
					<button onclick={autoFitCols} title={tr('grid.menu.autoFitCols.tooltip')}>{tr('grid.menu.autoFitCols')}</button>
				{/if}
			{:else if id === 'hide'}
				<button onclick={() => setHidden(rows, true)}>{tr('grid.menu.hide')}</button>
			{:else if id === 'unhide'}
				<button onclick={() => unhide(rows)}>{tr('grid.menu.unhide')}</button>
			{/if}
		{/each}
	{/each}
{/snippet}

{#snippet basicItem(id: string, clear: () => void, rules: () => void)}
	<!-- Entrées communes des menus de cellules (ctxmenu.ts) : presse-papiers, effacement, mise en forme, note. -->
	{#if id === 'cut'}
		<button data-shed="3"
			onclick={() => {
				menu = freeMenu = null;
				copySelection(true);
			}}>{tr('common.cut')} <span class="keys">Ctrl+X</span></button
		>
	{:else if id === 'copy'}
		<button data-shed="3"
			onclick={() => {
				menu = freeMenu = null;
				copySelection();
			}}>{tr('common.copy')} <span class="keys">Ctrl+C</span></button
		>
	{:else if id === 'paste'}
		<button data-shed="3"
			onclick={() => {
				menu = freeMenu = null;
				pasteClipboard();
			}}>{tr('common.paste')} <span class="keys">Ctrl+V</span></button
		>
	{:else if id === 'pasteSpecial'}
		<button data-shed="3"
			onclick={() => {
				menu = freeMenu = null;
				void pasteSpecial();
			}}>{tr('grid.pasteSpecial.menu')} <span class="keys">Ctrl+Alt+V</span></button
		>
	{:else if id === 'pasteChart'}
		<button
			onclick={() => {
				menu = freeMenu = null;
				if (chartClip) pasteChart(chartClip);
			}}>{tr('grid.chart.paste')}</button
		>
	{:else if id === 'pasteButton'}
		<button
			onclick={() => {
				menu = freeMenu = null;
				buttonsLayer?.pasteAt(active.row, active.col);
			}}>{tr('grid.controls.paste')}</button
		>
	{:else if id === 'clear'}
		<button onclick={clear}>{tr('grid.menu.clearContents')}</button>
	{:else if id === 'format'}
		<button data-shed="1" onclick={() => openFormat()}>{tr('grid.menu.formatCells')} <span class="keys">Ctrl+1</span></button>
	{:else if id === 'rules'}
		<button data-shed="1" onclick={rules}>{tr('grid.menu.conditionalFormat')}</button>
	{:else if id === 'note'}
		{@render linkItems()}
		{@render noteItems()}
	{:else if id === 'comment'}
		<button data-shed="1" onclick={newComment}>{activeThread() ? tr('grid.comment.menuReply') : tr('grid.comment.menuNew')} <span class="keys">Ctrl+Alt+M</span></button>
	{/if}
{/snippet}

{#snippet linkItems()}
	{#if source.setLink}
		{@const url = linkOf(active.row, active.col)}
		{#if url}
			<button
				data-shed="1"
				onclick={() => {
					menu = freeMenu = null;
					void followLink(url);
				}}>{tr('grid.link.open')}</button
			>
			<button data-shed="1" onclick={editLink}>{tr('grid.link.menuEdit')} <span class="keys">Ctrl+K</span></button>
			<button data-shed="1" onclick={removeLink}>{tr('grid.link.remove')}</button>
		{:else}
			<button data-shed="1" onclick={editLink}>{tr('grid.link.menu')} <span class="keys">Ctrl+K</span></button>
		{/if}
	{/if}
{/snippet}

{#snippet noteItems()}
	{#if noteOf(active.row, active.col)}
		<button data-shed="1" onclick={editNote}>{tr('grid.note.edit')}</button>
		<button data-shed="1"
			onclick={() => {
				menu = freeMenu = null;
				deleteNote();
			}}>{tr('grid.note.delete')}</button
		>
	{:else}
		<button data-shed="1" onclick={editNote}>{tr('grid.note.insert')}</button>
	{/if}
{/snippet}

{#snippet refMenuItems()}
	<!-- Colonne de référence : basculer sur la ligne citée ; clé d'un tableau : les lignes qui la citent. -->
	{#if refItems?.go}
		{@const g = refItems.go}
		{@const at = { row: refItems.row, col: refItems.col }}
		<button
			disabled={g.found === false}
			title={g.found === false ? tr('grid.refLink.missing', { value: g.value, table: g.table }) : tr('grid.refLink.go.tooltip', { value: g.value, table: g.table })}
			onclick={() => {
				menu = freeMenu = null;
				refItems = null;
				void followRef(at.row, at.col);
			}}>{tr('grid.refLink.go', { table: clip(g.table), value: clip(g.value) })} <span class="keys">Ctrl+clic</span></button
		>
	{:else if refItems?.cite?.list?.length}
		{@const ci = refItems.cite}
		{@const list = ci.list ?? []}
		{@const at = { row: refItems.row, col: refItems.col }}
		{#if list.length === 1}
			<button
				disabled={!list[0].count}
				title={list[0].count ? tr('grid.refLink.cite.tooltip', { table: list[0].table, column: list[0].column, value: ci.value }) : undefined}
				onclick={() => openCitation(at.row, at.col, list[0], ci.value)}>{citeLabel(clip(ci.value), list)}</button
			>
		{:else}
			<Submenu label={citeLabel(clip(ci.value), list)}>
				{#each list as c (c.table + '[' + c.column + ']')}
					<button disabled={!c.count} title={tr('grid.refLink.cite.tooltip', { table: c.table, column: c.column, value: ci.value })} onclick={() => openCitation(at.row, at.col, c, ci.value)}
						>{clip(c.table)} ({c.count})</button
					>
				{/each}
			</Submenu>
		{/if}
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
		{#if menu}
			{@const m = menu}
			{@const col = m.e.t.columns[m.i]}
			{@const cols = !m.rowOnly}
			{@const rowItems = !m.header && !m.rowOnly}
			<!-- Structure du tableau (colonnes, en-têtes, conversion, suppression) : grisée ou retirée avec son motif, comme au ruban (feuille protégée, classeur partagé sans droit de conception, tableau en lecture seule). -->
			{@const why = designWhy(m.e.t)}
			{@const filtered = cols && !!source.clearFilters && m.e.t.columns.some((c) => c.filter)}
			{@const sortCount = (cols && source.sortTable ? 2 : 0) + (cols && source.sortTableCustom ? 1 : 0) + (cols && source.setFilter ? 1 : 0) + (filtered ? 1 : 0)}
			{@const toTitle = !why && cols && !!source.headerToTitle && m.e.t.rows > 0}
			{@const toHeader = !why && cols && titleAbove(m.e.t)}
			{@const promote = !why && cols && !!source.promoteHeader && m.e.t.rows > 0 && !m.e.t.query && !m.e.t.twoLevel}
			{@const toValues = !why && cols && source.columnsToValues ? valueColumns(m.e, m.i) : []}
			{@const refresh = !!m.e.t.query && !!source.refreshTable}
			{@const share = cols && !!onsharedata && !m.e.t.query}
			{@const addShared = cols && !!onaddshared && m.e.t.shared === 'private' && !m.e.t.query}
			{@const pushSource = cols && !!onpushsource && sourceTables.includes(m.e.t.name)}
			{@const twoLevel = !why && cols && !!source.setTwoLevel && !m.e.t.query}
			{@const follow = cols && !!source.setFollow && !!data?.choice && !m.e.t.query}
			{@const tableItems = toTitle || toHeader || promote || twoLevel || refresh || share || pushSource || follow || !!source.convertToRange || !!source.deleteTables}
			<!-- Situation (ctxmenu.ts) : sur l'en-tête, du texte ou des groupes au-dessus de ses colonnes ; sur une ligne, la 1re (tableau sans en-têtes) ou la dernière. -->
			{@const ref = !m.rowOnly && !m.header && (!!refItems?.go || !!refItems?.cite?.list?.length)}
			{@const promoteRow = !why && !m.header && !!source.promoteHeader && m.e.t.rows > 0 && !m.e.t.query && !m.e.t.twoLevel && m.n === 0 && allGenerated(m.e.t.columns.map((c) => c.name))}
			{@const toNote = !why && !m.header && !!source.rowToNote && m.n === m.e.t.rows - 1}
			{@const headAbove = m.header && !m.e.t.query && titleAboveFilled(m.e.t, (r, c) => cells.get(key(r, c)))}
			{@const columnGroups = tableColumnSubmenu({ sort: sortCount > 0, typeKey: !!source.setType && !col?.formula && !why, validation: !!source.applyStyle && !col?.formula, tabChoice: !!source.setTabChoice && !!col, toValues: toValues.length > 0 })}
			{@const menuGroups = m.rowOnly
				? headerMenu({
						promote: promoteRow,
						toNote: toNote,
						format: !!source.sheetLines && !!source.applyStyle,
						rules: !!source.sheetLines && !!source.setSheetRules,
						lines: linesSubmenu(linesOpts('rows')).length > 0,
						table: tableItems
					})
				: tableCellMenu({
						ref,
						promote: promoteRow,
						toNote: toNote,
						toHeader: toHeader && headAbove,
						groupsOn: twoLevel && headAbove && !m.e.t.twoLevel,
						groupsOff: twoLevel && m.header && !!m.e.t.twoLevel,
						format: !!source.applyStyle,
						rules: !!source.setSheetRules,
						note: !!source.setNote,
						comment: !!comments,
						sort: sortCount > 0,
						row: rowItems,
						column: cols,
						table: tableItems
					})}
			{#snippet sortItems()}
				{#if cols && source.sortTable}
					{@const lab = sortLabels(col?.type ?? 'text', col?.result)}
					<button onclick={() => menuAction(() => source.sortTable!(m.e.t, m.i, false))}>{lab[0]}</button>
					<button onclick={() => menuAction(() => source.sortTable!(m.e.t, m.i, true))}>{lab[1]}</button>
				{/if}
				{#if cols && source.sortTableCustom}
					<button onclick={() => openCustomSort(m.e, m.i)}>{tr('grid.pivot.menu.customSort')}</button>
				{/if}
				{#if cols && source.setFilter}
					<button onclick={() => openFilter(m.e, m.i)}>{tr('grid.menu.filterColumn')}</button>
				{/if}
				{#if filtered}
					<button onclick={() => menuAction(() => source.clearFilters!(m.e.t))}>{tr('grid.menu.clearTableFilter')}</button>
				{/if}
			{/snippet}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (menu = null)}>
				<!-- Les noms (ligne, colonne, tableau) n'apparaissent qu'en titre de leur sous-menu, abrégés ; jamais dans les libellés. -->
				{#if m.confirmDelete}
					<div class="menu-title" class:default-name={isDefaultTableName(m.e.t.name)} title={isDefaultTableName(m.e.t.name) ? defaultNameHint() : clipped(m.e.t.name) ? m.e.t.name : undefined}>{tr('grid.menu.table', { name: clip(m.e.t.name) })}</div>
					<p class="menu-note">
						{tr('grid.menu.deleteTable.confirm', { n: m.e.t.rows + (m.e.t.hidden ?? 0) })}
					</p>
					<button onclick={() => deleteTable(m.e.t)}>{tr('grid.menu.deleteTable')}</button>
					<button onclick={() => (menu = { ...m, confirmDelete: false })}>{tr('common.cancel')}</button>
				{:else}
					<!-- Ordre (demande de Louis, ctxmenu.ts) : un sous-menu par objet, titré de son nom
					     (ligne, colonne, tableau) ; les actions propres à la situation ; le presse-papiers ;
					     effacement et mise en forme ; la note ; tri et filtre. -->
					{#snippet rowItem(id: string)}
						{#if id === 'insertAbove'}
							<button onclick={() => menuAction(source.insertRow && (() => tableLines(m.e, 'insertRow', m.n)))}>{tr('grid.menu.insertAbove')}</button>
						{:else if id === 'insertBelow'}
							<button onclick={() => menuAction(source.insertRow && (() => tableLines(m.e, 'insertRow', m.n + 1)))}>{tr('grid.menu.insertBelow')}</button>
						{:else if id === 'deleteRow'}
							<button onclick={() => menuAction(source.deleteRow && (() => tableLines(m.e, 'deleteRow', m.n)))}>{tr('grid.lines.deleteRow')}</button>
						{/if}
					{/snippet}
					{#snippet columnItem(id: string)}
						{#if id === 'sort'}
							{@render sortItems()}
						{:else if id === 'rename'}
							<button
								onclick={() => {
									const t = m.e.t;
									const c = t.col + m.i;
									menu = null;
									moveTo(t.row, c);
									startEdit(false);
								}}>{tr('grid.menu.renameColumn')}</button
							>
						{:else if id === 'typeKey'}
							<!-- Le menu ▾ de la colonne (tri, filtre, type, clé), sous son en-tête. -->
							<button onclick={() => openFilter(m.e, m.i)}>{tr('grid.menu.typeAndKey')}</button>
						{:else if id === 'validation'}
							<!-- Validation des données de toute la colonne (ses lignes de données). -->
							<button
								onclick={() => {
									const e = m.e;
									const c = e.t.col + m.i;
									menu = null;
									selectRange(e.t.row + 1, c, Math.max(e.last, e.t.row + 1), c);
									openValidation();
								}}>{tr('grid.menu.validation')}</button
							>
						{:else if id === 'tabChoice' && col}
							{@const tc = tabChoiceFor(m.e.t.name, col.name, { formula: !!col.formula })}
							<button title={tc.title} disabled={tc.disabled} onclick={() => runTabChoice(m.e.t.name, col.name, tc)}>{tc.label}</button>
						{:else if id === 'toValues'}
							<!-- Colonne calculée (ou colonnes calculées de la sélection) : ses valeurs, sans la formule. -->
							<button
								title={toValues.length > 1 ? tr('grid.menu.toValues.many.tooltip', { n: toValues.length }) : tr('grid.menu.toValues.tooltip')}
								onclick={() => menuAction(() => source.columnsToValues!(m.e.t, toValues))}>{tr('grid.menu.toValues')}</button
							>
						{:else if id === 'insertLeft'}
							<button disabled={!!why} title={why || undefined} onclick={() => menuAction(source.insertColumn && (() => tableLines(m.e, 'insertColumn', m.i)))}>{tr('grid.menu.insertLeft')}</button>
						{:else if id === 'insertRight'}
							<button disabled={!!why} title={why || undefined} onclick={() => menuAction(source.insertColumn && (() => tableLines(m.e, 'insertColumn', m.i + 1)))}>{tr('grid.menu.insertRight')}</button>
						{:else if id === 'deleteColumn'}
							<button disabled={!!why} title={why || undefined} onclick={() => menuAction(source.deleteColumn && (() => tableLines(m.e, 'deleteColumn', m.i)))}>{tr('grid.lines.deleteCol')}</button>
						{/if}
					{/snippet}
					{#snippet item(id: string)}
						{#if id === 'ref'}
							{@render refMenuItems()}
						{:else if id === 'promote'}
							<!-- Tableau sans en-têtes (que des « Colonne n ») : sur sa 1re ligne. -->
							<button title={tr('grid.menu.promote.tooltip')} onclick={() => menuAction(() => source.promoteHeader!(m.e.t))}>{tr('grid.menu.promote')}</button>
						{:else if id === 'toNote'}
							<!-- Seulement sur la dernière ligne de données (le moteur refuse les autres). -->
							<button title={tr('grid.menu.rowToNote.tooltip')} onclick={() => menuAction(() => source.rowToNote!(m.e.t, m.n))}>{tr('grid.menu.rowToNote')}</button>
						{:else if id === 'toHeader'}
							<!-- En-tête avec du texte tapé au-dessus de ses colonnes. -->
							<button title={tr('grid.menu.titleToHeader.tooltip')} onclick={() => menuAction(() => source.titleToHeader!(m.e.t))}>{tr('grid.menu.titleToHeader')}</button>
						{:else if id === 'groupsOn'}
							<button title={tr('grid.menu.twoLevel.on.tooltip')} onclick={() => menuAction(() => source.setTwoLevel!(m.e.t, true))}>{tr('grid.menu.twoLevel.on')}</button>
						{:else if id === 'groupsOff'}
							<!-- En-tête (ou bande) de colonnes groupées. -->
							<button title={tr('grid.menu.twoLevel.off.tooltip')} onclick={() => menuAction(() => source.setTwoLevel!(m.e.t, false))}>{tr('grid.menu.twoLevel.off')}</button>
						{:else if id === 'sort'}
							<Submenu label={tr('grid.menu.sortFilter')}>{@render sortItems()}</Submenu>
						{:else if id === 'row'}
							<!-- Titré de l'identifiant de la ligne que montre la gouttière : sa clé, sinon son numéro (rowLabel). -->
							{@const rid = rowLabel(m.e, m.e.t.row + 1 + m.n)}
							<Submenu label={tr('grid.menu.row', { name: clip(rid) })} title={clipped(rid) ? rid : undefined}>
								{#each tableRowSubmenu() as g, gi (gi)}{#if gi}<hr />{/if}{#each g as id (id)}{@render rowItem(id)}{/each}{/each}
							</Submenu>
						{:else if id === 'column'}
							<!-- Ce qui ne touche que la colonne ; sa suppression en dernier. -->
							<Submenu label={tr('grid.menu.column', { name: clip(col?.name) })} title={clipped(col?.name) ? col?.name : undefined}>
								{#each columnGroups as g, gi (gi)}{#if gi}<hr />{/if}{#each g as id (id)}{@render columnItem(id)}{/each}{/each}
							</Submenu>
						{:else if id === 'linesTop'}
							<!-- Numéro de ligne : la ligne de la feuille (insertion, suppression, hauteur, affichage), en tête du menu. -->
							{@render linesSub('rows', m.x, m.y)}
						{:else if id === 'table'}
							<!-- Le tableau, titré de son nom : en-têtes, requête, partage, conversion ; sa suppression en dernier. -->
							<Submenu
								label={tr('grid.menu.table', { name: clip(m.e.t.name) })}
								title={isDefaultTableName(m.e.t.name) ? defaultNameHint() : clipped(m.e.t.name) ? m.e.t.name : undefined}
							>
								{#if toTitle}
									<button title={tr('grid.menu.headerToTitle.tooltip')} onclick={() => menuAction(() => source.headerToTitle!(m.e.t))}>{tr('grid.menu.headerToTitle')}</button>
								{/if}
								{#if promote}
									<button title={tr('grid.menu.promote.tooltip')} onclick={() => menuAction(() => source.promoteHeader!(m.e.t))}>{tr('grid.menu.promote')}</button>
								{/if}
								{#if toHeader}
									<button title={tr('grid.menu.titleToHeader.tooltip')} onclick={() => menuAction(() => source.titleToHeader!(m.e.t))}>{tr('grid.menu.titleToHeader')}</button>
								{/if}
								{#if twoLevel}
									{@const on = !!m.e.t.twoLevel}
										<button
											title={on
												? tr('grid.menu.twoLevel.off.tooltip')
												: tr('grid.menu.twoLevel.on.tooltip')}
											onclick={() => menuAction(() => source.setTwoLevel!(m.e.t, !on))}>{on ? tr('grid.menu.twoLevel.off') : tr('grid.menu.twoLevel.on')}</button
									>
								{/if}
								{#if follow}
									{@const on = !!m.e.t.follows}
									{@const what = followLabel(data?.choice?.name ?? '')}
									<button
										role="menuitemcheckbox"
										aria-checked={on}
										title={data?.choice?.formula
											? on
												? tr('grid.menu.follow.off.tooltip')
												: tr('grid.menu.follow.on.formula.tooltip', { formula: data.choice.formula })
											: on
												? tr('grid.menu.follow.off.column.tooltip', { name: data?.choice?.name ?? '' })
												: tr('grid.menu.follow.on.tooltip', { name: data?.choice?.name ?? '' })}
										onclick={() => toggleFollow(m.e.t, !on)}><span class="check">{on ? '✓' : ''}</span>{tr('grid.menu.follow', { what })}</button
									>
								{/if}
								{#if refresh}
									{@const t = m.e.t}
									<button title={tr('grid.menu.refresh.tooltip', { name: t.query?.name ?? '' })} onclick={() => menuAction(() => source.refreshTable!(t))}>{tr('grid.menu.refresh')}</button>
										{#if source.setHandKey && t.columns.some((k) => k.hand && !k.formula)}
											<button title={tr('grid.menu.handKey.tooltip')} onclick={() => openHandKey(t)}>{tr('grid.menu.handKey')}</button>
										{/if}
										{#if source.forgetHandKept && t.handKept}
											<button title={tr('grid.menu.handForget.tooltip')} onclick={() => menuAction(() => source.forgetHandKept!(t))}>{tr('grid.menu.handForget', { n: t.handKept })}</button>
										{/if}
								{/if}
								{#if addShared}
									<button title={tr('grid.menu.addShared.tooltip')} onclick={() => { const n = m.e.t.name; menu = null; onaddshared?.(n); }}>{tr('grid.menu.addShared')}</button>
								{/if}
								{#if share}
									<button title={tr('grid.menu.shareData.tooltip')} onclick={() => { const n = m.e.t.name; menu = null; onsharedata?.(n); }}>{tr('grid.menu.shareData')}</button>
								{/if}
								{#if pushSource}
									<button title={tr('grid.menu.pushSource.tooltip')} onclick={() => { const n = m.e.t.name; menu = null; onpushsource?.(n); }}>{tr('grid.menu.pushSource')}</button>
								{/if}
								{#if source.convertToRange}
									<button disabled={!!why} title={why || tr('grid.menu.toRange.tooltip')} onclick={() => menuAction(() => source.convertToRange!(m.e.t))}>{tr('grid.menu.toRange')}</button>
								{/if}
								{#if source.deleteTables}
									<hr />
									<button
										disabled={!!why}
										title={why || undefined}
										onclick={() => {
											if (m.e.t.rows + (m.e.t.hidden ?? 0) > CONFIRM_DELETE_ROWS) menu = { ...m, confirmDelete: true };
											else deleteTable(m.e.t);
										}}>{tr('grid.menu.deleteTable')}</button
									>
								{/if}
							</Submenu>
						{:else}
							{@render basicItem(
								id,
								() => {
									menu = null;
									if (m.rowOnly && deleteWholeColumns()) return;
									clearSelection();
								},
								() => (m.rowOnly ? rulesActive() : openRules(m.e, m.i))
							)}
						{/if}
					{/snippet}
					{#each menuGroups as g, gi (gi)}
						{#if gi}<hr />{/if}
						{#each g as id (id)}{@render item(id)}{/each}
					{/each}
				{/if}
			</div>
		{/if}
		{#if followMenu}
			{@const m = followMenu}
			{@const what = followLabel(data?.choice?.name ?? '')}
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (followMenu = null)}>
				<div class="menu-title" title={clipped(m.e.t.name) ? m.e.t.name : undefined}>{tr('grid.menu.table', { name: clip(m.e.t.name) })}</div>
				<button role="menuitemcheckbox" aria-checked="true" disabled={!source.setFollow} onclick={() => toggleFollow(m.e.t, false)}><span class="check">✓</span>{tr('grid.menu.follow', { what })}</button>
			</div>
		{/if}
		{#if hintMenu}
			{@const m = hintMenu}
			{@const name = m.e.t.columns[0]?.name}
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (hintMenu = null)}>
				<div class="menu-title" title={clipped(name) ? name : undefined}>{tr('grid.menu.column', { name: clip(name) })}</div>
				<button
					onclick={() => {
						hintMenu = null;
						acceptKeyHint(m.e);
					}}>{tr('grid.key.accept')}</button
				>
				{#if source.dismissKeyHint}
					{@const dismissHint = source.dismissKeyHint}
					<button
						onclick={() => {
							hintMenu = null;
							menuAction(() => dismissHint(m.e.t));
						}}>{tr('grid.key.dismiss')}</button
					>
				{/if}
			</div>
		{/if}
		{#if freeMenu}
			{@const m = freeMenu}
			{@const below = groupCandidate()}
			<!-- Cellules libres verrouillées (protection, classeur partagé sans droit de conception) : pas d'actions qui les changent. -->
			{@const free = !locked && !structure}
			{@const cell = !m.kind}
			{@const join = cell && free && !!source.joinTable && keptOut(active.row, active.col)}
			{@const asTable = cell && free && !!source.formatAsTable && inRangeZone(active.row, active.col)}
			{@const under = cell && free ? titleTableAt(active.row, active.col) : undefined}
			{@const spilled = cell && free && source.convertSpill ? spillUnder(active.row, active.col) : null}
			{@const toValues = cell && free && !!source.rangeToValues && !spilled && formulasInSelection()}
			{@const fp = cell && source.setPivot ? pivotAt(active.row, active.col) : null}
			{@const fold = fp?.rows ? fp : null}
			<!-- Tout tableau croisé, même en erreur (source introuvable) : il se supprime ; en erreur, il se rattache à un autre tableau. -->
			{@const dp = cell && source.deletePivot ? (pivotAt(active.row, active.col) ?? brokenPivotAt(pivots, active.row, active.col)) : null}
			{@const pv = dp ?? fold}
			{@const pasteChart = !!chartClip}
			{@const pasteButton = !!buttonsLayer?.hasClip()}
			{@const lines = !!source.sheetLines}
			{@const menuGroups = m.kind
				? headerMenu({
						asGroups: !!below,
						pasteChart,
						pasteButton,
						format: lines && !!source.applyStyle,
						rules: lines && !!source.setSheetRules,
						lines: linesSubmenu(linesOpts(m.kind)).length > 0
					})
				: freeCellMenu({
						ref: !!refItems?.go || !!refItems?.cite?.list?.length,
						asHeader: !!under,
						join,
						asTable,
						spill: !!spilled,
						toValues,
						asGroups: !!below,
						pasteChart,
						pasteButton,
						format: !!source.applyStyle,
						rules: !!source.setSheetRules,
						note: !!source.setNote,
						comment: !!comments,
						lines,
						pivot: !!pv
					})}
			{#snippet pivotItem(id: string)}
				{#if fold && id === 'expand'}
					<!-- Comme le sous-menu « Développer/Réduire » d'Excel. -->
					<button disabled={!foldTarget(pivotRowOf(fold, active.row), true)} onclick={() => foldPivot(fold, 'expand')}>{tr('grid.pivot.fold.expand')}</button>
				{:else if fold && id === 'collapse'}
					<button disabled={!foldTarget(pivotRowOf(fold, active.row), false)} onclick={() => foldPivot(fold, 'collapse')}>{tr('grid.pivot.fold.collapse')}</button>
				{:else if fold?.rows && id === 'expandAll'}
					<button disabled={!canFoldAll(fold.rows, true)} onclick={() => foldPivot(fold, 'expandAll')}>{tr('grid.pivot.fold.expandAll')}</button>
				{:else if fold?.rows && id === 'collapseAll'}
					<button disabled={!canFoldAll(fold.rows, false)} onclick={() => foldPivot(fold, 'collapseAll')}>{tr('grid.pivot.fold.collapseAll')}</button>
				{:else if dp && id === 'source'}
					<button title={tr('grid.pivot.menu.source.tooltip')} onclick={() => pickPivotSource(dp)}>{tr('grid.pivot.menu.source')}</button>
				{:else if dp && id === 'move'}
					<button title={tr('grid.pivot.menu.move.tooltip')} onclick={() => startPivotMove(dp)}>{tr('grid.pivot.menu.move')}</button>
				{:else if dp && id === 'delete'}
					<button onclick={() => removePivot(dp)}>{tr('grid.pivot.delete')}</button>
				{/if}
			{/snippet}
			{#snippet item(id: string)}
				{@const at = { row: active.row, col: active.col }}
				{#if id === 'ref'}
					{@render refMenuItems()}
				{:else if id === 'asHeader' && under}
					{@const t = under}
					<button title={tr('grid.menu.asHeader.tooltip')} onclick={() => menuAction(() => source.titleToHeader!(t))}>{tr('grid.menu.asHeader', { table: t.name })}</button>
				{:else if id === 'join'}
					<button title={tr('grid.menu.join.tooltip')} onclick={() => menuAction(() => source.joinTable!(at.row, at.col))}>{tr('grid.menu.join')}</button>
				{:else if id === 'asTable'}
					<button title={tr('grid.menu.asTable.tooltip')} onclick={() => menuAction(() => source.formatAsTable!(at.row, at.col))}>{tr('grid.menu.asTable')}</button>
				{:else if id === 'spillLinked' && spilled}
					{@const sp = { row: spilled.row, col: spilled.col }}
					<button title={tr('grid.menu.spillLinked.tooltip')} onclick={() => menuAction(() => source.convertSpill!(sp.row, sp.col, true))}>{tr('grid.menu.spillLinked')}</button>
				{:else if id === 'spillValues' && spilled}
					{@const sp = { row: spilled.row, col: spilled.col }}
					<button title={tr('grid.menu.spillValues.tooltip')} onclick={() => menuAction(() => source.convertSpill!(sp.row, sp.col, false))}>{tr('grid.menu.spillValues')}</button>
				{:else if id === 'toValues'}
					<button title={tr('grid.menu.rangeToValues.tooltip')} onclick={() => toValuesActive()}>{tr('grid.menu.toValues')}</button>
				{:else if id === 'asGroups' && below}
					<!-- Ligne juste au-dessus de l'en-tête d'un tableau : ses cases peuvent devenir les groupes d'en-têtes. -->
					<button title={tr('grid.menu.asGroups.tooltip', { table: below.t.name })} onclick={() => menuAction(() => source.setTwoLevel!(below.t, true))}>{tr('grid.menu.asGroups')}</button>
				{:else if id === 'insertDialog'}
					<button onclick={() => openLinesDialog(true, m.x, m.y)}>{tr('grid.lines.insert')}</button>
				{:else if id === 'clearAll'}
					<button
						title={tr('grid.menu.clearAll.tooltip')}
						onclick={() => {
							menu = freeMenu = null;
							clearSelection(selection(), true);
						}}>{tr('common.delete')}</button
					>
				{:else if id === 'linesTop' && m.kind}
					{@render linesSub(m.kind, m.x, m.y)}
				{:else if id === 'pivot' && pv}
					<!-- Le tableau croisé, titré de son nom (ou de sa source) ; sa suppression en dernier. -->
					{@const pname = pv.config.name ? tr('grid.pivot.menu.title', { name: clip(pv.config.name) }) : tr('grid.pivot.menu.titleOf', { source: clip(pv.config.source) })}
					<Submenu label={pname} title={clipped(pv.config.name || pv.config.source) ? pv.config.name || pv.config.source : undefined}>
						{#each pivotSubmenu({ fold: !!fold, source: !!dp?.error && !!source.setPivot && pivotSources(dp).length > 0, move: !!dp && !!source.movePivot, remove: !!dp }) as g, gi (gi)}
							{#if gi}<hr />{/if}
							{#each g as pid (pid)}{@render pivotItem(pid)}{/each}
						{/each}
					</Submenu>
				{:else}
					{@render basicItem(
						id,
						() => {
							menu = freeMenu = null;
							if (!deleteWholeColumns()) clearSelection();
						},
						rulesActive
					)}
				{/if}
			{/snippet}
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (freeMenu = null)}>
				<!-- Ordre (demande de Louis, ctxmenu.ts), comme sur un tableau : le sous-menu de
				     l'objet (tableau croisé) ; la situation ; le presse-papiers ; effacement et mise
				     en forme ; la note ; insertion et suppression. -->
				{#each menuGroups as g, gi (gi)}
					{#if gi}<hr />{/if}
					{#each g as id (id)}{@render item(id)}{/each}
				{/each}
			</div>
		{/if}
		{#if linesDlg}
			{@const d = linesDlg}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="rules lines-dlg" use:dismissPanel={() => (linesDlg = null)} onkeydown={(e) => e.key === 'Enter' && applyLinesDialog()}>
				<div class="menu-title">{d.insert ? tr('grid.menu.insert') : tr('common.delete')}</div>
				<label><input type="radio" bind:group={d.choice} value="rows" /> {d.insert ? tr('grid.lines.dialog.rowAbove') : tr('grid.lines.dialog.row')}</label>
				<label><input type="radio" bind:group={d.choice} value="cols" /> {d.insert ? tr('grid.lines.dialog.colLeft') : tr('grid.lines.dialog.col')}</label>
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" onclick={applyLinesDialog}>{tr('common.ok')}</button>
					<button onclick={() => (linesDlg = null)}>{tr('common.cancel')}</button>
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
				aria-label={tr('grid.keyDialog.title')}
				use:focusFirst
				use:dismissPanel={cancelKeyDialog}
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Enter') applyKeyDialog();
					if (ev.key === 'Escape') cancelKeyDialog();
				}}
			>
				<div class="menu-title">{tr('grid.keyDialog.heading', { table: d.table })}</div>
				<label><input type="radio" bind:group={d.choice} value="index" /> {tr('grid.keyDialog.index')}</label>
				<label title={d.why ? tr('grid.keyDialog.notKey', { column: d.column, why: d.why }) : undefined}>
					<input type="radio" bind:group={d.choice} value="column" disabled={!!d.why} /> {tr('grid.keyDialog.column', { column: d.column })}
				</label>
				{#if d.why}
					<p class="menu-note">{tr('grid.keyDialog.notKey.note', { column: d.column, why: d.why })}</p>
				{:else if d.choice === 'index'}
					<p class="menu-note">{tr('grid.keyDialog.index.note')}</p>
				{/if}
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" onclick={applyKeyDialog}>{tr('grid.formula.validate')}</button>
					<button onclick={cancelKeyDialog}>{tr('common.cancel')}</button>
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
				aria-label={tr('grid.handDialog.title')}
				use:focusFirst
				use:dismissPanel={cancelHandDialog}
				onkeydown={(ev) => {
					ev.stopPropagation();
					if (ev.key === 'Enter') applyHandDialog();
					if (ev.key === 'Escape') cancelHandDialog();
				}}
			>
				<div class="menu-title">{tr('grid.handDialog.heading', { table: d.table })}</div>
				{#each d.columns as c (c)}
					<label><input type="radio" bind:group={d.choice} value={c} /> {c.includes(' + ') ? tr('grid.handDialog.columns', { name: c }) : tr('grid.handDialog.column', { name: c })}</label>
				{/each}
				<label><input type="radio" bind:group={d.choice} value="#" /> {tr('grid.handDialog.rank')}</label>
				{#if d.choice === '#'}
					<p class="menu-note">{tr('grid.handDialog.rank.note')}</p>
				{:else if d.t}
					<p class="menu-note">{tr('grid.handDialog.change.note')}</p>
				{:else}
					<p class="menu-note">{tr('grid.handDialog.key.note', { key: d.choice })}</p>
				{/if}
				{#if !d.columns.length && !d.t}
					<p class="menu-note">{tr('grid.handDialog.none.note')}</p>
				{/if}
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" onclick={applyHandDialog}>{tr('grid.formula.validate')}</button>
					<button onclick={cancelHandDialog}>{tr('common.cancel')}</button>
				</div>
			</div>
		{/if}
		{#if chartMenu}
			{@const m = chartMenu}
			{@const name = m.ch.config.title?.trim() ? tr('grid.chart.aria.titled', { title: clip(m.ch.config.title) }) : tr('grid.chart.aria')}
			{#snippet chartItem(id: string)}
				{#if id === 'cut'}
					<button onclick={() => copyChart(m.ch, true)}>{tr('common.cut')} <span class="keys">Ctrl+X</span></button>
				{:else if id === 'copy'}
					<button onclick={() => copyChart(m.ch, false)}>{tr('common.copy')} <span class="keys">Ctrl+C</span></button>
				{:else if id === 'copyImage'}
					<button onclick={() => copyChartImage(m.ch)}>{tr('grid.chart.copyImage')}</button>
				{:else if id === 'pasteHere'}
					<button
						disabled={!chartClip}
						onclick={() => {
							chartMenu = null;
							if (chartClip) pasteChart(chartClip);
						}}>{tr('grid.chart.pasteHere')}</button
					>
				{:else if id === 'edit'}
					<button
						onclick={() => {
							chartMenu = null;
							editChart(m.ch);
						}}>{tr('grid.chart.edit')}</button
					>
				{:else if id === 'delete'}
					<button onclick={() => removeChart(m.ch.id)}>{tr('grid.chart.delete')}</button>
				{:else if id === 'chart'}
					<!-- Le graphique, titré de son nom (son titre) quand il en a un ; sa suppression en dernier. -->
					<Submenu label={name} title={clipped(m.ch.config.title) ? m.ch.config.title : undefined}>
						{#each chartSubmenu({ edit: !(locked || structure) || !!hold }) as g, gi (gi)}
							{#if gi}<hr />{/if}
							{#each g as cid (cid)}{@render chartItem(cid)}{/each}
						{/each}
					</Submenu>
				{/if}
			{/snippet}
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (chartMenu = null)}>
				<!-- Le sous-menu du graphique, puis le presse-papiers (ctxmenu.ts). -->
				{#each chartMenuGroups({ image: m.ch.config.kind !== 'map' && !m.ch.error }) as g, gi (gi)}
					{#if gi}<hr />{/if}
					{#each g as id (id)}{@render chartItem(id)}{/each}
				{/each}
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
				<div class="menu-title">{tr('grid.dedup.title', { table: d.e.t.name })}</div>
				<p class="menu-note">{tr('grid.dedup.help')}</p>
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
						{tr('grid.dedup.allColumns')}
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
					{d.count === null ? tr('grid.dedup.computing') : d.count === 0 ? tr('grid.dedup.noneWith') : tr('grid.dedup.willRemove', { n: d.count })}
				</p>
				{#if d.error}<p class="menu-note error">{d.error}</p>{/if}
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" disabled={!d.count} onclick={applyDedup}>{tr('common.delete')}</button>
					<button
						onclick={() => {
							dedupDlg = null;
							wrap.focus();
						}}>{tr('common.cancel')}</button
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
				<div class="menu-title">{tr('grid.functions.title', { n: catalog?.functions.length ?? 0 })}</div>
				<!-- svelte-ignore a11y_autofocus -->
				<div class="add"><input class="search" bind:value={d.search} placeholder={tr('grid.functions.search')} autofocus /></div>
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
									<dt>{arg.name}{arg.optional ? ' ' + tr('grid.sig.optional') : ''}</dt>
									<dd>{arg.desc}</dd>
								{/each}
							</dl>
							{#if cur.repeat}<div class="count">{tr('grid.functions.repeat', { n: cur.repeat })}</div>{/if}
						{/if}
						{#if cur.example}<div>{tr('grid.sig.example')} <code>{cur.example}</code></div>{/if}
						<div class="count">{cur.category}{cur.other && cur.other !== cur.name ? ' · ' + tr('grid.functions.english', { name: cur.other }) : ''}</div>
					</div>
				{/if}
				<div class="foot">
					<span class="sp"></span>
					<button
						onclick={() => {
							fnDlg = null;
							wrap.focus();
						}}>{tr('common.close')}</button
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
						>{tr('grid.pivot.filter.field')}
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
					<div class="menu-title" title={clipped(d.field) ? d.field : undefined}>{d.kind === 'filter' ? tr('grid.pivot.filter.reportField', { field: clip(d.field) }) : tr('grid.quoted', { text: clip(d.field) })}</div>
				{/if}
				{#if d.kind === 'rows'}
					<button class="sort" class:on={d.p.config.sort?.[d.field] === 'asc'} onclick={() => sortPivot('asc')}>{tr('grid.sort.text.asc')}</button>
					<button class="sort" class:on={d.p.config.sort?.[d.field] === 'desc'} onclick={() => sortPivot('desc')}>{tr('grid.sort.text.desc')}</button>
					{#if source.pivotFieldValues}
						<button class="sort" class:on={!!d.p.config.orders?.[d.field]} onclick={openPivotCustomSort}>{tr('grid.pivot.menu.customSort')}</button>
					{/if}
					<hr />
				{/if}
				{#if d.kind === 'filter' ? d.p.config.filters?.some((f) => f.field === d.field && f.values.length) : pivotFiltered(d.p, d.field)}
					<button class="sort" onclick={() => applyPivotFilter(true)}>{tr('grid.filter.clearOf', { field: d.field })}</button>
				{/if}
				{#if source.setTabChoice}
					{@const tc = tabChoiceFor(d.p.config.source, d.field, { field: true })}
					<button class="sort" title={tc.title} disabled={tc.disabled} onclick={() => runTabChoice(d.p.config.source, d.field, tc)}>{tc.label}</button>
					<hr />
				{/if}
				<div class="add">
					<input class="search" bind:value={d.search} placeholder={tr('grid.filter.search.placeholder')} />
				</div>
				<div class="values">
					{#if d.loading}
						<p class="loading"><span class="spinner" aria-hidden="true"></span> {tr('grid.filter.loading')}</p>
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
							{d.search.trim() ? tr('grid.filter.selectAllResults') : d.kind === 'filter' ? tr('grid.pivot.filter.all') : tr('grid.filter.selectAll')}
						</label>
						{#each shown as v (v.text)}
							<label>
								<input type="checkbox" bind:checked={v.on} />
								<span class="what">{v.text === '' ? tr('grid.filter.blanks') : v.text}</span>
								<span class="count">{v.count}</span>
							</label>
						{/each}
					{/if}
				</div>
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" disabled={d.loading} onclick={() => applyPivotFilter()}>{tr('common.ok')}</button>
					<button onclick={() => (pivotDlg = null)}>{tr('common.cancel')}</button>
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
				<div class="menu-title" title={clipped(fcol?.name) ? fcol?.name : undefined}>{tr('grid.quoted', { text: clip(fcol?.name) })}</div>
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
						<button class="sort" onclick={() => openCustomSort(d.e, d.i)}>{tr('grid.pivot.menu.customSort')}</button>
					{/if}
					<hr />
				{/if}
				{#if fcol?.filter}
					<button class="sort" onclick={() => applyFilter(true)}>{tr('grid.filter.clearOf', { field: fcol.name })}</button>
				{/if}
				{#if source.setTabChoice && fcol}
					{@const tc = tabChoiceFor(d.e.t.name, fcol.name, { formula: !!fcol.formula })}
					<button class="sort" title={tc.title} disabled={tc.disabled} onclick={() => runTabChoice(d.e.t.name, fcol.name, tc)}>{tc.label}</button>
					<hr />
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
							<button class="sort" onclick={() => startCustom('eq')}>{tr('grid.filter.custom')}</button>
						</div>
					{/if}
					{#if fcol?.filter?.conds?.length && !d.custom}
						<p class="menu-note">
							{tr('grid.filter.active', { conds: describeConds(fcol.filter, kind) })}
							<button class="link" onclick={() => startCustom()}>{tr('grid.controls.menu.edit')}</button>
						</p>
					{/if}
					<hr />
					{#if d.custom}
						{@const cu = d.custom}
						<div class="custom">
							<p class="menu-note">{tr('grid.filter.custom.heading', { column: fcol?.name ?? '' })}</p>
							{#each cu.conds as c, k (k)}
								{#if k === 1}
									<div class="andor">
										<label><input type="radio" bind:group={cu.or} value={false} /> {tr('grid.filter.custom.and')}</label>
										<label><input type="radio" bind:group={cu.or} value={true} /> {tr('grid.filter.custom.or')}</label>
									</div>
								{/if}
								<div class="add">
									<select bind:value={c.op} aria-label={tr('grid.filter.custom.cond', { n: k + 1 })}>
										{#if k > 0}<option value="">{tr('grid.filter.custom.none')}</option>{/if}
										{#each condOps(kind) as op (op)}<option value={op}>{condLabel(op, kind)}</option>{/each}
									</select>
								</div>
								{#if c.op && condArgs(c.op) > 0}
									<div class="add">
										<input bind:value={c.value} placeholder={c.op === 'top' || c.op === 'bottom' ? tr('grid.filter.custom.number') : kind === 'date' ? tr('grid.filter.custom.date') : tr('grid.cf.placeholder.value')} />
										{#if condArgs(c.op) === 2}<span>{tr('grid.cf.and')}</span><input bind:value={c.to} placeholder={kind === 'date' ? tr('grid.filter.custom.date') : tr('grid.cf.placeholder.value')} />{/if}
									</div>
								{/if}
							{/each}
							{#if kind === 'text'}<p class="menu-note">{tr('grid.filter.custom.wildcards')}</p>{/if}
						</div>
					{:else}
						<div class="add">
							<input class="search" bind:value={d.search} oninput={() => (d.touched = true)} placeholder={tr('grid.filter.search.placeholder')} />
						</div>
						<div class="values">
							{#if d.loading}
								<p class="loading"><span class="spinner" aria-hidden="true"></span> {tr('grid.filter.loading')}</p>
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
								{d.search.trim() ? tr('grid.filter.selectAllResults') : tr('grid.filter.selectAll')}
							</label>
							{#each shown as v (v.text)}
								<label>
									<input type="checkbox" bind:checked={v.on} onchange={() => (d.touched = true)} />
									<span class="what">{v.text === '' ? tr('grid.filter.blanks') : v.text}</span>
									<span class="count">{v.count}</span>
								</label>
							{/each}
							{#if d.values.length >= 1000}<p class="menu-note">{tr('grid.filter.firstValues')}</p>{/if}
							{/if}
						</div>
					{/if}
				{/if}
				{#if d.e.t.columns[d.i]?.errors}
					<div class="add">
						<label><input type="checkbox" bind:checked={d.errors} /> {tr('grid.filter.errorsOnly', { n: d.e.t.columns[d.i]?.errors ?? 0 })}</label>
					</div>
				{/if}
				{#if d.error}<p class="menu-note error">{d.error}</p>{/if}
				{#if filterable}
					<div class="foot">
						{#if d.custom}<button onclick={() => (d.custom = null)}>{tr('grid.filter.custom.back')}</button>{/if}
						<span class="sp"></span>
						<button class="primary" disabled={d.loading && !d.custom} onclick={() => applyFilter()}>{tr('common.ok')}</button>
						<button onclick={closeFilter}>{tr('common.cancel')}</button>
					</div>
				{/if}
				{#if typed}
					<!-- Type de données de la colonne (inféré ou fixé), puis la clé du tableau. -->
					<hr />
					<div class="menu-title">{fcol.fixed ? tr('grid.type.title.fixed') : tr('grid.type.title.inferred')}</div>
					{#if d.pending}
						{@const p = d.pending}
						<p class="menu-note">
							{tr('grid.type.pending', { n: p.n, type: typeLabel(p.to) })}
						</p>
						<div class="foot">
							<span class="sp"></span>
							<button class="primary" onclick={() => chooseType(p.to, true)}>{tr('grid.type.convertAnyway')}</button>
							<button onclick={() => (d.pending = undefined)}>{tr('common.cancel')}</button>
						</div>
					{:else}
						<div class="types" role="radiogroup" aria-label={tr('grid.type.aria')}>
							{#each TYPES as t (t)}
								<button
									class="sort type"
									class:on={t === fcol.type}
									role="radio"
									aria-checked={t === fcol.type}
									title={t === fcol.type ? (fcol.fixed ? tr('grid.type.fixed.tooltip') : tr('grid.type.inferred.tooltip')) : tr('grid.type.convert.tooltip', { type: typeLabel(t) })}
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
								>{tr('grid.type.revert', { type: typeLabel(to), n: d.mix.off })}</button
							>
						{/if}
						{#if fcol.errors}<p class="menu-note error">{tr('grid.type.errors', { n: fcol.errors })}</p>{/if}
					{/if}
				{:else if fcol?.formula && source.columnsToValues}
					<!-- Colonne calculée : pas de type à choisir ; elle peut devenir une colonne de valeurs. -->
					<hr />
					<div class="menu-title">{tr('grid.type.computed')}</div>
					<button
						class="sort"
						title={tr('grid.type.toValues.tooltip')}
						onclick={() => {
							filterDlg = null;
							menuAction(() => source.columnsToValues!(d.e.t, [d.i]));
						}}>{tr('grid.menu.toValues')}</button
					>
				{/if}
				{#if source.setKey && !fcol?.formula}
					<hr />
					{#if d.e.t.key === d.i}
						<p class="menu-note">🔑 {d.e.t.autoNumber === d.i ? tr('grid.key.note.auto') : tr('grid.key.note')}</p>
						<button class="sort" onclick={() => chooseKey(false)}>{tr('grid.key.remove')}</button>
					{:else if d.keyWhy === undefined}
						<p class="menu-note loading"><span class="spinner" aria-hidden="true"></span> {tr('grid.key.checking')}</p>
					{:else if d.keyWhy === ''}
						<button class="sort" onclick={() => chooseKey(true)}>🔑 {d.e.t.key !== undefined ? tr('grid.key.use.replace') : tr('grid.key.use')}</button>
					{:else if d.keyWhy}
						<p class="menu-note">{tr('grid.key.impossible', { why: d.keyWhy })}</p>
					{/if}
					{#if d.e.t.autoNumber === d.i && d.e.t.key !== d.i}
						<p class="menu-note">{tr('grid.key.sequence')}</p>
					{/if}
				{/if}
			</div>
		{/if}
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
		{#if validDlg && dvPick}
			<!-- Plage source pointée dans la grille : le dialogue est retiré, comme le bouton « réduire » d'Excel. -->
			<div class="pick-bar" role="status">
				<span>{tr('grid.validation.pick.source')} <b>{dvPickRef || '…'}</b></span>
				<span class="sp"></span>
				<button class="primary" onclick={() => endPick(true)}>{tr('common.ok')} <span class="keys">{tr('grid.keys.enter')}</span></button>
				<button onclick={() => endPick(false)}>{tr('common.cancel')} <span class="keys">{tr('grid.keys.escape')}</span></button>
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
				aria-label={tr('grid.validation.title', { target: d.target })}
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
				<div class="menu-title" title={tr('grid.validation.title', { target: d.target })}>{tr('grid.validation.title', { target: d.target })}</div>
				<div class="dv-tabs" role="tablist">
					{#each ['options', 'input', 'error'] as const as tab (tab)}
						<button role="tab" aria-selected={d.tab === tab} class:on={d.tab === tab} onclick={() => (d.tab = tab)}>{tr(`grid.validation.tab.${tab}`)}</button>
					{/each}
				</div>
				{#if d.tab === 'options'}
					<div class="dv-row">
						<label for="dv-type">{tr('grid.validation.allow')}</label>
						<select id="dv-type" bind:value={d.type} onchange={() => ((d.error = ''), d.type === 'list' && d.mode === 'range' && refreshPreview(0))}>
							{#each DV_TYPES as ty (ty)}<option value={ty}>{tr(`grid.validation.type.${ty}`)}</option>{/each}
						</select>
					</div>
					{#if hasBounds(d.type)}
						<div class="dv-row">
							<label for="dv-op">{tr('grid.validation.data')}</label>
							<select id="dv-op" bind:value={d.op}>
								{#each DV_OPS as op (op)}<option value={op}>{tr(`grid.validation.op.${DV_OP_KEYS[op]}`)}</option>{/each}
							</select>
						</div>
						<div class="dv-row">
							<label for="dv-min">{twoBounds(d.op) ? tr('grid.validation.min') : tr('grid.validation.value')}</label>
							<input id="dv-min" type="text" spellcheck="false" bind:value={d.min} oninput={() => (d.error = '')} />
						</div>
						{#if twoBounds(d.op)}
							<div class="dv-row">
								<label for="dv-max">{tr('grid.validation.max')}</label>
								<input id="dv-max" type="text" spellcheck="false" bind:value={d.max} oninput={() => (d.error = '')} />
							</div>
						{/if}
						<p class="menu-note">{tr('grid.validation.boundHelp')}</p>
					{:else if d.type === 'custom'}
						<div class="dv-row">
							<label for="dv-f">{tr('grid.validation.formulaLabel')}</label>
							<input id="dv-f" type="text" spellcheck="false" placeholder="=ESTNUM({addrA1(active.row, active.col)})" bind:value={d.formula} oninput={() => (d.error = '')} />
						</div>
						<p class="menu-note">{tr('grid.validation.formulaHelp', { cell: addrA1(active.row, active.col) })}</p>
					{:else if d.type === 'list'}
						<div class="dv-mode" role="radiogroup" aria-label={tr('grid.validation.source')}>
							<label><input type="radio" name="dv-mode" value="list" bind:group={d.mode} onchange={() => (d.error = '')} /> {tr('grid.validation.mode.list')}</label>
							<label><input type="radio" name="dv-mode" value="range" bind:group={d.mode} onchange={() => ((d.error = ''), refreshPreview(0))} /> {tr('grid.validation.mode.range')}</label>
						</div>
						{#if d.mode === 'list'}
							<p class="menu-note">
								{tr('grid.validation.list.help')}
							</p>
							<div class="add"><textarea bind:value={d.text} rows="7" placeholder={tr('grid.validation.list.placeholder')}></textarea></div>
						{:else}
							<p class="menu-note">
								{tr('grid.validation.range.help')}
							</p>
							<div class="add dv-src">
								<input
									type="text"
									aria-label={tr('grid.validation.range.aria')}
									placeholder={tr('grid.validation.range.placeholder')}
									spellcheck="false"
									bind:value={d.src}
									oninput={() => ((d.error = ''), refreshPreview())}
									onkeydown={(ev) => {
										if (ev.key === 'Enter') applyValidation();
									}}
								/>
								<button title={tr('grid.validation.pick.tooltip')} onclick={startPick}>{tr('grid.validation.pick.button')}</button>
							</div>
							{#if d.loading}
								<p class="menu-note">{tr('grid.validation.reading')}</p>
							{:else if d.preview}
								<p class="menu-note" class:error={!!d.preview.error || !d.preview.values.length}>{dvPreview(d.preview)}</p>
							{/if}
						{/if}
					{/if}
					{#if d.type !== 'any'}
						<label class="dv-check"><input type="checkbox" bind:checked={d.blank} /> {tr('grid.validation.blank')}</label>
					{/if}
				{:else if d.tab === 'input'}
					<p class="menu-note">{tr('grid.validation.inputHelp')}</p>
					<div class="dv-row">
						<label for="dv-it">{tr('grid.validation.msgTitle')}</label>
						<input id="dv-it" type="text" maxlength="32" bind:value={d.inTitle} />
					</div>
					<div class="add"><textarea aria-label={tr('grid.validation.msgText')} placeholder={tr('grid.validation.msgText')} maxlength="255" rows="4" bind:value={d.inMsg}></textarea></div>
				{:else}
					<div class="dv-row">
						<label for="dv-st">{tr('grid.validation.styleLabel')}</label>
						<select id="dv-st" bind:value={d.errStyle}>
							{#each ['stop', 'warning', 'info'] as const as st (st)}<option value={st}>{tr(`grid.validation.style.${st}`)}</option>{/each}
						</select>
					</div>
					<p class="menu-note">{tr(`grid.validation.styleHelp.${d.errStyle}`)}</p>
					<div class="dv-row">
						<label for="dv-et">{tr('grid.validation.msgTitle')}</label>
						<input id="dv-et" type="text" maxlength="32" bind:value={d.errTitle} />
					</div>
					<div class="add"><textarea aria-label={tr('grid.validation.msgText')} placeholder={tr('grid.validation.errorPlaceholder')} maxlength="255" rows="4" bind:value={d.errMsg}></textarea></div>
				{/if}
				{#if d.error}<p class="menu-note error">{d.error}</p>{/if}
				<div class="foot">
					{#if dvRule(dvAt(active.row, active.col))}<button onclick={() => applyValidation(true)}>{tr('grid.validation.clear')}</button>{/if}
					<span class="sp"></span>
					<button class="primary" onclick={() => applyValidation()}>{tr('common.ok')}</button>
					<button
						onclick={() => {
							clearTimeout(previewTimer);
							validDlg = null;
							wrap.focus();
						}}>{tr('common.cancel')}</button
					>
				</div>
			</div>
			</div>
		{/if}
		{#if dvAlert}
			{@const d = dvAlert}
			<div class="dlg-back" role="presentation">
				<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
				<div
					class="rules dv-alert"
					role="alertdialog"
					aria-modal="true"
					aria-label={d.title || tr('grid.validation.alert.title')}
					tabindex="-1"
					use:focusFirst
					use:modal
					onkeydown={(ev) => {
						ev.stopPropagation();
						if (ev.key === 'Escape') answerDvAlert('cancel');
					}}
				>
					<div class="menu-title">{d.title || tr('grid.validation.alert.title')}</div>
					<p class="dv-alert-text" class:stop={d.style === 'stop'} class:warning={d.style === 'warning'}>{d.text}</p>
					{#if d.style === 'warning'}<p class="menu-note">{tr('grid.validation.alert.continue')}</p>{/if}
					<div class="foot">
						<span class="sp"></span>
						{#if d.style === 'stop'}
							<button class="primary" onclick={() => answerDvAlert('retry')}>{tr('grid.validation.alert.retry')}</button>
						{:else if d.style === 'warning'}
							<button class="primary" onclick={() => answerDvAlert('yes')}>{tr('grid.validation.alert.yes')}</button>
							<button onclick={() => answerDvAlert('retry')}>{tr('grid.validation.alert.no')}</button>
						{:else}
							<button class="primary" onclick={() => answerDvAlert('yes')}>{tr('common.ok')}</button>
						{/if}
						<button onclick={() => answerDvAlert('cancel')}>{tr('common.cancel')}</button>
					</div>
				</div>
			</div>
		{/if}
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
						<button class="note-ico" class:on={d.pinned} title={d.pinned ? tr('grid.note.unpin') : tr('grid.note.pin')} aria-pressed={d.pinned} onclick={togglePin}
							><Icon name="pin" size={15} /></button
						>
					{/if}
					<button class="note-ico" title={tr('grid.note.delete')} onclick={() => saveNote(true)}><Icon name="trash" size={15} /></button>
				</div>
				<textarea bind:value={d.text} rows="6" placeholder={tr('grid.note.placeholder')} aria-label={tr('grid.note.placeholder')}></textarea>
			</div>
		{/if}
		{#if listMenu}
			{@const m = listMenu}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="menu list-menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (listMenu = null)}>
				{#each m.values as v (v)}
					<button class:on={cellInfo(m.row, m.col).value === v} onclick={() => pickFromList(v)}>{v}</button>
				{/each}
				{#if m.error}<p class="menu-note error">{tr('grid.validation.listMenu.notFound', { error: m.error })}</p>{:else if m.loading && !m.values.length}<p class="menu-note">{tr('grid.validation.listMenu.reading')}</p>{:else if !m.values.length}<p class="menu-note">{tr('grid.validation.listMenu.empty')}</p>{/if}
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
				<div class="menu-title">{d.rows ? tr('grid.size.rowHeight') : tr('grid.size.colWidth')}</div>
				<div class="add"><input type="number" min="4" max="1000" bind:value={d.value} /></div>
				<div class="foot">
					<span class="sp"></span>
					<button class="primary" onclick={applySize}>{tr('common.ok')}</button>
					<button
						onclick={() => {
							sizeDlg = null;
							wrap.focus();
						}}>{tr('common.cancel')}</button
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
	/* Validation des données : onglets, lignes libellé + champ, alerte. */
	.dv-tabs {
		display: flex;
		gap: 2px;
		margin: 2px 14px 6px;
		border-bottom: 1px solid #e2e4e7;
	}
	.dv-tabs button {
		border: none;
		background: none;
		padding: 4px 8px;
		border-bottom: 2px solid transparent;
		color: #4b5563;
		cursor: pointer;
	}
	.dv-tabs button.on {
		border-bottom-color: var(--accent, #1a73e8);
		color: #111827;
	}
	.dv-row {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 3px 14px;
	}
	.dv-row label {
		width: 82px;
		flex: none;
	}
	.dv-row input,
	.dv-row select {
		flex: 1;
		min-width: 0;
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	.dv-check {
		display: flex;
		align-items: center;
		gap: 6px;
		margin: 6px 14px 0;
	}
	.valid-dlg .dv-check input {
		width: auto;
	}
	.dv-alert {
		width: 360px;
		min-width: 0;
	}
	.dv-alert-text {
		margin: 6px 14px;
		padding-left: 10px;
		border-left: 3px solid #2f80ed;
		white-space: pre-wrap;
	}
	.dv-alert-text.stop {
		border-left-color: #d93025;
	}
	.dv-alert-text.warning {
		border-left-color: #e8a400;
	}
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
	.comment-pop {
		position: absolute;
		z-index: 7;
	}
	.comment-pop.preview {
		pointer-events: none;
	}
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
