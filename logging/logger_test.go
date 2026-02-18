package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/JoshPattman/jpf"
)

type collectSlog struct {
	msg  string
	args []any
}

func (c *collectSlog) Slog(msg string, args ...any) {
	c.msg = msg
	c.args = args
}

func TestSlogLogger(t *testing.T) {
	slog := &collectSlog{}
	logger := NewSlog(slog.Slog, true)

	info := jpf.ModelLoggingInfo{
		Messages: []jpf.Message{
			{
				Role:    jpf.UserRole,
				Content: "Hi",
			},
		},
	}

	logger.ModelLog(info)

	expectedArgs := []any{
		"input_tokens", info.Usage.InputTokens,
		"output_tokens", info.Usage.OutputTokens,
		"time_taken", info.Duration.String(),
	}
	if info.Err != nil {
		expectedArgs = append(expectedArgs, "error", info.Err.Error())
	}
	expectedArgs = append(
		expectedArgs,
		"input_messages", info.Messages,
		"output_aux_messages", info.ResponseAuxMessages,
		"output_final_message", info.ResponseFinalMessage,
	)
	if slog.msg != "model_call" {
		t.Fatalf("expected model_call message, got %s", slog.msg)
	}
	cmp := func(a, b any) bool { return fmt.Sprint(a) == fmt.Sprint(b) }
	if !slices.EqualFunc(slog.args, expectedArgs, cmp) {
		t.Fatalf("expected %v args, got %v", expectedArgs, slog.args)
	}
}

func TestJsonLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewJson(buf)

	info := jpf.ModelLoggingInfo{
		Messages: []jpf.Message{
			{
				Role:    jpf.UserRole,
				Content: "Hi",
			},
		},
		Usage: jpf.Usage{InputTokens: 5, OutputTokens: 7},
	}

	err := logger.ModelLog(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	dec := json.NewDecoder(buf)
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if got["duration"] == nil {
		t.Errorf("missing duration field")
	}
	if got["usage"] == nil {
		t.Errorf("missing usage field")
	}
	if got["messages"] == nil {
		t.Errorf("missing messages field")
	}
	if got["final_response"] == nil {
		t.Errorf("missing final_response field")
	}

	usage, ok := got["usage"].(map[string]any)
	if !ok {
		t.Errorf("usage field is not a map: %T", got["usage"])
	} else {
		if usage["input_tokens"] != float64(5) {
			t.Errorf("input_tokens: want 5, got %v", usage["input_tokens"])
		}
		if usage["output_tokens"] != float64(7) {
			t.Errorf("output_tokens: want 7, got %v", usage["output_tokens"])
		}
	}

	msgs, ok := got["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Errorf("messages: want 1 message, got %v", got["messages"])
	} else {
		msg, ok := msgs[0].(map[string]any)
		if !ok {
			t.Errorf("message is not a map: %T", msgs[0])
		} else {
			if msg["role"] != jpf.UserRole.String() {
				t.Errorf("role: want %q, got %v", jpf.UserRole.String(), msg["role"])
			}
			if msg["content"] != "Hi" {
				t.Errorf("content: want 'Hi', got %v", msg["content"])
			}
		}
	}
}
