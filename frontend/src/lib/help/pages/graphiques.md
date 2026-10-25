# Graphiques

Un graphique se branche sur un tableau ou un tableau croisé, et se met à jour avec lui.

## Insérer un graphique

1. Cliquez dans le tableau (ou le TCD) à représenter.
2. Insertion › choisissez un type de graphique.
3. Le graphique se pose sur la feuille et le volet **Graphique** s'ouvre à droite.

Si plusieurs colonnes du tableau sont sélectionnées, elles fixent le graphique, comme dans Excel : la première colonne de texte donne les catégories, les colonnes de nombres les séries.

Le graphique se déplace à la souris et se redimensionne par ses poignées. Un clic dessus rouvre le volet ; clic droit : couper, copier, copier comme image (à coller dans un courriel ou un document), supprimer. Au clavier, une fois le graphique sélectionné : les flèches le déplacent, Maj+flèches le redimensionnent, Tab passe au graphique suivant, Suppr le supprime.

## Données tracées

Comme dans Excel, un graphique ne montre que les lignes affichées par le filtre du tableau ; la case **Lignes masquées par le filtre** du volet les reprend toutes.

Un graphique trace au plus 2 000 points : au-delà, il l'indique sous le dessin (« 2 000 premiers points sur 50 000 »). Une courbe trop longue est réduite plutôt que coupée : chaque intervalle garde son premier et son dernier point, son minimum et son maximum. Les boîtes à moustaches et les fréquences sont toujours calculées sur toutes les lignes.

Les valeurs reprennent le format de leur colonne (€, %, décimales, dates) sur les axes, les étiquettes et les info-bulles. Une colonne de dates en X donne un axe de temps pour les courbes et les aires, et se lit aussi dans un nuage de points.

## Types

Histogramme, barres horizontales, courbes, aires, secteurs, anneau, nuage de points, combiné (histogramme et courbes, axe secondaire possible), histogramme et secteurs 3D, cascade, compartimentage, radar, entonnoir, boîte à moustaches, histogramme de fréquences, jauge, et **carte** (points ou densité sur une carte OpenStreetMap, à partir de colonnes de latitude et de longitude).

## Le volet Graphique

- **Type** : change la forme sans perdre les données choisies ; « Empilé » pour les barres et les aires.
- **Données** : la source (un tableau ou un TCD), la colonne des catégories (axe horizontal) et une ou plusieurs séries de valeurs, dans l'ordre choisi par les flèches ▲ ▼ ; pour les courbes, les **cellules vides** coupent la ligne (intervalle), comptent pour zéro ou sont reliées.
- **Couleurs** : celle de chaque série, ou de chaque catégorie pour les secteurs, l'anneau, l'entonnoir et le compartimentage, et hausse, baisse et total pour la cascade.
- **Titres** : titre du graphique et des axes.
- **Éléments** : légende, étiquettes de données ; axe secondaire pour une série d'un graphique combiné.

## Cartes

Une carte lit soit deux colonnes (latitude, longitude), soit une seule colonne qui contient les deux. Les fonds de carte viennent d'OpenStreetMap : ils demandent une connexion, et l'accès au serveur de tuiles est soumis à votre accord. Cet accord ne dépend pas des scripts (il vaut aussi dans la version web) ; après un refus, le bouton **Afficher le fond de carte** de la carte le donne. La démo publique n'a pas de fond de carte : elle ne se connecte à rien, et la carte le dit.

Pour zoomer, sélectionnez la carte (un clic) : la molette zoome autour du pointeur, glisser déplace la carte, un double-clic rapproche (Maj+double-clic éloigne), et les touches **+** et **−** zooment aussi. Tant que la carte n'est pas sélectionnée, la molette fait défiler la feuille ou le document ; Alt+glisser déplace le graphique sélectionné. Les boutons **+**, **−** et **recentrer** apparaissent en haut à droite au survol. La vue choisie est enregistrée avec le classeur (Ctrl+Z la défait) ; **recentrer**, ici ou dans le volet du graphique, revient à la vue ajustée aux points.

**Couleur par** (volet du graphique, affichage en points) : choisissez une colonne, et chaque valeur distincte prend sa couleur, réparties sur tout le spectre ; une légende en bas à gauche les nomme (case **Légende des couleurs** pour la masquer). Les couleurs suivent l'ordre trié des valeurs, pas celui des lignes : trier ou filtrer le tableau ne les mélange pas. Au-delà de 20 valeurs, les 19 plus fréquentes gardent une couleur et les autres sont grises (« Autres ») ; les cellules vides sont gris clair. La densité reste une seule nappe : l'option n'y est pas proposée.

Le survol d'un point affiche son nom, sa couleur et sa valeur. Une fois la carte sélectionnée, un clic sur un point (sans glisser) sélectionne sa ligne dans le tableau, sur sa feuille, même depuis un document ; si plusieurs points se chevauchent, c'est le plus proche du pointeur. Le premier clic sur une carte non sélectionnée la sélectionne seulement.

## Copier un graphique

Un graphique copié se colle sur une autre feuille ou dans un [document](onglets), où il reste branché sur ses données.

## Images, formes et zones de texte

Insertion › **Image** (ou Ctrl+V d'une image copiée), **Formes** ou **Zone de texte** : l'objet se pose à la cellule active et suit ses lignes et colonnes. Glissez-le pour le déplacer, tirez ses poignées pour le redimensionner (une image garde ses proportions ; Maj pour les libérer), double-cliquez une forme pour y taper du texte. La barre au-dessus de l'objet règle les couleurs, le trait, la transparence et le texte ; Suppr supprime, Ctrl+D duplique. Une image très grande est réduite à l'insertion ; 10 Mo au plus (1 Mo dans un classeur partagé).

## Tendance et erreurs

Volet du graphique › **Analyse** (barres, courbes, aires, nuages, bulles) : choisissez une série, puis une **tendance** (linéaire, exponentielle, logarithmique, polynomiale, puissance, moyenne mobile), avec sa prévision avant ou après les données, son équation et son R². **Barres d'erreur** : valeur fixe, pourcentage, écart type, erreur type, ou montants d'une colonne. Les types **Bulles** (X, Y, taille) et **Boursier** (ouverture, haut, bas, clôture, volume) sont dans Autres graphiques.

## Sparklines

Insertion › **Sparklines** : un petit graphique dans une cellule (courbe, histogramme, positif/négatif). Sélectionnez les colonnes d'une ligne d'un tableau (Jan à Déc) : chaque ligne du tableau reçoit le sien, lignes ajoutées comprises. Hors tableau, donnez la plage des données et l'emplacement. Le même menu, sur une cellule qui en a un, règle les points mis en valeur, les couleurs et l'axe, ou le supprime.
