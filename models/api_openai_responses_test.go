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

func TestOpenAIResponsesInput(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	items, err := m.input([]jpf.Message{
		jpf.UserMessage{Content: "hi"},
		jpf.SystemMessage{Content: "system"},
		jpf.DeveloperMessage{Content: "be nice"},
		jpf.AssistantMessage{Content: "hello", ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "search", Args: map[string]any{"q": "cats"}}}},
		jpf.ToolResultMessage{CallID: "c1", Result: "done"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 6 {
		t.Fatalf("got %d items: %+v", len(items), items)
	}
	user := items[0].(map[string]any)
	if user["role"] != "user" || user["content"] != "hi" {
		t.Fatalf("got %+v", user)
	}
	system := items[1].(map[string]any)
	if system["role"] != "system" {
		t.Fatalf("got %+v", system)
	}
	dev := items[2].(map[string]any)
	if dev["role"] != "developer" {
		t.Fatalf("got %+v", dev)
	}
	assistant := items[3].(map[string]any)
	if assistant["role"] != "assistant" || assistant["content"] != "hello" {
		t.Fatalf("got %+v", assistant)
	}
	call := items[4].(map[string]any)
	if call["type"] != "function_call" || call["name"] != "search" || call["call_id"] != "c1" {
		t.Fatalf("got %+v", call)
	}
	result := items[5].(map[string]any)
	if result["type"] != "function_call_output" || result["call_id"] != "c1" {
		t.Fatalf("got %+v", result)
	}
}

func TestOpenAIResponsesInputToolResult(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	items, err := m.input([]jpf.Message{jpf.ToolResultMessage{CallID: "c1", Result: "done"}})
	if err != nil {
		t.Fatal(err)
	}
	item := items[0].(map[string]any)
	if item["type"] != "function_call_output" || item["call_id"] != "c1" || item["output"] != "done" {
		t.Fatalf("got %+v", item)
	}
}

func TestOpenAIResponsesInputAssistantWithoutContentOmitsMessage(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	items, err := m.input([]jpf.Message{jpf.AssistantMessage{ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "search"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected only the function call item, got %+v", items)
	}
	if items[0].(map[string]any)["type"] != "function_call" {
		t.Fatalf("got %+v", items[0])
	}
}

func TestOpenAIResponsesUserContentWithImages(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	content := m.userContent(jpf.UserMessage{Content: "look", Images: []jpf.ImageAttachment{{Source: img}}})
	parts, ok := content.([]map[string]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("got %+v", content)
	}
	if parts[0]["type"] != "input_text" || parts[0]["text"] != "look" {
		t.Fatalf("got %+v", parts[0])
	}
	if parts[1]["type"] != "input_image" {
		t.Fatalf("got %+v", parts[1])
	}
}

func TestOpenAIResponsesBody(t *testing.T) {
	temp := 0.5
	reasoning := HighReasoning
	topP := 5
	maxOut := 100
	verbosity := HighVerbosity
	m := &apiOpenAIResponsesModel{
		name: "gpt-test",
		settings: apiModelSettings{
			temperature: &temp,
			reasoning:   &reasoning,
			topP:        &topP,
			maxOutput:   &maxOut,
			verbosity:   &verbosity,
		},
	}
	body, err := m.body(nil, true, struct{ A int }{}, []jpf.ToolSchema{{Name: "search"}})
	if err != nil {
		t.Fatal(err)
	}
	if body["model"] != "gpt-test" || body["store"] != false {
		t.Fatalf("got %+v", body)
	}
	if body["temperature"] != temp || body["top_p"] != topP || body["max_output_tokens"] != maxOut {
		t.Fatalf("got %+v", body)
	}
	reasoningMap := body["reasoning"].(map[string]any)
	if reasoningMap["effort"] != "high" {
		t.Fatalf("got %+v", reasoningMap)
	}
	text := body["text"].(map[string]any)
	if text["verbosity"] != "high" {
		t.Fatalf("got %+v", text)
	}
	if _, ok := text["format"]; !ok {
		t.Fatal("expected a format to be set")
	}
	if body["stream"] != true {
		t.Fatal("expected stream true")
	}
	if _, ok := body["tools"]; !ok {
		t.Fatal("expected tools to be set")
	}
}

func TestOpenAIResponsesBodyMinimal(t *testing.T) {
	m := &apiOpenAIResponsesModel{name: "gpt-test"}
	body, err := m.body(nil, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"temperature", "reasoning", "top_p", "max_output_tokens", "text", "stream", "tools", "tool_choice"} {
		if _, ok := body[key]; ok {
			t.Fatalf("did not expect %q to be set, got %+v", key, body[key])
		}
	}
}

func TestOpenAIResponsesTools(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	tools := m.tools([]jpf.ToolSchema{
		{
			Name: "search",
			Args: []jpf.ToolArg{
				{Name: "q", Type: jpf.ToolArgString, Required: true},
			},
		},
	})
	if len(tools) != 1 {
		t.Fatalf("got %d tools", len(tools))
	}
	tool := tools[0].(map[string]any)
	if tool["type"] != "function" || tool["name"] != "search" {
		t.Fatalf("got %+v", tool)
	}
}

func TestOpenAIResponsesValidateNoUnusableArgs(t *testing.T) {
	if err := (&apiOpenAIResponsesModel{}).validateNoUnusableArgs(jpf.ModelResponseKwargs{}); err != nil {
		t.Fatal(err)
	}

	pp := 0.5
	if err := (&apiOpenAIResponsesModel{settings: apiModelSettings{presencePenalty: &pp}}).validateNoUnusableArgs(jpf.ModelResponseKwargs{}); err == nil {
		t.Fatal("expected an error for an unsupported presence penalty setting")
	}

	pred := "pred"
	if err := (&apiOpenAIResponsesModel{settings: apiModelSettings{prediction: &pred}}).validateNoUnusableArgs(jpf.ModelResponseKwargs{}); err == nil {
		t.Fatal("expected an error for an unsupported prediction setting")
	}
}

func TestOpenAIResponsesSchema(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	type out struct {
		A int `json:"a"`
	}
	schema, err := m.schema(out{})
	if err != nil {
		t.Fatal(err)
	}
	wrapped := schema.(map[string]any)
	if wrapped["type"] != "json_schema" || wrapped["name"] != "custom_schema" || wrapped["strict"] != true {
		t.Fatalf("got %+v", wrapped)
	}
}

func TestOpenAIResponsesCreateRequest(t *testing.T) {
	m := &apiOpenAIResponsesModel{
		key: "secret",
		settings: apiModelSettings{
			url:     "http://example.com/v1/responses",
			headers: map[string]string{"X-Custom": "1"},
		},
	}
	req, err := m.createRequest(context.Background(), strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != "http://example.com/v1/responses" {
		t.Fatalf("got %s", req.URL.String())
	}
	if req.Header.Get("Authorization") != "Bearer secret" {
		t.Fatalf("got %s", req.Header.Get("Authorization"))
	}
	if req.Header.Get("X-Custom") != "1" {
		t.Fatalf("got %s", req.Header.Get("X-Custom"))
	}
}

func TestOpenAIResponsesCreateBodyData(t *testing.T) {
	m := &apiOpenAIResponsesModel{name: "gpt-test"}
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
}

func TestOpenAIResponsesApiErrorResponse(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	resp := &http.Response{
		StatusCode: 400,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad request","type":"invalid_request_error","code":"400"}}`)),
	}
	_, err := m.apiErrorResponse(resp)
	var apiErr *openAIError
	if !errors.As(err, &apiErr) || apiErr.msg != "bad request" {
		t.Fatalf("got %v", err)
	}
}

func TestOpenAIResponsesApiErrorResponseMalformedBody(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader("not json")),
	}
	_, err := m.apiErrorResponse(resp)
	if err == nil || !contains(err.Error(), "500") {
		t.Fatalf("got %v", err)
	}
}

func TestOpenAIResponsesParseStaticResponse(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	body := `{"output":[{"type":"message","content":[{"type":"output_text","text":"hi"}]}],"usage":{"input_tokens":1,"output_tokens":2}}`
	resp, raw, err := m.parseStaticResponse(context.Background(), io.NopCloser(strings.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Output) != 1 || resp.Output[0].Content[0].Text != "hi" {
		t.Fatalf("got %+v", resp)
	}
	if resp.Usage.InputTokens != 1 || resp.Usage.OutputTokens != 2 {
		t.Fatalf("got %+v", resp.Usage)
	}
	if string(raw) != body {
		t.Fatalf("got %s", raw)
	}
}

func TestOpenAIResponsesParseStaticResponseInvalidJSON(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	_, _, err := m.parseStaticResponse(context.Background(), io.NopCloser(strings.NewReader("not json")))
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestOpenAIResponsesExtractOutput(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	content, toolCalls, err := m.extractOutput([]openAIResponsesOutputItem{
		{Type: "message", Content: []openAIResponsesContentPart{{Type: "output_text", Text: "hi "}, {Type: "refusal", Refusal: "no"}}},
		{Type: "function_call", CallID: "c1", Name: "search", Arguments: `{"q":"cats"}`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if content != "hi no" {
		t.Fatalf("got %q", content)
	}
	if len(toolCalls) != 1 || toolCalls[0].ID != "c1" || toolCalls[0].Tool != "search" || toolCalls[0].Args["q"] != "cats" {
		t.Fatalf("got %+v", toolCalls)
	}
}

func TestOpenAIResponsesExtractOutputInvalidArguments(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	_, _, err := m.extractOutput([]openAIResponsesOutputItem{
		{Type: "function_call", Name: "search", Arguments: "not json"},
	})
	if err == nil {
		t.Fatal("expected an error for invalid tool arguments")
	}
}

func TestOpenAIResponsesParseStreamResponse(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	final := openAIResponsesStaticResponse{Output: []openAIResponsesOutputItem{{Type: "message", Content: []openAIResponsesContentPart{{Type: "output_text", Text: "hi"}}}}}
	finalData, err := json.Marshal(final)
	if err != nil {
		t.Fatal(err)
	}
	stream := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"hi"}`,
		`data: {"type":"response.completed","response":` + string(finalData) + `}`,
	}, "\n")
	streamer := &recordingStreamer{}
	resp, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader(stream)), streamer)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Output[0].Content[0].Text != "hi" {
		t.Fatalf("got %+v", resp)
	}
	if !streamer.began || streamer.text != "hi" {
		t.Fatalf("got began=%v text=%q", streamer.began, streamer.text)
	}
}

func TestOpenAIResponsesParseStreamResponseErrorEvent(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	stream := `data: {"type":"error","message":"boom","code":"500"}`
	_, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader(stream)), &recordingStreamer{})
	var apiErr *openAIError
	if !errors.As(err, &apiErr) || apiErr.msg != "boom" {
		t.Fatalf("got %v", err)
	}
}

func TestOpenAIResponsesParseStreamResponseNoCompletion(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	stream := `data: {"type":"response.output_text.delta","delta":"hi"}`
	_, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader(stream)), &recordingStreamer{})
	if err == nil {
		t.Fatal("expected an error when the stream ends without completing")
	}
}

func TestOpenAIResponsesParseStreamResponseInvalidEvent(t *testing.T) {
	m := &apiOpenAIResponsesModel{}
	_, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader("data: not json")), &recordingStreamer{})
	if err == nil {
		t.Fatal("expected an error")
	}
}
