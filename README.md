# HoloSolve

ホロライブドリームスのカードから最適な5人編成を探索するソルバー。ゲーム内スコア計算式を実測解明し、ユニットスコアを高精度で予測する。

**Web版**: https://konono.github.io/holodreams_solver/

## セットアップ

```bash
# mise で開発環境を一括セットアップ（Go ビルド + Python 依存 + テスト用ブラウザ）
mise run setup

# 開発サーバー起動
mise run dev
# → http://localhost:8000
```

手動セットアップする場合:

```bash
uv sync --group dev
cd solver_go && go build -o solver . && cd ..
uv run python app.py
```

## 使い方

### 1. カード選択

ブラウザで `http://localhost:8000` にアクセスし、手持ちのカードをクリックして選択する。

- 「全選択」「全解除」で一括操作
- Happy / Pure / Cute でタイプフィルタリング
- 凸数（0〜4凸）とレベルをカードごとに指定可能
- 「IDコピー」で選択をJSON配列としてコピー、「ID貼付」で復元

### 2. 最強編成を探す

5枚以上選択して「最強編成を探す」ボタンを押すと、全組み合わせを探索してTop Nを表示する。

結果には以下が表示される:

| 項目 | 説明 |
|---|---|
| ユニットスコア | ゲーム内表示と対応する推定スコア |
| 総合力 | パラメータ + 衣装バフ + サポートバフ |
| SB (スコアボーナス) | アクティブ + 衣装SS + パッシブSS + スペシャル |

### 3. リーダー固定

ドロップダウンで特定のキャラをリーダーに固定して探索できる。「この推しをリーダーにした最強編成は？」という使い方。

### 4. 衣装スイープ

「衣装スイープ」を有効にすると、リーダーの衣装だけ別カードのものを使う組み合わせも含めて探索する。リーダーとして最適なカードと、衣装として最適なカードが異なる場合に有効。

### 5. 曲セレクター

曲を選択すると、その曲の再生秒数でスペシャルスキルの寄与を計算する。デフォルトは192秒。

### 6. レコメンド（次に取るべきカード推薦）

手持ちカードを選択した状態で「レコメンド」ボタンを押すと、次に入手すべきカード（新規取得 or 凸進め）を提案する。

- 全未所持カードおよび凸進め候補を評価
- 取得枚数（1〜5枚）を指定可能
- 現在のベストスコアからの伸び幅（Delta）で順位付け

### 7. キャリブレーション

ゲーム内のユニットスコアとソルバーの予測を合わせるための補正機能。

1. 同じ5人でリーダーだけ変えた2パターンを組む
2. それぞれのゲーム内ユニットスコアを記録
3. 「キャリブレーション」パネルで2つのリーダーとスコアを入力
4. `stat_scale`(レベル補正) と `baseline`(ボード等の定数) が算出される

補正値は `localStorage` に保存され、以降の計算に自動適用される。レベルやボード育成が変わったら再キャリブレーション。

## Go ソルバー CLI

Go ソルバーは stdin から JSON を受け取り、stdout に JSON を出力するCLIツール。

```bash
cd solver_go && go build -o solver .
```

### solve — 最強編成探索

```bash
echo '{
  "action": "solve",
  "cards": ["hoshimachi_suisei_1", "shirakami_fubuki_1", "tokino_sora_1", "sakura_miko_1", "houshou_marine_1"],
  "top_n": 3
}' | ./solver_go/solver
```

カードIDの代わりに凸・レベル指定も可能:

```bash
echo '{
  "action": "solve",
  "cards": [
    {"id": "hoshimachi_suisei_1", "potential": 2, "level": 70},
    {"id": "shirakami_fubuki_1", "potential": 0}
  ],
  "top_n": 5,
  "stat_scale": 0.85,
  "baseline": 5000
}' | ./solver_go/solver
```

オプション:
- `top_n`: 上位N件を返す（デフォルト: 10）
- `stat_scale`, `baseline`: キャリブレーション補正値
- `fixed_leader_id`: リーダーを固定
- `costume_only_leader_id`: 衣装だけ別カードのものを使用
- `song_length`: 曲の再生秒数（デフォルト: 192）
- `stability_lengths`: 複数の曲長でスコアを計算し安定性を評価
- `sweep_costumes`: 全衣装候補を自動探索

### recommend — カード推薦

```bash
echo '{
  "action": "recommend",
  "cards": [
    {"id": "hoshimachi_suisei_1", "potential": 0},
    {"id": "shirakami_fubuki_1", "potential": 1}
  ],
  "top_n": 5,
  "acquire_count": 1
}' | ./solver_go/solver
```

オプション:
- `acquire_count`: 同時に取得するカード枚数（デフォルト: 1）
- `sweep_costumes`: 衣装スイープを有効化

### calibrate — キャリブレーション

```bash
echo '{
  "action": "calibrate",
  "member_ids": ["card_a", "card_b", "card_c", "card_d", "card_e"],
  "leader_id_1": "card_a",
  "game_score_1": 678413,
  "leader_id_2": "card_b",
  "game_score_2": 642056
}' | ./solver_go/solver
```

出力: `{"stat_scale": 0.85, "baseline": 5000}`

## API Client CLI

FastAPI サーバーに対する Go 製のコマンドラインクライアント。UIの「IDコピー」で出力される JSON を設定ファイルとしてそのまま使える。

```bash
# ビルド
mise run build:cli

# UI の「IDコピー」で取得した JSON を設定ファイルに保存
pbpaste | ./cli/holosolve init

# 最強編成探索
./cli/holosolve solve

# カード推薦
./cli/holosolve recommend

# リーダー固定で探索
./cli/holosolve solve --leader houshou_marine_5

# キャリブレーション（結果を設定ファイルに保存）
./cli/holosolve calibrate \
  --members id1,id2,id3,id4,id5 \
  --leader1 id1 --score1 678413 \
  --leader2 id2 --score2 642056 \
  --save

# カード一覧
./cli/holosolve cards

# JSON 出力
./cli/holosolve solve --json
```

設定ファイル（`holosolve.json`）にはカード選択、凸数、レベル、キャリブレーション値が含まれる。`--config path` で別ファイルも指定可能。

## ファイル構成

```
holodre_sim/
├── cli/                      # API Client CLI (Go)
├── data/
│   ├── cards.json            # カードデータベース（70枚、0-4凸対応）
│   ├── songs.json            # 曲データ（194曲）
│   └── id_map.json           # HolodoriDB ID ↔ HoloSolve ID マッピング
├── solver_go/                # Go ソルバー（CLI + WASM）
│   ├── main.go               # CLI エントリポイント
│   ├── solver.go             # コアロジック
│   ├── evaluate.go           # スコア評価
│   ├── resolve.go            # カード解決・凸/レベル適用
│   ├── parse.go              # 入力パース
│   ├── types.go              # 型定義
│   ├── wasm.go               # WASM 版エントリポイント
│   └── recommend_test.go     # Go テスト
├── solver.py                 # Python 参照実装
├── solver_go_bridge.py       # Go CLI の Python ラッパー
├── app.py                    # FastAPI サーバー
├── index.html                # フロントエンド UI
├── build_static.py           # WASM 版 HTML 生成
├── wasm_bridge.js            # WASM Web Worker
├── wasm_exec.js              # Go 標準 WASM ブリッジ
├── scripts/
│   ├── sync_holodori.py      # HolodoriDB からデータ生成
│   └── validate_against_yagoo.py  # 検算スクリプト
├── test_solver.py            # ソルバーユニットテスト
├── test_e2e.py               # E2E テスト
├── test_recommend_ui.py      # レコメンド UI テスト（Playwright）
├── test_static_build.py      # 静的ビルドテスト
├── mise.toml                 # タスクランナー・ツールバージョン管理
├── docs/scoring-model.md     # スコア計算モデル技術資料
└── CLAUDE.md                 # Claude Code 用プロジェクト設定
```

## カードデータの更新

HolodoriDB（GitHub: `HolodoriDB/holodori-db-jpn-diff`）から自動生成する:

```bash
uv run python scripts/sync_holodori.py
```

GitHub Actions (`update-cards.yml`) による定期自動更新もある。

## テスト

```bash
# 全テスト
mise run test

# ユニットテストのみ
mise run test:unit

# E2Eテストのみ
mise run test:e2e

# Go テスト
cd solver_go && go test -v
```

Python テスト 81件 + Go テスト 8件。実測値ベースの回帰テスト、キャリブレーション精度、レコメンド機能、Playwright による UI テストを含む。

## 静的 HTML ビルド

GitHub Pages 用の WASM 版を生成できる。カードデータを埋め込んだ HTML + Go WASM ソルバーで動作する。

```bash
# Go ソルバーの WASM ビルド（初回のみ）
mise run build:solver

# HTML + WASM ファイル生成
mise run build
# → dist/index.html, dist/solver.wasm, dist/wasm_bridge.js, dist/wasm_exec.js
```

WASM 版は HTTP サーバー経由で配信する必要がある（`file://` では動作しない）。GitHub Pages にデプロイするか、ローカルで `python -m http.server -d dist` で確認できる。

## ベンチマーク

```bash
# Go ベンチマーク実行（結果を benchmarks/ に保存、benchstat で比較可能）
mise run bench

# ネイティブ vs WASM パフォーマンス比較（10分以上かかることがあります）
mise run bench:wasm
```

## 技術資料

スコア計算モデルの解明過程と数式の詳細は [docs/scoring-model.md](docs/scoring-model.md) を参照。
