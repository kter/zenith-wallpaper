# zenith-wallpaper

## リリース手順

main にマージしただけでは何も公開されない。公開は Release ワークフロー
（release.yml、`workflow_dispatch`）だけが行う。**手元でもクラウドでも同じ入口を使う:**

```sh
make release VERSION=X.Y
# make や gh の GraphQL が使えない環境では、同じ REST 呼び出しを直接実行する
gh api -X POST repos/kter/zenith-wallpaper/actions/workflows/release.yml/dispatches \
  -f ref=main -f 'inputs[version]=X.Y'
```

ワークフローは `release` 環境で 621 の承認を待つ。承認されると、main の Test が通っていることと
タグが未使用であることを確かめてから `vX.Y` タグを作り、2系統が並走する:

1. **dnf (Fedora)**: `kter/linux-pkg` の CI が RPM をビルド・GPG 署名・
   S3 公開 (`repo.devtools.site`)
2. **Homebrew (macOS)**: `homebrew` job が `kter/homebrew-tap` の formula を
   新バージョン・新 sha256 に自動 bump (secret `HOMEBREW_TAP_DEPLOY_KEY` が必要)

タグを手で push しても何も公開されない。既存の `v*` タグの移動・削除はルールセットで管理者だけに限っている。

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

- **やらないこと:** `git tag`・タグの push、main への直接 push（ブランチ保護で拒否される）、
  `make install`・`make install-units`（手元の systemd を触る）。
- **リリースするとき:** 変更を PR で main に入れ、main の Test が通ったあとで、上の「リリース手順」の
  REST 呼び出しで Release ワークフローを起動する。バージョンは `git tag --list 'v*' --sort=-v:refname | head -1` の次を選ぶ。
  起動したら 621 の承認待ちになるので、PR か Issue に「承認待ち」と run の URL を書いて人に戻す。
  `.claude/skills/zenith-release` は手元向け（main へ直接 commit・push する）なので、クラウドではこの手順を使う。
- **準備:** `bash scripts/agent/setup.sh` が go.mod の Go を用意する。依存は `vendor/` にあるので、ほかに入れるものは無い。
  Claude Code on the web では `.claude/settings.json` の SessionStart フックが自動で実行する。
  Codex Cloud では環境のセットアップスクリプトに同じコマンドを書く。
- **完了条件:** `make check`（`make test` と同じ。CI の test.yml と同じ確認）が終了コード 0 で、
  PR の説明に実行したコマンドと結果を書いていること。
- **人に戻すこと:** 実機での確認（sway の壁紙設定、macOS の osascript、GeoClue の位置取得）とリリースの承認。
  PR の説明に「未確認」として書く。macOS の実機確認は人が手で行う。
- **ホストに依存しないテストにする:** swaymsg・system_profiler・D-Bus・ipinfo.io を呼ぶのはタグ付きファイルだけで、
  テストはパーサーなどの純粋ロジックを対象にする。クラウドのコンテナにはどれも無い。

### 各クラウドの実測

| 項目 | Claude Code on the web | Codex Cloud |
| --- | --- | --- |
| Go | 1.25.0 が `/usr/local/go` に入っている。setup.sh はそれを使う（2026-11-15、PR #6） | `/usr/bin/go` は Go ではない別のコマンド。環境の install script が Go 1.25.14 を入れ、`. /workspace/.tools/zenith/activate.sh` で PATH に載る。setup.sh は偽の go を無視して go.dev から取る |
| proxy.golang.org・go.dev への到達 | どちらも 200 | どちらも 200 |
| `make check` | 終了コード 0（約34秒）。SessionStart フックが setup.sh を自動で実行した | activate.sh を読んでから実行する |
| GitHub への書き込み（リリースの起動） | `gh` は kter のユーザートークンで、REST の dispatch を呼べる。一部の API（environments など）はセッションのプロキシが 403 で止める | `gh` の REST は Forbidden。GitHub コネクターは読み取りのみで dispatch の手段が無い。**Codex からはリリースを起動できない**ので、PR に「リリースが必要」と書いて人に戻す |
