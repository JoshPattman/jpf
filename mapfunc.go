package jpf

// MapFunc transforms input of type T into output of type U using an LLM.
// It handles the encoding of input, interaction with the LLM, and decoding of output.
type MapFunc[T, U any] interface {
	Call(T) (U, Usage, error)
}
