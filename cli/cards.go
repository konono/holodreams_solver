package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type CardsResponse struct {
	Cards []CardInfo `json:"cards"`
}

type CardInfo struct {
	ID        string `json:"id"`
	Character string `json:"character"`
	CardName  string `json:"card_name"`
	Type      string `json:"type"`
	Rarity    int    `json:"rarity"`
	Group     string `json:"group"`
}

func runCards(args []string) {
	flags, _ := parseCommonFlags(args)

	body, err := apiGet(flags.server, "/api/cards")
	if err != nil {
		fatalf("Error: %v", err)
	}

	if flags.jsonOutput {
		fmt.Println(string(body))
		return
	}

	var resp CardsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		fatalf("パースエラー: %v", err)
	}

	w := newTabWriter()
	fmt.Fprintln(w, "ID\tCharacter\tName\tType\tGroup")
	fmt.Fprintln(w, "──\t─────────\t────\t────\t─────")
	for _, c := range resp.Cards {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", c.ID, c.Character, c.CardName, c.Type, c.Group)
	}
	w.Flush()
	fmt.Fprintf(os.Stderr, "\n%d cards\n", len(resp.Cards))
}
