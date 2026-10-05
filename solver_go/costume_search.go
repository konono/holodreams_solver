package main

import (
	"encoding/json"
	"sort"
)

type costumeSearchKey struct {
	cardID string
	target int
}

type costumeSearchRequest struct {
	key       costumeSearchKey
	skill     CostumeSkill
	excludeID string
}

type costumeSearchResult struct {
	score     float64
	team      [5]string
	leaderIdx int
}

type compiledCostume struct {
	condition    *ConditionObj
	perfRate     float64
	techRate     float64
	senseRate    float64
	scoreSupport float64
}

func compileCostume(skill *CostumeSkill) compiledCostume {
	compiled := compiledCostume{condition: skill.Condition}
	for _, effect := range skill.Effects {
		value := effect.Value / 100.0
		switch effect.Stat {
		case "score_support":
			compiled.scoreSupport += value
		case "all":
			compiled.perfRate += value
			compiled.techRate += value
			compiled.senseRate += value
		case "performance":
			compiled.perfRate += value
		case "technique":
			compiled.techRate += value
		case "sense":
			compiled.senseRate += value
		}
	}
	return compiled
}

func (skill compiledCostume) score(base *BaseScores) float64 {
	if !checkCondition(skill.condition, base.TypeCounts, base.GroupCounts) {
		return base.BasePower * (1 + base.BaseBonus/100) * unitScoreK
	}
	contribution := base.TotalPerf*skill.perfRate + base.TotalTech*skill.techRate + base.TotalSense*skill.senseRate
	return (base.BasePower + contribution) * (1 + (base.BaseBonus+skill.scoreSupport*100*costumeSSRate)/100) * unitScoreK
}

// searchCostumeAlternatives visits every owned five-member team once, keeping
// only the best team for each requested costume and optional excluded card.
// It replaces the unbounded slice of BaseScores previously held in memory.
func searchCostumeAlternatives(cards []*Card, requests []costumeSearchRequest, statScale, baseline, songLength float64) map[costumeSearchKey]costumeSearchResult {
	results := make(map[costumeSearchKey]costumeSearchResult, len(requests))
	if len(requests) == 0 || len(cards) < 5 {
		return results
	}
	type query struct {
		key       costumeSearchKey
		excludeID string
		best      costumeSearchResult
	}
	type skillGroup struct {
		skill   compiledCostume
		queries []*query
	}
	groups := make([]skillGroup, 0, len(requests))
	groupBySignature := make(map[string]int, len(requests))
	queries := make([]query, 0, len(requests))
	for _, request := range requests {
		encoded, _ := json.Marshal(request.skill)
		signature := string(encoded)
		groupIndex, found := groupBySignature[signature]
		if !found {
			groupIndex = len(groups)
			groupBySignature[signature] = groupIndex
			groups = append(groups, skillGroup{skill: compileCostume(&request.skill)})
		}
		queries = append(queries, query{key: request.key, excludeID: request.excludeID})
		groups[groupIndex].queries = append(groups[groupIndex].queries, &queries[len(queries)-1])
	}

	charGroups := make(map[string][]*Card)
	for _, card := range cards {
		charGroups[card.Character] = append(charGroups[card.Character], card)
	}
	type charEntry struct {
		name     string
		maxTotal float64
	}
	entries := make([]charEntry, 0, len(charGroups))
	for name, group := range charGroups {
		maxTotal := 0.0
		for _, card := range group {
			if card.Total > maxTotal {
				maxTotal = card.Total
			}
		}
		entries = append(entries, charEntry{name, maxTotal})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].maxTotal != entries[j].maxTotal {
			return entries[i].maxTotal > entries[j].maxTotal
		}
		return entries[i].name < entries[j].name
	})
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.name
	}
	n := len(names)
	if n < 5 {
		return results
	}
	total := comb(n, 5)
	reportStage("costume", 0, total)
	progressStep := max(1, total/200)
	done := 0
	for a := 0; a < n-4; a++ {
		for b := a + 1; b < n-3; b++ {
			for c := b + 1; c < n-2; c++ {
				for d := c + 1; d < n-1; d++ {
					for e := d + 1; e < n; e++ {
						lists := [5][]*Card{charGroups[names[a]], charGroups[names[b]], charGroups[names[c]], charGroups[names[d]], charGroups[names[e]]}
						for _, c0 := range lists[0] {
							for _, c1 := range lists[1] {
								for _, c2 := range lists[2] {
									for _, c3 := range lists[3] {
										for _, c4 := range lists[4] {
											team := [5]*Card{c0, c1, c2, c3, c4}
											base := computeBaseScores(team, 0, statScale, baseline, songLength)
											teamIDs := [5]string{c0.ID, c1.ID, c2.ID, c3.ID, c4.ID}
											for gi := range groups {
												group := &groups[gi]
												score := group.skill.score(&base)
												for _, q := range group.queries {
													if score <= q.best.score {
														continue
													}
													if q.excludeID != "" {
														excluded := false
														for _, id := range teamIDs {
															if id == q.excludeID {
																excluded = true
																break
															}
														}
														if excluded {
															continue
														}
													}
													q.best = costumeSearchResult{score: score, team: teamIDs, leaderIdx: 0}
												}
											}
										}
									}
								}
							}
						}
						done++
						if done%progressStep == 0 {
							reportStage("costume", done, total)
						}
					}
				}
			}
		}
	}
	reportStage("costume", total, total)
	for _, q := range queries {
		results[q.key] = q.best
	}
	return results
}
