package main

// SongTimeline holds per-song data for the Timeline Expected Score Engine.
type SongTimeline struct {
	Duration      float64        `json:"duration"`
	SpecialPoints [5]float64     `json:"special_points"`
	ScoreEvents   []ScoreEvent   `json:"score_events"`
}

// ScoreEvent represents a single scoring event (note) in a song chart.
type ScoreEvent struct {
	Time       float64 `json:"time"`
	ComboIndex int     `json:"combo_index"`
	Weight     float64 `json:"weight"`
}

// ActiveAttempt represents a single Active Skill trigger attempt and its window.
type ActiveAttempt struct {
	Start       float64
	End         float64
	Probability float64
	ScoreUp     float64
	CardIndex   int
}

// SpecialWindow represents the time window of a Special Skill activation.
type SpecialWindow struct {
	Start              float64
	End                float64
	ScoreSupport       float64
	SkillRateUp        float64
	SkillRateCondition *string
	SlotIndex          int
}

// PlayAssumption defines the assumed play conditions for Timeline evaluation.
// Currently AP (All Perfect) is assumed and this struct is not yet used in evaluation.
// Future: support GREAT/MISS rates, manual play, AUTO mode.
type PlayAssumption struct {
	AllPerfect   bool `json:"all_perfect"`
	StartingLife int  `json:"starting_life"`
}

// ScoreBin represents an aggregated time bin from chart_scores.json.
// Individual note positions are destroyed; only scoring-relevant totals remain.
type ScoreBin struct {
	T float64 `json:"t"` // bin center time (seconds)
	N int     `json:"n"` // note count in this bin
	W int     `json:"w"` // total weight (sum of note weights)
	C int     `json:"c"` // combo index at end of bin
}

// ChartScore holds the publishable per-chart scoring data.
type ChartScore struct {
	MusicID       string     `json:"music_id"`
	Difficulty    string     `json:"difficulty"`
	Duration      float64    `json:"duration"`
	BPM           float64    `json:"bpm"`
	TotalNotes    int        `json:"total_notes"`
	BinSize       float64    `json:"bin_size"`
	SpecialPoints []float64  `json:"special_points"`
	Bins          []ScoreBin `json:"bins"`
}

// BinsToScoreEvents converts binned chart data into ScoreEvents for the Timeline Engine.
func BinsToScoreEvents(bins []ScoreBin) []ScoreEvent {
	var events []ScoreEvent
	for _, b := range bins {
		if b.N == 0 {
			continue
		}
		events = append(events, ScoreEvent{
			Time:       b.T,
			ComboIndex: b.C,
			Weight:     float64(b.W),
		})
	}
	return events
}

// ChartScoreToTimeline converts a ChartScore into a SongTimeline.
// The special_points are padded or truncated to exactly 5 entries.
func ChartScoreToTimeline(cs *ChartScore) *SongTimeline {
	var sp [5]float64
	for i := 0; i < 5 && i < len(cs.SpecialPoints); i++ {
		sp[i] = cs.SpecialPoints[i]
	}
	return &SongTimeline{
		Duration:      cs.Duration,
		SpecialPoints: sp,
		ScoreEvents:   BinsToScoreEvents(cs.Bins),
	}
}

// TimelineEvalResult holds the output of Timeline Expected Score evaluation.
type TimelineEvalResult struct {
	LiveScoreIndex    float64
	ExpectedActive    float64
	ActiveOverlapLoss float64
	SPEfficiency      [5]float64
}
