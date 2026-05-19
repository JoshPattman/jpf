package main

import (
	"context"
	"fmt"
	"os"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/models"
)

func main() {
	// Create the model.
	// We can still use normal encoders, decoders, and retry logic (do be aware that retries will call the onBegin callback again).
	model := models.NewRemote(models.OpenAI, "gpt-4.1", os.Getenv("OPENAI_KEY"))
	//model := models.NewAPIModel(models.Google, "gemini-2.5-flash", os.Getenv("GEMINI_KEY"), models.WithStreamCallbacks(nil, onStream))

	fmt.Println("===== Stream =====")
	// We will use the model directly instead of going through a pipeline.
	// Pipelines are intended to be used for reliable data transformation, not realtime responses - this is due to their potential to retry on invalid messages etc.
	// For this reason, it does not make sense for a pipeline to stream.
	response, err := model.Respond(context.Background(), []jpf.Message{
		{
			Role:    jpf.SystemRole,
			Content: "Write 5 haikus about the topic",
		},
		{
			Role:    jpf.UserRole,
			Content: "Dogs",
		},
	}, jpf.WithStreamResponse(&printStreamer{}))
	fmt.Println()
	fmt.Println()

	fmt.Println("===== Tokens =====")
	// The usage still works as normal, but it is only returned after the full response is complete.
	fmt.Println(response.Usage.InputTokens, response.Usage.OutputTokens)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	fmt.Println()
}

type printStreamer struct{}

func (*printStreamer) OnMessageBegin() {}
func (*printStreamer) OnMessageText(text string) {
	fmt.Print(text)
}
func (*printStreamer) OnMessageReset() {}
