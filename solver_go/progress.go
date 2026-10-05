package main

// stageCallback reports recommendation phases separately from the legacy
// combination counter. A phase may have no known total while it starts.
var stageCallback func(phase string, current, total int)

func reportStage(phase string, current, total int) {
	if stageCallback != nil {
		stageCallback(phase, current, total)
	}
}
