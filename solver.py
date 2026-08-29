"""HoloSolve — ホロライブドリームス編成オプティマイザ

スコア計算モデル:
  ユニットスコア = 総合力 × (1 + スコアボーナス%) × 2.037

  総合力 = メンバーパラ × (1 + 衣装バフ率) + サポートステバフ + ベースライン
  スコアボーナス = アクティブ% + 衣装SS% + パッシブSB% + スペシャル%
    アクティブ%  = 52.89 + E[max(score_up × uptime)] / 12.82
                   uptime = min(1, duration / interval × boosted_prob)
                   boosted_prob = base_prob + Σ(SP発動率UP × SP継続 / 曲長)
    衣装SS%      = 衣装スコアサポート × 0.68
    パッシブSB%  = サポートSS合計 × 0.20
    スペシャル%  = Σ(SP_score_support × SP_duration) / 曲長
"""

import json
from functools import lru_cache
from itertools import combinations, permutations
from pathlib import Path

UNIT_SCORE_K = 2.0373
ACTIVE_BASE = 52.89
ACTIVE_DIVISOR = 12.82
COSTUME_SS_RATE = 0.68
SUPPORT_SS_RATE = 0.20
SONG_LENGTH = 192

DEFAULT_COMBO_TABLE = [
    (0, 0), (100, 10), (200, 20), (300, 30), (400, 40),
    (500, 50), (600, 60), (700, 70), (800, 80), (900, 90), (1000, 100),
]


def compute_avg_combo_bonus(note_count: int, combo_table: list | None = None) -> float:
    """フルコンボ前提の平均コンボボーナス(permil)を算出"""
    if note_count <= 0:
        return 0.0
    table = combo_table or DEFAULT_COMBO_TABLE
    total_bonus = 0.0
    for i, (threshold, bonus_permil) in enumerate(table):
        next_threshold = table[i + 1][0] if i + 1 < len(table) else note_count + 1
        start = max(threshold, 0)
        end = min(next_threshold, note_count + 1)
        if start < end:
            notes_in_range = end - start
            total_bonus += notes_in_range * bonus_permil
    return total_bonus / note_count


@lru_cache(maxsize=1)
def _load_card_data() -> dict:
    data_path = Path(__file__).parent / "data" / "cards.json"
    with open(data_path, encoding="utf-8") as f:
        return json.load(f)


def load_cards() -> tuple:
    data = _load_card_data()
    return tuple(data.get("cards", []))


def resolve_card(card: dict, potential: int = 0, level: int | None = None) -> dict:
    """カードを指定凸・レベルで解決し、evaluate_team互換のflatなdictを返す

    凸とレベルは独立:
    - 凸: スキル効果(Lv1/Lv2)とステボーナス(+100 permil at 2凸+)のみに影響
    - レベル: 1-80の範囲でステータス基礎値に影響（凸とは無関係）
    """
    pd = card.get("potential_data")
    if not pd:
        return card

    potential = max(0, min(potential, len(pd) - 1))
    snap = pd[potential]

    resolved = {
        "id": card["id"],
        "character": card["character"],
        "card_name": card.get("card_name", ""),
        "rarity": card.get("rarity", 5),
        "type": card["type"],
        "group": card["group"],
        "potential": potential,
        "center_skill": snap["center_skill"],
        "support_skill": snap["support_skill"],
        "costume_skill": snap["costume_skill"],
        "special_skill": snap["special_skill"],
    }
    if card.get("holodori_id"):
        resolved["holodori_id"] = card["holodori_id"]
    if card.get("variant"):
        resolved["variant"] = card["variant"]

    actual_level = max(1, min(level if level is not None else 80, 80))
    resolved["level"] = actual_level

    if actual_level == 80:
        resolved["stats"] = dict(snap.get("ref_stats_lv80", snap.get("stats", {})))
    else:
        data = _load_card_data()
        level_tables = data.get("level_tables", {})
        group_id = card.get("card_level_group_id", "")
        table = level_tables.get(group_id, {})
        base_value = int(table.get(str(actual_level), "0"))
        if base_value > 0:
            permil = card.get("permil", {})
            bonus = snap.get("param_bonus_permil", 0)
            multiplier = 1000 + bonus
            resolved["stats"] = {
                "performance": (base_value * permil.get("performance", 333) * multiplier + 999_999) // 1_000_000,
                "technique": (base_value * permil.get("technique", 333) * multiplier + 999_999) // 1_000_000,
                "sense": (base_value * permil.get("sense", 334) * multiplier + 999_999) // 1_000_000,
            }
        else:
            resolved["stats"] = dict(snap.get("ref_stats_lv80", snap.get("stats", {})))

    s = resolved["stats"]
    resolved["total"] = s["performance"] + s["technique"] + s["sense"]
    return resolved


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


def check_condition(condition, type_counts, group_counts, leader=None):
    if condition is None:
        return True
    if isinstance(condition, str):
        return False
    ctype = condition.get("type")
    if ctype == "type_count":
        return type_counts.get(condition["type_name"], 0) >= condition["min_count"]
    if ctype == "group_count":
        return group_counts.get(condition["group"], 0) >= condition["min_count"]
    if ctype == "leader_character" and leader:
        char_ids = condition.get("character_ids", [])
        leader_holodori_id = leader.get("holodori_id", "")
        leader_char_id = ""
        if leader_holodori_id:
            parts = leader_holodori_id.split("-")
            if len(parts) >= 2:
                leader_char_id = f"chr-{parts[1]}"
        return leader_char_id in char_ids
    if ctype == "leader_group" and leader:
        return leader.get("group", "") == condition.get("group", "")
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


def _compute_team_scores(team, leader_idx, song_length=SONG_LENGTH, override_costume_skill=None):
    leader = team[leader_idx]
    type_counts = count_types(team)
    group_counts = count_groups(team)

    # === 総合力: 衣装バフ（ステごとに正確に計算） ===
    costume_perf_rate = 0.0
    costume_tech_rate = 0.0
    costume_sense_rate = 0.0
    costume_ss = 0.0
    cs = override_costume_skill if override_costume_skill is not None else leader["costume_skill"]
    # NOTE: leader_character/leader_group条件はチーム内リーダー基準。現データにこの条件の衣装スキルはないが、追加時は要検討。
    if check_condition(cs.get("condition"), type_counts, group_counts, leader=leader):
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

        if not check_condition(condition, type_counts, group_counts, leader=leader):
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

        elif etype in ("group_score_support", "group_score_support_conditional"):
            target_group = ss["target"]["group"]
            required = ss["target"].get("count", 2)
            if group_counts.get(target_group, 0) >= required:
                support_ss += ss["value"] / 100.0

        elif etype == "self_stat":
            val = ss["value"] / 100.0
            stat = ss.get("stat", "performance")
            support_bonus[idx] += card["stats"].get(stat, 0) * val

        elif etype in ("group_all_param", "group_all_param_conditional"):
            val = ss["value"] / 100.0
            target_group = ss["target"]["group"]
            applied = 0
            for i, c in enumerate(team):
                if c["group"] == target_group and applied < ss["target"]["count"]:
                    s = c["stats"]
                    support_bonus[i] += (s["performance"] + s["technique"] + s["sense"]) * val
                    applied += 1

    total_ss = costume_ss + support_ss

    # === Special → Active 発動率向上の時間平均 ===
    rate_up_time_avg = 0.0
    for c in team:
        sp = c.get("special_skill", {})
        rate_up = sp.get("skill_rate_up", 0)
        if rate_up > 0:
            rate_up_time_avg += rate_up * 10 * sp.get("duration", 0) / song_length

    # === スコアボーナス: アクティブスキル% (Expected Maximum) ===
    active_members = []
    for card in team:
        cs_card = card["center_skill"]
        score_up = cs_card["score_up"]
        cond = cs_card.get("condition")
        if cond and _check_center_type_condition(cond, type_counts, group_counts):
            score_up = cs_card.get("conditional_score_up", score_up)
        base_prob = cs_card.get("activation_probability_permil", 1000) / 1000.0
        boosted_prob = min(1.0, base_prob + rate_up_time_avg / 1000.0)
        uptime = min(1.0, cs_card["duration"] / cs_card["interval"] * boosted_prob)
        active_members.append((score_up, uptime))

    active_members.sort(key=lambda x: -x[0])
    active_sum = 0.0
    prob_none_higher = 1.0
    for score_up, uptime in active_members:
        active_sum += score_up * uptime * prob_none_higher
        prob_none_higher *= (1.0 - uptime)

    active_pct = ACTIVE_BASE + active_sum / ACTIVE_DIVISOR

    # === スコアボーナス: 衣装SS% + パッシブSB% ===
    costume_sb_pct = costume_ss * 100 * COSTUME_SS_RATE
    passive_sb_pct = support_ss * 100 * SUPPORT_SS_RATE

    # === スコアボーナス: スペシャルスキル% ===
    special_pct = sum(
        c["special_skill"]["score_support"] * c["special_skill"]["duration"] / song_length
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


def _compute_base_scores(team, leader_idx, stat_scale, baseline, song_length=SONG_LENGTH):
    """衣装非依存のスコア中間値を計算する（sweep高速化用）"""
    scores = _compute_team_scores(team, leader_idx, song_length=song_length, override_costume_skill={"condition": None, "effects": []})
    total_perf = sum(c["stats"]["performance"] for c in team) * stat_scale
    total_tech = sum(c["stats"]["technique"] for c in team) * stat_scale
    total_sense = sum(c["stats"]["sense"] for c in team) * stat_scale
    member_params = total_perf + total_tech + total_sense
    support_contrib = scores["support_bonus_total"] * stat_scale
    base_power = member_params + support_contrib + baseline
    base_bonus = scores["active_pct"] + scores["passive_sb_pct"] + scores["special_pct"]
    type_counts = count_types(team)
    group_counts = count_groups(team)
    return {
        "base_power": base_power,
        "base_bonus": base_bonus,
        "total_perf": total_perf,
        "total_tech": total_tech,
        "total_sense": total_sense,
        "member_params": member_params,
        "support_contrib": support_contrib,
        "active_pct": scores["active_pct"],
        "passive_sb_pct": scores["passive_sb_pct"],
        "special_pct": scores["special_pct"],
        "support_ss": scores["support_ss"],
        "type_counts": type_counts,
        "group_counts": group_counts,
        "leader": team[leader_idx],
    }


def _apply_costume(base, costume_skill):
    """衣装スキルを適用してunit_scoreを高速計算する"""
    cpr = ctr = csr = costume_ss = 0.0
    if check_condition(costume_skill.get("condition"), base["type_counts"], base["group_counts"], leader=base["leader"]):
        for effect in costume_skill["effects"]:
            val = effect["value"] / 100.0
            stat = effect["stat"]
            if stat == "score_support":
                costume_ss += val
            elif stat == "all":
                cpr += val; ctr += val; csr += val
            elif stat == "performance":
                cpr += val
            elif stat == "technique":
                ctr += val
            elif stat == "sense":
                csr += val
    costume_contrib = base["total_perf"] * cpr + base["total_tech"] * ctr + base["total_sense"] * csr
    costume_sb_pct = costume_ss * 100 * COSTUME_SS_RATE
    total_power = base["base_power"] + costume_contrib
    score_bonus = base["base_bonus"] + costume_sb_pct
    unit_score = total_power * (1 + score_bonus / 100) * UNIT_SCORE_K
    return unit_score, total_power, score_bonus, costume_sb_pct, costume_ss, costume_contrib


def evaluate_team(team, leader_idx, stat_scale=1.0, baseline=0, song_length=SONG_LENGTH, override_costume_skill=None):
    scores = _compute_team_scores(team, leader_idx, song_length=song_length, override_costume_skill=override_costume_skill)

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


def optimize_order(team_ids: list[str], leader_id: str, stat_scale: float = 1.0, baseline: float = 0, card_specs: dict | None = None, song_length: float = SONG_LENGTH) -> dict:
    """リーダーを固定し、残り4人の配置順を全24通り試して最適な並びを返す"""
    all_cards = load_cards()
    card_map = {c["id"]: c for c in all_cards}

    specs = card_specs or {}

    def _resolve(cid):
        spec = specs.get(cid, {})
        return resolve_card(card_map[cid], potential=spec.get("potential", 0), level=spec.get("level"))

    leader_card = _resolve(leader_id)
    others = [_resolve(cid) for cid in team_ids if cid != leader_id]

    best = None
    best_order = None
    for perm in permutations(range(len(others))):
        team = [leader_card] + [others[i] for i in perm]
        score = evaluate_team(team, 0, stat_scale, baseline, song_length)
        if best is None or score["unit_score"] > best["unit_score"]:
            best = score
            best_order = [leader_id] + [others[i]["id"] for i in perm]

    best["leader_idx"] = 0
    best["team_ids"] = best_order
    return best


def _optimize_results(results: list[dict], card_map: dict, stat_scale: float, baseline: float, song_length: float = SONG_LENGTH, override_costume_skill=None) -> list[dict]:
    """Top 結果の配置順を最適化する"""
    optimized = []
    for r in results:
        leader_id = r["team_ids"][r["leader_idx"]]
        leader_card = card_map[leader_id]
        others = [card_map[cid] for cid in r["team_ids"] if cid != leader_id]

        best = r
        for perm in permutations(range(len(others))):
            team = [leader_card] + [others[i] for i in perm]
            score = evaluate_team(team, 0, stat_scale, baseline, song_length, override_costume_skill=override_costume_skill)
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
    card_specs: dict | None = None,
    song_length: float = SONG_LENGTH,
) -> dict:
    """2点キャリブレーション: stat_scale と baseline を算出"""
    all_cards = load_cards()
    card_map = {c["id"]: c for c in all_cards}
    specs = card_specs or {}
    team = []
    for mid in member_ids:
        spec = specs.get(mid, {})
        team.append(resolve_card(card_map[mid], potential=spec.get("potential", 0), level=spec.get("level")))

    li1 = member_ids.index(leader_id_1)
    li2 = member_ids.index(leader_id_2)

    s1 = _compute_team_scores(team, li1, song_length=song_length)
    s2 = _compute_team_scores(team, li2, song_length=song_length)

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


def recommend(
    owned_cards_input: list,
    top_n: int = 5,
    acquire_count: int = 1,
    stat_scale: float = 1.0,
    baseline: float = 0,
    fixed_leader_id: str | None = None,
    costume_only_leader_id: str | None = None,
    song_length: float = SONG_LENGTH,
) -> dict:
    """未所持カードの取得・既所持カードの凸UPによるスコア向上の優先度を算出"""
    all_cards = load_cards()
    acquire_count = max(1, min(acquire_count, 5))

    owned_specs = {}
    for spec in owned_cards_input:
        cid = spec["id"] if isinstance(spec, dict) else spec
        pot = spec.get("potential", 0) if isinstance(spec, dict) else 0
        lv = spec.get("level") if isinstance(spec, dict) else None
        owned_specs[cid] = {"id": cid, "potential": pot, "level": lv}

    sweep = not fixed_leader_id and not costume_only_leader_id
    solve_kwargs = dict(top_n=1, stat_scale=stat_scale, baseline=baseline, fixed_leader_id=fixed_leader_id, costume_only_leader_id=costume_only_leader_id, song_length=song_length, sweep_costumes=sweep)

    base_result = solve(list(owned_specs.values()), **solve_kwargs)
    base_score = base_result["results"][0]["unit_score"] if base_result["results"] else 0

    candidates = []
    for card in all_cards:
        cid = card["id"]
        if cid not in owned_specs:
            candidates.append({
                "card_id": cid,
                "card_name": card.get("card_name", ""),
                "character": card["character"],
                "action": "acquire",
                "current_potential": None,
                "target_potential": 0,
                "cost": 1,
            })
        else:
            cur_pot = owned_specs[cid]["potential"]
            max_pot = len(card.get("potential_data", [])) - 1
            for target in range(cur_pot + 1, max_pot + 1):
                candidates.append({
                    "card_id": cid,
                    "card_name": card.get("card_name", ""),
                    "character": card["character"],
                    "action": "uncap",
                    "current_potential": cur_pot,
                    "target_potential": target,
                    "cost": target - cur_pot,
                })

    def _apply_candidate(specs, cand):
        specs = dict(specs)
        if cand["action"] == "acquire":
            specs[cand["card_id"]] = {"id": cand["card_id"], "potential": 0, "level": None}
        else:
            old = specs[cand["card_id"]]
            specs[cand["card_id"]] = {**old, "potential": cand["target_potential"]}
        return specs

    # Phase 1: cost=1の候補を1枚ずつ評価して delta > 0 の候補を抽出
    single_results = []
    effective_card_ids = set()
    for cand in candidates:
        if cand["cost"] != 1:
            continue
        trial_specs = _apply_candidate(owned_specs, cand)
        trial_result = solve(list(trial_specs.values()), **solve_kwargs)
        if trial_result["results"]:
            best = trial_result["results"][0]
            delta = best["unit_score"] - base_score
            if delta > 0:
                single_results.append((cand, delta, best))
                effective_card_ids.add(cand["card_id"])

    single_results.sort(key=lambda x: x[1], reverse=True)

    if acquire_count == 1:
        results = []
        for cand, delta, best in single_results[:top_n]:
            results.append({
                "cards": [cand],
                "new_score": best["unit_score"],
                "delta": delta,
                "best_team": {"leader_id": best["leader_id"], "member_ids": best["member_ids"]},
            })
    else:
        multi_uncap = [
            cand for cand in candidates
            if cand["cost"] > 1 and cand["cost"] <= acquire_count and cand["card_id"] in effective_card_ids
        ]
        single_cands = [c for c, d, _ in single_results if d > 0]
        max_single = max(0, 20 - len(multi_uncap))
        shortlist = single_cands[:max_single] + multi_uncap

        def _combos_by_cost(items, total_cost, start=0):
            if total_cost == 0:
                yield []
                return
            for i in range(start, len(items)):
                c = items[i]["cost"]
                if c <= total_cost:
                    for rest in _combos_by_cost(items, total_cost - c, i + 1):
                        yield [items[i]] + rest

        results = []
        for combo_cards in _combos_by_cost(shortlist, acquire_count):
            card_ids = [c["card_id"] for c in combo_cards]
            if len(card_ids) != len(set(card_ids)):
                continue
            acquire_chars = [c["character"] for c in combo_cards if c["action"] == "acquire"]
            if len(acquire_chars) != len(set(acquire_chars)):
                continue
            trial_specs = dict(owned_specs)
            for cand in combo_cards:
                trial_specs = _apply_candidate(trial_specs, cand)
            trial_result = solve(list(trial_specs.values()), **solve_kwargs)
            if trial_result["results"]:
                new_score = trial_result["results"][0]["unit_score"]
                delta = new_score - base_score
                if delta > 0:
                    best = trial_result["results"][0]
                    results.append({
                        "cards": combo_cards,
                        "new_score": new_score,
                        "delta": delta,
                        "best_team": {"leader_id": best["leader_id"], "member_ids": best["member_ids"]},
                    })

        results.sort(key=lambda x: x["delta"], reverse=True)
        results = results[:top_n]

    for i, r in enumerate(results):
        r["rank"] = i + 1

    return {
        "base_score": base_score,
        "acquire_count": acquire_count,
        "recommendations": results,
    }


def _solve_sweep_costumes(owned_cards_input, all_cards, card_map, top_n, stat_scale, baseline, song_length, stability_lengths):
    """手持ち全衣装を高速スイープ: 衣装非依存部分を1回計算し、衣装部分だけ差し替える"""
    owned = []
    if owned_cards_input and isinstance(owned_cards_input[0], str):
        for cid in owned_cards_input:
            if cid in card_map:
                owned.append(resolve_card(card_map[cid], potential=0))
    else:
        for spec in owned_cards_input:
            cid = spec["id"] if isinstance(spec, dict) else spec
            pot = spec.get("potential", 0) if isinstance(spec, dict) else 0
            lv = spec.get("level") if isinstance(spec, dict) else None
            if cid in card_map:
                owned.append(resolve_card(card_map[cid], potential=pot, level=lv))

    if len(owned) < 5:
        return {"total_combinations": 0, "results": []}

    char_groups = {}
    for card in owned:
        char_groups.setdefault(card["character"], []).append(card)
    char_names = sorted(char_groups.keys(), key=lambda ch: -max(c["total"] for c in char_groups[ch]))
    n_chars = len(char_names)
    if n_chars < 5:
        return {"total_combinations": 0, "results": []}

    costume_skills = []
    owned_ids = {c["id"] for c in owned}
    for card in all_cards:
        if card["id"] in owned_ids:
            pd = card.get("potential_data", [{}])
            cs = pd[0].get("costume_skill") if pd else None
            if cs:
                costume_skills.append((card["id"], cs))

    results = []
    total_combos = 0

    for char_combo in combinations(range(n_chars), 5):
        cards_lists = [char_groups[char_names[i]] for i in char_combo]
        for c0 in cards_lists[0]:
            for c1 in cards_lists[1]:
                for c2 in cards_lists[2]:
                    for c3 in cards_lists[3]:
                        for c4 in cards_lists[4]:
                            team = [c0, c1, c2, c3, c4]
                            total_combos += 1

                            best_base = None
                            best_leader_idx = 0
                            for leader_idx in range(5):
                                base = _compute_base_scores(team, leader_idx, stat_scale, baseline, song_length)
                                if best_base is None or base["base_power"] > best_base["base_power"]:
                                    best_base = base
                                    best_leader_idx = leader_idx

                            for costume_id, cs in costume_skills:
                                unit_score, total_power, score_bonus, costume_sb_pct, costume_ss, costume_contrib = _apply_costume(best_base, cs)
                                if not results or len(results) < top_n or unit_score > results[-1]["unit_score"]:
                                    entry = {
                                        "unit_score": unit_score,
                                        "total_power": total_power,
                                        "score_bonus": score_bonus,
                                        "active_pct": best_base["active_pct"],
                                        "costume_sb_pct": costume_sb_pct,
                                        "passive_sb_pct": best_base["passive_sb_pct"],
                                        "special_pct": best_base["special_pct"],
                                        "costume_ss": costume_ss,
                                        "support_ss": best_base["support_ss"],
                                        "leader_idx": best_leader_idx,
                                        "team_ids": [c["id"] for c in team],
                                        "costume_only_leader_id": costume_id,
                                    }
                                    results.append(entry)
                                    if len(results) > top_n * 10:
                                        results.sort(key=lambda x: x["unit_score"], reverse=True)
                                        results = results[:top_n]

    results.sort(key=lambda x: x["unit_score"], reverse=True)
    results = results[:top_n]

    resolved_map = {c["id"]: c for c in owned}
    for ri in range(len(results)):
        r = results[ri]
        costume_id = r["costume_only_leader_id"]
        cs = next(c for cid, c in costume_skills if cid == costume_id)
        leader_card = resolved_map[r["team_ids"][r["leader_idx"]]]
        others = [resolved_map[cid] for cid in r["team_ids"] if cid != leader_card["id"]]
        for perm in permutations(range(len(others))):
            team = [leader_card] + [others[i] for i in perm]
            score = evaluate_team(team, 0, stat_scale, baseline, song_length, override_costume_skill=cs)
            if score["unit_score"] > r["unit_score"]:
                score["leader_idx"] = 0
                score["team_ids"] = [c["id"] for c in team]
                score["costume_only_leader_id"] = costume_id
                results[ri] = score
                r = score

    results.sort(key=lambda x: x["unit_score"], reverse=True)

    formatted = []
    for i, r in enumerate(results):
        entry = {
            "rank": i + 1,
            "unit_score": round(r["unit_score"]),
            "total_power": round(r["total_power"]),
            "score_bonus": round(r["score_bonus"], 1),
            "active_pct": round(r["active_pct"], 1),
            "costume_sb_pct": round(r.get("costume_sb_pct", 0), 1),
            "passive_sb_pct": round(r["passive_sb_pct"], 1),
            "special_pct": round(r["special_pct"], 1),
            "leader_id": r["team_ids"][r["leader_idx"]],
            "costume_only_leader_id": r.get("costume_only_leader_id"),
            "member_ids": r["team_ids"],
        }
        if stability_lengths:
            team = [resolved_map[cid] for cid in r["team_ids"]]
            cs_override = next(c for cid, c in costume_skills if cid == r["costume_only_leader_id"])
            scores_by_length = {}
            for sl in stability_lengths:
                s = evaluate_team(team, r["leader_idx"], stat_scale, baseline, sl, override_costume_skill=cs_override)
                scores_by_length[sl] = round(s["unit_score"])
            entry["stability"] = scores_by_length
        formatted.append(entry)

    return {"total_combinations": total_combos, "stat_scale": stat_scale, "baseline": baseline, "results": formatted}


def solve(
    owned_cards_input: list,
    top_n: int = 10,
    stat_scale: float = 1.0,
    baseline: float = 0,
    fixed_leader_id: str | None = None,
    costume_only_leader_id: str | None = None,
    song_length: float = SONG_LENGTH,
    stability_lengths: list[float] | None = None,
    sweep_costumes: bool = False,
) -> dict:
    all_cards = load_cards()
    card_map = {c["id"]: c for c in all_cards}

    if sweep_costumes and not fixed_leader_id and not costume_only_leader_id:
        return _solve_sweep_costumes(owned_cards_input, all_cards, card_map, top_n, stat_scale, baseline, song_length, stability_lengths)

    owned = []
    if owned_cards_input and isinstance(owned_cards_input[0], str):
        for cid in owned_cards_input:
            if cid in card_map:
                owned.append(resolve_card(card_map[cid], potential=0))
    else:
        for spec in owned_cards_input:
            cid = spec["id"] if isinstance(spec, dict) else spec
            pot = spec.get("potential", 0) if isinstance(spec, dict) else 0
            lv = spec.get("level") if isinstance(spec, dict) else None
            if cid in card_map:
                owned.append(resolve_card(card_map[cid], potential=pot, level=lv))

    owned_ids = {c["id"] for c in owned}
    if fixed_leader_id and (fixed_leader_id not in card_map or fixed_leader_id not in owned_ids):
        return {"total_combinations": 0, "results": []}
    if costume_only_leader_id and costume_only_leader_id not in card_map:
        return {"total_combinations": 0, "results": []}

    if fixed_leader_id and costume_only_leader_id:
        costume_only_leader_id = None

    override_costume_skill = None
    if costume_only_leader_id:
        costume_card = card_map[costume_only_leader_id]
        costume_pot_data = costume_card.get("potential_data", [{}])
        override_costume_skill = costume_pot_data[0].get("costume_skill") if costume_pot_data else None

    if len(owned) < 5:
        return {"total_combinations": 0, "results": []}

    char_groups = {}
    for card in owned:
        char_groups.setdefault(card["character"], []).append(card)

    char_names = sorted(char_groups.keys(), key=lambda ch: -max(c["total"] for c in char_groups[ch]))
    n_chars = len(char_names)

    if n_chars < 5:
        return {"total_combinations": 0, "results": []}

    results = []
    total_combos = 0

    if fixed_leader_id:
        leader_card = next(c for c in owned if c["id"] == fixed_leader_id)
        leader_char = leader_card["character"]
        other_chars = [ch for ch in char_names if ch != leader_char]

        for combo in combinations(range(len(other_chars)), 4):
            cards_lists = [char_groups[other_chars[i]] for i in combo]
            for c0 in cards_lists[0]:
                for c1 in cards_lists[1]:
                    for c2 in cards_lists[2]:
                        for c3 in cards_lists[3]:
                            team = [leader_card, c0, c1, c2, c3]
                            total_combos += 1
                            score = evaluate_team(team, 0, stat_scale, baseline, song_length, override_costume_skill=override_costume_skill)
                            score["leader_idx"] = 0
                            score["team_ids"] = [c["id"] for c in team]
                            results.append(score)

                            if len(results) > top_n * 10:
                                results.sort(key=lambda x: x["unit_score"], reverse=True)
                                results = results[:top_n]
    else:
        for char_combo in combinations(range(n_chars), 5):
            cards_lists = [char_groups[char_names[i]] for i in char_combo]
            for c0 in cards_lists[0]:
                for c1 in cards_lists[1]:
                    for c2 in cards_lists[2]:
                        for c3 in cards_lists[3]:
                            for c4 in cards_lists[4]:
                                team = [c0, c1, c2, c3, c4]
                                total_combos += 1

                                best = None
                                for leader_idx in range(5):
                                    score = evaluate_team(team, leader_idx, stat_scale, baseline, song_length, override_costume_skill=override_costume_skill)
                                    if best is None or score["unit_score"] > best["unit_score"]:
                                        best = score
                                        best["leader_idx"] = leader_idx

                                best["team_ids"] = [c["id"] for c in team]
                                results.append(best)

                                if len(results) > top_n * 10:
                                    results.sort(key=lambda x: x["unit_score"], reverse=True)
                                    results = results[:top_n]

    results.sort(key=lambda x: x["unit_score"], reverse=True)
    results = results[:top_n]

    resolved_map = {c["id"]: c for c in owned}
    results = _optimize_results(results, resolved_map, stat_scale, baseline, song_length, override_costume_skill=override_costume_skill)

    formatted = []
    for i, r in enumerate(results):
        entry = {
            "rank": i + 1,
            "unit_score": round(r["unit_score"]),
            "total_power": round(r["total_power"]),
            "score_bonus": round(r["score_bonus"], 1),
            "active_pct": round(r["active_pct"], 1),
            "costume_sb_pct": round(r.get("costume_sb_pct", 0), 1),
            "passive_sb_pct": round(r["passive_sb_pct"], 1),
            "special_pct": round(r["special_pct"], 1),
            "leader_id": r["team_ids"][r["leader_idx"]],
            "costume_only_leader_id": costume_only_leader_id,
            "member_ids": r["team_ids"],
        }
        if stability_lengths:
            team = [resolved_map[cid] for cid in r["team_ids"]]
            leader_idx = r["leader_idx"]
            scores_by_length = {}
            for sl in stability_lengths:
                s = evaluate_team(team, leader_idx, stat_scale, baseline, sl, override_costume_skill=override_costume_skill)
                scores_by_length[sl] = round(s["unit_score"])
            entry["stability"] = scores_by_length
        formatted.append(entry)

    return {
        "total_combinations": total_combos,
        "stat_scale": stat_scale,
        "baseline": baseline,
        "results": formatted,
    }
