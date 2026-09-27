package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type CompletionRequest struct {
	Messages []Message
	Tools    []ToolSpec
}

type CompletionResponse struct {
	Content   string
	ToolCalls []ToolCall
}

type ModelAdapter interface {
	Name() string
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
}

type EchoAdapter struct{}

func (EchoAdapter) Name() string { return "echo" }

func (EchoAdapter) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	var lastUser string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUser = req.Messages[i].Content
			break
		}
	}
	lower := strings.ToLower(lastUser)
	if len(req.Messages) > 0 && req.Messages[len(req.Messages)-1].Role == "tool" {
		last := req.Messages[len(req.Messages)-1]
		return &CompletionResponse{Content: fmt.Sprintf("Tool %s returned: %s", last.Name, last.Content)}, nil
	}
	for _, t := range req.Tools {
		if strings.Contains(lower, "use tool "+strings.ToLower(t.Name)) || strings.Contains(lower, "tool:"+strings.ToLower(t.Name)) {
			args := "{}"
			if t.Name == "calc" {
				args = `{"expression":"1+1"}`
				if i := strings.Index(lower, "calc"); i >= 0 {
					rest := strings.TrimSpace(lastUser[i+4:])
					rest = strings.TrimPrefix(rest, ":")
					rest = strings.TrimSpace(rest)
					if rest != "" {
						args = fmt.Sprintf(`{"expression":%q}`, rest)
					}
				}
			}
			return &CompletionResponse{ToolCalls: []ToolCall{{ID: "call_echo_1", Name: t.Name, Arguments: args}}}, nil
		}
	}
	if lastUser == "" {
		return &CompletionResponse{Content: "I'm the echo model (local). Ask me something, or say: use tool <name>"}, nil
	}
	return &CompletionResponse{Content: fmt.Sprintf("[echo] You said: %s\n(Tip: say “use tool echo_time” or “use tool calc 2+3”.)", lastUser)}, nil
}

// ResolveAdapter returns a model by name.
//
//   echo | local              — offline stub
//   openai                    — OpenAI-compatible (PARADOX_LLM_*)
//   omega | qwen | free       — OmegaTech free Qwen → Gpt-4-mini fallback
//   gpt-4-mini | omega-mini   — OmegaTech free Gpt-4-mini only
func ResolveAdapter(name string) (ModelAdapter, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "", "echo", "local":
		return EchoAdapter{}, nil
	case "openai":
		return NewOpenAIAdapter(""), nil
	case "omega", "qwen", "free", "qwen-claude-haiku":
		return &FallbackAdapter{
			Primary:   NewOmegaAdapter("Qwen-Claude-Haiku"),
			Secondary: NewOmegaAdapter("Gpt-4-mini"),
			label:     "omega:Qwen-Claude-Haiku|Gpt-4-mini",
		}, nil
	case "gpt-4-mini", "omega-mini", "gpt4mini":
		return NewOmegaAdapter("Gpt-4-mini"), nil
	default:
		if os.Getenv("PARADOX_LLM_API_KEY") != "" || os.Getenv("PARADOX_LLM_BASE_URL") != "" {
			return NewOpenAIAdapter(name), nil
		}
		return nil, fmt.Errorf("unknown model %q — use: echo, openai, omega, gpt-4-mini (or set PARADOX_LLM_API_KEY)", name)
	}
}
