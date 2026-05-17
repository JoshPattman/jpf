package models

import (
	"context"
	"errors"

	"github.com/JoshPattman/jpf"
)

var ErrNoMultiDispatchModels = errors.New("no models provided to multi dispatch model, it needs at least 1")

// Wrap a number of underlying models.
// Each model will have the same request sent to it at the same time.
// The fastest model to respond will provide the final response.
// This is useful when model response times are very variable,
// and you don't mind paying more to increase the chance of a fast response.
// WARNING: It is impossible to stream through this, because we do not know which
// response will be returned. Streaming does not work with this model in the chain.
// WARNING: Becasue we send off multiple requests but only wait for one to come back,
// the token usage tracking is inaccurate above this model in the chain.
// The tokens returned by this model are the tokens of the first response.
// For accurate token tracking, attach the token tracker below this model in the tree.
func MultiDispatch(models []jpf.Model) jpf.Model {
	return &multiDispatchModel{models}
}

type multiDispatchModel struct {
	models []jpf.Model
}

type mdResult struct {
	resp jpf.ModelResponse
	err  error
}

func (m *multiDispatchModel) Respond(ctx context.Context, messages []jpf.Message, _ jpf.ModelStreamer) (jpf.ModelResponse, error) {
	if len(m.models) == 0 {
		return jpf.ModelResponse{}, ErrNoMultiDispatchModels
	}
	result := make(chan mdResult, 1)
	for _, model := range m.models {
		go func(model jpf.Model) {
			// Note it is deliberate here that we are not passing the streamer
			resp, err := model.Respond(ctx, messages, nil)
			select {
			case result <- mdResult{resp, err}:
			default:
			}
		}(model)
	}
	select {
	case response := <-result:
		return response.resp, response.err
	case <-ctx.Done():
		cause := context.Cause(ctx)
		if cause != nil {
			return jpf.ModelResponse{}, cause
		}
		return jpf.ModelResponse{}, context.DeadlineExceeded
	}
}
