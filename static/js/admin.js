// GemmaLink Admin Dashboard — JS
// ─────────────────────────────────────────────────────────────────

// ─── Model URL defaults ────────────────────────────────────────
const DEFAULT_MODEL_URLS = {
  e2b:        'https://huggingface.co/lmstudio-community/gemma-4-E2B-it-GGUF/resolve/main/gemma-4-E2B-it-Q4_K_M.gguf',
  e4b:        'https://huggingface.co/lmstudio-community/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-E4B-it-Q4_K_M.gguf',
  //e31b:       'https://huggingface.co/lmstudio-community/gemma-4-E31B-it-GGUF/resolve/main/gemma-4-E31B-it-Q4_K_M.gguf',
  mmproj_e2b: 'https://huggingface.co/lmstudio-community/gemma-4-E2B-it-GGUF/resolve/main/mmproj-gemma-4-E2B-it-BF16.gguf',
  mmproj_e4b: 'https://huggingface.co/lmstudio-community/gemma-4-E4B-it-GGUF/resolve/main/mmproj-gemma-4-E4B-it-BF16.gguf',
  //mmproj_e31b:'https://huggingface.co/lmstudio-community/gemma-4-E31B-it-GGUF/resolve/main/mmproj-gemma-4-E31B-it-BF16.gguf',
};

// ─── Tab switching ─────────────────────────────────────────────
document.querySelectorAll('.tab-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.tab-panel').forEach(p => p.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById('tab-' + btn.dataset.tab).classList.add('active');
  });
});

// ─── Logging ───────────────────────────────────────────────────
function log(msg, type = '') {
  const box = document.getElementById('log-box');
  const line = document.createElement('div');
  if (type) line.className = 'log-' + type;
  line.textContent = `[${new Date().toLocaleTimeString()}] ${msg}`;
  box.appendChild(line);
  box.scrollTop = box.scrollHeight;
}

function setBar(id, pct) {
  const el = document.getElementById(id);
  el.style.width = pct + '%';
  el.className = 'bar-fill' + (pct > 89 ? ' crit' : pct > 69 ? ' warn' : '');
}
function setDot(id, state) { document.getElementById(id).className = 'dot ' + state; }
function fmtGB(b) { return (b / 1073741824).toFixed(1) + ' GB'; }

// ─── Remote mode UI ────────────────────────────────────────────
let remoteMode = false;
function applyRemoteMode(remote) {
  remoteMode = remote;
  document.getElementById('card-binary').style.display = remote ? 'none' : '';
  document.getElementById('card-model').style.display = remote ? 'none' : '';
  document.getElementById('row-stop').style.display = remote ? 'none' : '';
  document.getElementById('remote-notice').style.display = remote ? 'block' : 'none';
  document.getElementById('section-model-urls').style.display = remote ? 'none' : '';
}

// ─── Poll /api/sysinfo ─────────────────────────────────────────
let lastSysInfo = null;
async function pollSysinfo() {
  try {
    const d = await fetch('/api/sysinfo').then(r => r.json());
    lastSysInfo = d;
    const ramPct = Math.round(d.ram_used / d.ram_total * 100);
    const diskPct = Math.round(d.disk_used / d.disk_total * 100);
    document.getElementById('ram-label').textContent = `${fmtGB(d.ram_used)} / ${fmtGB(d.ram_total)} (${ramPct}%)`;
    document.getElementById('cpu-label').textContent = `${d.cpu_pct.toFixed(1)}%`;
    document.getElementById('disk-label').textContent = `${fmtGB(d.disk_used)} / ${fmtGB(d.disk_total)} (${diskPct}%)`;
    setBar('ram-bar', ramPct);
    setBar('cpu-bar', Math.round(d.cpu_pct));
    setBar('disk-bar', diskPct);
    const gpuEl = document.getElementById('gpu-info');
    if (d.gpu_name) {
      gpuEl.textContent = `GPU: ${d.gpu_name} — VRAM ${fmtGB(d.gpu_vram_used)} / ${fmtGB(d.gpu_vram_total)}`;
      gpuEl.style.color = '#aaa';
    } else {
      gpuEl.textContent = 'GPU: not detected (CPU inference mode)';
    }
  } catch (e) { log('sysinfo error: ' + e.message, 'err'); }
}

// ─── Poll /api/status ──────────────────────────────────────────
async function pollStatus() {
  try {
    const d = await fetch('/api/status').then(r => r.json());
    setDot('dot-server', 'ok');
    document.getElementById('val-server').textContent = 'running';

    // Update remoteMode flag only — don't touch config UI visibility
    // (that is controlled by the radio buttons and loadConfig)
    remoteMode = d.remote_mode === true;

    if (d.ready) {
      setDot('dot-llama', 'ok');
      document.getElementById('val-llama').textContent = d.llama_addr || ('port ' + d.llama_port);
      document.getElementById('btn-start-llama').style.display = 'none';
      document.getElementById('btn-stop-llama').style.display  = '';
    } else {
      setDot('dot-llama', 'missing');
      document.getElementById('val-llama').textContent = 'not running';
      document.getElementById('btn-start-llama').style.display = remoteMode ? 'none' : '';
      document.getElementById('btn-stop-llama').style.display  = 'none';
    }

    if (!remoteMode) {
      if (d.bin_present) {
        setDot('dot-bin', 'ok');
        document.getElementById('label-bin').textContent = 'Binary found';
        document.getElementById('val-bin').textContent = d.bin_path;
      } else {
        setDot('dot-bin', 'missing');
        document.getElementById('label-bin').textContent = 'Binary missing';
        document.getElementById('val-bin').textContent = d.bin_path;
      }
      updateLlamaSelectorUI(d.bin_present, d.bin_version);
    }
    if (d.model_present) {
      setDot('dot-model', 'ok');
      document.getElementById('val-model').textContent = remoteMode ? d.model_path : fmtGB(d.model_size);
    } else {
      setDot('dot-model', 'missing');
      document.getElementById('val-model').textContent = 'missing';
    }

    // Update per-model selector UI
    if (d.models_present) {
      lastModelsPresent = d.models_present;
      updateModelSelectorUI(d.models_present, d.model_path, d.vision_enabled);
    }
  } catch (e) {
    setDot('dot-server', 'missing');
    log('status error: ' + e.message, 'err');
  }
}

// ─── llama-server release selector ────────────────────────────
let llamaReleases = []; // [{tag, name, url}]
let llamaDownloadController = null;

async function loadLlamaReleases() {
  const sel = document.getElementById('llama-selector');
  try {
    llamaReleases = await fetch('/api/llama/releases').then(r => r.json());
    sel.innerHTML = '';
    if (!llamaReleases || llamaReleases.length === 0) {
      sel.innerHTML = '<option value="">No releases found</option>';
      return;
    }
    for (const r of llamaReleases) {
      const opt = document.createElement('option');
      opt.value = r.tag;
      const label = `${r.tag} — ${r.name}`;
      opt.dataset.label = label;
      opt.textContent = label;
      sel.appendChild(opt);
    }
    log(`Loaded ${llamaReleases.length} llama.cpp releases`, 'ok');
  } catch (e) {
    sel.innerHTML = '<option value="">GitHub unreachable</option>';
    log('Could not load llama releases: ' + e.message, 'warn');
  }
}

function updateLlamaSelectorUI(binPresent, activeTag) {
  const sel = document.getElementById('llama-selector');

  // Add ✓ checkmark to the option matching the active (downloaded) tag
  for (const opt of sel.options) {
    const lbl = opt.dataset.label || opt.textContent.replace(/^✓ /, '');
    opt.dataset.label = lbl;
    opt.textContent = (binPresent && opt.value === activeTag) ? '✓ ' + lbl : lbl;
  }

  const selectedTag = sel.value;
  const downloading = llamaDownloadController !== null;

  const isActive = binPresent && selectedTag === activeTag;
  const isPresent = binPresent;

  document.getElementById('dot-bin').className = 'dot ' + (isPresent ? 'ok' : 'missing');
  document.getElementById('val-bin').textContent = isPresent
    ? (activeTag ? `v${activeTag}` : 'present')
    : 'not downloaded';

  document.getElementById('btn-dl-llama').style.display       = (!downloading) ? '' : 'none';
  document.getElementById('btn-cancel-dl-llama').style.display = downloading ? '' : 'none';
  document.getElementById('btn-use-llama').style.display       = (isPresent && !isActive && !downloading) ? '' : 'none';

  const badge = document.getElementById('llama-active-badge');
  badge.style.display = isActive ? '' : 'none';
  badge.textContent   = isActive ? `✓ Active — ${activeTag}` : '';
}

document.getElementById('llama-selector').addEventListener('change', pollStatus);

document.getElementById('btn-dl-llama').addEventListener('click', async () => {
  const tag = document.getElementById('llama-selector').value;
  if (!tag) return;
  const controller = new AbortController();
  llamaDownloadController = controller;
  pollStatus();

  const fill  = document.getElementById('prog-llama-fill');
  const label = document.getElementById('prog-llama-label');
  const wrap  = document.getElementById('prog-llama-wrap');
  wrap.classList.add('visible'); fill.style.width = '0%'; fill.style.background = '';

  // Tell server which tag to download before starting
  await fetch(`/api/llama/select?tag=${encodeURIComponent(tag)}`, { method: 'POST' });
  log(`Downloading llama-server ${tag}…`, 'ok');

  const ok = await streamDownload('/api/download?target=llama', fill, label, controller.signal);
  llamaDownloadController = null;
  if (ok) {
    log(`llama-server ${tag} downloaded.`, 'ok');
  }
  pollStatus();
});

document.getElementById('btn-cancel-dl-llama').addEventListener('click', () => {
  if (llamaDownloadController) {
    llamaDownloadController.abort();
    llamaDownloadController = null;
    log('llama-server download cancelled', 'warn');
    document.getElementById('prog-llama-wrap').classList.remove('visible');
    pollStatus();
  }
});

document.getElementById('btn-use-llama').addEventListener('click', async () => {
  const tag = document.getElementById('llama-selector').value;
  if (!tag) return;
  try {
    const d = await fetch(`/api/llama/select?tag=${encodeURIComponent(tag)}`, { method: 'POST' }).then(r => r.json());
    if (d.ok) { log(`llama-server active version set to ${tag}`, 'ok'); pollStatus(); }
  } catch (e) { log('Select failed: ' + e.message, 'err'); }
});

// ─── Model selector ────────────────────────────────────────────
const MODEL_PATHS = {
  e2b:  { model: 'models/gemma-4-e2b.gguf',   mmproj: 'models/mmproj-gemma-4-e2b.gguf' },
  e4b:  { model: 'models/gemma-4-e4b.gguf',   mmproj: 'models/mmproj-gemma-4-e4b.gguf' },
  e31b: { model: 'models/gemma-4-e31b.gguf',  mmproj: 'models/mmproj-gemma-4-e31b.gguf' },
};
const MODEL_LABELS = {
  e2b:  'Gemma 4 E2B (2B params)',
  e4b:  'Gemma 4 E4B (4B params)',
  e31b: 'Gemma 4 E31B (31B params)',
};
// Minimum total RAM (bytes) required for each model variant (Q4_K_M + context overhead).
const MODEL_MIN_RAM = {
  e2b:  4  * 1073741824,  //  4 GB
  e4b:  6  * 1073741824,  //  6 GB
  e31b: 24 * 1073741824,  // 24 GB
};

let activeDownloadController = null;  // AbortController for model download cancellation
let mmprojDownloadController = null;  // AbortController for mmproj download (vision toggle)
let lastModelsPresent = null;          // cached models_present from last status poll

function updateModelSelectorUI(modelsPresent, activeModelPath, visionEnabled) {
  const sel = document.getElementById('model-selector');

  // Determine effective memory: prefer GPU VRAM if available, otherwise total RAM.
  const effectiveMem = (lastSysInfo && lastSysInfo.gpu_vram_total > 0)
    ? Math.max(lastSysInfo.ram_total, lastSysInfo.gpu_vram_total)
    : (lastSysInfo ? lastSysInfo.ram_total : Infinity);

  // Show only variants whose memory requirement is met; keep current selection if still valid.
  const currentVal = sel.value;
  sel.innerHTML = '';
  for (const [v, label] of Object.entries(MODEL_LABELS)) {
    if (effectiveMem < MODEL_MIN_RAM[v]) continue;  // not enough memory — skip
    const opt = document.createElement('option');
    opt.value = v;
    const ok = !!(modelsPresent[v] || {}).model;
    opt.dataset.label = label;
    opt.textContent = (ok ? '✓ ' : '') + label;
    if (v === currentVal) opt.selected = true;
    sel.appendChild(opt);
  }
  // If previous selection was removed, fall back to first available.
  if (!sel.value && sel.options.length > 0) sel.options[0].selected = true;

  const variant = sel.value;
  const p = modelsPresent[variant] || {};
  const modelOk = !!p.model;
  const paths = MODEL_PATHS[variant];

  const downloading = activeDownloadController !== null;
  document.getElementById('btn-dl-model').style.display  = (!modelOk && !downloading) ? '' : 'none';
  document.getElementById('btn-cancel-dl').style.display = downloading ? '' : 'none';
  document.getElementById('btn-use-model').style.display = (modelOk && !downloading) ? '' : 'none';

  // Active badge
  const badge = document.getElementById('model-active-badge');
  const isActive = activeModelPath === paths.model;
  badge.style.display = (modelOk && isActive) ? '' : 'none';
  badge.textContent = isActive ? '✓ Active model' : '';

  // Vision toggle
  document.getElementById('vision-toggle').checked = !!visionEnabled;
}

document.getElementById('model-selector').addEventListener('change', pollStatus);

document.getElementById('btn-dl-model').addEventListener('click', async () => {
  const variant = document.getElementById('model-selector').value;
  await downloadModel(variant);
});

document.getElementById('btn-cancel-dl').addEventListener('click', () => {
  if (activeDownloadController) {
    activeDownloadController.abort();
    activeDownloadController = null;
    log('Download cancelled', 'warn');
    document.getElementById('prog-model-wrap').classList.remove('visible');
    pollStatus();
  }
});

document.getElementById('btn-use-model').addEventListener('click', async () => {
  const variant = document.getElementById('model-selector').value;
  try {
    const d = await fetch(`/api/model/select?variant=${variant}`, { method: 'POST' }).then(r => r.json());
    if (d.ok) {
      log(`Model set to ${variant.toUpperCase()}: ${d.model_path}`, 'ok');
      pollStatus();
    }
  } catch (e) { log('Select failed: ' + e.message, 'err'); }
});

async function downloadModel(variant) {
  const controller = new AbortController();
  activeDownloadController = controller;
  pollStatus();

  const fill  = document.getElementById('prog-model-fill');
  const label = document.getElementById('prog-model-label');
  const wrap  = document.getElementById('prog-model-wrap');
  wrap.classList.add('visible');
  fill.style.background = ''; fill.style.width = '0%'; label.textContent = 'Starting…';

  log(`Downloading model for ${variant.toUpperCase()}…`, 'ok');
  const ok = await streamDownload(`/api/download?target=model-${variant}`, fill, label, controller.signal);
  activeDownloadController = null;
  if (ok) log(`Model downloaded for ${variant.toUpperCase()}.`, 'ok');
  pollStatus();
}

// ─── Vision toggle ─────────────────────────────────────────────
document.getElementById('vision-toggle').addEventListener('change', async function() {
  const enabled = this.checked;
  const variant = document.getElementById('model-selector').value;

  if (enabled) {
    const p = (lastModelsPresent?.[variant]) || {};
    if (!p.mmproj) {
      // mmproj not on disk — download it first
      mmprojDownloadController = new AbortController();
      const fill  = document.getElementById('prog-model-fill');
      const label = document.getElementById('prog-model-label');
      const wrap  = document.getElementById('prog-model-wrap');
      wrap.classList.add('visible');
      fill.style.background = ''; fill.style.width = '0%'; label.textContent = 'Downloading mmproj…';
      log(`Downloading mmproj for ${variant.toUpperCase()}…`, 'ok');

      const ok = await streamDownload(`/api/download?target=mmproj-${variant}`, fill, label, mmprojDownloadController.signal);
      mmprojDownloadController = null;
      if (!ok) {
        this.checked = false;
        pollStatus();
        return;
      }
      log(`mmproj downloaded for ${variant.toUpperCase()}.`, 'ok');
    }
    await fetch(`/api/vision/toggle?enabled=true&variant=${encodeURIComponent(variant)}`, { method: 'POST' });
    log('Vision enabled — restart llama-server to apply', 'ok');
  } else {
    await fetch('/api/vision/toggle?enabled=false', { method: 'POST' });
    log('Vision disabled', 'ok');
  }
  pollStatus();
});

// Returns true on success, false on cancel/error.
async function streamDownload(url, fill, label, signal) {
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
            fill.style.width = e.pct + '%';
            label.textContent = `${e.pct}% — ${fmtGB(e.bytes)} / ${fmtGB(e.total)}`;
          }
          if (e.done) { fill.style.background = '#44ff88'; label.textContent = 'Done!'; return true; }
          if (e.error) { fill.style.background = '#ff4444'; label.textContent = 'Error: ' + e.error; log('Download error: ' + e.error, 'err'); return false; }
        } catch { /* skip */ }
      }
    }
    return true;
  } catch (e) {
    if (e.name === 'AbortError') return false;
    label.textContent = 'Failed: ' + e.message;
    log('Download failed: ' + e.message, 'err');
    return false;
  }
}

// ─── Start/Stop llama-server ───────────────────────────────────
document.getElementById('btn-start-llama').addEventListener('click', async () => {
  const btnStart = document.getElementById('btn-start-llama');
  const btnStop  = document.getElementById('btn-stop-llama');
  btnStart.disabled = true;
  btnStart.textContent = '⏳ Starting…';
  log('Starting llama-server — loading model, please wait…', 'ok');
  try {
    const d = await fetch('/api/llama/start', { method: 'POST' }).then(r => r.json());
    if (d.error) { log('Start failed: ' + d.error, 'err'); }
    else { log('llama-server is ready.', 'ok'); }
  } catch (e) { log('Start failed: ' + e.message, 'err'); }
  btnStart.disabled = false;
  btnStart.textContent = '▶ Start llama-server';
  pollStatus();
});

document.getElementById('btn-stop-llama').addEventListener('click', async () => {
  const btn = document.getElementById('btn-stop-llama');
  btn.disabled = true;
  btn.textContent = '⏳ Stopping…';
  log('Stopping llama-server…', 'warn');
  try {
    await fetch('/api/llama/stop', { method: 'POST' });
    log('llama-server stopped.', 'warn');
  } catch (e) { log('Stop failed: ' + e.message, 'err'); }
  btn.disabled = false;
  btn.textContent = '■ Stop llama-server';
  pollStatus();
});

// ─── Llama mode radio ──────────────────────────────────────────
function setLlamaMode(mode) {
  const isRemote = mode === 'remote';
  document.getElementById('radio-local').checked = !isRemote;
  document.getElementById('radio-remote').checked = isRemote;
  document.getElementById('lbl-local').classList.toggle('active', !isRemote);
  document.getElementById('lbl-remote').classList.toggle('active', isRemote);
  document.getElementById('panel-local').style.display = isRemote ? 'none' : '';
  document.getElementById('panel-remote').style.display = isRemote ? '' : 'none';
  applyRemoteMode(isRemote);
}
document.querySelectorAll('input[name="llama-mode"]').forEach(r => {
  r.addEventListener('change', () => setLlamaMode(r.value));
});

// ─── Load config ───────────────────────────────────────────────
async function loadConfig() {
  try {
    const d = await fetch('/api/config').then(r => r.json());
    document.getElementById('cfg-http-host').value = d.http_host ?? '';
    document.getElementById('cfg-http-port').value = d.http_port ?? '';
    document.getElementById('cfg-https-port').value = d.https_port ?? '';
    document.getElementById('cfg-upload-dir').value = d.upload_dir ?? '';

    const loc = d.llama_local ?? {};
    const rem = d.llama_remote ?? {};
    document.getElementById('cfg-local-endpoint').value = loc.endpoint ?? 'http://localhost:11434';
    document.getElementById('cfg-llama-bin').value = loc.llama_bin ?? '';
    document.getElementById('cfg-model-path').value = loc.model_path ?? '';
    document.getElementById('cfg-mmproj-path').value = loc.mmproj_path ?? '';
    document.getElementById('cfg-llama-bin-version').value = loc.llama_bin_version ?? '';
    document.getElementById('cfg-context-size').value = loc.context_size ?? '';
    document.getElementById('cfg-remote-endpoint').value = rem.endpoint ?? '';

    const urls = d.model_urls ?? {};
    document.getElementById('cfg-url-e2b').value = urls.e2b || DEFAULT_MODEL_URLS.e2b;
    document.getElementById('cfg-url-e4b').value = urls.e4b || DEFAULT_MODEL_URLS.e4b;
    //document.getElementById('cfg-url-e31b').value = urls.e31b || DEFAULT_MODEL_URLS.e31b;

    setLlamaMode(rem.enabled ? 'remote' : 'local');
    log('Configuration loaded', 'ok');
  } catch (e) { log('Failed to load config: ' + e.message, 'err'); }
}

// ─── Save config ───────────────────────────────────────────────
document.getElementById('btn-save-cfg').addEventListener('click', async () => {
  const isRemote = document.getElementById('radio-remote').checked;
  const body = {
    http_host: document.getElementById('cfg-http-host').value.trim(),
    http_port: document.getElementById('cfg-http-port').value.trim(),
    https_port: document.getElementById('cfg-https-port').value.trim(),
    upload_dir: document.getElementById('cfg-upload-dir').value.trim(),
    llama_local: {
      enabled: !isRemote,
      endpoint: document.getElementById('cfg-local-endpoint').value.trim(),
      llama_bin: document.getElementById('cfg-llama-bin').value.trim(),
      model_path: document.getElementById('cfg-model-path').value.trim(),
      mmproj_path: document.getElementById('cfg-mmproj-path').value.trim(),
      llama_bin_version: document.getElementById('cfg-llama-bin-version').value.trim(),
      context_size: parseInt(document.getElementById('cfg-context-size').value.trim(), 10) || 0,
    },
    llama_remote: {
      enabled: isRemote,
      endpoint: document.getElementById('cfg-remote-endpoint').value.trim(),
    },
    model_urls: {
      e2b:  document.getElementById('cfg-url-e2b').value.trim(),
      e4b:  document.getElementById('cfg-url-e4b').value.trim(),
      //e31b: document.getElementById('cfg-url-e31b').value.trim(),
    },
  };
  const st = document.getElementById('cfg-status');
  st.textContent = 'Saving…'; st.style.color = '#888';
  try {
    const d = await fetch('/api/config', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).then(r => r.json());
    if (d.ok) {
      st.textContent = 'Saved ✓'; st.style.color = '#44ff88';
      document.getElementById('cfg-restart-warn').style.display = d.ports_changed ? 'block' : 'none';
      log('Configuration saved' + (d.ports_changed ? ' — restart required' : ''), d.ports_changed ? 'warn' : 'ok');
      pollStatus();
      document.querySelector('.tab-btn[data-tab="dashboard"]').click();
    } else { st.textContent = 'Error'; st.style.color = '#ff4444'; }
  } catch (e) { st.textContent = 'Failed'; st.style.color = '#ff4444'; log('Config save error: ' + e.message, 'err'); }
  setTimeout(() => { st.textContent = ''; st.style.color = ''; }, 4000);
});

// ─── Reset model URLs ──────────────────────────────────────────
document.getElementById('btn-reset-e2b').addEventListener('click', () => {
  document.getElementById('cfg-url-e2b').value = DEFAULT_MODEL_URLS.e2b;
});
document.getElementById('btn-reset-e4b').addEventListener('click', () => {
  document.getElementById('cfg-url-e4b').value = DEFAULT_MODEL_URLS.e4b;
});
//document.getElementById('btn-reset-e31b').addEventListener('click', () => {
//  document.getElementById('cfg-url-e31b').value = DEFAULT_MODEL_URLS.e31b;
//});

// ─── Test connection ───────────────────────────────────────────
async function testEndpoint(endpoint, statusEl) {
  statusEl.textContent = 'Testing…'; statusEl.className = ''; statusEl.style.cssText = 'font-size:0.8rem;color:#888';
  try {
    const d = await fetch(`/api/llama/test?endpoint=${encodeURIComponent(endpoint)}`).then(r => r.json());
    if (d.reachable) {
      statusEl.innerHTML = `<span class="badge badge-ok">✓ reachable${d.model_name ? ' — ' + d.model_name : ''}</span>`;
      log(`llama-server reachable at ${endpoint}${d.model_name ? ' · model: ' + d.model_name : ''}`, 'ok');
    } else {
      statusEl.innerHTML = `<span class="badge badge-err">✗ not reachable</span>`;
      log(`llama-server not reachable at ${endpoint}`, 'warn');
    }
  } catch (e) {
    statusEl.innerHTML = `<span class="badge badge-err">✗ error</span>`;
    log('Test failed: ' + e.message, 'err');
  }
}
document.getElementById('btn-test-local').addEventListener('click', () => {
  testEndpoint(
    document.getElementById('cfg-local-endpoint').value.trim(),
    document.getElementById('conn-status-local')
  );
});
document.getElementById('btn-test-remote').addEventListener('click', () => {
  testEndpoint(
    document.getElementById('cfg-remote-endpoint').value.trim(),
    document.getElementById('conn-status-remote')
  );
});

// ─── Regenerate SSL cert ───────────────────────────────────────
document.getElementById('btn-regen-cert').addEventListener('click', async () => {
  const st = document.getElementById('cert-status');
  st.textContent = 'Generating…'; st.style.color = '#888';
  log('Regenerating SSL certificate…');
  try {
    const d = await fetch('/api/cert/regenerate', { method: 'POST' }).then(r => r.json());
    if (d.ok) {
      st.textContent = 'Generated ✓'; st.style.color = '#44ff88';
      log(`Certificate generated — SANs: ${d.hostnames.join(', ')} | IPs: ${d.ips.join(', ')}`, 'ok');
    } else { st.textContent = 'Failed'; st.style.color = '#ff4444'; }
  } catch (e) { st.textContent = 'Failed'; st.style.color = '#ff4444'; log('Cert error: ' + e.message, 'err'); }
  setTimeout(() => { st.textContent = ''; st.style.color = ''; }, 5000);
});

// ─── QR Code ───────────────────────────────────────────────────
async function loadQR() {
  try {
    const d = await fetch('/api/qr-url').then(r => r.json());
    document.getElementById('qr-url').textContent = d.url;
    document.getElementById('qr-img').src = '/api/qr?' + Date.now();
  } catch (e) { /* silent — server may not be ready */ }
}

// ─── Init ──────────────────────────────────────────────────────
log('Admin dashboard loaded', 'ok');
loadQR();
loadConfig();
loadLlamaReleases();
pollSysinfo();
pollStatus();
setInterval(pollSysinfo, 5000);
setInterval(pollStatus, 4000);
