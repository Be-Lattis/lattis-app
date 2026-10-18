# Identité visuelle : Lattis et Be Lattis

Décision de Louis, 18/10/2026 : l'application Lattis prend l'icône « Grille en L », l'entreprise Be Lattis le logo « Losange de treillis ». Les deux sont verrouillés : toute variante passe par une nouvelle décision.

## Fichiers

| Fichier | Usage |
|---|---|
| `lattis-icone.svg` | Icône de l'application, source unique de toutes les tailles |
| `be-lattis-logo-clair.svg` | Logo de l'entreprise sur fond clair |
| `be-lattis-logo-sombre.svg` | Logo de l'entreprise sur fond sombre |
| `be-lattis-symbole.svg` | Symbole seul (avatar, favicon, petits formats) |

Dérivés de l'icône, à régénérer après toute retouche :

- `build/appicon.png` : rendu 1024 × 1024 de `lattis-icone.svg` (fond transparent) ;
- `build/windows/icon.ico` et `build/darwin/icons.icns` : `wails3 generate icons -input appicon.png -macfilename darwin/icons.icns -windowsfilename windows/icon.ico`, lancé depuis `build/` ;
- les logos MSIX (`build/windows/msix/build-msix.ps1`) et Linux (`.desktop`, nfpm) sont tirés de `build/appicon.png` au moment de la construction ;
- `build/appicon.icon/Assets/wails_icon_vector.svg` : copie de `lattis-icone.svg` (Icon Composer, macOS) ;
- l'icône `logo` de `frontend/src/lib/icons/icons.ts` : la même grille, sans le carré vert, pour la barre de titre (déjà verte).

## Couleurs

| Nom | Valeur | Rôle |
|---|---|---|
| Vert Lattis | `#217346` | Carré de l'icône, symbole et texte du logo sur fond clair ; c'est aussi le vert de l'interface (ruban, onglets) |
| Vert profond | `#144d2e` | Texte sur fond menthe, ombres |
| Menthe | `#b9ecca` | Symbole sur fond sombre |
| Encre | `#17201b` | Nom « be lattis » sur fond clair |
| Blanc | `#ffffff` | Cellules de l'icône ; nom sur fond sombre |

## Icône de l'application : la grille en L

Une feuille de 3 × 3 cellules sur un carré vert aux coins arrondis. Les cinq cellules pleines (la colonne de gauche et la ligne du bas) dessinent le L de Lattis ; les quatre autres ne sont qu'un contour blanc à 45 % d'opacité.

Construction, sur une grille de 64 :

- carré : 60 × 60 à (2, 2), rayon 14 ;
- colonnes de cellules à x = 12, 28 et 44, lignes à y = 12, 28 et 44 ; cellules de 12 de côté, la dernière colonne et la dernière ligne réduites à 8 pour que le L reste dans la marge ;
- rayon des cellules : 2 ; contour des cellules vides : 2 d'épaisseur.

Règles :

- l'icône se lit jusqu'à 16 px : ne pas ajouter de détail (texte, dégradé, ombre) ;
- elle reste sur son carré vert, sauf dans la barre de titre de l'application, déjà verte, où seule la grille est dessinée ;
- ne pas la tourner, ni changer l'ordre des cellules pleines.

## Logo de l'entreprise : le losange de treillis

Un symbole et un nom :

- **symbole** : un losange tracé (trait de 5 sur une grille de 60, extrémités et angles arrondis), ses deux diagonales à 55 % d'opacité, et un nœud plein de rayon 5 au centre, là où les lattes se croisent ;
- **nom** : « be lattis » en minuscules, Sora SemiBold (600), à droite du symbole ; la hauteur des capitales de Sora égale environ la moitié du symbole.

Sur fond clair : symbole en vert Lattis, nom en encre. Sur fond sombre : symbole en menthe, nom en blanc.

Règles :

- zone de protection : la largeur du nœud (le cercle central) tout autour du logo ;
- taille minimale : 24 px de haut pour le logo complet ; en dessous, le symbole seul ;
- le nom s'écrit toujours en minuscules dans le logo ; en texte courant, on écrit « Be Lattis » ;
- ne pas recolorer le symbole autrement qu'en vert Lattis, menthe, blanc ou noir (impression monochrome).

## À faire

- Vectoriser le texte des fichiers `be-lattis-logo-*.svg` (aujourd'hui en police Sora) avant de les envoyer à un imprimeur ou un tiers qui n'a pas la police.
- Remplacer l'icône du fichier DMG (`build/darwin/dmg-file-icon.*`), encore celle de Wails.
