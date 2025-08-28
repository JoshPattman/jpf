package jpf

// NewRawStringResponseDecoder creates a ResponseDecoder that returns the response as a raw string without modification.
func NewRawStringResponseDecoder() ResponseDecoder[string] {
	return &rawStringResponseDecoder{}
}

type rawStringResponseDecoder struct{}

func (d *rawStringResponseDecoder) ParseResponseText(response string) (string, error) {
	return response, nil
}
