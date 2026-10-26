<script lang="ts">
	// Éditeur SQL (CodeMirror 6) : coloration SQLite, noms de tableaux et de
	// colonnes reconnus mis en couleur, autocomplétion des noms affichés.
	import { onMount } from 'svelte';
	import { EditorState, StateEffect, RangeSetBuilder } from '@codemirror/state';
	import { EditorView, Decoration, ViewPlugin, keymap, lineNumbers, drawSelection, placeholder, type DecorationSet, type ViewUpdate } from '@codemirror/view';
	import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
	import { syntaxHighlighting, HighlightStyle, bracketMatching, indentOnInput } from '@codemirror/language';
	import { autocompletion, closeCompletion, closeBrackets, closeBracketsKeymap, completionKeymap, type CompletionContext, type CompletionResult } from '@codemirror/autocomplete';
	import { sql, SQLite } from '@codemirror/lang-sql';
	import { setDiagnostics } from '@codemirror/lint';
	import { tags } from '@lezer/highlight';
	import { analyze, errorSpan, statementAt, statements, type Catalog } from './names';
	import { complete } from './complete';
	import { t } from '#lib/i18n/index.js';

	interface Props {
		value: string;
		catalog: Catalog;
		/** Ctrl+Entrée : la sélection, sinon l'instruction sous le curseur. */
		onrun: (sql: string) => void;
		/** Alt+↑ / Alt+↓ : requête précédente / suivante de l'historique. */
		onhistory?: (step: 1 | -1) => string | null;
		/** F1 : référence SQL du mot sous le curseur ('' : aucun mot). */
		onhelp?: (word: string) => void;
	}

	let { value = $bindable(), catalog, onrun, onhistory, onhelp }: Props = $props();

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
		ambiguous: Decoration.mark({ class: 'cm-slt-ambiguous', attributes: { title: t('sql.editor.ambiguous') } })
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

	// Ligne active surlignée seulement sans sélection : la sélection est
	// dessinée sous le texte, et le fond de la ligne active la masquerait.
	const lineMark = Decoration.line({ class: 'cm-activeLine' });
	function activeLine(v: EditorView): DecorationSet {
		const s = v.state.selection;
		if (s.ranges.some((r) => !r.empty)) return Decoration.none;
		return Decoration.set([lineMark.range(v.state.doc.lineAt(s.main.head).from)]);
	}
	const highlightActiveLine = ViewPlugin.fromClass(
		class {
			decorations: DecorationSet;
			constructor(v: EditorView) {
				this.decorations = activeLine(v);
			}
			update(u: ViewUpdate) {
				if (u.docChanged || u.selectionSet) this.decorations = activeLine(u.view);
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
		// Sélection bien visible, éditeur actif ou non.
		'.cm-selectionBackground, &.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground': { background: '#b9dcc5' },		// Les noms reconnus l'emportent sur la coloration syntaxique (un nom entre
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
		closeCompletion(v);
		const s = v.state.selection.main;
		const doc = v.state.doc.toString();
		const picked = doc.slice(s.from, s.to);
		const st = picked.trim() ? { from: s.from, text: picked } : statementAt(doc, s.head);
		ran = { from: st.from, text: st.text };
		onrun(st.text);
		return true;
	}

	// Dernière requête exécutée et sa position, pour y souligner l'erreur.
	let ran: { from: number; text: string } | null = null;

	/** Souligne dans l'éditeur l'endroit désigné par l'erreur ('' efface). */
	export function showError(message: string) {
		if (!view) return;
		const span = message && ran ? errorSpan(ran.text, message) : null;
		const at = span && ran ? { from: ran.from + span.from, to: ran.from + span.to } : null;
		const ok = !!at && at.to <= view.state.doc.length;
		view.dispatch(setDiagnostics(view.state, at && ok ? [{ from: at.from, to: at.to, severity: 'error', message }] : []));
	}
	function step(dir: 1 | -1) {
		return (v: EditorView) => {
			const next = onhistory?.(dir);
			if (next == null) return false;
			v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: next }, selection: { anchor: next.length } });
			return true;
		};
	}

	/** F1 : le mot sous le curseur (ou juste avant) part à la référence. */
	function help(v: EditorView) {
		if (!onhelp) return false;
		const at = v.state.selection.main.head;
		const w = v.state.wordAt(at) ?? (at > 0 ? v.state.wordAt(at - 1) : null);
		onhelp(w ? v.state.sliceDoc(w.from, w.to) : '');
		return true;
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
					highlightActiveLine,
					placeholder(t('sql.editor.placeholder')),
					sql({ dialect: SQLite, upperCaseKeywords: true }),
					syntaxHighlighting(style),
					names,
					autocompletion({ override: [source], icons: false, activateOnTyping: true }),
					keymap.of([
						{ key: 'Mod-Enter', run: (v) => runCurrent(v) },
						{ key: 'Alt-ArrowUp', run: step(1) },
						{ key: 'Alt-ArrowDown', run: step(-1) },
						{ key: 'F1', run: help },
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

	/**
	 * Insère une requête comme instruction à part (lignes vides autour) : à
	 * la place de la sélection, sinon après l'instruction sous le curseur ;
	 * le curseur à sa fin, Ctrl+Entrée l'exécute seule. Ctrl+Z la retire.
	 */
	export function insertBlock(text: string) {
		if (!view) {
			value = value.trim() ? `${value.trimEnd()}\n\n${text}` : text;
			return;
		}
		const doc = view.state.doc.toString();
		let { from, to } = view.state.selection.main;
		if (from === to) {
			const st = statementAt(doc, from);
			if (st.text && from >= st.from && from <= st.to) from = to = st.to + (doc[st.to] === ';' ? 1 : 0);
		}
		const before = doc.slice(0, from);
		const after = doc.slice(to);
		const pre = !before.trim() ? '' : before.endsWith('\n\n') ? '' : before.endsWith('\n') ? '\n' : '\n\n';
		const post = !after.trim() ? '' : after.startsWith('\n\n') ? '' : after.startsWith('\n') ? '\n' : '\n\n';
		const insert = pre + text + post;
		view.dispatch({ changes: { from, to, insert }, selection: { anchor: from + pre.length + text.length }, scrollIntoView: true });
		view.focus();
	}

	/** Remplace tout le contenu (exemple, favori) ; Ctrl+Z le rend. */
	export function load(text: string) {
		if (!view) {
			value = text;
			return;
		}
		view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text }, selection: { anchor: 0 }, scrollIntoView: true });
		view.focus();
	}

	/** Texte sélectionné, sinon tout le contenu (pour les favoris). */
	export function selectedOrAll(): string {
		if (!view) return value;
		const s = view.state.selection.main;
		const picked = view.state.sliceDoc(s.from, s.to);
		return picked.trim() ? picked : view.state.doc.toString();
	}

	export function focus() {
		view?.focus();
	}
</script>

<!-- data-own-f1 : F1 y ouvre la référence SQL, pas l'aide (+page.svelte). -->
<div class="editor" bind:this={host} data-own-f1={onhelp ? '' : undefined}></div>

<style>
	.editor {
		height: 100%;
		min-height: 0;
		overflow: hidden;
	}
</style>
