# Importer des données et connexions

Données › Obtenir des données importe des données dans un tableau, à la manière de Power Query : chaque transformation devient une étape rejouable, et le tableau s'actualise à la demande.

## Sources

| Source | Détail |
| --- | --- |
| Fichier texte ou CSV | Séparateur au choix |
| Classeur Excel | Une feuille au choix |
| Fichier JSON, Fichier Parquet | |
| Base SQLite (fichier) | Une table ou une requête |
| PostgreSQL, MySQL / MariaDB, SQL Server | Par une connexion (voir plus bas) |
| Web (API JSON) | Une adresse qui renvoie du JSON |
| Tableau du classeur, Autre requête, Requête SQL du classeur | Pour transformer ce qui est déjà là |

## L'éditeur de requête

L'éditeur montre un aperçu des données et la liste des **étapes appliquées**. Les commandes de son ruban ajoutent une étape :

- **Colonnes** : choisir, supprimer, renommer, changer le type ;
- **Lignes** : filtrer, trier, supprimer les doublons, garder les premières lignes ;
- **Transformer** : fractionner, remplacer des valeurs, format (SUPPRESPACE, MAJUSCULE…), colonne personnalisée, colonne d'index, regrouper, pivoter, dépivoter ;
- **Combiner** : fusionner (joindre) ou ajouter les lignes d'une autre requête ou d'un tableau.

Un clic sur une colonne de l'aperçu la sélectionne ; `Ctrl+Z` retire la dernière étape. Le résultat se charge dans un tableau, ou reste une simple connexion.

## Requêtes et connexions

Le volet **Requêtes et connexions** (Données › Requêtes et connexions) liste les requêtes du classeur : modifier, actualiser, supprimer. `F5` dans un tableau chargé par une requête la relance ; `Ctrl+F5` ou Données › Actualiser tout relance toutes les requêtes puis recalcule les formules. Un nom tapé à droite de son en-tête ajoute une colonne saisie, gardée à l'actualisation (voir [Écrire à côté d'un résultat lié](sql)).

## Classeur reçu : accord des sources

Une requête est écrite dans le classeur. Pour qu'un classeur reçu ne puisse pas, à l'actualisation, joindre un serveur inconnu ou copier un fichier de votre poste, Lattis demande votre accord la première fois qu'une requête veut lire un fichier ou joindre un hôte (API web, serveur de base de données). L'accord vaut pour la session, ou pour ce classeur sur ce poste avec « Mémoriser ». Les sources d'une requête écrite dans l'éditeur, et les fichiers que vous y choisissez, sont accordés d'office. Les hôtes accordés se revoient dans l'onglet Script › Accès réseau.

## Connexions aux bases

Une **connexion** décrit un serveur (PostgreSQL, MySQL / MariaDB, SQL Server, ou une API web) : hôte, port, base, utilisateur. Elle est rangée dans le classeur et sert à plusieurs requêtes.

Les mots de passe ne sont jamais enregistrés dans le classeur : ils restent dans le trousseau du système, et Lattis les demande à la première actualisation. Le volet permet aussi de les oublier.

Dans une entreprise, des connexions peuvent être fournies par la configuration du poste ; elles apparaissent sans rien saisir.
