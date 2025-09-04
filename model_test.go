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
	model = NewRetryModel(model, WithDelay{X: time.Second}, WithRetries{X: 10})
	NewSystemReasonModel(model, WithReasoningPrefix{X: "Resoning: "})
}

func TestCachedModel(t *testing.T) {
	var model Model = &TestingModel{Responses: map[string][]string{
		"hello": {"hi", "bye"},
	}}
	model = NewCachedModel(model, NewInMemoryCache())
	for i := range 5 {
		_, r1, _, err := model.Respond([]Message{{Role: SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if r1.Content != "hi" {
			t.Fatalf("expected 'hi' but got '%v' on iteration %v", r1.Content, i)
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
		_, _, _, err := model.Respond([]Message{{Role: SystemRole, Content: "hello"}})
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
