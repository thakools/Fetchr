package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	oidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const stateCookie = "fetchr_oauth"
const stateTTL = 10 * time.Minute

// Provider wraps the Keycloak OIDC endpoints and the authorization code + PKCE flow.
type Provider struct {
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
	secret   []byte
	secure   bool
	store    *Store
}

func NewProvider(ctx context.Context, issuer, clientID, clientSecret, redirectURL string, scopes []string, secret string, secure bool, store *Store) (*Provider, error) {
	p, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery for %q: %w", issuer, err)
	}
	return &Provider{
		verifier: p.Verifier(&oidc.Config{ClientID: clientID}),
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     p.Endpoint(),
			RedirectURL:  redirectURL,
			Scopes:       scopes,
		},
		secret: []byte(secret),
		secure: secure,
		store:  store,
	}, nil
}

type authState struct {
	State    string `json:"s"`
	Verifier string `json:"v"`
	Return   string `json:"r"`
	Expires  int64  `json:"e"`
}

func (p *Provider) Login(w http.ResponseWriter, r *http.Request) {
	st := authState{
		State:    randString(),
		Verifier: oauth2.GenerateVerifier(),
		Return:   safeReturn(r.URL.Query().Get("return")),
		Expires:  time.Now().Add(stateTTL).Unix(),
	}
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    p.seal(st),
		Path:     "/",
		MaxAge:   int(stateTTL.Seconds()),
		HttpOnly: true,
		Secure:   p.secure,
		SameSite: http.SameSiteLaxMode,
	})
	url := p.oauth.AuthCodeURL(st.State, oauth2.AccessTypeOnline, oauth2.S256ChallengeOption(st.Verifier))
	http.Redirect(w, r, url, http.StatusFound)
}

func (p *Provider) Callback(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(stateCookie)
	if err != nil {
		http.Error(w, "missing login state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: p.secure, SameSite: http.SameSiteLaxMode})

	st, ok := p.open(c.Value)
	if !ok || time.Now().Unix() > st.Expires {
		http.Error(w, "invalid or expired login state", http.StatusBadRequest)
		return
	}
	if subtleEqual(st.State, r.URL.Query().Get("state")) != true {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Error(w, "login failed: "+errParam, http.StatusUnauthorized)
		return
	}

	tok, err := p.oauth.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.VerifierOption(st.Verifier))
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusUnauthorized)
		return
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in response", http.StatusUnauthorized)
		return
	}
	idToken, err := p.verifier.Verify(r.Context(), rawID)
	if err != nil {
		http.Error(w, "id_token verification failed", http.StatusUnauthorized)
		return
	}

	var claims struct {
		Sub      string `json:"sub"`
		Username string `json:"preferred_username"`
		Email    string `json:"email"`
		Name     string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "cannot read claims", http.StatusUnauthorized)
		return
	}
	username := claims.Username
	if username == "" {
		username = claims.Email
	}
	p.store.Create(w, Identity{Subject: claims.Sub, Username: username, Email: claims.Email, Name: claims.Name})

	dest := st.Return
	if dest == "" {
		dest = "/connect"
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

func (p *Provider) seal(st authState) string {
	payload, _ := json.Marshal(st)
	mac := hmac.New(sha256.New, p.secret)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (p *Provider) open(v string) (authState, bool) {
	var st authState
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 {
		return st, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return st, false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return st, false
	}
	mac := hmac.New(sha256.New, p.secret)
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return st, false
	}
	if err := json.Unmarshal(payload, &st); err != nil {
		return st, false
	}
	return st, true
}

func subtleEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// safeReturn only allows same-site relative paths, preventing open redirects.
func safeReturn(v string) string {
	if v == "" || !strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") {
		return ""
	}
	return v
}

func randString() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
