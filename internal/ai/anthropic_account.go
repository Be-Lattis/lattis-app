package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/config"
)

// Connexion par compte Anthropic, sans clé d'API : l'outil en ligne de
// commande d'Anthropic (« ant auth login ») ouvre le navigateur, l'utilisateur
// se connecte à son compte de la console Anthropic, et l'outil range un
// profil sur le poste (configs/ et credentials/ sous le dossier de
// configuration d'Anthropic, voir config.DefaultDir). Lattis lit ce profil par
// le SDK, qui renouvelle lui-même le jeton. Rien n'est écrit dans le classeur.

// anthropicCLI est l'outil de connexion d'Anthropic.
const anthropicCLI = "ant"

// anthropicLoginTimeout borne l'attente de la connexion dans le navigateur.
const anthropicLoginTimeout = 10 * time.Minute

// anthropicProfile rend le profil actif s'il est utilisable : configuration
// lisible et, pour une connexion par navigateur, jeton présent.
func anthropicProfile() (*config.Config, string, bool) {
	cfg, err := config.LoadConfig()
	if err != nil || cfg == nil || cfg.AuthenticationInfo == nil {
		return nil, "", false
	}
	a := cfg.AuthenticationInfo
	if a.Type == config.AuthenticationTypeUserOAuth {
		if a.CredentialsPath == "" {
			return nil, "", false
		}
		if _, err := os.Stat(a.CredentialsPath); err != nil {
			return nil, "", false
		}
	}
	name := os.Getenv("ANTHROPIC_PROFILE")
	if name == "" {
		name = activeProfileName()
	}
	return cfg, name, true
}

// activeProfileName lit le nom du profil actif, pour l'affichage.
func activeProfileName() string {
	b, err := os.ReadFile(config.ActiveConfigPath(config.DefaultDir()))
	if err == nil {
		if n := strings.TrimSpace(string(b)); n != "" {
			return n
		}
	}
	return "default"
}

func (anthropicProvider) Account() Account {
	acc := Account{Tool: anthropicCLI}
	if _, err := exec.LookPath(anthropicCLI); err == nil {
		acc.CanLogin = true
	}
	if cfg, name, ok := anthropicProfile(); ok {
		acc.Connected = true
		acc.Name = "profil « " + name + " »"
		var creds config.Credentials
		if b, err := os.ReadFile(cfg.AuthenticationInfo.CredentialsPath); err == nil && json.Unmarshal(b, &creds) == nil && creds.AccountEmail != "" {
			acc.Name = creds.AccountEmail
		}
	}
	return acc
}

func (anthropicProvider) Login(ctx context.Context) error {
	bin, err := exec.LookPath(anthropicCLI)
	if err != nil {
		return errors.New("l'outil « ant » d'Anthropic n'est pas installé sur ce poste : installez-le, ou enregistrez une clé d'API")
	}
	ctx, cancel := context.WithTimeout(ctx, anthropicLoginTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "auth", "login")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return errors.New("connexion abandonnée : le navigateur n'a pas répondu à temps")
		}
		msg := strings.TrimSpace(out.String())
		if len(msg) > 300 {
			msg = msg[len(msg)-300:]
		}
		if msg == "" {
			msg = err.Error()
		}
		return errors.New("connexion au compte Anthropic impossible : " + msg)
	}
	if _, _, ok := anthropicProfile(); !ok {
		return errors.New("connexion terminée mais aucun profil Anthropic utilisable n'a été trouvé")
	}
	return nil
}
