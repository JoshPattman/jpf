package jpf

import (
	"errors"
)

// NewFeedbackTypedChatCaller works with ChatCallers, and adds feedback to the conversation when errors are detected.
// It will only add to the conversation if the error returned from the parser is an ErrInvalidResponse (using errors.Is).
func NewFeedbackTypedChatCaller[T, U any](
	enc MessageEncoder[T],
	pars ResponseDecoder[U],
	fed FeedbackGenerator,
	model ChatCaller,
	feedbackRole Role,
	maxRetries int,
) Caller[T, U] {
	return &feedbackTypedChatCaller[T, U]{
		enc:          enc,
		pars:         pars,
		fed:          fed,
		model:        model,
		feedbackRole: feedbackRole,
		maxRetries:   maxRetries,
	}
}

type feedbackTypedChatCaller[T, U any] struct {
	enc          MessageEncoder[T]
	pars         ResponseDecoder[U]
	fed          FeedbackGenerator
	model        ChatCaller
	feedbackRole Role
	maxRetries   int
}

func (mf *feedbackTypedChatCaller[T, U]) Call(t T) (U, error) {
	var u U
	history, err := mf.enc.BuildInputMessages(t)
	if err != nil {
		return u, err
	}
	var lastErr error
	for range mf.maxRetries + 1 {
		res, err := mf.model.Call(history)
		if err != nil {
			return u, err
		}
		result, err := mf.pars.ParseResponseText(res.Primary.Content)
		if err == nil {
			// If the result was ok, return it
			return result, nil
		} else if errors.Is(err, ErrInvalidResponse) {
			// If it was a parse error, add to the conversation history and continue looping
			feedback := mf.fed.FormatFeedback(res.Primary, err)
			lastErr = err
			history = append(history, res.Primary)
			history = append(history, Message{
				Role:    mf.feedbackRole,
				Content: feedback,
			})
		} else {
			// Otherwise, it was another error so return the error (don't loop)
			return u, err
		}
	}
	return u, lastErr
}
