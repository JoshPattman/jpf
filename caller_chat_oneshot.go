package jpf

// NewOneShotTypedChatCaller works with ChatCaller and tries to build, respond, and parse the messages, resulting in typed input and output.
func NewOneShotTypedChatCaller[T, U any](enc MessageEncoder[T], pars ResponseDecoder[U], model ChatCaller) Caller[T, U] {
	return &oneShotTYpedCaller[T, U]{
		enc:   enc,
		pars:  pars,
		model: model,
	}
}

type oneShotTYpedCaller[T, U any] struct {
	enc   MessageEncoder[T]
	pars  ResponseDecoder[U]
	model ChatCaller
}

func (mf *oneShotTYpedCaller[T, U]) Call(t T) (U, error) {
	var u U
	msgs, err := mf.enc.BuildInputMessages(t)
	if err != nil {
		return u, err
	}
	res, err := mf.model.Call(msgs)
	if err != nil {
		return u, err
	}
	result, err := mf.pars.ParseResponseText(res.Primary.Content)
	if err != nil {
		return u, err
	}
	return result, nil
}
