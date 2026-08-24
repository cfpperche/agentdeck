package server

import (
	"net/http"
	"path/filepath"

	"github.com/cfpperche/agentdeck/internal/statusline"
)

func (s *Server) handleSessionStatus(w http.ResponseWriter, r *http.Request) {
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
	cwd := ss.Cwd
	if cwd == "" && s.Runner != nil {
		cwd = filepath.Join(s.Runner.Workspaces, id)
	}
	model := ""
	if s.Runner != nil {
		if c := s.Runner.LastControls(id); c != nil {
			model = c.Model
		}
	}
	path := ""
	if ss.Agent == "pi" {
		path = statusline.LatestPiSession(cwd)
	}
	bar := statusline.Build(cwd, path, ss.Agent, model)
	if s.Runner != nil {
		bar = statusline.ApplyLive(bar, s.Runner.Usage(id))
	}
	writeJSON(w, http.StatusOK, bar)
}
