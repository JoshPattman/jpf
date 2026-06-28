package utils

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/JoshPattman/jpf"
)

type TestCase interface {
	Name() string
	Test() error
}

func RunTests(t *testing.T, tests []TestCase) {
	for _, testCase := range tests {
		t.Run(testCase.Name(), func(t *testing.T) {
			err := testCase.Test()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

type TestStruct struct {
	A int    `json:"a"`
	B string `json:"b"`
}

var _ jpf.Model = &TestingModel{}

type TestingModel struct {
	Responses map[string][]string
	NFails    int
}

func (t *TestingModel) Respond(ctx context.Context, msgs []jpf.Message, opts ...jpf.ModelResponseOpt) (jpf.ModelResponse, error) {
	kwargs := jpf.GetModelResponseKwargs(opts...)
	if t.NFails > 0 {
		t.NFails--
		return jpf.ModelResponse{}, errors.New("deliberate fail")
	}
	var req string
	if len(msgs) > 0 {
		req = content(msgs[len(msgs)-1])
	}
	resps, ok := t.Responses[req]
	if !ok || len(resps) == 0 {
		return jpf.ModelResponse{}, fmt.Errorf("no responses left for request '%s'", req)
	}
	resp, remaining := resps[0], resps[1:]
	t.Responses[req] = remaining
	if kwargs.Streamer != nil {
		kwargs.Streamer.OnMessageBegin()
		kwargs.Streamer.OnMessageText(resp)
	}
	return jpf.ModelResponse{
		Message: jpf.AssistantMessage{Content: resp},
		Usage:   jpf.Usage{},
	}, nil
}

func content(msg jpf.Message) string {
	switch msg := msg.(type) {
	case jpf.UserMessage:
		return msg.Content
	case jpf.AssistantMessage:
		return msg.Content
	case jpf.DeveloperMessage:
		return msg.Content
	case jpf.SystemMessage:
		return msg.Content
	case jpf.ToolResultMessage:
		return msg.Result
	default:
		panic("unreachable")
	}
}

func (t *TestingModel) Tokens() (int, int) {
	return 100, 100
}

var _ jpf.Model = &SlowTestingModel{}

// SlowTestingModel is a testing model that simulates slow operations and respects context cancellation
type SlowTestingModel struct {
	Delay    time.Duration
	Response jpf.ModelResponse
}

func (s *SlowTestingModel) Respond(ctx context.Context, msgs []jpf.Message, opts ...jpf.ModelResponseOpt) (jpf.ModelResponse, error) {
	kwargs := jpf.GetModelResponseKwargs(opts...)
	// Use a timer that respects context cancellation
	timer := time.NewTimer(s.Delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		// Delay completed, return response
		if kwargs.Streamer != nil {
			kwargs.Streamer.OnMessageBegin()
			kwargs.Streamer.OnMessageText(s.Response.Message.Content)
		}
		return s.Response, nil
	case <-ctx.Done():
		// Context cancelled or timed out
		return jpf.ModelResponse{}, ctx.Err()
	}
}
