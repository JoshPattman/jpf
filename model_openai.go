package jpf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
		oaiRole, err := roleToOpenAI(msg.Role)
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

func (c *openAIModel) Respond(msgs []Message) (ModelResponse, error) {
	failedUsage := Usage{FailedCalls: 1}
	failedResp := ModelResponse{Usage: failedUsage}
	openAIMsgs, err := c.messagesToOpenAI(msgs)
	if err != nil {
		return failedResp, wrap(err, "could not convert messages to OpenAI format")
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
	body, err := json.Marshal(bodyMap)
	if err != nil {
		return failedResp, wrap(err, "could not encode body")
	}
	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(body))
	if err != nil {
		return failedResp, wrap(err, "could not create request")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.key))
	req.Header.Add("Content-Type", "application/json")
	for k, v := range c.extraHeaders {
		req.Header.Add(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return failedResp, wrap(err, "could not execute request")
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return failedResp, wrap(err, "could not read response body")
	}
	respTyped := struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			InputTokens  int `json:"prompt_tokens"`
			OutputTokens int `json:"completion_tokens"`
		}
	}{}
	err = json.Unmarshal(respBody, &respTyped)
	usage := Usage{
		InputTokens:  respTyped.Usage.InputTokens,
		OutputTokens: respTyped.Usage.OutputTokens,
	}
	if err != nil {
		return ModelResponse{Usage: usage.Add(Usage{FailedCalls: 1})}, wrap(err, "failed to parse response: %s", string(respBody))
	}
	if len(respTyped.Choices) == 0 {
		return ModelResponse{Usage: usage.Add(Usage{FailedCalls: 1})}, wrap(err, "response had no choices: %s", string(respBody))
	}
	content := respTyped.Choices[0].Message.Content
	return ModelResponse{
		PrimaryMessage: Message{Role: AssistantRole, Content: content},
		Usage:          usage.Add(Usage{SuccessfulCalls: 1}),
	}, nil
}
