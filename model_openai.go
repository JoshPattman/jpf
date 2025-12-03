package jpf

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
)

// NewOpenAIModel creates a Model that uses the OpenAI API.
// It requires an API key and model name, with optional configuration via variadic options.
func NewOpenAIModel(key, modelName string, opts ...OpenAIModelOpt) Model {
	model := &openAIModel{
		key:             key,
		model:           modelName,
		maxOutput:       0,
		url:             "https://api.openai.com/v1/chat/completions",
		temperature:     nil,
		reasoningEffort: nil,
		extraHeaders:    make(map[string]string),
		reasoningRole:   SystemRole,
	}
	for _, o := range opts {
		o.applyOpenAIModel(model)
	}
	return model
}

type OpenAIModelOpt interface {
	applyOpenAIModel(*openAIModel)
}

func (o WithTemperature) applyOpenAIModel(m *openAIModel)     { m.temperature = &o.X }
func (o WithReasoningEffort) applyOpenAIModel(m *openAIModel) { m.reasoningEffort = &o.X }
func (o WithURL) applyOpenAIModel(m *openAIModel)             { m.url = o.X }
func (o WithHTTPHeader) applyOpenAIModel(m *openAIModel)      { m.extraHeaders[o.K] = o.V }
func (o WithTopP) applyOpenAIModel(m *openAIModel)            { m.topP = &o.X }
func (o WithVerbosity) applyOpenAIModel(m *openAIModel)       { m.verbosity = &o.X }
func (o WithPresencePenalty) applyOpenAIModel(m *openAIModel) { m.presencePenalty = &o.X }
func (o WithPrediction) applyOpenAIModel(m *openAIModel)      { m.prediction = &o.X }
func (o WithJsonSchema) applyOpenAIModel(m *openAIModel)      { m.jsonSchema = o.X }
func (o WithMaxOutputTokens) applyOpenAIModel(m *openAIModel) { m.maxOutput = o.X }
func (o WithReasoningAs) applyOpenAIModel(m *openAIModel) {
	m.reasoningRole = o.X
	m.reasoningTransform = o.TransformContent
}
func (o WithStreamResponse) applyOpenAIModel(m *openAIModel) {
	m.streamCallbacks = &streamCallbacks{
		onBegin: o.OnBegin,
		onText:  o.OnText,
	}
}

type streamCallbacks struct {
	onBegin func()
	onText  func(string)
}

type openAIModel struct {
	key                string
	model              string
	maxOutput          int
	url                string
	temperature        *float64
	reasoningEffort    *ReasoningEffort
	topP               *int
	verbosity          *Verbosity
	presencePenalty    *float64
	prediction         *string
	extraHeaders       map[string]string
	jsonSchema         map[string]any
	reasoningRole      Role
	reasoningTransform func(string) string
	streamCallbacks    *streamCallbacks
}

func (c *openAIModel) Respond(ctx context.Context, msgs []Message) (ModelResponse, error) {
	failedUsage := Usage{FailedCalls: 1}
	failedResp := ModelResponse{Usage: failedUsage}
	body, err := c.createBodyData(msgs)
	if err != nil {
		return failedResp, wrap(err, "could not encode body")
	}
	req, err := c.createRequest(ctx, body)
	if err != nil {
		return failedResp, wrap(err, "could not create request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return failedResp, wrap(err, "could not execute request")
	}
	defer resp.Body.Close()
	var respTyped openAIAPIStaticResponse
	var rawRespBytes []byte
	if c.streamCallbacks != nil {
		respTyped, rawRespBytes, err = c.parseStreamResponse(ctx, resp.Body)
	} else {
		respTyped, rawRespBytes, err = c.parseStaticResponse(ctx, resp.Body)
	}
	usage := Usage{
		InputTokens:  respTyped.Usage.InputTokens,
		OutputTokens: respTyped.Usage.OutputTokens,
	}
	if err != nil {
		return ModelResponse{Usage: usage.Add(Usage{FailedCalls: 1})}, wrap(err, "failed to parse response: %s", string(rawRespBytes))
	}
	if respTyped.Error.Code != "" {
		return ModelResponse{Usage: usage.Add(Usage{FailedCalls: 1})}, &openAIError{
			respTyped.Error.Message,
			respTyped.Error.Type,
			respTyped.Error.Code,
		}
	}
	if len(respTyped.Choices) == 0 {
		return ModelResponse{Usage: usage.Add(Usage{FailedCalls: 1})}, wrap(err, "response had no choices: %s", string(rawRespBytes))
	}
	content := respTyped.Choices[0].Message.Content
	return ModelResponse{
		PrimaryMessage: Message{Role: AssistantRole, Content: content},
		Usage:          usage.Add(Usage{SuccessfulCalls: 1}),
	}, nil
}

// Create the body reader for an openai request.
func (c *openAIModel) createBodyData(msgs []Message) (io.Reader, error) {
	openAIMsgs, err := c.messagesToOpenAI(msgs)
	if err != nil {
		return nil, wrap(err, "could not convert messages to OpenAI format")
	}
	bodyMap := map[string]any{
		"model":    c.model,
		"messages": openAIMsgs,
	}
	if c.temperature != nil {
		bodyMap["temperature"] = *c.temperature
	}
	if c.reasoningEffort != nil {
		bodyMap["reasoning_effort"] = reasoningEffortToOpenAI(*c.reasoningEffort)
	}
	if c.verbosity != nil {
		bodyMap["verbosity"] = verbosityToOpenAI(*c.verbosity)
	}
	if c.topP != nil {
		bodyMap["top_p"] = *c.topP
	}
	if c.presencePenalty != nil {
		bodyMap["presence_penalty"] = *c.presencePenalty
	}
	if c.prediction != nil {
		bodyMap["prediction"] = *c.prediction
	}
	if c.maxOutput != 0 {
		bodyMap["max_completion_tokens"] = c.maxOutput
	}
	if c.jsonSchema != nil {
		bodyMap["response_format"] = jsonSchemaToOpenAI(c.jsonSchema)
	}
	if c.streamCallbacks != nil {
		bodyMap["stream"] = true
		bodyMap["stream_options"] = map[string]any{"include_usage": true}
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, wrap(err, "could not encode body")
	}
	return bytes.NewBuffer(body), nil
}

// Wrap the openai request body up as an http.Request, with headers and context.
func (c *openAIModel) createRequest(ctx context.Context, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest("POST", c.url, body)
	if err != nil {
		return nil, wrap(err, "could not create request")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.key))
	req.Header.Add("Content-Type", "application/json")
	for k, v := range c.extraHeaders {
		req.Header.Add(k, v)
	}
	return req.WithContext(ctx), nil
}

// Parse a non-streaming response from OpenAI.
func (c *openAIModel) parseStaticResponse(ctx context.Context, respBody io.ReadCloser) (openAIAPIStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()
	respTyped := openAIAPIStaticResponse{}
	respData, err := io.ReadAll(respBody)
	if err != nil {
		return openAIAPIStaticResponse{}, respData, wrap(err, "could not read response body")
	}
	err = json.Unmarshal(respData, &respTyped)
	if err != nil {
		return openAIAPIStaticResponse{}, respData, wrap(err, "could not unmarshal response body: %s", string(respData))
	}
	return respTyped, respData, nil
}

// Parse a streaming response from OpenAI, calling callbacks as data arrives.
func (c *openAIModel) parseStreamResponse(ctx context.Context, respBody io.ReadCloser) (openAIAPIStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()
	scanner := bufio.NewScanner(respBody)
	var fullContent strings.Builder
	var inputTokens, outputTokens int

	if c.streamCallbacks != nil && c.streamCallbacks.onBegin != nil {
		c.streamCallbacks.onBegin()
	}

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
			return openAIAPIStaticResponse{}, nil, wrap(err, "failed to unmarshal stream chunk")
		}
		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			fullContent.WriteString(content)
			if c.streamCallbacks != nil && c.streamCallbacks.onText != nil {
				c.streamCallbacks.onText(content)
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
		return openAIAPIStaticResponse{}, nil, wrap(err, "error reading stream")
	}

	// Build the response in the same format as parseStaticResponse
	response := openAIAPIStaticResponse{
		Choices: make([]struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}, 1),
	}
	response.Choices[0].Message.Content = fullContent.String()
	response.Usage.InputTokens = inputTokens
	response.Usage.OutputTokens = outputTokens

	return response, nil, nil
}

func roleToOpenAI(role Role) (string, error) {
	switch role {
	case SystemRole:
		return "system", nil
	case UserRole:
		return "user", nil
	case AssistantRole:
		return "assistant", nil
	default:
		return "", fmt.Errorf("openai does not support that role: %s", role.String())
	}
}

func (m *openAIModel) messagesToOpenAI(msgs []Message) (any, error) {
	jsonMessages := make([]map[string]any, 0)
	for _, msg := range msgs {
		role := msg.Role
		contentStr := msg.Content
		if role == ReasoningRole {
			role = m.reasoningRole
			if m.reasoningTransform != nil {
				contentStr = m.reasoningTransform(contentStr)
			}
		}
		var content any
		if len(msg.Images) == 0 {
			content = contentStr
		} else {
			cont := []map[string]any{
				{
					"type": "text",
					"text": contentStr,
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
			content = cont
		}
		oaiRole, err := roleToOpenAI(role)
		if err != nil {
			return nil, err
		}

		jsonMessages = append(jsonMessages, map[string]any{
			"role":    oaiRole,
			"content": content,
		})
	}
	return jsonMessages, nil
}

func reasoningEffortToOpenAI(re ReasoningEffort) string {
	switch re {
	case LowReasoning:
		return "low"
	case MediumReasoning:
		return "medium"
	case HighReasoning:
		return "high"
	default:
		panic("not possible")
	}
}

func verbosityToOpenAI(v Verbosity) string {
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

func jsonSchemaToOpenAI(schema map[string]any) map[string]any {
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "custom_schema",
			"schema": schema,
			"strict": true,
		},
	}
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type openAIAPIStaticResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		InputTokens  int `json:"prompt_tokens"`
		OutputTokens int `json:"completion_tokens"`
	}
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	}
}

type openAIError struct {
	msg     string
	errType string
	code    string
}

func (e *openAIError) Error() string {
	return fmt.Sprintf("openai api returned an error: %s.%s - %s", e.errType, e.code, e.msg)
}
