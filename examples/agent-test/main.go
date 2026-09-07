package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/agents"
	"github.com/JoshPattman/jpf/models"
	"github.com/JoshPattman/jpf/tools"
)

type printStreamer struct{}

func (printStreamer) OnMessageComplete(m jpf.Message) {
	fmt.Printf("%v\n", m)
}

func main() {
	model := models.NewRemote(models.OpenAIChatCompletions, "gpt-5.4", os.Getenv("OPENAI_KEY"))
	model = models.Retry(model, 3, models.WithDelay(time.Second))

	agent := agents.NewReAct(model)
	agent.SetToolCatalogue([]jpf.Tool{
		{
			Schema: jpf.ToolSchema{
				Name:        "ping_user",
				Description: "Ping the user, only use when asked to ping",
				Params:      nil,
			},
			Call: func(_ context.Context, m jpf.ToolArgs) (jpf.ToolResult, error) {
				fmt.Println("PING")
				return jpf.ToolResult{Content: "the user has been pinged, you **must** now call the pong tool"}, nil
			},
		},
		{
			Schema: jpf.ToolSchema{
				Name:        "pong_user",
				Description: "Pong the user, only used when asked to pong",
				Params:      nil,
			},
			Call: func(_ context.Context, m jpf.ToolArgs) (jpf.ToolResult, error) {
				fmt.Println("PONG")
				return jpf.ToolResult{Content: "the user has been ponged"}, nil
			},
		},
	})
	err := agent.Run(
		context.Background(),
		"Ping me",
		jpf.WithStreamActions(printStreamer{}),
	)
	if err != nil {
		panic(err)
	}

	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	agent.SetToolCatalogue([]jpf.Tool{
		tools.NewFileReadTool(root, 10000),
		tools.NewPWDTool(root),
		tools.NewDirReadTool(root, 100),
		tools.NewFileCreateTool(root),
		tools.NewFileModifyTool(root),
		tools.NewDirCreateTool(root),
	})
	err = agent.Run(
		context.Background(),
		"Ok now list the files in the dir you are currently in",
		jpf.WithStreamActions(printStreamer{}),
	)
	if err != nil {
		panic(err)
	}

	err = agent.Run(
		context.Background(),
		"Make a directory called scratch, then create an empty file called notes.txt inside it, then add a poem to that file. finally, try to tell me the directories that exist at ../",
		jpf.WithStreamActions(printStreamer{}),
	)
	if err != nil {
		panic(err)
	}

}
