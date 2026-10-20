import { afterEach, describe, expect, it, vi } from 'vitest';
import { flatten, namespaceOf, parseLang, render, resolveLang, shape, type Tree } from './core';
import { errorText, msgCause } from './errors';
import { compareText, decimalSep, fmtDate, fmtDateTime, fmtInteger, fmtNumber, fmtPercent, fmtTime, lang, languageFor, lower, setLang, setSystemLocale, t, tAny } from './index';
import keysFile from './keys.d.ts?raw';

afterEach(async () => {
	await setLang('fr');
	setSystemLocale('');
});

// Séparateur de milliers d'Intl en français : espace fine insécable.
const nnbsp = ' ';

describe('t', () => {
	it('remplace les paramètres et choisit le pluriel', () => {
		expect(t('common.cancel')).toBe('Annuler');
		expect(t('common.rows', { n: 0 })).toBe('aucune ligne');
		expect(t('common.rows', { n: 1 })).toBe('1 ligne');
		// Règle française (CLDR) : 1,5 est au singulier ; # suit la langue.
		expect(t('common.rows', { n: 1.5 })).toBe('1,5 ligne');
		expect(t('common.rows', { n: 1234 })).toBe(`1${nnbsp}234 lignes`);
		expect(t('shell.settings.language.auto', { name: 'English' })).toBe('Langue du système (English)');
	});

	it('rend le français quand la langue n’a pas le texte, la clé quand personne ne l’a', async () => {
		const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
		const error = vi.spyOn(console, 'error').mockImplementation(() => {});
		await setLang('de');
		expect(lang()).toBe('de');
		expect(t('common.cancel')).toBe('Annuler');
		// Pluriel du texte français, nombre écrit à l'allemande.
		expect(t('common.rows', { n: 1234 })).toBe('1.234 lignes');
		expect(warn).toHaveBeenCalled();
		expect(tAny('nulle.part')).toBe('nulle.part');
		expect(error).toHaveBeenCalled();
		warn.mockRestore();
		error.mockRestore();
	});
});

describe('render (syntaxe ICU minimale)', () => {
	const en = { plural: (n: number) => new Intl.PluralRules('en-US').select(n), number: (n: number) => n.toLocaleString('en-US') };
	const zh = { plural: (n: number) => new Intl.PluralRules('zh-CN').select(n), number: (n: number) => String(n) };
	const rows = '{n, plural, =0 {no rows} one {# row} other {# rows}} in “{table}”';
	it('choisit =N, puis la catégorie, puis other', () => {
		expect(render(rows, { n: 0, table: 'T' }, en)).toBe('no rows in “T”');
		expect(render(rows, { n: 1, table: 'T' }, en)).toBe('1 row in “T”');
		expect(render(rows, { n: 1.5, table: 'T' }, en)).toBe('1.5 rows in “T”');
		expect(render(rows, { n: 2000, table: 'T' }, en)).toBe('2,000 rows in “T”');
		expect(render('{n, plural, other {# 行}}', { n: 1 }, zh)).toBe('1 行');
	});
	it('laisse tel quel un paramètre absent et l’apostrophe', () => {
		expect(render("l'onglet « {name} »", {}, en)).toBe("l'onglet « {name} »");
		expect(render('{a} et {b}', { a: 'x', b: 2 }, en)).toBe('x et 2');
	});
	it('décrit la forme d’un message', () => {
		expect(shape('{n, plural, one {# ligne de {table}} other {# lignes}} ({who})')).toEqual({ params: ['table', 'who'], plurals: ['n'] });
	});
});

describe('langue', () => {
	it('lit les codes de langue', () => {
		expect(parseLang('en-US')).toBe('en');
		expect(parseLang('zh-Hans-CN')).toBe('zh');
		expect(parseLang('de_DE.UTF-8')).toBe('de');
		expect(parseLang('it')).toBeNull();
	});
	it('résout le réglage : une langue, sinon le système, sinon l’anglais', () => {
		expect(resolveLang('es', ['fr-FR'])).toBe('es');
		expect(resolveLang('auto', ['it-IT', 'de-AT'])).toBe('de');
		expect(resolveLang('auto', ['it-IT'])).toBe('en');
		expect(resolveLang(undefined, [])).toBe('en');
		setSystemLocale('zh-CN');
		expect(languageFor('auto')).toBe('zh');
		expect(languageFor('fr')).toBe('fr');
	});
});

describe('formateurs', () => {
	const d = new Date(2026, 9, 20, 14, 5, 9);
	it('suivent la langue', async () => {
		const cases: [Parameters<typeof setLang>[0], string, string, string, string, string][] = [
			['fr', `12${nnbsp}345,5`, '20/10/2026', `25${' '}%`, ',', '14:05:09'],
			['en', '12,345.5', '10/20/2026', '25%', '.', '2:05:09 PM'],
			['de', '12.345,5', '20.10.2026', `25${' '}%`, ',', '14:05:09'],
			['es', '12.345,5', '20/10/2026', `25${' '}%`, ',', '14:05:09'],
			['zh', '12,345.5', '2026/10/20', '25%', '.', '14:05:09']
		];
		for (const [l, num, date, pct, sep, time] of cases) {
			await setLang(l);
			expect([l, fmtNumber(12345.5)]).toEqual([l, num]);
			expect([l, fmtDate(d)]).toEqual([l, date]);
			expect([l, fmtPercent(0.25)]).toEqual([l, pct]);
			expect([l, decimalSep()]).toEqual([l, sep]);
			expect([l, fmtTime(d)]).toEqual([l, time]);
		}
	});
	it('en français par défaut (les tests qui attendent le français le restent)', () => {
		expect(fmtInteger(1234.4)).toBe(`1${nnbsp}234`);
		expect(fmtDateTime(d)).toBe('20/10/2026 14:05:09');
		expect(fmtTime(d, { hour: '2-digit', minute: '2-digit' })).toBe('14:05');
		expect(compareText('éte', 'ete', { sensitivity: 'base' })).toBe(0);
		expect(lower('ÉTÉ')).toBe('été');
	});
});

describe('erreurs du moteur', () => {
	it('prend le message traduit de la cause', () => {
		const cause = { id: 'table.nameTaken', args: { name: 'T' }, message: 'table: “T” already exists' };
		expect(errorText(new Error('tableau : « T » existe déjà', { cause }))).toBe('table: “T” already exists');
		// Bureau : la cause arrive parfois en texte JSON.
		expect(msgCause({ cause: JSON.stringify(cause) })?.id).toBe('table.nameTaken');
		expect(errorText(new Error('brut'))).toBe('brut');
		expect(errorText('texte')).toBe('texte');
	});
});

// ------------------------------------------------------- cohérence des fichiers

const files = import.meta.glob<Tree>('./messages/*/*.json', { eager: true, import: 'default' });
const byLang = new Map<string, Map<string, Record<string, string>>>();
for (const [path, tree] of Object.entries(files)) {
	const { lang: l, ns } = namespaceOf(path);
	if (!byLang.has(l)) byLang.set(l, new Map());
	byLang.get(l)!.set(ns, flatten(tree, ns));
}
const fr = Object.assign({}, ...byLang.get('fr')!.values()) as Record<string, string>;

describe('catalogues', () => {
	it('chaque langue a les mêmes fichiers que le français', () => {
		const ns = [...byLang.get('fr')!.keys()].sort();
		expect(ns).toEqual(expect.arrayContaining(['common', 'errors', 'grid', 'shell']));
		for (const [l, m] of byLang) expect([l, [...m.keys()].sort()]).toEqual([l, ns]);
	});
	it('les autres langues n’ont que des clés françaises, de même forme', () => {
		for (const [l, m] of byLang) {
			if (l === 'fr') continue;
			for (const msgs of m.values())
				for (const [key, text] of Object.entries(msgs)) {
					expect([l, key, key in fr]).toEqual([l, key, true]);
					expect([l, key, shape(text)]).toEqual([l, key, shape(fr[key])]);
				}
		}
	});
	it('keys.d.ts est à jour (npm run i18n:types)', () => {
		const declared = new Map<string, string[]>();
		for (const m of keysFile.matchAll(/^\t'([^']+)': (.*);\r?$/gm)) {
			const ps = m[2] === 'Record<never, never>' ? [] : [...m[2].matchAll(/(\w+): number \| string/g)].map((x) => x[1]);
			declared.set(m[1], ps);
		}
		expect([...declared.keys()]).toEqual(Object.keys(fr).sort());
		for (const [key, ps] of declared) {
			const s = shape(fr[key]);
			expect([key, ps]).toEqual([key, [...new Set([...s.params, ...s.plurals])].sort()]);
		}
	});
});
