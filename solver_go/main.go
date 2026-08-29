//go:build !js

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}

	var input CLIInput
	if err := json.Unmarshal(data, &input); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON input: %v\n", err)
		os.Exit(1)
	}

	cf, err := loadCardsFile("data/cards.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading cards: %v\n", err)
		os.Exit(1)
	}

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

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	switch input.Action {
	case "solve":
		cards := parseCardsInput(input.Cards, cf)
		fixedLeader := ""
		if input.FixedLeaderID != nil {
			fixedLeader = *input.FixedLeaderID
		}
		costumeOnly := ""
		if input.CostumeOnlyLeaderID != nil {
			costumeOnly = *input.CostumeOnlyLeaderID
		}

		// If both are set, costume_only is ignored
		if fixedLeader != "" && costumeOnly != "" {
			costumeOnly = ""
		}

		if input.SweepCostumes && fixedLeader == "" && costumeOnly == "" {
			rawCardMap := map[string]*CardRaw{}
			for i := range cf.Cards {
				rawCardMap[cf.Cards[i].ID] = &cf.Cards[i]
			}
			result := solveSweepCostumes(cards, cf.Cards, rawCardMap, topN, statScale, baseline, songLength, input.StabilityLengths, cf)
			enc.Encode(result)
			return
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

		// Check owned IDs
		ownedIDs := map[string]bool{}
		for _, c := range cards {
			ownedIDs[c.ID] = true
		}
		if fixedLeader != "" && !ownedIDs[fixedLeader] {
			enc.Encode(JSONOutput{TotalCombinations: 0, StatScale: statScale, Baseline: baseline, Results: []JSONResult{}})
			return
		}

		result := solve(cards, topN, statScale, baseline, songLength, fixedLeader, costumeOnly, overrideCostumeSkill, input.StabilityLengths)
		enc.Encode(result)

	case "calibrate":
		if input.CardSpecs == nil {
			input.CardSpecs = map[string]CardSpec{}
		}
		result := calibrate(input.MemberIDs, input.LeaderID1, input.GameScore1, input.LeaderID2, input.GameScore2, input.CardSpecs, songLength, cf)
		enc.Encode(result)

	case "recommend":
		ownedSpecs := parseOwnedSpecs(input.Cards)
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
		result := recommend(ownedSpecs, cf.Cards, topN, acquireCount, statScale, baseline, songLength, fixedLeader, costumeOnly, cf)
		enc.Encode(result)

	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", input.Action)
		os.Exit(1)
	}
}

func parseCardsInput(raw json.RawMessage, cf *CardsFile) []*Card {
	if len(raw) == 0 {
		// Use all cards at potential 0
		cards := make([]*Card, len(cf.Cards))
		for i := range cf.Cards {
			c := resolveCard(&cf.Cards[i], 0, nil, cf)
			cards[i] = &c
		}
		return cards
	}

	// Try as array of strings first
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

	// Try as array of card specs
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

func parseOwnedSpecs(raw json.RawMessage) map[string]CardSpec {
	specs := map[string]CardSpec{}

	// Try as array of strings
	var ids []string
	if err := json.Unmarshal(raw, &ids); err == nil {
		for _, id := range ids {
			specs[id] = CardSpec{ID: id, Potential: 0}
		}
		return specs
	}

	// Try as array of card specs
	var cardSpecs []CardSpec
	if err := json.Unmarshal(raw, &cardSpecs); err == nil {
		for _, spec := range cardSpecs {
			specs[spec.ID] = spec
		}
		return specs
	}

	return specs
}
