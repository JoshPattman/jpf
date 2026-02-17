package main

import (
	"context"
	"fmt"
	"os"

	"github.com/JoshPattman/jpf/encoders"
	"github.com/JoshPattman/jpf/models"
	"github.com/JoshPattman/jpf/parsers"
	"github.com/JoshPattman/jpf/pipelines"
)

func main() {
	// Here we are defining a callback that happens on each stream.
	// This is intended to be used as a supplement to the final response, and should not really be used instead.
	// For example, if the cache is hit, there will be no stream, so the final response is still needed.
	onStream := func(text string) {
		fmt.Print(text)
	}
	// Create the model.
	// We can still use normal encoders, decoders, and retry logic (do be aware that retries will call the onBegin callback again).
	model := models.NewAPIModel(models.OpenAI, os.Getenv("OPENAI_KEY"), "gpt-4.1", models.WithStreamCallbacks(nil, onStream))
	// model := jpf.NewGeminiModel(os.Getenv("GEMINI_KEY"), "gemini-2.5-flash", jpf.WithStreamResponse{OnText: onStream})
	encoder := encoders.NewFixedEncoder("Write 5 haikus about the topic")
	parser := parsers.NewStringParser()
	pipeline := pipelines.NewOneShotPipeline(encoder, parser, nil, model)

	fmt.Println("===== Stream =====")
	// When we call the model, the callback will be called as the stream comes in.
	// In this case, we are ignoring the final response. However, we really should not do this!
	// In a production system, we should wipe any text that has been streamed and replace it with this final response,
	// because we cannot trust that the stream actually happened.
	_, usage, err := pipeline.Call(context.Background(), "Dogs")
	fmt.Println()
	fmt.Println()

	fmt.Println("===== Tokens =====")
	// The usage still works as normal, but it is only returned after the full response is complete.
	fmt.Println(usage.InputTokens, usage.OutputTokens)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	fmt.Println()
}
