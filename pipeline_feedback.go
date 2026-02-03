package jpf

import (
	"context"
	"errors"
)

// NewFeedbackPipeline creates a [Pipeline] that first runs the encoder, then the model, finally parsing the response with the decoder.
// However, it adds feedback to the conversation when errors are detected.
// It will only add to the conversation if the error returned from the parser is an [ErrInvalidResponse] (using errors.Is).
func NewFeedbackPipeline[T, U any](
	encoder Encoder[T],
	parser Parser[U],
	validator Validator[T, U],
	feedbackGenerator FeedbackGenerator,
	model Model,
	feedbackRole Role,
	maxRetries int,
) Pipeline[T, U] {
	return &feedbackPipeline[T, U]{
		encoder:           encoder,
		parser:            parser,
		validator:         validator,
		feedbackGenerator: feedbackGenerator,
		model:             model,
		feedbackRole:      feedbackRole,
		maxRetries:        maxRetries,
	}
}

type feedbackPipeline[T, U any] struct {
	encoder           Encoder[T]
	parser            Parser[U]
	validator         Validator[T, U]
	feedbackGenerator FeedbackGenerator
	model             Model
	feedbackRole      Role
	maxRetries        int
}

func (mf *feedbackPipeline[T, U]) Call(ctx context.Context, t T) (U, Usage, error) {
	var u U
	history, err := mf.encoder.BuildInputMessages(t)
	if err != nil {
		return u, Usage{}, wrap(err, "failed to build input messages")
	}
	totalUsage := Usage{}
	var lastErr error
	for range mf.maxRetries + 1 {
		resp, err := mf.model.Respond(ctx, history)
		totalUsage = totalUsage.Add(resp.Usage)
		if err != nil {
			return u, totalUsage, wrap(err, "failed to get model response")
		}
		result, err := mf.parser.ParseResponseText(resp.PrimaryMessage.Content)
		// If there was no parse error and we have a validator, validate
		if err == nil && mf.validator != nil {
			err = mf.validator.ValidateParsedResponse(t, result)
		}
		if err == nil {
			// If the result was ok, return it
			return result, totalUsage, nil
		} else if errors.Is(err, ErrInvalidResponse) {
			// If it was a parse error, add to the conversation history and continue looping
			feedback := mf.feedbackGenerator.FormatFeedback(resp.PrimaryMessage, err)
			lastErr = err
			history = append(history, resp.PrimaryMessage)
			history = append(history, Message{
				Role:    mf.feedbackRole,
				Content: feedback,
			})
		} else {
			// Otherwise, it was another error so return the error (don't loop)
			return u, totalUsage, wrap(err, "failed to parse model response")
		}
	}
	return u, totalUsage, wrap(lastErr, "model failed to produce a valid response after trying %d times", mf.maxRetries+1)
}
