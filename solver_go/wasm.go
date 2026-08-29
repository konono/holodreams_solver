//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"
)

var wasmCardsFile *CardsFile

func jsInitCards(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return errorJSON("missing cards JSON")
	}
	jsonStr := args[0].String()
	var cf CardsFile
	if err := json.Unmarshal([]byte(jsonStr), &cf); err != nil {
		return errorJSON(err.Error())
	}
	wasmCardsFile = &cf
	cachedCardsFile = &cf
	return js.ValueOf(map[string]interface{}{"ok": true, "count": len(cf.Cards)})
}

func jsSolve(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 || wasmCardsFile == nil {
		return js.ValueOf(errorJSON("not initialized"))
	}

	var input CLIInput
	if err := json.Unmarshal([]byte(args[0].String()), &input); err != nil {
		return js.ValueOf(errorJSON(err.Error()))
	}

	result, err := dispatchAction(input, wasmCardsFile)
	if err != nil {
		return js.ValueOf(errorJSON(err.Error()))
	}

	out, err := json.Marshal(result)
	if err != nil {
		return js.ValueOf(errorJSON(err.Error()))
	}
	return js.ValueOf(string(out))
}

func errorJSON(msg string) string {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return string(b)
}

func main() {
	js.Global().Set("_solverInitCards", js.FuncOf(jsInitCards))
	js.Global().Set("_solverCall", js.FuncOf(jsSolve))
	js.Global().Set("_solverReady", js.ValueOf(true))
	select {}
}
