import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { FILL, localeFor, parseCode, plain } from './numcode';

// Cas partagés avec le moteur Go (internal/numfmt) : les deux rendent le même texte.
interface Case {
	code: string;
	value?: number;
	text_value?: string;
	lang?: string;
	text?: string;
	color?: string;
	ok?: boolean;
}
const cases = JSON.parse(readFileSync(new URL('../../../../internal/numfmt/testdata/cases.json', import.meta.url), 'utf8')) as Case[];

describe('codes de format (cas partagés avec Go)', () => {
	for (const c of cases) {
		it(`${c.code} ${c.text_value ?? c.value} ${c.lang ?? ''}`, () => {
			const f = parseCode(c.code);
			const r = c.text_value !== undefined ? f.text(c.text_value) : f.number(c.value!, localeFor(c.lang ?? 'fr'));
			if (c.ok === false) {
				expect(r.ok).toBe(false);
				return;
			}
			expect(r.text).toBe(c.text);
			expect(r.color).toBe(c.color);
		});
	}
});

describe('codes de format', () => {
	it('retire le remplissage', () => {
		expect(plain(' ' + FILL + ' 12 €')).toBe('  12 €');
	});
	it('reconnaît dates et texte', () => {
		expect(parseCode('jj/mm/aaaa').isDate()).toBe(true);
		expect(parseCode('[h]:mm').isDate()).toBe(true);
		expect(parseCode('# ##0').isDate()).toBe(false);
		expect(parseCode('@').isText()).toBe(true);
	});
});
