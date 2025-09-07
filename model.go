package jpf

import "time"

type ChatResult struct {
	Extra   []Message
	Primary Message
}

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

type ChatCaller Caller[[]Message, ChatResult]

// ReasoningEffort defines how hard a reasoning model should think.
type ReasoningEffort uint8

const (
	LowReasoning ReasoningEffort = iota
	MediumReasoning
	HighReasoning
)

type WithReasoningPrefix struct{ X string }
type WithRetries struct{ X int }
type WithDelay struct{ X time.Duration }
type WithTemperature struct{ X float64 }
type WithReasoningEffort struct{ X ReasoningEffort }
type WithURL struct{ X string }
type WithHTTPHeader struct {
	K string
	V string
}
type WithReasoningPrompt struct{ X string }
