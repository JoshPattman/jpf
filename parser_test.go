package jpf

import (
	"errors"
	"fmt"
	"testing"
)

func buildTestingSubstringRespDec() Parser[string] {
	return NewSubstringAfterParser(NewStringParser(), "::")
}

type RDCase[T comparable] struct {
	ID            string
	Build         func() Parser[T]
	Input         string
	Expected      T
	ExpectedError bool
}

func (testCase RDCase[T]) Name() string { return testCase.ID }

func (testCase RDCase[T]) Test() error {
	rd := testCase.Build()
	result, err := rd.ParseResponseText(testCase.Input)
	if testCase.ExpectedError {
		if err == nil {
			return fmt.Errorf("expected error, got none")
		}
	} else {
		if err != nil {
			return errors.Join(fmt.Errorf("expecting no error, got one"), err)
		}
		if result != testCase.Expected {
			return errors.Join(fmt.Errorf("expected and observed did not match. Expected %v but got %v", testCase.Expected, result))
		}
	}
	return nil
}

var RDCases = []TestCase{
	RDCase[TestStruct]{
		ID:            "json/no_response",
		Build:         NewJsonParser[TestStruct],
		Input:         "",
		ExpectedError: true,
	},
	RDCase[TestStruct]{
		ID:       "json/empty_valid_json",
		Build:    NewJsonParser[TestStruct],
		Input:    `{}`,
		Expected: TestStruct{},
	},
	RDCase[TestStruct]{
		ID:       "json/valid_json",
		Build:    NewJsonParser[TestStruct],
		Input:    `{"a":5, "b": "xyz"}`,
		Expected: TestStruct{A: 5, B: "xyz"},
	},
	RDCase[TestStruct]{
		ID:       "json/valid_json_within_decoration",
		Build:    NewJsonParser[TestStruct],
		Input:    "Here is my answer:\n```" + `{"a":5, "b": "xyz"}` + "```",
		Expected: TestStruct{A: 5, B: "xyz"},
	},
	RDCase[string]{
		ID:       "string/empty_string",
		Build:    NewStringParser,
		Input:    "",
		Expected: "",
	},
	RDCase[string]{
		ID:       "string/random_string",
		Build:    NewStringParser,
		Input:    "hdvdihiuhdibdb",
		Expected: "hdvdihiuhdibdb",
	},
	RDCase[string]{
		ID:       "string/random_string_with_whitespace",
		Build:    NewStringParser,
		Input:    "  hdvdihiuhdibdb  \n",
		Expected: "  hdvdihiuhdibdb  \n",
	},
	RDCase[string]{
		ID:       "substring/normal",
		Build:    buildTestingSubstringRespDec,
		Input:    "abcdefg",
		Expected: "abcdefg",
	},
	RDCase[string]{
		ID:       "substring/with_split",
		Build:    buildTestingSubstringRespDec,
		Input:    "abcdefg::1234",
		Expected: "1234",
	},
	RDCase[string]{
		ID:       "substring/with_two",
		Build:    buildTestingSubstringRespDec,
		Input:    "abcdefg::1234::xyz",
		Expected: "xyz",
	},
}

func TestParser(t *testing.T) {
	RunTests(t, RDCases)
}
