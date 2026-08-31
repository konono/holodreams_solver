package main

import (
	"encoding/json"
	"fmt"
	"math"
)

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

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

func boardOptForReranked(r TimelineRerankResult, cardMap map[string]*Card, timeline *SongTimeline, scoreEvents []ScoreEvent) *BoardOptResult {
	var team [5]*Card
	for i, id := range r.TeamIDs {
		team[i] = cardMap[id]
	}
	return OptimizeBoardForTeam(team, r.TotalPower, timeline.Duration, timeline, scoreEvents, r.AlwaysOnSupport)
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
			// Apply Timeline reranking to sweep results if chart data is provided
			timeline := input.SongTimeline
			if timeline == nil && input.ChartScoreData != nil {
				timeline = ChartScoreToTimeline(input.ChartScoreData)
			}

			hasTimeline := timeline != nil
			sweepTopN := topN
			if hasTimeline {
				scoreEvents := timeline.ScoreEvents
				if len(scoreEvents) == 0 && input.ChartScoreData != nil {
					scoreEvents = BinsToScoreEvents(input.ChartScoreData.Bins)
				}
				hasTimeline = len(scoreEvents) > 0
			}
			candidatePool := topN
			if hasTimeline {
				candidatePool = topN * 10
				if candidatePool < 1000 {
					candidatePool = 1000
				}
			}

			sweepResult := solveSweepCostumes(cards, cf.Cards, rawCardMap, candidatePool, statScale, baseline, songLength, input.StabilityLengths, cf)

			if hasTimeline {
				scoreEvents := timeline.ScoreEvents
				if len(scoreEvents) == 0 && input.ChartScoreData != nil {
					scoreEvents = BinsToScoreEvents(input.ChartScoreData.Bins)
				}
				if len(scoreEvents) > 0 {
					cardMap := make(map[string]*Card, len(cards))
					for _, c := range cards {
						cardMap[c.ID] = c
					}
					sweepPool := sweepResult

					timelineTopN := topN
					if input.TimelineTopN > 0 {
						timelineTopN = input.TimelineTopN
					}

					var legacySolveResults []SolveResult
					for _, r := range sweepPool.Results {
						team := [5]string{}
						for i, id := range r.MemberIDs {
							team[i] = id
						}
						leaderIdx := 0
						for i, id := range r.MemberIDs {
							if id == r.LeaderID {
								leaderIdx = i
								break
							}
						}
						legacySolveResults = append(legacySolveResults, SolveResult{
							Score: EvalResult{
								UnitScore:  float64(r.UnitScore),
								TotalPower: float64(r.TotalPower),
							},
							LeaderIdx:           leaderIdx,
							TeamIDs:             team,
							CostumeOnlyLeaderID: derefStr(r.CostumeOnlyLeaderID),
						})
					}

					reranked := RerankTopN(legacySolveResults, cardMap, timeline, scoreEvents, statScale, baseline, songLength, nil, timelineTopN)

					rawComboSumSweep := 0.0
					for i := range scoreEvents {
						ev := &scoreEvents[i]
						w := ev.Weight
						if w <= 0 { w = 1.0 }
						rawComboSumSweep += w * comboMultiplier(ev.ComboIndex)
					}
					top1LSI := 0.0
					if len(reranked) > 0 { top1LSI = reranked[0].LiveScoreIndex }

	
					if progressCallback != nil {
						progressCallback(-1, -1) // signal: entering timeline rerank phase
					}
					var timelineResults []TimelineJSONResult
					for i, r := range reranked {
						spEff := make([]float64, 0)
						for _, v := range r.TimelineResult.SPEfficiency {
							spEff = append(spEff, roundTo1(v))
						}
						skillEff := 0.0
						noSkillLSI := r.TotalPower * rawComboSumSweep
						if noSkillLSI > 0 { skillEff = r.LiveScoreIndex / noSkillLSI }
						top1Pct := 0.0
						if top1LSI > 0 { top1Pct = r.LiveScoreIndex / top1LSI * 100 }
						var costumePtr *string
						if r.CostumeOnlyLeaderID != "" {
							s := r.CostumeOnlyLeaderID
							costumePtr = &s
						}
						var boardOpt *BoardOptResult
						if i < 10 {
							boardOpt = boardOptForReranked(r, cardMap, timeline, scoreEvents)
						}
						timelineResults = append(timelineResults, TimelineJSONResult{
							Rank:                i + 1,
							UnitScore:           int(math.Round(r.UnitScore)),
							TotalPower:          int(math.Round(r.TotalPower)),
							LiveScoreIndex:      int(math.Round(r.LiveScoreIndex)),
							SkillEfficiency:     fixedFloat2(skillEff),
							Top1Pct:             fixedFloat2(top1Pct),
							ActiveOverlapLoss:   fixedFloat(r.TimelineResult.ActiveOverlapLoss * 100),
							ExpectedActive:      fixedFloat(r.TimelineResult.ExpectedActive),
							CostumeSBPct:        fixedFloat(r.CostumeSBPct),
							PassiveSBPct:        fixedFloat(r.PassiveSBPct),
							SpecialPct:          fixedFloat(r.SpecialPct),
							CostumeOnlyLeaderID: costumePtr,
							MemberIDs:           r.TeamIDs[:],
							SPEfficiency:        spEff,
							BoardOptimization:   boardOpt,
						})
					}
					legacyResults := sweepPool.Results
					if len(legacyResults) > sweepTopN {
						legacyResults = legacyResults[:sweepTopN]
					}
					return TimelineJSONOutput{
						LegacyResults: legacyResults,
						Timeline:      timelineResults,
						CandidatePool: candidatePool,
					}, nil
				}
			}

			// Non-timeline: truncate to topN
			if len(sweepResult.Results) > sweepTopN {
				sweepResult.Results = sweepResult.Results[:sweepTopN]
			}
			return sweepResult, nil
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

		// Timeline reranking if chart data is provided
		timeline := input.SongTimeline
		if timeline == nil && input.ChartScoreData != nil {
			timeline = ChartScoreToTimeline(input.ChartScoreData)
		}
		useTimeline := timeline != nil && len(timeline.ScoreEvents) > 0

		solveTopN := topN
		if useTimeline {
			solveTopN = topN * 10
			if solveTopN < 1000 {
				solveTopN = 1000
			}
		}

		legacyResult := solve(cards, solveTopN, statScale, baseline, songLength, fixedLeader, costumeOnly, overrideCostumeSkill, input.StabilityLengths)

		if useTimeline {
			cardMap := make(map[string]*Card, len(cards))
			for _, c := range cards {
				cardMap[c.ID] = c
			}

			candidatePool := solveTopN
			timelineTopN := topN
			if input.TimelineTopN > 0 {
				timelineTopN = input.TimelineTopN
			}

			var legacySolveResults []SolveResult
			for _, r := range legacyResult.Results {
				team := [5]string{}
				for i, id := range r.MemberIDs {
					team[i] = id
				}
				leaderIdx := 0
				for i, id := range r.MemberIDs {
					if id == r.LeaderID {
						leaderIdx = i
						break
					}
				}
				legacySolveResults = append(legacySolveResults, SolveResult{
					Score: EvalResult{
						UnitScore:  float64(r.UnitScore),
						TotalPower: float64(r.TotalPower),
					},
					LeaderIdx:           leaderIdx,
					TeamIDs:             team,
					CostumeOnlyLeaderID: derefStr(r.CostumeOnlyLeaderID),
				})
			}

			scoreEvents := timeline.ScoreEvents
			if len(scoreEvents) == 0 {
				scoreEvents = BinsToScoreEvents(input.ChartScoreData.Bins)
			}

			reranked := RerankTopN(legacySolveResults, cardMap, timeline, scoreEvents, statScale, baseline, songLength, overrideCostumeSkill, timelineTopN)

			// rawComboSum: sum of (noteWeight × comboMultiplier) with no skills
			rawComboSum := 0.0
			for i := range scoreEvents {
				ev := &scoreEvents[i]
				w := ev.Weight
				if w <= 0 {
					w = 1.0
				}
				rawComboSum += w * comboMultiplier(ev.ComboIndex)
			}

			top1LSI := 0.0
			if len(reranked) > 0 {
				top1LSI = reranked[0].LiveScoreIndex
			}

	
			if progressCallback != nil {
				progressCallback(-1, -1)
			}
			var timelineResults []TimelineJSONResult
			for i, r := range reranked {
				spEff := make([]float64, 0)
				for _, v := range r.TimelineResult.SPEfficiency {
					spEff = append(spEff, roundTo1(v))
				}
				skillEff := 0.0
				noSkillLSI := r.TotalPower * rawComboSum
				if noSkillLSI > 0 {
					skillEff = r.LiveScoreIndex / noSkillLSI
				}
				top1Pct := 0.0
				if top1LSI > 0 {
					top1Pct = r.LiveScoreIndex / top1LSI * 100
				}
				var costumePtr *string
				if r.CostumeOnlyLeaderID != "" {
					s := r.CostumeOnlyLeaderID
					costumePtr = &s
				}
				var boardOpt *BoardOptResult
				if i < 10 {
					boardOpt = boardOptForReranked(r, cardMap, timeline, scoreEvents)
				}
				timelineResults = append(timelineResults, TimelineJSONResult{
					Rank:                i + 1,
					UnitScore:           int(math.Round(r.UnitScore)),
					TotalPower:          int(math.Round(r.TotalPower)),
					LiveScoreIndex:      int(math.Round(r.LiveScoreIndex)),
					SkillEfficiency:     fixedFloat2(skillEff),
					Top1Pct:             fixedFloat2(top1Pct),
					ActiveOverlapLoss:   fixedFloat(r.TimelineResult.ActiveOverlapLoss * 100),
					ExpectedActive:      fixedFloat(r.TimelineResult.ExpectedActive),
					CostumeSBPct:        fixedFloat(r.CostumeSBPct),
					PassiveSBPct:        fixedFloat(r.PassiveSBPct),
					SpecialPct:          fixedFloat(r.SpecialPct),
					CostumeOnlyLeaderID: costumePtr,
					MemberIDs:           r.TeamIDs[:],
					SPEfficiency:        spEff,
					BoardOptimization:   boardOpt,
				})
			}

			// Stability: rerank top N across additional songs
			var stability []TimelineStabilityEntry
			if len(input.StabilityCharts) > 0 && len(reranked) > 0 {
				// Use only the top N teams (already determined) for stability
				var topTeams []SolveResult
				for _, r := range reranked {
					topTeams = append(topTeams, SolveResult{
						Score:     EvalResult{UnitScore: r.UnitScore},
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
					stReranked := RerankTopN(topTeams, cardMap, stTL, stEvents, statScale, baseline, songLength, overrideCostumeSkill, 1)
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

			legacyForDisplay := legacyResult.Results
			if len(legacyForDisplay) > topN {
				legacyForDisplay = legacyForDisplay[:topN]
			}
			return TimelineJSONOutput{
				LegacyResults: legacyForDisplay,
				Timeline:      timelineResults,
				CandidatePool: candidatePool,
				Stability:     stability,
			}, nil
		}

		// Non-timeline: truncate to topN
		if len(legacyResult.Results) > topN {
			legacyResult.Results = legacyResult.Results[:topN]
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
