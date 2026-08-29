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
	attempts := generateActiveAttempts(card, 0, 100, 0)

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
	attempts := generateActiveAttempts(card, 0, 50, 0)

	// Trigger at t=45, window [45, 50) (clipped from [45,55))
	if len(attempts) != 1 {
		t.Fatalf("got %d attempts, want 1", len(attempts))
	}
	if math.Abs(attempts[0].End-50) > 0.001 {
		t.Errorf("end=%f, want 50 (clipped)", attempts[0].End)
	}
}
