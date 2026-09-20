// ==========================================================================
// ReflexGate Frontend Application Logic (Wails / WebView2)
// ==========================================================================

const i18n = {
  ja: {
    title: "ReflexGate ランチャー",
    subtitle: "Qwen2.5-Coder-1.5B Fast AI Gateway",
    serverModelConfig: "サーバー & モデル設定",
    lblLlama: "llama-server.exe のパス",
    lblModel: "Qwen2.5-Coder-1.5B GGUF モデル",
    btnBrowse: "参照",
    btnAutoDL: "バイナリ & モデル自動取得",
    execSettings: "実行オプション & ポート",
    lblContext: "コンテキスト長",
    optCtx8k: "8,192 (推奨)",
    optCtx32k: "32,768 (最大)",
    lblBackend: "推論バックエンド",
    optBackendAuto: "自動検出 (推奨)",
    optBackendVulkan: "Vulkan (汎用 GPU / AMD / Intel)",
    optBackendCUDA: "CUDA (NVIDIA RTX / GeForce)",
    optBackendHIP: "ROCm / HIP (AMD Radeon)",
    optBackendSYCL: "Intel SYCL (Arc / Core Ultra)",
    optBackendCPU: "CPU のみ (NGL=0)",
    lblJevPort: "ReflexGate API ポート",
    lblLlamaPort: "llama-server ポート",
    btnStart: "ReflexGate 起動",
    btnStop: "停止",
    btnDemo: "Webデモを開く",
    btnCopyURL: "URLコピー",
    quickTest: "精度検証 & クイックテスト",
    btnTestStop: "Stop 判定",
    btnTestExtract: "ステータス抽出",
    btnTestScan: "エラー検知",
    btnRunSelfTest: "🧪 セルフテスト",
    btnSelfTestRunning: "🧪 実行中...",
    lblCtxUsage: "コンテキスト消費量 (直近リクエスト):",
    liveMonitor: "リアルタイム AI パフォーマンス監視",
    lblLatencyMeter: "応答レイテンシ・レベルメーター",
    kpiLatency: "応答レイテンシ",
    kpiTTFT: "TTFT (初回応答)",
    kpiSpeed: "生成速度",
    kpiCalls: "総コール数",
    recentLogs: "リアルタイム推論ログ",
    btnLogCopy: "コピー",
    btnLogClear: "クリア",
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
    msgCPUFallbackToast: "GPU初期化に失敗したため、CPUモードにフォールバックしました",
    modalSelfTestTitle: "セルフテスト精度検証",
    noFailures: "誤答はありません。すべてのジェネレーターが正常にパスしました。",
    btnRerun: "再テスト実行",
    btnClose: "閉じる"
  },
  en: {
    title: "ReflexGate Launcher",
    subtitle: "Qwen2.5-Coder-1.5B Fast AI Gateway",
    serverModelConfig: "Server & Model Configuration",
    lblLlama: "Path to llama-server.exe",
    lblModel: "Qwen2.5-Coder-1.5B GGUF Model",
    btnBrowse: "Browse",
    btnAutoDL: "Auto Fetch Binary & Model",
    execSettings: "Execution Options & Ports",
    lblContext: "Context Size",
    optCtx8k: "8,192 (Recommended)",
    optCtx32k: "32,768 (Maximum)",
    lblBackend: "Acceleration Backend",
    optBackendAuto: "Auto Detect (Recommended)",
    optBackendVulkan: "Vulkan (Generic GPU / AMD / Intel)",
    optBackendCUDA: "CUDA (NVIDIA RTX / GeForce)",
    optBackendHIP: "ROCm / HIP (AMD Radeon)",
    optBackendSYCL: "Intel SYCL (Arc / Core Ultra)",
    optBackendCPU: "CPU Only (NGL=0)",
    lblJevPort: "ReflexGate API Port",
    lblLlamaPort: "llama-server Port",
    btnStart: "Start ReflexGate",
    btnStop: "Stop",
    btnDemo: "Open Web Demo",
    btnCopyURL: "Copy URL",
    quickTest: "Accuracy & Quick Test Suite",
    btnTestStop: "Stop Check",
    btnTestExtract: "Status Extract",
    btnTestScan: "Error Scan",
    btnRunSelfTest: "🧪 Self-Test",
    btnSelfTestRunning: "🧪 Testing...",
    lblCtxUsage: "Context Usage (Recent Req):",
    liveMonitor: "Live AI Performance Monitor",
    lblLatencyMeter: "Response Latency Level Meter",
    kpiLatency: "Latency",
    kpiTTFT: "TTFT (First Token)",
    kpiSpeed: "Speed",
    kpiCalls: "Total Calls",
    recentLogs: "Real-time Telemetry Logs",
    btnLogCopy: "Copy",
    btnLogClear: "Clear",
    terminalEmpty: "No requests yet. Start server to monitor activity.",
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
    msgCPUFallbackToast: "GPU initialization failed; fell back to CPU mode",
    modalSelfTestTitle: "Self-Test Accuracy Verification",
    noFailures: "No failures. All test patterns passed successfully.",
    btnRerun: "Rerun Test",
    btnClose: "Close"
  }
};

let currentLang = "ja";
let currentHwInfo = null;
let currentStatus = { running: false, starting: false, jev_port: 8090 };
let latencyHistory = [];
let totalCallsCount = 0;
let isSelfTestRunning = false;

// Slot reset timers
let slotTimeouts = { 0: null, 1: null, 2: null };

// DOM Element References
const elements = {
  appVersion: document.getElementById("appVersion"),
  hdrSubtitle: document.getElementById("hdrSubtitle"),
  hwBadge: document.getElementById("hwBadge"),
  hwText: document.getElementById("hwText"),
  statusPill: document.getElementById("statusPill"),
  statusLabel: document.getElementById("statusLabel"),
  btnLangToggle: document.getElementById("btnLangToggle"),
  btnKofi: document.getElementById("btnKofi"),

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
  selectSelfTestMode: document.getElementById("selectSelfTestMode"),
  btnRunSelfTest: document.getElementById("btnRunSelfTest"),

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

  lblCtxUsage: document.getElementById("lblCtxUsage"),
  ctxUsageVal: document.getElementById("ctxUsageVal"),
  ctxWarnPill: document.getElementById("ctxWarnPill"),
  ctxWarnText: document.getElementById("ctxWarnText"),
  ctxGaugeBar: document.getElementById("ctxGaugeBar"),

  slotCard0: document.getElementById("slotCard0"),
  slot0Task: document.getElementById("slot0Task"),
  slot0State: document.getElementById("slot0State"),
  slot0Lat: document.getElementById("slot0Lat"),
  slot0Toks: document.getElementById("slot0Toks"),

  slotCard1: document.getElementById("slotCard1"),
  slot1Task: document.getElementById("slot1Task"),
  slot1State: document.getElementById("slot1State"),
  slot1Lat: document.getElementById("slot1Lat"),
  slot1Toks: document.getElementById("slot1Toks"),

  slotCard2: document.getElementById("slotCard2"),
  slot2Task: document.getElementById("slot2Task"),
  slot2State: document.getElementById("slot2State"),
  slot2Lat: document.getElementById("slot2Lat"),
  slot2Toks: document.getElementById("slot2Toks"),

  txtRecentLogs: document.getElementById("txtRecentLogs"),
  btnLogCopy: document.getElementById("btnLogCopy"),
  btnLogClear: document.getElementById("btnLogClear"),
  logTerminal: document.getElementById("logTerminal"),
  terminalEmpty: document.getElementById("terminalEmpty"),

  selfTestModal: document.getElementById("selfTestModal"),
  modalTitleText: document.getElementById("modalTitleText"),
  modalAccuracyBadge: document.getElementById("modalAccuracyBadge"),
  btnCloseSelfTest: document.getElementById("btnCloseSelfTest"),
  btnCloseSelfTestBtn: document.getElementById("btnCloseSelfTestBtn"),
  btnRerunSelfTest: document.getElementById("btnRerunSelfTest"),
  stOverallAcc: document.getElementById("stOverallAcc"),
  stCaseCount: document.getElementById("stCaseCount"),
  stAvgLatency: document.getElementById("stAvgLatency"),
  stAvgSpeed: document.getElementById("stAvgSpeed"),
  stTableBody: document.getElementById("stTableBody"),
  stFailCount: document.getElementById("stFailCount"),
  stFailureList: document.getElementById("stFailureList"),

  appToast: document.getElementById("appToast"),
  toastMessage: document.getElementById("toastMessage")
};

// Canvas 2D Context
const ctx = elements.waveformCanvas ? elements.waveformCanvas.getContext("2d") : null;

// ==========================================================================
// Dynamic Hardware & Backend Summary (Single Source of Truth)
// ==========================================================================
function updateHeaderHardwareSummary() {
  if (!elements.hwText) return;
  const b = elements.selectBackend ? elements.selectBackend.value : "auto";
  const lang = currentLang;
  const rawGpuName = currentHwInfo?.gpu_name || "";
  const summaryStr = lang === "en" ? (currentHwInfo?.summary_en || "") : (currentHwInfo?.summary || "");

  // If server is actively running, show active backend & active device
  if (currentStatus && currentStatus.running) {
    if (currentStatus.cpu_fallback) {
      elements.hwText.textContent = lang === "en" ? "CPU Fallback (NGL=0)" : "CPU フォールバック (NGL=0)";
      return;
    }
    const be = currentStatus.active_backend || "Vulkan";
    const dev = currentStatus.active_device ? ` (${currentStatus.active_device})` : "";
    elements.hwText.textContent = `${be}${dev}`;
    return;
  }

  // When stopped / configuring, synthesize from selected backend + detected hardware
  let text = "";
  if (b === "cpu") {
    text = lang === "en" ? "CPU Only (NGL=0)" : "CPU 推論モード (NGL=0)";
  } else if (b === "cuda") {
    text = rawGpuName ? `CUDA (${rawGpuName})` : "CUDA (NVIDIA RTX / GeForce)";
  } else if (b === "vulkan") {
    text = rawGpuName ? `Vulkan (${rawGpuName})` : "Vulkan (Generic GPU / AMD / Intel)";
  } else if (b === "hip") {
    text = rawGpuName ? `ROCm / HIP (${rawGpuName})` : "ROCm / HIP (AMD Radeon)";
  } else if (b === "sycl") {
    text = rawGpuName ? `Intel SYCL (${rawGpuName})` : "Intel SYCL (Arc / Core Ultra)";
  } else {
    // auto
    if (rawGpuName) {
      text = `${lang === "en" ? "Auto Detect" : "自動検出"} (${rawGpuName})`;
    } else if (summaryStr) {
      text = summaryStr;
    } else {
      text = lang === "en" ? "Hardware ready" : "ハードウェア準備完了";
    }
  }
  elements.hwText.textContent = text;
}

// ==========================================================================
// Language and UI Text Updates
// ==========================================================================
function updateLanguage(lang) {
  currentLang = lang;
  const t = i18n[lang] || i18n.ja;

  if (elements.hdrSubtitle) elements.hdrSubtitle.textContent = t.subtitle;
  if (elements.btnLangToggle) {
    elements.btnLangToggle.querySelector(".lang-text").textContent = (lang === "en" ? "JA" : "EN");
  }

  if (elements.txtServerModelConfig) elements.txtServerModelConfig.textContent = t.serverModelConfig;
  if (elements.lblLlamaServer) elements.lblLlamaServer.textContent = t.lblLlama;
  if (elements.btnBrowseLlama) elements.btnBrowseLlama.textContent = t.btnBrowse;
  if (elements.lblModel) elements.lblModel.textContent = t.lblModel;
  if (elements.btnBrowseModel) elements.btnBrowseModel.textContent = t.btnBrowse;
  if (elements.txtAutoDL) elements.txtAutoDL.textContent = t.btnAutoDL;

  if (elements.txtExecSettings) elements.txtExecSettings.textContent = t.execSettings;
  if (elements.lblContext) elements.lblContext.textContent = t.lblContext;
  if (elements.optCtx8k) elements.optCtx8k.textContent = t.optCtx8k;
  if (elements.optCtx32k) elements.optCtx32k.textContent = t.optCtx32k;
  if (elements.lblBackend) elements.lblBackend.textContent = t.lblBackend;
  if (elements.optBackendAuto) elements.optBackendAuto.textContent = t.optBackendAuto;
  if (elements.optBackendVulkan) elements.optBackendVulkan.textContent = t.optBackendVulkan;
  if (elements.optBackendCUDA) elements.optBackendCUDA.textContent = t.optBackendCUDA;
  if (elements.optBackendHIP) elements.optBackendHIP.textContent = t.optBackendHIP;
  if (elements.optBackendSYCL) elements.optBackendSYCL.textContent = t.optBackendSYCL;
  if (elements.optBackendCPU) elements.optBackendCPU.textContent = t.optBackendCPU;
  if (elements.lblJevPort) elements.lblJevPort.textContent = t.lblJevPort;
  if (elements.lblLlamaPort) elements.lblLlamaPort.textContent = t.lblLlamaPort;

  if (elements.txtStartServer) elements.txtStartServer.textContent = t.btnStart;
  if (elements.txtStopServer) elements.txtStopServer.textContent = t.btnStop;
  if (elements.txtOpenDemo) elements.txtOpenDemo.textContent = t.btnDemo;
  if (elements.txtCopyURL) elements.txtCopyURL.textContent = t.btnCopyURL;

  if (elements.txtQuickTest) elements.txtQuickTest.textContent = t.quickTest;
  if (elements.btnTestStop) elements.btnTestStop.textContent = t.btnTestStop;
  if (elements.btnTestExtract) elements.btnTestExtract.textContent = t.btnTestExtract;
  if (elements.btnTestScan) elements.btnTestScan.textContent = t.btnTestScan;
  if (elements.btnRunSelfTest && !isSelfTestRunning) {
    elements.btnRunSelfTest.textContent = t.btnRunSelfTest;
  }

  if (elements.lblCtxUsage) elements.lblCtxUsage.textContent = t.lblCtxUsage;
  if (elements.txtLiveMonitor) elements.txtLiveMonitor.textContent = t.liveMonitor;
  if (elements.lblLatencyMeter) elements.lblLatencyMeter.textContent = t.lblLatencyMeter;

  if (elements.kpiHdrLatency) elements.kpiHdrLatency.textContent = t.kpiLatency;
  if (elements.kpiHdrTTFT) elements.kpiHdrTTFT.textContent = t.kpiTTFT;
  if (elements.kpiHdrSpeed) elements.kpiHdrSpeed.textContent = t.kpiSpeed;
  if (elements.kpiHdrCalls) elements.kpiHdrCalls.textContent = t.kpiCalls;

  if (elements.txtRecentLogs) elements.txtRecentLogs.textContent = t.recentLogs;
  if (elements.btnLogCopy) elements.btnLogCopy.textContent = t.btnLogCopy;
  if (elements.btnLogClear) elements.btnLogClear.textContent = t.btnLogClear;

  if (elements.terminalEmpty) {
    elements.terminalEmpty.textContent = t.terminalEmpty;
  }

  if (elements.modalTitleText) elements.modalTitleText.textContent = t.modalSelfTestTitle;
  if (elements.btnRerunSelfTest) elements.btnRerunSelfTest.textContent = t.btnRerun;
  if (elements.btnCloseSelfTestBtn) elements.btnCloseSelfTestBtn.textContent = t.btnClose;

  updateHeaderHardwareSummary();

  if (currentStatus) {
    updateStatus(currentStatus);
  }
}

// ==========================================================================
// Toast Notification
// ==========================================================================
let toastTimer = null;
function showToast(msg, duration = 3000) {
  if (!elements.toastMessage || !elements.appToast) return;
  elements.toastMessage.textContent = msg;
  elements.appToast.classList.remove("hidden");
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    elements.appToast.classList.add("hidden");
  }, duration);
}

function appendErrorLog(msg) {
  if (!elements.logTerminal) return;
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
  if (!ctx || !elements.waveformCanvas) return;
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
  ctx.shadowBlur = 0;

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
// Slot Activity Update Helper
// ==========================================================================
function updateSlotActivity(slotId, task, latMs, tokPerSec) {
  const slotNum = slotId >= 0 && slotId <= 2 ? slotId : 0;
  const card = elements[`slotCard${slotNum}`];
  const taskPill = elements[`slot${slotNum}Task`];
  const stateBadge = elements[`slot${slotNum}State`];
  const latEl = elements[`slot${slotNum}Lat`];
  const toksEl = elements[`slot${slotNum}Toks`];
  const headerBadge = elements[`slot${slotNum}Badge`];

  if (taskPill) taskPill.textContent = (task || "REQ").toUpperCase();
  if (latEl) latEl.textContent = `${latMs}ms`;
  if (toksEl) toksEl.textContent = `${(tokPerSec || 0).toFixed(1)} t/s`;

  if (card) card.classList.add("active");
  if (stateBadge) {
    stateBadge.className = "slot-state-badge active";
    stateBadge.textContent = "ACTIVE";
  }
  if (headerBadge) headerBadge.classList.add("active");

  if (slotTimeouts[slotNum]) clearTimeout(slotTimeouts[slotNum]);
  slotTimeouts[slotNum] = setTimeout(() => {
    if (card) card.classList.remove("active");
    if (stateBadge) {
      stateBadge.className = "slot-state-badge idle";
      stateBadge.textContent = "IDLE";
    }
    if (headerBadge) headerBadge.classList.remove("active");
  }, 450);
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
  if (elements.kpiValLatency) elements.kpiValLatency.innerHTML = `${ev.LatencyMs} <span class="unit">ms</span>`;
  if (elements.kpiValTTFT) elements.kpiValTTFT.innerHTML = `${ev.TTFTMs || 0} <span class="unit">ms</span>`;
  if (elements.kpiValSpeed) elements.kpiValSpeed.innerHTML = `${(ev.TokPerSec || 0).toFixed(1)} <span class="unit">t/s</span>`;
  if (elements.kpiValCalls) elements.kpiValCalls.innerHTML = `${totalCallsCount} <span class="unit">reqs</span>`;

  // Update Audio-VU Latency Meter Bar
  if (elements.meterLatencyText) elements.meterLatencyText.textContent = `${ev.LatencyMs} ms`;
  if (elements.meterFill) {
    let pct = (ev.LatencyMs / 500.0) * 100.0;
    pct = Math.min(100.0, Math.max(4.0, pct));
    elements.meterFill.style.width = `${pct}%`;
  }

  // Update Real-time Context Usage Gauge
  const promptToks = ev.PromptToks || 0;
  const outToks = ev.OutToks || 0;
  const reqTokens = promptToks + outToks;
  const maxCtx = currentStatus?.effective_context || currentStatus?.configured_context || parseInt(elements.selectContext?.value, 10) || 8192;
  const ctxPct = Math.min(100, Math.round((reqTokens / maxCtx) * 100));

  if (elements.ctxUsageVal) {
    elements.ctxUsageVal.textContent = `${reqTokens.toLocaleString()} / ${maxCtx.toLocaleString()} (${ctxPct}%)`;
  }
  if (elements.ctxGaugeBar) {
    elements.ctxGaugeBar.style.width = `${ctxPct}%`;
  }

  // Update Slot Monitoring
  updateSlotActivity(ev.SlotID, ev.Task, ev.LatencyMs, ev.TokPerSec);

  // Draw Waveform
  drawWaveform();

  // Append to Activity Log Terminal
  appendLog(ev);
}

function appendLog(ev) {
  if (!elements.logTerminal) return;
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

  while (elements.logTerminal.children.length > 50) {
    elements.logTerminal.removeChild(elements.logTerminal.firstChild);
  }
  elements.logTerminal.scrollTop = elements.logTerminal.scrollHeight;
}

function escapeHtml(str) {
  if (!str) return "";
  return String(str).replace(/[&<>"']/g, m => ({
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

  // Context mismatch warning pill
  if (elements.ctxWarnPill) {
    if (status.context_warning) {
      elements.ctxWarnPill.classList.remove("hidden");
      if (elements.ctxWarnText) elements.ctxWarnText.textContent = status.context_warning;
    } else {
      elements.ctxWarnPill.classList.add("hidden");
    }
  }

  // Update Header text
  updateHeaderHardwareSummary();

  // Status pill & Action buttons
  if (elements.statusPill && elements.statusLabel) {
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
      if (elements.btnRunSelfTest) elements.btnRunSelfTest.disabled = false;
    } else {
      elements.statusLabel.textContent = t.statusStopped;
      elements.btnStartServer.disabled = false;
      elements.btnStopServer.disabled = true;
      elements.btnOpenDemo.disabled = true;
      elements.btnTestStop.disabled = true;
      elements.btnTestExtract.disabled = true;
      elements.btnTestScan.disabled = true;
      if (elements.btnRunSelfTest) elements.btnRunSelfTest.disabled = true;

      if (status.last_error) {
        showToast(status.last_error, 5000);
        appendErrorLog(status.last_error);
      }
    }
  }
}

// ==========================================================================
// Self-Test Execution & Modal Management
// ==========================================================================
async function runSelfTest() {
  if (isSelfTestRunning || !currentStatus?.running) return;
  if (!window.go?.main?.App?.RunSelfTest) return;

  const mode = elements.selectSelfTestMode ? elements.selectSelfTestMode.value : "quick";
  const t = i18n[currentLang] || i18n.ja;

  isSelfTestRunning = true;
  if (elements.btnRunSelfTest) {
    elements.btnRunSelfTest.disabled = true;
    elements.btnRunSelfTest.textContent = t.btnSelfTestRunning;
  }

  try {
    const res = await window.go.main.App.RunSelfTest(mode);
    renderSelfTestResults(res);
  } catch (err) {
    showToast("Self-test error: " + err, 5000);
    appendErrorLog("Self-test error: " + err);
  } finally {
    isSelfTestRunning = false;
    if (elements.btnRunSelfTest) {
      elements.btnRunSelfTest.disabled = !currentStatus?.running;
      elements.btnRunSelfTest.textContent = t.btnRunSelfTest;
    }
  }
}

function renderSelfTestResults(res) {
  if (!res || !elements.selfTestModal) return;

  const t = i18n[currentLang] || i18n.ja;
  const accPct = (res.accuracy * 100).toFixed(1);

  // Overall accuracy badge
  if (elements.modalAccuracyBadge) {
    if (res.accuracy >= 1.0) {
      elements.modalAccuracyBadge.className = "modal-badge pass";
      elements.modalAccuracyBadge.textContent = "100% PASS";
    } else {
      elements.modalAccuracyBadge.className = "modal-badge fail";
      elements.modalAccuracyBadge.textContent = `${accPct}% FAIL`;
    }
  }

  // Overview metrics
  if (elements.stOverallAcc) {
    elements.stOverallAcc.textContent = `${accPct}%`;
    elements.stOverallAcc.className = res.accuracy >= 1.0 ? "summary-val green" : "summary-val red";
  }
  if (elements.stCaseCount) {
    elements.stCaseCount.textContent = `${res.passed_count} / ${res.total_count}`;
  }
  if (elements.stAvgLatency) {
    elements.stAvgLatency.textContent = `${Math.round(res.avg_latency_ms)} ms`;
  }
  if (elements.stAvgSpeed) {
    elements.stAvgSpeed.textContent = `${(res.avg_tok_s || 0).toFixed(1)} t/s`;
  }

  // Breakdown table
  if (elements.stTableBody && res.generator_stats) {
    const genNames = Object.keys(res.generator_stats).sort();
    let rowsHtml = "";
    genNames.forEach(name => {
      const st = res.generator_stats[name];
      const genAcc = (st.accuracy * 100).toFixed(0);
      const isPass = st.accuracy >= 1.0;
      let taskType = "stop";
      if (name.includes("partial") || name.includes("code") || name.includes("null")) {
        taskType = "extract";
      } else if (name.includes("crash") || name.includes("injection") || name.includes("leak") || name.includes("harmless") || name.includes("minor")) {
        taskType = "scan";
      }

      rowsHtml += `
        <tr>
          <td><code>${escapeHtml(name)}</code></td>
          <td><span class="slot-task-pill">${taskType.toUpperCase()}</span></td>
          <td>${st.passed} / ${st.total}</td>
          <td><span class="badge-acc ${isPass ? "pass" : "fail"}">${genAcc}%</span></td>
        </tr>
      `;
    });
    elements.stTableBody.innerHTML = rowsHtml;
  }

  // Failure Inspector list
  if (elements.stFailCount) {
    elements.stFailCount.textContent = res.failures ? res.failures.length : 0;
  }
  if (elements.stFailureList) {
    if (!res.failures || res.failures.length === 0) {
      elements.stFailureList.innerHTML = `<div class="no-failures">${t.noFailures}</div>`;
    } else {
      let failHtml = "";
      res.failures.forEach(f => {
        failHtml += `
          <div class="failure-item">
            <div class="failure-item-header">
              <span>[${escapeHtml(f.task.toUpperCase())}] ${escapeHtml(f.generator)}</span>
              <span>ID: ${escapeHtml(f.id)}</span>
            </div>
            <div class="failure-detail-row">
              <span class="key">Expected:</span>
              <span class="val" style="color:var(--neon-green);">${escapeHtml(f.expected)}</span>
            </div>
            <div class="failure-detail-row">
              <span class="key">Actual:</span>
              <span class="val" style="color:#F87171;">${escapeHtml(f.actual)}</span>
            </div>
            ${f.reason ? `
              <div class="failure-detail-row">
                <span class="key">Reason:</span>
                <span class="val">${escapeHtml(f.reason)}</span>
              </div>
            ` : ""}
            <div class="failure-log-pre">${escapeHtml(f.log)}</div>
          </div>
        `;
      });
      elements.stFailureList.innerHTML = failHtml;
    }
  }

  // Show modal
  elements.selfTestModal.classList.remove("hidden");
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
        if (state.version && elements.appVersion) {
          elements.appVersion.textContent = state.version;
        }
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
          updateHeaderHardwareSummary();
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

      window.runtime.EventsOn("selftest-progress", (p) => {
        if (elements.btnRunSelfTest && isSelfTestRunning) {
          elements.btnRunSelfTest.textContent = `🧪 ${p.current}/${p.total} (${(p.pct * 100).toFixed(0)}%)`;
        }
      });

      window.runtime.EventsOn("download-progress", (p) => {
        elements.dlBanner.classList.remove("hidden");
        elements.dlLabel.textContent = p.label;
        elements.dlPercent.textContent = `${Math.round(p.pct)}% (${p.done_mb.toFixed(1)} / ${p.total_mb.toFixed(1)} MB)`;
        elements.dlProgressBar.style.width = `${Math.min(100, p.pct)}%`;

        if (p.complete) {
          setTimeout(() => elements.dlBanner.classList.add("hidden"), 3000);
          showToast(i18n[currentLang].msgDLDone);
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
    updateHeaderHardwareSummary();
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

  // Self-Test Actions
  if (elements.btnRunSelfTest) {
    elements.btnRunSelfTest.addEventListener("click", () => {
      runSelfTest();
    });
  }

  if (elements.btnCloseSelfTest) {
    elements.btnCloseSelfTest.addEventListener("click", () => {
      elements.selfTestModal.classList.add("hidden");
    });
  }
  if (elements.btnCloseSelfTestBtn) {
    elements.btnCloseSelfTestBtn.addEventListener("click", () => {
      elements.selfTestModal.classList.add("hidden");
    });
  }
  if (elements.btnRerunSelfTest) {
    elements.btnRerunSelfTest.addEventListener("click", () => {
      elements.selfTestModal.classList.add("hidden");
      runSelfTest();
    });
  }
  if (elements.selfTestModal) {
    elements.selfTestModal.addEventListener("click", (e) => {
      if (e.target === elements.selfTestModal) {
        elements.selfTestModal.classList.add("hidden");
      }
    });
  }
  window.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && elements.selfTestModal && !elements.selfTestModal.classList.contains("hidden")) {
      elements.selfTestModal.classList.add("hidden");
    }
  });

  // Ko-fi Support Link
  if (elements.btnKofi) {
    elements.btnKofi.addEventListener("click", (e) => {
      e.preventDefault();
      if (window.runtime?.BrowserOpenURL) {
        window.runtime.BrowserOpenURL("https://ko-fi.com/fallout_tokyo");
      } else {
        window.open("https://ko-fi.com/fallout_tokyo", "_blank");
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
