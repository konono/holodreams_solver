"""HoloSolve生成データをYagoo-dori生成データと突き合わせ検証する

Usage:
    uv run python scripts/validate_against_yagoo.py
"""

import json
import sys
import urllib.request
from pathlib import Path

YAGOO_PUBLIC_URL = "https://raw.githubusercontent.com/asciisyaez/yagoo-dori/main/data/generated/holodori-public.json"


def fetch_json(url: str) -> dict:
    with urllib.request.urlopen(url) as resp:
        return json.loads(resp.read().decode("utf-8"))


def main():
    data_path = Path(__file__).parent.parent / "data" / "cards.json"
    with open(data_path, encoding="utf-8") as f:
        holosolve = json.load(f)

    hs_cards = {c["holodori_id"]: c for c in holosolve["cards"] if "holodori_id" in c}

    print("Fetching Yagoo-dori holodori-public.json...")
    yagoo_data = fetch_json(YAGOO_PUBLIC_URL)
    yagoo = yagoo_data.get("cards", yagoo_data) if isinstance(yagoo_data, dict) else yagoo_data

    r5_yagoo = [c for c in yagoo if c.get("rarity") == 5]
    print(f"Yagoo-dori R5 cards: {len(r5_yagoo)}")
    print(f"HoloSolve R5 cards: {len(hs_cards)}")

    mismatches = []
    missing_in_hs = []
    missing_in_yagoo = []

    yagoo_by_id = {c["id"]: c for c in r5_yagoo}

    for card_id, yg_card in yagoo_by_id.items():
        if card_id not in hs_cards:
            missing_in_hs.append(card_id)
            continue

        hs_card = hs_cards[card_id]
        pot4 = hs_card["potential_data"][4]
        hs_stats = pot4["ref_stats_lv80"]

        yg_params = yg_card.get("parameters", {}).get("maxPotential", {})
        if not yg_params:
            continue

        for stat in ("performance", "technique", "sense"):
            hs_val = hs_stats.get(stat, 0)
            yg_val = yg_params.get(stat, 0)
            diff = abs(hs_val - yg_val)
            if diff > 0:
                mismatches.append({
                    "card_id": card_id,
                    "hs_id": hs_card["id"],
                    "stat": stat,
                    "holosolve": hs_val,
                    "yagoo": yg_val,
                    "diff": diff,
                })

    for card_id in hs_cards:
        if card_id not in yagoo_by_id:
            missing_in_yagoo.append(card_id)

    print(f"\n=== Results ===")
    print(f"Missing in HoloSolve: {len(missing_in_hs)}")
    for cid in missing_in_hs:
        yg = yagoo_by_id[cid]
        print(f"  {cid}: {yg.get('talentName', '?')} - {yg.get('titleJa', '?')}")

    print(f"Missing in Yagoo-dori: {len(missing_in_yagoo)}")
    for cid in missing_in_yagoo:
        hs = hs_cards[cid]
        print(f"  {cid}: {hs['character']} - {hs['card_name']}")

    print(f"\nStats mismatches (4凸 Lv80): {len(mismatches)}")
    if mismatches:
        for m in mismatches[:20]:
            print(f"  {m['hs_id']} ({m['stat']}): HoloSolve={m['holosolve']} Yagoo={m['yagoo']} diff={m['diff']}")
        if len(mismatches) > 20:
            print(f"  ... and {len(mismatches) - 20} more")

    max_diff = max((m["diff"] for m in mismatches), default=0)
    print(f"\nMax stat difference: {max_diff}")

    if max_diff <= 1:
        print("OK: All stats within ±1 (rounding difference only)")
    elif max_diff <= 5:
        print("WARN: Small differences detected (likely formula variant)")
    else:
        print("ERROR: Significant differences detected")

    return 0 if max_diff <= 1 else 1


if __name__ == "__main__":
    sys.exit(main())
