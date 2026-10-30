# Audit des formules : parité avec un tableur (30/10/2026)

*Suite de l'audit général du 30/10 (§ 1, « Parité formules et fichiers »). Périmètre : `internal/formula` et le catalogue des fonctions. L'aller-retour des fichiers .xlsx est traité à part (autre audit, `internal/transfer`).*

## Mesures

| | Avant | Après |
|---|---|---|
| Fonctions du catalogue | 274 (dont RECHERCHEH refusée) | 294, aucune refusée |
| Formules vérifiées sur les deux moteurs (`testdata/conformite.txt`) | 857 | 941, plus `TestArrayShape` (27 déversements) et `TestConvertLookupModes` (14 conversions) |
| Recherches sur des cellules libres (`RECHERCHEV(x; D2:F20; 2; FAUX)`) | refusées | converties en `INDEX(Cell[…]; EQUIV(…))` |

Méthode : liste des fonctions d'un tableur récent comparée au catalogue (`funcs.go`), lecture des chemins de conversion (`convert.go`), des tableaux dynamiques (`spill.go`) et de SOUS.TOTAL dans les deux moteurs, essais dans la grille (`internal/canvas`).

## Constats

Classés par fréquence d'usage probable. **[corrigé]** : fait, avec le commit ; **[arbitrage]** : décision de Louis ; **[plus tard]** : trop gros pour cette passe.

### Recherches

1. **[corrigé]** RECHERCHEV approchée (4e argument omis ou VRAI, la forme par défaut) était refusée. Elle devient `INDEX(T[Valeur]; EQUIV(x; T[Cherchée]; 1))`, sans clé à poser. *d35f3bbc*
2. **[corrigé]** RECHERCHEV, RECHERCHEX et RECHERCHEH sur une plage de cellules libres étaient refusées (« la plage ne correspond à aucun tableau »), alors que c'est l'usage le plus courant hors tableau. Elles deviennent INDEX et EQUIV (ou EQUIVX) sur des `Cell[…]`. RECHERCHEH n'est plus refusée que sur un tableau. *d35f3bbc*
3. **[corrigé]** EQUIVX manquait ; RECHERCHEX refusait ses modes (-1, 1, 2 et la recherche depuis la fin). EQUIVX est un parcours (`scan.go`), commun aux deux moteurs ; RECHERCHEX avec un mode devient `INDEX(…; EQUIVX(…))`. Les recherches dichotomiques (2, -2) parcourent tout : même résultat sur des données triées, résultat juste sur des données qui ne le sont pas (écart voulu). *d35f3bbc*
4. **[arbitrage]** RECHERCHEH sur un tableau reste refusée (les données d'un tableau sont en colonnes). Options : (a) garder le refus, message raccourci (fait) ; (b) la lire comme RECHERCHEV transposée. Recommandation : (a), la forme (b) n'a pas d'usage réel.
5. **[arbitrage]** Les recherches tapées sont réécrites à la saisie (REFERENCE, INDEX/EQUIV) : l'utilisateur ne retrouve pas sa formule. Options : (a) statu quo, l'infobulle « convertie depuis » le dit ; (b) garder le texte d'origine affiché et ne réécrire que le calcul (`Call.Origin` existe déjà) ; environ 1 jour, touche la barre de formule. Recommandation : (b) si des utilisateurs s'en plaignent.

### Tableaux dynamiques

6. **[corrigé]** ASSEMB.V, ASSEMB.H, CHOISIRCOLS, CHOISIRLIGNES, VERS.COL, VERS.LIGNE manquaient ; PRENDRE et EXCLURE refusaient leur 3e argument. Faits sur les deux moteurs (`funcs_arrays.go`). Le moteur SQL fixe la forme à la compilation : numéros de colonnes et modes écrits dans la formule, VERS.LIGNE sur une plage ou une constante. ASSEMB.V accepte plusieurs tableaux nommés, suivis par les dépendances, les renommages et la copie de feuille. *7a9d995e*
7. **[corrigé]** Bogue : une plage A1 tapée en source d'un tableau dynamique (`=UNIQUE(A1:B20)`, `=TRIER(A2:A9)`) restait une plage A1 et donnait #VALEUR! (`ConvertA1` sautait le 1er argument, pris pour un nom de tableau). *7a9d995e*
8. **[arbitrage]** Famille LAMBDA (LAMBDA, MAP, BYROW, BYCOL, REDUCE, SCAN, MAKEARRAY), jusqu'ici « remplacée par Lua ». Elle arrive par les classeurs importés (valeur gardée, formule #NOM?). Options : (a) statu quo ; (b) LAMBDA en ligne dans MAP, BYROW, BYCOL et MAKEARRAY, développée comme LET (substitution, même code pour les deux moteurs ; BYROW/BYCOL rendent une valeur par ligne, donc une forme connue) : 3 à 5 jours ; (c) en plus REDUCE et SCAN (calcul séquentiel : CTE récursive en SQL ou évaluation Go à part) et les LAMBDA nommées dans le gestionnaire de noms : 4 jours de plus. Recommandation : (b), qui couvre la plupart des formules importées.
9. **[arbitrage]** CHOISIRLIGNES avec un numéro hors du tableau rend une ligne de #VALEUR! (le tableur rend #VALEUR! entier). Le moteur SQL ne sait pas annuler tout le résultat après coup sans recompter la source. Options : (a) garder (documenté) ; (b) compter la source à part et rendre une seule erreur : environ 2 heures, une requête de plus. Recommandation : (a).
10. **[plus tard]** TABLEAU.ALEA (RANDARRAY), ORGA.LIGNES / ORGA.COLS (WRAPROWS, WRAPCOLS), DEVELOPPER (EXPAND) : forme connue seulement si les tailles sont écrites dans la formule, comme SEQUENCE. Environ 1 jour.
11. **[plus tard]** TRIER et UNIQUE avec `par_colonne` VRAI ; REGEXEXTRACT modes 1 et 2 (tableaux) ; INDEX à 3 arguments sur une plage. Environ 1 jour en tout.

### Texte

12. **[corrigé]** REGEXTEST, REGEXEXTRACT, REGEXREPLACE manquaient (même nom dans toutes les langues, comme dans le tableur). Motifs compilés gardés en cache (256). *accedd57*
13. **[arbitrage]** Syntaxe des expressions régulières : le moteur Go (RE2) n'a ni assertions avant / arrière ni références arrière (`\1`) ; elles donnent #VALEUR!. Options : (a) garder RE2, en temps linéaire : aucun motif ne peut bloquer un classeur partagé ; (b) un moteur à retour arrière (bibliothèque tierce) avec délai limite par appel : compatibilité complète, risque de lenteur sur des motifs piégés. Recommandation : (a).

### Agrégats

14. **[corrigé]** « SOUS.TOTAL ignore le filtre de la grille » (`docs/fonctions.md`, ligne 94) était faux : le moteur SQL, qui calcule la grille, ne lit que `Table.Visible` (essai `TestSubtotalFilter`). Ligne corrigée. *d35f3bbc*
15. **[plus tard]** SOUS.TOTAL 101 à 111 sur une plage de cellules compte encore les lignes masquées par le plan ; il compte aussi les SOUS.TOTAL imbriqués d'une plage. Il faut passer les lignes masquées de la feuille au moteur. Environ 1 jour.

### Fonctions courantes absentes

16. **[corrigé]** PLAFOND.MATH, PLANCHER.MATH, PAIR, IMPAIR, FACT, COMBIN, PERMUTATION, ATAN2, ESTERR, JOURS360. *57af385a*
17. **[plus tard]** Encore absentes, par fréquence probable : PGCD, PPCM, SOMME.CARRES, N, T, TYPE, TYPE.ERREUR, ESTNONTEXTE, ESTFORMULE, VALEURNOMBRE, CTXT, ROMAIN, CHIFFRE.ARABE, BASE, DECIMAL, conversions binaire / hexadécimal, CONVERT, SINH et autres hyperboliques, MATRICE.INVERSE, PRODUITMAT, DETERMAT, FREQUENCE, MODE.MULTIPLE, RANG.POURCENTAGE, CUMUL.INTER, CUMUL.PRINCPER, TRI.MODIFIE, TAUX.INT.EQUIV, PDUREE, ADRESSE, FEUILLE, RECHERCHE (forme vectorielle), PREVISION.ETS. Les scalaires prennent 2 à 3 jours avec l'aide en 5 langues ; les fonctions de matrices et FREQUENCE, 1 à 2 jours de plus.

### Fichiers

18. **[corrigé]** Export .xlsx / .ods des nouvelles fonctions : `exportFuncs` les écrit avec leur préfixe (`_xlfn.XMATCH`, `_xlfn.REGEXTEST`, `_xlfn.VSTACK`, `_xlfn._xlws.FILTER`, `_xlfn.CEILING.MATH`…) ; en .ods, seules les fonctions mathématiques et d'information (LibreOffice n'a pas les autres partout). Un tableau dynamique qui se déverse reste exporté en valeurs ; ces noms servent quand il est imbriqué (`SOMME(FILTRE(…))`). Chaque fonction du catalogue est traduite ou dans la liste `notExported` (essai `TestExportCatalogue`) ; aller-retour .xlsx du texte et de la valeur (`TestExportNewFunctionsRoundTrip`).

## Ce qui a été vérifié

- Chaque fonction ajoutée : exemples chiffrés des définitions du tableur, sur l'évaluateur Go et sur le SQL compilé (`TestConformite`, `TestArity` pour chaque nombre d'arguments documenté).
- Conversions des recherches : texte de la formule réécrite et valeur, dans les deux moteurs (`TestConvertLookupModes`) et dans la grille (`TestLookupFreeCells`, `TestStackSpills`).
- Aide : description, arguments et exemple en 5 langues (`TestFunctionDocs`, `TestDocsTranslated`) ; noms allemand et espagnol uniques (`TestFunctionNames`).
