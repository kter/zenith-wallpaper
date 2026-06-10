# zenith-wallpaper

現在地・現在時刻から計算した**実際の夜空**をデスクトップ壁紙として表示するツール。
「ラップトップの場所から真上を見上げた全天ドーム」を Lambert 等積方位投影で描画し、1時間ごとに自動更新する。
Linux (sway) と macOS に対応。

![スクリーンショット例](https://www.eso.org/public/images/eso0932a/) <!-- placeholder, replace with actual screenshot -->

## 何が表示されるか

| 要素 | 内容 |
|------|------|
| **天の川** | NASA Deep Star Maps 2020 銀河座標パノラマ（16384×8192）を天頂ドームへ逆ワープ。濃淡・暗黒帯・銀河中心まで写真的に再現 |
| **恒星** | Yale Bright Star Catalogue（約 8400 星、等級 ≤ 6.5）。等級に応じて点径・輝度を変調 |
| **惑星・月** | 軌道要素計算によるリアルタイム位置。地平線上の天体のみ表示 |
| **地平線・方位** | ドーム外周に地平線リング、N/E/S/W 方位ラベル |

### 投影方式

**Lambert 等積方位投影（天頂中心）**。北が上、東が左（真上を見上げた視点のため地図の鏡像）。
ドーム直径 = 画面の長辺。各ディスプレイの物理解像度ぴったりの画像を生成するため swaybg はスケール・歪みゼロで表示する。

### 位置情報の取得順

1. GeoClue2（D-Bus、Linux のみ）
2. ipinfo.io（HTTP、インターネット接続時）
3. `~/.cache/zenith-wallpaper/location.json`（前回キャッシュ）
4. フォールバック：グリニッジ (51.48°N, 0.00°E)

## 要件

**Linux:**
- [sway](https://swaywm.org/) (Wayland compositor)
- Go 1.21 以上（[mise](https://mise.jdx.dev/) 経由でも可）
- `swaymsg`（sway に同梱）

**macOS:**
- macOS Sonoma 以降（Apple Silicon / Intel）
- [Homebrew](https://brew.sh/)

## インストール

### dnf（Fedora）

```sh
# リポジトリを有効化（初回のみ）
sudo dnf install https://repo.devtools.site/rpm/noarch/kter-release-1-1.fc42.noarch.rpm

sudo dnf install zenith-wallpaper
```

インストール後、timer の有効化と sway config の追記が必要（下記参照）。

### Homebrew（macOS）

```sh
brew tap kter/tap
brew install zenith-wallpaper

# 初回のみ: Terminal から一度実行し、System Events の
# オートメーション許可ダイアログを承認する
zenith-wallpaper

# 毎時自動更新を有効化（launchd 経由）
brew services start zenith-wallpaper
```

ログは `$(brew --prefix)/var/log/zenith-wallpaper.log` に出力される。

**macOS の注意点:**
- 壁紙の適用は System Events の AppleScript 経由。初回実行時に
  オートメーション許可（TCC）のダイアログが出るので承認すること。
  許可しないと `osascript` がエラー -1743 で失敗する。
- [desktoppr](https://github.com/scriptingosx/desktoppr) が PATH にあれば
  そちらを優先して使う（Apple Events 不使用のため TCC 許可が不要）。
- Sonoma 以降では壁紙は各ディスプレイの**現在の操作スペース**にのみ適用される。
  他のスペースは次回そのスペースで実行された時に更新される。

### ソースから（mise / Go、Linux）

```sh
git clone https://github.com/kter/zenith-wallpaper
cd zenith-wallpaper
make install        # ~/.local/bin/zenith-wallpaper にコピー（sudo 不要）
make install-units  # systemd user timer を有効化
```

## 開発・リリース

main への変更だけでは dnf リポジトリは更新されない。バージョンタグの push が
トリガーになる。必ず以下のコマンドを使うこと:

```sh
make release VERSION=1.1
```

これが `v1.1` タグを push し、以下の2系統が並走する:

1. [kter/linux-pkg](https://github.com/kter/linux-pkg) の CI が RPM をビルド・署名・
   `repo.devtools.site` へ公開（dnf）
2. release.yml の `homebrew` job が [kter/homebrew-tap](https://github.com/kter/homebrew-tap)
   の formula を新バージョン・新 sha256 に自動 bump（brew）

homebrew job には `kter/homebrew-tap` への contents:write 権限を持つ
fine-grained PAT を secret `HOMEBREW_TAP_TOKEN` として登録しておく必要がある。

### sway config への追記（初回のみ）

`~/.config/sway/config` に以下を追加すると sway 起動時に自動実行される：

```
output * bg #000000 solid_color
exec /usr/bin/zenith-wallpaper          # dnf インストール時
# exec ~/.local/bin/zenith-wallpaper   # ソースインストール時
```

### timer の有効化（dnf インストール時）

```sh
systemctl --user enable --now zenith-wallpaper.timer
```

`make install-units`（ソースインストール）は自動で行うため不要。

## 使い方

```sh
# 即時実行（壁紙を今すぐ更新）
zenith-wallpaper

# ビルドのみ
make

# インストール（~/.local/bin）
make install

# systemd user timer の有効化
make install-units

# アンインストール
make uninstall
```

timer が有効化されると毎時 0 分に自動実行される（`OnCalendar=hourly`）。

```sh
# 次回実行時刻の確認
systemctl --user list-timers | grep zenith

# 直近のログ確認
journalctl --user -u zenith-wallpaper.service -n 20
```

## キャッシュ

| パス | 内容 |
|------|------|
| `~/.cache/zenith-wallpaper/<出力名>.png` | 各ディスプレイの壁紙 PNG |
| `~/.cache/zenith-wallpaper/location.json` | 前回取得した位置情報（オフライン時のフォールバック） |

macOS では `~/.cache` の代わりに `~/Library/Caches/zenith-wallpaper/` を使う。

画像・パノラマ・星表はバイナリに同梱しているため、初回ダウンロードは不要。

## ライセンス・データクレジット

- **Milky Way panorama**: NASA/Goddard Space Flight Center Scientific Visualization Studio —
  [Deep Star Maps 2020](https://svs.gsfc.nasa.gov/4851) (`milkyway_2020_16k_gal`, 16384×8192, galactic coordinates).
  Public domain.

- **Yale Bright Star Catalogue (BSC5)**: Hoffleit & Warren (1991).
  Retrieved from [CDS Strasbourg](https://cdsarc.cds.unistra.fr/ftp/V/50/). Public domain.

- **天文計算**: [soniakeys/meeus](https://github.com/soniakeys/meeus) (MIT)
