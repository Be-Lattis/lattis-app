// Raccourcis clavier de l'application, par domaine : la seule liste affichée
// à l'utilisateur (aide › Raccourcis clavier). Le traitement des touches est
// ailleurs (grid/shortcuts.ts, Sheet.svelte, +page.svelte, éditeurs) : une
// touche ajoutée là s'ajoute ici. Les lettres du ruban (KeyTips) viennent
// de ui/keytips.ts.

import { TAB_TIPS } from '#lib/ui/keytips.js';
import { isMac, isWeb, keyLabel } from '#lib/web/platform.js';

export interface ShortcutInfo {
	/** Touches, écrites comme sur le clavier français : « Ctrl+Maj+L ». */
	keys: string;
	/** Ce que fait le raccourci, en une phrase courte. */
	label: string;
}

export interface ShortcutGroup {
	id: string;
	title: string;
	items: ShortcutInfo[];
}

const groups: ShortcutGroup[] = [
	{
		id: 'fichier',
		title: 'Classeur',
		items: [
			{ keys: 'Ctrl+N', label: 'Nouveau classeur' },
			{ keys: 'Ctrl+O', label: 'Ouvrir un classeur' },
			{ keys: 'Ctrl+S', label: 'Enregistrer' },
			{ keys: 'F12', label: 'Enregistrer sous' },
			{ keys: 'Ctrl+P', label: 'Imprimer la feuille' },
			{ keys: 'Ctrl+W', label: 'Fermer le classeur' },
			{ keys: 'Ctrl+Z', label: 'Annuler' },
			{ keys: 'Ctrl+Y', label: 'Rétablir' },
			{ keys: 'F1', label: 'Aide (dans une formule : la page de la fonction sous le curseur)' }
		]
	},
	{
		id: 'clavier',
		title: 'Ruban, menus et fenêtres',
		items: [
			{ keys: 'Alt / F10', label: "Lettres d'accès (KeyTips) du ruban ; Échap revient en arrière" },
			{ keys: 'Ctrl+F1', label: 'Réduire ou épingler le ruban (aussi : double-clic sur un onglet du ruban)' },
			{ keys: 'Alt puis F', label: 'Menu Fichier' },
			...Object.entries(TAB_TIPS)
				.filter(([t]) => t !== 'Fichier' && t !== 'Aide')
				.map(([t, k]) => ({ keys: `Alt puis ${k}`, label: `Onglet ${t} du ruban (ou Alt+${k})` })),
			{ keys: 'Flèches', label: "Dans le ruban : onglet ou commande voisine ; Bas descend aux commandes, Haut remonte à l'onglet" },
			{ keys: 'F6 / Maj+F6', label: "Région suivante, précédente : ruban, barre de formule, grille, volet, onglets, barre d'état" },
			{ keys: 'Maj+F10 / Menu', label: 'Menu contextuel de la cellule active (ou du graphique sélectionné)' },
			{ keys: 'Haut / Bas', label: 'Dans un menu : entrée précédente, suivante (Origine, Fin : la première, la dernière)' },
			{ keys: 'Lettre', label: 'Dans un menu : entrée suivante qui commence par cette lettre' },
			{ keys: 'Entrée / Espace', label: "Dans un menu : choisir l'entrée" },
			{ keys: 'Tab / Maj+Tab', label: 'Dans une boîte de dialogue : champ suivant, précédent' },
			{ keys: 'Échap', label: 'Fermer le menu, la boîte de dialogue ou le volet ; le focus revient où il était' }
		]
	},
	{
		id: 'feuilles',
		title: 'Feuilles',
		items: [
			{ keys: 'Ctrl+Pg.Suiv', label: 'Feuille suivante' },
			{ keys: 'Ctrl+Pg.Préc', label: 'Feuille précédente' },
			{ keys: 'Maj+F11', label: 'Insérer une feuille' },
			{ keys: 'Gauche / Droite', label: "Sur un onglet de feuille (F6) : onglet voisin ; Entrée l'affiche" },
			{ keys: 'Alt+Bas', label: 'Sur un onglet à choix (« Ventes · 2025 ▾ ») : ouvrir la liste des valeurs' },
			{ keys: 'Ctrl+Alt+Pg.Suiv', label: "Onglet à choix : valeur suivante (l'année d'après)" },
			{ keys: 'Ctrl+Alt+Pg.Préc', label: 'Onglet à choix : valeur précédente' }
		]
	},
	{
		id: 'deplacement',
		title: 'Déplacement et sélection',
		items: [
			{ keys: 'Flèches', label: 'Cellule voisine' },
			{ keys: 'Ctrl+Flèche', label: 'Bord de la zone de données' },
			{ keys: 'Fin puis Flèche', label: 'Comme Ctrl+Flèche' },
			{ keys: 'Maj+Flèche', label: 'Étendre la sélection' },
			{ keys: 'Pg.Suiv / Pg.Préc', label: 'Page suivante, précédente' },
			{ keys: 'Origine', label: 'Début de la ligne' },
			{ keys: 'Ctrl+Origine', label: 'Cellule A1' },
			{ keys: 'Ctrl+Fin', label: 'Dernière cellule utilisée' },
			{ keys: 'Tab / Maj+Tab', label: 'Cellule suivante, précédente' },
			{ keys: 'Ctrl+A', label: 'Sélectionner le tableau' },
			{ keys: 'Ctrl+Espace', label: 'Sélectionner la colonne du tableau' },
			{ keys: 'Maj+Espace', label: 'Sélectionner la ligne' }
		]
	},
	{
		id: 'saisie',
		title: 'Saisie',
		items: [
			{ keys: 'F2', label: 'Modifier la cellule' },
			{ keys: 'Entrée', label: 'Valider et descendre' },
			{ keys: 'Ctrl+Entrée', label: 'Valider dans toute la sélection' },
			{ keys: 'Alt+Entrée', label: 'Saut de ligne dans la cellule' },
			{ keys: 'Échap', label: 'Abandonner la saisie' },
			{ keys: 'Suppr', label: 'Effacer la sélection' },
			{ keys: 'Ctrl+;', label: 'Date du jour' },
			{ keys: 'Ctrl+Maj+:', label: 'Heure actuelle' },
			{ keys: 'Alt+=', label: 'Somme automatique' },
			{ keys: 'Alt+Bas', label: 'Ouvrir la liste déroulante de la cellule' },
			{ keys: 'Ctrl+D', label: 'Recopier vers le bas' },
			{ keys: 'Ctrl+R', label: 'Recopier vers la droite' },
			{ keys: 'F4', label: 'Répéter la dernière action' }
		]
	},
	{
		id: 'edition',
		title: 'Édition et mise en forme',
		items: [
			{ keys: 'Ctrl+C', label: 'Copier' },
			{ keys: 'Ctrl+X', label: 'Couper' },
			{ keys: 'Ctrl+V', label: 'Coller' },
			{ keys: 'Ctrl+F', label: 'Rechercher' },
			{ keys: 'Ctrl+H', label: 'Remplacer' },
			{ keys: 'Ctrl+B / Ctrl+I / Ctrl+U', label: 'Gras, italique, souligné' },
			{ keys: 'Ctrl+1', label: 'Format de cellule' },
			{ keys: 'Ctrl++ / Ctrl+-', label: 'Insérer, supprimer des lignes ou colonnes' },
			{ keys: 'Ctrl+5', label: 'Barré' },
			{ keys: 'Ctrl+9 / Ctrl+0', label: 'Masquer les lignes, les colonnes de la sélection' },
			{ keys: 'Ctrl+L / Ctrl+T', label: 'Mettre sous forme de tableau' },
			{ keys: 'Ctrl+Maj+L', label: "Filtre de la colonne (efface les filtres s'il y en a)" },
			{ keys: 'Maj+F2', label: 'Nouvelle note (ou modifier la note)' },
			{ keys: 'Alt+F1 / F11', label: 'Graphique (histogramme) du tableau actif' }
		]
	},
	{
		id: 'calcul',
		title: 'Calcul et données',
		items: [
			{ keys: 'F5', label: 'Actualiser la requête du tableau, sinon recalculer' },
			{ keys: 'Ctrl+Alt+F5 / Ctrl+F5 / Maj+F5', label: 'Tout actualiser : requêtes et formules' },
			{ keys: 'Maj+F3', label: 'Insérer une fonction' },
			{ keys: 'F9', label: 'Recalculer' }
		]
	},
	{
		id: 'editeurs',
		title: 'Onglets SQL, Script et HTML',
		items: [
			{ keys: 'Ctrl+Entrée', label: "SQL : exécuter la sélection, sinon l'instruction sous le curseur" },
			{ keys: 'Ctrl+Espace', label: 'SQL : noms des tableaux et colonnes' },
			{ keys: 'Alt+Haut / Alt+Bas', label: 'SQL : historique des requêtes' },
			{ keys: 'Ctrl+Entrée', label: 'Script : exécuter le script' },
			{ keys: 'Ctrl+S', label: 'Script, HTML : enregistrer' },
			{ keys: 'Ctrl+Entrée', label: "HTML : mettre l'aperçu à jour" }
		]
	}
];

/**
 * Les raccourcis affichés. En web, ceux que le navigateur garde pour lui
 * (Ctrl+N, Ctrl+W, F12, Ctrl+Pg.Suiv…) sont remplacés par leur équivalent
 * (web/platform.ts), reconnu aussi sur le bureau ; sur macOS, Ctrl s'écrit Cmd.
 */
export const shortcutGroups: ShortcutGroup[] = isWeb || isMac ? webGroups(groups, isWeb) : groups;

export function webGroups(list: ShortcutGroup[], web = true): ShortcutGroup[] {
	return list.map((g) => ({ ...g, items: g.items.map((s) => ({ ...s, keys: keyLabel(s.keys, web) })) }));
}
