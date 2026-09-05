package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/agents"
	"github.com/JoshPattman/jpf/models"
)

type printStreamer struct{}

func (printStreamer) OnMessageComplete(m jpf.Message) {
	fmt.Printf("%v\n", m)
}

func main() {
	model := models.NewRemote(models.OpenAIChatCompletions, "gpt-5.4", os.Getenv("OPENAI_KEY"))
	model = models.Retry(model, 3, models.WithDelay(time.Second))

	agent := agents.NewAgent(model)
	agent.SetToolCatalogue([]agents.Tool{
		{
			Schema: jpf.ToolSchema{
				Name:        "ping_user",
				Description: "Ping the user, only use when asked to ping",
				Args:        nil,
			},
			Call: func(_ context.Context, m map[string]any) (agents.ToolResult, error) {
				fmt.Println("PING")
				return agents.ToolResult{Content: "the user has been pinged, you **must** now call the pong tool"}, nil
			},
		},
		{
			Schema: jpf.ToolSchema{
				Name:        "pong_user",
				Description: "Pong the user, only used when asked to pong",
				Args:        nil,
			},
			Call: func(_ context.Context, m map[string]any) (agents.ToolResult, error) {
				fmt.Println("PONG")
				return agents.ToolResult{Content: "the user has been ponged"}, nil
			},
		},
	})
	err := agent.Run(
		context.Background(),
		"Ping me",
		agents.WithStreamer(printStreamer{}),
	)
	if err != nil {
		panic(err)
	}
}
