package main

import (
	"time"
)

// Fin d'une démo publique (contrat de lattisd, docs/collaboration/version-web.md
// § 6.2) : lattisd lance lattis-engine --public-demo avec l'heure de fin
// (RFC 3339, UTC) et le préavis (durée Go, « 2m0s ») dans l'environnement.
const (
	envDemoDeadline   = "LATTIS_DEMO_DEADLINE"
	envDemoWarnBefore = "LATTIS_DEMO_WARN_BEFORE"
)

// demoDeadlineEnv lit la fin de la démo et son préavis. Sans heure de fin
// (lattisd plus ancien), ou illisible : heure nulle, rien de particulier.
// Préavis absent, illisible ou négatif : 0 (pas de bandeau, la fin reste).
// Chaque valeur écartée est journalisée par logf.
func demoDeadlineEnv(getenv func(string) string, logf func(string, ...any)) (time.Time, time.Duration) {
	raw := getenv(envDemoDeadline)
	if raw == "" {
		if w := getenv(envDemoWarnBefore); w != "" {
			logf("lattis-engine : %s=%q sans %s : ignoré", envDemoWarnBefore, w, envDemoDeadline)
		}
		return time.Time{}, 0
	}
	deadline, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		logf("lattis-engine : %s=%q illisible (RFC 3339 attendu) : ignoré", envDemoDeadline, raw)
		return time.Time{}, 0
	}
	var warn time.Duration
	switch w := getenv(envDemoWarnBefore); {
	case w == "":
		logf("lattis-engine : %s absent : pas de préavis avant la fin de la démo", envDemoWarnBefore)
	default:
		d, err := time.ParseDuration(w)
		if err != nil || d < 0 {
			logf("lattis-engine : %s=%q illisible (durée positive attendue, « 2m0s ») : pas de préavis", envDemoWarnBefore, w)
		} else {
			warn = d
		}
	}
	return deadline.UTC(), warn
}
