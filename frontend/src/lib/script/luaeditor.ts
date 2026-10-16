// Extensions CodeMirror de l'éditeur de scripts : linter Lua (lualint.ts) et
// bulle d'aide au survol d'une fonction de la référence (reference.ts).

import type { Extension } from '@codemirror/state';
import { EditorView, hoverTooltip } from '@codemirror/view';
import { linter, lintGutter, type Diagnostic } from '@codemirror/lint';
import { lintLua } from './lualint';
import { byName } from './reference';

const luaLinter = linter(
	(view): Diagnostic[] =>
		lintLua(view.state.doc.toString()).map((d) => ({ from: d.from, to: d.to, severity: d.severity, message: d.message, source: 'Lua' })),
	{ delay: 300 }
);

// Nom sous le pointeur, avec sa bibliothèque : « db.query », « print », « for ».
const word = /[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?/g;

const luaHover = hoverTooltip((view, pos) => {
	const line = view.state.doc.lineAt(pos);
	for (const m of line.text.matchAll(word)) {
		const from = line.from + m.index!;
		const to = from + m[0].length;
		if (pos < from || pos > to) continue;
		const entry = byName.get(m[0]);
		if (!entry) return null;
		return {
			pos: from,
			end: to,
			above: true,
			create() {
				const dom = document.createElement('div');
				dom.className = 'cm-lua-help';
				const sig = document.createElement('code');
				sig.textContent = entry.signature;
				const sum = document.createElement('div');
				sum.textContent = entry.summary;
				dom.append(sig, sum);
				return { dom };
			}
		};
	}
	return null;
});

const theme = EditorView.baseTheme({
	'.cm-lua-help': { padding: '4px 8px', maxWidth: '440px', font: "12px 'Segoe UI', system-ui, sans-serif" },
	'.cm-lua-help code': { display: 'block', marginBottom: '2px', fontFamily: "Consolas, 'Cascadia Mono', monospace", color: '#14532d' }
});

/** Linter, marges d'erreur et aide au survol. */
export function luaTools(): Extension[] {
	return [luaLinter, lintGutter(), luaHover, theme];
}
