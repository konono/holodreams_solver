package main

import (
	"encoding/json"
	"fmt"
	"math"
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

		legacyResult := solve(cards, topN, statScale, baseline, songLength, fixedLeader, costumeOnly, overrideCostumeSkill, input.StabilityLengths)

		// Timeline reranking if chart data is provided
		timeline := input.SongTimeline
		if timeline == nil && input.ChartScoreData != nil {
			timeline = ChartScoreToTimeline(input.ChartScoreData)
		}
		if timeline != nil && len(timeline.ScoreEvents) > 0 {
			cardMap := make(map[string]*Card, len(cards))
			for _, c := range cards {
				cardMap[c.ID] = c
			}

			candidatePool := topN * 10
			if candidatePool < 200 {
				candidatePool = 200
			}
			timelineTopN := topN
			if input.TimelineTopN > 0 {
				timelineTopN = input.TimelineTopN
			}

			legacyPool := solve(cards, candidatePool, statScale, baseline, songLength, fixedLeader, costumeOnly, overrideCostumeSkill, nil)

			var legacySolveResults []SolveResult
			for _, r := range legacyPool.Results {
				team := [5]string{}
				for i, id := range r.MemberIDs {
					team[i] = id
				}
				legacySolveResults = append(legacySolveResults, SolveResult{
					Score: EvalResult{
						UnitScore:  float64(r.UnitScore),
						TotalPower: float64(r.TotalPower),
					},
					LeaderIdx: 0,
					TeamIDs:   team,
				})
			}

			scoreEvents := timeline.ScoreEvents
			if len(scoreEvents) == 0 {
				scoreEvents = BinsToScoreEvents(input.ChartScoreData.Bins)
			}

			reranked := RerankTopN(legacySolveResults, cardMap, timeline, scoreEvents, statScale, baseline, songLength, timelineTopN)

			var timelineResults []TimelineJSONResult
			for i, r := range reranked {
				spEff := make([]float64, 0)
				for _, v := range r.TimelineResult.SPEfficiency {
					spEff = append(spEff, roundTo1(v))
				}
				timelineResults = append(timelineResults, TimelineJSONResult{
					Rank:              i + 1,
					UnitScore:         int(math.Round(r.UnitScore)),
					LiveScoreIndex:    int(math.Round(r.LiveScoreIndex)),
					ActiveOverlapLoss: fixedFloat(r.TimelineResult.ActiveOverlapLoss * 100),
					MemberIDs:         r.TeamIDs[:],
					SPEfficiency:      spEff,
				})
			}

			// Stability: rerank top N across additional songs
			var stability []TimelineStabilityEntry
			if len(input.StabilityCharts) > 0 && len(reranked) > 0 {
				// Use only the top N teams (already determined) for stability
				var topTeams []SolveResult
				for _, r := range reranked {
					topTeams = append(topTeams, SolveResult{
						Score:     EvalResult{UnitScore: r.UnitScore, TotalPower: r.UnitScore},
						LeaderIdx: r.LeaderIdx,
						TeamIDs:   r.TeamIDs,
					})
				}
				for _, sc := range input.StabilityCharts {
					stTL := ChartScoreToTimeline(&sc)
					stEvents := BinsToScoreEvents(sc.Bins)
					if len(stEvents) == 0 {
						continue
					}
					stReranked := RerankTopN(topTeams, cardMap, stTL, stEvents, statScale, baseline, songLength, 1)
					topLSI := 0
					if len(stReranked) > 0 {
						topLSI = int(math.Round(stReranked[0].LiveScoreIndex))
					}
					stability = append(stability, TimelineStabilityEntry{
						MusicID:    sc.MusicID,
						Difficulty: sc.Difficulty,
						Duration:   int(sc.Duration),
						TopLSI:     topLSI,
					})
				}
			}

			return TimelineJSONOutput{
				LegacyResults: legacyResult.Results,
				Timeline:      timelineResults,
				CandidatePool: candidatePool,
				Stability:     stability,
			}, nil
		}

		return legacyResult, nil

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
