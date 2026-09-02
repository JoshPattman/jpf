package jpf

import (
	"encoding/json"
	"image"
	"testing"
)

func TestMessageDTORoundTrip(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, image.White)

	tests := []struct {
		name string
		msg  Message
	}{
		{"user", UserMessage{Content: "hello"}},
		{"user with image", UserMessage{Content: "look", Images: []ImageAttachment{{Source: img}}}},
		{"assistant plain", AssistantMessage{Content: "hi there"}},
		{"assistant with tool calls", AssistantMessage{
			Content: "calling",
			ToolCalls: []ToolCall{
				{ID: "c1", Tool: "search", Args: map[string]any{"q": "cats", "n": float64(3)}},
			},
		}},
		{"developer", DeveloperMessage{Content: "be nice"}},
		{"system", SystemMessage{Content: "you are a bot"}},
		{"tool result", ToolResultMessage{CallID: "c1", Result: "done"}},
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
			if want, ok := tt.msg.(UserMessage); ok {
				gotUser := got.(UserMessage)
				if len(gotUser.Images) != len(want.Images) {
					t.Fatalf("image count: got %d want %d", len(gotUser.Images), len(want.Images))
				}
				for i, ia := range gotUser.Images {
					if ia.Source == nil || ia.Source.Bounds() != want.Images[i].Source.Bounds() {
						t.Fatalf("image %d did not round-trip: %+v", i, ia.Source)
					}
				}
				got = UserMessage{Content: gotUser.Content}
				tt.msg = UserMessage{Content: want.Content}
			}

			if !tt.msg.Eq(got) {
				t.Fatalf("round trip mismatch:\n got: %s\nwant: %s\njson: %s", got, tt.msg, data)
			}
		})
	}
}

func TestMessageDTOLoadMessageResetsState(t *testing.T) {
	dto := MessageDTO{Role: MessageRoleAssistant, ToolCalls: []ToolCall{{ID: "old"}}}
	if err := dto.LoadMessage(SystemMessage{Content: "fresh"}); err != nil {
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
