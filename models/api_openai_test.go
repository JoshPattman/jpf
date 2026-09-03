package models

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/JoshPattman/jpf"
)

type recordingStreamer struct {
	began bool
	text  string
	reset bool
}

func (r *recordingStreamer) OnMessageBegin()        { r.began = true }
func (r *recordingStreamer) OnMessageText(s string) { r.text += s }
func (r *recordingStreamer) OnMessageReset()        { r.reset = true }

func TestOpenAIMessages(t *testing.T) {
	m := &apiOpenAIModel{}
	msgs, err := m.messages([]jpf.Message{
		jpf.UserMessage{Content: "hi"},
		jpf.AssistantMessage{Content: "hello", ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "search", Args: map[string]any{"q": "cats"}}}},
		jpf.DeveloperMessage{Content: "be nice"},
		jpf.SystemMessage{Content: "system"},
		jpf.ToolResultMessage{CallID: "c1", Result: "done"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 5 {
		t.Fatalf("got %d messages", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hi" {
		t.Fatalf("got %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hello" || len(msgs[1].ToolCalls) != 1 {
		t.Fatalf("got %+v", msgs[1])
	}
	if msgs[1].ToolCalls[0].Function.Name != "search" {
		t.Fatalf("got %+v", msgs[1].ToolCalls[0])
	}
	if msgs[2].Role != "developer" || msgs[2].Content != "be nice" {
		t.Fatalf("got %+v", msgs[2])
	}
	if msgs[3].Role != "system" || msgs[3].Content != "system" {
		t.Fatalf("got %+v", msgs[3])
	}
	if msgs[4].Role != "tool" || msgs[4].Content != "done" || msgs[4].ToolCallID != "c1" {
		t.Fatalf("got %+v", msgs[4])
	}
}

func TestOpenAIMessageContentWithImages(t *testing.T) {
	m := &apiOpenAIModel{}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	content, err := m.messageContent(jpf.UserMessage{Content: "look", Images: []jpf.ImageAttachment{{Source: img}}})
	if err != nil {
		t.Fatal(err)
	}
	parts, ok := content.([]map[string]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("got %+v", content)
	}
	if parts[0]["type"] != "text" || parts[0]["text"] != "look" {
		t.Fatalf("got %+v", parts[0])
	}
	if parts[1]["type"] != "image_url" {
		t.Fatalf("got %+v", parts[1])
	}
}

func TestOpenAIToolCalls(t *testing.T) {
	m := &apiOpenAIModel{}
	calls, err := m.toolCalls(jpf.AssistantMessage{})
	if err != nil || calls != nil {
		t.Fatalf("expected nil calls for no tool calls, got %+v, %v", calls, err)
	}

	calls, err = m.toolCalls(jpf.AssistantMessage{ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "search", Args: map[string]any{"q": "cats"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].ID != "c1" || calls[0].Function.Name != "search" {
		t.Fatalf("got %+v", calls)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(calls[0].Function.Arguments), &args); err != nil {
		t.Fatal(err)
	}
	if args["q"] != "cats" {
		t.Fatalf("got %+v", args)
	}
}

func TestOpenAIBody(t *testing.T) {
	temp := 0.5
	reasoning := HighReasoning
	verbosity := HighVerbosity
	topP := 5
	presence := 0.2
	prediction := "pred"
	maxOut := 100
	m := &apiOpenAIModel{
		name: "gpt-test",
		settings: apiModelSettings{
			temperature:     &temp,
			reasoning:       &reasoning,
			verbosity:       &verbosity,
			topP:            &topP,
			presencePenalty: &presence,
			prediction:      &prediction,
			maxOutput:       &maxOut,
		},
	}
	body, err := m.body(nil, true, struct{ A int }{}, []jpf.ToolSchema{{Name: "search"}})
	if err != nil {
		t.Fatal(err)
	}
	if body["model"] != "gpt-test" {
		t.Fatalf("got %+v", body)
	}
	if body["temperature"] != temp {
		t.Fatalf("got %+v", body["temperature"])
	}
	if body["reasoning_effort"] != "high" {
		t.Fatalf("got %+v", body["reasoning_effort"])
	}
	if body["verbosity"] != "high" {
		t.Fatalf("got %+v", body["verbosity"])
	}
	if body["top_p"] != topP {
		t.Fatalf("got %+v", body["top_p"])
	}
	if body["presence_penalty"] != presence {
		t.Fatalf("got %+v", body["presence_penalty"])
	}
	if body["prediction"] != prediction {
		t.Fatalf("got %+v", body["prediction"])
	}
	if body["max_completion_tokens"] != m.settings.maxOutput {
		t.Fatalf("got %+v", body["max_completion_tokens"])
	}
	if _, ok := body["response_format"]; !ok {
		t.Fatal("expected response_format to be set")
	}
	if body["stream"] != true {
		t.Fatal("expected stream true")
	}
	if _, ok := body["tools"]; !ok {
		t.Fatal("expected tools to be set")
	}
	if body["tool_choice"] != "auto" {
		t.Fatalf("got %+v", body["tool_choice"])
	}
}

func TestOpenAIBodyMinimal(t *testing.T) {
	m := &apiOpenAIModel{name: "gpt-test"}
	body, err := m.body(nil, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"temperature", "reasoning_effort", "verbosity", "top_p", "presence_penalty", "prediction", "max_completion_tokens", "response_format", "stream", "tools", "tool_choice"} {
		if _, ok := body[key]; ok {
			t.Fatalf("did not expect %q to be set, got %+v", key, body[key])
		}
	}
}

func TestOpenAITools(t *testing.T) {
	m := &apiOpenAIModel{}
	tools := m.tools([]jpf.ToolSchema{
		{
			Name:        "search",
			Description: "search the web",
			Args: []jpf.ToolArg{
				{Name: "q", Type: jpf.ToolArgString, Required: true, Description: "query"},
				{Name: "n", Type: jpf.ToolArgInt},
				{Name: "score", Type: jpf.ToolArgFloat},
			},
		},
	})
	if len(tools) != 1 {
		t.Fatalf("got %d tools", len(tools))
	}
	tool := tools[0].(map[string]any)
	fn := tool["function"].(map[string]any)
	if fn["name"] != "search" {
		t.Fatalf("got %+v", fn)
	}
	params := fn["parameters"].(map[string]any)
	props := params["properties"].(map[string]any)
	if props["q"].(map[string]any)["type"] != "string" {
		t.Fatalf("got %+v", props["q"])
	}
	if props["n"].(map[string]any)["type"] != "integer" {
		t.Fatalf("got %+v", props["n"])
	}
	if props["score"].(map[string]any)["type"] != "number" {
		t.Fatalf("got %+v", props["score"])
	}
	required := params["required"].([]string)
	if len(required) != 1 || required[0] != "q" {
		t.Fatalf("got %+v", required)
	}
}

func TestOpenAIReasoningEffort(t *testing.T) {
	m := &apiOpenAIModel{}
	tests := map[ReasoningEffort]string{
		NoneReasoning:   "none",
		LowReasoning:    "low",
		MediumReasoning: "medium",
		HighReasoning:   "high",
		XHighReasoning:  "xhigh",
	}
	for re, want := range tests {
		if got := m.reasoningEffort(re); got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}

func TestOpenAIReasoningEffortPanicsOnUnknown(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic")
		}
	}()
	(&apiOpenAIModel{}).reasoningEffort(ReasoningEffort(255))
}

func TestOpenAIVerbosity(t *testing.T) {
	m := &apiOpenAIModel{}
	tests := map[Verbosity]string{
		LowVerbosity:    "low",
		MediumVerbosity: "medium",
		HighVerbosity:   "high",
	}
	for v, want := range tests {
		if got := m.verbosity(v); got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}

func TestOpenAIVerbosityPanicsOnUnknown(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic")
		}
	}()
	(&apiOpenAIModel{}).verbosity(Verbosity(255))
}

func TestOpenAIValidateNoUnusableArgs(t *testing.T) {
	m := &apiOpenAIModel{}
	if err := m.validateNoUnusableArgs(jpf.ModelResponseKwargs{}); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAISchema(t *testing.T) {
	m := &apiOpenAIModel{}
	type out struct {
		A int `json:"a"`
	}
	schema, err := m.schema(out{})
	if err != nil {
		t.Fatal(err)
	}
	wrapped := schema.(map[string]any)
	if wrapped["type"] != "json_schema" {
		t.Fatalf("got %+v", wrapped)
	}
	inner := wrapped["json_schema"].(map[string]any)
	if inner["name"] != "custom_schema" || inner["strict"] != true {
		t.Fatalf("got %+v", inner)
	}
}

func TestOpenAICreateRequest(t *testing.T) {
	m := &apiOpenAIModel{
		key: "secret",
		settings: apiModelSettings{
			url:     "http://example.com/v1/chat",
			headers: map[string]string{"X-Custom": "1"},
		},
	}
	req, err := m.createRequest(context.Background(), strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != "http://example.com/v1/chat" {
		t.Fatalf("got %s", req.URL.String())
	}
	if req.Header.Get("Authorization") != "Bearer secret" {
		t.Fatalf("got %s", req.Header.Get("Authorization"))
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("got %s", req.Header.Get("Content-Type"))
	}
	if req.Header.Get("X-Custom") != "1" {
		t.Fatalf("got %s", req.Header.Get("X-Custom"))
	}
}

func TestOpenAICreateBodyData(t *testing.T) {
	m := &apiOpenAIModel{name: "gpt-test"}
	r, err := m.createBodyData([]jpf.Message{jpf.UserMessage{Content: "hi"}}, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["model"] != "gpt-test" {
		t.Fatalf("got %+v", decoded)
	}
	msgs := decoded["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("got %+v", msgs)
	}
}

func TestOpenAIApiErrorResponse(t *testing.T) {
	m := &apiOpenAIModel{}
	resp := &http.Response{
		StatusCode: 400,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad request","type":"invalid_request_error","code":"400"}}`)),
	}
	_, err := m.apiErrorResponse(resp)
	var apiErr *openAIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an *openAIError, got %v (%T)", err, err)
	}
	if apiErr.msg != "bad request" {
		t.Fatalf("got %+v", apiErr)
	}
}

func TestOpenAIApiErrorResponseMalformedBody(t *testing.T) {
	m := &apiOpenAIModel{}
	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader("not json")),
	}
	_, err := m.apiErrorResponse(resp)
	if err == nil || !contains(err.Error(), "500") {
		t.Fatalf("got %v", err)
	}
}

func TestOpenAIParseStaticResponse(t *testing.T) {
	m := &apiOpenAIModel{}
	body := `{"choices":[{"message":{"content":"hi"}}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`
	resp, raw, err := m.parseStaticResponse(context.Background(), io.NopCloser(strings.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "hi" {
		t.Fatalf("got %+v", resp)
	}
	if resp.Usage.InputTokens != 1 || resp.Usage.OutputTokens != 2 {
		t.Fatalf("got %+v", resp.Usage)
	}
	if string(raw) != body {
		t.Fatalf("got %s", raw)
	}
}

func TestOpenAIParseStaticResponseInvalidJSON(t *testing.T) {
	m := &apiOpenAIModel{}
	_, _, err := m.parseStaticResponse(context.Background(), io.NopCloser(strings.NewReader("not json")))
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestOpenAIParseStreamResponse(t *testing.T) {
	m := &apiOpenAIModel{}
	stream := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"he"}}]}`,
		`data: {"choices":[{"delta":{"content":"llo"}}]}`,
		`data: {"usage":{"prompt_tokens":3,"completion_tokens":4}}`,
		`data: [DONE]`,
	}, "\n")
	streamer := &recordingStreamer{}
	resp, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader(stream)), streamer)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].Message.Content != "hello" {
		t.Fatalf("got %+v", resp.Choices[0].Message)
	}
	if resp.Usage.InputTokens != 3 || resp.Usage.OutputTokens != 4 {
		t.Fatalf("got %+v", resp.Usage)
	}
	if !streamer.began {
		t.Fatal("expected OnMessageBegin to be called")
	}
	if streamer.text != "hello" {
		t.Fatalf("got %q", streamer.text)
	}
}

func TestOpenAIParseStreamResponseWithToolCalls(t *testing.T) {
	m := &apiOpenAIModel{}
	stream := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"search","arguments":"{\"q\":"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"cats\"}"}}]}}]}`,
		`data: [DONE]`,
	}, "\n")
	resp, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader(stream)), &recordingStreamer{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("got %+v", resp.Choices[0].Message.ToolCalls)
	}
	tc := resp.Choices[0].Message.ToolCalls[0]
	if tc.ID != "c1" || tc.Function.Name != "search" || tc.Function.Arguments != `{"q":"cats"}` {
		t.Fatalf("got %+v", tc)
	}
}

func TestOpenAIParseStreamResponseErrorChunk(t *testing.T) {
	m := &apiOpenAIModel{}
	stream := `data: {"error":{"message":"boom","type":"server_error","code":"500"}}`
	_, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader(stream)), &recordingStreamer{})
	var apiErr *openAIError
	if !errors.As(err, &apiErr) || apiErr.msg != "boom" {
		t.Fatalf("got %v", err)
	}
}

func TestOpenAIParseStreamResponseInvalidChunk(t *testing.T) {
	m := &apiOpenAIModel{}
	stream := "data: not json"
	_, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader(stream)), &recordingStreamer{})
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestOpenAIErrorFormat(t *testing.T) {
	err := &openAIError{msg: "bad", errType: "invalid_request_error", code: "400"}
	want := "openai api returned an error: invalid_request_error.400 - bad"
	if err.Error() != want {
		t.Fatalf("got %q want %q", err.Error(), want)
	}
}
