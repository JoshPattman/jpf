package jpf

import (
	"errors"
	"fmt"
	"testing"
)

type MFCase[T any, U comparable] struct {
	ID            string
	Build         func() MapFunc[T, U]
	Input         T
	Expected      U
	ExpectedError bool
}

func (testCase MFCase[T, U]) Name() string { return testCase.ID }

func (testCase MFCase[T, U]) Test() error {
	mf := testCase.Build()
	result, _, err := mf.Call(testCase.Input)
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
		Build: func() MapFunc[string, string] {
			enc := NewRawStringMessageEncoder("")
			dec := NewRawStringResponseDecoder()
			model := &TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
			}}
			return NewOneShotMapFunc(enc, dec, model)
		},
		Input:    "ping",
		Expected: "pong",
	},
	MFCase[string, TestStruct]{
		ID: "feedback/nominal",
		Build: func() MapFunc[string, TestStruct] {
			enc := NewRawStringMessageEncoder("")
			dec := NewJsonResponseDecoder[TestStruct]()
			feedback := NewRawMessageFeedbackGenerator()
			model := &TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
				"response did not contain a json object\nllm produced an invalid response": {`{"a":5}`},
			}}
			return NewFeedbackMapFunc(enc, dec, feedback, model, SystemRole, 1)
		},
		Input:    "ping",
		Expected: TestStruct{A: 5},
	},
}

func TestMapFunc(t *testing.T) {
	RunTests(t, MFCases)
}
