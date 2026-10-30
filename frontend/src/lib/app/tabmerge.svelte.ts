// Regrouper les onglets aux mêmes colonnes : proposition, menu, fusion. Sorti de routes/+page.svelte
// (docs/decoupage-sheet.md).
import { DocumentService } from '#bindings/internal/bridge/index.js';
import type { DocState } from '#bindings/internal/bridge/models.js';
import { docFile } from '#lib/shell/doctitle.js';
import type { Layout } from '#bindings/internal/canvas/models.js';
import { initialPlan, pickOffer, planForSheets, type OfferMoment } from '#lib/shell/mergesheets.js';
import type { MergePlan, SheetGroup } from '#bindings/internal/canvas/models.js';
import { t } from '#lib/i18n/index.js';

/** Ce que createTabMerge lit et modifie dans +page.svelte. */
export interface TabMergeHost {
	readonly similar: SheetGroup[];
	readonly offered: Set<string>;
	mergeOffer: SheetGroup | null;
	mergeGroup: SheetGroup | null;
	notice: string;
	mergeOthers: { table: string; sheet: string }[];
	mergePlan: MergePlan | null;
	docState: DocState;
	readonly err: (e: unknown) => string;
	readonly layout: Layout | null;
	readonly reload: () => Promise<void>;
	readonly selectSheet: (id: number) => Promise<void>;
}

export function createTabMerge(h: TabMergeHost) {
	function offerMerge(moment: OfferMoment, sheet?: number) {
		const g = pickOffer(h.similar, h.offered, moment, sheet);
		if (!g) return;
		h.offered.add(g.id);
		h.mergeOffer = g;
	}

	/** Tableaux du classeur (tables.list), avec leur feuille. */
	async function workbookTables(): Promise<{ table: string; sheet: string }[]> {
		const list = ((await DocumentService.Call('tables.list', {}).catch(() => [])) ?? []) as { name: string; kind: string; sheet: string }[];
		return list.filter((t) => t.kind === 'table').map((t) => ({ table: t.name, sheet: t.sheet }));
	}

	async function openMerge(plan: MergePlan | null, group: SheetGroup | null = null) {
		h.mergeOffer = null;
		h.mergeGroup = group;
		if (!plan) {
			h.notice = t('shell.merge.notice.none');
			return;
		}
		h.mergeOthers = await workbookTables();
		h.mergePlan = plan;
	}

	/** Données › Regrouper des onglets… : le plus grand groupe trouvé, même peu sûr. */
	function openMergeMenu() {
		openMerge(h.similar[0] ? initialPlan(h.similar[0], docFile(h.docState)) : null, h.similar[0] ?? null).catch((e) => (h.notice = h.err(e)));
	}

	/** Clic droit sur des onglets › Regrouper des onglets… */
	async function openMergeFor(ids: number[]) {
		const all = await workbookTables();
		const names = new Map((h.layout?.sheets ?? []).map((s) => [s.id, s.name]));
		const plan = planForSheets(h.similar, ids, (id) => all.filter((t) => t.sheet === names.get(id)).map((t) => t.table), docFile(h.docState));
		if (!plan) {
			h.notice = t('shell.merge.notice.noTables');
			return;
		}
		await openMerge(plan, h.similar.find((g) => g.tables?.some((t) => plan.tables?.includes(t.table))) ?? null);
	}

	async function dismissGroup(g: SheetGroup | null | undefined) {
		if (!g) return;
		if (h.mergeOffer?.id === g.id) h.mergeOffer = null;
		try {
			h.docState = await DocumentService.DismissSimilar(g.id);
			await h.reload();
			h.notice = t('shell.merge.notice.dismissed');
		} catch (e) {
			h.notice = h.err(e);
		}
	}

	async function doMerge(plan: MergePlan): Promise<boolean> {
		try {
			const r = await DocumentService.MergeSheets(plan);
			await h.selectSheet(r.sheet);
			await h.reload();
			h.notice = r.notice;
			return true;
		} catch (e) {
			h.notice = h.err(e);
			return false;
		}
	}

	return {
		offerMerge,
		openMerge,
		openMergeMenu,
		openMergeFor,
		dismissGroup,
		doMerge
	};
}
