package main

import (
	"encoding/json"
	"testing"
)

func TestDispatchSolveWithTimeline(t *testing.T) {
	cf, _ := loadTestData(t)

	chart := ChartScore{
		MusicID:    "test",
		Difficulty: "expert",
		Duration:   100,
		BPM:        163,
		TotalNotes: 10,
		BinSize:    0.5,
		SpecialPoints: []float64{13, 32, 48, 64, 80},
		Bins: []ScoreBin{
			{T: 15, N: 3, W: 3000, C: 3},
			{T: 30, N: 5, W: 5050, C: 8},
			{T: 50, N: 4, W: 4000, C: 12},
			{T: 65, N: 3, W: 3150, C: 15},
			{T: 80, N: 2, W: 2000, C: 17},
		},
	}

	var cards []string
	for i := 0; i < 10 && i < len(cf.Cards); i++ {
		cards = append(cards, cf.Cards[i].ID)
	}
	cardsJSON, _ := json.Marshal(cards)
	sl := 100.0

	input := CLIInput{
		Action:         "solve",
		Cards:          json.RawMessage(cardsJSON),
		TopN:           3,
		SongLength:     &sl,
		ChartScoreData: &chart,
	}

	result, err := dispatchAction(input, cf)
	if err != nil {
		t.Fatal(err)
	}

	tOutput, ok := result.(TimelineJSONOutput)
	if !ok {
		t.Fatalf("expected TimelineJSONOutput, got %T", result)
	}

	if len(tOutput.Timeline) == 0 {
		t.Fatal("no timeline results")
	}
	if len(tOutput.LegacyResults) == 0 {
		t.Fatal("no legacy results")
	}
	if tOutput.CandidatePool < 30 {
		t.Errorf("candidate pool too small: %d", tOutput.CandidatePool)
	}

	for _, r := range tOutput.Timeline {
		if r.LiveScoreIndex <= 0 {
			t.Errorf("rank %d: LSI should be positive, got %d", r.Rank, r.LiveScoreIndex)
		}
		if len(r.MemberIDs) != 5 {
			t.Errorf("rank %d: expected 5 members, got %d", r.Rank, len(r.MemberIDs))
		}
	}

	t.Logf("Timeline #1: LSI=%d unit=%d overlap=%.1f%%",
		tOutput.Timeline[0].LiveScoreIndex,
		tOutput.Timeline[0].UnitScore,
		float64(tOutput.Timeline[0].ActiveOverlapLoss))
}

func TestDispatchSolveWithoutTimeline(t *testing.T) {
	cf, _ := loadTestData(t)

	var cards []string
	for i := 0; i < 8 && i < len(cf.Cards); i++ {
		cards = append(cards, cf.Cards[i].ID)
	}
	cardsJSON, _ := json.Marshal(cards)

	input := CLIInput{
		Action: "solve",
		Cards:  json.RawMessage(cardsJSON),
		TopN:   3,
	}

	result, err := dispatchAction(input, cf)
	if err != nil {
		t.Fatal(err)
	}

	_, isTimeline := result.(TimelineJSONOutput)
	if isTimeline {
		t.Error("should not return TimelineJSONOutput without chart_score")
	}

	jOutput, ok := result.(JSONOutput)
	if !ok {
		t.Fatalf("expected JSONOutput, got %T", result)
	}
	if len(jOutput.Results) == 0 {
		t.Fatal("no results")
	}
}

func TestDispatchSolveWithStabilityCharts(t *testing.T) {
	cf, _ := loadTestData(t)

	chart := ChartScore{
		MusicID: "main", Difficulty: "expert", Duration: 100, BPM: 163, TotalNotes: 5, BinSize: 0.5,
		SpecialPoints: []float64{13, 32, 48, 64, 80},
		Bins:          []ScoreBin{{T: 25, N: 5, W: 5000, C: 5}},
	}
	stChart := ChartScore{
		MusicID: "alt", Difficulty: "expert", Duration: 130, BPM: 150, TotalNotes: 5, BinSize: 0.5,
		SpecialPoints: []float64{20, 40, 60, 80, 100},
		Bins:          []ScoreBin{{T: 30, N: 5, W: 5000, C: 5}},
	}

	var cards []string
	for i := 0; i < 8 && i < len(cf.Cards); i++ {
		cards = append(cards, cf.Cards[i].ID)
	}
	cardsJSON, _ := json.Marshal(cards)
	sl := 100.0

	input := CLIInput{
		Action:          "solve",
		Cards:           json.RawMessage(cardsJSON),
		TopN:            3,
		SongLength:      &sl,
		ChartScoreData:  &chart,
		StabilityCharts: []ChartScore{stChart},
	}

	result, err := dispatchAction(input, cf)
	if err != nil {
		t.Fatal(err)
	}

	tOutput := result.(TimelineJSONOutput)
	if len(tOutput.Stability) != 1 {
		t.Fatalf("expected 1 stability entry, got %d", len(tOutput.Stability))
	}
	if tOutput.Stability[0].MusicID != "alt" {
		t.Errorf("stability music_id = %s, want alt", tOutput.Stability[0].MusicID)
	}
	if tOutput.Stability[0].TopLSI <= 0 {
		t.Error("stability top_lsi should be positive")
	}
	t.Logf("Stability: %s LSI=%d", tOutput.Stability[0].MusicID, tOutput.Stability[0].TopLSI)
}
