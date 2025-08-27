package jpf

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

type ModelBuilder interface {
	New() (Model, error)
}

// Model defines an interface to an LLM.
type Model interface {
	// Tokens specifies how many tokens are allowed to be sent.
	Tokens() (int, int)
	// Responds to a set of input messages, with a set of auxilliary messages and a final message.
	// There may be no auxilliary messages, or things like tool calls, function calls, and reasoning may go in the auxilliary messages,
	Respond([]Message) ([]Message, Message, Usage, error)
}

// ReasoningEffort defines how hard a reasoning model should think.
type ReasoningEffort uint8

const (
	LowReasoning ReasoningEffort = iota
	MediumReasoning
	HighReasoning
)
