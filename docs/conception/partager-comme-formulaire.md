# Partager un tableau comme formulaire

*Étude de conception, 19/10/2026, demandée par Louis : « Sur n'importe quel tableau partagé, on pourra faire un clic droit et "Partager comme formulaire". S'il n'est pas partagé, il faudra mettre une erreur pour dire qu'il faut partager le tableau. Je te laisse le spécifier tout seul. » Ajout de Louis : « pense que les utilisateurs aient un petit peu de personnalisation » (§4). Pas de code. Lue sur `main` de lattis-app (`22ea40d`) : `docs/collaboration/` (publication, synchronisation, droits, pipeline-table-source), décisions 0010, 0011 et 0014, `internal/canvas/validation.go` (listes tirées d'une plage), `internal/canvas/choice.go` (onglet à choix). Côté be-lattis-server, en lecture seule : `docs/contrat-client.md`, `schema/0004`, `0005`, `0020_pipelines.sql` (identité technique), `internal/httpserver` (`/api/preview`, `/api/push`, limiteur), `internal/admin/links.go` (`/w/<id>`).*

## En bref

- **Le besoin** : faire remplir un tableau par des personnes qui n'ouvrent pas le classeur. On leur donne une page web avec un champ par colonne et un bouton Envoyer. Chaque envoi ajoute une ligne au tableau, que les membres voient arriver en direct dans l'application.
- **Seul un tableau publié dans un classeur collaboratif** peut avoir un formulaire, parce que c'est le serveur qui sert la page et reçoit les réponses. Dans les autres cas, on affiche un message clair avec l'action à faire d'abord (« Publier sur un serveur… », « Ajouter au classeur partagé… »).
- **Le serveur fait l'essentiel** : il garde la définition du formulaire, sert la page `/f/<code>`, valide la réponse et insère la ligne. L'application conçoit et gère le formulaire. La ligne arrive sur les postes par la synchronisation existante (notification du tableau, `changes_since`) : la réception n'a rien de neuf à faire.
- **Choix structurant** : les lignes sont écrites sous une **identité technique** « Formulaire : <titre> », sur le modèle des pipelines (`0020_pipelines.sql`). Le répondant n'a donc aucun droit sur le classeur. Le serveur le note à part, dans le journal des réponses, et le formulaire peut aussi l'écrire dans une colonne « Répondant ».
- **Trois modes d'accès** : membres du classeur (par défaut), personnes ou équipes du serveur choisies (connexion Lattis), et lien public anonyme. Le lien public est désactivé par défaut au niveau du serveur et vient avec des protections contre les abus.
- **Estimation côté client** : M (concepteur et création), puis S (gestion), puis S (lien public et options). Côté serveur, c'est le gros morceau : M à L (§11).

## 1. Cas d'usage

| Cas | Exemple | Ce qui compte |
|---|---|---|
| **Relevé de terrain** | Le classeur des nids de Louis : des bénévoles saisissent une observation par ligne depuis leur téléphone (date, commune, espèce dans une liste, nombre d'œufs, remarque), sans voir le classeur | Page pour téléphone, listes déroulantes, date du jour par défaut, réseau faible (§5.4), plus tard la position GPS (pas de photo : hors du périmètre) |
| **Demandes internes** | Demandes d'achat, de congés, signalements d'incident, tickets informatiques | Répondant identifié (connexion), colonne « Statut » remplie par l'équipe dans le classeur, réponse modifiable tant qu'elle n'est pas traitée |
| **Inscriptions** | Sortie nature, formation, repas associatif | Lien public, date de fermeture, nombre de places (plafond), une inscription par personne |
| **Collecte ponctuelle** | Inventaire par plusieurs sites, sondage d'équipe | Formulaire en pause ou fermé à la fin, statistiques |

Ce qu'on ne fait pas : un outil de sondage complet (logique de branchement complexe, scores, quiz). Un formulaire Lattis est une **porte d'entrée vers un tableau** : une réponse donne une ligne, un champ donne une colonne.

## 2. Point d'entrée et conditions

### 2.1 Où le trouver

- **Clic droit sur un tableau › « Partager comme formulaire… »**, à côté de « Ajouter au classeur partagé… » et de « Partager au Data Lake Lattis… » (`Sheet.svelte`).
- Clic droit › Tableau › **Formulaires (2) ›** : les formulaires du tableau (§7). Ce sous-menu n'apparaît que s'il en existe.
- Fichier › Gérer l'accès… › nouvel onglet **Formulaires**, qui liste tous les formulaires du classeur.
- La palette de commandes. Plus tard, l'API (`forms.*` pour Lua, le MCP et l'Assistant ✦).

### 2.2 Conditions et messages

L'élément de menu est **toujours visible**. Quand une condition manque, il ouvre un message qui propose l'action à faire, au lieu d'être grisé sans explication.

| Situation | Message | Action proposée |
|---|---|---|
| Classeur local (fichier, NAS) | « Ce tableau n'est pas partagé. Un formulaire est servi par un Serveur Lattis : publiez d'abord le classeur en mode collaboratif. » | **Publier sur un serveur…** ; à la fin de la publication, le concepteur s'ouvre de lui-même |
| Tableau privé d'un classeur collaboratif | « Ce tableau n'est que sur votre poste. Ajoutez-le au classeur partagé pour en faire un formulaire. » | **Ajouter au classeur partagé…**, puis on enchaîne |
| Classeur figé | « Un classeur figé ne reçoit pas de lignes. Les formulaires demandent un classeur collaboratif. » | Aucune (question 9) |
| Tableau chargé par une requête, tableau lié du Data Lake, source poussée (pipeline-table-source) | « Ce tableau est rempli par une requête / une donnée liée / l'envoi de votre classeur : ses lignes seraient remplacées. » | Aucune |
| Hors ligne (décision 0014) | « Créer ou modifier un formulaire demande le serveur. » | Réessayer |
| Ni propriétaire ni concepteur (`can_design` faux) | « Réservé aux propriétaires et concepteurs du classeur. » Un membre peut quand même voir les formulaires qui lui sont ouverts et **copier leur lien** | — |
| Serveur sans les formulaires (`42883`, ou `forms` absent de la découverte) | « Ce serveur ne propose pas encore les formulaires. » | — |

La coédition sur un NAS (décision 0011) n'est pas concernée : sans serveur, il n'y a personne pour servir la page. Elle tombe dans le cas « classeur local ».

## 3. Le concepteur de formulaire

Une fenêtre en deux parties. À gauche, les réglages en trois onglets : **Champs**, **Apparence** (§4) et **Accès** (§5). À droite, l'**aperçu** en largeur de téléphone (§3.5). En bas, les boutons **Enregistrer le brouillon** et **Ouvrir le formulaire** (vert).

### 3.1 Champs

Il y a une ligne par colonne publiée, dans l'ordre des colonnes. Chaque ligne a : une case « dans le formulaire », une poignée pour la déplacer, le libellé (par défaut le nom de la colonne), un texte d'aide, un texte indicatif (*placeholder*), « obligatoire », une valeur par défaut et le type de champ.

| Colonne (type du serveur) | Champ proposé | Réglages |
|---|---|---|
| `text` | Ligne de texte ; **texte long** (zone de saisie) au choix, proposé d'office si les valeurs existantes dépassent en moyenne 80 caractères | Longueur maximale (10 000 au plus) ; format : libre, e-mail, téléphone, adresse web |
| `integer`, `decimal` | Nombre (clavier numérique sur téléphone) | Minimum, maximum, pas ; unité tirée du format de nombre (€, %) et affichée après le champ |
| `date` | Sélecteur de date du navigateur | Bornes ; valeur par défaut « aujourd'hui » |
| `datetime` | Date et heure | Valeur par défaut « maintenant » |
| `boolean` | Case à cocher, ou Oui / Non | « Obligatoire » veut alors dire « doit être cochée » (accord, consentement) |
| Colonne avec **liste de validation** (`dv`) | Boutons radio jusqu'à 7 valeurs, liste avec recherche au-delà | Voir ci-dessous |

**Listes.** Une liste de valeurs fixes (`["Oui", "Non"]`) est copiée dans la définition. Pour une liste **tirée d'une plage** (`{"src": …}`, `validation.go`), il y a deux cas :

- La source est une **colonne d'un tableau publié du même classeur** (`Espèces[Nom]`) : le serveur relit les valeurs à chaque affichage et à chaque envoi, avec au plus 1 000 valeurs comme `MaxValidationValues`. La liste est donc toujours à jour.
- Sinon (cellules libres, tableau privé, cellule nommée) : la liste est **copiée à l'enregistrement**. Le concepteur voit « Liste copiée le 19/10 » et un bouton **Mettre à jour**. Ouvrir le classeur rappelle de la mettre à jour quand la source locale a changé.

Une valeur envoyée doit appartenir à la liste (la comparaison ignore la casse). Le serveur enregistre la forme canonique de la liste.

**Colonnes qui ne sont pas des champs**

- Colonnes calculées : elles ne sont pas publiées, donc elles n'existent pas sur le serveur. Elles se calculent dans l'application à l'arrivée de la ligne.
- **Clé à numérotation automatique** (décision 0010, clé d'entiers) : elle est absente du formulaire. Le serveur pose `max + 1` dans la transaction d'envoi, sous un verrou sur le tableau. Deux réponses simultanées ne peuvent donc pas recevoir le même numéro. C'est plus sûr que le calcul sur le poste.
- **Clé saisie** (texte, code) : le champ est obligatoire et l'unicité est contrôlée à l'envoi (« Cette valeur existe déjà »), sans jamais montrer les autres valeurs.
- **Valeurs posées par le serveur**, sans champ visible, chacune sur une colonne choisie :
  - « Reçu le » (date et heure) ;
  - « Répondant » (adresse ou nom affiché, en mode connecté seulement) ;
  - « Formulaire » (son titre, utile s'il y a plusieurs formulaires sur un tableau) ;
  - une **valeur fixe** (« Source = Terrain »).

  Si la colonne n'existe pas encore, le concepteur propose « Ajouter une colonne Reçu le ». L'ajout passe par les opérations de structure existantes (`synchronisation.md` §2.9).
- Une colonne laissée hors du formulaire reçoit `NULL`, ou sa valeur par défaut.

### 3.2 Valeurs par défaut

Une valeur par défaut est soit une valeur fixe, soit l'un de quelques **jetons** : aujourd'hui, maintenant, année en cours, adresse du répondant. Les formules ne sont pas permises : le serveur ne fait pas tourner le moteur de calcul. Une valeur par défaut est pré-remplie et modifiable, sauf si le champ est marqué « caché ».

### 3.3 Onglet à choix

Si le tableau a la colonne de catégorie d'un onglet à choix (Année, Mois…, voir `regrouper-onglets-semblables.md` §6.3), le concepteur propose deux options :

- **une valeur fixe**, par défaut celle affichée par l'onglet (2026) ;
- **un champ**, sous forme de liste tirée de la validation de la colonne.

Avec une valeur fixe, « + Nouvelle année… » le rappelle : « Le formulaire « Nids » pose toujours Année = 2026 : passer à 2027 ? ». Le jeton « année en cours » évite cette question pour une catégorie d'années.

### 3.3 bis Pré-remplissage par l'adresse (dès le lot 1, décision de Louis)

Un champ peut être pré-rempli par un paramètre de l'adresse : `/f/<code>?site=Nord` (nom du paramètre = nom court du champ, choisi dans le concepteur). Usage : un QR code par site, chacun avec sa valeur. Le concepteur génère ces liens et leurs QR codes (« Lien pré-rempli… » : choisir les champs et leurs valeurs). Côté serveur, `GET /f/{code}` lit les paramètres connus, ignore les autres, et la valeur reste modifiable par le répondant sauf si le champ est marqué « verrouillé par le lien » (alors affichée en lecture seule et revérifiée par `submit_form`). Les valeurs passent par les mêmes contrôles que la saisie (type, liste, bornes).

### 3.4 Plus tard

- **Champs conditionnels** : « afficher si Espèce = Autre ».
- **Position GPS** : remplit deux colonnes Latitude / Longitude à partir du navigateur.
- **Photo ou fichier** : **hors du périmètre** (décision de Louis, 19/10) : pas de champ photo ni fichier, ni maintenant ni dans un lot ultérieur prévu.

### 3.5 Aperçu et vérifications

L'aperçu de droite est un rendu Svelte approché de la page, mis à jour à chaque réglage, avec une bascule téléphone / ordinateur. Pour le rendu exact, **Ouvrir l'aperçu dans le navigateur** enregistre le brouillon et ouvre `form_preview_url` : un lien signé, valable 10 minutes, qui montre la vraie page du serveur et n'enregistre pas les réponses de test.

Avant l'ouverture, les vérifications sont listées en orange, comme dans la publication :

- liste vide ;
- valeur par défaut hors liste ou hors bornes ;
- champ obligatoire sur une colonne qu'une restriction interdit d'écrire ;
- valeurs d'une liste qui seront **visibles par tous les répondants** (§9) ;
- lien public sans date de fermeture ni plafond.

## 4. Personnalisation (demande de Louis)

Le but est que chaque formulaire ait l'air de venir de son auteur, sans ouvrir la porte au code. Tout se règle dans l'onglet **Apparence** et l'aperçu suit à chaque changement.

### 4.1 Ce qui se personnalise

| Élément | Réglage | Limites |
|---|---|---|
| **Titre** | Texte, par défaut le nom du tableau | 120 caractères |
| **Introduction** | Texte simple avec une mise en forme réduite : gras, italique, liens, listes à puces, retours à la ligne. Les liens s'ouvrent dans un nouvel onglet | 2 000 caractères ; ni titres, ni images, ni HTML |
| **Logo** | Une image PNG, JPEG ou SVG nettoyé, affichée en haut à gauche ; par défaut, celui du serveur (découverte `logo`) | 200 Ko, recadré à 64 px de haut |
| **Bannière** | Une image en largeur au-dessus du titre (photo d'un nid, affiche de l'évènement) | 1 Mo ; le serveur la réduit (1 600 px de large) et retire ses métadonnées (EXIF : position GPS de la photo) |
| **Couleur d'accent** | Choisie dans une **palette de 8 couleurs** dont le contraste a été vérifié (AA, 4,5:1 sur blanc pour le bouton et les liens, et en thème sombre) ; pas de couleur libre au premier lot | — |
| **Champs** | Libellé, texte d'aide, texte indicatif, ordre, obligatoire (§3.1) | Libellé 200 caractères, aide 500 caractères |
| **Sections et pages** | Un **intertitre** (texte et courte description) se glisse entre les champs. Avec l'option « Une section par page », chaque section devient une page avec Suivant / Précédent et une barre d'avancement (pratique sur téléphone pour un long relevé) | 20 sections |
| **Bouton** | Libellé du bouton Envoyer (« S'inscrire », « Enregistrer l'observation ») | 40 caractères |
| **Remerciement** | Message après l'envoi (même mise en forme que l'introduction), bouton « Envoyer une autre réponse » (oui ou non), **lien de redirection** facultatif (« Retour au site de l'association ») | Adresse `https://` seulement ; affichée en lien, la page ne redirige pas toute seule |
| **Formulaire fermé** | Message affiché quand le formulaire est fermé, plein ou pas encore ouvert | 500 caractères |
| **Mention de protection des données** | Texte affiché au-dessus du bouton, avec un modèle pré-rempli (§9) | 1 000 caractères |
| **Langue de la page** | Français ou anglais pour tout le texte fourni par le serveur : « obligatoire », messages d'erreur, boutons Suivant et Précédent, dates. Le texte de l'auteur reste celui qu'il a écrit | `fr`, `en` (les autres langues suivront `docs/i18n.md`) |

Le concepteur peut **reprendre l'apparence d'un autre formulaire** du serveur (menu « Copier l'apparence de… ») pour garder la même allure d'un formulaire à l'autre.

### 4.2 Ce qui ne se personnalise pas (premier lot)

- **Ni CSS, ni JavaScript, ni HTML libre.** La page est servie par le domaine du serveur, qui porte aussi la connexion des membres : un script ou une feuille de style libre permettrait de voler une session, de collecter des données ailleurs ou d'imiter la page de connexion. La politique de sécurité de la page (CSP) interdit d'ailleurs tout script ou style extérieur.
- Pas de police libre, pas de mise en page libre (colonnes, champs côte à côte), pas de nom de domaine propre.
- Pas de couleur libre : la palette garantit un contraste lisible. On pourra l'ouvrir plus tard avec un contrôle de contraste à la saisie.
- Le pied de page « Formulaire servi par <serveur> · Lattis » reste en place, pour que le répondant sache où vont ses données.

### 4.3 Où c'est rangé

Tout ce qui précède est dans la définition JSON du formulaire, gardée par le serveur (§10.2, clé `appearance` et sections dans `fields`). Les images sont envoyées une fois au serveur (`POST /api/forms/<id>/images`) et la définition ne garde que leur identifiant. L'aperçu de l'application lit la même définition : ce qu'on voit à droite est ce que le serveur rendra.

## 5. Accès

| Mode | Qui répond | Connexion | Remarques |
|---|---|---|---|
| **Membres du classeur** (par défaut) | Toute personne qui a un rôle sur le classeur, **lecteurs compris** (question 8) | Compte Lattis, par la même session de site que `/w/<id>` | Le formulaire est un droit à part, que le concepteur choisit de donner |
| **Personnes et équipes choisies** | Adresses (une adresse inconnue reçoit un compte en attente, comme `invite`) et équipes (`list_teams`, sous-équipes comprises) | Compte Lattis | Elles **ne deviennent pas membres** du classeur et ne le voient pas dans `my_workbooks` |
| **Lien public** | Toute personne qui a le lien | Aucune | Proposé seulement si le serveur le permet (réglage `forms_public` du propriétaire du serveur, **désactivé par défaut**) |

**Position du produit sur le lien public.** Lattis est d'abord un outil interne. Un lien public met une page du serveur sur internet et laisse entrer des données que personne n'a vérifiées. On le propose parce que les inscriptions et les relevés par des bénévoles en ont besoin, mais sur décision explicite de deux personnes : le propriétaire du serveur l'autorise, puis le concepteur le choisit pour son formulaire. Les protections sont toujours actives et ne se désactivent pas :

- **Limites de débit** : par adresse IP (10 envois par minute), par formulaire (1 000 par jour par défaut, réglable) et par compte. Elles s'appuient sur le seau à jetons existant (`limiter.go`).
- **Pot de miel** : un champ caché qu'un humain ne remplit pas. S'y ajoutent un **délai minimal** de 3 s entre l'affichage et l'envoi, mesuré par un jeton signé, et une taille de corps de 64 Kio au plus.
- **Pas de CAPTCHA tiers** : ni Google ni hCaptcha, qui posent des problèmes de RGPD et d'accessibilité. Une preuve de travail légère pourra venir plus tard si les abus l'exigent (question 4).
- **Mise en pause automatique** en cas de pic (plus de 5 fois le débit habituel pendant 10 min). Le formulaire passe en pause, le classeur reçoit `*form:<id>` et l'application le signale aux concepteurs.

**Options communes**

- **Dates** d'ouverture et de fermeture.
- **Plafond** de réponses (« 30 places »).
- **Une réponse par personne** : sûre en mode connecté ; en mode public, seulement par un cookie, ce que le concepteur voit écrit.
- **Modification de sa réponse** (question 5) :
  - en mode connecté, par « Mes réponses », jusqu'à la fermeture ou pendant N jours ;
  - en mode public, par un **lien secret** affiché après l'envoi, valable 7 jours.

  Une ligne déjà modifiée dans le classeur par un membre n'est plus modifiable par le répondant : on compare `lattis_updated_by` à l'identité du formulaire.

## 6. Ce que voit le répondant

### 6.1 La page

- **Adresse** : `<issuer>/f/<code>`, où le code fait 10 caractères aléatoires en base 32. Ce n'est pas l'identifiant du classeur, pour ne rien révéler de la structure du serveur.
- Elle est servie par lattisd, en HTML simple, d'abord pour téléphone, **sans le moteur de calcul**. L'envoi marche sans JavaScript (formulaire HTML classique, validation par le serveur). Le JavaScript ajoute la validation au fil de la saisie, les pages de sections et le brouillon local.
- En mode connecté, la page passe par la connexion du site (`/admin/login?next=/f/<code>`, comme `/w/<id>`) puis revient au formulaire.
- **Contenu** : en-tête personnalisé (§4), champs, mention « * obligatoire », bouton. Les erreurs s'affichent sous chaque champ et les valeurs saisies sont gardées.

### 6.2 Après l'envoi

- Le message de remerciement s'affiche avec un récapitulatif de la réponse.
- Selon les réglages : « Envoyer une autre réponse » et « Modifier ma réponse ».
- En mode connecté, **Mes réponses** liste ses envois à ce formulaire. C'est la seule partie du tableau que le répondant voit jamais.

### 6.3 Accessibilité

Libellés liés aux champs, ordre du focus, contraste de la palette (§4.1), messages d'erreur annoncés aux lecteurs d'écran.

### 6.4 Réseau faible (relevés de terrain)

- **Premier lot** : la saisie en cours est gardée dans le navigateur (`localStorage`, par formulaire), donc un rechargement ou une coupure ne la perd pas. Si l'envoi échoue, le message dit « Pas de réseau : votre réponse est gardée sur ce téléphone » avec **Réessayer**.
- **Plus tard** (question 6) : une vraie application hors ligne (*service worker*) qui accumule plusieurs réponses et les envoie au retour du réseau.

## 7. Gestion dans l'application

- **Sous-menu du tableau « Formulaires (n) »** : pour chaque formulaire, son état (brouillon, ouvert, en pause, fermé) et les actions **Copier le lien**, **QR code…**, **Modifier…**, **Mettre en pause / Reprendre**, **Fermer** et **Supprimer…**. S'y ajoute **Nouveau formulaire…** (plusieurs formulaires par tableau, question 10).
- **Pastille sur le cadre du tableau** (comme l'icône d'un tableau privé) quand un formulaire est ouvert. Son infobulle donne « Formulaire « Nids 2026 » ouvert · 42 réponses · dernière il y a 3 min ».
- **Onglet Formulaires de « Gérer l'accès »** : un tableau avec tableau, titre, mode, état, réponses (total et 7 derniers jours), dernière réponse, lien. Un clic sur une ligne ouvre une petite courbe des réponses par jour (`form_stats`).
- **QR code** : généré sur le poste, enregistrable en PNG ou SVG. **Imprimer une affiche** donne le titre, la bannière et le QR code, en passant par la mise en page d'impression existante.
- **Nouvelles réponses** : quand une ligne arrive d'un formulaire, la barre d'état le dit (« 3 nouvelles réponses au formulaire Nids »). L'auteur d'une ligne (« Formulaire : Nids 2026 ») se lit déjà au survol de la marge.
- **Supprimer** un formulaire : confirmation, puis le lien répond « Ce formulaire n'existe plus ». **Les lignes restent.** L'identité technique est désactivée, mais elle reste nommée dans `lattis_author`, comme pour `drop_pipeline`.
- Ces actions sont des opérations du serveur : elles **ne sont pas dans l'historique d'annulation**, comme la gestion des droits. Les actions destructrices demandent une confirmation.

## 8. Interactions avec le reste

| Avec | Comportement |
|---|---|
| **Renommer une colonne** | Le formulaire suit : la définition cite les colonnes par leur `column_id` du serveur, pas par leur nom. Le libellé du champ ne change pas |
| **Supprimer une colonne** | L'application prévient avant : « Espèce est un champ du formulaire Nids ». Le serveur retire le champ ; si le champ était obligatoire ou portait une valeur posée par le serveur, il **met le formulaire en pause** et envoie `*form:<id>` |
| **Ajouter une colonne** | Elle n'entre pas dans le formulaire. La barre d'état propose « L'ajouter au formulaire Nids ? » |
| **Convertir le type d'une colonne** | Le champ suit le nouveau type. Une valeur par défaut ou des bornes devenues incompatibles mettent le formulaire en pause |
| **Clé posée ou retirée** | La numérotation automatique et le contrôle d'unicité suivent |
| **Renommer ou supprimer le tableau** | Le renommage ne change rien. La suppression supprime ses formulaires : la confirmation de suppression les nomme |
| **Droits** | L'identité du formulaire a son propre rôle interne (insertion seulement). Une restriction « tous les rôles » d'insertion (« Ajout interdit ») bloque aussi le formulaire : c'est voulu, le concepteur en est averti (§3.5). Les filtres des autres rôles ne s'appliquent pas à lui |
| **Onglet à choix** | Voir §3.3 |
| **Colonnes saisies à côté d'une requête** (décision 0008), TCD, graphiques, formules | Un tableau chargé par une requête est refusé (§2.2). Le reste se recalcule à l'arrivée de la ligne, comme pour une saisie d'un collègue |
| **Validation dans l'application** | Une ligne arrivée hors d'une liste (liste changée entre-temps) est marquée comme toute saisie invalide |
| **Hors ligne** (décision 0014) | La gestion demande le serveur. Les réponses continuent d'arriver sur le serveur et le poste les reçoit à son retour, par les deltas |
| **Copie locale, Détacher du serveur, `drop_workbook`** | Les formulaires ne sont pas copiés. Détacher ou supprimer le classeur supprime ses formulaires : la confirmation le dit |
| **Data Lake** | Une donnée tirée du tableau contient les réponses comme toutes les autres lignes |
| **Coédition sur un NAS** (décision 0011) | Pas de formulaire (§2.2) |
| **Version web** | Mêmes dialogues : le concepteur est dans le frontend |

## 9. Sécurité et vie privée

- **Ce que le répondant peut lire** : la définition (titres, libellés, aides, apparence), les valeurs des listes, et ses propres réponses. Rien d'autre. Le refus d'une clé en double ne renvoie aucune donnée, et ces refus comptent dans la limite de débit (pour empêcher de deviner les valeurs une à une). Le concepteur est prévenu quand une liste tirée d'un tableau expose ses valeurs, surtout en mode public.
- **Une seule porte d'écriture** : `lattis.submit_form`, appelée par lattisd. Les noms de colonnes viennent du catalogue du serveur, jamais de la requête, et les valeurs passent en paramètres. L'identité technique n'a que le droit d'insérer dans ce tableau, et la RLS et les restrictions s'appliquent à elle comme à tout rôle.
- **Valeurs** :
  - longueur bornée, caractères de contrôle retirés ;
  - une valeur qui commence par `=` est gardée **comme texte**. L'application ne la prend jamais pour une formule, mais il faut vérifier l'échappement à l'export CSV et xlsx (injection de formule dans Excel) ;
  - les dates et les nombres sont convertis comme à la publication.
- **Page** : tout est échappé, politique de sécurité (CSP) stricte sans ressource extérieure, jeton anti-CSRF signé, cookie de session `SameSite`.
- **RGPD** : le concepteur est responsable du traitement, et le modèle de mention le rappelle (qui collecte, pourquoi, combien de temps, contact).
  - L'adresse IP n'est gardée que tronquée et hachée, 30 jours, pour les limites de débit.
  - Le journal des réponses nomme le répondant connecté.
  - Effacer une réponse revient à supprimer la ligne dans le classeur : l'entrée du journal est purgée avec la pierre tombale.
- **Audit** : création, modification, changement d'état et suppression d'un formulaire vont dans `lattis.audit`. Les réponses vont dans `lattis.form_submissions` (trop nombreuses pour l'audit).

## 10. Ce que le serveur doit fournir

*À relayer au fil du serveur. Noms indicatifs ; codes d'erreur dans le style du contrat client.*

### 10.1 Tables et réglages

- `lattis.forms` : `id uuid`, `code text UNIQUE` (10 caractères en base 32), `workbook_id`, `table_id`, `identity uuid` (utilisateur technique « Formulaire : <titre> », `form+<id>@lattis.invalid`, membre d'un rôle interne du classeur qui a le seul droit d'insérer dans le tableau ; il ne se connecte jamais), `state` (`draft`, `open`, `paused`, `closed`), `pause_reason`, `access` (`members`, `audience`, `public`), `definition jsonb`, `opens_at`, `closes_at`, `max_responses`, `one_per_person`, `edit` (`none`, `until_close`, ou durée), `version int`, `created_by`, `created_at`, `updated_at`. `ON DELETE CASCADE` depuis le classeur.
- `lattis.form_audience (form_id, principal)` : personne ou équipe, sous-équipes comprises, sur le modèle de `add_data_audience`.
- `lattis.form_submissions` : `id`, `form_id`, `row_id` (`lattis_id`), `respondent` (utilisateur, ou `NULL` en mode public), `edit_token_hash`, `submitted_at`, `updated_at`, `client_hash` (IP tronquée et hachée, purgée à 30 jours).
- `lattis.form_images` : `id`, `form_id`, `kind` (`logo`, `banner`), `content_type`, `data bytea` (ou un fichier du dossier de données), taille bornée.
- Réglages du serveur : `forms_public` (faux par défaut, page Serveur de l'administration), limites (`LATTIS_FORMS_RATE_IP`, `LATTIS_FORMS_DAILY`, taille de corps). Découverte : `"forms": true, "forms_public": false`.

### 10.2 Définition (`definition jsonb`)

```json
{
  "format": 1,
  "lang": "fr",
  "appearance": {
    "title": "Relevé des nids 2026",
    "intro": "Une ligne **par nid**. [Guide](https://…)",
    "logo": "<image_id>", "banner": "<image_id>",
    "accent": "teal",
    "submit_label": "Enregistrer l'observation",
    "thanks": "Merci !", "thanks_link": {"label": "Retour au site", "url": "https://…"},
    "another": true, "closed_message": "…", "privacy": "…",
    "paged": false
  },
  "fields": [
    {"kind": "section", "title": "Où ?", "description": "…"},
    {"column": "<column_id>", "label": "Commune", "help": "…", "placeholder": "…",
     "required": true, "widget": "list", "default": {"token": "today"},
     "min": null, "max": null, "step": null, "max_length": 200, "format": "email",
     "list": {"values": ["…"]} | {"table_id": "…", "column": "<column_id>"} | null}
  ],
  "server_values": [
    {"column": "<column_id>", "value": {"token": "received_at" | "respondent" | "form_title" | "next_number"} | {"fixed": "Terrain"}}
  ]
}
```

`intro`, `thanks`, `privacy` et les descriptions de sections sont en Markdown réduit (gras, italique, liens `https`, listes). Le serveur le convertit en HTML échappé ; tout autre balisage reste du texte. `accent` est un nom de la palette, pas une couleur libre. Une définition invalide est refusée en `22023`, avec la raison (champ et règle).

### 10.3 Fonctions SQL (schéma `lattis`, sous l'identité de l'utilisateur)

| Fonction | Droits | Erreurs |
|---|---|---|
| `create_form(workbook_id, table_id, p jsonb) → {"id", "code", "url"}` : `p` = `definition`, `access`, `audience`, options ; crée l'identité technique et le rôle interne ; état `draft` | Propriétaire ou concepteur du classeur | `42501` ; `55000` classeur figé ou tableau non publié ; `P0002` tableau inconnu ; `22023` définition invalide ; `42501` mode public refusé par le serveur |
| `update_form(form_id, p jsonb, version) → {"version"}` : champs facultatifs | idem | `40001` « formulaire modifié entre-temps » (version différente) ; `22023` |
| `set_form_state(form_id, 'open'\|'paused'\|'closed')` : `open` revérifie la définition sur la structure actuelle | idem | `22023` avec les champs en cause |
| `drop_form(form_id)` : identité désactivée, lignes gardées | idem | `P0002` |
| `list_forms(workbook_id) → jsonb` : `[{"id", "code", "url", "table_id", "title", "state", "pause_reason", "access", "responses", "last_response_at", "manageable"}]` | Propriétaires et concepteurs : tout ; autres membres : formulaires ouverts qui leur sont ouverts, sans les chiffres | `42501` |
| `form_stats(form_id, days int DEFAULT 30) → {"total", "by_day": [{"day", "n"}], "last_at", "throttled"}` | Propriétaire ou concepteur | `42501`, `P0002` |
| `form_preview_url(form_id) → text` : lien signé valable 10 min, réponses non enregistrées | idem | — |
| `form_audience_add(form_id, principal)` / `form_audience_add_by_email(form_id, email)` / `form_audience_remove(form_id, principal)` | idem | `P0002` adresse sans compte (ou compte en attente, à trancher) |
| `submit_form(form_id, values jsonb, respondent jsonb) → {"row_id", "edit_token"}` : **réservée à lattisd** (`SECURITY DEFINER`, `EXECUTE` retiré à `PUBLIC`) ; contrôle l'état, l'audience, les dates, le plafond, une réponse par personne, les types, les listes, les bornes, les champs obligatoires et la clé ; pose les valeurs du serveur, le numéro (`max + 1` sous verrou) et `lattis_pos` (à la fin) ; insère sous l'identité du formulaire (RLS, déclencheurs, `NOTIFY` du tableau) ; écrit `form_submissions` | lattisd | `22023` avec `DETAIL` = `{"fields": {"<column_id>": "raison"}}` ; `23505` clé déjà prise ; `55000` fermé, pas encore ouvert ou plein ; `42501` hors audience ; `P0001` déjà répondu |
| `update_submission(form_id, row_id \| edit_token, values jsonb)` : mêmes contrôles, dans la fenêtre de modification, `UPDATE` conditionné à `lattis_updated_by` = identité du formulaire | lattisd | idem, plus `55000` « modifiée depuis par l'équipe » |
| `my_form_submissions(form_id) → jsonb` : réponses du répondant connecté | lattisd | — |
| `form_page(code) → jsonb` : définition publique et listes résolues | lattisd | `P0002` (inconnu et interdit donnent la même réponse) |

**Suivi de la structure** : `rename_column` n'a rien à faire (la définition cite les `column_id`). `drop_column`, `alter_column_type`, la clé et `drop_table` doivent mettre à jour les définitions, mettre en pause quand c'est nécessaire (`pause_reason`) et envoyer `*form:<id>`.

**Notification** : `*form:<id>` sur le canal du classeur quand un formulaire est créé, modifié, change d'état (pause automatique comprise) ou est supprimé. Les réponses ne notifient que l'identifiant du tableau, comme toute écriture.

### 10.4 Points d'accès HTTP

| Point | Rôle | Réponses |
|---|---|---|
| `GET /f/{code}` | Page du formulaire ; connexion si le mode le demande ; `?p=<jeton>` pour l'aperçu | `200` ; `404` page unique pour un formulaire inconnu, interdit ou supprimé ; `410` fermé, plein ou pas encore ouvert (message de l'auteur, titre visible seulement s'il est accessible) |
| `POST /f/{code}` | Envoi : `application/x-www-form-urlencoded` ou JSON, 64 Kio au plus, jeton anti-CSRF et minuterie, pot de miel | `303` vers `/f/{code}/merci` ; `400` page rendue avec les erreurs ; `409` clé prise ou déjà répondu ; `410` ; `429` avec `Retry-After` |
| `GET /f/{code}/merci`, `GET /f/{code}/mes-reponses`, `GET` et `POST /f/{code}/r/{token}` | Remerciement, réponses du répondant connecté, modification par lien secret | `404`, `410` |
| `GET /f/{code}/img/{image_id}` | Logo et bannière, avec un cache long | `404` |
| `POST /api/forms/{id}/images` | Envoi d'une image par l'application (`Authorization: Bearer`, comme `/api/preview`) ; réduction et retrait des métadonnées | `401`, `403`, `413`, `415` |

**Administration** : une page « Formulaires » pour le propriétaire du serveur (tous les formulaires, mode, débit ; mise en pause de n'importe lequel), et le réglage `forms_public`. **Tâche de fond** : purge de `client_hash` et des jetons de modification expirés, fermeture à `closes_at`.

## 11. Découpage côté client

| Lot | Contenu | Taille | Risque |
|---|---|---|---|
| 1 | Point d'entrée et messages du §2.2 ; liens pré-remplis par l'adresse et leurs QR codes (§3.3 bis) ; interface `workbook.Forms` (pgx et faux serveur `workbooktest/forms.go`) ; concepteur : champs, correspondance des types, listes fixes et tirées d'une plage, clé automatique, valeurs posées par le serveur, onglet à choix ; personnalisation du §4 (apparence, sections, palette, images) ; aperçu Svelte et aperçu dans le navigateur ; modes membres et personnes ou équipes choisies ; création, ouverture, copie du lien, QR code | **M** | Moyen : il dépend du serveur (§10) ; l'aperçu approché doit rester proche de la page du serveur (on lui donne une seule source : la définition) |
| 2 | Gestion : sous-menu, pastille, onglet Formulaires, statistiques, pause, fermeture et suppression, suivi de `*form:<id>`, avertissements avant suppression ou conversion de colonne, barre d'état « nouvelles réponses », affiche à imprimer | **S** | Faible |
| 3 | Lien public (selon `forms_public`), dates, plafond, une réponse par personne, modification des réponses, mention de protection des données, « Copier l'apparence de… » | **S** | Faible côté client ; le risque est côté serveur (abus) |
| 4 | Plus tard : champs conditionnels, position GPS, hors ligne complet sur le téléphone, notifications par e-mail, `forms.*` pour Lua et le MCP, couleur libre avec contrôle de contraste | **M + L** | Moyen : hors ligne |

**Essais** :

- faux serveur : création, refus, états, structure qui met en pause ;
- `bridge` : parcours du concepteur, messages du §2.2 ;
- frontend : `forms.test.ts` (correspondance des types, listes, sections, palette) ;
- contre PostgreSQL (`LATTIS_PG_TEST=1`) une fois le serveur prêt : une réponse envoyée par `submit_form` arrive dans le cache par les deltas, avec l'auteur « Formulaire : … ».

## 12. Questions pour Louis

1. **Auteur des lignes** : l'identité technique « Formulaire : <titre> », plus une colonne « Répondant » facultative ? Ou bien la personne elle-même, ce qui oblige à lui donner un rôle sur le classeur ? *Recommandé : identité technique.*
2. **Mode par défaut** : membres du classeur, personnes choisies ou public ? *Recommandé : membres du classeur.*
3. **Lien public dès le premier lot**, autorisé par le propriétaire du serveur et désactivé par défaut ? *Recommandé : oui (lot 3).*
4. **CAPTCHA** : pas de service tiers ; pot de miel, délai minimal, limites de débit et pause automatique suffisent-ils ? *Recommandé : oui.*
5. **Modifier sa réponse** : jusqu'à la fermeture en mode connecté, et 7 jours par lien secret en mode public ? *Recommandé : oui, désactivé par défaut pour chaque formulaire.*
6. **Hors ligne sur le terrain** : au premier lot, brouillon gardé sur le téléphone et Réessayer ; la file de plusieurs réponses hors ligne au lot 4 ? *Recommandé : oui.*
7. **Position GPS** pour les nids : au lot 4, ou dès le lot 2 (petite) ? *Recommandé : au lot 2.* (Photos et fichiers : hors du périmètre, décision de Louis.)
8. **Les lecteurs du classeur** peuvent-ils répondre en mode « membres » ? *Recommandé : oui, c'est le choix du concepteur.*
9. **Classeur figé** : on refuse simplement ? *Recommandé : oui.*
10. **Plusieurs formulaires par tableau** (une valeur fixe pour les distinguer) ? *Recommandé : oui.*
11. **Liste tirée de cellules libres ou d'un tableau privé** : copiée à l'enregistrement, avec « Mettre à jour » ? *Recommandé : oui (sinon, refuser ces sources).*
12. **Personnalisation** : la palette de 8 couleurs et l'absence de CSS ou de HTML libres conviennent-elles au premier lot ? *Recommandé : oui.*
13. **Langue de la page** : français et anglais au premier lot ? *Recommandé : oui.*

## 13. Proposition d'ajout au plan (non écrite)

En phase 9 :

> **Partager un tableau comme formulaire** (demande de Louis, 19/10/2026) : clic droit sur un tableau d'un classeur collaboratif › « Partager comme formulaire… » (sinon, message et action pour publier d'abord). Le concepteur fait un champ par colonne (types, listes de validation, clé numérotée par le serveur, valeurs posées par le serveur) et règle une personnalisation limitée (titre, introduction, logo, bannière, palette, sections, bouton, remerciement, langue). Trois modes d'accès : membres, personnes ou équipes, public avec protections. La page `/f/<code>` est servie par lattisd ; les réponses deviennent des lignes écrites sous une identité technique et arrivent en direct par la synchronisation. Étude : `conception/partager-comme-formulaire.md`. Client M + S + S ; serveur M à L.

## 14. Relecture du fil serveur (19/10) et choix retenus

- **Identité du formulaire** (Q1, d'accord) : `SECURITY DEFINER` ne peut pas faire `SET ROLE` ; lattisd se connecte donc sous l'identité du formulaire, comme les pipelines, et `submit_form` vérifie `session_user`.
- **Niveau interne « insertion seulement »** dans `compile_acl` : `INSERT`, plus `UPDATE` et `SELECT` limités par RLS sur `lattis_updated_by` pour `update_submission` (modifier sa réponse).
- **Colonnes citées par `column_id`** (choix côté appli) : `local_id` peut être nul pour une colonne créée côté serveur ; la définition cite donc l'identifiant de `lattis_meta_columns`. L'application connaît déjà la correspondance colonne locale ↔ `column_id` (publication et synchronisation). Les exemples du §10.2 sont à lire avec `column_id`.
- **Suppression d'un tableau** : `drop_table` supprime ses formulaires (réponses gardées dans le tableau jusqu'à sa suppression, journal `form_submissions` purgé) et envoie `*form:<id>` ; il n'est pas refusé comme pour une source du Data Lake ou un pipeline, puisque supprimer le tableau prive le formulaire de sa raison d'être. L'application prévient avant : « Ce tableau alimente N formulaires : ils seront supprimés ».
- **Numérotation** (`next_number`) : verrou consultatif par tableau, puis `max + 1`.
- **Pause automatique** : calculée en fenêtre glissante sur `form_submissions`, valable avec plusieurs lattisd.
- Estimation côté serveur : M à L.
