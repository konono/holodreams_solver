package main

import (
	"math"
	"sort"
)

// analyzePotential evaluates every reachable potential of each requested card.
// It deliberately keeps cards whose first copy has no effect: those are the
// cards the one-copy recommendation cannot discover.
func analyzePotential(ownedSpecs map[string]CardSpec, candidateIDs []string, allRawCards []CardRaw, maxCopies int, newCardLevel *int, statScale, baseline, songLength float64, fixedLeaderID, costumeOnlyLeaderID string, sweepCostumes bool, cf *CardsFile) []PotentialCard {
	outerProgress := progressCallback
	progressCallback = nil
	defer func() { progressCallback = outerProgress }()

	maxCopies = max(1, min(maxCopies, 5))
	rawCardMap := make(map[string]*CardRaw, len(allRawCards))
	for i := range allRawCards {
		rawCardMap[allRawCards[i].ID] = &allRawCards[i]
	}
	resolveSpecs := func(specs map[string]CardSpec) []*Card {
		cards := make([]*Card, 0, len(specs))
		for _, spec := range specs {
			if raw := rawCardMap[spec.ID]; raw != nil {
				card := resolveCard(raw, spec.Potential, spec.Level, cf)
				cards = append(cards, &card)
			}
		}
		return cards
	}

	level := 80
	if newCardLevel != nil {
		level = max(1, min(*newCardLevel, 80))
	}
	newLevel := &level
	baseCards := resolveSpecs(ownedSpecs)
	effectiveCostumeOnly := costumeOnlyLeaderID
	if fixedLeaderID != "" {
		effectiveCostumeOnly = ""
	}
	var overrideCostume *CostumeSkill
	if raw := rawCardMap[effectiveCostumeOnly]; raw != nil && len(raw.PotentialData) > 0 {
		potential := 0
		if spec, ok := ownedSpecs[effectiveCostumeOnly]; ok {
			potential = max(0, min(spec.Potential, len(raw.PotentialData)-1))
		}
		skill := raw.PotentialData[potential].CostumeSkill
		overrideCostume = &skill
	}
	useSweep := sweepCostumes && fixedLeaderID == "" && effectiveCostumeOnly == ""
	baseScore := 0
	if useSweep {
		base := solveSweepCostumes(baseCards, allRawCards, rawCardMap, 1, statScale, baseline, songLength, nil, cf)
		if len(base.Results) > 0 {
			baseScore = base.Results[0].UnitScore
		}
	} else {
		base := solve(baseCards, 1, statScale, baseline, songLength, fixedLeaderID, effectiveCostumeOnly, overrideCostume, nil)
		if len(base.Results) > 0 {
			baseScore = base.Results[0].UnitScore
		}
	}

	var ownedCostumes []CostumeEntry
	var ownedBases []precomputedBase
	if useSweep {
		for id, spec := range ownedSpecs {
			raw := rawCardMap[id]
			if raw == nil || len(raw.PotentialData) == 0 {
				continue
			}
			potential := max(0, min(spec.Potential, len(raw.PotentialData)-1))
			ownedCostumes = append(ownedCostumes, CostumeEntry{id, raw.PotentialData[potential].CostumeSkill})
		}
		ownedCostumes = pruneCostumes(ownedCostumes)
		ownedBases = precomputeOwnedBases(baseCards, statScale, baseline, songLength)
	}

	selected := map[string]bool{}
	for _, id := range candidateIDs {
		selected[id] = true
	}
	total := 0
	for i := range allRawCards {
		raw := &allRawCards[i]
		if len(selected) > 0 && !selected[raw.ID] {
			continue
		}
		current := -1
		if spec, ok := ownedSpecs[raw.ID]; ok {
			current = spec.Potential
		}
		total += max(0, min(maxCopies, len(raw.PotentialData)-1-current))
	}

	results := make([]PotentialCard, 0, len(allRawCards))
	done := 0
	for i := range allRawCards {
		raw := &allRawCards[i]
		if len(selected) > 0 && !selected[raw.ID] {
			continue
		}
		current := -1
		var currentPtr *int
		if spec, ok := ownedSpecs[raw.ID]; ok {
			current = spec.Potential
			copy := current
			currentPtr = &copy
		}
		available := max(0, min(maxCopies, len(raw.PotentialData)-1-current))
		if available == 0 {
			continue
		}
		entry := PotentialCard{
			CardID: raw.ID, CardName: raw.CardName, Character: raw.Character,
			CurrentPotential: currentPtr, Steps: make([]PotentialStep, 0, available),
		}
		var withoutCandidateBases []precomputedBase
		if useSweep && current >= 0 {
			without := make([]*Card, 0, len(baseCards)-1)
			for _, c := range baseCards {
				if c.ID != raw.ID {
					without = append(without, c)
				}
			}
			withoutCandidateBases = precomputeOwnedBases(without, statScale, baseline, songLength)
		}
		for copies := 1; copies <= available; copies++ {
			target := current + copies
			trialSpecs := make(map[string]CardSpec, len(ownedSpecs)+1)
			for id, spec := range ownedSpecs {
				trialSpecs[id] = spec
			}
			if current < 0 {
				trialSpecs[raw.ID] = CardSpec{ID: raw.ID, Potential: target, Level: newLevel}
			} else {
				spec := trialSpecs[raw.ID]
				spec.Potential = target
				trialSpecs[raw.ID] = spec
			}
			trialCards := resolveSpecs(trialSpecs)
			candidate := resolveCard(raw, target, trialSpecs[raw.ID].Level, cf)

			bestScore := baseScore
			var bestTeam RecommendBestTeam
			var candidateTeams []RecommendBestTeam
			role := "unused"
			consider := func(score float64, team [5]string, leaderIdx int, costumeID, candidateRole string) {
				rounded := int(math.Round(score))
				if rounded <= 0 || team[leaderIdx] == "" {
					return
				}
				teamResult := RecommendBestTeam{LeaderID: team[leaderIdx], MemberIDs: team[:]}
				if costumeID != "" {
					costume := costumeID
					teamResult.CostumeOnlyLeaderID = &costume
				}
				if candidateRole == "member" && team[leaderIdx] == raw.ID {
					candidateRole = "leader"
				}
				candidateTeams = append(candidateTeams, teamResult)
				if rounded > bestScore {
					bestScore = rounded
					bestTeam = teamResult
					role = candidateRole
				}
			}
			if useSweep {
				costumes := make([]CostumeEntry, 0, len(ownedCostumes)+1)
				for _, ce := range ownedCostumes {
					if ce.CardID != raw.ID {
						costumes = append(costumes, ce)
					}
				}
				costumes = append(costumes, CostumeEntry{raw.ID, raw.PotentialData[target].CostumeSkill})
				costumes = pruneCostumes(costumes)
				score, team, leaderIdx, costumeID := solveWithRequiredCardSweep(trialCards, &candidate, costumes, statScale, baseline, songLength)
				consider(score, team, leaderIdx, costumeID, "member")
				bases := ownedBases
				if current >= 0 {
					bases = withoutCandidateBases
				}
				skill := raw.PotentialData[target].CostumeSkill
				score, team, leaderIdx = solveForcedCostumeFromBases(bases, &skill)
				consider(score, team, leaderIdx, raw.ID, "costume")
			} else {
				score, team, leaderIdx := solveWithRequiredCard(trialCards, &candidate, statScale, baseline, songLength, fixedLeaderID, overrideCostume)
				consider(score.UnitScore, team, leaderIdx, effectiveCostumeOnly, "member")
			}
			step := PotentialStep{Copies: copies, TargetPotential: target, NewScore: bestScore, Delta: bestScore - baseScore, Role: role, BestTeam: bestTeam, CandidateTeams: candidateTeams}
			if entry.FirstUsefulCopies == 0 && step.Delta > 0 {
				entry.FirstUsefulCopies = copies
			}
			entry.Steps = append(entry.Steps, step)
			done++
			if outerProgress != nil {
				outerProgress(done, total)
			}
		}
		results = append(results, entry)
	}

	finalizePotentialProfiles(results)
	return results
}

func finalizePotentialProfiles(results []PotentialCard) {
	for i := range results {
		results[i].FirstUsefulCopies = 0
		for _, step := range results[i].Steps {
			if step.Delta > 0 {
				results[i].FirstUsefulCopies = step.Copies
				break
			}
		}
		for j := range results[i].Steps {
			cost := results[i].Steps[j].Copies
			bestOther := 0
			for k := range results {
				if i == k || cost > len(results[k].Steps) {
					continue
				}
				bestOther = max(bestOther, results[k].Steps[cost-1].Delta)
			}
			results[i].Steps[j].BestOtherDelta = bestOther
		}
	}
	sort.Slice(results, func(i, j int) bool {
		imax := results[i].Steps[len(results[i].Steps)-1].Delta
		jmax := results[j].Steps[len(results[j].Steps)-1].Delta
		if imax != jmax {
			return imax > jmax
		}
		if results[i].Steps[0].Delta != results[j].Steps[0].Delta {
			return results[i].Steps[0].Delta > results[j].Steps[0].Delta
		}
		return results[i].CardID < results[j].CardID
	})
}
