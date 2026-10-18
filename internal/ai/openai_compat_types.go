package ai

// À FUSIONNER DANS ai.go PAR L'INTÉGRATEUR (chantier B).
//
// Le chantier A ajoute à ai.go les champs Info.NeedsKey et Info.Local,
// l'interface KeyChecker et le champ Reply.Usage. Ce fichier en tient lieu
// pour que l'adaptateur compatible OpenAI (openai.go) compile seul ; à la
// fusion :
//   - supprimer KeyChecker ci-dessous (doublon de celui de A) ;
//   - déplacer ModelLister dans ai.go (nouveau, propre au chantier B) ;
//   - remplacer le corps de setInfoFlags par
//     i.NeedsKey, i.Local = needsKey, local ;
//   - remplacer le corps de setUsage par l'affectation du Reply.Usage de A
//     (jetons d'entrée et de sortie du tour ; le coût n'est pas connu de
//     ces fournisseurs, les tarifs variant d'un modèle à l'autre).
// Puis supprimer ce fichier.

import "context"

// KeyChecker est un fournisseur qui sait vérifier une clé d'API avant de
// l'enregistrer (doublon de celui du chantier A).
type KeyChecker interface {
	CheckKey(ctx context.Context, cred Credentials) error
}

// ModelLister est un fournisseur qui lit sa liste de modèles chez lui
// (GET /models) ; Info().Models rend ensuite la dernière liste lue, ou une
// petite liste fixe de repli.
type ModelLister interface {
	ListModels(ctx context.Context, cred Credentials) ([]Model, error)
}

// setInfoFlags : Info.NeedsKey (une clé d'API est nécessaire) et Info.Local
// (modèle sur ce poste). Sans effet tant que ces champs n'existent pas.
func setInfoFlags(i *Info, needsKey, local bool) {}

// setUsage : jetons consommés par le tour. Sans effet tant que Reply.Usage
// n'existe pas.
func setUsage(r *Reply, inputTokens, outputTokens int) {}
