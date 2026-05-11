package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
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

	// Per-model file presence (always checked, used by model selector UI)
	filePresent := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}
	modelsPresent := map[string]any{
		"e2b": map[string]bool{
			"model":  filePresent(DefaultModelPathE2B),
			"mmproj": filePresent(DefaultMmprojPathE2B),
		},
		"e4b": map[string]bool{
			"model":  filePresent(DefaultModelPathE4B),
			"mmproj": filePresent(DefaultMmprojPathE4B),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"remote_mode":    remote,
		"ready":          ready,
		"llama_addr":     llamaAddr,
		"model_path":     modelPath,
		"model_present":  modelPresent,
		"model_size":     modelSize,
		"bin_path":       h.cfg.LlamaLocal.LlamaBin,
		"bin_present":    binPresent,
		"bin_version":    h.cfg.LlamaLocal.LlamaBinVersion,
		"models_present": modelsPresent,
		"active_variant": h.cfg.LlamaLocal.MmprojPath,
		"vision_enabled": h.cfg.LlamaLocal.VisionEnabled,
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
// POST /api/llama/start — start the llama-server sidecar
// -----------------------------------------------------------------
func (h *Handlers) HandleLlamaStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := h.sidecar.Start(); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// -----------------------------------------------------------------
// POST /api/model/select?variant=e2b|e4b
// Sets model_path + mmproj_path in config and saves.
// -----------------------------------------------------------------
func (h *Handlers) HandleModelSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	variant := r.URL.Query().Get("variant")
	var modelPath, mmprojPath string
	switch variant {
	case "e2b":
		modelPath = DefaultModelPathE2B
		mmprojPath = DefaultMmprojPathE2B
	case "e4b":
		modelPath = DefaultModelPathE4B
		mmprojPath = DefaultMmprojPathE4B
	default:
		http.Error(w, "unknown variant, use e2b or e4b", http.StatusBadRequest)
		return
	}

	h.cfg.LlamaLocal.ModelPath = modelPath
	h.cfg.LlamaLocal.MmprojPath = mmprojPath
	// Reset LLM client so next /ask uses new paths
	h.llm = nil

	if err := h.cfg.Save(ConfigFile); err != nil {
		http.Error(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":          true,
		"model_path":  modelPath,
		"mmproj_path": mmprojPath,
	})
}

// -----------------------------------------------------------------
// POST /api/vision/toggle?enabled=true&variant=e2b|e4b
// Enables or disables mmproj vision for local llama-server.
// -----------------------------------------------------------------
func (h *Handlers) HandleVisionToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	enabled := r.URL.Query().Get("enabled") == "true"
	variant := r.URL.Query().Get("variant")

	h.cfg.LlamaLocal.VisionEnabled = enabled
	if enabled && variant != "" {
		switch variant {
		case "e2b":
			h.cfg.LlamaLocal.MmprojPath = DefaultMmprojPathE2B
		case "e4b":
			h.cfg.LlamaLocal.MmprojPath = DefaultMmprojPathE4B
		}
	}

	if err := h.cfg.Save(ConfigFile); err != nil {
		http.Error(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":             true,
		"vision_enabled": h.cfg.LlamaLocal.VisionEnabled,
	})
}

// -----------------------------------------------------------------
// GET /api/llama/releases — lists compatible llama.cpp GitHub releases
// Returns [{tag, name, url}] filtered by current OS (CPU variant).
// -----------------------------------------------------------------
func (h *Handlers) HandleLlamaReleases(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet,
		"https://api.github.com/repos/ggml-org/llama.cpp/releases?per_page=15", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "GitHub API unreachable: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	var releases []struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		http.Error(w, "failed to parse GitHub response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter assets by OS and pick the CPU variant (avx2 on Windows, plain ubuntu on Linux)
	type releaseEntry struct {
		Tag  string `json:"tag"`
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	var result []releaseEntry
	for _, rel := range releases {
		for _, asset := range rel.Assets {
			n := strings.ToLower(asset.Name)
			var match bool
			if runtime.GOOS == "windows" {
				match = strings.Contains(n, "win-avx2-x64") && strings.HasSuffix(n, ".zip")
			} else {
				match = strings.Contains(n, "ubuntu-x64") && strings.HasSuffix(n, ".tar.gz") &&
					!strings.Contains(n, "cuda") && !strings.Contains(n, "rocm")
			}
			if match {
				result = append(result, releaseEntry{
					Tag:  rel.TagName,
					Name: asset.Name,
					URL:  asset.BrowserDownloadURL,
				})
				break // one per release
			}
		}
		if len(result) >= 10 {
			break
		}
	}

	json.NewEncoder(w).Encode(result)
}

// -----------------------------------------------------------------
// POST /api/llama/select?tag=b9095
// Saves the active llama-server version tag to config.
// -----------------------------------------------------------------
func (h *Handlers) HandleLlamaSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		http.Error(w, "tag required", http.StatusBadRequest)
		return
	}

	h.cfg.LlamaLocal.LlamaBinVersion = tag
	if err := h.cfg.Save(ConfigFile); err != nil {
		http.Error(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":  true,
		"tag": tag,
	})
}

// -----------------------------------------------------------------
// GET /api/download?target=llama|model-e2b|model-e4b
// Streams download progress as SSE: {"pct":42,"bytes":N,"total":N}
// -----------------------------------------------------------------

// downloadTargets maps target names to download URLs and destination paths.
// Llama binary URL is built from the version tag stored in config.
func (h *Handlers) resolveDownload(target string) (downloadURL, destPath string, ok bool) {
	switch target {
	case "llama":
		u := llamaBinaryURLForTag(h.cfg.LlamaLocal.LlamaBinVersion)
		return u, h.cfg.LlamaLocal.LlamaBin, true
	case "model-e2b":
		u := h.cfg.ModelURLs.E2B
		if u == "" {
			u = DefaultModelURLe2b
		}
		return u, DefaultModelPathE2B, true
	case "model-e4b":
		u := h.cfg.ModelURLs.E4B
		if u == "" {
			u = DefaultModelURLe4b
		}
		return u, DefaultModelPathE4B, true
	case "mmproj-e2b":
		u := h.cfg.ModelURLs.MmprojE2B
		if u == "" {
			u = DefaultMmprojURLe2b
		}
		return u, DefaultMmprojPathE2B, true
	case "mmproj-e4b":
		u := h.cfg.ModelURLs.MmprojE4B
		if u == "" {
			u = DefaultMmprojURLe4b
		}
		return u, DefaultMmprojPathE4B, true
	}
	return "", "", false
}

// llamaBinaryURLForTag builds the GitHub download URL for a given release tag.
// If tag is empty, uses the /latest redirect (tag-agnostic filename not possible,
// so we fall back to a known-good recent build).
func llamaBinaryURLForTag(tag string) string {
	if tag == "" {
		tag = "b9095" // fallback until user selects a version
	}
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("https://github.com/ggml-org/llama.cpp/releases/download/%s/llama-%s-bin-win-avx2-x64.zip", tag, tag)
	}
	return fmt.Sprintf("https://github.com/ggml-org/llama.cpp/releases/download/%s/llama-%s-bin-ubuntu-x64.tar.gz", tag, tag)
}

func (h *Handlers) HandleDownload(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	url, destPath, ok := h.resolveDownload(target)
	if !ok {
		http.Error(w, "unknown target", http.StatusBadRequest)
		return
	}

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

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		sendEvt(map[string]string{"error": err.Error()})
		return
	}
	resp, err := http.DefaultClient.Do(req)
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

	ctx := r.Context()
	for {
		// Check if client cancelled
		select {
		case <-ctx.Done():
			f.Close()
			os.Remove(tmp)
			return
		default:
		}
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
			// Don't report error if cancelled by client
			if ctx.Err() == nil {
				sendEvt(map[string]string{"error": readErr.Error()})
			}
			return
		}
	}

	f.Close()
	if err := os.Rename(tmp, destPath); err != nil {
		sendEvt(map[string]string{"error": err.Error()})
		return
	}

	// For llama binary: extract the actual executable from the archive
	if target == "llama" {
		if err := extractLlamaBinary(destPath); err != nil {
			os.Remove(destPath)
			sendEvt(map[string]string{"error": "extract failed: " + err.Error()})
			return
		}
	}

	// Make binary executable on Linux/Mac
	if strings.HasSuffix(destPath, "llama-server") || strings.HasSuffix(destPath, "llama-server.exe") {
		os.Chmod(destPath, 0755) //nolint:errcheck
	}

	sendEvt(map[string]any{"pct": 100, "bytes": downloaded, "total": downloaded, "done": true})
}

// extractLlamaBinary extracts llama-server (or llama-server.exe) and all
// required shared libraries from the downloaded archive (tar.gz on Linux,
// zip on Windows) at archivePath, replacing the archive file with the binary.
func extractLlamaBinary(archivePath string) error {
	binName := "llama-server"
	if runtime.GOOS == "windows" {
		binName = "llama-server.exe"
	}

	destDir := filepath.Dir(archivePath)

	if runtime.GOOS == "windows" {
		return extractFromZipAll(archivePath, binName, destDir)
	}
	return extractFromTarGzAll(archivePath, binName, destDir)
}

// extractFromTarGzAll extracts binName + all .so/.so.* files into destDir,
// then removes the archive.
func extractFromTarGzAll(archivePath, binName, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	binTmp := archivePath + ".bin.tmp"
	tr := tar.NewReader(gz)
	foundBin := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		base := filepath.Base(hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeReg:
			isBin := base == binName
			isSo := strings.HasSuffix(base, ".so") || strings.Contains(base, ".so.")
			if !isBin && !isSo {
				continue
			}
			// Write binary to a temp file — never truncate the file we're reading.
			destFile := filepath.Join(destDir, base)
			if isBin {
				destFile = binTmp
				foundBin = true
			}
			out, err := os.Create(destFile)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			if err != nil {
				return err
			}
			if isBin {
				os.Chmod(destFile, 0755) //nolint:errcheck
			}

		case tar.TypeSymlink:
			// Restore symlinks (e.g. libllama-common.so.0 → libllama-common.so.0.0.9102)
			linkName := base
			linkTarget := filepath.Base(hdr.Linkname)
			isSoLink := strings.Contains(linkName, ".so")
			if !isSoLink {
				continue
			}
			symlinkPath := filepath.Join(destDir, linkName)
			os.Remove(symlinkPath)
			os.Symlink(linkTarget, symlinkPath) //nolint:errcheck
		}
	}

	if !foundBin {
		os.Remove(binTmp)
		return fmt.Errorf("%s not found in archive", binName)
	}

	// Replace the archive file with the extracted binary.
	f.Close()
	os.Remove(archivePath)
	return os.Rename(binTmp, archivePath)
}

// extractFromZipAll extracts binName + all .dll files into destDir,
// then removes the archive.
func extractFromZipAll(archivePath, binName, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	foundBin := false
	for _, f := range r.File {
		base := filepath.Base(f.Name)
		isBin := base == binName
		isDll := strings.HasSuffix(strings.ToLower(base), ".dll")
		if !isBin && !isDll {
			continue
		}
		destFile := filepath.Join(destDir, base)
		if isBin {
			destFile = archivePath
			foundBin = true
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(destFile)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	if !foundBin {
		return fmt.Errorf("%s not found in archive", binName)
	}
	return nil
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
		h.cfg.ModelURLs = incoming.ModelURLs

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
		"ips": func() []string {
			s := make([]string, len(ips))
			for i, ip := range ips {
				s[i] = ip.String()
			}
			return s
		}(),
	})
}
