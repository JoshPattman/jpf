package main

import (
	"fmt"
	"os"

	"github.com/JoshPattman/jpf"
)

func main() {
	topic := "Dogs"
	params := []struct {
		verbosity jpf.Verbosity
		pPenalty  float64
	}{
		{jpf.LowVerbosity, 0},
		{jpf.MediumVerbosity, 0},
		{jpf.HighVerbosity, 0},
	}

	for _, param := range params {
		fmt.Println("Verbosity", param.verbosity, "PPenalty", param.pPenalty)
		poemWriter := buildPoemWriter(param.verbosity, param.pPenalty)
		poem, usage, err := poemWriter.Call(topic)
		fmt.Println("Used", usage)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(poem)
		fmt.Println()
	}
}

func buildPoemWriter(verbosity jpf.Verbosity, pPenalty float64) jpf.MapFunc[string, string] {
	model := jpf.NewOpenAIModel(
		os.Getenv("OPENAI_KEY"),
		"gpt-5",
		jpf.WithVerbosity{X: verbosity},
		jpf.WithPresencePenalty{X: pPenalty},
	)
	enc := jpf.NewRawStringMessageEncoder("Write a poem about the topic the user asks you to.")
	dec := jpf.NewRawStringResponseDecoder()
	return jpf.NewOneShotMapFunc(enc, dec, model)
}
