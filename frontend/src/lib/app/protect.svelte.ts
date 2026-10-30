// Révision › Protéger (feuille, structure, classeur), thème de couleur des
// tableaux, zoom, commentaires, actions sur les onglets. Sorti de routes/+page.svelte
// (docs/decoupage-sheet.md).
import { DocumentService } from '#bindings/internal/bridge/index.js';
import type { DocState } from '#bindings/internal/bridge/models.js';
import type { Layout } from '#bindings/internal/canvas/models.js';
import Sheet from '#lib/grid/Sheet.svelte';
import { visibleSheet } from '#lib/shell/sheets.js';
import { setSetting } from '#lib/shell/settings.svelte.js';
import { themeExtras, themeId, themeLabel, workbookTheme, type WorkbookTheme } from '#lib/grid/theme.js';
import { isThemeRef } from '#lib/grid/colors.js';
import type { Thread } from '#lib/grid/comments.js';
import { clampZoom } from '#lib/grid/zoom.js';
import { t } from '#lib/i18n/index.js';
import type { Mode } from '#lib/app/model.js';
import type { ResolvedTheme } from '../grid/theme';
import type { ThemeDef } from '../grid/theme';

/** Ce que createProtect lit et modifie dans +page.svelte. */
export interface ProtectHost {
	docState: DocState;
	readonly reload: () => Promise<void>;
	notice: string;
	readonly err: (e: unknown) => string;
	readonly mode: Mode;
	readonly layoutShared: boolean;
	readonly activeDoc: boolean;
	readonly bookTheme: ResolvedTheme;
	themeOpen: boolean;
	readonly themeCat: ThemeDef[];
	themePreview: WorkbookTheme | null;
	zoomSet: Record<string, number>;
	readonly zoomKey: string;
	sheetId: number;
	zoomTimer: number;
	readonly selectSheet: (id: number) => Promise<void>;
	readonly sheet: ReturnType<typeof Sheet> | undefined;
	readonly layout: Layout | null;
	pw1: string;
	pw2: string;
	pwError: string;
	bookLock: 'tabs' | 'structure' | 'sheets';
	protecting: { what: 'sheet' | 'structure' | 'workbook'; id: number; name: string; off: boolean } | null;
	deleting: { id: number; name: string; items: string[] } | null;
}

export function createProtect(h: ProtectHost) {
	async function setBookTheme(th: WorkbookTheme) {
		try {
			h.docState = await DocumentService.SetTheme(th);
			await h.reload();
			h.notice = t('shell.notice.theme', { name: themeLabel(th) });
		} catch (e) {
			h.notice = h.err(e);
		}
	}

	// Une couleur du thème posée dans un classeur sans thème propre : le thème
	// appliqué (défaut du poste) y est gardé, pour que la couleur reste la même
	// ailleurs et à l'export (décision 0017).
	function keepThemeFor(patch: Record<string, unknown>) {
		if (h.mode !== 'doc' || h.docState.theme || h.docState.readOnly || h.layoutShared || h.activeDoc) return;
		// Bords « style couleur » et stylo des bordures : la couleur après le style.
		const edgeTheme = ['bt', 'bb', 'bl', 'br'].some((k) => typeof patch[k] === 'string' && isThemeRef((patch[k] as string).split(' ')[1])) || isThemeRef((patch.pen as { color?: string } | undefined)?.color);
		if (!['fg', 'bg', 'bc'].some((k) => isThemeRef(patch[k])) && !edgeTheme) return;
		const c = h.bookTheme.colors;
		DocumentService.SetTheme({ id: h.bookTheme.id ?? undefined, name: h.bookTheme.name, table: c.accent1, source: c.accent2, accents: [c.accent3, c.accent4, c.accent5, c.accent6], dark: c.dk2, light: c.lt2 })
			.then((d) => (h.docState = d))
			.catch(() => {});
	}

	function chooseTheme(id: string) {
		if (id === 'custom') {
			h.themeOpen = true;
			return;
		}
		const t = h.themeCat.find((x) => x.id === id);
		if (t) setBookTheme(workbookTheme(t));
	}

	async function applyCustomTheme(t: WorkbookTheme, save: boolean) {
		h.themeOpen = false;
		h.themePreview = null;
		if (save) {
			const id = themeId(t.name, h.themeCat.map((x) => x.id));
			try {
				await setSetting(`ui.themes.${id}`, { name: t.name, table: t.table, source: t.source, ...themeExtras(t) });
				t = { ...t, id };
			} catch (e) {
				h.notice = h.err(e);
			}
		}
		await setBookTheme(t);
	}

	function setZoom(z: number) {
		const v = clampZoom(z);
		h.zoomSet = { ...h.zoomSet, [h.zoomKey]: v };
		if (h.mode !== 'doc') return;
		const id = h.sheetId;
		clearTimeout(h.zoomTimer);
		h.zoomTimer = window.setTimeout(() => {
			DocumentService.SetZoom(id, Math.round(v * 100))
				.then((st) => (h.docState = st))
				.catch((e) => (h.notice = h.err(e)));
		}, 400);
	}

	/** Volet Commentaires : la feuille du fil, sa cellule, le fil ouvert. */
	async function gotoThread(th: Thread) {
		if (th.sheet !== h.sheetId) await h.selectSheet(th.sheet);
		h.sheet?.showComment(th.row, th.col);
	}

	/** Protège, ou ôte la protection (mot de passe demandé s'il y en a un). */
	function toggleProtect(what: 'sheet' | 'structure' | 'workbook', id = h.sheetId) {
		if (h.mode !== 'doc' || !h.layout) return;
		const s = h.layout.sheets?.find((x) => x.id === id);
		const on = what === 'sheet' ? !!s?.protected : what === 'structure' ? !!s?.structureProtected : !!h.layout.workbookProtected;
		const hasPw = what === 'sheet' ? !!s?.password : what === 'structure' ? !!s?.structurePassword : !!h.layout.workbookPassword;
		const name = what === 'workbook' ? h.docState.title : (s?.name ?? '');
		if (on && !hasPw) {
			applyProtect(what, id, name, true, '');
			return;
		}
		h.pw1 = h.pw2 = h.pwError = '';
		h.bookLock = 'tabs';
		h.protecting = { what, id, name, off: on };
	}

	async function applyProtect(what: 'sheet' | 'structure' | 'workbook', id: number, name: string, off: boolean, pw: string, lock: typeof h.bookLock = 'tabs') {
		try {
			if (what === 'sheet') await (off ? DocumentService.UnprotectSheet(id, pw) : DocumentService.ProtectSheet(id, pw));
			else if (what === 'structure') await (off ? DocumentService.UnprotectStructure(id, pw) : DocumentService.ProtectStructure(id, pw));
			else await (off ? DocumentService.UnprotectWorkbook(pw) : DocumentService.ProtectWorkbook(pw, lock === 'sheets', lock === 'structure'));
			h.protecting = null;
			await h.reload();
			h.notice =
				what === 'sheet'
					? off
						? t('shell.protect.notice.sheetOff', { name })
						: t('shell.protect.notice.sheetOn', { name })
					: what === 'structure'
						? off
							? t('shell.protect.notice.structureOff', { name })
							: t('shell.protect.notice.structureOn', { name })
						: off
							? t('shell.protect.notice.bookOff')
							: lock === 'sheets'
								? t('shell.protect.notice.bookSheets')
								: lock === 'structure'
									? t('shell.protect.notice.bookStructure')
									: t('shell.protect.notice.bookTabs');
		} catch (e) {
			if (h.protecting) h.pwError = h.err(e);
			else h.notice = h.err(e);
		}
	}

	function confirmProtect() {
		const p = h.protecting;
		if (!p) return;
		if (!p.off && h.pw1 !== h.pw2) {
			h.pwError = t('shell.protect.mismatch');
			return;
		}
		applyProtect(p.what, p.id, p.name, p.off, h.pw1, p.what === 'workbook' ? h.bookLock : 'tabs');
	}

	/** Supprimer une feuille : tout son contenu part avec elle (un Ctrl+Z rend tout) ; demandé d'abord si elle n'est pas vide. */
	async function askDeleteSheet(id: number) {
		try {
			// Le contenu à nommer seulement : rien à calculer (LayoutFirst).
			const l = await DocumentService.LayoutFirst(id);
			const name = l.sheets?.find((x) => x.id === id)?.name ?? '';
			const items: string[] = [];
			const add = (n: number, key: 'tables' | 'pivots' | 'charts' | 'buttons' | 'cells', names?: string[]) => {
				if (!n) return;
				const what = t(`shell.deleteSheet.items.${key}`, { n });
				items.push(names?.length ? t('shell.deleteSheet.withNames', { what, names: names.slice(0, 5).join(', ') + (names.length > 5 ? '…' : '') }) : what);
			};
			const tables = l.tables ?? [];
			add(tables.length, 'tables', tables.map((tb) => t('shell.deleteSheet.tableRows', { name: tb.name, n: tb.total })));
			add(l.pivots?.length ?? 0, 'pivots', l.pivots?.map((x) => x.config.name ?? ''));
			add(l.charts?.length ?? 0, 'charts');
			add(l.buttons?.length ?? 0, 'buttons');
			add(l.cells?.length ?? 0, 'cells');
			if (!items.length) return sheetAction(() => DocumentService.DeleteSheet(id));
			h.deleting = { id, name, items };
		} catch (e) {
			h.notice = h.err(e);
		}
	}

	async function sheetAction(fn: () => Promise<unknown>) {
		try {
			await fn();
			await h.reload();
			if (visibleSheet(h.layout?.sheets ?? [], h.sheetId) !== h.sheetId) {
				h.sheetId = visibleSheet(h.layout?.sheets ?? [], h.sheetId);
				await h.reload();
			}
		} catch (e) {
			h.notice = h.err(e);
		}
	}

	return {
		keepThemeFor,
		chooseTheme,
		applyCustomTheme,
		setZoom,
		gotoThread,
		toggleProtect,
		confirmProtect,
		askDeleteSheet,
		sheetAction
	};
}
