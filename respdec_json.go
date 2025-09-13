package jpf

import (
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
)

// NewJsonResponseDecoder creates a ResponseDecoder that tries to parse a json object from the response.
// It can ONLY parse json objects with an OBJECT as top level (i.e. it cannot parse a list directly).
func NewJsonResponseDecoder[T any]() ResponseDecoder[T] {
	var zero T
	// Ensure T is either a struct or a map at runtime using reflection
	{
		typ := reflect.TypeOf(zero)
		kind := typ.Kind()
		if !(kind == reflect.Struct || (kind == reflect.Map && typ.Key().Kind() == reflect.String)) {
			panic("NewJsonResponseDecoder: T must be a struct or a map with string keys")
		}
	}
	return &jsonResponseDecoder[T]{}
}

type jsonResponseDecoder[T any] struct{}

func (d *jsonResponseDecoder[T]) ParseResponseText(response string) (T, error) {
	re := regexp.MustCompile(`(?s)\{.*\}`)
	match := re.FindString(response)
	if match == "" {
		var zero T
		return zero, wrap(ErrInvalidResponse, "response did not contain a json object")
	}
	var result T
	err := json.Unmarshal([]byte(match), &result)
	if err != nil {
		var zero T
		return zero, wrap(errors.Join(err, ErrInvalidResponse), "llm returned an invalid json object")
	}
	return result, nil
}
