"""HoloSolve スコア計算の回帰テスト（実測値ベース）"""

import pytest

from solver import calibrate, evaluate_team, load_cards, solve

TEAM_IDS = ["nakiri_ayame_5", "houshou_marine_5", "momosuzu_nene_5", "hakui_koyori_5", "shirogane_noel_swim_5"]


@pytest.fixture
def card_map():
    return {c["id"]: c for c in load_cards()}


@pytest.fixture
def cal():
    return calibrate(TEAM_IDS, "houshou_marine_5", 678413, "hakui_koyori_5", 642056)


def _team(card_map, ids):
    return [card_map[cid] for cid in ids]


def test_card_count():
    assert len(load_cards()) == 59


def test_calibration(cal):
    assert abs(cal["stat_scale"] - 0.742165) < 0.001
    assert abs(cal["baseline"] - 10) < 50


def test_marine_leader_unit_score(card_map, cal):
    rm = evaluate_team(_team(card_map, TEAM_IDS), 1, stat_scale=cal["stat_scale"], baseline=cal["baseline"])
    assert abs(rm["unit_score"] - 678413) < 5


def test_koyori_leader_unit_score(card_map, cal):
    rk = evaluate_team(_team(card_map, TEAM_IDS), 3, stat_scale=cal["stat_scale"], baseline=cal["baseline"])
    assert abs(rk["unit_score"] - 642056) < 5


def test_score_bonus_marine(card_map):
    rm = evaluate_team(_team(card_map, TEAM_IDS), 1)
    assert abs(rm["active_pct"] - 67.1) < 0.2
    assert abs(rm["special_pct"] - 39.2) < 0.2
    assert rm["costume_sb_pct"] == 0.0


def test_score_bonus_koyori(card_map):
    rk = evaluate_team(_team(card_map, TEAM_IDS), 3)
    assert abs(rk["costume_sb_pct"] - 17.0) < 0.2
    assert abs(rk["score_bonus"] - 130.3) < 0.5


def test_perf_135_beats_all_50(card_map):
    """パフォ135%UPが全パラ50%UPより強い（パフォ比率42%のチーム）"""
    team_ids = ["nakiri_ayame_5", "oozora_subaru_5", "shiranui_flare_5", "kobo_kanaeru_5", "shirogane_noel_swim_5"]
    team = _team(card_map, team_ids)
    assert evaluate_team(team, 4)["unit_score"] > evaluate_team(team, 1)["unit_score"]


def test_solve_fixed_leader(card_map):
    """リーダー固定でsolveが動作し、指定リーダーが全結果のleader_idであること"""
    cards = ["nakiri_ayame_5", "houshou_marine_5", "momosuzu_nene_5", "hakui_koyori_5",
             "shirogane_noel_swim_5", "oozora_subaru_5"]
    r = solve(cards, fixed_leader_id="houshou_marine_5")
    assert r["total_combinations"] > 0
    assert all(x["leader_id"] == "houshou_marine_5" for x in r["results"])


def test_solve_rejects_unowned_leader():
    r = solve(TEAM_IDS, fixed_leader_id="tokino_sora_5")
    assert r["total_combinations"] == 0


def test_same_character_excluded(card_map):
    """同キャラ別カードが同一チームに入らない"""
    cards = ["shirogane_noel_5", "shirogane_noel_swim_5", "nakiri_ayame_5",
             "houshou_marine_5", "momosuzu_nene_5", "hakui_koyori_5"]
    r = solve(cards)
    for x in r["results"]:
        chars = [card_map[mid]["character"] for mid in x["member_ids"]]
        assert len(set(chars)) == 5, f"duplicate character in team: {chars}"


def test_js_constants_match_python():
    """build_static.py が生成する JS の定数が solver.py と一致する"""
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
