package jpf

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
	var u U
	msgs, err := mf.enc.BuildInputMessages(t)
	if err != nil {
		return u, Usage{}, err
	}
	_, resp, usage, err := mf.model.Respond(msgs)
	if err != nil {
		return u, usage, err
	}
	result, err := mf.pars.ParseResponseText(resp.Content)
	if err != nil {
		return u, usage, err
	}
	return result, usage, nil
}
