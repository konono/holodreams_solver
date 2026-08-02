---
name: add-card
description: ホロドリ ソルバーに新しい星5カードを追加する。appmediaからデータを収集し、cards.jsonに不整合なく追加する。
---

# 新カード追加スキル

ホロライブドリームス(ホロドリ)の新しい星5カードを `data/cards.json` に追加する手順。

## 起動条件

ユーザーが「カードを追加したい」「新しいガチャのカードを入れたい」等と言ったときに使う。

## データ収集手順

### 1. キャラ一覧ページでカード基本情報を取得

**URL**: https://appmedia.jp/hololive-dreams/80237596

取得項目:
- キャラ名
- カード名
- 総合値、パフォーマンス、テクニック、センス
- アクティブスキル（センタースキル）: 発動間隔(秒)、確率(high/medium/low)、効果時間(秒)、スコアUP%、条件、条件付きスコアUP%
- サポートスキル: 効果タイプ、値、条件、対象

### 2. タイプ（Happy/Pure/Cute）を確認

以下の3ページで星5カードがどのタイプに属するか確認:
- **ピュアタイプ**: https://appmedia.jp/hololive-dreams/80244547
- **キュートタイプ**: https://appmedia.jp/hololive-dreams/80244580
- **ハッピータイプ**: 上記2つに含まれなければハッピー

### 3. 衣装スキル（リーダースキル）を取得

**URL**: https://appmedia.jp/hololive-dreams/80246569

取得項目:
- 発動条件（タイプ2人以上 or グループ2人以上 or なし）
- 効果（全パラ○%UP / 特定ステ○%UP / スコアサポート○%）
- 複合効果の場合は全て記録（例: 全パラ30%UP + スコアサポート25%）

### 4. スペシャルスキルを取得

**URL**: https://appmedia.jp/hololive-dreams/80243005

取得項目:
- スコアサポート効果%
- 効果時間(秒)
- スキル発動率UP%（ある場合）とその条件

## カードIDの命名規則

```
{ローマ字名}_{レアリティ}
```

例:
- `tokino_sora_5` — 通常星5
- `shirogane_noel_swim_5` — 水着（限定）星5

限定カードには `variant` フィールドを追加: `"variant": "水着"` 等

## cards.json のデータ構造

```json
{
  "id": "character_name_5",
  "character": "キャラ名（日本語）",
  "card_name": "カード名",
  "rarity": 5,
  "type": "happy|pure|cute",
  "group": "0期生|1期生|...|ReGLOSS",
  "stats": {
    "performance": 数値,
    "technique": 数値,
    "sense": 数値
  },
  "total": 3ステ合計,
  "center_skill": {
    "interval": 秒数,
    "probability": "high|medium|low",
    "duration": 秒数,
    "score_up": 基本スコアUP%,
    "condition": null | "life_600" | "combo_40" | "pure_2" | "happy_2" | "cute_2",
    "conditional_score_up": 条件付きスコアUP% | null
  },
  "support_skill": {
    "effect_type": "下記参照",
    "value": 数値,
    "condition": null | {"type":"type_count","type_name":"...","min_count":2} | {"type":"group_count","group":"...","min_count":2},
    "target": "self" | {"type_match":"...", "count":2} | {"group":"...", "count":2},
    "stat": "performance|technique|sense" (stat系のみ)
  },
  "costume_skill": {
    "condition": null | {"type":"type_count","type_name":"...","min_count":2} | {"type":"group_count","group":"...","min_count":2},
    "effects": [
      {"target":"all", "stat":"all|performance|technique|sense|score_support", "value": 数値}
    ]
  },
  "special_skill": {
    "duration": 秒数,
    "score_support": スコアサポート効果%,
    "skill_rate_up": 発動率UP% (ある場合のみ),
    "skill_rate_condition": "life_1000|combo_100|group_XXX_2" (ある場合のみ)
  }
}
```

## support_skill の effect_type 一覧

| effect_type | 説明 | 例 |
|---|---|---|
| `type_score_support` | タイプ2人のスコアサポート | ハッピー2人のSS 11% |
| `type_stat` | タイプ2人のステUP（無条件） | ピュア2人のテク 41%UP |
| `type_stat_conditional` | タイプ2人のステUP（条件付き） | ハッピー2人以上でハッピー2人のパフォ 43%UP |
| `type_all_param` | タイプ2人の全パラUP | ピュア2人の全パラ 15%UP |
| `self_all_param` | 自身の全パラUP（条件付き） | 0期生2人以上で自身全パラ 33%UP |
| `self_all_param_conditional` | 同上（別条件） | キュート2人以上で自身全パラ 32%UP |
| `group_stat` | グループ2人のステUP（無条件） | 3期生2人のパフォ 43%UP |
| `group_stat_conditional` | グループ2人のステUP（条件付き） | 1期生2人以上で1期生2人のパフォ 45%UP |
| `group_score_support_conditional` | グループ2人のSS（条件付き） | 3期生2人以上で3期生2人のSS 12% |

## 所属グループ一覧

`0期生`, `1期生`, `2期生`, `ゲーマーズ`, `3期生`, `4期生`, `5期生`, `holoX`, `ID1期生`, `ID2期生`, `ID3期生`, `Myth`, `Promise`, `Advent`, `ReGLOSS`

## 追加後の検証

1. `uv run python -c "from solver import load_cards; cards=load_cards(); print(f'{len(cards)} cards loaded')"` でJSONパースエラーがないか確認
2. 新カードのIDで `solve()` が動作するか確認
3. 衣装スキルの condition/effects が正しい形式か確認

## 注意事項

- appmedia の数値は **0凸（未開花）のmax level** の値。凸で強化されたスキル値ではない
- center_skill の `score_up` は**基本値**（条件なし時の値）。条件付き値は `conditional_score_up` に入れる
- center_skill の `condition` はゲーム状態条件（`life_600`, `combo_40`）とタイプ条件（`pure_2`等）がある。タイプ条件のみスコアボーナス計算で考慮される
- 衣装スキルの stat が `"all"` 以外（`"performance"` 等の単一ステ特化）の場合、ソルバーはそのステのみにバフを適用する（全パラとして扱わない）
- `special_skill` の `skill_rate_up` は任意フィールド。ない場合は省略可
