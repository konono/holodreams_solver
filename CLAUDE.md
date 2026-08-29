# HoloSolve — ホロドリ編成オプティマイザ

## プロジェクト概要

ホロライブドリームス(ホロドリ)の手持ちカードから最適な5人編成を探索するソルバー。
FastAPI + HTML のWebアプリケーション。

## 起動方法

```bash
# 初回: Goソルバーのビルドが必要
cd solver_go && go build -o solver . && cd ..

uv run python app.py
# → http://localhost:8000
```

## ファイル構成

- `data/cards.json` — HolodoriDBから自動生成（61枚、0-4凸対応）
- `data/songs.json` — 曲データ（再生秒数、スコア係数）
- `data/id_map.json` — HolodoriDB ID ↔ HoloSolve IDマッピング
- `scripts/sync_holodori.py` — HolodoriDBからデータ生成
- `scripts/validate_against_yagoo.py` — Yagoo-dori生成データとの検算
- `solver.py` — スコア計算エンジン（Python参照実装）
- `solver_go/` — Goソルバー（CLI + WASM、本番計算はこちらを使用）
- `solver_go_bridge.py` — Go CLIをsubprocess経由で呼ぶPythonラッパー
- `app.py` — FastAPI サーバー（solver_go_bridge経由でGoソルバーを使用）
- `index.html` — フロントエンドUI（開発用、サーバー版）
- `build_static.py` — WASM版HTML生成（GitHub Pages用）
- `wasm_bridge.js` — Web Worker内でWASMソルバーを実行
- `wasm_exec.js` — Go標準WASMブリッジ

## スコア計算モデル

```
ユニットスコア = 総合力 × (1 + スコアボーナス%) × 2.037

総合力:
  メンバーパラメータ（5人の基礎ステ合計）
  + 衣装スキル（リーダー衣装のパラバフ）
  + パッシブスキル（サポートスキルのステバフ）
  + baseline（キャリブレーションで吸収）

スコアボーナス:
  アクティブスキル% = 52.89 + E[max(score_up × uptime)] / 12.82
    uptime = min(1, duration / interval × boosted_probability)
    boosted_probability = base_prob + Σ(SP発動率UP × SP継続 / 曲長)
    Expected Maximum: 5人のうち最強のActiveのみがスコアに貢献
  衣装SS%          = 衣装スコアサポート × 68%
  パッシブSB%       = サポートSS合計 × 20%
  スペシャルスキル%  = Σ(SS効果% × 効果秒数) / 曲長(デフォルト192秒)
```

### 重要な発見

- パフォ135%UPは全パラ50%UPより強い（パフォ比率37%超のチームで逆転）
- SSの効果は衣装SS(×0.68)とサポートSS(×0.20)で変換率が大きく異なる
- center_skillの条件はタイプ条件のみ計算に反映。ライフ/コンボ条件は基本値を使用
- データはappmediaの0凸max level値。凸カードは実測で約2%の誤差

## カードデータ更新

`/update-cards` スキルを使用。HolodoriDB（GitHub: `HolodoriDB/holodori-db-jpn-diff`）から `scripts/sync_holodori.py` で全カードを自動生成する。
詳細は `.claude/skills/update-cards.md` を参照。

## キャリブレーション

ユーザーのレベル・ボード育成状況に合わせるため、同じ5人でリーダー違いの2つのユニットスコアを入力して `stat_scale` と `baseline` を算出する2点キャリブレーション機能がある。
