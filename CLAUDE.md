# HoloSolve — ホロドリ編成オプティマイザ

## プロジェクト概要

ホロライブドリームス(ホロドリ)のカードから最適な5人編成を探索するソルバー。
ゲーム内スコア計算式を実測解明し、ユニットスコアを高精度で予測する。
FastAPI + HTML のWebアプリケーション + Go製CLIソルバー + WASM版スタンドアロンHTML。

主な機能:
- **最強編成探索** (solve): 手持ちカードから全組み合わせを探索し Top N を表示
- **カード推薦** (recommend): 次に入手すべきカード（新規取得 or 凸進め）を提案
- **キャリブレーション** (calibrate): ゲーム内スコアとの2点補正
- **曲セレクター**: 曲ごとの再生秒数でスペシャルスキル計算を調整
- **衣装スイープ**: 全衣装候補を自動探索して最適な衣装を選択
- **安定性チェック**: 複数の曲長でスコアの安定性を評価
- **凸・レベル指定**: 0〜4凸、任意レベルでの計算に対応

## 技術スタック

- **ランタイム管理**: mise (Go 1.24, Python 3.13, Node 22, uv)
- **ソルバー**: Go (CLI バイナリ + WASM)。Python版は参照実装
- **Webサーバー**: FastAPI + uvicorn（Go CLI を subprocess 経由で呼び出し）
- **フロントエンド**: バニラ HTML/CSS/JS（単一 `index.html`）
- **データソース**: HolodoriDB (`HolodoriDB/holodori-db-jpn-diff`) から自動生成
- **テスト**: pytest (ユニット + E2E + Playwright) + Go テスト
- **CI**: GitHub Actions (テスト + GitHub Pages デプロイ)

## 起動方法

```bash
# 初回セットアップ（依存 + Goソルバービルド + ブラウザインストール）
mise run setup

# 開発サーバー起動
mise run dev
# → http://localhost:8000
```

## mise タスク

| タスク | 説明 |
|---|---|
| `mise run setup` | 開発環境セットアップ（依存 + ソルバー + ブラウザ） |
| `mise run dev` | 開発サーバー起動 |
| `mise run test` | 全テスト実行 |
| `mise run test:unit` | ユニットテストのみ |
| `mise run test:e2e` | E2Eテストのみ |
| `mise run build` | スタンドアロン版HTML生成 |
| `mise run build:solver` | Goソルバービルド（CLI + WASM） |

## ファイル構成

- `data/cards.json` — HolodoriDBから自動生成（70枚、0-4凸対応、レベルテーブル付き）
- `data/songs.json` — 曲データ（194曲、再生秒数・スコア係数）
- `data/id_map.json` — HolodoriDB ID ↔ HoloSolve IDマッピング
- `scripts/sync_holodori.py` — HolodoriDBからデータ生成
- `scripts/validate_against_yagoo.py` — Yagoo-dori生成データとの検算
- `solver.py` — スコア計算エンジン（Python参照実装）
- `solver_go/` — Goソルバー（CLI + WASM、本番計算はこちらを使用）
  - `main.go` — CLIエントリポイント（stdin JSON → stdout JSON）
  - `solver.go` — solve/calibrate/recommend のコアロジック
  - `evaluate.go` — スコア評価関数
  - `resolve.go` — カード解決・凸/レベル適用
  - `parse.go` — CLI入力パース・アクションディスパッチ
  - `types.go` — 型定義
  - `wasm.go` — WASM版エントリポイント
  - `recommend_test.go` — Goテスト
- `solver_go_bridge.py` — Go CLIをsubprocess経由で呼ぶPythonラッパー
- `app.py` — FastAPI サーバー（solver_go_bridge経由でGoソルバーを使用）
- `index.html` — フロントエンドUI（開発用、サーバー版）
- `build_static.py` — WASM版HTML生成（GitHub Pages用）
- `wasm_bridge.js` — Web Worker内でWASMソルバーを実行
- `wasm_exec.js` — Go標準WASMブリッジ
- `docs/scoring-model.md` — スコア計算モデル技術資料
- `mise.toml` — タスクランナー定義・ツールバージョン管理
- `test_solver.py` — ソルバーユニットテスト
- `test_e2e.py` — E2Eテスト
- `test_recommend_ui.py` — レコメンドUIテスト（Playwright）
- `test_static_build.py` — 静的ビルドテスト

## スコア計算モデル

詳細は [docs/scoring-model.md](docs/scoring-model.md) を参照。

```
ユニットスコア = 総合力 × (1 + スコアボーナス%) × 2.037

総合力 = メンバーパラメータ + 衣装スキル + パッシブスキル + baseline

スコアボーナス = アクティブスキル% + 衣装SS% + パッシブSB% + スペシャルスキル%
```

### アクティブスキル（Expected Maximum モデル）

5人のうち最強のActiveのみがスコアに貢献する。発動率・SP発動率UPも考慮:

```
uptime = min(1, duration / interval × boosted_probability)
active_pct = 52.89 + E[max(score_up × uptime)] / 12.82
```

### center_skill の条件

タイプ条件のみ計算に反映。ライフ/コンボ条件は基本値を使用。

## Go ソルバー CLI

Go ソルバーは stdin から JSON を受け取り、stdout に JSON を出力する:

```bash
echo '{"action":"solve","cards":["card_id1","card_id2",...],"top_n":5}' | ./solver_go/solver
```

3つのアクション（`solve`, `recommend`, `calibrate`）に対応。詳細は README.md を参照。

## API Client CLI

FastAPI サーバーに対する Go 製コマンドラインクライアント（`cli/`）。UIの「IDコピー」JSON を設定ファイル（`holosolve.json`）として保存し、`solve`, `recommend`, `calibrate`, `cards` サブコマンドで API を叩く。ビルドは `mise run build:cli`。詳細は README.md を参照。

## カードデータ更新

`/update-cards` スキルを使用。HolodoriDB（GitHub: `HolodoriDB/holodori-db-jpn-diff`）から `scripts/sync_holodori.py` で全カードを自動生成する。
詳細は `.claude/skills/update-cards.md` を参照。

GitHub Actions (`update-cards.yml`) による定期自動更新もある。

## キャリブレーション

ユーザーのレベル・ボード育成状況に合わせるため、同じ5人でリーダー違いの2つのユニットスコアを入力して `stat_scale` と `baseline` を算出する2点キャリブレーション機能がある。
