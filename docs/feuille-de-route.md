# Feuille de route

Sujets connus, pas urgents, à reprendre plus tard. Le plus récent en haut.

## Mise à jour automatique : installation silencieuse (21/10/2026)

- Constat (poste de Louis, v0.3.2-beta par utilisateur) : « Redémarrez l'application pour appliquer la mise à jour » ferme Lattis sans rien installer, et le message revient à chaque lancement.
- Cause trouvée et corrigée sur main (871a2bf) : l'installeur n'était lancé que par `PostShutdown` de Wails, que la fermeture de la fenêtre n'atteint pas ; il part maintenant après `app.Run()`, à toute fermeture. Les essais sont notés, un échec s'affiche (« Installer manuellement »), l'installation automatique s'arrête après deux échecs, l'installeur laisse un compte rendu (`install-result.txt`).
- Reste à faire :
  - éprouver l'installation silencieuse sur de vrais postes (par utilisateur, pour tous les utilisateurs avec l'UAC, version portable), de version en version ;
  - les versions jusqu'à 0.3.3-beta gardent le défaut : une installation à la main, une fois ;
  - suites de l'étude (docs/conception/mise-a-jour-automatique.md) : signature Ed25519 de `latest.json`, miroir interne (`updates.url`), « ignorer cette version », retour arrière si la nouvelle version ne démarre pas, mise à jour réelle sous macOS et Linux.
