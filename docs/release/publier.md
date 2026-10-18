# Publier une version

Un tag `v*` poussé sur GitHub lance le workflow **Release** (`.github/workflows/release.yml`) : il construit les installeurs Windows, macOS et Linux, les signe quand les secrets sont configurés (voir [signature.md](signature.md)), puis publie la release GitHub avec les fichiers, leurs empreintes et les notes de version.

## Créer le tag

```bash
scripts/release.sh v1.2.0
```

Le script vérifie qu'on est sur `main`, à jour avec `origin/main`, sans fichier suivi modifié et que le tag n'existe pas ; il lance `go test ./...`, crée le tag annoté, affiche les notes de version puis pousse le tag. Options : `--skip-tests`, `--allow-dirty` (tague malgré des modifications locales, qui ne sont pas dans la version).

- `v1.2.0` : version publiée.
- `v1.2.0-beta.1` (tout tag avec un tiret) : pré-version.

## Notes de version

`scripts/release-notes.sh v1.2.0` les produit (aussi en local, pour relire avant de taguer) : la liste des commits depuis le tag précédent, sans les fusions ni les régénérations de liaisons, puis le tableau des fichiers à télécharger. La première version dit simplement « Première version publique de Lattis ».

## Brouillon ou publication directe

La release est publiée directement. Pour la relire avant publication, créer la variable de dépôt `RELEASE_DRAFT` à `true` (Settings › Secrets and variables › Actions › Variables) : la release reste alors en brouillon, à publier depuis GitHub.

## Essai sans release

Actions › Release › Run workflow, avec une version d'essai (`v0.0.0-test`) : les paquets sont construits et déposés comme artefacts du run, sans release.
