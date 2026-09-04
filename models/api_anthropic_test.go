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

func TestAnthropicMessages(t *testing.T) {
	m := &apiAnthropicModel{}
	system, msgs, err := m.messages([]jpf.Message{
		jpf.SystemMessage{Content: "be nice"},
		jpf.UserMessage{Content: "hi"},
		jpf.DeveloperMessage{Content: "and terse"},
		jpf.AssistantMessage{Content: "hello", ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "search", Args: map[string]any{"q": "cats"}}}},
		jpf.ToolResultMessage{CallID: "c1", Result: "done"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if system != "be nice\n\nand terse" {
		t.Fatalf("got %q", system)
	}
	if len(msgs) != 3 {
		t.Fatalf("got %d messages: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Content[0]["text"] != "hi" {
		t.Fatalf("got %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content[0]["text"] != "hello" {
		t.Fatalf("got %+v", msgs[1])
	}
	if msgs[1].Content[1]["type"] != "tool_use" || msgs[1].Content[1]["id"] != "c1" || msgs[1].Content[1]["name"] != "search" {
		t.Fatalf("got %+v", msgs[1])
	}
	if msgs[2].Role != "user" || msgs[2].Content[0]["type"] != "tool_result" || msgs[2].Content[0]["tool_use_id"] != "c1" {
		t.Fatalf("got %+v", msgs[2])
	}
}

func TestAnthropicMessagesMergesAdjacentSameRoleMessages(t *testing.T) {
	m := &apiAnthropicModel{}
	_, msgs, err := m.messages([]jpf.Message{
		jpf.AssistantMessage{ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "a"}, {ID: "c2", Tool: "b"}}},
		jpf.ToolResultMessage{CallID: "c1", Result: "r1"},
		jpf.ToolResultMessage{CallID: "c2", Result: "r2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected merged messages, got %d: %+v", len(msgs), msgs)
	}
	if msgs[1].Role != "user" || len(msgs[1].Content) != 2 {
		t.Fatalf("expected two tool results merged into one message, got %+v", msgs[1])
	}
}

func TestAnthropicUserContentWithImages(t *testing.T) {
	m := &apiAnthropicModel{}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	blocks, err := m.userContent(jpf.UserMessage{Content: "look", Images: []jpf.ImageAttachment{{Source: img}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("got %+v", blocks)
	}
	if blocks[0]["type"] != "text" || blocks[0]["text"] != "look" {
		t.Fatalf("got %+v", blocks[0])
	}
	if blocks[1]["type"] != "image" {
		t.Fatalf("got %+v", blocks[1])
	}
	source, ok := blocks[1]["source"].(map[string]any)
	if !ok || source["type"] != "base64" || source["media_type"] != "image/jpeg" || source["data"] == "" {
		t.Fatalf("got %+v", blocks[1]["source"])
	}
}

func TestAnthropicUserContentEmpty(t *testing.T) {
	m := &apiAnthropicModel{}
	blocks, err := m.userContent(jpf.UserMessage{})
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 0 {
		t.Fatalf("got %+v", blocks)
	}
}

func TestAnthropicAssistantContentWithoutContentOmitsTextBlock(t *testing.T) {
	m := &apiAnthropicModel{}
	blocks := m.assistantContent(jpf.AssistantMessage{ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "search"}}})
	if len(blocks) != 1 || blocks[0]["type"] != "tool_use" {
		t.Fatalf("got %+v", blocks)
	}
}

func TestAnthropicBody(t *testing.T) {
	temp := 0.5
	topP := 1
	maxOut := 100
	m := &apiAnthropicModel{
		name: "claude-test",
		settings: apiModelSettings{
			temperature: &temp,
			topP:        &topP,
			maxOutput:   &maxOut,
		},
	}
	msgs := []anthropicMessage{{Role: "user", Content: []map[string]any{{"type": "text", "text": "hi"}}}}
	body := m.body("be nice", msgs, true, []jpf.ToolSchema{{Name: "search"}})
	if body["model"] != "claude-test" || body["max_tokens"] != 100 || body["system"] != "be nice" {
		t.Fatalf("got %+v", body)
	}
	if body["temperature"] != 0.5 || body["top_p"] != 1 {
		t.Fatalf("got %+v", body)
	}
	if body["stream"] != true {
		t.Fatalf("got %+v", body)
	}
	if _, ok := body["tools"]; !ok {
		t.Fatalf("expected tools in body: %+v", body)
	}
	choice, ok := body["tool_choice"].(map[string]any)
	if !ok || choice["type"] != "auto" {
		t.Fatalf("got %+v", body["tool_choice"])
	}
}

func TestAnthropicBodyDefaultsMaxTokensWhenUnset(t *testing.T) {
	m := &apiAnthropicModel{name: "claude-test"}
	body := m.body("", nil, false, nil)
	if body["max_tokens"] != anthropicDefaultMaxTokens {
		t.Fatalf("got %+v", body)
	}
	if _, ok := body["system"]; ok {
		t.Fatalf("did not expect a system key: %+v", body)
	}
	if _, ok := body["tools"]; ok {
		t.Fatalf("did not expect tools: %+v", body)
	}
}

func TestAnthropicTools(t *testing.T) {
	m := &apiAnthropicModel{}
	tools := m.tools([]jpf.ToolSchema{
		{
			Name:        "search",
			Description: "search the web",
			Args: []jpf.ToolArg{
				{Name: "q", Type: jpf.ToolArgString, Description: "query", Required: true},
				{Name: "n", Type: jpf.ToolArgInt, Description: "count"},
				{Name: "f", Type: jpf.ToolArgFloat, Description: "factor"},
			},
		},
	})
	if len(tools) != 1 {
		t.Fatalf("got %+v", tools)
	}
	tool := tools[0].(map[string]any)
	if tool["name"] != "search" || tool["description"] != "search the web" {
		t.Fatalf("got %+v", tool)
	}
	schema := tool["input_schema"].(map[string]any)
	props := schema["properties"].(map[string]any)
	if props["q"].(map[string]any)["type"] != "string" {
		t.Fatalf("got %+v", props["q"])
	}
	if props["n"].(map[string]any)["type"] != "integer" {
		t.Fatalf("got %+v", props["n"])
	}
	if props["f"].(map[string]any)["type"] != "number" {
		t.Fatalf("got %+v", props["f"])
	}
	required := schema["required"].([]string)
	if len(required) != 1 || required[0] != "q" {
		t.Fatalf("got %+v", required)
	}
}

func TestAnthropicValidateNoUnusableArgs(t *testing.T) {
	tests := []struct {
		name     string
		settings apiModelSettings
		kwargs   jpf.ModelResponseKwargs
	}{
		{"reasoning", apiModelSettings{reasoning: reasoningPtr(HighReasoning)}, jpf.ModelResponseKwargs{}},
		{"verbosity", apiModelSettings{verbosity: verbosityPtr(HighVerbosity)}, jpf.ModelResponseKwargs{}},
		{"presencePenalty", apiModelSettings{presencePenalty: floatPtr(0.5)}, jpf.ModelResponseKwargs{}},
		{"prediction", apiModelSettings{prediction: stringPtr("pred")}, jpf.ModelResponseKwargs{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &apiAnthropicModel{settings: tt.settings}
			if err := m.validateNoUnusableArgs(tt.kwargs); err == nil {
				t.Fatalf("expected an error for %s", tt.name)
			}
		})
	}
}

func TestAnthropicValidateNoUnusableArgsAllowsSupportedSettings(t *testing.T) {
	temp := 0.5
	topP := 1
	m := &apiAnthropicModel{settings: apiModelSettings{temperature: &temp, topP: &topP}}
	if err := m.validateNoUnusableArgs(jpf.ModelResponseKwargs{OutputFormat: struct{}{}}); err != nil {
		t.Fatalf("did not expect an error, got %v", err)
	}
}

func TestAnthropicCreateRequest(t *testing.T) {
	m := &apiAnthropicModel{
		key: "secret",
		settings: apiModelSettings{
			url:     "http://example.com/v1/messages",
			headers: map[string]string{"X-Custom": "1"},
		},
	}
	req, err := m.createRequest(context.Background(), strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != "http://example.com/v1/messages" {
		t.Fatalf("got %s", req.URL.String())
	}
	if req.Header.Get("x-api-key") != "secret" {
		t.Fatalf("got %s", req.Header.Get("x-api-key"))
	}
	if req.Header.Get("anthropic-version") != anthropicAPIVersion {
		t.Fatalf("got %s", req.Header.Get("anthropic-version"))
	}
	if req.Header.Get("X-Custom") != "1" {
		t.Fatalf("got %s", req.Header.Get("X-Custom"))
	}
}

func TestAnthropicCreateBodyData(t *testing.T) {
	m := &apiAnthropicModel{name: "claude-test"}
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
	if decoded["model"] != "claude-test" {
		t.Fatalf("got %+v", decoded)
	}
	if _, ok := decoded["system"]; ok {
		t.Fatalf("did not expect a system key: %+v", decoded)
	}
}

type anthropicTestStruct struct {
	A int    `json:"a"`
	B string `json:"b"`
}

func TestAnthropicCreateBodyDataWithOutputFormat(t *testing.T) {
	m := &apiAnthropicModel{name: "claude-test"}
	r, err := m.createBodyData([]jpf.Message{jpf.SystemMessage{Content: "be nice"}, jpf.UserMessage{Content: "hi"}}, false, anthropicTestStruct{}, nil)
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
	system, ok := decoded["system"].(string)
	if !ok || !strings.HasPrefix(system, "be nice\n\n") || !contains(system, `"a"`) || !contains(system, `"b"`) {
		t.Fatalf("got %+v", decoded["system"])
	}
}

func TestAnthropicSchema(t *testing.T) {
	m := &apiAnthropicModel{}
	schema, err := m.schema(anthropicTestStruct{})
	if err != nil {
		t.Fatal(err)
	}
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		t.Fatalf("got %+v", schema)
	}
	props, ok := schemaMap["properties"].(map[string]any)
	if !ok {
		t.Fatalf("got %+v", schemaMap)
	}
	if _, ok := props["a"]; !ok {
		t.Fatalf("got %+v", props)
	}
	if _, ok := props["b"]; !ok {
		t.Fatalf("got %+v", props)
	}
}

func TestAnthropicOutputFormatInstruction(t *testing.T) {
	m := &apiAnthropicModel{}
	instruction, err := m.outputFormatInstruction(anthropicTestStruct{})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(instruction, "JSON") || !contains(instruction, `"a"`) {
		t.Fatalf("got %q", instruction)
	}
}

func TestAnthropicApiErrorResponse(t *testing.T) {
	m := &apiAnthropicModel{}
	resp := &http.Response{
		StatusCode: 400,
		Body:       io.NopCloser(strings.NewReader(`{"type":"error","error":{"type":"invalid_request_error","message":"bad request"}}`)),
	}
	_, err := m.apiErrorResponse(resp)
	var apiErr *anthropicError
	if !errors.As(err, &apiErr) || apiErr.msg != "bad request" || apiErr.errType != "invalid_request_error" {
		t.Fatalf("got %v", err)
	}
}

func TestAnthropicApiErrorResponseMalformedBody(t *testing.T) {
	m := &apiAnthropicModel{}
	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader("not json")),
	}
	_, err := m.apiErrorResponse(resp)
	if err == nil || !contains(err.Error(), "500") {
		t.Fatalf("got %v", err)
	}
}

func TestAnthropicParseStaticResponse(t *testing.T) {
	m := &apiAnthropicModel{}
	body := `{"type":"message","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"output_tokens":2}}`
	resp, raw, err := m.parseStaticResponse(context.Background(), io.NopCloser(strings.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text != "hi" {
		t.Fatalf("got %+v", resp)
	}
	if resp.Usage.InputTokens != 1 || resp.Usage.OutputTokens != 2 {
		t.Fatalf("got %+v", resp.Usage)
	}
	if string(raw) != body {
		t.Fatalf("got %s", raw)
	}
}

func TestAnthropicParseStaticResponseInvalidJSON(t *testing.T) {
	m := &apiAnthropicModel{}
	_, _, err := m.parseStaticResponse(context.Background(), io.NopCloser(strings.NewReader("not json")))
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestAnthropicExtractOutput(t *testing.T) {
	m := &apiAnthropicModel{}
	content, toolCalls, err := m.extractOutput([]anthropicContentBlock{
		{Type: "text", Text: "hi "},
		{Type: "text", Text: "there"},
		{Type: "tool_use", ID: "c1", Name: "search", Input: json.RawMessage(`{"q":"cats"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if content != "hi there" {
		t.Fatalf("got %q", content)
	}
	if len(toolCalls) != 1 || toolCalls[0].ID != "c1" || toolCalls[0].Tool != "search" || toolCalls[0].Args["q"] != "cats" {
		t.Fatalf("got %+v", toolCalls)
	}
}

func TestAnthropicExtractOutputToolUseWithoutInput(t *testing.T) {
	m := &apiAnthropicModel{}
	_, toolCalls, err := m.extractOutput([]anthropicContentBlock{{Type: "tool_use", ID: "c1", Name: "search"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(toolCalls) != 1 || toolCalls[0].Args == nil || len(toolCalls[0].Args) != 0 {
		t.Fatalf("got %+v", toolCalls)
	}
}

func TestAnthropicExtractOutputInvalidArguments(t *testing.T) {
	m := &apiAnthropicModel{}
	_, _, err := m.extractOutput([]anthropicContentBlock{{Type: "tool_use", Name: "search", Input: json.RawMessage("not json")}})
	if err == nil {
		t.Fatal("expected an error for invalid tool arguments")
	}
}

func TestAnthropicParseStreamResponse(t *testing.T) {
	m := &apiAnthropicModel{}
	stream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"usage":{"input_tokens":3,"output_tokens":0}}}`,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"c1","name":"search"}}`,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"q\":"}}`,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"cats\"}"}}`,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":1}`,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":5}}`,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
	}, "\n")
	streamer := &recordingStreamer{}
	resp, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader(stream)), streamer)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.InputTokens != 3 || resp.Usage.OutputTokens != 5 {
		t.Fatalf("got %+v", resp.Usage)
	}
	if len(resp.Content) != 2 {
		t.Fatalf("got %+v", resp.Content)
	}
	if resp.Content[0].Type != "text" || resp.Content[0].Text != "hi" {
		t.Fatalf("got %+v", resp.Content[0])
	}
	if resp.Content[1].Type != "tool_use" || resp.Content[1].ID != "c1" || resp.Content[1].Name != "search" {
		t.Fatalf("got %+v", resp.Content[1])
	}
	if string(resp.Content[1].Input) != `{"q":"cats"}` {
		t.Fatalf("got %s", resp.Content[1].Input)
	}
	if !streamer.began || streamer.text != "hi" {
		t.Fatalf("got began=%v text=%q", streamer.began, streamer.text)
	}
}

func TestAnthropicParseStreamResponseErrorEvent(t *testing.T) {
	m := &apiAnthropicModel{}
	stream := `data: {"type":"error","error":{"type":"overloaded_error","message":"boom"}}`
	_, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader(stream)), &recordingStreamer{})
	var apiErr *anthropicError
	if !errors.As(err, &apiErr) || apiErr.msg != "boom" || apiErr.errType != "overloaded_error" {
		t.Fatalf("got %v", err)
	}
}

func TestAnthropicParseStreamResponseInvalidEvent(t *testing.T) {
	m := &apiAnthropicModel{}
	_, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader("data: not json")), &recordingStreamer{})
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestAnthropicErrorMessage(t *testing.T) {
	err := &anthropicError{errType: "invalid_request_error", msg: "bad"}
	if !contains(err.Error(), "invalid_request_error") || !contains(err.Error(), "bad") {
		t.Fatalf("got %s", err.Error())
	}
}

func reasoningPtr(v ReasoningEffort) *ReasoningEffort { return &v }
func verbosityPtr(v Verbosity) *Verbosity             { return &v }
func floatPtr(v float64) *float64                     { return &v }
func stringPtr(v string) *string                      { return &v }
