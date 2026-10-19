import { describe, expect, it } from 'vitest';
import { COMMAND_TIPS, TAB_TIPS, assignKeyTips, keyChars, keyOf, matchTip } from './keytips.js';

/** Aucun code n'est préfixe d'un autre (ni égal). */
function prefixFree(tips: string[]) {
	for (let i = 0; i < tips.length; i++)
		for (let j = 0; j < tips.length; j++) if (i !== j) expect(tips[j].startsWith(tips[i]), `${tips[i]} / ${tips[j]}`).toBe(false);
}

describe('KeyTips du ruban', () => {
	it('lettres des onglets comme Excel, toutes différentes', () => {
		expect(TAB_TIPS.Accueil).toBe('H');
		expect(TAB_TIPS.Insertion).toBe('N');
		expect(TAB_TIPS.Fichier).toBe('F');
		prefixFree(Object.values(TAB_TIPS));
	});
	it('sans accents ni ponctuation, en majuscules', () => {
		expect(keyChars('Révision')).toBe('REVISION');
		expect(keyChars('Trier A → Z')).toBe('TRIERAZ');
		expect(keyOf('é')).toBe('E');
		expect(keyOf('Enter')).toBe('');
	});
	it('initiale seule si elle est unique, deux lettres sinon', () => {
		const tips = assignKeyTips(['Barres', 'Courbes', 'Secteurs', 'Nuage de points', 'Autres graphiques']);
		expect(tips).toEqual(['B', 'C', 'S', 'N', 'A']);
		const two = assignKeyTips(['Trier A → Z', 'Trier Z → A', 'Filtrer']);
		expect(two).toEqual(['TA', 'TZ', 'F']);
	});
	it("codes préférés (ceux d'Excel) quand ils sont libres", () => {
		const labels = ['Coller', 'Couper', 'Copier', 'Reproduire', 'Gras', 'Italique', 'Souligné', 'Centrer', 'Couleur de police'];
		const tips = assignKeyTips(labels, COMMAND_TIPS);
		expect(tips.slice(0, 7)).toEqual(['V', 'X', 'C', 'FP', '1', '2', '3']);
		expect(tips[7]).toBe('AC');
		expect(tips[8]).toBe('FC');
		prefixFree(tips);
	});
	it('un code préféré pris (doublon) laisse le suivant en choisir un autre', () => {
		const tips = assignKeyTips(['Copier', 'Copier'], COMMAND_TIPS);
		expect(tips[0]).toBe('C');
		expect(tips[1]).not.toBe('C');
		prefixFree(tips);
	});
	it('toujours sans préfixe commun, même pour un onglet chargé', () => {
		const labels = [
			'Annuler', 'Rétablir', 'Coller', 'Couper', 'Copier', 'Reproduire', 'Police', 'Taille de police (points)', 'Gras', 'Italique', 'Souligné', 'Couleur de police', 'Couleur de police ▾',
			'Couleur de remplissage', 'Couleur de remplissage ▾', 'Police …', 'Toutes', 'Contour', 'Aucune', 'Bordures …', 'Haut', 'Milieu', 'Bas', 'Gauche', 'Centrer', 'Droite',
			'Renvoyer à la ligne', 'Fusionner et centrer', 'Alignement …', 'Format de nombre', 'Monétaire', 'Pourcentage', 'Date', '0,00', 'Ajouter une décimale', 'Réduire les décimales',
			'Nombre …', 'Mise en forme conditionnelle', 'Mettre sous forme de tableau', 'Insérer', 'Supprimer', 'Format', 'Somme automatique', 'Remplissage', 'Effacer', 'Trier et filtrer', 'Rechercher', 'Remplacer'
		];
		const tips = assignKeyTips(labels, COMMAND_TIPS);
		// Liste des formats de nombre : N, comme Excel.
		expect(tips[labels.indexOf('Format de nombre')]).toBe('N');
		expect(tips.every((t) => /^[A-Z0-9]{1,2}$/.test(t))).toBe(true);
		prefixFree(tips);
	});
	it('codes d’Excel des onglets Insertion, Données et Formules, sans préfixe commun', () => {
		const insertion = assignKeyTips(['Tableau croisé dynamique', 'Tableau', 'Data Lake Lattis', 'Histogramme', 'Courbes', 'Secteurs', 'Nuage de points', 'Autres graphiques ▾', 'Bouton', 'Champ'], COMMAND_TIPS);
		expect(insertion.slice(0, 2)).toEqual(['V', 'T']);
		prefixFree(insertion);
		const donnees = assignKeyTips(['Texte/CSV', 'Classeur Excel', 'Autres fichiers ▾', 'Base de données ▾', 'Web', 'À partir du tableau', 'Actualiser tout', 'Requêtes et connexions', 'Trier A → Z', 'Trier Z → A', 'Filtrer', 'Effacer', 'Erreurs seulement', 'Convertir', 'Supprimer les doublons', 'Validation des données'], COMMAND_TIPS);
		prefixFree(donnees);
		const formules = assignKeyTips(['Insérer une fonction', 'Somme automatique', 'REFERENCE', 'Afficher les liens', 'Calculer'], COMMAND_TIPS);
		prefixFree(formules);
	});
	it('touches tapées : code atteint, ou début de code', () => {
		const tips = ['FP', 'FF', 'V', '1'];
		expect(matchTip(tips, 'V')).toEqual({ hit: 2, live: true });
		expect(matchTip(tips, 'F')).toEqual({ hit: -1, live: true });
		expect(matchTip(tips, 'FF')).toEqual({ hit: 1, live: true });
		expect(matchTip(tips, 'Q')).toEqual({ hit: -1, live: false });
	});
});
