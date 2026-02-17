package feedbacks

import (
	"github.com/JoshPattman/jpf"
)

// NewRawMessageFeedbackGenerator creates a FeedbackGenerator that formats feedback by returning the error message as a string.
func NewRawMessageFeedbackGenerator() jpf.FeedbackGenerator {
	return &rawMessageFeedbackGenerator{}
}

type rawMessageFeedbackGenerator struct{}

func (g *rawMessageFeedbackGenerator) FormatFeedback(_ jpf.Message, err error) string {
	return err.Error()
}
