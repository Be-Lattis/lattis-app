// Info-bulles de touches (KeyTips) du ruban, comme Excel : Alt (ou F10)
// affiche une lettre sur chaque onglet, puis sur chaque commande de
// l'onglet choisi. Les codes ne sont jamais préfixes l'un de l'autre : une
// touche tapée désigne une commande, ou annonce un code à deux touches.

import { t, type MessageKey } from '#lib/i18n/index.js';

/** Lettres des onglets du ruban (celles d'Excel quand elles existent), par identifiant d'onglet. */
export const TAB_TIPS: Record<string, string> = {
	file: 'F',
	home: 'H',
	insert: 'N',
	pageLayout: 'P',
	data: 'A',
	formulas: 'M',
	review: 'R',
	view: 'W',
	ai: 'I',
	sql: 'Q',
	script: 'L',
	help: 'Y'
};

/**
 * Codes d'Excel repris pour les commandes qui ont un équivalent (onglet
 * Accueil surtout), par clé du libellé de la commande (ribbon.json) ; un
 * libellé suivi de « … » est le lanceur de son groupe.
 */
const COMMAND_CODES: [label: MessageKey, code: string, launcher?: boolean][] = [
	['ribbon.cmd.undo.label', 'Z'],
	['ribbon.cmd.redo.label', 'Y'],
	['ribbon.cmd.paste.label', 'V'],
	['ribbon.cmd.cut.label', 'X'],
	['ribbon.cmd.copy.label', 'C'],
	['ribbon.cmd.painter.label', 'FP'],
	['ribbon.group.font', 'FF'],
	['ribbon.fontSize', 'FS'],
	['ribbon.cmd.bold.label', '1'],
	['ribbon.cmd.italic.label', '2'],
	['ribbon.cmd.underline.label', '3'],
	['ribbon.cmd.fontColor.label', 'FC'],
	['ribbon.cmd.fillColor.label', 'H'],
	['ribbon.group.font', 'FN', true],
	['ribbon.cmd.alignTop.label', 'AT'],
	['ribbon.cmd.alignMiddle.label', 'AM'],
	['ribbon.cmd.alignBottom.label', 'AB'],
	['ribbon.cmd.alignLeft.label', 'AL'],
	['ribbon.cmd.alignCenter.label', 'AC'],
	['ribbon.cmd.alignRight.label', 'AR'],
	['ribbon.cmd.wrap.label', 'W'],
	['ribbon.cmd.merge.label', 'M'],
	['ribbon.group.alignment', 'FA', true],
	['ribbon.cmd.numberFormat.label', 'N'],
	['ribbon.cmd.currency.label', 'AN'],
	['ribbon.cmd.percent.label', 'P'],
	['ribbon.cmd.decimalMore.label', '0'],
	['ribbon.cmd.decimalLess.label', '9'],
	['ribbon.group.number', 'FM', true],
	['ribbon.cmd.conditional.label', 'L'],
	['ribbon.cmd.formatAsTable.label', 'T'],
	['ribbon.cmd.insert.label', 'I'],
	['ribbon.cmd.delete.label', 'D'],
	['ribbon.cmd.format.label', 'O'],
	['ribbon.cmd.fill.label', 'FI'],
	['ribbon.cmd.clear.label', 'E'],
	['ribbon.cmd.sortFilter.label', 'S'],
	['ribbon.cmd.find.label', 'FD'],
	['ribbon.cmd.pivot.label', 'V'],
	['ribbon.cmd.table.label', 'T'],
	['ribbon.cmd.autoSum.label', 'U'],
	['ribbon.cmd.insertFunction.label', 'F'],
	['ribbon.cmd.gridlines.label', 'VG'],
	['ribbon.cmd.freeze.label', 'F'],
	['ribbon.cmd.zoom.label', 'Q'],
	['ribbon.cmd.theme.label', 'TH'],
	// Mise en page
	['ribbon.cmd.margins.label', 'M'],
	['ribbon.cmd.orientation.label', 'O'],
	['ribbon.cmd.size.label', 'SZ'],
	['ribbon.cmd.printArea.label', 'R'],
	['ribbon.cmd.breaks.label', 'B'],
	['ribbon.cmd.printTitles.label', 'I'],
	['ribbon.cmd.width.label', 'W'],
	['ribbon.cmd.height.label', 'H'],
	['ribbon.cmd.scale.label', 'SC'],
	['ribbon.cmd.showGridlines.label', 'VG'],
	['ribbon.cmd.printGridlines.label', 'PG'],
	['ribbon.cmd.showHeadings.label', 'VH'],
	['ribbon.cmd.printHeadings.label', 'PH']
];

/** Codes préférés des commandes, par libellé dans la langue courante (le premier l'emporte). */
export function commandTips(): Record<string, string> {
	const out: Record<string, string> = {};
	for (const [key, code, launcher] of COMMAND_CODES) {
		const label = launcher ? `${t(key)} …` : t(key);
		out[label] ??= code;
	}
	return out;
}

/** Lettres et chiffres d'un libellé, sans accents, en majuscules. */
export function keyChars(label: string): string {
	return label
		.normalize('NFD')
		.replace(/[̀-ͯ]/g, '')
		.toUpperCase()
		.replace(/[^A-Z0-9]/g, '');
}

/** Normalise une touche tapée (é → E) ; '' si ce n'est ni une lettre ni un chiffre. */
export function keyOf(key: string): string {
	return key.length === 1 ? keyChars(key) : '';
}

/** Initiales des mots après le premier (« Trier Z → A » → ZA). */
function initials(label: string): string {
	return label
		.split(/[\s'’/-]+/)
		.slice(1)
		.map((w) => keyChars(w)[0] ?? '')
		.join('');
}

const ALPHA = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';

/** Codes à deux touches possibles pour un libellé, du plus parlant au plus arbitraire. */
function* candidates(label: string): Generator<string> {
	const w = keyChars(label);
	const c = w[0] ?? 'X';
	for (const s of initials(label)) yield c + s;
	for (const s of w.slice(1)) yield c + s;
	for (const d of '123456789') yield c + d;
	for (let i = 1; i < w.length; i++) for (const s of w.slice(i + 1)) yield w[i] + s;
	for (const a of ALPHA) for (const b of ALPHA + '0123456789') yield a + b;
}

/**
 * Codes des libellés (même ordre) : le code préféré s'il est libre, sinon
 * l'initiale seule quand aucun autre libellé ne commence par elle, sinon
 * l'initiale et une lettre suivante. Aucun code n'est préfixe d'un autre.
 */
export function assignKeyTips(labels: string[], preferred: Record<string, string> = {}): string[] {
	const out = labels.map(() => '');
	const used: string[] = [];
	const free = (t: string) => !!t && used.every((u) => !u.startsWith(t) && !t.startsWith(u));
	const take = (i: number, t: string) => {
		out[i] = t;
		used.push(t);
	};
	labels.forEach((l, i) => {
		const p = preferred[l];
		if (p && free(p)) take(i, p);
	});
	// Libellés restants, groupés par initiale (dans l'ordre d'apparition).
	const groups = new Map<string, number[]>();
	labels.forEach((l, i) => {
		if (out[i]) return;
		const c = keyChars(l)[0] ?? 'X';
		groups.set(c, [...(groups.get(c) ?? []), i]);
	});
	for (const [c, idx] of groups) {
		if (idx.length === 1 && free(c)) {
			take(idx[0], c);
			continue;
		}
		for (const i of idx) {
			for (const t of candidates(labels[i])) {
				if (free(t)) {
					take(i, t);
					break;
				}
			}
		}
	}
	return out;
}

/** Effet des touches tapées : code atteint (indice) ou -1, et s'il reste des codes qui commencent ainsi. */
export function matchTip(tips: string[], typed: string): { hit: number; live: boolean } {
	return { hit: typed ? tips.indexOf(typed) : -1, live: tips.some((t) => t.startsWith(typed)) };
}
