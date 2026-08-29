package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type SolveRequest struct {
	Cards               []CardSpec `json:"cards"`
	StatScale           float64    `json:"stat_scale"`
	Baseline            float64    `json:"baseline"`
	FixedLeaderID       *string    `json:"fixed_leader_id,omitempty"`
	CostumeOnlyLeaderID *string    `json:"costume_only_leader_id,omitempty"`
	TopN                int        `json:"top_n"`
	SongLength          *float64   `json:"song_length,omitempty"`
	SweepCostumes       bool       `json:"sweep_costumes,omitempty"`
}

type SolveResponse struct {
	TotalCombinations int           `json:"total_combinations"`
	Results           []SolveResult `json:"results"`
}

type SolveResult struct {
	Rank                int        `json:"rank"`
	UnitScore           int        `json:"unit_score"`
	TotalPower          int        `json:"total_power"`
	ScoreBonus          float64    `json:"score_bonus"`
	ActivePct           float64    `json:"active_pct"`
	CostumeSBPct        float64    `json:"costume_sb_pct"`
	PassiveSBPct        float64    `json:"passive_sb_pct"`
	SpecialPct          float64    `json:"special_pct"`
	LeaderID            string     `json:"leader_id"`
	CostumeOnlyLeaderID *string    `json:"costume_only_leader_id"`
	MemberIDs           []string   `json:"member_ids"`
	Stability           map[string]int `json:"stability,omitempty"`
}

func runSolve(args []string) {
	flags, rest := parseCommonFlags(args)

	topN := 10
	var leaderID, costumeLeaderID *string
	var songLength *float64
	sweepCostumes := false

	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--top-n":
			if i+1 < len(rest) {
				n, _ := strconv.Atoi(rest[i+1])
				topN = n
				i++
			}
		case "--leader":
			if i+1 < len(rest) {
				s := rest[i+1]
				leaderID = &s
				i++
			}
		case "--costume-leader":
			if i+1 < len(rest) {
				s := rest[i+1]
				costumeLeaderID = &s
				i++
			}
		case "--song-length":
			if i+1 < len(rest) {
				f, _ := strconv.ParseFloat(rest[i+1], 64)
				songLength = &f
				i++
			}
		case "--sweep-costumes":
			sweepCostumes = true
		case "--help", "-h":
			fmt.Println(`holosolve solve — 最強編成探索

Options:
  --top-n N              上位N件 (default: 10)
  --leader ID            リーダー固定
  --costume-leader ID    衣装リーダー固定
  --song-length SEC      曲長（秒）
  --sweep-costumes       衣装スイープ有効`)
			return
		}
	}

	cfg, err := loadConfig(flags.configPath)
	if err != nil {
		fatalf("Error: %v", err)
	}

	req := SolveRequest{
		Cards:               cfg.buildCardSpecs(),
		StatScale:           cfg.statScaleVal(),
		Baseline:            cfg.baselineVal(),
		FixedLeaderID:       leaderID,
		CostumeOnlyLeaderID: costumeLeaderID,
		TopN:                topN,
		SongLength:          songLength,
		SweepCostumes:       sweepCostumes,
	}

	body, err := apiPost(flags.server, "/api/solve", req)
	if err != nil {
		fatalf("Error: %v", err)
	}

	if flags.jsonOutput {
		fmt.Println(string(body))
		return
	}

	var resp SolveResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		fatalf("パースエラー: %v", err)
	}

	fmt.Printf("探索組合せ数: %s\n\n", formatNumber(resp.TotalCombinations))

	w := newTabWriter()
	fmt.Fprintln(w, "Rank\tScore\tPower\tSB%\tLeader\tMembers")
	fmt.Fprintln(w, "────\t─────\t─────\t───\t──────\t───────")
	for _, r := range resp.Results {
		members := make([]string, 0, len(r.MemberIDs))
		for _, m := range r.MemberIDs {
			if m != r.LeaderID {
				members = append(members, m)
			}
		}
		costume := ""
		if r.CostumeOnlyLeaderID != nil && *r.CostumeOnlyLeaderID != "" {
			costume = " (衣装:" + *r.CostumeOnlyLeaderID + ")"
		}
		fmt.Fprintf(w, "#%d\t%s\t%s\t%.1f%%\t%s%s\t%s\n",
			r.Rank, formatNumber(r.UnitScore), formatNumber(r.TotalPower),
			r.ScoreBonus, r.LeaderID, costume, strings.Join(members, ", "))
	}
	w.Flush()
}
