package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Config struct {
	V                int            `json:"v"`
	IDs              []string       `json:"ids"`
	AllCards         bool           `json:"allCards"`
	Potentials       map[string]int `json:"potentials"`
	Levels           map[string]int `json:"levels"`
	DefaultPotential int            `json:"defaultPotential"`
	DefaultLevel     int            `json:"defaultLevel"`
	LevelEnabled     bool           `json:"levelEnabled"`
	StatScale        *float64       `json:"statScale,omitempty"`
	Baseline         *float64       `json:"baseline,omitempty"`
}

type CardSpec struct {
	ID        string `json:"id"`
	Potential int    `json:"potential"`
	Level     *int   `json:"level,omitempty"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("設定ファイルを読めません: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("設定ファイルのパースに失敗: %w", err)
	}
	return &cfg, nil
}

func saveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func (cfg *Config) buildCardSpecs() []CardSpec {
	specs := make([]CardSpec, 0, len(cfg.IDs))
	for _, id := range cfg.IDs {
		pot := cfg.DefaultPotential
		if p, ok := cfg.Potentials[id]; ok {
			pot = p
		}
		spec := CardSpec{ID: id, Potential: pot}
		if cfg.LevelEnabled {
			lv := cfg.DefaultLevel
			if l, ok := cfg.Levels[id]; ok {
				lv = l
			}
			spec.Level = &lv
		}
		specs = append(specs, spec)
	}
	return specs
}

func (cfg *Config) statScaleVal() float64 {
	if cfg.StatScale != nil {
		return *cfg.StatScale
	}
	return 1.0
}

func (cfg *Config) baselineVal() float64 {
	if cfg.Baseline != nil {
		return *cfg.Baseline
	}
	return 0
}

func runInit(args []string) {
	configPath := "holosolve.json"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			}
		}
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stdin読み取りエラー: %v\n", err)
		os.Exit(1)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "JSONパースエラー: %v\n", err)
		os.Exit(1)
	}

	if err := saveConfig(configPath, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "書き込みエラー: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s に保存しました（カード %d枚）\n", configPath, len(cfg.IDs))
}
