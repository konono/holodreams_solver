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
	Start            float64
	End              float64
	ScoreSupport     float64
	SkillRateUp      float64
	SlotIndex        int
}

// PlayAssumption defines the assumed play conditions for Timeline evaluation.
type PlayAssumption struct {
	AllPerfect   bool `json:"all_perfect"`
	StartingLife int  `json:"starting_life"`
}

// TimelineEvalResult holds the output of Timeline Expected Score evaluation.
type TimelineEvalResult struct {
	LiveScoreIndex   float64
	ExpectedActive   float64
	ActiveOverlapLoss float64
	SPEfficiency     [5]float64
}
