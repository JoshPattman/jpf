package jpf

import (
	"bytes"
	"context"
	"math"
	"os"
	"testing"
	"time"
)

func TestConstructOtherModels(t *testing.T) {
	model := NewOpenAIModel("abc", "123", WithHTTPHeader{K: "A", V: "B"}, WithReasoningEffort{X: HighReasoning}, WithTemperature{X: 0.5}, WithURL{X: "abc.com"})
	model = NewConcurrentLimitedModel(model, NewOneConcurrentLimiter())
	model = NewFakeReasoningModel(model, model, WithReasoningPrompt{X: "Reason please"})
	model = NewLoggingModel(model, NewJsonModelLogger(os.Stdout))
	model = NewRetryModel(model, 10, WithDelay{X: time.Second})
	NewRetryChainModel([]Model{model, model})
}

func TestCachedModel(t *testing.T) {
	var model Model = &TestingModel{Responses: map[string][]string{
		"hello": {"hi", "bye"},
	}}
	model = NewCachedModel(model, NewInMemoryCache())
	for i := range 5 {
		resp1, err := model.Respond(context.Background(), []Message{{Role: SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if resp1.PrimaryMessage.Content != "hi" {
			t.Fatalf("expected 'hi' but got '%v' on iteration %v", resp1.PrimaryMessage.Content, i)
		}
	}
}

func TestLoggingModel(t *testing.T) {
	var model Model = &TestingModel{Responses: map[string][]string{
		"hello": {"hi", "bye", "hi again"},
	}}
	buf := bytes.NewBuffer(nil)
	model = NewLoggingModel(model, NewJsonModelLogger(buf))
	for range 3 {
		_, err := model.Respond(context.Background(), []Message{{Role: SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	bs := buf.String()
	expected := `{"aux_responses":[],"duration":"420ns","final_response":{"content":"hi","num_images":0,"role":"assistant"},"messages":[{"content":"hello","num_images":0,"role":"system"}],"usage":{"input_tokens":0,"output_tokens":0}}
{"aux_responses":[],"duration":"80ns","final_response":{"content":"bye","num_images":0,"role":"assistant"},"messages":[{"content":"hello","num_images":0,"role":"system"}],"usage":{"input_tokens":0,"output_tokens":0}}
{"aux_responses":[],"duration":"80ns","final_response":{"content":"hi again","num_images":0,"role":"assistant"},"messages":[{"content":"hello","num_images":0,"role":"system"}],"usage":{"input_tokens":0,"output_tokens":0}}`
	// The times are logged so we cannot do a direct comparison
	if math.Abs(float64(len(bs)-len(expected))) > 9 {
		t.Fatalf("unexpected log: %v", bs)
	}
}

func TestRetryModel(t *testing.T) {
	var model Model = &TestingModel{
		Responses: map[string][]string{
			"hello": {"hi", "bye", "hi again"},
		},
		NFails: 3,
	}
	model = NewRetryModel(model, 3)
	resp, err := model.Respond(context.Background(), []Message{{Role: SystemRole, Content: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.PrimaryMessage.Content != "hi" {
		t.Fatalf("expected 'hi' but got '%v'", resp.PrimaryMessage.Content)
	}
}

func TestRetryChainModel(t *testing.T) {
	t.Run("first model succeeds", func(t *testing.T) {
		model := NewRetryChainModel([]Model{
			&TestingModel{
				Responses: map[string][]string{
					"hello": {"first response"},
				},
			},
			&TestingModel{
				Responses: map[string][]string{
					"hello": {"second response"},
				},
			},
		})
		resp, err := model.Respond(context.Background(), []Message{{Role: SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if resp.PrimaryMessage.Content != "first response" {
			t.Fatalf("expected 'first response' but got '%v'", resp.PrimaryMessage.Content)
		}
	})

	t.Run("first model fails, second succeeds", func(t *testing.T) {
		model := NewRetryChainModel([]Model{
			&TestingModel{
				Responses: map[string][]string{},
				NFails:    1,
			},
			&TestingModel{
				Responses: map[string][]string{
					"hello": {"second response"},
				},
			},
		})
		resp, err := model.Respond(context.Background(), []Message{{Role: SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if resp.PrimaryMessage.Content != "second response" {
			t.Fatalf("expected 'second response' but got '%v'", resp.PrimaryMessage.Content)
		}
	})

	t.Run("all models fail", func(t *testing.T) {
		model := NewRetryChainModel([]Model{
			&TestingModel{
				Responses: map[string][]string{},
				NFails:    1,
			},
			&TestingModel{
				Responses: map[string][]string{},
				NFails:    1,
			},
			&TestingModel{
				Responses: map[string][]string{},
				NFails:    1,
			},
		})
		_, err := model.Respond(context.Background(), []Message{{Role: SystemRole, Content: "hello"}})
		if err == nil {
			t.Fatal("expected error but got none")
		}
		// Check that error mentions all models failed
		errMsg := err.Error()
		if !contains(errMsg, "all 3 models") {
			t.Fatalf("expected error to mention all 3 models, got: %v", errMsg)
		}
	})

	t.Run("third model succeeds", func(t *testing.T) {
		model := NewRetryChainModel([]Model{
			&TestingModel{NFails: 1},
			&TestingModel{NFails: 1},
			&TestingModel{
				Responses: map[string][]string{
					"hello": {"third response"},
				},
			},
		})
		resp, err := model.Respond(context.Background(), []Message{{Role: SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if resp.PrimaryMessage.Content != "third response" {
			t.Fatalf("expected 'third response' but got '%v'", resp.PrimaryMessage.Content)
		}
	})
}

func TestTimeoutModel(t *testing.T) {
	t.Run("timeout triggers on slow model", func(t *testing.T) {
		// Create a slow model that takes 200ms
		slowModel := &SlowTestingModel{
			Delay: 200 * time.Millisecond,
			Response: ModelResponse{
				PrimaryMessage: Message{Role: AssistantRole, Content: "response"},
			},
		}

		// Wrap with 50ms timeout
		model := NewTimeoutModel(slowModel, 50*time.Millisecond)

		start := time.Now()
		_, err := model.Respond(context.Background(), []Message{{Role: SystemRole, Content: "hello"}})
		elapsed := time.Since(start)

		// Should fail with context deadline exceeded
		if err == nil {
			t.Fatal("expected timeout error but got none")
		}
		if !contains(err.Error(), "context deadline exceeded") {
			t.Fatalf("expected 'context deadline exceeded' error, got: %v", err)
		}

		// Should have timed out around 50ms, not 200ms
		if elapsed > 100*time.Millisecond {
			t.Fatalf("timeout took too long: %v (expected ~50ms)", elapsed)
		}
	})

	t.Run("succeeds when operation is fast enough", func(t *testing.T) {
		// Create a fast model that takes 10ms
		fastModel := &SlowTestingModel{
			Delay: 10 * time.Millisecond,
			Response: ModelResponse{
				PrimaryMessage: Message{Role: AssistantRole, Content: "fast response"},
			},
		}

		// Wrap with 100ms timeout (plenty of time)
		model := NewTimeoutModel(fastModel, 100*time.Millisecond)

		resp, err := model.Respond(context.Background(), []Message{{Role: SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.PrimaryMessage.Content != "fast response" {
			t.Fatalf("expected 'fast response' but got '%v'", resp.PrimaryMessage.Content)
		}
	})

	t.Run("parent context timeout takes precedence when shorter", func(t *testing.T) {
		// Create a slow model
		slowModel := &SlowTestingModel{
			Delay: 200 * time.Millisecond,
			Response: ModelResponse{
				PrimaryMessage: Message{Role: AssistantRole, Content: "response"},
			},
		}

		// Model configured with 100ms timeout
		model := NewTimeoutModel(slowModel, 100*time.Millisecond)

		// But parent context has 30ms timeout (shorter)
		parentCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := model.Respond(parentCtx, []Message{{Role: SystemRole, Content: "hello"}})
		elapsed := time.Since(start)

		// Should fail with context deadline exceeded
		if err == nil {
			t.Fatal("expected timeout error but got none")
		}

		// Should have timed out around 30ms (parent timeout), not 100ms
		if elapsed > 60*time.Millisecond {
			t.Fatalf("timeout took too long: %v (expected ~30ms from parent)", elapsed)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
