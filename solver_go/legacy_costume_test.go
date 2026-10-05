package main

import "sort"

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
