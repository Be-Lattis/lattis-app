# Mise en forme, notes et affichage

Les commandes du ruban suivent celles d'Excel, aux mêmes places. Elles s'appliquent à la sélection (ou à la cellule active) et s'annulent par `Ctrl+Z`.

## Mise en forme des cellules

Accueil › Police, Alignement et Nombre mettent en forme la sélection ; le petit lanceur dans le coin de chaque groupe, ou `Ctrl+1`, ouvre la boîte **Format de cellule** (nombre, alignement, police, bordure, remplissage). Cliquer sur l'alignement déjà choisi (Gauche, Centrer, Droite) revient à l'alignement standard, comme dans Excel.

Accueil › Édition › Effacer retire les formats, le contenu (`Suppr`) ou la note de la sélection. Remplissage recopie la première ligne vers le bas (`Ctrl+D`) ou la première colonne vers la droite (`Ctrl+R`).

Accueil › Mettre sous forme de tableau (`Ctrl+L`) fait un tableau du bloc de cellules libres de la cellule active, sa première ligne donnant les en-têtes.

## Mise en forme conditionnelle

Accueil › Mise en forme conditionnelle colore les cellules selon leur valeur : mise en évidence (supérieur à, entre, texte qui contient, doublons…), barres de données, nuances de couleurs et jeux d'icônes. Le **gestionnaire des règles** liste les règles de la feuille, pour les modifier ou les supprimer. Dans un tableau, une règle vaut pour toute la colonne, lignes à venir comprises.

## Validation des données

Données › Validation des données donne la liste des valeurs permises dans la sélection, ou dans toute une colonne de tableau : la cellule affiche une flèche de liste déroulante (`Alt+Bas` l'ouvre), et une autre saisie est refusée.

La liste se saisit (**Valeurs saisies**, une par ligne) ou se tire d'une **Plage**, comme dans Excel :

- une colonne de tableau, par exemple une table de référence : `Clients[Ville]` (ou certaines de ses lignes, `Clients[Ville][1:5]`) ;
- des cellules : `Cell[A1:A10]`, ou `Feuille2!Cell[A1:A10]` sur une autre feuille (`A1:A10` tapé à la Excel est accepté) ;
- une cellule nommée.

**Sélectionner** retire le dialogue le temps de désigner la plage dans la feuille (clic, glissé ou `Maj+flèches`), `Entrée` la reprend, `Échap` revient sans la changer. Le dialogue montre un aperçu des valeurs.

La liste d'une plage est relue à chaque ouverture de la liste déroulante et à chaque saisie : valeurs distinctes, dans l'ordre de la plage, sans les cellules vides. La référence suit le renommage du tableau, de la colonne ou de la feuille, et l'insertion ou la suppression de lignes et de colonnes. Si la source disparaît (tableau supprimé, lignes de la plage supprimées), la liste déroulante et le dialogue le signalent, et les saisies sont refusées jusqu'à ce que la validation soit corrigée.

Les validations ne sont pas exportées vers Excel (.xlsx), ni lues à l'import.

## Notes

Révision › Nouvelle note (`Maj+F2`) attache une note à la cellule active : un coin rouge la signale, et elle s'affiche au survol. Sur une cellule qui en a une, la commande devient **Modifier la note**. L'épingle de la note la garde affichée. Révision › Supprimer la note la retire.

## Figer les volets

Affichage › Figer les volets garde des lignes et des colonnes à l'écran pendant le défilement :

- **Figer les volets** : les lignes au-dessus et les colonnes à gauche de la cellule active ;
- **Figer la ligne supérieure** ou **la première colonne** ;
- **Libérer les volets** les remet à défiler.

Le réglage appartient à la feuille et s'enregistre avec le classeur.

## Protéger

Révision › Protéger la feuille refuse toute modification de son contenu ; on peut encore s'y déplacer, sélectionner, copier, rechercher, filtrer et imprimer. Révision › Protéger le classeur refuse d'ajouter, supprimer, renommer ou déplacer des feuilles. Les commandes refusées sont grisées, et leur infobulle dit comment ôter la protection (voir [Prise en main](prise-en-main#proteger-une-feuille-ou-le-classeur)).

Dans un classeur partagé sur un Serveur Lattis, un tableau publié que vos droits ne permettent pas de modifier (rôle Lecteur, classeur figé, hors ligne) grise de même les commandes qui changeraient ses données ; sa mise en forme, son tri et ses filtres restent permis, car ils ne valent que sur votre poste.

Dans un classeur collaboratif, le propriétaire et les concepteurs changent la **structure** d'un tableau partagé comme celle d'un tableau local : ajouter, renommer, supprimer une colonne, en changer le type, choisir la clé, renommer ou supprimer le tableau. Le changement est fait sur le serveur avant de s'appliquer sur le poste, et les autres le reçoivent aussitôt (leur tableau est relu ; l'historique d'annulation qui le touchait est effacé). Le serveur peut refuser, et son message s'affiche dans la barre d'état : une colonne citée par le filtre d'un rôle, une restriction ou l'audience d'une donnée ne se renomme, ne se supprime ni ne se convertit pas ; un tableau qui alimente le Data Lake ou un pipeline ne se supprime pas. `Ctrl+Z` défait le changement sur le serveur aussi (s'il le refuse, l'étape reste à annuler) ; un tableau supprimé puis rétabli ne revient que sur votre poste, comme tableau local. Les autres membres (éditeurs, participants) ne changent pas la structure, il faut être en ligne, et les index du serveur se posent avec les commandes `tables.add_index` et `tables.drop_index` (scripts, assistant).
