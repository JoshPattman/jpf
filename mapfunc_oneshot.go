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
	res, err := mf.model.Respond(msgs)
	if err != nil {
		return u, res.Usage, err
	}
	result, err := mf.pars.ParseResponseText(res.Primary.Content)
	if err != nil {
		return u, res.Usage, err
	}
	return result, res.Usage, nil
}
