package jpf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NewGeminiModel creates a Model that uses the Google Gemini API.
// It requires an API key and model name, with optional configuration via variadic options.
func NewGeminiModel(key, modelName string, opts ...GeminiModelOpt) Model {
	model := &geminiModel{
		key:          key,
		model:        modelName,
		url:          fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", modelName),
		extraHeaders: make(map[string]string),
	}
	for _, o := range opts {
		o.applyGeminiModel(model)
	}
	return model
}

type GeminiModelOpt interface {
	applyGeminiModel(*geminiModel)
}

func (o WithTemperature) applyGeminiModel(m *geminiModel)     { m.temperature = &o.X }
func (o WithTopP) applyGeminiModel(m *geminiModel)            { m.topP = &o.X }
func (o WithVerbosity) applyGeminiModel(m *geminiModel)       { m.verbosity = &o.X }
func (o WithHTTPHeader) applyGeminiModel(m *geminiModel)      { m.extraHeaders[o.K] = o.V }
func (o WithMaxOutputTokens) applyGeminiModel(m *geminiModel) { m.maxOutput = o.X }
func (o WithURL) applyGeminiModel(m *geminiModel)             { m.url = o.X }
func (o WithTimeout) applyGeminiModel(m *geminiModel)         { m.timeout = o.X }

type geminiModel struct {
	key                string
	model              string
	url                string
	temperature        *float64
	topP               *int
	verbosity          *Verbosity
	maxOutput          int
	extraHeaders       map[string]string
	reasoningRole      Role
	reasoningTransform func(string) string
	systemRole         Role
	systemTransform    func(string) string
	timeout            time.Duration
}

func roleToGemini(role Role) (string, error) {
	switch role {
	case UserRole:
		return "user", nil
	case AssistantRole:
		return "assistant", nil
	default:
		return "", fmt.Errorf("gemini does not support that role: %s", role.String())
	}
}

// Converts internal messages to Gemini's format
func (m *geminiModel) messagesToGemini(msgs []Message) (string, []map[string]any, error) {
	parts := make([]map[string]any, 0)
	systemMessage := ""
	for i, msg := range msgs {
		role := msg.Role
		contentStr := msg.Content
		if role == ReasoningRole {
			role = m.reasoningRole
			if m.reasoningTransform != nil {
				contentStr = m.reasoningTransform(contentStr)
			}
		} else if role == SystemRole {
			role = m.systemRole
			if m.systemTransform != nil {
				contentStr = m.systemTransform(contentStr)
			}
		}
		if role == SystemRole {
			if i == 0 {
				if len(msg.Images) > 0 {
					return "", nil, errors.New("cannot attach images to system messages in gemini")
				}
				systemMessage = contentStr
				continue
			} else {
				return "", nil, errors.New("gemini only supports at most one system message at the start of the conversation")
			}
		}
		textPart := map[string]any{
			"text": contentStr,
		}
		allParts := []map[string]any{textPart}

		for _, img := range msg.Images {
			b64, err := img.ToBase64Encoded(false)
			if err != nil {
				return "", nil, errors.Join(errors.New("failed to encode image to base64"), err)
			}
			allParts = append(allParts, map[string]any{
				"inline_data": map[string]any{
					"mime_type": "image/png",
					"data":      b64,
				},
			})
		}
		gRole, err := roleToGemini(role)
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, map[string]any{
			"role":  gRole,
			"parts": allParts,
		})
	}
	return systemMessage, parts, nil
}

func (c *geminiModel) Respond(msgs []Message) (ModelResponse, error) {
	failedUsage := Usage{FailedCalls: 1}
	failedResp := ModelResponse{Usage: failedUsage}

	systemMessage, geminiMsgs, err := c.messagesToGemini(msgs)
	if err != nil {
		return failedResp, wrap(err, "could not convert messages to Gemini format")
	}

	bodyMap := map[string]any{
		"contents": geminiMsgs,
	}
	if systemMessage != "" {
		bodyMap["systemInstruction"] = map[string]any{
			"parts": []map[string]any{
				{
					"text": systemMessage,
				},
			},
		}
	}
	if c.temperature != nil {
		bodyMap["generationConfig"] = map[string]any{
			"temperature": *c.temperature,
		}
	}
	if c.topP != nil {
		if bodyMap["generationConfig"] == nil {
			bodyMap["generationConfig"] = map[string]any{}
		}
		bodyMap["generationConfig"].(map[string]any)["topP"] = *c.topP
	}
	if c.maxOutput != 0 {
		if bodyMap["generationConfig"] == nil {
			bodyMap["generationConfig"] = map[string]any{}
		}
		bodyMap["generationConfig"].(map[string]any)["maxOutputTokens"] = c.maxOutput
	}

	body, err := json.Marshal(bodyMap)
	if err != nil {
		return failedResp, wrap(err, "could not encode request body")
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s?key=%s", c.url, c.key), bytes.NewBuffer(body))
	if err != nil {
		return failedResp, wrap(err, "could not create request")
	}
	req.Header.Add("Content-Type", "application/json")
	for k, v := range c.extraHeaders {
		req.Header.Add(k, v)
	}

	if c.timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		defer cancel()
		req = req.WithContext(ctx)
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
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			InputTokens  int `json:"promptTokenCount"`
			OutputTokens int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}{}

	err = json.Unmarshal(respBody, &respTyped)
	usage := Usage{
		InputTokens:  respTyped.UsageMetadata.InputTokens,
		OutputTokens: respTyped.UsageMetadata.OutputTokens,
	}
	if err != nil {
		return ModelResponse{Usage: usage.Add(Usage{FailedCalls: 1})},
			wrap(err, "failed to parse response: %s", string(respBody))
	}

	if len(respTyped.Candidates) == 0 || len(respTyped.Candidates[0].Content.Parts) == 0 {
		return ModelResponse{Usage: usage.Add(Usage{FailedCalls: 1})},
			fmt.Errorf("response had no content: %s", string(respBody))
	}

	content := respTyped.Candidates[0].Content.Parts[0].Text
	return ModelResponse{
		PrimaryMessage: Message{Role: AssistantRole, Content: content},
		Usage:          usage.Add(Usage{SuccessfulCalls: 1}),
	}, nil
}
