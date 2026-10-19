package workbook

import (
	"context"
	"sync"
	"time"
)

// Backoff règle les nouveaux essais de connexion à un serveur injoignable :
// le premier après Min, puis deux fois plus tard à chaque échec, jusqu'à
// Max ; Reset après une connexion réussie, Wake pour essayer tout de suite
// (bandeau « Réessayer », Fichier › Réessayer la connexion). Un serveur
// arrêté pendant des heures n'est donc pas sollicité toutes les quelques
// secondes, et ne remplit pas le journal d'erreurs.
type Backoff struct {
	Min, Max time.Duration

	mu   sync.Mutex
	cur  time.Duration
	wake chan struct{}
}

// NewBackoff crée un Backoff de min à max (max < min : min).
func NewBackoff(min, max time.Duration) *Backoff {
	if max < min {
		max = min
	}
	return &Backoff{Min: min, Max: max, wake: make(chan struct{}, 1)}
}

// Next donne le délai avant le prochain essai, et double le suivant.
func (b *Backoff) Next() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cur < b.Min {
		b.cur = b.Min
	}
	d := b.cur
	b.cur = min(b.cur*2, b.Max)
	return d
}

// Reset : connecté ; le prochain échec attendra de nouveau Min.
func (b *Backoff) Reset() {
	b.mu.Lock()
	b.cur = 0
	b.mu.Unlock()
}

// Wake fait partir tout de suite l'essai en attente (et remet le délai au
// minimum).
func (b *Backoff) Wake() {
	b.Reset()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// Wait attend le prochain essai (Next, ou Wake) ; faux si ctx est annulé.
func (b *Backoff) Wait(ctx context.Context) bool {
	t := time.NewTimer(b.Next())
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
	case <-b.wake:
	}
	return true
}
