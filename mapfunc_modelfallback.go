package jpf

import (
	"errors"
	"slices"
)

// Creates a map func that first tries to ask the first model,
// and if that produces an invalid format will try to ask the next models
// until a valid format is found.
// This is useful, for example, to try a second time with a model that overwrites the cache.
func NewModelFallbackOneShotMapFunc[T, U any](
	enc MessageEncoder[T],
	dec ResponseDecoder[U],
	models ...Model,
) MapFunc[T, U] {
	return &modelFallbackOneShotMapFunc[T, U]{
		enc,
		dec,
		models,
	}
}

type modelFallbackOneShotMapFunc[T, U any] struct {
	enc    MessageEncoder[T]
	dec    ResponseDecoder[U]
	models []Model
}

func (m *modelFallbackOneShotMapFunc[T, U]) Call(input T) (U, Usage, error) {
	totalUsage := Usage{}
	errs := make([]error, 0)
	for _, model := range m.models {
		result, usage, err := m.callOne(input, model)
		totalUsage = totalUsage.Add(usage)
		if err == nil {
			return result, totalUsage, nil
		}
		errs = append(errs, err)
	}
	errs = slices.Insert(errs, 0, errors.New("all models failed to produce valid outputs"))
	var zero U
	return zero, totalUsage, errors.Join(errs...)
}

func (mf *modelFallbackOneShotMapFunc[T, U]) callOne(t T, model Model) (U, Usage, error) {
	var zero U
	msgs, err := mf.enc.BuildInputMessages(t)
	if err != nil {
		return zero, Usage{}, wrap(err, "failed to build input messages")
	}
	resp, err := model.Respond(msgs)
	if err != nil {
		return zero, resp.Usage, wrap(err, "failed to get model response")
	}
	result, err := mf.dec.ParseResponseText(resp.PrimaryMessage.Content)
	if err != nil {
		return zero, resp.Usage, wrap(err, "failed to parse model response")
	}
	return result, resp.Usage, nil
}
