package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

// TestTimelineRegressionGolden verifies that Timeline evaluation produces
// stable results for a fixed input. If the scoring model changes intentionally,
// update the golden values.
func TestTimelineRegressionGolden(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Skip("cards.json not available")
	}

	chartData, err := loadChartScore("../data/chart_scores.json", "m0001_expert")
	if err != nil {
		t.Skip("chart_scores.json not available")
	}

	cardIDs := []string{
		"tokino_sora_5", "aki_rosenthal_5", "natsuiro_matsuri_5",
		"shirakami_fubuki_5", "akai_haato_5", "nakiri_ayame_5",
		"usada_pekora_5", "oozora_subaru_5", "shishiro_botan_5",
		"nekomata_okayu_5",
	}

	cardsJSON, _ := json.Marshal(cardIDs)
	sl := 100.0

	input := CLIInput{
		Action:         "solve",
		Cards:          json.RawMessage(cardsJSON),
		TopN:           3,
		SongLength:     &sl,
		ChartScoreData: chartData,
	}

	result, err := dispatchAction(input, cf)
	if err != nil {
		t.Fatal(err)
	}

	tOutput, ok := result.(TimelineJSONOutput)
	if !ok {
		t.Fatalf("expected TimelineJSONOutput, got %T", result)
	}

	if len(tOutput.Timeline) < 3 {
		t.Fatalf("expected >=3 timeline results, got %d", len(tOutput.Timeline))
	}

	type golden struct {
		lsi     int
		power   int
		unit    int
		overlap float64
	}
	// Golden values from fixed input — update only when scoring model changes intentionally
	expected := []golden{
		{253731214980, 199494, 908598, 29.7},
		{251793121695, 200519, 919010, 21.2},
		{250970726345, 205456, 934027, 32.6},
	}

	for i, exp := range expected {
		r := tOutput.Timeline[i]
		if math.Abs(float64(r.LiveScoreIndex-exp.lsi)) > float64(exp.lsi)*0.001 {
			t.Errorf("rank %d LSI: got %d, want %d (>0.1%% drift)", i+1, r.LiveScoreIndex, exp.lsi)
		}
		if r.TotalPower != exp.power {
			t.Errorf("rank %d TotalPower: got %d, want %d", i+1, r.TotalPower, exp.power)
		}
		if r.UnitScore != exp.unit {
			t.Errorf("rank %d UnitScore: got %d, want %d", i+1, r.UnitScore, exp.unit)
		}
		if math.Abs(float64(r.ActiveOverlapLoss)-exp.overlap) > 0.5 {
			t.Errorf("rank %d OverlapLoss: got %.1f, want %.1f", i+1, float64(r.ActiveOverlapLoss), exp.overlap)
		}
	}

	// Verify ordering: LSI descending
	for i := 1; i < len(tOutput.Timeline); i++ {
		if tOutput.Timeline[i].LiveScoreIndex > tOutput.Timeline[i-1].LiveScoreIndex {
			t.Errorf("rank %d LSI (%d) > rank %d LSI (%d) — not sorted",
				i+1, tOutput.Timeline[i].LiveScoreIndex, i, tOutput.Timeline[i-1].LiveScoreIndex)
		}
	}

	// Verify skill_efficiency is consistent with TotalPower
	// Higher TotalPower with lower efficiency should be possible
	for _, r := range tOutput.Timeline {
		if float64(r.SkillEfficiency) < 1.0 {
			t.Errorf("rank %d skill_efficiency %.2f < 1.0 — impossible", r.Rank, float64(r.SkillEfficiency))
		}
	}

	t.Logf("Golden test passed: #1 LSI=%d eff=%.2f #3 LSI=%d eff=%.2f",
		tOutput.Timeline[0].LiveScoreIndex, float64(tOutput.Timeline[0].SkillEfficiency),
		tOutput.Timeline[2].LiveScoreIndex, float64(tOutput.Timeline[2].SkillEfficiency))
}

func loadChartScore(path, key string) (*ChartScore, error) {
	data, err := jsonReadFile(path)
	if err != nil {
		return nil, err
	}
	var charts map[string]ChartScore
	if err := json.Unmarshal(data, &charts); err != nil {
		return nil, err
	}
	cs, ok := charts[key]
	if !ok {
		return nil, fmt.Errorf("chart %s not found", key)
	}
	return &cs, nil
}

func jsonReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
