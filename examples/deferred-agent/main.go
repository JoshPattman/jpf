package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/agents"
	"github.com/JoshPattman/jpf/models"
)

func main() {
	agent := createEmptyAgent()
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
		// Now we have set off the tasks, its safe to save the agent to a json file
		file := bytes.NewBuffer(nil)
		sessionDTO := agents.AgentSessionDTO{}
		err = sessionDTO.LoadSession(agent.Session())
		if err != nil {
			panic(err)
		}
		err = json.NewEncoder(file).Encode(sessionDTO)
		if err != nil {
			panic(err)
		}

		wg.Wait()

		// This could be a completely different service - notice how only data that can be serialised is needed
		fmt.Println("Resuming agent")
		agent2 := createEmptyAgent()
		sessionDTO2 := agents.AgentSessionDTO{}
		err = json.NewDecoder(file).Decode(&sessionDTO2)
		if err != nil {
			panic(err)
		}
		session2, err := sessionDTO2.ToSession()
		if err != nil {
			panic(err)
		}
		agent2.SetSession(session2)
		if err := agent2.Resume(context.Background(), defResponses, messageCallback); err != nil {
			panic(err)
		}

		// Ensure the loop checks the updated agent state
		agent = agent2
	}
}

func createEmptyAgent() *agents.Agent {
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
	return agent
}
