package jpf

import "errors"

var (
	ErrInvalidResponse = errors.New("llm produced an invalid response")
)

// Parser converts the LLM response into a structured piece of output data.
// When the LLM response is invalid, it should return [ErrInvalidResponse] (or an error joined on that).
type Parser[U any] interface {
	ParseResponseText(string) (U, error)
}

// Validator takes a parsed LLM response and validates it against the input.
// When the LLM response is invalid, it should return [ErrInvalidResponse] (or an error joined on that).
type Validator[T, U any] interface {
	ValidateParsedResponse(T, U) error
}

// FeedbackGenerator takes an error and converts it to a piece of text feedback to send to the LLM.
type FeedbackGenerator interface {
	FormatFeedback(Message, error) string
}
