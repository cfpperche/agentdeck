package statusline

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// OpenCodeDB is ~/.local/share/opencode/opencode.db (XDG data).
func OpenCodeDB() string {
	if p := os.Getenv("AGENTDECK_OPENCODE_DB"); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "opencode", "opencode.db")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
}

// scanOpenCode reads the newest session row for cwd from OpenCode's sqlite.
func scanOpenCode(cwd string) piUsage {
	return scanOpenCodeDB(OpenCodeDB(), cwd)
}

func scanOpenCodeDB(dbPath, cwd string) piUsage {
	var z piUsage
	if dbPath == "" || cwd == "" {
		return z
	}
	if _, err := os.Stat(dbPath); err != nil {
		return z
	}
	cwd = filepath.Clean(cwd)
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_pragma=busy_timeout(800)")
	if err != nil {
		return z
	}
	defer db.Close()

	var title sql.NullString
	var cost sql.NullFloat64
	var in, out, reason, cr, cw sql.NullInt64
	err = db.QueryRow(`
		SELECT title, cost, tokens_input, tokens_output, tokens_reasoning,
		       tokens_cache_read, tokens_cache_write
		FROM session
		WHERE directory = ? OR directory = ?
		ORDER BY time_updated DESC
		LIMIT 1`, cwd, cwd+"/").Scan(&title, &cost, &in, &out, &reason, &cr, &cw)
	if err != nil {
		return z
	}
	z.name = title.String
	z.cost = cost.Float64
	z.input = int(in.Int64)
	z.output = int(out.Int64)
	z.cacheRead = int(cr.Int64)
	z.cacheWrite = int(cw.Int64)
	z.lastTokens = z.input + z.output + int(reason.Int64) + z.cacheRead
	if z.cacheRead > 0 && (z.input+z.cacheRead) > 0 {
		h := 100 * float64(z.cacheRead) / float64(z.input+z.cacheRead)
		z.cacheHit = &h
	}
	return z
}
