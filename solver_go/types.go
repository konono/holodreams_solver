package main

import (
	"encoding/json"
	"fmt"
)

type Stats struct {
	Performance float64 `json:"performance"`
	Technique   float64 `json:"technique"`
	Sense       float64 `json:"sense"`
}

func (s Stats) Total() float64 {
	return s.Performance + s.Technique + s.Sense
}

type CenterSkill struct {
	Interval                   float64  `json:"interval"`
	Duration                   float64  `json:"duration"`
	ScoreUp                    float64  `json:"score_up"`
	Condition                  *string  `json:"condition"`
	ConditionalScoreUp         *float64 `json:"conditional_score_up"`
	ActivationProbabilityPermil *int    `json:"activation_probability_permil"`
}

type SupportTarget struct {
	TypeMatch string `json:"type_match"`
	Group     string `json:"group"`
	Count     int    `json:"count"`
}

type SupportSkillRaw struct {
	EffectType string          `json:"effect_type"`
	Value      float64         `json:"value"`
	Condition  json.RawMessage `json:"condition"`
	Stat       string          `json:"stat"`
	Target     json.RawMessage `json:"target"`
}

type SupportSkill struct {
	EffectType string
	Value      float64
	Condition  *ConditionObj
	Stat       string
	Target     *SupportTarget
}

type ConditionObj struct {
	Type     string `json:"type"`
	TypeName string `json:"type_name"`
	Group    string `json:"group"`
	MinCount int    `json:"min_count"`
}

type CostumeEffect struct {
	Stat  string  `json:"stat"`
	Value float64 `json:"value"`
}

type CostumeSkill struct {
	Condition *ConditionObj    `json:"condition"`
	Effects   []CostumeEffect `json:"effects"`
}

type SpecialSkill struct {
	Duration           float64 `json:"duration"`
	ScoreSupport       float64 `json:"score_support"`
	SkillRateUp        float64 `json:"skill_rate_up"`
	SkillRateCondition *string `json:"skill_rate_condition"`
}

type PotentialData struct {
	Potential        int         `json:"potential"`
	ParamBonusPermil int         `json:"param_bonus_permil"`
	RefStatsLv80     Stats       `json:"ref_stats_lv80"`
	CenterSkill      CenterSkill `json:"center_skill"`
	SupportSkillRaw  SupportSkillRaw `json:"support_skill"`
	CostumeSkill     CostumeSkill    `json:"costume_skill"`
	SpecialSkill     *SpecialSkill   `json:"special_skill"`
}

type CardRaw struct {
	ID                string          `json:"id"`
	HolodoriID        string          `json:"holodori_id"`
	Character         string          `json:"character"`
	CardName          string          `json:"card_name"`
	Rarity            int             `json:"rarity"`
	Type              string          `json:"type"`
	Group             string          `json:"group"`
	Variant           string          `json:"variant"`
	CardLevelGroupID  string          `json:"card_level_group_id"`
	Permil            *Stats          `json:"permil"`
	PotentialData     []PotentialData `json:"potential_data"`
	// Legacy flat fields for bench_go compatibility
	Stats        Stats           `json:"stats"`
	CenterSkill  CenterSkill     `json:"center_skill"`
	SupportSkill SupportSkillRaw `json:"support_skill"`
	CostumeSkill CostumeSkill    `json:"costume_skill"`
	SpecialSkill *SpecialSkill   `json:"special_skill"`
}

type Card struct {
	ID           string
	HolodoriID   string
	Character    string
	CardName     string
	Rarity       int
	Type         string
	Group        string
	Variant      string
	Potential    int
	Level        int
	Stats        Stats
	Total        float64
	CenterSkill  CenterSkill
	SupportSkill SupportSkill
	CostumeSkill CostumeSkill
	SpecialSkill *SpecialSkill
}

type CardsFile struct {
	Cards       []CardRaw                    `json:"cards"`
	LevelTables map[string]map[string]int `json:"level_tables"`
}

type EvalResult struct {
	UnitScore      float64
	TotalPower     float64
	MemberParams   float64
	CostumeContrib float64
	SupportContrib float64
	ActivePct      float64
	CostumeSBPct   float64
	PassiveSBPct   float64
	SpecialPct     float64
	ScoreBonus     float64
	CostumeSSVal   float64
	SupportSSVal   float64
}

type SolveResult struct {
	Score               EvalResult
	LeaderIdx           int
	TeamIDs             [5]string
	CostumeOnlyLeaderID string
}

type BaseScores struct {
	BasePower     float64
	BaseBonus     float64
	TotalPerf     float64
	TotalTech     float64
	TotalSense    float64
	MemberParams  float64
	SupportContrib float64
	ActivePct     float64
	PassiveSBPct  float64
	SpecialPct    float64
	SupportSS     float64
	TypeCounts    map[string]int
	GroupCounts   map[string]int
	Leader        *Card
}

type fixedFloat float64

func (f fixedFloat) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.1f", float64(f))), nil
}

type fixedFloat2 float64

func (f fixedFloat2) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.2f", float64(f))), nil
}

// JSON output types

type JSONResult struct {
	Rank                int          `json:"rank"`
	UnitScore           int          `json:"unit_score"`
	TotalPower          int          `json:"total_power"`
	ScoreBonus          fixedFloat   `json:"score_bonus"`
	ActivePct           fixedFloat   `json:"active_pct"`
	CostumeSBPct        fixedFloat   `json:"costume_sb_pct"`
	PassiveSBPct        fixedFloat   `json:"passive_sb_pct"`
	SpecialPct          fixedFloat   `json:"special_pct"`
	LeaderID            string       `json:"leader_id"`
	CostumeOnlyLeaderID *string      `json:"costume_only_leader_id"`
	MemberIDs           []string     `json:"member_ids"`
	Stability           map[string]int `json:"stability,omitempty"`
}

type JSONOutput struct {
	TotalCombinations int          `json:"total_combinations"`
	StatScale         float64      `json:"stat_scale"`
	Baseline          float64      `json:"baseline"`
	Results           []JSONResult `json:"results"`
}

// Input types for CLI

type CLIInput struct {
	Action  string `json:"action"`

	// solve/recommend common
	Cards    json.RawMessage `json:"cards"`
	TopN     int             `json:"top_n"`
	StatScale *float64       `json:"stat_scale"`
	Baseline  *float64       `json:"baseline"`
	SongLength *float64      `json:"song_length"`

	// timeline (optional)
	SongTimeline       *SongTimeline      `json:"song_timeline"`
	ChartScoreData     *ChartScore        `json:"chart_score"`
	StabilityCharts    []ChartScore       `json:"stability_charts"`
	PlayAssumption     *PlayAssumption    `json:"play_assumption"`
	TimelineTopN       int                `json:"timeline_top_n"`

	// solve
	FixedLeaderID       *string   `json:"fixed_leader_id"`
	CostumeOnlyLeaderID *string   `json:"costume_only_leader_id"`
	StabilityLengths    []float64 `json:"stability_lengths"`
	SweepCostumes       bool      `json:"sweep_costumes"`

	// recommend
	AcquireCount int `json:"acquire_count"`

	// calibrate
	MemberIDs   []string            `json:"member_ids"`
	LeaderID1   string              `json:"leader_id_1"`
	GameScore1  int                 `json:"game_score_1"`
	LeaderID2   string              `json:"leader_id_2"`
	GameScore2  int                 `json:"game_score_2"`
	CardSpecs   map[string]CardSpec `json:"card_specs"`
}

type CardSpec struct {
	ID        string `json:"id"`
	Potential int    `json:"potential"`
	Level     *int   `json:"level"`
}

type CalibrateOutput struct {
	StatScale float64  `json:"stat_scale"`
	Baseline  float64  `json:"baseline"`
	Warnings  []string `json:"warnings,omitempty"`
}

type RecommendCard struct {
	CardID           string  `json:"card_id"`
	CardName         string  `json:"card_name"`
	Character        string  `json:"character"`
	Action           string  `json:"action"`
	CurrentPotential *int    `json:"current_potential"`
	TargetPotential  int     `json:"target_potential"`
	Cost             int     `json:"cost"`
}

type RecommendResult struct {
	Rank     int               `json:"rank"`
	Cards    []RecommendCard   `json:"cards"`
	NewScore int               `json:"new_score"`
	Delta    int               `json:"delta"`
	BestTeam RecommendBestTeam `json:"best_team"`
}

type RecommendBestTeam struct {
	LeaderID            string   `json:"leader_id"`
	MemberIDs           []string `json:"member_ids"`
	CostumeOnlyLeaderID *string  `json:"costume_only_leader_id,omitempty"`
}

type TimelineJSONResult struct {
	Rank              int        `json:"rank"`
	UnitScore         int        `json:"unit_score"`
	LiveScoreIndex    int        `json:"live_score_index"`
	SkillEfficiency   fixedFloat2 `json:"skill_efficiency"`
	Top1Pct           fixedFloat2 `json:"top1_pct"`
	ActiveOverlapLoss fixedFloat `json:"active_overlap_loss"`
	MemberIDs         []string   `json:"member_ids"`
	SPEfficiency      []float64  `json:"sp_efficiency,omitempty"`
}

type TimelineStabilityEntry struct {
	MusicID    string `json:"music_id"`
	Difficulty string `json:"difficulty"`
	Duration   int    `json:"duration"`
	TopLSI     int    `json:"top_lsi"`
}

type TimelineJSONOutput struct {
	LegacyResults []JSONResult              `json:"legacy_results"`
	Timeline      []TimelineJSONResult      `json:"timeline_results"`
	CandidatePool int                       `json:"candidate_pool"`
	BaselineLSI   int                       `json:"baseline_lsi"`
	Stability     []TimelineStabilityEntry  `json:"stability,omitempty"`
}

type RecommendOutput struct {
	BaseScore       int               `json:"base_score"`
	AcquireCount    int               `json:"acquire_count"`
	Recommendations []RecommendResult `json:"recommendations"`
}
