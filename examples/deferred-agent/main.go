package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/agents"
	"github.com/JoshPattman/jpf/models"
)

func main() {
	model := models.NewRemote(models.OpenAIChatCompletions, "gpt-5.4", os.Getenv("OPENAI_KEY"))
	model = models.Retry(model, 3, models.WithDelay(time.Second))

	agent := agents.NewAgent(model)
	agent.SetToolCatalogue([]agents.Tool{
		{
			Schema: jpf.ToolSchema{
				Name:        "ping_user",
				Description: "Ping the user, only use when asked to ping. When they are ready, they will pong you back.",
				Args:        nil,
			},
			Call: nil, // To make a call deferred, simply do not include an auto-run function.
		},
	})
	messageCallback := func(m jpf.Message) {
		fmt.Printf("%v\n", m)
	}
	err := agent.Run(
		context.Background(),
		"Ping me",
		messageCallback,
	)
	if err != nil {
		panic(err)
	}
	fmt.Println("Agent execution stopped")
	// Whenever you are working with any deferred tools,
	// you will need some kind of loop or listener like this that can
	// (a) drain the current deferred calls
	// (b) resume the agent when all calls are complete.
	// However, these may not have to be both part of the same thing,
	// e.g. there could be one loop that dispatches tasks, and another
	// in paralell that checks if they are done, and if they are it resumes
	// the agent. This very much needs to be tailored to your usecase.
	for len(agent.CurrentDeferredToolCalls()) > 0 {
		defCalls := agent.CurrentDeferredToolCalls()
		// Note that although we keep them ordered here, deferred responses do not need to be ordered.
		defResponses := make([]agents.DeferredCallResponse, len(defCalls))
		wg := &sync.WaitGroup{}
		for i, defCall := range defCalls {
			switch defCall.ToolName {
			case "ping_user":
				wg.Go(func() {
					fmt.Println("Ping requested")
					time.Sleep(time.Second * 5)
					fmt.Println("Sending pong")
					defResponses[i] = agents.DeferredCallResponse{CallID: defCall.CallID, Result: "User has sent PONG, let the user know you got it"}
				})
			default:
				panic("not possible")
			}
		}
		wg.Wait()
		fmt.Println("Resuming agent")
		agent.Resume(context.Background(), defResponses, messageCallback)
	}
}
