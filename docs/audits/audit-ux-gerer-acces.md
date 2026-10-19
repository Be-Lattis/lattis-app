# Audit UX : « Gérer l'accès »

*19 octobre 2026. Boîte ouverte par Fichier › Gérer l'accès…, le menu Compte (« Gérer l'accès à « titre »… ») et le bouton « Modifier… » de l'onglet SQL (`frontend/src/lib/remote/RightsDialog.svelte`). Constat de départ (Louis) : l'écran doit rendre évident qui peut **voir**, qui peut **ajouter** des lignes, qui peut **modifier seulement ses lignes**, qui peut **tout modifier** et qui gère la **structure et les droits** ; or « participant » ou « restriction » ne disent pas ce qu'ils permettent. Référence de style : la refonte du partage au Data Lake (`audit-ux-partage-data-lake.md`, `shareForm.css`). Modèle : `docs/collaboration/droits.md`.*

## 1. Constats

### Vocabulaire

- **Les niveaux portent des noms de rôles** (Aucun, Lecteur, Participant, Éditeur) et ne disent pas ce qu'ils permettent. « Participant » (voir, ajouter, modifier et supprimer *ses* lignes) n'était expliqué que par une phrase grise sous l'arborescence.
- **« Restriction »** ne dit ni ce qui est retiré ni à qui. Une restriction s'affichait en `libellé · tableau · commande · rôles` suivi du prédicat SQL brut (`false`, `"Statut" = 'Ouvert'`).
- **« Commande »** (Tout, Lire, Ajouter, Modifier, Supprimer) et « Condition à respecter » : vocabulaire SQL ; rien ne dit qu'une restriction « Supprimer » avec la condition `false` veut dire « personne ne supprime ».
- « fourni / personnalisé · 3 + 1 équipe » dans la liste des rôles : le chiffre n'est pas qualifié.

### Membres

- **Grille de cases à cocher** une colonne par rôle : avec six rôles, une ligne par personne et des en-têtes de rôles sans explication ; on ne voit pas ce qu'une personne peut faire, seulement des cases.
- Rôles reçus par une équipe : « ✓ Ventes Nord » en petit bleu à côté d'une case vide, peu lisible.
- **Dernier propriétaire** : case grisée sans indication visible (infobulle seulement), « Retirer » qui disparaît sans dire pourquoi.
- Invitation : champ, cases de rôles en ligne et bouton désactivé tant que l'adresse ne contient pas « @ », sans dire pourquoi ; rien sur ce que donneront les rôles choisis.
- Équipes : même grille, puis un formulaire « Équipe… / rôle / Donner le rôle » et trois paragraphes gris.

### Rôles

- Liste à gauche, arborescence à droite : **aucune vue d'ensemble** de ce que chaque rôle permet ; il faut cliquer rôle par rôle.
- Listes natives « hérité (Lecteur) » : l'origine de l'héritage (feuille ou classeur) n'est pas dite.
- Filtres : un lien « Filtres… » / « Filtres ✓ » ; le filtre posé n'est visible qu'en ouvrant l'éditeur ; aucune distinction lecture / écriture dans la liste.
- Les restrictions qui touchent un rôle ne se voient pas depuis le rôle.

### Restrictions

- Formulaire « Tableau / Commande / Pour / Libellé / Condition » sans relecture ; aucun préréglage pour les besoins courants (interdire la suppression, ajout seulement, modifier selon un statut).
- **« Modifier ses lignes sans pouvoir en créer »** n'était exprimable qu'en sachant qu'il faut une restriction « Ajouter » de condition `false` sur le rôle participant.
- Bouton « Ajouter la restriction » désactivé sans dire ce qui manque.

### Voir en tant que…

- Un onglet à part, deux listes et « Voir » : pas d'accès depuis la ligne d'un membre ou d'un rôle, pas de résumé de ce que le membre peut faire, seulement des lignes.

### Écarts

- Un onglet au même niveau que Membres et Rôles, alors qu'il est rare et technique ; badge seulement.

### Clavier, accessibilité, cohérence

- Boîte nommée par `aria-label` fixe ; pas de bouton de fermeture ni d'aide dans l'en-tête ; onglets sans flèches ; Échap fermait même quand une liste ouverte devait se fermer seule.
- **Champs grisés pendant un envoi** : le focus partait sur le fond de la page, et Échap ne fermait plus la boîte après une erreur.
- Listes natives, boutons et liens aux styles disparates (lien bleu souligné, boutons gris) ; message et erreur en bas du corps, qui défile.

## 2. Décisions

1. **Capacités en clair, partout.** Cinq capacités nommées par ce qu'elles permettent : **Voir · Ajouter des lignes · Modifier ses lignes · Modifier toutes les lignes · Structure et droits**. Elles se déduisent du niveau (lecteur = Voir ; participant = Voir + Ajouter + Modifier ses lignes ; éditeur = + Modifier toutes les lignes ; concepteur = tout + structure, pas les droits ; propriétaire = tout), puis sont réduites par les filtres (« en partie ») et les restrictions qui visent le rôle (« non » pour une restriction sans exception, « en partie » sinon ; modifier et supprimer interdits ensemble : plus de modification). Classeur figé : tout le monde voit, personne ne modifie les données. Logique dans `access.ts` (`levelCaps`, `tableCaps`, `roleCaps`, `memberCaps`, `capsSummary`), testée.
2. **Membres en pastilles.** Une ligne par personne : initiale, nom et adresse, **rôles en pastilles colorées** (✕ pour retirer un rôle donné en personne, **+ Rôle** pour en donner un), rôles reçus par une équipe en pointillés (« Participant · Ventes Nord »), et **ce que la personne peut faire en une ligne** (« Voir, ajouter, modifier ses lignes », « (en partie) » avec la raison en infobulle). **Dernier propriétaire** : pastille avec cadenas sans ✕, étiquette « dernier propriétaire », pas de « Retirer ». Retirer tous les rôles demande confirmation. Recherche au-delà de 8 membres. Équipes et invitations en attente dans la même mise en page.
3. **Invitation inline**, en carte en tête : adresse, rôles en pastilles à cocher, **ce qu'ils permettront** relu dessous (« Pourra : Voir seulement »), Entrée invite ; adresse vide ou invalide et aucun rôle : message sous le champ et focus sur l'adresse (`emailProblem`), bouton toujours actif.
4. **Rôles et droits : la matrice d'abord.** Un tableau rôle × capacités (✓ oui, ◐ en partie, – non, raisons en infobulle et pour les lecteurs d'écran), le nombre de membres et d'équipes, et un bouton « Voir en tant que… » par rôle ; une légende et « Nouveau rôle ». Cliquer un rôle le choisit.
5. **Accès tableau par tableau, libellé par ce qu'il permet.** Sous la matrice, pour le rôle choisi : classeur → feuilles → tableaux, chacun avec une liste *Aucun accès / Voir / Voir, ajouter, modifier ses lignes / Voir et tout modifier*, ou « Comme la feuille : Voir » / « Comme le classeur : … » ; l'héritage est dit à côté (« · hérité de la feuille »), avec ce que le nœud donne en clair. Le nom du niveau (lecteur…) reste en infobulle.
6. **Filtres en phrases** sous chaque tableau : « Voit seulement les lignes où Région = « Nord » » / « Voit toutes les lignes », « Modifie seulement les lignes où Commercial = la personne connectée » / « Ajoute des lignes, modifie et supprime les siennes », avec **Limiter…** ou **Modifier** ; l'éditeur (`FilterEditor`, formulaire, SQL libre, assistant) s'ouvre en place, sa relecture dit « Voit les lignes où… » ou « Ajoute et modifie les lignes où… » (`readback`), et prévient quand l'enregistrement fixe l'accès d'un tableau qui héritait. Les règles avancées qui visent le rôle sur ce tableau s'y lisent aussi, avec un lien vers leur onglet.
7. **« Règles avancées »** au lieu de « Restrictions ». Chaque règle en **une phrase** (`ruleSentence`) : « Personne ne peut supprimer de lignes dans « Ventes », sauf les propriétaires et concepteurs. », « Les membres du rôle Participant ne peuvent modifier, dans « Stocks », que les lignes où Statut = « Ouvert ». » ; les restrictions qui ne diffèrent que par la commande sont regroupées (une phrase, un « Retirer »).
8. **Préréglages** : **Interdire la suppression**, **Ajout seulement** (modifier et supprimer interdits), **Modifier seulement les lignes au statut…** (colonne de statut devinée, valeur à saisir), **Modifier ses lignes sans en créer** (ajout interdit au rôle participant). Ils remplissent le **même formulaire en clair** : Tableau, Pour (tous les rôles / certains, en pastilles), Interdire (Voir, Ajouter, Modifier, Supprimer, Tout — une restriction par action), Exception (aucune = `false`, ou « Sauf les lignes où… » avec `FilterEditor`), Nom ; la règle est **relue en une phrase** avant l'ajout. Champs manquants sous leur champ, focus sur le premier (`draftProblems`).
9. **« Voir en tant que… » comme outil de vérification** : bouton sur chaque membre et chaque rôle (premier membre du rôle), qui ouvre l'onglet et affiche aussitôt le tableau ; un encadré dit ce que ce membre peut faire sur ce tableau (capacités et raisons), puis ses lignes. Sans la liste complète des droits (concepteur), seulement les lignes.
10. **Écarts secondaires** : onglet discret à droite, avec son badge ; un bandeau d'une ligne en tête des autres onglets quand il y en a (« Un tableau a des règles changées hors de Lattis » · Voir les écarts).
11. **Même socle que le Data Lake** : `shareForm.css` (champs, segments, boutons, pied toujours visible) plus `access.css` (pastilles, matrice, arborescence, règles), en-tête avec titre relié par `aria-labelledby`, Actualiser, « ? » (aide `fichiers#gerer-l-acces-a-un-classeur-partage`) et ✕ ; onglets au clavier (flèches, Début, Fin) ; Échap ferme sauf depuis une liste ouverte ; message et erreur du serveur dans le pied ; focus rendu après un envoi (les champs sont grisés pendant l'envoi) et mis sur l'adresse à l'ouverture ; listes `ui/Select.svelte` nommées.
12. **Rien ne change côté pont** : mêmes appels (`ShareRights`, `ShareInvite`, `ShareGrantRole`, `ShareRevokeInvitation`, `ShareCreateRole`, `ShareDropRole`, `ShareSetLevels`, `ShareAddRestriction`, `ShareDropRestriction`, `SharePreview`, `ShareRecompile`, `FieldValues`), une restriction par action envoyée l'une après l'autre.

## 3. Vérification

Écrans vérifiés dans le navigateur (mode web, faux moteur dans une page de test non gardée : six rôles dont un personnalisé filtré, une équipe, un membre par équipe, un pipeline, deux restrictions, une invitation) à 1280×800 et 1024×700 : Membres (pastilles, dernier propriétaire, + Rôle, invitation vide puis refusée par le serveur puis envoyée avec Entrée, focus rendu, Échap), Rôles et droits (matrice, rôle participant avec niveau de feuille hérité, rôle personnalisé avec filtres en phrases, édition et enregistrement d'un filtre), Règles avancées (préréglage « Modifier seulement les lignes au statut… » sans valeur puis complété : deux restrictions regroupées en une phrase ; « Modifier ses lignes sans en créer »), Voir en tant que… depuis la ligne d'un membre, concepteur (Voir en tant que… et Écarts seulement), classeur figé, écarts et recompilation. Onglets au clavier.

## 4. Reste

- La boîte « Publier sur un serveur » (`ShareDialog.svelte`) n'est que restylée : son choix des rôles à la publication pourrait reprendre les capacités en clair.
- Les capacités d'un membre sont calculées en local d'après la liste des droits ; le serveur reste seul juge (« Voir en tant que… » le montre).
