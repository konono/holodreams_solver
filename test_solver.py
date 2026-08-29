"""HoloSolve スコア計算の回帰テスト（実測値ベース + v2凸対応）"""

import pytest

from solver import calibrate, evaluate_team, load_cards, recommend, resolve_card, solve

TEAM_IDS = ["nakiri_ayame_5", "houshou_marine_5", "momosuzu_nene_5", "hakui_koyori_5", "shirogane_noel_swim_5"]


@pytest.fixture
def card_map():
    return {c["id"]: c for c in load_cards()}


@pytest.fixture
def resolved_team_4(card_map):
    return [resolve_card(card_map[cid], potential=4, level=80) for cid in TEAM_IDS]


def test_card_count():
    assert len(load_cards()) >= 59


def test_resolve_card_potential_0(card_map):
    sora = card_map["tokino_sora_5"]
    r = resolve_card(sora, potential=0, level=40)
    assert r["stats"]["performance"] > 0
    assert r["level"] == 40


def test_resolve_card_potential_4(card_map):
    sora = card_map["tokino_sora_5"]
    r = resolve_card(sora, potential=4, level=80)
    assert r["center_skill"]["score_up"] == 100
    assert r["support_skill"]["value"] == 33
    assert r["special_skill"]["score_support"] == 160


def test_potential_independent_of_level(card_map):
    """凸とレベルは独立: 同レベルでも凸が違えばスキル値が変わる"""
    sora = card_map["tokino_sora_5"]
    r0_lv80 = resolve_card(sora, potential=0, level=80)
    r4_lv80 = resolve_card(sora, potential=4, level=80)
    assert r4_lv80["stats"]["performance"] > r0_lv80["stats"]["performance"]
    assert r4_lv80["support_skill"]["value"] >= r0_lv80["support_skill"]["value"]


def test_resolve_card_custom_level(card_map):
    sora = card_map["tokino_sora_5"]
    r_high = resolve_card(sora, potential=0, level=80)
    r_low = resolve_card(sora, potential=0, level=20)
    assert r_low["stats"]["performance"] < r_high["stats"]["performance"]
    assert r_low["level"] == 20


def test_resolve_card_level_clamped(card_map):
    sora = card_map["tokino_sora_5"]
    r = resolve_card(sora, potential=0, level=100)
    assert r["level"] == 80


def test_potential_stats_bonus_at_2(card_map):
    """2凸以上でステボーナス+100 permilが適用される"""
    sora = card_map["tokino_sora_5"]
    r1 = resolve_card(sora, potential=1, level=80)
    r2 = resolve_card(sora, potential=2, level=80)
    assert r2["total"] > r1["total"], "2凸 should have higher stats due to +100 permil bonus"


def test_potential_skill_levels(card_map):
    """凸によるスキルレベル変化を確認"""
    marine = card_map["houshou_marine_5"]
    r0 = resolve_card(marine, potential=0)
    r1 = resolve_card(marine, potential=1)
    r4 = resolve_card(marine, potential=4)
    assert r1["center_skill"]["score_up"] >= r0["center_skill"]["score_up"]
    assert r4["support_skill"]["value"] >= r0["support_skill"]["value"]


def test_score_bonus_4pot(resolved_team_4):
    rm = evaluate_team(resolved_team_4, 1)
    assert rm["active_pct"] > 50
    assert rm["special_pct"] > 0


def test_solve_with_string_ids():
    """list[str]入力の後方互換（0凸として扱う）"""
    ids = ["tokino_sora_5", "robocosan_5", "sakura_miko_5", "hoshimachi_suisei_5", "aki_rosenthal_5", "akai_haato_5"]
    r = solve(ids, top_n=2)
    assert r["total_combinations"] > 0
    assert len(r["results"]) > 0


def test_solve_with_card_specs():
    """CardSpec形式の凸・レベル混在入力"""
    specs_high = [
        {"id": "tokino_sora_5", "potential": 4, "level": 80},
        {"id": "robocosan_5", "potential": 4, "level": 80},
        {"id": "sakura_miko_5", "potential": 4, "level": 80},
        {"id": "hoshimachi_suisei_5", "potential": 4, "level": 80},
        {"id": "aki_rosenthal_5", "potential": 4, "level": 80},
        {"id": "akai_haato_5", "potential": 4, "level": 80},
    ]
    r_high = solve(specs_high, top_n=2)
    assert r_high["total_combinations"] > 0

    r0 = solve([s["id"] for s in specs_high], top_n=2)
    score_0pot = r0["results"][0]["unit_score"]
    score_4pot = r_high["results"][0]["unit_score"]
    assert score_4pot > score_0pot, "4凸 should give higher score than 0凸 at same level"


def test_solve_fixed_leader():
    cards = [
        {"id": "nakiri_ayame_5", "potential": 4, "level": 80},
        {"id": "houshou_marine_5", "potential": 4, "level": 80},
        {"id": "momosuzu_nene_5", "potential": 4, "level": 80},
        {"id": "hakui_koyori_5", "potential": 4, "level": 80},
        {"id": "shirogane_noel_swim_5", "potential": 4, "level": 80},
        {"id": "oozora_subaru_5", "potential": 4, "level": 80},
    ]
    r = solve(cards, fixed_leader_id="houshou_marine_5")
    assert r["total_combinations"] > 0
    assert all(x["leader_id"] == "houshou_marine_5" for x in r["results"])


def test_solve_rejects_unowned_leader():
    r = solve(TEAM_IDS, fixed_leader_id="tokino_sora_5")
    assert r["total_combinations"] == 0


def test_solve_costume_only_applies_costume_skill():
    """costume_only_leader_id で衣装スキルが適用され、メンバーにも含まれ得る"""
    cards = [
        {"id": "nakiri_ayame_5", "potential": 4, "level": 80},
        {"id": "houshou_marine_5", "potential": 4, "level": 80},
        {"id": "momosuzu_nene_5", "potential": 4, "level": 80},
        {"id": "hakui_koyori_5", "potential": 4, "level": 80},
        {"id": "shirogane_noel_swim_5", "potential": 4, "level": 80},
        {"id": "oozora_subaru_5", "potential": 4, "level": 80},
    ]
    r = solve(cards, costume_only_leader_id="houshou_marine_5")
    assert r["total_combinations"] > 0
    for x in r["results"]:
        assert x["costume_only_leader_id"] == "houshou_marine_5"


def test_solve_costume_only_with_fixed_leader_ignores_costume_only():
    """fixed_leader_id と costume_only_leader_id 両方指定時は fixed が優先し、costume_only の副作用がない"""
    cards = [
        {"id": "nakiri_ayame_5", "potential": 4, "level": 80},
        {"id": "houshou_marine_5", "potential": 4, "level": 80},
        {"id": "momosuzu_nene_5", "potential": 4, "level": 80},
        {"id": "hakui_koyori_5", "potential": 4, "level": 80},
        {"id": "shirogane_noel_swim_5", "potential": 4, "level": 80},
        {"id": "oozora_subaru_5", "potential": 4, "level": 80},
    ]
    r_fixed = solve(cards, fixed_leader_id="nakiri_ayame_5")
    r_both = solve(cards, fixed_leader_id="nakiri_ayame_5", costume_only_leader_id="houshou_marine_5")
    assert all(x["leader_id"] == "nakiri_ayame_5" for x in r_both["results"])
    assert r_fixed["results"][0]["unit_score"] == r_both["results"][0]["unit_score"], \
        "Both-specified should produce same score as fixed-only"


def test_same_character_excluded(card_map):
    cards = [
        {"id": "shirogane_noel_5", "potential": 4, "level": 80},
        {"id": "shirogane_noel_swim_5", "potential": 4, "level": 80},
        {"id": "nakiri_ayame_5", "potential": 4, "level": 80},
        {"id": "houshou_marine_5", "potential": 4, "level": 80},
        {"id": "momosuzu_nene_5", "potential": 4, "level": 80},
        {"id": "hakui_koyori_5", "potential": 4, "level": 80},
    ]
    r = solve(cards)
    for x in r["results"]:
        chars = [card_map[mid]["character"] for mid in x["member_ids"]]
        assert len(set(chars)) == 5, f"duplicate character in team: {chars}"


def test_order_optimization(card_map):
    from solver import optimize_order
    team_ids = ["nakiri_ayame_5", "hakui_koyori_5", "otonose_kanade_5", "houshou_marine_5", "shirogane_noel_swim_5"]
    resolved = [resolve_card(card_map[cid], potential=4) for cid in team_ids]
    r_default = evaluate_team(resolved, 3)
    specs = {cid: {"potential": 4} for cid in team_ids}
    r_opt = optimize_order(team_ids, "houshou_marine_5", card_specs=specs)
    assert r_opt["unit_score"] >= r_default["unit_score"]


def test_activation_probability_stored(card_map):
    """activation_probability_permilが数値で保持されている"""
    sora = card_map["tokino_sora_5"]
    r = resolve_card(sora, potential=0, level=80)
    prob = r["center_skill"].get("activation_probability_permil")
    assert prob is not None
    assert isinstance(prob, int)
    assert 300 <= prob <= 700


def test_ceil_divide_stats(card_map):
    """ceilDivide方式でYagoo-doriと一致するstatsが生成される"""
    sora = card_map["tokino_sora_5"]
    r4 = resolve_card(sora, potential=4, level=80)
    assert r4["stats"]["performance"] == 10909
    assert r4["stats"]["technique"] == 7221
    assert r4["stats"]["sense"] == 7844


def test_costume_condition_type_count(card_map):
    """type_count条件のcostume_skillが正しく評価される"""
    team = [resolve_card(card_map[cid], potential=4, level=80) for cid in
            ["robocosan_5", "natsuiro_matsuri_5", "inugami_korone_5", "houshou_marine_5", "shirogane_noel_5"]]
    result = evaluate_team(team, 0)
    assert result["unit_score"] > 0


def test_center_type_condition_applied(card_map):
    """タイプ条件のActive skillでconditional_score_upが適用される"""
    from solver import _compute_team_scores
    team = [resolve_card(card_map[cid], potential=4, level=80) for cid in
            ["robocosan_5", "natsuiro_matsuri_5", "inugami_korone_5", "houshou_marine_5", "shirogane_noel_5"]]
    s = _compute_team_scores(team, 0)
    assert s["active_pct"] > 50


def test_center_life_combo_condition_ignored(card_map):
    """ライフ/コンボ条件はbase score_upを使用"""
    from solver import _check_center_type_condition
    type_counts = {"happy": 3, "pure": 1, "cute": 1}
    assert _check_center_type_condition("life_600", type_counts, {}) is False
    assert _check_center_type_condition("combo_40", type_counts, {}) is False
    assert _check_center_type_condition("happy_2", type_counts, {}) is True


def test_expected_maximum_less_than_linear_sum(card_map):
    """Expected Maximumモデルは線形和以下のactive_pctを生成する"""
    from solver import _compute_team_scores
    team = [resolve_card(card_map[cid], potential=4, level=80) for cid in
            ["houshou_marine_5", "hakui_koyori_5", "shirogane_noel_swim_5", "nakiri_ayame_5", "momosuzu_nene_5"]]
    s = _compute_team_scores(team, 0)
    linear_sum = sum(
        c["center_skill"]["score_up"] * c["center_skill"]["duration"] / c["center_skill"]["interval"]
        * c["center_skill"].get("activation_probability_permil", 1000) / 1000
        for c in team
    )
    assert s["active_pct"] <= 52.89 + linear_sum / 12.82 + 0.01


def test_special_rate_up_boosts_active(card_map):
    """skill_rate_upありのチームはActive%が上がる"""
    from solver import _compute_team_scores
    team_ids = ["robocosan_5", "azki_5", "sakura_miko_5", "hoshimachi_suisei_5", "aki_rosenthal_5"]
    team = [resolve_card(card_map[cid], potential=4, level=80) for cid in team_ids]
    s = _compute_team_scores(team, 0)
    assert s["active_pct"] > 52.89


def test_combo_bonus():
    from solver import compute_avg_combo_bonus
    assert compute_avg_combo_bonus(0) == 0.0
    assert compute_avg_combo_bonus(50) == 0.0
    bonus_500 = compute_avg_combo_bonus(500)
    assert 15 < bonus_500 < 30
    bonus_1000 = compute_avg_combo_bonus(1000)
    assert bonus_1000 > bonus_500


def test_level_none_defaults_to_80(card_map):
    """level=None時はLv80として扱う"""
    sora = card_map["tokino_sora_5"]
    r_none = resolve_card(sora, potential=0, level=None)
    r_80 = resolve_card(sora, potential=0, level=80)
    assert r_none["stats"] == r_80["stats"]
    assert r_none["level"] == 80


def test_python_js_stats_parity(card_map):
    """Python ceilDivide式がJS ceilDiv式と同じ結果を返すことを確認"""
    sora = card_map["tokino_sora_5"]
    for pot in [0, 2, 4]:
        for lv in [20, 40, 60, 80]:
            r = resolve_card(sora, potential=pot, level=lv)
            if lv == 80:
                continue
            pd = sora["potential_data"][pot]
            bonus = pd.get("param_bonus_permil", 0)
            mul = 1000 + bonus
            data = __import__("solver")._load_card_data()
            table = data.get("level_tables", {}).get(sora["card_level_group_id"], {})
            base = int(table.get(str(lv), "0"))
            if base == 0:
                continue
            permil = sora["permil"]
            expected_p = (base * permil["performance"] * mul + 999_999) // 1_000_000
            assert r["stats"]["performance"] == expected_p, \
                f"pot={pot} lv={lv}: got {r['stats']['performance']} expected {expected_p}"


def test_solve_stability_lengths():
    """stability_lengths 指定時にレスポンスに stability データが含まれる"""
    cards = [
        {"id": "nakiri_ayame_5", "potential": 4},
        {"id": "houshou_marine_5", "potential": 4},
        {"id": "momosuzu_nene_5", "potential": 4},
        {"id": "hakui_koyori_5", "potential": 4},
        {"id": "shirogane_noel_swim_5", "potential": 4},
        {"id": "oozora_subaru_5", "potential": 4},
    ]
    r = solve(cards, top_n=1, stability_lengths=[90, 120, 150])
    result = r["results"][0]
    assert "stability" in result
    assert set(result["stability"].keys()) == {90, 120, 150}
    for score in result["stability"].values():
        assert isinstance(score, int) and score > 0


def test_solve_no_stability_by_default():
    """stability_lengths 未指定時は stability キーがない"""
    cards = [
        {"id": "nakiri_ayame_5", "potential": 4},
        {"id": "houshou_marine_5", "potential": 4},
        {"id": "momosuzu_nene_5", "potential": 4},
        {"id": "hakui_koyori_5", "potential": 4},
        {"id": "shirogane_noel_swim_5", "potential": 4},
        {"id": "oozora_subaru_5", "potential": 4},
    ]
    r = solve(cards, top_n=1)
    assert "stability" not in r["results"][0]


def test_solve_sweep_costumes():
    """sweep_costumes=True で全衣装を試し、各結果に costume_only_leader_id が付く"""
    cards = [
        {"id": "nakiri_ayame_5", "potential": 0},
        {"id": "houshou_marine_5", "potential": 0},
        {"id": "momosuzu_nene_5", "potential": 0},
        {"id": "hakui_koyori_5", "potential": 0},
        {"id": "shirogane_noel_swim_5", "potential": 0},
        {"id": "oozora_subaru_5", "potential": 0},
    ]
    r = solve(cards, top_n=3, sweep_costumes=True)
    assert r["total_combinations"] > 0
    for x in r["results"]:
        assert x["costume_only_leader_id"] is not None, "Each result should have a costume_only_leader_id"


def test_solve_sweep_costumes_disabled_when_costume_set():
    """costume_only_leader_id 指定時は sweep_costumes=True でもスイープしない"""
    cards = [
        {"id": "nakiri_ayame_5", "potential": 0},
        {"id": "houshou_marine_5", "potential": 0},
        {"id": "momosuzu_nene_5", "potential": 0},
        {"id": "hakui_koyori_5", "potential": 0},
        {"id": "shirogane_noel_swim_5", "potential": 0},
        {"id": "oozora_subaru_5", "potential": 0},
    ]
    r = solve(cards, top_n=1, sweep_costumes=True, costume_only_leader_id="houshou_marine_5")
    for x in r["results"]:
        assert x["costume_only_leader_id"] == "houshou_marine_5"


def test_api_solve_costume_only():
    """API経由で costume_only_leader_id が衣装スキルを適用する"""
    from fastapi.testclient import TestClient
    from app import app
    client = TestClient(app)
    cards = [
        {"id": "nakiri_ayame_5", "potential": 4},
        {"id": "houshou_marine_5", "potential": 4},
        {"id": "momosuzu_nene_5", "potential": 4},
        {"id": "hakui_koyori_5", "potential": 4},
        {"id": "shirogane_noel_swim_5", "potential": 4},
        {"id": "oozora_subaru_5", "potential": 4},
    ]
    r = client.post("/api/solve", json={"cards": cards, "costume_only_leader_id": "houshou_marine_5", "top_n": 3})
    assert r.status_code == 200
    data = r.json()
    for x in data["results"]:
        assert x["costume_only_leader_id"] == "houshou_marine_5"


def test_api_solve_fixed_leader():
    """API経由で fixed_leader_id が全結果でリーダーになる"""
    from fastapi.testclient import TestClient
    from app import app
    client = TestClient(app)
    cards = [
        {"id": "nakiri_ayame_5", "potential": 4},
        {"id": "houshou_marine_5", "potential": 4},
        {"id": "momosuzu_nene_5", "potential": 4},
        {"id": "hakui_koyori_5", "potential": 4},
        {"id": "shirogane_noel_swim_5", "potential": 4},
        {"id": "oozora_subaru_5", "potential": 4},
    ]
    r = client.post("/api/solve", json={"cards": cards, "fixed_leader_id": "houshou_marine_5", "top_n": 3})
    assert r.status_code == 200
    data = r.json()
    for x in data["results"]:
        assert x["leader_id"] == "houshou_marine_5"


def test_api_solve_both_warns():
    """API: fixed + costume_only 同時指定で警告が返る"""
    from fastapi.testclient import TestClient
    from app import app
    client = TestClient(app)
    cards = [
        {"id": "nakiri_ayame_5", "potential": 4},
        {"id": "houshou_marine_5", "potential": 4},
        {"id": "momosuzu_nene_5", "potential": 4},
        {"id": "hakui_koyori_5", "potential": 4},
        {"id": "shirogane_noel_swim_5", "potential": 4},
        {"id": "oozora_subaru_5", "potential": 4},
    ]
    r = client.post("/api/solve", json={"cards": cards, "fixed_leader_id": "houshou_marine_5", "costume_only_leader_id": "nakiri_ayame_5", "top_n": 1})
    assert r.status_code == 200
    data = r.json()
    assert "warnings" in data
    assert any("同時" in w for w in data["warnings"])


def test_api_solve_song_length_zero():
    """song_length=0でバリデーションエラー"""
    from fastapi.testclient import TestClient
    from app import app
    client = TestClient(app)
    r = client.post("/api/solve", json={"card_ids": ["tokino_sora_5"], "song_length": 0})
    assert r.status_code == 422


def test_api_calibrate_song_length_zero():
    """calibrateもsong_length=0でバリデーションエラー"""
    from fastapi.testclient import TestClient
    from app import app
    client = TestClient(app)
    r = client.post("/api/calibrate", json={
        "member_ids": ["nakiri_ayame_5", "houshou_marine_5", "momosuzu_nene_5", "hakui_koyori_5", "shirogane_noel_swim_5"],
        "leader_id_1": "houshou_marine_5", "game_score_1": 678413,
        "leader_id_2": "hakui_koyori_5", "game_score_2": 642056,
        "song_length": -1,
    })
    assert r.status_code == 422


def test_api_calibrate_card_potential_spec():
    """CalibrateRequestがCardPotentialSpec(idなし)を受け付ける"""
    from fastapi.testclient import TestClient
    from app import app
    client = TestClient(app)
    r = client.post("/api/calibrate", json={
        "member_ids": ["nakiri_ayame_5", "houshou_marine_5", "momosuzu_nene_5", "hakui_koyori_5", "shirogane_noel_swim_5"],
        "leader_id_1": "houshou_marine_5", "game_score_1": 678413,
        "leader_id_2": "hakui_koyori_5", "game_score_2": 642056,
        "card_specs": {"houshou_marine_5": {"potential": 4}, "hakui_koyori_5": {"potential": 4}},
    })
    assert r.status_code == 200
    data = r.json()
    assert "stat_scale" in data
    assert "baseline" in data


def test_js_constants_match_python():
    from build_static import _generate_solver_js
    from solver import (
        ACTIVE_BASE, ACTIVE_DIVISOR, COSTUME_SS_RATE,
        SONG_LENGTH, SUPPORT_SS_RATE, UNIT_SCORE_K,
    )
    js = _generate_solver_js()
    assert f"const UNIT_SCORE_K = {UNIT_SCORE_K};" in js
    assert f"const ACTIVE_BASE = {ACTIVE_BASE};" in js
    assert f"const ACTIVE_DIVISOR = {ACTIVE_DIVISOR};" in js
    assert f"const COSTUME_SS_RATE = {COSTUME_SS_RATE};" in js
    assert f"const SUPPORT_SS_RATE = {SUPPORT_SS_RATE};" in js
    assert f"const SONG_LENGTH = {SONG_LENGTH};" in js


# --- recommend() ユニットテスト ---

RECOMMEND_CARDS_7 = [
    "tokino_sora_5", "robocosan_5", "hoshimachi_suisei_5",
    "sakura_miko_5", "shirakami_fubuki_5", "natsuiro_matsuri_5",
    "akai_haato_5",
]


def test_recommend_acquire1_no_multi_uncap():
    """acquire_count=1 では cost=1 の候補のみ"""
    cards = [{"id": cid, "potential": 0} for cid in RECOMMEND_CARDS_7]
    result = recommend(cards, top_n=10, acquire_count=1)
    for r in result["recommendations"]:
        for c in r["cards"]:
            assert c.get("cost", 1) == 1, f"acquire_count=1 should only have cost=1, got {c}"


def test_recommend_acquire2_has_multi_uncap():
    """acquire_count=2 で同一カード複数凸（cost=2）が候補に含まれる"""
    cards = [{"id": cid, "potential": 0} for cid in RECOMMEND_CARDS_7]
    result = recommend(cards, top_n=10, acquire_count=2)
    has_multi = any(
        c["cost"] > 1
        for r in result["recommendations"]
        for c in r["cards"]
    )
    assert has_multi, "Expected at least one multi-uncap (cost>1) recommendation"


def test_recommend_acquire2_cost_sum():
    """各レコメンドの合計コストが acquire_count と一致する"""
    cards = [{"id": cid, "potential": 0} for cid in RECOMMEND_CARDS_7]
    result = recommend(cards, top_n=10, acquire_count=2)
    for r in result["recommendations"]:
        total_cost = sum(c["cost"] for c in r["cards"])
        assert total_cost == 2, f"Expected total cost=2, got {total_cost}: {r['cards']}"


def test_recommend_acquire3_cost_sum():
    """acquire_count=3 でも合計コストが一致する"""
    cards = [{"id": cid, "potential": 0} for cid in RECOMMEND_CARDS_7]
    result = recommend(cards, top_n=10, acquire_count=3)
    for r in result["recommendations"]:
        total_cost = sum(c["cost"] for c in r["cards"])
        assert total_cost == 3, f"Expected total cost=3, got {total_cost}: {r['cards']}"


def test_recommend_no_duplicate_card_ids():
    """同一 card_id がコンボ内に重複しない"""
    cards = [{"id": cid, "potential": 0} for cid in RECOMMEND_CARDS_7]
    result = recommend(cards, top_n=10, acquire_count=2)
    for r in result["recommendations"]:
        card_ids = [c["card_id"] for c in r["cards"]]
        assert len(card_ids) == len(set(card_ids)), f"Duplicate card_ids: {card_ids}"


def test_recommend_multi_uncap_shortlist_preserved():
    """単体候補が多くても複数凸候補が shortlist から落ちない"""
    all_cards = load_cards()
    chars_seen = set()
    cards = []
    for c in all_cards:
        if c["character"] not in chars_seen:
            chars_seen.add(c["character"])
            cards.append({"id": c["id"], "potential": 0})
        if len(cards) >= 9:
            break
    result = recommend(cards, top_n=20, acquire_count=2)
    has_multi = any(
        c["cost"] > 1
        for r in result["recommendations"]
        for c in r["cards"]
    )
    assert has_multi, "Multi-uncap candidates should survive shortlist truncation even with many single candidates"


def test_recommend_delta_positive():
    """全レコメンドの delta が正"""
    cards = [{"id": cid, "potential": 0} for cid in RECOMMEND_CARDS_7]
    result = recommend(cards, top_n=10, acquire_count=2)
    for r in result["recommendations"]:
        assert r["delta"] > 0, f"Expected positive delta, got {r['delta']}"
