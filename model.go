package jpf

import (
	"slices"
	"time"
)

// Usage defines how many tokens were used when making calls to LLMs.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

func (u Usage) Add(u2 Usage) Usage {
	return Usage{
		u.InputTokens + u2.InputTokens,
		u.OutputTokens + u2.OutputTokens,
	}
}

type ModelResponse struct {
	// Extra messages that are not the final response,
	// but were used to build up the final response.
	// For example, reasoning messages.
	AuxilliaryMessages []Message
	// The primary repsponse to the users query.
	// Usually the only response that matters.
	PrimaryMessage Message
	// The usage of making this call.
	// This may be the sum of multiple LLM calls.
	Usage Usage
}

// Utility to allow you to return the usage but 0 value messages when an error occurs.
func (r ModelResponse) OnlyUsage() ModelResponse {
	return ModelResponse{Usage: r.Usage}
}

// Utility to include another usage object in this response object
func (r ModelResponse) IncludingUsage(u Usage) ModelResponse {
	return ModelResponse{
		AuxilliaryMessages: slices.Clone(r.AuxilliaryMessages),
		PrimaryMessage:     r.PrimaryMessage,
		Usage:              r.Usage.Add(u),
	}
}

// Model defines an interface to an LLM.
type Model interface {
	// Tokens specifies how many tokens are allowed to be sent.
	Tokens() (int, int)
	// Responds to a set of input messages.
	Respond([]Message) (ModelResponse, error)
}

// ReasoningEffort defines how hard a reasoning model should think.
type ReasoningEffort uint8

const (
	LowReasoning ReasoningEffort = iota
	MediumReasoning
	HighReasoning
)

type Verbosity uint8

const (
	LowVerbosity Verbosity = iota
	MediumVerbosity
	HighVerbosity
)

type WithReasoningPrefix struct{ X string }
type WithDelay struct{ X time.Duration }
type WithTemperature struct{ X float64 }
type WithReasoningEffort struct{ X ReasoningEffort }
type WithURL struct{ X string }
type WithHTTPHeader struct {
	K string
	V string
}
type WithReasoningPrompt struct{ X string }
type WithVerbosity struct{ X Verbosity }
type WithTopP struct{ X int }
type WithPresencePenalty struct{ X float64 }
type WithPrediction struct{ X string }
type WithJsonSchema struct{ X map[string]any }
