"""E2Eテスト — クリップボード・永続化・履歴のブラウザ動作検証

実行前提:
    uv run playwright install chromium

実行:
    uv run pytest test_e2e.py -v
"""

import json
import subprocess
import time
from pathlib import Path

import pytest
from playwright.sync_api import BrowserContext, Page

ROOT = Path(__file__).parent
BASE_URL = "http://127.0.0.1:8000"


@pytest.fixture(scope="module")
def server():
    proc = subprocess.Popen(
        ["uv", "run", "python", "app.py"],
        cwd=ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    for _ in range(30):
        try:
            import urllib.request
            urllib.request.urlopen(BASE_URL, timeout=1)
            break
        except Exception:
            time.sleep(0.5)
    else:
        proc.terminate()
        raise RuntimeError("Server did not start")
    yield BASE_URL
    proc.terminate()
    proc.wait(timeout=5)


@pytest.fixture
def ctx(browser) -> BrowserContext:
    context = browser.new_context(permissions=["clipboard-read", "clipboard-write"])
    yield context
    context.close()


@pytest.fixture
def fresh_page(ctx, server) -> Page:
    page = ctx.new_page()
    page.goto(server)
    page.wait_for_selector(".card", timeout=10000)
    page.evaluate("localStorage.clear()")
    page.reload()
    page.wait_for_selector(".card", timeout=10000)
    return page


def get_first_card_id(page: Page) -> str:
    return page.eval_on_selector(".card", "el => el.dataset.id")


def get_card_ids(page: Page, count: int = 3) -> list[str]:
    return page.eval_on_selector_all(".card", f"els => els.slice(0, {count}).map(e => e.dataset.id)")


def click_card(page: Page, card_id: str):
    page.click(f'.card[data-id="{card_id}"] .char-name')


def set_pot(page: Page, card_id: str, pot: int):
    page.click(f'.card[data-id="{card_id}"] .pot-btn[data-pot="{pot}"]')


def get_selected_ids(page: Page) -> list[str]:
    return page.eval_on_selector_all(".card.selected", "els => els.map(e => e.dataset.id)")


def get_clipboard_json(page: Page) -> dict | list:
    text = page.evaluate("() => navigator.clipboard.readText()")
    return json.loads(text)


class TestClipboardCopy:
    def test_all_cards_mode_copy_contains_all_ids(self, fresh_page):
        """全カード探索モードでIDコピー → 全カードIDが含まれる"""
        page = fresh_page
        total = page.eval_on_selector_all(".card", "els => els.length")
        page.click("#btnCopyIds")
        page.wait_for_timeout(500)
        data = get_clipboard_json(page)
        assert isinstance(data, dict)
        assert data["v"] == 1
        assert data["allCards"] is True
        assert len(data["ids"]) == total

    def test_all_cards_mode_copy_includes_pot_overrides(self, fresh_page):
        """全カード探索モードで凸変更後コピー → overrideが含まれる"""
        page = fresh_page
        card_id = get_first_card_id(page)
        set_pot(page, card_id, 4)
        page.click("#btnCopyIds")
        page.wait_for_timeout(500)
        data = get_clipboard_json(page)
        assert data["potentials"].get(card_id) == 4

    def test_partial_selection_copy(self, fresh_page):
        """部分選択でコピー → 選択カードのみ、allCards=false"""
        page = fresh_page
        ids = get_card_ids(page, 5)
        for cid in ids:
            click_card(page, cid)
        page.click("#btnCopyIds")
        page.wait_for_timeout(500)
        data = get_clipboard_json(page)
        assert data["allCards"] is False
        assert set(data["ids"]) == set(ids)


class TestClipboardPaste:
    def test_v1_paste_restores_selection_and_pot(self, fresh_page):
        """v1形式ペースト → 選択と凸が復元される"""
        page = fresh_page
        ids = get_card_ids(page, 5)
        click_card(page, ids[0])
        set_pot(page, ids[0], 3)
        page.click("#btnCopyIds")
        page.wait_for_timeout(500)
        clipboard_data = get_clipboard_json(page)

        page.click("#btnClear")
        assert len(get_selected_ids(page)) == 0

        page.click("#btnPasteIds")
        page.wait_for_timeout(500)
        assert ids[0] in get_selected_ids(page)
        active_pot = page.eval_on_selector(
            f'.card[data-id="{ids[0]}"] .pot-btn.active', "el => parseInt(el.dataset.pot)"
        )
        assert active_pot == 3

    def test_legacy_array_paste(self, fresh_page):
        """旧形式(配列)ペースト → カード選択が復元される"""
        page = fresh_page
        ids = get_card_ids(page, 5)
        page.evaluate(f"navigator.clipboard.writeText(JSON.stringify({json.dumps(ids)}))")
        page.click("#btnPasteIds")
        page.wait_for_timeout(500)
        selected = get_selected_ids(page)
        assert set(selected) == set(ids)

    def test_allcards_paste_stays_unselected(self, fresh_page):
        """allCards=trueのv1ペースト → 選択は空のまま(全カード探索モード)"""
        page = fresh_page
        card_id = get_first_card_id(page)
        payload = {"v": 1, "ids": [card_id], "allCards": True, "potentials": {card_id: 4}, "levels": {},
                   "defaultPotential": 0, "defaultLevel": 80, "levelEnabled": False}
        page.evaluate(f"navigator.clipboard.writeText({json.dumps(json.dumps(payload))})")
        page.click("#btnPasteIds")
        page.wait_for_timeout(500)
        assert len(get_selected_ids(page)) == 0
        active_pot = page.eval_on_selector(
            f'.card[data-id="{card_id}"] .pot-btn.active', "el => parseInt(el.dataset.pot)"
        )
        assert active_pot == 4


class TestPersistenceReload:
    def test_selection_persists_across_reload(self, fresh_page):
        """カード選択 → リロード → 選択が維持される"""
        page = fresh_page
        ids = get_card_ids(page, 5)
        for cid in ids:
            click_card(page, cid)
        assert len(get_selected_ids(page)) == 5

        page.reload()
        page.wait_for_selector(".card", timeout=10000)
        selected_after = get_selected_ids(page)
        assert set(selected_after) == set(ids)

    def test_pot_persists_across_reload(self, fresh_page):
        """凸設定 → リロード → UIに反映されている"""
        page = fresh_page
        card_id = get_first_card_id(page)
        click_card(page, card_id)
        set_pot(page, card_id, 4)

        page.reload()
        page.wait_for_selector(".card", timeout=10000)
        active_pot = page.eval_on_selector(
            f'.card[data-id="{card_id}"] .pot-btn.active', "el => parseInt(el.dataset.pot)"
        )
        assert active_pot == 4

    def test_all_cards_mode_overrides_persist(self, fresh_page):
        """全カード探索で凸変更 → リロード → override維持、選択なし"""
        page = fresh_page
        card_id = get_first_card_id(page)
        set_pot(page, card_id, 3)

        page.reload()
        page.wait_for_selector(".card", timeout=10000)
        assert len(get_selected_ids(page)) == 0
        active_pot = page.eval_on_selector(
            f'.card[data-id="{card_id}"] .pot-btn.active', "el => parseInt(el.dataset.pot)"
        )
        assert active_pot == 3


class TestSolveHistory:
    def test_results_and_selection_can_be_compared_and_restored(self, fresh_page):
        page = fresh_page
        ids = get_card_ids(page, 7)
        for cid in ids[:6]:
            click_card(page, cid)
        set_pot(page, ids[0], 3)
        page.check("#chkLevelEnabled")
        page.locator(f'.card[data-id="{ids[0]}"] .lv-input').fill("70")
        page.locator(f'.card[data-id="{ids[0]}"] .lv-input').press("Tab")
        page.evaluate("selectSong('')")
        page.select_option("#topN", "30")
        page.select_option("#boardSearchMode", "fast")
        page.click("#btnSolve")
        page.wait_for_function("document.querySelector('#historyCount').textContent === '1'", timeout=30000)
        before = page.eval_on_selector_all("#resultsArea .result-card", "els => els.map(e => e.innerText)")
        assert before

        click_card(page, ids[6])
        page.select_option("#topN", "10")
        page.select_option("#boardSearchMode", "balanced")
        page.click("#btnSolve")
        page.wait_for_function("document.querySelector('#historyCount').textContent === '2'", timeout=30000)
        after = page.eval_on_selector_all("#resultsArea .result-card", "els => els.map(e => e.innerText)")
        assert after

        page.reload()
        page.wait_for_selector(".card", timeout=10000)
        page.click("#historyToggle")
        page.locator(".history-entry").nth(1).locator("button[data-action=restore]").click()
        assert set(get_selected_ids(page)) == set(ids[:6])
        assert page.input_value("#topN") == "30"
        assert page.input_value("#boardSearchMode") == "fast"
        assert page.eval_on_selector(f'.card[data-id="{ids[0]}"] .pot-btn.active', "el => Number(el.dataset.pot)") == 3
        assert page.is_checked("#chkLevelEnabled")
        assert page.input_value(f'.card[data-id="{ids[0]}"] .lv-input') == "70"
        assert page.eval_on_selector_all("#resultsArea .result-card", "els => els.map(e => e.innerText)") == before

        page.reload()
        page.wait_for_selector(".card", timeout=10000)
        assert set(get_selected_ids(page)) == set(ids[:6])
        page.click("#historyToggle")

        page.locator(".history-entry").first.locator("button[data-action=restore]").click()
        assert set(get_selected_ids(page)) == set(ids)
        assert page.input_value("#boardSearchMode") == "balanced"
        assert page.eval_on_selector_all("#resultsArea .result-card", "els => els.map(e => e.innerText)") == after

    def test_old_server_history_keeps_saved_results(self, fresh_page):
        page = fresh_page
        for cid in get_card_ids(page, 6):
            click_card(page, cid)
        page.evaluate("selectSong('')")
        page.click("#btnSolve")
        page.wait_for_function("document.querySelector('#historyCount').textContent === '1'", timeout=30000)
        before = page.eval_on_selector_all("#resultsArea .result-card", "els => els.map(e => e.innerText)")

        page.evaluate("""async () => {
          const db = await getHistoryDB();
          const [entry] = await historyTransaction(db, 'readonly', store => store.getAll());
          const resultKey = `holodri_result_${entry.ts}`;
          localStorage.setItem(resultKey, JSON.stringify(entry.result));
          localStorage.setItem('holodri_solve_history', JSON.stringify([{
            ts: entry.ts, label: '以前の履歴', settings: entry.settings,
            snapshot: entry.snapshot, results: entry.results,
            isTimeline: entry.isTimeline, resultKey,
          }]));
          await historyTransaction(db, 'readwrite', store => store.clear());
        }""")
        page.reload()
        page.wait_for_selector(".card", timeout=10000)
        page.wait_for_function("document.querySelector('#historyCount').textContent === '1'", timeout=10000)
        page.click("#historyToggle")
        assert page.locator(".history-entry .h-label-input").first.input_value() == "以前の履歴"
        page.locator(".history-entry button[data-action=restore]").click()
        assert page.eval_on_selector_all("#resultsArea .result-card", "els => els.map(e => e.innerText)") == before
        assert page.evaluate("localStorage.getItem('holodri_solve_history')") is None

    def test_timeline_result_is_restored(self, fresh_page):
        page = fresh_page
        for cid in get_card_ids(page, 6):
            click_card(page, cid)
        page.evaluate("selectSong('m0001')")
        page.select_option("#boardSearchMode", "fast")
        page.click("#btnSolve")
        page.wait_for_function("document.querySelector('#historyCount').textContent === '1'", timeout=60000)
        before = page.eval_on_selector("#resultsArea", "el => el.innerText")
        assert "ライブ期待スコア" in before
        page.evaluate("selectSong('')")
        page.click("#historyToggle")
        page.locator(".history-entry button[data-action=restore]").click()
        assert page.input_value("#songSelect") == "m0001"
        assert page.eval_on_selector("#resultsArea", "el => el.innerText") == before


class TestServerResultParity:
    """サーバー版の計算結果が期待値（Go CLIスナップショット）と一致することを検証する"""

    def test_solve_result_matches_expected(self, server):
        """共通カードセットでsolve API → unit_score Top1がスナップショット値と一致"""
        import urllib.request
        from test_shared_fixtures import SHARED_CARD_SPECS, EXPECTED_SOLVE_TOP1_UNIT_SCORE

        payload = json.dumps({
            "cards": SHARED_CARD_SPECS,
            "top_n": 3,
            "sweep_costumes": True,
        }).encode()
        req = urllib.request.Request(
            f"{server}/api/solve",
            data=payload,
            headers={"Content-Type": "application/json"},
        )
        with urllib.request.urlopen(req, timeout=30) as resp:
            result = json.loads(resp.read().decode())

        results = result.get("results", [])
        assert len(results) >= 1, "Should produce at least 1 result"

        top1_unit_score = results[0].get("unit_score", 0)
        assert top1_unit_score == EXPECTED_SOLVE_TOP1_UNIT_SCORE, \
            f"Server unit_score {top1_unit_score} != expected {EXPECTED_SOLVE_TOP1_UNIT_SCORE}"


class TestStaticBuild:
    def test_build_produces_html(self):
        """build_static.py → dist/index.html が生成される"""
        result = subprocess.run(
            ["uv", "run", "python", "build_static.py"],
            cwd=ROOT, capture_output=True, text=True, timeout=60,
        )
        assert result.returncode == 0
        assert (ROOT / "dist" / "index.html").exists()

    def test_build_contains_key_functions(self):
        """生成HTMLと共通履歴スクリプトに主要関数が含まれる"""
        html = (ROOT / "dist" / "index.html").read_text()
        for fn in ["savePersistence", "holodri_all_cards_mode", 'src="history.js"']:
            assert fn in html, f"{fn} not found in static build"
        history_js = (ROOT / "dist" / "history.js").read_text()
        for fn in ["renderHistory", "restoreFromHistory", "saveToHistory"]:
            assert fn in history_js, f"{fn} not found in static history script"
