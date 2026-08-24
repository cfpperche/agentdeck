package server

import "sync"

// Rebind is a non-blocking signal channel the port-change handler
// pokes; the main serve loop selects on it and rebinds the listener.
type Rebind struct {
	mu sync.Mutex
	ch chan struct{}
}

func NewRebind() *Rebind { return &Rebind{ch: make(chan struct{}, 1)} }

// Signal requests a rebind (idempotent while pending).
func (r *Rebind) Signal() {
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case r.ch <- struct{}{}:
	default:
	}
}

// Chan exposes the signal channel for select.
func (r *Rebind) Chan() <-chan struct{} { return r.ch }
