package main

import (
	"math"
	"sort"
)

// generateActiveAttempts produces all Active Skill trigger attempts for a card
// within the song duration. Each attempt has a probability (without SP rate-up
// for now — that is added in Phase 3) and creates a window [start, start+duration).
func generateActiveAttempts(card *Card, cardIndex int, songDuration float64, rateUpAvg float64) []ActiveAttempt {
	cs := &card.CenterSkill
	if cs.Interval <= 0 || cs.Duration <= 0 {
		return nil
	}

	baseProb := 1.0
	if cs.ActivationProbabilityPermil != nil {
		baseProb = float64(*cs.ActivationProbabilityPermil) / 1000.0
	}
	boostedProb := math.Min(1.0, baseProb*(1.0+rateUpAvg))

	scoreUp := cs.ScoreUp
	// TODO: conditional score_up will be handled when typeCounts are passed in

	var attempts []ActiveAttempt
	for t := cs.Interval; t < songDuration; t += cs.Interval {
		end := math.Min(t+cs.Duration, songDuration)
		if end <= t {
			continue
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
	// Generate all attempts per card
	cardStates := make([]activeCardState, 0, 5)
	for i, card := range team {
		attempts := generateActiveAttempts(card, i, songDuration, rateUpAvg)
		cardStates = append(cardStates, activeCardState{
			scoreUp:  card.CenterSkill.ScoreUp,
			attempts: attempts,
		})
	}

	// Sort by scoreUp descending for E[max] calculation
	sort.Slice(cardStates, func(i, j int) bool {
		return cardStates[i].scoreUp > cardStates[j].scoreUp
	})

	// Generate uniform score events if none provided
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
