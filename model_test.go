package jpf

import (
	"testing"
)

func TestCachedModel(t *testing.T) {
	var model ChatCaller = &TestingModel{Responses: map[string][]string{
		"hello": {"hi", "bye"},
	}}
	model = NewCachedChatCaller(model, NewInMemoryCache())
	for i := range 5 {
		res, err := model.Call([]Message{{Role: SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if res.Primary.Content != "hi" {
			t.Fatalf("expected 'hi' but got '%v' on iteration %v", res.Primary.Content, i)
		}
	}
}

/*

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
}*/

func TestRetryModel(t *testing.T) {
	var model ChatCaller = &TestingModel{
		Responses: map[string][]string{
			"hello": {"hi", "bye", "hi again"},
		},
		NFails: 3,
	}
	model = NewRetryCaller(model, 3, 0)
	res, err := model.Call([]Message{{Role: SystemRole, Content: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Primary.Content != "hi" {
		t.Fatalf("expected 'hi' but got '%v'", res.Primary.Content)
	}
}
