"""ネイティブ（サーバー版）とWASM版で共有するテストデータ

test_e2e.py と test_static_build.py の両方で同一入力・期待値を使い、
結果が一致することを検証する。
"""

# 8枚のカードセット（5キャラ以上で sweep 可能）
SHARED_CARD_SPECS = [
    {"id": "tokino_sora_5", "potential": 0},
    {"id": "aki_rosenthal_5", "potential": 0},
    {"id": "natsuiro_matsuri_5", "potential": 0},
    {"id": "shirakami_fubuki_5", "potential": 0},
    {"id": "akai_haato_5", "potential": 0},
    {"id": "nakiri_ayame_5", "potential": 0},
    {"id": "usada_pekora_5", "potential": 0},
    {"id": "oozora_subaru_5", "potential": 0},
]

# Sweep solve (曲なし) の期待値
# unit_score の Top 1 をスナップショットとして固定
EXPECTED_SOLVE_TOP1_UNIT_SCORE = 798845

