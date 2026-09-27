package agent

import (
	"context"
	"fmt"
	"strings"
)

// FallbackAdapter tries primary, then secondary on transport/HTTP errors.
type FallbackAdapter struct {
	Primary   ModelAdapter
	Secondary ModelAdapter
	label     string
}

func (f *FallbackAdapter) Name() string {
	if f.label != "" {
		return f.label
	}
	return f.Primary.Name() + "|" + f.Secondary.Name()
}

func (f *FallbackAdapter) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	res, err := f.Primary.Complete(ctx, req)
	if err == nil {
		return res, nil
	}
	msg := strings.ToLower(err.Error())
	retryable := strings.Contains(msg, "connection") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "http 5") ||
		strings.Contains(msg, "http 429") ||
		strings.Contains(msg, "dial ") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "refused")
	if !retryable || f.Secondary == nil {
		return nil, err
	}
	res2, err2 := f.Secondary.Complete(ctx, req)
	if err2 != nil {
		return nil, fmt.Errorf("primary (%s): %v; fallback (%s): %w",
			f.Primary.Name(), err, f.Secondary.Name(), err2)
	}
	return res2, nil
}
