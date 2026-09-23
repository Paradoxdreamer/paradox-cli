package flags

import (
	"hash/fnv"
	"strings"
	"time"
)

// Flag is a feature flag definition.
type Flag struct {
	Key          string            `json:"key"`
	Description  string            `json:"description,omitempty"`
	Enabled      bool              `json:"enabled"`
	Percentage   int               `json:"percentage"`
	Environments []string          `json:"environments,omitempty"`
	AllowUsers   []string          `json:"allow_users,omitempty"`
	DenyUsers    []string          `json:"deny_users,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Meta         map[string]string `json:"meta,omitempty"`
}

// Context is the evaluation context (who / where).
type Context struct {
	UserID      string
	Email       string
	Environment string
}

// Evaluation is the result of checking a flag.
type Evaluation struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}

// Evaluate returns whether the flag is on for this context.
func (f *Flag) Evaluate(ctx Context) Evaluation {
	if f == nil {
		return Evaluation{Enabled: false, Reason: "flag not found"}
	}
	ev := Evaluation{Key: f.Key}

	if !f.Enabled {
		ev.Reason = "disabled (kill switch)"
		return ev
	}

	if len(f.Environments) > 0 {
		env := strings.ToLower(strings.TrimSpace(ctx.Environment))
		if env == "" {
			env = "development"
		}
		ok := false
		for _, e := range f.Environments {
			if strings.ToLower(e) == env {
				ok = true
				break
			}
		}
		if !ok {
			ev.Reason = "environment not allowed: " + env
			return ev
		}
	}

	if matchesList(ctx, f.DenyUsers) {
		ev.Reason = "user denied"
		return ev
	}

	if len(f.AllowUsers) > 0 && matchesList(ctx, f.AllowUsers) {
		ev.Enabled = true
		ev.Reason = "user allow-listed"
		return ev
	}

	pct := f.Percentage
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	if pct >= 100 {
		ev.Enabled = true
		ev.Reason = "100% rollout"
		return ev
	}
	if pct <= 0 {
		ev.Reason = "0% rollout"
		return ev
	}

	bucket := hashBucket(f.Key, ctx)
	if bucket < pct {
		ev.Enabled = true
		ev.Reason = "percentage rollout"
		return ev
	}
	ev.Reason = "outside percentage rollout"
	return ev
}

func matchesList(ctx Context, list []string) bool {
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if ctx.UserID != "" && item == ctx.UserID {
			return true
		}
		if ctx.Email != "" && strings.EqualFold(item, ctx.Email) {
			return true
		}
	}
	return false
}

func hashBucket(flagKey string, ctx Context) int {
	id := ctx.UserID
	if id == "" {
		id = ctx.Email
	}
	if id == "" {
		id = "anonymous"
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(flagKey + ":" + id))
	return int(h.Sum32() % 100)
}
