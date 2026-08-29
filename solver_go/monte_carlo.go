package main

import (
	"math"
	"math/rand"
	"sort"
)

// MonteCarloResult holds the output of a Monte Carlo simulation.
type MonteCarloResult struct {
	MeanScore    float64
	StdDev       float64
	Min          float64
	Max          float64
	P95          float64
	Trials       int
}

// MonteCarloTimeline runs a Monte Carlo simulation of the Timeline Engine.
// For each trial, it randomly determines which Active attempts succeed,
// then computes the actual score for that realization.
// This is used to validate the analytical expected value.
func MonteCarloTimeline(
	team [5]*Card,
	totalPower, songDuration float64,
	timeline *SongTimeline,
	scoreEvents []ScoreEvent,
	alwaysOnSupport float64,
	trials int,
	rng *rand.Rand,
) MonteCarloResult {
	spWindows := generateSpecialWindows(team, timeline)

	// Pre-generate all active attempts
	type cardAttempts struct {
		scoreUp  float64
		attempts []ActiveAttempt
	}
	var allCards []cardAttempts
	for i, card := range team {
		attempts := generateActiveAttempts(card, i, songDuration, 0, spWindows)
		allCards = append(allCards, cardAttempts{
			scoreUp:  card.CenterSkill.ScoreUp,
			attempts: attempts,
		})
	}

	scores := make([]float64, trials)

	for trial := 0; trial < trials; trial++ {
		// Determine which attempts succeed
		type activeWindow struct {
			start, end float64
			scoreUp    float64
		}
		var activeWindows []activeWindow

		for _, ca := range allCards {
			for _, a := range ca.attempts {
				if rng.Float64() < a.Probability {
					activeWindows = append(activeWindows, activeWindow{a.Start, a.End, a.ScoreUp})
				}
			}
		}

		// Compute score for this trial
		trialScore := 0.0
		for i := range scoreEvents {
			ev := &scoreEvents[i]
			t := ev.Time
			w := ev.Weight
			if w <= 0 {
				w = 1.0
			}

			// Find max active at this time
			maxActive := 0.0
			for _, aw := range activeWindows {
				if aw.start <= t && t < aw.end && aw.scoreUp > maxActive {
					maxActive = aw.scoreUp
				}
			}

			spSupport := 0.0
			if spWindows != nil {
				spSupport = scoreSupportAtTime(spWindows, t)
			}
			totalSupport := alwaysOnSupport + spSupport

			skillMult := (1.0 + maxActive/100.0) * (1.0 + totalSupport/100.0)
			combo := comboMultiplier(ev.ComboIndex)

			trialScore += w * combo * skillMult
		}

		scores[trial] = totalPower * trialScore
		activeWindows = activeWindows[:0]
	}

	// Statistics
	sort.Float64s(scores)
	sum := 0.0
	for _, s := range scores {
		sum += s
	}
	mean := sum / float64(trials)

	sumSqDev := 0.0
	for _, s := range scores {
		d := s - mean
		sumSqDev += d * d
	}
	stdDev := math.Sqrt(sumSqDev / float64(trials))

	p95Idx := int(float64(trials) * 0.95)
	if p95Idx >= trials {
		p95Idx = trials - 1
	}

	return MonteCarloResult{
		MeanScore: mean,
		StdDev:    stdDev,
		Min:       scores[0],
		Max:       scores[trials-1],
		P95:       scores[p95Idx],
		Trials:    trials,
	}
}
