package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

var httpClient = &http.Client{Timeout: 5 * time.Minute}

func apiGet(server, path string) ([]byte, error) {
	resp, err := httpClient.Get(server + path)
	if err != nil {
		return nil, fmt.Errorf("接続エラー: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func apiPost(server, path string, payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Post(server+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("接続エラー: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func fetchAllCardIDs(server string) ([]string, error) {
	body, err := apiGet(server, "/api/cards")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Cards []struct {
			ID string `json:"id"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	ids := make([]string, len(resp.Cards))
	for i, c := range resp.Cards {
		ids[i] = c.ID
	}
	return ids, nil
}

func printWarnings(data []byte) {
	var w struct {
		Warnings []string `json:"warnings"`
	}
	if json.Unmarshal(data, &w) == nil {
		for _, msg := range w.Warnings {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
		}
	}
}

func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

type commonFlags struct {
	server     string
	configPath string
	jsonOutput bool
}

func parseCommonFlags(args []string) (commonFlags, []string) {
	f := commonFlags{
		server:     defaultServer,
		configPath: "holosolve.json",
	}
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--server":
			if i+1 < len(args) {
				f.server = args[i+1]
				i++
			}
		case "--config":
			if i+1 < len(args) {
				f.configPath = args[i+1]
				i++
			}
		case "--json":
			f.jsonOutput = true
		default:
			rest = append(rest, args[i])
		}
	}
	return f, rest
}

func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		return s
	}
	result := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
