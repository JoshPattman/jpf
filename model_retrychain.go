package jpf

import (
	"errors"
	"fmt"
)

// NewRetryChainModel creates a Model that tries a list of models in order,
// returning the result from the first one that doesn't fail.
// If all models fail, it returns a joined error containing all the errors.
func NewRetryChainModel(models []Model) Model {
	if len(models) == 0 {
		panic("NewRetryChainModel requires at least one model")
	}
	return &retryChainModel{
		models: models,
	}
}

type retryChainModel struct {
	models []Model
}

func (m *retryChainModel) Respond(msgs []Message) (ModelResponse, error) {
	var errs []error
	var totalUsageSoFar Usage

	for i, model := range m.models {
		resp, err := model.Respond(msgs)
		resp = resp.IncludingUsage(totalUsageSoFar)

		if err == nil {
			// Success! Return this response
			return resp, nil
		}

		// Failed, accumulate the error and usage
		errs = append(errs, fmt.Errorf("model %d failed: %w", i, err))
		totalUsageSoFar = resp.Usage
	}

	// All models failed, return combined error
	return ModelResponse{Usage: totalUsageSoFar}, wrap(
		errors.Join(errs...),
		"all %d models in retry chain failed",
		len(m.models),
	)
}
