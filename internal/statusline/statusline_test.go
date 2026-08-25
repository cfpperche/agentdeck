package statusline

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/cfpperche/agentdeck/internal/agent"

	_ "modernc.org/sqlite"
)

func TestParseContextWindow(t *testing.T) {
	if ParseContextWindow("200K") != 200_000 {
		t.Fatal("200K")
	}
	if ParseContextWindow("1M") != 1_000_000 {
		t.Fatal("1M")
	}
	if ParseContextWindow("") != 0 {
		t.Fatal("empty")
	}
}

func TestFormatCwdHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	if formatCwd(home) != "~" {
		t.Fatalf("home: %s", formatCwd(home))
	}
	got := formatCwd(filepath.Join(home, "agentdeck"))
	if got != "~/agentdeck" {
		t.Fatalf("got %s", got)
	}
}

func TestPiDirName(t *testing.T) {
	got := piDirName("/home/goat/agentdeck/data/workspaces/abc")
	want := "--home-goat-agentdeck-data-workspaces-abc--"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestScanPiUsage(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s.jsonl")
	body := `{"type":"session_info","name":"demo"}
{"type":"message","message":{"usage":{"input":1000,"output":200,"cacheRead":4000,"cacheWrite":0,"totalTokens":5200,"cost":{"total":0.12}}}}
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	u := scanPiUsage(p)
	if u.name != "demo" || u.input != 1000 || u.output != 200 || u.lastTokens != 5200 {
		t.Fatalf("%+v", u)
	}
	if u.cacheHit == nil || *u.cacheHit < 79 || *u.cacheHit > 81 {
		t.Fatalf("cacheHit %+v", u.cacheHit)
	}
	if u.cost != 0.12 {
		t.Fatalf("cost %v", u.cost)
	}
}

func TestApplyLive(t *testing.T) {
	b := Build(t.TempDir(), "", "codex", "gpt-5.6-terra")
	b = ApplyLive(b, &agent.Usage{Input: 1000, Output: 80, CacheRead: 4000, Total: 5080, Window: 272000})
	if b.Input != 1000 || b.Output != 80 || b.ContextTokens == nil || *b.ContextTokens != 5080 {
		t.Fatalf("%+v", b)
	}
	if b.ContextPercent == nil || *b.ContextPercent < 1 {
		t.Fatalf("pct %+v", b.ContextPercent)
	}
}

func TestScanOpenCodeDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE session (
		id TEXT, directory TEXT, title TEXT, cost REAL,
		tokens_input INTEGER, tokens_output INTEGER, tokens_reasoning INTEGER,
		tokens_cache_read INTEGER, tokens_cache_write INTEGER, time_updated INTEGER)`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	cwd := filepath.Join(dir, "ws")
	_, err = db.Exec(`INSERT INTO session VALUES (?,?,?,?,?,?,?,?,?,?)`,
		"ses_1", cwd, "Greeting", 0.0, 1797, 21, 7, 7296, 0, 100)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	u := scanOpenCodeDB(dbPath, cwd)
	if u.name != "Greeting" || u.input != 1797 || u.output != 21 || u.cacheRead != 7296 {
		t.Fatalf("%+v", u)
	}
	if u.lastTokens != 1797+21+7+7296 {
		t.Fatalf("last %d", u.lastTokens)
	}
}

func TestBuildWithoutSession(t *testing.T) {
	b := Build(t.TempDir(), "", "pi", "")
	if b.Cwd == "" || b.ContextWindow == nil || *b.ContextWindow != 200_000 {
		t.Fatalf("%+v", b)
	}
}
