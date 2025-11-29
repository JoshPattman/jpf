package jpf

// NewRawStringResponseDecoder creates a ResponseDecoder that returns the response as a raw string without modification.
func NewRawStringResponseDecoder[T any]() ResponseDecoder[T, string] {
	return &rawStringResponseDecoder[T]{}
}

type rawStringResponseDecoder[T any] struct{}

func (d *rawStringResponseDecoder[T]) ParseResponseText(_ T, response string) (string, error) {
	return response, nil
}
