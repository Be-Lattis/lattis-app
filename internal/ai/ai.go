// Package ai est l'Assistant ✦ de Lattis : un assistant conversationnel qui
// aide à construire une analyse à partir de la structure du classeur, et qui
// agit dessus avec les commandes communes aux scripts Lua et au serveur MCP
// (internal/api).
//
// La fonction est opt-in (réglage ai.enabled, coupé par défaut, verrouillable
// par une stratégie). Les clés d'API ne vont jamais dans le classeur ni dans
// la configuration : elles sont gardées dans le trousseau du système
// (keys.go).
//
// Plusieurs fournisseurs sont prévus (Claude, Mistral, GPT, Gemini, Mammouth
// AI…) : la conversation est gardée dans un format neutre (Message, Block)
// et chaque fournisseur n'implémente qu'un tour d'échange (Provider.Turn) ;
// la boucle d'outils (agent.go) est commune. Seul Claude est branché pour
// l'instant (anthropic.go) ; les autres figurent au catalogue, indisponibles.
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
)

// Rôles d'un message.
const (
	User      = "user"
	Assistant = "assistant"
)

// Types de blocs.
const (
	TextBlock       = "text"
	ToolUseBlock    = "tool_use"
	ToolResultBlock = "tool_result"
)

// Block est un morceau d'un message : texte, appel d'outil (assistant) ou
// résultat d'outil (utilisateur).
type Block struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"` // texte, ou contenu du résultat
	// Appel d'outil.
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	// Résultat d'outil.
	ToolUseID string `json:"toolUseId,omitempty"`
	IsError   bool   `json:"isError,omitempty"`
}

// Message est un message de la conversation, dans le format neutre.
type Message struct {
	Role    string  `json:"role"`
	Content []Block `json:"content"`
	// Provider et Native : le message tel que le fournisseur l'a rendu
	// (blocs de réflexion compris). Il lui est renvoyé tel quel aux tours
	// suivants ; un autre fournisseur reçoit la forme neutre.
	Provider string `json:"-"`
	Native   any    `json:"-"`
}

// Tool est un outil proposé au modèle (schéma JSON de ses paramètres).
type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
}

// Request est un tour d'échange avec le modèle.
type Request struct {
	Model    string
	System   string
	Messages []Message
	Tools    []Tool
}

// Raisons de fin d'un tour.
const (
	StopEnd       = "end"
	StopToolUse   = "tool_use"
	StopMaxTokens = "max_tokens"
	StopRefusal   = "refusal"
)

// Reply est la réponse du modèle à un tour.
type Reply struct {
	Message Message
	Stop    string
	// Usage : jetons consommés par ce tour, tels que le fournisseur les
	// compte (zéro s'il ne les donne pas) ; le coût est calculé par la
	// session (Price).
	Usage Usage
}

// Credentials : de quoi joindre le fournisseur.
type Credentials struct {
	// Key : clé d'API ; vide pour un fournisseur sans clé (modèle local).
	Key string
	// Endpoint : adresse imposée (passerelle de l'entreprise, ai.endpoint) ;
	// vide : l'adresse publique du fournisseur.
	Endpoint string
}

// Provider est un fournisseur de modèles.
type Provider interface {
	Info() Info
	// Turn envoie la conversation et rend la réponse du modèle, en émettant
	// son texte au fil de l'eau (Event de type EventText).
	Turn(ctx context.Context, cred Credentials, req Request, emit func(Event)) (Reply, error)
}

// KeyChecker est un fournisseur qui sait vérifier une clé avant qu'elle
// soit enregistrée (appel léger, sans coût). L'erreur dit pourquoi la clé
// est refusée ; une erreur ErrUnreachable : le fournisseur n'a pas pu être
// joint, la clé n'est ni acceptée ni refusée.
type KeyChecker interface {
	CheckKey(ctx context.Context, cred Credentials) error
}

// ErrUnreachable : le fournisseur n'a pas pu être joint (réseau, proxy).
var ErrUnreachable = errors.New("fournisseur injoignable")

// Info décrit un fournisseur pour l'interface.
type Info struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Available : branché dans cette version.
	Available    bool    `json:"available"`
	Models       []Model `json:"models"`
	DefaultModel string  `json:"defaultModel"`
	// KeyURL : page où l'utilisateur crée sa clé d'API.
	KeyURL string `json:"keyUrl,omitempty"`
	// KeyHint : début attendu d'une clé (« sk-ant- »), pour l'aide.
	KeyHint string `json:"keyHint,omitempty"`
	// NeedsKey : une clé d'API est nécessaire (faux pour un modèle local).
	NeedsKey bool `json:"needsKey"`
	// Local : le modèle tourne sur ce poste ou le réseau local ; aucune
	// donnée ne part chez un fournisseur.
	Local bool `json:"local"`
}

// Model est un modèle proposé.
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
}

var (
	regMu     sync.RWMutex
	providers = map[string]Provider{}
	// planned : fournisseurs prévus, pas encore branchés (non proposés).
	planned = []Info{
		{ID: "mistral", Name: "Mistral AI", KeyURL: "https://console.mistral.ai/api-keys"},
		{ID: "openai", Name: "OpenAI (GPT)", KeyURL: "https://platform.openai.com/api-keys"},
		{ID: "gemini", Name: "Google Gemini", KeyURL: "https://aistudio.google.com/apikey"},
		{ID: "mammouth", Name: "Mammouth AI", KeyURL: "https://mammouth.ai"},
	}
)

// Register ajoute un fournisseur branché.
func Register(p Provider) {
	regMu.Lock()
	defer regMu.Unlock()
	providers[p.Info().ID] = p
}

// Lookup trouve un fournisseur branché.
func Lookup(id string) Provider {
	regMu.RLock()
	defer regMu.RUnlock()
	return providers[id]
}

// Catalog liste les fournisseurs branchés, seuls proposés (choix de Louis :
// pas de fournisseur annoncé mais pas encore intégré dans l'interface).
func Catalog() []Info {
	regMu.RLock()
	defer regMu.RUnlock()
	var out []Info
	for _, p := range providers {
		out = append(out, p.Info())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ErrNoKey : aucune clé d'API pour ce fournisseur.
var ErrNoKey = errors.New("aucune clé d'API enregistrée pour ce fournisseur : ouvrez « Connexion »")
