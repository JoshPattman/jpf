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
)

type apiOpenAIModel struct {
	name     string
	key      string
	settings apiModelSettings
}

func (m *apiOpenAIModel) Respond(ctx context.Context, msgs []jpf.Message) (jpf.ModelResponse, error) {
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

	var respTyped openAIAPIStaticResponse
	var rawRespBytes []byte
	if m.settings.stream != nil {
		respTyped, rawRespBytes, err = m.parseStreamResponse(ctx, resp.Body)
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
	return jpf.ModelResponse{
		PrimaryMessage: jpf.Message{Role: jpf.AssistantRole, Content: content},
		Usage:          usage.Add(jpf.Usage{SuccessfulCalls: 1}),
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

func (m *apiOpenAIModel) parseStreamResponse(ctx context.Context, respBody io.ReadCloser) (openAIAPIStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()
	scanner := bufio.NewScanner(respBody)
	var fullContent strings.Builder
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
			if m.settings.stream != nil && m.settings.stream.onText != nil {
				m.settings.stream.onText(content)
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

func (m *apiOpenAIModel) createBodyData(msgs []jpf.Message) (io.Reader, error) {
	apiMessages, err := m.messages(msgs)
	if err != nil {
		return nil, utils.Wrap(err, "could not convert messages to OpenAI format")
	}
	body, err := m.body(apiMessages)
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
		role, err := m.messageRole(msg.Role)
		if err != nil {
			return nil, err
		}
		content, err := m.messageContent(msg)
		if err != nil {
			return nil, err
		}
		jsonMessages = append(jsonMessages, openAIAPIMessage{
			Role:    role,
			Content: content,
		})
	}
	return jsonMessages, nil
}

func (m *apiOpenAIModel) messageRole(role jpf.Role) (string, error) {
	switch role {
	case jpf.SystemRole:
		return "system", nil
	case jpf.UserRole:
		return "user", nil
	case jpf.AssistantRole:
		return "assistant", nil
	case jpf.DeveloperRole:
		return "developer", nil
	default:
		return "", errUnsupportedSetting("role", role.String())
	}
}

func (m *apiOpenAIModel) messageContent(msg jpf.Message) (any, error) {
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
}

func (m *apiOpenAIModel) body(msgs []openAIAPIMessage) (map[string]any, error) {
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
	if m.settings.jsonSchema != nil {
		bodyMap["response_format"] = m.schema(m.settings.jsonSchema)
	}
	if m.settings.stream != nil {
		bodyMap["stream"] = true
		bodyMap["stream_options"] = map[string]any{"include_usage": true}
	}
	return bodyMap, nil
}

func (m *apiOpenAIModel) schema(schema any) any {
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "custom_schema",
			"schema": schema,
			"strict": true,
		},
	}
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

type openAIAPIMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
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
			Content string `json:"content"`
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
