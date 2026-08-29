# HoloSolve

ホロライブドリームスの星5カードから最適な5人編成を探索するソルバー。ゲーム内スコア計算式を実測解明し、ユニットスコアを高精度で予測する。

**Web版**: https://konono.github.io/holodreams_solver/

## セットアップ

```bash
# 依存インストール
uv sync

# 起動
uv run python app.py
# → http://localhost:8000
```

## 使い方

### 1. カード選択

ブラウザで `http://localhost:8000` にアクセスし、手持ちの星5カードをクリックして選択する。

- 「全選択」「全解除」で一括操作
- Happy / Pure / Cute でタイプフィルタリング
- 「IDコピー」で選択をJSON配列としてコピー、「ID貼付」で復元

### 2. 最強編成を探す

5枚以上選択して「最強編成を探す」ボタンを押すと、全組み合わせを探索してTop 10を表示する。サーバー版は**最大40枚**まで。全カードで探索する場合は静的版（`dist/index.html`）を使う。

結果には以下が表示される:

| 項目 | 説明 |
|---|---|
| ユニットスコア | ゲーム内表示と対応する推定スコア |
| 総合力 | パラメータ + 衣装バフ + サポートバフ |
| SB (スコアボーナス) | アクティブ + 衣装SS + パッシブSS + スペシャル |

### 3. リーダー固定

ドロップダウンで特定のキャラをリーダーに固定して探索できる。「この推しをリーダーにした最強編成は？」という使い方。

### 4. キャリブレーション

ゲーム内のユニットスコアとソルバーの予測を合わせるための補正機能。

1. 同じ5人でリーダーだけ変えた2パターンを組む
2. それぞれのゲーム内ユニットスコアを記録
3. 「キャリブレーション」パネルで2つのリーダーとスコアを入力
4. `stat_scale`(レベル補正) と `baseline`(ボード等の定数) が算出される

補正値は `localStorage` に保存され、以降の計算に自動適用される。レベルやボード育成が変わったら再キャリブレーション。

## ファイル構成

```
holodre_sim/
├── data/cards.json          # 星5カードデータベース（59枚）
├── solver.py                # スコア計算エンジン
├── app.py                   # FastAPI サーバー
├── index.html               # フロントエンド UI
├── CLAUDE.md                # Claude Code 用プロジェクト設定
├── .claude/skills/add-card.md  # カード追加スキル
└── docs/scoring-model.md    # スコア計算モデル技術資料
```

## カードの追加

新しいガチャでカードが追加された場合、appmedia から以下の4ページを参照してデータを収集する:

1. [キャラ一覧](https://appmedia.jp/hololive-dreams/80237596) — 基本ステ・アクティブ・サポートスキル
2. [タイプ別一覧](https://appmedia.jp/hololive-dreams/80244547) — Happy/Pure/Cute 分類
3. [衣装スキル一覧](https://appmedia.jp/hololive-dreams/80246569) — リーダースキル
4. [スキル検索](https://appmedia.jp/hololive-dreams/80243005) — スペシャルスキル

詳細なデータ構造と追加手順は `.claude/skills/add-card.md` を参照。

## テスト

```bash
uv run pytest
```

実測値ベースの回帰テスト（11件）。キャリブレーション精度、スコアボーナス内訳、パフォ135% > 全パラ50% の逆転、リーダー固定、同キャラ排除を検証。

## 静的 HTML ビルド

GitHub Pages 用の WASM 版を生成できる。カードデータを埋め込んだ HTML + Go WASM ソルバーで動作する。

```bash
# Go ソルバーのビルド（初回のみ）
cd solver_go && go build -o solver . && GOOS=js GOARCH=wasm go build -o solver.wasm . && cd ..

# HTML + WASM ファイル生成
uv run python build_static.py
# → dist/index.html, dist/solver.wasm, dist/wasm_bridge.js, dist/wasm_exec.js
```

WASM 版は HTTP サーバー経由で配信する必要がある（`file://` では動作しない）。GitHub Pages にデプロイするか、ローカルで `python -m http.server -d dist` で確認できる。キャリブレーション機能は含まれない（サーバー版のみ）。

## 技術資料

スコア計算モデルの解明過程と数式の詳細は [docs/scoring-model.md](docs/scoring-model.md) を参照。
