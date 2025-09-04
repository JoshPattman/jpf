package jpf

import (
	"errors"
	"fmt"
	"testing"
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

type TestingModel struct {
	Responses map[string][]string
	NFails    int
}

func (t *TestingModel) Respond(msgs []Message) ([]Message, Message, Usage, error) {
	if t.NFails > 0 {
		t.NFails--
		return nil, Message{}, Usage{}, errors.New("deliberate fail")
	}
	var req string
	if len(msgs) > 0 {
		req = msgs[len(msgs)-1].Content
	}
	resps, ok := t.Responses[req]
	if !ok || len(resps) == 0 {
		return nil, Message{}, Usage{}, fmt.Errorf("no responses left for request '%s'", req)
	}
	resp, remaining := resps[0], resps[1:]
	t.Responses[req] = remaining
	return nil, Message{Role: AssistantRole, Content: resp}, Usage{}, nil
}

func (t *TestingModel) Tokens() (int, int) {
	return 100, 100
}
