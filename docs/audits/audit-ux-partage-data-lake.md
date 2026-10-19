# Audit UX : « Partager au Data Lake Lattis »

*19 octobre 2026. Boîte ouverte par clic droit sur un tableau › Partager au Data Lake Lattis… (`frontend/src/lib/remote/DataShareDialog.svelte`), avec ses trois écrans : choix de la voie, publication du tableau seul (`TableSourceForm.svelte`), partage d'un tableau d'un classeur publié. Constat de départ : fenêtre haute et dense, aucune hiérarchie, trop de texte.*

## 1. Constats

### Hiérarchie et densité

- **Tout au même niveau.** Serveur, nom, description, nom SQL, dossier, colonnes, mode de mise à jour, envoi à l'enregistrement et audience se suivent sans distinction entre l'essentiel (nom, dossier, colonnes, audience) et les réglages rares (nom SQL, mode, description).
- **Bandeau de 4 lignes** en tête, qui répète l'aide (et change de texte selon une case cochée plus bas).
- **Trois paragraphes gris** (colonnes non envoyées, valeurs refusées, changement de mode) lus par personne, toujours affichés.
- **Mode de mise à jour** : trois boutons radio avec chacun une phrase longue, puis un paragraphe de plus.
- Titres de section en capitales grises (« COLONNES », « MISE À JOUR », « AUDIENCE ») qui ne structurent pas : les champs de la grille du haut n'ont pas le même alignement que les sections du bas.

### Contrôles

- **Dossier** : liste native à plat (« Main », « Main › submaub »…) qui ne montre pas l'arbre, avec à côté un champ « Nouveau dossier » et un bouton gris « Créer » toujours visibles ; on ne voit pas dans quel dossier le nouveau sera créé (le dossier choisi dans la liste, implicite).
- **Colonnes** : cases en ligne qui passent à la ligne sans grille, sans type, sans « tout / aucune », sans recherche ; la liste native du dossier ouverte les recouvre.
- **Serveur** : liste native même quand il n'y a qu'un serveur.
- **Clé** (mise à jour par clé) : liste native, apparue sous les boutons radio sans lien visuel.
- **Audience** : lignes à plat avec « Retirer / Garder » en lien, pas de pastilles ; le filtre des lignes est très technique (voir 1.6).

### Textes

- Phrases longues, jargon (« pipeline », `donnees.`, « source poussée ») dès le premier écran.
- Choix de la voie : deux paragraphes de trois lignes ; un classeur collaboratif affichait une voie grisée et un refus en rouge (« Un classeur collaboratif ne se republie pas : ce tableau n'y entre plus avec le classeur. ») que l'utilisateur ne comprenait pas.

### Erreurs et validation

- **Bouton principal désactivé sans dire pourquoi** (sauf une infobulle) : nom vide, aucune colonne, clé manquante, filtre invalide.
- Nom SQL non contrôlé avant l'envoi : l'erreur ne venait que du serveur.
- Filtre SQL libre : aucune erreur affichée sous la zone (le message n'apparaissait qu'à l'appelant, et pas partout).

### Clavier et accessibilité

- Entrée ne publiait pas ; Échap fermait toute la boîte même depuis une liste ouverte.
- Choix de la voie : pas d'Entrée ni de double-clic pour continuer.
- Pas de bouton de fermeture ni d'aide dans l'en-tête ; boîte nommée par `aria-label` fixe au lieu de son titre.
- Listes natives aux styles du système, qui jurent avec le reste de l'application (liste déroulante `ui/Select.svelte` ailleurs).

### Filtre des lignes visibles (audience)

- Onglet « SQL libre » avec une zone vide et une aide grise sur `current_user` et `lattis.my_teams()` : aucun moyen de connaître les noms des colonnes ni la syntaxe attendue.
- Formulaire : opérateurs proposés quel que soit le type (« contient » sur un nombre), valeurs spéciales « moi » / « mon équipe » peu claires, aucune relecture de la règle.

### Cohérence

- Les deux formulaires (tableau seul, tableau d'un classeur publié) dupliquaient le même balisage et les mêmes styles avec de petites différences.

## 2. Décisions

1. **L'essentiel d'abord, le reste replié.** Nom, Dossier, Colonnes, Audience (et « Utilisée par ») dans une grille à une colonne de libellés alignés. Description, Nom SQL, Mise à jour (mode, clé), Envoi (à chaque enregistrement, suspendre) dans **« Options avancées »**, replié, avec **un résumé d'une ligne** à côté du titre (« donnees.ventes · Mettre à jour par clé (ID) · à chaque enregistrement ») : rien n'est caché sans être dit. Le volet se déplie tout seul si un champ qu'il contient est à corriger.
2. **Bandeau d'une ligne** (« Le classeur reste sur votre poste : seules les colonnes cochées sont envoyées. ») avec **« En savoir plus »** qui ouvre l'aide (`fichiers#partager-un-tableau-au-data-lake-lattis`), et un « ? » dans l'en-tête.
3. **Dossier : arbre déroulant** (`FolderPicker.svelte`) : bouton qui montre le chemin, arbre dépliable (flèches haut/bas, droite/gauche pour déplier/replier ou aller au parent, Entrée pour choisir, Échap pour fermer sans fermer la boîte), recherche au-delà de 8 dossiers, et **« Nouveau dossier dans « X » »** en bas, qui crée dans le dossier en surbrillance et le choisit. Logique dans `data.ts` (`folderTree`, `folderRows`, `folderAncestors`, `folderPath`), testée.
4. **Colonnes : liste compacte** (`ColumnPicker.svelte`) en grille, type de chaque colonne (icône et infobulle), clé marquée, compteur « 5 sur 8 », Tout / Aucune, recherche au-delà de 10 colonnes ; hauteur bornée avec défilement.
5. **Mise à jour : contrôle segmenté** (Remplacer tout / Ajouter à la suite / Mettre à jour par clé), une ligne d'explication pour le mode choisi ; la clé apparaît juste dessous. La remarque sur les serveurs anciens n'apparaît que si le mode ou la clé change.
6. **Audience compacte** (`AudienceEditor.svelte`) : « Hériter de l'audience du dossier **X** » (le nom du dossier choisi), personnes et équipes en **pastilles** (✕ pour retirer, ↺ pour garder, « filtré » quand un filtre s'applique), ajout dans une carte avec adresse ou équipe.
7. **Filtre des lignes en clair** (`FilterEditor.svelte`, utilisé aussi par les droits) : formulaire par défaut, colonne avec son type, opérateurs selon le type (`opsFor`), valeurs proposées d'après les valeurs distinctes du tableau, valeurs spéciales « la personne connectée » et « une de ses équipes », et **la règle relue en une phrase** (« Visible si Région = « Nord » et Commercial = la personne connectée », `filterSentence`). En SQL libre : les colonnes en pastilles cliquables qui insèrent leur nom entre guillemets, des morceaux cliquables (égal à une valeur, la personne connectée, une de ses équipes, et, ou, est vide), l'erreur sous la zone. **« Rédiger avec l'assistant »** ouvre l'assistant avec une demande à compléter qui ne contient que les noms et les types des colonnes (`filterPrompt`), au-dessus de la boîte modale (comme l'aide).
8. **Validation sous les champs** (`shareProblems`, `sqlNameProblem`) : le bouton principal reste actif ; s'il manque quelque chose, la boîte met le focus sur le premier champ en cause, déplie les options avancées au besoin et le pied le dit en une ligne.
9. **Pied toujours visible**, action principale à droite, Retour / Retirer à gauche ; Entrée dans un champ texte publie, Échap ferme (sauf depuis une liste ou un arbre ouvert, qui se ferme seul) ; bouton ✕ et titre relié par `aria-labelledby`.
10. **Choix de la voie** : cartes plus courtes, Entrée et double-clic pour continuer, libellé du bouton selon la voie (« Publier le classeur… » / « Continuer »). Pour un tableau absent d'un classeur partagé en direct : titre « Ce tableau n'est que sur votre poste », une ligne d'explication, **seule la voie possible** (« Publier ce tableau seul au Data Lake ») et « Pourquoi pas tout le classeur ? » qui déplie une phrase d'explication, sans voie grisée ni refus en rouge (`chooser`). L'ajout d'un tableau à un classeur collaboratif n'existe pas (la synchronisation de structure ne sait que renommer ou supprimer un tableau, et ajouter, renommer, convertir ou supprimer des colonnes) : il n'est pas proposé.
11. **Une seule feuille de style** pour les deux formulaires (`shareForm.css`, classes préfixées `.sf-`), listes au style de l'application (`ui/Select.svelte`, qui reçoit un nom accessible).

## 3. Vérification

Écrans vérifiés dans le navigateur (mode web, faux serveur dans une page de test non gardée) à 1280×800 et 1024×700 : choix de la voie (classeur collaboratif), publication du tableau seul (création, erreurs, arbre des dossiers, création d'un dossier dans « Main », filtre en formulaire et en SQL libre avec erreur, options avancées), source existante (état, mode par clé), tableau d'un classeur publié (pastilles d'audience, « Utilisée par »). À 1024×700 le corps défile et le pied reste visible.
