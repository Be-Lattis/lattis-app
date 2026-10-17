package bridge

import (
	"context"
	"os"
	"sync"

	"github.com/lalamefine/lattis/internal/mcp"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Version est la version de l'application annoncée au client MCP.
var Version = "dev"

type mcpState struct {
	mu  sync.Mutex
	ln  *mcp.Listener
	err string
}

// MCPStatus décrit le serveur MCP pour le menu Fichier.
type MCPStatus struct {
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
	URL     string `json:"url"`
	Token   string `json:"token"`
	// Command lance le relais stdio (clients qui démarrent un programme).
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Error   string   `json:"error"`
}

// ServiceStartup démarre le serveur MCP s'il est activé.
func (s *DocumentService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	if st, err := mcp.LoadSettings(); err == nil && st.Enabled {
		s.startMCP(st)
	}
	return nil
}

func (s *DocumentService) startMCP(st mcp.Settings) {
	s.mcp.mu.Lock()
	defer s.mcp.mu.Unlock()
	if s.mcp.ln != nil {
		return
	}
	srv := &mcp.Server{Host: host{s}, Name: "lattis", Version: Version}
	ln, err := mcp.Listen(srv, st.Port, st.Token)
	if err != nil {
		s.mcp.err = err.Error()
		return
	}
	s.mcp.ln, s.mcp.err = ln, ""
	if ln.Port != st.Port { // port occupé : le relais stdio lira le nouveau
		st.Port = ln.Port
		mcp.SaveSettings(st)
	}
}

func (s *DocumentService) stopMCP() {
	s.mcp.mu.Lock()
	defer s.mcp.mu.Unlock()
	if s.mcp.ln != nil {
		s.mcp.ln.Close()
		s.mcp.ln = nil
	}
}

// MCPState décrit le serveur MCP.
func (s *DocumentService) MCPState() (MCPStatus, error) {
	st, err := mcp.LoadSettings()
	if err != nil {
		return MCPStatus{}, err
	}
	exe, _ := os.Executable()
	s.mcp.mu.Lock()
	defer s.mcp.mu.Unlock()
	out := MCPStatus{Enabled: st.Enabled, Running: s.mcp.ln != nil, URL: mcp.URL(st.Port), Token: st.Token, Command: exe, Args: []string{"mcp"}, Error: s.mcp.err}
	if s.mcp.ln != nil {
		out.URL = mcp.URL(s.mcp.ln.Port)
	}
	return out, nil
}

// SetMCPEnabled active ou désactive le serveur MCP (réglage gardé).
func (s *DocumentService) SetMCPEnabled(on bool) (MCPStatus, error) {
	st, err := mcp.LoadSettings()
	if err != nil {
		return MCPStatus{}, err
	}
	st.Enabled = on
	if err := mcp.SaveSettings(st); err != nil {
		return MCPStatus{}, err
	}
	if on {
		s.startMCP(st)
	} else {
		s.stopMCP()
	}
	return s.MCPState()
}
