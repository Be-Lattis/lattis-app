<script lang="ts">
	// Onglet « page HTML » : la page du classeur affichée dans un cadre isolé
	// (bridge.ts), et son éditeur (HTML, CSS, JavaScript) avec un aperçu.
	// Afficher / Modifier basculent entre les deux ; Ctrl+S enregistre (une
	// étape d'annulation), Ctrl+Entrée met l'aperçu à jour sans enregistrer.
	import { onMount, untrack } from 'svelte';
	import { EditorState } from '@codemirror/state';
	import { EditorView, keymap, lineNumbers, highlightActiveLine, drawSelection } from '@codemirror/view';
	import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
	import { bracketMatching, syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language';
	import { closeBrackets } from '@codemirror/autocomplete';
	import { html } from '@codemirror/lang-html';
	import Icon from '#lib/icons/Icon.svelte';
	import { buildSrcdoc, eventOf, parseCall, replyOf } from './bridge';

	interface Props {
		/** Nom de l'onglet (titre de la barre). */
		name: string;
		/** Contenu enregistré de la page. */
		source: string;
		/** Enregistre le contenu (annulable). */
		save: (source: string) => Promise<unknown>;
		/** Exécute une commande de l'API pour la page (DocumentService.PageCall). */
		call: (name: string, args: Record<string, unknown>) => Promise<unknown>;
		/** Feuille ou classeur protégé : message, et pas de modification. */
		locked?: string;
		/** Change à chaque modification du classeur : la page reçoit « change ». */
		stamp?: unknown;
		onnotice?: (msg: string) => void;
	}

	let { name, source, save, call, locked = '', stamp, onnotice }: Props = $props();

	let editing = $state(false);
	let dirty = $state(false);
	// Contenu affiché dans le cadre : l'enregistré, ou le brouillon en aperçu.
	let shown = $state(untrack(() => source));
	let frame = $state<HTMLIFrameElement>();
	let host = $state<HTMLDivElement>();
	let view: EditorView | undefined;
	let error = $state('');
	let reloads = $state(0);

	// Autre onglet, annulation, page modifiée par un script : le contenu
	// enregistré fait foi (hors modification en cours).
	$effect(() => {
		const s = source;
		untrack(() => {
			if (!dirty) {
				shown = s;
				if (view && view.state.doc.toString() !== s) view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: s } });
				dirty = false;
			}
		});
	});

	const srcdoc = $derived(buildSrcdoc(shown));

	// Pont : appels de la page vers le classeur.
	function onMessage(e: MessageEvent) {
		if (!frame || e.source !== frame.contentWindow) return;
		const c = parseCall(e.data);
		if (!c) return;
		const target = frame.contentWindow;
		call(c.name, c.args).then(
			(value) => target?.postMessage(replyOf(c.id, { ok: true, value }), '*'),
			(x) => target?.postMessage(replyOf(c.id, { ok: false, error: String(x instanceof Error ? x.message : x) }), '*')
		);
	}

	// Le classeur a changé : la page est prévenue (lattis.on("change", …)).
	let first = true;
	$effect(() => {
		void stamp;
		if (first) {
			first = false;
			return;
		}
		untrack(() => frame?.contentWindow?.postMessage(eventOf('change'), '*'));
	});

	function draft() {
		return view?.state.doc.toString() ?? shown;
	}

	async function doSave() {
		if (locked) {
			onnotice?.(locked);
			return;
		}
		const src = draft();
		try {
			await save(src);
			dirty = false;
			shown = src;
			error = '';
			onnotice?.(`Page « ${name} » enregistrée (Ctrl+Z pour annuler)`);
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	}

	function preview() {
		shown = draft();
	}

	function startEdit() {
		if (locked) {
			onnotice?.(locked);
			return;
		}
		editing = true;
	}

	function stopEdit(keep: boolean) {
		if (!keep) {
			view?.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: source } });
			dirty = false;
		}
		shown = keep ? draft() : source;
		editing = false;
	}

	$effect(() => {
		if (!editing || !host || view) return;
		view = new EditorView({
			parent: host,
			state: EditorState.create({
				doc: untrack(() => shown),
				extensions: [
					lineNumbers(),
					highlightActiveLine(),
					drawSelection(),
					history(),
					bracketMatching(),
					closeBrackets(),
					html(),
					syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
					keymap.of([
						{ key: 'Mod-Enter', run: () => (preview(), true) },
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
						'&.cm-focused': { outline: 'none' },
						'.cm-content': { fontFamily: "Consolas, 'Cascadia Mono', monospace" },
						'.cm-gutters': { background: '#f7f8f9', borderRight: '1px solid #e3e5e8' }
					})
				]
			})
		});
		view.focus();
		return () => {
			view?.destroy();
			view = undefined;
		};
	});

	onMount(() => {
		window.addEventListener('message', onMessage);
		return () => window.removeEventListener('message', onMessage);
	});
</script>

<div class="page">
	<div class="toolbar">
		<span class="title"><Icon name="web" size={14} />{name}</span>
		{#if editing}
			<button class="primary" onclick={doSave} disabled={!!locked}><Icon name="save" size={14} />Enregistrer{dirty ? ' •' : ''}</button>
			<button onclick={preview} title="Afficher le brouillon dans l’aperçu, sans l’enregistrer (Ctrl+Entrée)"><Icon name="play" size={14} />Aperçu</button>
			<button onclick={() => stopEdit(true)} title="Revenir à la page (le brouillon reste à enregistrer)"><Icon name="web" size={14} />Afficher</button>
			{#if dirty}<button onclick={() => stopEdit(false)}>Abandonner les modifications</button>{/if}
			<span class="muted">Ctrl+S : enregistrer · Ctrl+Entrée : aperçu · lattis.call("sheets.list"), lattis.query(sql), lattis.on("change", f)</span>
		{:else}
			<button onclick={startEdit} disabled={!!locked} title={locked || 'Modifier le HTML, le CSS et le JavaScript de la page'}><Icon name="edit" size={14} />Modifier</button>
			<button onclick={() => reloads++} title="Recharger la page"><Icon name="refresh" size={14} />Recharger</button>
			{#if dirty}<span class="muted">Modifications non enregistrées</span>{/if}
		{/if}
		{#if error}<span class="error">{error}</span>{/if}
	</div>
	<div class="body" class:split={editing}>
		{#if editing}<div class="editor" bind:this={host}></div>{/if}
		<!-- Cadre isolé : scripts permis, mais ni même origine, ni formulaires, ni fenêtres. -->
		{#key reloads}<iframe bind:this={frame} title="Page {name}" sandbox="allow-scripts" {srcdoc}></iframe>{/key}
	</div>
</div>

<style>
	.page {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
		background: #fff;
	}
	.toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px;
		padding: 6px 10px;
		border-bottom: 1px solid #e3e5e8;
	}
	.title {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-weight: 600;
		margin-right: 8px;
		--icon-accent: #217346;
	}
	.toolbar button {
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
		white-space: nowrap;
	}
	.toolbar .primary {
		background: #217346;
		color: #fff;
		border-color: #217346;
		--icon-accent: currentColor;
	}
	.toolbar button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.error {
		color: #b42318;
	}
	.body {
		flex: 1;
		display: flex;
		min-height: 0;
	}
	.editor {
		flex: 1;
		min-width: 0;
		overflow: auto;
		border-right: 1px solid #d4d7db;
	}
	iframe {
		flex: 1;
		min-width: 0;
		border: none;
		background: #fff;
	}
</style>
