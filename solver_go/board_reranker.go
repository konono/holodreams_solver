package main

import (
	"sort"
	"time"
)

type boundedOrder struct {
	team   [5]*Card
	ids    [5]string
	leader int
	bound  float64
}

type specialWindowKey struct {
	start, end, support, rate float64
	condition                 string
}

func specialOrderKey(team [5]*Card, timeline *SongTimeline) [5]specialWindowKey {
	var key [5]specialWindowKey
	for _, w := range generateSpecialWindows(team, timeline) {
		condition := ""
		if w.SkillRateCondition != nil {
			condition = *w.SkillRateCondition
		}
		key[w.SlotIndex] = specialWindowKey{w.Start, w.End, w.ScoreSupport, w.SkillRateUp, condition}
	}
	return key
}

// RerankBoardAware refines a small union of high Timeline and high board-bound
// candidates. Exhaustive mode refines every team in the legacy candidate pool.
// Within a refined team, every SP order is bounded and searched if it can
// improve the best exact result.
func RerankBoardAware(base []TimelineRerankResult, cardMap map[string]*Card, timeline *SongTimeline, events []ScoreEvent, finalN int, mode string) ([]TimelineRerankResult, BoardSearchStats) {
	stats := BoardSearchStats{TimelineCandidates: len(base)}
	if len(base) == 0 || finalN <= 0 {
		return nil, stats
	}
	if finalN > len(base) {
		finalN = len(base)
	}
	if mode == "fast" {
		return base[:finalN], stats
	}
	var boundElapsed, exactElapsed time.Duration

	// The best base order gives a useful and cheap board growth estimate.
	// Balanced selection is heuristic; exhaustive considers all legacy teams.
	selected := make([]bool, len(base))
	if mode == "exhaustive" {
		for i := range selected {
			selected[i] = true
		}
	} else {
		k := finalN * 2
		if k < 20 {
			k = 20
		}
		if k > len(base) {
			k = len(base)
		}
		for i := 0; i < k; i++ {
			selected[i] = true
		}
		bounds := make([]float64, len(base))
		started := time.Now()
		for i, r := range base {
			var team [5]*Card
			for j, id := range r.TeamIDs {
				team[j] = cardMap[id]
			}
			bounds[i] = BoardUpperBound(team, r.TotalPower, timeline.Duration, timeline, events, r.AlwaysOnSupport)
		}
		boundElapsed += time.Since(started)
		indices := make([]int, len(base))
		for i := range indices {
			indices[i] = i
		}
		sort.Slice(indices, func(i, j int) bool { return bounds[indices[i]] > bounds[indices[j]] })
		for _, i := range indices[:k] {
			selected[i] = true
		}
	}

	results := make([]TimelineRerankResult, len(base))
	copy(results, base)
	refine := func(i int) {
		r := results[i]
		var cards [5]*Card
		for j, id := range r.TeamIDs {
			cards[j] = cardMap[id]
		}
		orders := make([]boundedOrder, 0, len(perms5))
		seenWindows := make(map[[5]specialWindowKey]bool, len(perms5))
		started := time.Now()
		for _, perm := range perms5 {
			var order boundedOrder
			for j, p := range perm {
				order.team[j] = cards[p]
				order.ids[j] = r.TeamIDs[p]
				if p == r.LeaderIdx {
					order.leader = j
				}
			}
			key := specialOrderKey(order.team, timeline)
			if seenWindows[key] {
				stats.PermutationsEquivalent++
				continue
			}
			seenWindows[key] = true
			order.bound = BoardUpperBound(order.team, r.TotalPower, timeline.Duration, timeline, events, r.AlwaysOnSupport)
			orders = append(orders, order)
		}
		boundElapsed += time.Since(started)
		stats.PermutationsChecked += len(perms5)
		sort.Slice(orders, func(a, b int) bool { return orders[a].bound > orders[b].bound })
		best := 0.0
		for j, order := range orders {
			if order.bound <= best {
				stats.PermutationsPruned += len(orders) - j
				break
			}
			started = time.Now()
			opt := OptimizeBoardForTeam(order.team, r.TotalPower, timeline.Duration, timeline, events, r.AlwaysOnSupport)
			exactElapsed += time.Since(started)
			stats.ExactSearches++
			if opt != nil && opt.BestEval.LiveScoreIndex > best {
				best = opt.BestEval.LiveScoreIndex
				r.TeamIDs = order.ids
				r.LeaderIdx = order.leader
				r.LiveScoreIndex = best
				r.TimelineResult = opt.BestEval
				r.BoardOpt = opt
			}
		}
		results[i] = r
		selected[i] = true
		stats.RefinementTeams++
	}
	for i, yes := range selected {
		if yes {
			refine(i)
		}
	}

	// Any team that would enter the displayed Top N by its base score must be
	// refined before it is returned, so every displayed LSI is board optimized.
	for {
		sort.Slice(results, func(i, j int) bool { return rerankLess(results[i], results[j]) })
		need := -1
		for i := 0; i < finalN; i++ {
			if results[i].BoardOpt == nil {
				need = i
				break
			}
		}
		if need < 0 {
			break
		}
		// Results have been sorted; refine this result in place.
		refine(need)
	}
	sort.Slice(results, func(i, j int) bool { return rerankLess(results[i], results[j]) })
	stats.BoundMilliseconds = boundElapsed.Milliseconds()
	stats.ExactMilliseconds = exactElapsed.Milliseconds()
	return results[:finalN], stats
}
