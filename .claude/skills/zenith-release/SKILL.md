---
name: zenith-release
description: >-
  zenith-wallpaper の実装・修正が完了したとき、コミット・push・make release・
  CI 監視まで一貫して行い、dnf リポジトリへの RPM 公開と Homebrew tap の
  formula bump を完結させる。
  「実装完了」「修正完了」「デプロイして」「反映して」「リリース」「make release」
  「linux-pkg に登録」「RPM を更新」「brew を更新」などのワードが出たら積極的に起動すること。
  このスキルを使わずに git commit・git tag・make release を手動で行ってはいけない。
---

# zenith-release — コミットから RPM / Homebrew 公開まで

zenith-wallpaper への変更を `repo.devtools.site` の dnf リポジトリと
`kter/homebrew-tap`(brew)へ確実に届ける。
タグ push は公開ビルドを起動する不可逆操作なので、各ステップでユーザーの確認を
取りながら進む。

## Step 1 — リポジトリの状態を把握する

```bash
git status --short
git log origin/main..HEAD --oneline
git branch --show-current
```

確認すること:
- **ブランチ**: `main` 以外なら「`main` ブランチではありません」と警告し、
  続行するかユーザーに確認する。
- **未コミット変更**: あれば Step 2 へ。
- **未 push コミット**: コミット済みだが push 前の場合は Step 2b から。
- **クリーン & push 済み**: Step 3 へ。

## Step 2 — 未コミット変更をコミット・push する

### 2a. コミットメッセージを提案する

`git diff --stat HEAD` と `git diff HEAD` を読み、変更の意図を一行で要約して
コミットメッセージ案を提示する。

```
例: feat: improve star brightness rendering for high-DPI displays
```

ユーザーの確認(または修正)を得てからコミットする:

```bash
git add -A
git commit -m "<確認済みメッセージ>"
```

### 2b. push する

```bash
git push origin main
```

push 失敗(競合など)の場合はエラーを表示して停止し、ユーザーに対処を仰ぐ。

## Step 3 — バージョンを決定する

```bash
git tag -l 'v*' --sort=-version:refname | head -1
```

- タグが存在しない → `v1.0` を提案
- 最新タグが `vX.Y` → `vX.(Y+1)` を提案(マイナー上げがデフォルト)

ユーザーに確認する:

> 「`vX.Y` としてリリースします。よろしいですか?
> (メジャー上げや別バージョンにする場合は番号を指定してください)」

**ユーザーの承認を得るまでタグ push を実行してはいけない。**

## Step 4 — make release を実行する

```bash
make release VERSION=X.Y
```

`make release` には以下のガードが組み込まれている:
- 作業ツリーがクリーン
- `main` ブランチにいる
- `origin/main` と同期済み

ガードが通過すれば `vX.Y` タグが push され、`release.yml` が起動する。
失敗した場合はエラーメッセージを表示して停止する。

## Step 5 — CI を監視して公開を確認する

### 5a. zenith-wallpaper 側の release.yml を確認

```bash
sleep 5
gh run list --repo kter/zenith-wallpaper --limit 2
```

`Release (RPM build + Homebrew tap bump)` が `completed / success` であることを
確認する。この run には2つの job がある:
- `dispatch` — linux-pkg への RPM ビルド依頼
- `homebrew` — kter/homebrew-tap の formula を新バージョン・新 sha256 に bump
  (secret `HOMEBREW_TAP_DEPLOY_KEY` = tap の write 用 deploy key が必要)

失敗なら `gh run view <id> --repo kter/zenith-wallpaper --log` でログを表示する。
`homebrew` job 成功後、`gh api repos/kter/homebrew-tap/commits/main` で
formula bump コミットが入ったことも確認できる。

### 5b. linux-pkg 側の build-rpm.yml を特定・監視

```bash
gh run list --repo kter/linux-pkg --limit 3
```

`repository_dispatch` で起動した `Build and Publish RPM` の run ID を特定し、
完了まで監視する:

```bash
gh run watch <run-id> --repo kter/linux-pkg
```

### 5c. 結果を報告する

**成功時:**
```
✅ zenith-wallpaper vX.Y が公開されました。
   Fedora: `sudo dnf upgrade zenith-wallpaper`
   macOS:  `brew update && brew upgrade zenith-wallpaper`
```

**失敗時:**
```bash
gh run view <run-id> --log --repo kter/linux-pkg
```
ログを要約してユーザーに原因と次のアクションを伝える。
CloudFront キャッシュが古い場合は `docs/operations.md` の invalidation 手順を案内する。

## ハードルール

- タグ push (= `make release`) はユーザー確認後にのみ実行する。
- `main` 以外のブランチではリリースしない。
- 資格情報(PAT・GPG キー等)を出力しない。
- `make release` を使わずに `git tag` を直接実行しない。
