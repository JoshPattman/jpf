package feedbacks

import (
	"errors"
	"fmt"
	"testing"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/utils"
)

type FGCase struct {
	ID           string
	Build        func() jpf.FeedbackGenerator
	InputMessage jpf.Message
	InputError   error
	Expected     string
}

func (testCase FGCase) Name() string { return testCase.ID }

func (testCase FGCase) Test() error {
	rd := testCase.Build()
	result := rd.FormatFeedback(testCase.InputMessage, testCase.InputError)
	if result != testCase.Expected {
		return errors.Join(fmt.Errorf("expected and observed did not match. Expected %v but got %v", testCase.Expected, result))
	}
	return nil
}

var FGCases = []utils.TestCase{
	FGCase{
		ID:           "rawmessage/errormessage",
		Build:        NewRawMessageFeedbackGenerator,
		InputMessage: jpf.Message{},
		InputError:   errors.New("abcdef"),
		Expected:     "abcdef",
	},
}

func TestFeedbackGenerator(t *testing.T) {
	utils.RunTests(t, FGCases)
}
