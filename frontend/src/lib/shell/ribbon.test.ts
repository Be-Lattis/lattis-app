import { describe, expect, it } from 'vitest';
import { alignPatch, blockedBy, COLOR_NAMES, COLORS, gate, protectedThing, tipOf, type Gated, type GatedMenu, type GateState } from './ribbon';
import { hintKeys, keyLabel, pdfPrinterHint } from '#lib/web/platform.js';

const run = () => {};
const cmd = (label: string, flags: Partial<Gated> = {}): Gated => ({ label, run, ...flags });

describe('grisage du ruban (audit 3, I1 et I2)', () => {
	const sheet: GateState = { sheetProtected: true, sheetOwnProtected: true };
	const book: GateState = { workbookProtected: true };

	it('feuille protégée : seules les commandes sans effet restent', () => {
		expect(gate(cmd('Copier', { ro: true }), sheet).run).toBe(run);
		const g = gate(cmd('Coller'), sheet);
		expect(g.run).toBeUndefined();
		expect(g.hint).toBe('Coller : feuille protégée (Révision › Ôter la protection de la feuille)');
		// La mise en forme modifie la feuille.
		expect(gate(cmd('Gras', { local: true }), sheet).run).toBeUndefined();
	});

	it('feuille verrouillée par le classeur : le motif renvoie à la protection du classeur', () => {
		const g = gate(cmd('Coller'), { sheetProtected: true, sheetOwnProtected: false });
		expect(g.hint).toBe('Coller : classeur protégé (Révision › Ôter la protection du classeur)');
	});

	it('le grisage ne dépend pas du libellé (renommer une commande ne change rien)', () => {
		expect(gate(cmd('Ôter la protection de la feuille', { ro: true }), sheet).run).toBe(run);
		expect(gate(cmd('Ôter la protection'), sheet).run).toBeUndefined();
	});

	it('tableau croisé d’une feuille : permis sur une feuille protégée, refusé si la structure l’est', () => {
		const pivot = cmd('Tableau croisé dynamique', { ro: true, createsSheet: true });
		expect(gate(pivot, sheet).run).toBe(run);
		const g = gate(pivot, book);
		expect(g.run).toBeUndefined();
		expect(g.hint).toContain('structure du classeur est protégée');
	});

	it('Document : Imprimer reste permis, le tableau croisé (inséré dans le texte) est grisé', () => {
		const doc: GateState = { sheetProtected: true, sheetOwnProtected: true, workbookProtected: true };
		expect(gate(cmd('Imprimer / PDF', { ro: true }), doc).run).toBe(run);
		expect(gate(cmd('Tableau croisé dynamique'), doc).run).toBeUndefined();
		expect(gate(cmd('Coller'), { ...doc, context: 'doc' }).hint).toBe('Coller : document protégé (Révision › Ôter la protection du document)');
		expect(protectedThing('html').protect).toBe('Protéger la page');
		// Structure seule protégée : le Document n'en est pas touché.
		expect(gate(cmd('Tableau croisé dynamique'), book).run).toBe(run);
	});

	it('tableau partagé en lecture seule (rôle Lecteur) : données grisées, mise en forme et lecture permises', () => {
		const reader: GateState = { tableReadOnly: 'Tableau en lecture seule : vos droits ne permettent pas de le modifier (rôle : lecteur)' };
		const g = gate(cmd('Supprimer les doublons'), reader);
		expect(g.run).toBeUndefined();
		expect(g.hint).toBe('Supprimer les doublons : Tableau en lecture seule : vos droits ne permettent pas de le modifier (rôle : lecteur)');
		expect(gate(cmd('Gras', { local: true }), reader).run).toBe(run);
		expect(gate(cmd('Filtrer', { ro: true }), reader).run).toBe(run);
		expect(blockedBy(cmd('Coller'), {})).toBe('');
	});

	it('mise en forme protégée : la mise en forme est grisée, la saisie et les notes restent', () => {
		const fmt: GateState = { formatProtected: true };
		const g = gate(cmd('Gras', { local: true, format: true }), fmt);
		expect(g.run).toBeUndefined();
		expect(g.hint).toBe('Gras : mise en forme protégée (Révision › Ôter la protection de la mise en forme)');
		expect(gate(cmd('Coller'), fmt).run).toBe(run);
		expect(gate(cmd('Nouvelle note', { local: true }), fmt).run).toBe(run);
		// Format ▾ : « Format de cellule… » grisé, masquer les lignes permis.
		const format: GatedMenu = { label: 'Format', run, local: true, items: [{ label: 'Format de cellule…', format: true, run }, { label: 'Masquer les lignes', run }] };
		const m = gate(format, fmt);
		expect(m.run).toBe(run);
		expect(m.items?.map((it) => !!it.run)).toEqual([false, true]);
		// La protection de la feuille l'emporte (son motif).
		expect(gate(cmd('Gras', { format: true }), { ...fmt, sheetProtected: true, sheetOwnProtected: true }).hint).toContain('feuille protégée');
	});

	it('commande à menu : chaque choix est grisé à part, la commande quand il n’en reste aucun', () => {
		const effacer: GatedMenu = { label: 'Effacer', run, items: [{ label: 'Effacer les formats', local: true, run }, { label: 'Effacer le contenu', run }] };
		const ro = gate(effacer, { tableReadOnly: 'lecture seule' });
		expect(ro.run).toBe(run);
		expect(ro.items!.map((i) => !!i.run)).toEqual([true, false]);
		const prot = gate(effacer, sheet);
		expect(prot.run).toBeUndefined();
		expect(prot.hint).toContain('feuille protégée');
		// Les choix héritent des drapeaux de la commande.
		const trier = gate({ label: 'Trier et filtrer', run, items: [{ label: 'Trier de A à Z', local: true, run }, { label: 'Filtrer', ro: true, run }] }, sheet);
		expect(trier.items!.map((i) => !!i.run)).toEqual([false, true]);
		expect(trier.run).toBe(run);
	});

	it('commande déjà grisée : inchangée', () => {
		const c = { label: 'Coller' };
		expect(gate(c, sheet)).toBe(c);
	});
});

describe('infobulles (audit 3, M1, M3, M13)', () => {
	it('sans texte : le libellé, ou pourquoi elle est grisée sans classeur (plus de « à venir »)', () => {
		expect(tipOf({ label: 'Gauche', run })).toBe('Gauche');
		expect(tipOf({ label: 'Gauche' }, true)).toBe('Gauche : ouvrez ou créez un classeur');
		expect(tipOf({ label: 'Gauche' }, false)).not.toContain('à venir');
	});
	it('raccourcis écrits Cmd sur macOS', () => {
		expect(tipOf({ label: 'Gras', hint: 'Gras (Ctrl+B)', run }, false, true)).toBe('Gras (Cmd+B)');
		expect(tipOf({ label: 'Gras', hint: 'Gras (Ctrl+B)', run }, false, false)).toBe('Gras (Ctrl+B)');
		expect(hintKeys('Ctrl+Maj+L et Alt+=', true)).toBe('Cmd+Maj+L et Alt+=');
		expect(keyLabel('Ctrl+N', true, true)).toBe('Cmd+Alt+N');
		expect(keyLabel('F12', false, true)).toBe('F12');
	});
	it('imprimante PDF : le nom de Windows seulement sur le bureau Windows', () => {
		expect(pdfPrinterHint(true)).toContain('Microsoft Print to PDF');
		expect(pdfPrinterHint(false)).not.toContain('Microsoft');
	});
});

describe('palette et alignement (audit 3, M8, M10)', () => {
	it('chaque teinte a un nom en français', () => {
		expect(COLORS).toHaveLength(20);
		for (const c of COLORS) expect(COLOR_NAMES[c]).toMatch(/^[A-ZÉ][a-zéèêâ ]+$/);
	});
	it('cliquer sur l’alignement en cours revient au standard, comme Excel', () => {
		expect(alignPatch(undefined, 'left')).toEqual({ al: 'left' });
		expect(alignPatch('left', 'left')).toEqual({ al: null });
		expect(alignPatch('center', 'right')).toEqual({ al: 'right' });
	});
});
