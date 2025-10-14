//go:build integration

package jpf

import (
	"os"
	"testing"
)

func TestHelloModels(t *testing.T) {
	oaiKey := os.Getenv("OPENAI_KEY")
	gemKey := os.Getenv("GEMINI_KEY")
	models := []Model{
		NewOpenAIModel(oaiKey, "gpt-4.1"),
		NewOpenAIModel(oaiKey, "gpt-4.1", WithTemperature{0}),
		NewOpenAIModel(oaiKey, "gpt-4.1", WithPresencePenalty{1}),
		NewGeminiModel(gemKey, "gemini-2.5-flash"),
		NewGeminiModel(gemKey, "gemini-2.5-flash", WithTemperature{0}),
		NewOpenAIModel(oaiKey, "o3-mini"),
		NewOpenAIModel(oaiKey, "o3-mini", WithReasoningEffort{LowReasoning}),
		NewOpenAIModel(oaiKey, "gpt-5"),
		NewOpenAIModel(oaiKey, "gpt-5", WithVerbosity{HighVerbosity}),
	}
	for _, model := range models {
		testHelloModel(t, model)
	}
}

func testHelloModel(t *testing.T, model Model) {
	resp, err := model.Respond([]Message{
		{
			Role:    UserRole,
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
