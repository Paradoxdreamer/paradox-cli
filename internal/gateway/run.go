package gateway

import (
	"context"
	"net/http"
	"time"

	"github.com/paradox-cloud/paradox/internal/logging"
)

// RunServer starts the HTTP gateway and blocks until ctx is cancelled.
func RunServer(ctx context.Context, addr string, requireKey bool, rps int) error {
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if rps <= 0 {
		rps = 30
	}
	root, err := gatewayRoot()
	if err != nil {
		return err
	}
	ks, err := newKeyStore(root)
	if err != nil {
		return err
	}
	srv := &Server{
		Addr: addr, Keys: ks, Limiter: newLimiter(rps, rps*2), RequireKey: requireKey,
	}
	httpSrv := &http.Server{
		Addr: srv.Addr, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()
	logging.Info("gateway starting", "addr", addr, "require_key", requireKey)
	err = httpSrv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func AddrHost(addr string) string { return addrHost(addr) }

func EnsureReady() (string, error) {
	root, err := gatewayRoot()
	if err != nil {
		return "", err
	}
	if _, err := newKeyStore(root); err != nil {
		return "", err
	}
	return root, nil
}
