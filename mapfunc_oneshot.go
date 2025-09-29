package jpf

// NewOneShotMapFunc creates a MapFunc that first runs the encoder, then the model, finally parsing the response with the decoder.
func NewOneShotMapFunc[T, U any](enc MessageEncoder[T], pars ResponseDecoder[U], model Model) MapFunc[T, U] {
	return &oneShotMapFunc[T, U]{
		enc:   enc,
		pars:  pars,
		model: model,
	}
}

type oneShotMapFunc[T, U any] struct {
	enc   MessageEncoder[T]
	pars  ResponseDecoder[U]
	model Model
}

func (mf *oneShotMapFunc[T, U]) Call(t T) (U, Usage, error) {
	var zero U
	msgs, err := mf.enc.BuildInputMessages(t)
	if err != nil {
		return zero, Usage{}, wrap(err, "failed to build input messages")
	}
	resp, err := mf.model.Respond(msgs)
	if err != nil {
		return zero, resp.Usage, wrap(err, "failed to get model response")
	}
	result, err := mf.pars.ParseResponseText(resp.PrimaryMessage.Content)
	if err != nil {
		return zero, resp.Usage, wrap(err, "failed to parse model response")
	}
	return result, resp.Usage, nil
}
