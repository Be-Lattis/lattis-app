# Compression des classeurs

*19/10/2026. Demandée par Louis : un classeur fermé est stocké compressé, avec « une compression très rapide, même si le taux est moyen ». Puis : « Zstandard me paraît prometteur pour la compression, tu le testes ? », et : « Le problème d'ouverture avec Windows n'est pas un problème. » Code : `internal/archive/compress.go`, `archive.go` (`writeZip`, `bomb`), `salvage.go` ; essais : `compress_test.go`, `robust_test.go`.*

## Le problème

Jusqu'ici la base était rangée dans le ZIP sans compression (méthode `Store`), pour que l'enregistrement d'un gros classeur reste rapide. Une base SQLite se compresse pourtant bien (2 à 9 fois) : un classeur de 100 000 lignes pesait 10 Mo au lieu de 4.

## La décision

**La base est compressée en Zstandard (méthode ZIP 93), au niveau le plus rapide, coupée en blocs de 2 Mio compressés en parallèle (une trame par bloc).** Le manifeste est en Deflate. Les archives `Store` (classeurs d'avant) et Deflate se lisent toujours ; elles sont réenregistrées en Zstandard.

- **Format.** Des trames Zstandard qui se suivent forment un flux Zstandard ordinaire (RFC 8878, § 3.1). Go le lit une fois le décodeur déclaré ; 7-Zip et WinRAR aussi. L'Explorateur de Windows et l'Utilitaire d'archive de macOS ne lisent pas la méthode 93 : accepté (Louis). Le module `zipfile` de Python 3.14 s'arrête à la première trame (somme de contrôle fausse) ; `compression.zstd.decompress` sur les octets de l'entrée lit tout (`docs/format-lattis.md`).
- **Parallèle.** La compression d'une trame est indépendante des autres : jusqu'à 8 blocs à la fois (32 Mio de mémoire au plus), écrits dans l'ordre. Elle se fait pendant la lecture de l'instantané `VACUUM INTO`, qui reste l'essentiel du temps d'enregistrement.
- **Réglage.** `archive.Method` choisit la méthode (Zstd, Deflate, Store) ; aucun paramètre n'est exposé.

## Mesures

Base seule (instantané `VACUUM INTO`), machine à 16 cœurs, meilleur de trois essais : temps d'écriture du ZIP, taille, temps de lecture. Classeurs de démonstration (`--demo`) et un vrai classeur (fichier de suivi de nids, 0,6 Mo).

| Base | Méthode | Écriture | Taille | Taux | Lecture |
| --- | --- | --- | --- | --- | --- |
| 1M×10 (100 Mo) | Store (avant) | 122 ms | 100,1 Mo | 1,0× | 81 ms |
| | Deflate 1, bibliothèque standard | 1 022 ms | 44,6 Mo | 2,2× | 738 ms |
| | Deflate 1, klauspost | 691 ms | 44,7 Mo | 2,2× | 660 ms |
| | Deflate 1, klauspost, blocs parallèles | 121 ms | 44,8 Mo | 2,2× | 642 ms |
| | Zstd le plus rapide, un fil | 675 ms | 39,4 Mo | 2,5× | 335 ms |
| | Zstd le plus rapide, encodeur concurrent | 401 ms | 39,4 Mo | 2,5× | 124 ms |
| | Zstd par défaut, encodeur concurrent | 565 ms | 37,8 Mo | 2,6× | 149 ms |
| | Zstd meilleure compression, concurrent | 1 720 ms | 36,7 Mo | 2,7× | 178 ms |
| | **Zstd le plus rapide, trames de 2 Mio en parallèle (choisi)** | **≈ 120–175 ms** | **39,6 Mo** | **2,5×** | **117 ms** |
| 100K×100 (44,8 Mo) | Store | 53 ms | 44,8 Mo | 1,0× | 40 ms |
| | Deflate 1, blocs parallèles | 46 ms | 14,4 Mo | 3,1× | 228 ms |
| | Zstd par défaut, concurrent | 214 ms | 12,6 Mo | 3,6× | 74 ms |
| | **Zstd, trames parallèles** | **≈ 60–110 ms** | **12,8 Mo** | **3,5×** | **≈ 70 ms** |
| big (21,3 Mo) | Store | 27 ms | 21,3 Mo | 1,0× | 26 ms |
| | Deflate 1, blocs parallèles | 34 ms | 8,5 Mo | 2,5× | 141 ms |
| | **Zstd, trames parallèles** | **≈ 35–40 ms** | **7,6 Mo** | **2,8×** | **≈ 28 ms** |
| 100K×10 (10 Mo) | Store | 14 ms | 10,0 Mo | 1,0× | 15 ms |
| | Deflate 1, blocs parallèles | 18 ms | 4,3 Mo | 2,3× | 66 ms |
| | **Zstd, trames parallèles** | **≈ 40 ms** | **3,8 Mo** | **2,7×** | **≈ 35 ms** |
| med, vitrine (1 Mo) | Store | 2 ms | 0,98 Mo | 1,0× | 9 ms |
| | **Zstd, trames parallèles** | **4 ms** | **0,22 Mo** | **4,5×** | **9 ms** |
| vrai classeur (0,6 Mo) | Store | 1 ms | 0,64 Mo | 1,0× | 9 ms |
| | Deflate 1 | 3 ms | 0,09 Mo | 6,8× | 10 ms |
| | **Zstd, trames parallèles** | **2 ms** | **0,07 Mo** | **8,9×** | **10 ms** |

Les temps des trames parallèles varient d'une série à l'autre (machine chargée) ; à 1, 2, 4 ou 8 Mio par trame, le taux ne change qu'au dixième près.

De bout en bout (`TestMesureSauvegarde`, tableau de démonstration à 20 colonnes) :

| Lignes | Méthode | Enregistrer | Archive | Ouvrir |
| --- | --- | --- | --- | --- |
| 100 000 | Store | 127 ms | 17 Mo | 39 ms |
| | Deflate parallèle | 158 ms | 8 Mo | 156 ms |
| | **Zstd** | **135 ms** | **7 Mo** | **39 ms** |
| 1 000 000 | Store | 1,43 s | 177 Mo | 433 ms |
| | Deflate parallèle | 1,34 s | 82 Mo | 1,37 s |
| | **Zstd** | **1,43 s** | **73 Mo** | **327 ms** |

Zstandard en trames parallèles enregistre aussi vite que `Store`, donne le plus petit fichier des méthodes rapides et s'ouvre le plus vite (le décodage Deflate, sur un seul fil, quadruple le temps d'ouverture d'un gros classeur). Le Deflate parallèle aurait gardé un ZIP lisible partout ; ce n'est pas un critère.

## Bombe ZIP

Taux mesurés en Zstandard (Deflate) : vrais classeurs 2,5–9× ; un même texte sur 500 000 lignes 48× (30×) ; une base faite de blobs nuls, le pire cas, 814× (181×) : chaque page SQLite garde son en-tête. Une bombe faite de zéros dépasse 30 000× en Zstandard et approche 1 000× en Deflate (son maximum). Seuils, au-delà de 64 Mio décompressés : **2 000×** en Zstandard, **500×** en Deflate. La vérification de l'espace libre (taille annoncée) et celle des pages déclarées par l'en-tête SQLite ne changent pas. La fenêtre du décodeur Zstandard est bornée à 128 Mio.

## Récupération

Salvage lit une archive tronquée ou altérée en décompressant la base depuis son en-tête local jusqu'à la première erreur (Zstandard sur un seul fil, pour garder chaque bloc décodé avant la coupure ; Deflate ; ou tel quel en `Store`), en bornant la sortie au même taux que l'ouverture.
