package main

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"testing"
)

func TestBoardUpperBoundContainsExactSearch(t *testing.T) {
	cards, timeline, events := benchRerankCards()
	for _, permIndex := range []int{0, 17, 83, 119} {
		var order [5]*Card
		for i, p := range perms5[permIndex] {
			order[i] = cards[p]
		}
		bound := BoardUpperBound(order, 100000, timeline.Duration, timeline, events, 5)
		opt := OptimizeBoardForTeam(order, 100000, timeline.Duration, timeline, events, 5)
		if opt == nil || bound < opt.BestEval.LiveScoreIndex {
			t.Fatalf("permutation %d: bound %.6f below exact %.6f", permIndex, bound, opt.BestEval.LiveScoreIndex)
		}
	}
}

func TestBoardAwareMatchesAllOrders(t *testing.T) {
	cards, timeline, events := benchRerankCards()
	var candidate TimelineRerankResult
	candidate.TotalPower = 100000
	candidate.AlwaysOnSupport = 5
	cardMap := map[string]*Card{}
	for i, card := range cards {
		candidate.TeamIDs[i] = card.ID
		cardMap[card.ID] = card
	}
	result, stats := RerankBoardAware([]TimelineRerankResult{candidate}, cardMap, timeline, events, 1, "balanced")
	best := 0.0
	for _, perm := range perms5 {
		var order [5]*Card
		for i, p := range perm {
			order[i] = cards[p]
		}
		opt := OptimizeBoardForTeam(order, candidate.TotalPower, timeline.Duration, timeline, events, candidate.AlwaysOnSupport)
		if opt.BestEval.LiveScoreIndex > best {
			best = opt.BestEval.LiveScoreIndex
		}
	}
	if len(result) != 1 || math.Abs(result[0].LiveScoreIndex-best) > 1e-6 {
		t.Fatalf("balanced %.6f, exhaustive brute force %.6f", result[0].LiveScoreIndex, best)
	}
	if stats.PermutationsChecked != 120 || stats.ExactSearches+stats.PermutationsPruned+stats.PermutationsEquivalent != 120 {
		t.Fatalf("some orders were skipped without proof: %+v", stats)
	}
}

func TestEquivalentSpecialOrdersHaveSameBoardOptimum(t *testing.T) {
	cards, timeline, events := benchRerankCards()
	shared := &SpecialSkill{Duration: 10, ScoreSupport: 20}
	cards[0].SpecialSkill = shared
	cards[2].SpecialSkill = shared
	swapped := cards
	swapped[0], swapped[2] = swapped[2], swapped[0]
	if specialOrderKey(cards, timeline) != specialOrderKey(swapped, timeline) {
		t.Fatal("identical SP windows should have the same key")
	}
	a := OptimizeBoardForTeam(cards, 100000, timeline.Duration, timeline, events, 5)
	b := OptimizeBoardForTeam(swapped, 100000, timeline.Duration, timeline, events, 5)
	if math.Abs(a.BestEval.LiveScoreIndex-b.BestEval.LiveScoreIndex) > 1e-6 {
		t.Fatalf("equivalent SP windows changed board optimum: %.6f vs %.6f", a.BestEval.LiveScoreIndex, b.BestEval.LiveScoreIndex)
	}
}

func TestTimelineBalancedOutputUsesBoardScore(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	chart, err := loadChartScore("../data/chart_scores.json", "m0001_expert")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"tokino_sora_5", "aki_rosenthal_5", "natsuiro_matsuri_5", "shirakami_fubuki_5", "akai_haato_5", "nakiri_ayame_5"}
	cardsJSON, _ := json.Marshal(ids)
	out, err := dispatchAction(CLIInput{Action: "solve", Cards: cardsJSON, TopN: 2, ChartScoreData: chart}, cf)
	if err != nil {
		t.Fatal(err)
	}
	result, ok := out.(TimelineJSONOutput)
	if !ok || len(result.Timeline) != 2 || result.BoardSearch == nil {
		t.Fatalf("unexpected output: %T", out)
	}
	for i, r := range result.Timeline {
		if !r.BoardApplied || r.BoardOptimization == nil || math.Abs(float64(r.LiveScoreIndex-r.BoardOptimization.OptimizedLSI)) > 1 {
			t.Fatalf("rank %d LSI does not reflect board optimization: %+v", i+1, r)
		}
		if i > 0 && result.Timeline[i-1].LiveScoreIndex < r.LiveScoreIndex {
			t.Fatal("not ranked by optimized LSI")
		}
	}
}

func TestBalancedTop1MatchesExhaustiveHighOverlap(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	chart, err := loadChartScore("../data/chart_scores.json", "m0001_expert")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"robocosan_5", "azki_5", "sakura_miko_5", "aki_rosenthal_5", "akai_haato_5", "yuzuki_choco_5", "inugami_korone_5"}
	cardsJSON, _ := json.Marshal(ids)
	input := CLIInput{Action: "solve", Cards: cardsJSON, TopN: 1, ChartScoreData: chart}
	balancedResult, err := dispatchAction(input, cf)
	if err != nil {
		t.Fatal(err)
	}
	input.BoardSearchMode = "exhaustive"
	exhaustiveResult, err := dispatchAction(input, cf)
	if err != nil {
		t.Fatal(err)
	}
	balanced := balancedResult.(TimelineJSONOutput).Timeline[0]
	exhaustive := exhaustiveResult.(TimelineJSONOutput).Timeline[0]
	if balanced.LiveScoreIndex != exhaustive.LiveScoreIndex {
		t.Fatalf("balanced top1 %d (%v), exhaustive %d (%v)", balanced.LiveScoreIndex, balanced.MemberIDs, exhaustive.LiveScoreIndex, exhaustive.MemberIDs)
	}
}

func TestFastAndBalancedSharedTeams(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	chart, err := loadChartScore("../data/chart_scores.json", "m0001_expert")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"robocosan_5", "azki_5", "sakura_miko_5", "aki_rosenthal_5", "akai_haato_5", "yuzuki_choco_5", "inugami_korone_5"}
	cardsJSON, _ := json.Marshal(ids)
	input := CLIInput{Action: "solve", Cards: cardsJSON, TopN: 10, ChartScoreData: chart, BoardSearchMode: "fast"}
	fastOut, err := dispatchAction(input, cf)
	if err != nil {
		t.Fatal(err)
	}
	input.BoardSearchMode = "balanced"
	balancedOut, err := dispatchAction(input, cf)
	if err != nil {
		t.Fatal(err)
	}
	key := func(r TimelineJSONResult) string {
		ids := append([]string(nil), r.MemberIDs...)
		sort.Strings(ids)
		costume := ""
		if r.CostumeOnlyLeaderID != nil {
			costume = *r.CostumeOnlyLeaderID
		}
		return strings.Join(ids, ",") + "/" + costume
	}
	oldByTeam := map[string]TimelineJSONResult{}
	for _, r := range fastOut.(TimelineJSONOutput).Timeline {
		oldByTeam[key(r)] = r
	}
	shared := 0
	for _, now := range balancedOut.(TimelineJSONOutput).Timeline {
		old, ok := oldByTeam[key(now)]
		if !ok {
			continue
		}
		shared++
		if old.UnitScore != now.UnitScore || old.TotalPower != now.TotalPower {
			t.Fatalf("base team stats changed: old=%+v new=%+v", old, now)
		}
		if old.BoardOptimization == nil || float64(now.LiveScoreIndex) < float64(old.BoardOptimization.OptimizedLSI)-1 {
			t.Fatalf("same team's board result regressed: old=%+v new=%+v", old, now)
		}
		if strings.Join(old.MemberIDs, ",") == strings.Join(now.MemberIDs, ",") && math.Abs(float64(now.LiveScoreIndex-old.BoardOptimization.OptimizedLSI)) > 1 {
			t.Fatalf("same SP order produced a different board score: old=%+v new=%+v", old, now)
		}
	}
	if shared == 0 {
		t.Fatal("fixture has no shared Top10 teams")
	}
	t.Logf("shared Top10 teams: %d", shared)
}

func TestBoardAwareRecoversRankingInversion(t *testing.T) {
	prob := 500
	cardMap := map[string]*Card{}
	var plain, improved TimelineRerankResult
	plain.TotalPower, improved.TotalPower = 120, 100
	for i := 0; i < 5; i++ {
		plainID := string(rune('a' + i))
		improvedID := string(rune('f' + i))
		cardMap[plainID] = &Card{ID: plainID}
		cardMap[improvedID] = &Card{ID: improvedID}
		plain.TeamIDs[i], improved.TeamIDs[i] = plainID, improvedID
	}
	cardMap["f"].CenterSkill = CenterSkill{Interval: 10, Duration: 1, ScoreUp: 100, ActivationProbabilityPermil: &prob}
	cardMap["g"].CenterSkill = CenterSkill{Interval: 10, Duration: 1, ScoreUp: 80, ActivationProbabilityPermil: &prob}
	timeline := &SongTimeline{Duration: 40}
	events := []ScoreEvent{{Time: 9.7, Weight: 1}, {Time: 19.3, Weight: 1}, {Time: 28.9, Weight: 1}}
	var plainCards, improvedCards [5]*Card
	for i := 0; i < 5; i++ {
		plainCards[i], improvedCards[i] = cardMap[plain.TeamIDs[i]], cardMap[improved.TeamIDs[i]]
	}
	plain.LiveScoreIndex = EvaluateFullTimeline(plainCards, plain.TotalPower, timeline.Duration, timeline, events, 0).LiveScoreIndex
	improved.LiveScoreIndex = EvaluateFullTimeline(improvedCards, improved.TotalPower, timeline.Duration, timeline, events, 0).LiveScoreIndex
	if plain.LiveScoreIndex <= improved.LiveScoreIndex {
		t.Fatal("fixture has no base ranking inversion")
	}
	base := []TimelineRerankResult{plain, improved}
	for _, mode := range []string{"balanced", "exhaustive"} {
		results, stats := RerankBoardAware(base, cardMap, timeline, events, 1, mode)
		if len(results) != 1 || results[0].TeamIDs[0] != "f" || results[0].BoardOpt == nil {
			t.Fatalf("%s: expected improved team, got %+v", mode, results)
		}
		if math.Abs(results[0].LiveScoreIndex-results[0].BoardOpt.BestEval.LiveScoreIndex) > 1e-6 {
			t.Fatalf("%s: ranking LSI and board result differ", mode)
		}
		if results[0].BoardOpt.Members[0].CdReduceNodes == 0 && results[0].BoardOpt.Members[1].CdReduceNodes == 0 {
			t.Fatalf("%s: expected frequency nodes to cause the inversion", mode)
		}
		if stats.ExactSearches+stats.PermutationsPruned+stats.PermutationsEquivalent != stats.PermutationsChecked {
			t.Fatalf("%s: some orders were skipped without proof: %+v", mode, stats)
		}
	}
}
