package jpf

// NewRawMessageFeedbackGenerator creates a FeedbackGenerator that formats feedback by returning the error message as a string.
func NewRawMessageFeedbackGenerator() FeedbackGenerator {
	return &rawMessageFeedbackGenerator{}
}

type rawMessageFeedbackGenerator struct{}

func (g *rawMessageFeedbackGenerator) FormatFeedback(_ Message, err error) string {
	return err.Error()
}
