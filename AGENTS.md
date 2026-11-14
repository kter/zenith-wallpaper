# zenith-wallpaper

## リリース手順

main にマージしただけでは何も公開されない。公開にはバージョンタグを打つ必要が
ある。**必ず以下のコマンドを使うこと:**

```sh
make release VERSION=X.Y
```

これが `vX.Y` タグを push し、release.yml から2系統が並走する:

1. **dnf (Fedora)**: `kter/linux-pkg` の CI が RPM をビルド・GPG 署名・
   S3 公開 (`repo.devtools.site`)
2. **Homebrew (macOS)**: `homebrew` job が `kter/homebrew-tap` の formula を
   新バージョン・新 sha256 に自動 bump (secret `HOMEBREW_TAP_DEPLOY_KEY` が必要)

手動で `git tag` せず、常に `make release` を使う
(clean / main / origin 同期のガードが誤リリースを防ぐ)。

## プラットフォーム構成

ビルドタグで分割。共通の API は `Output` 構造体 (output.go) と
`GetOutputs()` / `SetWallpaper()` / `tryPlatformLocation()`:

- Linux: `output_linux.go` (swaymsg), `location_linux.go` (GeoClue2 D-Bus)
- macOS: `output_darwin.go` (system_profiler + osascript/desktoppr),
  `location_darwin.go` (スタブ、ipinfo.io フォールバックに委ねる)

純粋ロジック (JSON パース等) はビルドタグなしの共有ファイル
(`sysprofiler.go` / `swayoutputs.go` / `wallpaperfile.go` /
`wallpaperstore.go` / `macwallpaper.go`) に置き、どの開発
プラットフォームからでもテストできるようにしている。exec を伴う部分だけを
タグ付きファイルに残すこと。

変更時は `make test` を実行すること (gofmt チェック + vet + 全テスト + darwin
クロスコンパイル確認を含む。整形は `make fmt`)。CI (test.yml) でも同じチェックが走る。

## クラウドのエージェントで作業するとき

Claude Code on the web と Codex Cloud で作業するときの約束。

- **やらないこと:** リリース。`make release`・`git tag`・タグの push をしない。上の「リリース手順」は
  人が手元で実行するもので、クラウドのエージェントは使わない（`.claude/skills/zenith-release` も同じ）。
  `v*` タグはルールセットで管理者だけが作れる。main へ直接 push しない（ブランチ保護で拒否される）。
  `make install`・`make install-units` もしない（手元の systemd を触る）。
- **準備:** `bash scripts/agent/setup.sh` が go.mod の Go を用意する。依存は `vendor/` にあるので、ほかに入れるものは無い。
  Claude Code on the web では `.claude/settings.json` の SessionStart フックが自動で実行する。
  Codex Cloud では環境のセットアップスクリプトに同じコマンドを書く。
- **完了条件:** `make check`（`make test` と同じ。CI の test.yml と同じ確認）が終了コード 0 で、
  PR の説明に実行したコマンドと結果を書いていること。
- **人に戻すこと:** 実機での確認（sway の壁紙設定、macOS の osascript、GeoClue の位置取得）とリリース。
  PR の説明に「未確認」として書く。macOS の実機確認は人が手で行う。
- **ホストに依存しないテストにする:** swaymsg・system_profiler・D-Bus・ipinfo.io を呼ぶのはタグ付きファイルだけで、
  テストはパーサーなどの純粋ロジックを対象にする。クラウドのコンテナにはどれも無い。

### 各クラウドの実測

| 項目 | Claude Code on the web | Codex Cloud |
| --- | --- | --- |
| Go | 未計測 | 未計測 |
| proxy.golang.org・go.dev への到達 | 未計測 | 未計測 |
| `make check` | 未計測 | 未計測 |
