package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/paradox-cloud/paradox/internal/observability"
)

type Agent struct {
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	SystemPrompt string    `json:"system_prompt"`
	Model        string    `json:"model"`
	Tools        []string  `json:"tools,omitempty"`
	MaxSteps     int       `json:"max_steps"`
	CreatedAt    time.Time `json:"created_at"`
}

type RunResult struct {
	Agent     string    `json:"agent"`
	Input     string    `json:"input"`
	Output    string    `json:"output"`
	Steps     int       `json:"steps"`
	ToolCalls int       `json:"tool_calls"`
	Messages  []Message `json:"messages"`
	Model     string    `json:"model"`
	Duration  string    `json:"duration"`
}

type Runtime struct {
	Store *Store
	Tools *ToolRegistry
	mu    sync.Mutex
}

func NewRuntime(root string) (*Runtime, error) {
	st, err := NewStore(root)
	if err != nil {
		return nil, err
	}
	return &Runtime{Store: st, Tools: NewToolRegistry()}, nil
}

func (rt *Runtime) Run(ctx context.Context, agentName, userInput string) (*RunResult, error) {
	start := time.Now()
	a, err := rt.Store.Get(agentName)
	if err != nil {
		return nil, err
	}
	adapter, err := ResolveAdapter(a.Model)
	if err != nil {
		return nil, err
	}
	maxSteps := a.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 5
	}
	msgs := []Message{
		{Role: "system", Content: a.SystemPrompt},
		{Role: "user", Content: userInput},
	}
	tools := rt.allowedTools(a)
	toolCalls := 0
	steps := 0
	var final string
	for steps < maxSteps {
		steps++
		resp, err := adapter.Complete(ctx, CompletionRequest{Messages: msgs, Tools: tools})
		if err != nil {
			return nil, err
		}
		if len(resp.ToolCalls) == 0 {
			final = resp.Content
			msgs = append(msgs, Message{Role: "assistant", Content: final})
			break
		}
		msgs = append(msgs, Message{Role: "assistant", Content: fmt.Sprintf("(calling tools: %d)", len(resp.ToolCalls))})
		for _, tc := range resp.ToolCalls {
			toolCalls++
			out, callErr := rt.Tools.Call(ctx, tc.Name, tc.Arguments)
			if callErr != nil {
				out = fmt.Sprintf("error: %v", callErr)
			}
			msgs = append(msgs, Message{Role: "tool", Name: tc.Name, Content: out})
			_ = observability.Track("agent.tool_call", map[string]any{"agent": agentName, "tool": tc.Name})
		}
	}
	if final == "" {
		final = "(no final response — max steps reached)"
	}
	result := &RunResult{
		Agent: agentName, Input: userInput, Output: final, Steps: steps,
		ToolCalls: toolCalls, Messages: msgs, Model: adapter.Name(), Duration: time.Since(start).String(),
	}
	_ = rt.Store.SaveRun(result)
	_ = observability.Track("agent.run", map[string]any{"agent": agentName, "steps": steps, "tool_calls": toolCalls})
	return result, nil
}

func (rt *Runtime) allowedTools(a *Agent) []ToolSpec {
	all := rt.Tools.Specs()
	if len(a.Tools) == 0 {
		return all
	}
	allow := map[string]bool{}
	for _, n := range a.Tools {
		allow[n] = true
	}
	var out []ToolSpec
	for _, s := range all {
		if allow[s.Name] {
			out = append(out, s)
		}
	}
	return out
}

type Store struct {
	root string
	mu   sync.Mutex
}

func NewStore(root string) (*Store, error) {
	for _, d := range []string{"agents", "runs"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return nil, err
		}
	}
	return &Store{root: root}, nil
}

func (s *Store) Put(a *Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.Name == "" {
		return fmt.Errorf("agent name required")
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.SystemPrompt == "" {
		a.SystemPrompt = "You are a helpful Paradox agent."
	}
	if a.Model == "" {
		a.Model = "echo"
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.root, "agents", a.Name+".json"), data, 0o644)
}

func (s *Store) Get(name string) (*Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(s.root, "agents", name+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("agent %q not found", name)
		}
		return nil, err
	}
	var a Agent
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) List() ([]Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, "agents"))
	if err != nil {
		return nil, err
	}
	var out []Agent
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.root, "agents", e.Name()))
		if err != nil {
			continue
		}
		var a Agent
		if json.Unmarshal(data, &a) == nil {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *Store) SaveRun(r *RunResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := fmt.Sprintf("run_%d", time.Now().UnixNano())
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.root, "runs", id+".json"), data, 0o644)
}

func agentRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".paradox")
	if d := os.Getenv("PARADOX_DATA_DIR"); d != "" {
		base = d
	}
	return filepath.Join(base, "agent"), nil
}
