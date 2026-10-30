<!--
	Boîtes ouvertes depuis les menus : serveur MCP, historique des versions,
	à propos, export SQL et export, analyse de scénarios, sous-total, mise en
	page, paramètres, thème.
	Sorti de routes/+page.svelte (docs/decoupage-sheet.md).
-->
<script lang="ts">
	import type { DocState } from '#bindings/internal/bridge/models.js';
	import { docFileName } from '#lib/shell/doctitle.js';
	import type { Layout } from '#bindings/internal/canvas/models.js';
	import { type FileAction } from '#lib/shell/Ribbon.svelte';
	import PageSetupDialog from '#lib/shell/PageSetupDialog.svelte';
	import WhatIf from '#lib/shell/WhatIf.svelte';
	import type { WhatIfKind } from '#lib/shell/whatif.js';
	import SubtotalDialog from '#lib/grid/SubtotalDialog.svelte';
	import type { SubtotalOptions, SubtotalTable } from '#lib/grid/subtotal.js';
	import { type PageSetup, type PageSetupTab } from '#lib/shell/pagesetup.js';
	import McpPanel from '#lib/shell/McpPanel.svelte';
	import SettingsDialog from '#lib/shell/SettingsDialog.svelte';
	import { mcpAvailable, setSetting } from '#lib/shell/settings.svelte.js';
	import ThemeDialog from '#lib/shell/ThemeDialog.svelte';
	import { themeLabel, type WorkbookTheme } from '#lib/grid/theme.js';
	import AboutDialog from '#lib/shell/AboutDialog.svelte';
	import ExportReportDialog from '#lib/shell/ExportReportDialog.svelte';
	import SqlExportDialog, { type SqlDialect, type SqlScope } from '#lib/shell/SqlExportDialog.svelte';
	import { type ExportReport } from '#lib/shell/exportReport.js';
	import VersionHistory from '#lib/shell/VersionHistory.svelte';
	import { t } from '#lib/i18n/index.js';
	import type { Mode } from '#lib/app/model.js';

	/** Ce que le composant lit et modifie dans la feuille (Sheet.svelte). */
	export interface ToolDialogsView {
		aboutOpen: boolean;
		readonly activeInfo: import("#bindings/internal/canvas/models.js").SheetInfo | undefined;
		readonly applyCustomTheme: (t: WorkbookTheme, save: boolean) => Promise<void>;
		readonly bookTheme: import("#lib/grid/theme.js").ResolvedTheme;
		docState: DocState;
		readonly err: (e: unknown) => string;
		exportOpen: boolean;
		readonly exportSql: (dialect: SqlDialect, scope: SqlScope) => Promise<void>;
		readonly exported: { notice: string; report: ExportReport; format: string } | null;
		readonly file: (a: FileAction) => Promise<void>;
		readonly layout: Layout | null;
		mcpOpen: boolean;
		readonly mode: Mode;
		notice: string;
		readonly opened: (r: import("#bindings/internal/bridge/models.js").OpenResult) => Promise<void>;
		pageDlg: PageSetupTab | null;
		readonly reload: () => Promise<void>;
		readonly runSubtotal: (o: SubtotalOptions) => Promise<void>;
		readonly savePage: (p: PageSetup, label: string) => Promise<void>;
		readonly selectSheet: (id: number) => Promise<void>;
		settingsOpen: boolean;
		readonly sheet: ({ $on?(type: string, callback: (e: any) => void): () => void; $set?(props: Partial<Record<string, any>>): void; } & Record<string, any>) | undefined;
		readonly sheetId: number;
		sqlExport: { table?: string } | null;
		subtotalDlg: { tables: SubtotalTable[]; table?: string; column?: number } | null;
		readonly themeCat: import("#lib/grid/theme.js").ThemeDef[];
		themeOpen: boolean;
		themePreview: WorkbookTheme | null;
		versionsOpen: boolean;
		whatIf: WhatIfKind | null;
	}

	let { v }: { v: ToolDialogsView } = $props();
</script>

{#if v.mcpOpen && mcpAvailable()}
	<McpPanel onclose={() => (v.mcpOpen = false)} />
{/if}

{#if v.versionsOpen && v.mode === 'doc'}
	<VersionHistory
		onopened={async (r) => {
			v.versionsOpen = false;
			await v.opened(r);
		}}
		onsaved={(st) => {
			v.docState = st;
			if (st.notice) v.notice = st.notice;
		}}
		onclose={() => (v.versionsOpen = false)}
	/>
{/if}

{#if v.aboutOpen}
	<AboutDialog onclose={() => (v.aboutOpen = false)} />
{/if}

{#if v.sqlExport}
	<SqlExportDialog table={v.sqlExport.table} onexport={v.exportSql} onclose={() => (v.sqlExport = null)} />
{/if}

{#if v.exportOpen && v.exported}
	<ExportReportDialog report={v.exported.report} format={v.exported.format} onclose={() => (v.exportOpen = false)} />
{/if}

{#if v.whatIf && v.mode === 'doc'}
	<WhatIf
		kind={v.whatIf}
		sheet={v.sheetId}
		active={v.sheet?.activeCell() ?? { row: 0, col: 0 }}
		selection={v.sheet?.selectedRange() ?? { r0: 0, c0: 0, r1: 0, c1: 0 }}
		onchanged={async (st) => {
			await v.reload();
			if (st?.notice) v.notice = st.notice;
		}}
		onsheet={async (id) => {
			await v.reload();
			await v.selectSheet(id);
		}}
		onclose={() => (v.whatIf = null)}
	/>
{/if}

{#if v.subtotalDlg && v.mode === 'doc'}
	<SubtotalDialog tables={v.subtotalDlg.tables} table={v.subtotalDlg.table} column={v.subtotalDlg.column} onok={v.runSubtotal} oncancel={() => ((v.subtotalDlg = null), v.sheet?.focus())} />
{/if}

{#if v.pageDlg && v.mode === 'doc' && v.layout}
	<PageSetupDialog
		setup={v.layout.pageSetup}
		sheet={v.activeInfo?.name ?? ''}
		file={docFileName(v.docState)}
		selection={v.sheet?.selectedRange()}
		tab={v.pageDlg}
		onapply={(p) => {
			v.pageDlg = null;
			void v.savePage(p, t('shell.pageSetup.dialog.title'));
		}}
		onprint={(p) => {
			v.pageDlg = null;
			void v.savePage(p, t('shell.pageSetup.dialog.title')).then(() => v.file('print'));
		}}
		onclose={() => (v.pageDlg = null)}
	/>
{/if}

{#if v.settingsOpen}
	<SettingsDialog onclose={() => (v.settingsOpen = false)} onmcp={() => { v.settingsOpen = false; v.mcpOpen = true; }} />
{/if}

{#if v.themeOpen}
	<ThemeDialog
		initial={{ id: v.bookTheme.id ?? undefined, name: themeLabel(v.bookTheme), table: v.bookTheme.table.base, source: v.bookTheme.source.base, accents: [v.bookTheme.colors.accent3, v.bookTheme.colors.accent4, v.bookTheme.colors.accent5, v.bookTheme.colors.accent6], dark: v.bookTheme.colors.dk2, light: v.bookTheme.colors.lt2 }}
		mine={v.themeCat.filter((t) => t.origin === 'user')}
		onpreview={(t) => (v.themePreview = t)}
		onapply={v.applyCustomTheme}
		ondelete={(id) => setSetting(`ui.themes.${id}`, null).catch((e) => (v.notice = v.err(e)))}
		onclose={() => {
			v.themeOpen = false;
			v.themePreview = null;
		}}
	/>
{/if}

