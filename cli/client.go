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
)

func apiGet(server, path string) ([]byte, error) {
	resp, err := http.Get(server + path)
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
	resp, err := http.Post(server+path, "application/json", bytes.NewReader(data))
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
