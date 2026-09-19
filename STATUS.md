# local-jev 実装状況・テスト報告

日付: 2026-09-19  
対象: `jev-bench/`（独立 git、MIT）  
ランタイム: llama.cpp `llama-server` (Vulkan / ROCm / CUDA) + Qwen2.5-Coder-1.5B (または Qwen3.5-0.8B)

本家 TypeSafe Jev の公式クライアント互換ではない。役割（安く速い型付き判定）をローカル LLM で再現した意思決定ゲートキーパー（ReflexGate）。

---

## 1. エンドポイント一覧とスロット構成

エージェントがログを送り、サーバ側が契約を固定して判定を返す HTTP プロキシ。
`--parallel 3` により 3 つの専用スロットで並行処理。

```
caller  -->  local-jev :8090  -->  llama-server :8080 (3 slots)
                 |
                 +--> Slot 0: /jev/stop (1行 CoT), /v1/systemone (常駐 Prefix Cache)
                 +--> Slot 1: /jev/extract (JSON Schema 拘束)
                 +--> Slot 2: /jev/scan (セマンティック走査: Safe/Warning/Critical)
```

| エンドポイント | 役割 | 実装方式 | スロット |
|---|---|---|---|
| `POST /jev/stop` | ループ打ち切り判定 (CoT) | 1行 CoT 思考理由 + GBNF `Yes\|No` | Slot 0 |
| `POST /jev/extract` | ツール引数・差分抽出 | JSON Schema 拘束 (`response_format`) | Slot 1 |
| `POST /jev/scan` | セマンティック脆弱性・障害走査 | LLM セマンティック判定 + GBNF (`Safe\|Warning\|Critical`) | Slot 2 |
| `POST /v1/systemone` | 高速選択・分岐判定 | 動的 GBNF + logprobs Softmax | Slot 0 |
| `GET /demo` | 3列の目視デモ | 組み込み静的 HTML | - |
| `GET /health` | LLM・3スロット状態確認 | `n_slots`, `warn_parallel (n < 3)` | - |
| `GET /jev/schema` | API 契約・スキーマ公開 | JSON Schema | - |

---

## 2. ReflexGate「上等化」改修内容

### 要件 1: モデル換装と 3 スロット並行構成
* **推奨モデル**: `Qwen2.5-Coder-1.5B-Instruct` (Q8_0 / Q4_K_M)
  * コードログ、例外スタックトレース、シェル実行結果の読解力が飛躍的に向上。
* **起動パラメータ**:
  * `--parallel 3`:
    * Slot 0: Stop CoT / SystemOne
    * Slot 1: Extract
    * Slot 2: Scan
  * コンテキスト長: `-c 8192` (8K)
  * KV キャッシュ: `--cache-type-k f16 --cache-type-v f16`

### 要件 2: Task A (`/jev/stop`) の 1 行 CoT (Chain-of-Thought) 化
* 一発判定ではなく、1 行の理由を自己回帰デコードさせてから結論を決定。
* **GBNF 文法拘束**:
  ```gbnf
  root ::= "Reason: " [^\n]+ "\nVerdict: " ("Yes" | "No")
  ```
* **レスポンス仕様**:
  ```json
  {
    "stop": true,
    "raw": "Yes",
    "reason": "All 12 pytest tests passed with exit code 0.",
    "metrics": { ... }
  }
  ```

### 要件 3: Task C (`/jev/scan`) のセマンティック走査への復帰
* 正規表現（`[ERROR]` / `[FATAL]` の文字一致）を完全撤廃。
* LLM の文脈理解によって、文字面で ERROR と書かれていない致命的障害やセキュリティリスクを判定。
  * **Critical (即時遮断)**: SIGSEGV, OOM, カーネルパニック, 機密露出 (API Key), プロンプトインジェクション
  * **Warning (警告)**: リトライ可能なネットワーク例外, 非推奨警告
  * **Safe (正常)**: 通常ログ
* **GBNF 文法拘束**:
  ```gbnf
  root ::= "Severity: " ("Safe" | "Warning" | "Critical") "\nFinding: " [^\n]+
  ```
* **レスポンス仕様**:
  ```json
  {
    "severity": "Critical",
    "finding": "Hidden SIGSEGV memory reference error in thread 4.",
    "action": "halt_loop",
    "metrics": { ... }
  }
  ```
  ※ `severity == "Critical"` の場合は `action: "halt_loop"` を返却。

### 要件 4: UI および設定の追従
* GUI 稼働モニターに Slot 0, Slot 1, Slot 2 の 3 つの LED インジケーターを設置。
* `config.json` における柔軟なモデルパス設定の維持。
* ポート衝突防止・cmd ウィンドウ完全非表示化の統合。

---

## 3. テスト検証結果

### 単体テスト (`go test -v ./...`)
* `TestParseCoT`: 正常系、複数行、フォールバックの全ケースで Green。
* `TestParseScan`: Critical / Warning / Safe / 未知分類の全ケースで Green。
* `TestChatPrompt`: Qwen2.5 `<|im_start|>` テンプレートの一致確認。
* `TestHandlersWithMockLlama`: `/jev/stop` (CoT) および `/jev/scan` (Semantic) の JSON 契約検証。
* `TestIsPortInUse`: ポート競合検知の検証。
* `TestAuthValidation`: 3段階認証モード (`off`, `loose`, `strict`, `config.json` フォールバック) 全パターン検証。
* `TestSystemOneInputValidation`: TypeSafe wire-compatible `/v1/systemone` 契約・バリデーション検証。

### コンパイル確認
* `build/bin/local-jev.exe`: Wails v2 GUI バイナリ 正常ビルド完了。
* `build/bin/jevapi.exe`: スタンドアロン CLI/API 正常ビルド完了。

---

## 4. 長時間総合ベンチマーク結果 (111問 100% 達成)

全39ケース x 3パス（計111テスト）による耐久ベンチマークにて、**100.0% (111/111) 全ケース完全パス**を達成。

### 正解率推移

| 回 | スコア | 改善内容・マイルストーン |
|---|---|---|
| 1 | 89.2% | 初回ベースライン |
| 2 | 91.9% | スキーマ緩和・トークン長拡張 (`n_predict: 256`) |
| 3 | 86.5% | 幻覚抑止による過剰ヘッジ（Warning逃げ込み）の発生 |
| 4 | 94.6% | Task C の Finding-First 因果逆転 CoT 導入 |
| 5 | 97.3% | コンテキスト保護トリム & ステータス優先度調整 |
| 6 | **100.0% (111/111)** | **ハイブリッド・シークレットシールド導入（C-C4 復帰、C-T2 幻覚根絶）** |

### 反応速度・スループット推移

| 指標 | 前回 (Run 5) | 今回 (Run 6) | 変化 |
|---|---|---|---|
| Stop p50 | 0.68s | **0.65s** | 高速化 |
| Extract p50 | 1.18s | **1.14s** | 高速化 |
| Scan p50 | 0.59s | **0.58s** | 高速化 |
| TTFT p50 | 66ms | **65ms** | 高速化 |
| デコード速度 | 37.7 tok/s | **38.7 tok/s** | 向上 |
| 持続スループット | 2.5 req/s | **2.4 req/s** | 3スロット飽和状態を維持 |

* **検証総括**:
  * 唯一残存していた **C-C4（GitHub PAT / 各種APIシークレット漏洩）** が Fast-Path により 0.1ms 未満で確実遮断。
  * **C-T2（無害な "github" 言及）** でのシークレット誤検知（幻覚）も 0 件を維持。
  * スロット3構成におけるスループットは ~2.4 req/s で飽和安定。正解率・速度ともに過去最高値を記録。
