package main

import (
	"fmt"
	"math"
	"sort"
)

const defaultSongLength = 192.0

// sweepPruneTrigger: プルーニング発動の倍率。バッファが topN*sweepPruneTrigger を超えたらソート＆刈り込み。
// sweepPruneKeep: 刈り込み後の保持倍率。topN*sweepPruneKeep 件を残す。
// unitScore基準のプルーニングでLSI上位候補を取りこぼさないよう余裕を持たせている。
const (
	sweepPruneTrigger = 50
	sweepPruneKeep    = 25
)

var progressCallback func(current, total int)

func comb(n, k int) int {
	if k > n {
		return 0
	}
	r := 1
	for i := 0; i < k; i++ {
		r = r * (n - i) / (i + 1)
	}
	return r
}

func solve(cards []*Card, topN int, statScale, baseline, songLength float64, fixedLeaderID string, costumeOnlyLeaderID string, overrideCostumeSkill *CostumeSkill, stabilityLengths []float64) JSONOutput {
	if len(cards) < 5 {
		return JSONOutput{TotalCombinations: 0, StatScale: statScale, Baseline: baseline}
	}

	cardMap := make(map[string]*Card, len(cards))
	for _, c := range cards {
		cardMap[c.ID] = c
	}

	// Group cards by character
	charGroups := map[string][]*Card{}
	for _, c := range cards {
		charGroups[c.Character] = append(charGroups[c.Character], c)
	}

	type charEntry struct {
		name    string
		maxTotal float64
	}
	charEntries := make([]charEntry, 0, len(charGroups))
	for name, group := range charGroups {
		maxT := 0.0
		for _, c := range group {
			if c.Total > maxT {
				maxT = c.Total
			}
		}
		charEntries = append(charEntries, charEntry{name, maxT})
	}
	sort.Slice(charEntries, func(i, j int) bool {
		if charEntries[i].maxTotal != charEntries[j].maxTotal {
			return charEntries[i].maxTotal > charEntries[j].maxTotal
		}
		return charEntries[i].name < charEntries[j].name
	})
	charNames := make([]string, len(charEntries))
	for i, e := range charEntries {
		charNames[i] = e.name
	}
	nChars := len(charNames)
	if nChars < 5 {
		return JSONOutput{TotalCombinations: 0, StatScale: statScale, Baseline: baseline}
	}

	results := make([]SolveResult, 0, topN*10)
	totalCombos := 0
	charComboCount := 0

	if fixedLeaderID != "" {
		leaderCard, ok := cardMap[fixedLeaderID]
		if !ok {
			return JSONOutput{TotalCombinations: 0, StatScale: statScale, Baseline: baseline}
		}
		leaderChar := leaderCard.Character
		otherChars := make([]string, 0, nChars-1)
		for _, ch := range charNames {
			if ch != leaderChar {
				otherChars = append(otherChars, ch)
			}
		}
		nOther := len(otherChars)
		charComboCount = comb(nOther, 4)
		charCombosDone := 0
		progressStep := charComboCount / 200
		if progressStep < 1 {
			progressStep = 1
		}
		for a := 0; a < nOther-3; a++ {
			for b := a + 1; b < nOther-2; b++ {
				for c := b + 1; c < nOther-1; c++ {
					for d := c + 1; d < nOther; d++ {
						charCombosDone++
						if progressCallback != nil && charCombosDone%progressStep == 0 {
							progressCallback(charCombosDone, charComboCount)
						}
						lists := [4][]*Card{
							charGroups[otherChars[a]],
							charGroups[otherChars[b]],
							charGroups[otherChars[c]],
							charGroups[otherChars[d]],
						}
						for _, c0 := range lists[0] {
							for _, c1 := range lists[1] {
								for _, c2 := range lists[2] {
									for _, c3 := range lists[3] {
										team := [5]*Card{leaderCard, c0, c1, c2, c3}
										totalCombos++
										score := evaluateTeam(team, 0, statScale, baseline, songLength, overrideCostumeSkill)
										results = append(results, SolveResult{
											Score:               score,
											LeaderIdx:           0,
											TeamIDs:             [5]string{team[0].ID, team[1].ID, team[2].ID, team[3].ID, team[4].ID},
											CostumeOnlyLeaderID: costumeOnlyLeaderID,
										})
										if len(results) > topN*10 {
											sort.Slice(results, func(i, j int) bool {
												return results[i].Score.UnitScore > results[j].Score.UnitScore
											})
											results = results[:topN]
										}
									}
								}
							}
						}
					}
				}
			}
		}
	} else {
		charComboCount = comb(nChars, 5)
		charCombosDone := 0
		progressStep := charComboCount / 200
		if progressStep < 1 {
			progressStep = 1
		}
		for a := 0; a < nChars-4; a++ {
			for b := a + 1; b < nChars-3; b++ {
				for ci := b + 1; ci < nChars-2; ci++ {
					for d := ci + 1; d < nChars-1; d++ {
						for e := d + 1; e < nChars; e++ {
							charCombosDone++
							if progressCallback != nil && charCombosDone%progressStep == 0 {
								progressCallback(charCombosDone, charComboCount)
							}
							lists := [5][]*Card{
								charGroups[charNames[a]],
								charGroups[charNames[b]],
								charGroups[charNames[ci]],
								charGroups[charNames[d]],
								charGroups[charNames[e]],
							}
							for _, c0 := range lists[0] {
								for _, c1 := range lists[1] {
									for _, c2 := range lists[2] {
										for _, c3 := range lists[3] {
											for _, c4 := range lists[4] {
												team := [5]*Card{c0, c1, c2, c3, c4}
												totalCombos++

												var bestScore float64
												bestLeader := 0
												var bestResult EvalResult
												if overrideCostumeSkill != nil {
													score := evaluateTeam(team, 0, statScale, baseline, songLength, overrideCostumeSkill)
													bestScore = score.UnitScore
													bestResult = score
												} else {
													base := computeBaseScores(team, 0, statScale, baseline, songLength)
													for li := 0; li < 5; li++ {
														cs := &team[li].CostumeSkill
														us, tp, sb, csbp, csv, cc := applyCostume(&base, cs)
														if us > bestScore {
															bestScore = us
															bestLeader = li
															bestResult = EvalResult{
																UnitScore:      us,
																TotalPower:     tp,
																MemberParams:   base.MemberParams,
																CostumeContrib: cc,
																SupportContrib: base.SupportContrib,
																ActivePct:      base.ActivePct,
																CostumeSBPct:   csbp,
																PassiveSBPct:   base.PassiveSBPct,
																SpecialPct:     base.SpecialPct,
																ScoreBonus:     sb,
																CostumeSSVal:   csv,
																SupportSSVal:   base.SupportSS,
															}
														}
													}
												}
												results = append(results, SolveResult{
													Score:               bestResult,
													LeaderIdx:           bestLeader,
													TeamIDs:             [5]string{c0.ID, c1.ID, c2.ID, c3.ID, c4.ID},
													CostumeOnlyLeaderID: costumeOnlyLeaderID,
												})
												if len(results) > topN*10 {
													sort.Slice(results, func(i, j int) bool {
														return results[i].Score.UnitScore > results[j].Score.UnitScore
													})
													results = results[:topN]
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if progressCallback != nil {
		progressCallback(charComboCount, charComboCount)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score.UnitScore > results[j].Score.UnitScore
	})
	if len(results) > topN {
		results = results[:topN]
	}

	results = optimizeResults(results, cardMap, statScale, baseline, songLength, overrideCostumeSkill, fixedLeaderID)

	return formatOutput(results, totalCombos, statScale, baseline, costumeOnlyLeaderID, stabilityLengths, cardMap, songLength, overrideCostumeSkill)
}

func solveSweepCostumes(cards []*Card, allRawCards []CardRaw, cardMap map[string]*CardRaw, topN int, statScale, baseline, songLength float64, stabilityLengths []float64, cf *CardsFile) JSONOutput {
	if len(cards) < 5 {
		return JSONOutput{TotalCombinations: 0, StatScale: statScale, Baseline: baseline}
	}

	resolvedMap := map[string]*Card{}
	for _, c := range cards {
		resolvedMap[c.ID] = c
	}

	charGroups := map[string][]*Card{}
	for _, c := range cards {
		charGroups[c.Character] = append(charGroups[c.Character], c)
	}

	type charEntry struct {
		name     string
		maxTotal float64
	}
	charEntries := make([]charEntry, 0, len(charGroups))
	for name, group := range charGroups {
		maxT := 0.0
		for _, c := range group {
			if c.Total > maxT {
				maxT = c.Total
			}
		}
		charEntries = append(charEntries, charEntry{name, maxT})
	}
	sort.Slice(charEntries, func(i, j int) bool {
		if charEntries[i].maxTotal != charEntries[j].maxTotal {
			return charEntries[i].maxTotal > charEntries[j].maxTotal
		}
		return charEntries[i].name < charEntries[j].name
	})
	charNames := make([]string, len(charEntries))
	for i, e := range charEntries {
		charNames[i] = e.name
	}
	nChars := len(charNames)
	if nChars < 5 {
		return JSONOutput{TotalCombinations: 0, StatScale: statScale, Baseline: baseline}
	}

	// Collect costume skills from owned cards and prune dominated ones
	ownedIDs := map[string]bool{}
	for _, c := range cards {
		ownedIDs[c.ID] = true
	}
	var rawCostumes []CostumeEntry
	for i := range allRawCards {
		raw := &allRawCards[i]
		if !ownedIDs[raw.ID] {
			continue
		}
		if len(raw.PotentialData) > 0 {
			rawCostumes = append(rawCostumes, CostumeEntry{raw.ID, raw.PotentialData[0].CostumeSkill})
		}
	}
	costumeSkills := pruneCostumes(rawCostumes)

	type sweepResult struct {
		unitScore           float64
		totalPower          float64
		scoreBonus          float64
		activePct           float64
		costumeSBPct        float64
		passiveSBPct        float64
		specialPct          float64
		costumeSSVal        float64
		supportSS           float64
		leaderIdx           int
		teamIDs             [5]string
		costumeOnlyLeaderID string
	}

	sweepResults := make([]sweepResult, 0, topN*sweepPruneTrigger)
	totalCombos := 0
	charComboCount := comb(nChars, 5)
	charCombosDone := 0
	progressStep := charComboCount / 200
	if progressStep < 1 {
		progressStep = 1
	}

	for a := 0; a < nChars-4; a++ {
		for b := a + 1; b < nChars-3; b++ {
			for ci := b + 1; ci < nChars-2; ci++ {
				for d := ci + 1; d < nChars-1; d++ {
					for e := d + 1; e < nChars; e++ {
						charCombosDone++
						if progressCallback != nil && charCombosDone%progressStep == 0 {
							progressCallback(charCombosDone, charComboCount)
						}
						lists := [5][]*Card{
							charGroups[charNames[a]],
							charGroups[charNames[b]],
							charGroups[charNames[ci]],
							charGroups[charNames[d]],
							charGroups[charNames[e]],
						}
						for _, c0 := range lists[0] {
							for _, c1 := range lists[1] {
								for _, c2 := range lists[2] {
									for _, c3 := range lists[3] {
										for _, c4 := range lists[4] {
											team := [5]*Card{c0, c1, c2, c3, c4}
											totalCombos++

											// leaderIdx=0 is arbitrary: computeBaseScores uses emptyCostume,
											// so the result is identical for all leaders.
											base := computeBaseScores(team, 0, statScale, baseline, songLength)
											bestLeaderIdx := 0

											teamIDs := [5]string{c0.ID, c1.ID, c2.ID, c3.ID, c4.ID}
											for _, ce := range costumeSkills {
												us, tp, sb, csbp, csv, _ := applyCostume(&base, &ce.Skill)
												sweepResults = append(sweepResults, sweepResult{
													unitScore:           us,
													totalPower:          tp,
													scoreBonus:          sb,
													activePct:           base.ActivePct,
													costumeSBPct:        csbp,
													passiveSBPct:        base.PassiveSBPct,
													specialPct:          base.SpecialPct,
													costumeSSVal:        csv,
													supportSS:           base.SupportSS,
													leaderIdx:           bestLeaderIdx,
													teamIDs:             teamIDs,
													costumeOnlyLeaderID: ce.CardID,
												})
											}
											if len(sweepResults) > topN*sweepPruneTrigger {
												sort.Slice(sweepResults, func(i, j int) bool {
													return sweepResults[i].unitScore > sweepResults[j].unitScore
												})
												sweepResults = sweepResults[:topN*sweepPruneKeep]
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if progressCallback != nil {
		progressCallback(charComboCount, charComboCount)
	}

	sort.Slice(sweepResults, func(i, j int) bool {
		return sweepResults[i].unitScore > sweepResults[j].unitScore
	})
	if len(sweepResults) > topN {
		sweepResults = sweepResults[:topN]
	}

	// Post-optimize: try all 120 permutations (5 leaders × 24 orderings) for each result
	for ri := range sweepResults {
		r := &sweepResults[ri]
		costumeID := r.costumeOnlyLeaderID
		var cs *CostumeSkill
		for _, ce := range costumeSkills {
			if ce.CardID == costumeID {
				cs = &ce.Skill
				break
			}
		}
		cards5 := [5]*Card{}
		ids5 := r.teamIDs
		for ti, id := range ids5 {
			cards5[ti] = resolvedMap[id]
		}
		for li := 0; li < 5; li++ {
			leaderCard := cards5[li]
			var others [4]*Card
			var otherIDs [4]string
			oi := 0
			for ti := 0; ti < 5; ti++ {
				if ti != li {
					others[oi] = cards5[ti]
					otherIDs[oi] = ids5[ti]
					oi++
				}
			}
			for _, perm := range perms4 {
				team := [5]*Card{leaderCard, others[perm[0]], others[perm[1]], others[perm[2]], others[perm[3]]}
				score := evaluateTeam(team, 0, statScale, baseline, songLength, cs)
				if score.UnitScore > r.unitScore {
					r.unitScore = score.UnitScore
					r.totalPower = score.TotalPower
					r.scoreBonus = score.ScoreBonus
					r.activePct = score.ActivePct
					r.costumeSBPct = score.CostumeSBPct
					r.passiveSBPct = score.PassiveSBPct
					r.specialPct = score.SpecialPct
					r.costumeSSVal = score.CostumeSSVal
					r.supportSS = score.SupportSSVal
					r.leaderIdx = 0
					r.teamIDs = [5]string{leaderCard.ID, otherIDs[perm[0]], otherIDs[perm[1]], otherIDs[perm[2]], otherIDs[perm[3]]}
				}
			}
		}
	}

	sort.Slice(sweepResults, func(i, j int) bool {
		return sweepResults[i].unitScore > sweepResults[j].unitScore
	})

	// Format output
	jsonResults := make([]JSONResult, len(sweepResults))
	for i, r := range sweepResults {
		costumeID := r.costumeOnlyLeaderID
		entry := JSONResult{
			Rank:                i + 1,
			UnitScore:           int(math.Round(r.unitScore)),
			TotalPower:          int(math.Round(r.totalPower)),
			ScoreBonus:          fixedFloat(roundTo1(r.scoreBonus)),
			ActivePct:           fixedFloat(roundTo1(r.activePct)),
			CostumeSBPct:        fixedFloat(roundTo1(r.costumeSBPct)),
			PassiveSBPct:        fixedFloat(roundTo1(r.passiveSBPct)),
			SpecialPct:          fixedFloat(roundTo1(r.specialPct)),
			LeaderID:            r.teamIDs[r.leaderIdx],
			CostumeOnlyLeaderID: &costumeID,
			MemberIDs:           r.teamIDs[:],
		}
		if len(stabilityLengths) > 0 {
			var cs *CostumeSkill
			for _, ce := range costumeSkills {
				if ce.CardID == costumeID {
					cs = &ce.Skill
					break
				}
			}
			team := [5]*Card{}
			for ti, id := range r.teamIDs {
				team[ti] = resolvedMap[id]
			}
			entry.Stability = map[string]int{}
			for _, sl := range stabilityLengths {
				s := evaluateTeam(team, r.leaderIdx, statScale, baseline, sl, cs)
				entry.Stability[fmt.Sprintf("%g", sl)] = int(math.Round(s.UnitScore))
			}
		}
		jsonResults[i] = entry
	}

	return JSONOutput{
		TotalCombinations: totalCombos * len(costumeSkills),
		StatScale:         statScale,
		Baseline:          baseline,
		Results:           jsonResults,
	}
}

func calibrate(memberIDs []string, leaderID1 string, gameScore1 int, leaderID2 string, gameScore2 int, cardSpecs map[string]CardSpec, songLength float64, cf *CardsFile) CalibrateOutput {
	allRawCards, _ := loadCards("data/cards.json")
	rawCardMap := map[string]*CardRaw{}
	for i := range allRawCards {
		rawCardMap[allRawCards[i].ID] = &allRawCards[i]
	}

	specs := cardSpecs
	if specs == nil {
		specs = map[string]CardSpec{}
	}

	team := [5]*Card{}
	for i, mid := range memberIDs {
		raw := rawCardMap[mid]
		spec := specs[mid]
		c := resolveCard(raw, spec.Potential, spec.Level, cf)
		team[i] = &c
	}

	li1 := -1
	li2 := -1
	for i, mid := range memberIDs {
		if mid == leaderID1 {
			li1 = i
		}
		if mid == leaderID2 {
			li2 = i
		}
	}

	emptyCostume := &CostumeSkill{Effects: nil}

	s1 := evaluateTeam(team, li1, 1.0, 0, songLength, nil)
	s2 := evaluateTeam(team, li2, 1.0, 0, songLength, nil)

	// Get costume-independent data using empty costume
	s1NoCostume := evaluateTeam(team, li1, 1.0, 0, songLength, emptyCostume)
	_ = s1NoCostume

	rawPerf := 0.0
	rawTech := 0.0
	rawSense := 0.0
	for _, c := range team {
		rawPerf += c.Stats.Performance
		rawTech += c.Stats.Technique
		rawSense += c.Stats.Sense
	}
	rawTotal := rawPerf + rawTech + rawSense

	// Get support bonus from team with stat_scale=1
	supportRaw := s1.SupportContrib // already at statScale=1

	unit1Target := float64(gameScore1) / ((1 + s1.ScoreBonus/100) * unitScoreK)
	unit2Target := float64(gameScore2) / ((1 + s2.ScoreBonus/100) * unitScoreK)

	costume1 := s1.CostumeContrib
	costume2 := s2.CostumeContrib
	costumeDiff := costume1 - costume2

	result := CalibrateOutput{}
	if math.Abs(costumeDiff) < 1 {
		result.StatScale = 1.0
		result.Baseline = math.Round(unit1Target - (rawTotal + costume1 + supportRaw))
		result.Warnings = []string{"衣装バフの差が小さいリーダーの組み合わせです。精度が低い可能性があります。異なる衣装バフ率のリーダーで再キャリブレーションを推奨します。"}
	} else {
		statScale := (unit1Target - unit2Target) / costumeDiff
		baseline := unit1Target - (rawTotal+costume1+supportRaw)*statScale
		result.StatScale = math.Round(statScale*1000000) / 1000000
		result.Baseline = math.Round(baseline)
	}
	return result
}

// solveWithRequiredCard finds the best team that includes requiredCard.
// Instead of enumerating all C(N,5) teams, it fixes requiredCard and
// enumerates only C(N-1,4) remaining slots — a ~5/N reduction.
func solveWithRequiredCard(cards []*Card, requiredCard *Card, statScale, baseline, songLength float64, fixedLeaderID string, overrideCostumeSkill *CostumeSkill) (bestScore EvalResult, bestTeamIDs [5]string, bestLeaderIdx int) {
	charGroups := map[string][]*Card{}
	for _, c := range cards {
		if c.Character == requiredCard.Character {
			continue
		}
		charGroups[c.Character] = append(charGroups[c.Character], c)
	}

	type charEntry struct {
		name     string
		maxTotal float64
	}
	charEntries := make([]charEntry, 0, len(charGroups))
	for name, group := range charGroups {
		maxT := 0.0
		for _, c := range group {
			if c.Total > maxT {
				maxT = c.Total
			}
		}
		charEntries = append(charEntries, charEntry{name, maxT})
	}
	sort.Slice(charEntries, func(i, j int) bool {
		if charEntries[i].maxTotal != charEntries[j].maxTotal {
			return charEntries[i].maxTotal > charEntries[j].maxTotal
		}
		return charEntries[i].name < charEntries[j].name
	})
	charNames := make([]string, len(charEntries))
	for i, e := range charEntries {
		charNames[i] = e.name
	}
	nOther := len(charNames)
	if nOther < 4 {
		return
	}

	if fixedLeaderID != "" {
		if fixedLeaderID == requiredCard.ID {
			// Required card is the leader — pick 4 others
			for a := 0; a < nOther-3; a++ {
				for b := a + 1; b < nOther-2; b++ {
					for ci := b + 1; ci < nOther-1; ci++ {
						for d := ci + 1; d < nOther; d++ {
							lists := [4][]*Card{
								charGroups[charNames[a]],
								charGroups[charNames[b]],
								charGroups[charNames[ci]],
								charGroups[charNames[d]],
							}
							for _, c0 := range lists[0] {
								for _, c1 := range lists[1] {
									for _, c2 := range lists[2] {
										for _, c3 := range lists[3] {
											team := [5]*Card{requiredCard, c0, c1, c2, c3}
											score := evaluateTeam(team, 0, statScale, baseline, songLength, overrideCostumeSkill)
											if score.UnitScore > bestScore.UnitScore {
												bestScore = score
												bestLeaderIdx = 0
												bestTeamIDs = [5]string{requiredCard.ID, c0.ID, c1.ID, c2.ID, c3.ID}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		} else {
			// Fixed leader is someone else — required card is a member, pick 3 others
			leaderChar := ""
			var leaderCard *Card
			for _, c := range cards {
				if c.ID == fixedLeaderID {
					leaderCard = c
					leaderChar = c.Character
					break
				}
			}
			if leaderCard == nil {
				return
			}
			// Remove leader's character from pool too
			var filteredNames []string
			for _, ch := range charNames {
				if ch != leaderChar {
					filteredNames = append(filteredNames, ch)
				}
			}
			nFiltered := len(filteredNames)
			if nFiltered < 3 {
				return
			}
			for a := 0; a < nFiltered-2; a++ {
				for b := a + 1; b < nFiltered-1; b++ {
					for ci := b + 1; ci < nFiltered; ci++ {
						lists := [3][]*Card{
							charGroups[filteredNames[a]],
							charGroups[filteredNames[b]],
							charGroups[filteredNames[ci]],
						}
						for _, c0 := range lists[0] {
							for _, c1 := range lists[1] {
								for _, c2 := range lists[2] {
									team := [5]*Card{leaderCard, requiredCard, c0, c1, c2}
									score := evaluateTeam(team, 0, statScale, baseline, songLength, overrideCostumeSkill)
									if score.UnitScore > bestScore.UnitScore {
										bestScore = score
										bestLeaderIdx = 0
										bestTeamIDs = [5]string{leaderCard.ID, requiredCard.ID, c0.ID, c1.ID, c2.ID}
									}
								}
							}
						}
					}
				}
			}
		}
	} else {
		// No fixed leader — required card is in the team, try all leader positions
		for a := 0; a < nOther-3; a++ {
			for b := a + 1; b < nOther-2; b++ {
				for ci := b + 1; ci < nOther-1; ci++ {
					for d := ci + 1; d < nOther; d++ {
						lists := [4][]*Card{
							charGroups[charNames[a]],
							charGroups[charNames[b]],
							charGroups[charNames[ci]],
							charGroups[charNames[d]],
						}
						for _, c0 := range lists[0] {
							for _, c1 := range lists[1] {
								for _, c2 := range lists[2] {
									for _, c3 := range lists[3] {
										team := [5]*Card{requiredCard, c0, c1, c2, c3}
										for li := 0; li < 5; li++ {
											score := evaluateTeam(team, li, statScale, baseline, songLength, overrideCostumeSkill)
											if score.UnitScore > bestScore.UnitScore {
												bestScore = score
												bestLeaderIdx = li
												bestTeamIDs = [5]string{requiredCard.ID, c0.ID, c1.ID, c2.ID, c3.ID}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Post-optimize: try all 24 permutations of non-leader members
	if bestScore.UnitScore > 0 {
		leaderID := bestTeamIDs[bestLeaderIdx]
		var leaderCard *Card
		var others [4]*Card
		var otherIDs [4]string
		oi := 0
		for _, c := range cards {
			if c.ID == leaderID {
				leaderCard = c
			}
		}
		for _, id := range bestTeamIDs {
			if id != leaderID {
				for _, c := range cards {
					if c.ID == id {
						others[oi] = c
						otherIDs[oi] = id
						oi++
						break
					}
				}
			}
		}
		if leaderCard != nil && oi == 4 {
			for _, perm := range perms4 {
				team := [5]*Card{leaderCard, others[perm[0]], others[perm[1]], others[perm[2]], others[perm[3]]}
				score := evaluateTeam(team, 0, statScale, baseline, songLength, overrideCostumeSkill)
				if score.UnitScore > bestScore.UnitScore {
					bestScore = score
					bestLeaderIdx = 0
					bestTeamIDs = [5]string{leaderID, otherIDs[perm[0]], otherIDs[perm[1]], otherIDs[perm[2]], otherIDs[perm[3]]}
				}
			}
		}
	}

	return
}

// solveWithRequiredCardSweep finds the best (team, costume) where requiredCard is a member.
// Path A of costume-sweep recommend: fix requiredCard as member, sweep all costumes.
func solveWithRequiredCardSweep(cards []*Card, requiredCard *Card, costumeSkills []CostumeEntry, statScale, baseline, songLength float64) (bestUnitScore float64, bestTeamIDs [5]string, bestLeaderIdx int, bestCostumeID string) {
	charGroups := map[string][]*Card{}
	for _, c := range cards {
		if c.Character == requiredCard.Character {
			continue
		}
		charGroups[c.Character] = append(charGroups[c.Character], c)
	}

	type charEntry struct {
		name     string
		maxTotal float64
	}
	charEntries := make([]charEntry, 0, len(charGroups))
	for name, group := range charGroups {
		maxT := 0.0
		for _, c := range group {
			if c.Total > maxT {
				maxT = c.Total
			}
		}
		charEntries = append(charEntries, charEntry{name, maxT})
	}
	sort.Slice(charEntries, func(i, j int) bool {
		if charEntries[i].maxTotal != charEntries[j].maxTotal {
			return charEntries[i].maxTotal > charEntries[j].maxTotal
		}
		return charEntries[i].name < charEntries[j].name
	})
	charNames := make([]string, len(charEntries))
	for i, e := range charEntries {
		charNames[i] = e.name
	}
	nOther := len(charNames)
	if nOther < 4 {
		return
	}

	for a := 0; a < nOther-3; a++ {
		for b := a + 1; b < nOther-2; b++ {
			for ci := b + 1; ci < nOther-1; ci++ {
				for d := ci + 1; d < nOther; d++ {
					lists := [4][]*Card{
						charGroups[charNames[a]],
						charGroups[charNames[b]],
						charGroups[charNames[ci]],
						charGroups[charNames[d]],
					}
					for _, c0 := range lists[0] {
						for _, c1 := range lists[1] {
							for _, c2 := range lists[2] {
								for _, c3 := range lists[3] {
									team := [5]*Card{requiredCard, c0, c1, c2, c3}
									var bestBase *BaseScores
									bestLI := 0
									for li := 0; li < 5; li++ {
										base := computeBaseScores(team, li, statScale, baseline, songLength)
										if bestBase == nil || base.BasePower > bestBase.BasePower {
											b := base
											bestBase = &b
											bestLI = li
										}
									}
									for _, ce := range costumeSkills {
										us, _, _, _, _, _ := applyCostume(bestBase, &ce.Skill)
										if us > bestUnitScore {
											bestUnitScore = us
											bestLeaderIdx = bestLI
											bestTeamIDs = [5]string{requiredCard.ID, c0.ID, c1.ID, c2.ID, c3.ID}
											bestCostumeID = ce.CardID
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return
}

// precomputedBase holds BaseScores for a team, precomputed once for multiple costume applications.
type precomputedBase struct {
	base      BaseScores
	leaderIdx int
	teamIDs   [5]string
}

// precomputeOwnedBases computes BaseScores for all team combinations from owned cards.
// This is done once and reused across multiple candidate costume evaluations.
func precomputeOwnedBases(cards []*Card, statScale, baseline, songLength float64) []precomputedBase {
	charGroups := map[string][]*Card{}
	for _, c := range cards {
		charGroups[c.Character] = append(charGroups[c.Character], c)
	}

	type charEntry struct {
		name     string
		maxTotal float64
	}
	charEntries := make([]charEntry, 0, len(charGroups))
	for name, group := range charGroups {
		maxT := 0.0
		for _, c := range group {
			if c.Total > maxT {
				maxT = c.Total
			}
		}
		charEntries = append(charEntries, charEntry{name, maxT})
	}
	sort.Slice(charEntries, func(i, j int) bool {
		if charEntries[i].maxTotal != charEntries[j].maxTotal {
			return charEntries[i].maxTotal > charEntries[j].maxTotal
		}
		return charEntries[i].name < charEntries[j].name
	})
	charNames := make([]string, len(charEntries))
	for i, e := range charEntries {
		charNames[i] = e.name
	}
	nChars := len(charNames)
	if nChars < 5 {
		return nil
	}

	var bases []precomputedBase
	for a := 0; a < nChars-4; a++ {
		for b := a + 1; b < nChars-3; b++ {
			for ci := b + 1; ci < nChars-2; ci++ {
				for d := ci + 1; d < nChars-1; d++ {
					for e := d + 1; e < nChars; e++ {
						lists := [5][]*Card{
							charGroups[charNames[a]],
							charGroups[charNames[b]],
							charGroups[charNames[ci]],
							charGroups[charNames[d]],
							charGroups[charNames[e]],
						}
						for _, c0 := range lists[0] {
							for _, c1 := range lists[1] {
								for _, c2 := range lists[2] {
									for _, c3 := range lists[3] {
										for _, c4 := range lists[4] {
											team := [5]*Card{c0, c1, c2, c3, c4}
											var bestBase *BaseScores
											bestLI := 0
											for li := 0; li < 5; li++ {
												base := computeBaseScores(team, li, statScale, baseline, songLength)
												if bestBase == nil || base.BasePower > bestBase.BasePower {
													b := base
													bestBase = &b
													bestLI = li
												}
											}
											bases = append(bases, precomputedBase{
												base:      *bestBase,
												leaderIdx: bestLI,
												teamIDs:   [5]string{c0.ID, c1.ID, c2.ID, c3.ID, c4.ID},
											})
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return bases
}

// solveForcedCostumeFromBases finds the best team for a given forced costume
// using precomputed base scores. O(bases) per call instead of full enumeration.
func solveForcedCostumeFromBases(bases []precomputedBase, forcedCostume *CostumeSkill) (bestUnitScore float64, bestTeamIDs [5]string, bestLeaderIdx int) {
	for _, pb := range bases {
		us, _, _, _, _, _ := applyCostume(&pb.base, forcedCostume)
		if us > bestUnitScore {
			bestUnitScore = us
			bestTeamIDs = pb.teamIDs
			bestLeaderIdx = pb.leaderIdx
		}
	}
	return
}

func recommend(ownedSpecs map[string]CardSpec, allRawCards []CardRaw, topN, acquireCount int, statScale, baseline, songLength float64, fixedLeaderID, costumeOnlyLeaderID string, sweepCostumes bool, cf *CardsFile) RecommendOutput {
	outerProgress := progressCallback
	progressCallback = nil
	defer func() { progressCallback = outerProgress }()

	acquireCount = max(1, min(acquireCount, 5))

	rawCardMap := map[string]*CardRaw{}
	for i := range allRawCards {
		rawCardMap[allRawCards[i].ID] = &allRawCards[i]
	}

	// Resolve owned cards
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

	// Resolve the costume override once
	var overrideCostumeSkill *CostumeSkill
	effectiveCostumeOnly := costumeOnlyLeaderID
	if fixedLeaderID != "" && costumeOnlyLeaderID != "" {
		effectiveCostumeOnly = ""
	}
	if effectiveCostumeOnly != "" {
		raw := rawCardMap[effectiveCostumeOnly]
		if raw != nil && len(raw.PotentialData) > 0 {
			cs := raw.PotentialData[0].CostumeSkill
			overrideCostumeSkill = &cs
		}
	}

	// Compute baseline
	baseCards := resolveOwned(ownedSpecs)
	baseScore := 0
	if sweepCostumes && fixedLeaderID == "" && effectiveCostumeOnly == "" {
		rawCardMapPtr := map[string]*CardRaw{}
		for i := range allRawCards {
			rawCardMapPtr[allRawCards[i].ID] = &allRawCards[i]
		}
		baseResult := solveSweepCostumes(baseCards, allRawCards, rawCardMapPtr, 1, statScale, baseline, songLength, nil, cf)
		if len(baseResult.Results) > 0 {
			baseScore = baseResult.Results[0].UnitScore
		}
	} else {
		baseResult := solve(baseCards, 1, statScale, baseline, songLength, fixedLeaderID, effectiveCostumeOnly, overrideCostumeSkill, nil)
		if len(baseResult.Results) > 0 {
			baseScore = baseResult.Results[0].UnitScore
		}
	}

	// Build costume skills list and precompute bases for sweep mode
	// Costume pool is limited to owned cards only (consistent with solveSweepCostumes).
	// Per-candidate costumes are handled separately in the evaluation loop.
	var sweepCostumeSkills []CostumeEntry
	var ownedBases []precomputedBase
	if sweepCostumes {
		ownedIDs := map[string]bool{}
		for id := range ownedSpecs {
			ownedIDs[id] = true
		}
		var rawCostumes []CostumeEntry
		for i := range allRawCards {
			raw := &allRawCards[i]
			if !ownedIDs[raw.ID] {
				continue
			}
			if len(raw.PotentialData) > 0 {
				rawCostumes = append(rawCostumes, CostumeEntry{raw.ID, raw.PotentialData[0].CostumeSkill})
			}
		}
		sweepCostumeSkills = pruneCostumes(rawCostumes)
		ownedBases = precomputeOwnedBases(baseCards, statScale, baseline, songLength)
	}

	// Build candidates
	type candidate struct {
		cardID           string
		cardName         string
		character        string
		action           string
		currentPotential *int
		targetPotential  int
		cost             int
	}

	var candidates []candidate
	for i := range allRawCards {
		raw := &allRawCards[i]
		cid := raw.ID
		if _, owned := ownedSpecs[cid]; !owned {
			candidates = append(candidates, candidate{
				cardID:          cid,
				cardName:        raw.CardName,
				character:       raw.Character,
				action:          "acquire",
				targetPotential: 0,
				cost:            1,
			})
		} else {
			curPot := ownedSpecs[cid].Potential
			maxPot := len(raw.PotentialData) - 1
			for target := curPot + 1; target <= maxPot; target++ {
				cp := curPot
				candidates = append(candidates, candidate{
					cardID:           cid,
					cardName:         raw.CardName,
					character:        raw.Character,
					action:           "uncap",
					currentPotential: &cp,
					targetPotential:  target,
					cost:             target - curPot,
				})
			}
		}
	}

	// Phase 1: evaluate cost=1 candidates using required-card optimization
	type singleResult struct {
		cand  candidate
		delta int
		best  JSONResult
	}
	var singleResults []singleResult
	effectiveCardIDs := map[string]bool{}

	cost1Count := 0
	for _, c := range candidates {
		if c.cost == 1 {
			cost1Count++
		}
	}
	// Path B baseline: for sweep mode, pre-compute forcedCostume results for each candidate
	// that has a costume skill. This checks if using the candidate's costume with existing
	// members beats the baseline.
	evaluated := 0
	for _, cand := range candidates {
		if cand.cost != 1 {
			continue
		}
		evaluated++
		if outerProgress != nil {
			outerProgress(evaluated, cost1Count)
		}

		// Build the card pool with this candidate applied
		trialSpecs := map[string]CardSpec{}
		for k, v := range ownedSpecs {
			trialSpecs[k] = v
		}
		if cand.action == "acquire" {
			trialSpecs[cand.cardID] = CardSpec{ID: cand.cardID, Potential: 0}
		} else {
			old := trialSpecs[cand.cardID]
			old.Potential = cand.targetPotential
			trialSpecs[cand.cardID] = old
		}
		trialCards := resolveOwned(trialSpecs)

		candRaw := rawCardMap[cand.cardID]
		if candRaw == nil {
			continue
		}
		candPot := 0
		if cand.action == "uncap" {
			candPot = cand.targetPotential
		}
		resolvedCand := resolveCard(candRaw, candPot, nil, cf)

		var bestUnitScore float64
		var bestTeamIDs [5]string
		var bestLeaderIdx int
		var bestCostumeID string

		if sweepCostumes && fixedLeaderID == "" && effectiveCostumeOnly == "" {
			// Build costume list: owned + candidate's costume (if new acquire)
			candCostumes := sweepCostumeSkills
			if cand.action == "acquire" && len(candRaw.PotentialData) > 0 {
				candCostumes = make([]CostumeEntry, len(sweepCostumeSkills), len(sweepCostumeSkills)+1)
				copy(candCostumes, sweepCostumeSkills)
				candCostumes = append(candCostumes, CostumeEntry{cand.cardID, candRaw.PotentialData[0].CostumeSkill})
			}

			// Path A: candidate as member, sweep all costumes
			usA, teamA, liA, costumeA := solveWithRequiredCardSweep(trialCards, &resolvedCand, candCostumes, statScale, baseline, songLength)
			bestUnitScore = usA
			bestTeamIDs = teamA
			bestLeaderIdx = liA
			bestCostumeID = costumeA

			// Path B: candidate's costume with existing members (uses precomputed bases)
			if len(candRaw.PotentialData) > 0 {
				candCostume := candRaw.PotentialData[0].CostumeSkill
				usB, teamB, liB := solveForcedCostumeFromBases(ownedBases, &candCostume)
				if usB > bestUnitScore {
					bestUnitScore = usB
					bestTeamIDs = teamB
					bestLeaderIdx = liB
					bestCostumeID = cand.cardID
				}
			}
		} else {
			// Non-sweep: use required-card optimization
			score, teamIDs, leaderIdx := solveWithRequiredCard(trialCards, &resolvedCand, statScale, baseline, songLength, fixedLeaderID, overrideCostumeSkill)
			bestUnitScore = score.UnitScore
			bestTeamIDs = teamIDs
			bestLeaderIdx = leaderIdx
		}

		unitScore := int(math.Round(bestUnitScore))
		delta := unitScore - baseScore
		if delta > 0 {
			best := JSONResult{
				UnitScore: unitScore,
				LeaderID:  bestTeamIDs[bestLeaderIdx],
				MemberIDs: bestTeamIDs[:],
			}
			if bestCostumeID != "" {
				best.CostumeOnlyLeaderID = &bestCostumeID
			}
			singleResults = append(singleResults, singleResult{cand, delta, best})
			effectiveCardIDs[cand.cardID] = true
		}
	}

	sort.Slice(singleResults, func(i, j int) bool {
		return singleResults[i].delta > singleResults[j].delta
	})

	var output RecommendOutput
	output.BaseScore = baseScore
	output.AcquireCount = acquireCount

	if acquireCount == 1 {
		limit := topN
		if limit > len(singleResults) {
			limit = len(singleResults)
		}
		for i := 0; i < limit; i++ {
			sr := singleResults[i]
			output.Recommendations = append(output.Recommendations, RecommendResult{
				Rank: i + 1,
				Cards: []RecommendCard{{
					CardID:           sr.cand.cardID,
					CardName:         sr.cand.cardName,
					Character:        sr.cand.character,
					Action:           sr.cand.action,
					CurrentPotential: sr.cand.currentPotential,
					TargetPotential:  sr.cand.targetPotential,
					Cost:             sr.cand.cost,
				}},
				NewScore: sr.best.UnitScore,
				Delta:    sr.delta,
				BestTeam: RecommendBestTeam{
					LeaderID:            sr.best.LeaderID,
					MemberIDs:           sr.best.MemberIDs,
					CostumeOnlyLeaderID: sr.best.CostumeOnlyLeaderID,
				},
			})
		}
	} else {
		// Multi-acquire: combine cost=1 singles + multi-uncap candidates
		var multiUncap []candidate
		for _, cand := range candidates {
			if cand.cost > 1 && cand.cost <= acquireCount && effectiveCardIDs[cand.cardID] {
				multiUncap = append(multiUncap, cand)
			}
		}
		maxSingle := 20 - len(multiUncap)
		if maxSingle < 0 {
			maxSingle = 0
		}
		var shortlist []candidate
		for i := 0; i < len(singleResults) && i < maxSingle; i++ {
			shortlist = append(shortlist, singleResults[i].cand)
		}
		shortlist = append(shortlist, multiUncap...)

		applyCandidate := func(specs map[string]CardSpec, cand candidate) map[string]CardSpec {
			newSpecs := map[string]CardSpec{}
			for k, v := range specs {
				newSpecs[k] = v
			}
			if cand.action == "acquire" {
				newSpecs[cand.cardID] = CardSpec{ID: cand.cardID, Potential: 0}
			} else {
				old := newSpecs[cand.cardID]
				old.Potential = cand.targetPotential
				newSpecs[cand.cardID] = old
			}
			return newSpecs
		}

		// Generate combinations by cost — collect first, then evaluate with progress
		var allCombos [][]candidate
		var generateCombos func(items []candidate, totalCost, start int, current []candidate)
		generateCombos = func(items []candidate, totalCost, start int, current []candidate) {
			if totalCost == 0 {
				cardIDs := map[string]bool{}
				acquireChars := map[string]bool{}
				for _, c := range current {
					if cardIDs[c.cardID] {
						return
					}
					cardIDs[c.cardID] = true
					if c.action == "acquire" {
						if acquireChars[c.character] {
							return
						}
						acquireChars[c.character] = true
					}
				}
				combo := make([]candidate, len(current))
				copy(combo, current)
				allCombos = append(allCombos, combo)
				return
			}
			for i := start; i < len(items); i++ {
				if items[i].cost <= totalCost {
					generateCombos(items, totalCost-items[i].cost, i+1, append(current, items[i]))
				}
			}
		}
		generateCombos(shortlist, acquireCount, 0, nil)

		var comboResults []RecommendResult
		for ci, combo := range allCombos {
			if outerProgress != nil {
				outerProgress(cost1Count+ci+1, cost1Count+len(allCombos))
			}
			trialSpecs := map[string]CardSpec{}
			for k, v := range ownedSpecs {
				trialSpecs[k] = v
			}
			for _, cand := range combo {
				trialSpecs = applyCandidate(trialSpecs, cand)
			}
			trialCards := resolveOwned(trialSpecs)

			// Optimization: at least one combo card must be in the best team.
			// Try each combo card as required and take the best.
			var bestUnitScore float64
			var bestTeamIDs [5]string
			var bestLeaderIdx int
			var bestCostumeID string

			for _, cand := range combo {
				candRaw := rawCardMap[cand.cardID]
				if candRaw == nil {
					continue
				}
				candPot := 0
				if cand.action == "uncap" {
					candPot = cand.targetPotential
				}
				resolvedCand := resolveCard(candRaw, candPot, nil, cf)

				if sweepCostumes && fixedLeaderID == "" && effectiveCostumeOnly == "" {
					// Build costume list: owned + all combo cards' costumes
					comboCostumes := make([]CostumeEntry, len(sweepCostumeSkills))
					copy(comboCostumes, sweepCostumeSkills)
					for _, cc := range combo {
						ccRaw := rawCardMap[cc.cardID]
						if ccRaw != nil && len(ccRaw.PotentialData) > 0 {
							alreadyOwned := false
							for _, ce := range sweepCostumeSkills {
								if ce.CardID == cc.cardID {
									alreadyOwned = true
									break
								}
							}
							if !alreadyOwned {
								comboCostumes = append(comboCostumes, CostumeEntry{cc.cardID, ccRaw.PotentialData[0].CostumeSkill})
							}
						}
					}

					// Path A: this card as member, sweep costumes
					usA, teamA, liA, costumeA := solveWithRequiredCardSweep(trialCards, &resolvedCand, comboCostumes, statScale, baseline, songLength)
					if usA > bestUnitScore {
						bestUnitScore = usA
						bestTeamIDs = teamA
						bestLeaderIdx = liA
						bestCostumeID = costumeA
					}

					// Path B: this card's costume with existing+combo members
					if len(candRaw.PotentialData) > 0 {
						candCostume := candRaw.PotentialData[0].CostumeSkill
						// Use precomputed owned bases + also check trial cards
						usB, teamB, liB := solveForcedCostumeFromBases(ownedBases, &candCostume)
						if usB > bestUnitScore {
							bestUnitScore = usB
							bestTeamIDs = teamB
							bestLeaderIdx = liB
							bestCostumeID = cand.cardID
						}
					}
				} else {
					score, teamIDs, leaderIdx := solveWithRequiredCard(trialCards, &resolvedCand, statScale, baseline, songLength, fixedLeaderID, overrideCostumeSkill)
					if score.UnitScore > bestUnitScore {
						bestUnitScore = score.UnitScore
						bestTeamIDs = teamIDs
						bestLeaderIdx = leaderIdx
					}
				}
			}

			unitScore := int(math.Round(bestUnitScore))
			delta := unitScore - baseScore
			if delta > 0 {
				cards := make([]RecommendCard, len(combo))
				for i, c := range combo {
					cards[i] = RecommendCard{
						CardID:           c.cardID,
						CardName:         c.cardName,
						Character:        c.character,
						Action:           c.action,
						CurrentPotential: c.currentPotential,
						TargetPotential:  c.targetPotential,
						Cost:             c.cost,
					}
				}
				bt := RecommendBestTeam{
					LeaderID:  bestTeamIDs[bestLeaderIdx],
					MemberIDs: bestTeamIDs[:],
				}
				if bestCostumeID != "" {
					bt.CostumeOnlyLeaderID = &bestCostumeID
				}
				comboResults = append(comboResults, RecommendResult{
					Cards:    cards,
					NewScore: unitScore,
					Delta:    delta,
					BestTeam: bt,
				})
			}
		}

		sort.Slice(comboResults, func(i, j int) bool {
			return comboResults[i].Delta > comboResults[j].Delta
		})
		if len(comboResults) > topN {
			comboResults = comboResults[:topN]
		}
		for i := range comboResults {
			comboResults[i].Rank = i + 1
		}
		output.Recommendations = comboResults
	}

	if output.Recommendations == nil {
		output.Recommendations = []RecommendResult{}
	}
	return output
}

func formatOutput(results []SolveResult, totalCombos int, statScale, baseline float64, costumeOnlyLeaderID string, stabilityLengths []float64, cardMap map[string]*Card, songLength float64, overrideCostumeSkill *CostumeSkill) JSONOutput {
	jsonResults := make([]JSONResult, len(results))
	for i, r := range results {
		var costumePtr *string
		if costumeOnlyLeaderID != "" {
			s := costumeOnlyLeaderID
			costumePtr = &s
		}
		entry := JSONResult{
			Rank:                i + 1,
			UnitScore:           int(math.Round(r.Score.UnitScore)),
			TotalPower:          int(math.Round(r.Score.TotalPower)),
			ScoreBonus:          fixedFloat(roundTo1(r.Score.ScoreBonus)),
			ActivePct:           fixedFloat(roundTo1(r.Score.ActivePct)),
			CostumeSBPct:        fixedFloat(roundTo1(r.Score.CostumeSBPct)),
			PassiveSBPct:        fixedFloat(roundTo1(r.Score.PassiveSBPct)),
			SpecialPct:          fixedFloat(roundTo1(r.Score.SpecialPct)),
			LeaderID:            r.TeamIDs[r.LeaderIdx],
			CostumeOnlyLeaderID: costumePtr,
			MemberIDs:           r.TeamIDs[:],
		}
		if len(stabilityLengths) > 0 {
			team := [5]*Card{}
			for ti, id := range r.TeamIDs {
				team[ti] = cardMap[id]
			}
			entry.Stability = map[string]int{}
			for _, sl := range stabilityLengths {
				s := evaluateTeam(team, r.LeaderIdx, statScale, baseline, sl, overrideCostumeSkill)
				entry.Stability[fmt.Sprintf("%g", sl)] = int(math.Round(s.UnitScore))
			}
		}
		jsonResults[i] = entry
	}

	return JSONOutput{
		TotalCombinations: totalCombos,
		StatScale:         statScale,
		Baseline:          baseline,
		Results:           jsonResults,
	}
}
