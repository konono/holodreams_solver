package main

import (
	"math"
	"sort"
)

// attemptInfo holds the fixed geometry of an Active attempt (SP-independent).
type attemptInfo struct {
	start    float64
	end      float64
	baseProb float64
}

// precomputedCard holds per-card data shared across all 120 permutations.
type precomputedCard struct {
	origIdx  int
	scoreUp  float64
	attempts []attemptInfo
}

// rerankTeamAllPerms evaluates all 120 permutations of a single team efficiently.
//
// Strategy:
//  1. Pre-compute attempt timings and base probabilities (SP-independent) per card — 5 times total
//  2. Pre-compute per-event fixed values (weights, combo multipliers)
//  3. For each permutation:
//     a. Generate SP windows (lightweight: just 5 start/end calculations)
//     b. For each card's attempts, adjust probability only for attempts overlapping SP rate-up windows
//     c. Compute activeProbAtTime per event using adjusted attempts
//     d. Compute E[max], LSI, overlap loss using array lookups
func rerankTeamAllPerms(
	cards [5]*Card,
	totalPower float64,
	songDuration float64,
	timeline *SongTimeline,
	scoreEvents []ScoreEvent,
	alwaysOnSupport float64,
) []TimelineEvalResult {
	nEvents := len(scoreEvents)
	typeCounts := countTypes(cards)

	// --- Phase 1: Pre-compute card metadata and attempt geometry ---
	pcards := make([]precomputedCard, 0, 5)
	for i, card := range cards {
		cs := &card.CenterSkill
		su := cs.ScoreUp
		if cs.Condition != nil && checkCenterTypeCondition(cs.Condition, typeCounts) {
			if cs.ConditionalScoreUp != nil {
				su = *cs.ConditionalScoreUp
			}
		}

		var attempts []attemptInfo
		if cs.Interval > 0 && cs.Duration > 0 {
			baseProb := 1.0
			if cs.ActivationProbabilityPermil != nil {
				baseProb = float64(*cs.ActivationProbabilityPermil) / 1000.0
			}
			for t := cs.Interval; t < songDuration; t += cs.Interval {
				end := math.Min(t+cs.Duration, songDuration)
				if end <= t {
					continue
				}
				attempts = append(attempts, attemptInfo{
					start:    t,
					end:      end,
					baseProb: baseProb,
				})
			}
		}

		pcards = append(pcards, precomputedCard{
			origIdx:  i,
			scoreUp:  su,
			attempts: attempts,
		})
	}

	sort.Slice(pcards, func(i, j int) bool {
		return pcards[i].scoreUp > pcards[j].scoreUp
	})

	var scoreUps [5]float64
	for si := 0; si < 5; si++ {
		scoreUps[si] = pcards[si].scoreUp
	}

	// --- Phase 2: Pre-compute per-event fixed values ---
	evWeights := make([]float64, nEvents)
	evComboWeights := make([]float64, nEvents)
	for ei := range scoreEvents {
		w := scoreEvents[ei].Weight
		if w <= 0 {
			w = 1.0
		}
		evWeights[ei] = w
		evComboWeights[ei] = w * comboMultiplier(scoreEvents[ei].ComboIndex)
	}

	// --- Phase 3: Pre-compute base Active probabilities (no SP windows) ---
	baseProbs := make([][]float64, 5)
	for si := 0; si < 5; si++ {
		probs := make([]float64, nEvents)
		pc := &pcards[si]
		for ei := range scoreEvents {
			t := scoreEvents[ei].Time
			probInactive := 1.0
			for ai := range pc.attempts {
				a := &pc.attempts[ai]
				if a.start <= t && t < a.end {
					probInactive *= (1.0 - a.baseProb)
				}
			}
			probs[ei] = 1.0 - probInactive
		}
		baseProbs[si] = probs
	}

	// Reusable per-permutation probability buffer
	permProbs := make([][]float64, 5)
	for si := range permProbs {
		permProbs[si] = make([]float64, nEvents)
	}

	results := make([]TimelineEvalResult, 120)

	for pi, perm := range perms5 {
		var permTeam [5]*Card
		for i, p := range perm {
			permTeam[i] = cards[p]
		}

		spWindows := generateSpecialWindows(permTeam, timeline)

		hasRateUp := false
		for wi := range spWindows {
			if spWindows[wi].SkillRateUp > 0 && checkSPRateCondition(spWindows[wi].SkillRateCondition) {
				hasRateUp = true
				break
			}
		}

		if !hasRateUp {
			for si := 0; si < 5; si++ {
				copy(permProbs[si], baseProbs[si])
			}
		} else {
			for si := 0; si < 5; si++ {
				pc := &pcards[si]
				if len(pc.attempts) == 0 {
					copy(permProbs[si], baseProbs[si])
					continue
				}

				anyAffected := false
				for ai := range pc.attempts {
					if rateUpAtTime(spWindows, pc.attempts[ai].start) > 0 {
						anyAffected = true
						break
					}
				}

				if !anyAffected {
					copy(permProbs[si], baseProbs[si])
					continue
				}

				for ei := range scoreEvents {
					t := scoreEvents[ei].Time
					probInactive := 1.0
					for ai := range pc.attempts {
						a := &pc.attempts[ai]
						if a.start <= t && t < a.end {
							rateUp := rateUpAtTime(spWindows, a.start)
							boosted := math.Min(1.0, a.baseProb*(1.0+rateUp))
							probInactive *= (1.0 - boosted)
						}
					}
					permProbs[si][ei] = 1.0 - probInactive
				}
			}
		}

		// --- Phase 4: Compute LSI, E[max], overlap loss ---
		totalWeightedEmax := 0.0
		totalWeightedRaw := 0.0
		totalWeight := 0.0
		relativeSongScore := 0.0

		for ei := 0; ei < nEvents; ei++ {
			emax := 0.0
			raw := 0.0
			probNoneHigher := 1.0
			for si := 0; si < 5; si++ {
				p := permProbs[si][ei]
				emax += scoreUps[si] * p * probNoneHigher
				probNoneHigher *= (1.0 - p)
				raw += scoreUps[si] * p
			}

			w := evWeights[ei]
			totalWeightedEmax += emax * w
			totalWeightedRaw += raw * w
			totalWeight += w

			spSupport := scoreSupportAtTime(spWindows, scoreEvents[ei].Time)
			totalSupport := alwaysOnSupport + spSupport
			skillMultiplier := (1.0 + emax/100.0) * (1.0 + totalSupport/100.0)
			relativeSongScore += evComboWeights[ei] * skillMultiplier
		}

		var avgActive, overlapLoss float64
		if totalWeight > 0 {
			avgActive = totalWeightedEmax / totalWeight
		}
		if totalWeightedRaw > 0 {
			overlapLoss = 1.0 - totalWeightedEmax/totalWeightedRaw
		}

		liveScoreIndex := totalPower * relativeSongScore

		var spEfficiency [5]float64
		if timeline != nil {
			for i, card := range permTeam {
				if card.SpecialSkill == nil || card.SpecialSkill.ScoreSupport <= 0 {
					continue
				}
				start := timeline.SpecialPoints[i]
				end := math.Min(start+card.SpecialSkill.Duration, songDuration)
				if end <= start {
					continue
				}
				count := 0
				totalActiveInSP := 0.0
				for j := range scoreEvents {
					t := scoreEvents[j].Time
					if t >= start && t < end {
						em := 0.0
						pNone := 1.0
						for si := 0; si < 5; si++ {
							p := permProbs[si][j]
							em += scoreUps[si] * p * pNone
							pNone *= (1.0 - p)
						}
						totalActiveInSP += em
						count++
					}
				}
				if count > 0 {
					spEfficiency[i] = totalActiveInSP / float64(count)
				}
			}
		}

		results[pi] = TimelineEvalResult{
			LiveScoreIndex:    liveScoreIndex,
			ExpectedActive:    avgActive,
			ActiveOverlapLoss: overlapLoss,
			SPEfficiency:      spEfficiency,
		}
	}

	return results
}
