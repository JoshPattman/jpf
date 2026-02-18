package parsers

import (
	"errors"
	"fmt"
	"testing"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
)

func buildTestingSubstringRespDec() jpf.Parser[string] {
	return SubstringAfter(NewRaw(), "::")
}

func buildTestingSubstringJsonRespDec() jpf.Parser[string] {
	return SubstringJsonObject(NewRaw())
}

func buildTestingSubstringAfterRespDec() jpf.Parser[string] {
	return SubstringAfter(NewRaw(), ">>>")
}

type RDCase[T comparable] struct {
	ID            string
	Build         func() jpf.Parser[T]
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

var RDCases = []utils.TestCase{
	RDCase[utils.TestStruct]{
		ID:            "json/no_response",
		Build:         NewJson[utils.TestStruct],
		Input:         "",
		ExpectedError: true,
	},
	RDCase[utils.TestStruct]{
		ID:       "json/empty_valid_json",
		Build:    NewJson[utils.TestStruct],
		Input:    `{}`,
		Expected: utils.TestStruct{},
	},
	RDCase[utils.TestStruct]{
		ID:       "json/valid_json",
		Build:    NewJson[utils.TestStruct],
		Input:    `{"a":5, "b": "xyz"}`,
		Expected: utils.TestStruct{A: 5, B: "xyz"},
	},
	RDCase[utils.TestStruct]{
		ID:       "json/valid_json_within_decoration",
		Build:    NewJson[utils.TestStruct],
		Input:    "Here is my answer:\n```" + `{"a":5, "b": "xyz"}` + "```",
		Expected: utils.TestStruct{A: 5, B: "xyz"},
	},
	RDCase[string]{
		ID:       "string/empty_string",
		Build:    NewRaw,
		Input:    "",
		Expected: "",
	},
	RDCase[string]{
		ID:       "string/random_string",
		Build:    NewRaw,
		Input:    "hdvdihiuhdibdb",
		Expected: "hdvdihiuhdibdb",
	},
	RDCase[string]{
		ID:       "string/random_string_with_whitespace",
		Build:    NewRaw,
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
	RDCase[string]{
		ID:       "substringafter/normal",
		Build:    buildTestingSubstringAfterRespDec,
		Input:    `abc>>>def`,
		Expected: `def`,
	},
	RDCase[string]{
		ID:       "substringafter/nopresent",
		Build:    buildTestingSubstringAfterRespDec,
		Input:    `abc???def`,
		Expected: "abc???def",
	},
	RDCase[string]{
		ID:       "substringjson/normal",
		Build:    buildTestingSubstringJsonRespDec,
		Input:    `abc{"def":"123}ldsfmkdflkfm`,
		Expected: `{"def":"123}`,
	},
	RDCase[string]{
		ID:            "substringjson/nopresent",
		Build:         buildTestingSubstringJsonRespDec,
		Input:         `abc{"def":"123ldsfmkdflkfm`,
		ExpectedError: true,
	},
}

func TestParser(t *testing.T) {
	utils.RunTests(t, RDCases)
}
