package pipelines

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/encoders"
	"github.com/JoshPattman/jpf/feedbacks"
	"github.com/JoshPattman/jpf/parsers"
	"github.com/JoshPattman/jpf/utils"
)

type alwaysFailValidator[T, U any] struct{}

func (*alwaysFailValidator[T, U]) ValidateParsedResponse(T, U) error {
	return errors.Join(errors.New("expected fail"), jpf.ErrInvalidResponse)
}

type MFCase[T any, U comparable] struct {
	ID            string
	Build         func() jpf.Pipeline[T, U]
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

var MFCases = []utils.TestCase{
	MFCase[string, string]{
		ID: "oneshot/nominal",
		Build: func() jpf.Pipeline[string, string] {
			enc := encoders.NewFixedEncoder("")
			dec := parsers.NewStringParser()
			model := &utils.TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
			}}
			return NewOneShotPipeline(enc, dec, nil, model)
		},
		Input:    "ping",
		Expected: "pong",
	},
	MFCase[string, string]{
		ID: "oneshot/validate",
		Build: func() jpf.Pipeline[string, string] {
			enc := encoders.NewFixedEncoder("")
			dec := parsers.NewStringParser()
			val := &alwaysFailValidator[string, string]{}
			model := &utils.TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
			}}
			return NewOneShotPipeline(enc, dec, val, model)
		},
		Input:         "ping",
		ExpectedError: true,
	},
	MFCase[string, utils.TestStruct]{
		ID: "feedback/nominal",
		Build: func() jpf.Pipeline[string, utils.TestStruct] {
			enc := encoders.NewFixedEncoder("")
			dec := parsers.NewJsonParser[utils.TestStruct]()
			feedback := feedbacks.NewRawMessageFeedbackGenerator()
			model := &utils.TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
				"response did not contain a json object\nllm produced an invalid response": {`{"a":5}`},
			}}
			return NewFeedbackPipeline(enc, dec, nil, feedback, model, jpf.SystemRole, 1)
		},
		Input:    "ping",
		Expected: utils.TestStruct{A: 5},
	},
	MFCase[string, utils.TestStruct]{
		ID: "feedback/validate",
		Build: func() jpf.Pipeline[string, utils.TestStruct] {
			enc := encoders.NewFixedEncoder("")
			dec := parsers.NewJsonParser[utils.TestStruct]()
			val := &alwaysFailValidator[string, utils.TestStruct]{}
			feedback := feedbacks.NewRawMessageFeedbackGenerator()
			model := &utils.TestingModel{Responses: map[string][]string{
				"ping": {`{"a":5}`},
				"response did not contain a json object\nllm produced an invalid response": {`{"a":5}`},
			}}
			return NewFeedbackPipeline(enc, dec, val, feedback, model, jpf.SystemRole, 1)
		},
		Input:         "ping",
		ExpectedError: true,
	},
	MFCase[string, utils.TestStruct]{
		ID: "fallback/nominal",
		Build: func() jpf.Pipeline[string, utils.TestStruct] {
			enc := encoders.NewFixedEncoder("")
			dec := parsers.NewJsonParser[utils.TestStruct]()
			model1 := &utils.TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
			}}
			model2 := &utils.TestingModel{Responses: map[string][]string{
				"ping": {`{"a":5}`},
			}}
			return NewFallbackPipeline(enc, dec, nil, model1, model2)
		},
		Input:    "ping",
		Expected: utils.TestStruct{A: 5},
	},
	MFCase[string, utils.TestStruct]{
		ID: "fallback/fail",
		Build: func() jpf.Pipeline[string, utils.TestStruct] {
			enc := encoders.NewFixedEncoder("")
			dec := parsers.NewJsonParser[utils.TestStruct]()
			model1 := &utils.TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
			}}
			model2 := &utils.TestingModel{Responses: map[string][]string{
				"ping": {`{"a":"x"}`},
			}}
			return NewFallbackPipeline(enc, dec, nil, model1, model2)
		},
		Input:         "ping",
		ExpectedError: true,
	},
	MFCase[string, utils.TestStruct]{
		ID: "fallback/validate",
		Build: func() jpf.Pipeline[string, utils.TestStruct] {
			enc := encoders.NewFixedEncoder("")
			dec := parsers.NewJsonParser[utils.TestStruct]()
			val := &alwaysFailValidator[string, utils.TestStruct]{}
			model1 := &utils.TestingModel{Responses: map[string][]string{
				"ping": {"pong"},
			}}
			model2 := &utils.TestingModel{Responses: map[string][]string{
				"ping": {`{"a":5}`},
			}}
			return NewFallbackPipeline(enc, dec, val, model1, model2)
		},
		Input:         "ping",
		ExpectedError: true,
	},
}

func TestPipeline(t *testing.T) {
	utils.RunTests(t, MFCases)
}
