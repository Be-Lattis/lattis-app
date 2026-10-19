# Prototype de grille (spike 1.1) : mesures

Tableau de démonstration : 1 000 000 lignes × 20 colonnes (entiers, décimaux, textes, dates, booléen) dans une table SQLite `STRICT`, driver `ncruces/go-sqlite3`, WAL. Généré au premier lancement dans `os.UserCacheDir()/Lattis/prototype/demo.sqlite` (≈ 15 s, environ 300 Mo).

## Architecture mesurée

- **Rendu** : `<canvas>` limité à la zone visible, une zone de découpe par colonne, en-têtes et numéros de ligne figés. Éditeur `<input>` superposé à la cellule active.
- **Défilement** : zone native plafonnée à 8 M px et mise à l'échelle (1 M de lignes = 24 M px), molette gérée en pixels virtuels.
- **Données** : cache de blocs de 200 lignes côté front, 4 requêtes en vol au plus, blocs hors de la vue abandonnés avant d'être demandés. Côté Go, pagination par clé d'ordre `_pos` à partir d'une ancre relevée toutes les 1 000 lignes (`OFFSET` borné à 999).
- **Échange** : JSON, tableaux de tableaux (pas d'objets clé/valeur).
- **Écriture** : connexion d'écriture unique possédée par une goroutine (file), pool de lecture en `query_only`.

## Protocole

Bouton **Mesurer** dans l'application, ou mesure automatique :

```bash
LATTIS_BENCH=resultat.json ./bin/lattis
```

L'application s'ouvre, mesure, écrit le JSON et se ferme. La mesure enchaîne :
1. **défilement continu** de 10 lignes par image pendant 5 s (600 lignes/s à 60 Hz) : images par seconde, intervalle entre images, images > 33 ms, images où une ligne visible n'était pas encore chargée ;
2. **20 sauts aléatoires** dans le tableau (barre de défilement) : délai jusqu'à ce que toute la zone visible soit chargée ;
3. **20 saisies** de cellule : aller-retour front → Go → SQLite (commit) → front.

La CI exécute la même mesure sous WebKitGTK dans Xvfb (job `bench-linux`, artefact `bench-webkitgtk`). Le rendu y est logiciel : les chiffres sont pessimistes mais suivent les régressions.

## Résultats

| Mesure | Windows 10, WebView2 (Edge 155), écran 120 Hz | Linux WebKitGTK 6, CI Xvfb (rendu logiciel) |
|---|---|---|
| Défilement continu | 119 img/s, image p50 8,3 ms / p95 8,5 ms | 43 img/s, image p50 23 ms / p95 27 ms |
| Images > 33 ms sur 5 s | 1 sur 593 | 0 sur 214 |
| Images avec lignes non chargées | 0 | 0 |
| Saut aléatoire → données affichées | p50 17 ms / p95 25 ms | p50 50 ms / p95 67 ms |
| Saisie d'une cellule (aller-retour) | p50 2,5 ms / p95 3,3 ms | p50 1 ms / p95 14 ms |
| Dessin d'une image | p50 0,4 ms / p95 1,1 ms | p50 2 ms / p95 5 ms |
| Bloc de 200 lignes : aller-retour / dont Go | p50 6,8 ms / 1,6 ms | p50 25 ms / 1,6 ms |

Windows : mesuré le 15/10/2026 sur le poste de Louis. Linux : job `bench-linux` de la CI (runner Ubuntu 24.04, sans GPU ; WebKit arrondit ses minuteries à la milliseconde).

## Lecture

- Sous Windows, le critère de sortie du spike est atteint : défilement à la fréquence de l'écran, aucune ligne vide pendant un défilement rapide, saisie sans latence perceptible.
- Le dessin ne coûte presque rien (0,4 ms) : le canvas laisse une grosse marge pour la mise en forme, les bordures de tableaux et la mise en forme conditionnelle.
- Sur un bloc, l'IPC et le JSON coûtent environ 5 ms contre 1,6 ms pour SQLite : si un moteur web plus lent devient limitant, c'est le format d'échange qu'il faudra compacter (binaire), pas la requête.
- Sous **Linux (WebKitGTK)**, même en rendu logiciel sur un runner CI, aucune image ne dépasse 33 ms et aucune ligne visible ne reste vide pendant le défilement. Le défilement y est moins fluide (43 img/s) et l'IPC plus coûteuse (25 ms par bloc contre 1,6 ms de SQLite). Une mesure sur un vrai poste Linux avec GPU reste à faire pour clore M1.
