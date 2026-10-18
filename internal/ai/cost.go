package ai

// Coût d'une conversation : jetons rendus par le fournisseur à chaque tour,
// convertis en dollars US avec les tarifs publics par million de jetons.
// Un modèle absent de la table (fournisseur sans tarif connu, modèle local,
// passerelle) coûte 0 : l'interface n'affiche alors pas de coût.

// Usage compte les jetons d'un tour ou d'une conversation.
type Usage struct {
	// InputTokens : jetons d'entrée au total, cache compris (lus et écrits).
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	// Détail de l'entrée : jetons lus dans le cache, écrits dans le cache.
	CacheReadTokens  int64 `json:"cacheReadTokens,omitempty"`
	CacheWriteTokens int64 `json:"cacheWriteTokens,omitempty"`
	// Cost : coût approximatif en dollars US (0 : tarif inconnu).
	Cost float64 `json:"cost"`
}

// Add cumule u dans le total.
func (t *Usage) Add(u Usage) {
	t.InputTokens += u.InputTokens
	t.OutputTokens += u.OutputTokens
	t.CacheReadTokens += u.CacheReadTokens
	t.CacheWriteTokens += u.CacheWriteTokens
	t.Cost += u.Cost
}

// Price est le tarif d'un modèle, en dollars US par million de jetons.
type Price struct {
	Input, Output, CacheRead, CacheWrite float64
	// Long : tarif au-delà de LongAbove jetons d'entrée dans la requête
	// (zéro : pas de palier).
	LongAbove int64
	Long      *Price
}

// prices : tarifs publics de l'API Anthropic (octobre 2026). Écriture dans
// le cache (durée de 5 minutes, celle que Lattis demande) : 1,25 × l'entrée ;
// lecture : 0,20 $ pour Opus 5.5 et Sonnet 5.5, 0,1 × l'entrée pour Haiku
// 5.5, dont le tarif change au-delà de 100 000 jetons de requête.
var prices = map[string]Price{
	"claude-opus-5-5":   {Input: 4, Output: 20, CacheRead: 0.20, CacheWrite: 5},
	"claude-sonnet-5-5": {Input: 2, Output: 10, CacheRead: 0.20, CacheWrite: 2.50},
	"claude-haiku-5-5": {Input: 0.10, Output: 0.50, CacheRead: 0.01, CacheWrite: 0.125,
		LongAbove: 100_000, Long: &Price{Input: 0.50, Output: 2.50, CacheRead: 0.05, CacheWrite: 0.625}},
}

// SetPrice déclare le tarif d'un modèle (fournisseurs ajoutés ailleurs).
func SetPrice(model string, p Price) {
	regMu.Lock()
	defer regMu.Unlock()
	prices[model] = p
}

// Cost donne le coût de u pour model (0 si le tarif est inconnu).
func Cost(model string, u Usage) float64 {
	regMu.RLock()
	p, ok := prices[model]
	regMu.RUnlock()
	if !ok {
		return 0
	}
	if p.Long != nil && u.InputTokens > p.LongAbove {
		p = *p.Long
	}
	plain := u.InputTokens - u.CacheReadTokens - u.CacheWriteTokens
	if plain < 0 {
		plain = 0
	}
	return (float64(plain)*p.Input + float64(u.CacheReadTokens)*p.CacheRead +
		float64(u.CacheWriteTokens)*p.CacheWrite + float64(u.OutputTokens)*p.Output) / 1e6
}
