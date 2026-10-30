# Audit d'ergonomie et d'accessibilité — 30/10/2026

Suite du § 2 de l'audit général (`audit-general-2026-10-30.md`, note 3,5/5) :
clavier seul et lecteur d'écran, textes longs, erreurs non traduites,
cohérence ruban / menus / raccourcis. Mesures sur la version web
(`cmd/lattis-engine`, Edge sans tête, Playwright), après le découpage de
`Sheet.svelte` et de `+page.svelte`.

## Synthèse

| Mesure | Avant | Après |
|---|---|---|
| `svelte-ignore a11y_*` | 105, sans raison | 49, chacun avec sa raison ; 0 avertissement de `svelte-check` |
| Panneaux flottants sans rôle ni nom (filtre, ▾ de TCD, fonctions, doublons, note, lignes, taille) | 7 | 0 (`role="dialog"` nommé) |
| Erreurs de l'assistant IA en français dans toutes les langues | 33 textes | 0 (catalogue `internal/msg`, 5 langues) |
| Textes de plus de 120 caractères affichés (interface et moteur, branche « other » des pluriels, paramètre = 8 car.) | 134 | 119 ; au-delà de 140 : 78 → 23 |
| Noms de touches en français dans les autres langues (« Maj », « Suppr », « Pg.Suiv ») | partout | traduits (ruban, menus, aide) |
| Parcours au clavier seul (Playwright) | 0 | 6 (`frontend/e2e/keyboard.spec.ts`), 21 parcours en tout, ≈ 17 s |

Commits : 77684343 (erreurs traduites), 89dd8724 (textes raccourcis),
b512265b (accessibilité, raccourcis, parcours clavier).

## 1. Clavier seul et lecteur d'écran : les 10 tâches courantes

Parcours joués sans souris (`npm run e2e`, `keyboard.spec.ts`) ; le lecteur
d'écran est simulé par l'arbre d'accessibilité (rôles et noms que lit NVDA).

| Tâche | Au clavier | Constat |
|---|---|---|
| Saisir un tableau | oui (K1) | Le focus est sur la grille dès le chargement ; la cellule cachée de `GridA11y` annonce « B2, Prix, 3 ». |
| Mettre en forme | oui (K1) | Ctrl+B, Ctrl+1 ; la boîte rend le focus à la grille (Échap). |
| Trier, filtrer | oui (K5) | Alt, A, F ouvre le filtre de la colonne : **il n'avait ni rôle ni nom** (corrigé) ; cases cochées à l'Espace. |
| Formule | oui (parcours 3) | Complétion reprise par Tab. |
| TCD | oui | Alt, N, V ; la case de chaque champ reste l'élément du clavier. |
| Graphique | oui | Alt, N, H ; flèches et Maj+flèches le déplacent et le redimensionnent. |
| Requête SQL | oui | Alt, Q ; Ctrl+Entrée ; le séparateur se règle aux flèches. |
| Enregistrer | oui (K4) | Ctrl+S télécharge le classeur (web). |
| Partager | non mesuré | Demande un Serveur Lattis ; la boîte a `role="dialog"` et le focus piégé. |
| Paramètres | oui | Alt, F puis flèches dans le menu Fichier. |

Trouvé en route et corrigé : le sous-menu de la mise en forme conditionnelle
ne prenait pas le focus (Entrée l'ouvrait, Tab repartait dans la grille et
déplaçait la cellule active) ; la liste du gestionnaire de règles ne se
choisissait qu'à la souris ; les entrées des menus n'étaient annoncées
« élément de menu » qu'après une première flèche.

## 2. Constats, du plus gênant au moins gênant

1. **[corrigé] Erreurs de l'assistant en français dans les 5 langues**
   (77684343). `anthropicError`, `httpError`, `netError`, flux illisible ou
   interrompu, refus du modèle, `ErrBusy`, `ErrNoKey`, `ErrUnreachable`… :
   33 messages `ai.*` ; « le modèle local » se traduit ; les remarques de la
   conversation (arrêtée après N étapes, tronquée, session compactée) aussi
   (`ai.Notice`, traduite par le pont). Même passage pour « rien à annuler
   ou à rétablir », le refus d'annuler, l'erreur d'écriture du classeur,
   « requête vide », l'export SQL, le trousseau, le choix de fichier en web.
   Essai `TestProviderErrorsTranslated`.
2. **[corrigé] Bouton inexistant cité** : une clé refusée chez OpenAI,
   Mistral, Gemini… renvoyait au bouton « Compte » ; le bouton s'appelle
   « Connexion » (comme pour Claude).
3. **[corrigé] 7 panneaux flottants muets au lecteur d'écran** (filtre, ▾ de
   TCD, fonctions, doublons, note, insérer/supprimer des lignes, taille) :
   `role="dialog"`, nom, `tabindex="-1"` ; rien ne change à l'écran.
4. **[corrigé] Mise en forme conditionnelle au clavier** : sous-menu (Entrée
   ou → y entre, ← revient), liste des règles en `listbox` (flèches, Début,
   Fin, Entrée/F2 modifie, Suppr supprime) ; parcours K6.
5. **[corrigé] Noms de touches français partout** : « Ctrl+Maj+L »,
   « Suppr », « Pg.Suiv » s'affichaient tels quels en anglais, allemand,
   espagnol, chinois (ruban, menus, aide › Raccourcis). `keyLabel` les
   traduit (`common.keys.*`) ; essai `keynames.test.ts`.
6. **[corrigé] Menu contextuel et ruban** : le menu contextuel écrivait
   « Ctrl+X » en dur (pas de Cmd sur macOS, pas d'équivalent web) ; il passe
   par `keyLabel` comme le ruban et montre Maj+F2 sur la note, comme
   l'infobulle du ruban.
7. **[corrigé] 105 `svelte-ignore a11y_*`** : tous retirés puis
   `svelte-check` relancé : 56 ne masquaient plus rien ; les 49 gardés
   disent pourquoi (« volet : touches captées à son niveau », « surface de la
   grille ; le clavier passe par role=grid », « poignée de taille à la
   souris »…). Ceux sans équivalent clavier le disent et renvoient ici.
8. **[corrigé] 58 textes trop longs raccourcis** (89dd8724), 5 langues, même
   sens et mêmes paramètres : erreurs #CIRC!, #NOM?, #VALEUR! ; infobulle
   d'un tableau local (242 → 112) ; ajout d'un tableau partagé (291 → 169,
   deux paragraphes) ; fusion dans un tableau ; clé et jointure à choisir ;
   protection ; renommage ; version plus récente ; mise à jour…
9. **[arbitrage] « Excel » dans l'interface** : « Classeur Excel » (ruban
   Données, sources de requête), « Le classeur Excel (.xlsx)… », « à coller
   dans Excel » (SQL), 6 messages du moteur. Options : « Classeur .xlsx » /
   « fichier .xlsx » et « à coller dans un tableur » (règle « ne pas nommer
   de concurrent ») ; ou garder le mot que l'utilisateur cherche.
10. **[arbitrage] Même commande, deux noms** : « Nouvelle note » (ruban) et
    « Insérer une note » (menu contextuel) ; « Collage spécial » a la lettre
    AG sous Accueil et CS sous Données. Proposition : « Nouvelle note »
    partout, une seule lettre. Non fait : change des libellés visibles.
11. **[arbitrage] Maj+F3** : « Précédent » dans la barre de recherche,
    « Insérer une fonction » dans la grille. Selon le focus, sans conflit ;
    garder (comme un tableur) ou réserver Maj+F3 à la fonction.
12. **[arbitrage] Textes de consentement gardés longs** (accès réseau 248,
    fichier 193, tuiles de carte 197, page web 161) : ils expliquent un
    risque avant un « Autoriser ». Les raccourcir est possible (une phrase
    + « En savoir plus »), au prix de l'explication.
13. **[arbitrage] Lecteur d'écran réel** : NVDA n'a pas été lancé (pas de
    session interactive) ; 30 min avec NVDA sur les 10 tâches valideraient
    les annonces (cellule active, menus, boîtes).
14. **[plus tard] Sans équivalent clavier** (≈ 1,5 j) : coupures de
    Données › Convertir en largeur fixe (un champ « positions » changerait
    la boîte : arbitrage), déplacement et taille des segments et des
    contrôles de feuille, taille d'une forme ou d'une image (le déplacement
    aux flèches existe), glisser-déposer du catalogue Data Lake (« Déplacer
    vers… » à ajouter).
15. **[plus tard] Textes restants** (≈ 0,5 j) : 119 textes de 120 à 140
    caractères affichés ; consignes destinées à l'IA et mesure de défilement
    laissées volontairement.
16. **[plus tard] 63 `errors.New`/`fmt.Errorf` en français hors catalogue**
    (≈ 0,5 j à trier) : options de ligne de commande, sentinelles internes
    jamais montrées (`ErrPivot`, `errStop`…), générateur de démonstration,
    signature des mises à jour ; aucun n'est une erreur courante de
    l'interface. Faux serveurs d'essai laissés.
17. **[plus tard] Parcours clavier à ajouter** (≈ 0,5 j) : TCD, graphique,
    SQL, paramètres en e2e (aujourd'hui vérifiés à la main).

## Lancer

```
npm --prefix frontend run e2e          # 21 parcours dont 6 au clavier seul
npx --prefix frontend vitest run src/lib/web/keynames.test.ts
go test ./internal/ai -run TestProviderErrorsTranslated
```

Outils de relevé (non versionnés) : longueur affichée des textes (pluriel
« other », paramètre = 8 caractères), tri des `svelte-ignore` par
`svelte-check --output machine` sur les fichiers sans commentaires.
