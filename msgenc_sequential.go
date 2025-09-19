package jpf

// Create a new message encoder that appends the results of running each message encoder sequentially.
// Useful, for example, to have a templating system / user message encoder, and a custom agent history message encoder after.
func NewSequentialMessageEncoder[T any](msgEncs ...MessageEncoder[T]) MessageEncoder[T] {
	return &sequentialMessageEncoder[T]{
		msgEncs: msgEncs,
	}
}

type sequentialMessageEncoder[T any] struct {
	msgEncs []MessageEncoder[T]
}

func (s *sequentialMessageEncoder[T]) BuildInputMessages(input T) ([]Message, error) {
	msgs := []Message{}
	for _, enc := range s.msgEncs {
		encMsgs, err := enc.BuildInputMessages(input)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, encMsgs...)
	}
	return msgs, nil
}
