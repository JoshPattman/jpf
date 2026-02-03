package jpf

import (
	"context"
	"errors"
	"slices"
)

// Creates a [Pipeline] that first tries to ask the first model,
// and if that produces an invalid format will try to ask the next models
// until a valid format is found.
// This is useful, for example, to try a second time with a model that overwrites the cache.
func NewFallbackPipeline[T, U any](
	encoder Encoder[T],
	parser Parser[U],
	validator Validator[T, U],
	models ...Model,
) Pipeline[T, U] {
	return &fallbackPipeline[T, U]{
		encoder,
		parser,
		validator,
		models,
	}
}

type fallbackPipeline[T, U any] struct {
	encoder   Encoder[T]
	decoder   Parser[U]
	validator Validator[T, U]
	models    []Model
}

func (m *fallbackPipeline[T, U]) Call(ctx context.Context, input T) (U, Usage, error) {
	var zero U
	totalUsage := Usage{}
	errs := make([]error, 0)
	for _, model := range m.models {
		result, usage, err := m.callOne(ctx, input, model)
		totalUsage = totalUsage.Add(usage)
		if err == nil {
			return result, totalUsage, nil
		} else if !errors.Is(err, ErrInvalidResponse) {
			// Only non-expected errors go here
			return zero, totalUsage, err
		}
		errs = append(errs, err)
	}
	errs = slices.Insert(errs, 0, errors.New("all models failed to produce valid outputs"))
	return zero, totalUsage, errors.Join(errs...)
}

func (mf *fallbackPipeline[T, U]) callOne(ctx context.Context, t T, model Model) (U, Usage, error) {
	var zero U
	msgs, err := mf.encoder.BuildInputMessages(t)
	if err != nil {
		return zero, Usage{}, wrap(err, "failed to build input messages")
	}
	resp, err := model.Respond(ctx, msgs)
	if err != nil {
		return zero, resp.Usage, wrap(err, "failed to get model response")
	}
	result, err := mf.decoder.ParseResponseText(resp.PrimaryMessage.Content)
	if err != nil {
		return zero, resp.Usage, wrap(err, "failed to parse model response")
	}
	if mf.validator != nil {
		err = mf.validator.ValidateParsedResponse(t, result)
		if err != nil {
			return zero, resp.Usage, wrap(err, "failed to validate model response")
		}
	}
	return result, resp.Usage, nil
}
