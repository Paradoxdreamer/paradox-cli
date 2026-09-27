package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// OmegaAdapter — free OmegaTech GET APIs.
//   GET {base}/api/ai/Qwen-Claude-Haiku?message=...
//   GET {base}/api/ai/Gpt-4-mini?message=...
// Env: PARADOX_OMEGA_BASE_URL (default https://omegatech-api.dixonomega.tech)
type OmegaAdapter struct {
	BaseURL  string
	Endpoint string
	Client   *http.Client
	name     string
}

func NewOmegaAdapter(endpoint string) *OmegaAdapter {
	base := strings.TrimRight(os.Getenv("PARADOX_OMEGA_BASE_URL"), "/")
	if base == "" {
		base = "https://omegatech-api.dixonomega.tech"
	}
	if endpoint == "" {
		endpoint = "Qwen-Claude-Haiku"
	}
	return &OmegaAdapter{
		BaseURL: base, Endpoint: endpoint,
		Client: &http.Client{Timeout: 60 * time.Second}, name: "omega:" + endpoint,
	}
}

func (a *OmegaAdapter) Name() string { return a.name }

func (a *OmegaAdapter) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	var b strings.Builder
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			b.WriteString("System: " + m.Content + "\n")
		case "user":
			b.WriteString("User: " + m.Content + "\n")
		case "assistant":
			b.WriteString("Assistant: " + m.Content + "\n")
		case "tool":
			b.WriteString("Tool " + m.Name + ": " + m.Content + "\n")
		}
	}
	if len(req.Tools) > 0 {
		b.WriteString("Available tools (text reply; limited tool calling on free API): ")
		for i, t := range req.Tools {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(t.Name)
		}
		b.WriteString("\n")
	}
	b.WriteString("Assistant:")
	msg := strings.TrimSpace(b.String())
	u := fmt.Sprintf("%s/api/ai/%s?message=%s&model=qwen", a.BaseURL, a.Endpoint, url.QueryEscape(msg))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	res, err := a.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("omega request: %w", err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("omega HTTP %d: %s", res.StatusCode, truncate(string(data), 300))
	}
	text := strings.TrimSpace(string(data))
	var wrap map[string]any
	if json.Unmarshal(data, &wrap) == nil {
		for _, k := range []string{"response", "message", "content", "text", "answer", "output"} {
			if v, ok := wrap[k].(string); ok && strings.TrimSpace(v) != "" {
				text = strings.TrimSpace(v)
				break
			}
		}
	}
	if text == "" {
		return nil, fmt.Errorf("omega: empty response")
	}
	return &CompletionResponse{Content: text}, nil
}
