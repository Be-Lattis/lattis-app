package bridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/mcp"
)

// Version est la version de l'application annoncée au client MCP.
var Version = "dev"

type mcpState struct {
	mu  sync.Mutex
	ln  *mcp.Listener
	err string
	w   mcpWatch // fil d'activité, séries, demandes d'accord (mcp_activity.go)
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

// startup démarre le serveur MCP s'il est activé. Jamais en web : un
// serveur MCP local à la machine de lattisd ne servirait pas l'utilisateur.
func (s *DocumentService) startup(ctx context.Context) error {
	if s.mcpRefused() != nil {
		return nil
	}
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
	ln, err := mcp.Listen(s.mcpServer(), st.Port, st.Token)
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
	if err := s.mcpRefused(); on && err != nil {
		return MCPStatus{}, err
	}
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

// --- Claude Desktop et Claude Code ---------------------------------------------

// MCPClients décrit les clients d'Anthropic qui peuvent piloter Lattis : la
// voie « abonnement Claude » (Pro, Max, Team), où l'on discute dans Claude
// et où Lattis montre ce qui est fait.
type MCPClients struct {
	DesktopInstalled  bool   `json:"desktopInstalled"`
	DesktopConfigured bool   `json:"desktopConfigured"`
	DesktopConfig     string `json:"desktopConfig"` // chemin de claude_desktop_config.json
	CodeFound         bool   `json:"codeFound"`     // commande claude trouvée
	CodeConfigured    bool   `json:"codeConfigured"`
	CodeCommand       string `json:"codeCommand"` // commande « claude mcp add … » à copier
	Connected         string `json:"connected"`   // client vu récemment (nom affiché), "" sinon
	ConfirmWrites     bool   `json:"confirmWrites"`
}

// mcpExecutable est le programme que Claude Desktop lance en relais
// (« lattis mcp ») ; surchargeable pour les tests.
var mcpExecutable = func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	return exe
}

// mcpURL rend l'adresse effective du serveur (port retenu s'il tourne).
func (s *DocumentService) mcpURL(st mcp.Settings) string {
	s.mcp.mu.Lock()
	defer s.mcp.mu.Unlock()
	if s.mcp.ln != nil {
		return mcp.URL(s.mcp.ln.Port)
	}
	return mcp.URL(st.Port)
}

// MCPClients décrit Claude Desktop et Claude Code sur ce poste : installés,
// configurés pour Lattis, client connecté et réglage « Demander avant de
// modifier » (mcp.confirmWrites, à changer par SetSetting).
func (s *DocumentService) MCPClients() (MCPClients, error) {
	if err := s.mcpRefused(); err != nil {
		return MCPClients{}, err
	}
	st, err := mcp.LoadSettings()
	if err != nil {
		return MCPClients{}, err
	}
	url := s.mcpURL(st)
	path := mcp.DesktopConfigPath()
	return MCPClients{
		DesktopInstalled:  mcp.DesktopInstalled(),
		DesktopConfigured: mcp.DesktopConfigured(path, mcpExecutable()),
		DesktopConfig:     path,
		CodeFound:         mcp.FindClaude() != "",
		CodeConfigured:    mcp.CodeConfigured(url, st.Token),
		CodeCommand:       mcp.CodeCommandLine(url, st.Token),
		Connected:         s.mcpConnected(),
		ConfirmWrites:     mcpConfirmWrites(),
	}, nil
}

// MCPConfigureDesktop active le serveur MCP et ajoute « lattis » à
// claude_desktop_config.json (copie .bak du fichier avant). Claude Desktop
// doit être redémarré pour le voir.
func (s *DocumentService) MCPConfigureDesktop() (MCPClients, error) {
	if err := s.mcpRefused(); err != nil {
		return MCPClients{}, err
	}
	exe := mcpExecutable()
	if exe == "" {
		return MCPClients{}, errors.New("chemin de Lattis introuvable")
	}
	if _, err := s.SetMCPEnabled(true); err != nil {
		return MCPClients{}, err
	}
	if err := mcp.ConfigureDesktop(mcp.DesktopConfigPath(), exe); err != nil {
		return MCPClients{}, err
	}
	return s.MCPClients()
}

// mcpCodeTimeout borne chaque appel de la commande claude.
var mcpCodeTimeout = 30 * time.Second

// MCPConfigureCode active le serveur MCP et l'ajoute à Claude Code, portée
// user (« claude mcp add --transport http --scope user lattis … ») ; une
// entrée « lattis » déjà là est d'abord retirée (adresse ou jeton changés).
func (s *DocumentService) MCPConfigureCode(ctx context.Context) (MCPClients, error) {
	if err := s.mcpRefused(); err != nil {
		return MCPClients{}, err
	}
	bin := mcp.FindClaude()
	if bin == "" {
		return MCPClients{}, errors.New("Claude Code introuvable sur ce poste : installez-le, ou lancez vous-même la commande proposée dans un terminal")
	}
	if _, err := s.SetMCPEnabled(true); err != nil {
		return MCPClients{}, err
	}
	st, err := mcp.LoadSettings()
	if err != nil {
		return MCPClients{}, err
	}
	run := func(args ...string) ([]byte, error) {
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), mcpCodeTimeout)
		defer cancel()
		cmd := exec.CommandContext(c, bin, args...)
		hideConsole(cmd)
		return cmd.CombinedOutput()
	}
	// Absente, l'entrée donne une erreur sans conséquence.
	run("mcp", "remove", "--scope", "user", mcp.ServerName)
	if out, err := run(mcp.CodeArgs(s.mcpURL(st), st.Token)...); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return MCPClients{}, fmt.Errorf("Claude Code a refusé la configuration : %s", shortError(msg))
	}
	return s.MCPClients()
}
