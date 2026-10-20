# Regrouper les onglets aux mêmes colonnes

*Étude de conception, 19/10/2026, demandée par Louis. Pas de code. Lue sur `main` de lattis-app (`ead78d3`) : `internal/canvas` (import, copie de feuille, fusion de tableaux, report des renommages), `internal/transfer/xlsximport.go`, `internal/query` (étape « ajouter » de l'équivalent de Power Query), `docs/decisions/0005`.*

## En bref

- **Le besoin** : beaucoup de classeurs ont un onglet par mois, par année, par ville… avec les mêmes colonnes. Lattis peut le repérer et proposer de tout ranger dans **un seul tableau** avec une colonne de plus (Mois, Année, Ville…) qu'on choisit dans une liste déroulante ou par le filtre.
- **Détecter est facile.** Tout est déjà dans les métadonnées (`_lattis_tables`, `_lattis_columns`) : on compare les noms et les types des colonnes des tableaux de feuilles différentes, sans lire une seule ligne. On regarde ensuite si les noms des onglets forment une série (janvier → décembre, 2021 → 2025, des noms de villes). C'est instantané, même sur un gros classeur.
- **Fusionner les données est de difficulté moyenne.** Une bonne partie existe déjà : le chargement hors journal de l'import, la corbeille de `deleteTable`, l'élargissement des types (`canvas.Join`), les clés d'ordre (`order.Spread`) et le report des renommages (`rewriteRefs`). Il faut renuméroter les lignes (styles et saisies invalides y sont rattachés) et faire correspondre les colonnes par leur nom.
- **Le vrai travail est ailleurs.** Il faut reprendre ce qui citait les anciens onglets (une feuille « Synthèse » avec `SOMME(Janvier[Montant])`, des TCD, des graphiques). Et il faut une habitude qui remplace « je duplique l'onglet du mois dernier » : un **onglet à choix**. C'est un seul onglet, partagé par tous ses tableaux, avec une petite flèche sur l'onglet lui-même pour choisir l'année, le mois ou la ville affichés (décision de Louis, 19/10). La commande « Nouvelle année » fait partie de la même liste.
- **Estimation** : un premier lot utile (détection, dialogue, fusion annulable, liste déroulante) fait environ une taille **M** dans l'échelle du plan. La reprise des formules ajoute un M, et l'onglet à choix un autre M. L'onglet à choix remplace aussi les « onglets-vues » que j'avais d'abord envisagés (un onglet par mois servant de fenêtre sur le tableau commun), qui auraient été un L.

## 1. Les cas qu'on rencontre

| Cas | Exemple | Ce qu'on en fait |
|---|---|---|
| **A. Un onglet par période** | `Janvier`, `Février`… ou `2023`, `2024`, ou `01-2025`, `T1 2025` | Le cas principal. La catégorie sort du nom de l'onglet, avec un vrai type : date, entier pour une année, texte ordonné pour les mois sans année |
| **B. Un onglet par entité** | `Paris`, `Lyon`, `Marseille` ; `Client A`, `Client B` | Même chose, catégorie en texte. On propose « Ville » si les noms sont des communes (`internal/geo` les connaît déjà), sinon « Catégorie », à renommer |
| **C. Historique figé** | `Stock 2024-03-01`, `Stock 2024-04-01` (photo prise à chaque date) | Comme A. Mais la clé (référence produit) se répète d'une photo à l'autre : la clé du tableau commun ne peut pas être la référence seule (voir §7) |
| **D. Variantes** | Une colonne ajoutée en cours d'année, renommée (« Montant » → « Montant € »), déplacée ; un onglet avec une ligne de total ou un titre au-dessus | Toléré : colonnes réunies (`NULL` là où elles manquent), correspondance des noms tolérante, titres et totaux laissés hors du tableau (ce sont déjà des légendes, décision 0005) |
| **E. Plusieurs tableaux empilés sur une feuille** | Un titre « Paris » au-dessus d'un tableau, « Lyon » au-dessus du suivant | Même mécanique, catégorie tirée du **titre** (légende) au lieu de l'onglet. Deuxième temps |
| **F. Catégorie déjà dans les données** | Chaque onglet mensuel a une colonne Date dont toutes les valeurs tombent dans ce mois | Inutile d'ajouter « Mois » : on le dit, et le filtre par date suffit. Option « Ajouter quand même une colonne Mois » |
| **Hors sujet** | Années en colonnes (Pays · 2024 · 2025) | C'est un dépivotage, que l'équivalent de Power Query fait déjà (`OpUnpivot`). On pourrait le proposer un jour avec le même genre de suggestion, mais c'est une autre fonction |

Ce qu'on ne regroupe **pas** : une feuille de synthèse (colonnes différentes), un TCD, un tableau chargé par une requête (il sera écrasé à l'actualisation : on proposera plutôt d'ajouter une étape « Ajouter une requête », qui existe déjà), un tableau lié du Data Lake Lattis ou d'un classeur partagé sur un serveur (au premier lot du moins), une feuille protégée.

## 2. Comment détecter

### 2.1 Candidats

Pour chaque feuille (masquées comprises), on prend ses tableaux ordinaires. C'est une simple lecture de `_lattis_tables` jointe à `_lattis_columns`, sans les colonnes techniques `_…` (décision 0007).

On calcule la **signature** de chaque tableau : la liste de ses noms de colonnes mis à plat (minuscules, sans accents avec `foldAccents`, espaces resserrés, ponctuation de fin retirée, « n° » = « no » = « numéro »), avec leur type.

### 2.2 Regroupement

Deux tableaux de **feuilles différentes** vont ensemble si :

1. **Les colonnes se recouvrent, avec de la tolérance** (règle de Louis, 19/10) : au moins **deux tiers** des colonnes de chacun des deux tableaux se retrouvent dans l'autre, et au moins 2 colonnes sont communes. Le calcul se fait sur les noms mis à plat : nombre de colonnes communes divisé par le nombre de colonnes du plus grand des deux tableaux. Par exemple, A·B·C et B·C·D ont 2 colonnes communes sur 3 : ils vont ensemble, et le résultat aura A·B·C·D, avec des cases vides là où une colonne manquait. Une colonne qui ne correspond à rien peut s'apparier à une colonne restante du même rang et du même type si son nom est proche (distance d'édition ≤ 2, ou l'un contient l'autre : « Montant » / « Montant € »). Cet appariement-là est **montré** dans le dialogue, jamais fait en silence.
2. **La clé est la même** (règle de Louis) : si au moins un des tableaux a une clé, tous ceux qui en ont une doivent l'avoir sur une colonne **du même nom**. Si les clés sont sur deux colonnes de noms différents (« N° client » ici, « Référence » là), les tableaux ne vont pas ensemble. Un tableau sans clé peut rejoindre un groupe à clé si la colonne clé existe chez lui. Les **collisions** (une même valeur de clé dans deux onglets) se vérifient à part (§7, étape 5), car elles demandent de lire les données.
3. **Les types sont compatibles** : `canvas.Join` ne doit pas faire passer en texte une colonne qui est un nombre dans plus de la moitié des tableaux. Une colonne qui est en texte dans un seul onglet (une saisie ratée) devient du texte : on le signale.
4. **Les tailles se ressemblent** : on n'écarte pas un tableau de 3 lignes à côté d'onglets de 300 (le mois en cours), mais on ne pousse pas vers un groupe un tableau qui a 2 fois plus de colonnes.

On fait des groupes par union-find sur ces paires. La tolérance a un piège : de proche en proche, A·B·C ~ B·C·D ~ C·D·E ~ D·E·F ferait un groupe où A·B·C et D·E·F n'ont plus rien en commun. On vérifie donc le groupe entier, une fois formé. Ses colonnes **majoritaires** sont celles qui sont présentes dans plus de la moitié des tableaux. Chaque tableau doit en avoir au moins les deux tiers, sinon il sort du groupe, en commençant par le plus éloigné. Un groupe doit compter **au moins 3 tableaux**, ou 2 si les noms d'onglets forment une série reconnue (« 2024 », « 2025 », ou « Lot 1 », « Lot 2 »). Une feuille qui porte plusieurs tableaux participe à plusieurs familles (par exemple Ventes et Dépenses dans chaque onglet d'année) : on regroupe alors les **feuilles entières**, voir §6.5.

### 2.3 Indice de confiance

Le score décide si on **propose** spontanément ou si on attend qu'on le demande :

| Signal | Poids |
|---|---|
| Colonnes identiques (mêmes noms, même ordre) dans tout le groupe | fort |
| Noms d'onglets en série : mois (FR/EN, abrégés, `01`…`12`), années, mois-année (`janv-25`, `2025-01`, `01/2025`), trimestres, semaines, dates complètes | fort |
| **Simple suite de chiffres ou de lettres** (ajout de Louis, 19/10) : `1`, `2`, `3` ; `A`, `B`, `C` ; avec un préfixe ou un suffixe commun (`Lot 1`, `Lot 2` ; `Annexe A`, `Annexe B` ; `Relevé-01`, `Relevé-02`). On isole la partie commune des noms, puis on regarde si ce qui varie est une suite d'entiers (trous permis : 1, 2, 4) ou de lettres. Les noms par défaut `Feuille1`, `Feuille2` comptent aussi, mais un peu moins, car ils ne disent rien du contenu | fort (moyen pour `FeuilleN`) |
| Noms d'onglets de même forme (`Agence Nord`, `Agence Sud` ; communes reconnues) | moyen |
| Onglets copiés les uns des autres (la copie de feuille, `CopySheet`, est un signal net : on peut la noter dans `_lattis_info` au moment de la copie) | moyen |
| Mise en forme identique des colonnes | faible |
| Une feuille qui cite tous les tableaux du groupe (une synthèse) | fort : c'est le cas typique, et le dialogue devra la reprendre |

On ne propose seul qu'au-dessus d'un seuil élevé. En dessous, la commande manuelle trouve quand même le groupe et le présente cochable.

### 2.4 Coût

Que des métadonnées : quelques dizaines de lignes même pour 100 onglets. Seul le cas F lit des données : un `min`/`max` de la colonne date de chaque tableau, indexé ou non, ça reste négligeable. La détection peut tourner à chaque ouverture et après chaque import sans se voir.

## 3. Déduire la colonne de catégorie

| Noms des onglets | Nom pré-rempli (modifiable, décision de Louis) | Type | Valeur pour `Janv 2025` |
|---|---|---|---|
| Mois avec année | Mois | date (1er du mois), format `mmmm aaaa` | 2025-01-01 |
| Mois sans année | Mois | texte, mais **ordonné** (le tri suit le calendrier) ; si le classeur ou les dates du tableau donnent l'année, on propose de passer en date | « Janvier » |
| Années | Année | entier | 2025 |
| Trimestres | Trimestre | texte « 2025-T1 » (triable) | — |
| Suite de chiffres (`Lot 1`, `Lot 2`…) | la partie commune (« Lot »), sinon « Numéro » | entier (la partie qui varie) | `Lot 3` → 3 |
| Suite de lettres (`Annexe A`, `Annexe B`…) | la partie commune (« Annexe »), sinon « Lettre » | texte | `Annexe C` → « C » |
| Dates | Date | date | — |
| Communes | Ville | texte | — |
| Autre | Catégorie (nom en rouge comme les noms par défaut, à renommer) | texte | nom de l'onglet |

- Les reconnaissances de mois existent déjà en deux endroits (`formula/dates.go`, `frontend/src/lib/grid/numfmt.ts` et la recopie en série `fill.test.ts`). Il faut les mettre en commun côté Go dans un petit paquet de reconnaissance de séries.
- La catégorie peut aussi venir d'une **cellule libre** de la feuille (« Mois : janvier » en B1, un titre au-dessus du tableau). Si une même cellule libre change d'un onglet à l'autre et ressemble à la série, on la propose comme source, au choix avec le nom de l'onglet.
- La colonne se place **en premier** dans le tableau commun. Elle reçoit une **liste de validation** (le style `dv` déjà utilisé pour les listes déroulantes) avec les valeurs trouvées : une nouvelle ligne se saisit par un choix dans la liste. Le menu ▾ du tableau filtre déjà par valeur.

## 4. Quand le proposer

La suggestion doit rester discrète : on la fait à quatre moments, jamais dans une fenêtre bloquante.

1. **Juste après un import `.xlsx`.** C'est par là qu'arriveront la plupart de ces classeurs. Une bulle dans la barre d'état, au style « info » : « 12 onglets ont les mêmes colonnes (Janvier à Décembre). Les regrouper en un seul tableau ? » avec Regrouper… et Ne plus proposer. L'import lui-même ne change pas : il garde les onglets tels quels, puis la détection tourne.
2. **À l'ouverture** d'un classeur où le groupe existe et n'a jamais été refusé.
3. **À la troisième copie d'un onglet** de la même famille (« Copier la feuille » de Mars après Janvier et Février). C'est le moment où l'utilisateur prend l'habitude : on lui montre l'autre façon.
4. **À la demande** : Données › « Regrouper des onglets… » (toujours là, avec la détection en pré-remplissage), au clic droit sur un onglet ou une sélection d'onglets (la multi-sélection d'onglets existe déjà), et dans l'Assistant ✦ / le MCP (« regroupe mes onglets mensuels »).

« Ne plus proposer » se range dans `_lattis_info` (clé `suggest.mergesheets.dismissed`, avec la signature du groupe), donc dans le fichier : un collègue qui reçoit le classeur ne sera pas relancé non plus. Aucune migration n'est nécessaire.

### 4.1 L'affichage de la proposition (validé par Louis, 19/10)

La proposition se fait en trois temps, du plus discret au plus détaillé :

1. **Dans la barre d'onglets, en permanence** : les onglets du groupe sont soulignés d'un même trait, de la couleur d'accent, ce qui montre d'un coup d'œil qu'ils vont ensemble. Au bout de la série, une **puce** en pointillés : « ⧉ 12 onglets semblables ». Survoler la puce fait ressortir les onglets concernés, même ceux qui sont hors de vue (la barre défile jusqu'à eux, et le nombre caché s'affiche à côté des flèches de défilement). Quand il y a trop d'onglets pour la barre, la puce reste épinglée à droite, près du « + ». Clic droit sur la puce › « Ne plus proposer » ; elle disparaît alors, et le trait aussi.
2. **Une seule fois, au bon moment** (après un import xlsx, à l'ouverture, à la troisième copie d'onglet) : une bulle d'information dans la barre d'état. « 12 onglets (Janvier à Décembre) ont les mêmes colonnes. Les regrouper en un seul onglet « Ventes · Mois ▾ » ? », avec **Regrouper…**, Plus tard et Ne plus proposer. « Plus tard » ferme la bulle, mais la puce reste.
3. **Le volet Regrouper** (§5), ouvert par la puce ou la bulle, avec un **aperçu avant / après**. Avant : la liste des onglets cochés avec leurs tableaux et leurs nombres de lignes. Après : la future barre d'onglets (« Synthèse », « Ventes · Décembre ▾ ») et la disposition des tableaux, chacun avec sa pastille s'il suit le mois. Les avertissements (colonne appariée, clé retirée, formules à reprendre) sont en orange, comme dans la barre d'état.

Plusieurs groupes dans un même classeur (onglets par mois **et** onglets par agence) : chaque groupe a son trait, de couleurs différentes, et sa puce. La bulle ne parle que du groupe le plus grand.

## 5. Le dialogue « Regrouper des onglets »

Un volet de 500 px comme les autres, en trois blocs.

1. **Onglets** : la liste cochable des tableaux du groupe (feuille · tableau · lignes), dans l'ordre des onglets. On peut en ajouter un ou en retirer.
2. **Colonne de catégorie** : nom (« Mois »), type, source (« nom de l'onglet » ou « cellule B1 »), et un tableau qui montre la valeur tirée de chaque onglet. Chaque valeur peut être corrigée dans la case. Cas F : la case « Ne pas ajouter de colonne : la colonne Date suffit » est cochée d'office.
3. **Colonnes** : la grille de correspondance. Une ligne par colonne du résultat, une colonne par onglet, avec ✓, ∅ (absente : vide) ou « Montant € → Montant » (appariée, modifiable). Le type final est indiqué, et un avertissement s'affiche quand une colonne passe en texte.
4. **Après** : où va le tableau (nouvelle feuille, nom proposé = partie commune des noms ou nom du fichier : `Ventes`), et ce que deviennent les anciens onglets (voir §6.2).
5. Un aperçu : les 20 premières lignes du résultat et le total des lignes. Puis le bouton vert **Regrouper**. Tout est une seule étape d'annulation.

## 6. Le résultat dans le classeur

### 6.1 Le tableau commun

- Une nouvelle feuille `Ventes`, un tableau `Ventes` en A1, avec la colonne `Mois` en premier puis l'union des colonnes, dans l'ordre du premier onglet, les colonnes ajoutées en cours de route à la fin.
- Les lignes suivent l'ordre des onglets, puis l'ordre `_pos` de chacun. On peut donc revenir en arrière sans rien perdre de l'ordre.
- Une **ligne de total** commune à plusieurs onglets ne se fusionne pas. Chaque total était déjà une légende hors données (décision 0005). On ne recopie pas ces légendes : la barre d'état propose d'ajouter un total au tableau commun, ou un TCD Mois × Montant, qui remplace naturellement ces totaux.

### 6.2 Les anciens onglets

Trois choix, avec une recommandation :

| Choix | Effet | Avis |
|---|---|---|
| **Supprimer les onglets** (recommandé) | Ils partent dans la corbeille de l'annulation (`deleteTable` sait déjà le faire) ; Ctrl+Z les rend | Simple, clair, une seule source de vérité |
| Garder une copie masquée | Les feuilles sont masquées et leurs tableaux renommés `Janvier_ancien` | Rassurant les premiers jours, mais deux copies divergent vite ; à proposer seulement si des références n'ont pas pu être reprises |

### 6.3 L'onglet à choix (décision de Louis, 19/10)

Il n'y a plus qu'**un onglet**, partagé par tous les tableaux de la feuille. Le choix de l'année (du mois, de la ville…) se fait **sur l'onglet lui-même**, grâce à une petite flèche qu'on déroule :

```
 ┌──────────────────┐┌──────────┐┌─────┐
 │ Ventes · 2025 ▾  ││ Synthèse ││  +  │
 └──────────────────┘└──────────┘└─────┘
          │
          ├ (Toutes les années)
          ├ 2023
          ├ 2024
          ├ ● 2025
          ├ ─────────
          └ + Nouvelle année…
```

**Ce que fait le choix**

- La feuille porte une **catégorie** (nom « Année », type, valeur choisie). Le choix filtre **tous les tableaux de la feuille** qui ont une colonne de ce nom, ainsi que les TCD de la feuille (par un filtre de rapport imposé) et les graphiques qui en dépendent. Un tableau sans cette colonne ne change pas.
- Quand une seule valeur est choisie, la colonne Année est **masquée** dans les tableaux de la feuille : elle vaut la même chose partout et ne ferait qu'encombrer. Avec « (Toutes) », elle réapparaît en première colonne.
- Une **ligne saisie ou collée** reçoit d'office l'année choisie. C'est ce qui rend l'onglet utilisable comme avant : on tape sous le tableau, comme dans l'ancien onglet « 2025 ». Ce filtre-là n'est donc pas un filtre utilisateur, et la règle `ErrFiltered`, qui refuse en partie la saisie sous un tableau filtré, ne s'y applique pas. Avec « (Toutes) », la saisie reste libre et la colonne Année se remplit à la main, via sa liste déroulante.
- Les filtres ▾ de l'utilisateur s'ajoutent au choix de l'onglet. « Effacer les filtres » ne retire pas le choix de l'onglet.
- **Formules** : une nouvelle fonction `ONGLET.CHOIX()` (`SHEETCHOICE()`) rend la valeur choisie, pour écrire une synthèse qui suit l'onglet, par exemple `SOMME.SI.ENS(Ventes[Montant]; Ventes[Année]; ONGLET.CHOIX())`. `SOUS.TOTAL`, qui suit déjà le filtre, marche aussi tel quel. Les formules **de la feuille** voient ses tableaux filtrés par le choix, ce qui fait que `SOMME(Ventes[Montant])` posé sous le tableau donne le total de l'année affichée, comme dans l'ancien onglet (voir §6.5). Les formules des autres feuilles voient tout le tableau, sans filtre.
- **SQL et scripts** voient toujours toutes les lignes. Lua reçoit `sheets.choice(nom)` et `sheets.set_choice(nom, valeur)`, ainsi qu'un évènement `choice` pour réagir au changement.
- **« + Nouvelle année… »** propose la valeur suivante de la série (2026 après 2025, Décembre après Novembre, Lot 4 après Lot 3, grâce à la recopie en série), l'ajoute à la liste de validation de la colonne et règle l'onglet dessus : la feuille paraît vide, prête à être remplie, exactement comme un onglet neuf. Une valeur sans aucune ligne reste dans la liste tant qu'elle est dans la validation.
- **Clavier** : Alt+↓ sur l'onglet actif ouvre la liste. Ctrl+Alt+PgSuiv / PgPréc passent à la valeur suivante ou précédente (Ctrl+PgSuiv reste le changement d'onglet).
- La barre d'état rappelle « Année : 2025 (choix de l'onglet) » et le nombre de lignes cachées.

**Ce qui est enregistré**

- La catégorie de la feuille et la dernière valeur choisie sont enregistrées dans `_lattis_sheets`, comme le zoom ou les volets figés. Cela demande une migration qui ajoute une colonne JSON `choice`. Changer de valeur est de la **navigation** : ce n'est pas une étape d'annulation, de même que changer d'onglet n'en est pas une. Définir ou retirer la catégorie de la feuille, en revanche, est annulable.
- Dans un classeur partagé, le choix doit rester **propre à chaque personne** (deux collègues regardent deux années différentes). Il faudra donc le garder sur le poste et non dans les données synchronisées. C'est à prévoir dès le début, même si la fusion elle-même reste refusée sur un classeur partagé au premier lot.
- *Fait (lot 5, en partie)* : la catégorie est publiée avec la mise en page, cellules par valeur comprises, et la valeur choisie reste sur le poste ; un membre qui ouvre le classeur voit l'onglet à choix sur la dernière année. Une catégorie posée, changée ou retirée par un concepteur après la publication part au serveur et arrive chez les autres sans toucher à leur valeur choisie. Voir `collaboration/synchronisation.md` §2.13.

**En dehors de la fusion**

L'onglet à choix est utile tout seul. Un clic droit sur un onglet › « Choisir par… » › une colonne (proposée seulement si elle a peu de valeurs différentes : moins de 100, par exemple) permet de le poser sur n'importe quelle feuille. Un tableau déjà commun, avec une colonne Ville, devient ainsi un onglet « par ville » sans rien fusionner.

**Export** : à l'export xlsx/ods, on propose « Un onglet par année », qui rend des onglets séparés aux personnes restées sur Excel, ou « Un seul onglet ». C'est l'« éclatement » du §9, en plus simple.
### 6.4 Ce que l'utilisateur y gagne (pour l'expliquer dans la bulle)

- Un TCD ou un graphique sur toute l'année en deux clics, au lieu d'une synthèse à formules qu'on recopie chaque mois.
- Une seule mise en forme et une seule MFC à entretenir.
- Le SQL : `SELECT Mois, sum(Montant) FROM Ventes GROUP BY Mois`.
- Une colonne ajoutée une fois pour toutes, et non dans douze onglets.

### 6.5 Plusieurs tableaux par onglet : lesquels suivent l'année ? (question de Louis, 19/10)

Un onglet « 2025 » contient souvent plusieurs tableaux : Ventes et Dépenses, qui changent chaque année, mais aussi Tarifs ou Liste des commerciaux, recopiés tels quels d'un onglet à l'autre. Une fois tout ramené sur une seule feuille, il faut savoir quels tableaux suivent le choix de l'onglet. Il faut aussi savoir quoi faire d'un tableau créé ensuite.

**La règle, une seule** : un tableau suit le choix de l'onglet **si et seulement s'il a la colonne de catégorie** (Année). Il n'y a pas d'autre réglage caché : ce qui décide, c'est la colonne, qu'on peut voir, renommer et interroger en SQL.

**Au regroupement**, on regroupe des feuilles entières. Les tableaux sont appariés famille par famille, avec la détection du §2.2. Pour chaque famille :

| Ce qu'on trouve d'un onglet à l'autre | Résultat sur la feuille commune |
|---|---|
| Mêmes colonnes, données différentes (Ventes) | un tableau avec la colonne Année, qui **suit** le choix |
| Mêmes colonnes et **données identiques** dans tous les onglets (Tarifs recopié) | **un seul exemplaire, sans colonne Année**, qui reste affiché quelle que soit l'année. On compare une empreinte des lignes (un hachage par tableau, calculé en SQL) |
| Données presque identiques (Tarifs mis à jour une année) | par prudence, comme des données différentes : colonne Année, rien n'est perdu. Le dialogue le signale (« Tarifs change en 2024 : suivra l'année ») et propose de ne garder que la version la plus récente |
| Tableau présent dans un seul onglet | gardé tel quel, sans colonne Année ; le dialogue propose « suivre l'année » s'il vient d'un onglet d'année |

La **disposition** reprend celle de l'onglet le plus récent : chaque tableau commun prend la place qu'il avait dans cet onglet. Les tableaux absents de cet onglet viennent ensuite, sous les autres, avec les règles d'écart existantes (`place`, `ensureGapBelow`). Le dialogue montre cette disposition en aperçu.

**Les cellules libres** (un titre, un objectif tapé à la main, un total) suivent le même principe. Une cellule identique dans tous les onglets n'est gardée qu'une fois. Une formule qui ne diffère que par le nom du tableau (`SOMME(Ventes2024[Montant])`, `SOMME(Ventes2025[Montant])`) devient une seule formule sur le tableau commun, qui donne le total de l'année affichée puisque les formules de la feuille voient le filtre. Une **valeur qui change d'un onglet à l'autre** (Objectif : 5 000 en 2024, 6 000 en 2025) devient une **cellule par année** : elle garde une valeur pour chaque année et montre celle de l'année choisie. Il faut pour cela une petite table `_lattis_choice_cells` (feuille, ligne, colonne, valeur de catégorie, texte). C'est une nouveauté, mais elle reste locale.

**Un tableau créé plus tard** sur la feuille :

- Si l'onglet est réglé sur **une année précise** au moment de la création, le nouveau tableau la suit : il reçoit la colonne Année, masquée et remplie avec l'année affichée. On reproduit ainsi l'ancienne habitude : on était dans l'onglet 2025, ce qu'on tape appartient à 2025. La barre d'état le dit : « Ce tableau suit l'année (2025). Clic droit › Tableau › Ne pas suivre l'année ».
- Si l'onglet est sur **(Toutes les années)**, le nouveau tableau ne suit pas.
- On change d'avis à tout moment avec une case du menu du tableau, « Suivre l'année ». La cocher ajoute la colonne Année, remplie avec l'année affichée ; la décocher la retire (en avertissant si elle contient plusieurs années, car on perd alors cette information). C'est annulable, comme toute modification de colonne.
- Pour qu'on voie toujours ce qui suit et ce qui ne suit pas, un tableau qui suit l'année porte une petite **pastille « 2025 »** dans le coin de son cadre, de la couleur de l'onglet. Un tableau sans pastille est le même pour toutes les années.

Ce qui reste délicat, c'est surtout l'appariement des tableaux d'un onglet à l'autre, quand ils ont changé de place ou de nom. Une fois les familles formées, la règle « colonne Année = suit l'année » rend le reste prévisible.

### 6.6 Grouper par une formule (demande de Louis, 20/10 : « grouper par une formule », « sur les onglets groupés et sur les deux »)

Souvent la catégorie n'est pas une colonne mais se calcule : l'année d'une date, le mois, une tranche de montant, le préfixe d'un code. Plutôt que d'ajouter une colonne Année à remplir, on choisit l'onglet **par une formule** sur les colonnes de chaque ligne. Il en va de même, dans un tableau croisé, pour regrouper les lignes par une formule (« Champ calculé… », voir plus bas). Code : `internal/canvas/choiceformula.go`, `pivotcalcfield.go`.

**Onglet à choix par une formule** (clic droit sur l'onglet › Choisir par › **Une formule…**)

- **Ce qui est enregistré** : la colonne JSON `choice` de `_lattis_sheets` reçoit `formula` (la formule en anglais canonique, colonnes citées par leur nom comme dans une colonne calculée : `=YEAR([Date])`) et `name` (le nom de la catégorie, deviné par la boîte : Année, Mois, la colonne d'un `GAUCHE`…, modifiable) ; `tbl` et `col` restent vides. Pas de migration : `choice` est déjà du JSON.
- **Qui suit** : la même règle qu'au §6.5, transposée. Un tableau de la feuille suit l'onglet **si et seulement s'il a toutes les colonnes citées par la formule** ; il faut qu'elle en cite au moins une (`=1` est refusée). Un tableau sans ces colonnes ne change pas. « Suivre » (clic droit › Tableau) ne fait que lever ou poser le réglage `follow` du tableau : il n'y a pas de colonne à ajouter, et cocher sur un tableau qui n'a pas les colonnes est refusé avec leur nom. Un tableau créé ensuite suit s'il a les colonnes (rien à ajouter d'office).
- **Calcul** : la formule est compilée en SQL pour chaque tableau qui suit, avec le compilateur des colonnes calculées (`formula.CompileHoisted` dans la portée du tableau, agrégats calculés à part), puis comparée en **texte affiché** (`slt_label` : 2025, « VRAI », « #VALEUR! », une date au format affiché). La condition entre dans le filtre du tableau (`loadFilter`) : grille, `SOUS.TOTAL`, graphiques, recherche et formules de la feuille la voient comme le choix par colonne. Une formule qui ne se calcule pas pendant le calcul d'un autre filtre calculé n'est pas appliquée (garde contre la récursion, comme les filtres sur colonnes calculées).
- **Liste de l'onglet** : les valeurs distinctes non vides, rangées dans l'ordre naturel faute d'ordre de saisie : nombres croissants, puis textes ; des noms de jours ou de mois tous reconnus (listes prédéfinies du tri personnalisé) suivent le calendrier ; sinon ordre alphabétique avec les nombres inclus dans l'ordre (Lot 2 avant Lot 10).
- **Lignes ajoutées** : il n'y a pas de colonne où écrire la valeur choisie. Sans précaution, une ligne qu'on commence à taper sous l'onglet « 2025 » disparaîtrait dès la première cellule (`=ANNEE([Date])` ne vaut pas 2025 tant que la date est vide). Décision : la ligne ajoutée (saisie, collage, insertion) est **gardée visible** (`keep` dans `choice`, dans l'étape d'annulation de l'ajout, au plus 500 lignes) jusqu'au prochain changement de valeur de l'onglet ; elle rejoint alors la valeur que donne sa formule. Le choix n'impose donc rien aux données, et le refus `ErrFiltered` ne s'applique pas plus qu'avec une colonne.
- **« Nouvelle année… »** n'a pas de sens (les valeurs viennent des lignes) : l'entrée est remplacée dans la liste par le rappel de la formule, et `sheets.new_choice` est refusé.
- **`ONGLET.CHOIX()`** rend la valeur choisie, en nombre quand elle s'écrit comme un nombre (`=ANNEE(…)` : 2025), en texte sinon ; vide sur « (Toutes) ».
- **TCD de la feuille** : le filtre imposé (`PivotConfig.Choice`) porte la formule (`PivotFilter.Formula`) quand le tableau source a ses colonnes ; il se calcule comme un champ calculé.
- **Renommage** : renommer une colonne citée, dans un tableau de la feuille, réécrit la formule (`rewriteChoiceFormulas`), et le tableau continue de suivre.
- **Classeur partagé** : la formule fait partie de la catégorie publiée avec la mise en page (`{name, formula}`) ; la valeur choisie et `keep` restent sur le poste. Deux catégories ne sont « la même » que si leurs formules sont identiques : une formule changée par un concepteur ramène les autres postes sur la dernière valeur.
- **API** : `sheets.choose_by_formula(feuille, formule, nom, [valeur])`, `formula` dans `sheets.choice()` ; MCP `sheets_choose_by_formula`.

**Tableaux croisés : champ calculé** (volet des champs › **Champ calculé…**)

- **Enregistré** dans la configuration du TCD : `calc`, une liste de `{name, formula}` (formule en anglais canonique ; `text` porte la formule en français à la lecture, et une formule saisie à l'écriture, depuis le volet ou l'API). Le nom ne doit pas être celui d'une colonne de la source.
- **Calcul** : les champs calculés utilisés deviennent des colonnes de la source du TCD, `(SELECT *, <formule> AS c_N FROM t_X) AS t_X`, avec le même compilateur ; le champ se place ensuite comme une colonne : regroupement en Lignes ou en Colonnes (`GROUP BY` sur son texte affiché, tri croissant sur sa valeur : 9 avant 10), filtre, valeur synthétisée. Un TCD avec un champ calculé ne garde pas son résultat en cache par version de sa seule source (la formule peut lire d'autres tableaux).
- **Modifier, supprimer** : par le menu ▾ du champ dans la liste (entrées propres au champ en tête) ; supprimé, il quitte les zones ; renommé, il garde sa place, son tri et son filtre. Changer la source du TCD retire les champs calculés. Renommer une colonne de la source réécrit les formules.
- **Pas encore** : une colonne calculée du tableau source ne sert toujours pas de champ (mais un champ calculé peut reprendre sa formule). L'étape « Regrouper par » des requêtes (`internal/query`, OpGroup) ne regroupe que par des colonnes ; grouper par une formule y passe aujourd'hui par une étape « Ajouter une colonne » (OpAddColumn, expression SQLite) avant le regroupement. Une option « Regrouper par une expression » serait une suite naturelle.

## 7. Implémentation côté moteur

Une nouvelle commande `Model.MergeSheets(ctx, MergePlan)` dans `internal/canvas` (fichier `mergesheets.go`), une seule étape d'annulation, appelée par le bridge, l'API (`tables.merge_sheets`, donc Lua, MCP et Assistant ✦ d'un coup) et le dialogue. La détection, `Model.SimilarSheets(ctx) []SheetGroup`, est une lecture pure.

```
MergePlan {
  Tables   []string            // tableaux sources, dans l'ordre
  Category *{Name, Type, Values []string /* une par tableau */}  // nil : cas F
  Columns  []{Name, Type, From map[source]colID}                  // correspondance validée
  Sheet    string              // feuille du résultat (créée)
  Name     string              // nom du tableau
  Old      "delete" | "hide"
  Slicer   bool
}
```

Étapes, dans la transaction :

1. **Garde-fous** : `guardWorkbook` ; refus si un tableau source est chargé par une requête, publié sur un serveur ou dans le Data Lake, si une feuille est protégée, ou si un tableau est filtré (on demande d'abord de retirer le filtre, sinon des lignes masquées pourraient surprendre).
2. **Table de chargement** hors journal, comme `Import` : `CREATE TABLE … STRICT` avec `id`, `_pos`, `c_<id>` neufs, puis un `INSERT … SELECT` par source. On y met la constante de catégorie, `CAST` vers le type final (déjà élargi par `canvas.Join`) et `NULL` pour une colonne absente. Les clés `_pos` viennent de `order.Spread(n)`. Les `id` sont décalés par source (`id + offset`) pour garder une correspondance simple.
3. **Métadonnées** : `_lattis_tables` (nouvelle ligne), `_lattis_columns` (mise en forme et MFC reprises du premier tableau qui a la colonne ; les règles différentes sont réunies si elles ne se contredisent pas, sinon on garde celles du premier et on le dit), `_lattis_invalid` et `_lattis_styles` recopiés avec `rid + offset` et la nouvelle colonne. Les largeurs de colonnes viennent du premier onglet.
4. **Colonnes calculées** : gardées comme formules si la formule est la même partout (après report des noms), sinon gardées en valeurs avec un avertissement.
5. **Clé** : si les tableaux ont une clé (même nom, garanti par la détection), on cherche les **collisions** avant de valider. C'est une seule requête sur l'union des colonnes clés (`GROUP BY` clé `HAVING count(*) > 1`, sans tenir compte de la casse comme l'index `t_<id>_key`). Elle est rapide, car chaque table a déjà son index unique sur la clé.
   - **Aucune collision** : la clé est gardée sur le tableau commun (index unique recréé), et les `REFERENCE` qui s'en servent marchent toujours.
   - **Collisions** (cas C, numéros de facture qui recommencent chaque mois) : on **retire seulement la notion de clé** du tableau commun, sans rien demander (règle de Louis, 19/10). La colonne et toutes ses valeurs restent telles quelles, seul l'index unique n'est pas recréé. Le dialogue l'annonce par une ligne d'information, avec un exemple (« Pas de clé : F-001 est dans Janvier et dans Février »), et la barre d'état le redit après le regroupement. Les `REFERENCE` qui s'appuyaient sur cette clé sont listées parmi les formules à reprendre.
6. **Références** (§8), avec `rewriteRefs` pour ce qui est un simple renommage et une réécriture propre pour le reste.
7. **Anciens onglets** : `deleteTable` pour chaque source puis suppression de la feuille si elle est vide, ou masquage avec renommage.
8. **Onglet à choix et validation** : style `dv` de colonne (rid 0) avec les valeurs ; catégorie de la feuille posée sur la nouvelle feuille, réglée sur la dernière valeur (le mois le plus récent).

L'étape « Ajouter une requête » (`OpAppend`, `internal/query/compile.go`) fait déjà le `UNION ALL` avec élargissement des types et `NULL` pour les colonnes manquantes. Elle peut servir à l'**aperçu** du dialogue, mais la fusion réelle doit garder les styles et les saisies invalides ligne par ligne, ce qu'une requête ne fait pas. On écrit donc la copie dans `canvas`, sur le modèle de `copyTable` (`sheetcopy.go`).

**Volumes** : tout reste dans SQLite (`INSERT … SELECT`), sans lignes en Go. Même 50 onglets de 100 000 lignes, ça représente quelques secondes, avec la même barre de progression que l'import.

**Collaboration** : refusée au premier lot sur un classeur partagé. Plus tard, ce sera une suite d'opérations de structure (création de tableau, ajout de lignes, suppression), à relayer au serveur comme les autres opérations de structure SQL déjà relayées.

## 8. Reprendre ce qui citait les anciens onglets

C'est la partie la plus délicate, parce que ce n'est pas un renommage : `Janvier[Montant]` devient « la partie de `Ventes[Montant]` où Mois = Janvier ». Le report actuel (`refs.go`) parcourt déjà tout ce qu'il faut (cellules libres, colonnes calculées, MFC, TCD, graphiques, contrôles, requêtes). Il faut lui donner une nouvelle sorte de changement : « tableau remplacé par un tableau filtré ».

| Ce qui citait `Janvier` | Devient | Exact ? |
|---|---|---|
| `SOMME(Janvier[Montant])` | `SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "Janvier")` | oui |
| `MOYENNE`, `MAX`, `MIN` d'une colonne | `MOYENNE.SI.ENS`, `MAX.SI.ENS`, `MIN.SI.ENS` | oui |
| `NBVAL(Janvier[X])` / `NB` | `NB.SI.ENS(Ventes[Mois]; "Janvier"; Ventes[X]; "<>")` (NB : à vérifier sur les textes) | presque |
| `REFERENCE(Janvier; Prix; clé)` / RECHERCHEV | `REFERENCE` sur un `FILTRE(Ventes; Ventes[Mois]="Janvier")` si REFERENCE accepte un tableau dynamique, sinon à reprendre | à vérifier |
| Toute autre utilisation d'une colonne entière | `FILTRE(Ventes[X]; Ventes[Mois]="Janvier")` | oui pour les fonctions qui prennent une plage |
| `Janvier[Montant][3]` (une ligne précise) | pas d'équivalent sûr | **à reprendre** |
| TCD sur `Janvier` | TCD sur `Ventes` avec un filtre de rapport Mois = Janvier (les filtres de rapport existent) | oui |
| Graphique sur `Janvier` | graphique sur une plage filtrée : à vérifier dans `chart.go` | à vérifier |
| Requête SQL, script Lua | hors de portée, comme pour les renommages | **à reprendre** |

Le dialogue affiche avant de valider « 14 formules reprises, 2 à reprendre à la main » avec la liste (feuille, cellule, formule). S'il reste des formules à reprendre, il propose de garder les anciens onglets masqués plutôt que de les supprimer. Une **feuille de synthèse** qui faisait `SOMME(Janvier[Montant])`, `SOMME(Février[Montant])`… se reprend proprement. La bulle suggère ensuite de la remplacer par un TCD.

## 9. Pour plus tard : l'éclatement

L'onglet à choix (§6.3) remplace les « onglets-vues » que j'avais d'abord envisagés, c'est-à-dire un onglet par mois servant de fenêtre sur le tableau commun. Il couvre le même besoin avec un seul onglet, et il n'a pas besoin d'une nouvelle sorte de tableau qui délègue à un autre.

Une fonction inverse simple, « Éclater par Mois » (un onglet par valeur, en copies), est utile pour envoyer un fichier à quelqu'un qui reste sur Excel. C'est un S, et l'export xlsx pourrait le proposer.

## 10. Difficulté et découpage

| Lot | Contenu | Taille | Risque |
|---|---|---|---|
| 1 | Détection (`SimilarSheets`, reconnaissance des séries), dialogue, `MergeSheets` avec colonnes réunies, catégorie typée, liste de validation, suppression annulable des onglets, commande d'API, bulle après import xlsx et menu Données | **M** | faible : surtout de l'assemblage de briques existantes |
| 2 | Reprise des formules, TCD et graphiques (§8), option « garder masqués », liste « à reprendre » | **M** | moyen : la réécriture de formules est bien outillée, mais les cas sont nombreux et il faut des tests d'intégration (`internal/integration`) |
| 3 | Onglet à choix : flèche sur l'onglet, filtre de feuille appliqué aux tableaux et TCD, colonne masquée, valeur imposée aux nouvelles lignes, « Nouvelle année… », `ONGLET.CHOIX()`, « Choisir par… » sur toute feuille, migration `_lattis_sheets.choice` | **M** | moyen : le filtre de feuille doit passer partout où passe le filtre de tableau (vue de la grille, SOUS.TOTAL, TCD, graphiques, export) |
| 4 | Feuilles à plusieurs tableaux (§6.5) : appariement par familles, tableaux identiques gardés une fois, disposition reprise de l'onglet le plus récent, cellules par année, pastille et « Suivre l'année » ; cas E (tableaux empilés sous des titres), catégorie tirée d'une cellule, suggestion à la copie d'onglet, cas F | **M** | moyen : l'appariement quand les tableaux ont bougé ou changé de nom |
| 5 | Éclatement à l'export, fusion et choix propre à chaque personne dans un classeur partagé (fait : catégorie publiée, choix propre à chaque poste, `collaboration/synchronisation.md` §2.13) | **M** | moyen |

Les lots 1 à 3 font une fonction complète et différenciante : c'est exactement ce qu'Excel ne propose pas. Il faut sinon passer par Power Query (Ajouter des requêtes) ou par un copier-coller à la main.

## 11. Proposition d'ajout au plan (non écrite)

En phase 9, un point 12 :

> **12. Regrouper les onglets aux mêmes colonnes** (demande de Louis, 19/10/2026) : détecter les classeurs à un onglet par mois, année, ville… (mêmes colonnes, noms d'onglets en série, y compris une simple suite de chiffres ou de lettres) et proposer, après un import xlsx, à l'ouverture ou depuis Données › « Regrouper des onglets… », de les réunir en un seul tableau avec une colonne de catégorie typée (Mois, Année, Ville), en liste déroulante, en une étape d'annulation. Reprise des formules qui citaient les anciens onglets (SOMME → SOMME.SI.ENS, TCD avec filtre de rapport), onglet à choix (une flèche sur l'onglet pour choisir l'année, le mois ou la ville ; le choix filtre tous les tableaux de la feuille et donne sa valeur aux nouvelles lignes ; « Nouvelle année… »), utilisable aussi sans fusion. Plus tard : éclatement par catégorie à l'export. Étude : `conception/regrouper-onglets-semblables.md`. Lots 1-3 ≈ M+M+M.

## 12. Décisions de Louis (19/10)

1. **Anciens onglets** : supprimés par défaut (annulable), sauf s'il reste des formules à reprendre ; ils sont alors gardés masqués.
2. **Onglet à choix** : à l'ouverture, il reprend la dernière valeur choisie, sauf juste après « Nouvelle année… ».
3. **Suggestion** : après un import xlsx, à l'ouverture et à la troisième copie d'un onglet, avec « Ne plus proposer » gardé dans le fichier.
4. **Nom de la colonne de catégorie** : toujours modifiable, dans le volet Regrouper, avec une valeur pré-remplie selon la détection (Mois, Année, Trimestre, Ville, la partie commune d'une suite comme « Lot », sinon Catégorie). Il reste modifiable après coup, en renommant la colonne, et l'onglet (« Ventes · Mois ▾ ») suit le nouveau nom.
5. Affichage de la proposition (§4.1), onglet à choix (§6.3), tolérance des deux tiers et règle des clés (§2.2, §7) : validés.
