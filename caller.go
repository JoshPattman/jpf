package jpf

// Caller transforms input of type T into output of type U using.
// It may be a raw LLM call, image generation call, etc...
// It may also handle the encoding of input and decoding of output.
type Caller[T, U any] interface {
	Call(T) (U, error)
}
