# ja2en

WezTerm 運用前提の日本語→英語翻訳 CLI ツール。**Google AI Studio (Gemini 2.5 Flash-Lite)** をデフォルトに、叩いた瞬間に英訳が返る体験を最優先。

## 特徴

- **起動 ~10ms**: Go 製シングルバイナリ
- **デフォルト無料**: Google AI Studio の Gemini 2.5 Flash-Lite（stable、クレカ登録不要、~500 RPD）
- **多プロバイダ対応**: profile ごとに API base / API key の env var を切替可能（OpenRouter / OpenAI / その他互換 API）
- **設定ファイル + プロファイル切替**: シンプル英訳とカスタムプロンプトを使い分け
- **クリップボード対応**: `--clip` で読み込み、`--paste` で書き戻し
- **API キーは環境変数のみ**: 設定ファイルには書かない

## インストール

```bash
git clone <this-repo> ~/ja2en
cd ~/ja2en
make install        # $GOPATH/bin/ja2en に配置
```

## セットアップ

1. Google AI Studio で API key を発行: <https://aistudio.google.com/apikey>
2. シェル設定に追記:
   ```bash
   export GEMINI_API_KEY="..."
   ```
3. 設定ファイル生成:
   ```bash
   ja2en init
   ```

OpenRouter を使いたい場合は `OPENROUTER_API_KEY` を設定し、`ja2en init` 後に `~/.config/ja2en/config.toml` の openrouter プロファイルを有効化。

## 使い方

```bash
# 引数
ja2en "明日出社する"

# stdin
echo "今日は遅れる" | ja2en

# クリップボード読込
ja2en --clip

# 翻訳結果をクリップボードに書込
ja2en --paste "緊急対応します"

# 読込→翻訳→書戻し
ja2en --clip --paste

# プロファイル切替
ja2en --profile openrouter "..."

# 一発でモデル指定
ja2en --model gemini-2.5-flash-lite "..."
```

## 設定

`~/.config/ja2en/config.toml`:

```toml
default_profile = "simple"
model = "gemini-2.5-flash-lite"
api_base = "https://generativelanguage.googleapis.com/v1beta/openai"
api_key_env = "GEMINI_API_KEY"
timeout_seconds = 30

[profiles.simple]
prompt = "..."

[profiles.openrouter]
api_base = "https://openrouter.ai/api/v1"
api_key_env = "OPENROUTER_API_KEY"
model = "anthropic/claude-haiku-4.5"
prompt = "..."
```

## シェルエイリアス（オプション）

`~/.bashrc` に追加すると便利:

```bash
t() {
    if [ "$#" -eq 0 ]; then
        if [ -t 0 ]; then
            ja2en --clip --paste
        else
            ja2en
        fi
    else
        ja2en "$@"
    fi
}
tcr() { ja2en --clip; }
tp() { ja2en --paste "$@"; }
```

## ライセンス

MIT
