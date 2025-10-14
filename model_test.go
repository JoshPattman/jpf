package jpf

import (
	"bytes"
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
		resp1, err := model.Respond([]Message{{Role: SystemRole, Content: "hello"}})
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
		_, err := model.Respond([]Message{{Role: SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	bs := buf.String()
	expected := `{"aux_responses":[],"duration":"420ns","final_response":{"content":"hi","num_images":0,"role":"assistant"},"messages":[{"content":"hello","num_images":0,"role":"system"}],"usage":{"input_tokens":0,"output_tokens":0}}
{"aux_responses":[],"duration":"80ns","final_response":{"content":"bye","num_images":0,"role":"assistant"},"messages":[{"content":"hello","num_images":0,"role":"system"}],"usage":{"input_tokens":0,"output_tokens":0}}
{"aux_responses":[],"duration":"80ns","final_response":{"content":"hi again","num_images":0,"role":"assistant"},"messages":[{"content":"hello","num_images":0,"role":"system"}],"usage":{"input_tokens":0,"output_tokens":0}}`
	// The times are logged so we cannot do a direct comparison
	if math.Abs(float64(len(bs)-len(expected))) > 6 {
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
	resp, err := model.Respond([]Message{{Role: SystemRole, Content: "hello"}})
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
		resp, err := model.Respond([]Message{{Role: SystemRole, Content: "hello"}})
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
		resp, err := model.Respond([]Message{{Role: SystemRole, Content: "hello"}})
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
		_, err := model.Respond([]Message{{Role: SystemRole, Content: "hello"}})
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
		resp, err := model.Respond([]Message{{Role: SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if resp.PrimaryMessage.Content != "third response" {
			t.Fatalf("expected 'third response' but got '%v'", resp.PrimaryMessage.Content)
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
