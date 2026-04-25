# ja2en

WezTerm 運用前提の日本語→英語翻訳 CLI ツール。Go 製、デフォルトは OpenAI (gpt-5.4-nano) 直接呼び出し、Gemini 2.5 Flash-Lite と DeepL Free をフォールバックに持つ multi-provider 構成。

## 開発コマンド

| コマンド | 用途 |
|---|---|
| `make build` | ローカルビルド (`./ja2en` 生成) |
| `make install` | `$GOPATH/bin/ja2en` にインストール |
| `make test` | `go test ./...` |
| `make lint` | golangci-lint v2 実行 |
| `make check` | fmt + vet + lint + test 一括 |
| `make clean` | バイナリ削除 |

## 必須環境変数

profile ごとに参照する env が違う。default profile に対応するキーは必須、フォールバック profile を使う時に追加が必要。

- **`OPENAI_API_KEY`** — default `openai` profile (gpt-5.4-nano、有料) で必須。`sk-...` 形式  
  発行: <https://platform.openai.com/>（**最低 $5 prepaid 必須**、credits は 1 年で expire）
- **`GEMINI_API_KEY`** — フォールバック `gemini` profile 用（Google AI Studio、`AIzaSy...` 39文字）  
  発行: <https://aistudio.google.com/apikey>（CC 不要、free tier あり。本番は Tier 1 paid 推奨）
- **`DEEPL_API_KEY`** — 緊急避難 `deepl` profile 用。free key は `:fx` で終わる（例: `abc...xyz:fx`）  
  発行: <https://www.deepl.com/pro-api>（CC 必須、free tier 500K chars/月、課金なし）
- **`OPENROUTER_API_KEY`** — オプションの `openrouter` profile 用（`sk-or-v1-...` 73文字）

API キー設定は `~/.claude/rules/secrets-rotation.md` の secure pattern を必ず使う。`tail ~/.bashrc` 等で生値を Claude context に流出させない。

## アーキテクチャの肝

- `internal/config/config.go` の `Resolve()` が「CLI flag > profile-level > top-level > hard-coded default」の precedence で最終値を組み立てる
- **`provider` フィールド**で translator を切替: `"openai"` (OpenAI 互換 chat completions) または `"deepl"` (DeepL REST API)。デフォルトは `"openai"` で後方互換
- **`api_key_env` フィールド**で「どの環境変数を読むか」を config 側から指定可能 → 同じバイナリで OpenAI / Google AI Studio / OpenRouter / その他 OpenAI 互換 API を profile 単位で切替できる
- **`reasoning_effort` フィールド**で reasoning モデルの思考量を制御: GPT-5.4-nano は `"none"` / `"low"` / `"medium"` / `"high"` / `"xhigh"`（**`"minimal"` は非対応で 400 を返す**）、Gemini 2.5 は `"none"` で thinking 完全 OFF。翻訳タスクは推論不要なので `"none"` 一択
- `internal/translator/openrouter.go` は OpenAI 互換 (`/chat/completions`) を叩くだけ
- `internal/translator/deepl.go` は DeepL の `/v2/translate` を叩く。エンドポイントは API key suffix `:fx` で free/paid を自動判定

## 既知の制約

- **Gemma 3 free tier (`gemma-3-*`)** は system role 非対応で `Developer instruction is not enabled` 400 が返る。**Gemma 4 (`gemma-4-26b-a4b-it`, `gemma-4-31b-it`) は native system role 対応**で ja2en で動くが、API 経由では thinking モードを完全 OFF できない欠陥あり (公式が認めてる: "may occasionally generate a thought channel even when thinking mode is explicitly turned off")。翻訳特化用途では gpt-5.4-nano か gemini-2.5-flash-lite + reasoning_effort=none の方が安定
- **Gemini 2.5 系の thinking 罠**: API リクエストで `reasoning_effort` を送らないと thinking モードがデフォルト ON で、TPM 250K (project shared) を 1 リクエストで数千〜数万 tokens 消費して即座に枯渇 → 実効 RPD が公称 1000 から 20 に落ちる。**ja2en は `reasoning_effort = "none"` を必ず指定する設計**になっている
- **Gemini Tier 1 paid 移行の落とし穴**: Google AI Studio で billing 有効にしただけでは Tier 1 にならない場合がある。GCP Console 側の project に billing account が正しくリンクされてないと、有料化したのに `generativelanguage.googleapis.com/generate_content_free_tier_requests, limit: 20` で 429 が返り続ける。AI Studio の Rate Limits ページで実効値を必ず確認する
- **DeepL Free は system prompt 非対応** (構造的に存在しない)。formality は ja→en では non-supported、context パラメータと glossary でしか調整不可。`prompt` フィールドは設定しても無視される。Pro でも同じ
- **DeepL Free tier はデータを学習に使う** (opt-out 不可)。機密文書を翻訳しないこと。Pro tier ($5.49/月 base + $25/M chars) なら学習なし
- **`gemini-3.1-flash-lite-preview`** は理論上最速 (TTFT 2.5x、~381 t/s) だが 2026-04 時点で HTTP 503 "experiencing high demand" を頻発する。stable に昇格するまで `gemini-2.5-flash-lite` を使う
- **OpenRouter free tier** (`:free` suffix) は per-model 200 RPD で枯渇する。本番運用には不向き
- **WSL のクリップボード**: `atotto/clipboard` が `powershell.exe` / `clip.exe` 経由で動く。Claude Code の Bash サンドボックスは PATH にこれらが入ってないので、フルパス (`/mnt/c/Windows/System32/clip.exe` 等) を呼ぶこと

## デフォルトモデル変遷

| バージョン | デフォルトモデル | プロバイダ | 平均レイテンシ | 備考 |
|---|---|---|---|---|
| v0.1.0 | `openai/gpt-oss-120b:free` | OpenRouter | ~2000ms | per-model 200 RPD で詰まる |
| v0.2.0 | `gemini-2.5-flash-lite` | Google AI Studio 直接 (free) | ~850ms | thinking 漏れで実効 RPD 20 |
| v0.3.0 | `gpt-5.4-nano` + `reasoning_effort=none` | OpenAI 直接 (paid, $5 prepaid) | ~600ms | ja→en 品質最強、月 $0.13 想定 (`minimal` は 400 になるため `none` 採用) |

v0.3.0 では multi-provider 設計になり、`gemini` (Gemini 2.5 Flash-Lite + reasoning_effort=none、フォールバック) と `deepl` (DeepL Free 500K chars/月、緊急避難) の profile も同時に設定される。`ja2en --profile gemini` / `--profile deepl` で即切替可能。

## クリップボード操作の安全運用

`--clip` / `--paste` フラグ周りのテスト時は **必ず事前にダミー文字列で上書き**:

```bash
echo -n "DUMMY-CLIP-CONTENT" | /mnt/c/Windows/System32/clip.exe
sleep 1
./ja2en --paste "テスト"
# verify
/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -Command "Get-Clipboard"
```

`--paste` 失敗時にクリップボードに残るのは `DUMMY-CLIP-CONTENT` だけになり、API key 等の機密値の事故露出を防げる（過去に二度同じ事故を起こしたので必須）。

## バックアップ手順

`~/.bashrc` を編集する作業時は必ず `cp ~/.bashrc ~/.bashrc.pre-<task>` でバックアップを取ってから。secrets-rotation テンプレも自動でバックアップを取る。
