package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type RecommendAPIRequest struct {
	Cards               []CardSpec `json:"cards"`
	StatScale           float64    `json:"stat_scale"`
	Baseline            float64    `json:"baseline"`
	FixedLeaderID       *string    `json:"fixed_leader_id,omitempty"`
	CostumeOnlyLeaderID *string    `json:"costume_only_leader_id,omitempty"`
	TopN                int        `json:"top_n"`
	AcquireCount        int        `json:"acquire_count"`
	SongLength          *float64   `json:"song_length,omitempty"`
	SweepCostumes       bool       `json:"sweep_costumes,omitempty"`
}

type RecommendResponse struct {
	BaseScore       int                   `json:"base_score"`
	AcquireCount    int                   `json:"acquire_count"`
	Recommendations []RecommendResultJSON `json:"recommendations"`
}

type RecommendResultJSON struct {
	Rank     int              `json:"rank"`
	Cards    []RecommendCard  `json:"cards"`
	NewScore int              `json:"new_score"`
	Delta    int              `json:"delta"`
	BestTeam RecommendBestTeam `json:"best_team"`
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

type RecommendBestTeam struct {
	LeaderID            string   `json:"leader_id"`
	MemberIDs           []string `json:"member_ids"`
	CostumeOnlyLeaderID *string  `json:"costume_only_leader_id,omitempty"`
}

func runRecommend(args []string) {
	flags, rest := parseCommonFlags(args)

	topN := 5
	acquireCount := 1
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
		case "--acquire-count":
			if i+1 < len(rest) {
				n, _ := strconv.Atoi(rest[i+1])
				acquireCount = n
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
			fmt.Println(`holosolve recommend — カード推薦

Options:
  --top-n N              上位N件 (default: 5)
  --acquire-count N      同時取得枚数 (default: 1)
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

	req := RecommendAPIRequest{
		Cards:               cfg.buildCardSpecs(),
		StatScale:           cfg.statScaleVal(),
		Baseline:            cfg.baselineVal(),
		FixedLeaderID:       leaderID,
		CostumeOnlyLeaderID: costumeLeaderID,
		TopN:                topN,
		AcquireCount:        acquireCount,
		SongLength:          songLength,
		SweepCostumes:       sweepCostumes,
	}

	body, err := apiPost(flags.server, "/api/recommend", req)
	if err != nil {
		fatalf("Error: %v", err)
	}

	if flags.jsonOutput {
		fmt.Println(string(body))
		return
	}

	var resp RecommendResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		fatalf("パースエラー: %v", err)
	}

	fmt.Printf("現在のベストスコア: %s\n\n", formatNumber(resp.BaseScore))

	w := newTabWriter()
	fmt.Fprintln(w, "Rank\tDelta\tNew Score\tCard\tAction")
	fmt.Fprintln(w, "────\t─────\t─────────\t────\t──────")
	for _, r := range resp.Recommendations {
		cards := make([]string, 0, len(r.Cards))
		actions := make([]string, 0, len(r.Cards))
		for _, c := range r.Cards {
			cards = append(cards, c.CardName+" ("+c.Character+")")
			if c.Action == "acquire" {
				actions = append(actions, fmt.Sprintf("新規取得→%d凸", c.TargetPotential))
			} else {
				cur := 0
				if c.CurrentPotential != nil {
					cur = *c.CurrentPotential
				}
				actions = append(actions, fmt.Sprintf("%d凸→%d凸", cur, c.TargetPotential))
			}
		}
		fmt.Fprintf(w, "#%d\t+%s\t%s\t%s\t%s\n",
			r.Rank, formatNumber(r.Delta), formatNumber(r.NewScore),
			strings.Join(cards, " + "), strings.Join(actions, " + "))
	}
	w.Flush()
}
