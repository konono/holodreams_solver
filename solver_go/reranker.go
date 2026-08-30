package main

import (
	"sort"
)

// perms5 holds all 120 permutations of [0,1,2,3,4].
var perms5 [120][5]int

func init() {
	idx := 0
	var gen func([]int, int)
	gen = func(arr []int, k int) {
		if k == 1 {
			copy(perms5[idx][:], arr)
			idx++
			return
		}
		for i := 0; i < k; i++ {
			gen(arr, k-1)
			if k%2 == 0 {
				arr[i], arr[k-1] = arr[k-1], arr[i]
			} else {
				arr[0], arr[k-1] = arr[k-1], arr[0]
			}
		}
	}
	gen([]int{0, 1, 2, 3, 4}, 5)
}

// TimelineRerankResult holds one reranked team result.
type TimelineRerankResult struct {
	TeamIDs             [5]string
	LeaderIdx           int
	UnitScore           float64
	TotalPower          float64
	LiveScoreIndex      float64
	CostumeOnlyLeaderID string
	AlwaysOnSupport     float64
	TimelineResult      TimelineEvalResult
	BoardOpt            *BoardOptResult
}

// RerankTopN takes legacy top results, evaluates each with the Timeline Engine
// across all 5! member orderings, and returns the top finalN sorted by LiveScoreIndex.
//
// For each team, it re-evaluates with evaluateTeam to obtain the always-on
// Score Support (costume SS + passive SB) that feeds into the Timeline multiplier.
func RerankTopN(
	legacyResults []SolveResult,
	cardMap map[string]*Card,
	timeline *SongTimeline,
	scoreEvents []ScoreEvent,
	statScale, baseline, songLength float64,
	overrideCostumeSkill *CostumeSkill,
	finalN int,
) []TimelineRerankResult {
	if timeline == nil || len(scoreEvents) == 0 {
		return nil
	}

	songDuration := timeline.Duration

	var allResults []TimelineRerankResult

	for _, lr := range legacyResults {
		var cards [5]*Card
		for i, id := range lr.TeamIDs {
			cards[i] = cardMap[id]
		}

		// Resolve costume override for costume-only leaders
		var costumeSkill *CostumeSkill
		if overrideCostumeSkill != nil {
			costumeSkill = overrideCostumeSkill
		} else if lr.CostumeOnlyLeaderID != "" {
			if c, ok := cardMap[lr.CostumeOnlyLeaderID]; ok {
				costumeSkill = &c.CostumeSkill
			}
		}

		eval := evaluateTeam(cards, lr.LeaderIdx, statScale, baseline, songLength, costumeSkill)
		alwaysOnSupport := eval.CostumeSSVal*100*costumeSSRate + eval.SupportSSVal*100*supportSSRate

		for _, perm := range perms5 {
			var team [5]*Card
			var ids [5]string
			for i, p := range perm {
				team[i] = cards[p]
				ids[i] = lr.TeamIDs[p]
			}

			result := EvaluateFullTimeline(team, eval.TotalPower, songDuration, timeline, scoreEvents, alwaysOnSupport)

			allResults = append(allResults, TimelineRerankResult{
				TeamIDs:             ids,
				LeaderIdx:           lr.LeaderIdx,
				UnitScore:           eval.UnitScore,
				TotalPower:          eval.TotalPower,
				LiveScoreIndex:      result.LiveScoreIndex,
				CostumeOnlyLeaderID: lr.CostumeOnlyLeaderID,
				AlwaysOnSupport:     alwaysOnSupport,
				TimelineResult:      result,
			})
		}
	}

	// Sort by LiveScoreIndex descending
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].LiveScoreIndex > allResults[j].LiveScoreIndex
	})

	// Deduplicate: keep only the best permutation per (team set + costume)
	type dedupKey struct {
		members [5]string
		costume string
	}
	seen := map[dedupKey]bool{}
	var deduped []TimelineRerankResult
	for _, r := range allResults {
		var sorted5 [5]string
		copy(sorted5[:], r.TeamIDs[:])
		sort.Strings(sorted5[:])
		dk := dedupKey{members: sorted5, costume: r.CostumeOnlyLeaderID}
		if seen[dk] {
			continue
		}
		seen[dk] = true
		deduped = append(deduped, r)
		if len(deduped) >= finalN {
			break
		}
	}

	return deduped
}
