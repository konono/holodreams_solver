package main

import (
	"fmt"
	"os"
)

const defaultServer = "http://localhost:8000"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		runInit(os.Args[2:])
	case "cards":
		runCards(os.Args[2:])
	case "solve":
		runSolve(os.Args[2:])
	case "recommend":
		runRecommend(os.Args[2:])
	case "calibrate":
		runCalibrate(os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`holosolve — HoloSolve API Client CLI

Usage:
  holosolve <command> [options]

Commands:
  init         設定ファイル初期化（UIの「IDコピー」JSONをstdinから読む）
  cards        カード一覧表示
  solve        最強編成探索
  recommend    カード推薦
  calibrate    キャリブレーション

Global Options:
  --server URL    サーバーURL (default: http://localhost:8000)
  --config PATH   設定ファイルパス (default: ./holosolve.json)
  --json          JSON出力

Run 'holosolve <command> --help' for command-specific help.`)
}
