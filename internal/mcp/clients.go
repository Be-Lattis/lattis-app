package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Configuration de Claude Desktop et de Claude Code : la voie « abonnement
// Claude » (Pro, Max, Team). L'utilisateur discute dans Claude, qui pilote
// Lattis par ce serveur ; Lattis montre ce qui est fait.
//
// Claude Desktop lance un programme en stdio : son fichier
// claude_desktop_config.json reçoit mcpServers.lattis = {command: Lattis,
// args: ["mcp"]} (relais Proxy). Le fichier est dans le dossier de
// configuration de l'utilisateur (doc MCP « Connect to local servers ») :
// %APPDATA%\Claude sous Windows, ~/Library/Application Support/Claude sous
// macOS, ~/.config/Claude sous Linux (version bêta de Claude Desktop) :
// c'est os.UserConfigDir()/Claude partout.
//
// Claude Code se connecte en HTTP : « claude mcp add --transport http
// --scope user lattis <url> --header "Authorization: Bearer <jeton>" »
// (doc Claude Code « Connect Claude Code to tools via MCP ») ; la portée user
// est rangée au premier niveau de ~/.claude.json, clé mcpServers
// ($CLAUDE_CONFIG_DIR/.claude.json quand cette variable est posée).

// ServerName est le nom du serveur dans la configuration des clients.
const ServerName = "lattis"

// DesktopDir est le dossier de Claude Desktop ("" si le dossier de
// configuration de l'utilisateur est inconnu).
func DesktopDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Claude")
}

// DesktopConfigPath est le chemin de claude_desktop_config.json.
func DesktopConfigPath() string {
	d := DesktopDir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, "claude_desktop_config.json")
}

// DesktopInstalled : le dossier de Claude Desktop existe (l'application a
// été lancée au moins une fois sur ce compte).
func DesktopInstalled() bool {
	d := DesktopDir()
	if d == "" {
		return false
	}
	fi, err := os.Stat(d)
	return err == nil && fi.IsDir()
}

// DesktopConfigured dit si le fichier path lance déjà exe en relais.
func DesktopConfigured(path, exe string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var c struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if json.Unmarshal(b, &c) != nil {
		return false
	}
	e, ok := c.MCPServers[ServerName]
	return ok && samePath(e.Command, exe) && len(e.Args) == 1 && e.Args[0] == "mcp"
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// ConfigureDesktop ajoute (ou remplace) l'entrée « lattis » de
// claude_desktop_config.json sans toucher au reste du fichier : les autres
// clés et serveurs gardent leur ordre et leur contenu. Le fichier existant est
// d'abord copié en .bak. Un fichier illisible (JSON invalide) n'est pas
// écrasé.
func ConfigureDesktop(path, exe string) error {
	if path == "" {
		return errors.New("dossier de configuration de l'utilisateur introuvable")
	}
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	entry, _ := json.Marshal(map[string]any{"command": exe, "args": []string{"mcp"}})
	out, err := setNested(old, "mcpServers", ServerName, entry)
	if err != nil {
		return fmt.Errorf("%s n'est pas un JSON valide, il n'a pas été modifié : %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if old != nil {
		if fi, err := os.Stat(path); err == nil {
			mode = fi.Mode().Perm()
		}
		if err := os.WriteFile(path+".bak", old, mode); err != nil {
			return fmt.Errorf("copie de sauvegarde impossible : %w", err)
		}
	}
	return os.WriteFile(path, out, mode)
}

// member est une clé d'un objet JSON, dans l'ordre du fichier.
type member struct {
	key string
	val json.RawMessage
}

// objectMembers lit un objet JSON en gardant l'ordre des clés.
func objectMembers(b []byte) ([]member, error) {
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := t.(json.Delim); !ok || d != '{' {
		return nil, errors.New("objet JSON attendu")
	}
	var out []member
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		k, _ := t.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, member{k, v})
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("contenu après l'objet JSON")
	}
	return out, nil
}

func encodeMembers(ms []member) []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range ms {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(m.key)
		b.Write(k)
		b.WriteByte(':')
		b.Write(m.val)
	}
	b.WriteByte('}')
	return b.Bytes()
}

func setMember(ms []member, key string, val json.RawMessage) []member {
	for i := range ms {
		if ms[i].key == key {
			ms[i].val = val
			return ms
		}
	}
	return append(ms, member{key, val})
}

// setNested pose doc[outer][inner] = val et rend le document indenté.
func setNested(doc []byte, outer, inner string, val json.RawMessage) ([]byte, error) {
	top, err := objectMembers(doc)
	if err != nil {
		return nil, err
	}
	var sub []member
	for _, m := range top {
		if m.key == outer && !bytes.Equal(bytes.TrimSpace(m.val), []byte("null")) {
			if sub, err = objectMembers(m.val); err != nil {
				return nil, fmt.Errorf("%s : %w", outer, err)
			}
		}
	}
	sub = setMember(sub, inner, val)
	top = setMember(top, outer, encodeMembers(sub))
	var out bytes.Buffer
	if err := json.Indent(&out, encodeMembers(top), "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// --- Claude Code -------------------------------------------------------------

// lookPath est exec.LookPath ; surchargeable pour les tests.
var lookPath = exec.LookPath

// claudeCandidates sont les emplacements d'installation habituels de
// Claude Code : une application lancée depuis le Finder ou le menu
// Démarrer n'a souvent pas le PATH du terminal. Installeur natif :
// ~/.local/bin/claude (%USERPROFILE%\.local\bin\claude.exe sous Windows) ;
// ancienne installation locale : ~/.claude/local/claude ; Homebrew :
// /opt/homebrew/bin, /usr/local/bin ; npm global : %APPDATA%\npm,
// ~/.npm-global/bin ; WinGet : %LOCALAPPDATA%\Microsoft\WinGet\Links.
func claudeCandidates() []string {
	home, _ := os.UserHomeDir()
	var out []string
	add := func(parts ...string) {
		for _, p := range parts {
			if p == "" {
				return
			}
		}
		out = append(out, filepath.Join(parts...))
	}
	if runtime.GOOS == "windows" {
		add(home, ".local", "bin", "claude.exe")
		add(os.Getenv("APPDATA"), "npm", "claude.cmd")
		add(os.Getenv("LOCALAPPDATA"), "Microsoft", "WinGet", "Links", "claude.exe")
		add(home, ".claude", "local", "claude.cmd")
		return out
	}
	add(home, ".local", "bin", "claude")
	add(home, ".claude", "local", "claude")
	add("/opt/homebrew/bin/claude")
	add("/usr/local/bin/claude")
	add(home, ".npm-global", "bin", "claude")
	add(home, ".bun", "bin", "claude")
	add("/usr/bin/claude")
	return out
}

// FindClaude trouve la commande claude : dans le PATH, sinon aux
// emplacements d'installation habituels ("" si elle est introuvable).
func FindClaude() string {
	if p, err := lookPath("claude"); err == nil {
		return p
	}
	for _, p := range claudeCandidates() {
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			continue
		}
		if runtime.GOOS != "windows" && fi.Mode().Perm()&0o111 == 0 {
			continue
		}
		return p
	}
	return ""
}

// CodeArgs sont les arguments de « claude » qui ajoutent Lattis à la portée
// user de Claude Code.
func CodeArgs(url, token string) []string {
	return []string{"mcp", "add", "--transport", "http", "--scope", "user", ServerName, url, "--header", "Authorization: Bearer " + token}
}

// CodeCommandLine est la commande à copier dans un terminal.
func CodeCommandLine(url, token string) string {
	return fmt.Sprintf(`claude mcp add --transport http --scope user %s %s --header "Authorization: Bearer %s"`, ServerName, url, token)
}

// CodeConfigPath est le fichier où Claude Code range la portée user.
func CodeConfigPath() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, ".claude.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude.json")
}

// CodeConfigured dit si Claude Code connaît Lattis à cette adresse avec ce
// jeton (portée user). Un port ou un jeton changé depuis donne faux : il
// faut reconfigurer.
func CodeConfigured(url, token string) bool {
	b, err := os.ReadFile(CodeConfigPath())
	if err != nil {
		return false
	}
	var c struct {
		MCPServers map[string]struct {
			Type    string            `json:"type"`
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if json.Unmarshal(b, &c) != nil {
		return false
	}
	e, ok := c.MCPServers[ServerName]
	if !ok || e.URL != url {
		return false
	}
	for k, v := range e.Headers {
		if strings.EqualFold(k, "Authorization") {
			return v == "Bearer "+token
		}
	}
	return false
}
