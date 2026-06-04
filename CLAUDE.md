# zenith-wallpaper

## リリース手順

main にマージしただけでは何も公開されない。dnf リポジトリ (`repo.devtools.site`) へ
反映するにはバージョンタグを打つ必要がある。**必ず以下のコマンドを使うこと:**

```sh
make release VERSION=X.Y
```

これが `vX.Y` タグを push し、`kter/linux-pkg` の CI が自動的に RPM をビルド・
GPG 署名・S3 公開する。手動で `git tag` せず、常に `make release` を使う
(clean / main / origin 同期のガードが誤リリースを防ぐ)。
