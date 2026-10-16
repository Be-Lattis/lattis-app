<script lang="ts">
	// Classeur façon Excel : ruban en haut, feuille (ou onglet SQL / SCRIPT) au
	// centre, onglets de feuilles et barre d'état en bas.
	//
	// Deux modes : un classeur (DocumentService : saisie libre, tableaux qui
	// naissent, annuler, enregistrer) et la démo d'1 M de lignes (GridService),
	// qui sert aussi à la mesure automatique (SLITABLE_BENCH).
	import { onMount } from 'svelte';
	import { Events } from '@wailsio/runtime';
	import { DocumentService, GridService } from '#bindings/internal/bridge/index.js';
	import type { DocState, FormulaHelp } from '#bindings/internal/bridge/models.js';
	import type { Layout } from '#bindings/internal/canvas/models.js';
	import type { Type } from '#bindings/internal/grid/models.js';
	import type { Orphan } from '#bindings/internal/archive/models.js';
	import Sheet, { type SheetData, type SheetSource, type SheetStatus } from '#lib/grid/Sheet.svelte';
	import Ribbon, { type FileAction, type View } from '#lib/shell/Ribbon.svelte';
	import SheetTabs from '#lib/shell/SheetTabs.svelte';
	import SqlView from '#lib/shell/SqlView.svelte';
	import TitleBar from '#lib/shell/TitleBar.svelte';
	import ScriptView, { type Script } from '#lib/script/ScriptView.svelte';
	import { typeLabel, type ColumnType } from '#lib/grid/format.js';
	import type { Fetched } from '#lib/grid/cache.js';

	type Mode = 'starting' | 'recover' | 'doc' | 'demo';

	let mode = $state<Mode>('starting');
	let bench = $state(false);
	let view = $state<View>('grid');
	let failure = $state('');
	let notice = $state('');
	let sheetStatus = $state<SheetStatus | null>(null);
	let sheet = $state<ReturnType<typeof Sheet>>();

	// Classeur.
	let docState = $state<DocState>({ open: false, path: '', title: '', dirty: false, undo: '', redo: '' });
	let layout = $state<Layout | null>(null);
	let sheetId = $state(1);
	let orphans = $state<Orphan[]>([]);
	let help = $state<FormulaHelp | null>(null);
	let scripts = $state<Script[]>([]);

	// Démo.
	let demo = $state<{ name: string; rows: number; columns: { name: string; type: ColumnType }[]; sqlName: string; sqlColumns: string[] } | null>(null);
	let demoProgress = $state('');

	const err = (e: unknown) => String(e instanceof Error ? e.message : e);

	async function reload() {
		docState = await DocumentService.State();
		layout = await DocumentService.Layout(sheetId);
		help = await DocumentService.FormulaHelp();
		scripts = (await DocumentService.Scripts()) ?? [];
	}

	async function startDoc() {
		mode = 'doc';
		await DocumentService.New();
		await reload();
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

	onMount(() => {
		const off = Events.On('doc:changed', () => {
			if (mode === 'doc') reload();
		});
		(async () => {
			try {
				bench = await GridService.BenchRequested();
				if (bench) return startDemo();
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
				e.key === 'F12' ? 'saveAs' : ctrl && e.key.toLowerCase() === 's' ? 'save' : ctrl && e.key.toLowerCase() === 'o' ? 'open' : ctrl && e.key.toLowerCase() === 'n' ? 'new' : null;
			if (action) {
				e.preventDefault();
				file(action);
			}
		};
		window.addEventListener('keydown', keys);
		return () => {
			off();
			window.removeEventListener('keydown', keys);
		};
	});

	async function file(a: FileAction) {
		notice = '';
		try {
			switch (a) {
				case 'new':
					await startDoc();
					break;
				case 'open': {
					const r = await DocumentService.Open('');
					if (r.recoverable) {
						orphans = [r.recoverable];
						mode = 'recover';
					} else if (!r.cancelled) {
						mode = 'doc';
						await reload();
					}
					break;
				}
				case 'save':
				case 'saveAs':
					if (mode !== 'doc') return;
					docState = await DocumentService.Save(a === 'saveAs');
					if (!docState.dirty) notice = `Enregistré : ${docState.path}`;
					break;
				case 'demo':
					await startDemo();
					break;
			}
		} catch (e) {
			notice = err(e);
		}
		sheet?.focus();
	}

	async function recover(o: Orphan, keep: boolean) {
		try {
			if (keep) {
				await DocumentService.Recover(o.dir);
				mode = 'doc';
				await reload();
				notice = 'Modifications récupérées : pensez à enregistrer.';
			} else {
				await DocumentService.Discard(o.dir);
				orphans = orphans.filter((x) => x.dir !== o.dir);
				if (orphans.length === 0) await startDoc();
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
			if (ev.created) return `Tableau « ${ev.created} » créé`;
			if (ev.widened) return `Colonne ${ev.widened} passée de ${typeLabel(ev.from as ColumnType)} à ${typeLabel(ev.to as ColumnType)} (Ctrl+Z pour annuler)`;
			if (ev.invalid) return 'Valeur hors type : cellule marquée en erreur';
		},
		undo: () => DocumentService.Undo().then(reload),
		redo: () => DocumentService.Redo().then(reload),
		async paste(row, col, block) {
			const ev = await DocumentService.Paste(sheetId, row, col, block);
			await reload();
			if (ev.created) return `Tableau « ${ev.created} » créé (${block.length - 1} lignes)`;
		},
		insertRow: (t, n) => DocumentService.InsertRow(t.name, n).then(reload),
		deleteRow: (t, n) => DocumentService.DeleteRow(t.name, n).then(reload),
		insertColumn: (t, i) => DocumentService.InsertColumn(t.name, i, `Colonne ${t.columns.length + 1}`).then(reload),
		deleteColumn: (t, i) => DocumentService.DeleteColumn(t.name, i).then(reload),
		convertPreview: (t, i, to) => DocumentService.ConvertPreview(t.name, i, to as Type),
		setType: (t, i, to) => DocumentService.SetType(t.name, i, to as Type).then(reload),
		setColumnWidth: (col, width) => DocumentService.SetColumnWidth(sheetId, col, width).then(reload),
		setName: (row, col, name) => DocumentService.SetName(sheetId, row, col, name).then(reload),
		sortTable: (t, i, desc) => DocumentService.SortTable(t.name, i, desc).then(reload),
		setRules: (t, i, rules) => DocumentService.SetRules(t.name, i, rules).then(reload),
		applyStyle: (r0, c0, r1, c1, patch) => DocumentService.ApplyStyle(sheetId, r0, c0, r1, c1, patch).then(reload),
		find: async (text) => (await DocumentService.Find(sheetId, text)) ?? [],
		replace: async (text, by) => {
			const n = await DocumentService.Replace(sheetId, text, by);
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
					tables: (layout.tables ?? []).map((t) => ({
						id: t.id,
						name: t.name,
						row: t.row,
						col: t.col,
						rows: t.rows,
						columns: (t.columns ?? []).map((c) => ({
							name: c.name,
							type: (c.formula ? 'formula' : c.type) as ColumnType,
							fixed: c.fixed,
							errors: c.errors,
							formula: c.formula || undefined,
							style: c.style || undefined,
							headerStyle: c.headerStyle || undefined,
							rules: c.rules || undefined
						}))
					})),
					cells: layout.cells ?? [],
					widths: layout.widths ?? [],
					names: layout.names ?? [],
					styles: layout.styles ?? []
				}
			: null
	);

	const demoData = $derived<SheetData | null>(demo ? { tables: [{ id: 1, name: demo.name, row: 0, col: 0, rows: demo.rows, columns: demo.columns }], cells: [] } : null);

	const objects = $derived(
		mode === 'demo' && demo
			? [{ name: demo.name, sqlName: demo.sqlName, rows: demo.rows, columns: demo.columns.map((c, i) => ({ ...c, sql: demo!.sqlColumns[i] ?? '' })) }]
			: (layout?.tables ?? []).map((t) => ({
					name: t.name,
					sqlName: t.sql,
					rows: t.rows,
					columns: (t.columns ?? []).map((c) => ({ name: c.name, type: c.type as ColumnType, sql: c.sql }))
				}))
	);

	const query = (sql: string) => (mode === 'demo' ? GridService.Query(sql) : DocumentService.Query(sql));
	const data = $derived(mode === 'demo' ? demoData : docData);
	const title = $derived(mode === 'demo' ? 'Démo : 1 M de lignes' : docState.title);
	const tabs = $derived(mode === 'demo' ? (demo ? [{ id: 1, name: demo.name }] : []) : (layout?.sheets ?? []).map((s) => ({ id: s.id, name: s.name })));

	async function selectSheet(id: number) {
		view = 'grid';
		if (mode !== 'doc' || id === sheetId) return;
		sheetId = id;
		await reload();
		sheet?.focus();
	}

	async function sheetAction(fn: () => Promise<unknown>) {
		try {
			await fn();
			await reload();
			if (!(layout?.sheets ?? []).some((s) => s.id === sheetId)) {
				sheetId = layout?.sheets?.[0]?.id ?? 1;
				await reload();
			}
		} catch (e) {
			notice = err(e);
		}
	}
</script>

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
		undo={mode === 'doc' ? docState.undo : ''}
		redo={mode === 'doc' ? docState.redo : ''}
		onview={(v) => (view = v)}
		onfile={file}
		onundo={() => DocumentService.Undo().then(reload).catch((e) => (notice = err(e)))}
		onredo={() => DocumentService.Redo().then(reload).catch((e) => (notice = err(e)))}
		onbench={mode === 'demo' ? () => sheet?.runBench() : undefined}
		onsort={mode === 'doc' ? (desc) => sheet?.sortActive(desc) : undefined}
		onstyle={mode === 'doc' ? (patch) => sheet?.applyStyle(patch) : undefined}
		onfind={mode === 'doc' ? (replace) => sheet?.openFinder(replace) : undefined}
		onrules={mode === 'doc' ? () => sheet?.rulesActive() : undefined}
	/>

	<main>
		{#if failure}
			<div class="splash"><h1>Slitable</h1><p class="error">{failure}</p></div>
		{:else if mode === 'recover'}
			<div class="splash">
				<h1>Récupération</h1>
				<p>Slitable s'est fermé sans enregistrer. Ces classeurs ont des modifications récupérables :</p>
				{#each orphans as o (o.dir)}
					<div class="orphan">
						<div>
							<strong>{o.source ? o.source.split(/[\\/]/).pop() : 'Classeur sans titre'}</strong>
							<span class="muted">modifié le {new Date(o.modified).toLocaleString('fr-FR')}</span>
						</div>
						<button class="primary" onclick={() => recover(o, true)}>Récupérer</button>
						<button onclick={() => recover(o, false)}>Abandonner</button>
					</div>
				{/each}
			</div>
		{:else if data}
			{#key mode}
				<div class="pane" class:hidden={view !== 'grid'}>
					<Sheet
						bind:this={sheet}
						{data}
						source={mode === 'demo' ? demoSource : docSource}
						catalog={mode === 'doc' && help
							? { functions: help.functions ?? [], tables: help.tables ?? [], names: (layout?.names ?? []).map((n) => n.name) }
							: undefined}
						onstatus={(s) => (sheetStatus = s)}
						autoBench={bench ? (r) => GridService.ReportBench(r) : undefined}
					/>
				</div>
			{/key}
			{#if view === 'sql'}
				<div class="pane"><SqlView {objects} run={query} ongoto={() => (view = 'grid')} /></div>
			{:else if view === 'script'}
				<div class="pane">
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
							subscribe={(fn) => Events.On('script:print', (e: { data: string }) => fn(e.data))}
						/>
					{:else}
						<p class="placeholder">Les scripts s'exécutent sur un classeur : Fichier › Nouveau.</p>
					{/if}
				</div>
			{/if}
		{:else}
			<div class="splash">
				<h1>Slitable</h1>
				<p>{mode === 'demo' ? demoProgress : 'Ouverture…'}</p>
			</div>
		{/if}
	</main>

	<SheetTabs
		sheets={tabs}
		active={mode === 'demo' ? 1 : sheetId}
		onselect={selectSheet}
		onadd={mode === 'doc' ? () => sheetAction(async () => selectSheet(await DocumentService.AddSheet())) : undefined}
		onrename={mode === 'doc' ? (id, name) => sheetAction(() => DocumentService.RenameSheet(id, name)) : undefined}
		ondelete={mode === 'doc' ? (id) => sheetAction(() => DocumentService.DeleteSheet(id)) : undefined}
	/>
	<footer class="statusbar">
		{#if sheetStatus}
			<span class="cell">{sheetStatus.cell}</span>
		{/if}
		{#if notice || sheetStatus?.message}<span class="message">{sheetStatus?.message || notice}</span>{/if}
		<span class="spacer"></span>
		{#if sheetStatus}
			{#if sheetStatus.stats}<span class="stats">{sheetStatus.stats}</span>{/if}
			<span>{sheetStatus.fetch}</span>
			<span>{sheetStatus.draw}</span>
			<span>{sheetStatus.fps}</span>
			<span>{sheetStatus.edit}</span>
		{:else}
			<span>Prêt</span>
		{/if}
	</footer>
</div>

<style>
	:global(html, body) {
		margin: 0;
		height: 100%;
		background: #fff;
		overflow: hidden;
	}
	.app {
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
	.statusbar .cell {
		color: #fff;
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
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.error {
		color: #b42318;
	}
</style>
