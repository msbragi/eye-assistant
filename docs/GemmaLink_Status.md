# GemmaLink — Stato del Progetto
**Aggiornato:** 9 Maggio 2026  
**Submission Deadline:** 24 Maggio 2026 (15 giorni)  
**Repository:** https://github.com/msbragi/eye-assistant  
**Ultimo commit verificato:** `3a7ddd9` — "Refactor: nested config structure + remote llama-server support"

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
├── main.go          # entry point, routing, getLANIP/getMobileIP/isWSL/wslHostIP
├── config.go        # Config/LlamaLocal/LlamaRemote structs, load/save
├── handlers.go      # tutti gli HTTP handlers
├── llm.go           # LLMClient OpenAI-compatible
├── sidecar.go       # gestione processo llama-server
├── cert.go          # generazione self-signed cert TLS
├── sysinfo.go       # CPU/RAM/Disk via gopsutil
├── config.json      # configurazione runtime
├── go.mod / go.sum
├── scripts/
│   └── wsl-portforward.ps1   # port forward Windows→WSL2 (run as Admin)
├── static/
│   ├── index.html   # UA redirect: mobile→eye.html, desktop→admin.html
│   ├── eye.html     # interfaccia smartphone (camera + ask)
│   ├── admin.html   # dashboard PC [QR Code][Dashboard][Config]
│   └── css/
│       ├── eye.css
│       └── admin.css
├── bin/
│   └── linux/       # llama-server binary (DA SCARICARE)
├── models/          # gemma-4-e4b.gguf (DA SCARICARE)
└── uploads/         # immagini temporanee
```

---

## config.json (struttura attuale)

```json
{
  "http_host": "localhost",
  "http_port": "8080",
  "https_port": "8443",
  "llama_local": {
    "enabled": true,
    "endpoint": "http://localhost:11434",
    "model_path": "models/gemma-4-e4b.gguf",
    "llama_bin": "bin/linux/llama-server",
    "context_size": 4096
  },
  "llama_remote": {
    "enabled": false,
    "endpoint": "http://192.168.1.32:11434"
  },
  "upload_dir": "uploads"
}
```

---

## API Endpoints

| Method | Path | Descrizione |
|---|---|---|
| POST | `/ask` | multipart: image+question → SSE streaming LLM response |
| GET | `/api/status` | stato server, llama-server, bin/model presente |
| GET | `/api/sysinfo` | CPU/RAM/Disk/GPU |
| GET | `/api/config` | config corrente JSON |
| POST | `/api/config` | salva config |
| POST | `/api/llama/stop` | ferma llama-server sidecar |
| GET | `/api/download?target=` | SSE progress: `llama`, `model-e2b`, `model-e4b` |
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
- [x] **Modalità remote** — llama-server su altro PC, sidecar skip in remote mode
- [x] **SSL self-signed** — generazione automatica all'avvio se assente, regen via admin
- [x] **SSE streaming** — `/ask` e `/api/download` usano Server-Sent Events
- [x] **eye.html** — camera, capture, preset mode, streaming response, status dot
- [x] **admin.html** — 3 tab: [QR Code] [Dashboard] [Config]
  - Tab QR: immagine QR + URL testuale
  - Tab Dashboard: sysinfo bars, server status, binary/model cards, activity log
  - Tab Config: radio Local/Remote, form completo, test connection, SSL regen
- [x] **admin.css** — CSS estratto in file separato
- [x] **QR code** — `/api/qr` PNG + `/api/qr-url` JSON, WSL-aware IP detection
- [x] **WSL2 port forwarding** — script PowerShell `wsl-portforward.ps1`
- [x] **Build pulito** — `go build .` senza errori

---

## ❌ Da Fare (priorità)

### 1. Scaricare llama-server binary (BLOCCANTE per test locale)
Il binary non è presente. Dal tab Dashboard → pulsante "Download llama-server".  
Oppure manualmente:
```bash
# URL da verificare in handlers.go HandleDownload, target=llama
```
Verificare che `handlers.go` → `HandleDownload` abbia l'URL corretto per la build Linux.

### 2. Scaricare modello Gemma 4 E4B (BLOCCANTE per inferenza)
File GGUF da ~4GB. Dal tab Dashboard → "Download E4B (4B)".  
Verificare URL in `HandleDownload`, target=`model-e4b`.

### 3. Verificare URL download in HandleDownload
Aprire `handlers.go`, cercare `HandleDownload`, verificare che gli URL per `llama` e `model-e4b` siano corretti e attivi (Hugging Face o release GitHub llama.cpp).

### 4. Test end-to-end
1. Avviare `./gemmalink`
2. Aprire `http://localhost:8080` su PC → redirect a admin
3. Tab QR → scansionare con smartphone
4. Smartphone → accettare certificato self-signed → eye.html
5. Foto → domanda → risposta streaming

### 5. System prompt ottimizzato per Gemma 4
In `handlers.go → HandleAsk`: il system prompt attuale è generico.  
Ottimizzare per "assistente visivo" — descrive oggetti, simboli, etichette.

### 6. Certificato SSL su smartphone
Prima visita: il browser mobile mostrerà warning per cert self-signed.  
Il tab QR già avvisa l'utente. Considerare se aggiungere istruzioni più dettagliate.

### 7. Commit modifiche pendenti
Le seguenti modifiche non sono ancora committate:
- QR code feature (handlers.go, main.go, admin.html)
- admin.css estratto
- eye.html `/api/status` fix
- `applyRemoteMode()` bug fix
- WSL IP detection

```bash
cd /workspace/Go/eye-assistant
git add -A
git commit -m "feat: QR code tab, WSL-aware IP detection, admin.css"
git push
```

### 8. (Opzionale) Ricarica cert TLS a caldo
Attualmente il nuovo cert dopo "Regenerate" viene usato solo per nuove connessioni TLS.  
Per un riavvio del listener TLS senza kill del processo servirebbe `crypto/tls.Config` con `GetCertificate`.

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
