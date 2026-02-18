package parsers

import (
	"errors"
	"strings"

	"github.com/JoshPattman/jpf"
)

// Wrap an existing [Parser] with one that takes only the part of interest of the response into account.
// The part of interest is determined by the substring function.
// If an error is detected when getting the substring, [ErrInvalidResponse] is raised.
func Substring[T any](parser jpf.Parser[T], substring func(string) (string, error)) jpf.Parser[T] {
	return &substringParser[T]{
		decoder:   parser,
		substring: substring,
	}
}

// Wrap an existing [Parser] with one that takes only the part of the response after the last sep into account.
// If an error is detected when getting the substring, [ErrInvalidResponse] is raised.
func SubstringAfter[T any](parser jpf.Parser[T], separator string) jpf.Parser[T] {
	substringFunc := func(s string) (string, error) {
		substrings := strings.Split(s, separator)
		return substrings[len(substrings)-1], nil
	}
	return Substring(parser, substringFunc)
}

// Wrap an exising [Parser] with one that takes only the string between (including) the first and last curly brace into account.
func SubstringJsonObject[T any](parser jpf.Parser[T]) jpf.Parser[T] {
	substringFunc := func(s string) (string, error) {
		firstBracketIndex := strings.Index(s, "{")
		lastBracketIndex := strings.LastIndex(s, "}")
		if firstBracketIndex == -1 || lastBracketIndex == -1 {
			return "", errors.Join(errors.New("response did not contain an opening and closing curly brace"), jpf.ErrInvalidResponse)
		}
		return s[firstBracketIndex : lastBracketIndex+1], nil
	}
	return Substring(parser, substringFunc)
}

type substringParser[T any] struct {
	decoder   jpf.Parser[T]
	substring func(string) (string, error)
}

func (srd *substringParser[T]) ParseResponseText(resp string) (T, error) {
	var zero T
	sub, err := srd.substring(resp)
	if err != nil {
		return zero, errors.Join(err, jpf.ErrInvalidResponse)
	}
	return srd.decoder.ParseResponseText(sub)
}
