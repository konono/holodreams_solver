package main

import (
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
