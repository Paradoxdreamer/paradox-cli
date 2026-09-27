package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OpenAIAdapter talks to any OpenAI-compatible Chat Completions API.
// Env: PARADOX_LLM_BASE_URL, PARADOX_LLM_API_KEY, PARADOX_LLM_MODEL
type OpenAIAdapter struct {
	BaseURL string
	APIKey  string
	Model   string
	Client  *http.Client
	name    string
}

func NewOpenAIAdapter(model string) *OpenAIAdapter {
	base := strings.TrimRight(os.Getenv("PARADOX_LLM_BASE_URL"), "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	m := model
	if m == "" || m == "openai" {
		m = os.Getenv("PARADOX_LLM_MODEL")
	}
	if m == "" || m == "openai" {
		m = "gpt-4o-mini"
	}
	return &OpenAIAdapter{
		BaseURL: base, APIKey: os.Getenv("PARADOX_LLM_API_KEY"), Model: m,
		Client: &http.Client{Timeout: 90 * time.Second}, name: "openai:" + m,
	}
}

func (a *OpenAIAdapter) Name() string { return a.name }

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	Name       string        `json:"name,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Parameters  any    `json:"parameters"`
	} `json:"function"`
}

type oaiRequest struct {
	Model    string       `json:"model"`
	Messages []oaiMessage `json:"messages"`
	Tools    []oaiTool    `json:"tools,omitempty"`
}

type oaiResponse struct {
	Choices []struct {
		Message oaiMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *OpenAIAdapter) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	msgs := make([]oaiMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		om := oaiMessage{Role: m.Role, Content: m.Content, Name: m.Name}
		if m.Role == "tool" && m.Name != "" {
			om.ToolCallID = m.Name
		}
		msgs = append(msgs, om)
	}
	var tools []oaiTool
	for _, t := range req.Tools {
		ot := oaiTool{Type: "function"}
		ot.Function.Name = t.Name
		ot.Function.Description = t.Description
		ot.Function.Parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		tools = append(tools, ot)
	}
	raw, err := json.Marshal(oaiRequest{Model: a.Model, Messages: msgs, Tools: tools})
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.APIKey)
	}
	res, err := a.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	var parsed oaiResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("openai decode: %w (body=%s)", err, truncate(string(data), 200))
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("openai: %s", parsed.Error.Message)
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("openai HTTP %d: %s", res.StatusCode, truncate(string(data), 300))
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty choices")
	}
	msg := parsed.Choices[0].Message
	out := &CompletionResponse{Content: msg.Content}
	for _, tc := range msg.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments})
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
