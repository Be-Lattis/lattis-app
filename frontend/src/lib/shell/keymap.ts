// Raccourcis clavier de Lattis, en un seul endroit : la liste « Raccourcis
// clavier » de l'aide les affiche telles quelles. Les touches s'écrivent
// comme sur un clavier français (Ctrl, Maj, Alt, Entrée, Échap, Pg.Suiv…) ;
// plusieurs écritures d'un même raccourci sont des variantes.

import { TAB_TIPS, keyChars } from '#lib/ui/keytips.js';

export interface KeyEntry {
	/** Combinaisons équivalentes (« Ctrl+D ») ; « puis » sépare deux appuis successifs. */
	keys: string[];
	label: string;
}

export interface KeyGroup {
	title: string;
	entries: KeyEntry[];
}

const tabs = Object.entries(TAB_TIPS)
	.filter(([t]) => t !== 'Fichier')
	.map(([t, k]) => ({ keys: [`Alt puis ${k}`, `Alt+${k}`], label: `Onglet ${t} du ruban` }));

export const SHORTCUTS: KeyGroup[] = [
	{
		title: 'Classeur',
		entries: [
			{ keys: ['Ctrl+N'], label: 'Nouveau classeur' },
			{ keys: ['Ctrl+O'], label: 'Ouvrir un classeur' },
			{ keys: ['Ctrl+S'], label: 'Enregistrer' },
			{ keys: ['F12'], label: 'Enregistrer sous' },
			{ keys: ['Ctrl+P'], label: 'Imprimer la feuille' },
			{ keys: ['Ctrl+W'], label: 'Fermer le classeur' },
			{ keys: ['Ctrl+Z'], label: 'Annuler' },
			{ keys: ['Ctrl+Y'], label: 'Rétablir' },
			{ keys: ['F5'], label: 'Actualiser la requête du tableau actif (ailleurs : recalculer)' },
			{ keys: ['Ctrl+F5', 'Maj+F5'], label: 'Actualiser tout : requêtes puis recalcul' },
			{ keys: ['F9'], label: 'Recalculer les formules' }
		]
	},
	{
		title: 'Ruban, menus et fenêtres',
		entries: [
			{ keys: ['Alt', 'F10'], label: 'Afficher les lettres d’accès (KeyTips) du ruban ; Échap revient en arrière' },
			{ keys: ['Alt puis F', 'Alt+F'], label: 'Menu Fichier' },
			...tabs,
			{ keys: ['← →', '↑ ↓'], label: 'Dans le ruban : onglet ou commande voisine ; ↓ descend vers les commandes, ↑ remonte à l’onglet' },
			{ keys: ['F6', 'Maj+F6'], label: 'Région suivante (précédente) : ruban, barre de formule, grille, volet, onglets, barre d’état' },
			{ keys: ['Maj+F10', 'Menu'], label: 'Menu contextuel de la cellule active (ou du graphique sélectionné)' },
			{ keys: ['↑ ↓', 'Début', 'Fin'], label: 'Dans un menu : entrée précédente, suivante, première, dernière' },
			{ keys: ['Lettre'], label: 'Dans un menu : entrée suivante qui commence par cette lettre' },
			{ keys: ['Entrée', 'Espace'], label: 'Dans un menu : choisir l’entrée' },
			{ keys: ['Tab', 'Maj+Tab'], label: 'Dans une boîte de dialogue : champ suivant (précédent)' },
			{ keys: ['Échap'], label: 'Fermer le menu, la boîte de dialogue ou le volet ; le focus revient où il était' }
		]
	},
	{
		title: 'Feuilles',
		entries: [
			{ keys: ['Ctrl+Pg.Suiv'], label: 'Feuille suivante' },
			{ keys: ['Ctrl+Pg.Préc'], label: 'Feuille précédente' },
			{ keys: ['Maj+F11'], label: 'Insérer une feuille' },
			{ keys: ['← →'], label: 'Sur un onglet de feuille : onglet voisin (Entrée l’affiche)' }
		]
	},
	{
		title: 'Déplacement et sélection',
		entries: [
			{ keys: ['← → ↑ ↓'], label: 'Cellule voisine' },
			{ keys: ['Ctrl+flèche', 'Fin puis flèche'], label: 'Bout de la zone remplie' },
			{ keys: ['Maj+flèche'], label: 'Étendre la sélection d’une cellule' },
			{ keys: ['Ctrl+Maj+flèche'], label: 'Étendre la sélection jusqu’au bout de la zone' },
			{ keys: ['Pg.Suiv', 'Pg.Préc'], label: 'Un écran plus bas (plus haut)' },
			{ keys: ['Début'], label: 'Début de la ligne' },
			{ keys: ['Ctrl+Début'], label: 'Cellule A1' },
			{ keys: ['Ctrl+Fin'], label: 'Dernière cellule utilisée' },
			{ keys: ['Tab', 'Maj+Tab'], label: 'Cellule à droite (à gauche)' },
			{ keys: ['Entrée', 'Maj+Entrée'], label: 'Cellule en dessous (au-dessus)' },
			{ keys: ['Ctrl+A'], label: 'Sélectionner le tableau' },
			{ keys: ['Ctrl+Espace'], label: 'Sélectionner la colonne du tableau' },
			{ keys: ['Maj+Espace'], label: 'Sélectionner la ligne' }
		]
	},
	{
		title: 'Saisie',
		entries: [
			{ keys: ['F2'], label: 'Modifier la cellule active' },
			{ keys: ['Retour arrière'], label: 'Effacer la cellule et commencer la saisie' },
			{ keys: ['Suppr'], label: 'Effacer le contenu de la sélection' },
			{ keys: ['Échap'], label: 'Abandonner la saisie' },
			{ keys: ['Alt+Entrée'], label: 'Saut de ligne dans la cellule' },
			{ keys: ['Ctrl+Entrée'], label: 'Valider la saisie dans toute la sélection' },
			{ keys: ['Ctrl+;'], label: 'Date du jour' },
			{ keys: ['Ctrl+Maj+:'], label: 'Heure actuelle' },
			{ keys: ['Alt+='], label: 'Somme automatique' },
			{ keys: ['Ctrl+D'], label: 'Recopier vers le bas' },
			{ keys: ['Ctrl+R'], label: 'Recopier vers la droite' },
			{ keys: ['Alt+↓'], label: 'Ouvrir la liste déroulante de la cellule (validation)' },
			{ keys: ['F4'], label: 'Répéter la dernière action' }
		]
	},
	{
		title: 'Mise en forme et édition',
		entries: [
			{ keys: ['Ctrl+1'], label: 'Format de cellule' },
			{ keys: ['Ctrl+B'], label: 'Gras' },
			{ keys: ['Ctrl+I'], label: 'Italique' },
			{ keys: ['Ctrl+U'], label: 'Souligné' },
			{ keys: ['Ctrl+C'], label: 'Copier' },
			{ keys: ['Ctrl+X'], label: 'Couper' },
			{ keys: ['Ctrl+V'], label: 'Coller' },
			{ keys: ['Ctrl+F'], label: 'Rechercher' },
			{ keys: ['Ctrl+H'], label: 'Remplacer' },
			{ keys: ['Ctrl++'], label: 'Insérer des lignes ou des colonnes' },
			{ keys: ['Ctrl+-'], label: 'Supprimer des lignes ou des colonnes' },
			{ keys: ['Ctrl+Maj+L'], label: 'Filtres du tableau' }
		]
	},
	{
		title: 'Onglets SQL et SCRIPT',
		entries: [
			{ keys: ['Ctrl+Entrée'], label: 'Exécuter (la sélection, sinon l’instruction sous le curseur en SQL)' },
			{ keys: ['Ctrl+Espace'], label: 'SQL : proposer les noms de tableaux et de colonnes' },
			{ keys: ['Alt+↑', 'Alt+↓'], label: 'SQL : requête précédente (suivante) de l’historique' },
			{ keys: ['Ctrl+S'], label: 'SCRIPT : enregistrer le script' }
		]
	}
];

/** Raccourcis dont le libellé ou les touches contiennent le texte cherché (sans accents ni casse). */
export function searchShortcuts(q: string, groups: KeyGroup[] = SHORTCUTS): KeyGroup[] {
	const words = q.split(/\s+/).map(keyChars).filter(Boolean);
	if (!words.length) return groups;
	return groups
		.map((g) => ({
			title: g.title,
			entries: g.entries.filter((e) => {
				const hay = keyChars(`${g.title} ${e.label} ${e.keys.join(' ')}`);
				return words.every((w) => hay.includes(w));
			})
		}))
		.filter((g) => g.entries.length);
}
