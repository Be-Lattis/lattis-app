import { describe, expect, it } from 'vitest';
import { autoLink, isEmail, isLinkClick, linkFromInput, linkKind, onLinkText, parsePlace, placeLink, quoteSheet, HOLD_MS } from './hyperlink';
import { parseClipHtml, toClipHtml } from './clipboardHtml';
import { cellCss } from '#lib/print/cellcss.js';

describe('liens copiés, collés et imprimés', () => {
	it('copie HTML : <a href> pour un lien web ou e-mail', () => {
		const html = toClipHtml([['Site', 'Ici']], (_r, c) => (c === 0 ? { ln: 'https://lattis.be/?a=1&b=2' } : { ln: '#B3' }));
		expect(html).toContain('<a href="https://lattis.be/?a=1&amp;b=2">Site</a>');
		expect(html).not.toContain('href="#B3"');
	});
	it('collage HTML : le lien devient la propriété ln, les schémas refusés sont laissés', () => {
		const c = parseClipHtml('<table><tr><td><a href="https://a.fr/x">A</a></td><td><a href="javascript:alert(1)">B</a></td><td><a href="mailto:j@b.fr">C</a></td></tr></table>');
		expect(c?.rows[0]).toEqual(['A', 'B', 'C']);
		expect(c?.styles[0][0]?.ln).toBe('https://a.fr/x');
		expect(c?.styles[0][1]?.ln).toBeUndefined();
		expect(c?.styles[0][2]?.ln).toBe('mailto:j@b.fr');
	});
	it('impression : bleu souligné', () => {
		expect(cellCss({ ln: 'https://a.fr' })).toBe('color:#0563c1;text-decoration:underline');
		expect(cellCss({ ln: 'https://a.fr', fg: '#ff0000' })).toContain('color:#ff0000');
		expect(cellCss({ ln: 'javascript:x' })).toBe('');
	});
});

describe('autoLink (adresse tapée dans une cellule)', () => {
	it('reconnaît les adresses web et e-mail', () => {
		expect(autoLink('https://lattis.be/aide')).toBe('https://lattis.be/aide');
		expect(autoLink('http://exemple.fr')).toBe('http://exemple.fr');
		expect(autoLink('www.lattis.be')).toBe('http://www.lattis.be');
		expect(autoLink(' jean.dupont@exemple.fr ')).toBe('mailto:jean.dupont@exemple.fr');
		expect(autoLink('mailto:a@b.fr')).toBe('mailto:a@b.fr');
	});
	it('laisse le reste', () => {
		for (const s of ['', 'bonjour', 'lattis.be', 'voir https://a.fr', '=LIEN_HYPERTEXTE("https://a.fr")', 'javascript:alert(1)', 'file:///C:/x', 'a@b', '12,5', 'https://', 'www.x']) {
			expect(autoLink(s), s).toBeNull();
		}
	});
});

describe('linkFromInput (dialogue)', () => {
	it('complète et vérifie', () => {
		expect(linkFromInput('web', 'lattis.be')).toBe('http://lattis.be');
		expect(linkFromInput('web', 'https://a.fr/a b')).toBe('https://a.fr/a%20b');
		expect(linkFromInput('web', 'jean@exemple.fr')).toBe('mailto:jean@exemple.fr');
		expect(linkFromInput('mail', 'jean@exemple.fr')).toBe('mailto:jean@exemple.fr');
		expect(linkFromInput('mail', 'mailto:jean@exemple.fr?subject=Bonjour')).toBe('mailto:jean@exemple.fr?subject=Bonjour');
	});
	it('refuse les autres schémas', () => {
		for (const s of ['javascript:alert(1)', 'JAVASCRIPT:x', 'file:///etc/passwd', 'data:text/html,x', 'vbscript:x', 'bonjour', '']) {
			expect(linkFromInput('web', s), s).toBeNull();
		}
		expect(linkFromInput('mail', 'pas une adresse')).toBeNull();
	});
});

describe('linkKind', () => {
	it('classe les adresses enregistrées', () => {
		expect(linkKind('https://a.fr')).toBe('web');
		expect(linkKind('mailto:a@b.fr')).toBe('mail');
		expect(linkKind("#'Ma feuille'!B3")).toBe('place');
		expect(linkKind('#Ventes')).toBe('place');
		expect(linkKind('javascript:alert(1)')).toBeNull();
		expect(linkKind('file:///x')).toBeNull();
		expect(linkKind('#A1 B2')).toBeNull();
		expect(linkKind(undefined)).toBeNull();
	});
	it('isEmail', () => {
		expect(isEmail('a.b@c.fr')).toBe(true);
		expect(isEmail('a@b')).toBe(false);
		expect(isEmail('a b@c.fr')).toBe(false);
	});
});

describe('adresses dans le classeur', () => {
	it('lit et écrit', () => {
		expect(parsePlace('#B3')).toEqual({ ref: 'B3' });
		expect(parsePlace('#$b$3:c4')).toEqual({ ref: 'B3:C4' });
		expect(parsePlace('#Feuille2!A1')).toEqual({ sheet: 'Feuille2', ref: 'A1' });
		expect(parsePlace("#'L''été 2026'!A1")).toEqual({ sheet: "L'été 2026", ref: 'A1' });
		expect(parsePlace('#Ventes')).toEqual({ name: 'Ventes' });
		expect(parsePlace('#Feuille2!Ventes')).toBeNull();
		expect(parsePlace('B3')).toBeNull();
		expect(quoteSheet('Feuille2')).toBe('Feuille2');
		expect(quoteSheet('Ma feuille')).toBe("'Ma feuille'");
		expect(quoteSheet('A1')).toBe("'A1'");
		expect(placeLink("L'été", 'b3')).toBe("#'L''été'!B3");
		expect(placeLink('', 'c4')).toBe('#C4');
	});
});

describe('clic sur un lien', () => {
	it('sur le texte seulement', () => {
		// Cellule de 100 px à partir de x = 50, texte de 40 px.
		expect(onLinkText(60, 50, 100, 40, 'left')).toBe(true);
		expect(onLinkText(130, 50, 100, 40, 'left')).toBe(false);
		expect(onLinkText(130, 50, 100, 40, 'right')).toBe(true);
		expect(onLinkText(60, 50, 100, 40, 'right')).toBe(false);
		expect(onLinkText(100, 50, 100, 40, 'center')).toBe(true);
	});
	it('court : suivi ; maintenu, glissé ou ailleurs : sélection', () => {
		const press = { row: 2, col: 3, t: 1000, x: 10, y: 10 };
		expect(isLinkClick(press, 1100, { row: 2, col: 3 }, 11, 10)).toBe(true);
		expect(isLinkClick(press, 1000 + HOLD_MS + 1, { row: 2, col: 3 }, 10, 10)).toBe(false);
		expect(isLinkClick(press, 1100, { row: 2, col: 3 }, 30, 10)).toBe(false);
		expect(isLinkClick(press, 1100, { row: 2, col: 4 }, 10, 10)).toBe(false);
		expect(isLinkClick(press, 1100, null, 10, 10)).toBe(false);
	});
});
