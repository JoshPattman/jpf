package jpf

import (
	"context"
	"errors"
)

// NewFeedbackMapFunc creates a MapFunc that first runs the encoder, then the model, finally parsing the response with the decoder.
// However, it adds feedback to the conversation when errors are detected.
// It will only add to the conversation if the error returned from the parser is an ErrInvalidResponse (using errors.Is).
func NewFeedbackMapFunc[T, U any](
	enc MessageEncoder[T],
	pars ResponseDecoder[U],
	fed FeedbackGenerator,
	model Model,
	feedbackRole Role,
	maxRetries int,
) MapFunc[T, U] {
	return &feedbackMapFunc[T, U]{
		enc:          enc,
		pars:         pars,
		fed:          fed,
		model:        model,
		feedbackRole: feedbackRole,
		maxRetries:   maxRetries,
	}
}

type feedbackMapFunc[T, U any] struct {
	enc          MessageEncoder[T]
	pars         ResponseDecoder[U]
	fed          FeedbackGenerator
	model        Model
	feedbackRole Role
	maxRetries   int
}

func (mf *feedbackMapFunc[T, U]) Call(ctx context.Context, t T) (U, Usage, error) {
	var u U
	history, err := mf.enc.BuildInputMessages(t)
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
		result, err := mf.pars.ParseResponseText(resp.PrimaryMessage.Content)
		if err == nil {
			// If the result was ok, return it
			return result, totalUsage, nil
		} else if errors.Is(err, ErrInvalidResponse) {
			// If it was a parse error, add to the conversation history and continue looping
			feedback := mf.fed.FormatFeedback(resp.PrimaryMessage, err)
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
