// Textes de la référence Lua dans la langue de l'interface (docs/i18n.md).
// Le français est dans reference.ts et api.gen.ts ; les autres langues sont
// dans ref/<langue>.json (fonctions du langage et db.*, écrits à la main) et
// ref/<langue>.api.json (commandes de l'application, générés par
// internal/api avec api.gen.ts), chargés à la demande comme les catalogues
// de textes. Un texte absent reste en français.

import { lang, type Lang } from '#lib/i18n/index.js';

/** Textes d'une entrée dans une langue (absents : le français). */
export interface RefText {
	signature?: string;
	summary?: string;
	details?: string;
}

const files = import.meta.glob<Record<string, RefText>>('./ref/*.json', { import: 'default' });

/** Langue d'un fichier de ref/ (« en » pour ./ref/en.api.json). */
const langOf = (path: string) => path.slice(path.lastIndexOf('/') + 1).split('.')[0];

let texts = $state.raw<Partial<Record<Lang, Record<string, RefText>>>>({});
const loading = new Map<Lang, Promise<void>>();

/** Charge les textes d'une langue (une fois). */
export function loadRefTexts(l: Lang): Promise<void> {
	if (l === 'fr' || texts[l]) return Promise.resolve();
	let p = loading.get(l);
	if (!p) {
		const mine = Object.entries(files).filter(([path]) => langOf(path) === l);
		p = Promise.all(mine.map(([, imp]) => imp())).then((parts) => {
			texts = { ...texts, [l]: Object.assign({}, ...parts) };
		});
		loading.set(l, p);
	}
	return p;
}

/**
 * Texte d'une entrée dans la langue de l'interface, undefined pour le
 * français ou un texte absent (le premier appel dans une langue lance son
 * chargement ; l'affichage suit quand il arrive).
 */
export function refText(name: string, field: keyof RefText): string | undefined {
	const l = lang();
	if (l === 'fr') return undefined;
	const m = texts[l];
	if (!m) {
		void loadRefTexts(l);
		return undefined;
	}
	return m[name]?.[field];
}
