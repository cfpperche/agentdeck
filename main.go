// AgentDeck — the web cockpit for local AI coding agents.
//
// Single binary: embeds the web UI (go:embed) and serves the API over
// HTTPS with a self-signed certificate generated on first run.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/pem"
	"io/fs"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cfpperche/agentdeck/internal/agent"
	"github.com/cfpperche/agentdeck/internal/config"
	"github.com/cfpperche/agentdeck/internal/runner"
	"github.com/cfpperche/agentdeck/internal/server"
	"github.com/cfpperche/agentdeck/internal/store"
)

// Version is overridden at build time via -ldflags.
var Version = "dev"

//go:embed all:web/dist
var webFiles embed.FS

func main() {
	cfg := config.FromEnv()

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	reg := agent.NewRegistry(agent.EnvWhich(nil))
	if len(reg.List()) == 0 {
		log.Fatal("no agent CLI found (need claude, codex, grok, pi or opencode in PATH)")
	}

	r := runner.New(reg, st, filepath.Join(cfg.DataDir, "workspaces"))
	srv := &server.Server{
		Registry: reg, Store: st, Runner: r,
		Mode: string(cfg.Mode), Version: Version,
	}

	handler := withWebUI(srv.Routes())
	scheme := "https"
	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if cfg.TLS {
		cert, err := ensureCert(cfg.DataDir)
		if err != nil {
			log.Fatalf("tls: %v", err)
		}
		httpSrv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
		log.Fatal(httpSrv.ListenAndServeTLS("", ""))
	}
	scheme = "http"
	log.Printf("AgentDeck %s  %s://%s  (mode=%s, data=%s, agents=%d)",
		Version, scheme, cfg.Addr, cfg.Mode, cfg.DataDir, len(reg.List()))
	log.Fatal(httpSrv.ListenAndServe())
}

// withWebUI serves the embedded SPA for non-/api paths (SPA fallback).
func withWebUI(api http.Handler) http.Handler {
	dist, err := fs.Sub(webFiles, "web/dist")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for unknown paths
		if _, err := fs.Stat(dist, strings.TrimPrefix(r.URL.Path, "/")); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

// ensureCert loads or creates a self-signed certificate in the data dir.
// Native crypto — no openssl dependency.
func ensureCert(dataDir string) (tls.Certificate, error) {
	certPath := filepath.Join(dataDir, "cert.pem")
	keyPath := filepath.Join(dataDir, "key.pem")
	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		return cert, nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "agentdeck"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	// SANs: localhost + every local IPv4 (LAN + tailnet), so trusting
	// this one cert covers all access routes
	tmpl.IPAddresses = append(tmpl.IPAddresses,
		net.ParseIP("127.0.0.1"), net.ParseIP("::1"))
	tmpl.DNSNames = []string{"localhost"}
	seen := map[string]bool{}
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.To4() == nil || ip.IsLoopback() {
				continue
			}
			if !seen[ip.String()] {
				seen[ip.String()] = true
				tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
			}
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl,
		&key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return tls.Certificate{}, err
	}
	log.Printf("generated self-signed certificate: %s", certPath)
	return tls.X509KeyPair(certPEM, keyPEM)
}
