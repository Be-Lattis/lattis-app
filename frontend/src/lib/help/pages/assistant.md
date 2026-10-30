# Assistant IA

L'**Assistant ✦** aide à construire une analyse : il lit la structure de vos tableaux, propose une démarche, et peut ajouter des colonnes calculées, des clés, des tableaux croisés et des graphiques, trier ou mettre en forme. Il s'ouvre dans un volet à droite de la grille, et reste ouvert quand vous changez de feuille.

## Activer l'assistant

L'assistant est coupé par défaut : cochez **Assistant IA** dans Fichier › Paramètres › Fonctionnalités. Le bouton **Assistant ✦** apparaît alors en haut à droite du ruban (`Alt+I`) et ouvre son volet ; décoché, ni bouton, ni raccourci, ni volet. Dans une entreprise, la stratégie de l'organisation peut le couper ou l'imposer.

## Se connecter

Le bouton **Connexion**, dans l'en-tête du volet, propose deux voies. Le panneau s'ouvre de lui-même tant que rien n'est connecté.

### Clé d'API (recommandé)

1. Choisissez le fournisseur (Anthropic, Mistral, OpenAI, Gemini, Mammouth…) et le modèle, puis **Ouvrir la console** : la page du fournisseur où l'on crée une clé.
2. Copiez la clé, puis **Coller** (ou `Ctrl+V` dans le champ).
3. **Vérifier et enregistrer** : Lattis essaie la clé auprès du fournisseur avant de l'enregistrer. Une clé refusée ou un compte sans crédit donne un message clair dans le panneau.

La clé est gardée dans le trousseau du système, jamais dans le classeur. **Oublier la clé** la retire.

> Chez Anthropic, une clé ne marche qu'avec des crédits : ajoutez-en dans la page **Facturation** de la console (platform.claude.com). Les abonnements **Claude Max** et **Team** donnent des crédits d'API chaque mois (de 100 à 500 $ selon la formule) : liez l'organisation de la console dans claude.ai › Paramètres › Facturation pour les recevoir. Les abonnements Pro et Enterprise n'en donnent pas.

### Application de bureau (abonnement Claude Pro, Max, Team)

Un abonnement Claude ne s'utilise pas directement dans une autre application, mais **Claude Desktop** et **Claude Code** savent piloter Lattis par son serveur MCP. L'onglet « Application de bureau » du panneau de connexion (application : Claude) les configure en un clic ; redémarrez ensuite Claude Desktop.

Vous parlez alors à Claude dans son application. Le volet de l'assistant affiche **Depuis Claude** : ce que Claude fait dans le classeur, au fil de l'eau, avec **Annuler ces modifications** pour chaque série d'actions. L'option **Demander avant de modifier** fait valider chaque écriture dans Lattis, y compris l'enregistrement sous un autre nom, les exports et les imports. Le réglage **Données transmises à l'assistant** s'applique aussi à Claude (voir plus bas).

### Passerelle de l'organisation

Quand l'entreprise fournit sa propre passerelle, le volet indique « Connecté par la passerelle de votre organisation » : il n'y a ni clé à saisir ni fournisseur à choisir.

### Modèle local

Le fournisseur **Modèle local** utilise un modèle installé sur ce poste (Ollama, LM Studio…) : aucune donnée ne quitte l'ordinateur, et il n'y a pas de clé. Son adresse se règle dans le panneau de connexion ou dans Fichier › Paramètres › Assistant IA. C'est expérimental : les petits modèles appellent moins bien les outils de Lattis.

## Converser

Écrivez votre demande, puis `Entrée` (`Maj+Entrée` va à la ligne). Chaque message part avec ce que vous regardez : la feuille active, le tableau sous le curseur et la sélection, indiqués au-dessus de la zone de saisie (« Contexte : Commandes, sélection Montant[3:10] »). « Analyse ce tableau » ou « explique cette colonne » s'appliquent donc à ce qui est sélectionné.

- Les étapes de l'assistant (lecture d'un tableau, ajout d'une colonne…) se regroupent dans une ligne **N étapes**, dépliée pendant la réponse puis repliée ; un clic la rouvre. Le nom d'une étape soulignée mène à la feuille ou au tableau concerné.
- Une étape en échec que l'assistant a corrigée ensuite s'affiche en gris ; seule une erreur restée sans suite est en rouge.
- `Échap` arrête la réponse en cours. Arrêtée avant que le modèle ait répondu, ou en échec à ce moment-là (réseau, quota), elle disparaît et votre message revient dans la zone de saisie : le modèle ne l'a pas reçu.
- La flèche haut, dans une zone de saisie vide, rappelle le message précédent ; le bouton de copie, sous une réponse, la met dans le presse-papiers.
- **Réessayer**, sous une réponse interrompue par une erreur (fournisseur surchargé, coupure), la reprend là où elle s'est arrêtée, sans renvoyer votre message : ce qu'elle a déjà modifié n'est pas refait, et la réponse reste annulable d'un coup. Quand l'erreur demande d'attendre (« réessayez dans 30 s »), le bouton compte les secondes.
- Une réponse très longue s'arrête après 100 étapes : **Continuer** la reprend de la même façon.
- Une question sur les données (« quelle ville a le meilleur chiffre d'affaires ? ») reçoit un **résultat** sous la réponse : l'assistant écrit une requête, Lattis l'exécute sur le poste et vous montre au plus 50 lignes. En mode « Structure seulement », le résultat est pour vous seul : l'assistant n'en voit ni les valeurs ni le nombre de lignes.
- L'assistant répond dans votre langue et écrit les formules comme votre grille (`SOMME(a; b)` et `1,2` en français, `SUM(a, b)` et `1.2` en anglais). Après une formule, il vérifie les erreurs (#NOM?, #REF!…) et les corrige avant de répondre.
- Un lien dans une réponse affiche son domaine (« Voir le rapport (exemple.fr) ») ; un clic montre l'adresse entière et demande confirmation avant de l'ouvrir dans le navigateur.
- **Nouvelle** (en-tête du volet) commence une nouvelle conversation ; la précédente reste dans la liste des conversations (☰), d'où elle se reprend ou se supprime (après confirmation).
- Quand la conversation devient longue, l'assistant en résume le début pour continuer : le repère **Session compactée** marque l'endroit. Tout reste affiché dans le volet ; seul le modèle travaille désormais sur le résumé.

## Réparer des tableaux mal reconnus

La suggestion **Réparer les tableaux de cette feuille** fait relire à l'assistant la disposition de la feuille : où sont les tableaux, la nature des cellules de leur première et de leur dernière ligne (texte, nombre, date, formule, vide), les cellules libres en gras ou fusionnées, et les lignes qui ressemblent à un titre ou à un total. Il n'en lit pas les valeurs, même en mode « Structure seulement ». Il corrige ensuite avec les commandes qui gardent le tableau, sa clé et les formules qui le citent : titre pris pour l'en-tête, en-tête resté dans les données, ligne de total sortie du tableau, en-têtes sur deux lignes, colonnes à scinder.

Ce que l'assistant lit dans le classeur (noms, texte des cellules) lui est présenté comme une donnée, jamais comme une consigne : une cellule qui lui « demande » de supprimer une colonne ne doit pas être suivie, et il le signale. Il ne supprime une colonne, un graphique, un tableau croisé ou des doublons que si vous le demandez.

Le réglage **Garder la conversation dans le classeur** (Fichier › Paramètres › Assistant IA) l'enregistre avec le classeur ; sinon, elle s'efface quand on ferme le classeur ou qu'on en ouvre un autre. Gardée, elle voyage avec le fichier, résultats montrés et lignes lues par l'assistant compris, et n'est enregistrée que si le classeur a aussi été modifié. Si le classeur change hors des réponses (import, `Ctrl+Z`, conversation reprise plus tard), l'assistant reçoit sa structure à jour avec le message suivant.

## Annuler

Tout ce que l'assistant modifie pendant une réponse forme une seule étape : la réponse se termine par son bilan (« 3 modifications ») et le bouton **Annuler cette réponse**. `Ctrl+Z` dans la grille fait la même chose. Le bouton n'est actif que pour la dernière réponse, tant que rien d'autre n'a été modifié depuis ; sinon, utilisez `Ctrl+Z`.

## Données transmises

Le réglage **Données transmises à l'assistant** (Fichier › Paramètres › Assistant IA, rappelé en bas du volet) choisit ce que l'assistant peut lire :

| Réglage | Ce que reçoit le fournisseur |
| --- | --- |
| Structure seulement (par défaut) | Noms des feuilles, tableaux et colonnes, types, nombre de lignes et de valeurs distinctes, formules, place des tableaux et des cellules sur la feuille (sans leur contenu, avec leur nature et les indices « titre » ou « total ») ; pas le résultat des requêtes montrées, ni le nombre de cellules d'un remplacement ; étapes des requêtes et paramètres des connexions, sans mot de passe ni valeurs des en-têtes HTTP et des filtres. Les colonnes d'un tableau croisé qui portent les valeurs d'un champ sont résumées (« 3 colonne(s) de valeurs du champ « Client » ») |
| Structure et lignes d'exemple | La même chose, plus quelques lignes à chaque lecture : 20 lignes au plus, 200 caractères par valeur |
| Tout le classeur | Tout ce que l'assistant choisit de lire, sans limite |

Le fournisseur reçoit aussi vos messages et le contexte (feuille, tableau, sélection). Avec un modèle local, rien ne quitte le poste.

Le réglage vaut aussi pour **Claude Desktop** et **Claude Code** par le serveur MCP : sans « Tout le classeur », les outils qui lisent les valeurs (lignes des tableaux, SQL, cellules, scripts, exports) ne leur sont pas proposés ou sont bornés de la même façon. Après un changement, redémarrez Claude pour qu'il voie la nouvelle liste d'outils. Claude Code lit aussi les fichiers de votre disque : un classeur enregistré lui reste lisible directement.

## Coût

Avec une clé d'API, chaque réponse est facturée par le fournisseur selon le nombre de jetons lus et écrits. Avec une clé Anthropic, le pied du volet affiche une estimation du coût de la conversation (« ≈ 0,12 $ cette conversation ») d'après les tarifs publics ; la facture du fournisseur fait foi. Rien ne s'affiche pour les autres fournisseurs ni à travers une passerelle, dont les tarifs diffèrent. Une longue suite d'étapes coûte plus cher : un modèle plus léger (Sonnet plutôt qu'Opus) suffit le plus souvent.

## Limites

- L'assistant peut se tromper : relisez ce qu'il propose, et annulez une réponse qui ne convient pas.
- En mode « Structure seulement », il ne voit pas les valeurs : il ne peut pas juger du contenu des cellules. En mode « Structure et lignes d'exemple », il n'en voit qu'une partie.
- Une très longue conversation est résumée (« Session compactée ») : l'assistant peut alors oublier un détail des premiers échanges. Pour un nouveau sujet, commencez plutôt une nouvelle conversation.
- Avec Claude Desktop ou Claude Code, la conversation reste dans Claude ; Lattis ne voit que les actions.
