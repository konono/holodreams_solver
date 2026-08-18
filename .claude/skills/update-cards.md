---
name: update-cards
description: HolodoriDB からカードデータを同期して cards.json を更新する
---

# カードデータ更新スキル

HolodoriDB（GitHub: `HolodoriDB/holodori-db-jpn-diff`）をソースとして `data/cards.json` を最新状態に同期する。

## 起動条件

ユーザーが以下のいずれかを言ったときに使う:
- 「カードを追加したい」「新しいカードを入れたい」
- 「カードデータを更新したい」「最新に同期したい」
- 「sync してほしい」

## データソース

- **HolodoriDB**: `https://github.com/HolodoriDB/holodori-db-jpn-diff`
  - ゲーム内マスターデータの差分リポジトリ
  - JSON 形式で Card, Character, Skill 等のテーブルを公開
- **Yagoo-dori**: `https://github.com/asciisyaez/yagoo-dori` (検証用)
  - 別プロジェクトの生成データ。突き合わせ検証に使用

**旧ソース（appmedia）は使用しない。** 以前は appmedia のWebページからスキルデータを手動収集していたが、現在は HolodoriDB から全自動生成に移行済み。

## 更新手順

### 1. 同期スクリプトを実行

```bash
uv run python scripts/sync_holodori.py
```

これにより以下が自動生成される:
- `data/cards.json` — 全星5カードデータ（凸0-4対応、レベルテーブル含む）
- `data/id_map.json` — HolodoriDB ID ↔ HoloSolve ID のマッピング
- `data/songs.json` — 曲データ（再生秒数、スコア係数、コンボテーブル）

オプション: ローカルにクローンした HolodoriDB を使う場合
```bash
git clone https://github.com/HolodoriDB/holodori-db-jpn-diff.git /tmp/holodori-db
uv run python scripts/sync_holodori.py --local /tmp/holodori-db
```

### 2. 差分を確認

```bash
uv run python -c "from solver import load_cards; cards=load_cards(); print(f'{len(cards)} cards loaded')"
```

前回のカード数と比較し、新規追加・削除があれば確認する。

### 3. 検証（任意）

Yagoo-dori との突き合わせ検証:
```bash
uv run python scripts/validate_against_yagoo.py
```

### 4. スタンドアロン版を再ビルド

```bash
uv run python build_static.py
```

### 5. 動作確認

```bash
uv run python app.py
# ブラウザで http://localhost:8000 にアクセスし、新カードが表示されることを確認
```

## sync_holodori.py の処理内容

スクリプトは HolodoriDB の以下のマスターテーブルを取得して変換する:

| テーブル | 用途 |
|---|---|
| Card | カード基本情報（キャラ、属性、ステ配分 permil） |
| CardLevel | レベルごとの基礎パラメータ値 |
| CardPotential | 凸ごとのスキルレベル・ステボーナス |
| Character, CharacterGrouping | キャラ名・所属グループ |
| Costume, LiveLeaderSkill | 衣装スキル（リーダースキル） |
| LiveActiveSkillLevel/Effect | アクティブスキル（センタースキル） |
| LivePassiveSkillLevel/Effect | パッシブスキル（サポートスキル） |
| LiveSpecialSkillLevel | スペシャルスキル |
| LiveSkillEffectTarget, LiveSkillTrigger | スキル条件・対象 |
| Music, LiveCombo | 曲データ・コンボテーブル |
| Lang*_Jpn | 日本語テキスト |

変換ロジック:
- 星5カードのみ抽出（`RARITY_5`）
- 凸0-4の5段階でスキルレベル・ステータスを算出
- ステータスは `ref_stats_lv80`（Lv80基準値）として事前計算
- `param_bonus_permil` で凸によるステボーナスを反映（2凸以上で+100 permil = +10%）
- スキル効果タイプを HoloSolve の内部分類（`type_stat`, `self_all_param` 等）に変換

## HoloSolve ID の命名規則

`sync_holodori.py` が自動生成する。規則:
```
{ローマ字名}_{レアリティ}        — 通常カード（例: tokino_sora_5）
{ローマ字名}_swim_{レアリティ}   — 同キャラ2枚目（例: sakura_miko_swim_5）
```

`id_map.json` で HolodoriDB ID との対応を管理。

## トラブルシューティング

### HolodoriDB にアクセスできない

GitHub raw URL からの取得に失敗する場合:
1. `--local` オプションでローカルクローンを使う
2. ネットワーク・GitHub の可用性を確認

### 新カードのスキルが `unknown` になる

`sync_holodori.py` が未対応のスキルタイプに遭遇した場合、`data/unsupported.json` に記録される。
対応方法:
1. `unsupported.json` の内容を確認
2. `sync_holodori.py` の `classify_passive_effect` や `convert_trigger_to_holosolve` に新しいパターンを追加

### カード数が減った

HolodoriDB 側でカードが削除・非公開になった可能性。`id_map.json` の差分で確認。

## 注意事項

- `sync_holodori.py` は **全カードを再生成** する。個別カードの手動追加は不要
- 手動で `cards.json` を編集した場合、次回の sync で上書きされる
- 凸によるスキル効果変化（1凸: アクティブLv2、3凸: スペシャルLv2、4凸: パッシブLv2）は `CardPotential` テーブルから自動判定
- `variant` フィールド（`"水着"` 等）は同キャラ2枚目以降に自動付与される
