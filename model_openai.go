package jpf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type OpenAIModelBuilder struct {
	isReasoning bool
	model       *openAIModel
}

func BuildOpenAIModel(key, modelName string, isReasoning bool) *OpenAIModelBuilder {
	model := &OpenAIModelBuilder{
		model: &openAIModel{
			key:             key,
			model:           modelName,
			maxInput:        0,
			maxOutput:       0,
			url:             "https://api.openai.com/v1/chat/completions",
			temperature:     nil,
			reasoningEffort: nil,
			extraHeaders:    make(map[string]string),
		},
		isReasoning: isReasoning,
	}
	if isReasoning {
		re := LowReasoning
		model.model.reasoningEffort = &re
	} else {
		te := 0.0
		model.model.temperature = &te
	}
	return model
}

func (b *OpenAIModelBuilder) Validate() (Model, error) {
	if b.isReasoning && b.model.temperature != nil {
		return nil, fmt.Errorf("must not set temperature on a reasoning model")
	}
	if !b.isReasoning && b.model.reasoningEffort != nil {
		return nil, fmt.Errorf("must not set reasoning effort on a standard model")
	}
	return b.model, nil
}

func (b *OpenAIModelBuilder) WithTemperature(temp float64) *OpenAIModelBuilder {
	b.model.temperature = &temp
	return b
}

func (b *OpenAIModelBuilder) WithReasoningEffort(re ReasoningEffort) *OpenAIModelBuilder {
	b.model.reasoningEffort = &re
	return b
}

func (b *OpenAIModelBuilder) WithURL(url string) *OpenAIModelBuilder {
	b.model.url = url
	return b
}

func (b *OpenAIModelBuilder) WithTokens(input, output int) *OpenAIModelBuilder {
	b.model.maxInput = input
	b.model.maxOutput = output
	return b
}

func (b *OpenAIModelBuilder) WithHeader(key, val string) *OpenAIModelBuilder {
	b.model.extraHeaders[key] = val
	return b
}

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
		content := msg.Content
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

func (c *openAIModel) Respond(msgs []Message) ([]Message, Message, Usage, error) {
	openAIMsgs, err := messagesToOpenAI(msgs)
	if err != nil {
		return nil, Message{}, Usage{}, err
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
		return nil, Message{}, Usage{}, err
	}
	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer(body))
	if err != nil {
		return nil, Message{}, Usage{}, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.key))
	req.Header.Add("Content-Type", "application/json")
	for k, v := range c.extraHeaders {
		req.Header.Add(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, Message{}, Usage{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, Message{}, Usage{}, err
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
		return nil, Message{}, Usage(respTyped.Usage), fmt.Errorf("failed to parse response: %s", string(respBody))
	}
	content := respTyped.Choices[0].Message.Content
	return nil, Message{Role: AssistantRole, Content: content}, Usage(respTyped.Usage), nil
}
