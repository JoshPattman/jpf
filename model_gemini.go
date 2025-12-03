package jpf

import (
	"bytes"
	"context"
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
}

func (c *geminiModel) Respond(ctx context.Context, msgs []Message) (ModelResponse, error) {
	failedUsage := Usage{FailedCalls: 1}
	failedResp := ModelResponse{Usage: failedUsage}

	body, err := c.createBodyData(msgs)
	if err != nil {
		return failedResp, wrap(err, "could not create request body")
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

	var respTyped geminiStaticResponse
	var rawRespBytes []byte
	respTyped, rawRespBytes, err = c.parseStaticResponse(ctx, resp.Body)

	usage := Usage{
		InputTokens:  respTyped.UsageMetadata.InputTokens,
		OutputTokens: respTyped.UsageMetadata.OutputTokens,
	}
	if err != nil {
		return ModelResponse{Usage: usage.Add(Usage{FailedCalls: 1})},
			wrap(err, "failed to parse response: %s", string(rawRespBytes))
	}

	if len(respTyped.Candidates) == 0 || len(respTyped.Candidates[0].Content.Parts) == 0 {
		return ModelResponse{Usage: usage.Add(Usage{FailedCalls: 1})},
			fmt.Errorf("response had no content: %s", string(rawRespBytes))
	}

	content := respTyped.Candidates[0].Content.Parts[0].Text
	return ModelResponse{
		PrimaryMessage: Message{Role: AssistantRole, Content: content},
		Usage:          usage.Add(Usage{SuccessfulCalls: 1}),
	}, nil
}

func (c *geminiModel) parseStaticResponse(ctx context.Context, respBody io.ReadCloser) (geminiStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()
	respData, err := io.ReadAll(respBody)
	if err != nil {
		return geminiStaticResponse{}, nil, wrap(err, "could not read response body")
	}
	respTyped := geminiStaticResponse{}
	err = json.Unmarshal(respData, &respTyped)
	if err != nil {
		return geminiStaticResponse{}, respData, wrap(err, "could not unmarshal response body")
	}
	return respTyped, respData, nil
}

func (c *geminiModel) createBodyData(msgs []Message) (io.Reader, error) {
	systemMessage, geminiMsgs, err := c.messagesToGemini(msgs)
	if err != nil {
		return nil, wrap(err, "could not convert messages to Gemini format")
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
		return nil, wrap(err, "could not encode request body")
	}
	return bytes.NewReader(body), nil
}

func (c *geminiModel) createRequest(ctx context.Context, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest("POST", fmt.Sprintf("%s?key=%s", c.url, c.key), body)
	if err != nil {
		return nil, wrap(err, "could not create request")
	}
	req.Header.Add("Content-Type", "application/json")
	for k, v := range c.extraHeaders {
		req.Header.Add(k, v)
	}
	return req.WithContext(ctx), nil
}

type geminiStaticResponse struct {
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
