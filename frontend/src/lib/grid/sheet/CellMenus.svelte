<!--
	Menus contextuels de la grille (clic droit, Maj+F10) : cellule de tableau,
	en-têtes de lignes et de colonnes, cellule libre ; les entrées sont
	décrites par ctxmenu.ts, la logique est dans menus.svelte.ts.
	Sorti de Sheet.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import { allGenerated, defaultNameHint, isDefaultTableName } from '../defaultname';
	import { type PasteKind } from '../pastespecial';
	import { type CellRange } from '../mergesel';
	import { type FormatTab } from '../FormatDialog.svelte';
	import { type CommentHost } from '../comments';
	import { dismiss } from '../dismiss';
	import { followLabel } from '#lib/shell/mergesheets.js';
	import { titleAboveFilled } from '../titlehead';
	import { canFoldAll, foldTarget } from '../pivottree';
	import { citeLabel, type Citation } from '../reflink';
	import Submenu from '#lib/ui/Submenu.svelte';
	import { clip, clipped } from '#lib/ui/menu.js';
	import { brokenPivotAt } from '../pivotzone';
	import { freeCellMenu, headerMenu, linesSubmenu, pivotSubmenu, tableCellMenu, tableColumnSubmenu, tableRowSubmenu } from '../ctxmenu';
	import { sortLabels } from '../format';
	import { t as tr } from '#lib/i18n/index.js';
	import { keyLabel } from '#lib/web/platform.js';
	import { type SheetPivot, type ChartConfig, type SheetTable, type SheetData, type SheetSource, type Entry, pivotRowOf } from './model';
	import type { TabChoiceItem } from '../tabchoice';
	import type { LinesOp } from './model';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface CellMenusView {
		readonly CONFIRM_DELETE_ROWS: number;
		readonly active: { row: number; col: number; };
		readonly activeThread: () => boolean;
		readonly autoFitCols: () => void;
		readonly autoFitRows: () => void;
		readonly buttonsLayer: ({ $on?(type: string, callback: (e: any) => void): () => void; $set?(props: Partial<Record<string, any>>): void; } & Record<string, any>) | undefined;
		readonly cells: Map<string, string>;
		readonly chartClip: ChartConfig | null;
		readonly clearSelection: (range?: CellRange, all?: boolean) => Promise<void>;
		readonly comments: CommentHost | undefined;
		readonly copySelection: (cut?: boolean) => Promise<void>;
		readonly data: SheetData;
		readonly deleteNote: () => void;
		readonly deleteTable: (t: SheetTable) => void;
		readonly deleteWholeColumns: () => boolean;
		readonly designWhy: (t: { readOnly?: string; }) => string;
		readonly editLink: () => void;
		readonly editNote: () => void;
		readonly foldPivot: (p: SheetPivot, what: "expand" | "collapse" | "expandAll" | "collapseAll") => void;
		readonly followLink: (url: string) => Promise<void>;
		readonly followRef: (r: number, c: number) => Promise<void>;
		readonly formulasInSelection: () => boolean;
		freeMenu: { x: number; y: number; kind?: 'rows' | 'cols' } | null;
		readonly groupCandidate: () => Entry | undefined;
		readonly inRangeZone: (row: number, col: number) => boolean;
		readonly keptOut: (row: number, col: number) => boolean;
		readonly key: (r: number, c: number) => string;
		readonly linesOpts: (kind: "rows" | "cols") => { insert: boolean; size: boolean; hide: boolean; unhide: boolean; };
		readonly linkOf: (r: number, c: number) => string | undefined;
		readonly locked: string;
		menu: { x: number; y: number; e: Entry; n: number; i: number; header: boolean; rowOnly?: boolean; confirmDelete?: boolean } | null;
		readonly menuAction: (fn: (() => Promise<unknown>) | undefined, keepFocus?: boolean) => Promise<boolean>;
		readonly moveTo: (row: number, col: number, extend?: boolean) => void;
		readonly newComment: () => void;
		readonly noteOf: (r: number, c: number) => string | undefined;
		readonly onaddshared: ((table: string) => void) | undefined;
		readonly onpushsource: ((table: string) => void) | undefined;
		readonly onsharedata: ((table: string) => void) | undefined;
		readonly openCitation: (r: number, c: number, ci: Citation, value: string) => void;
		readonly openCustomSort: (e: Entry, i: number) => void;
		readonly openFilter: (e: Entry, i: number) => Promise<void>;
		readonly openFormat: (tab?: FormatTab) => void;
		readonly openHandKey: (t: SheetTable) => void;
		readonly openLinesDialog: (insert: boolean, x?: number, y?: number) => void;
		readonly openRules: (e: Entry, i: number) => void;
		readonly openSize: (rows: boolean, x: number, y: number) => void;
		readonly openValidation: () => void;
		readonly pasteChart: (cfg: ChartConfig) => void;
		readonly pasteClipboard: () => Promise<void>;
		readonly pasteSpecial: (kind?: PasteKind) => Promise<void>;
		readonly pickPivotSource: (p: SheetPivot) => void;
		readonly pivotAt: (r: number, c: number) => SheetPivot | undefined;
		readonly pivotSources: (p: SheetPivot) => string[];
		readonly pivots: SheetPivot[];
		refItems: {
		row: number;
		col: number;
		go?: { table: string; value: string; found?: boolean };
		cite?: { table: string; value: string; list: Citation[] | null };
	} | null;
		readonly removeLink: () => void;
		readonly removePivot: (p: SheetPivot) => void;
		readonly rowLabel: (e: Entry, r: number) => string;
		readonly rulesActive: () => void;
		readonly runTabChoice: (table: string, column: string, it: TabChoiceItem) => void;
		readonly selectRange: (r0: number, c0: number, r1: number, c1: number) => void;
		readonly selection: () => CellRange;
		readonly setHidden: (rows: boolean, hide: boolean) => void;
		readonly sheetLines: (op: LinesOp) => void;
		readonly source: SheetSource;
		readonly sourceTables: readonly string[];
		readonly spillUnder: (row: number, col: number) => { row: number; col: number; rows: string[][]; } | undefined;
		readonly startEdit: (byTyping: boolean, initial?: string) => void;
		readonly startPivotMove: (p: SheetPivot) => void;
		readonly structure: boolean;
		readonly tabChoiceFor: (table: string, column: string, o?: { formula?: boolean; field?: boolean; }) => TabChoiceItem;
		readonly tableLines: (e: Entry, op: "insertRow" | "deleteRow" | "insertColumn" | "deleteColumn", n: number) => Promise<unknown>;
		readonly titleAbove: (t: SheetTable) => boolean;
		readonly titleTableAt: (row: number, col: number) => SheetTable | undefined;
		readonly toValuesActive: () => void;
		readonly toggleFollow: (t: SheetTable, on: boolean) => void;
		readonly unhide: (rows: boolean) => void;
		readonly valueColumns: (e: Entry, i: number) => number[];
	}

	let { v }: { v: CellMenusView } = $props();
</script>

{#snippet linesSub(kind: 'rows' | 'cols', x: number, y: number)}
	<!-- Sous-menu Ligne « … » ou Colonne « … » d'un numéro ou d'une lettre : insertion et suppression, taille, affichage (ctxmenu.ts). -->
	{@const rows = kind === 'rows'}
	{@const { r0, r1, c0, c1 } = v.selection()}
	{#each linesSubmenu(v.linesOpts(kind)) as g, gi (gi)}
		{#if gi}<hr />{/if}
		{#each g as id (id)}
			{#if id === 'insert'}
				<button onclick={() => v.sheetLines(rows ? 'insertRows' : 'insertCols')}>{rows ? tr('grid.lines.insertRows', { n: r1 - r0 + 1 }) : tr('grid.lines.insertCols', { n: c1 - c0 + 1 })}</button>
			{:else if id === 'delete'}
				<button onclick={() => v.sheetLines(rows ? 'deleteRows' : 'deleteCols')}
					>{rows ? (r1 > r0 ? tr('grid.lines.deleteRows') : tr('grid.lines.deleteRow')) : c1 > c0 ? tr('grid.lines.deleteCols') : tr('grid.lines.deleteCol')}</button
				>
			{:else if id === 'size'}
				<button onclick={() => v.openSize(rows, x, y)}>{rows ? tr('grid.menu.rowHeight') : tr('grid.menu.colWidth')}</button>
			{:else if id === 'autoFit'}
				{#if rows}
					<button onclick={v.autoFitRows} title={tr('grid.menu.autoFitRows.tooltip')}>{tr('grid.menu.autoFitRows')}</button>
				{:else}
					<button onclick={v.autoFitCols} title={tr('grid.menu.autoFitCols.tooltip')}>{tr('grid.menu.autoFitCols')}</button>
				{/if}
			{:else if id === 'hide'}
				<button onclick={() => v.setHidden(rows, true)}>{tr('grid.menu.hide')}</button>
			{:else if id === 'unhide'}
				<button onclick={() => v.unhide(rows)}>{tr('grid.menu.unhide')}</button>
			{/if}
		{/each}
	{/each}
{/snippet}

{#snippet basicItem(id: string, clear: () => void, rules: () => void)}
	<!-- Entrées communes des menus de cellules (ctxmenu.ts) : presse-papiers, effacement, mise en forme, note. -->
	{#if id === 'cut'}
		<button data-shed="3"
			onclick={() => {
				v.menu = v.freeMenu = null;
				v.copySelection(true);
			}}>{tr('common.cut')} <span class="keys">{keyLabel('Ctrl+X')}</span></button
		>
	{:else if id === 'copy'}
		<button data-shed="3"
			onclick={() => {
				v.menu = v.freeMenu = null;
				v.copySelection();
			}}>{tr('common.copy')} <span class="keys">{keyLabel('Ctrl+C')}</span></button
		>
	{:else if id === 'paste'}
		<button data-shed="3"
			onclick={() => {
				v.menu = v.freeMenu = null;
				v.pasteClipboard();
			}}>{tr('common.paste')} <span class="keys">{keyLabel('Ctrl+V')}</span></button
		>
	{:else if id === 'pasteSpecial'}
		<button data-shed="3"
			onclick={() => {
				v.menu = v.freeMenu = null;
				void v.pasteSpecial();
			}}>{tr('grid.pasteSpecial.menu')} <span class="keys">{keyLabel('Ctrl+Alt+V')}</span></button
		>
	{:else if id === 'pasteChart'}
		<button
			onclick={() => {
				v.menu = v.freeMenu = null;
				if (v.chartClip) v.pasteChart(v.chartClip);
			}}>{tr('grid.chart.paste')}</button
		>
	{:else if id === 'pasteButton'}
		<button
			onclick={() => {
				v.menu = v.freeMenu = null;
				v.buttonsLayer?.pasteAt(v.active.row, v.active.col);
			}}>{tr('grid.controls.paste')}</button
		>
	{:else if id === 'clear'}
		<button onclick={clear}>{tr('grid.menu.clearContents')}</button>
	{:else if id === 'format'}
		<button data-shed="1" onclick={() => v.openFormat()}>{tr('grid.menu.formatCells')} <span class="keys">{keyLabel('Ctrl+1')}</span></button>
	{:else if id === 'rules'}
		<button data-shed="1" onclick={rules}>{tr('grid.menu.conditionalFormat')}</button>
	{:else if id === 'note'}
		{@render linkItems()}
		{@render noteItems()}
	{:else if id === 'comment'}
		<button data-shed="1" onclick={v.newComment}>{v.activeThread() ? tr('grid.comment.menuReply') : tr('grid.comment.menuNew')} <span class="keys">{keyLabel('Ctrl+Alt+M')}</span></button>
	{/if}
{/snippet}

{#snippet linkItems()}
	{#if v.source.setLink}
		{@const url = v.linkOf(v.active.row, v.active.col)}
		{#if url}
			<button
				data-shed="1"
				onclick={() => {
					v.menu = v.freeMenu = null;
					void v.followLink(url);
				}}>{tr('grid.link.open')}</button
			>
			<button data-shed="1" onclick={v.editLink}>{tr('grid.link.menuEdit')} <span class="keys">{keyLabel('Ctrl+K')}</span></button>
			<button data-shed="1" onclick={v.removeLink}>{tr('grid.link.remove')}</button>
		{:else}
			<button data-shed="1" onclick={v.editLink}>{tr('grid.link.menu')} <span class="keys">{keyLabel('Ctrl+K')}</span></button>
		{/if}
	{/if}
{/snippet}

{#snippet noteItems()}
	{#if v.noteOf(v.active.row, v.active.col)}
		<button data-shed="1" onclick={v.editNote}>{tr('grid.note.edit')} <span class="keys">{keyLabel('Maj+F2')}</span></button>
		<button data-shed="1"
			onclick={() => {
				v.menu = v.freeMenu = null;
				v.deleteNote();
			}}>{tr('grid.note.delete')}</button
		>
	{:else}
		<button data-shed="1" onclick={v.editNote}>{tr('grid.note.insert')} <span class="keys">{keyLabel('Maj+F2')}</span></button>
	{/if}
{/snippet}

{#snippet refMenuItems()}
	<!-- Colonne de référence : basculer sur la ligne citée ; clé d'un tableau : les lignes qui la citent. -->
	{#if v.refItems?.go}
		{@const g = v.refItems.go}
		{@const at = { row: v.refItems.row, col: v.refItems.col }}
		<button
			disabled={g.found === false}
			title={g.found === false ? tr('grid.refLink.missing', { value: g.value, table: g.table }) : tr('grid.refLink.go.tooltip', { value: g.value, table: g.table })}
			onclick={() => {
				v.menu = v.freeMenu = null;
				v.refItems = null;
				void v.followRef(at.row, at.col);
			}}>{tr('grid.refLink.go', { table: clip(g.table), value: clip(g.value) })} <span class="keys">{keyLabel('Ctrl+clic')}</span></button
		>
	{:else if v.refItems?.cite?.list?.length}
		{@const ci = v.refItems.cite}
		{@const list = ci.list ?? []}
		{@const at = { row: v.refItems.row, col: v.refItems.col }}
		{#if list.length === 1}
			<button
				disabled={!list[0].count}
				title={list[0].count ? tr('grid.refLink.cite.tooltip', { table: list[0].table, column: list[0].column, value: ci.value }) : undefined}
				onclick={() => v.openCitation(at.row, at.col, list[0], ci.value)}>{citeLabel(clip(ci.value), list)}</button
			>
		{:else}
			<Submenu label={citeLabel(clip(ci.value), list)}>
				{#each list as c (c.table + '[' + c.column + ']')}
					<button disabled={!c.count} title={tr('grid.refLink.cite.tooltip', { table: c.table, column: c.column, value: ci.value })} onclick={() => v.openCitation(at.row, at.col, c, ci.value)}
						>{clip(c.table)} ({c.count})</button
					>
				{/each}
			</Submenu>
		{/if}
	{/if}
{/snippet}

{#if v.menu}
	{@const m = v.menu}
	{@const col = m.e.t.columns[m.i]}
	{@const cols = !m.rowOnly}
	{@const rowItems = !m.header && !m.rowOnly}
	<!-- Structure du tableau (colonnes, en-têtes, conversion, suppression) : grisée ou retirée avec son motif, comme au ruban (feuille protégée, classeur partagé sans droit de conception, tableau en lecture seule). -->
	{@const why = v.designWhy(m.e.t)}
	{@const filtered = cols && !!v.source.clearFilters && m.e.t.columns.some((c) => c.filter)}
	{@const sortCount = (cols && v.source.sortTable ? 2 : 0) + (cols && v.source.sortTableCustom ? 1 : 0) + (cols && v.source.setFilter ? 1 : 0) + (filtered ? 1 : 0)}
	{@const toTitle = !why && cols && !!v.source.headerToTitle && m.e.t.rows > 0}
	{@const toHeader = !why && cols && v.titleAbove(m.e.t)}
	{@const promote = !why && cols && !!v.source.promoteHeader && m.e.t.rows > 0 && !m.e.t.query && !m.e.t.twoLevel}
	{@const toValues = !why && cols && v.source.columnsToValues ? v.valueColumns(m.e, m.i) : []}
	{@const refresh = !!m.e.t.query && !!v.source.refreshTable}
	{@const share = cols && !!v.onsharedata && !m.e.t.query}
	{@const addShared = cols && !!v.onaddshared && m.e.t.shared === 'private' && !m.e.t.query}
	{@const pushSource = cols && !!v.onpushsource && v.sourceTables.includes(m.e.t.name)}
	{@const twoLevel = !why && cols && !!v.source.setTwoLevel && !m.e.t.query}
	{@const follow = cols && !!v.source.setFollow && !!v.data?.choice && !m.e.t.query}
	{@const tableItems = toTitle || toHeader || promote || twoLevel || refresh || share || pushSource || follow || !!v.source.convertToRange || !!v.source.deleteTables}
	<!-- Situation (ctxmenu.ts) : sur l'en-tête, du texte ou des groupes au-dessus de ses colonnes ; sur une ligne, la 1re (tableau sans en-têtes) ou la dernière. -->
	{@const ref = !m.rowOnly && !m.header && (!!v.refItems?.go || !!v.refItems?.cite?.list?.length)}
	{@const promoteRow = !why && !m.header && !!v.source.promoteHeader && m.e.t.rows > 0 && !m.e.t.query && !m.e.t.twoLevel && m.n === 0 && allGenerated(m.e.t.columns.map((c) => c.name))}
	{@const toNote = !why && !m.header && !!v.source.rowToNote && m.n === m.e.t.rows - 1}
	{@const headAbove = m.header && !m.e.t.query && titleAboveFilled(m.e.t, (r, c) => v.cells.get(v.key(r, c)))}
	{@const columnGroups = tableColumnSubmenu({ sort: sortCount > 0, typeKey: !!v.source.setType && !col?.formula && !why, validation: !!v.source.applyStyle && !col?.formula, tabChoice: !!v.source.setTabChoice && !!col, toValues: toValues.length > 0 })}
	{@const menuGroups = m.rowOnly
		? headerMenu({
				promote: promoteRow,
				toNote: toNote,
				format: !!v.source.sheetLines && !!v.source.applyStyle,
				rules: !!v.source.sheetLines && !!v.source.setSheetRules,
				lines: linesSubmenu(v.linesOpts('rows')).length > 0,
				table: tableItems
			})
		: tableCellMenu({
				ref,
				promote: promoteRow,
				toNote: toNote,
				toHeader: toHeader && headAbove,
				groupsOn: twoLevel && headAbove && !m.e.t.twoLevel,
				groupsOff: twoLevel && m.header && !!m.e.t.twoLevel,
				format: !!v.source.applyStyle,
				rules: !!v.source.setSheetRules,
				note: !!v.source.setNote,
				comment: !!v.comments,
				sort: sortCount > 0,
				row: rowItems,
				column: cols,
				table: tableItems
			})}
	{#snippet sortItems()}
		{#if cols && v.source.sortTable}
			{@const lab = sortLabels(col?.type ?? 'text', col?.result)}
			<button onclick={() => v.menuAction(() => v.source.sortTable!(m.e.t, m.i, false))}>{lab[0]}</button>
			<button onclick={() => v.menuAction(() => v.source.sortTable!(m.e.t, m.i, true))}>{lab[1]}</button>
		{/if}
		{#if cols && v.source.sortTableCustom}
			<button onclick={() => v.openCustomSort(m.e, m.i)}>{tr('grid.pivot.menu.customSort')}</button>
		{/if}
		{#if cols && v.source.setFilter}
			<button onclick={() => v.openFilter(m.e, m.i)}>{tr('grid.menu.filterColumn')}</button>
		{/if}
		{#if filtered}
			<button onclick={() => v.menuAction(() => v.source.clearFilters!(m.e.t))}>{tr('grid.menu.clearTableFilter')}</button>
		{/if}
	{/snippet}
	<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (v.menu = null)}>
		<!-- Les noms (ligne, colonne, tableau) n'apparaissent qu'en titre de leur sous-menu, abrégés ; jamais dans les libellés. -->
		{#if m.confirmDelete}
			<div class="menu-title" class:default-name={isDefaultTableName(m.e.t.name)} title={isDefaultTableName(m.e.t.name) ? defaultNameHint() : clipped(m.e.t.name) ? m.e.t.name : undefined}>{tr('grid.menu.table', { name: clip(m.e.t.name) })}</div>
			<p class="menu-note">
				{tr('grid.menu.deleteTable.confirm', { n: m.e.t.rows + (m.e.t.hidden ?? 0) })}
			</p>
			<button onclick={() => v.deleteTable(m.e.t)}>{tr('grid.menu.deleteTable')}</button>
			<button onclick={() => (v.menu = { ...m, confirmDelete: false })}>{tr('common.cancel')}</button>
		{:else}
			<!-- Ordre (demande de Louis, ctxmenu.ts) : un sous-menu par objet, titré de son nom
			     (ligne, colonne, tableau) ; les actions propres à la situation ; le presse-papiers ;
			     effacement et mise en forme ; la note ; tri et filtre. -->
			{#snippet rowItem(id: string)}
				{#if id === 'insertAbove'}
					<button onclick={() => v.menuAction(v.source.insertRow && (() => v.tableLines(m.e, 'insertRow', m.n)))}>{tr('grid.menu.insertAbove')}</button>
				{:else if id === 'insertBelow'}
					<button onclick={() => v.menuAction(v.source.insertRow && (() => v.tableLines(m.e, 'insertRow', m.n + 1)))}>{tr('grid.menu.insertBelow')}</button>
				{:else if id === 'deleteRow'}
					<button onclick={() => v.menuAction(v.source.deleteRow && (() => v.tableLines(m.e, 'deleteRow', m.n)))}>{tr('grid.lines.deleteRow')}</button>
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
							v.menu = null;
							v.moveTo(t.row, c);
							v.startEdit(false);
						}}>{tr('grid.menu.renameColumn')}</button
					>
				{:else if id === 'typeKey'}
					<!-- Le menu ▾ de la colonne (tri, filtre, type, clé), sous son en-tête. -->
					<button onclick={() => v.openFilter(m.e, m.i)}>{tr('grid.menu.typeAndKey')}</button>
				{:else if id === 'validation'}
					<!-- Validation des données de toute la colonne (ses lignes de données). -->
					<button
						onclick={() => {
							const e = m.e;
							const c = e.t.col + m.i;
							v.menu = null;
							v.selectRange(e.t.row + 1, c, Math.max(e.last, e.t.row + 1), c);
							v.openValidation();
						}}>{tr('grid.menu.validation')}</button
					>
				{:else if id === 'tabChoice' && col}
					{@const tc = v.tabChoiceFor(m.e.t.name, col.name, { formula: !!col.formula })}
					<button title={tc.title} disabled={tc.disabled} onclick={() => v.runTabChoice(m.e.t.name, col.name, tc)}>{tc.label}</button>
				{:else if id === 'toValues'}
					<!-- Colonne calculée (ou colonnes calculées de la sélection) : ses valeurs, sans la formule. -->
					<button
						title={toValues.length > 1 ? tr('grid.menu.toValues.many.tooltip', { n: toValues.length }) : tr('grid.menu.toValues.tooltip')}
						onclick={() => v.menuAction(() => v.source.columnsToValues!(m.e.t, toValues))}>{tr('grid.menu.toValues')}</button
					>
				{:else if id === 'insertLeft'}
					<button disabled={!!why} title={why || undefined} onclick={() => v.menuAction(v.source.insertColumn && (() => v.tableLines(m.e, 'insertColumn', m.i)))}>{tr('grid.menu.insertLeft')}</button>
				{:else if id === 'insertRight'}
					<button disabled={!!why} title={why || undefined} onclick={() => v.menuAction(v.source.insertColumn && (() => v.tableLines(m.e, 'insertColumn', m.i + 1)))}>{tr('grid.menu.insertRight')}</button>
				{:else if id === 'deleteColumn'}
					<button disabled={!!why} title={why || undefined} onclick={() => v.menuAction(v.source.deleteColumn && (() => v.tableLines(m.e, 'deleteColumn', m.i)))}>{tr('grid.lines.deleteCol')}</button>
				{/if}
			{/snippet}
			{#snippet item(id: string)}
				{#if id === 'ref'}
					{@render refMenuItems()}
				{:else if id === 'promote'}
					<!-- Tableau sans en-têtes (que des « Colonne n ») : sur sa 1re ligne. -->
					<button title={tr('grid.menu.promote.tooltip')} onclick={() => v.menuAction(() => v.source.promoteHeader!(m.e.t))}>{tr('grid.menu.promote')}</button>
				{:else if id === 'toNote'}
					<!-- Seulement sur la dernière ligne de données (le moteur refuse les autres). -->
					<button title={tr('grid.menu.rowToNote.tooltip')} onclick={() => v.menuAction(() => v.source.rowToNote!(m.e.t, m.n))}>{tr('grid.menu.rowToNote')}</button>
				{:else if id === 'toHeader'}
					<!-- En-tête avec du texte tapé au-dessus de ses colonnes. -->
					<button title={tr('grid.menu.titleToHeader.tooltip')} onclick={() => v.menuAction(() => v.source.titleToHeader!(m.e.t))}>{tr('grid.menu.titleToHeader')}</button>
				{:else if id === 'groupsOn'}
					<button title={tr('grid.menu.twoLevel.on.tooltip')} onclick={() => v.menuAction(() => v.source.setTwoLevel!(m.e.t, true))}>{tr('grid.menu.twoLevel.on')}</button>
				{:else if id === 'groupsOff'}
					<!-- En-tête (ou bande) de colonnes groupées. -->
					<button title={tr('grid.menu.twoLevel.off.tooltip')} onclick={() => v.menuAction(() => v.source.setTwoLevel!(m.e.t, false))}>{tr('grid.menu.twoLevel.off')}</button>
				{:else if id === 'sort'}
					<Submenu label={tr('grid.menu.sortFilter')}>{@render sortItems()}</Submenu>
				{:else if id === 'row'}
					<!-- Titré de l'identifiant de la ligne que montre la gouttière : sa clé, sinon son numéro (rowLabel). -->
					{@const rid = v.rowLabel(m.e, m.e.t.row + 1 + m.n)}
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
							<button title={tr('grid.menu.headerToTitle.tooltip')} onclick={() => v.menuAction(() => v.source.headerToTitle!(m.e.t))}>{tr('grid.menu.headerToTitle')}</button>
						{/if}
						{#if promote}
							<button title={tr('grid.menu.promote.tooltip')} onclick={() => v.menuAction(() => v.source.promoteHeader!(m.e.t))}>{tr('grid.menu.promote')}</button>
						{/if}
						{#if toHeader}
							<button title={tr('grid.menu.titleToHeader.tooltip')} onclick={() => v.menuAction(() => v.source.titleToHeader!(m.e.t))}>{tr('grid.menu.titleToHeader')}</button>
						{/if}
						{#if twoLevel}
							{@const on = !!m.e.t.twoLevel}
								<button
									title={on
										? tr('grid.menu.twoLevel.off.tooltip')
										: tr('grid.menu.twoLevel.on.tooltip')}
									onclick={() => v.menuAction(() => v.source.setTwoLevel!(m.e.t, !on))}>{on ? tr('grid.menu.twoLevel.off') : tr('grid.menu.twoLevel.on')}</button
							>
						{/if}
						{#if follow}
							{@const on = !!m.e.t.follows}
							{@const what = followLabel(v.data?.choice?.name ?? '')}
							<button
								role="menuitemcheckbox"
								aria-checked={on}
								title={v.data?.choice?.formula
									? on
										? tr('grid.menu.follow.off.tooltip')
										: tr('grid.menu.follow.on.formula.tooltip', { formula: v.data.choice.formula })
									: on
										? tr('grid.menu.follow.off.column.tooltip', { name: v.data?.choice?.name ?? '' })
										: tr('grid.menu.follow.on.tooltip', { name: v.data?.choice?.name ?? '' })}
								onclick={() => v.toggleFollow(m.e.t, !on)}><span class="check">{on ? '✓' : ''}</span>{tr('grid.menu.follow', { what })}</button
							>
						{/if}
						{#if refresh}
							{@const t = m.e.t}
							<button title={tr('grid.menu.refresh.tooltip', { name: t.query?.name ?? '' })} onclick={() => v.menuAction(() => v.source.refreshTable!(t))}>{tr('grid.menu.refresh')}</button>
								{#if v.source.setHandKey && t.columns.some((k) => k.hand && !k.formula)}
									<button title={tr('grid.menu.handKey.tooltip')} onclick={() => v.openHandKey(t)}>{tr('grid.menu.handKey')}</button>
								{/if}
								{#if v.source.forgetHandKept && t.handKept}
									<button title={tr('grid.menu.handForget.tooltip')} onclick={() => v.menuAction(() => v.source.forgetHandKept!(t))}>{tr('grid.menu.handForget', { n: t.handKept })}</button>
								{/if}
						{/if}
						{#if addShared}
							<button title={tr('grid.menu.addShared.tooltip')} onclick={() => { const n = m.e.t.name; v.menu = null; v.onaddshared?.(n); }}>{tr('grid.menu.addShared')}</button>
						{/if}
						{#if share}
							<button title={tr('grid.menu.shareData.tooltip')} onclick={() => { const n = m.e.t.name; v.menu = null; v.onsharedata?.(n); }}>{tr('grid.menu.shareData')}</button>
						{/if}
						{#if pushSource}
							<button title={tr('grid.menu.pushSource.tooltip')} onclick={() => { const n = m.e.t.name; v.menu = null; v.onpushsource?.(n); }}>{tr('grid.menu.pushSource')}</button>
						{/if}
						{#if v.source.convertToRange}
							<button disabled={!!why} title={why || tr('grid.menu.toRange.tooltip')} onclick={() => v.menuAction(() => v.source.convertToRange!(m.e.t))}>{tr('grid.menu.toRange')}</button>
						{/if}
						{#if v.source.deleteTables}
							<hr />
							<button
								disabled={!!why}
								title={why || undefined}
								onclick={() => {
									if (m.e.t.rows + (m.e.t.hidden ?? 0) > v.CONFIRM_DELETE_ROWS) v.menu = { ...m, confirmDelete: true };
									else v.deleteTable(m.e.t);
								}}>{tr('grid.menu.deleteTable')}</button
							>
						{/if}
					</Submenu>
				{:else}
					{@render basicItem(
						id,
						() => {
							v.menu = null;
							if (m.rowOnly && v.deleteWholeColumns()) return;
							v.clearSelection();
						},
						() => (m.rowOnly ? v.rulesActive() : v.openRules(m.e, m.i))
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

{#if v.freeMenu}
	{@const m = v.freeMenu}
	{@const below = v.groupCandidate()}
	<!-- Cellules libres verrouillées (protection, classeur partagé sans droit de conception) : pas d'actions qui les changent. -->
	{@const free = !v.locked && !v.structure}
	{@const cell = !m.kind}
	{@const join = cell && free && !!v.source.joinTable && v.keptOut(v.active.row, v.active.col)}
	{@const asTable = cell && free && !!v.source.formatAsTable && v.inRangeZone(v.active.row, v.active.col)}
	{@const under = cell && free ? v.titleTableAt(v.active.row, v.active.col) : undefined}
	{@const spilled = cell && free && v.source.convertSpill ? v.spillUnder(v.active.row, v.active.col) : null}
	{@const toValues = cell && free && !!v.source.rangeToValues && !spilled && v.formulasInSelection()}
	{@const fp = cell && v.source.setPivot ? v.pivotAt(v.active.row, v.active.col) : null}
	{@const fold = fp?.rows ? fp : null}
	<!-- Tout tableau croisé, même en erreur (source introuvable) : il se supprime ; en erreur, il se rattache à un autre tableau. -->
	{@const dp = cell && v.source.deletePivot ? (v.pivotAt(v.active.row, v.active.col) ?? brokenPivotAt(v.pivots, v.active.row, v.active.col)) : null}
	{@const pv = dp ?? fold}
	{@const pasteChart = !!v.chartClip}
	{@const pasteButton = !!v.buttonsLayer?.hasClip()}
	{@const lines = !!v.source.sheetLines}
	{@const menuGroups = m.kind
		? headerMenu({
				asGroups: !!below,
				pasteChart,
				pasteButton,
				format: lines && !!v.source.applyStyle,
				rules: lines && !!v.source.setSheetRules,
				lines: linesSubmenu(v.linesOpts(m.kind)).length > 0
			})
		: freeCellMenu({
				ref: !!v.refItems?.go || !!v.refItems?.cite?.list?.length,
				asHeader: !!under,
				join,
				asTable,
				spill: !!spilled,
				toValues,
				asGroups: !!below,
				pasteChart,
				pasteButton,
				format: !!v.source.applyStyle,
				rules: !!v.source.setSheetRules,
				note: !!v.source.setNote,
				comment: !!v.comments,
				lines,
				pivot: !!pv
			})}
	{#snippet pivotItem(id: string)}
		{#if fold && id === 'expand'}
			<!-- Comme le sous-menu « Développer/Réduire » d'Excel. -->
			<button disabled={!foldTarget(pivotRowOf(fold, v.active.row), true)} onclick={() => v.foldPivot(fold, 'expand')}>{tr('grid.pivot.fold.expand')}</button>
		{:else if fold && id === 'collapse'}
			<button disabled={!foldTarget(pivotRowOf(fold, v.active.row), false)} onclick={() => v.foldPivot(fold, 'collapse')}>{tr('grid.pivot.fold.collapse')}</button>
		{:else if fold?.rows && id === 'expandAll'}
			<button disabled={!canFoldAll(fold.rows, true)} onclick={() => v.foldPivot(fold, 'expandAll')}>{tr('grid.pivot.fold.expandAll')}</button>
		{:else if fold?.rows && id === 'collapseAll'}
			<button disabled={!canFoldAll(fold.rows, false)} onclick={() => v.foldPivot(fold, 'collapseAll')}>{tr('grid.pivot.fold.collapseAll')}</button>
		{:else if dp && id === 'source'}
			<button title={tr('grid.pivot.menu.source.tooltip')} onclick={() => v.pickPivotSource(dp)}>{tr('grid.pivot.menu.source')}</button>
		{:else if dp && id === 'move'}
			<button title={tr('grid.pivot.menu.move.tooltip')} onclick={() => v.startPivotMove(dp)}>{tr('grid.pivot.menu.move')}</button>
		{:else if dp && id === 'delete'}
			<button onclick={() => v.removePivot(dp)}>{tr('grid.pivot.delete')}</button>
		{/if}
	{/snippet}
	{#snippet item(id: string)}
		{@const at = { row: v.active.row, col: v.active.col }}
		{#if id === 'ref'}
			{@render refMenuItems()}
		{:else if id === 'asHeader' && under}
			{@const t = under}
			<button title={tr('grid.menu.asHeader.tooltip')} onclick={() => v.menuAction(() => v.source.titleToHeader!(t))}>{tr('grid.menu.asHeader', { table: t.name })}</button>
		{:else if id === 'join'}
			<button title={tr('grid.menu.join.tooltip')} onclick={() => v.menuAction(() => v.source.joinTable!(at.row, at.col))}>{tr('grid.menu.join')}</button>
		{:else if id === 'asTable'}
			<button title={tr('grid.menu.asTable.tooltip')} onclick={() => v.menuAction(() => v.source.formatAsTable!(at.row, at.col))}>{tr('grid.menu.asTable')}</button>
		{:else if id === 'spillLinked' && spilled}
			{@const sp = { row: spilled.row, col: spilled.col }}
			<button title={tr('grid.menu.spillLinked.tooltip')} onclick={() => v.menuAction(() => v.source.convertSpill!(sp.row, sp.col, true))}>{tr('grid.menu.spillLinked')}</button>
		{:else if id === 'spillValues' && spilled}
			{@const sp = { row: spilled.row, col: spilled.col }}
			<button title={tr('grid.menu.spillValues.tooltip')} onclick={() => v.menuAction(() => v.source.convertSpill!(sp.row, sp.col, false))}>{tr('grid.menu.spillValues')}</button>
		{:else if id === 'toValues'}
			<button title={tr('grid.menu.rangeToValues.tooltip')} onclick={() => v.toValuesActive()}>{tr('grid.menu.toValues')}</button>
		{:else if id === 'asGroups' && below}
			<!-- Ligne juste au-dessus de l'en-tête d'un tableau : ses cases peuvent devenir les groupes d'en-têtes. -->
			<button title={tr('grid.menu.asGroups.tooltip', { table: below.t.name })} onclick={() => v.menuAction(() => v.source.setTwoLevel!(below.t, true))}>{tr('grid.menu.asGroups')}</button>
		{:else if id === 'insertDialog'}
			<button onclick={() => v.openLinesDialog(true, m.x, m.y)}>{tr('grid.lines.insert')}</button>
		{:else if id === 'clearAll'}
			<button
				title={tr('grid.menu.clearAll.tooltip')}
				onclick={() => {
					v.menu = v.freeMenu = null;
					v.clearSelection(v.selection(), true);
				}}>{tr('common.delete')}</button
			>
		{:else if id === 'linesTop' && m.kind}
			{@render linesSub(m.kind, m.x, m.y)}
		{:else if id === 'pivot' && pv}
			<!-- Le tableau croisé, titré de son nom (ou de sa source) ; sa suppression en dernier. -->
			{@const pname = pv.config.name ? tr('grid.pivot.menu.title', { name: clip(pv.config.name) }) : tr('grid.pivot.menu.titleOf', { source: clip(pv.config.source) })}
			<Submenu label={pname} title={clipped(pv.config.name || pv.config.source) ? pv.config.name || pv.config.source : undefined}>
				{#each pivotSubmenu({ fold: !!fold, source: !!dp?.error && !!v.source.setPivot && v.pivotSources(dp).length > 0, move: !!dp && !!v.source.movePivot, remove: !!dp }) as g, gi (gi)}
					{#if gi}<hr />{/if}
					{#each g as pid (pid)}{@render pivotItem(pid)}{/each}
				{/each}
			</Submenu>
		{:else}
			{@render basicItem(
				id,
				() => {
					v.menu = v.freeMenu = null;
					if (!v.deleteWholeColumns()) v.clearSelection();
				},
				v.rulesActive
			)}
		{/if}
	{/snippet}
	<div class="menu" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (v.freeMenu = null)}>
		<!-- Ordre (demande de Louis, ctxmenu.ts), comme sur un tableau : le sous-menu de
		     l'objet (tableau croisé) ; la situation ; le presse-papiers ; effacement et mise
		     en forme ; la note ; insertion et suppression. -->
		{#each menuGroups as g, gi (gi)}
			{#if gi}<hr />{/if}
			{#each g as id (id)}{@render item(id)}{/each}
		{/each}
	</div>
{/if}

<style>

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

	.menu-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}

	.menu-title.default-name {
		color: #c42b1c; /* DEFAULT_NAME_COLOR */
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

	.check {
		display: inline-block;
		width: 16px;
		color: #217346;
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
</style>
