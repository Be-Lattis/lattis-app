# Thèmes de couleur

*20/10/2026. Adoptée (demande de Louis : « Dans le ruban Affichage, j'aimerais qu'on puisse choisir un thème de couleur parmi une liste. Ça permettrait de changer à la fois la couleur des tableaux classiques et des tableaux sources externes », « Et que ce soit customisable. Y compris par l'entreprise. Donc exposé comme variable. », puis « ça doit être par classeur en général, mais le thème par défaut doit être exposé », et l'entreprise doit « pouvoir gérer les listes de différentes propositions de thèmes »). Code : `internal/canvas/theme.go`, `internal/config/ui.go`, `internal/remote/workbook/layout.go` (`Layout.Theme`, `applyTheme`), `frontend/src/lib/grid/theme.ts`, `theme.svelte.ts`, `shell/ThemeDialog.svelte`, `Sheet.svelte` (palette `C`), `Ribbon.svelte` (Affichage › Thème) ; essais : `canvas/theme_test.go`, `config/ui_test.go`, `remote/workbook/theme_test.go`, `grid/theme.test.ts`, `shell/settings.test.ts`.*

## Règle

**Un thème, ce sont deux couleurs de base** : celle des tableaux ordinaires (vert aujourd'hui) et celle des tableaux chargés par une source externe ou une requête (bleu). `family(base)` en tire les nuances : fond d'en-tête pâle (luminosité 92 %), texte d'en-tête foncé assombri jusqu'à un contraste d'au moins 4,5:1 sur ce fond, bordure, icônes, pastille du nom et pastille « suit l'année » (texte blanc lisible). Une base peu saturée (gris) reste grise. Un thème personnalisé n'a donc besoin que de deux couleurs, et un thème inconnu d'un poste s'affiche quand même. « Vert et bleu » garde exactement les couleurs d'avant (nuances fixées pour ses deux bases).

**Le thème choisi appartient au classeur.** Il est gardé dans `_lattis_info` (clé `theme`, JSON `{id, name, table, source}`), sans migration du format. Le choisir est une modification du classeur : `Ctrl+Z` l'annule, il s'enregistre avec le fichier, et un collègue le voit pareil même sans ce thème dans sa liste (les deux couleurs voyagent avec le nom). Un classeur partagé sur un Serveur Lattis le publie avec sa mise en page (`Layout.theme`, seule clé de `_lattis_info` publiée ; la table entière ne l'est pas : identifiant, conversation de l'assistant, état de synchronisation…). À la réception, le serveur l'emporte, comme pour le reste de la mise en page.

**Le thème par défaut et la liste proposée sont des réglages.** Un classeur sans thème (neuf, ou d'avant les thèmes) prend le thème par défaut du poste (`ui.theme`, « Vert et bleu » d'office) : on n'écrit pas de thème dans chaque classeur neuf, pour qu'un changement du défaut (par l'utilisateur ou la DSI) vaille pour tous les classeurs qui n'ont rien choisi.

| Clé | Rôle | Défaut |
|---|---|---|
| `ui.theme` | identifiant du thème par défaut | `vert-bleu` |
| `ui.themes.<id>` | thème ajouté : `{"name", "table", "source"}`, couleurs `#rrggbb` | aucun |
| `ui.builtinThemes` | proposer les thèmes prédéfinis | `true` |

Les objets de la configuration se fusionnant couche par couche, l'organisation fournit ses thèmes (fichier machine, configuration distante, stratégie) et l'utilisateur ajoute les siens à côté (« Ajouter à mes thèmes », écrit thème par thème dans son `config.json`, jamais `ui.themes` d'un bloc). L'interface ne propose de retirer que les thèmes venus du fichier de l'utilisateur (`managed` donne les autres). `ui.builtinThemes = false` ne propose que ceux de l'organisation ; une stratégie peut imposer `ui.theme`. Un `ui.theme` absent de la liste retombe sur « Vert et bleu », sinon sur le premier proposé.

**Exposé en variables.** Le thème appliqué est posé sur la racine du document en variables CSS (`--lattis-table`, `--lattis-table-head`, `--lattis-table-text`, `--lattis-table-border`, `--lattis-table-icon`, `--lattis-table-badge`, `--lattis-table-strong`, et `--lattis-source…`) ; la grille (canevas) lit les mêmes valeurs (`gridPalette`). L'accent de l'interface (#217346 : ruban, boutons, sélection) ne change pas : seules les couleurs des tableaux suivent le thème.

## Interface

- **Affichage › Thème ▾** : les thèmes prédéfinis (Vert et bleu, Bleu et orange, Violet et sarcelle, Sarcelle et violet, Gris et bleu, Orange et bleu, Bordeaux et gris), puis ceux de l'organisation, puis ceux de l'utilisateur, chacun avec ses deux couleurs ; celui du classeur est coché (un thème d'un autre poste aux mêmes couleurs qu'un thème de la liste est coché aussi). Puis **Personnaliser…** : deux sélecteurs de couleur, un aperçu dans la boîte et en direct dans la grille (Annuler le retire), un nom, « Ajouter à mes thèmes » ; la liste « Mes thèmes » permet de les retirer.
- **Fichier › Paramètres › Affichage** : le thème par défaut, parmi les thèmes proposés.
- **Scripts et MCP** : `doc.theme`, `doc.set_theme(table, source, name)` (sans couleurs : retour au défaut).

## Laissé de côté

- Un thème « à la Excel » plus large (polices, couleurs de graphiques, styles de cellules) : seules les couleurs des tableaux sont concernées.
- L'interdiction des thèmes personnels par l'organisation (une clé `ui.userThemes` pourrait venir) ; aujourd'hui elle peut seulement ne pas proposer les prédéfinis et imposer le défaut.
- Les panneaux et boîtes de l'interface (volet TCD, filtres…) gardent l'accent vert de l'application.
