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
)

//go:embed all:web/dist
var webFiles embed.FS

// withWebUI serves the embedded SPA for non-API paths (SPA fallback).
// /api/ and /ws/ go to the backend (the terminal dock is a WebSocket).
func withWebUI(api http.Handler) http.Handler {
	dist, err := fs.Sub(webFiles, "web/dist")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
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

// warnIfCertExpiring logs loudly when the leaf cert is close to expiry
// (the weekly systemd timer renews; this is the observability net).
func liveTLS(dataDir string) *tls.Config {
	certPath := filepath.Join(dataDir, "cert.pem")
	keyPath := filepath.Join(dataDir, "key.pem")
	return &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			c, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				return nil, err
			}
			return &c, nil
		},
	}
}

func warnIfCertExpiring(dataDir string, within time.Duration) {
	b, err := os.ReadFile(filepath.Join(dataDir, "cert.pem"))
	if err != nil {
		return
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return
	}
	crt, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return
	}
	if time.Until(crt.NotAfter) < within {
		log.Printf("WARNING: TLS certificate expires in %.0f days (%s) — run scripts/setup-cert.sh",
			time.Until(crt.NotAfter).Hours()/24, crt.NotAfter.Format("2006-01-02"))
	}
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
