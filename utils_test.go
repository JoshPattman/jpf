package jpf

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
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

var _ Model = &TestingModel{}

type TestingModel struct {
	Responses map[string][]string
	NFails    int
}

func (t *TestingModel) Respond(ctx context.Context, msgs []Message) (ModelResponse, error) {
	if t.NFails > 0 {
		t.NFails--
		return ModelResponse{}, errors.New("deliberate fail")
	}
	var req string
	if len(msgs) > 0 {
		req = msgs[len(msgs)-1].Content
	}
	resps, ok := t.Responses[req]
	if !ok || len(resps) == 0 {
		return ModelResponse{}, fmt.Errorf("no responses left for request '%s'", req)
	}
	resp, remaining := resps[0], resps[1:]
	t.Responses[req] = remaining
	return ModelResponse{
		PrimaryMessage: Message{Role: AssistantRole, Content: resp},
		Usage:          Usage{},
	}, nil
}

func (t *TestingModel) Tokens() (int, int) {
	return 100, 100
}

var _ Model = &SlowTestingModel{}

// SlowTestingModel is a testing model that simulates slow operations and respects context cancellation
type SlowTestingModel struct {
	Delay    time.Duration
	Response ModelResponse
}

func (s *SlowTestingModel) Respond(ctx context.Context, msgs []Message) (ModelResponse, error) {
	// Use a timer that respects context cancellation
	timer := time.NewTimer(s.Delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		// Delay completed, return response
		return s.Response, nil
	case <-ctx.Done():
		// Context cancelled or timed out
		return ModelResponse{}, ctx.Err()
	}
}
