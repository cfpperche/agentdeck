// AgentDeck — the web cockpit for local AI coding agents.
//
// Serve loop with live port rebinding (mechanism ported from PiCode):
// bind-NEW-then-drop-OLD, probe-bind as early courtesy, rollback on
// failure — the app is never unreachable during a port change.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/cfpperche/agentdeck/internal/acp"
	"github.com/cfpperche/agentdeck/internal/codexbridge"
	pibridge "github.com/cfpperche/agentdeck/internal/pibridge"
	"github.com/cfpperche/agentdeck/internal/agent"
	"github.com/cfpperche/agentdeck/internal/config"
	"github.com/cfpperche/agentdeck/internal/runner"
	"github.com/cfpperche/agentdeck/internal/server"
	"github.com/cfpperche/agentdeck/internal/share"
	"github.com/cfpperche/agentdeck/internal/store"
)

// Version is overridden at build time via -ldflags.
var Version = "dev"

// serveState is the live listener state; swapped atomically by rebind.
type serveState struct {
	srv  *http.Server
	ln   net.Listener
	port int
}

func main() {
	// fast paths before any store/registry work
	for i, a := range os.Args[1:] {
		if a == "--version" || a == "-v" {
			fmt.Println(Version)
			return
		}
		if a == "__pirpc" {
			// ADR-0007 bridge (pi native): pi --mode rpc <-> our wire
			args := os.Args[i+2:]
			if len(args) == 0 {
				log.Fatal("__pirpc needs the pi command")
			}
			cmd := exec.Command(args[0], append([]string{"--mode", "rpc"}, args[1:]...)...)
			if home, herr := os.UserHomeDir(); herr == nil {
				for _, dir := range []string{".bun/bin", ".local/bin", ".pi/agent/bin"} {
					cmd.Env = append(cmd.Env, "PATH="+os.Getenv("PATH")+":"+filepath.Join(home, dir))
				}
			}
			stdin, _ := cmd.StdinPipe()
			stdout, _ := cmd.StdoutPipe()
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				log.Fatalf("pi spawn: %v", err)
			}
			if err := pibridge.Run(stdout, stdin); err != nil {
				log.Printf("pi bridge: %v", err)
				stdin.Close()
				cmd.Wait()
				os.Exit(1)
			}
			stdin.Close()
			cmd.Wait()
			return
		}
		if a == "__codexas" {
			// ADR-0007: Codex app-server JSON-RPC <-> our wire
			args := os.Args[i+2:]
			if len(args) == 0 {
				log.Fatal("__codexas needs the codex command")
			}
			cmd := exec.Command(args[0], append([]string{"app-server", "--stdio"}, args[1:]...)...)
			cmd.Env = os.Environ()
			if home, herr := os.UserHomeDir(); herr == nil {
				for _, dir := range []string{".bun/bin", ".local/bin", ".pi/agent/bin"} {
					cmd.Env = append(cmd.Env, "PATH="+os.Getenv("PATH")+":"+filepath.Join(home, dir))
				}
				// Codex reads credentials from $CODEX_HOME (default ~/.codex).
				// A remapped HOME (tests, some units) 401s — pin it.
				if os.Getenv("CODEX_HOME") == "" {
					cmd.Env = append(cmd.Env, "CODEX_HOME="+filepath.Join(home, ".codex"))
				}
			}
			stdin, _ := cmd.StdinPipe()
			stdout, _ := cmd.StdoutPipe()
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				log.Fatalf("codex spawn: %v", err)
			}
			if err := codexbridge.Run(stdout, stdin); err != nil {
				log.Printf("codex bridge: %v", err)
				stdin.Close()
				cmd.Wait()
				os.Exit(1)
			}
			stdin.Close()
			cmd.Wait()
			return
		}
		if a == "__acp" {
			// ADR-0007 bridge: agent command follows; speaks our wire on
			// stdio, ACP to the child.
			args := os.Args[i+2:]
			if len(args) == 0 {
				log.Fatal("__acp needs an agent command")
			}
			cmd := exec.Command(args[0], args[1:]...)
			// systemd user units run with a minimal PATH; agents shell out
			// to user-local tools (HANDOFF war story) — widen it.
			if home, herr := os.UserHomeDir(); herr == nil {
				for _, dir := range []string{".bun/bin", ".local/bin", ".pi/agent/bin"} {
					cmd.Env = append(cmd.Env, "PATH="+os.Getenv("PATH")+":"+filepath.Join(home, dir))
				}
			}
			stdin, _ := cmd.StdinPipe()
			stdout, _ := cmd.StdoutPipe()
			cmd.Stderr = os.Stderr
			if err := cmd.Start(); err != nil {
				log.Fatalf("acp spawn: %v", err)
			}
			bridge := acp.NewBridge(acp.NewConn(stdout, stdin))
			code := 0
			if err := bridge.Run(); err != nil {
				log.Printf("acp bridge: %v", err)
				code = 1
			}
			stdin.Close()
			cmd.Wait()
			os.Exit(code)
		}
	}

	cfg := config.FromEnv()

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	reg := agent.NewRegistry(agent.EnvWhich(nil))
	if len(reg.List()) == 0 {
		log.Fatal("no agent CLI found (need claude, codex, grok, pi or opencode)")
	}

	r := runner.New(reg, st, filepath.Join(cfg.DataDir, "workspaces"))
	rebind := server.NewRebind()
	srv := &server.Server{
		Registry: reg, Store: st, Runner: r,
		Mode: string(cfg.Mode), Version: Version,
		Rebind: rebind, DataDir: cfg.DataDir, TLS: cfg.TLS,
	}
	handler := withWebUI(srv.Routes())

	// resolve port: settings > env > default range
	portCfg := resolvePort(st, cfg)

	state, err := bindAndServe(handler, cfg, portCfg)
	if err != nil {
		log.Fatalf("bind: %v", err)
	}
	srv.Host = cfg.Host
	srv.SetCurrentPort(state.port)
	logStartup(cfg, state.port, len(reg.List()))
	share.EnsureTrustHTTP()

	// graceful shutdown on signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-sigCh:
			log.Println("shutting down…")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			state.srv.Shutdown(ctx)
			cancel()
			return
		case <-rebind.Chan():
			// re-resolve (settings changed) and try to bind NEW first
			newCfg := cfg
			newPort := resolvePort(st, cfg)
			next, err := bindAndServe(handler, newCfg, newPort)
			if err != nil {
				log.Printf("rebind failed (%v) — keeping port %d", err, state.port)
				// roll the setting back so state matches reality
				st.SetSetting("server.port", strconv.Itoa(state.port))
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			state.srv.Shutdown(ctx) // drain old (live agents keep running)
			cancel()
			state = next
			srv.SetCurrentPort(state.port)
			logStartup(cfg, state.port, len(reg.List()))
		}
	}
}

// resolvePort precedence: saved setting > env > default range.
func resolvePort(st *store.Store, cfg config.Config) config.PortConfig {
	if v := st.GetSetting("server.port"); v != "" {
		if p, err := config.ParsePort(v); err == nil {
			return p
		}
	}
	if v := os.Getenv("AGENTDECK_PORT"); v != "" {
		if p, err := config.ParsePort(v); err == nil {
			return p
		}
	}
	return config.PortConfig{Min: 8444, Max: 8454}
}

// bindAndServe tries each port in the range; first bind wins.
func bindAndServe(handler http.Handler, cfg config.Config, ports config.PortConfig) (*serveState, error) {
	var ln net.Listener
	var port int
	var lastErr error
	for p := ports.Min; p <= ports.Max; p++ {
		l, err := net.Listen("tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(p)))
		if err != nil {
			lastErr = err
			continue
		}
		ln, port = l, p
		break
	}
	if ln == nil {
		return nil, fmt.Errorf("no free port in %s: %w", ports, lastErr)
	}

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if cfg.TLS {
		cert, err := ensureCert(cfg.DataDir)
		if err != nil {
			ln.Close()
			return nil, err
		}
		warnIfCertExpiring(cfg.DataDir, 30*24*time.Hour)
		_ = cert
		srv.TLSConfig = liveTLS(cfg.DataDir)
		// wrap for TLS while keeping the plain listener
		go func() {
			if err := srv.ServeTLS(ln, "", ""); err != nil && err != http.ErrServerClosed {
				log.Fatalf("serve: %v", err)
			}
		}()
	} else {
		go func() {
			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				log.Fatalf("serve: %v", err)
			}
		}()
	}
	return &serveState{srv: srv, ln: ln, port: port}, nil
}

func logStartup(cfg config.Config, port, agents int) {
	scheme := "https"
	if !cfg.TLS {
		scheme = "http"
	}
	log.Printf("AgentDeck %s  %s://%s:%d  (mode=%s, data=%s, agents=%d)",
		Version, scheme, cfg.Host, port, cfg.Mode, cfg.DataDir, agents)
}
