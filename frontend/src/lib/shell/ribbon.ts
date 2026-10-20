// Commandes du ruban : ce qui les grise (protection, droits d'un classeur
// partagé, aucun classeur) et le texte de leur infobulle, sans composant
// (testé par vitest). Le grisage dépend de drapeaux posés à la définition de
// chaque commande, jamais de son libellé.

import { t, tAny } from '#lib/i18n/index.js';
import { hintKeys } from '#lib/web/platform.js';

/** Libellé d'un onglet du ruban (identifiants : file, home, insert, pageLayout, data, formulas, review, view, ai, sql, script, help). */
export function tabLabel(id: string): string {
	return tAny(`ribbon.tabs.${id}`);
}

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
	const k = context === 'doc' || context === 'html' ? context : 'sheet';
	return { locked: tAny(`ribbon.protect.${k}.locked`), protect: tAny(`ribbon.protect.${k}.protect`), unprotect: tAny(`ribbon.protect.${k}.unprotect`) };
}

/** Motif des commandes grisées pour un éditeur ou un lecteur d'un classeur collaboratif. */
export function layoutShared(): string {
	return t('ribbon.gate.layoutShared');
}
/** Message d'une saisie refusée hors des tableaux (le même que le moteur, workbook.ErrLayout). */
export function layoutLocked(): string {
	return t('ribbon.gate.layoutLocked');
}

/** Pourquoi la commande est grisée ('' : elle ne l'est pas). */
export function blockedBy(c: Gated, s: GateState): string {
	if (c.createsSheet && s.workbookProtected) return t('ribbon.gate.workbookStructure');
	if (c.createsSheet && s.layoutShared) return layoutShared();
	if (c.ro) return '';
	if (s.layoutShared && !c.data && !s.sheetProtected) return layoutShared();
	if (s.sheetProtected) {
		const p = protectedThing(s.context);
		return s.sheetOwnProtected ? t('ribbon.gate.ownProtected', { locked: p.locked, unprotect: p.unprotect }) : t('ribbon.gate.bookProtected');
	}
	if (s.structureProtected && !c.data) return s.structureByBook ? t('ribbon.gate.structureByBook') : t('ribbon.gate.structure');
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
			return { ...c, items, run: undefined, hint: t('ribbon.gate.hint', { command: c.label, reason: why }) } as C;
		}
		return { ...c, items } as C;
	}
	const why = blockedBy(c, s);
	return why ? { ...c, run: undefined, hint: t('ribbon.gate.hint', { command: c.label, reason: why }) } : c;
}

/**
 * Infobulle d'une commande : son texte (raccourcis écrits pour macOS s'il le
 * faut) ; sans texte, son libellé, et pourquoi elle est grisée quand aucun
 * classeur n'est ouvert.
 */
export function tipOf(c: { label: string; hint?: string; run?: unknown }, disabled = false, mac?: boolean): string {
	if (c.hint) return hintKeys(c.hint, mac);
	return c.run || !disabled ? c.label : t('ribbon.gate.noWorkbook', { command: c.label });
}

/** Alignement horizontal : un bouton enfoncé ramène au standard, comme Excel. */
export function alignPatch(current: string | undefined, al: 'left' | 'center' | 'right'): { al: string | null } {
	return { al: current === al ? null : al };
}
