package main

import (
	"math"
	"math/rand"
	"testing"
)

func TestMonteCarloMatchesAnalytic_P1(t *testing.T) {
	// With p=1.0, MC should exactly match analytic (no randomness)
	card := makeTimelineCard(120, 25, 10, 1000)
	dummy := makeTimelineCard(0, 100, 0, 0)
	team := [5]*Card{card, dummy, dummy, dummy, dummy}

	timeline := &SongTimeline{
		Duration:      100,
		SpecialPoints: [5]float64{0, 0, 0, 0, 0},
	}

	events := []ScoreEvent{
		{Time: 30, ComboIndex: 50, Weight: 1},
		{Time: 40, ComboIndex: 150, Weight: 1},
		{Time: 55, ComboIndex: 250, Weight: 1},
		{Time: 90, ComboIndex: 500, Weight: 1},
	}

	analytic := EvaluateFullTimeline(team, 100000, 100, timeline, events, 0)
	mc := MonteCarloTimeline(team, 100000, 100, timeline, events, 0, 1000, rand.New(rand.NewSource(42)))

	// With p=1.0, all trials should give the same result
	if math.Abs(mc.MeanScore-analytic.LiveScoreIndex) > 1 {
		t.Errorf("MC mean=%f, analytic=%f", mc.MeanScore, analytic.LiveScoreIndex)
	}
	if mc.StdDev > 1 {
		t.Errorf("StdDev=%f, expected ~0 for p=1.0", mc.StdDev)
	}
}

func TestMonteCarloMatchesAnalytic_P05(t *testing.T) {
	// With p=0.5, MC mean should converge to analytic E[score]
	card := makeTimelineCard(120, 25, 10, 500)
	dummy := makeTimelineCard(0, 100, 0, 0)
	team := [5]*Card{card, dummy, dummy, dummy, dummy}

	timeline := &SongTimeline{
		Duration:      100,
		SpecialPoints: [5]float64{0, 0, 0, 0, 0},
	}

	events := make([]ScoreEvent, 100)
	for i := range events {
		events[i] = ScoreEvent{Time: float64(i), ComboIndex: i, Weight: 1}
	}

	analytic := EvaluateFullTimeline(team, 100000, 100, timeline, events, 0)
	mc := MonteCarloTimeline(team, 100000, 100, timeline, events, 0, 100000, rand.New(rand.NewSource(42)))

	// Should converge within 0.5%
	relDiff := math.Abs(mc.MeanScore-analytic.LiveScoreIndex) / analytic.LiveScoreIndex
	if relDiff > 0.005 {
		t.Errorf("MC mean=%f, analytic=%f, relDiff=%.4f%%", mc.MeanScore, analytic.LiveScoreIndex, relDiff*100)
	}
	t.Logf("MC mean=%.2f, analytic=%.2f, diff=%.4f%%", mc.MeanScore, analytic.LiveScoreIndex, relDiff*100)
}

func TestMonteCarloMatchesAnalytic_TwoCards(t *testing.T) {
	// Two cards with overlap: analytic E[max] should match MC
	cardA := makeTimelineCard(120, 25, 10, 600)
	cardB := makeTimelineCard(100, 30, 8, 500)
	dummy := makeTimelineCard(0, 100, 0, 0)
	team := [5]*Card{cardA, cardB, dummy, dummy, dummy}

	timeline := &SongTimeline{
		Duration:      120,
		SpecialPoints: [5]float64{0, 0, 0, 0, 0},
	}

	events := make([]ScoreEvent, 120)
	for i := range events {
		events[i] = ScoreEvent{Time: float64(i), ComboIndex: i, Weight: 1}
	}

	analytic := EvaluateFullTimeline(team, 100000, 120, timeline, events, 0)
	mc := MonteCarloTimeline(team, 100000, 120, timeline, events, 0, 100000, rand.New(rand.NewSource(42)))

	relDiff := math.Abs(mc.MeanScore-analytic.LiveScoreIndex) / analytic.LiveScoreIndex
	if relDiff > 0.005 {
		t.Errorf("MC mean=%f, analytic=%f, relDiff=%.4f%%", mc.MeanScore, analytic.LiveScoreIndex, relDiff*100)
	}
	t.Logf("MC mean=%.2f, analytic=%.2f, diff=%.4f%%", mc.MeanScore, analytic.LiveScoreIndex, relDiff*100)
}

func TestMonteCarloMatchesAnalytic_WithSP(t *testing.T) {
	// Active + SP Score Support + SP Rate Up
	card := makeTimelineCard(120, 25, 10, 450)
	spCard := makeTimelineCard(0, 100, 0, 0)
	spCard.SpecialSkill = &SpecialSkill{Duration: 15, ScoreSupport: 160, SkillRateUp: 45}
	dummy := makeTimelineCard(0, 100, 0, 0)
	team := [5]*Card{card, spCard, dummy, dummy, dummy}

	timeline := &SongTimeline{
		Duration:      100,
		SpecialPoints: [5]float64{0, 20, 0, 0, 0},
	}

	events := make([]ScoreEvent, 100)
	for i := range events {
		events[i] = ScoreEvent{Time: float64(i), ComboIndex: i, Weight: 1}
	}

	analytic := EvaluateFullTimeline(team, 100000, 100, timeline, events, 0)
	mc := MonteCarloTimeline(team, 100000, 100, timeline, events, 0, 100000, rand.New(rand.NewSource(42)))

	relDiff := math.Abs(mc.MeanScore-analytic.LiveScoreIndex) / analytic.LiveScoreIndex
	if relDiff > 0.005 {
		t.Errorf("MC mean=%f, analytic=%f, relDiff=%.4f%%", mc.MeanScore, analytic.LiveScoreIndex, relDiff*100)
	}
	t.Logf("MC mean=%.2f, analytic=%.2f, diff=%.4f%%", mc.MeanScore, analytic.LiveScoreIndex, relDiff*100)
}
