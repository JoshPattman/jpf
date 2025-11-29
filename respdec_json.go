package jpf

import (
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
)

// NewJsonResponseDecoder creates a ResponseDecoder that tries to parse a json object from the response.
// It can ONLY parse json objects with an OBJECT as top level (i.e. it cannot parse a list directly).
func NewJsonResponseDecoder[T, U any]() ResponseDecoder[T, U] {
	var zero U
	// Ensure T is either a struct or a map at runtime using reflection
	{
		typ := reflect.TypeOf(zero)
		kind := typ.Kind()
		if !(kind == reflect.Struct || (kind == reflect.Map && typ.Key().Kind() == reflect.String)) {
			panic("NewJsonResponseDecoder: U must be a struct or a map with string keys")
		}
	}
	return &jsonResponseDecoder[T, U]{}
}

type jsonResponseDecoder[T, U any] struct{}

func (d *jsonResponseDecoder[T, U]) ParseResponseText(_ T, response string) (U, error) {
	re := regexp.MustCompile(`(?s)\{.*\}`)
	match := re.FindString(response)
	if match == "" {
		var zero U
		return zero, wrap(ErrInvalidResponse, "response did not contain a json object")
	}
	var result U
	err := json.Unmarshal([]byte(match), &result)
	if err != nil {
		var zero U
		return zero, wrap(errors.Join(err, ErrInvalidResponse), "llm returned an invalid json object")
	}
	return result, nil
}
