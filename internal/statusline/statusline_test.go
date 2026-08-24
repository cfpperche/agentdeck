package statusline

import (
	"os"
	"path/filepath"
	"testing"
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

func TestBuildWithoutSession(t *testing.T) {
	b := Build(t.TempDir(), "", "pi", "")
	if b.Cwd == "" || b.ContextWindow == nil || *b.ContextWindow != 200_000 {
		t.Fatalf("%+v", b)
	}
}
