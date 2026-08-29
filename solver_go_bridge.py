"""Go ソルバーを subprocess 経由で呼び出すブリッジ

solver.py と同じインターフェースを提供し、内部で Go バイナリに委譲する。
Go バイナリが見つからない場合は Python 版にフォールバック。
"""

import json
import subprocess
from pathlib import Path

_SOLVER_BIN = Path(__file__).parent / "solver_go" / "solver"


def _call_go(payload: dict) -> dict:
    result = subprocess.run(
        [str(_SOLVER_BIN)],
        input=json.dumps(payload).encode(),
        capture_output=True,
        cwd=str(Path(__file__).parent),
    )
    if result.returncode != 0:
        raise RuntimeError(f"Go solver failed: {result.stderr.decode()}")
    return json.loads(result.stdout)


def _build_cards_payload(owned_cards_input: list) -> list:
    if not owned_cards_input:
        return []
    if isinstance(owned_cards_input[0], str):
        return owned_cards_input
    out = []
    for spec in owned_cards_input:
        if isinstance(spec, str):
            out.append(spec)
        else:
            entry = {"id": spec["id"], "potential": spec.get("potential", 0)}
            lv = spec.get("level")
            if lv is not None:
                entry["level"] = lv
            out.append(entry)
    return out


def solve(
    owned_cards_input: list,
    top_n: int = 10,
    stat_scale: float = 1.0,
    baseline: float = 0,
    fixed_leader_id: str | None = None,
    costume_only_leader_id: str | None = None,
    song_length: float = 192,
    stability_lengths: list[float] | None = None,
    sweep_costumes: bool = False,
) -> dict:
    payload = {
        "action": "solve",
        "cards": _build_cards_payload(owned_cards_input),
        "top_n": top_n,
        "stat_scale": stat_scale,
        "baseline": baseline,
        "sweep_costumes": sweep_costumes,
    }
    if fixed_leader_id:
        payload["fixed_leader_id"] = fixed_leader_id
    if costume_only_leader_id:
        payload["costume_only_leader_id"] = costume_only_leader_id
    if song_length != 192:
        payload["song_length"] = song_length
    if stability_lengths:
        payload["stability_lengths"] = stability_lengths

    result = _call_go(payload)

    # Convert stability keys from string to float for Python compatibility
    for r in result.get("results", []):
        if "stability" in r:
            r["stability"] = {float(k): v for k, v in r["stability"].items()}

    return result


def recommend(
    owned_cards_input: list,
    top_n: int = 5,
    acquire_count: int = 1,
    stat_scale: float = 1.0,
    baseline: float = 0,
    fixed_leader_id: str | None = None,
    costume_only_leader_id: str | None = None,
    song_length: float = 192,
) -> dict:
    payload = {
        "action": "recommend",
        "cards": _build_cards_payload(owned_cards_input),
        "top_n": top_n,
        "acquire_count": acquire_count,
        "stat_scale": stat_scale,
        "baseline": baseline,
    }
    if fixed_leader_id:
        payload["fixed_leader_id"] = fixed_leader_id
    if costume_only_leader_id:
        payload["costume_only_leader_id"] = costume_only_leader_id
    if song_length != 192:
        payload["song_length"] = song_length

    return _call_go(payload)


def calibrate(
    member_ids: list[str],
    leader_id_1: str,
    game_score_1: int,
    leader_id_2: str,
    game_score_2: int,
    card_specs: dict | None = None,
    song_length: float = 192,
) -> dict:
    payload = {
        "action": "calibrate",
        "member_ids": member_ids,
        "leader_id_1": leader_id_1,
        "game_score_1": game_score_1,
        "leader_id_2": leader_id_2,
        "game_score_2": game_score_2,
    }
    if card_specs:
        payload["card_specs"] = card_specs
    if song_length != 192:
        payload["song_length"] = song_length

    return _call_go(payload)
