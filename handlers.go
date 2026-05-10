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

	qrcode "github.com/skip2/go-qrcode"
)

type Handlers struct {
	cfg     *Config
	sidecar *Sidecar
	llm     *LLMClient
}

func NewHandlers(cfg *Config, sidecar *Sidecar) *Handlers {
	return &Handlers{cfg: cfg, sidecar: sidecar}
}

// refreshLLM rebuilds the LLMClient when needed.
// Remote mode: use the configured remote endpoint directly.
// Local mode: use the port the sidecar is actually listening on.
func (h *Handlers) refreshLLM() {
	if h.llm != nil {
		return
	}
	if h.cfg.IsRemote() {
		h.llm = NewLLMClient(h.cfg.LlamaRemote.Endpoint)
		return
	}
	if port := h.sidecar.Port(); port != "" {
		// Use the actual port the sidecar bound to (may differ from config if
		// the preferred port was taken).
		ep := "http://127.0.0.1:" + port
		h.llm = NewLLMClient(ep)
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
	remote := h.cfg.IsRemote()

	var ready bool
	var llamaAddr string
	var modelPresent bool
	var modelPath string
	var modelSize int64

	if remote {
		// For remote: probe /v1/models — check liveness and loaded model
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(h.cfg.LlamaRemote.Endpoint + "/v1/models")
		if err == nil && resp.StatusCode < 500 {
			ready = true
			var payload struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if jsonErr := json.NewDecoder(resp.Body).Decode(&payload); jsonErr == nil && len(payload.Data) > 0 {
				modelPresent = true
				modelPath = payload.Data[0].ID
			}
			resp.Body.Close()
		}
		llamaAddr = h.cfg.LlamaRemote.Endpoint
	} else {
		port := h.sidecar.Port()
		ready = port != ""
		if port != "" {
			llamaAddr = "127.0.0.1:" + port
		}
	}

	// Binary + model only meaningful in local mode
	binPresent := false
	if !remote {
		if _, err := os.Stat(h.cfg.LlamaLocal.LlamaBin); err == nil {
			binPresent = true
		}
	}

	// modelPresent and modelPath are set in the remote block above (via /v1/models)
	// or here for local mode
	if !modelPresent && !remote {
		if fi, err := os.Stat(h.cfg.LlamaLocal.ModelPath); err == nil {
			modelPresent = true
			modelSize = fi.Size()
			modelPath = h.cfg.LlamaLocal.ModelPath
		}
	}
	if modelPath == "" {
		modelPath = h.cfg.LlamaLocal.ModelPath
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"remote_mode":   remote,
		"ready":         ready,
		"llama_addr":    llamaAddr,
		"model_path":    modelPath,
		"model_present": modelPresent,
		"model_size":    modelSize,
		"bin_path":      h.cfg.LlamaLocal.LlamaBin,
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

// -----------------------------------------------------------------
// GET /api/qr     — PNG QR code pointing to /eye.html on the mobile IP.
// GET /api/qr-url — JSON {"url":"https://..."} with the same URL.
// Both use getMobileIP() which is WSL-aware (returns Windows host IP in WSL2).
// -----------------------------------------------------------------
func (h *Handlers) HandleQR(w http.ResponseWriter, r *http.Request) {
	mobileURL := fmt.Sprintf("https://%s:%s/eye.html", getMobileIP(), h.cfg.HTTPSPort)
	png, err := qrcode.Encode(mobileURL, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "QR generation failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(png) //nolint:errcheck
}

func (h *Handlers) HandleQRURL(w http.ResponseWriter, r *http.Request) {
	mobileURL := fmt.Sprintf("https://%s:%s/eye.html", getMobileIP(), h.cfg.HTTPSPort)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"url": mobileURL}) //nolint:errcheck
}

// jsonEscape wraps text in a JSON string for safe SSE transport.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// -----------------------------------------------------------------
// GET /api/llama/test?endpoint=http://host:port
// Probes the given endpoint's /v1/models and returns reachability + model name.
// -----------------------------------------------------------------
func (h *Handlers) HandleLlamaTest(w http.ResponseWriter, r *http.Request) {
	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		http.Error(w, "endpoint parameter required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint + "/v1/models")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"reachable": false})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		json.NewEncoder(w).Encode(map[string]any{"reachable": false})
		return
	}

	// Try to extract first model name from OpenAI-compatible /v1/models response
	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	modelName := ""
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err == nil && len(modelsResp.Data) > 0 {
		modelName = modelsResp.Data[0].ID
	}

	json.NewEncoder(w).Encode(map[string]any{
		"reachable":  true,
		"model_name": modelName,
	})
}

// -----------------------------------------------------------------
// GET /api/config — returns current config as JSON
// POST /api/config — saves updated config fields to config.json
// -----------------------------------------------------------------
func (h *Handlers) HandleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(h.cfg)

	case http.MethodPost:
		var incoming Config
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		// Validate ports (1-65535)
		for _, p := range []string{incoming.HTTPPort, incoming.HTTPSPort} {
			if !validPort(p) {
				http.Error(w, "invalid port value: "+p, http.StatusBadRequest)
				return
			}
		}

		portsChanged := incoming.HTTPPort != h.cfg.HTTPPort ||
			incoming.HTTPSPort != h.cfg.HTTPSPort

		h.cfg.HTTPHost = incoming.HTTPHost
		h.cfg.HTTPPort = incoming.HTTPPort
		h.cfg.HTTPSPort = incoming.HTTPSPort
		h.cfg.UploadDir = incoming.UploadDir
		h.cfg.LlamaLocal = incoming.LlamaLocal
		h.cfg.LlamaRemote = incoming.LlamaRemote

		// Reset LLM client so next /ask rebuilds with new endpoint
		h.llm = nil

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

// validPort returns true if s is a numeric string representing a port 1-65535.
func validPort(s string) bool {
	if s == "" {
		return false
	}
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
		n = n*10 + int(c-'0')
	}
	return n >= 1 && n <= 65535
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
