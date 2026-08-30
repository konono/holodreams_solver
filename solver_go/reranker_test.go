package main

import (
	"testing"
)

func TestPerms5_Count(t *testing.T) {
	// Verify all 120 permutations are unique
	seen := map[[5]int]bool{}
	for _, p := range perms5 {
		if seen[p] {
			t.Fatalf("duplicate permutation: %v", p)
		}
		seen[p] = true
	}
	if len(seen) != 120 {
		t.Errorf("got %d unique perms, want 120", len(seen))
	}
}

func TestRerankTopN_OrderMatters(t *testing.T) {
	// Two cards: one with strong SP ScoreSupport, another with weaker SP.
	// The order determines which SP point each card gets.
	// Placing strong SP at a note-dense SP point should score higher.

	strongSP := makeTimelineCard(80, 25, 10, 1000)
	strongSP.ID = "strong"
	strongSP.SpecialSkill = &SpecialSkill{Duration: 10, ScoreSupport: 160}

	weakSP := makeTimelineCard(80, 25, 10, 1000)
	weakSP.ID = "weak"
	weakSP.SpecialSkill = &SpecialSkill{Duration: 10, ScoreSupport: 40}

	activeCard := makeTimelineCard(120, 25, 10, 1000)
	activeCard.ID = "active"

	dummy1 := makeTimelineCard(60, 25, 10, 1000)
	dummy1.ID = "d1"
	dummy2 := makeTimelineCard(60, 25, 10, 1000)
	dummy2.ID = "d2"

	cardMap := map[string]*Card{
		"strong": strongSP,
		"weak":   weakSP,
		"active": activeCard,
		"d1":     dummy1,
		"d2":     dummy2,
	}

	// SP point 0 has many notes, SP point 1 has few
	timeline := &SongTimeline{
		Duration:      100,
		SpecialPoints: [5]float64{25, 70, 0, 0, 0},
	}

	// Dense notes around SP point 0 (t=25-35), sparse around SP point 1 (t=70-80)
	events := []ScoreEvent{
		{Time: 26, ComboIndex: 50, Weight: 1},
		{Time: 27, ComboIndex: 51, Weight: 1},
		{Time: 28, ComboIndex: 52, Weight: 1},
		{Time: 29, ComboIndex: 53, Weight: 1},
		{Time: 30, ComboIndex: 54, Weight: 1},
		{Time: 31, ComboIndex: 55, Weight: 1},
		{Time: 32, ComboIndex: 56, Weight: 1},
		{Time: 33, ComboIndex: 57, Weight: 1},
		{Time: 34, ComboIndex: 58, Weight: 1},
		{Time: 71, ComboIndex: 200, Weight: 1},
		{Time: 50, ComboIndex: 100, Weight: 1},
	}

	legacyResult := SolveResult{
		Score: EvalResult{
			UnitScore:  800000,
			TotalPower: 200000,
		},
		LeaderIdx: 0,
		TeamIDs:   [5]string{"strong", "weak", "active", "d1", "d2"},
	}

	results := RerankTopN(
		[]SolveResult{legacyResult},
		cardMap,
		timeline,
		events,
		1.0, 0, 100,
		5,
	)

	if len(results) == 0 {
		t.Fatal("no results")
	}

	// The best ordering should place strongSP at slot 0 (SP point at t=25, dense notes)
	best := results[0]
	if best.TeamIDs[0] != "strong" {
		t.Errorf("expected strong SP at slot 0 (dense notes), got %s", best.TeamIDs[0])
	}

	t.Logf("Best order: %v, LiveScoreIndex=%.0f", best.TeamIDs, best.LiveScoreIndex)
}

func TestRerankTopN_Dedup(t *testing.T) {
	// Same team appears in two legacy results (different leader) → only best kept
	card := makeTimelineCard(120, 25, 10, 1000)
	card.ID = "a"
	d := makeTimelineCard(60, 25, 10, 1000)

	cards := []*Card{
		{ID: "a", CenterSkill: card.CenterSkill, Stats: card.Stats, Type: "cute", Group: "holo1"},
		{ID: "b", CenterSkill: d.CenterSkill, Stats: d.Stats, Type: "cute", Group: "holo1"},
		{ID: "c", CenterSkill: d.CenterSkill, Stats: d.Stats, Type: "cute", Group: "holo1"},
		{ID: "d", CenterSkill: d.CenterSkill, Stats: d.Stats, Type: "cute", Group: "holo1"},
		{ID: "e", CenterSkill: d.CenterSkill, Stats: d.Stats, Type: "cute", Group: "holo1"},
	}

	cardMap := map[string]*Card{}
	for _, c := range cards {
		cardMap[c.ID] = c
	}

	timeline := &SongTimeline{Duration: 100, SpecialPoints: [5]float64{20, 40, 60, 80, 90}}
	events := []ScoreEvent{{Time: 50, ComboIndex: 0, Weight: 1}}

	legacyResults := []SolveResult{
		{Score: EvalResult{TotalPower: 100000}, TeamIDs: [5]string{"a", "b", "c", "d", "e"}},
		{Score: EvalResult{TotalPower: 100000}, TeamIDs: [5]string{"b", "a", "c", "d", "e"}},
	}

	results := RerankTopN(legacyResults, cardMap, timeline, events, 1.0, 0, 100, 10)

	// Should have only 1 unique team set (despite 2 inputs with same cards)
	if len(results) != 1 {
		t.Errorf("expected 1 deduped result, got %d", len(results))
	}
}
