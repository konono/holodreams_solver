"""WASM版 (dist/index.html) の Playwright テスト

build_static.py で生成した HTML が WASM ソルバーで正しく動作するか確認する。
WASM の読み込みに HTTP サーバーが必要。

実行前提:
    uv run playwright install chromium
    cd solver_go && GOOS=js GOARCH=wasm go build -o ../dist/solver.wasm .

実行:
    uv run python build_static.py
    uv run pytest test_static_build.py -v
"""

import http.server
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


class TestStaticRecommendDisplay:
    def test_recommend_best_team_shows_card_name(self, browser_context):
        """8枚選択+レコメンド → ベストチームにカード名(括弧内)が表示される"""
        page = open_page(browser_context)
        ids = page.eval_on_selector_all(".card", "els => els.slice(0, 8).map(e => e.dataset.id)")
        for cid in ids:
            page.click(f'.card[data-id="{cid}"] .char-name')
        selected_count = page.evaluate("() => document.querySelectorAll('.card.selected').length")
        assert selected_count >= 5, f"Expected >=5 selected, got {selected_count}"
        recommend_disabled = page.evaluate("() => document.getElementById('btnRecommend').disabled")
        assert not recommend_disabled, "Recommend button should be enabled with 5+ cards selected"
        page.click("#btnRecommend")
        page.wait_for_selector(".result-card", timeout=120000)
        best_team_text = page.eval_on_selector_all(
            ".result-card",
            "els => els.map(e => e.textContent)"
        )
        has_card_name = any("ベストチーム" in t and "(" in t for t in best_team_text)
        assert has_card_name, "Best team display should include card names in parentheses"
        page.close()
