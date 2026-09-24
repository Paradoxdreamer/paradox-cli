package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type ToolSpec struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions,omitempty"`
}

type ToolHandler func(ctx context.Context, argsJSON string) (string, error)

type ToolRegistry struct {
	specs    map[string]ToolSpec
	handlers map[string]ToolHandler
}

func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{specs: map[string]ToolSpec{}, handlers: map[string]ToolHandler{}}
	r.registerDefaults()
	return r
}

func (r *ToolRegistry) Register(spec ToolSpec, h ToolHandler) {
	r.specs[spec.Name] = spec
	r.handlers[spec.Name] = h
}

func (r *ToolRegistry) Specs() []ToolSpec {
	out := make([]ToolSpec, 0, len(r.specs))
	for _, s := range r.specs {
		out = append(out, s)
	}
	return out
}

func (r *ToolRegistry) Call(ctx context.Context, name, argsJSON string) (string, error) {
	h, ok := r.handlers[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	if argsJSON == "" {
		argsJSON = "{}"
	}
	return h(ctx, argsJSON)
}

func (r *ToolRegistry) registerDefaults() {
	r.Register(ToolSpec{Name: "echo_time", Description: "Returns the current UTC time"}, func(ctx context.Context, argsJSON string) (string, error) {
		return time.Now().UTC().Format(time.RFC3339), nil
	})
	r.Register(ToolSpec{Name: "calc", Description: "Evaluate a+b, a-b, a*b (integers)"}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Expression string `json:"expression"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
		expr := strings.ReplaceAll(args.Expression, " ", "")
		for _, op := range []string{"+", "-", "*"} {
			if i := strings.Index(expr, op); i > 0 {
				a, err1 := strconv.Atoi(expr[:i])
				b, err2 := strconv.Atoi(expr[i+1:])
				if err1 != nil || err2 != nil {
					return "", fmt.Errorf("calc expects integers")
				}
				var v int
				switch op {
				case "+":
					v = a + b
				case "-":
					v = a - b
				case "*":
					v = a * b
				}
				return strconv.Itoa(v), nil
			}
		}
		return "", fmt.Errorf("unsupported expression %q", args.Expression)
	})
	r.Register(ToolSpec{Name: "obs_track", Description: "Record observability track event"}, func(ctx context.Context, argsJSON string) (string, error) {
		var args struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		if args.Name == "" {
			args.Name = "agent.event"
		}
		return fmt.Sprintf("tracked:%s", args.Name), nil
	})
}
