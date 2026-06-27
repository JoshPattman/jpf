package pipelines

import (
	"context"
	"errors"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
)

// NewFeedbackRetry creates a [Pipeline] that first runs the encoder, then the model, finally parsing the response with the decoder.
// However, it adds feedback to the conversation when errors are detected.
// It will only add to the conversation if the error returned from the parser is an [ErrInvalidResponse] (using errors.Is).
func NewFeedbackRetry[T, U any](
	encoder jpf.Encoder[T],
	parser jpf.Parser[U],
	feedbackGenerator jpf.FeedbackGenerator,
	model jpf.Model,
	maxRetries int,
	opts ...ConstructionOpt[T, U],
) jpf.Pipeline[T, U] {
	kwargs := GetConstructionKwargs(opts...)
	return &feedbackPipeline[T, U]{
		encoder:           encoder,
		parser:            parser,
		validator:         kwargs.Validator,
		feedbackGenerator: feedbackGenerator,
		model:             model,
		maxRetries:        maxRetries,
		outputFormat:      kwargs.OutputFormat,
	}
}

type feedbackPipeline[T, U any] struct {
	encoder           jpf.Encoder[T]
	parser            jpf.Parser[U]
	validator         jpf.Validator[T, U]
	feedbackGenerator jpf.FeedbackGenerator
	model             jpf.Model
	maxRetries        int
	outputFormat      any
}

func (mf *feedbackPipeline[T, U]) Call(ctx context.Context, t T) (jpf.PipelineResponse[U], error) {
	history, err := mf.encoder.BuildInputMessages(t)
	if err != nil {
		return jpf.PipelineResponse[U]{}, utils.Wrap(err, "failed to build input messages")
	}
	totalUsage := jpf.Usage{}
	var lastErr error
	for range mf.maxRetries + 1 {
		resp, err := mf.model.Respond(ctx, history, jpf.WithOutputFormat(mf.outputFormat))
		totalUsage = totalUsage.Add(resp.Usage)
		if err != nil {
			return jpf.PipelineResponse[U]{Usage: totalUsage}, utils.Wrap(err, "failed to get model response")
		}
		result, err := mf.parser.ParseResponseText(resp.Message.Content)
		// If there was no parse error and we have a validator, validate
		if err == nil && mf.validator != nil {
			err = mf.validator.ValidateParsedResponse(t, result)
		}
		if err == nil {
			// If the result was ok, return it
			return jpf.PipelineResponse[U]{Result: result, Usage: totalUsage}, nil
		} else if errors.Is(err, jpf.ErrInvalidResponse) {
			// If it was a parse error, add to the conversation history and continue looping
			feedback := mf.feedbackGenerator.FormatFeedback(resp.Message, err)
			lastErr = err
			history = append(history, resp.Message)
			history = append(history, jpf.UserMessage{
				Content: feedback,
			})
		} else {
			// Otherwise, it was another error so return the error (don't loop)
			return jpf.PipelineResponse[U]{Usage: totalUsage}, utils.Wrap(err, "failed to parse model response")
		}
	}
	return jpf.PipelineResponse[U]{Usage: totalUsage}, utils.Wrap(lastErr, "model failed to produce a valid response after trying %d times", mf.maxRetries+1)
}
