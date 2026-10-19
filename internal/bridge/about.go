package bridge

// About décrit l'application pour la fenêtre « À propos » : version,
// licence, code source ; en édition web, le code source du serveur qui sert
// la session (AGPL-3.0 §13).
type About struct {
	Version string `json:"version"`
	License string `json:"license"`
	Source  string `json:"source"`
	// Server, ServerSource : serveur de la session web et son code source
	// (vides sur le bureau).
	Server       string `json:"server,omitempty"`
	ServerSource string `json:"serverSource,omitempty"`
}

const (
	appSource    = "https://github.com/Be-Lattis/lattis-app"
	serverSource = "https://github.com/Be-Lattis/lattis-server"
)

// About rend la fiche « À propos ».
func (s *DocumentService) About() About {
	a := About{Version: Version, License: "MPL-2.0", Source: appSource}
	if f := fixed; f != nil {
		a.Server, a.ServerSource = f.Server, serverSource
		f.mu.Lock()
		if f.disc != nil && f.disc.SourceURL != "" {
			a.ServerSource = f.disc.SourceURL
		}
		f.mu.Unlock()
	}
	return a
}
