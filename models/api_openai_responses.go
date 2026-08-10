package models

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
	"github.com/invopop/jsonschema"
)

// apiOpenAIResponsesModel talks to OpenAI's Responses API (https://api.openai.com/v1/responses),
// as opposed to apiOpenAIModel which uses the older Chat Completions API.
type apiOpenAIResponsesModel struct {
	name     string
	key      string
	settings apiModelSettings
}

func (m *apiOpenAIResponsesModel) Respond(ctx context.Context, msgs []jpf.Message, opts ...jpf.ModelResponseOpt) (jpf.ModelResponse, error) {
	kwargs := jpf.GetModelResponseKwargs(opts...)
	err := m.validateNoUnusableArgs(kwargs)
	if err != nil {
		return failedResponse(), utils.Wrap(err, "could not validate model setup")
	}
	isStreamed := kwargs.Streamer != nil
	body, err := m.createBodyData(msgs, isStreamed, kwargs.OutputFormat, kwargs.ToolSchemas)
	if err != nil {
		return failedResponse(), utils.Wrap(err, "could not create request body")
	}
	req, err := m.createRequest(ctx, body)
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

	var respTyped openAIResponsesStaticResponse
	var rawRespBytes []byte
	if kwargs.Streamer != nil {
		respTyped, rawRespBytes, err = m.parseStreamResponse(ctx, resp.Body, kwargs.Streamer)
	} else {
		respTyped, rawRespBytes, err = m.parseStaticResponse(ctx, resp.Body)
	}

	usage := jpf.Usage{
		InputTokens:  respTyped.Usage.InputTokens,
		OutputTokens: respTyped.Usage.OutputTokens,
	}
	if err != nil {
		return failedResponseAfter(usage), utils.Wrap(err, "failed to parse response: %s", string(rawRespBytes))
	}
	if respTyped.Error.Code != "" {
		return failedResponseAfter(usage), &openAIError{
			respTyped.Error.Message,
			respTyped.Error.Type,
			respTyped.Error.Code,
		}
	}

	content, toolCalls, err := m.extractOutput(respTyped.Output)
	if err != nil {
		return failedResponseAfter(usage), utils.Wrap(err, "could not extract output: %s", string(rawRespBytes))
	}

	return jpf.ModelResponse{
		Message: jpf.AssistantMessage{Content: content, ToolCalls: toolCalls},
		Usage:   usage.Add(jpf.Usage{SuccessfulCalls: 1}),
	}, nil
}

func (m *apiOpenAIResponsesModel) extractOutput(output []openAIResponsesOutputItem) (string, []jpf.ToolCall, error) {
	var content string
	toolCalls := make([]jpf.ToolCall, 0)
	for _, item := range output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				switch part.Type {
				case "output_text":
					content += part.Text
				case "refusal":
					content += part.Refusal
				}
			}
		case "function_call":
			args := make(map[string]any)
			if item.Arguments != "" {
				err := json.NewDecoder(bytes.NewBufferString(item.Arguments)).Decode(&args)
				if err != nil {
					return "", nil, utils.Wrap(err, "could not decode tool arguments")
				}
			}
			toolCalls = append(toolCalls, jpf.ToolCall{
				ID:   item.CallID,
				Tool: item.Name,
				Args: args,
			})
		}
	}
	return content, toolCalls, nil
}

func (m *apiOpenAIResponsesModel) parseStaticResponse(ctx context.Context, respBody io.ReadCloser) (openAIResponsesStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()
	respTyped := openAIResponsesStaticResponse{}
	respData, err := io.ReadAll(respBody)
	if err != nil {
		return openAIResponsesStaticResponse{}, respData, utils.Wrap(err, "could not read response body")
	}
	err = json.Unmarshal(respData, &respTyped)
	if err != nil {
		return openAIResponsesStaticResponse{}, respData, utils.Wrap(err, "could not unmarshal response body: %s", string(respData))
	}
	return respTyped, respData, nil
}

func (m *apiOpenAIResponsesModel) parseStreamResponse(ctx context.Context, respBody io.ReadCloser, streamer jpf.ModelStreamer) (openAIResponsesStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()
	scanner := bufio.NewScanner(respBody)

	streamer.OnMessageBegin()

	var finalResponse *openAIResponsesStaticResponse
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		data := line[6:]

		var event openAIResponsesStreamEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return openAIResponsesStaticResponse{}, nil, utils.Wrap(err, "failed to unmarshal stream event")
		}

		switch event.Type {
		case "response.output_text.delta":
			streamer.OnMessageText(event.Delta)
		case "response.completed", "response.incomplete":
			finalResponse = event.Response
		case "error":
			return openAIResponsesStaticResponse{}, nil, &openAIError{
				event.Message,
				"error",
				event.Code,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return openAIResponsesStaticResponse{}, nil, utils.Wrap(err, "error reading stream")
	}

	if finalResponse == nil {
		return openAIResponsesStaticResponse{}, nil, errors.New("stream ended without a completed response")
	}

	return *finalResponse, nil, nil
}

func (m *apiOpenAIResponsesModel) apiErrorResponse(resp *http.Response) (jpf.ModelResponse, error) {
	var errResp openAIErrorResponse
	respData, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respData, &errResp); err != nil {
		return failedResponse(), utils.Wrap(fmt.Errorf("http status %d", resp.StatusCode), "request failed: %s", string(respData))
	}
	return failedResponse(), &openAIError{
		errResp.Error.Message,
		errResp.Error.Type,
		errResp.Error.Code,
	}
}

func (m *apiOpenAIResponsesModel) createRequest(ctx context.Context, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest("POST", m.settings.url, body)
	if err != nil {
		return nil, utils.Wrap(err, "could not create request")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", m.key))
	req.Header.Add("Content-Type", "application/json")
	for k, v := range m.settings.headers {
		req.Header.Add(k, v)
	}
	return req.WithContext(ctx), nil
}

func (m *apiOpenAIResponsesModel) createBodyData(msgs []jpf.Message, isStreamed bool, outputFormat any, toolSchemas []jpf.ToolSchema) (io.Reader, error) {
	input, err := m.input(msgs)
	if err != nil {
		return nil, utils.Wrap(err, "could not convert messages to OpenAI responses format")
	}
	body, err := m.body(input, isStreamed, outputFormat, toolSchemas)
	if err != nil {
		return nil, utils.Wrap(err, "could not create OpenAI responses format body")
	}

	bodyData, err := json.Marshal(body)
	if err != nil {
		return nil, utils.Wrap(err, "could not encode body")
	}
	return bytes.NewReader(bodyData), nil
}

func (m *apiOpenAIResponsesModel) input(msgs []jpf.Message) ([]any, error) {
	items := make([]any, 0, len(msgs))
	for _, msg := range msgs {
		switch msg := msg.(type) {
		case jpf.UserMessage:
			items = append(items, map[string]any{
				"role":    "user",
				"content": m.userContent(msg),
			})
		case jpf.SystemMessage:
			items = append(items, map[string]any{
				"role":    "system",
				"content": msg.Content,
			})
		case jpf.DeveloperMessage:
			items = append(items, map[string]any{
				"role":    "developer",
				"content": msg.Content,
			})
		case jpf.AssistantMessage:
			if msg.Content != "" {
				items = append(items, map[string]any{
					"role":    "assistant",
					"content": msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls {
				args := bytes.NewBuffer(nil)
				if err := json.NewEncoder(args).Encode(tc.Args); err != nil {
					return nil, err
				}
				items = append(items, map[string]any{
					"type":      "function_call",
					"call_id":   tc.ID,
					"name":      tc.Tool,
					"arguments": args.String(),
				})
			}
		case jpf.ToolResultMessage:
			items = append(items, map[string]any{
				"type":    "function_call_output",
				"call_id": msg.CallID,
				"output":  msg.Result,
			})
		default:
			return nil, errUnsupportedSetting("role", fmt.Sprintf("%T", msg))
		}
	}
	return items, nil
}

func (m *apiOpenAIResponsesModel) userContent(msg jpf.UserMessage) any {
	if len(msg.Images) == 0 {
		return msg.Content
	}
	cont := []map[string]any{
		{
			"type": "input_text",
			"text": msg.Content,
		},
	}
	for _, img := range msg.Images {
		b64, err := img.ToBase64Encoded(true)
		if err != nil {
			continue
		}
		cont = append(cont, map[string]any{
			"type":      "input_image",
			"image_url": b64,
		})
	}
	return cont
}

func (m *apiOpenAIResponsesModel) body(input []any, isStreamed bool, outputFormat any, toolSchemas []jpf.ToolSchema) (map[string]any, error) {
	bodyMap := map[string]any{
		"model": m.name,
		"input": input,
		// The Responses API persists request/response data server-side by default;
		// disable that so behaviour matches the Chat Completions model.
		"store": false,
	}
	if m.settings.temperature != nil {
		bodyMap["temperature"] = *m.settings.temperature
	}
	if m.settings.reasoning != nil {
		bodyMap["reasoning"] = map[string]any{"effort": m.reasoningEffort(*m.settings.reasoning)}
	}
	if m.settings.topP != nil {
		bodyMap["top_p"] = *m.settings.topP
	}
	if m.settings.maxOutput != nil {
		bodyMap["max_output_tokens"] = *m.settings.maxOutput
	}

	text := map[string]any{}
	if m.settings.verbosity != nil {
		text["verbosity"] = m.verbosity(*m.settings.verbosity)
	}
	if outputFormat != nil {
		schem, err := m.schema(outputFormat)
		if err != nil {
			return nil, errors.Join(errors.New("failed to create schema"), err)
		}
		text["format"] = schem
	}
	if len(text) > 0 {
		bodyMap["text"] = text
	}

	if isStreamed {
		bodyMap["stream"] = true
	}

	if len(toolSchemas) > 0 {
		bodyMap["tools"] = m.tools(toolSchemas)
		bodyMap["tool_choice"] = "auto"
	}
	return bodyMap, nil
}

func (m *apiOpenAIResponsesModel) tools(toolSchemas []jpf.ToolSchema) []any {
	openAITools := make([]any, 0, len(toolSchemas))
	for _, tool := range toolSchemas {
		props := map[string]any{}
		required := []string{}

		for _, arg := range tool.Args {
			t := "string"

			switch arg.Type {
			case jpf.ToolArgString:
				t = "string"
			case jpf.ToolArgInt:
				t = "integer"
			case jpf.ToolArgFloat:
				t = "number"
			default:
				panic("unreachable")
			}

			props[arg.Name] = map[string]any{
				"type":        t,
				"description": arg.Description,
			}

			if arg.Required {
				required = append(required, arg.Name)
			}
		}

		params := map[string]any{
			"type":                 "object",
			"properties":           props,
			"additionalProperties": false,
		}

		if len(required) > 0 {
			params["required"] = required
		}

		openAITools = append(openAITools, map[string]any{
			"type":        "function",
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  params,
			"strict":      true,
		})
	}
	return openAITools
}

func (m *apiOpenAIResponsesModel) validateNoUnusableArgs(kwargs jpf.ModelResponseKwargs) error {
	if m.settings.presencePenalty != nil {
		return errUnsupportedSetting("presencePenalty", *m.settings.presencePenalty)
	}
	if m.settings.prediction != nil {
		return errUnsupportedSetting("prediction", *m.settings.prediction)
	}
	return nil
}

func (m *apiOpenAIResponsesModel) schema(obj any) (any, error) {
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
	return map[string]any{
		"type":   "json_schema",
		"name":   "custom_schema",
		"schema": schema,
		"strict": true,
	}, nil
}

func (m *apiOpenAIResponsesModel) reasoningEffort(re ReasoningEffort) string {
	switch re {
	case NoneReasoning:
		return "none"
	case LowReasoning:
		return "low"
	case MediumReasoning:
		return "medium"
	case HighReasoning:
		return "high"
	case XHighReasoning:
		return "xhigh"
	default:
		panic("not possible")
	}
}

func (m *apiOpenAIResponsesModel) verbosity(v Verbosity) string {
	switch v {
	case LowVerbosity:
		return "low"
	case MediumVerbosity:
		return "medium"
	case HighVerbosity:
		return "high"
	default:
		panic("not possible")
	}
}

type openAIResponsesContentPart struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Refusal string `json:"refusal"`
}

type openAIResponsesOutputItem struct {
	Type      string                       `json:"type"`
	Role      string                       `json:"role,omitempty"`
	Content   []openAIResponsesContentPart `json:"content,omitempty"`
	CallID    string                       `json:"call_id,omitempty"`
	Name      string                       `json:"name,omitempty"`
	Arguments string                       `json:"arguments,omitempty"`
}

type openAIResponsesStaticResponse struct {
	Output []openAIResponsesOutputItem `json:"output"`
	Usage  struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

type openAIResponsesStreamEvent struct {
	Type     string                         `json:"type"`
	Delta    string                         `json:"delta"`
	Response *openAIResponsesStaticResponse `json:"response"`
	Message  string                         `json:"message"`
	Code     string                         `json:"code"`
}
