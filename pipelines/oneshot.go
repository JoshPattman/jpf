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

func (mf *oneShotPipeline[T, U]) Call(ctx context.Context, t T) (jpf.PipelineResponse[U], error) {
	msgs, err := mf.encoder.BuildInputMessages(t)
	if err != nil {
		return jpf.PipelineResponse[U]{}, utils.Wrap(err, "failed to build input messages")
	}
	resp, err := mf.model.Respond(ctx, msgs, nil)
	if err != nil {
		return jpf.PipelineResponse[U]{Usage: resp.Usage}, utils.Wrap(err, "failed to get model response")
	}
	result, err := mf.parser.ParseResponseText(resp.Message.Content)
	if err != nil {
		return jpf.PipelineResponse[U]{Usage: resp.Usage}, utils.Wrap(err, "failed to parse model response")
	}
	if mf.validator != nil {
		err := mf.validator.ValidateParsedResponse(t, result)
		if err != nil {
			return jpf.PipelineResponse[U]{Usage: resp.Usage}, utils.Wrap(err, "failed to validate model response")
		}
	}
	return jpf.PipelineResponse[U]{Result: result, Usage: resp.Usage}, nil
}
