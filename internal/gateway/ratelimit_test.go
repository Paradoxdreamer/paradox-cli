package gateway

import "testing"

func TestLimiterAllowBurst(t *testing.T) {
	l := newLimiter(10, 3)
	if !l.Allow("a") || !l.Allow("a") || !l.Allow("a") {
		t.Fatal("burst should allow 3")
	}
	if l.Allow("a") {
		t.Fatal("4th should be denied")
	}
	if !l.Allow("b") {
		t.Fatal("other key should allow")
	}
}
