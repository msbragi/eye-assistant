package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

// Sidecar manages the llama-server child process.
type Sidecar struct {
	cfg  *Config
	cmd  *exec.Cmd
	mu   sync.Mutex
	port string
}

func NewSidecar(cfg *Config) *Sidecar {
	return &Sidecar{cfg: cfg}
}

// Start launches llama-server on a free port (or the configured one).
// In remote mode this is a no-op — the user manages the remote server.
func (s *Sidecar) Start() error {
	if s.cfg.IsRemote() {
		log.Println("Remote llama mode — sidecar not started. Manage llama-server on the remote host.")
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil && s.cmd.Process != nil {
		return nil // already running
	}

	preferredPort := endpointPort(s.cfg.LlamaLocal.Endpoint)
	if preferredPort == "" {
		preferredPort = "11434"
	}
	port, err := findFreePort(preferredPort)
	if err != nil {
		return fmt.Errorf("finding free port: %w", err)
	}

	if _, err := os.Stat(s.cfg.LlamaLocal.LlamaBin); os.IsNotExist(err) {
		return fmt.Errorf("llama-server binary not found at %s", s.cfg.LlamaLocal.LlamaBin)
	}
	if _, err := os.Stat(s.cfg.LlamaLocal.ModelPath); os.IsNotExist(err) {
		return fmt.Errorf("model not found at %s — run the downloader first", s.cfg.LlamaLocal.ModelPath)
	}

	s.cmd = exec.Command(s.cfg.LlamaLocal.LlamaBin,
		"--model", s.cfg.LlamaLocal.ModelPath,
		"--port", port,
		"--ctx-size", strconv.Itoa(s.cfg.LlamaLocal.ContextSize),
		"--host", "127.0.0.1",
	)
	s.cmd.Stdout = os.Stdout
	s.cmd.Stderr = os.Stderr

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("starting llama-server: %w", err)
	}

	s.port = port // set only after successful start
	log.Printf("llama-server started (pid=%d) on port %s", s.cmd.Process.Pid, s.port)

	// Wait until the server is ready (max 60s)
	if err := waitForPort("127.0.0.1:"+s.port, 60*time.Second); err != nil {
		return fmt.Errorf("llama-server did not become ready: %w", err)
	}
	log.Println("llama-server is ready")
	return nil
}

// Stop kills the llama-server process.
func (s *Sidecar) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
		s.cmd = nil
		log.Println("llama-server stopped")
	}
}

// Port returns the port llama-server is listening on.
func (s *Sidecar) Port() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// findFreePort tries to use the preferred port; falls back to a random one.
func findFreePort(preferred string) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:"+preferred)
	if err == nil {
		ln.Close()
		return preferred, nil
	}
	// Preferred port is taken — let the OS assign one
	ln, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer ln.Close()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	return port, err
}

// waitForPort polls until the TCP address is reachable or the deadline passes.
func waitForPort(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", addr)
}
