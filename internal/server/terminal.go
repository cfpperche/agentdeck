package server

import (
	"net/http"

	"github.com/cfpperche/agentdeck/internal/agent"
)

func (s *Server) handleTermGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	writeJSON(w, http.StatusOK, map[string]any{
		"available": s.Runner.TUIAvailable(),
		"open":      s.Runner.HasTUI(id),
		"session":   s.Runner.TUIName(id),
	})
}

func (s *Server) handleTermOpen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Controls *agent.Controls `json:"controls"`
	}
	_ = readBody(r, &in)
	name, err := s.Runner.StartTUI(id, in.Controls)
	if err != nil {
		writeErr(w, 409, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "session": name, "surface": "terminal",
	})
}

func (s *Server) handleTermClose(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.Runner.StopTUI(id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "surface": "chat"})
}
