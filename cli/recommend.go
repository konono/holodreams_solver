package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type RecommendAPIRequest struct {
	Cards               []CardSpec      `json:"cards"`
	StatScale           float64         `json:"stat_scale"`
	Baseline            float64         `json:"baseline"`
	FixedLeaderID       *string         `json:"fixed_leader_id,omitempty"`
	CostumeOnlyLeaderID *string         `json:"costume_only_leader_id,omitempty"`
	TopN                int             `json:"top_n"`
	AcquireCount        int             `json:"acquire_count"`
	SongLength          *float64        `json:"song_length,omitempty"`
	SweepCostumes       bool            `json:"sweep_costumes,omitempty"`
	IncludePotential    bool            `json:"include_potential,omitempty"`
	NewCardLevel        int             `json:"new_card_level,omitempty"`
	ChartScore          json.RawMessage `json:"chart_score,omitempty"`
	BoardSearchMode     string          `json:"board_search_mode,omitempty"`
}

type RecommendResponse struct {
	BaseScore       int                   `json:"base_score"`
	AcquireCount    int                   `json:"acquire_count"`
	Recommendations []RecommendResultJSON `json:"recommendations"`
	PotentialCards  []PotentialCardJSON   `json:"potential_cards"`
}

type PotentialCardJSON struct {
	CardID            string `json:"card_id"`
	CardName          string `json:"card_name"`
	Character         string `json:"character"`
	CurrentPotential  *int   `json:"current_potential"`
	FirstUsefulCopies int    `json:"first_useful_copies"`
	Steps             []struct {
		Copies          int `json:"copies"`
		TargetPotential int `json:"target_potential"`
		Delta           int `json:"delta"`
	} `json:"steps"`
}

type RecommendResultJSON struct {
	Rank     int               `json:"rank"`
	Cards    []RecommendCard   `json:"cards"`
	NewScore int               `json:"new_score"`
	Delta    int               `json:"delta"`
	BestTeam RecommendBestTeam `json:"best_team"`
}

type RecommendCard struct {
	CardID           string `json:"card_id"`
	CardName         string `json:"card_name"`
	Character        string `json:"character"`
	Action           string `json:"action"`
	CurrentPotential *int   `json:"current_potential"`
	TargetPotential  int    `json:"target_potential"`
	Cost             int    `json:"cost"`
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
	includePotential := false
	newCardLevel := 80
	chartPath := ""
	boardMode := "balanced"

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
		case "--potential":
			includePotential = true
		case "--new-card-level":
			if i+1 < len(rest) {
				n, _ := strconv.Atoi(rest[i+1])
				newCardLevel = n
				i++
			}
		case "--chart-score":
			if i+1 < len(rest) {
				chartPath = rest[i+1]
				i++
			}
		case "--board-mode":
			if i+1 < len(rest) {
				boardMode = rest[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Println(`holosolve recommend — カード推薦

Options:
  --top-n N              上位N件 (default: 5)
  --acquire-count N      同時取得枚数 (default: 1)
  --leader ID            リーダー固定
  --costume-leader ID    衣装リーダー固定
  --song-length SEC      曲長（秒）
  --sweep-costumes       衣装スイープ有効
  --potential            全カードの1〜5枚投入時の推移を表示
  --new-card-level N     未所持カードの想定レベル (default: 80)
  --chart-score PATH    譜面JSONを使ってTimeline分析
  --board-mode MODE     fast / balanced / exhaustive (default: balanced)`)
			return
		}
	}

	cfg, err := loadConfig(flags.configPath)
	if err != nil {
		fatalf("Error: %v", err)
	}

	var chartScore json.RawMessage
	if chartPath != "" {
		chartScore, err = os.ReadFile(chartPath)
		if err != nil || !json.Valid(chartScore) {
			fatalf("譜面JSONを読み込めません: %s", chartPath)
		}
	}
	req := RecommendAPIRequest{
		Cards:               cfg.buildCardSpecs(flags.server),
		StatScale:           cfg.statScaleVal(),
		Baseline:            cfg.baselineVal(),
		FixedLeaderID:       leaderID,
		CostumeOnlyLeaderID: costumeLeaderID,
		TopN:                topN,
		AcquireCount:        acquireCount,
		SongLength:          songLength,
		SweepCostumes:       sweepCostumes,
		IncludePotential:    includePotential,
		NewCardLevel:        newCardLevel,
		ChartScore:          chartScore,
		BoardSearchMode:     boardMode,
	}

	body, err := apiPost(flags.server, "/api/recommend", req)
	if err != nil {
		fatalf("Error: %v", err)
	}

	if flags.jsonOutput {
		printWarnings(body)
		fmt.Println(string(body))
		return
	}

	printWarnings(body)

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
	if includePotential {
		fmt.Println("\nカード別の将来性（各カードへ集中投資）:")
		pw := newTabWriter()
		fmt.Fprintln(pw, "Card\tNow +1\tBest within +5\tFirst useful")
		for _, card := range resp.PotentialCards {
			if len(card.Steps) == 0 {
				continue
			}
			first := card.Steps[0].Delta
			last := card.Steps[len(card.Steps)-1]
			milestone := "効果なし"
			if card.FirstUsefulCopies > 0 {
				milestone = fmt.Sprintf("%d枚", card.FirstUsefulCopies)
			}
			fmt.Fprintf(pw, "%s %s\t+%s\t+%s (%d凸/%d枚)\t%s\n",
				card.Character, card.CardName, formatNumber(first), formatNumber(last.Delta),
				last.TargetPotential, last.Copies, milestone)
		}
		pw.Flush()
	}
}
