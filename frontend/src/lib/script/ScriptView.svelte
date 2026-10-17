<script lang="ts">
	// Onglet SCRIPT : bibliothèque de scripts Lua du classeur, éditeur,
	// exécution avec sortie en direct et bouton Arrêter. Les écritures d'un
	// script forment une seule étape d'annulation. Référence : docs/scripting.md.
	import { onMount } from 'svelte';
	import { EditorState } from '@codemirror/state';
	import { EditorView, keymap, lineNumbers, highlightActiveLine, drawSelection } from '@codemirror/view';
	import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
	import { bracketMatching } from '@codemirror/language';
	import { closeBrackets } from '@codemirror/autocomplete';
	import { dismiss } from '#lib/grid/dismiss.js';
	import Icon from '#lib/icons/Icon.svelte';
	import { luaTools } from './luaeditor';
	import ScriptReference from './ScriptReference.svelte';
	import NetworkAccess, { type HostAccess } from './NetworkAccess.svelte';

	export interface Script {
		id: number;
		name: string;
		source: string;
	}
	export interface RunResult {
		output: string[] | null;
		writes: number;
		durationMs: number;
		error: string;
	}
	interface Props {
		scripts: Script[];
		save: (id: number, name: string, source: string) => Promise<number>;
		remove: (id: number) => Promise<unknown>;
		run: (name: string, source: string) => Promise<RunResult>;
		stop: () => Promise<unknown>;
		/** Lignes de print reçues en direct (événement script:print). */
		subscribe?: (fn: (line: string) => void) => () => void;
		/** Hôtes HTTP autorisés pour le classeur (bouton Accès réseau). */
		network?: HostAccess;
	}

	let { scripts, save, remove, run, stop, subscribe, network }: Props = $props();
	let netOpen = $state(false);

	const EXAMPLE = `-- Exemple : la liste des tableaux du classeur et leurs colonnes.
-- db.query, db.rows, db.insert, db.execute… : voir le bouton « Référence ».
for _, t in ipairs(db.tables()) do
  print(t.name, #t.columns .. " colonne(s)")
end
`;

	let current = $state<Script>({ id: 0, name: 'Nouveau script', source: EXAMPLE });
	let output = $state<string[]>([]);
	let status = $state('');
	let error = $state('');
	let running = $state(false);
	let dirty = $state(false);
	let showRef = $state(true);

	let host: HTMLDivElement;
	let view: EditorView | undefined;

	function setSource(src: string) {
		view?.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: src } });
		dirty = false;
	}

	function open(s: Script) {
		current = { ...s };
		setSource(s.source);
		output = [];
		error = status = '';
	}

	function fresh() {
		current = { id: 0, name: 'Nouveau script', source: EXAMPLE };
		setSource(EXAMPLE);
	}

	async function doSave() {
		try {
			current.source = view?.state.doc.toString() ?? current.source;
			current.id = await save(current.id, current.name, current.source);
			dirty = false;
			status = 'Script enregistré dans le classeur';
			error = '';
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	}

	async function doRun() {
		if (running) return;
		running = true;
		output = [];
		error = '';
		status = 'Exécution…';
		const unsub = subscribe?.((line) => (output = [...output, line]));
		try {
			const r = await run(current.name, view?.state.doc.toString() ?? '');
			output = r.output ?? output;
			error = r.error;
			status = `${r.error ? 'Arrêté' : 'Terminé'} en ${(r.durationMs / 1000).toLocaleString('fr-FR', { maximumFractionDigits: 2 })} s · ${r.writes} ligne(s) écrite(s)${r.writes ? ' (Ctrl+Z pour tout annuler)' : ''}`;
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
			status = '';
		} finally {
			unsub?.();
			running = false;
		}
	}

	// Insère un exemple de la référence sur ses propres lignes, au curseur.
	function insertSnippet(text: string) {
		if (!view) return;
		const { from, to } = view.state.selection.main;
		const line = view.state.doc.lineAt(from);
		const before = from > line.from ? '\n' : '';
		const after = text.endsWith('\n') ? '' : '\n';
		const insert = before + text + after;
		view.dispatch({ changes: { from, to, insert }, selection: { anchor: from + insert.length }, scrollIntoView: true });
		view.focus();
	}

	// Menu du clic droit sur un script : renommer, dupliquer, supprimer.
	let menu = $state<{ s: Script; x: number; y: number } | null>(null);
	let renaming = $state<number | null>(null);

	function onItemMenu(ev: MouseEvent, s: Script) {
		ev.preventDefault();
		menu = { s, x: ev.clientX, y: ev.clientY };
	}

	async function renameTo(s: Script, name: string) {
		renaming = null;
		name = name.trim();
		if (!name || name === s.name) return;
		try {
			await save(s.id, name, s.id === current.id ? (view?.state.doc.toString() ?? s.source) : s.source);
			if (s.id === current.id) current.name = name;
			status = `Script renommé « ${name} »`;
			error = '';
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	}

	async function duplicate(s: Script) {
		menu = null;
		const taken = new Set(scripts.map((x) => x.name.toLowerCase()));
		let name = `${s.name} (copie)`;
		for (let n = 2; taken.has(name.toLowerCase()); n++) name = `${s.name} (copie ${n})`;
		try {
			const id = await save(0, name, s.source);
			open({ id, name, source: s.source });
			status = `Script dupliqué : « ${name} »`;
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	}

	async function removeScript(s: Script) {
		menu = null;
		await remove(s.id);
		if (s.id === current.id) fresh();
	}

	async function doRemove() {
		if (!current.id) return fresh();
		await remove(current.id);
		fresh();
	}

	onMount(() => {
		view = new EditorView({
			parent: host,
			state: EditorState.create({
				doc: current.source,
				extensions: [
					lineNumbers(),
					highlightActiveLine(),
					drawSelection(),
					history(),
					bracketMatching(),
					closeBrackets(),
					luaTools(),
					keymap.of([
						{ key: 'Mod-Enter', run: () => (doRun(), true) },
						{ key: 'Mod-s', run: () => (doSave(), true) },
						indentWithTab,
						...defaultKeymap,
						...historyKeymap
					]),
					EditorView.updateListener.of((u) => {
						if (u.docChanged) dirty = true;
					}),
					EditorView.theme({
						'&': { height: '100%', fontSize: '13px' },
						'.cm-content': { fontFamily: "Consolas, 'Cascadia Mono', monospace" },
						'.cm-gutters': { background: '#f7f8f9', borderRight: '1px solid #e3e5e8' }
					})
				]
			})
		});
		return () => view?.destroy();
	});
</script>

<div class="script">
	<aside class="library">
		<h2>Scripts du classeur</h2>
		<button class="new" onclick={fresh}><Icon name="new-file" size={14} />Nouveau script</button>
		{#each scripts as s (s.id)}
			{#if renaming === s.id}
				<!-- svelte-ignore a11y_autofocus -->
				<input
					class="rename"
					value={s.name}
					autofocus
					onfocus={(e) => e.currentTarget.select()}
					onkeydown={(e) => {
						if (e.key === 'Enter') renameTo(s, e.currentTarget.value);
						else if (e.key === 'Escape') renaming = null;
					}}
					onblur={(e) => renameTo(s, e.currentTarget.value)}
					spellcheck="false"
				/>
			{:else}
				<button class="item" class:on={s.id === current.id} onclick={() => open(s)} oncontextmenu={(e) => onItemMenu(e, s)}>{s.name}</button>
			{/if}
		{/each}
		{#if scripts.length === 0}<p class="muted">Aucun script enregistré.</p>{/if}
	</aside>
	{#if menu}
		{@const m = menu}
		<div class="ctx" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={() => (menu = null)}>
			<button
				onclick={() => {
					menu = null;
					renaming = m.s.id;
				}}>Renommer</button
			>
			<button onclick={() => duplicate(m.s)}>Dupliquer</button>
			<button onclick={() => removeScript(m.s)}>Supprimer</button>
		</div>
	{/if}
	<section class="main">
		<div class="toolbar">
			<input class="name" bind:value={current.name} oninput={() => (dirty = true)} spellcheck="false" />
			<button class="run" onclick={doRun} disabled={running}><Icon name="play" size={14} />Exécuter</button>
			<button class="stop" onclick={() => stop()} disabled={!running}><Icon name="stop" size={14} />Arrêter</button>
			<button onclick={doSave}><Icon name="save" size={14} />Enregistrer{dirty ? ' •' : ''}</button>
			<button class="delete" onclick={doRemove} disabled={!current.id}><Icon name="trash" size={14} />Supprimer</button>
			<button onclick={() => (showRef = !showRef)} aria-pressed={showRef}><Icon name="book" size={14} />Référence</button>
			{#if network}<button onclick={() => (netOpen = true)} title="Hôtes que les scripts et pages HTML du classeur peuvent joindre en HTTP"><Icon name="web" size={14} />Accès réseau</button>{/if}
			<span class="muted">Ctrl+Entrée : exécuter · Ctrl+S : enregistrer le script</span>
		</div>
		<div class="editor" bind:this={host}></div>
		<div class="output">
			<div class="out-head">
				<span>Sortie</span>
				{#if status}<span class="muted">{status}</span>{/if}
			</div>
			<pre>{output.join('\n')}{#if error}<span class="error">{(output.length ? '\n' : '') + error}</span>{/if}</pre>
		</div>
	</section>
	{#if showRef}<ScriptReference insert={insertSnippet} close={() => (showRef = false)} />{/if}
	{#if netOpen && network}<NetworkAccess {...network} onclose={() => (netOpen = false)} />{/if}
</div>

<style>
	.script {
		display: flex;
		height: 100%;
		min-height: 0;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.library {
		width: 240px;
		border-right: 1px solid #d4d7db;
		background: #fafbfb;
		padding: 8px;
		overflow: auto;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	h2 {
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #6b7280;
		margin: 4px 4px 8px;
	}
	.library button {
		text-align: left;
		background: none;
		border: none;
		font: inherit;
		padding: 5px 8px;
		border-radius: 3px;
		cursor: pointer;
	}
	.library .item.on {
		background: #e3efe7;
		color: #14532d;
		font-weight: 600;
	}
	.library .rename {
		font: inherit;
		padding: 4px 7px;
		border: 1px solid #217346;
		border-radius: 3px;
		outline: none;
	}
	.ctx {
		position: fixed;
		z-index: 30;
		display: flex;
		flex-direction: column;
		min-width: 160px;
		padding: 4px 0;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
	}
	.ctx button {
		text-align: left;
		background: none;
		border: none;
		font: inherit;
		padding: 6px 14px;
		cursor: pointer;
	}
	.ctx button:hover {
		background: #e3efe7;
	}
	.library .new {
		display: flex;
		align-items: center;
		gap: 6px;
		color: #217346;
	}
	.main {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px;
		padding: 6px 10px;
		border-bottom: 1px solid #e3e5e8;
	}
	.toolbar button,
	.toolbar .muted {
		white-space: nowrap;
	}
	.toolbar button {
		/* Icône devant le libellé, accent vert comme le ruban. */
		display: inline-flex;
		align-items: center;
		gap: 5px;
		--icon-accent: #217346;
		font: inherit;
		padding: 4px 10px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.toolbar .run {
		background: #217346;
		color: #fff;
		border-color: #217346;
		--icon-accent: currentColor;
	}
	.toolbar .stop,
	.toolbar .delete {
		--icon-accent: #c0392b;
	}
	.toolbar button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.name {
		font: inherit;
		font-weight: 600;
		border: 1px solid transparent;
		padding: 3px 6px;
		width: 220px;
	}
	.name:focus {
		border-color: #217346;
		outline: none;
	}
	.editor {
		flex: 1;
		min-height: 120px;
		overflow: auto;
	}
	.output {
		height: 35%;
		border-top: 1px solid #d4d7db;
		display: flex;
		flex-direction: column;
	}
	.out-head {
		display: flex;
		gap: 12px;
		padding: 4px 10px;
		background: #f7f8f9;
		font-weight: 600;
	}
	pre {
		flex: 1;
		margin: 0;
		padding: 6px 10px;
		overflow: auto;
		font: 12px Consolas, 'Cascadia Mono', monospace;
	}
	.muted {
		color: #6b7280;
		font-weight: 400;
		font-size: 12px;
	}
	.error {
		color: #b42318;
	}
</style>
