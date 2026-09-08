package models

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/JoshPattman/jpf"
)

func TestWithStoreReasoningOpt(t *testing.T) {
	var settings apiModelSettings
	if settings.storeReasoning {
		t.Fatal("expected storeReasoning to default to false")
	}
	WithStoreReasoning()(&settings)
	if !settings.storeReasoning {
		t.Fatal("expected WithStoreReasoning to enable it")
	}
}

func TestFormatFamily(t *testing.T) {
	if got := formatFamily(Anthropic, "claude-x"); got != "anthropic/claude-x" {
		t.Fatalf("got %q", got)
	}
	if got := formatFamily(OpenAIResponses, "gpt-5"); got != "openai-responses/gpt-5" {
		t.Fatalf("got %q", got)
	}
	if got := formatFamily(Google, "gemini-2.5-pro"); got != "gemini/gemini-2.5-pro" {
		t.Fatalf("got %q", got)
	}
}

func TestReplayableReasoningDropsForeignBlocks(t *testing.T) {
	blocks := []jpf.OpaqueReasoningBlock{
		{FormatFamily: "anthropic/claude-x", Payload: "keep"},
		{FormatFamily: "openai-responses/gpt-5", Payload: "drop"},
		{FormatFamily: "anthropic/claude-y", Payload: "drop-other-model"},
	}
	got := replayableReasoning(blocks, "anthropic/claude-x")
	if len(got) != 1 || got[0].Payload != "keep" {
		t.Fatalf("got %+v", got)
	}
}

// --- OpenAI Chat Completions: unsupported -------------------------------------

func TestOpenAIChatRejectsStoreReasoning(t *testing.T) {
	m := &apiOpenAIModel{settings: apiModelSettings{storeReasoning: true}}
	_, err := m.Respond(context.Background(), []jpf.Message{jpf.UserMessage{Content: "hi"}})
	if err == nil || !strings.Contains(err.Error(), "StoreReasoning") {
		t.Fatalf("expected a storeReasoning error, got %v", err)
	}
}

// --- OpenAI Responses -------------------------------------------------------------

func TestOpenAIResponsesBodyIncludesEncryptedReasoning(t *testing.T) {
	m := &apiOpenAIResponsesModel{name: "gpt-5", settings: apiModelSettings{storeReasoning: true}}
	body, err := m.body(nil, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	inc, ok := body["include"].([]string)
	if !ok || len(inc) != 1 || inc[0] != "reasoning.encrypted_content" {
		t.Fatalf("got %+v", body["include"])
	}
	if body["store"] != false {
		t.Fatalf("expected store to stay false, got %+v", body["store"])
	}
}

func TestOpenAIResponsesBodyOmitsIncludeByDefault(t *testing.T) {
	m := &apiOpenAIResponsesModel{name: "gpt-5"}
	body, err := m.body(nil, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := body["include"]; ok {
		t.Fatalf("did not expect include: %+v", body)
	}
}

func TestOpenAIResponsesInputReplaysReasoning(t *testing.T) {
	m := &apiOpenAIResponsesModel{name: "gpt-5", settings: apiModelSettings{storeReasoning: true}}
	fam := m.formatFamily()
	items, err := m.input([]jpf.Message{
		jpf.AssistantMessage{
			Reasoning: []jpf.OpaqueReasoningBlock{{FormatFamily: fam, ID: "rs_turn", Payload: "enc-turn"}},
			ToolCalls: []jpf.ToolCall{{
				ID: "c1", Tool: "search", Args: map[string]any{"q": "x"},
				Reasoning: []jpf.OpaqueReasoningBlock{
					{FormatFamily: fam, ID: "rs_call", Payload: "enc-call"},
					{FormatFamily: "anthropic/other", Payload: "foreign"},
				},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// turn reasoning, then call reasoning, then the function call itself.
	if len(items) != 3 {
		t.Fatalf("got %d items: %+v", len(items), items)
	}
	first := items[0].(map[string]any)
	if first["type"] != "reasoning" || first["id"] != "rs_turn" || first["encrypted_content"] != "enc-turn" {
		t.Fatalf("got %+v", first)
	}
	second := items[1].(map[string]any)
	if second["type"] != "reasoning" || second["id"] != "rs_call" {
		t.Fatalf("got %+v", second)
	}
	third := items[2].(map[string]any)
	if third["type"] != "function_call" || third["call_id"] != "c1" {
		t.Fatalf("got %+v", third)
	}
}

// --- Anthropic ------------------------------------------------------------------

func TestAnthropicBodyEnablesThinking(t *testing.T) {
	m := &apiAnthropicModel{name: "claude-x", settings: apiModelSettings{storeReasoning: true}}
	temp := 0.7
	topP := 1
	m.settings.temperature = &temp
	m.settings.topP = &topP
	body := m.body("", nil, false, nil)

	think, ok := body["thinking"].(map[string]any)
	// storeReasoning without an explicit effort uses the medium tier.
	if !ok || think["type"] != "enabled" || think["budget_tokens"] != anthropicThinkingBudgetMedium {
		t.Fatalf("got %+v", body["thinking"])
	}
	if body["max_tokens"].(int) < anthropicThinkingBudgetMedium+anthropicDefaultMaxTokens {
		t.Fatalf("expected max_tokens to leave room for an answer, got %v", body["max_tokens"])
	}
	if _, ok := body["temperature"]; ok {
		t.Fatalf("temperature must be omitted with thinking on: %+v", body)
	}
	if _, ok := body["top_p"]; ok {
		t.Fatalf("top_p must be omitted with thinking on: %+v", body)
	}
}

func TestAnthropicThinkingBudgetFromReasoningEffort(t *testing.T) {
	cases := []struct {
		effort ReasoningEffort
		want   int
	}{
		{LowReasoning, anthropicThinkingBudgetLow},
		{MediumReasoning, anthropicThinkingBudgetMedium},
		{HighReasoning, anthropicThinkingBudgetHigh},
		{XHighReasoning, anthropicThinkingBudgetXHigh},
	}
	for _, c := range cases {
		eff := c.effort
		m := &apiAnthropicModel{name: "claude-x", settings: apiModelSettings{reasoning: &eff}}
		body := m.body("", nil, false, nil)
		think, ok := body["thinking"].(map[string]any)
		if !ok || think["budget_tokens"] != c.want {
			t.Fatalf("effort %d: got %+v", c.effort, body["thinking"])
		}
		if body["max_tokens"].(int) < c.want+anthropicDefaultMaxTokens {
			t.Fatalf("effort %d: max_tokens too small: %v", c.effort, body["max_tokens"])
		}
	}
}

func TestAnthropicNoneReasoningLeavesThinkingOff(t *testing.T) {
	eff := NoneReasoning
	m := &apiAnthropicModel{name: "claude-x", settings: apiModelSettings{reasoning: &eff}}
	body := m.body("", nil, false, nil)
	if _, ok := body["thinking"]; ok {
		t.Fatalf("did not expect thinking: %+v", body)
	}
}

func TestAnthropicStoreReasoningWithNoneReasoningErrors(t *testing.T) {
	eff := NoneReasoning
	m := &apiAnthropicModel{settings: apiModelSettings{storeReasoning: true, reasoning: &eff}}
	if err := m.validateNoUnusableArgs(jpf.ModelResponseKwargs{}); err == nil {
		t.Fatal("expected an error combining storeReasoning with NoneReasoning")
	}
}

func TestAnthropicCreateRequestAddsInterleavedThinkingBeta(t *testing.T) {
	m := &apiAnthropicModel{key: "k", settings: apiModelSettings{url: "http://x", storeReasoning: true}}
	req, err := m.createRequest(context.Background(), strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("anthropic-beta") != anthropicInterleavedThinkingBeta {
		t.Fatalf("got %q", req.Header.Get("anthropic-beta"))
	}
}

func TestAnthropicAssistantContentLeadsWithThinkingBlocks(t *testing.T) {
	m := &apiAnthropicModel{name: "claude-x", settings: apiModelSettings{storeReasoning: true}}
	fam := m.formatFamily()
	blocks := m.assistantContent(jpf.AssistantMessage{
		Content: "the answer",
		Reasoning: []jpf.OpaqueReasoningBlock{
			{FormatFamily: fam, Sig: "sig-1", Payload: "thought text"},
			{FormatFamily: fam, Payload: "redacted blob"},
			{FormatFamily: "gemini/other", Sig: "nope"},
		},
		ToolCalls: []jpf.ToolCall{{ID: "c1", Tool: "search", Args: map[string]any{}}},
	})
	if len(blocks) != 4 {
		t.Fatalf("got %d blocks: %+v", len(blocks), blocks)
	}
	if blocks[0]["type"] != "thinking" || blocks[0]["thinking"] != "thought text" || blocks[0]["signature"] != "sig-1" {
		t.Fatalf("got %+v", blocks[0])
	}
	if blocks[1]["type"] != "redacted_thinking" || blocks[1]["data"] != "redacted blob" {
		t.Fatalf("got %+v", blocks[1])
	}
	if blocks[2]["type"] != "text" || blocks[3]["type"] != "tool_use" {
		t.Fatalf("got %+v", blocks[2:])
	}
}

func TestAnthropicParseStreamResponseThinking(t *testing.T) {
	m := &apiAnthropicModel{name: "claude-x", settings: apiModelSettings{storeReasoning: true}}
	stream := strings.Join([]string{
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking"}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"step 1 "}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"step 2"}}`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig-abc"}}`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"hi"}}`,
	}, "\n")
	resp, _, err := m.parseStreamResponse(context.Background(), io.NopCloser(strings.NewReader(stream)), &recordingStreamer{})
	if err != nil {
		t.Fatal(err)
	}
	content, _, reasoning, err := m.extractOutput(resp.Content)
	if err != nil {
		t.Fatal(err)
	}
	if content != "hi" {
		t.Fatalf("got %q", content)
	}
	if len(reasoning) != 1 || reasoning[0].Payload != "step 1 step 2" || reasoning[0].Sig != "sig-abc" {
		t.Fatalf("got %+v", reasoning)
	}
}

// --- Gemini -------------------------------------------------------------------

func TestGeminiBodyEnablesThinkingConfig(t *testing.T) {
	m := &apiGeminiModel{name: "gemini-2.5-pro", settings: apiModelSettings{storeReasoning: true}}
	body, err := m.body("", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	gen := body["generationConfig"].(map[string]any)
	tc, ok := gen["thinkingConfig"].(map[string]any)
	if !ok || tc["includeThoughts"] != true {
		t.Fatalf("got %+v", gen["thinkingConfig"])
	}
	if _, ok := tc["thinkingBudget"]; ok {
		t.Fatalf("did not expect a budget without WithReasoningEffort: %+v", tc)
	}
}

func TestGeminiThinkingBudgetFromReasoningEffort(t *testing.T) {
	eff := HighReasoning
	m := &apiGeminiModel{name: "gemini-2.5-flash", settings: apiModelSettings{reasoning: &eff}}
	body, err := m.body("", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	tc := body["generationConfig"].(map[string]any)["thinkingConfig"].(map[string]any)
	if tc["thinkingBudget"] != geminiThinkingBudget(HighReasoning) {
		t.Fatalf("got %+v", tc)
	}
	if _, ok := tc["includeThoughts"]; ok {
		t.Fatalf("did not expect includeThoughts without storeReasoning: %+v", tc)
	}
}

func TestGeminiNoneReasoningLeavesThinkingConfigUnset(t *testing.T) {
	eff := NoneReasoning
	m := &apiGeminiModel{name: "gemini-2.5-flash", settings: apiModelSettings{reasoning: &eff}}
	body, err := m.body("", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if body["generationConfig"] != nil {
		if _, ok := body["generationConfig"].(map[string]any)["thinkingConfig"]; ok {
			t.Fatalf("did not expect thinkingConfig: %+v", body["generationConfig"])
		}
	}
}

func TestGeminiExtractOutputBindsThoughtSignatureToCall(t *testing.T) {
	m := &apiGeminiModel{name: "gemini-2.5-pro", settings: apiModelSettings{storeReasoning: true}}
	content, toolCalls, turnReasoning := m.extractOutput([]geminiResponsePart{
		{Text: "reasoning summary", Thought: true},
		{FunctionCall: &geminiResponseFunctionCall{Name: "search", Args: map[string]any{"q": "x"}}, ThoughtSignature: "sig-1"},
		{Text: "visible answer"},
	})
	if content != "visible answer" {
		t.Fatalf("got %q", content)
	}
	if len(toolCalls) != 1 || len(toolCalls[0].Reasoning) != 1 {
		t.Fatalf("got %+v", toolCalls)
	}
	if got := toolCalls[0].Reasoning[0]; got.Sig != "sig-1" || got.FormatFamily != "gemini/gemini-2.5-pro" {
		t.Fatalf("got %+v", got)
	}
	if len(turnReasoning) != 0 {
		t.Fatalf("did not expect turn reasoning: %+v", turnReasoning)
	}
}

func TestGeminiExtractOutputDropsReasoningWithoutStoreReasoning(t *testing.T) {
	m := &apiGeminiModel{name: "gemini-2.5-pro"}
	_, toolCalls, turnReasoning := m.extractOutput([]geminiResponsePart{
		{FunctionCall: &geminiResponseFunctionCall{Name: "search"}, ThoughtSignature: "sig-1"},
	})
	if len(toolCalls) != 1 || len(toolCalls[0].Reasoning) != 0 || len(turnReasoning) != 0 {
		t.Fatalf("got %+v / %+v", toolCalls, turnReasoning)
	}
}

func TestGeminiMessageContentReplaysThoughtSignature(t *testing.T) {
	m := &apiGeminiModel{name: "gemini-2.5-pro", settings: apiModelSettings{storeReasoning: true}}
	content, err := m.messageContent(jpf.AssistantMessage{
		ToolCalls: []jpf.ToolCall{{
			ID: "search", Tool: "search", Args: map[string]any{"q": "x"},
			Reasoning: []jpf.OpaqueReasoningBlock{
				{FormatFamily: m.formatFamily(), Sig: "sig-1"},
				{FormatFamily: "anthropic/other", Sig: "foreign"},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	parts := content.([]map[string]any)
	fc := parts[len(parts)-1]
	if fc["thoughtSignature"] != "sig-1" {
		t.Fatalf("got %+v", fc)
	}
}
