// Coloration syntaxique des scripts Lua (onglet Script) : mots-clés, chaînes
// (longues comprises), nombres, commentaires, et les noms de l'API Lattis
// (db, sheets, print…) tirés de la référence, dans les couleurs de l'éditeur SQL.

import { HighlightStyle, StreamLanguage, syntaxHighlighting, type StringStream } from '@codemirror/language';
import type { Extension } from '@codemirror/state';
import { tags } from '@lezer/highlight';
import { globals } from './reference';

const keywords = new Set([
	'and',
	'break',
	'do',
	'else',
	'elseif',
	'end',
	'for',
	'function',
	'goto',
	'if',
	'in',
	'local',
	'not',
	'or',
	'repeat',
	'return',
	'then',
	'until',
	'while'
]);
const atoms = new Set(['true', 'false', 'nil']);

interface State {
	/** Chaîne ou commentaire long en cours : nombre de « = » de son ouverture. */
	long: { level: number; comment: boolean } | null;
	/** Chaîne courte ouverte (« \ » en fin de ligne la prolonge). */
	quote: string | null;
	/** Nom lu juste après « function » : c'est une définition. */
	afterFunction: boolean;
}

// Ouverture d'un bloc long « [[ » ou « [==[ » : son niveau, ou -1.
function longOpen(stream: StringStream): number {
	const m = stream.match(/^\[(=*)\[/);
	return m ? (m as RegExpMatchArray)[1].length : -1;
}

function readLong(stream: StringStream, state: State): string {
	const close = ']' + '='.repeat(state.long!.level) + ']';
	const comment = state.long!.comment;
	while (!stream.eol()) {
		if (stream.match(close)) {
			state.long = null;
			break;
		}
		stream.next();
	}
	return comment ? 'comment' : 'string';
}

function readQuote(stream: StringStream, state: State): string {
	let escaped = false;
	let ch: string | void;
	while ((ch = stream.next()) != null) {
		if (ch === state.quote && !escaped) {
			state.quote = null;
			return 'string';
		}
		escaped = !escaped && ch === '\\';
	}
	// Fin de ligne : la chaîne continue seulement après un « \ ».
	if (!escaped) state.quote = null;
	return 'string';
}

export const lua = StreamLanguage.define<State>({
	name: 'lua',
	startState: () => ({ long: null, quote: null, afterFunction: false }),
	token(stream, state) {
		if (state.long) return readLong(stream, state);
		if (state.quote) return readQuote(stream, state);
		if (stream.eatSpace()) return null;
		if (stream.match('--')) {
			const level = longOpen(stream);
			if (level >= 0) {
				state.long = { level, comment: true };
				return readLong(stream, state);
			}
			stream.skipToEnd();
			return 'comment';
		}
		const level = longOpen(stream);
		if (level >= 0) {
			state.long = { level, comment: false };
			return readLong(stream, state);
		}
		const ch = stream.peek()!;
		if (ch === '"' || ch === "'") {
			state.quote = stream.next()!;
			return readQuote(stream, state);
		}
		if (stream.match(/^0[xX][0-9a-fA-F]+/) || stream.match(/^(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?/)) return 'number';
		if (stream.match(/^[A-Za-z_][A-Za-z0-9_]*/)) {
			const word = stream.current();
			const prev = stream.start > 0 ? stream.string.charAt(stream.start - 1) : '';
			if (keywords.has(word)) {
				state.afterFunction = word === 'function';
				return 'keyword';
			}
			if (atoms.has(word)) return 'atom';
			if (state.afterFunction) {
				state.afterFunction = false;
				return 'def';
			}
			// Champ après « . » ou « : » : db.query → query est une propriété.
			if (prev === '.' || prev === ':') return stream.match(/^\s*\(/, false) ? 'propertyName' : 'property';
			if (globals.has(word)) return 'builtin';
			return stream.match(/^\s*\(/, false) ? 'variableName.function' : 'variableName';
		}
		state.afterFunction = false;
		if (stream.match(/^(\.\.\.?|==|~=|<=|>=|::|[-+*/%^#<>=])/)) return 'operator';
		stream.next();
		return 'punctuation';
	},
	languageData: {
		commentTokens: { line: '--', block: { open: '--[[', close: ']]' } }
	},
	tokenTable: {
		builtin: tags.standard(tags.variableName),
		propertyName: tags.function(tags.propertyName),
		'variableName.function': tags.function(tags.variableName)
	}
});

const style = HighlightStyle.define([
	{ tag: tags.keyword, color: '#1d4ed8', fontWeight: '600' },
	{ tag: tags.string, color: '#a31515' },
	{ tag: [tags.number, tags.atom, tags.bool, tags.null], color: '#098658' },
	{ tag: tags.comment, color: '#6b7280', fontStyle: 'italic' },
	{ tag: tags.operator, color: '#4b5563' },
	{
		tag: tags.standard(tags.variableName),
		color: '#7c3aed',
		fontWeight: '600'
	},
	{
		tag: [tags.function(tags.propertyName), tags.function(tags.variableName), tags.definition(tags.variableName)],
		color: '#795e26'
	},
	{ tag: tags.propertyName, color: '#0e7490' }
]);

/** Langage Lua et sa coloration. */
export function luaSyntax(): Extension[] {
	return [lua, syntaxHighlighting(style)];
}
