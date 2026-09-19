# JEV ローカルサーバー TypeSafe API 互換化 実装仕様書 (API_COMPAT_SPEC.md)

本仕様書は、ローカル JEV サーバー（`/jev/stop`, `/jev/extract`, `/jev/scan`）を
TypeSafe AI 公式 Jev API と**完全なワイヤ互換（Wire-Compatible）**にするための仕様および実装記録である。

公式 `https://api.typesafe.ai/v1/systemone` を叩くクライアント（公式SDK、curl、AI SDK 等）が、
エンドポイントURLを差し替えるだけでローカルサーバーに接続して動作する。

互換対象バージョン: `jev-1.13.0` 系（エイリアス: `jev-latest`, `jev-preview`）。

---

## 1. エンドポイント仕様

### 1.1 `POST /v1/systemone`

本家と完全一致するリクエスト／レスポンス形式。

#### リクエストボディ

```json
{
  "model": "jev-latest",
  "state": "文字列 | JSONオブジェクト | 配列",
  "questions": {
    "<質問ID>: クライアントが決める任意キー": {
      "type": "noul | choice | score",
      "instructions": "質問文（必須）",
      "criteria": "typeにより異なる"
    }
  }
}
```

- `model`（string, 必須）: `jev-latest`, `jev-preview`, `jev-1.13.0` を受容。未指定またはそれ以外は 422。
- `state`（string | object | array, 必須）: 判定対象。構造化データは JSON 文字列化して評価。
- `questions`（map, 必須）: 空でないこと。複数質問を1リクエストに混在可。各質問を独立して並列判定。
- **質問ID（mapキー）をプロンプトへ絶対に出力しない**。モデルには `instructions` の全文と `criteria` のみ渡す。

#### 質問タイプ別要求・応答

| type | criteria | 応答の必須フィールド |
|------|----------|----------------------|
| `noul` | 任意: `{"true": "...", "false": "..."}` | `noul`（0〜1の確率）のみ。confidence は付けない |
| `choice` | 必須: `{オプション名: 説明}` 最大255個 | `choice`（選択結果）, `confidence`（0〜1）, `probabilities`（全オプションの確率） |
| `score` | 必須: 説明文の配列（順序あり, 2〜10段） | `score`（0〜len-1 の実数位置）, `confidence`, `legend`（index→ラベル）, `probabilities`（各段の確率） |

#### 応答サンプル（本家と同一）

```json
{
  "model": "jev-1.13.0",
  "answers": {
    "needs_review": { "type": "noul", "noul": 0.97 },
    "route": {
      "type": "choice",
      "choice": "billing",
      "confidence": 0.98,
      "probabilities": { "billing": 0.98, "shipping": 0.01, "technical": 0.01 }
    },
    "urgency": {
      "type": "score",
      "score": 1.6,
      "confidence": 0.62,
      "legend": { "0": "low", "1": "medium", "2": "high" },
      "probabilities": { "0": 0.02, "1": 0.36, "2": 0.62 }
    }
  },
  "usage": { "input_tokens": 190, "output_tokens": 0 }
}
```

#### 数値の制約
- `noul` は必ず [0,1]。四捨五入（小数点以下2桁）。
- `probabilities` は全項目合計≈1.0 になるよう正規化。欠落オプションは 0 として明示。
- `score.score` は `sum(index * prob(index))` で計算。

### 1.2 `GET /v1/models`

```json
{ "data": [ "jev-latest", "jev-preview", "jev-1.13.0" ] }
```

クライアントが `model` 欄へ指定可能なエイリアスを列挙。

---

## 2. エラーコード

| コード | 意味 | ボディの目安 |
|--------|------|--------------|
| 401 | Authorization ヘッダー欠落・不正 | `{"detail": "Invalid API key"}` |
| 422 | リクエストボディ検証失敗 | `{"detail": "<フィールド名>: <理由>"}` |
| 429 | レート制限超過 | `{"detail": "Rate limit exceeded"}` |
| 529 | LLM 過負荷 / エラー | `{"detail": "Service overloaded: ..."}` |

422 バリデーション対象:
- `choice` に `criteria` が無い / 空 / 255件超過
- `score` の `criteria` が配列でない / 長さが 2未満 or 10超
- `type` が noul/choice/score 以外
- `questions` が map でなく空
- `model` が未指定または未知の名称

---

## 3. 認証仕様 (3段階認証モード)

環境変数および `config.json` による3段階の柔軟な認証モード切替に対応（優先順位: `環境変数 > config.json`）。

- 設定項目:
  - モード指定: 環境変数 `JEV_AUTH` または `config.json` 内 `"auth_mode"` (`off` | `loose` | `strict`, デフォルト: `off`)
  - APIキー指定: 環境変数 `TYPESAFE_API_KEY` または `config.json` 内 `"api_key"`

### 動作モード一覧

1. **`off` (デフォルト / 未設定)**:
   - 完全バイパス。
   - `Authorization` ヘッダーの有無にかかわらず全リクエストを許可 (200 OK)。
   - 公式 SDK 自動付与のダミートークンやローカルスクリプトが無設定で即座に動作。

2. **`loose` (柔軟検証モード)**:
   - ヘッダー未指定時はローカル利用として許可 (200 OK)。
   - `Authorization` ヘッダーが指定された場合のみ、設定キーとの一致を検証（不一致時は 401）。

3. **`strict` (厳格検証モード)**:
   - 互換性テストおよび本番想定テスト用。
   - 一致する `Authorization: Bearer <key>` が必須（未指定または不一致時は 401）。

---

## 4. 既存 `/jev/*` API との関係

- 既存の `/jev/stop`, `/jev/extract`, `/jev/scan` は**完全凍結（後方互換100%維持）**。
- レスポンス形式や挙動に変更を加えない。
