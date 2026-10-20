// Commandes du ruban : ce qui les grise (protection, droits d'un classeur
// partagé, aucun classeur) et le texte de leur infobulle, sans composant
// (testé par vitest). Le grisage dépend de drapeaux posés à la définition de
// chaque commande, jamais de son libellé.

import { hintKeys } from '#lib/web/platform.js';

/** Drapeaux d'une commande, posés à sa définition (Ribbon.svelte). */
export interface Gated {
	label: string;
	run?: () => void;
	hint?: string;
	/** Ne modifie rien (lecture, copie, affichage, mise en page) : jamais grisée. */
	ro?: boolean;
	/** Ne change que la présentation locale (mise en forme, tri, notes) : permise sur un tableau partagé en lecture seule. */
	local?: boolean;
	/** Saisie dans les tableaux (coller, effacer le contenu, lignes, tri) : seule permise quand la structure de la feuille est protégée. */
	data?: boolean;
	/** Crée une feuille (tableau croisé) : refusée quand la structure du classeur est protégée. */
	createsSheet?: boolean;
}

export interface GateState {
	/** Feuille (ou Document) verrouillée : par sa propre protection ou par celle du classeur. */
	sheetProtected?: boolean;
	/** La feuille elle-même est protégée (sinon c'est le classeur). */
	sheetOwnProtected?: boolean;
	/** Structure du classeur protégée. */
	workbookProtected?: boolean;
	/** Structure de la feuille protégée (Révision › Protéger la structure) : seule la saisie dans les tableaux reste permise. */
	structureProtected?: boolean;
	/** … par la protection du classeur (structure de toutes les feuilles). */
	structureByBook?: boolean;
	/** Classeur collaboratif ouvert par un éditeur ou un lecteur : cellules libres et mise en page réservées aux propriétaires et concepteurs (comme une structure protégée). */
	layoutShared?: boolean;
	/** Classeur partagé : la cellule active est dans un tableau publié en lecture seule (le motif). */
	tableReadOnly?: string;
	/** Onglet affiché : une feuille (par défaut), un Document ou une page web. */
	context?: 'sheet' | 'doc' | 'html';
}

/** Ce que protège Révision › Protéger, selon l'onglet affiché. */
export function protectedThing(context: GateState['context'] = 'sheet'): { locked: string; protect: string; unprotect: string } {
	if (context === 'doc') return { locked: 'document protégé', protect: 'Protéger le document', unprotect: 'Ôter la protection du document' };
	if (context === 'html') return { locked: 'page protégée', protect: 'Protéger la page', unprotect: 'Ôter la protection de la page' };
	return { locked: 'feuille protégée', protect: 'Protéger la feuille', unprotect: 'Ôter la protection de la feuille' };
}

/** Pourquoi la commande est grisée ('' : elle ne l'est pas). */
/** Motif des commandes grisées pour un éditeur ou un lecteur d'un classeur collaboratif. */
export const LAYOUT_SHARED = 'seuls les propriétaires et concepteurs modifient les cellules libres et la mise en page de ce classeur partagé';
/** Message d'une saisie refusée hors des tableaux (le même que le moteur, workbook.ErrLayout). */
export const LAYOUT_LOCKED = 'Seuls les propriétaires et concepteurs modifient les cellules libres et la mise en page de ce classeur partagé';

export function blockedBy(c: Gated, s: GateState): string {
	if (c.createsSheet && s.workbookProtected) return 'la structure du classeur est protégée (cette commande crée une feuille)';
	if (c.createsSheet && s.layoutShared) return LAYOUT_SHARED;
	if (c.ro) return '';
	if (s.layoutShared && !c.data && !s.sheetProtected) return LAYOUT_SHARED;
	if (s.sheetProtected) {
		const p = protectedThing(s.context);
		return s.sheetOwnProtected ? `${p.locked} (Révision › ${p.unprotect})` : 'classeur protégé (Révision › Ôter la protection du classeur)';
	}
	if (s.structureProtected && !c.data)
		return s.structureByBook ? 'structure des feuilles protégée (Révision › Ôter la protection du classeur)' : 'structure protégée (Révision › Ôter la protection de la structure)';
	if (c.local) return '';
	return s.tableReadOnly ?? '';
}

/** Commande à menu (▾) : ses choix héritent de ses drapeaux, chacun peut les changer. */
export interface GatedMenu extends Gated {
	items?: Gated[];
}

/**
 * La commande telle qu'affichée : grisée avec son motif, ou inchangée. Une
 * commande à menu grise chacun de ses choix, et se grise elle-même quand
 * plus aucun ne reste.
 */
export function gate<C extends GatedMenu>(c: C, s: GateState): C {
	if (!c.run) return c;
	if (c.items) {
		const items = c.items.map((it) => gate({ ro: c.ro, local: c.local, data: c.data, createsSheet: c.createsSheet, ...it }, s));
		const lost = c.items.findIndex((it, i) => it.run && !items[i].run);
		if (lost >= 0 && !items.some((it) => it.run)) {
			const why = blockedBy({ ro: c.ro, local: c.local, data: c.data, createsSheet: c.createsSheet, ...c.items[lost] }, s);
			return { ...c, items, run: undefined, hint: `${c.label} : ${why}` } as C;
		}
		return { ...c, items } as C;
	}
	const why = blockedBy(c, s);
	return why ? { ...c, run: undefined, hint: `${c.label} : ${why}` } : c;
}

/**
 * Infobulle d'une commande : son texte (raccourcis écrits pour macOS s'il le
 * faut) ; sans texte, son libellé, et pourquoi elle est grisée quand aucun
 * classeur n'est ouvert.
 */
export function tipOf(c: { label: string; hint?: string; run?: unknown }, disabled = false, mac?: boolean): string {
	if (c.hint) return hintKeys(c.hint, mac);
	return c.run || !disabled ? c.label : `${c.label} : ouvrez ou créez un classeur`;
}

/** Noms des teintes de la palette du ruban (lecteurs d'écran, infobulles). */
export const COLOR_NAMES: Record<string, string> = {
	'#000000': 'Noir',
	'#404040': 'Gris foncé',
	'#7f7f7f': 'Gris',
	'#bfbfbf': 'Gris clair',
	'#ffffff': 'Blanc',
	'#c00000': 'Rouge foncé',
	'#ff0000': 'Rouge',
	'#ffc000': 'Orange',
	'#ffff00': 'Jaune',
	'#92d050': 'Vert clair',
	'#00b050': 'Vert',
	'#00b0f0': 'Bleu clair',
	'#0070c0': 'Bleu',
	'#002060': 'Bleu foncé',
	'#7030a0': 'Violet',
	'#ffc7ce': 'Rose pâle',
	'#fff2cc': 'Jaune pâle',
	'#c6efce': 'Vert pâle',
	'#ddebf7': 'Bleu pâle',
	'#e4dfec': 'Mauve pâle'
};

/** Couleurs de la palette, dans l'ordre d'affichage. */
export const COLORS = Object.keys(COLOR_NAMES);

/** Alignement horizontal : un bouton enfoncé ramène au standard, comme Excel. */
export function alignPatch(current: string | undefined, al: 'left' | 'center' | 'right'): { al: string | null } {
	return { al: current === al ? null : al };
}
