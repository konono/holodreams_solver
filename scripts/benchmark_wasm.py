"""WASM vs ネイティブ ベンチマーク比較スクリプト

Playwright で WASM 版の solve を実行し、ネイティブ Go CLI と実行時間を比較する。

実行前提:
    mise run build:solver
    uv run python build_static.py
    uv run playwright install chromium

実行:
    uv run python scripts/benchmark_wasm.py

注意: 70枚 sweep 込みのフル実行は 10分以上かかることがあります。
"""

import http.server
import json
import subprocess
import sys
import threading
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DIST_DIR = ROOT / "dist"
SOLVER_BIN = ROOT / "solver_go" / "solver"
CARDS_JSON = ROOT / "data" / "cards.json"

SERVER_PORT = 18765
WARMUP_RUNS = 1
BENCH_RUNS = 3


def start_server():
    handler = lambda *a: http.server.SimpleHTTPRequestHandler(*a, directory=str(DIST_DIR))
    server = http.server.HTTPServer(("127.0.0.1", SERVER_PORT), handler)
    t = threading.Thread(target=server.serve_forever, daemon=True)
    t.start()
    return server


def build_card_specs(n):
    with open(CARDS_JSON) as f:
        data = json.load(f)
    specs = []
    for card in data["cards"][:n]:
        specs.append({"id": card["id"], "potential": 0})
    card_ids = [s["id"] for s in specs]
    return specs, card_ids


def bench_native(card_ids, runs):
    if not SOLVER_BIN.exists():
        print(f"  SKIP: {SOLVER_BIN} not found (run: mise run build:solver)")
        return None

    payload = json.dumps({
        "action": "solve",
        "cards": card_ids,
        "top_n": 5,
        "sweep_costumes": True,
    })

    # warmup
    for _ in range(WARMUP_RUNS):
        try:
            subprocess.run(
                [str(SOLVER_BIN)],
                input=payload, capture_output=True, text=True, cwd=str(ROOT),
            )
        except OSError as e:
            print(f"  SKIP: cannot execute {SOLVER_BIN} ({e})")
            return None

    times = []
    for _ in range(runs):
        start = time.perf_counter()
        try:
            result = subprocess.run(
                [str(SOLVER_BIN)],
                input=payload, capture_output=True, text=True, cwd=str(ROOT),
            )
        except OSError as e:
            print(f"  SKIP: cannot execute {SOLVER_BIN} ({e})")
            return None
        elapsed = time.perf_counter() - start
        if result.returncode != 0:
            print(f"  Native error: {result.stderr[:200]}")
            return None
        times.append(elapsed)
    return times


def bench_wasm(page, card_specs, runs):
    # warmup
    for _ in range(WARMUP_RUNS):
        page.evaluate(f"""() => {{
            return new Promise((resolve, reject) => {{
                _wasmWorker.addEventListener('message', function handler(ev) {{
                    if (ev.data.type === 'done' || ev.data.type === 'error') {{
                        _wasmWorker.removeEventListener('message', handler);
                        resolve(ev.data);
                    }}
                }});
                _wasmWorker.postMessage({{
                    type: 'solve',
                    cards: {json.dumps(card_specs)},
                    topN: 5,
                    sweepCostumes: true
                }});
            }});
        }}""")

    times = []
    for _ in range(runs):
        elapsed_ms = page.evaluate(f"""() => {{
            const start = performance.now();
            return new Promise((resolve, reject) => {{
                _wasmWorker.addEventListener('message', function handler(ev) {{
                    if (ev.data.type === 'done' || ev.data.type === 'error') {{
                        _wasmWorker.removeEventListener('message', handler);
                        if (ev.data.type === 'error') {{
                            reject(new Error(ev.data.message));
                        }} else {{
                            resolve(performance.now() - start);
                        }}
                    }}
                }});
                _wasmWorker.postMessage({{
                    type: 'solve',
                    cards: {json.dumps(card_specs)},
                    topN: 5,
                    sweepCostumes: true
                }});
            }});
        }}""")
        times.append(elapsed_ms / 1000.0)
    return times


def fmt_time(seconds):
    if seconds < 0.001:
        return f"{seconds * 1_000_000:.0f}µs"
    if seconds < 1:
        return f"{seconds * 1000:.1f}ms"
    return f"{seconds:.2f}s"


def main():
    from playwright.sync_api import sync_playwright

    for req in [DIST_DIR / "index.html", DIST_DIR / "solver.wasm"]:
        if not req.exists():
            print(f"Error: {req} not found. Run: mise run build:solver && uv run python build_static.py")
            sys.exit(1)

    scenarios = [
        ("solve (25 cards, sweep)", 25),
        ("solve (70 cards, sweep)", 70),
    ]

    server = start_server()

    with sync_playwright() as pw:
        browser = pw.chromium.launch()
        context = browser.new_context()
        page = context.new_page()
        page.goto(f"http://127.0.0.1:{SERVER_PORT}/index.html")
        page.wait_for_selector(".card", timeout=15000)

        for _ in range(30):
            if page.evaluate("typeof _wasmWorker !== 'undefined' && _wasmWorker !== null"):
                break
            page.wait_for_timeout(1000)

        if not page.evaluate("typeof _wasmWorker !== 'undefined' && _wasmWorker !== null"):
            print("Error: WASM worker not ready after 30s")
            sys.exit(1)

        print(f"{'Phase':<30} {'Native':>10} {'WASM':>10} {'Ratio':>8}")
        print("-" * 62)

        for label, n_cards in scenarios:
            specs, ids = build_card_specs(n_cards)

            native_times = bench_native(ids, BENCH_RUNS)
            wasm_times = bench_wasm(page, specs, BENCH_RUNS)

            native_med = sorted(native_times)[len(native_times) // 2] if native_times else None
            wasm_med = sorted(wasm_times)[len(wasm_times) // 2]

            native_str = fmt_time(native_med) if native_med else "N/A"
            wasm_str = fmt_time(wasm_med)
            if native_med and native_med > 0:
                ratio = f"{wasm_med / native_med:.1f}x"
            else:
                ratio = "N/A"

            print(f"{label:<30} {native_str:>10} {wasm_str:>10} {ratio:>8}")

        page.close()
        browser.close()

    server.shutdown()
    print()
    print(f"Runs: {BENCH_RUNS} (median shown), warmup: {WARMUP_RUNS}")


if __name__ == "__main__":
    main()
