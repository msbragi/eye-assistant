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
func (s *Sidecar) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil && s.cmd.Process != nil {
		return nil // already running
	}

	port, err := findFreePort(s.cfg.LlamaPort)
	if err != nil {
		return fmt.Errorf("finding free port: %w", err)
	}

	if _, err := os.Stat(s.cfg.LlamaBin); os.IsNotExist(err) {
		return fmt.Errorf("llama-server binary not found at %s", s.cfg.LlamaBin)
	}
	if _, err := os.Stat(s.cfg.ModelPath); os.IsNotExist(err) {
		return fmt.Errorf("model not found at %s — run the downloader first", s.cfg.ModelPath)
	}

	s.cmd = exec.Command(s.cfg.LlamaBin,
		"--model", s.cfg.ModelPath,
		"--port", s.port,
		"--ctx-size", strconv.Itoa(s.cfg.ContextSize),
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
