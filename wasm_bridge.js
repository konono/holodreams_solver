/**
 * WASM ソルバーブリッジ
 * Go WASM バイナリを Web Worker 内で実行し、メインスレッドとメッセージでやりとりする
 */

// Worker 内で実行される
importScripts("wasm_exec.js");

let solverReady = false;

async function initWasm(cardsJSON) {
  const go = new Go();
  const result = await WebAssembly.instantiateStreaming(
    fetch("solver.wasm"),
    go.importObject
  );
  go.run(result.instance);

  // Initialize cards data
  const initResult = _solverInitCards(cardsJSON);
  if (initResult.error) {
    throw new Error("Failed to init cards: " + initResult.error);
  }
  solverReady = true;
  return initResult;
}

self._solverProgress = function (current, total) {
  self.postMessage({ type: "progress", current, total });
};

function callSolver(payload) {
  if (!solverReady) throw new Error("Solver not ready");
  const resultJSON = _solverCall(JSON.stringify(payload));
  return JSON.parse(resultJSON);
}

self.onmessage = async function (e) {
  const d = e.data;

  if (d.type === "init") {
    try {
      const result = await initWasm(d.cardsJSON);
      self.postMessage({ type: "init_done", count: result.count });
    } catch (err) {
      self.postMessage({ type: "error", message: err.message });
    }
    return;
  }

  if (d.type === "solve") {
    try {
      const payload = {
        action: "solve",
        cards: d.cards,
        top_n: d.topN || 10,
        stat_scale: d.statScale ?? 1.0,
        baseline: d.baseline ?? 0,
        sweep_costumes: d.sweepCostumes || false,
      };
      if (d.songLength) payload.song_length = d.songLength;
      if (d.fixedLeaderId) payload.fixed_leader_id = d.fixedLeaderId;
      if (d.costumeOnlyLeaderId)
        payload.costume_only_leader_id = d.costumeOnlyLeaderId;
      if (d.stabilityLengths) payload.stability_lengths = d.stabilityLengths;
      if (d.chartScore) payload.chart_score = d.chartScore;
      if (d.stabilityCharts) payload.stability_charts = d.stabilityCharts;

      const result = callSolver(payload);

      if (result.timeline_results) {
        self.postMessage({
          type: "done",
          timeline_results: result.timeline_results,
          legacy_results: result.legacy_results || [],
          candidate_pool: result.candidate_pool || 0,
          total_combinations: result.candidate_pool || 0,
          stability: result.stability || null,
        });
      } else {
        if (result.results) {
          for (const r of result.results) {
            if (r.stability) {
              const newStability = {};
              for (const [k, v] of Object.entries(r.stability)) {
                newStability[parseFloat(k)] = v;
              }
              r.stability = newStability;
            }
          }
        }
        self.postMessage({
          type: "done",
          results: result.results || [],
          total_combinations: result.total_combinations || 0,
        });
      }
    } catch (err) {
      self.postMessage({ type: "error", message: err.message });
    }
    return;
  }

  if (d.type === "recommend") {
    try {
      const payload = {
        action: "recommend",
        cards: d.cards,
        top_n: d.topN || 5,
        acquire_count: d.acquireCount || 1,
        stat_scale: d.statScale ?? 1.0,
        baseline: d.baseline ?? 0,
      };
      if (d.songLength) payload.song_length = d.songLength;
      if (d.fixedLeaderId) payload.fixed_leader_id = d.fixedLeaderId;
      if (d.costumeOnlyLeaderId)
        payload.costume_only_leader_id = d.costumeOnlyLeaderId;
      if (d.sweepCostumes) payload.sweep_costumes = true;

      const result = callSolver(payload);

      self.postMessage({
        type: "recommend_done",
        base_score: result.base_score || 0,
        acquire_count: result.acquire_count || 1,
        recommendations: result.recommendations || [],
      });
    } catch (err) {
      self.postMessage({ type: "error", message: err.message });
    }
    return;
  }
};
