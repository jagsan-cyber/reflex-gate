# ReflexGate

<div align="center">

**Ultra-Fast, Resilient Local AI Gateway for Autonomous Agent Loops**

[![Ko-fi](https://img.shields.io/badge/Ko--fi-Support%20ReflexGate-FF5E5B?style=for-the-badge&logo=ko-fi&logoColor=white)](https://ko-fi.com/fallout_tokyo)
![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Wails v2](https://img.shields.io/badge/Wails-v2.16-DF0000?style=for-the-badge)
![Platform](https://img.shields.io/badge/Platform-Windows%20x64-0078D6?style=for-the-badge&logo=windows&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-00FF88.svg?style=for-the-badge)

</div>

---

## ☕ Support the Project / 開発支援・寄付

ReflexGate is an open-source project created to drastically accelerate and stabilize local AI agent execution loops with zero token costs and minimal latency.

If ReflexGate helped speed up your local agent workflows, saved your API token expenses, or streamlined your LLM pipeline, please consider buying a coffee to support continued development and maintenance!

**👉 Support on Ko-fi: [https://ko-fi.com/fallout_tokyo](https://ko-fi.com/fallout_tokyo)**

> **開発支援のお願い**:
> ReflexGate は、ローカル環境で自律エージェントのループ制御（判定・抽出・セキュリティ検知）を極小レイテンシ＆ゼロトークンコストで安定稼働させるためのオープンソースプロジェクトです。
> 今後の継続的な機能強化やローカルLLM最適化のため、Ko-fi での温かいご支援をいただけると大変励みになります！

---

## ⚡ Overview

When autonomous coding agents run in iterative loops, evaluating stop conditions and extracting error states via large cloud models introduces high latency (2-5s+) and unnecessary token costs.

**ReflexGate** wraps a compact local model (`Qwen2.5-Coder-1.5B-Instruct` on `llama.cpp`) into a dedicated, low-latency (sub-second) local gateway server (`http://127.0.0.1:8090`). It provides a modern Windows Desktop GUI launcher, real-time audio-VU performance telemetry, and 3 high-precision endpoints with 100.0% benchmark accuracy.

---

## 🚀 Instant Quick Start (Prebuilt Binary)

Download the ready-to-run Windows executable from **[GitHub Releases](https://github.com/jagsan-cyber/reflex-gate/releases/latest)** (`reflexgate.exe`).

1. Download **`reflexgate.exe`** from the [Latest Release](https://github.com/jagsan-cyber/reflex-gate/releases/latest).
2. Double-click **`reflexgate.exe`** to launch the GUI.
3. Click **「バイナリ & モデル自動取得」 (Auto Fetch)**:
   - Downloads `qwen2.5-coder-1.5b-instruct-q8_0.gguf` (persisted in `models/`).
   - Downloads the matched `llama-server.exe` for your hardware (Vulkan / CUDA / ROCm / CPU).
4. Click **「ReflexGate 起動」 (Start Server)**.
5. Your local gateway is live at `http://127.0.0.1:8090`!

---

## 🌟 Key Features

- **Modern Cyberpunk Dark Desktop GUI**: Built with Wails v2 + WebView2 + Go.
- **3 Core Agent Gate Endpoints**:
  - `POST /jev/stop` -> Intelligent loop-exit gating with 1-line Chain-of-Thought (CoT) + GBNF grammar.
  - `POST /jev/extract` -> Tool-argument & status/error-code JSON extraction with strict JSON Schema.
  - `POST /jev/scan` -> Hybrid safety shield (<1ms regex secret detection + semantic crash & prompt injection scanner).
- **Built-in Self-Test Suite & Failure Inspector**:
  - Validates 13 production trap patterns (fake success, retry recovery, secret leaks, subtle crashes, etc.).
  - Select between Quick (15 cases) and Thorough (100 cases) modes with a graphical failure inspector modal.
- **Real-Time AI Performance Telemetry**:
  - Live Audio-VU latency level meter & Oscilloscope waveform canvas.
  - Real-time Context Usage Gauge (`tokens / context_size (%)`) with automatic model-metadata context clip warning.
  - 3-Slot parallel monitoring (`SLOT 0`, `SLOT 1`, `SLOT 2`) with active latency and tok/s indicators.
- **Zero Orphan Process Guarantee (Windows Job Object)**:
  - `llama-server.exe` is bound to a Windows Kernel Job Object (`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`).
  - When ReflexGate exits, is closed, or killed in Task Manager, Windows kernel automatically and instantly terminates `llama-server.exe`.
- **System Tray Residency**:
  - Minimizes to the Windows system notification tray.
  - Context menu for Show/Hide, Start/Stop server, and Exit.
- **Permanent & Resilient Downloads**:
  - Once fetched, assets are permanently kept locally for offline use.
  - Automatic fallback to immutable GitHub release CDN assets (`b11059`) prevents GitHub API rate limit (HTTP 403) failures.

---

## 📡 API Reference

All requests accept a single JSON payload: `{"log": "<string>"}`.

### 1. Loop Exit Gating (`POST /jev/stop`)
```bash
curl -s http://127.0.0.1:8090/jev/stop \
  -H "Content-Type: application/json" \
  -d '{"log":"pytest: 45 passed in 1.2s\nstatus: ALL_DONE"}'
```
Response:
```json
{
  "stop": true,
  "reason": "All 45 tests passed successfully and status is ALL_DONE."
}
```

### 2. Status & Error Extraction (`POST /jev/extract`)
```bash
curl -s http://127.0.0.1:8090/jev/extract \
  -H "Content-Type: application/json" \
  -d '{"log":"Build failed: E0382 use of moved value `x`"}'
```
Response:
```json
{
  "result": {
    "status": "fail",
    "error_code": "E0382",
    "summary": "use of moved value `x`"
  }
}
```

### 3. Safety & Crash Scanning (`POST /jev/scan`)
```bash
curl -s http://127.0.0.1:8090/jev/scan \
  -H "Content-Type: application/json" \
  -d '{"log":"Connecting with sk-proj-abc1234567890..."}'
```
Response:
```json
{
  "severity": "secret_leak",
  "reason": "Plaintext API key leaked in stdout log."
}
```

---

## 🛠️ Building from Source

### Prerequisites
- Go 1.23+
- Wails CLI v2 (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- Node.js (optional, frontend is vanilla HTML/CSS/JS)

### Build Command
```bash
wails build
```
The resulting standalone executable will be generated at `build/bin/reflexgate.exe`.

---

## 📊 Benchmark Verification

Tested with production-grade adversarial trap logs generated across 13 distinct generator categories:

| Task | Test Category | Target Metric | Score |
|---|---|---|---|
| **Stop** | Fake success, retry recovery, infinite loop, partial progress | Accuracy | **100.0%** |
| **Extract** | Ambiguous status, error code preservation, null handling | Accuracy | **100.0%** |
| **Scan** | Secret leakage, silent crash, prompt injection, harmless error words | Accuracy | **100.0%** |

---

## ⚡ Performance

Measured over 120 requests (40 per endpoint) using the built-in Self-Test Suite,
adversarial trap patterns included.

| Metric | `/jev/stop` | `/jev/extract` | `/jev/scan` |
|---|---:|---:|---:|
| Avg latency | 0.35s | 0.43s | 0.29s |
| Median latency | 0.30s | 0.41s | 0.29s |
| Max latency | 1.70s | 1.86s | 1.72s |

| Metric | Value |
|---|---|
| Decode speed (avg) | ~105 tok/s (76-140 tok/s range) |
| TTFT (median) | ~25ms |
| TTFT (worst-case, cold slot) | ~1.4s |
| Avg completion length | ~30 tokens |

> **Test environment:** AMD Radeon RX 9070 via Vulkan / ROCm backend,
> `qwen2.5-coder-1.5b-instruct-q8_0.gguf`, context length 8,192.
> Latency and throughput scale with your GPU/backend - results on CPU-only
> or lower-end integrated GPUs will be slower. Run the built-in Self-Test
> Suite on your own hardware to get numbers specific to your setup.

**Why this matters:** at sub-second latency and zero per-call token cost,
gating an agent loop through ReflexGate on every iteration adds negligible
overhead compared to a single cloud LLM call.

---

## 💾 Resource Requirements

| Resource | Requirement |
|---|---|
| VRAM (GPU backends) | ~2GB at max context (8,192 tokens) |
| RAM (CPU fallback) | ~4GB |
| Disk (model + binary) | ~2GB (downloaded on first run, kept for offline use) |
| OS | Windows 10/11 (x64) |

---

## 🔒 Privacy & Offline Guarantee

- After the initial model/binary download, ReflexGate makes **no outbound
  network requests**. All log content sent to `/jev/stop`, `/jev/extract`,
  and `/jev/scan` is processed entirely on your machine.
- This matters especially for `/jev/scan`, which is designed to inspect logs
  that may contain secrets or sensitive data - nothing leaves your device.

---

## ⚠️ Known Limitations

- Windows-only (Wails v2 + WebView2). No macOS/Linux build yet.
- Benchmarked primarily on AMD Radeon RX 9070 (Vulkan / ROCm); other
  backend/GPU combinations are supported but less thoroughly verified.
- Effective context length may be capped below the configured value
  depending on model metadata - the GUI's context usage gauge will warn
  you if this happens.

---

## 📄 License & Disclaimers

### License
ReflexGate is open-source software licensed under the [MIT License](LICENSE).  
Copyright (c) 2026 ReflexGate Contributors.

### Third-Party Licenses & Attribution
ReflexGate coordinates and runs local third-party models and inference engines:
- **llama.cpp**: Licensed under the MIT License. Copyright (c) 2023-2026 Georgi Gerganov and contributors.
- **Qwen2.5-Coder**: Developed by Alibaba Cloud / Qwen Team. Licensed under the Apache License 2.0.
- Additional dependencies and full legal notices are listed in [THIRD_PARTY_LICENSES.txt](THIRD_PARTY_LICENSES.txt).

### Trademark Disclaimer
- "Qwen" is a trademark of Alibaba Cloud.
- "llama.cpp" is maintained by Georgi Gerganov and the llama.cpp open-source community.
- "Jev" and "JEV" may be trademarks of TypeSafe AI or their respective owners.
- ReflexGate is an independent, unofficial open-source project and is not affiliated with, endorsed by, or sponsored by TypeSafe AI.
- Any use of "jev" in API paths (for example `/jev/stop`, `/jev/extract`, `/jev/scan`) or documentation is for interoperability / compatibility description only and does not imply any official relationship.
- All other trademarks, service marks, and company names are the property of their respective owners.
- ReflexGate is not affiliated with, endorsed by, or sponsored by Alibaba Cloud or the llama.cpp maintainers.

---

<div align="center">

**Created by [fallout_tokyo](https://ko-fi.com/fallout_tokyo)**  
*Contributions, issues, and feature requests are welcome!*

</div>
