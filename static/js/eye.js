const state = { current: 'ask' };
const els = {
    body: document.body,
    video: document.getElementById('video'),
    snapshot: document.getElementById('snapshot'),
    finder: document.getElementById('finder'),
    question: document.getElementById('question'),
    answerBox: document.getElementById('answer-box'),
    loader: document.getElementById('loader'),
    btnRetake: document.getElementById('btn-retake'),
    btns: {
        capture: document.getElementById('btn-capture'),
        ask: document.getElementById('btn-ask'),
        stop: document.getElementById('btn-stop'),
        new: document.getElementById('btn-new')
    }
};

let capturedBlob = null;
let abortController = null;
let cameraStream = null;
let serverVisionCapable = true;

// Chiediamo i permessi SUBITO all'avvio in background
async function prefetchCamera() {
    try {
        cameraStream = await navigator.mediaDevices.getUserMedia({
            video: { facingMode: 'environment', width: { ideal: 1280 }, height: { ideal: 720 } }
        });
        els.video.srcObject = cameraStream;
        await els.video.play();
        console.log("Fotocamera inizializzata e pronta.");
    } catch (e) {
        console.error("Permesso fotocamera negato o non disponibile:", e);
    }
}

// Cambia modalità (Vision vs Testo) in maniera fluida
async function switchMode(mode) {
    if (mode === 'vision' && serverVisionCapable) {
        els.body.classList.remove('mode-text');
        els.body.classList.add('mode-vision');
        els.btnRetake.style.display = 'block';

        // Se la fotocamera era stata spenta o rifiutata prima, riproviamo ad accenderla
        if (!cameraStream || !els.video.srcObject) {
            try {
                cameraStream = await navigator.mediaDevices.getUserMedia({
                    video: { facingMode: 'environment', width: { ideal: 1280 }, height: { ideal: 720 } }
                });
                els.video.srcObject = cameraStream;
                await els.video.play();
            } catch (e) {
                alert("Fotocamera non disponibile.");
                return switchMode('text');
            }
        }
        setUIState('preview');
    } else {
        // Modalità SOLO TESTO
        els.body.classList.remove('mode-vision');
        els.body.classList.add('mode-text');
        els.btnRetake.style.display = 'none';
        capturedBlob = null;
        els.snapshot.src = "";
        setUIState('ask');
    }
}

function setUIState(s) {
    state.current = s;
    els.body.setAttribute('data-state', s);
}

// Controllo del server all'inizio
window.addEventListener('DOMContentLoaded', async () => {
    const overlay = document.getElementById('boot-overlay');

    // Chiedi subito la fotocamera all'utente per toglierci il pensiero
    await prefetchCamera();

    try {
        const res = await fetch('/api/status');
        if (!res.ok) throw new Error();
        const status = await res.json();

        if (status && status.ready) {
            overlay.style.display = 'none';
            serverVisionCapable = status.vision_enabled;
            if (!serverVisionCapable) {
                els.body.classList.add('server-no-vision');
            }
            // Atterriamo in ogni caso su Solo Testo all'avvio
            switchMode('text');
        } else {
            overlay.className = 'error';
        }
    } catch (e) {
        overlay.className = 'error';
    }
});

document.querySelectorAll('.chip').forEach(chip => {
    chip.onclick = () => { els.question.value = chip.dataset.q; };
});

// Scatto foto - Versione Corretta e Adattiva
els.btns.capture.onclick = () => {
    if (!cameraStream) {
        console.error("Fotocamera non pronta.");
        return;
    }
    const canvas = document.createElement('canvas');
    const ctx = canvas.getContext('2d');

    // Recuperiamo i rettangoli di posizionamento nello schermo sia del mirino che del video
    const finderRect = els.finder.getBoundingClientRect();
    const videoRect = els.video.getBoundingClientRect();

    // Dimensioni reali (native) della sorgente video della fotocamera
    const vWidth = els.video.videoWidth;
    const vHeight = els.video.videoHeight;

    // Calcoliamo i rapporti di scala reali tra il flusso video nativo e come il tag video appare a schermo
    const scaleX = vWidth / videoRect.width;
    const scaleY = vHeight / videoRect.height;

    // Troviamo la posizione esatta del mirino RELATIVA ai bordi del video
    const relativeLeft = finderRect.left - videoRect.left;
    const relativeTop = finderRect.top - videoRect.top;

    // Definiamo la dimensione del canvas finale basandoci sulla porzione scalata del mirino
    canvas.width = finderRect.width * scaleX;
    canvas.height = finderRect.height * scaleY;

    // Eseguiamo il disegno prendendo solo l'area circoscritta dal mirino sul video
    ctx.drawImage(els.video,
        relativeLeft * scaleX, relativeTop * scaleY,      // Inizio del ritaglio sulla sorgente video
        finderRect.width * scaleX, finderRect.height * scaleY, // Dimensioni del ritaglio sulla sorgente
        0, 0, canvas.width, canvas.height                 // Destinazione sul canvas (riempimento totale)
    );

    // Conversione in JPEG
    canvas.toBlob(b => {
        if (!b) {
            console.error("Errore: Il Blob generato è nullo.");
            return;
        }
        capturedBlob = b;
        
        // Pulizia della memoria dal vecchio blob per evitare memory leak
        if (els.snapshot.src && els.snapshot.src.startsWith('blob:')) {
            URL.revokeObjectURL(els.snapshot.src);
        }

        els.snapshot.src = URL.createObjectURL(b);
        setUIState('ask');
    }, 'image/jpeg', 0.85);
};

els.btnRetake.onclick = () => {
    capturedBlob = null;
    els.snapshot.src = "";
    setUIState('preview');
};

els.btns.ask.onclick = async () => {
    const q = els.question.value || "Describe this.";
    setUIState('response');

    els.answerBox.innerHTML = `<div class="thinking-placeholder">Gemma is processing your request...</div>`;
    els.loader.classList.add('active');
    els.btns.stop.disabled = false;
    els.btns.new.disabled = true;

    abortController = new AbortController();
    const fd = new FormData();
    fd.append('question', q);

    if (capturedBlob && els.body.classList.contains('mode-vision')) {
        fd.append('image', capturedBlob, 'crop.jpg');
    }

    try {
        const response = await fetch('/ask', {
            method: 'POST',
            body: fd,
            signal: abortController.signal
        });

        if (!response.body) throw new Error("No response");
        const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();

        let fullText = ""; // Accumulatore per evitare i bug del Markdown in streaming
        let isFirstToken = true;

        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            fullText += value;
            const lines = fullText.split('\n');

            // Manteniamo l'ultima linea incompleta nel buffer di lettura
            fullText = lines.pop();

            for (const line of lines) {
                const trimmed = line.trim();
                if (trimmed.startsWith('data: ')) {
                    const dataPayload = trimmed.slice(6);
                    if (dataPayload === '[DONE]') break;
                    try {
                        const token = JSON.parse(dataPayload);

                        // Appena arriva il PRIMO token, cancelliamo istantaneamente il placeholder
                        if (isFirstToken) {
                            els.answerBox.innerHTML = "";
                            isFirstToken = false;
                        }

                        // Accumuliamo il testo pulito
                        els.answerBox.textContent += token;

                        // Applichiamo la formattazione dei grassetti sull'intero contenuto aggiornato
                        // Questo evita che i token spezzati rompano il Markdown
                        let formattedHTML = els.answerBox.textContent.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');

                        els.answerBox.innerHTML = formattedHTML;
                        els.answerBox.scrollTop = els.answerBox.scrollHeight;
                    } catch (e) { }
                }
            }
        }
    } catch (e) {
        if (e.name === 'AbortError') {
            els.answerBox.innerHTML += "<br><br><strong>[Interrotto dall'utente]</strong>";
        } else {
            els.answerBox.textContent = "Error connecting to backend.";
        }
    } finally {
        els.loader.classList.remove('active');
        els.btns.stop.disabled = true;
        els.btns.new.disabled = false;
        abortController = null;
    }
};

els.btns.stop.onclick = () => { if (abortController) abortController.abort(); };
els.btns.new.onclick = () => {
    els.question.value = '';
    els.answerBox.textContent = '';
    if (els.body.classList.contains('mode-text')) {
        setUIState('ask');
    } else {
        capturedBlob = null;
        els.snapshot.src = "";
        setUIState('preview');
    }
};
