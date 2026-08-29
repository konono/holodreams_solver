package main

import (
	"fmt"
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

// Golden test: capture results from current implementation to verify optimized version
func TestRecommendGolden(t *testing.T) {
	cf, owned := loadTestData(t)

	result := recommend(owned, cf.Cards, 10, 1, 1.0, 0.0, 192.0, "", "", false, cf)

	fmt.Printf("GOLDEN_BASE_SCORE=%d\n", result.BaseScore)
	for _, r := range result.Recommendations {
		fmt.Printf("GOLDEN_REC rank=%d card=%s delta=%d score=%d leader=%s members=%v\n",
			r.Rank, r.Cards[0].CardID, r.Delta, r.NewScore, r.BestTeam.LeaderID, r.BestTeam.MemberIDs)
	}
}
