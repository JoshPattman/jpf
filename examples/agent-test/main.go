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

func main() {
	model := models.NewRemote(models.OpenAI, "gpt-5.4", os.Getenv("OPENAI_KEY"))
	model = models.Retry(model, 3, models.WithDelay(time.Second))

	agent := agents.NewAgent(20, model)
	agent.SetTools([]agents.Tool{
		{
			Schema: jpf.ToolSchema{
				Name:        "ping_user",
				Description: "Ping the user, only use when asked to ping",
				Args:        nil,
			},
			Call: func(_ context.Context, m map[string]any) (string, error) {
				fmt.Println("PING")
				return "the user has been pinged, you **must** now call the pong tool", nil
			},
		},
		{
			Schema: jpf.ToolSchema{
				Name:        "pong_user",
				Description: "Pong the user, only used when asked to pong",
				Args:        nil,
			},
			Call: func(_ context.Context, m map[string]any) (string, error) {
				fmt.Println("PONG")
				return "the user has been ponged", nil
			},
		},
	})
	err := agent.Run(
		context.Background(),
		"Ping me",
		func(m jpf.Message) {
			fmt.Printf("%v\n", m)
		},
	)
	if err != nil {
		panic(err)
	}
}
