import { describe, expect, it } from 'vitest';
import { Catalog, translate, type SqlObject } from './names';
import cases from './translate.cases.json';

// Non-régression de la traduction des noms affichés (onglet SQL) : chaque
// requête du fichier garde exactement sa traduction (out) ou son refus
// (refused). Le même fichier sert au test Go internal/canvas/sqltranslate_test.go,
// qui exécute chaque traduction sur un vrai classeur aux mêmes noms internes :
// une traduction qui « passe » ici doit aussi tourner dans SQLite.
// before : traduction d'avant une correction voulue (colonne interne c_N
// commune à deux tableaux, que SQLite trouvait ambiguë ou rattachait au
// mauvais tableau).

interface Case {
	name: string;
	sql: string;
	out?: string;
	refused?: string;
	before?: string;
}

const cat = new Catalog(cases.objects as SqlObject[]);

describe('traduction des noms (non-régression)', () => {
	for (const c of cases.cases as Case[]) {
		it(c.name, () => {
			if (c.refused !== undefined) {
				expect(() => translate(c.sql, cat)).toThrow(c.refused);
				return;
			}
			expect(translate(c.sql, cat).sql).toBe(c.out);
		});
	}
});
