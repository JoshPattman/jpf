//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/models"
)

func TestHelloModels(t *testing.T) {
	oaiKey := os.Getenv("OPENAI_KEY")
	gemKey := os.Getenv("GEMINI_KEY")
	modelsToRun := []jpf.Model{
		models.NewAPIModel(models.OpenAI, "gpt-4.1", oaiKey),
		models.NewAPIModel(models.OpenAI, "gpt-4.1", oaiKey, models.WithTemperature(0)),
		models.NewAPIModel(models.OpenAI, "gpt-4.1", oaiKey, models.WithPresencePenalty(1)),
		models.NewAPIModel(models.Google, "gemini-2.5-flash", gemKey),
		models.NewAPIModel(models.Google, "gemini-2.5-flash", gemKey, models.WithTemperature(0)),
		models.NewAPIModel(models.OpenAI, "o3-mini", oaiKey),
		models.NewAPIModel(models.OpenAI, "o3-mini", oaiKey, models.WithReasoningEffort(models.MediumReasoning)),
		models.NewAPIModel(models.OpenAI, "gpt-5", oaiKey),
		models.NewAPIModel(models.OpenAI, "gpt-5", oaiKey, models.WithVerbosity(models.MediumVerbosity)),
	}
	for _, model := range modelsToRun {
		testHelloModel(t, models.Timeout(model, time.Minute))
	}
}

func testHelloModel(t *testing.T, model jpf.Model) {
	resp, err := model.Respond(
		context.Background(),
		[]jpf.Message{
			{
				Role:    jpf.UserRole,
				Content: "Hello there!",
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.PrimaryMessage.Content) == 0 {
		t.Fatal("primary message was empty")
	}
	t.Log(resp.PrimaryMessage.Content)
}
