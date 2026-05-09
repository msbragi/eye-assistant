package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Handlers struct {
	cfg     *Config
	sidecar *Sidecar
	llm     *LLMClient
}

func NewHandlers(cfg *Config, sidecar *Sidecar) *Handlers {
	return &Handlers{cfg: cfg, sidecar: sidecar}
}

// refreshLLM rebuilds the LLMClient using the sidecar's current port.
func (h *Handlers) refreshLLM() {
	if port := h.sidecar.Port(); port != "" && (h.llm == nil) {
		h.llm = NewLLMClient(port)
	}
}

// -----------------------------------------------------------------
// POST /ask
// Accepts multipart/form-data with optional "image" file + "question" text.
// Streams the LLM response as SSE (text/event-stream).
// -----------------------------------------------------------------
func (h *Handlers) HandleAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.refreshLLM()
	if h.llm == nil {
		http.Error(w, "LLM not ready — llama-server not started", http.StatusServiceUnavailable)
		return
	}

	// Parse multipart form (max 20 MB for the image)
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	question := r.FormValue("question")
	if strings.TrimSpace(question) == "" {
		question = "What do you see? Please explain clearly."
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sendSSE := func(text string) {
		fmt.Fprintf(w, "data: %s\n\n", jsonEscape(text))
		flusher.Flush()
	}

	// Handle optional image upload
	imageFile, _, err := r.FormFile("image")
	if err == nil {
		defer imageFile.Close()

		// Save image to a temp file
		if err := os.MkdirAll(h.cfg.UploadDir, 0755); err != nil {
			http.Error(w, "cannot create upload dir", http.StatusInternalServerError)
			return
		}
		tmpPath := filepath.Join(h.cfg.UploadDir, fmt.Sprintf("img_%d.jpg", time.Now().UnixNano()))
		f, err := os.Create(tmpPath)
		if err != nil {
			http.Error(w, "cannot save image", http.StatusInternalServerError)
			return
		}
		if _, err := io.Copy(f, imageFile); err != nil {
			f.Close()
			http.Error(w, "cannot write image", http.StatusInternalServerError)
			return
		}
		f.Close()
		defer os.Remove(tmpPath) // clean up after response

		if err := h.llm.AskWithImage(tmpPath, question, sendSSE); err != nil {
			log.Println("LLM error:", err)
			sendSSE("[ERROR] " + err.Error())
		}
	} else {
		// Text-only question
		if err := h.llm.AskText(question, sendSSE); err != nil {
			log.Println("LLM error:", err)
			sendSSE("[ERROR] " + err.Error())
		}
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// -----------------------------------------------------------------
// GET /api/status — extended JSON status for admin dashboard
// -----------------------------------------------------------------
func (h *Handlers) HandleStatus(w http.ResponseWriter, r *http.Request) {
	llamaPort := h.sidecar.Port()
	ready := llamaPort != ""

	// Check binary
	binPresent := false
	if _, err := os.Stat(h.cfg.LlamaBin); err == nil {
		binPresent = true
	}

	// Check model
	modelPresent := false
	var modelSize int64
	if fi, err := os.Stat(h.cfg.ModelPath); err == nil {
		modelPresent = true
		modelSize = fi.Size()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ready":         ready,
		"llama_port":    llamaPort,
		"model_path":    h.cfg.ModelPath,
		"model_present": modelPresent,
		"model_size":    modelSize,
		"bin_path":      h.cfg.LlamaBin,
		"bin_present":   binPresent,
	})
}

// -----------------------------------------------------------------
// GET /api/sysinfo — CPU, RAM, disk, GPU snapshot
// -----------------------------------------------------------------
func (h *Handlers) HandleSysInfo(w http.ResponseWriter, r *http.Request) {
	info := GetSysInfo()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// -----------------------------------------------------------------
// POST /api/llama/stop — stop the llama-server sidecar
// -----------------------------------------------------------------
func (h *Handlers) HandleLlamaStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.sidecar.Stop()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

// -----------------------------------------------------------------
// GET /api/download?target=llama|model-e2b|model-e4b
// Streams download progress as SSE: {"pct":42,"bytes":N,"total":N}
// -----------------------------------------------------------------

// downloadTargets maps target names to download URLs and destination paths.
// URLs are placeholders — replace with actual Hugging Face URLs when available.
var downloadTargets = map[string][2]string{
	"llama": {
		llamaBinaryURL(),
		"bin/linux/llama-server",
	},
	"model-e2b": {
		"https://huggingface.co/bartowski/google_gemma-3-2b-it-GGUF/resolve/main/google_gemma-3-2b-it-Q4_K_M.gguf",
		"models/gemma-4-e2b.gguf",
	},
	"model-e4b": {
		"https://huggingface.co/bartowski/google_gemma-3-4b-it-GGUF/resolve/main/google_gemma-3-4b-it-Q4_K_M.gguf",
		"models/gemma-4-e4b.gguf",
	},
}

func llamaBinaryURL() string {
	if runtime.GOOS == "windows" {
		return "https://github.com/ggml-org/llama.cpp/releases/latest/download/llama-b5765-bin-win-avx2-x64.zip"
	}
	return "https://github.com/ggml-org/llama.cpp/releases/latest/download/llama-b5765-bin-ubuntu-x64.zip"
}

func (h *Handlers) HandleDownload(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	entry, ok := downloadTargets[target]
	if !ok {
		http.Error(w, "unknown target", http.StatusBadRequest)
		return
	}
	url, destPath := entry[0], entry[1]

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sendEvt := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		sendEvt(map[string]string{"error": err.Error()})
		return
	}

	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		sendEvt(map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		sendEvt(map[string]string{"error": fmt.Sprintf("HTTP %d from upstream", resp.StatusCode)})
		return
	}

	total := resp.ContentLength
	tmp := destPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		sendEvt(map[string]string{"error": err.Error()})
		return
	}

	var downloaded int64
	buf := make([]byte, 32*1024)
	lastReport := time.Now()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(tmp)
				sendEvt(map[string]string{"error": werr.Error()})
				return
			}
			downloaded += int64(n)
			if time.Since(lastReport) > 300*time.Millisecond {
				pct := 0
				if total > 0 {
					pct = int(downloaded * 100 / total)
				}
				sendEvt(map[string]any{"pct": pct, "bytes": downloaded, "total": total})
				lastReport = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			os.Remove(tmp)
			sendEvt(map[string]string{"error": readErr.Error()})
			return
		}
	}

	f.Close()
	if err := os.Rename(tmp, destPath); err != nil {
		sendEvt(map[string]string{"error": err.Error()})
		return
	}

	// Make binary executable on Linux/Mac
	if strings.HasSuffix(destPath, "llama-server") {
		os.Chmod(destPath, 0755) //nolint:errcheck
	}

	sendEvt(map[string]any{"pct": 100, "bytes": downloaded, "total": downloaded, "done": true})
}

// jsonEscape wraps text in a JSON string for safe SSE transport.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// -----------------------------------------------------------------
// GET /api/config — returns current config as JSON
// POST /api/config — saves updated config fields to config.json
// Note: port changes require a server restart to take effect.
// -----------------------------------------------------------------
func (h *Handlers) HandleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(h.cfg)

	case http.MethodPost:
		var incoming struct {
			HTTPPort  string `json:"http_port"`
			HTTPSPort string `json:"https_port"`
			LlamaPort string `json:"llama_port"`
			Hostname  string `json:"hostname"`
		}
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		// Validate: ports must be non-empty numeric strings
		for _, p := range []string{incoming.HTTPPort, incoming.HTTPSPort, incoming.LlamaPort} {
			if p == "" {
				http.Error(w, "ports must not be empty", http.StatusBadRequest)
				return
			}
			for _, c := range p {
				if c < '0' || c > '9' {
					http.Error(w, "invalid port value: "+p, http.StatusBadRequest)
					return
				}
			}
		}

		portsChanged := incoming.HTTPPort != h.cfg.HTTPPort ||
			incoming.HTTPSPort != h.cfg.HTTPSPort

		h.cfg.HTTPPort = incoming.HTTPPort
		h.cfg.HTTPSPort = incoming.HTTPSPort
		h.cfg.LlamaPort = incoming.LlamaPort
		h.cfg.Hostname = incoming.Hostname

		if err := h.cfg.Save(ConfigFile); err != nil {
			http.Error(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"ok":            true,
			"ports_changed": portsChanged,
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// -----------------------------------------------------------------
// POST /api/cert/regenerate — generates a new self-signed SSL cert
// Uses the current hostname and all LAN IPs as SANs.
// Takes effect immediately for new TLS connections (server stays running).
// -----------------------------------------------------------------
func (h *Handlers) HandleCertRegenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hostnames, ips := DefaultCertSANs(h.cfg)
	if err := GenerateSelfSignedCert(hostnames, ips); err != nil {
		log.Println("Cert regeneration failed:", err)
		http.Error(w, "cert generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("SSL cert regenerated — SANs: %v / IPs: %v", hostnames, ips)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":        true,
		"hostnames": hostnames,
		"ips":       func() []string {
			s := make([]string, len(ips))
			for i, ip := range ips { s[i] = ip.String() }
			return s
		}(),
	})
}
