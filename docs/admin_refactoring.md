# GemmaLink Admin — Refactoring Plan (SAR Architecture)

Step-by-step guide. Complete one step, verify, then move to the next.

---

## Step 1 — UI Object (Store-aligned keys, by card/section)

Replace the flat `UI` cache with a namespaced version where **each key mirrors the corresponding `Store` key**.
No `document.getElementById()` will exist outside this object after Step 14.

| UI key | Store key | Card / section |
|---|---|---|
| `UI.qr` | `Store.qr` | QR tab |
| `UI.sys` | `Store.sys` | System Resources card |
| `UI.server` | `Store.server` | Server Status card |
| `UI.bin` | `Store.bin` | llama-server Binary card |
| `UI.model` | `Store.model` | Gemma Model card |
| `UI.log` | `Store.log` | Activity Log |
| `UI.config` | `Store.config` | All Config tab elements |

```js
const UI = {
    // ── Store.qr ──────────────────────────────────────────
    qr: {
        img: document.getElementById('qr-img'),
        url: document.getElementById('qr-url'),
    },

    // ── Store.sys ─────────────────────────────────────────
    sys: {
        ramBar:  document.getElementById('ram-bar'),
        ramVal:  document.getElementById('ram-label'),
        cpuBar:  document.getElementById('cpu-bar'),
        cpuVal:  document.getElementById('cpu-label'),
        diskBar: document.getElementById('disk-bar'),
        diskVal: document.getElementById('disk-label'),
        gpuInfo: document.getElementById('gpu-info'),
        refreshInput: document.getElementById('sysinfo-refresh'),
    },

    // ── Store.server ───────────────────────────────────────
    server: {
        dotServer:    document.getElementById('dot-server'),
        valServer:    document.getElementById('val-server'),
        dotLlama:     document.getElementById('dot-llama'),
        valLlama:     document.getElementById('val-llama'),
        dotModel:     document.getElementById('dot-model'),
        valModel:     document.getElementById('val-model'),
        rowStop:      document.getElementById('row-stop'),
        btnStart:     document.getElementById('btn-start-llama'),
        btnStop:      document.getElementById('btn-stop-llama'),
        remoteNotice: document.getElementById('remote-notice'),
    },

    // ── Store.bin ──────────────────────────────────────────
    bin: {
        card:        document.getElementById('card-binary'),
        selector:    document.getElementById('llama-selector'),
        dotBin:      document.getElementById('dot-bin'),
        labelBin:    document.getElementById('label-bin'),
        valBin:      document.getElementById('val-bin'),
        activeBadge: document.getElementById('llama-active-badge'),
        btnDl:       document.getElementById('btn-dl-llama'),
        btnCancel:   document.getElementById('btn-cancel-dl-llama'),
        btnUse:      document.getElementById('btn-use-llama'),
        progWrap:    document.getElementById('prog-llama-wrap'),
        progFill:    document.getElementById('prog-llama-fill'),
        progLabel:   document.getElementById('prog-llama-label'),
    },

    // ── Store.model ────────────────────────────────────────
    model: {
        card:         document.getElementById('card-model'),
        selector:     document.getElementById('model-selector'),
        visionToggle: document.getElementById('vision-toggle'),
        activeBadge:  document.getElementById('model-active-badge'),
        btnDl:        document.getElementById('btn-dl-model'),
        btnCancel:    document.getElementById('btn-cancel-dl'),
        btnUse:       document.getElementById('btn-use-model'),
        progWrap:     document.getElementById('prog-model-wrap'),
        progFill:     document.getElementById('prog-model-fill'),
        progLabel:    document.getElementById('prog-model-label'),
    },

    // ── Store.log ──────────────────────────────────────────
    log: {
        box: document.getElementById('log-box'),
    },

    // ── Store.config — all Config tab elements ──────────────
    config: {
        httpHost:        document.getElementById('cfg-http-host'),
        httpPort:        document.getElementById('cfg-http-port'),
        httpsPort:       document.getElementById('cfg-https-port'),
        uploadDir:       document.getElementById('cfg-upload-dir'),
        restartWarn:     document.getElementById('cfg-restart-warn'),
        radioLocal:      document.getElementById('radio-local'),
        radioRemote:     document.getElementById('radio-remote'),
        lblLocal:        document.getElementById('lbl-local'),
        lblRemote:       document.getElementById('lbl-remote'),
        panelLocal:      document.getElementById('panel-local'),
        panelRemote:     document.getElementById('panel-remote'),
        localEndpoint:   document.getElementById('cfg-local-endpoint'),
        btnTestLocal:    document.getElementById('btn-test-local'),
        connStatusLocal: document.getElementById('conn-status-local'),
        llamaBin:        document.getElementById('cfg-llama-bin'),
        llamaBinVersion: document.getElementById('cfg-llama-bin-version'),
        modelPath:       document.getElementById('cfg-model-path'),
        mmprojPath:      document.getElementById('cfg-mmproj-path'),
        contextSize:     document.getElementById('cfg-context-size'),
        remoteEndpoint:      document.getElementById('cfg-remote-endpoint'),
        btnTestRemote:       document.getElementById('btn-test-remote'),
        connStatusRemote:    document.getElementById('conn-status-remote'),
        sectionModelUrls: document.getElementById('section-model-urls'),
        urlE2b:           document.getElementById('cfg-url-e2b'),
        btnResetE2b:      document.getElementById('btn-reset-e2b'),
        urlE4b:           document.getElementById('cfg-url-e4b'),
        btnResetE4b:      document.getElementById('btn-reset-e4b'),
        btnSave:    document.getElementById('btn-save-cfg'),
        cfgStatus:  document.getElementById('cfg-status'),
        btnRegen:   document.getElementById('btn-regen-cert'),
        certStatus: document.getElementById('cert-status'),
    },
};
```

**Verify:** No errors in console on page load. All `UI.x.y` resolve correctly.

---

## Step 2 — Renderer.sys (bar colors)

Update `Renderer.sys` to apply warn/crit CSS classes on the bars (>69% → warn, >89% → crit),
matching the behaviour lost from `.bak`.

```js
// helper (replaces the bare style.width assignment)
function setBar(el, pct) {
    el.style.width = `${pct}%`;
    el.className = 'bar-fill' + (pct > 89 ? ' crit' : pct > 69 ? ' warn' : '');
}
```

All three bars in `Renderer.sys` use `setBar()` instead of direct `style.width`.

**Verify:** Load page, confirm bars turn yellow at ~70% and red at ~90%.

---

## Step 3 — Renderer.server (applyRemoteMode)

Extend `Renderer.server` to also toggle the cards and sections that should be hidden
in remote mode: `card-binary`, `card-model`, `row-stop`, `remote-notice`, `section-model-urls`.

```js
// inside Renderer.server, after the existing dot/button logic:
const hide = (el, yes) => el.style.display = yes ? 'none' : '';
const show = (el, yes) => el.style.display = yes ? '' : 'none';

hide(UI.bin.card,            isRemote);
hide(UI.model.card,          isRemote);
hide(UI.server.rowStop,      isRemote);
show(UI.server.remoteNotice, isRemote);
hide(UI.config.sectionModelUrls, isRemote);
```

**Verify:** Switch mode via radio buttons, confirm cards appear/disappear correctly.

---

## Step 4 — Renderer.config (missing fields + setLlamaMode)

Add the fields missing from the current `Renderer.config`:
- `cfg-upload-dir`
- `cfg-llama-bin-version`
- `cfg-mmproj-path`

Replace the simple radio check with a `setLlamaMode(mode)` helper that also
updates the `active` CSS class on `lbl-local` / `lbl-remote`.

**Verify:** Load Config tab, confirm all fields are populated correctly.

---

## Step 5 — Renderer.model (RAM/VRAM filter)

Add memory-aware filtering to `Renderer.model`: variants that require more RAM/VRAM than
available are hidden from the selector.

```js
const MODEL_MIN_RAM = {
    e2b:   4  * 1073741824,
    e4b:   6  * 1073741824,
    e31b:  24 * 1073741824,
};
```

Use `Store.sys.data` (ram_total / gpu_vram_total) to decide which options to render.

**Verify:** On a low-RAM machine (simulated or real) e4b/e31b options disappear.

---

## Step 6 — streamDownload() utility

Port the SSE streaming progress helper from `.bak`. This is the foundation for Steps 7 and 8.

```js
// Returns true on success, false on cancel/error.
async function streamDownload(url, progFill, progLabel, signal) { ... }
```

No UI wiring in this step — just the function.

**Verify:** Function exists and is callable (no console errors on load).

---

## Step 7 — llama-server Binary download handlers

Wire up all three buttons in `UI.Binary`:
- `btnDl` → select asset URL from `Store.bin.data.releases`, call `streamDownload`
- `btnCancel` → abort the active `AbortController`
- `btnUse` → `POST /api/llama/select`

Use a `llamaDownloadController` variable in module scope.
After each action call `Actions.sync('server', ...)` to refresh status.

**Verify:** Download a release, verify progress bar, cancel mid-download, verify cleanup.

---

## Step 8 — Gemma Model download handlers + vision toggle

Wire up `UI.Model`:
- `btnDl` → `downloadModel(variant)` using `streamDownload`
- `btnCancel` → abort `activeDownloadController`
- `btnUse` → `POST /api/model/select?variant=...`
- `visionToggle` → download mmproj if not present, then `POST /api/vision/toggle`

**Verify:** Download e2b model, switch variant, toggle vision on with missing mmproj.

---

## Step 9 — Start / Stop llama-server handlers

Wire `UI.Server.btnStart` and `UI.Server.btnStop` with button-disabling feedback
during the async call, matching `.bak` behaviour.

**Verify:** Start and stop llama-server from the dashboard, observe button states.

---

## Step 10 — Actions.saveConfig (complete)

Replace the stub in `Actions.saveConfig` with the full implementation:
- All config fields including `upload_dir`, `mmproj_path`, `llama_bin_version`
- `cfg-status` feedback ("Saved ✓" / error colour)
- `cfg-restart-warn` shown when ports change (check `res.ports_changed`)
- Auto-switch to Dashboard tab on success

**Verify:** Save config, modify a port, verify restart warning appears.

---

## Step 11 — Test Connection handlers

Replace `Actions.testConn(mode)` with `testEndpoint(endpoint, statusEl)` that renders
badge markup (`badge-ok` / `badge-err`) instead of plain text, matching `.bak`.

**Verify:** Test both local and remote endpoints, verify badge colours.

---

## Step 12 — SSL Certificate regenerate handler

Wire `UI.CfgActions.btnRegen` → `POST /api/cert/regenerate` with:
- "Generating…" feedback in `cert-status`
- "Generated ✓" / "Failed" result with 5s auto-clear

**Verify:** Click Regenerate, confirm feedback and log entry.

---

## Step 13 — Sysinfo refresh live-save (debounced)

Add an `input` event listener on `UI.Resources.refreshInput` that:
1. Immediately applies the new interval via `Actions.applySysinfoRefresh()`
2. After 800ms debounce, `POST /api/config` with only `sysinfo_refresh_seconds`

**Verify:** Change refresh value, confirm interval updates, confirm PATCH saved without full reload.

---

## Step 14 — Final cleanup

- Remove `admin.js.bak`
- Audit entire file: confirm zero `document.getElementById` calls outside `UI`
- Confirm no `document.querySelector` with IDs (use `UI` instead)
- Confirm all `Store[k].dirty` paths are covered by a `Renderer[k]`

**Verify:** Full end-to-end test of all tabs and interactions.
