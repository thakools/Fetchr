package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/local/sftpweb/internal/api"
	"github.com/local/sftpweb/internal/auth"
	"github.com/local/sftpweb/internal/config"
	"github.com/local/sftpweb/internal/sftpconn"
	"github.com/local/sftpweb/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sessions := auth.NewStore(cfg.SessionTTL, cfg.CookieSecure)
	defer sessions.Close()

	pool := sftpconn.NewPool(sftpconn.Options{
		IdleTimeout:     cfg.SessionTTL,
		DialTimeout:     cfg.DialTimeout,
		MaxConnections:  cfg.MaxConnections,
		AllowedHosts:    cfg.AllowedHosts,
		InsecureHostKey: cfg.InsecureHostKey,
	})
	defer pool.Close()
	sessions.OnEvict(pool.Disconnect)

	var provider *auth.Provider
	if cfg.SkipLogin {
		slog.Warn("--skip-login is enabled; every visitor gets a local session without authentication")
	} else {
		provider, err = auth.NewProvider(ctx, cfg.OIDCIssuer, cfg.OIDCClientID, cfg.OIDCClientSecret,
			cfg.OIDCRedirectURL, cfg.OIDCScopes, cfg.SessionSecret, cfg.CookieSecure, sessions)
		if err != nil {
			return err
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", api.New(sessions, pool, provider, cfg.SkipLogin, cfg.MaxUploadSize).Routes()))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	mux.Handle("/", web.Handler())

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr, "skipLogin", cfg.SkipLogin)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func parseLevel(s string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return l
}
