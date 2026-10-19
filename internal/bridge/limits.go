package bridge

import (
	"time"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/script"
)

// Limites réglables (audit 6, M14 ; Fichier › Paramètres › Limites, ou
// configuration d'entreprise, clés limits.*) : lues dans la configuration
// courante à chaque usage, un changement vaut tout de suite.
func init() {
	command.DepthSetting = func() int { return config.Limit(config.Current(), "limits.undoDepth") }
	script.TimeoutSetting = func() time.Duration { return limitSeconds("limits.scriptSeconds") }
	grid.ReadTimeoutSetting = func() time.Duration { return limitSeconds("limits.sqlReadSeconds") }
}

// limitSeconds donne une durée réglée en secondes (limits.*).
func limitSeconds(path string) time.Duration {
	return time.Duration(config.Limit(config.Current(), path)) * time.Second
}
