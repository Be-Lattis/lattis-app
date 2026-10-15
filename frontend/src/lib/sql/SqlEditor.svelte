<script lang="ts">
	// Éditeur SQL (CodeMirror 6) : coloration SQLite, noms de tableaux et de
	// colonnes reconnus mis en couleur, autocomplétion des noms affichés.
	import { onMount } from 'svelte';
	import { EditorState, StateEffect, RangeSetBuilder } from '@codemirror/state';
	import { EditorView, Decoration, ViewPlugin, keymap, lineNumbers, highlightActiveLine, drawSelection, placeholder, type DecorationSet, type ViewUpdate } from '@codemirror/view';
	import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
	import { syntaxHighlighting, HighlightStyle, bracketMatching, indentOnInput } from '@codemirror/language';
	import { autocompletion, closeBrackets, closeBracketsKeymap, completionKeymap, type CompletionContext, type CompletionResult } from '@codemirror/autocomplete';
	import { sql, SQLite } from '@codemirror/lang-sql';
	import { tags } from '@lezer/highlight';
	import { analyze, statementAt, statements, type Catalog } from './names';
	import { complete } from './complete';

	interface Props {
		value: string;
		catalog: Catalog;
		/** Ctrl+Entrée : la sélection, sinon l'instruction sous le curseur. */
		onrun: (sql: string) => void;
		/** Alt+↑ / Alt+↓ : requête précédente / suivante de l'historique. */
		onhistory?: (step: 1 | -1) => string | null;
	}

	let { value = $bindable(), catalog, onrun, onhistory }: Props = $props();

	let host: HTMLDivElement;
	let view: EditorView | undefined;
	const refresh = StateEffect.define<null>();

	const kinds = { table: 'class', column: 'property', keyword: 'keyword', function: 'function' } as const;
	function source(ctx: CompletionContext): CompletionResult | null {
		const c = complete(ctx.state.doc.toString(), ctx.pos, catalog, ctx.explicit);
		if (!c) return null;
		return {
			from: c.from,
			options: c.options.map((o) => ({ label: o.label, apply: o.apply, type: kinds[o.kind], detail: o.detail, boost: o.boost })),
			validFor: /^[\p{L}\p{N}_ ]*$/u
		};
	}

	const marks = {
		table: Decoration.mark({ class: 'cm-slt-table' }),
		alias: Decoration.mark({ class: 'cm-slt-alias' }),
		column: Decoration.mark({ class: 'cm-slt-column' }),
		ambiguous: Decoration.mark({ class: 'cm-slt-ambiguous', attributes: { title: 'Colonne présente dans plusieurs tableaux : préciser le tableau' } })
	};
	function decorate(v: EditorView): DecorationSet {
		const b = new RangeSetBuilder<Decoration>();
		for (const st of statements(v.state.doc.toString())) {
			const a = analyze(st.text, catalog);
			const all = [...a.resolved.map((r) => ({ token: r.token, mark: marks[r.role] })), ...a.ambiguous.map((x) => ({ token: x.token, mark: marks.ambiguous }))].sort((x, y) => x.token - y.token);
			for (const { token, mark } of all) {
				const t = a.tokens[token];
				b.add(st.from + t.from, st.from + t.from + t.text.length, mark);
			}
		}
		return b.finish();
	}
	const names = ViewPlugin.fromClass(
		class {
			decorations: DecorationSet;
			constructor(v: EditorView) {
				this.decorations = decorate(v);
			}
			update(u: ViewUpdate) {
				if (u.docChanged || u.transactions.some((t) => t.effects.some((e) => e.is(refresh)))) this.decorations = decorate(u.view);
			}
		},
		{ decorations: (p) => p.decorations }
	);

	const style = HighlightStyle.define([
		{ tag: tags.keyword, color: '#1d4ed8', fontWeight: '600' },
		{ tag: [tags.string, tags.special(tags.string)], color: '#a31515' },
		{ tag: [tags.number, tags.bool, tags.null], color: '#098658' },
		{ tag: tags.comment, color: '#6b7280', fontStyle: 'italic' },
		{ tag: [tags.operator, tags.punctuation], color: '#4b5563' },
		{ tag: tags.typeName, color: '#7c3aed' }
	]);

	const theme = EditorView.theme({
		'&': { height: '100%', fontSize: '13px', background: '#fff' },
		'&.cm-focused': { outline: 'none' },
		'.cm-scroller': { fontFamily: "Consolas, 'Cascadia Mono', monospace", lineHeight: '1.5' },
		'.cm-gutters': { background: '#fafbfb', color: '#9ca3af', borderRight: '1px solid #e3e5e8' },
		'.cm-activeLine': { background: '#f5f9f6' },
		// Les noms reconnus l'emportent sur la coloration syntaxique (un nom entre
		// guillemets n'est pas une chaîne).
		'.cm-slt-table, .cm-slt-table *': { color: '#217346', fontWeight: '600' },
		'.cm-slt-alias, .cm-slt-alias *': { color: '#217346' },
		'.cm-slt-column, .cm-slt-column *': { color: '#0e7490' },
		'.cm-slt-ambiguous': { textDecoration: 'underline wavy #d97706' },
		'.cm-tooltip.cm-tooltip-autocomplete > ul': { fontFamily: "'Segoe UI', system-ui, sans-serif", fontSize: '13px' },
		'.cm-completionDetail': { color: '#6b7280', fontStyle: 'normal', marginLeft: '12px' },
		'.cm-tooltip-autocomplete ul li[aria-selected]': { background: '#217346', color: '#fff' },
		'.cm-tooltip-autocomplete ul li[aria-selected] .cm-completionDetail': { color: '#e3efe7' }
	});

	/** Exécute la sélection, sinon l'instruction sous le curseur. */
	export function runCurrent(v: EditorView | undefined = view) {
		if (!v) return false;
		const s = v.state.selection.main;
		const doc = v.state.doc.toString();
		onrun(s.empty ? statementAt(doc, s.head).text : doc.slice(s.from, s.to));
		return true;
	}
	function step(dir: 1 | -1) {
		return (v: EditorView) => {
			const next = onhistory?.(dir);
			if (next == null) return false;
			v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: next }, selection: { anchor: next.length } });
			return true;
		};
	}

	onMount(() => {
		view = new EditorView({
			parent: host,
			state: EditorState.create({
				doc: value,
				extensions: [
					lineNumbers(),
					history(),
					drawSelection(),
					indentOnInput(),
					bracketMatching(),
					closeBrackets(),
					highlightActiveLine(),
					placeholder('SELECT … FROM Tableau — Ctrl+Espace pour les noms'),
					sql({ dialect: SQLite, upperCaseKeywords: true }),
					syntaxHighlighting(style),
					names,
					autocompletion({ override: [source], icons: false, activateOnTyping: true }),
					keymap.of([
						{ key: 'Mod-Enter', run: (v) => runCurrent(v) },
						{ key: 'Alt-ArrowUp', run: step(1) },
						{ key: 'Alt-ArrowDown', run: step(-1) },
						...closeBracketsKeymap,
						...completionKeymap,
						...historyKeymap,
						...defaultKeymap,
						indentWithTab
					]),
					theme,
					EditorView.updateListener.of((u) => {
						if (u.docChanged) value = u.state.doc.toString();
					})
				]
			})
		});
		return () => view?.destroy();
	});

	// Valeur changée de l'extérieur (historique, exemple).
	$effect(() => {
		const v = value;
		if (view && v !== view.state.doc.toString()) view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: v } });
	});

	// Nouveaux tableaux ou colonnes : recolorer.
	$effect(() => {
		void catalog;
		view?.dispatch({ effects: refresh.of(null) });
	});

	/** Insère un nom au curseur (clic dans la liste des objets). */
	export function insert(text: string) {
		if (!view) return;
		const s = view.state.selection.main;
		view.dispatch({ changes: { from: s.from, to: s.to, insert: text }, selection: { anchor: s.from + text.length } });
		view.focus();
	}

	export function focus() {
		view?.focus();
	}
</script>

<div class="editor" bind:this={host}></div>

<style>
	.editor {
		height: 100%;
		min-height: 0;
		overflow: hidden;
	}
</style>
