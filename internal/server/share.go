package server

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/cfpperche/agentdeck/internal/presence"
	"github.com/cfpperche/agentdeck/internal/share"
)

func (s *Server) handleShare(w http.ResponseWriter, _ *http.Request) {
	share.SyncCert(s.DataDir)
	rep := share.Diagnose(share.Input{
		Insecure: !s.TLS,
		BindHost: s.Host,
		Port:     s.CurrentPort(),
		DataDir:  s.DataDir,
	})
	if tp := share.EnsureTrustHTTP(); tp != "" {
		rep.TrustPort = tp
		if rep.URL != "" {
			if u, err := url.Parse(rep.URL); err == nil {
				base := share.TrustURL(u.Hostname(), tp)
				rep.TrustURL = base + "?next=" + url.QueryEscape(rep.URL)
			}
		}
	}
	writeJSON(w, http.StatusOK, rep)
}

func (s *Server) presence() *presence.Registry {
	if s.Presence == nil {
		s.Presence = presence.New(share.ReachableIPv4())
	}
	return s.Presence
}

func (s *Server) handleDevicePing(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		Host bool   `json:"host"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeErr(w, http.StatusBadRequest, "id required")
		return
	}
	writeJSON(w, http.StatusOK, s.presence().Ping(req.ID, r.UserAgent(), r.RemoteAddr, req.Host))
}

func (s *Server) handleDeviceList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.presence().List())
}
