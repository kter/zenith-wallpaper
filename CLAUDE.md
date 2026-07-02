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
   新バージョン・新 sha256 に自動 bump (secret `HOMEBREW_TAP_TOKEN` が必要)

手動で `git tag` せず、常に `make release` を使う
(clean / main / origin 同期のガードが誤リリースを防ぐ)。

## プラットフォーム構成

ビルドタグで分割。共通の API は `Output` 構造体 (output.go) と
`GetOutputs()` / `SetWallpaper()` / `tryPlatformLocation()`:

- Linux: `output_linux.go` (swaymsg), `location_linux.go` (GeoClue2 D-Bus)
- macOS: `output_darwin.go` (system_profiler + osascript/desktoppr),
  `location_darwin.go` (スタブ、ipinfo.io フォールバックに委ねる)

純粋ロジック (JSON パース等) はビルドタグなしの共有ファイル
(`sysprofiler.go` / `swayoutputs.go` / `wallpaperfile.go`) に置き、どの開発
プラットフォームからでもテストできるようにしている。exec を伴う部分だけを
タグ付きファイルに残すこと。

変更時は `make test` を実行すること (vet + 全テスト + darwin クロスコンパイル
確認を含む)。CI (test.yml) でも同じチェックが走る。
