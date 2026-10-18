package config

import (
	"sort"
	"strings"
)

// Serveurs Lattis (classeurs partagés) dans la configuration :
//
//	"features": { "sharedWorkbooks": true, "allowUserLattisServers": false },
//	"connections": {
//	  "acme": { "kind": "lattis", "name": "Serveur Acme",
//	            "config": { "url": "https://lattis.acme.fr" },
//	            "auth": { "provider": "acme-keycloak" } }
//	}
//
// Une entrée de genre lattis pré-déclare un serveur (rien à saisir) ;
// auth.provider impose un fournisseur ; allowUserLattisServers à faux
// (dans une stratégie) interdit d'en ajouter d'autres.

// LattisServer est un Serveur Lattis déclaré par l'organisation.
type LattisServer struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	URL  string `json:"url"`
	// Provider : fournisseur imposé (identifiant annoncé par le serveur).
	Provider string `json:"provider,omitempty"`
}

// SharedWorkbooks dit si les classeurs partagés sont activés.
func SharedWorkbooks(c *Config) bool {
	return c.Bool("features.sharedWorkbooks", false)
}

// AllowUserLattisServers dit si l'utilisateur peut ajouter des serveurs.
func AllowUserLattisServers(c *Config) bool {
	return c.Bool("features.allowUserLattisServers", true)
}

// LattisServers liste les serveurs déclarés (connections.* de genre
// lattis avec une adresse), par identifiant.
func LattisServers(c *Config) []LattisServer {
	var conns map[string]ManagedConnection
	if err := c.Decode("connections", &conns); err != nil {
		return nil
	}
	out := []LattisServer{}
	for id, mc := range conns {
		if !strings.EqualFold(mc.Kind, "lattis") {
			continue
		}
		u, _ := mc.Config["url"].(string)
		if strings.TrimSpace(u) == "" {
			continue
		}
		out = append(out, LattisServer{ID: id, Name: mc.Name, URL: strings.TrimSpace(u), Provider: mc.Auth.Provider})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
