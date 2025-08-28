package jpf

// FeedbackGenerator takes an error and converts it to a piece of text feedback to send to the LLM.
type FeedbackGenerator interface {
	FormatFeedback(Message, error) string
}
