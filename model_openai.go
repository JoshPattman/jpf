package jpf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type openAIModel struct {
	key             string
	model           string
	maxInput        int
	maxOutput       int
	temperature     *float64
	reasoningEffort *ReasoningEffort
}

// NewReasoningOpenAIModel creates a new reasoning model (i.e. o1, o3, ...) from openai.
func NewReasoningOpenAIModel(key, modelName string, maxInput, maxOutput int, reasoningEffort ReasoningEffort) Model {
	return &openAIModel{
		key:             key,
		model:           modelName,
		maxInput:        maxInput,
		maxOutput:       maxOutput,
		reasoningEffort: &reasoningEffort,
	}
}

// NewStandardOpenAIModel creates a new standard model (i.e. gpt4o, gpt4.1, ...) from openai.
func NewStandardOpenAIModel(key, modelName string, maxInput, maxOutput int, temperature float64) Model {
	return &openAIModel{
		key:         key,
		model:       modelName,
		maxInput:    maxInput,
		maxOutput:   maxOutput,
		temperature: &temperature,
	}
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
	case ReasoningRole:
		return "system"
	default:
		panic("not a valid role")
	}
}

func messagesToOpenAI(msgs []Message) any {
	jsonMessages := make([]map[string]any, 0)
	for _, msg := range msgs {
		content := msg.Content
		if msg.Role == ReasoningRole {
			content = "The following information outlines some reasoning about the conversation up to this point:\n\n" + content
		}
		jsonMessages = append(jsonMessages, map[string]any{
			"role":    roleToOpenAI(msg.Role),
			"content": content,
		})
	}
	return jsonMessages
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
	bodyMap := map[string]any{
		"model":    c.model,
		"messages": messagesToOpenAI(msgs),
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
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return nil, Message{}, Usage{}, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.key))
	req.Header.Add("Content-Type", "application/json")
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
