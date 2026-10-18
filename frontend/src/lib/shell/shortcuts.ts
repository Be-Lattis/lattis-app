// Raccourcis clavier de l'application, par domaine : la seule liste affichée
// à l'utilisateur (aide › Raccourcis clavier). Le traitement des touches est
// ailleurs (grid/shortcuts.ts, Sheet.svelte, +page.svelte, éditeurs) : une
// touche ajoutée là s'ajoute ici.

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

export const shortcutGroups: ShortcutGroup[] = [
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
		id: 'feuilles',
		title: 'Feuilles',
		items: [
			{ keys: 'Ctrl+Pg.Suiv', label: 'Feuille suivante' },
			{ keys: 'Ctrl+Pg.Préc', label: 'Feuille précédente' },
			{ keys: 'Maj+F11', label: 'Insérer une feuille' }
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
			{ keys: 'Ctrl+Maj+L', label: 'Filtre de la colonne (efface les filtres s’il y en a)' }
		]
	},
	{
		id: 'calcul',
		title: 'Calcul et données',
		items: [
			{ keys: 'F5', label: 'Actualiser la requête du tableau, sinon recalculer' },
			{ keys: 'Ctrl+F5 / Maj+F5', label: 'Tout actualiser : requêtes et formules' },
			{ keys: 'F9', label: 'Recalculer' }
		]
	},
	{
		id: 'editeurs',
		title: 'Onglets SQL, SCRIPT et HTML',
		items: [
			{ keys: 'Ctrl+Entrée', label: 'SQL : exécuter la sélection, sinon l’instruction sous le curseur' },
			{ keys: 'Ctrl+Espace', label: 'SQL : noms des tableaux et colonnes' },
			{ keys: 'Alt+Haut / Alt+Bas', label: 'SQL : historique des requêtes' },
			{ keys: 'Ctrl+Entrée', label: 'SCRIPT : exécuter le script' },
			{ keys: 'Ctrl+S', label: 'SCRIPT, HTML : enregistrer' },
			{ keys: 'Ctrl+Entrée', label: 'HTML : mettre l’aperçu à jour' }
		]
	}
];
