package auth

import (
	"context"
	"encoding/json"
	"net/http"
)

type ctxKey int

const sessionKey ctxKey = iota

// Middleware rejects unauthenticated API calls. When skipLogin is set it mints a
// local session on first contact so the app is usable without Keycloak.
func Middleware(store *Store, skipLogin bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var sess *Session
			if c, err := r.Cookie(SessionCookie); err == nil {
				if s, ok := store.Get(c.Value); ok {
					sess = s
				}
			}
			if sess == nil {
				if !skipLogin {
					writeUnauthorized(w)
					return
				}
				sess = store.Create(w, Identity{Subject: "local", Username: "local", Name: "Local User"})
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey, sess)))
		})
	}
}

func FromContext(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(sessionKey).(*Session)
	return s, ok
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": "unauthenticated", "message": "login required"})
}
