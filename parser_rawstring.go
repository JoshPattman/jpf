package jpf

// NewStringParser creates a [Parser] that returns the response as a raw string without modification.
func NewStringParser() Parser[string] {
	return &rawStringParser{}
}

type rawStringParser struct{}

func (d *rawStringParser) ParseResponseText(response string) (string, error) {
	return response, nil
}
