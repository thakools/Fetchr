// Package config parses runtime configuration from flags with SFTPWEB_* env fallbacks.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr      string
	LogLevel  string
	SkipLogin bool

	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	OIDCScopes       []string

	SessionSecret string
	SessionTTL    time.Duration
	CookieSecure  bool

	MaxUploadSize  int64
	AllowedHosts   []string
	MaxConnections int
	DialTimeout    time.Duration

	InsecureHostKey bool
}

// Parse reads args (excluding program name) and the process environment.
func Parse(args []string) (*Config, error) {
	fs := flag.NewFlagSet("sftpweb", flag.ContinueOnError)
	c := &Config{}

	fs.StringVar(&c.Addr, "addr", env("ADDR", ":8080"), "HTTP listen address")
	fs.StringVar(&c.LogLevel, "log-level", env("LOG_LEVEL", "info"), "log level: debug, info, warn, error")
	fs.BoolVar(&c.SkipLogin, "skip-login", envBool("SKIP_LOGIN", false), "bypass Keycloak login (testing only)")

	fs.StringVar(&c.OIDCIssuer, "oidc-issuer", env("OIDC_ISSUER", ""), "Keycloak realm issuer URL")
	fs.StringVar(&c.OIDCClientID, "oidc-client-id", env("OIDC_CLIENT_ID", ""), "OIDC client id")
	fs.StringVar(&c.OIDCClientSecret, "oidc-client-secret", env("OIDC_CLIENT_SECRET", ""), "OIDC client secret")
	fs.StringVar(&c.OIDCRedirectURL, "oidc-redirect-url", env("OIDC_REDIRECT_URL", ""), "OIDC redirect URL, e.g. https://host/api/auth/callback")
	scopes := fs.String("oidc-scopes", env("OIDC_SCOPES", "openid,profile,email"), "comma separated OIDC scopes")

	fs.StringVar(&c.SessionSecret, "session-secret", env("SESSION_SECRET", ""), "32+ byte secret for signing session cookies (random if empty)")
	fs.DurationVar(&c.SessionTTL, "session-ttl", envDuration("SESSION_TTL", 30*time.Minute), "idle lifetime of a session and its SFTP connection")
	fs.BoolVar(&c.CookieSecure, "cookie-secure", envBool("COOKIE_SECURE", true), "set the Secure attribute on cookies")

	maxUpload := fs.String("max-upload-size", env("MAX_UPLOAD_SIZE", "5GiB"), "maximum single upload size, e.g. 512MiB")
	hosts := fs.String("allowed-hosts", env("ALLOWED_HOSTS", ""), "comma separated allowlist of SFTP hosts; empty allows any")
	fs.IntVar(&c.MaxConnections, "max-connections", envInt("MAX_CONNECTIONS", 200), "maximum concurrent SFTP connections")
	fs.DurationVar(&c.DialTimeout, "dial-timeout", envDuration("DIAL_TIMEOUT", 10*time.Second), "SFTP dial timeout")
	fs.BoolVar(&c.InsecureHostKey, "insecure-host-key", envBool("INSECURE_HOST_KEY", true), "skip SSH host key verification")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	c.OIDCScopes = splitList(*scopes)
	c.AllowedHosts = splitList(*hosts)

	n, err := parseSize(*maxUpload)
	if err != nil {
		return nil, fmt.Errorf("max-upload-size: %w", err)
	}
	c.MaxUploadSize = n

	if c.SessionSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		c.SessionSecret = hex.EncodeToString(b)
	}

	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	if c.SkipLogin {
		return nil
	}
	missing := []string{}
	if c.OIDCIssuer == "" {
		missing = append(missing, "oidc-issuer")
	}
	if c.OIDCClientID == "" {
		missing = append(missing, "oidc-client-id")
	}
	if c.OIDCRedirectURL == "" {
		missing = append(missing, "oidc-redirect-url")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required flags %s (or pass --skip-login)", strings.Join(missing, ", "))
	}
	return nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv("SFTPWEB_" + key); ok {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, err := strconv.ParseBool(env(key, "")); err == nil {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(env(key, "")); err == nil {
		return v
	}
	return def
}

func splitList(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

var sizeUnits = []struct {
	suffix string
	mult   int64
}{
	{"GIB", 1 << 30}, {"MIB", 1 << 20}, {"KIB", 1 << 10},
	{"GB", 1e9}, {"MB", 1e6}, {"KB", 1e3},
	{"G", 1 << 30}, {"M", 1 << 20}, {"K", 1 << 10}, {"B", 1},
}

func parseSize(s string) (int64, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	for _, u := range sizeUnits {
		if strings.HasSuffix(s, u.suffix) {
			n, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, u.suffix)), 64)
			if err != nil {
				return 0, err
			}
			return int64(n * float64(u.mult)), nil
		}
	}
	return strconv.ParseInt(s, 10, 64)
}
