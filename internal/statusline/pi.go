package statusline

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PiDir is ~/.pi/agent/sessions/<DirName(cwd)> — pi's on-disk session tree.
func PiDir(cwd string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".pi", "agent", "sessions", piDirName(cwd))
}

func piDirName(cwd string) string {
	clean := filepath.ToSlash(filepath.Clean(cwd))
	enc := strings.ReplaceAll(clean, "/", "-")
	enc = strings.Trim(enc, "-")
	return "--" + enc + "--"
}

// LatestPiSession is the newest *.jsonl under PiDir(cwd), or "".
func LatestPiSession(cwd string) string {
	dir := PiDir(cwd)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestT time.Time
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if st.ModTime().After(bestT) {
			bestT = st.ModTime()
			best = p
		}
	}
	return best
}

func piAutoCompact() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return true
	}
	b, err := os.ReadFile(filepath.Join(home, ".pi", "agent", "settings.json"))
	if err != nil {
		return true
	}
	var s struct {
		Compaction *struct {
			Enabled *bool `json:"enabled"`
		} `json:"compaction"`
	}
	if json.Unmarshal(b, &s) != nil || s.Compaction == nil || s.Compaction.Enabled == nil {
		return true
	}
	return *s.Compaction.Enabled
}

type piUsage struct {
	cost                                 float64
	lastTokens                           int
	input, output, cacheRead, cacheWrite int
	cacheHit                             *float64
	name                                 string
}

func scanPiUsage(path string) (out piUsage) {
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var raw map[string]any
		if json.Unmarshal(sc.Bytes(), &raw) != nil {
			continue
		}
		if raw["type"] == "session_info" {
			if n, _ := raw["name"].(string); n != "" {
				out.name = n
			}
			continue
		}
		if raw["type"] != "message" {
			continue
		}
		msg, _ := raw["message"].(map[string]any)
		if msg == nil {
			continue
		}
		u, _ := msg["usage"].(map[string]any)
		if u == nil {
			continue
		}
		add := func(key string) int {
			if v, ok := u[key].(float64); ok {
				return int(v)
			}
			return 0
		}
		in, ou := add("input"), add("output")
		cr, cw := add("cacheRead"), add("cacheWrite")
		out.input += in
		out.output += ou
		out.cacheRead += cr
		out.cacheWrite += cw
		if c, _ := u["cost"].(map[string]any); c != nil {
			if v, ok := c["total"].(float64); ok {
				out.cost += v
			}
		}
		if v, ok := u["totalTokens"].(float64); ok && int(v) > 0 {
			out.lastTokens = int(v)
		}
		prompt := in + cr
		if prompt > 0 && cr > 0 {
			h := 100 * float64(cr) / float64(prompt)
			out.cacheHit = &h
		}
	}
	return out
}
