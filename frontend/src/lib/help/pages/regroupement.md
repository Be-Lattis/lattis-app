# Onglets semblables et onglet à choix

Réunir des onglets qui ont les mêmes colonnes (un par mois, par année, par ville) en un seul tableau, puis n'en afficher qu'une valeur à la fois.

## Regrouper des onglets

Beaucoup de classeurs ont **un onglet par mois, par année ou par ville**, avec les mêmes colonnes. Lattis les repère : leurs onglets sont soulignés d'un même trait de couleur, et une puce « ⧉ 12 onglets semblables » suit la série (la survoler fait ressortir les onglets concernés). Après l'import d'un fichier .xlsx, à l'ouverture, ou quand vous copiez un onglet de la série pour la troisième fois (même avant de renommer la copie), la barre d'état le propose une fois : **Regrouper…**, **Plus tard** (la puce reste) ou **Ne plus proposer** (gardé dans le fichier : un collègue qui le reçoit n'est pas relancé ; aussi au clic droit sur la puce).

Les onglets vont ensemble quand au moins les deux tiers de leurs colonnes se retrouvent d'un onglet à l'autre : une colonne ajoutée en cours d'année reste vide là où elle manquait, et une colonne renommée (« Montant € ») s'apparie à sa voisine (« Montant »), ce que le volet montre toujours. Les noms d'onglets en série sont reconnus : mois, années, trimestres, semaines, dates, suites de chiffres ou de lettres (« Lot 1 », « Lot 2 » ; « Annexe A », « Annexe B »), communes.

**Données › Regrouper des onglets…** (ou clic droit sur une sélection d'onglets) ouvre le volet :

1. **Onglets** : les tableaux à réunir, à cocher, dans l'ordre des onglets ;
2. **Colonne de catégorie** : son nom, pré-rempli selon les noms d'onglets (Mois, Année, Trimestre, Ville, la partie commune comme « Lot », sinon Catégorie) et toujours modifiable, son type, et la valeur tirée de chaque onglet, à corriger au besoin. Elle se place en tête du tableau, avec une liste déroulante de ses valeurs ;
3. **Colonnes** : la correspondance (✓ présente, ∅ absente, « Montant € → Montant » appariée) et le type final ; une colonne qui passe en texte est signalée ;
4. **Après** : le nom du tableau et de la nouvelle feuille, et ce que deviennent les anciens onglets : **supprimés** (par défaut) ou **gardés masqués**, leurs tableaux renommés `Janvier_ancien` ;
5. **Aperçu** : les premières lignes et le nombre total de lignes, puis le bilan des formules qui citaient les anciens onglets : « 14 formules reprises, 2 à reprendre à la main », avec la liste (feuille, cellule, formule avant → après).

**Regrouper** fait tout en une seule étape : `Ctrl+Z` rend les onglets d'origine. La mise en forme et les saisies en erreur suivent leurs lignes. La **clé** est gardée si aucune valeur ne se répète d'un onglet à l'autre ; sinon le tableau commun n'a pas de clé (les valeurs restent), et Lattis le dit avec un exemple : « Pas de clé : F-001 est dans Janvier et dans Février ».

Ce qui citait les anciens tableaux est **repris** sur le tableau commun, limité au mois (à l'année, à la ville) de l'ancien onglet, dans la même étape d'annulation et avec les mêmes valeurs. Une feuille « Synthèse » qui faisait `SOMME(Janvier[Montant])`, `SOMME(Février[Montant])`… se reprend ainsi :

| Avant | Après |
|---|---|
| `SOMME(Janvier[Montant])` | `SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Janvier")` |
| `MOYENNE`, `MAX`, `MIN` d'une colonne (ou d'un calcul sur les colonnes, `SOMME(Janvier[Prix]*Janvier[Qté])`) | `MOYENNE.SI.ENS`, `MAX.SI.ENS`, `MIN.SI.ENS`, `SOMME.SI.ENS` |
| `NBVAL(Janvier[Client])` | `NB.SI.ENS(Ventes[Client]; "<>"; Ventes[Mois]; "=Janvier")` |
| `SOMME.SI`, `NB.SI`, `SOMME.SI.ENS`… | la même, en `.SI.ENS`, avec le critère du mois en plus |
| `LIGNES(Janvier)` | `NB.SI.ENS(Ventes[Mois]; "=Janvier")` |
| `NB`, `MEDIANE`, plusieurs colonnes dans `MAX` ou `MOYENNE`, `UNIQUE(Janvier[Client])`… | la colonne filtrée : `FILTRE(Ventes[X]; Ventes[Mois]="Janvier")` |
| un tableau croisé sur Janvier | le même sur Ventes, avec un filtre de rapport Mois = Janvier |
| un graphique sur Janvier | le même sur Ventes, avec un filtre Mois = Janvier (montré dans son volet, retirable) |

Les colonnes calculées des autres tableaux et les règles de mise en forme conditionnelle suivent les mêmes règles. Reste **à reprendre à la main**, avec la raison : une ligne précise (`Janvier[Montant][3]`), `REFERENCE`, `RECHERCHEV` et `EXISTE` (ils cherchent une clé dans un tableau nommé), `SOUS.TOTAL`, le tableau entier (`FILTRE(Janvier; …)`), une liste de contrôle tirée d'un ancien tableau, une requête ; le texte SQL et les scripts Lua ne sont pas repérés. Dans ce cas, les anciens onglets sont gardés masqués par défaut pour que ces formules continuent de marcher ; sinon ils sont supprimés. Sans colonne de catégorie, rien ne distingue plus les onglets : tout reste à reprendre. Après coup, une synthèse par mois gagne à devenir un tableau croisé Mois × Montant, qui suit seul les nouvelles lignes.

Le regroupement est refusé sur une feuille protégée, sur un tableau filtré (retirez d'abord le filtre), chargé par une requête ou partagé sur un serveur.

### Onglets à plusieurs tableaux

Un onglet « 2025 » porte souvent plusieurs tableaux : les ventes et les dépenses, qui changent chaque année, mais aussi une liste de tarifs recopiée telle quelle d'un onglet à l'autre. Lattis regroupe alors des **feuilles entières** : il apparie les tableaux d'un onglet à l'autre, famille par famille (mêmes colonnes, comme plus haut), et le volet montre, sous **Autres tableaux des onglets**, ce que devient chacune :

- **les regrouper** (tableaux qui changent : ventes, notes du mois) : un tableau commun avec la colonne de catégorie, qui **suit** l'onglet à choix ;
- **en garder un seul exemplaire** (proposé quand les données sont les mêmes partout : la liste de tarifs) : celui du dernier onglet, sans colonne de catégorie, le même pour toutes les années. Si un exemplaire diffère (« Tarifs change dans 2024 »), le volet le signale et propose plutôt de les regrouper, pour ne rien perdre ;
- **les laisser sur leur onglet** (proposé pour un tableau présent dans un seul onglet) : l'onglet est alors gardé, visible, avec un onglet orange pour le retrouver, et le volet le dit.

La feuille commune reprend la **disposition de l'onglet le plus récent** : chaque tableau commun prend la place qu'avait le sien, ce qui est en dessous descend d'autant qu'il a grandi, et les tableaux absents de cet onglet viennent ensuite, dessous. Les **cellules libres** des onglets sont reprises (case à décocher dans le volet) : un titre, un libellé ou un total identique partout n'est gardé qu'une fois ; `=SOMME(Ventes2025[Montant])` sous le tableau devient `=SOMME(Ventes[Montant])`, qui donne le total de l'année affichée ; une valeur qui change d'un onglet à l'autre (« Objectif : 5 000 » en 2024, « 6 000 » en 2025, un titre « Janvier 2025 ») devient une **cellule par année** : elle montre la valeur de l'année choisie, et ce que vous y tapez ne vaut que pour cette année (vide avec « (Toutes les années) »).

La catégorie peut aussi venir d'une **cellule** plutôt que du nom de l'onglet (un titre « Janvier 2025 » en A1 de chaque onglet, quand les onglets s'appellent Feuille1, Feuille2…) : le volet propose la source la plus sûre, au choix dans **Tirée du**. Des tableaux semblables **empilés sur une même feuille** sous des titres (« Paris », « Lyon », « Marseille ») se regroupent de même, la catégorie venant des titres. Si chaque onglet a une colonne **Date** dont les valeurs tombent toutes dans son mois, la colonne Mois est inutile : la case « Ne pas ajouter de colonne » est cochée d'office (le filtre par date suffit), décochez-la pour l'ajouter quand même.

**Suivre l'année** : sur une feuille à onglet à choix, un tableau qui suit porte une petite **pastille** avec l'année affichée au coin de son cadre ; un tableau sans pastille est le même pour toutes les années. Clic droit sur le tableau › Tableau › **Suivre l'année** (ou un clic sur la pastille) coche ou décoche : cocher ajoute la colonne Année en dernier, remplie avec l'année affichée (masquée tant qu'une année est choisie) ; décocher la retire si elle n'a qu'une année, sinon la garde (rien ne se perd) et le tableau montre toutes ses lignes. Un tableau mis en forme (Insertion › Tableau) sur une feuille réglée sur une année la suit d'office ; la barre d'état le dit. `Ctrl+Z` annule. Scripts : `tables.follow_choice("Tarifs", false)`.

## Onglet à choix (une année, un mois, une ville à la fois)

Au lieu d'un onglet par année, gardez un seul tableau avec une colonne Année et faites de sa feuille un **onglet à choix** : clic droit sur l'onglet › **Choisir par** › la colonne (Lattis propose celles qui ont peu de valeurs différentes). L'onglet devient « Ventes · Année ▾ » ; sa petite flèche (ou `Alt+↓` sur l'onglet) déroule la liste des valeurs :

- **une valeur** (2025) : tous les tableaux de la feuille qui ont une colonne de ce nom n'affichent que ses lignes, et la colonne Année, la même partout, est masquée. Les tableaux sans cette colonne (une liste de tarifs) ne changent pas. Les tableaux croisés de la feuille ne comptent que cette année, comme un filtre de rapport imposé ; `SOUS.TOTAL`, les graphiques et la recherche suivent comme ils suivent un filtre ;
- **(Toutes les années)** : plus rien n'est masqué, la colonne réapparaît ;
- **Nouvelle année…** propose la suivante (2026 après 2025, Décembre après Novembre, Lot 4 après Lot 3), l'ajoute à la liste déroulante de la colonne et l'affiche : la feuille paraît vide, prête à être remplie, comme un onglet neuf. `Ctrl+Z` l'annule.

**Depuis la colonne** : sur un tableau déjà rempli, clic droit sur l'en-tête de la colonne Année (ou son menu ▾) › **Filtrer l'onglet par cette colonne** fait la même chose, sans passer par l'onglet ni fusionner d'onglets. L'onglet commence sur toutes les valeurs : choisissez l'année avec sa flèche ▾. L'entrée est grisée, infobulle à l'appui, pour une colonne calculée, vide ou de plus de 100 valeurs différentes ; sur la colonne qui sert déjà de choix, elle devient **Ne plus filtrer l'onglet**. `Ctrl+Z` annule.

**Par une formule** : clic droit sur l'onglet › **Choisir par** › **Une formule…** quand la valeur à choisir n'est pas une colonne mais se calcule à partir des colonnes de chaque ligne : `=ANNEE([Date])` donne un onglet par année sans colonne Année, `=TEXTE([Date];"mmmm")` par mois, `=SI([Montant]>1000;"Gros";"Petit")` par tranche. La boîte propose les fonctions et les colonnes pendant la frappe, devine le nom de la catégorie (Année, Mois…, modifiable) et montre les premières valeurs de la formule et les tableaux de la feuille qui la suivront : ceux qui ont toutes les colonnes qu'elle cite (les autres ne changent pas ; Clic droit › Tableau › **Suivre** les en retire ou les y remet). La liste de l'onglet range les valeurs dans l'ordre naturel (2024 avant 2025, janvier avant février). Pas de colonne à remplir, donc pas de « Nouvelle année… » : une ligne tapée sous le tableau ne reçoit rien d'office, mais elle reste affichée pendant qu'on la saisit, jusqu'au prochain changement de valeur de l'onglet, où elle rejoint la valeur que donne sa formule. `=ONGLET.CHOIX()` rend la valeur choisie (un nombre pour `=ANNEE(…)`) ; les tableaux croisés de la feuille construits sur un tableau qui a les colonnes de la formule ne comptent que la valeur choisie. Renommer une colonne citée met la formule à jour. Scripts : `sheets.choose_by_formula("Ventes", "=ANNEE([Date])", "Année")`.

**Depuis un tableau croisé** : clic droit sur un champ des lignes, des colonnes ou des filtres dans le volet du tableau croisé (ou son ▾ dans la grille) › **Filtrer l'onglet par ce champ**. L'onglet à choix est posé sur la feuille du tableau croisé, par la colonne de son tableau source : le tableau croisé ne compte que la valeur choisie, comme les autres tableaux croisés de la feuille construits sur un tableau qui a cette colonne, et les tableaux de la feuille qui l'ont. Le tableau source, s'il est sur une autre feuille, n'est pas filtré ; ses valeurs remplissent la liste de l'onglet. Cela vaut aussi pour un tableau croisé construit sur un tableau chargé par une requête.

Une ligne tapée sous le tableau, ou dans la bande grisée des lignes des autres années, reçoit d'office l'année affichée ; de même pour une ligne collée ou insérée. Le choix n'est pas un filtre : « Effacer le filtre » le laisse, et vos filtres ▾ s'y ajoutent. `Ctrl+Alt+Pg.Suiv` et `Ctrl+Alt+Pg.Préc` passent à l'année suivante ou précédente ; la barre d'état rappelle « Année : 2025 (choix de l'onglet) ».

Les formules libres **de la feuille** voient ses tableaux filtrés : `=SOMME(Ventes[Montant])` posé sous le tableau donne le total de l'année affichée, comme dans l'ancien onglet « 2025 ». Celles des autres feuilles, l'onglet SQL et les scripts voient toutes les lignes. `=ONGLET.CHOIX()` rend la valeur choisie, pour une synthèse qui suit l'onglet : `=SOMME.SI.ENS(Ventes[Montant]; Ventes[Année]; ONGLET.CHOIX())`.

Changer d'année est de la navigation, comme changer d'onglet : ce n'est pas une étape d'annulation, mais la dernière valeur choisie est gardée dans le fichier et revient à l'ouverture (si elle n'a plus aucune ligne, l'onglet montre la dernière valeur, sauf juste après « Nouvelle année… »). Renommer la colonne renomme la catégorie : l'onglet suit. Clic droit › Choisir par › « Ne plus choisir par… » retire le choix (annulable). Dans un classeur partagé, le choix reste sur votre poste : il n'est pas envoyé aux autres.

Scripts : `sheets.choice()`, `sheets.set_choice("Ventes", 2025)`, `sheets.choose_by("Ventes", "Année")` (pour la feuille d'un tableau croisé posé ailleurs : `sheets.choose_by{table = "Ventes", column = "Année", sheet = "Synthèse"}`), `sheets.new_choice("Ventes", 2026)` (voir [Lua](lua)).
