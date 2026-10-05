package main

import (
	"math"
	"time"
)

// timelineRecommendContext compares recommendations using the same song chart
// and Board mode as solve. Candidate teams are first selected by the legacy
// optimizer, so this remains an approximate search over team compositions.
type timelineRecommendContext struct {
	timeline           *SongTimeline
	events             []ScoreEvent
	boardMode          string
	rawCards           map[string]*CardRaw
	cf                 *CardsFile
	statScale          float64
	baseline           float64
	songLength         float64
	overrideCostume    *CostumeSkill
	baseScore          int
	legacyBaseScore    int
	baselineSolveMs    int64
	baselineTimelineMs int64
}

func newTimelineRecommendContext(input CLIInput, ownedSpecs map[string]CardSpec, allRawCards []CardRaw, statScale, baseline, songLength float64, fixedLeaderID, costumeOnlyLeaderID string, sweepCostumes bool, cf *CardsFile) *timelineRecommendContext {
	timeline := input.SongTimeline
	if timeline == nil && input.ChartScoreData != nil {
		timeline = ChartScoreToTimeline(input.ChartScoreData)
	}
	if timeline == nil {
		return nil
	}
	events := timeline.ScoreEvents
	if len(events) == 0 && input.ChartScoreData != nil {
		events = BinsToScoreEvents(input.ChartScoreData.Bins)
	}
	if len(events) == 0 {
		return nil
	}
	rawMap := make(map[string]*CardRaw, len(allRawCards))
	for i := range allRawCards {
		rawMap[allRawCards[i].ID] = &allRawCards[i]
	}
	ctx := &timelineRecommendContext{
		timeline: timeline, events: events, boardMode: input.BoardSearchMode,
		rawCards: rawMap, cf: cf, statScale: statScale, baseline: baseline, songLength: songLength,
	}
	if ctx.boardMode == "" {
		ctx.boardMode = "balanced"
	}
	if fixedLeaderID != "" {
		costumeOnlyLeaderID = ""
	}
	if raw := rawMap[costumeOnlyLeaderID]; raw != nil && len(raw.PotentialData) > 0 {
		potential := 0
		if spec, ok := ownedSpecs[costumeOnlyLeaderID]; ok {
			potential = max(0, min(spec.Potential, len(raw.PotentialData)-1))
		}
		skill := raw.PotentialData[potential].CostumeSkill
		ctx.overrideCostume = &skill
	}
	baseCards := ctx.resolveCards(ownedSpecs)
	poolSize := 1000
	var legacy []JSONResult
	reportStage("baseline", 0, 0)
	baselineStarted := time.Now()
	if sweepCostumes && fixedLeaderID == "" && costumeOnlyLeaderID == "" {
		result := solveSweepCostumes(baseCards, allRawCards, rawMap, poolSize, statScale, baseline, songLength, nil, cf)
		if len(result.Results) == 0 {
			return nil
		}
		legacy = result.Results
	} else {
		result := solve(baseCards, poolSize, statScale, baseline, songLength, fixedLeaderID, costumeOnlyLeaderID, ctx.overrideCostume, nil)
		if len(result.Results) == 0 {
			return nil
		}
		legacy = result.Results
	}
	ctx.baselineSolveMs = time.Since(baselineStarted).Milliseconds()
	ctx.legacyBaseScore = legacy[0].UnitScore
	reportStage("baseline_timeline", 0, 0)
	timelineStarted := time.Now()
	cardMap := make(map[string]*Card, len(baseCards)+1)
	for _, card := range baseCards {
		cardMap[card.ID] = card
	}
	if costumeOnlyLeaderID != "" && cardMap[costumeOnlyLeaderID] == nil {
		if raw := rawMap[costumeOnlyLeaderID]; raw != nil {
			card := resolveCard(raw, 0, nil, cf)
			cardMap[card.ID] = &card
		}
	}
	legacyResults := make([]SolveResult, 0, len(legacy))
	for _, result := range legacy {
		var ids [5]string
		copy(ids[:], result.MemberIDs)
		leaderIdx := 0
		for i, id := range result.MemberIDs {
			if id == result.LeaderID {
				leaderIdx = i
				break
			}
		}
		legacyResults = append(legacyResults, SolveResult{TeamIDs: ids, LeaderIdx: leaderIdx, CostumeOnlyLeaderID: derefStr(result.CostumeOnlyLeaderID)})
	}
	rerankN := len(legacyResults)
	if ctx.boardMode == "fast" {
		rerankN = 1
	}
	reranked := RerankTopN(legacyResults, cardMap, timeline, events, statScale, baseline, songLength, ctx.overrideCostume, rerankN)
	if len(reranked) == 0 {
		return nil
	}
	if ctx.boardMode != "fast" {
		reranked, _ = RerankBoardAware(reranked, cardMap, timeline, events, 1, ctx.boardMode)
	}
	ctx.baseScore = int(math.Round(reranked[0].LiveScoreIndex))
	ctx.baselineTimelineMs = time.Since(timelineStarted).Milliseconds()
	return ctx
}

func (ctx *timelineRecommendContext) resolveCards(specs map[string]CardSpec) []*Card {
	cards := make([]*Card, 0, len(specs))
	for _, spec := range specs {
		if raw := ctx.rawCards[spec.ID]; raw != nil {
			card := resolveCard(raw, spec.Potential, spec.Level, ctx.cf)
			cards = append(cards, &card)
		}
	}
	return cards
}

func (ctx *timelineRecommendContext) scoreTeam(team RecommendBestTeam, specs map[string]CardSpec) (int, RecommendBestTeam) {
	if len(team.MemberIDs) != 5 {
		return 0, RecommendBestTeam{}
	}
	cardMap := make(map[string]*Card, len(specs)+1)
	for _, card := range ctx.resolveCards(specs) {
		cardMap[card.ID] = card
	}
	if team.CostumeOnlyLeaderID != nil {
		id := *team.CostumeOnlyLeaderID
		if cardMap[id] == nil {
			if raw := ctx.rawCards[id]; raw != nil {
				card := resolveCard(raw, 0, nil, ctx.cf)
				cardMap[id] = &card
			}
		}
	}
	var ids [5]string
	leaderIdx := -1
	for i, id := range team.MemberIDs {
		if cardMap[id] == nil {
			return 0, RecommendBestTeam{}
		}
		ids[i] = id
		if id == team.LeaderID {
			leaderIdx = i
		}
	}
	if leaderIdx < 0 {
		return 0, RecommendBestTeam{}
	}
	legacy := SolveResult{TeamIDs: ids, LeaderIdx: leaderIdx, CostumeOnlyLeaderID: derefStr(team.CostumeOnlyLeaderID)}
	reranked := RerankTopN([]SolveResult{legacy}, cardMap, ctx.timeline, ctx.events, ctx.statScale, ctx.baseline, ctx.songLength, ctx.overrideCostume, 1)
	if len(reranked) == 0 {
		return 0, RecommendBestTeam{}
	}
	if ctx.boardMode != "fast" {
		reranked, _ = RerankBoardAware(reranked, cardMap, ctx.timeline, ctx.events, 1, ctx.boardMode)
	}
	best := reranked[0]
	var costume *string
	if best.CostumeOnlyLeaderID != "" {
		id := best.CostumeOnlyLeaderID
		costume = &id
	}
	return int(math.Round(best.LiveScoreIndex)), RecommendBestTeam{
		LeaderID: best.TeamIDs[best.LeaderIdx], MemberIDs: best.TeamIDs[:], CostumeOnlyLeaderID: costume,
	}
}

func rerankPotentialByTimeline(profiles []PotentialCard, ownedSpecs map[string]CardSpec, newCardLevel *int, ctx *timelineRecommendContext) {
	outerProgress := progressCallback
	progressCallback = nil
	defer func() { progressCallback = outerProgress }()
	total := 0
	for _, profile := range profiles {
		total += len(profile.Steps)
	}
	done := 0
	reportStage("timeline", 0, total)
	for i := range profiles {
		profile := &profiles[i]
		for j := range profile.Steps {
			step := &profile.Steps[j]
			trialSpecs := make(map[string]CardSpec, len(ownedSpecs)+1)
			for id, spec := range ownedSpecs {
				trialSpecs[id] = spec
			}
			if profile.CurrentPotential == nil {
				trialSpecs[profile.CardID] = CardSpec{ID: profile.CardID, Potential: step.TargetPotential, Level: newCardLevel}
			} else {
				spec := trialSpecs[profile.CardID]
				spec.Potential = step.TargetPotential
				trialSpecs[profile.CardID] = spec
			}
			bestScore := ctx.baseScore
			var bestTeam RecommendBestTeam
			role := "unused"
			for _, candidateTeam := range step.CandidateTeams {
				score, team := ctx.scoreTeam(candidateTeam, trialSpecs)
				if score <= bestScore {
					continue
				}
				bestScore = score
				bestTeam = team
				role = "costume"
				for _, id := range team.MemberIDs {
					if id == profile.CardID {
						role = "member"
						if team.LeaderID == id {
							role = "leader"
						}
						break
					}
				}
			}
			step.NewScore = bestScore
			step.Delta = bestScore - ctx.baseScore
			step.BestTeam = bestTeam
			step.Role = role
			done++
			if stageCallback != nil {
				reportStage("timeline", done, total)
			} else if outerProgress != nil {
				outerProgress(done, total)
			}
		}
	}
	finalizePotentialProfiles(profiles)
}
