package main

import (
	"math"
	"testing"
)

func TestStreamingCostumeSearchMatchesPrecomputedSearch(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	var cards []*Card
	var raws []*CardRaw
	seen := map[string]bool{}
	for i := range cf.Cards {
		raw := &cf.Cards[i]
		if seen[raw.Character] {
			continue
		}
		seen[raw.Character] = true
		card := resolveCard(raw, 0, nil, cf)
		cards = append(cards, &card)
		raws = append(raws, raw)
		if len(cards) == 10 {
			break
		}
	}
	requests := []costumeSearchRequest{
		{key: costumeSearchKey{"new-a", 0}, skill: raws[0].PotentialData[0].CostumeSkill},
		{key: costumeSearchKey{"owned-a", 1}, skill: raws[0].PotentialData[1].CostumeSkill, excludeID: cards[0].ID},
		{key: costumeSearchKey{"owned-b", 1}, skill: raws[1].PotentialData[1].CostumeSkill, excludeID: cards[1].ID},
		{key: costumeSearchKey{"new-b", 0}, skill: raws[0].PotentialData[0].CostumeSkill},
	}
	got := searchCostumeAlternatives(cards, requests, 1, 0, 192)
	bases := precomputeOwnedBases(cards, 1, 0, 192)
	for _, request := range requests {
		var want costumeSearchResult
		for _, pb := range bases {
			excluded := false
			for _, id := range pb.teamIDs {
				if id == request.excludeID && request.excludeID != "" {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
			score, _, _, _, _, _ := applyCostume(&pb.base, &request.skill)
			if score > want.score {
				want = costumeSearchResult{score: score, team: pb.teamIDs, leaderIdx: pb.leaderIdx}
			}
		}
		actual := got[request.key]
		if math.Abs(actual.score-want.score) > 1e-7 || actual.team != want.team || actual.leaderIdx != want.leaderIdx {
			t.Fatalf("%+v: streaming = %+v, precomputed = %+v", request.key, actual, want)
		}
	}
}

func TestRequiredCardCostumeBoundKeepsBestScore(t *testing.T) {
	cf, err := loadCardsFile("../data/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	var cards []*Card
	var costumes []CostumeEntry
	seen := map[string]bool{}
	for i := range cf.Cards {
		raw := &cf.Cards[i]
		if seen[raw.Character] || len(raw.PotentialData) < 2 {
			continue
		}
		seen[raw.Character] = true
		card := resolveCard(raw, 0, nil, cf)
		cards = append(cards, &card)
		costumes = append(costumes, CostumeEntry{raw.ID, raw.PotentialData[1].CostumeSkill})
		if len(cards) == 10 {
			break
		}
	}
	for _, required := range cards[:3] {
		got, _, _, _ := solveWithRequiredCardSweep(cards, required, costumes, 1, 0, 192)
		want := 0.0
		for a := 0; a < len(cards)-3; a++ {
			for b := a + 1; b < len(cards)-2; b++ {
				for c := b + 1; c < len(cards)-1; c++ {
					for d := c + 1; d < len(cards); d++ {
						others := [4]*Card{cards[a], cards[b], cards[c], cards[d]}
						invalid := false
						for _, card := range others {
							if card.Character == required.Character {
								invalid = true
							}
						}
						if invalid {
							continue
						}
						base := computeBaseScores([5]*Card{required, others[0], others[1], others[2], others[3]}, 0, 1, 0, 192)
						for _, costume := range costumes {
							score, _, _, _, _, _ := applyCostume(&base, &costume.Skill)
							want = math.Max(want, score)
						}
					}
				}
			}
		}
		if math.Abs(got-want) > 1e-7 {
			t.Fatalf("required %s: bounded score %f, exhaustive score %f", required.ID, got, want)
		}
	}
}
