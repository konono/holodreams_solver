package main

import "math"

const (
	defaultCdReducePermil    = 40
	defaultActivationUpPermil = 80
)

// OptimizeBoardForTeam tries all 4^5 = 1024 combinations of cdReduce ON/OFF ×
// activationUp ON/OFF per member and returns the combination that maximizes
// LiveScoreIndex.
func OptimizeBoardForTeam(
	team [5]*Card,
	totalPower, songDuration float64,
	timeline *SongTimeline,
	scoreEvents []ScoreEvent,
	alwaysOnSupport float64,
	cdReducePermil, activationUpPermil int,
) *BoardOptResult {
	if len(scoreEvents) == 0 {
		return nil
	}

	// Baseline: all board effects OFF
	var noBoardConfigs [5]*BoardConfig
	baseResult := EvaluateFullTimelineWithBoard(team, totalPower, songDuration, timeline, scoreEvents, alwaysOnSupport, noBoardConfigs)

	bestLSI := baseResult.LiveScoreIndex
	bestLoss := baseResult.ActiveOverlapLoss
	var bestMask int

	// Enumerate all 4^5 = 1024 combinations.
	// For each member, 2 bits: bit0 = cdReduce, bit1 = activationUp
	for mask := 0; mask < 1024; mask++ {
		var configs [5]*BoardConfig
		for i := 0; i < 5; i++ {
			bits := (mask >> (i * 2)) & 3
			if bits == 0 {
				continue
			}
			cfg := &BoardConfig{}
			if bits&1 != 0 {
				cfg.CdReducePermil = cdReducePermil
			}
			if bits&2 != 0 {
				cfg.ActivationUpPermil = activationUpPermil
			}
			configs[i] = cfg
		}

		result := EvaluateFullTimelineWithBoard(team, totalPower, songDuration, timeline, scoreEvents, alwaysOnSupport, configs)
		if result.LiveScoreIndex > bestLSI {
			bestLSI = result.LiveScoreIndex
			bestLoss = result.ActiveOverlapLoss
			bestMask = mask
		}
	}

	// Build result
	var members [5]BoardMemberResult
	for i := 0; i < 5; i++ {
		bits := (bestMask >> (i * 2)) & 3
		members[i] = BoardMemberResult{
			CdReduce:     bits&1 != 0,
			ActivationUp: bits&2 != 0,
			CardID:       team[i].ID,
		}
	}

	return &BoardOptResult{
		Members:       members,
		BaselineLoss:  fixedFloat(baseResult.ActiveOverlapLoss * 100),
		OptimizedLoss: fixedFloat(bestLoss * 100),
		BaselineLSI:   int(math.Round(baseResult.LiveScoreIndex)),
		OptimizedLSI:  int(math.Round(bestLSI)),
	}
}
