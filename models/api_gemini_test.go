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

func TestGeminiMessages(t *testing.T) {
	m := &apiGeminiModel{}
	systemMessage, parts, err := m.messages([]jpf.Message{
		jpf.SystemMessage{Content: "be nice"},
		jpf.UserMessage{Content: "hi"},
		jpf.AssistantMessage{Content: "hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if systemMessage != "be nice" {
		t.Fatalf("got %q", systemMessage)
	}
	if len(parts) != 2 {
		t.Fatalf("got %+v", parts)
	}
	first := parts[0].(map[string]any)
	if first["role"] != "user" {
		t.Fatalf("got %+v", first)
	}
	second := parts[1].(map[string]any)
	if second["role"] != "model" {
		t.Fatalf("got %+v", second)
	}
}

func TestGeminiMessagesSystemMessageMustBeFirst(t *testing.T) {
	m := &apiGeminiModel{}
	_, _, err := m.messages([]jpf.Message{
		jpf.UserMessage{Content: "hi"},
		jpf.SystemMessage{Content: "too late"},
	})
	if err == nil {
		t.Fatal("expected an error for a system message not at index 0")
	}
}

func TestGeminiMessageRole(t *testing.T) {
	m := &apiGeminiModel{}
	tests := []struct {
		msg  jpf.Message
		want string
	}{
		{jpf.UserMessage{}, "user"},
		{jpf.AssistantMessage{}, "model"},
		{jpf.ToolResultMessage{}, "user"},
	}
	for _, tt := range tests {
		got, err := m.messageRole(tt.msg)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Fatalf("got %q want %q for %T", got, tt.want, tt.msg)
		}
	}
}

func TestGeminiMessageRoleUnsupported(t *testing.T) {
	m := &apiGeminiModel{}
	if _, err := m.messageRole(jpf.DeveloperMessage{}); err == nil {
		t.Fatal("expected an error for an unsupported message type")
	}
}

func TestGeminiMessageContent(t *testing.T) {
	m := &apiGeminiModel{}

	content, err := m.messageContent(jpf.UserMessage{Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	parts := content.([]map[string]any)
	if len(parts) != 1 || parts[0]["text"] != "hi" {
		t.Fatalf("got %+v", parts)
	}

	content, err = m.messageContent(jpf.AssistantMessage{Content: "hi", ToolCalls: []jpf.ToolCall{{Tool: "search", Args: map[string]any{"q": "cats"}}}})
	if err != nil {
		t.Fatal(err)
	}
	parts = content.([]map[string]any)
	if len(parts) != 2 {
		t.Fatalf("got %+v", parts)
	}
	fc := parts[1]["functionCall"].(map[string]any)
	if fc["name"] != "search" {
		t.Fatalf("got %+v", fc)
	}

	content, err = m.messageContent(jpf.ToolResultMessage{CallID: "search", Result: "done"})
	if err != nil {
		t.Fatal(err)
	}
	parts = content.([]map[string]any)
	fr := parts[0]["functionResponse"].(map[string]any)
	if fr["name"] != "search" || fr["response"].(map[string]any)["result"] != "done" {
		t.Fatalf("got %+v", fr)
	}
}

func TestGeminiMessageContentWithImages(t *testing.T) {
	m := &apiGeminiModel{}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	content, err := m.messageContent(jpf.UserMessage{Content: "look", Images: []jpf.ImageAttachment{{Source: img}}})
	if err != nil {
		t.Fatal(err)
	}
	parts := content.([]map[string]any)
	if len(parts) != 2 {
		t.Fatalf("got %+v", parts)
	}
	inline := parts[1]["inline_data"].(map[string]any)
	if inline["mime_type"] != "image/png" {
		t.Fatalf("got %+v", inline)
	}
}

func TestGeminiMessageContentUnsupported(t *testing.T) {
	m := &apiGeminiModel{}
	if _, err := m.messageContent(jpf.SystemMessage{}); err == nil {
		t.Fatal("expected an error for an unsupported message type")
	}
}

func TestGeminiBody(t *testing.T) {
	temp := 0.5
	topP := 5
	maxOut := 100
	m := &apiGeminiModel{settings: apiModelSettings{temperature: &temp, topP: &topP, maxOutput: &maxOut}}
	body, err := m.body("be nice", []jpf.ToolSchema{{Name: "search"}}, struct{ A int }{}, []any{"content"})
	if err != nil {
		t.Fatal(err)
	}
	if len(body["contents"].([]any)) != 1 {
		t.Fatalf("got %+v", body["contents"])
	}
	sysInstr := body["systemInstruction"].(map[string]any)
	parts := sysInstr["parts"].([]map[string]any)
	if parts[0]["text"] != "be nice" {
		t.Fatalf("got %+v", sysInstr)
	}
	genConfig := body["generationConfig"].(map[string]any)
	if genConfig["temperature"] != temp || genConfig["topP"] != topP || genConfig["maxOutputTokens"] != maxOut {
		t.Fatalf("got %+v", genConfig)
	}
	if genConfig["responseMimeType"] != "application/json" {
		t.Fatalf("got %+v", genConfig)
	}
	if _, ok := body["tools"]; !ok {
		t.Fatal("expected tools to be set")
	}
}

func TestGeminiBodyMinimal(t *testing.T) {
	m := &apiGeminiModel{}
	body, err := m.body("", nil, nil, []any{})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"systemInstruction", "generationConfig", "tools"} {
		if _, ok := body[key]; ok {
			t.Fatalf("did not expect %q to be set, got %+v", key, body[key])
		}
	}
}

func TestGeminiTools(t *testing.T) {
	m := &apiGeminiModel{}
	tools := m.tools([]jpf.ToolSchema{
		{
			Name:        "search",
			Description: "search the web",
			Args: []jpf.ToolArg{
				{Name: "q", Type: jpf.ToolArgString, Required: true},
				{Name: "n", Type: jpf.ToolArgInt},
				{Name: "score", Type: jpf.ToolArgFloat},
			},
		},
	})
	if len(tools) != 1 {
		t.Fatalf("got %d tools", len(tools))
	}
	wrapper := tools[0].(map[string]any)
	decls := wrapper["functionDeclarations"].([]any)
	decl := decls[0].(map[string]any)
	if decl["name"] != "search" {
		t.Fatalf("got %+v", decl)
	}
	params := decl["parameters"].(map[string]any)
	props := params["properties"].(map[string]any)
	if props["q"].(map[string]any)["type"] != "STRING" {
		t.Fatalf("got %+v", props["q"])
	}
	if props["n"].(map[string]any)["type"] != "INTEGER" {
		t.Fatalf("got %+v", props["n"])
	}
	if props["score"].(map[string]any)["type"] != "NUMBER" {
		t.Fatalf("got %+v", props["score"])
	}
	required := params["required"].([]string)
	if len(required) != 1 || required[0] != "q" {
		t.Fatalf("got %+v", required)
	}
}

func TestGeminiValidateNoUnusableArgs(t *testing.T) {
	if err := (&apiGeminiModel{}).validateNoUnusableArgs(jpf.ModelResponseKwargs{}); err != nil {
		t.Fatal(err)
	}

	re := HighReasoning
	if err := (&apiGeminiModel{settings: apiModelSettings{reasoning: &re}}).validateNoUnusableArgs(jpf.ModelResponseKwargs{}); err == nil {
		t.Fatal("expected an error for an unsupported reasoning setting")
	}

	vb := HighVerbosity
	if err := (&apiGeminiModel{settings: apiModelSettings{verbosity: &vb}}).validateNoUnusableArgs(jpf.ModelResponseKwargs{}); err == nil {
		t.Fatal("expected an error for an unsupported verbosity setting")
	}

	pp := 0.5
	if err := (&apiGeminiModel{settings: apiModelSettings{presencePenalty: &pp}}).validateNoUnusableArgs(jpf.ModelResponseKwargs{}); err == nil {
		t.Fatal("expected an error for an unsupported presence penalty setting")
	}

	pred := "pred"
	if err := (&apiGeminiModel{settings: apiModelSettings{prediction: &pred}}).validateNoUnusableArgs(jpf.ModelResponseKwargs{}); err == nil {
		t.Fatal("expected an error for an unsupported prediction setting")
	}
}

func TestGeminiSchema(t *testing.T) {
	m := &apiGeminiModel{}
	type out struct {
		A int `json:"a"`
	}
	schema, err := m.schema(out{})
	if err != nil {
		t.Fatal(err)
	}
	cleaned := schema.(map[string]any)
	if _, ok := cleaned["$schema"]; ok {
		t.Fatalf("expected $schema to be stripped, got %+v", cleaned)
	}
}

func TestCleanGeminiSchema(t *testing.T) {
	input := map[string]any{
		"$schema":              "http://example.com",
		"$id":                  "id",
		"additionalProperties": false,
		"examples":             []any{"a"},
		"default":              "a",
		"type":                 "object",
		"properties": map[string]any{
			"a": map[string]any{"$schema": "nested"},
		},
	}
	cleaned := cleanGeminiSchema(input).(map[string]any)
	for _, key := range []string{"$schema", "$id", "additionalProperties", "examples", "default"} {
		if _, ok := cleaned[key]; ok {
			t.Fatalf("expected %q to be stripped, got %+v", key, cleaned)
		}
	}
	if cleaned["type"] != "object" {
		t.Fatalf("got %+v", cleaned)
	}
	props := cleaned["properties"].(map[string]any)
	nested := props["a"].(map[string]any)
	if _, ok := nested["$schema"]; ok {
		t.Fatalf("expected nested $schema to be stripped, got %+v", nested)
	}
}

func TestGeminiCreateRequest(t *testing.T) {
	m := &apiGeminiModel{
		name: "gemini-test",
		key:  "secret",
		settings: apiModelSettings{
			url:     "http://example.com/v1beta/models",
			headers: map[string]string{"X-Custom": "1"},
		},
	}
	req, err := m.createRequest(context.Background(), strings.NewReader("{}"), false)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != "http://example.com/v1beta/models/gemini-test:generateContent?key=secret" {
		t.Fatalf("got %s", req.URL.String())
	}
	if req.Header.Get("X-Custom") != "1" {
		t.Fatalf("got %s", req.Header.Get("X-Custom"))
	}

	streamedReq, err := m.createRequest(context.Background(), strings.NewReader("{}"), true)
	if err != nil {
		t.Fatal(err)
	}
	if streamedReq.URL.String() != "http://example.com/v1beta/models/gemini-test:streamGenerateContent?key=secret&alt=sse" {
		t.Fatalf("got %s", streamedReq.URL.String())
	}
}

func TestGeminiCreateBodyData(t *testing.T) {
	m := &apiGeminiModel{}
	r, err := m.createBodyData([]jpf.Message{jpf.UserMessage{Content: "hi"}}, nil, nil)
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
	if len(decoded["contents"].([]any)) != 1 {
		t.Fatalf("got %+v", decoded)
	}
}

func TestGeminiApiErrorResponse(t *testing.T) {
	m := &apiGeminiModel{}
	resp := &http.Response{
		StatusCode: 400,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad request","status":"INVALID_ARGUMENT","code":400}}`)),
	}
	_, err := m.apiErrorResponse(resp)
	var apiErr *geminiError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected a *geminiError, got %v (%T)", err, err)
	}
	if apiErr.msg != "bad request" {
		t.Fatalf("got %+v", apiErr)
	}
}

func TestGeminiApiErrorResponseFallback(t *testing.T) {
	m := &apiGeminiModel{}
	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader("not json")),
	}
	_, err := m.apiErrorResponse(resp)
	if err == nil || !contains(err.Error(), "500") {
		t.Fatalf("got %v", err)
	}
}

func TestGeminiParseStaticResponse(t *testing.T) {
	m := &apiGeminiModel{}
	body := `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":2}}`
	resp, raw, err := m.parseStaticResponse(context.Background(), io.NopCloser(strings.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Candidates[0].Content.Parts[0].Text != "hi" {
		t.Fatalf("got %+v", resp)
	}
	if resp.UsageMetadata.InputTokens != 1 || resp.UsageMetadata.OutputTokens != 2 {
		t.Fatalf("got %+v", resp.UsageMetadata)
	}
	if string(raw) != body {
		t.Fatalf("got %s", raw)
	}
}

func TestGeminiParseStaticResponseInvalidJSON(t *testing.T) {
	m := &apiGeminiModel{}
	_, _, err := m.parseStaticResponse(context.Background(), io.NopCloser(strings.NewReader("not json")))
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestGeminiParseStreamResponse(t *testing.T) {
	m := &apiGeminiModel{}
	stream := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"text":"he"}]}}]}`,
		`data: {"candidates":[{"content":{"parts":[{"text":"llo"}]}}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":4}}`,
		`data: [DONE]`,
	}, "\n")
	streamer := &recordingStreamer{}
	resp, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader(stream)), streamer)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Candidates[0].Content.Parts[0].Text != "hello" {
		t.Fatalf("got %+v", resp.Candidates[0].Content.Parts[0])
	}
	if resp.UsageMetadata.InputTokens != 3 || resp.UsageMetadata.OutputTokens != 4 {
		t.Fatalf("got %+v", resp.UsageMetadata)
	}
	if !streamer.began || streamer.text != "hello" {
		t.Fatalf("got began=%v text=%q", streamer.began, streamer.text)
	}
}

func TestGeminiParseStreamResponseMergesFunctionCallArgs(t *testing.T) {
	m := &apiGeminiModel{}
	stream := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"search","args":{"q":"cats"}}}]}}]}`,
		`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"search","args":{"n":3}}}]}}]}`,
	}, "\n")
	resp, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader(stream)), &recordingStreamer{})
	if err != nil {
		t.Fatal(err)
	}
	var call *geminiResponseFunctionCall
	for _, p := range resp.Candidates[0].Content.Parts {
		if p.FunctionCall != nil {
			call = p.FunctionCall
		}
	}
	if call == nil || call.Args["q"] != "cats" || call.Args["n"] != float64(3) {
		t.Fatalf("got %+v", call)
	}
}

func TestGeminiParseStreamResponseInvalidChunk(t *testing.T) {
	m := &apiGeminiModel{}
	_, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader("data: not json")), &recordingStreamer{})
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestGeminiErrorFormat(t *testing.T) {
	err := &geminiError{msg: "bad", status: "INVALID_ARGUMENT", code: 400}
	want := "gemini api returned an error: 400.INVALID_ARGUMENT - bad"
	if err.Error() != want {
		t.Fatalf("got %q want %q", err.Error(), want)
	}
}
