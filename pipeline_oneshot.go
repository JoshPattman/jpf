package jpf

import "context"

// NewOneShotPipeline creates a [Pipeline] that runs without retries.
// The validator may be nil.
func NewOneShotPipeline[T, U any](
	encoder Encoder[T],
	parser Parser[U],
	validator Validator[T, U],
	model Model,
) Pipeline[T, U] {
	return &oneShotPipeline[T, U]{
		encoder:   encoder,
		parser:    parser,
		validator: validator,
		model:     model,
	}
}

type oneShotPipeline[T, U any] struct {
	encoder   Encoder[T]
	parser    Parser[U]
	validator Validator[T, U]
	model     Model
}

func (mf *oneShotPipeline[T, U]) Call(ctx context.Context, t T) (U, Usage, error) {
	var zero U
	msgs, err := mf.encoder.BuildInputMessages(t)
	if err != nil {
		return zero, Usage{}, wrap(err, "failed to build input messages")
	}
	resp, err := mf.model.Respond(ctx, msgs)
	if err != nil {
		return zero, resp.Usage, wrap(err, "failed to get model response")
	}
	result, err := mf.parser.ParseResponseText(resp.PrimaryMessage.Content)
	if err != nil {
		return zero, resp.Usage, wrap(err, "failed to parse model response")
	}
	if mf.validator != nil {
		err := mf.validator.ValidateParsedResponse(t, result)
		if err != nil {
			return zero, resp.Usage, wrap(err, "failed to validate model response")
		}
	}
	return result, resp.Usage, nil
}
