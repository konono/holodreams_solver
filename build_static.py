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
    _load_card_data,
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
const MAX_LEVEL = 80;

function resolveCard(card, potential, level, levelTables) {{
  const pd = card.potential_data;
  if (!pd) return card;
  const pot = Math.max(0, Math.min(potential, pd.length - 1));
  const snap = pd[pot];
  if (!snap) return card;

  const resolved = {{
    id: card.id,
    holodori_id: card.holodori_id || "",
    character: card.character,
    card_name: card.card_name || "",
    rarity: card.rarity || 5,
    type: card.type,
    group: card.group,
    potential: pot,
    center_skill: snap.center_skill,
    support_skill: snap.support_skill,
    costume_skill: snap.costume_skill,
    special_skill: snap.special_skill,
  }};
  if (card.variant) resolved.variant = card.variant;

  const actualLv = Math.max(1, Math.min(level || MAX_LEVEL, MAX_LEVEL));
  resolved.level = actualLv;

  if (actualLv === 80) {{
    resolved.stats = {{ ...(snap.ref_stats_lv80 || snap.stats) }};
  }} else {{
    const table = levelTables[card.card_level_group_id];
    if (table) {{
      const baseValue = table[String(actualLv)];
      if (baseValue) {{
        const permil = card.permil || {{}};
        const bonus = snap.param_bonus_permil || 0;
        const mul = 1000 + bonus;
        function cd(a, b) {{ return Math.floor((a + b - 1) / b); }}
        resolved.stats = {{
          performance: cd(baseValue * (permil.performance || 333) * mul, 1000000),
          technique: cd(baseValue * (permil.technique || 333) * mul, 1000000),
          sense: cd(baseValue * (permil.sense || 334) * mul, 1000000),
        }};
      }} else {{
        resolved.stats = {{ ...(snap.ref_stats_lv80 || snap.stats) }};
      }}
    }} else {{
      resolved.stats = {{ ...(snap.ref_stats_lv80 || snap.stats) }};
    }}
  }}
  return resolved;
}}

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
function checkCond(cond, tc, gc, leader) {{
  if (!cond) return true;
  if (typeof cond === "string") return false;
  if (cond.type === "type_count") return (tc[cond.type_name]||0) >= cond.min_count;
  if (cond.type === "group_count") return (gc[cond.group]||0) >= cond.min_count;
  if (cond.type === "leader_character" && leader) {{
    const hid = leader.holodori_id || "";
    const parts = hid.split("-");
    const charId = parts.length >= 2 ? "chr-" + parts[1] : "";
    return (cond.character_ids || []).includes(charId);
  }}
  if (cond.type === "leader_group" && leader) return leader.group === cond.group;
  return false;
}}
function checkCenterTypeCond(cond, tc) {{
  if (!cond) return false;
  if (cond === "life_600" || cond === "combo_40") return false;
  if (typeof cond === "string" && cond.endsWith("_2")) return (tc[cond.slice(0,-2)]||0) >= 2;
  return false;
}}

function evaluateTeam(team, leaderIdx, songLen) {{
  const SLEN = songLen || SONG_LENGTH;
  const leader = team[leaderIdx];
  const tc = countTypes(team), gc = countGroups(team);

  let cpr=0, ctr=0, csr=0, costumeSS=0;
  const cs = leader.costume_skill;
  if (checkCond(cs.condition, tc, gc, leader)) {{
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
    if (!checkCond(sk.condition, tc, gc, leader)) continue;

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
    }} else if (et === "group_score_support_conditional" || et === "group_score_support") {{
      const req = sk.target.count || 2;
      if ((gc[sk.target.group]||0) >= req) supportSS += sk.value / 100;
    }} else if (et === "self_stat") {{
      supportBonus[idx] += (card.stats[sk.stat]||0) * sk.value / 100;
    }} else if (et === "group_all_param" || et === "group_all_param_conditional") {{
      let applied = 0;
      for (let i = 0; i < team.length && applied < sk.target.count; i++) {{
        if (team[i].group === sk.target.group) {{
          const ss = team[i].stats;
          supportBonus[i] += (ss.performance + ss.technique + ss.sense) * sk.value / 100;
          applied++;
        }}
      }}
    }}
  }}

  let rateUpTimeAvg = 0;
  for (const c of team) {{
    const sp = c.special_skill;
    if (sp && sp.skill_rate_up > 0)
      rateUpTimeAvg += sp.skill_rate_up * 10 * sp.duration / SLEN;
  }}

  const activeMembers = [];
  for (const card of team) {{
    const cs2 = card.center_skill;
    let su = cs2.score_up;
    if (cs2.condition && checkCenterTypeCond(cs2.condition, tc))
      su = cs2.conditional_score_up || su;
    const baseProb = (cs2.activation_probability_permil || 1000) / 1000;
    const boostedProb = Math.min(1, baseProb + rateUpTimeAvg / 1000);
    const uptime = Math.min(1, cs2.duration / cs2.interval * boostedProb);
    activeMembers.push({{ value: su, uptime }});
  }}
  activeMembers.sort((a,b) => b.value - a.value);
  let activeSum = 0, probNoneHigher = 1;
  for (const m of activeMembers) {{
    activeSum += m.value * m.uptime * probNoneHigher;
    probNoneHigher *= (1 - m.uptime);
  }}
  const activePct = ACTIVE_BASE + activeSum / ACTIVE_DIVISOR;
  const costumeSbPct = costumeSS * 100 * COSTUME_SS_RATE;
  const passiveSbPct = supportSS * 100 * SUPPORT_SS_RATE;
  const specialPct = team.reduce((s,c) =>
    s + (c.special_skill ? c.special_skill.score_support * c.special_skill.duration / SLEN : 0), 0);
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
  const {{ cards, fixedLeaderId, topN, potentials, levels, levelTables, songLength }} = e.data;
  const SLEN = songLength || SONG_LENGTH;

  const resolved = cards.map(c => {{
    const pot = potentials[c.id] ?? 0;
    const lv = levels[c.id] ?? MAX_LEVEL;
    return resolveCard(c, pot, lv, levelTables || {{}});
  }});

  const charGroups = {{}};
  for (const c of resolved) {{
    if (!charGroups[c.character]) charGroups[c.character] = [];
    charGroups[c.character].push(c);
  }}
  const charNames = Object.keys(charGroups).sort((a,b) =>
    Math.max(...charGroups[b].map(c=>c.stats.performance+c.stats.technique+c.stats.sense)) -
    Math.max(...charGroups[a].map(c=>c.stats.performance+c.stats.technique+c.stats.sense)));
  const nChars = charNames.length;

  const results = [];
  let count = 0;

  const charSizes = charNames.map(ch => charGroups[ch].length);
  let totalEst = 0;
  if (fixedLeaderId) {{
    const leaderCh = (resolved.find(c => c.id === fixedLeaderId) || {{}}).character;
    const otherIdx = charNames.map((ch, i) => i).filter(i => charNames[i] !== leaderCh);
    const m = otherIdx.length;
    for (let a=0;a<m-3;a++) for (let b=a+1;b<m-2;b++) for (let ci=b+1;ci<m-1;ci++) for (let d=ci+1;d<m;d++)
      totalEst += charSizes[otherIdx[a]] * charSizes[otherIdx[b]] * charSizes[otherIdx[ci]] * charSizes[otherIdx[d]];
  }} else {{
    for (let a=0;a<nChars-4;a++) for (let b=a+1;b<nChars-3;b++) for (let ci=b+1;ci<nChars-2;ci++) for (let d=ci+1;d<nChars-1;d++) for (let f=d+1;f<nChars;f++)
      totalEst += charSizes[a] * charSizes[b] * charSizes[ci] * charSizes[d] * charSizes[f];
  }}
  const reportAt = Math.max(1, Math.floor(totalEst / 40));

  function pushResult(r) {{
    if (results.length < topN) {{ results.push(r); results.sort((a,b)=>b.unitScore-a.unitScore); }}
    else if (r.unitScore > results[topN-1].unitScore) {{ results[topN-1]=r; results.sort((a,b)=>b.unitScore-a.unitScore); }}
  }}

  if (fixedLeaderId) {{
    const leaderCard = resolved.find(c => c.id === fixedLeaderId);
    if (!leaderCard) {{
      self.postMessage({{type:"done", results:[], totalCombinations:0}});
      return;
    }}
    const leaderChar = leaderCard.character;
    const otherChars = charNames.filter(ch => ch !== leaderChar);
    const m = otherChars.length;
    for (let a=0;a<m-3;a++) for (let b=a+1;b<m-2;b++) for (let ci=b+1;ci<m-1;ci++) for (let d=ci+1;d<m;d++) {{
      const gs = [charGroups[otherChars[a]], charGroups[otherChars[b]], charGroups[otherChars[ci]], charGroups[otherChars[d]]];
      for (const c0 of gs[0]) for (const c1 of gs[1]) for (const c2 of gs[2]) for (const c3 of gs[3]) {{
        const team = [leaderCard, c0, c1, c2, c3];
        count++;
        const r = evaluateTeam(team, 0, SLEN);
        r.teamIds = team.map(x=>x.id);
        pushResult(r);
        if (count % reportAt === 0) self.postMessage({{type:"progress",current:count,total:totalEst}});
      }}
    }}
  }} else {{
    for (let a=0;a<nChars-4;a++) for (let b=a+1;b<nChars-3;b++) for (let ci=b+1;ci<nChars-2;ci++) for (let d=ci+1;d<nChars-1;d++) for (let f=d+1;f<nChars;f++) {{
      const gs = [charGroups[charNames[a]], charGroups[charNames[b]], charGroups[charNames[ci]], charGroups[charNames[d]], charGroups[charNames[f]]];
      for (const c0 of gs[0]) for (const c1 of gs[1]) for (const c2 of gs[2]) for (const c3 of gs[3]) for (const c4 of gs[4]) {{
        const team = [c0, c1, c2, c3, c4];
        count++;
        let best = null;
        for (let li=0;li<5;li++) {{
          const r = evaluateTeam(team, li, SLEN);
          if (!best || r.unitScore > best.unitScore) best = r;
        }}
        best.teamIds = team.map(x=>x.id);
        pushResult(best);
        if (count % reportAt === 0) self.postMessage({{type:"progress",current:count,total:totalEst}});
      }}
    }}
  }}

  function permutations(arr) {{
    if (arr.length <= 1) return [arr];
    const result = [];
    for (let i = 0; i < arr.length; i++) {{
      const rest = arr.slice(0,i).concat(arr.slice(i+1));
      for (const p of permutations(rest)) result.push([arr[i], ...p]);
    }}
    return result;
  }}

  const resolvedMap = {{}};
  for (const c of resolved) resolvedMap[c.id] = c;

  for (let ri = 0; ri < results.length; ri++) {{
    const r = results[ri];
    const leaderCard = resolvedMap[r.teamIds[r.leaderIdx]];
    const otherCards = r.teamIds.filter((_,i) => i !== r.leaderIdx).map(id => resolvedMap[id]);
    const indices = otherCards.map((_,i) => i);

    for (const perm of permutations(indices)) {{
      const team = [leaderCard, ...perm.map(i => otherCards[i])];
      const score = evaluateTeam(team, 0, SLEN);
      if (score.unitScore > r.unitScore) {{
        score.teamIds = team.map(c => c.id);
        score.leaderIdx = 0;
        results[ri] = score;
      }}
    }}
  }}
  results.sort((a,b) => b.unitScore - a.unitScore);

  self.postMessage({{type:"done", results: results.map((r,i) => ({{
    rank:i+1, unit_score:Math.round(r.unitScore), total_power:Math.round(r.totalPower),
    score_bonus:r.scoreBonus, active_pct:r.activePct, costume_sb_pct:r.costumeSbPct,
    passive_sb_pct:r.passiveSbPct, special_pct:r.specialPct,
    leader_id:r.teamIds[r.leaderIdx], member_ids:r.teamIds
  }})), totalCombinations:count}});
}};
"""


def build():
    card_data = _load_card_data()
    cards = list(load_cards())
    level_tables = card_data.get("level_tables", {})
    cards_json = json.dumps(cards, ensure_ascii=False)
    level_tables_json = json.dumps(level_tables, ensure_ascii=False)
    songs_path = ROOT / "data" / "songs.json"
    songs = []
    if songs_path.exists():
        with open(songs_path, encoding="utf-8") as f:
            songs = json.load(f).get("songs", [])
    songs_json = json.dumps(songs, ensure_ascii=False)
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
    <select id="topN" style="background:#1e2d3d;border:1px solid #3a4f66;color:#8899aa;padding:6px 8px;border-radius:4px;font-size:0.8rem">
      <option value="10" selected>Top 10</option>
      <option value="30">Top 30</option>
      <option value="50">Top 50</option>
      <option value="100">Top 100</option>
    </select>
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
    <span class="counter">選択: <strong id="selectedCount">0</strong> / <span id="totalCount">-</span> <span id="limitWarn" style="color:#f06060;font-size:0.75rem"></span></span>
  </div>

  <div style="display:flex;align-items:center;gap:10px;margin-bottom:8px;font-size:0.85rem;color:#8899aa;flex-wrap:wrap">
    <label>凸:</label>
    <button class="btn-pot-global" id="btnPot0">全て0凸</button>
    <button class="btn-pot-global" id="btnPot4">全て4凸</button>
    <span style="color:#3a4f66">|</span>
    <label style="cursor:pointer"><input type="checkbox" id="chkLevelEnabled" style="vertical-align:middle;margin-right:2px">Lv設定</label>
    <span id="lvGlobalBtns" style="display:none">
      <button class="btn-pot-global btn-lv-global" data-lv="10">Lv10</button>
      <button class="btn-pot-global btn-lv-global" data-lv="20">Lv20</button>
      <button class="btn-pot-global btn-lv-global" data-lv="30">Lv30</button>
      <button class="btn-pot-global btn-lv-global" data-lv="40">Lv40</button>
      <button class="btn-pot-global btn-lv-global" data-lv="50">Lv50</button>
      <button class="btn-pot-global btn-lv-global" data-lv="60">Lv60</button>
      <button class="btn-pot-global btn-lv-global" data-lv="70">Lv70</button>
      <button class="btn-pot-global btn-lv-global" data-lv="80">Lv80</button>
    </span>
    <span style="color:#3a4f66">|</span>
    <label>曲:</label>
    <select id="songSelect" style="background:#1e2d3d;border:1px solid #3a4f66;color:#8899aa;padding:4px 6px;border-radius:4px;font-size:0.75rem;max-width:200px">
      <option value="">汎用（192秒）</option>
    </select>
  </div>

  <div id="cardArea"></div>

  <div class="progress-area" id="progressArea">
    <div class="progress-bar-bg"><div class="progress-bar-fill" id="progressFill"></div></div>
    <div class="progress-text" id="progressText">計算中...</div>
  </div>

  <div id="resultsWrapper" style="display:none">
    <div id="resultsToggle" style="cursor:pointer;color:#8899aa;font-size:0.85rem;padding:8px 0;border-bottom:1px solid #2a3a4a;user-select:none;margin-bottom:12px">
      ▼ 結果を折りたたむ
    </div>
    <div class="results-area" id="resultsArea"></div>
  </div>

  <div style="margin-top:24px">
    <div id="historyToggle" style="cursor:pointer;color:#6b7f92;font-size:0.85rem;padding:8px 0;border-top:1px solid #2a3a4a;user-select:none">
      ▶ 履歴 (<span id="historyCount">0</span>件)
    </div>
    <div id="historyArea" style="display:none"></div>
  </div>
</div>

<div id="fabSolve" class="fab-solve">最強編成を探す</div>

<script>
const CARDS = {cards_json};
const LEVEL_TABLES = {level_tables_json};
const SONGS = {songs_json};
const GROUP_ORDER = ["0期生","1期生","2期生","ゲーマーズ","3期生","4期生","5期生","holoX","ID1期生","ID2期生","ID3期生","Myth","Promise","Advent","ReGLOSS","水着"];
const TYPE_LABELS = {{happy:"Happy",pure:"Pure",cute:"Cute"}};
const MAX_LEVEL = 80;

let cardMap = {{}};
for (const c of CARDS) cardMap[c.id] = c;
const selected = new Set();
let activeFilter = "all";

let defaultPotential = 0;
let defaultLevel = 80;
const cardPotentials = {{}};
const cardLevels = {{}};
let levelEnabled = false;

function getCardPotential(id) {{ return cardPotentials[id] ?? defaultPotential; }}
function getCardLevel(id) {{
  if (!levelEnabled) return 80;
  if (cardLevels[id] != null) return cardLevels[id];
  return defaultLevel;
}}

function getCardStats(card, potential, level) {{
  const pd = card.potential_data;
  if (!pd) return card.stats;
  const snap = pd[Math.min(potential, pd.length - 1)];
  if (!snap) return card.stats;
  const actualLv = Math.max(1, Math.min(level || MAX_LEVEL, MAX_LEVEL));
  if (actualLv === MAX_LEVEL) return snap.ref_stats_lv80 || snap.stats;
  const table = LEVEL_TABLES[card.card_level_group_id];
  if (!table) return snap.ref_stats_lv80 || snap.stats;
  const baseValue = table[String(actualLv)];
  if (!baseValue) return snap.ref_stats_lv80 || snap.stats;
  const permil = card.permil || {{}};
  const bonus = snap.param_bonus_permil || 0;
  const mul = 1000 + bonus;
  function cd(a, b) {{ return Math.floor((a + b - 1) / b); }}
  return {{
    performance: cd(baseValue * (permil.performance || 333) * mul, 1000000),
    technique: cd(baseValue * (permil.technique || 333) * mul, 1000000),
    sense: cd(baseValue * (permil.sense || 334) * mul, 1000000),
  }};
}}

function loadPersistence() {{
  try {{
    const sp = localStorage.getItem("holodri_card_potentials");
    if (sp) Object.assign(cardPotentials, JSON.parse(sp));
    const sl = localStorage.getItem("holodri_card_levels");
    if (sl) Object.assign(cardLevels, JSON.parse(sl));
    const dp = localStorage.getItem("holodri_default_potential");
    if (dp != null) defaultPotential = parseInt(dp) || 0;
    const dl = localStorage.getItem("holodri_default_level");
    const le = localStorage.getItem("holodri_level_enabled");
    if (dl != null) {{
      defaultLevel = parseInt(dl) || 80;
    }} else if (le === "true") {{
      defaultLevel = 40;
    }}
    if (le != null) levelEnabled = le === "true";
    const ss = localStorage.getItem("holodri_selected");
    if (ss) {{ for (const id of JSON.parse(ss)) selected.add(id); }}
  }} catch {{}}
}}

function savePersistence() {{
  localStorage.setItem("holodri_card_potentials", JSON.stringify(cardPotentials));
  localStorage.setItem("holodri_card_levels", JSON.stringify(cardLevels));
  localStorage.setItem("holodri_default_potential", String(defaultPotential));
  localStorage.setItem("holodri_default_level", String(defaultLevel));
  localStorage.setItem("holodri_level_enabled", String(levelEnabled));
  localStorage.setItem("holodri_selected", JSON.stringify([...selected]));
}}

loadPersistence();
for (const id of [...selected]) {{ if (!cardMap[id]) selected.delete(id); }}

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
      const pot = getCardPotential(card.id);
      const lv = getCardLevel(card.id);
      const s = getCardStats(card, pot, lv);
      el.innerHTML = `
        <div class="char-name">${{card.character}}</div>
        <div class="card-name">${{card.card_name}}</div>
        <span class="type-badge type-${{card.type}}">${{TYPE_LABELS[card.type]}}</span>
        <div class="stats" data-card-id="${{card.id}}">
          <span>P:${{s.performance.toLocaleString()}}</span>
          <span>T:${{s.technique.toLocaleString()}}</span>
          <span>S:${{s.sense.toLocaleString()}}</span>
        </div>
        <div class="pot-row">
          <span class="pot-label">凸:</span>
          ${{[0,1,2,3,4].map(p => `<button class="pot-btn${{p === pot ? ' active' : ''}}" data-pot="${{p}}" data-card="${{card.id}}">${{p}}</button>`).join("")}}
        </div>
        <div class="lv-row" style="${{levelEnabled ? '' : 'display:none'}}">
          <span class="lv-label">Lv:</span>
          <button class="lv-btn" data-delta="-10" data-card="${{card.id}}">-10</button>
          <input class="lv-input" type="number" value="${{lv}}" min="1" max="${{MAX_LEVEL}}" data-card="${{card.id}}">
          <button class="lv-btn" data-delta="10" data-card="${{card.id}}">+10</button>
        </div>`;

      el.querySelector(".char-name").addEventListener("click", () => toggleCard(card.id, el));
      el.querySelector(".card-name").addEventListener("click", () => toggleCard(card.id, el));
      el.querySelector(".type-badge").addEventListener("click", () => toggleCard(card.id, el));

      el.querySelectorAll(".pot-btn").forEach(btn => {{
        btn.addEventListener("click", (e) => {{
          e.stopPropagation();
          const newPot = parseInt(btn.dataset.pot);
          cardPotentials[card.id] = newPot;

          savePersistence();
          updateCardDisplay(card.id, el);
        }});
      }});

      el.querySelectorAll(".lv-btn").forEach(btn => {{
        btn.addEventListener("click", (e) => {{
          e.stopPropagation();
          const delta = parseInt(btn.dataset.delta);
          const currentLv = getCardLevel(card.id);
          const newLv = Math.max(1, Math.min(currentLv + delta, MAX_LEVEL));
          cardLevels[card.id] = newLv;
          savePersistence();
          updateCardDisplay(card.id, el);
        }});
      }});

      const lvInput = el.querySelector(".lv-input");
      lvInput.addEventListener("click", (e) => e.stopPropagation());
      lvInput.addEventListener("change", (e) => {{
        e.stopPropagation();
        const newLv = Math.max(1, Math.min(parseInt(lvInput.value) || 1, MAX_LEVEL));
        cardLevels[card.id] = newLv;
        lvInput.value = newLv;
        savePersistence();
        updateCardDisplay(card.id, el);
      }});

      grid.appendChild(el);
    }}
    section.appendChild(grid);
    area.appendChild(section);
  }}
}}

function updateCardDisplay(cardId, el) {{
  const card = cardMap[cardId];
  const pot = getCardPotential(cardId);
  const lv = getCardLevel(cardId);
  const s = getCardStats(card, pot, lv);
  const statsEl = el.querySelector(".stats");
  statsEl.innerHTML = `
    <span>P:${{s.performance.toLocaleString()}}</span>
    <span>T:${{s.technique.toLocaleString()}}</span>
    <span>S:${{s.sense.toLocaleString()}}</span>`;
  el.querySelectorAll(".pot-btn").forEach(btn => {{
    btn.classList.toggle("active", parseInt(btn.dataset.pot) === pot);
  }});
  const lvInput = el.querySelector(".lv-input");
  lvInput.value = lv;
  lvInput.max = MAX_LEVEL;
}}

function toggleCard(id, el) {{
  if (selected.has(id)) {{ selected.delete(id); el.classList.remove("selected"); }}
  else {{ selected.add(id); el.classList.add("selected"); }}
  updateCounter();
}}

function updateCounter() {{
  document.getElementById("selectedCount").textContent = selected.size;
  document.getElementById("totalCount").textContent = CARDS.length;
  document.getElementById("btnSolve").disabled = selected.size > 0 && selected.size < 5;
  const leader = document.getElementById("fixedLeader").value;
  document.getElementById("limitWarn").textContent =
    selected.size === 0 ? (leader ? "(リーダー固定 + 全カードで探索)" : "(全カードで探索)") : "";
  const sel = document.getElementById("fixedLeader");
  const cur = sel.value;
  const pool = selected.size > 0 ? CARDS.filter(c => selected.has(c.id)) : CARDS;
  sel.innerHTML = '<option value="">リーダー自動選択</option>';
  for (const c of pool) {{
    const opt = document.createElement("option");
    opt.value = c.id; opt.textContent = c.character + (c.variant ? `[${{c.variant}}]` : "");
    sel.appendChild(opt);
  }}
  if (pool.some(c => c.id === cur)) sel.value = cur;
  if (typeof updateFab === "function") updateFab();
  localStorage.setItem("holodri_selected", JSON.stringify([...selected]));
}}

document.getElementById("btnSelectAll").addEventListener("click", () => {{
  (activeFilter === "all" ? CARDS : CARDS.filter(c => c.type === activeFilter)).forEach(c => selected.add(c.id));
  renderCards(); updateCounter();
}});
document.getElementById("btnClear").addEventListener("click", () => {{ selected.clear(); renderCards(); updateCounter(); }});

document.getElementById("btnPot0").addEventListener("click", () => {{
  defaultPotential = 0;
  for (const c of CARDS) cardPotentials[c.id] = 0;
  savePersistence(); renderCards();
}});
document.getElementById("btnPot4").addEventListener("click", () => {{
  defaultPotential = 4;
  for (const c of CARDS) cardPotentials[c.id] = 4;
  savePersistence(); renderCards();
}});

const chkLevel = document.getElementById("chkLevelEnabled");
chkLevel.checked = levelEnabled;
document.getElementById("lvGlobalBtns").style.display = levelEnabled ? "" : "none";
chkLevel.addEventListener("change", () => {{
  levelEnabled = chkLevel.checked;
  document.getElementById("lvGlobalBtns").style.display = levelEnabled ? "" : "none";
  savePersistence();
  renderCards();
}});

for (const btn of document.querySelectorAll(".btn-lv-global")) {{
  btn.addEventListener("click", () => {{
    if (!CARDS.length) return;
    const lv = parseInt(btn.dataset.lv);
    defaultLevel = lv;
    for (const c of CARDS) cardLevels[c.id] = lv;
    savePersistence();
    renderCards();
  }});
}}

const songSel = document.getElementById("songSelect");
for (const s of SONGS) {{
  const opt = document.createElement("option");
  opt.value = s.playing_seconds;
  opt.textContent = `${{s.name}} (${{s.playing_seconds}}秒)`;
  songSel.appendChild(opt);
}}

document.getElementById("btnCopyIds").addEventListener("click", () => {{
  const ids = selected.size > 0 ? [...selected] : CARDS.map(c => c.id);
  const pots = {{}};
  const lvs = {{}};
  for (const id of ids) {{
    const p = getCardPotential(id);
    if (p !== defaultPotential) pots[id] = p;
    const l = getCardLevel(id);
    if (l !== defaultLevel) lvs[id] = l;
  }}
  const payload = {{
    v: 1, ids, allCards: selected.size === 0,
    potentials: pots, levels: lvs,
    defaultPotential, defaultLevel, levelEnabled,
  }};
  navigator.clipboard.writeText(JSON.stringify(payload)).then(() => {{
    const b = document.getElementById("btnCopyIds"); b.textContent = "コピー済!"; setTimeout(() => b.textContent = "IDコピー", 1500);
  }});
}});
document.getElementById("btnPasteIds").addEventListener("click", async () => {{
  const btn = document.getElementById("btnPasteIds");
  let text;
  try {{ text = await navigator.clipboard.readText(); }} catch {{ text = prompt("カードデータのJSONを貼り付けてください:"); }}
  if (!text) return;
  try {{
    const parsed = JSON.parse(text);
    const validIds = new Set(CARDS.map(c => c.id));
    selected.clear();
    let loaded = 0;
    if (Array.isArray(parsed)) {{
      for (const id of parsed) {{
        if (validIds.has(id)) {{ selected.add(id); loaded++; }}
      }}
    }} else if (parsed && parsed.v === 1) {{
      for (const k of Object.keys(cardPotentials)) delete cardPotentials[k];
      for (const k of Object.keys(cardLevels)) delete cardLevels[k];
      if (!parsed.allCards) {{
        for (const id of (parsed.ids || [])) {{
          if (validIds.has(id)) {{ selected.add(id); loaded++; }}
        }}
      }} else {{
        loaded = (parsed.ids || []).filter(id => validIds.has(id)).length;
      }}
      if (parsed.defaultPotential != null) defaultPotential = parsed.defaultPotential;
      if (parsed.defaultLevel != null) defaultLevel = parsed.defaultLevel;
      if (parsed.potentials) {{
        for (const [id, p] of Object.entries(parsed.potentials)) {{
          if (validIds.has(id)) cardPotentials[id] = p;
        }}
      }}
      if (parsed.levels) {{
        for (const [id, l] of Object.entries(parsed.levels)) {{
          if (validIds.has(id)) cardLevels[id] = l;
        }}
      }}
      if (parsed.levelEnabled != null) levelEnabled = parsed.levelEnabled;
      chkLevel.checked = levelEnabled;
      document.getElementById("lvGlobalBtns").style.display = levelEnabled ? "" : "none";
      savePersistence();
    }} else {{
      throw new Error("不明な形式");
    }}
    renderCards(); updateCounter();
    btn.textContent = loaded + "枚読込!"; setTimeout(() => btn.textContent = "ID貼付", 1500);
  }} catch {{
    btn.textContent = "形式エラー"; setTimeout(() => btn.textContent = "ID貼付", 1500);
  }}
}});
for (const btn of document.querySelectorAll(".btn-filter")) {{
  btn.addEventListener("click", () => {{
    document.querySelectorAll(".btn-filter").forEach(b => b.classList.remove("active"));
    btn.classList.add("active"); activeFilter = btn.dataset.filter; renderCards();
  }});
}}

document.getElementById("fixedLeader").addEventListener("change", () => updateCounter());

const workerBlob = URL.createObjectURL(new Blob([{json.dumps(solver_js)}], {{type:"application/javascript"}}));

let fabMode = "solve";

function expandResults() {{
  const wrapper = document.getElementById("resultsWrapper");
  const area = document.getElementById("resultsArea");
  const toggle = document.getElementById("resultsToggle");
  wrapper.style.display = "block";
  area.style.display = "block";
  toggle.textContent = "▼ 結果を折りたたむ";
}}

function collapseResults() {{
  const area = document.getElementById("resultsArea");
  const toggle = document.getElementById("resultsToggle");
  area.style.display = "none";
  toggle.textContent = "▶ 結果を表示";
}}

function setFabMode(mode) {{
  fabMode = mode;
  const fab = document.getElementById("fabSolve");
  fab.classList.remove("disabled");
  if (mode === "back") {{
    fab.textContent = "カード編集に戻る";
    fab.classList.add("fab-back");
  }} else {{
    fab.classList.remove("fab-back");
    const disabled = selected.size > 0 && selected.size < 5;
    fab.classList.toggle("disabled", disabled);
    const count = selected.size || "全";
    fab.textContent = `探索 (${{count}}枚)`;
  }}
}}

function updateFab() {{
  if (fabMode === "solve") setFabMode("solve");
}}

function doSolve() {{
  const btn = document.getElementById("btnSolve");
  const fab = document.getElementById("fabSolve");
  const pa = document.getElementById("progressArea");
  btn.disabled = true; btn.textContent = "計算中...";
  fab.classList.add("disabled"); fab.textContent = "計算中...";
  pa.classList.add("visible");
  document.getElementById("resultsArea").innerHTML = "";

  const owned = selected.size === 0 ? CARDS : CARDS.filter(c => selected.has(c.id));
  const fixedLeaderId = document.getElementById("fixedLeader").value || null;

  const potentials = {{}};
  const levels = {{}};
  for (const c of owned) {{
    potentials[c.id] = getCardPotential(c.id);
    levels[c.id] = getCardLevel(c.id);
  }}

  const w = new Worker(workerBlob);
  const selSong = document.getElementById("songSelect").value;
  w.postMessage({{ cards: owned, fixedLeaderId, topN: parseInt(document.getElementById("topN").value), potentials, levels, levelTables: LEVEL_TABLES, songLength: selSong ? parseFloat(selSong) : null }});

  w.onerror = function() {{
    w.terminate();
    btn.disabled = false; btn.textContent = "最強編成を探す";
    pa.classList.remove("visible");
    expandResults();
    document.getElementById("resultsArea").innerHTML = '<div class="empty-msg">計算中にエラーが発生しました。</div>';
    setFabMode("back");
  }};
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
      expandResults();
      renderResults(ev.data);
      saveToHistory(ev.data);
      document.getElementById("resultsWrapper").scrollIntoView({{ behavior: "smooth" }});
      setFabMode("back");
    }}
  }};
}}

document.getElementById("btnSolve").addEventListener("click", doSolve);

document.getElementById("fabSolve").addEventListener("click", () => {{
  if (fabMode === "back") {{
    collapseResults();
    setFabMode("solve");
    document.getElementById("cardArea").scrollIntoView({{ behavior: "smooth" }});
  }} else {{
    doSolve();
  }}
}});

document.getElementById("resultsToggle").addEventListener("click", () => {{
  const area = document.getElementById("resultsArea");
  if (area.style.display === "none") {{
    expandResults();
  }} else {{
    collapseResults();
  }}
}});

function saveToHistory(solveData) {{
  const ids = selected.size > 0 ? [...selected] : CARDS.map(c => c.id);
  const pots = {{}};
  const lvs = {{}};
  for (const id of ids) {{
    const p = getCardPotential(id);
    if (p !== defaultPotential) pots[id] = p;
    const l = getCardLevel(id);
    if (l !== defaultLevel) lvs[id] = l;
  }}
  const entry = {{
    ts: Date.now(),
    label: "",
    settings: {{
      topN: parseInt(document.getElementById("topN").value),
      fixedLeaderId: document.getElementById("fixedLeader").value || null,
      songLength: document.getElementById("songSelect").value ? parseFloat(document.getElementById("songSelect").value) : null,
    }},
    snapshot: {{ ids: [...selected], allCards: selected.size === 0, potentials: pots, levels: lvs, defaultPotential, defaultLevel, levelEnabled }},
    results: (solveData.results || []).slice(0, 3).map(r => ({{
      rank: r.rank, unit_score: r.unit_score, total_power: r.total_power,
      score_bonus: r.score_bonus, leader_id: r.leader_id, member_ids: r.member_ids,
    }})),
  }};
  let history = [];
  try {{ history = JSON.parse(localStorage.getItem("holodri_solve_history") || "[]"); }} catch {{}}
  history.unshift(entry);
  if (history.length > 20) history = history.slice(0, 20);
  localStorage.setItem("holodri_solve_history", JSON.stringify(history));
  renderHistory();
}}

function renderHistory() {{
  let history = [];
  try {{ history = JSON.parse(localStorage.getItem("holodri_solve_history") || "[]"); }} catch {{}}
  document.getElementById("historyCount").textContent = history.length;
  const area = document.getElementById("historyArea");
  if (!history.length) {{
    area.innerHTML = '<div style="color:#5a6e80;padding:8px;font-size:0.8rem">履歴なし</div>';
    return;
  }}
  let html = "";
  for (let i = 0; i < history.length; i++) {{
    const h = history[i];
    const dt = new Date(h.ts);
    const timeStr = `${{dt.getMonth()+1}}/${{dt.getDate()}} ${{dt.getHours()}}:${{String(dt.getMinutes()).padStart(2,"0")}}`;
    const top1 = h.results[0];
    const leaderCard = top1 ? cardMap[top1.leader_id] : null;
    const leaderName = leaderCard ? leaderCard.character : "?";
    const nCards = (h.snapshot?.ids || []).length;
    const labelVal = (h.label || "").replace(/"/g, "&quot;");
    html += `<div class="history-entry">
      <div class="h-header">
        <span>
          <span class="h-time">${{timeStr}}</span>
          <input class="h-label-input" placeholder="メモ" value="${{labelVal}}" data-idx="${{i}}">
        </span>
        <span class="h-score">${{top1 ? top1.unit_score.toLocaleString() : "-"}}</span>
      </div>
      <div style="color:#8899aa;font-size:0.72rem">
        #1 ${{leaderName}}リーダー / ${{nCards || "全"}}枚 / SB ${{top1 ? top1.score_bonus : "-"}}%
      </div>
      <div style="display:flex;gap:4px;margin-top:6px">
        <button class="h-btn" data-action="restore" data-idx="${{i}}">復元</button>
        <button class="h-btn" data-action="delete" data-idx="${{i}}">削除</button>
      </div>
    </div>`;
  }}
  area.innerHTML = html;
  area.querySelectorAll(".h-label-input").forEach(inp => {{
    inp.addEventListener("click", e => e.stopPropagation());
    inp.addEventListener("change", () => {{
      const idx = parseInt(inp.dataset.idx);
      let hist = [];
      try {{ hist = JSON.parse(localStorage.getItem("holodri_solve_history") || "[]"); }} catch {{}}
      if (hist[idx]) {{ hist[idx].label = inp.value; localStorage.setItem("holodri_solve_history", JSON.stringify(hist)); }}
    }});
  }});
  area.querySelectorAll(".h-btn").forEach(btn => {{
    btn.addEventListener("click", () => {{
      const idx = parseInt(btn.dataset.idx);
      if (btn.dataset.action === "delete") deleteHistory(idx);
      else if (btn.dataset.action === "restore") restoreFromHistory(idx);
    }});
  }});
}}

function deleteHistory(idx) {{
  let history = [];
  try {{ history = JSON.parse(localStorage.getItem("holodri_solve_history") || "[]"); }} catch {{}}
  history.splice(idx, 1);
  localStorage.setItem("holodri_solve_history", JSON.stringify(history));
  renderHistory();
}}

function restoreFromHistory(idx) {{
  let history = [];
  try {{ history = JSON.parse(localStorage.getItem("holodri_solve_history") || "[]"); }} catch {{}}
  const entry = history[idx];
  if (!entry || !entry.snapshot) return;
  const snap = entry.snapshot;
  const validIds = new Set(CARDS.map(c => c.id));

  for (const k of Object.keys(cardPotentials)) delete cardPotentials[k];
  for (const k of Object.keys(cardLevels)) delete cardLevels[k];

  selected.clear();
  const isAllCards = snap.allCards || (!(snap.ids || []).length && Object.keys(snap.potentials || {{}}).length > 0);
  const restoreIds = isAllCards ? CARDS.map(c => c.id) : (snap.ids || []);
  for (const id of restoreIds) {{
    if (!isAllCards && validIds.has(id)) selected.add(id);
  }}
  if (snap.defaultPotential != null) defaultPotential = snap.defaultPotential;
  if (snap.defaultLevel != null) defaultLevel = snap.defaultLevel;
  if (snap.levelEnabled != null) levelEnabled = snap.levelEnabled;
  for (const id of restoreIds) {{
    if (!validIds.has(id)) continue;
    if (snap.potentials?.[id] != null) cardPotentials[id] = snap.potentials[id];
    if (snap.levels?.[id] != null) cardLevels[id] = snap.levels[id];
  }}
  if (entry.settings?.topN != null) document.getElementById("topN").value = entry.settings.topN;
  document.getElementById("fixedLeader").value = entry.settings?.fixedLeaderId || "";
  const songSel = document.getElementById("songSelect");
  songSel.value = entry.settings?.songLength != null ? entry.settings.songLength : "";

  savePersistence();
  chkLevel.checked = levelEnabled;
  document.getElementById("lvGlobalBtns").style.display = levelEnabled ? "" : "none";
  renderCards();
  updateCounter();
}}

document.getElementById("historyToggle").addEventListener("click", () => {{
  const area = document.getElementById("historyArea");
  const toggle = document.getElementById("historyToggle");
  if (area.style.display === "none") {{
    area.style.display = "block";
    toggle.innerHTML = toggle.innerHTML.replace("▶", "▼");
  }} else {{
    area.style.display = "none";
    toggle.innerHTML = toggle.innerHTML.replace("▼", "▶");
  }}
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
      const pot = getCardPotential(mid);
      const lv = getCardLevel(mid);
      const s = getCardStats(card, pot, lv);
      html += `<div class="member-card${{isLeader?" is-leader":""}}">
        <span class="type-badge type-${{card.type}}" style="float:right;margin-top:2px">${{TYPE_LABELS[card.type]}}</span>
        <div class="m-name">${{card.character}}</div>
        <div class="m-card-name">${{card.card_name}}</div>
        <div class="m-stats">
          <span>P:${{s.performance.toLocaleString()}}</span>
          <span>T:${{s.technique.toLocaleString()}}</span>
          <span>S:${{s.sense.toLocaleString()}}</span>
        </div>
        <div class="m-pot-lv">${{pot}}凸${{levelEnabled ? ` Lv${{lv}}` : ''}}</div>
      </div>`;
    }}
    html += `</div></div>`;
  }}
  area.innerHTML = html;
}}

renderCards();
updateCounter();
renderHistory();
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
