"""cards.json + solver ロジックを埋め込んだスタンドアロン HTML を生成する。

JS のソルバーロジックは solver.py の定数を動的に注入して生成される。
定数の二重管理は発生しない。

Usage:
    uv run python build_static.py
    # → dist/holosolve.html
"""

import json
from pathlib import Path

from solver import (
    ACTIVE_BASE,
    ACTIVE_DIVISOR,
    COSTUME_SS_RATE,
    SONG_LENGTH,
    SUPPORT_SS_RATE,
    UNIT_SCORE_K,
    load_cards,
)

ROOT = Path(__file__).parent


def _generate_solver_js() -> str:
    """solver.py の定数を注入した JS ソルバー（Web Worker用）を生成する"""
    return f"""
const UNIT_SCORE_K = {UNIT_SCORE_K};
const ACTIVE_BASE = {ACTIVE_BASE};
const ACTIVE_DIVISOR = {ACTIVE_DIVISOR};
const COSTUME_SS_RATE = {COSTUME_SS_RATE};
const SUPPORT_SS_RATE = {SUPPORT_SS_RATE};
const SONG_LENGTH = {SONG_LENGTH};

function countTypes(team) {{
  const c = {{happy:0, pure:0, cute:0}};
  for (const card of team) c[card.type]++;
  return c;
}}
function countGroups(team) {{
  const c = {{}};
  for (const card of team) c[card.group] = (c[card.group]||0) + 1;
  return c;
}}
function checkCond(cond, tc, gc) {{
  if (!cond) return true;
  if (cond.type === "type_count") return (tc[cond.type_name]||0) >= cond.min_count;
  if (cond.type === "group_count") return (gc[cond.group]||0) >= cond.min_count;
  return false;
}}
function checkCenterTypeCond(cond, tc) {{
  if (!cond) return false;
  if (cond === "life_600" || cond === "combo_40") return false;
  if (cond.endsWith("_2")) return (tc[cond.slice(0,-2)]||0) >= 2;
  return false;
}}

function evaluateTeam(team, leaderIdx) {{
  const leader = team[leaderIdx];
  const tc = countTypes(team), gc = countGroups(team);

  let cpr=0, ctr=0, csr=0, costumeSS=0;
  const cs = leader.costume_skill;
  if (checkCond(cs.condition, tc, gc)) {{
    for (const e of cs.effects) {{
      const v = e.value / 100;
      if (e.stat === "score_support") {{ costumeSS += v; continue; }}
      if (e.stat === "all" || e.stat === "performance") cpr += v;
      if (e.stat === "all" || e.stat === "technique") ctr += v;
      if (e.stat === "all" || e.stat === "sense") csr += v;
    }}
  }}

  const supportBonus = new Array(team.length).fill(0);
  let supportSS = 0;

  for (let idx = 0; idx < team.length; idx++) {{
    const card = team[idx], sk = card.support_skill;
    const et = sk.effect_type;
    if (!checkCond(sk.condition, tc, gc)) continue;

    if (et === "self_all_param" || et === "self_all_param_conditional") {{
      const s = card.stats;
      supportBonus[idx] += (s.performance + s.technique + s.sense) * sk.value / 100;
    }} else if (et === "type_stat" || et === "type_stat_conditional") {{
      let applied = 0;
      for (let i = 0; i < team.length && applied < sk.target.count; i++) {{
        if (team[i].type === sk.target.type_match) {{
          supportBonus[i] += (team[i].stats[sk.stat]||0) * sk.value / 100;
          applied++;
        }}
      }}
    }} else if (et === "type_all_param") {{
      let applied = 0;
      for (let i = 0; i < team.length && applied < sk.target.count; i++) {{
        if (team[i].type === sk.target.type_match) {{
          const s = team[i].stats;
          supportBonus[i] += (s.performance + s.technique + s.sense) * sk.value / 100;
          applied++;
        }}
      }}
    }} else if (et === "type_score_support") {{
      const req = sk.target.count || 2;
      if ((tc[sk.target.type_match]||0) >= req) supportSS += sk.value / 100;
    }} else if (et === "group_stat" || et === "group_stat_conditional") {{
      let applied = 0;
      for (let i = 0; i < team.length && applied < sk.target.count; i++) {{
        if (team[i].group === sk.target.group) {{
          supportBonus[i] += (team[i].stats[sk.stat]||0) * sk.value / 100;
          applied++;
        }}
      }}
    }} else if (et === "group_score_support_conditional") {{
      const req = sk.target.count || 2;
      if ((gc[sk.target.group]||0) >= req) supportSS += sk.value / 100;
    }}
  }}

  let activeSum = 0;
  for (const card of team) {{
    const cs2 = card.center_skill;
    let su = cs2.score_up;
    if (cs2.condition && checkCenterTypeCond(cs2.condition, tc))
      su = cs2.conditional_score_up || su;
    activeSum += su * cs2.duration / cs2.interval;
  }}
  const activePct = ACTIVE_BASE + activeSum / ACTIVE_DIVISOR;
  const costumeSbPct = costumeSS * 100 * COSTUME_SS_RATE;
  const passiveSbPct = supportSS * 100 * SUPPORT_SS_RATE;
  const specialPct = team.reduce((s,c) =>
    s + (c.special_skill ? c.special_skill.score_support * c.special_skill.duration / SONG_LENGTH : 0), 0);
  const scoreBonus = activePct + costumeSbPct + passiveSbPct + specialPct;

  const totalPerf = team.reduce((s,c) => s + c.stats.performance, 0);
  const totalTech = team.reduce((s,c) => s + c.stats.technique, 0);
  const totalSense = team.reduce((s,c) => s + c.stats.sense, 0);
  const memberParams = totalPerf + totalTech + totalSense;
  const costumeContrib = totalPerf*cpr + totalTech*ctr + totalSense*csr;
  const supportContrib = supportBonus.reduce((a,b) => a+b, 0);
  const totalPower = memberParams + costumeContrib + supportContrib;
  const unitScore = totalPower * (1 + scoreBonus/100) * UNIT_SCORE_K;

  return {{ unitScore, totalPower, scoreBonus: Math.round(scoreBonus*10)/10,
    activePct: Math.round(activePct*10)/10, costumeSbPct: Math.round(costumeSbPct*10)/10,
    passiveSbPct: Math.round(passiveSbPct*10)/10, specialPct: Math.round(specialPct*10)/10,
    leaderIdx }};
}}

self.onmessage = function(e) {{
  const {{ cards, fixedLeaderId, topN }} = e.data;
  const n = cards.length;
  const results = [];
  let count = 0;

  function comb(n,k) {{ let r=1; for(let i=0;i<k;i++) r=r*(n-i)/(i+1); return Math.round(r); }}
  const totalEst = fixedLeaderId ? comb(n-1,4) : comb(n,5);
  const reportAt = Math.max(1, Math.floor(totalEst / 40));

  function pushResult(r) {{
    if (results.length < topN) {{ results.push(r); results.sort((a,b)=>b.unitScore-a.unitScore); }}
    else if (r.unitScore > results[topN-1].unitScore) {{ results[topN-1]=r; results.sort((a,b)=>b.unitScore-a.unitScore); }}
  }}

  if (fixedLeaderId) {{
    const leaderCard = cards.find(c => c.id === fixedLeaderId);
    const others = cards.filter(c => c.id !== fixedLeaderId);
    const m = others.length;
    for (let a=0;a<m-3;a++) for (let b=a+1;b<m-2;b++) for (let c=b+1;c<m-1;c++) for (let d=c+1;d<m;d++) {{
      const team = [leaderCard, others[a], others[b], others[c], others[d]];
      if (new Set(team.map(x=>x.character)).size < 5) continue;
      count++;
      const r = evaluateTeam(team, 0);
      r.teamIds = team.map(x=>x.id);
      pushResult(r);
      if (count % reportAt === 0) self.postMessage({{type:"progress",current:count,total:totalEst}});
    }}
  }} else {{
    for (let a=0;a<n-4;a++) for (let b=a+1;b<n-3;b++) for (let c=b+1;c<n-2;c++) for (let d=c+1;d<n-1;d++) for (let f=d+1;f<n;f++) {{
      const team = [cards[a],cards[b],cards[c],cards[d],cards[f]];
      if (new Set(team.map(x=>x.character)).size < 5) continue;
      count++;
      let best = null;
      for (let li=0;li<5;li++) {{
        const r = evaluateTeam(team, li);
        if (!best || r.unitScore > best.unitScore) best = r;
      }}
      best.teamIds = team.map(x=>x.id);
      pushResult(best);
      if (count % reportAt === 0) self.postMessage({{type:"progress",current:count,total:totalEst}});
    }}
  }}

  self.postMessage({{type:"done", results: results.map((r,i) => ({{
    rank:i+1, unit_score:Math.round(r.unitScore), total_power:Math.round(r.totalPower),
    score_bonus:r.scoreBonus, active_pct:r.activePct, costume_sb_pct:r.costumeSbPct,
    passive_sb_pct:r.passiveSbPct, special_pct:r.specialPct,
    leader_id:r.teamIds[r.leaderIdx], member_ids:r.teamIds
  }})), totalCombinations:count}});
}};
"""


def build():
    cards = list(load_cards())
    cards_json = json.dumps(cards, ensure_ascii=False)
    solver_js = _generate_solver_js()

    with open(ROOT / "index.html", encoding="utf-8") as f:
        template = f.read()

    css_start = template.index("<style>")
    css_end = template.index("</style>") + len("</style>")
    css = template[css_start:css_end]

    static_html = f"""<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>HoloSolve</title>
{css}
<style>
.progress-area {{ margin: 16px 0; display: none; }}
.progress-area.visible {{ display: block; }}
.progress-bar-bg {{ height: 6px; background: #1a2735; border-radius: 3px; overflow: hidden; }}
.progress-bar-fill {{ height: 100%; background: linear-gradient(90deg, #4f8cff, #7b5cf0); width: 0%; transition: width 0.2s; }}
.progress-text {{ font-size: 0.8rem; color: #6b7f92; margin-top: 4px; text-align: center; }}
</style>
</head>
<body>
<div class="container">
  <header>
    <h1>HoloSolve</h1>
    <p>手持ちの星5カードを選択して最強の5人編成を探索（スタンドアロン版）</p>
  </header>

  <div class="controls">
    <button class="btn-solve" id="btnSolve" disabled>最強編成を探す</button>
    <select id="fixedLeader" style="background:#1e2d3d;border:1px solid #3a4f66;color:#8899aa;padding:6px 8px;border-radius:4px;font-size:0.8rem">
      <option value="">リーダー自動選択</option>
    </select>
    <button class="btn-select-all" id="btnSelectAll">全選択</button>
    <button class="btn-clear" id="btnClear">全解除</button>
    <button class="btn-clear" id="btnCopyIds" style="font-size:0.75rem">IDコピー</button>
    <button class="btn-clear" id="btnPasteIds" style="font-size:0.75rem">ID貼付</button>
    <span style="color:#3a4f66">|</span>
    <button class="btn-filter active" data-filter="all">全タイプ</button>
    <button class="btn-filter" data-filter="happy">Happy</button>
    <button class="btn-filter" data-filter="pure">Pure</button>
    <button class="btn-filter" data-filter="cute">Cute</button>
    <span class="counter">選択: <strong id="selectedCount">0</strong> / <span id="totalCount">-</span></span>
  </div>

  <div id="cardArea"></div>

  <div class="progress-area" id="progressArea">
    <div class="progress-bar-bg"><div class="progress-bar-fill" id="progressFill"></div></div>
    <div class="progress-text" id="progressText">計算中...</div>
  </div>

  <div class="results-area" id="resultsArea"></div>
</div>

<script>
const CARDS = {cards_json};
const GROUP_ORDER = ["0期生","1期生","2期生","ゲーマーズ","3期生","4期生","5期生","holoX","ID1期生","ID2期生","ID3期生","Myth","Promise","Advent","ReGLOSS","水着"];
const TYPE_LABELS = {{happy:"Happy",pure:"Pure",cute:"Cute"}};

let cardMap = {{}};
for (const c of CARDS) cardMap[c.id] = c;
const selected = new Set();
let activeFilter = "all";

function groupCards() {{
  const groups = {{}};
  for (const c of CARDS) {{
    const g = c.variant === "水着" ? "水着" : c.group;
    (groups[g] = groups[g] || []).push(c);
  }}
  return groups;
}}

function renderCards() {{
  const area = document.getElementById("cardArea");
  area.innerHTML = "";
  const groups = groupCards();
  for (const gName of GROUP_ORDER) {{
    const cards = groups[gName];
    if (!cards) continue;
    const filtered = activeFilter === "all" ? cards : cards.filter(c => c.type === activeFilter);
    if (filtered.length === 0) continue;
    const section = document.createElement("div");
    section.className = "group-section";
    section.innerHTML = `<div class="group-title">${{gName}}</div>`;
    const grid = document.createElement("div");
    grid.className = "card-grid";
    for (const card of filtered) {{
      const el = document.createElement("div");
      el.className = "card" + (selected.has(card.id) ? " selected" : "");
      const s = card.stats;
      el.innerHTML = `
        <div class="char-name">${{card.character}}</div>
        <div class="card-name">${{card.card_name}}</div>
        <span class="type-badge type-${{card.type}}">${{TYPE_LABELS[card.type]}}</span>
        <div class="stats">
          <span>P:${{s.performance.toLocaleString()}}</span>
          <span>T:${{s.technique.toLocaleString()}}</span>
          <span>S:${{s.sense.toLocaleString()}}</span>
        </div>`;
      el.addEventListener("click", () => {{
        if (selected.has(card.id)) {{ selected.delete(card.id); el.classList.remove("selected"); }}
        else {{ selected.add(card.id); el.classList.add("selected"); }}
        updateCounter();
      }});
      grid.appendChild(el);
    }}
    section.appendChild(grid);
    area.appendChild(section);
  }}
}}

function updateCounter() {{
  document.getElementById("selectedCount").textContent = selected.size;
  document.getElementById("totalCount").textContent = CARDS.length;
  document.getElementById("btnSolve").disabled = selected.size < 5;
  const sel = document.getElementById("fixedLeader");
  const cur = sel.value;
  sel.innerHTML = '<option value="">リーダー自動選択</option>';
  for (const id of selected) {{
    const c = cardMap[id]; if (!c) continue;
    const opt = document.createElement("option");
    opt.value = id; opt.textContent = c.character + (c.variant ? `[${{c.variant}}]` : "");
    sel.appendChild(opt);
  }}
  if (selected.has(cur)) sel.value = cur;
}}

document.getElementById("btnSelectAll").addEventListener("click", () => {{
  (activeFilter === "all" ? CARDS : CARDS.filter(c => c.type === activeFilter)).forEach(c => selected.add(c.id));
  renderCards(); updateCounter();
}});
document.getElementById("btnClear").addEventListener("click", () => {{ selected.clear(); renderCards(); updateCounter(); }});
document.getElementById("btnCopyIds").addEventListener("click", () => {{
  navigator.clipboard.writeText(JSON.stringify([...selected])).then(() => {{
    const b = document.getElementById("btnCopyIds"); b.textContent = "コピー済!"; setTimeout(() => b.textContent = "IDコピー", 1500);
  }});
}});
document.getElementById("btnPasteIds").addEventListener("click", async () => {{
  let text; try {{ text = await navigator.clipboard.readText(); }} catch {{ text = prompt("JSON配列を貼り付け:"); }}
  if (!text) return;
  try {{
    const ids = JSON.parse(text); if (!Array.isArray(ids)) throw 0;
    const valid = new Set(CARDS.map(c=>c.id)); selected.clear();
    let n=0; for (const id of ids) if (valid.has(id)) {{ selected.add(id); n++; }}
    renderCards(); updateCounter();
    const b = document.getElementById("btnPasteIds"); b.textContent = n+"枚読込!"; setTimeout(() => b.textContent = "ID貼付", 1500);
  }} catch {{ const b = document.getElementById("btnPasteIds"); b.textContent = "形式エラー"; setTimeout(() => b.textContent = "ID貼付", 1500); }}
}});
for (const btn of document.querySelectorAll(".btn-filter")) {{
  btn.addEventListener("click", () => {{
    document.querySelectorAll(".btn-filter").forEach(b => b.classList.remove("active"));
    btn.classList.add("active"); activeFilter = btn.dataset.filter; renderCards();
  }});
}}

const workerBlob = URL.createObjectURL(new Blob([{json.dumps(solver_js)}], {{type:"application/javascript"}}));

document.getElementById("btnSolve").addEventListener("click", () => {{
  const btn = document.getElementById("btnSolve");
  const pa = document.getElementById("progressArea");
  btn.disabled = true; btn.textContent = "計算中...";
  pa.classList.add("visible");
  document.getElementById("resultsArea").innerHTML = "";

  const owned = CARDS.filter(c => selected.has(c.id));
  const fixedLeaderId = document.getElementById("fixedLeader").value || null;
  const w = new Worker(workerBlob);
  w.postMessage({{ cards: owned, fixedLeaderId, topN: 10 }});

  w.onmessage = function(ev) {{
    if (ev.data.type === "progress") {{
      const pct = Math.min(100, ev.data.current / ev.data.total * 100);
      document.getElementById("progressFill").style.width = pct + "%";
      document.getElementById("progressText").textContent =
        `${{ev.data.current.toLocaleString()}} / ${{ev.data.total.toLocaleString()}} 組み合わせを評価中...`;
    }} else if (ev.data.type === "done") {{
      w.terminate();
      btn.disabled = false; btn.textContent = "最強編成を探す";
      document.getElementById("progressFill").style.width = "100%";
      document.getElementById("progressText").textContent =
        `完了！ ${{ev.data.totalCombinations.toLocaleString()}} 通りを評価`;
      renderResults(ev.data);
    }}
  }};
}});

function renderResults(data) {{
  const area = document.getElementById("resultsArea");
  const results = data.results;
  if (!results || !results.length) {{ area.innerHTML = '<div class="empty-msg">結果が見つかりませんでした。</div>'; return; }}
  let html = `<div class="results-title">最強編成 Top ${{results.length}}（${{data.totalCombinations.toLocaleString()}} 通り）</div>`;
  for (const r of results) {{
    const rankColors = {{ 1: "#ffd700", 2: "#c0c0c0", 3: "#cd7f32" }};
    const rc = rankColors[r.rank] || "";
    html += `<div class="result-card">
      <div class="result-header">
        <span class="result-rank" ${{rc ? `style="color:${{rc}}"` : ''}}>#${{r.rank}}</span>
        <div class="result-scores">
          <span>ユニットスコア: <span class="main-score">${{r.unit_score.toLocaleString()}}</span></span>
          <span>総合力: ${{r.total_power.toLocaleString()}}</span>
          <span>SB: ${{r.score_bonus}}%</span>
          <span style="font-size:0.7rem;color:#5a6e80">(Active ${{r.active_pct}}%${{r.costume_sb_pct > 0 ? ` / 衣装SS ${{r.costume_sb_pct}}%` : ''}} / SS ${{r.passive_sb_pct}}% / SP ${{r.special_pct}}%)</span>
        </div>
      </div>
      <div class="result-members">`;
    for (const mid of r.member_ids) {{
      const card = cardMap[mid]; if (!card) continue;
      const isLeader = mid === r.leader_id;
      const s = card.stats;
      html += `<div class="member-card${{isLeader?" is-leader":""}}">
        <span class="type-badge type-${{card.type}}" style="float:right;margin-top:2px">${{TYPE_LABELS[card.type]}}</span>
        <div class="m-name">${{card.character}}</div>
        <div class="m-card-name">${{card.card_name}}</div>
        <div class="m-stats">
          <span>P:${{s.performance.toLocaleString()}}</span>
          <span>T:${{s.technique.toLocaleString()}}</span>
          <span>S:${{s.sense.toLocaleString()}}</span>
        </div>
      </div>`;
    }}
    html += `</div></div>`;
  }}
  area.innerHTML = html;
}}

renderCards();
updateCounter();
</script>
</body>
</html>"""

    dist = ROOT / "dist"
    dist.mkdir(exist_ok=True)
    out = dist / "holosolve.html"
    out.write_text(static_html, encoding="utf-8")
    print(f"Built: {out} ({out.stat().st_size / 1024:.0f} KB)")
    print(f"Constants from solver.py: UNIT_SCORE_K={UNIT_SCORE_K}, ACTIVE_BASE={ACTIVE_BASE}, "
          f"ACTIVE_DIVISOR={ACTIVE_DIVISOR}, COSTUME_SS_RATE={COSTUME_SS_RATE}, "
          f"SUPPORT_SS_RATE={SUPPORT_SS_RATE}, SONG_LENGTH={SONG_LENGTH}")


if __name__ == "__main__":
    build()
