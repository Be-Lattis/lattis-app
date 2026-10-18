package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Claude (Anthropic), par le SDK officiel. Réflexion adaptative (par
// défaut sur ces modèles), réponse en flux, mise en cache du préfixe
// (outils et consigne, identiques d'un tour à l'autre).

// anthropicMaxTokens borne la longueur d'une réponse (en flux, pas de
// risque de délai dépassé).
const anthropicMaxTokens = 32000

type anthropicProvider struct{}

func init() { Register(anthropicProvider{}) }

func (anthropicProvider) Info() Info {
	return Info{
		ID:        "anthropic",
		Name:      "Claude (Anthropic)",
		Available: true,
		Models: []Model{
			{ID: "claude-opus-5-5", Name: "Claude Opus 5.5", Note: "le plus capable"},
			{ID: "claude-sonnet-5-5", Name: "Claude Sonnet 5.5", Note: "rapide et polyvalent"},
			{ID: "claude-haiku-5-5", Name: "Claude Haiku 5.5", Note: "le plus rapide et le moins cher"},
		},
		// Sonnet par défaut : bien moins cher qu'Opus pour une longue suite
		// d'appels d'outils, et suffisant pour construire une analyse.
		DefaultModel: "claude-sonnet-5-5",
		KeyURL:       "https://platform.claude.com/settings/keys",
		KeyHint:      "sk-ant-",
		NeedsKey:     true,
	}
}

// anthropicClient prépare le client : seule la clé donnée compte (variables
// d'environnement et profils de l'outil « ant » ignorés).
func anthropicClient(cred Credentials) (anthropic.Client, error) {
	// Sans clé : seulement à travers une passerelle d'entreprise qui
	// authentifie elle-même (ai.auth.mode = none).
	if cred.Key == "" && cred.Endpoint == "" {
		return anthropic.Client{}, ErrNoKey
	}
	opts := []option.RequestOption{option.WithoutEnvironmentDefaults()}
	if cred.Key != "" {
		opts = append(opts, option.WithAPIKey(cred.Key))
	}
	if cred.Endpoint != "" {
		opts = append(opts, option.WithBaseURL(cred.Endpoint))
	}
	return anthropic.NewClient(opts...), nil
}

// CheckKey vérifie la clé par la liste des modèles (appel gratuit). Une
// clé sans crédit y est acceptée : le manque de crédit n'apparaît qu'au
// premier message (anthropicError).
func (anthropicProvider) CheckKey(ctx context.Context, cred Credentials) error {
	client, err := anthropicClient(cred)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err = client.Models.List(ctx, anthropic.ModelListParams{Limit: anthropic.Int(1)}, option.WithMaxRetries(0))
	if err == nil {
		return nil
	}
	var apierr *anthropic.Error
	if !errors.As(err, &apierr) {
		return fmt.Errorf("%w : %v", ErrUnreachable, err)
	}
	if apierr.StatusCode >= 500 {
		return fmt.Errorf("%w (%d)", ErrUnreachable, apierr.StatusCode)
	}
	return anthropicError(err)
}

func (p anthropicProvider) Turn(ctx context.Context, cred Credentials, req Request, emit func(Event)) (Reply, error) {
	client, err := anthropicClient(cred)
	if err != nil {
		return Reply{}, err
	}

	params := anthropic.MessageNewParams{
		Model:        req.Model,
		MaxTokens:    anthropicMaxTokens,
		Messages:     p.messages(req.Messages),
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortMedium},
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
	}
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}
	for _, t := range req.Tools {
		props, _ := t.Schema["properties"].(map[string]any)
		tp := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.ToolInputSchemaParam{Properties: props},
		}
		if req, ok := t.Schema["required"].([]string); ok {
			tp.InputSchema.Required = req
		}
		params.Tools = append(params.Tools, anthropic.ToolUnionParam{OfTool: &tp})
	}
	var callOpts []option.RequestOption
	// Un refus des filtres de sécurité est servi par un modèle de repli dans
	// le même appel. Pas de repli pour Haiku, ni à travers une passerelle
	// d'entreprise (qui pourrait refuser le paramètre).
	if cred.Endpoint == "" && !strings.Contains(req.Model, "haiku") {
		callOpts = append(callOpts,
			option.WithHeaderAdd("anthropic-beta", "server-side-fallback-2026-07-01"),
			option.WithJSONSet("fallbacks", "default"))
	}

	stream := client.Messages.NewStreaming(ctx, params, callOpts...)
	msg := anthropic.Message{}
	for stream.Next() {
		ev := stream.Current()
		if err := msg.Accumulate(ev); err != nil {
			return Reply{}, err
		}
		if d, ok := ev.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if t, ok := d.Delta.AsAny().(anthropic.TextDelta); ok && t.Text != "" {
				emit(Event{Kind: EventText, Text: t.Text})
			}
		}
	}
	if err := stream.Err(); err != nil {
		return Reply{}, anthropicError(err)
	}

	out := Message{Role: Assistant, Provider: "anthropic", Native: msg.ToParam()}
	for _, b := range msg.Content {
		switch v := b.AsAny().(type) {
		case anthropic.TextBlock:
			out.Content = append(out.Content, Block{Type: TextBlock, Text: v.Text})
		case anthropic.ToolUseBlock:
			out.Content = append(out.Content, Block{Type: ToolUseBlock, ID: v.ID, Name: v.Name, Input: json.RawMessage(v.JSON.Input.Raw())})
		}
	}
	stop := StopEnd
	switch msg.StopReason {
	case anthropic.StopReasonToolUse:
		stop = StopToolUse
	case anthropic.StopReasonMaxTokens:
		stop = StopMaxTokens
	case anthropic.StopReasonRefusal:
		stop = StopRefusal
	}
	u := msg.Usage
	usage := Usage{
		InputTokens:      u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens,
		OutputTokens:     u.OutputTokens,
		CacheReadTokens:  u.CacheReadInputTokens,
		CacheWriteTokens: u.CacheCreationInputTokens,
	}
	return Reply{Message: out, Stop: stop, Usage: usage}, nil
}

// messages traduit la conversation : un message rendu par Claude lui est
// renvoyé tel quel (blocs de réflexion compris), les autres sont convertis.
func (anthropicProvider) messages(msgs []Message) []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, 0, len(msgs))
	for _, m := range msgs {
		if n, ok := m.Native.(anthropic.MessageParam); ok && m.Provider == "anthropic" {
			out = append(out, n)
			continue
		}
		var blocks []anthropic.ContentBlockParamUnion
		for _, b := range m.Content {
			switch b.Type {
			case TextBlock:
				if b.Text != "" {
					blocks = append(blocks, anthropic.NewTextBlock(b.Text))
				}
			case ToolUseBlock:
				var in any = map[string]any{}
				if len(b.Input) > 0 {
					_ = json.Unmarshal(b.Input, &in)
				}
				blocks = append(blocks, anthropic.NewToolUseBlock(b.ID, in, b.Name))
			case ToolResultBlock:
				blocks = append(blocks, anthropic.NewToolResultBlock(b.ToolUseID, b.Text, b.IsError))
			}
		}
		if len(blocks) == 0 {
			// Message réduit à des blocs de réflexion, perdus dans la forme
			// neutre : l'API refuse un message vide.
			blocks = append(blocks, anthropic.NewTextBlock("…"))
		}
		if m.Role == Assistant {
			out = append(out, anthropic.NewAssistantMessage(blocks...))
		} else {
			out = append(out, anthropic.NewUserMessage(blocks...))
		}
	}
	return out
}

var _ KeyChecker = anthropicProvider{}

// anthropicError rend une erreur de l'API lisible par l'utilisateur.
func anthropicError(err error) error {
	var apierr *anthropic.Error
	if !errors.As(err, &apierr) {
		if errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("Claude injoignable : %w", err)
	}
	msg := strings.ToLower(apierr.Error())
	switch apierr.StatusCode {
	case 400:
		switch {
		case strings.Contains(msg, "credit balance"):
			return errors.New("Crédits épuisés sur votre compte Anthropic : ajoutez-en (platform.claude.com › Facturation) ; abonnés Max ou Team : liez votre organisation pour recevoir vos crédits mensuels")
		case strings.Contains(msg, "prompt is too long") || strings.Contains(msg, "too many tokens") || strings.Contains(msg, "context window") || strings.Contains(msg, "context length"):
			return errors.New("conversation trop longue pour le modèle : commencez une « Nouvelle conversation »")
		}
	case 401:
		return errors.New("clé d'API refusée par Anthropic : vérifiez-la ou remplacez-la (bouton « Connexion »)")
	case 403:
		return errors.New("accès refusé par Anthropic (droits de la clé ou de l'organisation)")
	case 404:
		return errors.New("modèle inconnu d'Anthropic : choisissez-en un autre")
	case 413:
		return errors.New("conversation trop longue pour le modèle : commencez une « Nouvelle conversation »")
	case 429:
		return errors.New("limite d'utilisation atteinte chez Anthropic : réessayez dans un moment")
	case 529, 503:
		return errors.New("Claude est surchargé : réessayez dans un moment")
	}
	return fmt.Errorf("erreur de Claude (%d) : %s", apierr.StatusCode, apierr.Error())
}
