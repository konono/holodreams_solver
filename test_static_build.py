"""WASM版 (dist/index.html) の Playwright テスト

build_static.py で生成した HTML が WASM ソルバーで正しく動作するか確認する。
WASM の読み込みに HTTP サーバーが必要。

注意: test_e2e.py と同一 pytest セッションでは実行不可
（pytest-playwright の async ループと sync_playwright() が競合するため）。
CI では pytest を分離実行している。

実行前提:
    uv run playwright install chromium
    cd solver_go && GOOS=js GOARCH=wasm go build -o solver.wasm .

実行:
    uv run python build_static.py
    uv run pytest test_static_build.py -v
"""

import http.server
import json
import subprocess
import threading
from pathlib import Path

import pytest
from playwright.sync_api import sync_playwright

ROOT = Path(__file__).parent
DIST_DIR = ROOT / "dist"
STATIC_HTML = DIST_DIR / "index.html"
WASM_FILE = DIST_DIR / "solver.wasm"
SERVER_PORT = 18765


@pytest.fixture(scope="module", autouse=True)
def build_and_serve():
    result = subprocess.run(
        ["uv", "run", "python", "build_static.py"],
        cwd=ROOT, capture_output=True, text=True, timeout=60,
    )
    assert result.returncode == 0, f"build failed: {result.stderr}"
    assert STATIC_HTML.exists()
    assert WASM_FILE.exists(), "solver.wasm not found in dist/ — run: cd solver_go && GOOS=js GOARCH=wasm go build -o ../dist/solver.wasm ."

    class QuietHandler(http.server.SimpleHTTPRequestHandler):
        def __init__(self, *args, **kwargs):
            super().__init__(*args, directory=str(DIST_DIR), **kwargs)

        def log_message(self, format, *args):
            pass

    server = http.server.HTTPServer(("127.0.0.1", SERVER_PORT), QuietHandler)
    t = threading.Thread(target=server.serve_forever, daemon=True)
    t.start()
    yield
    server.shutdown()


@pytest.fixture(scope="module")
def browser_context():
    with sync_playwright() as p:
        browser = p.chromium.launch(
            headless=True,
            args=["--no-sandbox", "--disable-dev-shm-usage"],
        )
        context = browser.new_context()
        yield context
        context.close()
        browser.close()


def open_page(browser_context):
    page = browser_context.new_page()
    page.goto(f"http://127.0.0.1:{SERVER_PORT}/index.html")
    page.wait_for_selector(".card", timeout=10000)
    page.evaluate("localStorage.clear()")
    page.reload()
    page.wait_for_selector(".card", timeout=10000)
    return page


def select_cards(page, count=6):
    # Reset song selection to avoid Timeline reranking in tests
    page.select_option("#songSelect", value="")
    ids = page.eval_on_selector_all(".card", f"els => els.slice(0, {count}).map(e => e.dataset.id)")
    for cid in ids:
        page.click(f'.card[data-id="{cid}"] .char-name')
    return ids


class TestStaticBasicUI:
    def test_cards_rendered(self, browser_context):
        page = open_page(browser_context)
        count = page.eval_on_selector_all(".card", "els => els.length")
        assert count >= 60, f"Expected >=60 cards, got {count}"
        page.close()

    def test_costume_dropdown_shows_character_and_card_name(self, browser_context):
        page = open_page(browser_context)
        options = page.eval_on_selector_all(
            "#costumeSelect option:not(:first-child)",
            "els => els.map(e => e.textContent)"
        )
        assert len(options) > 0
        assert all(" / " in o for o in options[:5]), "Options should show 'character / card_name' format"
        page.close()

    def test_costume_dropdown_default_is_auto(self, browser_context):
        page = open_page(browser_context)
        val = page.eval_on_selector("#costumeSelect", "el => el.value")
        assert val == "", "Default should be empty (auto)"
        page.close()


class TestStaticWasm:
    def test_solve_loads_wasm(self, browser_context):
        """ページロード時にsolver.wasmがロードされることを確認（WASM版の証明）"""
        wasm_loaded = []
        page = browser_context.new_page()
        page.on("response", lambda res: wasm_loaded.append(res.url) if "solver.wasm" in res.url else None)
        page.goto(f"http://127.0.0.1:{SERVER_PORT}/index.html")
        page.wait_for_selector(".card", timeout=10000)
        page.wait_for_timeout(2000)
        assert any("solver.wasm" in url for url in wasm_loaded), "solver.wasm should be loaded (WASM version)"
        page.close()

    def test_html_does_not_contain_js_solver(self, browser_context):
        """生成HTMLにJSソルバーが含まれていないことを確認"""
        html = (DIST_DIR / "index.html").read_text()
        assert "function evaluateTeam" not in html, "JS solver should not be embedded"
        assert "function solveInternal" not in html, "JS solver should not be embedded"
        assert "wasm_bridge.js" in html, "Should reference WASM bridge"


class TestStaticSolve:
    def test_solve_produces_results(self, browser_context):
        page = open_page(browser_context)
        select_cards(page, 6)
        page.click("#btnSolve")
        page.wait_for_selector(".result-card", timeout=30000)
        results = page.eval_on_selector_all(".result-card", "els => els.length")
        assert results > 0, "Should produce at least 1 result"
        page.close()

    def test_solve_with_costume_member_include_shows_leader_badge(self, browser_context):
        """衣装選択+メンバーに含める → リーダーバッジあり、衣装バナーなし"""
        page = open_page(browser_context)
        select_cards(page, 6)
        costume_option = page.eval_on_selector(
            "#costumeSelect option:nth-child(2)", "el => el.value"
        )
        page.select_option("#costumeSelect", value=costume_option)
        page.click("#btnSolve")
        page.wait_for_selector(".result-card", timeout=30000)
        leaders = page.eval_on_selector_all(".member-card.is-leader", "els => els.length")
        assert leaders > 0, "Should have leader badge when member-include is on"
        page.close()

    def test_solve_with_costume_only_shows_banner(self, browser_context):
        """衣装選択+メンバー含めない → 衣装バナーあり、リーダーバッジなし"""
        page = open_page(browser_context)
        select_cards(page, 6)
        costume_option = page.eval_on_selector(
            "#costumeSelect option:nth-child(2)", "el => el.value"
        )
        page.select_option("#costumeSelect", value=costume_option)
        page.uncheck("#chkMemberInclude")
        page.click("#btnSolve")
        page.wait_for_selector(".result-card", timeout=30000)
        banner = page.query_selector("text=👗")
        assert banner is not None, "Should show costume label in result card"
        leaders = page.eval_on_selector_all(".member-card.is-leader", "els => els.length")
        assert leaders == 0, "Should not have leader badge when costume-only"
        page.close()


def select_cards_and_song(page, count=8):
    """カードを選択し、最初の曲を選択する共通ヘルパー"""
    page.select_option("#songSelect", value="")
    ids = page.eval_on_selector_all(".card", f"els => els.slice(0, {count}).map(e => e.dataset.id)")
    for cid in ids:
        page.click(f'.card[data-id="{cid}"] .char-name')
    page.evaluate("""(() => {
        const sel = document.getElementById('songSelect');
        const opts = Array.from(sel.options).filter(o => o.value);
        if (opts.length) { sel.value = opts[0].value; sel.dispatchEvent(new Event('change')); }
    })()""")
    return ids


class TestStaticTimelineSolve:
    def test_solve_with_timeline_shows_results(self, browser_context):
        """曲選択 + Timeline + sweep → .result-card が表示される"""
        page = open_page(browser_context)
        select_cards_and_song(page, 8)
        page.click("#btnSolve")
        page.wait_for_selector(".result-card", timeout=60000)
        results = page.eval_on_selector_all(".result-card", "els => els.length")
        assert results > 0, "Timeline solve should produce results"
        progress_text = page.eval_on_selector("#progressText", "el => el.textContent")
        assert "完了" in progress_text, f"Progress should show completion, got: {progress_text}"
        page.close()

    def test_timeline_results_show_lsi_and_board(self, browser_context):
        """Timeline結果にスキル効率・Active重複ロス・メンバーカードが表示されている"""
        page = open_page(browser_context)
        select_cards_and_song(page, 8)
        page.click("#btnSolve")
        page.wait_for_selector(".result-card", timeout=60000)
        area_text = page.eval_on_selector("#resultsArea", "el => el.textContent")
        assert "ライブ期待スコア" in area_text, \
            f"Song selected → timeline results expected, got: {area_text[:100]}"
        assert "スキル効率" in area_text, "Timeline results should display skill efficiency"
        assert "Active重複ロス" in area_text, "Timeline results should display overlap loss"
        member_cards = page.eval_on_selector_all(".member-card", "els => els.length")
        assert member_cards >= 5, f"Expected >=5 member cards, got {member_cards}"
        page.close()

    def test_timeline_no_js_errors(self, browser_context):
        """Timeline solve中にJSエラーが発生しない"""
        page = browser_context.new_page()
        errors = []
        page.on("pageerror", lambda err: errors.append(err.message))
        page.goto(f"http://127.0.0.1:{SERVER_PORT}/index.html")
        page.wait_for_selector(".card", timeout=10000)
        page.evaluate("localStorage.clear()")
        page.reload()
        page.wait_for_selector(".card", timeout=10000)
        select_cards_and_song(page, 8)
        page.click("#btnSolve")
        page.wait_for_selector(".result-card", timeout=60000)
        assert len(errors) == 0, f"JS errors during timeline solve: {errors}"
        page.close()


class TestStaticErrorHandling:
    def test_solve_button_disabled_with_insufficient_cards(self, browser_context):
        """3枚選択（5枚未満）→ solveボタンが無効化されている"""
        page = open_page(browser_context)
        ids = page.eval_on_selector_all(".card", "els => els.slice(0, 3).map(e => e.dataset.id)")
        for cid in ids:
            page.click(f'.card[data-id="{cid}"] .char-name')
        solve_disabled = page.evaluate("document.getElementById('btnSolve').disabled")
        assert solve_disabled, "Solve button should be disabled with <5 cards"
        page.close()

    def test_recommend_button_disabled_with_insufficient_cards(self, browser_context):
        """3枚選択 → レコメンドボタンが無効化されている"""
        page = open_page(browser_context)
        ids = page.eval_on_selector_all(".card", "els => els.slice(0, 3).map(e => e.dataset.id)")
        for cid in ids:
            page.click(f'.card[data-id="{cid}"] .char-name')
        rec_disabled = page.evaluate("document.getElementById('btnRecommend').disabled")
        assert rec_disabled, "Recommend button should be disabled with <5 cards"
        page.close()

    def test_done_handler_has_try_catch(self):
        """build_static.pyのdoneハンドラにtry-catchが存在する（ソース検証のみ、動作検証はしない）"""
        html = (DIST_DIR / "index.html").read_text()
        # solve done handler
        assert "catch (renderErr)" in html or "catch(renderErr)" in html, \
            "Solve done handler should have try-catch for render errors"
        # The error message pattern
        assert "結果の表示中にエラーが発生しました" in html, \
            "Error message should be present in the HTML"


class TestStaticWasmResultParity:
    """WASM版の計算結果が期待値（Go CLIスナップショット）と一致することを検証する"""

    def test_solve_result_matches_expected(self, browser_context):
        """共通カードセットでsolve → unit_score Top1がスナップショット値と一致"""
        from test_shared_fixtures import SHARED_CARD_SPECS, EXPECTED_SOLVE_TOP1_UNIT_SCORE

        page = browser_context.new_page()
        page.goto(f"http://127.0.0.1:{SERVER_PORT}/index.html")
        page.wait_for_selector(".card", timeout=10000)
        # Wait for WASM ready
        for _ in range(30):
            if page.evaluate("typeof _wasmWorker !== 'undefined' && _wasmWorker !== null"):
                break
            page.wait_for_timeout(1000)
        assert page.evaluate("typeof _wasmWorker !== 'undefined' && _wasmWorker !== null"), \
            "WASM worker not ready after 30s"

        page.evaluate(f"""(() => {{
            window._testDone = false;
            window._testResult = null;
            _wasmWorker.addEventListener('message', function handler(ev) {{
                if (ev.data.type === 'done' || ev.data.type === 'error') {{
                    window._testResult = ev.data;
                    window._testDone = true;
                    _wasmWorker.removeEventListener('message', handler);
                }}
            }});
            _wasmWorker.postMessage({{
                type: 'solve',
                cards: {json.dumps(SHARED_CARD_SPECS)},
                topN: 3,
                sweepCostumes: true
            }});
        }})()""")

        page.wait_for_function("window._testDone === true", timeout=60000)
        result = page.evaluate("window._testResult")

        assert result.get("type") != "error", f"WASM solve error: {result.get('message')}"

        results = result.get("results", [])
        assert len(results) >= 1, "Should produce at least 1 result"

        top1_unit_score = results[0].get("unit_score", 0)
        assert top1_unit_score == EXPECTED_SOLVE_TOP1_UNIT_SCORE, \
            f"WASM unit_score {top1_unit_score} != expected {EXPECTED_SOLVE_TOP1_UNIT_SCORE}"
        page.close()


class TestStaticRecommendWasm:
    def test_recommend_best_team_shows_card_name(self, browser_context):
        """8枚選択+レコメンド → ベストチームにカード名(括弧内)が表示される"""
        page = open_page(browser_context)
        ids = page.eval_on_selector_all(".card", "els => els.slice(0, 8).map(e => e.dataset.id)")
        for cid in ids:
            page.click(f'.card[data-id="{cid}"] .char-name')
        selected_count = page.evaluate("() => document.querySelectorAll('.card.selected').length")
        assert selected_count >= 5, f"Expected >=5 selected, got {selected_count}"
        page.click("#btnRecommend")
        page.wait_for_selector(".result-card", timeout=120000)
        best_team_text = page.eval_on_selector_all(
            ".result-card",
            "els => els.map(e => e.textContent)"
        )
        has_card_name = any(("メンバー" in t or "ベストチーム" in t) and "(" in t for t in best_team_text)
        assert has_card_name, "Best team display should include card names in parentheses"
        page.close()

    def test_recommend_shows_progress(self, browser_context):
        """レコメンド実行中にプログレス表示が出る"""
        page = open_page(browser_context)
        ids = page.eval_on_selector_all(".card", "els => els.slice(0, 8).map(e => e.dataset.id)")
        for cid in ids:
            page.click(f'.card[data-id="{cid}"] .char-name')
        # Watch for progress area becoming visible
        page.click("#btnRecommend")
        page.wait_for_function(
            "document.getElementById('progressArea')?.classList.contains('visible') || "
            "document.querySelectorAll('.result-card').length > 0",
            timeout=10000,
        )
        page.wait_for_selector(".result-card", timeout=120000)
        progress_text = page.eval_on_selector("#progressText", "el => el.textContent")
        assert "完了" in progress_text, f"Should show completion, got: {progress_text}"
        page.close()

    def test_recommend_no_js_errors(self, browser_context):
        """レコメンド実行中にJSエラーが発生しない"""
        page = browser_context.new_page()
        errors = []
        page.on("pageerror", lambda err: errors.append(err.message))
        page.goto(f"http://127.0.0.1:{SERVER_PORT}/index.html")
        page.wait_for_selector(".card", timeout=10000)
        page.evaluate("localStorage.clear()")
        page.reload()
        page.wait_for_selector(".card", timeout=10000)
        ids = page.eval_on_selector_all(".card", "els => els.slice(0, 8).map(e => e.dataset.id)")
        for cid in ids:
            page.click(f'.card[data-id="{cid}"] .char-name')
        page.click("#btnRecommend")
        page.wait_for_selector(".result-card", timeout=120000)
        assert len(errors) == 0, f"JS errors during recommend: {errors}"
        page.close()
