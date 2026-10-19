package workbook

import (
	"context"
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	b := NewBackoff(time.Second, 5*time.Second)
	var got []time.Duration
	for range 5 {
		got = append(got, b.Next())
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("délais : %v, attendu %v", got, want)
		}
	}
	b.Reset()
	if d := b.Next(); d != time.Second {
		t.Fatalf("après Reset : %v", d)
	}

	// Wake fait partir l'essai en attente tout de suite.
	b = NewBackoff(time.Hour, time.Hour)
	done := make(chan bool)
	go func() { done <- b.Wait(context.Background()) }()
	time.Sleep(10 * time.Millisecond)
	b.Wake()
	select {
	case ok := <-done:
		if !ok {
			t.Fatal("Wait : faux")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wake sans effet")
	}
	// Annulé : faux.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if b.Wait(ctx) {
		t.Fatal("Wait annulé : vrai")
	}
}
