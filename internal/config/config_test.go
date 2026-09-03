package config

import (
	"testing"
	"time"
)

func TestParseDefaultsRequireOIDCUnlessSkipLogin(t *testing.T) {
	if _, err := Parse([]string{}); err == nil {
		t.Fatal("expected an error when neither OIDC config nor --skip-login is given")
	}
	if _, err := Parse([]string{"--skip-login"}); err != nil {
		t.Fatalf("--skip-login should be sufficient: %v", err)
	}
}

func TestParseEnvIsOverriddenByFlags(t *testing.T) {
	t.Setenv("SFTPWEB_ADDR", ":9000")
	t.Setenv("SFTPWEB_SESSION_TTL", "5m")

	c, err := Parse([]string{"--skip-login"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":9000" {
		t.Errorf("Addr = %q, want :9000", c.Addr)
	}
	if c.SessionTTL != 5*time.Minute {
		t.Errorf("SessionTTL = %v, want 5m", c.SessionTTL)
	}

	c, err = Parse([]string{"--skip-login", "--addr", ":7000"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":7000" {
		t.Errorf("flag should win over env: Addr = %q", c.Addr)
	}
}

func TestParseGeneratesSessionSecret(t *testing.T) {
	c, err := Parse([]string{"--skip-login"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.SessionSecret) < 32 {
		t.Errorf("generated session secret too short: %d chars", len(c.SessionSecret))
	}
}

func TestParseSize(t *testing.T) {
	cases := map[string]int64{
		"1024":   1024,
		"5GiB":   5 << 30,
		"512MiB": 512 << 20,
		"2mib":   2 << 20,
		"10KB":   10000,
	}
	for in, want := range cases {
		got, err := parseSize(in)
		if err != nil {
			t.Errorf("parseSize(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseSize(%q) = %d, want %d", in, got, want)
		}
	}
	if _, err := parseSize("banana"); err == nil {
		t.Error("expected an error for a non-numeric size")
	}
}

func TestSplitListTrimsAndDropsEmpties(t *testing.T) {
	got := splitList(" a , ,b,")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("splitList = %#v", got)
	}
}
