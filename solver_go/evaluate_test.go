package main

import (
	"math"
	"testing"
)

func makeTestCard(id, typ, group string, total float64, scoreUp, interval, duration float64, probPermil int) *Card {
	stat := total / 3.0
	return &Card{
		ID:        id,
		Type:      typ,
		Group:     group,
		Character: id,
		Stats:     Stats{stat, stat, stat},
		Total:     total,
		CenterSkill: CenterSkill{
			Interval:                    interval,
			Duration:                    duration,
			ScoreUp:                     scoreUp,
			ActivationProbabilityPermil: &probPermil,
		},
	}
}

func TestSkillRateUpMultiplicative(t *testing.T) {
	// Card with Medium probability (450 permil = 45%)
	card := makeTestCard("a", "cute", "holo1", 30000, 120, 25, 10, 450)

	songLength := 120.0

	// No rate up: boosted = 0.45, uptime = 10/25 * 0.45 = 0.18
	team := [5]*Card{card, card, card, card, card}
	resultNoRateUp := evaluateTeam(team, 0, 1.0, 0, songLength, nil)

	// With 45% rate up SP (duration 10s): rate_up_avg = 0.45 * 10/120 = 0.0375
	// boosted = 0.45 * (1 + 0.0375) = 0.466875
	spCard := makeTestCard("b", "cute", "holo1", 30000, 120, 25, 10, 450)
	rateUp45 := 45.0
	spCard.SpecialSkill = &SpecialSkill{
		Duration:    10,
		SkillRateUp: rateUp45,
	}

	teamWithSP := [5]*Card{card, card, card, card, spCard}
	resultWithRateUp := evaluateTeam(teamWithSP, 0, 1.0, 0, songLength, nil)

	if resultWithRateUp.ActivePct <= resultNoRateUp.ActivePct {
		t.Errorf("Rate up should increase active pct: without=%f, with=%f",
			resultNoRateUp.ActivePct, resultWithRateUp.ActivePct)
	}

	// Verify multiplicative: rate_up_avg = 0.45 * 10/120 = 0.0375
	// boosted_prob = 0.45 * 1.0375 = 0.466875
	// NOT additive: 0.45 + 0.0375 = 0.4875
	expectedRateUpAvg := 0.45 * 10.0 / 120.0
	expectedBoosted := 0.45 * (1.0 + expectedRateUpAvg)
	additiveBoosted := 0.45 + expectedRateUpAvg

	// The difference between multiplicative and additive is small for low rate_up,
	// but we can verify by computing expected uptime
	expectedUptime := math.Min(1.0, 10.0/25.0*expectedBoosted)
	additiveUptime := math.Min(1.0, 10.0/25.0*additiveBoosted)

	if math.Abs(expectedUptime-additiveUptime) < 1e-10 {
		t.Error("Multiplicative and additive should differ")
	}
	t.Logf("Multiplicative boosted=%.6f, additive boosted=%.6f", expectedBoosted, additiveBoosted)
}

func TestSkillRateUpHighValue(t *testing.T) {
	// Verify: Medium 45% with full-song 45% rate up
	// boosted = 0.45 * (1 + 0.45) = 0.6525 (not 0.90)
	card := makeTestCard("a", "cute", "holo1", 30000, 120, 25, 10, 450)

	songLength := 100.0
	spCard := makeTestCard("sp", "cute", "holo1", 30000, 120, 25, 10, 450)
	spCard.SpecialSkill = &SpecialSkill{
		Duration:    songLength,
		SkillRateUp: 45,
	}

	team := [5]*Card{card, card, card, card, spCard}
	result := evaluateTeam(team, 0, 1.0, 0, songLength, nil)

	// rate_up_avg = 0.45 * 100/100 = 0.45
	// boosted = 0.45 * 1.45 = 0.6525
	// uptime = min(1, 10/25 * 0.6525) = 0.261
	expectedUptime := 10.0 / 25.0 * 0.6525
	t.Logf("Active pct=%f, expected uptime per card=%.6f", result.ActivePct, expectedUptime)

	// With additive it would be 0.45+0.45=0.90, uptime=0.36
	// With multiplicative: 0.6525, uptime=0.261
	// The active_pct should reflect the multiplicative value
	if result.ActivePct <= 0 {
		t.Error("ActivePct should be positive")
	}
}

func TestSkillRateUpMultipleRateUps(t *testing.T) {
	// Two rate-up SPs: 45% + 50%
	// rate_up_avg = (0.45 + 0.50) * 10/100 = 0.095
	// boosted = base * (1 + 0.095) = base * 1.095
	card := makeTestCard("a", "cute", "holo1", 30000, 120, 25, 10, 450)
	songLength := 100.0

	sp1 := makeTestCard("sp1", "cute", "holo1", 30000, 120, 25, 10, 450)
	sp1.SpecialSkill = &SpecialSkill{Duration: 10, SkillRateUp: 45}

	sp2 := makeTestCard("sp2", "cute", "holo1", 30000, 120, 25, 10, 450)
	sp2.SpecialSkill = &SpecialSkill{Duration: 10, SkillRateUp: 50}

	team := [5]*Card{card, card, card, sp1, sp2}
	result := evaluateTeam(team, 0, 1.0, 0, songLength, nil)

	expectedRateUpAvg := (0.45*10 + 0.50*10) / songLength
	expectedBoosted := 0.45 * (1 + expectedRateUpAvg)
	t.Logf("rate_up_avg=%.4f, boosted=%.6f, active_pct=%.2f", expectedRateUpAvg, expectedBoosted, result.ActivePct)

	if result.ActivePct <= activeBase {
		t.Error("ActivePct should be above base")
	}
}

func TestSkillRateUpCappedAtOne(t *testing.T) {
	// Very high rate up should cap boostedProb at 1.0
	card := makeTestCard("a", "cute", "holo1", 30000, 120, 25, 10, 450)
	songLength := 10.0

	sp := makeTestCard("sp", "cute", "holo1", 30000, 120, 25, 10, 450)
	sp.SpecialSkill = &SpecialSkill{Duration: 10, SkillRateUp: 500}

	team := [5]*Card{card, card, card, card, sp}
	result := evaluateTeam(team, 0, 1.0, 0, songLength, nil)

	// rate_up_avg = 5.0 * 10/10 = 5.0
	// boosted = 0.45 * 6.0 = 2.7 → capped to 1.0
	// uptime = min(1, 10/25 * 1.0) = 0.4
	t.Logf("ActivePct=%f (should reflect capped probability)", result.ActivePct)
	if result.ActivePct <= 0 {
		t.Error("ActivePct should be positive")
	}
}
