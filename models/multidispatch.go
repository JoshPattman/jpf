package models

import (
	"context"
	"errors"
	"sync"

	"github.com/JoshPattman/jpf"
)

var ErrNoMultiDispatchModels = errors.New("no models provided to multi dispatch model, it needs at least 1")

// Wrap a number of underlying models.
// Each model will have the same request sent to it at the same time.
// The fastest model to respond without error will provide the final response.
// If all models respond with error, the fastest erroring model will be returned.
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

type multiDispatchErrorResult struct {
	usage jpf.Usage
	err   error
}

func (m *multiDispatchModel) Respond(ctx context.Context, messages []jpf.Message, _ jpf.ModelStreamer) (jpf.ModelResponse, error) {
	if len(m.models) == 0 {
		return jpf.ModelResponse{}, ErrNoMultiDispatchModels
	}

	okResult := make(chan jpf.ModelResponse, 1)
	errResult := make(chan multiDispatchErrorResult, 1)
	allDone := &sync.WaitGroup{}
	allDone.Add(len(m.models))

	for _, model := range m.models {
		go func(model jpf.Model) {
			defer allDone.Done()
			// Note it is deliberate here that we are not passing the streamer
			resp, err := model.Respond(ctx, messages, nil)
			if err != nil {
				select {
				case errResult <- multiDispatchErrorResult{resp.Usage, err}:
				default:
				}
			} else {
				select {
				case okResult <- resp:
				default:
				}
			}
		}(model)
	}
	allDoneChan := make(chan struct{})
	go func() {
		allDone.Wait()
		close(allDoneChan)
	}()
	select {
	case response := <-okResult:
		return response, nil
	case <-ctx.Done():
		return jpf.ModelResponse{}, ctx.Err()
	case <-allDoneChan:
	}
	select {
	case response := <-errResult:
		return jpf.ModelResponse{Usage: response.usage}, response.err
	case <-ctx.Done():
		return jpf.ModelResponse{}, ctx.Err()
	}
}
