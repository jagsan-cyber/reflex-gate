//go:build windows

package gui

type I18nText struct {
	WindowTitle    string
	HeaderTitle    string
	GroupServer    string
	LblLlama       string
	LblModel       string
	BtnBrowse      string
	BtnAutoDL      string
	GroupExec      string
	LblContext     string
	LblGPU         string
	OptGPUAll      string
	OptGPUCPU      string
	LblJevPort     string
	LblLlamaPort   string
	BtnStart       string
	BtnStop        string
	BtnDemo        string
	BtnCopyURL     string
	LblStatusStop  string
	LblStatusStart string
	LblStatusRun   string
	LblStatusFail  string
	LblQuickTest   string
	BtnTestStop    string
	BtnTestExtract string
	BtnTestScan    string
	GroupMonitor   string
	LblLatency     string
	LblTTFT        string
	LblSpeed       string
	LblCalls       string
	MeterLabel     string
	Slot0Idle      string
	Slot0Active    string
	Slot1Idle      string
	Slot1Active    string
	LblRecentLogs  string
	BtnLogCopy     string
	BtnLogClear    string
	BtnLangToggle  string
	CallsFormat    string
	MsgURLCopied   string
	MsgLogCopied   string
	MsgCleared     string
	MsgDLDone      string
	MsgDLFail      string
	TrayOpen       string
	TrayStop       string
	TrayExit       string
}

var textJA = I18nText{
	WindowTitle:    "ReflexGate ランチャー (Qwen2.5-Coder-1.5B 高速AIゲートウェイ)",
	HeaderTitle:    "⚡ ReflexGate AI Gateway (Qwen2.5-Coder-1.5B)",
	GroupServer:    " サーバーとモデルの設定 ",
	LblLlama:       "llama-server.exe のパス:",
	LblModel:       "Qwen2.5-Coder-1.5B GGUF モデルのパス:",
	BtnBrowse:      "参照…",
	BtnAutoDL:      "📥 バイナリ＆モデルを自動取得",
	GroupExec:      " 実行設定 / 制御 ",
	LblContext:     "コンテキスト:",
	LblGPU:         "GPU:",
	OptGPUAll:      "全レイヤー (GPU)",
	OptGPUCPU:      "CPU のみ",
	LblJevPort:     "JEVポート:",
	LblLlamaPort:   "llama:",
	BtnStart:       "▶ ReflexGate 起動",
	BtnStop:        "■ 停止",
	BtnDemo:        "🌐 デモ画面",
	BtnCopyURL:     "📋 URLコピー",
	LblStatusStop:  "ステータス: 停止中",
	LblStatusStart: "llama-server 起動中…",
	LblStatusRun:   "稼働中（Port %d）",
	LblStatusFail:  "起動失敗",
	LblQuickTest:   "テスト発火:",
	BtnTestStop:    "⚡ Stop (120ms)",
	BtnTestExtract: "⚡ ステータス抽出",
	BtnTestScan:    "⚡ Scan (0ms)",
	GroupMonitor:   " ⚡ リアルタイム稼働モニター (Live AI Metrics) ",
	LblLatency:     "直近レイテンシ",
	LblTTFT:        "首尾ラグ (TTFT)",
	LblSpeed:       "生成速度",
	LblCalls:       "累計処理数",
	MeterLabel:     "直近レイテンシ:",
	Slot0Idle:      "Slot 0 [○ STOP/Sys1] IDLE",
	Slot0Active:     "Slot 0 [● STOP/Sys1] ACTIVE ⚡",
	Slot1Idle:      "Slot 1 [○ EXTRACT] IDLE",
	Slot1Active:     "Slot 1 [● EXTRACT] ACTIVE ⚡",
	LblRecentLogs:  "直近の処理ログ (最新 50 件):",
	BtnLogCopy:     "📋 ログコピー",
	BtnLogClear:    "🗑 クリア",
	BtnLangToggle:  "🌐 English",
	CallsFormat:    "%d 回",
	MsgURLCopied:   "URLをクリップボードにコピーしました！",
	MsgLogCopied:   "ログをクリップボードにコピーしました！",
	MsgCleared:     "ログとグラフをクリアしました。",
	MsgDLDone:      "バイナリとモデルの取得が完了しました！\n「▶ ReflexGate 起動」を押して起動できます。",
	MsgDLFail:      "ダウンロードに失敗しました。",
	TrayOpen:       "表示 (Open)",
	TrayStop:       "■ 停止 (Stop)",
	TrayExit:       "終了 (Exit)",
}

var textEN = I18nText{
	WindowTitle:    "ReflexGate Launcher (Qwen2.5-Coder-1.5B Fast AI Gateway)",
	HeaderTitle:    "⚡ ReflexGate AI Gateway (Qwen2.5-Coder-1.5B)",
	GroupServer:    " Server && Model Configuration ",
	LblLlama:       "llama-server.exe path:",
	LblModel:       "Qwen2.5-Coder-1.5B GGUF Model path:",
	BtnBrowse:      "Browse...",
	BtnAutoDL:      "📥 Auto Fetch Binary && Model",
	GroupExec:      " Execution && Controls ",
	LblContext:     "Context:",
	LblGPU:         "GPU:",
	OptGPUAll:      "All Layers (GPU)",
	OptGPUCPU:      "CPU Only",
	LblJevPort:     "JEV Port:",
	LblLlamaPort:   "llama:",
	BtnStart:       "▶ Start ReflexGate",
	BtnStop:        "■ Stop",
	BtnDemo:        "🌐 Web Demo",
	BtnCopyURL:     "📋 Copy URL",
	LblStatusStop:  "Status: Stopped",
	LblStatusStart: "Starting llama-server...",
	LblStatusRun:   "Running (Port %d)",
	LblStatusFail:  "Launch Failed",
	LblQuickTest:   "Quick Fire:",
	BtnTestStop:    "⚡ Stop (120ms)",
	BtnTestExtract: "⚡ Status Extract",
	BtnTestScan:    "⚡ Scan (0ms)",
	GroupMonitor:   " ⚡ Live AI Performance Monitor && Waveform ",
	LblLatency:     "Recent Latency",
	LblTTFT:        "First Token (TTFT)",
	LblSpeed:       "Speed",
	LblCalls:       "Total Requests",
	MeterLabel:     "Recent Latency:",
	Slot0Idle:      "Slot 0 [○ STOP/Sys1] IDLE",
	Slot0Active:     "Slot 0 [● STOP/Sys1] ACTIVE ⚡",
	Slot1Idle:      "Slot 1 [○ EXTRACT] IDLE",
	Slot1Active:     "Slot 1 [● EXTRACT] ACTIVE ⚡",
	LblRecentLogs:  "Recent Request Logs (Latest 50):",
	BtnLogCopy:     "📋 Copy Log",
	BtnLogClear:    "🗑 Clear",
	BtnLangToggle:  "🌐 日本語",
	CallsFormat:    "%d calls",
	MsgURLCopied:   "URL copied to clipboard!",
	MsgLogCopied:   "Logs copied to clipboard!",
	MsgCleared:     "Logs and waveform cleared.",
	MsgDLDone:      "Binary & model downloaded successfully!\nClick '▶ Start ReflexGate' to run.",
	MsgDLFail:      "Download failed.",
	TrayOpen:       "Open",
	TrayStop:       "■ Stop",
	TrayExit:       "Exit",
}

func getI18n(lang string) I18nText {
	if lang == "en" {
		return textEN
	}
	return textJA
}
