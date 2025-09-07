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
func NewOpenAIModel(key, modelName string, opts ...openAIModelOpt) ChatCaller {
	model := &openAIModel{
		key:             key,
		model:           modelName,
		maxInput:        0,
		maxOutput:       0,
		url:             "https://api.openai.com/v1/chat/completions",
		temperature:     nil,
		reasoningEffort: nil,
		extraHeaders:    make(map[string]string),
	}
	for _, o := range opts {
		o.applyOpenAIModel(model)
	}
	return model
}

type openAIModelOpt interface {
	applyOpenAIModel(*openAIModel)
}

func (o WithTemperature) applyOpenAIModel(m *openAIModel)     { m.temperature = &o.X }
func (o WithReasoningEffort) applyOpenAIModel(m *openAIModel) { m.reasoningEffort = &o.X }
func (o WithURL) applyOpenAIModel(m *openAIModel)             { m.url = o.X }
func (o WithHTTPHeader) applyOpenAIModel(m *openAIModel)      { m.extraHeaders[o.K] = o.V }

type openAIModel struct {
	key             string
	model           string
	maxInput        int
	maxOutput       int
	url             string
	temperature     *float64
	reasoningEffort *ReasoningEffort
	extraHeaders    map[string]string
}

func (c *openAIModel) Tokens() (int, int) {
	return c.maxInput, c.maxOutput
}

func roleToOpenAI(role Role) string {
	switch role {
	case SystemRole:
		return "system"
	case UserRole:
		return "user"
	case AssistantRole:
		return "assistant"
	default:
		panic("not a valid role")
	}
}

func messagesToOpenAI(msgs []Message) (any, error) {
	jsonMessages := make([]map[string]any, 0)
	for _, msg := range msgs {
		var content any
		if len(msg.Images) == 0 {
			content = msg.Content
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
			content = cont
		}
		if msg.Role == ReasoningRole {
			return nil, fmt.Errorf("reasoning role not allowed in openAI format, consider using NewSystemReasonModel")
		}
		jsonMessages = append(jsonMessages, map[string]any{
			"role":    roleToOpenAI(msg.Role),
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

func (c *openAIModel) Call(msgs []Message) (ChatResult, error) {
	openAIMsgs, err := messagesToOpenAI(msgs)
	if err != nil {
		return ChatResult{}, err
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
	body, err := json.Marshal(bodyMap)
	if err != nil {
		return ChatResult{}, err
	}
	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(body))
	if err != nil {
		return ChatResult{}, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.key))
	req.Header.Add("Content-Type", "application/json")
	for k, v := range c.extraHeaders {
		req.Header.Add(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ChatResult{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatResult{}, err
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
	if err != nil || len(respTyped.Choices) == 0 || respTyped.Choices[0].Message.Content == "" {
		return ChatResult{Usage: Usage(respTyped.Usage)}, fmt.Errorf("failed to parse response: %s", string(respBody))
	}
	content := respTyped.Choices[0].Message.Content
	return ChatResult{Primary: Message{Role: AssistantRole, Content: content}, Usage: Usage(respTyped.Usage)}, nil
}
