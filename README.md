# GemmaLink — Your Home AI Assistant

**Point your phone at anything at home. Get a clear, instant explanation. No cloud. No subscriptions. No data leaving your house.**

---

## What is GemmaLink?

GemmaLink is a local AI assistant designed for everyday household use. It runs entirely on your laptop and connects to any smartphone on the same Wi-Fi — no app to install, no account to create.

The core interaction is simple:

1. Open the GemmaLink web page on your phone (via QR code).
2. Point the camera at any object or document.
3. Ask a question, or pick a preset mode.
4. Get a clear, useful answer — in seconds.

---

## Why You'd Actually Use This

Modern homes are full of things nobody fully understands.

- **Appliances** with cryptic symbols and control panels that nobody reads the manual for.
- **Bank statements and bills** with terminology designed to confuse.
- **Medicine leaflets** written for doctors, not patients.
- **Instruction manuals** in five languages, none of them helpful.

GemmaLink bridges that gap. You don't need to google, scroll, or call customer support. You just ask.

### Example use cases

| You point the camera at... | You ask... | GemmaLink answers... |
|---|---|---|
| The washing machine control panel | "What does this symbol mean?" | "That's the delicate cycle for wool and silk. Use cold water." |
| Your electricity bill | "Summarise this" | "You used 12% more than last month. Your peak usage is between 6–9pm." |
| A medication package | "Is this safe for children?" | "This contains ibuprofen. The children's dose is listed on the back panel." |
| A router with blinking lights | "Why is it blinking red?" | "A red WAN light usually means no internet signal from your ISP." |

---

## Why Privacy Matters Here

When you photograph a **bank statement**, a **medical document**, or a **personal letter**, you should not be sending that image to a server in California.

GemmaLink processes everything **locally on your machine**. The image never leaves your home network. There is no account, no telemetry, no usage data collected.

This is the core reason GemmaLink exists as a local application rather than a web service.

---

## Why Gemma 4?

Gemma 4 is not just a language model — it is a **multimodal** model, meaning it understands both text and images natively. This is what makes GemmaLink possible.

Specifically, Gemma 4 excels at:

- **Document understanding** — reading printed text, tables, and structured layouts directly from photos, without a separate OCR step.
- **Symbol and icon recognition** — identifying appliance symbols, warning icons, and interface elements from real-world photographs.
- **Contextual reasoning** — not just reading what is in the image, but explaining what it *means* in plain language.
- **Compact efficiency** — the Gemma 4 E2B and E4B variants (2B and 4B parameters) are specifically designed to run well on consumer hardware with limited VRAM, making local deployment realistic without a dedicated AI workstation.

The combination of multimodal capability and a small footprint makes Gemma 4 uniquely suited to this use case. Larger cloud models would require an internet connection and expose user data; larger local models would require expensive hardware. Gemma 4 hits the right balance.

---

## Technical Architecture

GemmaLink uses a two-process "sidecar" design, orchestrated entirely in Go.

```
┌─────────────────────────────────┐        ┌──────────────────────┐
│         Go Application          │        │    llama-server       │
│  (Wails Desktop UI)             │◄──────►│  (llama.cpp binary)  │
│                                 │  REST  │                      │
│  - LAN HTTP server              │  JSON  │  Runs Gemma 4 GGUF   │
│  - QR code generation           │        │  on local port       │
│  - Model download manager       │        │                      │
│  - Request routing              │        └──────────────────────┘
└─────────────────────────────────┘
         ▲
         │ Wi-Fi (LAN only)
         │
┌────────┴────────┐
│  Mobile Browser │
│  Camera + Voice │
└─────────────────┘
```

### Stack

| Component | Technology | Reason |
|---|---|---|
| Orchestrator | **Go** | Single static binary, no runtime dependencies, excellent concurrency |
| Desktop UI | **Wails** | Native webview, far lighter than Electron, compiles to a single `.exe` / `.app` |
| Inference Engine | **llama.cpp** (`llama-server`) | Best-in-class CPU inference, pre-compiled binaries available for all platforms |
| AI Model | **Gemma 4 E4B (GGUF)** | Multimodal, compact, runs on CPU/GPU, open weights |
| Mobile Frontend | **HTML/JS served over LAN** | No app install required on the phone |

### Key design decisions

- **No installer touches system paths.** The app is self-contained. The model is downloaded to `%APPDATA%\GemmaLink` (Windows) or `~/.local/share/gemmalink` (Linux/Mac) on first run.
- **Dynamic port allocation.** The Go orchestrator finds a free port at startup and passes it to `llama-server`, avoiding conflicts.
- **LAN-only by design.** The mobile HTTP server binds to the local network interface only. There is no exposure to the internet.

---

## Getting Started

> Requirements: a laptop with at least 8 GB RAM. A GPU is optional but speeds up inference significantly.

1. Download the GemmaLink binary for your OS from the Releases page.
2. Run it. On first launch it will download the Gemma 4 E4B model (~3 GB).
3. A QR code appears on screen. Scan it with your phone.
4. Point, ask, done.

---

## Building from Source

```bash
# Prerequisites: Go 1.22+, Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

git clone https://github.com/your-username/gemmalink
cd gemmalink
wails build
```

Cross-compile for Linux from Windows (or vice versa):

```bash
GOOS=linux GOARCH=amd64 wails build
```

---

## Project Status

Built for the **Gemma 4 Challenge** — submission deadline May 24, 2026.  
See [GemmaLink_Tasks.md](GemmaLink_Tasks.md) for the full implementation roadmap.
