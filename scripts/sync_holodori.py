"""HolodoriDB → HoloSolve カードデータ生成スクリプト

Usage:
    uv run python scripts/sync_holodori.py
    uv run python scripts/sync_holodori.py --local /path/to/holodori-db-jpn-diff
"""

import json
import re
import sys
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

BASE_URL = "https://raw.githubusercontent.com/HolodoriDB/holodori-db-jpn-diff/main"

MASTER_FILES = [
    "Card", "CardLevel", "CardLevelLimit", "CardPotential",
    "Character", "CharacterGrouping",
    "Costume", "LiveLeaderSkill",
    "LiveActiveSkillLevel", "LiveActiveSkillEffect",
    "LivePassiveSkillLevel", "LivePassiveSkillEffect",
    "LiveSkillEffectTarget", "LiveSkillTrigger",
    "LiveSpecialSkillLevel",
    "Music", "LiveCombo",
    "SkillTreeNode", "SkillTreeEffect",
    "LangCard_Jpn", "LangCharacter_Jpn", "LangCharacterGrouping_Jpn",
    "LangMusic_Jpn",
]

ATTRIBUTE_MAP = {
    "CardAttributeType_CARD_ATTRIBUTE_TYPE_ATTRIBUTE_1": "cute",
    "CardAttributeType_CARD_ATTRIBUTE_TYPE_ATTRIBUTE_2": "pure",
    "CardAttributeType_CARD_ATTRIBUTE_TYPE_ATTRIBUTE_3": "happy",
}

ATTRIBUTE_NUM_MAP = {
    "attribute_1": "cute",
    "attribute_2": "pure",
    "attribute_3": "happy",
}

GROUP_ID_TO_NAME = {
    "grp-gen_0": "0期生", "grp-gen_1": "1期生", "grp-gen_2": "2期生",
    "grp-gamers": "ゲーマーズ", "grp-gen_3": "3期生", "grp-gen_4": "4期生",
    "grp-gen_5": "5期生", "grp-holox": "holoX",
    "grp-indonesia-gen_1": "ID1期生", "grp-indonesia-gen_2": "ID2期生",
    "grp-indonesia-gen_3": "ID3期生",
    "grp-myth": "Myth", "grp-promise": "Promise", "grp-advent": "Advent",
    "grp-regloss": "ReGLOSS",
}


def fetch_master(name: str, local_dir: str | None = None) -> list[dict]:
    if local_dir:
        path = Path(local_dir) / f"{name}.json"
        with open(path, encoding="utf-8") as f:
            raw = json.load(f)
    else:
        url = f"{BASE_URL}/{name}.json"
        with urllib.request.urlopen(url) as resp:
            raw = json.loads(resp.read().decode("utf-8"))

    if isinstance(raw, list):
        return [item.get("data", item) if isinstance(item, dict) else item for item in raw]
    if isinstance(raw, dict) and "data" in raw:
        data = raw["data"]
        if isinstance(data, list):
            return data
    return raw if isinstance(raw, list) else [raw]


def index_by(records: list[dict], key: str) -> dict:
    result = {}
    for r in records:
        result[r.get(key, "")] = r
    return result


def group_by(records: list[dict], key: str) -> dict:
    result = {}
    for r in records:
        k = r.get(key, "")
        result.setdefault(k, []).append(r)
    return result


def make_holosolve_id(name_eng: str, rarity: int, existing_ids: set, variant_chars: dict) -> str:
    slug = re.sub(r"[^a-z0-9]+", "_", name_eng.lower()).strip("_")
    char_key = slug
    if char_key in variant_chars:
        variant_chars[char_key] += 1
        base_id = f"{slug}_swim_{rarity}"
    else:
        variant_chars[char_key] = 1
        base_id = f"{slug}_{rarity}"

    if base_id not in existing_ids:
        existing_ids.add(base_id)
        return base_id

    n = 2
    while f"{base_id}_{n}" in existing_ids:
        n += 1
    final_id = f"{base_id}_{n}"
    existing_ids.add(final_id)
    return final_id


def resolve_effect(effect_group_id: str, effects_by_group: dict) -> dict | None:
    effects = effects_by_group.get(effect_group_id)
    if not effects:
        return None
    e = effects[0]
    return {
        "type": e.get("type", ""),
        "value": int(e.get("value", "0")),
        "target_id": e.get("liveSkillEffectTargetId"),
    }


def resolve_trigger(trigger_group_id: str | None, triggers_by_group: dict) -> dict | None:
    if not trigger_group_id:
        return None
    triggers = triggers_by_group.get(trigger_group_id)
    if not triggers:
        return None
    t = triggers[0]
    return t


def convert_trigger_to_holosolve(trigger: dict | None) -> dict | str | None:
    if trigger is None:
        return None

    ttype = trigger.get("type", "")
    threshold = int(trigger.get("threshold", "0"))

    if "COMBO_GTE" in ttype:
        return f"combo_{threshold}"
    if "LIFE_GTE" in ttype:
        return f"life_{threshold}"
    if "LIFE_LTE" in ttype:
        return f"life_lte_{threshold}"
    if "DECK_CARD_ATTRIBUTE" in ttype:
        attr_raw = trigger.get("cardAttributeType", "")
        attr = ATTRIBUTE_MAP.get(attr_raw, attr_raw)
        return {"type": "type_count", "type_name": attr, "min_count": threshold}
    if "DECK_CARD_CHARACTER_GROUPING" in ttype:
        grp = trigger.get("characterGroupingId", "")
        grp_name = GROUP_ID_TO_NAME.get(grp, grp)
        return {"type": "group_count", "group": grp_name, "min_count": threshold}
    if "DECK_LEADER_CHARACTER" in ttype:
        char_ids = trigger.get("characterIds", [])
        return {"type": "leader_character", "character_ids": char_ids}
    if "DECK_LEADER_CHARACTER_GROUPING" in ttype:
        grp = trigger.get("characterGroupingId", "")
        grp_name = GROUP_ID_TO_NAME.get(grp, grp)
        return {"type": "leader_group", "group": grp_name}
    if "JUDGEMENT_TYPE_GTE" in ttype:
        return None
    if "MUSIC_CHARACTER" in ttype:
        return None

    return None


def convert_target_to_holosolve(target: dict | None) -> str | dict:
    if target is None:
        return "self"

    ttype = target.get("type", "")
    count = int(target.get("targetCount", "0"))

    if "ALL" in ttype:
        return "all"
    if "SELF" in ttype:
        return "self"
    if "ATTRIBUTE" in ttype:
        attr_raw = target.get("cardAttributeType", "")
        attr = ATTRIBUTE_MAP.get(attr_raw, attr_raw)
        if not attr:
            for k, v in ATTRIBUTE_NUM_MAP.items():
                if k in target.get("id", ""):
                    attr = v
                    break
        return {"type_match": attr, "count": count}
    if "CHARACTER_GROUPING" in ttype:
        grp = target.get("characterGroupingId", "")
        grp_name = GROUP_ID_TO_NAME.get(grp, grp)
        return {"group": grp_name, "count": count}

    return "self"


def classify_passive_effect(effect_type: str, trigger, target_info, value_percent: float) -> dict:
    target_hs = convert_target_to_holosolve(target_info)
    condition_hs = convert_trigger_to_holosolve(trigger)

    if "ALL_PARAMETER_UP" in effect_type:
        if target_hs == "self" or (isinstance(target_hs, dict) and target_hs == {"type_match": "", "count": 0}):
            etype = "self_all_param_conditional" if condition_hs else "self_all_param"
            return {
                "effect_type": etype,
                "value": value_percent,
                "condition": condition_hs,
                "target": "self",
            }
        elif isinstance(target_hs, dict) and "type_match" in target_hs:
            return {
                "effect_type": "type_all_param",
                "value": value_percent,
                "condition": condition_hs,
                "target": target_hs,
            }
        elif isinstance(target_hs, dict) and "group" in target_hs:
            return {
                "effect_type": "group_all_param",
                "value": value_percent,
                "condition": condition_hs,
                "target": target_hs,
            }
        else:
            etype = "self_all_param_conditional" if condition_hs else "self_all_param"
            return {
                "effect_type": etype,
                "value": value_percent,
                "condition": condition_hs,
                "target": "self",
            }

    if "LIVE_ACTIVE_SKILL_EFFECT_UP" in effect_type:
        if isinstance(target_hs, dict) and "type_match" in target_hs:
            etype = "type_score_support"
            return {
                "effect_type": etype,
                "value": value_percent,
                "condition": condition_hs,
                "target": target_hs,
            }
        elif isinstance(target_hs, dict) and "group" in target_hs:
            etype = "group_score_support_conditional" if condition_hs else "group_score_support"
            return {
                "effect_type": etype,
                "value": value_percent,
                "condition": condition_hs,
                "target": target_hs,
            }
        elif target_hs == "all" or target_hs == "self":
            etype = "type_score_support"
            return {
                "effect_type": etype,
                "value": value_percent,
                "condition": condition_hs,
                "target": target_hs if isinstance(target_hs, dict) else {"type_match": "", "count": 0},
            }

    stat_name = None
    if "PERFORMANCE_UP" in effect_type:
        stat_name = "performance"
    elif "TECHNIQUE_UP" in effect_type:
        stat_name = "technique"
    elif "SENSE_UP" in effect_type:
        stat_name = "sense"

    if stat_name:
        if isinstance(target_hs, dict) and "type_match" in target_hs:
            etype = "type_stat_conditional" if condition_hs else "type_stat"
            return {
                "effect_type": etype,
                "value": value_percent,
                "stat": stat_name,
                "condition": condition_hs,
                "target": target_hs,
            }
        elif isinstance(target_hs, dict) and "group" in target_hs:
            etype = "group_stat_conditional" if condition_hs else "group_stat"
            return {
                "effect_type": etype,
                "value": value_percent,
                "stat": stat_name,
                "condition": condition_hs,
                "target": target_hs,
            }
        else:
            return {
                "effect_type": "self_stat",
                "value": value_percent,
                "stat": stat_name,
                "condition": condition_hs,
                "target": "self",
            }

    return {
        "effect_type": "unknown",
        "raw_type": effect_type,
        "value": value_percent,
        "condition": condition_hs,
        "target": target_hs,
    }


def convert_leader_skill(
    leader_skill: dict,
    passive_effects_by_group: dict,
    targets_by_id: dict,
    triggers_by_group: dict,
) -> dict:
    effects_list = []

    primary_group = leader_skill.get("livePassiveSkillEffectGroupId")
    primary_trigger_group = leader_skill.get("liveSkillTriggerGroupId")

    trigger = resolve_trigger(primary_trigger_group, triggers_by_group)
    condition = convert_trigger_to_holosolve(trigger)

    if primary_group:
        effs = passive_effects_by_group.get(primary_group, [])
        for eff in effs:
            eff_type = eff.get("type", "")
            value_permil = int(eff.get("value", "0"))
            target_id = eff.get("liveSkillEffectTargetId")
            target_info = targets_by_id.get(target_id)

            stat = None
            if "PERFORMANCE_UP" in eff_type:
                stat = "performance"
            elif "TECHNIQUE_UP" in eff_type:
                stat = "technique"
            elif "SENSE_UP" in eff_type:
                stat = "sense"
            elif "ALL_PARAMETER_UP" in eff_type:
                stat = "all"
            elif "LIVE_ACTIVE_SKILL_EFFECT_UP" in eff_type:
                stat = "score_support"

            if stat:
                v = value_permil / 10
                effects_list.append({
                    "target": "all",
                    "stat": stat,
                    "value": int(v) if v == int(v) else v,
                })

    additional_group = leader_skill.get("additionalLivePassiveSkillEffectGroupId")
    if additional_group:
        effs = passive_effects_by_group.get(additional_group, [])
        for eff in effs:
            eff_type = eff.get("type", "")
            value_permil = int(eff.get("value", "0"))

            stat = None
            if "PERFORMANCE_UP" in eff_type:
                stat = "performance"
            elif "TECHNIQUE_UP" in eff_type:
                stat = "technique"
            elif "SENSE_UP" in eff_type:
                stat = "sense"
            elif "ALL_PARAMETER_UP" in eff_type:
                stat = "all"
            elif "LIVE_ACTIVE_SKILL_EFFECT_UP" in eff_type:
                stat = "score_support"

            if stat:
                v = value_permil / 10
                effects_list.append({
                    "target": "all",
                    "stat": stat,
                    "value": int(v) if v == int(v) else v,
                })

    return {
        "condition": condition,
        "effects": effects_list,
    }


def build_active_skill(
    skill_level: dict,
    effects_by_group: dict,
    triggers_by_group: dict,
) -> dict:
    base_group = skill_level.get("liveActiveSkillEffectGroupId", "")
    base_effect = resolve_effect(base_group, effects_by_group)

    base_score_up = 0
    if base_effect and "SCORE_UP_PERMIL" in base_effect["type"]:
        base_score_up = base_effect["value"] / 10

    interval = skill_level.get("coolTimeMillisecond", 0) / 1000
    duration = skill_level.get("effectDurationMillisecond", 0) / 1000
    prob_permil = skill_level.get("activationProbabilityPermilMultiply", 460)

    if prob_permil >= 550:
        prob_str = "high"
    elif prob_permil >= 460:
        prob_str = "medium"
    else:
        prob_str = "low"

    cond_trigger_group = skill_level.get("additionalLiveSkillTriggerGroupId")
    cond_effect_group = skill_level.get("additionalLiveActiveSkillEffectGroupId")

    condition = None
    conditional_score_up = None

    if cond_trigger_group and cond_effect_group:
        trigger = resolve_trigger(cond_trigger_group, triggers_by_group)
        condition = convert_trigger_to_holosolve(trigger)

        cond_effect = resolve_effect(cond_effect_group, effects_by_group)
        if cond_effect and "SCORE_UP_PERMIL" in cond_effect["type"]:
            conditional_score_up = cond_effect["value"] / 10

    if isinstance(condition, dict):
        condition_str = None
        if condition.get("type") == "type_count":
            condition_str = f"{condition['type_name']}_{condition['min_count']}"
        elif condition.get("type") == "group_count":
            condition_str = f"group_{condition['group']}_{condition['min_count']}"
        condition = condition_str
    elif isinstance(condition, str):
        pass

    return {
        "interval": int(interval) if interval == int(interval) else interval,
        "probability": prob_str,
        "activation_probability_permil": prob_permil,
        "duration": int(duration) if duration == int(duration) else duration,
        "score_up": int(base_score_up) if base_score_up == int(base_score_up) else base_score_up,
        "condition": condition,
        "conditional_score_up": int(conditional_score_up) if conditional_score_up and conditional_score_up == int(conditional_score_up) else conditional_score_up,
    }


def build_passive_skill(
    skill_level: dict,
    passive_effects_by_group: dict,
    targets_by_id: dict,
    triggers_by_group: dict,
) -> dict:
    trigger_group = skill_level.get("liveSkillTriggerGroupId")
    trigger = resolve_trigger(trigger_group, triggers_by_group)

    effect_group = skill_level.get("livePassiveSkillEffectGroupId", "")
    effs = passive_effects_by_group.get(effect_group, [])
    if not effs:
        return {
            "effect_type": "unknown",
            "value": 0,
            "condition": None,
            "target": "self",
        }

    eff = effs[0]
    eff_type = eff.get("type", "")
    value_permil = int(eff.get("value", "0"))
    value_percent = value_permil / 10

    target_id = eff.get("liveSkillEffectTargetId")
    target_info = targets_by_id.get(target_id)

    result = classify_passive_effect(eff_type, trigger, target_info, value_percent)

    if result["value"] == int(result["value"]):
        result["value"] = int(result["value"])

    return result


def build_special_skill(
    skill_level: dict,
    effects_by_group: dict,
    triggers_by_group: dict,
) -> dict:
    effect_group = skill_level.get("liveActiveSkillEffectGroupId", "")
    effect = resolve_effect(effect_group, effects_by_group)

    duration_ms = skill_level.get("effectDurationMillisecond", 0)
    duration = duration_ms / 1000

    score_support = 0
    skill_rate_up = None

    if effect:
        if "SCORE_UP_EFFECT_UP" in effect["type"]:
            score_support = effect["value"] / 10
        elif "SCORE_UP_PERMIL" in effect["type"]:
            score_support = effect["value"] / 10

    cond_effect_group = skill_level.get("additionalLiveActiveSkillEffectGroupId")
    if cond_effect_group:
        cond_effect = resolve_effect(cond_effect_group, effects_by_group)
        if cond_effect and "ACTIVATION_PROBABILITY" in cond_effect["type"]:
            skill_rate_up = cond_effect["value"] / 10

    result = {
        "duration": int(duration) if duration == int(duration) else duration,
        "score_support": int(score_support) if score_support == int(score_support) else score_support,
    }

    if skill_rate_up is not None:
        result["skill_rate_up"] = int(skill_rate_up) if skill_rate_up == int(skill_rate_up) else skill_rate_up

        cond_trigger_group = skill_level.get("additionalLiveSkillTriggerGroupId")
        if cond_trigger_group:
            trigger = resolve_trigger(cond_trigger_group, triggers_by_group)
            cond = convert_trigger_to_holosolve(trigger)
            if isinstance(cond, str):
                result["skill_rate_condition"] = cond

    return result


def main():
    local_dir = None
    if "--local" in sys.argv:
        idx = sys.argv.index("--local")
        if idx + 1 < len(sys.argv):
            local_dir = sys.argv[idx + 1]

    print("Fetching HolodoriDB master tables...")
    masters = {}
    for name in MASTER_FILES:
        print(f"  {name}...", end=" ", flush=True)
        try:
            masters[name] = fetch_master(name, local_dir)
            print(f"({len(masters[name])} records)")
        except Exception as e:
            print(f"FAILED: {e}")
            if name.startswith("Lang"):
                masters[name] = []
            else:
                sys.exit(1)

    # --- Build lookup tables ---
    print("\nBuilding lookup tables...")

    cards_all = masters["Card"]
    r5_cards = [c for c in cards_all if "RARITY_5" in c.get("rarity", "")]
    r5_cards.sort(key=lambda c: c.get("order", 0))
    print(f"  R5 cards: {len(r5_cards)}")

    characters = index_by(masters["Character"], "id")
    char_groupings = index_by(masters["CharacterGrouping"], "id")

    card_levels_by_group = group_by(masters["CardLevel"], "groupId")
    card_level_limits_by_group = group_by(masters["CardLevelLimit"], "groupId")
    card_potentials_by_group = group_by(masters["CardPotential"], "groupId")

    active_skills_by_id = {}
    for s in masters["LiveActiveSkillLevel"]:
        key = (s.get("liveActiveSkillId", ""), s.get("level", 0))
        active_skills_by_id[key] = s

    passive_skills_by_id = {}
    for s in masters["LivePassiveSkillLevel"]:
        key = (s.get("livePassiveSkillId", ""), s.get("level", 0))
        passive_skills_by_id[key] = s

    special_skills_by_id = {}
    for s in masters["LiveSpecialSkillLevel"]:
        key = (s.get("liveSpecialSkillId", ""), s.get("level", 0))
        special_skills_by_id[key] = s

    active_effects_by_group = group_by(masters["LiveActiveSkillEffect"], "groupId")
    passive_effects_by_group = group_by(masters["LivePassiveSkillEffect"], "groupId")
    targets_by_id = index_by(masters["LiveSkillEffectTarget"], "id")
    triggers_by_group = group_by(masters["LiveSkillTrigger"], "groupId")

    costumes = index_by(masters["Costume"], "id")
    leader_skills = index_by(masters["LiveLeaderSkill"], "id")

    lang_cards = index_by(masters["LangCard_Jpn"], "id")
    lang_chars = index_by(masters["LangCharacter_Jpn"], "id")
    lang_groups = index_by(masters["LangCharacterGrouping_Jpn"], "id")

    # --- Build level tables ---
    print("Building level tables...")
    level_tables = {}
    r5_level_groups = set()
    for card in r5_cards:
        r5_level_groups.add(card.get("cardLevelGroupId", ""))

    for group_id in sorted(r5_level_groups):
        entries = card_levels_by_group.get(group_id, [])
        table = {}
        for entry in entries:
            lv = entry.get("level", 0)
            base_val = int(entry.get("parameterBaseValue", "0"))
            table[str(lv)] = base_val
        level_tables[group_id] = table

    # --- Build level limit lookup ---
    level_limits = {}
    for group_id, entries in card_level_limits_by_group.items():
        if "rarity_5" not in group_id:
            continue
        limits = {}
        for entry in entries:
            lb_count = entry.get("limitBreakCount", 0)
            level_limit = entry.get("levelLimit", 40)
            limits[lb_count] = level_limit
        level_limits[group_id] = limits

    # --- Build potential progression ---
    r5_potential_group = "card_potential_grp-rarity_5"
    potential_entries = card_potentials_by_group.get(r5_potential_group, [])
    potential_entries.sort(key=lambda x: x.get("upgradeCount", 0))

    def get_skill_levels(potential: int) -> tuple[int, int, int]:
        active_lv, passive_lv, special_lv = 1, 1, 1
        for entry in potential_entries:
            uc = entry.get("upgradeCount", 0)
            if uc > potential:
                break
            eff = entry.get("effectType", "")
            if "ACTIVE_SKILL_LEVEL_UP" in eff:
                active_lv = 2
            elif "PASSIVE_SKILL_LEVEL_UP" in eff:
                passive_lv = 2
            elif "SPECIAL_SKILL_LEVEL_UP" in eff:
                special_lv = 2
        return active_lv, passive_lv, special_lv

    def get_param_bonus(potential: int) -> int:
        bonus = 0
        for entry in potential_entries:
            uc = entry.get("upgradeCount", 0)
            if uc > potential:
                break
            eff = entry.get("effectType", "")
            if "ALL_PARAMETER_UP_PERMIL_UP" in eff:
                bonus += int(entry.get("value", "0"))
        return bonus

    # --- Character group name resolver ---
    def get_group_name(char_id: str) -> str:
        char = characters.get(char_id, {})
        group_ids = char.get("regularCharacterGroupingIds", [])
        for gid in group_ids:
            if gid in GROUP_ID_TO_NAME:
                return GROUP_ID_TO_NAME[gid]
        if group_ids:
            lang_key = char_groupings.get(group_ids[0], {}).get("nameLangId", "")
            lang_entry = lang_groups.get(lang_key, {})
            if lang_entry.get("text"):
                return lang_entry["text"]
        return "unknown"

    def get_char_name(char_id: str) -> str:
        char = characters.get(char_id, {})
        lang_id = char.get("nameLangId", "")
        lang = lang_chars.get(lang_id, {})
        if lang.get("text"):
            return lang["text"]
        return char.get("nameEng", char_id)

    def get_card_name(card: dict) -> str:
        lang_id = card.get("nameLangId", "")
        lang = lang_cards.get(lang_id, {})
        if lang.get("text"):
            return lang["text"]
        return ""

    # --- Generate cards ---
    print("\nGenerating card data...")
    generated_cards = []
    id_map = {}
    existing_ids = set()
    variant_chars = {}
    unsupported = {"unknown_effect_types": [], "unknown_trigger_types": [], "cards_with_issues": []}

    for card in r5_cards:
        card_id_db = card["id"]
        char_id = card.get("characterId", "")
        char = characters.get(char_id, {})

        name_eng = char.get("nameEng", "unknown")
        holosolve_id = make_holosolve_id(name_eng, 5, existing_ids, variant_chars)

        character_name = get_char_name(char_id)
        card_name = get_card_name(card)
        attribute = ATTRIBUTE_MAP.get(card.get("attributeType", ""), "unknown")
        group_name = get_group_name(char_id)

        level_group_id = card.get("cardLevelGroupId", "")
        level_limit_group = card.get("cardLevelLimitGroupId", "")
        limits = level_limits.get(level_limit_group, {0: 40, 1: 50, 2: 60, 3: 70, 4: 80})

        perf_permil = card.get("performancePermilMultiply", 0)
        tech_permil = card.get("techniquePermilMultiply", 0)
        sense_permil = card.get("sensePermilMultiply", 0)

        active_skill_id = card.get("liveActiveSkillId", "")
        passive_skill_id = card.get("livePassiveSkillId", "")
        special_skill_id = card.get("liveSpecialSkillId", "")

        costume_id = card.get("rewardCostumeId", "")
        costume = costumes.get(costume_id, {})
        leader_skill_id = costume.get("liveLeaderSkillId", "")
        leader_skill = leader_skills.get(leader_skill_id, {})

        costume_skill = convert_leader_skill(
            leader_skill, passive_effects_by_group, targets_by_id, triggers_by_group
        )

        is_variant = variant_chars.get(re.sub(r"[^a-z0-9]+", "_", name_eng.lower()).strip("_"), 0) > 1

        potential_data = []
        level_table = level_tables.get(level_group_id, {})
        ref_base_value = int(level_table.get("80", "0"))

        for pot in range(5):
            active_lv, passive_lv, special_lv = get_skill_levels(pot)
            param_bonus = get_param_bonus(pot)

            multiplier = 1000 + param_bonus
            perf = (ref_base_value * perf_permil * multiplier + 999_999) // 1_000_000
            tech = (ref_base_value * tech_permil * multiplier + 999_999) // 1_000_000
            sense = (ref_base_value * sense_permil * multiplier + 999_999) // 1_000_000

            active_skill_data = active_skills_by_id.get((active_skill_id, active_lv))
            if active_skill_data:
                center_skill = build_active_skill(active_skill_data, active_effects_by_group, triggers_by_group)
            else:
                center_skill = {"interval": 20, "probability": "medium", "duration": 8, "score_up": 0, "condition": None, "conditional_score_up": None}

            passive_skill_data = passive_skills_by_id.get((passive_skill_id, passive_lv))
            if passive_skill_data:
                support_skill = build_passive_skill(passive_skill_data, passive_effects_by_group, targets_by_id, triggers_by_group)
            else:
                support_skill = {"effect_type": "unknown", "value": 0, "condition": None, "target": "self"}

            special_skill_data = special_skills_by_id.get((special_skill_id, special_lv))
            if special_skill_data:
                special_skill = build_special_skill(special_skill_data, active_effects_by_group, triggers_by_group)
            else:
                special_skill = {"duration": 10, "score_support": 0}

            potential_data.append({
                "potential": pot,
                "param_bonus_permil": param_bonus,
                "ref_stats_lv80": {
                    "performance": perf,
                    "technique": tech,
                    "sense": sense,
                },
                "center_skill": center_skill,
                "support_skill": support_skill,
                "costume_skill": costume_skill,
                "special_skill": special_skill,
            })

        card_entry = {
            "id": holosolve_id,
            "holodori_id": card_id_db,
            "character": character_name,
            "card_name": card_name,
            "rarity": 5,
            "type": attribute,
            "group": group_name,
            "card_level_group_id": level_group_id,
            "permil": {
                "performance": perf_permil,
                "technique": tech_permil,
                "sense": sense_permil,
            },
            "potential_data": potential_data,
        }

        if is_variant:
            card_entry["variant"] = "水着"

        generated_cards.append(card_entry)
        id_map[card_id_db] = holosolve_id

        print(f"  {holosolve_id}: {character_name} ({attribute}) - {card_name}")

    # --- Write output ---
    output_dir = Path(__file__).parent.parent / "data"
    output_dir.mkdir(exist_ok=True)

    output = {
        "version": 2,
        "generated": datetime.now(timezone.utc).isoformat(),
        "source": "HolodoriDB/holodori-db-jpn-diff",
        "source_commit": "main",
        "level_tables": level_tables,
        "cards": generated_cards,
    }

    cards_path = output_dir / "cards.json"
    with open(cards_path, "w", encoding="utf-8") as f:
        json.dump(output, f, ensure_ascii=False, indent=2)
    print(f"\nWrote {cards_path} ({len(generated_cards)} cards)")

    id_map_path = output_dir / "id_map.json"
    with open(id_map_path, "w", encoding="utf-8") as f:
        json.dump(id_map, f, ensure_ascii=False, indent=2)
    print(f"Wrote {id_map_path}")

    if any(unsupported.values()):
        unsup_path = output_dir / "unsupported.json"
        with open(unsup_path, "w", encoding="utf-8") as f:
            json.dump(unsupported, f, ensure_ascii=False, indent=2)
        print(f"Wrote {unsup_path}")

    # --- Generate songs.json ---
    print("\nGenerating songs data...")
    lang_music = index_by(masters.get("LangMusic_Jpn", []), "id")
    songs = []
    for m in masters.get("Music", []):
        coeff = int(m.get("liveScoreCoefficientPermil", "0"))
        playing_sec = int(m.get("playingSeconds", "0"))
        if playing_sec <= 0:
            continue
        title_lang_id = m.get("titleLangId", "")
        name = lang_music.get(title_lang_id, {}).get("text", "")
        if not name:
            name = m.get("id", "")
        songs.append({
            "id": m.get("id", ""),
            "name": name,
            "playing_seconds": playing_sec,
            "score_coefficient_permil": coeff,
        })
    songs.sort(key=lambda s: s["name"])

    combo_entries = masters.get("LiveCombo", [])
    combo_table = []
    for e in combo_entries:
        combo_table.append({
            "threshold": int(e.get("threshold", "0")),
            "score_up_permil": int(e.get("scoreUpPermil", "0")),
        })
    combo_table.sort(key=lambda x: x["threshold"])

    songs_path = output_dir / "songs.json"
    with open(songs_path, "w", encoding="utf-8") as f:
        json.dump({"songs": songs, "combo_table": combo_table}, f, ensure_ascii=False, indent=2)
    print(f"Wrote {songs_path} ({len(songs)} songs, {len(combo_table)} combo breakpoints)")

    # --- Generate board_effects.json ---
    print("\nGenerating board effects data...")
    tree_nodes = masters.get("SkillTreeNode", [])
    tree_effects = index_by(masters.get("SkillTreeEffect", []), "id")

    cd_reduce_nodes = []
    activation_up_nodes = []

    for node in tree_nodes:
        if "CARD" not in node.get("type", ""):
            continue
        effect_id = node.get("skillTreeEffectId", "")
        effect = tree_effects.get(effect_id, {})
        effect_type = effect.get("effectType", "")
        value = int(effect.get("value", "0"))
        grade = node.get("grade", 1)
        cost_pts = node.get("consumptionSkillTreePointQuantity", 0)
        items = node.get("consumptions", [])

        entry = {
            "node_group": node.get("groupId", ""),
            "value_permil": value,
            "grade": grade,
            "cost_points": cost_pts,
            "cost_items": [{"id": it["resourceId"], "quantity": int(it["quantity"])} for it in items],
        }

        if "COOL_TIME_SHORTEN" in effect_type:
            cd_reduce_nodes.append(entry)
        elif "ACTIVATION_PROBABILITY" in effect_type:
            activation_up_nodes.append(entry)

    cd_reduce_nodes.sort(key=lambda x: x["node_group"])
    activation_up_nodes.sort(key=lambda x: (x["value_permil"], x["node_group"]))

    board_effects = {
        "generated": datetime.now(timezone.utc).isoformat(),
        "source": "HolodoriDB/holodori-db-jpn-diff SkillTreeNode + SkillTreeEffect",
        "cd_reduce": {
            "nodes": cd_reduce_nodes,
            "per_node_permil": cd_reduce_nodes[0]["value_permil"] if cd_reduce_nodes else 40,
            "max_nodes": len(cd_reduce_nodes),
            "total_permil": sum(n["value_permil"] for n in cd_reduce_nodes),
        },
        "activation_up": {
            "nodes": activation_up_nodes,
            "total_permil": sum(n["value_permil"] for n in activation_up_nodes),
        },
    }

    board_path = output_dir / "board_effects.json"
    with open(board_path, "w", encoding="utf-8") as f:
        json.dump(board_effects, f, ensure_ascii=False, indent=2)
    print(f"Wrote {board_path} (cdReduce: {len(cd_reduce_nodes)} nodes/{board_effects['cd_reduce']['total_permil']}‰, activationUp: {len(activation_up_nodes)} nodes/{board_effects['activation_up']['total_permil']}‰)")


if __name__ == "__main__":
    main()
