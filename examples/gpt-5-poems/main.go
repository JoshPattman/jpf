package main

import (
	"context"
	"fmt"
	"os"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/encoders"
	"github.com/JoshPattman/jpf/models"
	"github.com/JoshPattman/jpf/parsers"
	"github.com/JoshPattman/jpf/pipelines"
)

func main() {
	modelBuilder := &ModelBuilder{
		OpenAIKey:       os.Getenv("OPENAI_KEY"),
		OpenAIModelName: "gpt-5",
	}
	pipelineBuilder := &PoemPipelineBuilder{
		ModelBuilder: modelBuilder,
		SystemPrompt: "Write a poem about the topic the user asks you to.",
	}

	topic := "Dogs"
	params := []struct {
		verbosity models.Verbosity
		pPenalty  float64
	}{
		{models.LowVerbosity, 0},
		{models.MediumVerbosity, 0},
		{models.HighVerbosity, 0},
	}

	for _, param := range params {
		fmt.Println("Verbosity", param.verbosity, "PPenalty", param.pPenalty)
		pipeline := pipelineBuilder.Build(param.verbosity, param.pPenalty)
		poem, usage, err := pipeline.Call(context.Background(), topic)
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

func (builder *ModelBuilder) Build(verbosity models.Verbosity, pPenalty float64) jpf.Model {
	return models.NewAPIModel(
		models.OpenAI,
		builder.OpenAIKey,
		builder.OpenAIModelName,
		models.WithVerbosity(verbosity),
		models.WithPresencePenalty(pPenalty),
	)
}

type PoemPipelineBuilder struct {
	ModelBuilder *ModelBuilder
	SystemPrompt string
}

func (builder *PoemPipelineBuilder) Build(verbosity models.Verbosity, pPenalty float64) jpf.Pipeline[string, string] {
	model := builder.ModelBuilder.Build(verbosity, pPenalty)
	encoder := encoders.NewFixedEncoder(builder.SystemPrompt)
	parser := parsers.NewStringParser()
	return pipelines.NewOneShotPipeline(encoder, parser, nil, model)
}
