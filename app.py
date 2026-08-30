"""FastAPI サーバー — HoloSolve"""

import json
from pathlib import Path

import uvicorn
from fastapi import FastAPI, HTTPException
from fastapi.responses import FileResponse
from pydantic import BaseModel, field_validator

from solver import _load_card_data, load_cards, resolve_card
from solver_go_bridge import calibrate, recommend, solve

app = FastAPI(title="HoloSolve")

ROOT = Path(__file__).parent


def _card_map():
    return {c["id"]: c for c in load_cards()}


class CardSpec(BaseModel):
    id: str
    potential: int = 0
    level: int | None = None


class CardPotentialSpec(BaseModel):
    potential: int = 0
    level: int | None = None


def _validate_song_length(v):
    if v is not None and v <= 0:
        raise ValueError("song_length must be positive")
    return v


class SolveRequest(BaseModel):
    card_ids: list[str] | None = None
    cards: list[CardSpec] | None = None
    stat_scale: float = 1.0
    baseline: float = 0
    fixed_leader_id: str | None = None
    costume_only_leader_id: str | None = None
    top_n: int = 10
    song_length: float | None = None
    stability_lengths: list[float] | None = None
    sweep_costumes: bool = False
    chart_score: dict | None = None
    stability_charts: list[dict] | None = None

    @field_validator("song_length")
    @classmethod
    def check_song_length(cls, v):
        return _validate_song_length(v)


class CalibrateRequest(BaseModel):
    member_ids: list[str]
    leader_id_1: str
    game_score_1: int
    leader_id_2: str
    game_score_2: int
    card_specs: dict[str, CardPotentialSpec] | None = None
    song_length: float | None = None

    @field_validator("song_length")
    @classmethod
    def check_song_length(cls, v):
        return _validate_song_length(v)

    @field_validator("member_ids")
    @classmethod
    def must_have_5_members(cls, v):
        if len(v) != 5:
            raise ValueError(f"member_ids must have exactly 5 entries, got {len(v)}")
        return v


@app.get("/")
async def index():
    return FileResponse(ROOT / "index.html")


@app.get("/api/cards")
async def get_cards():
    data = _load_card_data()
    return {"cards": load_cards(), "level_tables": data.get("level_tables", {})}


@app.get("/api/songs")
async def get_songs():
    songs_path = ROOT / "data" / "songs.json"
    if songs_path.exists():
        with open(songs_path, encoding="utf-8") as f:
            return json.load(f)
    return {"songs": []}


@app.get("/api/chart_scores")
async def get_chart_scores():
    path = ROOT / "data" / "chart_scores.json"
    if path.exists():
        return FileResponse(path, media_type="application/json")
    return {}


@app.post("/api/solve")
def post_solve(req: SolveRequest):
    cm = _card_map()

    if req.cards is not None:
        if not req.cards:
            actual = [{"id": cid, "potential": 0} for cid in cm.keys()]
            dropped = 0
        else:
            card_specs = [{"id": c.id, "potential": c.potential, "level": c.level} for c in req.cards]
            actual_ids = [c.id for c in req.cards if c.id in cm]
            dropped = len(req.cards) - len(actual_ids)
            actual = [s for s in card_specs if s["id"] in cm]
    elif req.card_ids is not None:
        if not req.card_ids:
            actual = [{"id": cid, "potential": 0} for cid in cm.keys()]
            dropped = 0
        else:
            actual = [{"id": cid, "potential": 0} for cid in req.card_ids if cid in cm]
            dropped = len(req.card_ids) - len(actual)
    else:
        actual = [{"id": cid, "potential": 0} for cid in cm.keys()]
        dropped = 0

    warnings = []
    if req.fixed_leader_id and req.costume_only_leader_id:
        warnings.append("fixed_leader_id と costume_only_leader_id が同時に指定されました。fixed_leader_id を優先し、costume_only_leader_id は無視されます。")
    kwargs = dict(top_n=req.top_n, stat_scale=req.stat_scale, baseline=req.baseline, fixed_leader_id=req.fixed_leader_id, costume_only_leader_id=req.costume_only_leader_id)
    if req.song_length is not None:
        kwargs["song_length"] = req.song_length
    if req.stability_lengths:
        kwargs["stability_lengths"] = req.stability_lengths
    if req.sweep_costumes:
        kwargs["sweep_costumes"] = True
    if req.chart_score:
        kwargs["chart_score"] = req.chart_score
    if req.stability_charts:
        kwargs["stability_charts"] = req.stability_charts
    result = solve(actual, **kwargs)
    if dropped > 0:
        warnings.append(f"{dropped}枚の不明なカードIDを除外しました")
    if warnings:
        result["warnings"] = warnings
    return result


class RecommendRequest(BaseModel):
    cards: list[CardSpec]
    stat_scale: float = 1.0
    baseline: float = 0
    fixed_leader_id: str | None = None
    costume_only_leader_id: str | None = None
    top_n: int = 5
    acquire_count: int = 1
    song_length: float | None = None
    sweep_costumes: bool = False

    @field_validator("song_length")
    @classmethod
    def check_song_length(cls, v):
        return _validate_song_length(v)


@app.post("/api/recommend")
def post_recommend(req: RecommendRequest):
    if not req.cards:
        raise HTTPException(status_code=400, detail="レコメンドにはカードの選択が必要です")

    cm = _card_map()
    seen = set()
    card_specs = []
    for c in req.cards:
        if c.id in cm and c.id not in seen:
            seen.add(c.id)
            card_specs.append({"id": c.id, "potential": c.potential, "level": c.level})

    if len(card_specs) < 5:
        raise HTTPException(status_code=400, detail="レコメンドには5枚以上のカードが必要です")

    unique_chars = {cm[s["id"]]["character"] for s in card_specs}
    if len(unique_chars) < 5:
        raise HTTPException(status_code=400, detail="レコメンドには5キャラ以上のカードが必要です")

    owned_ids = {s["id"] for s in card_specs}
    if req.fixed_leader_id and req.fixed_leader_id not in owned_ids:
        raise HTTPException(status_code=400, detail="リーダーは選択カードに含まれている必要があります")

    dropped = len(req.cards) - len(card_specs)

    top_n = max(1, min(req.top_n, 20))
    acquire_count = max(1, min(req.acquire_count, 5))
    kwargs = dict(top_n=top_n, acquire_count=acquire_count, stat_scale=req.stat_scale, baseline=req.baseline, fixed_leader_id=req.fixed_leader_id, costume_only_leader_id=req.costume_only_leader_id)
    if req.song_length is not None:
        kwargs["song_length"] = req.song_length
    if req.sweep_costumes:
        kwargs["sweep_costumes"] = True

    result = recommend(card_specs, **kwargs)
    if dropped > 0:
        result.setdefault("warnings", []).append(f"{dropped}枚の不明または重複カードIDを除外しました")
    return result


@app.post("/api/calibrate")
def post_calibrate(req: CalibrateRequest):
    cm = _card_map()
    for mid in req.member_ids:
        if mid not in cm:
            raise HTTPException(status_code=400, detail=f"不明なカードID: {mid}")
    if req.leader_id_1 not in req.member_ids or req.leader_id_2 not in req.member_ids:
        raise HTTPException(status_code=400, detail="リーダーはメンバーに含まれている必要があります")
    if req.leader_id_1 == req.leader_id_2:
        raise HTTPException(status_code=400, detail="異なるリーダーを選んでください")
    if req.game_score_1 <= 0 or req.game_score_2 <= 0:
        raise HTTPException(status_code=400, detail="スコアは正の整数で入力してください")

    card_specs_dict = {}
    if req.card_specs:
        for cid, spec in req.card_specs.items():
            card_specs_dict[cid] = {"potential": spec.potential, "level": spec.level}

    resolved_cards = []
    for mid in req.member_ids:
        spec = card_specs_dict.get(mid, {})
        resolved_cards.append(resolve_card(cm[mid], potential=spec.get("potential", 0), level=spec.get("level")))

    chars = [c["character"] for c in resolved_cards]
    if len(set(chars)) < 5:
        raise HTTPException(status_code=400, detail="同キャラの別カードは同時編成できません")

    kwargs = dict(card_specs=card_specs_dict)
    if req.song_length is not None:
        kwargs["song_length"] = req.song_length

    return calibrate(
        req.member_ids,
        req.leader_id_1,
        req.game_score_1,
        req.leader_id_2,
        req.game_score_2,
        **kwargs,
    )


if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=8000)
