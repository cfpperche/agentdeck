// Package term bridges WebSocket connections to tmux sessions
// (ADR-0008). Binary frames = PTY bytes; text frames = resize JSON.
// Closing the socket detaches; the tmux session keeps running.
package term

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"

	"github.com/cfpperche/agentdeck/internal/tmux"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Bridge is GET /ws/term?session=agentdeck-<id>.
func Bridge(tm *tmux.Manager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("session")
		if !tmux.Owned(name) {
			http.Error(w, "unknown session", http.StatusBadRequest)
			return
		}
		exists, err := tm.HasSession(r.Context(), name)
		if err != nil {
			http.Error(w, "tmux unavailable", http.StatusInternalServerError)
			return
		}
		if !exists {
			http.Error(w, "session not running", http.StatusNotFound)
			return
		}

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()

		cmd := exec.Command("tmux", "attach-session", "-t", "="+name)
		// systemd units have no TERM; tmux attach then dies with
		// "terminal does not support clear" and the dock shows detached.
		cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
		ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
		if err != nil {
			_ = ws.WriteJSON(map[string]string{"type": "error", "message": err.Error()})
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		ptyDone := make(chan struct{})
		wsDone := make(chan struct{})

		go func() {
			defer close(wsDone)
			defer ptyFile.Close()
			_ = ws.SetReadDeadline(time.Now().Add(pongWait))
			ws.SetPongHandler(func(string) error {
				return ws.SetReadDeadline(time.Now().Add(pongWait))
			})
			for {
				msgType, data, rerr := ws.ReadMessage()
				if rerr != nil {
					return
				}
				switch msgType {
				case websocket.TextMessage:
					var msg struct {
						Type string `json:"type"`
						Cols uint16 `json:"cols"`
						Rows uint16 `json:"rows"`
					}
					if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
						_ = pty.Setsize(ptyFile, &pty.Winsize{Rows: msg.Rows, Cols: msg.Cols})
					}
				case websocket.BinaryMessage:
					if _, werr := ptyFile.Write(data); werr != nil {
						return
					}
				}
			}
		}()

		go func() {
			defer close(ptyDone)
			defer ws.Close()
			buf := make([]byte, 32*1024)
			for {
				n, rerr := ptyFile.Read(buf)
				if n > 0 {
					_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
					if werr := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
						return
					}
				}
				if rerr != nil {
					return
				}
			}
		}()

		// keep-alive
		go func() {
			t := time.NewTicker(pingPeriod)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ptyDone:
					return
				case <-t.C:
					_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
					if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
						return
					}
				}
			}
		}()

		<-wsDone
		<-ptyDone
		_ = cmd.Wait()
	})
}
