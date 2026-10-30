// Source de la feuille (SheetSource) : ce que la grille demande au classeur
// (lignes, saisies, styles, filtres, TCD, graphiques…), par DocumentService ;
// source de la démonstration sans classeur ; données de la feuille affichée. Sorti de routes/+page.svelte
// (docs/decoupage-sheet.md).
import { DocumentService, GridService } from '#bindings/internal/bridge/index.js';
import type { RuleChanges } from '#bindings/internal/canvas/models.js';
import type { Type } from '#bindings/internal/grid/models.js';
import { type SheetSource, type TypeMix } from '#lib/grid/sheet/model.js';
import { previewOf } from '#lib/ui/rowformula.js';
import { tabChoiceNotice } from '#lib/grid/tabchoice.js';
import QueryHost from '#lib/query/QueryHost.svelte';
import { refusedNotice } from '#lib/remote/share.js';
import type { ButtonConfig } from '#lib/grid/buttons.js';
import type { ObjectConfig } from '#lib/grid/objects.js';
import { typeLabel, type ColumnType } from '#lib/grid/format.js';
import type { Fetched } from '#lib/grid/cache.js';
import type { SearchResult } from '#lib/grid/findopts.js';
import { t } from '#lib/i18n/index.js';
import type { Mode } from '#lib/app/model.js';

/** Ce que createSheetSource lit et modifie dans +page.svelte. */
export interface SheetSourceHost {
	headerOffer: { sheet: number; table: string } | null;
	readonly reload: () => Promise<void>;
	notice: string;
	readonly sheetId: number;
	readonly queryHost: ReturnType<typeof QueryHost> | undefined;
	readonly selectSheet: (id: number) => Promise<void>;
	readonly err: (e: unknown) => string;
	readonly mode: Mode;
}

export function createSheetSource(sh: SheetSourceHost) {
	// --- Sources de la feuille ----------------------------------------------

	/** Clé posée d'office à la création (première colonne ID, ou 1, 2, 3…). */
	const autoKeyNote = (ev: { autoKey?: string }) => (ev.autoKey ? t('shell.notice.autoKey', { column: ev.autoKey }) : '');

	/** « Utiliser comme en-têtes » : la ligne juste au-dessus du tableau devient son en-tête. */
	async function titleToHeader(table: string) {
		sh.headerOffer = null;
		const st = await DocumentService.TitleToHeader(table);
		await sh.reload();
		if (st.notice) sh.notice = st.notice;
	}

	const docSource: SheetSource = {
		window: (t, first, count) => DocumentService.Window(sh.sheetId, t.id, first, count) as Promise<Fetched>,
		reload: () => sh.reload(),
		async setCell(row, col, raw, _target, key) {
			sh.headerOffer = null;
			const ev = await (key ? DocumentService.SetCellKeyed(sh.sheetId, row, col, raw, key) : DocumentService.SetCell(sh.sheetId, row, col, raw));
			// Relire tout de suite (l'événement doc:changed sert aux changements venus
			// d'ailleurs, et n'existe pas en mode serveur).
			await sh.reload();
			if (ev.headerOffer) sh.headerOffer = { sheet: sh.sheetId, table: ev.headerOffer };
			if (ev.keyed) return t('shell.notice.keyed', { column: ev.keyed.slice(ev.keyed.indexOf('.') + 1) });
			const undoBirth = ev.split ? t('shell.notice.undoBirth') : '';
			if (ev.created && ev.unnamed) return t('shell.notice.createdUnnamed', { table: ev.created, more: autoKeyNote(ev) + undoBirth });
			if (ev.created && ev.headless) return t('shell.notice.createdHeadless', { table: ev.created, more: autoKeyNote(ev) + undoBirth });
			if (ev.created) return t('shell.notice.created', { table: ev.created, more: autoKeyNote(ev) + (undoBirth || (ev.autoKey ? t('shell.notice.undoHint') : '')) });
			if (ev.widened) return t('shell.notice.widened', { column: ev.widened, from: typeLabel(ev.from as ColumnType), to: typeLabel(ev.to as ColumnType) });
			if (ev.invalid) return t('shell.notice.invalidValue');
		},
		editing: (row, col, on) => DocumentService.ShareEditing(sh.sheetId, row, col, on),
		undo: () => ((sh.headerOffer = null), DocumentService.Undo().then(sh.reload)),
		redo: () => DocumentService.Redo().then(sh.reload),
		async paste(row, col, block) {
			const ev = await DocumentService.Paste(sh.sheetId, row, col, block);
			await sh.reload();
			if (ev.refusedCount) return refusedNotice(ev);
			// Même règle d'en-tête qu'à la saisie (décision 0005), mêmes messages.
			if (ev.created && ev.unnamed) return t('shell.notice.createdUnnamed', { table: ev.created, more: autoKeyNote(ev) });
			if (ev.created && ev.headless) return t('shell.notice.createdHeadless', { table: ev.created, more: autoKeyNote(ev) });
			if (ev.created) return t(ev.split ? 'shell.notice.pastedSplit' : 'shell.notice.pasted', { table: ev.created, more: autoKeyNote(ev), n: block.length - 1 });
		},
		rangeStats: (t, first, count, cols) => DocumentService.RangeStats(sh.sheetId, t.id, first, count, cols),
		moveCells: (r0, c0, r1, c1, dr, dc) => DocumentService.MoveCells(sh.sheetId, r0, c0, r1, c1, dr, dc).then(sh.reload),
		clearCells: (r0, c0, r1, c1) => DocumentService.ClearCells(sh.sheetId, r0, c0, r1, c1).then(sh.reload),
		clearRange: (r0, c0, r1, c1, all) => DocumentService.ClearRange(sh.sheetId, r0, c0, r1, c1, all).then(sh.reload),
		async transfer(from, src, dst, move, values) {
			const ev = await DocumentService.Transfer(from, src, sh.sheetId, dst, move, values);
			await sh.reload();
			if (ev.refusedCount) return refusedNotice(ev);
			if (ev.created) return t('shell.notice.created', { table: ev.created, more: '' });
			if (ev.widened) return t('shell.notice.widened', { column: ev.widened, from: typeLabel(ev.from as ColumnType), to: typeLabel(ev.to as ColumnType) });
		},
		async pasteSpecial(from, src, dst, opt, values) {
			const ev = await DocumentService.PasteSpecial(from, src, sh.sheetId, dst, opt, values);
			await sh.reload();
			if (ev.refusedCount) return refusedNotice(ev);
			if (ev.created) return t('shell.notice.created', { table: ev.created, more: '' });
		},
		async pasteBlock(dst, block, styles, opt) {
			const ev = await DocumentService.PasteBlock(sh.sheetId, dst, block, styles, opt);
			await sh.reload();
			if (ev.refusedCount) return refusedNotice(ev);
			if (ev.created) return t('shell.notice.created', { table: ev.created, more: autoKeyNote(ev) });
		},
		sheetLines: (op, a, b) =>
			(op === 'insertRows'
				? DocumentService.InsertSheetRows(sh.sheetId, a, b)
				: op === 'deleteRows'
					? DocumentService.DeleteSheetRows(sh.sheetId, a, b)
					: op === 'insertCols'
						? DocumentService.InsertSheetColumns(sh.sheetId, a, b)
						: DocumentService.DeleteSheetColumns(sh.sheetId, a, b)
			).then(sh.reload),
		insertRow: (t, n) => DocumentService.InsertRow(t.name, n).then(sh.reload),
		deleteRow: (t, n) => DocumentService.DeleteRow(t.name, n).then(sh.reload),
		insertColumn: (tb, i) => DocumentService.InsertColumn(tb.name, i, t('shell.notice.defaultColumn', { n: tb.columns.length + 1 })).then(sh.reload),
		deleteColumn: (t, i) => DocumentService.DeleteColumn(t.name, i).then(sh.reload),
		suppressColumns: (t, i, j) => DocumentService.SuppressColumns(t.name, i, j).then(sh.reload),
		deleteColumnSpans: (spans) => DocumentService.DeleteColumnSpans(spans).then(sh.reload),
		convertPreview: (t, i, to) => DocumentService.ConvertPreview(t.name, i, to as Type),
		typeMix: (t, i) => DocumentService.TypeMix(t.name, i) as Promise<TypeMix>,
		setType: (t, i, to) => DocumentService.SetType(t.name, i, to as Type).then(sh.reload),
		setColumnWidth: (col, width) => DocumentService.SetColumnWidth(sh.sheetId, col, width).then(sh.reload),
		setColumnWidths: (widths) => DocumentService.SetColumnWidths(sh.sheetId, widths).then(sh.reload),
		setRowHeight: (row, height) => DocumentService.SetRowHeight(sh.sheetId, row, height).then(sh.reload),
		setRowHeights: (heights) => DocumentService.SetRowHeights(sh.sheetId, heights).then(sh.reload),
		setName: (row, col, name) => DocumentService.SetName(sh.sheetId, row, col, name).then(sh.reload),
		sortTable: (t, i, desc) => DocumentService.SortTable(t.name, i, desc).then(sh.reload),
		sortTableCustom: (t, i, order) => DocumentService.SortTableCustom(t.name, i, order).then(sh.reload),
		columnOrder: async (t, i) => {
			const o = (await DocumentService.ColumnOrder(t.name, i)).order;
			return o ? { ...o, values: o.values ?? [] } : null;
		},
		customLists: async () => ((await DocumentService.CustomLists()) ?? []).map((l) => ({ ...l, values: l.values ?? [] })),
		setCustomLists: (lists) => DocumentService.SetCustomLists(lists).then(sh.reload),
		refreshTable: (t) => sh.queryHost?.refreshOne(t.query!.id) ?? Promise.resolve(),
		setRules: (t, i, rules) => DocumentService.SetRules(t.name, i, rules).then(sh.reload),
		setLineRules: (rows, i0, i1, rules) => DocumentService.SetLineRules(sh.sheetId, rows, i0, i1, rules).then(sh.reload),
		setSheetRules: (ch) => DocumentService.SetSheetRules(sh.sheetId, ch as RuleChanges).then(sh.reload),
		ruleStats: (t, cols, req) => DocumentService.RuleStats(sh.sheetId, t.id, cols, req),
		filterValues: async (t, i) => (await DocumentService.FilterValues(t.name, i)) ?? [],
		setFilter: (t, i, f) => DocumentService.SetFilter(t.name, i, f).then(sh.reload),
		clearFilters: (t) => DocumentService.ClearFilters(t.name).then(sh.reload),
		renameTable: (t, name) => DocumentService.RenameTable(t.name, name).then(sh.reload),
		promoteHeader: (t) => DocumentService.PromoteHeader(t.name).then(sh.reload),
		headerToTitle: (t) => DocumentService.HeaderToTitle(t.name).then(sh.reload),
		titleToHeader: (t) => titleToHeader(t.name),
		setTwoLevel: (t, on) => DocumentService.SetTwoLevel(t.name, on).then(sh.reload),
		rowToNote: (t, n) => DocumentService.RowToNote(t.name, n).then(sh.reload),
		joinTable: (row, col) => DocumentService.JoinTable(sh.sheetId, row, col).then(sh.reload),
		keyCheck: (t, i) => DocumentService.KeyCheck(t.name, i),
		setKey: (t, i) => DocumentService.SetKey(t.name, i).then(sh.reload),
		clearKey: (t) => DocumentService.ClearKey(t.name).then(sh.reload),
		setHandKey: (t, column) => DocumentService.SetHandKey(t.name, column).then(sh.reload),
		forgetHandKept: (t) => DocumentService.ForgetHandKept(t.name).then(sh.reload),
		dismissKeyHint: (t) => DocumentService.DismissKeyHint(t.name).then(sh.reload),
		recalc: () => DocumentService.Recalc().then(sh.reload),
		dependencies: async (row, col) => {
			const d = await DocumentService.Dependencies(sh.sheetId, row, col);
			return { subject: d.subject, precedents: d.precedents ?? [], dependents: d.dependents ?? [] };
		},
		createPivot: async (cfg) => {
			const id = await DocumentService.CreatePivot(cfg);
			await sh.reload();
			await sh.selectSheet(id);
		},
		setPivot: (id, cfg) => DocumentService.SetPivot(id, cfg).then(sh.reload),
		// Champ calculé : une erreur (formule refusée) reste dans la boîte.
		setPivotCalc: (id, old, name, formula) =>
			DocumentService.SetPivotCalcField(id, old, name, formula).then(sh.reload, (e) => {
				throw new Error(sh.err(e));
			}),
		previewPivotFormula: async (table, formula) => previewOf(await DocumentService.PreviewPivotFormula(table, formula)),
		placePivot: (row, col, cfg) => DocumentService.PlacePivot(sh.sheetId, row, col, cfg).then(sh.reload),
		createPivotAt: (row, col, cfg) => DocumentService.CreatePivotAt(sh.sheetId, row, col, cfg).then(sh.reload),
		pivotFieldValues: async (table, field) => (await DocumentService.FieldValues(table, field)) ?? [],
		pivotValues: async (cfg, field) => (await DocumentService.PivotFieldValues(cfg as never, field)) ?? [],
		validationList: async (src) => {
			const l = await DocumentService.ValidationList(sh.sheetId, src);
			return { values: l.values ?? [], error: l.error || undefined };
		},
		validationAt: (row, col) => DocumentService.ValidationAt(sh.sheetId, row, col),
		invalidCells: async () => (await DocumentService.InvalidCells(sh.sheetId)) ?? [],
		referenceRow: (src, value) => DocumentService.ReferenceRow(src, value),
		citations: async (table, value) => (await DocumentService.Citations(table, value)) ?? [],
		createChart: (row, col, cfg) => DocumentService.CreateChart(sh.sheetId, row, col, cfg).then(sh.reload),
		setChart: (id, cfg) => DocumentService.SetChart(id, cfg).then(sh.reload),
		moveChart: (id, row, col) => DocumentService.MoveChart(id, row, col).then(sh.reload),
		placeChart: (id, row, col, offX, offY, w, h) => DocumentService.PlaceChart(id, row, col, offX, offY, w, h).then(sh.reload),
		deleteChart: (id) => DocumentService.DeleteChart(id).then(sh.reload),
		scripts: async () => (await DocumentService.Scripts()) ?? [],
		runButton: async (id) => {
			const r = await DocumentService.RunButton(id);
			if (r.error) return t('shell.notice.scriptError', { error: r.error });
			const out = r.output ?? [];
			return out.length ? out[out.length - 1] : '';
		},
		fireControl: async (id, event, value, old) => {
			const r = await DocumentService.FireControl(id, event, value, old);
			if (r.error) return t('shell.notice.scriptError', { error: r.error });
			const out = r.output ?? [];
			return out.length ? out[out.length - 1] : '';
		},
		pickFile: (title) => DocumentService.PickControlFile(title),
		createButton: (row, col, cfg: ButtonConfig) => DocumentService.CreateButton(sh.sheetId, row, col, cfg).then(async (id) => (await sh.reload(), id)),
		setButton: (id, cfg: ButtonConfig) => DocumentService.SetButton(id, cfg).then(sh.reload),
		placeButton: (id, row, col, offX, offY, w, h) => DocumentService.PlaceButton(id, row, col, offX, offY, w, h).then(sh.reload),
		deleteButton: (id) => DocumentService.DeleteButton(id).then(sh.reload),
		createObject: (row, col, cfg: ObjectConfig) => DocumentService.CreateObject(sh.sheetId, row, col, cfg as never).then(async (id) => (await sh.reload(), id)),
		createImage: (row, col, data, cfg: ObjectConfig) => DocumentService.CreateImage(sh.sheetId, row, col, data, cfg as never).then(async (id) => (await sh.reload(), id)),
		setObject: (id, cfg: ObjectConfig) => DocumentService.SetObject(id, cfg as never).then(sh.reload),
		placeObject: (id, row, col, offX, offY, w, h) => DocumentService.PlaceObject(id, row, col, offX, offY, w, h).then(sh.reload),
		placeLine: (id, row, col, offX, offY, w, h, flipH, flipV) => DocumentService.PlaceLine(id, row, col, offX, offY, w, h, flipH, flipV).then(sh.reload),
		orderObject: (id, front) => DocumentService.OrderObject(id, front).then(sh.reload),
		deleteObject: (id) => DocumentService.DeleteObject(id).then(sh.reload),
		createSparkline: (cfg) => DocumentService.CreateSparkline(sh.sheetId, cfg as never).then(sh.reload),
		setSparkline: (id, cfg) => DocumentService.SetSparkline(id, cfg as never).then(sh.reload),
		deleteSparkline: (id) => DocumentService.DeleteSparkline(id).then(sh.reload),
		imageData: (hash) => DocumentService.ImageData(hash),
		deletePivot: (id) => DocumentService.DeletePivot(id).then(sh.reload),
		movePivot: (id, row, col) => DocumentService.MovePivot(id, sh.sheetId, row, col).then(sh.reload),
		copyPivot: (id, row, col) => DocumentService.CopyPivot(id, sh.sheetId, row, col).then(sh.reload),
		duplicatesPreview: (t, cols) => DocumentService.DuplicatesPreview(t.name, cols),
		removeDuplicates: async (t, cols) => {
			const n = await DocumentService.RemoveDuplicates(t.name, cols);
			await sh.reload();
			return n;
		},
		textColumnsPreview: (r0, r1, col, o) => DocumentService.TextToColumnsPreview(sh.sheetId, r0, r1, col, o),
		textToColumns: (r0, r1, col, o) => DocumentService.TextToColumns(sh.sheetId, r0, r1, col, o).then((n) => {
			// L'assistant se ferme tout de suite ; la feuille se recharge ensuite.
			sh.reload().catch((e) => (sh.notice = sh.err(e)));
			return n;
		}),
		deleteTables: (ts) => DocumentService.DeleteTables(ts.map((t) => t.name)).then(sh.reload),
		convertToRange: (t) => DocumentService.ConvertToRange(t.name).then(sh.reload),
		columnsToValues: (t, cols) => DocumentService.ColumnsToValues(t.name, cols).then(sh.reload),
		rangeToValues: (r0, c0, r1, c1) => DocumentService.RangeToValues(sh.sheetId, r0, c0, r1, c1).then(sh.reload),
		// « Suivre l'année » (lot 4) : le message dit ce qui a été fait (colonne ajoutée, retirée ou gardée).
		setFollow: async (t, on) => {
			const st = await DocumentService.SetTableFollow(t.name, on);
			await sh.reload();
			if (st.notice) sh.notice = st.notice;
		},
		// « Filtrer l'onglet par cette colonne / ce champ » : comme « Choisir par… » de l'onglet, sur (Toutes).
		setTabChoice: async (table, column) => {
			await DocumentService.ChooseSheetBy(sh.sheetId, table, column, '');
			await sh.reload();
			sh.notice = tabChoiceNotice(column, false);
		},
		clearTabChoice: async (column) => {
			await DocumentService.ClearSheetChoice(sh.sheetId);
			await sh.reload();
			sh.notice = tabChoiceNotice(column, true);
		},
		choiceCandidates: async () => (await DocumentService.SheetChoiceCandidates(sh.sheetId)) ?? [],
		formatAsTable: (row, col) => DocumentService.FormatAsTable(sh.sheetId, row, col).then(sh.reload),
		convertSpill: (row, col, linked) => DocumentService.ConvertSpill(sh.sheetId, row, col, linked).then(sh.reload),
		moveTable: (t, row, col) => DocumentService.MoveTable(t.name, row, col).then(sh.reload),
		extractColumns: (t, i0, i1, row, col) => DocumentService.ExtractColumns(t.name, i0, i1, row, col).then(sh.reload),
		applyStyle: (r0, c0, r1, c1, patch) => DocumentService.ApplyStyle(sh.sheetId, r0, c0, r1, c1, patch).then(sh.reload),
		applyStyleParts: (parts) => DocumentService.ApplyStyleParts(sh.sheetId, parts).then(sh.reload),
		applyLineStyle: (rows, i0, i1, patch) => DocumentService.ApplyLineStyle(sh.sheetId, rows, i0, i1, patch).then(sh.reload),
		setNote: (row, col, text) => DocumentService.SetNote(sh.sheetId, row, col, text).then(sh.reload),
		pinNote: (row, col, pinned) => DocumentService.PinNote(sh.sheetId, row, col, pinned).then(sh.reload),
		setLink: (row, col, target, text) => DocumentService.SetLink(sh.sheetId, row, col, target, text).then(sh.reload),
		linkPlace: (target) => DocumentService.LinkPlace(sh.sheetId, target),
		mergeCells: async (r0, c0, r1, c1) => {
			const n = await DocumentService.MergeCells(sh.sheetId, r0, c0, r1, c1);
			await sh.reload();
			return n;
		},
		unmergeCells: (r0, c0, r1, c1) => DocumentService.UnmergeCells(sh.sheetId, r0, c0, r1, c1).then(sh.reload),
		setLinesHidden: (rows, i0, i1, hide) => DocumentService.SetLinesHidden(sh.sheetId, rows, i0, i1, hide).then(sh.reload),
		groupLines: (rows, i0, i1, group) => (group ? DocumentService.GroupLines(sh.sheetId, rows, i0, i1) : DocumentService.UngroupLines(sh.sheetId, rows, i0, i1)).then(sh.reload),
		clearOutline: () => DocumentService.ClearOutline(sh.sheetId).then(sh.reload),
		foldGroup: (rows, level, line, fold) => DocumentService.FoldGroup(sh.sheetId, rows, level, line, fold).then(sh.reload),
		showOutlineLevel: (rows, level) => DocumentService.ShowOutlineLevel(sh.sheetId, rows, level).then(sh.reload),
		setFreeze: (rows, cols) => DocumentService.SetFreeze(sh.sheetId, rows, cols).then(sh.reload),
		find: async (text, zone) => (await DocumentService.Find(sh.sheetId, text, zone)) ?? [],
		replace: async (text, by, zone) => {
			const n = await DocumentService.Replace(sh.sheetId, text, by, zone);
			await sh.reload();
			return n;
		},
		search: async (q) => (await DocumentService.Search(q)) as SearchResult,
		replaceAll: async (q, by) => {
			const r = await DocumentService.ReplaceAll(q, by);
			if (r.replaced) await sh.reload();
			return r;
		}
	};

	const demoSource: SheetSource = {
		window: (_t, first, count) => GridService.Window(first, count) as Promise<Fetched>,
		async setCell(_row, _col, raw, target) {
			if (target.rowId === undefined || target.column === undefined) return t('shell.demo.readOnly');
			// La démo ne relit pas sa mise en page : la feuille met la cellule à jour.
			return { value: await GridService.SetCell(target.rowId, target.column, raw) };
		}
	};

	const query = (sql: string) => (sh.mode === 'demo' ? GridService.Query(sql) : DocumentService.Query(sql));

	return {
		titleToHeader,
		docSource,
		demoSource,
		query
	};
}
