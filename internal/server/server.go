// Package server exposes the AgentDeck HTTP API (parity contract with
// the Phase-0 implementation) plus the SSE event stream.
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/cfpperche/agentdeck/internal/agent"
	"github.com/cfpperche/agentdeck/internal/runner"
	"github.com/cfpperche/agentdeck/internal/store"
)

type Server struct {
	Registry *agent.Registry
	Store    *store.Store
	Runner   *runner.Runner
	Mode     string // execution mode badge (ADR-0002)
	Version  string
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/agents", s.handleAgents)
	mux.HandleFunc("GET /api/server-info", s.handleServerInfo)
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

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	out := make([]map[string]string, 0)
	for _, a := range s.Registry.List() {
		out = append(out, map[string]string{"id": a.ID, "label": a.Label, "color": a.Color})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"mode": s.Mode, "version": s.Version,
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
	}
	if err := readBody(r, &in); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if _, ok := s.Registry.Get(in.Agent); !ok {
		writeErr(w, 400, "unknown agent: "+in.Agent)
		return
	}
	ss, err := s.Store.CreateSession(in.Agent, in.Title)
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
		Text string `json:"text"`
	}
	if err := readBody(r, &in); err != nil {
		writeErr(w, 400, "invalid body")
		return
	}
	if in.Text == "" {
		writeErr(w, 400, "empty message")
		return
	}
	queued, err := s.Runner.Send(r.PathValue("id"), in.Text)
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
