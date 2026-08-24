// Package config resolves runtime configuration and the execution-mode
// feature flag (ADR-0002): personal vs dedicated (aiagent-linux) user.
package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

type Mode string

const (
	ModeAuto      Mode = "auto"
	ModePersonal  Mode = "personal"
	ModeDedicated Mode = "dedicated"
)

// DedicatedUser is the conventional dedicated agent user provisioned by
// aiagent-linux (https://github.com/cfpperche/aiagent-linux).
const DedicatedUser = "aiagent"

type Config struct {
	Addr    string // host:port (effective, after bind)
	Host    string
	Port    int    // effective port (the one that bound)
	DataDir string // sessions db + workspaces + certs
	Mode    Mode   // effective mode (after auto-detection)
	TLS     bool   // serve HTTPS with a self-signed cert
}

// PortConfig is a port or a range ("8444" or "8444-8454").
type PortConfig struct{ Min, Max int }

func ParsePort(s string) (PortConfig, error) {
	var a, b int
	n, _ := fmt.Sscanf(s, "%d-%d", &a, &b)
	switch {
	case n == 2:
	case n == 1:
		b = a
	default:
		return PortConfig{}, fmt.Errorf("invalid port %q", s)
	}
	if a < 1 || b > 65535 || a > b || b-a > 99 {
		return PortConfig{}, fmt.Errorf("invalid port range %q", s)
	}
	return PortConfig{a, b}, nil
}

func (p PortConfig) String() string {
	if p.Min == p.Max {
		return strconv.Itoa(p.Min)
	}
	return fmt.Sprintf("%d-%d", p.Min, p.Max)
}

// FromEnv builds the configuration. Env parity with the Phase-0 server:
// AGENTDECK_PORT, AGENTDECK_HOST, AGENTDECK_DATA, AGENTDECK_MODE,
// plus AGENTDECK_INSECURE=1 to disable TLS (dev/behind-proxy).
func FromEnv() Config {
	host := getenv("AGENTDECK_HOST", "0.0.0.0")
	port := getenv("AGENTDECK_PORT", "8444")
	home, _ := os.UserHomeDir()
	dataDir := getenv("AGENTDECK_DATA", filepath.Join(home, "agentdeck", "data"))

	mode := Mode(getenv("AGENTDECK_MODE", string(ModeAuto)))
	if mode != ModePersonal && mode != ModeDedicated {
		mode = ModeAuto
	}
	if mode == ModeAuto {
		mode = detectMode()
	}

	return Config{
		Addr:    host + ":" + port,
		DataDir: dataDir,
		Mode:    mode,
		TLS:     os.Getenv("AGENTDECK_INSECURE") != "1",
	}
}

// detectMode: running under the dedicated user → dedicated; else personal.
// (var for test override)
var detectMode = func() Mode {
	if u, err := user.Current(); err == nil && u.Username == DedicatedUser {
		return ModeDedicated
	}
	return ModePersonal
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
