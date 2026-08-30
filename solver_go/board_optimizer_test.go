package main

import (
	"math"
	"testing"
)

func intPtr(v int) *int { return &v }

func TestOptimizeBoardSameInterval(t *testing.T) {
	// Two cards with same 25s interval should benefit from cdReduce
	// to desynchronize their activation windows.
	prob := 550
	cards := [5]*Card{
		{ID: "a", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob}},
		{ID: "b", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 80, ActivationProbabilityPermil: &prob}},
		{ID: "c", CenterSkill: CenterSkill{Interval: 30, Duration: 5, ScoreUp: 60, ActivationProbabilityPermil: &prob}},
		{ID: "d", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 50, ActivationProbabilityPermil: &prob}},
		{ID: "e", CenterSkill: CenterSkill{Interval: 35, Duration: 5, ScoreUp: 40, ActivationProbabilityPermil: &prob}},
	}

	events := make([]ScoreEvent, 100)
	for i := range events {
		events[i] = ScoreEvent{Time: float64(i) * 1.0, ComboIndex: i, Weight: 1.0}
	}
	timeline := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 30, 50, 70, 90}}

	result := OptimizeBoardForTeam(cards, 100000, 100, timeline, events, 0)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.OptimizedLSI <= result.BaselineLSI {
		t.Errorf("expected optimized LSI > baseline: %d vs %d", result.OptimizedLSI, result.BaselineLSI)
	}
	// At least one member should have cdReduce > 0
	anyCdReduce := false
	for _, m := range result.Members {
		if m.CdReduceNodes > 0 {
			anyCdReduce = true
			break
		}
	}
	if !anyCdReduce {
		t.Error("expected at least one member with cdReduce > 0")
	}
}

func TestOptimizeBoardActivationUpAlwaysApplied(t *testing.T) {
	// Verify that the baseline already includes activationUp (300‰)
	prob := 460
	cards := [5]*Card{
		{ID: "a", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob}},
		{ID: "b", CenterSkill: CenterSkill{Interval: 30, Duration: 5, ScoreUp: 80, ActivationProbabilityPermil: &prob}},
		{ID: "c", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 60, ActivationProbabilityPermil: &prob}},
		{ID: "d", CenterSkill: CenterSkill{Interval: 35, Duration: 5, ScoreUp: 50, ActivationProbabilityPermil: &prob}},
		{ID: "e", CenterSkill: CenterSkill{Interval: 22, Duration: 5, ScoreUp: 40, ActivationProbabilityPermil: &prob}},
	}

	events := make([]ScoreEvent, 50)
	for i := range events {
		events[i] = ScoreEvent{Time: float64(i) * 2.0, ComboIndex: i, Weight: 1.0}
	}
	timeline := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 30, 50, 70, 90}}

	result := OptimizeBoardForTeam(cards, 100000, 100, timeline, events, 0)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Baseline should have activationUp applied (higher LSI than raw)
	var noBoard [5]*BoardConfig
	rawResult := EvaluateFullTimelineWithBoard(cards, 100000, 100, timeline, events, 0, noBoard)
	if float64(result.BaselineLSI) <= rawResult.LiveScoreIndex*0.99 {
		t.Errorf("baseline LSI should be higher than raw (no board) due to activationUp: baseline=%d, raw=%.0f",
			result.BaselineLSI, rawResult.LiveScoreIndex)
	}
}

func TestOptimizeBoardNilEvents(t *testing.T) {
	cards := [5]*Card{
		{ID: "a", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 100}},
		{ID: "b", CenterSkill: CenterSkill{Interval: 30, Duration: 5, ScoreUp: 80}},
		{ID: "c", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 60}},
		{ID: "d", CenterSkill: CenterSkill{Interval: 35, Duration: 5, ScoreUp: 50}},
		{ID: "e", CenterSkill: CenterSkill{Interval: 22, Duration: 5, ScoreUp: 40}},
	}
	timeline := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 30, 50, 70, 90}}

	result := OptimizeBoardForTeam(cards, 100000, 100, timeline, nil, 0)
	if result != nil {
		t.Error("expected nil result for empty events")
	}
}

// optimizeBoardBruteForce is the original O(1024 × events × attempts) reference
// implementation. Used only in tests to verify the fast path produces identical results.
func optimizeBoardBruteForce(
	team [5]*Card,
	totalPower, songDuration float64,
	timeline *SongTimeline,
	scoreEvents []ScoreEvent,
	alwaysOnSupport float64,
) *BoardOptResult {
	if len(scoreEvents) == 0 {
		return nil
	}
	cdPerNode := getCdReducePerNode()
	maxNodes := getCdReduceMaxNodes()
	actUpPermil := getActivationUpTotalPermil()

	var baseConfigs [5]*BoardConfig
	for i := 0; i < 5; i++ {
		baseConfigs[i] = &BoardConfig{ActivationUpPermil: actUpPermil}
	}
	baseResult := EvaluateFullTimelineWithBoard(team, totalPower, songDuration, timeline, scoreEvents, alwaysOnSupport, baseConfigs)

	bestLSI := baseResult.LiveScoreIndex
	bestLoss := baseResult.ActiveOverlapLoss
	var bestLevels [5]int

	totalCombos := 1
	for i := 0; i < 5; i++ {
		totalCombos *= (maxNodes + 1)
	}
	for combo := 0; combo < totalCombos; combo++ {
		var configs [5]*BoardConfig
		var levels [5]int
		rem := combo
		for i := 0; i < 5; i++ {
			level := rem % (maxNodes + 1)
			rem /= (maxNodes + 1)
			levels[i] = level
			configs[i] = &BoardConfig{
				CdReducePermil:     level * cdPerNode,
				ActivationUpPermil: actUpPermil,
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
		members[i] = BoardMemberResult{CdReduceNodes: bestLevels[i], CardID: team[i].ID}
	}
	return &BoardOptResult{
		Members:       members,
		BaselineLoss:  fixedFloat(baseResult.ActiveOverlapLoss * 100),
		OptimizedLoss: fixedFloat(bestLoss * 100),
		BaselineLSI:   int(math.Round(baseResult.LiveScoreIndex)),
		OptimizedLSI:  int(math.Round(bestLSI)),
	}
}

func boardTestTeams() []struct {
	name  string
	cards [5]*Card
} {
	prob550 := 550
	prob460 := 460
	prob800 := 800
	condType := "type:vocal>=3"
	condScoreUp := 120.0
	return []struct {
		name  string
		cards [5]*Card
	}{
		{
			name: "mixed_intervals",
			cards: [5]*Card{
				{ID: "a", Type: "vocal", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob550}},
				{ID: "b", Type: "vocal", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 80, ActivationProbabilityPermil: &prob550}},
				{ID: "c", Type: "vocal", CenterSkill: CenterSkill{Interval: 30, Duration: 5, ScoreUp: 60, ActivationProbabilityPermil: &prob550}},
				{ID: "d", Type: "dance", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 50, ActivationProbabilityPermil: &prob550}},
				{ID: "e", Type: "visual", CenterSkill: CenterSkill{Interval: 35, Duration: 5, ScoreUp: 40, ActivationProbabilityPermil: &prob550}},
			},
		},
		{
			name: "all_same_interval",
			cards: [5]*Card{
				{ID: "a", Type: "vocal", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob460}},
				{ID: "b", Type: "dance", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob460}},
				{ID: "c", Type: "visual", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob460}},
				{ID: "d", Type: "vocal", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob460}},
				{ID: "e", Type: "dance", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob460}},
			},
		},
		{
			name: "one_active_only",
			cards: [5]*Card{
				{ID: "a", Type: "vocal", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob800}},
				{ID: "b", Type: "vocal", CenterSkill: CenterSkill{}},
				{ID: "c", Type: "dance", CenterSkill: CenterSkill{}},
				{ID: "d", Type: "visual", CenterSkill: CenterSkill{}},
				{ID: "e", Type: "vocal", CenterSkill: CenterSkill{}},
			},
		},
		{
			name: "conditional_scoreup",
			cards: [5]*Card{
				{ID: "a", Type: "vocal", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 80, Condition: &condType, ConditionalScoreUp: &condScoreUp, ActivationProbabilityPermil: &prob550}},
				{ID: "b", Type: "vocal", CenterSkill: CenterSkill{Interval: 30, Duration: 5, ScoreUp: 70, ActivationProbabilityPermil: &prob550}},
				{ID: "c", Type: "vocal", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 60, ActivationProbabilityPermil: &prob550}},
				{ID: "d", Type: "dance", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 50, ActivationProbabilityPermil: &prob550}},
				{ID: "e", Type: "visual", CenterSkill: CenterSkill{Interval: 35, Duration: 5, ScoreUp: 40, ActivationProbabilityPermil: &prob550}},
			},
		},
	}
}

func TestOptimizeBoardFastMatchesBrute(t *testing.T) {
	events := make([]ScoreEvent, 200)
	for i := range events {
		events[i] = ScoreEvent{Time: float64(i) * 0.5, ComboIndex: i * 3, Weight: 1.0}
	}
	timeline := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 30, 50, 70, 90}}

	for _, tc := range boardTestTeams() {
		t.Run(tc.name, func(t *testing.T) {
			fast := OptimizeBoardForTeam(tc.cards, 100000, 100, timeline, events, 5.0)
			brute := optimizeBoardBruteForce(tc.cards, 100000, 100, timeline, events, 5.0)

			if fast == nil || brute == nil {
				if fast != brute {
					t.Fatalf("nil mismatch: fast=%v brute=%v", fast, brute)
				}
				return
			}

			if fast.OptimizedLSI != brute.OptimizedLSI {
				t.Errorf("OptimizedLSI mismatch: fast=%d brute=%d", fast.OptimizedLSI, brute.OptimizedLSI)
			}
			if fast.BaselineLSI != brute.BaselineLSI {
				t.Errorf("BaselineLSI mismatch: fast=%d brute=%d", fast.BaselineLSI, brute.BaselineLSI)
			}
			if fast.Members != brute.Members {
				t.Errorf("Members mismatch:\n  fast=%+v\n  brute=%+v", fast.Members, brute.Members)
			}
		})
	}
}

func TestOptimizeBoardFastManyEvents(t *testing.T) {
	// Simulate a dense chart with 500 events
	events := make([]ScoreEvent, 500)
	for i := range events {
		events[i] = ScoreEvent{Time: float64(i) * 0.24, ComboIndex: i * 2, Weight: float64(1 + i%3)}
	}
	sp := [5]float64{15, 35, 55, 75, 95}
	timeline := &SongTimeline{Duration: 120, SpecialPoints: sp}

	for _, tc := range boardTestTeams() {
		t.Run(tc.name, func(t *testing.T) {
			fast := OptimizeBoardForTeam(tc.cards, 150000, 120, timeline, events, 10.0)
			brute := optimizeBoardBruteForce(tc.cards, 150000, 120, timeline, events, 10.0)

			if fast.OptimizedLSI != brute.OptimizedLSI {
				t.Errorf("OptimizedLSI mismatch: fast=%d brute=%d", fast.OptimizedLSI, brute.OptimizedLSI)
			}
			if fast.Members != brute.Members {
				t.Errorf("Members mismatch:\n  fast=%+v\n  brute=%+v", fast.Members, brute.Members)
			}
		})
	}
}

func BenchmarkOptimizeBoardOld(b *testing.B) {
	prob := 550
	cards := [5]*Card{
		{ID: "a", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob}},
		{ID: "b", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 80, ActivationProbabilityPermil: &prob}},
		{ID: "c", CenterSkill: CenterSkill{Interval: 30, Duration: 5, ScoreUp: 60, ActivationProbabilityPermil: &prob}},
		{ID: "d", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 50, ActivationProbabilityPermil: &prob}},
		{ID: "e", CenterSkill: CenterSkill{Interval: 35, Duration: 5, ScoreUp: 40, ActivationProbabilityPermil: &prob}},
	}
	events := make([]ScoreEvent, 200)
	for i := range events {
		events[i] = ScoreEvent{Time: float64(i) * 0.5, ComboIndex: i * 3, Weight: 1.0}
	}
	timeline := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 30, 50, 70, 90}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		optimizeBoardBruteForce(cards, 100000, 100, timeline, events, 0)
	}
}

func BenchmarkOptimizeBoardFast(b *testing.B) {
	prob := 550
	cards := [5]*Card{
		{ID: "a", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob}},
		{ID: "b", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 80, ActivationProbabilityPermil: &prob}},
		{ID: "c", CenterSkill: CenterSkill{Interval: 30, Duration: 5, ScoreUp: 60, ActivationProbabilityPermil: &prob}},
		{ID: "d", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 50, ActivationProbabilityPermil: &prob}},
		{ID: "e", CenterSkill: CenterSkill{Interval: 35, Duration: 5, ScoreUp: 40, ActivationProbabilityPermil: &prob}},
	}
	events := make([]ScoreEvent, 200)
	for i := range events {
		events[i] = ScoreEvent{Time: float64(i) * 0.5, ComboIndex: i * 3, Weight: 1.0}
	}
	timeline := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 30, 50, 70, 90}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		OptimizeBoardForTeam(cards, 100000, 100, timeline, events, 0)
	}
}
