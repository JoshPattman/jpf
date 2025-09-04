package jpf

import (
	"errors"
	"fmt"
	"testing"
)

type MECase[T any] struct {
	ID            string
	Build         func() MessageEncoder[T]
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

var MECases = []TestCase{
	MECase[string]{
		ID: "rawstring",
		Build: func() MessageEncoder[string] {
			return NewRawStringMessageEncoder("1234")
		},
		Input:    "abcd",
		Expected: []string{"1234", "abcd"},
	},
	MECase[TestStruct]{
		ID: "template/empty",
		Build: func() MessageEncoder[TestStruct] {
			return NewTemplateMessageEncoder[TestStruct]("", "")
		},
		Input:    TestStruct{},
		Expected: []string{},
	},
	MECase[TestStruct]{
		ID: "template/system",
		Build: func() MessageEncoder[TestStruct] {
			return NewTemplateMessageEncoder[TestStruct]("Data (A): {{.A}}", "")
		},
		Input:    TestStruct{A: 5},
		Expected: []string{"Data (A): 5"},
	},
	MECase[TestStruct]{
		ID: "template/user",
		Build: func() MessageEncoder[TestStruct] {
			return NewTemplateMessageEncoder[TestStruct]("", "Data (B): {{.B}}")
		},
		Input:    TestStruct{B: "x"},
		Expected: []string{"Data (B): x"},
	},
	MECase[TestStruct]{
		ID: "template/both",
		Build: func() MessageEncoder[TestStruct] {
			return NewTemplateMessageEncoder[TestStruct]("Data (A): {{.A}}", "Data (B): {{.B}}")
		},
		Input:    TestStruct{A: 5, B: "x"},
		Expected: []string{"Data (A): 5", "Data (B): x"},
	},
}

func TestMessageEncoder(t *testing.T) {
	RunTests(t, MECases)
}
