package jpf

import "errors"

// Creates a response decoder that wraps the provided one,
// but then performs an extra validation step on the parsed response.
// If an error is found during validation, the error is wrapped with ErrInvalidResponse and returned.
func NewValidatingResponseDecoder[T, U any](decoder ResponseDecoder[T, U], validate func(T, U) error) ResponseDecoder[T, U] {
	return &validatingResponseDecoder[T, U]{
		decoder:  decoder,
		validate: validate,
	}
}

type validatingResponseDecoder[T, U any] struct {
	decoder  ResponseDecoder[T, U]
	validate func(T, U) error
}

func (dec *validatingResponseDecoder[T, U]) ParseResponseText(input T, response string) (U, error) {
	var zero U
	result, err := dec.decoder.ParseResponseText(input, response)
	if err != nil {
		return zero, err
	}
	err = dec.validate(input, result)
	if err != nil {
		return zero, errors.Join(err, ErrInvalidResponse)
	}
	return result, nil
}
