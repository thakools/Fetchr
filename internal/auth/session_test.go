package auth

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestStoreCreateGetDelete(t *testing.T) {
	s := NewStore(time.Minute, false)
	defer s.Close()

	w := httptest.NewRecorder()
	sess := s.Create(w, Identity{Subject: "u1", Username: "alice"})

	if got := w.Result().Cookies(); len(got) != 1 || got[0].Name != SessionCookie {
		t.Fatalf("expected a session cookie, got %#v", got)
	}
	if _, ok := s.Get(sess.ID); !ok {
		t.Fatal("session should be retrievable after Create")
	}

	evicted := make(chan string, 1)
	s.OnEvict(func(id string) { evicted <- id })
	s.Delete(sess.ID)

	if _, ok := s.Get(sess.ID); ok {
		t.Error("session should be gone after Delete")
	}
	select {
	case id := <-evicted:
		if id != sess.ID {
			t.Errorf("evict callback got %q, want %q", id, sess.ID)
		}
	case <-time.After(time.Second):
		t.Error("evict callback was not fired")
	}
}

func TestStoreExpiry(t *testing.T) {
	s := NewStore(time.Millisecond, false)
	defer s.Close()

	sess := s.Create(httptest.NewRecorder(), Identity{Subject: "u1"})
	time.Sleep(5 * time.Millisecond)
	if _, ok := s.Get(sess.ID); ok {
		t.Error("expired session should not be returned")
	}
}

func TestSafeReturn(t *testing.T) {
	cases := map[string]string{
		"/files/var":          "/files/var",
		"":                    "",
		"//evil.example.com":  "",
		"https://evil.com":    "",
		"javascript:alert(1)": "",
	}
	for in, want := range cases {
		if got := safeReturn(in); got != want {
			t.Errorf("safeReturn(%q) = %q, want %q", in, got, want)
		}
	}
}
