# Eye Assistant Architecture

GemmaLink: Eye Assistant is a **Zero-Footprint**, private AI ecosystem designed to bridge a mobile device's camera with a local AI model (Gemma 4) running on a workstation. It eliminates the need for cloud intermediaries or external tunnels, maintaining 100% data sovereignty.

## Architectural Pillars
*   **Privacy-First:** No external data egress. All inference and transport remain within the local subnet.
*   **Minimalist Deployment:** A single Go binary manages the life cycle of the entire stack.
*   **Backend Agility:** The system dynamically identifies and scores available hardware backends (Vulkan vs AVX2) to ensure the best performance on consumer hardware.
*   **Technical Abstraction:** The orchestrator masks the complexity of local LLM management. It handles binary scoring, port allocation, and health monitoring, providing a "consumer-grade" experience for a high-end technical stack.

## Technical Stack
*   **Backend & Orchestrator:** Written in **Go** for high-concurrency process management and minimal memory footprint.
*   **Inference Engine:** `llama-server` (llama.cpp) running as a sidecar process.
*   **Model:** **Gemma 4**, specifically leveraged for its multimodal vision-to-text reasoning capabilities.
*   **Desktop UI:** Wails (Go + Svelte) providing a native OS experience.
*   **Mobile Lens:** A lightweight JS/HTML5 interface served via the internal Go HTTP server.

## System Components

```mermaid
graph TD
    subgraph "Mobile Device (The Eye)"
        A[Mobile Web Lens]
    end
    
    subgraph "Local PC (The Brain)"
        B[Go Orchestrator / Wails]
        C[Inference Sidecar: llama-server]
        D[Gemma 4 GGUF Model]
    end

    A <-->|Secure LAN HTTPS| B
    B <-->|Localhost JSON API| C
    C <-->|Memory Map| D

```

## Data Flow & Security

1. **Secure Context:** The Go backend generates a self-signed TLS certificate to satisfy browser requirements for `getUserMedia` (camera access) without external CA validation.
2. **Inference Pipeline:** Frames are captured by the "Lens", transmitted over the LAN via HTTPS, and fed directly into the sidecar's JSON API.
3. **Process Management:** The orchestrator monitors the sidecar's health and manages the MMAP'd model memory to prevent leaks and ensure a clean exit.

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
