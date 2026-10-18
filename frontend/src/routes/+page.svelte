<script lang="ts">
	// Classeur façon Excel : ruban en haut, feuille (ou onglet SQL / SCRIPT) au
	// centre, onglets de feuilles et barre d'état en bas.
	//
	// Deux modes : un classeur (DocumentService : saisie libre, tableaux qui
	// naissent, annuler, enregistrer) et la démo d'1 M de lignes (GridService),
	// qui sert aussi à la mesure automatique (LATTIS_BENCH).
	import { onMount, tick, untrack } from 'svelte';
	import { noticeTtl } from '#lib/shell/notice.js';
	import { Events } from '@wailsio/runtime';
	import { DocumentService, GridService } from '#bindings/internal/bridge/index.js';
	import type { DamagedFile, DocState, FormulaHelp } from '#bindings/internal/bridge/models.js';
	import type { Layout, RuleChanges } from '#bindings/internal/canvas/models.js';
	import type { Type } from '#bindings/internal/grid/models.js';
	import type { Orphan } from '#bindings/internal/archive/models.js';
	import Sheet, { type SheetData, type SheetSource, type SheetStatus, type ColumnFilter, type PivotConfig, type ChartConfig, type TypeMix } from '#lib/grid/Sheet.svelte';
	import Ribbon, { type DocCommand, type FileAction, type View } from '#lib/shell/Ribbon.svelte';
	import SheetTabs from '#lib/shell/SheetTabs.svelte';
	import { visibleSheet } from '#lib/shell/sheets.js';
	import { count } from '#lib/sql/result.js';
	import { printSheet } from '#lib/shell/print.js';
	import SqlView, { type Favorite } from '#lib/shell/SqlView.svelte';
	import AiView from '#lib/ai/AiView.svelte';
	import TitleBar from '#lib/shell/TitleBar.svelte';
	import ScriptView, { type Script } from '#lib/script/ScriptView.svelte';
	import ScriptDialog, { type DialogSpec } from '#lib/script/ScriptDialog.svelte';
	import PickBanner from '#lib/script/PickBanner.svelte';
	import type { ControlInfo } from '#lib/script/controls.js';
	import McpPanel from '#lib/shell/McpPanel.svelte';
	import SettingsDialog from '#lib/shell/SettingsDialog.svelte';
	import { aiEnabled, htmlTabsEnabled, loadSettings } from '#lib/shell/settings.svelte.js';
	import { isWeb } from '#lib/web/platform.js';
	import HtmlPage from '#lib/page/HtmlPage.svelte';
	import DocPage from '#lib/doc/DocPage.svelte';
	import type { DocFormat } from '#lib/doc/format.js';
	import QueryHost from '#lib/query/QueryHost.svelte';
	import LattisHost from '#lib/remote/LattisHost.svelte';
	import ShareDialog from '#lib/remote/ShareDialog.svelte';
	import OpenRemoteDialog from '#lib/remote/OpenRemoteDialog.svelte';
	import ShareBanner from '#lib/remote/ShareBanner.svelte';
	import ConflictPanel from '#lib/remote/ConflictPanel.svelte';
	import RightsDialog from '#lib/remote/RightsDialog.svelte';
	import { rightsObjects, type RightsObject } from '#lib/remote/rights.js';
	import { refusedNotice } from '#lib/remote/share.js';
	import type { Conflict } from '#bindings/internal/remote/collab/models.js';
	import { canRepublish } from '#lib/remote/share.js';
	import { requestLogin, shareRights } from '#lib/remote/lattis.svelte.js';
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
	// Écran de récupération : les 3 plus récents d'abord, les autres sur demande.
	const ORPHANS_SHOWN = 3;
	let allOrphans = $state(false);
	const sortedOrphans = $derived([...orphans].sort((a, b) => String(b.modified).localeCompare(String(a.modified))));
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
	// Fichier endommagé à l'ouverture : proposer d'en récupérer les données.
	let damaged = $state<DamagedFile | null>(null);
	// Échec d'une récupération de session : l'écran de récupération reste.
	let recoverError = $state('');
	// Boîtes de dialogue ouvertes par un script ou un assistant (ui.dialog…), la plus ancienne d'abord.
	let dialogs = $state<DialogSpec[]>([]);
	let mcpOpen = $state(false);
	let settingsOpen = $state(false);
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
	let shareDlg = $state<{ republish: boolean } | null>(null);
	// Fichier › Partager… : droits du classeur publié (propriétaire ou
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
	let openRemote = $state(false);
	let shareReloading = $state(false);
	// Données › Obtenir des données : requêtes d'import (lib/query).
	let queryHost = $state<ReturnType<typeof QueryHost>>();
	// Révision › Protéger : boîte du mot de passe (protéger, ou ôter une
	// protection qui en a un).
	// Suppression d'une feuille qui contient quelque chose : confirmée avec la liste de ce qui part.
	let deleting = $state<{ id: number; name: string; items: string[] } | null>(null);
	let protecting = $state<{ what: 'sheet' | 'workbook'; id: number; name: string; off: boolean } | null>(null);
	let pw1 = $state('');
	let pw2 = $state('');
	let pwError = $state('');
	// Protéger le classeur : verrouiller aussi le contenu de toutes les feuilles.
	let lockSheets = $state(false);

	// Démo.
	let demo = $state<{ name: string; rows: number; columns: { name: string; type: ColumnType }[]; sqlName: string; sqlColumns: string[] } | null>(null);
	let demoProgress = $state('');

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
	}

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
		await reload();
		await keepSheet();
		if (r.state.notice) notice = r.state.notice;
		workbookReady();
		await loadRecent();
	}

	const loadRecent = async () => (recent = (await DocumentService.Recent().catch(() => null)) ?? []);

	async function openPath(path: string) {
		const r = await DocumentService.Open(path);
		if (r.recoverable) {
			orphans = [r.recoverable];
			recoverError = '';
			mode = 'recover';
		} else if (r.damaged) {
			damaged = r.damaged;
		} else if (!r.cancelled) {
			view = 'grid';
			sheetId = 1;
			mode = 'doc';
			await reload();
			// Déclencheurs inconnus supprimés à l'ouverture (sécurité).
			if (r.state.notice) notice = r.state.notice;
			workbookReady();
		}
		await loadRecent();
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
		if (mode === 'doc' && save === undefined && !(await DocumentService.BeforeClose())) return;
		if (mode === 'doc' && docState.dirty && save === undefined) {
			closing = true;
			return;
		}
		if (save) {
			docState = await DocumentService.Save(false);
			if (docState.dirty) {
				// Enregistrement abandonné (ou refusé par un script : before_save).
				if (docState.notice) notice = docState.notice;
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
		await loadRecent();
	}

	async function startDoc() {
		mode = 'doc';
		view = 'grid';
		sheetId = 1;
		await DocumentService.New();
		await reload();
	}

	/** Ouvre le classeur de démonstration demandé au lancement (--demo preset). */
	async function startPresetDemo(preset: string) {
		let waiting = true;
		const poll = async () => {
			if (!waiting) return;
			const p = await DocumentService.DemoProgress().catch(() => null);
			if (p && waiting)
				demoProgress =
					`Démo « ${preset} » : ${p.done.toLocaleString('fr-FR')} / ${p.total.toLocaleString('fr-FR')} lignes` +
					(p.totalCells ? ` · ${p.cells.toLocaleString('fr-FR')} / ${p.totalCells.toLocaleString('fr-FR')} cellules` : '');
			setTimeout(poll, 250);
		};
		poll();
		try {
			const r = await DocumentService.OpenDemo();
			view = 'grid';
			sheetId = r.sheet;
			mode = 'doc';
			await reload();
		} finally {
			waiting = false;
			demoProgress = '';
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
	// Onglet IA désactivé (Fichier › Paramètres) pendant qu'il est affiché :
	// retour à la grille.
	$effect(() => {
		if (view === 'ai' && !aiEnabled()) untrack(() => (view = 'grid'));
	});

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
		loadSettings();
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
		const offUI = Events.On('ui:command', (e: { data: UICommand }) => {
			uiCommand(e.data).catch((x) => (notice = err(x)));
		});
		// Classeur ouvert depuis le système pendant que l'application tourne
		// (macOS : double-clic sur un .lattis dans le Finder).
		const offOpen = Events.On('doc:open-file', (e: { data: string }) => {
			openPath(e.data).catch((x) => (notice = err(x)));
		});
		(async () => {
			try {
				bench = await GridService.BenchRequested();
				if (bench) return startDemo();
				loadRecent();
				// Web : le moteur survit à l'onglet (rechargement, onglet
				// fermé puis rouvert) ; on reprend le classeur qu'il a ouvert.
				if (isWeb && (await DocumentService.State()).open) {
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
				orphans = (await DocumentService.Orphans()) ?? [];
				if (orphans.length > 0) mode = 'recover';
				else await startDoc();
			} catch (e) {
				failure = err(e);
			}
		})();
		const keys = (e: KeyboardEvent) => {
			if (e.defaultPrevented) return; // déjà traité (éditeur de script : Ctrl+S)
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
		// page de ce qui est affiché (SQL, SCRIPT, onglet HTML ou Document).
		const helpKey = (e: KeyboardEvent) => {
			if (e.key !== 'F1' || e.ctrlKey || e.altKey || e.metaKey) return;
			e.preventDefault();
			e.stopPropagation();
			const el = document.activeElement;
			if (el instanceof HTMLElement && el.closest('.help')) return;
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
			offShare();
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
					docState = await DocumentService.Save(a === 'saveAs');
					if (docState.notice) notice = docState.notice;
					else if (!docState.dirty) notice = `Enregistré : ${docState.path}`;
					await loadRecent();
					break;
				case 'import': {
					if (mode !== 'doc') await startDoc();
					const r = await DocumentService.Import('');
					if (r.cancelled) break;
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
				case 'exportXlsx': {
					if (mode !== 'doc') return;
					const r = await DocumentService.ExportXLSX();
					if (r.path)
						notice =
							`Classeur exporté (valeurs des formules) : ${r.path}` +
							(r.clipped ? ` ; ${r.clipped.toLocaleString('fr-FR')} cellule(s) au-delà des limites d'Excel (1 048 576 lignes × 16 384 colonnes) non exportée(s)` : '');
					break;
				}
				case 'print':
				case 'pdf': {
					// Feuille active, plage utilisée ; le PDF passe par la fenêtre
					// d'impression (imprimante « Microsoft Print to PDF »).
					if (mode !== 'doc') return;
					if (activeDoc && docPage) {
						if (a === 'pdf') exportDocPdf();
						else docPage.print(`${docState.title} – ${activeInfo?.name ?? ''}`);
						break;
					}
					const name = layout?.sheets?.find((s) => s.id === sheetId)?.name ?? '';
					if (a === 'pdf') notice = 'Exporter en PDF : choisissez l’imprimante « Microsoft Print to PDF » (ou « Enregistrer au format PDF »)';
					printSheet(await DocumentService.PrintHTML(sheetId), `${docState.title} – ${name}`);
					break;
				}
				case 'demo':
					await startDemo();
					break;
				case 'mcp':
					mcpOpen = true;
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

	async function recover(o: Orphan, keep: boolean) {
		recoverError = '';
		try {
			if (keep) await DocumentService.Recover(o.dir);
			else await DocumentService.Discard(o.dir);
		} catch (e) {
			// La session reste proposée : rien n'est perdu, l'utilisateur choisit.
			recoverError = err(e);
			return;
		}
		try {
			if (keep) {
				mode = 'doc';
				await reload();
				notice = 'Modifications récupérées : pensez à enregistrer.';
				workbookReady();
			} else {
				orphans = orphans.filter((x) => x.dir !== o.dir);
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
		async setCell(row, col, raw) {
			const ev = await DocumentService.SetCell(sheetId, row, col, raw);
			// Relire tout de suite (l'événement doc:changed sert aux changements venus
			// d'ailleurs, et n'existe pas en mode serveur).
			await reload();
			if (ev.keyed) return `Clé posée sur « ${ev.keyed.slice(ev.keyed.indexOf('.') + 1)} » pour REFERENCE`;
			if (ev.created) return `Tableau « ${ev.created} » créé`;
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
			if (ev.created && ev.headless) return `Tableau « ${ev.created} » créé sans en-têtes : clic droit › « Utiliser la 1re ligne comme en-têtes » si besoin`;
			if (ev.created) return `Tableau « ${ev.created} » créé (${count(block.length - 1)})`;
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
		keyCheck: (t, i) => DocumentService.KeyCheck(t.name, i),
		setKey: (t, i) => DocumentService.SetKey(t.name, i).then(reload),
		clearKey: (t) => DocumentService.ClearKey(t.name).then(reload),
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
						query: t.queryId ? { id: t.queryId, name: t.queryName } : undefined,
						shared: (t.shared || undefined) as 'published' | 'private' | undefined,
						readOnly: t.readOnly || undefined,
						access: t.access || undefined,
						filtered: t.filtered || undefined,
						columns: (t.columns ?? []).map((c) => ({
							name: c.name,
							type: (c.formula ? 'formula' : c.type) as ColumnType,
							fixed: c.fixed,
							errors: c.errors,
							formula: c.formula || undefined,
							result: c.result || undefined,
							style: c.style || undefined,
							headerStyle: c.headerStyle || undefined,
							rules: c.rules || undefined,
							filter: filter[String(c.id)]
						}))
					};
					}),
					sheet: sheetId,
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
	const tabs = $derived(mode === 'demo' ? (demo ? [{ id: 1, name: demo.name }] : []) : (layout?.sheets ?? []).map((s) => ({ id: s.id, name: s.name, protected: s.protected, color: s.color, hidden: s.hidden, kind: s.kind })));

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
		// Changement d'onglet : tous les volets de droite se ferment.
		sheet?.closeOverlays();
		queryHost?.closePanel();
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

	// Document en PDF : la fenêtre d'impression, imprimante « Microsoft Print to PDF ».
	function exportDocPdf() {
		notice = 'Exporter en PDF : choisissez l’imprimante « Microsoft Print to PDF » (ou « Enregistrer au format PDF »)';
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
			? `La feuille « ${activeInfo?.name ?? ''} » est protégée : ôtez la protection (Révision › Ôter la protection) pour la modifier`
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
		{title}
		dirty={mode === 'doc' && docState.dirty}
		disabled={mode !== 'doc' && mode !== 'demo'}
		{recent}
		onrecent={(p) => openPath(p).catch((e) => (notice = err(e)))}
		undo={mode === 'doc' ? docState.undo : ''}
		redo={mode === 'doc' ? docState.redo : ''}
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
		onfilter={mode === 'doc' ? (errorsOnly) => sheet?.filterActive(errorsOnly) : undefined}
		onaudit={mode === 'doc' ? (k) => sheet?.auditActive(k) : undefined}
		onpainter={mode === 'doc' ? () => sheet?.formatPainter() : undefined}
		onclip={mode === 'doc' ? (what) => (what === 'paste' ? sheet?.pasteClipboard() : sheet?.copySelection(what === 'cut')) : undefined}
		onchart={mode === 'doc' ? (kind) => sheet?.chartActive(kind) : undefined}
		onformula={mode === 'doc' ? (what) => (what === 'insert' ? sheet?.openFunctions() : what === 'sum' ? sheet?.autoSum() : sheet?.startFormula('=REFERENCE(')) : undefined}
		onpivot={mode === 'doc' ? () => sheet?.pivotActive() : undefined}
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
	/>

	{#if mode === 'doc'}
		<ShareBanner {share} busy={shareReloading} onreload={reloadShare} onlogin={() => share && requestLogin(share.server)} onconflicts={() => (conflictsOpen = true)} />
	{/if}

	<main>
		{#if failure}
			<div class="splash"><h1>Lattis</h1><p class="error">{failure}</p></div>
		{:else if mode === 'recover'}
			<div class="splash">
				<h1>Récupération</h1>
				<p>Lattis s'est fermé sans enregistrer. Ces classeurs ont des modifications récupérables :</p>
				<div class="orphans">
				{#each allOrphans ? sortedOrphans : sortedOrphans.slice(0, ORPHANS_SHOWN) as o (o.dir)}
					<div class="orphan">
						<div>
							<strong>{o.source ? o.source.split(/[\\/]/).pop() : 'Classeur sans titre'}</strong>
							<span class="muted">modifié le {new Date(o.modified).toLocaleString('fr-FR')}</span>
						</div>
						<button class="primary ico" onclick={() => recover(o, true)}><Icon name="undo" size={14} />Récupérer</button>
						<button class="ico discard" onclick={() => recover(o, false)}><Icon name="trash" size={14} />Abandonner</button>
					</div>
				{/each}
				</div>
				{#if !allOrphans && orphans.length > ORPHANS_SHOWN}
					<button class="link" onclick={() => (allOrphans = true)}>Voir les {orphans.length - ORPHANS_SHOWN} plus anciens</button>
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
				</div>
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
			{#if view === 'ai'}
				<div class="pane" data-region="grid"><AiView hasDoc={mode === 'doc'} onsettings={() => (settingsOpen = true)} /></div>
			{:else if view === 'sql'}
				<div class="pane" data-region="grid"><SqlView {objects} rights={rightsObjs} onrights={canRights && share?.canManage ? () => (rightsOpen = true) : undefined} run={query} exec={mode === 'doc' ? (q) => DocumentService.Execute(q) : undefined} ongoto={gotoObject}
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
		{#if mode === 'doc'}
			<QueryHost
				bind:this={queryHost}
				onchanged={(sh) => (sh && sh !== sheetId ? selectSheet(sh) : reload()).catch((e) => (notice = err(e)))}
				onnotice={(m) => (notice = m)}
			/>
		{/if}
		{#if helpState.open}
			<HelpPanel />
		{/if}
		{#if mode === 'doc' && conflictsOpen && share?.mode === 'collaborative'}
			<ConflictPanel {conflicts} busy={conflictBusy} onresolve={resolveConflict} ongoto={gotoConflict} onclose={() => (conflictsOpen = false)} />
		{/if}
	</main>

	<SheetTabs
		sheets={tabs}
		active={mode === 'demo' ? 1 : sheetId}
		onselect={selectSheet}
		onadd={mode === 'doc' ? () => sheetAction(async () => selectSheet(await DocumentService.AddSheet())) : undefined}
		oninsert={mode === 'doc' ? (before) => sheetAction(async () => selectSheet(await DocumentService.InsertSheet(before))) : undefined}
		onpage={mode === 'doc' && htmlTabsEnabled() ? (before) => addPage(before) : undefined}
		ondocpage={mode === 'doc' ? (before) => addDoc(before) : undefined}
		onmove={mode === 'doc' ? (id, to) => sheetAction(() => DocumentService.MoveSheet(id, to)) : undefined}
		onrename={mode === 'doc' ? (id, name) => sheetAction(() => DocumentService.RenameSheet(id, name)) : undefined}
		ondelete={mode === 'doc' ? (id) => askDeleteSheet(id) : undefined}
		oncopy={mode === 'doc' ? (id, to) => sheetAction(async () => selectSheet(await DocumentService.CopySheet(id, to))) : undefined}
		oncolor={mode === 'doc' ? (id, color) => sheetAction(() => DocumentService.SetSheetColor(id, color)) : undefined}
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
	/>
	{#each dialogs.slice(0, 1) as d (d.id)}
		<ScriptDialog spec={d} onanswer={(v) => answerDialog(d, v)} />
	{/each}
	{#if mcpOpen}
		<McpPanel onclose={() => (mcpOpen = false)} />
	{/if}
	{#if settingsOpen}
		<SettingsDialog onclose={() => (settingsOpen = false)} />
	{/if}
	{#if rightsOpen && canRights && share}
		<RightsDialog title={share.title} server={share.server} onclose={() => (rightsOpen = false)} />
	{/if}
	{#if shareDlg && mode === 'doc'}
		<ShareDialog
			republish={shareDlg.republish}
			{share}
			title={docState.title}
			ondone={(st) => {
				const again = shareDlg?.republish;
				shareDlg = null;
				if (!st) return;
				share = st;
				notice = again ? `Version ${st.version} publiée` : `Classeur publié sur ${st.serverName || st.server} (${st.mode === 'frozen' ? 'figé' : 'collaboratif'})`;
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
			<div class="modal" role="dialog" aria-modal="true" aria-label="Enregistrer les modifications" use:modal={() => (closing = false)}>
				<p>Enregistrer les modifications de « {docState.title} » avant de le fermer ?</p>
				<div class="actions">
					<button class="primary" onclick={() => closeDoc(true).catch((e) => (notice = err(e)))}>Enregistrer</button>
					<button onclick={() => closeDoc(false).catch((e) => (notice = err(e)))}>Ne pas enregistrer</button>
					<button onclick={() => (closing = false)}>Annuler</button>
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
	<footer class="statusbar" data-region="status" aria-label="Barre d'état">
		{#if notice || sheetStatus?.message}<span class="message">{sheetStatus?.message || notice}</span>{/if}
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
		background: #fff7e0;
		color: #7a4b00;
		padding: 1px 8px;
		border-radius: 3px;
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
		z-index: 50;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(0, 0, 0, 0.2);
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
