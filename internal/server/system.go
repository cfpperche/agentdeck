package server

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/cfpperche/agentdeck/internal/share"
	"github.com/cfpperche/agentdeck/internal/tmux"
)

// systemReport is the PiCode-shaped payload for GET /api/system.
type systemReport struct {
	Host struct {
		Name string `json:"name"`
		OS   string `json:"os"`
		Arch string `json:"arch"`
		WSL  bool   `json:"wsl"`
	} `json:"host"`
	Network struct {
		Bind      string   `json:"bind"`
		Port      int      `json:"port,omitempty"`
		HTTPS     bool     `json:"https"`
		LAN       []string `json:"lan"`
		Tailscale string   `json:"tailscale,omitempty"`
	} `json:"network"`
	Tmux struct {
		Installed bool   `json:"installed"`
		Version   string `json:"version,omitempty"`
	} `json:"tmux"`
	Mkcert struct {
		Installed bool `json:"installed"`
	} `json:"mkcert"`
	Tailscale struct {
		Installed bool   `json:"installed"`
		IP        string `json:"ip,omitempty"`
	} `json:"tailscale"`
	Agents   []systemAgent `json:"agents"`
	Warnings []string      `json:"warnings"`
	Version  string        `json:"version,omitempty"`
}

type systemAgent struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
}

func (s *Server) handleSystem(w http.ResponseWriter, _ *http.Request) {
	var rep systemReport
	rep.Version = s.Version
	rep.Host.OS = runtime.GOOS
	rep.Host.Arch = runtime.GOARCH
	rep.Host.WSL = runningOnWSL()
	if name, err := os.Hostname(); err == nil {
		rep.Host.Name = name
	}

	rep.Network.Bind = s.Host
	if rep.Network.Bind == "" {
		rep.Network.Bind = "0.0.0.0"
	}
	rep.Network.HTTPS = s.TLS
	rep.Network.Port = s.CurrentPort()
	rep.Network.LAN = []string{}

	if _, err := exec.LookPath("tailscale"); err == nil {
		rep.Tailscale.Installed = true
		if out, err := exec.Command("tailscale", "ip", "-4").Output(); err == nil {
			rep.Tailscale.IP = strings.TrimSpace(string(out))
			rep.Network.Tailscale = rep.Tailscale.IP
		}
	}
	for _, ip := range share.ReachableIPv4() {
		if ip != "" && ip != rep.Network.Tailscale {
			rep.Network.LAN = append(rep.Network.LAN, ip)
		}
	}

	tm := tmux.New()
	if tm.Available() {
		rep.Tmux.Installed = true
		rep.Tmux.Version = tm.Version()
	} else {
		rep.Warnings = append(rep.Warnings, "tmux is not installed — the Terminal dock needs it to keep a TUI after you close the browser")
	}
	if _, err := exec.LookPath("mkcert"); err == nil {
		rep.Mkcert.Installed = true
	}

	if s.Registry != nil {
		for _, a := range s.Registry.List() {
			rep.Agents = append(rep.Agents, systemAgent{
				ID: a.ID, Label: a.Label, Installed: true, Version: cliVersion(a.ID),
			})
		}
	}
	if len(rep.Warnings) == 0 {
		rep.Warnings = []string{}
	}
	if rep.Agents == nil {
		rep.Agents = []systemAgent{}
	}
	writeJSON(w, http.StatusOK, rep)
}

func cliVersion(name string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, "--version").Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if len(line) > 48 {
		line = line[:48]
	}
	return line
}

func runningOnWSL() bool {
	b, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(b)), "microsoft")
}
