---
name: generate-charts
description: ゲームアセットからチャートデータ (data/charts.json) を生成する
---

# チャートデータ生成スキル

ゲーム CDN からチャート（譜面）アセットを取得し、Timeline Engine 用の `data/charts.json` を生成する。

## 起動条件

ユーザーが以下のいずれかを言ったときに使う:
- 「チャートデータを更新したい」「譜面データを取得したい」
- 「charts.json を生成したい」
- 「Timeline Engine 用のデータがほしい」

## 注意: data/charts.json はリポジトリに含めない

チャートデータはゲームアセット（CDN から暗号化配信される譜面ファイル）を復号・パースしたもの。
マスターデータ（カード定義等）とは異なり、譜面は創作物としての著作権が強い。
HolodoriDB も `Ai0796/Holodori-sus` も譜面データ自体はリポジトリに含めていない。

- `data/charts.json` は `.gitignore` に登録済み
- ローカル環境でのみ生成・利用する
- CI/CD では charts.json が不要なテスト構成にする

## 前提条件

### 必要なツール

```bash
pip install holodori-asset-tools  # ゲームアセット取得・復号
pip install sonolus-level-converters  # SUS ファイルパーサー
```

`holodori-asset-tools` は C コンパイラ（gcc/g++）が必要（UnityPy のビルド）。

### ツールの役割

| ツール | リポジトリ | 役割 |
|---|---|---|
| holodori-asset-tools | `HolodoriDB/holodori-asset-tools` | ゲーム CDN からアセットを DL・復号 |
| sonolus-level-converters | `UntitledCharts/sonolus-level-converters` | SUS 譜面フォーマットのパース |
| generate_charts.py | 本リポジトリ `scripts/` | パース結果を charts.json に変換 |

## 生成手順

### 1. SUS ファイルをゲーム CDN から取得

```bash
python3 -m holodori_asset_tools download /tmp/holodori_assets --filter 'chart_m' --workers 4
```

- ゲーム CDN (`asset.game-hololive-dreams.com`) から暗号化アセットを DL
- `chart_m` でフィルタしてチャートアセットのみ取得（約 776 ファイル）
- 自動的に復号して `/tmp/holodori_assets/resources/chart_*.sus` として保存
- `--no-overwrite` で既存ファイルをスキップ可能

初回は数分かかる。2 回目以降は `--no-overwrite` で差分のみ。

### 2. 公開用ビン集約データを生成

```bash
python3 scripts/generate_charts.py /tmp/holodori_assets/resources/ \
  --output data/chart_scores.json --bin-size 0.5
```

これが `data/chart_scores.json`（リポジトリに含まれる公開可能データ）。
0.5 秒ビンに集約されており、個々のノート位置は復元不可能。

### 3. （オプション）ローカル用フルデータを生成

```bash
python3 scripts/generate_charts.py /tmp/holodori_assets/resources/ \
  --output data/charts.json
```

`data/charts.json` は `.gitignore` 対象。個別ノートのタイムスタンプを含む。

### 4. 確認

```bash
python3 -c "
import json
with open('data/chart_scores.json') as f:
    charts = json.load(f)
print(f'Charts: {len(charts)}')
print(f'Songs: {len(set(v[\"music_id\"] for v in charts.values()))}')
c = charts.get('m0001_expert', {})
print(f'SP points: {c.get(\"special_points\", [])}')
print(f'Bins: {len(c.get(\"bins\", []))}')
print(f'Notes: {c.get(\"total_notes\", 0)}')
"
```

## データフロー

```
ゲーム CDN (asset.game-hololive-dreams.com)
    │  暗号化アセットバンドル
    ▼
holodori-asset-tools download --filter 'chart_m'
    │  復号済み SUS ファイル (chart_*.sus)
    ▼
scripts/generate_charts.py
    │  sonolus-level-converters で SUS パース
    │  BPM 変化対応の beat→秒変換
    │  ノートタイプ別ウェイト付与
    ▼
data/charts.json (ローカルのみ、git 管理外)
    │
    ▼
Timeline Engine (solver_go/timeline_engine.go)
```

## charts.json のデータ構造

```json
{
  "m0001_expert": {
    "music_id": "m0001",
    "difficulty": "expert",
    "duration": 100,
    "bpm": 163.0,
    "total_notes": 454,
    "special_points": [13.2515, 32.3926, 48.589, 64.7853, 80.9816],
    "notes": [
      {"time": 0.9202, "weight": 1050, "combo_index": 1},
      {"time": 1.1043, "weight": 1050, "combo_index": 2},
      ...
    ]
  }
}
```

| フィールド | 説明 |
|---|---|
| `music_id` | 曲 ID（songs.json と対応） |
| `difficulty` | easy / normal / hard / expert |
| `duration` | 曲長（秒、songs.json から） |
| `bpm` | 基本 BPM |
| `total_notes` | 総ノート数 |
| `special_points` | SP 発動 5 地点のタイムスタンプ（秒） |
| `notes[].time` | ノートのタイムスタンプ（秒） |
| `notes[].weight` | ノートのスコアウェイト |
| `notes[].combo_index` | コンボ番号（1 始まり） |

### ノートウェイト（LiveNote.json 準拠）

| タイプ | weight | ソース |
|---|---|---|
| tap | 1000 | `scoreCoefficientPermilMultiply = 1000` |
| flick | 1050 | `scoreCoefficientPermilMultiply = 1050` |
| slide_start | 1000 | |
| slide_end | 1000 | |
| slide_relay | 100 | |
| slide_continue | 100 | |

## SUS ファイルフォーマット（参考）

SUS (Sliding Universal Score) はリズムゲームの譜面フォーマット。

- ヘッダ: `#BASEBPM`, `#MEASURE_COUNT`, `#FULL_COMBO_NOTE_COUNT` 等
- BPM 定義: `#BPM01:163`
- ノート: チャンネル別（タップ、スライド、フリック等）
- SP スキル: `#xxxB:0N` チャンネル（N = スロット番号 1-5）
- Fever: `#xxxD:0N` チャンネル

パースは `sonolus_converters.holodori_sus.load()` が担当。

## トラブルシューティング

### holodori-asset-tools のインストールに失敗する

UnityPy が C 拡張を含むため、gcc/g++ が必要:
```bash
# Debian/Ubuntu
sudo apt-get install gcc g++
# Fedora
sudo dnf install gcc gcc-c++
```

### ゲーム CDN からの DL に失敗する

- ネットワーク確認
- `holodori-app-protos` のバージョン情報が古い場合がある（稀）
- `--catalog` オプションでカタログ JSON を手動指定可能

### パースエラーが出る

一部の SUS ファイルでパーサーが非対応のノートタイプに遭遇する場合がある。
`generate_charts.py` はエラーをスキップして処理を継続する。

### ファイルサイズが大きい

全難易度含めると約 17MB。Expert のみにする場合:
```bash
python3 scripts/generate_charts.py /tmp/holodori_assets/resources/ \
  --output data/charts.json --filter expert
```
（`--filter` オプションは未実装。必要なら追加する）

## 関連リポジトリ

| リポジトリ | 用途 |
|---|---|
| [HolodoriDB/holodori-asset-tools](https://github.com/HolodoriDB/holodori-asset-tools) | アセット取得・復号ツール |
| [HolodoriDB/holodori-scores](https://github.com/HolodoriDB/holodori-scores) | SUS レンダラー（チャート画像生成） |
| [UntitledCharts/sonolus-level-converters](https://github.com/UntitledCharts/sonolus-level-converters) | SUS パーサー |
| [Ai0796/Holodori-sus](https://github.com/Ai0796/Holodori-sus) | SUS 分析ツール・music_meta.json 生成（参考実装） |
| [HolodoriDB/holodori-db-jpn-diff](https://github.com/HolodoriDB/holodori-db-jpn-diff) | LiveNote.json（ウェイト定義）、LiveCombo.json（コンボボーナス） |
