package agent

import (
	"context"
	"strings"
	"testing"
)

func TestEchoPlain(t *testing.T) {
	a := EchoAdapter{}
	res, err := a.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "hello") {
		t.Fatalf("content=%q", res.Content)
	}
	if len(res.ToolCalls) != 0 {
		t.Fatal("unexpected tool calls")
	}
}

func TestEchoToolCall(t *testing.T) {
	a := EchoAdapter{}
	res, err := a.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "use tool echo_time"}},
		Tools:    []ToolSpec{{Name: "echo_time", Description: "time"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "echo_time" {
		t.Fatalf("%+v", res.ToolCalls)
	}
}

func TestResolveAdapterEcho(t *testing.T) {
	ad, err := ResolveAdapter("echo")
	if err != nil {
		t.Fatal(err)
	}
	if ad.Name() != "echo" {
		t.Fatal(ad.Name())
	}
}
