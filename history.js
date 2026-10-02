/* Exploration history shared by the server and WASM pages. */
const HISTORY_LIMIT = 20;
let historyEntries = [];
let historyReady;
let historyOpenSequence = 0;

function openHistoryDB() {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open("holodri_solve_history", 2);
    request.onupgradeneeded = event => {
      const db = request.result;
      if (!db.objectStoreNames.contains("entries")) db.createObjectStore("entries", { keyPath: "id" });
      if (!db.objectStoreNames.contains("results")) db.createObjectStore("results", { keyPath: "id" });
      if (event.oldVersion === 1) {
        const entries = request.transaction.objectStore("entries");
        const results = request.transaction.objectStore("results");
        entries.openCursor().onsuccess = event => {
          const cursor = event.target.result;
          if (!cursor) return;
          const entry = cursor.value;
          entry.resultCount = (entry.result?.timeline_results || entry.result?.results || entry.results || []).length;
          if (entry.result) {
            results.put({ id: entry.id, result: entry.result });
            delete entry.result;
          }
          cursor.update(entry);
          cursor.continue();
        };
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

function historyTransaction(db, mode, action, storeName = "entries") {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(storeName, mode);
    const request = action(tx.objectStore(storeName));
    let value;
    if (request) request.onsuccess = () => { value = request.result; };
    tx.oncomplete = () => resolve(value);
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error);
  });
}

function writeHistoryRecord(db, entry, result) {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(["entries", "results"], "readwrite");
    tx.objectStore("entries").put(entry);
    if (result) tx.objectStore("results").put({ id: entry.id, result });
    tx.oncomplete = resolve;
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error);
  });
}

function deleteHistoryRecord(db, id) {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(["entries", "results"], "readwrite");
    tx.objectStore("entries").delete(id);
    tx.objectStore("results").delete(id);
    tx.oncomplete = resolve;
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error);
  });
}

async function getHistoryDB() {
  if (!historyReady) {
    historyReady = (async () => {
      const db = await openHistoryDB();
      // Older server entries kept their full result under a separate localStorage key.
      let old = [];
      try { old = JSON.parse(localStorage.getItem("holodri_solve_history") || "[]"); } catch {}
      if (Array.isArray(old)) {
        for (let i = 0; i < old.length; i++) {
          const entry = old[i];
          if (!entry?.snapshot) continue;
          let result = null;
          if (entry.resultKey) {
            try { result = JSON.parse(localStorage.getItem(entry.resultKey)); } catch {}
          }
          await writeHistoryRecord(db, {
            id: `legacy-${entry.ts}-${i}`, ts: entry.ts, label: entry.label || "",
            settings: entry.settings || {}, snapshot: entry.snapshot,
            results: entry.results || [], isTimeline: entry.isTimeline || !!result?.timeline_results,
            resultCount: (result?.timeline_results || result?.results || entry.results || []).length,
          }, result);
        }
        for (const entry of old) {
          if (entry.resultKey) localStorage.removeItem(entry.resultKey);
        }
        localStorage.removeItem("holodri_solve_history");
      }
      return db;
    })().catch(error => { historyReady = null; throw error; });
  }
  return historyReady;
}

function captureHistoryState() {
  const ids = selected.size ? [...selected] : CARDS.map(card => card.id);
  const potentials = {}, levels = {};
  for (const id of ids) {
    const potential = getCardPotential(id), level = getCardLevel(id);
    if (potential !== defaultPotential) potentials[id] = potential;
    if (level !== defaultLevel) levels[id] = level;
  }
  return {
    snapshot: {
      ids: [...selected], allCards: selected.size === 0, potentials, levels,
      defaultPotential, defaultLevel, levelEnabled,
    },
    settings: {
      topN: Number(document.getElementById("topN").value),
      costumeLeaderId: document.getElementById("costumeSelect").value || null,
      memberInclude: document.getElementById("chkMemberInclude").checked,
      songId: document.getElementById("songSelect").value || null,
      difficulty: document.getElementById("diffSelect").value,
      stability: document.getElementById("chkStability").checked,
      boardMode: document.getElementById("boardSearchMode").value,
      statScale: typeof statScale === "undefined" ? null : statScale,
      baseline: typeof baseline === "undefined" ? null : baseline,
    },
  };
}

function historySummary(result) {
  return (result.timeline_results || result.results || []).slice(0, 3).map(r => ({
    rank: r.rank, unit_score: r.unit_score, total_power: r.total_power,
    score_bonus: r.score_bonus, live_score_index: r.live_score_index,
    leader_id: r.leader_id, member_ids: r.member_ids,
  }));
}

function showHistoryError(error) {
  console.error("History storage error:", error);
  document.getElementById("historyError").textContent = "履歴を保存・読み込みできませんでした。ブラウザの保存領域を確認してください。";
}

async function saveToHistory(result, state = captureHistoryState()) {
  try {
    const db = await getHistoryDB();
    const entry = {
      id: `${Date.now()}-${crypto.randomUUID()}`, ts: Date.now(), label: "",
      ...state, results: historySummary(result), isTimeline: !!result.timeline_results,
      resultCount: (result.timeline_results || result.results || []).length,
    };
    await writeHistoryRecord(db, entry, result);
    const entries = await historyTransaction(db, "readonly", store => store.getAll());
    entries.sort((a, b) => b.ts - a.ts || b.id.localeCompare(a.id));
    for (const old of entries.slice(HISTORY_LIMIT)) {
      await deleteHistoryRecord(db, old.id);
    }
    document.getElementById("historyError").textContent = "";
    await renderHistory();
    return true;
  } catch (error) {
    showHistoryError(error);
    return false;
  }
}

function historyEscape(value) {
  const el = document.createElement("span");
  el.textContent = String(value ?? "");
  return el.innerHTML.replace(/"/g, "&quot;");
}

function historyScore(result, timeline) {
  if (!result) return "-";
  if (timeline && Number.isFinite(result.live_score_index)) {
    return `LSI ${Math.round(result.live_score_index).toLocaleString()}`;
  }
  return Number.isFinite(result.unit_score) ? result.unit_score.toLocaleString() : "-";
}

async function renderHistory() {
  try {
    const db = await getHistoryDB();
    historyEntries = await historyTransaction(db, "readonly", store => store.getAll());
    historyEntries.sort((a, b) => b.ts - a.ts || b.id.localeCompare(a.id));
    document.getElementById("historyCount").textContent = historyEntries.length;
    const area = document.getElementById("historyArea");
    if (!historyEntries.length) {
      area.innerHTML = '<div style="color:#5a6e80;padding:8px;font-size:0.8rem">履歴なし</div>';
      return;
    }
    area.innerHTML = historyEntries.map((entry, index) => {
      const date = new Date(entry.ts);
      const time = `${date.getMonth()+1}/${date.getDate()} ${date.getHours()}:${String(date.getMinutes()).padStart(2, "0")}`;
      const summaries = entry.results || [];
      const count = entry.snapshot?.allCards ? "全" : (entry.snapshot?.ids || []).length;
      const resultCount = entry.resultCount ?? summaries.length;
      const resultLabel = resultCount > summaries.length ? `保存済み Top ${resultCount} を見る` : `保存済み上位${resultCount}件を見る`;
      return `<div class="history-entry">
        <div class="h-header"><span class="h-time">${historyEscape(time)} / ${count}枚</span>
          <span class="h-score">${historyEscape(historyScore(summaries[0], entry.isTimeline))}</span></div>
        <input class="h-label-input" placeholder="メモ" value="${historyEscape(entry.label)}" data-index="${index}">
        <details class="h-details" data-index="${index}">
          <summary>${historyEscape(resultLabel)}</summary>
          <div class="history-results results-area"></div>
        </details>
        <div style="display:flex;gap:4px;margin-top:6px">
          <button class="h-btn" data-action="restore" data-index="${index}">復元</button>
          <button class="h-btn" data-action="delete" data-index="${index}">削除</button>
        </div></div>`;
    }).join("");
    area.querySelectorAll(".h-label-input").forEach(input => input.addEventListener("change", async () => {
      const entry = historyEntries[Number(input.dataset.index)];
      if (!entry) return;
      entry.label = input.value;
      try { await historyTransaction(db, "readwrite", store => store.put(entry)); }
      catch (error) { showHistoryError(error); }
    }));
    area.querySelectorAll(".h-btn").forEach(button => button.addEventListener("click", () => {
      const index = Number(button.dataset.index);
      if (button.dataset.action === "restore") restoreFromHistory(index);
      else deleteHistory(index);
    }));
    area.querySelectorAll(".h-details").forEach(details => details.addEventListener("toggle", async () => {
      const target = details.querySelector(".history-results");
      if (!details.open) {
        target.replaceChildren();
        return;
      }
      details.dataset.openedAt = String(++historyOpenSequence);
      const otherOpen = [...area.querySelectorAll(".h-details[open]")].filter(item => item !== details);
      otherOpen.sort((a, b) => Number(a.dataset.openedAt) - Number(b.dataset.openedAt));
      for (const old of otherOpen.slice(0, Math.max(0, otherOpen.length - 1))) old.open = false;
      const entry = historyEntries[Number(details.dataset.index)];
      target.textContent = "結果を読み込み中...";
      try {
        const saved = await historyTransaction(db, "readonly", store => store.get(entry.id), "results");
        if (!details.open) return;
        if (saved?.result) {
          renderResults(saved.result, target, entry);
        } else {
          const rows = (entry.results || []).map((r, i) => {
            const names = (r.member_ids || []).map(id => cardMap[id]?.character || id).join("・");
            return `<div class="h-result">#${r.rank || i+1} ${historyEscape(historyScore(r, entry.isTimeline))}　${historyEscape(names)}</div>`;
          }).join("");
          target.innerHTML = rows || '<div class="h-result">保存済みの結果データがありません</div>';
        }
      } catch (error) { showHistoryError(error); target.textContent = "結果を読み込めませんでした。"; }
    }));
  } catch (error) { showHistoryError(error); }
}

async function deleteHistory(index) {
  const entry = historyEntries[index];
  if (!entry) return;
  try {
    await deleteHistoryRecord(await getHistoryDB(), entry.id);
    await renderHistory();
  } catch (error) { showHistoryError(error); }
}

function restoreFromHistory(index) {
  const entry = historyEntries[index];
  if (!entry?.snapshot || isComputing) return;
  const snap = entry.snapshot, settings = entry.settings || {};
  const valid = new Set(CARDS.map(card => card.id));
  selected.clear();
  Object.keys(cardPotentials).forEach(id => delete cardPotentials[id]);
  Object.keys(cardLevels).forEach(id => delete cardLevels[id]);
  defaultPotential = snap.defaultPotential ?? 0;
  defaultLevel = snap.defaultLevel ?? 80;
  levelEnabled = snap.levelEnabled ?? false;
  if (!snap.allCards) {
    for (const id of snap.ids || []) if (valid.has(id)) selected.add(id);
  }
  for (const [id, potential] of Object.entries(snap.potentials || {})) {
    if (valid.has(id)) cardPotentials[id] = potential;
  }
  for (const [id, level] of Object.entries(snap.levels || {})) {
    if (valid.has(id)) cardLevels[id] = level;
  }
  if (typeof statScale !== "undefined" && settings.statScale != null) {
    statScale = settings.statScale;
    localStorage.setItem("holodri_stat_scale", String(statScale));
  }
  if (typeof baseline !== "undefined" && settings.baseline != null) {
    baseline = settings.baseline;
    localStorage.setItem("holodri_baseline", String(baseline));
  }
  if (settings.topN != null) document.getElementById("topN").value = settings.topN;
  document.getElementById("costumeSelect").value = settings.costumeLeaderId || settings.fixedLeaderId || "";
  document.getElementById("chkMemberInclude").checked = settings.memberInclude ?? true;
  document.getElementById("costumeSelect").dispatchEvent(new Event("change"));
  selectSong(settings.songId || "");
  if (settings.difficulty) document.getElementById("diffSelect").value = settings.difficulty;
  document.getElementById("chkStability").checked = settings.stability ?? false;
  if (settings.boardMode) document.getElementById("boardSearchMode").value = settings.boardMode;
  savePersistence();
  if (typeof syncLevelUI === "function") syncLevelUI();
  else {
    document.getElementById("chkLevelEnabled").checked = levelEnabled;
    document.getElementById("lvGlobalBtns").style.display = levelEnabled ? "" : "none";
  }
  if (typeof updateBaselineDisplay === "function") updateBaselineDisplay();
  renderCards();
  updateCounter();
  document.getElementById("resultsArea").innerHTML = "";
  document.getElementById("resultsWrapper").style.display = "none";
  setFabMode("solve");
  document.getElementById("cardArea").scrollIntoView({ behavior: "smooth" });
}

document.getElementById("historyToggle").addEventListener("click", () => {
  const area = document.getElementById("historyArea");
  const toggle = document.getElementById("historyToggle");
  area.style.display = area.style.display === "none" ? "block" : "none";
  toggle.innerHTML = toggle.innerHTML.replace(/[▶▼]/, area.style.display === "none" ? "▶" : "▼");
});
renderHistory();
