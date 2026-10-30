// Menu Fichier : ouvrir, récents, enregistrer (et la question du format
// .xlsx), fermer, quitter, démonstrations, export SQL, récupération. Sorti de routes/+page.svelte
// (docs/decoupage-sheet.md).
import { reportIssue } from '#lib/shell/report.js';
import { DocumentService, GridService } from '#bindings/internal/bridge/index.js';
import type { DamagedFile, DocState, FileQuestion, OpenResult, RecentFile } from '#bindings/internal/bridge/models.js';
import { recentDemo, recentLabel, recentRemote, savedNotice } from '#lib/shell/doctitle.js';
import { rememberedMode, XLSX_SAVE_SETTING, type XLSXQuestion } from '#lib/shell/xlsxsave.js';
import type { Layout } from '#bindings/internal/canvas/models.js';
import type { Orphan } from '#bindings/internal/archive/models.js';
import Sheet from '#lib/grid/Sheet.svelte';
import { type SheetStatus } from '#lib/grid/sheet/model.js';
import { type FileAction, type View } from '#lib/shell/Ribbon.svelte';
import { layoutShared as layoutSharedReason } from '#lib/shell/ribbon.js';
import { type OfferMoment } from '#lib/shell/mergesheets.js';
import type { MergePlan, SheetGroup } from '#bindings/internal/canvas/models.js';
import { printSheetPages } from '#lib/print/printsheet.js';
import { demoProgressText } from '#lib/shell/welcome.js';
import { isWeb } from '#lib/web/platform.js';
import type DocPage from '#lib/doc/DocPage.svelte';
import QueryHost from '#lib/query/QueryHost.svelte';
import { type SqlDialect, type SqlScope } from '#lib/shell/SqlExportDialog.svelte';
import { exportNotice, type ExportReport } from '#lib/shell/exportReport.js';
import type { State as CoEditState } from '#bindings/internal/coedit/models.js';
import { goneError, serverDown, serverLabel } from '#lib/remote/share.js';
import { lattis, loadLattis, requestLogin, requestLoginOrLocal } from '#lib/remote/lattis.svelte.js';
import { hostOf } from '#lib/remote/lattis.js';
import type { State as ShareState } from '#bindings/internal/remote/workbook/models.js';
import { type ColumnType } from '#lib/grid/format.js';
import { openHelp } from '#lib/help/state.svelte.js';
import { t } from '#lib/i18n/index.js';
import type { Mode } from '#lib/app/model.js';

/** Ce que createFiles lit et modifie dans +page.svelte. */
export interface FilesHost {
	linkAsk: { name: string; host: string; ok: (yes: boolean) => void } | null;
	readonly openedRemote: (r: { state: DocState; }) => Promise<void>;
	notice: string;
	readonly err: (e: unknown) => string;
	readonly loadRecent: () => Promise<RecentFile[]>;
	locked: FileQuestion | null;
	joining: boolean;
	mode: Mode;
	lockedCoEdit: boolean;
	orphans: Orphan[];
	recoverError: string;
	damaged: DamagedFile | null;
	view: View;
	sheetId: number;
	workbook: number;
	readonly queryHost: ReturnType<typeof QueryHost> | undefined;
	mergePlan: MergePlan | null;
	mergeOffer: SheetGroup | null;
	readonly reload: () => Promise<void>;
	readonly offerMerge: (moment: OfferMoment, sheet?: number) => void;
	conflict: (FileQuestion & { close: boolean }) | null;
	quitting: boolean;
	docState: DocState;
	readonly saving: (run: () => Promise<DocState>) => Promise<DocState>;
	xlsxAsk: { q: XLSXQuestion; close: boolean; remember: boolean } | null;
	closing: boolean;
	share: ShareState | null;
	unsentClose: number;
	readonly sheet: ReturnType<typeof Sheet> | undefined;
	layout: Layout | null;
	coedit: CoEditState | null;
	sheetStatus: SheetStatus | null;
	demoProgress: string;
	demo: { name: string; rows: number; columns: { name: string; type: ColumnType }[]; sqlName: string; sqlColumns: string[] } | null;
	failure: string;
	aboutOpen: boolean;
	openRemote: boolean;
	readonly canRights: boolean;
	rightsOpen: boolean;
	shareDlg: { republish: boolean } | null;
	readonly retryShare: () => Promise<void>;
	readonly askDetach: () => void;
	readonly askRemove: () => void;
	versionsOpen: boolean;
	readonly layoutShared: boolean;
	exported: { notice: string; report: ExportReport; format: string } | null;
	sqlExport: { table?: string } | null;
	readonly activeDoc: boolean;
	readonly docPage: ReturnType<typeof DocPage> | undefined;
	readonly exportDocPdf: () => void;
	showBreaks: boolean;
	settingsOpen: boolean;
}

export function createFiles(h: FilesHost) {
	async function openLink(raw: string) {
		const t = await DocumentService.ParseLink(raw);
		await loadLattis();
		if (!t.known) {
			const d = await DocumentService.LattisDiscover(t.server); // un vrai Serveur Lattis ?
			const yes = await new Promise<boolean>((ok) => (h.linkAsk = { name: d.name || hostOf(t.server), host: hostOf(t.server), ok }));
			h.linkAsk = null;
			if (!yes) return;
			await loadLattis();
		}
		if (!(lattis.state.servers ?? []).find((s) => s.url === t.server)?.user && !(await requestLogin(t.server))) return;
		await h.openedRemote(await DocumentService.ShareOpen(t.server, t.workbook, ''));
	}

	async function openPath(path: string) {
		await opened(await DocumentService.Open(path));
	}

	/**
	 * Ouvre un récent : un classeur du serveur se rouvre depuis le serveur
	 * (connexion d'abord si besoin) ; serveur injoignable, sa copie locale
	 * s'ouvre (hors ligne, lecture seule).
	 */
	async function openRecent(r: RecentFile) {
		if (recentDemo(r)) return openDemoPreset(r);
		if (!recentRemote(r)) return openPath(r.path);
		const server = r.server ?? '';
		const where = serverLabel({ server, serverName: r.serverName ?? '' });
		// Copie locale, sans le serveur (injoignable, connexion refusée ou
		// abandonnée) : modifiable hors ligne, bandeau Réessayer / Détacher.
		const local = async (reason: string) => {
			await opened(await DocumentService.OpenLocalCopy(r.path));
			h.notice = t('shell.share.localCopyOpened', { reason });
		};
		await loadLattis();
		if (!(lattis.state.servers ?? []).find((s) => s.url === server)?.user) {
			const ok = r.path ? await requestLoginOrLocal(server) : await requestLogin(server);
			if (ok === 'local') return local(t('shell.share.notSignedIn', { server: where }));
			if (!ok) return;
		}
		let res;
		try {
			res = await DocumentService.ShareOpen(server, r.workbookId ?? '', r.title ?? '');
		} catch (e) {
			// Supprimé du serveur, ou plus d'accès (sans cache à ouvrir) : pas une coupure.
			if (goneError(h.err(e))) {
				h.notice = h.err(e);
				if (confirm(t('shell.share.removeRecent', { error: h.err(e) }))) {
					await DocumentService.RemoveRecent(r);
					await h.loadRecent();
				}
				return;
			}
			if (!r.path) throw e;
			// Détail technique dans la console.
			console.warn('serveur injoignable', h.err(e));
			return local(serverDown(where));
		}
		await h.openedRemote(res);
	}

	/** Classeur ouvert sur un autre poste : en lecture seule, ou quand même (readOnly faux). */
	async function openLocked(path: string | null, readOnly: boolean) {
		h.locked = null;
		if (path) await opened(await DocumentService.OpenLocked(path, readOnly));
	}

	/** Classeur ouvert sur d'autres postes : demande à travailler à plusieurs (les autres l'accueillent). */
	async function openTogether(path: string) {
		h.locked = null;
		h.joining = true;
		h.notice = t('shell.coedit.requestSent');
		try {
			const r = await DocumentService.OpenTogether(path);
			if (r.state?.open) h.notice = '';
			await opened(r);
		} finally {
			h.joining = false;
		}
	}

	async function opened(r: OpenResult) {
		// Fichier de l'auteur d'un classeur partagé en direct : ouvert depuis le
		// serveur (en ligne, synchronisé), pas le fichier qui ne l'est pas.
		if (r.shared) {
			await openRecent(r.shared);
			if (h.mode === 'doc' && !h.notice) h.notice = t('shell.share.openedOnline', { title: r.shared.title ?? '', server: serverLabel({ server: r.shared.server ?? '', serverName: r.shared.serverName ?? '' }) });
			return;
		}
		if (r.locked) {
			h.locked = r.locked;
			h.lockedCoEdit = !!r.coEdit;
		} else if (r.recoverable) {
			h.orphans = [r.recoverable];
			h.recoverError = '';
			h.mode = 'recover';
		} else if (r.damaged) {
			h.damaged = r.damaged;
		} else if (!r.cancelled) {
			h.view = 'grid';
			h.sheetId = 1;
			h.mode = 'doc';
			h.workbook++;
			h.queryHost?.closePanel(); // volet du classeur quitté
			h.mergePlan = h.mergeOffer = null;
			await h.reload();
			// Déclencheurs inconnus supprimés (sécurité), lecture seule, verrou impossible.
			if (r.state.notice) h.notice = r.state.notice;
			workbookReady();
			h.offerMerge('open');
		}
		await h.loadRecent();
	}

	/** Réponse à « modifié par un autre poste » : écraser, enregistrer une copie (overwrite faux) ou rien (null). */
	async function resolveSave(overwrite: boolean | null) {
		const c = h.conflict;
		h.conflict = null;
		if (!c || overwrite === null) {
			h.quitting = false;
			return;
		}
		h.docState = await h.saving(() => DocumentService.ResolveSave(overwrite));
		if (h.docState.conflict) {
			h.conflict = { ...h.docState.conflict, close: c.close };
			return;
		}
		if (h.docState.dirty) {
			h.quitting = false; // copie annulée
			return;
		}
		h.notice = savedNotice(h.docState);
		await h.loadRecent();
		if (c.close) await closeDoc(false);
	}

	/** Fichier .xlsx ouvert : garder le format .xlsx (keep), convertir en .lattis, ou rien (null). */
	async function answerXlsx(keep: boolean | null) {
		const a = h.xlsxAsk;
		h.xlsxAsk = null;
		if (!a || keep === null) {
			h.quitting = false;
			return;
		}
		const mode = rememberedMode(keep, a.remember);
		if (mode) await DocumentService.SetSetting(XLSX_SAVE_SETTING, mode).catch((e) => (h.notice = h.err(e)));
		h.docState = await h.saving(() => DocumentService.SaveXLSX(keep));
		if (h.docState.conflict) {
			h.conflict = { ...h.docState.conflict, close: a.close };
			return;
		}
		if (h.docState.dirty) {
			h.quitting = false; // conversion annulée
			if (h.docState.notice) h.notice = h.docState.notice;
			return;
		}
		h.notice = savedNotice(h.docState, isWeb);
		await h.loadRecent();
		if (a.close) await closeDoc(false);
	}

	/** Classeur ouvert et affiché : ses écouteurs « open » (scripts Lua) s'exécutent, sans être attendus. */
	function workbookReady() {
		DocumentService.WorkbookReady().catch((e) => (h.notice = h.err(e)));
	}

	/** Récupère les données lisibles d'un classeur endommagé, dans un nouveau classeur. */
	async function salvage(path: string | null) {
		h.damaged = null;
		// Écran de récupération vidé (session abandonnée, puis fichier endommagé) : un classeur neuf.
		const fallback = () => (h.mode === 'recover' && h.orphans.length === 0 ? startDoc() : undefined);
		if (!path) return fallback();
		let r;
		try {
			r = await DocumentService.Salvage(path);
		} catch (e) {
			h.notice = h.err(e);
			return fallback();
		}
		h.view = 'grid';
		h.sheetId = 1;
		h.mode = 'doc';
		h.workbook++;
		await h.reload();
		h.notice = r.report;
	}

	/** Ferme le classeur ; modifié, il demande d'abord s'il faut l'enregistrer. */
	async function closeDoc(save?: boolean) {
		h.closing = false;
		// Écouteurs « before_close » des scripts : ils peuvent garder le classeur ouvert.
		if (h.mode === 'doc' && save === undefined && !(await DocumentService.BeforeClose())) {
			h.quitting = false;
			return;
		}
		if (h.mode === 'doc' && h.docState.dirty && save === undefined) {
			h.closing = true;
			return;
		}
		// Classeur collaboratif du serveur : rien à enregistrer (ses
		// modifications partent d'elles-mêmes, la file part à la fermeture) ;
		// hors ligne, prévenir de celles qui attendront la reconnexion.
		if (h.mode === 'doc' && save === undefined && h.docState.remote?.mode === 'collaborative' && !h.share?.online && (h.share?.pending ?? 0) > 0) {
			h.unsentClose = h.share?.pending ?? 0;
			return;
		}
		h.unsentClose = 0;
		if (save) {
			h.docState = await h.saving(() => DocumentService.Save(false));
			if (h.docState.conflict) {
				// Fichier modifié par un autre poste : la question d'abord, puis la fermeture.
				h.conflict = { ...h.docState.conflict, close: true };
				return;
			}
			if (h.docState.xlsx) {
				// Fichier .xlsx : garder le format ou convertir, puis la fermeture.
				h.xlsxAsk = { q: h.docState.xlsx, close: true, remember: false };
				return;
			}
			if (h.docState.dirty) {
				// Enregistrement abandonné (ou refusé par un script : before_save).
				if (h.docState.notice) h.notice = h.docState.notice;
				h.quitting = false;
				return;
			}
		}
		h.sheet?.closeOverlays();
		// Fermé dans tous les cas ; une erreur dit que la file d'envoi d'un
		// classeur partagé n'a pas pu être gardée (récupération au lancement).
		const kept = await DocumentService.CloseDocument().then(
			() => '',
			(e) => h.err(e)
		);
		h.docState = { open: false, path: '', title: '', dirty: false, undo: '', redo: '' };
		h.layout = null;
		h.share = null;
		h.coedit = null;
		h.sheetStatus = null;
		h.view = 'grid';
		h.mode = 'closed';
		h.workbook++;
		if (kept) h.notice = kept;
		if (h.quitting) {
			h.quitting = false;
			await DocumentService.QuitApp();
			return;
		}
		await h.loadRecent();
	}

	/** Croix de la fenêtre avec un classeur modifié : la question de Fichier › Fermer, puis Lattis se ferme. */
	async function requestQuit() {
		DocumentService.CloseRequestSeen().catch(() => {});
		if (h.quitting) return; // question déjà posée
		h.quitting = true;
		try {
			await closeDoc();
		} catch (e) {
			h.quitting = false;
			throw e;
		}
	}

	/** Réponse « Annuler » à la question d'enregistrement. */
	function cancelClose() {
		h.closing = false;
		h.unsentClose = 0;
		h.quitting = false;
	}

	async function startDoc() {
		h.mode = 'doc';
		h.view = 'grid';
		h.sheetId = 1;
		h.workbook++;
		h.queryHost?.closePanel();
		await DocumentService.New();
		await h.reload();
	}

	/** Fabrique une démonstration (open) en affichant son avancement, puis l'affiche. */
	async function buildDemo(label: string, open: () => Promise<{ sheet: number }>) {
		let waiting = true;
		const poll = async () => {
			if (!waiting) return;
			const p = await DocumentService.DemoProgress().catch(() => null);
			if (p && waiting) h.demoProgress = demoProgressText(label, p);
			setTimeout(poll, 250);
		};
		poll();
		try {
			const r = await open();
			h.view = 'grid';
			h.sheetId = r.sheet;
			h.mode = 'doc';
			h.workbook++;
			await h.reload();
		} finally {
			waiting = false;
			h.demoProgress = '';
		}
	}

	/** Ouvre le classeur de démonstration demandé au lancement (--demo preset). */
	async function startPresetDemo(preset: string) {
		await buildDemo(t('shell.demo.preset', { name: preset }), () => DocumentService.OpenDemo());
	}

	/** Démo publique : classeur de démonstration des récents (Démo small, Démo medium), à la place du classeur ouvert. */
	async function openDemoPreset(r: RecentFile) {
		h.queryHost?.closePanel();
		await buildDemo(recentLabel(r), () => DocumentService.OpenPreset(recentDemo(r)));
		workbookReady();
	}

	/** Classeur d'exemple de l'accueil : la vitrine, jamais enregistrée d'elle-même. */
	async function openSample() {
		h.mode = 'starting';
		h.queryHost?.closePanel();
		try {
			await buildDemo(t('shell.demo.sample'), () => DocumentService.OpenSample());
			workbookReady();
		} catch (e) {
			h.mode = 'closed';
			throw e;
		}
	}

	async function startDemo() {
		h.mode = 'demo';
		h.workbook++;
		const poll = async () => {
			const s = await GridService.Status();
			if (s.state === 'ready') {
				h.demo = {
					name: s.info.name,
					rows: s.info.rows,
					columns: (s.info.columns ?? []).map((c) => ({ name: c.name, type: c.type as ColumnType })),
					sqlName: s.info.sqlName,
					sqlColumns: s.info.sqlColumns ?? []
				};
			} else if (s.state === 'error') {
				h.failure = s.error ?? t('shell.demo.error');
			} else {
				h.demoProgress = s.state === 'generating' ? t('shell.demo.generating', { done: s.done, total: s.total }) : t('shell.demo.opening');
				setTimeout(poll, 250);
			}
		};
		poll();
	}

	async function file(a: FileAction) {
		if (a === 'help') return openHelp();
		if (a === 'report') return reportIssue();
		if (a === 'about') {
			h.aboutOpen = true;
			return;
		}
		h.notice = '';
		try {
			switch (a) {
				case 'new':
					await startDoc();
					break;
				case 'open':
					await openPath('');
					break;
				case 'openRemote':
					h.openRemote = true;
					break;
				case 'rights':
					if (h.canRights) h.rightsOpen = true;
					break;
				case 'share':
				case 'republish':
					if (h.mode !== 'doc') return;
					h.shareDlg = { republish: a === 'republish' };
					break;
				case 'reconnect':
					if (h.mode === 'doc') await h.retryShare();
					break;
				case 'detach':
					if (h.mode === 'doc') h.askDetach();
					break;
				case 'remove':
					if (h.mode === 'doc') h.askRemove();
					break;
				case 'close':
					await closeDoc();
					break;
				case 'versions':
					if (h.mode === 'doc') h.versionsOpen = true;
					break;
				case 'save':
				case 'saveAs':
					if (h.mode !== 'doc') return;
					h.docState = await h.saving(() => DocumentService.Save(a === 'saveAs'));
					if (h.docState.xlsx) h.xlsxAsk = { q: h.docState.xlsx, close: false, remember: false };
					else if (h.docState.conflict) h.conflict = { ...h.docState.conflict, close: false };
					else if (h.docState.notice || !h.docState.dirty) h.notice = savedNotice(h.docState, isWeb);
					await h.loadRecent();
					break;
				case 'download':
					if (h.mode !== 'doc') return;
					h.docState = await h.saving(() => DocumentService.Download());
					if (h.docState.notice) h.notice = h.docState.notice;
					break;
				case 'import': {
					// L'import crée une feuille : refusé d'emblée là où le moteur le refuserait.
					if (h.layoutShared) {
						h.notice = t('shell.notice.importCreatesSheet', { reason: layoutSharedReason() });
						break;
					}
					// Depuis l'accueil : un classeur neuf, refermé si l'import est annulé.
					const started = h.mode !== 'doc';
					if (started) await startDoc();
					const r = await DocumentService.Import('');
					if (r.cancelled) {
						if (started) await closeDoc();
						break;
					}
					if (r.sheet) h.sheetId = r.sheet;
					await h.reload();
					const tables = r.tables ?? [];
					const what = tables.length === 1 ? t('shell.notice.importedOne', { table: tables[0] }) : t('shell.notice.importedMany', { n: tables.length });
					const warnings = r.warnings ?? [];
					h.notice = warnings.length ? t('shell.notice.importWarnings', { what, n: warnings.length, list: warnings.join(' · ') }) : what;
					// L'import garde les onglets tels quels ; la détection tourne ensuite.
					h.offerMerge('import');
					break;
				}
				case 'export': {
					if (h.mode !== 'doc') return;
					const table = h.sheet?.activeTable() ?? h.layout?.tables?.[0]?.name;
					if (!table) {
						h.notice = t('shell.notice.noTableToExport');
						break;
					}
					const p = await DocumentService.ExportCSV(table);
					if (p) h.notice = t('shell.notice.tableExported', { table, path: p });
					break;
				}
				case 'exportSheet': {
					// Toute la feuille active, de A1 à la dernière cellule utilisée.
					if (h.mode !== 'doc') return;
					const name = h.layout?.sheets?.find((s) => s.id === h.sheetId)?.name ?? t('shell.tabs.menu.sheet');
					const p = await DocumentService.ExportSheetCSV(h.sheetId, name);
					if (p) h.notice = t('shell.notice.sheetExported', { name, path: p });
					break;
				}
				case 'exportXlsx':
				case 'exportOds': {
					// Formules traduites quand le format a un équivalent, sinon leur valeur.
					if (h.mode !== 'doc') return;
					const r = a === 'exportOds' ? await DocumentService.ExportODS() : await DocumentService.ExportXLSX();
					if (r.path) {
						h.notice = exportNotice(r.path, r.report);
						h.exported = { notice: h.notice, report: r.report, format: a === 'exportOds' ? 'OpenDocument' : 'Excel' };
					}
					break;
				}
				case 'exportSql':
					if (h.mode !== 'doc') return;
					h.sqlExport = { table: h.sheet?.activeTable() ?? undefined };
					return;
				case 'print': {
					// Feuille active selon sa mise en page ; le PDF passe par la
					// fenêtre d'impression (imprimante PDF du système).
					if (h.mode !== 'doc') return;
					if (h.activeDoc && h.docPage) {
						h.exportDocPdf();
						break;
					}
					const name = h.layout?.sheets?.find((s) => s.id === h.sheetId)?.name ?? '';
					if (!h.sheet) return;
					// Pages écrites par lots, avancement dans la barre d'état.
					await printSheetPages(h.sheetId, h.sheet, `${h.docState.title} – ${name}`);
					// Comme Excel : après une impression, la grille montre les sauts de page.
					h.showBreaks = true;
					break;
				}
				case 'demo':
					await startDemo();
					break;
				case 'settings':
					h.settingsOpen = true;
					break;
			}
		} catch (e) {
			h.notice = h.err(e);
		}
		h.sheet?.focus();
	}

	/** Écrit le script SQL choisi dans la boîte (avancement dans la barre d'état). */
	async function exportSql(dialect: SqlDialect, scope: SqlScope) {
		const table = h.sqlExport?.table;
		h.sqlExport = null;
		try {
			const r = await DocumentService.ExportSQL({ dialect, sheet: scope === 'sheet' ? h.sheetId : 0, table: scope === 'table' ? (table ?? '') : '', source: '' });
			if (r.path) h.notice = t('shell.notice.sqlExported', { n: r.report.tables, path: r.path });
		} catch (e) {
			h.notice = h.err(e);
		}
		h.sheet?.focus();
	}

	/** Récupère (keep) ou abandonne une session ; also : versions plus anciennes du même classeur, abandonnées avec elle. */
	async function recover(o: Orphan, keep: boolean, also: Orphan[] = []) {
		h.recoverError = '';
		const gone = new Set<string>();
		let st: DocState | null = null;
		try {
			if (keep) st = await DocumentService.Recover(o.dir);
			else
				for (const x of [o, ...also]) {
					await DocumentService.Discard(x.dir);
					gone.add(x.dir);
				}
		} catch (e) {
			// La session reste proposée : rien n'est perdu, l'utilisateur choisit.
			h.recoverError = h.err(e);
			if (gone.size) h.orphans = h.orphans.filter((x) => !gone.has(x.dir));
			return;
		}
		try {
			if (keep) {
				h.mode = 'doc';
				h.workbook++;
				await h.reload();
				// Fichier verrouillé ou modifié par un autre poste depuis la session : dit aussi.
				h.notice = [t('shell.recovery.recovered'), st?.notice].filter(Boolean).join(' ');
				workbookReady();
			} else {
				h.orphans = h.orphans.filter((x) => !gone.has(x.dir));
				if (h.orphans.length > 0) return;
				// Session d'un fichier abandonnée : le fichier tel qu'enregistré.
				if (o.source) await openPath(o.source).catch(() => startDoc());
				else await startDoc();
			}
		} catch (e) {
			h.failure = h.err(e);
		}
	}

	return {
		openLink,
		openPath,
		openRecent,
		openLocked,
		openTogether,
		opened,
		resolveSave,
		answerXlsx,
		workbookReady,
		salvage,
		closeDoc,
		requestQuit,
		cancelClose,
		startDoc,
		startPresetDemo,
		openDemoPreset,
		openSample,
		startDemo,
		file,
		exportSql,
		recover
	};
}
