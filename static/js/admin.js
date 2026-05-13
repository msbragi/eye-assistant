/**
 * GemmaLink Admin — Definitive Refactoring
 * Architecture: State-Action-Renderer (SAR)
 */

// --- 1. CONFIG & STATE ---
const DEFAULT_URLS = {
    e2b: 'https://huggingface.co/lmstudio-community/gemma-4-E2B-it-GGUF/resolve/main/gemma-4-E2B-it-Q4_K_M.gguf',
    e4b: 'https://huggingface.co/lmstudio-community/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-E4B-it-Q4_K_M.gguf'
};

const MODEL_LABELS = {
    e2b:  'Gemma 4 E2B (2B params)',
    e4b:  'Gemma 4 E4B (4B params)',
    e31b: 'Gemma 4 E31B (31B params)',
};

const MODEL_MIN_RAM = {
    e2b:   4 * 1073741824,   //  4 GB
    e4b:   6 * 1073741824,   //  6 GB
    e31b: 24 * 1073741824,   // 24 GB
};

const Store = {
    sys: { data: {}, dirty: false },
    server: { data: {}, dirty: false },
    config: { data: {}, dirty: false },
    qr: { data: { url: '', img: '' }, dirty: false },
    model: { data: { variants: {}, activePath: '', vision: false }, dirty: false },
    log: { data: [], dirty: false },
    dl: { data: { pct: 0, label: '', visible: false, type: 'model' }, dirty: false },
    bin: { data: { releases: [] }, dirty: false }
};

let pollSysInterval = null;

// --- 2. UI CACHE ---
// Keys mirror Store keys so Store.X <-> UI.X
const UI = {
    // ── Store.qr ──────────────────────────────────────────
    qr: {
        img: document.getElementById('qr-img'),
        url: document.getElementById('qr-url'),
    },

    // ── Store.sys ─────────────────────────────────────────
    sys: {
        ramBar:       document.getElementById('ram-bar'),
        ramVal:       document.getElementById('ram-label'),
        cpuBar:       document.getElementById('cpu-bar'),
        cpuVal:       document.getElementById('cpu-label'),
        diskBar:      document.getElementById('disk-bar'),
        diskVal:      document.getElementById('disk-label'),
        gpuInfo:      document.getElementById('gpu-info'),
        refreshInput: document.getElementById('sysinfo-refresh'),
    },

    // ── Store.server ──────────────────────────────────────
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

    // ── Store.bin ─────────────────────────────────────────
    bin: {
        card:        document.getElementById('card-binary'),
        selector:    document.getElementById('llama-selector'),
        btnRefresh:  document.getElementById('btn-refresh-llama'),
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

    // ── Store.model ───────────────────────────────────────
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

    // ── Store.log ─────────────────────────────────────────
    log: {
        box: document.getElementById('log-box'),
    },

    // ── Store.config — GemmaLink Server ───────────────────
    config: {
        httpHost:        document.getElementById('cfg-http-host'),
        httpPort:        document.getElementById('cfg-http-port'),
        httpsPort:       document.getElementById('cfg-https-port'),
        uploadDir:       document.getElementById('cfg-upload-dir'),
        restartWarn:     document.getElementById('cfg-restart-warn'),
        // Inference mode
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
        // Download URLs
        sectionModelUrls: document.getElementById('section-model-urls'),
        urlE2b:           document.getElementById('cfg-url-e2b'),
        btnResetE2b:      document.getElementById('btn-reset-e2b'),
        urlE4b:           document.getElementById('cfg-url-e4b'),
        btnResetE4b:      document.getElementById('btn-reset-e4b'),
        // Save / SSL
        btnSave:    document.getElementById('btn-save-cfg'),
        cfgStatus:  document.getElementById('cfg-status'),
        btnRegen:   document.getElementById('btn-regen-cert'),
        certStatus: document.getElementById('cert-status'),
    },

    // ── Tab panels (not Store-backed, UI-only) ────────────
    tabs: {
        qr:        document.getElementById('tab-qr'),
        dashboard: document.getElementById('tab-dashboard'),
        config:    document.getElementById('tab-config'),
        btns:      document.querySelectorAll('.tab-btn'),
    },
};

// --- 3. RENDERERS ---
function setBar(el, pct) {
    el.style.width = `${pct}%`;
    el.className = 'bar-fill' + (pct > 89 ? ' crit' : pct > 69 ? ' warn' : '');
}

const Renderer = {
    sys: (d) => {
        const rPct = Math.round(d.ram_used / d.ram_total * 100);
        setBar(UI.sys.ramBar, rPct);
        UI.sys.ramVal.textContent = `${fmtGB(d.ram_used)} / ${fmtGB(d.ram_total)} (${rPct}%)`;
        const cPct = Math.round(d.cpu_pct);
        setBar(UI.sys.cpuBar, cPct);
        UI.sys.cpuVal.textContent = `${d.cpu_pct.toFixed(1)}%`;
        const dPct = Math.round(d.disk_used / d.disk_total * 100);
        setBar(UI.sys.diskBar, dPct);
        UI.sys.diskVal.textContent = `${fmtGB(d.disk_used)} / ${fmtGB(d.disk_total)} (${dPct}%)`;
        UI.sys.gpuInfo.textContent = d.gpu_name ? `GPU: ${d.gpu_name} — VRAM ${fmtGB(d.gpu_vram_used)} / ${fmtGB(d.gpu_vram_total)}` : 'GPU: none detected';
    },

    server: (d) => {
        UI.server.dotServer.className = 'dot ok';
        UI.server.valServer.textContent = 'running';
        UI.server.dotLlama.className = `dot ${d.ready ? 'ok' : 'missing'}`;
        UI.server.valLlama.textContent = d.ready ? (d.llama_addr || `port ${d.llama_port}`) : 'not running';
        const activePath = Store.model.data.activePath || '';
        const activeVariant = Object.keys(MODEL_LABELS).find(k => activePath.includes(k));
        const modelLabel = activeVariant ? MODEL_LABELS[activeVariant] : (activePath ? activePath.split('/').pop() : 'none');
        UI.server.dotModel.className = `dot ${activePath ? 'ok' : 'missing'}`;
        UI.server.valModel.textContent = modelLabel;

        const isRemote = d.remote_mode === true;
        UI.server.btnStart.style.display     = (!d.ready && !isRemote) ? '' : 'none';
        UI.server.btnStop.style.display      = (d.ready && !isRemote)  ? '' : 'none';
        UI.server.rowStop.style.display      = isRemote ? 'none' : '';
        UI.server.remoteNotice.style.display = isRemote ? '' : 'none';

        // Local-only cards
        UI.bin.card.style.display            = isRemote ? 'none' : '';
        UI.model.card.style.display          = isRemote ? 'none' : '';
        UI.config.sectionModelUrls.style.display = isRemote ? 'none' : '';

        // Binary status (only meaningful in local mode)
        UI.bin.dotBin.className   = `dot ${d.bin_present ? 'ok' : 'missing'}`;
        UI.bin.valBin.textContent = d.bin_present ? `llama-${d.bin_version}` : 'not found';
        UI.bin.labelBin.textContent = d.bin_present ? 'Current Release' : 'Binary missing';
    },

    config: (d) => {
        // GemmaLink Server
        UI.config.httpHost.value  = d.http_host || '';
        UI.config.httpPort.value  = d.http_port || '';
        UI.config.httpsPort.value = d.https_port || '';
        UI.config.uploadDir.value = d.upload_dir || '';
        UI.sys.refreshInput.value = d.sysinfo_refresh_seconds || 5;

        // Llama mode
        const loc = d.llama_local ?? {};
        const rem = d.llama_remote ?? {};
        setLlamaMode(rem.enabled ? 'remote' : 'local');

        // Llama Local
        UI.config.localEndpoint.value   = loc.endpoint || '';
        UI.config.llamaBin.value        = loc.bin_path || loc.llama_bin || '';
        UI.config.llamaBinVersion.value = loc.llama_bin_version || '';
        UI.config.modelPath.value       = loc.model_path || '';
        UI.config.mmprojPath.value      = loc.mmproj_path || '';
        UI.config.contextSize.value     = loc.ctx_size || loc.context_size || 2048;

        // Llama Remote
        UI.config.remoteEndpoint.value = rem.endpoint || '';

        // Download URLs
        const urls = d.download_urls ?? d.model_urls ?? {};
        UI.config.urlE2b.value = urls.e2b || DEFAULT_URLS.e2b;
        UI.config.urlE4b.value = urls.e4b || DEFAULT_URLS.e4b;

        // Applica il refresh sysinfo basato sulla config caricata
        Actions.applySysinfoRefresh(d.sysinfo_refresh_seconds || 5);
    },

    bin: (d) => {
        const current = Store.server.data.bin_version || '';
        if (d.releases.length > 0) {
            UI.bin.selector.innerHTML = d.releases.map(r => {
                const isCurrent = current && r.tag === current;
                const label = (isCurrent ? '\u2713 ' : '') + r.name + ' (' + r.tag + ')';
                return `<option value="${r.tag}"${isCurrent ? ' selected' : ''}>${label}</option>`;
            }).join('');
            UI.bin.btnDl.style.display = '';
        } else {
            UI.bin.selector.innerHTML = '<option value="">No releases available</option>';
            UI.bin.btnDl.style.display = 'none';
        }
    },

    model: (d) => {
        // RAM/VRAM-aware selector: show only variants the machine can run.
        // Prefer GPU VRAM if available, otherwise total RAM.
        const sys = Store.sys.data;
        const effectiveMem = (sys.gpu_vram_total > 0)
            ? Math.max(sys.ram_total, sys.gpu_vram_total)
            : (sys.ram_total || Infinity);

        const currentVal = UI.model.selector.value;
        UI.model.selector.innerHTML = '';
        for (const [v, label] of Object.entries(MODEL_LABELS)) {
            if (effectiveMem < MODEL_MIN_RAM[v]) continue;
            const opt = document.createElement('option');
            opt.value = v;
            const downloaded = !!(d.variants[v] || {}).model;
            opt.textContent = (downloaded ? '\u2713 ' : '') + label;
            if (v === currentVal) opt.selected = true;
            UI.model.selector.appendChild(opt);
        }
        // Fallback: se la selezione precedente non è più disponibile, usa la prima
        if (!UI.model.selector.value && UI.model.selector.options.length > 0)
            UI.model.selector.options[0].selected = true;

        const variant = UI.model.selector.value;
        const exists  = !!(d.variants[variant] || {}).model;
        const isActive = d.activePath && d.activePath.includes(variant);

        UI.model.btnDl.style.display  = exists ? 'none' : '';
        UI.model.btnUse.style.display = (exists && !isActive) ? '' : 'none';

        UI.model.activeBadge.style.display = isActive ? '' : 'none';
        if (isActive) UI.model.activeBadge.textContent = `\u2713 Current active model: ${variant.toUpperCase()}`;

        UI.model.visionToggle.checked = !!d.vision;
    },

    qr: (d) => {
        UI.qr.url.textContent = d.url;
        UI.qr.img.src = d.img + '?v=' + encodeURIComponent(d.url);
    },

    log: () => {
        const e = Store.log.data[Store.log.data.length - 1];
        if (!e) return;
        const div = document.createElement('div');
        div.className = `log-entry ${e.type}`;
        div.textContent = `[${new Date().toLocaleTimeString()}] ${e.msg}`;
        UI.log.box.appendChild(div);
        UI.log.box.scrollTop = UI.log.box.scrollHeight;
    },

    dl: (d) => {
        const b = d.type === 'llama' ? UI.bin : UI.model;
        b.progWrap.classList.toggle('visible', d.visible);
        b.progFill.style.width = `${d.pct}%`;
        b.progLabel.textContent = d.label;
    }
};

// --- 4. ACTIONS ---
const Actions = {
    dispatch: () => {
        for (const k in Store) { if (Store[k].dirty) { Renderer[k](Store[k].data); Store[k].dirty = false; } }
    },
    sync: (sect, data) => { Store[sect].data = data; Store[sect].dirty = true; if (sect === 'model') Store.server.dirty = true; if (sect === 'server') Store.bin.dirty = true; Actions.dispatch(); },
    addLog: (msg, type = '') => { Store.log.data.push({ msg, type }); Store.log.dirty = true; Actions.dispatch(); },

    applySysinfoRefresh: (seconds) => {
        if (pollSysInterval) clearInterval(pollSysInterval);
        const ms = seconds * 1000;
        pollSysInterval = setInterval(async () => {
            try {
                const sys = await fetch('/api/sysinfo').then(r => r.json());
                Actions.sync('sys', sys);
            } catch (e) { }
        }, ms);
    },

    loadLlamaReleases: async () => {
        try {
            // Recupera la lista dal tuo backend Go
            Actions.addLog(`Loading llama-server releases`, 'ok');

            const res = await fetch('/api/llama/releases').then(r => r.json());

            // Sincronizza lo stato: questo imposta dirty = true e lancia il Redraw
            Actions.sync('bin', { releases: res });

            if (res && res.length > 0) {
                Actions.addLog(`Loaded ${res.length} llama-server releases`, 'ok');
            }

        } catch (e) {
            Actions.addLog("Failed to fetch llama-server releases from GitHub", "err");
        }
    },

    testConn: async (mode) => {
        const endpoint = mode === 'local'
            ? UI.config.localEndpoint.value.trim()
            : UI.config.remoteEndpoint.value.trim();
        const el = mode === 'local' ? UI.config.connStatusLocal : UI.config.connStatusRemote;
        el.innerHTML = '';
        el.style.cssText = 'font-size:0.8rem;color:#888';
        el.textContent = 'Testing…';
        try {
            const d = await fetch(`/api/llama/test?endpoint=${encodeURIComponent(endpoint)}`).then(r => r.json());
            if (d.reachable) {
                el.innerHTML = `<span class="badge badge-ok">✓ reachable${d.model_name ? ' — ' + d.model_name : ''}</span>`;
                Actions.addLog(`llama-server reachable at ${endpoint}${d.model_name ? ' · model: ' + d.model_name : ''}`, 'ok');
            } else {
                el.innerHTML = `<span class="badge badge-err">✗ not reachable</span>`;
                Actions.addLog(`llama-server not reachable at ${endpoint}`, 'warn');
            }
        } catch (e) {
            el.innerHTML = `<span class="badge badge-err">✗ error</span>`;
            Actions.addLog('Test failed: ' + e.message, 'err');
        }
    },
    saveConfig: async () => {
        const payload = {
            http_host:  UI.config.httpHost.value.trim(),
            http_port:  UI.config.httpPort.value.trim(),
            https_port: UI.config.httpsPort.value.trim(),
            upload_dir: UI.config.uploadDir.value.trim(),
            sysinfo_refresh_seconds: parseInt(UI.sys.refreshInput.value) || 5,
            llama_remote: {
                enabled:  UI.config.radioRemote.checked,
                endpoint: UI.config.remoteEndpoint.value.trim()
            },
            llama_local: {
                enabled:           !UI.config.radioRemote.checked,
                endpoint:          UI.config.localEndpoint.value.trim(),
                llama_bin:         UI.config.llamaBin.value.trim(),
                llama_bin_version: UI.config.llamaBinVersion.value.trim(),
                model_path:        UI.config.modelPath.value.trim(),
                mmproj_path:       UI.config.mmprojPath.value.trim(),
                context_size:      parseInt(UI.config.contextSize.value.trim()) || 0
            },
            model_urls: {
                e2b: UI.config.urlE2b.value.trim(),
                e4b: UI.config.urlE4b.value.trim()
            }
        };

        UI.config.cfgStatus.textContent = 'Saving…';
        UI.config.cfgStatus.style.color = '#888';
        Actions.addLog('Saving configuration…');
        try {
            const res = await fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            }).then(r => r.json());

            if (res.ok) {
                UI.config.cfgStatus.textContent = 'Saved ✓';
                UI.config.cfgStatus.style.color = '#44ff88';
                UI.config.restartWarn.style.display = res.ports_changed ? 'block' : 'none';
                Actions.addLog('Configuration saved' + (res.ports_changed ? ' — restart required' : ''), res.ports_changed ? 'warn' : 'ok');
                Actions.sync('config', payload);
                // Torna al Dashboard
                UI.tabs.btns.forEach(b => b.classList.toggle('active', b.dataset.tab === 'dashboard'));
                document.querySelectorAll('.tab-panel').forEach(p => p.classList.remove('active'));
                UI.tabs.dashboard.classList.add('active');
            } else {
                throw new Error(res.error || 'Save failed');
            }
        } catch (e) {
            UI.config.cfgStatus.textContent = 'Error';
            UI.config.cfgStatus.style.color = '#ff4444';
            Actions.addLog(`Error saving config: ${e.message}`, 'err');
        }
        setTimeout(() => { UI.config.cfgStatus.textContent = ''; UI.config.cfgStatus.style.color = ''; }, 4000);
    }
};

// --- 5. UTILITIES ---

// Returns true on success, false on cancel/error.
// Streams SSE progress events from the server and updates the progress bar.
async function streamDownload(url, progFill, progLabel, signal) {
    try {
        const resp = await fetch(url, { signal });
        const reader = resp.body.getReader();
        const dec = new TextDecoder();
        let buf = '';
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            buf += dec.decode(value, { stream: true });
            const lines = buf.split('\n'); buf = lines.pop();
            for (const line of lines) {
                if (!line.startsWith('data: ')) continue;
                try {
                    const e = JSON.parse(line.slice(6));
                    if (e.pct !== undefined) {
                        progFill.style.width = e.pct + '%';
                        progLabel.textContent = `${e.pct}% — ${fmtGB(e.bytes)} / ${fmtGB(e.total)}`;
                    }
                    if (e.done)  { progFill.style.background = '#44ff88'; progLabel.textContent = 'Done!'; return true; }
                    if (e.info)  { Actions.addLog(e.info, 'ok'); }
                    if (e.error) { progFill.style.background = '#ff4444'; progLabel.textContent = 'Error: ' + e.error; Actions.addLog('Download error: ' + e.error, 'err'); return false; }
                } catch { /* skip malformed line */ }
            }
        }
        return true;
    } catch (e) {
        if (e.name === 'AbortError') return false;
        progLabel.textContent = 'Failed: ' + e.message;
        Actions.addLog('Download failed: ' + e.message, 'err');
        return false;
    }
}

// --- 6. INITIALIZATION ---
function setLlamaMode(mode) {
    const isRemote = mode === 'remote';
    UI.config.radioLocal.checked  = !isRemote;
    UI.config.radioRemote.checked = isRemote;
    UI.config.lblLocal.classList.toggle('active', !isRemote);
    UI.config.lblRemote.classList.toggle('active', isRemote);
    UI.config.panelLocal.style.display  = isRemote ? 'none' : '';
    UI.config.panelRemote.style.display = isRemote ? '' : 'none';
}

function init() {
    // 1. Initial Data Fetch
    Actions.addLog('Admin dashboard loading...', 'ok');
    fetch('/api/config').then(r => r.json()).then(d => Actions.sync('config', d));
    fetch('/api/qr-url').then(r => r.json()).then(d => Actions.sync('qr', { url: d.url, img: '/api/qr' }));
    Actions.loadLlamaReleases();

    // 2. Tab logic
    UI.tabs.btns.forEach(b => {
        b.addEventListener('click', () => {
            UI.tabs.btns.forEach(x => x.classList.remove('active'));
            document.querySelectorAll('.tab-panel').forEach(x => x.classList.remove('active'));
            b.classList.add('active');
            UI.tabs[b.dataset.tab].classList.add('active');
        });
    });

    // 3. Status Polling (Status Llama/Model)
    setInterval(async () => {
        try {
            const status = await fetch('/api/status').then(r => r.json());
            Actions.sync('server', status);
            Actions.sync('model', {
                variants: status.models_present || {},
                activePath: status.model_path,
                vision: status.vision_enabled
            });
        } catch (e) { }
    }, 4000);

    // 4. Input Handlers
    UI.model.selector.onchange = () => Renderer.model(Store.model.data);
    UI.bin.btnRefresh.addEventListener('click', () => {
        UI.bin.selector.innerHTML = '<option value="">Loading releases…</option>';
        Actions.loadLlamaReleases();
    });
    UI.bin.selector.onchange   = () => Renderer.server(Store.server.data);

    // ── llama-server Binary ───────────────────────────────────
    let llamaDlController = null;

    UI.bin.btnDl.addEventListener('click', async () => {
        const tag = UI.bin.selector.value;
        if (!tag) return;

        const releases = Store.bin.data.releases || [];
        const entry = releases.find(r => r.tag === tag);
        const assetURL = entry ? entry.url : '';

        await fetch(`/api/llama/select?tag=${encodeURIComponent(tag)}&url=${encodeURIComponent(assetURL)}`, { method: 'POST' });

        llamaDlController = new AbortController();
        UI.bin.btnDl.style.display     = 'none';
        UI.bin.btnCancel.style.display = '';
        UI.bin.progWrap.classList.add('visible');
        UI.bin.progFill.style.width      = '0%';
        UI.bin.progFill.style.background = '';
        UI.bin.progLabel.textContent     = 'Starting…';
        Actions.addLog(`Downloading llama-server ${tag}…`, 'ok');

        const ok = await streamDownload('/api/download?target=llama', UI.bin.progFill, UI.bin.progLabel, llamaDlController.signal);
        llamaDlController = null;
        UI.bin.btnCancel.style.display = 'none';
        if (ok) Actions.addLog(`llama-server ${tag} downloaded.`, 'ok');

        const status = await fetch('/api/status').then(r => r.json());
        Actions.sync('server', status);
    });

    UI.bin.btnCancel.addEventListener('click', () => {
        if (!llamaDlController) return;
        llamaDlController.abort();
        llamaDlController = null;
        UI.bin.progWrap.classList.remove('visible');
        UI.bin.btnCancel.style.display = 'none';
        Actions.addLog('llama-server download cancelled', 'warn');
    });

    UI.bin.btnUse.addEventListener('click', async () => {
        const tag = UI.bin.selector.value;
        if (!tag) return;
        const releases = Store.bin.data.releases || [];
        const entry = releases.find(r => r.tag === tag);
        const assetURL = entry ? entry.url : '';
        try {
            const d = await fetch(`/api/llama/select?tag=${encodeURIComponent(tag)}&url=${encodeURIComponent(assetURL)}`, { method: 'POST' }).then(r => r.json());
            if (d.ok) {
                Actions.addLog(`llama-server active version set to ${tag}`, 'ok');
                const status = await fetch('/api/status').then(r => r.json());
                Actions.sync('server', status);
            }
        } catch (e) { Actions.addLog('Select failed: ' + e.message, 'err'); }
    });

    UI.config.btnSave.onclick = () => Actions.saveConfig();

    // ── Gemma Model ───────────────────────────────────────────
    let modelDlController  = null;
    let mmprojDlController = null;

    UI.model.btnDl.addEventListener('click', async () => {
        const variant = UI.model.selector.value;
        if (!variant) return;

        modelDlController = new AbortController();
        UI.model.btnDl.style.display     = 'none';
        UI.model.btnCancel.style.display = '';
        UI.model.progWrap.classList.add('visible');
        UI.model.progFill.style.width      = '0%';
        UI.model.progFill.style.background = '';
        UI.model.progLabel.textContent     = 'Starting…';
        Actions.addLog(`Downloading model for ${variant.toUpperCase()}…`, 'ok');

        const ok = await streamDownload(`/api/download?target=model-${variant}`, UI.model.progFill, UI.model.progLabel, modelDlController.signal);
        modelDlController = null;
        UI.model.btnCancel.style.display = 'none';
        if (ok) Actions.addLog(`Model downloaded for ${variant.toUpperCase()}.`, 'ok');

        const status = await fetch('/api/status').then(r => r.json());
        Actions.sync('server', status);
        Actions.sync('model', { variants: status.models_present || {}, activePath: status.model_path, vision: status.vision_enabled });
    });

    UI.model.btnCancel.addEventListener('click', () => {
        if (!modelDlController) return;
        modelDlController.abort();
        modelDlController = null;
        UI.model.progWrap.classList.remove('visible');
        UI.model.btnCancel.style.display = 'none';
        Actions.addLog('Model download cancelled', 'warn');
    });

    UI.model.btnUse.addEventListener('click', async () => {
        const variant = UI.model.selector.value;
        if (!variant) return;
        try {
            const d = await fetch(`/api/model/select?variant=${variant}`, { method: 'POST' }).then(r => r.json());
            if (d.ok) {
                Actions.addLog(`Model set to ${variant.toUpperCase()}: ${d.model_path}`, 'ok');
                const status = await fetch('/api/status').then(r => r.json());
                Actions.sync('server', status);
                Actions.sync('model', { variants: status.models_present || {}, activePath: status.model_path, vision: status.vision_enabled });
            }
        } catch (e) { Actions.addLog('Select failed: ' + e.message, 'err'); }
    });

    UI.model.visionToggle.addEventListener('change', async function () {
        const enabled = this.checked;
        const variant = UI.model.selector.value;

        if (enabled) {
            const modelsPresent = Store.model.data.variants || {};
            const hasmmproj = !!(modelsPresent[variant] || {}).mmproj;
            if (!hasmmproj) {
                // mmproj non presente — scaricalo prima
                mmprojDlController = new AbortController();
                UI.model.progWrap.classList.add('visible');
                UI.model.progFill.style.width      = '0%';
                UI.model.progFill.style.background = '';
                UI.model.progLabel.textContent     = 'Downloading mmproj…';
                Actions.addLog(`Downloading mmproj for ${variant.toUpperCase()}…`, 'ok');

                const ok = await streamDownload(`/api/download?target=mmproj-${variant}`, UI.model.progFill, UI.model.progLabel, mmprojDlController.signal);
                mmprojDlController = null;
                if (!ok) {
                    this.checked = false;
                    const status = await fetch('/api/status').then(r => r.json());
                    Actions.sync('model', { variants: status.models_present || {}, activePath: status.model_path, vision: status.vision_enabled });
                    return;
                }
                Actions.addLog(`mmproj downloaded for ${variant.toUpperCase()}.`, 'ok');
            }
            await fetch(`/api/vision/toggle?enabled=true&variant=${encodeURIComponent(variant)}`, { method: 'POST' });
            Actions.addLog('Vision enabled — restart llama-server to apply', 'ok');
        } else {
            await fetch('/api/vision/toggle?enabled=false', { method: 'POST' });
            Actions.addLog('Vision disabled', 'ok');
        }

        const status = await fetch('/api/status').then(r => r.json());
        Actions.sync('server', status);
        Actions.sync('model', { variants: status.models_present || {}, activePath: status.model_path, vision: status.vision_enabled });
    });

    // ── Start / Stop llama-server ─────────────────────────────
    UI.server.btnStart.addEventListener('click', async () => {
        UI.server.btnStart.disabled    = true;
        UI.server.btnStart.textContent = '⏳ Starting…';
        Actions.addLog('Starting llama-server — loading model, please wait…', 'ok');
        try {
            const d = await fetch('/api/llama/start', { method: 'POST' }).then(r => r.json());
            if (d.error) Actions.addLog('Start failed: ' + d.error, 'err');
            else         Actions.addLog('llama-server is ready.', 'ok');
        } catch (e) { Actions.addLog('Start failed: ' + e.message, 'err'); }
        UI.server.btnStart.disabled    = false;
        UI.server.btnStart.textContent = '▶ Start llama-server';
        const status = await fetch('/api/status').then(r => r.json());
        Actions.sync('server', status);
        Actions.sync('model', { variants: status.models_present || {}, activePath: status.model_path, vision: status.vision_enabled });
    });

    UI.server.btnStop.addEventListener('click', async () => {
        UI.server.btnStop.disabled    = true;
        UI.server.btnStop.textContent = '⏳ Stopping…';
        Actions.addLog('Stopping llama-server…', 'warn');
        try {
            await fetch('/api/llama/stop', { method: 'POST' });
            Actions.addLog('llama-server stopped.', 'warn');
        } catch (e) { Actions.addLog('Stop failed: ' + e.message, 'err'); }
        UI.server.btnStop.disabled    = false;
        UI.server.btnStop.textContent = '■ Stop llama-server';
        const status = await fetch('/api/status').then(r => r.json());
        Actions.sync('server', status);
        Actions.sync('model', { variants: status.models_present || {}, activePath: status.model_path, vision: status.vision_enabled });
    });
    [UI.config.radioLocal, UI.config.radioRemote].forEach(radio => {
        radio.addEventListener('change', (e) => setLlamaMode(e.target.value));
    });

    UI.config.btnTestLocal.onclick  = () => Actions.testConn('local');
    UI.config.btnTestRemote.onclick = () => Actions.testConn('remote');

    UI.config.btnResetE2b.onclick = () => { UI.config.urlE2b.value = DEFAULT_URLS.e2b; };
    UI.config.btnResetE4b.onclick = () => { UI.config.urlE4b.value = DEFAULT_URLS.e4b; };

    // ── SSL Certificate ───────────────────────────────────────
    UI.config.btnRegen.addEventListener('click', async () => {
        UI.config.certStatus.textContent = 'Generating…';
        UI.config.certStatus.style.color = '#888';
        Actions.addLog('Regenerating SSL certificate…');
        try {
            const d = await fetch('/api/cert/regenerate', { method: 'POST' }).then(r => r.json());
            if (d.ok) {
                UI.config.certStatus.textContent = 'Generated ✓';
                UI.config.certStatus.style.color = '#44ff88';
                Actions.addLog(`Certificate generated — SANs: ${d.hostnames.join(', ')} | IPs: ${d.ips.join(', ')}`, 'ok');
            } else {
                throw new Error(d.error || 'Generation failed');
            }
        } catch (e) {
            UI.config.certStatus.textContent = 'Failed';
            UI.config.certStatus.style.color = '#ff4444';
            Actions.addLog('Cert error: ' + e.message, 'err');
        }
        setTimeout(() => { UI.config.certStatus.textContent = ''; UI.config.certStatus.style.color = ''; }, 5000);
    });

    // ── Sysinfo refresh — applica subito, salva dopo 800ms ────
    let sysinfoDebounce = null;
    UI.sys.refreshInput.addEventListener('input', () => {
        const v = parseInt(UI.sys.refreshInput.value, 10);
        if (!v || v < 1) return;
        Actions.applySysinfoRefresh(v);
        clearTimeout(sysinfoDebounce);
        sysinfoDebounce = setTimeout(async () => {
            try {
                await fetch('/api/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ sysinfo_refresh_seconds: v })
                });
            } catch (e) { Actions.addLog('Failed to save refresh interval: ' + e.message, 'err'); }
        }, 800);
    });

    Actions.addLog("System Initialized", "ok");
}

function fmtGB(b) { return (b / 1073741824).toFixed(1) + ' GB'; }
document.addEventListener('DOMContentLoaded', init);