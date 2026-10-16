// Explication des erreurs de formule, affichée dans la barre d'état quand la
// cellule active est en erreur (Excel le fait par une infobulle).

import { freeKind } from './freecell';

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
				return `#NOM? : ${ref.toUpperCase()} n'existe pas. Écrivez Cell[${ref.toUpperCase().replaceAll('$', '')}] pour une cellule libre, [Colonne] pour la valeur de la ligne dans un tableau, Tableau[Colonne] pour une colonne entière, ou Tableau[Colonne][ligne] pour une valeur.`;
			return "#NOM? : nom inconnu. Vérifiez le nom de la fonction, du tableau, de la colonne ou de la cellule nommée.";
		}
		case '#VALEUR!': {
			for (const m of formula.matchAll(CELL)) {
				const [r, c] = cellPos(m[1], m[2]);
				const t = cellText?.(r, c);
				if (t && freeKind(t) === 'text' && !t.startsWith('#'))
					return `#VALEUR! : Cell[${m[1].toUpperCase()}${m[2]}] contient du texte, pas un nombre ni une date.`;
			}
			// Les adresses Cell[A1] ne sont pas des colonnes de tableau.
			const refs = formula.replace(/cell\[[^\]]*\]/giu, '0');
			if (/[\p{L}\p{N}_]\[[^\]]+\](?!\[)/u.test(refs) && !/^=\s*[\p{L}.]+\(/u.test(refs))
				return '#VALEUR! : une colonne entière Tableau[Colonne] est utilisée là où une seule valeur est attendue. Écrivez Tableau[Colonne][ligne], ou [Colonne] dans une ligne du tableau.';
			return '#VALEUR! : type de valeur inattendu (texte là où un nombre est attendu, ou argument invalide).';
		}
		case '#DIV/0!':
			return '#DIV/0! : division par zéro (ou par une cellule vide).';
		case '#REF!':
			return '#REF! : référence invalide, le tableau ou la colonne a été supprimé ou renommé.';
		case '#N/A':
			return '#N/A : valeur introuvable (REFERENCE, EQUIV…) ; vérifiez la clé cherchée.';
		case '#NOMBRE!':
			return '#NOMBRE! : nombre invalide ou hors limites.';
		case '#DEVERSEMENT!':
			return '#DEVERSEMENT! : le tableau dynamique ne peut pas se déverser, des cellules sont déjà remplies sur son chemin.';
		case '#CALC!':
			return '#CALC! : le tableau dynamique est vide (aucune ligne ne correspond).';
	}
	return '';
}
