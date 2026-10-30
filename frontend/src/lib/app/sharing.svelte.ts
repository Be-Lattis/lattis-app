// Classeur partagé : conflits et modifications en attente, ajout d'un
// tableau au partage, copie locale, détacher ou retirer, relecture. Sorti de routes/+page.svelte
// (docs/decoupage-sheet.md).
import { tick } from 'svelte';
import { DocumentService } from '#bindings/internal/bridge/index.js';
import type { DocState, FormulaHelp, OpenResult, RecentFile } from '#bindings/internal/bridge/models.js';
import Sheet from '#lib/grid/Sheet.svelte';
import { type View } from '#lib/shell/Ribbon.svelte';
import { popBack, pushBack, type RefSpot } from '#lib/grid/reflink.js';
import { sharedWorkbooksEnabled } from '#lib/shell/settings.svelte.js';
import { isWeb } from '#lib/web/platform.js';
import QueryHost from '#lib/query/QueryHost.svelte';
import type { State as CoEditState } from '#bindings/internal/coedit/models.js';
import { pushSummary, type SourceLink } from '#lib/remote/source.js';
import type { Conflict, Held } from '#bindings/internal/remote/collab/models.js';
import { detachQuestion, removeQuestion, retryNotice, serverLabel } from '#lib/remote/share.js';
import type { State as ShareState } from '#bindings/internal/remote/workbook/models.js';
import { syncCall } from '#lib/shell/busy.js';
import { t } from '#lib/i18n/index.js';
import type { Mode } from '#lib/app/model.js';

/** Ce que createSharing lit et modifie dans +page.svelte. */
export interface SharingHost {
	conflicts: Conflict[];
	held: Held[];
	share: ShareState | null;
	notice: string;
	readonly err: (e: unknown) => string;
	docState: DocState;
	readonly reload: () => Promise<void>;
	conflictBusy: boolean;
	readonly coedit: CoEditState | null;
	conflictsOpen: boolean;
	view: View;
	sheetId: number;
	readonly sheet: ReturnType<typeof Sheet> | undefined;
	mode: Mode;
	readonly help: FormulaHelp | null;
	refBack: RefSpot[];
	sourceTables: string[];
	shareRetrying: boolean;
	readonly opened: (r: OpenResult) => Promise<void>;
	readonly closeDoc: (save?: boolean) => Promise<void>;
	detachAsk: string[] | null;
	removeAsk: { question: string; lines: string[] } | null;
	removing: boolean;
	shareReloading: boolean;
	readonly keepSheet: () => Promise<void>;
	workbook: number;
	readonly queryHost: ReturnType<typeof QueryHost> | undefined;
	readonly workbookReady: () => void;
	readonly loadRecent: () => Promise<RecentFile[]>;
}

export function createSharing(sh: SharingHost) {
	async function loadConflicts() {
		try {
			sh.conflicts = (await DocumentService.ShareConflicts()) ?? [];
			sh.held = sh.share?.held ? ((await DocumentService.ShareHeld()) ?? []) : [];
		} catch (e) {
			sh.notice = sh.err(e);
		}
	}

	/** Classeur partagé en direct : ajoute un tableau qui n'est que sur ce poste au classeur du serveur (après confirmation). */
	async function addToShared(table: string): Promise<boolean> {
		try {
			const why = await DocumentService.ShareCanAddTable(table);
			if (why) {
				sh.notice = why;
				return false;
			}
			const ok = confirm(
				t('shell.share.addTableConfirm', { table, workbook: sh.share?.title ?? '' })
			);
			if (!ok) return false;
			sh.notice = t('shell.share.addingTable', { table });
			sh.docState = await syncCall(DocumentService.ShareAddTable(table));
			sh.notice = sh.docState.notice || t('shell.share.tableAdded', { table });
			sh.share = await DocumentService.ShareState().catch(() => sh.share);
			await sh.reload();
			return true;
		} catch (e) {
			sh.notice = sh.err(e);
			return false;
		}
	}

	async function resolveConflict(c: Conflict, mine: boolean) {
		sh.conflictBusy = true;
		try {
			sh.docState = await DocumentService.ShareResolve(c.id, mine);
			await sh.reload();
			await loadConflicts();
			sh.notice = mine ? (sh.coedit?.active ? t('shell.conflicts.mineAll') : t('shell.conflicts.mineServer')) : t('shell.conflicts.theirs');
			if (!sh.conflicts.length) sh.conflictsOpen = false;
		} catch (e) {
			sh.notice = sh.err(e);
		} finally {
			sh.conflictBusy = false;
		}
	}

	/** Volet Conflits : renvoie (retry) ou abandonne les modifications mises de côté d'un tableau. */
	async function resolveHeld(h: Held, retry: boolean) {
		sh.conflictBusy = true;
		try {
			sh.docState = await syncCall(DocumentService.ShareHeldResolve(h.table, retry));
			sh.share = await DocumentService.ShareState().catch(() => sh.share);
			await sh.reload();
			await loadConflicts();
			sh.notice = sh.docState.notice || (retry ? t('shell.conflicts.heldRetried', { table: h.name }) : t('shell.conflicts.heldDropped', { table: h.name }));
			if (!sh.conflicts.length && !sh.held.length) sh.conflictsOpen = false;
		} catch (e) {
			sh.notice = sh.err(e);
		} finally {
			sh.conflictBusy = false;
		}
	}

	/** Volet Conflits : garde toutes mes valeurs (mine) ou toutes celles des collègues. */
	async function resolveAllConflicts(mine: boolean) {
		sh.conflictBusy = true;
		let n = 0;
		try {
			for (const c of sh.conflicts) {
				sh.docState = await DocumentService.ShareResolve(c.id, mine);
				n++;
			}
			sh.notice = mine ? (sh.coedit?.active ? t('shell.conflicts.mineAllCount', { n }) : t('shell.conflicts.mineServerCount', { n })) : t('shell.conflicts.theirsCount', { n });
		} catch (e) {
			sh.notice = sh.err(e);
		} finally {
			await sh.reload().catch(() => {});
			await loadConflicts();
			if (!sh.conflicts.length) sh.conflictsOpen = false;
			sh.conflictBusy = false;
		}
	}

	/** Volet Conflits : va sur la cellule en conflit. */
	async function gotoConflict(c: Conflict) {
		if (!c.found) return;
		sh.view = 'grid';
		if (c.sheet !== sh.sheetId) {
			sh.sheet?.closeOverlays();
			sh.sheetId = c.sheet;
			await sh.reload();
		}
		await tick();
		await new Promise((r) => requestAnimationFrame(r));
		sh.sheet?.selectRange(c.row, c.col, c.row, c.col);
		sh.sheet?.focus();
	}
	/**
	 * Point de carte cliqué (feuille ou document) : va sur la feuille du
	 * tableau et sélectionne la ligne du point, retrouvée par son identifiant
	 * (le tri ou le filtre du moment sont suivis).
	 */
	async function gotoRow(table: string, id: number) {
		if (sh.mode !== 'doc') return;
		try {
			const spot = await DocumentService.LocateRow(table, id);
			sh.view = 'grid';
			if (spot.sheet !== sh.sheetId) {
				sh.sheet?.closeOverlays();
				sh.sheetId = spot.sheet;
				await sh.reload();
			}
			await tick();
			await new Promise((r) => requestAnimationFrame(r));
			const last = spot.col + Math.max(1, spot.cols) - 1;
			// Ligne masquée par le filtre : l'en-tête du tableau est sélectionné.
			sh.sheet?.selectRange(spot.row, spot.col, spot.row, last);
			if (spot.hidden) sh.notice = t('shell.ref.hiddenRow', { table: spot.table });
			sh.sheet?.focus();
		} catch (e) {
			sh.notice = sh.err(e);
		}
	}

	/** Basculer sur la ligne id de table (filter : sa colonne d'abord filtrée sur la valeur, « Lignes qui citent »). */
	async function followReference(to: { table: string; id: number; key?: string; filter?: { column: string; value: string } }, from: { row: number; col: number; label: string }) {
		if (sh.mode !== 'doc') return;
		try {
			const tb = sh.help?.tables?.find((x) => x.name === to.table);
			if (to.filter && tb) {
				const i = (tb.columns ?? []).indexOf(to.filter.column);
				if (i >= 0) await DocumentService.SetFilter(to.table, i, { values: [to.filter.value] });
			}
			const spot = await DocumentService.LocateRow(to.table, to.id);
			sh.refBack = pushBack(sh.refBack, { sheet: sh.sheetId, ...from });
			sh.view = 'grid';
			if (spot.sheet !== sh.sheetId || to.filter) {
				sh.sheet?.closeOverlays();
				sh.sheetId = spot.sheet;
				await sh.reload();
			}
			await tick();
			await new Promise((r) => requestAnimationFrame(r));
			const k = to.key && tb ? Math.max(0, (tb.columns ?? []).indexOf(to.key)) : 0;
			sh.sheet?.selectRange(spot.row, spot.col + k, spot.row, spot.col + k);
			sh.sheet?.flashRow(spot.row, spot.col, spot.col + Math.max(1, spot.cols) - 1);
			sh.sheet?.focus();
			sh.notice = spot.hidden ? t('shell.ref.hiddenRow', { table: spot.table }) : to.filter ? t('shell.ref.filtered', { table: to.table, column: to.filter.column, value: to.filter.value }) : t('shell.ref.back', { label: from.label });
		} catch (e) {
			sh.notice = sh.err(e);
		}
	}

	/** Alt+← : revenir à l'endroit d'avant le dernier « Basculer sur ». */
	async function refGoBack() {
		const { spot, rest } = popBack(sh.refBack);
		if (!spot) return;
		sh.refBack = rest;
		try {
			if (spot.sheet !== sh.sheetId) {
				sh.sheet?.closeOverlays();
				sh.sheetId = spot.sheet;
				await sh.reload();
			}
			await tick();
			await new Promise((r) => requestAnimationFrame(r));
			sh.sheet?.selectRange(spot.row, spot.col, spot.row, spot.col);
			sh.sheet?.flashRow(spot.row, spot.col, spot.col);
			sh.sheet?.focus();
			sh.notice = '';
		} catch (e) {
			sh.notice = sh.err(e);
		}
	}

	async function loadSources() {
		if (sh.mode !== 'doc' || !sharedWorkbooksEnabled()) {
			sh.sourceTables = [];
			return;
		}
		const links = ((await DocumentService.TableSources().catch(() => null)) ?? []) as unknown as SourceLink[];
		sh.sourceTables = links.map((l) => l.tableName).filter((n) => !!n);
	}
	async function pushSource(table: string) {
		sh.notice = t('shell.share.sourceUpdating', { table });
		try {
			const l = (await DocumentService.PushTableSource(table)) as unknown as SourceLink;
			sh.notice = pushSummary(table, l.lastResult, l.blanked ?? 0);
		} catch (e) {
			sh.notice = t('shell.share.sourceFailed', { table, error: sh.err(e) });
		}
	}

	/** Serveur injoignable : nouvel essai tout de suite (bandeau, Fichier › Réessayer la connexion). */
	async function retryShare() {
		if (sh.shareRetrying || !sh.share?.shared) return;
		sh.shareRetrying = true;
		sh.notice = retryNotice();
		try {
			await DocumentService.ShareRetry();
			// La réponse arrive par share:changed ; sans elle, le serveur ne répond toujours pas.
			for (let i = 0; i < 20 && sh.share && !sh.share.online; i++) await new Promise((r) => setTimeout(r, 200));
			if (sh.share?.online) sh.notice = t('shell.share.connected', { server: serverLabel(sh.share) });
			else if (sh.share) sh.notice = t('shell.share.stillDown', { server: serverLabel(sh.share) });
		} catch (e) {
			sh.notice = sh.err(e);
		} finally {
			sh.shareRetrying = false;
		}
	}

	/** « Travailler sur une copie locale » : copie sans lien avec le serveur, enregistrée puis ouverte. */
	async function shareLocalCopy() {
		try {
			await sh.opened(await DocumentService.ShareLocalCopy());
		} catch (e) {
			sh.notice = sh.err(e);
		}
	}

	/** Classeur disparu du serveur : « Retirer des récents » le ferme et efface son cache (après confirmation s'il reste des modifications non envoyées). */
	async function shareForget() {
		const n = sh.share?.pending ?? 0;
		if (n && !confirm(t('shell.share.forgetConfirm', { n }))) return;
		try {
			await DocumentService.ShareForget();
			await sh.closeDoc(false);
		} catch (e) {
			sh.notice = sh.err(e);
		}
	}

	/** « Détacher du serveur » (propriétaire) : d'abord expliquer, puis faire. */
	function askDetach() {
		if (sh.share?.shared && sh.share.canManage) sh.detachAsk = detachQuestion(sh.share, !!sh.docState.remote);
	}

	async function shareDetach() {
		sh.detachAsk = null;
		try {
			await sh.opened(await DocumentService.ShareDetach());
		} catch (e) {
			sh.notice = sh.err(e);
		}
	}

	/** « Retirer du serveur… » (propriétaire) : supprime le classeur du serveur pour tous, après confirmation ; le sien reste ouvert en classeur local. */
	function askRemove() {
		if (sh.share?.shared && sh.share.canManage && !sh.share.gone) sh.removeAsk = removeQuestion(sh.share, !!sh.docState.remote, isWeb);
	}

	async function shareRemove() {
		sh.removing = true;
		try {
			const r = await DocumentService.ShareRemove();
			sh.removeAsk = null;
			await sh.opened(r);
		} catch (e) {
			sh.removeAsk = null;
			sh.notice = sh.err(e);
		} finally {
			sh.removing = false;
		}
	}

	/** Bandeau « Nouvelle version disponible » : recharge la version publiée (ajouts locaux gardés). */
	async function reloadShare() {
		sh.shareReloading = true;
		try {
			sh.docState = await DocumentService.ShareReload();
			await sh.reload();
			await sh.keepSheet();
			sh.notice = t('shell.share.versionLoaded', { version: sh.share?.version ?? '' });
		} catch (e) {
			sh.notice = sh.err(e);
		} finally {
			sh.shareReloading = false;
		}
	}

	/** Classeur ouvert depuis un serveur (cache local). */
	async function openedRemote(r: { state: DocState }) {
		sh.docState = r.state;
		sh.view = 'grid';
		sh.sheetId = 1;
		sh.mode = 'doc';
		sh.workbook++;
		sh.queryHost?.closePanel();
		await sh.reload();
		await sh.keepSheet();
		if (r.state.notice) sh.notice = r.state.notice;
		sh.workbookReady();
		await sh.loadRecent();
	}

	return {
		loadConflicts,
		addToShared,
		resolveConflict,
		resolveHeld,
		resolveAllConflicts,
		gotoConflict,
		gotoRow,
		followReference,
		refGoBack,
		loadSources,
		pushSource,
		retryShare,
		shareLocalCopy,
		shareForget,
		askDetach,
		shareDetach,
		askRemove,
		shareRemove,
		reloadShare,
		openedRemote
	};
}
