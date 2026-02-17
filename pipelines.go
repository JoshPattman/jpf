package jpf

import "context"

// Pipeline transforms input of type T into output of type U using an LLM.
// It handles the encoding of input, interaction with the LLM, and decoding of output.
type Pipeline[T, U any] interface {
	Call(context.Context, T) (U, Usage, error)
}
