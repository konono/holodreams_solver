package main

import (
	"sort"
	"testing"
)

// --- helpers ---

func loadBenchCards(b *testing.B, n int) (*CardsFile, []*Card) {
	b.Helper()
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		b.Fatal(err)
	}
	cards := make([]*Card, 0, n)
	for i := 0; i < n && i < len(cf.Cards); i++ {
		c := resolveCard(&cf.Cards[i], 0, nil, cf)
		cards = append(cards, &c)
	}
	return cf, cards
}

func benchTimeline() (*SongTimeline, []ScoreEvent) {
	events := make([]ScoreEvent, 200)
	for i := range events {
		events[i] = ScoreEvent{Time: float64(i) * 0.5, ComboIndex: i * 3, Weight: 1.0}
	}
	return &SongTimeline{Duration: 100, SpecialPoints: [5]float64{10, 30, 50, 70, 90}}, events
}

func benchLegacyResults(cards []*Card, n int) ([]SolveResult, map[string]*Card) {
	cardMap := make(map[string]*Card, len(cards))
	for _, c := range cards {
		cardMap[c.ID] = c
	}

	charGroups := map[string][]*Card{}
	for _, c := range cards {
		charGroups[c.Character] = append(charGroups[c.Character], c)
	}
	type charEntry struct {
		name     string
		maxTotal float64
	}
	var entries []charEntry
	for name, group := range charGroups {
		maxT := 0.0
		for _, c := range group {
			if c.Total > maxT {
				maxT = c.Total
			}
		}
		entries = append(entries, charEntry{name, maxT})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].maxTotal > entries[j].maxTotal })
	charNames := make([]string, len(entries))
	for i, e := range entries {
		charNames[i] = e.name
	}

	var results []SolveResult
	nChars := len(charNames)
	for a := 0; a < nChars-4 && len(results) < n; a++ {
		for b := a + 1; b < nChars-3 && len(results) < n; b++ {
			for ci := b + 1; ci < nChars-2 && len(results) < n; ci++ {
				for d := ci + 1; d < nChars-1 && len(results) < n; d++ {
					for e := d + 1; e < nChars && len(results) < n; e++ {
						team := [5]string{
							charGroups[charNames[a]][0].ID,
							charGroups[charNames[b]][0].ID,
							charGroups[charNames[ci]][0].ID,
							charGroups[charNames[d]][0].ID,
							charGroups[charNames[e]][0].ID,
						}
						results = append(results, SolveResult{
							Score:     EvalResult{UnitScore: 500000, TotalPower: 200000},
							LeaderIdx: 0,
							TeamIDs:   team,
						})
					}
				}
			}
		}
	}
	return results, cardMap
}

// === Phase 1: 単体評価 (innermost hot path) ===

func BenchmarkEvaluateTeam(b *testing.B) {
	_, cards := loadBenchCards(b, 10)
	team := [5]*Card{cards[0], cards[1], cards[2], cards[3], cards[4]}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		evaluateTeam(team, 0, 1.0, 0.0, defaultSongLength, nil)
	}
}

func BenchmarkComputeBaseScores(b *testing.B) {
	_, cards := loadBenchCards(b, 10)
	team := [5]*Card{cards[0], cards[1], cards[2], cards[3], cards[4]}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		computeBaseScores(team, 0, 1.0, 0.0, defaultSongLength)
	}
}

func BenchmarkApplyCostume(b *testing.B) {
	_, cards := loadBenchCards(b, 10)
	team := [5]*Card{cards[0], cards[1], cards[2], cards[3], cards[4]}
	base := computeBaseScores(team, 0, 1.0, 0.0, defaultSongLength)
	costume := &cards[0].CostumeSkill
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		applyCostume(&base, costume)
	}
}

// === Phase 2: 組み合わせ探索 ===

func BenchmarkSolve_25cards(b *testing.B) {
	_, cards := loadBenchCards(b, 25)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		solve(cards, 5, 1.0, 0.0, defaultSongLength, "", "", nil, nil)
	}
}

func BenchmarkSolve_70cards(b *testing.B) {
	_, cards := loadBenchCards(b, 70)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		solve(cards, 5, 1.0, 0.0, defaultSongLength, "", "", nil, nil)
	}
}

// === Phase 3: 衣装スイープ ===

func BenchmarkPruneCostumes_25cards(b *testing.B) {
	cf, cards := loadBenchCards(b, 25)
	ownedIDs := map[string]bool{}
	for _, c := range cards {
		ownedIDs[c.ID] = true
	}
	var rawCostumes []CostumeEntry
	for i := range cf.Cards {
		raw := &cf.Cards[i]
		if !ownedIDs[raw.ID] {
			continue
		}
		if len(raw.PotentialData) > 0 {
			rawCostumes = append(rawCostumes, CostumeEntry{raw.ID, raw.PotentialData[0].CostumeSkill})
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pruneCostumes(rawCostumes)
	}
}

func BenchmarkPruneCostumes_70cards(b *testing.B) {
	cf, cards := loadBenchCards(b, 70)
	ownedIDs := map[string]bool{}
	for _, c := range cards {
		ownedIDs[c.ID] = true
	}
	var rawCostumes []CostumeEntry
	for i := range cf.Cards {
		raw := &cf.Cards[i]
		if !ownedIDs[raw.ID] {
			continue
		}
		if len(raw.PotentialData) > 0 {
			rawCostumes = append(rawCostumes, CostumeEntry{raw.ID, raw.PotentialData[0].CostumeSkill})
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pruneCostumes(rawCostumes)
	}
}

func BenchmarkSolveSweepCostumes_25cards(b *testing.B) {
	cf, cards := loadBenchCards(b, 25)
	cardMap := make(map[string]*CardRaw, len(cf.Cards))
	for i := range cf.Cards {
		cardMap[cf.Cards[i].ID] = &cf.Cards[i]
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		solveSweepCostumes(cards, cf.Cards, cardMap, 5, 1.0, 0.0, defaultSongLength, nil, cf)
	}
}

func BenchmarkSolveSweepCostumes_70cards(b *testing.B) {
	cf, cards := loadBenchCards(b, 70)
	cardMap := make(map[string]*CardRaw, len(cf.Cards))
	for i := range cf.Cards {
		cardMap[cf.Cards[i].ID] = &cf.Cards[i]
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		solveSweepCostumes(cards, cf.Cards, cardMap, 5, 1.0, 0.0, defaultSongLength, nil, cf)
	}
}

func BenchmarkPrecomputeOwnedBases_25cards(b *testing.B) {
	_, cards := loadBenchCards(b, 25)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		precomputeOwnedBases(cards, 1.0, 0.0, defaultSongLength)
	}
}

func BenchmarkSolveForcedCostume_25cards(b *testing.B) {
	_, cards := loadBenchCards(b, 25)
	bases := precomputeOwnedBases(cards, 1.0, 0.0, defaultSongLength)
	costume := &cards[0].CostumeSkill
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		solveForcedCostumeFromBases(bases, costume)
	}
}

// === Phase 4: 順列最適化 (post-solve) ===

func BenchmarkOptimizeResults_25cards(b *testing.B) {
	_, cards := loadBenchCards(b, 25)
	out := solve(cards, 5, 1.0, 0.0, defaultSongLength, "", "", nil, nil)
	cardMap := make(map[string]*Card, len(cards))
	for _, c := range cards {
		cardMap[c.ID] = c
	}
	results := make([]SolveResult, len(out.Results))
	for i, r := range out.Results {
		results[i] = SolveResult{
			Score:     EvalResult{UnitScore: float64(r.UnitScore), TotalPower: float64(r.TotalPower)},
			LeaderIdx: 0,
			TeamIDs:   [5]string{r.MemberIDs[0], r.MemberIDs[1], r.MemberIDs[2], r.MemberIDs[3], r.MemberIDs[4]},
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		optimizeResults(results, cardMap, 1.0, 0.0, defaultSongLength, nil)
	}
}

func BenchmarkOptimizeResults_70cards(b *testing.B) {
	_, cards := loadBenchCards(b, 70)
	out := solve(cards, 5, 1.0, 0.0, defaultSongLength, "", "", nil, nil)
	cardMap := make(map[string]*Card, len(cards))
	for _, c := range cards {
		cardMap[c.ID] = c
	}
	results := make([]SolveResult, len(out.Results))
	for i, r := range out.Results {
		results[i] = SolveResult{
			Score:     EvalResult{UnitScore: float64(r.UnitScore), TotalPower: float64(r.TotalPower)},
			LeaderIdx: 0,
			TeamIDs:   [5]string{r.MemberIDs[0], r.MemberIDs[1], r.MemberIDs[2], r.MemberIDs[3], r.MemberIDs[4]},
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		optimizeResults(results, cardMap, 1.0, 0.0, defaultSongLength, nil)
	}
}

// === Phase 5: タイムライン再評価 ===

func BenchmarkRerankTopN_25cards(b *testing.B) {
	_, cards := loadBenchCards(b, 25)
	results, cardMap := benchLegacyResults(cards, 100)
	timeline, events := benchTimeline()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RerankTopN(results, cardMap, timeline, events, 1.0, 0.0, 100, nil, 10)
	}
}

func BenchmarkRerankTopN_70cards(b *testing.B) {
	_, cards := loadBenchCards(b, 70)
	results, cardMap := benchLegacyResults(cards, 100)
	timeline, events := benchTimeline()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RerankTopN(results, cardMap, timeline, events, 1.0, 0.0, 100, nil, 10)
	}
}

// === Phase 6: ボード最適化 ===

func BenchmarkBoardOpt_25cards(b *testing.B) {
	_, cards := loadBenchCards(b, 25)
	teams := make([][5]*Card, 10)
	for t := 0; t < 10; t++ {
		for i := 0; i < 5; i++ {
			teams[t][i] = cards[(t*5+i)%len(cards)]
		}
	}
	timeline, events := benchTimeline()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, team := range teams {
			OptimizeBoardForTeam(team, 100000, 100, timeline, events, 5.0)
		}
	}
}

func BenchmarkBoardOpt_70cards(b *testing.B) {
	_, cards := loadBenchCards(b, 70)
	teams := make([][5]*Card, 10)
	for t := 0; t < 10; t++ {
		for i := 0; i < 5; i++ {
			teams[t][i] = cards[(t*5+i)%len(cards)]
		}
	}
	timeline, events := benchTimeline()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, team := range teams {
			OptimizeBoardForTeam(team, 100000, 100, timeline, events, 5.0)
		}
	}
}

// === Phase 7: カード解決 ===

func BenchmarkResolveCard(b *testing.B) {
	cf, _ := loadBenchCards(b, 1)
	raw := &cf.Cards[0]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolveCard(raw, 0, nil, cf)
	}
}

// === Phase 8: レコメンド (end-to-end) ===

func BenchmarkRecommend_25owned(b *testing.B) {
	cf, _ := loadBenchCards(b, 1)
	owned := map[string]CardSpec{}
	for i := 0; i < 25 && i < len(cf.Cards); i++ {
		id := cf.Cards[i].ID
		owned[id] = CardSpec{ID: id, Potential: 0}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recommend(owned, cf.Cards, 5, 1, 1.0, 0.0, defaultSongLength, "", "", false, cf)
	}
}
