package main

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"
)

func loadTestData(t testing.TB) (*CardsFile, map[string]CardSpec) {
	t.Helper()
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Fatal(err)
	}

	// Use first 25 cards as owned set
	owned := map[string]CardSpec{}
	for i := 0; i < 25 && i < len(cf.Cards); i++ {
		id := cf.Cards[i].ID
		owned[id] = CardSpec{ID: id, Potential: 0}
	}
	return cf, owned
}

func TestRecommendBaseline(t *testing.T) {
	cf, owned := loadTestData(t)

	start := time.Now()
	result := recommend(owned, cf.Cards, 5, 1, 1.0, 0.0, 192.0, "", "", false, cf)
	elapsed := time.Since(start)

	t.Logf("BaseScore: %d", result.BaseScore)
	t.Logf("Recommendations: %d", len(result.Recommendations))
	t.Logf("Elapsed: %v", elapsed)
	for _, r := range result.Recommendations {
		t.Logf("  #%d: %s (%s) delta=%d score=%d team=%v",
			r.Rank, r.Cards[0].CardName, r.Cards[0].Character, r.Delta, r.NewScore, r.BestTeam.MemberIDs)
	}
}

func thresholdOwnedCards() map[string]CardSpec {
	ids := []string{
		"tokino_sora_5", "robocosan_5", "hoshimachi_suisei_5",
		"sakura_miko_5", "shirakami_fubuki_5", "natsuiro_matsuri_5", "akai_haato_5",
	}
	owned := make(map[string]CardSpec, len(ids))
	for _, id := range ids {
		owned[id] = CardSpec{ID: id, Potential: 0}
	}
	return owned
}

func TestRecommendCostumeOnlyUsesOwnedPotential(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	owned := thresholdOwnedCards()
	spec := owned["tokino_sora_5"]
	spec.Potential = 2
	owned[spec.ID] = spec
	const costumeID = "tokino_sora_5"
	recommendation := recommend(owned, cf.Cards, 5, 1, 1, 0, 192, "", costumeID, false, cf)
	var cards []*Card
	var costume *CostumeSkill
	for i := range cf.Cards {
		raw := &cf.Cards[i]
		if ownedSpec, ok := owned[raw.ID]; ok {
			card := resolveCard(raw, ownedSpec.Potential, ownedSpec.Level, cf)
			cards = append(cards, &card)
			if raw.ID == costumeID {
				skill := raw.PotentialData[ownedSpec.Potential].CostumeSkill
				costume = &skill
			}
		}
	}
	full := solve(cards, 1, 1, 0, 192, "", costumeID, costume, nil)
	if recommendation.BaseScore != full.Results[0].UnitScore {
		t.Fatalf("fixed costume recommendation base score = %d, solve = %d", recommendation.BaseScore, full.Results[0].UnitScore)
	}
}

func TestPotentialFindsUnownedCardAfterSeveralCopies(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	owned := thresholdOwnedCards()
	cardID := "ninomae_ina_nis_5"
	profiles := analyzePotential(owned, []string{cardID}, cf.Cards, 5, nil, 1, 0, 192, "", "", false, nil, nil, cf)
	if len(profiles) != 1 || len(profiles[0].Steps) != 5 {
		t.Fatalf("expected five milestones for %s, got %+v", cardID, profiles)
	}
	steps := profiles[0].Steps
	if steps[0].TargetPotential != 0 || steps[0].Delta != 0 {
		t.Fatalf("first copy should be 0凸 with no score gain: %+v", steps[0])
	}
	if steps[2].TargetPotential != 2 || steps[2].Delta <= 0 || profiles[0].FirstUsefulCopies <= 1 {
		t.Fatalf("later potential should be found despite no first-copy gain: %+v", profiles[0])
	}

	trialSpecs := make(map[string]CardSpec, len(owned)+1)
	for id, spec := range owned {
		trialSpecs[id] = spec
	}
	trialSpecs[cardID] = CardSpec{ID: cardID, Potential: 2}
	var trialCards []*Card
	for i := range cf.Cards {
		if spec, ok := trialSpecs[cf.Cards[i].ID]; ok {
			card := resolveCard(&cf.Cards[i], spec.Potential, spec.Level, cf)
			trialCards = append(trialCards, &card)
		}
	}
	full := solve(trialCards, 1, 1, 0, 192, "", "", nil, nil)
	if got, want := steps[2].NewScore, full.Results[0].UnitScore; got != want {
		t.Fatalf("2凸 profile score = %d, full solve = %d", got, want)
	}
}

func TestRecommendIncludesNewCardMultiCopy(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	result := recommend(thresholdOwnedCards(), cf.Cards, 500, 3, 1, 0, 192, "", "", false, cf)
	for _, recommendation := range result.Recommendations {
		if len(recommendation.Cards) == 1 {
			card := recommendation.Cards[0]
			if card.CardID == "ninomae_ina_nis_5" && card.Action == "acquire" && card.TargetPotential == 2 && card.Cost == 3 {
				return
			}
		}
	}
	t.Fatal("new card at 2凸 was omitted from three-copy recommendations")
}

func TestRecommendTimelineUsesSelectedChart(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../data/chart_scores.json")
	if err != nil {
		t.Fatal(err)
	}
	var charts map[string]json.RawMessage
	if err := json.Unmarshal(data, &charts); err != nil {
		t.Fatal(err)
	}
	owned := thresholdOwnedCards()
	var specs []CardSpec
	for _, spec := range owned {
		specs = append(specs, spec)
	}
	cardsJSON, _ := json.Marshal(specs)
	var scores []int
	var leaders []string
	for _, key := range []string{"m0001_easy", "m0001_expert"} {
		var chart ChartScore
		if err := json.Unmarshal(charts[key], &chart); err != nil {
			t.Fatal(err)
		}
		input := CLIInput{Action: "recommend", Cards: cardsJSON, TopN: 5, AcquireCount: 1,
			IncludePotential: true, SweepCostumes: true, ChartScoreData: &chart, BoardSearchMode: "fast"}
		value, err := dispatchAction(input, cf)
		if err != nil {
			t.Fatal(err)
		}
		result := value.(RecommendOutput)
		if result.ScoreMetric != "live_score_index" || len(result.PotentialCards) == 0 || len(result.Recommendations) == 0 {
			t.Fatalf("chart was not used for recommendations: %+v", result)
		}
		scores = append(scores, result.BaseScore)
		leaders = append(leaders, result.Recommendations[0].Cards[0].CardID)
	}
	if scores[0] == scores[1] || leaders[0] == leaders[1] {
		t.Fatalf("changing the selected chart should affect the timeline ranking: scores=%v, leaders=%v", scores, leaders)
	}
}

func BenchmarkRecommendCurrent(b *testing.B) {
	cf, owned := loadTestData(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recommend(owned, cf.Cards, 5, 1, 1.0, 0.0, 192.0, "", "", false, cf)
	}
}

func BenchmarkRecommendFixedLeader(b *testing.B) {
	cf, owned := loadTestData(b)
	// Use first owned card as fixed leader
	var leaderID string
	for id := range owned {
		leaderID = id
		break
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recommend(owned, cf.Cards, 5, 1, 1.0, 0.0, 192.0, leaderID, "", false, cf)
	}
}

func TestSolveSweepCostumes(t *testing.T) {
	cf, owned := loadTestData(t)
	var cards []*Card
	for _, spec := range owned {
		for i := range cf.Cards {
			if cf.Cards[i].ID == spec.ID {
				c := resolveCard(&cf.Cards[i], spec.Potential, spec.Level, cf)
				cards = append(cards, &c)
				break
			}
		}
	}
	rawCardMap := map[string]*CardRaw{}
	for i := range cf.Cards {
		rawCardMap[cf.Cards[i].ID] = &cf.Cards[i]
	}

	start := time.Now()
	result := solveSweepCostumes(cards, cf.Cards, rawCardMap, 5, 1.0, 0.0, 192.0, nil, cf)
	elapsed := time.Since(start)

	t.Logf("SweepCostumes: %d results, %d combos, elapsed=%v", len(result.Results), result.TotalCombinations, elapsed)
	for _, r := range result.Results {
		t.Logf("  #%d score=%d leader=%s costume=%v", r.Rank, r.UnitScore, r.LeaderID, r.CostumeOnlyLeaderID)
	}
}

func TestPruneCostumes(t *testing.T) {
	cf, _ := loadTestData(t)

	var allCostumes []CostumeEntry
	for i := range cf.Cards {
		if len(cf.Cards[i].PotentialData) > 0 {
			allCostumes = append(allCostumes, CostumeEntry{cf.Cards[i].ID, cf.Cards[i].PotentialData[0].CostumeSkill})
		}
	}
	pruned := pruneCostumes(allCostumes)
	t.Logf("Costumes: %d -> %d (pruned %d)", len(allCostumes), len(pruned), len(allCostumes)-len(pruned))
}

func TestRecommendSweepCostumes(t *testing.T) {
	cf, owned := loadTestData(t)

	start := time.Now()
	result := recommend(owned, cf.Cards, 5, 1, 1.0, 0.0, 192.0, "", "", true, cf)
	elapsed := time.Since(start)

	t.Logf("BaseScore: %d", result.BaseScore)
	t.Logf("Recommendations: %d", len(result.Recommendations))
	t.Logf("Elapsed: %v", elapsed)
	for _, r := range result.Recommendations {
		costumeID := ""
		if r.BestTeam.CostumeOnlyLeaderID != nil {
			costumeID = *r.BestTeam.CostumeOnlyLeaderID
		}
		t.Logf("  #%d: %s (%s) delta=%d score=%d costume=%s team=%v",
			r.Rank, r.Cards[0].CardName, r.Cards[0].Character, r.Delta, r.NewScore, costumeID, r.BestTeam.MemberIDs)
	}
}

func TestRecommendGolden(t *testing.T) {
	cf, owned := loadTestData(t)

	result := recommend(owned, cf.Cards, 10, 1, 1.0, 0.0, 192.0, "", "", false, cf)

	if result.BaseScore != 806046 {
		t.Fatalf("BaseScore = %d, want 806046", result.BaseScore)
	}

	type golden struct {
		cardID string
		delta  int
		score  int
	}
	expected := []golden{
		{"aki_rosenthal_swim_5", 22758, 828804},
		{"anya_melfissa_swim_5", 22522, 828568},
		{"ookami_mio_swim_5", 21975, 828021},
		{"otonose_kanade_swim_5", 18955, 825001},
		{"airani_iofifteen_5", 16618, 822664},
		{"shirogane_noel_swim_5", 12946, 818992},
		{"nekomata_okayu_swim_5", 12569, 818615},
		{"sakura_miko_swim_5", 9587, 815633},
		{"himemori_luna_swim_5", 8533, 814579},
		{"kobo_kanaeru_5", 4943, 810989},
	}

	if len(result.Recommendations) != len(expected) {
		t.Fatalf("got %d recommendations, want %d", len(result.Recommendations), len(expected))
	}
	for i, exp := range expected {
		r := result.Recommendations[i]
		if r.Cards[0].CardID != exp.cardID || r.Delta != exp.delta || r.NewScore != exp.score {
			t.Errorf("rank %d: got card=%s delta=%d score=%d, want card=%s delta=%d score=%d",
				i+1, r.Cards[0].CardID, r.Delta, r.NewScore, exp.cardID, exp.delta, exp.score)
		}
	}
}

func TestRecommendMultiAcquire(t *testing.T) {
	cf, owned := loadTestData(t)

	start := time.Now()
	result := recommend(owned, cf.Cards, 5, 2, 1.0, 0.0, 192.0, "", "", false, cf)
	elapsed := time.Since(start)

	t.Logf("BaseScore: %d, AcquireCount: %d, Elapsed: %v", result.BaseScore, result.AcquireCount, elapsed)

	type goldenCombo struct {
		delta int
		score int
	}
	expected := []goldenCombo{
		{33764, 839810},
		{32824, 838870},
		{31397, 837443},
		{29301, 835347},
		{26771, 832817},
	}
	if len(result.Recommendations) != len(expected) {
		t.Fatalf("got %d recommendations, want %d", len(result.Recommendations), len(expected))
	}
	for i, exp := range expected {
		r := result.Recommendations[i]
		if r.Delta != exp.delta || r.NewScore != exp.score {
			t.Errorf("rank %d: got delta=%d score=%d, want delta=%d score=%d",
				i+1, r.Delta, r.NewScore, exp.delta, exp.score)
		}
	}
}

func TestRecommendMultiAcquireSweep(t *testing.T) {
	cf, owned := loadTestData(t)

	start := time.Now()
	result := recommend(owned, cf.Cards, 5, 2, 1.0, 0.0, 192.0, "", "", true, cf)
	elapsed := time.Since(start)

	t.Logf("BaseScore: %d, AcquireCount: %d", result.BaseScore, result.AcquireCount)
	t.Logf("Recommendations: %d, Elapsed: %v", len(result.Recommendations), elapsed)
	for _, r := range result.Recommendations {
		names := make([]string, len(r.Cards))
		for i, c := range r.Cards {
			names[i] = c.CardName
		}
		costumeID := ""
		if r.BestTeam.CostumeOnlyLeaderID != nil {
			costumeID = *r.BestTeam.CostumeOnlyLeaderID
		}
		t.Logf("  #%d: %v delta=%d score=%d costume=%s", r.Rank, names, r.Delta, r.NewScore, costumeID)
	}
}

// TestRecommendEquivalence verifies that solveWithRequiredCard finds the same
// best score as a full solve for each candidate.
func TestRecommendEquivalence(t *testing.T) {
	cf, owned := loadTestData(t)

	rawCardMap := map[string]*CardRaw{}
	for i := range cf.Cards {
		rawCardMap[cf.Cards[i].ID] = &cf.Cards[i]
	}
	resolveOwned := func(specs map[string]CardSpec) []*Card {
		cards := make([]*Card, 0, len(specs))
		for _, spec := range specs {
			raw := rawCardMap[spec.ID]
			if raw == nil {
				continue
			}
			c := resolveCard(raw, spec.Potential, spec.Level, cf)
			cards = append(cards, &c)
		}
		return cards
	}

	// Test a few acquire candidates
	tested := 0
	for i := range cf.Cards {
		raw := &cf.Cards[i]
		if _, ok := owned[raw.ID]; ok {
			continue
		}
		if tested >= 5 {
			break
		}
		tested++

		trialSpecs := map[string]CardSpec{}
		for k, v := range owned {
			trialSpecs[k] = v
		}
		trialSpecs[raw.ID] = CardSpec{ID: raw.ID, Potential: 0}
		trialCards := resolveOwned(trialSpecs)

		fullResult := solve(trialCards, 1, 1.0, 0.0, 192.0, "", "", nil, nil)
		fullScore := 0
		if len(fullResult.Results) > 0 {
			fullScore = fullResult.Results[0].UnitScore
		}

		resolvedCand := resolveCard(raw, 0, nil, cf)
		reqScore, _, _ := solveWithRequiredCard(trialCards, &resolvedCand, 1.0, 0.0, 192.0, "", nil)
		reqScoreInt := int(math.Round(reqScore.UnitScore))

		// Required-card should find score >= full solve's score minus the baseline
		// (since full solve can find teams without the candidate)
		baseCards := resolveOwned(owned)
		baseResult := solve(baseCards, 1, 1.0, 0.0, 192.0, "", "", nil, nil)
		baseScoreVal := 0
		if len(baseResult.Results) > 0 {
			baseScoreVal = baseResult.Results[0].UnitScore
		}

		if fullScore > baseScoreVal && reqScoreInt < fullScore {
			// The full solve found a better team using the candidate, but required-card missed it
			t.Errorf("candidate %s: full=%d required=%d base=%d — required-card missed an improvement",
				raw.ID, fullScore, reqScoreInt, baseScoreVal)
		}
	}
}
