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
	"github.com/invopop/jsonschema"
)

// anthropicAPIVersion is the value of the required `anthropic-version` header.
// See https://docs.anthropic.com/en/api/versioning
const anthropicAPIVersion = "2023-06-01"

// anthropicDefaultMaxTokens is used when no WithMaxOutput option is set, since
// Anthropic (unlike OpenAI/Gemini) requires `max_tokens` on every request.
const anthropicDefaultMaxTokens = 4096

// apiAnthropicModel talks to Anthropic's Messages API (https://api.anthropic.com/v1/messages).
type apiAnthropicModel struct {
	name     string
	key      string
	settings apiModelSettings
}

func (m *apiAnthropicModel) Respond(ctx context.Context, msgs []jpf.Message, opts ...jpf.ModelResponseOpt) (jpf.ModelResponse, error) {
	kwargs := jpf.GetModelResponseKwargs(opts...)
	err := m.validateNoUnusableArgs(kwargs)
	if err != nil {
		return failedResponse(), utils.Wrap(err, "could not validate model setup")
	}
	isStreamed := kwargs.Streamer != nil
	body, err := m.createBodyData(msgs, isStreamed, kwargs.OutputFormat, kwargs.ToolSchemas)
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

	var respTyped anthropicStaticResponse
	var rawRespBytes []byte
	if kwargs.Streamer != nil {
		respTyped, rawRespBytes, err = m.parseStreamResponse(ctx, resp.Body, kwargs.Streamer)
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
	if respTyped.Type == "error" {
		return failedResponseAfter(usage), &anthropicError{
			respTyped.Error.Type,
			respTyped.Error.Message,
		}
	}

	content, toolCalls, err := m.extractOutput(respTyped.Content)
	if err != nil {
		return failedResponseAfter(usage), utils.Wrap(err, "could not extract output: %s", string(rawRespBytes))
	}

	return jpf.ModelResponse{
		Message: jpf.AssistantMessage{Content: content, ToolCalls: toolCalls},
		Usage:   usage.Add(jpf.Usage{SuccessfulCalls: 1}),
	}, nil
}

func (m *apiAnthropicModel) extractOutput(blocks []anthropicContentBlock) (string, []jpf.ToolCall, error) {
	var content strings.Builder
	toolCalls := make([]jpf.ToolCall, 0)
	for _, block := range blocks {
		switch block.Type {
		case "text":
			content.WriteString(block.Text)
		case "tool_use":
			args := make(map[string]any)
			if len(block.Input) > 0 {
				if err := json.Unmarshal(block.Input, &args); err != nil {
					return "", nil, utils.Wrap(err, "could not decode tool arguments")
				}
			}
			toolCalls = append(toolCalls, jpf.ToolCall{
				ID:   block.ID,
				Tool: block.Name,
				Args: args,
			})
		}
	}
	return content.String(), toolCalls, nil
}

func (m *apiAnthropicModel) parseStaticResponse(ctx context.Context, respBody io.ReadCloser) (anthropicStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()
	respTyped := anthropicStaticResponse{}
	respData, err := io.ReadAll(respBody)
	if err != nil {
		return anthropicStaticResponse{}, respData, utils.Wrap(err, "could not read response body")
	}
	err = json.Unmarshal(respData, &respTyped)
	if err != nil {
		return anthropicStaticResponse{}, respData, utils.Wrap(err, "could not unmarshal response body: %s", string(respData))
	}
	return respTyped, respData, nil
}

func (m *apiAnthropicModel) parseStreamResponse(ctx context.Context, respBody io.ReadCloser, streamer jpf.ModelStreamer) (anthropicStaticResponse, []byte, error) {
	go func() {
		<-ctx.Done()
		respBody.Close()
	}()
	scanner := bufio.NewScanner(respBody)

	streamer.OnMessageBegin()

	blocks := make(map[int]*anthropicStreamedBlock)
	order := make([]int, 0)
	var inputTokens, outputTokens int

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		data := line[6:]

		var event anthropicStreamEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return anthropicStaticResponse{}, nil, utils.Wrap(err, "failed to unmarshal stream event")
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				inputTokens = event.Message.Usage.InputTokens
			}
		case "content_block_start":
			if event.ContentBlock != nil {
				acc := &anthropicStreamedBlock{Type: event.ContentBlock.Type, ID: event.ContentBlock.ID, Name: event.ContentBlock.Name}
				acc.Text.WriteString(event.ContentBlock.Text)
				blocks[event.Index] = acc
				order = append(order, event.Index)
			}
		case "content_block_delta":
			acc, ok := blocks[event.Index]
			if !ok {
				acc = &anthropicStreamedBlock{}
				blocks[event.Index] = acc
				order = append(order, event.Index)
			}
			if event.Delta != nil {
				switch event.Delta.Type {
				case "text_delta":
					acc.Text.WriteString(event.Delta.Text)
					streamer.OnMessageText(event.Delta.Text)
				case "input_json_delta":
					acc.Input.WriteString(event.Delta.PartialJSON)
				}
			}
		case "message_delta":
			if event.Usage != nil {
				outputTokens = event.Usage.OutputTokens
			}
		case "error":
			if event.Error != nil {
				return anthropicStaticResponse{}, nil, &anthropicError{event.Error.Type, event.Error.Message}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return anthropicStaticResponse{}, nil, utils.Wrap(err, "error reading stream")
	}

	content := make([]anthropicContentBlock, 0, len(order))
	for _, idx := range order {
		acc := blocks[idx]
		block := anthropicContentBlock{Type: acc.Type, Text: acc.Text.String(), ID: acc.ID, Name: acc.Name}
		if acc.Input.Len() > 0 {
			block.Input = json.RawMessage(acc.Input.String())
		}
		content = append(content, block)
	}

	return anthropicStaticResponse{
		Content: content,
		Usage:   anthropicUsage{InputTokens: inputTokens, OutputTokens: outputTokens},
	}, nil, nil
}

func (m *apiAnthropicModel) apiErrorResponse(resp *http.Response) (jpf.ModelResponse, error) {
	var errResp anthropicErrorResponse
	respData, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respData, &errResp); err != nil {
		return failedResponse(), utils.Wrap(fmt.Errorf("http status %d", resp.StatusCode), "request failed: %s", string(respData))
	}
	return failedResponse(), &anthropicError{
		errResp.Error.Type,
		errResp.Error.Message,
	}
}

func (m *apiAnthropicModel) createRequest(ctx context.Context, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest("POST", m.settings.url, body)
	if err != nil {
		return nil, utils.Wrap(err, "could not create request")
	}
	req.Header.Add("x-api-key", m.key)
	req.Header.Add("anthropic-version", anthropicAPIVersion)
	req.Header.Add("Content-Type", "application/json")
	for k, v := range m.settings.headers {
		req.Header.Add(k, v)
	}
	return req.WithContext(ctx), nil
}

func (m *apiAnthropicModel) createBodyData(msgs []jpf.Message, isStreamed bool, outputFormat any, toolSchemas []jpf.ToolSchema) (io.Reader, error) {
	system, apiMsgs, err := m.messages(msgs)
	if err != nil {
		return nil, utils.Wrap(err, "could not convert messages to Anthropic format")
	}
	if outputFormat != nil {
		instruction, err := m.outputFormatInstruction(outputFormat)
		if err != nil {
			return nil, utils.Wrap(err, "failed to build output format instruction")
		}
		if system != "" {
			system += "\n\n" + instruction
		} else {
			system = instruction
		}
	}
	body := m.body(system, apiMsgs, isStreamed, toolSchemas)

	bodyData, err := json.Marshal(body)
	if err != nil {
		return nil, utils.Wrap(err, "could not encode body")
	}
	return bytes.NewReader(bodyData), nil
}

// outputFormatInstruction builds a system-prompt instruction that asks Claude
// to reply with a JSON object matching the given type's schema. The Messages
// API has no stable, non-beta native structured-output field (unlike OpenAI's
// response_format or Gemini's responseSchema), so this relies on prompting
// instead - callers are expected to extract the JSON object from the response
// text (e.g. via parsers.NewJson), which only requires a JSON object to be
// present in the text, not the entire response to be pure JSON.
func (m *apiAnthropicModel) outputFormatInstruction(obj any) (string, error) {
	schema, err := m.schema(obj)
	if err != nil {
		return "", err
	}
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"Respond with a single JSON object that strictly conforms to the following JSON schema, and include no other text outside of the JSON object:\n%s",
		string(schemaBytes),
	), nil
}

func (m *apiAnthropicModel) schema(obj any) (any, error) {
	r := &jsonschema.Reflector{
		BaseSchemaID:   "Anonymous",
		Anonymous:      true,
		DoNotReference: true,
	}
	s := r.Reflect(obj)
	schemaBs, err := s.MarshalJSON()
	if err != nil {
		return nil, err
	}
	schema := make(map[string]any)
	err = json.Unmarshal(schemaBs, &schema)
	if err != nil {
		return nil, err
	}
	return schema, nil
}

// messages converts jpf messages into Anthropic's system string + alternating
// user/assistant message list. System and developer messages have no
// per-message equivalent in the Anthropic API, so their content is collected
// into the single top-level `system` field. Adjacent messages that resolve to
// the same role (e.g. consecutive tool results) are merged into one message,
// since Anthropic requires the message list to strictly alternate roles.
func (m *apiAnthropicModel) messages(msgs []jpf.Message) (string, []anthropicMessage, error) {
	systemParts := make([]string, 0)
	merged := make([]anthropicMessage, 0, len(msgs))

	appendBlocks := func(role string, blocks []map[string]any) {
		if len(blocks) == 0 {
			return
		}
		if len(merged) > 0 && merged[len(merged)-1].Role == role {
			merged[len(merged)-1].Content = append(merged[len(merged)-1].Content, blocks...)
			return
		}
		merged = append(merged, anthropicMessage{Role: role, Content: blocks})
	}

	for _, msg := range msgs {
		switch msg := msg.(type) {
		case jpf.SystemMessage:
			if msg.Content != "" {
				systemParts = append(systemParts, msg.Content)
			}
		case jpf.DeveloperMessage:
			if msg.Content != "" {
				systemParts = append(systemParts, msg.Content)
			}
		case jpf.UserMessage:
			blocks, err := m.userContent(msg)
			if err != nil {
				return "", nil, err
			}
			appendBlocks("user", blocks)
		case jpf.AssistantMessage:
			appendBlocks("assistant", m.assistantContent(msg))
		case jpf.ToolResultMessage:
			appendBlocks("user", []map[string]any{{
				"type":        "tool_result",
				"tool_use_id": msg.CallID,
				"content":     msg.Result,
			}})
		default:
			return "", nil, errUnsupportedSetting("role", fmt.Sprintf("%T", msg))
		}
	}

	return strings.Join(systemParts, "\n\n"), merged, nil
}

func (m *apiAnthropicModel) userContent(msg jpf.UserMessage) ([]map[string]any, error) {
	blocks := make([]map[string]any, 0, 1+len(msg.Images))
	if msg.Content != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": msg.Content})
	}
	for _, img := range msg.Images {
		block, err := m.imageBlock(img)
		if err != nil {
			return nil, errors.Join(errors.New("failed to encode image to base64"), err)
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func (m *apiAnthropicModel) imageBlock(img jpf.ImageAttachment) (map[string]any, error) {
	dataURI, err := img.ToBase64Encoded(true)
	if err != nil {
		return nil, err
	}
	idx := strings.Index(dataURI, ",")
	if idx == -1 {
		return nil, fmt.Errorf("could not parse encoded image data")
	}
	return map[string]any{
		"type": "image",
		"source": map[string]any{
			"type":       "base64",
			"media_type": "image/jpeg",
			"data":       dataURI[idx+1:],
		},
	}, nil
}

func (m *apiAnthropicModel) assistantContent(msg jpf.AssistantMessage) []map[string]any {
	blocks := make([]map[string]any, 0, 1+len(msg.ToolCalls))
	if msg.Content != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": msg.Content})
	}
	for _, tc := range msg.ToolCalls {
		blocks = append(blocks, map[string]any{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Tool,
			"input": tc.Args,
		})
	}
	return blocks
}

func (m *apiAnthropicModel) body(system string, msgs []anthropicMessage, isStreamed bool, toolSchemas []jpf.ToolSchema) map[string]any {
	maxTokens := anthropicDefaultMaxTokens
	if m.settings.maxOutput != nil {
		maxTokens = *m.settings.maxOutput
	}
	bodyMap := map[string]any{
		"model":      m.name,
		"max_tokens": maxTokens,
		"messages":   msgs,
	}
	if system != "" {
		bodyMap["system"] = system
	}
	if m.settings.temperature != nil {
		bodyMap["temperature"] = *m.settings.temperature
	}
	if m.settings.topP != nil {
		bodyMap["top_p"] = *m.settings.topP
	}
	if isStreamed {
		bodyMap["stream"] = true
	}
	if len(toolSchemas) > 0 {
		bodyMap["tools"] = m.tools(toolSchemas)
		bodyMap["tool_choice"] = map[string]any{"type": "auto"}
	}
	return bodyMap
}

func (m *apiAnthropicModel) tools(toolSchemas []jpf.ToolSchema) []any {
	anthropicTools := make([]any, 0, len(toolSchemas))
	for _, tool := range toolSchemas {
		props := map[string]any{}
		required := []string{}

		for _, arg := range tool.Args {
			t := "string"

			switch arg.Type {
			case jpf.ToolArgString:
				t = "string"
			case jpf.ToolArgInt:
				t = "integer"
			case jpf.ToolArgFloat:
				t = "number"
			default:
				panic("unreachable")
			}

			props[arg.Name] = map[string]any{
				"type":        t,
				"description": arg.Description,
			}

			if arg.Required {
				required = append(required, arg.Name)
			}
		}

		inputSchema := map[string]any{
			"type":       "object",
			"properties": props,
		}
		if len(required) > 0 {
			inputSchema["required"] = required
		}

		anthropicTools = append(anthropicTools, map[string]any{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": inputSchema,
		})
	}
	return anthropicTools
}

func (m *apiAnthropicModel) validateNoUnusableArgs(kwargs jpf.ModelResponseKwargs) error {
	if m.settings.reasoning != nil {
		return errUnsupportedSetting("reasoning", *m.settings.reasoning)
	}
	if m.settings.verbosity != nil {
		return errUnsupportedSetting("verbosity", *m.settings.verbosity)
	}
	if m.settings.presencePenalty != nil {
		return errUnsupportedSetting("presencePenalty", *m.settings.presencePenalty)
	}
	if m.settings.prediction != nil {
		return errUnsupportedSetting("prediction", *m.settings.prediction)
	}
	return nil
}

type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []map[string]any `json:"content"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// anthropicStreamedBlock accumulates a single content block's deltas while
// parsing a streamed response.
type anthropicStreamedBlock struct {
	Type  string
	Text  strings.Builder
	ID    string
	Name  string
	Input strings.Builder
}

type anthropicStaticResponse struct {
	Type    string                  `json:"type"`
	Content []anthropicContentBlock `json:"content"`
	Usage   anthropicUsage          `json:"usage"`
	Error   struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type anthropicStreamEvent struct {
	Type    string `json:"type"`
	Index   int    `json:"index"`
	Message *struct {
		Usage anthropicUsage `json:"usage"`
	} `json:"message"`
	ContentBlock *anthropicContentBlock `json:"content_block"`
	Delta        *struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
	Usage *anthropicUsage `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type anthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type anthropicError struct {
	errType string
	msg     string
}

func (e *anthropicError) Error() string {
	return fmt.Sprintf("anthropic api returned an error: %s - %s", e.errType, e.msg)
}
