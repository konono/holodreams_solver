package main

import "math"

// HolodoriDB SkillTreeNode.json / SkillTreeEffect.json derived constants:
//   cdReduce: 3 CARD-type nodes (B-013, B-020, B-031), each 40‰, grade 2
//   activationUp: 10 CARD-type nodes (20‰×3 + 30‰×6 + 60‰×1 = 300‰), grade 1-2
// All cards share the same board layout (tree-model-001~004).
const (
	cdReducePerNode         = 40
	cdReduceMaxNodes        = 3
	activationUpTotalPermil = 300
)

// OptimizeBoardForTeam optimizes per-member cdReduce node counts (0-3) to
// maximize LiveScoreIndex. activationUp is always fully unlocked (300‰).
// Search space: 4^5 = 1024 combinations.
func OptimizeBoardForTeam(
	team [5]*Card,
	totalPower, songDuration float64,
	timeline *SongTimeline,
	scoreEvents []ScoreEvent,
	alwaysOnSupport float64,
) *BoardOptResult {
	if len(scoreEvents) == 0 {
		return nil
	}

	// Baseline: activationUp only (cdReduce=0 for all)
	var baseConfigs [5]*BoardConfig
	for i := 0; i < 5; i++ {
		baseConfigs[i] = &BoardConfig{
			ActivationUpPermil: activationUpTotalPermil,
		}
	}
	baseResult := EvaluateFullTimelineWithBoard(team, totalPower, songDuration, timeline, scoreEvents, alwaysOnSupport, baseConfigs)

	bestLSI := baseResult.LiveScoreIndex
	bestLoss := baseResult.ActiveOverlapLoss
	var bestLevels [5]int

	// 4^5 = 1024 combinations of cdReduce levels (0-3 nodes per member)
	for combo := 0; combo < 1024; combo++ {
		var configs [5]*BoardConfig
		var levels [5]int
		for i := 0; i < 5; i++ {
			level := (combo >> (i * 2)) & 3
			levels[i] = level
			configs[i] = &BoardConfig{
				CdReducePermil:    level * cdReducePerNode,
				ActivationUpPermil: activationUpTotalPermil,
			}
		}

		result := EvaluateFullTimelineWithBoard(team, totalPower, songDuration, timeline, scoreEvents, alwaysOnSupport, configs)
		if result.LiveScoreIndex > bestLSI {
			bestLSI = result.LiveScoreIndex
			bestLoss = result.ActiveOverlapLoss
			bestLevels = levels
		}
	}

	var members [5]BoardMemberResult
	for i := 0; i < 5; i++ {
		members[i] = BoardMemberResult{
			CdReduceNodes: bestLevels[i],
			CardID:        team[i].ID,
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
