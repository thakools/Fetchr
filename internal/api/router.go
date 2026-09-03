// Package api wires HTTP routes onto the auth and SFTP layers.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/local/fetchr/internal/auth"
	"github.com/local/fetchr/internal/sftpconn"
)

type Server struct {
	sessions  *auth.Store
	pool      *sftpconn.Pool
	oidc      *auth.Provider
	skipLogin bool
	maxUpload int64
}

func New(sessions *auth.Store, pool *sftpconn.Pool, oidcProvider *auth.Provider, skipLogin bool, maxUpload int64) *Server {
	return &Server{sessions: sessions, pool: pool, oidc: oidcProvider, skipLogin: skipLogin, maxUpload: maxUpload}
}

// Routes returns the /api subtree. The SPA handler is mounted separately.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, securityHeaders)

	r.Route("/auth", func(r chi.Router) {
		r.Get("/me", s.handleMe)
		r.Get("/login", s.handleLogin)
		r.Get("/callback", s.handleCallback)
		r.Post("/logout", s.handleLogout)
	})

	r.Route("/sftp", func(r chi.Router) {
		r.Use(auth.Middleware(s.sessions, s.skipLogin))
		r.Post("/connect", s.handleConnect)
		r.Post("/disconnect", s.handleDisconnect)
		r.Get("/status", s.handleStatus)
		r.Get("/list", s.handleList)
		r.Get("/download", s.handleDownload)
		r.Post("/upload", s.handleUpload)
		r.Post("/mkdir", s.handleMkdir)
		r.Post("/rename", s.handleRename)
		r.Delete("/remove", s.handleRemove)
	})

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeErr(w, http.StatusNotFound, "not_found", "unknown endpoint")
	})
	return r
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
