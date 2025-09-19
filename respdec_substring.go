package jpf

import (
	"errors"
	"strings"
)

// Create a new substringer that returns the last block of text after `split`.
// It will never error.
func SubstringAfter(split string) func(string) (string, error) {
	return func(s string) (string, error) {
		substrings := strings.Split(s, split)
		return substrings[len(substrings)-1], nil
	}
}

// Wrap an existing response decoder with one that takes only the part of interest of the response into account.
// The part of interest is determined by the substring function.
// If an error is detected when getting the substring, ErrInvalidResponse is raised.
func NewSubstringResponseDecoder[T any](decoder ResponseDecoder[T], substring func(string) (string, error)) ResponseDecoder[T] {
	return &substringResponseDecoder[T]{
		decoder:   decoder,
		substring: substring,
	}
}

type substringResponseDecoder[T any] struct {
	decoder   ResponseDecoder[T]
	substring func(string) (string, error)
}

func (srd *substringResponseDecoder[T]) ParseResponseText(resp string) (T, error) {
	var zero T
	sub, err := srd.substring(resp)
	if err != nil {
		return zero, errors.Join(err, ErrInvalidResponse)
	}
	return srd.decoder.ParseResponseText(sub)
}
