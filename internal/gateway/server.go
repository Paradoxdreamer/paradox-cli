package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/paradox-cloud/paradox/internal/auth"
	"github.com/paradox-cloud/paradox/internal/flags"
	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/queue"
)

type Server struct {
	Addr       string
	Keys       *keyStore
	Limiter    *limiter
	RequireKey bool
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/whoami", s.handleWhoami)
	mux.HandleFunc("/v1/flags/eval", s.handleFlagsEval)
	mux.HandleFunc("/v1/queue/enqueue", s.handleQueueEnqueue)
	mux.HandleFunc("/v1/queue/status", s.handleQueueStatus)
	return s.middleware(mux)
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &wrapWriter{ResponseWriter: w, status: 200}
		w.Header().Set("X-Paradox-Gateway", "1")

		if r.URL.Path != "/health" {
			if s.RequireKey {
				key := extractAPIKey(r)
				if key == "" {
					writeErr(ww, http.StatusUnauthorized, "missing API key (header X-API-Key or Authorization: Bearer pk_…)")
					s.logReq(r, ww.status, start, "")
					return
				}
				ak, ok := s.Keys.Validate(key)
				if !ok {
					writeErr(ww, http.StatusUnauthorized, "invalid API key")
					s.logReq(r, ww.status, start, "")
					return
				}
				r = r.WithContext(context.WithValue(r.Context(), ctxKeyAPIKey, ak))
				if !s.Limiter.Allow(ak.ID) {
					writeErr(ww, http.StatusTooManyRequests, "rate limit exceeded")
					s.logReq(r, ww.status, start, ak.Prefix)
					return
				}
			} else if !s.Limiter.Allow(r.RemoteAddr) {
				writeErr(ww, http.StatusTooManyRequests, "rate limit exceeded")
				s.logReq(r, ww.status, start, "")
				return
			}
		}
		next.ServeHTTP(ww, r)
		prefix := ""
		if ak, ok := r.Context().Value(ctxKeyAPIKey).(*APIKey); ok && ak != nil {
			prefix = ak.Prefix
		}
		s.logReq(r, ww.status, start, prefix)
	})
}

func (s *Server) logReq(r *http.Request, status int, start time.Time, keyPrefix string) {
	logging.Info("gateway request", "method", r.Method, "path", r.URL.Path, "status", status,
		"duration_ms", time.Since(start).Milliseconds(), "key", keyPrefix, "remote", r.RemoteAddr)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "paradox-gateway", "time": time.Now().UTC().Format(time.RFC3339)})
}

func (s *Server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	token := extractBearer(r)
	if token == "" {
		writeErr(w, http.StatusUnauthorized, "missing Bearer JWT")
		return
	}
	claims, err := auth.ParseToken(token)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": claims.UserID, "email": claims.Email, "role": claims.Role})
}

func (s *Server) handleFlagsEval(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "GET or POST")
		return
	}
	key := r.URL.Query().Get("key")
	user := r.URL.Query().Get("user")
	email := r.URL.Query().Get("email")
	env := r.URL.Query().Get("env")
	if env == "" {
		env = "development"
	}
	if key == "" {
		writeErr(w, http.StatusBadRequest, "query param key is required")
		return
	}
	store, err := openFlagsStore()
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	f, err := store.Get(key)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	ev := f.Evaluate(flags.Context{UserID: user, Email: email, Environment: env})
	writeJSON(w, http.StatusOK, ev)
}

func (s *Server) handleQueueEnqueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST")
		return
	}
	var body struct {
		Type        string         `json:"type"`
		Payload     map[string]any `json:"payload"`
		MaxAttempts int            `json:"max_attempts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Type == "" {
		writeErr(w, http.StatusBadRequest, "type is required")
		return
	}
	store, err := openQueueStore()
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	id := fmt.Sprintf("job_%d", time.Now().UnixNano())
	j := &queue.Job{ID: id, Type: body.Type, Payload: body.Payload, MaxAttempts: body.MaxAttempts}
	if err := store.Enqueue(j); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "type": body.Type, "status": "pending"})
}

func (s *Server) handleQueueStatus(w http.ResponseWriter, r *http.Request) {
	store, err := openQueueStore()
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	counts, err := store.Counts()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]int{}
	for k, v := range counts {
		out[string(k)] = v
	}
	writeJSON(w, http.StatusOK, out)
}

type ctxKey int

const ctxKeyAPIKey ctxKey = 1

type wrapWriter struct {
	http.ResponseWriter
	status int
}

func (w *wrapWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func extractAPIKey(r *http.Request) string {
	if k := r.Header.Get("X-API-Key"); k != "" {
		return k
	}
	authz := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		tok := strings.TrimSpace(authz[7:])
		if strings.HasPrefix(tok, "pk_") {
			return tok
		}
	}
	return ""
}

func extractBearer(r *http.Request) string {
	authz := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		tok := strings.TrimSpace(authz[7:])
		if !strings.HasPrefix(tok, "pk_") {
			return tok
		}
	}
	return ""
}
