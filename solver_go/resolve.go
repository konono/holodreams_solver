package main

import (
	"encoding/json"
	"fmt"
	"os"
)

var (
	cachedCardsFile *CardsFile
)

func loadCardsFile(path string) (*CardsFile, error) {
	if cachedCardsFile != nil {
		return cachedCardsFile, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cf CardsFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, err
	}

	cachedCardsFile = &cf
	return &cf, nil
}

func loadCards(path string) ([]CardRaw, error) {
	cf, err := loadCardsFile(path)
	if err != nil {
		return nil, err
	}
	return cf.Cards, nil
}

func parseSupportSkill(raw SupportSkillRaw) SupportSkill {
	ss := SupportSkill{
		EffectType: raw.EffectType,
		Value:      raw.Value,
		Stat:       raw.Stat,
	}

	if len(raw.Condition) > 0 && raw.Condition[0] == '{' {
		cond := &ConditionObj{}
		json.Unmarshal(raw.Condition, cond)
		ss.Condition = cond
	}

	if len(raw.Target) > 0 && raw.Target[0] == '{' {
		t := &SupportTarget{}
		json.Unmarshal(raw.Target, t)
		ss.Target = t
	}

	return ss
}

func resolveCard(raw *CardRaw, potential int, level *int, cf *CardsFile) Card {
	pd := raw.PotentialData
	if len(pd) == 0 {
		return cardFromFlat(raw)
	}

	if potential < 0 {
		potential = 0
	}
	if potential >= len(pd) {
		potential = len(pd) - 1
	}
	snap := pd[potential]

	actualLevel := 80
	if level != nil {
		actualLevel = *level
	}
	if actualLevel < 1 {
		actualLevel = 1
	}
	if actualLevel > 80 {
		actualLevel = 80
	}

	var stats Stats
	if actualLevel == 80 {
		stats = snap.RefStatsLv80
	} else {
		resolved := false
		if cf != nil && raw.CardLevelGroupID != "" {
			if table, ok := cf.LevelTables[raw.CardLevelGroupID]; ok {
				baseValue, ok := table[fmt.Sprintf("%d", actualLevel)]
				if ok && baseValue > 0 && raw.Permil != nil {
					bv := int64(baseValue)
					bonus := int64(snap.ParamBonusPermil)
					multiplier := 1000 + bonus
					perfPermil := int64(raw.Permil.Performance)
					techPermil := int64(raw.Permil.Technique)
					sensePermil := int64(raw.Permil.Sense)
					stats.Performance = float64((bv*perfPermil*multiplier + 999999) / 1000000)
					stats.Technique = float64((bv*techPermil*multiplier + 999999) / 1000000)
					stats.Sense = float64((bv*sensePermil*multiplier + 999999) / 1000000)
					resolved = true
				}
			}
		}
		if !resolved {
			stats = snap.RefStatsLv80
		}
	}

	card := Card{
		ID:           raw.ID,
		HolodoriID:   raw.HolodoriID,
		Character:    raw.Character,
		CardName:     raw.CardName,
		Rarity:       raw.Rarity,
		Type:         raw.Type,
		Group:        raw.Group,
		Variant:      raw.Variant,
		Potential:    potential,
		Level:        actualLevel,
		Stats:        stats,
		Total:        stats.Total(),
		CenterSkill:  snap.CenterSkill,
		SupportSkill: parseSupportSkill(snap.SupportSkillRaw),
		CostumeSkill: snap.CostumeSkill,
		SpecialSkill: snap.SpecialSkill,
	}

	return card
}

func cardFromFlat(raw *CardRaw) Card {
	return Card{
		ID:           raw.ID,
		HolodoriID:   raw.HolodoriID,
		Character:    raw.Character,
		CardName:     raw.CardName,
		Rarity:       raw.Rarity,
		Type:         raw.Type,
		Group:        raw.Group,
		Variant:      raw.Variant,
		Stats:        raw.Stats,
		Total:        raw.Stats.Total(),
		CenterSkill:  raw.CenterSkill,
		SupportSkill: parseSupportSkill(raw.SupportSkill),
		CostumeSkill: raw.CostumeSkill,
		SpecialSkill: raw.SpecialSkill,
	}
}
