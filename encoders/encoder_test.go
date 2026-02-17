package encoders

import (
	"errors"
	"fmt"
	"testing"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/utils"
)

type MECase[T any] struct {
	ID            string
	Build         func() jpf.Encoder[T]
	Input         T
	Expected      []string
	ExpectedError bool
}

func (testCase MECase[T]) Name() string { return testCase.ID }

func (testCase MECase[T]) Test() error {
	me := testCase.Build()
	result, err := me.BuildInputMessages(testCase.Input)
	if testCase.ExpectedError {
		if err == nil {
			return fmt.Errorf("expected error, got none")
		}
	} else {
		if err != nil {
			return errors.Join(fmt.Errorf("expecting no error, got one"), err)
		}
		if len(result) != len(testCase.Expected) {
			return errors.Join(fmt.Errorf("expected and observed did not match. Expected %v but got %v", testCase.Expected, result))
		}
		for i := range result {
			if result[i].Content != testCase.Expected[i] {
				return errors.Join(fmt.Errorf("expected and observed did not match. Expected %v but got %v", testCase.Expected, result))
			}
		}
	}
	return nil
}

var MECases = []utils.TestCase{
	MECase[string]{
		ID: "rawstring",
		Build: func() jpf.Encoder[string] {
			return NewFixedEncoder("1234")
		},
		Input:    "abcd",
		Expected: []string{"1234", "abcd"},
	},
	MECase[utils.TestStruct]{
		ID: "template/empty",
		Build: func() jpf.Encoder[utils.TestStruct] {
			return NewTemplateEncoder[utils.TestStruct]("", "")
		},
		Input:    utils.TestStruct{},
		Expected: []string{},
	},
	MECase[utils.TestStruct]{
		ID: "template/system",
		Build: func() jpf.Encoder[utils.TestStruct] {
			return NewTemplateEncoder[utils.TestStruct]("Data (A): {{.A}}", "")
		},
		Input:    utils.TestStruct{A: 5},
		Expected: []string{"Data (A): 5"},
	},
	MECase[utils.TestStruct]{
		ID: "template/user",
		Build: func() jpf.Encoder[utils.TestStruct] {
			return NewTemplateEncoder[utils.TestStruct]("", "Data (B): {{.B}}")
		},
		Input:    utils.TestStruct{B: "x"},
		Expected: []string{"Data (B): x"},
	},
	MECase[utils.TestStruct]{
		ID: "template/both",
		Build: func() jpf.Encoder[utils.TestStruct] {
			return NewTemplateEncoder[utils.TestStruct]("Data (A): {{.A}}", "Data (B): {{.B}}")
		},
		Input:    utils.TestStruct{A: 5, B: "x"},
		Expected: []string{"Data (A): 5", "Data (B): x"},
	},
}

func TestEncoder(t *testing.T) {
	utils.RunTests(t, MECases)
}
