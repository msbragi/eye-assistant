package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

	args := []string{
		"--model", s.cfg.LlamaLocal.ModelPath,
		"--port", port,
		"--ctx-size", strconv.Itoa(s.cfg.LlamaLocal.ContextSize),
		"--host", "127.0.0.1",
	}
	if s.cfg.LlamaLocal.MmprojPath != "" && s.cfg.LlamaLocal.VisionEnabled {
		args = append(args, "--mmproj", s.cfg.LlamaLocal.MmprojPath)
	}
	s.cmd = exec.Command(s.cfg.LlamaLocal.LlamaBin, args...)
	s.cmd.Stdout = os.Stdout
	s.cmd.Stderr = os.Stderr
	// Ensure shared libs in the same directory as the binary are found.
	binDir, _ := filepath.Abs(filepath.Dir(s.cfg.LlamaLocal.LlamaBin))
	s.cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+binDir)

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("starting llama-server: %w", err)
	}

	s.port = port // set only after successful start
	log.Printf("llama-server started (pid=%d) on port %s", s.cmd.Process.Pid, s.port)

	// Wait until the model is fully loaded (max 120s), polling /health
	if err := waitForHealth("http://127.0.0.1:"+s.port+"/health", 120*time.Second); err != nil {
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
		s.port = "" // clear so Port() returns empty and status shows not running
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

// waitForHealth polls GET url until llama-server reports {"status":"ok"}.
func waitForHealth(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			var body struct {
				Status string `json:"status"`
			}
			json.NewDecoder(resp.Body).Decode(&body)
			resp.Body.Close()
			if body.Status == "ok" {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timeout waiting for llama-server to be ready")
}
