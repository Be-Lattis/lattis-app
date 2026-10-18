# Détection des tableaux : titres, notes et totaux

*18/10/2026. Suite de l'audit de la détection des tableaux (dossier du projet, `revue/audit-detection-tableaux.md`). Code : `internal/canvas/legend.go`, `internal/canvas/pasteislands.go`, `birth` et `input` (`internal/canvas/actions.go`), `joinStep` (`adjacent.go`), `ensureGapBelow`/`ensureGapRight` (`layout.go`), `hasHeader` (`paste.go`) ; menus dans `frontend/src/lib/grid/Sheet.svelte`.*

## En bref

Ce qui accompagne un tableau sans en faire partie reste une cellule libre : la **légende du tableau**.

| Cas | Décision | Statut |
|---|---|---|
| Titre au-dessus | Une ou plusieurs lignes d'une seule cellule (ou clairsemées : 2025 / 2026 au-dessus de T1 T2 T1 T2), au-dessus d'une ligne d'en-têtes, restent libres, quel que soit l'ordre de saisie (`titleRows` à la naissance ; `reshapeYoung` quand un tableau tout juste né a pris le titre pour en-tête). Titre tapé après : déjà libre (exception de `touching`) | Fait |
| Note dessous | Une saisie seule sur la ligne juste sous le tableau qui commence comme une note (`*`, `(1)`, Source, Note, NB, Remarque, Attention, flèche), un texte sous une colonne typée d'un tableau de plusieurs colonnes (la colonne ne passe plus en texte), ou une phrase bien plus longue que sa colonne : note, en gris italique. Une saisie sous une note est aussi une note | Fait |
| Total dessous | « Total », « Sous-total », « Somme », « Moyenne » en tête, ou **toute formule** tapée juste sous le tableau : total, en gras. Une formule y cite le tableau entier (`SOMME([Montant])` devient `SOMME(Ventes[Montant])`) au lieu de devenir une colonne calculée circulaire | Fait |
| Remarque à côté | Une saisie à droite ou à gauche d'une ligne de données qui commence comme une note : remarque, pas une colonne. Sur la ligne d'en-tête, une saisie reste un nouveau nom de colonne | Fait |
| Marque | Propriété `out` du style de la cellule libre (`_lattis_cell_styles`) : pas de migration, la marque suit la cellule dans ses déplacements. Une cellule marquée ne rejoint pas le tableau (`joinStep`), ne fait pas naître de tableau, et reste **collée** au tableau quand il grandit (`ensureGap*` ne lui demande pas de ligne vide). Effacer la cellule retire la marque | Fait |
| Collage | Dans une zone vide, le bloc est découpé aux lignes et colonnes vides (`blockIslands`), comme à l'import xlsx. Dans chaque îlot, titres du haut et notes ou totaux du bas restent libres. Un îlot sans en-tête juste sous un tableau du même bloc, de mêmes colonnes, le continue (ligne vide au milieu des données). En-têtes « années » (Pays · 2024 · 2025) reconnus comme à la saisie | Fait |
| Commandes | Clic droit sur l'en-tête : « Sortir l'en-tête du tableau (c'est un titre) » (`HeaderToTitle`, inverse d'« Utiliser la 1re ligne comme en-têtes »). Sur la dernière ligne : « Sortir la ligne du tableau (note) » (`RowToNote`). Sur une cellule marquée : « Ajouter au tableau » (`JoinTable`). Chaque décision automatique le dit dans la barre d'état | Fait |
| Pas fait | Ligne d'unités sous l'en-tête (« (kg) »), fiche clé/valeur prise pour un tableau, vraie « Ligne de total » d'Excel (agrégats hors données SQL) ; un titre suivi d'une seule colonne tapée dessous reste l'en-tête tant qu'aucune 2e colonne n'apparaît (commande « Sortir l'en-tête ») | À faire |
