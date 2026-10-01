package auth

import (
	"testing"
	"time"
)

func TestIssueAndParseToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PARADOX_DATA_DIR", dir)
	t.Setenv("PARADOX_JWT_SECRET", "test-secret-for-jwt-unit-tests-32b")

	u := &User{ID: "usr_1", Email: "a@b.com", Role: "admin"}
	tok, exp, err := IssueToken(u, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" || exp.Before(time.Now()) {
		t.Fatalf("token=%q exp=%v", tok, exp)
	}
	claims, err := ParseToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "usr_1" || claims.Email != "a@b.com" || claims.Role != "admin" {
		t.Fatalf("%+v", claims)
	}
}
