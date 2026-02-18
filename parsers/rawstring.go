package parsers

import "github.com/JoshPattman/jpf"

// NewRaw creates a [Parser] that returns the response as a raw string without modification.
func NewRaw() jpf.Parser[string] {
	return &rawStringParser{}
}

type rawStringParser struct{}

func (d *rawStringParser) ParseResponseText(response string) (string, error) {
	return response, nil
}
