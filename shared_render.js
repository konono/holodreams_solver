// Globals expected from host: cardMap, levelTables, cardPotentials, defaultPotential,
// cardLevels, defaultLevel, levelEnabled, window.SONGS
const TYPE_LABELS = { happy: "Happy", pure: "Pure", cute: "Cute" };
const MAX_LEVEL = 80;

function getCardPotential(id) { return cardPotentials[id] ?? defaultPotential; }
function getCardLevel(id) {
  if (!levelEnabled) return 80;
  if (cardLevels[id] != null) return cardLevels[id];
  return defaultLevel;
}

function getCardStats(card, potential, level) {
  const pd = card.potential_data;
  if (!pd) return card.stats;
  const snap = pd[Math.min(potential, pd.length - 1)];
  if (!snap) return card.stats;
  const actualLv = Math.max(1, Math.min(level || MAX_LEVEL, MAX_LEVEL));
  if (actualLv === 80) return snap.ref_stats_lv80 || snap.stats;
  const table = levelTables[card.card_level_group_id];
  if (!table) return snap.ref_stats_lv80 || snap.stats;
  const baseValue = table[String(actualLv)];
  if (!baseValue) return snap.ref_stats_lv80 || snap.stats;
  const permil = card.permil || {};
  const bonus = snap.param_bonus_permil || 0;
  const mul = 1000 + bonus;
  function ceilDiv(a, b) { return Math.floor((a + b - 1) / b); }
  return {
    performance: ceilDiv(baseValue * (permil.performance || 333) * mul, 1000000),
    technique: ceilDiv(baseValue * (permil.technique || 333) * mul, 1000000),
    sense: ceilDiv(baseValue * (permil.sense || 334) * mul, 1000000),
  };
}

function renderRecommendations(data) {
  const area = document.getElementById("resultsArea");
  const recs = data.recommendations;
  if (!recs || !recs.length) {
    area.innerHTML = `<div class="empty-msg">現在の編成からスコアを上げる候補が見つかりませんでした。<br>ベーススコア: ${(data.base_score || 0).toLocaleString()}</div>`;
    return;
  }

  const maxDelta = recs[0]?.delta || 1;
  const pctUp = (d) => data.base_score > 0 ? (d / data.base_score * 100).toFixed(2) : "0.00";

  let html = "";
  if (data.warnings && data.warnings.length) {
    html += `<div style="background:#3d2a0f;color:#f0a040;padding:8px 12px;border-radius:6px;margin-bottom:8px;font-size:0.85rem">${data.warnings.join("<br>")}</div>`;
  }
  const ac = data.acquire_count || 1;
  html += `<div class="results-title">強化レコメンド Top ${recs.length}（+${ac}枚 / ベーススコア: <span style="color:#4f8cff">${data.base_score.toLocaleString()}</span>）</div>`;
  html += `<div style="font-size:0.78rem;color:#6b7f92;margin-bottom:12px">${ac === 1 ? '各カードを取得/凸した場合' : `単体で効果のある候補（最大20件）から${ac}枚の組み合わせを探索した結果`}のスコア上昇幅を比較しています</div>`;

  for (const r of recs) {
    const cards = r.cards || [r];
    const rankColors = { 1: "#ffd700", 2: "#c0c0c0", 3: "#cd7f32" };
    const rankColor = rankColors[r.rank] || "#4f8cff";
    const barWidth = Math.max(4, (r.delta / maxDelta) * 100);

    html += `<div class="result-card">
      <div style="display:flex;align-items:center;gap:12px;margin-bottom:6px">
        <span class="result-rank" style="color:${rankColor};font-size:1.2rem">#${r.rank}</span>
        <span style="font-size:1.3rem;font-weight:800;color:#40d080">+${r.delta.toLocaleString()}</span>
        <span style="font-size:0.85rem;color:#40d080;font-weight:600">(+${pctUp(r.delta)}%)</span>
        <span style="font-size:0.8rem;color:#8899aa">→ ${r.new_score.toLocaleString()}</span>
      </div>
      <div style="background:#1a2735;border-radius:4px;height:8px;overflow:hidden;margin-bottom:10px">
        <div style="background:linear-gradient(90deg,#40d080,#30a060);height:100%;width:${barWidth}%;border-radius:4px;transition:width 0.3s"></div>
      </div>`;

    for (const c of cards) {
      const card = cardMap[c.card_id];
      const actionLabel = c.action === "acquire"
        ? '<span style="background:#0f3d1a;color:#40d080;padding:2px 8px;border-radius:3px;font-size:0.72rem;font-weight:600">新規取得</span>'
        : `<span style="background:#3d2a0f;color:#f0a040;padding:2px 8px;border-radius:3px;font-size:0.72rem;font-weight:600">${c.current_potential}凸→${c.target_potential}凸</span>`;
      html += `<div style="display:flex;align-items:center;gap:8px;margin-bottom:4px;flex-wrap:wrap">
        ${actionLabel}
        <span style="font-weight:700;font-size:0.9rem">${c.character}</span>
        <span style="font-size:0.72rem;color:#7a8c9e">${c.card_name}</span>
        ${card ? `<span class="type-badge type-${card.type}">${TYPE_LABELS[card.type]}</span>` : ''}
      </div>`;
    }

    if (r.best_team) {
      const hasCostume = !!r.best_team.costume_only_leader_id;
      if (hasCostume) {
        const costumeCard = cardMap[r.best_team.costume_only_leader_id];
        const costumeLabel = costumeCard ? `${costumeCard.character}(${costumeCard.card_name})` : r.best_team.costume_only_leader_id;
        html += `<div style="color:#c0c060;font-size:0.75rem;margin:8px 0 4px">👗 ${costumeLabel}</div>`;
      }
      html += `<div style="font-size:0.72rem;color:#6b7f92;margin:${hasCostume ? '0' : '8px'} 0 4px">${hasCostume ? 'メンバー' : 'ベストチーム'}:</div>`;
      html += `<div class="result-members">`;
      for (const mid of r.best_team.member_ids) {
        const mc = cardMap[mid];
        if (!mc) continue;
        const isLeader = mid === r.best_team.leader_id && !hasCostume;
        const pot = getCardPotential(mid);
        const lv = getCardLevel(mid);
        const s = getCardStats(mc, pot, lv);
        html += `<div class="member-card${isLeader ? " is-leader" : ""}">
          <span class="type-badge type-${mc.type}" style="float:right;margin-top:2px">${TYPE_LABELS[mc.type]}</span>
          <div class="m-name">${mc.character}</div>
          <div class="m-card-name">${mc.card_name}</div>
          <div class="m-stats">
            <span>P:${s.performance.toLocaleString()}</span>
            <span>T:${s.technique.toLocaleString()}</span>
            <span>S:${s.sense.toLocaleString()}</span>
          </div>
          <div class="m-pot-lv">${pot}凸${levelEnabled ? ` Lv${lv}` : ''}</div>
        </div>`;
      }
      html += `</div>`;
    }
    html += `</div>`;
  }
  area.innerHTML = html;
}

function renderTimelineResults(data) {
  const tResults = data.timeline_results;
  const songId = document.getElementById("songSelect").value;
  const songName = window.SONGS?.[songId]?.name || songId;
  const diff = document.getElementById("diffSelect").value || "expert";

  let html = `<div class="results-title">ライブ期待スコア Top ${tResults.length}（${songName} ${diff}）</div>`;
  html += `<div style="font-size:0.72rem;color:#6b7f92;margin-bottom:10px">候補プール: ${data.candidate_pool} 編成 × 120 順列 → Timeline評価</div>`;
  html += `<details style="margin-bottom:12px;font-size:0.7rem;color:#6b7f92">
    <summary style="cursor:pointer;color:#4f8cff;user-select:none">結果の見かた</summary>
    <div style="background:#0d1520;border:1px solid #2a3a4a;border-radius:6px;padding:10px 12px;margin-top:6px;line-height:1.7">
      <div style="margin-bottom:6px;padding-bottom:6px;border-bottom:1px solid #2a3a4a">
        <div style="color:#4f8cff;font-weight:600;margin-bottom:2px">スキルの種類</div>
        <div><b style="color:#e0e6ed">Activeスキル</b> — 一定周期で自動発動し、スコアをアップさせるスキル。カードごとに周期（秒）・スコアアップ率・発動確率が異なります。5人のうち最も高いActiveのみが有効（最大値モデル）で、同時発動は無駄になります。</div>
        <div><b style="color:#e0e6ed">SPスキル（スペシャル）</b> — 曲中の決まったタイミングで1回発動し、スコアサポートや他スキルの発動率UPを一定時間付与します。スロット順で発動タイミングが決まるため、配置が重要です。</div>
        <div><b style="color:#e0e6ed">パッシブスキル</b> — 常時発動するステータスバフやスコアサポート効果。</div>
        <div><b style="color:#e0e6ed">衣装スキル</b> — リーダーの衣装が全メンバーに付与するステータスバフやスコアサポート効果。</div>
      </div>
      <div><b style="color:#e0e6ed">LSI（ライブ期待スコア指標）</b> — 総合力 × ノートごとのスキル倍率の合計。同じ曲での編成比較用の相対指標で、値が大きいほど高スコアが期待できます。</div>
      <div><b style="color:#e0e6ed">スキル効率</b> — スキルなし時と比べた倍率。2.0x ならスキルで2倍のスコア。Active・衣装SS・パッシブSB・SP効果の合計効果です。</div>
      <div><b style="color:#e0e6ed">Active期待値</b> — 5人のActiveスキルの期待スコアアップ値。発動タイミング・確率・重複を考慮した解析値（E[max]モデル）。</div>
      <div><b style="color:#e0e6ed">重複ロス</b> — 複数メンバーのActiveが同時発動して無駄になる割合。発動周期の異なるメンバーで編成すると改善できます。</div>
      <div style="margin-top:4px;border-top:1px solid #2a3a4a;padding-top:4px"><b style="color:#e0e6ed">各メンバーの Active</b> — スコア: 発動時のスコアアップ% / 周期: 発動間隔（秒）/ 確率: 1回あたりの発動確率。同じ周期のメンバーが多いと重複ロスが増えます。</div>
      <div><b style="color:#e0e6ed">SP配置効果</b> — このスロットのSP発動中に他メンバーのActiveがどれだけ乗るかの値。slot 1 は曲序盤でActive発動が少なく低くなりやすい。値が低くても高Activeやステータスなど別の強みで選ばれています。◎(80+) ○(50+) △(20+) ▽(20未満)。</div>
      <div><b style="color:#e0e6ed">ボード推奨</b> — 確率UPは全ノード開放推奨。頻度UPは周期の重複パターンに応じて最適なノード数（0〜3）を算出。各ノードで発動周期が4%短縮されます。</div>
    </div>
  </details>`;

  const top1LSI = tResults[0]?.live_score_index || 1;
  const fmtLSI = v => { const m = v / 1e6; return m >= 1000 ? `${(m/1000).toFixed(1)}B` : `${Math.round(m).toLocaleString()}M`; };

  for (const r of tResults) {
    const rankColors = { 1: "#ffd700", 2: "#c0c0c0", 3: "#cd7f32" };
    const rankColor = rankColors[r.rank] || "#4f8cff";
    const barPct = top1LSI > 0 ? (r.live_score_index / top1LSI * 100) : 100;

    let costumeLabel = "";
    if (r.costume_only_leader_id) {
      const clCard = cardMap[r.costume_only_leader_id];
      costumeLabel = clCard ? `👗 ${clCard.character}(${clCard.card_name})` : `👗 ${r.costume_only_leader_id}`;
    }

    html += `<div class="result-card">
      ${costumeLabel ? `<div style="color:#c0c060;font-size:0.75rem;margin-bottom:6px">${costumeLabel}</div>` : ''}
      <div style="display:flex;align-items:baseline;gap:12px;margin-bottom:4px">
        <span class="result-rank" style="color:${rankColor}">#${r.rank}</span>
        <span style="font-size:0.75rem;color:#e0e6ed;font-weight:700">LSI: ${fmtLSI(r.live_score_index)}</span>
        <span style="font-size:0.65rem;color:#5a6e80" title="ライブ期待スコア指標の実値">(${r.live_score_index.toLocaleString()})</span>
        <span style="font-size:0.75rem;color:#8899aa;margin-left:auto">${r.top1_pct}%</span>
      </div>
      <div style="height:4px;background:#1a2735;border-radius:2px;margin-bottom:10px;overflow:hidden">
        <div style="width:${barPct}%;height:100%;background:${rankColor};border-radius:2px;transition:width 0.3s"></div>
      </div>
      <div style="display:flex;gap:16px;font-size:0.72rem;color:#8899aa;margin-bottom:6px;flex-wrap:wrap">
        <span>総合力: <span style="color:#e0e6ed;font-weight:600">${r.total_power.toLocaleString()}</span></span>
        <span title="スキルなし時と比べた倍率（Active・衣装SS・パッシブSB・SP効果の合計）">スキル効率: <span style="color:#e0e6ed;font-weight:600">${r.skill_efficiency}x</span></span>
        <span style="color:#5a6e80">ユニットスコア: ${r.unit_score.toLocaleString()}</span>
      </div>
      <div style="display:flex;gap:6px;flex-wrap:wrap;font-size:0.65rem;margin-bottom:8px">
        <span style="background:#0d1520;border:1px solid #2a3a4a;border-radius:4px;padding:2px 8px" title="5人のActiveスキルの期待スコアアップ値 (E[max]モデル)">Active期待値 <span style="color:#40d080">+${r.expected_active}%</span></span>
        <span style="background:#0d1520;border:1px solid #2a3a4a;border-radius:4px;padding:2px 8px" title="衣装スキルによるスコアサポート効果">衣装SS <span style="color:#e0e6ed">${r.costume_sb_pct}%</span></span>
        <span style="background:#0d1520;border:1px solid #2a3a4a;border-radius:4px;padding:2px 8px" title="パッシブスキルによるスコアサポート効果">パッシブSB <span style="color:#e0e6ed">${r.passive_sb_pct}%</span></span>
        <span style="background:#0d1520;border:1px solid #2a3a4a;border-radius:4px;padding:2px 8px" title="スペシャルスキルの時間加重スコア効果">SP効果 <span style="color:#e0e6ed">${r.special_pct}%</span></span>
        <span style="background:#0d1520;border:1px solid #3a2a1a;border-radius:4px;padding:2px 8px" title="複数メンバーのActiveが同時発動して無駄になる割合。周期の異なるメンバーで編成すると改善できます">重複ロス <span style="color:#f0a050">${r.active_overlap_loss}%</span>${r.board_optimization ? ' <span style="color:#5a6e80">(ボード適用前)</span>' : ''}</span>
      </div>
`;

    if (r.board_optimization) {
      const bo = r.board_optimization;
      const lsiPct = bo.baseline_lsi > 0 ? ((bo.optimized_lsi - bo.baseline_lsi) / bo.baseline_lsi * 100) : 0;
      const lossDelta = bo.optimized_loss - bo.baseline_loss;
      const hasImprovement = lsiPct > 0.01;
      const lossColor = lossDelta < -0.05 ? '#40d080' : lossDelta > 0.05 ? '#f0a050' : '#e0e6ed';
      html += `<div style="background:#0d1520;border:1px solid ${hasImprovement ? '#1a4a3a' : '#2a3a4a'};border-radius:6px;padding:8px 10px;margin:8px 0;font-size:0.72rem">
        <div style="color:#8899aa;font-weight:600;margin-bottom:4px" title="確率UPは全ノード開放推奨。頻度UPは周期の重複パターンに応じて最適なノード数を算出">ボード推奨</div>
        <div style="display:flex;gap:12px;flex-wrap:wrap;align-items:center;margin-bottom:6px">
          <span>スコア改善: <span style="color:${hasImprovement ? '#40d080' : '#e0e6ed'};font-weight:600">${lsiPct > 0 ? '+' : ''}${lsiPct.toFixed(2)}%</span></span>
          <span style="color:#5a6e80">重複ロス: ${bo.baseline_loss}% → ${bo.optimized_loss}%
            <span style="color:${lossColor}">(${lossDelta > 0 ? '+' : ''}${lossDelta.toFixed(1)}pt)</span></span>
        </div>
        <div style="font-size:0.65rem;color:#6b7f92;margin-bottom:4px">確率UP: 全ノード開放推奨 (全員共通)</div>
        <div style="display:flex;gap:6px;flex-wrap:wrap;font-size:0.65rem">
          ${bo.members.map((m, idx) => {
            const c = cardMap[r.member_ids[idx]];
            const name = c ? c.character : r.member_ids[idx];
            const cdLabel = m.cd_reduce_nodes > 0
              ? `<span style="color:#60b0e0">頻度UP ×${m.cd_reduce_nodes}</span>`
              : `<span style="color:#5a6e80">頻度UP 不要</span>`;
            return `<span style="background:#0a1018;border:1px solid #2a3a4a;border-radius:4px;padding:3px 8px">${name}: ${cdLabel}</span>`;
          }).join('')}
        </div>
        <div style="display:flex;gap:12px;align-items:center;margin-top:6px;font-size:0.6rem;color:#5a6e80">
          <span>頻度UP: grade2 / 各5ポイント消費</span>
          <a href="https://holodori.best/tools/board-optimizer" target="_blank" rel="noopener" style="color:#4f8cff;text-decoration:none">holodori.best Board Optimizer →</a>
        </div>
      </div>`;
    }

    html += `<div class="result-members">`;

    for (let i = 0; i < r.member_ids.length; i++) {
      const mid = r.member_ids[i];
      const card = cardMap[mid];
      if (!card) continue;
      const pot = getCardPotential(mid);
      const lv = getCardLevel(mid);
      const s = getCardStats(card, pot, lv);
      const spEff = r.sp_efficiency?.[i];
      let spLabel = '';
      if (spEff != null && spEff > 0) {
        const spRating = spEff >= 80 ? '◎' : spEff >= 50 ? '○' : spEff >= 20 ? '△' : '▽';
        const spColor = spEff >= 80 ? '#40d080' : spEff >= 50 ? '#c0a040' : spEff >= 20 ? '#8899aa' : '#f06060';
        spLabel = `<div style="font-size:0.6rem;color:${spColor}" title="SP発動中に他メンバーのActiveがどれだけ乗るか。低くても高Activeやステータスなど別の強みで選ばれています">SP配置効果 <span style="font-weight:600">${spRating} ${spEff.toFixed(1)}</span></div>`;
      }
      let boardLabel = '';
      if (r.board_optimization) {
        const bm = r.board_optimization.members[i];
        if (bm && bm.cd_reduce_nodes > 0) {
          boardLabel = `<div style="color:#60b0e0;font-size:0.6rem" title="頻度UPボードノードを${bm.cd_reduce_nodes}個開放推奨 (各4%周期短縮)">頻度UP ×${bm.cd_reduce_nodes}</div>`;
        }
      }
      let activeLabel = '';
      const pd = card.potential_data;
      if (pd) {
        const cs = pd[Math.min(pot, pd.length - 1)]?.center_skill;
        if (cs && cs.score_up > 0) {
          const prob = cs.activation_probability_permil != null ? (cs.activation_probability_permil / 10).toFixed(0) + '%' : '確定';
          activeLabel = `<div style="display:flex;gap:4px;font-size:0.6rem;color:#5a6e80;margin-top:3px">
            <span style="flex:1;text-align:center"><span style="display:block;color:#5a6e80">スコア</span><span style="color:#40d080">+${cs.score_up}%</span></span>
            <span style="flex:1;text-align:center"><span style="display:block;color:#5a6e80">周期</span><span style="color:#8899aa">${cs.interval}秒</span></span>
            <span style="flex:1;text-align:center"><span style="display:block;color:#5a6e80">確率</span><span style="color:#8899aa">${prob}</span></span>
          </div>`;
        }
      }
      html += `<div class="member-card">
        <span class="type-badge type-${card.type}" style="float:right;margin-top:2px">${TYPE_LABELS[card.type]}</span>
        <div style="color:#5a6e80;font-size:0.6rem">slot ${i+1}</div>
        <div class="m-name">${card.character}</div>
        <div class="m-card-name">${card.card_name}</div>
        <div class="m-stats">
          <span>P:${s.performance.toLocaleString()}</span>
          <span>T:${s.technique.toLocaleString()}</span>
          <span>S:${s.sense.toLocaleString()}</span>
        </div>
        <div class="m-pot-lv">${pot}凸${levelEnabled ? ` Lv${lv}` : ''}</div>
        ${activeLabel}
        ${spLabel}
        ${boardLabel}
      </div>`;
    }
    html += `</div></div>`;
  }

  if (data.stability?.length) {
    const mainLSI = tResults[0]?.live_score_index || 0;
    html += `<div style="margin-top:16px;padding-top:12px;border-top:1px solid #2a3a4a">
      <div style="font-size:0.85rem;color:#e0e6ed;margin-bottom:8px;font-weight:600">曲別安定性</div>
      <div style="display:flex;gap:8px;flex-wrap:wrap">`;
    for (const s of data.stability) {
      const song = window.SONGS?.[s.music_id];
      const name = song?.name || s.music_id;
      const diff = mainLSI > 0 ? ((s.top_lsi - mainLSI) / mainLSI * 100).toFixed(1) : "0.0";
      const diffColor = diff > 0 ? "#40d080" : diff < 0 ? "#f06060" : "#8899aa";
      html += `<div style="background:#0f1923;border:1px solid #2a3a4a;border-radius:6px;padding:8px 10px;font-size:0.72rem;min-width:120px;max-width:160px">
        <div style="color:#8899aa;white-space:nowrap;overflow:hidden;text-overflow:ellipsis" title="${name}">${name}</div>
        <div style="color:#5a6e80;font-size:0.65rem">${s.duration}秒 / ${s.difficulty}</div>
        <div style="color:#e0e6ed;font-weight:600;margin-top:2px">LSI: ${s.top_lsi.toLocaleString()}</div>
        <div style="color:${diffColor};font-size:0.65rem">${diff > 0 ? '+' : ''}${diff}%</div>
      </div>`;
    }
    html += `</div></div>`;
  }

  if (data.legacy_results?.length) {
    html += `<details style="margin-top:16px"><summary style="color:#6b7f92;font-size:0.8rem;cursor:pointer">Legacy Unit Score 順位（参考）</summary>`;
    html += `<div style="margin-top:8px">`;
    for (const r of data.legacy_results) {
      const members = r.member_ids.map(id => cardMap[id]?.character || id).join(", ");
      html += `<div style="font-size:0.75rem;color:#8899aa;padding:4px 0;border-bottom:1px solid #1a2735">#${r.rank} ${r.unit_score.toLocaleString()} — ${members}</div>`;
    }
    html += `</div></details>`;
  }

  return html;
}

function renderResults(data) {
  const area = document.getElementById("resultsArea");
  const isTimeline = !!data.timeline_results;
  const results = isTimeline ? data.legacy_results : data.results;
  if ((!results || !results.length) && !isTimeline) {
    area.innerHTML = '<div class="empty-msg">結果が見つかりませんでした。</div>';
    return;
  }

  const bLabel = data.baseline !== 0 ? ` / H=${data.baseline?.toLocaleString() || ""}` : "";
  let html = "";
  if (data.warnings && data.warnings.length) {
    html += `<div style="background:#3d2a0f;color:#f0a040;padding:8px 12px;border-radius:6px;margin-bottom:8px;font-size:0.85rem">${data.warnings.join("<br>")}</div>`;
  }

  if (isTimeline) {
    html += renderTimelineResults(data);
    area.innerHTML = html;
    return;
  }

  html += `<div class="results-title">最強編成 Top ${results.length}（${(data.total_combinations ?? 0).toLocaleString()} 通り${bLabel}）</div>`;

  for (const r of results) {
    const rankColors = { 1: "#ffd700", 2: "#c0c0c0", 3: "#cd7f32" };
    const rankColor = rankColors[r.rank] || "";

    let costumeLabel = "";
    if (r.costume_only_leader_id) {
      const clCard = cardMap[r.costume_only_leader_id];
      costumeLabel = clCard ? `👗 ${clCard.character}(${clCard.card_name})` : `👗 ${r.costume_only_leader_id}`;
    }

    html += `<div class="result-card">
      ${costumeLabel ? `<div style="color:#c0c060;font-size:0.75rem;margin-bottom:6px">${costumeLabel}</div>` : ''}
      <div class="result-header">
        <span class="result-rank" ${rankColor ? `style="color:${rankColor}"` : ''}>#${r.rank}</span>
        <div class="result-scores">
          <span>ユニットスコア: <span class="main-score">${r.unit_score.toLocaleString()}</span></span>
          <span>総合力: ${r.total_power.toLocaleString()}</span>
          <span>SB: ${r.score_bonus}%</span>
          <span style="font-size:0.7rem;color:#5a6e80">(Active ${r.active_pct}%${r.costume_sb_pct > 0 ? ` / 衣装SS ${r.costume_sb_pct}%` : ''} / SS ${r.passive_sb_pct}% / SP ${r.special_pct}%)</span>
        </div>
      </div>
      <div class="result-members">`;
    for (const mid of r.member_ids) {
      const card = cardMap[mid]; if (!card) continue;
      const isLeader = mid === r.leader_id;
      const pot = getCardPotential(mid);
      const lv = getCardLevel(mid);
      const s = getCardStats(card, pot, lv);
      const showLeader = isLeader && !r.costume_only_leader_id;
      html += `<div class="member-card${showLeader ? " is-leader" : ""}">
        <span class="type-badge type-${card.type}" style="float:right;margin-top:2px">${TYPE_LABELS[card.type]}</span>
        <div class="m-name">${card.character}</div>
        <div class="m-card-name">${card.card_name}</div>
        <div class="m-stats">
          <span>P:${s.performance.toLocaleString()}</span>
          <span>T:${s.technique.toLocaleString()}</span>
          <span>S:${s.sense.toLocaleString()}</span>
        </div>
        <div class="m-pot-lv">${pot}凸${levelEnabled ? ` Lv${lv}` : ''}</div>
      </div>`;
    }
    html += `</div>`;
    if (r.stability) {
      const entries = Object.entries(r.stability).sort((a, b) => a[0] - b[0]);
      const baseScore = r.unit_score;
      html += `<div style="margin-top:10px;padding-top:8px;border-top:1px solid #2a3a4a">
        <div style="font-size:0.72rem;color:#6b7f92;margin-bottom:6px">曲長別スコア安定性:</div>
        <div style="display:flex;gap:6px;flex-wrap:wrap">`;
      for (const [len, score] of entries) {
        const diff = score - baseScore;
        const diffColor = diff > 0 ? "#40d080" : diff < 0 ? "#f06060" : "#8899aa";
        const diffText = diff > 0 ? `+${diff.toLocaleString()}` : diff.toLocaleString();
        html += `<div style="background:#0f1923;border:1px solid #2a3a4a;border-radius:4px;padding:4px 8px;font-size:0.7rem;text-align:center;min-width:70px">
          <div style="color:#8899aa">${len}秒</div>
          <div style="color:#e0e6ed;font-weight:600">${score.toLocaleString()}</div>
          <div style="color:${diffColor};font-size:0.65rem">${diffText}</div>
        </div>`;
      }
      html += `</div></div>`;
    }
    html += `</div>`;
  }
  area.innerHTML = html;
}
