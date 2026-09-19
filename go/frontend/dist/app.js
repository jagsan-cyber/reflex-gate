// ==========================================================================
// LOCAL-JEV Frontend Application Logic (Wails / WebView2)
// ==========================================================================

const i18n = {
  ja: {
    title: "local-jev ランチャー",
    subtitle: "Qwen2.5-Coder-1.5B AI Gateway",
    serverModelConfig: "サーバー & モデル設定",
    lblLlama: "llama-server.exe のパス",
    lblModel: "GGUF モデルパス",
    btnBrowse: "参照",
    btnAutoDL: "バイナリ & モデル自動取得",
    execSettings: "実行オプション & ポート",
    lblContext: "コンテキスト長",
    optCtx8k: "8,192 (推奨)",
    optCtx32k: "32,768 (最大)",
    lblBackend: "推論バックエンド",
    optBackendAuto: "自動検出 (推奨)",
    optBackendVulkan: "Vulkan (AMD 860M / Intel / 汎用)",
    optBackendCUDA: "CUDA (NVIDIA RTX / GeForce)",
    optBackendHIP: "ROCm / HIP (AMD Radeon)",
    optBackendSYCL: "Intel SYCL (Arc / Core Ultra)",
    optBackendCPU: "CPU のみ (NGL=0)",
    lblJevPort: "JEV API ポート",
    lblLlamaPort: "llama-server ポート",
    btnStart: "JEV 起動",
    btnStop: "停止",
    btnDemo: "Webデモを開く",
    btnCopyURL: "URLコピー",
    quickTest: "クイックテスト実行",
    liveMonitor: "リアルタイム AI パフォーマンス監視",
    lblLatencyMeter: "応答レイテンシ・レベルメーター",
    kpiLatency: "応答レイテンシ",
    kpiTTFT: "TTFT (初回応答)",
    kpiSpeed: "生成速度",
    kpiCalls: "総コール数",
    recentLogs: "リアルタイム推論ログ",
    btnLogCopy: "コピー",
    btnLogClear: "クリア",
    btnTestStop: "Stop 判定 (CoT)",
    btnTestExtract: "Diff抽出",
    btnTestScan: "セマンティック走査",
    btnTestStress: "⚡ 負荷連打",
    terminalEmpty: "リクエスト履歴がありません。サーバーを起動すると推論ログがリアルタイム表示されます。",
    statusStopped: "停止中",
    statusStarting: "起動処理中...",
    statusFallbackCPU: "GPU失敗 -> CPU切替中...",
    statusRunning: "稼働中: Port %d",
    statusRunningCPU: "稼働中 (CPU): Port %d",
    msgURLCopied: "エンドポイント URL をコピーしました",
    msgLogCopied: "ログをクリップボードにコピーしました",
    msgLogCleared: "ログをクリアしました",
    msgDLDone: "ダウンロードが完了しました",
    msgDLFail: "ダウンロードに失敗しました: ",
    msgCPUFallbackToast: "GPU初期化に失敗したため、CPUモードにフォールバックしました"
  },
  en: {
    title: "local-jev Launcher",
    subtitle: "Qwen2.5-Coder-1.5B Fast AI Gateway",
    serverModelConfig: "Server & Model Configuration",
    lblLlama: "Path to llama-server.exe",
    lblModel: "GGUF Model Path",
    btnBrowse: "Browse",
    btnAutoDL: "Auto Fetch Binary & Model",
    execSettings: "Execution Options & Ports",
    lblContext: "Context Size",
    optCtx8k: "8,192 (Recommended)",
    optCtx32k: "32,768 (Maximum)",
    lblBackend: "Acceleration Backend",
    optBackendAuto: "Auto Detect (Recommended)",
    optBackendVulkan: "Vulkan (AMD 860M / Intel / Generic)",
    optBackendCUDA: "CUDA (NVIDIA RTX / GeForce)",
    optBackendHIP: "ROCm / HIP (AMD Radeon)",
    optBackendSYCL: "Intel SYCL (Arc / Core Ultra)",
    optBackendCPU: "CPU Only (NGL=0)",
    lblJevPort: "JEV API Port",
    lblLlamaPort: "llama-server Port",
    btnStart: "Start Server",
    btnStop: "Stop",
    btnDemo: "Open Web Demo",
    btnCopyURL: "Copy URL",
    quickTest: "Quick Test Suite",
    btnTestStop: "Stop Check (CoT)",
    btnTestExtract: "Diff Extract",
    btnTestScan: "Semantic Scan",
    btnTestStress: "⚡ Stress Run",
    terminalEmpty: "No requests yet. Start server to monitor activity.",
    liveMonitor: "Live AI Performance Monitor",
    lblLatencyMeter: "Response Latency Level Meter",
    kpiLatency: "Latency",
    kpiTTFT: "TTFT (First Token)",
    kpiSpeed: "Speed",
    kpiCalls: "Total Calls",
    recentLogs: "Real-time Telemetry Logs",
    btnLogCopy: "Copy",
    btnLogClear: "Clear",
    statusStopped: "STOPPED",
    statusStarting: "STARTING...",
    statusFallbackCPU: "GPU Failed -> Switching to CPU...",
    statusRunning: "RUNNING: Port %d",
    statusRunningCPU: "RUNNING (CPU): Port %d",
    msgURLCopied: "Endpoint URL copied to clipboard",
    msgLogCopied: "Logs copied to clipboard",
    msgLogCleared: "Logs cleared",
    msgDLDone: "Download complete",
    msgDLFail: "Download failed: ",
    msgCPUFallbackToast: "GPU initialization failed; fell back to CPU mode"
  }
};

let currentLang = "ja";
let currentHwInfo = null;
let currentStatus = { running: false, starting: false, jev_port: 8090 };
let latencyHistory = [];
let totalCallsCount = 0;
let slot0Timeout = null;
let slot1Timeout = null;
let slot2Timeout = null;

// DOM Element References
const elements = {
  hdrSubtitle: document.getElementById("hdrSubtitle"),
  hwText: document.getElementById("hwText"),
  statusPill: document.getElementById("statusPill"),
  statusLabel: document.getElementById("statusLabel"),
  btnLangToggle: document.getElementById("btnLangToggle"),

  dlBanner: document.getElementById("dlBanner"),
  dlLabel: document.getElementById("dlLabel"),
  dlPercent: document.getElementById("dlPercent"),
  dlProgressBar: document.getElementById("dlProgressBar"),

  txtServerModelConfig: document.getElementById("txtServerModelConfig"),
  lblLlamaServer: document.getElementById("lblLlamaServer"),
  inputLlamaServer: document.getElementById("inputLlamaServer"),
  btnBrowseLlama: document.getElementById("btnBrowseLlama"),
  lblModel: document.getElementById("lblModel"),
  inputModel: document.getElementById("inputModel"),
  btnBrowseModel: document.getElementById("btnBrowseModel"),
  btnAutoDL: document.getElementById("btnAutoDL"),
  txtAutoDL: document.getElementById("txtAutoDL"),

  txtExecSettings: document.getElementById("txtExecSettings"),
  lblContext: document.getElementById("lblContext"),
  selectContext: document.getElementById("selectContext"),
  optCtx8k: document.getElementById("optCtx8k"),
  optCtx32k: document.getElementById("optCtx32k"),
  lblBackend: document.getElementById("lblBackend"),
  selectBackend: document.getElementById("selectBackend"),
  optBackendAuto: document.getElementById("optBackendAuto"),
  optBackendVulkan: document.getElementById("optBackendVulkan"),
  optBackendCUDA: document.getElementById("optBackendCUDA"),
  optBackendHIP: document.getElementById("optBackendHIP"),
  optBackendSYCL: document.getElementById("optBackendSYCL"),
  optBackendCPU: document.getElementById("optBackendCPU"),
  lblJevPort: document.getElementById("lblJevPort"),
  inputJevPort: document.getElementById("inputJevPort"),
  lblLlamaPort: document.getElementById("lblLlamaPort"),
  inputLlamaPort: document.getElementById("inputLlamaPort"),

  btnStartServer: document.getElementById("btnStartServer"),
  txtStartServer: document.getElementById("txtStartServer"),
  btnStopServer: document.getElementById("btnStopServer"),
  txtStopServer: document.getElementById("txtStopServer"),
  btnOpenDemo: document.getElementById("btnOpenDemo"),
  txtOpenDemo: document.getElementById("txtOpenDemo"),
  btnCopyURL: document.getElementById("btnCopyURL"),
  txtCopyURL: document.getElementById("txtCopyURL"),

  txtQuickTest: document.getElementById("txtQuickTest"),
  btnTestStop: document.getElementById("btnTestStop"),
  btnTestExtract: document.getElementById("btnTestExtract"),
  btnTestScan: document.getElementById("btnTestScan"),
  btnTestStress: document.getElementById("btnTestStress"),

  txtLiveMonitor: document.getElementById("txtLiveMonitor"),
  engineBadge: document.getElementById("engineBadge"),
  engineIcon: document.getElementById("engineIcon"),
  engineText: document.getElementById("engineText"),
  slot0Badge: document.getElementById("slot0Badge"),
  slot1Badge: document.getElementById("slot1Badge"),
  slot2Badge: document.getElementById("slot2Badge"),

  lblLatencyMeter: document.getElementById("lblLatencyMeter"),
  meterLatencyText: document.getElementById("meterLatencyText"),
  meterFill: document.getElementById("meterFill"),
  waveformCanvas: document.getElementById("waveformCanvas"),

  kpiHdrLatency: document.getElementById("kpiHdrLatency"),
  kpiValLatency: document.getElementById("kpiValLatency"),
  kpiHdrTTFT: document.getElementById("kpiHdrTTFT"),
  kpiValTTFT: document.getElementById("kpiValTTFT"),
  kpiHdrSpeed: document.getElementById("kpiHdrSpeed"),
  kpiValSpeed: document.getElementById("kpiValSpeed"),
  kpiHdrCalls: document.getElementById("kpiHdrCalls"),
  kpiValCalls: document.getElementById("kpiValCalls"),

  txtRecentLogs: document.getElementById("txtRecentLogs"),
  btnLogCopy: document.getElementById("btnLogCopy"),
  btnLogClear: document.getElementById("btnLogClear"),
  logTerminal: document.getElementById("logTerminal"),
  terminalEmpty: document.getElementById("terminalEmpty"),

  appToast: document.getElementById("appToast"),
  toastMessage: document.getElementById("toastMessage")
};

// Canvas 2D Context
const ctx = elements.waveformCanvas.getContext("2d");

// ==========================================================================
// Language and UI Text Updates
// ==========================================================================
function updateLanguage(lang) {
  currentLang = lang;
  const t = i18n[lang] || i18n.ja;

  elements.hdrSubtitle.textContent = t.subtitle;
  elements.btnLangToggle.querySelector(".lang-text").textContent = (lang === "en" ? "JA" : "EN");

  elements.txtServerModelConfig.textContent = t.serverModelConfig;
  elements.lblLlamaServer.textContent = t.lblLlama;
  elements.btnBrowseLlama.textContent = t.btnBrowse;
  elements.lblModel.textContent = t.lblModel;
  elements.btnBrowseModel.textContent = t.btnBrowse;
  elements.txtAutoDL.textContent = t.btnAutoDL;

  elements.txtExecSettings.textContent = t.execSettings;
  elements.lblContext.textContent = t.lblContext;
  if (elements.optCtx8k) elements.optCtx8k.textContent = t.optCtx8k;
  if (elements.optCtx32k) elements.optCtx32k.textContent = t.optCtx32k;
  if (elements.lblBackend) elements.lblBackend.textContent = t.lblBackend;
  if (elements.optBackendAuto) elements.optBackendAuto.textContent = t.optBackendAuto;
  if (elements.optBackendVulkan) elements.optBackendVulkan.textContent = t.optBackendVulkan;
  if (elements.optBackendCUDA) elements.optBackendCUDA.textContent = t.optBackendCUDA;
  if (elements.optBackendHIP) elements.optBackendHIP.textContent = t.optBackendHIP;
  if (elements.optBackendSYCL) elements.optBackendSYCL.textContent = t.optBackendSYCL;
  if (elements.optBackendCPU) elements.optBackendCPU.textContent = t.optBackendCPU;
  elements.lblJevPort.textContent = t.lblJevPort;
  elements.lblLlamaPort.textContent = t.lblLlamaPort;

  elements.txtStartServer.textContent = t.btnStart;
  elements.txtStopServer.textContent = t.btnStop;
  elements.txtOpenDemo.textContent = t.btnDemo;
  elements.txtCopyURL.textContent = t.btnCopyURL;

  elements.txtQuickTest.textContent = t.quickTest;
  elements.txtLiveMonitor.textContent = t.liveMonitor;
  elements.lblLatencyMeter.textContent = t.lblLatencyMeter;

  elements.kpiHdrLatency.textContent = t.kpiLatency;
  elements.kpiHdrTTFT.textContent = t.kpiTTFT;
  elements.kpiHdrSpeed.textContent = t.kpiSpeed;
  elements.kpiHdrCalls.textContent = t.kpiCalls;

  elements.btnTestStop.textContent = t.btnTestStop;
  elements.btnTestExtract.textContent = t.btnTestExtract;
  elements.btnTestScan.textContent = t.btnTestScan;
  if (elements.btnTestStress && !stressRunning) {
    elements.btnTestStress.textContent = t.btnTestStress;
  }

  elements.txtRecentLogs.textContent = t.recentLogs;
  elements.btnLogCopy.textContent = t.btnLogCopy;
  elements.btnLogClear.textContent = t.btnLogClear;

  if (elements.terminalEmpty) {
    elements.terminalEmpty.textContent = t.terminalEmpty;
  }

  if (currentHwInfo) {
    elements.hwText.textContent = (lang === "en" ? currentHwInfo.summary_en : currentHwInfo.summary) || "Hardware ready";
  }

  if (currentStatus) {
    updateStatus(currentStatus);
  }
}

// ==========================================================================
// Toast Notification
// ==========================================================================
let toastTimer = null;
function showToast(msg, duration = 3000) {
  elements.toastMessage.textContent = msg;
  elements.appToast.classList.remove("hidden");
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    elements.appToast.classList.add("hidden");
  }, duration);
}

function appendErrorLog(msg) {
  if (elements.terminalEmpty) {
    elements.terminalEmpty.remove();
    elements.terminalEmpty = null;
  }

  const timeStr = new Date().toTimeString().split(" ")[0];
  const row = document.createElement("div");
  row.className = "log-row";
  row.style.borderLeft = "2px solid #F43F5E";
  row.innerHTML = `
    <span class="log-time" style="color:#F43F5E;">[${timeStr}]</span>
    <span class="log-task" style="color:#F43F5E;border-color:rgba(244,63,94,0.3);background:rgba(244,63,94,0.1);">ALERT</span>
    <span class="log-summary" style="color:#FDA4AF;">${escapeHtml(msg)}</span>
  `;

  elements.logTerminal.appendChild(row);
  while (elements.logTerminal.children.length > 50) {
    elements.logTerminal.removeChild(elements.logTerminal.firstChild);
  }
  elements.logTerminal.scrollTop = elements.logTerminal.scrollHeight;
}

// ==========================================================================
// Canvas Oscilloscope Waveform Renderer
// ==========================================================================
function drawWaveform() {
  const w = elements.waveformCanvas.width;
  const h = elements.waveformCanvas.height;

  ctx.clearRect(0, 0, w, h);

  // Background
  ctx.fillStyle = "#07080B";
  ctx.fillRect(0, 0, w, h);

  // Horizontal Grid Lines (500ms, 200ms, 100ms)
  const gridLines = [
    { ms: 500, label: "500ms", color: "#1E2638" },
    { ms: 200, label: "200ms", color: "#1E2638" },
    { ms: 100, label: "100ms", color: "#223344" }
  ];

  ctx.font = "9px Cascadia Code, monospace";
  ctx.fillStyle = "#64748B";

  gridLines.forEach(g => {
    const y = h - (g.ms / 600.0) * (h - 10);
    ctx.strokeStyle = g.color;
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(w, y);
    ctx.stroke();
    ctx.fillText(g.label, 6, y - 3);
  });

  // Plot Data Points
  const len = latencyHistory.length;
  if (len < 1) return;

  const xStep = (w - 30) / Math.max(29, len - 1);
  const points = [];

  for (let i = 0; i < len; i++) {
    const px = 15 + i * xStep;
    const ms = latencyHistory[i];
    let py = h - (ms / 600.0) * (h - 14) - 4;
    py = Math.max(6, Math.min(h - 6, py));
    points.push({ x: px, y: py });
  }

  // Draw Gradient Area under the curve
  ctx.beginPath();
  ctx.moveTo(points[0].x, h);
  points.forEach(p => ctx.lineTo(p.x, p.y));
  ctx.lineTo(points[points.length - 1].x, h);
  ctx.closePath();

  const grad = ctx.createLinearGradient(0, 0, 0, h);
  grad.addColorStop(0, "rgba(0, 210, 255, 0.25)");
  grad.addColorStop(1, "rgba(0, 210, 255, 0.0)");
  ctx.fillStyle = grad;
  ctx.fill();

  // Draw Smooth Neon Line
  ctx.beginPath();
  ctx.strokeStyle = "#00D2FF";
  ctx.lineWidth = 2.5;
  ctx.shadowColor = "#00D2FF";
  ctx.shadowBlur = 8;

  points.forEach((p, i) => {
    if (i === 0) ctx.moveTo(p.x, p.y);
    else ctx.lineTo(p.x, p.y);
  });
  ctx.stroke();
  ctx.shadowBlur = 0; // reset shadow

  // Draw Dot on Latest Data Point
  const lastPt = points[points.length - 1];
  ctx.beginPath();
  ctx.arc(lastPt.x, lastPt.y, 4.5, 0, Math.PI * 2);
  ctx.fillStyle = "#00FF88";
  ctx.shadowColor = "#00FF88";
  ctx.shadowBlur = 10;
  ctx.fill();
  ctx.shadowBlur = 0;
}

// ==========================================================================
// Telemetry Updates (Event Driven)
// ==========================================================================
function onTelemetry(ev) {
  totalCallsCount++;
  latencyHistory.push(ev.LatencyMs);
  if (latencyHistory.length > 30) {
    latencyHistory.shift();
  }

  // Update KPI displays
  elements.kpiValLatency.innerHTML = `${ev.LatencyMs} <span class="unit">ms</span>`;
  elements.kpiValTTFT.innerHTML = `${ev.TTFTMs || 0} <span class="unit">ms</span>`;
  elements.kpiValSpeed.innerHTML = `${(ev.TokPerSec || 0).toFixed(1)} <span class="unit">t/s</span>`;
  elements.kpiValCalls.innerHTML = `${totalCallsCount} <span class="unit">reqs</span>`;

  // Update Audio-VU Latency Meter Bar
  elements.meterLatencyText.textContent = `${ev.LatencyMs} ms`;
  let pct = (ev.LatencyMs / 500.0) * 100.0;
  pct = Math.min(100.0, Math.max(4.0, pct));
  elements.meterFill.style.width = `${pct}%`;

  // Update Slot LEDs
  if (ev.SlotID === 0) {
    elements.slot0Badge.classList.add("active");
    if (slot0Timeout) clearTimeout(slot0Timeout);
    slot0Timeout = setTimeout(() => elements.slot0Badge.classList.remove("active"), 350);
  } else if (ev.SlotID === 1) {
    elements.slot1Badge.classList.add("active");
    if (slot1Timeout) clearTimeout(slot1Timeout);
    slot1Timeout = setTimeout(() => elements.slot1Badge.classList.remove("active"), 350);
  } else if (ev.SlotID === 2) {
    if (elements.slot2Badge) {
      elements.slot2Badge.classList.add("active");
      if (slot2Timeout) clearTimeout(slot2Timeout);
      slot2Timeout = setTimeout(() => elements.slot2Badge.classList.remove("active"), 350);
    }
  }

  // Draw Waveform
  drawWaveform();

  // Append to Activity Log Terminal
  appendLog(ev);
}

function appendLog(ev) {
  if (elements.terminalEmpty) {
    elements.terminalEmpty.remove();
    elements.terminalEmpty = null;
  }

  const timeStr = new Date().toTimeString().split(" ")[0];
  const row = document.createElement("div");
  row.className = "log-row";
  row.innerHTML = `
    <span class="log-time">[${timeStr}]</span>
    <span class="log-task">${escapeHtml(ev.Task || "Request")}</span>
    <span class="log-stats">${ev.LatencyMs}ms (TTFT ${ev.TTFTMs || 0}ms, ${(ev.TokPerSec || 0).toFixed(0)}t/s)</span>
    <span class="log-summary">-> ${escapeHtml(ev.Summary || "")}</span>
  `;

  elements.logTerminal.appendChild(row);

  // Keep max 50 log rows
  while (elements.logTerminal.children.length > 50) {
    elements.logTerminal.removeChild(elements.logTerminal.firstChild);
  }

  // Auto scroll
  elements.logTerminal.scrollTop = elements.logTerminal.scrollHeight;
}

function escapeHtml(str) {
  return str.replace(/[&<>"']/g, m => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;"
  }[m]));
}

// ==========================================================================
// Status Badge Synchronization
// ==========================================================================
function updateStatus(status) {
  currentStatus = status;
  const t = i18n[currentLang] || i18n.ja;

  // Engine badge display
  if (elements.engineBadge) {
    if (status.running) {
      elements.engineBadge.classList.remove("hidden");
      elements.engineBadge.className = "engine-badge";
      const backend = (status.active_backend || "").toLowerCase();
      if (backend.includes("vulkan")) {
        elements.engineBadge.classList.add("vulkan");
        if (elements.engineIcon) elements.engineIcon.textContent = "⚡";
      } else if (backend.includes("cuda")) {
        elements.engineBadge.classList.add("cuda");
        if (elements.engineIcon) elements.engineIcon.textContent = "⚡";
      } else if (backend.includes("hip") || backend.includes("rocm")) {
        elements.engineBadge.classList.add("hip");
        if (elements.engineIcon) elements.engineIcon.textContent = "⚡";
      } else if (backend.includes("sycl") || backend.includes("intel")) {
        elements.engineBadge.classList.add("cuda");
        if (elements.engineIcon) elements.engineIcon.textContent = "⚡";
      } else {
        elements.engineBadge.classList.add("cpu");
        if (elements.engineIcon) elements.engineIcon.textContent = "⚠️";
      }

      if (status.cpu_fallback || backend === "cpu") {
        elements.engineText.textContent = currentLang === "en" ? "CPU Fallback (NGL=0)" : "CPU フォールバック (NGL=0)";
      } else {
        const dev = status.active_device ? ` (${status.active_device})` : "";
        elements.engineText.textContent = `GPU: ${status.active_backend || "Vulkan"}${dev}`;
      }
    } else {
      elements.engineBadge.classList.add("hidden");
    }
  }

  elements.statusPill.className = "status-pill";
  if (status.status === "fallback_cpu") {
    elements.statusPill.classList.add("starting");
    elements.statusLabel.textContent = t.statusFallbackCPU;
    elements.btnStartServer.disabled = true;
    elements.btnStopServer.disabled = false;
  } else if (status.starting) {
    elements.statusPill.classList.add("starting");
    elements.statusLabel.textContent = t.statusStarting;
    elements.btnStartServer.disabled = true;
    elements.btnStopServer.disabled = false;
  } else if (status.running) {
    if (status.cpu_fallback) {
      elements.statusPill.classList.add("cpu-fallback");
      elements.statusLabel.textContent = t.statusRunningCPU.replace("%d", status.jev_port || 8090);
      showToast(t.msgCPUFallbackToast);
    } else {
      elements.statusPill.classList.add("running");
      const backendTag = status.active_backend ? ` [${status.active_backend}]` : "";
      elements.statusLabel.textContent = (currentLang === "en" ? `RUNNING${backendTag}: Port ${status.jev_port || 8090}` : `稼働中${backendTag}: Port ${status.jev_port || 8090}`);
    }
    elements.btnStartServer.disabled = true;
    elements.btnStopServer.disabled = false;
    elements.btnOpenDemo.disabled = false;
    elements.btnTestStop.disabled = false;
    elements.btnTestExtract.disabled = false;
    elements.btnTestScan.disabled = false;
    if (elements.btnTestStress) elements.btnTestStress.disabled = false;
  } else {
    elements.statusLabel.textContent = t.statusStopped;
    elements.btnStartServer.disabled = false;
    elements.btnStopServer.disabled = true;
    elements.btnOpenDemo.disabled = true;
    elements.btnTestStop.disabled = true;
    elements.btnTestExtract.disabled = true;
    elements.btnTestScan.disabled = true;
    if (elements.btnTestStress) elements.btnTestStress.disabled = true;
    stopStress();

    if (status.last_error) {
      showToast(status.last_error, 5000);
      appendErrorLog(status.last_error);
    }
  }
}

// ==========================================================================
// Initialization & Wails Binding Connections
// ==========================================================================
window.addEventListener("DOMContentLoaded", async () => {
  drawWaveform();

  // Check if Wails runtime is injected
  if (window.go && window.go.main && window.go.main.App) {
    try {
      const state = await window.go.main.App.GetInitialState();
      if (state) {
        if (state.cfg) {
          elements.inputLlamaServer.value = state.cfg.llama_server || "";
          elements.inputModel.value = state.cfg.model || "";
          elements.inputJevPort.value = state.cfg.jev_port || 8090;
          elements.inputLlamaPort.value = state.cfg.llama_port || 8080;
          if (state.cfg.context) elements.selectContext.value = String(state.cfg.context);
          if (state.cfg.backend) {
            elements.selectBackend.value = state.cfg.backend;
          }
          if (state.cfg.lang) {
            updateLanguage(state.cfg.lang);
          }
        }
        if (state.hw) {
          currentHwInfo = state.hw;
          const hwDesc = (state.cfg && state.cfg.lang === "en") ? state.hw.summary_en : state.hw.summary;
          elements.hwText.textContent = hwDesc || "Hardware ready";
        }
        if (state.status) {
          currentStatus = state.status;
          updateStatus(state.status);
        }
      }
    } catch (err) {
      console.error("Failed to load initial state:", err);
    }

    // Subscribe to Wails Runtime Events
    if (window.runtime) {
      window.runtime.EventsOn("telemetry", onTelemetry);

      window.runtime.EventsOn("status-changed", (st) => {
        updateStatus(st);
      });

      window.runtime.EventsOn("download-progress", (p) => {
        elements.dlBanner.classList.remove("hidden");
        elements.dlLabel.textContent = p.label;
        elements.dlPercent.textContent = `${Math.round(p.pct)}% (${p.done_mb.toFixed(1)} / ${p.total_mb.toFixed(1)} MB)`;
        elements.dlProgressBar.style.width = `${Math.min(100, p.pct)}%`;

        if (p.complete) {
          setTimeout(() => elements.dlBanner.classList.add("hidden"), 3000);
          showToast(i18n[currentLang].msgDLDone);
          // Reload state to pick up new paths
          window.go.main.App.GetInitialState().then(st => {
            if (st && st.cfg) {
              elements.inputLlamaServer.value = st.cfg.llama_server || "";
              elements.inputModel.value = st.cfg.model || "";
            }
          });
        }
      });
    }
  }

  // Browse Buttons
  elements.btnBrowseLlama.addEventListener("click", async () => {
    if (window.go?.main?.App?.SelectLlamaServer) {
      const path = await window.go.main.App.SelectLlamaServer();
      if (path) elements.inputLlamaServer.value = path;
    }
  });

  elements.btnBrowseModel.addEventListener("click", async () => {
    if (window.go?.main?.App?.SelectModel) {
      const path = await window.go.main.App.SelectModel();
      if (path) elements.inputModel.value = path;
    }
  });

  // Auto Download
  elements.btnAutoDL.addEventListener("click", async () => {
    if (window.go?.main?.App?.StartAutoDownload) {
      elements.dlBanner.classList.remove("hidden");
      elements.dlLabel.textContent = "Connecting to repository...";
      elements.dlPercent.textContent = "0%";
      elements.dlProgressBar.style.width = "0%";
      try {
        await window.go.main.App.StartAutoDownload();
      } catch (err) {
        showToast("Download error: " + err);
      }
    }
  });

  // Backend Selector Change
  elements.selectBackend.addEventListener("change", async () => {
    const b = elements.selectBackend.value;
    if (window.go?.main?.App?.SetBackend) {
      const resolved = await window.go.main.App.SetBackend(b);
      elements.inputLlamaServer.value = resolved || "";
      if (!resolved) {
        showToast(currentLang === "en" ? "Binary not found locally. Click Auto Fetch to download." : "対象バイナリが未取得です。「自動取得」でダウンロードできます。");
      }
    }
  });

  // Start Server
  elements.btnStartServer.addEventListener("click", async () => {
    if (window.go?.main?.App?.StartServer) {
      const b = elements.selectBackend.value;
      const cfg = {
        llama_server: elements.inputLlamaServer.value.trim(),
        model: elements.inputModel.value.trim(),
        backend: b,
        context: parseInt(elements.selectContext.value, 10) || 8192,
        gpu_layers: b === "cpu" ? 0 : 99,
        jev_port: parseInt(elements.inputJevPort.value, 10) || 8090,
        llama_port: parseInt(elements.inputLlamaPort.value, 10) || 8080,
        host: "0.0.0.0",
        lang: currentLang
      };
      try {
        await window.go.main.App.StartServer(cfg);
      } catch (err) {
        showToast("" + err, 5000);
        appendErrorLog("" + err);
      }
    }
  });

  // Stop Server
  elements.btnStopServer.addEventListener("click", async () => {
    stopStress();
    if (window.go?.main?.App?.StopServer) {
      await window.go.main.App.StopServer();
    }
  });

  // Open Demo
  elements.btnOpenDemo.addEventListener("click", async () => {
    if (window.go?.main?.App?.OpenDemo) {
      await window.go.main.App.OpenDemo();
    }
  });

  // Copy URL
  elements.btnCopyURL.addEventListener("click", async () => {
    if (window.go?.main?.App?.CopyURL) {
      await window.go.main.App.CopyURL();
      showToast(i18n[currentLang].msgURLCopied);
    }
  });

  // Quick Tests
  elements.btnTestStop.addEventListener("click", async () => {
    if (window.go?.main?.App?.TriggerQuickTest) {
      await window.go.main.App.TriggerQuickTest("stop");
    }
  });
  elements.btnTestExtract.addEventListener("click", async () => {
    if (window.go?.main?.App?.TriggerQuickTest) {
      await window.go.main.App.TriggerQuickTest("extract");
    }
  });
  elements.btnTestScan.addEventListener("click", async () => {
    if (window.go?.main?.App?.TriggerQuickTest) {
      await window.go.main.App.TriggerQuickTest("scan");
    }
  });

  // Continuous Stress Test Loop (Slot 0 & Slot 1 Dual Worker with Async/Await)
  let stressRunning = false;
  let activeWorkers = 0;

  async function runStressWorker(workerId) {
    if (!stressRunning || !currentStatus?.running) return;
    activeWorkers++;
    try {
      while (stressRunning && currentStatus && currentStatus.running) {
        const tasks = ["stop", "extract", "scan"];
        const task = tasks[Math.floor(Math.random() * tasks.length)];
        if (window.go?.main?.App?.TriggerQuickTest) {
          try {
            await window.go.main.App.TriggerQuickTest(task);
          } catch (e) {
            if (!stressRunning || !currentStatus?.running) break;
          }
        }
        if (!stressRunning || !currentStatus?.running) break;
        // Pacing delay between sequential requests
        await new Promise(r => setTimeout(r, 40));
      }
    } finally {
      activeWorkers = Math.max(0, activeWorkers - 1);
      if (activeWorkers === 0) {
        finalizeStopStress();
      }
    }
  }

  function startStress() {
    if (!currentStatus || !currentStatus.running) return;
    if (stressRunning) return;
    stressRunning = true;
    if (elements.btnTestStress) {
      elements.btnTestStress.classList.add("active");
      elements.btnTestStress.textContent = currentLang === "en" ? "⏹ Stop Stress" : "⏹ 連打停止";
    }
    // Launch dual staggered workers for Slot 0 & Slot 1
    runStressWorker(0);
    setTimeout(() => {
      if (stressRunning && currentStatus?.running) {
        runStressWorker(1);
      }
    }, 45);
  }

  function stopStress() {
    stressRunning = false;
    finalizeStopStress();
  }

  function finalizeStopStress() {
    if (elements.btnTestStress) {
      elements.btnTestStress.classList.remove("active");
      const t = i18n[currentLang] || i18n.ja;
      elements.btnTestStress.textContent = t.btnTestStress || "⚡ 負荷連打";
    }
  }

  if (elements.btnTestStress) {
    elements.btnTestStress.addEventListener("click", () => {
      if (!stressRunning) {
        startStress();
      } else {
        stopStress();
      }
    });
  }

  // Language Toggle
  elements.btnLangToggle.addEventListener("click", async () => {
    const nextLang = currentLang === "ja" ? "en" : "ja";
    updateLanguage(nextLang);
    if (window.go?.main?.App?.SetLanguage) {
      await window.go.main.App.SetLanguage(nextLang);
    }
  });

  // Log Actions
  elements.btnLogCopy.addEventListener("click", () => {
    const rows = Array.from(elements.logTerminal.querySelectorAll(".log-row"));
    const text = rows.map(r => r.innerText).join("\n");
    if (text) {
      navigator.clipboard?.writeText(text);
      showToast(i18n[currentLang].msgLogCopied);
    }
  });

  elements.btnLogClear.addEventListener("click", () => {
    elements.logTerminal.innerHTML = `<div class="terminal-empty" id="terminalEmpty">No requests yet. Start server to monitor activity.</div>`;
    elements.terminalEmpty = document.getElementById("terminalEmpty");
    latencyHistory = [];
    drawWaveform();
    showToast(i18n[currentLang].msgLogCleared);
  });
});
