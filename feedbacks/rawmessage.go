package feedbacks

import (
	"github.com/JoshPattman/jpf"
)

// NewErrString creates a FeedbackGenerator that formats feedback by returning the error message as a string.
func NewErrString() jpf.FeedbackGenerator {
	return &errStringFG{}
}

type errStringFG struct{}

func (g *errStringFG) FormatFeedback(_ jpf.Message, err error) string {
	return err.Error()
}
