package jpf

// Encoder encodes a structured piece of data into a set of messages for an LLM.
type Encoder[T any] interface {
	BuildInputMessages(T) ([]Message, error)
}
