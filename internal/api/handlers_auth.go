package api

import (
	"net/http"

	"github.com/local/fetchr/internal/auth"
)

type meResponse struct {
	Authenticated bool           `json:"authenticated"`
	SkipLogin     bool           `json:"skipLogin"`
	User          *auth.Identity `json:"user,omitempty"`
}

// handleMe is public so the SPA can decide between the landing page and the app.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	resp := meResponse{SkipLogin: s.skipLogin}
	if c, err := r.Cookie(auth.SessionCookie); err == nil {
		if sess, ok := s.sessions.Get(c.Value); ok {
			resp.Authenticated = true
			id := sess.Identity
			resp.User = &id
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		s.sessions.Create(w, auth.Identity{Subject: "local", Username: "local", Name: "Local User"})
		http.Redirect(w, r, "/connect", http.StatusFound)
		return
	}
	s.oidc.Login(w, r)
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	s.oidc.Callback(w, r)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.SessionCookie); err == nil {
		s.sessions.Delete(c.Value)
	}
	s.sessions.ClearCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
