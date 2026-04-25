# ja2en

WezTerm 運用前提の日本語→英語翻訳 CLI ツール。Go 製、デフォルトは Google AI Studio (Gemini 2.5 Flash-Lite) 直接呼び出し。

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

- **`GEMINI_API_KEY`** — デフォルトプロファイル用（Google AI Studio キー、`AIzaSy...` 39文字）  
  発行: <https://aistudio.google.com/apikey>（クレカ登録不要、無料）
- **`OPENROUTER_API_KEY`** — `openrouter` プロファイル使うときのみ必要（`sk-or-v1-...` 73文字）

API キー設定は `~/.claude/rules/secrets-rotation.md` の secure pattern を必ず使う。`tail ~/.bashrc` 等で生値を Claude context に流出させない。

## アーキテクチャの肝

- `internal/config/config.go` の `Resolve()` が「CLI flag > profile-level > top-level > hard-coded default」の precedence で最終値を組み立てる
- **`api_key_env` フィールド**で「どの環境変数を読むか」を config 側から指定可能 → 同じバイナリで Google AI Studio / OpenRouter / その他 OpenAI 互換 API を profile 単位で切替できる
- `internal/translator/openrouter.go` は OpenAI 互換 (`/chat/completions`) を叩くだけ。互換エンドポイントなら何でも動く（Google の `https://generativelanguage.googleapis.com/v1beta/openai` も互換）

## 既知の制約

- **Gemma free tier (`gemma-*:free`)** は system role 非対応。`Developer instruction is not enabled` で 400 が返る。OpenRouter / Google AI Studio どちら経由でも同じ。ja2en はシステムプロンプト依存なので Gemma は使えない
- **`gemini-3.1-flash-lite-preview`** は理論上最速 (TTFT 2.5x、~381 t/s) だが 2026-04 時点で HTTP 503 "experiencing high demand" を頻発する。stable に昇格するまで `gemini-2.5-flash-lite` を使う
- **OpenRouter free tier** (例: `:free` suffix) は上流プロバイダのグローバル制限で 429 を返すことがある
- **WSL のクリップボード**: `atotto/clipboard` が `powershell.exe` / `clip.exe` 経由で動く。Claude Code の Bash サンドボックスは PATH にこれらが入ってないので、フルパス (`/mnt/c/Windows/System32/clip.exe` 等) を呼ぶこと

## デフォルトモデル変遷

| バージョン | デフォルトモデル | プロバイダ | 平均レイテンシ |
|---|---|---|---|
| v0.1.0 | `openai/gpt-oss-120b:free` | OpenRouter | ~2000ms |
| v0.2.0 | `gemini-2.5-flash-lite` | Google AI Studio 直接 | ~850ms |

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
