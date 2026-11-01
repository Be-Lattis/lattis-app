# Audit : validation des données et formulaires (01/11/2026)

Audit seul, aucun code produit modifié. Tu décides avant toute correction.

**Essais réels.** Moteur et pont : classeur de 13 colonnes, une règle de chaque type, essai jetable (saisie, collage, script, annulation, autre membre). Concepteur : définition produite par le vrai code (`newDraft`, `buildDefinition`), sans y toucher. Serveur : cette définition soumise à `lattis.form_check` et à la vraie page `/f/…` (copie du dépôt serveur, Postgres local `lattis-pgtest`), une vingtaine d'envois. Grille : édition web, Playwright. **Limite :** le concepteur n'a pas été cliqué en ligne (il faut lattisd et la connexion), le pont a été essayé avec le faux serveur. Production jamais touchée.

## Les 5 plus grosses incohérences

1. **Le formulaire proposé d'office est refusé par le vrai serveur** dès qu'une colonne a une liste courte (radio) ou un nom en plusieurs mots (`espece-observee`, le serveur interdit le tiret). Deux options du concepteur sont refusées aussi : le format Téléphone, et Texte sur une colonne nombre. Les essais de l'application passent, car le faux serveur ne contrôle ni le type de champ ni le nom court.
2. **Seules les listes passent de la grille au formulaire.** Entier, décimal, date, heure, longueur, formule, message de saisie, message d'erreur, alerte et « Ignorer si vide » sont ignorés sans rien dire. Le serveur a accepté Qté = 999 (règle 1 à 10), Prix = -5 (règle > 0), Heure = « bonjour », une date de 2000 (règle ≥ 2026).
3. **La même règle se définit à deux endroits, jamais reliés.** Le champ a ses propres Min, Max, Longueur, Obligatoire, Format et sa liste modifiable. On a posé 100–200 sur une colonne réglée 1–10 : personne ne le signale, et aucune valeur ne peut alors être valable partout. Une règle changée après coup ne suit pas, et les lignes reçues hors règle ne sont pas signalées.
4. **Une liste de dates, de nombres ou de Oui/Non casse à l'envoi.** `01/02/2026` a été **enregistré 2026-01-02 (jour et mois inversés, sans erreur)**. `19/10/2026`, `3,5` et `VRAI` donnent « Le serveur n'a pas pu enregistrer la réponse : réessayez », sans explication pour le répondant.
5. **Le type de colonne et la règle se contredisent.** Hors type, le type accepte et marque ; la règle refuse. Une règle Entier sur une colonne de texte refuse même `5`. Une règle Heure dans un tableau refuse tout, `10:00` compris, car il n'existe pas de colonne heure. Aucun avertissement à la pose.

## Matrice

| Règle ou réglage | Grille | Concepteur | Page `/f/` | Serveur à l'envoi | Écart |
|---|---|---|---|---|---|
| Liste fixe, texte | Déroulante, refus | Copiée, modifiable, jamais resynchronisée | Radio ≤ 7, sinon liste | Stricte, sans casse | Radio refusé par `form_check` |
| Liste fixe, nombre, date, booléen | Correct | Copiée telle quelle | Idem | Trouvée dans la liste, puis échec du cast | Date inversée ou erreur 503 |
| Liste : colonne d'un tableau publié | Déroulante, ordre source | Citée, relue | Tri alphabétique, `::text` | Stricte | Correct, sauf ordre et affichage ISO des dates |
| Liste : cellules, nom, tableau privé | Déroulante | Copiée (« copiée le… ») | Idem | Stricte sur la copie | Figée ; valeurs privées exposées sans avertissement |
| Entier, décimal + opérateur | Refus | Rien, Min et Max vides | Champ nombre | Type seul | **Ignorée** (999 et -5 acceptés) |
| Date + opérateur | Refus | Rien, pas de Min/Max | Sélecteur de date | Format seul | **Ignorée** |
| Heure | Cellule libre : correct. Tableau : tout refusé | Rien | Texte libre | Rien | **Ignorée** (« bonjour ») |
| Longueur | Refus | Max 10 000 d'office, sans lien | `maxlength` | Maximum seul | **Ignorée** (7 lettres pour « = 3 ») |
| Formule, borne en formule | Calculée | Rien | Rien | Impossible : pas de moteur | **Ignorée** |
| Message de saisie | Bulle | Aide vide | Rien | — | **Perdu** |
| Message d'erreur | Boîte | Rien | Texte du serveur | Texte du serveur | **Perdu** |
| Alerte Avertissement, Informations | Oui/Non, OK | Rien | Rien | Liste : refus strict. Autres : rien | La grille permet, le formulaire refuse |
| « Ignorer si vide » décoché | Refus à la frappe | Obligatoire reste décoché | Pas d'astérisque | Vide accepté | **Ignoré** |
| Règle sur une partie de colonne | Appliquée | Non vue (`rid = 0` seul) | Rien | Rien | **Ignorée** |
| Type entier, décimal, date, booléen | Accepte et marque | Choix du champ | Champ typé | Strict | Inverse : `abc` accepté en grille, refusé au formulaire |
| Obligatoire, Format, Pas (formulaire seul) | N'existent pas | Réglables | Astérisque | Pas : ni transmis ni contrôlé | Pas sans effet ; Téléphone refusé |

**Publication.** `dv` part bien au serveur : il est dans `_lattis_styles` et ses voisines (`layout.go:29`). Le serveur le garde sans le lire : `form_page` et `checkValue` ne regardent que la définition du formulaire. Il sait contrôler des bornes constantes, pas une formule.

## Constats classés

**Bloquant**
- **F1. Définition par défaut refusée.** Radio : `forms.ts:188` contre `0030_forms_app.sql:95-105`. Nom court : `forms.ts:254` contre `0030:124`. Téléphone : `FormDialog.svelte:549` (`tel`) contre `0030:116` (`phone`). Texte sur nombre : `forms.ts:189`. Faux serveur plus souple : `workbooktest/forms.go`, et `bridge/forms_test.go:174` envoie même `radio`.
- **F2. Listes non textuelles.** `validate.go:148-155` rend la valeur de la liste avant la normalisation par type ; `forms.go` la passe telle quelle à `$n::date`. Capture : [erreur à l'envoi](captures-validation-formulaires/form-page-erreur-decimale.png).
- **F3. Liste tirée d'un tableau privé (en attente depuis le 21/10).** La règle est publiée, pas la table. Essai avec un autre membre éditeur : « b » refusé, « Attendu : le tableau « Secret » n'existe plus » (`dvrules.go:669-673`). Il ne peut plus rien saisir dans cette colonne. Côté formulaire, les valeurs privées sont copiées dans une page servie aux répondants ; `forms.ts:599` ne prévient que pour une liste relue par le serveur. Je n'ai retrouvé du 21/10 que la ligne de `arbitrages-2026-10-30.md:48`, pas le texte de la proposition.

**Important**
- **F4. Règles perdues.** `bridge/forms.go:400-406` ne garde que les listes, et `:338` ne lit que la couche de colonne. Capture : [la page, sans aucune trace des règles](captures-validation-formulaires/form-page-reelle.png).
- **F5. Pas de suivi.** La définition est une photo. « Mettre à jour » n'existe que pour une liste copiée d'une plage. Rappel prévu à l'ouverture (conception, ligne 70) et ligne reçue marquée (ligne 231) : non faits. Une règle ajoutée après coup n'entre pas dans le champ (`draftFromForm`).
- **F6. Portée de la règle selon la sélection.** Le dialogue s'applique à ce qui est sélectionné : cellule active seule dans un tableau = cette ligne (« cellule B2 »), donc ni les lignes suivantes ni le formulaire. Il faut sélectionner toutes les lignes de données ou la colonne pour obtenir une règle de colonne (vérifié).
- **F7. Impasses du répondant.** Défaut caché invalide : « Corrigez les champs signalés » sans champ visible, envoi impossible (essai ; `form_check` ne vérifie pas les défauts). Même risque, d'après le code, pour un lien verrouillé avec une valeur hors liste. Valeur fixe de serveur invalide : erreur de cast.
- **F8. Aperçu différent de la vraie page.** « Une section par page » n'existe pas côté serveur, alors que l'aide la promet. « Liste » ≤ 7 valeurs s'affiche en liste à l'aperçu, en radio sur la page (`validate.go:109`). `data-min` et `data-max` sont écrits mais lus par aucun script (`form.html:59`). L'aide d'une case à cocher n'est pas affichée.

**À corriger, sans décision**
- **F9. Différences avec un tableur.** La recopie (`doFill`, valeurs seules) ne recopie pas la règle : D3 recopiée accepte 99, alors que le copier-coller la porte. Une formule n'est pas vérifiée (`actions.go:96`) : `=99` passe une règle < 3, un tableur contrôle le résultat.
- **F10. Deux affichages du refus.** Liste simple : barre d'état, sans boîte ([capture](captures-validation-formulaires/form-validation-liste-refus.png)). Liste avec options et autres règles : boîte Réessayer/Annuler ([capture](captures-validation-formulaires/form-validation-alerte.png)). Cause : deux chemins (`validation.svelte.ts:413`).
- **F11. Cercles.** Une valeur hors type (`abc` en décimale sans règle) n'est pas entourée. Elle peut aussi entrer dans une liste tirée de la colonne (`[500 abc]`).
- **F12. Accès.** « Lien public » est toujours désactivé dans le concepteur (`FormDialog.svelte:671`), même si le serveur le permet, alors que la conception en fait un cas d'usage.

**Conforme à l'attendu.** Essayé ici : `[@Colonne]`, alertes, annulation en une étape, ligne ajoutée qui hérite de la règle de colonne, collage et script sans contrôle (comme un tableur, avec « Entourer » pour voir après coup), Suppr qui passe, refus de type et de liste au formulaire. Couverts par les essais du dépôt, non rejoués : chaque opérateur, bornes relatives, import et export .xlsx.

## Conception cible (recommandée)

1. **La règle de la colonne (`dv`, `rid = 0`) est la seule source.** Le formulaire ne définit plus ce qui est permis.
2. Le champ ne garde que la présentation : libellé, aide, ordre, type de champ compatible, défaut, nom court, section.
3. À chaque enregistrement, l'application **compile** la règle dans le champ, en constantes normalisées (nombre avec point, date ISO) : `rule` en lecture seule dans le concepteur, avec une ligne « Vient de la colonne : entier de 1 à 10 ».
4. Le message de saisie devient l'aide par défaut, le message d'erreur remplace celui du serveur, « Ignorer si vide » décoché rend le champ obligatoire.
5. Le serveur contrôle `rule` sans jamais lire la mise en page. Il reste sans moteur.
6. Formule et borne en formule : non contrôlées au serveur, dit au concepteur, ligne marquée à l'arrivée.
7. Le formulaire peut seulement **durcir** : Obligatoire, Format, Min et Max dans la règle ; un réglage contraire bloque l'ouverture.
8. Règle changée : les formulaires ouverts du tableau sont recompilés et réenregistrés par le poste du concepteur, avec un message court.
9. Ligne reçue hors règle : entourée d'office à la réception, compteur dans la barre d'état.
10. Dans la grille, le type dit la forme de la valeur, la règle dit ce qui est permis ; une règle impossible pour le type est signalée à la pose.

## Décisions à prendre

**[arbitrage] A1. Correctifs immédiats (F1, F2, sans changement de modèle).** Oui / non. **Recommandé : oui, à sortir seul**, 1 j. Application (0,5 j) : `_` dans le nom court, `phone`, retrait de Texte sur nombre, mêmes contrôles dans le faux serveur. Serveur (0,5 j) : migration `form_check` qui admet `radio` si le champ a une liste ; `Field.widget` qui respecte `list` ; `checkValue` qui, après une liste trouvée, repasse par la branche du type, et lit vrai/faux.

**[arbitrage] A2. Qui définit la règle ?**
- a) La colonne, le formulaire en dérive (cible ci-dessus). 2 j application, 1,5 j serveur. **Recommandé.**
- b) Deux définitions, champ pré-rempli depuis la règle et alerte en cas d'écart. 1,5 j application, 1 j serveur ; le doublon reste.
- c) Statu quo, avec une phrase dans l'aide. 0,1 j ; le trou reste.

Serveur pour a) : clé `rule` dans le champ, format de définition inchangé : `{type: whole|decimal|date|time|length, op, min, max, blank, on_fail: stop|warning|info, message}`. `form_check` vérifie `type`, `op` et la forme des bornes ; `checkValue` applique les huit opérateurs (date et heure comprises, longueur minimale et exacte) et rend `message` s'il existe.

**[arbitrage] A3. Formules (non contrôlables au serveur).**
- a) Acceptées, avertissement au concepteur, ligne marquée à l'arrivée. Compris dans A2 et A6. **Recommandé.**
- b) Refuser d'ouvrir le formulaire. 0,2 j ; bloque pour une règle parfois secondaire.
- c) Mettre un moteur côté serveur. Plusieurs semaines ; contraire au choix « sans moteur ».

**[arbitrage] A4. Avertissement et Informations sur la page.**
- a) Avertissement : bouton « Envoyer quand même » au second passage ; Informations : envoi accepté. 0,5 j serveur (marche sans JavaScript), liste non stricte comprise. **Recommandé.**
- b) Tout traiter comme un refus. 0 j ; plus strict que la grille.
- c) Jamais de blocage. 0,2 j ; une liste n'en est plus une.

**[arbitrage] A5. Règle changée après l'ouverture.**
- a) Réenregistrement automatique par le poste du concepteur, en ligne (hors ligne : au retour). 1 j application. **Recommandé.**
- b) Pastille « Règle changée » et bouton Mettre à jour. 0,5 j ; on l'oublie.
- c) Le serveur lit la mise en page. 3 j ou plus ; il devrait connaître le format local.

**[arbitrage] A6. Ligne reçue hors règle.**
- a) Entourée d'office, compteur dans la barre d'état. 0,5 j. **Recommandé.**
- b) Seulement via « Entourer ». 0 j ; l'erreur reste invisible.

**[arbitrage] A7. Liste tirée d'un tableau privé (en attente du 21/10).**
- a) Source illisible : la saisie passe sur les autres postes, sans refus, et le concepteur est prévenu ; avertissement « valeurs visibles » aussi pour une liste copiée. 0,5 j. **Recommandé d'abord.**
- b) Figer les valeurs dans la règle à la publication (« copiée le… »). 1 j ; la liste vieillit et les valeurs sortent du poste.
- c) Refuser la pose, et proposer d'ajouter le tableau au classeur partagé. 0,5 j ; le plus net, mais bloque les règles déjà posées.

**[arbitrage] A8. Type et règle.**
- a) Avertissement à la pose si la règle est impossible pour le type, et lecture `hh:mm` pour une règle Heure sur du texte. 0,5 j. **Recommandé.**
- b) Interpréter le texte avant de comparer (`5` vaut 5 sur une colonne de texte). 1 j ; le résultat n'est plus lisible.
- c) Remplacer le type par une règle. Plusieurs semaines ; rien ne le justifie.

**[arbitrage] A9. Portée dans un tableau (F6).**
- a) Dans un tableau, case « Toute la colonne » cochée d'office. 0,3 j. **Recommandé.**
- b) Toujours toute la colonne. 0,2 j ; perd la règle par ligne.
- c) Statu quo.

**[arbitrage] A10. « Une section par page » (F8).**
- a) La faire sur la page du serveur (JavaScript, repli page unique). 1 j serveur.
- b) Retirer l'option du concepteur et de l'aide. 0,2 j. **Recommandé pour l'instant.**

Sans décision, à la suite : F9 à F11 (0,5 j), aperçu aligné sur la page (0,5 j). **Ordre conseillé :** A1, A7a, A8, A9, puis A2 avec A3, A4, A5, A6. Total environ 9 j, dont 2,5 j serveur.
