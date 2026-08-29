"""強化レコメンド機能のUIテスト（Playwright）"""

import subprocess
import time

import pytest
from playwright.sync_api import expect, sync_playwright


@pytest.fixture(scope="module")
def server():
    proc = subprocess.Popen(
        ["uv", "run", "python", "app.py"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    time.sleep(3)
    yield "http://localhost:8000"
    proc.terminate()
    proc.wait()


@pytest.fixture(scope="module")
def browser_context():
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True, args=["--no-sandbox", "--disable-dev-shm-usage"], channel="chromium")
        context = browser.new_context()
        yield context
        context.close()
        browser.close()


def fresh_page(browser_context, url):
    page = browser_context.new_page()
    page.add_init_script("() => localStorage.clear()")
    page.goto(url)
    page.wait_for_selector(".card", timeout=10000)
    return page


def select_cards_by_data_id(page, card_ids):
    for cid in card_ids:
        el = page.locator(f'div.card[data-id="{cid}"]')
        el.wait_for(timeout=5000)
        is_selected = el.evaluate("el => el.classList.contains('selected')")
        if not is_selected:
            el.locator(".char-name").click()
            page.wait_for_timeout(100)


TEST_CARDS_8 = [
    "tokino_sora_5", "robocosan_5", "hoshimachi_suisei_5",
    "sakura_miko_5", "shirakami_fubuki_5", "natsuiro_matsuri_5",
    "akai_haato_5", "nakiri_ayame_5",
]

TEST_CARDS_4_CHARS = [
    "tokino_sora_5", "robocosan_5",
    "sakura_miko_5", "sakura_miko_swim_5",
    "hoshimachi_suisei_5", "hoshimachi_suisei_swim_5",
]


def test_recommend_button_enabled_with_5_cards(server, browser_context):
    """カードを5枚以上選択して「強化レコメンド」ボタンが有効化されることを確認"""
    page = fresh_page(browser_context, server)

    btn = page.locator("#btnRecommend")
    expect(btn).to_be_disabled()

    select_cards_by_data_id(page, TEST_CARDS_8[:4])
    expect(btn).to_be_disabled()

    select_cards_by_data_id(page, [TEST_CARDS_8[4]])
    expect(btn).to_be_enabled()

    page.close()


def test_recommend_top5_single_card(server, browser_context):
    """+1枚でTop5レコメンドが表示されることを確認"""
    page = fresh_page(browser_context, server)

    select_cards_by_data_id(page, TEST_CARDS_8)
    page.select_option("#acquireCount", "1")
    page.select_option("#recommendTopN", "5")
    page.click("#btnRecommend")

    page.wait_for_selector(".results-area .result-card", timeout=60000)
    result_cards = page.locator(".results-area .result-card")
    count = result_cards.count()
    assert count == 5, f"Expected 5 results, got {count}"

    first_result = result_cards.first
    expect(first_result).to_contain_text("#1")
    expect(first_result).to_contain_text("+")

    page.close()


def test_recommend_multi_card_combo(server, browser_context):
    """+2〜3枚で組み合わせレコメンドが表示されることを確認"""
    page = fresh_page(browser_context, server)

    select_cards_by_data_id(page, TEST_CARDS_8)

    page.select_option("#acquireCount", "2")
    page.select_option("#recommendTopN", "5")
    page.click("#btnRecommend")
    page.wait_for_selector(".results-area .result-card", timeout=60000)

    results = page.locator(".results-area .result-card")
    assert results.count() >= 1

    first = results.first
    action_badges = first.locator('span:has-text("新規取得"), span:has-text("凸→")')
    assert action_badges.count() >= 2, "Expected at least 2 card actions in combo result"

    expect(page.locator(".results-title")).to_contain_text("+2枚")

    page.select_option("#acquireCount", "3")
    page.click("#btnRecommend")
    page.wait_for_selector(".results-area .result-card", timeout=120000)

    results3 = page.locator(".results-area .result-card")
    assert results3.count() >= 1
    expect(page.locator(".results-title")).to_contain_text("+3枚")

    page.close()


def set_card_potential(page, card_id, potential):
    btn = page.locator(f'button.pot-btn[data-pot="{potential}"][data-card="{card_id}"]')
    btn.click()
    page.wait_for_timeout(100)


TEST_CARDS_MANY = [
    "tokino_sora_5", "robocosan_5", "hoshimachi_suisei_5",
    "sakura_miko_5", "shirakami_fubuki_5", "natsuiro_matsuri_5",
    "akai_haato_5", "nakiri_ayame_5", "azki_5",
    "aki_rosenthal_5", "yuzuki_choco_5", "oozora_subaru_5",
    "ookami_mio_5", "nekomata_okayu_5", "inugami_korone_5",
]


def test_recommend_multi_uncap_acquire2(server, browser_context):
    """acquire_count=2で同一カードの複数凸（例: 0凸→2凸）がレコメンドに含まれることを確認"""
    page = fresh_page(browser_context, server)

    select_cards_by_data_id(page, TEST_CARDS_MANY)
    page.click("#btnPot0")
    page.wait_for_timeout(200)

    page.select_option("#acquireCount", "2")
    page.select_option("#recommendTopN", "10")
    page.click("#btnRecommend")
    page.wait_for_selector(".results-area .result-card", timeout=120000)

    results = page.locator(".results-area .result-card")
    assert results.count() >= 1

    all_text = page.locator(".results-area").inner_text()
    has_multi_uncap = "0凸→2凸" in all_text
    assert has_multi_uncap, (
        "Expected at least one '0凸→2凸' multi-uncap recommendation in results. "
        f"Actual text: {all_text[:500]}"
    )

    page.close()


def test_recommend_acquire3_produces_results(server, browser_context):
    """acquire_count=3でレコメンドが正しく生成されることを確認"""
    page = fresh_page(browser_context, server)

    select_cards_by_data_id(page, TEST_CARDS_MANY)
    page.click("#btnPot0")
    page.wait_for_timeout(200)

    page.select_option("#acquireCount", "3")
    page.select_option("#recommendTopN", "10")
    page.click("#btnRecommend")
    page.wait_for_selector(".results-area .result-card", timeout=180000)

    results = page.locator(".results-area .result-card")
    assert results.count() >= 1
    expect(page.locator(".results-title")).to_contain_text("+3枚")

    first = results.first
    action_badges = first.locator('span:has-text("新規取得"), span:has-text("凸→")')
    assert action_badges.count() >= 1, "Expected at least 1 card action in result"

    page.close()


def test_recommend_acquire1_no_multi_uncap(server, browser_context):
    """acquire_count=1では複数凸レコメンドが出ないことを確認（1枚しか引けないのでcost=1のみ）"""
    page = fresh_page(browser_context, server)

    select_cards_by_data_id(page, TEST_CARDS_MANY)
    page.click("#btnPot0")
    page.wait_for_timeout(200)

    page.select_option("#acquireCount", "1")
    page.select_option("#recommendTopN", "10")
    page.click("#btnRecommend")
    page.wait_for_selector(".results-area .result-card", timeout=60000)

    results = page.locator(".results-area .result-card")
    assert results.count() >= 1

    all_text = page.locator(".results-area").inner_text()
    for pattern in ["0凸→2凸", "0凸→3凸", "0凸→4凸", "1凸→3凸", "1凸→4凸", "2凸→4凸"]:
        assert pattern not in all_text, (
            f"acquire_count=1 should not have multi-uncap '{pattern}', but found it"
        )

    page.close()


def test_recommend_multi_uncap_display_format(server, browser_context):
    """複数凸レコメンドのUI表示が「N凸→M凸」形式で正しく表示されることを確認"""
    page = fresh_page(browser_context, server)

    select_cards_by_data_id(page, TEST_CARDS_MANY)
    page.click("#btnPot0")
    page.wait_for_timeout(200)

    page.select_option("#acquireCount", "2")
    page.select_option("#recommendTopN", "10")
    page.click("#btnRecommend")
    page.wait_for_selector(".results-area .result-card", timeout=120000)

    multi_uncap_badges = page.locator('.results-area span:has-text("0凸→2凸")')
    if multi_uncap_badges.count() > 0:
        badge = multi_uncap_badges.first
        bg_color = badge.evaluate("el => getComputedStyle(el).backgroundColor")
        assert bg_color, "Multi-uncap badge should have a background color"

        parent_card = badge.locator("xpath=ancestor::div[contains(@class,'result-card')]")
        expect(parent_card).to_contain_text("+")
        expect(parent_card).to_contain_text("#")

    page.close()


def test_recommend_fixed_leader_changes_result(server, browser_context):
    """リーダー固定で結果が変わることを確認"""
    page = fresh_page(browser_context, server)

    select_cards_by_data_id(page, TEST_CARDS_8)
    page.select_option("#acquireCount", "1")
    page.select_option("#recommendTopN", "5")

    page.select_option("#costumeSelect", "")
    page.click("#btnRecommend")
    page.wait_for_selector(".results-area .result-card", timeout=60000)
    auto_text = page.locator(".results-area").inner_text()

    page.select_option("#costumeSelect", "robocosan_5")
    page.click("#btnRecommend")
    page.wait_for_selector(".results-area .result-card", timeout=60000)
    fixed_text = page.locator(".results-area").inner_text()

    assert auto_text != fixed_text, "Results should differ with fixed leader"

    page.close()


def test_recommend_5chars_required(server, browser_context):
    """5キャラ未満でエラーメッセージが表示されることを確認"""
    browser = browser_context.browser
    ctx = browser.new_context()
    page = ctx.new_page()
    page.goto(server)
    page.wait_for_selector(".card", timeout=10000)

    select_cards_by_data_id(page, TEST_CARDS_4_CHARS)

    page.wait_for_timeout(300)
    count_val = int(page.locator("#selectedCount").inner_text())
    assert count_val >= 5, f"Need 5+ cards selected, got {count_val}"

    btn = page.locator("#btnRecommend")
    expect(btn).to_be_enabled()
    btn.click()

    page.wait_for_timeout(2000)
    expect(page.locator("#resultsArea")).to_contain_text("5キャラ以上")

    page.close()
    ctx.close()


def test_mutual_exclusion(server, browser_context):
    """計算中に「最強編成を探す」ボタンが無効化されることを確認"""
    page = fresh_page(browser_context, server)

    select_cards_by_data_id(page, TEST_CARDS_8)
    page.select_option("#acquireCount", "2")

    page.click("#btnRecommend")
    page.wait_for_timeout(500)

    solve_btn = page.locator("#btnSolve")
    expect(solve_btn).to_be_disabled()

    rec_btn = page.locator("#btnRecommend")
    expect(rec_btn).to_be_disabled()

    page.wait_for_selector(".results-area .result-card", timeout=120000)

    expect(solve_btn).to_be_enabled()
    expect(rec_btn).to_be_enabled()

    page.close()


def select_cards_standalone(page, card_ids):
    """静的版ではdata-idがないため、data-card属性からカード要素を特定して選択"""
    for cid in card_ids:
        card_el = page.locator(f'button.pot-btn[data-card="{cid}"]').first
        card_el.wait_for(timeout=5000)
        parent = card_el.locator("xpath=ancestor::div[contains(@class,'card') and not(contains(@class,'card-grid'))]")
        is_selected = parent.evaluate("el => el.classList.contains('selected')")
        if not is_selected:
            parent.locator(".char-name").click()
            page.wait_for_timeout(100)


def test_standalone_recommend(browser_context):
    """スタンドアロン版（dist/holosolve.html）でも同等の動作を確認"""
    import pathlib
    html_path = pathlib.Path(__file__).parent / "dist" / "holosolve.html"
    if not html_path.exists():
        pytest.skip("dist/holosolve.html not found")

    page = browser_context.new_page()
    page.add_init_script("() => localStorage.clear()")
    page.goto(f"file://{html_path.resolve()}")
    page.wait_for_selector(".card", timeout=10000)

    btn = page.locator("#btnRecommend")
    expect(btn).to_be_disabled()

    select_cards_standalone(page, TEST_CARDS_8)
    expect(btn).to_be_enabled()

    page.select_option("#acquireCount", "1")
    page.select_option("#recommendTopN", "5")
    btn.click()

    page.wait_for_selector(".results-area .result-card", timeout=120000)
    results = page.locator(".results-area .result-card")
    assert results.count() == 5, f"Expected 5 results in standalone, got {results.count()}"

    expect(results.first).to_contain_text("#1")
    expect(results.first).to_contain_text("+")

    page.close()
