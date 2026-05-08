package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
// GET /status — returns JSON with server and sidecar state
// -----------------------------------------------------------------
func (h *Handlers) HandleStatus(w http.ResponseWriter, r *http.Request) {
	llamaPort := h.sidecar.Port()
	ready := llamaPort != ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ready":      ready,
		"llama_port": llamaPort,
		"model":      h.cfg.ModelPath,
	})
}

// jsonEscape wraps text in a JSON string for safe SSE transport.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
