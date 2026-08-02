"""HoloSolve — ホロライブドリームス編成オプティマイザ

実測解明済みスコア計算モデル:
  ユニットスコア = 総合力 × (1 + スコアボーナス%) × 2.037

  総合力 = メンバーパラ × (1 + 衣装バフ率) + サポートステバフ + ベースライン
  スコアボーナス = アクティブ% + 衣装SS% + パッシブSB% + スペシャル%
    アクティブ%  = 52.89 + Σ(score_up × duration / interval) / 12.82
    衣装SS%      = 衣装スコアサポート × 0.68
    パッシブSB%  = サポートSS合計 × 0.20
    スペシャル%  = Σ(SP_score_support × SP_duration) / 192
"""

import json
from functools import lru_cache
from itertools import combinations, permutations
from pathlib import Path

UNIT_SCORE_K = 2.037
ACTIVE_BASE = 52.89
ACTIVE_DIVISOR = 12.82
COSTUME_SS_RATE = 0.68
SUPPORT_SS_RATE = 0.20
SONG_LENGTH = 192


@lru_cache(maxsize=1)
def load_cards() -> tuple:
    data_path = Path(__file__).parent / "data" / "cards.json"
    with open(data_path, encoding="utf-8") as f:
        return tuple(json.load(f)["cards"])


def count_types(team):
    counts = {"happy": 0, "pure": 0, "cute": 0}
    for card in team:
        counts[card["type"]] += 1
    return counts


def count_groups(team):
    counts = {}
    for card in team:
        g = card["group"]
        counts[g] = counts.get(g, 0) + 1
    return counts


def check_condition(condition, type_counts, group_counts):
    if condition is None:
        return True
    ctype = condition.get("type")
    if ctype == "type_count":
        return type_counts.get(condition["type_name"], 0) >= condition["min_count"]
    if ctype == "group_count":
        return group_counts.get(condition["group"], 0) >= condition["min_count"]
    return False


def _check_center_type_condition(condition, type_counts, group_counts):
    """センタースキルの条件チェック。タイプ条件のみ適用、ゲーム状態条件(ライフ/コンボ)は不適用"""
    if condition is None:
        return False
    if condition in ("life_600", "combo_40"):
        return False
    if condition.endswith("_2"):
        type_name = condition[:-2]
        return type_counts.get(type_name, 0) >= 2
    return False


def _compute_team_scores(team, leader_idx):
    leader = team[leader_idx]
    type_counts = count_types(team)
    group_counts = count_groups(team)

    # === 総合力: 衣装バフ（ステごとに正確に計算） ===
    costume_perf_rate = 0.0
    costume_tech_rate = 0.0
    costume_sense_rate = 0.0
    costume_ss = 0.0
    cs = leader["costume_skill"]
    if check_condition(cs.get("condition"), type_counts, group_counts):
        for effect in cs["effects"]:
            val = effect["value"] / 100.0
            stat = effect["stat"]
            if stat == "score_support":
                costume_ss += val
            elif stat == "all":
                costume_perf_rate += val
                costume_tech_rate += val
                costume_sense_rate += val
            elif stat == "performance":
                costume_perf_rate += val
            elif stat == "technique":
                costume_tech_rate += val
            elif stat == "sense":
                costume_sense_rate += val

    # === 総合力: サポートステバフ ===
    support_bonus = [0.0] * 5
    support_ss = 0.0

    for idx, card in enumerate(team):
        ss = card["support_skill"]
        etype = ss["effect_type"]
        condition = ss.get("condition")

        if not check_condition(condition, type_counts, group_counts):
            continue

        if etype in ("self_all_param", "self_all_param_conditional"):
            val = ss["value"] / 100.0
            s = card["stats"]
            support_bonus[idx] += (s["performance"] + s["technique"] + s["sense"]) * val

        elif etype in ("type_stat", "type_stat_conditional"):
            val = ss["value"] / 100.0
            stat = ss["stat"]
            target_type = ss["target"]["type_match"]
            applied = 0
            for i, c in enumerate(team):
                if c["type"] == target_type and applied < ss["target"]["count"]:
                    support_bonus[i] += c["stats"].get(stat, 0) * val
                    applied += 1

        elif etype == "type_all_param":
            val = ss["value"] / 100.0
            target_type = ss["target"]["type_match"]
            applied = 0
            for i, c in enumerate(team):
                if c["type"] == target_type and applied < ss["target"]["count"]:
                    s = c["stats"]
                    support_bonus[i] += (s["performance"] + s["technique"] + s["sense"]) * val
                    applied += 1

        elif etype == "type_score_support":
            target_type = ss["target"]["type_match"]
            required = ss["target"].get("count", 2)
            if type_counts.get(target_type, 0) >= required:
                support_ss += ss["value"] / 100.0

        elif etype in ("group_stat", "group_stat_conditional"):
            val = ss["value"] / 100.0
            stat = ss["stat"]
            target_group = ss["target"]["group"]
            applied = 0
            for i, c in enumerate(team):
                if c["group"] == target_group and applied < ss["target"]["count"]:
                    support_bonus[i] += c["stats"].get(stat, 0) * val
                    applied += 1

        elif etype == "group_score_support_conditional":
            target_group = ss["target"]["group"]
            required = ss["target"].get("count", 2)
            if group_counts.get(target_group, 0) >= required:
                support_ss += ss["value"] / 100.0

    total_ss = costume_ss + support_ss

    # === スコアボーナス: アクティブスキル% ===
    active_sum = 0.0
    for card in team:
        cs_card = card["center_skill"]
        score_up = cs_card["score_up"]
        cond = cs_card.get("condition")
        if cond and _check_center_type_condition(cond, type_counts, group_counts):
            score_up = cs_card.get("conditional_score_up", score_up)
        active_sum += score_up * cs_card["duration"] / cs_card["interval"]

    active_pct = ACTIVE_BASE + active_sum / ACTIVE_DIVISOR

    # === スコアボーナス: 衣装SS% + パッシブSB% ===
    costume_sb_pct = costume_ss * 100 * COSTUME_SS_RATE
    passive_sb_pct = support_ss * 100 * SUPPORT_SS_RATE

    # === スコアボーナス: スペシャルスキル% ===
    special_pct = sum(
        c["special_skill"]["score_support"] * c["special_skill"]["duration"] / SONG_LENGTH
        for c in team
        if "special_skill" in c
    )

    score_bonus = active_pct + costume_sb_pct + passive_sb_pct + special_pct

    return {
        "costume_perf_rate": costume_perf_rate,
        "costume_tech_rate": costume_tech_rate,
        "costume_sense_rate": costume_sense_rate,
        "support_bonus_total": sum(support_bonus),
        "active_pct": active_pct,
        "costume_sb_pct": costume_sb_pct,
        "passive_sb_pct": passive_sb_pct,
        "special_pct": special_pct,
        "score_bonus": score_bonus,
        "costume_ss": costume_ss,
        "support_ss": support_ss,
    }


def evaluate_team(team, leader_idx, stat_scale=1.0, baseline=0):
    scores = _compute_team_scores(team, leader_idx)

    total_perf = sum(c["stats"]["performance"] for c in team) * stat_scale
    total_tech = sum(c["stats"]["technique"] for c in team) * stat_scale
    total_sense = sum(c["stats"]["sense"] for c in team) * stat_scale
    member_params = total_perf + total_tech + total_sense

    costume_contrib = (
        total_perf * scores["costume_perf_rate"]
        + total_tech * scores["costume_tech_rate"]
        + total_sense * scores["costume_sense_rate"]
    )
    support_contrib = scores["support_bonus_total"] * stat_scale
    total_power = member_params + costume_contrib + support_contrib + baseline

    score_bonus_pct = scores["score_bonus"]
    unit_score = total_power * (1 + score_bonus_pct / 100) * UNIT_SCORE_K

    return {
        "unit_score": unit_score,
        "total_power": total_power,
        "member_params": member_params,
        "costume_contrib": costume_contrib,
        "support_contrib": support_contrib,
        "active_pct": scores["active_pct"],
        "costume_sb_pct": scores["costume_sb_pct"],
        "passive_sb_pct": scores["passive_sb_pct"],
        "special_pct": scores["special_pct"],
        "score_bonus": score_bonus_pct,
        "costume_ss": scores["costume_ss"],
        "support_ss": scores["support_ss"],
    }


def optimize_order(team_ids: list[str], leader_id: str, stat_scale: float = 1.0, baseline: float = 0) -> dict:
    """リーダーを固定し、残り4人の配置順を全24通り試して最適な並びを返す"""
    all_cards = load_cards()
    card_map = {c["id"]: c for c in all_cards}

    leader_card = card_map[leader_id]
    others = [card_map[cid] for cid in team_ids if cid != leader_id]

    best = None
    best_order = None
    for perm in permutations(range(len(others))):
        team = [leader_card] + [others[i] for i in perm]
        score = evaluate_team(team, 0, stat_scale, baseline)
        if best is None or score["unit_score"] > best["unit_score"]:
            best = score
            best_order = [leader_id] + [others[i]["id"] for i in perm]

    best["leader_idx"] = 0
    best["team_ids"] = best_order
    return best


def _optimize_results(results: list[dict], card_map: dict, stat_scale: float, baseline: float) -> list[dict]:
    """Top 結果の配置順を最適化する"""
    optimized = []
    for r in results:
        leader_id = r["team_ids"][r["leader_idx"]]
        leader_card = card_map[leader_id]
        others = [card_map[cid] for cid in r["team_ids"] if cid != leader_id]

        best = r
        for perm in permutations(range(len(others))):
            team = [leader_card] + [others[i] for i in perm]
            score = evaluate_team(team, 0, stat_scale, baseline)
            if score["unit_score"] > best["unit_score"]:
                score["leader_idx"] = 0
                score["team_ids"] = [leader_id] + [others[i]["id"] for i in perm]
                best = score

        if "team_ids" not in best or best is r:
            best["team_ids"] = r["team_ids"]
            best["leader_idx"] = r["leader_idx"]
        optimized.append(best)

    optimized.sort(key=lambda x: x["unit_score"], reverse=True)
    return optimized


def calibrate(
    member_ids: list[str],
    leader_id_1: str,
    game_score_1: int,
    leader_id_2: str,
    game_score_2: int,
) -> dict:
    """2点キャリブレーション: stat_scale と baseline を算出"""
    all_cards = load_cards()
    card_map = {c["id"]: c for c in all_cards}
    team = [card_map[mid] for mid in member_ids]

    li1 = member_ids.index(leader_id_1)
    li2 = member_ids.index(leader_id_2)

    s1 = _compute_team_scores(team, li1)
    s2 = _compute_team_scores(team, li2)

    raw_perf = sum(c["stats"]["performance"] for c in team)
    raw_tech = sum(c["stats"]["technique"] for c in team)
    raw_sense = sum(c["stats"]["sense"] for c in team)
    raw_total = raw_perf + raw_tech + raw_sense
    support_raw = s1["support_bonus_total"]

    unit1_target = game_score_1 / ((1 + s1["score_bonus"] / 100) * UNIT_SCORE_K)
    unit2_target = game_score_2 / ((1 + s2["score_bonus"] / 100) * UNIT_SCORE_K)

    costume1 = (raw_perf * s1["costume_perf_rate"] + raw_tech * s1["costume_tech_rate"] + raw_sense * s1["costume_sense_rate"])
    costume2 = (raw_perf * s2["costume_perf_rate"] + raw_tech * s2["costume_tech_rate"] + raw_sense * s2["costume_sense_rate"])
    costume_diff = costume1 - costume2

    warnings = []
    if abs(costume_diff) < 1:
        stat_scale = 1.0
        baseline = unit1_target - (raw_total + costume1 + support_raw)
        warnings.append("衣装バフの差が小さいリーダーの組み合わせです。精度が低い可能性があります。異なる衣装バフ率のリーダーで再キャリブレーションを推奨します。")
    else:
        stat_scale = (unit1_target - unit2_target) / costume_diff
        baseline = unit1_target - (raw_total + costume1 + support_raw) * stat_scale

    result = {
        "stat_scale": round(stat_scale, 6),
        "baseline": round(baseline),
    }
    if warnings:
        result["warnings"] = warnings
    return result


def solve(
    owned_card_ids: list[str],
    top_n: int = 10,
    stat_scale: float = 1.0,
    baseline: float = 0,
    fixed_leader_id: str | None = None,
) -> dict:
    all_cards = load_cards()
    card_map = {c["id"]: c for c in all_cards}

    owned = [card_map[cid] for cid in owned_card_ids if cid in card_map]

    owned_set = set(owned_card_ids)
    if fixed_leader_id and (fixed_leader_id not in card_map or fixed_leader_id not in owned_set):
        return {"total_combinations": 0, "results": []}

    if len(owned) < 5:
        return {"total_combinations": 0, "results": []}

    results = []
    total_combos = 0

    if fixed_leader_id:
        leader_card = card_map[fixed_leader_id]
        others = [c for c in owned if c["id"] != fixed_leader_id]
        for combo in combinations(range(len(others)), 4):
            team = [leader_card] + [others[i] for i in combo]

            if len({c["character"] for c in team}) < 5:
                continue

            total_combos += 1
            score = evaluate_team(team, 0, stat_scale, baseline)
            score["leader_idx"] = 0
            score["team_ids"] = [c["id"] for c in team]
            results.append(score)

            if len(results) > top_n * 10:
                results.sort(key=lambda x: x["unit_score"], reverse=True)
                results = results[:top_n]
    else:
        for combo in combinations(range(len(owned)), 5):
            team = [owned[i] for i in combo]

            if len({c["character"] for c in team}) < 5:
                continue

            total_combos += 1

            best = None
            for leader_idx in range(5):
                score = evaluate_team(team, leader_idx, stat_scale, baseline)
                if best is None or score["unit_score"] > best["unit_score"]:
                    best = score
                    best["leader_idx"] = leader_idx

            best["team_ids"] = [team[i]["id"] for i in range(5)]
            results.append(best)

            if len(results) > top_n * 10:
                results.sort(key=lambda x: x["unit_score"], reverse=True)
                results = results[:top_n]

    results.sort(key=lambda x: x["unit_score"], reverse=True)
    results = results[:top_n]

    results = _optimize_results(results, card_map, stat_scale, baseline)

    return {
        "total_combinations": total_combos,
        "stat_scale": stat_scale,
        "baseline": baseline,
        "results": [
            {
                "rank": i + 1,
                "unit_score": round(r["unit_score"]),
                "total_power": round(r["total_power"]),
                "score_bonus": round(r["score_bonus"], 1),
                "active_pct": round(r["active_pct"], 1),
                "costume_sb_pct": round(r.get("costume_sb_pct", 0), 1),
                "passive_sb_pct": round(r["passive_sb_pct"], 1),
                "special_pct": round(r["special_pct"], 1),
                "leader_id": r["team_ids"][r["leader_idx"]],
                "member_ids": r["team_ids"],
            }
            for i, r in enumerate(results)
        ],
    }
