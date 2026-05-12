## Go Source Files

### main.go — Entry Point & Router
**Purpose:** bootstraps the application, wires all components, starts HTTP/HTTPS servers.
- Loads config.json, creates `Sidecar` and `Handlers`, registers all routes
- UA detection: `/` → `admin.html` (desktop) or `eye.html` (mobile)
- Prints QR code and LAN URL on startup; generates TLS cert on first run
- Graceful shutdown on `SIGINT`/`SIGTERM`

Key functions: `main()`, `getLANIP()`, `getLANIPs()`, `getMobileIP()`, `printQR()`

---

### config.go — Configuration
**Purpose:** defines all config structs and handles persistence to config.json.
- Structs: `Config`, `LlamaLocal`, `LlamaRemote`, `ModelURLs`
- Package-level constants for all default model URLs and paths
- `loadConfig()` reads and parses; `cfg.Save()` writes back atomically
- `cfg.IsRemote()` / `cfg.ActiveEndpoint()` — routing helpers used by handlers

---

### handlers.go — HTTP Handler Layer
**Purpose:** implements every HTTP endpoint — the single largest file.

| Handler | Responsibility |
|---|---|
| `HandleAsk` | Chat + optional image → streams LLM response as SSE |
| `HandleStatus` | JSON snapshot: sidecar state, model, binary, sysinfo |
| `HandleLlamaStart/Stop` | Start/stop the sidecar child process |
| `HandleLlamaReleases` | GitHub API → scored asset list per platform |
| `HandleLlamaSelect` | Saves chosen tag + real asset URL to config |
| `HandleDownload` | SSE-streamed download; extracts archives; cleans old libs |
| `HandleModelSelect` / `HandleVisionToggle` | Update and persist model config |
| `HandleConfig` | GET/POST the runtime configuration |
| `HandleCertRegenerate` | Regenerates the self-signed TLS cert |
| `HandleLlamaTest` | Sends a minimal prompt, returns latency |
| `HandleQR` / `HandleQRURL` | Serves QR code PNG / JSON URL |

Internal helpers: `extractLlamaBinary()`, `extractFromTarGzAll()`, `extractFromZipAll()`, `cleanBinDir()`, `llamaBinaryURLForTag()`

---

### llm.go — LLM Client
**Purpose:** encapsulates all communication with llama-server's OpenAI-compatible API.
- `Chat()` builds a `/v1/chat/completions` streaming request, supports text-only and multimodal (text + base64 image)
- Parses SSE chunks, calls a token callback for each delta
- `loadImageAsBase64()` encodes an uploaded file for multimodal payloads

Key types: `LLMClient`, `ChatMessage`, `ContentPart`, `ChatRequest`, `StreamChunk`

---

### sidecar.go — llama-server Process Manager
**Purpose:** manages the `llama-server` child process lifecycle.
- `Start()` — finds a free port, validates binary/model, builds CLI args (including `--mmproj` for vision), sets `LD_LIBRARY_PATH`, spawns process
- `Stop()` — graceful termination
- `IsRunning()` — liveness check
- `WaitReady()` — polls health endpoint until ready or timeout

---

### cert.go — TLS Certificate Generator
**Purpose:** generates a self-signed RSA-2048 certificate at first run — no `openssl` dependency.
- `GenerateSelfSignedCert()` — RSA-2048 key + X.509 cert with SANs, writes to .ssl
- `DefaultCertSANs()` — collects all LAN IPs + hostnames for a LAN-valid certificate

---

### sysinfo.go — System Resource Monitor
**Purpose:** cross-platform hardware snapshot using `gopsutil`.
- Collects RAM, CPU (1-second sample), disk usage
- GPU stats via `nvidia-smi` — best-effort, silently skipped if unavailable

---

### static_dev.go / static_prod.go — Static File Serving
**Purpose:** build-tag pair providing two implementations of `staticHandler()`.
- **`dev` tag:** serves static from disk — live edits without rebuild
- **default (prod):** embeds static into the binary via `//go:embed` — fully self-contained release

---

## Diagrams

### 1 — Bootstrap sequence

How `main()` wires the components together at startup.

```mermaid
flowchart TD
    MAIN["main()"]
    MAIN -->|"loadConfig()"| CFG["Config\nconfig.json"]
    MAIN -->|"NewSidecar(cfg)"| SC["Sidecar"]
    MAIN -->|"NewHandlers(cfg, sidecar)"| H["Handlers"]
    MAIN -->|"staticHandler()"| STATIC["Static files\ndev: disk · prod: embed.FS"]
    MAIN -->|"GenerateSelfSignedCert()\n(first run only)"| SSL[".ssl/cert.pem\n.ssl/key.pem"]
    MAIN -->|"ListenAndServe :9380"| HTTP["HTTP server"]
    MAIN -->|"ListenAndServeTLS :9381"| HTTPS["HTTPS server"]
    HTTP  --> MUX["http.ServeMux\nrouter"]
    HTTPS --> MUX
    MUX --> H
    MUX --> STATIC
```

---

### 2 — Chat request flow

Path of a user message from the browser to the model and back.

```mermaid
sequenceDiagram
    participant B as Browser / Mobile
    participant H as handlers.go<br/>HandleAsk
    participant L as llm.go<br/>LLMClient
    participant S as llama-server<br/>(child process)

    B->>H: POST /ask (text + optional image)
    H->>L: Chat(messages, imageFile)
    L->>L: loadImageAsBase64(file)
    L->>S: POST /v1/chat/completions<br/>(stream: true)
    loop SSE token stream
        S-->>L: chunk {delta.content}
        L-->>H: callback(token)
        H-->>B: data: token\n\n
    end
    S-->>L: finish_reason: stop
    H-->>B: data: [DONE]
```

---

### 3 — llama-server lifecycle

How the sidecar starts, waits for readiness, and shuts down.

```mermaid
flowchart TD
    A["HandleLlamaStart\n(admin dashboard)"] -->|"sidecar.Start()"| B{"binary &\nmodel exist?"}
    B -->|no| ERR["return error\nto dashboard"]
    B -->|yes| C["findFreePort(preferred)"]
    C --> D["exec.Cmd\nllama-server --model ... --port ..."]
    D -->|"LD_LIBRARY_PATH=bin/OS/"| E["llama-server running"]
    E --> F["WaitReady()\npoll GET /health"]
    F -->|timeout| ERR2["return error"]
    F -->|200 OK| G["sidecar.IsRunning() = true"]

    H2["HandleLlamaStop\nor SIGINT/SIGTERM"] -->|"sidecar.Stop()"| I["cmd.Process.Kill()"]
    I --> J["sidecar.IsRunning() = false"]
```

---

### 4 — llama-server download & extraction

How a release is selected, downloaded, and installed.

```mermaid
flowchart TD
    A["Admin clicks\n'Load releases'"] -->|"GET /api/llama/releases"| B["HandleLlamaReleases"]
    B -->|"GitHub API\nreleases?per_page=15"| C["Score assets\nby platform & priority"]
    C -->|"top 10 entries\ntag · name · url"| A

    A2["Admin selects\na release + clicks Download"] -->|"POST /api/llama/select\n?tag=bXXXX&url=..."| D["Save tag + URL\nto config.json"]
    D -->|"GET /api/download?target=llama"| E["HandleDownload"]
    E -->|"HTTP GET asset URL"| F["Stream to\nbin/OS/llama-server.tar.gz\n(SSE progress)"]
    F --> G{"extract OK?"}
    G -->|no| H["Keep archive on disk\nfor manual inspection"]
    G -->|yes| I["extractLlamaBinary()\n→ llama-server + .so/.dll"]
    I --> J["cleanBinDir()\nremove old version files"]
    J --> K["os.Remove archive"]
```

---

### 5 — Configuration & infrastructure

Config persistence, TLS certificate, and system monitoring.

```mermaid
flowchart LR
    subgraph Config["config.go"]
        CFG["Config struct\nLlamaLocal · LlamaRemote · ModelURLs"]
        CFG -->|"Save()"| DISK[(config.json)]
        DISK -->|"loadConfig()"| CFG
    end

    subgraph Cert["cert.go"]
        GEN["GenerateSelfSignedCert()\nRSA-2048 + X.509 SANs"]
        GEN --> SSL2[".ssl/cert.pem\n.ssl/key.pem"]
        SANS["DefaultCertSANs()\nall LAN IPs + hostnames"] --> GEN
    end

    subgraph SysInfo["sysinfo.go"]
        SI["GetSysInfo()"]
        SI --> RAM["RAM used/total\ngopsutil/mem"]
        SI --> CPU["CPU %\ngopsutil/cpu"]
        SI --> DISK2["Disk used/total\ngopsutil/disk"]
        SI --> GPU["GPU VRAM\nnvidia-smi (optional)"]
    end

    H3["handlers.go"] -->|"HandleConfig"| CFG
    H3 -->|"HandleCertRegenerate"| GEN
    H3 -->|"HandleStatus\nHandleSysInfo"| SI
```
