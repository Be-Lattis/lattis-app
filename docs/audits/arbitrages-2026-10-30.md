# Arbitrages après les audits du 30/10/2026

Tout ce qui pouvait se faire sans hésitation est fait et sur main. Restent les choix ci-dessous. Pour chacun, la recommandation est marquée **(reco)**. Les détails sont dans `docs/audits/audit-*-2026-10-30.md`.

## À faire par Louis (personne d'autre ne peut)

1. **Clé de signature des mises à jour** (avant la 0.5.1) : `go run ./cmd/lattis-updsign keygen`, puis `gh secret set UPDATE_SIGNING_KEY < lattis-update.key`, coller la clé publique dans `publishedKeys`, garder la clé privée hors ligne. Je peux préparer le commit de la clé publique dès qu'elle existe.
2. **Publier la 0.5.1-beta** : dire « publie » ici. Tout le travail des audits y entre.
3. **Passe de 30 min avec NVDA** sur les 10 tâches courantes (rôles et noms vérifiés seulement par l'arbre d'accessibilité).
4. **Ouvrir un .xlsx à la main** dans l'appli de bureau et la version web, l'enregistrer en .xlsx (Ctrl+S), le rouvrir.

## Choix produit

| Sujet | Options | Reco |
|---|---|---|
| Signature du code Windows | SignPath Foundation (gratuit, libre) ; Azure Trusted Signing (~120 $/an, éligibilité incertaine) ; certificat OV + HSM (170–650 €/an) | SignPath |
| macOS | Apple Developer 99 €/an, seulement si macOS compte | plus tard |
| Famille LAMBDA | garder Lua ; LAMBDA dans MAP/BYROW/BYCOL/MAKEARRAY (3–5 j) ; + REDUCE/SCAN/noms (4 j de plus) | MAP & co (3–5 j) |
| Expressions régulières | RE2 linéaire ; retour arrière avec délai | RE2 |
| RECHERCHEV réécrite à la saisie | garder (on voit INDEX/EQUIV) ; garder le texte tapé (~1 j) | garder |
| RECHERCHEH sur un tableau | refuser ; RECHERCHEV transposée | refuser |
| CHOISIRLIGNES hors plage | ligne de #VALEUR! ; une seule erreur (2 h) | ligne |
| Ouverture d'un classeur de 10 M de lignes (5–17 s) | garder ; réutiliser la base extraite si l'archive n'a pas changé (~1 j) ; vue pendant l'extraction (~2 j) | réutiliser (~1 j) |
| Pilote SQLite natif (cgo) | garder WASM ; passer au natif | garder pour l'instant |
| Maj+F3 | « Précédent » dans la recherche et « Insérer une fonction » dans la grille | garder |
| Textes de consentement (réseau, fichier, carte, page web) | garder longs ; raccourcir | garder |
| Coupures de « Convertir » au clavier | ajouter un champ « positions » ; rien | ajouter |

## Technique

| Sujet | Options | Reco |
|---|---|---|
| DocumentService | un service, fichiers par domaine (fait) ; plusieurs services Wails (~1 j) | garder |
| Coût du défilement après la refonte (+7 % de script, 0,16 ms par image) | accepter ; objet fixe par image | accepter |
| Job « Bout en bout » en CI | bloquant après une semaine verte ; facultatif | bloquant après 1 semaine |
| Antivirus sur les runners Windows | le couper (−1 à 2 min) ; garder | couper |
| Job -race | bloquant ; facultatif | bloquant |
| -race sur bridge et canvas | en CI (~6 min) ; chaque semaine ; non | chaque semaine |

## Serveur Lattis (déjà transmis au fil serveur)

- **S6** : `max_connections` à dimensionner (règle : postes × 1,2 + 50 ; 400 ne suffit pas pour 300 postes).
- **A1** (appli) : sur un 503 du relais, attendre `Retry-After` (~30 s) avant de passer en LISTEN **(reco)**.
- **A2** (appli, avec S6) : fermer les connexions inactives au bout de 1–2 min.

## En attente depuis le 21/10

- Correctif des tables privées utilisées comme listes de validation : attend ton oui.

## Plus tard (estimés, pas urgents)

Mises à jour réelles macOS/Linux (3–4 j) ; retour arrière, ignorer une version, miroir d'entreprise (~2 j) ; dépôts APT/RPM ; RANDARRAY/WRAP/EXPAND (1 j) et ~30 fonctions (2–3 j) ; table des positions pour le tri (2–3 j) ; delta poussé par lattisd (3–5 j, avec le serveur) ; fin du découpage de la feuille et du ruban (1 j) ; internal/canvas (plusieurs jours) ; équivalents clavier des segments, contrôles et formes (1,5 j) ; aide détaillée encore longue (tableaux, regroupement, partage, cellules).
