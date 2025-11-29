package jpf

import "errors"

var (
	ErrInvalidResponse = errors.New("llm produced an invalid response")
)

// ResponseDecoder converts an input to an LLM and the LLM response into a structured piece of output data.
// When the LLM response is invalid, it should return ErrInvalidResponse (or an error joined on that).
type ResponseDecoder[T, U any] interface {
	ParseResponseText(T, string) (U, error)
}
