// Aide à la saisie des noms de contrôles dans l'éditeur de scripts :
// complétion du premier argument des fonctions controls.* qui prennent un
// contrôle (controls.get("…"), controls.on{control = "…"}…).

import type { Completion, CompletionContext, CompletionResult } from '@codemirror/autocomplete';
import { CONTROL_KINDS, type ControlKind } from '#lib/grid/buttons.js';
import { apiReference } from './api.gen';

/** Contrôle du classeur, tel que le renvoie controls.list. */
export interface ControlInfo {
	id: number;
	name: string;
	kind: string;
	label: string;
	sheet: string;
}

/** Libellé français du genre public (« button », « input »…). */
export function kindLabel(kind: string): string {
	return CONTROL_KINDS[(kind === 'button' ? '' : kind) as ControlKind] ?? kind;
}

/** Fonctions controls.* dont le premier paramètre est un contrôle (d'après la référence). */
export const CONTROL_FUNCS = new Set(
	apiReference.filter((e) => /^controls\.\w+\(\[?control\b/.test(e.signature)).map((e) => e.name.slice('controls.'.length))
);

/** Texte à insérer pour désigner un contrôle : son nom, à défaut son identifiant. */
export function controlRef(c: ControlInfo): string {
	return c.name ? JSON.stringify(c.name) : String(c.id);
}

/**
 * Où compléter un nom de contrôle dans `before` (le texte de la ligne avant
 * le curseur) : `from` est le décalage du début du mot à remplacer, `quote`
 * le guillemet ouvrant (vide si aucun n'est encore tapé).
 */
export function controlArgAt(before: string): { from: number; quote: string; typed: string } | null {
	const m = /controls\.(\w+)\s*(?:\(\s*|\{\s*control\s*=\s*)(?:(["'])([^"'\\]*))?$/.exec(before);
	if (!m || !CONTROL_FUNCS.has(m[1])) return null;
	const quote = m[2] ?? '';
	const typed = m[3] ?? '';
	return { from: before.length - typed.length - quote.length, quote, typed };
}

/** Source de complétion CodeMirror ; `list` donne les contrôles du classeur. */
export function controlCompletions(list: () => Promise<ControlInfo[]>) {
	return async (ctx: CompletionContext): Promise<CompletionResult | null> => {
		const line = ctx.state.doc.lineAt(ctx.pos);
		const at = controlArgAt(line.text.slice(0, ctx.pos - line.from));
		if (!at) return null;
		// Sans guillemet tapé, seulement à la demande (Ctrl+Espace) ou juste après « ( ».
		if (!at.quote && !ctx.explicit && !/[({=]\s*$/.test(line.text.slice(0, ctx.pos - line.from))) return null;
		let controls: ControlInfo[];
		try {
			controls = await list();
		} catch {
			return null;
		}
		if (!controls.length) return null;
		const q = at.quote || '"';
		// La fermeture automatique a pu poser le guillemet fermant juste après le curseur.
		const closed = at.quote && ctx.state.sliceDoc(ctx.pos, ctx.pos + 1) === at.quote;
		const options: Completion[] = controls.map((c) => {
			const name = c.name || String(c.id);
			const apply = c.name ? q + c.name.replaceAll('\\', '\\\\').replaceAll(q, '\\' + q) + (closed ? '' : q) : String(c.id);
			return {
				label: c.name ? q + name + q : name,
				apply: (view, _c, from, to) => {
					const end = closed && !c.name ? to + 1 : to;
					view.dispatch({
						changes: { from, to: end, insert: apply },
						selection: { anchor: from + apply.length + (closed && c.name ? 1 : 0) }
					});
				},
				detail: `${kindLabel(c.kind)} · ${c.sheet}`,
				info: c.label && c.label !== c.name ? `Libellé : ${c.label}` : undefined,
				type: 'variable'
			};
		});
		return { from: line.from + at.from, options, validFor: /^["']?[^"'\\]*$/ };
	};
}
