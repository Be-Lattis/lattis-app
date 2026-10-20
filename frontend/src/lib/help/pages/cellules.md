# Mise en forme, notes et affichage

Les commandes du ruban suivent celles d'Excel, aux mêmes places. Elles s'appliquent à la sélection (ou à la cellule active) et s'annulent par `Ctrl+Z`.

## Mise en forme des cellules

Accueil › Police, Alignement et Nombre mettent en forme la sélection ; le petit lanceur dans le coin de chaque groupe, ou `Ctrl+1`, ouvre la boîte **Format de cellule** (nombre, alignement, police, bordure, remplissage). Cliquer sur l'alignement déjà choisi (Gauche, Centrer, Droite) revient à l'alignement standard, comme dans Excel.

## Couleurs de police et de remplissage

Accueil › Couleur de police et Couleur de remplissage (Surlignage dans un Document) : le bouton pose la dernière couleur choisie, la flèche ▾ ouvre la palette, comme dans Excel.

- **Couleurs du thème** : les dix couleurs du thème du classeur (Arrière-plan 1 et Texte 1, le blanc et le noir ; Arrière-plan 2 et Texte 2 ; Accent 1 à 6, dont la couleur des tableaux et celle des sources externes), et sous chacune cinq variantes plus claires ou plus foncées. Une couleur prise ici **suit le thème** : changer de thème (Affichage › Thème) recolore ces cellules.
- **Couleurs standard** : rouge foncé, rouge, orange, jaune, vert clair, vert, bleu clair, bleu, bleu foncé et violet, les mêmes quel que soit le thème.
- **Couleurs récentes** : celles choisies par « Autres couleurs… » sur ce poste.
- **Aucun remplissage** ou **Automatique** retire la couleur ; **Autres couleurs…** en choisit une librement.

L'infobulle de chaque pastille donne son nom (« Vert, Accent 1, plus clair 40 % »). Au clavier, les flèches parcourent les pastilles et `Entrée` choisit. La même palette sert dans Format de cellule (`Ctrl+1`) et dans la mise en forme conditionnelle. À l'export Excel ou OpenDocument, les couleurs du thème sont écrites telles qu'elles s'affichent.

Accueil › Édition › Effacer retire les formats, le contenu (`Suppr`) ou la note de la sélection. Remplissage recopie la première ligne vers le bas (`Ctrl+D`) ou la première colonne vers la droite (`Ctrl+R`).

Accueil › Mettre sous forme de tableau (`Ctrl+L`) fait un tableau du bloc de cellules libres de la cellule active, sa première ligne donnant les en-têtes. Sur une cellule vide, Insertion › Tableau (ou `Ctrl+L`) pose un tableau vide d'une colonne « Colonne 1 », l'en-tête sur la cellule : tapez dessous pour le remplir, dans l'en-tête pour renommer la colonne.

## Mise en forme conditionnelle

Accueil › Mise en forme conditionnelle colore les cellules selon leur valeur : mise en évidence (supérieur à, entre, texte qui contient, doublons…), barres de données, nuances de couleurs et jeux d'icônes. Le **gestionnaire des règles** liste les règles de la feuille, pour les modifier ou les supprimer. Dans un tableau, une règle vaut pour toute la colonne, lignes à venir comprises.

## Validation des données

Données › Validation des données donne la liste des valeurs permises dans la sélection, ou dans toute une colonne de tableau : la cellule affiche une flèche de liste déroulante (`Alt+Bas` l'ouvre), et une autre saisie est refusée.

La liste se saisit (**Valeurs saisies**, une par ligne) ou se tire d'une **Plage**, comme dans Excel :

- une colonne de tableau, par exemple une table de référence : `Clients[Ville]` (ou certaines de ses lignes, `Clients[Ville][1:5]`) ;
- des cellules : `Cell[A1:A10]`, ou `Feuille2!Cell[A1:A10]` sur une autre feuille (`A1:A10` tapé à la Excel est accepté) ;
- une cellule nommée.

**Sélectionner** retire le dialogue le temps de désigner la plage (clic, glissé ou `Maj+flèches` ; un clic sur l'en-tête d'une colonne de tableau la prend entière), au besoin sur une autre feuille : cliquez son onglet, la référence devient `Feuille2!Cell[A1:A10]`. `Entrée` la reprend, `Échap` revient sans la changer ; dans les deux cas, on retrouve la feuille et la sélection de départ. Le titre du dialogue dit ce que vise la validation (colonne d'un tableau, cellules), et le dialogue montre un aperçu des valeurs.

La liste d'une plage est relue à chaque ouverture de la liste déroulante et à chaque saisie : valeurs distinctes, dans l'ordre de la plage, sans les cellules vides. La référence suit le renommage du tableau, de la colonne ou de la feuille, et l'insertion ou la suppression de lignes et de colonnes. Si la source disparaît (tableau supprimé, lignes de la plage supprimées), la liste déroulante et le dialogue le signalent, et les saisies sont refusées jusqu'à ce que la validation soit corrigée.

### Colonnes de référence : basculer d'un tableau à l'autre

Quand la plage est la **colonne clé** d'un autre tableau (celle marquée 🔑, par exemple `Espèces[Nom]`), la colonne validée devient une **référence** vers ce tableau, comme une clé étrangère : la barre d'état le signale.

- **Basculer sur** : `Ctrl+clic` sur une cellule de la colonne, le petit bouton ↗ à côté de sa flèche de liste, ou clic droit › Basculer sur « Espèces » › Pie, va à la ligne citée (sur sa feuille), qui s'éclaire un instant. Si la valeur n'existe pas dans le tableau visé, l'entrée est grisée et le dit.
- **Revenir** : `Alt+Gauche`, ou « ← Revenir à… » dans la barre d'état, ramène à la cellule de départ (plusieurs sauts se défont un à un).
- **Lignes qui citent** : clic droit sur une cellule de la colonne clé du tableau visé › Lignes qui citent « Pie » (3) filtre le tableau qui cite sur cette valeur et va à sa première ligne.

Rien d'autre n'est enregistré : retirer la validation (ou `Ctrl+Z`) ou la clé du tableau visé défait la référence, et elle suit le renommage du tableau et de sa colonne clé. Les valeurs absentes ne sont pas bloquées par une contrainte de la base : c'est la validation qui les refuse à la saisie.

Les validations ne sont pas exportées vers Excel (.xlsx), ni lues à l'import.

## Notes

Révision › Nouvelle note (`Maj+F2`) attache une note à la cellule active : un coin rouge la signale, et elle s'affiche au survol. Sur une cellule qui en a une, la commande devient **Modifier la note**. L'épingle de la note la garde affichée. Révision › Supprimer la note la retire.

## Figer les volets

Affichage › Figer les volets garde des lignes et des colonnes à l'écran pendant le défilement :

- **Figer les volets** : les lignes au-dessus et les colonnes à gauche de la cellule active ;
- **Figer la ligne supérieure** ou **la première colonne** ;
- **Libérer les volets** les remet à défiler.

Le réglage appartient à la feuille et s'enregistre avec le classeur.

## Thème de couleur

Affichage › Thème choisit la couleur des tableaux du classeur : celle des tableaux ordinaires (vert par défaut) et celle des tableaux chargés par une source externe ou une requête (bleu par défaut). Chaque thème du menu montre ses deux couleurs ; l'en-tête, son texte, le cadre, les icônes et la pastille du nom en sont tirés, avec un texte toujours lisible sur l'en-tête.

- **Personnaliser…** : deux couleurs au choix, avec un aperçu immédiat dans la grille, puis les autres couleurs des palettes (Accent 3 à 6, Texte 2, Arrière-plan 2), tirées des deux premières si vous n'y touchez pas ; **Ajouter à mes thèmes** le garde dans la liste pour vos autres classeurs (il se retire depuis la même fenêtre).
- Chaque thème a ses **couleurs de palette** (voir Couleurs de police et de remplissage) : les cellules colorées avec une couleur du thème changent avec lui. Un classeur sans thème propre garde le thème par défaut dès qu'on y pose une couleur du thème, pour qu'elle reste la même chez un collègue.
- Le thème s'enregistre avec le classeur (`Ctrl+Z` l'annule) : un collègue l'ouvre avec les mêmes couleurs, même s'il n'a pas ce thème. Un classeur partagé sur un Serveur Lattis le publie avec sa mise en page.
- Un classeur qui n'a pas choisi de thème (classeur neuf, ou d'avant les thèmes) prend le **thème par défaut** de Fichier › Paramètres › Affichage.

Votre organisation peut proposer ses propres thèmes, ne proposer qu'eux, ou imposer le thème par défaut. Les couleurs du thème sont aussi exposées en variables CSS (`--lattis-table-head`, `--lattis-source-head`…).

## Protéger

Révision › Protéger la feuille refuse toute modification de son contenu ; on peut encore s'y déplacer, sélectionner, copier, rechercher, filtrer et imprimer. Révision › Protéger la structure verrouille tout sauf la saisie dans les tableaux : on tape, colle (valeurs seules), efface des valeurs dans leurs cellules de données, on ajoute des lignes en tapant dessous, on en insère ou supprime, on trie et filtre ; les cellules libres, la mise en forme, les colonnes et en-têtes, les tableaux eux-mêmes, les objets, les noms, les règles, les lignes et colonnes de la feuille, la mise en page et l'onglet sont refusés. Révision › Protéger le classeur refuse d'ajouter, supprimer, renommer, déplacer ou masquer des feuilles ; on peut choisir d'y verrouiller aussi la structure de toutes les feuilles (saisie dans les tableaux seulement), ou leur contenu. Les commandes refusées sont grisées, et leur infobulle dit comment ôter la protection (voir [Prise en main](prise-en-main#proteger-une-feuille-ou-le-classeur)).

Dans un classeur partagé sur un Serveur Lattis, un tableau publié que vos droits ne permettent pas de modifier (rôle Lecteur, classeur figé, hors ligne) grise de même les commandes qui changeraient ses données ; sa mise en forme, son tri et ses filtres restent permis, car ils ne valent que sur votre poste.

Dans un classeur collaboratif, le propriétaire et les concepteurs changent la **structure** d'un tableau partagé comme celle d'un tableau local : ajouter, renommer, supprimer une colonne, en changer le type, choisir la clé, renommer ou supprimer le tableau. Le changement est fait sur le serveur avant de s'appliquer sur le poste, et les autres le reçoivent aussitôt (leur tableau est relu ; l'historique d'annulation qui le touchait est effacé). Le serveur peut refuser, et son message s'affiche dans la barre d'état : une colonne citée par le filtre d'un rôle, une restriction ou l'audience d'une donnée ne se renomme, ne se supprime ni ne se convertit pas ; un tableau qui alimente le Data Lake ou un pipeline ne se supprime pas. `Ctrl+Z` défait le changement sur le serveur aussi (s'il le refuse, l'étape reste à annuler) ; un tableau supprimé puis rétabli ne revient que sur votre poste, comme tableau local. Les autres membres (éditeurs, participants) ne changent pas la structure, il faut être en ligne, et les index du serveur se posent avec les commandes `tables.add_index` et `tables.drop_index` (scripts, assistant).
