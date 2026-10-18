package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
		DefaultModel: "claude-opus-5-5",
		KeyURL:       "https://console.anthropic.com/settings/keys",
		KeyHint:      "sk-ant-",
	}
}

func (p anthropicProvider) Turn(ctx context.Context, cred Credentials, req Request, emit func(Event)) (Reply, error) {
	if cred.Key == "" {
		return Reply{}, ErrNoKey
	}
	opts := []option.RequestOption{option.WithoutEnvironmentDefaults(), option.WithAPIKey(cred.Key)}
	if cred.Endpoint != "" {
		opts = append(opts, option.WithBaseURL(cred.Endpoint))
	}
	client := anthropic.NewClient(opts...)

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
	return Reply{Message: out, Stop: stop}, nil
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
		if m.Role == Assistant {
			out = append(out, anthropic.NewAssistantMessage(blocks...))
		} else {
			out = append(out, anthropic.NewUserMessage(blocks...))
		}
	}
	return out
}

// anthropicError rend une erreur de l'API lisible par l'utilisateur.
func anthropicError(err error) error {
	var apierr *anthropic.Error
	if !errors.As(err, &apierr) {
		if errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("Claude injoignable : %w", err)
	}
	switch apierr.StatusCode {
	case 401:
		return errors.New("clé d'API refusée par Anthropic : vérifiez-la (bouton « Clé d'API »)")
	case 403:
		return errors.New("accès refusé par Anthropic (droits de la clé ou de l'organisation)")
	case 404:
		return errors.New("modèle inconnu d'Anthropic : choisissez-en un autre")
	case 429:
		return errors.New("limite d'utilisation atteinte chez Anthropic : réessayez dans un moment")
	case 529, 503:
		return errors.New("Claude est surchargé : réessayez dans un moment")
	}
	return fmt.Errorf("erreur de Claude (%d) : %s", apierr.StatusCode, apierr.Error())
}
