package main

import (
	"math"
	"sort"
)

const (
	unitScoreK    = 2.0373
	activeBase    = 52.89
	activeDivisor = 12.82
	costumeSSRate = 0.68
	supportSSRate = 0.20
)

func checkCondition(cond *ConditionObj, typeCounts map[string]int, groupCounts map[string]int) bool {
	if cond == nil {
		return true
	}
	switch cond.Type {
	case "type_count":
		return typeCounts[cond.TypeName] >= cond.MinCount
	case "group_count":
		return groupCounts[cond.Group] >= cond.MinCount
	}
	return false
}

func checkCenterTypeCondition(cond *string, typeCounts map[string]int) bool {
	if cond == nil {
		return false
	}
	c := *cond
	if c == "life_600" || c == "combo_40" {
		return false
	}
	if len(c) > 2 && c[len(c)-2:] == "_2" {
		typeName := c[:len(c)-2]
		return typeCounts[typeName] >= 2
	}
	return false
}

func countTypes(team [5]*Card) map[string]int {
	m := map[string]int{}
	for _, c := range team {
		m[c.Type]++
	}
	return m
}

func countGroups(team [5]*Card) map[string]int {
	m := map[string]int{}
	for _, c := range team {
		m[c.Group]++
	}
	return m
}

func evaluateTeam(team [5]*Card, leaderIdx int, statScale, baseline, songLength float64, overrideCostumeSkill *CostumeSkill) EvalResult {
	typeCounts := countTypes(team)
	groupCounts := countGroups(team)
	leader := team[leaderIdx]

	// Costume skill
	var costumePerfRate, costumeTechRate, costumeSenseRate, costumeSSVal float64
	cs := &leader.CostumeSkill
	if overrideCostumeSkill != nil {
		cs = overrideCostumeSkill
	}
	if checkCondition(cs.Condition, typeCounts, groupCounts) {
		for _, eff := range cs.Effects {
			val := eff.Value / 100.0
			switch eff.Stat {
			case "score_support":
				costumeSSVal += val
			case "all":
				costumePerfRate += val
				costumeTechRate += val
				costumeSenseRate += val
			case "performance":
				costumePerfRate += val
			case "technique":
				costumeTechRate += val
			case "sense":
				costumeSenseRate += val
			}
		}
	}

	// Support skills
	var supportBonus [5]float64
	var supportSS float64
	for idx, card := range team {
		ss := &card.SupportSkill
		if !checkCondition(ss.Condition, typeCounts, groupCounts) {
			continue
		}
		switch ss.EffectType {
		case "self_all_param", "self_all_param_conditional":
			val := ss.Value / 100.0
			supportBonus[idx] += card.Stats.Total() * val

		case "self_stat":
			val := ss.Value / 100.0
			supportBonus[idx] += statByName(&card.Stats, ss.Stat) * val

		case "type_stat", "type_stat_conditional":
			val := ss.Value / 100.0
			applied := 0
			count := 0
			if ss.Target != nil {
				count = ss.Target.Count
			}
			for i, c := range team {
				if ss.Target != nil && c.Type == ss.Target.TypeMatch && applied < count {
					supportBonus[i] += statByName(&c.Stats, ss.Stat) * val
					applied++
				}
			}

		case "type_all_param":
			val := ss.Value / 100.0
			applied := 0
			count := 0
			if ss.Target != nil {
				count = ss.Target.Count
			}
			for i, c := range team {
				if ss.Target != nil && c.Type == ss.Target.TypeMatch && applied < count {
					supportBonus[i] += c.Stats.Total() * val
					applied++
				}
			}

		case "type_score_support":
			if ss.Target != nil {
				required := ss.Target.Count
				if required == 0 {
					required = 2
				}
				if typeCounts[ss.Target.TypeMatch] >= required {
					supportSS += ss.Value / 100.0
				}
			}

		case "group_stat", "group_stat_conditional":
			val := ss.Value / 100.0
			applied := 0
			count := 0
			if ss.Target != nil {
				count = ss.Target.Count
			}
			for i, c := range team {
				if ss.Target != nil && c.Group == ss.Target.Group && applied < count {
					supportBonus[i] += statByName(&c.Stats, ss.Stat) * val
					applied++
				}
			}

		case "group_all_param", "group_all_param_conditional":
			val := ss.Value / 100.0
			applied := 0
			count := 0
			if ss.Target != nil {
				count = ss.Target.Count
			}
			for i, c := range team {
				if ss.Target != nil && c.Group == ss.Target.Group && applied < count {
					supportBonus[i] += c.Stats.Total() * val
					applied++
				}
			}

		case "group_score_support", "group_score_support_conditional":
			if ss.Target != nil {
				required := ss.Target.Count
				if required == 0 {
					required = 2
				}
				if groupCounts[ss.Target.Group] >= required {
					supportSS += ss.Value / 100.0
				}
			}
		}
	}

	// Special → Active rate up time average
	rateUpTimeAvg := 0.0
	for _, c := range team {
		if c.SpecialSkill != nil && c.SpecialSkill.SkillRateUp > 0 {
			rateUpTimeAvg += c.SpecialSkill.SkillRateUp * 10 * c.SpecialSkill.Duration / songLength
		}
	}

	// Active skill % (Expected Maximum)
	type activeEntry struct {
		scoreUp float64
		uptime  float64
	}
	activeMembers := make([]activeEntry, 0, 5)
	for _, card := range team {
		cs := &card.CenterSkill
		scoreUp := cs.ScoreUp
		if cs.Condition != nil && checkCenterTypeCondition(cs.Condition, typeCounts) {
			if cs.ConditionalScoreUp != nil {
				scoreUp = *cs.ConditionalScoreUp
			}
		}
		baseProb := 1.0
		if cs.ActivationProbabilityPermil != nil {
			baseProb = float64(*cs.ActivationProbabilityPermil) / 1000.0
		}
		boostedProb := math.Min(1.0, baseProb+rateUpTimeAvg/1000.0)
		uptime := math.Min(1.0, cs.Duration/cs.Interval*boostedProb)
		activeMembers = append(activeMembers, activeEntry{scoreUp, uptime})
	}

	sort.Slice(activeMembers, func(i, j int) bool {
		return activeMembers[i].scoreUp > activeMembers[j].scoreUp
	})
	activeSum := 0.0
	probNoneHigher := 1.0
	for _, am := range activeMembers {
		activeSum += am.scoreUp * am.uptime * probNoneHigher
		probNoneHigher *= (1.0 - am.uptime)
	}
	activePct := activeBase + activeSum/activeDivisor

	costumeSBPct := costumeSSVal * 100 * costumeSSRate
	passiveSBPct := supportSS * 100 * supportSSRate

	// Special skill %
	specialPct := 0.0
	for _, c := range team {
		if c.SpecialSkill != nil {
			specialPct += c.SpecialSkill.ScoreSupport * c.SpecialSkill.Duration / songLength
		}
	}

	scoreBonus := activePct + costumeSBPct + passiveSBPct + specialPct

	// Total power
	totalPerf := 0.0
	totalTech := 0.0
	totalSense := 0.0
	for _, c := range team {
		totalPerf += c.Stats.Performance
		totalTech += c.Stats.Technique
		totalSense += c.Stats.Sense
	}
	totalPerf *= statScale
	totalTech *= statScale
	totalSense *= statScale

	memberParams := totalPerf + totalTech + totalSense
	costumeContrib := totalPerf*costumePerfRate + totalTech*costumeTechRate + totalSense*costumeSenseRate

	var supportBonusTotal float64
	for _, b := range supportBonus {
		supportBonusTotal += b
	}
	supportContrib := supportBonusTotal * statScale

	totalPower := memberParams + costumeContrib + supportContrib + baseline
	unitScore := totalPower * (1 + scoreBonus/100) * unitScoreK

	return EvalResult{
		UnitScore:      unitScore,
		TotalPower:     totalPower,
		MemberParams:   memberParams,
		CostumeContrib: costumeContrib,
		SupportContrib: supportContrib,
		ActivePct:      activePct,
		CostumeSBPct:   costumeSBPct,
		PassiveSBPct:   passiveSBPct,
		SpecialPct:     specialPct,
		ScoreBonus:     scoreBonus,
		CostumeSSVal:   costumeSSVal,
		SupportSSVal:   supportSS,
	}
}

func computeBaseScores(team [5]*Card, leaderIdx int, statScale, baseline, songLength float64) BaseScores {
	emptyCostume := &CostumeSkill{Effects: nil}
	scores := evaluateTeam(team, leaderIdx, statScale, baseline, songLength, emptyCostume)

	totalPerf := 0.0
	totalTech := 0.0
	totalSense := 0.0
	for _, c := range team {
		totalPerf += c.Stats.Performance
		totalTech += c.Stats.Technique
		totalSense += c.Stats.Sense
	}
	totalPerf *= statScale
	totalTech *= statScale
	totalSense *= statScale
	memberParams := totalPerf + totalTech + totalSense
	supportContrib := scores.SupportContrib
	basePower := memberParams + supportContrib + baseline
	baseBonus := scores.ActivePct + scores.PassiveSBPct + scores.SpecialPct

	return BaseScores{
		BasePower:      basePower,
		BaseBonus:      baseBonus,
		TotalPerf:      totalPerf,
		TotalTech:      totalTech,
		TotalSense:     totalSense,
		MemberParams:   memberParams,
		SupportContrib: supportContrib,
		ActivePct:      scores.ActivePct,
		PassiveSBPct:   scores.PassiveSBPct,
		SpecialPct:     scores.SpecialPct,
		SupportSS:      scores.SupportSSVal,
		TypeCounts:     countTypes(team),
		GroupCounts:    countGroups(team),
		Leader:         team[leaderIdx],
	}
}

func applyCostume(base *BaseScores, cs *CostumeSkill) (unitScore, totalPower, scoreBonus, costumeSBPct, costumeSSVal, costumeContrib float64) {
	var cpr, ctr, csr float64
	if checkCondition(cs.Condition, base.TypeCounts, base.GroupCounts) {
		for _, eff := range cs.Effects {
			val := eff.Value / 100.0
			switch eff.Stat {
			case "score_support":
				costumeSSVal += val
			case "all":
				cpr += val
				ctr += val
				csr += val
			case "performance":
				cpr += val
			case "technique":
				ctr += val
			case "sense":
				csr += val
			}
		}
	}
	costumeContrib = base.TotalPerf*cpr + base.TotalTech*ctr + base.TotalSense*csr
	costumeSBPct = costumeSSVal * 100 * costumeSSRate
	totalPower = base.BasePower + costumeContrib
	scoreBonus = base.BaseBonus + costumeSBPct
	unitScore = totalPower * (1 + scoreBonus/100) * unitScoreK
	return
}

func statByName(s *Stats, name string) float64 {
	switch name {
	case "performance":
		return s.Performance
	case "technique":
		return s.Technique
	case "sense":
		return s.Sense
	}
	return 0
}

func roundTo1(v float64) float64 {
	return math.Round(v*10) / 10
}

var perms4 [24][4]int

func init() {
	idx := 0
	for a := 0; a < 4; a++ {
		for b := 0; b < 4; b++ {
			if b == a {
				continue
			}
			for c := 0; c < 4; c++ {
				if c == a || c == b {
					continue
				}
				d := 6 - a - b - c
				perms4[idx] = [4]int{a, b, c, d}
				idx++
			}
		}
	}
}

func optimizeResults(results []SolveResult, cardMap map[string]*Card, statScale, baseline, songLength float64, overrideCostumeSkill *CostumeSkill) []SolveResult {
	optimized := make([]SolveResult, len(results))
	for ri, r := range results {
		leaderID := r.TeamIDs[r.LeaderIdx]
		leaderCard := cardMap[leaderID]

		var others [4]*Card
		var otherIDs [4]string
		oi := 0
		for _, id := range r.TeamIDs {
			if id != leaderID {
				others[oi] = cardMap[id]
				otherIDs[oi] = id
				oi++
			}
		}

		best := r
		for _, perm := range perms4 {
			team := [5]*Card{leaderCard, others[perm[0]], others[perm[1]], others[perm[2]], others[perm[3]]}
			score := evaluateTeam(team, 0, statScale, baseline, songLength, overrideCostumeSkill)
			if score.UnitScore > best.Score.UnitScore {
				best = SolveResult{
					Score:               score,
					LeaderIdx:           0,
					TeamIDs:             [5]string{leaderID, otherIDs[perm[0]], otherIDs[perm[1]], otherIDs[perm[2]], otherIDs[perm[3]]},
					CostumeOnlyLeaderID: r.CostumeOnlyLeaderID,
				}
			}
		}

		if best.Score.UnitScore == r.Score.UnitScore {
			best.TeamIDs = r.TeamIDs
			best.LeaderIdx = r.LeaderIdx
		}
		optimized[ri] = best
	}

	sort.Slice(optimized, func(i, j int) bool {
		return optimized[i].Score.UnitScore > optimized[j].Score.UnitScore
	})
	return optimized
}
