// Package auth provides Keycloak OIDC login and server-side session tracking.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const SessionCookie = "fetchr_session"

type Identity struct {
	Subject  string `json:"subject"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
}

type Session struct {
	ID       string
	Identity Identity
	expires  time.Time
}

// Store holds login sessions in memory and evicts them once idle past ttl.
type Store struct {
	mu       sync.Mutex
	sessions map[string]*Session
	ttl      time.Duration
	secure   bool
	onEvict  func(id string)
	stop     chan struct{}
}

func NewStore(ttl time.Duration, secure bool) *Store {
	s := &Store{
		sessions: map[string]*Session{},
		ttl:      ttl,
		secure:   secure,
		stop:     make(chan struct{}),
	}
	go s.janitor()
	return s
}

// OnEvict registers a callback fired when a session is dropped, used to close its SFTP connection.
func (s *Store) OnEvict(fn func(id string)) { s.onEvict = fn }

func (s *Store) Create(w http.ResponseWriter, id Identity) *Session {
	sess := &Session{ID: newID(), Identity: id, expires: time.Now().Add(s.ttl)}
	s.mu.Lock()
	s.sessions[sess.ID] = sess
	s.mu.Unlock()
	s.SetCookie(w, sess.ID)
	return sess
}

// Get returns the session and refreshes its idle deadline.
func (s *Store) Get(id string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok || time.Now().After(sess.expires) {
		return nil, false
	}
	sess.expires = time.Now().Add(s.ttl)
	return sess, true
}

func (s *Store) Delete(id string) {
	s.mu.Lock()
	_, existed := s.sessions[id]
	delete(s.sessions, id)
	s.mu.Unlock()
	if existed && s.onEvict != nil {
		s.onEvict(id)
	}
}

func (s *Store) SetCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Store) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Store) Close() { close(s.stop) }

func (s *Store) janitor() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.mu.Lock()
			var expired []string
			for id, sess := range s.sessions {
				if time.Now().After(sess.expires) {
					expired = append(expired, id)
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
			for _, id := range expired {
				if s.onEvict != nil {
					s.onEvict(id)
				}
			}
		}
	}
}

func newID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
