//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/models"
)

func TestHelloModels(t *testing.T) {
	oaiKey := os.Getenv("OPENAI_KEY")
	gemKey := os.Getenv("GEMINI_KEY")
	modelsToRun := []jpf.Model{
		models.NewRemote(models.OpenAI, "gpt-4.1", oaiKey),
		models.NewRemote(models.OpenAI, "gpt-4.1", oaiKey, models.WithTemperature(0)),
		models.NewRemote(models.OpenAI, "gpt-4.1", oaiKey, models.WithPresencePenalty(1)),
		models.NewRemote(models.Google, "gemini-2.5-flash", gemKey),
		models.NewRemote(models.Google, "gemini-2.5-flash", gemKey, models.WithTemperature(0)),
		models.NewRemote(models.OpenAI, "o3-mini", oaiKey),
		models.NewRemote(models.OpenAI, "o3-mini", oaiKey, models.WithReasoningEffort(models.MediumReasoning)),
		models.NewRemote(models.OpenAI, "gpt-5", oaiKey),
		models.NewRemote(models.OpenAI, "gpt-5", oaiKey, models.WithVerbosity(models.MediumVerbosity)),
	}
	for _, model := range modelsToRun {
		testHelloModel(t, models.Timeout(model, time.Minute))
	}
}

func testHelloModel(t *testing.T, model jpf.Model) {
	resp, err := model.Respond(
		context.Background(),
		[]jpf.Message{
			jpf.UserMessage{
				Content: "Hello there!",
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Message.Content) == 0 {
		t.Fatal("primary message was empty")
	}
	t.Log(resp.Message.Content)
}

func TestToolCallModels(t *testing.T) {
	oaiKey := os.Getenv("OPENAI_KEY")
	gemKey := os.Getenv("GEMINI_KEY")
	modelsToRun := []jpf.Model{
		models.NewRemote(models.OpenAI, "gpt-4.1", oaiKey),
		models.NewRemote(models.Google, "gemini-2.5-flash", gemKey),
		models.NewRemote(models.OpenAI, "o3-mini", oaiKey),
		models.NewRemote(models.OpenAI, "gpt-5", oaiKey),
	}
	for _, model := range modelsToRun {
		testToolCallModel(t, models.Timeout(model, time.Minute))
	}
}

func testToolCallModel(t *testing.T, model jpf.Model) {
	schemas := jpf.ToolSchema{
		Name:        "ping_user",
		Description: "ping the user, use only when asked",
		Args: []jpf.ToolArg{
			{
				Name:        "message",
				Description: "a nice message to ping the user with",
				Type:        jpf.ToolArgString,
				Required:    true,
			},
		},
	}
	msgs := []jpf.Message{
		jpf.SystemMessage{
			Content: "When calling tools, you **must** include a short natural language message explaining what you are doing. The ping tool will include a confirmation password. You **must** include that exact password in your final reasponse, as a regex will check for it.",
		},
		jpf.UserMessage{
			Content: "Ping me!",
		},
	}
	resp, err := model.Respond(context.Background(), msgs, jpf.WithToolSchemas(schemas))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Message.ToolCalls) == 0 {
		t.Fatal("no tools were called")
	}
	if resp.Message.ToolCalls[0].Tool != "ping_user" {
		t.Fatal("wrong tool was called")
	}
	if resp.Message.Content != "" {
		t.Log(resp.Message.Content)
	}
	t.Log("AI SENT YOU A PING:", resp.Message.ToolCalls[0].Args["message"])
	msgs = append(msgs, resp.Message)
	msgs = append(msgs, jpf.ToolResultMessage{
		CallID: resp.Message.ToolCalls[0].ID,
		Result: "Ping sent. Now please make sure to include the following conformation password in your response: 'noodles'",
	})
	resp, err = model.Respond(context.Background(), msgs, jpf.WithToolSchemas(schemas))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Message.Content, "noodles") {
		t.Fatal("response did not inclide confirmation")
	}
	t.Log(resp.Message.Content)
}

func TestStructuredOutputs(t *testing.T) {
	oaiKey := os.Getenv("OPENAI_KEY")
	gemKey := os.Getenv("GEMINI_KEY")
	modelsToRun := []jpf.Model{
		models.NewRemote(models.OpenAI, "gpt-4.1", oaiKey),
		models.NewRemote(models.Google, "gemini-2.5-flash", gemKey),
		models.NewRemote(models.OpenAI, "o3-mini", oaiKey),
		models.NewRemote(models.OpenAI, "gpt-5", oaiKey),
	}
	for _, model := range modelsToRun {
		testStructuredOutput(t, models.Timeout(model, time.Minute))
	}
}

type helloResponse struct {
	Sentiment string `json:"sentiment"`
	Response  string `json:"response"`
}

func testStructuredOutput(t *testing.T, model jpf.Model) {
	resp, err := model.Respond(
		context.Background(),
		[]jpf.Message{
			jpf.UserMessage{
				Content: "Hello there!",
			},
		}, jpf.WithOutputFormat(helloResponse{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Message.Content) == 0 {
		t.Fatal("primary message was empty")
	}
	t.Log(resp.Message.Content)
}
