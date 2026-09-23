package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultTokenTTL = 24 * time.Hour
	issuer          = "paradox-auth"
)

// Claims is the JWT payload for Paradox Auth.
type Claims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func signingKey() ([]byte, error) {
	storeDir, err := storePath()
	if err != nil {
		return nil, err
	}
	keyPath := filepath.Join(storeDir, "jwt.key")

	if k := os.Getenv("PARADOX_JWT_SECRET"); k != "" {
		return []byte(k), nil
	}

	data, err := os.ReadFile(keyPath)
	if err == nil && len(data) >= 32 {
		return data, nil
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate jwt key: %w", err)
	}
	if err := os.MkdirAll(storeDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		return nil, fmt.Errorf("write jwt key: %w", err)
	}
	return key, nil
}

// IssueToken creates a signed JWT for the user.
func IssueToken(u *User, ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 {
		ttl = defaultTokenTTL
	}
	key, err := signingKey()
	if err != nil {
		return "", time.Time{}, err
	}
	exp := time.Now().UTC().Add(ttl)
	claims := Claims{
		UserID: u.ID,
		Email:  u.Email,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   u.ID,
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ID:        hex.EncodeToString(mustRand(8)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// ParseToken validates a JWT and returns claims.
func ParseToken(tokenStr string) (*Claims, error) {
	key, err := signingKey()
	if err != nil {
		return nil, err
	}
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return key, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func mustRand(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func sessionTokenPath() (string, error) {
	storeDir, err := storePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(storeDir, "session.token"), nil
}

func SaveSessionToken(token string) error {
	path, err := sessionTokenPath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token), 0o600)
}

func LoadSessionToken() (string, error) {
	path, err := sessionTokenPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ClearSessionToken() error {
	path, err := sessionTokenPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
