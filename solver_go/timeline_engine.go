package main

import (
	"math"
	"sort"
)

// generateSpecialWindows produces SpecialWindows from a team and song timeline.
// slot i's Special Skill is placed at SpecialPoints[i].
func generateSpecialWindows(team [5]*Card, timeline *SongTimeline) []SpecialWindow {
	if timeline == nil {
		return nil
	}
	var windows []SpecialWindow
	for i, card := range team {
		if card.SpecialSkill == nil {
			continue
		}
		sp := card.SpecialSkill
		start := timeline.SpecialPoints[i]
		end := math.Min(start+sp.Duration, timeline.Duration)
		if end <= start {
			continue
		}
		windows = append(windows, SpecialWindow{
			Start:              start,
			End:                end,
			ScoreSupport:       sp.ScoreSupport,
			SkillRateUp:        sp.SkillRateUp,
			SkillRateCondition: sp.SkillRateCondition,
			SlotIndex:          i,
		})
	}
	return windows
}

// rateUpAtTime returns the total skill rate up factor at time t from SP windows.
// Returns the sum of (SkillRateUp / 100.0) for all active SP windows.
// checkSPRateCondition checks if a SP Rate Up condition is met.
// AP assumption: life conditions are always satisfied.
func checkSPRateCondition(cond *string) bool {
	if cond == nil {
		return true
	}
	c := *cond
	if len(c) >= 4 && c[:4] == "life" {
		return true // AP前提: ライフ条件は常に成立
	}
	// TODO: combo_100等の条件はSP発動時点のcombo indexで判定すべきだが、
	// 現在のSP発動地点にはcombo情報がないため常に成立として扱う。
	// 実影響は小さい（AP前提なら曲序盤以外はcombo条件を満たす）。
	return true
}

func rateUpAtTime(spWindows []SpecialWindow, t float64) float64 {
	total := 0.0
	for i := range spWindows {
		w := &spWindows[i]
		if w.SkillRateUp > 0 && w.Start <= t && t < w.End && checkSPRateCondition(w.SkillRateCondition) {
			total += w.SkillRateUp / 100.0
		}
	}
	return total
}

// scoreSupportAtTime returns the total score support at time t from SP windows.
func scoreSupportAtTime(spWindows []SpecialWindow, t float64) float64 {
	total := 0.0
	for i := range spWindows {
		w := &spWindows[i]
		if w.ScoreSupport > 0 && w.Start <= t && t < w.End {
			total += w.ScoreSupport
		}
	}
	return total
}

// generateActiveAttempts produces all Active Skill trigger attempts for a card.
// When spWindows is provided, each attempt's probability is computed using the
// SP Rate Up active at that trigger time (multiplicative model).
// When spWindows is nil, rateUpAvg (time-averaged) is used as fallback.
func generateActiveAttempts(card *Card, cardIndex int, songDuration float64, rateUpAvg float64, spWindows []SpecialWindow, typeCounts map[string]int) []ActiveAttempt {
	cs := &card.CenterSkill
	if cs.Interval <= 0 || cs.Duration <= 0 {
		return nil
	}

	baseProb := 1.0
	if cs.ActivationProbabilityPermil != nil {
		baseProb = float64(*cs.ActivationProbabilityPermil) / 1000.0
	}

	scoreUp := cs.ScoreUp
	if cs.Condition != nil && typeCounts != nil && checkCenterTypeCondition(cs.Condition, typeCounts) {
		if cs.ConditionalScoreUp != nil {
			scoreUp = *cs.ConditionalScoreUp
		}
	}

	var attempts []ActiveAttempt
	for t := cs.Interval; t < songDuration; t += cs.Interval {
		end := math.Min(t+cs.Duration, songDuration)
		if end <= t {
			continue
		}

		var boostedProb float64
		if spWindows != nil {
			rateUp := rateUpAtTime(spWindows, t)
			boostedProb = math.Min(1.0, baseProb*(1.0+rateUp))
		} else {
			boostedProb = math.Min(1.0, baseProb*(1.0+rateUpAvg))
		}

		attempts = append(attempts, ActiveAttempt{
			Start:       t,
			End:         end,
			Probability: boostedProb,
			ScoreUp:     scoreUp,
			CardIndex:   cardIndex,
		})
	}
	return attempts
}

// activeCardState groups a card's attempts and score_up for E[max] calculation.
type activeCardState struct {
	scoreUp  float64
	attempts []ActiveAttempt
}

// activeProbAtTime returns the probability that card i is active at time t.
// p(active) = 1 - Π(1 - q_j) for all attempts j covering t.
func activeProbAtTime(state *activeCardState, t float64) float64 {
	probInactive := 1.0
	for i := range state.attempts {
		a := &state.attempts[i]
		if a.Start <= t && t < a.End {
			probInactive *= (1.0 - a.Probability)
		}
	}
	return 1.0 - probInactive
}

// expectedMaxActiveAtTime computes E[max(active_score_up)] at time t.
// Cards are sorted by scoreUp descending. For each card:
//   contribution = scoreUp × p(card active) × Π(1 - p(higher cards active))
func expectedMaxActiveAtTime(states []activeCardState, t float64) float64 {
	emax := 0.0
	probNoneHigher := 1.0
	for i := range states {
		p := activeProbAtTime(&states[i], t)
		emax += states[i].scoreUp * p * probNoneHigher
		probNoneHigher *= (1.0 - p)
	}
	return emax
}

// rawActiveExposureAtTime computes the sum of independent contributions
// (ignoring overlap) at time t. Used for overlap loss calculation.
func rawActiveExposureAtTime(states []activeCardState, t float64) float64 {
	raw := 0.0
	for i := range states {
		p := activeProbAtTime(&states[i], t)
		raw += states[i].scoreUp * p
	}
	return raw
}

// EvaluateActiveTimeline computes the Active Timeline for a team at given score events.
// Returns the average expected active score_up across all score events and the overlap loss.
// If scoreEvents is nil or empty, it generates uniform events every 0.1s.
func EvaluateActiveTimeline(team [5]*Card, songDuration float64, rateUpAvg float64, scoreEvents []ScoreEvent) (avgExpectedActive float64, overlapLoss float64) {
	return evaluateTimeline(team, songDuration, rateUpAvg, nil, scoreEvents)
}

// comboMultiplier returns the combo bonus multiplier for a given combo index.
// +1% per 100 combo, max +10%.
func comboMultiplier(comboIndex int) float64 {
	bonus := comboIndex / 100
	if bonus > 10 {
		bonus = 10
	}
	return 1.0 + float64(bonus)/100.0
}

// EvaluateFullTimeline computes the full Timeline Expected Score including
// Special Skill windows (rate-up and score support).
// alwaysOnSupport is the sum of costume + passive score support (always active),
// expressed as a percentage value (e.g., 20.0 for 20%).
// totalPower is the team's total power for LiveScoreIndex calculation.
func EvaluateFullTimeline(team [5]*Card, totalPower, songDuration float64, timeline *SongTimeline, scoreEvents []ScoreEvent, alwaysOnSupport float64) TimelineEvalResult {
	spWindows := generateSpecialWindows(team, timeline)
	cardStates := buildCardStates(team, songDuration, 0, spWindows)

	// Compute per-event metrics
	totalWeightedEmax := 0.0
	totalWeightedRaw := 0.0
	totalWeight := 0.0
	relativeSongScore := 0.0

	for i := range scoreEvents {
		ev := &scoreEvents[i]
		t := ev.Time
		w := ev.Weight
		if w <= 0 {
			w = 1.0
		}

		emax := expectedMaxActiveAtTime(cardStates, t)
		raw := rawActiveExposureAtTime(cardStates, t)

		totalWeightedEmax += emax * w
		totalWeightedRaw += raw * w
		totalWeight += w

		// Score Support at this time
		spSupport := 0.0
		if spWindows != nil {
			spSupport = scoreSupportAtTime(spWindows, t)
		}
		totalSupport := alwaysOnSupport + spSupport

		// Expected skill multiplier: (1 + active%/100) × (1 + support%/100)
		skillMultiplier := (1.0 + emax/100.0) * (1.0 + totalSupport/100.0)

		combo := comboMultiplier(ev.ComboIndex)

		relativeSongScore += w * combo * skillMultiplier
	}

	var avgActive, overlapLoss float64
	if totalWeight > 0 {
		avgActive = totalWeightedEmax / totalWeight
	}
	if totalWeightedRaw > 0 {
		overlapLoss = 1.0 - totalWeightedEmax/totalWeightedRaw
	}

	liveScoreIndex := totalPower * relativeSongScore

	// SP efficiency per slot
	var spEfficiency [5]float64
	if timeline != nil {
		for i, card := range team {
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
					totalActiveInSP += expectedMaxActiveAtTime(cardStates, t)
					count++
				}
			}
			if count > 0 {
				spEfficiency[i] = totalActiveInSP / float64(count)
			}
		}
	}

	return TimelineEvalResult{
		LiveScoreIndex:    liveScoreIndex,
		ExpectedActive:    avgActive,
		ActiveOverlapLoss: overlapLoss,
		SPEfficiency:      spEfficiency,
	}
}

func buildCardStates(team [5]*Card, songDuration, rateUpAvg float64, spWindows []SpecialWindow) []activeCardState {
	typeCounts := countTypes(team)
	cardStates := make([]activeCardState, 0, 5)
	for i, card := range team {
		attempts := generateActiveAttempts(card, i, songDuration, rateUpAvg, spWindows, typeCounts)
		scoreUp := card.CenterSkill.ScoreUp
		if card.CenterSkill.Condition != nil && checkCenterTypeCondition(card.CenterSkill.Condition, typeCounts) {
			if card.CenterSkill.ConditionalScoreUp != nil {
				scoreUp = *card.CenterSkill.ConditionalScoreUp
			}
		}
		cardStates = append(cardStates, activeCardState{
			scoreUp:  scoreUp,
			attempts: attempts,
		})
	}
	sort.Slice(cardStates, func(i, j int) bool {
		return cardStates[i].scoreUp > cardStates[j].scoreUp
	})
	return cardStates
}

func evaluateTimeline(team [5]*Card, songDuration, rateUpAvg float64, spWindows []SpecialWindow, scoreEvents []ScoreEvent) (avgExpectedActive float64, overlapLoss float64) {
	cardStates := buildCardStates(team, songDuration, rateUpAvg, spWindows)

	if len(scoreEvents) == 0 {
		n := int(songDuration * 10)
		scoreEvents = make([]ScoreEvent, n)
		for i := 0; i < n; i++ {
			scoreEvents[i] = ScoreEvent{
				Time:       float64(i) * 0.1,
				ComboIndex: i,
				Weight:     1.0,
			}
		}
	}

	totalWeightedEmax := 0.0
	totalWeightedRaw := 0.0
	totalWeight := 0.0

	for i := range scoreEvents {
		t := scoreEvents[i].Time
		w := scoreEvents[i].Weight
		if w <= 0 {
			w = 1.0
		}

		emax := expectedMaxActiveAtTime(cardStates, t)
		raw := rawActiveExposureAtTime(cardStates, t)

		totalWeightedEmax += emax * w
		totalWeightedRaw += raw * w
		totalWeight += w
	}

	if totalWeight > 0 {
		avgExpectedActive = totalWeightedEmax / totalWeight
	}

	if totalWeightedRaw > 0 {
		overlapLoss = 1.0 - totalWeightedEmax/totalWeightedRaw
	}

	return
}
