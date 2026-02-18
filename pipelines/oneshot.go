package pipelines

import (
	"context"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
)

// NewOneShot creates a [Pipeline] that runs without retries.
// The validator may be nil.
func NewOneShot[T, U any](
	encoder jpf.Encoder[T],
	parser jpf.Parser[U],
	validator jpf.Validator[T, U],
	model jpf.Model,
) jpf.Pipeline[T, U] {
	return &oneShotPipeline[T, U]{
		encoder:   encoder,
		parser:    parser,
		validator: validator,
		model:     model,
	}
}

type oneShotPipeline[T, U any] struct {
	encoder   jpf.Encoder[T]
	parser    jpf.Parser[U]
	validator jpf.Validator[T, U]
	model     jpf.Model
}

func (mf *oneShotPipeline[T, U]) Call(ctx context.Context, t T) (U, jpf.Usage, error) {
	var zero U
	msgs, err := mf.encoder.BuildInputMessages(t)
	if err != nil {
		return zero, jpf.Usage{}, utils.Wrap(err, "failed to build input messages")
	}
	resp, err := mf.model.Respond(ctx, msgs)
	if err != nil {
		return zero, resp.Usage, utils.Wrap(err, "failed to get model response")
	}
	result, err := mf.parser.ParseResponseText(resp.PrimaryMessage.Content)
	if err != nil {
		return zero, resp.Usage, utils.Wrap(err, "failed to parse model response")
	}
	if mf.validator != nil {
		err := mf.validator.ValidateParsedResponse(t, result)
		if err != nil {
			return zero, resp.Usage, utils.Wrap(err, "failed to validate model response")
		}
	}
	return result, resp.Usage, nil
}
