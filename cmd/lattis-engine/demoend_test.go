package main

import (
	"fmt"
	"testing"
	"time"
)

func TestDemoDeadlineEnv(t *testing.T) {
	at := time.Date(2026, 10, 21, 14, 30, 0, 0, time.UTC)
	for _, c := range []struct {
		name           string
		deadline, warn string
		want           time.Time
		wantWarn       time.Duration
		logs           int
	}{
		{"sans variables", "", "", time.Time{}, 0, 0},
		{"les deux", "2026-10-21T14:30:00Z", "2m0s", at, 2 * time.Minute, 0},
		{"décalage horaire", "2026-10-21T16:30:00+02:00", "90s", at, 90 * time.Second, 0},
		{"heure illisible", "21/10/2026 14:30", "2m0s", time.Time{}, 0, 1},
		{"préavis seul", "", "2m0s", time.Time{}, 0, 1},
		{"préavis illisible", "2026-10-21T14:30:00Z", "deux minutes", at, 0, 1},
		{"préavis négatif", "2026-10-21T14:30:00Z", "-1m", at, 0, 1},
		{"préavis absent", "2026-10-21T14:30:00Z", "", at, 0, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			env := map[string]string{envDemoDeadline: c.deadline, envDemoWarnBefore: c.warn}
			var logs []string
			got, warn := demoDeadlineEnv(func(k string) string { return env[k] }, func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) })
			if !got.Equal(c.want) || warn != c.wantWarn || len(logs) != c.logs {
				t.Fatalf("= %v, %v, journal %q ; attendu %v, %v, %d ligne(s)", got, warn, logs, c.want, c.wantWarn, c.logs)
			}
		})
	}
}
