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
	"strings"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
	"github.com/invopop/jsonschema"
)

type apiOpenAIModel struct {
	name     string
	key      string
	settings apiModelSettings
}

func (m *apiOpenAIModel) Respond(ctx context.Context, msgs []jpf.Message, opts ...jpf.ModelResponseOpt) (jpf.ModelResponse, error) {
	kwargs := jpf.GetModelResponseKwargs(opts...)
	err := m.validateNoUnusableArgs(kwargs)
	if err != nil {
		return jpf.ModelResponse{}, err
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

	var respTyped openAIAPIStaticResponse
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
	if len(respTyped.Choices) == 0 {
		return failedResponseAfter(usage), utils.Wrap(err, "response had no choices: %s", string(rawRespBytes))
	}
	content := respTyped.Choices[0].Message.Content
	toolCalls := make([]jpf.ToolCall, len(respTyped.Choices[0].Message.ToolCalls))
	for i, tc := range respTyped.Choices[0].Message.ToolCalls {
		args := make(map[string]any)
		err := json.NewDecoder(bytes.NewBufferString(tc.Function.Arguments)).Decode(&args)
		if err != nil {
			return failedResponseAfter(usage), utils.Wrap(err, "could not decode tool arguments")
		}
		toolCalls[i] = jpf.ToolCall{
			ID:   tc.ID,
			Tool: tc.Function.Name,
			Args: args,
		}
	}
	return jpf.ModelResponse{
		Message: jpf.AssistantMessage{Content: content, ToolCalls: toolCalls},
		Usage:   usage.Add(jpf.Usage{SuccessfulCalls: 1}),
	}, nil
}

func (m *apiOpenAIModel) parseStaticResponse(ctx context.Context, respBody io.ReadCloser) (openAIAPIStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()
	respTyped := openAIAPIStaticResponse{}
	respData, err := io.ReadAll(respBody)
	if err != nil {
		return openAIAPIStaticResponse{}, respData, utils.Wrap(err, "could not read response body")
	}
	err = json.Unmarshal(respData, &respTyped)
	if err != nil {
		return openAIAPIStaticResponse{}, respData, utils.Wrap(err, "could not unmarshal response body: %s", string(respData))
	}
	return respTyped, respData, nil
}

func (m *apiOpenAIModel) parseStreamResponse(ctx context.Context, respBody io.ReadCloser, streamer jpf.ModelStreamer) (openAIAPIStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()
	scanner := bufio.NewScanner(respBody)
	var fullContent strings.Builder
	toolCalls := make(map[int]openAIToolCall)
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

		var chunk openAIStreamChunk

		if err := json.Unmarshal(data, &chunk); err != nil {
			return openAIAPIStaticResponse{}, nil, utils.Wrap(err, "failed to unmarshal stream chunk")
		}
		if chunk.Error.Code != "" {
			return openAIAPIStaticResponse{}, nil, &openAIError{
				chunk.Error.Message,
				chunk.Error.Type,
				chunk.Error.Code,
			}
		}
		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			fullContent.WriteString(content)
			streamer.OnMessageText(content)
			chunkToolCalls := chunk.Choices[0].Delta.ToolCalls
			for _, chunkToolCall := range chunkToolCalls {
				existingCall := toolCalls[chunkToolCall.Index]
				if chunkToolCall.ID != nil {
					existingCall.ID = *chunkToolCall.ID
				}
				if chunkToolCall.Type != nil {
					existingCall.Type = *chunkToolCall.Type
				}
				if chunkToolCall.Function != nil {
					if chunkToolCall.Function.Name != "" {
						existingCall.Function.Name = chunkToolCall.Function.Name
					}
					existingCall.Function.Arguments += chunkToolCall.Function.Arguments
				}
				toolCalls[chunkToolCall.Index] = existingCall
			}
		}
		if chunk.Usage.PromptTokens > 0 {
			inputTokens = chunk.Usage.PromptTokens
		}
		if chunk.Usage.CompletionTokens > 0 {
			outputTokens = chunk.Usage.CompletionTokens
		}
	}

	if err := scanner.Err(); err != nil {
		return openAIAPIStaticResponse{}, nil, utils.Wrap(err, "error reading stream")
	}

	// Build the response in the same format as parseStaticResponse
	response := openAIAPIStaticResponse{
		Choices: make([]openAIStaticChoiceResponse, 1),
	}
	calls := make([]openAIToolCall, len(toolCalls))
	for i := range calls {
		calls[i] = toolCalls[i]
	}
	response.Choices[0].Message.ToolCalls = calls
	response.Choices[0].Message.Content = fullContent.String()
	response.Choices[0].Message.ToolCalls = calls
	response.Usage.InputTokens = inputTokens
	response.Usage.OutputTokens = outputTokens

	return response, nil, nil
}

func (m *apiOpenAIModel) apiErrorResponse(resp *http.Response) (jpf.ModelResponse, error) {
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

func (m *apiOpenAIModel) createRequest(ctx context.Context, body io.Reader) (*http.Request, error) {
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

func (m *apiOpenAIModel) createBodyData(msgs []jpf.Message, isStreamed bool, outputFormat any, toolSchemas []jpf.ToolSchema) (io.Reader, error) {
	apiMessages, err := m.messages(msgs)
	if err != nil {
		return nil, utils.Wrap(err, "could not convert messages to OpenAI format")
	}
	body, err := m.body(apiMessages, isStreamed, outputFormat, toolSchemas)
	if err != nil {
		return nil, utils.Wrap(err, "could not create OpenAI format body")
	}

	bodyData, err := json.Marshal(body)
	if err != nil {
		return nil, utils.Wrap(err, "could not encode body")
	}
	return bytes.NewReader(bodyData), nil
}

func (m *apiOpenAIModel) messages(msgs []jpf.Message) ([]openAIAPIMessage, error) {
	jsonMessages := make([]openAIAPIMessage, 0)
	for _, msg := range msgs {
		role, err := m.messageRole(msg)
		if err != nil {
			return nil, err
		}
		content, err := m.messageContent(msg)
		if err != nil {
			return nil, err
		}
		var callID string
		if msg, ok := msg.(jpf.ToolResultMessage); ok {
			callID = msg.CallID
		}
		var toolCalls []openAIToolCall
		if msg, ok := msg.(jpf.AssistantMessage); ok {
			toolCalls, err = m.toolCalls(msg)
			if err != nil {
				return nil, err
			}
		}
		jsonMessages = append(jsonMessages, openAIAPIMessage{
			Role:       role,
			Content:    content,
			ToolCallID: callID,
			ToolCalls:  toolCalls,
		})
	}
	return jsonMessages, nil
}

func (m *apiOpenAIModel) messageRole(msg jpf.Message) (string, error) {
	switch msg.(type) {
	case jpf.UserMessage:
		return "user", nil
	case jpf.AssistantMessage:
		return "assistant", nil
	case jpf.DeveloperMessage:
		return "developer", nil
	case jpf.SystemMessage:
		return "system", nil
	case jpf.ToolResultMessage:
		return "tool", nil
	default:
		return "", errUnsupportedSetting("role", fmt.Sprintf("%T", msg))
	}
}

func (m *apiOpenAIModel) messageContent(msg jpf.Message) (any, error) {
	switch msg := msg.(type) {
	case jpf.UserMessage:
		if len(msg.Images) == 0 {
			return msg.Content, nil
		} else {
			cont := []map[string]any{
				{
					"type": "text",
					"text": msg.Content,
				},
			}
			for _, img := range msg.Images {
				b64, err := img.ToBase64Encoded(true)
				if err != nil {
					return nil, errors.Join(errors.New("failed to encode image to base64"), err)
				}
				cont = append(cont, map[string]any{
					"type": "image_url",
					"image_url": map[string]any{
						"url": b64,
					},
				},
				)
			}
			return cont, nil
		}
	case jpf.SystemMessage:
		return msg.Content, nil
	case jpf.AssistantMessage:
		return msg.Content, nil
	case jpf.DeveloperMessage:
		return msg.Content, nil
	case jpf.ToolResultMessage:
		return msg.Result, nil
	default:
		panic("unreachable")
	}
}

func (m *apiOpenAIModel) toolCalls(msg jpf.AssistantMessage) ([]openAIToolCall, error) {
	if len(msg.ToolCalls) == 0 {
		return nil, nil
	}
	calls := make([]openAIToolCall, len(msg.ToolCalls))
	for i, tc := range msg.ToolCalls {
		args := bytes.NewBuffer(nil)
		err := json.NewEncoder(args).Encode(tc.Args)
		if err != nil {
			return nil, err
		}
		calls[i] = openAIToolCall{
			Type: "function",
			ID:   tc.ID,
			Function: openAIToolCallFunction{
				Name:      tc.Tool,
				Arguments: args.String(),
			},
		}
	}
	return calls, nil
}

func (m *apiOpenAIModel) body(msgs []openAIAPIMessage, isStreamed bool, outputFormat any, toolSchemas []jpf.ToolSchema) (map[string]any, error) {
	bodyMap := map[string]any{
		"model":    m.name,
		"messages": msgs,
	}
	if m.settings.temperature != nil {
		bodyMap["temperature"] = *m.settings.temperature
	}
	if m.settings.reasoning != nil {
		bodyMap["reasoning_effort"] = m.reasoningEffort(*m.settings.reasoning)
	}
	if m.settings.verbosity != nil {
		bodyMap["verbosity"] = m.verbosity(*m.settings.verbosity)
	}
	if m.settings.topP != nil {
		bodyMap["top_p"] = *m.settings.topP
	}
	if m.settings.presencePenalty != nil {
		bodyMap["presence_penalty"] = *m.settings.presencePenalty
	}
	if m.settings.prediction != nil {
		bodyMap["prediction"] = *m.settings.prediction
	}
	if m.settings.maxOutput != nil {
		bodyMap["max_completion_tokens"] = m.settings.maxOutput
	}
	if outputFormat != nil {
		schem, err := m.schema(outputFormat)
		if err != nil {
			return nil, errors.Join(errors.New("failed to create schema"), err)
		}
		bodyMap["response_format"] = schem
	}
	if isStreamed {
		bodyMap["stream"] = true
		bodyMap["stream_options"] = map[string]any{"include_usage": true}
	}

	if len(toolSchemas) > 0 {
		bodyMap["tools"] = m.tools(toolSchemas)
		bodyMap["tool_choice"] = "auto"
	}
	return bodyMap, nil
}

func (m *apiOpenAIModel) tools(toolSchemas []jpf.ToolSchema) []any {
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
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  params,
				"strict":      true,
			},
		})
	}
	return openAITools
}

func (m *apiOpenAIModel) validateNoUnusableArgs(kwargs jpf.ModelResponseKwargs) error {
	return nil
}

func (m *apiOpenAIModel) schema(obj any) (any, error) {
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
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "custom_schema",
			"schema": schema,
			"strict": true,
		},
	}, nil
}

func (m *apiOpenAIModel) reasoningEffort(re ReasoningEffort) string {
	switch re {
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

func (m *apiOpenAIModel) verbosity(v Verbosity) string {
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

type openAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIToolCall struct {
	Type     string                 `json:"type"`
	ID       string                 `json:"id"`
	Function openAIToolCallFunction `json:"function"`
}

type openAIStreamedToolCallDelta struct {
	Type     *string                 `json:"type"`
	ID       *string                 `json:"id"`
	Index    int                     `json:"index"`
	Function *openAIToolCallFunction `json:"function"`
}

type openAIAPIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string                        `json:"content"`
			ToolCalls []openAIStreamedToolCallDelta `json:"tool_calls,omitempty"`
		} `json:"delta"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

type openAIStaticChoiceResponse struct {
	Message struct {
		Content   string           `json:"content"`
		ToolCalls []openAIToolCall `json:"tool_calls"`
	} `json:"message"`
}

type openAIAPIStaticResponse struct {
	Choices []openAIStaticChoiceResponse `json:"choices"`
	Usage   struct {
		InputTokens  int `json:"prompt_tokens"`
		OutputTokens int `json:"completion_tokens"`
	}
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

type openAIError struct {
	msg     string
	errType string
	code    string
}

func (e *openAIError) Error() string {
	return fmt.Sprintf("openai api returned an error: %s.%s - %s", e.errType, e.code, e.msg)
}
