// Package tmux owns detached TUI sessions for the terminal dock
// (ADR-0008). The conversation never goes through tmux (ADR-0004).
package tmux

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const Prefix = "agentdeck-"

type Manager struct{}

func New() *Manager { return &Manager{} }

func (m *Manager) Available() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

var nonName = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// SessionName is the tmux target for an AgentDeck session id.
func SessionName(id string) string {
	s := strings.ToLower(nonName.ReplaceAllString(id, "-"))
	s = strings.Trim(s, "-")
	if len(s) > 48 {
		s = s[:48]
	}
	if s == "" {
		s = "session"
	}
	return Prefix + s
}

func Owned(name string) bool {
	return strings.HasPrefix(name, Prefix) && len(name) > len(Prefix)
}

func (m *Manager) run(ctx context.Context, args ...string) (string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "tmux", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (m *Manager) HasSession(ctx context.Context, name string) (bool, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
	}
	err := exec.CommandContext(ctx, "tmux", "has-session", "-t", "="+name).Run()
	if err == nil {
		return true, nil
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func (m *Manager) NewSession(ctx context.Context, name, cwd, command string, args ...string) error {
	ok, err := m.HasSession(ctx, name)
	if err != nil {
		return err
	}
	if ok {
		return nil // already running — attach only
	}
	argv := []string{"new-session", "-d", "-s", name, "-c", cwd, command}
	argv = append(argv, args...)
	_, err = m.run(ctx, argv...)
	return err
}

func (m *Manager) KillSession(ctx context.Context, name string) error {
	ok, err := m.HasSession(ctx, name)
	if err != nil || !ok {
		return err
	}
	_, err = m.run(ctx, "kill-session", "-t", "="+name)
	return err
}


