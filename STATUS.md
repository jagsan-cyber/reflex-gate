# local-jev 実装状況・テスト報告

日付: 2026-09-19  
対象: `jev-bench/`（独立 git、MIT）  
ランタイム: llama.cpp `llama-server` (HIP/ROCm) + Qwen3.5-0.8B Q8_0

本家 TypeSafe Jev の公式クライアント互換ではない。役割（安く速い型付き判定）をローカル 0.8B で再現したゲートである。

---

## 1. 何を実装したか

エージェントがログだけ送り、サーバ側が契約を固定して判定を返す HTTP プロキシ。

```
caller  -->  jev_api.py :8090  -->  llama-server :8080  (slot 0/1)
                 |
                 +--> /jev/scan は LLM を使わず正規表現
```

| 口 | 役割 | 実装 |
|---|---|---|
| `POST /jev/stop` | ループ打ち切り Yes/No | Slot 0、chat + GBNF `Yes\|No` |
| `POST /jev/extract` | ツール引数 JSON | Slot 1、JSON Schema 拘束 |
| `POST /jev/scan` | `[ERROR]`/`[FATAL]` 1行 | 正規表現。LLM なし |
| `POST /v1/systemone` | 本家風 noul / choice | Slot 0、動的 GBNF + logprobs Softmax |
| `GET /demo` | 3列の目視デモ | 静的 HTML |
| `GET /health` | LLM・スロット数 | `n_slots`, slot 割り当て |
| `GET /jev/schema` | 契約の公開 | JSON |

スロット:

- Slot 0: stop / systemone（システム文固定、prefix cache）
- Slot 1: extract
- `POST /slots/{id}?action=erase` は廃止
- llama-server は `--parallel 2`、KV は **K/V f16**

extract 契約:

```json
{
  "status": "passed|failed|timeout|running",
  "error_code": "E123" | null,
  "files_changed": 0,
  "tool": "pytest"
}
```

`error_code: "none"` は禁止。成功は必ず `null`。

---

## 2. 実測（2026-09-19、このマシンで再実行）

モデル: Qwen3.5-0.8B Q8_0、ROCm、KV f16、`--parallel 2`。

### フルベンチ（ラベル付き 70 件、改修前の最終 LLM 走査時）

当時の scan はまだ LLM。extract 後に JSON Schema がスロットに残り、C が `E129` になった。

| タスク | 正答 | 遵守 | 備考 |
|---|---|---|---|
| A `/jev/stop` | 30/30 | 30/30 | 1K ログ |
| B `/jev/extract` | 30/30 | 30/30 | Schema 拘束 |
| C `/jev/scan` (LLM) | 0/10 | 0/10 | B の grammar リーク |

そのため scan は正規表現に切り替えた。

### 正規表現化・systemone 追加後の疎通（再実行済み）

| テスト | 結果 | レイテンシ |
|---|---|---|
| health | `n_slots=2`, llm_ok | - |
| systemone noul (Yes/No) | **Yes, p=0.968** (Yes 0.968 / No 0.032) | 136 ms |
| systemone choice (3肢) | **ruff_format**（GBNF） | 159 ms |
| `/jev/stop` 成功ログ | **stop=true / Yes** | 201 ms |
| `/jev/extract` | passed / error_code=null / files=2 / pytest | 447 ms（2回目 prompt_n=4 で cache） |
| `/jev/scan` | needle 行一致 | ~0 ms |
| `/demo` | HTTP 200 | - |

decode 目安: 約 60–80 tok/s。  
stop は約 0.2s。extract は約 0.4–0.7s。scan は 0ms。

### KV / FA 切り分け（scan を LLM で回していた時期）

| 設定 | 4K+ 出力 |
|---|---|
| K q8_0 / V turbo3, FA on | `????` 崩壊 |
| K/V f16, FA on または off | 意味のあるテキスト |
| `--no-cache-prompt` (ROCm) | 全面 `????`。採用しない |

0.8B は KV を f16 のまま 32K でもおよそ 1GB 未満。turbo3 に頼る理由は薄い。

---

## 3. 本家 Jev との差分

| | TypeSafe Jev | この local-jev |
|---|---|---|
| モデル | 判定専用 System One | 汎用 0.8B + 拘束デコード |
| 公式 API | `/v1/systemone` を中心に確率ヘッド | 同パスを追加。中身は llama.cpp |
| 出力 | 校正された確率分布 | 点推定 + Softmax(logprobs) |
| extract | 判定 + regex | JSON Schema 生成 |
| scan/screen | 確率で injection 等 | 正規表現のみ |
| OpenClaw 公式プラグイン | そのまま繋がる | **そのままでは繋がらない** |

`POST /v1/systemone` の JSON 形（type / result / p / distribution）は寄せた。重み・校正・MCP ツール一式は未実装。

---

## 4. 既知の制限

1. **choice の `p`**  
   `ruff_format` のような複数トークンは、先頭トークンが top-logprobs に乗らず `p` がほぼ 0 になることがある。`result` は GBNF が正しい。Yes/No は 1 トークンなので `p` は使える。
2. **scan は LLM ではない**  
   `[ERROR]` / `[FATAL]` 行頭だけ。意味理解はしない。
3. **ROCm で `--no-cache-prompt` 禁止**
4. **プロキシは Python**  
   llama-server が落ちると `/jev/stop` と `/v1/systemone` と extract は 502。scan だけは LLM なしで動く。
5. GitHub への push は未実施。ローカル git のみ。

---

## 5. 起動

```bat
start-qwen35-0.8b-rocm.bat
start-jev-api.bat
```

確認:

```bat
jev-bench\.venv\Scripts\python jev-bench\test_systemone.py
```

デモ: http://127.0.0.1:8090/demo

---

## 6. Go 化するなら

Python はプロンプトとスキーマをいじる実験用として妥当だった。常駐ゲートにするなら Go は筋が良い。

残してよい境界:

- llama-server は別プロセスのまま（HIP/Vulkan は C++ 側）
- 契約 JSON は今のまま（`/jev/stop`, `/jev/extract`, `/v1/systemone`）
- scan は Go の `regexp` で十分

Go に移す塊:

| 塊 | 対応 |
|---|---|
| HTTP | `net/http` または chi/echo。`encoding/json` |
| llama `/completion` と `/v1/chat/completions` | `net/http` クライアント |
| GBNF 組み立て | 文字列連結。エスケープだけ注意 |
| Softmax | `math.Exp` |
| スロット 0/1 | リクエスト JSON の `id_slot` |
| デモ | `embed.FS` で `demo.html` |

依存は標準ライブラリ中心で足りる。OpenAI 公式 SDK は不要。chat + `logprobs` の JSON を自分でパースすればよい。

先に固定するテスト:

1. `test_systemone.py` 相当を Go の `httptest` または実サーバ向け `TestMain` に移植
2. noul: 成功ログで `result=Yes` かつ `p > 0.9`
3. extract: `error_code == null`
4. scan: needle 完全一致
5. `--parallel 2` でないとき health が警告

Python 版は参照実装として残し、Go は同じポート契約で置き換えるのが安全である。
