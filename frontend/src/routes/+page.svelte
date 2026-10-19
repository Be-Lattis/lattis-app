<script lang="ts">
	// Classeur façon Excel : ruban en haut, feuille (ou onglet SQL / Script) au
	// centre, onglets de feuilles et barre d'état en bas.
	//
	// Deux modes : un classeur (DocumentService : saisie libre, tableaux qui
	// naissent, annuler, enregistrer) et la démo d'1 M de lignes (GridService),
	// qui sert aussi à la mesure automatique (LATTIS_BENCH).
	import { describe, installErrorReporting, reportError } from '#lib/shell/errors.js';
	import { onMount, tick, untrack } from 'svelte';
	import { isProblem, noticeTtl } from '#lib/shell/notice.js';
	import { reportIssue } from '#lib/shell/report.js';
	import { Events } from '@wailsio/runtime';
	import { DocumentService, GridService } from '#bindings/internal/bridge/index.js';
	import type { DamagedFile, DocState, FileQuestion, FormulaHelp, OpenResult } from '#bindings/internal/bridge/models.js';
	import type { Layout, RuleChanges } from '#bindings/internal/canvas/models.js';
	import type { Type } from '#bindings/internal/grid/models.js';
	import type { Orphan } from '#bindings/internal/archive/models.js';
	import Sheet, { type SheetData, type SheetSource, type SheetStatus, type ColumnFilter, type PivotConfig, type ChartConfig, type TypeMix } from '#lib/grid/Sheet.svelte';
	import Ribbon, { type DocCommand, type FileAction, type View } from '#lib/shell/Ribbon.svelte';
	import SheetTabs from '#lib/shell/SheetTabs.svelte';
	import { visibleSheet } from '#lib/shell/sheets.js';
	import { choiceStatus, choiceStep, type ChoiceActions } from '#lib/shell/choice.js';
	import { count } from '#lib/sql/result.js';
	import { printSheet } from '#lib/shell/print.js';
	import { withSaveProgress } from '#lib/shell/saveprogress.js';
	import { demoProgressText, firstSteps, hideSteps, stepsShown } from '#lib/shell/welcome.js';
	import PageSetupDialog from '#lib/shell/PageSetupDialog.svelte';
	import { pageAction, type PageAction, type PageSetup, type PageSetupTab } from '#lib/shell/pagesetup.js';
	import type { PageBreaks } from '#bindings/internal/transfer/models.js';
	import SqlView, { type Favorite } from '#lib/shell/SqlView.svelte';
	import AiPanel from '#lib/ai/AiPanel.svelte';
	import { chat as aiChat, openAssistant } from '#lib/ai/state.svelte.js';
	import McpConfirm from '#lib/shell/McpConfirm.svelte';
	import TitleBar from '#lib/shell/TitleBar.svelte';
	import ScriptView, { type Script } from '#lib/script/ScriptView.svelte';
	import ScriptDialog, { type DialogSpec } from '#lib/script/ScriptDialog.svelte';
	import PickBanner from '#lib/script/PickBanner.svelte';
	import type { ControlInfo } from '#lib/script/controls.js';
	import McpPanel from '#lib/shell/McpPanel.svelte';
	import SettingsDialog from '#lib/shell/SettingsDialog.svelte';
	import { htmlTabsEnabled, loadSettings, mcpAvailable, sharedWorkbooksEnabled } from '#lib/shell/settings.svelte.js';
	import { listenExternal } from '#lib/ai/external.svelte.js';
	import { isWeb } from '#lib/web/platform.js';
	import HtmlPage from '#lib/page/HtmlPage.svelte';
	import DocPage from '#lib/doc/DocPage.svelte';
	import type { DocFormat } from '#lib/doc/format.js';
	import QueryHost from '#lib/query/QueryHost.svelte';
	import LattisHost from '#lib/remote/LattisHost.svelte';
	import ShareDialog from '#lib/remote/ShareDialog.svelte';
	import OpenRemoteDialog from '#lib/remote/OpenRemoteDialog.svelte';
	import DataCatalogDialog from '#lib/remote/DataCatalogDialog.svelte';
	import DataShareDialog from '#lib/remote/DataShareDialog.svelte';
	import AboutDialog from '#lib/shell/AboutDialog.svelte';
	import ExportReportDialog from '#lib/shell/ExportReportDialog.svelte';
	import { exportNotice, printNotice, type ExportReport } from '#lib/shell/exportReport.js';
	import ShareBanner from '#lib/remote/ShareBanner.svelte';
	import ConflictPanel from '#lib/remote/ConflictPanel.svelte';
	import RightsDialog from '#lib/remote/RightsDialog.svelte';
	import { rightsObjects, type RightsObject } from '#lib/remote/rights.js';
	import { refusedNotice } from '#lib/remote/share.js';
	import { pushSummary, type SourceLink } from '#lib/remote/source.js';
	import type { Conflict } from '#bindings/internal/remote/collab/models.js';
	import { canRepublish, rolesLabel } from '#lib/remote/share.js';
	import { lattis, loadLattis, requestLogin, shareRights } from '#lib/remote/lattis.svelte.js';
	import { connected, hostOf } from '#lib/remote/lattis.js';
	import { dataGroup, type DataCatalog, type DataGroup } from '#lib/remote/data.js';
	import type { State as ShareState } from '#bindings/internal/remote/workbook/models.js';
	import type { ButtonConfig } from '#lib/grid/buttons.js';
	import type { UICommand } from '#bindings/internal/bridge/models.js';
	import { typeLabel, type ColumnType } from '#lib/grid/format.js';
	import type { Fetched } from '#lib/grid/cache.js';
	import { isReloadKey, shortcutOf } from '#lib/grid/shortcuts.js';
	import { cycleRegions, modal, modalOpen } from '#lib/ui/focus.js';
	import { clampZoom } from '#lib/grid/zoom.js';
	import ZoomBar from '#lib/shell/ZoomBar.svelte';
	import BusyBar from '#lib/shell/BusyBar.svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { groupSessions, olderLabel, sessionName } from '#lib/shell/recovery.js';
	import { setTileFetcher } from '#lib/chart/tiles.js';
	import HelpPanel from '#lib/help/HelpPanel.svelte';
	import { functionAt, viewTarget } from '#lib/help/help.js';
	import { helpState, openHelp } from '#lib/help/state.svelte.js';

	type Mode = 'starting' | 'recover' | 'doc' | 'demo' | 'closed';

	let mode = $state<Mode>('starting');
	let bench = $state(false);
	let view = $state<View>('grid');
	let failure = $state('');
	let notice = $state('');
	// Message de la barre d'état : effacé seul au bout d'un moment (noticeTtl),
	// et à chaque changement d'onglet.
	let noticeAt = 0;
	$effect(() => {
		const m = notice;
		if (!m) return;
		noticeAt = Date.now();
		const t = setTimeout(() => {
			if (notice === m) notice = '';
		}, noticeTtl(m));
		return () => clearTimeout(t);
	});
	$effect(() => {
		void sheetId;
		void view;
		untrack(() => {
			// Un message donné juste avant d'ouvrir l'onglet (« Résultat copié
			// dans… ») reste ; les anciens partent.
			if (Date.now() - noticeAt > 1500) notice = '';
			sheet?.clearMessage();
		});
	});
	let sheetStatus = $state<SheetStatus | null>(null);
	let sheet = $state<ReturnType<typeof Sheet>>();

	// Classeur.
	let docState = $state<DocState>({ open: false, path: '', title: '', dirty: false, undo: '', redo: '' });
	let layout = $state<Layout | null>(null);
	let sheetId = $state(1);
	// Défilement horizontal de la grille, affiché dans la ligne des onglets.
	let hscroll = $state<{ content: number; view: number; left: number } | undefined>();
	// Les adresses Cell_A1 de l'onglet SQL se lisent sur la feuille affichée.
	$effect(() => {
		if (mode === 'doc') DocumentService.SetActiveSheet(sheetId).catch(() => {});
	});
	let orphans = $state<Orphan[]>([]);
	// Écran de récupération : une entrée par classeur (son identifiant, pas
	// son nom), la session la plus récente ; les 3 classeurs les plus récents
	// d'abord, les autres sur demande.
	const ORPHANS_SHOWN = 3;
	let allOrphans = $state(false);
	const orphanGroups = $derived(groupSessions(orphans));
	// Versions plus anciennes dépliées, par classeur.
	let olderShown = $state<Record<string, boolean>>({});
	let help = $state<FormulaHelp | null>(null);
	let scripts = $state<Script[]>([]);
	// Contenu de l'onglet page HTML ou Document affiché (null : une feuille).
	let pageSource = $state<{ sheet: number; source: string } | null>(null);
	// Document affiché : son éditeur, et la mise en forme à la sélection (ruban).
	let docPage = $state<ReturnType<typeof DocPage>>();
	let docFmt = $state<DocFormat | null>(null);
	// Requêtes favorites de l'onglet SQL, enregistrées dans le classeur.
	let favorites = $state<Favorite[]>([]);
	let recent = $state<string[]>([]);
	// Fermer un classeur modifié : enregistrer d'abord ?
	let closing = $state(false);
	// Fermeture de la fenêtre en cours : une fois le classeur fermé, on quitte.
	let quitting = $state(false);
	// Fichier endommagé à l'ouverture : proposer d'en récupérer les données.
	let damaged = $state<DamagedFile | null>(null);
	// Classeur ouvert sur un autre poste (verrou ~$) : lecture seule, ou l'ouvrir quand même.
	let locked = $state<FileQuestion | null>(null);
	// Fichier modifié par un autre poste depuis l'ouverture : copie, écraser ou annuler (close : puis fermer).
	let conflict = $state<(FileQuestion & { close: boolean }) | null>(null);
	// Échec d'une récupération de session : l'écran de récupération reste.
	let recoverError = $state('');
	// Boîtes de dialogue ouvertes par un script ou un assistant (ui.dialog…), la plus ancienne d'abord.
	let dialogs = $state<DialogSpec[]>([]);
	let mcpOpen = $state(false);
	let settingsOpen = $state(false);
	// Onglet Mise en page : boîte ouverte (onglet), sauts de page affichés dans la grille.
	let pageDlg = $state<PageSetupTab | null>(null);
	let showBreaks = $state(false);
	let breaks = $state<PageBreaks | null>(null);
	let aboutOpen = $state(false);
	// Dernier export .xlsx / .ods : son compte rendu (lien « Détails » tant
	// que son message est affiché) et la boîte de détails.
	let exported = $state<{ notice: string; report: ExportReport; format: string } | null>(null);
	let exportOpen = $state(false);
	// Classeur partagé sur un Serveur Lattis : état (bandeau, Fichier ›
	// Republier), boîtes Partager / Republier et Ouvrir depuis un serveur.
	let share = $state<ShareState | null>(null);
	// Classeur collaboratif : volet Conflits (ouvert de lui-même quand un
	// conflit apparaît).
	let conflictsOpen = $state(false);
	let conflicts = $state<Conflict[]>([]);
	let conflictBusy = $state(false);
	let lastConflicts = 0;
	$effect(() => {
		const n = share?.conflicts ?? 0;
		if (n > lastConflicts) conflictsOpen = true;
		lastConflicts = n;
		if (conflictsOpen || n) loadConflicts();
	});

	async function loadConflicts() {
		try {
			conflicts = (await DocumentService.ShareConflicts()) ?? [];
		} catch (e) {
			notice = err(e);
		}
	}

	async function resolveConflict(c: Conflict, mine: boolean) {
		conflictBusy = true;
		try {
			docState = await DocumentService.ShareResolve(c.id, mine);
			await reload();
			await loadConflicts();
			notice = mine ? 'Votre valeur est renvoyée au serveur' : 'Valeur du collègue reprise';
			if (!conflicts.length) conflictsOpen = false;
		} catch (e) {
			notice = err(e);
		} finally {
			conflictBusy = false;
		}
	}

	/** Volet Conflits : garde toutes mes valeurs (mine) ou toutes celles des collègues. */
	async function resolveAllConflicts(mine: boolean) {
		conflictBusy = true;
		let n = 0;
		try {
			for (const c of conflicts) {
				docState = await DocumentService.ShareResolve(c.id, mine);
				n++;
			}
			notice = mine ? `${n} valeur(s) renvoyée(s) au serveur` : `${n} valeur(s) des collègues reprise(s)`;
		} catch (e) {
			notice = err(e);
		} finally {
			await reload().catch(() => {});
			await loadConflicts();
			if (!conflicts.length) conflictsOpen = false;
			conflictBusy = false;
		}
	}

	/** Volet Conflits : va sur la cellule en conflit. */
	async function gotoConflict(c: Conflict) {
		if (!c.found) return;
		view = 'grid';
		if (c.sheet !== sheetId) {
			sheet?.closeOverlays();
			sheetId = c.sheet;
			await reload();
		}
		await tick();
		await new Promise((r) => requestAnimationFrame(r));
		sheet?.selectRange(c.row, c.col, c.row, c.col);
		sheet?.focus();
	}
	/**
	 * Point de carte cliqué (feuille ou document) : va sur la feuille du
	 * tableau et sélectionne la ligne du point, retrouvée par son identifiant
	 * (le tri ou le filtre du moment sont suivis).
	 */
	async function gotoRow(table: string, id: number) {
		if (mode !== 'doc') return;
		try {
			const spot = await DocumentService.LocateRow(table, id);
			view = 'grid';
			if (spot.sheet !== sheetId) {
				sheet?.closeOverlays();
				sheetId = spot.sheet;
				await reload();
			}
			await tick();
			await new Promise((r) => requestAnimationFrame(r));
			const last = spot.col + Math.max(1, spot.cols) - 1;
			// Ligne masquée par le filtre : l'en-tête du tableau est sélectionné.
			sheet?.selectRange(spot.row, spot.col, spot.row, last);
			if (spot.hidden) notice = `Ligne masquée par le filtre de ${spot.table}`;
			sheet?.focus();
		} catch (e) {
			notice = err(e);
		}
	}
	let shareDlg = $state<{ republish: boolean } | null>(null);
	// Fichier › Gérer l’accès… : droits du classeur publié (propriétaire ou
	// concepteur), aussi depuis le menu Compte ; onglet SQL : règles et
	// rôles générés (lecture seule).
	let rightsOpen = $state(false);
	const canRights = $derived(mode === 'doc' && !!share?.shared && (!!share?.canManage || !!share?.canDesign));
	$effect(() => {
		shareRights.current = canRights ? { title: share?.title ?? '', open: () => (rightsOpen = true) } : null;
	});
	let rightsObjs = $state<RightsObject[]>([]);
	$effect(() => {
		const sh = share;
		if (view !== 'sql' || mode !== 'doc' || !sh?.shared) {
			rightsObjs = [];
			return;
		}
		DocumentService.ShareRights(!!sh.canManage && sh.online)
			.then((v) => (rightsObjs = rightsObjects(v)))
			.catch(() => (rightsObjs = []));
	});
	// Onglet SQL : vues donnees.* des données du Data Lake lisibles sur les
	// serveurs connectés (un groupe par serveur), interrogées sur le serveur.
	let dataGroups = $state<DataGroup[]>([]);
	$effect(() => {
		if (view !== 'sql' || mode !== 'doc') {
			dataGroups = [];
			return;
		}
		let stale = false;
		(async () => {
			await loadLattis();
			if (!lattis.state.enabled) return;
			const groups = await Promise.all(
				connected(lattis.state.servers).map((sv) =>
					DocumentService.DataCatalog(sv.url)
						.then((c) => dataGroup(sv.url, sv.name || hostOf(sv.url), c as unknown as DataCatalog))
						.catch(() => null)
				)
			);
			if (!stale) dataGroups = groups.filter((g): g is DataGroup => !!g && g.views.length > 0);
		})();
		return () => {
			stale = true;
		};
	});
	let openRemote = $state(false);
	// Données du Data Lake : insertion (cellule visée) et partage d'un tableau.
	let dataInsert = $state<{ sheet: number; row: number; col: number } | null>(null);
	let dataShare = $state<string | null>(null);
	// Publier le classeur depuis cette boîte : elle revient une fois publié.
	let dataShareAfter = $state<string | null>(null);
	// Tableaux publiés seuls comme source du Data Lake (sans le classeur) :
	// clic droit › « Mettre à jour la source du Data Lake ».
	let sourceTables = $state<string[]>([]);
	async function loadSources() {
		if (mode !== 'doc' || !sharedWorkbooksEnabled()) {
			sourceTables = [];
			return;
		}
		const links = ((await DocumentService.TableSources().catch(() => null)) ?? []) as unknown as SourceLink[];
		sourceTables = links.map((l) => l.tableName).filter((n) => !!n);
	}
	async function pushSource(table: string) {
		notice = `Mise à jour de la source « ${table} »…`;
		try {
			const l = (await DocumentService.PushTableSource(table)) as unknown as SourceLink;
			notice = pushSummary(table, l.lastResult, l.blanked ?? 0);
		} catch (e) {
			notice = `Source « ${table} » non mise à jour : ${err(e)}`;
		}
	}
	let shareReloading = $state(false);
	// Données › Obtenir des données : requêtes d'import (lib/query).
	let queryHost = $state<ReturnType<typeof QueryHost>>();
	// Révision › Protéger : boîte du mot de passe (protéger, ou ôter une
	// protection qui en a un).
	// Suppression d'une feuille qui contient quelque chose : confirmée avec la liste de ce qui part.
	let deleting = $state<{ id: number; name: string; items: string[] } | null>(null);
	// Plusieurs onglets sélectionnés à supprimer (une seule étape d'annulation).
	let deletingMany = $state<number[] | null>(null);
	let protecting = $state<{ what: 'sheet' | 'workbook'; id: number; name: string; off: boolean } | null>(null);
	let pw1 = $state('');
	let pw2 = $state('');
	let pwError = $state('');
	// Protéger le classeur : verrouiller aussi le contenu de toutes les feuilles.
	let lockSheets = $state(false);

	// Démo.
	let demo = $state<{ name: string; rows: number; columns: { name: string; type: ColumnType }[]; sqlName: string; sqlColumns: string[] } | null>(null);
	let demoProgress = $state('');
	// Accueil : premiers pas, jusqu'à « Ne plus afficher ».
	let showSteps = $state(stepsShown());

	const err = (e: unknown) => String(e instanceof Error ? e.message : e);

	async function reload() {
		docState = await DocumentService.State();
		layout = await DocumentService.Layout(sheetId);
		// Feuille affichée masquée ou disparue : la première feuille visible.
		const shownId = visibleSheet(layout?.sheets ?? [], sheetId);
		if (layout && shownId !== sheetId) {
			sheetId = shownId;
			layout = await DocumentService.Layout(sheetId);
		}
		// Onglet page HTML ou Document affiché : son contenu (la page se recharge s'il change).
		const shown = layout?.sheets?.find((s) => s.id === sheetId);
		if (shown?.kind === 'html' || shown?.kind === 'doc') {
			const src = await DocumentService.Page(sheetId);
			if (!pageSource || pageSource.sheet !== sheetId || pageSource.source !== src) pageSource = { sheet: sheetId, source: src };
		} else pageSource = null;
		help = await DocumentService.FormulaHelp();
		scripts = (await DocumentService.Scripts()) ?? [];
		favorites = (await DocumentService.Favorites()) ?? [];
		share = await DocumentService.ShareState().catch(() => null);
		await loadSources();
		// Volet Requêtes ouvert : ses listes suivent (Ctrl+Z, assistant, MCP, autre classeur).
		queryHost?.reloadLists();
	}

	/** Enregistre la mise en page de la feuille active (une étape d'annulation) ; les sauts de page s'affichent, comme dans Excel. */
	async function savePage(p: PageSetup, label: string) {
		try {
			await DocumentService.SetPageSetup(sheetId, p, label);
			showBreaks = true;
			await reload();
		} catch (e) {
			notice = err(e);
		}
	}

	/** Commande de l'onglet Mise en page. */
	function pageCmd(a: PageAction) {
		if (!layout) return;
		if (a.kind === 'dialog') {
			pageDlg = a.tab;
			return;
		}
		const c = sheet?.activeCell() ?? { row: 0, col: 0 };
		const sel = sheet?.selectedRange() ?? { r0: c.row, c0: c.col, r1: c.row, c1: c.col };
		const r = pageAction(layout.pageSetup, a, { ...c, sel });
		if (!r) return;
		if ('notice' in r) notice = r.notice;
		else void savePage(r.setup, r.label);
	}

	// Sauts de page de la feuille active, relus à chaque changement du classeur.
	$effect(() => {
		const l = layout;
		const id = sheetId;
		if (!showBreaks || mode !== 'doc' || !l) {
			breaks = null;
			return;
		}
		let live = true;
		DocumentService.PageBreaks(id)
			.then((b) => live && (breaks = b))
			.catch(() => live && (breaks = null));
		return () => {
			live = false;
		};
	});

	/** Bandeau « Nouvelle version disponible » : recharge la version publiée (ajouts locaux gardés). */
	async function reloadShare() {
		shareReloading = true;
		try {
			docState = await DocumentService.ShareReload();
			await reload();
			await keepSheet();
			notice = `Version ${share?.version ?? ''} chargée`;
		} catch (e) {
			notice = err(e);
		} finally {
			shareReloading = false;
		}
	}

	/** Classeur ouvert depuis un serveur (cache local). */
	async function openedRemote(r: { state: DocState }) {
		docState = r.state;
		view = 'grid';
		sheetId = 1;
		mode = 'doc';
		queryHost?.closePanel();
		await reload();
		await keepSheet();
		if (r.state.notice) notice = r.state.notice;
		workbookReady();
		await loadRecent();
	}

	// Lien lattis://open?server=…&workbook=… (navigateur, lien partagé) :
	// serveur jamais utilisé, confirmation d'abord ; connexion si besoin ; puis
	// le même chemin que « Ouvrir depuis un serveur ».
	let linkAsk = $state<{ name: string; host: string; ok: (yes: boolean) => void } | null>(null);
	async function openLink(raw: string) {
		const t = await DocumentService.ParseLink(raw);
		await loadLattis();
		if (!t.known) {
			const d = await DocumentService.LattisDiscover(t.server); // un vrai Serveur Lattis ?
			const yes = await new Promise<boolean>((ok) => (linkAsk = { name: d.name || hostOf(t.server), host: hostOf(t.server), ok }));
			linkAsk = null;
			if (!yes) return;
			await loadLattis();
		}
		if (!(lattis.state.servers ?? []).find((s) => s.url === t.server)?.user && !(await requestLogin(t.server))) return;
		await openedRemote(await DocumentService.ShareOpen(t.server, t.workbook, ''));
	}

	const loadRecent = async () => (recent = (await DocumentService.Recent().catch(() => null)) ?? []);

	async function openPath(path: string) {
		await opened(await DocumentService.Open(path));
	}

	/** Classeur ouvert sur un autre poste : en lecture seule, ou quand même (readOnly faux). */
	async function openLocked(path: string | null, readOnly: boolean) {
		locked = null;
		if (path) await opened(await DocumentService.OpenLocked(path, readOnly));
	}

	async function opened(r: OpenResult) {
		if (r.locked) {
			locked = r.locked;
		} else if (r.recoverable) {
			orphans = [r.recoverable];
			recoverError = '';
			mode = 'recover';
		} else if (r.damaged) {
			damaged = r.damaged;
		} else if (!r.cancelled) {
			view = 'grid';
			sheetId = 1;
			mode = 'doc';
			queryHost?.closePanel(); // volet du classeur quitté
			await reload();
			// Déclencheurs inconnus supprimés (sécurité), lecture seule, verrou impossible.
			if (r.state.notice) notice = r.state.notice;
			workbookReady();
		}
		await loadRecent();
	}

	/** Enregistre en montrant, pour un gros classeur, l'avancement dans la barre d'état. */
	function saving(run: () => Promise<DocState>): Promise<DocState> {
		return withSaveProgress(run, () => DocumentService.SaveProgress(), (t) => (notice = t));
	}

	/** Réponse à « modifié par un autre poste » : écraser, enregistrer une copie (overwrite faux) ou rien (null). */
	async function resolveSave(overwrite: boolean | null) {
		const c = conflict;
		conflict = null;
		if (!c || overwrite === null) {
			quitting = false;
			return;
		}
		docState = await saving(() => DocumentService.ResolveSave(overwrite));
		if (docState.conflict) {
			conflict = { ...docState.conflict, close: c.close };
			return;
		}
		if (docState.dirty) {
			quitting = false; // copie annulée
			return;
		}
		notice = `Enregistré : ${docState.path}`;
		await loadRecent();
		if (c.close) await closeDoc(false);
	}

	/** Classeur ouvert et affiché : ses écouteurs « open » (scripts Lua) s'exécutent, sans être attendus. */
	function workbookReady() {
		DocumentService.WorkbookReady().catch((e) => (notice = err(e)));
	}

	/** Récupère les données lisibles d'un classeur endommagé, dans un nouveau classeur. */
	async function salvage(path: string | null) {
		damaged = null;
		// Écran de récupération vidé (session abandonnée, puis fichier endommagé) : un classeur neuf.
		const fallback = () => (mode === 'recover' && orphans.length === 0 ? startDoc() : undefined);
		if (!path) return fallback();
		let r;
		try {
			r = await DocumentService.Salvage(path);
		} catch (e) {
			notice = err(e);
			return fallback();
		}
		view = 'grid';
		sheetId = 1;
		mode = 'doc';
		await reload();
		notice = r.report;
	}

	/** Ferme le classeur ; modifié, il demande d'abord s'il faut l'enregistrer. */
	async function closeDoc(save?: boolean) {
		closing = false;
		// Écouteurs « before_close » des scripts : ils peuvent garder le classeur ouvert.
		if (mode === 'doc' && save === undefined && !(await DocumentService.BeforeClose())) {
			quitting = false;
			return;
		}
		if (mode === 'doc' && docState.dirty && save === undefined) {
			closing = true;
			return;
		}
		if (save) {
			docState = await saving(() => DocumentService.Save(false));
			if (docState.conflict) {
				// Fichier modifié par un autre poste : la question d'abord, puis la fermeture.
				conflict = { ...docState.conflict, close: true };
				return;
			}
			if (docState.dirty) {
				// Enregistrement abandonné (ou refusé par un script : before_save).
				if (docState.notice) notice = docState.notice;
				quitting = false;
				return;
			}
		}
		sheet?.closeOverlays();
		await DocumentService.CloseDocument();
		docState = { open: false, path: '', title: '', dirty: false, undo: '', redo: '' };
		layout = null;
		share = null;
		sheetStatus = null;
		view = 'grid';
		mode = 'closed';
		if (quitting) {
			quitting = false;
			await DocumentService.QuitApp();
			return;
		}
		await loadRecent();
	}

	/** Croix de la fenêtre avec un classeur modifié : la question de Fichier › Fermer, puis Lattis se ferme. */
	async function requestQuit() {
		DocumentService.CloseRequestSeen().catch(() => {});
		if (quitting) return; // question déjà posée
		quitting = true;
		try {
			await closeDoc();
		} catch (e) {
			quitting = false;
			throw e;
		}
	}

	/** Réponse « Annuler » à la question d'enregistrement. */
	function cancelClose() {
		closing = false;
		quitting = false;
	}

	async function startDoc() {
		mode = 'doc';
		view = 'grid';
		sheetId = 1;
		queryHost?.closePanel();
		await DocumentService.New();
		await reload();
	}

	/** Fabrique une démonstration (open) en affichant son avancement, puis l'affiche. */
	async function buildDemo(label: string, open: () => Promise<{ sheet: number }>) {
		let waiting = true;
		const poll = async () => {
			if (!waiting) return;
			const p = await DocumentService.DemoProgress().catch(() => null);
			if (p && waiting) demoProgress = demoProgressText(label, p);
			setTimeout(poll, 250);
		};
		poll();
		try {
			const r = await open();
			view = 'grid';
			sheetId = r.sheet;
			mode = 'doc';
			await reload();
		} finally {
			waiting = false;
			demoProgress = '';
		}
	}

	/** Ouvre le classeur de démonstration demandé au lancement (--demo preset). */
	async function startPresetDemo(preset: string) {
		await buildDemo(`Démo « ${preset} »`, () => DocumentService.OpenDemo());
	}

	/** Classeur d'exemple de l'accueil : la vitrine, jamais enregistrée d'elle-même. */
	async function openSample() {
		mode = 'starting';
		queryHost?.closePanel();
		try {
			await buildDemo('Classeur d’exemple', () => DocumentService.OpenSample());
			workbookReady();
		} catch (e) {
			mode = 'closed';
			throw e;
		}
	}

	async function startDemo() {
		mode = 'demo';
		const poll = async () => {
			const s = await GridService.Status();
			if (s.state === 'ready') {
				demo = {
					name: s.info.name,
					rows: s.info.rows,
					columns: (s.info.columns ?? []).map((c) => ({ name: c.name, type: c.type as ColumnType })),
					sqlName: s.info.sqlName,
					sqlColumns: s.info.sqlColumns ?? []
				};
			} else if (s.state === 'error') {
				failure = s.error ?? 'erreur';
			} else {
				demoProgress = s.state === 'generating' ? `Création de la démo : ${s.done.toLocaleString('fr-FR')} / ${s.total.toLocaleString('fr-FR')} lignes` : 'Ouverture de la démo…';
				setTimeout(poll, 250);
			}
		};
		poll();
	}

	// Pas de menu natif du navigateur (Retour, Actualiser, Inspecter…) : les
	// zones qui ont un menu ouvrent le leur ; les champs de saisie gardent le
	// menu du système (couper, copier, coller).
	function blockNativeMenu(e: MouseEvent) {
		const t = e.target as HTMLElement | null;
		if (t?.closest('input, textarea, [contenteditable="true"], .cm-editor')) return;
		e.preventDefault();
	}

	/** La feuille affichée a disparu (classeur ouvert par un assistant, feuille supprimée) : première feuille. */
	async function keepSheet() {
		if (layout && visibleSheet(layout.sheets ?? [], sheetId) !== sheetId) {
			sheetId = visibleSheet(layout.sheets ?? [], sheetId);
			await reload();
		}
	}

	/** Commandes de l'interface venues d'un script ou du serveur MCP (internal/bridge/host.go). */

	async function uiCommand(c: UICommand) {
		switch (c.kind) {
			case 'select': {
				const sel = c.selection;
				if (!sel || mode !== 'doc') return;
				if (view !== 'grid') {
					sheet?.closeOverlays();
					view = 'grid';
				}
				if (sel.sheet && sel.sheet !== sheetId) {
					sheet?.closeOverlays();
					sheetId = sel.sheet;
				}
				await reload();
				await tick();
				await new Promise((r) => requestAnimationFrame(r));
				sheet?.selectRange(sel.r0, sel.c0, sel.r1, sel.c1);
				break;
			}
			case 'view':
				if (c.view === 'grid' || c.view === 'sql' || c.view === 'script') {
					if (c.view !== view) sheet?.closeOverlays();
					view = c.view;
				}
				break;
			case 'notify':
				notice = c.message ?? '';
				break;
			case 'dialog':
				if (c.dialog) dialogs = [...dialogs, c.dialog as DialogSpec];
				break;
			case 'dialog-close':
				dialogs = dialogs.filter((d) => d.id !== c.id);
				break;
		}
	}

	function answerDialog(d: DialogSpec, values: Record<string, unknown> | null) {
		dialogs = dialogs.filter((x) => x.id !== d.id);
		DocumentService.DialogAnswer(d.id, values ?? {}, values !== null).catch((e) => (notice = err(e)));
		if (!dialogs.length) sheet?.focus();
	}

	// Feuille affichée, sélection et vue, lues par ui.selection (scripts, MCP).
	let lastSel = '';
	$effect(() => {
		if (mode !== 'doc') return;
		const s = sheetStatus?.sel ?? { r0: 0, c0: 0, r1: 0, c1: 0 };
		const sel = { sheet: sheetId, r0: s.r0, c0: s.c0, r1: s.r1, c1: s.c1, view };
		const k = JSON.stringify(sel);
		if (k === lastSel) return;
		lastSel = k;
		DocumentService.SetSelection(sel).catch(() => {});
	});

	// Tuiles des cartes : par le classeur, soumises à l'autorisation de l'hôte.
	setTileFetcher((z, x, y) => DocumentService.Call('charts.tile', { z, x, y }) as Promise<string>);

	/**
	 * F5 : dans un tableau chargé par une requête, la relance ; ailleurs,
	 * recalcule (le recalcul vaut pour tout le classeur). Ctrl+F5, Maj+F5 et
	 * Données › Actualiser tout : toutes les requêtes, puis le recalcul.
	 */
	let refreshing = false;
	async function refreshKey(all: boolean) {
		if (mode !== 'doc' || refreshing) return;
		refreshing = true;
		sheet?.clearMessage(); // sinon il masquerait le bilan de l'actualisation
		try {
			if (all) {
				const r = (await queryHost?.refreshAll()) ?? { ok: true, count: 0 };
				if (!r.ok) return; // erreur ou abandon : QueryHost l'a dit
				await DocumentService.Recalc();
				await reload();
				notice = r.count ? `Tout actualisé : ${r.count} requête${r.count > 1 ? 's' : ''}, formules recalculées` : 'Formules recalculées (aucune requête chargée à actualiser)';
				return;
			}
			const t = view === 'grid' ? sheet?.tableHere() : undefined;
			if (t?.query && queryHost) {
				await queryHost.refreshOne(t.query.id); // message : QueryHost
				return;
			}
			await DocumentService.Recalc();
			await reload();
			notice = t ? `Tableau « ${t.name} » recalculé, avec ce qui en dépend` : 'Formules recalculées';
		} catch (e) {
			notice = err(e);
		} finally {
			refreshing = false;
		}
	}

	onMount(() => {
		installErrorReporting();
		// Fil des actions de Claude Desktop / Claude Code : écouté dès le
		// démarrage (réglages lus) pour ne rien manquer avant l'ouverture du
		// volet.
		loadSettings().then(() => {
			if (mcpAvailable()) listenExternal();
		});
		const off = Events.On('doc:changed', (e: { data?: DocState }) => {
			// Tableaux fusionnés, cellules rattachées : annoncé après la
			// relecture, pour passer après le message de l'action elle-même.
			const said = e?.data?.notice ?? '';
			if (mode === 'doc')
				reload()
					.then(keepSheet)
					.then(() => {
						if (said) setTimeout(() => (sheet ? sheet.announce(said) : (notice = said)), 250);
					});
			else if (said) notice = said;
		});
		// Classeur partagé : connexion perdue ou retrouvée, nouvelle version
		// publiée (tableaux en lecture seule ou non : mise en page relue).
		const offShare = Events.On('share:changed', (e: { data?: ShareState }) => {
			if (e?.data) share = e.data;
			if (mode === 'doc') reload().catch(() => {});
		});
		// Source du Data Lake mise à jour à l'enregistrement (en arrière-plan).
		const offSource = Events.On('data:source', (e: { data?: { message?: string } }) => {
			if (e?.data?.message) notice = e.data.message;
			loadSources();
		});
		const offUI = Events.On('ui:command', (e: { data: UICommand }) => {
			uiCommand(e.data).catch((x) => (notice = err(x)));
		});
		// Classeur ouvert depuis le système pendant que l'application tourne
		// (macOS : double-clic sur un .lattis dans le Finder).
		// Croix de la fenêtre (bureau) avec un classeur modifié.
		const offQuit = Events.On('window:close-request', () => {
			requestQuit().catch((x) => (notice = err(x)));
		});
		const offOpen = Events.On('doc:open-file', (e: { data: string }) => {
			openPath(e.data).catch((x) => (notice = err(x)));
		});
		// Lien lattis:// reçu pendant que l'application tourne.
		const offLink = Events.On('doc:open-link', (e: { data: string }) => {
			openLink(e.data).catch((x) => (notice = err(x)));
		});
		(async () => {
			try {
				bench = await GridService.BenchRequested();
				if (bench) return startDemo();
				loadRecent();
				// Le moteur survit à la page (web : rechargement, onglet fermé
				// puis rouvert ; bureau : rechargement de la fenêtre, wails3 dev) :
				// on reprend le classeur qu'il a ouvert, au lieu d'en créer un
				// autre par-dessus (qui fermait l'ancien, modifié ou non).
				if ((await DocumentService.State()).open) {
					mode = 'doc';
					view = 'grid';
					sheetId = 1;
					return await reload();
				}
				const preset = await DocumentService.DemoRequested();
				if (preset) return await startPresetDemo(preset);
				// Classeur passé par l'association de fichiers (double-clic sur un .lattis).
				const launch = await DocumentService.LaunchFile();
				if (launch) {
					try {
						return await openPath(launch);
					} catch (e) {
						notice = err(e); // fichier illisible : on démarre comme d'habitude
					}
				}
				// Lien lattis:// du lancement : un classeur vide le temps de le suivre.
				const link = await DocumentService.LaunchLink();
				if (link) {
					await startDoc();
					openLink(link).catch((x) => (notice = err(x)));
					return;
				}
				orphans = (await DocumentService.Orphans()) ?? [];
				if (orphans.length > 0) mode = 'recover';
				else await startDoc();
			} catch (e) {
				failure = err(e);
			}
		})();
		const keys = (e: KeyboardEvent) => {
			if (e.defaultPrevented) return; // déjà traité (éditeur de script : Ctrl+S)
			// Boîte modale ouverte (éditeur de requête…) : ni Ctrl+W, ni Ctrl+N, ni Ctrl+O sous elle.
			if (modalOpen()) return;
			const ctrl = e.ctrlKey || e.metaKey;
			const action: FileAction | null =
				e.key === 'F12' ? 'saveAs' : ctrl && e.key.toLowerCase() === 's' ? (e.shiftKey ? 'saveAs' : 'save') : ctrl && e.key.toLowerCase() === 'o' ? 'open' : ctrl && e.key.toLowerCase() === 'n' ? 'new' : ctrl && e.key.toLowerCase() === 'w' ? 'close' : ctrl && e.key.toLowerCase() === 'p' && mode === 'doc' ? 'print' : null;
			if (action) {
				e.preventDefault();
				file(action);
				return;
			}
			// Onglets : Ctrl+Pg.Suiv / Ctrl+Pg.Préc (feuille suivante, précédente), Maj+F11 (insérer une feuille).
			const sc = view === 'grid' && !protecting ? shortcutOf(e) : null;
			if (sc === 'nextSheet' || sc === 'prevSheet') {
				e.preventDefault();
				const k = tabs.findIndex((t) => t.id === sheetId) + (sc === 'nextSheet' ? 1 : -1);
				if (k >= 0 && k < tabs.length) selectSheet(tabs[k].id);
			} else if ((sc === 'nextChoice' || sc === 'prevChoice') && mode === 'doc') {
				e.preventDefault();
				stepChoice(sc === 'nextChoice' ? 1 : -1);
			} else if (sc === 'insertSheet' && mode === 'doc') {
				e.preventDefault();
				if (layout?.workbookProtected) notice = 'La structure du classeur est protégée : ôtez la protection pour insérer une feuille';
				else sheetAction(async () => selectSheet(await DocumentService.InsertSheet(sheetId)));
			}
		};
		window.addEventListener('keydown', keys);
		// Web : Ctrl+W ou la fermeture de l'onglet demandent confirmation
		// quand le classeur a des modifications non enregistrées.
		const leave = (e: BeforeUnloadEvent) => {
			if (isWeb && mode === 'doc' && docState.dirty) e.preventDefault();
		};
		window.addEventListener('beforeunload', leave);
		// F5 et Ctrl+R ne rechargent jamais la page de l'application (fenêtre
		// WebView2 ou navigateur) ; F5 actualise (refreshKey). En capture : rien
		// ne peut l'arrêter avant.
		const noReload = (e: KeyboardEvent) => {
			if (!isReloadKey(e)) return;
			e.preventDefault();
			const sc = shortcutOf(e);
			if ((sc !== 'refresh' && sc !== 'refreshAll') || e.repeat) return;
			const t = e.target as HTMLElement | null;
			if (t && (t.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(t.tagName))) return; // saisie en cours
			refreshKey(sc === 'refreshAll');
		};
		window.addEventListener('keydown', noReload, true);
		// F6 (Maj+F6 : en arrière) : ruban, barre de formule, grille, volet,
		// onglets, barre d'état. En capture : les éditeurs ne la gardent pas.
		const regions = (e: KeyboardEvent) => {
			if (e.key !== 'F6' || e.ctrlKey || e.altKey || e.metaKey) return;
			e.preventDefault();
			if (modalOpen()) return;
			e.stopPropagation();
			cycleRegions(e.shiftKey);
		};
		window.addEventListener('keydown', regions, true);
		// F1 : l'aide. Dans une formule en cours de saisie, la page de la
		// fonction sous le curseur (la saisie garde le focus) ; ailleurs, la
		// page de ce qui est affiché (SQL, Script, onglet HTML ou Document).
		const helpKey = (e: KeyboardEvent) => {
			if (e.key !== 'F1' || e.ctrlKey || e.altKey || e.metaKey) return;
			e.preventDefault();
			e.stopPropagation();
			const el = document.activeElement;
			if (el instanceof HTMLElement && el.closest('.help')) return;
			if (el instanceof HTMLElement && el.closest('.ai-panel')) return openHelp('assistant', false);
			if ((el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) && /^\s*=/.test(el.value)) {
				const f = functionAt(el.value, el.selectionStart ?? el.value.length, help?.functions ?? []);
				openHelp(f ? `fonction:${f.name}` : 'formules', false);
				return;
			}
			const editing = el instanceof HTMLElement && (el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName));
			openHelp(viewTarget(view, activePage ? (activeDoc ? 'doc' : 'html') : ''), !editing);
		};
		window.addEventListener('keydown', helpKey, true);
		// Un texte ou un lien lâché hors d'une zone prévue ne doit jamais faire
		// quitter la page de l'application (la fenêtre afficherait une erreur).
		const noDrop = (e: DragEvent) => e.preventDefault();
		window.addEventListener('dragover', noDrop);
		window.addEventListener('drop', noDrop);
		return () => {
			off();
			offUI();
			offOpen();
			offQuit();
			offLink();
			offShare();
			offSource();
			window.removeEventListener('keydown', keys);
			window.removeEventListener('beforeunload', leave);
			window.removeEventListener('keydown', noReload, true);
			window.removeEventListener('keydown', regions, true);
			window.removeEventListener('keydown', helpKey, true);
			window.removeEventListener('dragover', noDrop);
			window.removeEventListener('drop', noDrop);
		};
	});

	async function file(a: FileAction) {
		if (a === 'help') return openHelp();
		if (a === 'report') return reportIssue();
		if (a === 'about') {
			aboutOpen = true;
			return;
		}
		notice = '';
		try {
			switch (a) {
				case 'new':
					await startDoc();
					break;
				case 'open':
					await openPath('');
					break;
				case 'openRemote':
					openRemote = true;
					break;
				case 'rights':
					if (canRights) rightsOpen = true;
					break;
				case 'share':
				case 'republish':
					if (mode !== 'doc') return;
					shareDlg = { republish: a === 'republish' };
					break;
				case 'close':
					await closeDoc();
					break;
				case 'save':
				case 'saveAs':
					if (mode !== 'doc') return;
					docState = await saving(() => DocumentService.Save(a === 'saveAs'));
					if (docState.conflict) conflict = { ...docState.conflict, close: false };
					else if (docState.notice) notice = docState.notice;
					else if (!docState.dirty) notice = `Enregistré : ${docState.path}`;
					await loadRecent();
					break;
				case 'import': {
					// Depuis l'accueil : un classeur neuf, refermé si l'import est annulé.
					const started = mode !== 'doc';
					if (started) await startDoc();
					const r = await DocumentService.Import('');
					if (r.cancelled) {
						if (started) await closeDoc();
						break;
					}
					if (r.sheet) sheetId = r.sheet;
					await reload();
					const tables = r.tables ?? [];
					const what = tables.length === 1 ? `Tableau « ${tables[0]} » importé` : count(tables.length, 'tableau importé', 'tableaux importés');
					const warnings = r.warnings ?? [];
					notice = warnings.length ? `${what} ; ${warnings.length} point(s) à vérifier : ${warnings.join(' · ')}` : what;
					break;
				}
				case 'export': {
					if (mode !== 'doc') return;
					const table = sheet?.activeTable() ?? layout?.tables?.[0]?.name;
					if (!table) {
						notice = 'Aucun tableau à exporter sur cette feuille';
						break;
					}
					const p = await DocumentService.ExportCSV(table);
					if (p) notice = `Tableau « ${table} » exporté : ${p}`;
					break;
				}
				case 'exportSheet': {
					// Toute la feuille active, de A1 à la dernière cellule utilisée.
					if (mode !== 'doc') return;
					const name = layout?.sheets?.find((s) => s.id === sheetId)?.name ?? 'Feuille';
					const p = await DocumentService.ExportSheetCSV(sheetId, name);
					if (p) notice = `Feuille « ${name} » exportée : ${p}`;
					break;
				}
				case 'exportXlsx':
				case 'exportOds': {
					// Formules traduites quand le format a un équivalent, sinon leur valeur.
					if (mode !== 'doc') return;
					const r = a === 'exportOds' ? await DocumentService.ExportODS() : await DocumentService.ExportXLSX();
					if (r.path) {
						notice = exportNotice(r.path, r.report);
						exported = { notice, report: r.report, format: a === 'exportOds' ? 'OpenDocument' : 'Excel' };
					}
					break;
				}
				case 'print': {
					// Feuille active selon sa mise en page ; le PDF passe par la
					// fenêtre d'impression (imprimante PDF du système).
					if (mode !== 'doc') return;
					if (activeDoc && docPage) {
						exportDocPdf();
						break;
					}
					const name = layout?.sheets?.find((s) => s.id === sheetId)?.name ?? '';
					const charts = printNotice(layout?.charts?.length ?? 0);
					if (charts) notice = charts;
					printSheet(await DocumentService.PrintHTML(sheetId), `${docState.title} – ${name}`);
					// Comme Excel : après une impression, la grille montre les sauts de page.
					showBreaks = true;
					break;
				}
				case 'demo':
					await startDemo();
					break;
				case 'settings':
					settingsOpen = true;
					break;
			}
		} catch (e) {
			notice = err(e);
		}
		sheet?.focus();
	}

	/** Récupère (keep) ou abandonne une session ; also : versions plus anciennes du même classeur, abandonnées avec elle. */
	async function recover(o: Orphan, keep: boolean, also: Orphan[] = []) {
		recoverError = '';
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
			recoverError = err(e);
			if (gone.size) orphans = orphans.filter((x) => !gone.has(x.dir));
			return;
		}
		try {
			if (keep) {
				mode = 'doc';
				await reload();
				// Fichier verrouillé ou modifié par un autre poste depuis la session : dit aussi.
				notice = ['Modifications récupérées : pensez à enregistrer.', st?.notice].filter(Boolean).join(' ');
				workbookReady();
			} else {
				orphans = orphans.filter((x) => !gone.has(x.dir));
				if (orphans.length > 0) return;
				// Session d'un fichier abandonnée : le fichier tel qu'enregistré.
				if (o.source) await openPath(o.source).catch(() => startDoc());
				else await startDoc();
			}
		} catch (e) {
			failure = err(e);
		}
	}

	// --- Sources de la feuille ----------------------------------------------

	const docSource: SheetSource = {
		window: (t, first, count) => DocumentService.Window(sheetId, t.id, first, count) as Promise<Fetched>,
		async setCell(row, col, raw, _target, key) {
			const ev = await (key ? DocumentService.SetCellKeyed(sheetId, row, col, raw, key) : DocumentService.SetCell(sheetId, row, col, raw));
			// Relire tout de suite (l'événement doc:changed sert aux changements venus
			// d'ailleurs, et n'existe pas en mode serveur).
			await reload();
			if (ev.keyed) return `Clé posée sur « ${ev.keyed.slice(ev.keyed.indexOf('.') + 1)} » pour REFERENCE`;
			const undoBirth = ev.split ? ' (Ctrl+Z : le défaire, les cellules restent libres)' : '';
			if (ev.created && ev.headless) return `Tableau « ${ev.created} » créé sans en-têtes (« Colonne n ») : tapez les noms dans sa ligne d'en-tête${undoBirth}`;
			if (ev.created) return `Tableau « ${ev.created} » créé${undoBirth}`;
			if (ev.widened) return `Colonne ${ev.widened} passée de ${typeLabel(ev.from as ColumnType)} à ${typeLabel(ev.to as ColumnType)} (Ctrl+Z pour annuler)`;
			if (ev.invalid) return 'Valeur hors type : cellule marquée en erreur';
		},
		editing: (row, col, on) => DocumentService.ShareEditing(sheetId, row, col, on),
		undo: () => DocumentService.Undo().then(reload),
		redo: () => DocumentService.Redo().then(reload),
		async paste(row, col, block) {
			const ev = await DocumentService.Paste(sheetId, row, col, block);
			await reload();
			if (ev.refusedCount) return refusedNotice(ev);
			if (ev.created && ev.headless) return `Tableau « ${ev.created} » créé sans en-têtes : clic droit › Tableau › « Sortir l'en-tête » si besoin`;
			if (ev.created) return `Tableau « ${ev.created} » créé (${count(block.length - 1)}${ev.split ? ' ; Ctrl+Z : le défaire, les cellules restent libres' : ''})`;
		},
		rangeStats: (t, first, count, cols) => DocumentService.RangeStats(sheetId, t.id, first, count, cols),
		moveCells: (r0, c0, r1, c1, dr, dc) => DocumentService.MoveCells(sheetId, r0, c0, r1, c1, dr, dc).then(reload),
		clearCells: (r0, c0, r1, c1) => DocumentService.ClearCells(sheetId, r0, c0, r1, c1).then(reload),
		clearRange: (r0, c0, r1, c1, all) => DocumentService.ClearRange(sheetId, r0, c0, r1, c1, all).then(reload),
		async transfer(from, src, dst, move, values) {
			const ev = await DocumentService.Transfer(from, src, sheetId, dst, move, values);
			await reload();
			if (ev.refusedCount) return refusedNotice(ev);
			if (ev.created) return `Tableau « ${ev.created} » créé`;
			if (ev.widened) return `Colonne ${ev.widened} passée de ${typeLabel(ev.from as ColumnType)} à ${typeLabel(ev.to as ColumnType)} (Ctrl+Z pour annuler)`;
		},
		sheetLines: (op, a, b) =>
			(op === 'insertRows'
				? DocumentService.InsertSheetRows(sheetId, a, b)
				: op === 'deleteRows'
					? DocumentService.DeleteSheetRows(sheetId, a, b)
					: op === 'insertCols'
						? DocumentService.InsertSheetColumns(sheetId, a, b)
						: DocumentService.DeleteSheetColumns(sheetId, a, b)
			).then(reload),
		insertRow: (t, n) => DocumentService.InsertRow(t.name, n).then(reload),
		deleteRow: (t, n) => DocumentService.DeleteRow(t.name, n).then(reload),
		insertColumn: (t, i) => DocumentService.InsertColumn(t.name, i, `Colonne ${t.columns.length + 1}`).then(reload),
		deleteColumn: (t, i) => DocumentService.DeleteColumn(t.name, i).then(reload),
		suppressColumns: (t, i, j) => DocumentService.SuppressColumns(t.name, i, j).then(reload),
		deleteColumnSpans: (spans) => DocumentService.DeleteColumnSpans(spans).then(reload),
		convertPreview: (t, i, to) => DocumentService.ConvertPreview(t.name, i, to as Type),
		typeMix: (t, i) => DocumentService.TypeMix(t.name, i) as Promise<TypeMix>,
		setType: (t, i, to) => DocumentService.SetType(t.name, i, to as Type).then(reload),
		setColumnWidth: (col, width) => DocumentService.SetColumnWidth(sheetId, col, width).then(reload),
		setRowHeight: (row, height) => DocumentService.SetRowHeight(sheetId, row, height).then(reload),
		setName: (row, col, name) => DocumentService.SetName(sheetId, row, col, name).then(reload),
		sortTable: (t, i, desc) => DocumentService.SortTable(t.name, i, desc).then(reload),
		refreshTable: (t) => queryHost?.refreshOne(t.query!.id) ?? Promise.resolve(),
		setRules: (t, i, rules) => DocumentService.SetRules(t.name, i, rules).then(reload),
		setLineRules: (rows, i0, i1, rules) => DocumentService.SetLineRules(sheetId, rows, i0, i1, rules).then(reload),
		setSheetRules: (ch) => DocumentService.SetSheetRules(sheetId, ch as RuleChanges).then(reload),
		ruleStats: (t, cols, req) => DocumentService.RuleStats(sheetId, t.id, cols, req),
		filterValues: async (t, i) => (await DocumentService.FilterValues(t.name, i)) ?? [],
		setFilter: (t, i, f) => DocumentService.SetFilter(t.name, i, f).then(reload),
		clearFilters: (t) => DocumentService.ClearFilters(t.name).then(reload),
		renameTable: (t, name) => DocumentService.RenameTable(t.name, name).then(reload),
		promoteHeader: (t) => DocumentService.PromoteHeader(t.name).then(reload),
		headerToTitle: (t) => DocumentService.HeaderToTitle(t.name).then(reload),
		setTwoLevel: (t, on) => DocumentService.SetTwoLevel(t.name, on).then(reload),
		rowToNote: (t, n) => DocumentService.RowToNote(t.name, n).then(reload),
		joinTable: (row, col) => DocumentService.JoinTable(sheetId, row, col).then(reload),
		keyCheck: (t, i) => DocumentService.KeyCheck(t.name, i),
		setKey: (t, i) => DocumentService.SetKey(t.name, i).then(reload),
		clearKey: (t) => DocumentService.ClearKey(t.name).then(reload),
		setHandKey: (t, column) => DocumentService.SetHandKey(t.name, column).then(reload),
		forgetHandKept: (t) => DocumentService.ForgetHandKept(t.name).then(reload),
		dismissKeyHint: (t) => DocumentService.DismissKeyHint(t.name).then(reload),
		recalc: () => DocumentService.Recalc().then(reload),
		dependencies: async (row, col) => {
			const d = await DocumentService.Dependencies(sheetId, row, col);
			return { subject: d.subject, precedents: d.precedents ?? [], dependents: d.dependents ?? [] };
		},
		createPivot: async (cfg) => {
			const id = await DocumentService.CreatePivot(cfg);
			await reload();
			await selectSheet(id);
		},
		setPivot: (id, cfg) => DocumentService.SetPivot(id, cfg).then(reload),
		placePivot: (row, col, cfg) => DocumentService.PlacePivot(sheetId, row, col, cfg).then(reload),
		pivotFieldValues: async (table, field) => (await DocumentService.FieldValues(table, field)) ?? [],
		createChart: (row, col, cfg) => DocumentService.CreateChart(sheetId, row, col, cfg).then(reload),
		setChart: (id, cfg) => DocumentService.SetChart(id, cfg).then(reload),
		moveChart: (id, row, col) => DocumentService.MoveChart(id, row, col).then(reload),
		placeChart: (id, row, col, offX, offY, w, h) => DocumentService.PlaceChart(id, row, col, offX, offY, w, h).then(reload),
		deleteChart: (id) => DocumentService.DeleteChart(id).then(reload),
		scripts: async () => (await DocumentService.Scripts()) ?? [],
		runButton: async (id) => {
			const r = await DocumentService.RunButton(id);
			if (r.error) return `Erreur du script : ${r.error}`;
			const out = r.output ?? [];
			return out.length ? out[out.length - 1] : '';
		},
		fireControl: async (id, event, value, old) => {
			const r = await DocumentService.FireControl(id, event, value, old);
			if (r.error) return `Erreur du script : ${r.error}`;
			const out = r.output ?? [];
			return out.length ? out[out.length - 1] : '';
		},
		pickFile: (title) => DocumentService.PickControlFile(title),
		createButton: (row, col, cfg: ButtonConfig) => DocumentService.CreateButton(sheetId, row, col, cfg).then(async (id) => (await reload(), id)),
		setButton: (id, cfg: ButtonConfig) => DocumentService.SetButton(id, cfg).then(reload),
		placeButton: (id, row, col, offX, offY, w, h) => DocumentService.PlaceButton(id, row, col, offX, offY, w, h).then(reload),
		deleteButton: (id) => DocumentService.DeleteButton(id).then(reload),
		deletePivot: (id) => DocumentService.DeletePivot(id).then(reload),
		movePivot: (id, row, col) => DocumentService.MovePivot(id, sheetId, row, col).then(reload),
		copyPivot: (id, row, col) => DocumentService.CopyPivot(id, sheetId, row, col).then(reload),
		duplicatesPreview: (t, cols) => DocumentService.DuplicatesPreview(t.name, cols),
		removeDuplicates: async (t, cols) => {
			const n = await DocumentService.RemoveDuplicates(t.name, cols);
			await reload();
			return n;
		},
		textColumnsPreview: (r0, r1, col, o) => DocumentService.TextToColumnsPreview(sheetId, r0, r1, col, o),
		textToColumns: (r0, r1, col, o) => DocumentService.TextToColumns(sheetId, r0, r1, col, o).then((n) => {
			// L'assistant se ferme tout de suite ; la feuille se recharge ensuite.
			reload().catch((e) => (notice = err(e)));
			return n;
		}),
		deleteTables: (ts) => DocumentService.DeleteTables(ts.map((t) => t.name)).then(reload),
		convertToRange: (t) => DocumentService.ConvertToRange(t.name).then(reload),
		formatAsTable: (row, col) => DocumentService.FormatAsTable(sheetId, row, col).then(reload),
		moveTable: (t, row, col) => DocumentService.MoveTable(t.name, row, col).then(reload),
		extractColumns: (t, i0, i1, row, col) => DocumentService.ExtractColumns(t.name, i0, i1, row, col).then(reload),
		applyStyle: (r0, c0, r1, c1, patch) => DocumentService.ApplyStyle(sheetId, r0, c0, r1, c1, patch).then(reload),
		applyLineStyle: (rows, i0, i1, patch) => DocumentService.ApplyLineStyle(sheetId, rows, i0, i1, patch).then(reload),
		setNote: (row, col, text) => DocumentService.SetNote(sheetId, row, col, text).then(reload),
		pinNote: (row, col, pinned) => DocumentService.PinNote(sheetId, row, col, pinned).then(reload),
		mergeCells: async (r0, c0, r1, c1) => {
			const n = await DocumentService.MergeCells(sheetId, r0, c0, r1, c1);
			await reload();
			return n;
		},
		unmergeCells: (r0, c0, r1, c1) => DocumentService.UnmergeCells(sheetId, r0, c0, r1, c1).then(reload),
		setLinesHidden: (rows, i0, i1, hide) => DocumentService.SetLinesHidden(sheetId, rows, i0, i1, hide).then(reload),
		setFreeze: (rows, cols) => DocumentService.SetFreeze(sheetId, rows, cols).then(reload),
		find: async (text, zone) => (await DocumentService.Find(sheetId, text, zone)) ?? [],
		replace: async (text, by, zone) => {
			const n = await DocumentService.Replace(sheetId, text, by, zone);
			await reload();
			return n;
		}
	};

	const demoSource: SheetSource = {
		window: (_t, first, count) => GridService.Window(first, count) as Promise<Fetched>,
		async setCell(_row, _col, raw, target) {
			if (target.rowId === undefined || target.column === undefined) return 'Démo : seules les cellules du tableau sont modifiables';
			// La démo ne relit pas sa mise en page : la feuille met la cellule à jour.
			return { value: await GridService.SetCell(target.rowId, target.column, raw) };
		}
	};

	const docData = $derived<SheetData | null>(
		layout
			? {
					tables: (layout.tables ?? []).map((t) => {
						const filter = t.filter ? (JSON.parse(t.filter) as Record<string, ColumnFilter>) : {};
						return {
						id: t.id,
						name: t.name,
						row: t.row,
						col: t.col,
						rows: t.rows,
						hidden: t.total - t.rows,
						key: t.key >= 0 ? t.key : undefined,
						keyHint: t.keyHint || undefined,
						twoLevel: t.twoLevel || undefined,
						query: t.queryId ? { id: t.queryId, name: t.queryName } : undefined,
						handKey: t.handKey || undefined,
						handKept: t.handKept || undefined,
						shared: (t.shared || undefined) as 'published' | 'private' | undefined,
						readOnly: t.readOnly || undefined,
						access: t.access || undefined,
						filtered: t.filtered || undefined,
						follows: t.follows || undefined,
						choiceCol: t.follows ? (t.choiceCol ?? 0) : undefined,
						columns: (t.columns ?? []).map((c) => ({
							name: c.name,
							type: (c.formula ? 'formula' : c.type) as ColumnType,
							fixed: c.fixed,
							errors: c.errors,
							formula: c.formula || undefined,
							result: c.result || undefined,
							hand: c.hand || undefined,
							style: c.style || undefined,
							headerStyle: c.headerStyle || undefined,
							rules: c.rules || undefined,
							filter: filter[String(c.id)]
						}))
					};
					}),
					sheet: sheetId,
					choice: layout.sheets?.find((s) => s.id === sheetId)?.choice ?? null,
					cells: (layout.cells ?? []).map((c) => ({ ...c, spill: c.spill?.map((r) => r ?? []) })),
					widths: layout.widths ?? [],
					heights: layout.heights ?? [],
					names: layout.names ?? [],
					styles: layout.styles ?? [],
					rowStyles: layout.rowStyles ?? [],
					colStyles: layout.colStyles ?? [],
					rowRules: layout.rowRules ?? [],
					colRules: layout.colRules ?? [],
					ruleMarks: layout.ruleMarks ?? [],
					charts: (layout.charts ?? []).map((c) => ({
						...c,
						config: c.config as ChartConfig,
						labels: c.labels ?? [],
						series: (c.series ?? []).map((s) => ({ name: s.name, values: (s.values ?? []).map((v) => v ?? null) }))
					})),
					pivots: (layout.pivots ?? []).map((p) => ({ ...p, config: p.config as PivotConfig, cells: (p.cells ?? []).map((r) => r ?? []) })),
					buttons: (layout.buttons ?? []).map((b) => ({ ...b, config: b.config as ButtonConfig })),
					ranges: layout.ranges ?? []
				}
			: null
	);

	const demoData = $derived<SheetData | null>(demo ? { tables: [{ id: 1, name: demo.name, row: 0, col: 0, rows: demo.rows, columns: demo.columns }], cells: [] } : null);

	const objects = $derived(
		mode === 'demo' && demo
			? [{ name: demo.name, sqlName: demo.sqlName, rows: demo.rows, columns: demo.columns.map((c, i) => ({ ...c, sql: demo!.sqlColumns[i] ?? '' })) }]
			: // Tableaux et tableaux croisés (vues) de toutes les feuilles.
				(help?.tables ?? []).map((t) => ({
					name: t.name,
					sqlName: t.sql,
					rows: t.rows,
					kind: t.kind === 'pivot' ? ('pivot' as const) : ('table' as const),
					key: t.key >= 0 ? t.key : undefined,
					columns: (t.columns ?? []).map((c, i) => ({ name: c, type: (t.types?.[i] ?? 'text') as ColumnType, sql: t.columnSql?.[i] ?? c }))
				}))
	);

	const query = (sql: string) => (mode === 'demo' ? GridService.Query(sql) : DocumentService.Query(sql));
	const data = $derived(mode === 'demo' ? demoData : docData);
	const title = $derived(mode === 'demo' ? 'Démo : 1 M de lignes' : mode === 'recover' ? 'Récupération' : docState.title);
	const tabs = $derived(mode === 'demo' ? (demo ? [{ id: 1, name: demo.name }] : []) : (layout?.sheets ?? []).map((s) => ({ id: s.id, name: s.name, protected: s.protected, color: s.color, hidden: s.hidden, kind: s.kind, choice: s.choice ?? null })));

	// --- Onglet à choix (« Ventes · 2025 ▾ », lib/shell/choice.ts) ----------
	/** Lignes d'autres valeurs masquées sur la feuille affichée (barre d'état). */
	function choiceHidden(): number {
		return (layout?.tables ?? []).filter((t) => t.follows).reduce((n, t) => n + t.total - t.rows, 0);
	}
	const choiceActions: ChoiceActions = {
		values: async (id) => (await DocumentService.SheetChoiceValues(id)) ?? [],
		choose: (id, value, all) =>
			sheetAction(async () => {
				await DocumentService.ChooseSheetValue(id, value, all);
				await reload();
				const c = layout?.sheets?.find((s) => s.id === id)?.choice;
				if (c && id === sheetId) notice = choiceStatus(c, choiceHidden());
			}),
		add: (id, value) => sheetAction(() => DocumentService.NewSheetChoiceValue(id, value)),
		candidates: async (id) => (await DocumentService.SheetChoiceCandidates(id)) ?? [],
		by: (id, table, column) =>
			sheetAction(async () => {
				await DocumentService.ChooseSheetBy(id, table, column, '');
				if (id !== sheetId) await selectSheet(id);
				notice = `Onglet à choix par « ${column} » : la flèche ▾ de l’onglet choisit la valeur affichée (Alt+↓)`;
			}),
		clear: (id) => sheetAction(() => DocumentService.ClearSheetChoice(id))
	};
	/** Ctrl+Alt+Pg.Suiv / Pg.Préc : valeur suivante ou précédente de l'onglet à choix affiché. */
	async function stepChoice(d: 1 | -1) {
		const c = layout?.sheets?.find((s) => s.id === sheetId)?.choice;
		if (!c) {
			notice = 'Cette feuille n’a pas d’onglet à choix (clic droit sur l’onglet › Choisir par)';
			return;
		}
		const values = (await choiceActions.values(sheetId)).map((v) => v.text);
		const next = choiceStep(values, c, d);
		if (next !== null) await choiceActions.choose(sheetId, next, false);
	}

	// --- Assistant IA (volet de droite, lib/ai) ------------------------------

	// Zone des volets du classeur (aide, requêtes, conflits, assistant), logée
	// à droite : la grille (et les vues SQL, Script) se loge à sa gauche.
	let dockWidth = $state(0);
	// Ce que l'utilisateur regarde, joint à chaque message et affiché au-dessus
	// de la saisie : feuille active, tableau et sélection (comme la zone
	// d'adresse). Suit la grille par sheetStatus, envoyé à chaque sélection.
	const aiContext = $derived.by(() => {
		if (mode !== 'doc') return null;
		void sheetStatus;
		const onGrid = view === 'grid' && !activePage;
		const here = onGrid ? sheet?.aiContext() : undefined;
		return { sheet: activeInfo?.name ?? '', table: here?.table ?? '', selection: here?.selection ?? '' };
	});
	const aiTables = $derived((help?.tables ?? []).map((t) => ({ name: t.name, rows: t.rows, kind: t.kind })));

	/** Cible d'une étape de l'assistant (tableau, sinon feuille) : y aller. */
	async function gotoTarget(name: string) {
		if ((help?.tables ?? []).some((t) => t.name.toLowerCase() === name.toLowerCase())) return gotoObject(name);
		const s = (layout?.sheets ?? []).find((x) => x.name.toLowerCase() === name.toLowerCase());
		if (s) await selectSheet(s.id);
		else notice = `« ${name} » est introuvable dans le classeur`;
	}

	/** « Afficher sur la feuille » : va sur la feuille de l'objet et le sélectionne. */
	async function gotoObject(name: string) {
		view = 'grid';
		const t = (help?.tables ?? []).find((x) => x.name.toLowerCase() === name.toLowerCase());
		if (t && t.sheet && t.sheet !== sheetId) {
			sheet?.closeOverlays();
			sheetId = t.sheet;
			await reload();
		}
		await tick();
		await new Promise((r) => requestAnimationFrame(r));
		sheet?.revealObject(name);
		sheet?.focus();
	}

	async function selectSheet(id: number) {
		view = 'grid';
		if (mode !== 'doc') return;
		if (id === sheetId) {
			// Onglet déjà affiché : une formule en cours de saisie reprend.
			sheet?.resumeFormula();
			return;
		}
		// Changement d'onglet : les volets de la feuille (TCD, graphique) se
		// ferment ; ceux du classeur (assistant, requêtes) restent, comme dans Excel.
		sheet?.closeOverlays();
		sheetId = id;
		await reload();
		sheet?.focus();
		// Document : le curseur dans le texte (DocPage le met aussi en
		// place à son montage, une fois le contenu chargé).
		if (activeDoc) docPage?.focus();
	}

	// --- Protection (Révision) ----------------------------------------------

	const activeInfo = $derived(layout?.sheets?.find((s) => s.id === sheetId));
	// Onglet page HTML ou Document affiché à la place de la grille.
	const activePage = $derived(mode === 'doc' && (activeInfo?.kind === 'html' || activeInfo?.kind === 'doc') && pageSource?.sheet === sheetId ? pageSource : null);
	const activeDoc = $derived(!!activePage && activeInfo?.kind === 'doc');
	const addPage = (before = 0) => sheetAction(async () => selectSheet(await DocumentService.AddPage(before)));
	const addDoc = (before = 0) => sheetAction(async () => selectSheet(await DocumentService.AddDoc(before)));

	// Document en PDF : la fenêtre d'impression (imprimante PDF du système).
	function exportDocPdf() {
		docPage?.print(`${docState.title} – ${activeInfo?.name ?? ''}`);
	}

	// Commandes du ruban pour l'onglet Document.
	function docCommand(c: DocCommand) {
		const d = docPage;
		if (!d) return;
		if (c.startsWith('block:')) d.setBlock(c.slice(6) as never);
		else if (c.startsWith('align:')) d.setAlign(c.slice(6) as never);
		else if (c.startsWith('chart:')) d.insertChart(c.slice(6));
		else if (c === 'button') d.insertButton();
		else if (c === 'pivot') d.insertPivot();
		else if (c === 'pdf') exportDocPdf();
		else if (c === 'input') d.insertControl('input');
		else d.command(c as never);
	}

	// Zoom de la feuille active (Affichage › Zoom, Ctrl+molette, barre d'état) :
	// appliqué tout de suite, enregistré avec la feuille un instant après (le
	// curseur et la molette en envoient beaucoup).
	// Affichage › Animations : préférence de l'utilisateur, gardée sur ce poste.
	let animate = $state(
		(() => {
			try {
				return localStorage.getItem('lattis.animations') !== 'off';
			} catch {
				return true;
			}
		})()
	);
	function setAnimate(on: boolean) {
		animate = on;
		try {
			localStorage.setItem('lattis.animations', on ? 'on' : 'off');
		} catch {
			// stockage indisponible : le choix vaut pour la session
		}
	}

	let zoomSet = $state<Record<string, number>>({});
	const docKey = $derived(`${mode}|${docState.open}|${docState.path}|${docState.title}`);
	const zoomKey = $derived(`${docKey}|${sheetId}`);
	const zoom = $derived(zoomSet[zoomKey] ?? clampZoom((activeInfo?.zoom || 100) / 100));
	$effect(() => {
		void docKey; // autre classeur : les zooms enregistrés font foi
		zoomSet = {};
	});
	let zoomTimer = 0;
	function setZoom(z: number) {
		const v = clampZoom(z);
		zoomSet = { ...zoomSet, [zoomKey]: v };
		if (mode !== 'doc') return;
		const id = sheetId;
		clearTimeout(zoomTimer);
		zoomTimer = window.setTimeout(() => {
			DocumentService.SetZoom(id, Math.round(v * 100))
				.then((st) => (docState = st))
				.catch((e) => (notice = err(e)));
		}, 400);
	}
	// Le contenu des feuilles n'est verrouillé par le classeur que si on l'a choisi en le protégeant.
	const bookLocked = $derived(mode === 'doc' && !!layout?.workbookProtected && !!layout?.workbookSheets);
	const sheetLocked = $derived(mode === 'doc' && (!!activeInfo?.protected || bookLocked));
	const lockedMsg = $derived(
		activeInfo?.protected
			? `La feuille « ${activeInfo?.name ?? ''} » est protégée : ôtez la protection (Révision › Ôter la protection de la feuille) pour la modifier`
			: bookLocked
				? 'Le classeur est protégé : ôtez la protection (Révision › Ôter la protection du classeur) pour le modifier'
				: ''
	);

	/** Protège, ou ôte la protection (mot de passe demandé s'il y en a un). */
	function toggleProtect(what: 'sheet' | 'workbook', id = sheetId) {
		if (mode !== 'doc' || !layout) return;
		const s = layout.sheets?.find((x) => x.id === id);
		const on = what === 'sheet' ? !!s?.protected : !!layout.workbookProtected;
		const hasPw = what === 'sheet' ? !!s?.password : !!layout.workbookPassword;
		const name = what === 'sheet' ? (s?.name ?? '') : docState.title;
		if (on && !hasPw) {
			applyProtect(what, id, name, true, '');
			return;
		}
		pw1 = pw2 = pwError = '';
		lockSheets = false;
		protecting = { what, id, name, off: on };
	}

	async function applyProtect(what: 'sheet' | 'workbook', id: number, name: string, off: boolean, pw: string, sheets = false) {
		try {
			if (what === 'sheet') await (off ? DocumentService.UnprotectSheet(id, pw) : DocumentService.ProtectSheet(id, pw));
			else await (off ? DocumentService.UnprotectWorkbook(pw) : DocumentService.ProtectWorkbook(pw, sheets));
			protecting = null;
			await reload();
			notice =
				what === 'sheet'
					? off
						? `Protection de la feuille « ${name} » ôtée`
						: `Feuille « ${name} » protégée`
					: off
						? 'Protection du classeur ôtée'
						: sheets
							? 'Classeur protégé : structure et contenu de toutes les feuilles'
							: 'Structure du classeur protégée';
		} catch (e) {
			if (protecting) pwError = err(e);
			else notice = err(e);
		}
	}

	function confirmProtect() {
		const p = protecting;
		if (!p) return;
		if (!p.off && pw1 !== pw2) {
			pwError = 'Les deux mots de passe ne correspondent pas';
			return;
		}
		applyProtect(p.what, p.id, p.name, p.off, pw1, p.what === 'workbook' && lockSheets);
	}

	/** Supprimer une feuille : tout son contenu part avec elle (un Ctrl+Z rend tout) ; demandé d'abord si elle n'est pas vide. */
	async function askDeleteSheet(id: number) {
		try {
			const l = await DocumentService.Layout(id);
			const name = l.sheets?.find((x) => x.id === id)?.name ?? '';
			const items: string[] = [];
			const count = (n: number, one: string, many: string, names?: string[]) => {
				if (!n) return;
				const list = names?.length ? ` (${names.slice(0, 5).join(', ')}${names.length > 5 ? '…' : ''})` : '';
				items.push((n > 1 ? `${n.toLocaleString('fr-FR')} ${many}` : `1 ${one}`) + list);
			};
			const tables = l.tables ?? [];
			count(tables.length, 'tableau', 'tableaux', tables.map((t) => `${t.name} : ${t.total.toLocaleString('fr-FR')} ligne${t.total > 1 ? 's' : ''}`));
			count(l.pivots?.length ?? 0, 'tableau croisé', 'tableaux croisés', l.pivots?.map((x) => x.config.name ?? ''));
			count(l.charts?.length ?? 0, 'graphique', 'graphiques');
			count(l.buttons?.length ?? 0, 'bouton', 'boutons');
			count(l.cells?.length ?? 0, 'cellule remplie', 'cellules remplies');
			if (!items.length) return sheetAction(() => DocumentService.DeleteSheet(id));
			deleting = { id, name, items };
		} catch (e) {
			notice = err(e);
		}
	}

	async function sheetAction(fn: () => Promise<unknown>) {
		try {
			await fn();
			await reload();
			if (visibleSheet(layout?.sheets ?? [], sheetId) !== sheetId) {
				sheetId = visibleSheet(layout?.sheets ?? [], sheetId);
				await reload();
			}
		} catch (e) {
			notice = err(e);
		}
	}
</script>

<svelte:window oncontextmenu={blockNativeMenu} />

<div class="app">
	<TitleBar
		{title}
		dirty={mode === 'doc' && docState.dirty}
		actions={[
			{ label: 'Enregistrer (Ctrl+S)', icon: 'save', run: mode === 'doc' ? () => file('save') : undefined },
			{ label: docState.undo ? `Annuler : ${docState.undo} (Ctrl+Z)` : 'Annuler (Ctrl+Z)', icon: 'undo', run: mode === 'doc' && docState.undo ? () => DocumentService.Undo().then(reload).catch((e) => (notice = err(e))) : undefined },
			{ label: docState.redo ? `Rétablir : ${docState.redo} (Ctrl+Y)` : 'Rétablir (Ctrl+Y)', icon: 'redo', run: mode === 'doc' && docState.redo ? () => DocumentService.Redo().then(reload).catch((e) => (notice = err(e))) : undefined }
		]}
	/>
	<Ribbon
		{view}
		disabled={mode !== 'doc' && mode !== 'demo'}
		{recent}
		onrecent={(p) => openPath(p).catch((e) => (notice = err(e)))}
		undo={mode === 'doc' ? docState.undo : ''}
		redo={mode === 'doc' ? docState.redo : ''}
		ai={aiChat.open}
		onai={() => (aiChat.open ? (aiChat.open = false) : openAssistant())}
		onview={(v) => {
			if (v !== view) sheet?.closeOverlays();
			view = v;
		}}
		onfile={file}
		onundo={() => DocumentService.Undo().then(reload).catch((e) => (notice = err(e)))}
		onredo={() => DocumentService.Redo().then(reload).catch((e) => (notice = err(e)))}
		onbench={mode === 'demo' ? () => sheet?.runBench() : undefined}
		onsort={mode === 'doc' ? (desc) => sheet?.sortActive(desc) : undefined}
		oncells={mode === 'doc' ? (op) => sheet?.cellsActive(op) : undefined}
		onstyle={mode === 'doc' ? (patch) => (activeDoc ? docPage?.applyStyle(patch) : sheet?.applyStyle(patch)) : undefined}
		onfind={mode === 'doc' ? (replace) => sheet?.openFinder(replace) : undefined}
		onrules={mode === 'doc' ? (x, y) => sheet?.rulesMenu(x, y) : undefined}
		onfilter={mode === 'doc' ? (w) => (w === 'clear' ? sheet?.editActive('clearFilters') : sheet?.filterActive(w === 'errors')) : undefined}
		filters={!!sheetStatus?.filters}
		errorsOnly={!!sheetStatus?.errorsOnly}
		note={!!sheetStatus?.note}
		onedit={mode === 'doc' ? (w) => sheet?.editActive(w) : undefined}
		tableReadOnly={sheetStatus?.readOnly ? `${sheetStatus.readOnly}${share?.shared && !share.canDesign && share.roles?.length ? ` (rôle : ${rolesLabel(share.roles)})` : ''}` : undefined}
		onaudit={mode === 'doc' ? (k) => sheet?.auditActive(k) : undefined}
		onpainter={mode === 'doc' ? () => sheet?.formatPainter() : undefined}
		onclip={mode === 'doc' ? (what) => (what === 'paste' ? sheet?.pasteClipboard() : sheet?.copySelection(what === 'cut')) : undefined}
		onchart={mode === 'doc' ? (kind) => sheet?.chartActive(kind) : undefined}
		onformula={mode === 'doc' ? (what) => (what === 'insert' ? sheet?.openFunctions() : what === 'sum' ? sheet?.autoSum() : sheet?.startFormula('=REFERENCE(')) : undefined}
		onpivot={mode === 'doc' ? () => sheet?.pivotActive() : undefined}
		oninsertdata={mode === 'doc' ? () => {
			const c = sheet?.activeCell() ?? { row: 0, col: 0 };
			dataInsert = { sheet: sheetId, ...c };
		} : undefined}
		ondedup={mode === 'doc' ? () => sheet?.dedupActive() : undefined}
		ontextcols={mode === 'doc' ? () => sheet?.textToColumnsActive() : undefined}
		onbutton={mode === 'doc' ? (kind) => sheet?.insertButton(kind) : undefined}
		context={activePage && view === 'grid' ? (activeDoc ? 'doc' : 'html') : 'sheet'}
		ondoc={docCommand}
		docState={activeDoc && docFmt ? docFmt : undefined}
		onrecalc={mode === 'doc' ? () => DocumentService.Recalc().then(reload).catch((e) => (notice = err(e))) : undefined}
		gridlines={!layout?.sheets?.find((s) => s.id === sheetId)?.noGrid}
		ongridlines={mode === 'doc' ? (show) => DocumentService.SetGridlines(sheetId, show).then(reload).catch((e) => (notice = err(e))) : undefined}
		headings={!!layout?.sheets?.find((s) => s.id === sheetId)?.headings}
		onheadings={mode === 'doc' ? (show) => DocumentService.SetHeadings(sheetId, show).then(reload).catch((e) => (notice = err(e))) : undefined}
		sheetProtected={sheetLocked}
		sheetOwnProtected={!!activeInfo?.protected}
		workbookProtected={mode === 'doc' && !!layout?.workbookProtected}
		onprotect={mode === 'doc' ? (what) => toggleProtect(what) : undefined}
		ondata={mode === 'doc' ? (what) => (what === 'refreshAll' ? refreshKey(true) : queryHost?.data(what)) : undefined}
		onformat={mode === 'doc' ? (tab) => sheet?.openFormat(tab) : undefined}
		onmerge={mode === 'doc' ? () => sheet?.mergeActive(true) : undefined}
		onwrap={mode === 'doc' ? () => sheet?.toggleWrap() : undefined}
		onlines={mode === 'doc' ? (rows, hide) => sheet?.hideLines(rows, hide) : undefined}
		onvalidation={mode === 'doc' ? () => sheet?.openValidation() : undefined}
		onnote={mode === 'doc' ? (what) => (what === 'edit' ? sheet?.editNote() : sheet?.deleteNote()) : undefined}
		freeze={activeInfo ? { rows: activeInfo.freezeRows, cols: activeInfo.freezeCols } : undefined}
		onfreeze={mode === 'doc' ? (what) => sheet?.freezeActive(what) : undefined}
		{zoom}
		{animate}
		onanimate={setAnimate}
		onzoom={data && view === 'grid' && !activePage ? setZoom : undefined}
		cellStyle={activeDoc ? docFmt?.style : sheetStatus?.style}
		merged={!!sheetStatus?.merged}
		shared={mode === 'doc' && !!share?.shared}
		republish={mode === 'doc' && canRepublish(share)}
		rights={canRights}
		pageSetup={layout?.pageSetup}
		onpage={mode === 'doc' ? pageCmd : undefined}
		pageBreaks={showBreaks}
		onpagebreaks={mode === 'doc' ? (on) => (showBreaks = on) : undefined}
	/>

	{#if mode === 'doc'}
		<ShareBanner {share} busy={shareReloading} onreload={reloadShare} onlogin={() => share && requestLogin(share.server)} onconflicts={() => (conflictsOpen = true)} />
	{/if}

	<!-- Une erreur d’un composant ne remplace plus tout par un écran 500 : elle est journalisée (pile, dernières actions) et l’on peut réessayer. -->
	<svelte:boundary onerror={(e) => void reportError(`interface (${mode})`, e)}>
	<main style:--dock-w={dockWidth ? `${dockWidth}px` : undefined}>
		{#if failure}
			<div class="splash"><h1>Lattis</h1><p class="error">{failure}</p></div>
		{:else if mode === 'recover'}
			<div class="splash">
				<h1>Récupération</h1>
				<p>Lattis s'est fermé sans enregistrer. Ces classeurs ont des modifications récupérables :</p>
				<div class="orphans">
				{#each allOrphans ? orphanGroups : orphanGroups.slice(0, ORPHANS_SHOWN) as g (g.key)}
					{@const o = g.latest}
					<div class="orphan">
						<div>
							<strong>{sessionName(o)}</strong>
							<span class="muted">modifié le {new Date(o.modified).toLocaleString('fr-FR')}</span>
							{#if g.older.length}
								<button class="link older-toggle" aria-expanded={!!olderShown[g.key]} onclick={() => (olderShown[g.key] = !olderShown[g.key])}>{olderLabel(g.older.length)}</button>
							{/if}
						</div>
						<button class="primary ico" onclick={() => recover(o, true)}><Icon name="undo" size={14} />Récupérer</button>
						<button class="ico discard" title={g.older.length ? 'Abandonner cette session et ses versions plus anciennes' : undefined} onclick={() => recover(o, false, g.older)}><Icon name="trash" size={14} />{g.older.length ? 'Tout abandonner' : 'Abandonner'}</button>
					</div>
					{#if olderShown[g.key]}
						{#each g.older as v (v.dir)}
							<div class="orphan older">
								<div>
									<span>{sessionName(v)}</span>
									<span class="muted">modifié le {new Date(v.modified).toLocaleString('fr-FR')}</span>
								</div>
								<button class="ico" onclick={() => recover(v, true)}><Icon name="undo" size={14} />Récupérer</button>
								<button class="ico discard" onclick={() => recover(v, false)}><Icon name="trash" size={14} />Abandonner</button>
							</div>
						{/each}
					{/if}
				{/each}
				</div>
				{#if !allOrphans && orphanGroups.length > ORPHANS_SHOWN}
					<button class="link" onclick={() => (allOrphans = true)}>Voir les {orphanGroups.length - ORPHANS_SHOWN} plus anciens</button>
				{/if}
				{#if recoverError}<p class="error">{recoverError}</p>{/if}
			</div>
		{:else if mode === 'closed'}
			<div class="splash">
				<h1>Lattis</h1>
				<p>Aucun classeur ouvert.</p>
				<div class="actions">
					<button class="primary ico" onclick={() => file('new')}><Icon name="new-file" size={14} />Nouveau classeur</button>
					<button class="ico" onclick={() => file('open')}><Icon name="open" size={14} />Ouvrir…</button>
					<button class="ico" onclick={() => file('import')} title="Crée un classeur avec les feuilles importées"><Icon name="database" size={14} />Importer (CSV, Excel)…</button>
					{#if sharedWorkbooksEnabled()}<button class="ico" onclick={() => file('openRemote')}><Icon name="web" size={14} />Ouvrir depuis un serveur…</button>{/if}
				</div>
				{#if showSteps}
					<div class="steps">
						<p class="muted">Premiers pas</p>
						<ol>
							{#each firstSteps as s (s.help)}
								<li><button class="link" title="Ouvrir l’aide" onclick={() => openHelp(s.help)}>{s.title}</button> : {s.text}</li>
							{/each}
						</ol>
						<div class="steps-actions">
							<button class="ico" onclick={() => openSample().catch((e) => (notice = err(e)))}><Icon name="book" size={14} />Ouvrir le classeur d’exemple</button>
							<button
								class="link"
								onclick={() => {
									hideSteps();
									showSteps = false;
								}}>Ne plus afficher</button
							>
						</div>
					</div>
				{:else}
					<button class="link" onclick={() => openSample().catch((e) => (notice = err(e)))}>Ouvrir le classeur d’exemple</button>
				{/if}
				{#if recent.length}
					<div class="recent">
						<p class="muted">Récents</p>
						{#each recent as p (p)}
							<button class="link" title={p} onclick={() => openPath(p).catch((e) => (notice = err(e)))}>{p.split(/[\\/]/).pop()}</button>
						{/each}
					</div>
				{/if}
			</div>
		{:else if data}
			{#key mode}
				<div class="pane" data-region="grid" class:hidden={view !== 'grid' || !!activePage}>
					<Sheet
						bind:this={sheet}
						{data}
						source={mode === 'demo' ? demoSource : docSource}
						sources={(help?.tables ?? []).map((t) => ({ name: t.name, columns: t.columns ?? [], types: t.types ?? [] }))}
						catalog={mode === 'doc' && help
							? { functions: help.functions ?? [], tables: (help.tables ?? []).filter((t) => t.kind !== 'pivot'), names: (layout?.names ?? []).map((n) => n.name) }
							: undefined}
						onstatus={(s) => (sheetStatus = s)}
						onhscroll={(s) => (hscroll = s)}
						gridlines={!layout?.sheets?.find((s) => s.id === sheetId)?.noGrid}
						headings={!!layout?.sheets?.find((s) => s.id === sheetId)?.headings}
						locked={lockedMsg}
						freeze={activeInfo ? { rows: activeInfo.freezeRows, cols: activeInfo.freezeCols } : undefined}
						{zoom}
						{animate}
						onzoom={setZoom}
						autoBench={bench ? (r) => GridService.ReportBench(r) : undefined}
						sheetName={activeInfo?.name ?? ''}
						onsheet={(id) => selectSheet(id).catch((e) => (notice = err(e)))}
						onrowlink={mode === 'doc' ? gotoRow : undefined}
						onsharedata={mode === 'doc' && sharedWorkbooksEnabled() ? (t) => (dataShare = t) : undefined}
						{sourceTables}
						onpushsource={mode === 'doc' && sharedWorkbooksEnabled() ? (t) => void pushSource(t) : undefined}
						onask={mode === 'doc' ? openAssistant : undefined}
						pageBreaks={breaks && !breaks.empty ? { rows: breaks.rows ?? [], cols: breaks.cols ?? [], manualRows: breaks.manualRows ?? [], manualCols: breaks.manualCols ?? [], area: breaks.area, framed: !!layout?.pageSetup?.area } : undefined}
					/>
				</div>
			{/key}
			{#if view === 'grid' && activePage}
				{#key activePage.sheet}
					{@const pageId = activePage.sheet}
					<div class="pane" data-region="grid">
						{#if activeDoc}
						<DocPage
							bind:this={docPage}
							name={activeInfo?.name ?? ''}
							source={activePage.source}
							save={async (src) => {
								docState = await DocumentService.SavePage(pageId, src);
								await reload();
							}}
							locked={lockedMsg}
							stamp={layout}
							{animate}
							tables={(help?.tables ?? []).map((t) => ({ name: t.name, columns: t.columns ?? [], types: t.types ?? [], kind: t.kind }))}
							{scripts}
							loadChart={(cfg) => DocumentService.ChartData(cfg as never)}
							loadPivot={(cfg) => DocumentService.PivotData(cfg as never)}
							fieldValues={async (table, field) => (await DocumentService.FieldValues(table, field)) ?? []}
							sheets={(layout?.sheets ?? []).filter((s) => !s.kind).map((s) => s.name)}
							controls={{
								load: (id) => DocumentService.DocControl(id) as never,
								create: async (cfg) => {
									const id = await DocumentService.CreateButton(pageId, 0, 0, cfg as never);
									await reload();
									return id;
								},
								update: async (id, cfg) => {
									docState = await DocumentService.SetButton(id, cfg as never);
									await reload();
								},
								setValue: async (id, raw) => {
									const r = await DocumentService.SetControlValue(id, raw);
									await reload();
									if (r.error) notice = `Erreur du script : ${r.error}`;
									else if (r.output?.length) notice = r.output[r.output.length - 1];
								},
								click: async (id) => {
									const r = await DocumentService.RunButton(id);
									await reload();
									if (r.error) notice = `Erreur du script : ${r.error}`;
									else if (r.output?.length) notice = r.output[r.output.length - 1];
								},
								pickFile: (title) => DocumentService.PickControlFile(title)
							}}
							run={async (script, fn) => {
								const r = await DocumentService.RunFunction(script, fn);
								await reload();
								if (r.error) return `Erreur du script : ${r.error}`;
								const out = r.output ?? [];
								return out.length ? out[out.length - 1] : '';
							}}
							onformat={(f) => (docFmt = f)}
							onnotice={(m) => (notice = m)}
							onrowlink={gotoRow}
						/>
						{:else}
						<HtmlPage
							name={activeInfo?.name ?? ''}
							source={activePage.source}
							save={async (src) => {
								docState = await DocumentService.SavePage(pageId, src);
								await reload();
							}}
							call={(name, args) => DocumentService.PageCall(name, args)}
							trusted={(src) => DocumentService.PageAllowed(src)}
							allow={(src) => DocumentService.AllowPage(pageId, src)}
							locked={lockedMsg}
							stamp={layout}
							onnotice={(m) => (notice = m)}
						/>
						{/if}
					</div>
				{/key}
			{/if}
			{#if view === 'sql'}
				<div class="pane" data-region="grid"><SqlView {objects} data={dataGroups} rundata={mode === 'doc' ? (server, q) => DocumentService.QueryData(server, q) : undefined} rights={rightsObjs} onrights={canRights && share?.canManage ? () => (rightsOpen = true) : undefined} run={query} exec={mode === 'doc' ? (q) => DocumentService.Execute(q) : undefined} stop={mode === 'doc' ? () => void DocumentService.StopSQL() : undefined} ongoto={gotoObject}
						onrename={mode === 'doc'
							? async (from, to) => {
									docState = await DocumentService.RenameTable(from, to);
									await reload();
									notice = `Tableau « ${from} » renommé en « ${to} » (Ctrl+Z pour annuler)`;
								}
							: undefined}
						tosheet={mode === 'doc'
							? async (q, name, headers, link, columns) => {
									if (!link) {
										const r = await DocumentService.QueryToSheet(q, name, headers);
										notice = `Résultat copié dans le tableau « ${r.table} » (Ctrl+Z pour annuler)`;
										await selectSheet(r.sheet);
										return;
									}
									const r = await DocumentService.ReferenceQuery(q, name, columns, headers);
									if (r.errors?.length) throw new Error(r.errors.join('\n'));
									const l = r.loads?.[0];
									if (!l) return;
									notice = `Tableau « ${l.table} » lié à la requête : clic droit › Actualiser pour le relire`;
									await reload();
									await selectSheet(l.sheet);
								}
							: undefined}
						favorites={mode === 'doc'
							? {
									list: favorites,
									add: async (name, q) => {
										const id = await DocumentService.AddFavorite(name, q);
										await reload();
										return id;
									},
									rename: (id, name) => DocumentService.RenameFavorite(id, name).then(reload),
									remove: (id) => DocumentService.DeleteFavorite(id).then(reload)
								}
							: undefined}
					/></div>
			{:else if view === 'script'}
				<div class="pane" data-region="grid">
					{#if mode === 'doc'}
						<ScriptView
							{scripts}
							save={async (id, name, source) => {
								const nid = await DocumentService.SaveScript(id, name, source);
								await reload();
								return nid;
							}}
							remove={(id) => DocumentService.DeleteScript(id).then(reload)}
							run={async (name, source) => {
								const r = await DocumentService.RunScript(name, source);
								if (r.writes) await reload();
								return r;
							}}
							stop={() => DocumentService.StopScript()}
							controls={() => DocumentService.Call('controls.list', {}) as Promise<ControlInfo[]>}
							showSheet={(on) => (view = on ? 'grid' : 'script')}
							subscribe={(fn) => Events.On('script:print', (e: { data: string }) => fn(e.data))}
							network={{
								list: async () => (await DocumentService.HTTPHosts()) ?? [],
								allow: async (h) => {
									const l = (await DocumentService.AllowHTTPHost(h)) ?? [];
									docState = await DocumentService.State();
									return l;
								},
								forget: async (h) => {
									const l = (await DocumentService.ForgetHTTPHost(h)) ?? [];
									docState = await DocumentService.State();
									return l;
								}
							}}
						/>
					{:else}
						<p class="placeholder">Les scripts s'exécutent sur un classeur : Fichier › Nouveau.</p>
					{/if}
				</div>
			{/if}
		{:else}
			<div class="splash">
				<h1>Lattis</h1>
				<p>{demoProgress || 'Ouverture…'}</p>
			</div>
		{/if}
		<!-- Zone des volets du classeur : côte à côte, à droite de la vue, la somme de leurs largeurs bornée (resize.ts). -->
		<div class="dock" bind:offsetWidth={dockWidth}>
			{#if helpState.open && !helpState.over}
				<HelpPanel />
			{/if}
			{#if mode === 'doc'}
				<QueryHost
					bind:this={queryHost}
					onchanged={(sh) => (sh && sh !== sheetId ? selectSheet(sh) : reload()).catch((e) => (notice = err(e)))}
					onnotice={(m) => (notice = m)}
				/>
			{/if}
			{#if mode === 'doc' && conflictsOpen && share?.mode === 'collaborative'}
				<ConflictPanel {conflicts} busy={conflictBusy} onresolve={resolveConflict} onresolveall={resolveAllConflicts} ongoto={gotoConflict} onclose={() => (conflictsOpen = false)} />
			{/if}
			{#if aiChat.open && (mode === 'doc' || mode === 'closed')}
				<AiPanel
					hasDoc={mode === 'doc'}
					context={aiContext}
					tables={aiTables}
					onsettings={() => (settingsOpen = true)}
					ongoto={(t) => gotoTarget(t).catch((e) => (notice = err(e)))}
					onchanged={reload}
					onclose={() => (aiChat.open = false)}
				/>
			{/if}
		</div>
	</main>
	<!-- Aide ouverte depuis une boîte modale (F1) : au-dessus d'elle, et utilisable. -->
	{#if helpState.open && helpState.over}
		<HelpPanel over />
	{/if}
		{#snippet failed(error, reset)}
			<main>
				<div class="splash">
					<h1>Erreur de l’interface</h1>
					<p class="error">{describe(error).message}</p>
					<p class="muted">Le détail (pile, dernières actions) est dans le journal des erreurs. Le classeur n’est pas perdu.</p>
					<div class="actions">
						<button class="primary" onclick={reset}>Réessayer</button>
						<button onclick={() => location.reload()}>Recharger l’interface</button>
					</div>
				</div>
			</main>
		{/snippet}
	</svelte:boundary>

	<SheetTabs
		sheets={tabs}
		active={mode === 'demo' ? 1 : sheetId}
		onselect={selectSheet}
		onadd={mode === 'doc' ? () => sheetAction(async () => selectSheet(await DocumentService.AddSheet())) : undefined}
		oninsert={mode === 'doc' ? (before) => sheetAction(async () => selectSheet(await DocumentService.InsertSheet(before))) : undefined}
		onpage={mode === 'doc' && htmlTabsEnabled() ? (before) => addPage(before) : undefined}
		ondocpage={mode === 'doc' ? (before) => addDoc(before) : undefined}
		onmove={mode === 'doc' ? (id, to) => sheetAction(() => DocumentService.MoveSheet(id, to)) : undefined}
		onrename={mode === 'doc' ? async (id, name) => {
			await DocumentService.RenameSheet(id, name);
			await reload();
		} : undefined}
		ondelete={mode === 'doc' ? (id) => askDeleteSheet(id) : undefined}
		ondeletemany={mode === 'doc' ? (ids) => (deletingMany = ids) : undefined}
		onorder={mode === 'doc' ? (ids) => sheetAction(() => DocumentService.OrderSheets(ids)) : undefined}
		oncopy={mode === 'doc' ? (id, to) => sheetAction(async () => selectSheet(await DocumentService.CopySheet(id, to))) : undefined}
		oncolor={mode === 'doc' ? (id, color) => sheetAction(() => DocumentService.SetSheetColor(id, color)) : undefined}
		oncolormany={mode === 'doc' ? (ids, color) => sheetAction(() => DocumentService.SetSheetsColor(ids, color)) : undefined}
		onhidemany={mode === 'doc' ? (ids) => sheetAction(() => DocumentService.SetSheetsHidden(ids, true)) : undefined}
		onhide={mode === 'doc' ? (id, hidden) => sheetAction(async () => {
			await DocumentService.SetSheetHidden(id, hidden);
			if (!hidden) await selectSheet(id);
		}) : undefined}
		hscroll={view === 'grid' && !activePage ? hscroll : undefined}
		onhscroll={(x, atEnd) => sheet?.scrollLeftTo(x, atEnd)}
		onhhold={(on) => sheet?.holdHScroll(on)}
		onprotect={mode === 'doc' ? (id) => toggleProtect('sheet', id) : undefined}
		locked={mode === 'doc' && !!layout?.workbookProtected}
		onrefused={(m) => (notice = m)}
		onchoice={mode === 'doc' ? choiceActions : undefined}
	/>
	{#each dialogs.slice(0, 1) as d (d.id)}
		<ScriptDialog spec={d} onanswer={(v) => answerDialog(d, v)} />
	{/each}
	{#if mcpOpen && mcpAvailable()}
		<McpPanel onclose={() => (mcpOpen = false)} />
	{/if}
	<!-- Accord demandé avant une écriture de Claude Desktop / Code (mcp.confirmWrites). -->
	<McpConfirm />
	{#if aboutOpen}
		<AboutDialog onclose={() => (aboutOpen = false)} />
	{/if}
	{#if exportOpen && exported}
		<ExportReportDialog report={exported.report} format={exported.format} onclose={() => (exportOpen = false)} />
	{/if}
	{#if pageDlg && mode === 'doc' && layout}
		<PageSetupDialog
			setup={layout.pageSetup}
			sheet={activeInfo?.name ?? ''}
			file={docState.path ? (docState.path.split(/[\\/]/).pop() ?? '') : docState.title}
			selection={sheet?.selectedRange()}
			tab={pageDlg}
			onapply={(p) => {
				pageDlg = null;
				void savePage(p, 'Mise en page');
			}}
			onprint={(p) => {
				pageDlg = null;
				void savePage(p, 'Mise en page').then(() => file('print'));
			}}
			onclose={() => (pageDlg = null)}
		/>
	{/if}
	{#if settingsOpen}
		<SettingsDialog onclose={() => (settingsOpen = false)} onmcp={() => { settingsOpen = false; mcpOpen = true; }} />
	{/if}
	{#if rightsOpen && canRights && share}
		<RightsDialog title={share.title} server={share.server} link={share.link} onclose={() => (rightsOpen = false)} />
	{/if}
	{#if shareDlg && mode === 'doc'}
		<ShareDialog
			republish={shareDlg.republish}
			{share}
			title={docState.title}
			ondone={(st) => {
				const again = shareDlg?.republish;
				shareDlg = null;
				// Ouvert depuis Partager au Data Lake : la boîte revient, le tableau publié.
				const back = dataShareAfter;
				dataShareAfter = null;
				if (!st) return;
				if (back) dataShare = back;
				share = st;
				notice = again ? `Version ${st.version} publiée` : `Classeur publié sur ${st.serverName || st.server} (${st.mode === 'frozen' ? 'figé' : 'collaboratif'})`;
				reload().catch((e) => (notice = err(e)));
			}}
		/>
	{/if}
	{#if dataInsert}
		{@const at = dataInsert}
		<DataCatalogDialog
			sheet={at.sheet}
			row={at.row}
			col={at.col}
			ondone={(ok) => {
				dataInsert = null;
				if (ok) reload().catch((e) => (notice = err(e)));
			}}
		/>
	{/if}
	{#if dataShare}
		{@const t = dataShare}
		<DataShareDialog
			table={t}
			ondone={() => {
				dataShare = null;
				loadSources();
			}}
			onpublish={(republish) => {
				dataShareAfter = t;
				shareDlg = { republish };
			}}
			onnotice={(msg) => {
				notice = msg;
				reload().catch((e) => (notice = err(e)));
			}}
		/>
	{/if}
	{#if openRemote}
		<OpenRemoteDialog
			ondone={(r) => {
				openRemote = false;
				if (r) openedRemote(r).catch((e) => (notice = err(e)));
			}}
		/>
	{/if}
	<LattisHost />
	{#if linkAsk}
		{@const a = linkAsk}
		<div class="modal-back">
			<div class="modal" role="dialog" aria-modal="true" aria-label="Ouvrir un lien Lattis" use:modal={() => a.ok(false)}>
				<p>Ouvrir un classeur de « {a.name} » ?</p>
				<p class="muted">Lattis ne s’est jamais connecté à {a.host}. N’acceptez que si vous connaissez ce serveur et attendiez ce lien.</p>
				<div class="actions">
					<button class="primary" onclick={() => a.ok(true)}>Se connecter et ouvrir</button>
					<button onclick={() => a.ok(false)}>Annuler</button>
				</div>
			</div>
		</div>
	{/if}
	{#if protecting}
		{@const p = protecting}
		<div class="modal-back">
			<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
			<form
				class="modal protect"
				use:modal
				aria-label="Protection"
				onkeydown={(e) => {
					e.stopPropagation();
					if (e.key === 'Escape') protecting = null;
				}}
				onsubmit={(e) => {
					e.preventDefault();
					confirmProtect();
				}}
			>
				<p>
					{#if p.what === 'sheet'}
						{p.off ? `Ôter la protection de la feuille « ${p.name} »` : `Protéger la feuille « ${p.name} »`}
					{:else}
						{p.off ? 'Ôter la protection du classeur' : 'Protéger la structure du classeur'}
					{/if}
				</p>
				{#if !p.off}
					<p class="muted">
						{p.what === 'sheet'
							? 'Saisie, collage, mise en forme, lignes et colonnes, tri, objets et requêtes SQL d’écriture seront refusés sur cette feuille. Lecture, copie et filtres restent permis.'
							: lockSheets
								? 'Ajouter, supprimer, renommer ou déplacer des feuilles sera refusé, ainsi que toute modification du contenu des feuilles.'
								: 'Ajouter, supprimer, renommer ou déplacer des feuilles sera refusé. Le contenu des feuilles non protégées reste modifiable.'}
					</p>
					<p class="muted">
						Cette protection évite les modifications accidentelles ; ce n’est pas une sécurité : le fichier n’est pas chiffré, ses données restent lisibles et la
						protection peut être retirée par quelqu’un d’outillé.
					</p>
					{#if p.what === 'workbook'}
						<label class="check">
							<input type="checkbox" bind:checked={lockSheets} />
							Verrouiller aussi le contenu de toutes les feuilles
						</label>
					{/if}
				{/if}
				<label>
					Mot de passe{p.off ? '' : ' (facultatif)'}
					<!-- svelte-ignore a11y_autofocus -->
					<input type="password" bind:value={pw1} autofocus autocomplete="off" />
				</label>
				{#if !p.off}
					<label>
						Confirmer le mot de passe
						<input type="password" bind:value={pw2} autocomplete="off" />
					</label>
					<p class="muted">Sans le mot de passe, la protection ne pourra plus être ôtée : notez-le.</p>
				{/if}
				{#if pwError}<p class="error">{pwError}</p>{/if}
				<div class="actions">
					<button type="submit" class="primary">OK</button>
					<button type="button" onclick={() => (protecting = null)}>Annuler</button>
				</div>
			</form>
		</div>
	{/if}
	{#if deletingMany}
		{@const ids = deletingMany}
		<div class="modal-back">
			<div class="modal" role="dialog" aria-modal="true" aria-label="Supprimer les feuilles" use:modal={() => (deletingMany = null)}>
				<p>Supprimer ces {ids.length} feuilles et tout ce qu'elles contiennent ?</p>
				<ul class="delete-list">
					{#each ids as id (id)}<li>{layout?.sheets?.find((s) => s.id === id)?.name ?? id}</li>{/each}
				</ul>
				<p class="hint">Ctrl+Z les rend toutes, avec leur contenu.</p>
				<div class="actions">
					<button
						class="primary danger"
						onclick={() => {
							deletingMany = null;
							sheetAction(() => DocumentService.DeleteSheets(ids));
						}}>Supprimer</button
					>
					<button onclick={() => (deletingMany = null)}>Annuler</button>
				</div>
			</div>
		</div>
	{/if}
	{#if deleting}
		{@const d = deleting}
		<div class="modal-back">
			<div class="modal" role="dialog" aria-modal="true" aria-label="Supprimer la feuille" use:modal={() => (deleting = null)}>
				<p>Supprimer la feuille « {d.name} » et tout ce qu'elle contient ?</p>
				<ul class="delete-list">
					{#each d.items as it (it)}<li>{it}</li>{/each}
				</ul>
				<p class="hint">Ctrl+Z rend la feuille et tout son contenu.</p>
				<div class="actions">
					<button
						class="primary danger"
						onclick={() => {
							const id = d.id;
							deleting = null;
							sheetAction(() => DocumentService.DeleteSheet(id));
						}}>Supprimer</button
					>
					<button onclick={() => (deleting = null)}>Annuler</button>
				</div>
			</div>
		</div>
	{/if}
	{#if closing}
		<div class="modal-back">
			<div class="modal" role="dialog" aria-modal="true" aria-label="Enregistrer les modifications" use:modal={cancelClose}>
				<p>Enregistrer les modifications de « {docState.title} » avant {quitting ? 'de quitter Lattis' : 'de le fermer'} ?</p>
				<div class="actions">
					<button class="primary" onclick={() => closeDoc(true).catch((e) => ((quitting = false), (notice = err(e))))}>Enregistrer</button>
					<button onclick={() => closeDoc(false).catch((e) => ((quitting = false), (notice = err(e))))}>Ne pas enregistrer</button>
					<button onclick={cancelClose}>Annuler</button>
				</div>
			</div>
		</div>
	{/if}
	{#if damaged}
		{@const file = damaged}
		<div class="modal-back">
			<div class="modal" role="dialog" aria-modal="true" aria-label="Classeur endommagé" use:modal={() => salvage(null).catch((e) => (failure = err(e)))}>
				<p>{file.message}.</p>
				<p>Lattis peut en récupérer les données lisibles dans un nouveau classeur sans titre. Le fichier endommagé n'est pas modifié.</p>
				<div class="actions">
					<button class="primary" onclick={() => salvage(file.path).catch((e) => (notice = err(e)))}>Récupérer les données</button>
					<button onclick={() => salvage(null).catch((e) => (failure = err(e)))}>Annuler</button>
				</div>
			</div>
		</div>
	{/if}
	{#if locked}
		{@const file = locked}
		<div class="modal-back">
			<div class="modal" role="dialog" aria-modal="true" aria-label="Classeur déjà ouvert" use:modal={() => (locked = null)}>
				<p>{file.message}.</p>
				<p class="hint">En lecture seule, vos modifications s'enregistrent sous un autre nom. L'ouvrir quand même risque d'écraser les modifications de l'autre poste (Lattis vérifie le fichier avant d'enregistrer).</p>
				<div class="actions">
					<button class="primary" onclick={() => openLocked(file.path, true).catch((e) => (notice = err(e)))}>Ouvrir en lecture seule</button>
					<button onclick={() => openLocked(file.path, false).catch((e) => (notice = err(e)))}>Ouvrir quand même</button>
					<button onclick={() => (locked = null)}>Annuler</button>
				</div>
			</div>
		</div>
	{/if}
	{#if conflict}
		<div class="modal-back">
			<div class="modal" role="dialog" aria-modal="true" aria-label="Classeur modifié par ailleurs" use:modal={() => resolveSave(null)}>
				<p>{conflict.message}.</p>
				<p class="hint">« Enregistrer une copie » garde les deux versions ; « Écraser » remplace le fichier par la vôtre.</p>
				<div class="actions">
					<button class="primary" onclick={() => resolveSave(false).catch((e) => ((quitting = false), (notice = err(e))))}>Enregistrer une copie</button>
					<button onclick={() => resolveSave(true).catch((e) => ((quitting = false), (notice = err(e))))}>Écraser</button>
					<button onclick={() => resolveSave(null)}>Annuler</button>
				</div>
			</div>
		</div>
	{/if}
	<footer class="statusbar" data-region="status" aria-label="Barre d'état">
		{#if notice || sheetStatus?.message}
			{@const msg = sheetStatus?.message || notice}
			<!-- Deux niveaux : compte rendu de l'action (vert clair), avertissement (bulle orange). -->
			<span class="message" class:warn={isProblem(msg)} role={isProblem(msg) ? 'alert' : 'status'}>{msg}</span>
			{#if exported && msg === exported.notice}
				<button class="details" onclick={() => (exportOpen = true)}>Détails</button>
			{/if}
		{/if}
		<span class="spacer"></span>
		{#if sheetStatus}
			{#if sheetStatus.stats}<span class="stats">{sheetStatus.stats}</span>{/if}
			{#if import.meta.env.DEV}
				<!-- Mesures de développement : absentes de la version livrée. -->
				<span>{sheetStatus.fetch}</span>
				<span>{sheetStatus.draw}</span>
				<span>{sheetStatus.fps}</span>
				<span>{sheetStatus.edit}</span>
			{/if}
		{:else}
			<span>Prêt</span>
		{/if}
		<BusyBar />
		<PickBanner />
		{#if data && view === 'grid' && !activePage}<ZoomBar {zoom} onzoom={setZoom} />{/if}
	</footer>
</div>

<style>
	.delete-list {
		margin: 6px 0 4px;
		padding-left: 20px;
		font-size: 12px;
		line-height: 1.5;
	}
	.hint {
		margin: 4px 0 0;
		font-size: 12px;
		color: #5f6368;
	}
	.modal .primary.danger {
		background: #c0392b;
		border-color: #c0392b;
	}
	/* Coins des surfaces flottantes (menus, boîtes de dialogue, panneaux de filtre…) : légèrement arrondis. */
	:global(:root) {
		--float-radius: 6px;
		--focus-ring: #0f6cbd;
		/* Empilement : volets, boîtes ancrées et menus, boîtes modales (au-dessus
		   des couches de CodeMirror, jusqu'à 500 ; une boîte ouverte depuis une
		   autre ajoute 10 par niveau), aide ouverte depuis une boîte, demandes
		   d'accord, messages. */
		--z-panel: 20;
		--z-popover: 40;
		--z-dialog: 600;
		--z-help: 700;
		--z-confirm: 800;
		--z-toast: 1000;
		/* Fond des boîtes modales. */
		--backdrop: rgba(0, 0, 0, 0.25);
	}
	:global(html, body) {
		margin: 0;
		height: 100%;
		background: #fff;
		overflow: hidden;
	}
	.app :global(input),
	.app :global(textarea),
	.app :global(.cm-editor),
	.app :global(pre) {
		user-select: text;
	}
	/* Focus clavier visible partout (boutons, entrées de menu, liens,
	   éléments atteignables) ; les composants qui ont leur propre marque
	   (bordure d'un champ, fond d'une entrée) la gardent. */
	:global(:is(button, a[href], summary, [role^='menuitem'], [role='option'], [role='tab'], [tabindex='0']):focus-visible) {
		outline: 2px solid var(--focus-ring);
		outline-offset: -2px;
	}
	:global([role='grid']:focus-visible) {
		outline: none; /* la grille montre sa cellule active */
	}
	.app {
		user-select: none;
		display: flex;
		flex-direction: column;
		height: 100vh;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	main {
		flex: 1;
		min-height: 0;
		position: relative;
	}
	.pane {
		position: absolute;
		inset: 0;
		/* Volets du classeur ouverts : la vue se loge à leur gauche. */
		right: var(--dock-w, 0px);
	}
	.dock {
		position: absolute;
		top: 0;
		right: 0;
		bottom: 0;
		display: flex;
	}
	.hidden {
		visibility: hidden;
	}
	.placeholder {
		padding: 24px;
		color: #4b5563;
	}
	.statusbar {
		display: flex;
		align-items: center;
		gap: 14px;
		height: 24px;
		padding: 0 10px;
		background: #217346;
		color: #e8f3ec;
		font-size: 12px;
		white-space: nowrap;
		overflow: hidden;
	}
	.message {
		color: #b6ecc8;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.details {
		flex: none;
		background: none;
		border: none;
		padding: 0;
		margin-left: -8px;
		color: #fff;
		font-size: 12px;
		text-decoration: underline;
		cursor: pointer;
	}
	.message.warn {
		background: #ffb02e;
		color: #3d2600;
		font-weight: 600;
		padding: 1px 8px;
		border-radius: 3px;
		box-shadow: 0 0 0 1px #e08a00;
	}
	.spacer {
		flex: 1;
	}
	.stats {
		color: #fff;
		font-weight: 600;
		margin-right: 12px;
	}
	.splash {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
		gap: 10px;
	}
	h1 {
		font-weight: 600;
		color: #217346;
		margin: 0 0 8px;
	}
	/* Beaucoup de récupérations : la liste défile, l'écran garde son titre. */
	.orphans {
		display: flex;
		flex-direction: column;
		gap: 10px;
		max-height: 55vh;
		overflow-y: auto;
	}
	.orphan {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 8px 14px;
		border: 1px solid #d4d7db;
		border-radius: 4px;
		min-width: 460px;
	}
	.orphan div {
		flex: 1;
		display: flex;
		flex-direction: column;
	}
	/* Versions plus anciennes d'un classeur, dépliées sous sa session la plus récente. */
	.orphan.older {
		margin-left: 24px;
		min-width: 436px;
		padding: 4px 14px;
		border-style: dashed;
	}
	.orphan .older-toggle {
		align-self: flex-start;
		padding: 2px 0;
		font-size: 12px;
	}
	button {
		font: inherit;
		padding: 4px 12px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	button.primary {
		background: #217346;
		color: #fff;
		border-color: #217346;
		padding-left: 15px;
		padding-right: 15px;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		justify-content: center;
	}
	/* Boutons de l'accueil : icône devant le libellé. */
	.ico {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		--icon-accent: #217346;
	}
	.ico.primary {
		--icon-accent: currentColor;
	}
	.ico.discard {
		--icon-accent: #c0392b;
	}
	.recent {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 2px;
		margin-top: 12px;
	}
	.recent .muted {
		margin: 0 0 4px;
	}
	/* Accueil : premiers pas et classeur d'exemple. */
	.steps {
		max-width: 520px;
		margin-top: 12px;
		padding: 12px 16px;
		border: 1px solid #d6e4dc;
		border-radius: 6px;
		background: #f6faf7;
	}
	.steps .muted {
		margin: 0 0 6px;
	}
	.steps ol {
		margin: 0 0 10px;
		padding-left: 20px;
		line-height: 1.5;
	}
	.steps ol .link {
		padding: 0;
		font-weight: 600;
	}
	.steps-actions {
		display: flex;
		align-items: center;
		gap: 12px;
	}
	button.link {
		border: none;
		background: none;
		color: #217346;
		padding: 2px 8px;
	}
	button.link:hover {
		text-decoration: underline;
	}
	.modal-back {
		position: fixed;
		inset: 0;
		z-index: var(--z-dialog);
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--backdrop);
	}
	.modal {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		padding: 16px 20px;
		max-width: 440px;
	}
	.modal p {
		margin: 0 0 14px;
	}
	.modal .actions {
		justify-content: flex-end;
	}
	.modal.protect {
		min-width: 340px;
	}
	.modal.protect label {
		display: flex;
		flex-direction: column;
		gap: 4px;
		margin-bottom: 10px;
	}
	.modal.protect input {
		font: inherit;
		padding: 4px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	.modal.protect input:focus {
		outline: none;
		border-color: #217346;
	}
	.modal.protect .muted {
		margin-top: -6px;
	}
	.modal.protect label.check {
		flex-direction: row;
		align-items: center;
		gap: 6px;
	}
	.error {
		color: #b42318;
	}
</style>
