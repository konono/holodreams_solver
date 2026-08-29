package main

import (
	"math"
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

	if result.BaseScore != 805113 {
		t.Fatalf("BaseScore = %d, want 805113", result.BaseScore)
	}

	type golden struct {
		cardID string
		delta  int
		score  int
	}
	expected := []golden{
		{"otonose_kanade_swim_5", 18912, 824025},
		{"ookami_mio_swim_5", 17710, 822823},
		{"nekomata_okayu_swim_5", 13978, 819091},
		{"airani_iofifteen_5", 12606, 817719},
		{"shirogane_noel_swim_5", 11244, 816357},
		{"sakura_miko_swim_5", 11031, 816144},
		{"himemori_luna_swim_5", 8473, 813586},
		{"kobo_kanaeru_5", 6382, 811495},
		{"kureiji_ollie_swim_5", 2665, 807778},
		{"hakos_baelz_5", 2165, 807278},
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
		{32568, 837681},
		{27985, 833098},
		{27525, 832638},
		{25522, 830635},
		{24291, 829404},
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
