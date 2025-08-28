package jpf

// MessageEncoder encodes a structured piece of data into a set of messages for an LLM.
type MessageEncoder[T any] interface {
	BuildInputMessages(T) ([]Message, error)
}
