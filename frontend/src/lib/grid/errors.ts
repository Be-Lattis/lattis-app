// Explication des erreurs de formule, affichée dans la barre d'état quand la
// cellule active est en erreur (Excel le fait par une infobulle).

import { freeKind } from './freecell';
import { t } from '#lib/i18n/index.js';

const A1 = /(?:^|[^\p{L}\p{N}_@[\]."])(\$?[A-Za-z]{1,3}\$?\d{1,7})(?![\p{L}\p{N}_(])/u;

// Cellule libre de la feuille citée par la formule : Cell[C1] (pas
// Feuille2!Cell[C1], dont le contenu n'est pas connu ici).
const CELL = /(?<![\p{L}\p{N}_!'\]])cell\[\s*([a-z]{1,4})(\d+)\s*\]/giu;

function cellPos(letters: string, digits: string): [number, number] {
	let col = 0;
	for (const ch of letters.toUpperCase()) col = col * 26 + ch.charCodeAt(0) - 64;
	return [Number(digits) - 1, col - 1];
}

/**
 * Phrase qui explique une erreur ; formula : la formule de la cellule ;
 * cellText : texte d'une cellule libre de la feuille (saisie, ou résultat
 * de sa formule), pour nommer celle qui contient du texte.
 */
export function explainError(code: string, formula = '', cellText?: (row: number, col: number) => string | undefined): string {
	switch (code) {
		case '#NOM?': {
			const ref = A1.exec(formula)?.[1];
			if (ref)
				return t('grid.formulaError.name.ref', { ref: ref.toUpperCase(), cell: ref.toUpperCase().replaceAll('$', '') });
			return t('grid.formulaError.name.unknown');
		}
		case '#VALEUR!': {
			for (const m of formula.matchAll(CELL)) {
				const [r, c] = cellPos(m[1], m[2]);
				const text = cellText?.(r, c);
				if (text && freeKind(text) === 'text' && !text.startsWith('#'))
					return t('grid.formulaError.value.text', { cell: `${m[1].toUpperCase()}${m[2]}` });
			}
			// Les adresses Cell[A1] ne sont pas des colonnes de tableau.
			const refs = formula.replace(/cell\[[^\]]*\]/giu, '0');
			if (/[\p{L}\p{N}_]\[[^\]]+\](?!\[)/u.test(refs) && !/^=\s*[\p{L}.]+\(/u.test(refs))
				return t('grid.formulaError.value.column');
			return t('grid.formulaError.value.type');
		}
		case '#DIV/0!':
			return t('grid.formulaError.div0');
		case '#REF!':
			return t('grid.formulaError.ref');
		case '#CIRC!':
			return t('grid.formulaError.circ');
		case '#N/A':
			return t('grid.formulaError.na');
		case '#NOMBRE!':
			return t('grid.formulaError.num');
		case '#DEVERSEMENT!':
			return t('grid.formulaError.spill');
		case '#CALC!':
			return t('grid.formulaError.calc');
	}
	return '';
}
