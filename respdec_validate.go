package jpf

import "errors"

// Creates a response decoder that wraps the provided one,
// but then performs an extra validation step on the parsed response.
// If an error is found during validation, the error is wrapped with ErrInvalidResponse and returned.
func NewValidatingResponseDecoder[T any](decoder ResponseDecoder[T], validate func(T) error) ResponseDecoder[T] {
	return &validatingResponseDecoder[T]{
		decoder:  decoder,
		validate: validate,
	}
}

type validatingResponseDecoder[T any] struct {
	decoder  ResponseDecoder[T]
	validate func(T) error
}

func (dec *validatingResponseDecoder[T]) ParseResponseText(response string) (T, error) {
	var zero T
	result, err := dec.decoder.ParseResponseText(response)
	if err != nil {
		return zero, err
	}
	err = dec.validate(result)
	if err != nil {
		return zero, errors.Join(err, ErrInvalidResponse)
	}
	return result, nil
}
