// Liens hypertexte des cellules (Ctrl+K), comme dans un tableur : la cellule
// garde son texte, le lien est sa propriété ln (internal/canvas/links.go),
// ou l'adresse d'une formule =LIEN_HYPERTEXTE(…). Un clic sur le texte suit
// le lien, un clic maintenu sélectionne la cellule.
//
// Adresses : http(s)://… (page web), mailto:… (e-mail), #Feuille!A1, #A1 ou
// #Nom (dans le classeur). Tout autre schéma est refusé, ici comme dans le
// classeur.
import { isWeb } from '#lib/web/platform.js';
import { platform } from '#lib/shell/settings.svelte.js';

export type LinkKind = 'web' | 'mail' | 'place';

const EMAIL = /^[^\s@<>"()[\],;:]+@[^\s@<>"()[\],;:]+\.[^\s@<>"()[\],;:.]{2,}$/;
const HOST = /^[\p{L}\p{N}][\p{L}\p{N}-]*(\.[\p{L}\p{N}-]+)*\.\p{L}{2,}([/:?#]\S*)?$/u;

/** Le texte est une adresse e-mail seule (« nom@exemple.fr »). */
export function isEmail(s: string): boolean {
	return EMAIL.test(s.trim());
}

/** Sorte d'une adresse enregistrée, null si elle est refusée. */
export function linkKind(target: string | undefined | null): LinkKind | null {
	const s = (target ?? '').trim();
	if (/^https?:\/\/\S/i.test(s)) return 'web';
	if (/^mailto:\S/i.test(s)) return 'mail';
	if (s.startsWith('#') && parsePlace(s)) return 'place';
	return null;
}

/**
 * Saisie qui devient un lien d'office (comme la correction automatique d'un
 * tableur) : une adresse web (http://, https://, www.) ou e-mail, seule
 * dans la cellule ; l'adresse du lien, sinon null.
 */
export function autoLink(text: string): string | null {
	const s = text.trim();
	if (!s || /\s/.test(s) || s.startsWith('=')) return null;
	if (/^https?:\/\/[^\s/]+\.[^\s]+$/i.test(s)) return s;
	if (/^www\.[^\s/]+\.[^\s]{2,}$/i.test(s) && HOST.test(s)) return 'http://' + s;
	if (/^mailto:/i.test(s) && isEmail(s.slice(7))) return s;
	if (isEmail(s)) return 'mailto:' + s;
	return null;
}

/**
 * Adresse tapée dans le dialogue (page web ou e-mail) : la forme
 * enregistrée (http:// ou mailto: ajouté), null si elle n'est pas reconnue
 * ou si son schéma est refusé (javascript:, file:…).
 */
export function linkFromInput(kind: 'web' | 'mail', input: string): string | null {
	const s = input.trim();
	if (!s) return null;
	if (kind === 'mail') {
		const a = s.replace(/^mailto:/i, '');
		return isEmail(a.split('?')[0]) ? 'mailto:' + a : null;
	}
	if (/^https?:\/\/\S/i.test(s)) return s.replace(/ /g, '%20');
	if (/^[a-z][a-z0-9+.-]*:/i.test(s) && !HOST.test(s)) return null; // autre schéma
	if (isEmail(s)) return 'mailto:' + s;
	return HOST.test(s) ? 'http://' + s : null;
}

/** Destination dans le classeur : feuille (vide : celle du lien) et cellule, ou nom (tableau, cellule nommée). */
export interface Place {
	sheet?: string;
	ref?: string;
	name?: string;
}

const A1 = /^\$?[A-Za-z]{1,3}\$?[1-9][0-9]{0,6}(:\$?[A-Za-z]{1,3}\$?[1-9][0-9]{0,6})?$/;
const PLAIN = /^[\p{L}_][\p{L}\p{N}_.]*$/u;

/** Lit « #Feuille!A1 », « #'Ma feuille'!A1 », « #A1 » ou « #Nom ». */
export function parsePlace(target: string): Place | null {
	let rest = target.trim();
	if (!rest.startsWith('#')) return null;
	rest = rest.slice(1);
	const p: Place = {};
	if (rest.startsWith("'")) {
		let i = 1;
		let name = '';
		for (; i < rest.length; i++) {
			if (rest[i] === "'") {
				if (rest[i + 1] === "'") {
					name += "'";
					i++;
					continue;
				}
				break;
			}
			name += rest[i];
		}
		if (i >= rest.length || rest[i + 1] !== '!' || !name) return null;
		p.sheet = name;
		rest = rest.slice(i + 2);
	} else if (rest.includes('!')) {
		const i = rest.lastIndexOf('!');
		p.sheet = rest.slice(0, i).trim();
		rest = rest.slice(i + 1);
		if (!p.sheet) return null;
	}
	rest = rest.trim();
	if (A1.test(rest)) p.ref = rest.replace(/\$/g, '').toUpperCase();
	else if (!p.sheet && PLAIN.test(rest)) p.name = rest;
	else return null;
	return p;
}

/** Nom de feuille dans une adresse : entre apostrophes s'il n'est pas un simple identifiant. */
export function quoteSheet(name: string): string {
	return PLAIN.test(name) && !A1.test(name) ? name : "'" + name.replace(/'/g, "''") + "'";
}

/** Adresse d'une cellule ou plage du classeur (feuille vide : la même feuille). */
export function placeLink(sheet: string, ref: string): string {
	return sheet ? `#${quoteSheet(sheet)}!${ref.toUpperCase()}` : `#${ref.toUpperCase()}`;
}

/** Texte de l'adresse pour l'infobulle et le dialogue (sans « mailto: »). */
export function linkText(target: string): string {
	return target.replace(/^mailto:/i, '');
}

/**
 * Ouvre une adresse web ou e-mail hors de l'application : sur le bureau,
 * par le navigateur ou la messagerie du système ; en version web, un
 * nouvel onglet (noopener) pour une page. Rien d'autre ne s'ouvre ; faux si
 * l'adresse est refusée.
 */
export function openLink(target: string): boolean {
	const kind = linkKind(target);
	if (kind !== 'web' && kind !== 'mail') return false;
	const u = target.trim();
	const inBrowser = () => {
		if (kind === 'web') {
			window.open(u, '_blank', 'noopener,noreferrer');
			return;
		}
		const a = document.createElement('a');
		a.href = u;
		a.rel = 'noopener';
		a.click();
	};
	if (isWeb || platform.web) {
		inBrowser();
		return true;
	}
	import('@wailsio/runtime').then(
		(rt) => Promise.resolve(rt.Browser.OpenURL(u)).catch(inBrowser),
		inBrowser
	);
	return true;
}

/** Clic court sur un lien : il le suit ; maintenu au-delà, il sélectionne seulement (ms). */
export const HOLD_MS = 500;

/**
 * Le point x tombe-t-il sur le texte d'une cellule (x0 : bord gauche de la
 * cellule, cw : sa largeur, tw : largeur du texte, align : alignement) ? Un
 * clic à côté du texte sélectionne la cellule sans suivre le lien.
 */
export function onLinkText(x: number, x0: number, cw: number, tw: number, align: 'left' | 'center' | 'right'): boolean {
	const left = align === 'left' ? x0 + 6 : align === 'right' ? x0 + cw - 6 - tw : x0 + (cw - tw) / 2;
	return x >= left - 3 && x <= left + tw + 3;
}

/** Clic relâché : suivre le lien si c'était un clic court, sans glisser, sur la même cellule. */
export function isLinkClick(press: { row: number; col: number; t: number; x: number; y: number }, now: number, at: { row: number; col: number } | null, x: number, y: number): boolean {
	return !!at && at.row === press.row && at.col === press.col && now - press.t < HOLD_MS && Math.abs(x - press.x) < 5 && Math.abs(y - press.y) < 5;
}
