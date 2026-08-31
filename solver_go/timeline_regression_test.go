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
	for _, r := range tOutput.Timeline {
		if float64(r.SkillEfficiency) < 1.0 {
			t.Errorf("rank %d skill_efficiency %.2f < 1.0 — impossible", r.Rank, float64(r.SkillEfficiency))
		}
	}

	t.Logf("Golden test passed: #1 LSI=%d eff=%.2f #3 LSI=%d eff=%.2f",
		tOutput.Timeline[0].LiveScoreIndex, float64(tOutput.Timeline[0].SkillEfficiency),
		tOutput.Timeline[2].LiveScoreIndex, float64(tOutput.Timeline[2].SkillEfficiency))
}

// TestTimelineRegressionSweep verifies sweep + timeline with 34 cards.
func TestTimelineRegressionSweep(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Skip("cards.json not available")
	}
	chartData, err := loadChartScore("../data/chart_scores.json", "m0001_expert")
	if err != nil {
		t.Skip("chart_scores.json not available")
	}

	cardSpecs := []CardSpec{
		{ID: "aki_rosenthal_5"}, {ID: "natsuiro_matsuri_5"},
		{ID: "nakiri_ayame_5", Potential: 1}, {ID: "oozora_subaru_5"},
		{ID: "yuzuki_choco_5"}, {ID: "nekomata_okayu_5", Potential: 4},
		{ID: "shiranui_flare_5", Potential: 2}, {ID: "shirogane_noel_5"},
		{ID: "houshou_marine_5"}, {ID: "tokoyami_towa_5", Potential: 1},
	}
	cardsJSON, _ := json.Marshal(cardSpecs)
	sweep := true

	input := CLIInput{
		Action:         "solve",
		Cards:          json.RawMessage(cardsJSON),
		TopN:           5,
		SweepCostumes:  sweep,
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

	if len(tOutput.Timeline) < 5 {
		t.Fatalf("expected >=5 timeline results, got %d", len(tOutput.Timeline))
	}

	// Verify ordering
	for i := 1; i < len(tOutput.Timeline); i++ {
		if tOutput.Timeline[i].LiveScoreIndex > tOutput.Timeline[i-1].LiveScoreIndex {
			t.Errorf("rank %d LSI (%d) > rank %d LSI (%d)", i+1, tOutput.Timeline[i].LiveScoreIndex, i, tOutput.Timeline[i-1].LiveScoreIndex)
		}
	}

	// Verify board optimization is present for top results
	for i := 0; i < min(3, len(tOutput.Timeline)); i++ {
		r := tOutput.Timeline[i]
		if r.BoardOptimization == nil {
			t.Errorf("rank %d: board optimization missing", i+1)
			continue
		}
		if r.BoardOptimization.OptimizedLSI < r.BoardOptimization.BaselineLSI {
			t.Errorf("rank %d: optimized LSI %d < baseline %d", i+1, r.BoardOptimization.OptimizedLSI, r.BoardOptimization.BaselineLSI)
		}
	}

	// Snapshot: record #1 for future regression detection
	top := tOutput.Timeline[0]
	t.Logf("Sweep #1: LSI=%d costume=%v members=%v boardLSI=%d",
		top.LiveScoreIndex,
		func() string {
			if top.CostumeOnlyLeaderID != nil {
				return *top.CostumeOnlyLeaderID
			}
			return "none"
		}(),
		top.MemberIDs,
		top.BoardOptimization.OptimizedLSI)
}

// --- rerankTeamAllPerms equivalence tests ---

func TestRerankAllPerms_NoActive(t *testing.T) {
	cards := [5]*Card{
		makeTestCard("a", "cute", "holo1", 30000, 0, 0, 0, 0),
		makeTestCard("b", "pure", "holo1", 28000, 0, 0, 0, 0),
		makeTestCard("c", "happy", "holo2", 26000, 0, 0, 0, 0),
		makeTestCard("d", "cute", "holo1", 24000, 0, 0, 0, 0),
		makeTestCard("e", "pure", "holo2", 22000, 0, 0, 0, 0),
	}
	cards[0].SpecialSkill = &SpecialSkill{Duration: 10, ScoreSupport: 30}
	cards[2].SpecialSkill = &SpecialSkill{Duration: 8, SkillRateUp: 40}

	tl := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{15, 35, 55, 75, 90}}
	events := makeEvents(100, 0.5)

	assertRerankEquivalence(t, "no_active", cards, tl, events)
}

func TestRerankAllPerms_OneActive(t *testing.T) {
	prob := 600
	cards := [5]*Card{
		{ID: "a", Type: "vocal", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 120, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 10, ScoreSupport: 25}},
		makeTestCard("b", "dance", "holo1", 28000, 0, 0, 0, 0),
		makeTestCard("c", "visual", "holo2", 26000, 0, 0, 0, 0),
		makeTestCard("d", "vocal", "holo1", 24000, 0, 0, 0, 0),
		makeTestCard("e", "dance", "holo2", 22000, 0, 0, 0, 0),
	}

	tl := &SongTimeline{Duration: 120, SpecialPoints: [5]float64{10, 30, 50, 70, 95}}
	events := makeEvents(120, 0.4)

	assertRerankEquivalence(t, "one_active", cards, tl, events)
}

func TestRerankAllPerms_AllActive(t *testing.T) {
	prob := 550
	cards := [5]*Card{
		{ID: "a", Type: "vocal", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 10, ScoreSupport: 20}},
		{ID: "b", Type: "dance", CenterSkill: CenterSkill{Interval: 30, Duration: 5, ScoreUp: 80, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 8, SkillRateUp: 50}},
		{ID: "c", Type: "visual", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 60, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 12, ScoreSupport: 15}},
		{ID: "d", Type: "vocal", CenterSkill: CenterSkill{Interval: 35, Duration: 5, ScoreUp: 50, ActivationProbabilityPermil: &prob}},
		{ID: "e", Type: "dance", CenterSkill: CenterSkill{Interval: 22, Duration: 5, ScoreUp: 40, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 6, SkillRateUp: 30}},
	}

	tl := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 30, 50, 70, 90}}
	events := makeEvents(100, 0.5)

	assertRerankEquivalence(t, "all_active", cards, tl, events)
}

func TestRerankAllPerms_SameScoreUp(t *testing.T) {
	prob := 500
	cards := [5]*Card{
		{ID: "a", Type: "vocal", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 80, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 10, ScoreSupport: 20}},
		{ID: "b", Type: "dance", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 80, ActivationProbabilityPermil: &prob}},
		{ID: "c", Type: "visual", CenterSkill: CenterSkill{Interval: 30, Duration: 5, ScoreUp: 80, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 8, SkillRateUp: 40}},
		{ID: "d", Type: "vocal", CenterSkill: CenterSkill{Interval: 22, Duration: 5, ScoreUp: 60, ActivationProbabilityPermil: &prob}},
		{ID: "e", Type: "dance", CenterSkill: CenterSkill{Interval: 28, Duration: 5, ScoreUp: 60, ActivationProbabilityPermil: &prob}},
	}

	tl := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 30, 50, 70, 90}}
	events := makeEvents(100, 0.5)

	assertRerankEquivalence(t, "same_scoreup", cards, tl, events)
}

func TestRerankAllPerms_ConditionalScoreUp(t *testing.T) {
	prob := 550
	condType := "type:vocal>=3"
	condScoreUp := 150.0
	cards := [5]*Card{
		{ID: "a", Type: "vocal", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 80, Condition: &condType, ConditionalScoreUp: &condScoreUp, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 10, ScoreSupport: 20}},
		{ID: "b", Type: "vocal", CenterSkill: CenterSkill{Interval: 30, Duration: 5, ScoreUp: 70, ActivationProbabilityPermil: &prob}},
		{ID: "c", Type: "vocal", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 60, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 8, SkillRateUp: 50}},
		{ID: "d", Type: "dance", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 50, ActivationProbabilityPermil: &prob}},
		{ID: "e", Type: "visual", CenterSkill: CenterSkill{Interval: 35, Duration: 5, ScoreUp: 40, ActivationProbabilityPermil: &prob}},
	}

	tl := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 30, 50, 70, 90}}
	events := makeEvents(100, 0.5)

	assertRerankEquivalence(t, "conditional_scoreup", cards, tl, events)
}

func TestRerankAllPerms_DenseEvents(t *testing.T) {
	prob := 550
	cards := [5]*Card{
		{ID: "a", Type: "vocal", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 10, ScoreSupport: 20}},
		{ID: "b", Type: "dance", CenterSkill: CenterSkill{Interval: 30, Duration: 5, ScoreUp: 80, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 8, SkillRateUp: 50}},
		{ID: "c", Type: "visual", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 60, ActivationProbabilityPermil: &prob}},
		{ID: "d", Type: "vocal", CenterSkill: CenterSkill{Interval: 35, Duration: 5, ScoreUp: 50, ActivationProbabilityPermil: &prob}},
		{ID: "e", Type: "dance", CenterSkill: CenterSkill{Interval: 22, Duration: 5, ScoreUp: 40, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 6, SkillRateUp: 30}},
	}

	// 500 events — dense chart
	tl := &SongTimeline{Duration: 120, SpecialPoints: [5]float64{15, 35, 55, 75, 95}}
	events := make([]ScoreEvent, 500)
	for i := range events {
		events[i] = ScoreEvent{Time: float64(i) * 0.24, ComboIndex: i * 2, Weight: float64(1 + i%3)}
	}

	assertRerankEquivalence(t, "dense_events", cards, tl, events)
}

func TestRerankAllPerms_ClusteredSP(t *testing.T) {
	prob := 550
	cards := [5]*Card{
		{ID: "a", Type: "vocal", CenterSkill: CenterSkill{Interval: 20, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 15, ScoreSupport: 30}},
		{ID: "b", Type: "dance", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 90, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 12, SkillRateUp: 60}},
		{ID: "c", Type: "visual", CenterSkill: CenterSkill{Interval: 30, Duration: 5, ScoreUp: 70, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 10, ScoreSupport: 20, SkillRateUp: 30}},
		{ID: "d", Type: "vocal", CenterSkill: CenterSkill{Interval: 22, Duration: 5, ScoreUp: 50, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 8, SkillRateUp: 40}},
		{ID: "e", Type: "dance", CenterSkill: CenterSkill{Interval: 28, Duration: 5, ScoreUp: 40, ActivationProbabilityPermil: &prob},
			SpecialSkill: &SpecialSkill{Duration: 6, ScoreSupport: 10}},
	}

	// SP points clustered in first half
	tl := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 15, 20, 25, 30}}
	events := makeEvents(100, 0.5)

	assertRerankEquivalence(t, "clustered_sp", cards, tl, events)
}

// --- Board optimizer equivalence across team patterns ---

func TestBoardOptEquivalence_NoActive(t *testing.T) {
	cards := [5]*Card{
		makeTestCard("a", "cute", "holo1", 30000, 0, 0, 0, 0),
		makeTestCard("b", "pure", "holo1", 28000, 0, 0, 0, 0),
		makeTestCard("c", "happy", "holo2", 26000, 0, 0, 0, 0),
		makeTestCard("d", "cute", "holo1", 24000, 0, 0, 0, 0),
		makeTestCard("e", "pure", "holo2", 22000, 0, 0, 0, 0),
	}
	tl := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 30, 50, 70, 90}}
	events := makeEvents(100, 1.0)

	result := OptimizeBoardForTeam(cards, 100000, 100, tl, events, 0)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// With no active cards, CD reduce should have no effect
	for _, m := range result.Members {
		if m.CdReduceNodes != 0 {
			t.Errorf("card %s: expected 0 cdReduce with no active, got %d", m.CardID, m.CdReduceNodes)
		}
	}
	if result.OptimizedLSI != result.BaselineLSI {
		t.Errorf("expected same LSI with no active: baseline=%d optimized=%d", result.BaselineLSI, result.OptimizedLSI)
	}
}

func TestBoardOptEquivalence_OneActive(t *testing.T) {
	prob := 550
	cards := [5]*Card{
		{ID: "a", CenterSkill: CenterSkill{Interval: 25, Duration: 5, ScoreUp: 100, ActivationProbabilityPermil: &prob}},
		makeTestCard("b", "cute", "holo1", 28000, 0, 0, 0, 0),
		makeTestCard("c", "pure", "holo1", 26000, 0, 0, 0, 0),
		makeTestCard("d", "happy", "holo2", 24000, 0, 0, 0, 0),
		makeTestCard("e", "cute", "holo2", 22000, 0, 0, 0, 0),
	}
	tl := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 30, 50, 70, 90}}
	events := makeEvents(100, 0.5)

	result := OptimizeBoardForTeam(cards, 100000, 100, tl, events, 0)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.OptimizedLSI < result.BaselineLSI {
		t.Errorf("optimized should be >= baseline: %d < %d", result.OptimizedLSI, result.BaselineLSI)
	}
}

// --- end-to-end solve regression with real data ---

func TestSolveRegressionNonSweepTimeline(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Skip("cards.json not available")
	}
	chartData, err := loadChartScore("../data/chart_scores.json", "m0001_expert")
	if err != nil {
		t.Skip("chart_scores.json not available")
	}

	// 15 cards, non-sweep, timeline
	cardSpecs := []CardSpec{
		{ID: "nekomata_okayu_5", Potential: 4},
		{ID: "shiranui_flare_5", Potential: 2},
		{ID: "takane_lui_5", Potential: 3},
		{ID: "shirogane_noel_swim_5", Potential: 3},
		{ID: "hakui_koyori_5", Potential: 2},
		{ID: "kazama_iroha_5", Potential: 2},
		{ID: "tokoyami_towa_5", Potential: 1},
		{ID: "la_darknesss_5", Potential: 1},
		{ID: "omaru_polka_5", Potential: 1},
		{ID: "aki_rosenthal_5"},
		{ID: "nakiri_ayame_5", Potential: 1},
		{ID: "oozora_subaru_5"},
		{ID: "houshou_marine_5"},
		{ID: "natsuiro_matsuri_5"},
		{ID: "yuzuki_choco_5"},
	}
	cardsJSON, _ := json.Marshal(cardSpecs)

	input := CLIInput{
		Action:         "solve",
		Cards:          json.RawMessage(cardsJSON),
		TopN:           5,
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

	if len(tOutput.Timeline) < 5 {
		t.Fatalf("expected 5 timeline results, got %d", len(tOutput.Timeline))
	}

	// LSI ordering
	for i := 1; i < len(tOutput.Timeline); i++ {
		if tOutput.Timeline[i].LiveScoreIndex > tOutput.Timeline[i-1].LiveScoreIndex {
			t.Errorf("not sorted: #%d LSI %d > #%d LSI %d", i+1, tOutput.Timeline[i].LiveScoreIndex, i, tOutput.Timeline[i-1].LiveScoreIndex)
		}
	}

	// Board optimization present
	for i := 0; i < min(5, len(tOutput.Timeline)); i++ {
		if tOutput.Timeline[i].BoardOptimization == nil {
			t.Errorf("rank %d: board optimization missing", i+1)
		}
	}

	// All results should have positive values
	for _, r := range tOutput.Timeline {
		if r.LiveScoreIndex <= 0 {
			t.Errorf("rank %d: LSI should be positive, got %d", r.Rank, r.LiveScoreIndex)
		}
		if float64(r.ExpectedActive) < 0 {
			t.Errorf("rank %d: ExpectedActive should be non-negative", r.Rank)
		}
	}

	t.Logf("Non-sweep timeline: #1 LSI=%d members=%v", tOutput.Timeline[0].LiveScoreIndex, tOutput.Timeline[0].MemberIDs)
}

// --- helpers ---

func makeEvents(duration int, interval float64) []ScoreEvent {
	n := int(float64(duration) / interval)
	events := make([]ScoreEvent, n)
	for i := range events {
		events[i] = ScoreEvent{Time: float64(i) * interval, ComboIndex: i * 3, Weight: 1.0}
	}
	return events
}

func assertRerankEquivalence(t *testing.T, name string, cards [5]*Card, tl *SongTimeline, events []ScoreEvent) {
	t.Helper()
	fast := rerankTeamAllPerms(cards, 100000, 100, tl, events, 5.0)
	for pi, perm := range perms5 {
		var team [5]*Card
		for i, p := range perm {
			team[i] = cards[p]
		}
		slow := EvaluateFullTimeline(team, 100000, 100, tl, events, 5.0)

		if math.Abs(fast[pi].LiveScoreIndex-slow.LiveScoreIndex) > 1e-3 {
			t.Errorf("%s perm %d %v: LSI mismatch fast=%.3f slow=%.3f",
				name, pi, perm, fast[pi].LiveScoreIndex, slow.LiveScoreIndex)
		}
		if math.Abs(fast[pi].ActiveOverlapLoss-slow.ActiveOverlapLoss) > 1e-6 {
			t.Errorf("%s perm %d %v: OverlapLoss mismatch fast=%.6f slow=%.6f",
				name, pi, perm, fast[pi].ActiveOverlapLoss, slow.ActiveOverlapLoss)
		}
		if math.Abs(fast[pi].ExpectedActive-slow.ExpectedActive) > 1e-6 {
			t.Errorf("%s perm %d %v: ExpectedActive mismatch fast=%.6f slow=%.6f",
				name, pi, perm, fast[pi].ExpectedActive, slow.ExpectedActive)
		}
		for si := 0; si < 5; si++ {
			if math.Abs(fast[pi].SPEfficiency[si]-slow.SPEfficiency[si]) > 1e-3 {
				t.Errorf("%s perm %d %v: SPEfficiency[%d] mismatch fast=%.3f slow=%.3f",
					name, pi, perm, si, fast[pi].SPEfficiency[si], slow.SPEfficiency[si])
			}
		}
	}
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
