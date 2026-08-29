package main

import (
	"math"
	"testing"
)

func makeTimelineCard(scoreUp, interval, duration float64, probPermil int) *Card {
	return makeTestCard("c", "cute", "holo1", 30000, scoreUp, interval, duration, probPermil)
}

func dummyTeam(cards ...*Card) [5]*Card {
	var team [5]*Card
	dummy := makeTimelineCard(0, 100, 0, 0)
	for i := 0; i < 5; i++ {
		if i < len(cards) {
			team[i] = cards[i]
		} else {
			team[i] = dummy
		}
	}
	return team
}

func TestActiveTimeline_SingleCard_P1(t *testing.T) {
	// 1 card, p=1.0, interval=25, duration=10, song=100s
	// Active windows: [25,35), [50,60), [75,85)
	// At t=30: active → scoreUp=120
	// At t=40: inactive → 0
	card := makeTimelineCard(120, 25, 10, 1000)
	team := dummyTeam(card)

	// Use specific events at known times
	events := []ScoreEvent{
		{Time: 30, Weight: 1}, // inside [25,35)
		{Time: 40, Weight: 1}, // outside
		{Time: 55, Weight: 1}, // inside [50,60)
		{Time: 90, Weight: 1}, // outside
	}

	avg, loss := EvaluateActiveTimeline(team, 100, 0, events)

	// 2 of 4 events are active → avg = 120*2/4 = 60
	if math.Abs(avg-60) > 0.01 {
		t.Errorf("avg=%f, want 60", avg)
	}
	// Single card → no overlap
	if math.Abs(loss) > 0.001 {
		t.Errorf("overlap loss=%f, want 0", loss)
	}
}

func TestActiveTimeline_SingleCard_P05(t *testing.T) {
	// 1 card, p=0.5, interval=25, duration=10, song=100s
	card := makeTimelineCard(120, 25, 10, 500)
	team := dummyTeam(card)

	events := []ScoreEvent{
		{Time: 30, Weight: 1}, // inside [25,35): p=0.5
		{Time: 40, Weight: 1}, // outside: p=0
		{Time: 55, Weight: 1}, // inside [50,60): p=0.5
		{Time: 90, Weight: 1}, // outside: p=0
	}

	avg, loss := EvaluateActiveTimeline(team, 100, 0, events)

	// Expected: (120*0.5 + 0 + 120*0.5 + 0) / 4 = 30
	if math.Abs(avg-30) > 0.01 {
		t.Errorf("avg=%f, want 30", avg)
	}
	if math.Abs(loss) > 0.001 {
		t.Errorf("overlap loss=%f, want 0", loss)
	}
}

func TestActiveTimeline_TwoCards_FullOverlap(t *testing.T) {
	// 2 cards, same interval/duration, different scoreUp
	// A: 120%, p=1.0
	// B: 110%, p=1.0
	// Both active at same times → only 120% counts
	cardA := makeTimelineCard(120, 25, 10, 1000)
	cardB := makeTimelineCard(110, 25, 10, 1000)
	team := dummyTeam(cardA, cardB)

	events := []ScoreEvent{
		{Time: 30, Weight: 1}, // both active
		{Time: 40, Weight: 1}, // both inactive
	}

	avg, loss := EvaluateActiveTimeline(team, 100, 0, events)

	// At t=30: E[max] = 120*1.0 + 110*0*1.0 = 120. At t=40: 0. avg = 60
	if math.Abs(avg-60) > 0.01 {
		t.Errorf("avg=%f, want 60", avg)
	}
	// Raw = (120+110)/2 = 115, E[max] = 60
	// Overlap loss = 1 - 60/115 = ~0.478
	expectedLoss := 1.0 - 60.0/115.0
	if math.Abs(loss-expectedLoss) > 0.01 {
		t.Errorf("overlap loss=%f, want %f", loss, expectedLoss)
	}
}

func TestActiveTimeline_TwoCards_NoOverlap(t *testing.T) {
	// A: interval=20, duration=5 → windows [20,25), [40,45), [60,65), [80,85)
	// B: interval=30, duration=5 → windows [30,35), [60,65), [90,95)
	// At t=22: only A active
	// At t=32: only B active
	// At t=62: both active (overlap)
	cardA := makeTimelineCard(120, 20, 5, 1000)
	cardB := makeTimelineCard(110, 30, 5, 1000)
	team := dummyTeam(cardA, cardB)

	events := []ScoreEvent{
		{Time: 22, Weight: 1}, // A only
		{Time: 32, Weight: 1}, // B only
	}

	avg, loss := EvaluateActiveTimeline(team, 100, 0, events)

	// t=22: E[max]=120, t=32: E[max]=110, avg=(120+110)/2=115
	if math.Abs(avg-115) > 0.01 {
		t.Errorf("avg=%f, want 115", avg)
	}
	// No overlap at these specific events
	if math.Abs(loss) > 0.001 {
		t.Errorf("overlap loss=%f, want 0", loss)
	}
}

func TestActiveTimeline_SameScoreUp(t *testing.T) {
	// 2 cards with same scoreUp=120, p=0.5
	// E[max at active time] = 120 * P(A or B) = 120 * (1 - 0.5*0.5) = 90
	cardA := makeTimelineCard(120, 25, 10, 500)
	cardB := makeTimelineCard(120, 25, 10, 500)
	team := dummyTeam(cardA, cardB)

	events := []ScoreEvent{
		{Time: 30, Weight: 1}, // both potentially active
	}

	avg, _ := EvaluateActiveTimeline(team, 100, 0, events)

	// E[max] = 120 * 0.5 * 1.0 + 120 * 0.5 * (1-0.5) = 60 + 30 = 90
	if math.Abs(avg-90) > 0.01 {
		t.Errorf("avg=%f, want 90", avg)
	}
}

func TestActiveTimeline_SongEndClip(t *testing.T) {
	// Card with interval=90, duration=20, song=100s
	// Trigger at t=90, window [90, 100) (clipped from [90,110))
	card := makeTimelineCard(120, 90, 20, 1000)
	team := dummyTeam(card)

	events := []ScoreEvent{
		{Time: 95, Weight: 1},  // inside clipped window [90,100)
		{Time: 105, Weight: 1}, // outside (past song end, but window was clipped)
	}

	avg, _ := EvaluateActiveTimeline(team, 100, 0, events)

	// t=95 is inside [90,100): active. t=105 is beyond song but we still evaluate.
	// Actually the attempt window ends at 100, so t=105 is not covered.
	// avg = 120/2 = 60
	if math.Abs(avg-60) > 0.01 {
		t.Errorf("avg=%f, want 60", avg)
	}
}

func TestActiveTimeline_UniformEvents(t *testing.T) {
	// Test with auto-generated uniform events (nil scoreEvents)
	card := makeTimelineCard(120, 25, 10, 1000)
	team := dummyTeam(card)

	avg, loss := EvaluateActiveTimeline(team, 100, 0, nil)

	// 3 windows of 10s each in 100s song → ~30% uptime
	// avg ≈ 120 * 0.3 = 36
	if avg < 30 || avg > 40 {
		t.Errorf("avg=%f, expected ~36", avg)
	}
	if math.Abs(loss) > 0.001 {
		t.Errorf("overlap loss=%f, want 0 for single card", loss)
	}
}

func TestGenerateActiveAttempts_Basic(t *testing.T) {
	card := makeTimelineCard(120, 25, 10, 1000)
	attempts := generateActiveAttempts(card, 0, 100, 0, nil)

	// Triggers at t=25, 50, 75 (100 is not < 100)
	if len(attempts) != 3 {
		t.Fatalf("got %d attempts, want 3", len(attempts))
	}

	expected := [][2]float64{{25, 35}, {50, 60}, {75, 85}}
	for i, exp := range expected {
		if math.Abs(attempts[i].Start-exp[0]) > 0.001 || math.Abs(attempts[i].End-exp[1]) > 0.001 {
			t.Errorf("attempt %d: got [%f,%f), want [%f,%f)", i, attempts[i].Start, attempts[i].End, exp[0], exp[1])
		}
	}
}

func TestGenerateActiveAttempts_ClipEnd(t *testing.T) {
	card := makeTimelineCard(120, 45, 10, 1000)
	attempts := generateActiveAttempts(card, 0, 50, 0, nil)

	// Trigger at t=45, window [45, 50) (clipped from [45,55))
	if len(attempts) != 1 {
		t.Fatalf("got %d attempts, want 1", len(attempts))
	}
	if math.Abs(attempts[0].End-50) > 0.001 {
		t.Errorf("end=%f, want 50 (clipped)", attempts[0].End)
	}
}

// --- Phase 3: Special Timeline tests ---

func TestSPRateUp_BoostsProbability(t *testing.T) {
	// Card with Medium prob (450 permil = 45%), interval=25
	// SP Rate Up +45% at SP point 20 (duration 10 → [20,30))
	// Active trigger at t=25 is inside SP window → boosted = 0.45 * 1.45 = 0.6525
	card := makeTimelineCard(120, 25, 10, 450)
	spCard := makeTimelineCard(0, 100, 0, 0)
	spCard.SpecialSkill = &SpecialSkill{Duration: 10, SkillRateUp: 45}

	team := [5]*Card{card, spCard, makeTimelineCard(0, 100, 0, 0), makeTimelineCard(0, 100, 0, 0), makeTimelineCard(0, 100, 0, 0)}
	timeline := &SongTimeline{
		Duration:      100,
		SpecialPoints: [5]float64{20, 0, 0, 0, 0}, // slot 0 unused (no SP), slot 1 at t=20... wait
	}
	// slot 0 = card (no SP), slot 1 = spCard (SP at SpecialPoints[1])
	// Actually: SpecialPoints[i] is for team[i]'s SP
	// team[0] = card (no SP), team[1] = spCard (SP at SpecialPoints[1])
	timeline.SpecialPoints = [5]float64{0, 20, 0, 0, 0}

	spWindows := generateSpecialWindows(team, timeline)
	if len(spWindows) != 1 {
		t.Fatalf("expected 1 SP window, got %d", len(spWindows))
	}

	// Generate active attempts with SP windows
	attempts := generateActiveAttempts(card, 0, 100, 0, spWindows)

	// Trigger at t=25 is inside SP [20,30): boosted = 0.45 * 1.45 = 0.6525
	// Trigger at t=50 is outside: boosted = 0.45 * 1.0 = 0.45
	var probAt25, probAt50 float64
	for _, a := range attempts {
		if math.Abs(a.Start-25) < 0.001 {
			probAt25 = a.Probability
		}
		if math.Abs(a.Start-50) < 0.001 {
			probAt50 = a.Probability
		}
	}

	if math.Abs(probAt25-0.6525) > 0.001 {
		t.Errorf("prob at t=25 = %f, want 0.6525", probAt25)
	}
	if math.Abs(probAt50-0.45) > 0.001 {
		t.Errorf("prob at t=50 = %f, want 0.45", probAt50)
	}
}

func TestSPRateUp_MultipleSPs(t *testing.T) {
	// Two SPs: +45% and +50%, both active at t=25
	// boosted = 0.45 * (1 + 0.45 + 0.50) = 0.45 * 1.95 = 0.8775
	card := makeTimelineCard(120, 25, 10, 450)

	sp1 := makeTimelineCard(0, 100, 0, 0)
	sp1.SpecialSkill = &SpecialSkill{Duration: 10, SkillRateUp: 45}
	sp2 := makeTimelineCard(0, 100, 0, 0)
	sp2.SpecialSkill = &SpecialSkill{Duration: 10, SkillRateUp: 50}
	dummy := makeTimelineCard(0, 100, 0, 0)

	team := [5]*Card{card, sp1, sp2, dummy, dummy}
	timeline := &SongTimeline{
		Duration:      100,
		SpecialPoints: [5]float64{0, 20, 20, 0, 0},
	}

	spWindows := generateSpecialWindows(team, timeline)
	attempts := generateActiveAttempts(card, 0, 100, 0, spWindows)

	var probAt25 float64
	for _, a := range attempts {
		if math.Abs(a.Start-25) < 0.001 {
			probAt25 = a.Probability
		}
	}

	if math.Abs(probAt25-0.8775) > 0.001 {
		t.Errorf("prob at t=25 = %f, want 0.8775", probAt25)
	}
}

func TestSPBoundary_AtTriggerTime(t *testing.T) {
	// SP window [25, 35), Active trigger at t=25 → inside (half-open interval)
	card := makeTimelineCard(120, 25, 10, 450)
	sp := makeTimelineCard(0, 100, 0, 0)
	sp.SpecialSkill = &SpecialSkill{Duration: 10, SkillRateUp: 45}
	dummy := makeTimelineCard(0, 100, 0, 0)

	team := [5]*Card{card, sp, dummy, dummy, dummy}
	timeline := &SongTimeline{
		Duration:      100,
		SpecialPoints: [5]float64{0, 25, 0, 0, 0},
	}

	spWindows := generateSpecialWindows(team, timeline)
	attempts := generateActiveAttempts(card, 0, 100, 0, spWindows)

	var probAt25 float64
	for _, a := range attempts {
		if math.Abs(a.Start-25) < 0.001 {
			probAt25 = a.Probability
		}
	}
	// t=25 is inside [25,35) → boosted
	if math.Abs(probAt25-0.6525) > 0.001 {
		t.Errorf("prob at t=25 = %f, want 0.6525 (SP boundary)", probAt25)
	}
}

func TestSPBoundary_AtEndTime(t *testing.T) {
	// SP window [15, 25), Active trigger at t=25 → outside (half-open: end excluded)
	card := makeTimelineCard(120, 25, 10, 450)
	sp := makeTimelineCard(0, 100, 0, 0)
	sp.SpecialSkill = &SpecialSkill{Duration: 10, SkillRateUp: 45}
	dummy := makeTimelineCard(0, 100, 0, 0)

	team := [5]*Card{card, sp, dummy, dummy, dummy}
	timeline := &SongTimeline{
		Duration:      100,
		SpecialPoints: [5]float64{0, 15, 0, 0, 0},
	}

	spWindows := generateSpecialWindows(team, timeline)
	attempts := generateActiveAttempts(card, 0, 100, 0, spWindows)

	var probAt25 float64
	for _, a := range attempts {
		if math.Abs(a.Start-25) < 0.001 {
			probAt25 = a.Probability
		}
	}
	// t=25 is NOT inside [15,25) → unboosted
	if math.Abs(probAt25-0.45) > 0.001 {
		t.Errorf("prob at t=25 = %f, want 0.45 (SP ended)", probAt25)
	}
}

func TestScoreSupportTimeline(t *testing.T) {
	// Verify scoreSupportAtTime works
	windows := []SpecialWindow{
		{Start: 20, End: 30, ScoreSupport: 160, SlotIndex: 0},
		{Start: 50, End: 60, ScoreSupport: 120, SlotIndex: 1},
	}

	if s := scoreSupportAtTime(windows, 25); math.Abs(s-160) > 0.001 {
		t.Errorf("support at t=25 = %f, want 160", s)
	}
	if s := scoreSupportAtTime(windows, 55); math.Abs(s-120) > 0.001 {
		t.Errorf("support at t=55 = %f, want 120", s)
	}
	if s := scoreSupportAtTime(windows, 40); math.Abs(s) > 0.001 {
		t.Errorf("support at t=40 = %f, want 0", s)
	}
	// Overlapping: both active at once (if they overlapped)
	windows2 := []SpecialWindow{
		{Start: 20, End: 30, ScoreSupport: 160},
		{Start: 25, End: 35, ScoreSupport: 120},
	}
	if s := scoreSupportAtTime(windows2, 27); math.Abs(s-280) > 0.001 {
		t.Errorf("overlapping support at t=27 = %f, want 280", s)
	}
}

func TestSPEfficiency(t *testing.T) {
	// Card with active at t=25 (p=1.0, scoreUp=120)
	// SP with ScoreSupport=160 at t=20-30 (covers t=25 active window)
	card := makeTimelineCard(120, 25, 10, 1000)
	spCard := makeTimelineCard(0, 100, 0, 0)
	spCard.SpecialSkill = &SpecialSkill{Duration: 10, ScoreSupport: 160}
	dummy := makeTimelineCard(0, 100, 0, 0)

	team := [5]*Card{card, spCard, dummy, dummy, dummy}
	timeline := &SongTimeline{
		Duration:      100,
		SpecialPoints: [5]float64{0, 20, 0, 0, 0},
	}

	events := []ScoreEvent{
		{Time: 22, Weight: 1}, // inside SP [20,30), no active (first trigger at 25)
		{Time: 27, Weight: 1}, // inside SP [20,30), inside active [25,35)
	}

	result := EvaluateFullTimeline(team, 100, timeline, events, 0)

	// SP efficiency for slot 1 (spCard): average E[max active] during SP window
	// t=22: no active → 0, t=27: active 120 → 120
	// avg = (0+120)/2 = 60
	if math.Abs(result.SPEfficiency[1]-60) > 0.01 {
		t.Errorf("SP efficiency[1]=%f, want 60", result.SPEfficiency[1])
	}
}
