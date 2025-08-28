package jpf

import "errors"

var (
	ErrInvalidResponse = errors.New("llm produced an invalid response")
)

// ResponseDecoder converts an LLM response into a structured piece of data.
// When the LLM response is invalid, it should return ErrInvalidResponse (or an error joined on that).
type ResponseDecoder[T any] interface {
	ParseResponseText(string) (T, error)
}
