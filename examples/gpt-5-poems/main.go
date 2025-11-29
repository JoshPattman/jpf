package main

import (
	"context"
	"fmt"
	"os"

	"github.com/JoshPattman/jpf"
)

func main() {
	modelBuilder := &ModelBuilder{
		OpenAIKey:       os.Getenv("OPENAI_KEY"),
		OpenAIModelName: "gpt-5",
	}
	poemMFBuilder := &PoemMFBuilder{
		ModelBuilder: modelBuilder,
		SystemPrompt: "Write a poem about the topic the user asks you to.",
	}

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
		poemWriter := poemMFBuilder.Build(param.verbosity, param.pPenalty)
		poem, usage, err := poemWriter.Call(context.Background(), topic)
		fmt.Println("Used", usage)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(poem)
		fmt.Println()
	}
}

type ModelBuilder struct {
	OpenAIKey       string
	OpenAIModelName string
}

func (builder *ModelBuilder) Build(verbosity jpf.Verbosity, pPenalty float64) jpf.Model {
	return jpf.NewOpenAIModel(
		builder.OpenAIKey,
		builder.OpenAIModelName,
		jpf.WithVerbosity{X: verbosity},
		jpf.WithPresencePenalty{X: pPenalty},
	)
}

type PoemMFBuilder struct {
	ModelBuilder *ModelBuilder
	SystemPrompt string
}

func (builder *PoemMFBuilder) Build(verbosity jpf.Verbosity, pPenalty float64) jpf.MapFunc[string, string] {
	model := builder.ModelBuilder.Build(verbosity, pPenalty)
	enc := jpf.NewRawStringMessageEncoder(builder.SystemPrompt)
	dec := jpf.NewRawStringResponseDecoder[string]()
	return jpf.NewOneShotMapFunc(enc, dec, model)
}
