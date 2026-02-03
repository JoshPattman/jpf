package jpf

import (
	"errors"
	"strings"
)

// Wrap an existing [Parser] with one that takes only the part of interest of the response into account.
// The part of interest is determined by the substring function.
// If an error is detected when getting the substring, [ErrInvalidResponse] is raised.
func NewSubstringParser[T any](decoder Parser[T], substring func(string) (string, error)) Parser[T] {
	return &substringParser[T]{
		decoder:   decoder,
		substring: substring,
	}
}

// Wrap an existing [Parser] with one that takes only part of the response after the separator into account.
// If an error is detected when getting the substring, [ErrInvalidResponse] is raised.
func NewSubstringAfterParser[T any](decoder Parser[T], separator string) Parser[T] {
	substringFunc := func(s string) (string, error) {
		substrings := strings.Split(s, separator)
		return substrings[len(substrings)-1], nil
	}
	return NewSubstringParser(decoder, substringFunc)
}

type substringParser[T any] struct {
	decoder   Parser[T]
	substring func(string) (string, error)
}

func (srd *substringParser[T]) ParseResponseText(resp string) (T, error) {
	var zero T
	sub, err := srd.substring(resp)
	if err != nil {
		return zero, errors.Join(err, ErrInvalidResponse)
	}
	return srd.decoder.ParseResponseText(sub)
}
