package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type CalibrateAPIRequest struct {
	MemberIDs  []string                   `json:"member_ids"`
	LeaderID1  string                     `json:"leader_id_1"`
	GameScore1 int                        `json:"game_score_1"`
	LeaderID2  string                     `json:"leader_id_2"`
	GameScore2 int                        `json:"game_score_2"`
	CardSpecs  map[string]CardSpecCompact `json:"card_specs,omitempty"`
	SongLength *float64                   `json:"song_length,omitempty"`
}

type CardSpecCompact struct {
	Potential int  `json:"potential"`
	Level     *int `json:"level,omitempty"`
}

type CalibrateResponse struct {
	StatScale float64  `json:"stat_scale"`
	Baseline  float64  `json:"baseline"`
	Warnings  []string `json:"warnings,omitempty"`
}

func runCalibrate(args []string) {
	flags, rest := parseCommonFlags(args)

	var members string
	var leader1, leader2 string
	var score1, score2 int
	var songLength *float64
	save := false

	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--members":
			if i+1 < len(rest) {
				members = rest[i+1]
				i++
			}
		case "--leader1":
			if i+1 < len(rest) {
				leader1 = rest[i+1]
				i++
			}
		case "--score1":
			if i+1 < len(rest) {
				n, _ := strconv.Atoi(rest[i+1])
				score1 = n
				i++
			}
		case "--leader2":
			if i+1 < len(rest) {
				leader2 = rest[i+1]
				i++
			}
		case "--score2":
			if i+1 < len(rest) {
				n, _ := strconv.Atoi(rest[i+1])
				score2 = n
				i++
			}
		case "--song-length":
			if i+1 < len(rest) {
				f, _ := strconv.ParseFloat(rest[i+1], 64)
				songLength = &f
				i++
			}
		case "--save":
			save = true
		case "--help", "-h":
			fmt.Println(`holosolve calibrate — キャリブレーション

Required:
  --members ID1,ID2,ID3,ID4,ID5   メンバー5人のカードID
  --leader1 ID                     リーダー1のカードID
  --score1 N                       リーダー1のゲーム内スコア
  --leader2 ID                     リーダー2のカードID
  --score2 N                       リーダー2のゲーム内スコア

Options:
  --song-length SEC    曲長（秒）
  --save               結果を設定ファイルに保存`)
			return
		}
	}

	if members == "" || leader1 == "" || leader2 == "" || score1 == 0 || score2 == 0 {
		fatalf("必須オプションが不足しています。--help を参照してください。")
	}

	memberIDs := strings.Split(members, ",")
	if len(memberIDs) != 5 {
		fatalf("メンバーは5人指定してください（カンマ区切り）")
	}

	cfg, err := loadConfig(flags.configPath)
	if err != nil {
		fatalf("Error: %v", err)
	}

	cardSpecs := map[string]CardSpecCompact{}
	for _, id := range memberIDs {
		spec := CardSpecCompact{Potential: cfg.DefaultPotential}
		if p, ok := cfg.Potentials[id]; ok {
			spec.Potential = p
		}
		if cfg.LevelEnabled {
			lv := cfg.DefaultLevel
			if l, ok := cfg.Levels[id]; ok {
				lv = l
			}
			spec.Level = &lv
		}
		cardSpecs[id] = spec
	}

	req := CalibrateAPIRequest{
		MemberIDs:  memberIDs,
		LeaderID1:  leader1,
		GameScore1: score1,
		LeaderID2:  leader2,
		GameScore2: score2,
		CardSpecs:  cardSpecs,
		SongLength: songLength,
	}

	body, err := apiPost(flags.server, "/api/calibrate", req)
	if err != nil {
		fatalf("Error: %v", err)
	}

	if flags.jsonOutput {
		fmt.Println(string(body))
		return
	}

	var resp CalibrateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		fatalf("パースエラー: %v", err)
	}

	fmt.Printf("stat_scale: %.6f\n", resp.StatScale)
	fmt.Printf("baseline:   %.1f\n", resp.Baseline)
	for _, w := range resp.Warnings {
		fmt.Printf("Warning: %s\n", w)
	}

	if save {
		cfg.StatScale = &resp.StatScale
		cfg.Baseline = &resp.Baseline
		if err := saveConfig(flags.configPath, cfg); err != nil {
			fatalf("設定ファイル保存エラー: %v", err)
		}
		fmt.Printf("\n%s に保存しました\n", flags.configPath)
	}
}
