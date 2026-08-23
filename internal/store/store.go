// Package store persists sessions and messages in SQLite (WAL).
// The schema is intentionally identical to the Phase-0 Python
// implementation so both servers can share a data directory.
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	_ "modernc.org/sqlite"
)

type Session struct {
	ID           string `json:"id"`
	Agent        string `json:"agent"`
	Title        string `json:"title"`
	AgentRef     string `json:"agent_ref"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	MessageCount int    `json:"message_count"`
	Preview      string `json:"preview"`
}

type Message struct {
	ID        int64          `json:"id"`
	SessionID string         `json:"session_id"`
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	Meta      map[string]any `json:"meta,omitempty"`
	CreatedAt string         `json:"created_at"`
}

type Store struct {
	db   *sql.DB
	mu   sync.Mutex
	path string
}

var schema = `
CREATE TABLE IF NOT EXISTS sessions(
    id TEXT PRIMARY KEY,
    agent TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT 'New session',
    agent_ref TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS messages(
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    meta TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, id);
`

// Open creates/opens the database at dir/agentdeck.db.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "agentdeck.db"))
	if err != nil {
		return nil, err
	}
	// modernc sqlite: enable WAL + busy timeout for concurrent readers
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, path: dir}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func now() string { return time.Now().UTC().Format("2006-01-02T15:04:05") }

func newID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b[:])
}

// CreateSession inserts a session; title defaults to "New session"
// (the runner auto-titles from the first message).
func (s *Store) CreateSession(agent, title string) (*Session, error) {
	id := newID()
	t := now()
	if title == "" {
		title = "New session"
	}
	_, err := s.db.Exec(
		`INSERT INTO sessions(id, agent, title, created_at, updated_at)
		 VALUES(?,?,?,?,?)`, id, agent, title, t, t)
	if err != nil {
		return nil, err
	}
	return s.GetSession(id)
}

func (s *Store) GetSession(id string) (*Session, error) {
	row := s.db.QueryRow(sessionSelect+` WHERE s.id=?`, id)
	return scanSession(row)
}

const sessionSelect = `
SELECT s.id, s.agent, s.title, IFNULL(s.agent_ref,''), s.created_at, s.updated_at,
       (SELECT COUNT(*) FROM messages m WHERE m.session_id = s.id) AS message_count,
       (SELECT m.content FROM messages m WHERE m.session_id = s.id
         ORDER BY m.id DESC LIMIT 1) AS preview
FROM sessions s`

func scanSession(row *sql.Row) (*Session, error) {
	var ss Session
	var preview sql.NullString
	err := row.Scan(&ss.ID, &ss.Agent, &ss.Title, &ss.AgentRef,
		&ss.CreatedAt, &ss.UpdatedAt, &ss.MessageCount, &preview)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ss.Preview = CleanPreview(preview.String)
	return &ss, nil
}

func (s *Store) ListSessions() ([]Session, error) {
	rows, err := s.db.Query(sessionSelect + ` ORDER BY s.updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var ss Session
		var preview sql.NullString
		if err := rows.Scan(&ss.ID, &ss.Agent, &ss.Title, &ss.AgentRef,
			&ss.CreatedAt, &ss.UpdatedAt, &ss.MessageCount, &preview); err != nil {
			return nil, err
		}
		ss.Preview = CleanPreview(preview.String)
		out = append(out, ss)
	}
	return out, rows.Err()
}

func (s *Store) RenameSession(id, title string) (*Session, error) {
	if title == "" {
		title = "New session"
	}
	res, err := s.db.Exec(`UPDATE sessions SET title=?, updated_at=? WHERE id=?`,
		title, now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil
	}
	return s.GetSession(id)
}

func (s *Store) SetAgentRef(id, ref string) error {
	_, err := s.db.Exec(`UPDATE sessions SET agent_ref=? WHERE id=?`, ref, id)
	return err
}

func (s *Store) HasAssistantReply(id string) bool {
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_id=? AND role='assistant'`, id).Scan(&n)
	return n > 0
}

func (s *Store) AddMessage(sessionID, role, content string, meta map[string]any) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	metaJSON := ""
	if meta != nil {
		b, err := json.Marshal(meta)
		if err != nil {
			return nil, err
		}
		metaJSON = string(b)
	}
	t := now()
	res, err := s.db.Exec(
		`INSERT INTO messages(session_id, role, content, meta, created_at)
		 VALUES(?,?,?,?,?)`, sessionID, role, content, metaJSON, t)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if _, err := s.db.Exec(`UPDATE sessions SET updated_at=? WHERE id=?`, t, sessionID); err != nil {
		return nil, err
	}
	return &Message{ID: id, SessionID: sessionID, Role: role,
		Content: content, Meta: meta, CreatedAt: t}, nil
}

func (s *Store) ListMessages(sessionID string) ([]Message, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, IFNULL(meta,''), created_at
		 FROM messages WHERE session_id=? ORDER BY id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var meta string
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &meta, &m.CreatedAt); err != nil {
			return nil, err
		}
		if meta != "" {
			json.Unmarshal([]byte(meta), &m.Meta)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteSession removes the session, its messages and workspace dir.
func (s *Store) DeleteSession(id string, workspacesDir string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM sessions WHERE id=?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if _, err := s.db.Exec(`DELETE FROM messages WHERE session_id=?`, id); err != nil {
		return false, err
	}
	if workspacesDir != "" {
		os.RemoveAll(filepath.Join(workspacesDir, id))
	}
	return n > 0, nil
}

// ---- preview cleaning (parity with the Python prototype) ----

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// CleanPreview returns a one-line preview suitable for the sidebar:
// ANSI stripped, banners skipped, empty when nothing useful remains.
func CleanPreview(text string) string {
	if text == "" {
		return ""
	}
	t := ansiRe.ReplaceAllString(text, "")
	for _, l := range strings.Split(t, "\n") {
		l = strings.TrimSpace(l)
		if len(l) < 4 {
			continue
		}
		textual := 0
		for _, r := range l {
			if isPreviewRune(r) {
				textual++
			}
		}
		if float64(textual)/float64(len([]rune(l))) > 0.7 {
			l = strings.Join(strings.Fields(l), " ")
			if len(l) > 90 {
				l = l[:90]
			}
			return l
		}
	}
	return ""
}

func isPreviewRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r)
}
