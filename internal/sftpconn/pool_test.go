package sftpconn

import (
	"testing"
	"time"
)

func TestHostAllowed(t *testing.T) {
	open := &Pool{opts: Options{}}
	if !open.hostAllowed("anything.example.com") {
		t.Error("an empty allowlist should permit any host")
	}

	restricted := &Pool{opts: Options{AllowedHosts: []string{"sftp.example.com"}}}
	if !restricted.hostAllowed("sftp.example.com") {
		t.Error("listed host should be allowed")
	}
	if restricted.hostAllowed("evil.example.com") {
		t.Error("unlisted host should be denied")
	}
}

func TestEvictIdle(t *testing.T) {
	p := &Pool{conns: map[string]*Conn{}, opts: Options{IdleTimeout: time.Minute}}
	now := time.Now()
	p.conns["fresh"] = &Conn{lastUsed: now}
	p.conns["stale"] = &Conn{lastUsed: now.Add(-2 * time.Minute)}

	if n := p.evictIdle(now); n != 1 {
		t.Fatalf("evicted %d connections, want 1", n)
	}
	if _, ok := p.conns["stale"]; ok {
		t.Error("stale connection should have been evicted")
	}
	if _, ok := p.conns["fresh"]; !ok {
		t.Error("fresh connection should have been kept")
	}
}

func TestGetReportsNotConnected(t *testing.T) {
	p := &Pool{conns: map[string]*Conn{}}
	if _, err := p.Get("nobody"); err != ErrNotConnected {
		t.Errorf("Get on an unknown session = %v, want ErrNotConnected", err)
	}
}
