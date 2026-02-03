package jpf

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type MFCase[T any, U comparable] struct {
	ID            string
	Build         func() Pipeline[T, U]
	Input         T
	Expected      U
	ExpectedError bool
}

func (testCase MFCase[T, U]) Name() string { return testCase.ID }

func (testCase MFCase[T, U]) Test() error {
	mf := testCase.Build()
	result, _, err := mf.Call(context.Background(), testCase.Input)
	if testCase.ExpectedError {
		if err == nil {
			return errors.New("expected an error but got none")
		}
	} else {
		if err != nil {
			return errors.Join(errors.New("got error when expecting none"), err)
		}
		if result != testCase.Expected {
			return errors.Join(fmt.Errorf("expected and observed did not match. Expected %v but got %v", testCase.Expected, result))
		}
	}
	return nil
}

var MFCases = []TestCase{
	MFCase[string, string]{
		ID: "oneshot/nominal",
		Build: func() Pipeline[string, string] {
			enc := NewFixedEncoder("")
			dec := NewStringParser()
			model := &TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
			}}
			return NewOneShotPipeline(enc, dec, nil, model)
		},
		Input:    "ping",
		Expected: "pong",
	},
	MFCase[string, string]{
		ID: "oneshot/validate",
		Build: func() Pipeline[string, string] {
			enc := NewFixedEncoder("")
			dec := NewStringParser()
			val := &alwaysFailValidator[string, string]{}
			model := &TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
			}}
			return NewOneShotPipeline(enc, dec, val, model)
		},
		Input:         "ping",
		ExpectedError: true,
	},
	MFCase[string, TestStruct]{
		ID: "feedback/nominal",
		Build: func() Pipeline[string, TestStruct] {
			enc := NewFixedEncoder("")
			dec := NewJsonParser[TestStruct]()
			feedback := NewRawMessageFeedbackGenerator()
			model := &TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
				"response did not contain a json object\nllm produced an invalid response": {`{"a":5}`},
			}}
			return NewFeedbackPipeline(enc, dec, nil, feedback, model, SystemRole, 1)
		},
		Input:    "ping",
		Expected: TestStruct{A: 5},
	},
	MFCase[string, TestStruct]{
		ID: "feedback/validate",
		Build: func() Pipeline[string, TestStruct] {
			enc := NewFixedEncoder("")
			dec := NewJsonParser[TestStruct]()
			val := &alwaysFailValidator[string, TestStruct]{}
			feedback := NewRawMessageFeedbackGenerator()
			model := &TestingModel{Responses: map[string][]string{
				"ping": {`{"a":5}`},
				"response did not contain a json object\nllm produced an invalid response": {`{"a":5}`},
			}}
			return NewFeedbackPipeline(enc, dec, val, feedback, model, SystemRole, 1)
		},
		Input:         "ping",
		ExpectedError: true,
	},
	MFCase[string, TestStruct]{
		ID: "fallback/nominal",
		Build: func() Pipeline[string, TestStruct] {
			enc := NewFixedEncoder("")
			dec := NewJsonParser[TestStruct]()
			model1 := &TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
			}}
			model2 := &TestingModel{Responses: map[string][]string{
				"ping": {`{"a":5}`},
			}}
			return NewFallbackPipeline(enc, dec, nil, model1, model2)
		},
		Input:    "ping",
		Expected: TestStruct{A: 5},
	},
	MFCase[string, TestStruct]{
		ID: "fallback/fail",
		Build: func() Pipeline[string, TestStruct] {
			enc := NewFixedEncoder("")
			dec := NewJsonParser[TestStruct]()
			model1 := &TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
			}}
			model2 := &TestingModel{Responses: map[string][]string{
				"ping": {`{"a":"x"}`},
			}}
			return NewFallbackPipeline(enc, dec, nil, model1, model2)
		},
		Input:         "ping",
		ExpectedError: true,
	},
	MFCase[string, TestStruct]{
		ID: "fallback/validate",
		Build: func() Pipeline[string, TestStruct] {
			enc := NewFixedEncoder("")
			dec := NewJsonParser[TestStruct]()
			val := &alwaysFailValidator[string, TestStruct]{}
			model1 := &TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
			}}
			model2 := &TestingModel{Responses: map[string][]string{
				"ping": {`{"a":5}`},
			}}
			return NewFallbackPipeline(enc, dec, val, model1, model2)
		},
		Input:         "ping",
		ExpectedError: true,
	},
}

func TestPipeline(t *testing.T) {
	RunTests(t, MFCases)
}
