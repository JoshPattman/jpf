package jpf

import (
	"context"
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

func (m *retryChainModel) Respond(ctx context.Context, msgs []Message) (ModelResponse, error) {
	var errs []error
	var totalUsageSoFar Usage

	for i, model := range m.models {
		resp, err := model.Respond(ctx, msgs)
		resp = resp.IncludingUsage(totalUsageSoFar)

		if err == nil {
			return resp, nil
		}

		errs = append(errs, fmt.Errorf("model %d failed: %w", i, err))
		totalUsageSoFar = resp.Usage
	}

	return ModelResponse{Usage: totalUsageSoFar}, wrap(
		errors.Join(errs...),
		"all %d models in retry chain failed",
		len(m.models),
	)
}
