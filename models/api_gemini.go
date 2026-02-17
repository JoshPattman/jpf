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
	"github.com/JoshPattman/jpf/utils"
)

type apiGeminiModel struct {
	name     string
	key      string
	settings apiModelSettings
}

func (m *apiGeminiModel) Respond(ctx context.Context, msgs []jpf.Message) (jpf.ModelResponse, error) {
	err := m.validateNoUnusableArgs()
	if err != nil {
		return failedResponse(), utils.Wrap(err, "could not validate model setup")
	}
	body, err := m.createBodyData(msgs)
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

	var respTyped geminiStaticResponse
	var rawRespBytes []byte
	if m.settings.stream != nil {
		respTyped, rawRespBytes, err = m.parseStreamResponse(ctx, resp.Body)
	} else {
		respTyped, rawRespBytes, err = m.parseStaticResponse(ctx, resp.Body)
	}

	usage := jpf.Usage{
		InputTokens:  respTyped.UsageMetadata.InputTokens,
		OutputTokens: respTyped.UsageMetadata.OutputTokens,
	}
	if err != nil {
		return failedResponseAfter(usage), utils.Wrap(err, "failed to parse response: %s", string(rawRespBytes))
	}

	if len(respTyped.Candidates) == 0 || len(respTyped.Candidates[0].Content.Parts) == 0 {
		return failedResponseAfter(usage), fmt.Errorf("response had no content: %s", string(rawRespBytes))
	}

	content := respTyped.Candidates[0].Content.Parts[0].Text
	return jpf.ModelResponse{
		PrimaryMessage: jpf.Message{Role: jpf.AssistantRole, Content: content},
		Usage:          usage.Add(jpf.Usage{SuccessfulCalls: 1}),
	}, nil
}

func (m *apiGeminiModel) parseStaticResponse(ctx context.Context, respBody io.ReadCloser) (geminiStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()
	respData, err := io.ReadAll(respBody)
	if err != nil {
		return geminiStaticResponse{}, nil, utils.Wrap(err, "could not read response body")
	}
	respTyped := geminiStaticResponse{}
	err = json.Unmarshal(respData, &respTyped)
	if err != nil {
		return geminiStaticResponse{}, respData, utils.Wrap(err, "could not unmarshal response body")
	}
	return respTyped, respData, nil
}

func (m *apiGeminiModel) parseStreamResponse(ctx context.Context, respBody io.ReadCloser) (geminiStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()

	scanner := bufio.NewScanner(respBody)
	var fullText strings.Builder
	var inputTokens, outputTokens int

	if m.settings.stream != nil && m.settings.stream.onBegin != nil {
		m.settings.stream.onBegin()
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

		var chunk geminiStreamChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			return geminiStaticResponse{}, nil, utils.Wrap(err, "failed to unmarshal gemini stream chunk")
		}

		if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
			// concatenate all parts in this chunk
			for _, p := range chunk.Candidates[0].Content.Parts {
				fullText.WriteString(p.Text)
				if m.settings.stream != nil && m.settings.stream.onText != nil {
					m.settings.stream.onText(p.Text)
				}
			}
		}

		if chunk.UsageMetadata != nil {
			if chunk.UsageMetadata.InputTokens > 0 {
				inputTokens = chunk.UsageMetadata.InputTokens
			}
			if chunk.UsageMetadata.OutputTokens > 0 {
				outputTokens = chunk.UsageMetadata.OutputTokens
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return geminiStaticResponse{}, nil, utils.Wrap(err, "error reading gemini stream")
	}

	// Build a static-style response
	resp := geminiStaticResponse{
		Candidates: make([]struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		}, 1),
	}
	resp.Candidates[0].Content.Parts = make([]struct {
		Text string `json:"text"`
	}, 1)
	resp.Candidates[0].Content.Parts[0].Text = fullText.String()
	resp.UsageMetadata.InputTokens = inputTokens
	resp.UsageMetadata.OutputTokens = outputTokens

	return resp, nil, nil
}

func (m *apiGeminiModel) apiErrorResponse(resp *http.Response) (jpf.ModelResponse, error) {
	var geminiErr geminiErrorResponse
	respData, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respData, &geminiErr); err == nil && geminiErr.Error.Message != "" {
		return failedResponse(), &geminiError{
			msg:    geminiErr.Error.Message,
			status: geminiErr.Error.Status,
			code:   geminiErr.Error.Code,
		}
	}
	return failedResponse(), fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respData))
}

func (m *apiGeminiModel) createRequest(ctx context.Context, body io.Reader) (*http.Request, error) {
	var modelUrl, extraStreamParam string
	if m.settings.stream == nil {
		modelUrl = fmt.Sprintf("%s/%s:generateContent", m.settings.url, m.name)
	} else {
		modelUrl = fmt.Sprintf("%s/%s:streamGenerateContent", m.settings.url, m.name)
		extraStreamParam = "&alt=sse"
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("%s?key=%s%s", modelUrl, m.key, extraStreamParam), body)
	if err != nil {
		return nil, utils.Wrap(err, "could not create request")
	}
	req.Header.Add("Content-Type", "application/json")
	for k, v := range m.settings.headers {
		req.Header.Add(k, v)
	}
	return req.WithContext(ctx), nil
}

func (m *apiGeminiModel) createBodyData(msgs []jpf.Message) (io.Reader, error) {
	systemMessage, geminiMsgs, err := m.messages(msgs)
	if err != nil {
		return nil, utils.Wrap(err, "could not convert messages to Gemini format")
	}
	body, err := m.body(systemMessage, geminiMsgs)
	if err != nil {
		return nil, utils.Wrap(err, "could not create body")
	}
	bodyData, err := json.Marshal(body)
	if err != nil {
		return nil, utils.Wrap(err, "could not encode request body")
	}
	return bytes.NewReader(bodyData), nil
}

func (m *apiGeminiModel) messages(msgs []jpf.Message) (string, []any, error) {
	parts := make([]any, 0)
	systemMessage := ""
	for i, msg := range msgs {
		if msg.Role == jpf.SystemRole {
			if i != 0 {
				return "", nil, errors.New("gemini only supports at most one system message at the start of the conversation")
			}
			if len(msg.Images) > 0 {
				return "", nil, errors.New("cannot attach images to system messages in gemini")
			}
			systemMessage = msg.Content
		} else {
			role, err := m.messageRole(msg.Role)
			if err != nil {
				return "", nil, err
			}
			content, err := m.messageContent(msg)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, map[string]any{
				"role":  role,
				"parts": content,
			})
		}
	}
	return systemMessage, parts, nil
}

func (m *apiGeminiModel) messageRole(role jpf.Role) (string, error) {
	switch role {
	case jpf.UserRole:
		return "user", nil
	case jpf.AssistantRole:
		return "model", nil
	default:
		return "", errUnsupportedSetting("role", role.String())
	}
}

func (m *apiGeminiModel) messageContent(msg jpf.Message) (any, error) {
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
	return allParts, nil
}

func (m *apiGeminiModel) body(systemMessage string, msgs []any) (map[string]any, error) {
	body := map[string]any{
		"contents": msgs,
	}
	if systemMessage != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{
				{
					"text": systemMessage,
				},
			},
		}
	}
	if m.settings.temperature != nil {
		body["generationConfig"] = map[string]any{
			"temperature": *m.settings.temperature,
		}
	}
	if m.settings.topP != nil {
		if body["generationConfig"] == nil {
			body["generationConfig"] = map[string]any{}
		}
		body["generationConfig"].(map[string]any)["topP"] = *m.settings.topP
	}
	if m.settings.maxOutput != nil && *m.settings.maxOutput != 0 {
		if body["generationConfig"] == nil {
			body["generationConfig"] = map[string]any{}
		}
		body["generationConfig"].(map[string]any)["maxOutputTokens"] = *m.settings.maxOutput
	}
	return body, nil
}

func (m *apiGeminiModel) validateNoUnusableArgs() error {
	if m.settings.reasoning != nil {
		return errUnsupportedSetting("reasoning", m.settings.reasoning)
	}
	if m.settings.verbosity != nil {
		return errUnsupportedSetting("verbosity", m.settings.verbosity)
	}
	if m.settings.presencePenalty != nil {
		return errUnsupportedSetting("presencePenalty", m.settings.presencePenalty)
	}
	if m.settings.prediction != nil {
		return errUnsupportedSetting("prediction", m.settings.prediction)
	}
	if m.settings.jsonSchema != nil {
		return errUnsupportedSetting("jsonSchema", m.settings.jsonSchema)
	}
	return nil
}

type geminiStreamChunk struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata *struct {
		InputTokens  int `json:"promptTokenCount"`
		OutputTokens int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

type geminiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Status  string `json:"status"`
		Code    int    `json:"code"`
	} `json:"error"`
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

type geminiError struct {
	msg    string
	status string
	code   int
}

func (e *geminiError) Error() string {
	return fmt.Sprintf("gemini api returned an error: %d.%s - %s", e.code, e.status, e.msg)
}
