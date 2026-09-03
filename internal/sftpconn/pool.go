// Package sftpconn keeps one live SFTP connection per login session.
package sftpconn

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	ErrNotConnected = errors.New("not connected")
	ErrHostDenied   = errors.New("host not allowed")
	ErrPoolFull     = errors.New("connection limit reached")
)

// Conn is a single SFTP session. Callers must hold Lock while using Client.
type Conn struct {
	sync.Mutex
	Client *sftp.Client
	ssh    *ssh.Client

	Host string
	Port int
	User string
	Home string

	lastUsed time.Time
}

func (c *Conn) close() {
	if c.Client != nil {
		_ = c.Client.Close()
	}
	if c.ssh != nil {
		_ = c.ssh.Close()
	}
}

type Options struct {
	IdleTimeout     time.Duration
	DialTimeout     time.Duration
	MaxConnections  int
	AllowedHosts    []string
	InsecureHostKey bool
}

type Pool struct {
	mu    sync.Mutex
	conns map[string]*Conn
	opts  Options
	stop  chan struct{}
}

func NewPool(opts Options) *Pool {
	p := &Pool{conns: map[string]*Conn{}, opts: opts, stop: make(chan struct{})}
	go p.janitor()
	return p
}

type Credentials struct {
	Host     string
	Port     int
	User     string
	Password string
}

// Connect replaces any existing connection for the session with a new one.
func (p *Pool) Connect(sessionID string, cr Credentials) (*Conn, error) {
	if !p.hostAllowed(cr.Host) {
		return nil, ErrHostDenied
	}
	p.mu.Lock()
	if _, exists := p.conns[sessionID]; !exists && p.opts.MaxConnections > 0 && len(p.conns) >= p.opts.MaxConnections {
		p.mu.Unlock()
		return nil, ErrPoolFull
	}
	p.mu.Unlock()

	// Host keys are not pinned in v1; see README for the trade-off.
	if !p.opts.InsecureHostKey {
		return nil, errors.New("host key verification is enabled but no known_hosts source is configured")
	}

	cfg := &ssh.ClientConfig{
		User:            cr.User,
		Auth:            []ssh.AuthMethod{ssh.Password(cr.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         p.opts.DialTimeout,
	}
	addr := net.JoinHostPort(cr.Host, strconv.Itoa(cr.Port))
	sshClient, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", addr, err)
	}
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("start sftp subsystem: %w", err)
	}
	home, err := sftpClient.Getwd()
	if err != nil || home == "" {
		home = "/"
	}

	c := &Conn{Client: sftpClient, ssh: sshClient, Host: cr.Host, Port: cr.Port, User: cr.User, Home: home, lastUsed: time.Now()}

	p.mu.Lock()
	if old := p.conns[sessionID]; old != nil {
		go old.close()
	}
	p.conns[sessionID] = c
	p.mu.Unlock()
	return c, nil
}

func (p *Pool) Get(sessionID string) (*Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.conns[sessionID]
	if !ok {
		return nil, ErrNotConnected
	}
	c.lastUsed = time.Now()
	return c, nil
}

func (p *Pool) Disconnect(sessionID string) {
	p.mu.Lock()
	c := p.conns[sessionID]
	delete(p.conns, sessionID)
	p.mu.Unlock()
	if c != nil {
		c.close()
	}
}

func (p *Pool) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.conns)
}

func (p *Pool) Close() {
	close(p.stop)
	p.mu.Lock()
	conns := p.conns
	p.conns = map[string]*Conn{}
	p.mu.Unlock()
	for _, c := range conns {
		c.close()
	}
}

func (p *Pool) hostAllowed(host string) bool {
	if len(p.opts.AllowedHosts) == 0 {
		return true
	}
	for _, h := range p.opts.AllowedHosts {
		if h == host {
			return true
		}
	}
	return false
}

func (p *Pool) evictIdle(now time.Time) int {
	if p.opts.IdleTimeout <= 0 {
		return 0
	}
	p.mu.Lock()
	var dead []*Conn
	for id, c := range p.conns {
		if now.Sub(c.lastUsed) > p.opts.IdleTimeout {
			dead = append(dead, c)
			delete(p.conns, id)
		}
	}
	p.mu.Unlock()
	for _, c := range dead {
		c.close()
	}
	return len(dead)
}

func (p *Pool) janitor() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-p.stop:
			return
		case now := <-t.C:
			p.evictIdle(now)
		}
	}
}
