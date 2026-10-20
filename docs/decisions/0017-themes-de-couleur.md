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

## Extension : couleurs du thème dans les palettes (20/10/2026)

Demande de Louis : « Améliore les palettes de couleurs qu'il y a dans le surlignement et dans la mise en couleur du texte […] Essaye de travailler comme sur Excel avec des gradients. On a quelques couleurs de base liées au thème et tout un gradient qui leur est associé », puis « les classiques rouge, orange, vert doivent être partout ». Code : `frontend/src/lib/grid/colors.ts`, `ColorPalette.svelte`, `ColorButton.svelte`, `theme.ts` (`themeColors`, `themeExtras`), `internal/canvas/themecolor.go`, `internal/transfer/exportstyle.go` ; essais : `grid/colors.test.ts`, `canvas/themecolor_test.go`, `transfer/themecolor_test.go`, `config/ui_test.go`.

**Dix couleurs par thème, comme Excel.** Arrière-plan 1 et Texte 1 (blanc et noir, fixes), Arrière-plan 2 et Texte 2 (`light`, `dark`), Accent 1 et 2 (les deux bases `table` et `source`), Accent 3 à 6 (`accents`, quatre au plus). Les nouvelles clés sont facultatives partout (thème du classeur, `ui.themes.<id>` ; `doc.set_theme` ne prend que les deux bases) : absentes, elles se déduisent des deux bases (accents à 120° et 240° de la teinte de l'une et de l'autre, saturation 45–75 %, luminosité 45 % ; Texte 2 très foncé dans la teinte de la source ; Arrière-plan 2 très pâle dans celle des tableaux), et un classeur qui a gardé un thème prédéfini sans accents (d'avant cette extension) reprend ceux du thème prédéfini aux mêmes bases. Aucune migration : le JSON du thème gagne des champs.

| Thème | Accent 3 | Accent 4 | Accent 5 | Accent 6 | Texte 2 | Arrière-plan 2 |
|---|---|---|---|---|---|---|
| Vert et bleu | `#e07b24` orange | `#7b4fb0` violet | `#c99a1e` or | `#b83b5e` framboise | `#34495e` | `#e8eeea` |
| Bleu et orange | `#16a34a` vert | `#9333ea` violet | `#ca8a04` or | `#db2777` rose | `#1e293b` | `#e2e8f0` |
| Violet et sarcelle | `#f59e0b` ambre | `#e11d48` rouge | `#2563eb` bleu | `#65a30d` vert | `#3b3355` | `#ede9f6` |
| Sarcelle et violet | `#f59e0b` | `#e11d48` | `#2563eb` | `#65a30d` | `#134e4a` | `#e6f2f1` |
| Gris et bleu | `#0d9488` sarcelle | `#d97706` orange | `#dc2626` rouge | `#7c3aed` violet | `#1e293b` | `#e2e8f0` |
| Orange et bleu | `#15803d` vert | `#a16207` or | `#7c3aed` violet | `#be123c` rouge | `#3b2f2a` | `#f3ebe5` |
| Bordeaux et gris | `#b45309` ambre | `#0f766e` sarcelle | `#1d4ed8` bleu | `#6d28d9` violet | `#3f1d2b` | `#efe7ea` |

Les accents sont assez foncés pour une police lisible sur le blanc (luminosité < 62 %) et distincts dans chaque thème. La liste est dupliquée en Go (`builtinThemes`) pour l'export ; `TestBuiltinThemesMatchFrontend` relit `theme.ts` et vérifie qu'elles sont égales.

**Palette (police, remplissage, bordure).** « Couleurs du thème » : une ligne des dix couleurs, puis cinq lignes de variantes, colonne par colonne, choisies selon la luminosité L comme Excel : plus clair 80, 60, 40 % et plus foncé 25, 50 % en général ; blanc : plus foncé 5 à 50 % ; noir : plus clair 50 à 5 % ; couleur très claire (L > 0,8) : plus foncé 10 à 90 % ; très foncée (L < 0,2) : plus clair 90 à 10 %. Le calcul est celui d'Excel sur la luminosité TSL : plus clair de t, L + (1 − L)·t ; plus foncé de s, L·(1 − s). Puis « Couleurs standard », les dix d'Excel, fixes (rouge, orange et vert restent toujours proposés, quel que soit le thème), « Couleurs récentes » (celles d'« Autres couleurs… », dix au plus, propres au poste : `localStorage`), Aucun remplissage ou Automatique, et Autres couleurs… (sélecteur du système). Infobulles nommées (« Vert, Accent 1, plus clair 40 % »), flèches, Début, Fin et Entrée au clavier. La même palette sert dans le ruban (Accueil, et l'onglet Document), Format de cellule et la mise en forme conditionnelle (bouton à pastille `ColorButton`).

**Une couleur du thème est une référence au thème.** Le style garde `theme:<emplacement>[:<±pourcentage>]` (`theme:accent1`, `theme:accent1:+40`, `theme:dk2:-25` ; emplacements `lt1`, `dk1`, `lt2`, `dk2`, `accent1` à `accent6`) au lieu de `#rrggbb`, dans `fg`, `bg` et `bc` : changer le thème du classeur recolore ces cellules, comme Excel. Les couleurs standard et « Autres couleurs… » restent `#rrggbb` ; les anciens classeurs ne changent pas. La résolution se fait au dessin (`paint`, la grille, les aperçus de Format de cellule et de la mise en forme conditionnelle, le bouton de dernière couleur du ruban) avec le thème appliqué. Copier-coller entre classeurs garde la référence (comme Excel) : la couleur prend le thème du classeur d'arrivée. Une page (Document, éditeur de texte) reçoit la couleur résolue, `#rrggbb`, car son HTML n'a pas de thème.

**Un classeur qui utilise une couleur du thème garde son thème.** Un classeur sans thème propre suit le défaut du poste ; dès qu'une couleur du thème y est posée depuis le ruban, le thème appliqué y est enregistré (comme un fichier Excel porte toujours son thème), pour que la couleur soit la même chez un collègue et à l'export. Les références posées autrement (Format de cellule, mise en forme conditionnelle, scripts) dans un classeur sans thème se résolvent avec le défaut du poste à l'écran et « Vert et bleu » à l'export.

**Export.** L'export Excel et OpenDocument écrit la couleur résolue `#rrggbb` avec le thème du classeur (`canvas.ThemeColors`, `ResolveColor`, mêmes calculs que l'interface, vérifiés par les mêmes valeurs dans les deux suites d'essais), pas l'indice de thème d'Excel : le fichier s'affiche pareil partout, mais Excel n'y voit pas une couleur de thème. Les scripts et le MCP acceptent les références (`color`, `fill`, `border_color`).

## Laissé de côté

- Un thème « à la Excel » plus large (polices, couleurs de graphiques, styles de cellules) : seules les couleurs des tableaux et celles des palettes sont concernées.
- L'export des couleurs du thème en couleurs de thème d'Excel (`theme="4" tint="0.4"` et un `theme1.xml` complet) : couleurs résolues seulement.
- Les barres de données et nuances de couleurs de la mise en forme conditionnelle gardent leurs couleurs fixes.
- L'interdiction des thèmes personnels par l'organisation (une clé `ui.userThemes` pourrait venir) ; aujourd'hui elle peut seulement ne pas proposer les prédéfinis et imposer le défaut.
- Les panneaux et boîtes de l'interface (volet TCD, filtres…) gardent l'accent vert de l'application.
