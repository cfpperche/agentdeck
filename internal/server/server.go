// Package server exposes the AgentDeck HTTP API (parity contract with
// the Phase-0 implementation) plus the SSE event stream.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cfpperche/agentdeck/internal/agent"
	"github.com/cfpperche/agentdeck/internal/config"
	"github.com/cfpperche/agentdeck/internal/presence"
	"github.com/cfpperche/agentdeck/internal/runner"
	"github.com/cfpperche/agentdeck/internal/store"
)

type Server struct {
	Registry *agent.Registry
	Store    *store.Store
	Runner   *runner.Runner
	Mode     string // execution mode badge (ADR-0002)
	Version  string
	Rebind   *Rebind // optional: port-change signal (nil = disabled)
	Host     string  // bind host for the port probe (default 127.0.0.1)

	currentPort atomic.Int32

	DataDir  string
	TLS      bool
	Presence *presence.Registry
}

// CurrentPort returns the port this process is serving on.
func (s *Server) CurrentPort() int { return int(s.currentPort.Load()) }

// SetCurrentPort records the serving port (called by the serve loop).
func (s *Server) SetCurrentPort(p int) { s.currentPort.Store(int32(p)) }

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/agents", s.handleAgents)
	mux.HandleFunc("GET /api/fs/dirs", s.handleListDirs)
	mux.HandleFunc("POST /api/fs/mkdir", s.handleMkdir)
	mux.HandleFunc("GET /api/server-info", s.handleServerInfo)
	mux.HandleFunc("GET /api/share", s.handleShare)
	mux.HandleFunc("POST /api/devices/ping", s.handleDevicePing)
	mux.HandleFunc("GET /api/devices", s.handleDeviceList)
	mux.HandleFunc("GET /api/server/port", s.handleGetPort)
	mux.HandleFunc("PUT /api/server/port", s.handlePutPort)
	mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	mux.HandleFunc("POST /api/sessions", s.handleCreateSession)
	mux.HandleFunc("PATCH /api/sessions/{id}", s.handleRenameSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("GET /api/sessions/{id}/messages", s.handleMessages)
	mux.HandleFunc("POST /api/sessions/{id}/messages", s.handleSend)
	mux.HandleFunc("POST /api/sessions/{id}/control", s.handleControl)
	mux.HandleFunc("POST /api/sessions/{id}/queue/cancel", s.handleQueueCancel)
	mux.HandleFunc("POST /api/sessions/{id}/stop", s.handleStop)
	mux.HandleFunc("GET /api/sessions/{id}/events", s.handleEvents)

	return mux
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"detail": msg})
}

func readBody(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// ---- handlers ----

// handleListDirs lists DIRECTORIES ONLY under ?path= (default ~) —
// powers the cwd picker. No files, sorted, capped; entries carry a
// "traversable" hint (permission to read).
func (s *Server) handleListDirs(w http.ResponseWriter, r *http.Request) {
	root := r.URL.Query().Get("path")
	if root == "" {
		root, _ = os.UserHomeDir()
	}
	root = filepath.Clean(root)
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		writeErr(w, 400, "not a directory: "+root)
		return
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		writeErr(w, 400, "cannot read: "+root)
		return
	}
	type dirEntry struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	out := []dirEntry{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") && name != "." && name != ".." {
			continue // hidden dirs are noise for picking a project root
		}
		out = append(out, dirEntry{Name: name, Path: filepath.Join(root, name)})
		if len(out) >= 500 {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	writeJSON(w, http.StatusOK, map[string]any{"path": root, "dirs": out})
}

// handleMkdir creates a directory (parents allowed). Guardrails:
// absolute paths only, must stay under the user's home, rejects
// existing dirs and hidden names.
func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Path string `json:"path"`
	}
	if err := readBody(r, &in); err != nil || strings.TrimSpace(in.Path) == "" {
		writeErr(w, 400, "path required")
		return
	}
	abs, err := filepath.Abs(strings.TrimSpace(in.Path))
	if err != nil {
		writeErr(w, 400, "invalid path")
		return
	}
	home, _ := os.UserHomeDir()
	if home != "" && !strings.HasPrefix(abs, home+string(filepath.Separator)) && abs != home {
		writeErr(w, 400, "must be inside your home directory")
		return
	}
	if st, err := os.Stat(abs); err == nil {
		if st.IsDir() {
			writeErr(w, 409, "already exists")
			return
		}
		writeErr(w, 400, "path exists and is not a directory")
		return
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"path": abs})
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	out := make([]map[string]string, 0)
	for _, a := range s.Registry.List() {
		out = append(out, map[string]string{"id": a.ID, "label": a.Label, "color": a.Color})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	username := "you"
	if u, err := user.Current(); err == nil && u.Username != "" {
		username = u.Username
	}
	host, _ := os.Hostname()
	writeJSON(w, http.StatusOK, map[string]string{
		"mode": s.Mode, "version": s.Version,
		"user": username, "host": host,
	})
}

// handleGetPort reports the serving + configured port.
func (s *Server) handleGetPort(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"serving":    s.CurrentPort(),
		"configured": s.Store.GetSetting("server.port"),
	})
}

// handlePutPort changes the serving port (settings-sourced rebind).
// Probe-bind as early courtesy (occupied now → 409); the serve loop's
// bind-new-then-drop-old is the transactional guarantee (rollback on
// failure). Responds 202 via the OLD listener; the UI reconnects.
func (s *Server) handlePutPort(w http.ResponseWriter, r *http.Request) {
	if s.Rebind == nil {
		writeErr(w, 501, "port change disabled in this process")
		return
	}
	var in struct {
		Port string `json:"port"`
	}
	if err := readBody(r, &in); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	pc, err := config.ParsePort(in.Port)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if pc.Min != pc.Max {
		writeErr(w, 400, "UI accepts a single port (ranges are for env/headless)")
		return
	}
	// probe-bind (courtesy, not guarantee)
	for p := pc.Min; p <= pc.Max; p++ {
		l, err := net.Listen("tcp", net.JoinHostPort(s.Host, strconv.Itoa(p)))
		if err != nil {
			writeErr(w, 409, fmt.Sprintf("port %d is already in use", p))
			return
		}
		l.Close()
	}
	from := s.Store.GetSetting("server.port")
	s.Store.SetSetting("server.port", strconv.Itoa(pc.Min))
	s.Rebind.Signal()
	log.Printf("server_port_changed from=%q to=%d", from, pc.Min)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"moving": true, "port": pc.Min,
	})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	ss, err := s.Store.ListSessions()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if ss == nil {
		ss = []store.Session{}
	}
	writeJSON(w, http.StatusOK, ss)
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Agent string `json:"agent"`
		Title string `json:"title"`
		Cwd   string `json:"cwd"`
	}
	if err := readBody(r, &in); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if _, ok := s.Registry.Get(in.Agent); !ok {
		writeErr(w, 400, "unknown agent: "+in.Agent)
		return
	}
	// optional working directory: must exist and be a directory (realpath)
	if in.Cwd != "" {
		abs, err := filepath.Abs(in.Cwd)
		if err != nil {
			writeErr(w, 400, "invalid path")
			return
		}
		st, err := os.Stat(abs)
		if err != nil || !st.IsDir() {
			writeErr(w, 400, "not a directory: "+in.Cwd)
			return
		}
		in.Cwd = abs
	}
	ss, err := s.Store.CreateSession(in.Agent, in.Title, in.Cwd)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ss)
}

func (s *Server) handleRenameSession(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
	}
	if err := readBody(r, &in); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	ss, err := s.Store.RenameSession(r.PathValue("id"), in.Title)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if ss == nil {
		writeErr(w, 404, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, ss)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.Runner.Stop(id)
	ok, err := s.Store.DeleteSession(id, s.Runner.Workspaces)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if !ok {
		writeErr(w, 404, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ss, err := s.Store.GetSession(id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if ss == nil {
		writeErr(w, 404, "session not found")
		return
	}
	msgs, err := s.Store.ListMessages(id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text     string          `json:"text"`
		Controls *agent.Controls `json:"controls"`
	}
	if err := readBody(r, &in); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if in.Text == "" {
		writeErr(w, 400, "empty message")
		return
	}
	var ctrls []*agent.Controls
	if in.Controls != nil {
		ctrls = append(ctrls, in.Controls)
	}
	queued, err := s.Runner.Send(r.PathValue("id"), in.Text, ctrls...)
	switch {
	case err == nil && queued:
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "queued": true})
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "queued": false})
	case errors.Is(err, runner.ErrBusy):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, os.ErrNotExist):
		writeErr(w, 404, "session not found")
	default:
		writeErr(w, 500, err.Error())
	}
}

// handleQueueCancel discards queued (not yet delivered) messages.
func (s *Server) handleQueueCancel(w http.ResponseWriter, r *http.Request) {
	cleared := s.Runner.ClearQueue(r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "cleared": cleared})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": s.Runner.Stop(r.PathValue("id"))})
}

// handleControl answers a permission request on a live session (ADR-0004):
// {"request_id": "...", "behavior": "allow"|"deny"}
func (s *Server) handleControl(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RequestID    string          `json:"request_id"`
		Behavior     string          `json:"behavior"`
		UpdatedInput json.RawMessage `json:"updatedInput"`
	}
	if err := readBody(r, &in); err != nil || in.RequestID == "" {
		writeErr(w, 400, "request_id required")
		return
	}
	if err := s.Runner.Control(r.PathValue("id"), in.RequestID, in.Behavior, in.UpdatedInput); err != nil {
		writeErr(w, 409, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleEvents streams session events over SSE until the client leaves.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ss, err := s.Store.GetSession(id)
	if err != nil || ss == nil {
		writeErr(w, 404, "session not found")
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ch, unsub := s.Runner.Subscribe(id)
	defer unsub()

	// state snapshot on connect (parity with legacy) + late-subscriber
	// pending-permission replay (G1): a page reload during 'waiting'
	// must still show the banner
	st := s.Runner.Status(id)
	snapshot, _ := json.Marshal(runner.StreamEvent{
		Type: "state", Status: string(st), Running: st != runner.StatusIdle})
	w.Write([]byte("data: " + string(snapshot) + "\n\n"))
	for _, pev := range s.Runner.PendingPermissions(id) {
		if b, err := json.Marshal(pev); err == nil {
			w.Write([]byte("data: " + string(b) + "\n\n"))
		}
	}
	// composer surface replay (ADR-0006): a page reload must still show
	// the model/mode controls without waiting for the next agent spawn.
	if caps := s.Runner.Caps(id); caps != nil {
		if b, err := json.Marshal(runner.StreamEvent{Type: "capabilities",
			Models: caps.Models, Modes: caps.Modes}); err == nil {
			w.Write([]byte("data: " + string(b) + "\n\n"))
		}
	}
	fl.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := w.Write([]byte(": heartbeat\n\n")); err != nil {
				return
			}
			fl.Flush()
		case ev := <-ch:
			b, _ := json.Marshal(ev)
			if _, err := w.Write([]byte("data: " + string(b) + "\n\n")); err != nil {
				return
			}
			fl.Flush()
		}
	}
}
