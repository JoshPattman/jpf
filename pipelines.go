package jpf

import "context"

// Pipeline transforms input of type T into output of type U using an LLM.
// It handles the encoding of input, interaction with the LLM, and decoding of output.
type Pipeline[T, U any] interface {
	Call(context.Context, T) (PipelineResponse[U], error)
}

type PipelineResponse[U any] struct {
	// The parsed and validated result.
	Result U
	// The usage of making this call.
	// This may be the sum of multiple LLM calls.
	Usage Usage
}

// Utility to allow you to return the usage but 0 value messages when an error occurs.
func (r PipelineResponse[U]) OnlyUsage() PipelineResponse[U] {
	return PipelineResponse[U]{Usage: r.Usage}
}

// Utility to include another usage object in this response object
func (r PipelineResponse[U]) IncludingUsage(u Usage) PipelineResponse[U] {
	return PipelineResponse[U]{
		Result: r.Result,
		Usage:  r.Usage.Add(u),
	}
}
