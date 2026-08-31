package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

// --- Golden snapshot tests ---
// Update golden values when: scoring model changes, cards.json syncs new data,
// or chart_scores.json updates. Run `go test -run Golden -v` to check.

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

	tOutput := runSolveTimeline(t, input, cf)
	assertGoldenTimeline(t, tOutput, []goldenEntry{
		{lsi: 265944367781, power: 200519, unit: 921893, overlap: 25.0},
		{lsi: 265259368620, power: 205456, unit: 936917, overlap: 34.7},
		{lsi: 265192636783, power: 205243, unit: 934613},
	})
}

func TestTimelineRegressionGolden_M0005(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Skip("cards.json not available")
	}
	chartData, err := loadChartScore("../data/chart_scores.json", "m0005_expert")
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

	tOutput := runSolveTimeline(t, input, cf)
	assertGoldenTimeline(t, tOutput, []goldenEntry{
		{lsi: 291282678137, power: 200519, unit: 921893, overlap: 27.0},
		{lsi: 289992718539, power: 205456, unit: 936917, overlap: 34.0},
		{lsi: 288862810022, power: 204539, unit: 932286, overlap: 26.7},
	})
}

func TestTimelineRegressionSweep10Cards(t *testing.T) {
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

	input := CLIInput{
		Action:         "solve",
		Cards:          json.RawMessage(cardsJSON),
		TopN:           3,
		SweepCostumes:  true,
		ChartScoreData: chartData,
	}

	tOutput := runSolveTimeline(t, input, cf)
	assertGoldenTimeline(t, tOutput, []goldenEntry{
		{lsi: 292235942215},
		{lsi: 290740791296},
		{lsi: 290740791296},
	})
	assertBoardOptPresent(t, tOutput, 3)
}

func TestTimelineRegressionSweep34Cards(t *testing.T) {
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
		{ID: "yukihana_lamy_5"}, {ID: "momosuzu_nene_5"},
		{ID: "omaru_polka_5", Potential: 1}, {ID: "la_darknesss_5", Potential: 1},
		{ID: "takane_lui_5", Potential: 3}, {ID: "hakui_koyori_5", Potential: 2},
		{ID: "kazama_iroha_5", Potential: 2}, {ID: "anya_melfissa_5"},
		{ID: "kobo_kanaeru_5"}, {ID: "irys_5"},
		{ID: "ouro_kronii_5"}, {ID: "hakos_baelz_5"},
		{ID: "fuwawa_abyssgard_5"}, {ID: "nerissa_ravencroft_5"},
		{ID: "ichijou_ririka_5"}, {ID: "otonose_kanade_5"},
		{ID: "shirogane_noel_swim_5", Potential: 3}, {ID: "ookami_mio_5"},
		{ID: "inugami_korone_swim_5"}, {ID: "nekomata_okayu_swim_5", Potential: 2},
		{ID: "ookami_mio_swim_5"}, {ID: "pavolia_reine_5"},
		{ID: "mori_calliope_swim_5"}, {ID: "takanashi_kiara_5"},
	}
	cardsJSON, _ := json.Marshal(cardSpecs)

	input := CLIInput{
		Action:         "solve",
		Cards:          json.RawMessage(cardsJSON),
		TopN:           5,
		SweepCostumes:  true,
		ChartScoreData: chartData,
	}

	tOutput := runSolveTimeline(t, input, cf)
	assertGoldenTimeline(t, tOutput, []goldenEntry{
		{lsi: 326449317402, power: 219336, unit: 884800},
		{lsi: 326449317402, power: 219336, unit: 884800},
		{lsi: 322264082710, power: 216414, unit: 864487},
		{lsi: 322264082710, power: 216414, unit: 864487},
		{lsi: 321995200545, power: 217435, unit: 868349},
	})
	assertBoardOptPresent(t, tOutput, 5)
}

func TestSolveRegressionNonSweepTimeline(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Skip("cards.json not available")
	}
	chartData, err := loadChartScore("../data/chart_scores.json", "m0001_expert")
	if err != nil {
		t.Skip("chart_scores.json not available")
	}

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

	tOutput := runSolveTimeline(t, input, cf)
	assertGoldenTimeline(t, tOutput, []goldenEntry{
		{lsi: 326449317402},
	})
	assertBoardOptPresent(t, tOutput, 5)
}

// --- rerankTeamAllPerms equivalence tests (7 patterns × 120 permutations) ---

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
	assertRerankEquivalence(t, "no_active", cards, tl, makeEvents(100, 0.5))
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
	assertRerankEquivalence(t, "one_active", cards, tl, makeEvents(120, 0.4))
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
	assertRerankEquivalence(t, "all_active", cards, tl, makeEvents(100, 0.5))
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
	assertRerankEquivalence(t, "same_scoreup", cards, tl, makeEvents(100, 0.5))
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
	assertRerankEquivalence(t, "conditional_scoreup", cards, tl, makeEvents(100, 0.5))
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

	tl := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 15, 20, 25, 30}}
	assertRerankEquivalence(t, "clustered_sp", cards, tl, makeEvents(100, 0.5))
}

// --- Board optimizer edge cases ---

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
	for _, m := range result.Members {
		if m.CdReduceNodes != 0 {
			t.Errorf("card %s: expected 0 cdReduce with no active, got %d", m.CardID, m.CdReduceNodes)
		}
	}
	if result.OptimizedLSI != result.BaselineLSI {
		t.Errorf("expected same LSI: baseline=%d optimized=%d", result.BaselineLSI, result.OptimizedLSI)
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

// --- helpers ---

type goldenEntry struct {
	lsi     int
	power   int
	unit    int
	overlap float64
}

func runSolveTimeline(t *testing.T, input CLIInput, cf *CardsFile) TimelineJSONOutput {
	t.Helper()
	result, err := dispatchAction(input, cf)
	if err != nil {
		t.Fatal(err)
	}
	tOutput, ok := result.(TimelineJSONOutput)
	if !ok {
		t.Fatalf("expected TimelineJSONOutput, got %T", result)
	}
	return tOutput
}

func assertGoldenTimeline(t *testing.T, tOutput TimelineJSONOutput, expected []goldenEntry) {
	t.Helper()
	if len(tOutput.Timeline) < len(expected) {
		t.Fatalf("expected >=%d timeline results, got %d", len(expected), len(tOutput.Timeline))
	}

	for i, exp := range expected {
		r := tOutput.Timeline[i]
		if exp.lsi != 0 && math.Abs(float64(r.LiveScoreIndex-exp.lsi)) > float64(exp.lsi)*0.001 {
			t.Errorf("rank %d LSI: got %d, want %d (>0.1%% drift)", i+1, r.LiveScoreIndex, exp.lsi)
		}
		if exp.power != 0 && r.TotalPower != exp.power {
			t.Errorf("rank %d TotalPower: got %d, want %d", i+1, r.TotalPower, exp.power)
		}
		if exp.unit != 0 && r.UnitScore != exp.unit {
			t.Errorf("rank %d UnitScore: got %d, want %d", i+1, r.UnitScore, exp.unit)
		}
		if exp.overlap != 0 && math.Abs(float64(r.ActiveOverlapLoss)-exp.overlap) > 0.5 {
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

	// Verify skill_efficiency >= 1.0
	for _, r := range tOutput.Timeline {
		if float64(r.SkillEfficiency) < 1.0 {
			t.Errorf("rank %d skill_efficiency %.2f < 1.0", r.Rank, float64(r.SkillEfficiency))
		}
	}
}

func assertBoardOptPresent(t *testing.T, tOutput TimelineJSONOutput, n int) {
	t.Helper()
	for i := 0; i < min(n, len(tOutput.Timeline)); i++ {
		r := tOutput.Timeline[i]
		if r.BoardOptimization == nil {
			t.Errorf("rank %d: board optimization missing", i+1)
			continue
		}
		if r.BoardOptimization.OptimizedLSI < r.BoardOptimization.BaselineLSI {
			t.Errorf("rank %d: optimized LSI %d < baseline %d", i+1,
				r.BoardOptimization.OptimizedLSI, r.BoardOptimization.BaselineLSI)
		}
	}
}

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
	fast := rerankTeamAllPerms(cards, 100000, tl.Duration, tl, events, 5.0)
	for pi, perm := range perms5 {
		var team [5]*Card
		for i, p := range perm {
			team[i] = cards[p]
		}
		slow := EvaluateFullTimeline(team, 100000, tl.Duration, tl, events, 5.0)

		if math.Abs(fast[pi].LiveScoreIndex-slow.LiveScoreIndex) > 1e-6 {
			t.Errorf("%s perm %d %v: LSI mismatch fast=%.6f slow=%.6f",
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
			if math.Abs(fast[pi].SPEfficiency[si]-slow.SPEfficiency[si]) > 1e-6 {
				t.Errorf("%s perm %d %v: SPEfficiency[%d] mismatch fast=%.6f slow=%.6f",
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
