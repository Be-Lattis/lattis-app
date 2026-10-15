# Import et export (CSV, .xlsx)

*16/10/2026. Code : `internal/transfer`, entrée du modèle `canvas.Model.Import` / `ExportTable` (`internal/canvas/import.go`), menu Fichier.*

| Sujet | Décision | Statut |
|---|---|---|
| Où arrivent les données | Chaque fichier ajoute des feuilles après les existantes ; un CSV donne une feuille nommée d'après le fichier, avec un tableau en haut à gauche (il prend donc le nom de la feuille) | Proposé |
| Annulation | Tout l'import est **une** étape d'annulation. Les lignes ne passent pas par le journal : elles sont chargées dans une table `_slitable_trash_import_*` que l'opération renomme en `t_<id>` (l'annulation la renomme en sens inverse). Une table de chargement qu'aucune commande ne cite plus est supprimée au prochain import | Proposé |
| Types | Inférés sur **toutes** les valeurs avant la création (aucun élargissement après coup). Un nombre avec un zéro en tête (`01234`) ou de plus de 15 chiffres reste du texte | Proposé |
| CSV lu | Encodage détecté (BOM UTF-8 / UTF-16, sinon UTF-8, sinon Windows-1252), séparateur détecté parmi `; , tabulation \|`, guillemets et sauts de ligne dans les champs | Proposé |
| CSV écrit | Par défaut « Excel français » : `;`, virgule décimale, dates jj/mm/aaaa, VRAI/FAUX, UTF-8 avec BOM. Style `Standard` (RFC 4180) disponible côté Go. Écriture dans un fichier temporaire puis renommage | Proposé |
| .xlsx : tableaux | Une feuille est découpée en îlots (ligne ou colonne vide = fin du tableau, comme à la saisie). Un îlot de 2 lignes ou plus devient un tableau à la même place ; un îlot d'une ligne reste en cellules libres | Proposé |
| .xlsx : valeurs | Chaînes partagées, texte enrichi, booléens, dates reconnues à leur format (calendriers 1900 et 1904). Une date avec heure ou une heure seule devient du texte (pas encore de type heure). Une erreur Excel (`#N/A`) dans un tableau est importée vide, avec un avertissement | Proposé |
| .xlsx : formules | Une colonne dont toutes les cellules portent la **même formule recopiée** devient une colonne calculée : `C2*D2` → `[@Qté]*[@Prix]`, une plage qui couvre une colonne de tableau → `Tableau[Colonne]`, `RECHERCHEV(B2;Produits!$A$2:$B$10;2;FAUX)` → `REFERENCE(Produits; "Prix unitaire"; [@Produit])`. Sinon (formules différentes d'une ligne à l'autre, référence à une cellule isolée, fonction absente du catalogue, nom défini, plage relative qui glisse…), les valeurs calculées par Excel sont gardées et un avertissement l'explique | Proposé |

## Mesures (conteneur Linux, pas le poste de Louis)

| | Temps |
|---|---|
| CSV 1 M de lignes × 10 colonnes (70 Mo) | 10 s |
| .xlsx 100 000 lignes × 10 colonnes | 2,6 s |

La lecture des feuilles .xlsx passe par un lecteur XML écrit pour ce format (`xmlscan.go`) : `encoding/xml` prenait 80 % du temps. Chaque feuille est lue trois fois (îlots ; en-têtes, types et formules ; valeurs), plutôt que gardée en mémoire.

## Pas encore fait

- Ancien format `.xls`, et `.ods` : refusés avec un message.
- Tableaux Excel nommés (`Tableau1[@Prix]` dans les formules) : formule gardée en valeurs.
- Export `.xlsx`.
- Choisir où arrive un CSV (cellule active plutôt que nouvelle feuille) et le séparateur à la main dans l'interface (l'option existe côté Go).
