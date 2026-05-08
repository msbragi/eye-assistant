# GemmaLink: Project Implementation Roadmap
**Architecture:** Go Orchestrator + llama.cpp Sidecar  
**Goal:** Zero-Infrastructure Multimodal AI Hub for Gemma 4 Challenge  
**OS Target:** Windows (Dev) / WSL Debian (Testing)

---

## 1. Development Environment Setup
- [ ] **Go Initialization:** `go mod init gemmalink`
- [ ] **Wails UI Setup:** `wails init` (Select a lightweight frontend like Svelte or Vanilla JS).
- [ ] **Binaries Repository:** Download pre-compiled `llama-server` binaries for Windows and Linux.
    - Place in `./bin/windows/llama-server.exe`
    - Place in `./bin/linux/llama-server`

## 2. Phase 1: The Go Orchestrator (Backend)
- [ ] **Process Management:** Implement `os/exec` logic to start/stop the `llama-server` as a child process.
- [ ] **Dynamic Port Mapping:** Find an open port and pass it to the sidecar.
- [ ] **Model Downloader:** Implement a simple downloader in Go to fetch the Gemma 4 E2B/E4B GGUF files if missing from the local folder.

## 3. Phase 2: LAN & Connectivity
- [ ] **IP Detection:** Use `net.InterfaceAddrs()` to identify the laptop's LAN IP.
- [ ] **QR Code Integration:** Use `github.com/skip2/go-qrcode` to display the pairing URL in the desktop UI.
- [ ] **Mobile Gateway:** Implement a Go `http` handler to serve a simple mobile web page to any device on the same Wi-Fi.

## 4. Phase 3: The Multimodal "Lens" (Frontend)
- [ ] **Mobile Interface:** - JS-based Camera capture (`getUserMedia`).
    - Audio recording for native voice input.
- [ ] **PC Dashboard:** - A clean UI to show server status and the QR code.
    - Performance monitoring (CPU/RAM).

## 5. Phase 4: Gemma 4 Logic
- [ ] **Multimodal Pipeline:** Route the phone's image and audio data through the Go backend into the `llama-server` JSON API.
- [ ] **System Prompting:** Tune the prompt for household reasoning (e.g., "Analyze this image and explain the fix simply").

## 6. Cross-Platform Testing (WSL)
- [ ] **Build for Linux:** Run `GOOS=linux GOARCH=amd64 wails build`.
- [ ] **Validation:** Run the resulting binary in WSL Debian to ensure the sidecar `llama-server` executes correctly.

---
**Submission Deadline:** May 24, 2026.
