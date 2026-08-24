// Package statusline builds the composer pulse (cwd, git, context,
// tokens, cost) inspired by PiCode's Bar (receipts:
// docs/benchmarks/2026-08-24-picode-statusline.md).
package statusline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Bar is the JSON the composer statusline consumes.
type Bar struct {
	Cwd            string   `json:"cwd"`
	Branch         string   `json:"branch,omitempty"`
	Worktree       string   `json:"worktree,omitempty"`
	Dirty          bool     `json:"dirty"`
	SessionName    string   `json:"sessionName,omitempty"`
	Cost           float64  `json:"cost"`
	Input          int      `json:"input,omitempty"`
	Output         int      `json:"output,omitempty"`
	CacheRead      int      `json:"cacheRead,omitempty"`
	CacheWrite     int      `json:"cacheWrite,omitempty"`
	CacheHit       *float64 `json:"cacheHit,omitempty"`
	ContextTokens  *int     `json:"contextTokens,omitempty"`
	ContextWindow  *int     `json:"contextWindow,omitempty"`
	ContextPercent *float64 `json:"contextPercent,omitempty"`
	AutoCompact    bool     `json:"autoCompact"`
	Agent          string   `json:"agent,omitempty"`
}

// Build assembles footer facts for a session working directory.
// sessionPath is a pi JSONL (empty = cwd+git only).
func Build(cwd, sessionPath, agent, model string) Bar {
	b := Bar{Cwd: formatCwd(cwd), Agent: agent, AutoCompact: agent == "pi" && piAutoCompact()}
	b.Branch, b.Worktree, b.Dirty = gitInfo(cwd)
	win := defaultWindow(agent, model)
	if sessionPath != "" {
		u := scanPiUsage(sessionPath)
		b.Cost = u.cost
		b.Input = u.input
		b.Output = u.output
		b.CacheRead = u.cacheRead
		b.CacheWrite = u.cacheWrite
		b.CacheHit = u.cacheHit
		b.SessionName = u.name
		if u.lastTokens > 0 {
			t := u.lastTokens
			b.ContextTokens = &t
			if win > 0 {
				b.ContextWindow = &win
				pct := 100 * float64(t) / float64(win)
				b.ContextPercent = &pct
			}
		}
	}
	if win > 0 && b.ContextWindow == nil {
		b.ContextWindow = &win
	}
	return b
}

// ParseContextWindow turns catalog strings like "200K" / "1M" into tokens.
func ParseContextWindow(s string) int {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0
	}
	mult := 1
	switch {
	case strings.HasSuffix(s, "M"):
		mult = 1_000_000
		s = strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "K"):
		mult = 1_000
		s = strings.TrimSuffix(s, "K")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int(f * float64(mult))
}

func formatCwd(cwd string) string {
	clean := filepath.Clean(cwd)
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if clean == home {
			return "~"
		}
		if strings.HasPrefix(clean, home+string(os.PathSeparator)) {
			return "~" + clean[len(home):]
		}
	}
	return clean
}

func gitInfo(cwd string) (branch, worktree string, dirty bool) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--is-inside-work-tree").Output(); err != nil || strings.TrimSpace(string(out)) != "true" {
		return "", "", false
	}
	if out, err := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		branch = strings.TrimSpace(string(out))
		if branch == "HEAD" {
			branch = "detached"
		}
	}
	if gitDir, err := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--git-dir").Output(); err == nil {
		gd := filepath.ToSlash(strings.TrimSpace(string(gitDir)))
		if i := strings.LastIndex(gd, "/worktrees/"); i >= 0 {
			worktree = gd[i+len("/worktrees/"):]
		}
	}
	if out, err := exec.CommandContext(ctx, "git", "-C", cwd, "status", "--porcelain").Output(); err == nil {
		dirty = len(strings.TrimSpace(string(out))) > 0
	}
	return branch, worktree, dirty
}

func defaultWindow(agent, model string) int {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "grok"):
		return 500_000
	case strings.Contains(m, "claude") || strings.Contains(m, "sonnet") || strings.Contains(m, "opus"):
		return 200_000
	case strings.Contains(m, "gpt-5") || strings.Contains(m, "codex"):
		return 272_000
	case agent == "pi":
		return 200_000
	default:
		return 0
	}
}
