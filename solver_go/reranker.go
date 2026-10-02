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
	CostumeSBPct        float64
	PassiveSBPct        float64
	SpecialPct          float64
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

	// A candidate can appear with several leaders. Keep only its best order
	// while searching, rather than retaining and sorting 120 results per input.
	bestByTeam := make(map[rerankKey]TimelineRerankResult, len(legacyResults))

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

		permResults := rerankTeamAllPerms(cards, eval.TotalPower, songDuration, timeline, scoreEvents, alwaysOnSupport)

		var sortedIDs [5]string
		copy(sortedIDs[:], lr.TeamIDs[:])
		sort.Strings(sortedIDs[:])
		key := rerankKey{members: sortedIDs, costume: lr.CostumeOnlyLeaderID}
		for pi, perm := range perms5 {
			if old, ok := bestByTeam[key]; ok && permResults[pi].LiveScoreIndex <= old.LiveScoreIndex {
				continue
			}
			var ids [5]string
			newLeaderIdx := 0
			for i, p := range perm {
				ids[i] = lr.TeamIDs[p]
				if p == lr.LeaderIdx {
					newLeaderIdx = i
				}
			}

			bestByTeam[key] = TimelineRerankResult{
				TeamIDs:             ids,
				LeaderIdx:           newLeaderIdx,
				UnitScore:           eval.UnitScore,
				TotalPower:          eval.TotalPower,
				LiveScoreIndex:      permResults[pi].LiveScoreIndex,
				CostumeOnlyLeaderID: lr.CostumeOnlyLeaderID,
				CostumeSBPct:        eval.CostumeSBPct,
				PassiveSBPct:        eval.PassiveSBPct,
				SpecialPct:          eval.SpecialPct,
				AlwaysOnSupport:     alwaysOnSupport,
				TimelineResult:      permResults[pi],
			}
		}
	}

	results := make([]TimelineRerankResult, 0, len(bestByTeam))
	for _, r := range bestByTeam {
		results = append(results, r)
	}
	sort.Slice(results, func(i, j int) bool { return rerankLess(results[i], results[j]) })
	if finalN < len(results) {
		results = results[:finalN]
	}
	return results
}

type rerankKey struct {
	members [5]string
	costume string
}

func rerankLess(a, b TimelineRerankResult) bool {
	if a.LiveScoreIndex != b.LiveScoreIndex {
		return a.LiveScoreIndex > b.LiveScoreIndex
	}
	for i := 0; i < 5; i++ {
		if a.TeamIDs[i] != b.TeamIDs[i] {
			return a.TeamIDs[i] < b.TeamIDs[i]
		}
	}
	if a.CostumeOnlyLeaderID != b.CostumeOnlyLeaderID {
		return a.CostumeOnlyLeaderID < b.CostumeOnlyLeaderID
	}
	return a.LeaderIdx < b.LeaderIdx
}
