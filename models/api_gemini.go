package models

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
	"github.com/invopop/jsonschema"
)

type apiGeminiModel struct {
	name     string
	key      string
	settings apiModelSettings
}

func (m *apiGeminiModel) Respond(ctx context.Context, msgs []jpf.Message, opts ...jpf.ModelResponseOpt) (jpf.ModelResponse, error) {
	kwargs := jpf.GetModelResponseKwargs(opts...)
	err := m.validateNoUnusableArgs(kwargs)
	if err != nil {
		return failedResponse(), utils.Wrap(err, "could not validate model setup")
	}
	body, err := m.createBodyData(msgs, kwargs.ToolSchemas, kwargs.OutputFormat)
	if err != nil {
		return failedResponse(), utils.Wrap(err, "could not create request body")
	}
	isStreamed := kwargs.Streamer != nil
	req, err := m.createRequest(ctx, body, isStreamed)
	if err != nil {
		return failedResponse(), utils.Wrap(err, "could not create request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return failedResponse(), utils.Wrap(err, "could not execute request")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return m.apiErrorResponse(resp)
	}

	var respTyped geminiStaticResponse
	var rawRespBytes []byte
	if kwargs.Streamer != nil {
		respTyped, rawRespBytes, err = m.parseStreamResponse(ctx, resp.Body, kwargs.Streamer)
	} else {
		respTyped, rawRespBytes, err = m.parseStaticResponse(ctx, resp.Body)
	}

	usage := jpf.Usage{
		InputTokens:  respTyped.UsageMetadata.InputTokens,
		OutputTokens: respTyped.UsageMetadata.OutputTokens,
	}
	if err != nil {
		return failedResponseAfter(usage), utils.Wrap(err, "failed to parse response: %s", string(rawRespBytes))
	}

	if len(respTyped.Candidates) == 0 || len(respTyped.Candidates[0].Content.Parts) == 0 {
		return failedResponseAfter(usage), fmt.Errorf("response had no content: %s", string(rawRespBytes))
	}

	toolCalls := []jpf.ToolCall{}
	var text strings.Builder

	for _, part := range respTyped.Candidates[0].Content.Parts {

		if part.Text != "" {
			text.WriteString(part.Text)
		}

		if part.FunctionCall != nil {
			toolCalls = append(toolCalls, jpf.ToolCall{
				ID:   part.FunctionCall.Name, // Gemini doesn't provide ID but we use name
				Tool: part.FunctionCall.Name,
				Args: part.FunctionCall.Args,
			})
		}
	}
	return jpf.ModelResponse{
		Message: jpf.AssistantMessage{Content: text.String(), ToolCalls: toolCalls},
		Usage:   usage.Add(jpf.Usage{SuccessfulCalls: 1}),
	}, nil
}

func (m *apiGeminiModel) parseStaticResponse(ctx context.Context, respBody io.ReadCloser) (geminiStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()
	respData, err := io.ReadAll(respBody)
	if err != nil {
		return geminiStaticResponse{}, nil, utils.Wrap(err, "could not read response body")
	}
	respTyped := geminiStaticResponse{}
	err = json.Unmarshal(respData, &respTyped)
	if err != nil {
		return geminiStaticResponse{}, respData, utils.Wrap(err, "could not unmarshal response body")
	}
	return respTyped, respData, nil
}

func (m *apiGeminiModel) parseStreamResponse(ctx context.Context, respBody io.ReadCloser, streamer jpf.ModelStreamer) (geminiStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()

	scanner := bufio.NewScanner(respBody)
	var currentFunctionCall *geminiResponseFunctionCall
	responseContent := &strings.Builder{}
	functionCalls := make([]geminiResponseFunctionCall, 0)
	var inputTokens, outputTokens int

	streamer.OnMessageBegin()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		data := line[6:]
		if bytes.Equal(data, []byte("[DONE]")) {
			break
		}

		var chunk geminiStreamChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			return geminiStaticResponse{}, nil, utils.Wrap(err, "failed to unmarshal gemini stream chunk")
		}

		if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
			for _, p := range chunk.Candidates[0].Content.Parts {
				if p.Text != "" {
					responseContent.WriteString(p.Text)
					streamer.OnMessageText(p.Text)
				}
				if p.FunctionCall != nil {
					if currentFunctionCall == nil {
						currentFunctionCall = p.FunctionCall
					} else if currentFunctionCall.Name == p.FunctionCall.Name {
						if currentFunctionCall.Args == nil {
							currentFunctionCall.Args = make(map[string]any)
						}
						maps.Copy(currentFunctionCall.Args, p.FunctionCall.Args)
					} else {
						functionCalls = append(functionCalls, *currentFunctionCall)
						currentFunctionCall = p.FunctionCall
					}
				}
			}
		}

		if chunk.UsageMetadata != nil {
			if chunk.UsageMetadata.InputTokens > 0 {
				inputTokens = chunk.UsageMetadata.InputTokens
			}
			if chunk.UsageMetadata.OutputTokens > 0 {
				outputTokens = chunk.UsageMetadata.OutputTokens
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return geminiStaticResponse{}, nil, utils.Wrap(err, "error reading gemini stream")
	}

	if currentFunctionCall != nil {
		functionCalls = append(functionCalls, *currentFunctionCall)
	}

	// Build a static-style response
	resp := geminiStaticResponse{
		Candidates: make([]struct {
			Content struct {
				Parts []geminiResponsePart `json:"parts"`
			} `json:"content"`
		}, 1),
	}
	parts := []geminiResponsePart{
		{
			Text: responseContent.String(),
		},
	}
	for _, fn := range functionCalls {
		parts = append(parts, geminiResponsePart{
			FunctionCall: &fn,
		})
	}
	resp.Candidates[0].Content.Parts = parts
	resp.UsageMetadata.InputTokens = inputTokens
	resp.UsageMetadata.OutputTokens = outputTokens

	return resp, nil, nil
}

func (m *apiGeminiModel) apiErrorResponse(resp *http.Response) (jpf.ModelResponse, error) {
	var geminiErr geminiErrorResponse
	respData, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respData, &geminiErr); err == nil && geminiErr.Error.Message != "" {
		return failedResponse(), &geminiError{
			msg:    geminiErr.Error.Message,
			status: geminiErr.Error.Status,
			code:   geminiErr.Error.Code,
		}
	}
	return failedResponse(), fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respData))
}

func (m *apiGeminiModel) createRequest(ctx context.Context, body io.Reader, isStreamed bool) (*http.Request, error) {
	var modelUrl, extraStreamParam string
	if !isStreamed {
		modelUrl = fmt.Sprintf("%s/%s:generateContent", m.settings.url, m.name)
	} else {
		modelUrl = fmt.Sprintf("%s/%s:streamGenerateContent", m.settings.url, m.name)
		extraStreamParam = "&alt=sse"
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("%s?key=%s%s", modelUrl, m.key, extraStreamParam), body)
	if err != nil {
		return nil, utils.Wrap(err, "could not create request")
	}
	req.Header.Add("Content-Type", "application/json")
	for k, v := range m.settings.headers {
		req.Header.Add(k, v)
	}
	return req.WithContext(ctx), nil
}

func (m *apiGeminiModel) createBodyData(msgs []jpf.Message, toolSchemas []jpf.ToolSchema, outputFormat any) (io.Reader, error) {
	systemMessage, geminiMsgs, err := m.messages(msgs)
	if err != nil {
		return nil, utils.Wrap(err, "could not convert messages to Gemini format")
	}
	body, err := m.body(systemMessage, toolSchemas, outputFormat, geminiMsgs)
	if err != nil {
		return nil, utils.Wrap(err, "could not create body")
	}
	bodyData, err := json.Marshal(body)
	if err != nil {
		return nil, utils.Wrap(err, "could not encode request body")
	}
	return bytes.NewReader(bodyData), nil
}

func (m *apiGeminiModel) messages(msgs []jpf.Message) (string, []any, error) {
	parts := make([]any, 0)
	systemMessage := ""
	for i, msg := range msgs {
		switch msg := msg.(type) {
		case jpf.SystemMessage:
			if i != 0 {
				return "", nil, errors.New("gemini only supports at most one system message at the start of the conversation")
			}
			systemMessage = msg.Content
		default:
			role, err := m.messageRole(msg)
			if err != nil {
				return "", nil, err
			}
			content, err := m.messageContent(msg)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, map[string]any{
				"role":  role,
				"parts": content,
			})
		}
	}
	return systemMessage, parts, nil
}

func (m *apiGeminiModel) messageRole(msg jpf.Message) (string, error) {
	switch msg.(type) {
	case jpf.UserMessage:
		return "user", nil
	case jpf.AssistantMessage:
		return "model", nil
	case jpf.ToolResultMessage:
		return "user", nil
	default:
		return "", errUnsupportedSetting("role", fmt.Sprintf("%T", msg))
	}
}

func (m *apiGeminiModel) messageContent(msg jpf.Message) (any, error) {
	var content string
	var imageAttachments []jpf.ImageAttachment
	var toolCallParts []map[string]any
	switch msg := msg.(type) {
	case jpf.UserMessage:
		content = msg.Content
		imageAttachments = msg.Images
	case jpf.AssistantMessage:
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				toolCallParts = append(toolCallParts, map[string]any{
					"functionCall": map[string]any{
						"name": tc.Tool,
						"args": tc.Args,
					},
				})
			}
		}
		content = msg.Content
	case jpf.ToolResultMessage:
		return []map[string]any{
			{
				"functionResponse": map[string]any{
					"name": msg.CallID, // Gemini does not have unique ID per tool call, but JPF sets the ID to be the tool name.
					"response": map[string]any{
						"result": msg.Result,
					},
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("cannot get content for %T", msg)
	}
	textPart := map[string]any{
		"text": content,
	}
	allParts := []map[string]any{textPart}

	for _, img := range imageAttachments {
		b64, err := img.ToBase64Encoded(false)
		if err != nil {
			return nil, errors.Join(errors.New("failed to encode image to base64"), err)
		}
		allParts = append(allParts, map[string]any{
			"inline_data": map[string]any{
				"mime_type": "image/png",
				"data":      b64,
			},
		})
	}
	if len(toolCallParts) > 0 {
		allParts = append(allParts, toolCallParts...)
	}
	return allParts, nil
}

func (m *apiGeminiModel) body(systemMessage string, toolSchemas []jpf.ToolSchema, outputFormat any, msgs []any) (map[string]any, error) {
	body := map[string]any{
		"contents": msgs,
	}
	if systemMessage != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{
				{
					"text": systemMessage,
				},
			},
		}
	}
	if m.settings.temperature != nil {
		body["generationConfig"] = map[string]any{
			"temperature": *m.settings.temperature,
		}
	}
	if m.settings.topP != nil {
		if body["generationConfig"] == nil {
			body["generationConfig"] = map[string]any{}
		}
		body["generationConfig"].(map[string]any)["topP"] = *m.settings.topP
	}
	if m.settings.maxOutput != nil && *m.settings.maxOutput != 0 {
		if body["generationConfig"] == nil {
			body["generationConfig"] = map[string]any{}
		}
		body["generationConfig"].(map[string]any)["maxOutputTokens"] = *m.settings.maxOutput
	}
	if outputFormat != nil {
		schema, err := m.schema(outputFormat)
		if err != nil {
			return nil, utils.Wrap(err, "failed to build response schema")
		}

		if body["generationConfig"] == nil {
			body["generationConfig"] = map[string]any{}
		}

		gen := body["generationConfig"].(map[string]any)

		gen["responseMimeType"] = "application/json"
		gen["responseSchema"] = schema
	}
	if len(toolSchemas) > 0 {
		body["tools"] = m.tools(toolSchemas)
	}
	return body, nil
}

func (m *apiGeminiModel) tools(toolSchemas []jpf.ToolSchema) []any {
	decls := make([]any, 0, len(toolSchemas))

	for _, tool := range toolSchemas {
		props := map[string]any{}
		required := []string{}

		for _, arg := range tool.Args {
			typ := "STRING"
			switch arg.Type {
			case jpf.ToolArgString:
				typ = "STRING"
			case jpf.ToolArgInt:
				typ = "INTEGER"
			case jpf.ToolArgFloat:
				typ = "NUMBER"
			}

			props[arg.Name] = map[string]any{
				"type":        typ,
				"description": arg.Description,
			}

			if arg.Required {
				required = append(required, arg.Name)
			}
		}

		params := map[string]any{
			"type":       "OBJECT",
			"properties": props,
		}

		if len(required) > 0 {
			params["required"] = required
		}

		decls = append(decls, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  params,
		})
	}

	return []any{
		map[string]any{
			"functionDeclarations": decls,
		},
	}
}

func (m *apiGeminiModel) validateNoUnusableArgs(kwargs jpf.ModelResponseKwargs) error {
	if m.settings.reasoning != nil {
		return errUnsupportedSetting("reasoning", m.settings.reasoning)
	}
	if m.settings.verbosity != nil {
		return errUnsupportedSetting("verbosity", m.settings.verbosity)
	}
	if m.settings.presencePenalty != nil {
		return errUnsupportedSetting("presencePenalty", m.settings.presencePenalty)
	}
	if m.settings.prediction != nil {
		return errUnsupportedSetting("prediction", m.settings.prediction)
	}
	return nil
}

func (m *apiGeminiModel) schema(obj any) (any, error) {
	r := &jsonschema.Reflector{
		BaseSchemaID:   "Anonymous",
		Anonymous:      true,
		DoNotReference: true,
	}
	s := r.Reflect(obj)
	schemaBs, err := s.MarshalJSON()
	if err != nil {
		return nil, err
	}
	schema := make(map[string]any)
	err = json.Unmarshal(schemaBs, &schema)
	if err != nil {
		return nil, err
	}
	return cleanGeminiSchema(schema), nil
}

func cleanGeminiSchema(v any) any {
	switch x := v.(type) {
	case map[string]any:
		// delete unsupported Gemini fields
		delete(x, "$schema")
		delete(x, "$id")
		delete(x, "additionalProperties")
		delete(x, "examples")
		delete(x, "default")

		for k, vv := range x {
			x[k] = cleanGeminiSchema(vv)
		}
		return x

	case []any:
		for i := range x {
			x[i] = cleanGeminiSchema(x[i])
		}
		return x

	default:
		return v
	}
}

type geminiStreamChunk struct {
	Candidates []struct {
		Content struct {
			Parts []geminiResponsePart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata *struct {
		InputTokens  int `json:"promptTokenCount"`
		OutputTokens int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

type geminiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Status  string `json:"status"`
		Code    int    `json:"code"`
	} `json:"error"`
}

type geminiResponseFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type geminiResponsePart struct {
	Text         string                      `json:"text"`
	FunctionCall *geminiResponseFunctionCall `json:"functionCall"`
}

type geminiStaticResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiResponsePart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		InputTokens  int `json:"promptTokenCount"`
		OutputTokens int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

type geminiError struct {
	msg    string
	status string
	code   int
}

func (e *geminiError) Error() string {
	return fmt.Sprintf("gemini api returned an error: %d.%s - %s", e.code, e.status, e.msg)
}
