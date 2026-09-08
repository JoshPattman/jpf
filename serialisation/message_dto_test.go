package serialisation

import (
	"encoding/json"
	"image"
	"testing"

	"github.com/JoshPattman/jpf"
)

func TestMessageDTORoundTrip(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, image.White)

	tests := []struct {
		name string
		msg  jpf.Message
	}{
		{"user", jpf.UserMessage{Content: "hello"}},
		{"user with image", jpf.UserMessage{Content: "look", Images: []jpf.ImageAttachment{{Source: img}}}},
		{"assistant plain", jpf.AssistantMessage{Content: "hi there"}},
		{"assistant with tool calls", jpf.AssistantMessage{
			Content: "calling",
			ToolCalls: []jpf.ToolCall{
				{ID: "c1", Tool: "search", Args: map[string]any{"q": "cats", "n": float64(3)}},
			},
		}},
		{"assistant with reasoning", jpf.AssistantMessage{
			Content: "calling",
			Reasoning: []jpf.OpaqueReasoningBlock{
				{FormatFamily: "anthropic/claude-x", Sig: "sig-1", Payload: "thinking text"},
			},
			ToolCalls: []jpf.ToolCall{
				{
					ID: "c1", Tool: "search", Args: map[string]any{"q": "cats"},
					Reasoning: []jpf.OpaqueReasoningBlock{
						{FormatFamily: "openai-responses/gpt-5", ID: "rs_1", Payload: "enc"},
					},
				},
			},
		}},
		{"developer", jpf.DeveloperMessage{Content: "be nice"}},
		{"system", jpf.SystemMessage{Content: "you are a bot"}},
		{"tool result", jpf.ToolResultMessage{CallID: "c1", Result: "done"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dto MessageDTO
			if err := dto.LoadMessage(tt.msg); err != nil {
				t.Fatalf("LoadMessage: %v", err)
			}

			data, err := json.Marshal(dto)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var loaded MessageDTO
			if err := json.Unmarshal(data, &loaded); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			got, err := loaded.ToMessage()
			if err != nil {
				t.Fatalf("ToMessage: %v", err)
			}

			// Images do not survive Eq (decoded image is a different value), so compare
			// them structurally and clear before the Eq check.
			if want, ok := tt.msg.(jpf.UserMessage); ok {
				gotUser := got.(jpf.UserMessage)
				if len(gotUser.Images) != len(want.Images) {
					t.Fatalf("image count: got %d want %d", len(gotUser.Images), len(want.Images))
				}
				for i, ia := range gotUser.Images {
					if ia.Source == nil || ia.Source.Bounds() != want.Images[i].Source.Bounds() {
						t.Fatalf("image %d did not round-trip: %+v", i, ia.Source)
					}
				}
				got = jpf.UserMessage{Content: gotUser.Content}
				tt.msg = jpf.UserMessage{Content: want.Content}
			}

			if !tt.msg.Eq(got) {
				t.Fatalf("round trip mismatch:\n got: %s\nwant: %s\njson: %s", got, tt.msg, data)
			}
		})
	}
}

func TestMessageDTOLoadMessageResetsState(t *testing.T) {
	dto := MessageDTO{Role: MessageRoleAssistant, ToolCalls: []jpf.ToolCall{{ID: "old"}}}
	if err := dto.LoadMessage(jpf.SystemMessage{Content: "fresh"}); err != nil {
		t.Fatalf("LoadMessage: %v", err)
	}
	if dto.ToolCalls != nil {
		t.Fatalf("expected ToolCalls to be cleared, got %+v", dto.ToolCalls)
	}
	if dto.Role != MessageRoleSystem || dto.Content != "fresh" {
		t.Fatalf("unexpected dto: %+v", dto)
	}
}

func TestMessageDTOToMessageUnknownRole(t *testing.T) {
	dto := MessageDTO{Role: "nonsense"}
	if _, err := dto.ToMessage(); err == nil {
		t.Fatal("expected an error for unknown role")
	}
}

func TestMessageDTOLoadMessageUnknownType(t *testing.T) {
	var dto MessageDTO
	if err := dto.LoadMessage(nil); err == nil {
		t.Fatal("expected an error for nil message")
	}
}
