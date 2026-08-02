"""FastAPI サーバー — HoloSolve"""

from pathlib import Path

import uvicorn
from fastapi import FastAPI, HTTPException
from fastapi.responses import FileResponse
from pydantic import BaseModel, field_validator

from solver import calibrate, load_cards, solve

app = FastAPI(title="HoloSolve")

ROOT = Path(__file__).parent
MAX_CARDS_FOR_SOLVE = 40


def _card_map():
    return {c["id"]: c for c in load_cards()}


class SolveRequest(BaseModel):
    card_ids: list[str]
    stat_scale: float = 1.0
    baseline: float = 0
    fixed_leader_id: str | None = None


class CalibrateRequest(BaseModel):
    member_ids: list[str]
    leader_id_1: str
    game_score_1: int
    leader_id_2: str
    game_score_2: int

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
    return {"cards": load_cards(), "max_cards": MAX_CARDS_FOR_SOLVE}


@app.post("/api/solve")
def post_solve(req: SolveRequest):
    cm = _card_map()
    actual = [cid for cid in req.card_ids if cid in cm]
    dropped = len(req.card_ids) - len(actual)
    if len(actual) > MAX_CARDS_FOR_SOLVE:
        raise HTTPException(
            status_code=400,
            detail=f"カード数が上限を超えています（{len(actual)}/{MAX_CARDS_FOR_SOLVE}枚）。サーバー版は{MAX_CARDS_FOR_SOLVE}枚まで。全カードで探索する場合は静的版(dist/holosolve.html)をご利用ください。",
        )
    result = solve(actual, stat_scale=req.stat_scale, baseline=req.baseline, fixed_leader_id=req.fixed_leader_id)
    if dropped > 0:
        result["warnings"] = [f"{dropped}枚の不明なカードIDを除外しました"]
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
    chars = [cm[mid]["character"] for mid in req.member_ids]
    if len(set(chars)) < 5:
        raise HTTPException(status_code=400, detail="同キャラの別カードは同時編成できません")
    return calibrate(
        req.member_ids,
        req.leader_id_1,
        req.game_score_1,
        req.leader_id_2,
        req.game_score_2,
    )


if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=8000)
