package bridge

import (
	"context"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/config"
)

// Fichier › Paramètres : les réglages de l'utilisateur que l'interface
// affiche et modifie (liste config.UserEditable), écrits dans son
// config.json. Une stratégie de l'organisation les verrouille.

// Setting est l'état d'un réglage pour l'interface.
type Setting = config.UserSetting

// Settings donne les réglages modifiables depuis l'interface, avec leur
// valeur effective et s'ils sont verrouillés.
func (s *DocumentService) Settings(ctx context.Context) []Setting {
	return config.UserSettings(config.Current())
}

// SetSetting écrit un réglage de l'utilisateur (chemin pointé de la
// configuration, par exemple features.htmlTabs) et rend les réglages à jour.
// Refusé pour une clé hors de la liste ou imposée par une stratégie.
func (s *DocumentService) SetSetting(ctx context.Context, path string, value any) ([]Setting, error) {
	c, err := config.SetUserValue(ctx, path, value)
	if err != nil {
		return nil, err
	}
	if path == "ai.enabled" {
		s.syncMCP()
	}
	return config.UserSettings(c), nil
}

// ResetSettings remet les réglages paths (ceux de Fichier › Paramètres) à
// leur valeur par défaut et rend les réglages à jour. Les réglages imposés
// par une stratégie restent tels quels ; serveurs Lattis, fichiers récents,
// clés d'API et thèmes créés ne sont pas touchés.
func (s *DocumentService) ResetSettings(ctx context.Context, paths []string) ([]Setting, error) {
	c, err := config.ResetUserValues(ctx, paths)
	if err != nil {
		return nil, err
	}
	if slices.ContainsFunc(paths, func(p string) bool { return strings.EqualFold(p, "ai.enabled") }) {
		s.syncMCP()
	}
	return config.UserSettings(c), nil
}
