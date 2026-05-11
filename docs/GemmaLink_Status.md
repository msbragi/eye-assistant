# GemmaLink — Stato del Progetto
**Aggiornato:** 12 Maggio 2026  
**Submission Deadline:** 24 Maggio 2026 (12 giorni)  
**Repository:** https://github.com/msbragi/eye-assistant  
**Ultimo commit verificato:** in corso

> **Stato:** ✅ Build & release system completato. In corso: test E2E su Windows (download llama-server → modello → inferenza).

---

## Stack Tecnico

| Componente | Dettaglio |
|---|---|
| Linguaggio | Go 1.22.4 — `/usr/local/go` in WSL Debian 13 |
| Modulo Go | `gemmalink` |
| LLM backend | `llama-server` (llama.cpp) — API OpenAI-compatible |
| Modello target | Gemma 4 E4B (4B params) — `models/gemma-4-e4b.gguf` |
| UI mobile | `/eye.html` — pura HTML/JS, camera `getUserMedia` |
| UI admin | `/admin.html` — dashboard PC con tab QR/Dashboard/Config |
| TLS | Self-signed cert Go puro — `.ssl/cert.pem` / `.ssl/key.pem` |
| HTTP | porta 8080 (plain) + 8443 (HTTPS, richiesto per camera mobile) |
| QR library | `github.com/skip2/go-qrcode` |
| Sysinfo | `github.com/shirou/gopsutil/v3` |

---

## Struttura File (stato attuale)

```
eye-assistant/
├── main.go           # entry point, routing, UA redirect server-side, cert auto-gen
├── config.go         # Config structs, load/save, costanti path/URL modelli
├── handlers.go       # tutti gli HTTP handlers, binary extraction (tar.gz/zip)
├── llm.go            # LLMClient OpenAI-compatible
├── sidecar.go        # gestione processo llama-server, LD_LIBRARY_PATH, waitForHealth
├── cert.go           # generazione self-signed cert TLS (auto al primo avvio)
├── sysinfo.go        # CPU/RAM/Disk via gopsutil
├── static_dev.go     # [build tag: dev] static serving da disco
├── static_prod.go    # [build tag: !dev] static embedded via go:embed
├── config.json       # configurazione runtime
├── go.mod / go.sum
├── scripts/
│   ├── build.sh             # release build: win|linux|all + versione → dist/releases/
│   ├── build-dev.sh         # dev build con -tags dev
│   └── wsl-portforward.ps1  # port forward Windows→WSL2 (run as Admin)
├── static/
│   ├── index.html    # fallback (redirect UA ora server-side in main.go)
│   ├── eye.html      # interfaccia smartphone (camera + ask)
│   ├── admin.html    # dashboard PC [QR Code][Dashboard][Config]
│   ├── css/
│   │   ├── eye.css
│   │   └── admin.css
│   └── js/
│       └── admin.js  # logica JS dashboard, memory-based model filtering
├── bin/
│   ├── linux/        # llama-server + .so + symlinks soname
│   └── windows/      # llama-server.exe + .dll (scaricato dal dashboard)
├── models/           # gemma-4-e2b.gguf / mmproj (scaricati dal dashboard)
├── uploads/          # immagini temporanee
└── dist/             # [gitignore] staging + release artifacts
    ├── staging/      # dir riusata tra build
    └── releases/     # gemmalink-{ver}-{os}-amd64.{tar.gz|zip}
```

---

## config.json (struttura attuale)

```json
{
  "llama_local": {
    "enabled": true,
    "endpoint": "http://localhost:11434",
    "model_path": "models/gemma-4-e2b.gguf",
    "mmproj_path": "models/mmproj-gemma-4-e2b.gguf",
    "vision_enabled": true,
    "llama_bin": "bin/linux/llama-server",
    "llama_bin_version": "b9102",
    "context_size": 4096
  },
  "llama_remote": {
    "enabled": false,
    "endpoint": "http://192.168.1.32:11434"
  },
  "model_urls": {
    "e2b": "<HuggingFace URL gemma-4-e2b Q4_K_M>",
    "e4b": "<HuggingFace URL gemma-4-e4b Q4_K_M>"
  }
}
```

---

## API Endpoints

| Method | Path | Descrizione |
|---|---|---|
| POST | `/ask` | multipart: image+question → SSE streaming LLM response |
| GET | `/api/status` | stato server, llama-server, bin/model presente, vision_enabled |
| GET | `/api/sysinfo` | CPU/RAM/Disk/GPU |
| GET | `/api/config` | config corrente JSON |
| POST | `/api/config` | salva config |
| POST | `/api/llama/start` | avvia llama-server sidecar (attende readiness) |
| POST | `/api/llama/stop` | ferma llama-server sidecar |
| POST | `/api/vision/toggle` | abilita/disabilita mmproj, salva VisionEnabled |
| GET | `/api/download?target=` | SSE progress: `llama`, `model-e2b`, `model-e4b`, `mmproj-e2b`, `mmproj-e4b` |
| GET | `/api/llama/releases` | lista release llama.cpp da GitHub |
| GET | `/api/llama/test?endpoint=` | verifica raggiungibilità llama-server |
| POST | `/api/cert/regenerate` | rigenera SSL cert con SANs aggiornati |
| GET | `/api/qr` | PNG QR code (256×256) per mobile URL |
| GET | `/api/qr-url` | JSON `{"url":"https://..."}` URL mobile |
| GET | `/` | static file server → index.html |

---

## Funzionamento WSL2 / Networking

**Problema:** Il server gira in WSL2 (IP interno `172.x.x.x`). Lo smartphone chiama l'IP Windows (`192.168.0.65`).

**Soluzione:** `scripts/wsl-portforward.ps1` — da eseguire su **Windows come Administrator** ad ogni riavvio WSL2:

```powershell
# In PowerShell Admin su Windows
cd C:\Sviluppo\GO
PowerShell -ExecutionPolicy Bypass -File .\wsl-portforward.ps1
```

Fa `netsh portproxy`: `0.0.0.0:8080/8443` → `<WSL2-IP>:8080/8443`

**IP rilevamento mobile (WSL-aware in `main.go`):**
- `getMobileIP()` → `isWSL()` → se WSL2: `wslHostIP()`
- `wslHostIP()` chiama `powershell.exe Get-NetIPAddress` (primario) oppure `ipconfig.exe` (fallback), esclude `127.*`, `169.*`, `172.*`
- Restituisce `192.168.0.65` → il QR code punta a `https://192.168.0.65:8443/eye.html`

---

## ✅ Completato

- [x] **Go backend completo** — main.go, config.go, handlers.go, llm.go, sidecar.go, cert.go, sysinfo.go
- [x] **Config struttura nested** — `llama_local` / `llama_remote` con `IsRemote()` / `ActiveEndpoint()`
- [x] **VisionEnabled** — flag in config, toggle API, `--mmproj` condizionale al sidecar
- [x] **Modalità remote** — llama-server su altro PC, sidecar skip in remote mode
- [x] **SSL self-signed** — generazione automatica al primo avvio se `.ssl/cert.pem` assente
- [x] **SSE streaming** — `/ask` e `/api/download` usano Server-Sent Events
- [x] **eye.html** — camera, capture, preset mode, streaming response, status dot
- [x] **admin.html + admin.js** — 3 tab: [QR Code] [Dashboard] [Config]
- [x] **Binary extraction** — tar.gz (Linux): binario + .so + symlinks soname; zip (Windows): exe + dll
- [x] **LD_LIBRARY_PATH** — impostato automaticamente al lancio del sidecar su Linux
- [x] **waitForHealth** — polling `/health` → `{"status":"ok"}`
- [x] **Start/Stop llama-server** — bottoni dashboard, stato sincronizzato
- [x] **llama releases** — dropdown versioni da GitHub (parsata dal body release, non dagli assets)
- [x] **Memory filtering modelli** — dropdown mostra solo modelli compatibili con RAM/VRAM disponibile
- [x] **Build tags dev/prod** — `static_dev.go` (disco) / `static_prod.go` (go:embed); `go build -tags dev`
- [x] **Versioning ldflags** — `var Version = "dev"`, iniettato con `-ldflags "-X main.Version=x.y.z"`
- [x] **UA redirect server-side** — `main.go` gestisce `/` con `Cache-Control: no-store` (fix Firefox cache)
- [x] **Build & release system** — `scripts/build.sh <win|linux|all> <version>` → `dist/releases/`
- [x] **Cross-compile** — Linux tar.gz + Windows zip da WSL, testati con `gemmalink.exe` su Windows host
- [x] **Fix config.json Windows** — heredoc `<<'EOF'` per backslash corretti in JSON

---

## 🗓 Da Fare (prossima sessione)

### 1. Fix download llama-server (404)
- `HandleDownload` costruisce l'URL con vecchio pattern — aggiornare per usare l'URL già noto dalla release
- Verificare come `admin.js` passa la URL a `/api/download`

### 2. URL releases in config.json
- Spostare `https://api.github.com/repos/ggerganov/llama.cpp/releases?per_page=15` in `config.json`
- Così configurabile senza ricompilare

### 3. Test E2E su Windows
- Download llama-server Windows → estrazione exe + dll
- Download modello E2B
- Start llama-server → test `/ask` con foto

### 4. README finale
Documentare setup, avvio, download modelli, WSL2 note, screenshot.

---

## Come Avviare il Server

```bash
cd /workspace/Go/eye-assistant
go build . && ./gemmalink
```

- Admin PC: `http://localhost:8080`  
- Mobile: `https://192.168.0.65:8443/eye.html` (dopo aver eseguito `wsl-portforward.ps1`)

**Prima di usare da smartphone:**
1. Eseguire `wsl-portforward.ps1` come Admin su Windows
2. Aprire `https://192.168.0.65:8443` sul browser mobile → accettare certificato
3. Navigare a `/eye.html`

---

## Note per Nuova Sessione

- Iniziare da: **punto 3 (verificare URL download)** → poi **punto 1 e 2 (download binary+model)** → **punto 4 (test E2E)**
- Il codice compila pulito (`go build .` — zero errori)
- Il QR code funziona e punta all'IP corretto (`192.168.0.65`)
- Il port forwarding WSL→Windows funziona
- Manca solo il motore (llama-server + modello GGUF) per avere l'app funzionante end-to-end

## Creazione release
- gemmalink.exe per windows 
- gemmalink per linux / wsl 
- I file statici devono essere embed negli eseguibili  
- Creare lo zip per windows ed il tar per linux
.
├── config.json
├── gemmalink || gemmalink.exe
├── README.md
├── .ssl
│   └── .keep
├── bin
│   ├── linux
│   │   └── .keep
│   └── windows
│       └── .keep
├── models
│   └── .keep
├── scripts
│   └── wsl-portforward.ps1
└── uploads 
    └── .keep
