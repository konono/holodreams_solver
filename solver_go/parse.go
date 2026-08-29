package main

import (
	"encoding/json"
	"fmt"
)

func parseCardsFromJSON(raw json.RawMessage, cf *CardsFile) []*Card {
	if len(raw) == 0 {
		cards := make([]*Card, len(cf.Cards))
		for i := range cf.Cards {
			c := resolveCard(&cf.Cards[i], 0, nil, cf)
			cards[i] = &c
		}
		return cards
	}

	var ids []string
	if err := json.Unmarshal(raw, &ids); err == nil {
		rawCardMap := map[string]*CardRaw{}
		for i := range cf.Cards {
			rawCardMap[cf.Cards[i].ID] = &cf.Cards[i]
		}
		var cards []*Card
		for _, id := range ids {
			if r, ok := rawCardMap[id]; ok {
				c := resolveCard(r, 0, nil, cf)
				cards = append(cards, &c)
			}
		}
		return cards
	}

	var specs []CardSpec
	if err := json.Unmarshal(raw, &specs); err == nil {
		rawCardMap := map[string]*CardRaw{}
		for i := range cf.Cards {
			rawCardMap[cf.Cards[i].ID] = &cf.Cards[i]
		}
		var cards []*Card
		for _, spec := range specs {
			if r, ok := rawCardMap[spec.ID]; ok {
				c := resolveCard(r, spec.Potential, spec.Level, cf)
				cards = append(cards, &c)
			}
		}
		return cards
	}

	return nil
}

func parseOwnedSpecsFromJSON(raw json.RawMessage) map[string]CardSpec {
	specs := map[string]CardSpec{}

	var ids []string
	if err := json.Unmarshal(raw, &ids); err == nil {
		for _, id := range ids {
			specs[id] = CardSpec{ID: id, Potential: 0}
		}
		return specs
	}

	var cardSpecs []CardSpec
	if err := json.Unmarshal(raw, &cardSpecs); err == nil {
		for _, spec := range cardSpecs {
			specs[spec.ID] = spec
		}
	}
	return specs
}

func dispatchAction(input CLIInput, cf *CardsFile) (interface{}, error) {
	statScale := 1.0
	if input.StatScale != nil {
		statScale = *input.StatScale
	}
	baseline := 0.0
	if input.Baseline != nil {
		baseline = *input.Baseline
	}
	songLength := defaultSongLength
	if input.SongLength != nil {
		songLength = *input.SongLength
	}
	topN := input.TopN
	if topN <= 0 {
		topN = 10
	}

	switch input.Action {
	case "solve":
		cards := parseCardsFromJSON(input.Cards, cf)
		fixedLeader := ""
		if input.FixedLeaderID != nil {
			fixedLeader = *input.FixedLeaderID
		}
		costumeOnly := ""
		if input.CostumeOnlyLeaderID != nil {
			costumeOnly = *input.CostumeOnlyLeaderID
		}
		if fixedLeader != "" && costumeOnly != "" {
			costumeOnly = ""
		}

		if input.SweepCostumes && fixedLeader == "" && costumeOnly == "" {
			rawCardMap := map[string]*CardRaw{}
			for i := range cf.Cards {
				rawCardMap[cf.Cards[i].ID] = &cf.Cards[i]
			}
			return solveSweepCostumes(cards, cf.Cards, rawCardMap, topN, statScale, baseline, songLength, input.StabilityLengths, cf), nil
		}

		var overrideCostumeSkill *CostumeSkill
		if costumeOnly != "" {
			for i := range cf.Cards {
				if cf.Cards[i].ID == costumeOnly && len(cf.Cards[i].PotentialData) > 0 {
					cs := cf.Cards[i].PotentialData[0].CostumeSkill
					overrideCostumeSkill = &cs
					break
				}
			}
		}

		ownedIDs := map[string]bool{}
		for _, c := range cards {
			ownedIDs[c.ID] = true
		}
		if fixedLeader != "" && !ownedIDs[fixedLeader] {
			return JSONOutput{TotalCombinations: 0, StatScale: statScale, Baseline: baseline, Results: []JSONResult{}}, nil
		}

		return solve(cards, topN, statScale, baseline, songLength, fixedLeader, costumeOnly, overrideCostumeSkill, input.StabilityLengths), nil

	case "calibrate":
		if input.CardSpecs == nil {
			input.CardSpecs = map[string]CardSpec{}
		}
		return calibrate(input.MemberIDs, input.LeaderID1, input.GameScore1, input.LeaderID2, input.GameScore2, input.CardSpecs, songLength, cf), nil

	case "recommend":
		ownedSpecs := parseOwnedSpecsFromJSON(input.Cards)
		acquireCount := input.AcquireCount
		if acquireCount <= 0 {
			acquireCount = 1
		}
		fixedLeader := ""
		if input.FixedLeaderID != nil {
			fixedLeader = *input.FixedLeaderID
		}
		costumeOnly := ""
		if input.CostumeOnlyLeaderID != nil {
			costumeOnly = *input.CostumeOnlyLeaderID
		}
		sweepCostumes := input.SweepCostumes
		return recommend(ownedSpecs, cf.Cards, topN, acquireCount, statScale, baseline, songLength, fixedLeader, costumeOnly, sweepCostumes, cf), nil

	default:
		return nil, fmt.Errorf("unknown action: %s", input.Action)
	}
}
