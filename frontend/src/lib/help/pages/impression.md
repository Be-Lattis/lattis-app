# Mise en page et impression

Fichier › Imprimer / PDF (`Ctrl+P`) imprime la feuille active. Pour un PDF, choisissez l'imprimante « Microsoft Print to PDF » (ou « Enregistrer au format PDF ») dans la fenêtre d'impression. Ce qui s'imprime se règle dans le groupe Mise en page de l'onglet **Affichage** du ruban (`Alt+W`). Les réglages appartiennent à la feuille : ils s'enregistrent avec le classeur, s'annulent par `Ctrl+Z` et suivent une copie de la feuille.

## Groupe Mise en page (onglet Affichage)

| Commande | Effet |
| --- | --- |
| Marges | Normales, Larges ou Étroites ; « Marges personnalisées… » ouvre la boîte Mise en page |
| Orientation | Portrait ou Paysage |
| Taille | Format du papier : A4, A3, A5, B5, Lettre US, Légal US, Tabloïd, Exécutive |
| Zone d'impression | « Définir » : seule la sélection s'imprime. « Annuler » : toute la plage utilisée s'imprime de nouveau |
| Sauts de page | « Insérer » : une nouvelle page commence au-dessus de la cellule active et à sa gauche (en ligne 1, seulement à gauche). « Supprimer » retire ces sauts. « Rétablir tous » retire tous les sauts manuels |
| Imprimer les titres | Lignes à répéter en haut et colonnes à répéter à gauche de chaque page |

Insérer ou supprimer des lignes et des colonnes déplace la zone, les titres et les sauts avec elles.

## Mise à l'échelle

**Largeur** et **Hauteur** ajustent la feuille à un nombre de pages : Lattis la réduit (jusqu'à 10 %) pour qu'elle tienne. « 1 page » en largeur et « Automatique » en hauteur donne toutes les colonnes sur la largeur d'une page. Pendant un ajustement, les sauts de page manuels sont ignorés, comme dans Excel. **Échelle** imprime à un pourcentage fixe et arrête l'ajustement.

## Options de la feuille

« Afficher le quadrillage » et « Afficher les en-têtes » ne changent que l'écran. « Imprimer le quadrillage » trace toutes les cellules. « Imprimer les en-têtes » ajoute les lettres de colonnes et les numéros de lignes. Sans quadrillage, seules les cellules des tableaux sont encadrées.

## La boîte Mise en page

Le petit bouton en bas à droite de chaque groupe ouvre la boîte **Mise en page**. Elle a quatre onglets :

- **Page** : orientation, échelle ou ajustement en pages, format du papier, numéro de la première page.
- **Marges** : haut, bas, gauche, droite, en-tête et pied de page (en cm), centrage horizontal et vertical sur la page.
- **En-tête/Pied de page** : un modèle (« Page 1 sur ? », nom de la feuille, du fichier…), ou trois zones libres (gauche, centre, droite). Les boutons insèrent des codes : `&[Page]`, `&[Pages]`, `&[Date]`, `&[Heure]`, `&[Fichier]`, `&[Chemin]`, `&[Onglet]`. Écrivez `&&` pour une perluète seule.
- **Feuille** : zone d'impression (`$A$1:$F$40`), lignes à répéter en haut (`$1:$1`) et colonnes à répéter à gauche (`$A:$A`). Le bouton **Sélection** reprend la sélection de la feuille. On y règle aussi le quadrillage, les en-têtes, le noir et blanc et l'ordre des pages (vers le bas puis à droite, ou l'inverse).

**Imprimer…** applique les réglages puis imprime.

## Sauts de page dans la grille

Après une impression ou un changement de mise en page, des pointillés montrent où les pages se coupent. Les sauts manuels sont en tirets plus marqués, et une zone d'impression est encadrée. Affichage › **Sauts de page** les montre ou les masque.

## Ce qui s'imprime

La feuille telle que la grille l'affiche : valeurs et formats de nombre, couleurs, polices, bordures, alignements, renvoi à la ligne, mise en forme conditionnelle (couleurs, barres, icônes), cellules fusionnées, tableaux croisés et graphiques, avec les largeurs de colonnes et les hauteurs de lignes. Les lignes et colonnes masquées ne s'impriment pas. Lattis découpe lui-même les pages : chacune a la taille du papier choisi, avec ses marges, son en-tête et son pied de page. Une grande feuille (100 000 lignes et plus) se prépare page par page, avec l'avancement dans la barre d'état.

Limites :

- Des milliers de pages : la fenêtre d'impression peut mettre du temps à les afficher. Une zone d'impression ou un filtre en réduit le nombre.
- Dans la fenêtre d'impression, gardez les marges « Par défaut » et l'échelle à 100 %. Sinon les pages préparées par Lattis se décalent.
- Si la fenêtre propose l'option « En-têtes et pieds de page », laissez-la décochée : elle ajoute ceux du navigateur (date, adresse).

## Excel et OpenDocument

La mise en page part avec le classeur exporté en `.xlsx` ou `.ods` : papier, orientation, marges, échelle ou ajustement, zone d'impression, titres, sauts de page, quadrillage, en-têtes, centrage, en-tête et pied de page. À l'import d'un `.xlsx`, Lattis reprend la mise en page de chaque feuille.

## Scripts

`sheets.page_setup()` lit la mise en page de la feuille et `sheets.set_page_setup({...})` la change (voir [la référence Lua](lua:sheets.set_page_setup)).
