package parsers

import (
	"encoding/json"
	"errors"
	"reflect"
	"regexp"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/utils"
)

// NewJsonParser creates a [Parser] that tries to parse a json object from the response.
// It can ONLY parse json objects with an OBJECT as top level (i.e. it cannot parse a list directly).
func NewJsonParser[T any]() jpf.Parser[T] {
	var zero T
	// Ensure T is either a struct or a map at runtime using reflection
	{
		typ := reflect.TypeOf(zero)
		kind := typ.Kind()
		if !(kind == reflect.Struct || (kind == reflect.Map && typ.Key().Kind() == reflect.String)) {
			panic("NewJsonParser: T must be a struct or a map with string keys")
		}
	}
	return &jsonParser[T]{}
}

type jsonParser[T any] struct{}

func (d *jsonParser[T]) ParseResponseText(response string) (T, error) {
	re := regexp.MustCompile(`(?s)\{.*\}`)
	match := re.FindString(response)
	if match == "" {
		var zero T
		return zero, utils.Wrap(jpf.ErrInvalidResponse, "response did not contain a json object")
	}
	var result T
	err := json.Unmarshal([]byte(match), &result)
	if err != nil {
		var zero T
		return zero, utils.Wrap(errors.Join(err, jpf.ErrInvalidResponse), "llm returned an invalid json object")
	}
	return result, nil
}
