package jpf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

type geminiModel struct {
	key          string
	model        string
	url          string
	temperature  *float64
	topP         *int
	verbosity    *Verbosity
	maxOutput    int
	extraHeaders map[string]string
}

func roleToGemini(role Role) string {
	switch role {
	case SystemRole:
		return "user" // Gemini does not have explicit "system", treat as user
	case UserRole:
		return "user"
	case AssistantRole:
		return "model"
	default:
		panic("not a valid role")
	}
}

// Converts internal messages to Gemini's format
func messagesToGemini(msgs []Message) ([]map[string]any, error) {
	parts := make([]map[string]any, 0)
	for _, msg := range msgs {
		// Gemini supports text and images as separate parts
		textPart := map[string]any{
			"text": msg.Content,
		}
		allParts := []map[string]any{textPart}

		for _, img := range msg.Images {
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

		parts = append(parts, map[string]any{
			"role":  roleToGemini(msg.Role),
			"parts": allParts,
		})
	}
	return parts, nil
}

func (c *geminiModel) Respond(msgs []Message) (ModelResponse, error) {
	failedUsage := Usage{FailedCalls: 1}
	failedResp := ModelResponse{Usage: failedUsage}

	geminiMsgs, err := messagesToGemini(msgs)
	if err != nil {
		return failedResp, wrap(err, "could not convert messages to Gemini format")
	}

	bodyMap := map[string]any{
		"contents": geminiMsgs,
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
