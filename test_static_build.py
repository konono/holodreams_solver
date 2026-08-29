"""スタンドアロン版 (dist/holosolve.html) の Playwright テスト

build_static.py で生成した HTML が正しく動作するか確認する。
サーバー不要（file:// で開く）。

実行前提:
    uv run playwright install chromium

実行:
    uv run python build_static.py
    uv run pytest test_static_build.py -v
"""

import subprocess
from pathlib import Path

import pytest
from playwright.sync_api import sync_playwright

ROOT = Path(__file__).parent
STATIC_HTML = ROOT / "dist" / "holosolve.html"


@pytest.fixture(scope="module", autouse=True)
def build_static():
    result = subprocess.run(
        ["uv", "run", "python", "build_static.py"],
        cwd=ROOT, capture_output=True, text=True, timeout=60,
    )
    assert result.returncode == 0, f"build failed: {result.stderr}"
    assert STATIC_HTML.exists()


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
    page.goto(f"file://{STATIC_HTML.resolve()}")
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

    def test_leader_dropdown_shows_character_and_card_name(self, browser_context):
        page = open_page(browser_context)
        options = page.eval_on_selector_all(
            "#fixedLeader option:not(:first-child)",
            "els => els.map(e => e.textContent)"
        )
        assert len(options) > 0
        assert all(" / " in o for o in options[:5]), "Options should show 'character / card_name' format"
        page.close()

    def test_costume_only_checkbox_exists_and_disabled(self, browser_context):
        page = open_page(browser_context)
        disabled = page.eval_on_selector("#chkCostumeOnly", "el => el.disabled")
        assert disabled, "Costume-only checkbox should be disabled when no leader selected"
        page.close()

    def test_costume_only_checkbox_enables_on_leader_select(self, browser_context):
        page = open_page(browser_context)
        page.select_option("#fixedLeader", index=1)
        disabled = page.eval_on_selector("#chkCostumeOnly", "el => el.disabled")
        assert not disabled, "Costume-only checkbox should be enabled when leader is selected"
        page.close()

    def test_costume_only_checkbox_disables_on_auto(self, browser_context):
        page = open_page(browser_context)
        page.select_option("#fixedLeader", index=1)
        page.check("#chkCostumeOnly")
        page.select_option("#fixedLeader", value="")
        disabled = page.eval_on_selector("#chkCostumeOnly", "el => el.disabled")
        checked = page.eval_on_selector("#chkCostumeOnly", "el => el.checked")
        assert disabled, "Should be disabled after returning to auto"
        assert not checked, "Should be unchecked after returning to auto"
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

    def test_solve_with_fixed_leader(self, browser_context):
        """fixed_leader_id 指定カードが全結果でリーダーになる"""
        page = open_page(browser_context)
        ids = select_cards(page, 6)
        leader_id = ids[0]
        page.select_option("#fixedLeader", value=leader_id)
        page.click("#btnSolve")
        page.wait_for_selector(".result-card", timeout=30000)
        fixed_char = page.evaluate(f"() => CARDS.find(c => c.id === '{leader_id}')?.character")
        all_leader_names = page.eval_on_selector_all(
            ".member-card.is-leader .m-name",
            "els => els.map(e => e.textContent)"
        )
        assert len(all_leader_names) > 0, "Should have at least 1 result with a leader"
        assert all(n == fixed_char for n in all_leader_names), \
            f"All leaders should be {fixed_char}, got {all_leader_names}"
        page.close()

    def test_solve_costume_only_excludes_from_members(self, browser_context):
        """衣装のみモードで衣装カードがメンバーに含まれないことを確認"""
        page = open_page(browser_context)
        select_cards(page, 6)
        costume_option = page.eval_on_selector(
            "#fixedLeader option:nth-child(2)", "el => el.value"
        )
        page.select_option("#fixedLeader", value=costume_option)
        page.check("#chkCostumeOnly")
        page.click("#btnSolve")
        page.wait_for_selector(".result-card", timeout=30000)
        banner = page.query_selector("text=衣装リーダー")
        assert banner is not None, "Should show costume leader banner"
        costume_char = page.evaluate(f"() => CARDS.find(c => c.id === '{costume_option}')?.character")
        member_names = page.eval_on_selector_all(
            ".result-card:first-child .member-card .m-name",
            "els => els.map(e => e.textContent)"
        )
        assert costume_char not in member_names, \
            f"Costume-only leader {costume_char} should not be in members: {member_names}"
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
